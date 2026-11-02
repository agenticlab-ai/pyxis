# Review: wake-watch-v2 — design-review report

| Field | Value |
|---|---|
| **In one line** | Supersession review of wake-watch-v2 over wake-watch-v1 — a lineage-only supersession forced by INV-41 (incident I6): every binding statement carried forward verbatim, the Builds-on falsification genuine, no dependent stranded; approve, zero findings. |
| **Relevant-when** | auditing or consuming the review of design:wake-watch-v2; asking why the v1→v2 supersession was approved; tuning tech-design-review calibration |
| **Subject** | design:wake-watch-v2 · shape: none · supersedes: wake-watch-v1 |
| **Moment** | supersession |
| **Reviewer** | tech-design-review · guide 2026-07 (`.pyxis/authoring/tech-design-review.md`) |
| **Date(s)** | 2026-07-30 |
| **Verdict** | approve |
| **Tracking** | — *(the subject's introducing PR, once filed)* |
| **Template version** | design-review-report/v1 |

## Round 1

*Supersession-integrity scope over the successor + its declared target (target content is evidence, never the finding's object). The v1→v2 delta, verified line-by-line (20 changed lines): the title, the Status row, `Supersedes: wake-watch-v1`, the `Builds-on` re-point (implement-now → implement-now-routed-execution; state-reader-v1 → state-reader-v2), the sibling cross-references (state-reader-v1 → state-reader-v2), and one italic provenance note citing incident I6 — zero binding-element changes.*

*The integrity walk: (1) **carried forward** — every Decision callout, DINV, A-row, risk, Q-row, and Principles bullet is byte-identical to the target's; nothing dropped. (2) **The contradiction is genuine** — the target's `Builds-on: implement-now` header statement went false against the live decision graph when #811 superseded its target; INV-41 forbids the row edit, so whole-document supersession is the conventions' own named shape for exactly this ("supersede wholly, carrying forward what remains valid"). (3) **No dependent stranded** — the index shows no third design building on wake-watch-v1; the intra-pair edge (wake-watch-v2 → Builds-on → state-reader-v2) supersedes in the same change. (4) The successor's Serves set is identical to the target's, so the basis's coverage is unchanged; the re-run contract gate (plangate precheck self-drive-wake) is green over the pair. No findings.*

**Verdict: approve.**

## Verdict history

| Round | Verdict | Date |
|---|---|---|
| 1 | approve | 2026-07-30 |

## Escalation brief

none

## Measurements

- **Rounds**: 1
- **Findings**: 0 blocking · 0 advisory
- **Dispositions**: 0 applied · 0 declined · 0 withheld-as-violation

## Overlay corrections

none
