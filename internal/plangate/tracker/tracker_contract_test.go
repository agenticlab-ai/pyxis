package tracker

import (
	"strings"
	"testing"
)

// runTrackerContract exercises the Tracker interface behaviorally — the shared
// contract every adapter must satisfy (design plangate-carina-adapter A-1):
// create → label → link → wire → comment, then the read half the marker probe
// depends on, plus edge idempotency. Adapter-specific mappings (labels,
// identity) are asserted by each adapter's own test, not here.
func runTrackerContract(t *testing.T, tr Tracker) {
	t.Helper()
	const epicMarker = "<!-- test:epic -->"

	if err := tr.EnsureLabel("kind:feat", "0e8a16", "feature"); err != nil {
		t.Fatalf("EnsureLabel: %v", err)
	}
	epic, err := tr.CreateIssue("Epic", "epic body "+epicMarker, []string{"kind:feat"}, "epic")
	if err != nil || epic <= 0 {
		t.Fatalf("CreateIssue epic: id=%d err=%v", epic, err)
	}
	t1, err := tr.CreateIssue("T1", "t1 body", []string{"kind:feat"}, "task")
	if err != nil || t1 <= 0 {
		t.Fatalf("CreateIssue t1: id=%d err=%v", t1, err)
	}
	t2, err := tr.CreateIssue("T2", "t2 body", []string{"kind:feat"}, "task")
	if err != nil || t2 <= 0 {
		t.Fatalf("CreateIssue t2: id=%d err=%v", t2, err)
	}
	for _, child := range []int{t1, t2} {
		if err := tr.LinkSubIssue(epic, child); err != nil {
			t.Fatalf("LinkSubIssue %d: %v", child, err)
		}
	}
	if err := tr.AddBlockedBy(t2, t1); err != nil {
		t.Fatalf("AddBlockedBy: %v", err)
	}
	if err := tr.Comment(t1, "a comment"); err != nil {
		t.Fatalf("Comment: %v", err)
	}

	// OpenIssues carries bodies (the epic marker) — the stray-scan probe.
	open, err := tr.OpenIssues()
	if err != nil {
		t.Fatalf("OpenIssues: %v", err)
	}
	if !anyBodyContains(open, epicMarker) {
		t.Fatalf("OpenIssues did not surface the epic body marker (bodies missing?) — got %d issues", len(open))
	}
	// SubIssues(epic) includes both children.
	subs, err := tr.SubIssues(epic)
	if err != nil {
		t.Fatalf("SubIssues: %v", err)
	}
	if !hasNumber(subs, t1) || !hasNumber(subs, t2) {
		t.Fatalf("SubIssues(%d) missing a child: got %v want %d,%d", epic, numbers(subs), t1, t2)
	}
	// BlockedBy(t2) resolves t1.
	bb, err := tr.BlockedBy(t2)
	if err != nil {
		t.Fatalf("BlockedBy: %v", err)
	}
	if countInt(bb, t1) != 1 {
		t.Fatalf("BlockedBy(%d) = %v, want exactly one %d", t2, bb, t1)
	}
	// Comments(t1) includes the comment.
	cs, err := tr.Comments(t1)
	if err != nil {
		t.Fatalf("Comments: %v", err)
	}
	if !hasString(cs, "a comment") {
		t.Fatalf("Comments(%d) = %v, missing the comment", t1, cs)
	}

	// Idempotency: re-link and re-wire converge — no error, no duplicate edge.
	if err := tr.LinkSubIssue(epic, t1); err != nil {
		t.Fatalf("re-LinkSubIssue: %v", err)
	}
	if err := tr.AddBlockedBy(t2, t1); err != nil {
		t.Fatalf("re-AddBlockedBy: %v", err)
	}
	bb2, _ := tr.BlockedBy(t2)
	if countInt(bb2, t1) != 1 {
		t.Fatalf("blocked-by not idempotent: %v", bb2)
	}
}

func anyBodyContains(issues []Issue, s string) bool {
	for _, i := range issues {
		if strings.Contains(i.Body, s) {
			return true
		}
	}
	return false
}
func hasNumber(issues []Issue, n int) bool {
	for _, i := range issues {
		if i.Number == n {
			return true
		}
	}
	return false
}
func numbers(issues []Issue) []int {
	out := make([]int, 0, len(issues))
	for _, i := range issues {
		out = append(out, i.Number)
	}
	return out
}
func countInt(xs []int, n int) int {
	c := 0
	for _, x := range xs {
		if x == n {
			c++
		}
	}
	return c
}
func hasString(xs []string, s string) bool {
	for _, x := range xs {
		if x == s {
			return true
		}
	}
	return false
}
