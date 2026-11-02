# Workbench UI v1 — the Go reading room — technical design

| Field | Value |
|---|---|
| **Status** | Approved (2026-07-10 — operator review) *(merge is the freezing event; "superseded" is computed by the index, never written here)* |
| **Owner** | @mdarabi |
| **Approver(s)** | @mdarabi (operator) — mandatory named reviewer: the design opens a local network-bound surface (Security/privacy impact: yes) |
| **Date** | 2026-07-10 |
| **Relevant-when** | designing or changing the reading room's runtime — its corpus reader, class registry, server lifecycle, search, progress derivation, document presentation, or entry point; implementing against it; triaging defects in browsing, search, citation resolution, or the progress view; superseding the Go runtime ruling |
| **Scope** | The reading room's runtime shape: the corpus-read boundary (machine-faces-first), the class registry, the loopback server and its lifecycle, search, progress derivation, read-only document presentation, the conversation hand-off, and the Go entry point. **Not**: the product perimeter (the merged PRD owns it); the operator conversation or plan renderer themselves; tracker/network enrichment (PRD waiting room) |
| **Tracking** | introducing PR (this branch; rides with the gated design-brainstorm record) |
| **Builds-on** | `docs/tech-design/plan-viewer-v1.md` (the composed plan page + review-scratch output precedent) · `docs/tech-design/repo-layout-and-system-footprint.md` (the managed-footprint home, buckets, release/resolver surface) · `docs/tech-design/solution-shape-v1.md` (the paired-shape model this design is authored under) |
| **Supersedes** | — |
| **Serves** | prd:workbench-ui/FR-1 · prd:workbench-ui/FR-2 · prd:workbench-ui/FR-3 · prd:workbench-ui/FR-4 · prd:workbench-ui/FR-5 · prd:workbench-ui/FR-6 · prd:workbench-ui/FR-7 · prd:workbench-ui/FR-8 · prd:workbench-ui/FR-9 · prd:workbench-ui/UB-1 · prd:workbench-ui/UB-2 · prd:workbench-ui/UB-3 · prd:workbench-ui/UB-4 · prd:workbench-ui/NR-1 · prd:workbench-ui/NR-2 · prd:workbench-ui/NR-3 · prd:workbench-ui/NFR-1 · prd:workbench-ui/NFR-2 · prd:workbench-ui/NFR-3 · prd:workbench-ui/NFR-4 · prd:workbench-ui/ABUSE-1 · prd:workbench-ui/ABUSE-2 · prd:workbench-ui/ABUSE-3 |
| **Security/privacy impact** | yes — a loopback-bound local HTTP surface serving repository content to the local browser; session-secret URL + Host validation, zero mutating endpoints, zero egress · reviewer: @mdarabi (operator) |
| **Template version** | tech-design-template/v1 |

## 1. Problem

The merged workbench-ui PRD charters the reading room — read-only browse, search, citation tracing, and corpus-derived progress over the governed document corpus — and deliberately defers its runtime shape to this design (its Q1–Q5). This document fixes what an implementer would otherwise invent: where class knowledge lives and how it stays honest beside the writing tools, how the server guards a network-bound surface over repo content, how live reads and the performance budget coexist, how plans reuse the Go viewer, and how the binary enters the system release.

## 2. Decision

**Decision:** The reading room is **one Go process in the managed system footprint** (design-brainstorm:workbench-ui-tech-design#D-1, #D-2, #D-3). It reads the corpus **directly, machine-faces-first** — the generated one-writer JSON contracts (ledger projections, the design index's machine face, plan folds) are the primary inputs, and header/anchor scanning exists only for classes with no machine face, confined to a **class registry** that is the sole home of class knowledge. It serves **server-rendered pages over loopback**, guarded by a per-session secret in the URL path and Host validation. Markdown classes use a safe Go read-only presentation that preserves headings, anchors, and escaped source; plans use the existing Go plan-viewer package. It writes nothing inside the repository but gitignored scratch, exits with its terminal or on idle, and hands review to the operator conversation by exposing the exact repository path and tracked reference. It has no build-time asset pipeline or package-install stage.

## 3. The design

### 3.1 The corpus reader — machine faces first

**Decision:** Corpus knowledge enters through exactly two lanes, in strict preference order: (1) a class's **machine face** — the generated, schema-versioned JSON the writing tools already emit (the ledger's `effective.source.json`, the design index's JSON face, the plan fold) — consumed as data, never re-derived; (2) for the classes with no machine face (the governing docs, incidents, wiki, brainstorm records, review reports), a **header-and-anchor scan**: the routing-header table, headings, and the anchor/citation grammars — never full semantic parsing. The reader emits one uniform record per document (the shape's `DocEntry`); everything downstream (catalog, search, citations, progress) consumes records, not files.

