# Review: execution-demand-contract-v1 — design-review report

| Field | Value |
|---|---|
| **In one line** | Approve: plangate is the sole profile author and capsule refusal is proven on implementation and review paths. |
| **Relevant-when** | auditing or consuming the review of execution-demand-contract-v1; asking why it was approved or bounced; tuning tech-design-review calibration |
| **Subject** | design:execution-demand-contract-v1 · shape: none · supersedes: routing-envelope-v1 |
| **Moment** | supersession |
| **Reviewer** | tech-design-review · 2026-07-28 guide |
| **Date(s)** | 2026-07-28 |
| **Verdict** | approve |
| **Tracking** | — *(the subject's introducing PR, once filed)* |
| **Template version** | design-review-report/v1 |

## Round 1

### FND-1 — derived-profile authorship contradicts the recorded single-author boundary

- **Severity**: blocking (CAL-1)
- **Lens**: coherence
- **Evidence**: design: §3.2 makes planner assembly render the proposed profile — vs — `design-brainstorm:planner-driven-model-routing-tech-design#D-2`, where plangate alone applies floors/promotions/raise and freezes the profile.
- **Acceptance condition**: make plangate the sole materializer of profile and capsule; any authoring preview is explicitly non-authoritative, with DINV-2/A-2 aligned.
- **Disposition**: applied

### FND-2 — capsule integrity has no runnable review-path acceptance

- **Severity**: blocking (CAL-3)
- **Lens**: contract-quality
- **Evidence**: design: #DINV-7 forbids both claim and review process on mismatch, while A-5 tests only pre-claim refusal — vs — `prd:planner-driven-model-routing/FR-7`'s implementation-and-review boundary preservation.
- **Acceptance condition**: extend A-5 with direct and restarted review fixtures proving every capsule defect creates zero process, reviewer launch, or model invocation.
- **Disposition**: applied

## Round 2

Scope-locked convergence found both Round 1 findings closed. Section 3.2 and A-2 now leave the planner with raw assessment/evidence/raise inputs and make plangate the sole authoritative profile/capsule materializer; A-5 now proves direct and restarted review-process refusal for every capsule defect with zero reviewer or model launch. No new findings were admitted.

## Verdict history

| Round | Verdict | Date |
|---|---|---|
| 1 | request-changes | 2026-07-28 |
| 2 | approve | 2026-07-28 |

## Escalation brief

none

## Measurements

- **Rounds**: 2
- **Findings**: 2 blocking · 0 advisory
- **Dispositions**: 2 applied · 0 declined · 0 withheld-as-violation

## Overlay corrections

- **2026-07-29 — tech-design-review:** inspected the post-approve partition overlay at the `Builds-on` header, §0 item 2, §1, §3.2, and §4. Removing the plan-new predecessor dependency makes plan-new consume this shared contract instead of creating a cycle; the clarified text preserves plangate as the sole generated-profile materializer required by Round-1 FND-1. The provenance/alternative additions change no binding behavior. Verdict remains **approve**.
