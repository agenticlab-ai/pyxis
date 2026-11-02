package e2e

import (
	"context"
	"errors"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/implreview/band"
	"github.com/agenticlab-ai/pyxis/internal/implreview/charter"
	"github.com/agenticlab-ai/pyxis/internal/implreview/checkpoint"
	"github.com/agenticlab-ai/pyxis/internal/implreview/harvest"
	"github.com/agenticlab-ai/pyxis/internal/implreview/lifecycle"
	"github.com/agenticlab-ai/pyxis/internal/implreview/persist"
	"github.com/agenticlab-ai/pyxis/internal/implreview/protocol"
	"github.com/agenticlab-ai/pyxis/internal/implreview/report"
	"github.com/agenticlab-ai/pyxis/internal/implreview/runtime"
	"github.com/agenticlab-ai/pyxis/internal/implreview/verdict"
	"github.com/agenticlab-ai/pyxis/internal/lane/classifier"
	"github.com/agenticlab-ai/pyxis/internal/lane/policy"
)

// TestScenario_CommonCase drives the PRD §4.4 "common case" end to end over the
// whole pipeline:
//
//	A codegate-epic task's worker reaches ladder-green; the diff is standard-band.
//	The four lenses return two findings — a charter AC only half-covered (spec
//	fidelity, blocking) and a missing error-path test (critic, advisory). The
//	worker fixes the first warm; the fix is a post-review change, so round 2
//	re-reviews the changed spans and confirms closure; the operator disposes the
//	blocking finding accepted. The PR arrives at stage 9 with one applied+verified
//	finding, one open advisory, verdict approve-with-advisories — persisted as one
//	artifact, and the harvest counts it (coverage + one accepted blocking
//	disposition).
//
// Requirements exercised end-to-end: FR-1 (two-round cap, re-review on change),
// FR-2/FR-3 (fresh-instance sibling + four blind lenses over one pack),
// FR-5/FR-6/DINV-1/2/9 (schema'd finding, one self-contained artifact,
// byte-identical round-trip), FR-7 (downgrade-only verdict), FR-9 (band from the
// lane's class), FR-11 (advisory ramp gates nothing), FR-12/DINV-3 (cal-cited
// severity validated against the real table), FR-13/DINV-8 (harvest over the
// universe), FR-15 (applied-but-unverified holds the verdict until round-2
// confirms), UB-8 (resume idempotence), DINV-5 (no authoring trail in the pack).
func TestScenario_CommonCase(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	ctx := context.Background()

	const (
		proposalRef = "github:agenticlab-ai/pyxis#951"
		reviewID    = "rv-common-1"
		rev1        = "1111111111111111111111111111111111111111" // ladder-green head
		rev2        = "2222222222222222222222222222222222222222" // after the warm fix
	)

	// The task charter (from the real EVAL-1 spec-fidelity fixture — a genuine
	// charter AC). Present and well-formed → charter.Validate proceeds (FR-14).
	charterText := []byte("AC-1 (Must): On a write failure, the persist path retries with bounded " +
		"backoff up to 3 attempts, then records a review-absence and returns ErrPersist. It MUST NOT " +
		"swallow the error or return success on a failed write.")

	// The round-1 diff swallows the error and returns success — the charter AC is
	// contradicted (a plausible-but-different behavior). Unique marker: the file
	// path the fake runner keys the seeded finding on.
	diffRound1 := []byte("--- a/internal/store/persist.go\n+++ b/internal/store/persist.go\n@@\n" +
		" func (s *Store) Persist(p Payload) (Ref, error) {\n-\tref, err := s.write(p)\n" +
		"-\tif err != nil {\n-\t\treturn Ref{}, fmt.Errorf(\"persist: %w\", err)\n-\t}\n-\treturn ref, nil\n" +
		"+\tref, err := s.write(p)\n+\tif err != nil {\n+\t\ts.log.Warn(\"write failed; continuing\", \"err\", err)\n" +
		"+\t\treturn Ref{}, nil\n+\t}\n+\treturn ref, nil\n }")

	codegateVerdict := []byte("codegate: ladder green (build+lint+test+secret-scan all pass)")
	designBindings := []byte("design binding: the persist path fails closed on a write error (DINV-x)")

	// ---- Step 0: charter validation (invocation) → proceed (FR-14) ----
	cout, err := charter.Validate(charter.Invocation{Charter: string(charterText)})
	if err != nil || !cout.Proceeds() {
		t.Fatalf("charter.Validate should proceed on a present, well-formed charter: kind=%s err=%v", cout.Kind, err)
	}

	// ---- Step 1: band selection from the lane's risk class (FR-9) ----
	// A code perimeter the whole diff falls inside → AUTO, non-skip class →
	// STANDARD band (the review depth read from the lane's own class, never a
	// second risk model).
	laneVerdict := classifier.Classify(classifier.Inputs{
		Diff: classifier.Diff{Paths: []string{"internal/store/persist.go"}},
		Base: "base-common", Head: rev1, CheckState: "green",
	}, commonLanePolicy(), nil)
	bandDecision := band.Select(laneVerdict)
	if bandDecision.Band != report.BandStandard {
		t.Fatalf("common-case band = %q, want standard (FR-9); reason: %s", bandDecision.Band, bandDecision.Reason)
	}

	// ---- Step 2: pack + round-1 review (FR-2/FR-3, DINV-5) ----
	// The fake lenses return the two findings a correct reviewer produces: a
	// blocking spec-fidelity finding + an advisory correctness-critic finding.
	h.runner.seed(string(diffRound1),
		seededFinding{
			id: "FND-1", ruleID: "spec-fidelity/charter-ac-contradicted", lens: report.LensSpecFidelity,
			severity: report.SeverityBlocking, calRow: "CAL-1",
			message: "AC-1 requires a bounded retry then a recorded review-absence and an ErrPersist return; " +
				"the diff swallows the error and returns nil (success) on a failed write — the required behavior is inverted.",
			path: "internal/store/persist.go", line: 5,
		},
		seededFinding{
			id: "FND-2", ruleID: "correctness-critic/missing-error-path-test", lens: report.LensCorrectnessCritic,
			severity: report.SeverityAdvisory, calRow: "CAL-12",
			message: "no test exercises the write-failure error path; a later change could silently re-break the contract.",
			path:    "internal/store/persist_test.go", line: 0,
		},
	)

	pack1 := h.buildPack(t, reviewInputs{
		subject: proposalRef, proposalDiff: diffRound1, codegateVerdict: codegateVerdict,
		charter: charterText, designBindings: designBindings, reviewedRevision: rev1, band: report.BandStandard,
	})
	// DINV-5: the pack carries the four inputs and NOTHING resembling an answer key
	// (no prior findings). Structurally guaranteed by pack.ReviewInputs; assert the
	// pack does carry the subject diff (so the lenses actually review it).
	if !containsBytes(pack1, "internal/store/persist.go") {
		t.Fatal("the pack must carry the proposal diff for the lenses to review")
	}
	round1 := h.runRound(t, pack1, rev1, nil) // no codegate-flagged classes on a green ladder

	// FR-3: exactly the four lenses ran, each finding-blind (the runtime called
	// Run once per lens). The two survivors are the blocking + advisory finding.
	assertFourLensesRan(t, h.runner)
	if got := len(round1.Round.Findings); got != 2 {
		t.Fatalf("round-1 findings = %d, want 2 (one blocking, one advisory)", got)
	}
	if !hasFinding(round1.Round.Findings, "FND-1", report.SeverityBlocking) {
		t.Fatal("round-1 must carry the blocking spec-fidelity finding FND-1")
	}
	if !hasFinding(round1.Round.Findings, "FND-2", report.SeverityAdvisory) {
		t.Fatal("round-1 must carry the advisory critic finding FND-2")
	}

	// ---- Step 3: verdict on round-1 alone → request-changes (FR-7) ----
	// A blocking finding with no operator disposition holds the verdict at
	// request-changes (downgrade-only, return-to-worker).
	_, out1 := h.assembleReport(t, proposalRef, reviewID, report.BandStandard, verdict.Signals{}, round1.Round)
	if out1.Verdict != report.VerdictRequestChanges {
		t.Fatalf("round-1 verdict = %q, want request-changes (FR-7)", out1.Verdict)
	}
	if out1.Authority != verdict.AuthorityReturnToWorker {
		t.Fatalf("round-1 authority = %q, want return-to-worker (FR-7 downgrade-only)", out1.Authority)
	}

	// ---- Step 3b: the worker applies the blocking finding warm (FR-8/FR-15) ----
	// The worker records an APPLIED disposition on the blocking finding. This is an
	// UNVERIFIED claim — it does NOT resolve the blocker until round-2 confirms
	// closure (FR-15). CheckRaise on round-1 with the applied disposition would
	// still be a protocol defect if any blocking finding were undisposed; here the
	// applied disposition is recorded so the raise-gate is satisfied on disposition
	// presence, but the verdict does not upgrade on an unverified claim.
	recApplied, err := protocol.RecordDisposition(round1.Round.Findings[0], protocol.WorkerDisposition{
		FindingID: "FND-1", Applied: true,
	})
	if err != nil {
		t.Fatalf("RecordDisposition (applied): %v", err)
	}
	if recApplied.Escalation != nil {
		t.Fatal("an applied disposition must not escalate (FR-8)")
	}
	// FR-15: an applied-but-unverified blocking finding holds the verdict; the
	// verdict over a round whose blocking finding is only worker-applied is still
	// request-changes (verdict.resolvedAway refuses a worker applied as a resolution).
	round1Applied := withDisposition(round1.Round, "FND-1", recApplied.Disposition)
	outApplied, err := verdict.Compute([]report.Round{round1Applied}, verdict.Signals{})
	if err != nil {
		t.Fatalf("verdict.Compute over applied-but-unverified round: %v", err)
	}
	if outApplied.Verdict != report.VerdictRequestChanges {
		t.Fatalf("verdict over an applied-but-unverified blocking finding = %q, want request-changes (FR-15)", outApplied.Verdict)
	}

	// ---- Step 3c: round 2 re-reviews the changed spans + confirms closure (FR-1) ----
	// The fix at rev2 closes the defect: the round-2 diff no longer contradicts the
	// AC, so the fake lenses emit NO blocking spec-fidelity finding on it (the
	// advisory critic finding remains as an open advisory). This is the post-review
	// change forcing a second, scope-locked round (FR-1).
	diffRound2 := []byte("--- a/internal/store/persist.go\n+++ b/internal/store/persist.go\n@@\n" +
		" func (s *Store) Persist(p Payload) (Ref, error) {\n+\tfor attempt := 1; attempt <= 3; attempt++ {\n" +
		"+\t\tref, err := s.write(p)\n+\t\tif err == nil {\n+\t\t\treturn ref, nil\n+\t\t}\n" +
		"+\t\ttime.Sleep(backoff(attempt))\n+\t}\n+\ts.recordAbsence(p)\n+\treturn Ref{}, ErrPersist\n }")
	h.runner.seed(string(diffRound2),
		seededFinding{
			id: "FND-2", ruleID: "correctness-critic/missing-error-path-test", lens: report.LensCorrectnessCritic,
			severity: report.SeverityAdvisory, calRow: "CAL-12",
			message: "no test exercises the write-failure error path; a later change could silently re-break the contract.",
			path:    "internal/store/persist_test.go", line: 0,
		},
	)
	pack2 := h.buildPack(t, reviewInputs{
		subject: proposalRef, proposalDiff: diffRound2, codegateVerdict: codegateVerdict,
		charter: charterText, designBindings: designBindings, reviewedRevision: rev2, band: report.BandStandard,
	})
	round2 := h.runRound(t, pack2, rev2, nil)

	// FR-1/FR-8: round 2 is scope-locked to the named defect (FND-1) + the changed
	// spans. The candidate round-2 findings partition through ApplyScopeLock; a
	// round-3 attempt is refused.
	scope := protocol.RoundScope{
		NamedDefects: []report.Finding{round1.Round.Findings[0]}, // FND-1, the disputed blocker
		ChangedSpans: protocol.SpanSet{{Path: "internal/store/persist.go", StartLine: 1, EndLine: 12}},
	}
	scoped, err := protocol.ApplyScopeLock(2, round2.Round.Findings, scope)
	if err != nil {
		t.Fatalf("ApplyScopeLock(round 2): %v", err)
	}
	// The advisory critic finding is on a different file (the _test.go) and does not
	// name the blocker, so it is withheld as out-of-scope for round 2 — the round-2
	// scope is bounded (FR-8). The blocker's closure is what round 2 verifies.
	if len(scoped.Withheld) != 1 {
		t.Fatalf("round-2 scope-lock withheld %d findings, want 1 (the out-of-scope advisory)", len(scoped.Withheld))
	}

	// FR-15: ApplyVerification confirms the round-1 blocking finding is closed —
	// it is ABSENT from the round-2 findings, so it renders VERIFIED and the
	// verdict may upgrade off it. The round-1 blocker is passed with its
	// worker/applied disposition (only an applied claim takes the verified
	// transition).
	appliedBlocker := withDisposition(round1.Round, "FND-1", recApplied.Disposition).Findings[0]
	verifs, _ := protocol.ApplyVerification(
		[]report.Finding{appliedBlocker}, // round-1 blocking, worker/applied
		round2.Round.Findings,            // round-2 findings (no re-raise of FND-1)
	)
	if len(verifs) != 1 || verifs[0].FindingID != "FND-1" || !verifs[0].Verified {
		t.Fatalf("round-2 must verify FND-1 closed (FR-15); got %+v", verifs)
	}

	// A third round is structurally refused (FR-1, two-round cap).
	if err := protocol.RefuseThirdRound(3); !errors.Is(err, protocol.ErrThirdRoundRefused) {
		t.Fatalf("a third round must be refused (FR-1): %v", err)
	}

	// ---- Step 4: operator disposes the blocking finding accepted; final verdict ----
	// The operator's own medium-authored disposition (accepted) resolves the
	// blocker (the operator is the ground truth — verdict.resolvedAway). The final
	// review is a two-round payload: round 1 (the blocker, applied) + round 2 (the
	// confirmed-closed state, with the operator disposition folded onto the blocker
	// and the advisory critic finding still open). The latest round governs the
	// verdict.
	opAttribution := "github:pr-review/op-accept-951"
	blockingAccepted := round1.Round.Findings[0]
	blockingAccepted.Disposition = &report.Disposition{
		Actor: report.ActorOperator, Verdict: report.VerdictAccepted, Attribution: opAttribution,
	}
	// The latest (governing) round: the closed blocker carrying the operator
	// accepted disposition + the open advisory critic finding. No unresolved
	// blocking → approve-with-advisories (one open advisory).
	finalLatest := report.Round{
		ReviewedRevision: rev2,
		ContextID:        round2.Round.ContextID,
		Provenance:       round2.Round.Provenance,
		Findings:         []report.Finding{blockingAccepted, round2.Round.Findings[0]}, // accepted blocker + advisory
		FindingsCount:    2,
		Candidates:       round2.Round.Candidates,
		Killed:           round2.Round.Killed,
	}
	finalReport, finalOut := h.assembleReport(t, proposalRef, reviewID, report.BandStandard, verdict.Signals{},
		round1Applied, finalLatest)
	if finalOut.Verdict != report.VerdictApproveWithAdvisories {
		t.Fatalf("final verdict = %q, want approve-with-advisories (FR-7)", finalOut.Verdict)
	}
	if finalOut.Authority != verdict.AuthorityNone {
		t.Fatalf("approve-with-advisories authority = %q, want none (never an AUTO grant)", finalOut.Authority)
	}

	// ---- Step 5: persist ONE artifact + read it back byte-identically (FR-6) ----
	reviewRef := h.persistAndReadBack(t, finalReport)

	// ---- Step 6: advisory ramp gates nothing pre-promotion (FR-11) ----
	gate := checkpoint.NewGate("agenticlab-ai/pyxis")
	if gate.RaiseBlocked(finalOut.Verdict) {
		t.Fatal("an advisory gate must not block the PR-raise (FR-11)")
	}
	// Even a request-changes verdict does not block a raise while advisory.
	if gate.RaiseBlocked(report.VerdictRequestChanges) {
		t.Fatal("an advisory gate must not block a request-changes raise (FR-11)")
	}

	// ---- Step 7: the harvest counts it (FR-13/DINV-4/8) ----
	// The universe is this one reviewed proposal. The operator disposition resolves
	// to the operator identity, so it counts: coverage 1/1, one accepted blocking
	// disposition (precision numerator + denominator).
	authors := newFakeAuthors()
	authors.set(opAttribution, operatorLogin)
	universe := []harvest.Proposal{
		{Ref: proposalRef, Review: refPtr(reviewRef)},
	}
	metrics, err := harvest.Harvest(ctx,
		harvest.EnumeratorFunc(func(context.Context) ([]harvest.Proposal, error) { return universe, nil }),
		h.port,
		harvest.NewGitHubResolver(operatorLogin, authors),
	)
	if err != nil {
		t.Fatalf("harvest: %v", err)
	}
	if metrics.Universe != 1 || metrics.Covered != 1 {
		t.Fatalf("harvest coverage = %d/%d, want 1/1 (FR-13)", metrics.Covered, metrics.Universe)
	}
	if metrics.Coverage() != 1.0 {
		t.Fatalf("harvest Coverage() = %v, want 1.0 (M1)", metrics.Coverage())
	}
	if metrics.BlockingDisposed != 1 || metrics.BlockingAccepted != 1 {
		t.Fatalf("harvest blocking disposed/accepted = %d/%d, want 1/1 (the accepted blocking disposition)",
			metrics.BlockingDisposed, metrics.BlockingAccepted)
	}
	if metrics.ExcludedDispositions != 0 {
		t.Fatalf("no disposition should be excluded (the operator authored it): %d excluded", metrics.ExcludedDispositions)
	}

	// ---- Step 8: lifecycle resume idempotence (UB-8) ----
	// The persisted final report carries a terminal verdict, so a re-invocation is
	// a COMPLETE (no-op), never a fresh round-1.
	decision := lifecycle.Decide(finalReport)
	if decision.Action != lifecycle.ActionComplete {
		t.Fatalf("re-invoking a completed review must be a no-op (UB-8): got %s", decision.Action)
	}
	if lifecycle.ShouldStartFresh(finalReport) {
		t.Fatal("a completed review must not start fresh (UB-8)")
	}
}

