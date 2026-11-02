package codegatemirror

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestUnbornHistoryCannotReuseAGreenVerdict(t *testing.T) {
	dir := t.TempDir()
	cmd := exec.Command("git", "init", "-q", dir)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, out)
	}
	writeF(t, dir, ".gitignore", "counter.log\n")
	cfg := Config{Checks: []Check{{Name: "counter", Command: "sh -c 'echo ran >> counter.log'", Class: ClassRequired}}}
	for range 2 {
		rep, hit, err := cfg.RunCached(t.Context(), dir)
		if err != nil || hit || !rep.Passed {
			t.Fatalf("unborn gate = %+v, %v, %v", rep, hit, err)
		}
	}
	if countRuns(t, dir) != 2 {
		t.Fatal("an unresolved history input was treated as cacheable")
	}
	// A malformed Git index must likewise run the check instead of trusting
	// the cache. This exercises the temporary-index preparation failure path.
	if err := os.WriteFile(filepath.Join(dir, ".git/index"), []byte("not an index"), 0o644); err != nil {
		t.Fatal(err)
	}
	if rep, hit, err := cfg.RunCached(t.Context(), dir); err != nil || hit || !rep.Passed {
		t.Fatalf("corrupt-index gate = %+v, %v, %v", rep, hit, err)
	}
}

func TestFingerprintTracksHistoryAndEnvironmentValues(t *testing.T) {
	dir := gitFixture(t)
	gd := mustGitDir(t, dir)
	cfg := Config{Checks: []Check{{Name: "memory", Command: "true", Env: []string{"PYXIS_CI_BASE"}}}}
	fingerprint := func() string {
		t.Helper()
		fp, err := cfg.fingerprint(t.Context(), dir, gd)
		if err != nil {
			t.Fatal(err)
		}
		return fp
	}
	before := fingerprint()
	git := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@e", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@e")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}
	git("update-ref", "refs/remotes/origin/main", "HEAD")
	if fingerprint() == before {
		t.Fatal("new trusted mainline ref reused old verdict")
	}
	before = fingerprint()
	git("-c", "core.hooksPath=/dev/null", "commit", "--allow-empty", "-qm", "same content, new history")
	if fingerprint() == before {
		t.Fatal("same-tree HEAD change reused old verdict")
	}
	before = fingerprint()
	git("update-ref", "refs/remotes/origin/main", "HEAD")
	if fingerprint() == before {
		t.Fatal("advanced trusted base reused old verdict")
	}
	t.Setenv("PYXIS_CI_BASE", "old")
	before = fingerprint()
	t.Setenv("PYXIS_CI_BASE", "new")
	if fingerprint() == before {
		t.Fatal("changed base environment reused old verdict")
	}
}

func TestMemoryHistoryAvailabilityNeverUsesCachedGreen(t *testing.T) {
	dir := gitFixture(t)
	writeF(t, dir, ".gitignore", "counter.log\n")
	cfg := Config{Checks: []Check{{Name: "memory", Command: "sh -c 'echo ran >> counter.log'", Class: ClassRequired}}}
	for range 2 {
		rep, hit, err := cfg.RunCached(t.Context(), dir)
		if err != nil || hit || !rep.Passed {
			t.Fatalf("memory replay: %+v %v %v", rep, hit, err)
		}
	}
	if countRuns(t, dir) != 2 {
		t.Fatal("history availability was trusted from a tree/ref cache")
	}
}
