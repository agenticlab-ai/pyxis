# Session log — closing the implementation review on the T3 golden-session scenario

Started: 2026-08-05 19:50 · CLI: Claude Code · Stage or skill: /impl-review follow-up on the T3 pull request

## P-1 — the review asks for something only the plan's owner can decide

- **Found:** 2026-08-05 19:55
- **Problem:** The reviewer flagged the three seeded-defect cases that calibrate
  the golden battery. The task's charter puts "any failing-direction case" out of
  scope, and one of the seeded cases makes the panel fire where the rule says it
  should skip. The charter does not say whether deliberately mutating a copy of
  the committed record to check the battery notices counts as a failing-direction
  case or as ordinary calibration of the test itself.
- **Blocked:** I could close the finding two ways and both change the contract
  rather than the code: delete the calibration and move it to the task that owns
  the defect fixtures, or widen the charter's out-of-scope line to admit it.
  Guessing either way silently rewrites what the task was asked to do, so I left
  the code as it stands.
- **Asked:** 2026-08-05 20:20 — put to the operator in the hand-off: does
  calibrating the battery against a mutated copy of the reference record belong
  to this task, or does it move to the task that owns the defect fixtures?
- **Decided:** Still waiting.
- **How:** Every other finding is closed in code; this one is reported as needing
  a ruling and nothing was changed for it.
- **Refs:** PR #1020 · finding FND-3 (calibration row CAL-3) ·
  docs/plans/tech-design-brainstorm/001-plan.md T3 "Out of scope" ·
  docs/tech-design/design-brainstorm-record-model.md A-2 ·
  internal/tdbrainstorm/e2e/golden_test.go
