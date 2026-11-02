package wake

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/docgate/argusledger"
)

func TestTickDefectsNameProbeAndMalformedSlug(t *testing.T) {
	probe := probeReadDefect("delivery.md", argusledger.Await{GoalObject: "goal-1"}, errors.New("offline"))
	bad := badSlugDefect("delivery.md", "Bad Slug")
	if !strings.Contains(probe.Reason, "goal-1") || !strings.Contains(probe.Reason, "offline") {
		t.Fatalf("probe defect = %+v", probe)
	}
	if !strings.Contains(bad.Reason, "Bad Slug") {
		t.Fatalf("slug defect = %+v", bad)
	}
}

func newBackstop(l Ledgers, p Prober, d FrontDoor) *Backstop {
	return &Backstop{Ledgers: l, Prober: p, Door: d}
}

// AC1 (suppressed-signal leg): with push disabled, the backstop delivers an
// awaited completion — one effective front-door touch, bound to the verified
// goal object.
func TestTick_DeliversDueCompletion(t *testing.T) {
	docs := []Doc{deliveryDoc("foo", stageEntered+awaitEntry("E-2", "pr:214", "merged", "2026-07-11"))}
	prober := stubProber{ancestor: map[string]bool{"214": true}}
	door := &recordDoor{}
	rep, err := newBackstop(stubLedgers{docs: docs}, prober, door).Tick(context.Background())
	if err != nil {
		t.Fatalf("Tick: %v", err)
	}
	if len(rep.Touches) != 1 || rep.Touches[0].Signal.Slug != "foo" || rep.Touches[0].Signal.Reason != ReasonMerge {
		t.Fatalf("touches = %+v, want one merge touch on foo", rep.Touches)
	}
	if rep.Touches[0].Source != SourceBackstop || rep.Touches[0].GoalObject != "pr:214" {
		t.Fatalf("touch metadata = %+v, want backstop/pr:214", rep.Touches[0])
	}
	if len(door.touches) != 1 {
		t.Fatalf("front door touched %d times, want 1", len(door.touches))
	}
	if rep.Idle || rep.Probes != 1 {
		t.Fatalf("idle=%v probes=%d, want idle=false probes=1", rep.Idle, rep.Probes)
	}
}

// AC4: an idle tick (no outstanding awaits anywhere) performs zero probes and
// zero touches — its cost is reading the ledgers and exiting.
func TestTick_IdleZeroCost(t *testing.T) {
	docs := []Doc{
		deliveryDoc("foo", stageEntered),
		deliveryDoc("bar", stageEntered),
	}
	door := &recordDoor{}
	rep, err := newBackstop(stubLedgers{docs: docs}, stubProber{}, door).Tick(context.Background())
	if err != nil {
		t.Fatalf("Tick: %v", err)
	}
	if !rep.Idle {
		t.Errorf("idle = false, want true (no awaits registered)")
	}
	if rep.Probes != 0 {
		t.Errorf("probes = %d, want 0 on an idle tick", rep.Probes)
	}
	if len(rep.Touches) != 0 || len(door.touches) != 0 {
		t.Errorf("touches = %d, want 0 on an idle tick", len(door.touches))
	}
}

// AC1 + AC4 (park leg): a parked problem is skipped before any probe — zero
// probes, zero touches, and a recorded park skip (DINV-2).
func TestTick_ParkedDrawsZeroActivity(t *testing.T) {
	// The parked ledger also carries an await, to prove the park exclusion is by
	// construction and does not depend on the await being absent.
	parkedJournal := stageEntered +
		awaitEntry("E-2", "pr:99", "merged", "2026-07-11") +
		"### E-3 — brief-emitted — need a ruling\n\n- **Date**: 2026-07-12\n- **Body**: escalated; class: override\n\n---\n"
	docs := []Doc{deliveryDoc("parked", parkedJournal)}
	// The prober would report pr:99 merged — but the tick must never probe it.
	prober := stubProber{ancestor: map[string]bool{"99": true}}
	door := &recordDoor{}
	rep, err := newBackstop(stubLedgers{docs: docs}, prober, door).Tick(context.Background())
	if err != nil {
		t.Fatalf("Tick: %v", err)
	}
	if rep.Probes != 0 || len(door.touches) != 0 {
		t.Fatalf("parked problem drew probes=%d touches=%d, want 0/0", rep.Probes, len(door.touches))
	}
	if len(rep.Skips) != 1 || rep.Skips[0].Reason != SkipParked {
		t.Fatalf("skips = %+v, want one parked skip", rep.Skips)
	}
}

