# Review: implement-now amendment 001 — review report

| Field | Value |
|---|---|
| **In one line** | Approve after separating unsupported from resolved outcome chains, aligning the production checkpoint, restoring complete DoD coverage, requiring three successfully verified dogfood profiles, binding post-cutover defect disposition, and strengthening the worker eval. |
| **Relevant-when** | auditing or consuming implement-now amendment 001; asking why executor routing was approved or bounced; tuning prd-review calibration against routing-consumer findings |
| **Subject** | `prd:implement-now/001` |
| **Moment** | amendment |
| **Reviewer** | prd-review · 2026-07-27 |
| **Date(s)** | 2026-07-27 (rounds 1–2) |
| **Verdict** | approve |
| **Tracking** | — *(introducing PR not filed)* |
| **Template version** | review-report/v1 |

## Round 1

### FND-1 — unsupported attempts are both required and forbidden to have worker outcomes

- **Severity**: blocking (CAL-1)
- **Lens**: coherence
- **Evidence**: FR-25 requires “every implementation routing attempt” to record exactly one worker outcome, while its own fit criterion requires EVT-10 only “when resolved” and §5.4 says outcomes are absent for unsupported pre-claim attempts — vs — `prd:planner-driven-model-routing/FR-10` and `FR-20`, which stop unsupported routes before invocation and require outcomes only for resolved routes.
- **Acceptance condition**: Define two mutually exclusive, testable chains everywhere: unsupported/policy-invalid has one origin plus one EVT-8 and zero EVT-10; exact/promoted has one origin plus one EVT-7 and exactly one EVT-10. Reject missing, duplicate, or cross-class terminals.
- **Disposition**: applied

### FND-2 — the production checkpoint uses inconsistent metric sets and pre-requires production evidence

- **Severity**: blocking (CAL-1)
- **Lens**: coherence
- **Evidence**: §3.3's ship rule requires M1 and M3–M6 but omits M2; §6.1 repeats that set; §6.5 instead requires every metric target. M6's single target includes production while §6.5's pre-release stage permits no production routing and requires every target before FR-27 activates production.
- **Acceptance condition**: Separate pre-release from post-launch metric checkpoints; make the ship rule, milestone, and rollout table name the same pre-release set, consistently include or explicitly exclude M2, and require no production-attempt evidence before direct activation.
- **Disposition**: applied

### FND-3 — the blocking DoD omits every newly added consequence row

- **Severity**: blocking (CAL-1)
- **Lens**: coherence
- **Evidence**: DOD-2 sources only NR-1..NR-4, DOD-3 only UB-1..UB-8, and DOD-4 only ABUSE-1..ABUSE-4, while amendment 001 adds critical/high NR-5..NR-7, Must UB-9..UB-12, and critical ABUSE-5; §8 nevertheless marks every blocking DoD traced and the DoD complete.
- **Acceptance condition**: Supersede or replace the three blocking DoD rows so their sources and evidence cover NR-1..NR-7, UB-1..UB-12, and ABUSE-1..ABUSE-5 respectively, and keep §8 checked only against those live rows.
- **Disposition**: applied

### FND-4 — failed profiles can count toward the three-profile dogfood proof

- **Severity**: blocking (CAL-2)
- **Lens**: upstream-alignment
- **Evidence**: DOD-12 permits the three resolved profiles to end in “a genuinely-green or bounded-failure outcome” — vs — `prd:planner-driven-model-routing/M4` and `DOD-4`, which require at least three distinct native profiles exercised by verified dogfood tasks with verified outcomes.
- **Acceptance condition**: Require each of the three diversity-counted native profiles to join to a succeeded outcome with `verification=verified`; retain failed/blocked attempts in evidence but exclude them from the diversity threshold.
- **Disposition**: applied

### FND-5 — post-cutover mapping defects have prose rollback but no executable acceptance path

- **Severity**: blocking (CAL-4)
- **Lens**: completeness-critic
- **Evidence**: §6.5 says defective unaccepted evidence is invalidated and accepted work receives an operator disposition, but no live UB or blocking DoD verifies affected-claim stoppage, exact evidence invalidation, unaffected-work preservation, rerun, or the accepted-artifact brief — vs — `prd:planner-driven-model-routing/UB-10`, `UB-14`, and `DOD-14`, which make that post-cutover path part of the parent release contract.
- **Acceptance condition**: Add a Must post-cutover failure row and blocking evidence criterion covering strongest-profile fallback, stopped affected claims, `rerun_required` invalidation for affected in-flight/unaccepted artifacts, continued unaffected work, exact-revision replacement evidence, and an operator remediation brief without destructive rollback for already accepted artifacts.
- **Disposition**: applied

### FND-6 — the worker eval can pass with zero successful implementations

- **Severity**: advisory (CAL-3)
- **Lens**: contract-quality
- **Evidence**: EVAL-1 permits every positive task to finish “green or a bounded escalation,” with no run minimum or success threshold — vs — Product Vision stage 8's implementation-and-proposal output and AP-14's observed-behavior acceptance posture.
- **Acceptance condition**: Define a versioned positive/adversarial corpus, a positive numeric run minimum per supported harness, and a required green threshold for positive cases; accept bounded escalation only for designated failure/adversarial cases and record required/executed counts plus outcomes.
- **Disposition**: applied

## Round 2

No further findings survived the scope-locked re-review. FR-25 now defines mutually exclusive resolved-with-outcome and pre-claim-failure-without-outcome chains; M1–M6 have one pre-release checkpoint and M4–M6 continue post-launch; DOD-15..DOD-17 cover every live NR, UB, and abuse row; M5/DOD-12 count only three distinct profiles with succeeded verified outcomes; UB-13, EVT-11, and DOD-18 make post-cutover invalidation, exact-revision replacement, unaffected-work preservation, and accepted-artifact remediation executable; and EVAL-3 requires five decode-pinned positive runs per harness with a 100% green positive threshold. No regression was introduced within the correction spans.

## Verdict history

| Round | Verdict | Date |
|---|---|---|
| 1 | request-changes | 2026-07-27 |
| 2 | approve | 2026-07-27 |

## Escalation brief

none

## Measurements

- **Rounds**: 2
- **Findings**: 5 blocking · 1 advisory
- **Dispositions**: 6 applied · 0 declined · 0 withheld-as-violation

## Overlay corrections

none
