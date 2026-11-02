package reporoot

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestFromUsesRequestedGitDirectory(t *testing.T) {
	root := t.TempDir()
	if out, err := exec.Command("git", "init", "-q", root).CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, out)
	}
	nested := filepath.Join(root, "nested")
	if err := os.Mkdir(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	want, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	// The test process remains in this source repository. The requested
	// directory belongs to a different repository with no Pyxis config.
	if got, err := From(nested); err != nil || got != want {
		t.Fatalf("From(%s) = %q, %v; want %s", nested, got, err, want)
	}
}
