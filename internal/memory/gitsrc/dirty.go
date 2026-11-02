package gitsrc

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"sort"
	"strings"
)

// Git status can execute arbitrary configured clean filters while refreshing
// the index. Evidence checks instead compare raw filesystem bytes with pinned
// blobs. Checkout transformations must therefore preserve these exact bytes.
// DirtyPaths checks the union of declared paths with one batch of Git reads.
func (r *Repo) DirtyPaths(ctx context.Context, prefixes []string) (bool, error) {
	paths, err := r.DirtyPathNames(ctx, prefixes)
	return len(paths) > 0, err
}

// DirtyPathNames enumerates the exact dirty paths using the same raw-byte
// checks as evidence capture, without executing Git content filters.
func (r *Repo) DirtyPathNames(ctx context.Context, prefixes []string) ([]string, error) {
	if len(prefixes) == 0 {
		return nil, nil
	}
	head, err := r.Head(ctx)
	if err != nil {
		return nil, err
	}
	dirty := map[string]bool{}
	for _, args := range [][]string{
		{"diff-index", "--cached", "--no-renames", "--name-only", "--no-ext-diff", "--no-textconv", "-z", head, "--"},
		{"ls-files", "--others", "--exclude-standard", "-z", "--"},
	} {
		out, err := r.run(ctx, nil, append(args, prefixes...)...)
		if err != nil {
			return nil, err
		}
		for name := range strings.SplitSeq(string(out), "\x00") {
			if name != "" {
				dirty[name] = true
			}
		}
	}
	entries, err := r.EvidenceTree(ctx, head)
	if err != nil {
		return nil, err
	}
	var selected []TreeEntry
	var paths []string
	for _, e := range entries {
		matched := false
		for _, prefix := range prefixes {
			matched = matched || prefix == "." || e.Path == prefix || strings.HasPrefix(e.Path, prefix+"/")
		}
		if !matched {
			continue
		}
		if e.Type != "blob" {
			return nil, fmt.Errorf("cannot compare non-blob working path %s without executing a substrate", e.Path)
		}
		selected = append(selected, e)
		paths = append(paths, e.Path)
	}
	blobs, err := r.Files(ctx, head, paths)
	if err != nil {
		return nil, err
	}
	root, err := os.OpenRoot(r.Root)
	if err != nil {
		return nil, err
	}
	defer func() { _ = root.Close() }() // Closing a read-only directory handle has no durable effect.
	for _, e := range selected {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		info, err := root.Lstat(e.Path)
		if errors.Is(err, fs.ErrNotExist) {
			dirty[e.Path] = true
			continue
		}
		if err != nil {
			return nil, err
		}
		var body []byte
		if e.Mode == "120000" {
			if info.Mode()&fs.ModeSymlink == 0 {
				dirty[e.Path] = true
				continue
			}
			target, readErr := root.Readlink(e.Path)
			err = readErr
			body = []byte(target)
		} else {
			if !info.Mode().IsRegular() {
				dirty[e.Path] = true
				continue
			}
			mode := "100644"
			if info.Mode().Perm()&0o111 != 0 {
				mode = "100755"
			}
			if mode != e.Mode {
				dirty[e.Path] = true
				continue
			}
			body, err = root.ReadFile(e.Path)
		}
		if err != nil {
			return nil, err
		}
		expected, ok := blobs[e.Path]
		if !ok {
			return nil, fmt.Errorf("missing pinned working-tree blob %s", e.Path)
		}
		if !bytes.Equal(expected, body) {
			dirty[e.Path] = true
		}
	}
	out := make([]string, 0, len(dirty))
	for name := range dirty {
		out = append(out, name)
	}
	sort.Strings(out)
	return out, nil
}
