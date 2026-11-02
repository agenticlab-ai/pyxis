// memory — the repository's software memory: typed, immutable records under
// memory/<subject>/, one JSON file each, with every view rendered from them.
// Thin main: verb dispatch lives in internal/memory/cli.
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/agenticlab-ai/pyxis/internal/memory/cli"
)

// The process seams, replaceable by the main's own test.
var (
	processExit = os.Exit
	processArgs = func() []string { return os.Args[1:] }
	processEnv  = cli.OSEnv
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	processExit(cli.Run(ctx, processArgs(), processEnv()))
}
