# Pipeline measurement — brainstorm record

| Field | Value |
|---|---|
| **In one line** | Measure what each pyxis pipeline phase costs and how efficient it is — per-phase, per-model token/cost spend plus artifact/git-derived efficiency signals — as the factory's own first self-measurement capability. |
| **Relevant-when** | brainstorming or contracting the factory's own per-phase measurement — token/cost attribution to a pipeline stage, the phase-bounded spend record, or artifact/git-derived efficiency signals (questions, fog, rework, defects, latency); deciding where the factory Ledger's spend/event/trace records live or how a phase is bounded for attribution; checking prior rulings on token-telemetry, cost lines, or cross-harness spend capture. |
| **Idea** | From this session: "is there a way to … know how much token did we burn on which model to perform a specific phase of the pipeline (e.g. feature-brainstorming)?" — and, more broadly, "what information do we gather per phases in pyxis … what we measure when running any skills." Measure per pipeline phase: token/model/cost spend, and structural efficiency signals (questions asked, fog, rework/churn, defects, latency). Must ultimately work across the supported agentic CLIs (Claude Code, Codex, Cursor CLI, OpenCode). |
| **Operator** | @mdarabi |
| **Date(s)** | 2026-07-20 |
| **Status** | gated: ready-for-prd |
| **Tracking** | — |
| **Feeds** | docs/prds/pipeline-measurement/ |
| **Prior records** | — |
| **Template version** | brainstorm-record/v1 |

## Orientation

| Source | Why loaded |
|---|---|
| docs/prds/context-pack/effective.md | Nearest neighbor: reduces per-session context cost and **explicitly defers** the token-telemetry plane (NG-5), the delivery-ledger cost line (WR-3), and inline-vs-subagent (NG-3) — the boundary this idea sits against. |
| docs/prds/observability-and-spend.md · docs/prds/README.md | The OBS-1..6 baseline (harbormaster parity, "Implemented in Pyxis: none yet"); OBS-2 (token-spend records) is the closest requirement. |
| docs/domain-model.md §3.7 | The planned observability entities — TokenUsageRecord, Trace, DomainEvent, PassReport, HealthAlarm, GlobalTelemetry — the data model this idea would instantiate. |
| docs/system-design.md §2 · F5/F6 · D7 | Names the **Ledger** (events + trace + spend) as the factory's observability store and leaves its schema an open decision (D7). |
| docs/product-vision.md §2, §10 | Medium-independence (CLI is an adapter); §10 the metrics this plane feeds (cost per shipped feature, interventions). |
| docs/brainstorms/observable-by-construction.md · docs/prds/observable-by-construction/ | The **application** evidence plane (E0–E4) — disjoint from the factory's own plane; confirms this idea is the other side of the boundary. |
| .pyxis/pipeline.md | The stage map: which phases run as skills and freeze a durable artifact (the phase-boundary candidate). |
| .pyxis/templates/brainstorm-record-template.md · docs/tech-design/brainstorm-record-model.md | The record contract this session writes. |
| docs/wiki/glossary.md | Checked existing spend/ledger/trace vocabulary; no term yet names per-phase pipeline spend attribution. |

**Overlaps surfaced:**

- **context-pack (Draft) explicitly defers this idea** — NG-5 refuses a token-telemetry/observability plane; WR-3 waits for "a cost line in the delivery ledger fed by M1 — when a token-accounting convention lands"; A1 assumes session accounting is faithful enough without a telemetry plane. This idea is the deferred convention it names. context-pack also rules inline-vs-subagent (NG-3) and model routing (NG-1) as harness/orchestration concerns *upstream* of measurement.
- **OBS-1..6 are the factory's operational plane, unbuilt in pyxis** — OBS-2 (per-turn token quartet, pass/worker attribution) is the parity requirement; no pyxis feature realizes it yet.
- **domain-model §3.7 plans the entities but nothing is built** — TokenUsageRecord/Trace/DomainEvent/PassReport carry verdicts Generalize/Port/New.
- **system-design names the Ledger and leaves D7 open** — the event/trace/spend schema is an explicitly undecided design ("append-only files first; queryable later").
- **observable-by-construction is disjoint** — it is the *built application's* runtime evidence contract, not the factory measuring itself; its own record notes that treating OBS rows as an application standard would broaden their segment.

