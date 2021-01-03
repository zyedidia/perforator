package main

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"strconv"

	"github.com/hodgesds/perf-utils"
	"github.com/zyedidia/perforator/ptrace"
	"golang.org/x/sys/unix"
)

var (
	interrupt = []byte{0xCC}

	ErrInvalidBreakpoint = errors.New("Invalid breakpoint")
	ErrInvalidState      = errors.New("Invalid state")
)

type ProcState int

const (
	PStart ProcState = iota
	PEnd
)

type Proc struct {
	prof      *ProfRegion
	tracer    *ptrace.Tracer
	state     ProcState
	aslr_base uintptr

	breakpoints map[uintptr][]byte
}

func NewTracedProc(pid int, prof *ProfRegion, has_aslr bool) *Proc {
	var err error
	prof.profiler, err = perf.NewGroupProfiler(pid, -1, 0, prof.attrs...)
	must(err)
	p := &Proc{
		prof:        prof,
		tracer:      ptrace.NewTracer(pid),
		state:       PStart,
		breakpoints: make(map[uintptr][]byte),
	}
	if has_aslr {
		data, err := ioutil.ReadFile(fmt.Sprintf("/proc/%d/maps", pid))
		must(err)
		aslr, err := strconv.ParseInt(fmt.Sprintf("0x%s", string(bytes.Split(data, []byte("-"))[0])), 0, 64)
		must(err)
		p.aslr_base = uintptr(aslr)
	}

	// TODO: multithreading support
	p.tracer.SetOptions(unix.PTRACE_O_EXITKILL)
	must(p.setBreak(p.aslr_base + prof.block.Start()))
	return p
}

func StartProc(target string, args []string, prof *ProfRegion, has_aslr bool) (*Proc, error) {
	cmd := exec.Command(target, args...)
	cmd.Stdout = os.Stdout
	cmd.Stdin = os.Stdin
	cmd.Stderr = os.Stderr
	cmd.SysProcAttr = &unix.SysProcAttr{
		Ptrace: true,
	}

	err := cmd.Start()
	if err != nil {
		return nil, err
	}
	cmd.Wait()

	p := NewTracedProc(cmd.Process.Pid, prof, has_aslr)
	return p, nil
}

func (p *Proc) setBreak(pc uintptr) error {
	var err error

	orig := make([]byte, len(interrupt))
	_, err = p.tracer.PeekData(pc, orig)
	if err != nil {
		return err
	}
	_, err = p.tracer.PokeData(pc, interrupt)
	if err != nil {
		return err
	}

	p.breakpoints[pc] = orig
	return nil
}

func (p *Proc) removeBreak(pc uintptr) error {
	orig, ok := p.breakpoints[pc]
	if !ok {
		return ErrInvalidBreakpoint
	}
	_, err := p.tracer.PokeData(pc, orig)
	delete(p.breakpoints, pc)
	return err
}

func (p *Proc) HandleInterrupt() error {
	var regs unix.PtraceRegs
	// TODO: check errors
	p.tracer.GetRegs(&regs)
	regs.Rip -= 1
	p.tracer.SetRegs(&regs)

	switch p.state {
	case PStart:
		log.Printf("interrupt at 0x%x: profile starting", regs.Rip)
		must(p.removeBreak(uintptr(regs.Rip)))
		must(p.setBreak(p.prof.block.End(regs.Rsp, p.tracer)))
		p.state = PEnd
		p.prof.profiler.Reset()
		p.prof.profiler.Start()
	case PEnd:
		log.Printf("interrupt at 0x%x: profile finished", regs.Rip)
		p.prof.profiler.Stop()
		must(p.removeBreak(uintptr(regs.Rip)))
		must(p.setBreak(p.aslr_base + p.prof.block.Start()))
		p.state = PStart

		result, err := p.prof.profiler.Profile()
		if err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
		}
		p.prof.callback(p.prof, result)
	default:
		return ErrInvalidState
	}
	return nil
}

func (p *Proc) Continue(sig unix.Signal) error {
	return p.tracer.Cont(sig)
}
