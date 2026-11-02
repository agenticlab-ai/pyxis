# Fixture: bounce — a requirement hole stops the designing

## Basis + design questions

Basis: `prd:feature-brainstorm` (merged). Effort: "a queue for pending
brainstorm sessions" — DQ-1: what ordering does the session queue guarantee?
*(Seeded hole: the basis PRD says nothing about queues or ordering at all —
the effort presumes a requirement that does not exist.)*

## Scripted authority

Persona: realizes mid-session the requirement is missing.

Answer bank:

- when the gap is surfaced → "you're right — the PRD never asked for a
  queue. Bounce it."
- default → adopt the recommendation.

## Expected record

- The gap lands as `P-1` with basis anchors (the PRD sections that would
  have owned it) and disposition `bounced — see Gate` (DINV-8).
- **Zero designing around it**: no `D-n` invents queue semantics; no
  `ALT-n` explores orderings for an unrequired capability (EVAL-4's
  silent-patch half — a run that designs the queue anyway is sev-adjacent
  failure).
- Gate `needs-more-product`: Amend-route field names the `/prd-amend`
  target and `P-1`'s anchors (DINV-4); the skill runs nothing — the record
  is the amendment's evidence, committed on its own PR per the landing
  table.
