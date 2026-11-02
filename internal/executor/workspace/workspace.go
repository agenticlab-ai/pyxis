// Package workspace is the git-worktree adapter for the executor's Workspace
// port (docs/tech-design/implement-now.md §3.1, seam S-4). It gives each task an
// isolated linked git worktree on a fresh branch, always based off the
// freshly-fetched mainline tip — never the parent checkout's (possibly stale)
// HEAD and never an unmerged blocker's branch (DINV-2, AP-26; harbormaster #982).
// A mechanical commit-guard installed on the parent checkout makes the isolation
// enforced, not conventional: a worker commits only inside its own worktree,
// never the parent checkout (AP-20, DINV-1, prd:implement-now/FR-4).
//
// It is a real adapter package (AP-1): the executor core declares the port and
// imports this adapter, never the reverse. Git is driven through os/exec against
// a real repository — the local-substitutable seam the hermetic tests exercise
// with a temp origin plus a working clone, no network (§3.10).
package workspace

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"hash/fnv"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/changesize"
	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/executor/port"
	executorruntime "github.com/agenticlab-ai/pyxis/internal/executor/runtime"
)

// Compile-time proof the adapter satisfies the port contracts, so any signature
// drift the parallel sibling adapters would also see is a build failure here.
var (
	_ port.Workspace = (*Workspace)(nil)
	_ port.Workdir   = (*Workdir)(nil)
)

// defaultRemote and defaultMainline are the fallback remote/branch a blank
// Config.Remote/Config.Mainline takes. They are sourced from the repo config
// schema's single source (config.Defaults) rather than carrying an independent
// literal, so the value the workspace resolves comes from the config (config-
// driven-portability-v1 §3.2, DINV-1) — "origin"/"main" on a default config,
// byte-identical (DINV-2). The composition root (cmd/implement-now) resolves
// the configured branch/remote and passes them explicitly; this fallback only
// serves a caller that leaves them blank (the hermetic tests).
var (
	defaultRemote   = config.Defaults().Remote
	defaultMainline = config.Defaults().Branch
)

const (
	// guardDirName is the adapter-owned hooks directory under the parent's common
	// git dir; core.hooksPath is pointed at it.
	guardDirName = "pyxis-guard-hooks"
	// worktreesDirName is the default parent-sibling directory holding the
	// per-task worktrees — a sibling, so worktrees never nest inside the guarded
	// parent checkout.
	worktreesDirName = ".pyxis-worktrees"
)

// Config parameterizes the worktree adapter.
type Config struct {
	// Parent is the path to the parent checkout — the shared clone the executor
	// operates. Worktrees are added to it and it is guarded against mutation.
	// Required; must be a git working tree.
	Parent string

	// Remote is the mainline remote fetched before every Prepare. Defaults to
	// "origin".
	Remote string

	// Mainline is the mainline branch every task is based off. Defaults to "main".
	Mainline string

	// Root is the directory the per-task worktrees are created under. Defaults to
	// a "<Parent>/../.pyxis-worktrees" sibling of the parent checkout.
	Root string
}

// Workspace is the git-worktree implementation of port.Workspace. Its fields are
// immutable after New and it is safe to share across the Dispatcher's Workers: it
// holds no cross-task state, and the part of Prepare that touches the SHARED
// parent checkout is serialized (see parentMu) so concurrent Workers cannot
// corrupt each other's view of it.
type Workspace struct {
	parent   string // absolute path to the parent checkout
	remote   string // mainline remote (e.g. "origin")
	mainline string // mainline branch (e.g. "main")
	root     string // absolute directory holding the per-task worktrees

	// parentMu serializes Prepare's parent-checkout prologue — arming the guard
	// and fetching-then-pinning the mainline tip. Both write process-global git
	// state in the parent's COMMON git dir, so concurrent Workers (the Dispatcher
	// runs up to parallel_width of them against one Workspace) otherwise interfere:
	// `git config core.hooksPath` contends on config.lock, and FETCH_HEAD — a
	// single file rewritten by every fetch, not an atomically-updated ref — can be
	// read by one Worker while another's fetch is rewriting it, which surfaces as
	// "ambiguous argument 'FETCH_HEAD'" and fails a task that did nothing wrong.
	//
	// The lock's scope is deliberately narrow: only the shared-parent prologue is
	// serialized. `git worktree add` and everything downstream stay concurrent —
	// they act on this task's own branch and directory, which is the work that
	// benefits from parallelism.
	//
	// This makes one Workspace safe for its own Workers. Two separate processes
	// sharing one parent checkout would still contend on FETCH_HEAD; that is out
	// of scope here (a run owns its checkout) and the git-ref CAS, not this lock,
	// is what makes cross-process work safe.
	parentMu sync.Mutex
}

