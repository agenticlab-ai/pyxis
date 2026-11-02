// Package githooks owns the single composed desk and executor hook installation.
package githooks

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
)

// guardedHooks are the client-side "about to create a commit from the working
// tree/index" hooks. Guarding all three closes every ordinary commit path into
// the parent checkout: an interactive commit (pre-commit), a merge commit that
// skips pre-commit (pre-merge-commit), and a patch applied by git am
// (pre-applypatch). They share one guard body.
var guardedHooks = []string{"pre-commit", "pre-merge-commit", "pre-applypatch"}

// All Git hook entrypoints share the same installation. Forwarders are present
// even before a repository adds a hook, so adding pre-push later needs no rewire.
var forwardedHooks = []string{
	"applypatch-msg", "pre-applypatch", "post-applypatch", "pre-commit",
	"pre-merge-commit", "prepare-commit-msg", "commit-msg", "post-commit",
	"pre-rebase", "post-checkout", "post-merge", "pre-push", "pre-receive",
	"update", "proc-receive", "post-receive", "post-update", "reference-transaction",
	"push-to-checkout", "pre-auto-gc", "post-rewrite", "sendemail-validate",
	"fsmonitor-watchman", "p4-changelist", "p4-prepare-changelist",
	"p4-post-changelist", "p4-pre-submit", "post-index-change",
}

const guardDirName = "pyxis-guard-hooks"

type repository struct{ root string }

const hookSourceKey = "pyxis.repositoryHooksPath"

// Install is the shared desk/executor hook installer. The adapter-owned
// directory is a dispatcher, not a second set of repository checks: it guards
// parent commits and forwards Git's original argv/stdin to the repository hooks.
// Reinstallation preserves the source path and never nests another dispatcher.
func Install(ctx context.Context, parent string) error {
	root, err := git(ctx, parent, "rev-parse", "--show-toplevel")
	if err != nil {
		return fmt.Errorf("locating the repository for hook setup: %w", err)
	}
	return (&repository{root: root}).install(ctx)
}

// guardHook is the mechanical parent-checkout commit-guard (AP-20 / DINV-1). It
// is shared by every worktree through core.hooksPath, so it must tell the parent
// checkout's main worktree — which it rejects — from the linked per-task
// worktrees, which it permits. git answers that itself: in the main worktree the
// per-worktree git dir *is* the common git dir; in a linked worktree it is a
// distinct subdirectory of it. The hook needs no state and no configuration.
//
// It is a genuine mechanical guard: git runs it on every ordinary commit, so a
// worker that wanders out of its worktree and commits in the parent is stopped
// by default, not by convention. It does not defend against `--no-verify` or a
// raw filesystem write — those are the OS/sandbox boundary the design leaves to
// ambient-local policy (NG-2 / NG-3), not this git-level guard.
const guardHook = `#!/bin/sh
# pyxis workspace guard (AP-20 / DINV-1): the parent checkout is never mutated by
# a worker -- workers commit only inside their own per-task worktree. This hook
# rejects any commit created in the parent checkout's main worktree and permits
# commits in the linked worktrees the executor creates. It is installed and owned
# by internal/githooks via core.hooksPath; do not edit by hand.
gitdir=$(cd "$(git rev-parse --git-dir)" 2>/dev/null && pwd -P) || exit 1
common=$(cd "$(git rev-parse --git-common-dir)" 2>/dev/null && pwd -P) || exit 1
if [ "$gitdir" = "$common" ]; then
  echo "pyxis: refusing to mutate the parent checkout; a worker commits only in its own worktree (AP-20/DINV-1)" >&2
  exit 1
fi
exit 0
`

// install installs the one dispatcher and preserves the existing hook
// source, including pre-push and prepare-commit-msg. Supervisor Git operations
// still disable hooks explicitly; repository code never runs outside their
// verification boundary as a side effect of this installation.
func (r *repository) install(ctx context.Context) error {
	commonDir, err := r.commonGitDir(ctx)
	if err != nil {
		return fmt.Errorf("locating the parent git dir: %w", err)
	}
	guardDir := filepath.Join(commonDir, guardDirName)
	source, err := r.hookSource(ctx, commonDir, guardDir)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(guardDir, 0o755); err != nil {
		return fmt.Errorf("creating the guard hooks dir %q: %w", guardDir, err)
	}
	for _, hook := range forwardedHooks {
		path := filepath.Join(guardDir, hook)
		body := hookDispatcher(source, hook)
		// Rearming an unchanged installation must not truncate live hooks while
		// another worktree is committing or pushing through them.
		if existing, err := os.ReadFile(path); err != nil || string(existing) != body {
			if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
				return fmt.Errorf("writing the guard hook %q: %w", path, err)
			}
		}
		// WriteFile keeps an existing file's mode, so re-assert the executable bit
		// on a rewrite — an un-executable hook would silently disarm the guard.
		if err := os.Chmod(path, 0o755); err != nil {
			return fmt.Errorf("marking the guard hook %q executable: %w", path, err)
		}
	}
	if _, err := git(ctx, r.root, "config", "--local", hookSourceKey, source); err != nil {
		return fmt.Errorf("recording the repository hook source: %w", err)
	}
	if _, err := git(ctx, r.root, "config", "core.hooksPath", guardDir); err != nil {
		return fmt.Errorf("pointing core.hooksPath at the guard: %w", err)
	}
	return nil
}

