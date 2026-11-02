# Read to rule (§0) — design-brainstorm record

| Field | Value |
|---|---|
| **In one line** | The §0 fine print as a design: how typed amendment ops address the section (maintenance and introduction), where the capture line lives relative to the review-feedback ledger, and the template contract's constants (tier rule, cap, honest-zero forms, staged-gate reading). |
| **Relevant-when** | designing or changing the §0 read-to-rule template contract, its amendment-op semantics, the gate capture line, or its federation with the review-feedback ledger; checking prior design rulings or refused shapes about §0 mechanics |
| **Basis** | prd:read-to-rule (merged ledger, 2026-07-14) |
| **Authority** | @mdarabi |
| **Date(s)** | 2026-07-14 |
| **Status** | gated: ready-for-design |
| **Tracking** | — |
| **Feeds** | docs/tech-design/read-to-rule-v1.md |
| **Prior records** | brainstorm:gate-attention · design-brainstorm:review-feedback-capture-tech-design (via its authored design) |
| **Template version** | design-brainstorm-record/v1 |

## Orientation

| Source | Why loaded |
|---|---|
| docs/tech-design/index.md | routing — no live design's Serves touches prd:read-to-rule (new ground); located the review-feedback-capture design (live, serves its FR-1..9) |
| docs/prds/read-to-rule/effective.md | the basis — Q1 enumerates this session's ground; Q2, NG-1..6, FR-1..9, the capture convention |
| docs/tech-design/prd-amendment-model.md §3–§4 | the op grammar: `supersede` takes "an existing ID or **section anchor**" with a full-section payload; `add` admits "a new anchored section"; prose without anchors evolves "wholesale at section granularity" |
| docs/tech-design/review-feedback-capture.md | the federation counterpart: per-subject ledger + coded writer + `CaptureSource` port (doc-review + code-host adapters) + `review-feedback:<stem>#<anchor>` citation grammar + one-directional cross-links (their NG-5 posture) |
| docs/observability/incidents.md (all entries, 2026-07-14) | I1 (a design's Serves must name committed FRs), I2 (badge never certifies main — stable refs), I3 (fail-open on misparse) |
| docs/wiki/glossary.md | term fidelity — read to rule (§0), fold, projection, blast radius, DecisionBrief |
| brainstorm:gate-attention (in-context) | the ruled basis behind the PRD — D-1..D-9 boundaries this design must not re-open |

**Overlaps surfaced:** `review-feedback-capture` (live design) — its adapters already capture doc-review annotations and code-host comments, so a §0 gate's feedback items will exist as its ledger entries regardless of this design; the capture line must join or knowingly duplicate (DQ-2). `prd-amendment-model` (live, grandfathered) — already decides section-granularity ops; this design extends its semantics to a section numbered 0, never edits it. No live design covers the basis; proceeding.

## Design questions

- **DQ-1**: How do typed amendment ops address §0 — maintenance on a §0-bearing ledger, introduction into a pre-§0 ledger, placement before §1 at fold time, and the blast-radius tier a §0-only op computes?
- **DQ-2**: Where does the capture line live relative to the review-feedback ledger — a sign-off-surface line, a ledger-native signal, or a federated line that cites?
- **DQ-3**: The template contract's constants — the tier rule (which tiers carry §0; the skip form), the length cap and ranked-call bounds, the per-block honest-zero form, and the budget hint.
- **DQ-4**: The capture-line format and its gate scope — the one-line-per-review-event shape, the sampled-read marker, and what a staged (problem-only) gate owes §0.

## Panel log

- **DQ-1**: extends — skipped — the amendment model already grants section-granularity `supersede` and `add`-a-new-anchored-section; §0 is another anchored section (placement and blast tier are parameterizations, not a new seam)
- **DQ-2**: opens — fired — the capture line ↔ review-feedback ledger join is a new data seam; three structurally distinct shapes scored (ALT-1..3)
- **DQ-3**: extends — skipped — constants inside the template seam prd-template already owns (versioned guidance, NFR-2)
- **DQ-4**: extends — skipped — the sign-off surface convention the basis already places (FR-7/FR-8); format details only

## Decisions

### D-1 — DQ-2: the federated line (ALT-3)

- **Question**: the capture line's home — standalone sign-off line (ALT-1), ledger-native in review-feedback (ALT-2), or a federated sign-off line citing their entry anchors (ALT-3, recommended)?
- **Answer**: ALT-3, as recommended — the line stays on the sign-off surface; its pinned format cites each item's `review-feedback:<subject-stem>#<entry-anchor>` one-directionally; M1/M2 stay hand-computed from lines (DOD-7); graduation to ledger-native is explicit and runs through their channel if the checkpoint proves the bet.
- **Reasoning**: mirrors the counterpart design's own cross-link posture; no schema or timing coupling; the basis's appetite intact; the join beats both uncorrelated duplication (ALT-1) and an I1-class cross-contract inversion (ALT-2).
- **Anchor**: 2026-07-14 · t1
- **Status**: live

### D-2 — the simplicity mandate: §0 is an ordinary section; reuse everything, build nothing

- **Question**: DQ-1's op semantics were put as a four-clause ruling; the authority challenged the altitude: "if we are just adding a simple section on top of the prd/techspec doc why do we need this level of complexity???"
- **Answer**: the challenge is the ruling — §0 is an ordinary section. Maintenance uses the grammar's existing wholesale section-supersede (F-1); nothing new is built (no sub-anchor namespace, no fold changes, no docgate work); unexercised paths (introduction into pre-§0 ledgers, blast-tier fine print) are deferred in one line until someone actually hits them; the template constants (cap, tier rule, honest-zero form, capture-line shape) are delegated to design authorship within this posture, vetoable at the design's own review.
- **Reasoning**: the authority's — the feature's whole point is cheap attention; a design that manufactures decisions re-imports the cost it exists to remove. Mechanical consequences (e.g. a §0-only op cannot be contract-changing, per the basis's NG-1) are facts to state, not rulings to extract.
- **Anchor**: 2026-07-14 · t2
- **Status**: live

## Alternatives

### ALT-1 — the sign-off-surface line (standalone)

- **Shape**: the capture line is hand-recorded on the gate's sign-off row (draft-phase gate tables) or the approving review (amendments); M1/M2 hand-computed from those lines (DOD-7); no link to the review-feedback ledger.
- **Trade-offs**: wins — zero coupling, ships with the template alone, works on any surface; costs — the same review event is captured twice (its comments land in the review-feedback ledger via its adapters) with no join; the in-§0/outside-§0 classification never reaches a machine face; hand-authored measurement data.
- **Principles**: AP-19 honored (the convention's cost stated); AP-11 strained (two uncorrelated records of one review event); AP-10 strained (measurement stays convention, not code).
- **Incidents**: none matched (registry read 2026-07-14; I2's stable-ref lesson n/a — no refs).
- **Verdict**: rejected — the double capture without a join squanders the substrate that shipped mid-authoring; hybridized into ALT-3.
- **From**: DQ-2 · panel

### ALT-2 — ride the review-feedback ledger (ledger-native)

- **Shape**: the capture line's items ARE review-feedback entries (the adapters already capture them); this feature adds a routing field (in-§0 · outside-§0) and a per-gate minutes/sampled datum to that ledger's schema; M1/M2 computed ledger-native like its FR-5 measures.
- **Trade-offs**: wins — one substrate, coded writer, machine face for free, no double capture; costs — requires evolving another feature's frozen design/schema through its own channel (a cross-feature dependency the basis's small appetite never priced); the minutes datum is per-gate, not per-comment — no natural home in a per-comment entry; a clean approve (zero items) produces no entries yet still needs minutes; timing couples the template launch to their implementation.
- **Principles**: AP-10/AP-21 honored (codified, frozen at write); AP-11 honored (one record); AP-19 strained (adoption cost exported to another ledger); the basis's D-8 appetite strained (excluded exactly this class of cross-feature work).
- **Incidents**: I1 matched as a caution — a design creating obligations on another feature's committed contract without its channel is the same class of inversion.
- **Verdict**: rejected — right end-state, wrong moment and wrong channel; named as the explicit graduation path inside ALT-3.
- **From**: DQ-2 · panel

### ALT-3 — the federated line (cite, don't couple)

- **Shape**: the capture line stays a sign-off-surface line (ALT-1's home), but its pinned format carries one-directional citations: each feedback item cites its `review-feedback:<subject-stem>#<entry-anchor>` where the adapters captured the underlying comment; the routing classification and minutes stay in the line; M1/M2 hand-computed from lines (DOD-7 intact), with the citation join available to any future consumer.
- **Trade-offs**: wins — no schema change on their ledger (mirrors their own one-directional cross-link rule), no timing coupling, the basis's appetite intact, an explicit graduation path (a later amendment through their channel can absorb the line's fields once the checkpoint proves the bet); costs — the join is citation-discipline-dependent; minutes stay hand-recorded (owned at solo scale by the basis's R3/sampling posture).
- **Principles**: AP-11 honored via the join (one comment record, one measurement line, linked); AP-19 honored (costs stated); AP-21 honored (classification frozen at signing); AP-10 deferred knowingly (a resolution lint is the review-feedback design's own check pattern, adoptable later).
- **Incidents**: I2 matched — citations use their grammar's stable anchors (entry anchors, never working-branch refs); I1 respected (no obligation lands on their contract).
- **Verdict**: chosen — D-1.
- **From**: DQ-2 · panel

## Seam map

- **S-1**: the prd-template §0 contract (placement, blocks, constants) — extends — behind it: prd-template v2's guidance text — feeds: the design's Scope + Serves (prd:read-to-rule/FR-1..5, NFR-1/2)
- **S-2**: the amendment-op semantics for a section numbered 0 — extends — behind it: `supersede §0` (full body) · `add` §0 (introduction; fold placement before §1) · blast tier for §0-only ops — feeds: Scope + Serves (FR-6; Q1's op-mechanics clause)
- **S-3**: the capture line ↔ review-feedback ledger join — opens — behind it: the line's citation fields (`review-feedback:` grammar) — feeds: Scope + Serves (FR-7/FR-8; Q1's federation clause)
- **S-4**: the gate sign-off convention — extends — behind it: the pinned capture-line format + staged-gate scope — feeds: Scope + Serves (FR-7..9, UB-4)

## Facts

- **F-1**: the op vocabulary already admits §0 — `supersede` targets "an existing ID or section anchor" with a full-section payload; `add` admits "a new anchored section"; unanchored prose evolves "wholesale at section granularity" — source: docs/tech-design/prd-amendment-model.md §3
- **F-2**: new 000-bases need zero docgate change — §0 is ordinary base-document text to the fold (the basis's NR-1 posture); only the pre-§0-ledger introduction path touches fold placement, and that path is deferred by NG-6 (may-adopt, mechanics at first exercise) — source: prd:read-to-rule NR-1/NG-6; prd-amendment-model §4
- **F-3**: the review-feedback design is live and its adapters capture doc-review annotations and code-host comments at their sources — a §0 gate's feedback items will be captured there regardless of this design — source: docs/tech-design/review-feedback-capture.md §3.3
- **F-4**: their citation grammar is `review-feedback:<subject-stem>#<entry-anchor>` with stable outcome-refs (never working-branch shas) and one-directional cross-links (their NG-5) — source: docs/tech-design/review-feedback-capture.md §3.5
- **F-5**: blast radius is computed from ops by docgate, declared-may-exceed-computed; a §0-only op's tier is a semantics question this design must state — source: prd-amendment-model §7; glossary
- **F-6**: docgate's own template contract makes template PRs ship updated goldens and parses headers against declared template versions — the v2 bump's mechanical accompaniments are already priced by the basis (NR-1, DOD-6) — source: prd:docgate §5.5; prd:read-to-rule NR-1

## Fog

none

## Out of scope

- **O-1**: any new machinery for §0 — no sub-anchor ID namespace, no fold or docgate changes, no schema work; the existing section-granularity ops are the whole mechanism (from D-2)
- **O-2**: designing the pre-§0-ledger introduction path now — deferred in one line until first exercised (NG-6's may-adopt posture) (from D-2)

## Candidate glossary terms

none

## Parked product gaps

none

## Gate

| Field | Value |
|---|---|
| **Verdict** | ready-for-design |
| **Date** | 2026-07-14 |
| **Authority** | @mdarabi |
| **Covering design** | — |
| **Coverage check** | pass — core four covered (§2 Decision←D-1/D-2, both DQs panel-logged · §4 Alternatives←ALT-1..3 verdicted · Serves+Scope←S-1..4 + the basis · §3.2 boundaries←O-1..2), zero blocking fog |
| **Amend route** | — |
