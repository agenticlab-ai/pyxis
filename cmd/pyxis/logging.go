// logging.go — the verbs' two-sink log surface (issue #898): the terminal
// carries a curated narrative for the human running the verb, while the
// full slog stream lands as JSON lines in a per-run repository-local journal.
// --verbose restores the stream inline; the journal is written
// either way, so forensics never depend on remembering a flag.
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/runlogs"
	"github.com/agenticlab-ai/pyxis/internal/termstyle"
)

// journalDir resolves the operand's ignored execution-log directory.
func journalDir(root string) (string, error) {
	return runlogs.Resolve(context.Background(), root, runlogs.CLI, "")
}

// initRunLogger is init's first filesystem-write boundary. Tests interrupt at
// this boundary to verify signal handling is ready before journal setup starts.
var initRunLogger = runLogger

// runLogger builds a verb's logger: WARN+ to stderr for humans (INFO+ when
// verbose), INFO+ as JSON into the journal file. The returned path is ""
// when the journal could not open — a logging sink must never block the
// verb, so every failure here degrades to terminal-only. The journal file
// handle stays open for the process's remaining life by design (the verb
// exits when the run ends).
func runLogger(root string, stderr io.Writer, verb string, verbose bool) (*slog.Logger, string) {
	level := slog.LevelWarn
	if verbose {
		level = slog.LevelInfo
	}
	term := slog.NewTextHandler(stderr, &slog.HandlerOptions{Level: level})
	if root == "" {
		return slog.New(term), "" // previews retain their no-write contract
	}
	dir, err := journalDir(root)
	if err != nil {
		return slog.New(term), ""
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return slog.New(term), ""
	}
	// The pid keeps the file per-run: concurrent same-verb runs (a fleet
	// sweeping many repos under one user) must not interleave one journal.
	path := filepath.Join(dir, fmt.Sprintf("%s-%s-%d.jsonl", verb, time.Now().Format("20060102-150405"), os.Getpid()))
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return slog.New(term), ""
	}
	journal := slog.NewJSONHandler(f, &slog.HandlerOptions{Level: slog.LevelInfo})
	return slog.New(fanout{term, journal}), path
}

// fanout hands each record to both sinks at their own levels.
type fanout struct{ term, journal slog.Handler }

func (f fanout) Enabled(ctx context.Context, l slog.Level) bool {
	return f.term.Enabled(ctx, l) || f.journal.Enabled(ctx, l)
}

func (f fanout) Handle(ctx context.Context, r slog.Record) error {
	var errs []error
	if f.term.Enabled(ctx, r.Level) {
		errs = append(errs, f.term.Handle(ctx, r.Clone()))
	}
	if f.journal.Enabled(ctx, r.Level) {
		errs = append(errs, f.journal.Handle(ctx, r.Clone()))
	}
	return errors.Join(errs...)
}

func (f fanout) WithAttrs(attrs []slog.Attr) slog.Handler {
	return fanout{f.term.WithAttrs(attrs), f.journal.WithAttrs(attrs)}
}

func (f fanout) WithGroup(name string) slog.Handler {
	return fanout{f.term.WithGroup(name), f.journal.WithGroup(name)}
}

// printJournalPointer names the journal once, at the end of a verb's
// output — the one line that connects the narrative to the forensics.
// Dimmed on a TTY (#948): it should recede behind the epilogue, not
// compete with it.
func printJournalPointer(w io.Writer, path string) {
	if path != "" {
		terminalPrintf(w, "%s\n", termstyle.Detect(w).Dim("detail: "+path))
	}
}
