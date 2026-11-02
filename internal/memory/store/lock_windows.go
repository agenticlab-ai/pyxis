//go:build windows

package store

import (
	"errors"
	"os"
	"syscall"
	"unsafe"
)

var memoryLockFileEx = syscall.NewLazyDLL("kernel32.dll").NewProc("LockFileEx")

func tryMemoryLock(f *os.File) (bool, error) {
	var overlapped syscall.Overlapped
	result, _, err := memoryLockFileEx.Call(f.Fd(), 3, 0, 1, 0, uintptr(unsafe.Pointer(&overlapped)))
	if result != 0 {
		return true, nil
	}
	if errors.Is(err, syscall.Errno(33)) {
		return false, nil
	} // ERROR_LOCK_VIOLATION
	return false, err
}