// A closed ledger is skipped before any probe and never touched (DINV-3).
func TestTick_ClosedSkipped(t *testing.T) {
	closedJournal := stageEntered +
		awaitEntry("E-2", "pr:5", "merged", "2026-07-11") +
		"### E-3 — delivered — done\n\n- **Date**: 2026-07-13\n- **Body**: verified on the mainline\n\n---\n"
	docs := []Doc{deliveryDoc("done", closedJournal)}
	door := &recordDoor{}
	rep, err := newBackstop(stubLedgers{docs: docs}, stubProber{ancestor: map[string]bool{"5": true}}, door).Tick(context.Background())
	if err != nil {
		t.Fatalf("Tick: %v", err)
	}
	if rep.Probes != 0 || len(door.touches) != 0 {
		t.Fatalf("closed ledger drew probes=%d touches=%d, want 0/0", rep.Probes, len(door.touches))
	}
	if len(rep.Skips) != 1 || rep.Skips[0].Reason != SkipClosed {
		t.Fatalf("skips = %+v, want one closed skip", rep.Skips)
	}
}

// An outstanding-but-incomplete await is probed (cost paid) but not delivered.
func TestTick_NotYetCompleteProbesNoTouch(t *testing.T) {
	docs := []Doc{deliveryDoc("foo", stageEntered+awaitEntry("E-2", "pr:214", "merged", "2026-07-11"))}
	door := &recordDoor{}
	rep, err := newBackstop(stubLedgers{docs: docs}, stubProber{ancestor: map[string]bool{"214": false}}, door).Tick(context.Background())
	if err != nil {
		t.Fatalf("Tick: %v", err)
	}
	if rep.Probes != 1 || len(door.touches) != 0 || rep.Idle {
		t.Fatalf("probes=%d touches=%d idle=%v, want 1/0/false", rep.Probes, len(door.touches), rep.Idle)
	}
}

// The tick is a touchpoint: a stale await surfaces the named wake-staleness
// defect from ledger data (DINV-5, via T7's computation).
func TestTick_SurfacesStalenessAtTouchpoint(t *testing.T) {
	docs := []Doc{deliveryDoc("foo", stageEntered+awaitEntry("E-2", "pr:214", "merged", "2026-01-01"))}
	b := newBackstop(stubLedgers{docs: docs}, stubProber{}, &recordDoor{})
	b.Bound = 24 * time.Hour
	b.Now = func() time.Time { return time.Date(2026, 7, 15, 0, 0, 0, 0, time.UTC) }
	rep, err := b.Tick(context.Background())
	if err != nil {
		t.Fatalf("Tick: %v", err)
	}
	if len(rep.Stale) == 0 {
		t.Fatalf("want a wake-staleness defect for an await ~6 months past a 24h bound")
	}
}

// AC3: racing deliveries for one problem converge to a single effective touch;
// the extras are converged no-ops, never a duplicated external effect (DINV-6).
func TestDeliver_ConvergesToOneEffectiveTouch(t *testing.T) {
	sig, _ := NewSignal("foo", ReasonMerge)
	batch := []dueTouch{
		{Signal: sig, Source: SourceBackstop, GoalObject: "pr:1"},
		{Signal: sig, Source: SourceSignal, GoalObject: "pr:1"}, // the racing push
		{Signal: sig, Source: SourceBackstop, GoalObject: "pr:1"},
	}
	door := &recordDoor{}
	eff, noops, defects := deliver(context.Background(), door, batch)
	if len(defects) != 0 {
		t.Fatalf("unexpected defects: %v", defects)
	}
	if len(eff) != 1 || len(door.touches) != 1 {
		t.Fatalf("effective touches = %d (door hit %d), want exactly 1", len(eff), len(door.touches))
	}
	if len(noops) != 2 {
		t.Fatalf("converged no-ops = %d, want 2", len(noops))
	}
	for _, n := range noops {
		if n.Reason != NoOpConverged {
			t.Errorf("no-op reason = %q, want converged", n.Reason)
		}
	}
}

// A front-door failure for one touch is a named defect, not a hard error — the
// sweep continues and the other problem still delivers (resilient delivery).
func TestTick_FrontDoorFailureIsResilient(t *testing.T) {
	docs := []Doc{
		deliveryDoc("flaky", stageEntered+awaitEntry("E-2", "pr:1", "merged", "2026-07-11")),
		deliveryDoc("good", stageEntered+awaitEntry("E-2", "pr:2", "merged", "2026-07-11")),
	}
	prober := stubProber{ancestor: map[string]bool{"1": true, "2": true}}
	door := &recordDoor{failSlugs: map[string]bool{"flaky": true}}
	rep, err := newBackstop(stubLedgers{docs: docs}, prober, door).Tick(context.Background())
	if err != nil {
		t.Fatalf("Tick returned a hard error for a per-touch failure: %v", err)
	}
	if len(door.touches) != 1 || door.touches[0].Slug != "good" {
		t.Fatalf("door touches = %+v, want only 'good' delivered", door.touches)
	}
	if len(rep.Defects) == 0 {
		t.Fatalf("want a named defect for the failed front-door touch")
	}
}

// Only the ledger source itself failing is a hard error — the tick cannot sweep.
func TestTick_LedgerSourceErrorIsHard(t *testing.T) {
	b := newBackstop(stubLedgers{err: context.DeadlineExceeded}, stubProber{}, &recordDoor{})
	if _, err := b.Tick(context.Background()); err == nil {
		t.Fatal("want a hard error when the ledger source cannot be read")
	}
}
