# Review: impl-review-routed-assurance — design-review report

| Field | Value |
|---|---|
| **In one line** | Approve: pass admission, retry generations, remediation, and the reusable reviewer-program seam now converge. |
| **Relevant-when** | auditing or consuming the review of impl-review-routed-assurance; asking why it was approved or bounced; tuning tech-design-review calibration |
| **Subject** | design:impl-review-routed-assurance · shape: none · supersedes: impl-review-v1 |
| **Moment** | supersession |
| **Reviewer** | tech-design-review · 2026-07-28 guide |
| **Date(s)** | 2026-07-28 |
| **Verdict** | approve |
| **Tracking** | — *(the subject's introducing PR, once filed)* |
| **Template version** | design-review-report/v1 |

## Round 1

### FND-1 — partial pass-set admission strands resolved attempts

- **Severity**: blocking (CAL-3)
- **Lens**: contract-quality
- **Evidence**: design: §3.3 launches none when any admission fails but does not terminal already admitted routes — vs — routing-evidence #DINV-7.
- **Acceptance condition**: append compensating blocked outcomes for every earlier resolved pass and test every failure position.
- **Disposition**: applied

### FND-2 — a completed failed process cannot rerun under stable process identity

- **Severity**: blocking (CAL-1)
- **Lens**: coherence
- **Evidence**: design: §3.2 hashes semantic inputs into process ID and §3.12 reruns the same revision after terminal failure — vs — routing-evidence's one terminal per process ID and `prd:impl-review/UB-9` recovery.
- **Acceptance condition**: add a stable review-subject identity plus CAS-allocated process generation; resume an in-flight generation, but mint/link a fresh generation after a terminal.
- **Disposition**: applied

### FND-3 — remediation mixes revisions unless made non-certifying

- **Severity**: blocking (CAL-3)
- **Lens**: contract-quality
- **Evidence**: design: §3.6 puts a post-fix new revision in an old pass lineage while §3.7 stales the entire process — vs — `prd:impl-review/FR-18`, FR-20, and FR-26.
- **Acceptance condition**: make remediation feedback/disposition-only; any code change starts a new full process, and fixtures prove it never certifies or counts as corroboration.
- **Disposition**: applied

### FND-4 — supersession strands the reusable reviewer-program seam

- **Severity**: blocking (CAL-2)
- **Lens**: upstream-alignment
- **Evidence**: design: §3.5 hardens one four-lens pass but does not preserve the generic reviewer-program seam — vs — target `impl-review-v1` §3.4 and live dependent `e2e-verification-skills` §3.2's two-lens instance.
- **Acceptance condition**: preserve a generic closed reviewer-program definition with per-instance lens set/convergence and conformance for both four-lens impl-review and two-lens e2e review.
- **Disposition**: applied

## Round 2

Scope-locked convergence found all four Round 1 findings closed. Partial admission now terminals every prior resolution without launching; stable review subjects allocate linked process generations after a terminal; remediation is non-certifying and every changed revision starts a complete new process; and `reviewer-program/v1` preserves both the four-lens impl-review and two-lens e2e instances. No new findings were admitted.

## Verdict history

| Round | Verdict | Date |
|---|---|---|
| 1 | request-changes | 2026-07-28 |
| 2 | approve | 2026-07-28 |

## Escalation brief

none

## Measurements

- **Rounds**: 2
- **Findings**: 4 blocking · 0 advisory
- **Dispositions**: 4 applied · 0 declined · 0 withheld-as-violation

## Overlay corrections

- **2026-07-29 — tech-design-review:** inspected the post-approve §1 partition re-anchor and §4 rejection of folding review into implement-now. Both explain the already-reviewed boundary; pass identity, shared-base/two-blind-pass process, aggregation, invalidation, and universal review floor are unchanged. Verdict remains **approve**.
