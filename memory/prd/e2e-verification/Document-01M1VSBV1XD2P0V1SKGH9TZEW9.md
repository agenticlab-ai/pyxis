# Amendment 002 — the harness is designed, the plan is buildable in parallel, and the suite's cost is declared

| Field | Value |
|---|---|
| **Amends** | e2e-verification *(chain through 001)* |
| **Status** | Approved |
| **Owner** | @mdarabi |
| **Date** | 2026-08-06 |
| **Learned-from** | build-result:docs/e2e/tech-design-brainstorm/design.md · build-result:docs/e2e/plan-new/design.md |
| **Blast radius** | tier: contract-changing; touches: FR-1→FR-18, FR-6→FR-19, FR-8→FR-24, FR-16→FR-25, FR-17→FR-26, EVT-2→EVT-5, EVT-3→EVT-6, DOD-3→DOD-9, Q2→Q7, adds FR-20, FR-21, FR-22, FR-23, UB-8, UB-9, NFR-5, EVAL-3, DOD-8, and supersedes §0, §3.1, §3.2, §4.2, §5.3 (adding ABUSE-5), §5.4, §6.1, §6.3 (adding R6), §8; reason: supersedes five Must functional rows and nine sections — the e2e design doc must now specify the machine that runs the tests, not only what the tests must prove |
| **Template version** | amendment-template/v2 |

## Operations

### op: supersede FR-1

Reason: the produced doc named "harness libraries" as one of its six parts, and both built e2e design docs answered it with a library preference rather than a harness — so the section that decides how every test is built was never a design.

| ID | Requirement (EARS) | Rationale | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|
| FR-18 | When e2e-design is invoked on a feature whose implementation tasks have all landed, the skill shall produce an e2e design doc containing scenarios, setup and teardown, a mock strategy, a harness design, a coverage map, and a per-scenario route. | the design doc is the reviewed artifact everything downstream consumes, and it must carry the machine that proves the coverage as well as the coverage itself | Must | med | on the dogfood feature and the fixture features, the produced doc contains all six parts, each non-empty or an explicit `none — <reason>`; the harness part satisfies FR-19 or FR-20 rather than naming a library only | Demo, Inspection | G1, J1, F1 |

### op: supersede FR-6

Reason: naming a harness that can drive the interface is not enough to build one; the mechanics that every scenario task depends on were being invented by whichever worker picked up the harness task, after review had closed.

| ID | Requirement (EARS) | Rationale | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|
| FR-19 | Where the feature has no existing harness to reuse, the e2e design doc's harness section shall specify the harness mechanically, answering each of ten elements — the driver (how a scenario starts the system under test and what it gets back), the world (what the run executes against and who creates it), the scenario contract (what one scenario is as an artifact and where it lives), the seams (the named interfaces a fake attaches to), the observation surface (what a scenario is handed to assert on), lifecycle and isolation (setup, teardown, what is shared, what must run serially), determinism controls (clock, randomness, model tier, retries, and what an assertion may therefore claim), cost controls (the per-run bound and how it is enforced), failure diagnostics (what a red run leaves behind), and the boundary (what the harness does not do). | the harness is the most reused and most load-bearing code in the suite and was the least designed part of the doc; specifying it at design time is what lets the review see it and lets a scenario be written against a known contract | Must | high | across fixtures of at least two app shapes and on the dogfood, the harness section carries a stated answer for each of the ten elements; a section naming libraries with an element unanswered fails the well-formedness check and the buildability lens (FR-24); the buildability lens returns no finding against the section | Inspection, Test | G1, J1, F1 |

### op: add FR

Reason: a harness is per-app-shape and shared, not per-feature — repeating the full mechanical design for every feature would be waste, and re-deciding it would let sibling features drift apart.

| ID | Requirement (EARS) | Rationale | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|
| FR-20 | Where a harness able to drive the feature's primary external interface already exists in the repo, the harness section shall cite that harness and specify only this feature's additions to it — the fixtures, fakes, and helpers this feature needs — rather than restating or re-deciding its mechanics. | the mechanical design is paid once per app shape and reused thereafter; a citation plus a delta keeps sibling features on one harness instead of forking it | Must | med | on a fixture whose harness already exists, the harness section cites it and lists only additions, with no element of FR-19 re-decided; on a fixture with no existing harness, the section carries the full FR-19 design; a section that restates a cited harness's mechanics differently is a review finding | Inspection, Test | G1, J1, F1 |

### op: add FR

Reason: scenarios were written as English prose nobody had checked was buildable, so the coverage map could not tell which scenarios shared a fixture or a file — which is what left the plan unable to assign work without collisions.

