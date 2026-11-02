# Review: installer amendment 004 — review report

| Field | Value |
|---|---|
| **In one line** | Approved after four findings clarified complete execution, payload absence, and advisory timing evidence. |
| **Relevant-when** | reviewing installer amendment 004 or its runtime-simplification acceptance and compatibility boundary |
| **Subject** | prd:installer/004 |
| **Moment** | amendment |
| **Reviewer** | prd-review · 2026-09-17 guide |
| **Date(s)** | 2026-09-17 |
| **Verdict** | approve |
| **Tracking** | prerequisite of https://github.com/agenticlab-ai/pyxis/pull/1199 |
| **Template version** | review-report/v1 |

## Round 1

Four independent lenses examined the validated folded amendment, excluding human-only §0 and the mechanical gate floor. Each candidate below survived independent adversarial screening. Coherence produced no additional candidate.

### FND-1 — Verify complete self-contained operations

- **Severity**: advisory (CAL-3)
- **Lens**: contract-quality
- **Evidence**: FR-31 originally required only that operations “pass their prerequisite boundaries”; repair or packaging could then invoke an undeclared package-install stage. UB-14 closes init/doctor only. AP-14 requires observing the promised behavior.
- **Acceptance condition**: require completion of repair and packaging using only declared release artifacts, verify their outputs, and give observable feedback-capture and plan-viewer smoke outcomes.
- **Disposition**: applied — FR-31 now requires complete operations and their outputs; release-entrypoint preflight remains additional evidence.

### FND-2 — Prove the payload contains only declared assets

- **Severity**: advisory (CAL-4)
- **Lens**: completeness-critic
- **Evidence**: FR-31's original prerequisite checks permitted dormant, undeclared files to remain shipped. The operator explicitly required deleting the approved paths; AP-14 calls for packaged-artifact evidence.
- **Acceptance condition**: inspect the payload and fresh installed tree against the class-policy inventory; every shipped path must be declared and every approved deletion must be absent.
- **Disposition**: applied — FR-31 now states the file-absence boundary and preserves the independent tools.

### FND-3 — Align the live payload description

- **Severity**: blocking (CAL-2)
- **Lens**: upstream-alignment
- **Evidence**: folded §4.1 still included a secondary runtime in the payload after FR-12 changed, contradicting FR-31 and the operator's decision. Product vision §13 requires code and current authoritative intent to agree.
- **Acceptance condition**: supersede §4.1's payload enumeration with the self-contained artifact set while retaining independent viewer assets.
- **Disposition**: applied — the amendment replaces §4.1 with the corrected enumeration.

### FND-4 — Keep timing measurements outside blocking acceptance

- **Severity**: blocking (CAL-2)
- **Lens**: upstream-alignment
- **Evidence**: the replacement NFR-9 retained a percentile timing test and blocking DOD-14 required correctness “at the install budget.” AP-29 and incident I5 prohibit timing measurements as correctness gates.
- **Acceptance condition**: preserve the numerical targets as advisory measurements; make DOD-14 block on matrix correctness only.
- **Disposition**: applied — NFR-9 records advisory percentiles, and DOD-14 explicitly separates those measurements from its verdict.

## Round 2

Scope-locked review of the changed spans closes FND-1 through FND-4. FR-31 requires complete self-contained operations and exact payload/fresh-install inventory proof; §4.1 agrees with that contract; NFR-9 and DOD-14 separate timing measurements from blocking correctness. No changed-span regression or new finding survived. The amendment validates and folds after the revisions.

## Verdict history

| Round | Verdict | Date |
|---|---|---|
| 1 | request-changes | 2026-09-17 |
| 2 | approve | 2026-09-17 |

## Escalation brief

none

## Measurements

- **Rounds**: 2
- **Findings**: 2 blocking · 2 advisory
- **Dispositions**: 4 applied · 0 declined · 0 withheld-as-violation

## Overlay corrections

none
