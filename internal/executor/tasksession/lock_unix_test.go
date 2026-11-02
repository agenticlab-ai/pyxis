//go:build !windows

package tasksession

import (
	"errors"
	"path/filepath"
	"testing"
)

func TestLockLedgerFailsClosedWhenAdvisoryLockFails(t *testing.T) {
	want := errors.New("lock unavailable")
	original := flockLedger
	flockLedger = func(int, int) error { return want }
	t.Cleanup(func() { flockLedger = original })

	lock, err := lockLedger(filepath.Join(t.TempDir(), "ledger.lock"))
	if lock != nil || !errors.Is(err, want) {
		t.Fatalf("lockLedger = %#v, %v; want injected lock failure", lock, err)
	}
}
