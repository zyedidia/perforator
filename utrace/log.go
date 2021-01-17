package utrace

import (
	"io/ioutil"
	"log"
)

var logger *log.Logger

func init() {
	logger = log.New(ioutil.Discard, "", 0)
}

// SetLogger assigns the package-wide logger.
func SetLogger(l *log.Logger) {
	logger = l
}
