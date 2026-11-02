package workspace

// These tests exercise the adapter against a REAL temporary git repository — a
// local origin plus a working clone, built commit-by-commit, no network — the
// local-substitutable seam of docs/tech-design/implement-now.md §3.10. They are
// the acceptance harness for T3: worktree isolation + the parent-checkout guard
// (DINV-1), a dependent task based off the freshly-fetched mainline tip
// (DINV-2), and the reaper (Cleanup).

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/executor/port"
)

// --- harness: a real origin + working clone -----------------------------------

type harness struct {
	t      *testing.T
	root   string // t.TempDir(): holds origin, parent and the default worktree root
	origin string // the mainline the parent fetches from
	parent string // the parent checkout the executor operates + guards
	baseA  string // origin's initial mainline tip (commit "A")
}

// newHarness builds a hermetic git world: origin at commit A on main, and a
// parent clone of it. All git identity, dates and config isolation come from the
// process environment (t.Setenv), so commits are deterministic and the host's
// ~/.gitconfig cannot leak in — and the adapter under test, which shells out with
// the same inherited environment, is hermetic too.
func newHarness(t *testing.T) *harness {
	t.Helper()
	t.Setenv("GIT_TERMINAL_PROMPT", "0")
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_AUTHOR_NAME", "pyxis-test")
	t.Setenv("GIT_AUTHOR_EMAIL", "test@pyxis.local")
	t.Setenv("GIT_COMMITTER_NAME", "pyxis-test")
	t.Setenv("GIT_COMMITTER_EMAIL", "test@pyxis.local")
	t.Setenv("GIT_AUTHOR_DATE", "2026-07-16T00:00:00Z")
	t.Setenv("GIT_COMMITTER_DATE", "2026-07-16T00:00:00Z")

	root := t.TempDir()
	origin := filepath.Join(root, "origin")
	parent := filepath.Join(root, "parent")
	if err := os.MkdirAll(origin, 0o755); err != nil {
		t.Fatal(err)
	}
	gitOK(t, origin, "init", "-q", "-b", "main")
	writeFile(t, filepath.Join(origin, "f"), "A\n")
	gitOK(t, origin, "add", "f")
	gitOK(t, origin, "commit", "-q", "-m", "A")
	a := gitOK(t, origin, "rev-parse", "HEAD")
	gitOK(t, root, "clone", "-q", origin, parent)

	return &harness{t: t, root: root, origin: origin, parent: parent, baseA: a}
}

func (h *harness) newWorkspace(t *testing.T, ctx context.Context) *Workspace {
	t.Helper()
	ws, err := New(ctx, Config{Parent: h.parent})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return ws
}

// --- acceptance criterion 1: isolation + parent-checkout guard (DINV-1) --------

