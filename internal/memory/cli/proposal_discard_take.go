package cli

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"strconv"
)

// Move the actual pathname object into a private ignored recovery directory.
// Never unlink a name after comparing an earlier object: a replacement in that
// gap must remain recoverable. Taken objects are retained, even on success.
func removeProposalFile(ctx context.Context, root *os.Root, file proposalDiscardFile, archive string, ignored func(context.Context, []string) error) error {
	parent, err := proposalParent(ctx, root, file.Path, false)
	if err != nil {
		return err
	}
	closePolicyRoot(parent)
	taken, err := newProposalTakenPath(ctx, root, file, archive, ignored)
	if err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := root.Rename(file.Path, taken); err != nil {
		return err
	}
	actual := file
	actual.Path = taken
	present, err := proposalFilePresent(ctx, root, actual)
	if err == nil && present {
		return nil
	}
	// Restore the exact taken object, including a dirty replacement, without
	// overwriting another writer. A conflict leaves it at the reported path.
	if err == nil {
		err = fmt.Errorf("taken proposal disappeared")
	}
	return errors.Join(fmt.Errorf("withdrawal refused; actual object retained at %s: %w", taken, err), root.Link(taken, file.Path))
}

func newProposalTakenPath(ctx context.Context, root *os.Root, file proposalDiscardFile, archive string, ignored func(context.Context, []string) error) (string, error) {
	for i := range 100 {
		name := archive + "/taken-" + path.Base(file.Path) + "-" + strconv.Itoa(i)
		taken := name + "/payload"
		if err := ignored(ctx, []string{name + "/", taken}); err != nil {
			return "", err
		}
		parent, err := proposalParent(ctx, root, name, true)
		if err != nil {
			return "", err
		}
		err = parent.Mkdir(path.Base(name), 0700)
		closePolicyRoot(parent)
		if errors.Is(err, fs.ErrExist) {
			continue
		}
		return taken, err
	}
	return "", fmt.Errorf("too many retained withdrawal attempts for %s", file.Path)
}
