package charter

import (
	"errors"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/implreview/report"
	"github.com/agenticlab-ai/pyxis/internal/implreview/verdict"
)

// goodBrief is a complete FR-17 escalation brief the escalate cases carry.
func goodBrief() report.EscalationBrief {
	return report.EscalationBrief{
		Conflict:       "the charter requires both retry-on-timeout and fail-fast on the same path",
		Options:        []string{"ratify retry-on-timeout and amend the charter", "ratify fail-fast and amend the charter"},
		DecisionNeeded: "which behavior the charter intends on the timeout path",
	}
}

// TestValidate_TriState is the whole FR-14 contract as one table: every
// invocation shape resolves to exactly one of proceed / refuse / escalate, the
// error is present iff the outcome is not proceed, and no shape yields a silent
// pass.
func TestValidate_TriState(t *testing.T) {
	cases := []struct {
		name string
		inv  Invocation

		wantKind        Kind
		wantErr         error // errors.Is target; nil ⇒ no error expected
		wantReview      bool  // ProducesReview()
		wantWithholding bool  // WithheldSpan != nil
	}{
		{
			name:            "charter-less (empty) is refused, named, no review",
			inv:             Invocation{Charter: ""},
			wantKind:        KindRefuse,
			wantErr:         ErrNoCharter,
			wantReview:      false,
			wantWithholding: false,
		},
		{
			name:            "whitespace-only charter is no charter — refused",
			inv:             Invocation{Charter: "  \n\t "},
			wantKind:        KindRefuse,
			wantErr:         ErrNoCharter,
			wantReview:      false,
			wantWithholding: false,
		},
		{
			name:            "absence dominates a defect report — a charter-less invocation refuses even with a Defect",
			inv:             Invocation{Charter: "", Defect: &MaterialDefect{Class: DefectAmbiguous, Brief: goodBrief()}},
			wantKind:        KindRefuse,
			wantErr:         ErrNoCharter,
			wantReview:      false,
			wantWithholding: false,
		},
		{
			name:            "present charter, no defect reported — proceeds",
			inv:             Invocation{Charter: "The change must debounce input by 200ms."},
			wantKind:        KindProceed,
			wantErr:         nil,
			wantReview:      true,
			wantWithholding: false,
		},
		{
			name: "present but materially defective (localized) — escalates upstream, withholds against the span",
			inv: Invocation{
				Charter: "Retry on timeout. Also, fail fast on timeout.",
				Defect: &MaterialDefect{
					Class: DefectSelfContradictory,
					Span:  Span{Ref: "REQ-timeout-behavior"},
					Brief: goodBrief(),
				},
			},
			wantKind:        KindEscalate,
			wantErr:         ErrDefectiveCharter,
			wantReview:      true, // an escalate produces the escalate verdict + withheld-span record
			wantWithholding: true,
		},
		{
			name: "present but defective, un-localized span — escalates, withholds against the whole charter",
			inv: Invocation{
				Charter: "Make it robust.",
				Defect: &MaterialDefect{
					Class: DefectAmbiguous,
					// no Span ⇒ whole-charter fail-closed withholding
					Brief: goodBrief(),
				},
			},
			wantKind:        KindEscalate,
			wantErr:         ErrDefectiveCharter,
			wantReview:      true,
			wantWithholding: true,
		},
		{
			name: "charter contradicting the governing design escalates as its named class",
			inv: Invocation{
				Charter: "Store the token in the client cookie.",
				Defect: &MaterialDefect{
					Class: DefectContradictsDesign,
					Span:  Span{Ref: "REQ-token-storage"},
					Brief: goodBrief(),
				},
			},
			wantKind:        KindEscalate,
			wantErr:         ErrDefectiveCharter,
			wantReview:      true,
			wantWithholding: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out, err := Validate(tc.inv)

			if out.Kind != tc.wantKind {
				t.Errorf("Kind = %s, want %s", out.Kind, tc.wantKind)
			}
			if tc.wantErr == nil {
				if err != nil {
					t.Errorf("err = %v, want nil", err)
				}
			} else if !errors.Is(err, tc.wantErr) {
				t.Errorf("err = %v, want errors.Is %v", err, tc.wantErr)
			}
			if got := out.ProducesReview(); got != tc.wantReview {
				t.Errorf("ProducesReview() = %v, want %v", got, tc.wantReview)
			}
			if got := out.WithheldSpan != nil; got != tc.wantWithholding {
				t.Errorf("WithheldSpan present = %v, want %v", got, tc.wantWithholding)
			}

			// The "no silent pass" postcondition, asserted for EVERY row:
			// a non-proceed outcome ALWAYS carries an error, and a proceed NEVER
			// does — there is no third, error-free-yet-not-proceed state.
			if out.Kind == KindProceed {
				if err != nil {
					t.Errorf("proceed carried an error (a silent-pass boundary violation): %v", err)
				}
			} else if err == nil {
				t.Errorf("a non-proceed outcome (%s) carried NO error — that is the silent pass FR-14 forbids", out.Kind)
			}
		})
	}
}

