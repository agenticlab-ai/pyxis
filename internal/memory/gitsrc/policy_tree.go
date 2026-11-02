package gitsrc

import (
	"bytes"
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/memory/record"
)

// TreeNodes includes directories (even empty trees) and decodes names without
// Git's display quoting. Closed policy roots must account for every node.
func (r *Repo) TreeNodes(ctx context.Context, ref string, prefixes ...string) ([]TreeEntry, error) {
	args := append([]string{"ls-tree", "-r", "-t", "-z", ref, "--"}, prefixes...)
	out, err := r.read(ctx, record.ValidCommitID(ref), args...)
	if err != nil {
		return nil, err
	}
	var entries []TreeEntry
	for line := range bytes.SplitSeq(out, []byte{0}) {
		if len(line) == 0 {
			continue
		}
		meta, name, ok := strings.Cut(string(line), "\t")
		fields := strings.Fields(meta)
		if !ok || len(fields) != 3 {
			return nil, fmt.Errorf("invalid ls-tree node %q", line)
		}
		entries = append(entries, TreeEntry{Mode: fields[0], Type: fields[1], Path: name})
	}
	return entries, nil
}

// ChangedPaths lists both sides of renames. No path can be hidden by a rename
// while checking that a migration changes only its policy representation.
func (r *Repo) ChangedPaths(ctx context.Context, base, head string) ([]string, error) {
	out, err := r.run(ctx, nil, "diff", "--no-renames", "--name-only", "-z", base, head, "--")
	if err != nil {
		return nil, err
	}
	return nulPaths(out), nil
}

// WorkChanges is the complete working/index delta, including untracked paths.
// Index paths let a writer validate staged bytes separately from worktree bytes.
type WorkChanges struct {
	Paths      []string
	IndexPaths []string
}

func (r *Repo) WorkingChanges(ctx context.Context) (WorkChanges, error) {
	out, err := r.run(ctx, nil, "status", "--porcelain=v1", "-z", "--untracked-files=all", "--no-renames")
	if err != nil {
		return WorkChanges{}, err
	}
	changes := WorkChanges{}
	for line := range bytes.SplitSeq(out, []byte{0}) {
		if len(line) == 0 {
			continue
		}
		if len(line) < 4 || line[2] != ' ' {
			return WorkChanges{}, fmt.Errorf("invalid git status entry %q", line)
		}
		name := string(line[3:])
		changes.Paths = append(changes.Paths, name)
		if line[0] != ' ' && line[0] != '?' {
			changes.IndexPaths = append(changes.IndexPaths, name)
		}
	}
	sort.Strings(changes.Paths)
	sort.Strings(changes.IndexPaths)
	return changes, nil
}

// IndexEntries exposes modes and refuses an unmerged index, before any write.
func (r *Repo) IndexEntries(ctx context.Context, prefixes ...string) ([]TreeEntry, error) {
	args := append([]string{"ls-files", "--stage", "-z", "--"}, prefixes...)
	out, err := r.run(ctx, nil, args...)
	if err != nil {
		return nil, err
	}
	var entries []TreeEntry
	for line := range bytes.SplitSeq(out, []byte{0}) {
		if len(line) == 0 {
			continue
		}
		meta, name, ok := strings.Cut(string(line), "\t")
		fields := strings.Fields(meta)
		if !ok || len(fields) != 3 || fields[2] != "0" {
			return nil, fmt.Errorf("invalid or unmerged index entry %q", line)
		}
		entries = append(entries, TreeEntry{Mode: fields[0], Type: "blob", Path: name})
	}
	return entries, nil
}

func nulPaths(body []byte) []string {
	var paths []string
	for p := range bytes.SplitSeq(body, []byte{0}) {
		if len(p) != 0 {
			paths = append(paths, string(p))
		}
	}
	sort.Strings(paths)
	return paths
}
