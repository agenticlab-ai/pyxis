# Amendment NNN — <slug>

| Field | Value |
|---|---|
| **Amends** | <feature> *(chain through NNN-1)* |
| **Status** | Draft → Approved *(merge requires Approved; merge is the freezing event)* |
| **Owner** | <one accountable person> |
| **Date** | <YYYY-MM-DD> |
| **Learned-from** | <class:ref · class:ref — e.g. `incident:I1 · telemetry-query:<query link>`> |
| **Blast radius** | <tier: additive / corrective / contract-changing; touches: <IDs>; reason: <why this tier>> |
| **Tracking** | <optional — the issue/PR that tracks work this amendment specifies but does not itself land; delete the row if the introducing PR is the whole work> |
| **Template version** | amendment-template/v2 |

## 0. How to use this template

*Guidance paragraphs look like this one — italic. Delete every guidance
paragraph when instantiating. The model this document instantiates is
`docs/tech-design/prd-amendment-model.md`; read it once before your first
amendment, then this skeleton is enough.*

*An amendment is a **typed ledger entry**, not an essay: a thin header plus
one `### op:` block per change. It lives in the feature's ledger directory as
`NNN-<slug>.md` — take the next free number (dense, monotonic; if another PR
claims it first, renumber onto the next free one). Once merged it is
**immutable forever**: to change what it did, write a later amendment. Keep
one intent per amendment — small entries keep races content-free and reviews
honest. One intent does not mean one operation: a coherent correction may
supersede FR-3, DOD-2, and NR-1 together — splitting one intent across
amendments creates temporary incoherence.*

*The header rules:*

- ***Learned-from** is typed and mandatory — closed classes: `incident` ·
  `review-thread` · `telemetry-query` · `build-result` · `user-research` ·
  `support-ticket` · `operator-decision` · `dependency-change` ·
  `policy-change`, each with a structured reference. "Learned from review"
  does not parse; evidence is a reference, not a vibe.*
- ***Blast radius** is checked, not trusted: `docgate` computes the minimum
  from your ops — additive (`add` of Could/Should rows, `answer`,
  waiting-room `rescope` outward) · corrective (`supersede`/`retire` of
  non-Must rows) · contract-changing (anything touching a Must row, a
  non-goal flip inward, a DoD blocking row, or §8-relevant content). You may
  declare higher than computed, never lower. The declared tier sets the
  review bar. Write it in the structured form
  `tier: <t>; touches: <IDs>; reason: <one line>` so the gate parses it.*
- ***Tracking** (optional, v2) names the tracker for work this amendment
  specifies but does not itself land — an implementation to follow, a
  migration pending. Closing that tracker before the work exists is the
  anti-pattern the row makes visible (prd-lifecycle-home rule 3). Envelopes
  declaring `amendment-template/v1` never carry it; validation keys off the
  declared version, so merged v1 amendments are grandfathered forever.*
- ***Status** freezes at `Approved` — a merged amendment carrying `Draft` is
  a gate error.*

*The operation rules (the fold applies these verbatim — nothing is
interpreted):*

- *Every payload is **complete and resolved**. "Tighten NFR-1" is not an
  operation; `supersede NFR-1` with the full replacement row is. Payload
  tables must match the base's template-version schema exactly — column
  names and order included.*
- *Fresh IDs come from the PRD's one global namespace and are **never
  reused** — not across tables, not after supersession. References are typed
  everywhere: internal anchors exact (`G1`, `FR-3`, `DOD-2`, `NG-1`);
  external ones as `design:<path#anchor>` · `issue:<id>` · `dashboard:<link>`.*
- *Prose without an anchor changes only by **section-granularity
  supersede** — there is no merge-these-paragraphs operation. A section-level
  supersede is **contract-changing by default** unless the schema registry
  classifies that section as non-contract metadata.*
