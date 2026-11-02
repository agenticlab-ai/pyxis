# Defect fixture: unanchored decision — brainstorm record

| Field | Value |
|---|---|
| **In one line** | A global `--quiet` flag across CLI verbs *(defect fixture: D-2 lacks its anchor)*. |
| **Relevant-when** | walking the structure checklist against seeded defect classes *(fixture corpus)* |
| **Idea** | "Add a --quiet flag to every CLI verb." |
| **Operator** | scripted-defect (fixture) |
| **Date(s)** | 2026-07-09 |
| **Status** | gated: ready-for-prd |
| **Tracking** | — *(fixture)* |
| **Feeds** | — |
| **Prior records** | — |
| **Template version** | brainstorm-record/v1 |

## Orientation

| Source | Why loaded |
|---|---|
| docs/prds/README.md | index scan |

**Overlaps surfaced:** none

## Framings

- **Global flag** — one flag, every verb; thesis: uniform surface.
- **Per-verb opt-in** — only chatty verbs gain it; thesis: minimal change.

**Destination:** global flag — Operator: "uniform or nothing."

## Decisions

### D-1 — scope of the flag

- **Question**: global or per-verb?
- **Answer**: global.
- **Reasoning**: uniformity.
- **Anchor**: 2026-07-09 · t2
- **Status**: live

### D-2 — default verbosity *(seeded defect: no Anchor field)*

- **Question**: does quiet become the default?
- **Answer**: no, opt-in only.
- **Reasoning**: least surprise.
- **Status**: live

## Facts

none

## Fog

none

## Out of scope

none

## Candidate glossary terms

none

## Parked technical constraints

none

## Gate

| Field | Value |
|---|---|
| **Verdict** | ready-for-prd |
| **Date** | 2026-07-09 |
| **Operator** | scripted-defect (fixture) |
| **Covering source** | — |
| **Coverage check** | pass — (fixture; not walked) |
