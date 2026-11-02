// Command coverage-gate runs all default Go tests with repository-wide
// statement instrumentation and fails when any measured Go source file falls
// below a declared minimum. It is a Go-stack check invoked by the repository's
// generic single-source code-gate.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/agenticlab-ai/pyxis/internal/coveragegate"
)

const (
	exitPass  = 0
	exitRed   = 1
	exitUsage = 2
)

type measurer func(context.Context, string, string, io.Writer) (coveragegate.Report, error)

var processExit = os.Exit

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	processExit(run(ctx, os.Args[1:], os.Stdout, os.Stderr, coveragegate.Measure))
}

func run(ctx context.Context, args []string, stdout, stderr io.Writer, measure measurer) int {
	fs := flag.NewFlagSet("coverage-gate", flag.ContinueOnError)
	fs.SetOutput(stderr)
	minimumText := fs.String("min", "", "required statement coverage percentage (0..100)")
	pattern := fs.String("packages", "./...", "Go package pattern to test and instrument")
	if err := fs.Parse(args); err != nil {
		return exitUsage
	}
	if fs.NArg() != 0 {
		fmt.Fprintf(stderr, "coverage-gate: unexpected arguments: %v\n", fs.Args())
		return exitUsage
	}
	minimum, err := coveragegate.ParseMinimum(*minimumText)
	if err != nil {
		fmt.Fprintf(stderr, "coverage-gate: %v\n", err)
		return exitUsage
	}
	dir, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(stderr, "coverage-gate: resolve working directory: %v\n", err)
		return exitUsage
	}
	report, err := measure(ctx, dir, *pattern, stderr)
	if err != nil {
		fmt.Fprintf(stderr, "coverage-gate: %v\n", err)
		return exitRed
	}

	fmt.Fprintf(stdout, "coverage-gate: %.2f%% (%d/%d statements), minimum %.2f%%\n",
		report.Percent(), report.Covered, report.Total, float64(minimum)/100)
	fmt.Fprintln(stdout, "coverage-gate: lowest-covered packages:")
	for _, row := range report.Bottom(5) {
		fmt.Fprintf(stdout, "  %6.2f%%  %d/%d  %s\n", row.Percent(), row.Covered, row.Total, row.Path)
	}
	fmt.Fprintln(stdout, "coverage-gate: lowest-covered files:")
	for _, row := range report.BottomFiles(5) {
		fmt.Fprintf(stdout, "  %6.2f%%  %d/%d  %s\n", row.Percent(), row.Covered, row.Total, row.Path)
	}
	below := report.Below(minimum)
	if len(below) != 0 {
		fmt.Fprintf(stderr, "coverage-gate: FAIL — %d of %d measured Go files are below %.2f%%:\n", len(below), len(report.Files), float64(minimum)/100)
		for _, row := range below {
			fmt.Fprintf(stderr, "  %6.2f%%  %d/%d  %s\n", row.Percent(), row.Covered, row.Total, row.Path)
		}
		return exitRed
	}
	fmt.Fprintf(stdout, "coverage-gate: PASS — every measured Go file meets %.2f%%\n", float64(minimum)/100)
	return exitPass
}
