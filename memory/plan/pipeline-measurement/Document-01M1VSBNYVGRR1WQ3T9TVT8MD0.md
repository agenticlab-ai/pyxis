# Pipeline measurement — plan 001

| Field | Value |
|---|---|
| **Feature** | pipeline-measurement |
| **PRD** | docs/prds/pipeline-measurement |
| **Design** | docs/tech-design/pipeline-measurement-v2.md |
| **Plan version** | 001 |
| **Dialect** | harbormaster/fs-spec:v1 |
| **Epic title** | Build the pipeline-measurement tool — per-phase spend & efficiency across the four harnesses |
| **Status** | Draft — freezes at merge |
| **Template version** | plan-template/v1 |

## Tasks

### T1 — The per-run record store and schema

| Field | Value |
|---|---|
| **Blocked by** | — |
| **Size band** | 40–90 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | — |
| **Pattern** | — |
| **Touches** | internal/phasemeter/store, internal/phasemeter/record, .gitignore |

**Goal:** A schema-versioned JSON run record can be appended to and read back from an append-only JSONL store, one file per run.

**Current behavior:** No measurement store exists.

**Desired behavior:** A record type carrying a schema version, run id, feature id, phase id, harness, per-model spend slots, efficiency slots, price-snapshot version, and unavailable markers, serialized one-per-line to a per-run file in a gitignored observability-logs location; a loader reads a run's records back; two concurrent runs write distinct files and never contend; unknown/missing optional fields default on read (additive-schema).

**Scope:**
- The record type and its schema version.
- The per-run-file append and the run loader.
- The gitignore entry for the store location.

**Out of scope:**
- Capturing spend or efficiency, or deriving cost (later tasks).

**Acceptance criteria:**
- [ ] Writing N records to a run then loading that run yields exactly N equal records.
- [ ] Two runs writing concurrently produce two distinct files with no lost or interleaved lines.
- [ ] A record missing an optional field loads with that field defaulted, never an error.

**Key interfaces:**
- The run-record struct (schema-versioned); the store's append-one and load-run contracts.

**Traces:** prd:pipeline-measurement/FR-1 · prd:pipeline-measurement/FR-8 · design:pipeline-measurement-v2#3.6 · design:pipeline-measurement-v2#DINV-6 · design:pipeline-measurement-v2#DINV-7

### T2 — Phase-boundary detection and innermost-active-phase attribution

| Field | Value |
|---|---|
| **Blocked by** | T1 |
| **Size band** | 50–110 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | — |
| **Pattern** | — |
| **Touches** | internal/phasemeter/phase |

**Goal:** The recorder brackets a run's phases from the harness's own session record as a stack and attributes each turn to the innermost active phase.

**Current behavior:** No phase concept exists over a harness session record.

**Desired behavior:** A phase stack where a skill-invocation marker pushes and an artifact-write pops; each turn attributes to the top of the stack; a nested sub-skill gets its own bracket and record, never blended into its parent; a phase with no close is recorded incomplete with its spend, never dropped or rolled into the next; each phase carries the identity of the harness that ran it.

**Scope:**
- The phase-stack detector over a harness session record; the push/pop marker predicates.
- Incomplete-phase handling and per-phase harness identity.

**Out of scope:**
- Reading spend figures or writing the spend fields (T3).

**Acceptance criteria:**
- [ ] A nested-invocation fixture yields distinct phase brackets with no spend blended into the parent.
- [ ] An aborted phase (no close marker) is recorded incomplete, never dropped or merged into the next.
- [ ] A harness-switch fixture yields records each naming the harness that ran that phase.

**Key interfaces:**
- The phase-stack type; the boundary-marker predicates (open/close).

**Traces:** prd:pipeline-measurement/FR-3 · prd:pipeline-measurement/UB-3 · prd:pipeline-measurement/UB-4 · prd:pipeline-measurement/UB-5 · design:pipeline-measurement-v2#3.8 · design:pipeline-measurement-v2#DINV-5

### T3 — Spend-source port + Claude Code adapter: measure a phase end to end

| Field | Value |
|---|---|
| **Blocked by** | T1, T2 |
| **Size band** | 60–130 |
| **Mechanical** | no |
| **Size accepted** | the first end-to-end tracer bullet — the port, the primary adapter, the record-write path, and the recorder main are one demoable slice; splitting them yields no independently-demoable behavior |
| **Overlap accepted** | — |
| **Routing** | — |
| **Pattern** | — |
| **Touches** | cmd/phasemeter, internal/phasemeter/spend, internal/phasemeter/adapter/claudecode |

