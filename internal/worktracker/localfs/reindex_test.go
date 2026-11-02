package localfs

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/worktracker"
	"github.com/agenticlab-ai/pyxis/internal/worktracker/model"
)

func TestReindexReportsUnwritableRootAndCleanEmptyStore(t *testing.T) {
	blocked := filepath.Join(t.TempDir(), "blocked")
	if err := os.WriteFile(blocked, []byte("file"), 0o600); err != nil {
		t.Fatal(err)
	}
	s := New(blocked, fixedClock())
	if _, defects := s.Reindex(t.Context()); len(defects) == 0 {
		t.Fatal("Reindex accepted a file as store root")
	}

	empty := New(t.TempDir(), fixedClock())
	empty.touchStamp()
	if empty.dirty() {
		t.Fatal("empty store with a stamp is dirty")
	}
}

// TestDirtySameTickHandEditDetected pins the coarse-mtime fix: a hand-edit whose
// item-file mtime EQUALS the stamp's — the collision a jiffy-granular filesystem
// (Linux) produces for a same-tick write — must still read as dirty and heal on a
// decision read. A strict `>` comparison missed this and failed on Linux CI while
// passing on ns-resolution macOS; forcing the equal mtimes with Chtimes makes the
// regression reproducible on any filesystem.
func TestDirtySameTickHandEditDetected(t *testing.T) {
	s := New(t.TempDir(), fixedClock())
	ctx := context.Background()
	epic, _ := s.Create(ctx, model.CreateSpec{Type: model.TypeEpic, Title: "epic"})
	child, _ := s.Create(ctx, model.CreateSpec{Title: "child"})

	// Build the index (writes the stamp), then hand-edit the child to add an edge
	// with no marker.
	if _, _, err := s.buildIndex(); err != nil {
		t.Fatalf("buildIndex: %v", err)
	}
	handEditParent(t, s, child.ID, epic.ID)

	// Put every item strictly before the stamp EXCEPT the hand-edited child, whose
	// mtime we collapse onto the stamp's — what a coarse-mtime clock does to a
	// same-tick write. If any other item were left at or after the stamp it would
	// trip dirty() on its own and hide whether the same-tick child is caught.
	same := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	before := same.Add(-time.Hour)
	if err := os.Chtimes(itemPath(s.itemsDir(), epic.ID), before, before); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(s.stampPath(), same, same); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(itemPath(s.itemsDir(), child.ID), same, same); err != nil {
		t.Fatal(err)
	}

	if !s.dirty() {
		t.Fatalf("a same-mtime hand-edit read as clean — a strict > misses it (Linux coarse mtime)")
	}
	// And a decision read heals it before returning.
	dec, defects := s.Children(ctx, epic.ID, worktracker.ChildFilter{}, worktracker.Decision)
	if len(defects) > 0 {
		t.Fatalf("decision read: %v", defects)
	}
	if len(dec) != 1 || dec[0].ID != child.ID {
		t.Fatalf("same-tick hand-edit not healed on the decision read: %v", childIDs(dec))
	}
}