func TestPrepare_WorktreeIsolationAndParentGuard(t *testing.T) {
	ctx := context.Background()
	h := newHarness(t)
	ws := h.newWorkspace(t, ctx)

	wd, err := ws.Prepare(ctx, port.Task{ID: "T1", Number: 1, HeadBranch: "feat/iso-1"})
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if wd.Branch() != "feat/iso-1" {
		t.Errorf("Branch() = %q, want feat/iso-1", wd.Branch())
	}
	// The worktree is isolated: a real directory, not the parent, under the
	// adapter's default worktree root (a sibling of the parent, never nested in
	// the guarded checkout).
	if wd.Dir() == h.parent {
		t.Fatal("worktree dir is the parent checkout")
	}
	wantRoot := filepath.Join(h.root, ".pyxis-worktrees")
	if !strings.HasPrefix(wd.Dir(), wantRoot+string(os.PathSeparator)) {
		t.Errorf("worktree dir %q not under the default root %q", wd.Dir(), wantRoot)
	}
	if fi, err := os.Stat(wd.Dir()); err != nil || !fi.IsDir() {
		t.Fatalf("worktree dir is not a directory: %v", err)
	}

	// (a) A write inside Workdir.Dir() stays there. Commit it exactly as a worker
	// would — the guard permits commits in a linked worktree.
	writeFile(t, filepath.Join(wd.Dir(), "worker-artifact.txt"), "made by the worker\n")
	gitOK(t, wd.Dir(), "add", "worker-artifact.txt")
	gitOK(t, wd.Dir(), "commit", "-q", "-m", "worker work")

	if _, err := os.Stat(filepath.Join(h.parent, "worker-artifact.txt")); !os.IsNotExist(err) {
		t.Fatal("the worker's artifact leaked into the parent checkout")
	}
	if head := gitOK(t, h.parent, "rev-parse", "HEAD"); head != h.baseA {
		t.Fatalf("parent HEAD moved to %s; a worktree write leaked into the parent", head)
	}

	// (b) A parent-checkout mutation attempt is mechanically BLOCKED. Staging is
	// allowed (it touches only the index); the commit that would record the
	// mutation is refused by the guard.
	writeFile(t, filepath.Join(h.parent, "sneaky.txt"), "mutate the parent\n")
	gitOK(t, h.parent, "add", "sneaky.txt")
	out, err := gitTry(t, h.parent, "commit", "-m", "mutate the parent")
	if err == nil {
		t.Fatal("a commit in the parent checkout succeeded; the guard did not block it")
	}
	if !strings.Contains(out, "refusing to mutate the parent checkout") {
		t.Fatalf("parent commit failed for the wrong reason; want the guard message, got:\n%s", out)
	}
	if head := gitOK(t, h.parent, "rev-parse", "HEAD"); head != h.baseA {
		t.Fatalf("parent HEAD moved to %s despite the guard", head)
	}
}

// --- acceptance criterion 2: based off freshly-fetched mainline (DINV-2) -------

func TestPrepare_BasesOffFreshlyFetchedMainline(t *testing.T) {
	ctx := context.Background()
	h := newHarness(t)

	// A blocker: a branch in the parent off A with commit B, never merged to the
	// mainline. It is exactly the base a stacked train (ALT-1, incident I2) would
	// wrongly pick.
	blkDir := filepath.Join(h.root, "wt-blocker")
	gitOK(t, h.parent, "worktree", "add", "-q", "-b", "blocker", blkDir, "HEAD")
	writeFile(t, filepath.Join(blkDir, "b"), "B\n")
	gitOK(t, blkDir, "add", "b")
	gitOK(t, blkDir, "commit", "-q", "-m", "B")
	commitB := gitOK(t, blkDir, "rev-parse", "HEAD")

	// The mainline advances to C, leaving the parent STALE at A: origin/main in
	// the parent still points at A until Prepare fetches.
	writeFile(t, filepath.Join(h.origin, "f"), "A\nC\n")
	gitOK(t, h.origin, "add", "f")
	gitOK(t, h.origin, "commit", "-q", "-m", "C")
	commitC := gitOK(t, h.origin, "rev-parse", "HEAD")
	if stale := gitOK(t, h.parent, "rev-parse", "origin/main"); stale != h.baseA {
		t.Fatalf("expected the parent to be stale at A=%s, got %s", h.baseA, stale)
	}

	ws := h.newWorkspace(t, ctx)
	wd, err := ws.Prepare(ctx, port.Task{ID: "T2", Number: 2, HeadBranch: "feat/dep-2", BlockedBy: []int{1}})
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}

	base := gitOK(t, wd.Dir(), "rev-parse", "HEAD")
	switch base {
	case commitC:
		// correct: the freshly-fetched mainline tip
	case h.baseA:
		t.Fatalf("worktree based off the stale parent tip A=%s, want fetched C=%s", h.baseA, commitC)
	case commitB:
		t.Fatalf("worktree based off the unmerged blocker B=%s, want fetched C=%s", commitB, commitC)
	default:
		t.Fatalf("worktree base = %s, want the freshly-fetched mainline tip C=%s", base, commitC)
	}
	// And it is not stacked on the blocker: B is not in the worktree's history.
	if _, err := gitTry(t, wd.Dir(), "merge-base", "--is-ancestor", commitB, "HEAD"); err == nil {
		t.Fatal("the worktree is stacked on the blocker branch (B is an ancestor of its base)")
	}
}