// New builds the adapter and arms the parent-checkout commit-guard. It fails
// closed: an empty or non-git-working-tree parent, or a guard that cannot be
// installed, is an error — never a silently unguarded Workspace (AP-20, DINV-1).
// Arming the guard writes core.hooksPath on the parent; that is engine setup, not
// a worker mutation, and is idempotent.
func New(ctx context.Context, cfg Config) (*Workspace, error) {
	if strings.TrimSpace(cfg.Parent) == "" {
		return nil, errors.New("workspace: empty parent checkout path")
	}
	parent, err := filepath.Abs(cfg.Parent)
	if err != nil {
		return nil, fmt.Errorf("workspace: resolving parent path %q: %w", cfg.Parent, err)
	}
	w := &Workspace{
		parent:   parent,
		remote:   orDefault(cfg.Remote, defaultRemote),
		mainline: orDefault(cfg.Mainline, defaultMainline),
	}
	if strings.TrimSpace(cfg.Root) == "" {
		w.root = filepath.Join(filepath.Dir(parent), worktreesDirName)
	} else {
		root, err := filepath.Abs(cfg.Root)
		if err != nil {
			return nil, fmt.Errorf("workspace: resolving worktree root %q: %w", cfg.Root, err)
		}
		w.root = root
	}
	// The parent must be a real working tree — a guard over a bare repo would
	// protect nothing, so reject it rather than run unguarded.
	inside, err := runGit(ctx, w.parent, "rev-parse", "--is-inside-work-tree")
	if err != nil || inside != "true" {
		return nil, fmt.Errorf("workspace: %q is not a git working tree: %w", parent, err)
	}
	if err := w.ensureGuard(ctx); err != nil {
		return nil, fmt.Errorf("workspace: arming the parent-checkout guard: %w", err)
	}
	return w, nil
}

// Prepare provisions an isolated worktree for the task on a fresh branch off the
// freshly-fetched mainline tip. The returned Workdir is the Worker's only
// writable surface.
//
// The base is always the just-fetched mainline tip — resolved from FETCH_HEAD
// immediately after the fetch and pinned to that commit — so a dependent task is
// never based off the parent's stale HEAD nor off an unmerged blocker's branch,
// and no stacked proposal is created (DINV-2, AP-26; harbormaster #982). The
// commit-guard is re-armed first, so no worktree run ever proceeds against an
// unguarded parent (fail-closed, DINV-1).
func (w *Workspace) Prepare(ctx context.Context, task port.Task) (port.Workdir, error) {
	branch := strings.TrimSpace(task.HeadBranch)
	if branch == "" {
		return nil, fmt.Errorf("workspace: task %s has no head branch", task.ID)
	}
	base, err := w.armAndPinBase(ctx, task)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(w.root, 0o755); err != nil {
		return nil, fmt.Errorf("workspace: creating the worktree root for task %s: %w", task.ID, err)
	}
	dir := filepath.Join(w.root, slug(branch))
	workdir := &Workdir{workspace: w, parent: w.parent, dir: dir, branch: branch, remote: w.remote, base: base,
		taskID: task.ID, sourceRef: task.SourceRef}

	// Idempotent resume (AP-22): a worker that hit its bound is respawned as a
	// FRESH process against the SAME task and re-enters Prepare. Rather than
	// hard-fail on "branch already exists", reuse the accumulated worktree so the
	// resuming agent continues within its next bounded window — its WIP,
	// committed or not, is preserved.
	existing := w.worktreeBranch(ctx, dir)
	switch {
	case existing == branch:
		return workdir, nil
	case existing != "":
		// The deterministic path holds a worktree on a different branch — a
		// foreign/stale dir. Fail closed rather than silently building on the
		// wrong branch; the janitor reaps it.
		return nil, fmt.Errorf("workspace: worktree %s is checked out on %q, not the task branch %q (task %s)", dir, existing, branch, task.ID)
	}

	// No worktree at the path. If the branch already exists (its worktree was
	// reaped) attach a new worktree to it; otherwise create both fresh off the
	// pinned tip.
	branchThere, err := w.branchExists(ctx, branch)
	if err != nil {
		return nil, fmt.Errorf("workspace: checking branch %s for task %s: %w", branch, task.ID, err)
	}
	addArgs := []string{"worktree", "add", "-b", branch, dir, base}
	if branchThere {
		addArgs = []string{"worktree", "add", dir, branch}
	}
	if _, err := runGit(ctx, w.parent, addArgs...); err != nil {
		return nil, fmt.Errorf("workspace: adding worktree for task %s (branch %s): %w", task.ID, branch, err)
	}
	return workdir, nil
}

