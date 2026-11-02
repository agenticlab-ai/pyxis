# Review: queue-widget/000 — review report

| Field | Value |
|---|---|
| **In one line** | Approved with one open advisory after two rounds — four blocking substance defects (vague Must criterion, unmeasured goal, uncovered Must, ignored incident class) found and closed. |
| **Relevant-when** | auditing the queue-widget-000 review (fixture); tuning prd-review calibration against real findings |
| **Subject** | prd:queue-widget/000 *(fixture subject)* |
| **Moment** | new-base |
| **Reviewer** | prd-review · guide 2026-07-10 |
| **Date(s)** | 2026-07-10 |
| **Verdict** | approve-with-advisories |
| **Tracking** | — *(fixture)* |
| **Template version** | review-report/v1 |

## Round 1

### FND-1 — a Must row's fit criterion cannot fail

- **Severity**: blocking (CAL-3 — the defect makes a Must row unverifiable)
- **Lens**: contract-quality
- **Evidence**: FR-2 fit criterion "the widget is fast" — vs — prd-template Appendix A's banned-vagueness list (subjective language; comparatives without numbers)
- **Acceptance condition**: FR-2 carries a percentile bound at a stated load (e.g. p95 render ≤ n ms at the capacity corpus)
- **Disposition**: applied

### FND-2 — a goal nothing measures

- **Severity**: blocking (CAL-1)
- **Lens**: coherence
- **Evidence**: G2 ("the widget feels current") names no metric and no qual method — vs — the goal→signal→metric derivation rule (template §3.2)
- **Acceptance condition**: G2 gains a qual method (operator verdicts at a named checkpoint) or is dropped with reason
- **Disposition**: applied

### FND-3 — a Must requirement without a blocking DoD row

- **Severity**: blocking (CAL-1)
- **Lens**: coherence
- **Evidence**: FR-3 (Must) absent from §5.8; DOD-1 covers FR-1/FR-2 only — vs — the every-Must-covered chain (template §5.8/§8)
- **Acceptance condition**: a blocking DoD row sources FR-3 with its evidence class
- **Disposition**: applied

### FND-4 — the staleness decision ignores a registered incident class

- **Severity**: blocking (CAL-2)
- **Lens**: upstream-alignment
- **Evidence**: §6.3 documents feed lag only, while FR-3's staleness mark is decided from a cached index — vs — the stale-read incident class (AP-15 lineage: destructive/dispatch decisions on cached reads)
- **Acceptance condition**: §6.3 gains the stale-read risk with a mitigation (exact read on the staleness decision path, or an explicit display-only rationale)
- **Disposition**: applied

### FND-5 — the surface's second failure path is unwritten

- **Severity**: advisory (CAL-4 — not §8-load-bearing for this surface)
- **Lens**: completeness-critic
- **Evidence**: §5.2 covers malformed data only; a feed racing a queue mutation mid-read has no row — vs — the failure-space enumeration rule (template §5.2)
- **Acceptance condition**: a UB row for the mid-read race, or a reasoned skip
- **Disposition**: open (advisory)

## Round 2

none — closures of FND-1..FND-4 confirmed against their stated acceptance conditions; no regressions in the changed spans; FND-5 remains open by the author's choice (advisory).

## Verdict history

| Round | Verdict | Date |
|---|---|---|
| 1 | request-changes | 2026-07-10 |
| 2 | approve-with-advisories | 2026-07-10 |

## Escalation brief

none

## Measurements

- **Rounds**: 2
- **Findings**: 4 blocking · 1 advisory
- **Dispositions**: 4 applied · 0 declined · 0 withheld-as-violation · 1 advisory open

## Overlay corrections

none