// TestValidate_RefusalProducesNoReview pins the FR-14 "produces no review"
// guarantee for the charter-less case: a refusal carries the named defect, no
// withheld span, no escalate signals, and ProducesReview()==false.
func TestValidate_RefusalProducesNoReview(t *testing.T) {
	out, err := Validate(Invocation{Charter: ""})

	if !errors.Is(err, ErrNoCharter) {
		t.Fatalf("err = %v, want ErrNoCharter", err)
	}
	if !out.Refused() {
		t.Fatalf("Refused() = false, want true (Kind=%s)", out.Kind)
	}
	if out.ProducesReview() {
		t.Errorf("a refusal ProducesReview() = true, want false — a refused invocation produces no review (FR-14)")
	}
	if out.WithheldSpan != nil {
		t.Errorf("a refusal carries a withheld span %v — it should have none (no review to withhold from)", out.WithheldSpan)
	}
	if out.Signals != nil {
		t.Errorf("a refusal carries escalate Signals %+v — a refusal is not an escalate", out.Signals)
	}
	if out.Class != "" {
		t.Errorf("a refusal carries a defect class %q — the no-charter defect is not a material-defect class", out.Class)
	}
}

// TestValidate_EscalateWithholdsAffectedSpan pins the FR-14 withholding rule: on
// a localized material defect, a finding against the defective span is withheld
// and a finding against a DIFFERENT span is not — the review withholds against
// the affected span rather than reviewing against a broken spec, but still
// reviews the rest.
func TestValidate_EscalateWithholdsAffectedSpan(t *testing.T) {
	defectiveSpan := Span{Ref: "REQ-timeout-behavior"}
	otherSpan := Span{Ref: "REQ-logging"}

	out, err := Validate(Invocation{
		Charter: "Retry on timeout. Also, fail fast on timeout.",
		Defect: &MaterialDefect{
			Class: DefectSelfContradictory,
			Span:  defectiveSpan,
			Brief: goodBrief(),
		},
	})
	if !errors.Is(err, ErrDefectiveCharter) {
		t.Fatalf("err = %v, want ErrDefectiveCharter", err)
	}
	if !out.Escalated() {
		t.Fatalf("Escalated() = false, want true (Kind=%s)", out.Kind)
	}
	if out.Class != DefectSelfContradictory {
		t.Errorf("Class = %q, want %q", out.Class, DefectSelfContradictory)
	}
	if out.WithheldSpan == nil || out.WithheldSpan.Ref != defectiveSpan.Ref {
		t.Fatalf("WithheldSpan = %v, want %v", out.WithheldSpan, defectiveSpan)
	}

	// A finding against the defective span is WITHHELD.
	if !out.Withholds(defectiveSpan) {
		t.Errorf("Withholds(defective span) = false, want true — a finding against the broken span must be withheld")
	}
	// A finding against an UNAFFECTED span is NOT withheld — the rest is reviewed.
	if out.Withholds(otherSpan) {
		t.Errorf("Withholds(other span) = true, want false — only the defective span is withheld from, not the whole review")
	}
}

