package watch

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestFileRecoveryLedgerBoundaryFailures(t *testing.T) {
	if _, err := NewFileRecoveryLedger(" \t"); err == nil {
		t.Fatal("accepted an empty recovery ledger path")
	}

	ledger, err := NewFileRecoveryLedger(filepath.Join(t.TempDir(), "recovery.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := ledger.Exact(canceled, "scope"); err == nil {
		t.Fatal("Exact ignored a canceled context")
	}
	if _, err := ledger.Append(canceled, RecoveryEntry{}); err == nil {
		t.Fatal("Append ignored a canceled context")
	}

	if err := os.WriteFile(ledger.path, []byte("not-json\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ledger.Exact(context.Background(), "scope"); err == nil {
		t.Fatal("Exact accepted a malformed ledger row")
	}
}
