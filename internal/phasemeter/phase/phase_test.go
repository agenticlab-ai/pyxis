package phase_test

import (
	"reflect"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/phasemeter/phase"
)

func open(harness, skill string) phase.Turn {
	return phase.Turn{Harness: harness, SkillInvocation: skill}
}

func plain(harness string) phase.Turn {
	return phase.Turn{Harness: harness}
}

func closeTurn(harness string) phase.Turn {
	return phase.Turn{Harness: harness, ArtifactWrite: true}
}

func phaseIDs(phases []phase.Phase) []string {
	ids := make([]string, len(phases))
	for i, p := range phases {
		ids[i] = p.ID
	}
	return ids
}

// Acceptance (T2/A-6): "A nested-invocation fixture yields distinct phase
// brackets with no spend blended into the parent." prd-new invokes the
// sub-skill prd-review; the parent's turns must exclude every turn the
// nested phase accrued.
func TestNestedInvocation_NoBlendingIntoParent(t *testing.T) {
	t1 := open("claude-code", "prd-new")
	t2 := plain("claude-code")
	t3 := open("claude-code", "prd-review")
	t4 := plain("claude-code")
	t5 := closeTurn("claude-code")
	t6 := plain("claude-code")
	t7 := closeTurn("claude-code")

	got := phase.Detect([]phase.Turn{t1, t2, t3, t4, t5, t6, t7})

	if len(got) != 2 {
		t.Fatalf("want 2 phases (parent + nested), got %d: %+v", len(got), got)
	}
	parent, nested := got[0], got[1]

	if parent.ID != "prd-new" || nested.ID != "prd-review" {
		t.Fatalf("want IDs [prd-new prd-review], got %v", phaseIDs(got))
	}
	if !parent.Complete || !nested.Complete {
		t.Fatalf("both phases closed with an artifact-write marker; want both Complete, got parent=%v nested=%v", parent.Complete, nested.Complete)
	}

	wantParentTurns := []phase.Turn{t1, t2, t6, t7}
	if !reflect.DeepEqual(parent.Turns, wantParentTurns) {
		t.Errorf("parent.Turns must exclude the nested bracket entirely:\nwant %+v\ngot  %+v", wantParentTurns, parent.Turns)
	}
	wantNestedTurns := []phase.Turn{t3, t4, t5}
	if !reflect.DeepEqual(nested.Turns, wantNestedTurns) {
		t.Errorf("nested.Turns mismatch:\nwant %+v\ngot  %+v", wantNestedTurns, nested.Turns)
	}
}

// Acceptance (T2/A-12): "An aborted phase (no close marker) is recorded
// incomplete, never dropped or merged into the next."
func TestAbortedPhase_RecordedIncompleteNeverDropped(t *testing.T) {
	t1 := open("claude-code", "tech-design-new")
	t2 := plain("claude-code")
	t3 := plain("claude-code")

	got := phase.Detect([]phase.Turn{t1, t2, t3})

	if len(got) != 1 {
		t.Fatalf("an aborted phase must still be recorded, not dropped: got %d phases", len(got))
	}
	p := got[0]
	if p.Complete {
		t.Errorf("phase with no close marker must be Complete == false, got true")
	}
	want := []phase.Turn{t1, t2, t3}
	if !reflect.DeepEqual(p.Turns, want) {
		t.Errorf("an incomplete phase must still carry every turn it accrued:\nwant %+v\ngot  %+v", want, p.Turns)
	}
}

// A phase that never closes can still have a nested child open and close
// inside it (the stack has no "end of record" signal to force a close). The
// parent's pre-nesting turns must stay exactly as they were — never merged
// with the child's — and the parent itself must still surface as its own
// incomplete phase, not be dropped just because something nested inside it.
func TestAbortedPhase_WithNestedChildStillClosingInside(t *testing.T) {
	openParent := open("claude-code", "plan-new")
	parentBody := plain("claude-code")
	openChild := open("claude-code", "prd-review")
	childBody := plain("claude-code")
	closeChild := closeTurn("claude-code")
	// no close for the parent — the record ends here, aborted

	got := phase.Detect([]phase.Turn{openParent, parentBody, openChild, childBody, closeChild})

	if want := []string{"plan-new", "prd-review"}; !reflect.DeepEqual(phaseIDs(got), want) {
		t.Fatalf("want phases %v, got %v", want, phaseIDs(got))
	}
	parent, child := got[0], got[1]
	if parent.Complete {
		t.Errorf("parent never got a close marker; want Complete == false, got true")
	}
	if !child.Complete {
		t.Errorf("child closed properly; want Complete == true, got false")
	}
	if want := []phase.Turn{openParent, parentBody}; !reflect.DeepEqual(parent.Turns, want) {
		t.Errorf("parent.Turns must exclude the nested child:\nwant %+v\ngot  %+v", want, parent.Turns)
	}
	if want := []phase.Turn{openChild, childBody, closeChild}; !reflect.DeepEqual(child.Turns, want) {
		t.Errorf("child.Turns mismatch:\nwant %+v\ngot  %+v", want, child.Turns)
	}
}

