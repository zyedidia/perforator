package perforator

import (
	"errors"
	"strconv"
	"strings"

	"github.com/zyedidia/perforator/pkg/utrace"
	"github.com/zyedidia/perforator/pkg/utrace/bininfo"
)

func parseLocation(s string, bin *bininfo.BinFile) (uint64, error) {
	if strings.Contains(s, ":") {
		parts := strings.Split(s, ":")
		file, lineStr := parts[0], parts[1]
		line, err := strconv.Atoi(lineStr)
		if err != nil {
			return 0, err
		}
		return bin.LineToPC(file, line)
	}
	return strconv.ParseUint(s, 0, 64)
}

// ParseRegion parses an address region. The region is written as loc-loc,
// where 'loc' is a location specified as either a file:line source code
// location (if the elf binary has DWARF debugging information), or a direct
// hexadecimal address in the form 0x...
func ParseRegion(s string, bin *bininfo.BinFile) (*utrace.AddressRegion, error) {
	parts := strings.Split(s, "-")
	if len(parts) != 2 {
		return nil, errors.New("invalid region")
	}

	start, err := parseLocation(parts[0], bin)
	if err != nil {
		return nil, err
	}
	end, err := parseLocation(parts[1], bin)
	if err != nil {
		return nil, err
	}

	return &utrace.AddressRegion{
		StartAddr: start,
		EndAddr:   end,
	}, nil
}
