package gate

import (
	"github.com/agenticlab-ai/pyxis/internal/argus/attribution"
)

// The Argus attribution gate (docs/tech-design/argus-attribution-gate.md §3.4):
// a self-registered, base-requiring check that derives each of the PR's diffs'
// Argus authorship from in-tree commit trailers read via `git` — no ambient
// credential, no mutable badge (DINV-3) — and refuses the PR when an
// Argus-authored (or, on a protected-path diff, an authorship-underivable)
// change touches a protected path, reusing the shared classifier and the
// author-independent scope resolver (internal/argus/boundary). A human-authored
// diff is never refused (DINV-4).
//
// It composes through the registry — one new file, zero shared enumeration
// lines (§3.4, Q3; AP-27) — and runs in the already-required docs-gate CI
// context (which invokes the umbrella with --merge-base origin/main), so no new
// workflow or credential is introduced. It is base-requiring (NeedsMergeBase):
// the umbrella skips it with no merge base, exactly as the ledger-history arm
// degrades, and every non-offending tree's verdict is unchanged (report-only).
//
// Bootstrap note (incident I1 / UB-5): this check cannot gate the very PR that
// introduces it — operator review is the sole backstop for that one PR; it
// gates every subsequent PR.
func init() {
	Register(Check{
		Verb:             attribution.Verb,
		Help:             "an Argus-authored (or underivable) diff touching a protected path is refused at the merge boundary",
		NeedsMergeBase:   true,
		RunOnPyxisSource: true,
		Run:              func(c Ctx) Defects { return attribution.Evaluate(c.Root, c.MergeBase, c.Cfg) },
	})
}
