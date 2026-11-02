# Amendment 002 — Slice-scoped plan coverage: plan an amendment without re-covering shipped work

| Field | Value |
|---|---|
| **Amends** | plan-new *(chain: 000 + 001)* |
| **Status** | Approved |
| **Owner** | @mdarabi |
| **Date** | 2026-07-15 |
| **Learned-from** | build-result:plangate-fold-rejects-amendment-plans-2026-07-15 · operator-decision:fix-amendment-coverage-gap-2026-07-15 |
| **Blast radius** | tier: contract-changing; touches: FR-15, FR-26; reason: supersedes the Must coverage-completion criterion (FR-15) with a slice-scoped one |
| **Template version** | amendment-template/v2 |

## Operations

### op: supersede FR-15

Reason: FR-15's "every in-scope requirement of the governing PRD" reads, in the fold, as every live **Must** FR of the whole feature — so the moment a feature is amended with a new requirement, every existing plan of that feature becomes uncoverable (a frozen plan cannot be edited to add or defer it) and a new plan for the amendment would have to re-cover already-shipped, already-planned work. Observed three times in one session: the plan-new Carina adapter (FR-25) could not be planned; local-work-tracker's frozen plan 001 went red after FR-19 was added; the hierarchy-index cannot be planned. Coverage must be scoped to the plan's **slice**, with feature-level Must completeness taken as the **union** across the feature's plans.

| ID | Requirement (EARS) | Rationale | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|
| FR-26 | When a plan is assembled, the planner shall check coverage against the plan's **in-scope slice** — the live requirements its governing design serves — mapping every slice requirement to at least one task and every task to at least one requirement or design anchor, and reporting any unmapped in-scope item as a named defect; a live requirement that a prior plan of the same feature already covers shall not be re-demanded of a later plan, and feature-level Must completeness shall be the **union** of coverage across the feature's plans, a live Must in no plan's slice being reported as a named feature-level coverage defect (never silently dropped). | the completion criterion is coverage of the plan's slice, not re-coverage of the whole feature on every plan — otherwise an amendment makes existing plans uncoverable and forces re-planning shipped work (the amendment-coverage gap, observed 3×); the union across a feature's plans is the feature-level bar, so nothing Must is dropped | Must | high | fixtures: an amendment-slice plan covering only its design's new requirements validates; a frozen base plan stays green after a later amendment adds a requirement served by a later plan's design; a live Must in no plan's slice → a named feature-level coverage defect; a dropped in-scope requirement or a rogue task → a named defect | Test | J4, G1 |
