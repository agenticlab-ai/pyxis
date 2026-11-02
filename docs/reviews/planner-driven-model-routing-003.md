# Review: planner-driven-model-routing amendment 003 — review report

| Field | Value |
|---|---|
| **In one line** | Approve after making append-first route admission mandatory and coordinating the same pre-invocation boundary with the merged impl-review consumer contract. |
| **Relevant-when** | auditing or consuming planner-driven-model-routing amendment 003; designing the offline-resolution to append-first-admission boundary; tuning prd-review calibration for ordering defects |
| **Subject** | `prd:planner-driven-model-routing/003` |
| **Moment** | amendment |
| **Reviewer** | prd-review · 2026-07-28 |
| **Date(s)** | 2026-07-28 (rounds 1–2) |
| **Verdict** | approve |
| **Tracking** | — *(introducing PR not filed)* |
| **Template version** | review-report/v1 |

## Round 1

### FND-1 — append-first admission remained optional on the healthy path

- **Severity**: advisory (CAL-3)
- **Lens**: contract-quality
- **Evidence**: the initial NFR-8 said the runtime “may perform only” canonical append/exact-read after resolution and required traces merely to “contain only” those operations, so a healthy route could perform neither operation and still satisfy the literal allowed-operation condition — vs — the amendment's learned-from operator decision and operation reason, which require canonical git evidence before launch; live FR-21 required eventual git retention but did not supply that ordering.
- **Acceptance condition**: Require every resolved or failed routing terminal to be durably appended and exact-read successfully before task claim or reviewer/worker invocation, and make a healthy route with zero evidence operations fail the boundary suite.
- **Disposition**: applied

### FND-2 — the review consumer still prohibited the parent contract's sole pre-invocation evidence operation

- **Severity**: blocking (CAL-2)
- **Lens**: upstream-alignment
- **Evidence**: corrected NFR-8 requires canonical git append/exact-read before a selected reviewer invocation — vs — merged `prd:impl-review/NFR-9`, which still required “no network call before the first selected review model invocation.” Landing only the parent amendment would make its review path impossible to satisfy and its declared blast radius omitted the sibling contract.
- **Acceptance condition**: Add a coordinated impl-review amendment that preserves offline deterministic resolution but admits and requires only canonical git append/exact-read before review invocation; declare the sibling dependency and require the consumer amendment to land before this parent amendment.
- **Disposition**: applied

No coherence or completeness candidate beyond FND-1/FND-2 survived adversarial screening.

## Round 2

*Scope-locked to FND-1/FND-2 and regressions within the corrected NFR-8 and dependency spans plus the coordinated consumer replacement.*

- **FND-1** — closed. NFR-8 now requires every terminal to be durably appended and successfully exact-read before claim or worker/reviewer invocation. Its fit criterion fails a zero-evidence healthy path and separately proves unavailable or unverifiable evidence yields zero claim and zero invocation.
- **FND-2** — closed. `impl-review` amendment 003 lands first as a dormant compatibility prerequisite that preserves the current zero-network boundary, then switches to offline-resolution plus canonical append/exact-read only when planner 003 becomes effective. Planner amendment 003's §6.2 requires both consumer amendments and their direct mainline ancestry before this parent amendment lands.

No regression was found within the corrected spans; the folded projections preserve pure resolution, fail-closed evidence admission, selected-harness ownership, and the existing review floor.

## Verdict history

| Round | Verdict | Date |
|---|---|---|
| 1 | request-changes | 2026-07-28 |
| 2 | approve | 2026-07-28 |

## Escalation brief

none

## Measurements

- **Rounds**: 2
- **Findings**: 1 blocking · 1 advisory
- **Dispositions**: 2 applied · 0 declined · 0 withheld-as-violation

## Overlay corrections

none
