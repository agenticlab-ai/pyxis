# Review: prd:feature-draft/000 — review report

| Field | Value |
|---|---|
| **In one line** | The feature-draft base, approved in round 2 after round 1's eight blocking findings — post-release fit criteria, the undefined "areas" overlap, plan validity at the lock, the previous plan's tasks on a reopen (I9), dropped contract-gate checks, the silent read-to-rule gap, an invented dependency timing, and lock-approval integrity — and six advisories were all applied. |
| **Relevant-when** | auditing or consuming the review of prd:feature-draft; asking why its base was bounced or approved; tuning prd-review calibration against real findings |
| **Subject** | prd:feature-draft/000 |
| **Moment** | new-base |
| **Reviewer** | prd-review · `.pyxis/authoring/prd-review.md` as of 2026-09-22 · run in the authoring session |
| **Date(s)** | 2026-09-22 |
| **Verdict** | approve |
| **Tracking** | — |
| **Template version** | review-report/v1 |

## Round 1

### FND-1 — fit criteria that only post-release features can satisfy

- **Severity**: blocking (CAL-1)
- **Lens**: coherence
- **Evidence**: FR-3 fit "Every part of each fixture draft and of the first three draft-born features is retrievable…", FR-5 fit "In every fixture and in the first three draft-born features…", FR-17 fit "…every lock of the first three draft-born features has a human approval record" (§5.1) — vs — DOD-1 "Every Must FR passes its fit criterion", blocking, and §6.4 Functionality requiring DOD-1 before the release lands (§5.8, §6.4); the first three draft-born features exist only after the release (§6.7).
- **Acceptance condition**: each of FR-3, FR-5, and FR-17 states a fit criterion evaluable on the release candidate, and any post-release observation moves to a metric or the §6.7 checkpoint.
- **Disposition**: applied — FR-3, FR-5, and FR-17 fit criteria now evaluate on fixtures, the end-to-end fixture feature included; post-release observation stays in M3 and §6.7

### FND-2 — "areas" in the open-draft overlap rows is undefined

- **Severity**: blocking (CAL-3 — the undefined term makes part of two Must rows unverifiable)
- **Lens**: contract-quality
- **Evidence**: FR-23 "…the other open drafts that touch the same requirements or areas" and FR-24 "…touching the same requirements or areas…", whose fit criteria test only a shared requirement (§5.1) — vs — prd-template Appendix A: every requirement verifiable and unambiguous; brainstorm:feature-draft#D-10 carries the word but no definition.
- **Acceptance condition**: "area" is defined in observable terms that a fixture can decide, or the rows are restated without it with the operator's confirmation of the narrower reading.
- **Disposition**: applied — by brainstorm:feature-draft#D-22 (superseding D-10): FR-23 restated as an advisory notice on requirements or decisions, FR-24 as readiness computed against the landing mainline; "areas" no longer appears

### FND-3 — readiness omits plan validity, and a locked plan's projection refusal has no failure row

