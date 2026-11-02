# implement-now — e2e task consumption — reopened design-brainstorm record

| Field | Value |
|---|---|
| **In one line** | The solution shape for the executor's second task-consumption class: how a ready tracker task is recognized as e2e-planned, resolved to exactly one scenario or the harness slot in the feature's approved e2e design doc, briefed, held to the test-not-the-feature mandate and its recorded route, and proven hermetic before it counts green. |
| **Relevant-when** | designing or changing how `/implement-now` recognizes and executes e2e-planned tasks — the task→plan→e2e-design-doc join, the pre-claim basis resolution and its refusals, the worker's e2e brief, route fidelity and the no-behavior-change mandate, hermeticity before green, the manual-human no-spawn class; checking prior design rulings on the executor's read surface or its supersession shape |
| **Basis** | `prd:implement-now` (merged ledger as of amendment 002, PR #760, commit `4bbae8e`, 2026-07-28) |
| **Authority** | @mdarabi (operator) |
| **Date(s)** | 2026-07-28 |
| **Status** | in-session |
| **Tracking** | — |
| **Feeds** | `docs/tech-design/implement-now-e2e-tasks.md` |
| **Prior records** | `brainstorm:implement-now` · `design-brainstorm:implement-now-tech-design` (frozen, ready-for-design) · `design-brainstorm:e2e-verification-tech-design` · `design-brainstorm:planner-driven-model-routing-tech-design-2` |
| **Template version** | design-brainstorm-record/v1 |

## Orientation

| Source | Why loaded |
|---|---|
| `docs/prds/implement-now/effective.md` + `002-consume-e2e-tasks.md` | the merged basis — FR-28..FR-32, NR-8, UB-14..UB-17, EVT-12, DOD-19, and the live FR-1..FR-27 they sit beside |
| `docs/tech-design/implement-now.md` (live) | the predecessor design: Worker + five ports, Dispatcher, frontier-sweep, drive-to-green, DINV-1..11 |
| `docs/tech-design/e2e-verification-artifacts.md` §3.1 | the `e2e-design/v1` class — six sections, header `Basis`/governing-design revisions, freeze-at-approval |
| `docs/tech-design/e2e-verification-skills.md` §3.3, §3.5 | e2e-plan authors an ordinary plan/v1; its `#DINV-5` binds "the executor is unchanged" |
| `docs/tech-design/plan-new-v1.md` §3.1, §3.8 | the plan class, its header, freeze-at-merge, and the tracker projection |
| `docs/tech-design/tech-design-conventions.md` | supersession semantics — whole-document in v1; the `Builds-on` boundary is contradiction |
| `docs/architecture-principles.md` (AP-6, AP-11, AP-13, AP-14, AP-19, AP-21, AP-29) | the principles the candidate shapes are scored against |
| `docs/observability/incidents.md` — I5 | the paid-for hermeticity lesson the basis cites by name |
| `docs/tech-design/index.md` | which live designs' `Serves` touch this basis |
| `internal/executor/**`, `internal/plangate/**`, `internal/e2e/docclass/**`, `cmd/implement-now` | the built code the design must land against |
| `docs/brainstorms/implement-now-tech-design.md` (frozen) | the predecessor record's rulings and `S-1..S-9` seam map — none re-asked |
| `docs/brainstorms/planner-driven-model-routing-tech-design-2.md` (frozen, needs-more-product) | the sibling routing effort that bounced and is now unblocked |

**Overlaps surfaced:**

- `design:implement-now` (live) decides the executor's shape and holds `DINV-1..11`; it is silent on e2e consumption — it does not decide this question, but this effort must either supersede or narrow it (`DQ-1`).
- `design:e2e-verification-skills` (live) decides everything *upstream* of the frontier — coverage, routes, mock strategy, harness choice — and explicitly binds that the executor is unchanged. It is the reason this work exists on the executor side, not a design that covers it. The basis' `NG-5` re-states the boundary.
- `design:e2e-verification-artifacts` (live) decides the `e2e-design/v1` class this design must *read*; it settles the document format, not its consumption.
- No live design serves `prd:implement-now/FR-21..FR-32`; the routing half (amendment 001) is likewise undesigned (`F-1`, `F-11`).

## Design questions

- **DQ-1**: Does the e2e-consumption half land as a whole-document supersession of `docs/tech-design/implement-now.md` — and if so, does that supersession also carry amendment 001's routing seam — or as a new sibling design that `Builds-on` the live one and serves only the 002 rows?
- **DQ-2**: How does the executor establish a ready task's e2e origin and resolve it to exactly one scenario or the harness slot, given that it reads no plan and no repo document today, and that no new required task or charter field may be introduced (`NR-8`) — and what read seam does that open?
- **DQ-3**: How does the resolved basis reach the worker (`FR-29`) as data alongside the charter, without an e2e-specific required charter field?
- **DQ-4**: What mechanism holds an e2e proposal to the test-not-the-feature mandate and its recorded route (`FR-30`, `FR-31`), and where does `UB-16`'s consequence fire relative to the drive-to-green green check?
- **DQ-5**: What establishes an authored test's hermeticity before the task counts succeeded (`FR-32`), and where does it sit relative to the local gate that already single-sources required CI (`incident:I5`, AP-29)?
- **DQ-6**: How do the non-spawning outcomes — the `UB-14` refusals and the `UB-15` manual-human class — and `EVT-12` express in the frontier walk, the classed report, and the two-sink event stream?

## Panel log

*One line per `DQ-n`, whatever happened (DINV-7).*

- **DQ-1**: extends — skipped — the question partitions an existing document class and confirms an existing module boundary; it opens no new interface, port, class, or module, so the seam rule does not fire. The two candidate document shapes are both already named in `tech-design-conventions` (`F-7`) — a panel would re-derive a settled convention.
- **DQ-2**: opens — fired — the executor gains a read path onto repo documents (the frozen plan and the feature's e2e design doc) that it has never had; that is a new inbound port and a new untrusted-input boundary. Divergence ran under four orthogonal constraints — read the repo at run time, read only the tracker, freeze the join at plan time, be told by the invoker — landing `ALT-1..ALT-4`.
- **DQ-3**: opens — fired — the executor↔worker hand-off gains a second carrier beside the charter, and what the worker is handed is a contract boundary. Divergence ran under three orthogonal constraints — an artifact in the workspace, a typed field at the port, inline in the instruction — landing `ALT-5..ALT-7`.
- **DQ-4**: opens — fired — a verification stage between genuinely-green and succeeded is a new boundary on the Worker's terminal path, which `DINV-3` currently ends at the verified artifact. Divergence ran under three orthogonal constraints — a deterministic predicate, a model judgment, an after-the-fact audit — landing `ALT-8..ALT-10`.
- **DQ-5**: opens — fired — hermeticity introduces an admission boundary onto the repo's canonical `code-gate` list and a per-scenario execution contract the argv-only gate config cannot express. Divergence ran under two orthogonal constraints — observe by repeated execution, detect by static inspection — landing `ALT-11..ALT-12`.
- **DQ-6**: extends — skipped — the two new outcomes join the pre-claim refusal stage and the two-sink event surface amendment 001 already established (`EVT-8`, `S-9` in the predecessor record); no new interface, port, class, or module appears, so the seam rule does not fire.

## Decisions

### D-1 — Sibling design document; one executor in code

- **Question**: Does the e2e-consumption half land as a whole-document supersession of `design:implement-now` (optionally also carrying amendment 001's routing seam), or as a sibling design that `Builds-on` the live one and serves only the 002 rows?
- **Answer**: A sibling design document. The *implementation* is not separate — `implement-now` itself is updated in place.
- **Reasoning**: The authority's: the design document may partition, but the executor does not — there is no second implementation to build. Recommendation context adopted: e2e consumption and native model routing are independently revisable questions (RW-5); nothing in the 002 rows makes a binding statement of the live design false (`F-12`), so `Builds-on` narrowing is legal under `F-7`; coupling the halves would block the e2e work behind an undesigned routing half. The basis independently forbids a code split — `NR-8` requires one frontier, one claim path, one dispatch path.
- **Anchor**: 2026-07-28 · turn 2
- **Status**: live

### D-2 — Standing delegation for the remaining questions

- **Question**: `DQ-2`'s panel was put to the authority with `ALT-1` recommended, and the revision-pinning consequence flagged as the next question.
- **Answer**: "go with your recommendations" — the authority delegates the remaining design questions to the author's recommendation rather than ruling each individually.
- **Reasoning**: Theirs, unstated beyond the delegation. Its scope is read as the session's open `DQ-n` set (`DQ-2`..`DQ-6`), not as authority over the gate verdict, which is put separately, nor over any requirement gap — a delegation cannot dispose of a `P-n` (DINV-8).
- **Anchor**: 2026-07-28 · turn 3
- **Status**: live

### D-3 — Origin and basis resolution: run-time marker traversal

- **Question**: How does the executor establish a ready task's e2e origin and resolve it to exactly one scenario or the harness slot, given it reads no plan today and nothing upstream may change (`DQ-2`)?
- **Answer**: `ALT-1` — traverse the existing `plan-task` marker to the frozen plan, read its `Design` header, and where that names an approved, checklist-clean `e2e-design/v1` document, recover the slot by re-running `internal/e2e/plan.Decide`. The document is read at the plan's own freeze revision, and the resolved content fingerprint is recorded in `EVT-12`'s `basis_ref`.
- **Reasoning**: Adopted under D-2's delegation, on the panel's scoring: `ALT-1` is the only shape satisfying `NR-8` without an upstream edit; its two moving parts already exist and are tested (`F-3`, `F-6`); the five `UB-14` refusal classes fall out of `docclass.Check` and the class pin rather than being invented; and `Decide` keeps one join predicate in one place (AP-11) instead of a positional convention re-derived on the consumer side. The freeze-revision read resolves §5.4's "at the revision the plan's recorded reference names" without altering it: the plan is frozen at merge, so the reference resolves to exactly one revision of the referenced document, and the recorded fingerprint makes which bytes were briefed an auditable fact.
- **Anchor**: 2026-07-28 · turn 3 — adopted under the D-2 delegation
- **Status**: live

### D-4 — The basis reaches the worker as an artifact, not as instruction

- **Question**: How does the resolved basis reach the worker as data alongside the charter, without an e2e-specific required charter field (`DQ-3`)?
- **Answer**: `ALT-5` — the executor writes the resolved basis into the task's isolated worktree as a closed versioned artifact before launch; the charter's protocol preamble names its path and states that its contents are data. `port.Task` and `port.LaunchSpec` stay untouched.
- **Reasoning**: Adopted under D-2's delegation. `FR-29` requires the basis be carried "as data and never as instruction", and the assembled brief is definitionally the instruction — so the split has to be physical, not typographic. A workspace artifact behaves identically across all four CLI backends, needs no adapter capability that differs between them, and is diffable evidence for `FR-29`'s per-scenario exclusivity criterion and for the `ABUSE-1` drill.
- **Anchor**: 2026-07-28 · turn 3 — adopted under the D-2 delegation
- **Status**: superseded by D-10 — the artifact's location moved outside the publishing worktree; the data-not-instruction ruling itself stands

### D-5 — Mandate and route fidelity proved by scope diff plus falsification

- **Question**: What holds an e2e proposal to the test-not-the-feature mandate and its recorded route, and where does `UB-16` fire (`DQ-4`)?
- **Answer**: `ALT-8` — two mechanical checks in a stage between genuinely-green and succeeded: a scope diff (changed paths must lie within the task's declared test surface) and a broken-subject falsification run (perturb the subject the scenario names; the authored test must go red). A `mocked` scenario additionally checks its doubles cover only the surfaces the recorded mock strategy names. Either failure withholds the succeeded class under `UB-16`, naming the scenario, the recorded route, and the offending change.
- **Reasoning**: Adopted under D-2's delegation. `UB-16` is a critical row whose fit criterion demands the violation be *named* on seeded fixtures, which requires a deterministic predicate rather than a judgment. Falsification proves route fidelity by observation rather than by scanning for mock constructs, so it is language- and framework-agnostic: a `run-for-real` scenario that stays green against a broken subject has been mocked away, which is exactly the substitution `FR-31` forbids. `DOD-19` already requires this falsification evidence per automatable scenario, so the gate and the release evidence are one artifact (AP-11, AP-14).
- **Anchor**: 2026-07-28 · turn 3 — adopted under the D-2 delegation
- **Status**: live

### D-6 — Hermeticity by perturbation battery, gating admission to the canonical check list

- **Question**: What establishes an authored test's hermeticity before the task counts succeeded, and where does it sit relative to the local gate that already single-sources required CI (`DQ-5`)?
- **Answer**: `ALT-11` — once the authored test first passes, re-run its own selector repeatedly under deliberate perturbation (shuffled order, race detector, concurrent load, perturbed clock and timezone, denied network, dirty ambient repository state). A divergent verdict exits immediately to `UB-17` naming the dependency, before the self-heal loop ever sees the red. A stable verdict admits the test to the repo's canonical `code-gate` list; an unstable one is never wired in. A cheap static scan runs first inside the probe, only to name the dependency when it can.
- **Reasoning**: Adopted under D-2's delegation. This is the direct executable form of `FR-32`'s own definition — a pure function of declared inputs, the same verdict on any machine and any run — and each perturbation is labelled, which is what lets `UB-17` name the dependency. It places the verdict as an *admission test* onto `design:code-gate-mirror`'s single canonical list rather than forking a second list (AP-11), which is exactly where `UB-17`'s "never wire it into required checks" belongs. `incident:I5` argues directly against the cheaper shapes: the mirror could not have caught it, and a static denylist would have missed a wall-clock budget asserted over a git-derived diff.
- **Anchor**: 2026-07-28 · turn 3 — adopted under the D-2 delegation
- **Status**: superseded by D-9 — the battery's position moved from "before the self-heal loop" to "before publication"; the perturbation-battery mechanism itself stands

### D-7 — Two pre-claim terminal classes on the existing report and event stream

- **Question**: How do the `UB-14` refusals, the `UB-15` manual-human outcome, and `EVT-12` express in the frontier walk, the classed report, and the two-sink event stream (`DQ-6`)?
- **Answer**: Two new terminal classes — `basis-refused` and `human-execution-required` — both reached before claim, both spawning no worker and consuming no model tokens, both emitting `EVT-12`, and both reported through the existing classed report and two-sink stream unchanged.
- **Reasoning**: Adopted under D-2's delegation. Amendment 001 already establishes a pre-claim refusal stage (`EVT-8`, `UB-9`..`UB-11`); these two classes join it rather than creating a second gate, so `NR-7`'s zero-work posture and `DINV-8`'s zero-token idle rule extend to them for free. Resolving before claim is also what makes `UB-15`'s "repeated runs report it identically without claim churn" true by construction: a manual-human task is never claimed, so there is no lease to churn.
- **Anchor**: 2026-07-28 · turn 3 — adopted under the D-2 delegation
- **Status**: live

### D-8 — The document read is injected into the frontier walk, not a Worker port

- **Question**: The unmanned design review found that `S-2`'s "consumer-side port on the Worker" cannot hold: resolution runs before claim, and a Worker exists only after one. Port, or injection into the frontier walk?
- **Answer**: Injection — a function value the frontier walk holds, matching the executor's own precedent rather than growing its frozen port surface.
- **Reasoning**: Adopted under D-2's delegation when the authority directed that all review findings be applied with the author's recommendations. Two facts decide it: a Worker port cannot serve a pre-claim read (`review:implement-now-e2e-tasks#FND-7`), and the executor already injects the self-heal loop's fetcher and pusher precisely so hermetic testability does not cost port growth — a single production reader does not earn a sixth port (`#FND-11`).
- **Anchor**: 2026-07-28 · review
- **Status**: live

### D-9 — The hermeticity battery runs before publication, not before the self-heal loop

- **Question**: The review found `D-6`'s placement unenforceable: this repository's canonical check list is suite-scoped, so a published test is already required and there is no "admission to the list" act to withhold — and under the design's own drawn ordering the self-heal loop burns first. Where does the battery run?
- **Answer**: Before any proposal is opened. A non-hermetic verdict publishes nothing, so the check never becomes required and the loop — which acts only on proposals — never sees it.
- **Reasoning**: Adopted under D-2's delegation on the same direction. Publication, not a later wiring step, is the moment a check becomes required on a suite-scoped list, so pre-publication is the only position from which "never wired into required checks" and "zero self-heal iterations" are structurally true rather than merely counted (`#FND-1`, `#FND-2`).
- **Anchor**: 2026-07-28 · review
- **Status**: live

### D-10 — The basis artifact lives outside the publishing worktree

- **Question**: The review found that the workspace commits its whole tree when capturing an agent's uncommitted work, so an executor-written artifact inside the worktree lands in the proposal — and is then rejected by this design's own scope diff as an out-of-scope path. Where does it live?
- **Answer**: Outside the publishing worktree, in an executor-owned per-task location whose path is passed into the launch spec. Falsification damage likewise moves to a disposable copy.
- **Reasoning**: Adopted under D-2's delegation on the same direction. An ignore rule inside the worktree is swept in by the same capture, so the fix has to be structural placement rather than exclusion (`#FND-3`).
- **Anchor**: 2026-07-28 · review
- **Status**: live

## Alternatives

### ALT-1 — Run-time marker traversal, one join predicate

- **Shape**: Read the existing `pyxis:plan-task <feature>/<NNN>/<Tn>` marker from the ready task's body → `docs/plans/<feature>/<NNN>-plan.json` → its `design` header. If that path is an `e2e-design/v1` document, class-pinned, `Status: approved`, and `docclass.Check`-clean, parse it and recover the slot by re-running the same fixed rule that authored the plan (`internal/e2e/plan.Decide`): task ID → exactly one scenario ID, or the harness slot. Anything else resolves `not_e2e`. Opens one consumer-side port — a repo-document reader — used before claim.
- **Trade-offs**: wins — every upstream artifact stays byte-unchanged (`NR-8` satisfied without argument); the reader (`docclass`) and the join rule (`Decide`) both exist and are tested; the five `UB-14` refusal classes fall out of `docclass.Check` and the class pin rather than being invented; the join is one predicate in one place instead of a positional convention re-derived on the consumer side. Costs — the executor gains a repo-read surface it has never had, so document content becomes a new untrusted-input boundary; and the reference records a path but no revision, so the revision must be resolved rather than read off the header (settled in D-3; residue in `G-3`).
- **Principles**: AP-11 honored — `Decide` is the single join predicate, authored and consumed; AP-6 honored — a closed machine marker and a closed document class, no prose; AP-19/AP-21 honored — deterministic, zero inference, resolved before claim so a refused or empty frontier costs nothing; AP-13 honored — a refusal is an inspected artifact defect, never a judgment call.
- **Incidents**: none matched — registry scanned 2026-07-28 for executor, plan-consumption, and document-read entries; `I5` bears on the hermeticity question (`DQ-5`), not on the read path.
- **Verdict**: chosen (D-3) — the only shape that satisfies `NR-8` and `FR-28`'s per-task exclusivity without an upstream edit.
- **From**: DQ-2 · panel

### ALT-2 — Projection-carried basis marker

- **Shape**: plangate stamps a second machine marker into each projected item body at projection time — the design-doc path and revision, the scenario ID or harness slot, and the route. The executor reads only the tracker, exactly as today: no repo read, no new port.
- **Trade-offs**: wins — the smallest possible executor change; the basis travels with the work item like the charter; the frozen revision is captured at projection time for free. Costs — it requires editing plangate and the projection, which `NR-8` forbids in terms; and it copies the approved document's content into a mutable tracker beside the immutable source `prd:implement-now` §5.4 names as the source of truth, so the two can drift with nothing deciding which governs.
- **Principles**: AP-6 honored — a typed marker; AP-21 honored — judged once at projection. Strained: AP-11 — the basis would live in two places with no single predicate deciding which wins; AP-24 — a derived copy standing beside the governing artifact.
- **Incidents**: none matched — registry scanned 2026-07-28.
- **Verdict**: rejected — `NR-8` forbids the plangate and projection edit it needs; recorded because it is the shape that would win if that non-regression row did not exist.
- **From**: DQ-2 · panel

### ALT-3 — Compiled e2e-basis sidecar, frozen at plan time

- **Shape**: a new `e2e-basis/v1` artifact beside the frozen plan, keyed plan-task-id → the resolved basis (doc path and revision, scenario or harness slot, route, mock strategy, coverage rows). Written by e2e-plan or a fold step; the executor reads one closed JSON file and never parses markdown.
- **Trade-offs**: wins — the executor's read surface is one closed schema rather than a document class; the basis freezes with the plan, so a later edit to the design doc cannot silently change what a task must satisfy; no markdown parsing in the executor at all. Costs — a new artifact class and a new writer; e2e-plan or plangate must change, straining `NR-8` and `design:e2e-verification-skills#DINV-5`; and the sidecar becomes a copy with no stated authority, since §5.4 names the approved document itself as the source of truth.
- **Principles**: AP-21 honored at its strongest — judge once, freeze the result; AP-6 honored — closed versioned schema. Strained: AP-11 — a second home for the same content; AP-24 — a derived artifact standing in for a governing one.
- **Incidents**: none matched — registry scanned 2026-07-28.
- **Verdict**: rejected — it buys freeze-at-plan-time at the price of an upstream contract change the basis forbids, and the same freeze is available inside `ALT-1` by pinning the revision the frozen plan's own merge carries.
- **From**: DQ-2 · panel

### ALT-4 — Invoker-supplied plan reference

- **Shape**: `implement-now --plan docs/plans/<feature>/<NNN>` names the originating plan; the executor reads that plan's `design` header and treats the whole sweep as e2e or not.
- **Trade-offs**: wins — trivial to build, no marker traversal, no new port. Costs — the frontier is label-scoped and legitimately mixes plans and features, so a run-level flag cannot classify per task and `NR-8`'s mixed-frontier criterion fails outright; recognition would depend on how the run was invoked rather than on recorded data, which `FR-28` forbids; and a forgotten flag silently downgrades every e2e task to feature work — precisely the misread `FR-28` exists to make impossible.
- **Principles**: strained AP-13 — classification becomes an operator claim rather than a verified property of the artifact; strained AP-6 — an out-of-band argument replaces a recorded reference.
- **Incidents**: none matched — registry scanned 2026-07-28.
- **Verdict**: rejected — cannot deliver the per-task exclusivity `FR-28` states, and breaks on the mixed frontier `NR-8` requires.
- **From**: DQ-2 · panel

### ALT-5 — Basis materialized as a read-only artifact in the worktree

- **Shape**: Before launch the executor writes the resolved basis into the task's isolated worktree as a closed versioned artifact; the charter's protocol preamble names its path and states its contents are data. `port.Task` and `port.LaunchSpec` are untouched; the existing Workspace port already owns the worktree.
- **Trade-offs**: wins — the data/instruction boundary is physical rather than typographic; it behaves identically across all four CLI backends with no adapter capability differences; the artifact is diffable evidence for `FR-29`'s per-scenario exclusivity criterion and for the `ABUSE-1` injection drill. Costs — the artifact sits inside the worktree, so it must be kept out of the proposal diff, and a worker can still ignore a file it was pointed at.
- **Principles**: AP-6 honored — a closed versioned artifact, not free-form text; `NFR-2`/`ABUSE-1` honored — read content is data by construction; AP-13 honored — the brief becomes an inspectable artifact rather than a claim about what was sent.
- **Incidents**: none matched — registry scanned 2026-07-28 for context-delivery and injection entries.
- **Verdict**: chosen.
- **From**: DQ-3 · panel

### ALT-6 — Additive `LaunchSpec` field rendered per adapter

- **Shape**: `LaunchSpec` gains an optional basis field — legal under its declared additive evolution (`F-10`) — and each of the four adapters renders it into its own CLI's context mechanism.
- **Trade-offs**: wins — typed at the port, no file hygiene, no worktree pollution. Costs — four adapters each grow a rendering path and they differ in what they support; the rendering is invisible at inspection time, so `FR-29`'s per-scenario exclusivity becomes hard to prove; and most CLIs would concatenate it into the prompt anyway, collapsing back into instruction.
- **Principles**: AP-6 honored — typed field. Strained: AP-11 — four renderings of one contract; `NFR-2` — the data/instruction split becomes adapter-dependent rather than structural.
- **Incidents**: none matched — registry scanned 2026-07-28.
- **Verdict**: rejected — it makes the one property `FR-29` cares about depend on which backend happened to run.
- **From**: DQ-3 · panel

### ALT-7 — Inline in the charter brief

- **Shape**: append the resolved basis to the assembled Brief string the runtime is handed.
- **Trade-offs**: wins — zero new machinery, works everywhere immediately. Costs — `FR-29` requires the basis be carried as data and never as instruction, and the Brief is definitionally the instruction, so the `ABUSE-1` drill would run over text the runtime is told to obey.
- **Principles**: violates `NFR-2`'s boundary; strained AP-6 — contractual data dissolved into prose.
- **Incidents**: none matched — registry scanned 2026-07-28.
- **Verdict**: rejected — contradicts `FR-29` in terms.
- **From**: DQ-3 · panel

### ALT-8 — Scope diff plus broken-subject falsification

- **Shape**: two mechanical checks in a stage between genuinely-green and succeeded. (1) Scope diff — the proposal's changed paths must lie within the task's declared test surface; a change to behavior under test outside the charter's stated scope fails `UB-16` naming the path. (2) Falsification — perturb the subject the scenario names and require the authored test to go red. A `mocked` scenario additionally checks its doubles cover only the surfaces its recorded mock strategy names.
- **Trade-offs**: wins — both checks are mechanical and name the offending change, which `UB-16`'s fit criterion demands; falsification proves route fidelity by observation rather than by scanning for mock constructs, so it is language- and framework-agnostic; `DOD-19` already requires this exact falsification evidence, so the gate and the release evidence are one artifact. Costs — "perturb the subject" needs a per-scenario definition, and each scenario pays a second test execution.
- **Principles**: AP-14 honored strongly — acceptance is observed by execution, never asserted; AP-13 honored — the verdict is an artifact; AP-11 honored — one falsification predicate serves both the admission gate and `DOD-19`'s evidence.
- **Incidents**: none matched — registry scanned 2026-07-28 for false-green and verification-theater entries.
- **Verdict**: chosen.
- **From**: DQ-4 · panel

### ALT-9 — Reviewer-lens judgment

- **Shape**: hand the proposal and its basis to the impl-review spec-fidelity lens and let a model judge whether behavior under test changed or the route was substituted.
- **Trade-offs**: wins — catches subtle bending a path diff cannot see; reuses a reviewer that already exists. Costs — `UB-16` is a critical row whose fit criterion requires the violation be named deterministically on seeded fixtures, and a model verdict is neither deterministic nor free; it also spends inference on every e2e proposal.
- **Principles**: strained AP-19 — inference where a deterministic predicate suffices; strained AP-13 — a judgment substituting for a verified artifact; strained AP-10 — the rule is codifiable and so should be codified.
- **Incidents**: none matched — registry scanned 2026-07-28.
- **Verdict**: rejected as the primary mechanism — retained as the complement the existing impl-review seam already supplies.
- **From**: DQ-4 · panel

### ALT-10 — Trust the green checks, audit afterwards

- **Shape**: no new pre-succeeded stage; a periodic audit compares landed e2e proposals against their bases and reports violations.
- **Trade-offs**: wins — nothing to build now. Costs — `UB-16` requires the succeeded class be *withheld*, which an after-the-fact audit cannot do; and the amendment states the reason the row exists is that `FR-7`'s green-checks test cannot see the violation at all.
- **Principles**: strained AP-29 — an audit records, it does not gate; strained AP-13.
- **Incidents**: none matched — registry scanned 2026-07-28.
- **Verdict**: rejected — contradicts `UB-16` in terms.
- **From**: DQ-4 · panel

### ALT-11 — Perturbation battery as admission test for the canonical check list

- **Shape**: once the authored test first passes and before the task may count succeeded, re-run that test's own selector repeatedly under deliberate perturbation — shuffled order, race detector, concurrent load, perturbed clock and timezone, denied network, dirty ambient repository state. A divergent verdict exits immediately to `UB-17` naming the dependency, before the self-heal loop sees the red. A stable verdict admits the test to the repo's canonical `code-gate` list; an unstable one is never wired in.
- **Trade-offs**: wins — it is the direct executable form of `FR-32`'s own definition, and labelled perturbations are what let `UB-17` name the dependency; the local probe plus the required-CI run are the "differently-loaded machines" pair `incident:I5` actually burned on; and hermeticity becomes an admission test onto one canonical list rather than a second list. Costs — several extra test executions per scenario, and the per-scenario selector must be expressible, which today's argv-only gate config cannot carry (`G-1`).
- **Principles**: AP-29 honored — a gate asserts correctness, so a check that cannot hold its verdict is refused entry; AP-14 honored — hermeticity is observed, not asserted; AP-11 honored — one canonical gate list, entry-controlled. Strained: AP-19 — the probe spends real compute, though zero inference.
- **Incidents**: `I5` matched — a wall-clock budget passed on a fast dev machine and stranded CI red, and the record states a command-level mirror would not have caught it; that is precisely why the mirror is not the mechanism here.
- **Verdict**: chosen.
- **From**: DQ-5 · panel

### ALT-12 — Static scan for non-hermetic constructs

- **Shape**: scan the authored test for known non-hermetic calls — clock reads, network dials, environment and ambient-git access, unordered iteration — and refuse on a hit.
- **Trade-offs**: wins — cheap, instant, and names the dependency precisely when it fires. Costs — it is a denylist over an open set, so it passes everything it has not been taught; `I5`'s own failure was a wall-clock budget asserted over a git-derived diff, which a naive scan would miss; and it cannot establish verdict stability, which `FR-32`'s fit criterion states in terms of repeated runs.
- **Principles**: strained AP-13 and AP-14 — a static claim standing in for an observed property.
- **Incidents**: `I5` matched — and the incident argues against this shape as the sole mechanism, since the failing check was syntactically ordinary.
- **Verdict**: rejected as the sole mechanism — retained as a cheap first pass inside `ALT-11`, used only to name the dependency when it can.
- **From**: DQ-5 · panel

## Seam map

- **S-1**: e2e origin and basis resolution — **opens** — behind it: the `plan-task` marker traversal, the frozen-plan read at its freeze revision, the `e2e-design/v1` parse through `docclass`, and the `internal/e2e/plan.Decide` join to exactly one scenario or the harness slot — feeds: Scope "e2e origin resolution, before claim", Serves `FR-28` · `UB-14` · `EVT-12`
- **S-2**: repo-document read — **opens** — behind it: a read-only document source over the executor's own checkout, resolving a path at a named revision; the executor's first inbound document surface and therefore a new untrusted-input boundary. *Injected into the frontier walk, not a Worker port (revised at review — D-8).* — feeds: Scope "the executor's read surface", Serves `FR-28` · `NR-8` · `NFR-2`
- **S-3**: the worker basis artifact — **opens** — behind it: the resolved basis written as a closed versioned artifact *outside the publishing worktree* (revised at review — D-10), its path named by the charter protocol and its content read as data — feeds: Scope "the e2e brief", Serves `FR-29` · `NFR-2` · `ABUSE-1`
- **S-4**: mandate and route verification stage — **opens** — behind it: the scope diff and the broken-subject falsification run, sitting between genuinely-green and succeeded — feeds: Scope "e2e proposal admission", Serves `FR-30` · `FR-31` · `UB-16` · `DOD-19`
- **S-5**: hermeticity probe and publication gate — **opens** — behind it: the labelled perturbation battery and its verdict controlling whether a proposal is opened at all *(revised at review — D-9: publication, not list admission, is the moment a check becomes required on a suite-scoped list)* — feeds: Scope "hermeticity before publication", Serves `FR-32` · `UB-17`
- **S-6**: pre-claim terminal classes and evidence — **extends** — behind it: two zero-token classes on the existing frontier walk plus `EVT-12` on the existing two-sink event stream — feeds: Scope "the classed report", Serves `UB-14` · `UB-15` · `EVT-12`

## Facts

- **F-1**: No live technical design serves `prd:implement-now/FR-21..FR-32`; `implement-now.md` serves `FR-1..FR-20` + `NR-1..NR-4` and carries a `stale-serves` flag — source: `docs/tech-design/index.md`
- **F-2**: The executor takes no plan reference. `implement-now --repo-slug owner/name` sweeps the tracker by label; `Task.Brief` is the issue body verbatim; nothing under `internal/executor/` reads a plan file, a plan-task marker, or any repo document — source: `cmd/implement-now/run.go`, `internal/executor/ghtracker/tracker.go:100`
- **F-3**: Every projected task body already carries `<!-- pyxis:plan-task <feature>/<NNN>/<Tn> -->`, and each epic `<!-- pyxis:plan-epic <feature>/<NNN> -->` — plangate's idempotency key, written today, read by nothing on the executor side — source: `internal/plangate/dialect/harbormaster.go:118`, `internal/plangate/project/project.go:120`
- **F-4**: A frozen plan's header carries `feature`, `prd`, `design` (one path), `plan_version`, `dialect`, `status` — `design` is the governing-document field every plan already requires — source: `docs/plans/e2e-verification/001-plan.json`, `docs/tech-design/plan-new-v1.md` §3.1
- **F-5**: e2e-plan authors an ordinary plan-new `plan/v1` and hands it to the unchanged pipeline; `design:e2e-verification-skills#DINV-5` binds that the executor and plan-new are unchanged by the authoring line — source: `docs/tech-design/e2e-verification-skills.md` §3.3, §3.5
- **F-6**: `internal/e2e/docclass` already parses an `e2e-design/v1` document into `Doc{Header, Sections, Scenarios[], Coverage[], Routes[]}` and `Check` reports every well-formedness violation — the reader `FR-28`/`UB-14` need exists and is tested — source: `internal/e2e/docclass/docclass.go`, `check.go`
- **F-7**: Supersession is whole-document in v1; `Builds-on` narrowing is legal only while both documents can be simultaneously true — the boundary is contradiction, and any binding statement made false forces supersession — source: `docs/tech-design/tech-design-conventions.md`
- **F-8**: `port.AgentRuntime.TierMap(rung int)` and `port.Task.Rung` are live binding statements of `design:implement-now`; amendment 001's dependency row requires a superseding design that replaces them — source: `internal/executor/port/agentruntime.go`, `internal/executor/port/task.go`, `prd:implement-now` §6.2
- **F-9**: `incident:I5` — the first implement-now sweep drove green locally and stranded CI red on a check asserting a wall-clock budget; the lesson graduated to `AP-29` (a gate asserts correctness; a measurement only records), and the record states a command-level mirror would not have caught it — source: `docs/observability/incidents.md`
- **F-10**: `port.LaunchSpec` is declared to evolve additively under AP-6 — new optional fields may be added, existing ones are stable — source: `internal/executor/port/agentruntime.go`
- **F-11**: The routing PRD's amendment 002 (`universal-review-floor`) landed, resolving the `P-1` gap that gated `design-brainstorm:planner-driven-model-routing-tech-design-2` as `needs-more-product`; that effort is unblocked but undesigned — source: `docs/prds/planner-driven-model-routing/002-universal-review-floor.md`
- **F-12**: `design:implement-now#DINV-3` makes a verified artifact *necessary* for `succeeded`, not sufficient — so `FR-30`/`FR-32`'s extra conditions narrow it rather than contradict it — source: `docs/tech-design/implement-now.md` §3.8

## Fog

- **G-1** (non-blocking — blocks: a design `Q-n`): How a *single scenario's* test is named and run. The executor's gate config is argv-only (`GateCheck{Name, Argv}`), which carries no per-test selector — yet both the falsification run (`S-4`) and the hermeticity probe (`S-5`) must execute one scenario's test alone. Whether that lands as a per-repo e2e config block, a naming convention the harness task establishes, or an extension of the `code-gate` list is a mechanism behind settled rulings; the direction stands under any of them.
- **G-2** (non-blocking — blocks: a design `Q-n`): What "perturb the subject" means per scenario for the falsification run — which artifact is broken, how the perturbation is chosen so it genuinely exercises the scenario's own subject rather than something incidental, and how it is bounded so it never escapes the task's worktree.
- **G-3** (non-blocking — blocks: a design `Q-n`): How the plan's freeze revision is determined in practice — the commit that added the frozen plan JSON is the obvious answer, but the behavior on a repository where that history is unavailable (a shallow clone, an export) needs a stated fail-closed path.

## Out of scope

- **O-1**: A second executor — a separate binary, package tree, dispatch route, or projection path for e2e work — is refused; e2e consumption is an in-place extension of the one executor, whatever the document partition (from D-1, reinforced by `prd:implement-now/NR-8`)
- **O-2**: Amendment 001's native-routing seam (`FR-21..FR-27`) — not served by this design; it remains the undesigned obligation of a later implement-now supersession (from D-1)
- **O-3**: Any upstream edit to plan-new, plangate, the projection, or e2e-plan — refused; the origin reference rides a plan header field that already exists and already folds (from D-3, `ALT-2`/`ALT-3` rejected)
- **O-4**: Model judgment as the primary mandate or route check — refused; the impl-review seam stays a complement, never the gate that withholds the succeeded class (from D-5, `ALT-9` rejected)
- **O-5**: A static scan as the sole hermeticity mechanism — refused; it survives only as a naming aid inside the perturbation probe (from D-6, `ALT-12` rejected)
- **O-6**: A second gate list for authored e2e checks — refused; hermeticity controls *admission* to the repo's one canonical `code-gate` list rather than standing up a parallel one (from D-6)

## Candidate glossary terms

- **T-1**: e2e basis — the resolved, per-task slice of a feature's approved e2e design doc: exactly one scenario (its flow, recorded route, mock strategy, setup/teardown lifecycle, harness selection, and the coverage rows it proves) or the harness slot, carried into one attempt as data · avoid: "e2e context", "test spec", "scenario brief"
- **T-2**: falsification run — deliberately breaking the subject a scenario names and requiring the authored test to go red, as the admission proof that the test exercises the real subject · avoid: "mutation testing", which is a coverage technique over a whole codebase rather than a per-scenario admission check
- **T-3**: hermeticity probe — the labelled perturbation battery that establishes an authored test's verdict stability before it may join the repo's canonical check list · avoid: "flake detection", which names a post-hoc measurement; this one gates

## Parked product gaps

- **P-1**: `FR-31`'s mocked-route half may not be satisfiable from what the pipeline records. The row requires that "each double is checked against the real dependency's observed contract, and a stub encoding a wrong guess about that contract fails rather than going green" — but the approved e2e document records a mock *strategy* as prose with no per-surface grammar, and nothing in the pipeline captures the real dependency's contract to compare a double against. Surfaced by the unmanned design review (`review:implement-now-e2e-tasks#FND-4`), which all four lenses raised independently. — anchors: `prd:implement-now/FR-31`, `prd:implement-now/UB-16`, `prd:e2e-verification/FR-1` — disposition: **proceed-anyway per D-2** — the design carries the obligation to the worker in its basis, declares the executor-side check absent rather than asserting it, and parks the mechanism as its own `Q4` with a named risk. If `Q4` concludes the check is unmechanizable from recorded data, the honest resolution is a `/prd-amend` against `FR-31`'s fit criterion, not a design workaround.

*The other contract tension examined — §5.4's "at the revision the plan's recorded reference names" against a header field that records only a path — resolves inside the requirement rather than around it (D-3), and so is a mechanism, not a gap.*

## Gate

| Field | Value |
|---|---|
| **Verdict** | ready-for-design |
| **Date** | 2026-07-28 |
| **Authority** | @mdarabi (operator) |
| **Covering design** | — |
| **Coverage check** | pass — core four covered, zero blocking fog. §2 Decision: `D-3` (live, from the fired `DQ-2` panel), with `D-1`, `D-4`..`D-7`. §4 Alternatives: `ALT-1`..`ALT-12`, every entry verdicted, ≥ 2 per fired panel (4 / 3 / 3 / 2). Serves + Scope: `S-1`..`S-6` against the `prd:implement-now` basis. Boundaries: `O-1`..`O-6`. Fog: `G-1`..`G-3`, all non-blocking, each riding the authored design as its own `Q-n`. |
| **Amend route** | — |

*Verdict anchor: 2026-07-28 · turn 4 — the authority answered the gate by invoking `/tech-design-new`, the handoff path only a `ready-for-design` record admits, on a gate put with `ready-for-design` recommended.*
