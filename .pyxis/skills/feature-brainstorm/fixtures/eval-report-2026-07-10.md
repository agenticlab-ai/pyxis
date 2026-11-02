# Eval report — 2026-07-10 (mechanical re-walk: header consistency)

Runner: the implementing agent. Scope: structure-checklist re-walk over
`records/` after the `brainstorm-record/v1` header gained the house routing
pair (`In one line`, `Relevant-when`) and `Tracking` (additive within v1 per
the design's evolution clause; zero real records existed, so no migration).
Behavioral results from the 2026-07-09 report are unaffected (no session
behavior changed).

| Record | Result | Detail |
|---|---|---|
| `weekly-digest.md` | **pass** — 11/11 rows | header row now checks all ten fields; coverage walk unchanged (6/6, zero blocking fog) |
| `defect-unanchored-decision.md` | **red** (intended) | still exactly row 3 (D-2 anchor); header row passes with the new fields |
| `defect-untyped-fog.md` | **red** (intended) | still exactly row 6 |
| `defect-double-gate.md` | **red** (intended) | still exactly row 8 |
| `defect-deleted-supersession.md` | **red** (intended) | still exactly rows 5 + 10 |

Discrimination preserved: every defect record is red only at its seeded
row(s); the golden record passes the widened header check.
