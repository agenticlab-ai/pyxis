# Defect fixture: missing panel-log line — design-brainstorm record

| Field | Value |
|---|---|
| **In one line** | A spend-rollup artifact *(defect fixture: DQ-1 has no panel-log line)*. |
| **Relevant-when** | walking the structure checklist against seeded defect classes *(fixture corpus)* |
| **Basis** | corpus:OBS-2 |
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
| docs/prds/observability-and-spend.md | basis (OBS-2) |

**Overlaps surfaced:** none

## Design questions

- **DQ-1**: what granularity does the rollup aggregate to?

## Panel log

none *(seeded defect: DQ-1 exists with no line — DINV-7)*

## Decisions

### D-1 — granularity

- **Question**: per-pass, per-worker, or per-feature rollup?
- **Answer**: per-feature, derived from pass attribution.
- **Reasoning**: matches the cost-per-shipped-feature metric.
- **Anchor**: 2026-07-10 · t2
- **Status**: live

## Alternatives

none

## Seam map

- **S-1**: spend read surface — extends — behind it: aggregation — feeds: Scope

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
