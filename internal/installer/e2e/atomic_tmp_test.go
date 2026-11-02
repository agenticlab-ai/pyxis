package e2e

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCrashCleanupOnlyDropsRegularAtomicWriteTemps(t *testing.T) {
	root := t.TempDir()
	remove := []string{".pyxis-converge-123", "nested/.pyxis-converge-4294967295"}
	keep := []string{"payload.md", ".pyxis-converge-not-numeric", ".pyxis-converge-4294967296"}
	for _, path := range append(remove, keep...) {
		writeFile(t, filepath.Join(root, path), []byte("fixture"), 0o644)
	}
	if err := os.Mkdir(filepath.Join(root, ".pyxis-converge-456"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("payload.md", filepath.Join(root, ".pyxis-converge-789")); err != nil {
		t.Fatal(err)
	}
	dropAtomicWriteTemps(t, root)
	for _, path := range remove {
		if _, err := os.Lstat(filepath.Join(root, path)); !os.IsNotExist(err) {
			t.Fatalf("atomic temp survived: %s %v", path, err)
		}
	}
	for _, path := range append(keep, ".pyxis-converge-456", ".pyxis-converge-789") {
		if _, err := os.Lstat(filepath.Join(root, path)); err != nil {
			t.Fatalf("ordinary artifact lost: %s %v", path, err)
		}
	}
}