// Existing reopens a task's isolated worktree for an operator-authorized
// recovery. Unlike Prepare it never creates a worktree or branch, and it
// refuses any checkout it cannot vouch for: the deterministic worktree must
// hold the task branch at exactly revision, with nothing uncommitted. Its
// base is the freshly-fetched mainline tip, pinned as Prepare pins it.
func (w *Workspace) Existing(ctx context.Context, task port.Task, revision string) (port.Workdir, error) {
	return w.existing(ctx, task, revision, false)
}

// ExistingDescendant reopens a clean task checkpoint containing the expected
// published head, including repairs checkpointed before an interrupted push.
func (w *Workspace) ExistingDescendant(ctx context.Context, task port.Task, revision string) (port.Workdir, error) {
	return w.existing(ctx, task, revision, true)
}

func (w *Workspace) existing(ctx context.Context, task port.Task, revision string, descendant bool) (port.Workdir, error) {
	branch := strings.TrimSpace(task.HeadBranch)
	if branch == "" {
		return nil, fmt.Errorf("workspace: task %s has no head branch", task.ID)
	}
	dir := filepath.Join(w.root, slug(branch))
	if existing := w.worktreeBranch(ctx, dir); existing != branch {
		return nil, fmt.Errorf("workspace: task %s has no existing worktree on branch %s at %s", task.ID, branch, dir)
	}
	status, err := runGit(ctx, dir, "status", "--porcelain")
	if err != nil {
		return nil, fmt.Errorf("workspace: inspect existing worktree for task %s: %w", task.ID, err)
	}
	if status != "" {
		return nil, fmt.Errorf("workspace: existing worktree for task %s has uncommitted changes", task.ID)
	}
	head, err := runGit(ctx, dir, "rev-parse", "--verify", "HEAD^{commit}")
	if err != nil {
		return nil, fmt.Errorf("workspace: read existing worktree head for task %s: %w", task.ID, err)
	}
	if head != revision {
		if !descendant {
			return nil, fmt.Errorf("workspace: existing worktree for task %s is at %s, not the expected checkpoint %s", task.ID, head, revision)
		}
		if _, err := runGit(ctx, dir, "merge-base", "--is-ancestor", "--", revision, head); err != nil {
			return nil, fmt.Errorf("workspace: existing checkpoint %s for task %s does not contain published head %s: %w", head, task.ID, revision, err)
		}
	}
	base, err := w.armAndPinBase(ctx, task)
	if err != nil {
		return nil, err
	}
	return &Workdir{workspace: w, parent: w.parent, dir: dir, branch: branch, remote: w.remote, base: base,
		taskID: task.ID, sourceRef: task.SourceRef}, nil
}

