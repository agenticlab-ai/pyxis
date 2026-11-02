# PRD — Design & Planning Pipeline

**Status: draft v0.1 (baseline inventory — as built in harbormaster) · 2026-07-01**
Part of the [Pyxis PRD set](README.md); serves the [product vision](../product-vision.md). Northstar categories: C1 (design & knowledge
capture), C2 (work management). Pipeline stages 3–5 + 7 (brainstorm → design →
review → plan); stages 1–2 (feature definition/PRD) and 6 (contract
compilation — VNEXT-1) have **no harbormaster ancestor** — harbormaster's
pipeline starts at the design doc and plans straight from it.

## Scope

Turning intent into dependency-ordered, executable work: brainstorming
approaches, authoring immutable design docs, human review, decomposition into
epics/tasks with charters, routing, and backlog triage. Users: **engineers**
(author/review), the **factory** (decompose/triage/route).

## Features

### PLAN-1 — Brainstorm stage
**What:** Divergent pre-design exploration (`/brainstorm`).
**Requirements (as built):**
- Generate 2–3 genuinely distinct approaches for a problem; score each against the design principles **and matched past incidents** ("how does this one die? which incident does it repeat?").
- End with an explicit recommendation + handoff to the design stage.
- Zero tracker writes — brainstorming precedes the artifact.
**Origin:** brainstorm skill; design-principles-and-feature-design.md.

### PLAN-2 — Design authoring
**What:** Draft a design doc from a problem statement per the house template
(`/feature-design`).
**Requirements (as built):**
- Reads the principles; discovers related prior docs (Builds-on lineage); writes the doc with the required header table (Status/Owner/Date/Relevant-when/Scope/Tracking/Builds-on) and a required **Principles** section (cite honored/deviated + reasoning).
- Opens the doc as a PR for human review; creates **zero** epics/issues (design iteration must not churn the tracker).
- Hands off to planning on acceptance.
**Origin:** feature-design skill; design-stage-memory.md; design-doc-gate.md.

### PLAN-3 — Design review loop
**What:** Rendered, inline-annotatable human review of a design doc
(`/design-review`).
**Requirements (as built):**
- Codified markdown→HTML transform (never hand-authored HTML); opens in the browser via the self-hosted review surface; polls the reviewer's inline annotations; applies each to the source doc by **propose-and-confirm** (ambiguous → clarifying question, never a silent edit); re-renders after edits.
- Zero GitHub writes / zero git mutations of its own; edits land via the normal finalize path.
- Degrades gracefully when the live loop is unavailable (static-open → render-only), always logging the reason; never a silent no-op.
**Origin:** design-review.md; design-review-self-host.md.

### PLAN-4 — Feature planning (decomposition)
**What:** Decompose an accepted design into executable, dependency-ordered work
(`/feature-plan`).
**Requirements (as built):**
- Create an epic + sub-issues with dependency edges (blocked_by); every sub-issue is atomic (single-proposal-sized).
- Stamp each sub-issue with a **spec (worker charter)** as structured data: goal, acceptance[], scope, type, deps, definition-of-done, design_ref anchor, capability rung, runtime override. Judged once at plan time, frozen; workers read the spec, not prose.
- **Conflict-magnet prevention**: tasks declare isolation_files; shared-file collisions are sequenced, and shared append-lists are structured line-per-item so parallel edits merge cleanly.
- Single-repo invariant per task (a task never spans clones).
- Default kickoff after planning (local routing), with plan-only / GitHub overrides.
**Origin:** feature-plan skill; local-multiagent-rig.md; conflict-magnet-prevention.md; feature-plan-default-kickoff.md.

### PLAN-5 — Plan umbrella (multi-epic features)
**What:** Group N epics under one plan for a single coordinated feature.
**Requirements (as built):**
- A plan issue links member epics; kickoff fans out across all of them.
- Epic-level dependency edges sequence foundational → downstream clusters.
- A terminal **verification epic**, blocked by all others, validates the design end-to-end before the plan closes (validated-done).
**Origin:** plan-umbrella.md.

### PLAN-6 — Kickoff routing
**What:** Route planned work to an execution path
(`/feature-implement-local`, `/feature-implement-github`).
**Requirements (as built):**
- Local path: label sub-issues for the autonomous pool. GitHub path: post `@claude` so hosted CI implements. Routing decided at plan time; standalone skills = late/manual re-routing.
- Idempotent; closed sub-issues skipped; a plan ID fans out over member epics.
**Origin:** work-order-routing.md; feature-implement-* skills.

### PLAN-7 — Completion monitor
**What:** Non-blocking progress reporting after kickoff.
**Requirements (as built):**
- A background monitor reports sub-issue closes, worker-failed proposals, and the final all-implemented event in-session, without blocking or polling by the user.
**Origin:** feature-plan-completion-monitor.md.

### PLAN-8 — Backlog triage
**What:** One sweep over a repo's open backlog applying explicit dispositions
(`/issue-triage`).
**Requirements (as built):**
- Visit every open, unblocked, undecided issue; apply exactly one disposition — plan / operator / reject (+ structural skip) — with a reasoning comment and a recorded marker; marker-gated so the steady state is a cheap no-op; `retriage` forces a fresh look; reopened-after-reject escalates to operator (never silently re-closed).
- Serialized by a per-issue claim lock so concurrent sweeps never double-decide.
- Issues that name a broken prior decision also get an **incident classification** routed to incident capture.
- A **foreign-PR pass** sweeps PRs from non-factory authors: adopt / operator / reject under the same marker + lock protocol.
- `--dry-run` prints dispositions with zero writes.
**Origin:** issue-triage.md.
