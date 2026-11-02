//go:build !windows

package watch

import (
	"errors"
	"fmt"
	"os"
	"syscall"
)

type recoveryFileLock struct{ file *os.File }

var flockRecovery = flockRecoveryFile

func lockRecoveryLedger(path string) (*recoveryFileLock, error) {
	if err := os.MkdirAll(filepathDir(path), 0o700); err != nil {
		return nil, fmt.Errorf("watch: create recovery lock directory: %w", err)
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("watch: open recovery lock: %w", err)
	}
	if err := flockRecovery(f); err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("watch: lock recovery ledger: %w", err)
	}
	return &recoveryFileLock{file: f}, nil
}

func flockRecoveryFile(f *os.File) error {
	return syscall.Flock(int(f.Fd()), syscall.LOCK_EX)
}

func (l *recoveryFileLock) Close() error {
	if l == nil || l.file == nil {
		return nil
	}
	unlockErr := syscall.Flock(int(l.file.Fd()), syscall.LOCK_UN)
	closeErr := l.file.Close()
	return errors.Join(unlockErr, closeErr)
}
