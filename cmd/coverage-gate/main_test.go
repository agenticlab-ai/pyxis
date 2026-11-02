package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/coveragegate"
)

func TestMainDelegatesUsageExit(t *testing.T) {
	oldArgs, oldExit := os.Args, processExit
	defer func() {
		os.Args = oldArgs
		processExit = oldExit
	}()
	os.Args = []string{"coverage-gate"}
	got := -1
	processExit = func(code int) { got = code }
	main()
	if got != exitUsage {
		t.Fatalf("main exit = %d, want %d", got, exitUsage)
	}
}

func TestRunPassesAtExactMinimumAndPrintsBottomPackages(t *testing.T) {
	t.Parallel()
	measure := func(context.Context, string, string, io.Writer) (coveragegate.Report, error) {
		return coveragegate.Report{
			Covered: 26,
			Total:   30,
			Packages: []coveragegate.Package{
				{Path: "example.com/high", Covered: 9, Total: 10},
				{Path: "example.com/low", Covered: 8, Total: 10},
			},
			Files: []coveragegate.File{
				{Path: "example.com/high/high.go", Package: "example.com/high", Covered: 9, Total: 10},
				{Path: "example.com/low/low.go", Package: "example.com/low", Covered: 17, Total: 20},
			},
		}, nil
	}
	var stdout, stderr bytes.Buffer
	if code := run(context.Background(), []string{"-min", "85"}, &stdout, &stderr, measure); code != exitPass {
		t.Fatalf("run exit = %d, stderr = %q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "86.67% (26/30 statements)") || !strings.Contains(stdout.String(), "lowest-covered files") || !strings.Contains(stdout.String(), "example.com/low") || !strings.Contains(stdout.String(), "every measured Go file") {
		t.Fatalf("stdout = %q", stdout.String())
	}
}

func TestRunFailsWhenOneFileIsBelowMinimum(t *testing.T) {
	t.Parallel()
	measure := func(context.Context, string, string, io.Writer) (coveragegate.Report, error) {
		return coveragegate.Report{
			Covered: 9499,
			Total:   10000,
			Files: []coveragegate.File{
				{Path: "example.com/high.go", Covered: 9415, Total: 9900},
				{Path: "example.com/low.go", Covered: 84, Total: 100},
			},
		}, nil
	}
	var stdout, stderr bytes.Buffer
	if code := run(context.Background(), []string{"-min=85"}, &stdout, &stderr, measure); code != exitRed {
		t.Fatalf("run exit = %d, want %d", code, exitRed)
	}
	if !strings.Contains(stderr.String(), "1 of 2 measured Go files") || !strings.Contains(stderr.String(), "example.com/low.go") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestRunRejectsUsageAndMeasurementFailures(t *testing.T) {
	t.Parallel()
	never := func(context.Context, string, string, io.Writer) (coveragegate.Report, error) {
		t.Fatal("measurer called for invalid usage")
		return coveragegate.Report{}, nil
	}
	for _, args := range [][]string{{}, {"-min", "101"}, {"-min", "85", "extra"}} {
		var stdout, stderr bytes.Buffer
		if code := run(context.Background(), args, &stdout, &stderr, never); code != exitUsage {
			t.Fatalf("run(%v) exit = %d, want %d", args, code, exitUsage)
		}
	}

	failing := func(context.Context, string, string, io.Writer) (coveragegate.Report, error) {
		return coveragegate.Report{}, errors.New("tests failed")
	}
	var stdout, stderr bytes.Buffer
	if code := run(context.Background(), []string{"-min", "85"}, &stdout, &stderr, failing); code != exitRed {
		t.Fatalf("measurement failure exit = %d, want %d", code, exitRed)
	}
	if !strings.Contains(stderr.String(), "tests failed") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}
