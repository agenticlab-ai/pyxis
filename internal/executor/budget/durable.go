package budget

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/statedir"
)

// The durability protocol, per design:state-reader-v4#DINV-12.
//
// Crash-consistency here is against HOST AND POWER LOSS, not merely process
// termination, so ordering call sites is not enough: an unsynced rename can
// reach disk after a later one. Every record is therefore replaced by
// write-temp → fsync temp → rename → fsync the containing directory, and the
// next record of an ordered pair does not begin until the previous one's
// directory sync has returned.
//
// Locks are SIDECARS and are never replaced. flock binds to an inode, and
// this protocol renames a new inode over the record — so a process locking
// the record file itself would hold the OLD inode while a second process
// locking the record after the rename holds the new one and proceeds
// concurrently. Mutual exclusion would silently stop existing at exactly the
// moment a write commits.
const (
	lockSuffix = ".lock"
	tempPrefix = ".budget-"
)

// durableFile and durableOps are the narrow syscall seam A-16/A-17 require:
// production binds os, while tests can inject a fault at one exact durability
// step and observe the complete ordering without pretending process-kill proves
// power-loss order.
type durableFile interface {
	io.Writer
	Name() string
	Chmod(os.FileMode) error
	Sync() error
	Close() error
}

type durableOps struct {
	createTemp func(dir, pattern string) (durableFile, error)
	rename     func(oldpath, newpath string) error
	remove     func(path string) error
	openDir    func(path string) (durableFile, error)
}

var osDurableOps = durableOps{
	createTemp: func(dir, pattern string) (durableFile, error) { return os.CreateTemp(dir, pattern) },
	rename:     os.Rename,
	remove:     os.Remove,
	openDir:    func(path string) (durableFile, error) { return os.Open(path) },
}

// writeDurable atomically replaces path with v's JSON encoding, and does not
// return until the replacement is durable through a power loss.
func writeDurable(path string, v any) error {
	return writeDurableWith(osDurableOps, path, v)
}

func writeDurableWith(ops durableOps, path string, v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("budget: encode %s: %w", filepath.Base(path), err)
	}
	dir := filepath.Dir(path)
	tmp, err := ops.createTemp(dir, tempPrefix+"*")
	if err != nil {
		return fmt.Errorf("budget: create temp for %s: %w", filepath.Base(path), err)
	}
	name := tmp.Name()
	// Removed on every failure path; on success the rename consumes it.
	defer ops.remove(name)

	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return fmt.Errorf("budget: chmod temp: %w", err)
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("budget: write temp: %w", err)
	}
	// fsync BEFORE the rename: a rename is only meaningful once the bytes it
	// publishes are on disk.
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return fmt.Errorf("budget: sync temp: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("budget: close temp: %w", err)
	}
	if err := ops.rename(name, path); err != nil {
		return fmt.Errorf("budget: replace %s: %w", filepath.Base(path), err)
	}
	// The rename itself is metadata; without this the replacement can be lost
	// to a power cut even though write() and rename() both returned.
	return fsyncDirWith(ops, dir)
}

// removeDurable unlinks path and does not return until the unlink is durable.
// Used for the cutover marker's invalidation, where a deletion that does not
// survive a power loss would silently exempt a re-upgrade from its barrier.
func removeDurable(path string) error {
	return removeDurableWith(osDurableOps, path)
}

func removeDurableWith(ops durableOps, path string) error {
	if err := ops.remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("budget: remove %s: %w", filepath.Base(path), err)
	}
	return fsyncDirWith(ops, filepath.Dir(path))
}

// fsyncDir flushes a directory's own metadata, which is what makes a rename
// or unlink within it durable.
func fsyncDir(dir string) error {
	return fsyncDirWith(osDurableOps, dir)
}

func fsyncDirWith(ops durableOps, dir string) error {
	d, err := ops.openDir(dir)
	if err != nil {
		return fmt.Errorf("budget: open dir for sync: %w", err)
	}
	defer d.Close()
	if err := d.Sync(); err != nil {
		return fmt.Errorf("budget: sync dir: %w", err)
	}
	return nil
}

// reapTemps removes scratch files a crashed writer left beside path. Every
// acquisition is accounted (AP-12): a resource type that can be stranded
// ships with its reaper, or husks accumulate.
//
// It runs only under the caller's lock on this record, and only removes
// temps older than the longest window — young ones may belong to a writer
// that is mid-commit right now.

func reapTemps(path string, now time.Time) error {
	return reapTempsWith(path, now, os.ReadDir, os.Remove, fsyncDir)
}

func reapTempsWith(
	path string,
	now time.Time,
	readDir func(string) ([]os.DirEntry, error),
	remove func(string) error,
	syncDir func(string) error,
) error {
	dir := filepath.Dir(path)
	entries, err := readDir(dir)
	if err != nil {
		return fmt.Errorf("budget: inspect scratch beside %s: %w", filepath.Base(path), err)
	}
	cutoff := now.Add(-longestWindow)
	removed := false
	for _, e := range entries {
		if e.IsDir() || !strings.HasPrefix(e.Name(), tempPrefix) {
			continue
		}
		info, err := e.Info()
		if err != nil {
			return fmt.Errorf("budget: inspect scratch %s: %w", e.Name(), err)
		}
		if info.ModTime().After(cutoff) {
			continue
		}
		if err := remove(filepath.Join(dir, e.Name())); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("budget: reap scratch %s: %w", e.Name(), err)
		}
		removed = true
	}
	if removed {
		if err := syncDir(dir); err != nil {
			return fmt.Errorf("budget: persist scratch reaping: %w", err)
		}
	}
	return nil
}

