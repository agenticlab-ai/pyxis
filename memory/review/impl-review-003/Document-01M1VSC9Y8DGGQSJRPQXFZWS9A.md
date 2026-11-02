# Review: impl-review amendment 003 — review report

| Field | Value |
|---|---|
| **In one line** | Approve after making the consumer transition backward-compatible, validating the complete frozen review demand, and binding append-first admission directly into production activation. |
| **Relevant-when** | auditing or consuming impl-review amendment 003; landing the consumer-before-parent compatibility sequence; implementing review-route evidence admission |
| **Subject** | `prd:impl-review/003` |
| **Moment** | amendment |
| **Reviewer** | prd-review · 2026-07-28 |
| **Date(s)** | 2026-07-28 (rounds 1–2) |
| **Verdict** | approve |
| **Tracking** | — *(introducing PR not filed)* |
| **Template version** | review-report/v1 |

## Round 1

### FND-1 — review-demand integrity omitted most route-defining input

- **Severity**: blocking (CAL-3)
- **Lens**: contract-quality
- **Evidence**: initial NFR-12 closed and tested planned assurance, actual band, join policy, review-role mappings, qualifications, and overrides but omitted frozen capability, deliberation, context, demand identity/version, and the selected review harness. UB-14 likewise refused only assurance/band defects, while DOD-14 transported only planned assurance and harness — vs — `prd:planner-driven-model-routing/FR-7`, which requires field-identical preservation of the complete derived profile and evidence through the review boundary.
- **Acceptance condition**: Close NFR-12 over the complete frozen semantic demand and identity/version plus selected harness/role and routing-policy inputs; extend UB-14 negative fixtures to every field and harness mismatch; make DOD-14 prove field-identical complete-demand transport before FR-25 resolution.
- **Disposition**: applied

### FND-2 — append-before-use was not a conjunct of the single activation predicate

- **Severity**: blocking (CAL-1)
- **Lens**: coherence
- **Evidence**: initial NFR-12 required every routing terminal to be appended and exact-read before review invocation or process-result use, and §6.4 named boundary traces — vs — §3.3, whose activation predicate was composed from live blocking DoDs, §6.6, integration seams, and the parent gate. DOD-13 proved eventual round-trip only and DOD-10 excluded NFRs, so production could activate with evidence appended after invocation.
- **Acceptance condition**: Make the complete NFR-12 boundary proof a direct conjunct of the single activation predicate through a live blocking DoD, covering ordering, exclusive network use, zero-evidence paths, and evidence-store outage; cite that DoD in §8.
- **Disposition**: applied

### FND-3 — the consumer amendment introduced a cyclic parent dependency

- **Severity**: blocking (CAL-2)
- **Lens**: upstream-alignment
- **Evidence**: initial NFR-12 immediately required canonical-git append/exact-read before review invocation — vs — the currently merged parent `prd:planner-driven-model-routing/NFR-6`, which permits no outbound call before invocation. Parent amendment 003 was absent from the consumer branch's ancestry, so either merge order would temporarily contradict one frozen contract; the learned-from thread was provenance, not a normative dependency or landing constraint. This also repeated `incident:I2`'s ancestry-risk class.
- **Acceptance condition**: Make impl-review 003 a dormant compatibility prerequisite that preserves the parent-through-002 zero-network boundary, activates append-first only when planner 003 becomes effective, declares consumer-first landing in §6.2, and requires parent ancestry proof before parent merge.
- **Disposition**: applied

No completeness-critic candidate beyond these three defects survived adversarial screening.

## Round 2

*Scope-locked to FND-1..FND-3 and regressions within the corrected NFR-12, UB-14, DOD-13/DOD-14, §3.3, §6.2, and §8 spans.*

- **FND-1** — closed. NFR-12 now covers the complete frozen semantic demand, identities and versions, actual band/join, selected harness/role, effective policy, qualifications, and override. UB-14 refuses every missing/changed/unknown/native/version/harness-mismatch class before invocation, and DOD-14 requires field-identical complete-demand transport.
- **FND-2** — closed. DOD-13 now proves pre-parent zero-network compatibility, post-parent append/exact-read ordering and exclusivity, zero-evidence behavior, and evidence-store outage; §3.3 requires DOD-13 explicitly and §8 accounts for it.
- **FND-3** — closed. NFR-12 remains zero-network while dormant and activates append-first only with planner 003. Both PRDs' §6.2 sections declare consumer-first landing and require direct mainline ancestry proof before the parent merges.

No regression survived the scope-locked re-review. The folded projection preserves selected-harness ownership, the universal review floor, the exact-revision aggregate, and the existing no-network behavior until coordinated activation.

## Verdict history

| Round | Verdict | Date |
|---|---|---|
| 1 | request-changes | 2026-07-28 |
| 2 | approve | 2026-07-28 |

## Escalation brief

none

## Measurements

- **Rounds**: 2
- **Findings**: 3 blocking · 0 advisory
- **Dispositions**: 3 applied · 0 declined · 0 withheld-as-violation

## Overlay corrections

none
