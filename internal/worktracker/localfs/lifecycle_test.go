package localfs

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/worktracker/model"
)

func ptr(s string) *string { return &s }

// raw reads an item file's on-disk bytes so a test can diff the exact stored form.
func raw(t *testing.T, s *Store, id int) string {
	t.Helper()
	b, err := os.ReadFile(itemPath(s.itemsDir(), id))
	if err != nil {
		t.Fatalf("read #%d: %v", id, err)
	}
	return string(b)
}

// changedLines returns the header/body lines that differ between two renderings.
func changedLines(before, after string) []string {
	b := strings.Split(before, "\n")
	a := strings.Split(after, "\n")
	var diff []string
	for i := 0; i < len(a); i++ {
		if i >= len(b) || a[i] != b[i] {
			diff = append(diff, a[i])
		}
	}
	return diff
}

// TestEditDiffsOnlyNamedFieldAndStamp: on-disk, an edit changes exactly the
// title line and the updated line — nothing else (T2 acceptance 1, byte-level).
func TestEditDiffsOnlyNamedFieldAndStamp(t *testing.T) {
	s := New(t.TempDir(), advancingClock())
	ctx := context.Background()
	w, defects := s.Create(ctx, model.CreateSpec{Type: model.TypeTask, Title: "before", Body: "keep me"})
	if len(defects) > 0 {
		t.Fatalf("create: %v", defects)
	}
	before := raw(t, s, w.ID)

	if _, defects := s.Edit(ctx, w.ID, model.EditPatch{Title: ptr("after")}); len(defects) > 0 {
		t.Fatalf("edit: %v", defects)
	}
	after := raw(t, s, w.ID)

	diff := changedLines(before, after)
	if len(diff) != 2 {
		t.Fatalf("edit changed %d lines, want exactly 2 (title, updated):\n%q", len(diff), diff)
	}
	for _, l := range diff {
		if !strings.HasPrefix(l, "title:") && !strings.HasPrefix(l, "updated:") {
			t.Fatalf("edit touched an unexpected line: %q", l)
		}
	}
	if !strings.Contains(after, "title: after") || !strings.Contains(after, "keep me") {
		t.Fatalf("edit result malformed:\n%s", after)
	}
}

// TestEditNoOpLeavesFileByteIdentical: an edit that names no changing field
// writes nothing — the file (and its updated stamp) is untouched.
func TestEditNoOpLeavesFileByteIdentical(t *testing.T) {
	s := New(t.TempDir(), advancingClock())
	ctx := context.Background()
	w, defects := s.Create(ctx, model.CreateSpec{Title: "steady"})
	if len(defects) > 0 {
		t.Fatalf("create: %v", defects)
	}
	before := raw(t, s, w.ID)

	// Setting the title to its current value, and an empty patch, both no-op.
	if _, defects := s.Edit(ctx, w.ID, model.EditPatch{Title: ptr("steady")}); len(defects) > 0 {
		t.Fatalf("edit same: %v", defects)
	}
	if _, defects := s.Edit(ctx, w.ID, model.EditPatch{}); len(defects) > 0 {
		t.Fatalf("edit empty: %v", defects)
	}
	if after := raw(t, s, w.ID); after != before {
		t.Fatalf("no-op edit mutated the file:\nbefore=%q\nafter=%q", before, after)
	}
}

// TestCloseAppendsAuditAndRoundTrips: a close writes state, close_reason, and
// exactly one transition audit entry that Get reads back (T2 acceptance 2).
func TestCloseAppendsAuditAndRoundTrips(t *testing.T) {
	s := New(t.TempDir(), advancingClock())
	ctx := context.Background()
	w, defects := s.Create(ctx, model.CreateSpec{Title: "close me"})
	if len(defects) > 0 {
		t.Fatalf("create: %v", defects)
	}

	closed, defects := s.Transition(ctx, w.ID, model.StateClosed, model.CloseCompleted, "alice")
	if len(defects) > 0 {
		t.Fatalf("close: %v", defects)
	}
	if closed.State != model.StateClosed || closed.CloseReason != model.CloseCompleted {
		t.Fatalf("close not applied: %+v", closed)
	}
	on := raw(t, s, w.ID)
	if !strings.Contains(on, "state: closed") || !strings.Contains(on, "close_reason: completed") {
		t.Fatalf("header not updated on disk:\n%s", on)
	}
	if !strings.Contains(on, "- transition · alice · ") || !strings.Contains(on, "open → closed (completed)") {
		t.Fatalf("audit entry not written legibly:\n%s", on)
	}

	got, defects := s.Get(ctx, w.ID)
	if len(defects) > 0 {
		t.Fatalf("get: %v", defects)
	}
	if len(got.Log) != 1 || got.Log[0].Kind != model.LogTransition || got.Log[0].Author != "alice" {
		t.Fatalf("log did not round-trip: %+v", got.Log)
	}
	if got.Log[0].Payload != "open → closed (completed)" {
		t.Fatalf("payload = %q", got.Log[0].Payload)
	}
}

