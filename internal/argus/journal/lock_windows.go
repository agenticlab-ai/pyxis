//go:build windows

package journal

import (
	"errors"
	"fmt"
	"os"
	"syscall"
	"unsafe"
)

var journalLockFileEx = syscall.NewLazyDLL("kernel32.dll").NewProc("LockFileEx")

func lockJournalFile(file *os.File) error {
	var overlapped syscall.Overlapped
	result, _, err := journalLockFileEx.Call(file.Fd(), 3, 0, 1, 0, uintptr(unsafe.Pointer(&overlapped)))
	if result != 0 {
		return nil
	}
	if errors.Is(err, syscall.Errno(33)) {
		return fmt.Errorf("another journal correction holds the lock; retry after it finishes")
	}
	return err
}