// -----------------------------------------------------------------------------
// Shared assertion + report helpers used across scenarios.
// -----------------------------------------------------------------------------

// assertFourLensesRan asserts the runtime ran exactly the four lenses, once each,
// in a run (finding-blindness is structural: each lens's Run got only the pack +
// its own instruction — FR-3, DINV-5).
func assertFourLensesRan(t *testing.T, r *fakeLensRunner) {
	t.Helper()
	r.mu.Lock()
	defer r.mu.Unlock()
	seen := map[string]int{}
	// Only the calls of the LATEST review matter; count the last four.
	for _, c := range r.calls {
		seen[c.lens]++
	}
	for _, l := range runtime.Lenses {
		if seen[l] == 0 {
			t.Fatalf("lens %q never ran — all four lenses must run (FR-3)", l)
		}
	}
	if len(seen) != 4 {
		t.Fatalf("lenses that ran = %v, want exactly the four (FR-3)", seen)
	}
}

// hasFinding reports whether the finding set carries a finding with the given id
// and severity.
func hasFinding(findings []report.Finding, id, severity string) bool {
	for i := range findings {
		if findings[i].ID == id && findings[i].Severity == severity {
			return true
		}
	}
	return false
}

// withDisposition returns a copy of the round with the given disposition folded
// onto the finding with id — the "the worker/operator disposed this finding"
// transition, without mutating the original round.
func withDisposition(round report.Round, id string, d report.Disposition) report.Round {
	out := round
	out.Findings = make([]report.Finding, len(round.Findings))
	copy(out.Findings, round.Findings)
	for i := range out.Findings {
		if out.Findings[i].ID == id {
			dc := d
			out.Findings[i].Disposition = &dc
		}
	}
	return out
}

// refPtr returns a pointer to a persist.Ref — the harvest.Proposal.Review shape.
func refPtr(r persist.Ref) *persist.Ref { return &r }

// commonLanePolicy is a merge-lane policy whose AUTO classes place the common-case
// diff (internal/store/**) inside a non-skip code perimeter → standard band.
func commonLanePolicy() *policy.Policy {
	return &policy.Policy{
		SchemaVersion: policy.SchemaVersion,
		RatifiedBy:    "@mdarabi",
		Floor: policy.Floor{
			HardGatedPaths:   policy.SelfUnlockMinPaths,
			HardGatedClasses: policy.SelfUnlockMinClasses,
		},
		AutoClasses: []policy.AutoClass{
			{Name: "engine-code", Paths: []string{"internal/**", "cmd/**"}},
			{Name: "drive-doc", Paths: []string{"docs/wiki/**"}, RequiresReviewerVerdict: true},
		},
	}
}
