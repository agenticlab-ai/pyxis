package localfs

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestPredicateScansReportDirectoryErrorsAndQuarantineEntries(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "items"), []byte("not a directory"), 0o600); err != nil {
		t.Fatal(err)
	}
	s := New(root, fixedClock())
	if items, defects := s.ReadySet(context.Background()); items != nil || len(defects) != 1 {
		t.Fatalf("ReadySet = %v, %v", items, defects)
	}
	if items, defects := s.TriageSet(context.Background()); items != nil || len(defects) != 1 {
		t.Fatalf("TriageSet = %v, %v", items, defects)
	}

	root = t.TempDir()
	itemsDir := filepath.Join(root, "items")
	if err := os.MkdirAll(filepath.Join(itemsDir, "subdir"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(itemsDir, "scratch.tmp"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(itemsDir, "1.md"), []byte("malformed"), 0o600); err != nil {
		t.Fatal(err)
	}
	s = New(root, fixedClock())
	if got, defects := s.scanAll("coverage"); len(got) != 0 || len(defects) != 0 {
		t.Fatalf("quarantined scan = %v, %v", got, defects)
	}
}
