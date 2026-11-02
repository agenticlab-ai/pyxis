# Defect fixture: fired panel, one alternative — design-brainstorm record

| Field | Value |
|---|---|
| **In one line** | An incident-export feed *(defect fixture: panel fired, only ALT-1 exists)*. |
| **Relevant-when** | walking the structure checklist against seeded defect classes *(fixture corpus)* |
| **Basis** | corpus:KNOW-4 |
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
| docs/prds/knowledge-system.md | basis (KNOW-4) |

**Overlaps surfaced:** none

## Design questions

- **DQ-1**: what contract exports incident entries to external consumers?

## Panel log

- **DQ-1**: opens — fired — a new export contract is a new seam

## Decisions

### D-1 — export shape

- **Question**: which shape carries the export?
- **Answer**: the file projection.
- **Reasoning**: consistent with sibling read-models.
- **Anchor**: 2026-07-10 · t3
- **Status**: live

## Alternatives

### ALT-1 — file projection *(seeded defect: a fired panel with a single alternative — DINV-7)*

- **Shape**: a maintained export artifact consumers read.
- **Trade-offs**: cheap reads; lag between refreshes.
- **Principles**: AP-6 honored · AP-19 honored.
- **Incidents**: none checked (registry reviewed 2026-07-10).
- **Verdict**: chosen — unopposed.
- **From**: DQ-1 · panel

## Seam map

- **S-1**: incident-export contract — opens — behind it: export schema — feeds: Serves

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
