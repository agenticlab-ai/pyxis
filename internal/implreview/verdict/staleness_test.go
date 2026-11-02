package verdict

import (
	"errors"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/implreview/report"
)

// reportAt builds a report whose latest round evaluated `rev`, with a set
// envelope verdict, for the staleness tests.
func reportAt(verdict string, revisions ...string) *report.Report {
	rounds := make([]report.Round, 0, len(revisions))
	for _, rev := range revisions {
		rounds = append(rounds, report.Round{ReviewedRevision: rev})
	}
	return &report.Report{
		SchemaVersion: report.SchemaVersion,
		ProposalRef:   "github:agenticlab-ai/pyxis#412",
		ReviewID:      "rv-1",
		Verdict:       verdict,
		Rounds:        rounds,
	}
}

// TestStaleness is UB-6/DINV-7: head == latest reviewed_revision → live; head !=
// latest → stale and treated as absent.
func TestStaleness(t *testing.T) {
	cases := []struct {
		name        string
		report      *report.Report
		head        string
		wantStale   bool
		wantPresent bool
		wantLive    string
	}{
		{
			name:        "head == latest reviewed_revision → live, present",
			report:      reportAt(report.VerdictRequestChanges, "sha-1", "sha-2"),
			head:        "sha-2",
			wantStale:   false,
			wantPresent: true,
			wantLive:    report.VerdictRequestChanges,
		},
		{
			name:        "head != latest (moved past) → stale, absent",
			report:      reportAt(report.VerdictRequestChanges, "sha-1", "sha-2"),
			head:        "sha-3",
			wantStale:   true,
			wantPresent: false,
		},
		{
			name:        "head == an EARLIER round but not the latest → stale (latest governs)",
			report:      reportAt(report.VerdictApprove, "sha-1", "sha-2"),
			head:        "sha-1",
			wantStale:   true,
			wantPresent: false,
		},
		{
			name:        "single round, head matches → live",
			report:      reportAt(report.VerdictApprove, "sha-1"),
			head:        "sha-1",
			wantStale:   false,
			wantPresent: true,
			wantLive:    report.VerdictApprove,
		},
		{
			name:        "empty head → stale (cannot certify)",
			report:      reportAt(report.VerdictApprove, "sha-1"),
			head:        "",
			wantStale:   true,
			wantPresent: false,
		},
		{
			name:        "no rounds → stale (nothing certified)",
			report:      reportAt(report.VerdictApprove),
			head:        "sha-1",
			wantStale:   true,
			wantPresent: false,
		},
		{
			name:        "verdict unset but head matches → not stale, but absent (no live verdict)",
			report:      reportAt("", "sha-1"),
			head:        "sha-1",
			wantStale:   false,
			wantPresent: false,
		},
		{
			name:        "nil report → stale, absent",
			report:      nil,
			head:        "sha-1",
			wantStale:   true,
			wantPresent: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsStale(tc.report, tc.head); got != tc.wantStale {
				t.Errorf("IsStale = %v, want %v", got, tc.wantStale)
			}
			live, present := Current(tc.report, tc.head)
			if present != tc.wantPresent {
				t.Errorf("Current present = %v, want %v", present, tc.wantPresent)
			}
			if present && live != tc.wantLive {
				t.Errorf("Current live = %q, want %q", live, tc.wantLive)
			}
			if !present && live != "" {
				t.Errorf("absent Current must return empty string, got %q", live)
			}
		})
	}
}

// TestStaleErrorSentinel — StaleError returns nil when current and a wrapped
// ErrStaleVerdict when stale (the error path a post-adoption raise uses).
func TestStaleErrorSentinel(t *testing.T) {
	live := reportAt(report.VerdictApprove, "sha-1")
	if err := StaleError(live, "sha-1"); err != nil {
		t.Errorf("StaleError on a current verdict = %v, want nil", err)
	}
	if err := StaleError(live, "sha-2"); !errors.Is(err, ErrStaleVerdict) {
		t.Errorf("StaleError on a stale verdict = %v, want ErrStaleVerdict", err)
	}
	// No-round report also errors stale.
	if err := StaleError(reportAt(report.VerdictApprove), "sha-1"); !errors.Is(err, ErrStaleVerdict) {
		t.Errorf("StaleError on a no-round report = %v, want ErrStaleVerdict", err)
	}
}

// TestLatestReviewedRevision — the latest round's revision, and absence.
func TestLatestReviewedRevision(t *testing.T) {
	rev, ok := LatestReviewedRevision(reportAt(report.VerdictApprove, "sha-1", "sha-2"))
	if !ok || rev != "sha-2" {
		t.Errorf("LatestReviewedRevision = (%q, %v), want (sha-2, true)", rev, ok)
	}
	if _, ok := LatestReviewedRevision(reportAt(report.VerdictApprove)); ok {
		t.Errorf("LatestReviewedRevision on a no-round report must be absent")
	}
	if _, ok := LatestReviewedRevision(nil); ok {
		t.Errorf("LatestReviewedRevision(nil) must be absent")
	}
}
