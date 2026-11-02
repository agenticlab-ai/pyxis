# e2e authoring skills — e2e-design · e2e-review · e2e-plan behavior — technical design

| Field | Value |
|---|---|
| **Status** | Approved *(2026-07-24 — operator accepted, operator review waived; unmanned tech-design-review: approve-with-advisories. Merge is the freezing event; "superseded" is computed by the index.)* |
| **Owner** | @mdarabi |
| **Approver(s)** | @mdarabi (operator — named reviewer required: Security/privacy impact is yes) |
| **Date** | 2026-07-24 |
| **Relevant-when** | designing or changing the three e2e authoring skills (e2e-design · e2e-review · e2e-plan) — coverage derivation, governing-design-set selection, app-shape harness/route choice, the review lens set + convergence, the plan decomposition and its dependency edges; implementing or superseding the skills' behavior |
| **Scope** | The **behavior** of the three e2e authoring skills over the artifact contract (`design:e2e-verification-artifacts`): **e2e-design** (coverage derivation from PRD + live design set + built code, governing-design-set selection, app-shape harness/route choice, the bounded read, the refusals and the no-automatable-surface stop); **e2e-review** (reuse of the reviewer program — context pack + blind fresh-context lenses + convergence — the coverage & anti-gaming lenses, blocking/advisory findings, the human review half); **e2e-plan** (authoring a plan-new `plan/v1` — per-scenario tasks + a conditional harness task with `blocked-by` edges). **Not**: the artifact schemas / home / lifecycle (`design:e2e-verification-artifacts`); building or running the tests (`prd:e2e-verification/WR-1`); the reviewer's downstream-gate authority (deferred, §7 Q3). |
| **Tracking** | — *(the introducing PR)* |
| **Builds-on** | e2e-verification-artifacts (the artifact contract these skills produce/consume) · impl-review-v1 (the reviewer-program runtime reused) · context-pack-v1 (the shared context pack reused) · plan-new-v1 (the plan pipeline reused) · implement-now (the executor frontier the plan's edges feed) |
| **Supersedes** | — |
| **Serves** | prd:e2e-verification/FR-1 · prd:e2e-verification/FR-3 · prd:e2e-verification/FR-6 · prd:e2e-verification/FR-7 · prd:e2e-verification/FR-8 · prd:e2e-verification/FR-9 · prd:e2e-verification/FR-10 · prd:e2e-verification/FR-16 · prd:e2e-verification/FR-17 · prd:e2e-verification/UB-1 · prd:e2e-verification/UB-2 · prd:e2e-verification/UB-3 · prd:e2e-verification/UB-4 · prd:e2e-verification/UB-5 · prd:e2e-verification/UB-7 · prd:e2e-verification/NR-1 · prd:e2e-verification/NR-2 · prd:e2e-verification/NR-3 · prd:e2e-verification/NFR-1 · prd:e2e-verification/NFR-2 · prd:e2e-verification/NFR-3 · prd:e2e-verification/NFR-4 |
| **Security/privacy impact** | yes — model/provider egress: the skills send the feature's own repo artifacts (PRD, technical design, code, the e2e design doc) to a model — the same egress class as the existing authoring/review skills, no new destination · reviewer: @mdarabi |
| **Template version** | tech-design-template/v1 |

## 0. Read to rule — the human reviewer's section (~5 min)

*The body is the contract; this section only routes attention — on any conflict the body governs; machines ignore it.*

**Judge these**

| # | The call | Could have gone | If you disagree |
|---|---|---|---|
| 1 | **e2e-review is the fourth instance of the reviewer program**, reusing its context pack + blind lenses + convergence (§3.2) | a parallel e2e-specific reviewer (§4) | you'd re-invent proven review machinery and its independence guarantees → §3.2 · §4 |
| 2 | **e2e-design reads the live design *set*, not a single design** — refusing only on genuine ambiguity (§3.1) | refuse whenever >1 live design (the split pattern is normal) | you'd refuse on the split-design dogfood feature itself → §3.1 · #DINV-8 · A-9 |
| 3 | **No automatable e2e surface → e2e-design produces nothing and stops the pipeline** (§3.1) | author a manual-only design + checklist (the pre-amendment UB-6) | you'd emit artifacts nobody asked for instead of a clean stop → §3.1 · #DINV-7 · `prd:e2e-verification/UB-7` |
| 4 | **The harness task is conditional; blocking findings block approval** (§3.2 · §3.3) | always a harness task; advisory-only review | you'd build a redundant harness or ship a toothless review → §3.3 · #DINV-4 · #DINV-5 |
| 5 | **No design-time genuine-exercise check; app-neutral skill code** (§3.1 · §3.2) | a design-time per-edge exercise check; per-shape branches | you'd add a check that can't verify a built test bites, or tie the skills to a shape set → §3.2 · #DINV-2 |

**Sanity-check the numbers** — **two** review lenses (coverage · anti-gaming, §3.2); the bounded code read + the convergence round cap are **enforced** — a run reaching a cap stops (A-7); e2e-design **refuses and produces nothing** when any of PRD / live design set / built code is absent (#DINV-1), and likewise **produces nothing and stops** when there is no automatable e2e surface (#DINV-7).

**The author's open questions** — §7 Q1 (the exact read/round caps — tuned, enforced), Q2 (the exact lens prompt wording — the two dimensions are fixed), Q3 (the reviewer's advisory→blocking checkpoint — engages once a gate exists).

**Skip the rest** — the artifact schemas / home / lifecycle → `design:e2e-verification-artifacts`; structure/schema conformance → docgate; substance/reuse honesty → the unmanned `tech-design-review`; building or running the tests → the deferred build follow-on (`prd:e2e-verification/WR-1`).

## 1. Problem

The artifact contract (`design:e2e-verification-artifacts`) fixes *what* the three e2e artifacts are; nothing yet decides the **skills' behavior** that produces and consumes them — how e2e-design derives coverage and picks a harness, how e2e-review reviews the design and what its lenses are, how e2e-plan decomposes and orders the work. The merged PRD (as amended by 001) binds the behavior contract; the stage-3 brainstorm grilled and gated a **reuse-heavy** direction (`design-brainstorm:e2e-verification-tech-design`, ready-for-design). Decided wrong, the skills either re-invent proven machinery (the reviewer program, the planner, the executor's dependency ordering) or leak the review's independence by seeding a lens with the author's reasoning. The cheapest moment is now: the artifact contract is fresh and every reused mechanism is proven and stable.

