# Review: prd:implement-now/007 — review report

| Field | Value |
|---|---|
| **In one line** | Approve, round 2 — implement-now now emits one exact reciprocal publication terminal per impl-review EVT-13, with status, failure class, identity, receipt, and proof agreement enforced as one evidentiary join. |
| **Relevant-when** | auditing or consuming the review of implement-now amendment 007; implementing the EVT-13↔EVT-17 publication join; tuning prd-review calibration against cross-contract event findings |
| **Subject** | `prd:implement-now/007` |
| **Moment** | amendment |
| **Reviewer** | prd-review · `.pyxis/authoring/prd-review.md` (run inline by the authoring session, bootstrap-era) |
| **Date(s)** | 2026-08-20 |
| **Verdict** | approve |
| **Tracking** | — *(the amendment's introducing PR, once filed)* |
| **Template version** | review-report/v1 |

## Round 1

### FND-1 — Failed reciprocal terminals could disagree on failure class

- **Severity**: blocking (CAL-3)
- **Lens**: contract-quality
- **Evidence**: the first FR-49 draft required EVT-13 and EVT-17 status equality and seven identity/evidence pairs, but not `failure_class` equality; EVT-17 only required failure fields — vs — `prd:impl-review/EVT-13`, whose failed publication terminal carries the same closed `failure_class:enum{artifact_unverified,artifact_untrusted,infrastructure}` and represents the same publication moment.
- **Acceptance condition**: require a failed EVT-17 failure class to equal its paired EVT-13 value; reject every cross-class mismatch even when status and all seven reciprocal fields match; keep the obligation pending, apply FR-42's conflict disposition, and award no M7 credit.
- **Disposition**: applied — FR-49 now requires paired failed terminals to carry the same failure class and names disagreement as non-evidentiary; EVT-17 repeats the schema constraint; the fit criterion and DOD-36 require the complete cross-class mismatch matrix.

## Round 2

No new findings. The scope-locked re-review confirmed:

- FR-49 and EVT-17 require identical status and, for failures, identical failure class;
- the four identity pairs remain mandatory, the three evidence pairs remain presence-and-value identical, and verified terminals require separate native-receipt and attempt-proof fields;
- every cross-class mismatch remains pending, contributes no M7 credit, and takes FR-42's typed conflict path; and
- DOD-36 carries the status/failure-class matrix into blocking evidence.

No coherence, upstream-alignment, or completeness-critic candidate survived adversarial scrutiny. §0 was excluded as human-only.

## Verdict history

| Round | Verdict | Date |
|---|---|---|
| 1 | request-changes | 2026-08-20 |
| 2 | approve | 2026-08-20 |

## Escalation brief

none

## Measurements

- **Rounds**: 2
- **Findings**: 1 blocking · 0 advisory
- **Dispositions**: 1 applied · 0 declined · 0 withheld-as-violation

## Overlay corrections

| Correction | Found by | Date |
|---|---|---|
| The approved draft left §6.2 naming impl-review amendment 003 and reporting the host-performer and native-receipt contract absent after amendment 004 had already supplied both. Amendment 007 now records amendment 004's FR-32/FR-33/FR-35 contract as available and narrows the remaining blocker to the impl-review host-persistence successor design; FR-41's fail-closed contract-and-design gate is unchanged. | human review, PR #1099 | 2026-08-20 |
