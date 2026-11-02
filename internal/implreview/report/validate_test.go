package report_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/implreview/report"
)

// stubCal is the test's stand-in for the T5 calibration table (a CalLookup):
// the two rows the golden fixture cites, mapping each to the severity that row
// carries. Passing a stub is exactly why Validate takes a CalLookup — this
// package never depends on the calibration data.
var stubCal = report.CalLookupFunc(func(calRow string) (string, bool) {
	switch calRow {
	case "CAL-3":
		return report.SeverityBlocking, true
	case "CAL-7":
		return report.SeverityAdvisory, true
	default:
		return "", false
	}
})

// loadGolden reads and closed-parses the golden valid payload. A parse failure
// here is a fixture bug, not a validation outcome.
func loadGolden(t *testing.T) *report.Report {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "golden-valid.json"))
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}
	r, err := report.Unmarshal(data)
	if err != nil {
		t.Fatalf("unmarshal golden: %v", err)
	}
	return r
}

// TestGoldenValidates — the golden payload parses closed and validates against
// every integrity rule (plan T1 acceptance #1).
func TestGoldenValidates(t *testing.T) {
	t.Parallel()
	r := loadGolden(t)
	if err := report.Validate(r, stubCal); err != nil {
		t.Fatalf("golden failed validation: %v", err)
	}
	// The uniqueness rule holds trivially for the single golden report.
	if err := report.ValidateUnique([]*report.Report{r}); err != nil {
		t.Fatalf("golden failed uniqueness: %v", err)
	}
	// The rendered face regenerates and matches itself.
	if err := report.ValidateRenderFidelity(r, report.Render(r)); err != nil {
		t.Fatalf("golden render fidelity: %v", err)
	}
	// Spot-check that the parse recovered the shape.
	if r.SchemaVersion != report.SchemaVersion {
		t.Fatalf("schema version = %q", r.SchemaVersion)
	}
	if len(r.Rounds) != 2 {
		t.Fatalf("rounds = %d, want 2", len(r.Rounds))
	}
	if r.EscalationBrief != nil {
		t.Fatalf("golden verdict is request-changes; no escalation brief expected")
	}
}

// TestValidateDefectFixtures — one seeded defect per rule; each must fail
// validation naming its rule via errors.Is (plan T1 acceptance #1; the A-2
// defect set: missing cal_row, severity≠cited row, a rendered region diverging,
// a round missing reviewed_revision, a reason-less decline; plus the schema
// version, escalation-brief presence, and at-least-one-round rules).
func TestValidateDefectFixtures(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		mutate  func(r *report.Report) // corrupts a copy of the golden
		wantErr error
	}{
		{
			name:    "wrong schema version",
			mutate:  func(r *report.Report) { r.SchemaVersion = "impl-review-report/v2" },
			wantErr: report.ErrSchemaVersion,
		},
		{
			name:    "no rounds",
			mutate:  func(r *report.Report) { r.Rounds = nil },
			wantErr: report.ErrNoRounds,
		},
		{
			name:    "round missing reviewed_revision",
			mutate:  func(r *report.Report) { r.Rounds[1].ReviewedRevision = "" },
			wantErr: report.ErrMissingReviewedRevision,
		},
		{
			name:    "round reviewed_revision whitespace-only",
			mutate:  func(r *report.Report) { r.Rounds[0].ReviewedRevision = "   " },
			wantErr: report.ErrMissingReviewedRevision,
		},
		{
			name:    "finding missing cal_row",
			mutate:  func(r *report.Report) { r.Rounds[0].Findings[0].CalRow = "" },
			wantErr: report.ErrMissingCalRow,
		},
		{
			name:    "finding cites unknown cal_row",
			mutate:  func(r *report.Report) { r.Rounds[0].Findings[0].CalRow = "CAL-999" },
			wantErr: report.ErrUnknownCalRow,
		},
		{
			name: "severity contradicts cited row",
			// FND-2 cites CAL-7 (advisory) but is flipped to blocking.
			mutate:  func(r *report.Report) { r.Rounds[0].Findings[1].Severity = report.SeverityBlocking },
			wantErr: report.ErrSeverityMismatch,
		},
		{
			name: "reason-less decline",
			// FND-3 is a declined disposition; drop its reason.
			mutate:  func(r *report.Report) { r.Rounds[1].Findings[0].Disposition.Reason = "" },
			wantErr: report.ErrDeclineNoReason,
		},
		{
			name: "escalation brief present without escalate verdict",
			mutate: func(r *report.Report) {
				r.EscalationBrief = &report.EscalationBrief{Conflict: "x", DecisionNeeded: "y"}
			},
			wantErr: report.ErrEscalationBrief,
		},
		{
			name: "escalate verdict missing its brief",
			mutate: func(r *report.Report) {
				r.Verdict = report.VerdictEscalate
				r.EscalationBrief = nil
			},
			wantErr: report.ErrEscalationBrief,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			r := loadGolden(t)
			tc.mutate(r)
			err := report.Validate(r, stubCal)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("Validate error = %v, want errors.Is(_, %v)", err, tc.wantErr)
			}
		})
	}
}

// TestValidateNilCalFailsClosed — a nil CalLookup cannot certify DINV-3, so
// Validate refuses rather than skip the rule.
func TestValidateNilCalFailsClosed(t *testing.T) {
	t.Parallel()
	r := loadGolden(t)
	if err := report.Validate(r, nil); err == nil {
		t.Fatal("Validate with nil CalLookup should fail closed, got nil")
	}
}

