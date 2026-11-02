# Defect fixture: unscored alternative — design-brainstorm record

| Field | Value |
|---|---|
| **In one line** | A backlog read view *(defect fixture: ALT-2 has no Principles/Incidents fields)*. |
| **Relevant-when** | walking the structure checklist against seeded defect classes *(fixture corpus)* |
| **Basis** | corpus:FLEET-4 |
| **Authority** | scripted-defect (fixture) |
| **Date(s)** | 2026-07-10 |
| **Status** | gated: ready-for-design |
| **Tracking** | — *(fixture)* |
| **Feeds** | — |
| **Prior records** | — |
| **Template version** | design-brainstorm-record/v1 |

## Orientation

| Source | Why loaded |
|---|---|
| docs/prds/fleet-operations.md | basis (FLEET-4) |

**Overlaps surfaced:** none

## Design questions

- **DQ-1**: what read-model shape serves the backlog view?

## Panel log

- **DQ-1**: opens — fired — a new read contract

## Decisions

### D-1 — the shape

- **Question**: projection or query?
- **Answer**: query verb.
- **Reasoning**: freshness matters for a takeable-work view.
- **Anchor**: 2026-07-10 · t3
- **Status**: live

## Alternatives

### ALT-1 — query verb

- **Shape**: compute the view per request from exact reads.
- **Trade-offs**: always fresh; per-request compute.
- **Principles**: AP-15 honored · AP-19 strained (bounded: operator-invoked only).
- **Incidents**: none checked (registry reviewed 2026-07-10).
- **Verdict**: chosen.
- **From**: DQ-1 · panel

### ALT-2 — snapshot projection *(seeded defect: Principles and Incidents fields absent)*

- **Shape**: a maintained view artifact.
- **Trade-offs**: cheap reads; staleness on a work-selection surface.
- **Verdict**: rejected — staleness unacceptable here.
- **From**: DQ-1 · panel

## Seam map

- **S-1**: backlog read contract — opens — behind it: the view query — feeds: Serves

## Facts

none

## Fog

none

## Out of scope

none

## Candidate glossary terms

none

## Parked product gaps

none

## Gate

| Field | Value |
|---|---|
| **Verdict** | ready-for-design |
| **Date** | 2026-07-10 |
| **Authority** | scripted-defect (fixture) |
| **Covering design** | — |
| **Coverage check** | pass — (fixture; not walked) |
| **Amend route** | — |
