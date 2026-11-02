# Review: feature-implementation-drive — design-review report

| Field | Value |
|---|---|
| **In one line** | approve after one scope-locked revision closed custom-ref authentication, stale-base merge enforcement, and merge-strategy-neutral delivery proof; one traceability advisory also applied |
| **Relevant-when** | auditing or consuming the review of `design:feature-implementation-drive`; asking why its ownership, final-candidate, or delivery mechanics were strengthened; tuning tech-design-review calibration |
| **Subject** | design:feature-implementation-drive · shape: none · supersedes: — |
| **Moment** | new-design |
| **Reviewer** | tech-design-review · guide 2026-07-25 |
| **Date(s)** | 2026-07-25 · 2026-07-25 round 2 |
| **Verdict** | approve |
| **Tracking** | — |
| **Template version** | design-review-report/v1 |

## Round 1

### FND-1 — custom-ref CAS did not authenticate ownership transitions

- **Severity**: blocking (CAL-4)
- **Lens**: feasibility-completeness
- **Evidence**: design: §3.4 relied on “repository policy protects the feature-drive and candidate namespaces” while R1 admitted that a configured code host may not enforce that writer boundary — vs — `prd:feature-implementation-drive/NFR-6` requires authenticated holder/transition evidence and DOD-5 requires stale-owner drills to fail closed.
- **Acceptance condition**: make transition authentication independent of optional arbitrary-ref protection, define the authenticated payload and authorization check, and make deletion/forgery fail closed with runnable abuse fixtures.
- **Disposition**: applied

### FND-2 — final-candidate eligibility had a base-advance merge race

- **Severity**: blocking (CAL-4)
- **Lens**: feasibility-completeness
- **Evidence**: design: §3.9 said an advancing main “turns [the verdict] stale” but named no medium-enforced mechanism preventing a human merge between base advance and the next wake — vs — `prd:feature-implementation-drive/FR-17`, UB-8, and the stale-verification guardrail require zero stale-authorized final proposals.
- **Acceptance condition**: bind the required verdict to exact proposal and main heads, require a code-host current-base/required-check capability at activation, define base-change refresh/backstop behavior, and add the base-advance race fixture.
- **Disposition**: applied

### FND-3 — delivery “correspondence” was undefined across merge strategies

- **Severity**: blocking (CAL-1)
- **Lens**: coherence
- **Evidence**: design: §3.9 required the landed revision to “correspond[] to the accepted feature delivery,” while §3.6 explicitly supports merge, squash, and rebase and #DINV-13 named ancestry alone — vs — `prd:feature-implementation-drive/FR-23` requires proof that the delivered feature revision, not merely some ancestor, reached main.
- **Acceptance condition**: define a merge-strategy-neutral equality predicate between the accepted candidate and landed result, bind it in #DINV-13, and add wrong-tree delivery fixtures to A-9.
- **Disposition**: applied

### FND-4 — the header omitted non-functional anchors the design substantively implements

- **Severity**: advisory (CAL-2)
- **Lens**: upstream-alignment
- **Evidence**: design: §3.2, §3.3, §3.10, §3.14, §3.15, and A-1/A-2/A-11/A-12 directly implement concurrency, injection, resumability, portability, idle-cost, identity, and audit requirements — vs — the header `Serves` ended at NR-5 and omitted `prd:feature-implementation-drive/NFR-1..NFR-8`.
- **Acceptance condition**: add every substantively implemented NFR anchor to `Serves`, explicitly enumerated.
- **Disposition**: applied

## Round 2

Scope-locked re-review inspected only §3.3–§3.4, §3.9, §3.13, §3.15–§3.16, R1/R10, A-8/A-9/A-11, and the `Serves` row. FND-1 is closed by canonical signed transition envelopes, authorization checks, loud deletion handling, and abuse fixtures. FND-2 is closed by exact-head verdict binding plus activation-time current-base/required-verdict enforcement and a base-advance race drill. FND-3 is closed by accepted-base, candidate-tree equality, and main-ancestry proof. FND-4 is closed by the explicit NFR joins. No unchanged content was re-reviewed and no new finding was introduced.

## Verdict history

| Round | Verdict | Date |
|---|---|---|
| 1 | request-changes | 2026-07-25 |
| 2 | approve | 2026-07-25 |

## Escalation brief

none

## Measurements

- **Rounds**: 2
- **Findings**: 3 blocking · 1 advisory
- **Dispositions**: 4 applied · 0 declined · 0 withheld-as-violation

## Overlay corrections

none
