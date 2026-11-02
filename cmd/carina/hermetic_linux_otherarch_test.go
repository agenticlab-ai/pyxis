//go:build linux && !amd64 && !arm64

package main

import "syscall"

// A Linux arch without a syscall table here: the tracer compiles but the proof
// skips (hermeticArchSupported is false). Add the arch's socket-family numbers to
// enable it. syscallNumber is never called on this path.

const hermeticArchSupported = false

func syscallNumber(*syscall.PtraceRegs) uint64 { return 0 }

func networkSyscallSet() map[uint64]string { return map[uint64]string{} }
