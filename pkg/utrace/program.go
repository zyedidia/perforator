package utrace

import (
	"errors"

	"github.com/zyedidia/utrace/bininfo"
	"golang.org/x/sys/unix"
)

var ErrFinishedTrace = errors.New("tracing finished")

type Status struct {
	unix.WaitStatus

	sig       unix.Signal
	groupStop bool
}

type Program struct {
	procs    map[int]*Proc
	untraced map[int]*Proc

	regions []Region
	bin     *bininfo.BinFile
}

func NewProgram(bin *bininfo.BinFile, target string, args []string, regions []Region) (*Program, int, error) {
	proc, err := StartProc(bin, target, args, regions)
	if err != nil {
		return nil, 0, err
	}

	prog := new(Program)
	prog.procs = map[int]*Proc{
		proc.Pid(): proc,
	}
	prog.regions = regions
	prog.bin = bin

	return prog, proc.Pid(), err
}

func (p *Program) Wait(status *Status) (*Proc, []Event, error) {
	ws := &status.WaitStatus
	sig := &status.sig

	wpid, err := unix.Wait4(-1, ws, 0, nil)
	if err != nil {
		return nil, nil, err
	}

	*sig = 0
	status.groupStop = false
	untraced := false

	proc, ok := p.procs[wpid]
	if !ok {
		proc, ok = p.untraced[wpid]
		if ok {
			untraced = true
		} else {
			proc, err = NewTracedProc(wpid, p.bin, p.regions)
			if err != nil {
				return nil, nil, err
			}
			p.procs[wpid] = proc
			Logger.Printf("%d: new process created (tracing enabled)\n", wpid)
			Logger.Printf("%d: %s\n", wpid, ws.StopSignal())
			return proc, nil, nil
		}
	}

	if ws.Exited() || ws.Signaled() {
		Logger.Printf("%d: exited\n", wpid)
		delete(p.procs, wpid)

		if len(p.procs) == 0 {
			return proc, nil, ErrFinishedTrace
		}
	} else if !ws.Stopped() {
		return proc, nil, nil
	} else if ws.StopSignal() != unix.SIGTRAP {
		if statusPtraceEventStop(*ws) {
			status.groupStop = true
			Logger.Printf("%d: received group stop\n", wpid)
		} else {
			Logger.Printf("%d: received signal '%s'\n", wpid, ws.StopSignal())
			*sig = ws.StopSignal()
		}
	} else if ws.TrapCause() == unix.PTRACE_EVENT_CLONE {
		Logger.Printf("%d: called clone()\n", wpid)
	} else if ws.TrapCause() == unix.PTRACE_EVENT_FORK {
		Logger.Printf("%d: called fork()\n", wpid)
	} else if ws.TrapCause() == unix.PTRACE_EVENT_VFORK {
		Logger.Printf("%d: called vfork()\n", wpid)
	} else if ws.TrapCause() == unix.PTRACE_EVENT_EXEC {
		Logger.Printf("%d: called exec() (tracing disabled)\n", wpid)
		delete(p.procs, wpid)
		p.untraced[wpid] = proc
	} else if !untraced {
		events, err := proc.handleInterrupt()
		if err != nil {
			return nil, nil, err
		}
		return proc, events, nil
	}
	return proc, nil, nil
}

func (p *Program) Continue(pr *Proc, status Status) error {
	return pr.Continue(status.sig, status.groupStop)
}

func statusPtraceEventStop(status unix.WaitStatus) bool {
	return int(status)>>16 == unix.PTRACE_EVENT_STOP
}
