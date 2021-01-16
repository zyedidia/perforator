package perforator

import (
	"fmt"
	"io"
	"sort"
	"time"
)

// A Result represents a single event, marked by Label, and the counter value
// returned by the perf monitor.
type Result struct {
	Label string
	Value uint64
}

// Metrics stores a set of results and the time elapsed while they were
// profiling.
type Metrics struct {
	Results []Result
	Elapsed time.Duration
}

func (m Metrics) WriteTo(w io.Writer, csv bool) {
	var table MetricsWriter
	if csv {
		table = NewCSVWriter(w)
	} else {
		table = NewTableWriter(w)
	}
	table.SetHeader([]string{"Event", "Count"})

	for _, r := range m.Results {
		table.Append([]string{
			r.Label,
			fmt.Sprintf("%d", r.Value),
		})
	}
	table.Append([]string{
		"time-elapsed",
		fmt.Sprintf("%s", m.Elapsed),
	})

	table.Render()
}

// NamedMetrics associates a metrics structure with a name. This is useful for
// associated metrics structures with regions.
type NamedMetrics struct {
	Metrics
	Name string
}

// TotalMetrics is a list of metrics and the region they are associated with.
type TotalMetrics []NamedMetrics

// WriteTo pretty-prints the metrics and writes the result to a writer. By
// default, an ASCII table will be created, but if csv is true then the metrics
// table will be written in CSV format. The sortKey and reverse parameters
// configure the table arrangement: which entry to sort by and whether the sort
// should be in reverse order.
func (t TotalMetrics) WriteTo(w io.Writer, csv bool, sortKey string, reverse bool) {
	var table MetricsWriter
	if csv {
		table = NewCSVWriter(w)
	} else {
		table = NewTableWriter(w)
	}

	var sortIdx int
	header := []string{"region"}
	for _, m := range t {
		for i, result := range m.Results {
			if result.Label == sortKey {
				sortIdx = i
			}
			header = append(header, result.Label)
		}
		break
	}
	header = append(header, "time-elapsed")

	table.SetHeader(header)

	type kv struct {
		Key   string
		Value Metrics
	}

	var ss []kv
	for _, v := range t {
		ss = append(ss, kv{v.Name, v.Metrics})
	}

	sort.Slice(ss, func(i, j int) bool {
		if sortKey == "time-elapsed" {
			vali := ss[i].Value.Elapsed
			valj := ss[j].Value.Elapsed
			if reverse {
				return vali < valj
			}
			return valj < vali
		}
		if reverse {
			return ss[i].Value.Results[sortIdx].Value < ss[j].Value.Results[sortIdx].Value
		}
		return ss[i].Value.Results[sortIdx].Value > ss[j].Value.Results[sortIdx].Value
	})

	for _, kv := range ss {
		row := []string{kv.Key}
		m := kv.Value
		for _, result := range m.Results {
			row = append(row, fmt.Sprintf("%d", result.Value))
		}
		row = append(row, fmt.Sprintf("%s", m.Elapsed))
		table.Append(row)
	}

	table.Render()
}
