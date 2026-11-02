# Review: prototype tech spec — design-review report

| Field | Value |
|---|---|
| **In one line** | Request changes — D-1 fetches before the unsupported-layout check despite promising refusal before any change. |
| **Relevant-when** | Revising or auditing the version-2 prototype tech spec and its application of A-6. |
| **Subject** | design:prototype · shape: none · supersedes: — |
| **Moment** | draft |
| **Reviewer** | tech-design-review · repository guide at a2cda8f0a486db9dd4da57a781cc8f78ee369871 |
| **Date(s)** | 2026-10-15 |
| **Verdict** | request-changes |
| **Tracking** | local branch draft/prototype-v2 (no pull request yet) |
| **Template version** | design-review-report/v1 |

**Review subject:** tech-spec

**Draft digest:** sha256:ba5a041dfd27a7ba4145c10dc2ffcfe1d7daa5a61647bd70d34a4332b9f81a95

## Round 1

Reviewed the complete tech spec and its PRD requirement basis from commit `a2cda8f0a486db9dd4da57a781cc8f78ee369871`, excluding the PRD's section 0. This is round 1 of version 2's first review episode. Particular attention went to the diff from `2253a135`: D-1's ordinary-layout restriction and concurrency limitation, and the four repaired design-brainstorm citations. Operator rulings A-1 through A-6 and the locked brainstorm decisions were accepted as inputs, not reconsidered.

The lenses ran sequentially in this session:

1. **contract-quality** — checked D-1 through D-5, their binding statements, refusal conditions, and runnable verification against the served requirements.
2. **coherence** — checked each decision against its concrete layout and the other decisions. FND-1 survived this pass and subsequent adversarial screening.
3. **upstream-alignment** — checked PRD fidelity, the locked version-1 design, record-002's live D-2 through D-4 and chosen alternatives, principles, and incidents. The repaired citations resolve to the locked `brainstorm/prototype` record-002 and carry its decisions faithfully. The new concurrency limitation carries A-6 without introducing a stronger coordination guarantee.
4. **feasibility-completeness** — checked Git semantics, routing and effective catalog loading, runtime construction, brief assembly, task-state preparation, launch access, memory-pack exclusion, failures, and the five-task estimate. Checked the relevant source implementations and installed Git 2.55.0 documentation; no state-changing Git experiment was run.

Only the finding below survived adversarial screening. Its closure reorders an existing check and adds no mechanism or product scope. Mechanical lint and readiness conditions were excluded after reading their implementations.

Verification limitation: remote draft status failed because GitHub's hostname could not be resolved. Local Git confirms `draft/prototype-v2`; the digest command reports the supplied digest at local head `a024efa8229d93490ade1964e3c821903e2faf5d`. All subject reads remained pinned to the requested commit.

### FND-1 — Check the repository layout before fetching

- **Severity**: blocking (CAL-1)
- **Lens**: coherence
- **Evidence**: design: D-1 introduces the numbered flow as "Create, in order, refusing before any change when a check fails". Step 4 fetches the configured mainline; step 5 then checks that "The repository has the ordinary layout". The new layout paragraph promises that "both forms refuse and name the layout before changing anything" while citing `problem:prototype#A-6`. These instructions cannot all hold on an unsupported layout with a valid route and a reachable remote: following the numbered order performs the fetch first. Git's installed `git help fetch`, DESCRIPTION and `--write-fetch-head`, document that fetch updates remote-tracking refs and writes `FETCH_HEAD` by default. The existing `internal/draft/lifecycle/open.go`, `Open`, confirms the referenced fetch is `git fetch <remote> <branch>`, not a dry run. This conflict exists without any concurrent process, so A-6's concurrency limitation does not remove it. The upstream ruling A-6's layout refusal remains accepted; the defect is the ordering within D-1's concrete application of it.
- **Acceptance condition**: Move ordinary-layout validation and its named refusal ahead of the fetch in D-1's create sequence. Make D-1's unsupported-layout verification explicitly observe that no fetch or other mutation is attempted before refusal; retain the same early refusal for removal. No new locking or layout support is required.
- **Disposition**: applied — the layout check and its named refusal move into step 1, ahead of step 4's fetch, so step 5 again checks only the existing branch and directory and no step is renumbered. The layout paragraph states where each form refuses: the create form at step 1, the remove form before it changes anything. The verification observes that a repository with a separate Git directory is refused by both forms before any fetch or other Git change. No locking or layout support is added.

## Verdict history

| Round | Verdict | Date |
|---|---|---|
| 1 | request-changes | 2026-10-15 |

## Escalation brief

none

## Measurements

- **Rounds**: 1
- **Findings**: 1 blocking · 0 advisory
- **Dispositions**: 1 applied · 0 declined · 0 withheld-as-violation · 0 pending

## Overlay corrections

none
