package protocol

import (
	"errors"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/implreview/report"
)

// blocking builds a blocking finding with the given id/rule/location.
func blockingF(id, rule, path string, line int) report.Finding {
	return report.Finding{
		ID:       id,
		RuleID:   rule,
		Path:     path,
		Line:     line,
		Message:  "m",
		CalRow:   "CAL-1",
		Lens:     report.LensSpecFidelity,
		Severity: report.SeverityBlocking,
	}
}

// advisoryF builds an advisory finding.
func advisoryF(id, rule string) report.Finding {
	return report.Finding{
		ID:       id,
		RuleID:   rule,
		Message:  "m",
		CalRow:   "CAL-2",
		Lens:     report.LensSpecFidelity,
		Severity: report.SeverityAdvisory,
	}
}

func goodBrief() report.EscalationBrief {
	return report.EscalationBrief{
		Conflict:       "the fix is a spec decision, not a code change",
		Options:        []string{"ratify the current behavior and amend the charter", "change the behavior to match the charter"},
		DecisionNeeded: "which behavior the charter intends",
	}
}

// TestRecordDispositionApplied — a worker/applied claim yields a worker/applied
// disposition and NO escalation (an unverified claim that holds the verdict).
func TestRecordDispositionApplied(t *testing.T) {
	f := blockingF("FND-1", "spec-fidelity/R1", "internal/a/a.go", 10)
	res, err := RecordDisposition(f, WorkerDisposition{FindingID: "FND-1", Applied: true})
	if err != nil {
		t.Fatalf("RecordDisposition(applied): %v", err)
	}
	if res.Disposition.Actor != report.ActorWorker || res.Disposition.Verdict != report.VerdictApplied {
		t.Errorf("disposition = %+v, want worker/applied", res.Disposition)
	}
	if res.Escalation != nil {
		t.Errorf("an applied claim must not escalate, got %+v", res.Escalation)
	}
}

// TestReasonlessDisputeVoidsRound — a dispute with no reason is refused with
// ErrDisputeNoReason, and VoidRound reports the round is void (UB-2, FR-8).
func TestReasonlessDisputeVoidsRound(t *testing.T) {
	f := blockingF("FND-1", "spec-fidelity/R1", "internal/a/a.go", 10)

	_, err := RecordDisposition(f, WorkerDisposition{FindingID: "FND-1", Applied: false, Reason: ""})
	if !errors.Is(err, ErrDisputeNoReason) {
		t.Fatalf("err = %v, want ErrDisputeNoReason", err)
	}
	if !VoidRound(err) {
		t.Errorf("VoidRound(ErrDisputeNoReason) = false, want true — a reason-less dispute voids its round")
	}

	// A whitespace-only reason is still reason-less (voids the round).
	_, err = RecordDisposition(f, WorkerDisposition{FindingID: "FND-1", Applied: false, Reason: "   "})
	if !errors.Is(err, ErrDisputeNoReason) {
		t.Fatalf("whitespace reason: err = %v, want ErrDisputeNoReason", err)
	}

	// An unrelated error is NOT a void condition.
	if VoidRound(errors.New("some other failure")) {
		t.Errorf("VoidRound(other) = true, want false — only a reason-less dispute voids")
	}
}

