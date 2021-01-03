package bininfo

import (
	"debug/elf"
	"fmt"
	"sort"
	"strings"

	elfinfo "github.com/yalue/elf_reader"
)

func getFuncs(f elfinfo.ELFFile) (map[string]uintptr, error) {
	fns := make(map[string]uintptr)
	count := f.GetSectionCount()
	for i := uint16(0); i < count; i++ {
		if !f.IsSymbolTable(uint16(i)) {
			continue
		}
		symbols, names, e := f.GetSymbols(uint16(i))
		if e != nil {
			return nil, fmt.Errorf("Couldn't read symbol table: %s", e)
		}
		for j, s := range symbols {
			if s.GetInfo().SymbolType() == byte(elf.STT_FUNC) {
				fns[names[j]] = uintptr(s.GetValue())
			}
		}
	}
	return fns, nil
}

type BinFile struct {
	funcs map[string]uintptr
}

func OpenBinFile(data []byte) (*BinFile, bool, error) {
	elf, err := elfinfo.ParseELFFile(data)
	if err != nil {
		return nil, false, err
	}

	aslr := true
	if elf.GetFileType() == elfinfo.ELFTypeExecutable {
		aslr = false
	}

	fns, err := getFuncs(elf)
	if err != nil {
		return nil, false, err
	}

	return &BinFile{
		funcs: fns,
	}, aslr, nil
}

func (b *BinFile) FuzzyFunc(fn string) []string {
	fns := make([]string, 0)
	for k := range b.funcs {
		if k == fn {
			return []string{fn}
		}
		if strings.Contains(k, fn) {
			fns = append(fns, k)
		}
	}
	sort.Strings(fns)
	return fns
}

func (b *BinFile) FuncToPC(fn string) (uintptr, bool) {
	pc, ok := b.funcs[fn]
	return pc, ok
}