// armAndPinBase is Prepare's parent-checkout prologue, run under parentMu: it
// re-arms the commit-guard, fetches the mainline, and pins the freshly-fetched
// tip — the DINV-2 base, never the parent's HEAD and never a blocker's branch.
//
// The three steps are one critical section because they are one fact. FETCH_HEAD
// is a plain file the next fetch overwrites, so a fetch by a sibling Worker
// between this Worker's fetch and its read would either corrupt the read or,
// worse, silently hand back a tip this Worker never fetched. Holding the lock
// across both makes "the tip I just fetched" true when it is read.
func (w *Workspace) armAndPinBase(ctx context.Context, task port.Task) (string, error) {
	w.parentMu.Lock()
	defer w.parentMu.Unlock()

	if err := w.ensureGuard(ctx); err != nil {
		return "", fmt.Errorf("workspace: re-arming the guard for task %s: %w", task.ID, err)
	}
	if err := w.fetchMainline(ctx); err != nil {
		return "", fmt.Errorf("workspace: fetching %s/%s for task %s: %w", w.remote, w.mainline, task.ID, err)
	}
	base, err := runGit(ctx, w.parent, "rev-parse", "FETCH_HEAD")
	if err != nil {
		return "", fmt.Errorf("workspace: resolving the fetched %s/%s tip for task %s: %w", w.remote, w.mainline, task.ID, err)
	}
	return base, nil
}

// fetchMainline fetches the mainline, retrying a transient failure a bounded
// number of times with backoff (AP-16) — a momentary network blip must not fail
// a whole task. It honors ctx cancellation between attempts.
func (w *Workspace) fetchMainline(ctx context.Context) error {
	const attempts = 3
	var lastErr error
	for i := range attempts {
		if _, err := runGit(ctx, w.parent, "fetch", w.remote, w.mainline); err == nil {
			return nil
		} else {
			lastErr = err
		}
		if i == attempts-1 {
			break
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Duration(i+1) * 500 * time.Millisecond):
		}
	}
	return fmt.Errorf("after %d attempts: %w", attempts, lastErr)
}

// worktreeBranch returns the branch checked out in the worktree at dir, or ""
// when dir is not a live git worktree (missing, empty, or unusable) — so Prepare
// can distinguish a resume from a fresh creation without erroring on absence.
func (w *Workspace) worktreeBranch(ctx context.Context, dir string) string {
	if _, err := os.Stat(dir); errors.Is(err, os.ErrNotExist) {
		return ""
	}
	out, err := runGit(ctx, dir, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		// dir exists but is not a usable worktree (e.g. a leftover empty dir):
		// treat as no worktree so the caller (re)creates it.
		return ""
	}
	return out
}

// branchExists reports whether branch exists in the parent checkout.
func (w *Workspace) branchExists(ctx context.Context, branch string) (bool, error) {
	out, err := runGit(ctx, w.parent, "branch", "--list", branch)
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(out) != "", nil
}

// Workdir is one task's isolated git worktree.
type Workdir struct {
	workspace *Workspace // owns the serialized fetch-and-pin operation
	parent    string     // the parent checkout the worktree is administered from
	dir       string     // absolute path to the worktree
	branch    string     // the worktree's branch
	remote    string     // the mainline remote the branch is published to (e.g. "origin")
	base      string     // the freshly-fetched mainline tip the branch was cut from
	taskID    string     // trusted task identity used in supervisor-created commits
	sourceRef string     // tracker-native provenance, never inferred from agent output
}

// Dir is the absolute worktree path — the workdir handed to the agent runtime
// and read by the code host when opening a proposal.
func (d *Workdir) Dir() string { return d.dir }

// Branch is the worktree's branch — the head a proposal is opened from.
func (d *Workdir) Branch() string { return d.branch }

// Push publishes the worktree's branch to its remote so a proposal can be opened
// from it after the supervisor checkpoints it (prd:implement-now/FR-10, §3.1).
// It refuses a dirty tree: publishing must never create an unverified commit.
// It counts commits beyond the base the branch was cut from: a HEAD no further
// along than that base
// means the agent produced nothing, so Push makes no remote change and returns
// published=false — the Worker maps that to hollow (DINV-3), and no junk remote
// branch is created. Otherwise it fast-forward pushes the head and returns
// published=true. It never merges (DINV-6).
func (d *Workdir) Push(ctx context.Context) (bool, error) {
	if strings.TrimSpace(d.base) == "" {
		return false, fmt.Errorf("workspace: worktree for branch %s has no recorded base to compare against", d.branch)
	}
	status, err := runGit(ctx, d.dir, "status", "--porcelain")
	if err != nil {
		return false, fmt.Errorf("workspace: inspect branch %s before publication: %w", d.branch, err)
	}
	if status != "" {
		return false, fmt.Errorf("workspace: branch %s has uncheckpointed changes; refusing publication", d.branch)
	}
	ahead, err := runGit(ctx, d.dir, "rev-list", "--count", d.base+"..HEAD")
	if err != nil {
		return false, fmt.Errorf("workspace: counting commits on %s beyond base: %w", d.branch, err)
	}
	if ahead == "0" {
		// Truly nothing beyond the base — the agent produced no change at all.
		// Publish nothing; the run is hollow (DINV-3), no remote branch to reap.
		return false, nil
	}
	if err := d.ValidateIntegration(ctx); err != nil {
		return false, err
	}
	// Publication preserves the reviewed head. Appending task and integration
	// commits fast-forwards on a re-run without rewriting published history.
	if _, err := runGit(ctx, d.dir, "push", d.remote, d.branch); err != nil {
		return false, fmt.Errorf("workspace: publishing %s to %s: %w", d.branch, d.remote, err)
	}
	return true, nil
}