**Goal:** Measure a real Claude Code phase's per-model spend and write its record — the first end-to-end measured phase.

**Current behavior:** No spend capture exists.

**Desired behavior:** A consumer-side spend-source port; a Claude Code adapter that reads Claude Code's own accounting for a phase bracket into a per-model token quartet; the recorder writes the phase's record (spend + harness + ids) to the store. The recorder invokes no model, reads only accounting + the run, mutates nothing of the run, writes no secret value, and its main refuses a flag-like positional before any write.

**Scope:**
- The spend-source port interface (consumer-side).
- The Claude Code adapter; the record-write path; the recorder main (fail-closed on misparse).

**Out of scope:**
- The other three adapters (T6–T8); reconciliation (T4); the degradation path (T5).

**Acceptance criteria:**
- [ ] Running a real Claude Code phase yields a record whose per-model tokens equal Claude Code's own accounting for that bracket.
- [ ] Over a full measurement pass the recorder consumes zero model tokens and leaves the run's artifacts and harness state byte-unchanged.
- [ ] A seeded-secret transcript produces no record containing the secret.
- [ ] A flag-like positional (e.g. a flag placed after a directory arg) is refused as a usage error before any record is written.

**Key interfaces:**
- The spend-source port; the Claude Code adapter (implements it); the recorder entrypoint.

**Traces:** prd:pipeline-measurement/FR-1 · prd:pipeline-measurement/FR-5 · prd:pipeline-measurement/FR-7 · prd:pipeline-measurement/FR-10 · prd:pipeline-measurement/NR-1 · design:pipeline-measurement-v2#3.7 · design:pipeline-measurement-v2#3.11 · design:pipeline-measurement-v2#DINV-1 · design:pipeline-measurement-v2#DINV-2 · design:pipeline-measurement-v2#DINV-8 · design:pipeline-measurement-v2#DINV-9

### T4 — Tiered reconciliation

| Field | Value |
|---|---|
| **Blocked by** | T3 |
| **Size band** | 30–70 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | — |
| **Pattern** | — |
| **Touches** | internal/phasemeter/reconcile |

**Goal:** Each recorded per-model figure is reconciled to the harness's own accounting, with the reconciliation tier recorded per figure.

**Current behavior:** Recorded spend is unverified.

**Desired behavior:** The recorder cross-checks the per-phase per-model sum against the harness's authoritative total for the same span where one is reported, and records the tier — exact where the harness reports an authoritative figure, harness-precision where only a coarser figure exists (never inventing digits), unavailable where non-reconcilable.

**Scope:**
- The reconciler and the per-figure tier recording.

**Out of scope:**
- The unavailable degradation path (T5).

**Acceptance criteria:**
- [ ] On a reference run every recorded figure carries a reconciliation tier.
- [ ] An authoritative-total fixture reconciles exact (recorded equals the harness figure).
- [ ] A coarser-total fixture records harness-precision and never reports more precision than the harness gave.

**Key interfaces:**
- The reconciler contract; the tier enum (exact / harness-precision / unavailable).

**Traces:** prd:pipeline-measurement/FR-1 · design:pipeline-measurement-v2#3.9 · design:pipeline-measurement-v2#DINV-4

### T5 — Honest degradation: unavailable, never a silent zero

| Field | Value |
|---|---|
| **Blocked by** | T3 |
| **Size band** | 25–60 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | — |
| **Pattern** | — |
| **Touches** | internal/phasemeter/spend |

**Goal:** A figure a harness cannot expose is recorded as a declared unavailable marker, never a zero or an estimate.

**Current behavior:** An adapter that cannot read a figure has no defined recorded outcome.

**Desired behavior:** The spend-source port's contract lets an adapter return a declared unavailable marker naming the harness and the missing figure; the recorder writes the marker; no zero or estimated value is ever presented as measured spend.

**Scope:**
- The unavailable marker type and the port's degradation contract; the recorder's handling of it.

