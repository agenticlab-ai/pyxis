package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"

	"github.com/agenticlab-ai/pyxis/internal/memory/store"
)

func proposalIntent(out *proposalDiscardOutput) []byte {
	body, _ := json.MarshalIndent(out.proposalDiscardPlan, "", "  ") // This closed struct contains only JSON-supported types.
	return append(body, '\n')
}

func proposalArchivePaths(out *proposalDiscardOutput) []string {
	paths := []string{out.Archive + "/", out.Archive + "/intent.json"}
	for _, file := range out.Files {
		paths = append(paths, out.Archive+"/"+file.Path)
	}
	return paths
}

func checkProposalArchive(ctx context.Context, root *os.Root, out *proposalDiscardOutput, complete bool) error {
	files := append([]proposalDiscardFile(nil), out.Files...)
	files = append(files, proposalDiscardFile{Path: "intent.json", Bytes: proposalIntent(out)})
	for _, file := range files {
		name := out.Archive + "/" + file.Path
		parent, err := proposalParent(ctx, root, name, false)
		if !complete && errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return err
		}
		info, err := parent.Lstat(path.Base(name))
		if !complete && errors.Is(err, fs.ErrNotExist) {
			closePolicyRoot(parent)
			continue
		}
		if err == nil && !info.Mode().IsRegular() {
			err = fmt.Errorf("archive %s is not a regular file", name)
		}
		if err == nil {
			body, readErr := parent.ReadFile(path.Base(name))
			err = readErr
			if err == nil && !bytes.Equal(body, file.Bytes) {
				err = fmt.Errorf("archive differs from committed proposal: %s", name)
			}
		}
		err = errors.Join(err, parent.Close())
		if err != nil {
			return err
		}
	}
	return nil
}

func checkProposalState(ctx context.Context, s *evidenceSession, root *os.Root, out *proposalDiscardOutput) error {
	if err := s.unchangedPins(ctx); err != nil {
		return err
	}
	if err := s.m.repo.RequireIgnored(ctx, proposalArchivePaths(out)); err != nil {
		return err
	}
	staged, err := s.m.repo.IndexChangedPaths(ctx, s.head, store.Dir)
	if err != nil {
		return err
	}
	if len(staged) != 0 {
		return fmt.Errorf("memory index differs from committed HEAD: %s", staged[0])
	}
	wanted := map[string]bool{}
	missing := false
	for _, file := range out.Files {
		if err := ctx.Err(); err != nil {
			return err
		}
		present, err := proposalFilePresent(ctx, root, file)
		if err != nil {
			return err
		}
		missing = missing || !present
		wanted[file.Path] = true
	}
	if err := checkProposalArchive(ctx, root, out, missing); err != nil {
		return fmt.Errorf("proposal recovery archive (complete required=%t): %w", missing, err)
	}
	dirty, err := s.m.repo.DirtyPathNames(ctx, []string{store.Dir})
	if err != nil {
		return err
	}
	for _, p := range dirty {
		if !wanted[p] {
			return fmt.Errorf("unselected memory differs from committed HEAD: %s", p)
		}
	}
	entries, err := s.m.repo.ListTree(ctx, s.head, store.Dir)
	if err != nil {
		return err
	}
	committed := make(map[string]bool, len(entries))
	for _, entry := range entries {
		committed[entry.Path] = true
	}
	// Git's untracked inventory excludes ignored additions. Memory loads them,
	// so enumerate the complete confined working tree before any withdrawal.
	if err := fs.WalkDir(root.FS(), store.Dir, func(name string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if !entry.IsDir() {
			if _, exists := committed[name]; !exists {
				return fmt.Errorf("unselected memory is absent from committed HEAD: %s", name)
			}
		}
		return nil
	}); err != nil {
		return err
	}
	return s.unchangedPins(ctx)
}

// The filesystem functions are injectable for meaningful write-failure and
// concurrent-edit tests. Production uses only the confined native functions.
type proposalDiscardOps struct {
	archive func(context.Context, *os.Root, string, []byte, fs.FileMode) error
	remove  func(context.Context, *os.Root, proposalDiscardFile) error
	restore func(context.Context, *os.Root, proposalDiscardFile) error
}

func nativeProposalOps(archive string, ignored func(context.Context, []string) error) proposalDiscardOps {
	return proposalDiscardOps{archive: saveProposalFile, remove: func(ctx context.Context, root *os.Root, file proposalDiscardFile) error {
		return removeProposalFile(ctx, root, file, archive, ignored)
	}, restore: func(ctx context.Context, root *os.Root, file proposalDiscardFile) error {
		return restoreProposalFile(ctx, root, file, archive)
	}}
}

func applyProposalDiscard(ctx context.Context, s *evidenceSession, root *os.Root, out *proposalDiscardOutput, ops proposalDiscardOps) error {
	if err := checkProposalState(ctx, s, root, out); err != nil {
		return err
	}
	for _, file := range out.Files {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := ops.archive(ctx, root, out.Archive+"/"+file.Path, file.Bytes, 0600); err != nil {
			return err
		}
	}
	if err := ops.archive(ctx, root, out.Archive+"/intent.json", proposalIntent(out), 0600); err != nil {
		return err
	}
	if err := checkProposalArchive(ctx, root, out, true); err != nil {
		return err
	}
	var removed []proposalDiscardFile
	rollback := func(cause error) error {
		for _, file := range removed {
			if err := ops.restore(context.WithoutCancel(ctx), root, file); err != nil {
				cause = errors.Join(cause, fmt.Errorf("restore %s from %s: %w", file.Path, out.Archive, err))
			}
		}
		return cause
	}
	for _, file := range out.Files {
		if err := checkProposalState(ctx, s, root, out); err != nil {
			return rollback(err)
		}
		present, err := proposalFilePresent(ctx, root, file)
		if err != nil {
			return rollback(err)
		}
		if !present {
			continue // An identical interrupted attempt already removed this file.
		}
		removed = append(removed, file) // Restore even if Remove's outcome is uncertain.
		if err := ops.remove(ctx, root, file); err != nil {
			return rollback(err)
		}
	}
	if err := checkProposalState(ctx, s, root, out); err != nil {
		return rollback(err)
	}
	for _, file := range out.Files {
		present, err := proposalFilePresent(ctx, root, file)
		if err != nil {
			return rollback(err)
		}
		if present {
			return rollback(fmt.Errorf("proposal remains after withdrawal: %s", file.Path))
		}
	}
	if err := s.unchangedPins(ctx); err != nil {
		return rollback(err)
	}
	out.Applied = true
	return nil
}
