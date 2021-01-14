// Package utrace provides an interface for tracing user-level code with
// ptrace. The implementation transparently places and removes software
// breakpoints to regain control from a traced program. Multithreaded programs
// are supported with the caveat that each region may only be executed once
// (ever) by a certain thread. In other words, it is best to only use utrace
// for tracing single threaded programs, but if the program is multithreaded it
// will still work as long as only one of the threads executes the region of
// interest.
//
// NOTE: you make sure runtime.LockOSThread() has been called before using any
// of the following functions, and may not unlock the thread until you are
// finished calling any trace functions.
package utrace

import (
	"errors"

	"github.com/zyedidia/perforator/pkg/utrace/bininfo"
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
	bin         *bininfo.BinFile
	breakpoints map[uintptr][]byte
}

// NewProgram returns a new running program created from the given elf binary
// file and instantiation command 'target args...'. The list of regions
// specifies which regions in the target to track. When Wait is called, it will
// block until the target process or one of its threads/children begins or
// finishes executing a region.
func NewProgram(bin *bininfo.BinFile, target string, args []string, regions []Region) (*Program, int, error) {
	proc, err := startProc(bin, target, args, regions)
	if err != nil {
		return nil, 0, err
	}

	prog := new(Program)
	prog.procs = map[int]*Proc{
		proc.Pid(): proc,
	}
	prog.regions = regions
	prog.bin = bin
	prog.breakpoints = make(map[uintptr][]byte)
	for k, v := range proc.breakpoints {
		prog.breakpoints[k] = make([]byte, len(v))
		copy(prog.breakpoints[k], v)
	}

	return prog, proc.Pid(), err
}

// Wait blocks until a thread/child process enters or exits a region. The wait
// status will be placed in the 'status' variable. The affected process will be
// returned. Since multiple regions may be affected (if two regions end on the
// same address), a list of events is returned indicating which regions were
// affected and whether they have been entered or exited by the process.
func (p *Program) Wait(status *Status) (*Proc, []Event, error) {
	ws := &status.WaitStatus

	wpid, err := unix.Wait4(-1, ws, 0, nil)
	if err != nil {
		return nil, nil, err
	}

	status.sig = 0
	status.groupStop = false
	untraced := false
	proc, ok := p.procs[wpid]
	if !ok {
		proc, untraced = p.untraced[wpid]
		if !untraced {
			proc, err = newTracedProc(wpid, p.bin, p.regions, p.breakpoints)
			if err != nil {
				return nil, nil, err
			}
			p.procs[wpid] = proc
			Logger.Printf("%d: new process created (tracing enabled)\n", wpid)
			return proc, nil, nil
		}
	}

	if ws.Exited() || ws.Signaled() {
		Logger.Printf("%d: exited\n", wpid)
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
			Logger.Printf("%d: received group stop\n", wpid)
		} else {
			Logger.Printf("%d: received signal '%s'\n", wpid, ws.StopSignal())
			status.sig = ws.StopSignal()
		}
	} else if ws.TrapCause() == unix.PTRACE_EVENT_CLONE {
		newpid, err := proc.tracer.GetEventMsg()
		Logger.Printf("%d: called clone() = %d (err=%v)\n", wpid, newpid, err)
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

// Continue resumes execution of the given process. The wait status must be
// passed to replay any signals that were received while waiting.
func (p *Program) Continue(pr *Proc, status Status) error {
	return pr.cont(status.sig, status.groupStop)
}

func statusPtraceEventStop(status unix.WaitStatus) bool {
	return int(status)>>16 == unix.PTRACE_EVENT_STOP
}
