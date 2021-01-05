package main

import (
	"bytes"
	"fmt"
	"sort"
	"time"

	"github.com/olekukonko/tablewriter"
)

type Result struct {
	Label string
	Value uint64
}

type Metrics struct {
	results []Result
	elapsed time.Duration
}

func (m Metrics) String() string {
	buf := &bytes.Buffer{}
	table := tablewriter.NewWriter(buf)
	table.SetHeader([]string{"Event", "Count"})

	for _, r := range m.results {
		table.Append([]string{
			r.Label,
			fmt.Sprintf("%d", r.Value),
		})
	}
	table.Append([]string{
		"time-elapsed",
		fmt.Sprintf("%s", m.elapsed),
	})

	table.SetAutoFormatHeaders(false)
	table.SetHeaderAlignment(tablewriter.ALIGN_LEFT)
	table.SetAlignment(tablewriter.ALIGN_LEFT)
	table.Render()

	return buf.String()
}

type TotalMetrics map[string]Metrics

func (t TotalMetrics) String(sortKey string, reverse bool) string {
	buf := &bytes.Buffer{}
	table := tablewriter.NewWriter(buf)

	var sortIdx int
	header := []string{"region"}
	for _, m := range t {
		for i, result := range m.results {
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
	for k, v := range t {
		ss = append(ss, kv{k, v})
	}

	sort.Slice(ss, func(i, j int) bool {
		if sortKey == "time-elapsed" {
			vali := ss[i].Value.elapsed
			valj := ss[j].Value.elapsed
			if reverse {
				return vali < valj
			}
			return valj < vali
		}
		if reverse {
			return ss[i].Value.results[sortIdx].Value < ss[j].Value.results[sortIdx].Value
		}
		return ss[i].Value.results[sortIdx].Value > ss[j].Value.results[sortIdx].Value
	})

	for _, kv := range ss {
		row := []string{kv.Key}
		m := kv.Value
		for _, result := range m.results {
			row = append(row, fmt.Sprintf("%d", result.Value))
		}
		row = append(row, fmt.Sprintf("%s", m.elapsed))
		table.Append(row)
	}

	table.SetAutoFormatHeaders(false)
	table.SetHeaderAlignment(tablewriter.ALIGN_LEFT)
	table.SetAlignment(tablewriter.ALIGN_LEFT)
	table.Render()

	return buf.String()
}
