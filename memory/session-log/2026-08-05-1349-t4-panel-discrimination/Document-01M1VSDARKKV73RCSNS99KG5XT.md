# Session log — plan task T4, panel discrimination and the near-duplicate redo

Started: 2026-08-05 13:49 · CLI: Claude Code · Stage or skill: plan task
tech-design-brainstorm/001/T4

## P-1 — the driven session writes a session log, which the write perimeter forbids

- **Found:** 2026-08-05 14:29
- **Problem:** I drove one of the fixtures against a real session for the first
  time. The session ran in a throwaway copy of this repository, read the
  operating manual at the top of it, and did what that manual tells every agent
  session to do: it opened a session log of its own before starting work. The
  requirement this suite checks says the opposite — a grilling session writes
  its record and nothing else — so the run was reported as having written
  outside its own record, for a file it was instructed to write.
- **Blocked:** Nothing in my own work stopped, and I have left the check as it
  stands. What I cannot settle is which rule gives way. Letting the harness wave
  through anything under the log directory quietly widens what the write
  requirement means, for every scenario in this suite and not only mine.
  Telling a driven session not to open a log contradicts the manual it is
  reading. Either choice changes something a human decided on purpose, and both
  are outside what this task was asked to do.
- **Asked:** 2026-08-05 14:34 — put to the operator in the hand-off: should a
  driven session's own log be permitted by the write perimeter, should driven
  sessions be told not to open one, or should the requirement itself say which
  it means?
- **Decided:** Still waiting.
- **How:** The check is unchanged and will keep reporting the log file, so the
  question stays visible rather than being absorbed. Every live run in this
  repository will report it until a human rules.
- **Refs:** prd:tech-design-brainstorm/FR-15 · prd:tech-design-brainstorm/DOD-7
  · AGENTS.md "Session logs — every session, every stage, no exceptions" ·
  `internal/tdbrainstorm/e2e/harness.go` (`Result.WroteOutside`) ·
  `internal/tdbrainstorm/e2e/panel_test.go` (`driveFixture`) · observed writing
  `logs/2026-08-05-1415-docgate-quiet-design-brainstorm.md` beside its record
  while driving the extension fixture.
