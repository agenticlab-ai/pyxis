# Amendment 013 — coverage on read: the tech-design index stops embodying the requirements-coverage join

| Field | Value |
|---|---|
| **Amends** | docgate (chain: 000 → 012) |
| **Status** | Approved |
| **Owner** | @mdarabi |
| **Date** | 2026-07-09 |
| **Learned-from** | review-thread:issue-70 · operator-decision:2026-07-09-coverage-on-read |
| **Blast radius** | tier: corrective; touches: FR-26, FR-28; reason: relocates the coverage projection from the committed index to an on-read verb — a Should row moves and the index.json `coverage` key is removed; no Must row changes |
| **Template version** | amendment-template/v1 |

## Operations

### op: supersede FR-26

Reason: FR-26 committed the requirements-coverage **join** (the PRD anchor universe × live designs' `serves`) into `docs/tech-design/index.*` and byte-verified it. The join is cross-plane, so committing it there couples the planes: adding a PRD — a `docs/prds/` concern with no design and no code yet — forces regenerating a `docs/tech-design/` artifact. That is an ownership surprise and a conflict magnet on one generated file pair (issue #70; observed on PR #67 landing into PR #68's rebase), the shared-append surface AP-23 says to remove structurally. The universe already lives under `docs/prds/` (each ledger's `effective.source.json` + the flat corpus), so the fix is to stop committing the join and compute it on read.

| ID | Requirement (EARS) | Rationale | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|
| FR-28 | When invoked as `coverage`, docgate shall compute the requirements-coverage view — for every corpus ID and every live ledger unit, the live designs serving it and the explicit unserved remainder per family — from committed inputs on read and print it (a human table by default, `--json` for the machine face), committing it to no artifact; and `gen-index`/`verify-index` shall carry only the design-plane graph (design liveness and `serves_inverse`), never the coverage join, so that adding or amending a ledger never changes `docs/tech-design/index.*`. | coverage is a cross-plane join; committing it anywhere couples the planes (issue #70). The universe already lives under `docs/prds/`, so computing on read removes the coupling in both directions and `index.*` regenerates only when designs change | Should | med | temp-repo fixtures: `docgate coverage` yields the expected served/unserved partition in both faces; `gen-index` emits no `coverage` key/section and `verify-index` is green over the coverage-free index; **adding a ledger amendment leaves `index.*` byte-identical**; superseding a design still returns its solely-served anchors to unserved in `coverage` output; CI publishes the view as a job summary | Test | G1, J2 |
