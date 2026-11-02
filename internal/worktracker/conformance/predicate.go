package conformance

import (
	"context"
	"strconv"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/worktracker"
	"github.com/agenticlab-ai/pyxis/internal/worktracker/model"
)

// The predicate rows extend the port contract with the two dispatch sets — ready
// and triage (prd:local-work-tracker/FR-12, FR-13, FR-16). They build a corpus
// through the port alone, so the same behaviors prove localfs today and any
// future medium unchanged. Membership is asserted over the port's own set reads,
// and one row proves read-your-writes on the decision path (DINV-4): a set read
// right after a qualifying write reflects that write.

// readySetOverCorpus: over a corpus built through the port, the ready set is
// exactly the authorized, open, unblocked, non-rollup leaves with no closing
// disposition (FR-12 — the port half of acceptance criterion 1).
func readySetOverCorpus(t *testing.T, h Harness) {
	store, _ := h.NewStore(t)
	c := buildDispatchCorpus(t, store)

	if got := setIDs(readySet(t, store)); !equalInts(got, []int{c.authorized, c.planned}) {
		t.Fatalf("ready set = %v, want %v", got, []int{c.authorized, c.planned})
	}
}

// triageSetOverCorpus: over the same corpus, the triage (undecided) set is
// exactly the open, undecided, unblocked, unassigned non-rollups (FR-13 — the
// port half of acceptance criterion 2).
func triageSetOverCorpus(t *testing.T, h Harness) {
	store, _ := h.NewStore(t)
	c := buildDispatchCorpus(t, store)

	if got := setIDs(triageSet(t, store)); !equalInts(got, []int{c.authorized, c.unauth}) {
		t.Fatalf("triage set = %v, want %v", got, []int{c.authorized, c.unauth})
	}
}

// membershipReflectsWrite: a membership read taken immediately after a
// qualifying write reflects that write, with no index or cache on the path
// (DINV-4 — acceptance criterion 4). An authorized item blocked by an open item
// is absent from the ready set; closing the blocker makes it present on the very
// next read.
func membershipReflectsWrite(t *testing.T, h Harness) {
	store, _ := h.NewStore(t)
	ctx := context.Background()

	blocker, defects := store.Create(ctx, model.CreateSpec{Title: "the blocker"})
	mustClean(t, "create-blocker", defects)
	item, defects := store.Create(ctx, model.CreateSpec{Title: "waits on the blocker"})
	mustClean(t, "create-item", defects)
	if _, defects := store.AddSignal(ctx, item.ID, "agent"); len(defects) > 0 {
		t.Fatalf("add agent: %v", defects)
	}
	if _, defects := store.AddBlockedBy(ctx, item.ID, strconv.Itoa(blocker.ID)); len(defects) > 0 {
		t.Fatalf("add blocker edge: %v", defects)
	}

	// While the blocker is open, the item is not ready.
	if contains(setIDs(readySet(t, store)), item.ID) {
		t.Fatalf("#%d ready while blocked by open #%d", item.ID, blocker.ID)
	}
	// Close the blocker; the immediately following read must reflect it.
	if _, defects := store.Transition(ctx, blocker.ID, model.StateClosed, model.CloseCompleted, "kit"); len(defects) > 0 {
		t.Fatalf("close blocker: %v", defects)
	}
	if !contains(setIDs(readySet(t, store)), item.ID) {
		t.Fatalf("#%d not ready immediately after closing its blocker (stale read on the decision path)", item.ID)
	}
}

// dispatchCorpus records the ids of the fixture items whose membership the rows
// assert, so the expectations read by name rather than by brittle literal id.
type dispatchCorpus struct {
	authorized int // open task, agent signal, nothing against it: ready + triage
	unauth     int // open task, no signal: triage only
	blocked    int // open task, agent signal, blocked by an open item: neither
	epic       int // structural rollup, agent signal: neither
	rejected   int // open task, agent signal, reject disposition: neither
	planned    int // open task, agent signal, plan disposition: ready only
}

// buildDispatchCorpus creates the six fixture items through the port and returns
// their ids. It is the single corpus both the ready and triage rows read.
func buildDispatchCorpus(t *testing.T, store worktracker.WorkTracker) dispatchCorpus {
	t.Helper()
	ctx := context.Background()
	mk := func(title, typ string) int {
		w, defects := store.Create(ctx, model.CreateSpec{Type: model.Type(typ), Title: title})
		mustClean(t, "create "+title, defects)
		return w.ID
	}
	sig := func(id int) {
		if _, defects := store.AddSignal(ctx, id, "agent"); len(defects) > 0 {
			t.Fatalf("add agent to #%d: %v", id, defects)
		}
	}
	disp := func(id int, payload string) {
		if _, defects := store.Record(ctx, id, model.DecisionRecord{Namespace: model.NamespaceTriageDisposition, Author: "triager", Payload: payload}); len(defects) > 0 {
			t.Fatalf("record %s on #%d: %v", payload, id, defects)
		}
	}

	var c dispatchCorpus
	c.authorized = mk("authorized leaf", "task")
	sig(c.authorized)
	c.unauth = mk("unauthorized leaf", "task")
	c.blocked = mk("blocked leaf", "task")
	sig(c.blocked)
	if _, defects := store.AddBlockedBy(ctx, c.blocked, strconv.Itoa(c.authorized)); len(defects) > 0 {
		t.Fatalf("block #%d on #%d: %v", c.blocked, c.authorized, defects)
	}
	c.epic = mk("the epic", "epic")
	sig(c.epic)
	c.rejected = mk("rejected leaf", "task")
	sig(c.rejected)
	disp(c.rejected, "reject")
	c.planned = mk("planned leaf", "task")
	sig(c.planned)
	disp(c.planned, "plan")
	return c
}

func readySet(t *testing.T, store worktracker.WorkTracker) []model.WorkItem {
	t.Helper()
	items, defects := store.ReadySet(context.Background())
	mustClean(t, "ready-set", defects)
	return items
}

func triageSet(t *testing.T, store worktracker.WorkTracker) []model.WorkItem {
	t.Helper()
	items, defects := store.TriageSet(context.Background())
	mustClean(t, "triage-set", defects)
	return items
}

func setIDs(items []model.WorkItem) []int {
	out := make([]int, len(items))
	for i, w := range items {
		out[i] = w.ID
	}
	return out
}

func equalInts(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func contains(xs []int, v int) bool {
	for _, x := range xs {
		if x == v {
			return true
		}
	}
	return false
}
