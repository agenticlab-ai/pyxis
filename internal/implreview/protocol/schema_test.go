package protocol

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/implreview/report"
)

// TestRecordRoundValidPassesFirstTry — a schema-valid round is recorded on the
// first attempt (no retry, no absence).
func TestRecordRoundValidPassesFirstTry(t *testing.T) {
	valid := baseReport(report.VerdictRequestChanges, roundOf("sha-1", blockingF("FND-1", "r1", "a.go", 5)))
	calls := 0
	attempt := func() (*report.Report, error) {
		calls++
		return valid, nil
	}
	r, err := RecordRound(attempt, stubCal())
	if err != nil {
		t.Fatalf("RecordRound(valid): %v", err)
	}
	if r == nil {
		t.Fatalf("RecordRound returned a nil report")
	}
	if calls != 1 {
		t.Errorf("attempts = %d, want 1 (a valid round needs no retry)", calls)
	}
}

// TestSchemaInvalidRoundVoidsRetriesOnceThenAbsent — a schema-invalid round is
// voided, retried EXACTLY once, and then records review-absent (UB-2). Never a
// silent pass, never an unbounded retry.
func TestSchemaInvalidRoundVoidsRetriesOnceThenAbsent(t *testing.T) {
	t.Run("invalid then invalid → ABSENT after exactly one retry", func(t *testing.T) {
		// An invalid report: a finding whose severity contradicts its cal_row
		// (CAL-2 → advisory, but the finding is blocking) — report.Validate rejects
		// it (DINV-3).
		invalid := baseReport(report.VerdictRequestChanges, report.Round{
			ReviewedRevision: "sha-1",
			Findings: []report.Finding{{
				ID: "FND-1", RuleID: "r1", Message: "m", CalRow: "CAL-2",
				Lens: report.LensSpecFidelity, Severity: report.SeverityBlocking, // mismatch
			}},
		})
		calls := 0
		attempt := func() (*report.Report, error) {
			calls++
			return invalid, nil
		}
		_, err := RecordRound(attempt, stubCal())
		if !errors.Is(err, ErrReviewAbsent) {
			t.Fatalf("RecordRound = %v, want ErrReviewAbsent", err)
		}
		if !IsReviewAbsent(err) {
			t.Errorf("IsReviewAbsent = false, want true")
		}
		if calls != 2 {
			t.Errorf("attempts = %d, want 2 (initial + exactly one retry)", calls)
		}
		// The absence is the branchable terminal (ErrReviewAbsent), and its message
		// surfaces the underlying schema failure for the reader (not a silent pass).
		// The cause is rendered as text, not a %w chain — ErrReviewAbsent is the one
		// sentinel a caller maps to absence.
		if !strings.Contains(err.Error(), "severity contradicts") {
			t.Errorf("the absence message should name the schema failure; got %v", err)
		}
	})

	t.Run("invalid then valid → the retry succeeds (no absence)", func(t *testing.T) {
		invalid := baseReport(report.VerdictRequestChanges, report.Round{
			ReviewedRevision: "", // missing reviewed_revision → DINV-7 failure
			Findings:         nil,
		})
		valid := baseReport(report.VerdictApprove, roundOf("sha-1"))
		calls := 0
		attempt := func() (*report.Report, error) {
			calls++
			if calls == 1 {
				return invalid, nil
			}
			return valid, nil
		}
		r, err := RecordRound(attempt, stubCal())
		if err != nil {
			t.Fatalf("RecordRound(invalid→valid): %v", err)
		}
		if r == nil || calls != 2 {
			t.Errorf("want the second attempt to succeed; report=%v calls=%d", r, calls)
		}
	})

	t.Run("an attempt that ERRORS is treated as an invalid round (voids, retries, absent)", func(t *testing.T) {
		calls := 0
		attempt := func() (*report.Report, error) {
			calls++
			return nil, fmt.Errorf("the evaluation crashed")
		}
		_, err := RecordRound(attempt, stubCal())
		if !errors.Is(err, ErrReviewAbsent) {
			t.Fatalf("a failing attempt must go absent, not pass: %v", err)
		}
		if calls != 2 {
			t.Errorf("attempts = %d, want 2", calls)
		}
	})
}

// TestRecordRoundGuards — nil attempt and nil calibration are refused.
func TestRecordRoundGuards(t *testing.T) {
	if _, err := RecordRound(nil, stubCal()); err == nil {
		t.Errorf("RecordRound(nil attempt) must error")
	}
	valid := baseReport(report.VerdictApprove, roundOf("sha-1"))
	if _, err := RecordRound(func() (*report.Report, error) { return valid, nil }, nil); err == nil {
		t.Errorf("RecordRound(nil cal) must error — DINV-3 cannot be certified")
	}
}

// TestMaxRoundsConstant — the two-round cap is exactly 2 and the retry budget is
// exactly 1 (structural constants, not knobs).
func TestMaxRoundsConstant(t *testing.T) {
	if MaxRounds != 2 {
		t.Errorf("MaxRounds = %d, want 2", MaxRounds)
	}
	if MaxSchemaRetries != 1 {
		t.Errorf("MaxSchemaRetries = %d, want 1", MaxSchemaRetries)
	}
}
