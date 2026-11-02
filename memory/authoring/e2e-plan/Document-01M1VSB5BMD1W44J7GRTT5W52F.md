# e2e-plan — turn an approved e2e design doc into a plan-new plan

Author the **plan** half of the e2e authoring line: read one feature's
**approved** `e2e-design/v2` doc (`docs/e2e/<feature>/design.md`) and write
an ordinary plan-new **`plan/v1`** for it — one task per scenario, plus a
harness task only where the feature's e2e harness does not already exist and
a conventions task only where no scenario is built against it yet —
then hand the result to plan-new's own pipeline, unchanged
(`docs/tech-design/e2e-verification-skills-v2.md` §3.3;
`docs/tech-design/e2e-verification-artifacts-v2.md` §3.2). The task-graph rule
(the two conditional tasks, the scenario `Blocked-by` edges, and each
scenario's exclusively-owned artifact) is fixed and tested at
`internal/e2e/plan` (`DecideWithConventions`) — this guide applies the same
rule; that package is the citable source of truth if the two ever disagree.
Everything else — the template, the fold, the PR, the freeze, the
projection — **is** `.pyxis/authoring/plan-new.md`, run verbatim; read that
guide too. Input: the feature slug (or a path to its design doc); ask if
missing.

## Steps

0. **Gate the e2e design doc — e2e-plan's own precondition (DINV-6).**
   Read `docs/e2e/<feature>/design.md` and parse it as an `e2e-design/v2`
   document (`internal/e2e/docclass`). The whole gate is `Decide`'s, and it
   is the whole class contract, not the `Status` cell alone: the header
   `Template version` must name **this** class, `Status` must read exactly
   `approved`, and `docclass.Check` must report **no defect**. **Refuse on
   anything else** — `draft`, missing, unreadable, unparseable, a successor
   or foreign class, or a doc the checklist rejects — produce no plan, and
   name the missing approval verbatim (`prd:e2e-verification/UB-4`).
   Reading an approval off a document you never verified is the class of
   document is a closed contract read leniently at its consumer; a repeated
   header row is a checklist defect for exactly that reason (the first
   occurrence governs, so a second `Status` row can never override the one
   a reader sees). This gate is additional to, never a replacement for,
   plan-new's own step 1 below.
1. **Run plan-new's own steps 0 and 1, unchanged**: `go run ./cmd/plangate
   precheck <feature>` (the feature's PRD ledger must be folded and its
   governing design(s) merged; a non-clean verdict refuses, exactly as
   plan-new), then check for the feature's existing plan(s) under
   `docs/plans/<feature>/` — the e2e plan **extends** that history as the
   next dense-numbered plan (`002-plan.md` if `001` already exists for the
   feature's implementation), never a duplicate or a fresh `001`. A
   feature that reached e2e-design at all already had built code against
   a clean contract, so the precheck should already be green — but a
   later amendment could have reopened it, and this refuses exactly as
   plan-new would.
2. **Decide the task graph — fixed, not judgment.** This is the one place
   e2e-plan replaces plan-new's own step 3 (open decomposition) with a
   closed rule (`internal/e2e/plan.DecideWithConventions`; `#DINV-5`):
   - **One task per scenario**, in the order the design doc's Scenarios
     section declares them.
   - **Determine whether the feature's e2e harness already exists** — a
     repo-inspection judgment, not a fixed path lookup: read the design
     doc's Harness libraries section (the named harness + why it fits) and
     check whether that harness's support code is already present in the
     repo. Record the finding (yes/no, and where) — this mirrors
     e2e-design's own app-shape reasoning: authored judgment, never
     skill-code inferring a convention that does not exist yet.
   - **If the harness does not exist**: emit exactly one additional
     harness task, first in the plan (`T1`), and give every scenario
     task's `Blocked by` field that task's ID
     (`prd:e2e-verification/FR-16`).
   - **If the harness already exists**: emit zero harness tasks, and no
     scenario task's `Blocked by` field names one
     (`prd:e2e-verification/FR-17`).
   - **Determine whether the repository already carries a scenario built
     against that harness** — a second, separate repo-inspection judgment,
     asked of the harness the previous step settled. Record the finding
     (yes/no, and which scenario).
   - **If no scenario exists against it**: emit exactly one **conventions
     task**, and give every scenario task's `Blocked by` field that task's
     ID as well (`prd:e2e-verification/FR-25`). Its condition is its
     **own** — a feature whose harness already exists but which has no
     scenario against it yet still owes this task, so do not fold the two
     judgments into one.
   - **If a scenario already exists against it**: emit zero conventions
     tasks, and no scenario task's `Blocked by` names one — the shape the
     later tasks would copy is already in the tree.
   - The conditional tasks are emitted first, harness before conventions,
     so their IDs are stable and citable — `internal/plangate/plan`'s
     parser requires dense, ascending task IDs from `T1`, and the
     executor's ready-frontier (open ∧ unblocked ∧ unclaimed) is what
     actually sequences conventions-before-scenarios; the edge is all this
     skill declares. When both are emitted, the conventions task is itself
     `Blocked by` the harness task: there is no scenario contract to land
     until the harness it belongs to exists.
   - **Carry each scenario's owned artifact onto its task.** Every scenario
     task names the **one artifact it exclusively owns**, taken verbatim
     from that scenario's row in the design doc's Scenario build facts
     table — never invented here. Two scenario tasks in one plan must never
     name the same artifact: exclusive ownership is what lets the executor
     run them in parallel without two workers editing one file
     (`prd:e2e-verification/FR-26`, `#DINV-7`). A document whose scenarios
     do not each name exactly one distinct artifact is **refused**, not
     planned around.
   - **A degenerate input is refused, never planned around.** `Decide`
     refuses an approved doc that declares **no scenario** — a feature with
     no automatable e2e surface stops the line (`prd:e2e-verification/UB-7`,
     `#DINV-7`), and a task-less plan is one `plangate` rejects outright —
     and refuses a scenario row carrying **no ID**, which leaves nothing to
     name the task after and nothing for a coverage row to cite. Which KIND
     each slot is — harness, conventions, or scenario — is a decided field on
     the slot, never inferred from which of its other fields happen to be
     empty.
3. **Author** `docs/plans/<feature>/<NNN>-plan.md` (the next dense number
   from step 1) from `.pyxis/templates/plan-template.md`, exactly as
   plan-new step 4 — charters are durable behavior; file paths/surfaces go
   in `Touches` only, never in charter prose. Every task's charter
   **restates** content the approved design doc already carries and the
   e2e review already settled — it makes no new decision:
   - the harness task's Goal/Desired behavior/Key interfaces come from the
     Harness libraries section;
   - the **conventions task** lands two things and says so in its charter:
     **the harness's scenario contract** — what one scenario IS as an
     artifact and where it lives, restated from the design doc's
     `scenario-contract` slot — and **one worked scenario** built against
     it, chosen as the first scenario the Scenarios section declares. Its
     Desired behavior is that a worker opening any later scenario task
     finds a landed example to copy rather than an empty package, and its
     Acceptance is that the worked scenario runs green under the harness.
     Name the worked scenario's owned artifact in its `Touches`. It makes
     no new decision: the contract is the design doc's, and the scenario is
     one the coverage map already routed;
   - each scenario task's Goal/Desired behavior comes from that scenario's
     Flow (Scenarios section), its recorded Mock strategy, and its
     Per-scenario route; a `manual-human` scenario's task names the
     route's recorded Owner;
   - every **scenario** task's `Traces` cites the feature's PRD requirement
     row(s) the Coverage map maps to that scenario (`prd:<feature>/FR-n`),
     resolved against the feature's own governing PRD — `Traces` is closed
     to `prd:` / `design:` / `corpus:` references that resolve
     (`internal/plangate/plan`'s `checkTraces`); never a citation to the
     e2e design doc itself, which that grammar does not admit.
   - the **harness** task has no scenario and therefore no Coverage-map row
     of its own, but `checkTraces` defects on a task with zero traces — so
     it cites the **union** of the requirement rows its dependent scenario
     tasks cite (it exists to make exactly those scenarios runnable), plus
     the governing design anchor for the harness decision
     (`design:<feature>-...#<section>`) where the design set names one. A
     harness task with an empty `Traces` is a plan that will not fold.
   - the plan's `Coverage` section maps each cited requirement to the
     tasks that carry it, per the ordinary plan-new rule — it need not
     re-achieve full feature coverage (the feature's original
     implementation plan already did; feature-level completeness is the
     union across all of a feature's plans, `plangate coverage`, never a
     single plan's burden).
4. **Fold, review, freeze, project — plan-new's own steps 5–7, run
   unchanged**: `go run ./cmd/plangate fold docs/plans/<feature>`; raise
   the plan PR on `plan/<feature>-<NNN>`; `/doc-review` it; the operator's
   merge is the approval and the freezing event; project per the PRD's
   rollout modes. Nothing here is e2e-specific — it is exactly plan-new's
   pipeline, reused (`prd:e2e-verification/NR-2`).
5. **Record the line-step event** (`prd:e2e-verification/§5.7` `EVT-1`,
   `e2e_line_step`): `feature_id`, `step=plan`,
   `outcome=produced|refused`, `tokens` — in the run log, on every
   invocation including a refusal.

## Never

- Author a plan from an e2e design doc whose `Status` is not exactly
  `approved` — refuse, name the missing approval, and write nothing
  (`#DINV-6`).
- Trust a `Status` cell on a document you have not verified is an
  `e2e-design/v2` document the checklist accepts — the class pin and
  `docclass.Check` are part of the same gate, not optional extras.
- Author a plan from a doc that declares no scenario, or from a scenario
  row with no ID — refuse both, naming the reason; a harness-only plan and
  a task-less plan are fabricated artifacts, not plans
  (`prd:e2e-verification/UB-7`, `#DINV-7`).
- Emit a harness task when the feature's e2e harness already exists, or
  omit one when it does not (`#DINV-5`).
- Emit a conventions task when the repository already carries a scenario
  built against that harness, or omit one when it does not
  (`prd:e2e-verification/FR-25`). Its condition is its own — never decide
  it from whether a harness task was emitted.
- Leave a scenario task without a `Blocked by` edge onto each conditional
  task that WAS emitted, or add an edge onto one that was not.
- Let two scenario tasks name the same owned artifact, or let a scenario
  task carry an artifact the design doc's Scenario build facts table does
  not give it (`prd:e2e-verification/FR-26`, `#DINV-7`).
- Invent a scenario, route, mock strategy, or harness choice the approved
  design doc does not already state — e2e-plan restates reviewed content,
  it never makes a new coverage decision (that is e2e-design's and
  e2e-review's job, already settled by approval).
- Add a new task type, a new charter field, or otherwise extend
  plan-new's grammar for these tasks — every task is an ordinary
  `internal/plangate/plan.Task`, nothing more
  (`prd:e2e-verification/FR-14`).
- Hand-edit `<NNN>-plan.json` (fold owns it); project an unmerged or
  edited plan; special-case e2e tasks in plan-new or the executor — both
  stay byte-unchanged (`prd:e2e-verification/NR-2`).