## 2. Decision

**Decision:** The e2e authoring line is **three separate skills** — `/e2e-design`, `/e2e-review`, `/e2e-plan` — each its own command and a **single invocation**, reusing existing machinery **under the hood** (`design-brainstorm:e2e-verification-tech-design#D-3`, `#D-5`):

- **e2e-design** derives the coverage checklist from the feature's PRD (the requirement backbone) + its **live technical-design set** + the built code — refusing and producing nothing if any is absent, and **producing nothing and stopping the pipeline** when there is no automatable e2e surface (`prd:e2e-verification/UB-7`) — and otherwise authors the `e2e-design/v1` doc **app-shape-aware**, carrying no shape-specific code (`#D-6`, `#D-7` as refined at review by `#D-14`, `#D-9`, `#D-10`).
- **e2e-review** is the **fourth instance of the reviewer program**, reusing the context pack + blind fresh-context lenses + convergence, running **two** lenses — coverage and anti-gaming — whose **blocking** findings must be settled before approval, alongside the human review in the conversation (`#D-3`, `#D-4`, `#D-12`).
- **e2e-plan** authors an ordinary plan-new `plan/v1` from the approved doc — per-scenario tasks plus a **conditional** harness task with `blocked-by` edges — and hands it to plan-new's pipeline, whose ready-frontier orders harness-before-scenarios for free (`#D-5`, `#D-10`; `prd:e2e-verification/FR-16`, `FR-17`).

The reasoning: the three authoring steps carry enough e2e-specific structure to warrant their own commands, while every heavy mechanism they need already exists and is proven — so reuse under the hood keeps new surface small and the operator's experience is "one call per skill, done."

## 3. The design

### 3.1 e2e-design — coverage, the live design set, harness, bounded read, refusals

