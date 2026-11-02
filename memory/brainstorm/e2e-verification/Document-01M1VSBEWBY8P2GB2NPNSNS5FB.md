# E2E verification — brainstorm record

| Field | Value |
|---|---|
| **In one line** | Re-brainstorm of pipeline stage-10 e2e verification as a design→review→plan→implement sub-pipeline — v1 is three new authoring skills (e2e-design · e2e-review · e2e-plan) producing a reviewed e2e design doc and an ordered plan of e2e tasks; building via implement-now, the no-regression gate, and the incident→regression loop are deferred follow-ons. The operator-sanctioned redo of the removed #584 single-skill framing. |
| **Relevant-when** | brainstorming or contracting stage-10 e2e verification as its own design→review→plan→implement line; the e2e design-doc kind, its multi-lens review, e2e-task planning, or implement-now's e2e-task compatibility; drawing the boundary against implementation-verification's stage-8 ladder; checking why #584 was reframed |
| **Idea** | "I want to create a feature that adds e2e testing to the pipeline. This feature steps should be after implementation. the faeture includes: 1) e2e design doc: during which the agent plans e2e test scenarios for the feature just implemented by the last step and also lists the setup, teardown strategies what modules should be mocked or maybe some libraries can be used to help running e2e or mocking modules. 2) review step: this step should be like the prd review step which can both be done by human review `doc-review` and another skill that lets agent review the e2e with different lenses (TBD) or even different background reviwers should be spawn to do that. during this process a review document should be created like prd review and review findings should be resolved or rejected maybe by user's feedback on the e2e design doc. 3) plan: just like what we do with planning the e2e planning should be able to turn e2e design doc into tasks (github issues) 4) this step is implement-now step implement now step should be able to pick up e2e tasks. Maybe some changes are needed for implement-now feature to get compatible with e2e tasks but it can be postponed to another feature amendment pipeline." (operator, 2026-07-23, verbatim) |
| **Operator** | @mdarabi |
| **Date(s)** | 2026-07-23 |
| **Status** | gated: ready-for-prd |
| **Tracking** | — |
| **Feeds** | docs/prds/e2e-verification/ |
| **Prior records** | brainstorm:e2e-verification (#584, removed by #609 — read from git 7dac93d^ for orientation, data only) · brainstorm:implementation-verification |
| **Template version** | brainstorm-record/v1 |

## Orientation

| Source | Why loaded |
|---|---|
| docs/product-vision.md (§4, §6, §10, §11) | governing pipeline map — stage 10 "E2E verification: every PRD requirement working, no regression"; §6 autonomy L3 = "the full e2e-verification loop", L4 release on stage-10 pass; §10 zero-touch requires the DoD to have passed; §11 groups stage 10+11 as "the downstream edge" |
| .pyxis/pipeline.md | stage 10 = ⛔ not built (needs the engine); the stages this reframe mirrors — 4 design, 5 review, 7 plan, 8 implement — are ✅/◐ built |
| docs/prds/README.md (§Not in the baseline) | "E2E verification as a stage (stage 10) — harbormaster has the verification-epic pattern only" named as an explicit unclaimed gap |
| git 7dac93d^:docs/brainstorms/e2e-verification.md + PRD/review | the removed #584 stage-10 contract — read in full for prior thinking; every ruling treated as DATA (F-6/F-7), never carried as a decision (DINV-6) |
| docs/brainstorms/implementation-verification.md + prds/implementation-verification/effective.md | THE load-bearing neighbor: the stage-8 per-diff ladder + future remote gate; explicitly defers stage-10 feature-level e2e to its WR-4; owns the anti-gaming checks (falsification, test-delta) this reuses |
| docs/wiki/glossary.md | the unmanned reviewer program shape the "review step" mirrors (impl-review: four blind lenses, closable findings, review-report/v1, downgrade-only verdict, advisory→checkpoint); ladder/rung, verification manifest, floor-clearing, verification methods |
| docs/domain-model.md (§3.2, §3.6) | E2EVerification named a first-class future entity ("New — graduates the verification-epic pattern"); e2e verification is a JudgmentPoint; INV-42 traceability |
| docs/prds/design-and-planning.md (PLAN-5) | the parity ancestor: a terminal "verification epic, blocked by all others, validates the design end-to-end before the plan closes" |

**Overlaps surfaced:**

- **The removed #584 e2e-verification contract — the sanctioned redo, not a duplicate.** The operator undid #584 in #609 explicitly "to reframe the stage from scratch under a richer framing (design doc → review → plan → implement)" — which is exactly this session's idea. Its prior rulings inform orientation (F-6) but are re-opened, not carried. Its co-headline — the incident→regression-e2e-test loop (old D-6/D-16) — is **absent** from the reframed idea (F-7): a live scope question, not an assumption.
- **implementation-verification (stage-8 harness) — the drawn boundary.** It owns the per-diff deterministic ladder + a future remote CI gate and **defers stage-10 feature-level e2e to its WR-4** ("this harness's exec-probe verdicts are its seed material"). Boundary: theirs = per-diff suite-level no-regression + anti-gaming; this = feature-level behavioral verification on the integrated system. Its anti-gaming checks are the reuse target so auto-written e2e tests genuinely bite.
- **The pipeline skills the framing mirrors (F-4/F-5).** tech-design-new + the reviewer program (prd-review/tech-design-review/impl-review) + plan-new + implement-now already exist. The reframe recurses the pipeline onto e2e-test authoring; the open axis is degree of reuse vs. new e2e-specific surface — the framing question below.
- **incident-machinery (stage 12) + VNEXT-1/2/3.** The incident-regression feeder (if in scope) depends on an incident-machinery amendment adding reproduction data. VNEXT-1 verification graph / VNEXT-2 structured DoD / VNEXT-3 traceability are largely unbuilt substrates; old D-5 ruled the coverage list is derived from PRD+design, not blocked on them.

No source covers the reframed idea; the center of gravity — e2e verification as its own design→review→plan→implement line — is open ground.

## Framings

*Three genuinely distinct theses of what this feature is. The four steps are the operator's given; these differ on how much is new machinery vs. reused pipeline, and where the feature's weight sits. Destination pending the operator's pick or hybrid.*

- **F-A · The factory turned on itself (recurse-and-reuse).** The e2e stage *is* the existing pipeline (design → review → plan → implement) recursively applied to the sub-problem of authoring a feature's e2e tests: the e2e design doc is a technical-design (or close variant), its review reuses the unmanned reviewer program + doc-review, its plan reuses plan-new, its build reuses implement-now. e2e-specific concerns (scenarios, setup/teardown, mock strategy, harness libraries; coverage/anti-gaming/CI-runnability lenses) ride as *content and lenses inside the reused artifacts*, not a parallel skill set. Thesis: *e2e verification is not new machinery — it's proof the pipeline is general enough to build its own tests; maximal reuse, uniform artifacts.*
- **F-B · A dedicated e2e verification line (parallel-and-specialized).** The e2e stage is a *new* four-step sub-pipeline of purpose-built e2e skills (`/e2e-design`, an e2e reviewer with e2e-specific lenses, `/e2e-plan`, e2e-aware execution), each carrying e2e-unique structure the generic artifacts don't natively hold. Thesis: *e2e authoring differs enough from feature authoring (black-box scenarios, harness, mocking, CI-runnability, anti-gaming on self-written tests) that it earns its own line that merely rhymes with the pipeline.*
- **F-C · The reviewed e2e design is the deliverable (contract-first, thin back).** The feature's center of gravity is the e2e design doc + its multi-lens review — turning "what e2e must prove and how" into a reviewed, contract-quality artifact. Plan and implement are deliberately minimal reuse: plan-new decomposes it into ordinary tasks, implement-now picks them up as-is, and implement-now's e2e adaptation is explicitly deferred to a later amendment (as the idea states). Thesis: *the hard, human-in-the-loop value is deciding and reviewing e2e coverage/strategy; building the tests is downstream commodity — ship the design+review, reuse and defer the rest.*

**Destination:** **F-B for the authoring line + F-C's back half.** Three new, dedicated e2e skills — e2e-design, e2e-review, e2e-plan — each purpose-built for e2e (paralleling, not reusing, tech-design-new / the reviewer program / plan-new); the fourth step is **implement-now reused exactly**, with its adaptation to e2e tasks explicitly deferred to a later amendment (out of this feature's build). — Operator's rationale: the three authoring steps are e2e-specific enough to be their own skills; the executor is generic enough to reuse as-is, and its e2e adaptation is a separable follow-on (framing turn, 2026-07-23).