| ID | Requirement (EARS) | Rationale | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|
| FR-21 | When the harness section is settled, each scenario shall be restated in that harness's own terms — naming the fixture it requires, the seam or seams it exercises, and the one artifact it exclusively owns under the harness's scenario contract. | a scenario expressed in the harness's vocabulary is one a worker can build and a planner can assign; exclusive artifact ownership is what makes parallel scenario tasks safe | Must | med | on the dogfood and the fixtures, every scenario names its fixture, its seams, and exactly one owned artifact; no two scenarios name the same artifact; a scenario naming a seam the harness section does not carry is a review finding | Inspection, Test | G1, J1, F1 |

### op: add FR

Reason: a feature whose flow requires human decisions cannot be tested end to end by a suite that waits for a human, and the one built example invented an answer-bank stand-in with no contract behind it — so nothing said what such a stand-in must do or must never do.

| ID | Requirement (EARS) | Rationale | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|
| FR-22 | Where the feature under test requires human decisions inside the flow it exercises, the harness section shall specify a decision-authority stand-in that supplies those decisions with no live human — a scripted answer set consulted first, a persona fallback constrained to a written character sheet, every fallback answer recorded as an improvisation with the assertions depending on it reported non-authoritative, at least one adversarial persona (silence, contradiction, or refusal), and no visibility of the run's assertions to the stand-in. | an agentic feature has no other way to reach its own end-to-end behavior; a stand-in that only agrees, or that can see what the test asserts, manufactures a green rather than proving one | Must | high | on the agentic fixture committed in §4.2, the harness section specifies all five properties; a stand-in specified as improvising silently, as cooperative-only, or with sight of the assertions is a review finding; specifying the stand-in is authoring, and building or running it remains the deferred follow-on (NG-1, WR-1) | Inspection, Test | G1, J1, F1 |

### op: add FR

Reason: a suite that drives real agent sessions costs real money per run, and nothing in the doc said what a run would cost or how often it would happen — so cost was discovered at build time instead of judged at review time.

| ID | Requirement (EARS) | Rationale | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|
| FR-23 | Each scenario shall declare a cost class — deterministic, bounded-model, or full-session — and a run cadence — per-change, scheduled, checkpoint, or on-demand — alongside its route, and the harness section shall state the per-run bound that keeps a scheduled run inside it. | the route says how a behavior is proven and the cost class says what proving it costs; declaring both makes the suite's economics a reviewable property of the design rather than a surprise for the build | Must | med | on the dogfood and the fixtures, every scenario carries exactly one cost class and one cadence, and the harness section states a per-run bound; a reviewer can derive the cost of a scheduled run from the doc alone; a scenario with no cost class or no cadence is a well-formedness defect | Inspection, Test | G1, J1, F1 |

### op: supersede FR-8

Reason: the two mandated lenses judge what the doc claims to prove and never judge whether the machine that proves it could be built — so a harness section that was only a library name passed review twice.

| ID | Requirement (EARS) | Rationale | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|
| FR-24 | The e2e-review lens set shall include lenses for coverage completeness, anti-gaming (a scenario reds if the behavior breaks; no mock hollows out what it proves), and buildability (the harness section answers every element it owes, and every scenario is buildable against the harness as specified). | each mandated lens maps to a real e2e risk and carries a proving eval (§5.9); buildability is the lens that would have caught a harness section carrying no mechanics; the set is named here, its wording refined in design | Must | high | the catch corpus (seeded coverage gaps, hollow scenarios, and harness holes) is flagged by the coverage, anti-gaming, and buildability lenses respectively (M2, M3, M4); a fully-covered fixture doc with honest mocks and a complete harness section yields no coverage, anti-gaming, or buildability finding | Test | G2, J2, F4 |

### op: supersede FR-16

Reason: the plan put every scenario task in the ready frontier at once, so parallel workers each invented their own scaffolding in the same package with nothing to reuse and no owner for any shared file.

| ID | Requirement (EARS) | Rationale | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|
| FR-25 | When e2e-plan is invoked on an approved e2e design doc, the skill shall decompose it into an ordered plan of per-scenario tasks, plus a conventions task landing the harness's scenario contract and one worked scenario — omitted only when the repo already carries a scenario built against that harness — plus a harness task only when the feature's e2e harness does not already exist, and project it onto the tracker as ordinary tasks. | the plan is the buildable output; ordinary tasks so the existing executor consumes them; the conventions task creates the shape parallel scenario workers copy instead of each inventing one; neither the conventions task nor the harness task is emitted when the repo already supplies it | Must | med | on the dogfood, e2e-plan produces one task per scenario plus a conventions task iff no scenario is already built against the harness plus a harness task iff no harness already exists, projected onto the tracker; the tasks validate against the existing task and charter schema; a fixture with a pre-existing harness and existing scenarios yields zero harness tasks and zero conventions tasks | Demo, Test | G1, J3, F3 |

