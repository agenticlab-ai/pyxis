package drivetogreen

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/executor/port"
	"github.com/agenticlab-ai/pyxis/internal/executor/runtime"
)

type gateTaskKey struct{}

// GateTask carries the owning task into an injected runner's diagnostic context.
func GateTask(ctx context.Context) port.Task {
	task, _ := ctx.Value(gateTaskKey{}).(port.Task)
	return task
}

// Check is one deterministic gate command — the same command a required CI check
// runs. The local gate runs each Check in the task's workdir; because the check
// set is injected config shared with CI, a change that fails a required CI check
// fails the gate on the same input (prd:implement-now/FR-8, DINV-4). Name is the
// required-check name it mirrors, carried only for diagnostics.
type Check struct {
	Name string   // the required-check name this mirrors (diagnostics only)
	Argv []string // the command and its arguments, run in the workdir
}

// CommandResult is the outcome of running one gate command in a workdir.
type CommandResult struct {
	// Passed is true when the command exited zero — the check passed.
	Passed bool
	// Output is the command's captured combined stdout+stderr — the diagnostic a
	// fixer is shown (redacted + clamped) when the check fails.
	Output string
	// Err is an operational failure to RUN the command at all (not a check
	// failing): the binary is missing, the workdir is gone, the context expired.
	// It fails the gate closed — the gate never certifies green on an unrun check.
	Err error
}

// CommandRunner runs one gate command with dir as the working directory and
// reports the result. It is injected so the gate is hermetic-testable; a
// production runner execs argv[0] with argv[1:] and dir as cwd, capturing
// combined output. It must be deterministic given the same tree — the whole
// point of a gate that mirrors CI is that it reads the same signal (AP-11).
type CommandRunner func(ctx context.Context, dir string, argv []string) CommandResult

// Gate is the local gate: the one predicate that runs the CI-required checks over
// a workdir (DINV-4 — a single implementation, shared by the pre-push filter and
// every re-push of the self-heal loop). It holds the injected check set and the
// runner; it caches nothing.
type Gate struct {
	checks []Check
	run    CommandRunner
}

// NewGate builds a Gate over an injected check set and runner. The check set is
// repo-configured to the SAME checks CI requires (prd:implement-now/FR-8). An
// empty check set is permitted — the gate then vacuously passes and CI remains
// the authoritative oracle (DINV-3: success is always the verified remote
// artifact, never the local gate) — but a real deployment mirrors CI's required
// checks here so "passed local, failed CI" cannot happen (UB-5).
func NewGate(checks []Check, run CommandRunner) *Gate {
	return &Gate{checks: checks, run: run}
}

// CheckFailure is one gate check that did not pass, with its captured output.
type CheckFailure struct {
	Name   string // the check's name (or its command, when unnamed)
	Output string // the check's captured output (raw; redacted before a fixer)
}

// GateResult is the local gate's verdict over one workdir.
type GateResult struct {
	// Passed is true only when every configured check passed. The zero value is
	// not-passed (fail-closed).
	Passed bool
	// Failures lists the checks that did not pass, in configured order. Empty when
	// Passed.
	Failures []CheckFailure
}

// Evaluate runs every configured gate check in the workdir and returns the
// verdict. It is the local-gate predicate (DINV-4): the same deterministic checks
// CI requires, run on the same input, so a required-CI-check failure reproduces
// here before a push (prd:implement-now/FR-8, UB-5).
//
// Semantics:
//   - Every check runs (the result lists ALL failures, so a fixer sees the whole
//     picture, not just the first break).
//   - A check that exits non-zero is a Failure; the gate is not Passed.
//   - An operational failure to RUN a check (CommandResult.Err) fails closed: the
//     gate cannot certify green on a check it could not run, so Evaluate returns a
//     non-nil error (never a false pass).
//   - Context cancellation is honored between checks and surfaced as an error.
//
// A nil work is a wiring error, reported (never panicked) so the caller fails
// closed.
func (g *Gate) Evaluate(ctx context.Context, work port.Workdir) (GateResult, error) {
	if work == nil {
		return GateResult{}, errors.New("drivetogreen: gate evaluated with a nil workdir")
	}
	if g.run == nil {
		return GateResult{}, errors.New("drivetogreen: gate has no command runner")
	}
	dir := work.Dir()
	if integration, ok := work.(interface {
		RefreshMainline(context.Context) (string, error)
	}); ok {
		base, err := integration.RefreshMainline(ctx)
		if err != nil {
			return GateResult{}, fmt.Errorf("drivetogreen: refresh integration tree: %w", err)
		}
		ctx = runtime.WithVerificationBase(ctx, base)
	}

	var failures []CheckFailure
	for _, c := range g.checks {
		if err := ctx.Err(); err != nil {
			return GateResult{}, fmt.Errorf("drivetogreen: gate cancelled before check %q: %w", checkLabel(c), err)
		}
		res := g.run(ctx, dir, c.Argv)
		if res.Err != nil {
			// Could not run the check at all — fail closed, never a silent pass.
			return GateResult{}, fmt.Errorf("drivetogreen: run gate check %q: %w", checkLabel(c), res.Err)
		}
		if !res.Passed {
			failures = append(failures, CheckFailure{Name: checkLabel(c), Output: res.Output})
		}
	}
	return GateResult{Passed: len(failures) == 0, Failures: failures}, nil
}

// summarize renders a gate result's failures into a single human-readable
// diagnostic — the freshest signal fed (after redaction) to the next fixer when
// the local gate blocks a push. It names each failing check and inlines its
// captured output, clearly fenced as data.
func (r GateResult) summarize() string {
	if r.Passed || len(r.Failures) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("the local gate (mirroring required CI) is red — fix these checks:\n")
	for _, f := range r.Failures {
		fmt.Fprintf(&b, "\ncheck %q failed:\n", f.Name)
		out := strings.TrimRight(f.Output, "\n")
		if out == "" {
			b.WriteString("(no output captured)\n")
			continue
		}
		b.WriteString(out)
		b.WriteString("\n")
	}
	return b.String()
}

// failedNames lists the names of the failing checks, for a compact escalation
// reason that carries no log body.
func (r GateResult) failedNames() []string {
	names := make([]string, 0, len(r.Failures))
	for _, f := range r.Failures {
		names = append(names, f.Name)
	}
	return names
}

// checkLabel is a check's diagnostic label: its configured Name, or its command
// line when unnamed.
func checkLabel(c Check) string {
	if strings.TrimSpace(c.Name) != "" {
		return c.Name
	}
	if len(c.Argv) > 0 {
		return strings.Join(c.Argv, " ")
	}
	return "(empty check)"
}
