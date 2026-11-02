package gitrepo_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/docgate/feedback"
	"github.com/agenticlab-ai/pyxis/internal/docgate/feedback/adapters/gitrepo"
)

// newRepo stands up a throwaway git repo with a fixed identity and commit date,
// so the adapter's ancestry/existence reads are exercised deterministically with
// no ambient git config and no clock (the hermetic-test discipline).
func newRepo(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	dir := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t",
			"GIT_AUTHOR_DATE=2026-07-13T00:00:00Z", "GIT_COMMITTER_DATE=2026-07-13T00:00:00Z")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init", "-q", "-b", "main")
	return dir
}

func commit(t *testing.T, dir, path, content, msg string) string {
	t.Helper()
	full := filepath.Join(dir, path)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	env := append(os.Environ(),
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t",
		"GIT_AUTHOR_DATE=2026-07-13T00:00:00Z", "GIT_COMMITTER_DATE=2026-07-13T00:00:00Z")
	for _, args := range [][]string{{"add", path}, {"commit", "-q", "-m", msg}} {
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		cmd.Env = env
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	out, err := exec.Command("git", "-C", dir, "rev-parse", "HEAD").Output()
	if err != nil {
		t.Fatal(err)
	}
	return string(out[:40])
}

// A recorded edit-commit that is an ancestor of the authoritative ref is
// observed landed, with a stable outcome sha and a non-zero edit size.
func TestEditLandedAncestor(t *testing.T) {
	dir := newRepo(t)
	commit(t, dir, "docs/tech-design/some-doc.md", "line one\n", "seed subject")
	edit := commit(t, dir, "docs/tech-design/some-doc.md", "line one\nline two added\n", "the annotation edit")

	repo := gitrepo.Repo{Worktree: dir, Ref: "HEAD", Subject: "some-doc", SubjectPath: "docs/tech-design/some-doc.md"}
	land, err := repo.EditLanded(edit)
	if err != nil {
		t.Fatal(err)
	}
	if !land.Landed {
		t.Fatal("an ancestor edit-commit must be observed landed")
	}
	if len(land.Outcome) != 40 {
		t.Fatalf("outcome %q is not a resolved sha (DINV-4)", land.Outcome)
	}
	if land.EditSize <= 0 {
		t.Fatalf("edit size = %d, want the numstat changed-line count", land.EditSize)
	}
}

// A commit that is not on the authoritative ref's history (an unmerged /
// squash-orphaned edit) is observed NOT landed — the entry stays pending, never
// a false applied (AP-13/I2, DINV-3).
func TestEditNotLandedOffHistory(t *testing.T) {
	dir := newRepo(t)
	commit(t, dir, "docs/tech-design/some-doc.md", "base\n", "seed")
	// A commit on a side branch that main never absorbs.
	run := func(args ...string) {
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t",
			"GIT_AUTHOR_DATE=2026-07-13T00:00:00Z", "GIT_COMMITTER_DATE=2026-07-13T00:00:00Z")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("checkout", "-q", "-b", "side")
	side := commit(t, dir, "docs/tech-design/some-doc.md", "base\nside\n", "side edit")
	run("checkout", "-q", "main")

	repo := gitrepo.Repo{Worktree: dir, Ref: "main", Subject: "some-doc", SubjectPath: "docs/tech-design/some-doc.md"}
	land, err := repo.EditLanded(side)
	if err != nil {
		t.Fatal(err)
	}
	if land.Landed {
		t.Fatal("an off-history edit must be observed NOT landed (pending), never a false applied")
	}
}

// A subject file present at the ref resolves; a never-committed one does not
// (retained + flagged stale, UB-6).
func TestSubjectResolves(t *testing.T) {
	dir := newRepo(t)
	commit(t, dir, "docs/tech-design/live.md", "x\n", "seed")

	live := gitrepo.Repo{Worktree: dir, Ref: "HEAD", Subject: "live", SubjectPath: "docs/tech-design/live.md"}
	if ok, err := live.SubjectResolves("live#1"); err != nil || !ok {
		t.Fatalf("present subject must resolve: ok=%v err=%v", ok, err)
	}
	gone := gitrepo.Repo{Worktree: dir, Ref: "HEAD", Subject: "gone", SubjectPath: "docs/tech-design/gone.md"}
	if ok, err := gone.SubjectResolves("gone#1"); err != nil || ok {
		t.Fatalf("absent subject must not resolve (stale): ok=%v err=%v", ok, err)
	}
}

// SubjectFinalized is the explicit operator assertion, never inferred: off by
// default (pending), on only when ConcludeNoEdit is set, and then it names a
// stable ref.
func TestSubjectFinalizedIsAsserted(t *testing.T) {
	dir := newRepo(t)
	commit(t, dir, "docs/tech-design/d.md", "x\n", "seed")

	notConcluded := gitrepo.Repo{Worktree: dir, Ref: "HEAD", Subject: "d", SubjectPath: "docs/tech-design/d.md"}
	if fin, _, _ := notConcluded.SubjectFinalized("d"); fin {
		t.Fatal("without --concluded a subject is not finalized (entries stay pending)")
	}
	concluded := notConcluded
	concluded.ConcludeNoEdit = true
	fin, ref, err := concluded.SubjectFinalized("d")
	if err != nil || !fin || len(ref) != 40 {
		t.Fatalf("with --concluded: fin=%v ref=%q err=%v", fin, ref, err)
	}
}

// End-to-end through the port: a captured entry with a landed edit-commit is
// observed applied and its disposition appended to the file on disk.
func TestObserveThroughGitAdapter(t *testing.T) {
	dir := newRepo(t)
	commit(t, dir, "docs/tech-design/some-doc.md", "hello\n", "seed subject")
	edit := commit(t, dir, "docs/tech-design/some-doc.md", "hello world\n", "annotation edit")

	entry := &feedback.Captured{
		Entry: "001", Source: "conversation", SubjectAnchor: "some-doc#1",
		EditCommit: edit, SourceFile: "001-conversation.md",
	}
	repo := gitrepo.Repo{Worktree: dir, Ref: "HEAD", Subject: "some-doc", SubjectPath: "docs/tech-design/some-doc.md"}
	got, err := feedback.Observe([]*feedback.Captured{entry}, repo)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || !got[0].Resolved() || got[0].Disposition.Disposition != "applied" {
		t.Fatalf("want applied via git adapter, got %+v", got)
	}
}

func TestAdapterBoundaryFailures(t *testing.T) {
	t.Run("empty edit is not landed", func(t *testing.T) {
		land, err := (gitrepo.Repo{}).EditLanded(" \t")
		if err != nil || land.Landed {
			t.Fatalf("empty edit: landing=%+v err=%v", land, err)
		}
	})

	t.Run("invalid worktree rejects ancestry read", func(t *testing.T) {
		_, err := (gitrepo.Repo{Worktree: filepath.Join(t.TempDir(), "missing"), Ref: "HEAD"}).EditLanded("deadbeef")
		if err == nil {
			t.Fatal("expected the git ancestry failure to surface")
		}
	})

	t.Run("invalid worktree rejects finalization read", func(t *testing.T) {
		_, _, err := (gitrepo.Repo{
			Worktree:       filepath.Join(t.TempDir(), "missing"),
			Ref:            "HEAD",
			ConcludeNoEdit: true,
		}).SubjectFinalized("subject")
		if err == nil {
			t.Fatal("expected the git ref failure to surface")
		}
	})

	t.Run("missing git executable surfaces invocation failure", func(t *testing.T) {
		t.Setenv("PATH", "")
		_, err := (gitrepo.Repo{Worktree: t.TempDir(), Ref: "HEAD", SubjectPath: "subject.md"}).SubjectResolves("subject")
		if err == nil {
			t.Fatal("expected the git invocation failure to surface")
		}
	})
}