// TestValidateUniqueDuplicate — DINV-1: two reports sharing (proposal_ref,
// review_id) are rejected; distinct keys pass.
func TestValidateUniqueDuplicate(t *testing.T) {
	t.Parallel()

	a := loadGolden(t)
	dup := loadGolden(t) // same proposal_ref + review_id as a
	if err := report.ValidateUnique([]*report.Report{a, dup}); !errors.Is(err, report.ErrDuplicateReport) {
		t.Fatalf("duplicate (proposal_ref, review_id) error = %v, want ErrDuplicateReport", err)
	}

	// A different review of the same proposal is a distinct key — allowed.
	other := loadGolden(t)
	other.ReviewID = "rv-02OTHER"
	if err := report.ValidateUnique([]*report.Report{a, other}); err != nil {
		t.Fatalf("distinct review_id should be unique, got %v", err)
	}

	// A different proposal with the same review id is also distinct.
	other2 := loadGolden(t)
	other2.ProposalRef = "github:agenticlab-ai/pyxis#999"
	if err := report.ValidateUnique([]*report.Report{a, other2}); err != nil {
		t.Fatalf("distinct proposal_ref should be unique, got %v", err)
	}
}

// TestRenderFidelityDivergence — DINV-2: a rendered region diverging from the
// payload fails; the untouched rendering passes.
func TestRenderFidelityDivergence(t *testing.T) {
	t.Parallel()
	r := loadGolden(t)

	good := report.Render(r)
	if err := report.ValidateRenderFidelity(r, good); err != nil {
		t.Fatalf("faithful rendering rejected: %v", err)
	}

	// A hand-edited region (a verdict a human tampered with in the face) diverges.
	tampered := strings.Replace(good, "verdict: request-changes", "verdict: approve", 1)
	if tampered == good {
		t.Fatal("fixture bug: the tamper produced no change")
	}
	if err := report.ValidateRenderFidelity(r, tampered); !errors.Is(err, report.ErrRenderFidelity) {
		t.Fatalf("divergent rendering error = %v, want ErrRenderFidelity", err)
	}
}

// TestRenderDeterministic — Render is pure: the same input yields byte-identical
// output across repeated calls (DINV-2's basis).
func TestRenderDeterministic(t *testing.T) {
	t.Parallel()
	r := loadGolden(t)
	first := report.Render(r)
	for i := 0; i < 50; i++ {
		if got := report.Render(r); got != first {
			t.Fatalf("Render is nondeterministic at call %d", i)
		}
	}
	// The rendering must actually project the payload's load-bearing content.
	for _, want := range []string{
		"impl-review-report/v1",
		"github:agenticlab-ai/pyxis#412",
		"verdict: request-changes",
		"#FND-1",
		"#FND-3",
		"disposition: worker declined",
	} {
		if !strings.Contains(first, want) {
			t.Fatalf("rendering omits %q:\n%s", want, first)
		}
	}
}

// TestValidateAppendOnly — plan T1 acceptance #4: a write that mutates a prior
// round's reviewed_revision (or drops a prior round) is rejected; a pure append
// of a new round is accepted.
func TestValidateAppendOnly(t *testing.T) {
	t.Parallel()
	prior := loadGolden(t)

	// A faithful append: keep both prior rounds, add a third.
	appended := loadGolden(t)
	appended.Rounds = append(appended.Rounds, report.Round{
		ReviewedRevision: "c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0c1d2",
	})
	if err := report.ValidateAppend(prior, appended); err != nil {
		t.Fatalf("faithful append rejected: %v", err)
	}

	// A mutated prior round's revision is rejected.
	mutated := loadGolden(t)
	mutated.Rounds[0].ReviewedRevision = "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef"
	if err := report.ValidateAppend(prior, mutated); !errors.Is(err, report.ErrRoundMutated) {
		t.Fatalf("mutated prior revision error = %v, want ErrRoundMutated", err)
	}

	// A dropped prior round is rejected.
	dropped := loadGolden(t)
	dropped.Rounds = dropped.Rounds[:1]
	if err := report.ValidateAppend(prior, dropped); !errors.Is(err, report.ErrRoundMutated) {
		t.Fatalf("dropped prior round error = %v, want ErrRoundMutated", err)
	}
}

// TestUnmarshalClosedParse — the parse fails closed on an unknown key and on
// trailing data (AP-6, DINV-2's closed-parse basis); the golden round-trips
// through Marshal→Unmarshal unchanged.
func TestUnmarshalClosedParse(t *testing.T) {
	t.Parallel()

	if _, err := report.Unmarshal([]byte(`{"schema_version":"impl-review-report/v1","surprise":1}`)); err == nil {
		t.Fatal("unknown key should fail the closed parse, got nil")
	}
	if _, err := report.Unmarshal([]byte(`{"schema_version":"impl-review-report/v1"} {}`)); err == nil {
		t.Fatal("trailing data should fail the parse, got nil")
	}

	// Round-trip: Marshal then Unmarshal recovers an equal payload that still
	// validates (the one parser both producers and readers share).
	r := loadGolden(t)
	data, err := r.Marshal()
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	r2, err := report.Unmarshal(data)
	if err != nil {
		t.Fatalf("unmarshal round-trip: %v", err)
	}
	if err := report.Validate(r2, stubCal); err != nil {
		t.Fatalf("round-tripped payload failed validation: %v", err)
	}
	if report.Render(r2) != report.Render(r) {
		t.Fatal("round-trip changed the rendered face")
	}
}