// TestReCloseIsNoOpFileFrozen: closing a closed item writes nothing at all.
func TestReCloseIsNoOpFileFrozen(t *testing.T) {
	s := New(t.TempDir(), advancingClock())
	ctx := context.Background()
	w, _ := s.Create(ctx, model.CreateSpec{Title: "x"})
	if _, defects := s.Transition(ctx, w.ID, model.StateClosed, model.CloseCompleted, "bob"); len(defects) > 0 {
		t.Fatalf("close: %v", defects)
	}
	before := raw(t, s, w.ID)
	if _, defects := s.Transition(ctx, w.ID, model.StateClosed, model.CloseCompleted, "bob"); len(defects) > 0 {
		t.Fatalf("re-close: %v", defects)
	}
	if after := raw(t, s, w.ID); after != before {
		t.Fatalf("re-close mutated the file:\nbefore=%q\nafter=%q", before, after)
	}
}

// TestReopenClearsReasonAndAppends: reopening clears the reason and records a
// second audit entry.
func TestReopenClearsReasonAndAppends(t *testing.T) {
	s := New(t.TempDir(), advancingClock())
	ctx := context.Background()
	w, _ := s.Create(ctx, model.CreateSpec{Title: "cycle"})
	if _, defects := s.Transition(ctx, w.ID, model.StateClosed, model.CloseRejected, "carol"); len(defects) > 0 {
		t.Fatalf("close: %v", defects)
	}
	reopened, defects := s.Transition(ctx, w.ID, model.StateOpen, model.CloseReasonNone, "carol")
	if len(defects) > 0 {
		t.Fatalf("reopen: %v", defects)
	}
	if reopened.State != model.StateOpen || reopened.CloseReason != model.CloseReasonNone {
		t.Fatalf("reopen did not clear the reason: %+v", reopened)
	}
	on := raw(t, s, w.ID)
	if !strings.Contains(on, "close_reason: \n") && !strings.Contains(on, "close_reason:\n") {
		t.Fatalf("close_reason not cleared on disk:\n%s", on)
	}
	got, _ := s.Get(ctx, w.ID)
	if len(got.Log) != 2 || got.Log[1].Payload != "closed → open" {
		t.Fatalf("reopen audit wrong: %+v", got.Log)
	}
}

// TestCloseReasonOutOfSetRefusedNoWrite: an out-of-set reason is refused before
// any write, even validation-first when the item is already in the target
// state; the item is left untouched (T2 acceptance 3, fail closed).
func TestCloseReasonOutOfSetRefusedNoWrite(t *testing.T) {
	s := New(t.TempDir(), advancingClock())
	ctx := context.Background()
	w, _ := s.Create(ctx, model.CreateSpec{Title: "guarded"})
	before := raw(t, s, w.ID)

	_, defects := s.Transition(ctx, w.ID, model.StateClosed, model.CloseReason("bogus"), "d")
	if len(defects) == 0 {
		t.Fatal("out-of-set reason accepted")
	}
	d := defects[0]
	if d.Verb != "close" || !strings.Contains(d.Reason, `"bogus"`) {
		t.Fatalf("defect does not name verb+value: %+v", d)
	}
	if !strings.Contains(d.Remedy, "completed") || !strings.Contains(d.Remedy, "rejected") {
		t.Fatalf("defect does not name the allowed set: %q", d.Remedy)
	}
	if after := raw(t, s, w.ID); after != before {
		t.Fatalf("refused close still wrote the file")
	}
}

// TestTransitionMissingItem: transitioning a missing item is a defect naming
// the transition's verb, not "show".
func TestTransitionMissingItem(t *testing.T) {
	s := New(t.TempDir(), advancingClock())
	_, defects := s.Transition(context.Background(), 9, model.StateClosed, model.CloseCompleted, "e")
	if len(defects) != 1 || defects[0].Verb != "close" || !strings.Contains(defects[0].Reason, "no work item") {
		t.Fatalf("missing-item transition defect wrong: %v", defects)
	}
}

// TestParseRejectsMalformedLogEntry: a hand-corrupted log line fails closed with
// a named defect (the log parses as closed as the header, DINV-11).
func TestParseRejectsMalformedLogEntry(t *testing.T) {
	s := New(t.TempDir(), advancingClock())
	ctx := context.Background()
	w, _ := s.Create(ctx, model.CreateSpec{Title: "t"})
	if _, defects := s.Transition(ctx, w.ID, model.StateClosed, model.CloseCompleted, "f"); len(defects) > 0 {
		t.Fatalf("close: %v", defects)
	}
	on := raw(t, s, w.ID)
	// Break the kind of the transition entry.
	broken := strings.Replace(on, "- transition · ", "- teleport · ", 1)
	_, defects := Parse("show", "items/1.md", []byte(broken))
	found := false
	for _, d := range defects {
		if d.Op == "parse-log" && strings.Contains(d.Reason, "out of set") {
			found = true
		}
	}
	if !found {
		t.Fatalf("malformed log kind not rejected: %v", defects)
	}
}
