package main

import (
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestRunGate_CacheAndNoCache proves runGate reuses a cached green verdict on an
// unchanged tree (the check runs once, not once per invocation) and that
// --no-cache forces a fresh run. The check appends to counter.log (gitignored, so
// running it does not perturb the content fingerprint).
func TestRunGate_CacheAndNoCache(t *testing.T) {
	dir := gitGateRepo(t)
	runs := func() int {
		data, err := os.ReadFile(filepath.Join(dir, "counter.log"))
		if err != nil {
			return 0
		}
		return strings.Count(string(data), "\n")
	}

	if code := runGate(dir, false, io.Discard, io.Discard); code != exitPass {
		t.Fatalf("run 1: exit %d, want pass", code)
	}
	if got := runs(); got != 1 {
		t.Fatalf("run 1: check ran %d times, want 1", got)
	}

	if code := runGate(dir, false, io.Discard, io.Discard); code != exitPass {
		t.Fatalf("run 2: exit %d, want pass", code)
	}
	if got := runs(); got != 1 {
		t.Fatalf("run 2: cache miss — the check ran again (%d), want a hit (still 1)", got)
	}

	if code := runGate(dir, true, io.Discard, io.Discard); code != exitPass {
		t.Fatalf("--no-cache: exit %d, want pass", code)
	}
	if got := runs(); got != 2 {
		t.Fatalf("--no-cache did not bypass the cache: the check ran %d times, want 2", got)
	}
}

// gitGateRepo builds an isolated git repo carrying a .pyxis/config.yaml whose one
// check appends to counter.log (gitignored).
func gitGateRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	git := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@e",
			"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@e")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}
	write := func(rel, content string) {
		t.Helper()
		full := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	git("init", "-q")
	write(".gitignore", "counter.log\n")
	write(".pyxis/config.yaml", "code-gate:\n  counter:\n    command: sh -c 'echo ran >> counter.log'\n")
	write("a.txt", "hi\n")
	git("add", "-A")
	git("commit", "-qm", "init")
	return dir
}
