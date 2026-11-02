# Review: contract-gate-v2 — design-review report

| Field | Value |
|---|---|
| **In one line** | Approve after round 2. The supersession is genuine (v1's planning-entry rule refuses a draft-born feature), but round 1 required the successor to state what it changed besides that rule: three superseded requirement rows dropped from Serves, and a resolved open question. |
| **Relevant-when** | auditing or consuming the review of contract-gate-v2 or the supersession of contract-gate-v1; asking why it was approved; tuning tech-design-review calibration |
| **Subject** | design:contract-gate-v2 · shape: none · supersedes: contract-gate-v1 |
| **Moment** | supersession |
| **Reviewer** | tech-design-review · `.pyxis/authoring/tech-design-review.md` as of 2026-09-23, run in the authoring session |
| **Date(s)** | 2026-09-23 (round 1) · 2026-09-23 (round 2) |
| **Verdict** | approve |
| **Tracking** | — *(the feature-draft introducing PR, once filed)* |
| **Template version** | design-review-report/v1 |

## Round 1

Supersession integrity checked: the contradiction is genuine, since `contract-gate-v1` §3.10 binds plan-new to call the gate before any plan, and a feature with no PRD ledger yields `configuration-error`, so `Builds-on` could not hold. The designs that build on the target (`memory-staged-contracts`, `plan-new-routed-demand`) keep their edges. Neither serves a feature newly blocked by this change; the latter already carried a stale lineage flag.

### FND-1 — The successor claims an unchanged carry-forward while changing Serves and an open question

- **Severity**: blocking (CAL-2)
- **Lens**: upstream-alignment
- **Evidence**: design: "Everything else in `contract-gate-v1` stays true and is carried forward unchanged" (§1) — vs — target anchors: `contract-gate-v1` header Serves, which lists `prd:contract-gate/FR-2`, `FR-9`, and `FR-10` (dropped from the successor's Serves because contract-gate amendment 001 superseded them); `contract-gate-v1` §7 Q3, open in the target and resolved in the successor; and `contract-gate-v1` A-2, whose trace `prd:contract-gate/FR-2..FR-6` includes a superseded row
- **Acceptance condition**: §1 states each change beyond the scoped rule, with its reason, and A-2 stops tracing a superseded row as live
- **Disposition**: applied — §1 names the dropped rows and their successors (not designed here) and Q3's graduation to AP-28; A-2 traces `FR-3..FR-6`, noting the currency check as built for the superseded `FR-2`

## Round 2

Scope-locked to the spans changed for FND-1: §1's successor paragraph and A-2 now state the dropped rows, their successors, Q3's resolution, and the currency check's provenance. FND-1 closed; no new finding.

## Verdict history

| Round | Verdict | Date |
|---|---|---|
| 1 | request-changes | 2026-09-23 |
| 2 | approve | 2026-09-23 |

## Escalation brief

none

## Measurements

- **Rounds**: 2
- **Findings**: 1 blocking · 0 advisory
- **Dispositions**: 1 applied · 0 declined · 0 withheld-as-violation

## Overlay corrections

- 2026-09-23 · operator (plan-new dry run, after approval): the planning precheck found the design building on the superseded `docgate-v1` and `plan-new-v1`; Builds-on now names their live successors, `prd-amendment-model-v2` and `plan-new-routed-demand`.
