# Review: prototype tech spec — design-review report

| Field | Value |
|---|---|
| **In one line** | Approve — FND-1's acceptance condition is met, with no regressions found in the changed spans. |
| **Relevant-when** | Auditing closure of the round-1 ordering defect at the revised contract digest. |
| **Subject** | design:prototype · shape: none · supersedes: — |
| **Moment** | draft |
| **Reviewer** | tech-design-review · repository guide at 47dd8274274a8d0db8c221f63e30d512733b6065 |
| **Date(s)** | 2026-10-15 |
| **Verdict** | approve |
| **Tracking** | local branch draft/prototype-v2 (no pull request yet) |
| **Template version** | design-review-report/v1 |

**Review subject:** tech-spec

**Draft digest:** sha256:2599c039b74afda07f42732844014b1fddb13af935da306582dd280f95c0e2be

## Round 2

This is the final, scope-locked round of the episode begun in [010.md](010.md). Reviewed the computed delta from `a2cda8f0a486db9dd4da57a781cc8f78ee369871` to `47dd8274274a8d0db8c221f63e30d512733b6065` for the two contract files. All contract evidence was pinned to those commits. Scope was FND-1's acceptance condition and regressions within the changed spans; the unchanged remainder was not reopened. The PRD's section 0 was not read, cited, or judged. Operator rulings A-1 through A-6 stand; only A-6's layout slice was consulted for this change.

### Closure of round-1 FND-1

FND-1 in [010.md](010.md#fnd-1--check-the-repository-layout-before-fetching) required ordinary-layout validation and its named refusal before the create fetch, explicit verification that unsupported layouts refuse before any fetch or other mutation, and retention of the early removal refusal.

The revised design: D-1 step 1 now requires that "the repository has the ordinary layout described below" and says "Another layout refuses and names it." Fetch remains step 4. Step 5 now checks only for an existing branch or target directory. Thus an unsupported layout refuses before the fetch in the numbered sequence itself.

The changed layout paragraph explicitly places refusal at "the create form at step 1, before the fetch" and retains refusal by "the remove form before it changes anything". The changed verification requires "a repository with a separate Git directory refused by both forms before any fetch or other Git change". These statements meet every part of the acceptance condition; the author's disposition was checked against the committed delta rather than accepted as proof.

Checked the relevant code and Git premise independently: at the reviewed head, `internal/draft/lifecycle/open.go`, `Open`, invokes `git fetch` with `cfg.Remote` and `cfg.Branch`, without a dry-run flag. The installed Git 2.55.0 `git help fetch`, DESCRIPTION, documents remote-tracking branch updates and writing fetched refs to `FETCH_HEAD`. Fetch is therefore a mutation that must follow layout refusal. No fetch or other state-changing Git experiment was performed. This verifies the contract's ordering repair, not an implementation or test run of the proposed prototype command.

**FND-1 outcome:** closed; the recorded `applied` disposition is substantiated. No new finding block or FND number is created for this closure.

### Changed-span regression assessment

Applied contract-quality, coherence, upstream-alignment, and feasibility-completeness checks only to the changed spans. The numbered order, layout paragraph, and verification now agree. The changes carry A-6's ordinary-layout restriction without adding layout support, locking, or a stronger concurrency guarantee. They reorder an existing check and make its existing verification explicit. No changed-span regression survived scrutiny. No candidate against unchanged content was generated; there is no `violation-withheld` entry and no new FND-2.

Local Git confirms branch `draft/prototype-v2` and the requested head. The PRD blob is identical across the two heads. The shared digest was independently recomputed from the committed contract bytes using `internal/draft/compile/document.go`, `draftDigest` and `writeDigestPart`, and matches the supplied digest; the bytes were hashed as opaque data.

## Verdict history

| Round | Report | Verdict | Date |
|---|---|---|---|
| 1 | [010.md](010.md) | request-changes | 2026-10-15 |
| 2 | [012.md](012.md) | approve | 2026-10-15 |

## Escalation brief

none

## Measurements

- **Rounds**: 2 of 2
- **Findings**: episode total 1 blocking · 0 advisory, retained in 010.md; round 2 adds 0 blocking · 0 advisory; 0 unresolved
- **Dispositions**: 1 applied and verified closed · 0 declined · 0 violation-withheld · 0 pending

## Overlay corrections

none
