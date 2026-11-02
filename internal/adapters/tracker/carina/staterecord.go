package carinatracker

import (
	"sort"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/executor/port"
	"github.com/agenticlab-ai/pyxis/internal/worktracker/model"
)

// The record-to-state field mapping (docs/tech-design/state-reader-v2.md
// §3.7, Q1 — the frozen design's one open question, resolved here). A
// Carina-tracked run has no separate review-request record: the work item
// itself is the trackable unit the reader answers about, so its own
// open/closed lifecycle stands in for a proposal's landing, its governed
// status-role signals (local-work-tracker-v2 §3.5/§3.14, model.SeedSignals)
// are the only verdict-like surface the closed schema carries, and its
// append-only comment entries are the notes surface. Nothing here reads a
// field local-work-tracker-v2 does not already define, and nothing here
// writes at all (out of scope: no schema or write-path change).

// landing derives a work item's Landing claim and, when landed, the revision
// id a consumer verifies mainline ancestry from with no further read
// (DINV-3): an open item is open; a closed item is landed only when its
// close_reason is the store's own "done" marker (model.CloseCompleted) —
// the plan's "accepted" marker — and anything else closed (rejected, or a
// hand-edited empty reason) is closed-unlanded, the safe-in-the-strict-
// direction default (mirroring state-reader-v2 §3.6's own required-check
// fallback: an ambiguous case reads as the stricter answer, never a
// generous one).
func landing(w model.WorkItem) (port.Landing, string) {
	switch w.State {
	case model.StateOpen:
		return port.LandingOpen, ""
	case model.StateClosed:
		if w.CloseReason == model.CloseCompleted {
			return port.LandingLanded, landedRevision(w)
		}
		return port.LandingClosedUnlanded, ""
	default:
		// Unreachable in practice: Store.Get already refuses an out-of-set
		// State at parse time, so deriveState's caller never reaches here
		// with a malformed item — this is a fail-closed backstop only.
		return port.LandingUnknown, ""
	}
}

// landedRevision answers the "recorded commit" state-reader-v2 §3.7 asks
// for. Carina has no git-commit concept — a work item is a backlog record,
// not a code artifact — so the closest already-recorded, immutable
// identifier for "the moment this landed" is the closing transition's own
// append-log timestamp: transitions are append-only and never rewritten
// (DINV-5), so the value survives any later, unrelated edit to the item
// (e.g. a title fix after closing). The log is scanned newest-first so a
// reopen-then-reclose is answered by the *last* closing transition, never a
// stale earlier one. A hand-authored file that sets state/close_reason
// directly, with no recorded transition at all, falls back to the item's
// own updated stamp — still a genuinely recorded value, just less precise.
func landedRevision(w model.WorkItem) string {
	for i := len(w.Log) - 1; i >= 0; i-- {
		if w.Log[i].Kind == model.LogTransition {
			return w.Log[i].At.UTC().Format(time.RFC3339)
		}
	}
	return w.Updated.UTC().Format(time.RFC3339)
}

// statusCheckSeverity maps each of local-work-tracker-v2's governed
// status-role signals (model.SeedSignals, model.RoleStatus) to the check
// state it represents when present on an item — the closed schema's only
// verdict-like surface (§3.7's "recorded verdict fields"). "hold" is
// deliberately excluded: it names a scheduling pause, not a verification
// outcome, so it carries no check state.
var statusCheckSeverity = map[string]port.CheckState{
	"worker-failed":    port.CheckRed,
	"merge-conflict":   port.CheckRed,
	"empty-proposal":   port.CheckRed,
	"verify-pending":   port.CheckPending,
	"feedback-stalled": port.CheckPending,
}

// checksFor derives the per-context check states a work item currently
// carries from its status signals, each named after the signal itself so
// the reported context is exactly the governed vocabulary name a human
// already sees on the item — no invented naming. RequiredMissing is always
// empty: Carina has no branch-protection-like declaration of which contexts
// are required (out of scope: no schema change), so there is never a
// required-but-silent context to name.
func checksFor(w model.WorkItem) []port.CheckContext {
	var out []port.CheckContext
	for _, name := range w.Signals {
		state, ok := statusCheckSeverity[name]
		if !ok {
			continue
		}
		out = append(out, port.CheckContext{Name: name, State: state})
	}
	// Signals render in on-disk (insertion) order; sorting keeps the reported
	// context list deterministic regardless of that order.
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// commentsFor projects a work item's free-form comment log entries onto the
// port's Comment shape — the "record notes" state-reader-v2 §3.7 names.
// Decision-records and transitions are structured audit state, not human
// dialogue (mirroring localfs's own Search, which excludes them from its
// text surface too), so only LogComment entries become notes. Carina has
// one dialogue surface, not GitHub's three, so every comment reports as
// SurfaceDiscussion.
func commentsFor(w model.WorkItem) []port.Comment {
	var out []port.Comment
	for _, e := range w.Log {
		if e.Kind != model.LogComment {
			continue
		}
		out = append(out, port.Comment{
			Author:    e.Author,
			CreatedAt: e.At,
			Surface:   port.SurfaceDiscussion,
			Body:      e.Payload,
		})
	}
	return out
}

// deriveState maps one fully-read work item onto its ProposalState — Q1's
// resolution composed. Comments are returned unfiltered here; a caller's
// cursor can differ call to call, so since-filtering happens per read
// (statereader.go), never in this pure mapping.
func deriveState(w model.WorkItem) port.ProposalState {
	l, rev := landing(w)
	return port.ProposalState{
		Landing:        l,
		LandedRevision: rev,
		Checks:         checksFor(w),
		Comments:       commentsFor(w),
	}
}
