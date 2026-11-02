package localfs

import (
	"os"
	"path/filepath"
	"strconv"
	"syscall"
)

// Per-item write serialization (docs/tech-design/local-work-tracker.md §3.4,
// DINV-5). Concurrent writers to *one* item are serialized by a per-item
// advisory lock (flock) held only across that read-modify-write, so no append is
// lost and no concurrent write clobbers another. Writers to *different* items
// take distinct locks and never contend. Reads take no lock — the atomic rename
// (§3.4, DINV-2) already gives a reader the whole old or whole new file.
//
// The lock lives on a stable path that is never renamed (the item file itself is
// replaced by rename, so locking it would not serialize across the swap). Lock
// files sit under a dedicated <root>/locks/ directory, gitignored whole (they are
// ephemeral runtime state, never operand content — DINV-8), so the backlog's git
// history stays clean.

const lockDirName = "locks"

func (s *Store) lockDir() string { return filepath.Join(s.root, lockDirName) }

// withItemLock runs critical while holding the exclusive per-item advisory lock
// for id. flock across distinct open file descriptions serializes both
// concurrent goroutines (each its own open) and concurrent local processes
// (PRD UB-4). The lock is released whether critical succeeds or fails.
func (s *Store) withItemLock(id int, critical func() error) error {
	dir := s.lockDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	ensureLockGitignore(dir)
	path := filepath.Join(dir, strconv.Itoa(id)+".lock")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		return err
	}
	defer func() { _ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN) }()
	return critical()
}

// ensureLockGitignore writes a `*` .gitignore into the lock directory once, so
// the ephemeral lock files never enter the operand's git history. A failure to
// write it is not fatal to a mutation — the lock still works — so the error is
// deliberately ignored (the directory is transient runtime state).
func ensureLockGitignore(dir string) {
	path := filepath.Join(dir, ".gitignore")
	if _, err := os.Stat(path); err == nil {
		return
	}
	_ = os.WriteFile(path, []byte("*\n"), 0o644)
}
