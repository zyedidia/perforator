package perforator

import (
	"fmt"
	"io/ioutil"
	"sort"
	"strings"

	"github.com/zyedidia/perf"
)

const (
	traceDir = "/sys/kernel/debug/tracing"
)

var hardwareEvents = map[string]perf.HardwareCounter{
	"instructions":            perf.Instructions,
	"cpu-cycles":              perf.CPUCycles,
	"cache-references":        perf.CacheReferences,
	"cache-misses":            perf.CacheMisses,
	"branch-instructions":     perf.BranchInstructions,
	"branch-misses":           perf.BranchMisses,
	"bus-cycles":              perf.BusCycles,
	"stalled-cycles-frontend": perf.StalledCyclesFrontend,
	"stalled-cycles-backend":  perf.StalledCyclesBackend,
	"ref-cycles":              perf.RefCPUCycles,
}

var softwareEvents = map[string]perf.SoftwareCounter{
	"cpu-clock":        perf.CPUClock,
	"task-clock":       perf.TaskClock,
	"page-faults":      perf.PageFaults,
	"context-switches": perf.ContextSwitches,
	"cpu-migrations":   perf.CPUMigrations,
	"minor-faults":     perf.MinorPageFaults,
	"major-faults":     perf.MajorPageFaults,
	"alignment-faults": perf.AlignmentFaults,
	"emulation-faults": perf.EmulationFaults,
}

var caches = map[string]perf.Cache{
	"l1d":  perf.L1D,
	"l1i":  perf.L1I,
	"ll":   perf.LL,
	"dtlb": perf.DTLB,
	"itlb": perf.ITLB,
	"bpu":  perf.BPU,
	"node": perf.NODE,
}

var cacheAccesses = map[string]perf.CacheOp{
	"read":     perf.Read,
	"write":    perf.Write,
	"prefetch": perf.Prefetch,
}

var cacheResults = map[string]perf.CacheOpResult{
	"accesses": perf.Access,
	"misses":   perf.Miss,
}

type cacheEvent struct {
	cache  perf.Cache
	op     perf.CacheOp
	result perf.CacheOpResult
	label  string
}

func (e cacheEvent) Configure(attr *perf.Attr) error {
	attr.Type = perf.HardwareCacheEvent
	attr.Config = uint64(e.cache) | uint64(e.op<<8) | uint64(e.result<<16)
	attr.Label = e.label
	return nil
}

func cacheEvents() map[string]cacheEvent {
	events := make(map[string]cacheEvent)
	for cn, c := range caches {
		for an, a := range cacheAccesses {
			for rn, r := range cacheResults {
				evn := fmt.Sprintf("%s-%s-%s", cn, an, rn)
				event := cacheEvent{
					cache:  c,
					op:     a,
					result: r,
					label:  evn,
				}
				events[evn] = event
			}
		}
	}
	return events
}

// IsAvailable returns true if the given event is available on the current
// system.
func IsAvailable(ev perf.Configurator) bool {
	fa := &perf.Attr{}
	ev.Configure(fa)
	p, err := perf.Open(fa, perf.CallingThread, perf.AnyCPU, nil)
	if err == nil {
		p.Close()
		return true
	}
	return false
}

// AvailableHardwareEvents returns the list of available hardware events.
func AvailableHardwareEvents() []string {
	events := make([]string, 0, len(hardwareEvents))
	for evn, ev := range hardwareEvents {
		if !IsAvailable(ev) {
			continue
		}
		events = append(events, evn)
	}
	sort.Strings(events)
	return events
}

// AvailableSoftwareEvents returns the list of available software events.
func AvailableSoftwareEvents() []string {
	events := make([]string, 0, len(softwareEvents))
	for evn, ev := range softwareEvents {
		if !IsAvailable(ev) {
			continue
		}
		events = append(events, evn)
	}
	sort.Strings(events)
	return events
}

// AvailableCacheEvents returns the list of available cache events.
func AvailableCacheEvents() []string {
	cevs := cacheEvents()
	events := make([]string, 0, len(cevs))
	for evn, ev := range cevs {
		if !IsAvailable(ev) {
			continue
		}
		events = append(events, evn)
	}
	sort.Strings(events)
	return events
}

// AvailableTraceEvents returns the list of available trace events.
func AvailableTraceEvents() []string {
	events, err := ioutil.ReadFile(traceDir + "/available_events")
	if err != nil {
		return nil
	}
	lines := strings.Split(string(events), "\n")
	sort.Strings(lines)
	return lines
}

// NameToConfig converts a string representation of an event to a perf
// configurator.
func NameToConfig(name string) (perf.Configurator, error) {
	if ev, ok := hardwareEvents[name]; ok {
		return ev, nil
	} else if ev, ok := softwareEvents[name]; ok {
		return ev, nil
	} else if ev, ok := cacheEvents()[name]; ok {
		return ev, nil
	} else if strings.Contains(name, ":") {
		parts := strings.Split(name, ":")
		subsystem, event := parts[0], parts[1]
		config := perf.Tracepoint(subsystem, event)
		return config, nil
	}

	return nil, fmt.Errorf("not found: event %s", name)
}
