package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"runtime"

	"acln.ro/perf"
	"github.com/jessevdk/go-flags"
	"github.com/zyedidia/perforator"
	"github.com/zyedidia/perforator/utrace"
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

func metricsWriter(w io.Writer) perforator.MetricsWriter {
	if opts.Csv {
		return perforator.NewCSVWriter(w)
	}
	return perforator.NewTableWriter(w)
}

func main() {
	runtime.LockOSThread()

	flagparser := flags.NewParser(&opts, flags.PassDoubleDash|flags.PrintErrors)
	flagparser.Usage = "[OPTIONS] COMMAND [ARGS]"
	args, err := flagparser.Parse()
	if err != nil {
		os.Exit(1)
	}

	if opts.Version {
		fmt.Println("perforator version", Version)
		os.Exit(0)
	}

	if opts.Verbose {
		logger := log.New(os.Stdout, "INFO: ", 0)
		perforator.SetLogger(logger)
		utrace.SetLogger(logger)
	}

	if opts.List != "" {
		var events []string
		switch opts.List {
		case "software":
			events = perforator.AvailableSoftwareEvents()
		case "hardware":
			events = perforator.AvailableHardwareEvents()
		case "cache":
			events = perforator.AvailableCacheEvents()
		case "trace":
			events = perforator.AvailableTraceEvents()
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

	perfOpts := perf.Options{
		ExcludeKernel:     !opts.Kernel,
		ExcludeHypervisor: !opts.Hypervisor,
		ExcludeUser:       opts.ExcludeUser,
	}

	var configs []perf.Configurator
	if len(opts.Events) >= 1 {
		configs, err = ParseEventList(opts.Events)
		if len(configs) == 0 {
			fmt.Println("No events found, do you have the right permissions?")
		}
		must("event-parse", err)
	}

	var groups [][]perf.Configurator
	for _, g := range opts.GroupEvents {
		gconfigs, err := ParseEventList(g)
		must("group-parse", err)
		groups = append(groups, gconfigs)
	}

	evs := perforator.Events{
		Base:   configs,
		Groups: groups,
	}

	var out io.Writer = os.Stdout
	if opts.Summary {
		out = ioutil.Discard
	}
	immediate := func() perforator.MetricsWriter {
		return metricsWriter(out)
	}

	total, err := perforator.Run(target, args, opts.Regions, evs, perfOpts, immediate)
	if err != nil {
		fatal(err)
	}

	if opts.Summary {
		var out io.WriteCloser = os.Stdout

		if opts.Output != "" {
			f, err := os.OpenFile(opts.Output, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0666)
			if err != nil {
				fmt.Fprintln(os.Stderr, "open-output :", err)
			}
			out = f
		}

		mv := metricsWriter(out)
		if opts.NoSort {
			total.WriteTo(mv)
		} else {
			total.WriteToSorted(mv, opts.SortKey, opts.ReverseSort)
		}
		out.Close()
	}
}