// TestPrepare_HonorsConfiguredNonDefaultMainline proves the workspace resolves
// the CONFIGURED remote/branch (config-driven-portability-v1 §3.2, FR-2): given
// an origin whose mainline is "trunk" (not "main"), a Config{Mainline: "trunk"}
// fetches and bases the worktree off trunk's tip. A repo whose default branch is
// not "main" is honored — the adapter carries no "main" literal of its own.
func TestPrepare_HonorsConfiguredNonDefaultMainline(t *testing.T) {
	ctx := context.Background()
	t.Setenv("GIT_TERMINAL_PROMPT", "0")
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_AUTHOR_NAME", "pyxis-test")
	t.Setenv("GIT_AUTHOR_EMAIL", "test@pyxis.local")
	t.Setenv("GIT_COMMITTER_NAME", "pyxis-test")
	t.Setenv("GIT_COMMITTER_EMAIL", "test@pyxis.local")
	t.Setenv("GIT_AUTHOR_DATE", "2026-07-23T00:00:00Z")
	t.Setenv("GIT_COMMITTER_DATE", "2026-07-23T00:00:00Z")

	root := t.TempDir()
	origin := filepath.Join(root, "origin")
	parent := filepath.Join(root, "parent")
	if err := os.MkdirAll(origin, 0o755); err != nil {
		t.Fatal(err)
	}
	// The origin's mainline is "trunk" — deliberately NOT "main".
	gitOK(t, origin, "init", "-q", "-b", "trunk")
	writeFile(t, filepath.Join(origin, "f"), "T\n")
	gitOK(t, origin, "add", "f")
	gitOK(t, origin, "commit", "-q", "-m", "T")
	trunkTip := gitOK(t, origin, "rev-parse", "HEAD")
	gitOK(t, root, "clone", "-q", origin, parent)

	ws, err := New(ctx, Config{Parent: parent, Remote: "origin", Mainline: "trunk"})
	if err != nil {
		t.Fatalf("New with a non-default mainline: %v", err)
	}
	wd, err := ws.Prepare(ctx, port.Task{ID: "T1", Number: 1, HeadBranch: "feat/on-trunk"})
	if err != nil {
		t.Fatalf("Prepare off trunk: %v", err)
	}
	if base := gitOK(t, wd.Dir(), "rev-parse", "HEAD"); base != trunkTip {
		t.Fatalf("worktree based off %s, want the configured trunk tip %s", base, trunkTip)
	}
}

// --- acceptance criterion 3: Cleanup reaps the worktree + branch ---------------

func TestCleanup_ReapsWorktreeAndBranch(t *testing.T) {
	ctx := context.Background()
	h := newHarness(t)
	ws := h.newWorkspace(t, ctx)

	wd, err := ws.Prepare(ctx, port.Task{ID: "T3", Number: 3, HeadBranch: "feat/reap-3"})
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if !worktreeRegistered(t, h, wd.Dir()) {
		t.Fatal("worktree not registered after Prepare")
	}
	if gitOK(t, h.parent, "branch", "--list", "feat/reap-3") == "" {
		t.Fatal("branch missing after Prepare")
	}

	// Leave an uncommitted change so the reaper must force-remove a dirty worktree.
	writeFile(t, filepath.Join(wd.Dir(), "dirty.txt"), "uncommitted\n")
	stateRoot := wd.Dir() + "-runtime"
	writeFile(t, filepath.Join(stateRoot, "claude", "conversation"), "task conversation\n")

	if err := wd.Cleanup(ctx); err != nil {
		t.Fatalf("Cleanup: %v", err)
	}
	if _, err := os.Stat(wd.Dir()); !os.IsNotExist(err) {
		t.Fatalf("worktree dir survived cleanup: %v", err)
	}
	if _, err := os.Stat(stateRoot); !os.IsNotExist(err) {
		t.Fatalf("task runtime state survived cleanup: %v", err)
	}
	if left := gitOK(t, h.parent, "branch", "--list", "feat/reap-3"); left != "" {
		t.Fatalf("branch survived cleanup: %q", left)
	}
	if worktreeRegistered(t, h, wd.Dir()) {
		t.Fatal("worktree still registered after cleanup")
	}
	// The reaper is idempotent — a second call is a safe no-op.
	if err := wd.Cleanup(ctx); err != nil {
		t.Fatalf("second Cleanup was not idempotent: %v", err)
	}
}

