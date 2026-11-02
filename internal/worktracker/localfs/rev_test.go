package localfs

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/docgate/schema"
	"github.com/agenticlab-ai/pyxis/internal/worktracker/model"
)

// containsLine reports whether body has line as a whole line (trimmed).
func containsLine(body []byte, line string) bool {
	for _, l := range bytes.Split(body, []byte("\n")) {
		if string(bytes.TrimSpace(l)) == line {
			return true
		}
	}
	return false
}

// countLine counts how many whole lines of body equal line (trimmed).
func countLine(body []byte, line string) int {
	n := 0
	for _, l := range bytes.Split(body, []byte("\n")) {
		if string(bytes.TrimSpace(l)) == line {
			n++
		}
	}
	return n
}

// The reverse-parent index is a localfs implementation detail — invisible
// through the port — so its structure is proven here, white-box, by inspecting
// rev/ on disk (docs/tech-design/worktracker-hierarchy-index.md §3.6). The
// port-level query behavior it backs is proven by the conformance kit (T2).

// markerExists reports whether the edge parent→child has a marker on disk.
func markerExists(s *Store, parentID, childID int) bool {
	_, err := os.Stat(s.markerPath(strconv.Itoa(parentID), childID))
	return err == nil
}

// TestRevMarkersMaintained: a set, a reparent, and a clear each move exactly one
// marker — the new edge gains one and the vacated edge loses one (DINV-4,
// field-first via the write choke point, DINV-5).
func TestRevMarkersMaintained(t *testing.T) {
	s := New(t.TempDir(), fixedClock())
	ctx := context.Background()
	epicA, _ := s.Create(ctx, model.CreateSpec{Type: model.TypeEpic, Title: "epic A"})
	epicB, _ := s.Create(ctx, model.CreateSpec{Type: model.TypeEpic, Title: "epic B"})
	child, _ := s.Create(ctx, model.CreateSpec{Title: "child"})

	// Set: the child's marker appears under epic A.
	if _, defects := s.SetParent(ctx, child.ID, epicA.ID); len(defects) > 0 {
		t.Fatalf("set parent: %v", defects)
	}
	if !markerExists(s, epicA.ID, child.ID) {
		t.Fatalf("no marker for edge %d→%d after set", epicA.ID, child.ID)
	}

	// Reparent: the marker moves from A to B, none left behind.
	if _, defects := s.SetParent(ctx, child.ID, epicB.ID); len(defects) > 0 {
		t.Fatalf("reparent: %v", defects)
	}
	if markerExists(s, epicA.ID, child.ID) {
		t.Fatalf("stale marker for vacated edge %d→%d after reparent", epicA.ID, child.ID)
	}
	if !markerExists(s, epicB.ID, child.ID) {
		t.Fatalf("no marker for edge %d→%d after reparent", epicB.ID, child.ID)
	}

	// Clear: the marker is gone.
	if _, defects := s.ClearParent(ctx, child.ID); len(defects) > 0 {
		t.Fatalf("clear parent: %v", defects)
	}
	if markerExists(s, epicB.ID, child.ID) {
		t.Fatalf("marker survives after clear for edge %d→%d", epicB.ID, child.ID)
	}

	// Re-clearing an already-unparented child is a no-op and churns no marker.
	if _, defects := s.ClearParent(ctx, child.ID); len(defects) > 0 {
		t.Fatalf("idempotent clear: %v", defects)
	}
}

