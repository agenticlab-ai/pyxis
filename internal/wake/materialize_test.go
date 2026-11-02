package wake

import (
	"context"
	"errors"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/docgate/argusledger"
	"github.com/agenticlab-ai/pyxis/internal/docgate/schema"
)

// stubProjector returns successive results for successive Project calls (the
// route-back re-invokes), clamping to the last; calls counts the invocations.
type stubProjector struct {
	results []ProjectionResult
	calls   int
	err     error
}

func (p *stubProjector) Project(context.Context) (ProjectionResult, error) {
	if p.err != nil {
		return ProjectionResult{}, p.err
	}
	i := p.calls
	if i >= len(p.results) {
		i = len(p.results) - 1
	}
	p.calls++
	return p.results[i], nil
}

type stubChecker struct {
	exists map[string]bool
	tag    map[string]bool
	calls  int
	err    error
}

func (c *stubChecker) Verify(_ context.Context, ref string) (bool, bool, error) {
	c.calls++
	if c.err != nil {
		return false, false, c.err
	}
	return c.exists[ref], c.tag[ref], nil
}

// entriesByType counts a result's journal entries by type.
func entriesByType(r MaterializeResult) map[string]int {
	m := map[string]int{}
	for _, e := range r.Entries {
		m[e.Type]++
	}
	return m
}

func planFreeze() PlanFreeze {
	return PlanFreeze{Slug: "argus-unattended-drive", PlanRef: "pr:292", Plan: "argus-unattended-drive/001", Since: "2026-07-14"}
}

// AC1: a plan frozen at its merge projects through the stage's own contract with
// no human prompt; every created item is verified at the goal object and
// journaled with its trigger recorded.
func TestMaterialize_ProjectsAndVerifiesNoPrompt(t *testing.T) {
	prober := stubProber{ancestor: map[string]bool{"292": true}}
	projector := &stubProjector{results: []ProjectionResult{{Items: []ProjectedItem{
		{PlanTaskID: "f/001/T1", Ref: "item:1", Created: true},
		{PlanTaskID: "f/001/T2", Ref: "item:2", Created: true},
	}}}}
	checker := &stubChecker{exists: map[string]bool{"item:1": true, "item:2": true}, tag: map[string]bool{"item:1": true, "item:2": true}}

	res, err := Materialize(context.Background(), planFreeze(), prober, projector, checker)
	if err != nil {
		t.Fatalf("Materialize: %v", err)
	}
	if res.Outcome != OutcomeMaterialized {
		t.Fatalf("outcome = %q, want materialized", res.Outcome)
	}
	if projector.calls != 1 {
		t.Fatalf("projector called %d times, want 1 (no prompt, no re-trigger)", projector.calls)
	}
	byType := entriesByType(res)
	if byType[argusledger.TypeRulingRecorded] != 1 || byType[argusledger.TypeItemMaterialized] != 2 {
		t.Fatalf("entries = %v, want 1 ruling-recorded + 2 item-materialized", byType)
	}
	// The trigger records trigger=wake; each confirmation records tag=verified.
	for _, e := range res.Entries {
		g, defects := argusledger.ParseMaterializationGist(verb, "f", "E", e.Gist)
		if len(defects) != 0 {
			t.Fatalf("entry gist %q did not parse: %v", e.Gist, defects)
		}
		if g.Trigger != argusledger.TriggerWake {
			t.Errorf("entry %q trigger = %q, want wake", e.Type, g.Trigger)
		}
		if e.Type == argusledger.TypeItemMaterialized && g.Tag != argusledger.TagVerified {
			t.Errorf("confirmation %q tag = %q, want verified", e.Gist, g.Tag)
		}
	}
}

// A premature or forged plan-freeze wake finds the merge unverified at the goal
// object and triggers nothing.
func TestMaterialize_RefusedWhenNotMerged(t *testing.T) {
	prober := stubProber{ancestor: map[string]bool{"292": false}}
	projector := &stubProjector{results: []ProjectionResult{{}}}
	res, err := Materialize(context.Background(), planFreeze(), prober, projector, &stubChecker{})
	if err != nil {
		t.Fatalf("Materialize: %v", err)
	}
	if res.Outcome != OutcomeRefused || len(res.Entries) != 0 {
		t.Fatalf("res = %+v, want refused with no entries", res)
	}
	if projector.calls != 0 {
		t.Fatalf("projector called %d times on an unverified freeze, want 0", projector.calls)
	}
}

// AC2 (no-op leg): a re-trigger with the items already live records a journaled
// no-op and creates nothing — zero duplicates, and the item verifier is never
// called (nothing was created).
func TestMaterialize_ReTriggerIsJournaledNoOp(t *testing.T) {
	prober := stubProber{ancestor: map[string]bool{"292": true}}
	projector := &stubProjector{results: []ProjectionResult{{Items: []ProjectedItem{
		{PlanTaskID: "f/001/T1", Ref: "item:1", Created: false},
		{PlanTaskID: "f/001/T2", Ref: "item:2", Created: false},
	}}}}
	checker := &stubChecker{}
	res, err := Materialize(context.Background(), planFreeze(), prober, projector, checker)
	if err != nil {
		t.Fatalf("Materialize: %v", err)
	}
	if res.Outcome != OutcomeNoOp {
		t.Fatalf("outcome = %q, want no-op", res.Outcome)
	}
	if entriesByType(res)[argusledger.TypeItemMaterialized] != 0 {
		t.Fatalf("a no-op created item-materialized entries: %v", res.Entries)
	}
	if checker.calls != 0 {
		t.Fatalf("item verifier called %d times on a no-op, want 0", checker.calls)
	}
	if len(res.Defects) != 0 {
		t.Fatalf("no-op raised defects: %v", res.Defects)
	}
}

