# Review: prd:forgejo-feature-draft/draft — review report

| Field | Value |
|---|---|
| **In one line** | approve — P-9 resolves the escalated 008/FND-1 conflict for installed Forgejo, and no defect survives review of the changed spans. |
| **Relevant-when** | Reviewing the Forgejo feature-draft PRD at this digest or tracing the disposition of 008/FND-1. |
| **Subject** | prd:forgejo-feature-draft/draft |
| **Moment** | draft |
| **Reviewer** | prd-review |
| **Date(s)** | 2026-10-03, 2026-10-04 |
| **Verdict** | approve |
| **Tracking** | https://github.com/agenticlab-ai/pyxis/pull/1365 |
| **Template version** | review-report/v1 |

**Review subject:** prd
**Draft digest:** sha256:7ca390f9bbbd47613d3b5f33081acdc904d59677316b2eec94dfe228f956ac10

## Round 1 — preserved 008 history

[Report 008](008.md) records a `request-changes` verdict on 2026-10-03 for committed head `d2f0e5640858cf1e9f0fce3736f6acc3a84c0ef1` and digest `sha256:ade0bfd49a5a0b21f3c61701a2c2e79e07904a2f952b031af579d85c3a92ec7e`. Its original digest and verdict remain historical evidence; Source journal E-197 archives the original native 008 evidence. The finding below is linked history, not a new finding against the revised subject.

### FND-1 — Blocking acceptance covers merges but not direct mainline writes

- **Severity**: blocking (CAL-2)
- **Lens**: upstream-alignment
- **Evidence**: The original PRD FR-5 required enforcement before permitting a prepared contract to *merge*, while its fit criterion and DOD-3 tested negative merge attempts. Direct or force pushes and path exemptions could still place compiled records on mainline before FR-6 refused a lock. This conflicted with frozen `design:feature-draft-v1#DINV-1` and §3.9; the original tech-spec D-5 also required protection against those writes. See [008/FND-1](008.md).
- **Acceptance condition**: Require FR-5 and blocking DOD-3 evidence to cover direct and force pushes, privileged actors, and path exemptions; native negative fixtures must show unchanged mainline, and uncertain effective protection must refuse preparation.
- **Disposition**: declined — operator · P-9 on 2026-10-04 accepts the explicitly scoped installed-Forgejo trusted-governance model in place of bypass-enforcement proof; authenticated exact-head human review and verified merged artifacts remain required. The original acceptance condition is not claimed implemented, GitHub behavior remains, and separate human lock approval is pending.

## Round 2 — scope-locked re-review

I inspected committed subject `3e79a38c2dd3ce17d898ae12c4fabb6e8d4ae3f8` against canonical main `2a2125c28f65ea37f66bc6e12c74552ca981e3fe`, using the supplied computed PRD and tech-spec changed spans. I read only the original 008/FND-1 and its disposition, exact-head problems.md P-9, brainstorm 003 D-2/ALT-2 and affected scope/coverage, and the supplied frozen feature-draft §3.9 slice. Human-only PRD §0 did not inform this review. I performed read-only inspection; I ran no gates, tests, or network checks.

- **Contract-quality**: Revised FR-5 and DOD-3 state testable positive and negative lock conditions. The critical risk remains explicit; R-2 states that an out-of-band unapproved mainline write is possible under the chosen governance assumption.
- **Coherence**: FR-5 and blocking DOD-3 both require the configured human’s latest authenticated exact-head review and verified merged artifacts for lock authorization. Their Forgejo preparation criterion does not claim bypass-prevention proof.
- **Upstream-alignment**: P-9 expressly scopes the change to installed Forgejo and to frozen §3.9/DINV-1 after a separate v2 human lock. Brainstorm 003 D-2/ALT-2, the revised PRD, and tech-spec D-5 carry that boundary and preserve existing GitHub behavior.
- **Completeness-critic**: The changed criteria refuse stale, agent, rejected, changed-head, and uncertain reviews with zero lock-tag writes. The design retains independent content and ancestry verification and does not present it as prevention of an out-of-band write.

Adversarial screening left no changed-span defect to deliver. The original bypass-prevention acceptance condition remains declined by operator ruling; it cannot be recast as implemented or raised again against unchanged content.

The blocking decline triggered CAL-2 auto-escalation. [P-9](../problems.md) records the conflict, options, risks, and the operator’s 2026-10-04 answer, “ok, your proposa is aproved.” That answer resolves the product choice for this review episode without approving this draft’s lock. With no surviving changed-span finding or unanswered authority question, the legal round-two verdict is `approve`.

Supplied deterministic ground reports that native `.11` final digest succeeds for this head and digest. At `b562`, readiness named stale 007/008 and 008/FND-1, with no unanswered P-9; native attribution refused the edited 008 path for merge-base ownership. Those are review-currency and ownership conditions, not substantive findings. Public PR #1365 remains OPEN DRAFT at `d2f0e5640858cf1e9f0fce3736f6acc3a84c0ef1`. This report asserts no 009 attribution, hosted CI result, current tech-spec review, human lock, Source landing, planning, implementation, or delivery.

## Verdict history

| Round | Verdict | Date |
|---|---|---|
| 1 — preserved in 008 | request-changes | 2026-10-03 |
| 2 — this report | approve | 2026-10-04 |

## Escalation brief

none

## Measurements

- **Rounds**: 2 in the 008 episode; round 2 is recorded here because the combined contract digest changed.
- **Findings**: 1 historical blocking (008/FND-1) · 0 advisory · 0 new round-two findings.
- **Dispositions**: 0 applied · 1 declined, escalated and answered at P-9 · 0 withheld-as-violation · 0 pending.

## Overlay corrections

none