// --- #397: Push publishes the branch (a REAL git push to a real origin) --------

// After a real commit in the worktree, Push publishes the branch to the origin so
// a proposal can be opened from a genuinely-published head. This exercises the
// exact seam the hermetic e2e faked (nothing was ever pushed) and the A-12
// dogfood caught (OpenProposal 404 on an unpublished head).
func TestPush_PublishesCommittedBranchToOrigin(t *testing.T) {
	ctx := context.Background()
	h := newHarness(t)
	ws := h.newWorkspace(t, ctx)

	wd, err := ws.Prepare(ctx, port.Task{ID: "T1", Number: 1, HeadBranch: "feat/push-1"})
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	// A supervisor-created commit in the isolated worktree is publishable.
	writeFile(t, filepath.Join(wd.Dir(), "impl.txt"), "the implementation\n")
	gitOK(t, wd.Dir(), "add", "impl.txt")
	gitOK(t, wd.Dir(), "commit", "-q", "-m", "implement the task")
	want := gitOK(t, wd.Dir(), "rev-parse", "HEAD")

	published, err := wd.Push(ctx)
	if err != nil {
		t.Fatalf("Push: %v", err)
	}
	if !published {
		t.Fatal("Push reported nothing published despite a real commit")
	}
	// The origin now carries the branch at exactly the worktree's HEAD.
	if got := gitOK(t, h.origin, "rev-parse", "feat/push-1"); got != want {
		t.Fatalf("origin feat/push-1 = %s, want the published head %s", got, want)
	}
}

// Push on a worktree with no commits beyond its base publishes nothing and
// reports published=false — the Worker maps that to hollow (DINV-3) — and leaves
// no junk branch on the origin.
func TestPush_NothingToPublishWhenNoCommits(t *testing.T) {
	ctx := context.Background()
	h := newHarness(t)
	ws := h.newWorkspace(t, ctx)

	wd, err := ws.Prepare(ctx, port.Task{ID: "T2", Number: 2, HeadBranch: "feat/push-2"})
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	// No commit: the agent produced nothing.
	published, err := wd.Push(ctx)
	if err != nil {
		t.Fatalf("Push: %v", err)
	}
	if published {
		t.Fatal("Push reported published for a worktree with no commits")
	}
	// The origin has no such branch — nothing was published, so nothing to reap.
	if _, err := gitTry(t, h.origin, "rev-parse", "--verify", "feat/push-2"); err == nil {
		t.Fatal("origin has feat/push-2 despite nothing to publish")
	}
}

func TestPush_RequiresSupervisorCheckpoint(t *testing.T) {
	ctx := context.Background()
	h := newHarness(t)
	ws := h.newWorkspace(t, ctx)

	wd, err := ws.Prepare(ctx, port.Task{ID: "T3", Number: 3, HeadBranch: "feat/push-3"})
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	// The agent only edits. Publication must not silently manufacture a commit.
	writeFile(t, filepath.Join(wd.Dir(), "impl.txt"), "the implementation\n")
	writeFile(t, filepath.Join(wd.Dir(), "sub", "more.txt"), "more work\n")
	// Supervisor checkpoints do not execute agent-authored repository hooks.
	hooks := filepath.Join(wd.Dir(), "hooks")
	writeFile(t, filepath.Join(hooks, "pre-commit"), "#!/bin/sh\nexit 97\n")
	if err := os.Chmod(filepath.Join(hooks, "pre-commit"), 0o700); err != nil {
		t.Fatal(err)
	}
	gitOK(t, wd.Dir(), "config", "core.hooksPath", hooks)
	if published, err := wd.Push(ctx); err == nil || published {
		t.Fatalf("dirty Push = %v, %v; want refusal", published, err)
	}
	if _, err := wd.Checkpoint(ctx); err != nil {
		t.Fatalf("Checkpoint: %v", err)
	}

	published, err := wd.Push(ctx)
	if err != nil {
		t.Fatalf("Push: %v", err)
	}
	if !published {
		t.Fatal("Push reported nothing published despite uncommitted implementation work")
	}
	// The origin now carries the explicitly checkpointed worktree HEAD.
	head := gitOK(t, wd.Dir(), "rev-parse", "HEAD")
	if got := gitOK(t, h.origin, "rev-parse", "feat/push-3"); strings.TrimSpace(got) != strings.TrimSpace(head) {
		t.Fatalf("origin feat/push-3 = %s, want the checkpoint head %s", got, head)
	}
	// That commit actually contains the agent's files...
	files := gitOK(t, wd.Dir(), "show", "--name-only", "--format=", "HEAD")
	if !strings.Contains(files, "impl.txt") || !strings.Contains(files, "sub/more.txt") {
		t.Fatalf("checkpoint missing the agent's files; got:\n%s", files)
	}
	// ...and the worktree is clean again — everything was captured, not lost.
	if st := gitOK(t, wd.Dir(), "status", "--porcelain"); strings.TrimSpace(st) != "" {
		t.Fatalf("worktree still dirty after checkpoint: %q", st)
	}
}

