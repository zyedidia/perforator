module github.com/zyedidia/perforator

go 1.15

require (
	github.com/hodgesds/perf-utils v0.2.5
	github.com/yalue/elf_reader v0.0.0-20201008162652-04ba8f01deb5
	golang.org/x/sys v0.0.0-20201223074533-0d417f636930
)

replace github.com/hodgesds/perf-utils => ./pkg/perf-utils
