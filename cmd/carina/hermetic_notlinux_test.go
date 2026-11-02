//go:build !linux

package main

import "testing"

// The hermetic syscall-trace proof uses ptrace, which is Linux-only. On other
// platforms it skips with a stated reason. TestOfflineLifecycle separately
// verifies offline operation under network denial on macOS; it does not claim
// to detect attempted network syscalls. The tracer remains runnable on Linux.
func TestHermeticLifecycle(t *testing.T) {
	t.Skip("hermetic syscall trace is Linux-only (ptrace); run it on Linux to verify FR-17/DINV-7")
}
