package main

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/codegatemirror"
)

// TestPrintReport_SurfacesCapturedOutput pins the CI-facing half of the
// legibility fix. A reader of a red run must learn WHAT broke from the log
// alone; before this, the report carried a single summary line per check and the
// failing test name never reached CI at all, so diagnosing required reproducing
// the run locally.
func TestPrintReport_SurfacesCapturedOutput(t *testing.T) {
	t.Parallel()
	rep := codegatemirror.Report{
		Passed: false,
		Outcomes: []codegatemirror.Outcome{
			{Name: "gofmt", Class: codegatemirror.ClassRequired, Passed: true, Duration: time.Second},
			{
				Name:     "test-race",
				Class:    codegatemirror.ClassRequired,
				Passed:   false,
				Duration: 2 * time.Minute,
				Detail:   "exited 1",
				Output:   "ok  \texample.com/pkg/a\t(cached)\n--- FAIL: TestSomething\nFAIL\texample.com/pkg/c",
			},
		},
	}

	var stderr bytes.Buffer
	if got := printReport(rep, &stderr); got != exitRed {
		t.Fatalf("printReport = %d, want %d", got, exitRed)
	}
	out := stderr.String()

	for _, want := range []string{
		"FAIL",                           // the summary row survives
		"--- test-race: captured output", // the block is labelled with its check
		"--- FAIL: TestSomething",        // the actual failure reaches the log
		"FAIL\texample.com/pkg/c",        // …and so does the failing package
	} {
		if !strings.Contains(out, want) {
			t.Errorf("report is missing %q:\n%s", want, out)
		}
	}
}

// TestPrintReport_QuietWhenGreen keeps a passing run's report a one-liner per
// check — the dump is failure evidence, not a log.
func TestPrintReport_QuietWhenGreen(t *testing.T) {
	t.Parallel()
	rep := codegatemirror.Report{
		Passed:   true,
		Outcomes: []codegatemirror.Outcome{{Name: "gofmt", Class: codegatemirror.ClassRequired, Passed: true}},
	}

	var stderr bytes.Buffer
	if got := printReport(rep, &stderr); got != exitPass {
		t.Fatalf("printReport = %d, want %d", got, exitPass)
	}
	if out := stderr.String(); strings.Contains(out, "captured output") {
		t.Errorf("green run printed a captured-output block:\n%s", out)
	}
}
