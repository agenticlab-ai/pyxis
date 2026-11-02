# Fixture: golden — panel fires on a new seam, skips on an extension

## Basis + design questions

Basis: `corpus:OBS-5` (live monitoring surfaces) · `corpus:OBS-1` (the
domain event stream). Effort: "the fleet dashboard's data feed" — DQ-1: what
feed contract serves the live fleet dashboard? DQ-2: where does the feed's
snapshot artifact live?

## Scripted authority

Persona: decisive; adopts good recommendations; wants cheap-by-default.

Answer bank:

- on the feed direction (DQ-1 panel presented) → "the snapshot file, but
  take the on-demand refresh verb from the query option — hybrid."
- on snapshot home (DQ-2) → "you decide." *(delegation)*
- on real-time push → "out of scope — the alarm surface owns urgency."
- on refresh cadence default → "park it — the design doc can carry it."
- default → adopt the recommendation.

## Expected record

- Filename `fleet-feed-tech-design.md`; gate `ready-for-design` with the
  coverage check embedded (core four + zero blocking fog — DINV-3/4).
- **Panel log**: DQ-1 `opens — fired` (a new feed contract is a new seam);
  DQ-2 `extends — skipped` (artifact placement follows existing repo
  conventions) — both lines with reasons (DINV-7, EVAL-6).
- ≥ 3 `ALT-n` from DQ-1, each with principles by ID, an incidents field
  naming *where checked* (the registry holds no matching entries), and a
  verdict; the chosen entry records the hybrid (EVAL-6; A-2).
- `D-1` the direction (hybrid), `D-2` the delegated snapshot home (recorded
  as delegation), `D-3` the real-time exclusion → `O-1` cites it.
- ≥ 2 `F-n` (the event stream exists; the alarm surface exists) — looked
  up from the corpus, not asked (EVAL-3).
- `G-1` non-blocking (refresh cadence → the design's `Q-n`).
- `S-1` opens the feed-contract seam; `S-2` extends the artifact/read
  surface — each with a feeds-line.
- Structure checklist: all rows pass. EVAL-1: 100% single-question with
  recommendation. EVAL-2: zero unanchored rulings.
- EVAL-5 follow-on: `/tech-design-new` on this record authors §2 citing
  `(design-brainstorm:fleet-feed-tech-design#D-1)` and §4 rows citing
  `#ALT-n`, asking 0 decision-questions.
