package incidents

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCoverage90AtomicWriteFailureCleansTemporaryFile(t *testing.T) {
	root := t.TempDir()
	readonly := filepath.Join(root, "readonly.tmp")
	if err := os.WriteFile(readonly, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	prior := incidentCreateTemp
	t.Cleanup(func() { incidentCreateTemp = prior })
	incidentCreateTemp = func(string, string) (*os.File, error) { return os.Open(readonly) }
	if err := atomicWrite(filepath.Join(root, "incidents.md"), []byte("new")); err == nil {
		t.Fatal("atomicWrite ignored temporary-file write failure")
	}
	if _, err := os.Stat(readonly); !os.IsNotExist(err) {
		t.Fatalf("failed temporary file remains: %v", err)
	}
}
