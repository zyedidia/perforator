package main

import (
	"encoding/binary"
	"fmt"

	"github.com/zyedidia/perforator/ptrace"
)

type Block interface {
	Start() uintptr
	End(sp uint64, tracer *ptrace.Tracer) uintptr
}

type AddressBlock struct {
	start uintptr
	end   uintptr
}

func (a *AddressBlock) Start() uintptr {
	return a.start
}

func (a *AddressBlock) End(sp uint64, tracer *ptrace.Tracer) uintptr {
	return a.end
}

func (a *AddressBlock) String() string {
	return fmt.Sprintf("0x%x-0x%x", a.start, a.end)
}

type FuncBlock struct {
	addr uintptr
}

func (f *FuncBlock) Start() uintptr {
	return f.addr
}

func (f *FuncBlock) End(sp uint64, tracer *ptrace.Tracer) uintptr {
	b := make([]byte, 8)
	// read the return address from the top of the stack
	_, err := tracer.ReadVM(uintptr(sp), b)
	must(err)

	retaddr := binary.LittleEndian.Uint64(b)
	return uintptr(retaddr)
}

func (f *FuncBlock) String() string {
	return fmt.Sprintf("Function at 0x%x", f.addr)
}