No source covers the idea. The open ground is the factory's own per-phase measurement — attributing token/model/cost spend (and efficiency signals) to a pipeline stage — named across OBS-2, context-pack's WR-3, domain-model §3.7, and system-design's Ledger/D7, but built nowhere.

## Framings

- **A — the token-accounting convention (spend-first).** The feature *is* the missing convention that attributes intelligence spend to a pipeline phase: a phase-bounded, per-model spend record (token quartet + cost), the pyxis realization of OBS-2 / domain-model's TokenUsageRecord, and exactly the "token-accounting convention" context-pack's WR-3 waits on. Leads with spend because that is the dominant, most-asked line item (cost per feature; where do the tokens go, on which model). Opens: the phase-boundary rule (Skill-invocation → artifact-write), the first slice of the Ledger / a delivery-ledger cost line, and a per-harness spend-capture capability. Cost: token/cost is the harness-coupled part — cross-harness uniformity is limited by the weakest CLI, so v1 must be honest about where it degrades.
- **B — the artifact+git efficiency reader (structural-first, universal).** The feature is a *reader* over the frozen artifacts + git history that computes per-phase efficiency signals — questions (`D-n`), fog (`G-n`), rework/churn (supersessions, amendments), defects (review `FND-n`), coverage, latency (git timestamps) — with zero runtime instrumentation and identical behavior on all four CLIs today. Token/cost is a later, adapter-fed enrichment, not v1. Thesis: *measure what is universal, free, and buildable now first; the harness-coupled spend plane is a separable second capability.* Opens: a `docgate metrics`-style verb, immediate cross-harness coverage, no engine or CLI dependency. Cost: defers the very number (per-model tokens) the idea started from.
- **C — the pipeline-run Ledger (unified event/trace/spend, phase-scoped).** The feature is the first brick of the factory Ledger (system-design F5, D7): a per-phase run record that unifies *both* planes under one schema — a phase-entered/phase-completed event pair carrying the structural signals **and** the spend, threaded by a run/trace id, appended to the Ledger — the substrate §10's metrics and the F6 optimizer read from. Thesis: *define the run/event/trace/spend record once (D7) and have every phase emit into it, rather than build two disconnected readers.* Opens: the whole Ledger, INV-42 trace context, the self-improvement loop. Cost: broadest scope; needs the D7 schema decision; edges toward "build the entire OBS plane" as one feature.

**Destination:** the **A-spine hybrid** — a per-phase, per-model spend record (A) as the headline, shaped as one *per-phase run record* defined once (C's discipline) so the artifact+git structural signals (B) ride the same record as an additive, universal floor. Operator adopted the recommendation (t1): lead with the spend number the corpus reserves (context-pack WR-3) rather than defer it (B's weakness), keep the record unified so structural signals are not a second system, and let the free structural floor cover the harnesses where spend capture degrades; fall back to pure-B only if cross-harness spend capture proves too ragged for v1.

## Decisions

### D-1 — the destination framing (A-spine hybrid)

