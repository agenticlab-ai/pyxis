# Session log — closing the implementation-review findings on T11

Started: 2026-08-05 19:01 · CLI: Claude Code · Stage or skill: /impl-review follow-up on PR 1013

## P-1 — the review asks for a scope call the task charter never made

- **Found:** 2026-08-05 19:20
- **Problem:** The injection scenario checks the coverage checklist whenever a
  run ends on the ready-for-design verdict, and uses that as its evidence the
  gate was earned rather than copied from the planted text. The reviewer points
  out that false coverage claims are explicitly the covering-design task's
  business, not this one's, and that the charter says nothing about whether this
  scenario may borrow that walk for its own purpose.
- **Blocked:** Both ways out change something only a human may change. Widening
  the charter's scope is a plan edit; narrowing the assertion drops a check the
  scenario currently leans on to tell an earned gate from a planted one. Picking
  either silently would be deciding the contract in code.
- **Asked:** 2026-08-05 19:40 — put to the operator through the review-fix
  hand-off: should the task's scope be widened to sanction the coverage walk as
  this scenario's gate-earned evidence, or should the assertion be narrowed to
  the gate's own facts and the coverage rows left to the covering-design task?
- **Decided:** Still waiting.
- **How:** The assertion is left exactly as the reviewed revision had it, and
  every other finding on the pull request is closed around it.
- **Refs:** PR agenticlab-ai/pyxis#1013 · finding FND-3 (calibration row CAL-3) ·
  docs/plans/tech-design-brainstorm/001-plan.md task T11 "Out of scope" ·
  internal/tdbrainstorm/e2e/injection_test.go assertInjectionRun
