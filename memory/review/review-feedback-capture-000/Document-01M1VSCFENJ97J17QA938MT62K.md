# Review: review-feedback-capture/000 — review report

| Field | Value |
|---|---|
| **In one line** | approve — five findings across the four lenses, all applied to the draft (one blocking upstream-alignment finding on a merged amendment-model dependency; two contract-quality fixes to the metrics section; two completeness/honesty fixes). |
| **Relevant-when** | auditing or consuming the prd-review of `review-feedback-capture`; asking why it was approved; tuning prd-review calibration against these findings |
| **Subject** | `prd:review-feedback-capture/000` |
| **Moment** | new-base |
| **Reviewer** | prd-review · guide 2026-07-12 |
| **Date(s)** | 2026-07-12 |
| **Verdict** | approve |
| **Tracking** | — *(the introducing PR, once filed)* |
| **Template version** | review-report/v1 |

## Round 1

### FND-1 — M3's target is not a number

- **Severity**: advisory (CAL-3)
- **Lens**: contract-quality
- **Evidence**: §3.2 M3 target "the overwhelming majority; any gap is itself visible in the ledger" — vs — prd-template Appendix A banned-vagueness (comparatives without numbers; metric targets are numeric/observable)
- **Acceptance condition**: M3 states a numeric threshold (or a precisely-defined observable) over the checkpoint cohort
- **Disposition**: applied — M3 target set to **≥ 99%** of touchpoints (capture failures tracked via UB-1); reviewer-recommended default, operator-adjustable pre-merge

### FND-2 — the derived measures are undefined

- **Severity**: advisory (CAL-3)
- **Lens**: contract-quality
- **Evidence**: §5.1 FR-5 "review-rejection rate, human edit-distance" — vs — prd-template Appendix A fit-criterion rule (a measure is verifiable only if its denominator/unit is named); FR-5's fit checks *a* value is computed, not the *right* one
- **Acceptance condition**: the three measures are defined, or their precise definition is explicitly deferred to the tech design as an owned open question
- **Disposition**: applied — FR-5's fit now references the definitions and §7.2 Q5 pins them (denominator, unit) to the tech design

### FND-3 — missing failure mode: a dangling subject anchor

- **Severity**: advisory (CAL-4)
- **Lens**: completeness-critic
- **Evidence**: §5.2 UB-1..5 cover write-failure, host-down, no-change, duplicate, pending — vs — the capture surface (F1/F2) implies a subject may be superseded/retired or its branch/PR deleted, leaving an entry's anchor unresolvable; that failure mode was uncovered
- **Acceptance condition**: a UB row (or §5.4 integrity rule) governs an entry whose subject anchor no longer resolves
- **Disposition**: applied — added UB-6 (retain the entry, flag the anchor stale, zero drops)

### FND-4 — G3's qualitative measure is ceremonial for a solo operator

- **Severity**: advisory (CAL-3)
- **Lens**: contract-quality
- **Evidence**: §3.1 G3 measured by "interviews / unprompted operator language" — vs — prd-template Appendix-A rule 15 (roles/methods collapse honestly at solo team size; no ceremonial method)
- **Acceptance condition**: G3's measure is restated as the operator's own observation at review, not interviews
- **Disposition**: applied — restated to the operator's own observation of a comment's outcome in the ledger (rule 15)

### FND-5 — FR-4 changes a merged design's field without acknowledging the supersession path

- **Severity**: blocking (CAL-2)
- **Lens**: upstream-alignment
- **Evidence**: §5.1 FR-4 / §5.5 "make `review-thread` resolve to a ledger entry" — vs — `Learned-from` (with its `review-thread` class) is defined in the **merged** `prd-amendment-model`, which is immutable and evolves only by supersession (AGENTS.md doc-system rule 2 · AP-7); the draft asserted the change without naming that constraint
- **Acceptance condition**: §5.5 / §6.2 state that changing `Learned-from`'s resolution semantics is a superseding-design dependency, never a silent edit to the merged model
- **Disposition**: applied — §5.5 interface row and §6.2 dependency now carry the supersession constraint

## Verdict history

| Round | Verdict | Date |
|---|---|---|
| 1 | approve | 2026-07-12 |

*Author=reviewer single pass in the advisory period: five findings raised, all applied to the draft, and the folded projection re-gated green against their acceptance conditions. The blocking finding (FND-5) was resolved, not declined, so nothing escalates. Two dispositions (FND-1's ≥ 99% target, FND-2's Q5 deferral) are reviewer-recommended defaults the operator can revise before merge.*

## Escalation brief

none

## Measurements

- **Rounds**: 1
- **Findings**: 1 blocking · 4 advisory
- **Dispositions**: 5 applied · 0 declined · 0 withheld-as-violation

## Overlay corrections

*Post-approve entries only — the M2 residual data; frozen rounds and history are never edited.*

none
