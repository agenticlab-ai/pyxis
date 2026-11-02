//go:build linux && amd64

package main

import "syscall"

// amd64 keeps the original syscall number in Orig_rax across the call (rax holds
// the return value at exit), so a syscall-stop read yields it at either entry or
// exit. The network (socket-family) syscalls are the 41–55 block plus the later
// accept4/recvmmsg/sendmmsg additions.

const hermeticArchSupported = true

func syscallNumber(regs *syscall.PtraceRegs) uint64 { return regs.Orig_rax }

func networkSyscallSet() map[uint64]string {
	return map[uint64]string{
		41:  "socket",
		42:  "connect",
		43:  "accept",
		44:  "sendto",
		45:  "recvfrom",
		46:  "sendmsg",
		47:  "recvmsg",
		48:  "shutdown",
		49:  "bind",
		50:  "listen",
		51:  "getsockname",
		52:  "getpeername",
		53:  "socketpair",
		54:  "setsockopt",
		55:  "getsockopt",
		288: "accept4",
		299: "recvmmsg",
		307: "sendmmsg",
	}
}
