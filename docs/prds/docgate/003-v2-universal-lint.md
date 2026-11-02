# Amendment 003 — v2: universal document lint and the tech-design index

| Field | Value |
|---|---|
| **Amends** | docgate (chain: 000 + 001 + 002) |
| **Status** | Approved |
| **Owner** | @mdarabi |
| **Date** | 2026-07-07 |
| **Learned-from** | operator-decision:2026-07-07-enumeration-ruling · review-thread:pr-38-v2-design · operator-decision:issue-37-audit |
| **Blast radius** | tier: additive; touches: FR-11, FR-12, FR-13, FR-14, FR-15; reason: adds the v2 verb contract — no existing unit changes; WR-1 comes due |
| **Template version** | amendment-template/v1 |

## Operations

### op: add FR

Reason: the frozen v2 design (docgate-v2.md, serving WR-1 and issue #37)
turns the five unguarded classes into checks. The operator ruled the
`Serves` ref grammar requires explicit enumeration — no range shorthand
(`FR-1..FR-8` is a defect): ranges make the inverse map depend on
expansion semantics across superseded IDs, and every live design already
enumerates. That ruling is baked into FR-11's fit criterion and recorded
here as this ledger's durable evidence (the design's Q1 resolution).

| ID | Requirement (EARS) | Rationale | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|
| FR-11 | When invoked as `lint-tech-designs`, docgate shall validate every design's closed header schema, typed `Serves` grammar (enumerated refs only — range shorthand is a defect), reference resolution, mandatory-by-name Observability and Security & privacy subsections with the header pair, and slug rules, grandfathering designs merged before the v2 gate. | the class's conventions become checks | Must | med | one red fixture per defect class; the real corpus green with pre-v2 docs flagged grandfathered; a `FR-1..FR-8` ref → named defect | Test | G1 |
| FR-12 | When invoked as `gen-index`, docgate shall emit the tech-design index (`index.md` + `index.json`) deterministically with liveness computed from `Supersedes` edges, lineage, `Serves` inverse map, and staleness flags; `verify-index` shall byte-reject any committed index differing from the recomputed one. | the class's projection; liveness computed, never written | Must | med | hand-edited index red; regenerated green; fixture graph incl. a supersession chain yields expected liveness and flags | Test | G1 |
| FR-13 | When invoked as `lint-wiki` or `lint-rollups`, docgate shall verify the class's metadata header rows are present and well-formed (wiki: In one line, Relevant-when, Authority, Sources, Last reviewed; rollups: In one line, Relevant-when, Authority, Update rule, Seeded). | routing metadata becomes checkable | Must | low | missing-row and malformed-date fixtures red; real corpus green | Test | J3 |
| FR-14 | When invoked as `check-governing` with a merge base, docgate shall fail any diff touching the vision or principles unless the commits carry the `Governing-Change: operator` trailer, and shall fail principles line modifications or deletions even with the trailer (append-only). | the trust root gains machine backing; attestation, not authorization | Must | high | untrailed diff red; trailed append green; trailed line-edit of principles red | Test | G1, J3 |
| FR-15 | When invoked as `lint-incidents`, docgate shall verify the registry is append-only vs the merge base and each entry carries a dense `I<n>` ID with the four named fields. | the append-only registry becomes checkable | Must | low | tampered-entry and gap fixtures red; real registry green | Test | J3 |
