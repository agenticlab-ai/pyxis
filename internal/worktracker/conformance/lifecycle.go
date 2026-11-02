package conformance

import (
	"context"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/worktracker/model"
)

// The T2 lifecycle rows — edit and idempotent transitions (docs/tech-design/
// local-work-tracker.md §3.10, DINV-11). They exercise the port only: no
// adapter symbol, so they prove any medium. The harness supplies a clock that
// advances between calls, so "the updated stamp moved" is observable.

// editTouchesOnlyNamedFields: an edit changes exactly the named field plus the
// updated stamp; every other field is byte-identical (T2 acceptance 1).
func editTouchesOnlyNamedFields(t *testing.T, h Harness) {
	store, _ := h.NewStore(t)
	ctx := context.Background()
	created, defects := store.Create(ctx, model.CreateSpec{
		Type: model.TypeEpic, Title: "before", Body: "body stays", Priority: model.PriorityHigh,
	})
	mustClean(t, "create", defects)

	before, defects := store.Get(ctx, created.ID)
	mustClean(t, "get-before", defects)

	// Edit the title only.
	newTitle := "after"
	edited, defects := store.Edit(ctx, created.ID, model.EditPatch{Title: &newTitle})
	mustClean(t, "edit", defects)

	after, defects := store.Get(ctx, created.ID)
	mustClean(t, "get-after", defects)
	mustCondition(t, sameItem(edited, after), "edit return and re-read diverge:\nreturned=%+v\nre-read=%+v", edited, after)
	mustCondition(t, after.Title == newTitle, "title not updated: %q", after.Title)
	mustCondition(t, after.Updated.After(before.Updated), "updated stamp did not advance: before=%v after=%v", before.Updated, after.Updated)
	assertUnchangedExceptTitleUpdated(t, before, after)

	// Edit the body only — the title just written must not move.
	newBody := "body moved"
	if _, defects := store.Edit(ctx, created.ID, model.EditPatch{Body: &newBody}); len(defects) > 0 {
		t.Fatalf("edit body: %v", defects)
	}
	final, defects := store.Get(ctx, created.ID)
	mustClean(t, "get-final", defects)
	mustCondition(t, final.Body == newBody, "body not updated: %q", final.Body)
	mustCondition(t, final.Title == newTitle, "body edit disturbed the title: %q", final.Title)
}

// idempotentTransition: closing a closed item is a recorded no-op, and the
// transition log carries exactly one entry per applied change (T2 acceptance 2).
func idempotentTransition(t *testing.T, h Harness) {
	store, _ := h.NewStore(t)
	ctx := context.Background()
	created, defects := store.Create(ctx, model.CreateSpec{Title: "lifecycle"})
	mustClean(t, "create", defects)
	id := created.ID

	// Close once — one applied change, one audit entry.
	closed, defects := store.Transition(ctx, id, model.StateClosed, model.CloseCompleted, "tester")
	mustClean(t, "close", defects)
	mustCondition(t, closed.State == model.StateClosed && closed.CloseReason == model.CloseCompleted, "close did not apply: %+v", closed)
	mustCondition(t, countTransitions(closed) == 1, "after one close, want 1 transition entry, got %d", countTransitions(closed))

	// Close again — a no-op: still closed, still exactly one entry, stamp frozen.
	again, defects := store.Transition(ctx, id, model.StateClosed, model.CloseCompleted, "tester")
	mustClean(t, "close-again", defects)
	mustCondition(t, again.State == model.StateClosed, "re-close changed state: %+v", again)
	mustCondition(t, countTransitions(again) == 1, "re-close appended an entry: want 1, got %d", countTransitions(again))
	mustCondition(t, again.Updated.Equal(closed.Updated), "no-op re-close moved the updated stamp: %v → %v", closed.Updated, again.Updated)

	// Reopen — a second applied change, a second entry; the reason is cleared.
	reopened, defects := store.Transition(ctx, id, model.StateOpen, model.CloseReasonNone, "tester")
	mustClean(t, "reopen", defects)
	mustCondition(t, reopened.State == model.StateOpen && reopened.CloseReason == model.CloseReasonNone, "reopen did not apply/clear reason: %+v", reopened)
	mustCondition(t, countTransitions(reopened) == 2, "after close+reopen, want 2 transition entries, got %d", countTransitions(reopened))

	// Reopen again — a no-op: still two entries.
	final, defects := store.Transition(ctx, id, model.StateOpen, model.CloseReasonNone, "tester")
	mustClean(t, "reopen-again", defects)
	mustCondition(t, countTransitions(final) == 2, "re-reopen appended an entry: want 2, got %d", countTransitions(final))
}

