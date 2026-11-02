# Review: ecosystem-help-v1 — design-review report

| Field | Value |
|---|---|
| **In one line** | Approved after one revision round — one blocking upstream-alignment defect (UB-1's degraded behavior lived only in an acceptance row, not the design's mechanics) and one advisory (DINV-3 statelessness had no verification hook), both applied. |
| **Relevant-when** | auditing or consuming the review of ecosystem-help-v1; asking why it was approved; tuning tech-design-review calibration |
| **Subject** | design:ecosystem-help-v1 · shape: none · supersedes: — |
| **Moment** | new-design |
| **Reviewer** | tech-design-review · `.pyxis/authoring/tech-design-review.md` (run 2026-07-23) |
| **Date(s)** | 2026-07-23 |
| **Verdict** | approve |
| **Tracking** | agenticlab-ai/pyxis#636 — the subject's introducing PR |
| **Template version** | design-review-report/v1 |

## Round 1

### FND-1 — UB-1's degraded behavior is served but absent from the design's mechanics

- **Severity**: blocking (CAL-2)
- **Lens**: upstream-alignment
- **Evidence**: design: the header `Serves` cites `prd:ecosystem-help/UB-1` and A-6 pins its test, but §3.1's walk and §3.3's invariants never state the behavior itself — DINV-2 covers fabrication, refusal, and unbuilt stages, not the missing-surface case — vs — prd:ecosystem-help/UB-1 ("present what the remaining surfaces yield and name the absent surface — never fabricate entries for it"). A served anchor must be implemented by design content, not only asserted testable; a criterion living solely in an acceptance row is a requirement hiding from the body (the PRD template's own DoD aggregation rule, applied here).
- **Acceptance condition**: the degraded-walk behavior is stated as a binding element in §3 — either a §3.1 mechanic or a DINV extension — and A-6 traces to it.
- **Disposition**: applied — DINV-2 extended with the missing-surface clause (absence named, entries never fabricated for an absent surface); §3.1 gained the degraded-walk sentence; A-6 now traces to #DINV-2.

### FND-2 — DINV-3 (statelessness) binds implementation but nothing verifies it

- **Severity**: advisory (CAL-3)
- **Lens**: contract-quality
- **Evidence**: design: `DINV-3 — Statelessness. The skill writes nothing, caches nothing…` — vs — the acceptance table, where no A-row pins it: A-1..A-7 cover FR-1..6, UB-1, NR-1, DINV-1/2/4, leaving DINV-3 the one binding element with no verification hook (a binding element without a testable bar is intent, per the template's §6 contract).
- **Acceptance condition**: an acceptance row verifies DINV-3 by a named method (inspection of help's own files for write/cache/state instructions suffices for a prompt-only skill).
- **Disposition**: applied — A-8 added: inspection that help's files contain no write, cache, or persistence instruction, with the sandbox transcripts as corroborating evidence.

## Round 2

Scope-locked re-review of the two named defects and the changed spans (§3.1 degraded-walk sentence, DINV-2 extension, A-6 trace, new A-8). Both acceptance conditions are met; no regression within the changed spans — the DINV-2 extension stays property-testable and consistent with A-2/A-6, and A-8's method is runnable inspection with named evidence. `lint-tech-designs` green on the revision. No new findings on unchanged content (scope lock).

## Verdict history

| Round | Verdict | Date |
|---|---|---|
| 1 | request-changes | 2026-07-23 |
| 2 | approve | 2026-07-23 |

## Escalation brief

none

## Measurements

- **Rounds**: 2
- **Findings**: 1 blocking · 1 advisory
- **Dispositions**: 2 applied · 0 declined · 0 withheld-as-violation

## Overlay corrections

none
