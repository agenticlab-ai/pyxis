package localfs

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"syscall"
)

// writePhase names a cut point in the atomic-write sequence (stage temp → fsync
// → rename → dir-sync). It exists for the conformance kit's crash-injection
// suite: a simulated crash at any phase before the rename must leave the target
// wholly old, and a crash after it wholly new — never torn (DINV-2). Production
// callers pass no injector, so no fire ever runs on the live path.
type writePhase int

const (
	phaseBeforeTemp   writePhase = iota // before the sibling temp is created → old
	phaseAfterTemp                      // temp written, before its fsync → old
	phaseBeforeRename                   // temp fsync'd, before the rename (the danger point) → old
	phaseAfterRename                    // rename committed, before the dir fsync → new
)

// crashInjector, when non-nil, is consulted at each write phase; a non-nil
// return simulates a crash at that phase and aborts the remainder of the write.
// Live callers pass nil.
type crashInjector func(writePhase) error

// fire consults inj at phase p, tolerating a nil injector (the live path).
func fire(inj crashInjector, p writePhase) error {
	if inj == nil {
		return nil
	}
	return inj(p)
}

// The atomic write discipline (docs/tech-design/local-work-tracker.md §3.4,
// DINV-2): every whole-file write lands via a sibling temp file that is
// fsync'd and then renamed over the target. rename() is atomic on POSIX, so a
// reader observes either the whole prior file or the whole new one, never a
// torn one; a crash before the rename leaves the prior file intact and drops
// only an orphan temp, swept on the next run.

const tempPrefix = ".tmp-"

var tempSeq atomic.Uint64

// tempName is a per-call sibling temp path in dir — process id plus a
// monotonic counter, so two concurrent writers in one process never collide
// and no clock or randomness is consulted (hermetic; AGENTS.md test rules).
func (s *Store) tempName(dir string) string {
	n := tempSeq.Add(1)
	return filepath.Join(dir, fmt.Sprintf("%s%d-%d", tempPrefix, os.Getpid(), n))
}

// createTemp opens a fresh sibling temp with O_EXCL, retrying with the next
// counter value on the rare EEXIST — a leftover orphan temp whose name this run
// happens to reproduce (a crashed writer whose pid was later reused; the one
// case the pid-aware sweep cannot reap while that pid is alive). Retrying makes
// a write robust to stale temp state without depending on the sweep for
// correctness. The bound guards against a non-EEXIST error masquerading as one.
func (s *Store) createTemp(dir string) (string, *os.File, error) {
	for attempt := 0; attempt < 1024; attempt++ {
		tmp := s.tempName(dir)
		f, err := os.OpenFile(tmp, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
		if err == nil {
			return tmp, f, nil
		}
		if !os.IsExist(err) {
			return "", nil, err
		}
	}
	return "", nil, fmt.Errorf("could not open a fresh temp in %s after 1024 attempts", dir)
}

// writeSyncTemp writes data to a fresh sibling temp of target's directory and
// fsyncs it durable, returning the temp path. It does not touch target. inj, if
// non-nil, is fired at the after-temp and before-rename phases so the crash suite
// can abort a half-staged write; a fired abort removes the temp.
func (s *Store) writeSyncTemp(target string, data []byte, inj crashInjector) (string, error) {
	dir := filepath.Dir(target)
	tmp, f, err := s.createTemp(dir)
	if err != nil {
		return "", err
	}
	if _, err := f.Write(data); err != nil {
		f.Close()
		os.Remove(tmp)
		return "", err
	}
	if err := fire(inj, phaseAfterTemp); err != nil {
		f.Close()
		os.Remove(tmp)
		return "", err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		os.Remove(tmp)
		return "", err
	}
	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return "", err
	}
	if err := fire(inj, phaseBeforeRename); err != nil {
		os.Remove(tmp)
		return "", err
	}
	return tmp, nil
}

// atomicReplace writes data over target atomically: temp + fsync + rename. inj,
// when non-nil, is the conformance kit's crash seam — fired at each cut point of
// the sequence; a fired abort before the rename leaves target untouched (wholly
// old), one after it leaves target replaced (wholly new). Callers on the live
// path pass nil, so no cut point ever fires.
func (s *Store) atomicReplace(target string, data []byte, inj crashInjector) error {
	if err := fire(inj, phaseBeforeTemp); err != nil {
		return err // nothing staged; target is wholly old
	}
	tmp, err := s.writeSyncTemp(target, data, inj)
	if err != nil {
		return err
	}
	if err := os.Rename(tmp, target); err != nil {
		os.Remove(tmp)
		return err
	}
	if err := fire(inj, phaseAfterRename); err != nil {
		return err // target is already wholly new; only the dir fsync is skipped
	}
	return syncDir(filepath.Dir(target))
}

