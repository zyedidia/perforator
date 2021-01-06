package perforator

import (
	"bytes"
	"time"

	"acln.ro/perf"
)

type Profiler interface {
	Enable() error
	Disable() error
	Reset() error
	Metrics() Metrics
}

type MultiError struct {
	errs []error
}

func (e *MultiError) Error() string {
	b := &bytes.Buffer{}
	for _, err := range e.errs {
		b.WriteString(err.Error())
		b.WriteByte('\n')
	}
	return b.String()
}

func MultiErr(errs []error) error {
	if len(errs) == 0 {
		return nil
	}
	return &MultiError{
		errs: errs,
	}
}

type SingleProfiler struct {
	*perf.Event
}

func NewSingleProfiler(attr *perf.Attr, pid, cpu int) (*SingleProfiler, error) {
	p, err := perf.Open(attr, pid, cpu, nil)
	return &SingleProfiler{
		Event: p,
	}, err
}

func (p *SingleProfiler) Metrics() Metrics {
	c, _ := p.ReadCount()
	if c.Enabled != c.Running {
		Logger.Printf("%s: multiplexing occured (enabled: %s, running %s)\n", c.Label, c.Enabled, c.Running)
	}
	return Metrics{
		results: []Result{
			Result{
				Value: uint64(float64(c.Value) * float64(c.Enabled) / float64(c.Running)),
				Label: c.Label,
			},
		},
		elapsed: c.Enabled,
	}
}

type MultiProfiler struct {
	profilers []Profiler
}

func NewMultiProfiler(attrs []*perf.Attr, pid, cpu int) (*MultiProfiler, error) {
	profs := make([]Profiler, len(attrs))
	var errs []error
	for i, attr := range attrs {
		p, err := NewSingleProfiler(attr, pid, cpu)
		if err != nil {
			errs = append(errs, err)
		}
		profs[i] = p
	}
	return &MultiProfiler{
		profilers: profs,
	}, MultiErr(errs)
}

func (p *MultiProfiler) Enable() error {
	var errs []error
	for _, prof := range p.profilers {
		err := prof.Enable()
		if err != nil {
			errs = append(errs, err)
		}
	}
	return MultiErr(errs)
}

func (p *MultiProfiler) Disable() error {
	var errs []error
	for _, prof := range p.profilers {
		err := prof.Disable()
		if err != nil {
			errs = append(errs, err)
		}
	}
	return MultiErr(errs)
}

func (p *MultiProfiler) Reset() error {
	var errs []error
	for _, prof := range p.profilers {
		err := prof.Reset()
		if err != nil {
			errs = append(errs, err)
		}
	}
	return MultiErr(errs)
}

func (p *MultiProfiler) Metrics() Metrics {
	results := make([]Result, 0, len(p.profilers))
	var elapsed time.Duration
	for _, prof := range p.profilers {
		metrics := prof.Metrics()
		results = append(results, metrics.results...)
		elapsed = metrics.elapsed
	}
	return Metrics{
		results: results,
		elapsed: elapsed,
	}
}

type GroupProfiler struct {
	*perf.Event
}

func NewGroupProfiler(attrs []*perf.Attr, pid, cpu int) (*GroupProfiler, error) {
	var g perf.Group
	for i, attr := range attrs {
		if i != 0 {
			attr.Options.Disabled = false
		}
		g.Add(attr)
	}
	hw, err := g.Open(pid, cpu)
	return &GroupProfiler{
		Event: hw,
	}, err
}

func (p *GroupProfiler) Metrics() Metrics {
	gc, _ := p.ReadGroupCount()

	if gc.Running == 0 {
		return Metrics{}
	}

	scale := float64(gc.Enabled) / float64(gc.Running)
	if gc.Enabled != gc.Running {
		Logger.Printf("%s: multiplexing occured (enabled: %s, running %s)\n", "group", gc.Enabled, gc.Running)
	}

	var results []Result
	for _, v := range gc.Values {
		results = append(results, Result{
			Value: uint64(float64(v.Value) * scale),
			Label: v.Label,
		})
	}
	return Metrics{
		results: results,
		elapsed: gc.Enabled,
	}
}