func (r *repository) hookSource(ctx context.Context, commonDir, guardDir string) (string, error) {
	active, err := r.hookConfig(ctx, "core.hooksPath")
	if err != nil {
		return "", err
	}
	if sameHookDir(r.root, active, guardDir) {
		source, err := r.hookConfig(ctx, hookSourceKey)
		if err != nil {
			return "", err
		}
		if source != "" {
			if sameHookDir(r.root, source, guardDir) {
				return "", fmt.Errorf("repository hook source points back to the dispatcher: %q", source)
			}
			return source, nil
		}
		// Upgrade the old guard-only installation, which never recorded the
		// displaced hook source. Pyxis's tracked .githooks is the canonical source.
		active = ""
	}
	if active != "" {
		return active, nil
	}
	if info, err := os.Stat(filepath.Join(r.root, ".githooks")); err == nil && info.IsDir() {
		return ".githooks", nil
	}
	return filepath.Join(commonDir, "hooks"), nil
}

// Config inspection must not use runGit: its command-scoped hooksPath=/dev/null
// is a supervisor security boundary, not the repository's effective hook path.
func (r *repository) hookConfig(ctx context.Context, key string) (string, error) {
	out, err := exec.CommandContext(ctx, "git", "-C", r.root, "config", "--get", key).Output()
	if err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) && exit.ExitCode() == 1 {
			return "", nil // Git reports an absent config key with status 1.
		}
		return "", fmt.Errorf("reading %s: %w", key, err)
	}
	return strings.TrimRight(string(out), "\n"), nil
}

func sameHookDir(root, left, right string) bool {
	if left == "" {
		return false
	}
	if !filepath.IsAbs(left) {
		left = filepath.Join(root, left)
	}
	if resolved, err := filepath.EvalSymlinks(left); err == nil {
		left = resolved
	}
	if resolved, err := filepath.EvalSymlinks(right); err == nil {
		right = resolved
	}
	return filepath.Clean(left) == filepath.Clean(right)
}

func hookDispatcher(source, hook string) string {
	body := "#!/bin/sh\n# Managed by pyxis hooks-setup and the executor; edit the repository hook source.\nset -e\n"
	if slices.Contains(guardedHooks, hook) {
		body += strings.TrimSuffix(strings.TrimPrefix(guardHook, "#!/bin/sh\n"), "exit 0\n")
	}
	// Quoted literals are generated in Go; the shim only locates and delegates.
	path := filepath.ToSlash(filepath.Join(source, hook))
	quoted := "'" + strings.ReplaceAll(path, "'", "'\"'\"'") + "'"
	if !filepath.IsAbs(source) {
		quoted = `"$(git rev-parse --show-toplevel)"/` + quoted
	}
	body += "HOOK=" + quoted + "\n"
	body += "if [ -x \"$HOOK\" ]; then exec \"$HOOK\" \"$@\"; fi\n"
	return body
}

// commonGitDir resolves the parent checkout's common git directory to an
// absolute path — the directory shared by the parent's main worktree and every
// linked worktree, and the natural home for the adapter-owned guard hooks.
func (r *repository) commonGitDir(ctx context.Context) (string, error) {
	out, err := git(ctx, r.root, "rev-parse", "--git-common-dir")
	if err != nil {
		return "", err
	}
	if !filepath.IsAbs(out) {
		// git reports the common dir relative to the queried worktree (the parent).
		out = filepath.Join(r.root, out)
	}
	return out, nil
}

func git(ctx context.Context, root string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", append([]string{"-c", "core.hooksPath=/dev/null", "-c", "core.fsmonitor=false", "-C", root}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %s: %w: %s", args[0], err, strings.TrimSpace(string(out)))
	}
	return strings.TrimSpace(string(out)), nil
}