The duplicated knowledge is deliberately small — header tables, heading anchors, and citation grammars — and it is **pinned, not trusted**: conformance fixtures assert the scan against golden documents produced by the real templates, so a grammar change on the writing side turns the suite red instead of silently mis-indexing (R1).

### 3.2 Boundaries and non-goals

- **Not solved here** — tracker/PR enrichment, saved views, multi-repo aggregation (PRD waiting room); comment/edit workflows and plan-renderer internals; stage semantics beyond what the corpus states.
- **Owned by a future design** — any write path or UI-triggered mutating verb (PRD NG-2/WR-2); in-place comments (PRD WR-3).
- **Implementation must not infer** — that reading implies authority (the docs and gates stay the only truth); that the hand-off may invoke an editor or mutate the repo; that a missing machine face licenses semantic contract inference (the faceless lane is headers and anchors only).
- **Must remain unchanged** — the write-side tooling's behavior (PRD NR-1), the Go-only runtime and no-package-install posture (NR-2), and the CI gate surface (NR-3).

### 3.3 Invariants

- **DINV-1** — The UI process creates and modifies no file inside the repository; its only writes land in the gitignored render-scratch directory. *(prd:workbench-ui/FR-7)*
- **DINV-2** — Every document class routes through its single registered read-only presentation; plan classes use the Go plan-viewer package, and hostile document content remains inert. *(prd:workbench-ui/FR-2)*
- **DINV-3** — The server binds only the loopback interface, exposes zero mutating endpoints, and refuses any request that lacks the session secret or carries an unexpected Host. *(prd:workbench-ui/NFR-2, ABUSE-3)*
- **DINV-4** — Document-class knowledge exists in exactly one place — the class registry; admitting a new class adds a registry entry and touches no existing class surface. *(prd:workbench-ui/NFR-4)*
- **DINV-5** — Corpus state is read live per request against a cheap tree fingerprint; no derived index or snapshot persists anywhere. *(prd:workbench-ui/FR-9)*

### 3.4 Observability

Local single-user tool: the surface is logs and the statefile, not telemetry.

- **Events/logs** — structured request lines and defect quadruples (path · operation · reason · remedy) to stderr; launch/exit/degradation events named.
- **Metrics** — none — no runtime telemetry for a local tool; the PRD's success is human-judged (its DOD-4).
- **Alerts** — none.
- **Debugging surface** — the statefile (pid, port, start time, secret hash) plus an in-UI status panel naming the corpus root, per-class document counts, active degradations, and the tree fingerprint.
- **Trace/correlation** — none — a single process; the request line carries the request id.

### 3.5 Security & privacy

Yes — this design opens a local network-bound surface over repository content (echoes the header row). Surfaces: a loopback HTTP listener and repo content served to the local browser. Threats and mitigations: a hostile local process reading the served corpus (PRD ABUSE-1) is accepted residual — it already holds filesystem privilege; hostile document content attempting script execution or egress (ABUSE-2) is contained by DINV-2's escaped, registered presentations, no external assets, and zero egress (NFR-3); a hostile web page probing loopback via cross-origin fetch or DNS rebinding (ABUSE-3) is refused by DINV-3's session secret and Host validation. There are no accounts, no user data beyond the repo's own files, no secrets at rest (the session secret dies with the process), and no network egress of any kind. Mitigations trace to A-6.

### 3.6 Server lifecycle

