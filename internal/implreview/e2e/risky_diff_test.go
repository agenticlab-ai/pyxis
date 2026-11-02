package e2e

import (
	"context"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/implreview/band"
	"github.com/agenticlab-ai/pyxis/internal/implreview/charter"
	"github.com/agenticlab-ai/pyxis/internal/implreview/checkpoint"
	"github.com/agenticlab-ai/pyxis/internal/implreview/harvest"
	"github.com/agenticlab-ai/pyxis/internal/implreview/report"
	"github.com/agenticlab-ai/pyxis/internal/implreview/verdict"
	"github.com/agenticlab-ai/pyxis/internal/lane/classifier"
	"github.com/agenticlab-ai/pyxis/internal/lane/policy"
)

// TestScenario_RiskyDiff drives the PRD §4.4 "risky diff" end to end:
//
//	A proposal touches the acquisition-pin path (machinery — deep band). The
//	security lens runs with the served ABUSE rows and the incident registry in
//	context and flags a fail-open pattern the tests never exercise. Request-changes;
//	the worker fixes; round 2 confirms; the report shows the catch. Post-adoption,
//	this verdict also forces the HUMAN lane.
//
// Requirements exercised end-to-end: FR-9 (deep band selected from the lane's
// FLOOR hit — a hard-gated surface), FR-10 (the deep-band MANDATORY context —
// served ABUSE rows + incident registry — is present in the pack the lenses read),
// FR-2/FR-3 (fresh-instance sibling + four blind lenses), FR-7 (request-changes,
// downgrade-only), FR-1 (round 2 confirms closure), FR-6/DINV-1/2/9 (one artifact,
// byte-identical), FR-11 (advisory→adopted ramp), UB-6/DINV-7 (staleness), and the
// post-adoption HUMAN routing (the checkpoint gate blocks the downgrade verdict +
// the emitted downgrade signal).
func TestScenario_RiskyDiff(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	ctx := context.Background()

	const (
		proposalRef = "github:agenticlab-ai/pyxis#977"
		reviewID    = "rv-risky-1"
		rev1        = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
		rev2        = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	)

	// A charter for the machinery-touching task. Present + well-formed → proceed.
	charterText := []byte("Task: add the acquisition-pin verification path. The pin check MUST fail " +
		"closed — a check error denies acquisition; it must never widen eligibility on error.")

	// The round-1 diff introduces a fail-open on the authority-granting path — a
	// check that returns 'allowed' when verification errors (CAL-9). The tests only
	// exercise the happy path, so the deterministic ladder is green.
	diffRound1 := []byte("--- a/internal/machinery/acquire.go\n+++ b/internal/machinery/acquire.go\n@@\n" +
		" func (m *Machinery) mayAcquire(pin string) bool {\n-\tok, err := m.verifyPin(pin)\n" +
		"-\tif err != nil {\n-\t\treturn false // fail closed\n-\t}\n-\treturn ok\n" +
		"+\tok, err := m.verifyPin(pin)\n+\tif err != nil {\n+\t\treturn true // TODO: verification flaky; allow for now\n+\t}\n+\treturn ok\n }")

	codegateVerdict := []byte("codegate: ladder green (happy-path tests pass; the error path is untested)")
	designBindings := []byte("design binding DINV: authority-granting paths fail closed on error (no eligibility-widening on error)")

	// ---- charter proceeds (FR-14) ----
	cout, err := charter.Validate(charter.Invocation{Charter: string(charterText)})
	if err != nil || !cout.Proceeds() {
		t.Fatalf("charter.Validate should proceed: kind=%s err=%v", cout.Kind, err)
	}

	// ---- Step 1: DEEP band from the lane's FLOOR hit (FR-9) ----
	// The acquisition-pin path is a hard-gated (floor) surface → HUMAN with a floor
	// hit → the review concentrates its scrutiny → DEEP band. The class recorded is
	// the floor hit.
	laneVerdict := classifier.Classify(classifier.Inputs{
		Diff: classifier.Diff{Paths: []string{"internal/machinery/acquire.go"}, Classes: []string{"machinery-edit"}},
		Base: "base-risky", Head: rev1, CheckState: "green",
	}, riskyLanePolicy(), nil)
	bandDecision := band.Select(laneVerdict)
	if bandDecision.Band != report.BandDeep {
		t.Fatalf("risky-diff band = %q, want deep (FR-9); reason: %s", bandDecision.Band, bandDecision.Reason)
	}
	if bandDecision.Class == "" {
		t.Fatal("a deep band selected from a floor hit must record the floor class (FR-9)")
	}

	// ---- Step 2: deep-band pack WITH the mandatory context (FR-10) + review ----
	// The served ABUSE rows + incident registry are the deep-band mandatory context
	// band.EnrichInputs injects into the pack. The fake security lens asserts (via
	// the runner's call log) that the context reached it — the deep-band cost audit.
	deepContext := band.DeepContext{
		AbuseRows: []byte("ABUSE-5: a fail-open on an authority-granting path is forbidden — a check that " +
			"widens eligibility on error is an abuse the security lens must catch."),
		IncidentRegistry: []byte("I3 (fail-closed lesson): an unclassifiable/erroring input must fail closed, never widen access."),
	}
	// The security lens flags the fail-open (CAL-9, blocking). The other lenses find
	// nothing on this diff.
	h.runner.seed(string(diffRound1),
		seededFinding{
			id: "FND-1", ruleID: "security-abuse/fail-open-on-authority-path", lens: report.LensSecurityAbuse,
			severity: report.SeverityBlocking, calRow: "CAL-9",
			message: "mayAcquire returns true when verifyPin ERRORS — a fail-open on an authority-granting path " +
				"(ABUSE-5): a verification error now WIDENS eligibility. The tests exercise only the happy path.",
			path: "internal/machinery/acquire.go", line: 4,
		},
	)

	pack1 := h.buildPack(t, reviewInputs{
		subject: proposalRef, proposalDiff: diffRound1, codegateVerdict: codegateVerdict,
		charter: charterText, designBindings: designBindings, reviewedRevision: rev1,
		band: report.BandDeep, deep: deepContext,
	})
	round1 := h.runRound(t, pack1, rev1, nil)

	// FR-10: the security lens ran with the deep-band mandatory context in the pack.
	assertSecurityLensSawDeepContext(t, h.runner)

	// FR-3/FR-10: the seeded fail-open finding survived and is blocking.
	if !hasFinding(round1.Round.Findings, "FND-1", report.SeverityBlocking) {
		t.Fatal("the deep-band security lens must catch the fail-open finding (FR-10)")
	}

	// ---- Step 3: verdict → request-changes (FR-7) ----
	r1, out1 := h.assembleReport(t, proposalRef, reviewID, report.BandDeep, verdict.Signals{}, round1.Round)
	if out1.Verdict != report.VerdictRequestChanges {
		t.Fatalf("risky-diff round-1 verdict = %q, want request-changes (FR-7)", out1.Verdict)
	}
	if out1.Authority != verdict.AuthorityReturnToWorker {
		t.Fatalf("request-changes authority = %q, want return-to-worker (FR-7)", out1.Authority)
	}

	// UB-6/DINV-7: the round-1 verdict certifies rev1. After the worker's fix moves
	// the head to rev2, the round-1 verdict is STALE (treated as absent) until a
	// re-review binds the new tree.
	if verdict.IsStale(r1, rev1) {
		t.Fatal("the round-1 verdict must be current at its own reviewed revision (DINV-7)")
	}
	if !verdict.IsStale(r1, rev2) {
		t.Fatal("after the head moves to rev2 the round-1 verdict must be stale (UB-6/DINV-7)")
	}
	if _, present := verdict.Current(r1, rev2); present {
		t.Fatal("a stale verdict must be treated as absent, never current (UB-6)")
	}

	// ---- Step 4: the worker fixes; round 2 confirms closure (FR-1) ----
	diffRound2 := []byte("--- a/internal/machinery/acquire.go\n+++ b/internal/machinery/acquire.go\n@@\n" +
		" func (m *Machinery) mayAcquire(pin string) bool {\n+\tok, err := m.verifyPin(pin)\n" +
		"+\tif err != nil {\n+\t\treturn false // fail closed on a verification error\n+\t}\n+\treturn ok\n }")
	// The fix closes the defect — the security lens finds nothing on rev2.
	h.runner.seed(string(diffRound2)) // no seeded findings → the round is clean
	pack2 := h.buildPack(t, reviewInputs{
		subject: proposalRef, proposalDiff: diffRound2, codegateVerdict: codegateVerdict,
		charter: charterText, designBindings: designBindings, reviewedRevision: rev2,
		band: report.BandDeep, deep: deepContext,
	})
	round2 := h.runRound(t, pack2, rev2, nil)
	if len(round2.Round.Findings) != 0 {
		t.Fatalf("round 2 over the fixed diff should be clean; got %d findings", len(round2.Round.Findings))
	}

	// The final two-round report: round 1 (the catch, worker-applied) + round 2
	// (clean). The latest round is clean → approve. The report SHOWS the catch
	// (round 1's finding persists in the payload — the catch is durable, §3.1).
	appliedCatch := withDisposition(round1.Round, "FND-1", report.Disposition{
		Actor: report.ActorWorker, Verdict: report.VerdictApplied,
	})
	finalReport, finalOut := h.assembleReport(t, proposalRef, reviewID, report.BandDeep, verdict.Signals{},
		appliedCatch, round2.Round)
	if finalOut.Verdict != report.VerdictApprove {
		t.Fatalf("final verdict after a confirmed fix = %q, want approve (FR-7)", finalOut.Verdict)
	}
	// The catch is shown in the report (round 1's blocking finding is in the payload).
	if !reportCarriesFinding(finalReport, "FND-1") {
		t.Fatal("the report must show the security catch even after the fix (§3.1 durable findings)")
	}

	// ---- Step 5: persist ONE artifact + read back byte-identically (FR-6) ----
	reviewRef := h.persistAndReadBack(t, finalReport)

	// ---- Step 6: post-adoption the request-changes verdict forces HUMAN ----
	// Build a request-changes payload at its live head (the pre-fix state the lane
	// would route on) and prove the ADOPTED ramp gates it while the advisory ramp
	// does not (FR-11), and the emitted downgrade signal routes it (F3/J4).
	//
	// Advisory: nothing gates (FR-11).
	advisoryGate := checkpoint.NewGate("agenticlab-ai/pyxis")
	if advisoryGate.LaneRoutingBlocked(report.VerdictRequestChanges) {
		t.Fatal("an advisory gate must not force lane routing on request-changes (FR-11)")
	}
	if advisoryGate.RaiseBlocked(report.VerdictRequestChanges) {
		t.Fatal("an advisory gate must not block a request-changes raise (FR-11)")
	}

	// Adopt the ramp on a clearing checkpoint (the operator's recorded act), then
	// the same downgrade verdict gates the raise + lane routing (post-adoption
	// HUMAN routing).
	adoptedGate := promoteGate(t, "agenticlab-ai/pyxis")
	if !adoptedGate.RaiseBlocked(report.VerdictRequestChanges) {
		t.Fatal("post-adoption a request-changes verdict must block the raise (FR-11)")
	}
	if !adoptedGate.LaneRoutingBlocked(report.VerdictRequestChanges) {
		t.Fatal("post-adoption a request-changes verdict must force lane routing (FR-11, F3)")
	}

	// The emitted downgrade signal (verdict.SignalFor) is the well-formed record a
	// merge lane reads. On the pre-fix (request-changes) state at its live head, the
	// signal is present and downgrades (return-to-worker); it never grants AUTO.
	sig := verdict.SignalFor(r1, rev1)
	if !sig.Present {
		t.Fatal("the downgrade signal must be present for a current request-changes verdict (F3)")
	}
	if !sig.Downgrades() {
		t.Fatal("a request-changes verdict must emit a downgrade signal (F3/J4)")
	}
	if down, _ := sig.DowngradesToHuman(); down {
		t.Fatal("a request-changes verdict returns to the worker, not the HUMAN lane, per the signal (F3)")
	}
	// A stale signal (head moved) is ABSENT and downgrades nothing (UB-6).
	staleSig := verdict.SignalFor(r1, rev2)
	if staleSig.Present || staleSig.Downgrades() {
		t.Fatal("a stale verdict emits an absent signal that downgrades nothing (UB-6/DINV-7)")
	}

	// ---- Step 7: the harvest counts this deep-band review (FR-13) ----
	universe := []harvest.Proposal{{Ref: proposalRef, Review: refPtr(reviewRef)}}
	metrics, err := harvest.Harvest(ctx,
		harvest.EnumeratorFunc(func(context.Context) ([]harvest.Proposal, error) { return universe, nil }),
		h.port,
		harvest.NewGitHubResolver(operatorLogin, newFakeAuthors()),
	)
	if err != nil {
		t.Fatalf("harvest: %v", err)
	}
	if metrics.Covered != 1 || metrics.Universe != 1 {
		t.Fatalf("harvest coverage = %d/%d, want 1/1 (FR-13)", metrics.Covered, metrics.Universe)
	}
}