// Checkpoint commits the agent's uncommitted WIP without publishing. Both
// completion and interruption use it; a resume continues from a committed
// base, and a clean tree is a no-op.
func TestCheckpoint_CommitsUncommittedWorkWithoutPublishing(t *testing.T) {
	ctx := context.Background()
	h := newHarness(t)
	ws := h.newWorkspace(t, ctx)

	wd, err := ws.Prepare(ctx, port.Task{ID: "T4", Number: 4, HeadBranch: "feat/checkpoint-1"})
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	base := gitOK(t, wd.Dir(), "rev-parse", "HEAD")

	// The agent hit its bound mid-implementation, leaving its work uncommitted.
	writeFile(t, filepath.Join(wd.Dir(), "impl.txt"), "the implementation\n")
	writeFile(t, filepath.Join(wd.Dir(), "sub", "more.txt"), "more work\n")

	hasWork, err := wd.Checkpoint(ctx)
	if err != nil {
		t.Fatalf("Checkpoint: %v", err)
	}
	if !hasWork {
		t.Fatal("Checkpoint reported no work despite committing the agent's implementation")
	}

	// Exactly one commit beyond base, containing the agent's files, tree clean.
	if ahead := strings.TrimSpace(gitOK(t, wd.Dir(), "rev-list", "--count", base+"..HEAD")); ahead != "1" {
		t.Fatalf("commits beyond base = %s, want 1 (the checkpoint commit)", ahead)
	}
	files := gitOK(t, wd.Dir(), "show", "--name-only", "--format=", "HEAD")
	if !strings.Contains(files, "impl.txt") || !strings.Contains(files, "sub/more.txt") {
		t.Fatalf("checkpoint commit missing the agent's files; got:\n%s", files)
	}
	if st := gitOK(t, wd.Dir(), "status", "--porcelain"); strings.TrimSpace(st) != "" {
		t.Fatalf("worktree still dirty after checkpoint: %q", st)
	}
	// Crucially, NOTHING was published — Checkpoint is not Push. The origin has no
	// such branch, so a suspended run leaves no half-done remote proposal.
	if _, err := gitTry(t, h.origin, "rev-parse", "--verify", "feat/checkpoint-1"); err == nil {
		t.Fatal("origin has feat/checkpoint-1 after Checkpoint — it must not publish")
	}

	// A second checkpoint on the now-clean tree is a no-op commit, but the branch
	// is still ahead of base, so it still reports work (shippable/resumable).
	hasWork, err = wd.Checkpoint(ctx)
	if err != nil {
		t.Fatalf("second Checkpoint: %v", err)
	}
	if !hasWork {
		t.Fatal("second Checkpoint reported no work despite the branch being ahead of base")
	}
	if ahead := strings.TrimSpace(gitOK(t, wd.Dir(), "rev-list", "--count", base+"..HEAD")); ahead != "1" {
		t.Fatalf("commits beyond base = %s after a no-op checkpoint, want still 1", ahead)
	}
}

