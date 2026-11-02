package lifecycle

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// rollbackDraftStart restores the clean caller worktree after opening or
// reopening fails. A committed draft branch remains available for recovery.
func rollbackDraftStart(ctx context.Context, root, branch, priorBranch, priorHead string, written []string, committed bool, run RunCommand) error {
	cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 20*time.Second)
	defer cancel()
	var errs []error
	if !committed {
		for _, name := range written {
			if err := os.Remove(filepath.Join(root, name)); err != nil && !os.IsNotExist(err) {
				errs = append(errs, fmt.Errorf("remove partial draft file %s: %w", name, err))
			}
		}
	}
	if _, err := run(cleanupCtx, root, nil, "git", "reset", "--hard"); err != nil {
		errs = append(errs, fmt.Errorf("reset failed draft branch: %w", err))
	}
	if priorBranch != "" {
		if _, err := run(cleanupCtx, root, nil, "git", "switch", priorBranch); err != nil {
			errs = append(errs, fmt.Errorf("restore prior branch %s: %w", priorBranch, err))
		}
	} else if priorHead != "" {
		if _, err := run(cleanupCtx, root, nil, "git", "switch", "--detach", priorHead); err != nil {
			errs = append(errs, fmt.Errorf("restore prior head %s: %w", priorHead, err))
		}
	}
	if !committed && (priorBranch != "" || priorHead != "") {
		if _, err := run(cleanupCtx, root, nil, "git", "branch", "-D", branch); err != nil {
			errs = append(errs, fmt.Errorf("remove incomplete draft branch %s: %w", branch, err))
		}
	}
	return errors.Join(errs...)
}
