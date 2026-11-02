# Pyxis PRD set — requirements index

**Status: requirements baseline · assembled 2026-07-01 · nothing here is built in Pyxis yet**

This directory is the **requirements** side of the document system: what Pyxis
must be able to do. It is deliberately *not* the living capability rollup —
that is `docs/prd.md` (the live "what the system does" document, AP-7),
seeded 2026-07-07 when the first capability (docgate) shipped and updated
as capabilities actually ship (AGENTS.md rule 3 — convention since docgate
amendment 017 retired the FR-10 rollup guard). A capability claimed in a
living doc with no code behind it is exactly the drift VNEXT-4 exists to
catch.

Two requirement sets, with stable citable feature IDs:

- **The baseline** (index below; details in the ten area PRDs): the complete
  feature inventory of harbormaster as built — the parity target the rewrite
  must meet.
- **[vNext](vnext-requirements.md)** (VNEXT-1..13): the new ground beyond
  parity.
- **[vNext: self-improvement & adaptability](vnext-self-improvement.md)**
  (VNEXT-14..15, appended 2026-07-02): convention adoption (org/repo style
  profiles) + self-optimization of prompts, thresholds, and policies —
  extends the immutable vNext doc by addition.

New features enter via pipeline stage 1 (feature definition → per-feature
PRD). A real per-feature PRD is **not** another flat file here: it is a
**ledger directory** — `docs/prds/<feature>/`, with `000-base.md`
instantiated from `prd-template/v2` and evolving only by typed amendments
(`amendment-template/v2`) folded into a generated projection — the
template surface is named in AGENTS.md's layout table — per
[`docs/tech-design/prd-amendment-model.md`](../tech-design/prd-amendment-model.md).
The flat area PRDs and vNext documents in this directory are the
**placeholder requirement corpus** — upstream sources the real PRDs will
cite, exempt from those conventions by operator ruling.

## Inherited north star

*A product engineer's only job is to define the right solution to the business
problem — and what "done" means. Everything else is the factory's job.* The
engineer touches a feature exactly twice: **author the contract** (PRD + design
+ testable definition of done) and **accept the result**. Any other
engineer-minute is a factory gap (the inversion test). Progress metrics:
**zero-touch rate** (primary), **interventions per feature**, **design-share of
engineer time**, guarded by bypass rate, drift MTTR, fleet compliance %, and
cost per shipped feature. *(Origin: harbormaster factory-northstar.md; the
Pyxis product surface is defined in [product-vision.md](../product-vision.md).)*

## Baseline feature index (as built in harbormaster)

Status of the *harbormaster* implementation — i.e. the parity requirement, not
Pyxis state: ✅ shipped there · ◐ partially shipped there (gap noted in the
area PRD). **Implemented in Pyxis: none yet.**

### [Scaffolding & Onboarding](scaffolding-and-onboarding.md)

| ID | Feature | Status |
|---|---|---|
| SCAF-1 | Project scaffolding wizard (repo from nothing, < 60s, idempotent) | ✅ |
| SCAF-2 | Repo bootstrap / adoption (two-phase, permission-split, calibrated checks) | ✅ |
| SCAF-3 | Language profiles (pinned toolchain, CI suite, templates as data) | ✅ |
| SCAF-4 | Org adoption sweep (classify + approve + act, exclusions honored) | ✅ |
| SCAF-5 | Welcome path (install + first design doc = productive) | ✅ |

### [Design & Planning](design-and-planning.md)

| ID | Feature | Status |
|---|---|---|
| PLAN-1 | Brainstorm stage (scored vs principles + incident memory) | ✅ |
| PLAN-2 | Design authoring (house template, principles section, zero tracker churn) | ✅ |
| PLAN-3 | Design review loop (rendered, inline-annotatable, propose-and-confirm) | ✅ |
| PLAN-4 | Feature planning (epic + dependency DAG + frozen worker charters) | ✅ |
| PLAN-5 | Plan umbrella (multi-epic features + verification epic) | ✅ |
| PLAN-6 | Kickoff routing (pool vs hosted-CI implementation paths) | ✅ |
| PLAN-7 | Completion monitor (non-blocking progress reporting) | ✅ |
| PLAN-8 | Backlog triage (one disposition per issue, marker-gated, foreign-PR pass) | ✅ |

### [Autonomous Implementation](autonomous-implementation.md)

