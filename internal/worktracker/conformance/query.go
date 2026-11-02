package conformance

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/worktracker"
	"github.com/agenticlab-ai/pyxis/internal/worktracker/model"
)

// The query rows extend the port contract with discovery — list/filter and
// exact search (prd:local-work-tracker/FR-10, FR-11, FR-15). They run against
// any adapter through the port alone, so the same behaviors prove localfs today
// and any future medium unchanged. Later behaviors append rows without editing
// these.

// listByStateAndSignal: a fixture corpus returns the exact expected set and
// order for the state and signal-name filters, and combined filters AND
// together — the discovery primitive, stably ordered by creation (FR-10 —
// acceptance criterion 1).
func listByStateAndSignal(t *testing.T, h Harness) {
	store, _ := h.NewStore(t)
	ctx := context.Background()
	corpus := seedCorpus(t, store)

	// No filter: the whole backlog in creation order.
	all, defects := store.List(ctx, model.Query{})
	mustClean(t, "list-all", defects)
	wantIDs(t, "list-all", all, corpus.all)

	// State filter: only the open items, in order.
	open, defects := store.List(ctx, model.Query{State: model.StateOpen})
	mustClean(t, "list-open", defects)
	wantIDs(t, "list-open", open, corpus.open)

	closed, defects := store.List(ctx, model.Query{State: model.StateClosed})
	mustClean(t, "list-closed", defects)
	wantIDs(t, "list-closed", closed, corpus.closed)

	// Signal-name filter: only the items carrying that exact signal name.
	held, defects := store.List(ctx, model.Query{Signal: "hold"})
	mustClean(t, "list-hold", defects)
	wantIDs(t, "list-hold", held, corpus.hold)

	// Combined: open AND carrying the agent signal.
	combined, defects := store.List(ctx, model.Query{State: model.StateOpen, Signal: "agent"})
	mustClean(t, "list-open-agent", defects)
	wantIDs(t, "list-open-agent", combined, corpus.openAgent)
}

// listByRoleSelectsAuthorization: a role filter selects exactly the items
// carrying a signal of that role — the authorization-role selection is how the
// factory-implementable set is named (FR-6, FR-10 — acceptance criterion 1's
// authorization-role clause). Any authorization-role signal qualifies, not just
// one name.
func listByRoleSelectsAuthorization(t *testing.T, h Harness) {
	store, _ := h.NewStore(t)
	ctx := context.Background()
	corpus := seedCorpus(t, store)

	auth, defects := store.List(ctx, model.Query{Role: model.RoleAuthorization})
	mustClean(t, "list-authorization", defects)
	wantIDs(t, "list-authorization", auth, corpus.authorization)

	routing, defects := store.List(ctx, model.Query{Role: model.RoleRouting})
	mustClean(t, "list-routing", defects)
	wantIDs(t, "list-routing", routing, corpus.routing)

	// An out-of-set role is refused with a named defect, not a silent empty set.
	if _, defects := store.List(ctx, model.Query{Role: "made-up"}); len(defects) == 0 {
		t.Fatalf("out-of-set role filter was accepted")
	}
}

// searchExactNoFalsePositives: search returns every seeded exact match and only
// exact matches; a near-miss token does not match and the fixture yields zero
// false positives (FR-11 — acceptance criteria 1 & 2).
func searchExactNoFalsePositives(t *testing.T, h Harness) {
	store, _ := h.NewStore(t)
	ctx := context.Background()
	corpus := seedCorpus(t, store)

	// A title token present in exactly the expected items.
	hits, defects := store.Search(ctx, "substrate")
	mustClean(t, "search-substrate", defects)
	wantIDs(t, "search-substrate", hits, corpus.substrate)

	// A near-miss spelling variant a fuzzy matcher would catch: exact matching
	// returns nothing (zero false positives).
	miss, defects := store.Search(ctx, "substraight")
	mustClean(t, "search-near-miss", defects)
	if len(miss) != 0 {
		t.Fatalf("near-miss %q matched %v, want no matches", "substraight", ids(miss))
	}

	// Body text is searched, not only titles.
	body, defects := store.Search(ctx, "hexagon")
	mustClean(t, "search-body", defects)
	wantIDs(t, "search-body", body, corpus.hexagon)

	// An empty query is refused, never a match-everything.
	if _, defects := store.Search(ctx, "   "); len(defects) == 0 {
		t.Fatalf("empty search query was accepted")
	}
}

// queryMachineOutputSchemaValid: both discovery verbs' machine projections
// validate against their golden schemas — the pinned schema tag, a count equal
// to the item array's length, and every item a full work-item view (FR-15 —
// acceptance criterion 3).
func queryMachineOutputSchemaValid(t *testing.T, h Harness) {
	store, _ := h.NewStore(t)
	ctx := context.Background()
	seedCorpus(t, store)

	items, defects := store.List(ctx, model.Query{State: model.StateOpen})
	mustClean(t, "list", defects)
	listView := model.ProjectList(model.Query{State: model.StateOpen}, items)
	assertResultShape(t, "list", mustJSON(t, listView), model.ListSchemaVersion, listView.Count)

	found, defects := store.Search(ctx, "substrate")
	mustClean(t, "search", defects)
	searchView := model.ProjectSearch("substrate", found)
	assertResultShape(t, "search", mustJSON(t, searchView), model.SearchSchemaVersion, searchView.Count)
}

