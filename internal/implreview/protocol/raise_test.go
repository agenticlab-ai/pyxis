package protocol

import (
	"errors"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/implreview/report"
)

func workerApplied() *report.Disposition {
	return &report.Disposition{Actor: report.ActorWorker, Verdict: report.VerdictApplied}
}
func opAccepted() *report.Disposition {
	return &report.Disposition{Actor: report.ActorOperator, Verdict: report.VerdictAccepted}
}

// TestCheckRaiseUndisposedBlockingIsDefect — a raise carrying a blocking finding
// with no disposition is a protocol defect; an advisory needs none (FR-15).
func TestCheckRaiseUndisposedBlockingIsDefect(t *testing.T) {
	t.Run("undisposed blocking → ErrUndisposedBlocking", func(t *testing.T) {
		f := blockingF("FND-1", "r1", "a.go", 5) // no disposition
		if err := CheckRaise([]report.Finding{f}); !errors.Is(err, ErrUndisposedBlocking) {
			t.Fatalf("CheckRaise = %v, want ErrUndisposedBlocking", err)
		}
		un := UndisposedBlocking([]report.Finding{f})
		if len(un) != 1 || un[0].ID != "FND-1" {
			t.Errorf("UndisposedBlocking = %+v, want [FND-1]", un)
		}
	})

	t.Run("every blocking disposed → no defect", func(t *testing.T) {
		a := blockingF("FND-1", "r1", "a.go", 5)
		a.Disposition = workerApplied()
		b := blockingF("FND-2", "r2", "b.go", 7)
		b.Disposition = opAccepted()
		adv := advisoryF("FND-3", "r3") // advisory needs no disposition
		if err := CheckRaise([]report.Finding{a, b, adv}); err != nil {
			t.Errorf("CheckRaise on a fully-disposed raise: %v", err)
		}
		if len(UndisposedBlocking([]report.Finding{a, b, adv})) != 0 {
			t.Errorf("expected no undisposed blockers")
		}
	})

	t.Run("advisory with no disposition is not a defect", func(t *testing.T) {
		if err := CheckRaise([]report.Finding{advisoryF("FND-1", "r1")}); err != nil {
			t.Errorf("an undisposed advisory must not block a raise: %v", err)
		}
	})
}

// TestAppliedUnverifiedHoldsRequestChanges — an applied blocking finding renders
// VERIFIED only after a round-2 re-review confirms the named defect closed;
// otherwise it is an UNVERIFIED claim that HOLDS the verdict at request-changes
// (FR-15). Verified via the round-2 finding set verdict.Compute reads.
func TestAppliedUnverifiedHoldsRequestChanges(t *testing.T) {
	// Round-1 blocking finding, worker claims applied.
	r1 := blockingF("FND-1", "spec-fidelity/R1", "internal/a/a.go", 10)
	r1.Disposition = workerApplied()

	t.Run("round 2 re-raises the defect → UNVERIFIED, holds request-changes", func(t *testing.T) {
		// Round 2 re-raised the SAME named defect (by rule_id) → not closed.
		reRaised := blockingF("FND-1b", "spec-fidelity/R1", "internal/a/a.go", 10)
		results, round2Set := ApplyVerification([]report.Finding{r1}, []report.Finding{reRaised})

		if len(results) != 1 || results[0].Verified {
			t.Fatalf("results = %+v, want one UNVERIFIED result", results)
		}
		// The round-2 set carries the re-raised blocker → verdict.Compute holds.
		v, err := computeWith([]report.Round{roundOf("sha-2", round2Set...)}, nil)
		if err != nil {
			t.Fatalf("computeWith: %v", err)
		}
		if v != report.VerdictRequestChanges {
			t.Errorf("verdict = %q, want request-changes (an unverified applied claim holds)", v)
		}
	})

	t.Run("round 2 does NOT re-raise the defect → VERIFIED, no longer holds", func(t *testing.T) {
		// Round 2 found nothing about the named defect → the applied fix closed it.
		results, round2Set := ApplyVerification([]report.Finding{r1}, nil)

		if len(results) != 1 || !results[0].Verified {
			t.Fatalf("results = %+v, want one VERIFIED result", results)
		}
		if len(round2Set) != 0 {
			t.Errorf("a verified applied defect must be dropped from the round-2 set; got %+v", round2Set)
		}
		// verdict.Compute over the (empty) round-2 set → approve, the blocker no
		// longer holds.
		v, err := computeWith([]report.Round{roundOf("sha-2", round2Set...)}, nil)
		if err != nil {
			t.Fatalf("computeWith: %v", err)
		}
		if v != report.VerdictApprove {
			t.Errorf("verdict = %q, want approve (the verified defect no longer holds)", v)
		}
	})

	t.Run("no round-2 re-review at all → UNVERIFIED (an applied claim is unverified until confirmed)", func(t *testing.T) {
		// A raise attempted on the round-1 applied claim, with round 2 not yet run,
		// must NOT be treated as verified: verdict.Compute over round 1's applied
		// blocker still holds (verdict.resolvedAway refuses a worker applied to
		// resolve). This is the "applied-but-unverified holds" guarantee at the
		// verdict layer this package never contradicts.
		v, err := computeWith([]report.Round{roundOf("sha-1", r1)}, nil)
		if err != nil {
			t.Fatalf("computeWith: %v", err)
		}
		if v != report.VerdictRequestChanges {
			t.Errorf("verdict = %q, want request-changes — an applied claim is unverified until a round-2 re-review confirms it", v)
		}
	})
}

