# plan-new — decompose an approved contract into a frozen plan

Author a **plan** — the stage-7 artifact: an approved feature contract
(PRD + technical design) decomposed into dependency-ordered, vertically-
sliced tasks with frozen charters, projected onto the tracker only after
a human freezes it. The contract is `docs/prds/plan-new/` (the PRD) +
`docs/tech-design/plan-new-v1.md` (the design); the deterministic half is
the `plangate` binary — every enumerable rule is its verdict, never your
recollection. Input: the feature slug; ask if missing.

## Steps

0. **Gate the contract (FR-27) — the planning-entry precondition.** Before
   you draft, freeze, or project anything, run the mechanical stage-6 gate
   over the feature's assembled contract: `go run ./cmd/plangate precheck
   <slug>` (which runs `docgate contract-gate <slug>` read-only). On any
   non-clean verdict it exits 1 and names every contract defect (an
   incomplete requirement, a stale or orphaned serving design, a dangling
   citation, a malformed join). **Refuse to proceed:** produce no draft, no
   plan, no projection — surface the emitted defect report to the operator as
   the bounce-back to the contract owner, exactly as a fog bounce hands the
   contract back (FR-16). Only a clean verdict permits planning to begin;
   this precondition runs before the input gate and everything below it.

1. **Gate the inputs (FR-1).** The governing PRD ledger must be folded
   (`docs/prds/<slug>/effective.md` exists) and the design merged with
   Approved status. A draft or missing input refuses the run — plans
   derive from human-approved contracts only. Check for an existing plan
   (`docs/plans/<slug>/`) and an open epic covering the feature
   (`plangate report docs/plans/<slug>`, or a tracker scan): extend or
   supersede deliberately, never duplicate (FR-3).
2. **Read the template fully**: `.pyxis/templates/plan-template.md` (the
   grammar skeleton + field notes). Read the PRD's `effective.md` and the
   design end to end — the coverage matrix is built against their live
   units and anchors.
3. **Decompose — the judgment half.** Tracer bullets: each task delivers
   ONE behavior end to end, demoable alone, sized to a fresh worker
   context. Prefactoring is its own task, sequenced first. Wide refactors
   use the declared expand → migrate-batch → contract pattern. Provenance
   work is quarantined into `Mechanical: yes` tasks. Apply the fog test
   per portion of the contract: *can you state the task precisely now?*
   If any in-scope portion is fog, STOP — produce no plan; hand back a
   fog report naming what is missing and what unblocks it (FR-16 is all
   or nothing; a partial plan is worse than an honest bounce).
4. **Author** `docs/plans/<slug>/<NNN>-plan.md` from the template (next
   dense number; 001 first). Charters are durable: behavior and
   interfaces, never file paths or line numbers — surfaces go in
   `Touches` (plan-internal, drives overlap sequencing, never rendered).
   Every ordering constraint is a `Blocked by` edge. Every task cites the
   requirement IDs and design anchors it serves.
5. **Fold until green**: `go run ./cmd/plangate fold docs/plans/<slug>` —
   defects name document/rule/reason/remedy; fix the plan, never argue
   with the gate (thresholds are config data; a genuinely atomic oversize
   is a recorded `Size accepted:`). Commit `<NNN>-plan.md` and
   `<NNN>-plan.json` together.
6. **The checkpoint (FR-17)**: raise the plan PR on `plan/<slug>-<NNN>`,
   one concern. Review the source plan with the operator, optionally using
   `plangate render` for an offline preview; apply confirmed edits. The operator's merge is the
   approval and the freezing event. Nothing exists on the tracker yet.
7. **Project (after merge)**, staged per the PRD's rollout modes:
   `go run ./cmd/plangate project docs/plans/<slug> --dry-run` (read the
   write set) → `--no-routing` (issues + wiring, invisible to dispatch)
   → full (routing applied last). The report prints and lands on the
   epic; `plangate report` regenerates it from tracker reality anytime.
   A duplicate-marker halt names both issues — close one by hand, re-run.

## Never

Draft, freeze, or project a plan whose contract `plangate precheck <slug>`
(the contract gate) reds — the non-clean verdict is the bounce, produce zero
plan output (FR-27); hand-edit `<NNN>-plan.json` (fold owns it); project an
unmerged or edited plan (projection refuses — DINV-1); paper over fog with a
vague task
(bounce the contract instead); put file paths or line numbers in charter
text (Touches only); renumber or reuse task IDs once the plan PR is open
(plan-task-id is the idempotency key); close or edit tracker items the
projection did not create; route around a red dialect contract test.
