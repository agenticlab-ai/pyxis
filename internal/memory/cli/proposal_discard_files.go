package cli

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"strconv"
	"strings"
)

// Pin every directory descriptor and refuse symlink components. os.Root
// additionally confines reads/writes if a parent is renamed during the call.
func proposalParent(ctx context.Context, root *os.Root, name string, create bool) (*os.Root, error) {
	parent := root
	for part := range strings.SplitSeq(path.Dir(name), "/") {
		if err := ctx.Err(); err != nil {
			if parent != root {
				closePolicyRoot(parent)
			}
			return nil, err
		}
		info, err := parent.Lstat(part)
		if errors.Is(err, fs.ErrNotExist) && create {
			err = parent.Mkdir(part, 0700)
			if err == nil || errors.Is(err, fs.ErrExist) {
				info, err = parent.Lstat(part)
			}
		}
		if err == nil && !info.IsDir() {
			err = fmt.Errorf("proposal directory %s is not a regular directory", part)
		}
		if err != nil {
			if parent != root {
				err = errors.Join(err, parent.Close())
			}
			return nil, err
		}
		next, openErr := parent.OpenRoot(part)
		if parent != root {
			openErr = errors.Join(openErr, parent.Close())
		}
		if openErr != nil {
			if next != nil {
				openErr = errors.Join(openErr, next.Close())
			}
			return nil, openErr
		}
		parent = next
	}
	return parent, nil
}

func proposalFilePresent(ctx context.Context, root *os.Root, file proposalDiscardFile) (bool, error) {
	parent, err := proposalParent(ctx, root, file.Path, false)
	if err != nil {
		return false, err
	}
	defer closePolicyRoot(parent)
	info, err := parent.Lstat(path.Base(file.Path))
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if !info.Mode().IsRegular() {
		return false, fmt.Errorf("proposal %s is not a regular file", file.Path)
	}
	mode := "100644"
	if info.Mode().Perm()&0111 != 0 {
		mode = "100755"
	}
	body, err := parent.ReadFile(path.Base(file.Path))
	if err != nil {
		return false, err
	}
	if mode != file.Mode || !bytes.Equal(body, file.Bytes) {
		return false, fmt.Errorf("proposal %s differs from committed HEAD", file.Path)
	}
	return true, nil
}

// Existing destinations are only read. Exclusive creation prevents an
// archive or rollback from overwriting a concurrent replacement.
func saveProposalFile(ctx context.Context, root *os.Root, name string, body []byte, mode fs.FileMode) error {
	parent, err := proposalParent(ctx, root, name, true)
	if err != nil {
		return err
	}
	defer closePolicyRoot(parent)
	base := path.Base(name)
	if _, err := parent.Lstat(base); !errors.Is(err, fs.ErrNotExist) {
		return matchingProposalDestination(ctx, parent, base, body, mode)
	}
	// Incomplete temporary files left by process interruption never become
	// archive or restored record bytes. Try exclusive names without randomness
	// or overwriting an earlier attempt's diagnostics.
	f, temp, err := newProposalTemp(ctx, parent, base, mode)
	if err != nil {
		return err
	}
	_, writeErr := f.Write(body)
	if writeErr == nil {
		writeErr = f.Sync()
	}
	writeErr = errors.Join(writeErr, f.Close())
	if writeErr == nil {
		writeErr = parent.Link(temp, base)
		if errors.Is(writeErr, fs.ErrExist) {
			writeErr = matchingProposalDestination(ctx, parent, base, body, mode)
		}
	}
	return errors.Join(writeErr, parent.Remove(temp))
}

func newProposalTemp(ctx context.Context, parent *os.Root, base string, mode fs.FileMode) (*os.File, string, error) {
	for i := range 100 {
		if err := ctx.Err(); err != nil {
			return nil, "", err
		}
		name := "." + base + ".pending-" + strconv.Itoa(i)
		f, err := parent.OpenFile(name, os.O_CREATE|os.O_EXCL|os.O_WRONLY, mode)
		if errors.Is(err, fs.ErrExist) {
			continue
		}
		return f, name, err
	}
	return nil, "", fmt.Errorf("too many incomplete recovery copies for %s", base)
}

func matchingProposalDestination(ctx context.Context, parent *os.Root, name string, body []byte, mode fs.FileMode) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	info, err := parent.Lstat(name)
	if err != nil || !info.Mode().IsRegular() {
		return fmt.Errorf("recovery destination is not a regular file: %s", name)
	}
	old, err := parent.ReadFile(name)
	if err != nil || !bytes.Equal(old, body) || info.Mode().Perm()&0111 != mode.Perm()&0111 {
		return fmt.Errorf("recovery destination differs: %s", name)
	}
	return nil
}

func restoreProposalFile(ctx context.Context, root *os.Root, file proposalDiscardFile, archive string) error {
	mode := fs.FileMode(0600)
	if file.Mode == "100755" {
		mode = 0700
	}
	// Keep partial restoration files outside memory, even if the process stops
	// during rollback. The original archive stays a separate, unchanged copy.
	copyPath := archive + "/restore/" + file.Path
	if err := saveProposalFile(ctx, root, copyPath, file.Bytes, mode); err != nil {
		return err
	}
	parent, err := proposalParent(ctx, root, file.Path, true)
	if err != nil {
		return err
	}
	closePolicyRoot(parent)
	err = root.Link(copyPath, file.Path)
	if errors.Is(err, fs.ErrExist) {
		var present bool
		present, err = proposalFilePresent(ctx, root, file)
		if err == nil && !present {
			err = fmt.Errorf("restoration destination disappeared: %s", file.Path)
		}
	}
	return errors.Join(err, root.Remove(copyPath))
}
