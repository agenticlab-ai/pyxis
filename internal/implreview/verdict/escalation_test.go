package verdict

import (
	"errors"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/implreview/report"
)

// TestEscalateProducesWellFormedBrief — an escalate condition yields an escalate
// verdict carrying a well-formed FR-17 brief; a non-escalate carries none.
func TestEscalateProducesWellFormedBrief(t *testing.T) {
	goodBrief := report.EscalationBrief{
		Conflict:       "the charter is self-contradictory about the divergent behavior",
		Options:        []string{"ratify behavior A and amend the charter", "ratify behavior B and amend the charter"},
		DecisionNeeded: "which behavior the charter intends on the load-bearing path",
	}

	t.Run("escalate → well-formed brief, force-human", func(t *testing.T) {
		o, err := Compute([]report.Round{round(blocking("FND-1", nil))}, Signals{Escalation: &Escalation{Brief: goodBrief}})
		if err != nil {
			t.Fatalf("Compute escalate: %v", err)
		}
		if o.Verdict != report.VerdictEscalate {
			t.Fatalf("verdict = %q, want escalate", o.Verdict)
		}
		if o.Authority != AuthorityForceHuman {
			t.Errorf("authority = %q, want force-human", o.Authority)
		}
		if o.Brief == nil {
			t.Fatalf("escalate carries no brief")
		}
		if o.Brief.Conflict != goodBrief.Conflict || len(o.Brief.Options) != 2 || o.Brief.DecisionNeeded != goodBrief.DecisionNeeded {
			t.Errorf("brief content mismatch: %+v", o.Brief)
		}
	})

	t.Run("escalate dominates a would-be-approve findings set", func(t *testing.T) {
		// Even with NO blocking findings, an escalate signal forces escalate —
		// it is the spec-decision case, stronger than a severity-derived verdict.
		o, err := Compute([]report.Round{round(advisory("FND-1"))}, Signals{Escalation: &Escalation{Brief: goodBrief}})
		if err != nil {
			t.Fatal(err)
		}
		if o.Verdict != report.VerdictEscalate {
			t.Errorf("escalate signal did not dominate: verdict = %q", o.Verdict)
		}
	})

	t.Run("non-escalate carries no brief", func(t *testing.T) {
		for _, r := range [][]report.Round{
			{round()},                       // approve
			{round(advisory("FND-1"))},      // approve-with-advisories
			{round(blocking("FND-1", nil))}, // request-changes
		} {
			o, err := Compute(r, Signals{})
			if err != nil {
				t.Fatal(err)
			}
			if o.Verdict == report.VerdictEscalate {
				t.Fatalf("no-signal Compute produced escalate for %v", r)
			}
			if o.Brief != nil {
				t.Errorf("non-escalate verdict %q carries a brief: %+v", o.Verdict, o.Brief)
			}
		}
	})

	t.Run("the Outcome brief does not alias the caller's Signals", func(t *testing.T) {
		sig := Signals{Escalation: &Escalation{Brief: goodBrief}}
		o, err := Compute([]report.Round{round()}, sig)
		if err != nil {
			t.Fatal(err)
		}
		// Mutating the caller's brief must not change the computed Outcome.
		sig.Escalation.Brief.Conflict = "MUTATED"
		if o.Brief.Conflict == "MUTATED" {
			t.Errorf("Outcome.Brief aliases the caller's Signals brief")
		}
	})
}

// TestEscalateIncompleteBriefRejected — an escalate signalled with a hollow brief
// is refused (no conflict, no options, or no decision-needed).
func TestEscalateIncompleteBriefRejected(t *testing.T) {
	cases := []struct {
		name  string
		brief report.EscalationBrief
	}{
		{"no conflict", report.EscalationBrief{Options: []string{"A"}, DecisionNeeded: "d"}},
		{"no options", report.EscalationBrief{Conflict: "c", DecisionNeeded: "d"}},
		{"empty option in the list", report.EscalationBrief{Conflict: "c", Options: []string{""}, DecisionNeeded: "d"}},
		{"no decision needed", report.EscalationBrief{Conflict: "c", Options: []string{"A"}}},
		{"fully empty", report.EscalationBrief{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Compute([]report.Round{round()}, Signals{Escalation: &Escalation{Brief: tc.brief}})
			if !errors.Is(err, ErrIncompleteBrief) {
				t.Fatalf("err = %v, want ErrIncompleteBrief", err)
			}
		})
	}
}
