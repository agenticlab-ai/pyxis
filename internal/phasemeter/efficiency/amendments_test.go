package efficiency_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/phasemeter/efficiency"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// A ledger mirroring docs/tech-design/prd-amendment-model.md §3's layout:
// 000-base (excluded), two amendments, one rebase checkpoint, plus the
// generated projection (excluded — no numeric prefix).
func TestAmendmentCount(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "000-base.md"), "base")
	writeFile(t, filepath.Join(dir, "001-widen-scope.md"), "amendment 1")
	writeFile(t, filepath.Join(dir, "002-fix-typo.md"), "amendment 2")
	writeFile(t, filepath.Join(dir, "003-rebase.md"), "rebase checkpoint")
	writeFile(t, filepath.Join(dir, "effective.md"), "GENERATED")
	writeFile(t, filepath.Join(dir, "effective.source.json"), "{}")

	got, err := efficiency.AmendmentCount(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got != 3 {
		t.Errorf("AmendmentCount = %d, want 3 (001, 002, 003-rebase; not 000-base or effective.*)", got)
	}
}

func TestAmendmentCountBaseOnly(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "000-base.md"), "base")

	got, err := efficiency.AmendmentCount(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got != 0 {
		t.Errorf("AmendmentCount = %d, want 0", got)
	}
}

// A feature that was never amended has no ledger dir at all yet in some
// callers' framing; that reads as zero, not a fault.
func TestAmendmentCountMissingDir(t *testing.T) {
	got, err := efficiency.AmendmentCount(filepath.Join(t.TempDir(), "does-not-exist"))
	if err != nil {
		t.Fatalf("a missing ledger dir must not error, got %v", err)
	}
	if got != 0 {
		t.Errorf("AmendmentCount = %d, want 0", got)
	}
}
