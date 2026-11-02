package store

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/phasemeter/record"
)

func validRecord(runID string) record.Record {
	return record.Record{
		SchemaVersion: record.SchemaVersion,
		RunID:         runID,
		FeatureID:     "feature",
		PhaseID:       "phase",
		Harness:       "codex",
	}
}

func TestStoreFilesystemAndDecodeFailures(t *testing.T) {
	blocked := filepath.Join(t.TempDir(), "blocked")
	if err := os.WriteFile(blocked, []byte("file"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := New(filepath.Join(blocked, "store")).Append(validRecord("run")); err == nil {
		t.Fatal("blocked store directory: want error")
	}

	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "run.jsonl"), 0o755); err != nil {
		t.Fatal(err)
	}
	s := New(dir)
	if err := s.Append(validRecord("run")); err == nil {
		t.Fatal("directory at run-file path: want append error")
	}
	if _, err := s.LoadRun("run"); err == nil {
		t.Fatal("directory at run-file path: want scan error")
	}

	if err := os.WriteFile(filepath.Join(dir, "bad.jsonl"), []byte("\n{not json}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := s.LoadRun("bad"); err == nil || !strings.Contains(err.Error(), "line 2") {
		t.Fatalf("malformed JSON error = %v", err)
	}
	if err := s.Append(validRecord("")); err == nil {
		t.Fatal("empty run ID: want error")
	}
}

func TestAppendLockedRejectsClosedAndReadOnlyFiles(t *testing.T) {
	closed, err := os.CreateTemp(t.TempDir(), "closed")
	if err != nil {
		t.Fatal(err)
	}
	if err := closed.Close(); err != nil {
		t.Fatal(err)
	}
	if err := appendLocked(closed, []byte("line\n")); err == nil {
		t.Fatal("closed file: want lock error")
	}

	path := filepath.Join(t.TempDir(), "readonly")
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	readOnly, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer readOnly.Close()
	if err := appendLocked(readOnly, []byte("line\n")); err == nil {
		t.Fatal("read-only file: want write error")
	}
}
