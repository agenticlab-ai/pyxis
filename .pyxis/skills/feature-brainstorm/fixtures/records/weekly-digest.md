# Weekly digest — brainstorm record

| Field | Value |
|---|---|
| **In one line** | A factory-generated weekly digest artifact — what merged, what's blocked, what escalated — replacing the operator's dashboard trawl. |
| **Relevant-when** | brainstorming or contracting operator digests, periodic reporting, or observability rollups; checking prior rulings on digest transport or real-time scope *(fixture — the golden scripted run, not a real session)* |
| **Idea** | "The factory should give me a weekly digest: what merged, what's blocked, what escalated — so I don't have to trawl dashboards to know how my installation is doing." |
| **Operator** | scripted-golden (fixture) |
| **Date(s)** | 2026-07-09 |
| **Status** | gated: ready-for-prd |
| **Tracking** | — *(fixture)* |
| **Feeds** | — |
| **Prior records** | — |
| **Template version** | brainstorm-record/v1 |

## Orientation

| Source | Why loaded |
|---|---|
| docs/product-vision.md | §5 capability map (Observability & Spend) · §10 progress metrics |
| docs/prds/README.md | index scan for observability/status features |
| docs/prds/observability-and-spend.md | Relevant-when match: OBS-1 event stream, OBS-4 alarms, OBS-5 monitoring surfaces |
| docs/prds/fleet-operations.md | FLEET-4 fleet status (point-in-time read surface) |
| docs/domain-model.md | §3.7 observability entities |
| docs/wiki/glossary.md | term check (digest / report / status) |

**Overlaps surfaced:** OBS-5 (live monitoring: real-time surfaces, not a periodic rollup) · OBS-4 (health alarms: threshold push, not a summary) · FLEET-4 (fleet status: point-in-time) — adjacent, none covering a periodic operator digest.

## Framings

- **Scheduled status command** — a `digest` verb the operator runs; thesis: a pull surface, the operator still initiates.
- **Generated report artifact** — the factory periodically produces a digest artifact; thesis: the factory reports to the operator.
- **Notification rollup** — the event stream batched into periodic notifications; thesis: delivery-first, transport built in.

**Destination:** the *generated report artifact*, with the rollup's cadence flexibility — the factory produces a digest on a configurable cadence (weekly by default, not weekly by definition); delivery/transport is not part of the artifact's definition. — Operator: "weekly by default, not weekly by definition."

## Decisions

### D-1 — which framing is the feature

- **Question**: is the digest a command you run, an artifact the factory produces, or a notification stream?
- **Answer**: the generated artifact, cadence configurable (weekly default) — hybrid of framings B + C's cadence point.
- **Reasoning**: the factory should report to the operator, not wait to be asked; transport is separable.
- **Anchor**: 2026-07-09 · t3
- **Status**: live

### D-2 — who it serves

- **Question**: primary segment and anti-segment?
- **Answer**: solo or near-solo operators; explicitly not large teams with real dashboards and on-call rotations.
- **Reasoning**: the digest replaces surface-polling for a person, not a NOC.
- **Anchor**: 2026-07-09 · t4
- **Status**: live

### D-3 — the job

- **Question**: what moment is this for, concretely?
- **Answer**: "Monday morning: know what shipped, what's stuck, what needs me — without opening anything else."
- **Reasoning**: one artifact read replaces trawling dashboards/status surfaces.
- **Anchor**: 2026-07-09 · t5
- **Status**: live

### D-4 — the bet and its observable success

- **Question**: what does success look like, falsifiably?
- **Answer**: if the digest is good, the operator stops polling surfaces — reading it *is* the weekly check.
- **Reasoning**: measurable as reduced surface-visits / operator-confirmed replacement.
- **Anchor**: 2026-07-09 · t6
- **Status**: live

### D-5 — delivery and transport in v1

- **Question**: does v1 include delivery (email, push, chat)?
- **Answer**: no — land it as an artifact the operator can open; transport later.
- **Reasoning**: transport multiplies scope and infra; the artifact is the value.
- **Anchor**: 2026-07-09 · t7
- **Status**: live

### D-6 — relation to real-time alerting

- **Question**: does the digest carry urgent/real-time signals?
- **Answer**: no — real-time is the health-alarm surface's job (F-1); the digest is a periodic rollup only.
- **Reasoning**: mixing urgency into a weekly artifact dulls both.
- **Anchor**: 2026-07-09 · t8
- **Status**: live

### D-7 — appetite

- **Question**: what is this worth?
- **Answer**: small — a generator and a template; no new service, no new infra.
- **Reasoning**: reads existing streams (F-2); assembly, not architecture.
- **Anchor**: 2026-07-09 · t9
- **Status**: live

### D-8 — digest file format

- **Question**: what format is the artifact?
- **Answer**: operator delegated ("you decide"); recommendation adopted: markdown.
- **Reasoning**: consistent with every other operator-facing artifact in the system.
- **Anchor**: 2026-07-09 · t10
- **Status**: live

## Facts

- **F-1**: a real-time health-alarm surface already exists (starvation, churn, failure runs, fleet silence) — source: docs/prds/observability-and-spend.md (OBS-4)
- **F-2**: a structured domain event stream exists as the natural digest data source — source: docs/prds/observability-and-spend.md (OBS-1)

## Fog

- **G-1** (non-blocking — blocks: design phase): the digest's exact section layout and ordering
- **G-2** (non-blocking — blocks: design phase): where the artifact lands (repo file, issue, other surface) — parked with operator consent at t11

## Out of scope

- **O-1**: delivery/transport (email, push, chat) in v1 — the artifact is the value; transport multiplies infra (from D-5)
- **O-2**: real-time or urgent signaling in the digest — owned by the health-alarm surface (from D-6)

## Candidate glossary terms

- **T-1**: digest — the periodic, operator-facing rollup artifact summarizing factory activity over a window · avoid: report (collides with pass reports), summary

## Parked technical constraints

- **P-1**: generation should read the domain event stream (OBS-1 lineage) and work-item states through ports — medium-independent per AP-1 — context for stages 3–4

## Gate

| Field | Value |
|---|---|
| **Verdict** | ready-for-prd |
| **Date** | 2026-07-09 |
| **Operator** | scripted-golden (fixture) |
| **Covering source** | — |
| **Coverage check** | pass — all six inputs covered (§2.1←D-1/D-3 · §2.2←D-2 · §2.3←D-3 · §3.1←D-4 · §3.4←O-1/O-2 · appetite←D-7), zero blocking fog |
