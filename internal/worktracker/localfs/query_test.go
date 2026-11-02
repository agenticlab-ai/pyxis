package localfs

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/worktracker/model"
)

// Search reaches comment log entries — the "comments" in FR-11's "titles,
// bodies, and comments". The comment verb lands in a later task, so this
// white-box test seeds a comment (and a decision-record) directly on disk and
// asserts search reads the comment but not the audit-only record payload.
func TestSearchOverComments(t *testing.T) {
	s := New(t.TempDir(), fixedClock())
	ctx := context.Background()
	w, defects := s.Create(ctx, model.CreateSpec{Title: "plain title"})
	if len(defects) > 0 {
		t.Fatalf("create: %v", defects)
	}
	at := time.Date(2026, 7, 15, 0, 0, 0, 0, time.UTC)
	w.Log = []model.LogEntry{
		{Kind: model.LogComment, Author: "a", At: at, Payload: "found in a comment: needle-c"},
		{Kind: model.LogDecisionRecord, Author: "a", At: at, Namespace: "triage-disposition", Payload: "needle-d in a record"},
	}
	if err := os.WriteFile(itemPath(s.itemsDir(), w.ID), Render(w), 0o644); err != nil {
		t.Fatal(err)
	}

	got, defects := s.Search(ctx, "needle-c")
	if len(defects) > 0 {
		t.Fatalf("search comment: %v", defects)
	}
	if len(got) != 1 || got[0].ID != w.ID {
		t.Fatalf("comment search = %v, want just #%d", ids(got), w.ID)
	}
	// A decision-record payload is audit state, not searchable comment text.
	rec, defects := s.Search(ctx, "needle-d")
	if len(defects) > 0 {
		t.Fatalf("search record: %v", defects)
	}
	if len(rec) != 0 {
		t.Fatalf("search matched a decision-record payload: %v", ids(rec))
	}
}

// A hand-corrupted item is quarantined during a scan: list and search skip it
// and still return every well-formed sibling (§3.13). One bad file cannot blind
// a whole query.
func TestQueryQuarantinesMalformed(t *testing.T) {
	s := New(t.TempDir(), fixedClock())
	ctx := context.Background()
	good, defects := s.Create(ctx, model.CreateSpec{Title: "well formed"})
	if len(defects) > 0 {
		t.Fatalf("create good: %v", defects)
	}
	bad, defects := s.Create(ctx, model.CreateSpec{Title: "will be corrupted"})
	if len(defects) > 0 {
		t.Fatalf("create bad: %v", defects)
	}
	if err := os.WriteFile(itemPath(s.itemsDir(), bad.ID), []byte("not a work item\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, defects := s.List(ctx, model.Query{})
	if len(defects) > 0 {
		t.Fatalf("list with a corrupt sibling reported defects: %v", defects)
	}
	if len(got) != 1 || got[0].ID != good.ID {
		t.Fatalf("list = %v, want just the well-formed #%d", ids(got), good.ID)
	}
}

// List reflects a mutation on its next call with no cache in the path (DINV-4):
// a signal added between two lists changes the filtered result immediately.
func TestListReflectsWritesImmediately(t *testing.T) {
	s := New(t.TempDir(), advancingClock())
	ctx := context.Background()
	w, defects := s.Create(ctx, model.CreateSpec{Title: "item"})
	if len(defects) > 0 {
		t.Fatalf("create: %v", defects)
	}

	before, defects := s.List(ctx, model.Query{Signal: "agent"})
	if len(defects) > 0 || len(before) != 0 {
		t.Fatalf("pre-add agent list = %v (defects %v), want empty", ids(before), defects)
	}
	if _, defects := s.AddSignal(ctx, w.ID, "agent"); len(defects) > 0 {
		t.Fatalf("add signal: %v", defects)
	}
	after, defects := s.List(ctx, model.Query{Signal: "agent"})
	if len(defects) > 0 {
		t.Fatalf("post-add list: %v", defects)
	}
	if len(after) != 1 || after[0].ID != w.ID {
		t.Fatalf("post-add agent list = %v, want just #%d", ids(after), w.ID)
	}
}

// ids projects an item slice to its ids for a readable failure message.
func ids(ws []model.WorkItem) []int {
	out := make([]int, len(ws))
	for i, w := range ws {
		out[i] = w.ID
	}
	return out
}
