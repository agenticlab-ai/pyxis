package conformance

import (
	"context"
	"strconv"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/worktracker"
	"github.com/agenticlab-ai/pyxis/internal/worktracker/model"
)

// The hierarchy rows extend the port contract with the Feature→Milestone→Epic→
// Task shape (prd:local-work-tracker/FR-7, UB-2). They run against any adapter
// through the port alone, so the same behaviors prove localfs today and any
// future medium unchanged (mirroring the first-task rows). Later behaviors
// append rows without editing these.

// parentBidirectional: after SetParent the child names its parent and appears in
// the parent's Children; after ClearParent both directions drop the relation
// (FR-7 — the parent/child relation is maintained both directions).
func parentBidirectional(t *testing.T, h Harness) {
	store, _ := h.NewStore(t)
	ctx := context.Background()
	epic, defects := store.Create(ctx, model.CreateSpec{Type: model.TypeEpic, Title: "the epic"})
	mustClean(t, "create-epic", defects)
	child, defects := store.Create(ctx, model.CreateSpec{Title: "the child"})
	mustClean(t, "create-child", defects)

	set, defects := store.SetParent(ctx, child.ID, epic.ID)
	mustClean(t, "set-parent", defects)
	mustCondition(t, set.Parent == strconv.Itoa(epic.ID), "SetParent returned parent %q, want %q", set.Parent, strconv.Itoa(epic.ID))

	// Forward: the child names its parent.
	got, defects := store.Get(ctx, child.ID)
	mustClean(t, "get-child", defects)
	mustCondition(t, got.Parent == strconv.Itoa(epic.ID), "child parent = %q, want %q", got.Parent, strconv.Itoa(epic.ID))
	// Reverse: the child appears under its parent.
	kids, defects := store.Children(ctx, epic.ID, worktracker.ChildFilter{}, worktracker.Display)
	mustClean(t, "children", defects)
	mustCondition(t, len(kids) == 1 && kids[0].ID == child.ID, "Children(%d) = %v, want just #%d", epic.ID, ids(kids), child.ID)

	// Clear drops the relation both directions.
	_, defects = store.ClearParent(ctx, child.ID)
	mustCondition(t, len(defects) == 0, "clear-parent: %v", defects)
	got, defects = store.Get(ctx, child.ID)
	mustClean(t, "get-cleared", defects)
	mustCondition(t, got.Parent == "", "child parent after clear = %q, want empty", got.Parent)
	kids, defects = store.Children(ctx, epic.ID, worktracker.ChildFilter{}, worktracker.Display)
	mustClean(t, "children-after-clear", defects)
	mustCondition(t, len(kids) == 0, "Children(%d) after clear = %v, want none", epic.ID, ids(kids))
}

// epicRollupOpenChildren: an epic reports exactly the count and identity of its
// open children — unrelated items and non-children are excluded (FR-7). Closing
// a child to prove closed-child exclusion needs the transition verb (a later
// task); the localfs white-box test covers that, and this row proves the rollup
// selects the epic's children exactly.
func epicRollupOpenChildren(t *testing.T, h Harness) {
	store, _ := h.NewStore(t)
	ctx := context.Background()
	epic, defects := store.Create(ctx, model.CreateSpec{Type: model.TypeEpic, Title: "rollup epic"})
	mustClean(t, "create-epic", defects)
	other, defects := store.Create(ctx, model.CreateSpec{Type: model.TypeEpic, Title: "other epic"})
	mustClean(t, "create-other", defects)

	var want []int
	for i := 0; i < 3; i++ {
		c, defects := store.Create(ctx, model.CreateSpec{Title: "child"})
		mustClean(t, "create-child", defects)
		if _, defects := store.SetParent(ctx, c.ID, epic.ID); len(defects) > 0 {
			t.Fatalf("set-parent: %v", defects)
		}
		want = append(want, c.ID)
	}
	// A child of the other epic and an unparented item must not appear.
	stray, defects := store.Create(ctx, model.CreateSpec{Title: "stray"})
	mustClean(t, "create-stray", defects)
	if _, defects := store.SetParent(ctx, stray.ID, other.ID); len(defects) > 0 {
		t.Fatalf("set-parent-stray: %v", defects)
	}
	if _, defects := store.Create(ctx, model.CreateSpec{Title: "orphan"}); len(defects) > 0 {
		t.Fatalf("create-orphan: %v", defects)
	}

	kids, defects := store.Children(ctx, epic.ID, worktracker.ChildFilter{}, worktracker.Display)
	mustClean(t, "children", defects)
	open := model.OpenChildren(kids)
	mustCondition(t, len(open) == len(want), "open-child count = %d, want %d (%v)", len(open), len(want), ids(open))
	for i, c := range open {
		mustCondition(t, c.ID == want[i], "open child %d = #%d, want #%d (%v)", i, c.ID, want[i], ids(open))
	}
}