### op: supersede FR-17

Reason: scenario tasks blocked only on the harness still started simultaneously against an empty package, so collisions on shared files were structural rather than accidental.

| ID | Requirement (EARS) | Rationale | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|
| FR-26 | Where e2e-plan emits a harness task or a conventions task, each scenario task shall carry an explicit blocked-by dependency on it, and where the repo already supplies either, scenario tasks shall carry no such dependency; each scenario task shall additionally name the one artifact it exclusively owns, taken from that scenario's normalization (FR-21), and no two scenario tasks in a plan shall name the same artifact. | harness and conventions before scenarios where either is built, so parallel workers reuse a landed shape rather than each inventing one; exclusive artifact ownership is what stops concurrent scenario tasks colliding, and the executor already builds unblocked-first so only the edges need declaring | Must | high | in a produced plan with a harness or conventions task, every scenario task is blocked-by it (native relation); in a plan where the repo supplies both, scenario tasks carry neither edge; every scenario task names exactly one owned artifact and no artifact is named twice; a scenario task with no owned artifact, or two tasks sharing one, is a plan defect | Test | G1, J3, F3 |

### op: add UB

Reason: with no rule against it, an unsettleable mechanical element would be papered over with a library name — the exact defect this amendment exists to close.

| ID | If… (trigger) | Then the system shall… | User sees | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|---|
| UB-8 | no existing harness fits the feature's primary external interface and some element of the mechanical design cannot be settled — for example no way to observe an outcome, or no way to isolate a run from its neighbours | record that element as an explicit blocking open item naming what is unresolved and why, and hold the design doc short of approval until it is settled or the scenarios that depend on it are re-routed; never let a library name or a tool name stand in for missing mechanics | the doc lists the unsettled element as a blocking open item; the doc cannot be approved while it stands | Must | high | the unsettleable-harness fixture produces a doc carrying the element as a blocking open item and does not reach approval; zero harness sections pass with an element answered only by a library name | Test | F1, FR-19 |

### op: add UB

Reason: a plan cannot assign exclusive ownership from scenarios that never named what they own, and emitting the tasks anyway reproduces the collisions this amendment closes.

| ID | If… (trigger) | Then the system shall… | User sees | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|---|
| UB-9 | e2e-plan is invoked on an approved e2e design doc whose scenarios do not each name the one artifact they exclusively own | refuse, naming the scenarios that carry no owned artifact — a plan that cannot assign exclusive ownership would emit tasks guaranteed to collide | refusal naming the un-normalized scenarios; zero tasks are projected | Must | med | the un-normalized-doc fixture yields refusal naming each offending scenario; zero tasks are projected | Test | F3, FR-21, FR-26 |

### op: add NFR

Reason: NFR-3 bounds what the authoring skills spend; nothing bounded or even disclosed what the suite they specify would cost to run.

| ID | Area | Requirement | Fit criterion | Verify |
|---|---|---|---|---|
| NFR-5 | Cost (the specified suite) | The e2e design doc shall carry the run cost of the suite it specifies as a reviewable property — the per-scenario cost class and cadence (FR-23) together with the harness section's stated per-run bound — so that the cost of the suite is judged at review time rather than discovered when it is built. | on the dogfood and the fixtures, a reviewer can derive the cost of one scheduled run from the design doc alone; every scenario carries a cost class and a cadence and the harness section states its per-run bound; a doc from which the scheduled-run cost cannot be derived is a review finding | Inspection, Analysis |

### op: add EVAL

Reason: FR-24 mandates a third lens, and this ledger mandates a lens only where a proving eval backs it.

| ID | Behavior | Eval set / harness | Threshold | Negative examples |
|---|---|---|---|---|
| EVAL-3 | the buildability lens flags a harness section a worker could not build from — an element of FR-19 unanswered, or answered only by naming a library or tool — and a scenario that cannot be built against the harness as specified | a corpus of e2e design docs with seeded harness holes (an unanswered element, a library-name-only section, a scenario naming a seam the harness does not carry, a scenario with no owned artifact) plus complete controls | every seeded harness hole flagged; zero buildability findings on the complete controls | a harness section answering all ten elements, with every scenario naming a seam the harness carries and one artifact it owns, yields no buildability finding |

### op: supersede EVT-2

Reason: the lens enum admits only two lenses, so a buildability finding could not be recorded at all.

| ID | Event name | Fires when | Properties (typed) | Source | Feeds |
|---|---|---|---|---|---|
| EVT-5 | review_finding | an e2e-review lens emits a finding | lens: enum(coverage·anti_gaming·buildability), class: string, disposition: enum(open·resolved·rejected) | e2e-review log | M2, M3, M4, §3.3 |

