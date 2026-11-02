//go:build !windows

package watch

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestCoverage90FlockRejectsClosedDescriptor(t *testing.T) {
	f, err := os.OpenFile(filepath.Join(t.TempDir(), "lock"), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	if err := flockRecoveryFile(f); err == nil {
		t.Fatal("flock accepted a closed descriptor")
	}
}

func TestCoverage90RecoveryLockPropagatesFlockFailure(t *testing.T) {
	want := errors.New("flock failed")
	prior := flockRecovery
	flockRecovery = func(*os.File) error { return want }
	t.Cleanup(func() { flockRecovery = prior })
	if _, err := lockRecoveryLedger(filepath.Join(t.TempDir(), "lock")); !errors.Is(err, want) {
		t.Fatalf("flock failure = %v", err)
	}
}
