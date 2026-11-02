# Session log — running the tdbrainstorm end-to-end suite to a green result

Started: 2026-08-06 01:31 · CLI: Claude Code · Stage or skill: ad-hoc test run

## P-1 — Asked to make the suite pass, but most of it only runs against a real agent

- **Found:** 2026-08-06 01:31
- **Problem:** The suite has two halves. One half runs with no model call and is
  already green. The other half — twenty-two scenarios — stays switched off
  unless an environment switch is set, and each one drives a real agent session
  in a throwaway copy of the repository, with a thirty-minute ceiling apiece.
- **Blocked:** "Make sure they all pass" reads either way. Turning the switch on
  commits hours of wall clock and real spend; leaving it off means twenty-two
  scenarios stay skipped and the answer is hollow. Guessing either way is
  costly, so I could not settle it alone.
- **Asked:** 2026-08-06 01:33 — whether to run the gated live half, and if so
  whether to run it one at a time or several at once.
- **Decided:** 2026-08-06 01:35 — run the whole gated half, one scenario at a
  time rather than several at once.
- **How:** Turned the switch on for a single sequential pass over the package,
  with the run's own clock lifted so a long scenario is not cut off by the
  default ceiling.
- **Refs:** `internal/tdbrainstorm/e2e/`, `PYXIS_TDB_E2E_LIVE`,
  `go test ./internal/tdbrainstorm/...`

## P-2 — Two checks went red because another session's unfinished work is sitting in the shared checkout

- **Found:** 2026-08-06 03:34
- **Problem:** Two of the doc-gate checks in this package failed. They were
  green when I measured the same package two minutes before I started. The
  cause is not this package: a second agent session is working in the same
  checkout and left a new, still-draft requirement amendment plus a generated
  projection that no longer matches it. The gate reads the whole tree, sees the
  draft, and goes red before the test has done anything of its own. Proved it by
  running the same gate against a clean copy of the last commit, where it passes.
- **Blocked:** I cannot get those two checks green without touching work that
  belongs to another session — regenerating their projection or setting their
  amendment to approved. The repo's own rule forbids sweeping another session's
  unfinished work into mine, and it records that this has already gone wrong here
  once. So the only route past the blockage is one I am not allowed to take.
- **Asked:** 2026-08-06 07:41 — reported it and left the other session's files
  untouched, flagging that these two failures are not the feature's.
- **Decided:** Still waiting
- **How:** Left both files exactly as found. Ran the gate against a clean copy of
  the last commit to prove the failures are environmental, and reported the two
  checks separately from the seventeen real ones.
- **Refs:** `docs/prds/e2e-verification/002-harness-design-in-doc-and-serial-scenario-chain.md`
  (untracked, `Status: Draft`), `effective.md`, `effective.source.json`,
  `TestDocgateAndIndexUnchangedWithRecordsPresent`,
  `TestDocgateStaysGreenWithProducedRecordsPresent`, `go run ./cmd/docgate gate`

## P-3 — Most of the driven sessions break the record contract, and mending that means editing the thing under test

- **Found:** 2026-08-06 07:20
- **Problem:** Seventeen of the twenty-two driven scenarios failed, and they do
  not fail for seventeen separate reasons. Three faults repeat across scenarios
  that share no code: sessions finish without writing the single closing verdict
  the contract demands, or write two; the entries they write run two to three
  times longer than the length the contract caps them at; and interview turns
  arrive without the grounded example every turn is supposed to carry. Only one
  fault is genuinely in the test code — seven places forbid a session from
  opening the session log this repo obliges every session to open, while two
  other places correctly allow it.
- **Blocked:** The one test-side fault I can mend safely. The other three are the
  skill's own behaviour drifting from its written contract, and the only way to
  make those scenarios pass is to change the skill the scenarios exist to
  measure. Loosening the checks instead would destroy the very signal this run
  was paid for, and rewriting the skill is a product change well past "run the
  tests". I cannot pick between those on my own.
- **Asked:** 2026-08-06 07:41 — put the split to the human: mend the test-side
  fault, and take direction on whether the skill or the expectations should move.
- **Decided:** Still waiting
- **How:** Pending.
- **Refs:** `internal/tdbrainstorm/e2e/`, `covered_test.go` (`sessionLogsHome`,
  `writePerimeterBreaches`), `.pyxis/skills/tech-design-brainstorm/`,
  gate checks `gate-single` · `gate-verdict-enum` · `gate-conditional-fields`,
  `entry-length` (NFR-2), EVAL-1 · EVAL-4 · EVAL-7
