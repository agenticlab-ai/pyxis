# Session log — revising plan-new's e2e design against its review

Started: 2026-08-01 15:28 · CLI: Claude Code · Stage or skill: /e2e-design

## P-1 — an e2e review report, written exactly as its own template says, is refused by the document gate

- **Found:** 2026-08-03 01:05
- **Problem:** The e2e review report opens with a Subject line naming what it
  reviewed. Its template says to write that as the words "e2e-design" followed
  by a colon and the feature name. The repository's document gate reads the
  tail of that phrase as a reference to a technical design called "plan-new",
  finds no such design, and refuses the whole tree. The template itself slips
  through only because it leaves a placeholder there rather than a real feature
  name, so the problem appears the moment anyone fills it in. The older review
  reports never hit this, because they live in a folder the gate deliberately
  skips for exactly this reason; the e2e ones were given a different home and
  inherited no such skip.
- **Blocked:** It stopped the pull request from going green, and I could not
  tell which side is meant to change. Either the report's own wording is wrong,
  or the gate's list of folders to skip is missing one — and every future e2e
  review will land on the same wall until someone decides which.
- **Asked:** 2026-08-03 01:14 — put to the operator: should the gate skip the
  e2e review folder the way it already skips the older review folder, or should
  the report template's subject line be written in a form the gate does not read
  as a reference?
- **Decided:** Still waiting.
- **How:** To get this pull request green without touching either contract, I
  wrapped the value in backticks, which the gate treats as plain text rather
  than a reference. That is a formatting change to one line and settles nothing
  for the next report.
- **Refs:** `docs/e2e/plan-new/review.md` line 7 · `.pyxis/templates/e2e-review-report-template.md`
  line 7 · `internal/docgate/gate/reg_check_citations.go` (the `docs/reviews`
  carve-out and the code-span exemption) · failing check `docs-gate` on PR #959,
  `check-citations: citation "design:plan-new" does not resolve`
