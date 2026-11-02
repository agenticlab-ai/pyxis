//go:build !windows

package report

import (
	"os"
	"syscall"
)

// The kernel releases flock on close or process exit, including a crash. The
// sidecar must remain in place so future owners lock the same inode.
func lockDeliveryFile(f *os.File) error {
	return syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
}