// parentMissingRefused: a parent reference to a nonexistent item is refused with
// a defect naming the target, and the child is not repointed (UB-2, DINV-1).
func parentMissingRefused(t *testing.T, h Harness) {
	store, _ := h.NewStore(t)
	ctx := context.Background()
	child, defects := store.Create(ctx, model.CreateSpec{Title: "child"})
	mustClean(t, "create-child", defects)

	const missing = 9999
	_, defects = store.SetParent(ctx, child.ID, missing)
	if len(defects) == 0 {
		t.Fatalf("SetParent to missing #%d was accepted", missing)
	}
	named := false
	for _, d := range defects {
		if d.File == "" || d.Op == "" || d.Reason == "" || d.Remedy == "" {
			t.Fatalf("defect does not name the full quadruple: %+v", d)
		}
		if strings.Contains(d.Reason, strconv.Itoa(missing)) {
			named = true
		}
	}
	if !named {
		t.Fatalf("refusal does not name the missing target #%d: %v", missing, defects)
	}
	// Nothing was written: the child stays unparented.
	got, defects := store.Get(ctx, child.ID)
	mustClean(t, "get-child", defects)
	if got.Parent != "" {
		t.Fatalf("child was repointed despite the refusal: parent=%q", got.Parent)
	}
}

// childrenAllStatesAndStateFilter: a parent-scoped children read returns the
// direct children across ALL states by default, a state filter narrows to
// exactly the matching members, and a childless parent returns empty, not an
// error (prd:local-work-tracker/FR-19, worktracker-hierarchy-index §3.1). This is
// the reverse-parent query the amendment added over the open-only rollup — one
// implementation shared by the port and the CLI (DINV-7).
func childrenAllStatesAndStateFilter(t *testing.T, h Harness) {
	store, _ := h.NewStore(t)
	ctx := context.Background()
	epic, defects := store.Create(ctx, model.CreateSpec{Type: model.TypeEpic, Title: "epic"})
	mustClean(t, "create-epic", defects)
	openChild, defects := store.Create(ctx, model.CreateSpec{Title: "open child"})
	mustClean(t, "create-open", defects)
	closedChild, defects := store.Create(ctx, model.CreateSpec{Title: "closed child"})
	mustClean(t, "create-closed", defects)
	for _, id := range []int{openChild.ID, closedChild.ID} {
		if _, defects := store.SetParent(ctx, id, epic.ID); len(defects) > 0 {
			t.Fatalf("set-parent #%d: %v", id, defects)
		}
	}
	if _, defects := store.Transition(ctx, closedChild.ID, model.StateClosed, model.CloseCompleted, "kit"); len(defects) > 0 {
		t.Fatalf("close child: %v", defects)
	}

	// All states by default — both children, creation-ordered; a completed epic
	// still surfaces its full task set (the regression FR-19 targets).
	all, defects := store.Children(ctx, epic.ID, worktracker.ChildFilter{}, worktracker.Display)
	mustClean(t, "children-all", defects)
	if len(all) != 2 || all[0].ID != openChild.ID || all[1].ID != closedChild.ID {
		t.Fatalf("all-states children = %v, want [%d %d]", ids(all), openChild.ID, closedChild.ID)
	}
	// The state filter narrows to exactly the matching members.
	openOnly, defects := store.Children(ctx, epic.ID, worktracker.ChildFilter{State: model.StateOpen}, worktracker.Display)
	mustClean(t, "children-open", defects)
	if len(openOnly) != 1 || openOnly[0].ID != openChild.ID {
		t.Fatalf("open children = %v, want [%d]", ids(openOnly), openChild.ID)
	}
	closedOnly, defects := store.Children(ctx, epic.ID, worktracker.ChildFilter{State: model.StateClosed}, worktracker.Display)
	mustClean(t, "children-closed", defects)
	if len(closedOnly) != 1 || closedOnly[0].ID != closedChild.ID {
		t.Fatalf("closed children = %v, want [%d]", ids(closedOnly), closedChild.ID)
	}
	// A childless parent returns empty, not an error.
	lone, defects := store.Create(ctx, model.CreateSpec{Type: model.TypeEpic, Title: "childless"})
	mustClean(t, "create-lone", defects)
	none, defects := store.Children(ctx, lone.ID, worktracker.ChildFilter{}, worktracker.Display)
	mustClean(t, "children-none", defects)
	if len(none) != 0 {
		t.Fatalf("childless parent returned %v, want empty", ids(none))
	}
}

