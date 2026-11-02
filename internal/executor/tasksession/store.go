package tasksession

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Store persists task-session records: append-only, latest record per key wins.
type Store interface {
	// Lookup returns the latest record for (repository, source, taskID), if any.
	Lookup(ctx context.Context, repository, source, taskID string) (Record, bool, error)
	// Put appends a record; a later Put for the same key supersedes it.
	Put(ctx context.Context, record Record) error
}

// FileStore is the JSONL Store under a file lock. Every Put appends one line;
// Lookup replays the file and keeps the last line per key, so a torn or
// malformed line is a named error rather than a silently adopted state.
type FileStore struct {
	path string
}

var (
	marshalRecord       = json.Marshal
	makeLedgerDirectory = os.MkdirAll
)

// NewFileStore binds a store to a JSONL path (created on first Put).
func NewFileStore(path string) (*FileStore, error) {
	if strings.TrimSpace(path) == "" {
		return nil, errors.New("tasksession: empty ledger path")
	}
	return &FileStore{path: path}, nil
}

// DefaultPath is the ledger's home for a repository checkout: the git common
// dir's pyxis state directory, shared by every worktree of that clone and
// never inside a task worktree.
func DefaultPath(ctx context.Context, repoDir string) (string, error) {
	if strings.TrimSpace(repoDir) == "" {
		return "", errors.New("tasksession: empty repository path")
	}
	out, err := exec.CommandContext(ctx, "git", "-C", repoDir, "rev-parse", "--git-common-dir").Output()
	if err != nil {
		return "", fmt.Errorf("tasksession: resolve git common directory: %w", err)
	}
	common := strings.TrimSpace(string(out))
	if common == "" {
		return "", errors.New("tasksession: git returned an empty common directory")
	}
	if !filepath.IsAbs(common) {
		common = filepath.Join(repoDir, common)
	}
	common, err = filepath.Abs(common)
	if err != nil {
		return "", fmt.Errorf("tasksession: resolve absolute common directory: %w", err)
	}
	return filepath.Join(filepath.Clean(common), "pyxis", "task-sessions.jsonl"), nil
}

// Path is the ledger file.
func (s *FileStore) Path() string { return s.path }

// Lookup replays the ledger and returns the latest record for the key.
func (s *FileStore) Lookup(ctx context.Context, repository, source, taskID string) (Record, bool, error) {
	if err := ctx.Err(); err != nil {
		return Record{}, false, err
	}
	lock, err := lockLedger(s.path + ".lock")
	if err != nil {
		return Record{}, false, err
	}
	defer func() { _ = lock.Close() }()
	records, err := s.readAll()
	if err != nil {
		return Record{}, false, err
	}
	rec, ok := records[Record{Repository: repository, Source: source, TaskID: taskID}.Key()]
	return rec, ok, nil
}

// Put validates and appends one record.
func (s *FileStore) Put(ctx context.Context, record Record) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := record.Validate(); err != nil {
		return err
	}
	line, err := marshalRecord(record)
	if err != nil {
		return fmt.Errorf("tasksession: marshal record: %w", err)
	}
	if bytes.ContainsRune(line, '\n') {
		return errors.New("tasksession: record encodes with a newline")
	}
	lock, err := lockLedger(s.path + ".lock")
	if err != nil {
		return err
	}
	defer func() { _ = lock.Close() }()
	if err := makeLedgerDirectory(filepath.Dir(s.path), 0o700); err != nil {
		return fmt.Errorf("tasksession: create ledger directory: %w", err)
	}
	f, err := os.OpenFile(s.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return fmt.Errorf("tasksession: open ledger: %w", err)
	}
	if _, err := f.Write(append(line, '\n')); err != nil {
		_ = f.Close()
		return fmt.Errorf("tasksession: append ledger: %w", err)
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		return fmt.Errorf("tasksession: sync ledger: %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("tasksession: close ledger: %w", err)
	}
	return nil
}

// readAll replays the ledger into latest-per-key records. Caller holds the lock.
func (s *FileStore) readAll() (map[string]Record, error) {
	out := map[string]Record{}
	f, err := os.Open(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return out, nil
	}
	if err != nil {
		return nil, fmt.Errorf("tasksession: open ledger: %w", err)
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64*1024), 4*1024*1024)
	lineNo := 0
	for sc.Scan() {
		lineNo++
		line := bytes.TrimSpace(sc.Bytes())
		if len(line) == 0 {
			continue
		}
		var rec Record
		if err := json.Unmarshal(line, &rec); err != nil {
			return nil, fmt.Errorf("tasksession: ledger line %d is malformed: %w", lineNo, err)
		}
		if err := rec.Validate(); err != nil {
			return nil, fmt.Errorf("tasksession: ledger line %d: %w", lineNo, err)
		}
		out[rec.Key()] = rec
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("tasksession: scan ledger: %w", err)
	}
	return out, nil
}
