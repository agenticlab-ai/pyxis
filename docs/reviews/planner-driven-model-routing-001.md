# Review: planner-driven-model-routing amendment 001 — review report

| Field | Value |
|---|---|
| **In one line** | Approve after closing the removed mode's stale lineage, making assessment evidence and diagnostics testable, and completing the plan/v1 and plan/v2 demand-to-runtime evidence chains. |
| **Relevant-when** | auditing or consuming amendment 001; asking why removal of `no_inference` also changed routing evidence and eval coverage; tuning prd-review calibration against this review |
| **Subject** | `prd:planner-driven-model-routing/001` |
| **Moment** | amendment |
| **Reviewer** | prd-review · 2026-07-26 |
| **Date(s)** | 2026-07-26 (rounds 1–2) |
| **Verdict** | approve |
| **Tracking** | — *(introducing PR not filed)* |
| **Template version** | review-report/v1 |

## Round 1

### FND-1 — F1 cited the superseded missing-assessment failure row

- **Severity**: blocking (CAL-1)
- **Lens**: coherence
- **Evidence**: folded §4.3 said F1 failure coverage was `UB-1, UB-2`, while live §5.2 defined `UB-11 *(001 ← UB-1)*` and §8 claimed F1→UB-11 — vs — `design-brainstorm:planner-driven-model-routing-tech-design#D-5`, which makes the assessed plan/v2 contract universal and removes the exception that UB-1 described.
- **Acceptance condition**: Declare §4.3 in the blast radius, make F1 cite UB-11, and leave UB-1 only in folded history.
- **Disposition**: applied

### FND-2 — NG-8 had no typed amendment provenance

- **Severity**: blocking (CAL-2)
- **Lens**: upstream-alignment
- **Evidence**: folded §3.4 contained new row NG-8, but `effective.source.json` contained no NG-8 entry because the row existed only inside an `op: supersede §3.4` payload — vs — AP-6's requirement that cross-boundary data remain schema-versioned and the PRD-ledger contract's unit-level amendment provenance.
- **Acceptance condition**: Add NG-8 through `op: add NG`, remove the section-only introduction, refold, and prove the source map resolves NG-8 to amendment 001.
- **Disposition**: applied

### FND-3 — UB-11 could pass without enforcing its timing or diagnostic

- **Severity**: blocking (CAL-3)
- **Lens**: contract-quality
- **Evidence**: UB-11 required rejection “before freeze” and a diagnostic naming the field and vocabulary, but its fit criterion checked only rejection “before projection” — vs — product vision §4 stage 6, where defective contracts bounce with named defects rather than entering planning.
- **Acceptance condition**: Require missing- and unknown-value fixtures to prove rejection before freeze and projection and to assert the offending field plus complete accepted vocabulary.
- **Disposition**: applied

### FND-4 — FR-23 verified resolvability but not substantive support

- **Severity**: blocking (CAL-3)
- **Lens**: contract-quality
- **Evidence**: FR-23 required citations “supporting its assessment values,” while its fit criterion rejected only absent or unresolved citations — vs — `design-brainstorm:planner-driven-model-routing-tech-design#F-22`, which preserves substantive supporting evidence while allowing one anchor to support multiple axes.
- **Acceptance condition**: Define an axis-support rubric and eval cases that reject irrelevant, contradictory, and partially supporting citation sets while permitting one anchor to support multiple axes.
- **Disposition**: applied

### FND-5 — EVAL-1 did not directly exercise removal of `no_inference`

- **Severity**: advisory (CAL-3)
- **Lens**: contract-quality
- **Evidence**: amended §5.9 said the eval no longer permitted a deterministic task mode, but EVAL-1 named neither a mechanical-looking agent task nor an attempted `no_inference` output — vs — `design-brainstorm:planner-driven-model-routing-tech-design#D-5`, which requires every executable `/implement-now` task to carry the complete assessment and profile.
- **Acceptance condition**: Add mechanical-looking agent tasks and attempted `no_inference` outputs to EVAL-1 and require zero such outputs, while retaining NR-8 as the separate empty-frontier proof.
- **Disposition**: applied

