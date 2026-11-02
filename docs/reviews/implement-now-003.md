# Review: implement-now amendment 003 — review report

| Field | Value |
|---|---|
| **In one line** | Approve after separating evidence-store outage from a healthy refusal and staging the consumer-first compatibility boundary before parent activation. |
| **Relevant-when** | auditing or consuming implement-now amendment 003; implementing append-first route admission; tuning prd-review calibration for impossible failure-row fit criteria |
| **Subject** | `prd:implement-now/003` |
| **Moment** | amendment |
| **Reviewer** | prd-review · 2026-07-28 |
| **Date(s)** | 2026-07-28 (rounds 1–2) |
| **Verdict** | approve |
| **Tracking** | — *(introducing PR not filed)* |
| **Template version** | review-report/v1 |

## Round 1

### FND-1 — UB-18's evidence-outage case had no decidable expected terminal

- **Severity**: blocking (CAL-3)
- **Lens**: contract-quality
- **Evidence**: the initial UB-18 was a Must row whose fit criterion required every mapping/qualification trigger to record one verified EVT-8 and then said an evidence-store failure remains unclaimed and non-invoked. A trigger combined with an unavailable or unverifiable evidence store cannot produce a verified canonical EVT-8, so the same fixture could not decide whether absence of EVT-8 was correct outage behavior or a fit-criterion failure — vs — NFR-9's explicit unavailable/unverifiable-evidence branch.
- **Acceptance condition**: Partition the fit criterion: with a healthy store, require exactly one append-and-exact-read EVT-8 before refusal; when append or exact-read fails, assert no verified canonical EVT-8, define the observable outage and retry behavior, and test both cases independently.
- **Disposition**: applied

### FND-2 — immediate append-first admission created a cyclic parent dependency

- **Severity**: blocking (CAL-2)
- **Lens**: upstream-alignment
- **Evidence**: the corrected consumer initially required canonical git append/exact-read immediately — vs — the merged parent `prd:planner-driven-model-routing/NFR-6`, whose existing compatibility boundary permits no outbound call before invocation. Merging the consumer or parent first would leave one frozen side contradictory until its companion landed.
- **Acceptance condition**: Preserve the existing zero-network path until the coordinated parent policy is effective; declare implement-now 003 a consumer-first compatibility prerequisite in §6.2, activate mandatory append-first behavior only with planner 003, and require direct mainline ancestry proof before the parent merges.
- **Disposition**: applied

No coherence or completeness candidate beyond FND-1/FND-2 survived adversarial screening.

## Round 2

*Scope-locked to FND-1/FND-2 and regressions within the corrected NFR-9, UB-18, and §6.2 spans.*

- **FND-1** — closed. UB-18 now partitions the pre-parent zero-network refusal path from post-parent healthy-store and admission-failure paths. After activation a healthy store must yield exactly one verified EVT-8 before refusal; append or exact-read failure asserts no verified canonical EVT-8, names the admission failure and attempted terminal, leaves the task unclaimed and retryable, and a successful retry appends exactly one immutable EVT-8. NFR-9 independently requires append-first admission for every terminal once the parent policy is effective.
- **FND-2** — closed. Before parent activation NFR-9/UB-18 retain the zero-network refusal path; afterward they require canonical admission. §6.2 declares consumer-first landing and requires mainline ancestry proof before planner 003 activates the boundary.

No regression was found within the corrected spans; the folded projection carries the same failure semantics while preserving the current parent boundary until coordinated activation.

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

none
