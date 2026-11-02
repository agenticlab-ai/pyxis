//go:build !windows

package reviewobligation

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestFileLockFailureAndContentionPaths(t *testing.T) {
	root := t.TempDir()
	blocked := filepath.Join(root, "blocked")
	if err := os.WriteFile(blocked, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := lockExclusive(filepath.Join(blocked, "lock")); err == nil {
		t.Fatal("lockExclusive accepted a file parent")
	}
	if _, err := tryLockExclusive(filepath.Join(blocked, "lock")); err == nil {
		t.Fatal("tryLockExclusive accepted a file parent")
	}
	dirTarget := filepath.Join(root, "target")
	if err := os.Mkdir(dirTarget, 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := lockExclusive(dirTarget); err == nil {
		t.Fatal("lockExclusive opened a directory")
	}
	if _, err := tryLockExclusive(dirTarget); err == nil {
		t.Fatal("tryLockExclusive opened a directory")
	}

	path := filepath.Join(root, "held.lock")
	held, err := lockExclusive(path)
	if err != nil {
		t.Fatal(err)
	}
	defer held.Close()
	if _, err := tryLockExclusive(path); !errors.Is(err, ErrAttemptActive) {
		t.Fatalf("contended lock error = %v", err)
	}
	var nilLock *fileLock
	if err := nilLock.Close(); err != nil {
		t.Fatalf("nil close: %v", err)
	}
}

func TestSyncDirectorySuccessAndMissingPath(t *testing.T) {
	if err := syncDirectory(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	if err := syncDirectory(filepath.Join(t.TempDir(), "missing")); err == nil {
		t.Fatal("syncDirectory accepted a missing directory")
	}
	if err := syncDirectory("/dev/null"); err == nil {
		t.Fatal("syncDirectory unexpectedly synced a device")
	}
}