// TestRevGitignored: the store root gains a .gitignore ignoring rev/ once the
// index is first written, and the rule is never duplicated (DINV-3).
func TestRevGitignored(t *testing.T) {
	root := t.TempDir()
	s := New(root, fixedClock())
	ctx := context.Background()

	// Pre-existing user rule must survive the append.
	giPath := root + "/.gitignore"
	if err := os.WriteFile(giPath, []byte("*.log\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	epic, _ := s.Create(ctx, model.CreateSpec{Type: model.TypeEpic, Title: "epic"})
	child, _ := s.Create(ctx, model.CreateSpec{Title: "child"})
	if _, defects := s.SetParent(ctx, child.ID, epic.ID); len(defects) > 0 {
		t.Fatalf("set parent: %v", defects)
	}

	body, err := os.ReadFile(giPath)
	if err != nil {
		t.Fatalf("read .gitignore: %v", err)
	}
	if !gitignoreHasRev(body) {
		t.Fatalf(".gitignore does not ignore rev/: %q", body)
	}
	if want := "*.log"; !containsLine(body, want) {
		t.Fatalf(".gitignore lost the pre-existing rule %q: %q", want, body)
	}

	// A second Store over the same root must not duplicate the rule (cross-process
	// idempotency via the read-check).
	s2 := New(root, fixedClock())
	c2, _ := s2.Create(ctx, model.CreateSpec{Title: "child2"})
	if _, defects := s2.SetParent(ctx, c2.ID, epic.ID); len(defects) > 0 {
		t.Fatalf("set parent 2: %v", defects)
	}
	body, _ = os.ReadFile(giPath)
	if n := countLine(body, "rev/"); n != 1 {
		t.Fatalf("rev/ appears %d times in .gitignore, want 1: %q", n, body)
	}
}

// TestBuildIndexFromEmpty: with rev/ deleted (a fresh clone, DINV-3), a full
// rebuild reconstructs every edge purely from the item `parent` fields (DINV-6),
// and reports the item and marker counts.
func TestBuildIndexFromEmpty(t *testing.T) {
	s := New(t.TempDir(), fixedClock())
	ctx := context.Background()
	epic, _ := s.Create(ctx, model.CreateSpec{Type: model.TypeEpic, Title: "epic"})

	var kids []int
	for i := 0; i < 5; i++ {
		c, _ := s.Create(ctx, model.CreateSpec{Title: "child"})
		if _, defects := s.SetParent(ctx, c.ID, epic.ID); len(defects) > 0 {
			t.Fatalf("set parent: %v", defects)
		}
		kids = append(kids, c.ID)
	}
	orphan, _ := s.Create(ctx, model.CreateSpec{Title: "orphan"}) // no edge

	// Wipe the derived index — as if freshly cloned.
	if err := os.RemoveAll(s.revDir()); err != nil {
		t.Fatal(err)
	}
	if markerExists(s, epic.ID, kids[0]) {
		t.Fatalf("marker survived the wipe")
	}

	items, markers, err := s.buildIndex()
	if err != nil {
		t.Fatalf("buildIndex: %v", err)
	}
	if wantItems := len(kids) + 2; items != wantItems { // epic + kids + orphan
		t.Fatalf("rebuilt over %d items, want %d", items, wantItems)
	}
	if markers != len(kids) {
		t.Fatalf("rebuilt %d markers, want %d", markers, len(kids))
	}
	for _, id := range kids {
		if !markerExists(s, epic.ID, id) {
			t.Fatalf("edge %d→%d missing after rebuild", epic.ID, id)
		}
	}
	if markerExists(s, epic.ID, orphan.ID) {
		t.Fatalf("orphan wrongly indexed under the epic")
	}
}

func TestBuildIndexTreatsAbsentItemsDirectoryAsEmpty(t *testing.T) {
	s := New(filepath.Join(t.TempDir(), "new-store"), fixedClock())
	items, markers, err := s.buildIndex()
	if err != nil || items != 0 || markers != 0 {
		t.Fatalf("buildIndex() = %d, %d, %v", items, markers, err)
	}
}

// TestTruthWinsOverStaleIndex: the `parent` field is authoritative — deleting a
// marker (simulating a crash-desync or hand-edit, DINV-5/R2) does not change the
// truth, and a rebuild restores the marker from it (DINV-6).
func TestTruthWinsOverStaleIndex(t *testing.T) {
	s := New(t.TempDir(), fixedClock())
	ctx := context.Background()
	epic, _ := s.Create(ctx, model.CreateSpec{Type: model.TypeEpic, Title: "epic"})
	child, _ := s.Create(ctx, model.CreateSpec{Title: "child"})
	if _, defects := s.SetParent(ctx, child.ID, epic.ID); len(defects) > 0 {
		t.Fatalf("set parent: %v", defects)
	}

	// Desync: drop the marker but leave the truth.
	if err := os.Remove(s.markerPath(strconv.Itoa(epic.ID), child.ID)); err != nil {
		t.Fatal(err)
	}
	got, _ := s.Get(ctx, child.ID)
	if got.Parent != strconv.Itoa(epic.ID) {
		t.Fatalf("truth changed by a marker deletion: parent=%q", got.Parent)
	}

	// A rebuild heals the index back to the truth.
	if _, _, err := s.buildIndex(); err != nil {
		t.Fatalf("buildIndex: %v", err)
	}
	if !markerExists(s, epic.ID, child.ID) {
		t.Fatalf("rebuild did not restore the marker from the truth")
	}
}

// TestConcurrentDisjointMarkers: parallel parent-writes to different children
// under one parent touch disjoint marker files and lose none (DINV-4, AP-27) —
// the store needs no coordinator for its real concurrency (§3, A-6). Run under
// -race, this also proves the maintenance path is data-race free.
func TestConcurrentDisjointMarkers(t *testing.T) {
	s := New(t.TempDir(), fixedClock())
	ctx := context.Background()
	epic, _ := s.Create(ctx, model.CreateSpec{Type: model.TypeEpic, Title: "epic"})

	const n = 24
	kids := make([]int, n)
	for i := range kids {
		c, _ := s.Create(ctx, model.CreateSpec{Title: "child"})
		kids[i] = c.ID
	}

	var wg sync.WaitGroup
	failures := make([][]schema.Defect, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if _, defects := s.SetParent(ctx, kids[i], epic.ID); len(defects) > 0 {
				failures[i] = defects
			}
		}(i)
	}
	wg.Wait()

	for i, defects := range failures {
		if len(defects) > 0 {
			t.Fatalf("concurrent set-parent #%d failed: %v", kids[i], defects)
		}
	}
	for _, id := range kids {
		if !markerExists(s, epic.ID, id) {
			t.Fatalf("lost marker for edge %d→%d under concurrency", epic.ID, id)
		}
	}
}
