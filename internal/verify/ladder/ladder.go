// Package ladder executes the derived check registry end to end: a
// latency-ordered run with first-failure stop (FR-1), skip:verb-absent at
// check granularity (FR-6), skip:not-reached for every rung a stop leaves
// behind (DINV-4), and the resulting green / green-degraded / red / error
// state (FR-17, FR-19).
//
// It imports internal/verify/checks unconditionally so that package's
// self-registering check units (format, lint, build, exec-probe — T3; more
// arrive by later tasks as pure additions) are always present before a run:
// the ladder is the one caller that must see the full derived registry.
package ladder

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/runlogs"
	"github.com/agenticlab-ai/pyxis/internal/verify/checks"
	"github.com/agenticlab-ai/pyxis/internal/verify/diffset"
	"github.com/agenticlab-ai/pyxis/internal/verify/ecosystem"
	"github.com/agenticlab-ai/pyxis/internal/verify/registry"
	"github.com/agenticlab-ai/pyxis/internal/verify/schema"
)

// nowFunc is the injectable clock (tests override it for deterministic
// timestamps), mirroring internal/verify/invoke's own pattern.
var nowFunc = time.Now

// stateRank orders CheckResult/Rung states by severity so a rung's aggregate
// is its worst check: error (UB-1/UB-2's infra classing) outranks fail (a
// genuine check red), which outranks pass — a skip never raises the rung
// above pass (FR-6's pass-with-skips).
var stateRank = map[string]int{"pass": 0, "skip": 0, "fail": 1, "error": 2}

// Run executes every registered check in registry (latency) order, stopping
// between rungs on the first fail or error, and returns a complete verdict:
// every configured rung is represented — reached, with every one of its
// registered checks recorded (pass, fail, error, or skip:verb-absent) — or,
// past the stop, skip:not-reached.
func Run(ctx context.Context, workspace string, manifest *schema.Manifest, bindings schema.Bindings, mode string) *schema.Verdict {
	return run(ctx, workspace, manifest, bindings, mode, nil)
}

// run is Run's shared implementation, plus T8's incremental hook: when plan
// is non-nil, a rung plan marks as carried is spliced in from the prior
// verdict instead of executed (RunIncremental, incremental.go). plan == nil
// (Run's own path) behaves exactly as before this task — every rung always
// executes.
func run(ctx context.Context, workspace string, manifest *schema.Manifest, bindings schema.Bindings, mode string, plan *RerunSet) *schema.Verdict {
	start := nowFunc()
	runID := fmt.Sprintf("run-%d", start.UnixNano())
	logDir := filepath.Join(workspace, filepath.FromSlash(runlogs.Codegate), runID)

	eco := ecosystem.New()
	verbs := eco.ResolveVerbs(manifest.Stack, manifest.Verbs)
	rungs := registry.GetLadder()

	// Extract the proposal diff once, shared by every diff-based check through
	// the RunContext (rather than each re-running git). A failure leaves Diff
	// nil; a diff-based check fails closed on a nil diff — it cannot verify what
	// it cannot see.
	diff, err := diffset.Extract(ctx, workspace, bindings.DiffBase, bindings.HeadSHA)
	if err != nil {
		diff = nil
	}

	base := checks.RunContext{
		Workspace: workspace,
		Verbs:     verbs,
		LogDir:    logDir,
		Bindings:  bindings,
		Manifest:  manifest,
		Ecosystem: eco,
		Diff:      diff,
	}

	verdict := &schema.Verdict{
		RunID:     runID,
		Workspace: workspace,
		Mode:      mode,
		StartTime: start,
		Rungs:     map[string]*schema.Rung{},
		Findings:  []schema.Finding{},
		Bindings:  bindings,
	}

	anySkip := false
	stoppedState := ""

	for i, r := range rungs {
		var rung *schema.Rung
		var findings []schema.Finding
		if plan != nil {
			rung = plan.Carry[r.Name]
		}
		if rung == nil {
			rung, findings = evaluateRung(ctx, r, base)
		}
		verdict.Rungs[r.Name] = rung
		verdict.Findings = append(verdict.Findings, findings...)
		if rungHasSkip(rung) {
			anySkip = true
		}
		if rung.State == "error" || rung.State == "fail" {
			stoppedState = rung.State
			for _, remaining := range rungs[i+1:] {
				verdict.Rungs[remaining.Name] = &schema.Rung{Name: remaining.Name, Order: remaining.Order, Skipped: true, SkipReason: "not-reached"}
			}
			break
		}
	}

	switch {
	case stoppedState == "error":
		verdict.State = "error"
	case stoppedState == "fail":
		verdict.State = "red"
	case !anyExecuted(verdict):
		// Every check skipped — nothing was actually verified (e.g. an unknown
		// or unsupported stack bound no verbs). "Verified nothing" is not a
		// pass: fail closed (INV-6) rather than certify green-degraded and exit
		// 0 on a run that executed zero checks.
		verdict.State = "error"
		verdict.Findings = append(verdict.Findings, schema.Finding{
			CheckID: "ladder",
			Class:   "no-checks-executed",
			Message: fmt.Sprintf("no verification check executed for stack %q — every check skipped (verb-absent); nothing was verified", manifest.Stack),
		})
	case anySkip:
		verdict.State = "green-degraded"
	default:
		verdict.State = "green"
	}
	verdict.EndTime = nowFunc()

	if err := verdict.ValidateCompleteness(expectedChecks(rungs)); err != nil {
		// Every rung the loop above touched got a full Checks map, and every
		// remaining rung got skip:not-reached — a completeness failure here
		// can only mean a bug in this function's own bookkeeping, not a
		// runtime condition a caller could act on.
		panic(fmt.Sprintf("ladder: verdict incomplete against the derived registry: %v", err))
	}

	return verdict
}

