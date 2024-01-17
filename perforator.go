package perforator

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/zyedidia/perf"
	"github.com/zyedidia/perforator/bininfo"
	"github.com/zyedidia/perforator/utrace"
)

// Events is a specification for which perf events should be tracked.  A Base
// set of events is tracked using standard perf, and sets of groups of events
// may also be given to avoid multiplexing between events in the same group.
type Events struct {
	Base   []perf.Configurator
	Groups [][]perf.Configurator
}

// Run executes the given command with tracing for certain events enabled. A
// structure with all perf metrics is returned.
func Run(target string, args []string,
	regionNames []string,
	events Events,
	attropts perf.Options,
	immediate func() MetricsWriter,
	ignoreMissingRegions bool,
	rangeInnerDelimiter string,
	excludeClones bool) (TotalMetrics, error) {

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	path, err := exec.LookPath(target)
	if err != nil {
		return TotalMetrics{}, fmt.Errorf("lookpath: %w", err)
	}

	f, err := os.Open(path)
	if err != nil {
		return TotalMetrics{}, fmt.Errorf("open: %w", err)
	}

	bin, err := bininfo.Read(f, f.Name())
	if err != nil {
		return TotalMetrics{}, fmt.Errorf("elf-read: %w", err)
	}

	var regions []utrace.Region
	var regionIds []int

	addregion := func(reg utrace.Region, id int) {
		regions = append(regions, reg)
		regionIds = append(regionIds, id)
	}

	for i, name := range regionNames {
		if strings.Contains(name, "-") {
			reg, err := ParseRegion(name, bin, rangeInnerDelimiter)
			if err != nil {
				return TotalMetrics{}, fmt.Errorf("region-parse: %w", err)
			}

			logger.Printf("%s: 0x%x-0x%x\n", name, reg.StartAddr, reg.EndAddr)

			addregion(reg, i)
		} else {
			fnpc, fnerr := bin.FuncToPC(name, excludeClones)

			if fnerr == nil {
				logger.Printf("%s: 0x%x\n", name, fnpc)
				addregion(&utrace.FuncRegion{
					Addr: fnpc,
				}, i)
			}

			inlinings, err := bin.InlinedFuncToPCs(name, excludeClones)

			if len(inlinings) == 0 {
				logger.Printf("%s not inlined (error: %s)\n", name, err)
			}

			if err != nil {
				if fnerr != nil {
					if err != nil && !ignoreMissingRegions {
						return TotalMetrics{}, fmt.Errorf("func-lookup: %w, inlined-func-lookup: %s", fnerr, err)
					}
				}

				continue
			}
			for _, in := range inlinings {
				logger.Printf("%s (inlined): 0x%x-0x%x\n", name, in.Low, in.High)

				addregion(&utrace.AddressRegion{
					StartAddr: in.Low,
					EndAddr:   in.High,
				}, i)
			}
		}
	}

	prog, pid, err := utrace.NewProgram(bin, target, args, regions)
	if err != nil {
		return TotalMetrics{}, err
	}

	fa := &perf.Attr{
		CountFormat: perf.CountFormat{
			Enabled: true,
			Running: true,
		},
		Options: attropts,
	}
	fa.Options.Disabled = true

	base := make([]*perf.Attr, len(events.Base))
	for i, c := range events.Base {
		attr := *fa
		c.Configure(&attr)
		base[i] = &attr
	}
	groups := make([][]*perf.Attr, len(events.Groups))
	for i, group := range events.Groups {
		for _, c := range group {
			attr := *fa
			c.Configure(&attr)
			groups[i] = append(groups[i], &attr)
		}
	}

	total := make(TotalMetrics, 0)
	ptable := make(map[int][]Profiler)
	ptable[pid], err = makeProfilers(pid, len(regions), base, groups, fa)
	if err != nil {
		return total, err
	}

	for {
		var ws utrace.Status

		p, evs, err := prog.Wait(&ws)
		if err == utrace.ErrFinishedTrace {
			break
		}
		if err != nil {
			return total, fmt.Errorf("wait: %w", err)
		}

		profilers, ok := ptable[p.Pid()]
		if !ok {
			ptable[p.Pid()], err = makeProfilers(p.Pid(), len(regions), base, groups, fa)
			if err != nil {
				return total, err
			}
		}

		for _, ev := range evs {
			switch ev.State {
			case utrace.RegionStart:
				logger.Printf("%d: Profiler %d enabled\n", p.Pid(), ev.Id)
				profilers[ev.Id].Disable()
				profilers[ev.Id].Reset()
				profilers[ev.Id].Enable()
			case utrace.RegionEnd:
				profilers[ev.Id].Disable()
				logger.Printf("%d: Profiler %d disabled\n", p.Pid(), ev.Id)
				nm := NamedMetrics{
					Metrics: profilers[ev.Id].Metrics(),
					Name:    regionNames[regionIds[ev.Id]],
				}
				total = append(total, nm)
				writer := immediate()
				if writer != nil {
					nm.WriteTo(writer)
				}
			}
		}

		err = prog.Continue(p, ws)
		if err != nil {
			return total, fmt.Errorf("trace-continue: %w", err)
		}
	}

	return total, nil
}

func makeProfilers(pid, n int, attrs []*perf.Attr, groups [][]*perf.Attr, fa *perf.Attr) ([]Profiler, error) {
	profilers := make([]Profiler, n)
	for i := 0; i < n; i++ {
		mprof, err := NewMultiProfiler(attrs, pid, perf.AnyCPU)
		if err != nil {
			return nil, fmt.Errorf("profiler: %w", err)
		}
		for _, gattrs := range groups {
			gprof, err := NewGroupProfiler(gattrs, pid, perf.AnyCPU)
			if err != nil {
				return nil, fmt.Errorf("profiler: %w", err)
			}
			mprof.profilers = append(mprof.profilers, gprof)
		}

		profilers[i] = mprof
	}
	return profilers, nil
}
