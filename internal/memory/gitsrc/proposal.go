package gitsrc

import (
	"context"
	"fmt"
	"strings"
)

// RequireIgnored refuses any destination that Git would track, including
// already tracked files. It uses the same hardened, local-only Git reader as
// evidence operations and checks every destination, not just its directory.
func (r *Repo) RequireIgnored(ctx context.Context, paths []string) error {
	if len(paths) == 0 {
		return nil
	}
	in := []byte(strings.Join(paths, "\x00") + "\x00")
	// check-ignore consumes literal filenames itself and rejects Git's global
	// literal-pathspec magic. This override leaves every other guard intact.
	out, err := r.run(ctx, in, "--no-literal-pathspecs", "check-ignore", "--stdin", "-z")
	if err != nil {
		return fmt.Errorf("archive destinations must be Git-ignored and untracked: %w", err)
	}
	ignored := map[string]bool{}
	for p := range strings.SplitSeq(string(out), "\x00") {
		ignored[p] = true
	}
	for _, p := range paths {
		if !ignored[p] {
			return fmt.Errorf("archive destination is not Git-ignored and untracked: %s", p)
		}
	}
	return nil
}

// IndexChangedPaths reads only the index delta, without refreshing the index
// or executing clean filters. Proposal withdrawal never changes this delta.
func (r *Repo) IndexChangedPaths(ctx context.Context, ref string, prefixes ...string) ([]string, error) {
	args := append([]string{"diff-index", "--cached", "--no-renames", "--name-only", "--no-ext-diff", "--no-textconv", "-z", ref, "--"}, prefixes...)
	out, err := r.run(ctx, nil, args...)
	if err != nil {
		return nil, err
	}
	return nulPaths(out), nil
}
