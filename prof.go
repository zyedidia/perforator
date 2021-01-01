package main

import (
	"github.com/hodgesds/perf-utils"
	"golang.org/x/sys/unix"
)

type ProfRegion struct {
	block Block
	attrs []unix.PerfEventAttr

	profiler perf.GroupProfiler
}
