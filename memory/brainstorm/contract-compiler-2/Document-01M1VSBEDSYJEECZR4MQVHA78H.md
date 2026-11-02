# Contract compiler — mechanical core — brainstorm record

| Field | Value |
|---|---|
| **In one line** | Reopen of the deferred stage-6 compiler, scoped to the code-only half and **reframed from translator to gate**: a deterministic contract gate (currency, inverse coverage, link resolution, structural completeness, compilability) that bounces a flawed contract with a defect report and materializes **nothing** on pass, with every LLM-judgment check split into a tracked follow-up. |
| **Relevant-when** | brainstorming or contracting the mechanical contract gate, contractgate, the failure-only defect report, planning-entry gating, or coverage/currency enforcement; checking prior rulings on the mechanical/LLM split, the gate-not-translator reframe, or the compiler↔plan-new boundary |
| **Idea** | "ok, let's start by puting together a prd for the sections that requires only code checks. and create a follow up for 'llm' based checks later." (operator, 2026-07-17, following a stage-6 study splitting the compiler's duties into mechanical vs LLM columns) |
| **Operator** | @mdarabi |
| **Date(s)** | 2026-07-17 |
| **Status** | gated: ready-for-prd |
| **Tracking** | — |
| **Feeds** | docs/prds/contract-gate/ |
| **Prior records** | brainstorm:contract-compiler |
| **Template version** | brainstorm-record/v1 |

## Orientation

| Source | Why loaded |
|---|---|
| docs/brainstorms/contract-compiler.md (frozen) | the predecessor record — inherited rulings D-1/D-2/O-1 + F-1..F-8; D-3 names Q3 (plan-new posture) as the reopen's first question |
| docs/product-vision.md §4 · §11 | the stage-6 definition (compile-or-bounce, five-way verdict) and the road-ahead framing |
| docs/prds/vnext-requirements.md VNEXT-1 | the requirement of record: verdict classes, minimum defect list, output set, structured-data mandate |
| docs/system-design.md §2.1 · §2.3 · §2.4 · F2 · F3 | Contract Compiler as a Judgment service; Executable Contract artifact; judgment contract (frozen inputs, abstain, fail-closed) |
| .pyxis/pipeline.md | stage 6 status (⏸ deferred) and the built/unbuilt map around it |
| docs/tech-design/prd-amendment-model.md §12 · R7 | the already-specified currency contract: derived artifacts pin ledger position + fingerprint + cited IDs; Unaffected/Stale/Blocked/Needs-recompile; enforcement explicitly assigned to the Contract Compiler |
| docs/prds/plan-new/effective.md (FR-2, FR-16, FR-26) + effective.source.json | the downstream customer: slice-scoped coverage, all-or-nothing fog refusal, re-derived linkage — and the machine face's actual unit fields |
| docs/wiki/glossary.md | source map = "what the Contract Compiler consumes"; Serves = "the stage-6 compiler's join key"; binding elements = "what stage 6 compiles"; content fingerprint; downstream invalidation |
| internal/docgate/techdesigns/index.go · internal/plangate/plan/plan.go · internal/docgate/gate/reg_check_citations.go | what's already built: serves_inverse + GenCoverage (read-on-demand), stale-serves/stale-builds-on flags (non-fatal), plan source_fingerprint pinning, citation-dangler gating |
| docs/prds/README.md | overlap check across the feature index; prd-template is v2 |

**Overlaps surfaced:** plan-new (plan-vs-design slice coverage + fog bounce — the plan-level bar, not the feature-level design-set bar, predecessor F-6) · prd-review/tech-design-review (per-document substance — the O-1 boundary, untouched) · check-citations (never-existed citation targets already gate; moved-on targets only flag) · GenCoverage FR-28 (the coverage *view* exists, advisory, committed nowhere) · amendment model §12 (invalidation *specified* but explicitly unenforced until this feature) · implementation-verification (stage-8 harness; the verification graph's eventual stage-10 consumer is distinct). None covers the mechanical compiler; proceed.

## Framings

- **A — The salvage gates (checks-only)** — ship the two enforcement points with no new artifact: feature-level inverse coverage and input currency made gating at planning entry (a plan-new precondition), plus any docgate check additions. Graphs and artifact deferred wholesale to the LLM phase. Thesis: *enforce first, translate later; the artifact waits for its judgment half.*
- **B — The mechanical compiler (ExecutableContract v1)** — a deterministic compiler assembles the mechanically-derivable fields — requirement graph from the source map + Serves joins, verification skeleton from per-requirement Verify/fit data, compile pins per §12, coverage and structural verdicts — into the committed two-face artifact; the planning entry ticket is its verdict; LLM fields land additively later on the same schema. Thesis: *the translator ships now minus judgment; the gate rides the artifact (D-1's identity, phase-scoped).*
- **C — §12 made real (enforcement absorber)** — frame v1 as the invalidation enforcer the amendment model already assigns here: the compile is pins + cited-ID set + Needs-recompile/Stale refusal + coverage, with graphs kept minimal until their consumers (fan-out Planner, stage-10 Verifier) exist. Thesis: *currency enforcement is the product; graphs arrive with their consumers.*

**Destination:** a refinement of **A — the gate**, sharpened by the operator's 2026-07-17 ruling (D-1): the mechanical phase is a **planning-entry gate that materializes nothing on pass and a defect report on failure** — not the translator artifact of the inherited brainstorm:contract-compiler#D-1. B and C are rejected: both materialize the ExecutableContract on a passing contract, which the operator ruled out. The gate's checks are the code-only assembly properties (currency · inverse coverage · link resolution · structural completeness · compilability); the bounce mirrors plan-new's fog report — an artifact that exists only on refusal (FR-16).

## Decisions

### D-1 — the compile is failure-only: a passing contract materializes nothing

- **Question**: On a passing contract, does the mechanical phase materialize the ExecutableContract artifact (the inherited translator model, brainstorm:contract-compiler#D-1), or is an artifact produced only when the contract is flawed/incomplete?
- **Answer**: Artifact only on failure. "artifacts should only be generated when the contract is flawed or is not complete. if the check passes, no artifact is needed." The mechanical phase is a **gate**: pass → nothing persistent, planning proceeds; fail → a **defect report** naming the flaws/incompleteness (the named-defect bounce of vision §4).
- **Reasoning**: departs from — and supersedes for this reopen — the inherited translator posture (brainstorm:contract-compiler#D-1, "the artifact is the product"). The operator takes the gate reading of vision §4's compile-or-bounce: the bounce is the product; a pass is a green light, not an artifact. Well-founded — the artifact's inherited consumers (fan-out Planner, stage-10 Verifier, VNEXT-3 traceability) don't exist yet, so materializing a compiled program on every pass builds what nothing reads; plan-new already re-derives linkage from the docs (F-7). Mirrors plan-new's fog report (exists only on refusal, FR-16). The inherited D-2's no-human audit intent — evidence planning was entered legitimately — is carried by the plan's own `source_fingerprint` pin (F-5), not a separate compile.
- **Anchor**: 2026-07-17 · t1
- **Status**: live (supersedes inherited brainstorm:contract-compiler#D-1 for this reopen)

### D-2 — scope: the deterministic code-only core now; the LLM-judgment layer is a tracked follow-up

- **Question**: does this effort build the whole stage-6 compiler, or a phased slice?
- **Answer**: a phased slice — "put together a prd for the sections that require only code checks, and create a follow up for llm based checks later." This phase = the deterministic, code-decidable checks (currency · inverse coverage · link resolution · structural completeness · compilability). The reasoning/LLM checks (semantic ambiguity, PRD↔design contradiction, verification-path adequacy, risk, cost, suggested evals, required human decisions) are a separate follow-up feature.
- **Reasoning**: the two halves have opposite cost/verifiability profiles — the mechanical core is cheap, deterministic, testable now; the LLM layer is dear, non-deterministic, its outputs mostly analytical. Splitting lets the mechanical gate ship and gate immediately (closing the currency + coverage gaps that today have no owner) while judgment lands additively. Confirmed and refined across the value audit (t2–t3).
- **Anchor**: 2026-07-17 · t0 (opening instruction), ratified t1–t3
- **Status**: live

### D-3 — the mechanical check set: five assembly-property checks

- **Question**: what does the gate check?
- **Answer**: "adopt all" — the five code-decidable assembly-property checks: (1) **currency** (every contract input at current main; the §12 Stale/Needs-recompile contract finally *enforced*), (2) **inverse coverage** (every live Must served by ≥1 live design; reuses `GenCoverage`), (3) **link resolution** (every typed contract citation resolves live; reuses `check-citations`), (4) **structural completeness** (every requirement has a fit criterion + a *declared* verification method; DoD §5.8 present; §8 present — field-presence only), (5) **compilability/joint well-formedness** (source map + Serves joins assemble consistently: no orphan Serves, no broken supersession, no cyclic dependency).
- **Reasoning**: these are exactly the assembly properties D-1(inherited) scoped, each with an existing substrate so the gate is cheap; together they close the currency + coverage gaps that today have no owner. Pass = "no mechanical defects" → a reduced verdict set (clean / incomplete / malformed / stale), not VNEXT-1's full "executable"; the semantic verdicts arrive with the follow-up (O-2). Verification-method *presence* is here; *adequacy* is the follow-up.
- **Anchor**: 2026-07-17 · t4
- **Status**: live

### D-4 — the gate's trigger: hard at planning entry, advisory drift surface continuously

- **Question**: where and when does the gate fire (the inherited Q3, integration posture against plan-new)?
- **Answer**: both firing points, at asymmetric severity. **Planning entry — a hard gate**: `plan-new` refuses to draft until all five checks clear; a flawed contract bounces with the defect report. **Contract-doc PRs — a continuous advisory surface**: the same checks run as a docgate-style CI context at PRD/design merge; currency + link + structural may flag hard, but **inverse coverage stays advisory continuously** (a PRD legitimately merges before its designs exist) and is hard **only** at planning entry. Q3 resolved: planning *depends on* the gate as a precondition — not standalone-ignored, not retire-anything.
- **Reasoning**: continuous-only reproduces F-5's drift (surfaced-not-gating is *how* assemblies drift); planning-entry-only lets a broken contract sit latent until someone plans. The severity split kills the false-positive on mid-authoring contracts. Mirrors the predecessor's salvage shape (inverse coverage as a docgate check + stale-serves gating at planning entry).
- **Anchor**: 2026-07-17 · t5
- **Status**: live

### D-5 — the defect report: one structured, dual-face, template-governed artifact

- **Question**: on a bounce, what does the gate emit — and is it ephemeral or retained?
- **Answer**: "one artifact including all the failures, named, machine and human-readable. it has structure (follows a template) so can be parsed later if needed." A **single** report per bounce bundling every failed check's named defect (offender + what unblocks it), in **two faces** — a structured machine-readable projection and a human-readable rendering — governed by a template/schema so downstream tooling can parse it. Retained/parseable, **not** a fire-and-forget diagnostic (overrides the ephemeral recommendation).
- **Reasoning**: the report's value is downstream consumption — a machine consumer (the LLM follow-up, Argus remediation, telemetry over defect classes, the review-feedback loop) parses *why* a contract bounced; that value is independent of recompute cost (O-1's principle governs the *success* artifact, not this failure artifact). Template-governance matches AP-6 and the repo's two-face machine/human pattern (effective.md/.json, index.md/.json). Consistent with D-1: an artifact exists only on failure — this is that artifact, given its shape.
- **Anchor**: 2026-07-17 · t6
- **Status**: live

### D-6 — problem, beneficiary, jobs

- **Question**: how is the problem framed, for whom, and what jobs does the gate do?
- **Answer**: "sounds good." **Problem**: nothing validates a contract's *assembly* — currency, joint completeness (every live Must served), link-resolvability, structural well-formedness — before planning consumes it; reviewers judge each doc at birth and plan-new checks plan-vs-design, but the merged PRD + design-set + DoD has no owner, so drift or incompleteness first surfaces mid-implementation, the most expensive place. **Serves**: primary = the planning stage + everything downstream (protected from a broken contract) and the contract owner (Argus/human, gets a named bounce not a mid-build surprise); anti-segment = mid-authoring contracts (shielded by the advisory-vs-hard split) + draft-only L0 repos (hard gate never fires). **Jobs**: (1) stop a flawed/stale/incomplete contract entering planning, with named defects; (2) give the currency + coverage gaps a mechanical owner; (3) surface drift continuously.
- **Reasoning**: vision §4 (bad contracts → bad autonomous work) + the ownerless gap F-6/F-7 + the unenforced §12; beneficiary framed adapter-neutrally (the pipeline actor protected/served, not a medium role).
- **Anchor**: 2026-07-17 · t7
- **Status**: live

### D-7 — the bet and observable success

- **Question**: what's the bet, and how would we know it worked?
- **Answer**: "yep." **Bet**: a cheap deterministic gate at planning entry catches the five mechanical defect classes at the cheapest point, so no worker implements against a mechanically-broken contract — the wager being these classes are common + cheap enough that pre-planning gating pays for itself immediately. **Observable success**: (primary/guarantee) no plan freezes against a contract carrying any of the five mechanical defect classes — enforced by construction, verifiable by running the gate over every planned contract's frozen inputs; (secondary) a measurable pre-planning bounce count with valid named defects (from the D-5 parseable reports as telemetry), drift surfaced at introduction; (north-star) lowers interventions-per-feature + cost-per-shipped-feature.
- **Reasoning**: moves contract-defect discovery from mid-implementation to pre-planning. Honest caveat baked in: "flawed" = *mechanically* flawed; the semantic classes (ambiguous, contradictory) wait for the LLM follow-up (O-2) — this is a floor, not the full stage-6 guarantee.
- **Anchor**: 2026-07-17 · t8
- **Status**: live

### D-8 — appetite: the complete mechanical gate, bounded by the scope line

- **Question**: what spend is the problem worth?
- **Answer**: "go for full mechanical gates." Build the complete mechanical gate properly — all five checks, both triggers, the dual-face defect report, fully tested; bounded not by internal corner-cutting but by the scope line. **In budget**: the five checks (each wiring an existing substrate — `GenCoverage`, `check-citations`, §12 invalidation — into an enforced gate), the defect-report template + emitter, the `plan-new` planning-entry precondition, the continuous CI context, the P-2 check-input data-path work. **Out**: the LLM layer (O-2), any success artifact (O-1).
- **Reasoning**: a load-bearing problem (vision's most-important-capability claim) is worth doing well; the scope boundary (D-2/O-1/O-2) bounds it, not reduced completeness. P-2 is a **flag-and-surface** risk — raise it if exposing per-requirement fit/verification fields becomes a large machine-face change, don't silently absorb. Critical-path gate → middle-to-high PRD tier expected.
- **Anchor**: 2026-07-17 · t9
- **Status**: live

## Facts

- **F-1**: inherited rulings, not re-asked: D-1 — the identity is the **translator**; the ExecutableContract artifact is the product and the five-way verdict is its compile-error contract, scoped to assembly properties (currency · inverse coverage · joint consistency · compilability), never per-document substance. D-2 — the compile is planning's **hash-pinned entry ticket**: stale refuses to plan (never warns), recompilation is demand-driven, recompile-on-merge is ruled out (AP-27). O-1 — the compiler never re-runs reviewer lenses. Predecessor facts F-1..F-8 carry — source: brainstorm:contract-compiler#D-1 · #D-2 · #O-1 (frozen 2026-07-10)
- **F-2**: VNEXT-1 binds the charter: five-way classification (executable / ambiguous / incomplete / contradictory / unverifiable) with named-defect bounces; outputs = requirement graph, verification graph, risk, missing-information + ambiguity reports, suggested tests/evals, required human decisions, cost, readiness score — all structured data (AP-6) consumed by stages 7, 10, and VNEXT-4. Of the verdict classes, *ambiguous* and *contradictory* are semantic (LLM); *incomplete*, *unverifiable-as-presence*, and currency are mechanically decidable — source: docs/prds/vnext-requirements.md VNEXT-1 + the 2026-07-16 study in-session
- **F-3**: the machine faces were built for this consumer: the source map is specified as "what the Contract Compiler, traceability, and invalidation consume"; **Serves** is "the stage-6 compiler's join key"; design **binding elements** (Decision callouts, DINV-n, A-n) are "what stage 6 compiles" — source: docs/wiki/glossary.md (amendment model §4 · tech-design-conventions §3.1/§3.3)
- **F-4**: the currency contract is already specified and assigned: every derived artifact records ledger position + content fingerprint + cited unit IDs; invalidation states Unaffected / Stale / Blocked / **Needs-recompile** are computed by `docgate diff` and emitted as data; contract-changing amendments block execution until recompile; "enforcement is absorbed by the Contract Compiler (VNEXT-1)" — today it is review discipline only — source: docs/tech-design/prd-amendment-model.md §12 · R7
- **F-5**: the mechanical substrate shipped: per-projection sha256 `content_fingerprint` (tool version recorded, never hashed); plangate plans already pin `source_fingerprint`; the design index computes `serves_inverse` and the non-fatal `stale-serves`/`stale-builds-on` flags; `GenCoverage` (docgate FR-28) computes the requirement-universe ↔ live-design coverage join on read, committed nowhere; `check-citations` gates never-existed citation targets repo-wide — source: internal/docgate/techdesigns/index.go · internal/plangate/plan/plan.go:122 · internal/docgate/gate/reg_check_citations.go
- **F-6**: source-map units are **lineage-only** today — `{status, introduced_by, supersedes, superseded_by, source_location, current_location}`; no kind/tier/verify/fit-criterion fields — so mechanical requirement/verification-graph assembly requires either enriching the fold's machine face (a docgate amendment) or parsing effective.md tables — source: docs/prds/plan-new/effective.source.json inspected 2026-07-17
- **F-7**: the coverage bar splits by level: plan-new's FR-26 owns the **plan-level** bar (slice-scoped coverage + feature-level union across plans) and FR-16 the all-or-nothing fog bounce; the **feature-level design-set** bar — every live Must requirement served by ≥1 live design *before planning* — has no owner (predecessor F-6); GenCoverage computes exactly this join but nothing gates on it — source: docs/prds/plan-new/effective.md FR-26 · FR-16 + brainstorm:contract-compiler#F-6
- **F-8**: the toolchain family precedent for a deterministic gate binary with two-face committed outputs exists as cmd/{docgate,plangate,codegate} — source: repo tree, cmd/ listing 2026-07-17
- **F-9**: prd-template §8 (Contract readiness) is the hand-run precursor: "the authoring-side mirror of pipeline stage 6 — once the Contract Compiler exists it runs these checks mechanically" — source: docs/wiki/glossary.md (prd-template §8)

## Fog

none

## Out of scope

- **O-1**: materializing a **success artifact** — the always-on ExecutableContract of the inherited translator model — is out of scope for this phase. Reason: its value (a consumed input for the Planner / stage-10 Verifier / VNEXT-3 traceability; a cache of expensive judgment) is deferred to consumers that don't exist yet and to the LLM follow-up where recompute is dear; the mechanical checks are cheap to recompute, so a persisted success artifact is net-negative now. **Revisit trigger**: a stage-10 Verifier or fan-out Planner is built and needs the contract-as-graph as a consumed input, or the LLM follow-up needs to freeze expensive judgment as data (AP-21). (from D-1)
- **O-2**: the **LLM-judgment checks and analytical outputs** — semantic ambiguity, PRD↔design / design↔design contradiction, verification-path *adequacy*, risk assessment, cost estimate, suggested tests/evals, required human decisions — are out of scope for this phase; they are the tracked follow-up feature. Reason: they require reasoning (non-deterministic, dear to recompute) where this phase is the deterministic code-only core. (from D-2)
- **O-3**: the gate **never re-judges per-document substance** — no prd-review / tech-design-review lenses, no requirement-quality judgment; it consumes reviewed, merged documents and checks only assembly properties. The inherited boundary (brainstorm:contract-compiler#O-1), preserved. (from D-3)

## Candidate glossary terms

- **T-1**: **contract gate** — the mechanical stage-6 gate that validates a feature's *assembled* contract (currency · inverse coverage · link resolution · structural completeness · compilability) and bounces a flawed contract with a named-defect report, materializing nothing on pass (D-1). · avoid: *contract compiler* — the inherited translator framing implies artifact-on-success (superseded for this phase by D-1); "compiler" survives as the full-stage-6 / LLM-inclusive name. Feature-slug candidate for `/prd-new`: `contract-gate` (vs the inherited `contract-compiler`).

## Parked technical constraints

- **P-1**: tool home and name (new `cmd/contractgate` + `internal/contractgate` per the family precedent F-8, vs a docgate verb set) — stage-3/4 matter; the PRD binds behavior, not the binary boundary
- **P-2**: check-input data path — the structural + coverage checks need per-unit kind/tier/verify/fit fields the source map does not expose today (lineage-only, F-6); enrich the fold's machine face (a docgate amendment, single-parser principle) vs the gate parsing effective.md tables — stage-3 decides
- **P-3**: defect-report storage mechanism + staleness — the report (D-5) is retained/parseable, but *where* (a git-committed file à la review reports, vs a CI-run artifact / planning-session record / scratch path) is a stage-3/4 call. Tension to resolve there: the report is a deterministic derivable of a failing contract state, so a git-committed report goes stale the moment the contract is fixed (the #149 derived-artifact-staleness family) — argues for a build/session artifact over a committed repo file, unless retention-as-history is explicitly wanted for the telemetry/learning consumers named in D-5

## Gate

| Field | Value |
|---|---|
| **Verdict** | ready-for-prd |
| **Date** | 2026-07-17 |
| **Operator** | @mdarabi |
| **Covering source** | — |
| **Coverage check** | pass — all six inputs covered (D-6 problem/segments/jobs · D-7 goals direction · O-1/O-2/O-3 non-goals · D-8 appetite), zero blocking fog |
