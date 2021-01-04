package main

import (
	"errors"
	"strconv"
	"strings"

	"acln.ro/perf"
	"github.com/zyedidia/utrace"
	"github.com/zyedidia/utrace/bininfo"
)

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

func parseLocation(s string, bin *bininfo.BinFile) (uint64, error) {
	if strings.Contains(s, ":") {
		parts := strings.Split(s, ":")
		file, lineStr := parts[0], parts[1]
		line, err := strconv.Atoi(lineStr)
		if err != nil {
			return 0, err
		}
		return bin.LineToPC(file, line)
	}
	return strconv.ParseUint(s, 0, 64)
}

func ParseRegion(s string, bin *bininfo.BinFile) (*utrace.AddressRegion, error) {
	parts := strings.Split(s, "-")
	if len(parts) != 2 {
		return nil, errors.New("invalid region")
	}

	start, err := parseLocation(parts[0], bin)
	if err != nil {
		return nil, err
	}
	end, err := parseLocation(parts[1], bin)
	if err != nil {
		return nil, err
	}

	return &utrace.AddressRegion{
		StartAddr: start,
		EndAddr:   end,
	}, nil
}

func ParseEventList(s string, config *perf.Attr) ([]*perf.Attr, error) {
	parts := strings.Split(s, ",")
	var attrs []*perf.Attr
	for _, ev := range parts {
		fa := *config
		// TODO: handle error
		event, _ := NameToConfig(ev)
		event.Configure(&fa)

		attrs = append(attrs, &fa)
	}

	return attrs, nil
}