// assertSecurityLensSawDeepContext asserts the security lens ran with the deep-band
// mandatory context present in the pack it read (FR-10 — the deep-band cost audit).
func assertSecurityLensSawDeepContext(t *testing.T, r *fakeLensRunner) {
	t.Helper()
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, c := range r.calls {
		if c.lens == report.LensSecurityAbuse {
			if !c.packHasAbuse {
				t.Fatal("the deep-band security lens ran WITHOUT the served ABUSE rows in its pack (FR-10)")
			}
			return
		}
	}
	t.Fatal("the security lens never ran (FR-3)")
}

// reportCarriesFinding reports whether any round of the report carries a finding
// with the given id — the "the catch is durable in the payload" assertion.
func reportCarriesFinding(r *report.Report, id string) bool {
	for ri := range r.Rounds {
		for fi := range r.Rounds[ri].Findings {
			if r.Rounds[ri].Findings[fi].ID == id {
				return true
			}
		}
	}
	return false
}

// promoteGate builds an ADOPTED checkpoint gate: it evaluates a clearing
// ten-review checkpoint over a harvested corpus that meets the ship rule and
// records the operator's promotion (the only path to adoption — §3.3). It is the
// shared "post-adoption" fixture the risky-diff scenario uses.
func promoteGate(t *testing.T, repo string) *checkpoint.Gate {
	t.Helper()
	// A clearing corpus: 10 proposals all reviewed (100% coverage), 6 operator-
	// accepted blocking findings (≥ the floor of 5, precision 1.0), zero nits.
	clearing := harvest.Metrics{
		Universe: 10, Covered: 10,
		BlockingDisposed: 6, BlockingAccepted: 6,
		Reviews: 10, OperatorRejected: 0,
	}
	rec := checkpoint.Evaluate(checkpoint.Inputs{
		Reviews:    10,
		Metrics:    clearing,
		M3Green:    true,
		NitP50:     0,
		Engagement: checkpoint.Engagement{DwellP50: 3.0, BurstFlag: false},
		Policy:     checkpoint.DefaultPolicy(),
	})
	if !rec.ShipRuleMet {
		t.Fatalf("the clearing checkpoint must meet the ship rule (§3.3); outcome=%s hold=%s", rec.Outcome, rec.HoldReason)
	}
	gate := checkpoint.NewGate(repo)
	if err := gate.Promote(operatorLogin, rec); err != nil {
		t.Fatalf("gate.Promote on a clearing checkpoint: %v", err)
	}
	if !gate.Adopted() {
		t.Fatal("the gate must be adopted after a recorded promotion")
	}
	return gate
}

// riskyLanePolicy is a merge-lane policy whose FLOOR hard-gates the
// machinery-edit class (and the machinery path), so the risky diff hits the floor
// → HUMAN with a floor hit → DEEP band.
func riskyLanePolicy() *policy.Policy {
	return &policy.Policy{
		SchemaVersion: policy.SchemaVersion,
		RatifiedBy:    "@mdarabi",
		Floor: policy.Floor{
			HardGatedPaths:   []string{"internal/machinery/**"},
			HardGatedClasses: []string{"machinery-edit"},
		},
		AutoClasses: []policy.AutoClass{
			{Name: "engine-code", Paths: []string{"internal/**", "cmd/**"}},
		},
	}
}
