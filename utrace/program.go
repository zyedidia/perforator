// Package utrace provides an interface for tracing user-level code with
// ptrace. The implementation transparently places and removes software
// breakpoints to regain control from a traced program. Multithreaded programs
// are supported with the caveat that each region may only be executed once
// (ever) by a certain thread. In other words, it is best to only use utrace
// for tracing single threaded programs, but if the program is multithreaded it
// will still work as long as only one of the threads executes the region of
// interest.
//
// NOTE: make sure runtime.LockOSThread() has been called before using any of
// the following functions, and may not unlock the thread until you are
// finished calling any trace functions.
package utrace

import (
	"errors"
	"os"

	"golang.org/x/sys/unix"
)

var ErrFinishedTrace = errors.New("tracing finished")

// Status represents a return status from a call to Wait.
type Status struct {
	unix.WaitStatus

	sig       unix.Signal
	groupStop bool
}

// A Program is a collection of running processes that are being traced.
// Threads or processes that are executing the same code as the original parent
// will be traced, but if they ever call execve, they will no longer be traced.
// The Program struct makes it simpler to support multiple threads in the child
// since it will handle the nitty gritty of tracing each thread.
type Program struct {
	procs    map[int]*Proc
	untraced map[int]*Proc

	regions     []Region
	pie         PieOffsetter
	breakpoints map[uintptr][]byte
}

// NewProgram returns a new running program created from the given elf binary
// file and instantiation command 'target args...'. The list of regions
// specifies which regions in the target to track. When Wait is called, it will
// block until the target process or one of its threads/children begins or
// finishes executing a region.
// Also returns the PID of the newly created process.
func NewProgram(pie PieOffsetter, target string, args []string, regions []Region) (*Program, int, error) {
	proc, err := startProc(pie, target, args, regions)
	if err != nil {
		return nil, 0, err
	}

	return newProgFromProc(proc, pie, regions)
}

func NewAttachedProgram(pid int, pie PieOffsetter, regions []Region) (*Program, int, error) {
	proc, err := attachProc(pid, pie, regions)
	if err != nil {
		return nil, 0, err
	}

	return newProgFromProc(proc, pie, regions)
}

func newProgFromProc(proc *Proc, pie PieOffsetter, regions []Region) (*Program, int, error) {
	prog := new(Program)
	prog.procs = map[int]*Proc{
		proc.Pid(): proc,
	}
	prog.untraced = make(map[int]*Proc)
	prog.regions = regions
	prog.pie = pie
	prog.breakpoints = make(map[uintptr][]byte)
	for k, v := range proc.breakpoints {
		prog.breakpoints[k] = make([]byte, len(v))
		copy(prog.breakpoints[k], v)
	}

	return prog, proc.Pid(), nil
}

type waitResult struct {
	pid int
	ws  unix.WaitStatus
	err error
}

// Wait blocks until a thread/child process enters or exits a region. The wait
// status will be placed in the 'status' variable. The affected process will be
// returned. Since multiple regions may be affected (if two regions end on the
// same address), a list of events is returned indicating which regions were
// affected and whether they have been entered or exited by the process.
func (p *Program) Wait(status *Status, interrupt <-chan os.Signal) (*Proc, []Event, error) {
	wait := make(chan waitResult)
	go func() {
		var ws unix.WaitStatus
		pid, err := unix.Wait4(-1, &ws, 0, nil)
		wait <- waitResult{
			pid: pid,
			ws:  ws,
			err: err,
		}
	}()

	var wpid int
	select {
	case result := <-wait:
		if result.err != nil {
			return nil, nil, result.err
		}
		status.WaitStatus = result.ws
		wpid = result.pid
	case <-interrupt:
		p.detach(wait)
		return nil, nil, ErrFinishedTrace
	}
	ws := &status.WaitStatus

	status.sig = 0
	status.groupStop = false
	untraced := false
	proc, ok := p.procs[wpid]
	if !ok {
		proc, untraced = p.untraced[wpid]
		if !untraced {
			var err error
			proc, err = newTracedProc(wpid, p.pie, p.regions, p.breakpoints)
			if err != nil {
				return nil, nil, err
			}
			p.procs[wpid] = proc
			logger.Printf("%d: new process created (tracing enabled)\n", wpid)
			return proc, nil, nil
		}
	}

	if ws.Exited() || ws.Signaled() {
		logger.Printf("%d: exited\n", wpid)
		delete(p.procs, wpid)
		proc.exit()

		if len(p.procs) == 0 {
			return proc, nil, ErrFinishedTrace
		}
	} else if !ws.Stopped() {
		return proc, nil, nil
	} else if ws.StopSignal() != unix.SIGTRAP {
		if statusPtraceEventStop(*ws) {
			status.groupStop = true
			logger.Printf("%d: received group stop\n", wpid)
		} else {
			logger.Printf("%d: received signal '%s'\n", wpid, ws.StopSignal())
			status.sig = ws.StopSignal()
		}
	} else if ws.TrapCause() == unix.PTRACE_EVENT_CLONE {
		newpid, err := proc.tracer.GetEventMsg()
		logger.Printf("%d: called clone() = %d (err=%v)\n", wpid, newpid, err)
	} else if ws.TrapCause() == unix.PTRACE_EVENT_FORK {
		logger.Printf("%d: called fork()\n", wpid)
	} else if ws.TrapCause() == unix.PTRACE_EVENT_VFORK {
		logger.Printf("%d: called vfork()\n", wpid)
	} else if ws.TrapCause() == unix.PTRACE_EVENT_EXEC {
		logger.Printf("%d: called exec() (tracing disabled)\n", wpid)
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

// Continue resumes execution of the given process. The wait status must be
// passed to replay any signals that were received while waiting.
func (p *Program) Continue(pr *Proc, status Status) error {
	return pr.cont(status.sig, status.groupStop)
}

// Detach removes breakpoints and stops tracing all processes in this program.
func (p *Program) detach(wait chan waitResult) {
	for pid, proc := range p.procs {
		proc.tracer.Interrupt()
		<-wait
		proc.clearBreaks()
		proc.tracer.Detach()
		delete(p.procs, pid)
	}
}

func statusPtraceEventStop(status unix.WaitStatus) bool {
	return int(status)>>16 == unix.PTRACE_EVENT_STOP
}
