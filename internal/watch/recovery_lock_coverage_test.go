//go:build !windows

package watch

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRecoveryLockFailureAndNilClosePaths(t *testing.T) {
	root := t.TempDir()
	blocked := filepath.Join(root, "blocked")
	if err := os.WriteFile(blocked, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := lockRecoveryLedger(filepath.Join(blocked, "lock")); err == nil {
		t.Fatal("lock accepted a file as its parent directory")
	}
	dirTarget := filepath.Join(root, "target")
	if err := os.Mkdir(dirTarget, 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := lockRecoveryLedger(dirTarget); err == nil {
		t.Fatal("lock opened a directory as its lock file")
	}
	var lock *recoveryFileLock
	if err := lock.Close(); err != nil {
		t.Fatalf("nil close: %v", err)
	}
}
