package codex

import "testing"

func TestCoverage90EmptyPathIsNotArtifact(t *testing.T) {
	if isDocArtifact("") {
		t.Fatal("empty path classified as an artifact")
	}
}
