package codegatemirror

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/executor/runtime"
)

// gitFixture builds an isolated git repo with a .gitignore (ignoring `ignored/`
// and `counter.log` — the observable-check counter) and one committed file.
func gitFixture(t *testing.T) string {
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
	git("init", "-q")
	writeF(t, dir, ".gitignore", "ignored/\ncounter.log\n")
	writeF(t, dir, "a.txt", "hello\n")
	git("add", "-A")
	git("commit", "-qm", "init")
	return dir
}

func writeF(t *testing.T, dir, rel, content string) {
	t.Helper()
	full := filepath.Join(dir, rel)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func mustGitDir(t *testing.T, dir string) string {
	t.Helper()
	gd, err := gitDir(dir)
	if err != nil {
		t.Fatalf("gitDir: %v", err)
	}
	return gd
}

// TestFingerprint pins the correctness property the cache rests on: the
// fingerprint changes iff a check input changes.
func TestFingerprint(t *testing.T) {
	dir := gitFixture(t)
	gd := mustGitDir(t, dir)
	cfg := Config{Checks: []Check{{Name: "x", Command: "true", FailOn: FailOnExit, Class: ClassRequired}}}
	fp := func() string {
		s, err := cfg.fingerprint(context.Background(), dir, gd)
		if err != nil {
			t.Fatalf("fingerprint: %v", err)
		}
		return s
	}

	base := fp()
	if fp() != base {
		t.Fatal("same tree changed the fingerprint")
	}

	writeF(t, dir, "ignored/secret.txt", "x")
	if fp() != base {
		t.Fatal("a gitignored file changed the fingerprint (must not)")
	}

	writeF(t, dir, "a.txt", "changed\n")
	afterEdit := fp()
	if afterEdit == base {
		t.Fatal("editing tracked content did not change the fingerprint")
	}

	writeF(t, dir, "b.txt", "new\n")
	if fp() == afterEdit {
		t.Fatal("an untracked non-ignored file did not change the fingerprint")
	}

	cfg2 := Config{Checks: []Check{{Name: "y", Command: "true", FailOn: FailOnExit, Class: ClassRequired}}}
	s2, err := cfg2.fingerprint(context.Background(), dir, gd)
	if err != nil {
		t.Fatal(err)
	}
	if s2 == fp() {
		t.Fatal("a config check change did not change the fingerprint")
	}

	cfg2.Checks[0].Name = cfg.Checks[0].Name
	cfg2.Checks[0].Env = []string{"LOCAL_CHECK_ENV"}
	s2, err = cfg2.fingerprint(context.Background(), dir, gd)
	if err != nil {
		t.Fatal(err)
	}
	if s2 == fp() {
		t.Fatal("a check environment change did not change the fingerprint")
	}
}

func TestVerdictCacheRejectsMalformedDataAndSwallowsWriteFailure(t *testing.T) {
	gitDir := t.TempDir()
	path := cachePath(gitDir)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("not-json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, ok := loadVerdict(gitDir); ok {
		t.Fatal("malformed verdict cache was accepted")
	}
	readOnlyPath := filepath.Join(t.TempDir(), "read-only")
	if err := os.WriteFile(readOnlyPath, []byte("existing"), 0o600); err != nil {
		t.Fatal(err)
	}
	readOnly, err := os.Open(readOnlyPath)
	if err != nil {
		t.Fatal(err)
	}
	old := cacheCreateTemp
	cacheCreateTemp = func(string, string) (*os.File, error) { return readOnly, nil }
	t.Cleanup(func() {
		cacheCreateTemp = old
		_ = readOnly.Close()
	})
	writeVerdict(gitDir, cachedVerdict{Schema: verdictSchema, Fingerprint: "new", Passed: true})
}

// TestFingerprint_CacheDoesNotPollute proves the verdict cache (under the git dir)
// is outside the fingerprint's inputs — writing it never changes the fingerprint.
func TestFingerprint_CacheDoesNotPollute(t *testing.T) {
	dir := gitFixture(t)
	gd := mustGitDir(t, dir)
	cfg := Config{Checks: []Check{{Name: "x", Command: "true", FailOn: FailOnExit, Class: ClassRequired}}}
	before, err := cfg.fingerprint(context.Background(), dir, gd)
	if err != nil {
		t.Fatal(err)
	}
	writeVerdict(gd, cachedVerdict{Schema: verdictSchema, Fingerprint: before, Passed: true})
	after, err := cfg.fingerprint(context.Background(), dir, gd)
	if err != nil {
		t.Fatal(err)
	}
	if after != before {
		t.Fatal("writing the verdict cache changed the fingerprint")
	}
}

// countRuns counts how many times the observable check ran (one line per run).
func countRuns(t *testing.T, dir string) int {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, "counter.log"))
	if err != nil {
		return 0
	}
	return len(nonBlankLines(string(data)))
}

