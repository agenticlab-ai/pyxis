# E2E verification — design-brainstorm record

| Field | Value |
|---|---|
| **In one line** | Stage-3 solution-shape for the e2e authoring line — the document class of the three e2e artifacts, how the three skills reuse the reviewer program / plan-new / doc-review, and e2e-design's coverage-derivation and harness-selection internals. |
| **Relevant-when** | designing or changing the e2e authoring line (e2e-design · e2e-review · e2e-plan) or its artifacts; deciding the e2e design-doc / review-report / plan document class and home; checking prior design rulings or refused shapes about e2e coverage-map derivation, harness selection, governing-design selection, or the reviewer/planner reuse boundary |
| **Basis** | `prd:e2e-verification` (merged ledger — 000-base, prd-review approve-with-advisories, frozen on main #610) |
| **Authority** | @mdarabi (operator) |
| **Date(s)** | 2026-07-23 · 2026-07-24 |
| **Status** | gated: ready-for-design — 2026-07-24, operator @mdarabi *(after gating, the record changes only through the introducing PR's review loop — rulings added or superseded with review anchors — and freezes entirely at merge)* |
| **Tracking** | — |
| **Feeds** | `docs/tech-design/e2e-verification-artifacts.md` · `docs/tech-design/e2e-verification-skills.md` |
| **Prior records** | `brainstorm:e2e-verification` (stage-1, gated ready-for-prd) |
| **Template version** | design-brainstorm-record/v1 |

## Orientation

| Source | Why loaded |
|---|---|
| docs/prds/e2e-verification/effective.md | the basis — merged 000-base, prd-review approve-with-advisories; the FR/UB/NFR/ABUSE contract this design must serve |
| docs/reviews/e2e-verification-000.md | basis provenance — the lens set narrowed to coverage+anti-gaming, the catch corpus reframed to a recorded measurement (I5), FND-4 over-mapping and FND-6 governing-design routed to design |
| docs/brainstorms/e2e-verification.md | the stage-1 record — every `D/F/P/O/T-n` anchor the basis cites (identity, scope, placement, the six-part doc, the runnability rule, the reviewer mirror) |
| docs/tech-design/index.md | the design index — **no live design Serves `prd:e2e-verification/*`**: open ground, no `covered` candidate |
| docs/tech-design/design-brainstorm-record-model.md | the closest precedent for adding a new lightweight authored document class (shared/flat home, freeze-at-merge, ungoverned behind a shared graduation trigger) |
| docs/wiki/glossary.md | the e2e coverage-map term (`#T-1`), the reviewer-program vocabulary (closable finding, scope lock, ladder), the seam definition that drives the panel rule |
| docs/architecture-principles.md · docs/observability/incidents.md | AP/RW registry + I1–I5 — the panel's principle citations and incident matching |
| docs/tech-design/{impl-review-v1,prd-review-artifacts,plan-new-v1,implement-now}.md | routing rows only (index + glossary) — the reviewer-program / planner / executor shapes the three skills reuse; full bodies loaded when their reuse DQ is grilled |

**Overlaps surfaced:**

- **No live technical design serves `prd:e2e-verification`** — the solution space is open ground; nothing to gate `covered`.
- **The reviewer program** (prd-review · tech-design-review · impl-review + their artifact designs) is the proven shape `e2e-review` adopts (basis FR-7) — a reuse target, not a competing decision.
- **plan-new-v1 + implement-now** already provide dependency-ordered pickup (blocked-by edges + the ready frontier) — `e2e-plan` (FR-13) need only declare the edge (`brainstorm:e2e-verification#F-9`).
- **design-brainstorm-record-model** is the precedent for how a new lightweight authored class was added — directly informs DQ-1.
- **implementation-verification (stage-8)** is the drawn boundary — per-diff mechanical ladder vs feature-level behavioral e2e (basis NG-5).

## Design questions

- **DQ-1**: What document class do the three e2e artifacts belong to — reuse existing kinds, a new lightweight authored class, or new governed docgate kinds? *(Focus: the e2e design doc; the basis near-settles the review report and plan as reuses.)*
- **DQ-2**: How is the feature's design partitioned — one technical design, or split (e.g. an artifacts/data-model design + a skills/behavior design, as argus and the reviewer program split)?
- **DQ-3**: How does `e2e-review` stand up — a fourth instance of the reviewer program reusing its context pack + review-report + blind-lens/adversarial-screen/convergence machinery, or a parallel build? And where do the two lenses (coverage · anti-gaming) + the over-mapping per-edge check (basis ABUSE-4) live?
- **DQ-4**: How does `e2e-plan` produce its plan — reuse plangate's fold/projection and the task/charter schema directly, or a thinner decomposer emitting plan-new-shaped tasks? What makes it freeze and project idempotently (FR-15)?
- **DQ-5**: `e2e-design`'s internals — how it derives the coverage map, selects the governing design under multiple/superseded designs (basis Q6), picks an app-shape-aware harness (FR-6), and bounds its code-context read (NFR-3).
- **DQ-6**: The unmanned reviewer's advisory→blocking adoption ramp / checkpoint (basis Q3) — mirror the existing ten-review checkpoint, or otherwise.

## Panel log

- **DQ-1**: opens — fired — a new-or-reused document class (the e2e artifacts' home/kind/mutability) is a seam the design opens; parallel alternatives warranted (basis Q1/`#P-4`).
- **DQ-3**: extends — skipped — e2e-review is its own separate skill (the fourth reviewer) but reuses the established reviewer pattern and its shared building blocks (context-pack + the `review-report/v1` format from D-2), opening no new seam; the operator affirmed keeping the skill separate (D-3). A parallel-reviewer alternative was neither warranted nor forced.
- **DQ-4**: extends — skipped — e2e-plan is its own separate skill reusing plan-new's projection dialect + the executor's existing dependency-ordered frontier under the hood (one invocation, no separate planner call), opening no new seam; the operator affirmed the separate-skill + single-invocation shape (D-5).
- **DQ-5**: extends — skipped — e2e-design's internals are skill-internal judgment (coverage from PRD + design + code; harness/mock choice as authored content per NFR-1, not skill code) plus reuse of existing mechanisms (the design index's liveness for governing-design selection; a bounded context read); no new interface, port, or class. No seam.
- **DQ-2**: no seam (documentation-partition, not a system interface) — skipped — one-vs-two design docs is a packaging choice that opens no system interface/port/class; operator ruled two designs (D-11).
- **DQ-6**: extends — skipped — the reviewer's blocking/advisory findings + resolve-or-reject convergence come from the reused reviewer program and the basis (FR-10 / UB-5); the reviewer-level advisory→blocking authority mirrors the existing program's checkpoint and only engages once a downstream enforcement surface exists — no new seam. Operator ruled finding-level blocking is enforced in v1 (D-12).

## Decisions

### D-1 — the e2e design doc's document class

- **Question**: what document class is the e2e design doc — reuse the technical-design class (ALT-1), a new lightweight authored class (ALT-2), or a new governed docgate kind (ALT-3)?
- **Answer**: a new lightweight authored class (ALT-2) — its own light template and per-feature home, frozen at approval/merge, not docgate-folded and not in the design index; docgate governance can graduate later behind a shared trigger.
- **Reasoning**: operator adopted the recommendation — the doc is per-feature, high-volume, model-authored, and freeze-at-merge; that is the design-brainstorm/review-report class shape, whereas the technical-design class mis-fits the semantics and pollutes the design index (ALT-1) and a governed kind pays for amendment machinery a non-ledger artifact never uses (ALT-3).
- **Anchor**: 2026-07-23 · DQ-1 panel ruling
- **Status**: live

### D-2 — the review report and plan reuse existing shapes

- **Question**: do the other two artifacts reuse existing shapes — the review report as `review-report/v1`, the plan as plan-new's projection dialect + the ordinary task/charter schema?
- **Answer**: yes — the e2e review report is a `review-report/v1` instance (homed with the reviewer reports); the e2e plan reuses plan-new's frozen-plan + idempotent tracker-projection dialect and the ordinary task/charter schema, adding no new task type.
- **Reasoning**: operator confirmed — the basis near-settles both (FR-7; FR-14/FR-15); reuse keeps the two artifacts on proven, already-governed shapes and lets implement-now consume the tasks unchanged.
- **Anchor**: 2026-07-23 · DQ-1 panel ruling
- **Status**: superseded by D-13 — at design review, `review-report/v1` was found to be prd-review's PRD-typed schema; the review-report reuse is refined to an own instance in the *family* (`e2e-review-report/v1`); the plan half of D-2 stands

### D-3 — e2e-review is its own separate skill (the fourth reviewer), reusing shared building blocks

- **Question**: is e2e-review a reuse/modification of an existing reviewer, or its own separate skill — and does it reuse the shared review machinery?
- **Answer**: its own separate skill — a fourth reviewer, exactly as prd-review / tech-design-review / impl-review are each separate skills. It follows the same proven pattern and reuses the shared building blocks (the `review-report/v1` format from D-2 and the shared context bundler, context-pack-v1) but is not a reuse of another reviewer and modifies none.
- **Reasoning**: operator's correction and ruling — the existing three are separate skills that share building blocks, not one reviewer reused three ways; e2e-review is kept separate the same way. Reusing the shared report format + context bundler (rather than rebuilding them) keeps new surface small while honoring the separation.
- **Anchor**: 2026-07-24 · reviewer-shape turn
- **Status**: live

### D-4 — the review lenses' design-time reach: keep the on-paper checks, defer the deep bite-check to run-time

- **Question**: does e2e-review add a dedicated design-time check that each coverage-map edge is genuinely exercised (the over-mapping / coverage-inflation catch — basis ABUSE-4 / §7.2 Q2)?
- **Answer**: no dedicated design-time mechanism for the genuine-exercise ("does the built test really fail when the code breaks?") check — that can only be confirmed by running the built test against broken code (falsification), which the basis already defers to the run-time build follow-on (ABUSE-1, WR-1). The two mandated lenses keep their on-paper reach: coverage flags a missing scenario and a scenario obviously unrelated to a behavior it is mapped to (gross over-mapping); anti-gaming flags a written mock strategy that guts the scenario it names (visible because the per-scenario route/mock is recorded).
- **Reasoning**: operator's point — the deep bite-check is post-implementation, not design-time; the basis routes ABUSE-4's mechanism to design (design-owned), and design rules it the same way the basis already treats run-time anti-gaming (deferred to WR-1). Keeps v1 honest — no weak/theatrical design-time check — while the mandated FR-8 lenses still do everything a written design can prove.
- **Anchor**: 2026-07-24 · reviewer-lenses turn
- **Status**: live

### D-5 — e2e-plan is its own separate skill; planner reuse is under the hood, one invocation

- **Question**: does reusing the planner machinery keep e2e-plan a separate user-facing skill, and does calling e2e-plan avoid any separate call to the existing planner?
- **Answer**: yes on both. e2e-plan is its own separate skill (its own command), like the other two. Reuse is internal only — under the hood it uses the existing planner's projection machinery (the blocked-by projection dialect, the idempotent tracker projection, the ordinary task/charter schema) and the executor's existing dependency-ordered frontier. The operator invokes `/e2e-plan` once; in that single call it decomposes the approved design into a harness task + per-scenario tasks (each blocked-by the harness) and projects them — never a second, separate invocation of the feature planner.
- **Reasoning**: operator's requirement — each e2e skill is a distinct user-facing command and a single invocation, and machinery reuse must be invisible to the operator (mirrors D-3 for the reviewer). Reusing the projection dialect + executor frontier under the hood keeps new surface small (freeze-at-approval and no-duplicate projection come free) while the operator's experience stays "one e2e-plan call, done."
- **Anchor**: 2026-07-24 · planner turn
- **Status**: live

### D-6 — e2e-design's inputs and input-gating: PRD backbone + design + code; refuse when any is absent

- **Question**: what does e2e-design work from, and what happens if those inputs are absent (the skill called by mistake)?
- **Answer**: it derives the coverage checklist from three inputs together — the feature's PRD requirement list (the backbone), its technical design, and the built code — never freewheeling an unanchored list. If any input is absent it refuses and produces nothing: no PRD or design → refuse naming the missing input; not-yet-implemented (no built code) → refuse naming the unlanded work.
- **Reasoning**: operator's ruling — coverage must be a checkable list anchored to the written requirements (topped up by design + code), and a mistaken call with the inputs absent must fail closed, never fabricate coverage from nothing. This honors the basis's already-mandated refusals (UB-1 not-fully-implemented; UB-2 missing/unreadable PRD or design) — no new mechanism, the design just obeys them.
- **Anchor**: 2026-07-24 · e2e-design inputs turn
- **Status**: live

### D-7 — governing-design selection: read the live design, fail-closed on ambiguity

- **Question**: which technical design does e2e-design read when a feature has more than one (a superseded old one plus a live successor)?
- **Answer**: it reads the current (live, non-superseded) design, following pyxis's existing liveness signal — the design index computes liveness from the supersession edges and flags replaced designs. On genuine ambiguity (more than one live design, or liveness indeterminable) it surfaces/refuses rather than silently picking, consistent with D-6's fail-closed posture.
- **Reasoning**: operator adopted the recommendation — reuse the existing liveness computation (no new mechanism) and never guess under ambiguity; resolves the basis's design-owned Q6 (a present-but-superseded design must not silently yield stale coverage).
- **Anchor**: 2026-07-24 · governing-design turn
- **Status**: superseded by D-14 — at design review, D-7's "more than one live design → refuse" was found to contradict basis §7.2 Q6 (read the live *set*) and would refuse on the split-design dogfood; refined by D-14

### D-8 — feature-evolution / e2e-test-delta capability: deferred as a follow-on (out of v1)

- **Question**: is the feature-evolution capability — when a feature's PRD/design later changes, tracking which existing e2e scenarios/tests must be changed or deleted — in this feature's scope, or a follow-on?
- **Answer**: deferred as a named follow-on, out of v1. This feature stays the first-time authoring line (a just-implemented feature → its e2e design → review → plan). The evolution/delta job is its own future feature, pairing naturally with the deferred build layer (a test can only be changed or deleted once it is built — basis WR-1/NG-1).
- **Reasoning**: operator's ruling (defer) — the delta capability is a distinct maintenance job the basis does not currently ask for; adding it would expand v1's agreed appetite and require a PRD amendment. Keeps v1 the clean first-time line; P-1 stands as the follow-on's evidence.
- **Anchor**: 2026-07-24 · evolution-scope turn
- **Status**: live

### D-9 — app-shape-aware harness selection is authored content, not skill code

- **Question**: how does e2e-design pick a test harness/mock approach for different app shapes without tying the skill to one shape?
- **Answer**: the skill reasons about the feature's primary external interface (web HTTP endpoint / CLI argv+stdout / library public API) and names a fitting harness + mock approach as authored content in the design doc; it carries no app-shape-specific code. New app shapes are handled by the skill's judgment producing different content, never by adding per-shape branches to skill code.
- **Reasoning**: confirms the basis's already-pinned contract — FR-6 (the named harness must be able to drive the app-shape's primary interface) and NFR-1 (app-neutral by construction; shape-specific choices live in authored content, not skill code). Keeps the authoring line general across app types with no new code per type.
- **Anchor**: 2026-07-24 · harness turn
- **Status**: live

### D-10 — e2e-design bounds its code-context read

- **Question**: how much code does e2e-design read on a run?
- **Answer**: a bounded, focused slice — the feature's own changed files plus the modules the design points to — never the whole tree unbounded. The exact cap is an implementation tuning detail.
- **Reasoning**: confirms the basis's NFR-3 (each run bounds the code-context it ingests, a run reaching the cap stops rather than fanning out; specific caps design-owned). Bounds steady-state cost on a per-feature recurring path (AP-19).
- **Anchor**: 2026-07-24 · context-bound turn
- **Status**: live

### D-11 — the feature is authored as two technical designs: the artifact contract, then the skills' behavior

- **Question**: is the feature written up as one technical design or split?
- **Answer**: two designs — (A) the **e2e artifact/contract** design: the new `e2e-design/v1` document class (its sections/template/home/freeze-at-approval, D-1) plus the reuse confirmations for the review report (`review-report/v1`) and the plan (plan-new's projection dialect + ordinary task schema, D-2); (B) the **e2e skills/behavior** design: the three skills (e2e-design's coverage derivation + governing-design pick + harness + bound; e2e-review's reviewer-program reuse + lenses + convergence; e2e-plan's decomposition + projection). (A) is the contract (B) builds against; the exact per-FR Serves split is the authoring step's job.
- **Reasoning**: operator adopted the recommendation — pin the artifact contract as its own frozen document first, then author the behavior against it; matches pyxis's "shape-of-the-data vs behavior-that-uses-it" split (the argus-artifacts/argus-agent and reviewer-program precedents) and RW-5 (one independently-revisable decision per document).
- **Anchor**: 2026-07-24 · partition turn
- **Status**: live

### D-12 — findings carry blocking + advisory severity; a blocking finding blocks approval until settled

- **Question**: does the e2e review have real teeth in v1 — blocking findings followed to resolution — or is it advisory-only?
- **Answer**: real teeth at the finding level. Each finding carries a severity (blocking or advisory) and a recommendation/acceptance condition. The review→converge loop runs until every finding is settled: a blocking finding must be solved in the design (or explicitly rejected with a stated reason) before the design can be approved and advance to the plan; advisory findings carry recommendations and are likewise resolved or rejected. Nothing advances with an open finding. Separately, the reviewer's overall verdict forces no downstream automated gate in v1 (none exists) — that reviewer-level advisory→blocking authority graduates like the other reviewers once a build/gate enforcement surface exists; the model-driven catch-rate stays a measurement, never a hard gate (I5).
- **Reasoning**: operator's ruling — blocking findings must be followed to resolution in the design, with recommendations, until all findings settle. This is the basis's convergence contract (FR-10 resolve-or-reject-before-approval; UB-5 block-advance-while-open) plus the reviewer program's blocking/advisory closable findings; the only deferred part is the reviewer forcing a downstream automated gate, which has no surface in v1 (NG-1/NG-4). Resolves the basis's design-owned Q3.
- **Anchor**: 2026-07-24 · reviewer-authority turn
- **Status**: live

### D-13 — the e2e review report is its own instance in the review-report family (supersedes D-2's report half)

- **Question**: does the e2e review report reuse `review-report/v1` verbatim (D-2), or its own instance-schema?
- **Answer**: its own `e2e-review-report/v1` in the review-report *family* — `review-report/v1` is prd-review's schema with a closed PRD-typed header; the family pattern is one instance-schema per reviewer class (`design-review-report/v1`, `impl-review-report/v1`), so the e2e report mirrors that. D-2's plan half stands (the plan reuses plan-new's `plan/v1` verbatim).
- **Reasoning**: design-review finding (the artifacts review report, FND-1, which rides the design PR) — the verbatim `review-report/v1` reuse is infeasible (its header only admits PRD subjects); the family reuse honors D-2's intent (reuse the proven review-report shape) while being buildable.
- **Anchor**: 2026-07-24 · review
- **Status**: live

### D-14 — e2e-design reads the live design *set*; ambiguity is superseded-no-successor (supersedes D-7)

- **Question**: which technical design(s) does e2e-design read — a single live design (D-7), or the live set?
- **Answer**: the live (non-superseded) *set* of designs serving the feature; a feature legitimately has several live designs (the sanctioned artifacts+behavior split — this feature has two). It surfaces/refuses only on genuine ambiguity — a superseded design with no clear live successor, or liveness indeterminable — never merely because more than one design is live.
- **Reasoning**: design-review finding (the skills review report, FND-2, which rides the design PR) — D-7's "more than one live design → refuse" contradicts basis §7.2 Q6 ("pick the live set") and would refuse on the split-design dogfood feature itself; the fix keeps D-7's intent (never use a stale superseded design) while reading the whole live set.
- **Anchor**: 2026-07-24 · review
- **Status**: live

## Alternatives

### ALT-1 — Reuse the technical-design class

- **Shape**: the e2e design doc is a technical design at `docs/tech-design/<feature>-e2e.md` (tech-design-template/v1), Serves the feature's FRs, linted by `lint-tech-designs`, listed in the design index.
- **Trade-offs**: maximal class uniformity and free reuse of the design lifecycle; but the e2e doc is per-feature test-coverage, not an engine-architecture decision — it mis-fits the class's semantics (DINV-n, Supersedes lineage, principle citations) and floods the design index with a per-feature entry each run.
- **Principles**: AP-11 (one predicate/place) surface-honored by reuse but strained — the class's meaning diverges; AP-27 (a shared append point is a defect) strained — every e2e doc edits the shared design index; AP-7 (immutable-once-merged) fits the freeze-at-approval need.
- **Incidents**: I4 (a `Serves` omission costs a supersession) and I1 (`Serves` must cite committed FRs) both bite a high-volume per-feature doc forced through the design-contract machinery — checked incidents.md 2026-07-23.
- **Verdict**: rejected — wrong semantics + design-index pollution + the append-point strain outweigh the uniformity.
- **From**: DQ-1 · panel

### ALT-2 — A new lightweight authored class

- **Shape**: a new `e2e-design/v1` class — its own light template and per-feature home (exact path design-owned, e.g. `docs/e2e/<feature>/`), a typed header joining it to the feature (`Basis: prd:<feature>`), frozen at approval/merge, **not** docgate-folded and **not** in the design index; the review report reuses `review-report/v1` (docs/reviews/), the plan reuses plan-new's projection dialect + the tracker.
- **Trade-offs**: right-sized to the artifact — per-feature, high-volume, model-authored, freeze-at-merge — with no index pollution and clean separation from engine-architecture designs; costs one new (but precedented) class definition.
- **Principles**: AP-7 (a new mutability class, frozen-at-merge — exactly how the brainstorm/review classes were added); AP-6 (contractual — typed header + structured coverage map); AP-23/AP-27 (one file per feature, flat/per-feature home, no shared append surface — honored); AP-21 (the frozen doc is the judge-once coverage artifact built against).
- **Incidents**: none directly — the document-system classes carry no incident yet (as design-brainstorm-record-model noted for its own class); I1/I4 avoided by staying off the technical-design class — checked incidents.md 2026-07-23.
- **Verdict**: chosen — operator ruling 2026-07-23 (D-1); matches the closest precedent (the design-brainstorm record class) and the basis's near-settled report/plan reuses.
- **From**: DQ-1 · panel

### ALT-3 — A new governed docgate kind

- **Shape**: like ALT-2 but docgate-governed — its own fold / immutability / citation checks and CI wiring, so freeze and traceability are mechanically enforced from day one.
- **Trade-offs**: strongest integrity guarantees; but the e2e design doc does not evolve by typed amendments (a changed feature re-runs e2e-design; it is not a ledger), so a fold/amendment machine is machinery the artifact does not need — over-built for v1's appetite (three skills + a dogfood).
- **Principles**: AP-10 (codify enforcement) honored but premature; AP-7 (governed kind) — heavier than the freeze-at-merge the artifact needs; the header Appetite argues against standing up new governance machinery in v1.
- **Incidents**: none directly (no document-system-class incidents yet) — checked incidents.md 2026-07-23.
- **Verdict**: rejected for v1 — the lightweight freeze-at-merge class (ALT-2) suffices; docgate governance can graduate later behind a shared trigger if volume or abuse warrants (the brainstorm/review-class staging pattern).
- **From**: DQ-1 · panel

## Seam map

- **S-1**: the e2e design-doc document class — opens — behind it: a new `e2e-design/v1` lightweight authored class (template + per-feature home + freeze-at-approval), ungoverned by docgate in v1 — feeds: the design's §2 Decision (new class) + §3.1 Scope (from D-1)
- **S-2**: the review-report family and the plan-new plan pipeline — extends — behind it: the e2e review report as its own `e2e-review-report/v1` instance in the review-report family, and the e2e plan as an ordinary plan-new `plan/v1` reused wholesale — feeds: the design's Scope (reused vs new) + Serves (from D-2, refined by D-13/D-14)

## Facts

- **F-1**: No live technical design lists `prd:e2e-verification/*` in its `Serves` — the solution space is open ground — source: docs/tech-design/index.md
- **F-2**: The basis near-settles two of the three artifacts as reuses: FR-7/§5.5 has `e2e-review` adopt the existing versioned review-report/closable-finding shapes; FR-14/FR-15/§5.5 has `e2e-plan` reuse plan-new's projection dialect and the ordinary task/charter schema — source: prd:e2e-verification/FR-7 · FR-14 · FR-15 · §5.5
- **F-3**: A new lightweight authored document class has a working precedent — the design-brainstorm record class was added as ungoverned markdown, shared/flat home, frozen-at-merge, docgate governance staged behind a shared graduation trigger — source: design:design-brainstorm-record-model §3.1 · Principles (AP-7 by extension)
- **F-4**: Dependency-ordered pickup already exists (plan-new blocked-by edges + implement-now's ready frontier), so `e2e-plan` (FR-13) need only declare the edge — source: brainstorm:e2e-verification#F-9 · design:implement-now §3.2/§3.6
- **F-5**: The reviewer program is three proven instances (prd-review · tech-design-review · impl-review) — blind fresh-context lenses over one shared context pack, closable findings in review-report/v1, scope-locked two-round convergence, downgrade-only verdict, advisory until a ten-review adoption checkpoint — source: docs/wiki/glossary.md (impl-review) · design:impl-review-v1
- **F-6**: The basis already mandates fail-closed refusal when e2e-design's inputs are absent — UB-1 (invoked before the feature's implementation tasks have landed → refuse, zero docs produced) and UB-2 (PRD or technical design missing/unreadable → refuse, naming the input) — source: prd:e2e-verification/UB-1 · UB-2

## Fog

none

## Out of scope

- **O-1**: a dedicated design-time genuine-exercise / per-edge over-mapping check — the deep "does the built test really fail when the code breaks?" verification — deferred to the run-time falsification of the build follow-on (basis WR-1 / ABUSE-1); design-time review catches only gross over-mapping and self-contradicting mock plans (from D-4)
- **O-2**: the feature-evolution / e2e-test-delta capability — updating or deleting existing e2e tests when a feature's PRD/design later changes — deferred to a follow-on feature; v1 is the first-time authoring line only (from D-8; pairs with the deferred build layer, basis WR-1)

## Candidate glossary terms

none

## Parked product gaps

- **P-1**: the basis frames e2e-design as first-time authoring for a just-implemented feature and does not cover feature **evolution** — when a feature's PRD or technical design later changes, detecting and recording in the new e2e design doc which existing e2e scenarios/tests must be **changed or deleted** (the coverage delta), rather than only authoring fresh coverage — anchors: prd:e2e-verification/FR-1 (first-time-authoring framing) · §1 · §2.6 ("coverage decided once") · placement (stage-1 `brainstorm:e2e-verification#D-4`, once per feature, terminal) · no delta/maintenance FR exists — disposition: **proceed-anyway per D-8** — deferred as a named follow-on (out of v1); this record is the follow-on's evidence
- **P-2** *(2026-07-24 · review)*: the operator ruled during technical design that a feature with **no meaningfully-automatable e2e surface** should stop the pipeline — e2e-design produces **no** e2e documents and signals the operator not to continue the downstream e2e steps — rather than the basis's UB-6 "explicit manual-only (or empty, reason-recorded) e2e design + manual-checklist result" — anchors: prd:e2e-verification/UB-6 · the skills review report's FND-1 (rides the design PR) — disposition: **bounced** — to `prd:e2e-verification` amendment 001
- **P-3** *(2026-07-24 · review)*: the operator ruled the "build the harness" task is **conditional** — e2e-plan emits it only when the feature's e2e harness does not already exist (scenario tasks depend on the harness whether newly built or pre-existing) — refining the basis's FR-12/FR-13, which assume an unconditional per-plan harness task — anchors: prd:e2e-verification/FR-12 · prd:e2e-verification/FR-13 — disposition: **bounced** — to `prd:e2e-verification` amendment 001

## Gate

| Field | Value |
|---|---|
| **Verdict** | ready-for-design |
| **Date** | 2026-07-24 |
| **Authority** | @mdarabi (operator) |
| **Covering design** | — |
| **Coverage check** | pass — core four covered (§2 direction: D-1 new artifact class + the three-skills-reuse shape D-3/D-5/D-11; §4 alternatives: ALT-1/2/3 from the DQ-1 fired panel, all verdicted; Serves+Scope: S-1/S-2 seam map + Basis `prd:e2e-verification`; §3.2 boundaries: O-1/O-2) — zero blocking fog |
| **Amend route** | — |
