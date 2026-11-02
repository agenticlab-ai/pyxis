// Package checks: nonewfailures.go implements rung 2's no-new-failures floor
// (design codegate-v1 §3.7, basis FR-10, UB-9): a change must introduce no
// test failure that was passing before it. The check runs the bound test verb,
// extracts per-test failure IDENTITIES via the stack's ecosystem parser, and
// compares them to a manifest-bound baseline — reding on any identity the
// baseline does not already carry (and recording, as the ratchet's only
// permitted movement, how many baseline entries this run no longer reproduces).
// With no baseline bound, any failure at all is new. A stack whose test output
// has no failure-identity parser cannot produce identities: it skips, unless a
// baseline is bound — a misconfiguration it fails closed on.
//
// `run` NEVER writes the baseline (DINV-6): only `codegate capture-baseline`
// does. This check only ever READS it.
package checks

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/verify/baseline"
	"github.com/agenticlab-ai/pyxis/internal/verify/ecosystem"
	"github.com/agenticlab-ai/pyxis/internal/verify/runner"
	"github.com/agenticlab-ai/pyxis/internal/verify/schema"
)

const (
	noNewFailuresCheckID = "no-new-failures"
	noNewFailuresVerbID  = "test"
	newFailureClass      = "new-failure"
)

func init() {
	register(schema.Check{
		ID:                noNewFailuresCheckID,
		Rung:              "2",
		VerbDeps:          []string{noNewFailuresVerbID},
		AuthorIndependent: true,
		InputSet:          []string{"tree"},
	}, runNoNewFailures)
}

func runNoNewFailures(ctx context.Context, rc RunContext) Outcome {
	stack := rc.Manifest.Stack
	baselinePath := strings.TrimSpace(rc.Manifest.BaselinePath)
	baselineBound := baselinePath != ""

	// A stack with no failure-identity parser cannot compute per-test
	// identities. Skip (a capability gap, like verb-absent) — unless a baseline
	// is bound, which claims an evaluation this stack cannot perform: fail
	// closed rather than silently certify it.
	if !rc.Ecosystem.HasFailureIdentityParser(stack) {
		if baselineBound {
			return noNewFailuresError(fmt.Sprintf("a baseline is declared but stack %q has no failure-identity parser to evaluate it against", stack), nil)
		}
		return Outcome{Result: schema.CheckResult{CheckID: noNewFailuresCheckID, State: "skip", SkipReason: "parser-absent"}}
	}

	verb := rc.Verbs[noNewFailuresVerbID]
	res := runner.Exec(ctx, rc.Workspace, verb)
	logRef := writeLog(rc, noNewFailuresCheckID, res)
	if res.Class != "" {
		return runnerOutcome(noNewFailuresCheckID, "2", res, logRef)
	}

	failures, err := rc.Ecosystem.ParseFailureIdentities(stack, res.Stdout)
	if err != nil {
		// Unparseable test output. A bound baseline cannot be evaluated against
		// it (never silently read as zero failures — UB-9); with no baseline,
		// the verb's own exit code is the only signal left (the exit-code path).
		if baselineBound {
			return noNewFailuresError(fmt.Sprintf("cannot evaluate the bound baseline against unparseable test output: %v", err), &res)
		}
		if res.Passed {
			return passOutcome(noNewFailuresCheckID, res)
		}
		return failOutcome(noNewFailuresCheckID, "the test verb failed and its output could not be parsed for per-test identities (no baseline bound)", res, []schema.Finding{{
			CheckID: noNewFailuresCheckID, Class: newFailureClass, Rung: "2",
			Message: fmt.Sprintf("test verb exited %d", res.ExitCode), LogRef: logRef,
		}})
	}

	byID := make(map[string]ecosystem.TestFailure, len(failures))
	ids := make([]string, 0, len(failures))
	for _, f := range failures {
		byID[f.Identity] = f
		ids = append(ids, f.Identity)
	}
	sort.Strings(ids)

	if !baselineBound {
		if len(ids) == 0 {
			return passOutcome(noNewFailuresCheckID, res)
		}
		return failOutcome(noNewFailuresCheckID,
			fmt.Sprintf("%d test failure(s) with no baseline bound — every failure is new", len(ids)),
			res, newFailureFindings(ids, byID, logRef))
	}

	b, err := baseline.Load(filepath.Join(rc.Workspace, filepath.FromSlash(baselinePath)))
	if err != nil {
		// Bind-time resolution (invoke.resolveBaselineHash) already refused a
		// declared-but-missing/malformed baseline; reaching here is a fail-closed
		// backstop, never a silent pass.
		return noNewFailuresError(fmt.Sprintf("load bound baseline %q: %v", baselinePath, err), &res)
	}
	result := baseline.Evaluate(ids, b)
	if len(result.NewFailures) > 0 {
		out := failOutcome(noNewFailuresCheckID,
			fmt.Sprintf("%d new test failure(s) not in the baseline", len(result.NewFailures)),
			res, newFailureFindings(result.NewFailures, byID, logRef))
		out.Shrinkage = len(result.Shrunk)
		return out
	}
	out := passOutcome(noNewFailuresCheckID, res)
	out.Shrinkage = len(result.Shrunk)
	return out
}

// newFailureFindings builds one finding per new failure identity, carrying the
// file:line the parser recovered where it has one (§3.6).
func newFailureFindings(ids []string, byID map[string]ecosystem.TestFailure, logRef string) []schema.Finding {
	findings := make([]schema.Finding, 0, len(ids))
	for _, id := range ids {
		f := byID[id]
		message := id
		if f.Message != "" {
			message = id + ": " + f.Message
		}
		findings = append(findings, schema.Finding{
			CheckID: noNewFailuresCheckID, Class: newFailureClass, Rung: "2",
			Path: f.Path, Line: f.Line, Message: message, LogRef: logRef,
		})
	}
	return findings
}

// noNewFailuresError is a rung-2 error-class Outcome (exit 3): a configuration
// or capability fault, distinct from a check's own red. res, when non-nil,
// carries the completed run's duration.
func noNewFailuresError(message string, res *runner.Result) Outcome {
	cr := schema.CheckResult{CheckID: noNewFailuresCheckID, State: "error", Message: message}
	if res != nil {
		cr.Duration = schema.DurationMS(res.Duration)
	}
	return Outcome{Result: cr}
}
