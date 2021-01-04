module github.com/zyedidia/perforator

go 1.15

require (
	acln.ro/perf v0.0.0-20200512125540-4d8e4e566115
	github.com/jessevdk/go-flags v1.4.0
	github.com/olekukonko/tablewriter v0.0.4
	github.com/zyedidia/utrace v0.0.0-00010101000000-000000000000
	golang.org/x/sys v0.0.0-20201231184435-2d18734c6014
)

replace github.com/zyedidia/utrace => ./pkg/utrace
