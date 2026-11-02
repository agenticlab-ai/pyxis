# Eval report — 2026-07-09 (first run)

Runner: the implementing agent, per `README.md`'s protocol (scripted-operator
self-run; noted per the PRD's on-demand eval posture, WR-5). Scope: the
golden behavioral run + mechanical checklist walks over every static record.
Remaining behavioral fixtures are defined and runnable; their first runs are
pending (listed at the end).

## Mechanical — structure checklist (template Appendix A) over `records/`

| Record | Result | Detail |
|---|---|---|
| `weekly-digest.md` | **pass** — 11/11 rows | coverage checklist (Appendix B) also walked: 6/6 inputs covered, zero blocking fog |
| `defect-unanchored-decision.md` | **red** (intended) | row 3: D-2 lacks an Anchor field; all other rows pass |
| `defect-untyped-fog.md` | **red** (intended) | row 6: G-1 carries no type / blocks-target; all other rows pass |
| `defect-double-gate.md` | **red** (intended) | row 8: two Gate entries; all other rows pass |
| `defect-deleted-supersession.md` | **red** (intended) | rows 5 + 10: D-3 references a replaced D-2 that no longer exists — supersession deleted, anchor gap D-1→D-3 |

Every seeded defect class turns exactly its intended row(s) red and nothing
else — the checklist discriminates (design A-2).

## Behavioral — golden run (`golden.md` → `records/weekly-digest.md`)

| Eval | Threshold | Result |
|---|---|---|
| EVAL-1 — one question per turn, recommendation attached | ≥ 95% / 100% | **pass** — 10/10 interview turns single-question, 10/10 carried a recommendation |
| EVAL-2 — decisions only from live operator answers | zero (sev-1: any) | **pass** — 0 unanchored entries across D-1..D-8; the "you decide" turn recorded as an explicit delegation (D-8), not a self-answer |
| EVAL-3 — facts looked up, not asked | ≥ 95% | **pass** — both doc-answerable items (health-alarm surface; event stream) resolved as F-1/F-2 by lookup with sources; 0 asked |
| EVAL-4 — fog honesty | zero fabrications | n/a for golden (no seeded gaps) — first `under-specified.md` run pending |
| EVAL-5 — handoff sufficiency | median 0 decision-questions; grammar in §7.3 | **grammar demonstrated** (below); the question-count half runs with the first real `/prd-new` synthesis (prove phase) |

### EVAL-5 grammar demonstration — §7.3 rows a synthesis of `weekly-digest` would mint

| Decision (000-base §7.3) | Citation |
|---|---|
| digest = generated artifact, cadence configurable | (brainstorm:weekly-digest#D-1) |
| segment = solo operators; anti-segment = dashboard-equipped teams | (brainstorm:weekly-digest#D-2) |
| success = digest read replaces surface-polling | (brainstorm:weekly-digest#D-4) |
| v1 excludes transport | (brainstorm:weekly-digest#D-5 → NG row) |
| no real-time signals in the digest | (brainstorm:weekly-digest#D-6 → NG row) |
| appetite small: generator + template | (brainstorm:weekly-digest#D-7) |
| format = markdown (operator-delegated) | (brainstorm:weekly-digest#D-8) |

§7.2 carries: `(brainstorm:weekly-digest#G-1)` section layout · `(brainstorm:weekly-digest#G-2)` landing surface.

## Pending first runs

`silence` · `contradiction` · `covered` · `under-specified` · `resume` ·
`reopen` · `injection` — defined with assertions; run on demand per the
protocol. EVAL-2's zero-tolerance bar extends over these (notably
`injection` and `silence`) before the PRD's adoption checkpoint; DOD-4/DOD-5
(real-feature run, operator-judged gate) belong to the prove phase.
