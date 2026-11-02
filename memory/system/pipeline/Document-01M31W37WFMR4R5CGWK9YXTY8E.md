# pyxis pipeline — raw idea → verified feature

The pyxis factory turns a product idea into merged, verified software through
an ordered pipeline of stages. This file is the **operational map**: what each
stage produces, the skill or mechanism that runs it, where the human gates are,
and what is actually built today.

It is **descriptive, not governing**. The canonical definition of the pipeline
is pyxis's own `docs/product-vision.md` §4 (present in the pyxis repo); on any
conflict, §4 governs and this file is corrected to match. It lives here so the
pipeline travels with the distributable agent surface (`.pyxis/`): a repo that
installs pyxis gets the skills that run these stages *and* the map they belong
to.

**Actors.** *Factory* = an automated stage a skill or the engine runs.
*Human* = a human-in-the-loop gate. The human gates are stages **2, 5, and 9**
(plus authoring the contract); every other stage the factory runs unattended.

**Status legend.** ✅ built · ◐ partial · ⏸ deferred · ⛔ not built (needs the
engine). Each built stage's method guide is `.pyxis/authoring/<skill>.md`.

## The stages

| # | Stage | Actor | Produces | Skill / mechanism | Status |
|---|---|---|---|---|---|
| 1 | Feature definition | Factory | A per-feature **PRD** contract | `/feature-brainstorm` → `/prd-new` | ✅ |
| 2 | Feature review | Human | The **approved PRD** | `/prd-review` (unmanned, advisory) + human approve + merge | ✅ \* |
| 3 | Technical-design brainstorm | Factory ↔ Human | Candidate design directions | `/tech-design-brainstorm` | ✅ |
| 4 | Technical design | Factory | The **technical-design doc(s)** | `/tech-design-new` | ✅ |
| 5 | Technical review | Human | The **approved design** | `/tech-design-review` (unmanned, advisory) + human approve + merge | ✅ \* |
| 6 | Contract compilation | Factory | The contract compiled to executable form (requirement + verification graphs, risk, readiness verdict); ambiguous contracts bounce back with named defects | `docgate contract-gate` (the mechanical gate; plangate's precheck enforces it as the planning-entry precondition, and a continuous mode exists) | ◐ mechanical gate built; the LLM-judgment layer (semantic + analytical checks) not built (#419) |
| 7 | Implementation planning | Factory | **Milestones, epics, tasks** + dependency graph + frozen charters | `/plan-new` (+ `plangate`) | ✅ |
| 8 | Implementation | Factory | Per task: code + tests + doc updates + a PR carrying its evidence package | frozen plan projected onto the tracker (repo-configured automated-implementation tag); the pyxis-native local engine (`/implement-now`) or an external fleet claims and implements | ◐ native local engine (`/implement-now`) landed + external fleet; native cutover earned per the dogfood (RW-6) |
| 9 | Implementation review | Human | Reviewed code + artifacts per task | `/impl-review` (unmanned, advisory) at a proposal's ladder-green boundary + human code-host review + merge | ✅ \* |
| 10 | E2E verification | Factory | Every PRD requirement verified end-to-end; no regressions | — | ⛔ |
| 11 | Periodic maintenance | Factory | Drift, dead code, and refactor findings against the live docs/policy | — | ⛔ |
| 12 | Incident management | Factory | A captured lesson in the append-only incident registry | `/incident` | ◐ capture built; detection is engine |

\* **Advisory period.** `/prd-review`, `/tech-design-review`, and
`/impl-review` each complete a full verdict unmanned, but during the advisory
period their verdicts **block nothing** — findings accumulate as measurement
until each reviewer's adoption checkpoint (ten reviews for the document
reviewers; the impl reviewer's ramp per its own design). Today the human
approval + merge is what gates stages 2, 5, and 9; merge is the freezing event
for the PRD ledger and the design.

## Where a problem enters

- A **raw idea** whose decisions aren't settled yet → start at
  `/feature-brainstorm`. It grills the idea into a gated brainstorm record;
  `/prd-new` refuses an ungated one.
- An idea already shaped → go straight to `/prd-new`.
- Everything downstream chains from the **approved PRD**: design brainstorm →
  design → (compile) → plan → (engine-run) implementation → verification.

## Cross-cutting skills

- **`/prd-amend`** — evolve an already-approved (frozen) PRD by a typed
  amendment when building or review teaches something. Frozen PRDs are never
  hand-edited; the ledger only grows (`authoring/prd-amend.md`).
- **`/help`** — the ecosystem's front door: derives what this repo's pyxis
  surface offers and routes a described problem to its entry, live on every
  invocation, holding no catalog of its own. Bare `/help` lists the
  capabilities; `/help <problem>` names the command that owns it; `/help start`
  gives a newcomer the first step. Tied to no stage — it points at all of them
  (`authoring/help.md`).

## Shared records and execution diagnostics

Record decisions, approvals, workflow state, and retained evaluation evidence
in their owning shared Git artifacts. Carina work items, Argus ledgers, review
reports, feedback, and review/recovery/routing evidence remain shared.

Per-run diagnostics live under ignored `.pyxis/logs/<producer>/`.
Historical session decision records remain tracked; empty sessions create no
documentation. The binding policy lives in AGENTS.md, "Shared records and
local execution logs".

## What "not built" means

Stages 10–11 are **designed but not yet executable in pyxis**: the engine
parts that verify end-to-end and run maintenance do not exist yet — those
stages run outside pyxis today (by hand or an external factory). Stage 6's
**mechanical** contract gate is live (`docgate contract-gate`, enforced at
planning entry by plangate's precheck); its LLM-judgment layer remains open
(#419). Stage 9's unmanned reviewer (`/impl-review`) is live on the advisory
ramp, invoked at a proposal's ladder-green boundary; the human code-host
review + merge remains the gate. **Stage 8 has a pyxis-native local engine** (`/implement-now`,
the local-synchronous executor) alongside the external fleet: a frozen plan
projects onto the tracker with the repo-configured automated-implementation
tag, and either the native engine (same machine, driven by the operator or
argus) or an external fleet claims and implements each task. The native
engine is implemented and hermetically verified; native cutover is earned per
the dogfood (RW-6) — the external fleet remains the proven path until then.
This map is the live,
PR-corrected surface that owns that operational binding; when the engine era
changes it, the row moves here. The front half — raw idea → approved PRD →
approved design → frozen plan — is the pipeline pyxis runs itself, through the
skills above.