// outOfSetCloseReasonRefused: an out-of-set close reason is refused with a
// defect naming the value and the allowed set, and nothing is written (T2
// acceptance 3).
func outOfSetCloseReasonRefused(t *testing.T, h Harness) {
	store, _ := h.NewStore(t)
	ctx := context.Background()
	created, defects := store.Create(ctx, model.CreateSpec{Title: "guarded"})
	mustClean(t, "create", defects)

	_, defects = store.Transition(ctx, created.ID, model.StateClosed, model.CloseReason("bogus"), "tester")
	mustCondition(t, len(defects) != 0, "out-of-set close reason accepted")
	d := defects[0]
	mustCondition(t, d.File != "" || d.Op != "" || d.Reason != "" || d.Remedy != "", "defect is empty: %+v", d)
	if !strings.Contains(d.Reason, "bogus") {
		t.Fatalf("defect does not name the offending value: %q", d.Reason)
	}
	if !strings.Contains(d.Remedy, string(model.CloseCompleted)) || !strings.Contains(d.Remedy, string(model.CloseRejected)) {
		t.Fatalf("defect does not name the allowed set: %q", d.Remedy)
	}

	// Fail closed: the item is untouched — still open, no audit entry.
	got, defects := store.Get(ctx, created.ID)
	mustClean(t, "get", defects)
	if got.State != model.StateOpen {
		t.Fatalf("refused close still mutated the item: state=%s", got.State)
	}
	if n := countTransitions(got); n != 0 {
		t.Fatalf("refused close appended an entry: %d", n)
	}
}

func countTransitions(w model.WorkItem) int {
	n := 0
	for _, e := range w.Log {
		if e.Kind == model.LogTransition {
			n++
		}
	}
	return n
}

// assertUnchangedExceptTitleUpdated asserts every field but Title and Updated
// round-tripped unchanged across an edit.
func assertUnchangedExceptTitleUpdated(t *testing.T, before, after model.WorkItem) {
	t.Helper()
	if after.ID != before.ID || after.Type != before.Type || after.State != before.State ||
		after.Body != before.Body || after.Priority != before.Priority ||
		after.CloseReason != before.CloseReason || after.Parent != before.Parent ||
		!after.Created.Equal(before.Created) ||
		!sameStrings(after.Signals, before.Signals) || !sameStrings(after.Assignees, before.Assignees) ||
		!sameStrings(after.BlockedBy, before.BlockedBy) || len(after.Log) != len(before.Log) {
		t.Fatalf("edit mutated a field it should not have:\nbefore=%+v\nafter=%+v", before, after)
	}
}

// sameItem reports whether two items are field-equal (times compared by instant).
func sameItem(a, b model.WorkItem) bool {
	if a.ID != b.ID || a.Type != b.Type || a.State != b.State || a.Title != b.Title ||
		a.Body != b.Body || a.Priority != b.Priority || a.CloseReason != b.CloseReason ||
		a.Parent != b.Parent || !a.Created.Equal(b.Created) || !a.Updated.Equal(b.Updated) ||
		!sameStrings(a.Signals, b.Signals) || !sameStrings(a.Assignees, b.Assignees) ||
		!sameStrings(a.BlockedBy, b.BlockedBy) || len(a.Log) != len(b.Log) {
		return false
	}
	for i := range a.Log {
		if a.Log[i].Kind != b.Log[i].Kind || a.Log[i].Author != b.Log[i].Author ||
			!a.Log[i].At.Equal(b.Log[i].At) || a.Log[i].Namespace != b.Log[i].Namespace ||
			a.Log[i].Payload != b.Log[i].Payload {
			return false
		}
	}
	return true
}

func sameStrings(a, b []string) bool {
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
