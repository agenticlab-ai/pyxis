# Review: state-reader-v5 — design-review report

| Field | Value |
|---|---|
| **In one line** | Approve: exact predecessor carry-forward and the provider/recovery join now preserve handoff, read-only status, unknown state, and durable evidence without laundering. |
| **Relevant-when** | auditing or consuming state-reader-v5; checking review-readiness joins, status purity, or predecessor conformance |
| **Subject** | design:state-reader-v5 · shape: none · supersedes: state-reader-v4 |
| **Moment** | supersession |
| **Reviewer** | tech-design-review · 2026-08-06 guide |
| **Date(s)** | 2026-08-06 |
| **Verdict** | approve |
| **Tracking** | issue:1029 · PR #1034 |
| **Template version** | design-review-report/v1 |

## Round 1

### FND-1 — exact incorporation omitted two predecessor acceptance rows

- **Severity**: blocking (CAL-1)
- **Lens**: coherence
- **Evidence**: design §2 pinned state-reader-v4 but carried only A-1..A-19 — vs — the pinned object, which contains A-20 and A-21, and successor A-1's every-anchor claim.
- **Acceptance condition**: carry A-1..A-21 and require A-20/A-21 mappings in the closed manifest and negative corpus.
- **Disposition**: applied

## Round 2

Scope-locked convergence found FND-1 closed. The frozen hash matches, the exact range is A-1..A-21, and the closed manifest/checker rejects missing, duplicate, unknown, or favorable-subset mappings. The final design also distinguishes `handoff_pending` from `review_pending`, and `DeriveReadiness` from zero-write `SnapshotStatus`, with typed unobserved semantics and acceptance coverage.

## Verdict history

| Round | Verdict | Date |
|---|---|---|
| 1 | request-changes | 2026-08-06 |
| 2 | approve | 2026-08-06 |

## Escalation brief

none

## Measurements

- **Rounds**: 2
- **Findings**: 1 blocking · 0 advisory
- **Dispositions**: 1 applied · 0 declined · 0 withheld-as-violation

## Overlay corrections

none
