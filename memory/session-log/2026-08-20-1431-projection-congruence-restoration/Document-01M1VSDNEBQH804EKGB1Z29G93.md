# Session log — projection-congruence restoration fidelity

Started: 2026-08-20 14:31 · CLI: Codex · Stage or skill: /prd-amend + GitHub review fix

Nothing escalated this session.

## P-1 — Publishing the contract correction needs authorization

- **Found:** 2026-08-20 14:46
- **Problem:** The contract correction is folded and verified, but staging, committing, and pushing it would change the open proposal.
- **Blocked:** Reviewers cannot see the correction until those repository writes are authorized.
- **Asked:** 2026-08-20 14:46 — permission to stage the five contract-proposal files, commit them, and push the existing proposal branch.
- **Decided:** Still waiting
- **How:** No staging, commit, or push has been performed.
- **Refs:** PR #1093, FR-40, DOD-7, `docs/prds/docgate/018-projection-congruence.md`

Decision update, 2026-08-20 14:47: The operator authorized staging, committing, and pushing the verified correction to PR #1093. PR #1091 remains unchanged.

## P-2 — The authorized proposal merged before the update reached it

- **Found:** 2026-08-20 14:55
- **Problem:** The proposal merged and its branch was deleted while the correction was being prepared. The attempted update therefore recreated the deleted branch instead of changing the frozen proposal.
- **Blocked:** A merged amendment cannot be edited; carrying the correction now requires a new dense amendment and a separate proposal.
- **Asked:** 2026-08-20 14:55 — whether to turn the verified correction into the next docgate amendment and raise a new proposal.
- **Decided:** Still waiting
- **How:** The accidentally recreated branch was deleted immediately. No new proposal was opened, and the implementation proposal remains unchanged.
- **Refs:** PR #1093, amendment 018, amendment 019, branch `docs/projection-congruence-contract`, commit `d7a2624`, PR #1091

Decision update, 2026-08-20 15:00: The operator directed that the docgate follow-up be abandoned, the implementation proposal remain parked, and the work move directly to the minimal reciprocal review-terminal amendment needed to unblock delivery.

Time correction, 2026-08-20 15:41: The preceding decision update was recorded at 15:41, not 15:00. Its substance is unchanged.

## P-3 — The exact reciprocal-terminal contract needs approval before folding

- **Found:** 2026-08-20 15:51
- **Problem:** The narrow amendment text now defines the exact cross-contract event join and the failure behavior for missing or conflicting evidence. Marking that contract approved before the operator sees its exact wording would freeze a critical security and observability obligation without the required human gate.
- **Blocked:** The document gate will not fold or validate a draft amendment, and the amendment cannot honestly be marked approved without the operator's decision.
- **Asked:** 2026-08-20 15:51 — approval of the exact rendered draft so it can be marked approved, folded, and reviewed.
- **Decided:** Still waiting
- **How:** Kept the amendment in Draft and opened its rendered review surface. No generated projection, commit, push, or proposal was created.
- **Refs:** `docs/prds/implement-now/007-reciprocal-review-publication-terminal.md`, FR-49, EVT-17, DOD-36

Decision update, 2026-08-20 15:53: The operator approved the exact draft by directing that its proposal be raised. The amendment may now be marked Approved, folded, reviewed, committed, pushed, and opened as a draft proposal.

## P-4 — Publishing the human review correction needs authorization

- **Found:** 2026-08-20 16:30
- **Problem:** The supplied review finding is corrected locally and the folded contract passes its gates. Updating the open proposal requires a new commit and push.
- **Blocked:** The corrected dependency row is not visible to proposal reviewers until those repository writes are authorized.
- **Asked:** 2026-08-20 16:30 — permission to commit and push the verified correction to the existing proposal.
- **Decided:** Still waiting
- **How:** Kept the correction unstaged and unpushed after verification.
- **Refs:** PR #1099, §6.2, impl-review amendment 004

Decision update, 2026-08-20 16:45: The operator authorized committing and pushing the verified dependency-row correction to the existing proposal.
