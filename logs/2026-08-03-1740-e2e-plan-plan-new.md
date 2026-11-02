# Session log — authoring the e2e plan for plan-new

Started: 2026-08-03 17:40 · CLI: Claude Code · Stage or skill: /e2e-plan

## P-1 — the contract gate refuses planning, and the only fixes it names are edits I am not allowed to make

- **Found:** 2026-08-03 17:41
- **Problem:** The planning-entry gate for this feature fails. Two approved,
  merged design documents still say they are built on an older design that has
  since been replaced, so the gate reports the feature's contract as out of
  date and stops. The gate offers two ways out: replace each of those two
  documents with a successor, or correct the line in each that points at the
  old design. Both documents were frozen when they merged, and the repository's
  rules forbid an automated author from changing a frozen design; the tooling
  blocks it as well. Writing successors is a different stage of the pipeline
  and a product decision, not something this session can settle.
- **Blocked:** No plan can be written. Both the plan skill and the e2e plan
  skill state plainly that a failing gate means producing nothing at all — not
  a draft, not a partial plan. The failure has nothing to do with the work I
  was asked to do; it comes from the feature's own design set having moved on.
  Everything else was ready: the approved design document passed its own gate,
  and the shape of the plan it implies is fully determined.
- **Asked:** 2026-08-03 17:45 — I put the choice to the operator: hand the
  problem back to whoever owns the feature's designs and stop here, or have
  them decide how the two stale documents should be brought up to date, and
  whether that is one successor document or two.
- **Decided:** Still waiting.
- **How:** Nothing was written. The refusal was recorded as the line's own
  step event, with the outcome marked as refused.
- **Refs:** `go run ./cmd/plangate precheck plan-new` (exit 1, check
  `currency`); offending designs `docs/tech-design/plangate-carina-adapter.md`
  and `docs/tech-design/plangate-slice-coverage.md`, both `Builds-on:
  docs/tech-design/plan-new-v1.md`; `plan-new-v1` is
  `superseded → plan-new-routed-demand` per `docs/tech-design/index.md:59`;
  gate logic `internal/docgate/contractgate/reg_check_currency.go:54`;
  rule `prd:contract-gate/FR-2`, `prd:plan-new/FR-27`; guides
  `.pyxis/authoring/e2e-plan.md` step 1 and `.pyxis/authoring/plan-new.md`
  step 0. Subject document `docs/e2e/plan-new/design.md` — gate passed
  (`go run ./cmd/e2e-design check` reports well-formed; approved;
  21 scenarios).
