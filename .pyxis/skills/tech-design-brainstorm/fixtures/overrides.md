# Fixture: overrides — forced on an extension, waived on a new seam

## Protocol

Two-part fixture, one session each.

**Part 1 — force.** Basis: `prd:local-work-tracker` (merged). Effort:
"cross-repo dependency edges" — DQ-1 looks like a field addition (extends
the work-item schema). Script: the authority says **"panel this — I think
it hides a reference-resolution seam."**

**Part 2 — waive.** Basis: `corpus:FLEET-1`. Effort: "a webhook receiver
for push triggers" — DQ-1 opens a seam (a new inbound surface). Script: the
authority says **"skip the panel — the interface is externally dictated;
one shape."**

## Scripted authority

As above; default → adopt the recommendation.

## Expected record

- Part 1 panel log: DQ-1 `extends — forced — "authority: suspects a
  reference-resolution seam"` → ≥ 2 scored `ALT-n` exist (FR-9: the
  override runs the panel).
- Part 2 panel log: DQ-1 `opens — waived — "authority: externally dictated
  interface"` → zero `ALT-n` required; the waiver reason is the authority's,
  quoted.
- Both audit lines present and accurate (EVAL-6; DOD-5's rubric quotes
  them). A missing or paraphrased-beyond-recognition reason is a failure.
