# Session log — freezing and projecting the tech-design-brainstorm e2e plan

Started: 2026-08-04 20:10 · CLI: Claude Code · Stage or skill: /e2e-plan

Line-step event: feature tech-design-brainstorm, step plan, outcome produced.

## P-1 — the contract gate still refuses, and this session was told to freeze and put work on the tracker anyway

- **Found:** 2026-08-04 20:11
- **Problem:** The check that has to pass before any plan may be written,
  frozen, or turned into tracker work still fails for this feature, exactly as
  it did yesterday. Fifteen of its important requirements have no technical
  design behind them; the single design written for it covers the record file
  and says in its own scope line that it does not cover the conversation,
  which is where those fifteen live. Nothing in the repository changed
  overnight, so the check reds for the same reason and names the same
  requirements.
- **Blocked:** Both the planning guide and the end-to-end wrapper treat a red
  contract as a full stop covering all three acts — no draft, no freeze, no
  tracker work. Yesterday's ruling covered only the first of those, and that
  session deliberately stopped before raising anything or creating any work
  items. Freezing and projecting were therefore still forbidden when this
  session opened, and lifting that is not mine to do.
- **Asked:** 2026-08-04 20:10 — the operator opened this session by asking for
  exactly the two acts the earlier ruling had left out: freeze the tasks and
  create the work items.
- **Decided:** 2026-08-04 20:10 — proceed. The operator's instruction extends
  yesterday's ruling to cover the freeze and the tracker work.
- **How:** Everything except the failing check was verified first and all of it
  passed: the design document is approved and well-formed, its review closed
  with every finding resolved, the plan folds clean, and the task shape matches
  the fixed rule — one harness task first because this feature has no harness
  yet, then one task per scenario, each waiting on the harness. The plan was
  then committed and raised for the operator's merge, with the work items to
  follow once it lands. The underlying gap is untouched and still open.
- **Refs:** docs/plans/tech-design-brainstorm/001-plan.md ·
  docs/e2e/tech-design-brainstorm/design.md ·
  docs/tech-design/design-brainstorm-record-model.md ·
  `go run ./cmd/plangate precheck tech-design-brainstorm` (exit 1, check
  `inverse-coverage`) · `.pyxis/authoring/e2e-plan.md` step 1 ·
  `.pyxis/authoring/plan-new.md` step 0 and its Never list ·
  prior ruling in logs/2026-08-03-2116-e2e-design-tech-design-brainstorm.md P-2

## P-2 — the operator told me to merge my own work, which the house rules forbid

- **Found:** 2026-08-04 20:45
- **Problem:** The repository requires every change to be approved and merged
  by a person, and says plainly that an automated author never merges its own
  work. The merge is also the moment the plan freezes, so it is the one act
  that decides whether the tasks become real. With the checks green, the
  operator told me to merge it myself.
- **Blocked:** I could not follow both the rule and the instruction. Merging
  would break a standing rule; refusing would ignore a direct instruction from
  the person the rule exists to protect.
- **Asked:** 2026-08-04 20:30 — I had already said twice, unprompted, that the
  merge had to be theirs and explained why the work items could not exist
  before it.
- **Decided:** 2026-08-04 20:45 — merge it and carry on.
- **How:** Squashed it onto the mainline, deleted the branch, and confirmed the
  commit really had landed before doing anything else. Then projected the work
  items in stages.
- **Refs:** PR #972 · commit 24ab521 ·
  `.pyxis/authoring/plan-new.md` step 6 · the repository's own rule that every
  change needs a human approval and a human merge

## P-3 — whether to let the worker fleet start on a feature whose contract is red

- **Found:** 2026-08-04 21:05
- **Problem:** The work items were created and wired but deliberately held back
  from dispatch, which is the last stage of putting them on the tracker.
  Releasing them lets automated workers start building. Doing that on a feature
  whose contract check is still failing turns a paperwork override into real
  code being written against requirements no design covers.
- **Blocked:** Either answer could be the wrong one and the cost is real —
  holding stalls work the operator asked for, releasing starts building on a
  contract nobody has fixed. Not a call I should make quietly.
- **Asked:** 2026-08-04 21:08 — hold dispatch, or release it and let workers
  begin.
- **Decided:** 2026-08-04 21:10 — release it.
- **How:** Ran the final stage so the tasks became dispatchable. Only the
  harness task is unblocked; the other twenty open up once it lands.
- **Refs:** epic #973 · tasks #974–#994 ·
  `.pyxis/authoring/plan-new.md` step 7
