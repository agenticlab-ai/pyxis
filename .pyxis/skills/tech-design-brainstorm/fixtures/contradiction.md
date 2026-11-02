# Fixture: contradiction — a later answer conflicts with an earlier ruling

## Basis + design questions

Basis: `prd:docgate` (merged). Effort: "defect-output streaming" — DQ-1:
one JSON line per defect, or a single document?

## Scripted authority

Answer bank:

- on the shape → "one line per defect, strictly — consumers tail it."
- **later, on buffering** → "buffer everything and emit one array at the
  end." *(contradicts line-per-defect)*
- when challenged, quoting both → "keep line-per-defect; no buffering."
- default → adopt the recommendation.

## Expected record

- The contradiction is challenged before anything is recorded — the
  transcript quotes both the earlier `D-n` and the new answer (UB-2).
- The losing position, if it was recorded, is `superseded by D-m — <reason>`
  with **both entries present** (DINV-5); no anchor gaps.
