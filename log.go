package main

import (
	"log"
)

var (
	Logger *log.Logger
)

// NullWriter simply sends writes into the void
type NullWriter struct{}

// Write is empty
func (NullWriter) Write(data []byte) (n int, err error) {
	return 0, nil
}

func init() {
	Logger = log.New(NullWriter{}, "INFO: ", 0)
}
