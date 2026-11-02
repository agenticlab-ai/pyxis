# Maintenance sweep — brainstorm record

| Field | Value |
|---|---|
| **In one line** | Build stage 11's first mechanical slice: a read-only, on-demand + schedulable maintenance sweep that detects operational rot — landed-but-unwired packages, frozen-plan Touches that never materialized, stale living rollups — as typed findings with named remedies, never fixes. |
| **Relevant-when** | brainstorming or contracting periodic maintenance / drift detection (stage 11), dead-code or unwired-code detection, plan-charter under-delivery signals, living-rollup freshness; checking prior rulings about findings-vs-fixes boundaries or what the sweep deliberately does not gate. |
| **Idea** | *(composed by Argus under the operator's delegated intake — problem:maintenance-sweep#A-1; the delegation's verbatim text and the composed statement are the problem ledger's Idea section)* Nothing in the factory notices operational rot: this session hand-found a whole feature landed as dead code (#514, #515) and pipeline-map rows lagging reality (PR #519). Build stage 11's first slice: a periodic maintenance sweep that mechanically detects that rot class and emits typed findings with named remedies — read-only, findings never fixes, on demand and schedulable like the wake tick. |
| **Operator** | Argus (decision principal, tier-2) under problem:maintenance-sweep#A-1; @mdarabi (operator) is the overturning authority through the normal channels |
| **Date(s)** | 2026-07-19 |
| **Status** | gated: ready-for-prd |
| **Tracking** | — *(the introducing PR, once filed)* |
| **Feeds** | docs/prds/maintenance-sweep/ |
| **Prior records** | brainstorm:argus-unattended-drive, brainstorm:implementation-verification, brainstorm:contract-gate-tech-design *(orientation only)* |
| **Template version** | brainstorm-record/v1 |

*Authoring note — agent principal (feature-brainstorm FR-16/FR-17). This record was driven by Argus as decision principal at tier-2, not by a live human interview. Each decision below was resolved by a recorded, attributed assumption that passed the legality tests (tier · channel · regression), cited to its `assumption-recorded` journal entry in `delivery:maintenance-sweep`; the intake pre-ruling `problem:maintenance-sweep#A-1` is the live operator ruling binding throughout. Every `D-n` carries its authority anchor, standing until a human overturns it through the normal channels. No governance-tier content was authored or modified.*

## Orientation

| Source | Why loaded |
|---|---|
| docs/product-vision.md | §4 row 11 — the stage's charter: "stale information, dead code, refactoring opportunities — and drift: places where reality has diverged from documents or policy"; §11's "Conformance & drift sentinel" bullet ("drift detection is central to a trustworthy factory"); "the downstream edge" names stages 10–11 as first-class stages with their own gates and reports |
| .pyxis/pipeline.md | stage-11 row ⛔ not built; the stage-8/9 wiring reality the sweep audits |
| docs/prds/docgate/effective.md | FR-10 *(retired, amendment 017)* — the per-PR rollup-freshness gate was deliberately retired; freshness is now an unowned git-history convention — exactly the vacancy a staleness detector fills without re-fighting 017's ruling |
| internal/verify/checks/ (codegate) | the stage-8 verification harness is **proposal-scoped** (secret scan, forbidden artifacts, machinery protection, test-delta, build/lint on a diff); it never scans factory state periodically — disjoint by moment and subject |
| internal/docgate/gate/ | docgate gates **documents** at push/CI (immutability, ledger grammar, content paths); no check owns code-wiring rot or rollup freshness (post-017) |
| .pyxis/templates/plan-template.md | Touches semantics: "plan-internal surface list — used for overlap sequencing, never rendered to workers" — so a Touches-vs-shipped divergence is an advisory drift *signal*, not a breach proof |
| docs/tech-design/wake-channel-v1.md · .pyxis/scripts/wake-tick.sh | the scheduling pattern the sweep mirrors: one bounded pass, nothing resident, host scheduler binding, ~0 idle cost (AP-19) |
| docs/architecture-principles.md | AP-10 (mechanize judgment), AP-17 (fail closed), AP-19 (recurring paths state their cost), AP-25 (vendor-neutral agent surface), AP-26 (goal objects, never badges) |
| docs/wiki/glossary.md | vocabulary check — "docgate", "attribution gate", "wake signal" exist; no collision with the candidate terms below |
| docs/problems/argus-unattended-drive/ · #514 · #515 · PR #519 | the empirical rot evidence: lane + wake organs landed reachable from no binary (T4/T8 charter Touches named wiring files that never shipped); two map rows stale for days |

**Overlaps surfaced:**
- **codegate (implementation-verification)** — verifies a *proposal's* diff at stage 8; the sweep reviews *factory state* periodically (stage 11). Different moment, different subject; the sweep must not re-gate proposals. *Disjoint — proceed.*
- **docgate gate** — documents at push/CI. The retired FR-10 (017) shows the factory deliberately moved rollup freshness OUT of the per-PR gate; a periodic detector honors that ruling by living in a different moment. *Vacancy, not overlap.*
- **contract-gate (stage 6)** — compiles contracts at planning entry; does not audit landed reality afterward. *Disjoint.*
- **The drive's touchpoint duties** (argus FR-15: parks/aging bumps) — per-problem, drive-scoped; the sweep is repo-wide and problem-agnostic. *Disjoint.*

## Framings

- **Framing A — the drift sentinel program**: build the vision's full conformance sentinel (design-vs-architecture, capability-vs-tests, incident-lessons-vs-changes, policy-vs-PR-behavior). Implications: semantic judgment, engine-era substrate, months of ground; directly the §11 bullet but far beyond a first slice.
- **Framing B — the first mechanical slice**: a read-only sweep with a small closed set of *mechanical* detectors targeting the rot classes just observed, typed findings + remedies, on-demand + schedulable. Opens the stage-11 lane, earns evidence for the sentinel program, ships in one plan.
- **Framing C — fold rot checks into docgate's gate**: add the detectors as docgate checks at push/CI. Rejected: wrong moment (per-PR gates punish the wrong actor for pre-existing repo state; 017 retired exactly that shape for freshness) and wrong actor (stage 11 is Factory-periodic, not a proposal gate).

**Destination:** Framing B — the first mechanical slice. Rationale: A-1's delegation asks for a missing feature made real, not a program; the evidence base (#514/#515/#519) defines a concrete, mechanically detectable rot class; the vision's stage-11 row is satisfied incrementally ("first-class stage with its own gates and reports" grows from a working slice). (delivery:maintenance-sweep#E-2)

## Decisions

### D-1 — Destination: one feature — the stage-11 first slice, mechanical detectors only

- **Question**: Program (Framing A), first mechanical slice (B), or docgate extension (C)?
- **Answer**: B. One feature: a read-only maintenance sweep, closed detector set, typed findings with remedies; its own moment (periodic / on-demand), not a per-PR gate.
- **Reasoning**: The evidence class is mechanical; 017's retirement rules out the per-PR shape (C) for freshness-class checks; A (the semantic sentinel) is engine-era ground the vision reserves for later slices. Legality: tier — contradicts no governance content, no A-n; channel — a new feature PRD; regression — no existing gate or contract moves.
- **Anchor**: 2026-07-19 · Argus assumption (delivery:maintenance-sweep#E-2)
- **Status**: live

### D-2 — The v1 detector set: three detectors, closed

- **Question**: Which detectors ship in v1?
- **Answer**: Exactly three: (1) **unwired-packages** — first-party packages reachable from no `cmd/` binary (the dead-feature class of #514/#515), with an explicit waiver surface for deliberately library-only packages; (2) **plan-touches under-delivery** — a frozen plan task's Touches naming surfaces that do not exist on the mainline (advisory signal, per Touches' planning-metadata semantics); (3) **living-rollup staleness** — docs/techspec.md and docs/prd.md older than the shipped structure they describe (the git-history freshness convention, mechanized read-only).
- **Reasoning**: Each detector has a proven instance from this session's evidence and a purely mechanical decision procedure. Map-row semantic drift fails that bar → parked (G-1). Dead exported *symbols* needs an analysis dependency decision → parked (G-2). Legality: tier/channel/regression all clean — read-only detection, no standing contract moves.
- **Anchor**: 2026-07-19 · Argus assumption (delivery:maintenance-sweep#E-2)
- **Status**: live

### D-3 — Findings, never fixes; typed, closed, deterministic

- **Question**: What does the sweep emit, and what may it do about what it finds?
- **Answer**: A typed finding per hit — detector, subject, evidence, named remedy — in a closed, versioned schema with deterministic output and meaningful exit codes. The sweep mutates nothing: no auto-fix, no issue filing, no tracker writes in v1; the invoker (human, drive, or a later engine stage) disposes findings.
- **Reasoning**: The idea's own boundary ("findings, never fixes"); AP-17 (a detector that also mutates is a new failure surface); keeps v1's blast radius zero so the schedulable path is safe. Legality: clean (adds no authority, touches nothing standing).
- **Anchor**: 2026-07-19 · Argus assumption (delivery:maintenance-sweep#E-2)
- **Status**: live

### D-4 — Invocation: on-demand verb + schedulable like the wake tick; never resident

- **Question**: How does the sweep run?
- **Answer**: One bounded pass per invocation — read state, emit findings, exit — runnable on demand and bindable to the host scheduler exactly as the wake tick is (thin shim, operator-installed schedule); nothing resident; idle/steady-state cost stated (AP-19).
- **Reasoning**: The wake-tick pattern is the landed house shape for periodic factory passes; reusing it keeps stage 11's "periodic" real without a daemon. Legality: clean.
- **Anchor**: 2026-07-19 · Argus assumption (delivery:maintenance-sweep#E-2)
- **Status**: live

### D-5 — Consumers and segments: the factory's operators and drives, not per-PR contributors

- **Question**: Who consumes findings, and who must this NOT burden?
- **Answer**: Primary: the operator and an Argus drive at its touchpoints (a finding becomes an issue or a problem statement — as #514/#515 did by hand). Secondary: the future engine's stage-11 report. Anti-segment: PR authors — the sweep never gates a proposal and adds no per-PR friction.
- **Reasoning**: Mirrors the stage's Factory actor in the vision table; the anti-segment honors 017's ruling and Framing C's rejection. Legality: clean.
- **Anchor**: 2026-07-19 · Argus assumption (delivery:maintenance-sweep#E-2)
- **Status**: live

### D-6 — Success observable: the sweep catches this session's rot on replay, and runs clean-or-honest on the live repo

- **Question**: What observable makes v1 "working"?
- **Answer**: Two-sided: (a) **replay** — on repo states where the known rot existed (lane/wake unwired before #517/#518; the T4/T8 Touches gap), the sweep emits the corresponding findings; (b) **live** — on current mainline the sweep completes a full pass with zero false positives among its emissions (every finding is a true, named condition or an explicit waiver candidate).
- **Reasoning**: The feature exists because these instances went unnoticed; detecting them retroactively is the honest acceptance bar, and zero-false-positive on live keeps the tool trusted (a noisy sweep dies of ignoring). Legality: clean.
- **Anchor**: 2026-07-19 · Argus assumption (delivery:maintenance-sweep#E-2)
- **Status**: live

### D-7 — Appetite: one plan, small; stdlib + go-toolchain reads only

- **Question**: How much is v1 allowed to cost?
- **Answer**: One implementation plan of ≤ 6 tasks; no new third-party dependencies (stdlib + invoking the go toolchain and git as read-only subprocesses); detectors that would breach that (symbol-level dead-code analysis) stay parked.
- **Reasoning**: The repo's dependency posture (AGENTS: stdlib first, small supply-chain surface); the three detectors are implementable within it (`go list` reachability, file existence vs plan JSON, git log age reads). Legality: clean.
- **Anchor**: 2026-07-19 · Argus assumption (delivery:maintenance-sweep#E-2)
- **Status**: live

## Facts

- **F-1**: The vision's stage-11 charter is "Review of the code base and live documents for stale information, dead code, refactoring opportunities — and drift" (Factory actor, continuous), with §11's "Conformance & drift sentinel" naming drift detection "central to a trustworthy factory" — source: docs/product-vision.md §4 row 11, §11 bullets.
- **F-2**: docgate FR-10 (the per-PR rollup-freshness gate) was retired by amendment 017; rollup freshness is today an unowned convention read from git history — source: docs/prds/docgate/effective.md (FR-10 retired (017)); AGENTS.md rule 3.
- **F-3**: codegate's check set is proposal-scoped (secret scan, forbidden artifacts, machinery protection, test-delta, build/lint/falsification over a diff) — no periodic factory-state scan exists — source: internal/verify/checks/.
- **F-4**: Plan Touches is "plan-internal surface list — used for overlap sequencing, never rendered to workers" — planning metadata, not a delivery contract — source: .pyxis/templates/plan-template.md header table.
- **F-5**: The empirical rot instances: lane + wake organs landed with zero reachability from any binary (issues #514/#515 — `go list -deps ./cmd/...` reached no `internal/lane|wake` package; plan T4/T8 Touches named `.github/workflows/merge-lane.yml` and `scripts/wake-tick.sh`, never shipped); pipeline-map stages 6/9 lagged landed reality (corrected in PR #519) — source: the issues, PR #519, delivery:maintenance-sweep#E-1.
- **F-6**: The wake tick is the landed pattern for a periodic factory pass: one bounded pass, nothing resident, host-scheduler binding via a thin shim, stated ~0 idle cost — source: docs/tech-design/wake-channel-v1.md §3.2; .pyxis/scripts/wake-tick.sh.

## Fog

- **G-1** (non-blocking — blocks: a later detector slice): pipeline-map row drift (a stage row's status vs landed mechanisms) needs a semantic mapping from rows to observable mechanisms; not mechanically honest in v1 — parked for a future slice or the LLM-era sentinel.
- **G-2** (non-blocking — blocks: a later detector slice): symbol-level dead-code detection (unused exported identifiers) needs an analysis-tooling dependency decision (x/tools or equivalent) that D-7's appetite excludes; parked with the dependency question.
- **G-3** (non-blocking — blocks: a later policy decision): whether sweep findings eventually auto-file tracker issues (and under what dedup discipline) is a stage-12/engine-era policy question; v1 stays read-only per D-3.

## Out of scope

- **O-1**: No fixes, ever, from the sweep — findings only; mutation is the consumer's (from D-3).
- **O-2**: No per-PR gating moment — the sweep never blocks or evaluates a proposal; that ground belongs to docgate/codegate/the lane (from D-1, D-5).
- **O-3**: No semantic/LLM detectors in v1 — mechanical decision procedures only (from D-1, D-2).
- **O-4**: No map-row drift detection in v1 (G-1's semantic bar) (from D-2).
- **O-5**: No tracker writes in v1 — no auto-filed issues, no item edits (from D-3; G-3 parks the future question).
- **O-6**: No new third-party dependencies — stdlib + go/git subprocess reads (from D-7).

## Candidate glossary terms

- **T-1**: maintenance sweep — stage 11's bounded, read-only factory pass detecting operational rot as typed findings with named remedies; never fixes, never gates a proposal · avoid: "janitor" (implies mutation), "audit" (implies human judgment).
- **T-2**: rot finding — one typed maintenance-sweep emission: detector, subject, evidence, named remedy; advisory by construction · avoid: "violation" (the sweep gates nothing).
- **T-3**: unwired package — a first-party package no shipped binary reaches (dead-feature rot, the #514/#515 class) as distinct from deliberately library-only packages, which carry a waiver · avoid: "dead code" (that names the symbol-level class, parked in G-2).

## Parked technical constraints

- **P-1**: The sweep's home (its own binary vs a verb on an existing one) and package layout — stage 3/4 ground; the detector set's closed schema and the check-registry pattern reuse (docgate's Register model) are candidate shapes, not rulings.
- **P-2**: The findings schema and output faces (human + machine, docgate's dual-face convention), exit-code contract, and determinism requirements — design ground within D-3's boundary.
- **P-3**: The waiver surface for unwired-packages (config-surface list vs in-tree markers) must parse closed and stay operator-legible — design ground; the config surface's closed-parse rule (unknown keys are errors) binds whichever home is chosen.
- **P-4**: Replay fixtures for D-6(a): how the historical rot states are pinned (fixture repos vs recorded `go list` outputs) — test-strategy ground for the design.
- **P-5**: The scheduling binding reuses the wake-tick shim pattern (thin delegate-only script, operator-installed schedule) — the design names the concrete shim and its test.

## Gate

| Field | Value |
|---|---|
| **Verdict** | ready-for-prd |
| **Date** | 2026-07-19 |
| **Operator** | Argus (decision principal, tier-2) — `sign-gate-verdict` GRANTED at tier-2; verdict signed by Argus per the authority contract, journaled at delivery:maintenance-sweep#E-3. Not a human sign-off; the operator overturns through the normal channels. |
| **Covering source** | — |
| **Coverage check** | pass — all six decision-shaped inputs covered (§2.1 problem via Destination/D-1; §2.2 segments via D-5; §2.3 jobs via D-2/D-5 — detect the three rot classes, feed dispositions; §3.1 goals direction via D-6; §3.4 non-goals via O-1..O-6; header appetite via D-7), zero blocking fog (G-1, G-2, G-3 all non-blocking). |
