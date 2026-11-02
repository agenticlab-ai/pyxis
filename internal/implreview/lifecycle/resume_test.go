package lifecycle

import (
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/implreview/report"
)

// reportWith builds a persisted review with `nRounds` recorded rounds and the
// given envelope verdict, for the lifecycle tests. An empty verdict is the
// in-flight (T4-produced-round-but-T7-unstamped) state; a non-empty one is
// terminal. Zero rounds models a report that has not started a round.
func reportWith(verdict string, nRounds int) *report.Report {
	rounds := make([]report.Round, 0, nRounds)
	for i := 0; i < nRounds; i++ {
		rounds = append(rounds, report.Round{ReviewedRevision: "sha-" + string(rune('a'+i))})
	}
	return &report.Report{
		SchemaVersion: report.SchemaVersion,
		ProposalRef:   "github:agenticlab-ai/pyxis#412",
		ReviewID:      "rv-1",
		Verdict:       verdict,
		Rounds:        rounds,
	}
}

// TestDecide is the idempotent-resume decision (UB-8, DINV-1): a re-invocation
// reads the persisted round state as authoritative and resumes there; a
// double-invocation is a no-op that starts NO fresh round 1. The decision is
// deterministic from (started-round, terminal-verdict).
func TestDecide(t *testing.T) {
	cases := []struct {
		name        string
		persisted   *report.Report
		wantAction  Action
		wantAtRound int
	}{
		{
			name:        "nil persisted (never persisted) → start round 1",
			persisted:   nil,
			wantAction:  ActionStart,
			wantAtRound: 0,
		},
		{
			name:        "zero rounds (started nothing) → start round 1",
			persisted:   reportWith("", 0),
			wantAction:  ActionStart,
			wantAtRound: 0,
		},
		{
			name:        "one round, NO terminal verdict → resume at round 1 (not a fresh round 1)",
			persisted:   reportWith("", 1),
			wantAction:  ActionResume,
			wantAtRound: 1,
		},
		{
			name:        "two rounds, NO terminal verdict → resume at round 2",
			persisted:   reportWith("", 2),
			wantAction:  ActionResume,
			wantAtRound: 2,
		},
		{
			name:        "one round + approve verdict → complete (no-op)",
			persisted:   reportWith(report.VerdictApprove, 1),
			wantAction:  ActionComplete,
			wantAtRound: 1,
		},
		{
			name:        "one round + request-changes (terminal — bounced, but review completed) → complete",
			persisted:   reportWith(report.VerdictRequestChanges, 1),
			wantAction:  ActionComplete,
			wantAtRound: 1,
		},
		{
			name:        "two rounds + escalate (terminal — routed human, review completed) → complete at round 2",
			persisted:   reportWith(report.VerdictEscalate, 2),
			wantAction:  ActionComplete,
			wantAtRound: 2,
		},
		{
			name:        "one round + approve-with-advisories → complete",
			persisted:   reportWith(report.VerdictApproveWithAdvisories, 1),
			wantAction:  ActionComplete,
			wantAtRound: 1,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := Decide(tc.persisted)
			if got.Action != tc.wantAction {
				t.Errorf("Decide action = %q, want %q", got.Action, tc.wantAction)
			}
			if got.AtRound != tc.wantAtRound {
				t.Errorf("Decide AtRound = %d, want %d", got.AtRound, tc.wantAtRound)
			}
		})
	}
}

// TestDecideIsDeterministicAndIdempotent asserts the core UB-8 property directly:
// re-invoking Decide on the SAME persisted state yields the SAME decision (no
// clock, no drift), and — critically — a persisted report that already has a
// round NEVER yields ActionStart. That is the "double-invocation starts no fresh
// round 1" guarantee made structural: the only path to ActionStart is an unstarted
// state, so a second call over a started review resumes or completes, never
// restarts.
func TestDecideIsDeterministicAndIdempotent(t *testing.T) {
	started := []*report.Report{
		reportWith("", 1),                           // resume
		reportWith("", 2),                           // resume
		reportWith(report.VerdictApprove, 1),        // complete
		reportWith(report.VerdictRequestChanges, 2), // complete
		reportWith(report.VerdictEscalate, 1),       // complete
	}
	for _, r := range started {
		first := Decide(r)
		second := Decide(r)
		if first != second {
			t.Errorf("Decide not idempotent: first=%v second=%v", first, second)
		}
		if first.Action == ActionStart {
			t.Errorf("a started review (%d rounds, verdict=%q) decided ActionStart — a double-invocation must NOT start a fresh round 1",
				len(r.Rounds), r.Verdict)
		}
		if ShouldStartFresh(r) {
			t.Errorf("ShouldStartFresh returned true for a started review (%d rounds, verdict=%q) — no fresh round 1",
				len(r.Rounds), r.Verdict)
		}
	}
}

// TestShouldStartFresh confirms the one-liner guard: true ONLY for an unstarted
// state, false for resume/complete — the no-fresh-round-1 gate a caller uses.
func TestShouldStartFresh(t *testing.T) {
	cases := []struct {
		name      string
		persisted *report.Report
		want      bool
	}{
		{"nil → start fresh", nil, true},
		{"zero rounds → start fresh", reportWith("", 0), true},
		{"started, no verdict → NOT fresh (resume)", reportWith("", 1), false},
		{"started, terminal → NOT fresh (complete)", reportWith(report.VerdictApprove, 1), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ShouldStartFresh(tc.persisted); got != tc.want {
				t.Errorf("ShouldStartFresh = %t, want %t", got, tc.want)
			}
		})
	}
}

// TestTerminalAndStartedHelpers pins the two exported bits the janitor reads.
func TestTerminalAndStartedHelpers(t *testing.T) {
	if HasStartedRound(nil) {
		t.Error("HasStartedRound(nil) = true, want false")
	}
	if HasStartedRound(reportWith("", 0)) {
		t.Error("HasStartedRound(zero rounds) = true, want false")
	}
	if !HasStartedRound(reportWith("", 1)) {
		t.Error("HasStartedRound(one round) = false, want true")
	}
	if TerminalVerdict(nil) {
		t.Error("TerminalVerdict(nil) = true, want false")
	}
	if TerminalVerdict(reportWith("", 1)) {
		t.Error("TerminalVerdict(empty verdict) = true, want false (in-flight)")
	}
	if !TerminalVerdict(reportWith(report.VerdictApprove, 1)) {
		t.Error("TerminalVerdict(approve) = false, want true")
	}
	if !TerminalVerdict(reportWith(report.VerdictRequestChanges, 1)) {
		t.Error("TerminalVerdict(request-changes) = false, want true (review completed even though bounced)")
	}
}