// TestRunCached_HitMiss is the load-bearing correctness test: a green tree is
// cached, an UNCHANGED tree hits (the check never re-runs), and an edited tree
// misses (a stale reuse here would reintroduce the very drift code-gate fixes).
// The check appends to counter.log (gitignored, so running it does not perturb
// the fingerprint).
func TestRunCached_HitMiss(t *testing.T) {
	dir := gitFixture(t)
	cfg := Config{Checks: []Check{{
		Name: "counter", Command: "sh -c 'echo ran >> counter.log'",
		FailOn: FailOnExit, Class: ClassRequired,
	}}}

	rep, hit, err := cfg.RunCached(context.Background(), dir)
	if err != nil || !rep.Passed || hit {
		t.Fatalf("first run: rep=%+v hit=%v err=%v — want pass + miss", rep, hit, err)
	}
	if got := countRuns(t, dir); got != 1 {
		t.Fatalf("first run: check ran %d times, want 1", got)
	}

	rep, hit, err = cfg.RunCached(context.Background(), dir)
	if err != nil || !rep.Passed || !hit {
		t.Fatalf("second run (unchanged): rep=%+v hit=%v err=%v — want pass + HIT", rep, hit, err)
	}
	if got := countRuns(t, dir); got != 1 {
		t.Fatalf("second run: check ran again (%d) — a cache hit must not re-run it", got)
	}

	writeF(t, dir, "a.txt", "changed\n")
	rep, hit, err = cfg.RunCached(context.Background(), dir)
	if err != nil || !rep.Passed || hit {
		t.Fatalf("after edit: rep=%+v hit=%v err=%v — want pass + miss", rep, hit, err)
	}
	if got := countRuns(t, dir); got != 2 {
		t.Fatalf("after edit: check ran %d times, want 2 (a fresh run on the changed tree)", got)
	}
}

func TestProtectedRunRejectsUnconfinedCachedVerdict(t *testing.T) {
	dir := gitFixture(t)
	cfg := Config{Checks: []Check{{Name: "counter", Command: "sh -c 'echo ran >> counter.log'", FailOn: FailOnExit, Class: ClassRequired}}}
	if rep, _, err := cfg.RunCached(t.Context(), dir); err != nil || !rep.Passed {
		t.Fatalf("legacy run: %+v, %v", rep, err)
	}
	ctx, cleanup, err := runtime.VerificationContext(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	if rep, hit, err := cfg.RunCached(ctx, dir); err != nil || !rep.Passed || hit {
		t.Fatalf("protected run reused legacy cache: %+v, %v, %v", rep, hit, err)
	}
	if countRuns(t, dir) != 2 {
		t.Fatal("protected run did not execute the check")
	}
	if rep, hit, err := cfg.RunCached(ctx, dir); err != nil || !rep.Passed || !hit {
		t.Fatalf("protected repeat lost its cache: %+v, %v, %v", rep, hit, err)
	}
}

// TestRunCached_OnlyGreenCached proves a RED gate is never cached — it re-runs
// every time so its fresh output is always shown.
func TestRunCached_OnlyGreenCached(t *testing.T) {
	dir := gitFixture(t)
	cfg := Config{Checks: []Check{{
		Name: "red", Command: "sh -c 'echo ran >> counter.log; false'",
		FailOn: FailOnExit, Class: ClassRequired,
	}}}
	for i := 1; i <= 2; i++ {
		rep, hit, err := cfg.RunCached(context.Background(), dir)
		if err != nil || rep.Passed || hit {
			t.Fatalf("run %d: rep=%+v hit=%v err=%v — a red gate is never cached", i, rep, hit, err)
		}
	}
	if got := countRuns(t, dir); got != 2 {
		t.Fatalf("red gate ran %d times, want 2 (never cached)", got)
	}
}

// TestRunCached_FailSafeNonGit proves a non-git workspace degrades to a fresh Run
// (no fingerprint, no cache) rather than erroring or failing closed.
func TestRunCached_FailSafeNonGit(t *testing.T) {
	dir := t.TempDir() // not a git repo
	cfg := Config{Checks: []Check{{Name: "x", Command: "true", FailOn: FailOnExit, Class: ClassRequired}}}
	rep, hit, err := cfg.RunCached(context.Background(), dir)
	if err != nil {
		t.Fatalf("non-git: RunCached returned an error, want a fail-safe fallback: %v", err)
	}
	if hit {
		t.Fatal("non-git: reported a cache hit, want a fresh run")
	}
	if !rep.Passed {
		t.Fatalf("non-git: gate should have run and passed, got %+v", rep)
	}
}

func TestWriteVerdictIsFailSafeWhenCacheDirectoryIsBlocked(t *testing.T) {
	root := t.TempDir()
	blocked := filepath.Join(root, "blocked")
	if err := os.WriteFile(blocked, []byte("file"), 0o644); err != nil {
		t.Fatal(err)
	}
	writeVerdict(filepath.Join(blocked, "git"), cachedVerdict{Schema: verdictSchema})
}