## Decisions

### D-1 — the identity: three new dedicated e2e skills (design · review · plan) + implement-now reused, adapted later

- **Question**: which framing is the feature — reuse the pipeline (F-A), a dedicated parallel e2e line (F-B), or the reviewed-design-is-the-deliverable with a thin reused back (F-C)?
- **Answer**: dedicated **new skills** for e2e design, e2e review, and e2e plan (F-B — purpose-built, paralleling not reusing tech-design-new / the reviewer program / plan-new); the **implementation step is implement-now reused exactly**, with its adaptation to e2e tasks deferred to a later amendment (F-C's back half), out of this feature's build.
- **Reasoning**: operator's pick — the three authoring steps carry enough e2e-specific structure (scenarios, setup/teardown, mock strategy, harness libraries; e2e-specific review lenses; e2e-test tasks) to warrant their own skills; the executor is generic enough to reuse untouched, and its e2e adaptation is a separable follow-on the operator explicitly named.
- **Anchor**: 2026-07-23 · framing turn
- **Status**: live

### D-2 — scope: this feature is the e2e-test authoring line; the dedicated no-regression gate is deferred

- **Question**: does the feature stop at authoring a feature's e2e tests (design → review → plan → build), or also own a dedicated feature-level no-regression gate (the accumulated e2e suite as a distinct required check, a feature-level verdict, advisory→blocking graduation)?
- **Answer**: authoring only. v1 = the four-step line that produces the current feature's reviewed, planned, and built e2e tests. The dedicated feature-level no-regression gate is **out of scope for v1** (O-1), a self-contained follow-on.
- **Reasoning**: operator adopted the recommendation — every one of the four steps is about producing tests, none about a gate; regression protection already comes for free once tests merge (they run in ordinary CI) and from the stage-8 harness's no-new-failures enforcement over the suite; the dedicated gate is a *refinement* (distinct category, feature-level verdict, own required badge, advisory→blocking ceremony) that builds later without touching the authoring line. Keeps v1 one coherent job.
- **Anchor**: 2026-07-23 · scope-boundary turn
- **Status**: live

### D-3 — the incident→regression-e2e-test loop is deferred to a separate follow-on

- **Question**: is the "recorded incident → auto-written permanent regression e2e test" loop (a #584 co-headline, F-7) in scope, deferred, or dropped?
- **Answer**: deferred — a separate follow-on, out of scope for this feature (O-2).
- **Reasoning**: operator adopted the recommendation — the reframe is organized around "the feature just implemented" (a forward-looking, feature-triggered line); the incident loop is a different, out-of-band trigger (a past failure), carries a hard cross-feature dependency (incident records must first carry reproduction data, owned by incident-machinery), and reuses this feature's same authoring machinery — so it bolts on cheaply later without rework. Keeps v1 one coherent job.
- **Anchor**: 2026-07-23 · incident-loop turn
- **Status**: live

### D-4 — placement: once per feature, terminal, fired after all the feature's tasks are implemented

- **Question**: does the line run once per feature (terminal, after all the feature's tasks land) or per task, and is it operator-invoked or auto-fired?
- **Answer**: once per feature, as a terminal step — fired after the whole feature's implementation tasks are built (and reviewed). Not per task. Operator-invoked in v1 (auto-firing on feature-completion is a later convenience, parked P-1).
- **Reasoning**: operator's confirm — an end-to-end test needs the whole integrated feature assembled; a single task is only a slice, so there is no end-to-end behavior to exercise until all tasks land. Matches the harbormaster terminal verification-epic pattern (F-8, PLAN-5).
- **Anchor**: 2026-07-23 · placement turn
- **Status**: live

### D-5 — the e2e design doc's shape: six sections, incl. an explicit coverage map and a per-scenario route

- **Question**: what does the e2e design doc contain?
- **Answer**: six parts — (1) scenarios (the end-to-end flows), (2) setup/teardown, (3) what to mock (and with what), (4) helper libraries / harness, (5) **coverage map** — each thing the feature must do → the scenario(s) proving it (an explicit, checkable list, not left to agent judgment), (6) **route per scenario** — run-for-real / mocked / manual-human.
- **Reasoning**: operator confirmed the four listed contents + both additions. The coverage map is what the reviewers (human + agent) check against — without it "is coverage good?" has no answer (T-1). The per-scenario route is the honesty valve against mocking away the hard part (P-2).
- **Anchor**: 2026-07-23 · design-doc-shape turn
- **Status**: live

### D-6 — coverage source: derived from the feature's PRD + technical design + built code; not blocked on the structured DoD

- **Question**: where does the "what to prove" coverage list come from?
- **Answer**: the e2e-design skill derives it by reading the feature's PRD + technical design (plus the built code); the operator reviews it. It does **not** wait on the not-yet-built structured definition-of-done.
- **Reasoning**: operator adopted the recommendation — buildable today with no blocking dependency; the structured DoD becomes a sharper input later but is not a prerequisite (mirrors #584's F-6 lean; keeps the coverage list explicit without a wait).
- **Anchor**: 2026-07-23 · design-doc-shape turn
- **Status**: live

### D-7 — the runnability rule: every scenario CI-runnable, else mocked or routed to a human; nothing dropped

- **Question**: how are scenarios that are impossible/very hard to run in CI handled?
- **Answer**: every scenario must be runnable in CI; whatever cannot be is **mocked** or **routed to manual/human testing**, and the route is recorded per scenario (D-5 item 6) — never silently dropped.
- **Reasoning**: operator adopted the recommendation — bounds the generality risk by construction; the manual route is the "Human judgment" verification method (named owner + rubric); honest degradation mirroring the stage-8 harness's skip-with-record posture.
- **Anchor**: 2026-07-23 · design-doc-shape turn
- **Status**: live

### D-8 — the review step: human doc-review + a new unmanned e2e-review skill, review-report + resolve-or-reject, mirroring the reviewer program

- **Question**: is the review step two halves on the e2e design doc — human `doc-review` inline annotation + a new unmanned multi-lens e2e-review skill producing a review report — with findings resolved (edit the doc) or rejected (with reason), like prd-review?
- **Answer**: yes. Two halves on the same e2e design doc: (1) human `doc-review` (rendered, inline-annotatable); (2) a new **e2e-review** skill — the reviewer program's fourth instance — spinning up blind, fresh-context lens reviewers over one context pack, emitting closable findings in a review report. Each finding is resolved (edit the doc) or rejected (with reason); the doc is a live draft through review and freezes at merge.
- **Reasoning**: operator confirmed — mirrors the established reviewer program (F-4) and doc-review, reusing its shapes (closable findings, review-report artifact, scope-locked convergence, advisory-until-checkpoint verdict) while being a new e2e skill per D-1.
- **Anchor**: 2026-07-23 · review-step turn
- **Status**: live

### D-9 — the four e2e-review lenses: coverage · anti-gaming/will-it-bite · feasibility · faithfulness-to-contract

- **Question**: what are the e2e-review lenses (flagged TBD)?
- **Answer**: four — (1) **coverage** (every required behavior has a proving scenario; no gaps), (2) **anti-gaming / will-it-bite** (each scenario reds if the feature breaks; no mock hollows out what it proves; routes honest), (3) **feasibility** (the scenarios/harness/setup/libraries can actually run in CI as described), (4) **faithfulness to the contract** (the e2e design matches the feature's PRD + technical design — proving what was specified).
- **Reasoning**: operator adopted the proposed set — each maps to a real e2e risk and to the existing reviewers' lens philosophy (completeness critic → coverage; correctness/falsification → anti-gaming; feasibility is e2e-specific; upstream-alignment → faithfulness). Exact wording may still refine in the design phase.
- **Anchor**: 2026-07-23 · review-step turn
- **Status**: live

### D-10 — the plan emits explicit harness→scenario dependency edges; ordered pickup then comes free from implement-now (not part of the deferred adaptation)

- **Question**: how do we guarantee implement-now builds the harness before the scenarios, since they can't be done in parallel — and is that ordered pickup part of the deferred implement-now adaptation?
- **Answer**: the e2e-plan emits **explicit dependency edges** — every scenario task is `blocked-by` the harness task — using plan-new's existing edge mechanism. Ordered pickup then comes **free** from implement-now's existing behavior (it builds only the ready frontier — open ∧ unblocked ∧ unclaimed over the frozen DAG — and never builds a dependent off an unmerged blocker), so it is **not** part of the deferred adaptation (correcting the operator's "do it later" assumption, F-9).
- **Reasoning**: operator's refinement (make the harness→scenario dependency obvious so the harness and scenarios are never built in parallel); the lookup (F-9) shows plan-new already wires blocked-by edges and implement-now already respects the DAG frontier — so v1's e2e-plan need only declare the edges; the harness builds, the driver merges it, the next sweep unblocks the scenarios.
- **Anchor**: 2026-07-23 · plan-step turn
- **Status**: live

### D-11 — segments: the factory's own loop is P1; the operator is P2; hand-only checks and operator-less repos are anti-segments

- **Question**: who is the primary user the design optimizes for, who is secondary, and who is out?
- **Answer**: P1 = the factory's own loop (the three new skills + the reused executor) — what runs constantly and the core design optimizes; P2 = the operator (reviews/accepts the e2e design doc, answers run/mock/manual when asked, trusts the result). Anti-segments: behaviors only checkable by hand/physically (routed to manual, not auto-e2e, D-7); repos with no operator to review and accept.
- **Reasoning**: operator confirmed — matches the north star (the engineer only defines + accepts) and implementation-verification's machine-P1 precedent (the machine runs the loop many times, the human touches it a handful per feature).
- **Anchor**: 2026-07-23 · segments turn
- **Status**: live

### D-12 — the jobs: factory authors→reviews→plans→builds the e2e tests; the operator actively reviews (doc-review + comments) and accepts

- **Question**: are these the jobs — the factory's four-step loop plus the operator's review/accept touch?
- **Answer**: confirmed. **Factory:** from a just-finished feature → author the e2e design (coverage map + scenarios + harness/mock strategy) → get it reviewed through the four lenses → plan it into ordered tracker tasks → build them via implement-now. **Operator:** actively review — including running `doc-review` and leaving comments *alongside* the unmanned four-lens review — resolve/reject findings, decide run/mock/manual when asked, and accept.
- **Reasoning**: operator confirmed and refined — the human half is active review (doc-review annotations), not just final acceptance; it runs in parallel with the unmanned lenses and both feed the convergence (reinforces D-8).
- **Anchor**: 2026-07-23 · segments turn
- **Status**: live

### D-13 — the bet and its observables: reviewed→planned→built e2e catches what units miss; dogfood primary

- **Question**: what is the bet, and what would we observe that proves it works?
- **Answer**: bet — authoring a feature's e2e tests as a *reviewed, planned, built* artifact (design → review → plan → implement) yields genuinely-covering, non-hollow, feature-level e2e tests that catch what unit tests miss, while the engineer only defines and accepts. Observables: (1) **dogfood (primary)** — the whole line on a real pyxis feature produces a reviewed e2e design doc + an ordered plan + built e2e tests the operator accepts; (2) **coverage catches gaps** — a seeded missing scenario is flagged by the coverage lens, not slipped; (3) **anti-gaming bites** — a seeded hollow scenario is caught by the will-it-bite lens (and by the falsification check when built). Guardrail: the false-red rate on the coverage/anti-gaming lenses is watched.
- **Reasoning**: operator adopted the recommendation, dogfood primary — each observable maps to a value prop (produce reviewed covering tests → 1; coverage completeness → 2; non-hollow → 3); the guardrail guards against a cry-wolf reviewer training red-blindness.
- **Anchor**: 2026-07-23 · bet turn
- **Status**: superseded by D-14 — the operator narrowed v1's dogfood to authoring-only (excludes implement-now/build); observable 1 ("built e2e tests accepted") is deferred, so the observables are restated to v1's real scope. The bet-wager itself is unchanged.

### D-14 — appetite: v1 is the three authoring skills only (design→review→plan); the build step (implement-now) is out of v1 and its dogfood, deferred to the e2e adaptation

- **Question**: how big is v1, how general, and does the dogfood run implement-now to build the tests?
- **Answer**: v1 = the **three new skills only** (e2e-design, e2e-review, e2e-plan). The build step — implement-now picking up and building the e2e tasks — is **out of v1 and out of its dogfood** (O-3), deferred until implement-now is adapted to the e2e pipeline. The dogfood therefore proves **design → review → plan** on a real pyxis feature: a reviewed e2e design doc + an ordered plan of e2e tasks on the tracker, accepted by the operator. General-app *execution* is also deferred with that adaptation; the three authoring skills stay general by construction (they produce a design + tasks, not a running system, and the design skill picks the right harness/mocks per app shape). Restated observables (superseding D-13's): (1) dogfood → reviewed design + ordered plan accepted; (2) the coverage lens catches a seeded missing scenario; (3) the will-it-bite lens catches a seeded hollow scenario *at review* (the build-time falsification check rides in with the deferred build step).
- **Reasoning**: operator's explicit refinement — do not run implement-now for this stage until it is adapted; keeps v1 a clean, coherent authoring line (three skills) with no reliance on an unadapted executor, and moves every execution/generality risk into the named follow-on.
- **Anchor**: 2026-07-23 · appetite turn
- **Status**: live

## Facts

- **F-1**: Stage 10 (E2E verification) is ⛔ not built and is named an explicit unclaimed gap ("harbormaster has the verification-epic pattern only") — source: .pyxis/pipeline.md; docs/prds/README.md §Not in the baseline
- **F-2**: #584 (49ba796) added a stage-10 e2e-verification contract (brainstorm + PRD 000-base + prd-review report + 2 glossary terms); #609 (7dac93d, operator-authorized) deleted it "to reframe the stage from scratch under a richer framing (design doc → review → plan → implement)"; content recoverable at git 7dac93d^ — source: git log
- **F-3**: implementation-verification (stage-8 harness) owns the per-diff deterministic ladder (lint/build/test+baseline/falsification/exec-probe) + a future remote CI gate, and **explicitly defers stage-10 feature-level e2e to its WR-4**; its anti-gaming checks (falsification, test-delta) are the reuse target so auto-written e2e tests genuinely bite — source: docs/prds/implementation-verification/effective.md §3.4 WR-4; brainstorm:implementation-verification D-8/F-2
- **F-4**: the unmanned reviewer program (prd-review · tech-design-review · impl-review) is the shape the "review step" mirrors: four blind fresh-context lenses over one shared context pack, closable findings in a versioned review-report payload, scope-locked two-round convergence, downgrade-only verdict, advisory until a ten-review adoption checkpoint — source: docs/wiki/glossary.md (impl-review); docs/prds/{prd-review,tech-design-review,impl-review}
- **F-5**: the pipeline skills the framing mirrors are built — tech-design-new (design authoring, immutable-once-merged), plan-new (decompose contract → frozen tasks projected onto the tracker), implement-now (local synchronous executor: claim ready task → drive-to-green → open proposal, never merges) — source: .pyxis/pipeline.md stages 4/7/8
- **F-6**: #584's still-relevant prior rulings (DATA, not decisions): factory auto-authors the e2e tests; an explicit enumerated coverage list drives coverage; coverage derived from PRD+design, not blocked on structured DoD; every e2e test CI-runnable, the untestable mocked or routed to manual/Human-judgment, never dropped; e2e is an additive feature-level layer over implementation-verification, not a replacement; the gate is a new required check, advisory→blocking; aim general across app types — source: git 7dac93d^:docs/brainstorms/e2e-verification.md D-2/D-3/D-5/D-7/D-8/D-10/D-11
- **F-7**: the incident→regression-e2e-test loop was a #584 co-headline (a recorded incident triggers auto-authoring a permanent regression test; depends on an incident-machinery amendment adding reproduction data) but is **absent** from the reframed idea, which describes only the current-feature design→review→plan→implement flow — source: git 7dac93d^ D-6/D-16 vs the session idea
- **F-8**: domain-model names E2EVerification a first-class future entity ("New — graduates the verification-epic pattern") and e2e verification a JudgmentPoint; PLAN-5's terminal verification epic is the parity ancestor — source: docs/domain-model.md §3.2/§3.6; docs/prds/design-and-planning.md PLAN-5
- **F-9**: dependency-ordered pickup **already exists** for feature tasks: plan-new emits explicit `blocked-by` dependency edges (native tracker relation, cycle-refused) and projects them "wired before anything becomes routable"; implement-now builds only the **ready frontier** (open ∧ unblocked ∧ unclaimed over the frozen plan's DAG) and **never** bases a dependent on an unmerged blocker (DINV-2) — reporting blocked dependents, not waiting. So harness-before-scenarios needs only the edge declared, not an implement-now change — source: docs/prds/plan-new/effective.md (FR-10 frontier; §Evidence rows); docs/tech-design/implement-now.md §3.2/§3.6, DINV-2

## Fog

none

## Out of scope

- **O-1**: the dedicated feature-level no-regression **gate** — the accumulated e2e suite run as a distinct required check with a feature-level "nothing regressed" verdict and an advisory→blocking graduation. Deferred, not dropped: a self-contained follow-on; regression protection already holds via ordinary CI + the stage-8 harness. WR seed: revisit when the authoring line ships and a feature-level required check is wanted (from D-2)
- **O-2**: the **incident→regression-e2e-test loop** — a recorded incident triggering auto-authoring a permanent regression test reproducing the failure. Deferred, not dropped: a separate follow-on reusing this feature's authoring machinery, blocked on incident records carrying reproduction data (incident-machinery's to add). WR seed: revisit after this line ships and the incident-reproduction-data amendment lands (from D-3)
- **O-3**: the **build/execution step** — implement-now picking up the e2e tasks and building the tests to green (incl. the build-time falsification/anti-gaming check, running the harder app shapes in CI). Out of v1 and its dogfood: v1 stops at the ordered plan on the tracker. WR seed: the implement-now e2e adaptation follow-on (P-3); until then no unadapted executor runs this stage (from D-14)

## Candidate glossary terms

- **T-1**: **e2e coverage map** — the explicit "what the e2e tests must prove" list inside the e2e design doc: each thing the feature must do → the scenario(s) proving it, plus each scenario's route (run-for-real / mocked / manual-human); factory-derived from the feature's PRD + design, what the reviewers check against · avoid: bare *coverage list* (#584's term — "map" marks the requirement→scenario join), *definition of done* (the engineer-authored artifact this is derived from, not identical to)

## Parked technical constraints

- **P-1**: auto-firing the e2e line when a feature's implementation completes (vs operator-invoked) is a later engine-era convenience — it does not change the design; v1 is operator-invoked (from D-4) — context: placement turn
- **P-2**: the **mock route is itself a gaming/coverage risk** — mocking away the hard part can make a scenario prove nothing; what was mocked must be recorded per scenario (D-5 item 6) and visible to the reviewers, so a shortcut is always seen — context: design-doc-shape turn
- **P-3**: the **auto-written e2e tests need build/run-time anti-gaming** too, not just the design-review "will-it-bite" lens (D-9) — reuse implementation-verification's falsification + test-delta checks at the e2e level so a hollow auto-written test reds; lands when implement-now is adapted to e2e tasks (D-1's deferred follow-on) — context: review-step turn
- **P-4**: the **home, document-kind, and mutability of the three new artifacts** (e2e design doc, e2e review report, e2e plan) is a stage-3/4 design decision — new governed kind(s) vs reuse of technical-design / review-report/v1 / plan/v1 vs a lighter provenance class; where each lives (docs/…); whether docgate governs them. v1 intent: the e2e design doc is reviewed then frozen at merge and built against (D-8); the review report and plan mirror the existing artifacts' shapes. Not resolved here — the design skill is also app-shape-aware (picks harness/mocks per web/CLI/library, D-14) — context: appetite turn

## Gate

| Field | Value |
|---|---|
| **Verdict** | ready-for-prd |
| **Date** | 2026-07-23 |
| **Operator** | @mdarabi |
| **Covering source** | — |
| **Coverage check** | pass — all six inputs covered (§2.1 problem → D-1 identity + D-14 bound; §2.2 segments → D-11; §2.3 jobs → D-12; §3.1 goals/bet → D-14 live; §3.4 non-goals → O-1/O-2/O-3; header appetite → D-14), zero blocking fog |