**Decision:** One session per **working tree** (the statefile's home) — sibling worktrees of one repository run independent sessions over their own corpus states: launch mints a crypto-random secret, binds an ephemeral loopback port, writes the statefile into the render-scratch directory, and opens the browser at the secret URL. A second launch finds the statefile, probes liveness (pid + a guarded ping), and refuses with the named quadruple while the first lives (PRD UB-4) — a dead statefile is replaced, never trusted (acting on absence needs the liveness probe, AP-12/AP-17). The process exits on terminal end (SIGINT/SIGTERM), and auto-exits after a generous idle window with a named log line, so an abandoned session costs nothing (AP-19; default rides in the shape; revisit on usage evidence, Q1). Bind failure refuses the same way — no half-served state (UB-4). Exit removes the statefile; scratch pages are regenerable and may remain.

### 3.7 Class-registered document presentation

**Decision:** The class registry chooses one read-only presentation for each document class. Markdown classes are presented by a Go `html/template` path that emits navigable headings and anchors while escaping document content; plans call the existing Go plan-viewer package over the folded model. If a registered presentation is unavailable, the entry stays in the catalog with its metadata and a named remedy replacing the page (PRD UB-2). Degradation is per-document and loud, never a blank page or dead server.

### 3.8 Search

**Decision:** Search runs over the reader's records in memory, rebuilt only when the tree fingerprint moves (DINV-5): a query is first tested against the anchor-ID grammars (requirement rows, principles, invariants, incidents, record anchors, review findings) — hits are exact-match results ranked first (PRD FR-3); the remainder is a plain case-folded token scan over titles, headings, and body text with heading-level context and open-at-match anchors. At the PRD's corpus bound (a thousand documents) a full rescan fits the budgets (NFR-1); nothing smarter ships until evidence says the budget breaks (no ranking heuristics, no dependencies).

### 3.9 Progress derivation

**Decision:** The progress view derives each feature's row from corpus signals only, joined on the feature slug: the brainstorm record and its gate verdict; the PRD ledger's presence and status; designs whose `Serves` cite the feature (from the index's machine face) and their status; plan presence and status; review reports attached to each artifact. The row names the furthest artifact, its pending human gates (from the status ladders), and links every cell to its artifact. Anything the corpus does not state — implementation, merge, release — renders as **beyond the corpus**, honestly unknown (PRD FR-5); contested or unparseable states render as their defects (UB-3), never guessed.

### 3.10 The entry point

**Decision:** The entry is the released `workbench-ui` Go binary, invoked in an installed repo through `pyxis exec workbench-ui -- [--repo <path>] [--no-open]` and directly in source mode. No shell launcher or secondary runtime is added. Arguments parse fail-closed: any flag-like positional or unknown flag is a usage refusal before any action (incident I3's lesson, applied from birth).

### 3.11 Testing strategy

Seams and bins: the **reader and registry** — in-process, table-driven Go fixtures per class plus the golden-pinned faceless-grammar suite (R1); the **server** — local-substitutable, real HTTP through `httptest` (secret-less request, rebound-Host request, mutation attempt, UB-1/UB-4 matrices); the **plan presentation** — in-process over folded fixtures with an unavailable-presenter branch (UB-2); the **browser** — pages are server-rendered and asserted as golden DOM without browser automation. Hostile-content (ABUSE-2), egress-off (NFR-3), scaled-corpus (NFR-1), and class-onboarding (NFR-4: a synthetic class lands with a registry entry and zero diffs elsewhere — the conformance fixture) complete the suite.

The concrete artifact layout, registry and record schemas, route table, statefile, and stage enum live in this design's paired solution shape: `shape:workbench-ui-v1` (`docs/solution-shapes/workbench-ui-v1.md`) — authoritative until the implementing code lands.

## 4. Alternatives considered

| Alternative | Why rejected |
|---|---|
| A generated corpus snapshot consumed by a separate UI process (design-brainstorm:workbench-ui-tech-design#ALT-2) | Adds a second artifact and refresh loop; direct machine-face reads already give request-time freshness. |
| Two live processes — an API server plus a separate frontend runtime (#ALT-3) | Doubles lifecycles, ports, secrets, and failure modes for a single-user local tool. |
| A shell launcher around the server (the session's DQ-2 options) | Nothing about launch is shell-specific; the released Go binary is the entry (§3.10). |
| A served framework or asset bundler | The standard library suffices at this scale; a build pipeline and dependency tree add no required behavior. |
| Full semantic markdown parsing for faceless classes | The faceless lane needs headings, anchors, links, and escaped content; semantic re-parsing creates contract drift without a consumer that needs it. |

## 5. Risks

| ID | Risk | Impact | Mitigation | Verification / evidence |
|---|---|---|---|---|
| R1 | The header/anchor grammars drift from the writing tools' | mis-indexed corpus, broken citations | machine-faces-first (§3.1) shrinks the exposed surface; conformance fixtures golden-pinned to real template output turn drift red | the pinned suite fails on a grammar change fixture |
| R2 | A registered presentation is unavailable | affected document views missing | per-document loud degradation (§3.7) | UB-2 fixture matrix |
| R3 | Per-request live reads break the performance budget at corpus scale | sluggish UI, budget breach | fingerprint-keyed in-memory rebuild (§3.8, DINV-5) — rescan only on change | NFR-1 scaled-corpus fixture (p95/p99 asserted) |
| R4 | The read-only server grows an authoring path | review and mutation semantics split across tools | GET-only routing, no writer imports, write-audit fixtures | route-table and import-boundary tests |
| R5 | The session secret leaks (URL sharing, shoulder-surf) | another local actor reads the corpus over HTTP | accepted residual: loopback-only + ephemeral secret + no writes possible; same privilege class as ABUSE-1 | documented in §3.5; rebinding/secret fixtures prove the guard's floor |

## 6. Acceptance

| ID | Acceptance criterion | Verify | Required evidence | Traces to |
|---|---|---|---|---|
| A-1 | On a fixture corpus, the catalog lists every governed document exactly once by class, and each opens through its registered read-only presentation | Test | fixture suite run | prd:workbench-ui/FR-1 · prd:workbench-ui/FR-2 · #DINV-2 |
| A-2 | ID queries land their defining anchor first; free-text queries return doc + heading context and open at the match; a no-hit query returns an explicit empty state | Test | search fixture suite | prd:workbench-ui/FR-3 |
| A-3 | Every citation grammar in fixture docs resolves to its target anchor; a dangling citation renders named-unresolved | Test | citation fixture suite | prd:workbench-ui/FR-4 |
| A-4 | The progress view derives the expected stage and pending gates per fixture feature, offline; adding an artifact advances the row; beyond-corpus states render honestly | Test | progress fixture suite (egress disabled) | prd:workbench-ui/FR-5 · prd:workbench-ui/FR-9 |
| A-5 | A full session (browse/search/trace/progress) writes nothing in the repo; scratch only in the gitignored directory; exit and idle-exit leave no process; second launch and bind failure refuse with the quadruple | Test | write-audit + lifecycle fixture matrix | prd:workbench-ui/FR-7 · prd:workbench-ui/FR-8 · prd:workbench-ui/UB-1 · prd:workbench-ui/UB-4 · #DINV-1 |
| A-6 | Secret-less, rebound-Host, and mutation requests are refused; a hostile-content fixture executes nothing and egresses nothing; an egress-blocked session completes every flow | Test | security fixture suite | prd:workbench-ui/NFR-2 · prd:workbench-ui/NFR-3 · prd:workbench-ui/ABUSE-2 · prd:workbench-ui/ABUSE-3 · #DINV-3 |
| A-7 | With a registered presentation unavailable, the catalog serves and affected views degrade to named remedies; a corrupted document shows its defects while the corpus stays browsable | Test | degradation fixture matrix | prd:workbench-ui/UB-2 · prd:workbench-ui/UB-3 |
| A-8 | A synthetic document class onboards with one registry entry and zero edits to existing class surfaces | Test + Inspection | class-onboarding conformance fixture + diff audit | prd:workbench-ui/NFR-4 · #DINV-4 |
| A-9 | Launch-to-catalog and request budgets hold on the scaled fixture corpus | Test | benchmark run asserting prd:workbench-ui/NFR-1's fit criterion | prd:workbench-ui/NFR-1 · #DINV-5 |

## 7. Open questions

| ID | Question | Owner | Blocks | Status | Resolution |
|---|---|---|---|---|---|
| Q1 | The idle-exit default (shape carries a starting value) — right window against real usage | @mdarabi | none | open | tune on usage evidence post-launch; never a config key while the config schema stays closed |
| Q2 | Golden-pinning mechanics for the faceless grammars — fixtures generated from the live templates at test time vs committed goldens refreshed by script | implementation | implementation | open | decided at implementation inside the R1 suite |
| Q3 | Whether long markdown documents default to whole-document or section-focused presentation | implementation | implementation | open | decide against the usability fixture without changing the registry contract |

## Principles

- **AP-11 — honored with a bounded registry.** Machine faces are consumed directly; the only duplicated class knowledge is the faceless header/anchor grammar in one Go registry, guarded by golden conformance fixtures (R1).
- **AP-6 — honored.** The reader's primary inputs are the schema-versioned one-writer machine faces; the UI adds no parallel authority and emits no new cross-boundary format.
- **AP-10 — honored.** Every mechanical rule here is code or fixture (registry, guards, budgets), never prompt discipline.
- **AP-17 — honored.** Refusals are loud and fail-closed (UB-1/UB-4, I3-style argument parsing); the only fail-open is additive per-document degradation (UB-2/UB-3), which spawns no risk.
- **AP-19 — honored.** On-demand, ephemeral, idle-exits; an abandoned session costs nothing recurring.
- **AP-20 — honored.** Writes confined to gitignored scratch (DINV-1); the repository is never the UI's mutation scope.
- **AP-22 — honored.** Second launches, races, and re-runs converge: the loser records a refusal quadruple, never a broken state; dead statefiles are replaced on evidence.
- **AP-25 — honored.** The UI lives on the vendor-neutral footprint surface; no agent-CLI or vendor directory carries content.
- **AP-27 — honored.** Extension is by addition: the class registry is the single append surface for new classes (DINV-4), proven by the onboarding fixture (A-8).
- **RW-5 — honored.** One decision — the reading room's runtime shape; the product perimeter, operator conversation, and plan renderer stay their own documents' ground.
- **Deviation:** none.
