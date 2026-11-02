package conformance

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/worktracker/model"
)

// The T3 append-log rows — comments and latest-wins decision-records
// (docs/tech-design/local-work-tracker.md §3.1, §3.14, DINV-5). They exercise
// the port only: no adapter symbol, so the same rows prove any medium. The
// interleaved-append row is the contention half of DINV-5 — concurrent writers
// serialize and lose nothing (PRD UB-4).

// interleavedAppendsLoseNothing: a burst of interleaved comment appends from two
// concurrent writers lands every entry, in per-writer order, with zero loss (T3
// acceptance 1; DINV-5). Each writer tags its comments so the row can prove both
// that all 2N landed and that no writer's stream was reordered or dropped.
func interleavedAppendsLoseNothing(t *testing.T, h Harness) {
	store, _ := h.NewStore(t)
	ctx := context.Background()
	created, defects := store.Create(ctx, model.CreateSpec{Title: "busy dialogue"})
	mustClean(t, "create", defects)
	id := created.ID

	const perWriter = 25
	writers := []string{"alice", "bob"}

	var wg sync.WaitGroup
	errs := make(chan string, len(writers)*perWriter)
	for _, who := range writers {
		wg.Add(1)
		go func(author string) {
			defer wg.Done()
			for i := 0; i < perWriter; i++ {
				_, defects := store.Comment(ctx, id, author, fmt.Sprintf("%s #%d", author, i))
				if len(defects) > 0 {
					errs <- fmt.Sprintf("%s comment %d: %v", author, i, defects)
					return
				}
			}
		}(who)
	}
	wg.Wait()
	close(errs)
	for e := range errs {
		t.Fatalf("concurrent append failed: %s", e)
	}

	got, defects := store.Get(ctx, id)
	mustClean(t, "get", defects)

	// Every append landed: exactly perWriter comments per writer, none lost.
	seen := map[string]int{}
	var order []string
	for _, e := range got.Log {
		if e.Kind != model.LogComment {
			continue
		}
		seen[e.Author]++
		order = append(order, e.Payload)
	}
	for _, who := range writers {
		mustCondition(t, seen[who] == perWriter, "writer %q landed %d comments, want %d (an append was lost)", who, seen[who], perWriter)
	}
	mustCondition(t, len(order) == len(writers)*perWriter, "log carries %d comments, want %d", len(order), len(writers)*perWriter)
	// Each writer's own stream is in the order it wrote (append-only, no reorder).
	for _, who := range writers {
		want := 0
		for _, payload := range order {
			var author string
			var n int
			if _, err := fmt.Sscanf(payload, "%s #%d", &author, &n); err != nil {
				continue
			}
			if author == who {
				mustCondition(t, n == want, "writer %q stream out of order: saw #%d, want #%d", who, n, want)
				want++
			}
		}
	}
}

// latestWinsAndIdempotentRepeat: the latest record per namespace is authoritative
// and an identical repeat is a recorded no-op (T3 acceptance 2; DINV-5, AP-22).
func latestWinsAndIdempotentRepeat(t *testing.T, h Harness) {
	store, _ := h.NewStore(t)
	ctx := context.Background()
	created, defects := store.Create(ctx, model.CreateSpec{Title: "decisions"})
	mustClean(t, "create", defects)
	id := created.ID

	// First judgment in the triage-disposition namespace.
	_, defects = store.Record(ctx, id, model.DecisionRecord{Namespace: model.NamespaceTriageDisposition, Author: "triager", Payload: "plan"})
	mustClean(t, "record plan", defects)

	rec, ok, defects := store.LatestRecord(ctx, id, model.NamespaceTriageDisposition)
	mustClean(t, "latest after plan", defects)
	mustCondition(t, ok && rec.Payload == "plan", "latest triage-disposition = %+v (ok=%v), want payload %q", rec, ok, "plan")

	// A second, different judgment supersedes it — latest-wins.
	superseded, defects := store.Record(ctx, id, model.DecisionRecord{Namespace: model.NamespaceTriageDisposition, Author: "triager", Payload: "operator"})
	mustClean(t, "record operator", defects)
	rec, ok, defects = store.LatestRecord(ctx, id, model.NamespaceTriageDisposition)
	mustClean(t, "latest after operator", defects)
	mustCondition(t, ok && rec.Payload == "operator", "latest after supersede = %+v, want payload %q", rec, "operator")
	beforeRepeat := countRecords(superseded, model.NamespaceTriageDisposition)

	// An identical repeat of the current latest writes nothing (recorded no-op).
	afterRepeat, defects := store.Record(ctx, id, model.DecisionRecord{Namespace: model.NamespaceTriageDisposition, Author: "triager", Payload: "operator"})
	mustClean(t, "record repeat", defects)
	mustCondition(t, countRecords(afterRepeat, model.NamespaceTriageDisposition) == beforeRepeat, "identical repeat appended a record: %d → %d", beforeRepeat, countRecords(afterRepeat, model.NamespaceTriageDisposition))
	mustCondition(t, afterRepeat.Updated.Equal(superseded.Updated), "no-op repeat moved the updated stamp: %v → %v", superseded.Updated, afterRepeat.Updated)

	// A different namespace is independent — its own latest, unaffected by the above.
	if _, defects := store.Record(ctx, id, model.DecisionRecord{Namespace: model.NamespaceCharter, Author: "planner", Payload: "ship the slice"}); len(defects) > 0 {
		t.Fatalf("record charter: %v", defects)
	}
	charter, ok, defects := store.LatestRecord(ctx, id, model.NamespaceCharter)
	mustClean(t, "latest charter", defects)
	mustCondition(t, ok && charter.Payload == "ship the slice", "charter namespace latest = %+v, want %q", charter, "ship the slice")
	// The triage-disposition latest is still "operator" — namespaces don't cross.
	rec, _, defects = store.LatestRecord(ctx, id, model.NamespaceTriageDisposition)
	mustClean(t, "latest triage after charter", defects)
	if rec.Payload != "operator" {
		t.Fatalf("charter write disturbed the triage-disposition latest: %q", rec.Payload)
	}
}