- **Severity**: blocking (CAL-4 — load-bearing for §8's failure coverage of flow F3)
- **Lens**: completeness-critic
- **Evidence**: FR-12–FR-16 list readiness without any condition that the plan passes the planning rules the projection enforces; §5.5 "projection refusal → the lock stands and the handoff is fixed forward"; FR-25 promises the locked plan in the form today's executor consumes — vs — §8 "Every key flow (§4.3) has functional, failure, and non-regression coverage (§5.1–5.2)"; F3 ends in projection.
- **Acceptance condition**: readiness includes a condition that the draft's plan passes the rules the plan projection enforces, so no ready draft can lock an unprojectable plan — or a UB row states the behavior when a locked plan is refused.
- **Disposition**: applied — FR-37 adds plan validity to readiness; UB-10 covers a projection refusal of a locked plan; §5.5 updated

### FND-4 — a reopen or migration can silently invalidate the previous binding plan's tasks (I9)

- **Severity**: blocking (CAL-2)
- **Lens**: upstream-alignment
- **Evidence**: FR-22 "…make it the feature's current locked version and keep the replaced version readable as history" and FR-28 (migration) say nothing about tasks of the previously binding plan that were already projected, in flight, or delivered while their traced requirements change or retire (§5.1) — vs — incident I9: "A requirement ID retired by an amendment silently invalidates every merged plan that cites it, and the plan class offers no repair once frozen."
- **Acceptance condition**: the PRD states what happens, at the lock of a reopened or migrated draft, to each task of the previously binding plan whose traced requirement the new version changes or retires — or records it as an owned open question the solution gate must close.
- **Disposition**: applied — by brainstorm:feature-draft#D-19 and brainstorm:feature-draft#D-23: FR-43 refuses a reopen or migration while the current plan has unfinished tasks; FR-44 keeps finished tasks tracing the version they delivered

### FND-5 — replacing the contract gate drops its currency, link, structural, and compilability checks

- **Severity**: blocking (CAL-2)
- **Lens**: upstream-alignment
- **Evidence**: FR-31 "…with FR-11 through FR-16 as the planning-entry gate" in place of prd:contract-gate/FR-1; FR-11–FR-16 cover answered questions, inverse coverage, fit criteria, plan coverage, and review dispositions (§5.1) — vs — prd:contract-gate's gate "validates a feature's assembled contract — currency, inverse coverage, link resolution, structural completeness, compilability"; vision §4 stage 6 bounces contradictory or unverifiable contracts before planning.
- **Acceptance condition**: readiness carries the contract gate's remaining checks adapted to a draft (at least currency against other features' superseded content and link resolution), or the PRD names each dropped check with its reason.
- **Disposition**: applied — by brainstorm:feature-draft#D-20: FR-38 (currency), FR-39 (links), FR-40 and FR-41 (structure), FR-42 (consistency) join readiness; FR-11 and FR-31 updated

### FND-6 — read-to-rule's §0 and gate capture line silently do not reach draft-born PRDs

- **Severity**: blocking (CAL-2)
- **Lens**: upstream-alignment
- **Evidence**: the subject never states whether a draft-born PRD follows the PRD template, carries a §0 read-to-rule section, or whether the lock approval carries the gate capture line (§4, §5.1 FR-17/FR-18) — vs — prd:read-to-rule "Every new PRD opens with §0 'Read to rule'…"; prd:read-to-rule/FR-7 (capture line on gate sign-offs).
- **Acceptance condition**: the PRD states whether a draft-born PRD follows the PRD template including §0, and whether the lock approval carries the capture line — or records the question as an owned open question with the gate it blocks.
- **Disposition**: applied — by brainstorm:feature-draft#D-21: FR-41 holds draft-born PRDs to the PRD template with §0; FR-45 records the capture line or a lineless mark

### FND-7 — a dependency timing the basis never ruled

- **Severity**: blocking (CAL-2)
- **Lens**: upstream-alignment
- **Evidence**: §6.2 "Vision §4 note … | before this PRD's approval | requested" — vs — brainstorm:feature-draft#D-13 records that the operator appends the note separately and sets no timing.
- **Acceptance condition**: the row's Needed-by states only what the basis rules (no timing), or the operator rules a timing recorded in the basis.
- **Disposition**: applied — the §6.2 row now reads "not ruled — the operator's timing"

### FND-8 — lock approval authenticity and binding are unstated