### FND-6 — plan/v1 attempts could not satisfy the universal evidence promise

- **Severity**: blocking (CAL-1)
- **Lens**: coherence
- **Evidence**: FR-15 translates frozen plan/v1 rungs without rewriting them, while FR-16 required every attempt to record a frozen assessment and begin at plan/v2-only EVT-1 — vs — `design-brainstorm:planner-driven-model-routing-tech-design#F-2`, which establishes that plan/v1 contains only `Mechanical` plus optional `Rung`.
- **Acceptance condition**: Define a typed, no-rewrite legacy-origin event for explicit- and absent-rung plan/v1 tasks and make both legacy and plan/v2 origins join the terminal routing chain.
- **Disposition**: applied

### FND-7 — M2 could not detect valid-value transport substitution

- **Severity**: blocking (CAL-1)
- **Lens**: coherence
- **Evidence**: M2 measured only whether runtime `requested_profile` was satisfied or promoted; it never compared that request with the frozen or translated demand, so substitution with another valid profile could remain green — vs — AP-21's requirement that downstream execution read frozen judgment verbatim rather than reinterpret it.
- **Acceptance condition**: Join origin demand to each implementation/review request, assert unchanged non-assurance components and the exact role-appropriate assurance join before native promotion, and include a substitution fixture that fails M2.
- **Disposition**: applied

## Round 2

### FND-8 — the first legacy-origin correction was not uniquely joinable

- **Severity**: blocking (CAL-1)
- **Lens**: coherence
- **Evidence**: corrected FR-24 promised each plan/v1 attempt would join EVT-8 to EVT-2/3, but EVT-8 shared no stable origin or attempt key with those terminal events, leaving retries and review attempts ambiguous — vs — AP-21's verbatim downstream consumption rule and FR-24's own complete-chain promise.
- **Acceptance condition**: Carry one stable `demand_id` across EVT-1/EVT-8 and EVT-2/EVT-3, retain a distinct `attempt_id` for each terminal attempt, and cover explicit-rung, absent-rung, retry, and review fixtures.
- **Disposition**: applied

### FND-9 — the first integrity metric treated a valid review join as substitution

- **Severity**: blocking (CAL-1)
- **Lens**: coherence
- **Evidence**: corrected M2 initially demanded whole-profile identity with the origin, while F3 and FR-13 permit the actual proposal band to raise review assurance; EVT-2/3 recorded no join source — vs — `brainstorm:planner-driven-model-routing#D-11`, which requires a monotone join between planned assurance and the actual proposal band.
- **Acceptance condition**: Require origin identity for non-assurance components, distinguish implementation assurance from the exact review join, record proposal band and join-raise state, and feed M2 from both origin and terminal events.
- **Disposition**: applied

### FND-10 — the first phasing correction omitted blocking DOD-9

- **Severity**: blocking (CAL-1)
- **Lens**: coherence
- **Evidence**: corrected §6.1 allowed Phase C to exit with DOD-1..DOD-8, while §5.8 marks DOD-9 blocking and §8 accounts for DOD-1..DOD-9 — vs — the PRD template's rule that every blocking DoD row participate in the executability gate.
- **Acceptance condition**: Require DOD-1..DOD-9 at Phase C exit.
- **Disposition**: applied

No further findings survived the scope-locked re-review of the corrected spans.

## Verdict history

| Round | Verdict | Date |
|---|---|---|
| 1 | request-changes | 2026-07-26 |
| 2 | approve | 2026-07-26 |

## Escalation brief

none

## Measurements

- **Rounds**: 2
- **Findings**: 9 blocking · 1 advisory
- **Dispositions**: 10 applied · 0 declined · 0 withheld-as-violation

## Overlay corrections

none
