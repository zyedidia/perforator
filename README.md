# Perforator

Perforator is a tool for measuring performance metrics on individual functions
using the Linux "perf" interface. The `perf` tool provided by the Linux kernel
only supports collecting statistics over the complete lifetime of a program,
which is often inconvenient when a program includes setup and cleanup that
should not be profiled along with the benchmark. Perforator is not as
comprehensive as `perf` but it allows you to collect statistics for individual
functions.

Perforator only supports Linux AMD64. The target ELF binary may be generated
from any language. For function lookup, make sure the binary is not stripped
(it must contain a symbol table), and for additional information (source code
regions, inlined function lookup), the binary must include DWARF information.
Perforator supports position-independent binaries.

# Installation

1. Download the prebuilt binary from the releases page.

2. Install with `go get`:

```
$ go get github.com/zyedidia/perforator/cmd/perforator
```

# Usage

First make sure that you have the perf interface installed, and that you have
the appropriate permissions to record the events you are interested in (this
may require running Perforator with `sudo` or modifying
`/proc/sys/kernel/perf_event_paranoid` -- see [this
post](https://superuser.com/questions/980632/run-perf-without-root-rights)).

### Example

Suppose we had a C function that summed an array and wanted to benchmark it for
some large array of numbers. We could write a small benchmark program like so:

```c
#include <stdio.h>
#include <stdlib.h>
#include <time.h>
#include <stdint.h>

#define SIZE 10000000

uint64_t sum(uint32_t* numbers) {
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
$ gcc -g -O2 -o bench bench.c
```

Now we can measure with Perforator:

```
$ perforator -r sum ./bench
+---------------------+-------------+
| Event               | Count (sum) |
+---------------------+-------------+
| instructions        | 50000004    |
| branch-instructions | 10000002    |
| branch-misses       | 10          |
| cache-references    | 1246340     |
| cache-misses        | 14984       |
| time-elapsed        | 4.144814ms  |
+---------------------+-------------+
10736533065142551
```

Results are printed immediately when the profiled function returns.

Note: in this case we compiled with `-g` to include DWARF debugging
information.  This was necessary because GCC will inline the call to `sum`, so
Perforator needs to be able to read the DWARF information to determine where it
was inlined to. If you compile without `-g` make sure the target function is
not being inlined (either you know it is not inlined, or you mark it with the
`noinline` attribute).

Fun fact: clang does a better job optimizing this code than gcc. I tried
running this example with clang instead and found it only had 1,250,000 branch
instructions (roughly 8x fewer than gcc!). The reason: vector instructions.

By default, Perforator will measure some basic events such as instructions
executed, cache references, cache misses, branches, branch misses. You can
specify events yourself with the `-e` flag:

```
$ perforator -e l1d-read-accesses,l1d-read-misses -r sum ./bench
+-------------------+-------------+
| Event             | Count (sum) |
+-------------------+-------------+
| l1d-read-accesses | 10010311    |
| l1d-read-misses   | 625399      |
| time-elapsed      | 4.501523ms  |
+-------------------+-------------+
10736888439771461
```

To view available events, use the `--list` flag:

```
$ perforator --list hardware # List hardware events
$ perforator --list software # List software events
$ perforator --list cache    # List cache events
$ perforator --list trace    # List kernel trace events
```

### Source Code Regions

In additional to profiling functions, you may profile regions specified by source
code ranges if your binary has DWARF debugging information.

```
$ perforator -r bench.c:18-bench.c:23 ./bench
+---------------------+-------------------------------+
| Event               | Count (bench.c:18-bench.c:23) |
+---------------------+-------------------------------+
| instructions        | 668794280                     |
| branch-instructions | 169061639                     |
| branch-misses       | 335360                        |
| cache-references    | 945581                        |
| cache-misses        | 3569                          |
| time-elapsed        | 78.433272ms                   |
+---------------------+-------------------------------+
10737167007294257
```

Only certain line numbers are available for breakpoints. The range is exclusive
on the upper bound, meaning that in the example above `bench.c:23` is not
included in profiling.

You may also directly specify addresses as decimal or hexadecimal numbers. This
is useful if you don't have DWARF information but you know the addresses you
want to profile (for example, by inspecting the disassembly via `objdump`).

You can also profile multiple regions at once:

```
$ perforator -r bench.c:18-bench.c:23 -r sum -r main ./bench
+---------------------+-------------------------------+
| Event               | Count (bench.c:18-bench.c:23) |
+---------------------+-------------------------------+
| instructions        | 697120715                     |
| branch-instructions | 162949718                     |
| branch-misses       | 302849                        |
| cache-references    | 823087                        |
| cache-misses        | 3645                          |
| time-elapsed        | 78.832332ms                   |
+---------------------+-------------------------------+
+---------------------+-------------+
| Event               | Count (sum) |
+---------------------+-------------+
| instructions        | 49802557    |
| branch-instructions | 10000002    |
| branch-misses       | 9           |
| cache-references    | 1246639     |
| cache-misses        | 14382       |
| time-elapsed        | 4.235705ms  |
+---------------------+-------------+
10739785644063349
+---------------------+--------------+
| Event               | Count (main) |
+---------------------+--------------+
| instructions        | 675150939    |
| branch-instructions | 184259174    |
| branch-misses       | 386503       |
| cache-references    | 1128637      |
| cache-misses        | 8368         |
| time-elapsed        | 83.132829ms  |
+---------------------+--------------+
```

In this case, it may be useful to use the `--summary` option, which will
aggregate all results into a table that is printed when tracing stops.

```
$ perforator --summary -r bench.c:19-bench.c:24 -r sum -r main ./bench
10732787118410148
+-----------------------+--------------+---------------------+---------------+------------------+--------------+--------------+
| region                | instructions | branch-instructions | branch-misses | cache-references | cache-misses | time-elapsed |
+-----------------------+--------------+---------------------+---------------+------------------+--------------+--------------+
| bench.c:18-bench.c:23 | 718946520    | 172546336           | 326000        | 833098           | 3616         | 81.798381ms  |
| main                  | 678365328    | 174259806           | 363737        | 1115394          | 4403         | 86.321344ms  |
| sum                   | 43719896     | 10000002            | 9             | 1248069          | 16931        | 4.453342ms   |
+-----------------------+--------------+---------------------+---------------+------------------+--------------+--------------+
```

You can use the `--sort-key` and `--reverse-sort` options to modify which
columns are sorted and how. In addition, you can use the `--csv` option to
write the output table in CSV form.

Note: to an astute observer, the results from the above table don't look very
accurate.  In particular the totals for the main function seem questionable.
This is due to event multiplexing (explained more below), and for best results
you should not profile multiple regions simultaneously. In the table above, you
can see that it's likely that profiling for `main` was disabled while `sum` was
running.

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

# Notes and Caveats

* If your program receives a segmentation fault while being run by Perforator,
  you will most likely just see a "trace-continue : no such process" error. To
  confirm that a segmentation fault was received, enable verbose mode (with
  `-V`), which will display the additional info.
* Many CPUs expose additional/non-standardized raw perf events. Perforator does
  not currently support those events.
* Perforator has only limited support for multithreaded programs. It supports
  profiling programs with multiple threads as long as it is the case that each
  profiled region is only run by one thread (ever). In addition, the beginning
  and end of a region must be run by the same thread. This means if you are
  benchmarking Go you should call `runtime.LockOSThread` in your benchmark to
  prevent a goroutine migration while profiling.
* A region is either active or inactive, it cannot be active multiple times at
  once. This means for recursive functions only the first invocation of the
  function is tracked.
* Be careful of multiplexing, which occurs when you are trying to record more
  events than there are hardware counter registers. In particular, if you
  profile a function inside of another function being profiled, this will
  likely result in multiplexing and possibly incorrect counts. Perforator will
  automatically attempt to scale counts when multiplexing occurs. To see if
  this has happened, use the `-V` flag, which will print information when
  multiplexing is detected.
* Be careful if your target functions are being inlined. Perforator will
  automatically attempt to read DWARF information to determine the inline sites
  for target functions but it's a good idea to double check if you are seeing
  weird results. Use the `-V` flag to see where Perforator thinks the inline
  site is.

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
