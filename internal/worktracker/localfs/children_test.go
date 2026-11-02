package localfs

import (
	"context"
	"os"
	"slices"
	"strconv"
	"testing"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/worktracker"
	"github.com/agenticlab-ai/pyxis/internal/worktracker/model"
)

// childIDs extracts the ids of a child set for test messages.
func childIDs(ws []model.WorkItem) []int {
	out := make([]int, len(ws))
	for i, w := range ws {
		out[i] = w.ID
	}
	return out
}

// handEditParent rewrites a child's authoritative `parent` field directly on
// disk, bypassing the CLI/port so the reverse-parent index is NOT maintained —
// exactly the hand-edit case the design keeps supported (NR-2, R1). The truth
// changes; the index lags until a reindex.
func handEditParent(t *testing.T, s *Store, childID, parentID int) {
	t.Helper()
	w, defects := s.read("test", childID)
	if len(defects) > 0 {
		t.Fatalf("read #%d: %v", childID, defects)
	}
	w.Parent = strconv.Itoa(parentID)
	if err := s.atomicReplace(itemPath(s.itemsDir(), childID), Render(w), nil); err != nil {
		t.Fatalf("hand-edit #%d: %v", childID, err)
	}
}

// TestChildrenIsIndexBacked proves the query reads the reverse-parent INDEX, not
// the item files, and that the two read modes differ exactly where the index and
// the truth disagree (worktracker-hierarchy-index §3.6/§3.7, DINV-2, A-3):
//   - a marker present but the truth pointing elsewhere: Display returns the
//     child (a scan never could), Decision re-verifies and drops it;
//   - the truth pointing here but no marker (a hand-edit): Display does NOT
//     return it (a scan would), until a reindex heals the index.
func TestChildrenIsIndexBacked(t *testing.T) {
	s := New(t.TempDir(), fixedClock())
	ctx := context.Background()
	epicA, _ := s.Create(ctx, model.CreateSpec{Type: model.TypeEpic, Title: "A"})
	epicB, _ := s.Create(ctx, model.CreateSpec{Type: model.TypeEpic, Title: "B"})
	child, _ := s.Create(ctx, model.CreateSpec{Title: "child"})

	// The child's truth is epicB.
	if _, defects := s.SetParent(ctx, child.ID, epicB.ID); len(defects) > 0 {
		t.Fatalf("set parent: %v", defects)
	}
	// Plant a STALE marker under epicA — a lagging or hand-edited index. epicA's
	// index claims the child, but the truth names epicB.
	if err := os.MkdirAll(s.parentDir(strconv.Itoa(epicA.ID)), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(s.markerPath(strconv.Itoa(epicA.ID), child.ID), nil, 0o644); err != nil {
		t.Fatal(err)
	}

	// Display trusts the marker: it returns the child — a parent-scan enumerator
	// could NOT, since the child's `parent` names epicB. This is the O(k) proof:
	// the read is driven by the index, not by scanning item fields.
	disp, defects := s.Children(ctx, epicA.ID, worktracker.ChildFilter{}, worktracker.Display)
	if len(defects) > 0 {
		t.Fatalf("display: %v", defects)
	}
	if len(disp) != 1 || disp[0].ID != child.ID {
		t.Fatalf("Display children of A = %v, want the stale-marked child #%d", childIDs(disp), child.ID)
	}
	// Decision re-verifies each candidate's `parent` and drops the stale one.
	dec, defects := s.Children(ctx, epicA.ID, worktracker.ChildFilter{}, worktracker.Decision)
	if len(defects) > 0 {
		t.Fatalf("decision: %v", defects)
	}
	if len(dec) != 0 {
		t.Fatalf("Decision children of A = %v, want none (stale marker dropped, DINV-2)", childIDs(dec))
	}

	// Now hand-edit the child's truth to epicA WITHOUT maintaining the index (as
	// an out-of-band edit would). The truth names epicA, but no marker exists —
	// so even Display cannot find it, because it reads the index, not a scan.
	handEditParent(t, s, child.ID, epicA.ID)
	// Remove the stale marker we planted so the index is now simply missing the edge.
	_ = os.Remove(s.markerPath(strconv.Itoa(epicA.ID), child.ID))
	missing, _ := s.Children(ctx, epicA.ID, worktracker.ChildFilter{}, worktracker.Display)
	if len(missing) != 0 {
		t.Fatalf("Display found a child with no marker = %v — the query is scanning, not index-backed", childIDs(missing))
	}
	// A reindex rebuilds the index from the truth, and the edge reappears (DINV-6).
	if _, _, err := s.buildIndex(); err != nil {
		t.Fatalf("reindex: %v", err)
	}
	healed, _ := s.Children(ctx, epicA.ID, worktracker.ChildFilter{}, worktracker.Display)
	if len(healed) != 1 || healed[0].ID != child.ID {
		t.Fatalf("after reindex, children of A = %v, want [%d]", childIDs(healed), child.ID)
	}
}

// TestChildrenLazyBuildsAbsentIndex: with rev/ absent (a fresh clone, DINV-3), a
// hierarchy read lazily builds the index from the item files and returns the
// correct children (DINV-6, A-4).
func TestChildrenLazyBuildsAbsentIndex(t *testing.T) {
	s := New(t.TempDir(), fixedClock())
	ctx := context.Background()
	epic, _ := s.Create(ctx, model.CreateSpec{Type: model.TypeEpic, Title: "epic"})
	var want []int
	for range 4 {
		c, _ := s.Create(ctx, model.CreateSpec{Title: "child"})
		if _, defects := s.SetParent(ctx, c.ID, epic.ID); len(defects) > 0 {
			t.Fatalf("set parent: %v", defects)
		}
		want = append(want, c.ID)
	}

	// Simulate a fresh clone: the derived index is gone.
	if err := os.RemoveAll(s.revDir()); err != nil {
		t.Fatal(err)
	}
	if s.revExists() {
		t.Fatalf("rev/ still present after removal")
	}

	got, defects := s.Children(ctx, epic.ID, worktracker.ChildFilter{}, worktracker.Display)
	if len(defects) > 0 {
		t.Fatalf("children: %v", defects)
	}
	if len(got) != len(want) {
		t.Fatalf("lazy-built children = %v, want %v", childIDs(got), want)
	}
	if !s.revExists() {
		t.Fatalf("the index was not lazily built on the read")
	}
}

// TestListParentRoutesThroughIndex: a parent-scoped list is answered by the one
// shared children enumerator over the index and composes with the state filter
// (worktracker-hierarchy-index §3.1, FR-19, DINV-7).
func TestListParentRoutesThroughIndex(t *testing.T) {
	s := New(t.TempDir(), fixedClock())
	ctx := context.Background()
	epic, _ := s.Create(ctx, model.CreateSpec{Type: model.TypeEpic, Title: "epic"})
	if _, defects := s.Create(ctx, model.CreateSpec{Title: "unrelated"}); len(defects) > 0 {
		t.Fatalf("create unrelated: %v", defects)
	}
	var want []int
	for range 3 {
		c, _ := s.Create(ctx, model.CreateSpec{Title: "child"})
		if _, defects := s.SetParent(ctx, c.ID, epic.ID); len(defects) > 0 {
			t.Fatalf("set parent: %v", defects)
		}
		want = append(want, c.ID)
	}

	pid := epic.ID
	listed, defects := s.List(ctx, model.Query{Parent: &pid})
	if len(defects) > 0 {
		t.Fatalf("list --parent: %v", defects)
	}
	if len(listed) != len(want) {
		t.Fatalf("list --parent = %v, want %v (the unrelated item must not appear)", childIDs(listed), want)
	}
	// It is the same set the direct Children read returns (one shared path).
	kids, _ := s.Children(ctx, epic.ID, worktracker.ChildFilter{}, worktracker.Display)
	if len(kids) != len(listed) {
		t.Fatalf("list --parent (%d) diverges from Children (%d)", len(listed), len(kids))
	}
	// Compose with the state filter: close one child, then filter to open.
	if _, defects := s.Transition(ctx, want[0], model.StateClosed, model.CloseCompleted, "t"); len(defects) > 0 {
		t.Fatalf("close: %v", defects)
	}
	openKids, defects := s.List(ctx, model.Query{Parent: &pid, State: model.StateOpen})
	if len(defects) > 0 {
		t.Fatalf("list --parent --state open: %v", defects)
	}
	if len(openKids) != len(want)-1 {
		t.Fatalf("open children under the parent = %d, want %d", len(openKids), len(want)-1)
	}
}

// TestChildrenReadsOnlyIndexedChildrenAtCapacity verifies the display access
// path (NFR-8/A-2) by counting filesystem reads, independent of host speed.
func TestChildrenReadsOnlyIndexedChildrenAtCapacity(t *testing.T) {
	// Fixed k at different N detects a full-store scan by its actual reads.
	for _, n := range []int{100, capacityN} {
		t.Run(strconv.Itoa(n), func(t *testing.T) {
			s := seedCapacityFixture(t, n)
			ctx := context.Background()
			epic, defects := s.Create(ctx, model.CreateSpec{Type: model.TypeEpic, Title: "cap epic"})
			if len(defects) > 0 {
				t.Fatal(defects)
			}
			const k = 8
			want := make([]int, 0, k)
			paths := make(map[string]bool)
			for range k {
				c, defects := s.Create(ctx, model.CreateSpec{Title: "cap child"})
				if len(defects) > 0 {
					t.Fatal(defects)
				}
				if _, defects := s.SetParent(ctx, c.ID, epic.ID); len(defects) > 0 {
					t.Fatal(defects)
				}
				want = append(want, c.ID)
				paths[itemPath(s.itemsDir(), c.ID)] = true
			}
			reads, dirs, entries := 0, 0, 0
			s.readFile = func(path string) ([]byte, error) {
				reads++
				if !paths[path] {
					t.Errorf("read unrelated file %q", path)
				}
				return os.ReadFile(path)
			}
			s.readDir = func(path string) ([]os.DirEntry, error) {
				dirs++
				if path != s.parentDir(strconv.Itoa(epic.ID)) {
					t.Errorf("scanned unrelated directory %q", path)
				}
				xs, err := os.ReadDir(path)
				entries += len(xs)
				return xs, err
			}
			start := time.Now()
			kids, defects := s.Children(ctx, epic.ID, worktracker.ChildFilter{}, worktracker.Display)
			t.Logf("children(k=%d) over %d unrelated items: %v; advisory (AP-29)", k, n, time.Since(start))
			if len(defects) > 0 || !slices.Equal(childIDs(kids), want) {
				t.Fatalf("children=%v defects=%v, want %v", childIDs(kids), defects, want)
			}
			if reads != k || dirs != 1 || entries != k {
				t.Fatalf("file reads=%d directories=%d entries=%d, want %d/1/%d independent of N", reads, dirs, entries, k, k)
			}
		})
	}
}

// TestRollupGroupsAndSurfacesClosed: the rollup groups children by open and
// closed, and a fully-completed epic surfaces its entire task set through the
// closed group — the regression the amendment targets over the base open-only
// rollup (prd:local-work-tracker/FR-7, worktracker-hierarchy-index A-7, DINV-7).
func TestRollupGroupsAndSurfacesClosed(t *testing.T) {
	s := New(t.TempDir(), fixedClock())
	ctx := context.Background()
	epic, _ := s.Create(ctx, model.CreateSpec{Type: model.TypeEpic, Title: "epic"})
	var all []int
	for range 3 {
		c, _ := s.Create(ctx, model.CreateSpec{Title: "child"})
		if _, defects := s.SetParent(ctx, c.ID, epic.ID); len(defects) > 0 {
			t.Fatalf("set parent: %v", defects)
		}
		all = append(all, c.ID)
	}

	// Mixed: close the last child. The rollup splits open/closed.
	if _, defects := s.Transition(ctx, all[2], model.StateClosed, model.CloseCompleted, "t"); len(defects) > 0 {
		t.Fatalf("close: %v", defects)
	}
	view, defects := s.Rollup(ctx, epic.ID, worktracker.Display)
	if len(defects) > 0 {
		t.Fatalf("rollup: %v", defects)
	}
	if len(view.Open) != 2 || len(view.Closed) != 1 {
		t.Fatalf("rollup groups = open %v / closed %v, want 2 open / 1 closed", childIDs(view.Open), childIDs(view.Closed))
	}
	if view.Closed[0].ID != all[2] {
		t.Fatalf("closed group = %v, want [#%d]", childIDs(view.Closed), all[2])
	}

	// Complete the epic: close the rest. Its full task set surfaces in the closed
	// group — where the base open-only rollup would have shown nothing.
	for _, id := range all[:2] {
		if _, defects := s.Transition(ctx, id, model.StateClosed, model.CloseCompleted, "t"); len(defects) > 0 {
			t.Fatalf("close: %v", defects)
		}
	}
	done, defects := s.Rollup(ctx, epic.ID, worktracker.Display)
	if len(defects) > 0 {
		t.Fatalf("rollup: %v", defects)
	}
	if len(done.Open) != 0 || len(done.Closed) != len(all) {
		t.Fatalf("completed epic rollup = open %v / closed %v, want 0 open / %d closed", childIDs(done.Open), childIDs(done.Closed), len(all))
	}
}