// TestDeclinedBlockingEscalates — a worker DISPUTE of a BLOCKING finding produces
// the escalate signal (FR-8 → FR-17): a worker/declined disposition plus a
// verdict.Escalation the caller feeds to verdict.Compute. An advisory dispute
// escalates nothing.
func TestDeclinedBlockingEscalates(t *testing.T) {
	t.Run("disputed blocking → escalate signal", func(t *testing.T) {
		f := blockingF("FND-1", "spec-fidelity/R1", "internal/a/a.go", 10)
		res, err := RecordDisposition(f, WorkerDisposition{
			FindingID:       "FND-1",
			Applied:         false,
			Reason:          "the finding misreads the charter; behavior A is intended",
			EscalationBrief: goodBrief(),
		})
		if err != nil {
			t.Fatalf("RecordDisposition(disputed blocking): %v", err)
		}
		if res.Disposition.Actor != report.ActorWorker || res.Disposition.Verdict != report.VerdictDeclined {
			t.Errorf("disposition = %+v, want worker/declined", res.Disposition)
		}
		if res.Disposition.Reason == "" {
			t.Errorf("a declined disposition must carry its reason")
		}
		if res.Escalation == nil {
			t.Fatalf("a disputed blocking finding must escalate — got nil Escalation")
		}
		if res.Escalation.Brief.Conflict != goodBrief().Conflict {
			t.Errorf("escalation brief not carried: %+v", res.Escalation.Brief)
		}
	})

	t.Run("disputed advisory → no escalation", func(t *testing.T) {
		f := advisoryF("FND-9", "spec-fidelity/R9")
		res, err := RecordDisposition(f, WorkerDisposition{
			FindingID: "FND-9",
			Applied:   false,
			Reason:    "cosmetic; not a real issue",
		})
		if err != nil {
			t.Fatalf("RecordDisposition(disputed advisory): %v", err)
		}
		if res.Escalation != nil {
			t.Errorf("a disputed advisory finding must not escalate, got %+v", res.Escalation)
		}
		if res.Disposition.Verdict != report.VerdictDeclined {
			t.Errorf("disposition = %+v, want worker/declined", res.Disposition)
		}
	})
}

// TestDeclinedEscalationFeedsVerdictCompute proves the boundary: the
// verdict.Escalation this package produces is exactly the shape verdict.Compute
// consumes, so a declined blocking finding drives an escalate verdict through
// T7 (this package detects; T7 acts).
func TestDeclinedEscalationFeedsVerdictCompute(t *testing.T) {
	f := blockingF("FND-1", "spec-fidelity/R1", "internal/a/a.go", 10)
	res, err := RecordDisposition(f, WorkerDisposition{
		FindingID:       "FND-1",
		Applied:         false,
		Reason:          "the finding misreads the charter",
		EscalationBrief: goodBrief(),
	})
	if err != nil {
		t.Fatalf("RecordDisposition: %v", err)
	}
	if res.Escalation == nil {
		t.Fatalf("expected an escalation")
	}
	// Fold the disposition onto the finding, build a one-round report, and feed
	// the escalation to verdict.Compute — it must produce escalate.
	disposed := f
	disposed.Disposition = &res.Disposition
	out, err := computeWith([]report.Round{roundOf("sha-1", disposed)}, res.Escalation)
	if err != nil {
		t.Fatalf("verdict.Compute with the escalation: %v", err)
	}
	if out != report.VerdictEscalate {
		t.Errorf("verdict = %q, want escalate — the declined-blocking escalation did not drive T7", out)
	}
}

// TestThirdRoundRefused — opening a round past the two-round cap is refused
// (§3.1), via both the standalone gate and the scope-lock.
func TestThirdRoundRefused(t *testing.T) {
	// The standalone round-cap gate.
	if err := RefuseThirdRound(1); err != nil {
		t.Errorf("round 1 must be allowed: %v", err)
	}
	if err := RefuseThirdRound(2); err != nil {
		t.Errorf("round 2 must be allowed: %v", err)
	}
	if err := RefuseThirdRound(3); !errors.Is(err, ErrThirdRoundRefused) {
		t.Errorf("RefuseThirdRound(3) = %v, want ErrThirdRoundRefused", err)
	}

	// The scope-lock refuses a third round too.
	_, err := ApplyScopeLock(3, []report.Finding{blockingF("FND-1", "r", "a.go", 1)}, RoundScope{})
	if !errors.Is(err, ErrThirdRoundRefused) {
		t.Errorf("ApplyScopeLock(round 3) = %v, want ErrThirdRoundRefused", err)
	}
}
