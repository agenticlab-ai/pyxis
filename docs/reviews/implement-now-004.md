# Review: implement-now amendment 004 — review report

| Field | Value |
|---|---|
| **In one line** | Approve — the verified review terminal now has coherent live-event metrics, release gates, ownership boundaries, and readiness joins. |
| **Relevant-when** | auditing the review of implement-now amendment 004; asking why its contract joins were revised before approval |
| **Subject** | `prd:implement-now/004` |
| **Moment** | amendment |
| **Reviewer** | prd-review · 2026-07-10 guide |
| **Date(s)** | 2026-08-06 (rounds 1–2) |
| **Verdict** | approve |
| **Tracking** | PR #1034 |
| **Template version** | review-report/v1 |

## Round 1

### FND-1 — Review telemetry is not joined into the live metric plan

- **Severity**: blocking (CAL-1)
- **Lens**: coherence
- **Evidence**: FR-33 and replacement events EVT-13/EVT-14 make the review terminal run-significant, while §3.2 still feeds M1/M2 from superseded EVT-10/EVT-6 and M5/M6 from EVT-10 — vs — the coherence lens requires every metric to be fed by a stated live plan, and §8's instrumentation row claims that join is complete.
- **Acceptance condition**: replace §3.2 with the complete live metric table, pointing task-outcome consumers to EVT-13 and run-level consumers to EVT-14 and naming the verified/failed review denominator.
- **Disposition**: applied — amendment 004 now supersedes §3.2 with live event joins and an explicit configured-review denominator.

### FND-2 — The pre-release gate can pass while DOD-21 is red

- **Severity**: blocking (CAL-1)
- **Lens**: coherence
- **Evidence**: DOD-21 is blocking, but the §3.3 ship rule, §6.1 pre-release exit, and §6.5 pre-release evidence stop at DOD-20; §6.2 also says full review activation is not owned by implement-now while FR-33 assigns this executor the launch and terminal obligation — vs — the coherence lens requires every blocking DoD row to participate in release and every dependency statement to agree with the live contract.
- **Acceptance condition**: replace the affected ship-rule, phase, dependency, release-criterion, and rollout spans so DOD-21 and the configured review terminal are mandatory pre-release evidence and the impl-review dependency describes the actual ownership boundary.
- **Disposition**: applied — amendment 004 now supersedes §3.3 and §6.1/§6.2/§6.4/§6.5 with one coherent pre-release gate.

### FND-3 — Contract readiness asserts completeness without FR-33 or DOD-21

- **Severity**: blocking (CAL-4)
- **Lens**: completeness-critic
- **Evidence**: §8 says every live Must FR, key flow, metric, and blocking DoD is covered but enumerates only FR-1..FR-32, EVT-4/EVT-6..EVT-10, and DOD-5/DOD-8..DOD-20 — vs — CAL-4 makes an omitted load-bearing §8 join blocking, and amendment 004 adds critical Must FR-33 plus blocking DOD-21.
- **Acceptance condition**: replace §8 in full so its coverage, telemetry, DoD, risk, and completeness claims explicitly include FR-33, EVT-13/EVT-14, and DOD-21.
- **Disposition**: applied — amendment 004 now supplies a complete replacement §8 with the new live anchors.

## Round 2

No new findings. Scope-locked re-review confirmed the revised §3.2/§3.3, §6.1/§6.2/§6.4/§6.5, and §8 spans close FND-1..FND-3: M7 has a complete EVT-13/EVT-14 denominator, DOD-21 participates in every pre-release gate, the impl-review/implement-now ownership split is explicit, and readiness names every new live anchor.

## Verdict history

| Round | Verdict | Date |
|---|---|---|
| 1 | request-changes | 2026-08-06 |
| 2 | approve | 2026-08-06 |

## Escalation brief

none

## Measurements

- **Rounds**: 2
- **Findings**: 3 blocking · 0 advisory
- **Dispositions**: 3 applied · 0 declined · 0 withheld-as-violation

## Overlay corrections

| Correction | Found by | Date |
|---|---|---|
| The approved round's EVT-13 replacement overloaded `routed_task_outcome` and left superseded event rows live. Amendment 004 now retains EVT-10 as the sole immutable implementation outcome, adds EVT-13 as the separate `impl_review_terminal`, replaces EVT-6 with EVT-14, and supersedes §5.7 wholesale so the live event table contains no duplicate name or superseded row. The frozen Round 1/2 wording above records the pre-overlay disposition and is not retroactively edited. | human review, PR #1041 | 2026-08-06 |
| FR-25, DOD-12, §4.4, §5.4, and §5.5 legitimately continue to cite EVT-10 because implementation routing and dogfood still join to the immutable worker outcome; review lifecycle consumers join EVT-13/EVT-14 instead. The amendment replaces each affected aggregate section so this distinction is explicit. | human review, PR #1041 | 2026-08-06 |
| `Learned-from` now uses stable repository review-thread identities for PR #1034, PR #1041, and issue #1029 rather than the force-pushed local commit identity or the inaccurate support-ticket type. | human review, PR #1041 | 2026-08-06 |
| Review failure is now durable drive state rather than a one-invocation exit-code side effect: FR-33..FR-39, NR-9, UB-19, EVT-13/EVT-14, and split DOD-21..DOD-26 define a proposal-revision obligation, exact-read-first review-only reconciliation, bounded driver retries, closed `artifact_unverified`/`infrastructure` failure classes, pending counts, and an explicit self-drive/feature-drive consumer dependency. | human review, PR #1041 | 2026-08-06 |