// Checkpoint on a run that produced NOTHING reports hasWork=false — the signal
// the Worker uses to respawn an empty truncation rather than try to ship it.
func TestCheckpoint_EmptyRunReportsNoWork(t *testing.T) {
	ctx := context.Background()
	h := newHarness(t)
	ws := h.newWorkspace(t, ctx)

	wd, err := ws.Prepare(ctx, port.Task{ID: "T5", Number: 5, HeadBranch: "feat/checkpoint-empty"})
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}

	// No files written — the agent produced nothing before its bound.
	hasWork, err := wd.Checkpoint(ctx)
	if err != nil {
		t.Fatalf("Checkpoint: %v", err)
	}
	if hasWork {
		t.Fatal("Checkpoint reported work on an empty run — nothing was committed beyond base")
	}
}

// A re-run publishes idempotently: appending a commit and pushing again
// fast-forwards the origin to the new head (no force needed — the publish is
// append-only), so a resumed Worker re-publishes without error (AP-22).
func TestPush_RepublishesAppendedCommits(t *testing.T) {
	ctx := context.Background()
	h := newHarness(t)
	ws := h.newWorkspace(t, ctx)

	wd, err := ws.Prepare(ctx, port.Task{ID: "T1", Number: 1, HeadBranch: "feat/push-3"})
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	writeFile(t, filepath.Join(wd.Dir(), "impl.txt"), "first\n")
	gitOK(t, wd.Dir(), "add", "impl.txt")
	gitOK(t, wd.Dir(), "commit", "-q", "-m", "first")
	if _, err := wd.Push(ctx); err != nil {
		t.Fatalf("first Push: %v", err)
	}

	// A second commit, then re-publish.
	writeFile(t, filepath.Join(wd.Dir(), "impl.txt"), "first\nsecond\n")
	gitOK(t, wd.Dir(), "add", "impl.txt")
	gitOK(t, wd.Dir(), "commit", "-q", "-m", "second")
	want := gitOK(t, wd.Dir(), "rev-parse", "HEAD")

	published, err := wd.Push(ctx)
	if err != nil {
		t.Fatalf("re-Push: %v", err)
	}
	if !published {
		t.Fatal("re-Push reported nothing published")
	}
	if got := gitOK(t, h.origin, "rev-parse", "feat/push-3"); got != want {
		t.Fatalf("origin feat/push-3 = %s, want the re-published head %s", got, want)
	}
}

// --- resume: Prepare is idempotent for a respawned worker --------------------

// A worker that hit its bound is respawned and re-Prepares the SAME task. Prepare
// must REUSE the existing worktree (with its accumulated WIP) instead of
// hard-failing on "branch already exists" — the resume path a bounded retry
// rests on (AP-22, DINV-9).
func TestPrepare_IdempotentResume(t *testing.T) {
	ctx := context.Background()
	h := newHarness(t)
	ws := h.newWorkspace(t, ctx)
	task := port.Task{ID: "T1", Number: 1, HeadBranch: "feat/resume-1"}

	wd1, err := ws.Prepare(ctx, task)
	if err != nil {
		t.Fatalf("first Prepare: %v", err)
	}
	// The agent's WIP before it was killed: one committed file, one uncommitted.
	writeFile(t, filepath.Join(wd1.Dir(), "committed.txt"), "done\n")
	gitOK(t, wd1.Dir(), "add", "committed.txt")
	gitOK(t, wd1.Dir(), "commit", "-q", "-m", "partial progress")
	writeFile(t, filepath.Join(wd1.Dir(), "uncommitted.txt"), "wip\n")

	// Respawn: re-Prepare the same task. It must succeed and return the SAME
	// worktree, with both the committed and uncommitted WIP intact.
	wd2, err := ws.Prepare(ctx, task)
	if err != nil {
		t.Fatalf("resume Prepare must reuse the worktree, got: %v", err)
	}
	if wd2.Dir() != wd1.Dir() {
		t.Errorf("resume dir = %q, want the original %q", wd2.Dir(), wd1.Dir())
	}
	for _, f := range []string{"committed.txt", "uncommitted.txt"} {
		if _, err := os.Stat(filepath.Join(wd2.Dir(), f)); err != nil {
			t.Errorf("WIP file %s lost on resume: %v", f, err)
		}
	}
}

