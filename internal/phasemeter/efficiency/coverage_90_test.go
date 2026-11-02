package efficiency

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCoverage90ComputePropagatesUnreadableArtifactClasses(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "artifact"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name string
		src  Source
	}{
		{name: "anchor", src: Source{RepoRoot: root, AnchorPaths: []string{"artifact"}}},
		{name: "review", src: Source{RepoRoot: root, ReviewPaths: []string{"artifact"}}},
		{name: "plan", src: Source{RepoRoot: root, PlanPath: "artifact"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := Compute(tc.src); err == nil {
				t.Fatal("Compute accepted a directory as an artifact file")
			}
		})
	}
}

func TestCoverage90AmendmentDirectoryAndLatencyErrors(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "001-directory.md"), 0o755); err != nil {
		t.Fatal(err)
	}
	if got, err := AmendmentCount(root); err != nil || got != 0 {
		t.Fatalf("AmendmentCount = %d, %v", got, err)
	}
	if _, err := Latency(filepath.Join(root, "missing-repo"), []string{"x"}); err == nil {
		t.Fatal("Latency accepted a missing repository")
	}
}
