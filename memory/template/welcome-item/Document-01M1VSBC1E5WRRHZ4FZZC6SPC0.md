# Welcome item — template

*Guidance — this template (`welcome-item/v1`) is the welcome work item's
body. `pyxis init` resolves every substitution token from the captured knob
answers and paths, then files everything below the horizontal rule as a work
item in the wired tracker (`{{tracker}}`); in every mode the same next steps
also print to the terminal. Everything above the rule is template matter and
never ships.*

---

# Welcome — your factory is installed and green

This repository just converged onto a pinned pyxis release: structure
seeded, hooks live, tracker `{{tracker}}` wired. {{gates_active}}

The code gate is pending: no verification manifest exists yet, so a task's
code changes cannot clear it until one does. Once this repository holds
code, seed one with `pyxis exec codegate -- seed` — it detects your stack
from marker files and writes a template manifest for you to own; `pyxis
doctor` names the gap until then.

The docs gate passes today on seeded skeletons — two authoring duties stand
between you and your first shipped feature.

## 1. Author your governing documents (start here)

Governance is yours to write; no tool will ever write it for you:

1. `docs/product-vision.md` — the north star: what this product is for and
   what winning looks like. Author by appending below its authoring line,
   per the guidance inside (the file is append-only from birth).
2. `docs/architecture-principles.md` — the citable rules your designs will
   be judged against, under permanent IDs. Same append-only discipline.

Then the living maps, edited in place as understanding grows:

3. `docs/domain-model.md` — entities, relationships, and the invariant
   registry.
4. `docs/system-design.md` — the narrative map of how the system works and
   its decision registry.

`pyxis doctor` lists whichever of these are still unauthored skeletons.

## 2. Ship your first feature

The pipeline runs from your terminal, one stage at a time:

1. `/feature-brainstorm` — grill the raw idea into a gated brainstorm
   record.
2. `/prd-new` — instantiate the PRD ledger at
   `{{prds_path}}/<feature>/000-base.md` and fold its projection.
3. `/prd-review` — the unmanned reviewer's verdict; then human approve and
   merge (merge freezes the contract).
4. `/tech-design-brainstorm` → `/tech-design-new` — explore the solution
   space, then author the design at `{{tech_designs_path}}/<slug>.md`.
5. `/tech-design-review` — the design-class verdict; human approve and
   merge.
6. `/plan-new` — decompose the approved contract into a frozen, projected
   plan.
7. `/implement-now` — the local engine claims the plan's ready tasks and
   opens green proposals; a task's code changes clear the code gate once
   its manifest is seeded (see above).

## If something looks off

- `pyxis doctor` — read-only: drift by ownership class, environment health,
  epoch, advisories.
- `pyxis converge` — repair engine-owned drift (`--preview` first shows the
  exact write set).

Close this item once the governing documents are authored and the first
feature is through its PRD gate.