// Checkpoint is the supervisor's sole commit operation. Call it only after the
// agent is quiescent, before verification or publication, and on interrupted
// attempts to preserve WIP. A clean tree is a no-op. hasWork reports whether
// the branch has commits beyond its recorded base; it is not a success verdict.
func (d *Workdir) Checkpoint(ctx context.Context) (bool, error) {
	if strings.TrimSpace(d.base) == "" {
		return false, fmt.Errorf("workspace: worktree for branch %s has no recorded base to compare against", d.branch)
	}
	branch, err := runGit(ctx, d.dir, "symbolic-ref", "--short", "HEAD")
	if err != nil {
		return false, fmt.Errorf("workspace: resolve checkpoint branch: %w", err)
	}
	if branch != d.branch {
		return false, fmt.Errorf("workspace: refusing checkpoint on branch %q; expected %q", branch, d.branch)
	}
	if err := d.commitPending(ctx); err != nil {
		return false, fmt.Errorf("workspace: checkpointing pending changes on %s: %w", d.branch, err)
	}
	ahead, err := runGit(ctx, d.dir, "rev-list", "--count", d.base+"..HEAD")
	if err != nil {
		return false, fmt.Errorf("workspace: counting commits on %s beyond base: %w", d.branch, err)
	}
	return strings.TrimSpace(ahead) != "0", nil
}

// ChangeSize measures the committed branch against its recorded base, the
// mainline tip it was cut from or last integrated (RefreshMainline). That
// base is the merge base the code host diffs the proposal against, so the
// figure matches the one the pr-description check computes.
func (d *Workdir) ChangeSize(ctx context.Context) (changesize.Size, error) {
	if strings.TrimSpace(d.base) == "" {
		return changesize.Size{}, fmt.Errorf("workspace: worktree for branch %s has no recorded base to compare against", d.branch)
	}
	size, err := changesize.Measure(ctx, d.dir, d.base, "HEAD")
	if err != nil {
		return changesize.Size{}, fmt.Errorf("workspace: measure the change on %s: %w", d.branch, err)
	}
	return size, nil
}

// commitPending stages explicit, NUL-delimited paths in the isolated task
// worktree, honoring .gitignore and treating pathspec magic literally. Existing
// staged changes from a recovered attempt are retained. No model-authored Git
// command or commit message participates in this operation.
func (d *Workdir) commitPending(ctx context.Context) error {
	status, err := runGit(ctx, d.dir, "status", "--porcelain")
	if err != nil {
		return err
	}
	if strings.TrimSpace(status) == "" {
		return nil // the supervisor already checkpointed, or nothing changed
	}
	paths, err := runGitInput(ctx, d.dir, "", "ls-files", "--modified", "--deleted", "--others", "--exclude-standard", "-z")
	if err != nil {
		return err
	}
	if paths != "" {
		if _, err := runGitInput(ctx, d.dir, paths, "--literal-pathspecs", "add", "--pathspec-from-file=-", "--pathspec-file-nul"); err != nil {
			return err
		}
	}
	msg := fmt.Sprintf("chore: checkpoint implementation for %s\n\nTask: %s\nSource: %s\nBranch: %s", d.taskID, d.taskID, d.sourceRef, d.branch)
	if _, err := runGit(ctx, d.dir, "commit", "-m", msg); err != nil {
		return err
	}
	return nil
}

