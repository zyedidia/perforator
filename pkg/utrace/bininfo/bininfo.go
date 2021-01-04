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
	pie   bool
	funcs map[string]uint64
	// we use this map structure so that we can fuzzy match on the filename
	lines map[int][]address
}

// Read creates a new BinFile from an io.ReaderAt.
func Read(r io.ReaderAt) (*BinFile, error) {
	f, err := elf.NewFile(r)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	b := &BinFile{
		pie:   false,
		funcs: nil,
		lines: nil,
	}

	if f.Type == elf.ET_DYN || f.Type == elf.ET_REL {
		b.pie = true
	} else if f.Type != elf.ET_EXEC {
		return nil, ErrInvalidElfType
	}

	b.buildFuncCache(f)
	b.buildLineCache(f)

	return b, nil
}

func (b *BinFile) buildFuncCache(f *elf.File) error {
	symbols, err := f.Symbols()
	if err != nil {
		return err
	}

	b.funcs = make(map[string]uint64)

	for _, s := range symbols {
		if elf.ST_TYPE(s.Info) == elf.STT_FUNC {
			b.funcs[s.Name] = s.Value
		}
	}

	return nil
}

func (b *BinFile) buildLineCache(f *elf.File) error {
	dw, err := f.DWARF()
	if err != nil {
		return err
	}

	b.lines = make(map[int][]address)

	r := dw.Reader()
	for {
		e, err := r.Next()
		if err != nil {
			return err
		}
		if e == nil {
			break
		}

		if e.Tag == dwarf.TagCompileUnit {
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

				a := address{
					file: file,
					addr: entry.Address,
				}

				if existing, ok := b.lines[entry.Line]; ok {
					found := false
					for i, fa := range existing {
						if fa.file == file && entry.Address < fa.addr {
							b.lines[entry.Line][i].addr = entry.Address
						}
						if fa.file == file {
							found = true
						}
					}
					if !found {
						b.lines[entry.Line] = append(b.lines[entry.Line], a)
					}
				} else {
					b.lines[entry.Line] = []address{
						a,
					}
				}
			}
		}
	}
	return nil
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
	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return 0, err
		}
	}
	parts := strings.Split(scanner.Text(), "-")
	if len(parts) < 1 {
		return 0, errors.New("invalid /proc/pid/maps format")
	}
	off, err := strconv.ParseUint("0x"+parts[0], 0, 64)
	if err != nil {
		return 0, err
	}
	return off, nil
}
