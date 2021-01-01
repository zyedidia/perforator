package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"runtime"

	"github.com/hodgesds/perf-utils"
	"github.com/zyedidia/perforator/bininfo"
	"golang.org/x/sys/unix"
)

func fatal(a ...interface{}) {
	fmt.Fprintln(os.Stderr, a...)
	os.Exit(1)
}

func must(err error) {
	if err != nil {
		fatal(err)
	}
}

type Funcs map[string]uintptr

func init() {
	runtime.LockOSThread()
}

var fn = flag.String("fn", "", "function to profile")
var verbose = flag.Bool("V", false, "verbose output")

func main() {
	flag.Parse()

	if len(flag.Args()) <= 0 {
		fatal("no command given")
	}

	if *fn == "" {
		fatal("no function given")
	}

	target := flag.Args()[0]
	args := flag.Args()[1:]

	data, err := ioutil.ReadFile(target)
	if err != nil {
		log.Fatal(err)
	}

	bin, err := bininfo.OpenBinFile(data)
	if err != nil {
		log.Fatal(err)
	}

	var fnaddr uintptr
	fns := bin.FuzzyFunc(*fn)
	if len(fns) == 0 {
		fatal("function", *fn, "not found.")
	} else if len(fns) == 1 {
		fnaddr, _ = bin.FuncToPC(fns[0])
	} else {
		fmt.Println("Multiple functions matched, please pick one:")
		for _, fn := range fns {
			fmt.Println(fn)
		}
		os.Exit(0)
	}

	fmt.Printf("%s: 0x%x\n", *fn, fnaddr)
	proc, err := StartProc(target, args, &ProfRegion{
		block: &FuncBlock{
			addr: fnaddr,
		},
		attrs: []unix.PerfEventAttr{
			perf.CPUInstructionsEventAttr(),
		},
	})
	if err != nil {
		log.Fatal(err)
	}

	proc.Continue(0)

	var ws unix.WaitStatus
	for {
		_, err := unix.Wait4(-1, &ws, 0, nil)
		must(err)

		var sig unix.Signal
		if ws.Exited() {
			break
		} else if ws.Stopped() && ws.StopSignal() != unix.SIGTRAP {
			sig = ws.StopSignal()
		} else {
			proc.HandleInterrupt()
		}

		proc.Continue(sig)
	}
}