- **Severity**: blocking (CAL-4 — load-bearing for §8's failure coverage of flow F3 on a critical surface)
- **Lens**: completeness-critic
- **Evidence**: FR-17 fit "approval attempts by any agent identity are refused" and §5.5 "a human identity is required", with no statement of how identity is established or that the approval binds to the version presented; ABUSE-2 covers self-approval, not forgery or a changed draft between request and approval (§5.1, §5.3, §5.5) — vs — cmd/memory/README.md: a record's actor is taken from `--actor`, then `$MEMORY_ACTOR`, then git `user.email` — self-declared, not authenticated.
- **Acceptance condition**: the PRD requires that a lock approval come from an authenticated human identity that agent sessions cannot assume, and that it bind to the exact version presented, with a changed draft voiding it; an ABUSE row covers approval forgery and a UB row covers the version race.
- **Disposition**: applied — FR-46 (authenticated human identity only), FR-47 (approval bound to the presented version), UB-11, ABUSE-8; §5.3–§5.5 updated

### FND-9 — reported contract state for reopened and unmigrated features

- **Severity**: advisory (CAL-3)
- **Lens**: contract-quality
- **Evidence**: FR-7 "…as drafting while a feature draft of it is open and as its locked version number otherwise", while its fit expects a distinct "reopened while v1 binds" state and nothing covers a feature still in the document pipeline (§5.1).
- **Acceptance condition**: FR-7 names the state reported for a reopened feature and for an unmigrated feature.
- **Disposition**: applied — FR-7 restated with the four states and fixtures for each

### FND-10 — M2's baseline measures a broader class than its signal

- **Severity**: advisory (CAL-3)
- **Lens**: contract-quality
- **Evidence**: M2 signal "reopens … whose confirmed cause is a gap the readiness checks should have caught" with baseline "22 of 59 amendments cited later-stage evidence", which also counts cross-feature dependency changes (§3.2).
- **Acceptance condition**: the baseline is labeled as a proxy upper bound, or restated in the signal's own terms.
- **Disposition**: applied — the M2 baseline is labeled a proxy upper bound

### FND-11 — abandoning an open draft is unspecified

- **Severity**: advisory (CAL-4)
- **Lens**: completeness-critic
- **Evidence**: §5.4 "Retention — until its lock or abandonment" has no flow or requirement; an abandoned but open draft would keep appearing in FR-23 overlap lists and trigger FR-24 re-checks.
- **Acceptance condition**: the PRD states what abandoning an open draft does, or records it as an owned open question.
- **Disposition**: applied — recorded as the owned open question Q11

### FND-12 — document-pipeline proposals still open at the switch-over

- **Severity**: advisory (CAL-4)
- **Lens**: completeness-critic
- **Evidence**: FR-27 refuses new ledger entries and designs once the release lands; F6 and UB-5 do not say what happens to a document-pipeline proposal already open at that moment.
- **Acceptance condition**: the PRD states the fate of document-pipeline proposals open at the switch-over.
- **Disposition**: applied — UB-5 and F6 cover a document-pipeline proposal still open at the switch-over

### FND-13 — FR-36's risk tier understates its surface

- **Severity**: advisory (CAL-3)
- **Lens**: contract-quality
- **Evidence**: FR-36 risk "medium" for changes to the repository's agent instructions (§5.1) — vs — prd-template risk tiers: "critical — … anything that can compromise factory control (governance, gates, policy)".
- **Acceptance condition**: FR-36's risk is raised to match the governing surface it changes, or the medium tier is justified.
- **Disposition**: applied — FR-36 risk raised to high

### FND-14 — retiring requirements through a reopen is implicit

- **Severity**: advisory (CAL-4)
- **Lens**: completeness-critic
- **Evidence**: FR-9 applies "the ruled change" and FR-2 accepts "revisions", but nothing states that a draft can retire a requirement as a typed terminal disposition — the path prd:abandoned-feature-retirement/FR-2 needs once amendments are refused (FR-27).
- **Acceptance condition**: the PRD states that a ruling may retire a requirement and that the locked version records it as retired.
- **Disposition**: applied — FR-9 names retirement among ruled changes and its fit checks the retired state; F4 states the abandonment path

## Round 2

Scope-locked to round 1's named defects and regressions within the spans the revision changed.

- **Closure** — FND-1 through FND-14 each meet their acceptance condition in the revised subject (dispositions above).
- **Regressions in changed spans** — none. Checked: the new readiness rows FR-37 through FR-42 against EARS form and verifiability; FR-43 through FR-47; the restated FR-7, FR-9, FR-23, FR-24, UB-5, and UB-8; UB-10, UB-11, and ABUSE-8; NG-7 against FR-23; the revised flows F2 through F6; the §5.3–§5.5 rows; EVT-5; the DoD sources; R8; Q11; and the §7.3 rows citing the review-born rulings.
- **New findings** — none.

## Verdict history

| Round | Verdict | Date |
|---|---|---|
| 1 | request-changes | 2026-09-22 |
| 2 | approve | 2026-09-22 |

## Escalation brief

none

## Measurements

- **Rounds**: 2
- **Findings**: 8 blocking · 6 advisory (round 1); none in round 2
- **Dispositions**: 14 applied · 0 declined · 0 withheld-as-violation

## Overlay corrections

none
