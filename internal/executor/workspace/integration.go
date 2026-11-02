package workspace

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/executor/port"
)

// RefreshMainline updates a quiescent, checkpointed candidate by an append-only
// merge. It never rewrites task commits or publishes. Every verification gets
// an exact fetched base; unrelated fetches cannot change that check's input.
func (d *Workdir) RefreshMainline(ctx context.Context) (string, error) {
	if d.workspace == nil {
		return d.base, nil // directly constructed workdirs have no fetch owner
	}
	status, err := runGit(ctx, d.dir, "status", "--porcelain")
	if err != nil {
		return "", err
	}
	if status != "" {
		return "", fmt.Errorf("workspace: refusing integration of uncheckpointed branch %s", d.branch)
	}
	branch, err := runGit(ctx, d.dir, "symbolic-ref", "--short", "HEAD")
	if err != nil {
		return "", err
	}
	if branch != d.branch {
		return "", fmt.Errorf("workspace: refusing integration on branch %q; expected %q", branch, d.branch)
	}
	base, err := d.workspace.armAndPinBase(ctx, port.Task{ID: d.taskID})
	if err != nil {
		return "", err
	}
	if _, err := runGit(ctx, d.dir, "merge-base", "--is-ancestor", base, "HEAD"); err == nil {
		d.base = base
		return base, nil
	} else {
		var exit *exec.ExitError
		if !errors.As(err, &exit) || exit.ExitCode() != 1 {
			return "", err
		}
	}
	head, err := runGit(ctx, d.dir, "rev-parse", "HEAD")
	if err != nil {
		return "", err
	}
	if _, err := runGit(ctx, d.dir, "merge", "--no-edit", "--no-ff", base); err != nil {
		// Abort only the merge we started. The clean checkpoint remains the
		// resumable artifact, and conflicts are reported with both exact pins.
		abortCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
		defer cancel()
		_, abortErr := runGit(abortCtx, d.dir, "merge", "--abort")
		return "", errors.Join(fmt.Errorf("workspace: mainline integration failed (candidate %s, base %s); checkpoint retained: %w", head, base, err), abortErr)
	}
	d.base = base
	return base, nil
}

// ValidateIntegration never changes a reviewed head. A base that advances
// during verification requires a fresh integration/gate/review before publish.
func (d *Workdir) ValidateIntegration(ctx context.Context) error {
	if d.workspace == nil {
		return nil
	}
	base, err := d.workspace.armAndPinBase(ctx, port.Task{ID: d.taskID})
	if err != nil {
		return err
	}
	if base != d.base {
		return fmt.Errorf("workspace: mainline advanced after verification (verified base %s, current base %s); refresh integration, gate and exact-head review before publication", d.base, base)
	}
	return nil
}
