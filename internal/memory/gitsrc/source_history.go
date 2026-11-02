package gitsrc

import (
	"context"
	"fmt"
	"path"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/memory/record"
)

// PathCommits lists the first-parent commits that changed one path after base,
// oldest first. Both refs are pinned, and base must be an ancestor of head.
func (r *Repo) PathCommits(ctx context.Context, baseRef, headRef, name string) ([]string, error) {
	if name == "" || path.Clean(name) != name || strings.HasPrefix(name, "../") || strings.HasPrefix(name, "/") {
		return nil, fmt.Errorf("invalid history path %q", name)
	}
	base, err := r.ResolveCommit(ctx, baseRef)
	if err != nil {
		return nil, err
	}
	head, err := r.ResolveCommit(ctx, headRef)
	if err != nil {
		return nil, err
	}
	ancestor, err := r.MergeBase(ctx, base, head)
	if err != nil {
		return nil, err
	}
	if ancestor != base {
		return nil, fmt.Errorf("history base %s is not an ancestor of %s", base, head)
	}
	out, err := r.run(ctx, nil, "rev-list", "--reverse", "--first-parent", base+".."+head, "--", name)
	if err != nil {
		return nil, fmt.Errorf("read path history for %s: %w", name, err)
	}
	var commits []string
	for line := range strings.SplitSeq(strings.TrimSpace(string(out)), "\n") {
		if line != "" {
			commits = append(commits, line)
		}
	}
	return commits, nil
}

// SourceIntroduction pins the most recent addition, rather than a path's
// oldest appearance. A deletion and reintroduction must invalidate old seals.
func (r *Repo) SourceIntroduction(ctx context.Context, commit, name string) (string, error) {
	return r.sourceEvent(ctx, commit, name, "A")
}

// BeforeSourceDeletion returns the first-parent snapshot immediately before
// the most recent deletion, including a squash deletion's actual parent.
func (r *Repo) BeforeSourceDeletion(ctx context.Context, commit, name string) (string, error) {
	deleted, err := r.sourceEvent(ctx, commit, name, "D")
	if err != nil {
		return "", err
	}
	return r.ResolveCommit(ctx, deleted+"^1")
}

func (r *Repo) sourceEvent(ctx context.Context, commit, name, filter string) (string, error) {
	shallow, err := r.run(ctx, nil, "rev-parse", "--is-shallow-repository")
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(string(shallow)) != "false" {
		return "", fmt.Errorf("source history requires a complete retained repository")
	}
	out, err := r.read(ctx, record.ValidCommitID(commit), "log", "--first-parent", "--diff-merges=first-parent", "--no-patch", "--no-renames", "--format=%H", "--diff-filter="+filter, "-1", commit, "--", name)
	if err != nil {
		return "", fmt.Errorf("read source history for %s: %w", name, err)
	}
	identity := strings.TrimSpace(string(out))
	if identity == "" {
		return "", fmt.Errorf("source %s has no retained %s event", name, filter)
	}
	return identity, nil
}

func (r *Repo) FirstParentAncestor(ctx context.Context, ancestor, descendant string) error {
	out, err := r.read(ctx, record.ValidCommitID(descendant), "rev-list", "--first-parent", descendant)
	if err != nil {
		return fmt.Errorf("read retained mainline: %w", err)
	}
	for line := range strings.SplitSeq(string(out), "\n") {
		if line == ancestor {
			return nil
		}
	}
	return fmt.Errorf("commit %s is not on the retained first-parent history of %s", ancestor, descendant)
}
