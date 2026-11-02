package drivetogreen

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// requiredCheck is THE single deterministic required-check predicate — the one
// implementation both "CI" and the local gate run over the same input (DINV-4,
// prd:implement-now/FR-18). Modeled concretely: a change is good iff the workdir
// holds a "build.ok" marker. Because there is exactly one predicate, two copies
// cannot diverge (that is the invariant); the test drives the gate through it and
// asserts the gate's verdict tracks CI's on the same tree.
func requiredCheck(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, "build.ok"))
	return err == nil
}

// gateOverRequiredCheck builds a local gate whose one check runs requiredCheck on
// the workdir — i.e. the SAME command CI requires (prd:implement-now/FR-8). The
// captured output stands in for a failing check's log.
func gateOverRequiredCheck() *Gate {
	run := func(_ context.Context, dir string, _ []string) CommandResult {
		if requiredCheck(dir) {
			return CommandResult{Passed: true, Output: "build: ok\n"}
		}
		return CommandResult{Passed: false, Output: "build: FAIL — missing build.ok\n"}
	}
	return NewGate([]Check{{Name: "build", Argv: []string{"make", "build"}}}, run)
}

// Acceptance criterion 1: a change that fails a required CI check also fails the
// local gate on the same input (prd:implement-now/FR-8, UB-5).
func TestGate_MirrorsRequiredCI(t *testing.T) {
	ctx := context.Background()

	// A change that fails the required CI check: no build.ok marker in the tree.
	failing := &fakeWorkdir{dir: t.TempDir(), branch: "feat/x"}
	if requiredCheck(failing.dir) {
		t.Fatal("precondition: the failing tree must fail the required CI check")
	}
	gate := gateOverRequiredCheck()
	res, err := gate.Evaluate(ctx, failing)
	if err != nil {
		t.Fatalf("Evaluate(failing): unexpected error: %v", err)
	}
	// CI would be red on this input; the local gate is red on the SAME input.
	if res.Passed {
		t.Fatal("a change failing the required CI check passed the local gate (FR-8 violated)")
	}
	if len(res.Failures) != 1 || res.Failures[0].Name != "build" {
		t.Fatalf("gate failures = %+v, want one failure named build", res.Failures)
	}
	if res.Failures[0].Output == "" {
		t.Error("a failing check must carry its output for the fixer diagnostic")
	}

	// A change that PASSES the required CI check: same predicate, same tree, now
	// with the marker. The gate must agree (green), proving it tracks CI both ways.
	passing := &fakeWorkdir{dir: t.TempDir(), branch: "feat/x"}
	if err := os.WriteFile(filepath.Join(passing.dir, "build.ok"), []byte("ok"), 0o644); err != nil {
		t.Fatalf("seed passing tree: %v", err)
	}
	if !requiredCheck(passing.dir) {
		t.Fatal("precondition: the passing tree must pass the required CI check")
	}
	green, err := gate.Evaluate(ctx, passing)
	if err != nil {
		t.Fatalf("Evaluate(passing): unexpected error: %v", err)
	}
	if !green.Passed {
		t.Fatalf("a change passing the required CI check failed the local gate: %+v", green.Failures)
	}
}

// A gate runs EVERY configured check, so the result lists all failures (a fixer
// sees the whole picture), and the verdict is red if any check fails.
func TestGate_RunsAllChecksAndListsFailures(t *testing.T) {
	calls := map[string]int{}
	run := func(_ context.Context, _ string, argv []string) CommandResult {
		name := argv[0]
		calls[name]++
		// "vet" passes; "test" fails.
		if name == "vet" {
			return CommandResult{Passed: true}
		}
		return CommandResult{Passed: false, Output: name + " failed\n"}
	}
	gate := NewGate([]Check{
		{Name: "vet", Argv: []string{"vet"}},
		{Name: "test", Argv: []string{"test"}},
	}, run)

	res, err := gate.Evaluate(context.Background(), &fakeWorkdir{dir: t.TempDir()})
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if res.Passed {
		t.Fatal("gate passed with a failing check")
	}
	if calls["vet"] != 1 || calls["test"] != 1 {
		t.Fatalf("each check must run exactly once: %v", calls)
	}
	if len(res.Failures) != 1 || res.Failures[0].Name != "test" {
		t.Fatalf("failures = %+v, want [test]", res.Failures)
	}
}

// An operational failure to RUN a check fails closed: the gate never certifies
// green on a check it could not run (fail-closed, DINV-3).
func TestGate_UnrunnableCheckFailsClosed(t *testing.T) {
	run := func(_ context.Context, _ string, _ []string) CommandResult {
		return CommandResult{Err: errors.New("exec: command not found")}
	}
	gate := NewGate([]Check{{Name: "test", Argv: []string{"nope"}}}, run)

	res, err := gate.Evaluate(context.Background(), &fakeWorkdir{dir: t.TempDir()})
	if err == nil {
		t.Fatal("Evaluate: want an error when a check cannot be run")
	}
	if res.Passed {
		t.Fatal("gate certified green despite an unrunnable check")
	}
}

// An empty gate vacuously passes — CI remains the authoritative oracle (DINV-3);
// a real deployment configures the CI-required checks here.
func TestGate_EmptyIsVacuousPass(t *testing.T) {
	gate := NewGate(nil, func(context.Context, string, []string) CommandResult {
		t.Fatal("an empty gate must not run any command")
		return CommandResult{}
	})
	res, err := gate.Evaluate(context.Background(), &fakeWorkdir{dir: t.TempDir()})
	if err != nil {
		t.Fatalf("Evaluate(empty): %v", err)
	}
	if !res.Passed {
		t.Fatal("an empty gate must vacuously pass")
	}
}

// A nil workdir is a wiring error, reported (never panicked) so the caller fails
// closed.
func TestGate_NilWorkdirIsError(t *testing.T) {
	gate := gateOverRequiredCheck()
	if _, err := gate.Evaluate(context.Background(), nil); err == nil {
		t.Fatal("Evaluate(nil workdir): want an error")
	}
}

func TestGate_MissingRunnerAndCancellationFailClosed(t *testing.T) {
	work := &fakeWorkdir{dir: t.TempDir()}
	if _, err := NewGate(nil, nil).Evaluate(context.Background(), work); err == nil {
		t.Fatal("Evaluate without a runner: want an error")
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	gate := NewGate([]Check{{Argv: []string{"test"}}}, func(context.Context, string, []string) CommandResult {
		t.Fatal("a pre-cancelled gate must not run a command")
		return CommandResult{}
	})
	if _, err := gate.Evaluate(ctx, work); !errors.Is(err, context.Canceled) {
		t.Fatalf("Evaluate(cancelled) error = %v, want context.Canceled", err)
	}
}

func TestGateDiagnosticsCoverEmptyOutputAndFallbackLabels(t *testing.T) {
	result := GateResult{Failures: []CheckFailure{{Name: "test"}}}
	if got := result.summarize(); !strings.Contains(got, "(no output captured)") {
		t.Fatalf("summarize(empty output) = %q", got)
	}
	if got := checkLabel(Check{Argv: []string{"go", "test", "./..."}}); got != "go test ./..." {
		t.Fatalf("checkLabel(command) = %q", got)
	}
	if got := checkLabel(Check{}); got != "(empty check)" {
		t.Fatalf("checkLabel(empty) = %q", got)
	}
}
