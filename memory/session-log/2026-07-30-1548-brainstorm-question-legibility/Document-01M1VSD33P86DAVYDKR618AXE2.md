# Session log — question-legibility amendments for the two brainstorm skills

Started: 2026-07-30 15:48 · CLI: Claude Code · Stage or skill: /prd-amend (feature-brainstorm · tech-design-brainstorm)

## P-1 — implementation was authored and PR'd before its contract froze, and no gate objected

- **Found:** 2026-07-30 16:20
- **Problem:** The guide and fixture changes were written while the contract
  amendments were still drafts, and their PRs were opened first because the
  amendments' Tracking rows needed PR numbers to cite. Every mechanical gate
  ran green on all four branches. The contract-first rule turned out to be
  held only by convention, not by any check.
- **Blocked:** Nothing was blocked mechanically — that is the problem. The
  operator flagged that the pipeline should have stopped this ordering, and
  deciding what to do about it (capture, mechanize, or unwind the open PRs)
  was not mine to settle.
- **Asked:** 2026-07-30 16:25 — offered three paths: record an incident,
  amend the docgate contract with an ordering check, or close the
  implementation drafts until the amendments merge.
- **Decided:** 2026-07-30 16:35 — the operator chose to file a GitHub issue
  capturing the gap and the affected PRs; merge order is held manually for
  this instance.
- **How:** Filed the issue with the diagnosis (single-PR-scoped checks, the
  missing implements-edge, the Tracking-row aggravator) and the proposed
  merge-base resolution check; merge order noted in it.
  Postscript 16:45: overtaken before the manual hold could apply — the
  operator directly merged all four PRs, implementation first (squash, no
  formal reviews), while the review surface idled. Main verified coherent
  after the fact (gate green, projections verified, content byte-identical
  with the branch tips). The merge-boundary inversion is appended to the
  issue as further evidence.
- **Refs:** issue #860 · PRs #856/#854 (feature-brainstorm) · #858/#855
  (tech-design-brainstorm) · docgate checks: check-standalone-docs,
  check-single-amendment, check-immutable, check-citations
