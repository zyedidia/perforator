package main

import (
	"strings"

	"acln.ro/perf"
	"github.com/zyedidia/perforator"
)

var opts struct {
	List        string   `short:"l" long:"list" description:"List available events for {hardware, software, cache, trace} event types"`
	Events      string   `short:"e" long:"events" default-mask:"-" default:"instructions,branch-instructions,branch-misses,cache-references,cache-misses" description:"Comma-separated list of events to profile"`
	GroupEvents []string `short:"g" long:"group" description:"Comma-separated list of events to profile together as a group"`
	Regions     []string `short:"r" long:"region" description:"Region(s) to profile: 'function' or 'start-end'; start/end locations may be file:line or hex addresses"`
	Kernel      bool     `long:"kernel" description:"Include kernel code in measurements"`
	Hypervisor  bool     `long:"hypervisor" description:"Include hypervisor code in measurements"`
	ExcludeUser bool     `long:"exclude-user" description:"Exclude user code from measurements"`
	Summary     bool     `short:"s" long:"summary" description:"Instead of printing results immediately, show an aggregated summary afterwards"`
	SortKey     string   `long:"sort-key" description:"Key to sort summary tables with"`
	ReverseSort bool     `long:"reverse-sort" description:"Reverse summary table sorting"`
	NoSort      bool     `long:"no-sort" description:"Don't sort the summary table"`
	Csv         bool     `long:"csv" description:"Write summary output in CSV format"`
	Output      string   `short:"o" long:"output" description:"Write summary output to file"`
	Verbose     bool     `short:"V" long:"verbose" description:"Show verbose debug information"`
	Version     bool     `short:"v" long:"version" description:"Show version information"`
	Help        bool     `short:"h" long:"help" description:"Show this help message"`
}

// ParseEventList looks at a comma-separated list of events and returns the
// perf Configurators corresponding to those events.
func ParseEventList(s string) ([]perf.Configurator, error) {
	parts := strings.Split(s, ",")
	var configs []perf.Configurator
	var errs []error
	for _, ev := range parts {
		event, err := perforator.NameToConfig(ev)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		configs = append(configs, event)
	}

	return configs, perforator.MultiErr(errs)
}
