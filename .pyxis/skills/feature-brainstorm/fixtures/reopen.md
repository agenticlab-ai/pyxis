# Fixture: reopen — a new session on an idea with a frozen record

## Protocol

Precondition: a frozen (merged) record exists — use `records/weekly-digest.md`
as the stand-in predecessor (treat it as frozen at
`docs/brainstorms/weekly-digest.md`).

Run: `/feature-brainstorm` with the idea "revisit the weekly digest — I want
to rethink what goes in it now that we have incident records."

## Scripted operator

Persona: same operator, months later.

Answer bank:

- on reusing prior rulings → "keep everything we decided unless I say
  otherwise."
- on the one change → "add an incidents section; the rest stands."
- default → adopt the recommendation.

## Expected record

- The frozen record is **never edited** (DINV-2) — assert its content is
  byte-identical after the session.
- A fresh record `weekly-digest-2.md` exists, its **Prior records** row
  citing `brainstorm:weekly-digest`.
- Orientation surfaces the predecessor's rulings (FR-2), and **none of them
  is re-asked** (A-6) — the session grills only the delta (the incidents
  section).
- The delta lands as new `D-n` entries in the new record; prior rulings are
  referenced, not copied wholesale.
