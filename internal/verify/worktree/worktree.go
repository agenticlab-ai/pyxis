// Package worktree implements the ephemeral git worktree rung 3's
// falsification check runs its two-leg protocol in (design codegate-v1 §3.9):
// a throwaway checkout at the workspace's head revision, provisioned with
// whatever state a bare checkout lacks, torn down at run end. Nothing here
// ever touches the workspace under verification — DINV-7 holds because every
// mutating operation (the reverse-apply, the test run) targets the worktree's
// own path, never the workspace's.
package worktree

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// ScratchRelPath is the workspace-relative directory ephemeral worktrees live
// under: inside codegate's own scratch (".codegate", the invocation contract's
// fixed constant — design §3.6), so a worktree is exempt from the dirty-tree
// refusal the same way the rest of scratch is, and is never part of what
// DINV-7's tree-hash equality inspects. Not imported from package invoke to
// keep worktree free of a dependency on it — invoke and worktree sit at the
// same layer, and ".codegate" is a stable, fixed constant of the invocation
// contract either package may spell independently.
const ScratchRelPath = ".codegate/worktrees"

// EphemeralWorktree is a throwaway git worktree checked out at a fixed
// revision — never the agent's own workspace tree.
type EphemeralWorktree struct {
	// Path is the worktree's absolute filesystem path.
	Path string
	// workspace is the origin repository Path was added from; Remove and
	// Provision both need it (Provision to locate the state dirs it links from).
	workspace string
}

// Create adds a git worktree at headSHA under a run-id-unique path inside the
// workspace's own scratch directory. runID must be unique per run (the
// ladder's own run id is the intended source) so a run's worktree can never
// collide with a still-registered leftover from a prior, killed run — a live
// invocation never has two runs in flight at once (the workspace flock,
// design §3.6, guarantees that), so the only collision this guards against is
// with a dead run's stale registration, which a later invocation's janitor
// (T9) prunes.
func Create(ctx context.Context, workspace, headSHA, runID string) (*EphemeralWorktree, error) {
	if strings.TrimSpace(runID) == "" || runID == "." || runID == ".." {
		return nil, fmt.Errorf("worktree: refusing a degenerate run id %q", runID)
	}

	root := filepath.Join(workspace, filepath.FromSlash(ScratchRelPath))
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, fmt.Errorf("worktree: create scratch root: %w", err)
	}
	path := filepath.Join(root, runID)

	// `-c core.hooksPath=/dev/null` disables git hooks for the checkout: a
	// worktree add runs the repo's post-checkout hook, and a proposal can commit
	// a `.githooks/post-checkout` (or the repo sets core.hooksPath to a tracked
	// dir — pyxis's own repos do) — that would run repo-controlled code with
	// codegate's full ambient environment, OUTSIDE the runner's DINV-5 strip,
	// before any test runs. Pointing hooksPath at a path with no hooks closes it.
	if err := runGit(ctx, workspace, "-c", "core.hooksPath=/dev/null", "worktree", "add", "--detach", "--quiet", path, headSHA); err != nil {
		// A failed add may still have registered an administrative entry before
		// erroring; prune it so a half-created worktree can never wedge the origin.
		_ = runGit(ctx, workspace, "worktree", "prune")
		return nil, fmt.Errorf("worktree: create at %s: %w", headSHA, err)
	}
	return &EphemeralWorktree{Path: path, workspace: workspace}, nil
}

// Provision makes the manifest's declared state dirs (node_modules-class
// directories a bare checkout lacks — design §3.9) available inside the
// worktree, at the same workspace-relative path they hold in the origin. Each
// is symlinked from the origin workspace where possible — cheap, and correct
// for content the reverted tests only ever read — falling back to a full copy
// when the platform or filesystem rejects the symlink (e.g. no symlink
// privilege). A declared dir absent from the origin workspace is skipped, not
// an error: a repo with no such state simply has nothing to provision.
func (w *EphemeralWorktree) Provision(ctx context.Context, stateDirs []string) error {
	for _, rel := range stateDirs {
		if err := ctx.Err(); err != nil {
			return err
		}
		rel = filepath.FromSlash(rel)
		src := filepath.Join(w.workspace, rel)
		if _, err := os.Lstat(src); err != nil {
			continue // absent from the origin: nothing to provision, not an error
		}
		dst := filepath.Join(w.Path, rel)
		// A git-TRACKED state dir (e.g. a committed vendor/) is already checked
		// out into the worktree by `worktree add`; leave it rather than erroring
		// trying to overlay an existing directory.
		if _, err := os.Lstat(dst); err == nil {
			continue
		}
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return fmt.Errorf("worktree: provision %s: %w", rel, err)
		}
		// COPY, never symlink: a symlink back to the origin would let untrusted
		// test code (run twice inside this worktree) write THROUGH it into the
		// workspace under verification — both a DINV-7 breach and a cross-leg
		// signalling channel that defeats the anti-tautology floor. A copy keeps
		// every write contained in the throwaway worktree.
		if err := os.CopyFS(dst, os.DirFS(src)); err != nil {
			return fmt.Errorf("worktree: provision %s: %w", rel, err)
		}
	}
	return nil
}

