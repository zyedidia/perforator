# Perforator

Perforator is a tool for measuring performance metrics on individual functions
using the Linux "perf" interface. The `perf` tool provided by the Linux kernel
only supports collecting statistics over the complete lifetime of a program,
which is often inconvenient when a program includes setup and cleanup that
should not be profiled along with the benchmark. Perforator is not as
comprehensive as `perf` but it allows you to collect statistics for individual
functions.

Perforator only supports Linux AMD64. The target binary may be an ELF binary
generated from any language, but should not be stripped. Perforator supports
position-independent binaries.

# Installation

1. Download the prebuilt binary from the releases page.

2. Install with `go get`:

```
$ go get github.com/zyedidia/perforator
```

# Usage

First make sure that you have the perf interface installed, and that you have
the appropriate permissions to record the events you are interested in (this
may require running Perforator with `sudo`).

Suppose we had a C function that summed an array and wanted to benchmark it for
some large array of numbers. We could write a small benchmark program like so:

```c
#include <stdio.h>
#include <stdlib.h>
#include <time.h>
#include <stdint.h>

#define SIZE 10000000

// Perforator requires that benchmark functions are not inlined.
uint64_t __attribute__ ((noinline)) sum(uint32_t* numbers) {
    uint64_t sum = 0;
    for (int i = 0; i < SIZE; i++) {
        sum += numbers[i];
    }
    return sum;
}

int main() {
    srand(time(NULL));
    uint32_t* numbers = malloc(SIZE * sizeof(uint32_t));
    for (int i = 0; i < SIZE; i++) {
        numbers[i] = rand();
    }

    uint64_t result = sum(numbers);
    printf("%lu\n", result);
    return 0;
}
```

If we want to determine the number of cache misses, branch mispredictions, etc... `perf`
is not suitable because running `perf stat` on this program will profile the creation of
the array in addition to the sum. With Perforator, we can measure just the sum. First
compile with

```
$ gcc -O2 -o bench bench.c
```

Now we can measure with Perforator:

```
$ perforator -f sum ./bench
Summary for 'sum':
+---------------------+------------+
| EVENT               | COUNT      |
+---------------------+------------+
| instructions        | 50000007   |
| branch-instructions | 10000003   |
| branch-misses       | 9          |
| cache-references    | 1248551    |
| cache-misses        | 18480      |
| time elapsed        | 4.455265ms |
+---------------------+------------+
10735190467306398
```

Results are printed immediately when the profiled function returns.

By default, Perforator will measure some basic events such as instructions
executed, cache references, cache misses, branches, branch misses. You can
specify events yourself with the `-e` flag:

```
$ perforator -e l1d-read-accesses,l1d-read-misses -f sum ./bench
Summary for 'sum':
+-------------------+------------+
| EVENT             | COUNT      |
+-------------------+------------+
| l1d-read-accesses | 10010773   |
| l1d-read-misses   | 625368     |
| time elapsed      | 4.488998ms |
+-------------------+------------+
10737690284779529
```

To view available events, use the `--list` flag:

```
$ perforator --list hardware # List hardware events
$ perforator --list software # List software events
$ perforator --list cache    # List cache events
$ perforator --list trace    # List kernel trace events
```

## Advanced Usage

### Regions

In additional to profiling functions, you may profile regions specified by source
code ranges if your binary has DWARF debugging information. For example, if we compile
the previous example with

```
$ gcc -O2 -g -o bench bench.c
```

we can now profile specific lines. In particular, if we wanted to profile the generation
of the dataset, we could do so with

```
$ perforator -r bench.c:19-bench.c:24 ./bench
Summary for 'bench.c:19-bench.c:24':
+---------------------+------------+
| EVENT               | COUNT      |
+---------------------+------------+
| instructions        | 668794281  |
| branch-instructions | 169061640  |
| branch-misses       | 335307     |
| cache-references    | 950388     |
| cache-misses        | 2803       |
| time elapsed        | 73.89277ms |
+---------------------+------------+
10738993047151290
```

We can also profile multiple regions at once:

```
$ perforator -r bench.c:19-bench.c:24 -f sum -f main ./bench
Summary for 'bench.c:19-bench.c:24':
+---------------------+-------------+
| EVENT               | COUNT       |
+---------------------+-------------+
| instructions        | 658238065   |
| branch-instructions | 173282494   |
| branch-misses       | 349532      |
| cache-references    | 1037942     |
| cache-misses        | 2459        |
| time elapsed        | 77.929411ms |
+---------------------+-------------+
Summary for 'sum':
+---------------------+------------+
| EVENT               | COUNT      |
+---------------------+------------+
| instructions        | 46652091   |
| branch-instructions | 10000003   |
| branch-misses       | 10         |
| cache-references    | 1247711    |
| cache-misses        | 17311      |
| time elapsed        | 4.460274ms |
+---------------------+------------+
10732394201030672
Summary for 'main':
+---------------------+-------------+
| EVENT               | COUNT       |
+---------------------+-------------+
| instructions        | 736908891   |
| branch-instructions | 173772061   |
| branch-misses       | 338576      |
| cache-references    | 901855      |
| cache-misses        | 5809        |
| time elapsed        | 82.498118ms |
+---------------------+-------------+
```

### Groups

The CPU has a fixed number of performance counters. If you try recording more
events than there are counters, "multiplexing" will be performed to estimate
the totals for all the events. For example, if we record 6 events on the sum
benchmark, the instruction count becomes less stable. This is because the
number of events now exceeds the number of hardware registers for counting, and
multiplexing occurs. To ensure that certain events are always counted together,
you can put them all in a group with the `-g` option. The `-g` option has the
same syntax as the `-e` option, but may be specified multiple times (for
multiple groups).

# Notes

* If your program receives a segmentation fault while being run by Perforator,
  you will most likely just see a "trace-continue : no such process" error. To
  confirm that a segmentation fault was received, enable verbose mode (with
  `-V`), which will display the additional info.
* Many CPUs expose additional/non-standardized raw perf events. Perforator does
  not currently support those events.
* Perforator does not currently support multithreaded programs.
* A region is either active or inactive, it cannot be active multiple times at
  once. This means for recursive functions only the first invocation of the
  function is tracked.

# How it works

Perforator uses `ptrace` to trace the target program and enable profiling for
certain parts of the target program. Perforator places the `0xCC` "interrupt"
instruction at the beginning of the profiled function which allows it to regain
control when the function is executed. At that point, Perforator will place the
original code back (whatever was initially overwritten by the interrupt byte),
determine the return address by reading the top of the stack, and place an
interrupt byte at that address. Then Perforator will enable profiling and
resume the target process. When the next interrupt happens, the target will
have reached the return address and Perforator can stop profiling, remove the
interrupt, and place a new interrupt back at the start of the function.
