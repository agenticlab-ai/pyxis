# Amendment 011 — requirements coverage: the index answers "what is still unserved?"

| Field | Value |
|---|---|
| **Amends** | docgate (chain: 000 + 001 + 002 + 003 + 004 + 005 + 006 + 007 + 008 + 009 + 010) |
| **Status** | Approved |
| **Date** | 2026-07-08 |
| **Owner** | @mdarabi |
| **Learned-from** | review-thread:issue-54 |
| **Blast radius** | tier: additive; touches: FR-26; reason: additive index face; existing index content unchanged |
| **Template version** | amendment-template/v1 |

## Operations

### op: add FR

Reason: "which requirements have no design yet" is the design phase's queue,
and today answering it is archaeology across 67 baseline features, 15 VNEXT
requirements, and the ledger units (#54 C4). The index already records each
design's `Serves`; the inverse — per requirement, its serving designs, and
the explicit unserved remainder — was never computed. It lands as an
additive face of the existing index projection, byte-verified like the rest.

| ID | Requirement (EARS) | Rationale | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|
| FR-26 | When invoked as `gen-index`, docgate shall additionally emit a requirements-coverage view in both index faces: for every requirement ID present in the flat placeholder corpus (the SCAF/PLAN/IMPL/GATE/FLEET/IDEN/GOV/KNOW/OBS/BENCH/VNEXT families) and every live unit of every ledger projection, the live designs serving it, plus the explicit list of IDs served by no live design; `verify-index` shall byte-verify the coverage content with the rest of the index. | the serves[] edge exists per design; planning needs its precomputed inverse and the unserved remainder, not archaeology (#54 C4) | Should | med | a fixture corpus + ledger + design set yields the expected served/unserved partition; the unserved list names IDs verbatim; a hand-edited coverage section red under `verify-index`; the real corpus green | Test | G1, J2 |
