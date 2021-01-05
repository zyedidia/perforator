package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"runtime"

	"acln.ro/perf"
	"github.com/jessevdk/go-flags"
	"github.com/zyedidia/utrace"
	"github.com/zyedidia/utrace/bininfo"
)

func perr(desc string, err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, desc, ":", err)
	}
}

func fatal(a ...interface{}) {
	fmt.Fprintln(os.Stderr, a...)
	os.Exit(1)
}

func must(desc string, err error) {
	if err != nil {
		fatal(desc, ":", err)
	}
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

	if opts.Verbose {
		Logger = log.New(os.Stdout, "INFO: ", 0)
		utrace.SetLogger(Logger)
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

		if len(events) == 0 {
			fmt.Println("No events found, do you have the right permissions?")
		}

		for _, e := range events {
			fmt.Printf("[%s event]: %s\n", opts.List, e)
		}
		os.Exit(0)
	}

	if len(args) <= 0 || opts.Help {
		flagparser.WriteHelp(os.Stdout)
		os.Exit(0)
	}

	target := args[0]
	args = args[1:]

	path, err := exec.LookPath(target)
	must("lookpath", err)

	f, err := os.Open(path)
	must("open", err)

	bin, err := bininfo.Read(f, f.Name())
	must("elf-read", err)

	var regionNames []string
	var regions []utrace.Region
	for _, fn := range opts.Fns {
		fnpc, err := bin.FuncToPC(fn)
		if err != nil {
			perr("func-lookup", err)
			continue
		}

		Logger.Printf("%s: 0x%x\n", fn, fnpc)

		regions = append(regions, &utrace.FuncRegion{
			Addr: fnpc,
		})
		regionNames = append(regionNames, fn)
	}

	for _, r := range opts.Regions {
		reg, err := ParseRegion(r, bin)
		if err != nil {
			perr("region-parse", err)
			continue
		}

		Logger.Printf("%s: 0x%x-0x%x\n", r, reg.StartAddr, reg.EndAddr)

		regions = append(regions, reg)
		regionNames = append(regionNames, r)
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

	var attrs []*perf.Attr
	if len(opts.Events) >= 1 {
		attrs, err = ParseEventList(opts.Events, fa)
		if len(attrs) == 0 {
			fmt.Println("No events found, do you have the right permissions?")
		}
		must("event-parse", err)
	}

	ptable := make(map[int][]Profiler)
	ptable[pid] = makeProfilers(pid, len(regions), attrs, fa)

	total := make(TotalMetrics)

	for {
		var ws utrace.Status

		p, evs, err := prog.Wait(&ws)
		if err == utrace.ErrFinishedTrace {
			break
		}
		must("wait", err)

		profilers, ok := ptable[p.Pid()]
		if !ok {
			ptable[p.Pid()] = makeProfilers(p.Pid(), len(regions), attrs, fa)
		}

		for _, ev := range evs {
			switch ev.State {
			case utrace.RegionStart:
				Logger.Printf("%d: Profiling enabled\n", p.Pid())
				profilers[ev.Id].Disable()
				profilers[ev.Id].Reset()
				profilers[ev.Id].Enable()
			case utrace.RegionEnd:
				profilers[ev.Id].Disable()
				Logger.Printf("%d: Profiling disabled\n", p.Pid())
				if opts.Summary {
					total[regionNames[ev.Id]] = profilers[ev.Id].Metrics()
				} else {
					fmt.Printf("Summary for '%s':\n", regionNames[ev.Id])
					fmt.Print(profilers[ev.Id].Metrics())
				}
			}
		}

		err = prog.Continue(p, ws)
		must("trace-continue", err)
	}

	if opts.Summary {
		fmt.Print(total.String(opts.SortKey, opts.ReverseSort))
	}
}

func makeProfilers(pid, n int, attrs []*perf.Attr, fa *perf.Attr) []Profiler {
	profilers := make([]Profiler, n)
	for i := 0; i < n; i++ {
		mprof, err := NewMultiProfiler(attrs, pid, perf.AnyCPU)
		must("profiler", err)
		for _, g := range opts.GroupEvents {
			gattrs, err := ParseEventList(g, fa)
			must("group-event-parse", err)
			gprof, err := NewGroupProfiler(gattrs, pid, perf.AnyCPU)
			must("group-profiler", err)
			mprof.profilers = append(mprof.profilers, gprof)
		}

		profilers[i] = mprof
	}
	return profilers
}
