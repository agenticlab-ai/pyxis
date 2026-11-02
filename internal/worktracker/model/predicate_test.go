package model

import (
	"reflect"
	"testing"
	"time"
)

// rec builds a triage-disposition decision-record log entry with the given
// payload — the frozen judgment the predicates read.
func rec(payload string) LogEntry {
	return LogEntry{Kind: LogDecisionRecord, Namespace: string(NamespaceTriageDisposition), Payload: payload, At: time.Unix(0, 0)}
}

// corpus is the hand-authored fixture the ready/triage expectations are computed
// against by hand. Every clause of both predicates is exercised by at least one
// item, and each item's membership is asserted below against a set computed by
// reading the predicate definitions (§3.6, FR-12/FR-13), never the code.
func corpus() []WorkItem {
	return []WorkItem{
		// 1 — authorized open leaf, nothing against it: ready AND triage.
		{ID: 1, Type: TypeTask, State: StateOpen, Signals: []string{"agent"}},
		// 2 — authorized, but blocked by #3 which is open: neither.
		{ID: 2, Type: TypeTask, State: StateOpen, Signals: []string{"agent"}, BlockedBy: []string{"3"}},
		// 3 — unauthorized open leaf, nothing against it: triage only (no auth ⇒ not ready).
		{ID: 3, Type: TypeTask, State: StateOpen},
		// 4 — closed: neither (closed ⇒ out of both).
		{ID: 4, Type: TypeTask, State: StateClosed, Signals: []string{"agent"}},
		// 5 — epic (structural rollup): neither.
		{ID: 5, Type: TypeEpic, State: StateOpen, Signals: []string{"agent"}},
		// 6 — authorized, but a reject disposition (closing): not ready; decided ⇒ not triage.
		{ID: 6, Type: TypeTask, State: StateOpen, Signals: []string{"agent"}, Log: []LogEntry{rec("reject")}},
		// 7 — authorized, a plan disposition (non-closing): ready; decided ⇒ not triage.
		{ID: 7, Type: TypeTask, State: StateOpen, Signals: []string{"agent"}, Log: []LogEntry{rec("plan")}},
		// 8 — unauthorized and assignee-held: not ready (no auth); in flight ⇒ not triage.
		{ID: 8, Type: TypeTask, State: StateOpen, Assignees: []string{"alice"}},
		// 9 — authorized, blocked by #4 which is closed: ready AND triage (closed blocker doesn't block).
		{ID: 9, Type: TypeTask, State: StateOpen, Signals: []string{"agent"}, BlockedBy: []string{"4"}},
		// 10 — authorized, blocked by a cross-repo ref (opaque, never resolved): ready AND triage.
		{ID: 10, Type: TypeTask, State: StateOpen, Signals: []string{"agent"}, BlockedBy: []string{"octo/repo#5"}},
	}
}

func ids(items []WorkItem) []int {
	out := make([]int, len(items))
	for i, w := range items {
		out[i] = w.ID
	}
	return out
}

// TestReadySetMatchesHandComputed: the ready set over the fixture corpus equals
// the hand-computed expectation exactly, in creation (id) order (FR-12,
// acceptance criterion 1).
func TestReadySetMatchesHandComputed(t *testing.T) {
	want := []int{1, 7, 9, 10}
	if got := ids(ReadySet(corpus())); !reflect.DeepEqual(got, want) {
		t.Fatalf("ready set = %v, want %v", got, want)
	}
}

// TestTriageSetMatchesHandComputed: the triage set over the same corpus equals
// its hand-computed expectation exactly, in creation order (FR-13, acceptance
// criterion 2).
func TestTriageSetMatchesHandComputed(t *testing.T) {
	want := []int{1, 3, 9, 10}
	if got := ids(TriageSet(corpus())); !reflect.DeepEqual(got, want) {
		t.Fatalf("triage set = %v, want %v", got, want)
	}
}

// TestReadyExcludesEveryDisqualifier pins each individual ready-clause: flipping
// exactly one qualifying attribute of a ready item drops it from the set, so no
// clause is vacuous.
func TestReadyExcludesEveryDisqualifier(t *testing.T) {
	base := WorkItem{ID: 1, Type: TypeTask, State: StateOpen, Signals: []string{"agent"}}
	idx := map[int]WorkItem{1: base, 2: {ID: 2, State: StateOpen}}
	if !IsReady(base, idx) {
		t.Fatalf("base item is not ready: %+v", base)
	}
	cases := map[string]func(w *WorkItem){
		"closed":              func(w *WorkItem) { w.State = StateClosed },
		"no authorization":    func(w *WorkItem) { w.Signals = nil },
		"structural rollup":   func(w *WorkItem) { w.Type = TypeEpic },
		"open blocker":        func(w *WorkItem) { w.BlockedBy = []string{"2"} },
		"closing disposition": func(w *WorkItem) { w.Log = []LogEntry{rec("reject")} },
	}
	for name, mutate := range cases {
		w := base
		mutate(&w)
		if IsReady(w, idx) {
			t.Errorf("%s: item still ready: %+v", name, w)
		}
	}
}

// TestTriageExcludesEveryDisqualifier pins each individual triage-clause the same
// way — one flipped attribute drops the item.
func TestTriageExcludesEveryDisqualifier(t *testing.T) {
	base := WorkItem{ID: 1, Type: TypeTask, State: StateOpen}
	idx := map[int]WorkItem{1: base, 2: {ID: 2, State: StateOpen}}
	if !IsTriage(base, idx) {
		t.Fatalf("base item is not in triage: %+v", base)
	}
	cases := map[string]func(w *WorkItem){
		"closed":            func(w *WorkItem) { w.State = StateClosed },
		"has disposition":   func(w *WorkItem) { w.Log = []LogEntry{rec("plan")} },
		"structural rollup": func(w *WorkItem) { w.Type = TypeMilestone },
		"open blocker":      func(w *WorkItem) { w.BlockedBy = []string{"2"} },
		"assignee held":     func(w *WorkItem) { w.Assignees = []string{"bob"} },
	}
	for name, mutate := range cases {
		w := base
		mutate(&w)
		if IsTriage(w, idx) {
			t.Errorf("%s: item still in triage: %+v", name, w)
		}
	}
}

// TestSupersededRejectDoesNotClose: a reject disposition later overridden by a
// plan disposition is not a closing disposition — latest-wins (DINV-5), so the
// item is ready again.
func TestSupersededRejectDoesNotClose(t *testing.T) {
	w := WorkItem{ID: 1, Type: TypeTask, State: StateOpen, Signals: []string{"agent"},
		Log: []LogEntry{rec("reject"), rec("plan")}}
	if !IsReady(w, map[int]WorkItem{1: w}) {
		t.Fatalf("a reject superseded by plan should be ready: %+v", w.Log)
	}
}

// TestSetViewShape: the machine projection carries the schema tag, the predicate
// name, a never-null Items slice, and one ItemView per member (FR-15/FR-16).
func TestSetViewShape(t *testing.T) {
	v := Set("ready", ReadySet(corpus()))
	if v.Schema != PredicateSetSchema || v.Predicate != "ready" {
		t.Fatalf("set view header = %q/%q", v.Schema, v.Predicate)
	}
	if len(v.Items) != 4 {
		t.Fatalf("set view has %d items, want 4", len(v.Items))
	}
	empty := Set("triage", nil)
	if empty.Items == nil {
		t.Fatalf("empty set view Items is null, want []")
	}
}
