package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/agenticlab-ai/pyxis/internal/featuredrive"
)

var processExit = os.Exit

func main() {
	if err := run(context.Background(), os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, err)
		processExit(1)
	}
}

func run(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	if len(args) != 0 {
		switch args[0] {
		case "dogfood":
			return runDogfood(ctx, args, stdout, stderr)
		case "register-local":
			return registerLocal(ctx, args[1:], stdout, stderr)
		}
	}
	return fmt.Errorf("usage: feature-drive dogfood --workspace <empty-directory> | register-local [--config-home <directory>]")
}

func runDogfood(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("feature-drive dogfood", flag.ContinueOnError)
	fs.SetOutput(stderr)
	workspace := fs.String("workspace", "", "empty directory for disposable local and remote Git journeys")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if fs.NArg() != 0 || *workspace == "" {
		return fmt.Errorf("usage: feature-drive dogfood --workspace <empty-directory>")
	}
	report, err := featuredrive.RunExecutableFeatureDriveConformance(ctx, *workspace)
	if err != nil {
		return err
	}
	b, err := report.JSON()
	if err != nil {
		return fmt.Errorf("encode feature-drive dogfood report: %w", err)
	}
	_, err = fmt.Fprintln(stdout, string(b))
	return err
}
