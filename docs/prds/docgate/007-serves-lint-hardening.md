# Amendment 007 — Serves hardening: corpus refs resolve, dead units red at introduction, Draft never merges

| Field | Value |
|---|---|
| **Amends** | docgate (chain: 000 + 001 + 002 + 003 + 004 + 005 + 006) |
| **Status** | Approved |
| **Date** | 2026-07-08 |
| **Owner** | @mdarabi |
| **Learned-from** | incident:I1 · review-thread:issue-54 |
| **Blast radius** | tier: contract-changing; touches: FR-16, FR-20; reason: supersedes the Must-row lint contract |
| **Template version** | amendment-template/v1 |

## Operations

### op: supersede FR-16

Reason: I1's fix closed the WR-stub hole narrowly (#54 A4/E5). Three gaps
remained: `corpus:` references — the majority of real `Serves` anchors while
requirements live in the placeholder corpus — resolved "by review" only, so a
design can serve a nonexistent baseline ID; resolution checked existence, not
liveness, so a **new** design could anchor itself to an already-superseded or
retired unit and pass; and the conventions' promised "a merged design carrying
Draft is a gate error" (§3.1) was never specified as a requirement. All three
join the lint contract. Later staleness (a unit superseded *after* a design
merged) deliberately stays a computed index flag (`stale-serves`), never a
defect — that is the conventions' decided behavior, unchanged here.

| ID | Requirement (EARS) | Rationale | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|
| FR-20 | When invoked as `lint-tech-designs`, docgate shall validate every design's closed header schema, typed `Serves` grammar (enumerated refs only), reference resolution — with `corpus:` references resolving against requirement IDs actually present in the flat placeholder-corpus files — mandatory-by-name Observability and Security & privacy subsections with the header pair, slug rules, and a Status row recording `Approved`; shall fail any design whose `Serves` cites a `prd:` ledger without at least one requirement-row anchor (FR-n, UB-n, or NFR-n); and, when a merge base is given, shall fail any design added since the merge base whose `Serves` cites a superseded or retired ledger unit. Designs merged before the v2 gate stay grandfathered; the config's precedence grandfather list (I1) exempts from the precedence clause only. | a resolvable stub must not read as a satisfied ordering rule (I1), an unresolvable corpus ref must not read as resolved, and dead intent must not anchor new designs (#54 A4) | Must | med | corpus-ref-to-nonexistent-ID fixture red naming the ID; the real corpus green (`local-work-tracker`'s corpus refs resolve); with a merge base, a new design serving a superseded FR red, a pre-existing design serving one green (flag only); a `Status: Draft` design red; grandfathered designs green; every FR-16 fixture keeps its verdict | Test | G1 |
