package relmanifest

import (
	"path/filepath"
	"testing"
)

func TestCoverage90HashFileRejectsMissingPath(t *testing.T) {
	if _, err := hashFile(filepath.Join(t.TempDir(), "missing")); err == nil {
		t.Fatal("hashFile accepted a missing path")
	}
}
