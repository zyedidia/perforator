package perforator

import (
	"encoding/csv"
	"io"

	"github.com/olekukonko/tablewriter"
)

// A MetricsWriter is an interface for writing tables.
type MetricsWriter interface {
	SetHeader(headers []string)
	Append(record []string)
	Render()
}

type CSVWriter struct {
	*csv.Writer
}

func NewCSVWriter(w io.Writer) *CSVWriter {
	return &CSVWriter{
		Writer: csv.NewWriter(w),
	}
}

func (c *CSVWriter) SetHeader(headers []string) {
	c.Writer.Write(headers)
}

func (c *CSVWriter) Append(record []string) {
	c.Writer.Write(record)
}

func (c *CSVWriter) Render() {
	c.Writer.Flush()
}

func NewTableWriter(w io.Writer) *tablewriter.Table {
	t := tablewriter.NewWriter(w)
	t.SetAutoFormatHeaders(false)
	t.SetHeaderAlignment(tablewriter.ALIGN_LEFT)
	t.SetAlignment(tablewriter.ALIGN_LEFT)
	return t
}
