# Review: local-git-codehost PRD — review report

| Field | Value |
|---|---|
| **In one line** | Approve the revised draft requirements after closing four blocking consistency and release findings from PR #1312 review. |
| **Relevant-when** | Auditing local Git requirements, the review-driven corrections, or the accepted shared-account limits. |
| **Subject** | `prd:local-git-codehost/draft` |
| **Moment** | draft |
| **Reviewer** | Codex using prd-review guide to verify disposition of GitHub review comments; same session as draft author, not an independent identity |
| **Date(s)** | 2026-09-28 |
| **Verdict** | approve |
| **Tracking** | https://github.com/agenticlab-ai/pyxis/pull/1312 |
| **Template version** | review-report/v1 |

**Review subject:** prd
**Draft digest:** sha256:979aa76fe1c5712c7fdace9ca487a863c17bbe25f9b47c11c40ba0b464cdbe35

## Round 1

The GitHub review of prepared head `1155baf70aa080e58bb32251c4cf6574d5d6d193` exposed the findings below. This report checks their disposition in the revised contract at the digest above through contract-quality, coherence, upstream-alignment, and completeness-critic lenses. It is not operator approval of the new head.

### FND-1 — proposal publication had a split-write race

- **Severity**: blocking (CAL-1)
- **Lens**: coherence
- **Evidence**: prior `UB-4` allowed “publication fails after branch push” while `FR-2` promised one reviewable proposal at the pushed head; [review discussion 4125900289](https://github.com/agenticlab-ai/pyxis/pull/1312#discussion_r4125900289) showed a second writer can move the branch between those writes.
- **Acceptance condition**: require atomic, leased branch and proposal ref publication and an idempotent all-or-nothing retry.
- **Disposition**: applied
- **Applied in**: `FR-2`, `UB-4`, `P-3`, and tech-spec D-2.

### FND-2 — installed proof depended on the final release that it gated

- **Severity**: blocking (CAL-4)
- **Lens**: completeness-critic
- **Evidence**: prior `DOD-4` required installed AEC evidence before release, while tech-spec D-7 required AEC to pin that final release first; [review discussion 4125900303](https://github.com/agenticlab-ai/pyxis/pull/1312#discussion_r4125900303) cited the release-tag artifact lifecycle in `design:installer-v1#§3.1`.
- **Acceptance condition**: use a checksum-verified candidate artifact set for DOD-4, then publish the same set at the same source commit and repin AEC.
- **Disposition**: applied
- **Applied in**: `DOD-4`, §6.1, `P-4`, and tech-spec D-7.

### FND-3 — wrong-repository refusal lacked a stable identity

- **Severity**: blocking (CAL-3; a Must failure row was unverifiable)
- **Lens**: contract-quality
- **Evidence**: prior `UB-1` required refusal of a wrong repository but defined no clone-stable identity; [review discussion 4125900312](https://github.com/agenticlab-ai/pyxis/pull/1312#discussion_r4125900312) identified path, URL, and mutable-ref ambiguity.
- **Acceptance condition**: define a one-time origin ID, pin and compare it from clones, test distinct origins, and disclose the full-copy limit of the accepted shared-account model.
- **Disposition**: applied
- **Applied in**: `UB-1`, §3.2 guardrail, `P-5`, and tech-spec D-1/D-3/D-6.

### FND-4 — implementation receipts named the wrong landing ref

- **Severity**: blocking (CAL-1)
- **Lens**: coherence
- **Evidence**: prior `FR-7` promised the “mainline result” although `design:feature-implementation-drive#§3.5–3.6` lands task proposals on feature-integration branches; [review discussion 4125958223](https://github.com/agenticlab-ai/pyxis/pull/1312#discussion_r4125958223) showed the receipt would check an unrelated ref.
- **Acceptance condition**: bind the receipt to the workflow-derived target ref and its old full SHA, refusing a proposal-chosen alternate target or a moved ref.
- **Disposition**: applied
- **Applied in**: `FR-4`, `FR-7`, `UB-1`, `UB-2`, `P-6`–`P-8`, and tech-spec D-2/D-3/D-5.

The revised Must rows have falsifiable fit criteria and DOD-1/2/4 cover the races, identity mismatch, target-ref mismatch, and candidate proof. DOD-5 still blocks enabling local mode until the existing authenticated-approval contract is amended and its design superseded. No blocking finding survives at this digest.

## Verdict history

| Round | Verdict | Date |
|---|---|---|
| 1 | approve | 2026-09-28 |

## Escalation brief

none

## Measurements

- **Rounds**: 1
- **Findings**: 4 blocking · 0 advisory
- **Dispositions**: 4 applied · 0 declined · 0 withheld-as-violation

## Overlay corrections

none
