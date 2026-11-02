package installer

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/payload"
)

// seedAuthorityContract creates only a new repository's initial policy. Even
// an ignored existing policy or symlink refuses; init must never replace an
// operator-owned grant. The created object participates in ordinary rollback.
func seedAuthorityContract(ctx context.Context, run *initRun) error {
	root, err := os.OpenRoot(run.opts.Target)
	if err != nil {
		return fmt.Errorf("seed: open authority root: %w", err)
	}
	defer func() { _ = root.Close() }() // no writes remain on this directory handle
	template, err := root.ReadFile(filepath.FromSlash(payload.AuthorityContractTemplate))
	if err != nil {
		return fmt.Errorf("seed: read authority template: %w", err)
	}
	body, err := payload.InitialAuthorityContract(template, now().UTC().Format("2006-01-02"))
	if err != nil {
		return fmt.Errorf("seed: authority contract: %w", err)
	}
	if err := authoritySeedPaths(ctx, root, run.j); err != nil {
		return err
	}
	// Root confines the open even if a parent changes after the Lstat probe;
	// O_EXCL separately protects every existing final object, including links.
	f, err := root.OpenFile(filepath.FromSlash(payload.AuthorityContractTarget), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("seed: preserve existing authority contract %s: %w", payload.AuthorityContractTarget, err)
	}
	info, statErr := f.Stat()
	entry := journalEntry{kind: journalRootedFile, path: filepath.Join(run.j.target, filepath.FromSlash(payload.AuthorityContractTarget)), ownedInfo: info, ownedDigest: sha256.Sum256(nil)}
	run.j.entries = append(run.j.entries, entry)
	if statErr != nil {
		return fmt.Errorf("seed: record authority identity: %w", errors.Join(statErr, f.Close()))
	}
	n, writeErr := f.Write(body)
	run.j.entries[len(run.j.entries)-1].ownedDigest = sha256.Sum256(body[:n])
	if err := errors.Join(writeErr, f.Close()); err != nil {
		return fmt.Errorf("seed: write authority contract: %w", err)
	}
	return nil
}

// probeAuthoritySeed is shared by preview and apply before any mutation. A
// nil journal makes the parent walk strictly read-only.
func probeAuthoritySeed(ctx context.Context, target string) error {
	root, err := os.OpenRoot(target)
	if err != nil {
		return fmt.Errorf("seed: open authority root: %w", err)
	}
	defer func() { _ = root.Close() }() // read-only probe completed
	return authoritySeedPaths(ctx, root, nil)
}

func authoritySeedPaths(ctx context.Context, root *os.Root, j *journal) error {
	parent := ""
	for component := range strings.SplitSeq(path.Dir(payload.AuthorityContractTarget), "/") {
		if err := ctx.Err(); err != nil {
			return err
		}
		parent = path.Join(parent, component)
		info, err := root.Lstat(filepath.FromSlash(parent))
		if errors.Is(err, fs.ErrNotExist) {
			if j == nil {
				return nil // no final object can exist below a missing parent
			}
			if err := root.Mkdir(filepath.FromSlash(parent), 0o755); err != nil {
				return fmt.Errorf("seed: create authority parent %s: %w", parent, err)
			}
			info, statErr := root.Lstat(filepath.FromSlash(parent))
			j.entries = append(j.entries, journalEntry{kind: journalRootedDir, path: filepath.Join(j.target, filepath.FromSlash(parent)), ownedInfo: info})
			if statErr != nil {
				return fmt.Errorf("seed: record authority parent identity %s: %w", parent, statErr)
			}
			continue
		}
		if err != nil {
			return fmt.Errorf("seed: probe authority parent %s: %w", parent, err)
		}
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("seed: authority parent %s must be a directory, never a symlink", parent)
		}
	}
	if _, err := root.Lstat(filepath.FromSlash(payload.AuthorityContractTarget)); !errors.Is(err, fs.ErrNotExist) {
		if err == nil {
			err = fs.ErrExist
		}
		return fmt.Errorf("seed: preserve existing authority contract %s: %w", payload.AuthorityContractTarget, err)
	}
	return nil
}
