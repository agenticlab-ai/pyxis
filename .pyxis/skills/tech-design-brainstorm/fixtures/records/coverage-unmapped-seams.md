# Coverage fixture: unmapped seams — design-brainstorm record

| Field | Value |
|---|---|
| **In one line** | The check-set contract behind the pre-proposal quality gate *(coverage fixture: the seam the panel opened was never mapped)*. |
| **Relevant-when** | walking the coverage checklist against a record whose rows are unmet while its gaps stay non-blocking *(fixture corpus)* |
| **Basis** | corpus:GATE-2 · corpus:GATE-7 |
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
| docs/prds/quality-and-merge.md | Basis match: GATE-2 local CI mirror, GATE-7 required-checks calibration |
| docs/architecture-principles.md | scoring substrate for the panel |

**Overlaps surfaced:** none covering — GATE-7 calibrates the host's required set, not what the local mirror reads.

## Design questions

- **DQ-1**: what tells the local gate which checks the code host will require?

## Panel log

- **DQ-1**: opens — fired — a contract between the desk gate and the code host is a new seam; first-idea anchoring risk is real

## Decisions

### D-1 — the check-set contract

- **Question**: which shape carries the required-check set to the local gate?
- **Answer**: a declared check manifest the gate reads, not a live host query.
- **Reasoning**: the gate must run offline and identically on every desk (AP-19 caution recorded on ALT-2).
- **Anchor**: 2026-07-10 · t3
- **Status**: live

## Alternatives

### ALT-1 — declared check manifest

- **Shape**: the repo declares its check set; the gate reads the declaration and runs what it names.
- **Trade-offs**: offline and deterministic; drifts from the host's real required set until re-declared.
- **Principles**: AP-6 honored (versioned structured artifact) · AP-19 honored (no network on the desk path).
- **Incidents**: none checked (registry reviewed 2026-07-10 — no gate/check-set entries).
- **Verdict**: chosen — the drift is bounded by the separate calibration pass (D-1).
- **From**: DQ-1 · panel

### ALT-2 — live host query

- **Shape**: no declaration; the gate asks the code host for the branch's required checks at run time.
- **Trade-offs**: never drifts; needs a network round trip and host credentials on every desk run.
- **Principles**: AP-15 honored (exact reads) · AP-19 strained (network on the hot path).
- **Incidents**: none checked (same review).
- **Verdict**: rejected — a desk gate that needs credentials to start is a gate people route around.
- **From**: DQ-1 · panel

## Seam map

none *(coverage fixture: the panel called DQ-1 an opened seam and the map was never filled in — the design's Serves and Scope have nothing to derive from)*

## Facts

- **F-1**: a local pre-proposal gate mirrors CI before a change is proposed — source: docs/prds/quality-and-merge.md (GATE-2)
- **F-2**: the host's required-check set is calibrated on its own cadence — source: docs/prds/quality-and-merge.md (GATE-7)

## Fog

- **G-1** (non-blocking — blocks: the design's own Q-n): how often the declared manifest is re-calibrated against the host

## Out of scope

- **O-1**: changing what the host itself requires — the calibration pass owns that set (from D-1)

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
| **Coverage check** | fail — Serves/Scope unmet: no S-n seam map to derive from. Zero blocking fog remains (G-1 is non-blocking), so what refuses the ready verdict here is the unmet row, not the gate condition. |
| **Amend route** | — |
