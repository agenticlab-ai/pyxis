package codegatemirror

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/verify/runner"
)

// goTestShapedStdout reproduces the stream that made a red gate unreadable: a
// multi-package `go test ./...` run whose FIRST line is a PASSING cached package
// and whose failure only appears at the end. Every assertion below is written
// against this shape because it is the one the real defect had.
const goTestShapedStdout = `ok  	example.com/pkg/a	(cached)
ok  	example.com/pkg/b	0.104s
--- FAIL: TestResolverDelegateShimSuite (14.69s)
    delegate_shim_test.go:36: the resolver-delegate shim suite failed
FAIL	example.com/pkg/c	58.025s
FAIL`

// TestExitDetailNeverReadsStdout pins the regression directly. Reading stdout's
// first line reported `ok ... (cached)` — a package that PASSED — as the reason a
// check went red, and that string reached both the CI log and the self-heal
// fixer's prompt. Stderr is the diagnostic stream; stdout is not.
func TestExitDetailNeverReadsStdout(t *testing.T) {
	t.Parallel()
	got := exitDetail(runner.Result{ExitCode: 1, Stdout: goTestShapedStdout})

	if strings.Contains(got, "ok  ") || strings.Contains(got, "cached") {
		t.Fatalf("detail names a PASSING package as the failure reason: %q", got)
	}
	if !strings.Contains(got, "exited 1") {
		t.Fatalf("detail = %q, want it to fall back to the exit code", got)
	}
}

// TestExitDetailPrefersStderr keeps the useful half: a tool that diagnoses on
// stderr (go vet, go build) still gets its message as the one-line summary.
func TestExitDetailPrefersStderr(t *testing.T) {
	t.Parallel()
	tests := map[string]struct {
		res  runner.Result
		want string
	}{
		"stderr wins":             {runner.Result{ExitCode: 2, Stderr: "vet: bad format verb\n", Stdout: "noise"}, "vet: bad format verb"},
		"blank stderr falls back": {runner.Result{ExitCode: 3, Stderr: "\n  \n"}, "exited 3"},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if got := exitDetail(tt.res); got != tt.want {
				t.Fatalf("exitDetail = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestTailLinesTakesTheTail proves the direction. A tool announces what broke as
// it finishes; the head of a multi-package run is whatever ran first.
func TestTailLinesTakesTheTail(t *testing.T) {
	t.Parallel()
	got := tailLines(goTestShapedStdout, 2)

	if strings.Contains(got, "pkg/a") {
		t.Errorf("tail includes the FIRST line, so it is a head: %q", got)
	}
	if !strings.Contains(got, "FAIL	example.com/pkg/c") {
		t.Errorf("tail = %q, want it to carry the failing package", got)
	}
	if n := len(strings.Split(got, "\n")); n != 2 {
		t.Errorf("tail returned %d lines, want 2", n)
	}
}

func TestTailLinesShorterThanBound(t *testing.T) {
	t.Parallel()
	if got := tailLines("only\nthree\nlines", 40); got != "only\nthree\nlines" {
		t.Fatalf("tailLines = %q, want the whole input", got)
	}
	if got := tailLines("  \n\n", 10); got != "" {
		t.Fatalf("tailLines(blank) = %q, want empty", got)
	}
}

// writeFailingScript drops a script that mimics `go test ./...`: multi-line
// stdout that opens green, closes red, and exits non-zero with a silent stderr.
func writeFailingScript(t *testing.T, dir string) string {
	t.Helper()
	path := filepath.Join(dir, "failing.sh")
	body := "#!/bin/sh\ncat <<'EOF'\n" + goTestShapedStdout + "\nEOF\nexit 1\n"
	if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

// TestFailingCheckRetainsCapturedOutput is the end-to-end half: the evidence the
// design promises (§3.4 "Failures name the failing check and surface its captured
// output") must actually reach the Outcome.
func TestFailingCheckRetainsCapturedOutput(t *testing.T) {
	t.Parallel()
	work := t.TempDir()
	writeFailingScript(t, work)

	rep := Config{Checks: []Check{{
		Name: "test-race", Command: "sh failing.sh", FailOn: FailOnExit, Class: ClassRequired,
	}}}.Run(context.Background(), work)

	if len(rep.Outcomes) != 1 {
		t.Fatalf("got %d outcomes, want 1", len(rep.Outcomes))
	}
	o := rep.Outcomes[0]
	if o.Passed {
		t.Fatal("check unexpectedly passed")
	}
	if !strings.Contains(o.Output, "--- FAIL: TestResolverDelegateShimSuite") {
		t.Errorf("Output does not carry the failing test name:\n%s", o.Output)
	}
	if !strings.Contains(o.Output, "FAIL	example.com/pkg/c") {
		t.Errorf("Output does not carry the failing package:\n%s", o.Output)
	}
	if strings.Contains(o.Detail, "cached") {
		t.Errorf("Detail still reports a passing package: %q", o.Detail)
	}
}

// TestPassingCheckCarriesNoOutput keeps the field a failure record, not a log.
func TestPassingCheckCarriesNoOutput(t *testing.T) {
	t.Parallel()
	rep := Config{Checks: []Check{{
		Name: "green", Command: "true", FailOn: FailOnExit, Class: ClassRequired,
	}}}.Run(context.Background(), t.TempDir())

	if got := rep.Outcomes[0].Output; got != "" {
		t.Fatalf("passing check retained output %q, want empty", got)
	}
}

// TestFailureSummaryCarriesOutputTail covers the consumer that is not a human:
// cmd/implement-now/heal.go feeds this string to the fixer agent that must repair
// the red gate. Handed only the one-line detail, that agent was told a package
// which PASSED was the failure.
func TestFailureSummaryCarriesOutputTail(t *testing.T) {
	t.Parallel()
	work := t.TempDir()
	writeFailingScript(t, work)

	rep := Config{Checks: []Check{{
		Name: "test-race", Command: "sh failing.sh", FailOn: FailOnExit, Class: ClassRequired,
	}}}.Run(context.Background(), work)

	summary := rep.FailureSummary()
	if !strings.Contains(summary, `check "test-race"`) {
		t.Errorf("summary does not name the check:\n%s", summary)
	}
	if !strings.Contains(summary, "--- FAIL: TestResolverDelegateShimSuite") {
		t.Errorf("summary withholds the actual failure from the fixer:\n%s", summary)
	}
}

// TestFailureSummaryEmptyWhenGreen keeps the "nothing failed" contract.
func TestFailureSummaryEmptyWhenGreen(t *testing.T) {
	t.Parallel()
	rep := Config{Checks: []Check{{
		Name: "green", Command: "true", FailOn: FailOnExit, Class: ClassRequired,
	}}}.Run(context.Background(), t.TempDir())

	if got := rep.FailureSummary(); got != "" {
		t.Fatalf("FailureSummary on a green run = %q, want empty", got)
	}
}