- **Question**: Which framing is the destination — A (spend-first), B (structural-first, universal), C (unified run Ledger), or a hybrid?
- **Answer**: The A-spine hybrid. The feature is the per-phase, per-model spend record (A) — the pyxis realization of OBS-2 and the token-accounting convention context-pack's WR-3 reserves — shaped as one per-phase run record defined once (C's discipline), with the artifact+git structural signals (B) riding that same record as an additive universal floor. The problem it fixes: the factory cannot see what each pipeline phase costs (tokens/model/cost) or how efficient it is, so "cost per shipped feature" and "where the tokens went, on which model/phase" are unanswerable and the routing/optimizer loops have no substrate.
- **Reasoning**: A is the number the idea started from and the gap the corpus explicitly reserves, so deferring it (B's weakness) is wrong; the single run-record keeps B's signals additive rather than a second system; the free structural floor still answers where cross-harness spend capture degrades. Operator delegated to this recommendation.
- **Anchor**: 2026-07-20 · t1
- **Status**: live

### D-2 — segments (operator primary; machine consumers secondary; app-runtime + routing anti)

- **Question**: Who is the primary segment this measurement serves, and the anti-segment?
- **Answer**: Primary = the **operator running pipeline drives under a token budget** — wants cost per shipped feature, where the tokens went (by model and phase), and which phase is inefficient/churny, at human-actionable precision. Secondary (reads the same records later, not a v1 target) = the factory's machine consumers: routing intelligence (VNEXT-7) and the F6 optimizer. Anti-segment = the built application's runtime (observable-by-construction's E0–E4 plane, disjoint) and the model-routing *decision* itself (context-pack NG-1 — this produces the substrate, it does not route).
- **Reasoning**: operator-primary keeps v1 a human-actionable cost/efficiency read and defers machine-consumer precision (streaming, low-latency, an optimizer-shaped schema) that would balloon scope toward the full Ledger; the record must be *readable* by the secondary consumers later, not *built for* them now.
- **Anchor**: 2026-07-20 · t2
- **Status**: live

### D-3 — jobs (J1 attribute spend · J2 efficiency beside cost · J3 cost-per-feature rollup)

- **Question**: What are the primary jobs the operator does with this measurement?
- **Answer**: **J1 — attribute per-phase spend**: on phase/run completion, see tokens by model + dollar cost, to find where the budget goes and which phase/model to change (the headline, A). **J2 — read efficiency beside cost**: the per-phase structural signals (questions, fog, rework/churn, defects, latency) shown next to cost, so a cheap phase is distinguishable from an effective one (the universal floor, B). **J3 — roll up to cost-per-feature**: a shipped feature's phases sum to one "cost per shipped feature" number (§10's guardrail metric), scoped as "carry a run/feature id on each phase record now, aggregate as a thin read later."
- **Reasoning**: J1+J2 serve the operator's actionable question (which phase, which model, is it worth it); J3 reaches the vision metric without committing v1 to run-grouping machinery beyond a carried id.
- **Anchor**: 2026-07-20 · t3
- **Status**: live

### D-4 — the bet and observable success

- **Question**: What is the bet, and what observable result proves the feature works?
- **Answer**: **Bet** — per-phase spend (tokens by model + cost) and efficiency signals in a per-phase run record let the operator locate and fix expensive/inefficient phases (cost-per-feature down, quality held) and give the factory's routing/optimizer loops (VNEXT-7, F6) their substrate. **Observable success** — for a real front-half run, per completed phase the operator reads tokens-by-model + cost + efficiency signals, where (1) spend **reconciles to the harness's own accounting** (measured, not estimated) and is attributed to the **correct phase**; (2) structural signals are present on **every supported harness** (universal floor); (3) cross-harness spend **degrades honestly** — declared "unavailable on harness X," never a silent zero. **Guardrail banked**: the measurement is **near-zero model-token overhead** — it reads accounting the harness already produces and computes structural signals from artifacts+git; it never spawns an LLM to measure.
- **Reasoning**: tests both halves of the framing — the spend number is real (reconciles, right phase) and the plane is honest cross-harness (universal floor + declared degradation). Near-zero overhead falls out of the design and honors §10's token-consciousness constraint.
- **Anchor**: 2026-07-20 · t4
- **Status**: live

### D-5 — the scope boundary (non-goal set)

- **Question**: What is out of scope — the boundary that keeps this a bounded slice, not the whole OBS plane?
- **Answer**: Eight rulings (O-1..O-8). **Permanent non-goals** — another feature owns it: model routing (O-1), the optimizer/self-improvement loop (O-2), how a phase executes incl. headless/resume (O-3), context reduction/context-pack (O-4), the built application's runtime evidence E0–E4 (O-5). **Waiting room** — our own plane but not v1: the rest of the OBS plane — ~60-kind event stream, health alarms, live dashboard, prompt logging (O-6), global telemetry export (O-7), a query/analytics API (O-8).
- **Reasoning**: O-1..O-5 stop the feature absorbing neighbors that already have owners; O-6..O-8 keep it a bounded slice of the Ledger, not the whole plane — the guard against the A-spine hybrid sliding into all-of-C.
- **Anchor**: 2026-07-20 · t5
- **Status**: live

### D-6 — appetite (v1 covers all four supported harnesses)

- **Question**: What bounds v1's spend/scope — how many harnesses does v1's spend capture cover?
- **Answer**: v1 implements per-phase spend capture across **all four supported harnesses — Claude Code, Codex, Cursor CLI, OpenCode** — alongside the universal structural floor, the per-phase run record, and the thin operator read; proven on real front-half runs on each. The operator **overrode** the recommended "one reconciled harness + seam for the rest": the cross-harness requirement is met concretely in v1, not deferred. D-4's honest-degradation stays the fail-closed floor for any figure a harness *provably* cannot expose (declared unavailable, never a silent zero), but the target is genuine four-harness coverage. **Human attention**: this PRD's review, the design review, the operator reading the measurement on real runs. **Machine work**: the per-phase run record + the structural reader + four harness spend adapters + the carried run/feature id + the thin read. **Rollout burden**: near-zero (additive, opt-in, no migration). **Excluded**: O-1..O-8, the J3 rollup view, any query surface.
- **Reasoning**: the operator set cross-harness support as a hard product constraint earlier this session; committing v1 to all four makes the feature *deliver* that constraint rather than promise it as a seam. The external dependency — whether each harness exposes per-model spend on the user's own key — becomes a v1 design input to resolve (P-1), not a deferred check.
- **Anchor**: 2026-07-20 · t6
- **Status**: live

## Facts

- **F-1**: OBS-2 (token-spend records — per session turn-end, pass/worker attribution, input/output/cache token quartet, turn + tool-use counts) is a harbormaster-parity requirement, unbuilt in pyxis ("Implemented in Pyxis: none yet") — source: docs/prds/observability-and-spend.md OBS-2; docs/prds/README.md
- **F-2**: context-pack (Draft PRD) defers this exact plane — NG-5 (no token-telemetry/observability plane), NG-3 (inline-vs-subagent is harness policy), NG-1 (model routing is a separate effort), WR-3 ("a cost line in the delivery ledger fed by M1 — when a token-accounting convention lands"), A1 (session accounting assumed faithful without a telemetry plane) — source: docs/prds/context-pack/effective.md §3.4, §6.3, §7.3
- **F-3**: domain-model §3.7 plans, but nothing is built — TokenUsageRecord (per-turn: kind, end, token quartet, turns, tool_uses; verdict Generalize), Trace (Generalize / make-explicit), DomainEvent (Generalize; EventSink port; ~60 kinds), PassReport (Port), HealthAlarm (Port), GlobalTelemetry (New) — source: docs/domain-model.md §3.7
- **F-4**: system-design routes "every component → events + trace + spend → Ledger → metrics (zero-touch · interventions · cost)" (F5) and leaves the Ledger event/trace/spend schema an open decision D7 ("append-only files first; queryable later") — source: docs/system-design.md F5, F6, §11 D7
- **F-5**: the observability plane exists to measure zero-touch rate (primary), interventions per feature, design-share, and **cost per shipped feature** (tokens, runner minutes); token/size-consciousness is a standing design constraint — source: docs/product-vision.md §10
- **F-6**: medium-independence — pyxis provides only the engine; the agentic CLI (Claude Code, Codex, Cursor, OpenCode) is one adapter among many, no privileged substrate — source: docs/product-vision.md §2
- **F-7**: observable-by-construction is the *application* evidence plane (E0–E4 for built software), explicitly disjoint from the factory's own operational plane — source: docs/brainstorms/observable-by-construction.md (overlaps); docs/prds/observable-by-construction/
- **F-8**: the pyxis front-half stages (1 `/feature-brainstorm`→`/prd-new`, 3 `/tech-design-brainstorm`, 4 `/tech-design-new`, 7 `/plan-new`) each run as a skill and freeze a durable artifact — the deterministic phase-boundary candidate is Skill-invocation → artifact-write — source: .pyxis/pipeline.md

## Fog

none

## Out of scope

- **O-1**: model routing / which model runs a phase — VNEXT-7 and context-pack NG-1 own routing; this produces the spend substrate routing reads, it never routes (from D-5; boundary of D-2)
- **O-2**: the optimizer / self-improvement loop — F6/VNEXT-15 consume these records; building the adaptation loop is separate (from D-5)
- **O-3**: deciding how a phase executes (inline / subagent / headless / resume) — harness/orchestration policy (context-pack NG-3); the measurement is execution-agnostic, reading whatever accounting the run produced (from D-5)
- **O-4**: context reduction / the context-pack — a separate existing feature; this measures spend, it does not reduce it (from D-5)
- **O-5**: the built application's runtime evidence (E0–E4) — observable-by-construction's disjoint plane; this measures the factory's phases, not the software it builds (from D-5; boundary of D-2)
- **O-6**: the rest of the OBS plane — the full ~60-kind domain-event stream (OBS-1), health alarms/detectors (OBS-4), a live monitoring dashboard (OBS-5), prompt logging (OBS-6): v1 is the per-phase spend+efficiency slice, not the whole plane (from D-5 — waiting room)
- **O-7**: global/anonymous telemetry export (§9) — cross-install aggregation is decoupled from local operation; not v1 (from D-5 — waiting room)
- **O-8**: a query / analytics API — v1 is producer-side records + a thin operator read; a normalized query surface is later (from D-5 — waiting room)

## Candidate glossary terms

- **T-1**: **phase run record** — the per-phase record this feature produces: one record per pipeline phase carrying model × token-quartet × cost + structural efficiency signals + a run/feature id · avoid: *TokenUsageRecord* (OBS-2's per-turn record — this is per-phase, aggregated over the phase's turns), *Ledger* (the store, not the record)
- **T-2**: **phase boundary** — the deterministic bracket that attributes spend and turns to a phase: Skill-invocation (open) → artifact-write (close) · avoid: *session* (may span several phases), *turn* (the sub-unit inside a phase)
- **T-3**: **spend reconciliation** — checking a recorded per-phase spend figure against the harness's own accounting, so the number is measured, not estimated · avoid: *estimation* (the bar is reconciliation to ground truth)

## Parked technical constraints

- **P-1**: The cross-harness spend-capability matrix is a v1 design input (D-6): for each of Claude Code, Codex, Cursor CLI, OpenCode, determine whether per-model token+cost is exposed **on the user's own key** and by what channel — headless result envelope, session transcript/JSONL, OTEL metrics, or an LLM proxy — and where D-4's honest-degradation must apply. Decides per-harness adapter feasibility and the "two-adapters-make-a-seam" question — context: D-4, D-6, F-6
- **P-2**: The deterministic **phase boundary** — Skill-invocation → artifact-write — as the attribution bracket, and how each harness signals it (a Skill-tool call or slash-command expansion for the open; the frozen artifact write / PR for the close). Attributing spend and turns to a phase rests on this — context: F-8, D-4
- **P-3**: The per-phase run-record **schema and home** — one record per phase carrying model × token-quartet × cost + the structural signals + a run/feature id; whether it lands as a first append-only slice of the Ledger (system-design D7, open) or as the delivery-ledger cost line (context-pack WR-3). Additive-schema discipline (AP-6) — context: F-2, F-3, F-4
- **P-4**: Per-harness spend-capture **mechanism** must be **execution-agnostic** (O-3): the same phase-attributed figure whether the phase ran inline, as a subagent, or headless — reading whatever accounting the run produced — context: O-3, D-4
- **P-5**: **Structural-signal extraction** from the frozen corpus + git — questions (`D-n`), fog (`G-n`), rework (supersessions / amendments), defects (review `FND-n`), coverage, latency (git timestamps) — a reader over records/reviews/plans, no runtime dependency — context: D-1 (B-floor), F-8
- **P-6**: **Spend reconciliation** — the method and tolerance by which a recorded per-phase figure is checked against the harness's own accounting (D-4's "reconciles" bar), under the near-zero-overhead constraint (no LLM spawned to measure) — context: D-4
- **P-7**: **Dollar-cost derivation** — per-model price tables (which drift) as the source for the $ figure layered on tokens × model; prices kept as versioned data — context: D-3, D-4

## Gate

| Field | Value |
|---|---|
| **Verdict** | ready-for-prd |
| **Date** | 2026-07-20 |
| **Operator** | @mdarabi |
| **Covering source** | — |
| **Coverage check** | pass — all six inputs covered (problem D-1 · segments D-2 · jobs D-3 · goals D-4 · non-goals O-1..O-8 · appetite D-6); zero blocking fog |
