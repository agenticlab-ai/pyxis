# Review: prd:planner-driven-model-routing/000 — review report

| Field | Value |
|---|---|
| **In one line** | approve-with-advisories after a scope-locked round 2 closed nine blocking routing-contract gaps and one applied fit-criterion advisory with no changed-span regressions. |
| **Relevant-when** | auditing or consuming the review of `prd:planner-driven-model-routing/000`; asking why the routing contract was bounced or approved; tuning prd-review calibration against factory-control and model-routing requirements |
| **Subject** | `prd:planner-driven-model-routing/000` |
| **Moment** | new-base |
| **Reviewer** | prd-review · guide 2026-07-26 |
| **Date(s)** | 2026-07-26 (rounds 1–2) |
| **Verdict** | approve-with-advisories |
| **Tracking** | — |
| **Template version** | review-report/v1 |

## Round 1

### FND-1 — the frozen routing vocabularies were absent from the normative contract

- **Severity**: blocking (CAL-2)
- **Lens**: upstream-alignment
- **Evidence**: FR-1 required six fields only “using their closed value sets,” and FR-3 named four profile components without their values — vs — `brainstorm:planner-driven-model-routing#D-3`, `#D-4`, and `#D-7`, which freeze every assessment and profile value.
- **Acceptance condition**: Normatively enumerate and define every assessment-axis and execution-profile value; make FR-1/FR-3, UB-1, and EVAL-1 reject unknown values and cover every accepted one.
- **Disposition**: applied — §5.1 now carries two normative vocabulary tables; FR-1/FR-3 and EVAL-1 bind to them; UB-1 already rejects missing/unknown values.

### FND-2 — the conformance target could pass by shrinking its own harness/profile domain

- **Severity**: blocking (CAL-2)
- **Lens**: upstream-alignment
- **Evidence**: FR-11 applied only to combinations the policy “declares supported,” while M3 and DOD-3 measured only that declared set — vs — `brainstorm:planner-driven-model-routing#D-14/#D-15` and `prd:implement-now/FR-6`, which bind all baseline runtime adapters and both roles.
- **Acceptance condition**: Define the required harness×role×derivable-profile domain independently of the mapping declaration; make omission or relabeling a conformance failure while preserving FR-10's explicit unsupported outcome.
- **Disposition**: applied — §5.1 fixes the baseline harnesses, both roles, and every derivable profile as the required domain; FR-11, M3, EVT-4, DOD-3, EVAL-2, and release text consume that domain.

### FND-3 — M1 had no observable denominator

- **Severity**: blocking (CAL-1)
- **Lens**: coherence
- **Evidence**: M1 targeted 100% complete model-backed plans but fed only from EVT-1, which fired after a task passed validation; rejected or incomplete tasks vanished — vs — prd-template/v2 §5.7 and `brainstorm:planner-driven-model-routing#D-14`'s completeness requirement.
- **Acceptance condition**: Define the denominator as every non-deterministic plan/v2 candidate and instrument pass/fail before validation so invalid contracts lower M1.
- **Disposition**: applied — M1 is now valid contracts divided by every task presented for freeze that is not explicitly deterministic; EVT-6 emits pass/fail and violations for the full denominator; §8 cites the formula.

### FND-4 — pre-claim routing failures required an outcome event that could never fire

- **Severity**: blocking (CAL-1)
- **Lens**: coherence
- **Evidence**: FR-16 required failed attempts to join EVT-3 and EVT-5, but EVT-3 ends before claim/invocation while EVT-5 fires only after implementation or review completes; M5 also disagreed with EVT-3's feed — vs — `brainstorm:planner-driven-model-routing#D-10/#D-14`.
- **Acceptance condition**: Give every route an achievable terminal chain: EVT-3 is terminal for pre-claim failure; resolved EVT-2 attempts join exactly one EVT-5; align FR-16, M5, events, and §8.
- **Disposition**: applied — FR-16 now ends every routing attempt at exactly one EVT-2 or EVT-3; FR-20 owns EVT-2→EVT-5; M5 and §8 state both legal chains; EVT-3's properties no longer imply a native profile.

### FND-5 — the kill switch could select a “strongest” profile below a frozen minimum

- **Severity**: blocking (CAL-3)
- **Lens**: contract-quality
- **Evidence**: FR-18 selected the strongest approved profile but tested only that selection and FR-13, leaving capability, deliberation, and context unconstrained — vs — `brainstorm:planner-driven-model-routing#D-10/#D-17`, which preserve fail-closed, no-downgrade behavior under fallback.
- **Acceptance condition**: Make fallback a cost preference only: its profile must dominate every semantic minimum, otherwise FR-10 fires before claim; drill every component and an unsatisfiable case.
- **Disposition**: applied — FR-18 now requires four-component dominance or FR-10; DOD-7 and §6.5 require the unsatisfiable-fallback drill and preserve harness/review semantics.

### FND-6 — F4 claimed complete flow coverage without a non-regression contract

- **Severity**: blocking (CAL-1)
- **Lens**: coherence
- **Evidence**: F4 is the operator audit/override flow, but §8 cited only FR-12/FR-16 and UB-4/UB-7 while claiming functional, failure, and non-regression coverage — vs — prd-template/v2 §8 and `brainstorm:planner-driven-model-routing#D-13`.
- **Acceptance condition**: Add or cite a non-regression contract protecting the existing authorization/closed-policy behavior for repository overrides and trace F4 to it.
- **Disposition**: applied — NR-7 protects the existing config-policy authorization and closed-parse guarantees with executable fit criteria; F4 and §8 now trace to NR-7.

