# Review: implement-now amendment 002 — review report

| Field | Value |
|---|---|
| **In one line** | Request changes: e2e recognition guarantees an exclusivity no artifact is required to record, and a feature whose whole output is new blocking checks carries no hermeticity row against the incident that already paid for it. |
| **Relevant-when** | auditing or consuming implement-now amendment 002; asking why e2e task consumption was bounced then approved; tuning prd-review calibration against executor-consumer findings |
| **Subject** | `prd:implement-now/002` |
| **Moment** | amendment |
| **Reviewer** | prd-review · 2026-07-28 |
| **Date(s)** | 2026-07-28 (rounds 1–2) |
| **Verdict** | approve |
| **Tracking** | pyxis#760 |
| **Template version** | review-report/v1 |

## Round 1

### FND-1 — FR-28 guarantees exclusive e2e recognition that no artifact is required to record

- **Severity**: blocking (CAL-1)
- **Lens**: coherence
- **Evidence**: FR-28 requires the executor to "resolve the task to exactly one scenario — or to the harness slot — in that feature's approved e2e design doc … without introducing an e2e-specific task type or any additional required charter field", with the fit criterion "every e2e task resolves to exactly one scenario or the harness slot and **every non-e2e task resolves to none**"; NR-8 closes the remaining door — "no e2e-specific task type, required charter field, or second projection path is introduced, and plan-new and plangate are not edited" — vs — **AP-6** ("No free-form text between components: all cross-boundary information is schema-versioned structured data"). No live row in this amendment, in `prd:e2e-verification`, or in `prd:plan-new` obliges any artifact to record that a plan is an e2e plan, so the only path left to the outcome FR-28 states is inference over charter prose. UB-14 fails closed on an *ambiguous* join but cannot see a **confident wrong** one — a feature-implementation task that resolves cleanly to a scenario — which is precisely the case the fit criterion forbids. Every other new row (FR-29, FR-30, FR-31, UB-15, UB-16) keys off this resolution being right, so the gap is load-bearing rather than mechanical. The mirror precedent exists: `prd:plan-new/§6.2` carries "executor sibling amendment consuming plan/v2 demand and preserving plan/v1 translation … required before end-to-end activation" for exactly this kind of cross-feature obligation.
- **Acceptance condition**: name the authoritative, machine-readable signal the executor resolves the origin from — an existing required field is sufficient and needs no plan-new edit (`internal/plangate/plan.loadDesign` reads the plan header's `Design` row as a verbatim path, so an e2e plan may name `docs/e2e/<feature>/design.md` there) — and add the matching `§6.2` dependency row binding the upstream side to write it.
- **Disposition**: applied

### FND-2 — a feature whose entire output is new blocking checks carries no hermeticity requirement, repeating I5

- **Severity**: blocking (CAL-2)
- **Lens**: upstream-alignment
- **Evidence**: FR-30 makes the deliverable "the automated test the scenario names", and live FR-7 counts a task done only on "a proposal whose local gate and required remote CI are green" — so every e2e task adds a check that can fail a change — vs — **`incident:I5`** (2026-07-21) tagged `[code-gate, implement-now, ci-mirror, hermeticity]`, whose lesson is that "a blocking-gate check must be hermetic (same input → same verdict on any machine)" and whose record states "**Even a command-level mirror would not have caught it locally**"; graduated to **AP-29** ("A blocking gate … must be **hermetic**: a pure function of its declared input"). FR-8's CI mirror is the *other* half of I5 and does not make a flaky test deterministic. Nothing upstream absorbs the duty either: `prd:e2e-verification`, `design:e2e-verification-skills`, and the e2e-design authoring guide contain no hermeticity, flakiness, or determinism row. The executor is the last actor before the check becomes required CI, and the reachable failure — green locally, red in CI, FR-9 self-heal loop, FR-15 escalation — is I5's own path re-walked on every e2e task. WR-3 does not park it: it names falsification sweeps, e2e test-delta, hard app shapes, and the no-regression gate, and a parked concern must be named to be parked.
- **Acceptance condition**: either add a live row requiring an executor-authored e2e test to be hermetic before it is counted green, with a fit criterion a `Verify` method can fail, or name hermeticity explicitly in WR-3 with the reason it waits.
- **Disposition**: applied

### FND-3 — FR-31 bounds mock extent but not mock fidelity, against AP-14's paid-for corollary

- **Severity**: advisory (CAL-4)
- **Lens**: completeness-critic
- **Evidence**: FR-31 requires that "a `mocked` scenario shall mock only the surfaces its recorded mock strategy names", verified by "a mocked scenario mocking only its named surfaces" — vs — **AP-14** ("a test double must satisfy the **real dependency's contract**, not the code-under-test's assumption about it — a stub that mirrors your own wrong guess makes a never-working feature test green"). Extent and fidelity are different properties: a mock of exactly the named surface, written to the worker's own assumption about that surface, passes FR-31, passes UB-16 (the recorded route was honored), and survives DOD-19's broken-subject falsification whenever the mocked surface is a dependency rather than the subject. e2e-review's anti-gaming lens judges the recorded *strategy* at design time; it never sees the mock the executor writes.
- **Acceptance condition**: extend FR-31's fit criterion (or WR-3's parked scope, with a reason) to cover the mock double honoring the real dependency's contract.
- **Disposition**: applied

