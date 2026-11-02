# impl-review — design-brainstorm record

| Field | Value |
|---|---|
| **In one line** | Design-space grill for the proposal-class reviewer (prd:impl-review): the machine-harvestable PR-resident finding/report artifact (breaking the siblings' human-markdown pattern where FR-13 demands it), the harvest-vs-ledger measurement boundary, the fresh-context lens execution + eval protocol, schema kinship with codegate's finding core, and the seam onto implement-now's worker exit. |
| **Relevant-when** | designing or changing the impl-review reviewer — its finding/report schema and PR-resident persistence (both media), the measurement harvest, the fresh-context lens runner + eval suite, the downgrade-signal→merge-lane wiring, the review-staleness binding, or the worker-seam invocation; checking prior design rulings or refused shapes for machine code review |
| **Basis** | prd:impl-review (merged ledger, PR #407, 2026-07-17) |
| **Authority** | @mdarabi |
| **Date(s)** | 2026-07-17 |
| **Status** | gated: ready-for-design |
| **Tracking** | — |
| **Feeds** | docs/tech-design/impl-review-v1.md |
| **Prior records** | brainstorm:impl-review · design-brainstorm:reviewer-artifacts-tech-design · design-brainstorm:implementation-verification-tech-design |
| **Template version** | design-brainstorm-record/v1 |

## Orientation

| Source | Why loaded |
|---|---|
| docs/prds/impl-review/effective.md | the basis — FR-1..17, UB-1..8, NR-1..4, NFR-1..5, EVT-1..6, EVAL-1..8, the four open Qs; authored + merged this session, held verbatim |
| docs/tech-design/prd-review-artifacts.md + tech-design-review-artifacts.md (live) | the sibling pattern to mirror: typed-markdown report in the record class (`review-report/v1`), calibration as versioned markdown beside the skill glue, embedded escalation brief, JSON-native mechanization **deliberately parked for the engine era** — the exact ruling FR-13/D-9 force this feature to break |
| docs/tech-design/codegate-v1.md (live) | the finding-core to extend (FR-5 kinship): codegate's finding schema (check_id·class·rung·path·line·message·log_ref) lives in `internal/verify/schema`, frozen as implement-now's joint contract; NR-1 = read-only reuse, zero codegate change |
| docs/tech-design/implement-now.md (live) | the host seam: the Worker exit path (ladder green → PR) + the VerificationGate additive-amendment class NR-2 shares; the fresh-process-per-task runtime the lens runner echoes |
| docs/tech-design/merge-lane-v1.md (live) | the GATE-4 downgrade-only slot FR-7/NR-4's verdict signal feeds; the risk-class data FR-9's bands read |
| docs/tech-design/review-feedback-capture.md (live) | the ledger DQ-2/Q1 weighs as the disposition-mirror alternative; its federated-closable-entry shape |
| docs/tech-design/local-work-tracker-v2.md (live) | the Carina medium: typed decision-records (the marker-grammar replacement) FR-6 persists to; the two-media persistence target |
| docs/architecture-principles.md + incidents (session context) | AP-11 (one risk predicate) · AP-13 (revision binding, UB-6) · AP-6 (closed schemas) · AP-21 (judge-once) · AP-27 (derived registration for lenses/checks) · AP-12 (the orphan-review janitor, UB-8) |
| the prd-review/tech-design-review **skills** driven live this session | the fresh-context lens execution precedent: parallel subagent spawn, adversarial screen, calibration citation — the runtime model DQ-3 extends |

**Overlaps surfaced:**

- `design:prd-review-artifacts` / `tech-design-review-artifacts` (live) — decide the *report artifact* for the document reviewers as **human markdown, JSON deferred**. impl-review's FR-13 ("harvestable without reading free text") + D-9 (machine corpus) **cannot** adopt that ruling wholesale — the divergence is the design's central seam (DQ-1), not a re-litigation: the siblings serve a human reader, this serves the harvest + WR-2. The verdict-protocol shape (rounds, dispositions, escalation brief, calibration citation) still transfers.
- `design:codegate-v1` finding schema (live, frozen) — the core FR-5 extends; NR-1 forbids editing it. The kinship mechanism (extend-by-composition, read-only) is DQ-1's sub-question, not open ground — the constraint is fixed, the mechanism is design.
- `design:merge-lane-v1` GATE-4 slot (live) — the downgrade consumer already exists; FR-7/NR-4 wire *into* it, adding no lane logic. Extends an existing seam.
- `design:review-feedback-capture` (live) — the ledger is a *candidate* measurement home (DQ-2); the PRD parks the mirror as WR-3 and fixes the product bar (harvestable from the proposal alone), so the boundary is a design choice with a floor, not fog.
- `design:implement-now` VerificationGate class (live) — the seam NR-2 rides; one integration class, two consumers (codegate + impl-review). Extends the queued seam.

No live design covers the proposal-class reviewer; the machine-harvestable-finding seam is open ground.

## Design questions

- **DQ-1**: What is the finding/report artifact — a human-markdown report in the sibling `review-report/v1` pattern (JSON deferred), or a **machine-native payload** (closed schema embedded in the PR-resident surface) extending codegate's finding core — and how does it persist across the two media (GitHub comment marker block · Carina decision-record) while staying harvestable-without-free-text (FR-13) and attribution-bound (FR-16)?
- **DQ-2**: Where does measurement live — the harvest reads the PR-resident payloads directly (the FR-13 floor), or mirrors dispositions into the review-feedback-capture ledger (WR-3) — and what owns the M4 capture line?
- **DQ-3**: What runs the lenses — a standalone `/impl-review` skill spawning fresh-context subagents (the sibling-skill precedent), a codegate-style binary, or an engine service — and where do the eval protocol (run-count N, decode pinning) and the fresh-instance attestation (FR-2/NFR-3) live?
- **DQ-4**: Does the deep band run a second independent lens instance per blocking finding before adoption (Q3, perspective-diverse verify) — a design-time seam, or an adoption-time operator dial?

## Panel log

- **DQ-1**: opens (a new artifact/schema class + its persistence seam, breaking the sibling pattern) — fired — genuinely distinct shapes exist (human-markdown-mirror vs machine-native-payload vs dual-face), with different FR-13/harvest consequences
- **DQ-2**: opens (the measurement data-home seam, spanning two features) — fired — two structurally distinct homes (PR-resident-direct vs ledger-mirror) with different ownership and coupling
- **DQ-3**: extends (the fresh-context lens-execution model proven in the sibling reviewer skills; the review context pack extends the live docgate context-pack; eval protocol + fresh-instance attestation compose existing surfaces) — skipped — the runtime model is the sibling-skill precedent and the pack is an existing seam extended; the artifact-shape choice (skill + shared pack) is a ruling, not a new seam class
- **DQ-4**: extends (a config-gated verify path over the D-3 subagent runtime; the pack makes the second instance cheap) — skipped — the perspective-diverse-verify pattern is proven; the question is build-the-dormant-seam vs commit-now, a ruling not a new seam class

## Decisions

### D-1 — the artifact: a machine-native payload extending codegate's finding core, dual-rendered

- **Question**: DQ-1 — human-markdown report (sibling pattern), machine-native payload, or dual-artifact — given FR-13/FR-16 break the siblings' JSON-deferral?
- **Answer**: **ALT-2** — a closed, versioned `impl-review-report/v1` schema composing codegate's finding-core struct (read-only reuse, NR-1) + review-only fields (lens, calibration-row, disposition + actor-attribution, reviewed-revision); persisted as a machine block inside the PR-resident surface (fenced payload in the GitHub marker comment / typed decision-record body on Carina) with a human report rendered above it from the same payload — one source of truth. Kinship is composition, not edit (Q2 resolved).
- **Reasoning**: the only shape meeting FR-13 (harvest the block, never prose) + FR-16 (attribution) + D-9 (machine corpus) while keeping a readable report; the siblings' JSON-deferral was licensed by a human consumer this feature's harvest + WR-2 corpus is not; dual-artifact (ALT-3) reintroduces the state⇄verdict divergence codegate rejected. Recommendation adopted.
- **Anchor**: 2026-07-17 · t1
- **Status**: live

### D-2 — measurement: harvest the PR-resident payloads directly; the ledger stays a future derived mirror

- **Question**: DQ-2 (Q1) — measure from the PR-resident payloads directly, or mirror into the review-feedback-capture ledger?
- **Answer**: **ALT-4** — measurement is a checkpoint harvest that parses the `impl-review-report/v1` blocks from a repo's proposals (coverage/precision/nit/miss over the parsed corpus); no second store; M4 capture lines read the same payloads. The review-feedback-capture ledger mirror stays WR-3 — reachable later as a *derived* mirror, never the source.
- **Reasoning**: exactly the FR-13 floor (harvestable from the proposal alone); one data home, zero cross-feature coupling, NFR-4 trivial; the ledger would have to be a derived mirror regardless (FR-13 forces the payload authoritative), so making it the source only adds coupling + the divergence class. O(corpus) re-parse is a stated note at today's scale. Recommendation adopted.
- **Anchor**: 2026-07-17 · t2
- **Status**: live

### D-3 — the runner: an /impl-review skill spawning fresh-context subagents over one shared review context pack

- **Question**: DQ-3 (Q4) — skill or binary; and where do the eval protocol + fresh-instance attestation + cost live?
- **Answer**: the **`/impl-review` skill** spawning **four parallel fresh-context subagents** (the sibling-reviewer runtime, proven live this session), the host invoking it at the proposal boundary. **Cost move (operator, t3):** a **shared review context pack** — assemble the common input **once** (diff + charter + codegate verdict + governing-design binding elements + calibration) and seed all four subagents from it, rather than each re-loading — reusing/extending the live `docgate context-pack` machinery, not a new one. **Independence guard:** the pack is shared read-only *input*; each lens's *judgment* stays independent and blind to the others' findings (FR-2 = distinct from the implementing session **and** from each other's conclusions — the adversarial screen depends on it). Eval fixtures, run-count N, and band budgets are versioned data beside the skill glue (Q4 = data, PR-tuned); fresh-instance attestation (NFR-3) = the subagent context identity in the payload provenance.
- **Reasoning**: mirrors the two proven reviewers (program symmetry); the machine-payload artifact (D-1) is the only forced divergence; the context pack cuts the dominant cost (four re-reads of one large input → one build), and the docgate context-pack precedent means reuse-not-rebuild (AP-11). A binary buys none of codegate's determinism for LLM orchestration while forfeiting the runtime and symmetry.
- **Anchor**: 2026-07-17 · t3
- **Status**: live

### D-4 — deep-band second-instance verify: build the dormant seam, ship it off, flip at adoption on data

- **Question**: DQ-4 (Q3) — does the deep band run a second independent lens instance per blocking finding, always-on, off, or as a dial?
- **Answer**: **build the seam, ship it off** — a deep-band config flag (versioned data, like the band budgets) that when on re-runs each blocking finding through a **second independent subagent** (perspective-diverse verify: an independent confirm, else downgrade); off in v1, the operator flips it at the Phase-C adoption decision if dogfood precision shows deep-band false positives.
- **Reasoning**: honors the PRD's routing of Q3 as an adoption-time operator dial (owner @mdarabi, needed-by the adoption decision); the context pack (D-3) makes the second instance cheap (same pack, one more subagent); building the dormant seam turns a potential re-design into a data flip. Always-on pre-empts the operator's dial before precision data exists; no-seam leaves the adoption gate no lever. Recommendation adopted.
- **Anchor**: 2026-07-17 · t4
- **Status**: live

### D-5 — one source of truth: one medium per proposal, one self-contained artifact; compactness at the source

- **Question**: (operator, at the design review gate) should a proposal's review information live across multiple media (Carina + GitHub), and how is a large payload handled?
- **Answer**: **no cross-platform sharing** — a proposal is raised on exactly one medium and its complete review lives there as **one self-contained artifact** (one source of truth); the two persistence adapters exist only because pyxis supports two code-hosts, and a proposal uses one. A large payload is **not** a persistence problem to engineer around (no continuation/spill — that would itself break one-source-of-truth); it is a **verbosity defect fixed in the skill prompts** (findings terse, the sibling NFR-2 posture), bounded by a compactness eval. 64KB is ample for a compact code review.
- **Reasoning**: operator's — "wherever the PR is raised should include all the information; one source of truth is a design principle; if artifacts get big, update the prompts so the LLMs aren't verbose." Supersedes the tech-design-review's FND-5 disposition (which had added GitHub continuation/spill machinery — the wrong answer, and itself a split-artifact violation).
- **Anchor**: 2026-07-17 · review
- **Status**: live

### D-6 — build order: GitHub first, Carina later

- **Question**: (operator, at the design review gate) which persistence medium ships first — Carina (the cleaner attribution surface) or GitHub?
- **Answer**: **GitHub first; Carina later.** The design authored Carina-first (RW-4 substrate-first); the operator reversed it — the dogfood proposals are GitHub PRs (the codegate epic), so v1 must review where the real proposals live.
- **Reasoning**: operator's — "I want to support GitHub first; Carina can be done later." Consequence surfaced and accepted: GitHub-first puts the anti-forgery attribution mechanism on the v1 critical path, so §3.2 now *specifies* it for v1 (an operator-authored PR review/comment as the disposition artifact) rather than deferring it; only the squash-merge survival remains an integration detail (Q3).
- **Anchor**: 2026-07-17 · review
- **Status**: live

## Alternatives

### ALT-1 — human-markdown report, sibling `review-report/v1` pattern (JSON deferred)

- **Shape**: mirror prd-review-artifacts exactly — one typed-markdown report per proposal at a `docs/reviews/`-analog (or on the PR), findings/dispositions/verdict as markdown sections, calibration as markdown beside the skill; the harvest greps structured markdown.
- **Trade-offs**: maximal sibling uniformity, zero new schema machinery, proven; but FR-13's "harvestable **without reading free text**" fails by construction (markdown *is* free text), M2/nit computation becomes brittle grep, and attribution (FR-16) has no typed home — the exact reasons the PRD's D-9 refused free-form.
- **Principles**: AP-23 (parallel-authorship-safe markdown) honored; violates the basis's FR-13/D-9 (a merged contract) — not a principle deviation, a requirement violation.
- **Incidents**: the predecessor's "README that lied" markdown-drift class — untyped review data rots the same way.
- **Verdict**: rejected — cannot meet FR-13/FR-16 (basis requirements); the sibling pattern's JSON-deferral was licensed by a *human* consumer, which this feature's harvest + WR-2 corpus is not.
- **From**: DQ-1 · panel

### ALT-2 — machine-native finding payload extending codegate's core, dual-rendered

- **Shape**: a closed, versioned finding/report **schema** (`impl-review-report/v1`) that composes codegate's finding-core struct (read-only, NR-1) + review-only fields (lens, cal-row, disposition+actor-attribution, reviewed-revision); persisted as a **machine payload block inside** the PR-resident surface (a fenced JSON block in the GitHub marker comment — the spec-marker pattern; a typed decision-record body on Carina) with a **human rendering above it** (the sibling report shape, generated from the payload). The harvest parses the block, never the prose; attribution reads the medium's own authorship of the disposition act.
- **Trade-offs**: meets FR-13 (harvest the block) + FR-16 (attribution) + D-9; the human still gets a readable report (rendered from the payload, one source of truth); costs a real schema + a two-media serializer. Kinship is composition, not edit — NR-1 holds.
- **Principles**: AP-6 (closed schema, parsed closed) · AP-13 (reviewed-revision in the payload, UB-6) · AP-11 (one finding-grammar family via codegate-core reuse) · AP-21 (judge-once: the payload is the frozen record downstream reads) — all honored.
- **Incidents**: I2/AP-26 (badge true of the wrong object) closed by the reviewed-revision field; the codegate bundling lesson argues for un-bundled schema reuse (composition, not a forked copy).
- **Verdict**: chosen — the only shape meeting FR-13/FR-16/D-9 with a human-readable rendering; kinship by composition (D-1)
- **From**: DQ-1 · panel

### ALT-3 — dual-artifact: machine payload in a sidecar, human report as the sibling markdown

- **Shape**: two artifacts — a machine JSON sidecar (harvest source) *and* a separate human-markdown report (sibling pattern), both on the proposal.
- **Trade-offs**: each consumer gets its ideal face; but two artifacts = two consistency domains (the state⇄verdict divergence class codegate's D-4 rejected), doubled write/staleness handling (UB-5/UB-6 ×2), and attribution split across both — the divergence risk the single-payload-with-rendering (ALT-2) removes by construction.
- **Principles**: AP-6 honored twice; strains the one-consistency-domain shape and AP-13 (two revision bindings to keep in sync).
- **Incidents**: the codegate incremental-state divergence class (two records of one truth) — directly analogous.
- **Verdict**: rejected — two consistency domains reintroduce the divergence class ALT-2 removes by construction (D-1)
- **From**: DQ-1 · panel

## Seam map

- **S-1**: the `impl-review-report/v1` schema — **opens** — behind it: a closed versioned finding/report schema composing codegate's finding-core (read-only) + review-only fields; harvest-parseable, attribution-bound, revision-bound — feeds: the design's Scope ("the finding/report schema") + Serves prd:impl-review/FR-5, FR-13, FR-16, UB-6 (D-1)
- **S-2**: the two-media persistence + rendering — **opens** — behind it: a serializer writing the machine block + rendering the human report above it, per medium (GitHub marker comment · Carina decision-record) — feeds: Serves FR-6; §5.5 both-media rows (D-1)

### ALT-4 — the harvest reads the PR-resident payloads directly (the FR-13 floor)

- **Shape**: measurement is a harvest pass that enumerates a repo's proposals, parses each `impl-review-report/v1` block from the medium, and computes coverage/precision/nit/miss over the parsed corpus — no second store; the review-feedback-capture ledger is untouched. M4 capture lines read the same payloads (review presence + verdict), not a new event stream.
- **Trade-offs**: exactly the FR-13 product bar, one data home, zero cross-feature coupling; the payload *is* the corpus, so NFR-4 (reconstruct from the proposal alone) holds trivially. Cost: harvest re-parses the proposal set each checkpoint (O(corpus) — a note, tens of proposals today); the ledger mirror stays a future option (WR-3), not a dependency.
- **Principles**: AP-24 (the corpus is derived from the proposals, governs nothing) · AP-19 (idle cost zero — the harvest runs at checkpoints, not continuously) · AP-11 (one measurement predicate over one home) honored.
- **Incidents**: none matched (registry checked 2026-07-17); the ledger-coupling alternative risks the governance-derived-from-governed class if the mirror ever became authoritative.
- **Verdict**: chosen — the FR-13 floor, one home, no cross-feature coupling (D-2)
- **From**: DQ-2 · panel

### ALT-5 — mirror dispositions into the review-feedback-capture ledger, measure from there

- **Shape**: on each disposition, write a federated-closable entry into the review-feedback-capture ledger; measurement reads the ledger (the human-review-feedback corpus + machine-review dispositions unified).
- **Trade-offs**: one unified feedback corpus across human and machine review (the VNEXT-15/VNEXT-3 dream); but a hard cross-feature dependency at Phase B (that feature's schema must admit machine dispositions), two writes per disposition (PR payload + ledger — the divergence class again), and the FR-13 floor ("harvestable from the proposal alone") demands the PR payload be authoritative anyway, making the ledger a *derived mirror*, not the source.
- **Principles**: AP-5 (traceability) attractive; strains AP-11 (two homes) and the FR-13 floor; the mirror-not-source shape is the only AP-24-safe form.
- **Incidents**: the codegate two-records-of-one-truth divergence class if the ledger and payload disagree.
- **Verdict**: rejected for now — reachable later as WR-3's derived mirror if a unified human+machine corpus becomes load-bearing (D-2)
- **From**: DQ-2 · panel

## Seam map addendum

- **S-3**: the measurement harvest — **opens** — behind it: a checkpoint pass parsing the PR-resident payloads into the coverage/precision/nit/miss corpus; the ledger mirror deferred (WR-3) — feeds: Serves FR-13; §3.2 M1/M2/M4 computation; §3.3 the ship rule (D-2)
- **S-4**: the `/impl-review` skill + the review context pack — **extends** (the sibling-reviewer skill runtime + the live docgate context-pack) — behind it: the host-invoked skill spawning four fresh-context subagents seeded from one shared read-only input pack; per-lens judgment independent + finding-blind; eval fixtures + N + budgets as data beside the glue — feeds: Serves FR-1, FR-2, FR-3, FR-10, NFR-1; §5.9 the eval suite (D-3)

## Facts

- **F-1**: the sibling reviewer designs ship typed **markdown** reports + markdown calibration, with JSON-native mechanization **deliberately parked for the engine era** — licensed by a human report-consumer — source: docs/tech-design/prd-review-artifacts.md §2
- **F-2**: codegate's finding schema (check_id·class·rung·path·line·message·log_ref) is frozen in `internal/verify/schema` as implement-now's joint feedback contract; impl-review NR-1 = read-only reuse, zero codegate change — source: docs/tech-design/codegate-v1.md §3.6; prd:impl-review NR-1/FR-5
- **F-3**: the basis fixes the product bars the design must meet: FR-13 harvestable-without-free-text, FR-16 attribution-bound dispositions, D-9 machine corpus, UB-6 reviewed-revision binding, FR-7/NR-4 downgrade-only into the lane's GATE-4 slot — source: docs/prds/impl-review/effective.md
- **F-4**: the merge lane's GATE-4 downgrade-only slot and its risk-class data are live (`merge-lane-v1`); the reviewer wires into the slot and reads the classes, adding no lane logic — source: docs/tech-design/merge-lane-v1.md; prd:impl-review FR-9/NR-4
- **F-5**: the fresh-context lens execution model has a working precedent — the prd-review/tech-design-review skills spawn parallel subagents in fresh contexts, adversarially screen, and cite calibration (driven live this session) — source: the sibling reviewer skills
- **F-6**: Carina's typed decision-records replace GitHub's HTML-comment marker grammar with typed, namespaced structured data (the second persistence medium FR-6 targets) — source: docs/tech-design/local-work-tracker-v2.md §3.1

## Fog

none

## Out of scope

- **O-1**: the review-feedback-capture ledger mirror as the measurement source — deferred to WR-3 as a derived mirror if a unified human+machine feedback corpus becomes load-bearing; never the source while FR-13's proposal-alone floor holds (from D-2)
- **O-2**: a pinned reviewer binary (codegate's shipping shape) — rejected: LLM orchestration gains none of codegate's determinism from pinning while forfeiting the proven skill runtime and program symmetry (from D-3)
- **O-3**: always-on deep-band second-instance verify — not shipped; the seam is built dormant and flipped on data at adoption (from D-4)

## Candidate glossary terms

- **T-1**: **review context pack** — the shared read-only input (diff + charter + codegate verdict + governing-design binding elements + calibration) assembled once and seeding all four lens subagents, extending the docgate context-pack; shared *input*, never shared *judgment* · avoid: bare *context pack* (docgate's is stage-scoped; this is the review-class instance), *shared context* (implies shared reasoning — the lenses stay finding-blind)
- **T-2**: **impl-review-report/v1** — the closed versioned finding/report schema composing codegate's finding-core (read-only) + review-only fields (lens, calibration-row, disposition+attribution, reviewed-revision), persisted as the machine block inside the PR-resident surface with a human rendering derived from it · avoid: *review report* (the sibling markdown class — this is machine-native), *finding schema* alone (it carries the whole report, not one finding)

## Parked product gaps

- **P-1**: the technical-design class has no §0 read-to-rule (human-reviewer-routing) section — read-to-rule shipped PRD-only — anchors: `prd:read-to-rule` (prd-template-scoped); `docs/templates/tech-design-template.md` (no §0) — disposition: **proceed-anyway** — a §0 was piloted on `impl-review-v1` at the operator's review gate (2026-07-17 · review); the class-wide template/conventions adoption is tracked as agenticlab-ai/pyxis#411, not blocking this design

## Gate

| Field | Value |
|---|---|
| **Verdict** | ready-for-design |
| **Date** | 2026-07-17 |
| **Authority** | @mdarabi |
| **Covering design** | — |
| **Coverage check** | pass — core four covered, zero blocking fog |
| **Amend route** | — |
