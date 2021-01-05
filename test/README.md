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
$ perforator -f main.sum ./sum
Summary for 'main.sum':
+---------------------+------------+
| Event               | Count      |
+---------------------+------------+
| instructions        | 70001570   |
| branch-instructions | 10000333   |
| branch-misses       | 102        |
| cache-references    | 1248065    |
| cache-misses        | 7430       |
| time-elapsed        | 5.851634ms |
+---------------------+------------+
10738640711842731
```
