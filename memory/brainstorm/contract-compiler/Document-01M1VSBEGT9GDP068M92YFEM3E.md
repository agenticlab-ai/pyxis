# Contract compiler — brainstorm record

| Field | Value |
|---|---|
| **In one line** | Stage 6: compile a feature's approved contract (PRD + designs + shapes + DoD) into an executable form — requirement graph, verification graph, risk, cost, readiness verdict — or bounce it with named defects before any planning token burns. |
| **Relevant-when** | brainstorming or contracting the contract compiler, the ExecutableContract artifact, or the stage-6 gate; checking prior rulings on contract validation, compile triggers, or the compiler↔planner boundary |
| **Idea** | "Let's start the contract compiler" (operator, 2026-07-10) — the stage-6 capability: vision §4's compile-or-bounce, VNEXT-1's "validate the contract before execution", framed against plan-new/plangate in the pre-session comparison |
| **Operator** | @mdarabi |
| **Date(s)** | 2026-07-10 |
| **Status** | gated: needs-investigation |
| **Tracking** | — |
| **Feeds** | — |
| **Prior records** | — |
| **Template version** | brainstorm-record/v1 |

## Orientation

| Source | Why loaded |
|---|---|
| docs/product-vision.md | §4 stage 6 (the compile-or-bounce definition, the "most load-bearing capability" claim); §1 (DoD makes done decidable); §10 (zero-touch is only safe if done is machine-decidable); §11 VNEXT-1 |
| docs/domain-model.md | the ExecutableContract entity (§3.1): requirement graph + verification graph + risk + readiness verdict; the Planner's sole input |
| docs/system-design.md | §2.3 the Contract Compiler judgment service; F2 (approved docs → CC → Planner; defects bounce); F3 (the judgment contract: frozen inputs, structured results, abstain semantics) |
| docs/prds/plan-new/effective.md + docs/tech-design/plan-new-v1.md | the downstream customer and the architectural precedent (deterministic gate core + judgment skill; fold to byte-verified machine faces; coverage + fog bounce it currently runs compiler-less) |
| the machine faces: effective.source.json · index.json (binding blocks, requirements-inverse map) · Serves joins · review reports | the compiler's raw inputs — all shipped within the last two weeks |
| prd-template §8 | the compiler's hand-run precursor ("the authoring-side mirror of the engine's contract gate — once the Contract Compiler exists it runs these checks mechanically") |
| docs/prds/prd-review/effective.md + tech-design-review/effective.md | the upstream reviewers: per-document substance at birth — what the compiler must NOT redo |
| docs/tech-design/distributed-check-registration.md (#149, loaded mid-session) | how gate checks compose — registry, no shared enumeration (§3.8 names the umbrellas any compiler checks would join); its §4 rejects regenerate-in-the-loop aggregates ("the generated artifact re-conflicts on every addition") — a constraint on the compile artifact's write moment |
| product-vision §12 + AP-27 (#150, loaded mid-session) | parallel authorship as governance: extension is by addition; a needed central aggregate is **derived, never authored** — no author's diff touches it |

**Overlaps surfaced:** plangate validates the *plan's* form, not the contract (its coverage is plan-vs-design); the reviewers validate each *document's* substance, not the assembly; §8 is a hand-walked checklist inside one document. Cross-document assembly + executability is open ground; no covering capability exists.

## Framings

- **The gate that emits graphs** — the identity is the stage-6 quality gate: the five-way verdict (executable / ambiguous / incomplete / contradictory / unverifiable) with named-defect bounces; the compiled graphs are the passing gate's evidence. Thesis: *refusal is the product; the artifact proves the pass.*
- **The translator** — the identity is the **ExecutableContract**: the machine-native program the Planner decomposes and stage 10 executes (requirement graph, verification graph, risk, cost, human-decisions); a bounce is just a compile error. Thesis: *the artifact is the product; refusal is what failing to produce it looks like.*
- **plangate's sibling** — the identity is the toolchain shape: a deterministic `contractgate` core (link resolution, coverage, contradiction checks over the machine faces, fold to byte-verified output, CI-wired) with a thin judgment skill for the semantic residue (ambiguity, verifiability-of-prose, cost). Thesis: *the mechanical/judgment split is the identity.*

**Destination:** the **translator** — the ExecutableContract artifact is the product (D-1); the gate framing's five-way verdict rides as its compile-error contract, scoped to assembly properties; the toolchain-shape framing is the tech design's ground, not the identity.

## Decisions

### D-1 — the identity: the translator, with the gate's semantics as its error contract

- **Question**: which framing is the feature — the gate that emits graphs, the translator, or plangate's sibling?
- **Answer**: **the translator** — "go with B with A's as error checking." The ExecutableContract artifact is the product; the five-way verdict (executable · ambiguous · incomplete · contradictory · unverifiable — vision §4) is its compile-error contract, scoped to **assembly properties** — currency, inverse coverage, joint consistency, compilability — never per-document substance. The toolchain-shape framing (deterministic core + judgment skill) is demoted to the tech design: implementation shape, not identity.
- **Reasoning**: ruled through the operator's linearity challenge (F-5, F-6): with documents born reviewed, per-document validation is already spent upstream — a gate-first identity is thin. But the artifact three stages consume (Planner decomposition, implementation traceability, stage-10 verification) exists nowhere, and "plannable now" / "covered by the design set" are undecidable today. The value concentrates in the artifact; a bounce is what failing to produce it looks like.
- **Anchor**: 2026-07-10 · t4–t5
- **Status**: live

### D-2 — the currency contract: planning's entry ticket, hash-pinned

- **Question**: what makes a compile authoritative, and what happens when its inputs move after it? (Persistence taken as ground, not asked: the compile is a committed artifact like every machine face — files-as-truth, contractual data, and in the no-human world the compile *is* the audit evidence planning was entered legitimately.)
- **Answer**: **A — the entry ticket.** The ExecutableContract records the content hash of every input it compiled (the effective projection, each design, shapes, DoD). Planning may start **only** from a compile whose pins match current main: staleness is a mechanical check, and a stale compile **refuses to plan** — never warns. No active invalidation machinery — any contract-touching merge silently stales the compile; the pin check catches it at the next consumption, read-only. Recompilation is demand-driven.
- **Reasoning**: C (advisory flags) reproduces F-5's documented failure — surfaced-not-gating is how assemblies drift. B (recompile-on-merge) violates AP-27's derived-never-authored discipline: the regenerated aggregate rides every contract-touching sibling PR — the regenerate-in-the-loop shape #149's alternatives table rejects (F-7, F-8) — or a post-merge bot commits it back, racing itself. A keeps the write moment single-actor per feature (one moment: planning entry), matching the effective.md precedent, and makes the amendment model's `Needs-recompile` state mechanical for free (a pin mismatch). Trigger-independence: eager recompilation stays available later as pure policy atop A's mechanics — the contract is the verb + the staleness predicate, never the schedule.
- **Anchor**: 2026-07-10 · t6–t7
- **Status**: live

### D-3 — the gate: deferred — needs-investigation

- **Question**: does the compiler proceed to a PRD now?
- **Answer**: no. "As we are moving forward with this feature, I feel we do not need it and just adds complexity for less value… bring it back when the need is real" — gated **needs-investigation**; the record commits so a reopen inherits D-1/D-2 and F-1..F-8 instead of re-deriving them. The session closed mid-frontier: Q3 — the integration posture against plan-new (retire-now / consume-if-present / standalone) — was posed and unruled, and is the reopen session's first question.
- **Reasoning**: the ExecutableContract's consumers are mostly unbuilt — stage 10, risk-lane routing, and the fan-out Planner don't exist; plan-new works compiler-less (F-4), and the reviewers ate the validation half of stage 6's original charter (the t4 challenge, F-5/F-6). What remains is thin today against the full feature apparatus (a judgment service, an artifact class, gates, adoption staging, evals). The deferral is safe because the mechanical residue needs no compiler — two cheap deterministic checks cover it: inverse coverage as a docgate check over the index (F-6), and stale-serves made gating at planning entry (F-5) — unqueued at close, available any time. **Reopen triggers**: those flags hurting at real scale, or stage-10 / fan-out-Planner work starting. The vision stays untouched — it is the end state, not the roadmap; absence of a PRD carries "not now" (a vision edit was floated at t8; the leave-it recommendation stood un-overruled at the verdict).
- **Anchor**: 2026-07-10 · t8–t9
- **Status**: live

## Facts

- **F-1**: stage 6's actor is the Factory and its contract is compile-or-bounce with named defects — the no-human posture is definitional here, not a ruling to re-make — source: product-vision §4
- **F-2**: the ExecutableContract is specified as the Planner's **sole** input — source: docs/domain-model.md §3.1
- **F-3**: plangate already ships the deterministic-core + judgment-skill split, byte-verified machine faces, and defects-as-data — the in-house precedent — source: docs/tech-design/plan-new-v1.md lineage (techspec)
- **F-4**: plan-new currently re-derives requirement linkage from the design and runs its own fog bounce — the compiler-less costs the pre-session comparison named — source: plan-new FR-15/16, FR-2 lineage
- **F-5**: per-stage validity is point-in-time, not current — PRDs amend and designs supersede *after* dependents froze against older projections, so the assembly drifts even when every stage gate passed honestly; live evidence: a docgate-ledger amendment superseded FR-11 while a merged design still served the old row, caught only by the index's non-gating stale-serves flag — source: operator's linearity challenge + the index's stale-serves evidence, 2026-07-10 · t4
- **F-6**: inverse coverage is unowned — tech-design-review proves the anchors a design *cites* (design→PRD); nothing proves every Must requirement is served by some design's content (PRD→design-set), and plan-new's coverage is plan-vs-design; "complete enough to enter planning" has no owner — source: t4 analysis over the reviewer PRDs + plan-new FR-15/16
- **F-7**: parallel authorship is governance as of 2026-07-10 — extension is by addition, a shared append point is a design defect, and a genuinely needed central aggregate is **derived, never authored** (no author's diff touches it; order is explicit data) — source: vision §12 + AP-27 (#150)
- **F-8**: the registry precedent — docgate/plangate umbrellas assemble checks from per-file self-registration (any compiler checks are born into this shape, §3.8), and the design's alternatives table rejects aggregates regenerated inside the authoring loop as re-conflicting on every addition — a constraint on *when* the compiled artifact may be written — source: docs/tech-design/distributed-check-registration.md §3.8 · §4 (#149)

## Fog

none

## Out of scope

- **O-1 — per-document substance, the reviewers' ground**: the compiler never re-runs prd-review / tech-design-review lenses or re-judges document quality; it consumes reviewed, merged documents and checks only assembly properties — the boundary mirror of the reviewers' never-re-judge rule — originates: D-1

## Candidate glossary terms

- **compile pins** — the content hashes of every input a compile consumed, recorded in the ExecutableContract; pin mismatch against main = stale (D-2)
- **stale compile** — a compile whose pins no longer match main; refuses to plan, remedied only by recompiling (D-2; the amendment model's `Needs-recompile`, made mechanical)

## Parked technical constraints

none

## Gate

| Field | Value |
|---|---|
| **Verdict** | needs-investigation |
| **Date** | 2026-07-10 |
| **Operator** | @mdarabi |
| **Covering source** | — |
| **Coverage check** | — *(walked only for ready-for-prd)* |
