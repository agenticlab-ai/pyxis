package adopt

// The tree-state dispatch (design existing-repo-onboarding-v2 §3.1): `adopt`
// decides what kind of target it is holding BEFORE any classification runs.
// A target carrying no .git entry has nothing to negotiate with — refused by
// name (UB-5). A git repository with zero tracked files and a clean
// worktree is the anti-segment: there is nothing to negotiate either, but
// unlike the non-git case it is a legitimate target for the empty-repo
// installer, so Inspect routes it to init rather than refusing (UB-1). Only
// a non-empty git repository proceeds to classification.
//
// This mirrors internal/installer's own admission probe in spirit (a .git
// check, then `git ls-files` + `git status --porcelain`), but the verdict
// differs on purpose: init treats "no .git" as "I will git init"; adopt
// treats it as a hard refusal — there is no repository to adopt. The two
// packages do not share this code (installer's admitTarget is unexported,
// and the verdicts diverge), so this is a small, deliberate duplication of
// the underlying git probes, not a second implementation of the same
// predicate.

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/runlogs"
)

// ErrNotGitRepository is UB-5's refusal: the target itself carries no .git
// entry (directory or, for a linked worktree, file), so there is nothing to
// negotiate — refused by name before any classification of the tree's
// content (prd:existing-repo-onboarding/UB-5).
var ErrNotGitRepository = errors.New("adopt: the target is not a git repository")

// isGitRepository reports whether target carries a .git entry — the same
// shallow, read-only admission probe installer's admitTarget makes.
func isGitRepository(target string) (bool, error) {
	gitDir := filepath.Join(target, ".git")
	if _, err := os.Stat(gitDir); err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("adopt: probe %s: %w", gitDir, err)
	}
	return true, nil
}

// treeStatus reads target's tracked file list and working-tree status —
// UB-1's emptiness predicate: zero tracked files and a clean worktree is the
// adoptable-empty-repo anti-segment (design §3.1; brainstorm:existing-repo-onboarding#D-10).
func treeStatus(ctx context.Context, target string) (tracked, dirty []string, err error) {
	tracked, err = gitLines(ctx, target, "ls-files")
	if err != nil {
		return nil, nil, fmt.Errorf("adopt: list tracked files in %s: %w", target, err)
	}
	dirty, err = gitLines(ctx, target, "status", "--porcelain", "--untracked-files=all", "--", ".", ":(exclude)"+runlogs.Root)
	if err != nil {
		return nil, nil, fmt.Errorf("adopt: read worktree status in %s: %w", target, err)
	}
	return tracked, dirty, nil
}

// gitLines runs one git subcommand in target and splits stdout into
// non-empty lines.
func gitLines(ctx context.Context, target string, args ...string) ([]string, error) {
	out, err := gitOutput(ctx, target, args...)
	if err != nil {
		return nil, err
	}
	var lines []string
	for _, ln := range strings.Split(out, "\n") {
		if strings.TrimSpace(ln) != "" {
			lines = append(lines, ln)
		}
	}
	return lines, nil
}

// gitOutput runs one git subcommand in target, honoring ctx, returning
// stdout. A non-zero exit wraps stderr so refusals stay named.
func gitOutput(ctx context.Context, target string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", target}, args...)...)
	var out, errOut bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errOut
	if err := cmd.Run(); err != nil {
		if msg := strings.TrimSpace(errOut.String()); msg != "" {
			return "", fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, msg)
		}
		return "", fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}
	return out.String(), nil
}
