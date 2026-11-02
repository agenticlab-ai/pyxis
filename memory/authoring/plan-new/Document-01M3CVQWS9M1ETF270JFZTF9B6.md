# plan-new — decompose a binding contract into a frozen plan

Author a **plan** — the stage-7 artifact: a binding feature contract
(approved documents or a locked draft) decomposed into dependency-ordered,
vertically sliced tasks with frozen charters, projected onto the tracker only after
a human freezes it. The contract is `docs/prds/plan-new/` (the PRD) +
`docs/tech-design/plan-new-v1.md` (the design); the deterministic half is
the `plangate` binary — every enumerable rule is its verdict, never your
recollection. Input: the feature slug; ask if missing.

## Steps

0. **Gate the binding contract — the planning-entry precondition.** Before
   drafting, freezing, or projecting, run `go run ./cmd/plangate precheck
   <slug>`. If `docs/prds/<slug>/000-base.md` exists, this runs the document
   contract gate (FR-27). Otherwise it fetches current mainline and the
   latest binding lock, then rechecks that locked version's readiness against
   current memory and documents (feature-draft FR-31). Its successful output
   is the exact `lock/<slug>/vN` Basis to put in the plan. An open reopened
   draft is not a planning input; its last locked version remains binding.
   On any defect, **refuse to proceed:** produce no draft plan or projection;
   hand the named defects back to the contract owner (FR-16).

1. **Gate the inputs (FR-1).** For a document-pipeline feature, the governing
   PRD ledger must be folded (`docs/prds/<slug>/effective.md` exists) and
   the design merged with Approved status. For a draft-born feature, use
   only the locked PRD and tech-spec Documents named by the Basis returned
   in step 0. No lock means no plan. Check for an existing plan
   (`docs/plans/<slug>/`) and an open epic covering the feature
   (`plangate report docs/plans/<slug>`, or a tracker scan): extend or
   supersede deliberately, never duplicate (FR-3).
2. **Read the template fully**: `.pyxis/templates/plan-template.md` (the
   grammar skeleton + field notes). Read the binding PRD and design end to
   end — `effective.md` plus the merged design for a document-pipeline
   feature, or the literal Documents at the lock for a draft-born feature.
   Build the coverage matrix against their live units and anchors.
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
   requirement IDs and design anchors it serves. New plans use
   `plan-template/v2`. A draft-born plan's header uses `Basis: lock/<slug>/vN`,
   `PRD: prd/<slug>`, and `Design: design/<slug>`; a document-pipeline plan
   retains its document paths and omits Basis. Run `go run ./cmd/plangate dimensions` to read enum values
   and the derivation policy. Author a `Requirements` JSON object for each task
   with separate implementation and review assessments. Supply reasoning,
   coupling, novelty, verification, impact_scope, and architectural_role, plus
   citations and a rationale. Cite `task:goal`, `task:scope`, `task:acceptance`,
   or one of the task's declared Traces. Fold derives capability, deliberation,
   context, and assurance and records applied rules and policy identity. An
   authored profile, if supplied, must match that derivation exactly. Mechanical
   tasks require the same complete assessment. Do not author harnesses, models,
   effort, rungs, or tiers. Old plan/v1 files remain compatibility inputs only.
5. **Fold until green**: `go run ./cmd/plangate fold docs/plans/<slug>` —
   defects name document/rule/reason/remedy; fix the plan, never argue
   with the gate (thresholds are config data; a genuinely atomic oversize
   is a recorded `Size accepted:`). Commit `<NNN>-plan.md` and
   `<NNN>-plan.json` together.
6. **The checkpoint (FR-17)**: raise the plan PR on `plan/<slug>-<NNN>`,
   one concern. Review both assessments, every derived requirement, citations,
   and rationale alongside the charter. Changed dimensions require review of
   the changed plan; provider selection belongs to execution.
   Review the source plan with the operator, optionally using
   `plangate render` for an offline preview; apply confirmed edits. The operator's merge is the
   approval and the freezing event. Nothing exists on the tracker yet.
7. **Project (after merge)**, staged per the PRD's rollout modes:
   `go run ./cmd/plangate project docs/plans/<slug> --dry-run` (read the
   write set) → `--no-routing` (issues + wiring, invisible to dispatch)
   → full (routing applied last). The report prints and lands on the
   epic; `plangate report` regenerates it from tracker reality anytime.
   A duplicate-marker halt names both issues — close one by hand, re-run.

## Never

Draft, freeze, or project a plan whose binding contract `plangate precheck <slug>`
reds — the non-clean verdict is the bounce, produce zero
plan output (FR-27); hand-edit `<NNN>-plan.json` (fold owns it); project an
unmerged or edited plan (projection refuses — DINV-1); paper over fog with a
vague task
(bounce the contract instead); put file paths or line numbers in charter
text (Touches only); renumber or reuse task IDs once the plan PR is open
(plan-task-id is the idempotency key); close or edit tracker items the
projection did not create; route around a red dialect contract test.
