package baseline

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCoverage90AtomicWriteFilesystemBoundaries(t *testing.T) {
	b := Capture(nil, "v", "operator", "hash", time.Unix(1, 0))

	t.Run("parent is a file", func(t *testing.T) {
		root := t.TempDir()
		parent := filepath.Join(root, "occupied")
		if err := os.WriteFile(parent, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := b.WriteAtomicToFile(filepath.Join(parent, "baseline.json")); err == nil {
			t.Fatal("write under file parent succeeded")
		}
	})

	t.Run("destination is a directory", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "baseline.json")
		if err := os.Mkdir(path, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := b.WriteAtomicToFile(path); err == nil {
			t.Fatal("rename over destination directory succeeded")
		}
	})
}

func TestCoverage90AtomicWriteReportsTemporaryWriteFailure(t *testing.T) {
	path := filepath.Join(t.TempDir(), "read-only")
	if err := os.WriteFile(path, []byte("existing"), 0o600); err != nil {
		t.Fatal(err)
	}
	readOnly, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	old := baselineCreateTemp
	baselineCreateTemp = func(string, string) (*os.File, error) { return readOnly, nil }
	t.Cleanup(func() {
		baselineCreateTemp = old
		_ = readOnly.Close()
	})
	b := Capture(nil, "v", "operator", "hash", time.Unix(1, 0))
	if err := b.WriteAtomicToFile(filepath.Join(t.TempDir(), "baseline.json")); err == nil {
		t.Fatal("write through a read-only temporary file succeeded")
	}
}
