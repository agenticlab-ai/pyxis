# Review: inherited-learning-repair PRD — review report

| Field | Value |
|---|---|
| **In one line** | Approve with one advisory to state the recurring scan and wait verification cost. |
| **Relevant-when** | Auditing the incident-scoped shared Learning repair contract or its recurring execution cost. |
| **Subject** | prd:inherited-learning-repair/draft |
| **Moment** | draft |
| **Reviewer** | prd-review · `.pyxis/authoring/prd-review.md` at 23be656079001ac50540d7140df53d135e2e710a |
| **Date(s)** | 2026-10-05 |
| **Verdict** | approve-with-advisories |
| **Tracking** | Draft inherited-learning-repair; introducing proposal not yet published |
| **Template version** | review-report/v1 |

**Review subject:** prd
**Draft digest:** sha256:40fd5527131b959e6299f0f070d62d2cde823a5522a63a6fde34da1a5927d4d9

## Round 1

Reviewed the committed PRD body and sibling tech spec at `23be656079001ac50540d7140df53d135e2e710a`, with problem-ledger A-1 through A-5 and delivery-ledger E-3 as the decision basis. The four template-header corrections since the initial substance read were verified separately; requirement text and the tech spec were unchanged. Human-only §0 and the mechanical gate floor are excluded from judgment.

Contract-quality, coherence, upstream-alignment, and completeness-critic received distinct lens passes under the drive's sequential fallback. A separate fresh-context reviewer independently screened the candidate before it was retained. The review preserves the operator's boundary: #1437's inherited frozen Learning witness mismatch, with crash recovery, other failure classes, and semantic triage deferred.

Goals and jobs map to requirements and metrics; FR-1 through FR-10 and the stated negative boundaries have blocking DoD coverage. DOD-6 requires a retained worker trial, so hermetic coordination evidence alone cannot establish completion. The fetched-target and successor requirements preserve the distinction between a published proposal, landed correction, and caller adoption. No blocking finding survived review.

### FND-1 — State the recurring scan and wait work

- **Severity**: advisory (CAL-4, `.pyxis/skills/prd-review/calibration.md`)
- **Lens**: completeness-critic
- **Evidence**: PRD FR-9 says “When a bounded scan runs”; §5.5 makes it callable by an existing daily trigger. The sibling tech spec D-3 supplies a five-second wait cadence and D-2 says wait checks fetch and pin again, but neither states the steady-state verification work per scan or wait poll. AP-19 requires “Every recurring path states its steady-state cost.” Zero model calls before dispatch (NFR-1) does not describe the deterministic work. This omission does not make a Must row unverifiable or invalidate §8, so CAL-4 remains advisory.
- **Acceptance condition**: State the scan and wait work and frequency, where target verification runs or is reused if applicable, and the existing executor bound that controls admitted work. No performance target, cache, numeric spending allowance, scheduler, or maximum wait duration is required.
- **Disposition**: applied — Argus, authority-contract/v1 tier-2, added the technical cost explanation in tech-spec D-5, Steady-state work, in the next authoring round. This report retains its original reviewed digest; the revised contract requires a separate scope-locked report.

## Verdict history

| Round | Verdict | Date |
|---|---|---|
| 1 | approve-with-advisories | 2026-10-05 |

## Escalation brief

none

## Measurements

- **Rounds**: 1
- **Findings**: 0 blocking · 1 advisory
- **Dispositions**: 1 applied · 0 declined · 0 withheld-as-violation

## Overlay corrections

none
