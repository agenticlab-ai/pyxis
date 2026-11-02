package workspace

import (
	"context"
	"path/filepath"

	"github.com/agenticlab-ai/pyxis/internal/githooks"
)

// The executor and the desk share one installer, preserving every repository
// hook whenever the parent-checkout guard is rearmed.
func (w *Workspace) ensureGuard(ctx context.Context) error {
	return githooks.Install(ctx, w.parent)
}

// commonGitDir resolves the shared metadata directory for workspace diagnostics.
func (w *Workspace) commonGitDir(ctx context.Context) (string, error) {
	out, err := runGit(ctx, w.parent, "rev-parse", "--git-common-dir")
	if err != nil {
		return "", err
	}
	if !filepath.IsAbs(out) {
		out = filepath.Join(w.parent, out)
	}
	return out, nil
}
