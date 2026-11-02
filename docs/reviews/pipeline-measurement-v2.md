# Review: pipeline-measurement-v2 — design-review report

| Field | Value |
|---|---|
| **In one line** | approve — a faithful Serves-completeness supersession of `pipeline-measurement`: mechanism carried forward verbatim, the five Must `UB-1..5` added to `Serves` with acceptance, contract gate green. |
| **Relevant-when** | auditing or consuming the review of design:pipeline-measurement-v2; asking why the supersession was approved; tuning tech-design-review calibration. |
| **Subject** | design:pipeline-measurement-v2 · shape: none · supersedes: pipeline-measurement |
| **Moment** | supersession |
| **Reviewer** | tech-design-review · 2026-07-21 |
| **Date(s)** | 2026-07-21 |
| **Verdict** | approve |
| **Tracking** | — *(the subject's introducing PR, once filed)* |
| **Template version** | design-review-report/v1 |

## Round 1

*No findings survived. The supersession-integrity checks (the load-bearing lens for a supersession):*

- **Content carried forward** — the mechanism (§2 Decision, §3.1–3.13, DINV-1..9, §4 alternatives, Principles) is carried forward **verbatim** from `design:pipeline-measurement`; the only substantive change is the `Serves` header and two added acceptance rows.
- **The correction is real and complete** — `Serves` now enumerates `UB-1..UB-5` (the five Must rows the predecessor handled in substance but omitted from the header — the incident-I4 defect), and A-12/A-13 add the two missing acceptance rows (UB-3 incomplete-phase, UB-4 harness-switch) so all five UBs are served *with verification*, not a hollow header add.
- **The supersession is the genuine mechanism** — the predecessor is frozen at merge; a completeness fix to a frozen design can only be a whole-document supersession (not `Builds-on`), and two live designs of the same mechanism cannot coexist. `Supersedes: pipeline-measurement` is declared; the index computes `pipeline-measurement → superseded`.
- **No live dependent stranded** — the predecessor had no live dependents; the feeding design-brainstorm record's `Feeds` pointed at the (now-superseded) predecessor, which is historically accurate and unaffected.
- **The decisive downstream check passes** — `plangate precheck pipeline-measurement` (the stage-6 contract gate) is now **clean**: every live Must, including UB-1..5, is served by the live v2.

## Verdict history

| Round | Verdict | Date |
|---|---|---|
| 1 | approve | 2026-07-21 |

## Escalation brief

none

## Measurements

- **Rounds**: 1
- **Findings**: 0 blocking · 0 advisory
- **Dispositions**: 0 applied · 0 declined · 0 withheld-as-violation

## Overlay corrections

none
