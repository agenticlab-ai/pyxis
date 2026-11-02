# Maintenance sweep — design-brainstorm record

| Field | Value |
|---|---|
| **In one line** | Shape the stage-11 maintenance sweep's realization: one new read-only binary + package, a closed finding/v1 dual-face schema, one closed-parse policy file (waivers + bounds), hermetic replay fixtures, and the landed delegate-only shim pattern for scheduling. |
| **Relevant-when** | designing or changing the maintenance sweep's binary/package home, its findings schema or exit-code contract, the maintenance policy file (waivers, staleness bound), detector decision procedures, or its scheduler binding; checking prior design rulings or refused shapes for stage-11 machinery. |
| **Basis** | prd:maintenance-sweep (merged ledger — frozen at PR #522, as of 000, sha256:4d361cc7…) |
| **Authority** | Argus (decision principal, tier-2) under problem:maintenance-sweep#A-1; @mdarabi (operator) overturns through the normal channels |
| **Date(s)** | 2026-07-19 |
| **Status** | gated: ready-for-design |
| **Tracking** | — *(the introducing PR, once filed)* |
| **Feeds** | docs/tech-design/maintenance-sweep-v1.md |
| **Prior records** | brainstorm:maintenance-sweep *(the stage-1 basis record; P-1..P-5 are this session's question set)* · design-brainstorm:argus-unattended-drive-tech-design *(orientation: the sibling periodic-pass and policy-file shapes)* |
| **Template version** | design-brainstorm-record/v1 |

*Authoring note — agent principal. This record was driven by Argus as decision principal at tier-2 under the landed agent-principal seam, not by a live human interview; each `D-n` is a recorded, attributed, legality-tested assumption journaled at `delivery:maintenance-sweep#E-9`, standing until a human overturns it through the normal channels. No governance-tier content was authored or modified.*

## Orientation

| Source | Why loaded |
|---|---|
| docs/prds/maintenance-sweep/effective.md | the frozen basis — FR-1..FR-8, UB-1..UB-5, NR/NFR/DOD; §4.1's evaluated-state definition; §5.5's interface table |
| docs/brainstorms/maintenance-sweep.md | the stage-1 record: D-1..D-7 rulings, P-1..P-5 (this session's question set), O-1..O-6, F-1..F-6 |
| docs/tech-design/index.md + docs/tech-design/wake-channel-v1.md | Serves overlap scan; the periodic-pass pattern (one bounded pass, host shim, ~0 idle — F-6) |
| docs/tech-design/merge-lane-v1.md + docs/solution-shapes/merge-lane-v1.md | the sibling policy-file precedent (closed parse, paths key, docs/policy home) and the shape-pairing discipline (DINV witnessing) |
| docs/tech-design/solution-shape-v1.md | when a shape is required and what it must carry (DINV-1: a frozen design contains no concrete shape) |
| docs/tech-design/docgate-v1.md + internal/docgate/gate/registry.go | the dual-face defect convention and the check-registry pattern (candidate shapes per stage-1 P-1) |
| internal/docgate/config/config.go | the closed config parse; the `paths.*` key pattern (risk_policy precedent) the policy home reuses |
| docs/plans/argus-unattended-drive/001-plan.json | the plan machine face the Touches detector reads: `tasks[].touches` — middot-joined surface strings incl. non-path tokens ("pipeline-map") |
| .pyxis/scripts/wake-tick.sh + .pyxis/scripts/tests/test-wake-tick-shim.sh | the landed delegate-only shim + shim-test conventions FR-7 pins |
| docs/architecture-principles.md · docs/observability/incidents.md | AP-5/6/10/17/19/25/26/28; incident I3 (fail-closed misparse — UB-5's source) |

**Overlaps surfaced:** wake-channel-v1 decides the periodic-pass *pattern* (reused, not re-decided); merge-lane-v1 decides the policy-file *pattern* (reused); no live design decides the sweep's own home, schema, or detectors — proceed.

## Design questions

- **DQ-1**: Where does the sweep live — its own binary + package tree, a docgate verb, or a wake-tick verb — and what package layout?
- **DQ-2**: What are the findings' concrete faces — schema class, output channels, exit codes — and how is FR-6's determinism realized?
- **DQ-3**: Where do waivers and the staleness bound live, and under what parse?
- **DQ-4**: What are the three detectors' exact mechanical decision procedures (incl. the non-path Touches tokens and the in-flight-plan noise problem)?
- **DQ-5**: How are the replay fixtures (FR-2..FR-4, DOD-2) pinned?
- **DQ-6**: What is the scheduler binding?

## Panel log

- **DQ-1**: opens (a new binary/package boundary) — fired — the home is a seam with ≥ 3 structurally distinct candidates
- **DQ-2**: opens (a new schema/document class — finding/v1) — fired — the finding's carrier could be the existing defect schema, a new class, or unstructured text
- **DQ-3**: extends (the merge-lane policy-file + paths-key pattern) — skipped — one landed precedent fits all constraints; no new seam
- **DQ-4**: extends (detector semantics inside the chosen package; no new seam) — skipped — decision procedures, not boundaries
- **DQ-5**: extends (the house hermetic temp-git fixture pattern) — skipped — test strategy on an existing seam
- **DQ-6**: extends (the landed delegate-only shim pattern; FR-7 already binds it) — skipped — reuse is the requirement, not a choice

## Decisions

### D-1 — The home: a new `cmd/maintenance-sweep` binary over one `internal/sweep` package

- **Question**: DQ-1 — own binary, docgate verb, or wake-tick verb; package layout?
- **Answer**: A new thin main `cmd/maintenance-sweep` delegating to one focused package `internal/sweep` (findings schema, policy load, the three detectors, the pass runner — one package; no sub-package sprawl at this size). ALT-2/ALT-3 rejected.
- **Reasoning**: The PRD's NG-2/NR-2 demand a moment and failure domain disjoint from docgate; the glossary term is the binary's name; one package fits D-7's appetite and the house "small, focused packages" rule. Legality: tier/channel/regression clean (new code surface only).
- **Anchor**: 2026-07-19 · Argus assumption (delivery:maintenance-sweep#E-9)
- **Status**: live

### D-2 — The faces: a new closed `finding/v1` report schema; machine face stdout, human face stderr; exit 0/1/2

- **Question**: DQ-2 — the findings' carrier, channels, exit codes, determinism?
- **Answer**: A new closed, versioned report schema (`maintenance-report/v1` carrying `finding/v1` elements): ONE deterministic JSON report object on stdout (the machine face; EVT-1's record), human-readable lines on stderr (the human face); exit 0 clean · 1 findings-present · 2 fault (usage/misparse/unreadable state/malformed policy — UB-1/UB-2/UB-5). Determinism: the machine face carries no wall-clock field (FR-6); ordering is fixed (detector order, then subject sort).
- **Reasoning**: Reusing docgate's `schema.Defect` (ALT-5) conflates gate semantics ("defect" = a gate failure; a rot finding is advisory by contract — R3); unstructured text (ALT-6) fails NFR-3/M3. The dual-face split mirrors the house convention. Legality: clean.
- **Anchor**: 2026-07-19 · Argus assumption (delivery:maintenance-sweep#E-9)
- **Status**: live

### D-3 — The policy home: one closed-parse file `docs/policy/maintenance-sweep.yaml` behind a `paths.maintenance_policy` config key

- **Question**: DQ-3 — where do waivers and the staleness bound live?
- **Answer**: One policy file owning all sweep policy — schema version, the staleness bound, package waivers, and plan waivers (each waiver reason-required per FR-5; a reasonless entry refuses under the closed parse) — homed at `docs/policy/maintenance-sweep.yaml`, located via one new `paths.maintenance_policy` config key (the risk-policy precedent). Absent file = empty policy + default bound (a legal state, unlike the lane's fail-closed policy — the sweep is advisory); malformed/unknown-key = UB-2 fault.
- **Reasoning**: One surface, one validator, operator-legible in review (docs/policy/ is floor-gated in the merge lane — waiver edits get human routing for free). Splitting bounds into `.pyxis/config.yaml` would put one feature's policy in two homes. Legality: clean (additive config key; the closed section list grows by one `paths.` entry).
- **Anchor**: 2026-07-19 · Argus assumption (delivery:maintenance-sweep#E-9)
- **Status**: live

### D-4 — Detector procedures: reachability by `go list`; Touches path-tokens only, plan-waivable; staleness by capability-path commit ages

- **Question**: DQ-4 — the exact mechanical procedures, incl. non-path Touches tokens and in-flight plans?
- **Answer**: (1) **unwired-packages**: first-party = `go list ./...` under the module path, minus `cmd/*` mains and test-only conformance packages' callers; unwired = not in the union of `go list -deps` over every `cmd/*` main; minus package waivers. (2) **touches-underdelivery**: for each `docs/plans/*/NNN-plan.json` task, split each `touches` entry on the house middot, keep only path-like tokens (contains `/` or `.`), report tokens absent from the evaluated state (file OR directory-prefix match); non-path tokens (e.g. `pipeline-map`) are skipped as non-mechanical; a plan waiver (reason-required) suppresses its plan's findings surfacing the reason — the in-flight-plan noise control. (3) **rollup-staleness**: for each living rollup (docs/techspec.md, docs/prd.md), stale iff newest commit touching the config's capability paths is newer than the rollup's newest commit by more than the policy bound; ages from committed timestamps (deterministic per state — FR-6-legal).
- **Reasoning**: Each procedure is a pure function of the evaluated state + policy; the path-token rule and plan waivers keep FR-3's advisory framing honest against R3 and the zero-FP live bar (M2) with the freshly-frozen installer plan in flight. Legality: clean.
- **Anchor**: 2026-07-19 · Argus assumption (delivery:maintenance-sweep#E-9)
- **Status**: live

### D-5 — Replay pinning: hermetic temp-git fixture repos reproducing the historical shapes by name

- **Question**: DQ-5 — how are the FR-2/FR-3 historical replays and the FR-4 constructed fixture pinned?
- **Answer**: Hermetic temp-git fixture repos built in-test (the house pattern): the unwired fixture reproduces the #514/#515 shape with the real package names (`internal/lane/*`, `internal/wake` present; no `cmd/*` import path reaching them); the Touches fixture reproduces the T4/T8 gap verbatim (a frozen-plan JSON whose touches name `.github/workflows/merge-lane.yml` and `scripts/wake-tick.sh`, absent from the tree); the staleness fixture constructs a rollup older than a capability-path commit beyond the bound. No dependence on the real repo's history at test time.
- **Reasoning**: Pinning real SHAs would make the suite non-hermetic (needs full history at depth); shape-plus-names replay preserves the honest bar (DOD-2's "pinned fixtures") while keeping tests hermetic per the repo Go rules. Legality: clean.
- **Anchor**: 2026-07-19 · Argus assumption (delivery:maintenance-sweep#E-9)
- **Status**: live

### D-6 — Scheduling: `.pyxis/scripts/maintenance-sweep.sh`, delegate-only, with the standard shim test

- **Question**: DQ-6 — the scheduler binding?
- **Answer**: A delegate-only shim `.pyxis/scripts/maintenance-sweep.sh` mirroring the wake-tick shim line-for-line in discipline (locate root, exec `go run ./cmd/maintenance-sweep`), scheduler examples documented in its header, covered by a `.pyxis/scripts/tests/` shim test (delegation, thinness, bash-3.2 floor, one hermetic dry pass). The schedule itself is operator-installed, never by the repo (FR-7, NFR-2).
- **Reasoning**: FR-7 binds the pattern; the wake-tick shim + test are the landed exemplar. Legality: clean.
- **Anchor**: 2026-07-19 · Argus assumption (delivery:maintenance-sweep#E-9)
- **Status**: live

## Alternatives

### ALT-1 — Own binary + one package (`cmd/maintenance-sweep` + `internal/sweep`)

- **Shape**: a thin main over one focused package holding schema, policy, detectors, and the pass.
- **Trade-offs**: wins on failure-domain isolation (NR-2), naming clarity (glossary T-1), and the sweep's disjoint moment; costs one more binary in the repo.
- **Principles**: AP-19 honored (one bounded pass, stated cost); AP-25 honored (shim-invokable, vendor-neutral); the house thin-main/small-package rules honored.
- **Incidents**: none matched — registry read 2026-07-19 (I1–I3 checked; I3 lands as the UB-5 misparse contract, not a home constraint).
- **Verdict**: chosen — the only shape satisfying NG-2/NR-2's disjointness without inventing a new host.
- **From**: DQ-1 · panel

### ALT-2 — A docgate verb (`docgate maintain`)

- **Shape**: register the detectors as docgate checks / a new verb inside the docgate binary.
- **Trade-offs**: wins on zero new binaries and registry reuse; costs moment-conflation — docgate is the push/CI document gate, and NR-2 demands its behavior unchanged; couples the sweep's faults to the gate's exit-code contract; the PRD's own §4.1 rejected the gate-fold framing (option B).
- **Principles**: strains AP-11 (one meaning per verb surface — a gate binary emitting advisory non-gate output muddies both).
- **Incidents**: none matched — registry read 2026-07-19.
- **Verdict**: rejected — wrong host; the basis's D-1/NG-2 already refused the moment, and the binary boundary is what keeps NR-2 trivially true.
- **From**: DQ-1 · panel

### ALT-3 — A wake-tick verb (extend `cmd/wake-tick` with a sweep pass)

- **Shape**: the sweep as a second verb on the existing periodic binary (shared shim, shared scheduling).
- **Trade-offs**: wins on shim reuse and one periodic surface; costs concern-fusion — the tick is the wake channel's delivery guarantee (its contract, ledger-scoped), the sweep is repo-state analysis; a sweep fault would share the tick's exit surface and blur DINV-4's stated-cost line; separate schedules become impossible.
- **Principles**: strains RW-5 (two independently-revisable behaviors in one surface) and AP-6 (two report schemas on one stdout).
- **Incidents**: none matched — registry read 2026-07-19.
- **Verdict**: rejected — pattern reuse is the shim discipline, not binary cohabitation.
- **From**: DQ-1 · panel

### ALT-4 — New closed `maintenance-report/v1` schema (chosen face)

- **Shape**: one JSON report object per pass — schema_version, evaluated state, per-detector blocks (findings, applied waivers, fault), outcome — stdout machine face; stderr human face; exit 0/1/2.
- **Trade-offs**: wins on NFR-3/M3 (parseable, disposable findings), FR-6 (deterministic by construction, no wall-clock), and honest partials (UB-3 names the faulted detector inside the report); costs one more schema class to maintain.
- **Principles**: AP-6 honored (schema-versioned closed contract, dual faces); AP-5 honored (the report is the event record — EVT-1).
- **Incidents**: none matched — registry read 2026-07-19.
- **Verdict**: chosen.
- **From**: DQ-2 · panel

### ALT-5 — Reuse docgate's `schema.Defect` as the finding carrier

- **Shape**: findings emitted as the existing defect JSON lines.
- **Trade-offs**: wins on zero new schema; costs semantic conflation — a defect is a gate failure demanding remediation before proceed; a rot finding is advisory by contract (R3's misread risk becomes structural), and the defect shape lacks the finding's four contracted fields (detector/subject/evidence/remedy — FR-1).
- **Principles**: strains AP-6 (a schema reused against its meaning) and the basis's D-3.
- **Incidents**: none matched — registry read 2026-07-19.
- **Verdict**: rejected — the finding's contract needs its own closed type; defects remain the FAULT face (UB rows), findings the content face.
- **From**: DQ-2 · panel

### ALT-6 — Unstructured human-only output

- **Shape**: printf findings, no machine face.
- **Trade-offs**: cheapest; fails NFR-3 (machine face is contracted), M3's disposability, and the engine-era consumer (P3 segment).
- **Principles**: violates AP-6.
- **Incidents**: none matched — registry read 2026-07-19.
- **Verdict**: rejected.
- **From**: DQ-2 · panel

## Seam map

- **S-1**: `cmd/maintenance-sweep` + `internal/sweep` — opens — behind it: the pass runner, three detectors, policy load, report emission — feeds: the design's Scope + Serves (FR-1..FR-8)
- **S-2**: `maintenance-report/v1` (+ `finding/v1` elements) — opens — behind it: the machine face every consumer parses (EVT-1, NFR-3) — feeds: the design's §3 schema section + the solution shape (authoritative)
- **S-3**: `docs/policy/maintenance-sweep.yaml` + `paths.maintenance_policy` — extends (the risk-policy pattern) — behind it: waivers (package + plan, reason-required) and the staleness bound — feeds: the design's policy section (FR-5, UB-2)
- **S-4**: `.pyxis/scripts/maintenance-sweep.sh` — extends (the wake-tick shim pattern) — behind it: the host-scheduler binding — feeds: the design's scheduling section (FR-7, DOD-5)

## Facts

- **F-1**: The plan machine face carries `tasks[].touches` as middot-joined strings incl. non-path tokens — e.g. T4 `"internal/lane/acting · .github/workflows/merge-lane.yml"`, T11 `"pipeline-map"` — source: docs/plans/argus-unattended-drive/001-plan.json.
- **F-2**: The shape-consistency gate requires every declared `**DINV-n**` witnessed by a shape element (fence caption citations) and every shape element to cite a design anchor — source: internal/docgate/techdesigns/witness.go; solution-shape-v1 DINV-4.
- **F-3**: The config parse is closed with a `paths.*` key set; the risk-policy home precedent is `paths.risk_policy: docs/policy/risk-policy.yaml` — source: internal/docgate/config/config.go.
- **F-4**: The freshly-frozen installer plan (docs/plans/installer/001-plan.json, 14 tasks) is materialized and in flight — its Touches surfaces are legitimately absent today, the concrete case the plan-waiver control exists for — source: issues #499–#513; the plan file.
- **F-5**: The landed shim + test conventions: delegate-only, ≤ ~6 code lines, bash-3.2 floor, hermetic dry pass — source: .pyxis/scripts/wake-tick.sh; .pyxis/scripts/tests/test-wake-tick-shim.sh.
- **F-6**: `go list -deps ./cmd/...` is the attested reachability read (the #514/#515 discovery command) — source: issue #514 body; delivery:argus-unattended-drive#E-39.

## Fog

- **G-1** (non-blocking — blocks: the design's Q-n on test-only packages): whether test-only harness packages (e.g. conformance suites) should be auto-excluded from unwired detection or left to waivers — v1 rules waivers-only (simpler, visible); revisit if waiver volume proves noisy.

## Out of scope

- **O-1**: No docgate/wake-tick cohabitation — the sweep is its own binary (from D-1).
- **O-2**: No defect-schema reuse for findings — findings are their own closed class; defects carry only faults (from D-2).
- **O-3**: No second policy home — all sweep policy in one file (from D-3).
- **O-4**: No semantic interpretation of non-path Touches tokens — skipped, never guessed (from D-4).
- **O-5**: No real-history SHA pinning in the replay suite — hermetic shape replay only (from D-5).

## Candidate glossary terms

none — the stage-1 terms (T-1..T-3) cover the vocabulary; no new collisions found.

## Parked product gaps

none — the frozen basis covered every design question; no requirement hole surfaced.

## Gate

| Field | Value |
|---|---|
| **Verdict** | ready-for-design |
| **Date** | 2026-07-19 |
| **Authority** | Argus (decision principal, tier-2) — `sign-gate-verdict` GRANTED; journaled at delivery:maintenance-sweep#E-9; the operator overturns through the normal channels |
| **Covering design** | — |
| **Coverage check** | pass — core four covered (§2 Decision ← D-1/D-2 with DQ-1/DQ-2 panel-logged; §4 Alternatives ← ALT-1..ALT-6 all verdicted; Serves+Scope ← S-1..S-4 + the Basis anchors; §3.2 boundaries ← O-1..O-5), zero blocking fog (G-1 non-blocking) |
| **Amend route** | — |