### op: supersede EVT-3

Reason: the case-family enum admits only two seeded families, so the buildability corpus could not be scored.

| ID | Event name | Fires when | Properties (typed) | Source | Feeds |
|---|---|---|---|---|---|
| EVT-6 | catch_corpus_run | the catch corpus is run at the checkpoint | case_family: enum(coverage_gap·hollow_scenario·harness_hole), case: string, flagged: bool | catch-corpus run | M2, M3, M4 |

### op: supersede DOD-3

Reason: the catch-corpus done row covered two seeded families and would have passed while the third lens was never scored.

| ID | Done criterion | Source | Verify | Required evidence | Blocking? |
|---|---|---|---|---|---|
| DOD-9 | The catch corpus is run at the catch-suite checkpoint and its catch-rate over the seeded coverage-gap, hollow-scenario, and harness-hole cases is recorded — a measurement feeding the reviewer's advisory-to-blocking ramp (§7.2 Q3), not a per-commit gate (the lenses are model-driven; per I5 it measures, never gates) | M2, M3, M4; FR-24; ABUSE-1/2 | Eval, Analysis | catch-corpus run record (EVT-6) plus the EVAL rows (§5.9) | no — recorded measurement |

### op: add DOD

Reason: the amendment's whole bet is that the doc now carries a machine somebody else can build, and that its plan can be built in parallel without collisions — both are checkable on the dogfood.

| ID | Done criterion | Source | Verify | Required evidence | Blocking? |
|---|---|---|---|---|---|
| DOD-8 | The dogfood's e2e design doc carries a stated answer under every FR-19 element — or an existing harness cited with only its delta (FR-20) — with the buildability lens returning no finding against it, and the plan cut from it names one exclusively-owned artifact per scenario task with no artifact named twice | FR-19, FR-20, FR-21, FR-26 | Demo, Inspection | the dogfood design doc, the well-formedness checker's clean result over its harness section, the buildability lens's finding-free verdict, and the plan's per-task owned-artifact list | yes |

### op: supersede §0

Reason: §0 routed the reviewer to the harness-ordering call and to a two-lens set, both of which this amendment changes; the §0 replacement is informative only and binds nothing (prd:read-to-rule/FR-6).

*The body is the contract; this section only routes attention — on any conflict the body governs; machines ignore it.*

**Judge these**

| # | The call | Could have gone | If you disagree |
|---|---|---|---|
| 1 | **v1 stops at the plan — no tests are built or run** — the line delivers a reviewed e2e design and an ordered plan of tracker tasks; building them is a named follow-on | v1 also runs implement-now to build the tests (accepting an unadapted executor) | "e2e verification" ships with nothing executing yet; the dogfood proves authoring, not passing tests → §3.4 NG-1, §4.2, D-14 |
| 2 | **Three dedicated new skills, not reuse of the existing pipeline skills** — e2e-design/e2e-review/e2e-plan parallel, rather than reuse, tech-design-new / the reviewer program / plan-new | reuse the existing skills, e2e-specifics as content inside them | more surface to build and maintain; the alternative would have been fewer skills carrying e2e content → §4.1, D-1 |
| 3 | **The design doc carries the machine, not just the coverage** — the harness section specifies ten mechanical elements (or cites an existing harness and states only its delta), and every scenario is restated in that harness's terms | name a harness and let whoever builds it decide the mechanics | the most reused code in the suite gets designed by one worker after review has closed, and no reviewer ever sees it → FR-19, FR-20, FR-21, UB-8 |
| 4 | **The coverage map and per-scenario route are mandatory doc sections, and each scenario also declares a cost class and cadence** | let the agent's judgment carry coverage, mocking, and cost implicitly | the review loses its yardstick, mocking-away-the-hard-part becomes invisible, and the suite's price is discovered at build time → FR-2, FR-4, FR-23, NFR-5 |
| 5 | **Coverage is derived from the feature's PRD + design + built code** — not from a structured definition-of-done | wait for the structured-DoD schema (not yet built) | a not-yet-built dependency would block v1; the structured DoD becomes a sharper input later → FR-3, §3.4 WR (build follow-on) |
| 6 | **The plan lands conventions before it fans out, and every scenario task owns exactly one artifact** — ordering still rides the existing plan and executor; only the edges and the ownership are declared | keep scenario tasks fully independent, each blocked only on the harness | parallel workers start against an empty package, each inventing scaffolding nobody reuses and colliding on shared files → FR-25, FR-26, UB-9 |
| 7 | **An agentic feature is tested with a specified decision-authority stand-in, never a live human** — scripted first, persona fallback, improvisations recorded, at least one adversarial persona, blind to the assertions | leave the stand-in to whoever builds the tests | a stand-in that only agrees, or that can see what the test asserts, manufactures a green → FR-22 |
| 8 | **The factory's own loop is P1; the operator is P2** — the design optimizes the machine's authoring loop; the operator reviews and accepts | operator-first segmentation | trade-offs tip toward human-facing polish over the loop the machine runs every feature → §2.2, D-11 |

