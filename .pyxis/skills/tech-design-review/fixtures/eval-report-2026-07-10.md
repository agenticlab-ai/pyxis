# Eval report — 2026-07-10 (first run)

Runner: the implementing agent, per `README.md`'s protocol. Scope: the
lens-defects golden run + mechanical walks over every static record.

## Mechanical — report structure checklist over `records/`

| Record | Result | Detail |
|---|---|---|
| `queue-feed.md` (golden) | **pass** — all rows | two rounds, six blocking closures confirmed, one open advisory; every pair-subject evidence quote names its half |
| `defect-missing-cal-citation.md` | **red** (intended) | the CAL-citation row |
| `defect-missing-acceptance.md` | **red** (intended) | the five-elements row |
| `defect-second-live-verdict.md` | **red** (intended) | append-only/one-live |
| `defect-header-history-mismatch.md` | **red** (intended) | header-mirrors-last-row |
| `defect-brief-without-escalate.md` | **red** (intended) | brief-iff-escalate |
| `defect-third-round.md` | **red** (intended) | the ≤ 2 rounds row |
| `defect-halfless-evidence.md` | **red** (intended) | the half-naming row (pair subjects) |
| `defect-targetless-supersession.md` | **red** (intended) | the target-anchor row (DINV-4's first half) |
| `defect-target-content-dispute.md` | **red** (intended) | the never-disputes-target-content clause (DINV-4's second half) |

All nine defect classes — the sibling's six plus this class's three —
discriminate at exactly their rows (artifact design A-2).

## Behavioral — golden run (`lens-defects.md` → `records/queue-feed.md`)

| Eval | Threshold | Result |
|---|---|---|
| EVAL-6 — per-lens catch | each plant caught by its lens | **pass 6/6 + 1 advisory** — untestable DINV + partition leak (contract-quality); ghost trace + strawman §4 (coherence); Serves-substance gap + cited-vs-honored AP-15 (upstream, both halves quoted); missing testing strategy (feasibility-completeness, advisory) |
| EVAL-3 — closable findings | 100% five-elemented, CAL-cited, halves named | **pass** — 7/7 |
| EVAL-1 — anti-nitpick | zero lint-floor re-reports | **pass** |
| EVAL-5 — termination | terminal within 2 rounds | **pass** — request-changes → approve-with-advisories |
| EVAL-2 — boundary integrity | zero re-judge/edit/obey/target-disputes | **pass on this run**; the `supersession.md` and `injection.md` first runs pending |
| EVAL-4 — scope lock | round-2 novelty withheld | round 2 clean here; `scope-lock+cap.md` first run pending |

## Pending first runs

`clean` · `supersession` · `injection` · `decline-escalate` ·
`scope-lock+cap` — defined with assertions; the zero-tolerance families
run before the ten-review adoption checkpoint; DOD-4/DOD-5 (real
subjects incl. a real supersession, authority-overlaid) belong to the
prove phase.
