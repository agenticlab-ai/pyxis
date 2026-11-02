# PRD — Knowledge & Documentation System

**Status: draft v0.1 (baseline inventory — as built in harbormaster) · 2026-07-01**
Part of the [Pyxis PRD set](README.md); serves the [product vision](../product-vision.md). Northstar category: C1 (design & knowledge
capture). Pipeline stages 1–6 produce into it; 11–12 maintain it.

## Scope

Where decisions live so humans *and agents* can act on them: the immutable
decision record, the living current-state layer, the failure memory, and the
principles distilled from it. Users: **engineers** (author/consume), the
**factory** (consume at design/plan/review time, maintain the living layer).

## Features

### KNOW-1 — Immutable design docs (decision deltas)
**What:** Every mechanism decision is a frozen, citable point-in-time record.
**Requirements (as built):**
- House template with a required header (Status / Owner / Date / Relevant-when / Scope / Tracking / Builds-on) and a required **Principles** section (which principles this design honors or deviates from, with reasoning); linted locally and in CI.
- Immutable after merge: a changed decision is a **new** doc citing the old via Builds-on — the rationale trail is never rewritten. *(Pyxis extends the same contract to per-feature PRDs — AP-7.)*
**Origin:** design-stage-memory.md; check-design-doc.sh.

### KNOW-2 — Corpus discovery & routing
**What:** A 50+-doc corpus stays navigable for agents.
**Requirements (as built):**
- Every doc carries a one-line **Relevant-when** ("when to open me"); a grep-able routing menu turns "which doc governs X?" into one lookup.
- **Builds-on** edges form the lineage DAG (what supersedes/extends what).
**Origin:** design-doc-discovery.md.

### KNOW-3 — Living docs (current-state layer)
**What:** Two rollups absorb change so the immutable layer never has to lie.
**Requirements (as built):**
- `techspec.md` (per-subsystem *Current* state + *Decided-by* citations) and `prd.md` (live capability inventory) — rationale stays in design docs; the living layer holds only "what is."
- **A single coded drift signal** ("does this diff touch design-owned code?") shared by every consumer — the finalize step, the autonomous worker path, and incident capture (one predicate, one place).
- A **sole writer** (the reconcile flow) applies living-doc edits: propose-and-confirm interactively, direct-commit on the autonomous worker path (the worker pauses with a reconcile-pending state, reconciles, then finishes).
- Advisory by design — living-doc completeness never blocks a merge; CI checks only forward link integrity.
**Origin:** living-docs-layer.md; worker-path-reconcile-docs.md.

### KNOW-4 — Incident registry (failure memory)
**What:** Paid-for lessons are captured once and retrieved at design time.
**Requirements (as built):**
- Append-only entries with stable citable IDs (never renumbered), subsystem **tags** for grep retrieval, and a four-field discipline: Decision / Broke / Lesson / Links (~6 lines).
- One coded writer (ID allocation is code, never prompted); capture is handoff-invoked: triage's incident classification, fix-time finalization, or ad hoc.
- Lessons that stabilize **graduate** into principles (entry gains a Graduated pointer). *(Pyxis strengthens the link contract: each incident links the introducing technical design and implementing PR(s) — AP-7.)*
**Origin:** incidents registry; incident skill; incident-add.sh.

### KNOW-5 — Design principles
**What:** The stable, citable rule set distilled from incidents.
**Requirements (as built):**
- Numbered principles (P0–P11), never renumbered or edited in place; every design doc must cite the ones it honors or deviates from — deviation requires stated reasoning.
**Origin:** design-principles.md.

### KNOW-6 — Northstar (strategic map)
**What:** The org-level reference everything hangs off.
**Requirements (as built):**
- Human-authored input (never derived from the governed system): the north-star statement — *an engineer touches a feature exactly twice: author the contract (PRD + design + definition of done), accept the result* — the inversion test (any other engineer-minute is a factory bug), the complete primitive taxonomy (C1–C11) with a scored current-state, trigger-based roadmap horizons, and the metrics (§ OBS / prd.md).
**Origin:** factory-northstar.md.

### KNOW-7 — Operator references
**What:** The catalogs that make the system operable without spelunking.
**Requirements (as built):**
- An events reference documenting every observability event kind (what fires it, key fields, required action); operator setup guide; per-repo agent instruction files (CLAUDE.md / AGENTS.md) describing how to work in each repo.
**Origin:** events-reference.md; operator-setup.md.
