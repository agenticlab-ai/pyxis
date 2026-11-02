# Review: wake-watch-v3 — design-review report

| Field | Value |
|---|---|
| **In one line** | Approve: combined readiness, selected recovery, append-first accounting, no-launch exhaustion, and pure status now compose without false idle or implementation redispatch. |
| **Relevant-when** | auditing or consuming wake-watch-v3; checking durable review rounds, bounded recovery, status, or exhaustion/rearm behavior |
| **Subject** | design:wake-watch-v3 · shape: none · supersedes: wake-watch-v2 |
| **Moment** | supersession |
| **Reviewer** | tech-design-review · 2026-08-06 guide |
| **Date(s)** | 2026-08-06 |
| **Verdict** | approve |
| **Tracking** | issue:1029 · PR #1034 |
| **Template version** | design-review-report/v1 |

## Round 1

### FND-1 — handoff-pending had no route before review selection

- **Severity**: blocking (CAL-1)
- **Lens**: coherence
- **Evidence**: provider design §3.3 made `handoff_pending` review-ineligible, while wake §3.2 selected every pending row and state-reader exposed only eligible/exhausted.
- **Acceptance condition**: expose a distinct handoff-repair state, route it through normal executor EVT-10 repair, spend no consumer recovery admission, and prove it cannot render idle.
- **Disposition**: applied

### FND-2 — status purity conflicted with append-first initialization

- **Severity**: blocking (CAL-1)
- **Lens**: coherence
- **Evidence**: the only review-state join initialized first observation, while the exact-incorporated status contract and wake A-6 required zero writes.
- **Acceptance condition**: add a typed read-only snapshot port and never-observed semantics, keep initialization solely in drive derivation, and prove first-status performs zero writes.
- **Disposition**: applied

## Round 2

Scope-locked convergence found both findings closed. `handoff_pending` now schedules only normal exact EVT-10 repair and never consumes recovery; selected review-only requests admit only `review_pending`. Status calls `SnapshotStatus`, renders an unobserved row with literal unknown recovery fields, and an append spy proves zero mutation. The combined-empty, unknown, exhausted, concurrency, and selected-cardinality criteria remain closed.

## Verdict history

| Round | Verdict | Date |
|---|---|---|
| 1 | request-changes | 2026-08-06 |
| 2 | approve | 2026-08-06 |

## Escalation brief

none

## Measurements

- **Rounds**: 2
- **Findings**: 2 blocking · 0 advisory
- **Dispositions**: 2 applied · 0 declined · 0 withheld-as-violation

## Overlay corrections

none
