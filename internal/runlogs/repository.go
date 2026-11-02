package runlogs

import (
	"context"
	"os/exec"
	"path/filepath"
	"strings"
)

type repositoryKey struct{}

// WithRepository binds the host's owning repository for diagnostics emitted
// while working in an independent disposable clone. It grants no child access
// to that repository. An empty owner leaves the existing binding unchanged.
func WithRepository(ctx context.Context, repo string) context.Context {
	if strings.TrimSpace(repo) == "" {
		return ctx
	}
	return context.WithValue(ctx, repositoryKey{}, repo)
}

// Repository returns the persistent owning checkout for a linked worktree or
// an explicitly bound owner. Agent diagnostics must outlive disposable clones.
// Non-Git workspaces (including initialization) retain logs in their own root.
func Repository(ctx context.Context, workspace string) string {
	if owner, ok := ctx.Value(repositoryKey{}).(string); ok {
		workspace = owner
	}
	if workspace == "" {
		return ""
	}
	cmd := exec.CommandContext(ctx, "git", "-C", workspace, "rev-parse", "--path-format=absolute", "--git-common-dir")
	if out, err := cmd.Output(); err == nil {
		common := strings.TrimSpace(string(out))
		if filepath.IsAbs(common) && filepath.Base(common) == ".git" {
			return filepath.Dir(common)
		}
	}
	return workspace
}
