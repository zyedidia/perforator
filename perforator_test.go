package perforator

import (
	"io/ioutil"
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
	total, err := Run(target, []string{}, regions, evs, opts, ioutil.Discard)
	must(err, t)

	for k, v := range total {
		ev, ok := expected[k]
		if !ok {
			t.Errorf("had unexpected extra region %s", k)
		}
		if len(ev.results) != len(v.results) {
			t.Errorf("unexpected result length %d", len(v.results))
		}
		for i, result := range v.results {
			if abs(int(result.Value)-int(ev.results[i].Value)) > near {
				t.Errorf("unexpected result for %s: %d", k, result.Value)
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
		"main.sum": Metrics{
			results: []Result{
				Result{
					Label: "instructions",
					Value: 70000000,
				},
				Result{
					Label: "branch-instructions",
					Value: 10000000,
				},
				Result{
					Label: "branch-misses",
					Value: 10,
				},
			},
		},
	}
	check("test/sum", regions, events, expected, t)
}
