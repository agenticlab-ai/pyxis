package runlog

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestOpenReusesFeatureLog(t *testing.T) {
	repo := t.TempDir()
	first, firstPath, err := Open(repo, ".pyxis/custom", "Widget Export/001", time.Date(2026, 9, 9, 1, 2, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if err := WriteLine(first, time.Date(2026, 9, 9, 1, 2, 0, 0, time.UTC), "run started"); err != nil {
		t.Fatal(err)
	}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}

	second, secondPath, err := Open(repo, ".pyxis/custom", "Widget Export/001", time.Date(2026, 9, 10, 1, 2, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if err := WriteLine(second, time.Date(2026, 9, 10, 1, 2, 0, 0, time.UTC), "run resumed"); err != nil {
		t.Fatal(err)
	}
	if err := second.Close(); err != nil {
		t.Fatal(err)
	}
	if firstPath != secondPath {
		t.Fatalf("second run path = %q, want reused %q", secondPath, firstPath)
	}
	want := filepath.Join(repo, ".pyxis/custom", "09-09-26-%57idget%20%45xport%2F001.log")
	if firstPath != want {
		t.Fatalf("path = %q, want %q", firstPath, want)
	}
	b, err := os.ReadFile(firstPath)
	if err != nil || !strings.Contains(string(b), "run started") || !strings.Contains(string(b), "run resumed") {
		t.Fatalf("log = %q, %v", b, err)
	}
}

func TestOpenReportsFilesystemErrors(t *testing.T) {
	for _, tc := range []struct{ name, want string }{
		{"directory is a file", "create implement-now log directory"},
		{"log is a directory", "open implement-now log"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := t.TempDir()
			now := time.Date(2026, 9, 9, 0, 0, 0, 0, time.UTC)
			if tc.name == "directory is a file" {
				if err := os.WriteFile(filepath.Join(repo, "logs"), nil, 0o644); err != nil {
					t.Fatal(err)
				}
			} else if err := os.MkdirAll(filepath.Join(repo, "logs", "09-09-26-demo.log"), 0o755); err != nil {
				t.Fatal(err)
			}
			f, path, err := Open(repo, "logs", "demo", now)
			var pathErr *os.PathError
			if f != nil || path != "" || err == nil || !strings.Contains(err.Error(), tc.want) || !errors.As(err, &pathErr) {
				t.Fatalf("Open = %v, %q, %v; want wrapped filesystem error containing %q", f, path, err, tc.want)
			}
		})
	}
}

func TestWriteLineTimestampAndErrors(t *testing.T) {
	now := time.Date(2026, 9, 9, 1, 2, 3, 0, time.FixedZone("west", -7*60*60))
	var out bytes.Buffer
	if err := WriteLine(&out, now, "finished %d tasks", 2); err != nil {
		t.Fatal(err)
	}
	if want := "2026-09-09T08:02:03Z | finished 2 tasks\n"; out.String() != want {
		t.Fatalf("line = %q, want %q", out.String(), want)
	}
	if err := WriteLine(nil, now, "no sink"); err != nil {
		t.Fatal(err)
	}
	if err := WriteLine(failedWriter{}, now, "failed write"); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatalf("write error = %v, want closed pipe", err)
	}
}

type failedWriter struct{}

func (failedWriter) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }
