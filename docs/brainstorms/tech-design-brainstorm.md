# Tech-design-brainstorm skill — brainstorm record

| Field | Value |
|---|---|
| **In one line** | The stage-3 explore skill: given an approved PRD, brainstorm candidate technical designs with the operator and hand the chosen direction to tech-design-new. |
| **Relevant-when** | brainstorming or contracting the stage-3 design-brainstorm skill, its record class, or the design-it-twice / judge-panel machinery; checking prior rulings on solution-space exploration, PLAN-1 parity, or the grilling-primitive extraction (WR-2) |
| **Idea** | "The tech-design-brainstorm skill — the stage-3 explore half: given an approved PRD, brainstorm candidate technical designs with the operator, then hand the chosen direction to tech-design-new." (from the four-skill System-2 program; operator sequenced 2026-07-09/10: prd-new improvements → this → tech-design-new improvements) |
| **Operator** | @mdarabi |
| **Date(s)** | 2026-07-10 |
| **Status** | gated: ready-for-prd |
| **Tracking** | — |
| **Feeds** | docs/prds/tech-design-brainstorm/ |
| **Prior records** | — |
| **Template version** | brainstorm-record/v1 |

## Orientation

| Source | Why loaded |
|---|---|
| docs/product-vision.md | §4 stage 3: "Technical design brainstorming — Factory ↔ Human — multiple candidate options; constant back-and-forth" |
| docs/prds/design-and-planning.md | PLAN-1 — the harbormaster ancestor: 2–3 distinct approaches, scored vs design principles **and matched past incidents**, recommendation + handoff, zero tracker writes |
| docs/prds/feature-brainstorm/effective.md | the sibling spine: NG-2 parked solution-space exploration to this feature; WR-2 names it the grilling-primitive extraction trigger |
| docs/tech-design/brainstorm-record-model.md | §3.6: the stage-3 record class is "owned by a future design" that Builds-on it and shares the citation grammar |
| docs/tech-design/tech-design-conventions.md + .pyxis/authoring/tech-design-new.md | the consumer: what a design doc needs decided before authoring starts (Serves anchors, alternatives, DINV/A rows, Principles) |
| docs/domain-model.md | §2.1 stage-3 row; judgment-plane posture (AP-21 judge-once) |
| docs/architecture-principles.md | the scoring-rubric candidate (AP/RW citations) + the incident-registry loop |

**Overlaps surfaced:** PLAN-1 is the *parity ancestor*, not coverage — the baseline names harbormaster's `/brainstorm` (approaches scored vs principles + incidents) as a requirement pyxis has not built; this feature is that gap. The sibling PRD's NG-2/WR-2 explicitly reserve this ground. No covering capability exists.

## Framings

- **The judge panel** — design-it-twice is the essence: N parallel design alternatives under orthogonal constraints (minimize-interface / maximize-flexibility / optimize-common-caller / ports-and-adapters), compared on depth, locality, and seam placement; recommendation + hybrid. The conversation reviews the panel's output. Thesis: *structured divergence is the value.*
- **The design grilling** — the solution-space twin of feature-brainstorm: a HITL interview over seams, constraints, and trade-offs with the same record discipline (operator-anchored decisions, typed fog, explicit gate); the parallel panel is one *move* inside the session. Thesis: *resolved design decisions with provenance are the value*, feeding tech-design-new the way brainstorm records feed prd-new.
- **The scored bake-off** — PLAN-1 parity, modernized: 2–3 candidate approaches scored against the architecture principles and matched incidents ("how does this one die? which incident does it repeat?"); recommendation + handoff; deliberately lighter than a full session. Thesis: *risk-scored comparison against institutional memory is the value.*

**Destination:** the *design grilling* — a HITL interview over the solution space with the full record discipline (operator-anchored decisions, typed fog, explicit gate), producing a design-brainstorm record that feeds tech-design-new; the parallel judge panel is one move available inside the session, not its identity. — Operator: "B."

## Decisions

### D-1 — which framing is the feature

- **Question**: is this skill a judge panel (A), a design grilling with the record discipline (B), or a scored bake-off (C)?
- **Answer**: B — the design grilling.
- **Reasoning**: the record-with-provenance spine is the value and matches the proven stage-1 pattern; the panel remains one move inside the session per B's own definition (recommendation was the B-spine hybrid; operator chose B directly).
- **Anchor**: 2026-07-10 · t2
- **Status**: live

### D-2 — the record class

- **Question**: does a design-grilling session write into `brainstorm-record/v1`, or a sibling template?
- **Answer**: a sibling — a distinct design-brainstorm record template.
- **Reasoning**: shared spine (header incl. routing pair, anchor discipline, citation-grammar family, gate mechanics) with design-specific sections; the coverage checklist must target tech-design-template sections, not PRD inputs — stretching v1 would make both stages' gates conditional. Recommended section shape (seam map · dependency classification · alternatives-with-scores · parked product gaps) presented and to be pinned by the feature's own tech design; pre-signaled by brainstorm-record-model §3.6.
- **Anchor**: 2026-07-10 · t3
- **Status**: live

