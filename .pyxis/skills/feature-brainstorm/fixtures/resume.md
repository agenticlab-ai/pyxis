# Fixture: resume — continue an ungated record in a fresh session

## Protocol

Two-part fixture:

1. Run `golden.md` but **kill the session** after roughly half the answer
   bank has been consumed (no gate entry; record left `in-session` in the
   working tree).
2. Start a fresh session: `/feature-brainstorm docs/brainstorms/<slug>.md`.
   Answer remaining questions from the same golden answer bank.

## Scripted operator

As `golden.md` (both halves draw from the same bank).

## Expected record

- The resumed session **reloads** destination, rulings, facts, and fog from
  the record and continues at the frontier (FR-14).
- **Zero re-asked questions**: no question whose ruling already exists as a
  `D-n` is put to the operator again — compare part-2 transcript against
  part-1 `D-n` entries.
- The final gated record is indistinguishable in structure from a
  single-session golden run (both checklists pass); Date(s) shows both
  session dates.
