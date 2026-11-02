package localfs

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/worktracker"
	"github.com/agenticlab-ai/pyxis/internal/worktracker/model"
)

// The epic rollup reports exactly the *open* children: a closed child is counted
// out (prd:local-work-tracker/FR-7). The transition verb that closes an item is
// a later task, so this white-box test closes a child by rewriting its state on
// disk — a legitimate stored state the port cannot yet reach — and asserts the
// rollup excludes it while the open siblings remain.
func TestRollupExcludesClosedChild(t *testing.T) {
	s := New(t.TempDir(), fixedClock())
	ctx := context.Background()
	epic, _ := s.Create(ctx, model.CreateSpec{Type: model.TypeEpic, Title: "epic"})

	var childIDs []int
	for i := 0; i < 3; i++ {
		c, defects := s.Create(ctx, model.CreateSpec{Title: "child"})
		if len(defects) > 0 {
			t.Fatalf("create child: %v", defects)
		}
		if _, defects := s.SetParent(ctx, c.ID, epic.ID); len(defects) > 0 {
			t.Fatalf("set parent: %v", defects)
		}
		childIDs = append(childIDs, c.ID)
	}

	// Close the middle child by rewriting its authoritative file.
	closeChild(t, s, childIDs[1])

	kids, defects := s.Children(ctx, epic.ID, worktracker.ChildFilter{}, worktracker.Display)
	if len(defects) > 0 {
		t.Fatalf("children: %v", defects)
	}
	if len(kids) != 3 {
		t.Fatalf("Children should still see all 3 children, got %d", len(kids))
	}
	open := model.OpenChildren(kids)
	if len(open) != 2 {
		t.Fatalf("open-child count = %d, want 2", len(open))
	}
	for _, c := range open {
		if c.ID == childIDs[1] {
			t.Fatalf("closed child #%d appeared in the open rollup", childIDs[1])
		}
	}
}

// An item cannot be its own parent — the self-reference is refused naming it,
// and nothing is written (fail closed).
func TestSetParentRefusesSelf(t *testing.T) {
	s := New(t.TempDir(), fixedClock())
	ctx := context.Background()
	item, _ := s.Create(ctx, model.CreateSpec{Title: "loner"})

	_, defects := s.SetParent(ctx, item.ID, item.ID)
	if len(defects) == 0 {
		t.Fatalf("self-parent was accepted")
	}
	if !strings.Contains(defects[0].Reason, "its own parent") {
		t.Fatalf("self-parent defect wrong: %+v", defects[0])
	}
	got, _ := s.Get(ctx, item.ID)
	if got.Parent != "" {
		t.Fatalf("self-parent was written: parent=%q", got.Parent)
	}
}

// closeChild rewrites a child's file with state closed, standing in for the
// transition verb (a later task).
func closeChild(t *testing.T, s *Store, id int) {
	t.Helper()
	w, defects := s.Get(context.Background(), id)
	if len(defects) > 0 {
		t.Fatalf("get #%d: %v", id, defects)
	}
	w.State = model.StateClosed
	if err := os.WriteFile(itemPath(s.itemsDir(), id), Render(w), 0o644); err != nil {
		t.Fatalf("close #%d: %v", id, err)
	}
}
