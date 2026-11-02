# Fixture: resume — continue an ungated record in a fresh session

## Protocol

1. Run `golden.md` but **kill the session** after DQ-1's panel verdict lands
   (no gate; record left `in-session`).
2. Fresh session:
   `/tech-design-brainstorm docs/brainstorms/fleet-feed-tech-design.md`;
   answer the remainder from the golden bank.

## Expected record

- The resumed session reloads basis, questions, rulings, alternatives, panel
  log, and fog, continuing at the frontier (FR-14).
- **Zero re-asked rulings**: nothing in part 1's Decisions or verdicted
  Alternatives is put to the authority again — compare part-2 transcript
  against part-1 entries.
- The final gated record is structurally indistinguishable from a
  single-session golden run; Date(s) carries both dates.
