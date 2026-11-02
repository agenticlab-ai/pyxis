package stamp

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestRegisterRejectsDuplicateAdapter(t *testing.T) {
	const name = "coverage-threshold-test-adapter"
	a := &Adapter{Name: name}
	register(a)
	defer delete(registry, name)

	deferredPanic := func() (got any) {
		defer func() { got = recover() }()
		register(&Adapter{Name: name})
		return nil
	}()
	if deferredPanic == nil || !strings.Contains(deferredPanic.(string), "registered twice") {
		t.Fatalf("duplicate registration panic = %v, want registered-twice diagnostic", deferredPanic)
	}
}

func TestGitBindingRejectsNonRepository(t *testing.T) {
	if _, err := GitBinding(t.TempDir()); err == nil || !strings.Contains(err.Error(), "computing the commit tree") {
		t.Fatalf("GitBinding(non-repository) error = %v, want contextual write-tree failure", err)
	}
}

func TestGitBindingIncludesExistingParent(t *testing.T) {
	repo := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=coverage", "GIT_AUTHOR_EMAIL=coverage@example.invalid",
			"GIT_COMMITTER_NAME=coverage", "GIT_COMMITTER_EMAIL=coverage@example.invalid")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}
	run("init", "-q")
	if err := os.WriteFile(filepath.Join(repo, "tracked"), []byte("parent"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", "tracked")
	run("commit", "-q", "-m", "parent")
	if got, err := GitBinding(repo); err != nil || got == "" {
		t.Fatalf("GitBinding(committed repo) = %q, %v", got, err)
	}
}
