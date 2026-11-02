//go:build windows

package reviewobligation

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"unsafe"
)

const (
	lockfileFailImmediately = 0x00000001
	lockfileExclusiveLock   = 0x00000002
	errorLockViolation      = syscall.Errno(33)
)

var (
	kernel32         = syscall.NewLazyDLL("kernel32.dll")
	procLockFileEx   = kernel32.NewProc("LockFileEx")
	procUnlockFileEx = kernel32.NewProc("UnlockFileEx")
)

type fileLock struct {
	f          *os.File
	overlapped syscall.Overlapped
}

func lockExclusive(path string) (*fileLock, error) {
	return acquireWindowsLock(path, false)
}

func tryLockExclusive(path string) (*fileLock, error) {
	return acquireWindowsLock(path, true)
}

func acquireWindowsLock(path string, nonblocking bool) (*fileLock, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("review obligation: create lock directory: %w", err)
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("review obligation: open lock: %w", err)
	}
	lock := &fileLock{f: f}
	flags := uintptr(lockfileExclusiveLock)
	if nonblocking {
		flags |= lockfileFailImmediately
	}
	result, _, callErr := procLockFileEx.Call(f.Fd(), flags, 0, 1, 0, uintptr(unsafe.Pointer(&lock.overlapped)))
	if result == 0 {
		_ = f.Close()
		if nonblocking && errors.Is(callErr, errorLockViolation) {
			return nil, ErrAttemptActive
		}
		return nil, fmt.Errorf("review obligation: LockFileEx: %w", callErr)
	}
	return lock, nil
}

func (l *fileLock) Close() error {
	if l == nil || l.f == nil {
		return nil
	}
	result, _, callErr := procUnlockFileEx.Call(l.f.Fd(), 0, 1, 0, uintptr(unsafe.Pointer(&l.overlapped)))
	closeErr := l.f.Close()
	if result == 0 {
		return fmt.Errorf("review obligation: UnlockFileEx: %w", callErr)
	}
	return closeErr
}

func syncDirectory(string) error {
	// Windows makes a newly created file's directory entry durable as part of
	// FlushFileBuffers on the file handle; opening directories for flush is not
	// supported through os.File.
	return nil
}
