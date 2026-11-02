package e2e

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/implreview/band"
	"github.com/agenticlab-ai/pyxis/internal/implreview/charter"
	"github.com/agenticlab-ai/pyxis/internal/implreview/checkpoint"
	"github.com/agenticlab-ai/pyxis/internal/implreview/harvest"
	"github.com/agenticlab-ai/pyxis/internal/implreview/lifecycle"
	"github.com/agenticlab-ai/pyxis/internal/implreview/persist"
	"github.com/agenticlab-ai/pyxis/internal/implreview/report"
	"github.com/agenticlab-ai/pyxis/internal/implreview/runtime"
	"github.com/agenticlab-ai/pyxis/internal/lane/classifier"
)

// errLensBackendDown is the lens-backend failure the reviewer-is-down scenario
// injects — the runner returns it past the bounded retry.
var errLensBackendDown = errors.New("lens backend error: model provider 503 (past retry)")

// TestScenario_ReviewerDown drives the PRD §4.4 "reviewer is down" end to end:
//
//	The lens backend errors past its bound. After bounded retry the review records
//	`review-absent` on the proposal — visible to stage 9, never a silent pass — and
//	the host proceeds (advisory) with the absence flagged; post-adoption the
//	proposal routes HUMAN.
//
// Requirements exercised end-to-end: UB-1 (a lens-backend error past the bounded
// retry records a review-absent — NEVER a silent pass, NEVER an indefinite stall),
// FR-2/FR-3 (the real runtime.Review is what errors — the failure rides the real
// orchestration), FR-11 (advisory: the host proceeds unblocked), FR-13/DINV-8 (the
// harvest counts the absence AGAINST coverage — the absence is visible but is NOT a
// review), and the post-adoption HUMAN routing.
func TestScenario_ReviewerDown(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	ctx := context.Background()

	const (
		proposalRef = "github:agenticlab-ai/pyxis#996"
		rev1        = "eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee"
	)

	charterText := []byte("AC-1 (Must): the migration is idempotent — re-running it is a no-op.")
	codegateVerdict := []byte("codegate: ladder green")
	designBindings := []byte("design binding: migrations are idempotent")
	diff := []byte("--- a/internal/migrate/run.go\n+++ b/internal/migrate/run.go\n@@\n+\t// migration body\n")

	// charter proceeds (FR-14).
	cout, err := charter.Validate(charter.Invocation{Charter: string(charterText)})
	if err != nil || !cout.Proceeds() {
		t.Fatalf("charter.Validate should proceed: kind=%s err=%v", cout.Kind, err)
	}

	// Band is standard.
	laneVerdict := classifier.Classify(classifier.Inputs{
		Diff: classifier.Diff{Paths: []string{"internal/migrate/run.go"}},
		Base: "base-down", Head: rev1, CheckState: "green",
	}, commonLanePolicy(), nil)
	if got := band.Select(laneVerdict).Band; got != report.BandStandard {
		t.Fatalf("band = %q, want standard (FR-9)", got)
	}

	// The lens backend is DOWN: every Run errors (past the bounded retry).
	h.runner.failWith(errLensBackendDown)

	// The review pack is built normally (the pack build does not touch the LLM).
	packBytes := h.buildPack(t, reviewInputs{
		subject: proposalRef, proposalDiff: diff, codegateVerdict: codegateVerdict,
		charter: charterText, designBindings: designBindings, reviewedRevision: rev1, band: report.BandStandard,
	})

	// ---- The reviewer attempt runs the REAL runtime.Review and errors ----
	// A direct runtime.Review over the erroring runner surfaces the lens error —
	// never a silent pass (the runtime returns the error, it does not swallow it).
	if _, rerr := runtime.Review(ctx, runtime.ReviewRequest{
		ReviewerRoot:     h.reviewerRoot,
		ImplementerRoot:  h.implementerRoot,
		Pack:             packBytes,
		ReviewedRevision: rev1,
		Instructions:     h.instr,
		Provenance:       report.Provenance{Model: "fake", PromptsVersion: "impl-review-lenses/v1", CalVersion: "impl-review-calibration/v1"},
		Runner:           h.runner,
	}); rerr == nil {
		t.Fatal("runtime.Review over a down lens backend must return an error, never a silent pass (UB-1)")
	}

	// ---- UB-1: bounded retry, then record `review-absent` (never a hang) ----
	// The lifecycle runs the reviewer attempt under a BOUNDED retry (the bound is
	// the attempt count, so there is no clock wait and no indefinite stall) and,
	// past the bound, records the absence through the injected recorder. Each
	// attempt is a FRESH runtime.Review — the real orchestration, failing every time.
	var attempts int
	attempt := func(n int) error {
		attempts = n
		_, err := runtime.Review(ctx, runtime.ReviewRequest{
			ReviewerRoot:     h.reviewerRoot,
			ImplementerRoot:  h.implementerRoot,
			Pack:             packBytes,
			ReviewedRevision: rev1,
			Instructions:     h.instr,
			Provenance:       report.Provenance{Model: "fake", PromptsVersion: "impl-review-lenses/v1", CalVersion: "impl-review-calibration/v1"},
			Runner:           h.runner,
		})
		return err
	}

	// The recorder writes a `review-absent` marker to the proposal's medium (the
	// alternate signal) — the fake code-host records it in-memory, returning a ref
	// so the absence is visible and harvestable.
	var absenceRef persist.Ref
	recorder := lifecycle.AbsenceRecorderFunc(func(proposal string, cause error) (persist.Ref, error) {
		absenceRef = persist.Ref{Medium: persist.MediumGitHub, Locator: "impl-review-absent-996"}
		return absenceRef, nil
	})

	cfg := lifecycle.Config{AbsentTimeout: 30 * time.Minute, MaxAttempts: 3}
	absErr := lifecycle.RecordAbsence(proposalRef, attempt, recorder, cfg)

	// UB-1: NEVER nil on failure (never a silent pass).
	if absErr == nil {
		t.Fatal("a reviewer down past its bound must NOT return nil — that is the silent pass UB-1 forbids")
	}
	// It is the recorded-absence outcome (distinguishable, harvestable).
	if !lifecycle.IsRecordedAbsence(absErr) {
		t.Fatalf("the outcome must be a recorded review-absence (UB-1): %v", absErr)
	}
	var are *persist.AbsenceRecordedError
	if !errors.As(absErr, &are) {
		t.Fatalf("the error must be a *persist.AbsenceRecordedError: %v", absErr)
	}
	if !are.Recorded {
		t.Fatal("the absence must be recorded on the medium (never a silent loss, UB-1)")
	}
	if are.Attempts != cfg.MaxAttempts {
		t.Fatalf("the retry must be bounded at %d attempts; got %d (never an indefinite stall)", cfg.MaxAttempts, are.Attempts)
	}
	if attempts != cfg.MaxAttempts {
		t.Fatalf("the real runtime.Review must have run every bounded attempt: ran %d, want %d", attempts, cfg.MaxAttempts)
	}

	// ---- FR-11: the host proceeds ADVISORY (unblocked) despite the absence ----
	// In advisory, a review-absence does not block the raise (the host proceeds
	// flagged). The gate gates nothing while advisory — including on an absence.
	advisoryGate := checkpoint.NewGate("agenticlab-ai/pyxis")
	// (A review-absence carries no verdict; the advisory gate blocks nothing at all.)
	if advisoryGate.RaiseBlocked(report.VerdictRequestChanges) || advisoryGate.RaiseBlocked(report.VerdictEscalate) {
		t.Fatal("an advisory gate must not block a raise — the host proceeds flagged (FR-11)")
	}

	// ---- FR-13/DINV-8: the harvest counts the absence AGAINST coverage ----
	// The proposal carries a recorded review-absence (Absent), not a review, so it
	// is UNCOVERED and a miss (the harvest is the only source that sees a
	// never-reviewed / absent proposal). A second, genuinely-reviewed proposal is
	// added so coverage is a proper fraction the absence lowers.
	otherRef := h.persistAndReadBack(t, blockingAcceptedReview("github:agenticlab-ai/pyxis#997", "rv-down-other", "github:pr-review/op-997"))
	authors := newFakeAuthors()
	authors.set("github:pr-review/op-997", operatorLogin)

	universe := []harvest.Proposal{
		{Ref: proposalRef, Review: nil, Absent: true, AbsentReason: are.Cause.Error()}, // the review-absent proposal
		{Ref: "github:agenticlab-ai/pyxis#997", Review: refPtr(otherRef)},              // a genuinely reviewed one
	}
	metrics, err := harvest.Harvest(ctx,
		harvest.EnumeratorFunc(func(context.Context) ([]harvest.Proposal, error) { return universe, nil }),
		h.port,
		harvest.NewGitHubResolver(operatorLogin, authors),
	)
	if err != nil {
		t.Fatalf("harvest: %v", err)
	}
	if metrics.Universe != 2 {
		t.Fatalf("universe = %d, want 2 (both proposals enumerated, incl. the absent one)", metrics.Universe)
	}
	if metrics.Covered != 1 {
		t.Fatalf("covered = %d, want 1 — the review-absent proposal is UNCOVERED (FR-13/DINV-8)", metrics.Covered)
	}
	if metrics.Coverage() != 0.5 {
		t.Fatalf("coverage = %v, want 0.5 — the absence lowers coverage (M1)", metrics.Coverage())
	}
	// The absence is in the miss corpus, classified as a review-absent miss, with
	// its cause recorded (self-describing, DINV-8) — never silently dropped.
	if len(metrics.Misses) != 1 {
		t.Fatalf("miss corpus size = %d, want 1 (the review-absent proposal)", len(metrics.Misses))
	}
	miss := metrics.Misses[0]
	if miss.Kind != harvest.MissAbsent {
		t.Fatalf("miss kind = %q, want %q (a recorded absence, not a never-reviewed one)", miss.Kind, harvest.MissAbsent)
	}
	if miss.Proposal != proposalRef || miss.Cause == "" {
		t.Fatalf("the miss must name the proposal and carry the absence cause (DINV-8): %+v", miss)
	}

	// ---- Post-adoption: the absent proposal routes HUMAN ----
	// Once the ramp is adopted, the host routes an absent/blocked proposal to the
	// HUMAN lane rather than proceeding. The gate's post-adoption behavior on a
	// downgrade verdict (the host treats a review-absence as a downgrade to HUMAN
	// post-adoption — §3.10 recovery) is proven via the adopted gate blocking the
	// raise; the advisory gate above did not.
	adoptedGate := promoteGate(t, "agenticlab-ai/pyxis")
	if !adoptedGate.RaiseBlocked(report.VerdictEscalate) {
		t.Fatal("post-adoption the host must route an absent/downgraded proposal HUMAN (block the raise) — §3.10 recovery")
	}
}
