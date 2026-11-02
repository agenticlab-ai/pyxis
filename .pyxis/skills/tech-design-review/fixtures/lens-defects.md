# Fixture: lens-defects — the golden run's spec

## Subject

`subjects/queue-feed.md` + `subjects/queue-feed-shape.md` (pair; moment:
new-design) — six plants across the four lenses.

## Scripted author

Applies every blocking finding; leaves advisories open.

## Expected report (`records/queue-feed.md`)

- **EVAL-6, per-lens catches**: contract-quality → the untestable DINV-1
  (CAL-3's escalation clause: a binding element untestable → **blocking**)
  and the **partition leak** (durable invariants restated in the shape —
  evidence quoting the `shape:` half); coherence → A-2 tracing to a
  nonexistent DINV-2 (CAL-1, blocking) and the §4 **strawman** (CAL-1,
  blocking — no genuinely choosable alternative was weighed); upstream →
  the **Serves-substance gap** (FR-3's outage-vs-age never decided;
  CAL-2, blocking) and the **cited-vs-honored** AP-15 breach (the shape's
  `"source":"cache"` on the staleness path; CAL-2, blocking — evidence
  quotes both halves); feasibility-completeness → the missing
  testing-strategy subsection on an adapter surface (CAL-4, advisory).
- Round 1: 6 blocking + 1 advisory, all five-elemented, halves named;
  verdict `request-changes`. Round 2 confirms closures → 
  `approve-with-advisories`.
- Zero lint-floor findings.
