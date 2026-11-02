# Amendment 001 — no-shared-grilling-primitive

| Field | Value |
|---|---|
| **Amends** | feature-brainstorm *(chain through 000)* |
| **Status** | Approved |
| **Owner** | @mdarabi |
| **Date** | 2026-07-10 |
| **Learned-from** | operator-decision:brainstorm:tech-design-brainstorm#D-6 |
| **Blast radius** | tier: contract-changing; touches: WR-2, NG-7 (new); reason: a boundary flip (deferral → refusal) — docgate computes contract-changing for NG changes; no requirement or DoD row is touched, and the ruling was made and reviewed in the live session (D-6) |
| **Template version** | amendment-template/v1 |

## Operations

### op: rescope WR-2

Reason: WR-2's revisit trigger fired — the second consumer (the stage-3 tech-design-brainstorm skill) arrived — and the operator refused the extraction: the two grilling disciplines evolve separately in the long run (brainstorm:tech-design-brainstorm#D-6).

| ID | Non-goal | Why refused |
|---|---|---|
| NG-7 | A shared grilling primitive across the stage-1 and stage-3 skills | operator ruling at the WR-2 revisit (2026-07-10): the disciplines evolve separately; coupling their evolution costs more than the bounded duplication, which per-skill eval corpora keep honest (brainstorm:tech-design-brainstorm#D-6) |