// lockFile opens (creating if needed) the stable sidecar lock beside a record.
func lockFile(record string) (*os.File, error) {
	if err := ensureRecordDir(record); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(record+lockSuffix, os.O_CREATE|os.O_RDWR|syscall.O_NOFOLLOW, 0o600)
	if err != nil {
		return nil, fmt.Errorf("budget: open lock: %w", err)
	}
	if err := validatePrivateFile(f); err != nil {
		f.Close()
		return nil, fmt.Errorf("budget: unsafe lock %s: %w", filepath.Base(f.Name()), err)
	}
	return f, nil
}

// ensureRecordDir establishes and verifies the marker's only anti-forgery
// boundary. The shared pyxis root may also contain non-secret journals, so it
// need only reject foreign ownership and group/other write access; the budget
// subtree and host directory are owner-only in full, as §3.5/§3.8 bind.
func ensureRecordDir(record string) error {
	dir := filepath.Dir(record)
	stateRoot, err := statedir.Root()
	if err != nil {
		return err
	}
	budgetRoot := filepath.Join(stateRoot, "content-budget")
	rel, err := filepath.Rel(budgetRoot, dir)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fmt.Errorf("budget: record directory %q escapes %q", dir, budgetRoot)
	}
	stateHome := filepath.Dir(stateRoot)
	if err := os.MkdirAll(stateHome, 0o700); err != nil {
		return fmt.Errorf("budget: create state home: %w", err)
	}
	if err := validateOwnedDir(stateHome, false); err != nil {
		return fmt.Errorf("budget: unsafe state directory %s: %w", stateHome, err)
	}
	if err := ensureOwnedDir(stateRoot, false); err != nil {
		return fmt.Errorf("budget: unsafe state directory %s: %w", stateRoot, err)
	}
	current := budgetRoot
	if err := ensureOwnedDir(current, true); err != nil {
		return fmt.Errorf("budget: unsafe state directory %s: %w", current, err)
	}
	if rel != "." {
		for _, elem := range strings.Split(rel, string(filepath.Separator)) {
			current = filepath.Join(current, elem)
			if err := ensureOwnedDir(current, true); err != nil {
				return fmt.Errorf("budget: unsafe state directory %s: %w", current, err)
			}
		}
	}
	return nil
}

func ensureOwnedDir(path string, ownerOnly bool) error {
	if _, err := os.Lstat(path); os.IsNotExist(err) {
		if err := os.Mkdir(path, 0o700); err != nil && !os.IsExist(err) {
			return err
		}
	} else if err != nil {
		return err
	}
	return validateOwnedDir(path, ownerOnly)
}

func validateOwnedDir(path string, ownerOnly bool) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return fmt.Errorf("not a real directory")
	}
	if err := validateOwner(info); err != nil {
		return err
	}
	forbidden := os.FileMode(0o022)
	if ownerOnly {
		forbidden = 0o077
	}
	if info.Mode().Perm()&forbidden != 0 {
		return fmt.Errorf("mode %04o permits untrusted access", info.Mode().Perm())
	}
	return nil
}

func validateOwner(info os.FileInfo) error {
	return validateOwnerWith(info, os.Geteuid)
}

func validateOwnerWith(info os.FileInfo, effectiveUID func() int) error {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return fmt.Errorf("cannot determine owner")
	}
	uid := effectiveUID()
	if stat.Uid != uint32(uid) {
		return fmt.Errorf("owned by uid %d, running as uid %d", stat.Uid, uid)
	}
	return nil
}

func validatePrivateFile(f *os.File) error {
	return validatePrivateFileWith(f, os.Geteuid)
}

func validatePrivateFileWith(f *os.File, effectiveUID func() int) error {
	info, err := f.Stat()
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("not a regular file")
	}
	if err := validateOwnerWith(info, effectiveUID); err != nil {
		return err
	}
	if info.Mode().Perm()&0o077 != 0 {
		return fmt.Errorf("mode %04o is not owner-only", info.Mode().Perm())
	}
	return nil
}

func readPrivateFile(path string) ([]byte, error) {
	return readPrivateFileWith(path, os.Geteuid)
}

func readPrivateFileWith(path string, effectiveUID func() int) ([]byte, error) {
	f, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	if err := validatePrivateFileWith(f, effectiveUID); err != nil {
		return nil, err
	}
	return io.ReadAll(f)
}

func decodeClosedJSON(data []byte, out any) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(out); err != nil {
		return err
	}
	if err := dec.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		if err == nil {
			return fmt.Errorf("multiple JSON values")
		}
		return err
	}
	return nil
}

func unlock(f *os.File) { _ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN) }