**Decision:** e2e-design reads the feature's **PRD requirement list** (the coverage backbone), its **live technical-design set**, and the **built code**, and authors the `e2e-design/v1` doc. It **refuses and produces nothing** if any input is absent — no PRD or live design → refuse naming the input (`prd:e2e-verification/UB-2`); not-yet-implemented (no built code) → refuse naming the unlanded work (`prd:e2e-verification/UB-1`) — and it **produces nothing and signals the operator to stop the pipeline** when the feature has no meaningfully-automatable e2e surface (`prd:e2e-verification/UB-7`) (`#D-6`).

- **Coverage derivation** (`prd:e2e-verification/FR-3`) — the required-behavior list is anchored to the feature's PRD requirement rows (the backbone), topped up by behaviors from the live design set's binding elements and the built code; each required behavior maps to ≥ 1 scenario or an explicit routed disposition (the coverage-map *structure* is `design:e2e-verification-artifacts` §3.1). No structured definition-of-done is read.
- **Governing-design-set selection** (`#D-14`; `prd:e2e-verification/UB-2`, `§7.2 Q6`) — a feature may legitimately have **several live technical designs** (the sanctioned artifacts+behavior split — this very feature has two). e2e-design reads the **live (non-superseded) set** of designs serving the feature, following the generated design index's liveness (computed from supersession edges), and derives coverage from the whole set. It **surfaces or refuses only on genuine ambiguity** — a *superseded* design present with no clear live successor, or liveness indeterminable — never merely because more than one design is live (`#DINV-8`).
- **No automatable surface** (`prd:e2e-verification/UB-7`) — when every required behavior routes to manual, or the feature is pure-doc/refactor with no end-to-end behavior to exercise, e2e-design produces **no e2e documents** and signals the operator (naming the reason) that the downstream e2e steps must not run — a clean stop, never a fabricated or manual-only artifact (`#DINV-7`).
- **App-shape harness/route** (`prd:e2e-verification/FR-6`, `NFR-1`, `#D-9`) — the skill reasons about the feature's primary external interface (web HTTP endpoint / CLI argv+stdout / library public API) and names a fitting harness + mock approach **as authored content**, carrying no app-shape-specific code (`#DINV-2`). Each scenario is routed run-for-real / mocked / manual-human; an unroutable *individual* behavior (in a feature that otherwise has an automatable surface) is **surfaced as an open item, never dropped** (`prd:e2e-verification/UB-3`).
- **Bounded read** (`prd:e2e-verification/NFR-3`, `#D-10`) — e2e-design reads a bounded slice (the feature's changed files + the modules the design set names), never the whole tree; the cap is enforced (a run reaching it stops) with the exact number tuned (§7 Q1).

### 3.2 e2e-review — the reviewer-program reuse, the two lenses, convergence

**Decision:** e2e-review is a **new separate skill** that stands up as the **fourth instance of the reviewer program** (`#D-3`): it reuses `context-pack-v1`'s machinery via a new stage declaration — mapping the e2e design doc onto the admitted **frozen-subject** kind and the feature's PRD + live design set onto **governing-peer** kinds, all existing admitted kinds, **no enum change** (the impl-review precedent) — and reuses the sibling-session **blind fresh-context lens** runtime and the scope-locked two-round convergence, emitting an `e2e-review-report/v1` report (`design:e2e-verification-artifacts` §3.2).

- **The two lenses** (`prd:e2e-verification/FR-8`, `#D-4`) — **coverage** (every required behavior has a genuine scenario; flags a missing scenario and a scenario obviously unrelated to a behavior it is mapped to — gross over-mapping) and **anti-gaming** (flags a written mock strategy that guts the scenario it names — visible because the per-scenario route/mock is recorded). Lenses live as **data** (a calibration table + the lens prompts), so adding or tuning one touches no other lens (`prd:e2e-verification/NFR-4`, `#DINV-3`).
- **The genuine-exercise boundary** (`#D-4`; `prd:e2e-verification/ABUSE-4`) — there is **no dedicated design-time mechanism** for verifying a coverage-map edge is genuinely exercised: whether a built test actually fails when the code breaks can only be confirmed by running it against broken code (falsification), which the basis defers to the run-time build follow-on (`prd:e2e-verification/WR-1`, `ABUSE-1`). Design-time catches only gross over-mapping and self-contradicting mock plans (`design-brainstorm:e2e-verification-tech-design#O-1`).
- **Blocking / advisory convergence** (`prd:e2e-verification/FR-10`, `UB-5`, `#D-12`) — each finding carries a severity (**blocking** or **advisory**) and a recommendation/acceptance condition; the review→converge loop runs until every finding is **settled** — a blocking finding must be solved in the design (or explicitly rejected with a stated reason), an advisory finding resolved or rejected — and **no design with an open finding advances to e2e-plan** (`#DINV-4`).
- **The human half** (`prd:e2e-verification/FR-9`) — the operator's comments on the design document in conversation run **alongside** the unmanned lenses, both feeding one convergence loop.
- **Reviewer-level authority** (`#D-12`) — the reviewer is **advisory in v1**: it forces no downstream automated gate (none exists; the operator is the approval point). That advisory→blocking authority graduates like the other reviewers once a build/gate enforcement surface exists (§7 Q3); the model-driven catch corpus **measures**, never gates (I5).

### 3.3 e2e-plan — a plan-new plan/v1 with a conditional harness task

**Decision:** e2e-plan authors an ordinary plan-new **`plan/v1`** from the **approved** e2e design doc — **per-scenario tasks** plus, **only when the feature's e2e harness does not already exist**, a **harness task** (`prd:e2e-verification/FR-16`); each scenario task carries a native `blocked-by` edge on the harness task **when one is emitted**, and **none when the harness already exists** (`prd:e2e-verification/FR-17`) — and hands the plan to plan-new's pipeline (fold → plan-PR → merge-freeze → `plangate project`). It **refuses on an unapproved** e2e design doc (`prd:e2e-verification/UB-4`) (`#D-5`, `#D-10`).

The declared edge is all that is needed: the executor's existing **ready-frontier** (open ∧ unblocked ∧ unclaimed) builds harness-before-scenarios for free and never bases a scenario on an unmerged harness; because the plan is an ordinary plan-new `plan/v1`, freeze-at-merge, idempotent projection, and the task/charter schema are **reused verbatim** and **implement-now is unchanged** (`design:e2e-verification-artifacts` §3.2; `prd:e2e-verification/FR-14`, `NR-1`, `NR-2`, `#DINV-5`).

### 3.4 Skill separation and single invocation

**Decision:** each of the three skills is its **own user-facing command and a single invocation**; every reuse of existing machinery is **internal** — the operator never separately invokes a reused skill or tool (`#D-3`, `#D-5`). One `/e2e-design`, one `/e2e-review`, one `/e2e-plan` call each does that skill's whole job.

### 3.5 Boundaries and non-goals

- **Not solved here** — the artifact schemas / home / lifecycle (`design:e2e-verification-artifacts`).
- **Owned by a future design / feature** — building or running the tests, and run-time anti-gaming (`prd:e2e-verification/WR-1`); the feature-evolution / e2e-test-delta capability (`design-brainstorm:e2e-verification-tech-design#O-2`); the reviewer's downstream-gate authority (§7 Q3, engages once a gate exists).
- **Implementation must not infer** — that e2e-review edits the reviewed doc (it raises findings; the author/operator resolves them); that the reused context pack shares judgment (input-only, finding-blind, no authoring-trail kind — `#DINV-3`); that the executor or plan-new is changed (`#DINV-5`); that a blocking finding is un-rejectable (it must be **settled** — solved or reasoned-rejection — never left open); that the no-automatable-surface case yields any artifact (it produces nothing and stops — `#DINV-7`).
- **Must remain unchanged** — the existing pipeline skills and reviewer program (`prd:e2e-verification/NR-1`); implement-now and plan-new's pipeline (`NR-2`); the repo's existing required CI contexts (`NR-3`).

### 3.6 Invariants

- **DINV-1** — e2e-design refuses and produces nothing when the PRD, the live technical-design set, or the built code is absent (`prd:e2e-verification/UB-1`, `UB-2`).
- **DINV-2** — e2e-design carries no app-shape-specific code; every app-shape choice is authored content in the doc, so a new app shape needs no skill-code change (`prd:e2e-verification/NFR-1`).
- **DINV-3** — e2e-review's lens subagents are blind to each other and to the authoring trail: the context pack is input-only over admitted kinds and admits no authoring-trail kind, so a lens is never seeded with the answer key (reused context-pack independence; `prd:e2e-verification/FR-7`).
- **DINV-4** — a blocking finding must be settled (solved in the design, or explicitly rejected with a stated reason) before the design is approved; no design advances to e2e-plan with an open finding (`prd:e2e-verification/FR-10`, `UB-5`).
- **DINV-5** — the plan is an ordinary plan-new `plan/v1`: scenario tasks carry a `blocked-by` edge on the harness task when one is emitted (and none when the harness already exists), the tasks use the ordinary task/charter schema with no new type, and the executor's + plan-new's pipelines are consumed unchanged (`prd:e2e-verification/FR-16`, `FR-17`, `FR-14`, `NR-2`).
- **DINV-6** — e2e-plan refuses on an unapproved e2e design doc (`prd:e2e-verification/UB-4`).
- **DINV-7** — nothing is silently dropped or fabricated: an unroutable *individual* behavior is surfaced as an open item (`UB-3`), and a feature with **no automatable e2e surface** yields **no e2e documents** and a stop-the-pipeline signal — never a fabricated or manual-only artifact (`prd:e2e-verification/UB-7`).
- **DINV-8** — e2e-design reads the live (non-superseded) design *set* serving a feature; it surfaces or refuses only on a superseded design with no clear live successor or indeterminable liveness — never merely because more than one design is live (`prd:e2e-verification/UB-2`, `§7.2 Q6`).

### 3.7 Observability

- **Events/logs** — the skills emit the PRD's tracking plan (`prd:e2e-verification/§5.7`): `e2e_line_step`, `review_finding`, `catch_corpus_run`, `finding_disposition`; logged via `log/slog` with the run context. The **fuel-bearing outputs are retained in git as committed, append-only records** (AP-28): the review report (`review.md`, carrying findings + dispositions) and the catch-corpus run record accrete into the corpus the advisory→blocking ramp reads — never only ephemeral logs.
- **Metrics** — M1 (a feature carried design→review→plan to an accepted plan), M2/M3 (the coverage/anti-gaming catch-rates at the checkpoint) — computed from the retained records, surfaced at the checkpoint, not per-commit gates (I5).
- **Alerts** — none intrinsic (invoked skills, no daemon); a stuck review is bounded by the convergence round cap (`prd:e2e-verification/NFR-3`) and surfaced as a non-terminating-run signal.
- **Debugging surface** — the artifacts are self-describing: the design doc's coverage map + routes show what was decided; the review report's findings + dispositions show what the lenses caught and how each was settled.
- **Trace/correlation** — each skill run carries its invocation context threading the run to its feature and artifacts.

### 3.8 Security & privacy

Matches the header row: **yes**. (1) **Model/provider egress** — e2e-design and e2e-review make model calls carrying the feature's own repo artifacts (PRD, technical design, code, the e2e design doc under review); this is the **same egress class as every existing authoring/review skill**, no new destination, and the content is the operand's own repo (`prd:e2e-verification/§5.4`). (2) **No standing access surface** — the skills are operator-invoked and hold no roles or credentials; their outputs (design doc, review report, plan, tracker tasks) are human-reviewed content (`prd:e2e-verification/§5.3`). (3) **Review independence** — e2e-review's lenses are finding-blind over an **input-only** context pack that admits no authoring-trail kind (the reused context-pack independence boundary, `#DINV-3`), so a lens is never handed the author's reasoning as an answer key. No personal data; no secrets. Mitigations trace to `#DINV-3` and the reused context-pack boundary.

## 4. Alternatives considered

| Alternative | Why rejected |
|---|---|
| **A parallel e2e-specific reviewer** (not reusing the reviewer program) | re-invents proven machinery (context pack, blind lenses, convergence, the report family) and its independence guarantees; the PRD mandates adopting the existing shapes (`prd:e2e-verification/FR-7`); more surface to build and maintain (`design-brainstorm:e2e-verification-tech-design#D-3`) |
| **A new ordering mechanism** for harness-before-scenarios | plan-new already wires `blocked-by` edges and the executor already builds unblocked-first and never off an unmerged blocker — only the edge need be declared; a new mechanism would duplicate existing machinery and touch implement-now (`#F-9`, `#D-10`) |
| **App-shape-specific skill code** (per-shape branches/adapters in the skill) | ties the skills to an enumerated shape set and needs code per new shape; carrying shape choices as authored content keeps them app-neutral by construction (`prd:e2e-verification/NFR-1`, `#D-9`) |
| **A dedicated design-time genuine-exercise check** (the over-mapping mechanism) | genuine exercise can only be confirmed by running the built test against broken code (falsification), deferred to `prd:e2e-verification/WR-1`; a design-time version would be theatrical (`#D-4`) |
| **A manual-only design + checklist for the no-automatable-surface case** (the pre-amendment UB-6) | emits artifacts nobody asked for; the amended `prd:e2e-verification/UB-7` has e2e-design produce nothing and stop the pipeline cleanly (`design-brainstorm:e2e-verification-tech-design#P-2`) |
| **Always emit a harness task** (the pre-amendment FR-12/FR-13) | rebuilds a harness that may already exist; the amended `FR-16`/`FR-17` make the harness task conditional (`design-brainstorm:e2e-verification-tech-design#P-3`) |
| **One combined e2e skill** (or a multi-step invocation chain) | blurs the user-facing boundary and the reuse-under-the-hood property; each step is a distinct command with a single invocation (`#D-3`, `#D-5`) |

## 5. Risks

| ID | Risk | Impact | Mitigation | Verification / evidence |
|---|---|---|---|---|
| R1 | auto-authored coverage is shallow — e2e-design misses behaviors or writes thin scenarios | end-to-end gaps ship unproven | coverage anchored to the PRD requirement list (`FR-3`) + the coverage lens + the human review; the dogfood measures it; the explicit coverage map is the yardstick | A-1, A-3; the dogfood (M1) |
| R2 | the lenses cry wolf; the operator learns to wave findings through | red-blindness | the false-red guardrail (`prd:e2e-verification/§3.3`); the catch corpus gates the lenses' own correctness as a measurement | A-3 (EVAL-1/2) |
| R3 | the reused context pack leaks the authoring trail into a lens, collapsing independence | the review's blindness becomes theater | the context-pack independence boundary (admitted-kinds allowlist, no authoring-trail kind), reused and asserted here (`#DINV-3`) | the independence property test |
| R4 | *(from `#P-1`, proceed-anyway)* a feature changes later and its existing e2e tests go stale | stale coverage until re-authored | out of v1 by ruling — the feature-evolution / delta capability is a deferred follow-on (`#O-2`); v1 is first-time authoring | boundary stated (§3.5); follow-on named |
| R5 | over-mocking hollows the suite — subtle cases invisible at design time | a routed-looking suite that exercises little | routes recorded per scenario and reviewed (`FR-4`) + the anti-gaming paper check; the deep bite-check is the deferred run-time falsification (`WR-1`) | A-3; dogfood review |

## 6. Acceptance

| ID | Acceptance criterion | Verify | Required evidence | Traces to |
|---|---|---|---|---|
| A-1 | e2e-design produces a valid `e2e-design/v1` doc from the feature's PRD + live design set + built code on the fixtures; it refuses and produces zero docs when any input is absent | Test | fixture-suite results | `prd:e2e-verification/FR-1` · `FR-3` · `UB-1` · `UB-2` · `#DINV-1` |
| A-2 | across fixtures of ≥ 2 app shapes e2e-design produces a valid design and the harness choice lives in authored content; inspection confirms no per-shape branch in skill code | Inspection, Test | multi-shape fixtures + code inspection | `prd:e2e-verification/FR-6` · `NFR-1` · `#DINV-2` |
| A-3 | e2e-review runs the two blind lenses over the reused context pack and emits an `e2e-review-report/v1` report; every seeded coverage-gap and hollow-scenario case is flagged by the coverage and anti-gaming lens respectively, and clean controls yield no finding | Test, Eval | catch-corpus run record | `prd:e2e-verification/FR-7` · `FR-8` · `ABUSE-1` · `ABUSE-2` · `EVAL-1` · `EVAL-2` |
| A-4 | a blocking finding blocks approval until settled; an e2e design doc with an open finding does not advance to e2e-plan | Test | open-finding fixture | `prd:e2e-verification/FR-10` · `UB-5` · `#DINV-4` |
| A-5 | e2e-plan authors an approved doc into a plan-new `plan/v1` — per-scenario tasks plus a harness task iff no harness exists, each scenario `blocked-by` an emitted harness task and none when the harness pre-exists — and refuses on an unapproved doc | Test | plan fixtures (fresh-harness + pre-existing-harness) | `prd:e2e-verification/FR-16` · `FR-17` · `FR-14` · `UB-4` · `#DINV-5` · `#DINV-6` |
| A-6 | implement-now, plan-new's pipeline, and the existing pipeline skills / reviewer program are byte-unchanged by this feature, and the existing required CI contexts stay green | Inspection, Test | diff inspection + CI | `prd:e2e-verification/NR-1` · `NR-2` · `NR-3` |
| A-7 | e2e-design's bounded read and e2e-review's convergence round cap are enforced — a run reaching either cap stops rather than fanning out | Analysis, Test | run-log analysis + cap fixture | `prd:e2e-verification/NFR-3` |
| A-8 | the operator's comments and the unmanned findings converge in one loop on the dogfood; the design reflects both | Demo | dogfood run record | `prd:e2e-verification/FR-9` |
| A-9 | on a feature with **two live designs** e2e-design reads the whole live set (no refusal); it surfaces or refuses only on a superseded design with no clear successor or indeterminable liveness | Test | multi-live-design + superseded-design fixtures | `prd:e2e-verification/UB-2` · `#DINV-8` · `design-brainstorm:e2e-verification-tech-design#D-14` |
| A-10 | a no-automatable-surface feature (all-manual, or pure-doc) yields **zero e2e documents** and a stop-the-pipeline signal naming the reason — never a fabricated or manual-only artifact | Test | no-surface fixtures | `prd:e2e-verification/UB-7` · `#DINV-7` |
| A-11 | this document passes `docgate lint-tech-designs` (once the design-class gate exists) and ships with the regenerated index | Test | gate output in the introducing PR | conventions §3.2 |

## 7. Open questions

| ID | Question | Owner | Blocks | Status | Resolution |
|---|---|---|---|---|---|
| Q1 | The exact bounded-read cap (e2e-design) and convergence round cap (e2e-review) | implementation | nothing | open | the caps are **enforced** (A-7); the numbers are tuned at implementation against the dogfood's spend |
| Q2 | The exact lens prompt wording (the two dimensions — coverage · anti-gaming — are fixed; the phrasing is design-detail per `prd:e2e-verification/§7.2 Q2`) | implementation | nothing | open | decided at implementation; the catch corpus (A-3) is the behavioral bar |
| Q3 | The reviewer's advisory→blocking adoption checkpoint (`prd:e2e-verification/§7.2 Q3`) — mirrors the existing reviewer program | operator | nothing | open | advisory in v1 (`#D-12`); engages when a downstream build/gate enforcement surface exists |

## Principles

- **AP-10 / AP-21 — honored.** Mechanical parts are codified or reused (the coverage-map structure, the plan's `blocked-by` ordering, the projection); judgment (coverage, lens verdicts, harness choice) is placed once at the skill and frozen into the artifact the downstream reads verbatim.
- **AP-11 — honored.** One reviewer program (a fourth instance), one context-pack machinery, one plan pipeline, one executor frontier — reused, not twinned.
- **AP-13 / AP-14 — honored.** The review's teeth are on the artifact (blocking findings settled before approval); acceptance of a test's genuine bite is **not** claimed at design time — it is the deferred run-time execution check (`prd:e2e-verification/WR-1`).
- **AP-19 — honored.** Bounded per-run cost — the bounded code read and the convergence round cap (`NFR-3`); the skills are invoked, with no standing daemon.
- **AP-5 — honored.** The skills emit the PRD's event plan and the artifacts are self-describing.
- **AP-28 — honored.** The improvement-fuel outputs (the review report's findings + dispositions, the catch-corpus run record) are retained as committed, append-only git records — the corpus the advisory→blocking ramp reads — never only ephemeral logs (§3.7).
- **AP-25 — honored.** The skills are vendor-neutral authoring surfaces (skills + guides), app-neutral by construction (`NFR-1`, `#DINV-2`).
- **NR-1 / NR-2 — honored.** Additive: the existing skills, reviewer program, plan-new pipeline, and executor are byte-unchanged; e2e-plan's plan is the ordinary `plan/v1` the pipeline already consumes.
- **RW-5 — honored.** One decision — the three skills' behavior; the artifact contract they produce and consume is the separate, independently-revisable `design:e2e-verification-artifacts`.
- **Deviation:** none.