| ID | Feature | Status |
|---|---|---|
| IMPL-1 | Gated supervisor invocation (claim-then-fire; idle ≈ 0 tokens; liveness floor) | ✅ |
| IMPL-2 | Supervisor pass (artifact-verified success; hollow detection) | ✅ |
| IMPL-3 | Cross-machine lease system (atomic, dual-TTL, presence-anchored, fenced) | ✅ |
| IMPL-4 | Isolated worker workspaces (guards; TTL pruning) | ✅ |
| IMPL-5 | WIP checkpointing & resume (checkpoint-and-exit; respawn caps; clean final PR) | ✅ |
| IMPL-6 | Multi-CLI worker backends (claude/codex/opencode/cursor behind one contract) | ✅ |
| IMPL-7 | Capability rungs (plan-time judgment frozen as data; per-backend tiermaps) | ✅ |
| IMPL-8 | Single-issue worker runtime (per-leaf scheduler dispatch) | ✅ |
| IMPL-9 | Janitors & self-healing (reapers, pruning, recovery, freeze-restore) | ✅ |
| IMPL-10 | Fair scheduling & liveness (rotation, epic-ready gate, starvation, battery policy) | ✅ |

### [Quality Gates & Merge](quality-and-merge.md)

| ID | Feature | Status |
|---|---|---|
| GATE-1 | Change finalization (docs-first, conventional commit, current branch) | ✅ |
| GATE-2 | Pre-proposal quality gate (local CI mirror + pass token + guard backstop) | ✅ |
| GATE-3 | Design-doc-first gate (feat changes are design-backed) | ✅ |
| GATE-4 | Risk-tiered merge approval (deterministic floor; LLM downgrade-only; bot-attributed) | ◐ classifier + lanes + shadow measurement shipped; platform enforcement + e2e validation open |
| GATE-5 | Merge automation (authorized, idempotent, raceable, redundant paths, review-gated repos) | ✅ |
| GATE-6 | Bounded recovery loops (ci-fix, conflict, stuck-PR, feedback — capped + escalate) | ✅ (open limitation: shared-file conflict self-heal) |
| GATE-7 | Required-checks calibration (per-repo derived contexts, one predicate) | ✅ |

### [Fleet Operations](fleet-operations.md)

| ID | Feature | Status |
|---|---|---|
| FLEET-1 | Containerized execution node (one-liner, multi-arch, state volume, monitor) | ✅ |
| FLEET-2 | Supervisor pool + watchdog (warm slots, revival, auth-outage handling) | ✅ |
| FLEET-3 | Node identity, presence & telemetry (beacons, TTLs, kind-blind, silence observer) | ✅ |
| FLEET-4 | Fleet status (read-only: nodes, spend, leases, flagged work) | ✅ |
| FLEET-5 | Doctor (self-healing health checks, node + engineer variants) | ✅ |
| FLEET-6 | Install / uninstall lifecycle (idempotent on, conservative off) | ✅ |
| FLEET-7 | Multi-machine collaboration (no coordinator; join/leave freely) | ✅ |
| FLEET-8 | Rig self-update (per-tick sync; freeze detection + restore) | ✅ |
| FLEET-9 | Org portability (zero-edit fork; dynamic App/slug resolution; CI portability guard) | ✅ |

### [Identity & Security](identity-and-security.md)

| ID | Feature | Status |
|---|---|---|
| IDEN-1 | Two-identity model (fleet bot vs human; separate reviewer identity) | ✅ |
| IDEN-2 | Scoped short-lived credentials (per-operation mints; nothing persisted) | ✅ |
| IDEN-3 | Action guards (one denylist, per-CLI fail-closed shims, gate backstops) | ✅ |
| IDEN-4 | Secret hygiene (scanning, secure storage, redaction, no secrets in images) | ✅ |

### [Governance & Compliance](governance-and-compliance.md)

| ID | Feature | Status |
|---|---|---|
| GOV-1 | Primitive manifest (classes L/M/D/V × repo/org/user tiers) | ✅ |
| GOV-2 | Drift detection & repair (mechanical auto-fix vs operator escalation) | ✅ |
| GOV-3 | Org policy (zero-touch exclusion list, synced fleet-wide) | ✅ |
| GOV-4 | Sanctioned per-repo customization (closed schema; extension conventions) | ✅ |
| GOV-5 | Control-signal taxonomy (governed label vocabulary; merge authorization) | ✅ |
| GOV-6 | Governance meta-rules (human-authored inputs; key-level merge; self-unlock) | ✅ |

### [Knowledge System](knowledge-system.md)

| ID | Feature | Status |
|---|---|---|
| KNOW-1 | Immutable design docs (template + lint; supersede-never-edit) | ✅ |
| KNOW-2 | Corpus discovery & routing (Relevant-when menu; Builds-on DAG) | ✅ |
| KNOW-3 | Living docs (techspec + prd rollups; one drift signal; sole writer; advisory) | ✅ |
| KNOW-4 | Incident registry (stable IDs, tags, graduation to principles) | ✅ |
| KNOW-5 | Design principles (stable citable rules; cited by every design) | ✅ |
| KNOW-6 | Northstar (taxonomy C1–C11; inversion test; metrics) | ✅ |
| KNOW-7 | Operator references (events catalog; setup guides; agent instruction files) | ✅ |

