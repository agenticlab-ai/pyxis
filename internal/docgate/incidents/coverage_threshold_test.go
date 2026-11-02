package incidents

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAtomicWriteSuccessAndFailures(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "incidents.md")
	if err := atomicWrite(path, []byte("complete")); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil || string(got) != "complete" {
		t.Fatalf("written bytes = %q, %v", got, err)
	}
	if err := atomicWrite(filepath.Join(root, "missing", "incidents.md"), nil); err == nil {
		t.Fatal("atomicWrite accepted a missing parent directory")
	}
	targetDir := filepath.Join(root, "target-dir")
	if err := os.Mkdir(targetDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := atomicWrite(targetDir, []byte("cannot replace a directory")); err == nil {
		t.Fatal("atomicWrite unexpectedly replaced a directory")
	}
}
