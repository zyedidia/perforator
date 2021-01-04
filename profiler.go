package main

import (
	"bytes"
	"time"

	"acln.ro/perf"
)

type Result struct {
	Label string
	Value uint64
}

type Profiler interface {
	Enable() error
	Disable() error
	Reset() error
	Metrics() ([]Result, time.Duration)
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

func (p *SingleProfiler) Metrics() ([]Result, time.Duration) {
	c, _ := p.ReadCount()
	return []Result{
		Result{
			Value: c.Value * uint64(c.Enabled) / uint64(c.Running),
			Label: c.Label,
		},
	}, c.Enabled
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

func (p *MultiProfiler) Metrics() ([]Result, time.Duration) {
	results := make([]Result, 0, len(p.profilers))
	var elapsed time.Duration
	for _, prof := range p.profilers {
		metrics, enabled := prof.Metrics()
		results = append(results, metrics...)
		elapsed = enabled
	}
	return results, elapsed
}

type GroupProfiler struct {
	*perf.Event
}

func NewGroupProfiler(attrs []*perf.Attr, pid, cpu int) (*GroupProfiler, error) {
	var g perf.Group
	for _, attr := range attrs {
		g.Add(attr)
	}
	hw, err := g.Open(pid, cpu)
	return &GroupProfiler{
		Event: hw,
	}, err
}

func (p *GroupProfiler) Metrics() ([]Result, time.Duration) {
	gc, _ := p.ReadGroupCount()
	scale := uint64(gc.Enabled) / uint64(gc.Running)
	var results []Result
	for _, v := range gc.Values {
		results = append(results, Result{
			Value: v.Value * scale,
			Label: v.Label,
		})
	}
	return results, gc.Enabled
}
