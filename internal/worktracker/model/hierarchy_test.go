package model

import "testing"

func TestOpenChildren(t *testing.T) {
	children := []WorkItem{
		{ID: 1, State: StateOpen},
		{ID: 2, State: StateClosed},
		{ID: 3, State: StateOpen},
	}
	open := OpenChildren(children)
	if len(open) != 2 {
		t.Fatalf("open count = %d, want 2", len(open))
	}
	if open[0].ID != 1 || open[1].ID != 3 {
		t.Fatalf("open ids = [%d %d], want [1 3]", open[0].ID, open[1].ID)
	}
	// Order is preserved and the closed child is excluded.
	if OpenChildren(nil) == nil {
		t.Fatalf("OpenChildren(nil) should be an empty, non-nil slice")
	}
}
