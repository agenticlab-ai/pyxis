# Review: prd:channel-inbox/000 — review report

| Field | Value |
|---|---|
| **In one line** | approve-with-advisories — a coherent, upstream-aligned T3 draft whose closed-schema/typed-field design actively honors AP-6; two advisories (a subjective NFR-1 phrase, a missing hook-wrapper failure mode) applied in place. |
| **Relevant-when** | auditing or consuming the review of the channel-inbox PRD; asking why it was approved-with-advisories; tuning prd-review calibration against real findings |
| **Subject** | prd:channel-inbox/000 |
| **Moment** | new-base |
| **Reviewer** | prd-review · .pyxis/authoring/prd-review.md (2026-07-23) |
| **Date(s)** | 2026-07-23 |
| **Verdict** | approve-with-advisories |
| **Tracking** | — *(the introducing PR, once filed)* |
| **Template version** | review-report/v1 |

## Round 1

### FND-1 — NFR-1 statement uses subjective "not perceptibly delayed"

- **Severity**: advisory (CAL-3)
- **Lens**: contract-quality
- **Evidence**: §5.6 NFR-1 statement "so session start is not perceptibly delayed" (§5.6) — vs — prd-template Appendix A banned-vagueness list (subjective language struck on sight); the row stays advisory, not blocking, because the fit criterion (p95 ≤ 200 ms; ≤ 2 s fetch timeout) is numeric and independently verifiable, so the Must-adjacent requirement is not made unverifiable.
- **Acceptance condition**: the NFR-1 statement references the numeric bound in its own fit criterion instead of a subjective adverb.
- **Disposition**: applied — reworded to "shall complete within the wall-clock bound stated in its fit criterion so session start is not delayed beyond it."

### FND-2 — no failure row for an unresolvable `pyxis` binary at session-start-hook time

- **Severity**: advisory (CAL-4)
- **Lens**: completeness-critic
- **Evidence**: §5.2 UB-1..UB-7 and §5.1 NR-1 cover inbox *running and then failing/slow/erroring* (§5.2, §5.1), but the surface implies a distinct mode — the session-start hook fires and the `pyxis` binary is not on PATH, so inbox never runs at all — vs — `brainstorm:channel-inbox#F-8` (the operand-hooks wiring model, where a git hook fails closed when `pyxis` is absent); a *notification* hook must instead no-op so it never wounds a session (NR-1's intent, §3.3 guardrail row 1). Load-bearing for §8's session-wound safety plan, so it strengthens contract readiness rather than merely polishing prose.
- **Acceptance condition**: a failure requirement states that when `pyxis` is unresolvable at hook time the wrapper no-ops and the session starts normally.
- **Disposition**: applied — added UB-8 (Must, high risk; traces NR-1, F1, FR-11) and referenced it in the §8 flow-coverage row.

## Verdict history

| Round | Verdict | Date |
|---|---|---|
| 1 | approve-with-advisories | 2026-07-23 |

## Escalation brief

none

## Measurements

- **Rounds**: 1
- **Findings**: 0 blocking · 2 advisory
- **Dispositions**: 2 applied · 0 declined · 0 withheld-as-violation

## Overlay corrections

none
