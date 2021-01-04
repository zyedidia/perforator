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

	bin, err := bininfo.Read(f)
	must("elf-read", err)

	var regionNames []string
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
		regionNames = append(regionNames, fn)
	}

	for _, r := range opts.Regions {
		reg, err := ParseRegion(r, bin)
		if err != nil {
			fmt.Fprintln(os.Stderr, "region-parse :", err)
			continue
		}
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
		must("event-parse", err)
	}

	profilers := make([]Profiler, len(regions))
	for i := 0; i < len(regions); i++ {
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

	for {
		var ws utrace.Status

		p, evs, err := prog.Wait(&ws)
		must("wait", err)

		for _, ev := range evs {
			switch ev.State {
			case utrace.RegionStart:
				profilers[ev.Id].Disable()
				profilers[ev.Id].Reset()
				profilers[ev.Id].Enable()
			case utrace.RegionEnd:
				profilers[ev.Id].Disable()
				fmt.Printf("Summary for '%s':\n", regionNames[ev.Id])
				fmt.Print(profilers[ev.Id].Metrics())
				if err != nil {
					fmt.Fprintln(os.Stderr, "count error :", err)
				}
			}
		}

		prog.Continue(p, ws)
	}
}