### FND-4 — EVT-12 is the only evidence-bearing event with no §5.4 data row

- **Severity**: advisory (CAL-4)
- **Lens**: completeness-critic
- **Evidence**: the amendment adds EVT-12 `e2e_task_basis_resolved` and a new consumed artifact (the resolved e2e basis) but leaves §5.4 untouched — vs — the section's own established pattern, where every sibling evidence event carries an entity row with its source of truth, retention, and integrity rules: "routing attempt … immutable EVT-7 or EVT-8 … forever in git evidence" and "evidence invalidation … immutable EVT-11 … append-only". The consequence is that the basis the worker is briefed from has no stated integrity rule or retention in the section that exists to state them, while NG-5's never-edit posture sits in a non-goals table rather than the data contract.
- **Acceptance condition**: add the e2e basis and its EVT-12 evidence to §5.4 with source of truth, retention, integrity rules, and privacy class, matching the rows already there.
- **Disposition**: applied

## Round 2

*Scope-locked to the four named defects and regressions within the changed spans.*

- **FND-1** — closed. FR-28 now resolves the origin from "the plan's own governing-document header naming that feature's approved e2e design doc", NR-8 records that this uses an existing required field with no plan-new or plangate edit, and the new §6.2 row binds e2e-plan to write it before the phase can proceed. The fit criterion's "every non-e2e task resolves to none" is now supported by a recorded signal rather than inference.
- **FND-2** — closed. FR-32 requires an executor-authored e2e test to be hermetic before it is counted green, UB-17 handles the non-hermetic case without an unbounded self-heal loop, and DOD-19 carries the repeat-run evidence. AP-29 and I5 are cited in the rationale.
- **FND-3** — closed. FR-31's fit criterion now requires each mock double to satisfy the real dependency's observed contract, with a wrong-guess stub as the named negative case.
- **FND-4** — closed. §5.4 gains the e2e scenario basis and the EVT-12 origin-decision rows, both with source of truth, retention, integrity rules, and privacy class.

No regression found within the changed spans: the added rows introduce no new ID collisions, the declared blast radius still equals the computed tier, and §8's enumerations were re-extended to cover FR-32, UB-17, and the widened DOD-19.

## Verdict history

| Round | Verdict | Date |
|---|---|---|
| 1 | request-changes | 2026-07-28 |
| 2 | approve | 2026-07-28 |

## Escalation brief

none

## Measurements

- **Rounds**: 2
- **Findings**: 2 blocking · 2 advisory
- **Dispositions**: 4 applied · 0 declined · 0 withheld-as-violation

## Overlay corrections

*Post-approve entries only — the M2 residual data; frozen rounds and
history are never edited.*

none
