# Workbench UI (the reading room) — design-brainstorm record

| Field | Value |
|---|---|
| **In one line** | The reading room's solution shape: a pure-Node, zero-Go feature — the UI reads the corpus directly (machine faces first, markdown for faceless classes), serves loopback with a secret, and composes the existing renderers; the Go-snapshot and two-process shapes were considered and rejected by the authority. |
| **Relevant-when** | designing or changing the reading room's corpus reader, server lifecycle, search, progress derivation, or renderer composition; checking prior design rulings on the UI's language boundary or refused shapes (Go snapshot emitter, Go API server, Go launcher) |
| **Basis** | prd:workbench-ui (merged ledger) |
| **Authority** | @mdarabi |
| **Date(s)** | 2026-07-10 |
| **Status** | gated: ready-for-design |
| **Tracking** | — *(the authored design's introducing PR, once filed)* |
| **Feeds** | docs/tech-design/workbench-ui-v1.md |
| **Prior records** | brainstorm:workbench-ui |
| **Template version** | design-brainstorm-record/v1 |

## Orientation

| Source | Why loaded |
|---|---|
| docs/prds/workbench-ui/effective.md | the basis: FR-1..9, UB-1..4, NR-1..3, ABUSE-1..3, NFR-1..4, NG/WR perimeter, Q1–Q5 (this session's charter) |
| docs/tech-design/index.md | live designs whose Serves adjoin the basis; liveness ground for every source below |
| docs/tech-design/plan-viewer-v1.md | the composed plan surface: Approved; `render` verb emits one self-contained page — the composition target FR-2 names |
| docs/tech-design/distributed-check-registration.md | the house registry pattern for extension-by-addition (NFR-4's implementation precedent, language-neutral) |
| docs/tech-design/solution-shape-v1.md | the authored design must pair concrete schemas (reader output, statefile, routes) as a solution shape, not carry them in-body |
| docs/tech-design/repo-layout-and-system-footprint.md | the managed-footprint home, release channel + resolver, content-purity (FR-31) constraints on the authored docs |
| docs/tech-design/prd-amendment-model.md + tech-design-conventions.md | the machine faces (effective.source.json; index.json) — generated, schema-versioned, one-writer contracts the reader can consume without twin parsing |
| docs/tech-design/plan-new-v1.md | the plan class's two faces the reader consumes for catalog/progress |
| docs/observability/incidents.md | I1–I3 reviewed: I3 (fail-closed argument parsing on consequential CLIs) applies to the node entry; I1/I2 process-class, n/a |
| docs/wiki/glossary.md | vocabulary (reading room, ledger, projection, routing surface) |
| docs/brainstorms/workbench-ui.md + reviewer-artifacts-tech-design.md | the stage-1 rulings this session must not re-ask; the stage-3 record precedent |
| AGENTS.md | language policy (the carve-out P-1 rides on), review-tooling rules, footprint table |

**Overlaps surfaced:** no live design decides browser access to the corpus; plan-viewer-v1 decides plan *rendering* only (composed, not re-litigated — its NG-2 boundary was already handled at the basis's stage 1); the record-model designs decide record grammar, consumed read-only here. `docs/system-design.md` is uncommitted work-in-progress and was not loaded.

## Design questions

- **DQ-1**: where does corpus knowledge live — how does the Node UI obtain catalog, anchors, citations, search text, and progress inputs?
- **DQ-2**: the launch story and the Go side's home — what does the operator type; which binary (if any) owns emission/launching?
- **DQ-3**: search mechanics — index shape, anchor-ID lookup, dependency posture.
- **DQ-4**: server lifecycle — port, secret, single-instance, shutdown/idle.
- **DQ-5**: progress derivation — the stage function over corpus signals.
- **DQ-6**: annotation hand-off — reaching the doc-review loop without owning review machinery.
- **DQ-7**: runtime dependency and build posture for the UI's own code.

## Panel log

- **DQ-1**: opens — fired — the UI↔doc-system boundary is a new seam; the language split shapes every consumer (three shapes scored: ALT-1..3)
- **DQ-2**: opens — skipped — the entry-point seam collapsed once DQ-1 chose pure Node (no Go component exists to home); the residual is one ruling (D-2)
- **DQ-3**: extends — skipped — search rides the DQ-1 reader's output; mechanics are one ruling (D-3a)
- **DQ-4**: extends — skipped — lifecycle extends the review-runtime's proven statefile/port/secret pattern (D-3b)
- **DQ-5**: extends — skipped — the stage chain is derivation over the DQ-1 reader, not a new seam (D-3c)
- **DQ-6**: extends — skipped — the hand-off consumes the doc-review contract as-is (D-3d)
- **DQ-7**: extends — skipped — applies the house dependency posture to the Node side (D-3e)

## Decisions

### D-1 — the corpus boundary: Node parses everything (ALT-1)

- **Question**: which of the three paneled shapes carries corpus knowledge (DQ-1)?
- **Answer**: ALT-1 — the Node UI reads the corpus directly; no Go snapshot verb, no Go API. Chosen over the recommendation (ALT-2).
- **Reasoning**: the authority's, after the distribution question was answered (F-3 — shipping parity across shapes): the feature stays one self-contained footprint artifact with zero Go surface to build, version, or resolve. The stated cost is accepted: JS-side parsing duplicates class knowledge (AP-11 strain) — bounded at design time by machine-faces-first reading (F-1) so genuine twin parsing shrinks to the faceless markdown classes (G-1).
- **Anchor**: 2026-07-10 · batched round (4 questions, recommendations attached; shipping clarification preceded)
- **Status**: live

### D-2 — no Go launcher; the entry is a documented Node invocation

- **Question**: what does the operator type, and does any Go binary own launch (DQ-2)?
- **Answer**: no Go launcher and no new Go verb anywhere in this feature; the session starts by a documented node invocation of the UI's entry in the managed footprint. Chosen over the recommendation (new `cmd/workbench`).
- **Reasoning**: consistent with D-1's zero-Go thesis; the language-policy carve-out (P-1 of the basis record) therefore covers the executable entry point too — its exact wording and any thin-shim question ride the design (G-2). I3's fail-closed argument-parsing lesson binds the node entry from birth.
- **Anchor**: 2026-07-10 · batched round
- **Status**: live

### D-3 — the mechanics set, delegated and confirmed as recommended

- **Question**: confirm (a)–(e) as rulings (DQ-3..7)?
- **Answer**: all as recommended — (a) search: in-memory index from the reader's output, anchor-ID exact match ranked first, zero search dependencies; (b) lifecycle: ephemeral port + session-secret URL + loopback, per-repo single-instance refusal via a statefile in the review-runtime's pattern, exit with the terminal, generous idle auto-exit (~2h); (c) progress: stage chain record→PRD→design→plan with attached review verdicts, from the corpus only; beyond-corpus states render honestly-unknown; (d) hand-off: present + copy the exact /doc-review invocation, link the tracked PR, never run the loop; (e) zero runtime npm dependencies — node:http, server-rendered vanilla ESM, no bundler or build step; any future dep is a justified-decision entry.
- **Reasoning**: delegation confirmed by the authority in one ruling; each item lands as a concrete design section, residual values as the design's Q-n (G-3, G-4).
- **Anchor**: 2026-07-10 · batched round
- **Status**: live

## Alternatives

### ALT-1 — Node parses everything

- **Shape**: the Node server reads the working tree directly per request — generated machine faces (ledger projections, the design index's JSON, plan folds) as first-class inputs, markdown parsing only for classes without a machine face; a JS-side class registry admits new classes by addition; renderers composed, not reimplemented.
- **Trade-offs**: one self-contained footprint artifact, zero Go build/version/resolve surface, live reads make FR-9 free; costs the faceless-class grammar knowledge existing twice (JS + the Go tools), to be pinned by conformance fixtures against drift.
- **Principles**: AP-11 strained (twin knowledge for faceless classes; machine-faces-first bounds it, fixtures pin it) · AP-6 honored (consumes the existing one-writer JSON contracts) · AP-27 honored (class registry, extension by addition — NFR-4) · AP-19 honored (idle-zero lifecycle) · AP-25 honored (vendor-neutral footprint home).
- **Incidents**: I3 applies to the node entry's argument handling (fail closed on misparse); I1/I2 n/a — registry reviewed 2026-07-10.
- **Verdict**: chosen (D-1) — authority's ruling over the recommendation; self-containment valued above the single-parser property.
- **From**: DQ-1 · panel

### ALT-2 — Go derives, Node serves (the recommendation)

- **Shape**: a Go snapshot verb reuses the internal parsers to emit one schema-versioned JSON corpus snapshot; Node consumes only the snapshot; refresh by re-emit.
- **Trade-offs**: one parser per class (no twin knowledge), class extensibility lands where the parsers live; costs a Go emit surface + a refresh loop, and the feature spans two languages and two release artifacts.
- **Principles**: AP-11 honored · AP-6 honored (the snapshot is a contract) · AP-10 honored; strains nothing structural.
- **Incidents**: I3 would apply to the new verb; none else — registry reviewed 2026-07-10.
- **Verdict**: rejected — authority's ruling (D-1): not worth a Go surface on a bootstrap-era human tool; distribution parity (F-3) removed the shipping argument in both directions, leaving surface-count decisive.
- **From**: DQ-1 · panel

### ALT-3 — two live processes (Go API + Node UI)

- **Shape**: a Go API server (a verb on the shipped tool set) owns the data plane; the Node server presents and proxies.
- **Trade-offs**: cleanest language split; doubles the runtime surface — two lifecycles, ports, secrets, failure modes — for a single-user local tool.
- **Principles**: AP-11 honored · AP-19 strained (two live processes idling) · AP-22 strained (two-process startup/shutdown races to make convergent).
- **Incidents**: none matched — registry reviewed 2026-07-10.
- **Verdict**: rejected — the doubled lifecycle buys nothing a batch emit (ALT-2) or direct reads (ALT-1) don't provide here.
- **From**: DQ-1 · panel

## Seam map

- **S-1**: the corpus reader — opens — behind it: machine-faces-first reading, faceless-class markdown parsing, the JS class registry — feeds: Serves prd:workbench-ui/FR-1 · FR-3 · FR-4 · FR-5 · FR-9 · NFR-4
- **S-2**: the serving surface — opens — behind it: node:http loopback server, session secret, statefile lifecycle, idle exit — feeds: Serves prd:workbench-ui/FR-7 · FR-8 · UB-1 · UB-4 · NFR-1 · NFR-2 · NFR-3 · ABUSE-1..3
- **S-3**: renderer composition — extends — behind it: invoking the codified doc renderer and the plan renderer's page, degradation per UB-2 — feeds: Serves prd:workbench-ui/FR-2 · UB-2 · UB-3 · NR-2
- **S-4**: the annotation hand-off — extends — behind it: present-the-invocation against the doc-review contract — feeds: Serves prd:workbench-ui/FR-6
- **S-5**: the footprint home + language carve-out — extends — behind it: the UI's directory in the managed footprint; the operator-authored policy carve-out riding implementation — feeds: the design's Scope + dependency table

## Facts

- **F-1**: generated, schema-versioned machine faces already exist for ledgers (`effective.source.json`), the design corpus (the index's JSON face), and plans (the fold's machine face) — one writer each — sources: prd-amendment-model · tech-design-conventions · plan-new-v1
- **F-2**: the plan renderer is implemented: a verb that folds in memory and emits one self-contained review page — sources: the plan tool's usage; plan-viewer-v1 (Approved)
- **F-3**: distribution parity — any Go component would ship inside the repo's existing release artifacts (tagged releases, resolver, per-platform sha256 pins; source mode today), never a separate install — answered to the authority before the DQ-1 ruling — sources: repo-layout-and-system-footprint; the release-channel implementation
- **F-4**: the review-surface runtime already proves the Node lifecycle pattern the UI adopts: vendored runtime, setup-once deps, per-file statefile with PID/port/secret, probe-and-degrade — sources: the lavish driver + vendor README
- **F-5**: the language policy makes every new executable artifact Go by default — a pure-Node feature including its entry point needs the operator-authored carve-out at implementation — source: AGENTS.md (basis record P-1)

## Fog

- **G-1** (non-blocking — blocks: design Q-n): the faceless-class inventory — exactly which classes lack machine faces and need JS-side markdown parsing, and how those grammars are pinned against drift (conformance fixtures vs the writing tools' goldens)
- **G-2** (non-blocking — blocks: design Q-n): the exact entry surface — the documented node invocation's form, whether any thin shim is admissible (bash burden-of-proof), and the carve-out's proposed wording for the operator
- **G-3** (non-blocking — blocks: design Q-n): statefile schema and the idle-timeout value; single-instance semantics across worktrees of one repo
- **G-4** (non-blocking — blocks: design Q-n): read/caching strategy per request class so live reads (FR-9) and the performance budgets (NFR-1) hold together at 1,000 docs
- **G-5** (non-blocking — blocks: design Q-n): locating and invoking the composed Go renderer from Node across source mode and release mode (the resolver), and the degradation when neither resolves (UB-2)

## Out of scope

- **O-1**: any Go code in this feature — no snapshot verb, no API server, no launcher, no new binary (from D-1, D-2)
- **O-2**: any bundler, build step, or runtime npm dependency in the UI (from D-3e)

## Candidate glossary terms

none

## Parked product gaps

none

## Gate

| Field | Value |
|---|---|
| **Verdict** | ready-for-design |
| **Date** | 2026-07-10 |
| **Authority** | @mdarabi |
| **Covering design** | — |
| **Coverage check** | pass — core four covered, zero blocking fog (§2 Decision ← D-1, DQ-1 panel-logged · §4 Alternatives ← ALT-1..3, all verdicted · Serves/Scope ← S-1..S-5 + Basis · §3.2 boundaries ← O-1..O-2 atop the basis NG set) |
| **Amend route** | — |
