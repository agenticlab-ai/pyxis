package state_test

import (
	"context"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/lane/acting"
	"github.com/agenticlab-ai/pyxis/internal/lane/classifier"
	"github.com/agenticlab-ai/pyxis/internal/lane/policy"
	"github.com/agenticlab-ai/pyxis/internal/lane/state"
	"github.com/agenticlab-ai/pyxis/internal/lane/verdict"
)

// These tests compose the lane-state record (T5) with the frozen acting adapter
// (T4): the halt this package writes is read at the next act gate through the
// adapter's Preconditions.Halted, and a kill trip's directive resolves the
// adapter Killed. They verify the seam T4 left — Preconditions.Halted and the
// p.KillSwitch state — is exactly the one T5 fills.

// --- the acting adapter's read/act seams, filled locally ---

type composeMedium struct {
	head   string
	green  bool
	review verdict.CheckState
	merged bool
}

func (m *composeMedium) Head(context.Context, string) (string, error) { return m.head, nil }
func (m *composeMedium) AllRequiredGreen(context.Context, string, string) (bool, error) {
	return m.green, nil
}
func (m *composeMedium) ReviewVerdict(context.Context, string, string) (verdict.CheckState, error) {
	return m.review, nil
}
func (m *composeMedium) MergedIntoMainline(context.Context, string, string) (bool, error) {
	return m.merged, nil
}

type composeRecorder struct{ n int }

func (r *composeRecorder) RecordVerdict(context.Context, string, verdict.Record) error {
	r.n++
	return nil
}

type composeMachine struct{ acts int }

func (m *composeMachine) Identity() acting.Identity {
	return acting.Identity{Name: "pyxis-lane-bot", Machine: true}
}
func (m *composeMachine) Act(context.Context, string, string) error { m.acts++; return nil }

func enforcingPolicy(kill bool) *policy.Policy {
	return &policy.Policy{
		SchemaVersion: policy.SchemaVersion,
		RatifiedBy:    "@mdarabi",
		Floor: policy.Floor{
			HardGatedPaths:   policy.SelfUnlockMinPaths,
			HardGatedClasses: policy.SelfUnlockMinClasses,
		},
		AutoClasses: []policy.AutoClass{{Name: "fleet-task", Paths: []string{"internal/carina/**"}}},
		Activation:  policy.Activation{Enforce: true, BackstopChangeRef: "refs/backstop@1"},
		KillSwitch:  kill,
	}
}

func autoEvent(proposal string) acting.Event {
	return acting.Event{
		Proposal:      proposal,
		Diff:          classifier.Diff{Paths: []string{"internal/carina/foo.go"}},
		Base:          "base-sha",
		Head:          "head-sha",
		CheckSnapshot: "snap-1",
	}
}

// A-6 end-to-end: an act whose goal verification fails is halted through the
// lane-state record, and the NEXT act gate — resolving state from the record —
// refuses to act. The halt written synchronously by T5 is read at every
// subsequent act gate (DINV-7).
func TestCompose_HaltWrittenIsReadAtNextActGate(t *testing.T) {
	ctx := context.Background()
	store := &memStore{writer: machineWriter}
	p := enforcingPolicy(false)

	// Event 1: the act completes but the badge lied (merged=false) → the adapter
	// signals a halt.
	med := &composeMedium{head: "head-sha", green: true, merged: false}
	mach := &composeMachine{}
	a := &acting.Adapter{Medium: med, Recorder: &composeRecorder{}, Machine: mach}
	res1, err := a.Handle(ctx, autoEvent("p1"), p, acting.Preconditions{RefProtectionPresent: true})
	if err != nil {
		t.Fatalf("event 1: %v", err)
	}
	if !res1.Acted || !res1.Halt {
		t.Fatalf("event 1 did not act-then-halt: acted=%v halt=%v", res1.Acted, res1.Halt)
	}

	// T5 persists the halt synchronously (the acting workflow step's write).
	if err := state.WriteHalt(ctx, store, &memAlarm{}, "docs/problems/x/delivery.md", "p1", res1.Act.HeadRevision, "goal-verify failed"); err != nil {
		t.Fatalf("WriteHalt: %v", err)
	}

	// Event 2: the next act gate resolves lane state from the record FIRST.
	b, w, _ := store.ReadState(ctx)
	sr, err := state.Resolve(ctx, b, w, newDisp())
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if !sr.Halted {
		t.Fatal("the persisted halt did not resolve halted")
	}

	// Feed the resolved halt into the adapter's precondition: a clean AUTO
	// proposal is now refused, no act.
	med2 := &composeMedium{head: "head-sha", green: true, merged: true}
	mach2 := &composeMachine{}
	a2 := &acting.Adapter{Medium: med2, Recorder: &composeRecorder{}, Machine: mach2}
	res2, err := a2.Handle(ctx, autoEvent("p2"), p, acting.Preconditions{RefProtectionPresent: true, Halted: sr.Halted})
	if err != nil {
		t.Fatalf("event 2: %v", err)
	}
	if res2.State != acting.Halted || res2.Acted || mach2.acts != 0 {
		t.Fatalf("the next act gate acted despite the halt: state=%q acted=%v acts=%d", res2.State, res2.Acted, mach2.acts)
	}
}

