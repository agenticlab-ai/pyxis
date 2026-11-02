// Package tracker is the projection's one narrow seam to the tracker
// medium (plan-new-v1 §3.2/§3.7): the RW-4 deviation's mitigation is that
// every GitHub-ism lives behind this interface in its adapter file, so the
// WorkTracker-port rebinding (PRD WR-3) is adapter work. Consumers accept
// the interface; the gh adapter returns concrete results (repo Go rules).
package tracker

import (
	"errors"
	"slices"
	"strings"
)

// Issue is the projection's read model — exactly the fields the probe and
// wiring decisions consume, nothing more (AP-6 compactness).
type Issue struct {
	Number int
	Title  string
	Body   string
	Labels []string
	State  string // "open" | "closed"
	Type   string // native issue type name, "" when unset
}

// HasLabel reports whether the issue carries the named label. Label names
// match without regard to case, as GitHub and Forgejo treat them: a
// repository's existing `Local-Agent` is the routing label `local-agent`.
func (i Issue) HasLabel(name string) bool {
	return HasLabel(i.Labels, name)
}

// HasLabel reports whether labels hold name, ignoring case.
func HasLabel(labels []string, name string) bool {
	return slices.ContainsFunc(labels, func(l string) bool { return strings.EqualFold(l, name) })
}

// ErrInPlaceUnsupported reports a tracker that cannot rewrite a comment, so
// a plan changed in place cannot reach its tasks there.
var ErrInPlaceUnsupported = errors.New("tracker cannot change a task's charter in place")

// Tracker is the write-narrow projection surface. Every read backing a
// decision is an exact, bounded enumeration (DINV-4); the interface has no
// close, and its edits rewrite or withdraw only what the projection itself
// wrote — a task or epic issue's title and body, a task's spec comment, a
// routing or classification label, and a dependency edge between the plan's
// own tasks — so the projection mutates nothing it did not create (DINV-7).
type Tracker interface {
	// EnsureLabel creates the label iff missing; an existing label is
	// never recolored or redescribed (NR-2: no mutation of what exists).
	EnsureLabel(name, color, description string) error
	// CreateIssue opens an issue and returns its number. issueType names a
	// native issue type ("" = none); an unknown type falls back to untyped
	// creation rather than failing the projection (legibility is additive).
	CreateIssue(title, body string, labels []string, issueType string) (int, error)
	// OpenIssues enumerates open issues (exact REST listing, paginated) —
	// the stray-scan half of the idempotency probe.
	OpenIssues() ([]Issue, error)
	// SubIssues enumerates an issue's children (exact listing).
	SubIssues(parent int) ([]Issue, error)
	// LinkSubIssue makes child a sub-issue of parent (additive only).
	LinkSubIssue(parent, child int) error
	// BlockedBy lists the issue's existing blocked-by edge targets.
	BlockedBy(issue int) ([]int, error)
	// AddBlockedBy wires issue ← blocker (additive; re-adding converges).
	AddBlockedBy(issue, blocker int) error
	// RemoveBlockedBy drops the edge issue ← blocker that a plan changed in
	// place no longer names.
	RemoveBlockedBy(issue, blocker int) error
	// AddLabels applies labels to an issue (additive; existing = no-op).
	AddLabels(issue int, labels []string) error
	// RemoveLabel withdraws a routing or classification label that a plan
	// changed in place no longer gives the task.
	RemoveLabel(issue int, label string) error
	// Comments lists an issue's comment bodies (exact listing, in order).
	Comments(issue int) ([]string, error)
	// Comment appends a comment.
	Comment(issue int, body string) error
	// EditIssue rewrites the title and body of an issue the projection
	// created, when the plan it was projected from changed in place.
	EditIssue(issue int, title, body string) error
	// ReplaceComment rewrites the one comment on issue whose body is old:
	// the projection's own spec comment, when the plan changed in place.
	// A tracker whose comments are immutable returns ErrInPlaceUnsupported.
	ReplaceComment(issue int, old, body string) error
}
