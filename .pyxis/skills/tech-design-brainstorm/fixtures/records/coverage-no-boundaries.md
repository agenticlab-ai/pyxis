# Coverage fixture: no boundaries — design-brainstorm record

| Field | Value |
|---|---|
| **In one line** | Where a language profile pins the tool versions it installs *(coverage fixture: the session ruled nothing out)*. |
| **Relevant-when** | walking the coverage checklist against a record whose rows are unmet while its gaps stay non-blocking *(fixture corpus)* |
| **Basis** | corpus:SCAF-3 |
| **Authority** | scripted-coverage (fixture) |
| **Date(s)** | 2026-07-10 |
| **Status** | gated: needs-investigation |
| **Tracking** | — *(fixture)* |
| **Feeds** | — |
| **Prior records** | — |
| **Template version** | design-brainstorm-record/v1 |

## Orientation

| Source | Why loaded |
|---|---|
| docs/prds/scaffolding-and-onboarding.md | basis (SCAF-3 language profiles) |

**Overlaps surfaced:** none

## Design questions

- **DQ-1**: where does a language profile pin the tool versions it installs?

## Panel log

- **DQ-1**: extends — skipped — the profile file already exists; a version pin is a field on it, not a new shape

## Decisions

### D-1 — where the pins live

- **Question**: on the profile itself, or in a lockfile beside it?
- **Answer**: on the profile — one file, one closed schema.
- **Reasoning**: a second file describing the same profile invites the two drifting apart.
- **Anchor**: 2026-07-10 · t2
- **Status**: live

## Alternatives

none

## Seam map

- **S-1**: the language-profile schema — extends — behind it: the version-pin fields — feeds: the design's Scope

## Facts

- **F-1**: language profiles already carry per-language tooling — source: docs/prds/scaffolding-and-onboarding.md (SCAF-3)

## Fog

- **G-1** (non-blocking — blocks: the design's own Q-n): whether a pin may name a version range or must name an exact version

## Out of scope

none *(coverage fixture: nothing was ruled out — §3.2's boundaries have nothing to seed from)*

## Candidate glossary terms

none

## Parked product gaps

none

## Gate

| Field | Value |
|---|---|
| **Verdict** | needs-investigation |
| **Date** | 2026-07-10 |
| **Authority** | scripted-coverage (fixture) |
| **Covering design** | — |
| **Coverage check** | fail — Boundaries unmet: no O-n rulings. Zero blocking fog remains (G-1 is non-blocking), so what refuses the ready verdict here is the unmet row, not the gate condition. |
| **Amend route** | — |
