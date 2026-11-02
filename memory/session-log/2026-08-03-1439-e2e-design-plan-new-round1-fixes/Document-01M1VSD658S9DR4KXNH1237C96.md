# Session log — fixing plan-new's e2e design against review round 1

Started: 2026-08-03 14:39 · CLI: Claude Code · Stage or skill: /e2e-design, then /e2e-review

## P-1 — the human half of the review was never confirmed, and the rules make that decide the verdict

- **Found:** 2026-08-03 15:05
- **Problem:** The review pairs two automated reading passes with a pass the
  operator does by hand on the same document. Round one closed with the human
  pass unconfirmed, and said so. Round two arrived at the same wall. The rules
  say an approval must not be reached by out-running the human half, and they
  also say the reviewer must never assume that half produced nothing — someone
  has to say so out loud.
- **Blocked:** I could not write the round's measurements or its verdict
  honestly. Guessing either way would have put something in the permanent
  record that nobody had actually established.
- **Asked:** 2026-08-03 15:08 — put to the operator: has the hand pass on this
  document happened, did it produce anything, or is it still running?
- **Decided:** 2026-08-03 15:10 — the operator did not answer that question.
  They answered the slot with a different instruction — mark round one's
  findings as fixed — and separately chose to settle the round rather than let
  it escalate.
- **How:** I recorded the human half as unconfirmed at both rounds, in plain
  words, in the one line at the top of the report, and noted that the round was
  settled on the two automated passes alone at the operator's instruction. I
  did not claim a result nobody gave. The report's measurements section turned
  out to be a closed set of three labels with no room for this, which is why it
  sits in the header line instead.
- **Refs:** `docs/e2e/plan-new/review.md` header and Measurements ·
  `.pyxis/authoring/e2e-review.md` step 10 · `internal/e2e/report/parse.go`
  (the closed measurement set)

## P-2 — two new problems surfaced at the last round the rules allow

- **Found:** 2026-08-03 15:06
- **Problem:** The second reading pass found two places where the *first*
  round's own repairs had over-claimed — each a sentence added to close an
  earlier finding that then said more than the tests behind it could show. The
  rules allow only two rounds. Anything still unsettled at the second forces
  the whole review to stop and hand the question to a human, however small the
  problem is.
- **Blocked:** Both routes were legitimate and they lead to different places. I
  could stop the review and hand over two minor wording problems, or the
  document could be corrected first so the round closes cleanly. That is a call
  about how much process a small defect deserves, which is not mine.
- **Asked:** 2026-08-03 15:08 — put to the operator: stop and hand these over,
  or correct the two sentences first and let the round close.
- **Decided:** 2026-08-03 15:10 — correct them first, then close.
- **How:** I recorded both problems as findings, logged them as newly found and
  unsettled, then left the reviewing role — a reviewer is never allowed to edit
  what it reviews — corrected the two sentences as the author, and recorded
  both as settled. The tool that computes the verdict from the record agrees:
  the round closes as approved with advisories.
- **Refs:** `docs/e2e/plan-new/review.md` findings 14 and 15 ·
  `docs/e2e/plan-new/design.md` scenarios 13 and 15, the mock-strategy section,
  and the coverage row for the dialect rendering half

## P-3 — a safety hook refused the commit, and every way past it had a cost

- **Found:** 2026-08-03 15:24
- **Problem:** The operator asked for the work to be committed and pushed to
  the open pull request. A hook refused it. The hook exists so that automated
  workers can only commit inside their own private copy of the repository and
  never in the shared one, and it cannot tell an automated worker apart from an
  operator sitting at the keyboard. Three worker copies were live at the time,
  so the protection was guarding something real, not left over from an old run.
  The hook's own header says not to edit it by hand.
- **Blocked:** I could not carry out a direct instruction. The three ways past
  were: turn the protection off for the whole repository, skip it for one
  commit, or work inside a private copy the hook already allows. The first two
  weaken a safety property that three running jobs depend on, and that is not a
  trade I can make on someone else's behalf.
- **Asked:** 2026-08-03 15:26 — put to the operator: which of the three, given
  what each costs.
- **Decided:** 2026-08-03 15:28 — work inside a private copy. Nothing turned
  off, nothing skipped, no configuration touched.
- **How:** I put the shared copy back exactly as I found it, made a temporary
  private copy of the branch, re-ran the three checks there, committed and
  pushed from inside it, then removed it. The protection stayed on throughout
  and the three running jobs were untouched.
- **Refs:** the hook at `.git/pyxis-guard-hooks/pre-commit`, installed by
  `internal/executor/workspace` · commit `19ac85b` on branch
  `docs/plan-new-e2e-design` · PR #959