// Cleanup reaps the worktree and deletes its branch — the janitor's decisive
// teardown, run on (merged ∧ closed) or age. It force-removes the worktree (even
// with uncommitted changes), prunes its administrative metadata, and
// force-deletes the branch. It is idempotent: a worktree or branch that is
// already gone is not an error, so a re-run of the reaper is safe.
func (d *Workdir) Cleanup(ctx context.Context) error {
	if _, err := runGit(ctx, d.parent, "worktree", "remove", "--force", d.dir); err != nil {
		// Tolerate an already-reaped worktree (idempotent re-run); surface anything
		// else — a live worktree that would not remove is a real failure.
		if _, statErr := os.Stat(d.dir); !errors.Is(statErr, os.ErrNotExist) {
			return fmt.Errorf("workspace: removing worktree %s: %w", d.dir, err)
		}
	}
	if err := os.RemoveAll(executorruntime.TaskStateRoot(d.dir)); err != nil {
		return fmt.Errorf("workspace: remove task runtime state: %w", err)
	}
	// Prune stale administrative files for a worktree dir removed out-of-band.
	if _, err := runGit(ctx, d.parent, "worktree", "prune"); err != nil {
		return fmt.Errorf("workspace: pruning worktree metadata for %s: %w", d.dir, err)
	}
	if _, err := runGit(ctx, d.parent, "branch", "-D", d.branch); err != nil {
		// A branch that no longer exists is already reaped; only a delete that
		// fails while the branch is still present is a failure.
		exists, existsErr := d.branchExists(ctx)
		if existsErr != nil {
			return existsErr
		}
		if exists {
			return fmt.Errorf("workspace: deleting branch %s: %w", d.branch, err)
		}
	}
	return nil
}

// branchExists reports whether the worktree's branch still exists in the parent.
func (d *Workdir) branchExists(ctx context.Context) (bool, error) {
	out, err := runGit(ctx, d.parent, "branch", "--list", d.branch)
	if err != nil {
		return false, fmt.Errorf("workspace: checking branch %s: %w", d.branch, err)
	}
	return strings.TrimSpace(out) != "", nil
}

// runGit runs a git subcommand in dir and returns its trimmed stdout. It honors
// ctx cancellation (the process is killed when ctx is done) and disables
// interactive credential prompts so an unattended run fails loudly rather than
// hanging (prd:implement-now/FR-16 posture). On failure it wraps the error with
// the command and git's stderr for a diagnosable message.
func runGit(ctx context.Context, dir string, args ...string) (string, error) {
	out, err := runGitInput(ctx, dir, "", args...)
	return strings.TrimSpace(out), err
}

// runGitInput preserves literal filename bytes in NUL-delimited Git output.
func runGitInput(ctx context.Context, dir, input string, args ...string) (string, error) {
	// Agent-authored hooks must never execute outside the verification boundary.
	cmd := exec.CommandContext(ctx, "git", append([]string{"-c", "core.hooksPath=/dev/null", "-c", "core.fsmonitor=false", "-C", dir}, args...)...)
	cmd.Stdin = strings.NewReader(input)
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0", "GIT_OPTIONAL_LOCKS=0")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			return "", fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
		}
		return "", fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, msg)
	}
	return stdout.String(), nil
}

// slug maps a branch name to a filesystem-safe, collision-resistant directory
// name for its worktree (e.g. "feat/widget-42" → "feat-widget-42-<hash>"). The
// hash suffix keeps distinct branches on distinct paths even when sanitizing
// would otherwise collide, and is deterministic so a task's worktree path is
// stable across calls.
func slug(branch string) string {
	var b strings.Builder
	for _, r := range branch {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_', r == '.':
			b.WriteRune(r)
		default:
			b.WriteByte('-')
		}
	}
	clean := strings.Trim(b.String(), "-.")
	if clean == "" {
		clean = "task"
	}
	h := fnv.New32a()
	_, _ = h.Write([]byte(branch)) // fnv.Write never errors
	return fmt.Sprintf("%s-%08x", clean, h.Sum32())
}

// orDefault returns v, or def when v is blank.
func orDefault(v, def string) string {
	if strings.TrimSpace(v) == "" {
		return def
	}
	return v
}
