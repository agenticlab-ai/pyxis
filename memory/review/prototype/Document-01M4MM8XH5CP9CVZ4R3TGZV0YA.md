# Review: prototype — tech-spec review report

| Field | Value |
|---|---|
| **In one line** | Request changes: the removal command exists in the design, but the normal lock and abandonment flows do not assign its invocation. |
| **Relevant-when** | Auditing the technical review of the prototype draft or closing its resource-lifecycle finding before lock. |
| **Subject** | design:prototype · shape: none · supersedes: — |
| **Moment** | draft |
| **Reviewer** | tech-design-review · repository guide at ffc1bb4319979178058e8c66dc36ef8d97ab4752 |
| **Date(s)** | 2026-10-10 |
| **Verdict** | request-changes |
| **Tracking** | local branch draft/prototype (no pull request yet) |
| **Template version** | design-review-report/v1 |

**Review subject:** tech-spec

**Draft digest:** sha256:43fe81c871bdb5d207a4bfb41829e2b813024d97c4f6566d196d558433837945

## Round 1

Reviewed the committed tech spec against the PRD and both brainstorm records at ffc1bb4319979178058e8c66dc36ef8d97ab4752, after completing the PRD review in report 001. The problems file at that head contains no raised problem. `pyxis draft digest prototype` reproduced that head and the digest above. The current branch is `draft/prototype`; the remote lifecycle status check failed because GitHub DNS resolution was unavailable. This report judges the supplied open-draft subject.

Ran the four lenses sequentially: contract-quality, coherence, upstream-alignment, then feasibility-completeness. Challenged every candidate after those passes and retained only the finding below. The four decisions provide concrete layouts and verification methods within the three-task estimate. The numbered brainstorm capture, citation resolution, and exclusion of records from the contract digest agree with `internal/draft/compile/document.go` and `internal/draft/readiness/links.go`; the proposed worktree directory and `--no-track` creation match `scripts/startup/worktree.zsh`. Mechanical compiler, lint, and readiness conditions were excluded from substantive findings.

### FND-1 — Assign removal after lock or abandonment

- **Severity**: blocking (CAL-2)
- **Lens**: upstream-alignment
- **Evidence**: Tech spec D-1 says the prototype "stays on the local machine until the feature's draft locks or is abandoned" and defines `pyxis draft prototype <feature> --remove`. D-2's final guide step is "Leave the worktree in place and report." D-4's guide changes add a pre-lock findings check but no post-lock or abandonment removal step. By contrast, PRD §7.2/Q1 resolves the retention question as "until the feature's draft locks or is abandoned, then removed with the same command that created them"; brainstorm record 002/D-3 owns that lifecycle direction. A removal interface alone does not assign the promised release in either normal terminal flow: a session can follow every listed guide step and leave the branch and worktree indefinitely. AP-12 requires acquired resources to reach a consumed, released, or reaped state. This finding concerns completion of the recorded ordinary lifecycle, not a change to record 002/O-3's decision against automatic removal at lock.
- **Acceptance condition**: Name the responsible local session and include explicit post-lock and abandonment instructions in the proposed guide changes to invoke `pyxis draft prototype <feature> --remove`. Include verification that following each normal terminal flow removes a clean, committed prototype's worktree and branch, while retaining D-1's dirty-worktree refusal. A guide passage and coverage of those two flows suffice; no automatic lock hook or periodic reaper is required.
- **Disposition**: applied — D-2 gains a release passage naming the session that ends the draft as the one that runs the removal, after a verified lock and after an abandonment; D-4 adds the same step to the Argus drive guide; D-1 and D-2 state the verification. No automatic hook or reaper is added, and the PRD is unchanged.

## Verdict history

| Round | Verdict | Date |
|---|---|---|
| 1 | request-changes | 2026-10-10 |

## Escalation brief

none

## Measurements

- **Rounds**: 1
- **Findings**: 1 blocking · 0 advisory
- **Dispositions**: 1 applied · 0 declined · 0 withheld-as-violation · 0 pending

## Overlay corrections

none
