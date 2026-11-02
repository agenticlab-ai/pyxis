# Fleet dashboard data feed — design-brainstorm record

| Field | Value |
|---|---|
| **In one line** | The feed contract behind the live fleet dashboard — a schema-versioned snapshot artifact with an on-demand refresh verb, chosen over stream-tailing and per-view queries. |
| **Relevant-when** | designing or changing the fleet dashboard's data feed, monitoring read-models, or snapshot/refresh contracts; checking prior rulings on feed shape or real-time scope *(fixture — the golden scripted run, not a real session)* |
| **Basis** | corpus:OBS-5 · corpus:OBS-1 |
| **Authority** | scripted-golden (fixture) |
| **Date(s)** | 2026-07-10 |
| **Status** | gated: ready-for-design |
| **Tracking** | — *(fixture)* |
| **Feeds** | — |
| **Prior records** | — |
| **Template version** | design-brainstorm-record/v1 |

## Orientation

| Source | Why loaded |
|---|---|
| docs/prds/observability-and-spend.md | Basis match: OBS-1 event stream, OBS-4 alarms, OBS-5 monitoring surfaces |
| docs/prds/fleet-operations.md | FLEET-4 fleet-status read surface (adjacent) |
| docs/tech-design/index.md | Serves scan — no live design serves a dashboard feed |
| docs/architecture-principles.md | scoring substrate (AP-5/6/15/19 relevant) |
| docs/observability/incidents.md | incident matching for the panel |
| docs/wiki/glossary.md | term check (feed / stream / snapshot) |

**Overlaps surfaced:** none covering — OBS-5's surfaces are the consumers, not the contract; FLEET-4 is a point-in-time read, noted as adjacent.

## Design questions

- **DQ-1**: what feed contract serves the live fleet dashboard?
- **DQ-2**: where does the feed's snapshot artifact live?

## Panel log

- **DQ-1**: opens — fired — a new read-model contract between the engine's observability data and a display consumer is a new seam; first-idea anchoring risk is real
- **DQ-2**: extends — skipped — artifact placement follows the existing observability-artifact conventions; nothing left to diverge on

## Decisions

### D-1 — the feed direction

- **Question**: which of the three paneled shapes carries the dashboard feed?
- **Answer**: the snapshot-file projection, hybridized with the query option's refresh verb — "the snapshot file, but take the on-demand refresh verb from the query option."
- **Reasoning**: cheap-by-default reads with a freshness escape hatch; display surface tolerates projection lag (AP-15 caution recorded on ALT-1).
- **Anchor**: 2026-07-10 · t4
- **Status**: live

### D-2 — snapshot artifact home

- **Question**: where does the snapshot live?
- **Answer**: authority delegated ("you decide"); recommendation adopted: alongside the existing observability artifacts, exact path pinned at design authoring.
- **Reasoning**: placement is convention-following (DQ-2 skipped); no new home warranted.
- **Anchor**: 2026-07-10 · t5
- **Status**: live

### D-3 — real-time scope

- **Question**: does the feed carry real-time/urgent signals?
- **Answer**: no — the alarm surface owns urgency; the feed is a periodic/refreshable snapshot only.
- **Reasoning**: mixing urgency into a display feed duplicates OBS-4's job (F-2).
- **Anchor**: 2026-07-10 · t6
- **Status**: live

## Alternatives

### ALT-1 — snapshot-file projection

- **Shape**: the engine maintains a schema-versioned snapshot artifact; the dashboard reads it; a refresh verb regenerates on demand.
- **Trade-offs**: cheapest reads, idle-zero writes; projection can lag between refreshes.
- **Principles**: AP-6 honored (versioned structured artifact) · AP-19 honored (write-on-change, idle ≈ 0) · AP-15 strained — lag tolerated only because no destructive decision reads this surface; the caution rides into the design.
- **Incidents**: none checked (registry reviewed 2026-07-10 — no feed/display entries; the stale-read class concerns decision paths, carried as the AP-15 caution).
- **Verdict**: chosen — hybridized with ALT-3's refresh verb (D-1).
- **From**: DQ-1 · panel

### ALT-2 — event-stream tail

- **Shape**: the dashboard tails the domain event stream (OBS-1) and derives fleet state itself.
- **Trade-offs**: perfect lineage; every consumer re-implements state derivation; an always-on consumer.
- **Principles**: AP-5 honored (pure event lineage) · AP-6 honored · AP-19 strained (standing consumer burns idle budget).
- **Incidents**: none checked (same review).
- **Verdict**: rejected — state derivation belongs to the engine, not to each display consumer.
- **From**: DQ-1 · panel

### ALT-3 — on-demand query verb

- **Shape**: no artifact; a verb computes the fleet view per request.
- **Trade-offs**: always fresh; per-view compute on every open; nothing greppable between requests.
- **Principles**: AP-15 honored (exact reads) · AP-19 strained (hot-path compute per view).
- **Incidents**: none checked (same review).
- **Verdict**: hybridized into ALT-1 — its verb survives as the snapshot's on-demand regeneration (D-1).
- **From**: DQ-1 · panel

## Seam map

- **S-1**: the fleet-feed contract — opens — behind it: the snapshot schema + the refresh verb — feeds: the design's Serves (corpus:OBS-5) and Scope
- **S-2**: the observability artifact/read surface — extends — behind it: placement + retention conventions — feeds: the design's Scope

## Facts

- **F-1**: a structured, cataloged domain event stream exists as the derivation source — source: docs/prds/observability-and-spend.md (OBS-1)
- **F-2**: a real-time health-alarm surface already owns urgency — source: docs/prds/observability-and-spend.md (OBS-4)
- **F-3**: a point-in-time fleet-status read surface exists (nodes, spend, leases) — source: docs/prds/fleet-operations.md (FLEET-4)

## Fog

- **G-1** (non-blocking — blocks: the design's own Q-n): the refresh cadence default (and whether it is config or constant)

## Out of scope

- **O-1**: real-time or push delivery in the feed — the alarm surface owns urgency (from D-3)

## Candidate glossary terms

none

## Parked product gaps

none

## Gate

| Field | Value |
|---|---|
| **Verdict** | ready-for-design |
| **Date** | 2026-07-10 |
| **Authority** | scripted-golden (fixture) |
| **Covering design** | — |
| **Coverage check** | pass — core four covered (§2←D-1, DQ-1 panel-logged · §4←ALT-1..3 all verdicted · Serves/Scope←S-1..2 + Basis · Boundaries←O-1), zero blocking fog (G-1 non-blocking) |
| **Amend route** | — |
