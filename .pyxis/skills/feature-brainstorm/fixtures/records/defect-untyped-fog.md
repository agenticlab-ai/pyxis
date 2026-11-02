# Defect fixture: untyped fog — brainstorm record

| Field | Value |
|---|---|
| **In one line** | Config snapshots as a drift tripwire *(defect fixture: G-1 untyped)*. |
| **Relevant-when** | walking the structure checklist against seeded defect classes *(fixture corpus)* |
| **Idea** | "Snapshot the config on every run so drift is diffable." |
| **Operator** | scripted-defect (fixture) |
| **Date(s)** | 2026-07-09 |
| **Status** | gated: needs-investigation |
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

- **Audit trail** — snapshots as history; thesis: forensics.
- **Drift tripwire** — snapshots as comparison input; thesis: detection.

**Destination:** drift tripwire — Operator: "detection is the point."

## Decisions

### D-1 — what the snapshot is for

- **Question**: history or detection?
- **Answer**: detection.
- **Reasoning**: forensics is a byproduct.
- **Anchor**: 2026-07-09 · t2
- **Status**: live

## Facts

none

## Fog

- **G-1**: what "the config" even includes across install tiers *(seeded defect: no type, no blocks-target)*

## Out of scope

none

## Candidate glossary terms

none

## Parked technical constraints

none

## Gate

| Field | Value |
|---|---|
| **Verdict** | needs-investigation |
| **Date** | 2026-07-09 |
| **Operator** | scripted-defect (fixture) |
| **Covering source** | — |
| **Coverage check** | — |
