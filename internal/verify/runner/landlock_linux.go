//go:build linux

package runner

import "syscall"

// prSetNoNewPrivs is linux/prctl.h's PR_SET_NO_NEW_PRIVS option, required
// before landlock_restrict_self for an unprivileged process (else EPERM):
// without it, Landlock could be used to make a setuid program believe it
// runs confined when it does not. The value is architecture-invariant (a
// prctl "option" selector, not a per-arch syscall-numbered symbol) —
// confirmed against every arch this Go toolchain's zerrors_linux_*.go
// exposes it for (arm64, loong64, mips*, ppc64*, riscv64, s390x: all 0x26).
const prSetNoNewPrivs = 0x26

// oPath is linux/fcntl.h's O_PATH, architecture-invariant (0x200000 on every
// Linux architecture) but — per this Go toolchain's own
// src/syscall/exec_linux_test.go — "not defined in syscall for 386||amd64",
// so it is declared here directly rather than relied on from the stdlib.
const oPath = 0x200000

// Only this adapter binds the ruleset protocol to a real Linux kernel.
func nativeLandlockKernel() landlockKernel {
	return landlockKernel{
		syscall:  syscall.Syscall,
		syscall6: syscall.Syscall6,
		openPath: func(path string) (int, error) {
			return syscall.Open(path, syscall.O_DIRECTORY|syscall.O_CLOEXEC|oPath, 0)
		},
		close: syscall.Close,
		noNewPrivs: func() error {
			if _, _, errno := syscall.Syscall6(syscall.SYS_PRCTL, prSetNoNewPrivs, 1, 0, 0, 0, 0); errno != 0 {
				return errno
			}
			return nil
		},
	}
}