// AC2 (halt leg): a true concurrent both-probe-miss race lands the stage's own
// detected named-both halt — routing withheld, surfaced loudly, no verification,
// no repair, and no serialization added around the stage.
func TestMaterialize_BothMissRaceLandsStageHalt(t *testing.T) {
	prober := stubProber{ancestor: map[string]bool{"292": true}}
	stageDefect := schema.Defect{Verb: "plangate", File: "plan", Op: "duplicate",
		Reason: "T1 exists as #10, #11 — concurrent projections raced", Remedy: "close one by hand and re-run; routing stays withheld for the pair"}
	projector := &stubProjector{results: []ProjectionResult{{
		Items:   []ProjectedItem{{PlanTaskID: "f/001/T1", Ref: "item:1", Duplicate: true}},
		Halted:  true,
		Defects: []schema.Defect{stageDefect},
	}}}
	checker := &stubChecker{}
	res, err := Materialize(context.Background(), planFreeze(), prober, projector, checker)
	if err != nil {
		t.Fatalf("Materialize: %v", err)
	}
	if res.Outcome != OutcomeHalt || !res.Alarm {
		t.Fatalf("res = %+v, want a halt with alarm", res)
	}
	if len(res.Defects) == 0 {
		t.Fatalf("the halt must carry the stage's own defects")
	}
	if projector.calls != 1 {
		t.Fatalf("projector called %d times, want 1 (no repair, no serialization)", projector.calls)
	}
	if checker.calls != 0 {
		t.Fatalf("item verifier called %d times on a halt, want 0 (routing withheld, not verified)", checker.calls)
	}
}

// AC3: a seeded item with the automated-implementation tag absent fails the tag
// verification, journals the failure as a reconciliation, routes back through
// the stage (a re-trigger), and edits no item (structural — there is no
// item-edit port).
func TestMaterialize_TagDiscrepancyRoutesBackThroughStage(t *testing.T) {
	prober := stubProber{ancestor: map[string]bool{"292": true}}
	created := ProjectionResult{Items: []ProjectedItem{{PlanTaskID: "f/001/T1", Ref: "item:9", Created: true}}}
	projector := &stubProjector{results: []ProjectionResult{created, created}}
	// item:9 exists but the tag did not apply.
	checker := &stubChecker{exists: map[string]bool{"item:9": true}, tag: map[string]bool{"item:9": false}}

	res, err := Materialize(context.Background(), planFreeze(), prober, projector, checker)
	if err != nil {
		t.Fatalf("Materialize: %v", err)
	}
	if res.Outcome != OutcomeDiscrepancy || !res.RoutedBack || !res.Alarm {
		t.Fatalf("res = %+v, want a routed-back discrepancy with alarm", res)
	}
	if entriesByType(res)[argusledger.TypeReconciliation] != 1 {
		t.Fatalf("entries = %v, want one reconciliation entry", entriesByType(res))
	}
	if projector.calls != 2 {
		t.Fatalf("projector called %d times, want 2 (the trigger + the route-back through the stage)", projector.calls)
	}
	// The reconciliation records tag=failed.
	for _, e := range res.Entries {
		if e.Type != argusledger.TypeReconciliation {
			continue
		}
		g, _ := argusledger.ParseMaterializationGist(verb, "f", "E", e.Gist)
		if g.Tag != argusledger.TagFailed {
			t.Errorf("reconciliation tag = %q, want failed", g.Tag)
		}
	}
}

// A created item that does not exist at the goal object is a discrepancy too.
func TestMaterialize_MissingItemIsDiscrepancy(t *testing.T) {
	prober := stubProber{ancestor: map[string]bool{"292": true}}
	created := ProjectionResult{Items: []ProjectedItem{{PlanTaskID: "f/001/T1", Ref: "item:9", Created: true}}}
	projector := &stubProjector{results: []ProjectionResult{created, created}}
	checker := &stubChecker{exists: map[string]bool{"item:9": false}, tag: map[string]bool{"item:9": false}}
	res, err := Materialize(context.Background(), planFreeze(), prober, projector, checker)
	if err != nil {
		t.Fatalf("Materialize: %v", err)
	}
	if res.Outcome != OutcomeDiscrepancy || !res.RoutedBack {
		t.Fatalf("res = %+v, want a routed-back discrepancy", res)
	}
}

// An infrastructure fault (a projection call that errored) is a hard error.
func TestMaterialize_ProjectionErrorIsHard(t *testing.T) {
	prober := stubProber{ancestor: map[string]bool{"292": true}}
	projector := &stubProjector{err: errors.New("tracker down")}
	if _, err := Materialize(context.Background(), planFreeze(), prober, projector, &stubChecker{}); err == nil {
		t.Fatal("want a hard error when projection faults")
	}
}

func TestPlanFreezeAwait(t *testing.T) {
	a := PlanFreezeAwait(planFreeze())
	if a.GoalObject != "pr:292" || a.Since != "2026-07-14" || a.Condition == "" {
		t.Fatalf("PlanFreezeAwait = %+v, want the pr:292 merge await", a)
	}
}
