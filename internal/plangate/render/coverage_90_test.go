package render

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCoverage90SelectVersionEmptyAndMalformed(t *testing.T) {
	root := t.TempDir()
	empty := "docs/plans/empty"
	if err := os.MkdirAll(filepath.Join(root, filepath.FromSlash(empty)), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, defects := selectVersion(root, empty, ""); len(defects) == 0 {
		t.Fatal("empty plan directory was accepted")
	}

	populated := "docs/plans/populated"
	dir := filepath.Join(root, filepath.FromSlash(populated))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "001-plan.md"), []byte("plan"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, defects := selectVersion(root, populated, "not-a-version"); len(defects) == 0 {
		t.Fatal("malformed version was accepted")
	}
}
