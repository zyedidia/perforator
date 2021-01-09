package perforator

import (
	"io/ioutil"
	"log"
)

var (
	Logger *log.Logger
)

func init() {
	Logger = log.New(ioutil.Discard, "", 0)
}

func SetLogger(l *log.Logger) {
	Logger = l
}
