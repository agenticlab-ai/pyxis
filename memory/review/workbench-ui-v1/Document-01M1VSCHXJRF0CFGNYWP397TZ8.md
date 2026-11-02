# Review: workbench-ui-v1 — design-review report

| Field | Value |
|---|---|
| **In one line** | Approve-with-advisories: the pure-Node decision is faithfully assembled from its record and executable; four advisories applied — an anchor-instead-of-restated budget, the faceless-lane registry example, per-working-tree session wording, and an asset-traversal bound. |
| **Relevant-when** | auditing or consuming the review of workbench-ui-v1; asking why it was approved; tuning tech-design-review calibration |
| **Subject** | design:workbench-ui-v1 · shape: present · supersedes: — |
| **Moment** | new-design |
| **Reviewer** | tech-design-review · guide as merged 2026-07-10 |
| **Date(s)** | 2026-07-10 |
| **Verdict** | approve-with-advisories |
| **Tracking** | the subject's introducing PR (this PR) |
| **Template version** | design-review-report/v1 |

## Round 1

### FND-1 — A-9 restates the PRD's budget numbers instead of anchoring them

- **Severity**: advisory (CAL-3)
- **Lens**: contract-quality
- **Evidence**: design: §6 A-9 evidence "(p95 ≤ 3s launch; p95 ≤ 300ms request; p99 ≤ 1s)" — vs — the template's frozen-prose rule (state no volatile facts; point at the surface that owns the value) and prd:workbench-ui/NFR-1, which owns these numbers and can move by amendment while the frozen design cannot.
- **Acceptance condition**: A-9's evidence cites the budgets by anchor (prd:workbench-ui/NFR-1) rather than restating them.
- **Disposition**: applied — A-9 evidence now names the benchmark run against NFR-1's fit criterion.

### FND-2 — the shape shows only the faced registry lane; the harder faceless lane has no concrete example

- **Severity**: advisory (CAL-4)
- **Lens**: feasibility-completeness
- **Evidence**: shape: the class-registry element carries one example (`"id": "tech-design"`, `face: index-json`) — vs — design §3.1, whose riskier lane (R1's whole surface) is the faceless header/anchor scan; an implementer reads the shape for exactly that variant.
- **Acceptance condition**: the registry element also shows a faceless entry (face: null, scan grammars carried).
- **Disposition**: applied — an incidents-class example (face null, anchor grammar `I-n`) added beside the faced one.

### FND-3 — "one session per repository" misstates the mechanism's actual boundary

- **Severity**: advisory (CAL-3)
- **Lens**: contract-quality
- **Evidence**: design: §3.6 "One session per repository" — vs — the same section's mechanism (the statefile lives in the working tree's render scratch), which bounds sessions per **working tree**; the record's G-3 asked precisely the worktree question, and the answer as written contradicts the mechanism.
- **Acceptance condition**: the Decision states the per-working-tree boundary explicitly.
- **Disposition**: applied — §3.6 now reads "one session per working tree (the statefile's home); sibling worktrees of one repository run independent sessions over their own corpus states."

### FND-4 — the asset route carries no traversal bound

- **Severity**: advisory (CAL-4)
- **Lens**: feasibility-completeness
- **Evidence**: shape: route table `GET /s/<secret>/assets/<file>` with no path constraint — vs — design §3.5's threat sketch (the served-content surface) and the defense-in-depth posture of DINV-3; a traversal under a leaked secret widens R5 beyond the corpus.
- **Acceptance condition**: the route block bounds asset resolution to the render directory, traversal refused.
- **Disposition**: applied — the route table now states assets resolve inside the render directory only; traversal → 404 quadruple.

## Verdict history

| Round | Verdict | Date |
|---|---|---|
| 1 | approve-with-advisories | 2026-07-10 |

## Escalation brief

none

## Measurements

- **Rounds**: 1
- **Findings**: 0 blocking · 4 advisory
- **Dispositions**: 4 applied · 0 declined · 0 withheld-as-violation

## Overlay corrections

none
