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

// A CSVWriter is a MetricsWriter that outputs the information in CSV format.
type CSVWriter struct {
	*csv.Writer
}

// NewCSVWriter creates a CSVWriter that writes to the given output writer.
func NewCSVWriter(w io.Writer) *CSVWriter {
	return &CSVWriter{
		Writer: csv.NewWriter(w),
	}
}

// SetHeader adds a table header.
func (c *CSVWriter) SetHeader(headers []string) {
	c.Writer.Write(headers)
}

// Append creates a new row in the table.
func (c *CSVWriter) Append(record []string) {
	c.Writer.Write(record)
}

// Render flushes the table content to the writer.
func (c *CSVWriter) Render() {
	c.Writer.Flush()
}

// NewTableWriter creates a MetricsWriter that writes a pretty-printed ASCII
// table.
func NewTableWriter(w io.Writer) *tablewriter.Table {
	t := tablewriter.NewWriter(w)
	t.SetAutoFormatHeaders(false)
	t.SetHeaderAlignment(tablewriter.ALIGN_LEFT)
	t.SetAlignment(tablewriter.ALIGN_LEFT)
	return t
}
