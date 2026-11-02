# Session log — implement-now amendment 006

Started: 2026-08-20 11:53 · CLI: Codex · Stage or skill: /prd-amend

Nothing escalated this session.

## P-1 — An older generated contract row is missing and blocks an honest approval

- **Found:** 2026-08-20 12:18
- **Problem:** The source ledger says a mandatory rollout requirement is live, but the generated contract omits its row while other live rows still cite it. The amendment review cannot honestly approve a contract whose readiness gate claims every live mandatory requirement is present and covered.
- **Blocked:** This stops the amendment review, approval, and final fold. Repairing it inside this amendment would mix an older projection repair into the reciprocal-publication contract; keeping one concern means landing a prerequisite repair and renumbering this draft.
- **Asked:** 2026-08-20 12:29 — whether to absorb the repair here or keep the reciprocal amendment single-purpose behind a prerequisite repair.
- **Decided:** Still waiting
- **How:** No repair or renumbering has been applied while the choice is open.
- **Refs:** `docs/prds/implement-now/001-consume-execution-demand.md`, `docs/prds/implement-now/effective.md`, `docs/prds/implement-now/effective.source.json`, `docs/prds/implement-now/006-reciprocal-review-publication-terminal.md`, `FR-27`, `DOD-13`, `docgate fold --dry-run docs/prds/implement-now`

Decision update, 2026-08-20 12:31: The operator chose the recommended single-purpose path. Land the projection repair as amendment 006, preserve the reciprocal-publication draft for amendment 007, and do not combine the two contracts.

## P-2 — Publishing requires separate authorization for the commit and push

- **Found:** 2026-08-20 13:32
- **Problem:** The requested proposal cannot be opened until the finalized files are staged, committed, and pushed. The publishing workflow requires those state changes to be authorized separately rather than inferred from the request to open a proposal.
- **Blocked:** The amendment is approved, generated, and green locally, but no branch content can be published until that authorization is explicit.
- **Asked:** 2026-08-20 13:32 — permission to stage the six named files, commit them, push the feature branch, and open one draft proposal.
- **Decided:** Still waiting
- **How:** No staging, commit, push, or proposal creation has been performed.
- **Refs:** `docs/prds/implement-now/006-restore-rollout-requirement.md`, `docs/prds/implement-now/effective.md`, `docs/prds/implement-now/effective.source.json`, `docs/reviews/implement-now-006.md`, `logs/2026-08-20-1153-implement-now-amendment-006.md`, `git add`, `git commit`, `git push`, GitHub PR

Correction, 2026-08-20 13:32: The request covers five named files, not six.

Decision update, 2026-08-20 13:37: The operator authorized staging the five named files, committing them, pushing the feature branch, and opening one draft proposal.

Execution update, 2026-08-20 13:43: The five reviewed files were committed and pushed on the feature branch, and one draft proposal was opened. The pre-push code gate passed before publication.

Refs: commit `df3a1f1`, branch `docs/implement-now-006-projection-repair`, PR #1094.

## P-3 — Publishing the review correction and cross-proposal follow-up needs authorization

- **Found:** 2026-08-20 14:01
- **Problem:** The review correction is applied and locally verified. Publishing it changes the open proposal, and recording the narrower residual gate gap changes a separate open proposal; neither remote write can be inferred from applying the local correction.
- **Blocked:** The corrected amendment cannot reach reviewers, and the already-open gate work cannot receive the residual case, until both writes are authorized.
- **Asked:** 2026-08-20 14:01 — permission to commit and push the five-file correction, and to add one focused comment to the existing gate proposal instead of filing a duplicate issue.
- **Decided:** Still waiting
- **How:** No correction commit, push, cross-proposal comment, or thread resolution has been performed.
- **Refs:** PR #1094, PR #1091, `FR-20`, `TestProjectionCongruenceRejectsSameIDMutationBySectionSupersede`, `docs/reviews/implement-now-006.md`

Decision update, 2026-08-20 14:08: The operator authorized committing and pushing the correction to PR #1094 first. No write to PR #1091 and no issue filing is authorized in this step.
