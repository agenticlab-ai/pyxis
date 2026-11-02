//go:build linux && arm64

package main

import "syscall"

// arm64 uses the asm-generic syscall table: the syscall number is in register
// x8 (Regs[8]) and preserved across the call, so a syscall-stop read yields it
// at either entry or exit. The network (socket-family) syscalls are a
// contiguous block plus the later accept4/recvmmsg/sendmmsg additions.

const hermeticArchSupported = true

func syscallNumber(regs *syscall.PtraceRegs) uint64 { return regs.Regs[8] }

func networkSyscallSet() map[uint64]string {
	return map[uint64]string{
		198: "socket",
		199: "socketpair",
		200: "bind",
		201: "listen",
		202: "accept",
		203: "connect",
		204: "getsockname",
		205: "getpeername",
		206: "sendto",
		207: "recvfrom",
		208: "setsockopt",
		209: "getsockopt",
		210: "shutdown",
		211: "sendmsg",
		212: "recvmsg",
		242: "accept4",
		243: "recvmmsg",
		269: "sendmmsg",
	}
}
