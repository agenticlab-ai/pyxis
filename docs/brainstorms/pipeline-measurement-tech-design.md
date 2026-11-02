# Pipeline measurement — technical design brainstorm

| Field | Value |
|---|---|
| **In one line** | The technical shape of pipeline-measurement — where the per-phase run record lives and its schema, how per-model spend is captured across the four harnesses, how the phase boundary is detected (incl. nested invocations), reconciled, and how the efficiency signals are extracted. |
| **Relevant-when** | designing or changing the pipeline-measurement mechanism — the per-phase run-record store/schema, the per-harness spend-capture adapters, phase-boundary detection, spend reconciliation, or artifact/git efficiency extraction; checking prior design rulings or refused shapes about the factory's own spend measurement. |
| **Basis** | prd:pipeline-measurement |
| **Authority** | @mdarabi |
| **Date(s)** | 2026-07-21 |
| **Status** | gated: ready-for-design |
| **Tracking** | — |
| **Feeds** | docs/tech-design/pipeline-measurement.md |
| **Prior records** | — |
| **Template version** | design-brainstorm-record/v1 |

## Orientation

| Source | Why loaded |
|---|---|
| docs/prds/pipeline-measurement/effective.md | The basis: FR-1..10, M1/M3, NG/WR, and Q1–Q4 (the design-owned open questions this session resolves). |
| docs/brainstorms/pipeline-measurement.md | The stage-1 record: D-1..D-6 and the parked P-1..P-7 design constraints this design must honor. |
| docs/tech-design/index.md | Overlap check — no live design's `Serves` touches this basis (greenfield). |
| docs/wiki/glossary.md | The `seam` term and the "one adapter = hypothetical seam; two = real" rule (bears on the spend-capture port); confirmed no pipeline-measurement terms yet. |
| docs/tech-design/design-brainstorm-record-model.md · .pyxis/templates/design-brainstorm-record-template.md | The record contract this session writes. |

**Overlaps surfaced:**

- **No live design decides any of this** — the design index shows nothing Serving `prd:pipeline-measurement` (greenfield). Not `covered`.
- **The Ledger (system-design §2 / D7) is unmerged, undesigned WIP** — `docs/system-design.md` is not committed on main; "a first slice of the Ledger" as the record home would build on an artifact that does not yet exist as a frozen contract (bears on DQ-1).
- **Adjacent but disjoint prior designs** — argus's attribution family (`argus-attribution-*`) attributes *authorship*, not token spend; impl-review is a reviewer. Neither decides the measurement shape; both confirm the corpus's append-only-ledger and adapter idioms.

## Design questions

- **DQ-1**: Where does the per-phase run record live, and what is its store/schema shape? (Q3 / P-3)
- **DQ-2**: What is the per-harness spend-capture mechanism, and the shape of the capture adapter? (Q1 / P-1 / P-4)
- **DQ-3**: How is the phase boundary detected, and how is a nested skill invocation attributed? (P-2 / Q4)
- **DQ-4**: How is a recorded figure reconciled to the harness's own accounting, and what is the tolerance? (Q2 / P-6)
- **DQ-5**: How are the structural efficiency signals extracted from the frozen artifacts + git? (P-5)
- **DQ-6**: How is the recorder/reader packaged and invoked? (a docgate-family verb, a standalone binary, or an engine component)
- **DQ-7**: Where do the per-model prices live, and how are they versioned? (P-7)

## Panel log

- **DQ-1**: opens/replaces (the record-store seam) — **fired** — a new store is a new module boundary; three orthogonal homes scored (standalone / Ledger-slice / delivery-ledger).
- **DQ-2**: opens (the spend-source seam) — **fired** — a new port + adapters is a new boundary; three orthogonal mechanisms scored (per-harness adapter / universal transcript / proxy).
- **DQ-3**: extends (the S-2 spend-source seam — the detector reads the same harness record) — **skipped** — no new boundary; the nested rule is a D-n (D-3), not a shape choice.
- **DQ-4**: extends (reconciliation is a check over the S-2 adapter output) — **skipped** — a verification rule, not a new seam.
- **DQ-5**: extends (the corpus-reader idiom docgate already uses) — **skipped** — a deterministic reader over existing typed anchors + git, not a new seam.
- **DQ-6**: extends (the `cmd/` + `internal/` hexagon) — **skipped** — a new standalone tool is standard structure, not a novel seam.
- **DQ-7**: extends (the closed-schema config-data pattern) — **skipped** — prices are vendored external data, not a new seam.