// Acceptance (T2/A-13): "A harness-switch fixture yields records each
// naming the harness that ran that phase."
func TestHarnessSwitch_EachPhaseNamesItsOwnHarness(t *testing.T) {
	turns := []phase.Turn{
		open("claude-code", "prd-new"),
		plain("claude-code"),
		closeTurn("claude-code"),
		open("codex", "tech-design-new"),
		plain("codex"),
		closeTurn("codex"),
	}

	got := phase.Detect(turns)

	if len(got) != 2 {
		t.Fatalf("want 2 phases, got %d", len(got))
	}
	if got[0].Harness != "claude-code" {
		t.Errorf("phase 0: want harness claude-code, got %q", got[0].Harness)
	}
	if got[1].Harness != "codex" {
		t.Errorf("phase 1: want harness codex, got %q", got[1].Harness)
	}
	for i, p := range got {
		for _, tn := range p.Turns {
			if tn.Harness != p.Harness {
				t.Errorf("phase %d (harness %q) contains a turn from harness %q — spend must never blend across harnesses", i, p.Harness, tn.Harness)
			}
		}
	}
}

// A turn before the first open marker, or after the outermost phase has
// closed, belongs to no phase — attributed to none, never mis-attributed to
// a neighboring phase (design §3.8; FR-3's fit criterion).
func TestTurnOutsideAnyPhase_AttributedToNone(t *testing.T) {
	stray1 := plain("claude-code")
	t1 := open("claude-code", "prd-new")
	t2 := plain("claude-code")
	t3 := closeTurn("claude-code")
	stray2 := plain("claude-code")

	got := phase.Detect([]phase.Turn{stray1, t1, t2, t3, stray2})

	if len(got) != 1 {
		t.Fatalf("want exactly 1 phase, got %d", len(got))
	}
	want := []phase.Turn{t1, t2, t3}
	if !reflect.DeepEqual(got[0].Turns, want) {
		t.Errorf("stray turns outside the boundary must not be attributed:\nwant %+v\ngot  %+v", want, got[0].Turns)
	}
}

// A close marker arriving with nothing open has no phase to pop — it must
// not panic and must not fabricate a phase.
func TestCloseWithNothingOpen_IsANoOp(t *testing.T) {
	got := phase.Detect([]phase.Turn{closeTurn("claude-code"), plain("claude-code")})
	if len(got) != 0 {
		t.Fatalf("want 0 phases from a record with no open marker, got %d: %+v", len(got), got)
	}
}

// Two phases opened and closed one after another (not nested) must yield
// two independent brackets, each excluding the other's turns.
func TestSequentialPhases_EachExcludesTheOther(t *testing.T) {
	a1 := open("claude-code", "feature-brainstorm")
	a2 := plain("claude-code")
	a3 := closeTurn("claude-code")
	b1 := open("claude-code", "prd-new")
	b2 := plain("claude-code")
	b3 := closeTurn("claude-code")

	got := phase.Detect([]phase.Turn{a1, a2, a3, b1, b2, b3})

	if len(got) != 2 {
		t.Fatalf("want 2 phases, got %d", len(got))
	}
	if !reflect.DeepEqual(got[0].Turns, []phase.Turn{a1, a2, a3}) {
		t.Errorf("phase 0 turns mismatch: %+v", got[0].Turns)
	}
	if !reflect.DeepEqual(got[1].Turns, []phase.Turn{b1, b2, b3}) {
		t.Errorf("phase 1 turns mismatch: %+v", got[1].Turns)
	}
	if !got[0].Complete || !got[1].Complete {
		t.Errorf("both sequential phases closed; want both Complete")
	}
}

