package main

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"

	"acln.ro/perf"
	"github.com/jessevdk/go-flags"
	"github.com/zyedidia/utrace"
	"github.com/zyedidia/utrace/bininfo"
)

func fatal(a ...interface{}) {
	fmt.Fprintln(os.Stderr, a...)
	os.Exit(1)
}

func must(desc string, err error) {
	if err != nil {
		fatal(desc, ":", err)
	}
}

var opts struct {
	List        string   `short:"l" long:"list" description:"List available events for {hardware, software, cache, trace} event types"`
	Events      string   `short:"e" long:"events" description:"Comma-separated list of events to profile"`
	GroupEvents []string `short:"g" long:"group" description:"Comma-separated list of events to profile together as a group"`
	Fns         []string `short:"f" long:"func" description:"Function(s) to profile"`
	Regions     []string `short:"r" long:"region" description:"Region(s) to profile: 'start-end'; locations may be file:line or hex addresses"`
	Kernel      bool     `long:"kernel" description:"Include kernel code in measurements"`
	Hypervisor  bool     `long:"hypervisor" description:"Include hypervisor code in measurements"`
	ExcludeUser bool     `long:"exclude-user" description:"Exclude user code from measurements"`
	Verbose     bool     `short:"V" long:"verbose" description:"Show verbose debug information"`
	Version     bool     `short:"v" long:"version" description:"Show version information"`
	Help        bool     `short:"h" long:"help" description:"Show this help message"`
}

func init() {
	runtime.LockOSThread()
}

func main() {
	flagparser := flags.NewParser(&opts, flags.PassDoubleDash|flags.PrintErrors)
	args, err := flagparser.Parse()
	if err != nil {
		os.Exit(1)
	}

	if len(args) <= 0 || opts.Help {
		flagparser.WriteHelp(os.Stdout)
		os.Exit(0)
	}

	if opts.List != "" {
		var events []string
		switch opts.List {
		case "software":
			events = AvailableSoftwareEvents()
		case "hardware":
			events = AvailableHardwareEvents()
		case "cache":
			events = AvailableCacheEvents()
		case "trace":
			events = AvailableTraceEvents()
		default:
			fatal("error: invalid event type", opts.List)
		}

		for _, e := range events {
			fmt.Printf("[%s event]: %s\n", opts.List, e)
		}
		os.Exit(0)
	}

	target := args[0]
	args = args[1:]

	path, err := exec.LookPath(target)
	must("lookpath", err)

	f, err := os.Open(path)
	must("open", err)

	bin, err := bininfo.Read(f)
	must("elf-read", err)

	var regions []utrace.Region
	for _, fn := range opts.Fns {
		fnpc, err := bin.FuncToPC(fn)
		if err != nil {
			fmt.Fprintln(os.Stderr, "func-lookup :", err)
			continue
		}

		regions = append(regions, &utrace.FuncRegion{
			Addr: fnpc,
		})
	}

	prog, pid, err := utrace.NewProgram(bin, target, args, regions)
	must("trace", err)

	fa := &perf.Attr{
		CountFormat: perf.CountFormat{
			Enabled: true,
			Running: true,
		},
		Options: perf.Options{
			ExcludeKernel:     !opts.Kernel,
			ExcludeHypervisor: !opts.Hypervisor,
			ExcludeUser:       opts.ExcludeUser,
			Disabled:          true,
		},
	}

	attr, err := NameToConfig(opts.Events)
	must("perf-lookup", err)
	attr.Configure(fa)

	profilers := make([]*perf.Event, len(regions))
	for i := 0; i < len(regions); i++ {
		hw, err := perf.Open(fa, pid, perf.AnyCPU, nil)
		if err != nil {
			fmt.Fprintln(os.Stderr, "perf-open :", err)
			continue
		}
		profilers[i] = hw
	}

	for {
		var ws utrace.Status

		p, evs, err := prog.Wait(&ws)
		must("wait", err)

		for _, ev := range evs {
			switch ev.State {
			case utrace.RegionStart:
				profilers[ev.Id].Reset()
				profilers[ev.Id].Enable()
			case utrace.RegionEnd:
				profilers[ev.Id].Disable()
				c, err := profilers[ev.Id].ReadCount()
				if err != nil {
					fmt.Fprintln(os.Stderr, "count error :", err)
				} else {
					fmt.Println(c)
					fmt.Println(c.Enabled)
				}
			}
		}

		prog.Continue(p, ws)
	}
}