// A cleared halt (a machine-written clear citing a recorded human disposition)
// lets the next act gate act again — the circuit breaker is resettable only the
// authorized way.
func TestCompose_ValidClearReenablesActs(t *testing.T) {
	ctx := context.Background()
	store := &memStore{writer: machineWriter}
	if err := state.WriteHalt(ctx, store, &memAlarm{}, "docs/problems/x/delivery.md", "p1", "head-sha", "badge lied"); err != nil {
		t.Fatalf("WriteHalt: %v", err)
	}
	if err := store.AppendState(ctx, state.Clear(disp7)); err != nil {
		t.Fatalf("append clear: %v", err)
	}

	b, w, _ := store.ReadState(ctx)
	sr, err := state.Resolve(ctx, b, w, newDisp(disp7))
	if err != nil || sr.Halted {
		t.Fatalf("a valid clear did not un-halt: halted=%v err=%v", sr.Halted, err)
	}

	med := &composeMedium{head: "head-sha", green: true, merged: true}
	mach := &composeMachine{}
	a := &acting.Adapter{Medium: med, Recorder: &composeRecorder{}, Machine: mach}
	res, err := a.Handle(ctx, autoEvent("p2"), enforcingPolicy(false), acting.Preconditions{RefProtectionPresent: true, Halted: sr.Halted})
	if err != nil {
		t.Fatalf("handle after clear: %v", err)
	}
	if !res.Acted || res.State != acting.Enforcing {
		t.Fatalf("the lane did not resume after an authorized clear: state=%q acted=%v", res.State, res.Acted)
	}
}

// A-10 end-to-end: after a floor-violation kill trip, the directive flips the
// policy's kill switch, the adapter resolves Killed and routes every proposal
// HUMAN, and re-activation is refused until a fresh ratified policy clears it.
func TestCompose_KillTripResolvesKilled(t *testing.T) {
	ctx := context.Background()
	root, cfg := incidentFixture(t)

	trip, defects := state.TripKill(root, cfg, &memAlarm{}, floorViolation())
	if len(defects) != 0 {
		t.Fatalf("TripKill: %+v", defects)
	}

	// Apply the directive to the policy the lane loads (the single data change).
	p := enforcingPolicy(false)
	p.KillSwitch = trip.Directive.KillSwitch

	med := &composeMedium{head: "head-sha", green: true, merged: true}
	mach := &composeMachine{}
	a := &acting.Adapter{Medium: med, Recorder: &composeRecorder{}, Machine: mach}
	res, err := a.Handle(ctx, autoEvent("p2"), p, acting.Preconditions{RefProtectionPresent: true})
	if err != nil {
		t.Fatalf("handle under kill: %v", err)
	}
	if res.State != acting.Killed || res.Acted || mach.acts != 0 {
		t.Fatalf("the kill trip did not stop acts: state=%q acted=%v", res.State, res.Acted)
	}
	if res.Verdict.Lane != classifier.Human {
		t.Fatalf("the kill did not route HUMAN: %q", res.Verdict.Lane)
	}

	// Re-activation is refused while killed.
	if refused, _ := state.ReactivationRefused(p); !refused {
		t.Fatal("a killed lane was not refused re-activation")
	}
}
