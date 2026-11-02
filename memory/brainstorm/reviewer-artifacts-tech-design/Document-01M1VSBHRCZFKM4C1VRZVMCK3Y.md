# Reviewer artifacts — design-brainstorm record

| Field | Value |
|---|---|
| **In one line** | The artifact model behind both reviewers — finding/report/brief schemas, the calibration-table format, and their homes — one design direction for the contracts prd-review and tech-design-review both consume. |
| **Relevant-when** | designing or changing the reviewer artifact schemas, their homes, or the calibration-table format; checking prior design rulings on review-artifact shape or the markdown-vs-JSON source question |
| **Basis** | prd:prd-review · prd:tech-design-review |
| **Authority** | @mdarabi |
| **Date(s)** | 2026-07-10 |
| **Status** | gated: ready-for-design |
| **Tracking** | — |
| **Feeds** | docs/tech-design/prd-review-artifacts.md |
| **Prior records** | brainstorm:authoring-reviewers · brainstorm:tech-design-review |
| **Template version** | design-brainstorm-record/v1 |

## Orientation

| Source | Why loaded |
|---|---|
| docs/prds/prd-review/effective.md | the schema requirements: FR-5 (finding elements), FR-6 (calibration as versioned data, row citations), FR-7 (report = evidence package riding the PR), FR-11 (brief), FR-14 (measurement fields, append-only verdict history); Q1 defers schemas/homes here |
| docs/prds/tech-design-review/effective.md | the sibling's Q1: whether its design Builds-on this one or shares verbatim; its class deltas (supersession findings, pair subjects) the schemas must carry |
| docs/tech-design/brainstorm-record-model.md + design-brainstorm-record-model.md | the artifact-design precedents: typed-markdown classes, checklist enforcement, freeze semantics, shared-home + suffix naming, staged mechanization |
| docs/tech-design/solution-shape-v1.md | the newest artifact-class design — its home/config-key pattern and non-frozen lifecycle vocabulary |
| docs/tech-design/docgate-v1.md (defects-as-data) | the house pattern for machine-consumed findings: schema-versioned JSON lines, closed grammar |
| docs/tech-design/tech-design-conventions.md | where review reports would sit relative to frozen designs (they are provenance, not governed docs) |

**Overlaps surfaced:** no live design decides review-artifact schemas; the two record-model designs are the nearest kin (Builds-on candidates); docgate's `--defects-json` is the machine-findings precedent, not coverage.

## Design questions

