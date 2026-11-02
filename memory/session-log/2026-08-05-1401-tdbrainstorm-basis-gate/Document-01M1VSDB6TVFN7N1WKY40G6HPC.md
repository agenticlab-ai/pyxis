# Session log — proving the basis gate turns bad input away at the door

Started: 2026-08-05 14:01 · CLI: Claude Code · Stage or skill: plan task tech-design-brainstorm/001/T2

## P-1 — the session log every session must open is a write the skill is not allowed

- **Found:** 2026-08-05 14:14
- **Problem:** The operating manual tells every agent session, in every CLI, to
  open a session log file before doing anything else, and says plainly that a
  skill run is one of the things it binds. The design-grilling skill's own
  contract says the opposite for the case this task covers: a session that
  refuses a bad basis writes nothing at all. A session cannot do both. Driving
  the two refusals against the real assistant showed the split in practice — one
  refused and opened a log, the other refused and opened nothing.
- **Blocked:** This task's job is to assert that a refusing session leaves the
  tree untouched. Asserting that literally makes the check fail about half the
  time, over a rule the skill did not write and cannot obey alongside its own.
  Dropping the check would stop the task proving the thing it exists to prove. I
  could not pick between two rules that are both binding.
- **Asked:** 2026-08-05 14:19 — which rule gives for a refusing session: does the
  skill's write perimeter widen to allow the log every session must open, or is
  the session-log rule not meant to reach a run that refuses at its first step
  and writes nothing else?
- **Decided:** Still waiting.
- **How:** The check now holds everything neither rule disputes — no record is
  written, nothing already in the tree is changed or deleted, and nothing is
  added anywhere outside the session-log home. A log file that does appear is
  reported in the run's output and in a note in the code saying it is here
  pending this decision, so it is visible rather than quietly allowed. If the
  answer is that the perimeter stands, one line of the check tightens and the
  finding turns red again.
- **Refs:** AGENTS.md "Session logs — every session, every stage, no exceptions" ·
  prd:tech-design-brainstorm/FR-15 · prd:tech-design-brainstorm/UB-5 ·
  .pyxis/authoring/tech-design-brainstorm.md (Never list, "write anything beyond
  the record") · .pyxis/skills/tech-design-brainstorm/fixtures/refusals.md
  ("zero writes") · internal/tdbrainstorm/e2e/basis_gate_test.go (sessionLogHome) ·
  plan task tech-design-brainstorm/001/T2
