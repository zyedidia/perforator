package main

import (
	"bytes"
	"fmt"
	"io"
	"time"

	"acln.ro/perf"
	"github.com/olekukonko/tablewriter"
)

type Profiler interface {
	Enable() error
	Disable() error
	Reset() error
	WriteMetrics(w io.Writer) error
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

type MultiProfiler struct {
	profilers []*perf.Event
}

func NewMultiProfiler(attrs []*perf.Attr, pid, cpu int) (*MultiProfiler, error) {
	profs := make([]*perf.Event, len(attrs))
	var errs []error
	for i, attr := range attrs {
		p, err := perf.Open(attr, pid, cpu, nil)
		if err != nil {
			errs = append(errs, err)
		}
		profs[i] = p
	}
	if len(errs) > 0 {
		return nil, &MultiError{
			errs: errs,
		}
	}
	return &MultiProfiler{
		profilers: profs,
	}, nil
}

func (p *MultiProfiler) Enable() error {
	var errs []error
	for _, prof := range p.profilers {
		err := prof.Enable()
		if err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return &MultiError{
			errs: errs,
		}
	}
	return nil
}

func (p *MultiProfiler) Disable() error {
	var errs []error
	for _, prof := range p.profilers {
		err := prof.Disable()
		if err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return &MultiError{
			errs: errs,
		}
	}
	return nil
}

func (p *MultiProfiler) Reset() error {
	var errs []error
	for _, prof := range p.profilers {
		err := prof.Reset()
		if err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return &MultiError{
			errs: errs,
		}
	}
	return nil
}

func (p *MultiProfiler) WriteMetrics(w io.Writer) error {
	results := make([]struct {
		val   uint64
		label string
	}, len(p.profilers))

	var enabled time.Duration
	for i, prof := range p.profilers {
		c, _ := prof.ReadCount()
		scale := uint64(c.Enabled) / uint64(c.Running)
		results[i].val = c.Value * scale
		results[i].label = c.Label

		enabled = c.Enabled
	}

	table := tablewriter.NewWriter(w)
	table.SetHeader([]string{"Event", "Count"})

	for _, v := range results {
		table.Append([]string{
			v.label,
			fmt.Sprintf("%d", v.val),
		})
	}
	table.Append([]string{
		"time elapsed",
		fmt.Sprintf("%s", enabled),
	})
	table.SetHeaderAlignment(tablewriter.ALIGN_LEFT)
	table.SetAlignment(tablewriter.ALIGN_LEFT)
	table.Render()
	return nil
}

type GroupProfiler struct {
	profiler *perf.Event
}

func NewGroupProfiler(attrs []*perf.Attr, pid, cpu int) (*GroupProfiler, error) {
	var g perf.Group
	for _, attr := range attrs {
		g.Add(attr)
	}
	hw, err := g.Open(pid, cpu)
	return &GroupProfiler{
		profiler: hw,
	}, err
}

func (p *GroupProfiler) Enable() error {
	return p.profiler.Enable()
}

func (p *GroupProfiler) Disable() error {
	return p.profiler.Disable()
}

func (p *GroupProfiler) Reset() error {
	return p.profiler.Reset()
}

func (p *GroupProfiler) WriteMetrics(w io.Writer) error {
	gc, err := p.profiler.ReadGroupCount()
	if err != nil {
		return err
	}

	table := tablewriter.NewWriter(w)
	table.SetHeader([]string{"Event", "Count"})

	scale := uint64(gc.Enabled) / uint64(gc.Running)
	for _, v := range gc.Values {
		table.Append([]string{
			v.Label,
			fmt.Sprintf("%d", v.Value*scale),
		})
	}
	table.Append([]string{
		"time elapsed",
		fmt.Sprintf("%s", gc.Enabled),
	})
	table.SetHeaderAlignment(tablewriter.ALIGN_LEFT)
	table.SetAlignment(tablewriter.ALIGN_LEFT)
	table.Render()
	return nil
}