// epicRollupGroupsOpenClosed: the epic rollup groups its direct children by open
// and closed, each creation-ordered — a completed epic surfaces its full task set
// through the closed group, not the empty open-only view the base rollup returned
// (prd:local-work-tracker/FR-7, worktracker-hierarchy-index §3.1, A-7). One
// implementation shared by the port and the CLI (DINV-7).
func epicRollupGroupsOpenClosed(t *testing.T, h Harness) {
	store, _ := h.NewStore(t)
	ctx := context.Background()
	epic, defects := store.Create(ctx, model.CreateSpec{Type: model.TypeEpic, Title: "epic"})
	mustClean(t, "create-epic", defects)

	var open, closed []int
	for i := 0; i < 2; i++ {
		c, defects := store.Create(ctx, model.CreateSpec{Title: "open child"})
		mustClean(t, "create-open", defects)
		if _, defects := store.SetParent(ctx, c.ID, epic.ID); len(defects) > 0 {
			t.Fatalf("set-parent: %v", defects)
		}
		open = append(open, c.ID)
	}
	for i := 0; i < 3; i++ {
		c, defects := store.Create(ctx, model.CreateSpec{Title: "closed child"})
		mustClean(t, "create-closed", defects)
		if _, defects := store.SetParent(ctx, c.ID, epic.ID); len(defects) > 0 {
			t.Fatalf("set-parent: %v", defects)
		}
		if _, defects := store.Transition(ctx, c.ID, model.StateClosed, model.CloseCompleted, "kit"); len(defects) > 0 {
			t.Fatalf("close: %v", defects)
		}
		closed = append(closed, c.ID)
	}

	view, defects := store.Rollup(ctx, epic.ID, worktracker.Display)
	mustClean(t, "rollup", defects)
	if len(view.Open) != len(open) {
		t.Fatalf("rollup open = %v, want %v", ids(view.Open), open)
	}
	if len(view.Closed) != len(closed) {
		t.Fatalf("rollup closed = %v, want %v", ids(view.Closed), closed)
	}
	for i, c := range view.Open {
		if c.ID != open[i] || c.State != model.StateOpen {
			t.Fatalf("rollup open[%d] = #%d(%s), want #%d open", i, c.ID, c.State, open[i])
		}
	}
	for i, c := range view.Closed {
		if c.ID != closed[i] || c.State != model.StateClosed {
			t.Fatalf("rollup closed[%d] = #%d(%s), want #%d closed", i, c.ID, c.State, closed[i])
		}
	}
}

func ids(ws []model.WorkItem) []int {
	out := make([]int, len(ws))
	for i, w := range ws {
		out[i] = w.ID
	}
	return out
}
