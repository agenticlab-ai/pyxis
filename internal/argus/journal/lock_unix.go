//go:build !windows

package journal

import (
	"errors"
	"fmt"
	"os"
	"syscall"
)

func lockJournalFile(file *os.File) error {
	err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
	if errors.Is(err, syscall.EWOULDBLOCK) || errors.Is(err, syscall.EAGAIN) {
		return fmt.Errorf("another journal correction holds the lock; retry after it finishes")
	}
	return err
}
