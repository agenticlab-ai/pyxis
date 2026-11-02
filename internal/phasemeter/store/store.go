// Package store is the pipeline-measurement run-record store: an
// append-only, one-file-per-run JSONL store rooted at a directory —
// normally the observability-logs directory of the system footprint
// (docs/tech-design/pipeline-measurement-v2.md §3.6, D-1/ALT-1). Each run
// owns its own file, named by run_id, so two runs never contend on a
// shared file (AP-23); the store is local-substitutable — callers in
// tests root it at a temp dir (design §3.13).
package store

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"syscall"

	"github.com/agenticlab-ai/pyxis/internal/phasemeter/record"
	"github.com/agenticlab-ai/pyxis/internal/runlogs"
)

// DefaultRelDir is the store's default location relative to the repo
// root (internal/reporoot): the observability-logs directory of the
// system footprint (design §3.6, D-1). Gitignored — per-run operational
// data, never committed.
const DefaultRelDir = runlogs.Phasemeter

// Store is an append-only, one-file-per-run JSONL run-record store rooted
// at dir.
type Store struct {
	dir string
}

// New returns a Store rooted at dir. dir need not yet exist — Append
// creates it, and the run's file, on first use.
func New(dir string) *Store {
	return &Store{dir: dir}
}

// Append writes rec as one JSON line to its run's file
// (<dir>/<run_id>.jsonl), creating the store directory and the file as
// needed. Append refuses (no file created or modified) a record missing
// an identity field (Record.Validate) or carrying a run_id that is not a
// plain filename.
//
// The write is a single, whole-line write to a file opened O_APPEND,
// under that file's exclusive advisory lock: two different runs never
// contend since each owns a distinct file, and two callers appending to
// the *same* run — goroutines in this process or another local process —
// never tear or interleave each other's line (design §3.6, AP-23).
func (s *Store) Append(rec record.Record) error {
	if err := rec.Validate(); err != nil {
		return fmt.Errorf("phasemeter/store: append: %w", err)
	}
	if err := validRunID(rec.RunID); err != nil {
		return fmt.Errorf("phasemeter/store: append: run_id %q: %w", rec.RunID, err)
	}
	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return fmt.Errorf("phasemeter/store: append: create store dir: %w", err)
	}
	line, err := json.Marshal(rec)
	if err != nil {
		return fmt.Errorf("phasemeter/store: append: marshal record: %w", err)
	}
	line = append(line, '\n')

	f, err := os.OpenFile(s.runPath(rec.RunID), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("phasemeter/store: append: open run file: %w", err)
	}
	if err := appendLocked(f, line); err != nil {
		_ = f.Close() // best-effort: appendLocked's error is what we report
		return fmt.Errorf("phasemeter/store: append: %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("phasemeter/store: append: close run file: %w", err)
	}
	return nil
}

// appendLocked writes line to f while holding f's exclusive advisory
// lock, so a concurrent writer to the same run file never interleaves
// with this write.
func appendLocked(f *os.File, line []byte) error {
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		return fmt.Errorf("lock run file: %w", err)
	}
	// Released on Close too; unlocking explicitly just shortens the
	// critical section. A failure here is not actionable — Close follows
	// immediately either way.
	defer func() { _ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN) }()

	if _, err := f.Write(line); err != nil {
		return fmt.Errorf("write record: %w", err)
	}
	return nil
}

// LoadRun reads back every record appended to runID's file, in append
// order. A run with no file yet returns a nil slice, not an error — there
// is nothing to load, not a fault.
//
// Reads take no lock: Append always writes one line as a single Write
// call, so a concurrent reader never observes a torn line — either it
// sees a line or it doesn't, never half of one.
func (s *Store) LoadRun(runID string) ([]record.Record, error) {
	if err := validRunID(runID); err != nil {
		return nil, fmt.Errorf("phasemeter/store: load run %q: %w", runID, err)
	}
	f, err := os.Open(s.runPath(runID))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("phasemeter/store: load run %q: %w", runID, err)
	}
	defer func() { _ = f.Close() }() // read-only handle

	var records []record.Record
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := scanner.Bytes()
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		var rec record.Record
		if err := json.Unmarshal(line, &rec); err != nil {
			return nil, fmt.Errorf("phasemeter/store: load run %q: line %d: %w", runID, lineNo, err)
		}
		records = append(records, rec)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("phasemeter/store: load run %q: %w", runID, err)
	}
	return records, nil
}

func (s *Store) runPath(runID string) string {
	return filepath.Join(s.dir, runID+".jsonl")
}

// validRunID rejects a run id that would not stay within the store
// directory as a single filename — e.g. one containing a path separator
// or ".." — so the store's one-file-per-run contract cannot be silently
// broken by a malformed id.
func validRunID(id string) error {
	if id == "" {
		return fmt.Errorf("must not be empty")
	}
	if id == "." || id == ".." || id != filepath.Base(id) {
		return fmt.Errorf("must be a plain name, not a path")
	}
	return nil
}
