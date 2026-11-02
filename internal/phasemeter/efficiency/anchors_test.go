package efficiency_test

import (
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/phasemeter/efficiency"
)

// brainstormFixture mirrors the literal shape
// .pyxis/templates/brainstorm-record-template.md instantiates: "### D-n —
// <gist>" decision blocks with a "- **Status**:" line, and "- **G-n**
// (...)" fog bullets. D-2 is superseded by D-3.
const brainstormFixture = `## Decisions

### D-1 — the framing

- **Question**: what shape?
- **Answer**: A
- **Reasoning**: because
- **Anchor**: 2026-07-01 · turn 3
- **Status**: live

### D-2 — the scope

- **Question**: how much?
- **Answer**: B
- **Reasoning**: because
- **Anchor**: 2026-07-01 · turn 5
- **Status**: superseded by D-3 — the operator changed the scope

### D-3 — the scope, revised

- **Question**: how much, revised?
- **Answer**: C
- **Reasoning**: the operator reconsidered
- **Anchor**: 2026-07-02 · turn 1
- **Status**: live

## Fog

- **G-1** (blocking — blocks: destination): what about X?
- **G-2** (non-blocking — blocks: design): what about Y?
`

// liveStatusMentioningSupersededFixture mirrors a real, already-committed
// shape (docs/brainstorms/autonomous-developer.md's D-3): the decision's
// own Status is "live", but its trailing narrative mentions "superseded"
// about one of its own sub-rows being revised by a later decision. Status
// is a closed two-way enum (live | superseded by D-m — <reason>), so this
// must not count as a superseded decision.
const liveStatusMentioningSupersededFixture = `## Decisions

### D-1 — the authority perimeter

- **Question**: what may never be ruled?
- **Answer**: a graduated list
- **Reasoning**: because
- **Anchor**: 2026-07-10 · t5
- **Status**: live — row 6 superseded by D-4 (narrowed); rows 1-5 stand as written
`

// designBrainstormFixture pins that DQ-n (design questions) and ALT-n
// (alternatives) headings — both present in
// design-brainstorm-record-template.md alongside D-n — never count as D-n.
const designBrainstormFixture = `## Design questions

- **DQ-1**: which shape?

## Decisions

### D-1 — the direction

- **Question**: which shape?
- **Answer**: the hybrid
- **Reasoning**: because
- **Anchor**: 2026-07-03 · turn 1
- **Status**: live

## Alternatives

### ALT-1 — the pure form

- **Shape**: ...
- **Trade-offs**: ...
- **Principles**: ...
- **Incidents**: none checked
- **Verdict**: rejected — too narrow
- **From**: DQ-1 · panel
`

// reviewFixture mirrors prd-review-report-template.md / its
// design-review-report-template.md sibling: "### FND-n — <gist>" blocks,
// two rounds.
const reviewFixture = `## Round 1

### FND-1 — missing rationale

- **Severity**: advisory (CAL-2)
- **Lens**: contract-quality
- **Evidence**: subject quote — vs — upstream anchor
- **Acceptance condition**: add the rationale
- **Disposition**: applied

### FND-2 — untyped fog

- **Severity**: blocking (CAL-1)
- **Lens**: coherence
- **Evidence**: subject quote — vs — upstream anchor
- **Acceptance condition**: type the fog entry
- **Disposition**: declined — not applicable

## Round 2

### FND-3 — residual gap

- **Severity**: advisory (CAL-2)
- **Lens**: completeness-critic
- **Evidence**: subject quote — vs — upstream anchor
- **Acceptance condition**: fill the gap
- **Disposition**: applied
`

func TestQuestions(t *testing.T) {
	if got := efficiency.Questions([]byte(brainstormFixture)); got != 3 {
		t.Errorf("Questions = %d, want 3", got)
	}
}

func TestQuestionsExcludesDesignQuestionsAndAlternatives(t *testing.T) {
	if got := efficiency.Questions([]byte(designBrainstormFixture)); got != 1 {
		t.Errorf("Questions = %d, want 1 (DQ-n and ALT-n must not count as D-n)", got)
	}
}

func TestFog(t *testing.T) {
	if got := efficiency.Fog([]byte(brainstormFixture)); got != 2 {
		t.Errorf("Fog = %d, want 2", got)
	}
}

func TestSupersededDecisions(t *testing.T) {
	if got := efficiency.SupersededDecisions([]byte(brainstormFixture)); got != 1 {
		t.Errorf("SupersededDecisions = %d, want 1", got)
	}
}

func TestSupersededDecisionsNoneLive(t *testing.T) {
	if got := efficiency.SupersededDecisions([]byte(designBrainstormFixture)); got != 0 {
		t.Errorf("SupersededDecisions = %d, want 0", got)
	}
}

func TestSupersededDecisionsLiveStatusMentioningSupersededElsewhere(t *testing.T) {
	if got := efficiency.SupersededDecisions([]byte(liveStatusMentioningSupersededFixture)); got != 0 {
		t.Errorf("SupersededDecisions = %d, want 0 (a live decision's own status must not match on a substring mention)", got)
	}
}

func TestDefects(t *testing.T) {
	if got := efficiency.Defects([]byte(reviewFixture)); got != 3 {
		t.Errorf("Defects = %d, want 3", got)
	}
}

func TestCountersOnEmptyDoc(t *testing.T) {
	if got := efficiency.Questions(nil); got != 0 {
		t.Errorf("Questions(nil) = %d, want 0", got)
	}
	if got := efficiency.Fog(nil); got != 0 {
		t.Errorf("Fog(nil) = %d, want 0", got)
	}
	if got := efficiency.SupersededDecisions(nil); got != 0 {
		t.Errorf("SupersededDecisions(nil) = %d, want 0", got)
	}
	if got := efficiency.Defects(nil); got != 0 {
		t.Errorf("Defects(nil) = %d, want 0", got)
	}
}