// assertResultShape checks a discovery verb's machine object carries its pinned
// schema tag, a numeric count matching the items array length, and an items
// array of full work-item views (each with the item schema tag).
func assertResultShape(t *testing.T, verb string, raw []byte, wantSchema string, wantCount int) {
	t.Helper()
	var obj struct {
		Schema string            `json:"schema"`
		Count  int               `json:"count"`
		Items  []json.RawMessage `json:"items"`
	}
	if err := json.Unmarshal(raw, &obj); err != nil {
		t.Fatalf("%s view is not a JSON object: %v", verb, err)
	}
	if obj.Schema != wantSchema {
		t.Fatalf("%s schema tag = %q, want %q", verb, obj.Schema, wantSchema)
	}
	if obj.Count != wantCount || obj.Count != len(obj.Items) {
		t.Fatalf("%s count = %d, items = %d, want %d", verb, obj.Count, len(obj.Items), wantCount)
	}
	for i, it := range obj.Items {
		var m map[string]json.RawMessage
		if err := json.Unmarshal(it, &m); err != nil {
			t.Fatalf("%s item %d is not an object: %v", verb, i, err)
		}
		for _, k := range requiredViewKeys {
			if _, ok := m[k]; !ok {
				t.Fatalf("%s item %d missing required key %q", verb, i, k)
			}
		}
	}
}

// corpus records the fixture's expected id sets per filter — hand-computed so a
// filter's result is checked against a known-exact answer, not a re-derivation.
type corpus struct {
	all           []int
	open          []int
	closed        []int
	hold          []int
	openAgent     []int
	authorization []int
	routing       []int
	substrate     []int
	hexagon       []int
}

// seedCorpus builds a small, fully-known backlog and returns the exact id set
// each filter and search must return. The order of every set is creation (id)
// order, so a stable-sort regression is caught too.
func seedCorpus(t *testing.T, store worktracker.WorkTracker) corpus {
	t.Helper()

	// #1 open, agent (authorization) — title "the substrate", body mentions hexagon.
	i1 := create(t, store, model.CreateSpec{Title: "the substrate", Body: "one file per item behind the hexagon boundary"})
	addSignal(t, store, i1, "agent")

	// #2 open, agent + hold — title "substrate proof".
	i2 := create(t, store, model.CreateSpec{Title: "substrate proof"})
	addSignal(t, store, i2, "agent")
	addSignal(t, store, i2, "hold")

	// #3 open, needs-plan (routing) — unrelated title.
	i3 := create(t, store, model.CreateSpec{Title: "draft the plan"})
	addSignal(t, store, i3, "needs-plan")

	// #4 closed, hold (status) — mentions hexagon in the body.
	i4 := create(t, store, model.CreateSpec{Title: "retired approach", Body: "the old hexagon sketch"})
	addSignal(t, store, i4, "hold")
	transitionClosed(t, store, i4)

	// #5 open, no signals — unrelated text, so neither "substrate" nor the
	// near-miss "substraight" appears anywhere in the corpus by accident.
	i5 := create(t, store, model.CreateSpec{Title: "unrelated chore", Body: "tidy the desk"})

	return corpus{
		all:           []int{i1, i2, i3, i4, i5},
		open:          []int{i1, i2, i3, i5},
		closed:        []int{i4},
		hold:          []int{i2, i4},
		openAgent:     []int{i1, i2},
		authorization: []int{i1, i2}, // items carrying agent (authorization role)
		routing:       []int{i3},     // items carrying needs-plan (routing role)
		substrate:     []int{i1, i2}, // exact "substrate" in the title; #5 is a near miss
		hexagon:       []int{i1, i4}, // "hexagon" in the body
	}
}

func create(t *testing.T, store worktracker.WorkTracker, spec model.CreateSpec) int {
	t.Helper()
	w, defects := store.Create(context.Background(), spec)
	mustClean(t, "create", defects)
	return w.ID
}

func addSignal(t *testing.T, store worktracker.WorkTracker, id int, name string) {
	t.Helper()
	if _, defects := store.AddSignal(context.Background(), id, name); len(defects) > 0 {
		t.Fatalf("add signal %q to #%d: %v", name, id, defects)
	}
}

func transitionClosed(t *testing.T, store worktracker.WorkTracker, id int) {
	t.Helper()
	if _, defects := store.Transition(context.Background(), id, model.StateClosed, model.CloseCompleted, "conformance"); len(defects) > 0 {
		t.Fatalf("close #%d: %v", id, defects)
	}
}

// wantIDs asserts got is exactly want, in order — both the membership and the
// stable creation order.
func wantIDs(t *testing.T, op string, got []model.WorkItem, want []int) {
	t.Helper()
	g := ids(got)
	if len(g) != len(want) {
		t.Fatalf("%s = %v, want %v (length)", op, g, want)
	}
	for i := range want {
		if g[i] != want[i] {
			t.Fatalf("%s = %v, want %v (order/membership at %d)", op, g, want, i)
		}
	}
}

func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return b
}
