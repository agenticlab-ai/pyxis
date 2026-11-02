# Fixture: question-form — turns legible in the authority's register

## Basis + design questions

Basis: `corpus:OBS-4` (health alarms & escalation — starvation, churn,
failure runs, fleet silence). Effort: "the stall detector's state seam" —
DQ-1: where does stall state live (derived on read vs persisted)? DQ-2:
what threshold marks a feature stalled, and what is the surface called?

*(The effort forces the turn shapes EVAL-7 distinguishes: a structural
question whose panel fires — its example must be one flow traced through
each shape — an open-form threshold turn wanting sample answers, and a
naming turn whose only honest example is a typed skip.)*

## Scripted authority

Persona: terse; answers by option label whenever the turn offers labels.

Answer bank:

- on where stall state lives (DQ-1 panel presented) → "A".
- on the stall threshold (open-form) → adopt the first sample answer.
- on the surface's name → adopt the recommendation.
- default → adopt the recommendation, by label where offered.

The bank answers by bare label on purpose: a turn whose options cannot be
answered that way fails the form.

## Expected record

- Gate verdict: any — this fixture asserts on the *transcript*, not the
  direction.
- **Transcript, every question turn** (EVAL-7 / FR-17): the form is
  present — plain-language question; one example (real with its source, or
  marked *hypothetical*/*illustrative*) **or** the literal
  `no example: <reason>` line; enumerated options each stating its outcome
  **on that turn's example** (or one to two sample answers with their
  meanings on open-form turns); the recommendation marked; the turn
  answerable by bare label.
- **Transcript, the DQ-1 panel turn**: the `ALT-n` shapes are the options
  and each is traced through the *same* flow (quote the turn; an
  alternative illustrated on its own separate scenario is the negative).
- **Transcript**: zero bare anchors — any `F-n`/`DQ-n`/`ALT-n`/document
  reference inside a question turn carries an inline gloss (quote a
  violating turn as evidence).
- **Transcript, the naming turn**: carries a typed example-skip — a forced
  scenario bolted onto it is the negative.
- **Transcript, every example-bearing turn**: no two options share an
  outcome on the turn's example (quote the turn otherwise).
- Record entries: unaffected by the conversational form — rulings and
  `ALT-n` entries stay compact per the template (panel alternatives ≤ 10
  lines each); the examples do **not** bloat the record (NFR-2 spot-check).
- Panel log: DQ-1 `opens — fired` with reason (DINV-7) — the form changes
  presentation only, never the seam call.