// TestValidate_WholeCharterWithholdsEverything pins the fail-closed default: an
// un-localized defect (no Span) withholds EVERY finding — better to withhold
// broadly than review part of a charter whose defect the caller could not bound.
func TestValidate_WholeCharterWithholdsEverything(t *testing.T) {
	out, err := Validate(Invocation{
		Charter: "Make it robust.",
		Defect: &MaterialDefect{
			Class: DefectAmbiguous,
			Brief: goodBrief(),
		},
	})
	if !errors.Is(err, ErrDefectiveCharter) {
		t.Fatalf("err = %v, want ErrDefectiveCharter", err)
	}
	if out.WithheldSpan == nil || !out.WithheldSpan.Whole() {
		t.Fatalf("WithheldSpan = %v, want the whole-charter span", out.WithheldSpan)
	}
	for _, span := range []Span{{Ref: "REQ-a"}, {Ref: "REQ-b"}, {}} {
		if !out.Withholds(span) {
			t.Errorf("Withholds(%v) = false under a whole-charter defect, want true (every finding withheld)", span)
		}
	}
}

// TestOutcome_WithholdsNeverUnderProceedOrRefuse pins that only an escalate
// withholds: proceed and refuse withhold nothing (a proceed reviews normally; a
// refuse produces no findings at all).
func TestOutcome_WithholdsNeverUnderProceedOrRefuse(t *testing.T) {
	span := Span{Ref: "REQ-x"}

	proceed, err := Validate(Invocation{Charter: "A real charter."})
	if err != nil {
		t.Fatal(err)
	}
	if proceed.Withholds(span) {
		t.Errorf("a proceed outcome withholds %v — it should review normally", span)
	}

	refuse, err := Validate(Invocation{Charter: ""})
	if !errors.Is(err, ErrNoCharter) {
		t.Fatal(err)
	}
	if refuse.Withholds(span) {
		t.Errorf("a refuse outcome withholds %v — a refusal produces no findings to withhold", span)
	}
}

// TestValidate_EscalateFeedsVerdictLayer pins the T7 handoff: the escalate
// Outcome's Signals is exactly what verdict.Compute consumes to produce an
// escalate verdict carrying the same brief — the FR-17 handshake, proven by
// driving verdict.Compute with it (no re-derivation in this package).
func TestValidate_EscalateFeedsVerdictLayer(t *testing.T) {
	brief := goodBrief()
	out, err := Validate(Invocation{
		Charter: "Retry on timeout. Also, fail fast on timeout.",
		Defect: &MaterialDefect{
			Class: DefectSelfContradictory,
			Span:  Span{Ref: "REQ-timeout-behavior"},
			Brief: brief,
		},
	})
	if !errors.Is(err, ErrDefectiveCharter) {
		t.Fatalf("err = %v, want ErrDefectiveCharter", err)
	}
	if out.Signals == nil {
		t.Fatal("escalate Outcome carries no verdict.Signals — the T7 handoff is missing")
	}

	// Drive the real verdict layer with the produced signal: it must escalate,
	// force-human, and carry the brief this package built (FR-17). A dummy round
	// stands in for "a review happened" — verdict.Compute requires ≥1 round.
	dummyRound := report.Round{ReviewedRevision: "abc123"}
	vout, verr := verdict.Compute([]report.Round{dummyRound}, *out.Signals)
	if verr != nil {
		t.Fatalf("verdict.Compute(escalate signal) errored: %v", verr)
	}
	if vout.Verdict != report.VerdictEscalate {
		t.Errorf("verdict = %q, want escalate — the charter escalate did not drive an escalate verdict", vout.Verdict)
	}
	if vout.Authority != verdict.AuthorityForceHuman {
		t.Errorf("authority = %q, want force-human", vout.Authority)
	}
	if vout.Brief == nil || vout.Brief.Conflict != brief.Conflict || vout.Brief.DecisionNeeded != brief.DecisionNeeded {
		t.Errorf("verdict brief mismatch: got %+v, want the charter's brief", vout.Brief)
	}
}