**Out of scope:**
- Which specific harnesses degrade (each adapter's empirical concern, T6–T8).

**Acceptance criteria:**
- [ ] A no-usage fixture yields a declared unavailable marker naming the harness and figure.
- [ ] No zero or estimate ever appears where a figure was unavailable (a red-first-then-green degradation test).

**Key interfaces:**
- The unavailable marker type; the spend-source port's return contract.

**Traces:** prd:pipeline-measurement/FR-6 · prd:pipeline-measurement/UB-1 · design:pipeline-measurement-v2#DINV-3 · design:pipeline-measurement-v2#3.7

### T6 — Codex spend adapter

| Field | Value |
|---|---|
| **Blocked by** | T3 |
| **Size band** | 40–90 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | — |
| **Pattern** | — |
| **Touches** | internal/phasemeter/adapter/codex |

**Goal:** Capture-or-declare per-model spend on Codex.

**Current behavior:** No Codex adapter.

**Desired behavior:** A Codex adapter reading Codex's own accounting through whichever channel it exposes on the user's own key (the empirical capability-matrix spike), producing per-model spend for a phase bracket, or a declared unavailable where Codex exposes none. It alters no Codex state.

**Scope:**
- The Codex adapter and the channel investigation for Codex.

**Out of scope:**
- The port itself (T3); the other adapters.

**Acceptance criteria:**
- [ ] A Codex reference-run fixture yields per-model spend or a declared unavailable, attributed to the correct phase.
- [ ] Reading Codex's accounting leaves Codex's own output byte-unchanged.

**Key interfaces:**
- The Codex adapter (implements the spend-source port).

**Traces:** prd:pipeline-measurement/FR-5 · design:pipeline-measurement-v2#3.7

### T7 — Cursor CLI spend adapter

| Field | Value |
|---|---|
| **Blocked by** | T3 |
| **Size band** | 40–90 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | — |
| **Pattern** | — |
| **Touches** | internal/phasemeter/adapter/cursor |

**Goal:** Capture-or-declare per-model spend on Cursor CLI.

**Current behavior:** No Cursor CLI adapter.

**Desired behavior:** A Cursor CLI adapter reading its accounting through whichever channel it exposes on the user's own key; the design flags Cursor's proxy-billing as the likeliest degradation case, so a declared unavailable is an expected, first-class outcome here, not a failure. It alters no Cursor state.

**Scope:**
- The Cursor CLI adapter and the channel investigation for Cursor.

**Out of scope:**
- The port itself (T3); the other adapters.

**Acceptance criteria:**
- [ ] A Cursor CLI reference-run fixture yields per-model spend or a declared unavailable, attributed to the correct phase.
- [ ] Where Cursor exposes no per-model spend on the user's key, the adapter declares unavailable, never a fabricated figure.

**Key interfaces:**
- The Cursor CLI adapter (implements the spend-source port).

**Traces:** prd:pipeline-measurement/FR-5 · design:pipeline-measurement-v2#3.7

### T8 — OpenCode spend adapter

| Field | Value |
|---|---|
| **Blocked by** | T3 |
| **Size band** | 40–90 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | — |
| **Pattern** | — |
| **Touches** | internal/phasemeter/adapter/opencode |

**Goal:** Capture-or-declare per-model spend on OpenCode.

**Current behavior:** No OpenCode adapter.

**Desired behavior:** An OpenCode adapter reading its accounting through whichever channel it exposes on the user's own key, producing per-model spend for a phase bracket, or a declared unavailable. It alters no OpenCode state.

**Scope:**
- The OpenCode adapter and the channel investigation for OpenCode.

**Out of scope:**
- The port itself (T3); the other adapters.

**Acceptance criteria:**
- [ ] An OpenCode reference-run fixture yields per-model spend or a declared unavailable, attributed to the correct phase.
- [ ] Reading OpenCode's accounting leaves OpenCode's own output byte-unchanged.

**Key interfaces:**
- The OpenCode adapter (implements the spend-source port).

**Traces:** prd:pipeline-measurement/FR-5 · design:pipeline-measurement-v2#3.7

### T9 — Deterministic artifact+git efficiency reader

| Field | Value |
|---|---|
| **Blocked by** | T1 |
| **Size band** | 60–120 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | — |
| **Pattern** | — |
| **Touches** | internal/phasemeter/efficiency |

**Goal:** Per-phase efficiency signals are computed deterministically from the frozen artifacts + git, joined to the run records.

**Current behavior:** No efficiency signals are computed.

**Desired behavior:** A reader counts the corpus's existing typed anchors per phase — questions from `D-n`, fog from `G-n`, rework from superseded `D-n` and amendment count, defects from review `FND-n`, coverage from the plan matrix, latency from git timestamps — joined to records by run/phase id, with no model invocation and no runtime hook, yielding identical signals whichever harness produced the artifacts.

**Scope:**
- The anchor-counting extractor, the git-timestamp latency, and the join to records.

**Out of scope:**
- Any spend path; the operator presentation (T12).

**Acceptance criteria:**
- [ ] For a reference run the efficiency signals are byte-identical when computed from two different harnesses' artifacts.
- [ ] Computing the signals consumes zero model tokens.

**Key interfaces:**
- The efficiency-signal struct; the per-anchor-family counters; the git latency reader.

**Traces:** prd:pipeline-measurement/FR-4 · design:pipeline-measurement-v2#3.10 · design:pipeline-measurement-v2#DINV-1

### T10 — Vendor the pinned per-model price data

| Field | Value |
|---|---|
| **Blocked by** | — |
| **Size band** | 0 |
| **Mechanical** | yes |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | — |
| **Pattern** | — |
| **Touches** | the vendored price-data asset; the footprint version-pin file |

**Goal:** The pinned upstream per-model price map is vendored as committed data, with a re-vendor script and a recorded version pin.

**Provenance:**
- Vendor the pinned upstream per-model price map (the LiteLLM `model_prices_and_context_window.json`) as committed data in the footprint's vendor area.
- Add a re-vendor script mirroring the mermaid/lavish vendoring idiom.
- Record the pinned version in the footprint's version-pin file.

**Acceptance criteria:**
- [ ] The vendored price map is present as committed data at the recorded pin, and the re-vendor script reproduces it.

**Traces:** design:pipeline-measurement-v2#3.12

### T11 — Cost derivation from the vendored prices

| Field | Value |
|---|---|
| **Blocked by** | T1, T10 |
| **Size band** | 30–70 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | — |
| **Pattern** | — |
| **Touches** | internal/phasemeter/price |

**Goal:** Each record's per-model dollar cost is derived from the pinned price snapshot, with the snapshot version stamped and un-priced models flagged.

**Current behavior:** Records carry tokens but no cost.

**Desired behavior:** Parse the vendored price data; compute per-model cost as tokens times the pinned per-class price; stamp the price-snapshot version on the record; flag an un-priced model rather than emitting a silent zero cost.

**Scope:**
- The price parser and the cost derivation; the snapshot-version stamp; the un-priced flag.

**Out of scope:**
- Maintaining the prices themselves (vendored, T10); the presentation (T12).

**Acceptance criteria:**
- [ ] A record reproduces its dollar cost from the price-snapshot version it stamped.
- [ ] A model with no price entry is flagged, never given a $0 cost.

**Key interfaces:**
- The price-table parser; the cost-derivation contract; the snapshot-version field.

**Traces:** prd:pipeline-measurement/FR-2 · prd:pipeline-measurement/UB-2 · design:pipeline-measurement-v2#3.12 · design:pipeline-measurement-v2#DINV-6

### T12 — The operator reader

| Field | Value |
|---|---|
| **Blocked by** | T4, T9, T11 |
| **Size band** | 40–90 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | — |
| **Pattern** | — |
| **Touches** | cmd/phasemeter, internal/phasemeter/reader |

**Goal:** The operator can read, per phase of a run, tokens by model, dollar cost, and the efficiency signals.

**Current behavior:** Records exist in the store but there is no operator-facing read.

**Desired behavior:** A reader that queries a run's records, joins the reconciled spend, cost, and efficiency, and presents each phase's tokens-by-model, cost, and efficiency signals (or the unavailable markers); its main fails closed on misparse.

**Scope:**
- The reader main and the per-phase presentation.

**Out of scope:**
- A query/analytics surface or a cost-per-feature rollup view (waiting room).

**Acceptance criteria:**
- [ ] For a reference run, the read lists every phase with its per-model tokens, cost, and efficiency signals, or the declared unavailable markers.
- [ ] A flag-like positional is refused as a usage error before any read side effect.

**Key interfaces:**
- The reader entrypoint; the per-phase view contract.

**Traces:** prd:pipeline-measurement/FR-9 · design:pipeline-measurement-v2#3.11

## Coverage

| Requirement | Tasks |
|---|---|
| prd:pipeline-measurement/FR-1 | T1, T3, T4 |
| prd:pipeline-measurement/FR-2 | T10, T11 |
| prd:pipeline-measurement/FR-3 | T2 |
| prd:pipeline-measurement/FR-4 | T9 |
| prd:pipeline-measurement/FR-5 | T3, T6, T7, T8 |
| prd:pipeline-measurement/FR-6 | T5 |
| prd:pipeline-measurement/FR-7 | T3 |
| prd:pipeline-measurement/FR-8 | T1 |
| prd:pipeline-measurement/FR-9 | T12 |
| prd:pipeline-measurement/FR-10 | T3, T9 |
| prd:pipeline-measurement/NR-1 | T3 |
| prd:pipeline-measurement/UB-1 | T5 |
| prd:pipeline-measurement/UB-2 | T11 |
| prd:pipeline-measurement/UB-3 | T2 |
| prd:pipeline-measurement/UB-4 | T2 |
| prd:pipeline-measurement/UB-5 | T2 |
