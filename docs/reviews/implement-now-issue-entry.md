# Review: implement-now-issue-entry — design-review report

| Field | Value |
|---|---|
| **In one line** | Approved: bounded selection reads and existing execution/recovery authority are explicit. |
| **Relevant-when** | auditing the direct issue entry design and its exact selection boundary |
| **Subject** | design:implement-now-issue-entry · shape: none · supersedes: — |
| **Moment** | new-design |
| **Reviewer** | tech-design-review · 2026-10-01 |
| **Date(s)** | 2026-10-01 |
| **Verdict** | approve |
| **Tracking** | — |
| **Template version** | design-review-report/v1 |

## Round 1

Four lenses used the same docgate context pack with informative §0 excluded. Independent screening retained this feasibility finding. The candidate requiring a second planning-stage authorization system was screened out against the operator's explicit bounded issue lane; §3.1 states that this lane does not claim the full planning workflow executed.

### FND-1 — Literal zero shared obligation reads contradict substrate reuse

- **Severity**: blocking (CAL-4; load-bearing for A-2)
- **Lens**: feasibility-completeness
- **Evidence**: Design §3.2 originally restricted selection before any obligation read, while §3.3/§3.4 reuse the existing Git-backed substrate and review machinery. `reviewobligation.GitStore.Open(scope)` fetches/decode-selects shared metadata before returning the exact scope. A-2 cannot satisfy a literal prohibition of that shared index read through the chosen seam.
- **Acceptance condition**: Permit bounded shared metadata/index selection and exact source/blocker fact reads; apply the contract restriction before unrelated proposal/artifact lookup, repair, backend authentication, claim, or model launch.
- **Disposition**: applied — §3.2 states this precise read exception without expanding execution authority. §3.8 also declares the frozen NR-1 consumer impact found by PRD review.

## Round 2

Scope-locked review closed FND-1 and found no surviving changed-span regression. A retry-wording candidate was screened out against the inherited prohibition on redispatch from check color alone; §3.4 explicitly separates original-worker bounded repair from later selected review/metadata recovery. The other revised spans accurately declare fingerprint, local-proof, retained-observation, historical-citation, and evaluation boundaries.

## Verdict history

| Round | Verdict | Date |
|---|---|---|
| 1 | request-changes | 2026-10-01 |
| 2 | approve | 2026-10-01 |

## Escalation brief

none

## Measurements

- **Rounds**: 2
- **Findings**: 1 blocking · 0 advisory
- **Dispositions**: 1 applied · 0 declined · 0 withheld-as-violation

## Overlay corrections

none
