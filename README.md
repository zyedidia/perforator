# Perforator

Perforator is a tool for measuring performance metrics on individual functions
using the Linux "perf" interface. The `perf` tool provided by the Linux kernel
only supports collecting statistics over the complete lifetime of a program,
which is often inconvenient when a program includes setup and cleanup that
should not be profiled along with the benchmark. Perforator is not as
comprehensive as `perf` but it allows you to collect statistics for individual
functions.

Perforator only supports Linux AMD64. The target binary may be an ELF binary
generated from any language, but should not be stripped and should not be
position-independent (you may need to recompile with `-no-pie`).

# Installation

1. Download the prebuilt binary from the releases page.

2. Install with `go get`:

```
$ go get github.com/zyedidia/perforator
```

# Usage

Suppose we had a C function that summed an array and wanted to benchmark it for
some large array of numbers. We could write a small benchmark program like so:

```c
#include <stdio.h>
#include <stdlib.h>
#include <time.h>

#define SIZE 10000000

// Perforator requires that benchmark functions are not inlined.
int64_t __attribute__ ((noinline)) sum(int32_t* numbers) {
    int64_t sum = 0;
    for (int32_t i = 0; i < SIZE; i++) {
        sum += numbers[i];
    }
    return sum;
}

int main() {
    srand(time(NULL));
    int32_t* numbers = malloc(SIZE * sizeof(int32_t));
    for (int i = 0; i < SIZE; i++) {
        numbers[i] = rand();
    }

    int64_t result = sum(numbers);
    printf("%lu\n", result);
    return 0;
}
```

If we want to determine the number of cache misses, branch mispredictions, etc... `perf`
is not suitable because running `perf stat` on this program will profile the creation of
the array in addition to the sum. With Perforator, we can measure just the sum. First
compile with

```
$ gcc -O2 -no-pie -o sumbench sumbench.c
```

Now we can measure with Perforator:

```
$ perforator -fn sum ./sumbench
+--------------+------------+
| EVENT        | COUNT      |
+--------------+------------+
| instruction  | 50000007   |
| cache-ref    | 1251232    |
| cache-miss   | 17010      |
| branch       | 10000003   |
| branch-miss  | 9          |
| time elapsed | 5.390111ms |
+--------------+------------+
10735190467306398
```

Results are printed immediately when the profiled function returns.

By default, Perforator will measure some basic events such as instructions
executed, cache references, cache misses, branches, branch misses. You can
specify events yourself with the `-e` flag:

```
$ perforator -e l1d-read-access,l1d-read-miss -fn sum ./sumbench
+-----------------+------------+
| EVENT           | COUNT      |
+-----------------+------------+
| l1d-read-access | 10010935   |
| l1d-read-miss   | 625148     |
| time elapsed    | 5.219392ms |
+-----------------+------------+
10737690284779529
```

To view available events, use the `-events` flag:

```
$ perforator -events hardware # List hardware events
$ perforator -events software # List software events
$ perforator -events cache    # List cache events
$ perforator -events trace    # List kernel trace events
```

# Limitations

Perforator was written in a short amount of time and thus has a number of
limitations that may be addressed in the future.

* Binaries with ASLR do not work, since Perforator uses the ELF symbol table to
  determine the address of a function. Apparently GDB is able to work around
  this somehow, but I'm not sure how it works. If anyone has info, let me know.
* Many CPUs expose additional/non-standardized raw perf events. Perforator does
  not currently support those events.
* Source code ranges: if a binary is compiled with debugging information,
  Perforator should be able to specify a range of code to profile instead of
  just a function.
* If your code has a segmentation fault during execution, perforator will be
  confused.
* Perforator does not currently support multithreaded programs.
* Perforator currently only supports profiling one function (although the
  function may be called multiple times).

# How it works

Perforator uses `ptrace` to trace the target program and enable profiling for
certain parts of the target program. Perforator places the `0xCC` "interrupt"
instruction at the beginning of the profiled function which allows it to regain
control when the function is executed. At that point, Perforator will place the
original code back (whatever was initially overwritten by the interrupt byte),
determine the return address by reading the top of the stack, and place an
interrupt byte at that address. Then Perforator will enable profiling and
resume the target process. When the next interrupt happens, the target will
have reached the return address and perforator can stop profiling, remove the
interrupt, and place a new interrupt back at the start of the function.
