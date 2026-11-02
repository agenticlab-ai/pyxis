package spend_test

import (
	"reflect"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/phasemeter/phase"
	"github.com/agenticlab-ai/pyxis/internal/phasemeter/record"
	"github.com/agenticlab-ai/pyxis/internal/phasemeter/spend"
)

// fakeSource is the in-memory stand-in design §3.13 calls for: it satisfies
// spend.Source without touching any harness-specific adapter, so this
// package's own tests never depend on internal/adapters/spend/*.
type fakeSource struct {
	harness string
	turns   []phase.Turn
	byPhase map[string]func(phase.Phase) map[string]record.ModelSpend
}

func (f fakeSource) Harness() string     { return f.harness }
func (f fakeSource) Turns() []phase.Turn { return f.turns }
func (f fakeSource) Spend(p phase.Phase) (map[string]record.ModelSpend, []record.Unavailable) {
	if fn, ok := f.byPhase[p.ID]; ok {
		return fn(p), nil
	}
	return nil, []record.Unavailable{{Harness: f.harness, MissingFigure: "per-model spend"}}
}

func open(skill string) phase.Turn { return phase.Turn{Harness: "claude-code", SkillInvocation: skill} }
func plain() phase.Turn            { return phase.Turn{Harness: "claude-code"} }
func closeTurn() phase.Turn        { return phase.Turn{Harness: "claude-code", ArtifactWrite: true} }

