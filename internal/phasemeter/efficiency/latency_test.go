package efficiency_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/phasemeter/efficiency"
)

// newGitRepo stands up a throwaway git repo with a fixed identity, so
// commit timestamps are exercised deterministically with no ambient git
// config and no wall clock (the hermetic-test discipline; mirrors
// internal/docgate/feedback/adapters/gitrepo's test fixture).
func newGitRepo(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	dir := t.TempDir()
	runGit(t, dir, "2026-07-01T00:00:00Z", "init", "-q", "-b", "main")
	return dir
}

func runGit(t *testing.T, dir, date string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t",
		"GIT_AUTHOR_DATE="+date, "GIT_COMMITTER_DATE="+date)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func commitAt(t *testing.T, dir, path, content, date string) {
	t.Helper()
	full := filepath.Join(dir, path)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, date, "add", path)
	runGit(t, dir, date, "commit", "-q", "-m", "commit "+path+" at "+date)
}

func TestLatencySpansFirstToLastCommit(t *testing.T) {
	dir := newGitRepo(t)
	commitAt(t, dir, "docs/brainstorms/foo.md", "draft\n", "2026-07-01T00:00:00Z")
	commitAt(t, dir, "docs/brainstorms/foo.md", "revised\n", "2026-07-01T02:00:00Z")

	got, err := efficiency.Latency(dir, []string{"docs/brainstorms/foo.md"})
	if err != nil {
		t.Fatal(err)
	}
	if want := 2 * time.Hour; got != want {
		t.Errorf("Latency = %v, want %v", got, want)
	}
}

func TestLatencyAcrossMultiplePaths(t *testing.T) {
	dir := newGitRepo(t)
	commitAt(t, dir, "docs/prds/foo/000-base.md", "base\n", "2026-07-01T00:00:00Z")
	commitAt(t, dir, "docs/prds/foo/001-amend.md", "amendment\n", "2026-07-03T00:00:00Z")

	got, err := efficiency.Latency(dir, []string{"docs/prds/foo/000-base.md", "docs/prds/foo/001-amend.md"})
	if err != nil {
		t.Fatal(err)
	}
	if want := 48 * time.Hour; got != want {
		t.Errorf("Latency = %v, want %v (earliest across all paths to latest across all paths)", got, want)
	}
}

func TestLatencySingleCommitIsZero(t *testing.T) {
	dir := newGitRepo(t)
	commitAt(t, dir, "docs/brainstorms/foo.md", "draft\n", "2026-07-01T00:00:00Z")

	got, err := efficiency.Latency(dir, []string{"docs/brainstorms/foo.md"})
	if err != nil {
		t.Fatal(err)
	}
	if got != 0 {
		t.Errorf("Latency = %v, want 0 for a single-commit (e.g. squash-merged) history", got)
	}
}

func TestLatencyNeverCommittedPathIsZero(t *testing.T) {
	dir := newGitRepo(t)
	commitAt(t, dir, "docs/brainstorms/foo.md", "draft\n", "2026-07-01T00:00:00Z")

	got, err := efficiency.Latency(dir, []string{"docs/brainstorms/never-written.md"})
	if err != nil {
		t.Fatal(err)
	}
	if got != 0 {
		t.Errorf("Latency = %v, want 0 for a path with no git history", got)
	}
}

func TestLatencyNoPaths(t *testing.T) {
	dir := newGitRepo(t)
	got, err := efficiency.Latency(dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got != 0 {
		t.Errorf("Latency = %v, want 0", got)
	}
}
