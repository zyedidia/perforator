package utrace

import (
	"encoding/binary"

	"github.com/zyedidia/utrace/ptrace"
)

type Region interface {
	Start() uint64
	End(sp uint64, tracer *ptrace.Tracer) (uint64, error)
}

type AddressRegion struct {
	StartAddr uint64
	EndAddr   uint64
	Name      string
}

func (a *AddressRegion) Start() uint64 {
	return a.StartAddr
}

func (a *AddressRegion) End(sp uint64, tracer *ptrace.Tracer) (uint64, error) {
	return a.EndAddr, nil
}

func (f *AddressRegion) String() string {
	return f.Name
}

type FuncRegion struct {
	Addr uint64
	Name string
}

func (f *FuncRegion) Start() uint64 {
	return f.Addr
}

func (f *FuncRegion) End(sp uint64, tracer *ptrace.Tracer) (uint64, error) {
	b := make([]byte, 8)
	// read the return address from the top of the stack
	_, err := tracer.ReadVM(uintptr(sp), b)
	if err != nil {
		return 0, err
	}

	retaddr := binary.LittleEndian.Uint64(b)
	return retaddr, nil
}

func (f *FuncRegion) String() string {
	return f.Name
}

type RegionState byte

const (
	RegionStart = iota
	RegionEnd
)

// type UserTracer interface {
// 	Init(pid int, r Region) error
// 	Start(pid int, pc uint64, r Region) error
// 	End(pid int, pc uint64, r Region) error
// }
//
// type RegionFunc func(r Region)

type activeRegion struct {
	region       Region
	state        RegionState
	curInterrupt uint64

	id int
}
