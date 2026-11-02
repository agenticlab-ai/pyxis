# Review: implement-now-routed-execution — design-review report

| Field | Value |
|---|---|
| **In one line** | Approve: terminal ordering, activation proof, fixer ownership, codegate, and activation coverage now converge. |
| **Relevant-when** | auditing or consuming the review of implement-now-routed-execution; asking why it was approved or bounced; tuning tech-design-review calibration |
| **Subject** | design:implement-now-routed-execution · shape: none · supersedes: implement-now |
| **Moment** | supersession |
| **Reviewer** | tech-design-review · 2026-07-28 guide |
| **Date(s)** | 2026-07-28 |
| **Verdict** | approve |
| **Tracking** | — *(the subject's introducing PR, once filed)* |
| **Template version** | design-review-report/v1 |

## Round 1

### FND-1 — ReviewGate requires a terminal implementation outcome that occurs after review

- **Severity**: blocking (CAL-1)
- **Lens**: coherence
- **Evidence**: design: §3.5 passes route/outcome references into ReviewGate while §3.6 orders review before the single routed outcome — vs — `prd:implement-now/FR-25` and EVT-10.
- **Acceptance condition**: pass the canonical resolution plus exact genuinely-green/codegate proof; append the one terminal worker outcome only after review.
- **Disposition**: applied

### FND-2 — activation inputs lack one closed mainline-pinned manifest

- **Severity**: blocking (CAL-3)
- **Lens**: contract-quality
- **Evidence**: design: §3.8 reads main plus qualifications/evidence/dogfood/DOD artifacts on other refs, but #DINV-17 binds only a main object.
- **Acceptance condition**: define a closed signed activation-proof manifest pinning every external digest/ref/event/eval/DOD/reachability input and test stale/missing/unauthorized variants.
- **Disposition**: applied

### FND-3 — review-driven fixer invocations can bypass the admitted route

- **Severity**: blocking (CAL-3)
- **Lens**: contract-quality
- **Evidence**: design: §3.5 permits post-review fixing without specifying whether the original attempt remains owner — vs — `prd:implement-now/FR-25`'s route/outcome chain.
- **Acceptance condition**: define one admitted Worker attempt owning every bounded fresh sublaunch through final review, or route each fresh fixer separately; prove launch fidelity and one terminal in applied/declined/repeated fixtures.
- **Disposition**: applied

### FND-4 — the supersession omits codegate's VerificationGate seam

- **Severity**: blocking (CAL-2)
- **Lens**: upstream-alignment
- **Evidence**: design: §3.1 retains five ports and §3.5 consumes an unexplained codegate result — vs — live `codegate-v1` §3.6's implement-now `VerificationGate` and exact-revision pinned-binary adapter.
- **Acceptance condition**: re-anchor codegate-v1 and carry the VerificationGate port/adapter/exact-revision verdict into the Worker lifecycle with conformance.
- **Disposition**: applied

### FND-5 — activation ownership does not Serve impl-review's activation requirement

- **Severity**: blocking (CAL-2)
- **Lens**: upstream-alignment
- **Evidence**: design: §3.8 owns universal activation but header omits `prd:impl-review/FR-23`; sibling review design serves it while excluding activation.
- **Acceptance condition**: add FR-23 here and bind the ten-review checkpoint, parent full-seam gate, and universal activation; sibling retains review-process proof only.
- **Disposition**: applied

## Round 2

Scope-locked convergence found all five Round 1 findings closed. ReviewGate now consumes resolution plus exact green/codegate proof before the single terminal outcome; the signed activation manifest pins every external proof; one admitted attempt owns every bounded fixer sublaunch; the VerificationGate seam is preserved; and activation serves the ten-review/full-seam requirement. No new findings were admitted.

## Verdict history

| Round | Verdict | Date |
|---|---|---|
| 1 | request-changes | 2026-07-28 |
| 2 | approve | 2026-07-28 |

## Escalation brief

none

## Measurements

- **Rounds**: 2
- **Findings**: 5 blocking · 0 advisory
- **Dispositions**: 5 applied · 0 declined · 0 withheld-as-violation

## Overlay corrections

- **2026-07-29 — tech-design-review:** inspected the post-approve `Builds-on` change, §0 cardinality, §1 provenance, §3.7 ownership wording, §3.8 activation manifest/predicate, and §4 composition alternative. The executor now names the plan-new successor and the activation proof explicitly pins all **six** runtime generations (demand, plan producer, native policy, evidence, review, executor); the existing full-union/stale-input closure from Round-1 FND-2 remains intact and gains the missing producer generation. Verdict remains **approve**.
