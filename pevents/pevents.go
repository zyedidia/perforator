package pevents

import (
	"fmt"
	"sort"
	"strings"

	"github.com/hodgesds/perf-utils"
	"golang.org/x/sys/unix"
)

type Event struct {
	Type   uint32
	Config uint64
}

var hardwareEvents = map[string]Event{
	"instruction": Event{
		Type:   unix.PERF_TYPE_HARDWARE,
		Config: unix.PERF_COUNT_HW_INSTRUCTIONS,
	},
	"cycle": Event{
		Type:   unix.PERF_TYPE_HARDWARE,
		Config: unix.PERF_COUNT_HW_CPU_CYCLES,
	},
	"cache-ref": Event{
		Type:   unix.PERF_TYPE_HARDWARE,
		Config: unix.PERF_COUNT_HW_CACHE_REFERENCES,
	},
	"cache-miss": Event{
		Type:   unix.PERF_TYPE_HARDWARE,
		Config: unix.PERF_COUNT_HW_CACHE_MISSES,
	},
	"branch": Event{
		Type:   unix.PERF_TYPE_HARDWARE,
		Config: unix.PERF_COUNT_HW_BRANCH_INSTRUCTIONS,
	},
	"branch-miss": Event{
		Type:   unix.PERF_TYPE_HARDWARE,
		Config: unix.PERF_COUNT_HW_BRANCH_MISSES,
	},
	"bus-cycle": Event{
		Type:   unix.PERF_TYPE_HARDWARE,
		Config: unix.PERF_COUNT_HW_BUS_CYCLES,
	},
	"stalled-cycle-frontend": Event{
		Type:   unix.PERF_TYPE_HARDWARE,
		Config: unix.PERF_COUNT_HW_STALLED_CYCLES_FRONTEND,
	},
	"stalled-cycle-backend": Event{
		Type:   unix.PERF_TYPE_HARDWARE,
		Config: unix.PERF_COUNT_HW_STALLED_CYCLES_BACKEND,
	},
	"ref-cycle": Event{
		Type:   unix.PERF_TYPE_HARDWARE,
		Config: unix.PERF_COUNT_HW_REF_CPU_CYCLES,
	},
}

var softwareEvents = map[string]Event{
	"cpu-clock": Event{
		Type:   unix.PERF_TYPE_SOFTWARE,
		Config: unix.PERF_COUNT_SW_CPU_CLOCK,
	},
	"task-clock": Event{
		Type:   unix.PERF_TYPE_SOFTWARE,
		Config: unix.PERF_COUNT_SW_TASK_CLOCK,
	},
	"page-fault": Event{
		Type:   unix.PERF_TYPE_SOFTWARE,
		Config: unix.PERF_COUNT_SW_PAGE_FAULTS,
	},
	"ctx-switch": Event{
		Type:   unix.PERF_TYPE_SOFTWARE,
		Config: unix.PERF_COUNT_SW_CONTEXT_SWITCHES,
	},
	"migration": Event{
		Type:   unix.PERF_TYPE_SOFTWARE,
		Config: unix.PERF_COUNT_SW_CPU_MIGRATIONS,
	},
	"minor-fault": Event{
		Type:   unix.PERF_TYPE_SOFTWARE,
		Config: unix.PERF_COUNT_SW_PAGE_FAULTS_MIN,
	},
	"major-fault": Event{
		Type:   unix.PERF_TYPE_SOFTWARE,
		Config: unix.PERF_COUNT_SW_PAGE_FAULTS_MAJ,
	},
	"align-fault": Event{
		Type:   unix.PERF_TYPE_SOFTWARE,
		Config: unix.PERF_COUNT_SW_ALIGNMENT_FAULTS,
	},
	"emul-fault": Event{
		Type:   unix.PERF_TYPE_SOFTWARE,
		Config: unix.PERF_COUNT_SW_EMULATION_FAULTS,
	},
}

