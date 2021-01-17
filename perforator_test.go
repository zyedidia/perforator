package perforator

import (
	"os/exec"
	"runtime"
	"testing"

	"acln.ro/perf"
)

// Tests require permissions to run perf from user code (see the perf paranoid
// setting). The test also may depend on particular hardware characteristics.
// If the reported metrics are wildly different from the expected results, this
// probably indicates an error rather than different hardware, but I'm not
// sure.
// Try: `sudo sh -c 'echo 0 >/proc/sys/kernel/perf_event_paranoid'`

const near = 100000

func must(err error, t *testing.T) {
	if err != nil {
		t.Error(err)
	}
}

func abs(a int) int {
	if a < 0 {
		return -a
	}
	return a
}

func buildGo(src, out string, dwarf, pie bool) error {
	args := []string{"build"}
	if !dwarf {
		args = append(args, "-ldflags", "-w")
	}
	if pie {
		args = append(args, "-buildmode=pie")
	}
	args = append(args, "-o", out, src)

	cmd := exec.Command("go", args...)
	_, err := cmd.Output()
	return err
}

func check(target string, regions []string, events []perf.Configurator, expected TotalMetrics, t *testing.T) {
	evs := Events{
		Base: events,
	}
	opts := perf.Options{
		ExcludeKernel:     true,
		ExcludeHypervisor: true,
	}
	total, err := Run(target, []string{}, regions, evs, opts, func() MetricsWriter { return nil })
	must(err, t)

	for i, v := range total {
		nm := expected[i]
		if len(nm.Results) != len(v.Results) {
			t.Errorf("unexpected result length %d", len(v.Results))
		}
		for i, result := range v.Results {
			if abs(int(result.Value)-int(nm.Results[i].Value)) > near {
				t.Errorf("unexpected result for %s: %d", nm.Name, result.Value)
			}
		}
	}
}

// Tests a single region with PIE active (the test target is a Go program, so
// it also tests multithreading support, since the Go runtime automatically
// spawns threads).
func TestSingleRegion(t *testing.T) {
	runtime.LockOSThread()

	must(buildGo("test/sum.go", "test/sum", true, true), t)
	regions := []string{
		"main.sum",
	}
	events := []perf.Configurator{
		perf.Instructions,
		perf.BranchInstructions,
		perf.BranchMisses,
	}
	expected := TotalMetrics{
		NamedMetrics{
			Name: "main.sum",
			Metrics: Metrics{
				Results: []Result{
					{
						Label: "instructions",
						Value: 70000000,
					},
					{
						Label: "branch-instructions",
						Value: 10000000,
					},
					{
						Label: "branch-misses",
						Value: 10,
					},
				},
			},
		},
	}
	check("test/sum", regions, events, expected, t)
}
