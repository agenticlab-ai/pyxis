// Package ghtracker implements the executor's WorkTracker port against a
// GitHub issue tracker (docs/tech-design/implement-now.md §3.1, §3.10 — the S-5
// seam). It reads the ready frontier — the OPEN issues tagged for autonomous
// agent pickup (the harbormaster dialect's "local-agent" routing label,
// consumed unchanged: prd:implement-now/NR-1) — and posts progress comments
// back onto an item. Every decision-critical read is exact and fresh: an exact
// REST listing, never the eventually-consistent search index, and never a
// cacheable conditional GET (AP-15; the predecessor's I4/I12 lesson).
//
// All GitHub interaction sits behind a narrow, consumer-side client interface
// (client). The production implementation shells an authenticated `gh api`
// (client.go). The adapter's own logic — the open ∧ tagged ∧ not-a-PR filter
// and the issue → port.Task mapping — is unit-tested against a fake client with
// recorded responses, so it is hermetic (no network, no gh). The executor only
// reads and comments through this port; it never merges (prd:implement-now/NR-3,
// DINV-6).
package ghtracker
