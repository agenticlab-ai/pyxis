# Fixture: lens-defects — the golden run's spec

## Subject

`subjects/queue-widget-000.md` (moment: new-base) — five planted defects,
one per lens plus one critic gap.

## Scripted author

Applies every blocking finding; leaves advisories open.

## Expected report (`records/queue-widget-000.md`)

- **EVAL-6, per-lens catches**: contract-quality → FR-2's vague fit
  criterion on a Must row (CAL-3's escalation clause → **blocking**);
  coherence → G2 unmeasured **and** Must FR-3 without blocking DoD
  coverage (CAL-1, blocking ×2); upstream-alignment → the staleness
  decision ignoring the stale-read incident class (CAL-2, blocking);
  completeness-critic → the missing race failure row (CAL-4, advisory —
  not §8-load-bearing).
- Round 1: 4 blocking + 1 advisory, all five-elemented with CAL citations;
  verdict `request-changes`.
- Round 2 (scope-locked): the four applied closures confirmed against
  their stated acceptance conditions; the advisory left open; verdict
  `approve-with-advisories`.
- Measurements consistent; no docgate-floor findings anywhere (the
  subject's abbreviations are fixture-scoped, not findings).
