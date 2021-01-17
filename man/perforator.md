---
title: perforator
section: 1
header: Perforator Manual
---

# NAME
  Perforator - Performance analysis and tracing tool for Linux

# SYNOPSIS
  perforator `[--version] [--help] [OPTIONS] COMMAND [ARGS]`

# DESCRIPTION
  Perforator is a tool for measuring performance metrics on individual
  functions and regions using the Linux **perf_event_open**(2) interface.
  Perforator supports measuring instructions executed, cache misses, branch
  mispredictions, etc... during a single function call or region of user code.

# EVENTS

Perforator supports recording the following events (some may not be available on your
system, use **`perforator --list [event-type]`** to view available events). The following
descriptions are adapted from **perf_event_open**(2), the system call used by Perforator
to record metrics.

  _hardware_

:   * **instructions**: Retired instructions. Be careful, these can be affected by various issues, most notably hardware interrupt counts.
    * **cpu-cycles**: Total cycles. Be wary of what happens during CPU frequency scaling.
    * **cache-references**: Cache accesses. Usually this indicates Last Level Cache accesses but this may vary depending on your CPU. This may include prefetches and coherency messages; again this depends on the design of your CPU.
    * **cache-misses**: Cache misses. Usually this indicates Last Level Cache misses; this is intended to be used in conjunction with **cache-references** to calculate cache miss rates.
    * **branch-instructions**: Retired branch instructions.
    * **branch-misses**: Mispredicted branch instructions.
    * **stalled-cycles-frontent**: Stalled cycles during issue.
    * **stalled-cycles-backend**: Stalled cycles during retirement.
    * **ref-cycles**: Total cycles; not affected by CPU frequency scaling.

  _software_

:   * **cpu-clock**: The CPU clock, a high-resolution per-CPU timer.
    * **task-clock**: A clock count specific to the task that is running.
    * **context-switches**: Context switches.
    * **cpu-migrations**: The number of times the process has migrated to a new CPU.
    * **page-faults**: The number of page faults.
    * **major-faults**: The number of major page faults.
    * **minor-faults**: The number of minor page faults.
    * **alignment-faults**: The number of alignment faults. These happen when unaligned memory accesses happen; the kernel can handle these but it reduces performance. This happens only on some architectures (never on x86).
    * **emulation-faults**: The number of emulation faults. The kernel sometimes traps on unimplemented instructions and emulates them for user space. This can negatively impact performance.

  _cache_

:   A cache event is made up of three parts: a cache, an operation type, and an operation result. The resulting event is written as **cache-type-result** -- for example **l1d-read-misses**.

    _caches_

:   >* **l1d**: for measuring the Level 1 Data Cache.
    * **l1i**: for measuring the Level 1 Instruction Cache.
    * **ll**: for measuring the Last-Level Cache.
    * **dtlb**: for measuring the Data TLB.
    * **itlb**: for measuring the Instruction TLB.
    * **bpu**: for measuring the branch prediction unit.
    * **node**: for measuring local memory accesses.

    _operation types_
:   >* **read**: for reads.
    * **write**: for writes.
    * **prefetch**: for prefetches.

    _operation results_
:   >* **accesses**: to measure accesses.
    * **misses**: to measure misses.

_trace_

:    System-dependent. Usually this includes kernel trace events, such as system call entry
     points to count the number of times a system call is executed.

# OPTIONS
  `-l, --list=`

:    List available events for {hardware, software, cache, trace} event types.

  `-e, --events=`

:    Comma-separated list of events to profile.

  `-g, --group=`

:    Comma-separated list of events to profile together as a group.

  `-r, --region=`

:    Region(s) to profile: 'function' or 'start-end'; start/end locations may be
    file:line or hex addresses.

  `--kernel`

:    Include kernel code in measurements.

  `--hypervisor`

:    Include hypervisor code in measurements.

  `--exclude-user`

:    Exclude user code from measurements.

  `-s, --summary`

:    Instead of printing results immediately, show an aggregated summary afterwards.

  `--sort-key=`

:    Key to sort summary tables with.

  `--reverse-sort`

:    Reverse summary table sorting.

  `--csv`

:    Write summary output in CSV format.

  `-o, --output=`

:    Write summary output to file.

  `-V, --verbose`

:    Show verbose debug information.

  `-v, --version`

:    Show version information.

  `-h, --help`

:    Show this help message.


# BUGS

See GitHub Issues: <https://github.com/zyedidia/perforator/issues>

# AUTHOR

Zachary Yedidia <zyedidia@gmail.com>

# SEE ALSO

**perf(1)**, **perf\_event\_open(2)**

