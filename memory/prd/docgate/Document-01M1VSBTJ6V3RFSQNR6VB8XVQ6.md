# Amendment 016 — content-path purity: the corpus cites version pins, never the system footprint

| Field | Value |
|---|---|
| **Amends** | docgate (chain: 000 → 015) |
| **Status** | Approved |
| **Date** | 2026-07-10 |
| **Owner** | @mdarabi |
| **Learned-from** | review-thread:issue-54 · operator-decision:2026-07-09-foundation-focus |
| **Blast radius** | tier: additive; touches: FR-31; reason: adds a check over previously ungated citation surfaces; no existing unit changes |
| **Template version** | amendment-template/v2 |

## Operations

### op: add FR

Reason: the repo-layout design ruled that content never links into the
system footprint by path — documents cite templates by version pin, so
bucket-1 content stays interpretable after `pyxis remove` (A-2/DINV-4
there) — and promised the rule as a grep gate that was never built
(#54 finding A5/5b). Today the rule is violated in three editable places
(the requirements index's template links, two wiki entries), and nothing
stops new couplings. The check lands scoped to where the rule is
load-bearing and enforceable forward: frozen history is never
retro-edited, the rollups legitimately describe the shipped footprint,
and the root operating manual is the pointer surface that *owns* those
paths.

| ID | Requirement (EARS) | Rationale | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|
| FR-31 | When invoked as `check-content-paths`, docgate shall fail any system-footprint path reference (a `.pyxis/` path) in wiki entries or in the flat requirement corpus (its README included), and — when a merge base is given — in any PRD ledger entry or technical design added since the merge base; the live rollups, the root operating manual, generated artifacts, and documents present at the merge base are out of scope (rollups describe the shipped footprint; the root manual owns the pointers; frozen history is never retro-edited). Templates are cited by version pin. | the corpus must survive footprint removal interpretable (repo-layout A-2/DINV-4); the promised grep gate becomes real, scoped forward-only (#54 A5) | Must | low | fixtures: a wiki entry or corpus file containing a footprint path → red naming file and line; a ledger entry or design added since the merge base containing one → red; a pre-existing frozen document containing one → green; rollup and root-manual references → green; the real corpus green after the three known violations are fixed | Test | J3, G1 |
