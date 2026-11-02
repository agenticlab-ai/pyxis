# Review: native-routing-policy-v1 — design-review report

| Field | Value |
|---|---|
| **In one line** | Approve: native-profile qualification is reproducible and the selected alternative is correctly grounded. |
| **Relevant-when** | auditing or consuming the review of native-routing-policy-v1; asking why it was approved or bounced; tuning tech-design-review calibration |
| **Subject** | design:native-routing-policy-v1 · shape: none · supersedes: — |
| **Moment** | new-design |
| **Reviewer** | tech-design-review · 2026-07-28 guide |
| **Date(s)** | 2026-07-28 |
| **Verdict** | approve |
| **Tracking** | — *(the subject's introducing PR, once filed)* |
| **Template version** | design-review-report/v1 |

## Round 1

### FND-1 — qualification is asserted but not a reproducible contract

- **Severity**: blocking (CAL-3)
- **Lens**: contract-quality
- **Evidence**: design: §3.2/#DINV-3 admit “currently qualified” profiles without a closed qualification artifact or expiry/invalidation evaluator — vs — `prd:planner-driven-model-routing/FR-12` and ABUSE-1's reviewed evidence bar.
- **Acceptance condition**: define a closed qualification schema binding adapter/model/effort/generation, per-component proof, evaluator/version, authorization, expiry/invalidation; add missing/stale/self-declared/insufficient fixtures.
- **Disposition**: applied

### FND-2 — selected native-catalog alternative cites the wrong ALT

- **Severity**: blocking (CAL-1)
- **Lens**: coherence
- **Evidence**: design: §4 cites the selected catalogs as `design-brainstorm:planner-driven-model-routing-tech-design#ALT-7` — vs — that record's #D-3/#ALT-11; ALT-7 is the execution-demand policy alternative.
- **Acceptance condition**: cite #D-3/#ALT-11 and faithfully retain role separation, exhaustive matrix, unsupported result, and complete-catalog overrides.
- **Disposition**: applied

## Round 2

Scope-locked convergence found both Round 1 findings closed. Section 3.2 now defines the closed, generation-bound `native-profile-qualification/v1` evidence contract and A-4 exercises stale, self-declared, insufficient, invalidated, and wrong-generation failures; §4 now cites #D-3/#ALT-11 and retains the selected role-separated exhaustive policy shape. No new findings were admitted.

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

- **2026-07-29 — tech-design-review:** inspected the post-approve §1 provenance re-anchor and §4 partition alternative. Both are explanatory only: role-qualified adapter catalogs, the pure resolver, qualification, and fail-closed selection are byte-semantically unchanged. Verdict remains **approve**.
