package workspace

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestEnsureGuardReportsGitAndDirectoryFailures(t *testing.T) {
	if _, err := (&Workspace{parent: t.TempDir()}).commonGitDir(t.Context()); err == nil {
		t.Fatal("common Git directory outside a repository: want error")
	}
	if err := (&Workspace{parent: t.TempDir()}).ensureGuard(t.Context()); err == nil {
		t.Fatal("ensureGuard outside a repository: want error")
	}

	repo := t.TempDir()
	if out, err := exec.Command("git", "-C", repo, "init", "-q").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, out)
	}
	common, err := (&Workspace{parent: repo}).commonGitDir(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	guardPath := filepath.Join(common, guardDirName)
	if err := os.WriteFile(guardPath, []byte("occupied"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := (&Workspace{parent: repo}).ensureGuard(t.Context()); err == nil {
		t.Fatal("ensureGuard with occupied hook directory: want error")
	}
	if err := os.Remove(guardPath); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(guardPath, "pre-commit"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := (&Workspace{parent: repo}).ensureGuard(t.Context()); err == nil {
		t.Fatal("ensureGuard with a directory in place of a hook: want error")
	}
}
