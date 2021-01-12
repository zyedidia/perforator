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

// A SingleProfiler profiles one event
type SingleProfiler struct {
	*perf.Event
	// perf tracks "enabled time" but does not reset it when "reset" is called
	// so whenever there is a reset we manually track the time enabled so far
	// so that we can subtract it from the total
	enabled time.Duration
}

func NewSingleProfiler(attr *perf.Attr, pid, cpu int) (*SingleProfiler, error) {
	p, err := perf.Open(attr, pid, cpu, nil)
	return &SingleProfiler{
		Event: p,
	}, err
}

func (p *SingleProfiler) Reset() error {
	c, err := p.ReadCount()
	if err != nil {
		return err
	}
	p.enabled = c.Enabled
	return p.Event.Reset()
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
		elapsed: c.Enabled - p.enabled,
	}
}

// A MultiProfiler runs multiple profilers, each of which may profile for
// groups of events.
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

// A GroupProfiler profiles a set of events as one group so that the events
// cannot be multiplexed with respect to each other.
type GroupProfiler struct {
	*perf.Event
	enabled time.Duration
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

func (p *GroupProfiler) Reset() error {
	gc, err := p.ReadGroupCount()
	if err != nil {
		return err
	}
	p.enabled = gc.Enabled
	return p.Event.Reset()
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
		elapsed: gc.Enabled - p.enabled,
	}
}
