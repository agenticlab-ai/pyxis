//go:build windows

package watch

import (
	"fmt"
	"os"
)

type recoveryFileLock struct{ file *os.File }

func lockRecoveryLedger(path string) (*recoveryFileLock, error) {
	if err := os.MkdirAll(filepathDir(path), 0o700); err != nil {
		return nil, fmt.Errorf("watch: create recovery lock directory: %w", err)
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("watch: open recovery lock: %w", err)
	}
	return &recoveryFileLock{file: f}, nil
}

func (l *recoveryFileLock) Close() error {
	if l == nil || l.file == nil {
		return nil
	}
	return l.file.Close()
}
