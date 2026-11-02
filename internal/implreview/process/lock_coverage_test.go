//go:build !windows

package process

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestProcessLockFailureContentionAndNilClose(t *testing.T) {
	root := t.TempDir()
	blocked := filepath.Join(root, "blocked")
	if err := os.WriteFile(blocked, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := tryLockExclusive(filepath.Join(blocked, "lock")); err == nil {
		t.Fatal("accepted a file parent")
	}
	dirTarget := filepath.Join(root, "target")
	if err := os.Mkdir(dirTarget, 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := tryLockExclusive(dirTarget); err == nil {
		t.Fatal("opened a directory as a lock")
	}
	path := filepath.Join(root, "held.lock")
	held, err := tryLockExclusive(path)
	if err != nil {
		t.Fatal(err)
	}
	defer held.Close()
	if _, err := tryLockExclusive(path); !errors.Is(err, ErrProcessActive) {
		t.Fatalf("contention error = %v", err)
	}
	var nilLock *fileLock
	if err := nilLock.Close(); err != nil {
		t.Fatalf("nil close: %v", err)
	}
}

func TestProcessLockReportsUnlockFailure(t *testing.T) {
	lock, err := tryLockExclusive(filepath.Join(t.TempDir(), "lock"))
	if err != nil {
		t.Fatal(err)
	}
	if err := lock.file.Close(); err != nil {
		t.Fatal(err)
	}
	if err := lock.Close(); err == nil {
		t.Fatal("unlocking a closed lock file succeeded")
	}
}