## Decisions

### D-1 — the per-phase run-record store (standalone JSONL under `.pyxis/observability/logs`)

- **Question**: Where does the per-phase run record live, and what store shape?
- **Answer**: A new standalone append-only store owned by this feature — schema-versioned JSONL (one record per line) under `.pyxis/observability/logs/`, gitignored runtime data (per-run operational output, never committed). Independent of the unmerged Ledger and the argus delivery-ledger (ALT-1).
- **Reasoning**: buildable now with no dependency on anything unmerged (ALT-2 rejected — incident I1: don't build on an unmerged design); works for every run regardless of orchestrator (ALT-3 rejected — argus-coupling strains FR-7 + the operator-primary segment); "append-only files first" is D7's own endorsed shape, owned here so the Ledger absorbs it verbatim when it lands. Operator fixed the location `.pyxis/observability/logs`.
- **Anchor**: 2026-07-21 · t1
- **Status**: live

### D-2 — the spend-capture shape (per-harness adapter behind a spend-source port)

- **Question**: How is per-model spend captured on each harness, and what adapter shape?
- **Answer**: One "spend-source" port with a per-harness adapter for each of the four; each adapter reads its harness's native accounting (headless envelope / transcript / OTEL — the best channel it exposes), normalizes to the run-record schema, or declares `unavailable` (FR-6). The per-harness channel is each adapter's private concern (ALT-4). Operator delegated to the recommendation.
- **Reasoning**: the ports/adapters idiom (AP-1/3) + medium-independence (AP-25); the port is real (4 adapters, F-3); isolates Q1's empirical matrix per adapter. ALT-5 rejected (false uniformity); ALT-6 rejected (proxy is a running service + Cursor base-URL gap, over-engineered for zero-overhead v1).
- **Anchor**: 2026-07-21 · t2
- **Status**: live

### D-3 — phase-boundary detection + nested attribution (innermost-active-phase, stack-based)

- **Question**: How is the phase boundary detected, and how is a nested skill invocation attributed?
- **Answer**: A phase **stack** — a skill-invocation marker (Skill tool-call / slash-command) in the harness's own record pushes a phase; the artifact-write (frozen doc / PR) pops it. Spend and turns attribute to the top of the stack (**innermost-active-phase**): a nested sub-skill gets its own record; the parent's record excludes the nested bracket (UB-5). The detector extends S-2 (reads the same harness record the spend adapter does).
- **Reasoning**: innermost-active-phase is the only rule where each phase's figure is what it actually spent; parent-absorbs over-charges and hides the sub-skill's cost; split is ambiguous. The detector rides the existing spend-source seam, not a new one. Operator confirmed.
- **Anchor**: 2026-07-21 · t3
- **Status**: live

### D-4 — reconciliation mechanism + tiered tolerance

- **Question**: How is a recorded figure reconciled to the harness's accounting, and what tolerance?
- **Answer**: The per-phase per-model total is the **sum of the harness's own per-turn usage within the phase bracket**, cross-checked against the harness's authoritative total for the same span where reported. Tolerance is **tiered**: exact where the harness reports an authoritative per-model figure (recorded == harness, count-exact); harness-precision where only a coarser/rounded figure exists (never invent digits); `unavailable` (FR-6) where non-reconcilable, never a guess.
- **Reasoning**: makes M1's "reconciles" concrete without a fabricated numeric tolerance the CLIs do not support; the G-1 matrix decides which harness lands in which tier. Operator confirmed.
- **Anchor**: 2026-07-21 · t4
- **Status**: live

### D-5 — structural-signal extraction (deterministic artifact+git reader)

- **Question**: How are the structural efficiency signals extracted?
- **Answer**: A **deterministic** git + frozen-artifact reader — no model, no runtime hook (FR-4/FR-10) — that derives the per-phase signals by counting the typed anchors the corpus already carries: questions = `D-n`, fog = `G-n`, rework = superseded `D-n` / amendment count, defects = review `FND-n`, coverage from the plan matrix, latency from git timestamps — joined to the run record by phase/run id.
- **Reasoning**: free, deterministic, and identical on every harness (the artifacts are the same whoever produced them) — the universal floor that holds even where spend capture degrades. Rejected: runtime skill instrumentation (unreliable on external harnesses) and LLM quality-judgment (costs model tokens, non-deterministic). Operator confirmed the deterministic approach.
- **Anchor**: 2026-07-21 · t5
- **Status**: live

### D-6 — packaging (standalone tool: recorder + reader)

- **Question**: How is the recorder/reader packaged?
- **Answer**: A new standalone tool (`cmd/<name>` + `internal/<name>`), not a docgate verb — two faces: a **recorder** (post-hoc parse of the harness's own session record → bracket the phase stack per D-3, read spend via the S-2 adapter, write the run record to the S-1 store) and a **reader** (query the store, compute the D-5 efficiency signals, present per phase, FR-9). Post-hoc parsing keeps it execution-agnostic (FR-7) and zero-overhead (FR-10).
- **Reasoning**: measurement is a distinct concern from docgate's doc governance; a separate tool keeps packages small and focused (Go rules); the future engine absorbs it later. The recorder's trigger (git hook / orchestrator / on-demand) is a minor design detail. Operator confirmed.
- **Anchor**: 2026-07-21 · t6
- **Status**: live

### D-7 — per-model prices from an existing maintained source (not self-maintained)

- **Question**: Where do the per-model prices live?
- **Answer**: Do **not** hand-maintain a price table. Consume an existing, actively-maintained cross-provider pricing source (LiteLLM's `model_prices_and_context_window.json` is the de-facto, MIT, covers Anthropic/OpenAI/…), **vendored + pinned** (not runtime-fetched — preserves the no-new-egress / offline posture and controls supply-chain, matching the Mermaid vendoring idiom), refreshed by a pinned re-vendor script. Each run record **stamps the pricing-snapshot version** it used (reproducibility); an un-priced model is flagged (UB-2), never silently $0.
- **Reasoning**: prices drift frequently and a self-maintained table would rot — a maintained upstream is a justified consumption (vendored *data*, parsed with stdlib — no code dependency). Cost stays a thin derived layer over the ground-truth token counts. Operator directed: use an existing source, do not reinvent.
- **Anchor**: 2026-07-21 · t7
- **Status**: live

## Alternatives

### ALT-1 — standalone append-only JSONL store (chosen)

- **Shape**: schema-versioned JSONL run records under `.pyxis/observability/logs/` (gitignored runtime data), written by this feature's recorder, read by its reader; independent of Ledger and delivery-ledger.
- **Trade-offs**: wins — buildable now, no unmerged dependency, works for every run; costs — a store the future Ledger must later absorb (mitigated: records are schema-versioned, absorbable verbatim).
- **Principles**: AP-6 (schema-versioned contractual data) · AP-23 (append-only, parallel-safe) · AP-5 (first-class observability).
- **Incidents**: none matched (I1–I3 checked 2026-07-21).
- **Verdict**: chosen — depends on nothing unmerged; orchestrator-agnostic.
- **From**: DQ-1 · panel

### ALT-2 — a first slice of the system-design Ledger (D7)

- **Shape**: the record is an event/spend row in the Ledger's append-only store (system-design §2 / D7).
- **Trade-offs**: wins — aligns with the eventual first-class Ledger; costs — the Ledger is unmerged and its schema (D7) is explicitly open.
- **Principles**: AP-5 (Ledger first-class) — honored in spirit, but premature.
- **Incidents**: **I1 matched** — building on an unmerged design / WR-stub is exactly I1's paid-for lesson.
- **Verdict**: rejected — premature; `system-design.md`/D7 is not a frozen contract.
- **From**: DQ-1 · panel

### ALT-3 — the argus delivery-ledger cost line (context-pack WR-3)

- **Shape**: per-phase spend appended to argus's delivery ledger.
- **Trade-offs**: wins — corpus-named home (WR-3); costs — per-problem and argus-scoped, so it fails standalone skill runs.
- **Principles**: strains medium-independence (measurement would require argus) and FR-7 (execution-agnostic).
- **Incidents**: none matched (I1–I3 checked 2026-07-21).
- **Verdict**: rejected — argus-coupling; measurement is broader than argus-driven runs.
- **From**: DQ-1 · panel

### ALT-4 — per-harness adapter behind a spend-source port (chosen)

- **Shape**: one spend-source port; four adapters, each reading its harness's native accounting (envelope/transcript/OTEL) → run-record schema, or `unavailable` (FR-6).
- **Trade-offs**: wins — best channel per harness, honest degradation by construction, ports/adapters testable; costs — four adapters to build, and the per-harness channel is an empirical unknown (G-1).
- **Principles**: AP-1/AP-3 (consumer-side port, adapters implement) · AP-25 (medium-independence).
- **Incidents**: none matched (I1–I3 checked 2026-07-21).
- **Verdict**: chosen — the idiom the engine mandates; isolates Q1's matrix per adapter.
- **From**: DQ-2 · panel

### ALT-5 — one universal transcript parser

- **Shape**: a single mechanism reads every harness's session transcript uniformly.
- **Trade-offs**: wins — one code path; costs — transcripts aren't uniform across CLIs, some carry no per-model usage, and it can't use a cleaner envelope where one exists.
- **Principles**: strains AP-25 (assumes a uniform substrate that does not exist).
- **Incidents**: none matched (I1–I3 checked 2026-07-21).
- **Verdict**: rejected — false uniformity.
- **From**: DQ-2 · panel

### ALT-6 — an LLM proxy/gateway metering centrally

- **Shape**: all harnesses route through a pyxis-controlled proxy that meters every request.
- **Trade-offs**: wins — harness-independent metering; costs — needs each CLI's base-URL redirected (Cursor likely can't), and it is a running service — not zero-overhead-simple.
- **Principles**: strains the FR-10 zero-overhead posture; the Cursor base-URL gap breaks FR-5.
- **Incidents**: none matched (I1–I3 checked 2026-07-21).
- **Verdict**: rejected — heavyweight + Cursor gap; over-engineered for v1.
- **From**: DQ-2 · panel

## Seam map

- **S-1**: the per-phase run-record store — **opens** — behind it: schema-versioned JSONL run records at `.pyxis/observability/logs/` — feeds: the design's Scope + `Serves prd:pipeline-measurement/FR-1, FR-8`.
- **S-2**: the spend-source port — **opens** — behind it: four per-harness spend adapters (envelope/transcript/OTEL → run-record, or `unavailable`) — feeds: the design's Scope + `Serves prd:pipeline-measurement/FR-5, FR-6`.

## Facts

- **F-1**: No live technical design's `Serves` touches `prd:pipeline-measurement` — greenfield — source: docs/tech-design/index.md (this session)
- **F-2**: `docs/system-design.md` (the Ledger; open decision D7 "append-only files first, queryable later") is **not committed on main** — it remains untracked WIP — source: worktree at origin/main (file absent)
- **F-3**: Design-discipline rule "one adapter = hypothetical seam; two = real" — a port is justified only by ≥ 2 real adapters — source: docs/wiki/glossary.md (design class)
- **F-4**: The argus **delivery ledger** is the corpus-named intended home for a per-problem cost line (context-pack WR-3), but it is per-problem and argus-scoped; argus's `attribution` family attributes authorship, not token spend — source: prd:pipeline-measurement Q3; docs/wiki/glossary.md
- **F-5**: The PRD binds v1 to all four harnesses (FR-5) with honest degradation (FR-6), zero model-token overhead (FR-10), and execution-agnostic capture (FR-7) — source: docs/prds/pipeline-measurement/effective.md

## Fog

- **G-1** (non-blocking — blocks: the authored design's implementation, as its own `Q-n`): the per-harness capability matrix — which channel (headless envelope / transcript / OTEL / proxy) each of the four harnesses exposes per-model spend on the user's own key, and where FR-6 degradation applies — is an empirical spike, unresolved at design time (prd:pipeline-measurement Q1).

## Out of scope

- **O-1**: The design does not build or depend on the system-design Ledger (D7) — it writes a standalone store the Ledger later absorbs (from D-1)
- **O-2**: Measurement does not couple to argus / the delivery ledger — it is orchestrator-agnostic (from D-1)
- **O-3**: No LLM proxy/gateway capture path — per-harness adapters only (from D-2)
- **O-4**: The tool does not self-maintain per-model prices — they are consumed from a vendored, maintained external source (from D-7)

## Candidate glossary terms

none

## Parked product gaps

none

## Gate

| Field | Value |
|---|---|
| **Verdict** | ready-for-design |
| **Date** | 2026-07-21 |
| **Authority** | @mdarabi |
| **Covering design** | — |
| **Coverage check** | pass — core four covered (§2 D-1..D-7; §4 ALT-1..6; Serves/Scope S-1..S-2 + basis; §3.2 O-1..O-4); zero blocking fog |
| **Amend route** | — |
