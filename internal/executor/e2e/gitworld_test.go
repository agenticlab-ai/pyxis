package e2e

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/executor/lease"
	"github.com/agenticlab-ai/pyxis/internal/executor/port"
	"github.com/agenticlab-ai/pyxis/internal/executor/workspace"
)

// gitWorld is the harness's REAL git substrate: a bare "origin" — the only thing
// the code host can see — plus the parent checkout the executor operates. Two of
// the five ports run against it as their PRODUCTION adapters, unmodified:
//
//   - workspace.Workspace, so every task gets a genuine linked worktree cut from
//     the freshly-fetched mainline tip, under the real parent-checkout commit
//     guard, publishing by a real `git push` (DINV-1, DINV-2);
//   - lease.Lease, so a claim is a real create-or-fail ref CAS against the shared
//     origin, released by a real guarded ref delete (DINV-5).
//
// Nothing here is a stand-in for git. The mainline exists, the branches exist,
// the commits exist, and "what CI would see" is literally what was pushed — so a
// publish that did not happen cannot read green, which is exactly the class of
// defect an in-memory workspace fake cannot catch.
//
// No network is involved: origin is a local bare repository on disk. Every git
// invocation inherits an isolated environment (no system/global config, fixed
// identity and dates), so the host's ~/.gitconfig cannot leak in and the
// adapters under test — which shell out with the same inherited environment —
// stay hermetic. The only external requirement is a `git` binary on PATH, which
// the sibling adapter suites (internal/executor/workspace, internal/executor/lease)
// already depend on.
type gitWorld struct {
	root     string // t.TempDir(): holds origin, the parent checkout and the worktree root
	origin   string // the bare repository standing in for the shared code host's git side
	parent   string // the parent checkout the executor fetches, worktrees and guards
	remote   string // the remote name the parent reaches origin by
	mainline string // the mainline branch every task is cut from
	baseline string // origin's initial mainline tip
}

// newGitWorld builds the substrate: an isolated git environment, a parent
// checkout with one mainline commit, and a bare origin holding that mainline.
func newGitWorld(t *testing.T) *gitWorld {
	t.Helper()
	isolateGit(t)

	root := t.TempDir()
	w := &gitWorld{
		root:     root,
		origin:   filepath.Join(root, "origin.git"),
		parent:   filepath.Join(root, "parent"),
		remote:   "origin",
		mainline: "main",
	}

	// The parent checkout, with one real mainline commit. `-b main` pins the
	// initial branch rather than inheriting the host's init.defaultBranch.
	if err := os.MkdirAll(w.parent, 0o755); err != nil {
		t.Fatalf("create the parent checkout dir: %v", err)
	}
	mustGit(t, w.parent, "init", "-q", "-b", w.mainline)
	mustWrite(t, filepath.Join(w.parent, "README.md"), "pyxis executor e2e fixture repository\n")
	mustGit(t, w.parent, "add", "README.md")
	mustGit(t, w.parent, "commit", "-q", "-m", "seed the mainline")

	// The bare origin, and the mainline published onto it. Bare because it is a
	// push target: the claim CAS and every task branch land here.
	mustGit(t, root, "init", "-q", "--bare", w.origin)
	mustGit(t, w.parent, "remote", "add", w.remote, w.origin)
	mustGit(t, w.parent, "push", "-q", w.remote, w.mainline)
	w.baseline = mustGit(t, w.parent, "rev-parse", "HEAD")

	return w
}

// isolateGit pins every git invocation in this test — the harness's own setup
// commands AND the adapters' shell-outs, which inherit this environment — to a
// hermetic world: no system or global config (so the developer's ~/.gitconfig
// cannot change a result), no credential prompt, a fixed identity and clock (so
// commits are deterministic), and file:// as the ONLY permitted protocol, which
// makes an accidental network reach a hard failure rather than a slow test.
func isolateGit(t *testing.T) {
	t.Helper()
	for k, v := range map[string]string{
		"GIT_CONFIG_NOSYSTEM": "1",
		"GIT_CONFIG_GLOBAL":   os.DevNull,
		"GIT_TERMINAL_PROMPT": "0",
		"GIT_ALLOW_PROTOCOL":  "file",
		"GIT_AUTHOR_NAME":     "pyxis-e2e",
		"GIT_AUTHOR_EMAIL":    "e2e@pyxis.invalid",
		"GIT_COMMITTER_NAME":  "pyxis-e2e",
		"GIT_COMMITTER_EMAIL": "e2e@pyxis.invalid",
		"GIT_AUTHOR_DATE":     "2026-08-31T00:00:00Z",
		"GIT_COMMITTER_DATE":  "2026-08-31T00:00:00Z",
	} {
		t.Setenv(k, v)
	}
}

// newWorkspace builds the REAL git-worktree adapter over this world, rooting the
// per-task worktrees inside the test's temp tree rather than beside the parent.
func (w *gitWorld) newWorkspace(t *testing.T) *workspace.Workspace {
	t.Helper()
	space, err := workspace.New(context.Background(), workspace.Config{
		Parent:   w.parent,
		Remote:   w.remote,
		Mainline: w.mainline,
		Root:     filepath.Join(w.root, "worktrees"),
	})
	if err != nil {
		t.Fatalf("build the real workspace adapter: %v", err)
	}
	return space
}

