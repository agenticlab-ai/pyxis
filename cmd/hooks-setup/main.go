// Command hooks-setup installs the same composed Git hooks used by the executor.
package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"

	"github.com/agenticlab-ai/pyxis/internal/githooks"
)

var processExit = os.Exit

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	processExit(run(ctx, os.Args[1:], os.Stderr))
}

func run(ctx context.Context, args []string, stderr io.Writer) int {
	if len(args) != 0 {
		// Usage and diagnostics are best effort; the exit status carries failure.
		_, _ = fmt.Fprintln(stderr, "usage: go run ./cmd/hooks-setup (from the repository)")
		return 2
	}
	if err := githooks.Install(ctx, "."); err != nil {
		// A broken output stream cannot report its own failure.
		_, _ = fmt.Fprintf(stderr, "hooks-setup: %v\n", err)
		return 1
	}
	return 0
}
