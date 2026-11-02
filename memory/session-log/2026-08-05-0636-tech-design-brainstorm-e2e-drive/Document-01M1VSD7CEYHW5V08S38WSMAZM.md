# Session log — drive the tech-design-brainstorm e2e plan with implement-now

Started: 2026-08-05 06:36 · CLI: Claude Code · Stage or skill: /implement-now

## P-1 — an amendment cannot be reviewed before it is approved, but approval is meant to follow the review

- **Found:** 2026-08-06 01:36
- **Problem:** The amendment authoring guide says to fold the ledger and then run
  the unmanned reviewer over the folded effect, and the reviewer in turn refuses
  any amendment that has not been folded, because one of its four lenses judges
  the resulting projection. But the fold refuses outright unless the amendment
  already records the operator's approval, and there is no flag to relax that.
  The amendment template says approval is what lets an amendment merge — the
  tool applies that bar much earlier, at the fold.
- **Blocked:** I could not run the review the authoring flow requires. Following
  the reviewer's own rule means refusing the subject and writing nothing;
  following the authoring flow means approving first, which puts the operator's
  approval before the review that is supposed to inform it. I could not choose
  between them without deciding whose rule gives way, which is not mine to
  decide.
- **Asked:** 2026-08-06 01:36 — put to the operator: either approve the
  amendment first and accept that the review comes after approval, or treat the
  fold's approval check as the defect and let a draft fold so the review can run
  before approval.
- **Decided:** 2026-08-06 16:35 — the operator ruled that the check sits in the
  wrong place. Working out what a ledger would project to is a calculation, not
  a merge, so it must not ask for anyone's approval; the approval belongs on
  writing the projection down and on merging it.
- **How:** Refused the review per its own first step and wrote no report, then
  gave the fold a compute-only mode: it prints the projection and writes
  nothing, and in that mode alone it stops asking whether the amendment was
  approved. Every other check still runs, and an approved ledger projects to
  exactly the same bytes either way, so a review can never bless a document
  different from the one that merges. This turned out to restore what the
  requirement already said — the approval is written there as a check made when
  something is about to merge.
- **Refs:** docs/prds/e2e-verification/002-harness-design-in-doc-and-serial-scenario-chain.md ·
  .pyxis/authoring/prd-amend.md steps 5 and 5b · .pyxis/authoring/prd-review.md step 1 ·
  internal/docgate/fold/fold.go:439 · `go run ./cmd/docgate fold docs/prds/e2e-verification`
