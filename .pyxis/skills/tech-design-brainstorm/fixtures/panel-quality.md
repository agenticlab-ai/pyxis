# Fixture: panel-quality — a divergence whose obvious shapes collapse

## Basis + design questions

Basis: `corpus:VNEXT-3` (end-to-end traceability).
Effort: "the trace chain's query surface" — DQ-1: what does a reader follow to
get from a merged code change back to the requirement that caused it?

*(The question is chosen because its obvious answers are one shape wearing three
homes: an identifier stamped at write time and grepped at read time, whether it
sits in a commit trailer, a field on the proposal, or a row in a sidecar index.
A genuinely distinct shape has to move the seam — links derived at read time
from the citations the documents already carry, with nothing stored, or an
append-only edge log the engine writes as a first-class artifact. The
requirement fixes that links are resolvable identifiers into the document layer;
it does not decide what carries them, so the divergence is real. The fixture
cannot force a weak first attempt — it can only make one likely.)*

## Scripted authority

Persona: an operator who audits merged work; wants to still get an answer a year
later, after the documents that caused a change have been superseded.

Answer bank:

- on the trace link's shape (DQ-1 panel presented) → "take the one that still
  answers after the document it points at has been superseded."
- on how far back a query has to reach → "park it — the design doc can carry
  it."
- default → adopt the recommendation.

## Expected record

- **Panel log**: DQ-1 `opens — fired` with its reason — a query surface over
  the trace chain is a new seam, and nothing in the tree implements one yet
  (DINV-7, FR-8).
- ≥ 2 scored `ALT-n` from DQ-1, each with its principles by ID, an incidents
  field naming where it was checked, and a verdict.
- **Transcript, the DQ-1 panel turn** (UB-7): where the first divergence the
  session presents is variations of one shape — the same stamped identifier in
  three homes — the session redoes the divergence under sharpened orthogonal
  constraints and presents the sharpened set instead. A weak first divergence
  presented as the final set is the failure.
- The trigger cannot be manufactured. Where the first divergence already comes
  out distinct, the row is **not reached** — reported as such and never as a
  pass: a green meaning "the condition did not arise" is indistinguishable from
  one meaning "the remedy works", and only one of them is evidence.
- Negative check: three alternatives differing only in which file the identifier
  is written to, or a redo that renames the first set rather than moving the
  seam, are both the weak case — neither is a pass.
