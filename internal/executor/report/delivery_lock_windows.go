//go:build windows

package report

import (
	"os"
	"syscall"
	"unsafe"
)

var deliveryLockFileEx = syscall.NewLazyDLL("kernel32.dll").NewProc("LockFileEx")

func lockDeliveryFile(f *os.File) error {
	var overlapped syscall.Overlapped
	// Exclusive, nonblocking byte-range lock. Closing the file handle releases
	// it, and process termination releases it even if Close was never reached.
	result, _, err := deliveryLockFileEx.Call(f.Fd(), 3, 0, 1, 0, uintptr(unsafe.Pointer(&overlapped)))
	if result == 0 {
		return err
	}
	return nil
}
