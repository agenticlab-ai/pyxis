# Workbench UI (the reading room) — design-brainstorm record

| Field | Value |
|---|---|
| **In one line** | The reading room's solution shape: one Go service reads the corpus directly (machine faces first, safe presentation for faceless classes), serves loopback with a secret, and reuses the Go plan presenter. |
| **Relevant-when** | designing or changing the reading room's corpus reader, server lifecycle, search, progress derivation, document presentation, or Go entry point; checking prior design rulings on precomputed snapshots or split-process shapes |
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
| docs/observability/incidents.md | I1–I3 reviewed: I3 (fail-closed argument parsing on consequential CLIs) applies to the Go entry; I1/I2 process-class, n/a |
| docs/wiki/glossary.md | vocabulary (reading room, ledger, projection, routing surface) |
| docs/brainstorms/workbench-ui.md + reviewer-artifacts-tech-design.md | the stage-1 rulings this session must not re-ask; the stage-3 record precedent |
| AGENTS.md | Go-by-default language policy, review boundary, and footprint table |

**Overlaps surfaced:** no live design decides browser access to the corpus; plan-viewer-v1 decides plan *rendering* only (composed, not re-litigated — its NG-2 boundary was already handled at the basis's stage 1); the record-model designs decide record grammar, consumed read-only here. `docs/system-design.md` is uncommitted work-in-progress and was not loaded.

## Design questions

- **DQ-1**: where does corpus knowledge live — how does the Go UI obtain catalog, anchors, citations, search text, and progress inputs?
- **DQ-2**: the launch story — what does the operator type, and which Go binary owns the server?
- **DQ-3**: search mechanics — index shape, anchor-ID lookup, dependency posture.
- **DQ-4**: server lifecycle — port, secret, single-instance, shutdown/idle.
- **DQ-5**: progress derivation — the stage function over corpus signals.
- **DQ-6**: conversation hand-off — returning an exact source reference without owning review machinery.
- **DQ-7**: runtime dependency and build posture for the UI's own code.

## Panel log

- **DQ-1**: opens — fired — the UI↔doc-system boundary is a new seam; direct reads versus a precomputed snapshot shape every consumer (three shapes scored: ALT-1..3)
- **DQ-2**: opens — skipped — the entry-point seam collapses into the single Go service selected by DQ-1; the residual is one ruling (D-2)
- **DQ-3**: extends — skipped — search rides the DQ-1 reader's output; mechanics are one ruling (D-3a)
- **DQ-4**: extends — skipped — lifecycle uses the repository's established loopback statefile/port/secret pattern (D-3b)
- **DQ-5**: extends — skipped — the stage chain is derivation over the DQ-1 reader, not a new seam (D-3c)
- **DQ-6**: extends — skipped — the hand-off returns an exact source reference to the operator conversation (D-3d)
- **DQ-7**: extends — skipped — applies the Go standard-library-first dependency posture (D-3e)

## Decisions

### D-1 — the corpus boundary: one Go process reads and presents the corpus (ALT-1)

- **Question**: which of the three paneled shapes carries corpus knowledge (DQ-1)?
- **Answer**: ALT-1 — one Go process reads the working tree directly, consumes existing machine faces, safely presents faceless markdown classes, and serves the UI. No precomputed snapshot or proxy process is introduced.
- **Reasoning**: one Go process keeps class knowledge, lifecycle, and distribution in the repository's normal implementation path. Machine-faces-first reading prevents re-deriving existing contracts, while the class registry confines the small faceless grammar surface.
- **Anchor**: 2026-07-10 · batched round (4 questions, recommendations attached; shipping clarification preceded)
- **Status**: live

### D-2 — the entry is the released Go binary

- **Question**: what does the operator type, and which binary owns launch (DQ-2)?
- **Answer**: the released `workbench-ui` Go binary owns the complete session and is invoked through the system front door in installed mode or directly in source mode.
- **Reasoning**: the entry follows the existing binary release and resolver path, adds no shell logic, and applies I3's fail-closed argument-parsing lesson from birth.
- **Anchor**: 2026-07-10 · batched round
- **Status**: live

### D-3 — the mechanics set, delegated and confirmed as recommended

- **Question**: confirm (a)–(e) as rulings (DQ-3..7)?
- **Answer**: all as recommended, with the implementation language later normalized to the repository's Go policy — (a) search: in-memory index from the reader's output, anchor-ID exact match ranked first, zero search dependencies; (b) lifecycle: ephemeral port + session-secret URL + loopback, per-repo single-instance refusal via a statefile, exit with the terminal, generous idle auto-exit (~2h); (c) progress: stage chain record→PRD→design→plan with attached review verdicts, from the corpus only; beyond-corpus states render honestly-unknown; (d) hand-off: present + copy the exact source path and tracked reference, never run a review loop; (e) one Go server using standard-library HTTP and templates, with no runtime package installation.
- **Reasoning**: delegation confirmed by the authority in one ruling; each item lands as a concrete design section, residual values as the design's Q-n (G-3, G-4).
- **Anchor**: 2026-07-10 · batched round
- **Status**: live

## Alternatives

### ALT-1 — one Go process reads and presents everything

- **Shape**: one Go server reads the working tree directly per request — generated machine faces (ledger projections, the design index's JSON, plan folds) as first-class inputs, header-and-anchor scanning only for classes without a machine face; a Go class registry admits new classes by addition; the plan presenter is reused in-process.
- **Trade-offs**: one self-contained binary and one lifecycle; live reads make FR-9 direct. The faceless-class grammar still needs conformance fixtures against drift.
- **Principles**: AP-11 honored (one implementation path) · AP-6 honored (consumes the existing one-writer JSON contracts) · AP-27 honored (class registry, extension by addition — NFR-4) · AP-19 honored (idle-zero lifecycle) · AP-25 honored (vendor-neutral footprint home).
- **Incidents**: I3 applies to the Go entry's argument handling (fail closed on misparse); I1/I2 n/a — registry reviewed 2026-07-10.
- **Verdict**: chosen (D-1) — one process and the repository's standard implementation language.
- **From**: DQ-1 · panel

### ALT-2 — precomputed corpus snapshot plus static viewer

- **Shape**: a snapshot verb emits one schema-versioned JSON corpus snapshot that a static viewer consumes; refresh requires re-emission.
- **Trade-offs**: a compact presentation client, but the snapshot can drift from the working tree and adds a refresh lifecycle plus a second derived artifact.
- **Principles**: AP-11 honored · AP-6 honored (the snapshot is a contract) · AP-10 honored; strains nothing structural.
- **Incidents**: I3 would apply to the new verb; none else — registry reviewed 2026-07-10.
- **Verdict**: rejected — D-1 requires live working-tree reads and one process without a derived snapshot lifecycle.
- **From**: DQ-1 · panel

### ALT-3 — two live processes (data API + presentation server)

- **Shape**: one server owns the data plane while a second presents and proxies it.
- **Trade-offs**: clean separation; doubles the runtime surface — two lifecycles, ports, secrets, and failure modes — for a single-user local tool.
- **Principles**: AP-11 honored · AP-19 strained (two live processes idling) · AP-22 strained (two-process startup/shutdown races to make convergent).
- **Incidents**: none matched — registry reviewed 2026-07-10.
- **Verdict**: rejected — the doubled lifecycle buys nothing a batch emit (ALT-2) or direct reads (ALT-1) don't provide here.
- **From**: DQ-1 · panel

## Seam map

- **S-1**: the corpus reader — opens — behind it: machine-faces-first reading, faceless-class header-and-anchor scanning, the Go class registry — feeds: Serves prd:workbench-ui/FR-1 · FR-3 · FR-4 · FR-5 · FR-9 · NFR-4
- **S-2**: the serving surface — opens — behind it: standard-library Go loopback server, session secret, statefile lifecycle, idle exit — feeds: Serves prd:workbench-ui/FR-7 · FR-8 · UB-1 · UB-4 · NFR-1 · NFR-2 · NFR-3 · ABUSE-1..3
- **S-3**: document presentation — extends — behind it: safe Go presentation plus the in-process plan presenter, degradation per UB-2 — feeds: Serves prd:workbench-ui/FR-2 · UB-2 · UB-3 · NR-2
- **S-4**: the conversation hand-off — extends — behind it: present the exact source path and tracked reference — feeds: Serves prd:workbench-ui/FR-6
- **S-5**: the footprint home + release registration — extends — behind it: the Go binary's system-release entry — feeds: the design's Scope + dependency table

## Facts

- **F-1**: generated, schema-versioned machine faces already exist for ledgers (`effective.source.json`), the design corpus (the index's JSON face), and plans (the fold's machine face) — one writer each — sources: prd-amendment-model · tech-design-conventions · plan-new-v1
- **F-2**: the plan renderer is implemented: a verb that folds in memory and emits one self-contained review page — sources: the plan tool's usage; plan-viewer-v1 (Approved)
- **F-3**: distribution parity — any Go component would ship inside the repo's existing release artifacts (tagged releases, resolver, per-platform sha256 pins; source mode today), never a separate install — answered to the authority before the DQ-1 ruling — sources: repo-layout-and-system-footprint; the release-channel implementation
- **F-4**: existing local servers prove the loopback lifecycle pattern the UI adopts: per-repo state with PID/port/secret, probe-and-degrade, and terminal-owned shutdown — sources: repository-local server implementations
- **F-5**: the language policy makes every new executable artifact Go by default, and the system release already carries Go binaries — source: AGENTS.md and the installer design

## Fog

- **G-1** (non-blocking — blocks: design Q-n): the faceless-class inventory — exactly which classes lack machine faces and need header-and-anchor scanning, and how those grammars are pinned against drift (conformance fixtures vs the writing tools' goldens)
- **G-2** (non-blocking — blocks: design Q-n): the exact entry surface — binary name, installed `pyxis exec` invocation, and source-mode invocation
- **G-3** (non-blocking — blocks: design Q-n): statefile schema and the idle-timeout value; single-instance semantics across worktrees of one repo
- **G-4** (non-blocking — blocks: design Q-n): read/caching strategy per request class so live reads (FR-9) and the performance budgets (NFR-1) hold together at 1,000 docs
- **G-5** (non-blocking — blocks: design Q-n): reusing the in-process plan presenter and defining degradation when a registered class cannot be presented (UB-2)

## Out of scope

- **O-1**: any second process, snapshot artifact, shell launcher, or external presentation runtime (from D-1, D-2)
- **O-2**: any bundler, asset build step, or runtime package dependency in the UI (from D-3e)

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
