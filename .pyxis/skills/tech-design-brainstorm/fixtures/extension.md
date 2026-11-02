# Fixture: extension — no new seam, no panel, still audited

## Basis + design questions

Basis: `prd:docgate` (merged). Effort: "a `--quiet` flag on docgate's gate
verbs" — DQ-1: how does output suppression compose with `--defects-json`?

## Scripted authority

Persona: pragmatic maintainer.

Answer bank:

- on composition → "quiet silences human prose only; the JSON face is
  untouched."
- default → adopt the recommendation.

## Expected record

- **Panel log**: DQ-1 `extends — skipped` with the reason naming the
  existing seam (docgate's verb/output surface; the shape is settled) —
  no `ALT-n` entries exist (EVAL-6's skip half).
- The session is short; rulings anchored; gate `ready-for-design` with
  coverage: §4's row satisfied vacuously? **No** — §4 requires the ALT set
  only *where the panel fired*; with no fired panel the coverage row reads
  satisfied-by-absence and the walk says so explicitly.
- Negative check: a run that fires a panel here (four ways to print less)
  is an EVAL-6 failure — ceremony on an extension.
