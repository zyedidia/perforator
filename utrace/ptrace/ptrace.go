package ptrace

import (
	"golang.org/x/sys/unix"
)

// A Tracer keeps track of a process and allows running ptrace functions on
// that process.
type Tracer struct {
	pid int
}

// NewTracer returns a tracer for the given PID.
func NewTracer(pid int) *Tracer {
	return &Tracer{
		pid: pid,
	}
}

// ReAttachAndContinue re-attaches to a traced process with PTRACE_SEIZE.  The
// Ptrace API is a bit of a mess and requires a hack to get group stops to work
// properly with multithreaded programs. If the tracer re-attaches to the child
// with PTRACE_SEIZE then group stops will work properly.
func (t *Tracer) ReAttachAndContinue(options int) error {
	unix.Kill(t.pid, unix.SIGSTOP)
	unix.PtraceDetach(t.pid)
	_, _, err := unix.Syscall6(unix.SYS_PTRACE, unix.PTRACE_SEIZE, uintptr(t.pid), 0, uintptr(options), 0, 0)
	unix.Kill(t.pid, unix.SIGCONT)
	if err == 0 {
		return nil
	}
	return error(err)
}

// SetOptions changes the ptrace options.
func (t *Tracer) SetOptions(options int) error {
	return unix.PtraceSetOptions(t.pid, options)
}

// GetEventMsg returns the newest event message.
func (t *Tracer) GetEventMsg() (uint, error) {
	return unix.PtraceGetEventMsg(t.pid)
}

// Cont continues execution of the child until the next event.
func (t *Tracer) Cont(sig unix.Signal) error {
	return unix.PtraceCont(t.pid, int(sig))
}

// Syscall continues execution of the child until the next syscall event.
func (t *Tracer) Syscall(sig unix.Signal) error {
	err := unix.PtraceSyscall(t.pid, int(sig))
	return err
}

// Listen should be used to continue execution when a group stop occurs.
func (t *Tracer) Listen() error {
	_, _, err := unix.Syscall6(unix.SYS_PTRACE, unix.PTRACE_LISTEN, uintptr(t.pid), 0, 0, 0, 0)
	if err == 0 {
		return nil
	} else {
		return error(err)
	}
}

// SetRegs assigns the registers of the tracee.
func (t *Tracer) SetRegs(regs *unix.PtraceRegs) error {
	return unix.PtraceSetRegs(t.pid, regs)
}

// GetRegs fetches the registers of the tracee.
func (t *Tracer) GetRegs(regs *unix.PtraceRegs) error {
	return unix.PtraceGetRegs(t.pid, regs)
}

// PeekData reads len(data) bytes at 'addr' in the child and places the bytes
// in the data slice. It returns the amount of data read or an error.
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

// PeekText is the same as PeekData, except for the text segment. On AMD64
// there is no difference between PeekData and PeekText.
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

// PokeData writes data to the child's memory at 'addr'.
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

// PokeText is the same as PokeData on AMD64.
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

// ReadVM uses the process_read_vm system call to read len(data) bytes from the
// child's 'addr' address. This is the same as PeekData, except process_read_vm
// may be subject to additional permissions that the child is restricted to.
// Generally ReadVM is faster than PeekData.
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

// WriteVM uses the process_write_vm system call to write data to addr in the
// child. It is functionally the same as PokeData but requires the region to be
// writable for the child as well.
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

// Pid returns the PID of the traced process.
func (t *Tracer) Pid() int {
	return t.pid
}