// syncDir fsyncs a directory so a create/rename within it is durable.
func syncDir(dir string) error {
	d, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer d.Close()
	// Some platforms reject fsync on a directory fd; that is not a failure of
	// the write itself, so it is tolerated (the rename already landed).
	_ = d.Sync()
	return nil
}

// reserveID claims the next free identifier by atomic exclusive-create
// (§3.3, DINV-3): starting at the highest-known id + 1 it opens
// items/<n>.md with O_CREATE|O_EXCL, incrementing on collision until the
// create wins. The winning exclusive create *is* the allocation — collision-
// free and never-reused under concurrent local processes, no counter or lock.
// The reserved file is an empty placeholder the caller fills via atomicReplace;
// a crash between reserve and fill leaves a zero-byte file that Get reports as
// absent and that stays a permanent, never-reused gap.
func (s *Store) reserveID(start int) (int, error) {
	dir := s.itemsDir()
	for n := start; ; n++ {
		f, err := os.OpenFile(itemPath(dir, n), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
		if err == nil {
			f.Close()
			return n, nil
		}
		if os.IsExist(err) {
			continue
		}
		return 0, err
	}
}

// maxID scans the items directory for the highest allocated identifier,
// returning 0 for an empty or absent store. Every well-formed and every
// reserved-but-crashed file counts, so the next id is always past the last
// allocation (gaps are permanent).
func (s *Store) maxID() (int, error) {
	entries, err := os.ReadDir(s.itemsDir())
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}
	max := 0
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if n, ok := idFromName(e.Name()); ok && n > max {
			max = n
		}
	}
	return max, nil
}

// sweepTemps removes orphan temp files left by a crash mid-write. Item files
// are never swept — a zero-byte reserved file is a permanent gap, not an
// orphan. A temp names the pid writing it (tempName); a temp whose pid is still
// alive belongs to a concurrent writer mid-rename, so it is left untouched —
// reaping it would make that writer's rename fail and lose the write (DINV-2,
// the parallel-create contract of UB-5). Only a dead-pid orphan is swept. This
// is the same local-POSIX process assumption the O_EXCL id allocation (§3.3)
// and the flock serialization (§3.4) already make. The sweep is pure
// housekeeping (reclaiming disk): createTemp's EEXIST retry, not this sweep, is
// what keeps a write correct against any leftover temp.
func (s *Store) sweepTemps() {
	entries, err := os.ReadDir(s.itemsDir())
	if err != nil {
		return
	}
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasPrefix(name, tempPrefix) {
			continue
		}
		if pid, ok := tempPID(name); ok && processAlive(pid) {
			continue
		}
		// Best-effort reclaim: a failure to remove an orphan is not fatal to any
		// write (createTemp retries past it), so the error is deliberately dropped.
		_ = os.Remove(filepath.Join(s.itemsDir(), name))
	}
}

// tempPID parses the writer pid from a temp name ".tmp-<pid>-<seq>".
func tempPID(name string) (int, bool) {
	rest := strings.TrimPrefix(name, tempPrefix)
	dash := strings.IndexByte(rest, '-')
	if dash <= 0 {
		return 0, false
	}
	pid, err := strconv.Atoi(rest[:dash])
	if err != nil || pid <= 0 {
		return 0, false
	}
	return pid, true
}

// processAlive reports whether pid names a live process. Signal 0 does the
// kernel's permission/existence check without delivering a signal: nil (alive,
// signalable) or EPERM (alive, not ours) means live; ESRCH means gone. Local
// POSIX only, matching the store's medium assumptions (§3.3, §3.4).
func processAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	err := syscall.Kill(pid, 0)
	return err == nil || err == syscall.EPERM
}

func itemPath(dir string, id int) string {
	return filepath.Join(dir, strconv.Itoa(id)+".md")
}

// idFromName parses "<n>.md" to n; ok is false for any other name (temps,
// dotfiles, non-numeric).
func idFromName(name string) (int, bool) {
	if !strings.HasSuffix(name, ".md") {
		return 0, false
	}
	n, err := strconv.Atoi(strings.TrimSuffix(name, ".md"))
	if err != nil || n <= 0 {
		return 0, false
	}
	return n, true
}
