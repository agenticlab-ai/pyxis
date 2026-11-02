# Review: prd:pipeline-measurement/000 — review report

| Field | Value |
|---|---|
| **In one line** | approve after one round of changes — a well-formed T2 contract; one blocking coherence fix (a goal citing the wrong metric) and three advisories (vague overhead adverb, a primary-metric tolerance wholly deferred, and unaddressed nested skill invocations), all applied. |
| **Relevant-when** | auditing or consuming the review of prd:pipeline-measurement/000; asking why it was approved; tuning prd-review calibration against real findings. |
| **Subject** | prd:pipeline-measurement/000 |
| **Moment** | new-base |
| **Reviewer** | prd-review · 2026-07-20 |
| **Date(s)** | 2026-07-20 |
| **Verdict** | approve |
| **Tracking** | — *(the subject's introducing PR, once filed)* |
| **Template version** | review-report/v1 |

## Round 1

### FND-1 — overhead claimed "near-zero" where the bar is exactly zero

- **Severity**: advisory (CAL-3)
- **Lens**: contract-quality
- **Evidence**: header In-one-line + §4.2 + NFR-1 + FR-10 rationale "near-zero … overhead" — vs — FR-10 fit + §3.3 guardrail "zero model tokens" and Appendix A banned-vagueness (ambiguous adverbs). The measured bar is exactly zero model tokens; "near-zero" is a vague adverb the template strikes on sight.
- **Acceptance condition**: the prose states "zero model-token overhead" wherever it means the model-token bar, matching the fit criterion.
- **Disposition**: applied — In-one-line, §4.2, NFR-1, FR-10 rationale, and §7.1 FAQ now read "zero model-token(s)".

### FND-2 — the primary metric's threshold is wholly deferred to an open question

- **Severity**: advisory (CAL-3)
- **Lens**: contract-quality
- **Evidence**: FR-1 fit + M1 target "reconciles … within the Q2 tolerance / within tolerance" — vs — §7.2 Q2 (the tolerance is open) and template rule 10 (zero solution-blocking TBDs). A primary metric whose pass/fail threshold is entirely TBD reads as an unowned gap, not a design knob.
- **Acceptance condition**: M1 and FR-1 state a default bound — exact reconciliation where the harness reports an authoritative per-model total, a looser tolerance being the Q2 design knob — so the threshold is not wholly deferred.
- **Disposition**: applied — FR-1 fit and M1 target now carry the exact-where-authoritative default with Q2 as the residual knob.

### FND-3 — the phase boundary does not address nested skill invocations

- **Severity**: advisory (CAL-4)
- **Lens**: completeness-critic
- **Evidence**: FR-3 + §4.1 boundary "skill-invocation → artifact-write" — vs — the observed reality that a phase can invoke a sub-skill (this PRD was produced by prd-new invoking prd-review as a nested skill); §5.2 and the parked P-2 leave the nested case unspecified, so a builder would guess the attribution rule (a §8 "verify without guessing" risk).
- **Acceptance condition**: the PRD acknowledges nested invocation — the nested phase's spend is recorded distinctly, never silently blended into the parent — and routes the exact rule to design.
- **Disposition**: applied — UB-5 added (record nested distinctly, never blend) and Q4 added (the nested-attribution rule is design-owned, citing brainstorm:pipeline-measurement#P-2).

### FND-4 — G2 is measured by a metric that does not bear it

- **Severity**: blocking (CAL-1)
- **Lens**: coherence
- **Evidence**: §3.1 G2 "Measured by… M1 (floor coverage), qualitative" — vs — §3.2, where M1 measures spend-attribution accuracy (not the efficiency floor) and the floor's coverage is M2; template rule 7 / §3.1 (a qualitative goal must name its method). G2's stated metric neither measures G2 nor names a qualitative method — a goal effectively unmeasured.
- **Acceptance condition**: G2's "Measured by" cites the signal that actually bears it — M2's efficiency-floor coverage (present on 100% of phases) — and drops the unmethodded "qualitative".
- **Disposition**: applied — G2 now reads "Measured by… M2 (efficiency-floor coverage)".

## Round 2

*Scope-locked: only the four named defects' spans and regressions within changed spans were examined; no finding was raised against unchanged content.*

- **FND-1** — the model-token claim reads "zero" throughout; the fit criterion and prose agree. Closed.
- **FND-2** — M1/FR-1 state the exact-where-authoritative default with Q2 as the residual knob; the primary metric is no longer wholly deferred. Closed.
- **FND-3** — UB-5 + Q4 acknowledge nested invocation and route the rule to design; the changed spans introduce no new defect (the fold/gate remains green). Closed.
- **FND-4** — G2 cites M2's floor coverage; the goal is measured by a signal that bears it. Closed.

No regressions in the changed spans; the folded projection re-gated green. Verdict: **approve**.

## Verdict history

| Round | Verdict | Date |
|---|---|---|
| 1 | request-changes | 2026-07-20 |
| 2 | approve | 2026-07-20 |

## Escalation brief

none

## Measurements

- **Rounds**: 2
- **Findings**: 1 blocking · 3 advisory
- **Dispositions**: 4 applied · 0 declined · 0 withheld-as-violation

## Overlay corrections

*Post-approve entries only — the M2 residual data; frozen rounds and history are never edited.*

none