### [Observability & Spend](observability-and-spend.md)

| ID | Feature | Status |
|---|---|---|
| OBS-1 | Domain event stream (structured, cataloged, additive, bounded) | ✅ |
| OBS-2 | Token-spend records (pass/worker attribution per turn) | ✅ |
| OBS-3 | Pass reports (artifact-verified audit rows) | ✅ |
| OBS-4 | Health alarms & escalation (starvation, churn, failure runs, fleet silence) | ✅ |
| OBS-5 | Live monitoring surfaces (dashboard, host-native logs, status command) | ✅ |
| OBS-6 | Prompt logging & provenance (redacted capture; weekly review) | ◐ fleet migration pending |

### [Engineer Workbench](engineer-workbench.md)

| ID | Feature | Status |
|---|---|---|
| BENCH-1 | Workbench install (one command, own identity, pinned tools, doctor) | ✅ |
| BENCH-2 | Pre-push gate (the CLI-agnostic quality chokepoint) | ✅ |
| BENCH-3 | Multi-CLI authoring (auto-detect + wire; fail-closed guards; AGENTS.md) | ✅ |
| BENCH-4 | Rendered review surface (vendored, self-hosted, annotate→apply loop) | ✅ |
| BENCH-5 | Session handoff (/transfer — paste-ready prompt for a fresh instance/machine) | ✅ |

**67 features across 10 areas.** *(Cross-checked 2026-07-01 against all 536
harbormaster GitHub issues — 320 `[Feature]`, 5 epics, 4 plans, open backlog —
in addition to the docs; corrections from that sweep: FLEET-9 and BENCH-5 added,
/transfer description fixed — harbormaster's own living prd.md mis-describes it
— GATE-4 status refined, known GATE-6 limitation recorded.)*

## Not in the baseline (designed or absent in harbormaster)

Known gaps that Pyxis should treat as new work, not parity. The rewrite's
**specified** new requirements live in
[`prds/vnext-requirements.md`](vnext-requirements.md) (VNEXT-1..13,
operator-fed 2026-07-01: contract compilation & quality gate, structured DoD
schema, traceability, drift sentinel, evidence packages, repo intelligence,
routing intelligence, autonomy levels, expanded risk, policy engine, isolation
contract, pipeline-wide recovery, escalation protocol); the list below tracks
the remaining gaps and program-level work:

- **Feature definition stage** (raw idea → PRD) and per-feature PRD docs — pipeline stages 1–2 (product-vision.md); harbormaster starts at the design doc. *Stage-1 elicitation is now specified and built: [`docs/prds/feature-brainstorm/`](feature-brainstorm/effective.md) (the `/feature-brainstorm` skill + brainstorm-record contract); the synthesis half is `/prd-new`. Stage-3 elicitation — PLAN-1's pyxis successor — is likewise built: [`docs/prds/tech-design-brainstorm/`](tech-design-brainstorm/effective.md) (the `/tech-design-brainstorm` skill + design-brainstorm-record contract); the authoring half is `/tech-design-new`. Both classes are born reviewed: the unmanned reviewer pair — [`docs/prds/prd-review/`](prd-review/effective.md) and [`docs/prds/tech-design-review/`](tech-design-review/effective.md) — runs at every authoring moment, advisory until its ten-review checkpoints.*
- **E2E verification as a stage** (stage 10) — harbormaster has the verification-epic pattern only.
- **Periodic maintenance as a stage** (stage 11) — scattered janitors/doc-audit today.
- **Webhook push trigger** — designed-first backlog (harbormaster #921); poll-only today.
- **Fine-grained model routing** — backlog (harbormaster #1003); coarse rungs shipped (IMPL-7); now specified as **VNEXT-7** (backend scorecards & routing intelligence).
- **Evals as a verification primitive** (northstar C5 gap — bespoke in one repo).
- **Operations pillar** (deploy/SLOs/on-call — northstar C7: absent).
- **Two-mode restructure + org-installer program** (engine↔backbone carve, App-manifest-flow creator, org-tier installer) — designed, in flight; the zero-edit org *parameterization* half is already shipped (FLEET-9).
- **Multi-engineer team readiness** (per-engineer identity, review routing — northstar H2).
- **Auto-update channels / A/B cohorts / global telemetry** — new Pyxis requirements (product-vision.md), only seeds exist (self-update, upgrade --pull).
