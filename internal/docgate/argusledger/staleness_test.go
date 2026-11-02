package argusledger

import (
	"strings"
	"testing"
	"time"
)

// now is the fixed read moment every staleness test injects — a deterministic
// clock, never the wall clock (the repo Go rules).
var now = time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC)

// An await past its bound surfaces the named wake-staleness defect naming the
// stale await and its age; the same pure computation is what a state read and a
// touchpoint each call (DINV-5; UB-9).
func TestWakeStalenessSurfacesPastBound(t *testing.T) {
	awaits := []Await{{GoalObject: "pr:42", Condition: "merged", Since: "2026-07-10"}} // 5 days before now
	bound := 24 * time.Hour
	defects := WakeStaleness("wake", "docs/problems/foo/delivery.md", awaits, bound, now)
	if len(defects) != 1 {
		t.Fatalf("want one wake-staleness defect, got %d: %v", len(defects), defects)
	}
	d := defects[0]
	if d.Op != "wake-staleness" {
		t.Fatalf("op = %q, want wake-staleness", d.Op)
	}
	if !strings.Contains(d.Reason, "pr:42") {
		t.Fatalf("defect does not name the stale await: %q", d.Reason)
	}
	if !strings.Contains(d.Reason, "5d") {
		t.Fatalf("defect does not name the await's age: %q", d.Reason)
	}
}

// An await within the bound surfaces nothing.
func TestWakeStalenessWithinBound(t *testing.T) {
	awaits := []Await{{GoalObject: "pr:42", Condition: "merged", Since: "2026-07-15"}} // same day → age < 24h
	if defects := WakeStaleness("wake", "docs/problems/foo/delivery.md", awaits, 24*time.Hour, now); len(defects) != 0 {
		t.Fatalf("await within bound flagged: %v", defects)
	}
}

// A mixed set flags only the members past the bound.
func TestWakeStalenessMixedSet(t *testing.T) {
	awaits := []Await{
		{GoalObject: "pr:2", Condition: "landed", Since: "2026-07-01"},            // 14.5 days → stale
		{GoalObject: "item:9", Condition: "answer appended", Since: "2026-07-15"}, // 12h → fresh
	}
	defects := WakeStaleness("wake", "docs/problems/foo/delivery.md", awaits, 24*time.Hour, now)
	if len(defects) != 1 {
		t.Fatalf("want exactly one stale member, got %d: %v", len(defects), defects)
	}
	if !strings.Contains(defects[0].Reason, "pr:2") {
		t.Fatalf("wrong await flagged: %q", defects[0].Reason)
	}
}

// The bound is exclusive: an await whose age equals the bound exactly is NOT
// stale; one an hour older is. since=2026-07-14 at now (12:00) is age 36h.
func TestWakeStalenessBoundExclusive(t *testing.T) {
	awaits := []Await{{GoalObject: "pr:1", Condition: "merged", Since: "2026-07-14"}}
	if d := WakeStaleness("wake", "p", awaits, 36*time.Hour, now); len(d) != 0 {
		t.Fatalf("age exactly at the bound flagged stale: %v", d)
	}
	if d := WakeStaleness("wake", "p", awaits, 35*time.Hour, now); len(d) != 1 {
		t.Fatalf("age past the bound not flagged: %v", d)
	}
}

// An empty await set (an idle drive) is silent — no defect, no panic.
func TestWakeStalenessEmpty(t *testing.T) {
	if defects := WakeStaleness("wake", "docs/problems/foo/delivery.md", nil, time.Hour, now); len(defects) != 0 {
		t.Fatalf("empty set produced defects: %v", defects)
	}
}

// An await whose since-date passes the grammar's digit shape but names a
// nonexistent calendar date is a named defect, not a silent skip.
func TestWakeStalenessUnparseableSince(t *testing.T) {
	awaits := []Await{{GoalObject: "pr:42", Condition: "merged", Since: "2026-13-45"}}
	defects := WakeStaleness("wake", "docs/problems/foo/delivery.md", awaits, 24*time.Hour, now)
	if len(defects) != 1 || defects[0].Op != "await-since" {
		t.Fatalf("want one await-since defect, got %v", defects)
	}
}

// The staleness read composes over the journal-derived await set: a ledger's
// outstanding awaits feed WakeStaleness, so the alarm is computed from ledger
// data alone (DINV-5, "computed from repo/ledger data alone").
func TestWakeStalenessOverDerivedAwaits(t *testing.T) {
	awaits, grammar := DeriveAwaits(verb, mustParse(t, "docs/problems/foo/delivery.md", awaitDelivery))
	if len(grammar) != 0 {
		t.Fatalf("fixture ledger has grammar defects: %v", grammar)
	}
	// awaitDelivery's journal leaves item:argus/epic#7 (since 2026-07-12)
	// outstanding — 3+ days at `now`, past a 24h bound.
	defects := WakeStaleness(verb, "docs/problems/foo/delivery.md", awaits, 24*time.Hour, now)
	if len(defects) != 1 || !strings.Contains(defects[0].Reason, "item:argus/epic#7") {
		t.Fatalf("a long-outstanding derived await surfaced no staleness defect: %v", defects)
	}
}
