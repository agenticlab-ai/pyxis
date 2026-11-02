# Review: feature-implementation-drive-failure-contract — design-review report

| Field | Value |
|---|---|
| **In one line** | Approved after closing the compound-failure priority gap and clarifying the NR-6 guardrail's place in the closed vocabulary. |
| **Relevant-when** | auditing or consuming the review of the feature implementation drive failure contract; asking why it was approved; tuning tech-design-review calibration |
| **Subject** | design:feature-implementation-drive-failure-contract · shape: none · supersedes: — |
| **Moment** | new-design |
| **Reviewer** | tech-design-review · 2026-07-25 |
| **Date(s)** | 2026-07-25 (rounds 1–2) |
| **Verdict** | approve |
| **Tracking** | PR #706 |
| **Template version** | design-review-report/v1 |

## Round 1

### FND-1 — Compound-failure priority was not closed

- **Severity**: blocking (CAL-3 — the binding predicate and `DINV-1` were not fully testable)
- **Lens**: contract-quality
- **Evidence**: design: §3.1 said only that ownership invalidity outranked owner-derived recovery, cancellation outranked new work, and phase failures blocked downstream phases, while R2 required a compound-defect fixture for every priority boundary — vs — `DINV-1`, which requires exactly one primary disposition for every ambiguous observation.
- **Acceptance condition**: define a closed ordering for cross-cutting triggers and a deterministic rule for multiple or phase-illegal local triggers, sufficient to derive every compound fixture's expected result without adapter ordering.
- **Disposition**: applied — §3.1 now gives the complete cross-cutting order and maps multiple or phase-illegal local triggers to `malformed-state`.

### FND-2 — The main-before-final guard could be read as an undeclared twelfth trigger

- **Severity**: advisory (CAL-2)
- **Lens**: upstream-alignment
- **Evidence**: design: §3.1 called eleven rows the complete v1 vocabulary, then classified a seeded main-before-final revision as `malformed-state` — vs — `prd:feature-implementation-drive/NR-6`, which is a safety constraint served in addition to UB-1..UB-11, not a new recoverable unwanted-behavior row.
- **Acceptance condition**: state whether the NR-6 observation extends the trigger vocabulary or enters an existing fallback, preserving honest one-to-one UB coverage.
- **Disposition**: applied — §3.1 identifies it as a guardrail observation entering the existing unknown/impossible-state fallback, not a twelfth recoverable trigger.

## Round 2

Scope-locked re-review inspected the two revised §3.1 spans against `DINV-1`, R2, A-7, and `prd:feature-implementation-drive/NR-6`. The closed cross-cutting order plus the malformed-state rule now determines every compound result. The guardrail clarification preserves the eleven-row UB vocabulary while substantively serving NR-6. No unchanged content was reopened and no new finding arose from the changes.

## Verdict history

| Round | Verdict | Date |
|---|---|---|
| 1 | request-changes | 2026-07-25 |
| 2 | approve | 2026-07-25 |

## Escalation brief

none

## Measurements

- **Rounds**: 2
- **Findings**: 1 blocking · 1 advisory
- **Dispositions**: 2 applied · 0 declined · 0 withheld-as-violation

## Overlay corrections

none