// Records ties phase.Detect and Source.Spend together: one record per
// phase found, stamped with the caller's run/feature id and the Source's
// own harness, carrying whatever Spend that phase's Source.Spend call
// returned.
func TestRecords_OnePerPhase_StampedAndSpendJoined(t *testing.T) {
	quartetA := record.TokenQuartet{Input: 100, Output: 20, CacheCreation: 5, CacheRead: 50}
	quartetB := record.TokenQuartet{Input: 7, Output: 3}
	src := fakeSource{
		harness: "claude-code",
		turns: []phase.Turn{
			open("prd-new"), plain(), closeTurn(),
			open("prd-review"), plain(), closeTurn(),
		},
		byPhase: map[string]func(phase.Phase) map[string]record.ModelSpend{
			"prd-new": func(phase.Phase) map[string]record.ModelSpend {
				return map[string]record.ModelSpend{"claude-sonnet-5": {TokenQuartet: quartetA}}
			},
			"prd-review": func(phase.Phase) map[string]record.ModelSpend {
				return map[string]record.ModelSpend{"claude-opus-4-8": {TokenQuartet: quartetB}}
			},
		},
	}

	got := spend.Records(src, "run-1", "pipeline-measurement")

	if len(got) != 2 {
		t.Fatalf("want 2 records, got %d: %+v", len(got), got)
	}
	want := []record.Record{
		{
			SchemaVersion: record.SchemaVersion,
			RunID:         "run-1",
			FeatureID:     "pipeline-measurement",
			PhaseID:       "prd-new",
			Harness:       "claude-code",
			Spend:         map[string]record.ModelSpend{"claude-sonnet-5": {TokenQuartet: quartetA}},
		},
		{
			SchemaVersion: record.SchemaVersion,
			RunID:         "run-1",
			FeatureID:     "pipeline-measurement",
			PhaseID:       "prd-review",
			Harness:       "claude-code",
			Spend:         map[string]record.ModelSpend{"claude-opus-4-8": {TokenQuartet: quartetB}},
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Records mismatch:\nwant %+v\ngot  %+v", want, got)
	}
}

// Acceptance (DINV-3): a phase this Source could capture no usage for
// carries a declared Unavailable, never a silent zero — Spend stays nil,
// not an empty-but-present map.
func TestRecords_UnavailablePhase_NeverASilentZero(t *testing.T) {
	src := fakeSource{
		harness: "claude-code",
		turns:   []phase.Turn{open("tech-design-new"), plain(), closeTurn()},
		byPhase: map[string]func(phase.Phase) map[string]record.ModelSpend{},
	}

	got := spend.Records(src, "run-1", "pipeline-measurement")

	if len(got) != 1 {
		t.Fatalf("want 1 record, got %d", len(got))
	}
	r := got[0]
	if r.Spend != nil {
		t.Errorf("want a nil Spend map for an uncapturable phase, got %+v", r.Spend)
	}
	want := []record.Unavailable{{Harness: "claude-code", MissingFigure: "per-model spend"}}
	if !reflect.DeepEqual(r.Unavailable, want) {
		t.Errorf("want Unavailable %+v, got %+v", want, r.Unavailable)
	}
}

// Acceptance (T5, DINV-3, prd:pipeline-measurement ABUSE-1): the port must
// not simply trust a Source — a flawed adapter that reports a zero-valued
// token quartet (indistinguishable from "never measured") for a model,
// without declaring it Unavailable itself, must still never let that zero
// reach the record as if it were real spend. Records itself guards the
// figure out into a declared Unavailable naming the harness and the model.
func TestRecords_ZeroValuedSpend_GuardedAsUnavailable(t *testing.T) {
	src := fakeSource{
		harness: "claude-code",
		turns:   []phase.Turn{open("prd-new"), plain(), closeTurn()},
		byPhase: map[string]func(phase.Phase) map[string]record.ModelSpend{
			"prd-new": func(phase.Phase) map[string]record.ModelSpend {
				return map[string]record.ModelSpend{"claude-sonnet-5": {}} // a flawed adapter's silent zero
			},
		},
	}

	got := spend.Records(src, "run-1", "pipeline-measurement")

	if len(got) != 1 {
		t.Fatalf("want 1 record, got %d", len(got))
	}
	r := got[0]
	if r.Spend != nil {
		t.Errorf("want a nil Spend map — the zero-valued entry must be pulled out, got %+v", r.Spend)
	}
	want := []record.Unavailable{{Harness: "claude-code", MissingFigure: "per-model spend (claude-sonnet-5)"}}
	if !reflect.DeepEqual(r.Unavailable, want) {
		t.Errorf("want Unavailable %+v, got %+v", want, r.Unavailable)
	}
}

// A phase using more than one model, where only one of them is zero-valued,
// must keep the real model's figure and guard out only the zero one — the
// guard is per-model, never an all-or-nothing wipe of the whole phase.
func TestRecords_MixedZeroAndReal_OnlyZeroGuarded(t *testing.T) {
	real := record.TokenQuartet{Input: 100, Output: 20}
	src := fakeSource{
		harness: "claude-code",
		turns:   []phase.Turn{open("plan-new"), plain(), closeTurn()},
		byPhase: map[string]func(phase.Phase) map[string]record.ModelSpend{
			"plan-new": func(phase.Phase) map[string]record.ModelSpend {
				return map[string]record.ModelSpend{
					"claude-sonnet-5":  {TokenQuartet: real},
					"claude-haiku-4-5": {},
				}
			},
		},
	}

	got := spend.Records(src, "run-1", "pipeline-measurement")

	r := got[0]
	wantSpend := map[string]record.ModelSpend{"claude-sonnet-5": {TokenQuartet: real}}
	if !reflect.DeepEqual(r.Spend, wantSpend) {
		t.Errorf("want Spend %+v (real model kept, zero model removed), got %+v", wantSpend, r.Spend)
	}
	wantUnavailable := []record.Unavailable{{Harness: "claude-code", MissingFigure: "per-model spend (claude-haiku-4-5)"}}
	if !reflect.DeepEqual(r.Unavailable, wantUnavailable) {
		t.Errorf("want Unavailable %+v, got %+v", wantUnavailable, r.Unavailable)
	}
}

// Multiple zero-valued models in one phase must each get their own named
// entry, in a deterministic (sorted) order — map iteration order must never
// leak into the written record.
func TestRecords_MultipleZeroModels_EachNamedDeterministically(t *testing.T) {
	src := fakeSource{
		harness: "claude-code",
		turns:   []phase.Turn{open("tech-design-new"), plain(), closeTurn()},
		byPhase: map[string]func(phase.Phase) map[string]record.ModelSpend{
			"tech-design-new": func(phase.Phase) map[string]record.ModelSpend {
				return map[string]record.ModelSpend{"claude-sonnet-5": {}, "claude-opus-4-8": {}}
			},
		},
	}

	got := spend.Records(src, "run-1", "pipeline-measurement")

	r := got[0]
	if r.Spend != nil {
		t.Errorf("want a nil Spend map, got %+v", r.Spend)
	}
	want := []record.Unavailable{
		{Harness: "claude-code", MissingFigure: "per-model spend (claude-opus-4-8)"},
		{Harness: "claude-code", MissingFigure: "per-model spend (claude-sonnet-5)"},
	}
	if !reflect.DeepEqual(r.Unavailable, want) {
		t.Errorf("want deterministically-ordered Unavailable %+v, got %+v", want, r.Unavailable)
	}
}

// Acceptance (T5): a Source that captured nothing at all for a phase — an
// empty (but non-nil) Spend map and no declared Unavailable, a flawed
// adapter forgetting DINV-3's own discipline — must still never read as "no
// spend, nothing missing": the port itself declares the phase unavailable
// rather than letting an all-silent phase pass through unflagged.
func TestRecords_EmptySpendNoDeclaration_PortSynthesizesUnavailable(t *testing.T) {
	src := fakeSource{
		harness: "claude-code",
		turns:   []phase.Turn{open("prd-review"), plain(), closeTurn()},
		byPhase: map[string]func(phase.Phase) map[string]record.ModelSpend{
			"prd-review": func(phase.Phase) map[string]record.ModelSpend {
				return map[string]record.ModelSpend{} // a flawed adapter: nothing captured, nothing declared
			},
		},
	}

	got := spend.Records(src, "run-1", "pipeline-measurement")

	r := got[0]
	if r.Spend != nil {
		t.Errorf("want a nil Spend map, got %+v", r.Spend)
	}
	if len(r.Unavailable) != 1 || r.Unavailable[0].Harness != "claude-code" {
		t.Errorf("want exactly one Unavailable naming claude-code, got %+v", r.Unavailable)
	}
}

// An aborted phase (no close marker) must still yield its own record —
// Records must never drop it, carrying UB-3's "never dropped" through to
// the record-write path.
func TestRecords_IncompletePhase_StillYieldsARecord(t *testing.T) {
	quartet := record.TokenQuartet{Input: 1, Output: 1}
	src := fakeSource{
		harness: "claude-code",
		turns:   []phase.Turn{open("plan-new"), plain()}, // no close
		byPhase: map[string]func(phase.Phase) map[string]record.ModelSpend{
			"plan-new": func(phase.Phase) map[string]record.ModelSpend {
				return map[string]record.ModelSpend{"claude-sonnet-5": {TokenQuartet: quartet}}
			},
		},
	}

	got := spend.Records(src, "run-1", "pipeline-measurement")

	if len(got) != 1 {
		t.Fatalf("an incomplete phase must still be recorded, not dropped: got %d records", len(got))
	}
	if got[0].PhaseID != "plan-new" {
		t.Errorf("want phase_id plan-new, got %q", got[0].PhaseID)
	}
}

// No phases found (an empty or boundary-free record) yields no records —
// never a fabricated one.
func TestRecords_NoPhases_YieldsNoRecords(t *testing.T) {
	src := fakeSource{harness: "claude-code", turns: []phase.Turn{plain(), plain()}}
	got := spend.Records(src, "run-1", "pipeline-measurement")
	if len(got) != 0 {
		t.Fatalf("want 0 records, got %d: %+v", len(got), got)
	}
}
