package localfs

import (
	"context"
	"strconv"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/worktracker/model"
)

// The cross-repo owner/repo#id form is stored and rendered opaquely — it
// round-trips through the on-disk grammar verbatim and is never resolved, so a
// blocker in another repo can be recorded before any cross-repo resolution
// exists (§3.5, Q5).
func TestCrossRepoRefStoredOpaquely(t *testing.T) {
	s := New(t.TempDir(), advancingClock())
	ctx := context.Background()
	item, _ := s.Create(ctx, model.CreateSpec{Title: "waits on another repo"})

	const ref = "agenticlab-ai/pyxis#123"
	if _, defects := s.AddBlockedBy(ctx, item.ID, ref); len(defects) > 0 {
		t.Fatalf("add cross-repo ref: %v", defects)
	}
	got, defects := s.Get(ctx, item.ID)
	if len(defects) > 0 {
		t.Fatalf("get: %v", defects)
	}
	if len(got.BlockedBy) != 1 || got.BlockedBy[0] != ref {
		t.Fatalf("cross-repo ref did not round-trip verbatim: %v", got.BlockedBy)
	}
	// It is never counted as a local edge: Blocking over its numeric tail sees
	// nothing (no resolution).
	blocking, defects := s.Blocking(ctx, 123)
	if len(defects) > 0 {
		t.Fatalf("blocking: %v", defects)
	}
	if len(blocking) != 0 {
		t.Fatalf("cross-repo tail resolved to a local edge: %v", blocking)
	}
}

// A self edge is the shortest cycle and is refused naming `#id → #id`, writing
// nothing.
func TestSelfDependencyRefused(t *testing.T) {
	s := New(t.TempDir(), advancingClock())
	ctx := context.Background()
	item, _ := s.Create(ctx, model.CreateSpec{Title: "loner"})

	_, defects := s.AddBlockedBy(ctx, item.ID, strconv.Itoa(item.ID))
	if len(defects) == 0 {
		t.Fatalf("self dependency was accepted")
	}
	want := "#" + strconv.Itoa(item.ID) + " → #" + strconv.Itoa(item.ID)
	if !strings.Contains(defects[0].Reason, want) {
		t.Fatalf("self-cycle path not named %q: %+v", want, defects[0])
	}
	got, _ := s.Get(ctx, item.ID)
	if len(got.BlockedBy) != 0 {
		t.Fatalf("self dependency was written: %v", got.BlockedBy)
	}
}

// The named cycle path is the full loop source → … → source in edge order, so
// the operator can see exactly which edge to redirect (DINV-10).
func TestCyclePathIsTheFullLoop(t *testing.T) {
	s := New(t.TempDir(), advancingClock())
	ctx := context.Background()
	a, _ := s.Create(ctx, model.CreateSpec{Title: "a"})
	b, _ := s.Create(ctx, model.CreateSpec{Title: "b"})
	c, _ := s.Create(ctx, model.CreateSpec{Title: "c"})
	mustAdd(t, s, a.ID, b.ID) // a blocked_by b
	mustAdd(t, s, b.ID, c.ID) // b blocked_by c

	// c blocked_by a closes a → b → c → a.
	_, defects := s.AddBlockedBy(ctx, c.ID, strconv.Itoa(a.ID))
	if len(defects) == 0 {
		t.Fatalf("cycle-closing edge accepted")
	}
	want := "#" + strconv.Itoa(c.ID) + " → #" + strconv.Itoa(a.ID) + " → #" + strconv.Itoa(b.ID) + " → #" + strconv.Itoa(c.ID)
	if !strings.Contains(defects[0].Reason, want) {
		t.Fatalf("cycle path = %q, want it to contain %q", defects[0].Reason, want)
	}
}

// A duplicate add is a recorded no-op: the edge is not written twice and the
// updated stamp does not move.
func TestDuplicateEdgeIsNoOp(t *testing.T) {
	s := New(t.TempDir(), advancingClock())
	ctx := context.Background()
	a, _ := s.Create(ctx, model.CreateSpec{Title: "a"})
	b, _ := s.Create(ctx, model.CreateSpec{Title: "b"})

	first, defects := s.AddBlockedBy(ctx, a.ID, strconv.Itoa(b.ID))
	if len(defects) > 0 {
		t.Fatalf("first add: %v", defects)
	}
	again, defects := s.AddBlockedBy(ctx, a.ID, strconv.Itoa(b.ID))
	if len(defects) > 0 {
		t.Fatalf("duplicate add reported defects: %v", defects)
	}
	if len(again.BlockedBy) != 1 {
		t.Fatalf("duplicate add wrote the edge twice: %v", again.BlockedBy)
	}
	if !again.Updated.Equal(first.Updated) {
		t.Fatalf("no-op add moved the updated stamp: %v → %v", first.Updated, again.Updated)
	}
}

func TestDependencyOperationsRejectBadSourcesAndReferences(t *testing.T) {
	s := New(t.TempDir(), advancingClock())
	ctx := context.Background()
	if _, defects := s.AddBlockedBy(ctx, 99, "1"); len(defects) == 0 {
		t.Fatal("missing source was accepted")
	}
	item, _ := s.Create(ctx, model.CreateSpec{Title: "item"})
	if _, defects := s.AddBlockedBy(ctx, item.ID, "not-a-reference"); len(defects) == 0 {
		t.Fatal("malformed blocker was accepted")
	}
	if _, defects := s.AddBlockedBy(ctx, item.ID, "999"); len(defects) == 0 {
		t.Fatal("dangling local blocker was accepted")
	}
	before, defects := s.RemoveBlockedBy(ctx, item.ID, "absent")
	if len(defects) != 0 || before.ID != item.ID {
		t.Fatalf("absent removal = %+v, %v", before, defects)
	}
	if got, defects := s.Blocking(ctx, item.ID); len(defects) != 0 || len(got) != 0 {
		t.Fatalf("Blocking(empty) = %v, %v", got, defects)
	}
}

func mustAdd(t *testing.T, s *Store, id, target int) {
	t.Helper()
	if _, defects := s.AddBlockedBy(context.Background(), id, strconv.Itoa(target)); len(defects) > 0 {
		t.Fatalf("add #%d blocked_by #%d: %v", id, target, defects)
	}
}