- **DQ-1**: what is the artifact source format — typed markdown (the record-class pattern), JSON-first (the defects-as-data pattern), or a two-faces projection?
- **DQ-2**: where do review artifacts live, and how do they ride the subject's introducing PR?
- **DQ-3**: one artifact design serving both reviewers, or two (the sibling PRDs' twin Q1s)?
- **DQ-4**: the calibration table — format and home, per class.

## Panel log

- **DQ-1**: opens — fired — a new artifact-class family's contract is a new seam; the source-format choice shapes every consumer
- **DQ-2**: extends — skipped — homes follow the established artifact-home pattern (flat docs/ directory + naming rule); placement is convention, the naming needs one ruling inside DQ-2
- **DQ-3**: extends — skipped — the PRDs already lean one-design-plus-Builds-on (both Q1s name it); confirming is a ruling, not a divergence
- **DQ-4**: extends — skipped — calibration format follows the source-format ruling of DQ-1; per-class homes follow DQ-2

## Decisions

### D-1 — the source format: typed markdown

- **Question**: which of the three paneled shapes carries the reviewer artifacts (DQ-1)?
- **Answer**: ALT-1 — typed markdown, the record-class pattern.
- **Reasoning**: the bootstrap's human surface is now (the authority reads reports in PRs during the overlay era); zero new tooling; the pattern proved twice this week. The recommendation's engine-era graduation to JSON-native (ALT-2's strengths, when a mechanical consumer exists) rides as a parked deferral, not a binding.
- **Anchor**: 2026-07-10 · t2
- **Status**: live

### D-2 — the homes (DQ-2, and DQ-4 by composition)

- **Question**: where do reports, briefs, and calibration tables live?
- **Answer**: reports in `docs/reviews/`, flat — **one report artifact per subject per moment**, rounds/dispositions/verdict-history appending inside it, named by the subject's identity (`prd-review-000.md`, `prd-review-001.md`, `reviewer-artifacts.md`), riding the subject's introducing PR; **briefs embedded in the report** as its escalation section (the minting flow copies from it); **calibration tables with their skill** (`.pyxis/skills/<reviewer>/calibration.md`), one per class, versioned reviewable data. DQ-4 thereby composes: calibration format = markdown table (D-1) at the skill home (here).
- **Reasoning**: one artifact per review keeps FR-7's append-only history a document lifecycle rather than a file sprawl; embedding the brief keeps the escalation payload inside the evidence package; skill-adjacent calibration keeps the dial on the vendor-neutral surface its consumer reads.
- **Anchor**: 2026-07-10 · t3
- **Status**: live

### D-3 — two artifact designs, one per reviewer

- **Question**: one artifact design serving both PRDs, or two (DQ-3)?
- **Answer**: two separate designs — each reviewer feature owns its artifact contract.
- **Reasoning**: the authority's ruling (recommendation was one; chosen: two) — the two-PRD independent-evolution posture applied at the design layer. Consequences: this record **feeds the first** (prd-review's artifact design); the sibling's design authors via tech-design-new's recordless path — its direction is settled by this session's D-1/D-2, cited as prior rulings and Builds-on lineage — and owns its class-specific schema content (supersession-finding fields, pair-subject references) in its own document.
- **Anchor**: 2026-07-10 · t4
- **Status**: live

## Alternatives

### ALT-1 — typed markdown, the record-class pattern

- **Shape**: findings/report/brief as one markdown artifact per review with typed sections and dense anchors (`FND-n`), checklist-walked; calibration a markdown table, versioned.
- **Trade-offs**: human-readable in PRs today (the operator reviews PRs during bootstrap); zero new tooling; proven twice this week; measurements computable by inspection — but machine consumption stays parse-by-convention until a graduation lint, and dispositions are prose-adjacent.
- **Principles**: AP-6 strained-but-staged (typed sections are contractual-lite; graduation mechanizes) · AP-10 honored-staged (checklists now, lint later — the record classes' exact posture) · AP-23 honored (one file per review).
- **Incidents**: none checked (registry reviewed 2026-07-10 — no artifact-format entries).
- **Verdict**: chosen (D-1) — the bootstrap's human surface and the proven pattern win; mechanization staged.
- **From**: DQ-1 · panel

### ALT-2 — JSON-first, the defects-as-data pattern

- **Shape**: findings/report/brief as schema-versioned JSON (docgate's `--defects-json` lineage); human faces rendered on demand (overlay/PR render), never committed.
- **Trade-offs**: AP-6 fully honored from day one; dispositions and measurements natively machine-computable; matches the D-1 no-human world — but PR review during bootstrap reads raw JSON (the operator's overlay era is *now*), diff review is hostile, and hand-inspection (J5 audit) needs tooling immediately rather than eventually.
- **Principles**: AP-6 honored fully · AP-5 honored · AP-25 neutral — but the bootstrap human surface pays.
- **Incidents**: none checked (same review).
- **Verdict**: rejected for now — the right shape for the engine era, when a mechanical consumer and its tooling exist; parked as the graduation path (G-1).
- **From**: DQ-1 · panel

### ALT-3 — two-faces projection (JSON source, committed markdown face)

- **Shape**: JSON source + a generated markdown face committed beside it, byte-verified — the effective.md / index.md house projection pattern.
- **Trade-offs**: both consumers first-class — but "generated, never authored" demands a fold tool with sole-writer + verify discipline (AP-10's hard reading); building reviewer-artifact fold tooling now front-loads the engine era into a bootstrap skill.
- **Principles**: AP-6 honored · AP-10 honored-expensive (the tool must exist before the artifact does — the index's own born-with-the-tool rule cuts against hand-generated interim faces).
- **Incidents**: none checked (same review).
- **Verdict**: rejected — the fold tool must precede the artifact (born-with-the-tool), which front-loads engine tooling into a bootstrap skill.
- **From**: DQ-1 · panel

## Seam map

- **S-1**: the reviewer-artifact contract (finding/report/brief schemas + calibration format) — opens — behind it: every reviewer's output and every consumer's input — feeds: the design's Serves (both PRDs' FR-5/6/7/11/14) and Scope
- **S-2**: the artifact home + naming — extends — behind it: the established flat-docs-directory + suffix conventions — feeds: Scope

## Facts

- **F-1**: docgate already emits machine findings as one JSON line per defect (`--defects-json`), schema-versioned — the in-house precedent for machine-native findings — source: docs/prds/docgate/effective.md (FR-2)
- **F-2**: both record classes shipped as typed markdown with checklist enforcement and staged mechanization, and both proved workable within days — source: the two record-model designs
- **F-3**: reports must ride the subject's introducing PR as its evidence package, with append-only verdict history and post-approve overlay corrections — source: prd:prd-review/FR-7 · FR-14

## Fog

- **G-1** (non-blocking — blocks: the engine-era artifact-format design, not this one): the JSON-native graduation (ALT-2's shape) when a mechanical consumer of findings/measurements exists — same staging as record lint

## Out of scope

- **O-1**: reviewer-artifact fold tooling (the two-faces projection's precondition) — rejected with ALT-3; revisits only with the engine-era graduation (G-1) (from D-1)
- **O-2**: the tech-design-review class's schema specifics — owned by its own design per the two-designs ruling (from D-3)

## Candidate glossary terms

none

## Parked product gaps

none

## Gate

| Field | Value |
|---|---|
| **Verdict** | ready-for-design |
| **Date** | 2026-07-10 |
| **Authority** | @mdarabi |
| **Covering design** | — |
| **Coverage check** | pass — core four covered (§2←D-1, DQ-1 panel-logged · §4←ALT-1..3 all verdicted · Serves/Scope←S-1..2 + Basis · Boundaries←O-1..2), zero blocking fog (G-1 non-blocking) |
| **Amend route** | — |
