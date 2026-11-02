# Review: state-reader-v1 — design-review report

| Field | Value |
|---|---|
| **In one line** | Round 1 request-changes on one principles-honesty gap (AP-24 load-bearing in §3.6 but absent from the Principles footer); applied same-day; round 2 approve — the port shape, claim-marking, and budget mechanics survived all four lenses clean. |
| **Relevant-when** | auditing or consuming the review of design:state-reader-v1; asking why it was approved; tuning tech-design-review calibration |
| **Subject** | design:state-reader-v1 · shape: none · supersedes: — |
| **Moment** | new-design |
| **Reviewer** | tech-design-review · guide 2026-07 (`.pyxis/authoring/tech-design-review.md`) |
| **Date(s)** | 2026-07-30 (both rounds) |
| **Verdict** | approve |
| **Tracking** | — *(the subject's introducing PR, once filed)* |
| **Template version** | design-review-report/v1 |

## Round 1

*Subject post-lint (only the pre-merge Status rows outstanding, which are the floor's ground). Context: the governing corpus (vision · principles · incidents), the basis `prd:self-drive-wake` (effective, fold `0d2d047e…`), the record `design-brainstorm:self-drive-wake-tech-design` (gated ready-for-design), and the sibling draft `wake-watch-v1` — all read in full. Four lenses; candidates killed under scrutiny: a Serves-substance challenge on UB-1 (the reader cites it for its report-unknown half — legitimate under the conventions' overlapping-Serves rule; the real gap was the sibling's omission and ships on its report); a ProposalRef under-specification challenge (single-repo identity is the executor port package's existing convention); a §3.4 alerts-none challenge (honest — alarms are contracted to the consumer and said so). One survivor.*

### FND-1 — AP-24 is load-bearing in the body but absent from the Principles footer

- **Severity**: blocking (CAL-2)
- **Lens**: upstream-alignment
- **Evidence**: design: §3.6 "a repo that wants looser green declares protection (the medium's own authority — AP-24: the governing input is the platform's configuration, not a parallel config of ours)" — vs — the template's Principles rule ("cite every AP/RW this design honors or deviates from, by ID… silent omission is not" legitimate) and the Principles section, which lists AP-1/3/6/11/15/17/19/22/26/27/28/RW-3 but not AP-24 despite it deciding the required-set-fallback call (§0 judge-row 4's substance).
- **Acceptance condition**: an AP-24 bullet in the Principles section stating how it is honored, consistent with §3.6.
- **Disposition**: applied — AP-24 bullet added ("the required-check set is read from the medium's own branch protection… no parallel required-list of ours exists to drift from it").

## Round 2

*Scope-locked to the named defect and its changed span (the Principles section). FND-1 closed: the bullet is present, agrees with §3.6, and introduces no new claim. No regression in the changed span; nothing else examined.*

**Verdict: approve.**

## Verdict history

| Round | Verdict | Date |
|---|---|---|
| 1 | request-changes | 2026-07-30 |
| 2 | approve | 2026-07-30 |

## Escalation brief

none

## Measurements

- **Rounds**: 2
- **Findings**: 1 blocking · 0 advisory
- **Dispositions**: 1 applied · 0 declined · 0 withheld-as-violation

## Overlay corrections

none
