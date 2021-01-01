package main

import (
	"github.com/hodgesds/perf-utils"
	"golang.org/x/sys/unix"
)

type ProfFunc func(prof *ProfRegion, result *perf.GroupProfileValue)

type ProfRegion struct {
	block    Block
	attrs    []unix.PerfEventAttr
	callback ProfFunc

	profiler perf.GroupProfiler
}