// One Workspace serves every Worker in a sweep, and the Dispatcher runs them
// concurrently up to parallel_width — so Prepare must be safe under real
// concurrency, not merely on paper.
//
// It was not. Prepare fetches the mainline and then pins the tip by reading
// FETCH_HEAD, and FETCH_HEAD is a single file every fetch rewrites, not an
// atomically-updated ref. A sibling Worker fetching between this Worker's fetch
// and its read made the read fail outright ("ambiguous argument 'FETCH_HEAD'"),
// failing a task that had done nothing wrong; the same window could otherwise
// hand back a tip this Worker never fetched. Arming the commit-guard raced the
// same way, on git's config.lock.
//
// This is the regression: many Workers preparing at once must all succeed and
// must all be based off the one mainline tip.
func TestPrepare_ConcurrentWorkersShareTheParentSafely(t *testing.T) {
	ctx := context.Background()
	h := newHarness(t)
	ws := h.newWorkspace(t, ctx)

	const workers = 8
	type prepared struct {
		dir string
		err error
	}
	results := make([]prepared, workers)

	var start sync.WaitGroup
	var done sync.WaitGroup
	start.Add(1)
	for i := range workers {
		done.Add(1)
		go func(i int) {
			defer done.Done()
			start.Wait() // release them together, to actually overlap
			wd, err := ws.Prepare(ctx, port.Task{
				ID:         fmt.Sprintf("T%d", i),
				Number:     i + 1,
				HeadBranch: fmt.Sprintf("feat/concurrent-%d", i),
			})
			if err != nil {
				results[i] = prepared{err: err}
				return
			}
			results[i] = prepared{dir: wd.Dir()}
		}(i)
	}
	start.Done()
	done.Wait()

	dirs := map[string]bool{}
	for i, r := range results {
		if r.err != nil {
			t.Errorf("worker %d: Prepare failed under concurrency: %v", i, r.err)
			continue
		}
		if dirs[r.dir] {
			t.Errorf("worker %d: got worktree %s, already handed to another task", i, r.dir)
		}
		dirs[r.dir] = true
		// Every task is cut from the SAME freshly-fetched mainline tip (DINV-2):
		// a torn FETCH_HEAD read would show up here as a wrong or missing base.
		if head := gitOK(t, r.dir, "rev-parse", "HEAD"); head != h.baseA {
			t.Errorf("worker %d: worktree based off %s, want the mainline tip %s", i, head, h.baseA)
		}
	}
}

// If the branch exists but its worktree was reaped, Prepare attaches a fresh
// worktree to the existing branch rather than failing.
func TestPrepare_AttachesToOrphanedBranch(t *testing.T) {
	ctx := context.Background()
	h := newHarness(t)
	ws := h.newWorkspace(t, ctx)
	task := port.Task{ID: "T2", Number: 2, HeadBranch: "feat/orphan-2"}

	wd, err := ws.Prepare(ctx, task)
	if err != nil {
		t.Fatalf("first Prepare: %v", err)
	}
	// Reap the worktree (its branch survives).
	if err := wd.Cleanup(ctx); err != nil {
		t.Fatalf("cleanup: %v", err)
	}
	// Recreate the branch so it exists without a worktree (Cleanup deletes it).
	gitOK(t, h.parent, "branch", "feat/orphan-2", h.baseA)

	wd2, err := ws.Prepare(ctx, task)
	if err != nil {
		t.Fatalf("Prepare over an orphaned branch must attach a worktree, got: %v", err)
	}
	if wd2.Branch() != "feat/orphan-2" {
		t.Errorf("branch = %q, want feat/orphan-2", wd2.Branch())
	}
}

// --- fail-closed construction and prepare -------------------------------------

func TestNew_RejectsInvalidParent(t *testing.T) {
	ctx := context.Background()
	// Isolate git from host config so a non-repo really has no repo.
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)

	if _, err := New(ctx, Config{Parent: ""}); err == nil {
		t.Error("New with an empty parent: want an error")
	}
	if _, err := New(ctx, Config{Parent: t.TempDir()}); err == nil {
		t.Error("New over a non-git directory: want an error")
	}
}

