# tech-design-review — brainstorm record

| Field | Value |
|---|---|
| **In one line** | The design-class sibling of prd-review: the same self-contained verdict protocol pointed at technical designs (and their solution-shapes), with its own lens pack — Serves substance, principle/incident honesty, supersession integrity, feasibility against the shape. |
| **Relevant-when** | brainstorming or contracting the tech-design-review skill, its lens pack, or its subjects (design + solution-shape co-review, supersessions); checking prior rulings on the reviewer protocol shared with prd-review |
| **Idea** | "Two reviewers: prd-review and tech-design-review…" (operator, 2026-07-10 — the pair ruling D-4 split them; this session grills the design-side deltas only) |
| **Operator** | @mdarabi |
| **Date(s)** | 2026-07-10 |
| **Status** | gated: ready-for-prd |
| **Tracking** | — |
| **Feeds** | docs/prds/tech-design-review/ |
| **Prior records** | brainstorm:authoring-reviewers |
| **Template version** | brainstorm-record/v1 |

## Orientation

| Source | Why loaded |
|---|---|
| docs/brainstorms/authoring-reviewers.md (frozen) | the shared rulings this session inherits: D-1 (no-human world), D-2 (the verdict protocol + loop brakes), D-3 (escalation reaction), D-4 (two PRDs — this is the second); D-6's parked feasibility lens was explicitly noted for this session |
| docs/prds/prd-review/effective.md | the sibling contract this feature mirrors by pattern (not by artifact — D-4) |
| docs/tech-design/tech-design-conventions.md + tech-design-template/v1 | the review subject's class rules: binding elements, one-decision, Serves grammar, supersession model |
| docs/tech-design/solution-shape-v1.md (merged #127, same day) | **new ground**: designs now split into a frozen Decision + a non-frozen solution-shape, co-authored and **co-reviewed on one surface** — the reviewer's subject question changes shape |
| docs/tech-design/index.md + docgate lint-tech-designs | the mechanical floor for this class (header schema, Serves resolution/liveness, mandatory sections — checked; **substance unchecked**) |
| docs/architecture-principles.md + docs/observability/incidents.md | the honesty substrate: cited-vs-honored principles; incident matching |

**Overlaps surfaced:** prd-review (merged #128) covers the protocol pattern — inherited here by D-4's ruling, not coverage; `lint-tech-designs` checks Serves *resolution and liveness*, never whether the design's content actually implements the anchors — substance is open; the solution-shape co-review surface (#127) is a rendering capability, not a reviewer.

## Framings

Inherited by ruling — the identity question was settled at the pair level (prior D-2: the verdict-producing protocol; prior D-4: this feature is its second, separate binding). This session grills only the design-class deltas: subjects and moments, the lens pack, and the mirrored confirmations.

**Destination:** the tech-design-review feature — prd-review's protocol pointed at the design class, deltas ruled below.

## Decisions

### D-1 — subjects and moments

- **Question**: what does tech-design-review review, and when?
- **Answer**: the subject is the **pair** — the design and its solution-shape, reviewed together (degrading to the design alone where no shape exists); the moments are **every tech-design-new authoring event** — new designs (record-fed or cold) after `lint-tech-designs` passes, before the PR — **including supersessions as a first-class case** with their own checks: valid content carried forward, genuine contradiction (vs should-have-been Builds-on), no live dependent stranded. The mechanical floor (header schema, Serves resolution/liveness, mandatory sections) is excluded — the reviewer starts where the lint stops: substance.
- **Reasoning**: the #127 split makes the shape the concrete half a feasibility check needs; supersession is where the class changes — the amendment-door lesson applied (prior D-7's widening, mirrored).
- **Anchor**: 2026-07-10 · t2
- **Status**: live

### D-2 — the lens pack

- **Question**: what does tech-design-review judge the design+shape pair against?
- **Answer**: four lenses — **contract quality** (binding elements deliberate: Decision callouts binding, DINVs property-testable, A-rows runnable; one-decision held; §3.4/3.5 honesty; **partition honesty** per #127 — durable not leaked into the shape, concrete not frozen into the design) · **coherence** (DINV↔A↔§3 consistency; boundaries vs Decision; §4 alternatives genuinely distinct and verdicted — strawman detection; record-fed ALT ledger carried faithfully) · **upstream alignment** (**Serves substance** — each anchor actually implemented, where the lint only checks existence/liveness; PRD fidelity; **principles honesty** — cited-vs-honored, no silent omissions, reasoned deviations; **incidents matched**; D-1's supersession integrity; record fidelity) · **feasibility & completeness critic** (the parked lens lands: buildable within the PRD's appetite; the shape implementable and Decision-consistent; testing strategy per the template prompt — dependency bins, one-adapter rule, seam economy; risks owned with verification; the missing-concerns sweep). Calibration mirrors the sibling — coherence+alignment blocking-default, contract-quality advisory unless a binding element goes untestable, critic blocking only when acceptance-load-bearing — as its own versioned data table.
- **Reasoning**: same count, design-class substance; the shape gives feasibility something concrete; Serves-substance and principles-honesty are the two checks nothing mechanical can do.
- **Anchor**: 2026-07-10 · t3
- **Status**: live

### D-3 — the mirrors, confirmed with deltas

- **Question**: audience, bet, perimeter, appetite — inherited from the pair session with what design-class deltas?
- **Answer**: confirmed by citation to brainstorm:authoring-reviewers#D-8..#D-11, with: P3 concretely includes the Planner (plans decompose from designs, so defects propagate into charters); the bet identical (M1 ≥ 80% acceptance · M2 overlay residual → 0 · checkpoint ten reviews with supersessions counting · same guardrails); the perimeter adds never-edit-the-shape and never-relitigate a predecessor's frozen Decision during supersession review (integrity yes, content no), and rules **plans out for both siblings** (plangate's ground); appetite small with **one** additive guide edit (tech-design-new carries both moments) and its artifact design likely Builds-on prd-review's.
- **Reasoning**: the mirrors hold because the protocol is shared by pattern (prior D-4); only class-specific edges needed rulings.
- **Anchor**: 2026-07-10 · t4
- **Status**: live

## Facts

- **F-1**: `lint-tech-designs` enforces header schema, typed-Serves grammar, anchor resolution/liveness, and mandatory-section presence — it never checks whether the design's substance implements a Serves anchor, honors a cited principle, or genuinely differs across §4 alternatives — source: docs/prds/docgate/effective.md (FR-11/FR-20 lineage)
- **F-2**: as of #127, a technical design may ship with a co-reviewed, non-frozen solution-shape whose lifecycle is authoritative-until-code — source: docs/tech-design/solution-shape-v1.md
- **F-3**: contradiction between live designs requires supersession, and the index computes liveness from declared edges — the class's own integrity rules a reviewer can substance-check — source: docs/tech-design/tech-design-conventions.md §2

## Fog

none

## Out of scope

- **O-1**: editing the reviewed design or its solution-shape (from D-3)
- **O-2**: re-judging recorded rulings — including a predecessor's frozen Decision content during supersession review; integrity is judged, content never relitigated (from D-3)
- **O-3**: signing approvals — the verdict is data beside the authority's acts (from D-3)
- **O-4**: re-reporting the `lint-tech-designs` floor (from D-1)
- **O-5**: reviewing PRDs (the sibling's class) or plans (plangate's ground — neither sibling's) (from D-3)
- **O-6**: tracker writes by the reviewer itself — the surrounding flow reacts to verdicts (inherited, prior D-3)

## Candidate glossary terms

none

## Parked technical constraints

- **P-1**: the finding/report/brief schemas, calibration-table format, and artifact homes for this class — pinned by this feature's tech design, which likely Builds-on prd-review's artifact design rather than redefining it (from D-3)

## Gate

| Field | Value |
|---|---|
| **Verdict** | ready-for-prd |
| **Date** | 2026-07-10 |
| **Operator** | @mdarabi |
| **Covering source** | — |
| **Coverage check** | pass — all six inputs covered (§2.1←pair D-1/D-2 via Prior records + D-1 · §2.2←D-3 · §2.3←D-3 · §3.1←D-3 · §3.4←O-1..6 · appetite←D-3), zero blocking fog |
