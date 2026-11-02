# Amendment 004 — requirement precedence: a stub anchor is not a requirement

| Field | Value |
|---|---|
| **Amends** | docgate (chain: 000 + 001 + 002 + 003) |
| **Status** | Approved |
| **Date** | 2026-07-07 |
| **Owner** | @mdarabi |
| **Learned-from** | incident:I1 · review-thread:pr-38-v2-design |
| **Blast radius** | tier: contract-changing; touches: FR-11, FR-16; reason: supersedes a Must row — the lint contract gains the precedence clause |
| **Template version** | amendment-template/v1 |

## Operations

### op: supersede FR-11

Reason: I1 — the v2 design merged before its requirements existed, and
no gate fired because `Serves: prd:docgate/WR-1` resolved: a one-line
waiting-room stub satisfied "PRDs precede designs" while FR-11..FR-15
were still unwritten, then back-filled. The lint contract closes the
stub-anchor hole: citing a feature's ledger obliges citing at least one
real requirement row, and because reference resolution runs against the
ledger at the design's introducing commit, requirements must exist by
then — back-filling becomes structurally impossible. The bootstrap
limitation stays named, not papered over: a PR installing a class's
gate cannot be gated by it; operator review is that PR's sole backstop.
docgate-v2.md itself would red under the new clause (its Serves cites
WR-1 alone) and is frozen — it enters a check-scoped precedence
grandfather list in config, citing I1: exempt from this clause only,
fully linted otherwise.

| ID | Requirement (EARS) | Rationale | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|
| FR-16 | When invoked as `lint-tech-designs`, docgate shall validate every design's closed header schema, typed `Serves` grammar (enumerated refs only), reference resolution, mandatory-by-name Observability and Security & privacy subsections with the header pair, and slug rules — and shall fail any design whose `Serves` cites a `prd:` ledger without at least one requirement-row anchor (FR-n, UB-n, or NFR-n): waiting-room, non-goal, and question anchors alone do not satisfy PRD-precedes-design. Designs merged before the v2 gate stay grandfathered; designs in the config's precedence grandfather list (I1) are exempt from the precedence clause only. | a resolvable stub must not read as a satisfied ordering rule (I1) | Must | med | WR-only `Serves` fixture red naming I1; FR-anchored fixture green; `docgate-v2` exempt via the precedence list; the real corpus green | Test | G1 |