// outOfSetNamespaceRefused: a decision-record under an out-of-set namespace is
// refused with a defect naming the value, and nothing is written (T3 acceptance
// 3; DINV-11, fail closed).
func outOfSetNamespaceRefused(t *testing.T, h Harness) {
	store, _ := h.NewStore(t)
	ctx := context.Background()
	created, defects := store.Create(ctx, model.CreateSpec{Title: "guarded namespace"})
	mustClean(t, "create", defects)
	id := created.ID

	_, defects = store.Record(ctx, id, model.DecisionRecord{Namespace: model.Namespace("bogus-ns"), Author: "x", Payload: "whatever"})
	if len(defects) == 0 {
		t.Fatalf("out-of-set namespace accepted")
	}
	// A validate-time refusal names no file (none was read), but it names the
	// offending value and the allowed set (the transition precedent, DINV-11).
	d := defects[0]
	if d.Op == "" || d.Reason == "" || d.Remedy == "" {
		t.Fatalf("out-of-set namespace defect incomplete: %+v", d)
	}
	if !strings.Contains(d.Reason, "bogus-ns") {
		t.Fatalf("defect does not name the offending value: %q", d.Reason)
	}
	if !strings.Contains(d.Remedy, string(model.NamespaceTriageDisposition)) {
		t.Fatalf("defect does not name the allowed set: %q", d.Remedy)
	}

	// The read side refuses the same value.
	if _, _, defects := store.LatestRecord(ctx, id, model.Namespace("bogus-ns")); len(defects) == 0 {
		t.Fatalf("LatestRecord accepted an out-of-set namespace")
	}

	// Fail closed: no record landed on the item.
	got, defects := store.Get(ctx, id)
	mustClean(t, "get", defects)
	if n := countAllRecords(got); n != 0 {
		t.Fatalf("a refused record still wrote to the log: %d records", n)
	}
}

// emptyCommentRefused: an empty comment body is refused naming the offense, and
// nothing is written (append-only discipline; fail closed, AP-17).
func emptyCommentRefused(t *testing.T, h Harness) {
	store, _ := h.NewStore(t)
	ctx := context.Background()
	created, defects := store.Create(ctx, model.CreateSpec{Title: "no empty comments"})
	mustClean(t, "create", defects)

	_, defects = store.Comment(ctx, created.ID, "author", "   ")
	if len(defects) == 0 {
		t.Fatalf("empty comment accepted")
	}
	for _, d := range defects {
		if d.Op == "" || d.Reason == "" || d.Remedy == "" {
			t.Fatalf("empty-comment defect incomplete: %+v", d)
		}
	}
	got, defects := store.Get(ctx, created.ID)
	mustClean(t, "get", defects)
	if n := countComments(got); n != 0 {
		t.Fatalf("a refused comment still appended: %d", n)
	}
}

func countComments(w model.WorkItem) int {
	n := 0
	for _, e := range w.Log {
		if e.Kind == model.LogComment {
			n++
		}
	}
	return n
}

func countAllRecords(w model.WorkItem) int {
	n := 0
	for _, e := range w.Log {
		if e.Kind == model.LogDecisionRecord {
			n++
		}
	}
	return n
}

func countRecords(w model.WorkItem, ns model.Namespace) int {
	n := 0
	for _, e := range w.Log {
		if e.Kind == model.LogDecisionRecord && model.Namespace(e.Namespace) == ns {
			n++
		}
	}
	return n
}
