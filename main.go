package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/hodgesds/perf-utils"
	"github.com/olekukonko/tablewriter"
	"github.com/zyedidia/perforator/bininfo"
	"github.com/zyedidia/perforator/pevents"
	"golang.org/x/sys/unix"
)

func fatal(a ...interface{}) {
	fmt.Fprintln(os.Stderr, a...)
	os.Exit(1)
}

func must(err error) {
	if err != nil {
		fatal(err)
	}
}

type Funcs map[string]uintptr

func init() {
	runtime.LockOSThread()
}

var fn = flag.String("fn", "", "function to profile")
var verbose = flag.Bool("V", false, "verbose output")
var kernel = flag.Bool("kernel", false, "include kernel code while profiling")
var events = flag.String("e", "", "comma-separated list of events to profile")
var list = flag.String("events", "", "list available events for {hardware, software, cache, trace} event types")

var defaultEvents = []string{
	"instruction",
	"cache-ref",
	"cache-miss",
	"branch",
	"branch-miss",
}

func main() {
	flag.Parse()

	if *list != "" {
		available := map[string]func() []string{
			"hardware": pevents.AvailableHardwareEvents,
			"software": pevents.AvailableSoftwareEvents,
			"cache":    pevents.AvailableCacheEvents,
			"trace":    pevents.AvailableTracepoints,
		}
		if fn, ok := available[*list]; ok {
			evs := fn()

			if len(evs) == 0 {
				fatal("no events found, do you have the right permissions?")
			}

			for _, ev := range evs {
				fmt.Printf("[%s event]: %s\n", *list, ev)
			}
			os.Exit(0)
		} else {
			fatal("Invalid event type, must be one of {hardware, software, cache, trace}.")
		}
	}

	if len(flag.Args()) <= 0 {
		fatal("no command given")
	}

	if *fn == "" {
		fatal("no function given")
	}

	if !*verbose {
		log.SetOutput(NullWriter{})
	}

	var evsplit []string
	if *events == "" {
		evsplit = defaultEvents
	} else {
		evsplit = strings.Split(*events, ",")
	}
	eventAttrs := make([]unix.PerfEventAttr, 0, len(evsplit))
	for _, ev := range evsplit {
		event, err := pevents.NameToEvent(ev)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			continue
		}
		if pevents.IsAvailable(event) {
			attr := unix.PerfEventAttr{
				Type:        event.Type,
				Config:      event.Config,
				Size:        perf.EventAttrSize,
				Bits:        unix.PerfBitExcludeHv,
				Read_format: unix.PERF_FORMAT_TOTAL_TIME_RUNNING | unix.PERF_FORMAT_TOTAL_TIME_ENABLED,
			}
			if !*kernel {
				attr.Bits |= unix.PerfBitExcludeKernel
			}
			eventAttrs = append(eventAttrs, attr)
		}
	}

	if len(eventAttrs) == 0 {
		fatal("no valid events (do you have the right permissions?)")
	}

	target := flag.Args()[0]
	args := flag.Args()[1:]

	data, err := ioutil.ReadFile(target)
	if err != nil {
		fatal(err)
	}

	bin, aslr, err := bininfo.OpenBinFile(data)
	if err != nil {
		fatal(err)
	}

	var fnaddr uintptr
	fns := bin.FuzzyFunc(*fn)
	if len(fns) == 0 {
		fatal("function", *fn, "not found.")
	} else if len(fns) == 1 {
		fnaddr, _ = bin.FuncToPC(fns[0])
	} else {
		fmt.Println("Multiple functions matched, please pick one:")
		for _, fn := range fns {
			fmt.Println(fn)
		}
		os.Exit(0)
	}

	log.Printf("%s: 0x%x\n", *fn, fnaddr)
	proc, err := StartProc(target, args, &ProfRegion{
		block: &FuncBlock{
			addr: fnaddr,
		},
		attrs: eventAttrs,
		callback: func(prof *ProfRegion, result *perf.GroupProfileValue) {
			table := tablewriter.NewWriter(os.Stdout)
			table.SetHeader([]string{"Event", "Count"})
			// scale := result.TimeEnabled / result.TimeRunning
			scale := uint64(1)

			for i, v := range result.Values {
				ev := pevents.Event{
					Type:   prof.attrs[i].Type,
					Config: prof.attrs[i].Config,
				}
				table.Append([]string{
					pevents.EventToName(ev),
					fmt.Sprintf("%d", v*scale),
				})
			}
			table.Append([]string{
				"time elapsed",
				fmt.Sprintf("%s", time.Duration(result.TimeEnabled)),
			})

			table.SetHeaderAlignment(tablewriter.ALIGN_LEFT)
			table.SetAlignment(tablewriter.ALIGN_LEFT)
			// table.SetBorders(tablewriter.Border{Left: true, Top: false, Right: true, Bottom: false})
			// table.SetCenterSeparator("|")

			table.Render()
		},
	}, aslr)
	if err != nil {
		fatal(err)
	}

	proc.Continue(0)

	var ws unix.WaitStatus
	for {
		_, err := unix.Wait4(-1, &ws, 0, nil)
		must(err)

		var sig unix.Signal
		if ws.Exited() {
			break
		} else if ws.Stopped() && ws.StopSignal() != unix.SIGTRAP {
			sig = ws.StopSignal()
		} else {
			proc.HandleInterrupt()
			sig = 0
		}

		proc.Continue(sig)
	}
}