// evaluateRung runs every check registered for r, resolving each against
// verbs first: a check with any unbound verb dependency records
// skip:verb-absent WITHOUT executing (FR-6) — the ladder's own concern, never
// a check's. The rung's state is the worst of its checks' states.
func evaluateRung(ctx context.Context, r *registry.Rung, base checks.RunContext) (*schema.Rung, []schema.Finding) {
	rungStart := nowFunc()
	rung := &schema.Rung{Name: r.Name, Order: r.Order, Checks: map[string]*schema.CheckResult{}, State: "pass"}
	var findings []schema.Finding

	for _, meta := range r.Checks {
		if !allBound(meta.VerbDeps, base.Verbs) {
			// A per-check skip. The rung-level SkipReason is NOT set here — it
			// belongs to a whole-rung skip (not-reached); tagging a rung that
			// still ran other checks with "verb-absent" would contradict its
			// own skipped:false and mislead a consumer keying off the field.
			rung.Checks[meta.ID] = &schema.CheckResult{CheckID: meta.ID, State: "skip", SkipReason: "verb-absent"}
			continue
		}

		fn := checks.Get(meta.ID)
		if fn == nil {
			// register() always pairs metadata with an executor in the same
			// call (internal/verify/checks.register) — a registered check
			// with no executor is unreachable except by a coding mistake.
			panic(fmt.Sprintf("ladder: check %q is registered with no executor wired", meta.ID))
		}

		outcome := safeRun(ctx, fn, base, meta.ID)
		result := outcome.Result
		if !knownState(result.State) {
			// A check must return one of the modeled states; any other value is
			// a harness/check bug and fails closed (error), never silently
			// ranked as a pass.
			result.Message = fmt.Sprintf("check %q returned an unmodeled state %q (failed closed)", meta.ID, result.State)
			result.State = "error"
		}
		rung.Checks[meta.ID] = &result
		findings = append(findings, outcome.Findings...)
		rung.Shrinkage += outcome.Shrinkage

		if stateRank[result.State] > stateRank[rung.State] {
			rung.State = result.State
		}
	}

	rung.Duration = schema.DurationMS(nowFunc().Sub(rungStart))
	return rung, findings
}

// safeRun invokes a check executor behind a panic barrier: a check that panics
// (a nil deref, or a malformed-input bug in a future rung's executor) is
// contained as an error-class result, so the run fails closed and still writes
// a verdict artifact instead of crashing the process past the writer with an
// unrecognized exit code.
func safeRun(ctx context.Context, fn checks.Func, rc checks.RunContext, checkID string) (out checks.Outcome) {
	defer func() {
		if r := recover(); r != nil {
			out = checks.Outcome{Result: schema.CheckResult{CheckID: checkID, State: "error", Message: fmt.Sprintf("check %q panicked: %v", checkID, r)}}
		}
	}()
	return fn(ctx, rc)
}

// knownState reports whether s is one of the modeled check/rung states; an
// unknown value fails closed (evaluateRung) rather than ranking as a pass.
func knownState(s string) bool {
	switch s {
	case "pass", "skip", "fail", "error":
		return true
	}
	return false
}

// allBound reports whether every verb name in deps resolves in verbs.
func allBound(deps []string, verbs map[string]*schema.Verb) bool {
	for _, d := range deps {
		if _, ok := verbs[d]; !ok {
			return false
		}
	}
	return true
}

// rungHasSkip reports whether any check in rung recorded a skip — the signal
// that drives the overall verdict to green-degraded (FR-17) when nothing
// failed or errored.
func rungHasSkip(rung *schema.Rung) bool {
	for _, c := range rung.Checks {
		if c.State == "skip" {
			return true
		}
	}
	return false
}

// anyExecuted reports whether at least one check in the whole verdict actually
// ran (produced a non-skip result). It is the fail-closed floor: a verdict in
// which every check skipped verified nothing and must never be green.
func anyExecuted(verdict *schema.Verdict) bool {
	for _, rung := range verdict.Rungs {
		for _, c := range rung.Checks {
			if c.State != "skip" {
				return true
			}
		}
	}
	return false
}

func expectedChecks(rungs []*registry.Rung) map[string][]string {
	expected := make(map[string][]string, len(rungs))
	for _, r := range rungs {
		ids := make([]string, len(r.Checks))
		for i, c := range r.Checks {
			ids[i] = c.ID
		}
		expected[r.Name] = ids
	}
	return expected
}