// Three levels deep: each level's turns must exclude every level nested
// inside it, and Phases orders brackets by open order (outermost first).
func TestDeeplyNested_ThreeLevels(t *testing.T) {
	outer := open("claude-code", "outer")
	mid := open("claude-code", "mid")
	inner := open("claude-code", "inner")
	innerBody := plain("claude-code")
	closeInner := closeTurn("claude-code")
	midBody := plain("claude-code")
	closeMid := closeTurn("claude-code")
	outerBody := plain("claude-code")
	closeOuter := closeTurn("claude-code")

	got := phase.Detect([]phase.Turn{
		outer, mid, inner, innerBody, closeInner, midBody, closeMid, outerBody, closeOuter,
	})

	if want := []string{"outer", "mid", "inner"}; !reflect.DeepEqual(phaseIDs(got), want) {
		t.Fatalf("want phases in open order %v, got %v", want, phaseIDs(got))
	}
	for _, p := range got {
		if !p.Complete {
			t.Errorf("phase %q: want Complete, got incomplete", p.ID)
		}
	}
	byID := map[string]phase.Phase{}
	for _, p := range got {
		byID[p.ID] = p
	}
	if want := []phase.Turn{outer, outerBody, closeOuter}; !reflect.DeepEqual(byID["outer"].Turns, want) {
		t.Errorf("outer.Turns must exclude mid and inner entirely:\nwant %+v\ngot  %+v", want, byID["outer"].Turns)
	}
	if want := []phase.Turn{mid, midBody, closeMid}; !reflect.DeepEqual(byID["mid"].Turns, want) {
		t.Errorf("mid.Turns must exclude inner and outer:\nwant %+v\ngot  %+v", want, byID["mid"].Turns)
	}
	if want := []phase.Turn{inner, innerBody, closeInner}; !reflect.DeepEqual(byID["inner"].Turns, want) {
		t.Errorf("inner.Turns mismatch:\nwant %+v\ngot  %+v", want, byID["inner"].Turns)
	}
}

// Stack is a live detector, not a batch-only helper: Phases() reads a
// snapshot at any point in the push sequence, and reflects new state (e.g.
// Complete flipping true) on a later call.
func TestStack_PhasesIsALiveSnapshot(t *testing.T) {
	var s phase.Stack
	t1 := open("claude-code", "prd-new")
	t2 := plain("claude-code")
	s.Push(t1)
	s.Push(t2)

	mid := s.Phases()
	if len(mid) != 1 || mid[0].Complete {
		t.Fatalf("mid-stream: want 1 incomplete phase, got %+v", mid)
	}
	if !reflect.DeepEqual(mid[0].Turns, []phase.Turn{t1, t2}) {
		t.Fatalf("mid-stream turns mismatch: %+v", mid[0].Turns)
	}

	t3 := closeTurn("claude-code")
	s.Push(t3)
	final := s.Phases()
	if len(final) != 1 || !final[0].Complete {
		t.Fatalf("after close: want 1 complete phase, got %+v", final)
	}

	// The earlier snapshot must not have been mutated by the later Push.
	if mid[0].Complete {
		t.Errorf("an earlier Phases() snapshot must not retroactively flip Complete")
	}
}

// A spend-source adapter stamps Ordinal on each Turn it builds so it can
// later correlate a Phase's Turns back to its own per-turn usage data
// (design §3.7 S-2, §3.8). This package must carry that field through
// Push/Phases untouched — it neither reads nor requires it.
func TestOrdinal_CarriedThroughUntouched(t *testing.T) {
	t1 := phase.Turn{Harness: "claude-code", SkillInvocation: "prd-new", Ordinal: 10}
	t2 := phase.Turn{Harness: "claude-code", Ordinal: 11}
	t3 := phase.Turn{Harness: "claude-code", ArtifactWrite: true, Ordinal: 12}

	got := phase.Detect([]phase.Turn{t1, t2, t3})

	if len(got) != 1 {
		t.Fatalf("want 1 phase, got %d", len(got))
	}
	want := []int{10, 11, 12}
	gotOrdinals := make([]int, len(got[0].Turns))
	for i, tn := range got[0].Turns {
		gotOrdinals[i] = tn.Ordinal
	}
	if !reflect.DeepEqual(gotOrdinals, want) {
		t.Errorf("Ordinal not carried through in order: want %v, got %v", want, gotOrdinals)
	}
}

func TestIsSkillInvocation(t *testing.T) {
	tests := []struct {
		name     string
		turn     phase.Turn
		wantID   string
		wantOpen bool
	}{
		{"skill invocation", open("claude-code", "prd-new"), "prd-new", true},
		{"plain turn", plain("claude-code"), "", false},
		{"close marker", closeTurn("claude-code"), "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id, ok := phase.IsSkillInvocation(tt.turn)
			if ok != tt.wantOpen || id != tt.wantID {
				t.Errorf("IsSkillInvocation(%+v) = (%q, %v), want (%q, %v)", tt.turn, id, ok, tt.wantID, tt.wantOpen)
			}
		})
	}
}

func TestIsArtifactWrite(t *testing.T) {
	tests := []struct {
		name      string
		turn      phase.Turn
		wantClose bool
	}{
		{"artifact write", closeTurn("claude-code"), true},
		{"plain turn", plain("claude-code"), false},
		{"skill invocation", open("claude-code", "prd-new"), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := phase.IsArtifactWrite(tt.turn); got != tt.wantClose {
				t.Errorf("IsArtifactWrite(%+v) = %v, want %v", tt.turn, got, tt.wantClose)
			}
		})
	}
}
