# Amendment 001 — NR-4 corrective: the lane consumption is owed, not existing

| Field | Value |
|---|---|
| **Amends** | impl-review (chain: 000) |
| **Status** | Approved |
| **Owner** | @mdarabi |
| **Date** | 2026-07-18 |
| **Learned-from** | review-thread:impl-review-v1-design-review · operator-decision:2026-07-17-nr4-owed-corrective |
| **Blast radius** | tier: corrective; touches: NR-4; reason: supersedes a non-Must non-regression whose fit criterion overstated the integration — the merge-lane consumption of the reviewer's downgrade signal is an owed amendment, not the existing GATE-4 slot |
| **Template version** | amendment-template/v2 |

## Operations

### op: supersede NR-4

Reason: the merged design impl-review-v1 §3.6 showed NR-4 overstated the integration — "the reviewer emits into the lane's existing downgrade-only slot" and "the downgrade signal is consumed by the existing GATE-4 slot" imply a lane consumption that does not exist; merge-lane's `Downgrader` seam is synchronous, diff-only, and nil-by-default, so consuming the impl-review verdict is an owed separate merge-lane amendment (design §7 Q4), not the existing slot today.

| ID | Existing behavior protected | Why protected | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|
| NR-5 | the merge lane's classifier and floor are untouched by impl-review, which adds no lane-side risk logic; the reviewer EMITS a well-formed downgrade signal (the persisted verdict) — the lane's CONSUMPTION of that signal is an owed separate merge-lane amendment (a `Downgrader` adapter that reads the verdict), not the existing GATE-4 slot today | AP-11 — one risk predicate; impl-review reads the lane's risk classes and emits a downgrade signal, it does not redesign the lane. The base NR-4 overstated "the existing slot consumes it": merge-lane's `Downgrader` seam is synchronous, diff-only, and nil-by-default, so end-to-end lane-forcing requires the owed adapter (design:impl-review-v1#3.6, §7 Q4; the codegate-NR-2 corrective precedent) | med | zero impl-review-side diff to the merge lane's classifier or floor (inspection); the persisted verdict validates as a well-formed downgrade signal a lane adapter can read; the consuming `Downgrader` adapter is named owed work (design:impl-review-v1 §7 Q4), gated before adoption relies on lane-forcing — never asserted as an existing consumption | Inspection | FR-7, F3 |
