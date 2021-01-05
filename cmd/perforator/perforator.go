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
	"github.com/zyedidia/utrace"
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

	total, err := perforator.Run(target, args, opts.Regions, evs, perfOpts, out)
	if err != nil {
		fatal(err)
	}

	if opts.Summary {
		fmt.Print(total.String(opts.SortKey, opts.ReverseSort))
	}
}