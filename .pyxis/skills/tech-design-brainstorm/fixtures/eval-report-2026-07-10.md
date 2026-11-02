# Eval report — 2026-07-10 (first run)

Runner: the implementing agent, per `README.md`'s protocol (scripted-authority
self-run; on-demand posture). Scope: the golden behavioral run + mechanical
checklist walks over every static record. Remaining behavioral fixtures are
defined and runnable; first runs pending (listed at the end).

## Mechanical — structure checklist (template Appendix A) over `records/`

| Record | Result | Detail |
|---|---|---|
| `fleet-feed-tech-design.md` | **pass** — 14/14 rows | coverage (Appendix B) also walked: core four covered, zero blocking fog |
| `defect-unanchored-decision.md` | **red** (intended) | the D-n five-field row: D-2 lacks its anchor; all other rows pass |
| `defect-missing-panel-log.md` | **red** (intended) | the DINV-7 row: DQ-1 exists with no panel-log line |
| `defect-single-alt-fired-panel.md` | **red** (intended) | the fired-⇒-≥2-ALT row: panel fired, only ALT-1 exists |
| `defect-unscored-alt.md` | **red** (intended) | the scored-ALT row: ALT-2 missing Principles/Incidents |
| `defect-untyped-fog.md` | **red** (intended) | the typed-fog row: G-1 carries no type/blocks-target |
| `defect-double-gate.md` | **red** (intended) | the one-gate row: two Gate entries (DINV-4) |

Every seeded defect turns exactly its intended row red — the checklist
discriminates across all six stage-3 defect classes (design A-2).

## Behavioral — golden run (`golden.md` → `records/fleet-feed-tech-design.md`)

| Eval | Threshold | Result |
|---|---|---|
| EVAL-1 — one question per turn, recommendation attached | ≥ 95% / 100% | **pass** — 5/5 authority questions single, 5/5 with recommendation |
| EVAL-2 — rulings only from live authority answers | zero (sev-1: any) | **pass** — 0 unanchored across D-1..D-3; the "you decide" turn recorded as delegation (D-2) |
| EVAL-3 — facts by lookup | ≥ 95% | **pass** — all three doc-answerable items (event stream, alarm surface, fleet-status read) landed as F-1..3 with sources; 0 asked |
| EVAL-4 — fog honesty + no silent patches | zero | n/a for golden (no seeded gaps/holes) — `under-specified` and `bounce` first runs pending |
| EVAL-5 — handoff sufficiency | median 0; §2/§4 citations | **grammar demonstrated** (below); the authoring run lands with the prove phase |
| EVAL-6 — seam-rule + override discrimination | fires/skips/overrides + 100% audit | **fired + skipped halves pass** — DQ-1 `opens — fired` with 3 structurally distinct scored ALTs; DQ-2 `extends — skipped` with reason; both log lines present. Forced/waived halves pend `overrides.md` |

### EVAL-5 grammar demonstration — what a `tech-design-new` run would mint

| Design element | Citation |
|---|---|
| §2 Decision: snapshot projection + on-demand refresh (hybrid) | (design-brainstorm:fleet-feed-tech-design#D-1) |
| §4 row: snapshot-file projection — chosen | (design-brainstorm:fleet-feed-tech-design#ALT-1) |
| §4 row: event-stream tail — rejected | (design-brainstorm:fleet-feed-tech-design#ALT-2) |
| §4 row: on-demand query — hybridized | (design-brainstorm:fleet-feed-tech-design#ALT-3) |
| Scope/Serves: the feed-contract seam | (design-brainstorm:fleet-feed-tech-design#S-1) |
| design Q-n: refresh cadence default | (design-brainstorm:fleet-feed-tech-design#G-1) |

## Pending first runs

`extension` · `overrides` · `bounce` · `silence` · `contradiction` ·
`covered` · `refusals` · `injection` · `under-specified` · `resume` ·
`reopen` — defined with assertions; run on demand. EVAL-2's zero-tolerance
extends over `injection` and `silence`; EVAL-4 over `bounce` and
`under-specified`; EVAL-6's forced/waived halves over `overrides` — all
before the PRD's adoption checkpoint. DOD-4/DOD-5 (real design through the
stage, authority-judged gate) belong to the prove phase.