// TestVerificationCarriesFreshRound2Findings — a fresh, in-scope round-2 finding
// (not a re-raise of any named defect) is carried through the transition
// unchanged, alongside the verified/unverified named defects.
func TestVerificationCarriesFreshRound2Findings(t *testing.T) {
	// Two round-1 applied blockers.
	closed := blockingF("FND-1", "spec-fidelity/R1", "a.go", 10)
	closed.Disposition = workerApplied()
	stillOpen := blockingF("FND-2", "design-conformance/R2", "b.go", 20)
	stillOpen.Disposition = workerApplied()

	// Round 2: re-raises R2 (still open), and a FRESH finding on a changed span.
	reRaisedR2 := blockingF("FND-2b", "design-conformance/R2", "b.go", 20)
	fresh := blockingF("FND-7", "security-abuse/R7", "b.go", 22) // new, in-scope

	results, round2Set := ApplyVerification([]report.Finding{closed, stillOpen}, []report.Finding{reRaisedR2, fresh})

	// FND-1 verified (not re-raised), FND-2 unverified (re-raised).
	var verifiedFND1, unverifiedFND2 bool
	for _, r := range results {
		if r.FindingID == "FND-1" && r.Verified {
			verifiedFND1 = true
		}
		if r.FindingID == "FND-2" && !r.Verified {
			unverifiedFND2 = true
		}
	}
	if !verifiedFND1 || !unverifiedFND2 {
		t.Fatalf("results = %+v, want FND-1 verified and FND-2 unverified", results)
	}

	// The round-2 set must carry the re-raised R2 blocker AND the fresh finding,
	// and NOT a phantom for the verified FND-1.
	ids := map[string]bool{}
	for _, f := range round2Set {
		ids[f.ID] = true
	}
	if !ids["FND-2b"] || !ids["FND-7"] {
		t.Errorf("round-2 set missing the re-raised blocker or the fresh finding: %+v", round2Set)
	}
	if ids["FND-1"] {
		t.Errorf("a verified defect must not appear in the round-2 set: %+v", round2Set)
	}
	// verdict.Compute holds at request-changes (the still-open R2 blocker).
	v, err := computeWith([]report.Round{roundOf("sha-2", round2Set...)}, nil)
	if err != nil {
		t.Fatalf("computeWith: %v", err)
	}
	if v != report.VerdictRequestChanges {
		t.Errorf("verdict = %q, want request-changes", v)
	}
}

// TestOperatorAcceptedBlockerNotInAppliedTransition — an operator-accepted
// blocker is resolved by verdict.resolvedAway, not by the applied-verified
// transition, so ApplyVerification does not touch it (it never carries a
// non-applied round-1 finding into the round-2 set).
func TestOperatorAcceptedBlockerNotInAppliedTransition(t *testing.T) {
	accepted := blockingF("FND-1", "spec-fidelity/R1", "a.go", 10)
	accepted.Disposition = opAccepted()
	results, round2Set := ApplyVerification([]report.Finding{accepted}, nil)
	if len(results) != 0 {
		t.Errorf("an operator-accepted blocker is not part of the applied transition, got %+v", results)
	}
	if len(round2Set) != 0 {
		t.Errorf("round-2 set should be empty (no applied re-raise, no fresh findings), got %+v", round2Set)
	}
}