### FND-7 — evidence-backed planner judgment was stated but not required on production plans

- **Severity**: blocking (CAL-4)
- **Lens**: completeness-critic
- **Evidence**: J1 and EVAL-1 required evidence, but FR-1 required only labels and FR-6 required a citation only for an assurance raise — vs — `brainstorm:planner-driven-model-routing#D-13`, which makes the frozen demand evidence-backed, and §8's claim that J1 was covered.
- **Acceptance condition**: Require at least one resolvable assessment-evidence citation on every model-backed task, preserve it across boundaries, and reject missing/unresolved citations before freeze.
- **Disposition**: applied — FR-19 adds the production requirement and fit; FR-7 preserves citations across every downstream boundary; EVT-1 and the data contract carry them; EVAL-1 and §8 include their failure and coverage paths.

### FND-8 — routing/outcome learning fuel could remain outside git

- **Severity**: blocking (CAL-2)
- **Lens**: upstream-alignment
- **Evidence**: G3 promised successor evidence, but §5.4 left decisions in an “execution journal/event stream” and outcomes under an unspecified evidence policy — vs — AP-28's immutable git-event-record rule and `brainstorm:planner-driven-model-routing#D-14`'s calibration corpus.
- **Acceptance condition**: Retain the bounded terminal decision/outcome evidence used by M5 as immutable, append-only git event records without prompt or code payloads.
- **Disposition**: applied — FR-21 makes terminal evidence immutable git records; §5.4 defines their source, retention, and integrity; DOD-8 proves exact, promoted, unsupported, and joined-review examples.

### FND-9 — direct-production rollback did not disposition work already touched by a defective route

- **Severity**: blocking (CAL-4)
- **Lens**: completeness-critic
- **Evidence**: §3.3 and §6.5 engaged fallback and stopped new claims after a production qualification defect but said nothing about affected in-flight, unaccepted, accepted, or merged evidence — vs — `brainstorm:planner-driven-model-routing#D-17`'s fail-closed safeguard and §8's claimed critical safety plan.
- **Acceptance condition**: Invalidate and rerun all affected open evidence before acceptance; for already accepted/merged artifacts, preserve state and emit an exact operator incident/remediation brief rather than silently retaining or destructively rolling back the claim.
- **Disposition**: applied — UB-9 and UB-10 define both branches; EVT-7 records them; §3.3/§6.5 invoke them; DOD-9 requires a post-cutover defect-disposition drill.

### FND-10 — “bounded” routing evaluation had no measurable bound

- **Severity**: advisory (CAL-3)
- **Lens**: contract-quality
- **Evidence**: NFR-6 claimed “local, bounded policy evaluation,” but its fit named no latency, operation, memory, or input-size bound — vs — prd-template/v2 Appendix A's pass/fail fit-criterion rule.
- **Acceptance condition**: Define a measurable bound, or remove the unsupported claim and retain only mechanically verifiable zero-inference/no-network behavior.
- **Disposition**: applied — NFR-6 now requires zero routing inference and zero outbound calls before the selected worker invocation; inspection and offline tests can decide both.

## Round 2

*Scope-locked to the changed spans named by FND-1..FND-10; the unchanged remainder was out of scope by protocol.*

- **FND-1** — closed: both normative tables define every frozen assessment and profile value; FR-1/FR-3 and EVAL-1 consume them. No changed-span regression.
- **FND-2** — closed: the required domain is fixed independently of policy declarations; FR-11/M3/EVT-4/DOD-3/EVAL-2 make omission fail while preserving explicit unsupported results. No changed-span regression.
- **FND-3** — closed: M1 has the full non-deterministic freeze-candidate denominator and EVT-6 observes pass/fail before validation outcome. No changed-span regression.
- **FND-4** — closed: every route terminates at EVT-3 or EVT-2→EVT-5; FR-16/FR-20, M5, event properties, and §8 agree. No changed-span regression.
- **FND-5** — closed: fallback must dominate all four semantic components or invoke FR-10; DOD-7 includes an unsatisfiable drill. No changed-span regression.
- **FND-6** — closed: NR-7 protects config-policy authorization and closed parsing and is traced through F4 and §8. No changed-span regression.
- **FND-7** — closed: FR-19 requires resolvable evidence and FR-7 preserves it through every named boundary; EVT-1, data, EVAL-1, and §8 agree. No changed-span regression.
- **FND-8** — closed: FR-21 and §5.4 make terminal improvement evidence immutable git event records; DOD-8 proves committed examples. No changed-span regression.
- **FND-9** — closed: UB-9 invalidates affected open evidence and UB-10 briefs already accepted artifacts; EVT-7, rollout, guardrails, and DOD-9 compose. No changed-span regression.
- **FND-10** — closed: NFR-6 now states testable zero-inference and zero-network behavior and no unmeasured bound. No changed-span regression.

No new finding on unchanged content; no violation-withheld candidate arose.

## Verdict history

| Round | Verdict | Date |
|---|---|---|
| 1 | request-changes | 2026-07-26 |
| 2 | approve-with-advisories | 2026-07-26 |

## Escalation brief

none

## Measurements

- **Rounds**: 2
- **Findings**: 9 blocking · 1 advisory
- **Dispositions**: 10 applied · 0 declined · 0 withheld-as-violation

## Overlay corrections

none