**Sanity-check the numbers** — the e2e design doc has **six** sections; where a harness must be built its harness section answers **ten** mechanical elements; the review runs **three** lenses (coverage · anti-gaming · buildability), each backed by a proving eval (§5.9); every scenario carries **exactly one** route, **one** cost class, **one** cadence, and **one** exclusively-owned artifact. The dogfood demands **one real pyxis feature** carried design→review→plan to an accepted plan (M1); the catch tests demand **every** seeded coverage-gap, hollow scenario, and harness hole flagged (M2, M3, M4); the guardrail tolerates **zero** unresolved false-reds at the dogfood checkpoint (§3.3). Task counts and retry caps are not this document's numbers, and the lenses' exact wording is refined in design (§7.2).

**The author's open questions** — where the three artifacts live and whether they are new governed document kinds (§7.2 Q1, design-owned); the exact lens membership and wording (§7.2 Q7, design-owned); the unmanned reviewer's advisory-ramp/adoption checkpoint (§7.2 Q3, mirrors the existing program); how e2e-design picks the live design when a feature has multiple/superseded designs (§7.2 Q6, design-owned). None blocks the solution gate.

**Skip the rest** — structure/ID/fold correctness → docgate; substance and coherence → the unmanned `/prd-review`; document-home and schema mechanics → the technical design (§7.2 design-owned rows); building, driving-to-green, e2e anti-gaming at run time, and the harder app shapes → the deferred implement-now e2e adaptation; the dedicated no-regression gate and the incident→regression loop → their own follow-on features.

### op: supersede §3.1

Reason: G2 names the lens set the review runs, and this amendment adds a third lens to it.

| # | We believe that… | Will result in… | Measured by… |
|---|---|---|---|
| G1 | auto-authoring a feature's e2e tests as a *reviewed, planned* artifact (coverage map + scenarios + routes + the harness that runs them, reviewed, then decomposed) | covering, honest, buildable e2e plans the operator accepts — with the engineer only defining and accepting | M1 |
| G2 | reviewing the e2e design through blind lenses (coverage, anti-gaming, buildability) plus a human doc-review pass | coverage gaps, hollow or mocked-away scenarios, and harness designs nobody could build from all caught *before* anything is built | M2, M3, M4 |
| G3 *(qualitative)* | making the coverage map, the per-scenario route, and the harness design mandatory, reviewable doc sections | the operator trusting the e2e scope is complete, honest, and buildable — unprompted reliance on the coverage map and the harness section when accepting | coverage-map-consulted-in-acceptance observed in dogfood and post-launch review (§6.7) |

### op: supersede §3.2

Reason: the metric table carried a catch-rate per mandated lens, and the third lens needs its own.

| Metric | Goal | Signal | Baseline | Target (at checkpoint) | Feeds from |
|---|---|---|---|---|---|
| M1 (primary) | G1 | a real pyxis feature carried design → review → plan to an operator-accepted reviewed design doc + ordered plan | 0 (no stage-10 line exists) | 1 feature accepted at the dogfood checkpoint (the pyxis-feature dogfood run complete) | EVT-1, joined to the accepted plan |
| M2 | G2 | seeded coverage gaps (a required behavior with no scenario) flagged by the coverage lens | n/a (no reviewer exists) | every seeded coverage-gap case flagged, measured at the catch-suite checkpoint (a recorded score, not a per-commit gate) | EVT-5, EVT-6 |
| M3 | G2 | seeded hollow scenarios (a scenario that mocks away the behavior it names) flagged by the anti-gaming lens | n/a (no reviewer exists) | every seeded hollow-scenario case flagged, measured at the catch-suite checkpoint | EVT-5, EVT-6 |
| M4 | G2 | seeded harness holes (a mechanical element unanswered, a library-name-only harness section, a scenario naming a seam the harness does not carry, a scenario with no owned artifact) flagged by the buildability lens | n/a (no reviewer exists) | every seeded harness-hole case flagged, measured at the catch-suite checkpoint | EVT-5, EVT-6 |

### op: supersede §4.2

Reason: the scope perimeter enumerated the design doc's parts as ending in "harness libraries" and the plan as carrying only harness-to-scenario edges, both of which this amendment changes.

