package ptrace

import (
	"golang.org/x/sys/unix"
)

const (
	PTRACE_LISTEN = 16904
	PTRACE_SEIZE  = 16902
)

type Tracer struct {
	pid int
}

func NewTracer(pid int) *Tracer {
	return &Tracer{
		pid: pid,
	}
}

func (t *Tracer) ReAttachAndContinue(options int) error {
	// The Ptrace API is a bit of a mess and requires a hack to get group stops
	// to work properly with multithreaded programs. This code re-attaches to
	// the child with PTRACE_SEIZE so that group stops will work properly.
	unix.Kill(t.pid, unix.SIGSTOP)
	unix.PtraceDetach(t.pid)
	_, _, err := unix.Syscall6(unix.SYS_PTRACE, PTRACE_SEIZE, uintptr(t.pid), 0, uintptr(options), 0, 0)
	unix.Kill(t.pid, unix.SIGCONT)
	if err == 0 {
		return nil
	}
	return error(err)
}

func (t *Tracer) SetOptions(options int) error {
	return unix.PtraceSetOptions(t.pid, options)
}

func (t *Tracer) Cont(sig unix.Signal) error {
	return unix.PtraceCont(t.pid, int(sig))
}

func (t *Tracer) Syscall(sig unix.Signal) error {
	err := unix.PtraceSyscall(t.pid, int(sig))
	return err
}

func (t *Tracer) Listen() error {
	_, _, err := unix.Syscall6(unix.SYS_PTRACE, PTRACE_LISTEN, uintptr(t.pid), 0, 0, 0, 0)
	if err == 0 {
		return nil
	} else {
		return error(err)
	}
}

func (t *Tracer) SetRegs(regs *unix.PtraceRegs) error {
	return unix.PtraceSetRegs(t.pid, regs)
}

func (t *Tracer) GetRegs(regs *unix.PtraceRegs) error {
	return unix.PtraceGetRegs(t.pid, regs)
}

func (t *Tracer) PeekData(addr uintptr, data []byte) (int, error) {
	var nread int
	for nread < len(data) {
		n, err := unix.PtracePeekData(t.pid, addr, data[nread:])
		if n == 0 || err != nil {
			return nread, err
		}
		nread += n
	}
	return nread, nil
}

func (t *Tracer) PeekText(addr uintptr, data []byte) (int, error) {
	var nread int
	for nread < len(data) {
		n, err := unix.PtracePeekText(t.pid, addr, data[nread:])
		if n == 0 || err != nil {
			return nread, err
		}
		nread += n
	}
	return nread, nil
}

func (t *Tracer) PokeData(addr uintptr, data []byte) (int, error) {
	var nwritten int
	for nwritten < len(data) {
		n, err := unix.PtracePokeData(t.pid, addr, data[nwritten:])
		if n == 0 || err != nil {
			return nwritten, err
		}
		nwritten += n
	}
	return nwritten, nil
}

func (t *Tracer) PokeText(addr uintptr, data []byte) (int, error) {
	var nwritten int
	for nwritten < len(data) {
		n, err := unix.PtracePokeText(t.pid, addr, data[nwritten:])
		if n == 0 || err != nil {
			return nwritten, err
		}
		nwritten += n
	}
	return nwritten, nil
}

func (t *Tracer) ReadVM(addr uintptr, data []byte) (int, error) {
	remoteIov := unix.RemoteIovec{
		Base: addr,
		Len:  len(data),
	}
	localIov := unix.Iovec{
		Base: &data[0],
		Len:  uint64(len(data)),
	}
	return unix.ProcessVMReadv(t.pid, []unix.Iovec{localIov}, []unix.RemoteIovec{remoteIov}, 0)
}

func (t *Tracer) WriteVM(addr uintptr, data []byte) (int, error) {
	remoteIov := unix.RemoteIovec{
		Base: addr,
		Len:  len(data),
	}
	localIov := unix.Iovec{
		Base: &data[0],
		Len:  uint64(len(data)),
	}
	return unix.ProcessVMWritev(t.pid, []unix.Iovec{localIov}, []unix.RemoteIovec{remoteIov}, 0)
}

func (t *Tracer) Pid() int {
	return t.pid
}
