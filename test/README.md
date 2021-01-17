This is the same example from the main readme but written in Go. Compile with

```
$ go build sum.go
```

or

```
$ go build -buildmode=pie sum.go
```

Then you can profile with perforator:

```
$ perforator -r main.sum ./sum
+---------------------+------------------+
| Event               | Count (main.sum) |
+---------------------+------------------+
| instructions        | 70000011         |
| branch-instructions | 10000005         |
| branch-misses       | 7                |
| cache-references    | 1252194          |
| cache-misses        | 5240             |
| time-elapsed        | 6.171575ms       |
+---------------------+------------------+
10738640711842731
```