var caches = map[string]uint64{
	"l1d":  unix.PERF_COUNT_HW_CACHE_L1D,
	"l1i":  unix.PERF_COUNT_HW_CACHE_L1I,
	"ll":   unix.PERF_COUNT_HW_CACHE_LL,
	"dtlb": unix.PERF_COUNT_HW_CACHE_DTLB,
	"itlb": unix.PERF_COUNT_HW_CACHE_ITLB,
	"bpu":  unix.PERF_COUNT_HW_CACHE_BPU,
	"node": unix.PERF_COUNT_HW_CACHE_NODE,
}

var cache_accesses = map[string]uint64{
	"read":     unix.PERF_COUNT_HW_CACHE_OP_READ,
	"write":    unix.PERF_COUNT_HW_CACHE_OP_WRITE,
	"prefetch": unix.PERF_COUNT_HW_CACHE_OP_PREFETCH,
}

var cache_results = map[string]uint64{
	"access": unix.PERF_COUNT_HW_CACHE_RESULT_ACCESS,
	"miss":   unix.PERF_COUNT_HW_CACHE_RESULT_MISS,
}

func cacheEvents() map[string]Event {
	events := make(map[string]Event)
	for cn, c := range caches {
		for an, a := range cache_accesses {
			for rn, r := range cache_results {
				evn := fmt.Sprintf("%s-%s-%s", cn, an, rn)
				event := Event{
					Type:   unix.PERF_TYPE_HW_CACHE,
					Config: c | a<<8 | r<<16,
				}
				events[evn] = event
			}
		}
	}
	return events
}

func IsAvailable(ev Event) bool {
	p, err := perf.NewProfiler(ev.Type, ev.Config, 0, -1, 0)
	if err == nil {
		p.Close()
		return true
	}
	return false
}

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

func AvailableTracepoints() []string {
	evs, err := perf.AvailableEvents()
	if err != nil {
		return []string{}
	}
	names := make([]string, 0, len(evs))
	for subsystem, v := range evs {
		for _, event := range v {
			names = append(names, fmt.Sprintf("%s:%s", subsystem, event))
		}
	}
	sort.Strings(names)
	return names
}

func EventToName(ev Event) string {
	// this function is pretty inefficient but I don't think it's a big deal
	switch ev.Type {
	case unix.PERF_TYPE_HARDWARE:
		for k, v := range hardwareEvents {
			if v == ev {
				return k
			}
		}
	case unix.PERF_TYPE_SOFTWARE:
		for k, v := range softwareEvents {
			if v == ev {
				return k
			}
		}
	case unix.PERF_TYPE_HW_CACHE:
		cevs := cacheEvents()
		for k, v := range cevs {
			if v == ev {
				return k
			}
		}
	case unix.PERF_TYPE_TRACEPOINT:
		evs, err := perf.AvailableEvents()
		if err != nil {
			break
		}
		for subsystem, v := range evs {
			for _, event := range v {
				if config, err := perf.GetTracepointConfig(subsystem, event); err == nil && config == ev.Config {
					return fmt.Sprintf("%s:%s", subsystem, event)
				}
			}
		}
	}
	return "unknown"
}

func NameToEvent(name string) (Event, error) {
	if ev, ok := hardwareEvents[name]; ok {
		return ev, nil
	} else if ev, ok := softwareEvents[name]; ok {
		return ev, nil
	} else if ev, ok := cacheEvents()[name]; ok {
		return ev, nil
	} else if strings.Contains(name, ":") {
		parts := strings.Split(name, ":")
		subsystem, event := parts[0], parts[1]
		config, err := perf.GetTracepointConfig(subsystem, event)
		if err != nil {
			return Event{}, err
		}
		return Event{
			Type:   unix.PERF_TYPE_TRACEPOINT,
			Config: config,
		}, nil
	}

	return Event{}, fmt.Errorf("not found: event %s", name)
}
