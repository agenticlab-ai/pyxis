# tech-design-review — eval fixture corpus

The harness for the PRD's EVAL-1..6 and the artifact design's A-2/A-3.
Mirrors the sibling corpus's protocol (each reviewer owns its fixtures —
the independence rulings): a fixture names a **subject** under `subjects/`
(abbreviated pseudo-designs/shapes, deliberately imperfect), the
**scripted author**, and the **expected report**. `records/` holds the
golden report and seeded-defect reports, each red at exactly its checklist
row. Real reports live in `docs/reviews/`.

## Runner protocol

As the sibling's: run `/tech-design-review` on the subject → terminal
verdict → walk the checklist → evaluate assertions → append per-eval
results (EVAL-2's boundary family — never re-judge/edit/obey, **incl.
never disputing supersession targets' content** — is sev-1).

## Corpus

| Fixture | Exercises | Primary rows |
|---|---|---|
| `clean.md` | a defect-free pair → honest approve | EVAL-1 · UB-2 |
| `lens-defects.md` | the seeded pair's six plants across the four lenses | EVAL-6 · FR-3..6 (golden spec) |
| `supersession.md` | integrity checks: stranded A-row, false contradiction; targets-as-evidence | FR-13 · EVAL-2 |
| `injection.md` | directives planted in the design/shape | EVAL-2 · UB-4 · ABUSE-1 |
| `decline-escalate.md` | reasoned decline → escalate + embedded brief | FR-8 · FR-11 |
| `scope-lock+cap.md` | round-2 novelty withheld; no third round | EVAL-4 · EVAL-5 |
