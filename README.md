# The Go Programming Language For Bare-Metal ARMv7a

This repo contains my modifications to Go that enable it
to run bare-metal on armv7a SOCs. The basic OS primitives that
Go relies on have been re-implemented in Go and Plan 9 assembly
inside the `runtime` package.

This repo tracks the master branch of the main go repo at https://github.com/golang/go.

This modified runtime is also an integral part of G.E.R.T,
the Golang Embedded RunTime. Check it out for working examples.

https://github.com/ycoroneos/G.E.R.T

## Modifications To Go

The majority of the runtime modifications are inside `src/runtime`.
Each of the files below have comments in them which distinguish the
different sections and explain the functions.

  |File | Functions |
  |----------|----------|
  |`src/runtime/gert_arm.go`| Scheduler, context switching, SMP booting, virtual memory, trap handling|
  |`src/runtime/gertasm_arm.s`| Assembly routines for ARM global timer, saving/restoring trapframes, interrupt entry points, trampolines for booting cpus, mpcore configuration, loading ttbr0|
  |`src/runtime/gertcommon.go`| Contains global `Armhackmode` variable for triggering certain GERT-specific boot tasks in the runtime|
  |`src/runtime/asm_arm.s`| GERT boot functions in `rt0_go` |
  |`src/runtime/sys_linux_arm.s`| Replaced all syscalls with calls to `trap_debug` |

## Bootloaders

Uboot sets up the initial device clocks and chainloads the GERT
bootloader, which is written in C. The GERT bootloader prepares a stack before
loading the GERT ELF and jumping into it. GERT is linked and loaded
at 0x1100\_0000. The C bootloader is linked and loaded at 0x5000\_0000.
These addresses are due to the memory map of the iMX6.

## GERT Boot Process

The GERT entry point is `main.Entry` inside the `main` package.
It's easy to tell Go about a new entry point and link
address in a Makefile:
```
GOLINKFLAGS := "-T <link_address> -E main.Entry"
```

`main.Entry` sets `runtime.Armhackmode=True` and calls `runtime.Runtime_main()`, which continues
the Go boot process. Since `Armhackmode=True`, the normal Go boot
process takes a few detours to setup threading, virtual memory, and
context switching.

## Virtual Memory

GERT uses a single L1 page table with 1MB pages in it. This is because
GERT runs in a single address space. Go's own memory safety and
isolation primitives means that multiple address spaces with fine page
granularity are unnecessary. The reason the MMU is even on
is to deal with gaps in the physical memory space near address 0x0.
Additionally, the Go runtime sometimes requests a fixed page mapping
which can't exist in the physical space.

## Syscalls

Calls to `SWI` have been converted to `trap_debug`. All
relevent linux syscalls that Go relies on have been re-implemented in Go
inside `trap_debug` inside `sys_linux_arm.s`. `trap_debug` lives inside `gert_arm.go`. When the Go runtime
executes a syscall, it actually redirects execution back into Go.
Sometimes syscalls can be blocking (such as `yield` or `futex`) so
the modified runtime maintains a threadlist and trapframes to switch between.

## Time

GERT uses the 64bit ARM global timer as its primary time source. Each
tick is approximately 2ns and it won't roll over for a few thousand
years. Rounding error causes inaccuracy to build up though.

## Interrupts

GERT can service interrupts at any time, even when Go's garbage colector
has stopped the world to scan stacks. This is because the GERT interrupt
handler is effectively a secret to the rest of the runtime. In the
interrupt handler **there can be no blocking operations like channel
loads, or heap allocations with make()**. This isn't very bad, just toggle
a global flag for other goroutines to use. You can still interact with
peripherals during an interrupt because those are just memory
reads/writes.

## Modification
If the iMX6Quad is not your SOC then you can still use this code but you
will have to write some drivers to port it.

Write a new UART driver in `src/runtime/write_err.go`

Modify `map_kernel()` in `src/runtime/gert_arm.go` to reflect your memory map

Modify `mp_init()` in `src/runtime/gert_arm.go` as well as
`boot_any` in `src/runtime/gertasm_arm.s` to boot your other cpus