// Remove tears the worktree down via `git worktree remove --force`, so both
// its directory and its registration in the origin repo are gone in one step.
// If the directory was already gone or otherwise unremovable by that call
// (e.g. hand-deleted out from under it), it falls back to removing whatever
// remains and pruning the registration directly — a worktree must never be
// able to wedge the origin repo's `git worktree list` permanently.
func (w *EphemeralWorktree) Remove(ctx context.Context) error {
	// Teardown must run even when the caller's ctx is already cancelled — the
	// common abnormal exit is a leg that hit its bound. Detach from cancellation
	// so `git worktree remove`/`prune` are not refused on arrival, which would
	// leak the .git/worktrees registration until the external janitor prunes it.
	ctx = context.WithoutCancel(ctx)
	if err := runGit(ctx, w.workspace, "worktree", "remove", "--force", w.Path); err != nil {
		rmErr := os.RemoveAll(w.Path)
		pruneErr := runGit(ctx, w.workspace, "worktree", "prune")
		if rmErr != nil || pruneErr != nil {
			return fmt.Errorf("worktree: `git worktree remove` failed (%v); fallback dir-removal=%v, prune=%v", err, rmErr, pruneErr)
		}
	}
	return nil
}

// PruneStaleWorktrees removes every ephemeral worktree registered under the
// workspace's own scratch namespace (ScratchRelPath) — the AP-12 janitor for
// codegate's own worktree residue (design §3.6, §3.9). It is meant to run
// once at the start of every invocation, AFTER the workspace flock is held:
// since only one codegate invocation ever holds that lock at a time, any
// worktree registered under ScratchRelPath at that point necessarily belongs
// to a run that is no longer live — normally reaped by its own deferred
// Remove, but left behind whenever a prior invocation was killed mid-rung-3.
// A worktree directory that is already gone but still git-registered (a
// partial `worktree add`, or a hand-deleted directory) is cleared the same
// way Remove already falls back to for the single-worktree case. One
// unremovable entry does not stop the rest from being pruned.
func PruneStaleWorktrees(ctx context.Context, workspace string) error {
	root := filepath.Join(workspace, filepath.FromSlash(ScratchRelPath))
	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("worktree: read scratch root %s: %w", root, err)
	}

	var errs []error
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		stale := &EphemeralWorktree{Path: filepath.Join(root, e.Name()), workspace: workspace}
		if err := stale.Remove(ctx); err != nil {
			errs = append(errs, fmt.Errorf("prune %s: %w", e.Name(), err))
		}
	}
	return errors.Join(errs...)
}

// gitOpTimeout bounds a SINGLE worktree-administrative git invocation
// (add/remove/prune) — plumbing against the repo's own .git metadata, never
// a repo-supplied verb, so it gets its own short, fixed bound distinct from
// DINV-5's per-verb bounds (mirroring package invoke's own short bound
// around its git-plumbing bindings resolution): a hang here must become a
// bounded rung error, never an indefinite stall.
const gitOpTimeout = 30 * time.Second

// gitLockRetries and gitLockRetryDelay bound the resilience window for a
// TRANSIENT collision on the shared .git metadata a sibling workspace's own
// concurrent worktree operation can legitimately cause (two agent sessions,
// two `git worktree` checkouts, one shared .git) — never an unbounded retry:
// once the budget is exhausted the caller sees a normal, fail-closed error.
const (
	gitLockRetries    = 5
	gitLockRetryDelay = 200 * time.Millisecond
)

// runGitOnceFunc is the injectable single-attempt git runner: tests override
// it to simulate a transient lock collision deterministically without
// needing to provoke real git contention. Production code never reassigns
// it past init.
var runGitOnceFunc = runGitOnce

// runGit runs a git subcommand rooted at dir with credential prompts and
// optional locks disabled, retrying a bounded few times on a RECOGNIZABLE
// transient lock-collision error (a sibling workspace's concurrent worktree
// operation on the same shared .git) before surfacing as a wrapped error.
// Every attempt is individually bounded by gitOpTimeout, so this never
// hangs regardless of whether the error is ever recognized as transient;
// only the recognized shape is retried, so a genuine, non-transient defect
// still surfaces on its first attempt.
func runGit(ctx context.Context, dir string, args ...string) error {
	var lastErr error
	for attempt := 0; attempt <= gitLockRetries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return fmt.Errorf("git %s: %w", strings.Join(args, " "), ctx.Err())
			case <-time.After(gitLockRetryDelay):
			}
		}

		opCtx, cancel := context.WithTimeout(ctx, gitOpTimeout)
		err := runGitOnceFunc(opCtx, dir, args...)
		cancel()
		if err == nil {
			return nil
		}
		lastErr = err
		if !isTransientLockError(err) {
			return err
		}
	}
	return fmt.Errorf("git %s: exhausted retries on a transient lock collision: %w", strings.Join(args, " "), lastErr)
}

// runGitOnce is a single, unretried attempt: exactly the prior behavior of
// runGit before T9 added bounded retries, wrapping stderr into any error.
func runGitOnce(ctx context.Context, dir string, args ...string) error {
	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0", "GIT_OPTIONAL_LOCKS=0")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if msg := strings.TrimSpace(stderr.String()); msg != "" {
			return fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, msg)
		}
		return fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}
	return nil
}

// isTransientLockError reports whether err's message matches git's own
// wording for a contended lock file on its shared administrative state
// (index.lock-class files, and the worktree-specific locks `worktree
// add`/`remove`/`prune` take) — the shape a sibling workspace's concurrent
// worktree operation on the same shared .git produces. Matched by TEXT (git
// has no structured exit code for this), deliberately narrow: anything else
// is treated as a real, non-retryable defect and returned on the first
// attempt rather than masked behind a retry loop.
func isTransientLockError(err error) bool {
	msg := strings.ToLower(err.Error())
	if !strings.Contains(msg, "lock") {
		return false
	}
	return strings.Contains(msg, "file exists") ||
		strings.Contains(msg, "unable to create") ||
		strings.Contains(msg, "unable to obtain") ||
		strings.Contains(msg, "already exists")
}
