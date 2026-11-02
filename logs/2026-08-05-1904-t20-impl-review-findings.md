# Session log — resolving the implementation review's findings on T20

Started: 2026-08-05 19:04 · CLI: Claude Code · Stage or skill: /impl-review follow-up on plan task tech-design-brainstorm/001/T20

## P-1 — the task charter asks for a neighbour's test corpus that its own scope and acceptance never mention

- **Found:** 2026-08-05 19:10
- **Problem:** The review says the non-regression task promised more than it
  built. The task's "desired behavior" says the stage-one sibling's own
  evaluation corpus must still pass; nothing in the work runs that corpus. But
  the same task's scope list and its acceptance list never mention the corpus at
  all — they only ask for a diff of the sibling's files. So the task asks for
  two different things in two different places.
- **Blocked:** Either side can be made true, and they lead opposite ways. One
  way builds a runner over the neighbour's own test corpus — which means
  reaching into the very surface this task exists to leave alone. The other way
  narrows the task's stated goal to what its scope and acceptance actually list.
  Picking one silently would be deciding the contract, which is not mine to
  decide, so I changed neither the test nor the task.
- **Asked:** 2026-08-05 19:40 — put to the human with the two options above and
  a recommendation to narrow the goal, since the neighbour's corpus is the
  neighbour's suite's business and walking it here would touch the protected
  surface.
- **Decided:** Still waiting.
- **How:** Left the code as it stands and reported the question up with the rest
  of the review outcome. Everything else the review raised is fixed.
- **Refs:** review finding FND-1 (calibration row CAL-3, blocking); plan task
  tech-design-brainstorm/001/T20 in docs/plans/tech-design-brainstorm/001-plan.md;
  internal/tdbrainstorm/e2e/non_regression_test.go; PR #1005.
