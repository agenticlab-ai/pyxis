package gitsrc

import (
	"context"
	"fmt"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/memory/record"
)

// FullHistory refuses a shallow boundary instead of treating it as a root.
// All subsequent object reads disable lazy fetching and fail on missing data.
func (r *Repo) FullHistory(ctx context.Context) error {
	out, err := r.run(ctx, nil, "rev-parse", "--is-shallow-repository")
	if err != nil {
		return err
	}
	if strings.TrimSpace(string(out)) != "false" {
		return fmt.Errorf("evidence-history-unavailable: a full, non-shallow mainline is required")
	}
	return nil
}

// EvidenceTree uses NUL delimiters so Git quoting cannot reinterpret a path.
func (r *Repo) EvidenceTree(ctx context.Context, commit string) ([]TreeEntry, error) {
	if !record.ValidCommitID(commit) {
		return nil, fmt.Errorf("evidence tree needs a pinned commit")
	}
	out, err := r.read(ctx, true, "ls-tree", "-rz", "--full-tree", commit)
	if err != nil {
		return nil, err
	}
	var entries []TreeEntry
	for line := range strings.SplitSeq(string(out), "\x00") {
		if line == "" {
			continue
		}
		meta, p, ok := strings.Cut(line, "\t")
		fields := strings.Fields(meta)
		if !ok || len(fields) != 3 || !record.ValidEvidencePath(p) {
			return nil, fmt.Errorf("invalid evidence tree entry %q", line)
		}
		entries = append(entries, TreeEntry{Path: p, Mode: fields[0], Type: fields[1]})
	}
	return entries, nil
}

// TreeChange describes one exact path change against a commit's first parent.
type TreeChange struct {
	Commit  string
	Path    string
	OldMode string
	NewMode string
	OldOID  string
	NewOID  string
}

// FirstParentChanges reads all exact-path events in a single traversal. Root
// and merge additions are included; path simplification and rename following
// are disabled. A path deletion/readdition remains two separate events.
func (r *Repo) FirstParentChanges(ctx context.Context, commit, prefix string) ([]TreeChange, error) {
	if !record.ValidCommitID(commit) || !record.ValidEvidencePath(prefix) {
		return nil, fmt.Errorf("history needs a pinned commit and canonical prefix")
	}
	out, err := r.read(ctx, true, "log", "--first-parent", "--full-history", "--reverse", "--root", "--format=%x00%H%x00", "--raw", "-z", "--no-abbrev", "--no-renames", "--diff-merges=first-parent", commit, "--", prefix)
	if err != nil {
		return nil, err
	}
	tokens := strings.Split(string(out), "\x00")
	var changes []TreeChange
	var current string
	for i := 0; i < len(tokens); i++ {
		token := strings.TrimPrefix(tokens[i], "\n")
		if token == "" {
			continue
		}
		if record.ValidCommitID(token) {
			current = token
			continue
		}
		fields := strings.Fields(token)
		if current == "" || len(fields) != 5 || !strings.HasPrefix(fields[0], ":") || i+1 >= len(tokens) {
			return nil, fmt.Errorf("malformed evidence history at %q", token)
		}
		i++
		p := tokens[i]
		if !record.ValidEvidencePath(p) {
			return nil, fmt.Errorf("invalid evidence history path %q", p)
		}
		changes = append(changes, TreeChange{Commit: current, Path: p, OldMode: strings.TrimPrefix(fields[0], ":"), NewMode: fields[1], OldOID: fields[2], NewOID: fields[3]})
	}
	return changes, nil
}

// Ancestor requires actual ancestry, not merely an object in this clone.
func (r *Repo) Ancestor(ctx context.Context, ancestor, descendant string) (bool, error) {
	if !record.ValidCommitID(ancestor) || !record.ValidCommitID(descendant) {
		return false, fmt.Errorf("ancestry needs pinned commits")
	}
	base, err := r.MergeBase(ctx, ancestor, descendant)
	if err != nil {
		return false, err
	}
	return base == ancestor, nil
}
