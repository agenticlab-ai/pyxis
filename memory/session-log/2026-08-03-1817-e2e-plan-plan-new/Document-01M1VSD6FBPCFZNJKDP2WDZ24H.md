# Session log — authoring the e2e plan for plan-new (second attempt)

Started: 2026-08-03 18:17 · CLI: Claude Code · Stage or skill: /e2e-plan

Line-step event: feature plan-new, step plan, outcome refused.

## P-1 — the same gate still refuses planning, and the only way through is to write new design documents

- **Found:** 2026-08-03 18:19
- **Problem:** This is the second run at the same job, and the gate that has
  to pass before any plan may be written still fails for the same reason. Two
  approved, merged design documents for this feature still say they are built
  on an older design that has since been replaced, so the feature's contract
  reads as out of date. Nothing changed in the repository between the two
  attempts. This run also settled something the first one left open: of the
  two ways out the gate offers, only one is actually available. Correcting the
  line in each document is not just forbidden by the repository's rules — the
  tooling itself rejects any change to a merged design document, so the edit
  could never pass. That leaves writing replacement documents as the only
  path, which is a different stage of the pipeline and a product decision.
  Worth knowing for whoever picks this up: the same out-of-date pointer is
  carried by eleven design documents across the repository, not two. Only the
  two that serve this feature stop this particular job.
- **Blocked:** No plan can be written. Both governing guides say a failing
  gate means producing nothing at all. Everything else is ready and was
  confirmed this run: the approved design document passes its own gate, the
  feature has no plans yet so this would be its first, the end-to-end harness
  the design names does not exist for this feature yet, and the plan's shape
  is therefore fully determined — one harness task first, then one task per
  scenario, each waiting on the harness.
- **Asked:** 2026-08-03 18:26 — I put three choices to the operator: one
  replacement document covering both stale ones, two replacement documents
  one for each, or stop here and hand the problem back to whoever owns the
  feature's designs. I also noted that whether the replacements are still
  substantively correct under the newer design is a judgment I cannot make.
- **Decided:** Still waiting.
- **How:** Nothing was written. The refusal was recorded as the line's own
  step event above, with the outcome marked as refused.
- **Refs:** `go run ./cmd/plangate precheck plan-new` (exit 1, check
  `currency`); offending designs `docs/tech-design/plangate-carina-adapter.md`
  and `docs/tech-design/plangate-slice-coverage.md`, both `Builds-on:
  docs/tech-design/plan-new-v1.md`; `plan-new-v1` is `superseded →
  plan-new-routed-demand` per `docs/tech-design/index.md:59`; gate logic
  `internal/docgate/contractgate/reg_check_currency.go:42`; in-place edit
  blocked by `internal/docgate/gate/reg_check_immutable.go:47` (INV-41);
  `Supersedes` accepts multiple slugs and the one-live-successor rule is
  per-target, per `docs/tech-design/tech-design-conventions.md:125,215`;
  repo-wide `stale-builds-on` count from `docs/tech-design/index.md`;
  rules `prd:contract-gate/FR-2`, `prd:plan-new/FR-27`; guides
  `.pyxis/authoring/e2e-plan.md` step 1 and `.pyxis/authoring/plan-new.md`
  step 0. Subject document `docs/e2e/plan-new/design.md` — gate passed
  (`go run ./cmd/e2e-design check` reports well-formed; approved;
  21 scenarios). Harness absence: `internal/plangate/` has no `e2e` package,
  unlike `internal/installer/e2e`, `internal/executor/e2e` and
  `internal/implreview/e2e`, the three exemplars the design doc names.
  Prior attempt's log: `logs/2026-08-03-1740-e2e-plan-plan-new.md`.