### D-3 — the input contract

- **Question**: what may enter a design-grilling session, and what is refused?
- **Answer**: admit a merged per-feature PRD (`effective.md`) or, bootstrap-era, citable corpus anchors; refuse a draft/unmerged PRD (finish stage 2 first) and a no-basis request (route to /feature-brainstorm → /prd-new, or cite the corpus) — refusals name the remedy.
- **Reasoning**: grilling against an unfrozen contract yields a record citing unstable anchors; the two-admission shape mirrors the `Serves` grammar (`prd:` and `corpus:` are both legal join keys), keeping one vocabulary.
- **Anchor**: 2026-07-10 · t4
- **Status**: live

### D-4 — when the judge-panel move fires

- **Question**: does the parallel-alternatives panel run always, only when the operator invokes it, or skill-judged by a rule?
- **Answer**: skill-judged by the seam rule, operator-overridable both ways — the panel fires when the session's design question opens or replaces a seam (new interface, port, document class, module boundary); pure extensions of an existing seam skip it; the operator may force ("panel this") or waive ("skip the panel"); the record notes which path was taken and why.
- **Reasoning**: *always* is ceremony on trivial extensions; *operator-invoked-only* under-uses it where first-idea anchoring is strongest (novel seams, where a wrong shape costs a supersession); the audit note keeps the call challengeable. Worked examples recorded in-session (fires: brainstorm-record-model, WorkTracker port; skips: docgate pre-push verb, #111 header fields). A per-repo config dial is deferred to the pyxis-config surface.
- **Anchor**: 2026-07-10 · t6
- **Status**: live

### D-5 — the gate verdict enum

- **Question**: what verdicts can a design-grilling session end with?
- **Answer**: five — ready-for-design (hand to tech-design-new) · **needs-more-product** (a PRD hole surfaced: stop designing, name the gap, route to `/prd-amend` — never a silent local patch of requirements) · needs-investigation (research/prototype detour first) · covered (a live design already decides it — point and stop) · abandoned (discarded, not committed).
- **Reasoning**: stage 1's enum plus the distinctively stage-3 case — solution-space grilling is where product under-specification actually surfaces, so the bounce gets a formal verdict and route rather than folding into needs-investigation (different remedy, different owner). Blocking fog retargets to the design-template's decision-shaped inputs (the §2 Decision, seam placement, Serves set); non-blocking rides the design doc as its Q-n rows.
- **Anchor**: 2026-07-10 · t7
- **Status**: live

### D-6 — no shared grilling primitive

- **Question**: extract the shared interview discipline (WR-2's named trigger: the second consumer) into a common authoring reference both skills use?
- **Answer**: no — the two skills carry their own interview discipline and evolve separately in the long run.
- **Reasoning**: operator's ruling — stage-1 (product-space) and stage-3 (solution-space) grilling are expected to diverge; a shared module would couple their evolution, and that coupling cost outweighs the duplication/drift cost (which stays bounded by each skill's own eval corpus). Recommendation was to extract; refused. Consequence: feature-brainstorm's ledger WR-2 should be resolved-refused by a typed amendment; this feature's PRD carries "no shared primitive" as a non-goal (→ O-1).
- **Anchor**: 2026-07-10 · t8
- **Status**: live

### D-7 — the audience, autonomy-ready

- **Question**: who is this skill for, given the long-run goal of a fully automated cycle where human involvement ends at feature-brainstorm?
- **Answer**: P1 = the **stage-3 decision authority** — the operator at today's autonomy level, the judgment plane acting within a risk envelope later, with the operator as escalation authority always; P2 = tech-design-new / the Planner (consumes direction + rejected alternatives + seams, unchanged); P3 = traceability/audit, which grows in importance with autonomy (the record is the evidence package for design decisions); anti-segment = design decisions made **without a record** — autonomy without provenance is refused forever, while the autonomous answerer itself is a waiting-room row ("revisit when the autonomy ladder reaches design decisions"), not a non-goal.
- **Reasoning**: the grilling protocol is answerer-neutral by construction — every question carries its recommended answer, so autonomous mode = the factory adopting its own recommendations within an envelope (AP-21), escalating exceptions as decision briefs; D-4's seam rule doubles as the natural escalation boundary (novel seams stay human longest) and D-5's needs-more-product bounce is structurally an escalation. HITL is the current binding, not the identity.
- **Anchor**: 2026-07-10 · t10
- **Status**: live
- *Jobs, per the same ruling*: (J1) explore the solution space with recommendations and end with a ruled direction; (J2) tech-design-new authors as assembly from the record; (J3) a surfaced product hole gets the formal needs-more-product bounce, never a silent patch; (J4) "why this architecture" traces to the authority's ruling — all read against the decision authority, not "the operator".

### D-8 — the success bet

- **Question**: what does this skill measurably win?
- **Answer**: a gated ready-for-design record makes tech-design-new pure assembly. Primary: decision-questions during design authoring from a gated record — median 0 at checkpoint "first five designs through stage 3". Provenance: 100% of the authored design's §2 Decision + §4 Alternatives rows trace to `design-brainstorm:<slug>#D-n` anchors at the same checkpoint. Qualitative: panel-fired sessions produce genuinely distinct alternatives, confirmed by the decision authority at the gate.
- **Reasoning**: mirrors the proven stage-1 shape (M1/M2/G3) retargeted at the design seam; the provenance measure doubles as the audit trail the autonomy path requires (D-7).
- **Anchor**: 2026-07-10 · t11
- **Status**: live

### D-9 — non-goals and waiting room

- **Question**: what does this feature refuse, and what waits?
- **Answer**: refuses — authoring the tech design itself (tech-design-new's job; the explore/exploit split stands); engine/judgment-service code; a shared grilling primitive (per D-6); tracker writes or work-item creation (write perimeter = the record + synthesis-time glossary candidates); stage-6/contract-compilation concerns (the bounce routes to /prd-amend); modifying governed documents. Waiting room — the autonomous answerer within a risk envelope (D-7); the per-repo config dial for the panel rule (D-4); record-lint graduation for the sibling record class (aligned with stage 1's five-record trigger).
- **Reasoning**: mirrors the stage-1 perimeter where the same forces apply; the distinctive rows (no shared primitive, prd-amend routing) carry this session's own rulings.
- **Anchor**: 2026-07-10 · t12
- **Status**: live

### D-10 — appetite

- **Question**: what is this feature worth?
- **Answer**: small — one authoring guide + the sibling `design-brainstorm-record/v1` template (own checklists targeting the design-template's decision-shaped inputs) + skill glue + an eval fixture corpus with its own HITL/integrity fixtures; ≤ 2 human review rounds per document; modest eval-run spend; zero rollout/support burden. Excluded: engine code, the autonomous answerer, docgate changes, and any edit to the shipped feature-brainstorm surface (D-6's consequence) — the only stage-1 touch is the separate one-row `/prd-amend` resolving WR-2 as refused.
- **Reasoning**: the stage-1 shape held; D-6 narrows blast radius further.
- **Anchor**: 2026-07-10 · t13
- **Status**: live

## Facts

- **F-1**: harbormaster's brainstorm stage scored approaches against design principles and matched past incidents, ended with an explicit recommendation, and made zero tracker writes — source: docs/prds/design-and-planning.md (PLAN-1)
- **F-2**: the stage-3 record class was deliberately left to a future design that Builds-on brainstorm-record-model and shares its citation grammar — source: docs/tech-design/brainstorm-record-model.md §3.6

## Fog

- **G-1** (non-blocking — blocks: a future governance append, not this feature): the operator's stated long-run posture ("human involvement ends at feature-brainstorm; the system drives to the end") is sharper than vision §1, which has the engineer authoring the design as part of the contract — reconciliation is a human-authored vision append (AP-24); this feature cites §10's zero-touch north star meanwhile

## Out of scope

- **O-1**: a shared grilling primitive across the stage-1 and stage-3 skills — refused; the disciplines evolve independently, each guide self-contained, drift bounded by per-skill eval corpora (from D-6)

## Candidate glossary terms

- **T-1**: seam — the place where two parts of a system meet such that either side can change without editing the other; the spot where an interface lives (the interface is the contract, the seam is its location). Load-bearing in the panel rule: "opens a seam" = a new meeting point whose contract must be designed; "extends a seam" = adds to a settled contract. (Feathers.) · avoid: boundary (overloaded), interface (the contract at the seam, not the place)

## Parked technical constraints

- **P-1**: the sibling record template's exact section set and coverage mapping — seam map · dependency classification · alternatives-with-scores (principles + matched incidents as axes) · parked product gaps recommended in-session — pinned by this feature's own tech design, not here (from D-2)

## Gate

| Field | Value |
|---|---|
| **Verdict** | ready-for-prd |
| **Date** | 2026-07-10 |
| **Operator** | @mdarabi |
| **Covering source** | — |
| **Coverage check** | pass — all six inputs covered (§2.1←D-1 · §2.2←D-7 · §2.3←D-7/J1–J4 · §3.1←D-8 · §3.4←D-9/O-1 · appetite←D-10), zero blocking fog (G-1 non-blocking) |