// newLease builds the REAL git-ref CAS adapter over this world, claiming against
// the same origin the branches are published to — the production arrangement
// (cmd/implement-now wires both at the run's repo dir).
func (w *gitWorld) newLease(t *testing.T) *countingLease {
	t.Helper()
	adapter, err := lease.New(lease.Config{
		WorkDir: w.parent,
		Remote:  w.remote,
		Holder:  "pyxis-e2e/" + t.Name(),
	})
	if err != nil {
		t.Fatalf("build the real lease adapter: %v", err)
	}
	return &countingLease{inner: adapter}
}

// branchHead is the commit origin has for branch, and whether origin carries the
// branch at all. It reads the bare repository directly: what is here is exactly
// what was published, with no adapter in between.
func (w *gitWorld) branchHead(t *testing.T, branch string) (string, bool) {
	t.Helper()
	head, err := git(context.Background(), w.origin, "rev-parse", "--verify", "--quiet", "refs/heads/"+branch)
	if err != nil {
		return "", false
	}
	return head, head != ""
}

// publishedArtifact is the impl artifact's content as published on origin — the
// harness's witness for "what CI would see". Absent branch or absent file both
// report not-present.
func (w *gitWorld) publishedArtifact(t *testing.T, branch string) (string, bool) {
	t.Helper()
	if _, ok := w.branchHead(t, branch); !ok {
		return "", false
	}
	content, err := git(context.Background(), w.origin, "show", branch+":"+implFile)
	if err != nil {
		return "", false
	}
	return strings.TrimSpace(content), true
}

// isAncestor reports whether one commit is reachable from another on origin —
// the real-ancestry predicate the branch-base assertions rest on.
func (w *gitWorld) isAncestor(t *testing.T, ancestor, descendant string) bool {
	t.Helper()
	_, err := git(context.Background(), w.origin, "merge-base", "--is-ancestor", ancestor, descendant)
	return err == nil
}

// worktreeCount is how many worktrees the parent checkout administers, itself
// included. It stands in for "how many task workspaces were prepared" now that
// the Workspace is the real adapter and keeps no counter of its own.
func (w *gitWorld) worktreeCount(t *testing.T) int {
	t.Helper()
	out := mustGit(t, w.parent, "worktree", "list", "--porcelain")
	return strings.Count(out, "worktree ")
}

// claimRefs lists the claim-ref namespace on origin. An empty list after a run
// is the real fire-or-release witness (DINV-5): every claim this process took
// was genuinely deleted from the shared remote, not merely forgotten in memory.
func (w *gitWorld) claimRefs(t *testing.T) []string {
	t.Helper()
	out := mustGit(t, w.origin, "for-each-ref", "--format=%(refname)", "refs/claim/")
	if strings.TrimSpace(out) == "" {
		return nil
	}
	return strings.Split(out, "\n")
}

// --- the real Lease, instrumented -----------------------------------------------

// countingLease wraps the REAL lease adapter to count acquires and releases. It
// is instrumentation, never a stand-in: every call is delegated to the git-ref
// CAS, and the counters only observe it. The claim-ref assertions on origin are
// the independent check that the counts describe real remote state.
type countingLease struct {
	inner    port.Lease
	acquires int32 // atomic
	releases int32 // atomic
}

func (l *countingLease) Acquire(ctx context.Context, task port.Task) (port.Claim, error) {
	claim, err := l.inner.Acquire(ctx, task)
	if err != nil {
		return nil, err
	}
	atomic.AddInt32(&l.acquires, 1)
	return &countingClaim{inner: claim, lease: l}, nil
}

func (l *countingLease) acquireCount() int { return int(atomic.LoadInt32(&l.acquires)) }
func (l *countingLease) releaseCount() int { return int(atomic.LoadInt32(&l.releases)) }

type countingClaim struct {
	inner port.Claim
	lease *countingLease
	once  sync.Once
	err   error
}

func (c *countingClaim) Release(ctx context.Context) error {
	c.once.Do(func() {
		c.err = c.inner.Release(ctx)
		atomic.AddInt32(&c.lease.releases, 1)
	})
	return c.err
}

// --- git helpers ----------------------------------------------------------------

// git runs a git subcommand in dir and returns its trimmed stdout, honouring
// context cancellation. It is the harness's own shell-out — the adapters have
// their own.
func git(ctx context.Context, dir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", dir}, args...)...)
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", &gitError{args: args, dir: dir, stderr: strings.TrimSpace(stderr.String()), err: err}
	}
	return strings.TrimSpace(stdout.String()), nil
}

type gitError struct {
	args   []string
	dir    string
	stderr string
	err    error
}

func (e *gitError) Error() string {
	return "git " + strings.Join(e.args, " ") + " (in " + e.dir + "): " + e.err.Error() + ": " + e.stderr
}

func (e *gitError) Unwrap() error { return e.err }

// mustGit runs git in dir and fails the test on error — for harness setup, where
// a git failure is never an interesting result.
func mustGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	out, err := git(context.Background(), dir, args...)
	if err != nil {
		t.Fatalf("%v", err)
	}
	return out
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
