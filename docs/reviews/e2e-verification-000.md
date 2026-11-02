# Review: prd:e2e-verification/000 — review report

| Field | Value |
|---|---|
| **In one line** | request-changes on two blocking findings (round 1) → both applied, plus all six advisories dispositioned (five applied, one routed to a design-owned question) → approve-with-advisories (round 2): the lens set was narrowed to the two lenses it proves, the model-driven catch corpus became a recorded checkpoint measurement (per I5), and the coverage/cost/vagueness/degenerate-feature advisories were fixed. |
| **Relevant-when** | auditing or consuming the review of the e2e-verification 000-base; asking why round 1 requested changes and how it converged; tuning prd-review calibration against real findings |
| **Subject** | prd:e2e-verification/000 |
| **Moment** | new-base |
| **Reviewer** | prd-review · 2026-07-23 |
| **Date(s)** | 2026-07-23 |
| **Verdict** | approve-with-advisories *(round 2; history below is authoritative)* |
| **Tracking** | — *(the subject's introducing PR, once filed)* |
| **Template version** | review-report/v1 |

## Round 1

*Generated through four blind, adversarially-screened lenses over the docgate context pack (vision · principles · incidents · folded subject · calibration). The gate floor and §0 are excluded. No finding disputes a recorded decision's content.*

### FND-1 — FR-8 mandates four review lenses, but only two of them can ever fail a check

- **Severity**: blocking (CAL-3 — becomes blocking when the defect makes a Must row unverifiable)
- **Lens**: contract-quality
- **Evidence**: FR-8 (§5.1, Must, Risk high) required "lenses for **coverage completeness, anti-gaming** …, **feasibility** …, and **faithfulness** …", but its fit criterion exercised only coverage and anti-gaming, and §5.9's only eval rows were EVAL-1 (coverage) and EVAL-2 (anti-gaming) — so a build that stubbed the feasibility and faithfulness lenses would pass every stated check. — vs — Appendix A's fit-criterion rule and §5.9 eval-adequacy for model-driven features; §7.2 Q2 fixed the four dimensions as Must, not deferrable.
- **Acceptance condition**: verify all four lenses, **or** demote feasibility + faithfulness so FR-8 mandates only what it verifies.
- **Disposition**: applied (demote path) — feasibility and faithfulness lenses removed; FR-8 now mandates only coverage + anti-gaming, the two the catch corpus verifies. Updated to match: §0 sanity-check, §1 Direction, §3.1 G2, §4.3 F2, FR-8 (requirement + rationale + fit), the EVT-2 lens enum, ABUSE-3 / R5 mitigations, §7.2 Q2, and a §7.3 decision-log entry recording the narrowing from the brainstorm's four (feasibility/faithfulness noted as design-owned advisory-only options).

### FND-2 — the model-driven catch corpus is specified as a permanent *blocking* CI/ship gate with no hermeticity strategy (repeats I5)

- **Severity**: blocking (CAL-2 — upstream-alignment; default blocking; declined-with-reason → escalate)
- **Lens**: upstream-alignment
- **Evidence**: the lenses are model-driven (§5.9), yet the catch corpus over them was a blocking gate with an every-case bar: DOD-3 (**Blocking? yes**) "runs as a permanent CI suite"; ship rule "any seeded case slipping through → not shippable". No determinism NFR shipped. — vs — incident I5: "a blocking-gate check **must be hermetic** … Measurements record; they never gate." A model-driven verdict can flip on identical seeded input. The adjacent sibling kept its blocking gate deterministic-by-construction (`implementation-verification` NFR-2; zero LLM judgment) and deferred judgment out of the gate "before trust exists".
- **Acceptance condition**: re-specify the catch corpus as a recorded checkpoint measurement, **or** make it hermetic-by-construction (pass-rate-over-N + a flakiness/aggregation NFR) — citing I5.
- **Disposition**: applied (measurement path) — the catch corpus is now a recorded checkpoint measurement feeding the reviewer's advisory→blocking ramp, not a per-commit gate: DOD-3 → "no — recorded measurement"; ship rule reworded so a low catch-rate keeps the reviewer advisory and triggers lens tuning (never a per-commit red), citing I5; M2/M3 "measured at the checkpoint"; EVT-3 source "catch-corpus run"; §6.1 Phase A, §6.4 Reliability, and the §8 DoD-complete note aligned. The dogfood (DOD-2) and false-red / displaced-authoring guardrails (DOD-4) remain the blocking quality bars.

### FND-3 — FR-6's fit criterion turns on banned-vagueness ("shape-appropriate") with no rubric

- **Severity**: advisory (CAL-3 — default advisory; FR-6 is a Should and no Must row's verifiability depends on it)
- **Lens**: contract-quality
- **Evidence**: FR-6 named a harness "**appropriate to the feature's app shape**", fit "inspection confirms the choice is **shape-appropriate**", Verify a bare "Inspection" — subjective and circular. — vs — the banned-vagueness rule; FR-1 checks only presence, not fitness.
- **Acceptance condition**: restate FR-6's fit as an objective interface-driving check.
- **Disposition**: applied — FR-6 rewritten to require a harness that can drive the app-shape's primary external interface (web HTTP endpoint / CLI argv+stdout / library public API), verified against each fixture's declared primary-interface class; Verify now "Inspection, Test".

### FND-4 — the §5.3 adversary model misses coverage-inflation (over-mapping), a distinct "look complete" vector that reads green

- **Severity**: advisory (CAL-4 — default advisory; weakens FR-2's guarantee but does not by itself falsify a §8 readiness ✓)
- **Lens**: completeness-critic
- **Evidence**: §5.3's three vectors (hollow, omit, over-route) miss **mapping a scenario to behaviors it does not actually exercise** — FR-2's fit only catches an *unmapped* entry and FR-8's anti-gaming lens is scenario-scoped, so an over-mapped behavior reads green.
- **Acceptance condition**: add an ABUSE row for over-mapping, mitigated by a per-map-edge check plus a seeded over-mapped catch case.
- **Disposition**: applied (captured; mechanism to design) — ABUSE-4 added naming over-mapping/coverage-inflation (high) and mapping the F4 flow to it; the per-map-edge exercise check and its seeded catch case are routed to design (§7.2 Q2). The threat is on record; the mechanism is design-owned.

### FND-5 — no defined handling for a feature with no meaningfully-automatable e2e surface (all-manual / pure-doc)

- **Severity**: advisory (CAL-4 — a real input state with no defined output; not tied to a specific false §8 ✓)
- **Lens**: completeness-critic
- **Evidence**: §2.2's anti-segment names manual-only behaviors and the repo is largely docs/skills/tooling, so the dogfood's "one real pyxis feature" may have zero CI-runnable behaviors — but UB-3 handles only a *single* unroutable behavior; no row defines the all-manual / pure-doc aggregate.
- **Acceptance condition**: add a UB row (or FR clause) defining the all-manual / no-automatable-surface outcome.
- **Disposition**: applied — UB-6 added (Must): a feature with no meaningfully-automatable e2e surface yields an explicit manual-only (or empty, reason-recorded) design and a manual checklist, never fabricated scenarios or a degenerate build plan; fixtures (an all-manual feature and a pure-doc feature) specified; §8 F1 coverage updated.

### FND-6 — §5.5 reads "its technical design" (singular) and UB-2 catches only missing/unreadable, so a superseded design silently yields stale coverage

- **Severity**: advisory (CAL-4 — a silent-wrong-output state with no refusal; design-time-resolvable)
- **Lens**: completeness-critic
- **Evidence**: §5.5 has e2e-design read "**its technical design**" (singular); UB-2 refuses only "missing or unreadable" — but designs evolve by supersession (old stays present), so a present-but-superseded design slips through.
- **Acceptance condition**: define how e2e-design selects the live (non-superseded) design set, surfacing or refusing on ambiguity.
- **Disposition**: deferred to design — recorded as §7.2 Q6 (governing-design selection under multiple/superseded designs), with §0 and the §7.2/§8 open-question notes updated; v1 assumes a single live design. Acknowledged and assigned to the design step, not fixed in the PRD.

### FND-7 — the recurring per-feature loop states no bounded steady-state cost; NFR-3 verifies only that spend is *recorded* (AP-19)

- **Severity**: advisory (CAL-4 — cost has no blocking DoD row, so it does not gate §8)
- **Lens**: completeness-critic
- **Evidence**: a per-feature, model-driven, multi-skill loop with a convergence loop and a whole-tree read; NFR-3's fit tested only that spend is "**recorded**" and asserted "no unbounded fan-out" with no mechanism. — vs — AP-19 ("every recurring path states its steady-state cost, and per-decision context is bounded").
- **Acceptance condition**: require an enforced cost bound and test it (numbers design-owned).
- **Disposition**: applied — NFR-3 tightened to require an enforced convergence-round cap and a bound on the code-context each e2e-design run ingests (specific caps design-owned); the fit now demonstrates the cap and bound are *enforced* (a run reaching the cap stops), Verify "Analysis, Test".

### FND-8 — ABUSE-3's Verify references a catch-corpus family ("over-mocked cases") the doc never defines and elsewhere defers

- **Severity**: advisory (CAL-3 — a Verify method naming a non-existent artifact; the row keeps a working Inspection prong)
- **Lens**: contract-quality
- **Evidence**: ABUSE-3's Verify read "Inspection (dogfood review); **catch corpus (over-mocked cases)**" while its Mitigated-by deferred route-balance to WR-1 — but the corpus defines only two families (coverage_gap · hollow_scenario).
- **Acceptance condition**: drop the non-existent catch-family reference from ABUSE-3's Verify.
- **Disposition**: applied — "; catch corpus (over-mocked cases)" removed; ABUSE-3's Verify is now "Inspection (dogfood review)".

## Round 2

*Terminal re-review, scope-locked to the round-1 findings and regressions within their changed spans. No finding is raised against unchanged content.*

- **Blocking — both closed.** FND-1: the lens set is narrowed to coverage + anti-gaming, so every mandated lens carries a proving eval; FR-8's fit now verifies exactly what FR-8 mandates. FND-2: the catch corpus is a recorded checkpoint measurement, not a per-commit blocking gate — the I5-repeating model-driven ship-gate is gone and the reviewer follows its sibling program's advisory→blocking posture.
- **Advisories — dispositioned.** FND-3/5/7/8 applied in the PRD (objective FR-6 fit; UB-6 for the no-automatable-surface feature; enforced NFR-3 cost bound; ABUSE-3 dangling reference removed). FND-4 captured (ABUSE-4 names over-mapping; the per-edge mechanism routed to design Q2). FND-6 deferred to design (recorded as Q6).
- **Regressions in changed spans**: none. The lens removal makes FR-8 self-consistent (mandate two, verify two) with traceability intact (G2/J2/F4, EVT-2 enum aligned); the catch-corpus reframe keeps DOD-2 and DOD-4 as the blocking bars, so no Must FR loses its check; the advisory edits are additive (UB-6, ABUSE-4, tightened FR-6/NFR-3, Q6) with §8 flow-coverage and open-question notes updated to match. docgate `gate`, `verify-generated`, and `lint-structure` are green on the refold.

## Verdict history

| Round | Verdict | Date |
|---|---|---|
| 1 | request-changes | 2026-07-23 |
| 2 | approve-with-advisories | 2026-07-23 |

## Escalation brief

none

## Measurements

- **Rounds**: 2
- **Findings**: 2 blocking · 6 advisory
- **Dispositions**: 7 applied · 0 declined · 0 withheld-as-violation · 1 deferred-to-design (FND-6 → Q6)

## Overlay corrections

*Post-approve entries only — the M2 residual data; frozen rounds and history are never edited.*

none
