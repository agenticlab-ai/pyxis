package host

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCoverage90LedgersRejectDeliveryDirectory(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "docs", "problems", "broken", "delivery.md")
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := (Ledgers{Root: root}).List(t.Context()); err == nil {
		t.Fatal("delivery directory was accepted as a ledger file")
	}
}

func TestCoverage90ShellRunSuccessAndExitFailure(t *testing.T) {
	if out, err := shellRun(t.Context(), "go", "version"); err != nil || len(out) == 0 {
		t.Fatalf("shellRun(go version) = %q, %v", out, err)
	}
	if _, err := shellRun(t.Context(), "sh", "-c", "exit 7"); err == nil {
		t.Fatal("shellRun ignored nonzero exit")
	}
}
