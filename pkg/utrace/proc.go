package utrace

import (
	"errors"
	"os"
	"os/exec"

	"github.com/zyedidia/utrace/bininfo"
	"github.com/zyedidia/utrace/ptrace"
	"golang.org/x/sys/unix"
)

var (
	interrupt = []byte{0xCC}

	ErrInvalidBreakpoint = errors.New("Invalid breakpoint")
)

type Proc struct {
	tracer    *ptrace.Tracer
	regions   []activeRegion
	pieOffset uint64

	breakpoints map[uintptr][]byte
}

func StartProc(bin *bininfo.BinFile, target string, args []string, regions []Region) (*Proc, error) {
	cmd := exec.Command(target, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	cmd.SysProcAttr = &unix.SysProcAttr{
		Ptrace: true,
	}

	err := cmd.Start()
	if err != nil {
		return nil, err
	}
	// wait for execve
	cmd.Wait()

	p, err := NewTracedProc(cmd.Process.Pid, bin, regions)
	return p, err
}

func NewTracedProc(pid int, bin *bininfo.BinFile, regions []Region) (*Proc, error) {
	off, err := bin.PieOffset(pid)
	if err != nil {
		return nil, err
	}

	p := &Proc{
		tracer:      ptrace.NewTracer(pid),
		regions:     make([]activeRegion, 0, len(regions)),
		pieOffset:   off,
		breakpoints: make(map[uintptr][]byte),
	}

	// TODO: more options
	p.tracer.SetOptions(unix.PTRACE_O_EXITKILL)

	for id, r := range regions {
		err := p.setBreak(r.Start())
		if err != nil {
			return nil, err
		}

		p.regions = append(p.regions, activeRegion{
			region:       r,
			state:        RegionStart,
			curInterrupt: r.Start(),
			id:           id,
		})
	}

	return p, p.Continue(0)
}

func (p *Proc) setBreak(pc uint64) error {
	var err error
	pcptr := uintptr(pc + p.pieOffset)

	if _, ok := p.breakpoints[pcptr]; ok {
		// breakpoint already exists
		return nil
	}

	orig := make([]byte, len(interrupt))
	_, err = p.tracer.PeekData(pcptr, orig)
	if err != nil {
		return err
	}
	_, err = p.tracer.PokeData(pcptr, interrupt)
	if err != nil {
		return err
	}

	p.breakpoints[pcptr] = orig
	return nil
}

func (p *Proc) removeBreak(pc uint64) error {
	pcptr := uintptr(pc + p.pieOffset)
	orig, ok := p.breakpoints[pcptr]
	if !ok {
		return ErrInvalidBreakpoint
	}
	_, err := p.tracer.PokeData(pcptr, orig)
	delete(p.breakpoints, pcptr)
	return err
}

type Event struct {
	Id    int
	State RegionState
}

func (p *Proc) handleInterrupt() ([]Event, error) {
	var regs unix.PtraceRegs
	p.tracer.GetRegs(&regs)
	regs.Rip -= uint64(len(interrupt))
	p.tracer.SetRegs(&regs)

	err := p.removeBreak(regs.Rip - p.pieOffset)
	if err != nil {
		return nil, err
	}

	events := make([]Event, 0)
	for i, r := range p.regions {
		var err error
		if r.curInterrupt+p.pieOffset == regs.Rip {
			events = append(events, Event{
				Id:    r.id,
				State: r.state,
			})
			switch r.state {
			case RegionStart:
				p.regions[i].state = RegionEnd
				var addr uint64
				addr, err = r.region.End(regs.Rsp, p.tracer)
				if err != nil {
					return nil, err
				}
				addr -= p.pieOffset
				p.regions[i].curInterrupt = addr
				err = p.setBreak(addr)
			case RegionEnd:
				p.regions[i].state = RegionStart
				p.regions[i].curInterrupt = r.region.Start()
				err = p.setBreak(p.regions[i].curInterrupt)
			default:
				return nil, errors.New("invalid state")
			}
		}
		if err != nil {
			return nil, err
		}
	}

	return events, nil
}

func (p *Proc) Continue(sig unix.Signal) error {
	return p.tracer.Cont(sig)
}

func (p *Proc) Pid() int {
	return p.tracer.Pid()
}
