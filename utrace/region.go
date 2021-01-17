package utrace

import (
	"encoding/binary"
)

// A Region defines a start and an end address.
type Region interface {
	Start(p *Proc) uint64
	End(sp uint64, p *Proc) (uint64, error)
}

// An AddressRegion is the simplest possible region that directly stores the
// start and end addresses.
type AddressRegion struct {
	StartAddr uint64
	EndAddr   uint64
}

// Start returns this region's start address.
func (a *AddressRegion) Start(p *Proc) uint64 {
	return a.StartAddr + p.pieOffset
}

// End returns this region's end address.
func (a *AddressRegion) End(sp uint64, p *Proc) (uint64, error) {
	return a.EndAddr + p.pieOffset, nil
}

// A FuncRegion refers to a function, where the region begins at the start of
// the function and ends when the function returns.
type FuncRegion struct {
	Addr uint64
}

// Start returns this region's start address.
func (f *FuncRegion) Start(p *Proc) uint64 {
	return f.Addr + p.pieOffset
}

// End calculates the return address of this function given the current stack
// frame. It is assumed that a call instruction has just been executed, so the
// return address is at the top of the stack.
func (f *FuncRegion) End(sp uint64, p *Proc) (uint64, error) {
	b := make([]byte, 8)
	// read the return address from the top of the stack
	_, err := p.tracer.ReadVM(uintptr(sp), b)
	if err != nil {
		return 0, err
	}

	retaddr := binary.LittleEndian.Uint64(b)
	return retaddr, nil
}

// A RegionState represents the current state of the region.
type RegionState byte

const (
	// RegionStart indicates that the child has just begun execution of this
	// region.
	RegionStart = iota
	// RegionEnd indicates that the child has just finished execution of this
	// region.
	RegionEnd
)

type activeRegion struct {
	region       Region
	state        RegionState
	curInterrupt uint64

	id int
}