- **In (plan of record)** — the three skills: **e2e-design** (the e2e design doc with its six parts — scenarios, setup and teardown, mock strategy, harness design, coverage map, per-scenario route — derived from the feature's PRD + design + built code, app-shape-aware; the harness section specifying the ten mechanical elements where a harness must be built, or citing an existing harness and stating only its delta; each scenario restated in the harness's terms with its fixture, seams, and one exclusively-owned artifact; a decision-authority stand-in specified where the feature has humans in its flow; a cost class and cadence per scenario); **e2e-review** (the human doc-review half + an unmanned three-lens reviewer — coverage, anti-gaming, buildability — emitting closable findings in a review report; finding convergence — resolve or reject; the design freezes at approval); **e2e-plan** (an ordered, frozen plan of ordinary tracker tasks — per-scenario tasks, a conventions task and a harness task each emitted only when the repo does not already supply it, blocked-by edges onto both, and one exclusively-owned artifact named per scenario task — projected onto the tracker); a catch-test corpus (seeded coverage gaps, hollow scenarios, and harness holes) as permanent tests; an **agentic fixture** — a feature whose exercised flow requires human decisions — carrying the decision-authority stand-in FR-22 is verified against; the pyxis-feature dogfood proving design→review→plan.
- **Future (believed next, not committed)** — the waiting room: building/executing the tests (WR-1), the dedicated no-regression gate (WR-2), the incident→regression loop (WR-3), auto-firing (WR-4), hard-app-shape execution (WR-5); the structured-DoD as a sharper coverage input.
- **Out** — building, running, or gating on the tests, including building or running the specified decision-authority stand-in (NG-1, NG-2); the incident loop (NG-3); merge/landing authority (NG-4); touching the stage-8 harness (NG-5).

### op: supersede §5.3

Reason: the decision-authority stand-in is a new adversarial surface with no abuse row — a stand-in that only agrees, or that can see what the run asserts, manufactures a green; and every existing mitigation cell cited FR-8, superseded here by FR-24.

*No user roles or standing access surface ship: the skills are invoked by the operator; their outputs (design doc, review report, plan, tracker tasks) are human-reviewed content. The abuse table is the heart of this section — the adversary is the e2e-design skill optimizing to look complete, caught at design-review time (there is no execution surface to attack in this version).*

| ID | Actor and intent | Vector | Impact | Severity | Mitigated by | Verify |
|---|---|---|---|---|---|---|
| ABUSE-1 | e2e-design optimizing to look complete | writes a scenario that mocks away the very behavior it names (hollow scenario) | the coverage map claims coverage that proves nothing | high | FR-4 (per-scenario route recorded and visible) + FR-24 (anti-gaming/will-it-bite lens); design-time only — run-time anti-gaming is WR-1 (`brainstorm:e2e-verification#P-2`, `#P-3`) | catch corpus (hollow-scenario cases), measured at the checkpoint (M3) |
| ABUSE-2 | e2e-design optimizing to look complete | omits scenarios for required behaviors, leaving the feature under-covered | end-to-end gaps ship unproven | high | FR-2 (every required behavior mapped) + FR-24 (coverage lens) | catch corpus (coverage-gap cases), measured at the checkpoint (M2) |
| ABUSE-3 | e2e-design under CI-runnability pressure | over-uses the mocked/manual routes to avoid hard real runs, hollowing the suite | the suite looks routed but exercises little | med | FR-4 routes are recorded and reviewed; FR-24 anti-gaming lens; the human doc-review pass (FR-9); residual named — quantitative route-balance thresholds are a build-follow-on concern (WR-1) | Inspection (dogfood review) |
| ABUSE-4 | e2e-design optimizing to look complete | maps a scenario to required behaviors it does not actually exercise (over-mapping / coverage inflation) — honest about each scenario's own behavior, but extra map edges claim coverage the scenario never touches | the coverage map reads fully green while proving a fraction of what it claims | high | FR-2 (every behavior mapped) + FR-24 (coverage/anti-gaming lens); the per-map-edge exercise check — each behavior→scenario edge genuinely exercised, not just present — and its seeded catch case are design-owned (§7.2 Q2) | Inspection (dogfood review); per-edge check design-owned |
| ABUSE-5 | e2e-design specifying a decision-authority stand-in that will pass rather than one that will probe | specifies a stand-in that only ever agrees, that is handed the run's assertions, or that improvises an unscripted answer without recording it — so the agentic scenario reaches its end state whatever the feature did | the suite drives an agentic feature to green by supplying the answers that make it green, proving the stand-in cooperative rather than the feature correct | high | FR-22 (scripted-first, character-sheet-bound fallback, recorded improvisations with dependent assertions non-authoritative, at least one adversarial persona, assertion-blind) + FR-24 (buildability lens); design-time only — a stand-in's behavior under a real run is WR-1 | catch corpus (harness-hole cases incl. a cooperative-only and an assertion-sighted stand-in), measured at the checkpoint (M4) |

