package utrace

import (
	"log"

	"github.com/zyedidia/utrace/bininfo"
	"golang.org/x/sys/unix"
)

type Status struct {
	unix.WaitStatus
	sig unix.Signal
}

type Program struct {
	procs map[int]*Proc
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
	return prog, proc.Pid(), nil
}

func (p *Program) Wait(status *Status) (*Proc, []Event, error) {
	ws := &status.WaitStatus
	sig := &status.sig

	wpid, err := unix.Wait4(-1, ws, 0, nil)
	if err != nil {
		return nil, nil, err
	}

	*sig = 0
	proc, ok := p.procs[wpid]
	if !ok {
		// TODO: multithreading
		log.Fatal("Invalid pid")
	}

	if ws.Exited() {
		Logger.Printf("%d: exited\n", wpid)
		delete(p.procs, wpid)
	} else if !ws.Stopped() {
		return proc, nil, nil
	} else if ws.StopSignal() != unix.SIGTRAP {
		Logger.Printf("%d: received signal '%s'\n", wpid, ws.StopSignal())
		*sig = ws.StopSignal()
	} else if ws.TrapCause() == unix.PTRACE_EVENT_CLONE {
		Logger.Printf("%d: called clone()\n", wpid)
		// TODO: multithreading
	} else if ws.TrapCause() == unix.PTRACE_EVENT_EXEC {
		Logger.Printf("%d: called exec()\n", wpid)
		// TODO: multithreading
	} else {
		events, err := proc.handleInterrupt()
		if err != nil {
			return nil, nil, err
		}
		Logger.Printf("%d: %d region events occurred\n", wpid, len(events))
		return proc, events, nil
	}
	return proc, nil, nil
}

func (p *Program) Continue(pr *Proc, status Status) error {
	return pr.Continue(status.sig)
}