- *After authoring, regenerate the projection (`docgate fold`) and commit
  the generated artifacts with the amendment — `verify-generated` refuses
  hand-edits and staleness. **Never edit `effective.md` or
  `effective.source.json` directly**: if the projection is wrong, the fix is
  the amendment operation or the fold implementation, never the output.
  (The gate is live: the `docs-gate` CI context and the pre-push hook run
  `docgate gate` — a stale, missing, or hand-edited projection cannot merge.
  Regenerate with `go run ./cmd/docgate fold docs/prds/<feature>`.)*

*The operation grammar — one `###` block per op:*

| Op | Syntax | Payload |
|---|---|---|
| add | `### op: add FR` *(target kind: FR · NR · UB · NFR · DOD · EVAL · EVT · Q)* | complete fresh row(s) for that kind's table |
| supersede | `### op: supersede FR-3` | complete replacement row |
| supersede (section) | `### op: supersede §4.3` | full replacement section body |
| retire | `### op: retire FR-3` | reason only |
| answer | `### op: answer Q2` | typed answer block |
| rescope | `### op: rescope NG-1` *(or WR-n)* | complete absorbing row(s) — no separate `add` |

*Before opening the PR: pick the next dense number · fill the header · keep
only the op blocks you need · then `docgate fold docs/prds/<feature>` ·
`docgate diff docs/prds/<feature>` · `docgate verify-generated
docs/prds/<feature>` — enforced by the `docs-gate` CI
context and the pre-push hook.*

*Keep only the `### op:` blocks you need; delete the rest of the examples
below.*

## Operations

### op: add FR

Reason: <one sentence — what was learned that makes this row necessary>

| ID | Requirement (EARS) | Rationale | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|
| FR-<next> | When <trigger>, the <system> shall <response>. | <why> | Should | low·med·high·critical | <measurable pass/fail> | Test | G1, J1 |

### op: add DOD

Reason: <one sentence — the completion criterion this change introduces>

| ID | Done criterion | Source | Verify | Required evidence | Blocking? |
|---|---|---|---|---|---|
| DOD-<next> | <criterion> | FR-<n> | Test | <evidence class> | yes |

### op: supersede FR-<n>

Reason: <one sentence — what was learned>

| ID | Requirement (EARS) | Rationale | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|
| FR-<next> | When <trigger>, the <system> shall <response>. | <why this replaces FR-n> | Must | high | <measure> | Test | G1 |

### op: supersede §<n.m> <section title>

Reason: <one sentence — why this section's prose is replaced wholesale>

<the complete replacement section BODY — the heading stays OUT of the
payload: a `### <n.m> <title>` line inside it ends the op block at the
parser's boundary and the fold refuses the emptied payload (#523) —
contract-changing by default>

### op: retire FR-<n>

Reason: <one sentence — why this requirement no longer applies; no replacement>

### op: answer Q<n>

| Field | Value |
|---|---|
| **Answer** | <the decision> |
| **Evidence** | <link> |
| **Decided-by** | <person / role> |
| **Effects** | <IDs affected, or none — carried into `docgate diff` as *declared*> |

### op: rescope NG-<n>

Reason: <one sentence — what changed the boundary; target is an NG-n or WR-n anchor>

| ID | Requirement (EARS) | Rationale | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|
| FR-<next> | <the absorbing requirement — materialized by this op, no separate `add`> | <why> | Should | med | <measure> | Test | G1 |

## Appendix — envelope maintenance *(delete when instantiating)*

*This template is the authoritative source of the amendment **envelope**
schema (header fields + op grammar), version-pinned as
`amendment-template/v2` (v1 plus the optional `Tracking` row; FR-24) — the instrument counterpart of the model decided in
`docs/tech-design/prd-amendment-model.md`, per its registries-derive-from-the-
template rule. Payload tables are validated against the **base PRD's**
`Template version` (they patch its tables), while the envelope is validated
against this one — two pins, separate lifecycles. Envelope changes bump this
version and, once `docgate` exists, ship with updated goldens in the same PR;
this template's example ops double as golden fixtures.*