// TestValidate_MalformedDefectReportRefused pins that a caller-supplied defect
// report that is itself malformed — an unnamed class, or a hollow brief — is
// REFUSED (a wrapping error, no escalate Outcome) rather than escalated under an
// unnamed class or with an unactionable brief. A malformed report is a
// programmer error surfaced, never a silent degrade.
func TestValidate_MalformedDefectReportRefused(t *testing.T) {
	cases := []struct {
		name   string
		defect *MaterialDefect
	}{
		{
			name:   "unnamed defect class",
			defect: &MaterialDefect{Class: DefectClass("bogus"), Brief: goodBrief()},
		},
		{
			name:   "empty defect class",
			defect: &MaterialDefect{Class: "", Brief: goodBrief()},
		},
		{
			name:   "hollow brief — no conflict",
			defect: &MaterialDefect{Class: DefectAmbiguous, Brief: report.EscalationBrief{Options: []string{"A"}, DecisionNeeded: "d"}},
		},
		{
			name:   "hollow brief — no options",
			defect: &MaterialDefect{Class: DefectAmbiguous, Brief: report.EscalationBrief{Conflict: "c", DecisionNeeded: "d"}},
		},
		{
			name:   "hollow brief — empty option in the list",
			defect: &MaterialDefect{Class: DefectAmbiguous, Brief: report.EscalationBrief{Conflict: "c", Options: []string{""}, DecisionNeeded: "d"}},
		},
		{
			name:   "hollow brief — no decision needed",
			defect: &MaterialDefect{Class: DefectAmbiguous, Brief: report.EscalationBrief{Conflict: "c", Options: []string{"A"}}},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out, err := Validate(Invocation{Charter: "A present charter.", Defect: tc.defect})
			if err == nil {
				t.Fatalf("malformed defect report did not error (silent degrade): out=%+v", out)
			}
			// It must NOT escalate under a malformed report (no partial escalate).
			if out.Escalated() {
				t.Errorf("a malformed defect report escalated (Kind=%s) — it must be refused, not escalated", out.Kind)
			}
			if out.Signals != nil {
				t.Errorf("a malformed defect report produced escalate Signals %+v", out.Signals)
			}
			// And it is emphatically not a silent proceed.
			if out.Proceeds() {
				t.Errorf("a malformed defect report PROCEEDED — the silent pass FR-14 forbids")
			}
		})
	}
}

// TestValidate_OutcomeDoesNotAliasInvocation pins that mutating the caller's
// Invocation after Validate does not change the produced Outcome's brief — the
// same non-aliasing guarantee verdict.Compute makes.
func TestValidate_OutcomeDoesNotAliasInvocation(t *testing.T) {
	inv := Invocation{
		Charter: "Retry on timeout. Also, fail fast on timeout.",
		Defect: &MaterialDefect{
			Class: DefectSelfContradictory,
			Span:  Span{Ref: "REQ-timeout-behavior"},
			Brief: goodBrief(),
		},
	}
	out, err := Validate(inv)
	if !errors.Is(err, ErrDefectiveCharter) {
		t.Fatal(err)
	}
	if out.Signals == nil || out.Signals.Escalation == nil {
		t.Fatal("no escalate signal produced")
	}

	// Mutate the caller's brief; the Outcome's carried brief must not change.
	inv.Defect.Brief.Conflict = "MUTATED"
	if out.Signals.Escalation.Brief.Conflict == "MUTATED" {
		t.Errorf("Outcome.Signals aliases the caller's Invocation brief")
	}
}

// TestKind_String and the span helpers — cheap coverage of the human-facing
// renderings so a Kind/Span never surfaces as a bare integer or an empty string
// in a log.
func TestKind_String(t *testing.T) {
	for k, want := range map[Kind]string{
		KindProceed:  "proceed",
		KindRefuse:   "refuse",
		KindEscalate: "escalate",
		Kind(99):     "charter.Kind(99)",
	} {
		if got := k.String(); got != want {
			t.Errorf("Kind(%d).String() = %q, want %q", int(k), got, want)
		}
	}
}

func TestSpan_WholeAndString(t *testing.T) {
	whole := Span{}
	if !whole.Whole() {
		t.Errorf("empty Span.Whole() = false, want true")
	}
	if whole.String() != "the whole charter" {
		t.Errorf("empty Span.String() = %q, want %q", whole.String(), "the whole charter")
	}
	local := Span{Ref: "REQ-7"}
	if local.Whole() {
		t.Errorf("localized Span.Whole() = true, want false")
	}
	if local.String() != "REQ-7" {
		t.Errorf("localized Span.String() = %q, want %q", local.String(), "REQ-7")
	}
}