// TestReindexReconstructsFromTruth: reindex rebuilds the index wholly from the
// item `parent` fields — restoring a dropped real edge and sweeping a bogus one —
// and reports the item and marker counts (worktracker-hierarchy-index §3.7,
// DINV-6).
func TestReindexReconstructsFromTruth(t *testing.T) {
	s := New(t.TempDir(), fixedClock())
	ctx := context.Background()
	epic, _ := s.Create(ctx, model.CreateSpec{Type: model.TypeEpic, Title: "epic"})
	var kids []int
	for i := 0; i < 4; i++ {
		c, _ := s.Create(ctx, model.CreateSpec{Title: "child"})
		if _, defects := s.SetParent(ctx, c.ID, epic.ID); len(defects) > 0 {
			t.Fatalf("set parent: %v", defects)
		}
		kids = append(kids, c.ID)
	}

	// Corrupt the index: drop one real marker and plant a bogus one under a
	// nonexistent parent.
	if err := os.Remove(s.markerPath(strconv.Itoa(epic.ID), kids[0])); err != nil {
		t.Fatal(err)
	}
	const bogusParent = 9999
	if err := os.MkdirAll(s.parentDir(strconv.Itoa(bogusParent)), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(s.markerPath(strconv.Itoa(bogusParent), kids[1]), nil, 0o644); err != nil {
		t.Fatal(err)
	}

	report, defects := s.Reindex(ctx)
	if len(defects) > 0 {
		t.Fatalf("reindex: %v", defects)
	}
	if report.Trigger != "explicit" {
		t.Fatalf("trigger = %q, want explicit", report.Trigger)
	}
	if report.Items != len(kids)+1 { // the epic + its children
		t.Fatalf("report items = %d, want %d", report.Items, len(kids)+1)
	}
	if report.Markers != len(kids) {
		t.Fatalf("report markers = %d, want %d", report.Markers, len(kids))
	}
	for _, id := range kids {
		if !markerExists(s, epic.ID, id) {
			t.Fatalf("real edge %d→%d missing after reindex", epic.ID, id)
		}
	}
	if markerExists(s, bogusParent, kids[1]) {
		t.Fatalf("bogus marker survived reindex")
	}
}

// TestCrashBetweenItemAndMarkerHealed reproduces the state a crash between the
// item write and the marker op leaves — the `parent` truth committed but no
// marker (field-first, DINV-5) — and proves the truth is intact and reindex
// restores the index (prd:local-work-tracker acceptance A-5).
func TestCrashBetweenItemAndMarkerHealed(t *testing.T) {
	s := New(t.TempDir(), fixedClock())
	ctx := context.Background()
	epic, _ := s.Create(ctx, model.CreateSpec{Type: model.TypeEpic, Title: "epic"})
	child, _ := s.Create(ctx, model.CreateSpec{Title: "child"})

	// Establish the index first — a real crash leaves it stale by one edge, not
	// absent (an absent index would just lazily rebuild and hide the lag).
	if _, defects := s.Children(ctx, epic.ID, worktracker.ChildFilter{}, worktracker.Display); len(defects) > 0 {
		t.Fatalf("baseline read: %v", defects)
	}

	// The item write committed; the marker op never ran (the crash). handEditParent
	// writes the `parent` field without maintaining the index — exactly that state.
	handEditParent(t, s, child.ID, epic.ID)

	// The truth survived the crash.
	got, defects := s.Get(ctx, child.ID)
	if len(defects) > 0 {
		t.Fatalf("get: %v", defects)
	}
	if got.Parent != strconv.Itoa(epic.ID) {
		t.Fatalf("truth lost after simulated crash: parent = %q", got.Parent)
	}
	// The index lags by the one edge — a display read does not yet see it.
	disp, _ := s.Children(ctx, epic.ID, worktracker.ChildFilter{}, worktracker.Display)
	if len(disp) != 0 {
		t.Fatalf("display saw an unindexed edge before reindex: %v", childIDs(disp))
	}

	// Reindex restores the marker from the truth (DINV-6).
	if _, defects := s.Reindex(ctx); len(defects) > 0 {
		t.Fatalf("reindex: %v", defects)
	}
	healed, _ := s.Children(ctx, epic.ID, worktracker.ChildFilter{}, worktracker.Display)
	if len(healed) != 1 || healed[0].ID != child.ID {
		t.Fatalf("reindex did not restore the edge: %v", childIDs(healed))
	}
}

// TestDecisionReadHealsHandEdit: a hand-edit that adds an edge without a marker
// is invisible to a display read (which stays O(k) and never heals), but a
// decision read trips the dirty check, rebuilds the index, and sees the edge —
// the exact-read-before-decision discipline (worktracker-hierarchy-index §3.7,
// design Q1, R1).
func TestDecisionReadHealsHandEdit(t *testing.T) {
	s := New(t.TempDir(), fixedClock())
	ctx := context.Background()
	epic, _ := s.Create(ctx, model.CreateSpec{Type: model.TypeEpic, Title: "epic"})
	child, _ := s.Create(ctx, model.CreateSpec{Title: "child"})

	// Establish a fresh index baseline (a display read lazily builds it + stamps).
	if _, defects := s.Children(ctx, epic.ID, worktracker.ChildFilter{}, worktracker.Display); len(defects) > 0 {
		t.Fatalf("baseline read: %v", defects)
	}

	// Hand-edit adds the edge out of band — the item is now newer than the stamp.
	handEditParent(t, s, child.ID, epic.ID)

	// A display read stays O(k) and does not heal: the edge is not yet visible.
	disp, _ := s.Children(ctx, epic.ID, worktracker.ChildFilter{}, worktracker.Display)
	if len(disp) != 0 {
		t.Fatalf("display healed a hand-edit (it must stay O(k)/lagging): %v", childIDs(disp))
	}

	// A decision read trips the dirty check, rebuilds, and sees the edge.
	dec, defects := s.Children(ctx, epic.ID, worktracker.ChildFilter{}, worktracker.Decision)
	if len(defects) > 0 {
		t.Fatalf("decision read: %v", defects)
	}
	if len(dec) != 1 || dec[0].ID != child.ID {
		t.Fatalf("decision read did not heal the hand-edit: %v", childIDs(dec))
	}
	// The heal persisted: a display read now sees the edge too.
	disp2, _ := s.Children(ctx, epic.ID, worktracker.ChildFilter{}, worktracker.Display)
	if len(disp2) != 1 || disp2[0].ID != child.ID {
		t.Fatalf("the decision heal did not persist to the index: %v", childIDs(disp2))
	}
}