### op: supersede §5.4

Reason: the data table described the design doc's contents as ending in "harness libraries" and the plan as harness-plus-scenarios, and the tracker-task row cited a superseded requirement.

| Entity / field | Meaning | Source of truth | Retention | Integrity rules | Privacy class |
|---|---|---|---|---|---|
| e2e design doc | the reviewed artifact: scenarios, setup and teardown, mock strategy, the harness design (its ten mechanical elements, or a citation plus this feature's delta), coverage map, per-scenario routes with a cost class and cadence each | authored by e2e-design; a live draft through review | repo lifetime | frozen at approval (its exact home/kind is a design decision, §7.2 Q1); every scenario names its fixture, seams, and one exclusively-owned artifact (FR-21); built against downstream | internal |
| e2e review report | the unmanned reviewer's closable findings + dispositions for one design doc, across the coverage, anti-gaming, and buildability lenses | e2e-review | rides the design doc's change | closable findings; each resolved or rejected-with-reason (FR-10) | internal |
| e2e plan | the ordered, frozen plan of tracker tasks — per-scenario tasks plus a conventions task and a harness task where the repo does not already supply them, with blocked-by edges | e2e-plan | frozen at approval | mirrors the existing plan schema (FR-14); idempotent projection (FR-15) | internal |
| e2e tracker tasks | the projected issues implement-now (later) consumes | the tracker (projected by e2e-plan) | tracker lifetime | ordinary task/charter schema, no new type (FR-14); scenario tasks blocked-by the conventions and harness tasks where emitted, each naming one exclusively-owned artifact, no artifact named twice (FR-26) | internal |

- **Migration** — none: all entities are new.
- **Parallel-run / cutover** — none: the line is additive; nothing pre-existing is replaced (NR-1, NR-2).
- **Rollback** — the artifacts are inert documents and tracker tasks; not invoking the skills leaves the pipeline exactly as before.
- **Decommission** — n/a for v1 (no persistent services).
- **Privacy check** — no personal data; no DPIA trigger.
- **Egress** — the skills read repo content and (e2e-plan) write tasks to the repo's configured tracker; no other content leaves the environment. The e2e-design skill is model-driven (§5.9): the prompt content it sends to a model is the feature's own repo artifacts (PRD, design, code) — the same egress class as the existing authoring/review skills, governed by their conventions.

### op: supersede §6.1

Reason: Phase A never committed the agentic fixture FR-22 is verified against, and its exit criterion cited DOD-3, superseded here by DOD-9.

| Phase | Purpose | Scope | Exit criterion |
|---|---|---|---|
| A — skills + fixtures | prove the three skills on controlled ground | e2e-design, e2e-review (lenses + report), e2e-plan; the catch corpus (coverage-gap, hollow-scenario, and harness-hole cases); multi-app-shape fixtures; the agentic fixture carrying the decision-authority stand-in (FR-22) | the fixture-verifiable Must FRs and DOD-6 green in CI; the catch-corpus catch-rate (DOD-9) recorded |
| B — dogfood | prove the line on a real feature | design → review → plan on one pyxis feature to an accepted plan | M1 met + guardrails clean (DOD-2, DOD-4); NR rows verified (DOD-5); the harness and ownership bar met (DOD-8) |

### op: supersede §8

Reason: the readiness gate read ✓ on cells this amendment falsified — the high-risk cell named two superseded rows and omitted the five high-risk rows 002 adds, the job and flow traceability cells indexed superseded IDs, and the metric and DoD cells cited superseded events and a superseded DoD row.

| Check | Pass? | Notes |
|---|---|---|
| Problem basis accepted — §1 problem accepted (T1); §2 problem gate signed (T2/T3) | ☐ | pending the introducing review session (§2.6) |
| Zero solution-blocking TBDs or open questions (§7.2) | ✓ | Q1, Q3, Q6 and Q7 (which supersedes Q2) design-owned with product bars fixed in §5; Q4/Q5 deferred by ruling |
| Every Must FR has a fit criterion and a verification method (§5.1) | ✓ | every §5.1 Must row carries both, the nine rows 002 adds included |
| Every requirement row carries a risk tier; high/critical rows have a declared safety plan (§5.1, §6.5) | ✓ | no critical rows (no execution/merge surface in v1); the live high rows are FR-2/4 (coverage explicit and honesty-routed — the review lenses + catch corpus + human acceptance), FR-19 (the ten mechanical elements — the well-formedness check + the buildability lens + UB-8's blocking open item), FR-22 (the stand-in — the five specified properties + ABUSE-5 + the buildability lens), FR-24 (the three-lens set — each lens backed by a proving eval, §5.9), FR-26 (blocked-by edges + exclusive artifact ownership — a plan defect is a test failure, and UB-9 refuses an un-normalized doc), UB-3 (no silent drop) and UB-8 (no library name standing in for missing mechanics); kill switch = stop invoking (§6.5) |
| Non-goals drawn (§3.4); non-regression rows named (§5.1) | ✓ | NG-1..5; NR-1..3 |
| Every primary job (§2.3) is covered by a requirement, a non-goal, or a waiting-room entry | ✓ | J1→FR-18/2/3/4 + FR-19/20/21/22/23; J2→FR-7/24/9/10; J3→FR-25/26/14/15; J4→FR-5/9 + operator acceptance |
| Every key flow (§4.3) has functional, failure, and non-regression coverage (§5.1–5.2) | ✓ | F1→FR-18/2/3/4/19/20/21/22/23 + UB-1/2/3/7/8; F2→FR-7/24/9/10/11 + UB-5; F3→FR-25/26/14/15 + UB-4/9; F4→FR-4/24 + ABUSE-1/2/4/5 |
| Every primary metric (§3.2) has instrumentation (§5.7) or an explicit measurement plan | ✓ | M1←EVT-1 joined to the accepted plan; M2/M3/M4←EVT-5/EVT-6; guardrails←EVT-4 |
| Every blocking DoD row traces to a requirement, guardrail, release criterion, or human-judgment rubric (§5.8) | ✓ | DOD rows each cite their source column; all blocking rows sourced, DOD-8 included (FR-19/20/21/26) |
| Risks and assumptions have owners (§6.3) | ✓ | R1–R6, A1 owned |
| Human-judgment criteria name an owner and a rubric (§5.8) | ✓ | no human-judgment DoD rows — DOD-8's evidence is the well-formedness checker's result, the buildability lens's verdict, and the plan's owned-artifact list, all mechanical; FR-5 requires a named owner on manual-routed scenarios |
| DoD complete: every blocking row sourced and evidenced; every n/a explained (§5.8) | ✓ | DOD-9 is a recorded checkpoint measurement (model-driven catch-rates measure, not gate — I5); all other DOD rows blocking and sourced; §6.6 n/a rows carry their reason |

### op: supersede Q2

Reason: the open question described the lens set as the two dimensions named in FR-8; this amendment makes it three, named in FR-24, so the question's own text contradicted the contract it defers to design.

| # | Question | Owner | Needed by | Status |
|---|---|---|---|---|
| Q7 | The exact e2e-review lens wording (the set — coverage · anti-gaming · buildability — is named in FR-24; refinement is design's) (`#D-9`) | technical design | design handoff | open — design-owned; the three dimensions are fixed, the phrasing is not |

### op: supersede §6.3

Reason: R1's mitigation cited FR-8, superseded here by FR-24, and R2's and R5's cited only the two evals and the two lenses this amendment extends to three.

| # | Risk / assumption / rabbit hole | Likelihood | Impact | Mitigation or test | Owner |
|---|---|---|---|---|---|
| R1 | Auto-authored coverage is shallow — e2e-design misses behaviors or writes thin scenarios | M | H | the coverage lens + human doc-review (FR-24, FR-9); the dogfood measures it (M1); FR-2 forces an explicit map to check against | @mdarabi |
| R2 | The lenses cry wolf; the operator learns to wave findings through | M | H | the false-red guardrail with same-run dispositions (§3.3); catch corpus gates the lenses' own correctness (EVAL-1/2/3) | @mdarabi |
| R3 | v1 ships an authoring line whose tests never run in this version; value depends on the build follow-on landing | M | M | scoped deliberately (`#D-14`); the plan/tasks are the accepted deliverable and the follow-on's direct input; the boundary is stated in §1/§0/NG-1 | @mdarabi |
| R4 | The three artifacts' home/kind is unsettled and churns in design | L | M | routed to the tech design as a named question (§7.2 Q1, `#P-4`); v1 intent recorded (frozen-at-approval, built-against) | @mdarabi |
| R5 | The mock route hollows the suite — over-mocking passes review | M | M | routes recorded per scenario and reviewed (FR-4, ABUSE-3); the anti-gaming lens; quantitative route-balance is a build-follow-on concern (WR-1) | @mdarabi |
| R6 | The harness section is answered element-by-element but shallowly — ten stated answers that still leave a worker guessing | M | H | the buildability lens with its own proving eval (FR-24, EVAL-3); UB-8 forbids a library name standing in for mechanics; DOD-8 requires the lens finding-free on the dogfood | @mdarabi |
| A1 | *Assumption (from §2.4, Med):* the feature's PRD + design + built code carry enough to derive good e2e coverage without a structured DoD | M | M | the dogfood tests it; if coverage is thin, the structured DoD (a later sharper input) is the named upgrade path | @mdarabi |