func TestPrepare_RejectsEmptyHeadBranch(t *testing.T) {
	ctx := context.Background()
	h := newHarness(t)
	ws := h.newWorkspace(t, ctx)
	if _, err := ws.Prepare(ctx, port.Task{ID: "T9", Number: 9, HeadBranch: ""}); err == nil {
		t.Fatal("Prepare with no head branch: want an error")
	}
}

// --- slug ---------------------------------------------------------------------

func TestSlug(t *testing.T) {
	// Sanitized, non-empty, and deterministic; distinct branches never collide.
	seen := map[string]string{}
	for _, br := range []string{"feat/widget-42", "feat-widget-42", "feat/widget/42", "", "///", "T-1"} {
		s := slug(br)
		if strings.ContainsAny(s, "/ ") {
			t.Errorf("slug(%q) = %q contains a path separator or space", br, s)
		}
		if s == "" {
			t.Errorf("slug(%q) is empty", br)
		}
		if s != slug(br) {
			t.Errorf("slug(%q) is not deterministic", br)
		}
		if prev, ok := seen[s]; ok {
			t.Errorf("slug collision: %q and %q both map to %q", prev, br, s)
		}
		seen[s] = br
	}
}

// --- git helpers --------------------------------------------------------------

func git(t *testing.T, dir string, args ...string) (string, error) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	out, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

func gitOK(t *testing.T, dir string, args ...string) string {
	t.Helper()
	out, err := git(t, dir, args...)
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return out
}

// gitTry runs git and returns its combined output and error without failing the
// test — for the paths where a non-zero exit is the assertion (a blocked commit,
// a non-ancestor check).
func gitTry(t *testing.T, dir string, args ...string) (string, error) {
	t.Helper()
	return git(t, dir, args...)
}

// worktreeRegistered reports whether dir is a live worktree of the parent,
// comparing symlink-resolved paths so a /tmp vs /private/tmp indirection never
// makes the check lie.
func worktreeRegistered(t *testing.T, h *harness, dir string) bool {
	t.Helper()
	out := gitOK(t, h.parent, "worktree", "list", "--porcelain")
	want := realpath(dir)
	for line := range strings.SplitSeq(out, "\n") {
		if p, ok := strings.CutPrefix(line, "worktree "); ok {
			if realpath(strings.TrimSpace(p)) == want {
				return true
			}
		}
	}
	return false
}

func realpath(p string) string {
	if r, err := filepath.EvalSymlinks(p); err == nil {
		return r
	}
	return p
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// ChangeSize measures the committed branch against the base it was cut from,
// as the code host diffs the proposal; uncommitted work does not count yet.
func TestChangeSize_MeasuresTheCommittedBranchAgainstItsBase(t *testing.T) {
	ctx := context.Background()
	h := newHarness(t)
	ws := h.newWorkspace(t, ctx)
	wd, err := ws.Prepare(ctx, port.Task{ID: "T9", Number: 9, HeadBranch: "feat/size-9"})
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	writeFile(t, filepath.Join(wd.Dir(), "impl.go"), "package impl\n\n// Answer is documented.\nvar Answer = 42\n")
	gitOK(t, wd.Dir(), "add", "impl.go")
	gitOK(t, wd.Dir(), "commit", "-q", "-m", "implement the task")
	writeFile(t, filepath.Join(wd.Dir(), "later.go"), "package impl\n\nvar Later = 1\n")

	size, err := wd.ChangeSize(ctx)
	if err != nil {
		t.Fatalf("ChangeSize: %v", err)
	}
	if size.String() != "2 (+2/-0)" {
		t.Fatalf("ChangeSize = %s, want the committed impl.go only", size)
	}
	if _, err := (&Workdir{branch: "feat/unbased"}).ChangeSize(ctx); err == nil || !strings.Contains(err.Error(), "no recorded base") {
		t.Fatalf("a workdir without a base: %v", err)
	}
	if _, err := (&Workdir{dir: t.TempDir(), branch: "feat/broken", base: "not-a-commit"}).ChangeSize(ctx); err == nil || !strings.Contains(err.Error(), "measure the change on feat/broken") {
		t.Fatalf("an unmeasurable branch: %v", err)
	}
}
