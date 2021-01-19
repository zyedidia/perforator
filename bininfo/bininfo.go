// Package bininfo provides functions for reading elf binary files and
// converting file/line pairs or functions to addresses.
package bininfo

import (
	"bufio"
	"bytes"
	"debug/dwarf"
	"debug/elf"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

var (
	ErrInvalidElfType = errors.New("invalid elf type")
)

// ErrMultipleMatches is an error that describes a filename or function name
// matching multiple known files/functions.
type ErrMultipleMatches struct {
	Matches []string
}

func (e *ErrMultipleMatches) Error() string {
	if len(e.Matches) == 0 {
		return "no matches"
	}

	b := &bytes.Buffer{}
	b.WriteString("Multiple matches:\n")
	for _, m := range e.Matches {
		b.WriteString(m)
		b.WriteByte('\n')
	}
	return b.String()
}

type address struct {
	file string
	addr uint64
}

// A BinFile provides functions for converting source code structures such as
// functions and line numbers into addresses. The BinFile also tracks if the
// executable is position-independent and if so provides a function to compute
// the PIE offset for a running instance.
type BinFile struct {
	pie     bool
	funcs   map[string]uint64
	inlined map[string][]InlinedFunc
	// we use this map structure so that we can fuzzy match on the filename
	lines map[int][]address
	name  string
}

// FromPid creates a new BinFile from a running process.
func FromPid(pid int) (*BinFile, error) {
	binpath, err := os.Readlink(fmt.Sprintf("/proc/%d/exe", pid))
	if err != nil {
		return nil, err
	}
	f, err := os.Open(binpath)
	if err != nil {
		return nil, err
	}
	return Read(f, filepath.Base(binpath))
}

// Read creates a new BinFile from an io.ReaderAt.
func Read(r io.ReaderAt, name string) (*BinFile, error) {
	f, err := elf.NewFile(r)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	b := &BinFile{
		pie:     false,
		funcs:   nil,
		lines:   nil,
		inlined: nil,
		name:    name,
	}

	if f.Type == elf.ET_DYN {
		b.pie = true
	} else if f.Type != elf.ET_EXEC {
		return nil, ErrInvalidElfType
	}

	// Get the vaddr of the first loadable segment. I'm not sure if this is the
	// right way to find the vaddr offset but it seems to work and I couldn't
	// find any documentation about this.
	var vaddr uint64
	if b.pie {
		for _, p := range f.Progs {
			if p.Type == elf.PT_LOAD {
				vaddr = p.Vaddr
				break
			}
		}
	}

	b.buildFuncCache(f, vaddr)
	b.buildInlinedFuncCache(f, vaddr)
	b.buildLineCache(f, vaddr)

	return b, nil
}

func (b *BinFile) buildFuncCache(f *elf.File, offset uint64) error {
	symbols, err := f.Symbols()
	if err != nil {
		return err
	}

	b.funcs = make(map[string]uint64)

	for _, s := range symbols {
		if elf.ST_TYPE(s.Info) == elf.STT_FUNC {
			b.funcs[s.Name] = s.Value - offset
		}
	}

	return nil
}

// An InlinedFunc is a range of addresses representing the beginning and end of
// the inlined function.
type InlinedFunc struct {
	Low  uint64
	High uint64
}

func (b *BinFile) buildInlinedFuncCache(f *elf.File, offset uint64) error {
	dw, err := f.DWARF()
	if err != nil {
		return err
	}

	b.inlined = make(map[string][]InlinedFunc)
	inlinedAbstract := make(map[dwarf.Offset][]InlinedFunc)

	r := dw.Reader()
	for {
		e, err := r.Next()
		if err != nil {
			return err
		}
		if e == nil {
			break
		}
		if e.Tag == dwarf.TagInlinedSubroutine {
			dwoffset, okOff := e.Val(dwarf.AttrAbstractOrigin).(dwarf.Offset)
			lowpc, okLow := e.Val(dwarf.AttrLowpc).(uint64)
			var highpc uint64
			var okHigh bool

			highpcEntry := e.AttrField(dwarf.AttrHighpc)
			if highpcEntry != nil {
				switch highpcEntry.Class {
				case dwarf.ClassAddress:
					highpc, okHigh = highpcEntry.Val.(uint64)
				case dwarf.ClassConstant:
					rel, ok := highpcEntry.Val.(int64)
					if ok {
						highpc = lowpc + uint64(rel)
						okHigh = true
					}
				}
			}

			if okOff && okLow && okHigh {
				inlinedAbstract[dwoffset] = append(inlinedAbstract[dwoffset], InlinedFunc{
					Low:  lowpc - offset,
					High: highpc - offset,
				})
			}
		}
	}
	r = dw.Reader()
	for {
		e, err := r.Next()
		if err != nil {
			return err
		}
		if e == nil {
			break
		}
		if e.Tag == dwarf.TagSubprogram {
			if addrs, ok := inlinedAbstract[e.Offset]; ok {
				fnname, ok := e.Val(dwarf.AttrName).(string)
				if ok {
					b.inlined[fnname] = addrs
				}
			}
		}
	}
	return nil
}

func (b *BinFile) buildLineCache(f *elf.File, offset uint64) error {
	dw, err := f.DWARF()
	if err != nil {
		return err
	}

	b.lines = make(map[int][]address)

	var filetable []string
	r := dw.Reader()
	for {
		e, err := r.Next()
		if err != nil {
			return err
		}
		if e == nil {
			break
		}

		if e.Tag == dwarf.TagInlinedSubroutine {
			callfile, okFile := e.Val(dwarf.AttrCallFile).(int64)
			callline, okLine := e.Val(dwarf.AttrCallLine).(int64)
			lowpc, okPC := e.Val(dwarf.AttrLowpc).(uint64)
			if okFile && okLine && okPC && callfile < int64(len(filetable)) {
				b.addLineCacheEntry(
					filetable[callfile],
					int(callline),
					lowpc-offset,
				)
			}
		} else if e.Tag == dwarf.TagCompileUnit {
			filetable = make([]string, 0)

			lr, err := dw.LineReader(e)
			if err != nil || lr == nil {
				continue
			}
			var entry dwarf.LineEntry

			for {
				err = lr.Next(&entry)
				if err == io.EOF {
					break
				} else if err != nil || !entry.IsStmt {
					continue
				}

				var file string
				if entry.File == nil {
					file = "<unknown>"
				} else {
					file = entry.File.Name
				}
				filetable = append(filetable, file)

				b.addLineCacheEntry(
					file,
					entry.Line,
					entry.Address-offset,
				)
			}
		}
	}
	return nil
}

func (b *BinFile) addLineCacheEntry(file string, line int, addr uint64) {
	a := address{
		file: file,
		addr: addr,
	}

	if existing, ok := b.lines[line]; ok {
		found := false
		for i, fa := range existing {
			if fa.file == file && addr < fa.addr {
				b.lines[line][i].addr = addr
			}
			if fa.file == file {
				found = true
			}
		}
		if !found {
			b.lines[line] = append(b.lines[line], a)
		}
	} else {
		b.lines[line] = []address{
			a,
		}
	}
}

// Pie returns true if this executable is position-independent.
func (b *BinFile) Pie() bool {
	return b.pie
}

// FuncToPC converts a function name to a PC. It does a "fuzzy" search so if
// the given name is a substring of a real function name, and the substring
// uniquely identifies it, that function is used. If there are multiple matches
// it returns a multiple match error describing all the matches.
func (b *BinFile) FuncToPC(name string) (uint64, error) {
	if b.funcs == nil {
		return 0, errors.New("no elf symbol table")
	}

	if addr, ok := b.funcs[name]; ok {
		return addr, nil
	}

	matches := make([]string, 0)
	for fn := range b.funcs {
		if strings.Contains(fn, name) {
			matches = append(matches, fn)
		}
	}

	if len(matches) == 1 {
		return b.funcs[matches[0]], nil
	}

	return 0, &ErrMultipleMatches{
		Matches: matches,
	}
}

// InlinedFuncToPCs is the same as FuncToPCs but works for inlined functions
// and returns all start addresses and end addresses of the various inlinings
// of the specified function.
func (b *BinFile) InlinedFuncToPCs(name string) ([]InlinedFunc, error) {
	if b.inlined == nil {
		return []InlinedFunc{}, errors.New("no DWARF debugging data")
	}

	if addrs, ok := b.inlined[name]; ok {
		return addrs, nil
	}

	matches := make([]string, 0)
	for fn := range b.funcs {
		if strings.Contains(fn, name) {
			matches = append(matches, fn)
		}
	}

	if len(matches) == 1 {
		return b.inlined[matches[0]], nil
	}

	return []InlinedFunc{}, &ErrMultipleMatches{
		Matches: matches,
	}
}

// LineToPC converts a file/line location to a PC. It performs a "fuzzy" search
// on the filename similar to FuncToPC.
func (b *BinFile) LineToPC(file string, line int) (uint64, error) {
	if b.lines == nil {
		return 0, errors.New("no DWARF debugging data")
	}

	addrs, ok := b.lines[line]
	if !ok {
		return 0, fmt.Errorf("%s:%d has no associated PC", file, line)
	}

	var addr uint64
	matches := make([]string, 0)
	for _, fa := range addrs {
		if fa.file == file {
			return fa.addr, nil
		} else if strings.Contains(fa.file, file) {
			matches = append(matches, fa.file)
			addr = fa.addr
		}
	}

	if len(matches) == 1 {
		return addr, nil
	} else if len(matches) == 0 {
		return 0, fmt.Errorf("%s:%d has no associated PC", file, line)
	}

	return 0, &ErrMultipleMatches{
		Matches: matches,
	}
}

// PieOffset returns the PIE/ASLR offset for a running instance of this binary
// file. It reads /proc/pid/maps to determine the right location, so the caller
// must have ptrace permissions. If possible, you should cache the result of
// this function instead of calling it multiple times.
func (b *BinFile) PieOffset(pid int) (uint64, error) {
	if !b.pie {
		return 0, nil
	}

	maps, err := os.Open(fmt.Sprintf("/proc/%d/maps", pid))
	if err != nil {
		return 0, err
	}
	defer maps.Close()

	// only read the first line of the file (this will show the bottom mapping
	// for the text segment)
	scanner := bufio.NewScanner(maps)
	for {
		if !scanner.Scan() {
			if err := scanner.Err(); err != nil {
				return 0, err
			}
			break
		}
		line := scanner.Text()
		if !strings.Contains(line, filepath.Base(b.name)) {
			continue
		}
		parts := strings.Split(line, "-")
		if len(parts) < 1 {
			continue
		}
		off, err := strconv.ParseUint("0x"+parts[0], 0, 64)
		if err != nil {
			return 0, err
		}
		return off, nil
	}
	return 0, errors.New("could not find pie offset")
}
