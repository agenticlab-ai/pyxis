package runlogs

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/agenticlab-ai/pyxis/internal/reporoot"
)

// EventLogger retains a CLI's structured diagnostic stream without changing
// its stderr protocol. The caller closes the journal when the command returns.
func EventLogger(ctx context.Context, stderr io.Writer, name string) (*slog.Logger, func()) {
	root, err := reporoot.Find()
	if err != nil {
		root, err = os.Getwd()
	}
	if err == nil {
		var f *os.File
		f, err = Create(ctx, root, Reviews, name+"-*.jsonl")
		if err == nil {
			return slog.New(slog.NewJSONHandler(io.MultiWriter(stderr, f), nil)), func() {
				if err := f.Close(); err != nil {
					fmt.Fprintf(stderr, "close diagnostic log %s: %v\n", filepath.Base(f.Name()), err)
				}
			}
		}
	}
	fmt.Fprintf(stderr, "cannot retain execution diagnostics: %v\n", err)
	return slog.New(slog.NewJSONHandler(stderr, nil)), func() {}
}
