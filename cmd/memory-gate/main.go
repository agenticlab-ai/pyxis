// Command memory-gate composes the same memory predicates locally and in CI.
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/agenticlab-ai/pyxis/internal/memory/cli"
)

var (
	processExit = os.Exit
	processArgs = func() []string { return os.Args[1:] }
	processEnv  = cli.OSEnv
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	processExit(cli.RunGate(ctx, processArgs(), processEnv()))
}
