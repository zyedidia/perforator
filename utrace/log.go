package utrace

import (
	"io/ioutil"
	"log"
)

var logger *log.Logger

func init() {
	logger = log.New(ioutil.Discard, "", 0)
}

func SetLogger(l *log.Logger) {
	logger = l
}
