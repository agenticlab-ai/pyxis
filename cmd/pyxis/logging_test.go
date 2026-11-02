package main

import (
	"bytes"
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// TestMain isolates machine-local state and handles subprocess test boundaries.
// Execution logs use each verb test's temporary repository instead.
func TestMain(m *testing.M) {
	if os.Getenv("PYXIS_TEST_GH_HELPER") == "1" {
		_, _ = os.Stdout.WriteString(os.Getenv("PYXIS_TEST_GH_STDOUT"))
		_, _ = os.Stderr.WriteString(os.Getenv("PYXIS_TEST_GH_STDERR"))
		code, _ := strconv.Atoi(os.Getenv("PYXIS_TEST_GH_EXIT"))
		os.Exit(code)
	}
	dir, err := os.MkdirTemp("", "pyxis-test-state-*")
	if err == nil {
		os.Setenv("XDG_STATE_HOME", dir)
	}
	code := m.Run()
	if dir != "" {
		os.RemoveAll(dir)
	}
	os.Exit(code)
}

// The two-sink split (#898): INFO stays off the terminal and lands in the
// journal; WARN reaches both; --verbose restores INFO inline.
func TestRunLoggerTwoSinkSplit(t *testing.T) {
	var term bytes.Buffer
	logger, path := runLogger(t.TempDir(), &term, "init", false)
	if path == "" {
		t.Fatal("no journal path — the repository is writable, the journal must open")
	}
	logger.LogAttrs(context.Background(), slog.LevelInfo, "resolve_fetch", slog.String("artifact", "x"))
	logger.LogAttrs(context.Background(), slog.LevelWarn, "init_rolled_back")

	if strings.Contains(term.String(), "resolve_fetch") {
		t.Errorf("INFO leaked onto the terminal:\n%s", term.String())
	}
	if !strings.Contains(term.String(), "init_rolled_back") {
		t.Errorf("WARN missing from the terminal:\n%s", term.String())
	}
	j, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read journal: %v", err)
	}
	for _, want := range []string{"resolve_fetch", "init_rolled_back"} {
		if !strings.Contains(string(j), want) {
			t.Errorf("journal lacks %q:\n%s", want, j)
		}
	}
	if !strings.HasSuffix(path, ".jsonl") || !strings.Contains(filepath.Base(path), "init-") {
		t.Errorf("journal path %q does not follow <verb>-<stamp>.jsonl", path)
	}
}

func TestRunLoggerVerboseRestoresTheStream(t *testing.T) {
	var term bytes.Buffer
	logger, _ := runLogger(t.TempDir(), &term, "init", true)
	logger.LogAttrs(context.Background(), slog.LevelInfo, "resolve_fetch")
	if !strings.Contains(term.String(), "resolve_fetch") {
		t.Errorf("--verbose did not restore INFO on the terminal:\n%s", term.String())
	}
}

// A logging sink must never block the verb: an unopenable log directory
// degrades to terminal-only, path "".
func TestRunLoggerDegradesWithoutAJournal(t *testing.T) {
	file := filepath.Join(t.TempDir(), "not-a-dir")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	// MkdirAll under a regular file fails.
	var term bytes.Buffer
	logger, path := runLogger(file, &term, "init", false)
	if path != "" {
		t.Fatalf("path = %q, want \"\" when the journal cannot open", path)
	}
	logger.LogAttrs(context.Background(), slog.LevelWarn, "still_logs")
	if !strings.Contains(term.String(), "still_logs") {
		t.Error("terminal handler lost after journal degradation")
	}
}

func TestFanoutDerivedHandlersPreserveBothSinks(t *testing.T) {
	var term, journal bytes.Buffer
	h := fanout{slog.NewTextHandler(&term, nil), slog.NewJSONHandler(&journal, nil)}
	logger := slog.New(h.WithAttrs([]slog.Attr{slog.String("scope", "tests")}))
	logger.Info("with attrs")
	grouped := slog.New(h.WithGroup("nested"))
	grouped.Info("with group", slog.String("key", "value"))
	for name, output := range map[string]string{"term": term.String(), "journal": journal.String()} {
		for _, want := range []string{"with attrs", "tests", "with group", "value"} {
			if !strings.Contains(output, want) {
				t.Errorf("%s sink missing %q: %s", name, want, output)
			}
		}
	}
}
