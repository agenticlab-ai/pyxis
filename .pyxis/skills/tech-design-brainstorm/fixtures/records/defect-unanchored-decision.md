# Defect fixture: unanchored decision — design-brainstorm record

| Field | Value |
|---|---|
| **In one line** | A quiet flag on gate verbs *(defect fixture: D-2 lacks its anchor)*. |
| **Relevant-when** | walking the structure checklist against seeded defect classes *(fixture corpus)* |
| **Basis** | prd:docgate |
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
| docs/prds/docgate/effective.md | basis |

**Overlaps surfaced:** none

## Design questions

- **DQ-1**: how does `--quiet` compose with `--defects-json`?

## Panel log

- **DQ-1**: extends — skipped — existing verb/output surface; shape settled

## Decisions

### D-1 — composition rule

- **Question**: what does quiet silence?
- **Answer**: human prose only; the JSON face untouched.
- **Reasoning**: machine consumers must never lose data to a UX flag.
- **Anchor**: 2026-07-10 · t2
- **Status**: live

### D-2 — default verbosity *(seeded defect: no Anchor field)*

- **Question**: is quiet ever the default?
- **Answer**: no — opt-in only.
- **Reasoning**: least surprise.
- **Status**: live

## Alternatives

none

## Seam map

- **S-1**: docgate verb/output surface — extends — behind it: flag plumbing — feeds: Scope

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
