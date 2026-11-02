//go:build !windows

package tasksession

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

type ledgerLock struct{ file *os.File }

var flockLedger = syscall.Flock

// lockLedger takes an exclusive advisory lock on the ledger's sibling lock
// file, so concurrent sweeps on one machine serialize their read-modify-append.
func lockLedger(path string) (*ledgerLock, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("tasksession: create lock directory: %w", err)
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("tasksession: open lock: %w", err)
	}
	if err := flockLedger(int(f.Fd()), syscall.LOCK_EX); err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("tasksession: lock ledger: %w", err)
	}
	return &ledgerLock{file: f}, nil
}

func (l *ledgerLock) Close() error {
	if l == nil || l.file == nil {
		return nil
	}
	unlockErr := syscall.Flock(int(l.file.Fd()), syscall.LOCK_UN)
	closeErr := l.file.Close()
	return errors.Join(unlockErr, closeErr)
}
