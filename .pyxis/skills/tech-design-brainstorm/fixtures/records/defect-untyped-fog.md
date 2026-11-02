# Defect fixture: untyped fog — design-brainstorm record

| Field | Value |
|---|---|
| **In one line** | Config snapshot diffing *(defect fixture: G-1 carries no type)*. |
| **Relevant-when** | walking the structure checklist against seeded defect classes *(fixture corpus)* |
| **Basis** | corpus:GOV-2 |
| **Authority** | scripted-defect (fixture) |
| **Date(s)** | 2026-07-10 |
| **Status** | gated: needs-investigation |
| **Tracking** | — *(fixture)* |
| **Feeds** | — |
| **Prior records** | — |
| **Template version** | design-brainstorm-record/v1 |

## Orientation

| Source | Why loaded |
|---|---|
| docs/prds/governance-and-compliance.md | basis (GOV-2) |

**Overlaps surfaced:** none

## Design questions

- **DQ-1**: what does a config snapshot include across install tiers?

## Panel log

- **DQ-1**: extends — skipped — snapshot content is enumeration, not shape

## Decisions

### D-1 — purpose

- **Question**: history or drift detection?
- **Answer**: drift detection; history is a byproduct.
- **Reasoning**: matches the drift-repair loop.
- **Anchor**: 2026-07-10 · t2
- **Status**: live

## Alternatives

none

## Seam map

- **S-1**: config read surface — extends — behind it: enumeration — feeds: Scope

## Facts

none

## Fog

- **G-1**: which tiers' config is in scope *(seeded defect: no type, no blocks-target)*

## Out of scope

none

## Candidate glossary terms

none

## Parked product gaps

none

## Gate

| Field | Value |
|---|---|
| **Verdict** | needs-investigation |
| **Date** | 2026-07-10 |
| **Authority** | scripted-defect (fixture) |
| **Covering design** | — |
| **Coverage check** | — |
| **Amend route** | — |
