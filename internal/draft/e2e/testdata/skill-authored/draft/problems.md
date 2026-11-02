# Requirement problems

### P-1 — FR-1 — Archiving contradicts input preservation

**Raised by:** design-brainstorm:widget-summary-tech-design, drafts/widget-summary/brainstorm/002.md · 2026-09-27

**Problem:** Current FR-1 requires returning Total “and archive every input widget.” The same draft's §1 says “Input remains unchanged”; NR-1 preserves caller-owned IDs and states after success; NG-3 excludes input mutation. An active input cannot both become archived and keep its state. The archive clause also has no fit criterion in FR-1 and no live stage-1 ruling: brainstorm:widget-summary#D-5 explicitly excludes mutation. This is a blocking product contradiction, not an implementation choice. The design side leaves FR-1 and all other PRD bytes unchanged. The product side must record an authorized Ruling on this problem before editing prd.md; then design rereads the ruled bytes.

**Ruling:** operator · Caller-owned widgets must remain available in their original state; this function only counts. · Replace only the FR-1 requirement sentence with “When valid input completes without cancellation, the widget summarizer shall return its number of entries as Total without modifying any input widget.” Keep every other FR-1 cell and every requirement ID unchanged. Live scripted product ruling, 2026-09-27; applied by the /prd-new open-draft product-ruling path after recording this ruling. Evaluation only; no production approval or lock.

### P-2 — UB-1 — Overlapping error outcomes affecting UB-2..UB-4

**Raised by:** prd-review and tech-design-review, reviews/001.md#FND-1 and reviews/002.md#FND-1 · 2026-09-27

**Problem:** A pre-cancelled context and 10,001 entries activate both UB-1's typed invalid-input error and UB-4's context error. The separate product decisions provide no precedence; D-1's context-first implementation cannot satisfy both obligations. Record the authorized product ruling before changing either contract. At this discovery the PRD remains unchanged.

**Ruling:** operator · The caller needs one deterministic error when cancellation overlaps invalid input. · Check cancellation at function entry, before each entry's validation, and before successful return. A nonnil observed context error ends the call with that error and Summary{}. If a check observes no cancellation, its subsequent length or entry validation may end the call with typed ErrInvalidWidget and Summary{}; later cancellation does not replace an already returned validation error. Qualify UB-1..UB-4 and D-1/A-2/A-3 accordingly and require combined-trigger cases. Live scripted operator event from the fixture host, 2026-09-27; evaluation only, no production approval or lock.
