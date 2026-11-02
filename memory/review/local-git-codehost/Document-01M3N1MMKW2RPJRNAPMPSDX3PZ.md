# Review: local-git-codehost tech spec — design-review report

| Field | Value |
|---|---|
| **In one line** | Approve the revised receipt-recovery path after embedding canonical receipt bytes in the final proposal record. |
| **Relevant-when** | Auditing fresh-clone and crash-recovery evidence for a local Git lock or implementation landing. |
| **Subject** | `design:local-git-codehost` · shape: none · supersedes: — |
| **Moment** | draft |
| **Reviewer** | Codex using tech-design-review guide to verify GitHub review disposition; same session as draft author, not an independent identity |
| **Date(s)** | 2026-09-28 |
| **Verdict** | approve |
| **Tracking** | https://github.com/agenticlab-ai/pyxis/pull/1312 |
| **Template version** | design-review-report/v1 |

**Review subject:** tech-spec
**Draft digest:** sha256:280dec5bde8f5c1a21e9c64a15faf355e31ece028e264f384091b2ff8240a126

## Round 1

Reviewed the current PRD, D-1 through D-7, and the receipt-recovery finding raised on the prior draft. The proposal transaction, target-ref CAS, origin identity, candidate release proof, and accepted unauthenticated-actor limit remain coherent with the revised PRD. This verdict is a same-session document review, not the operator's exact-head approval.

### FND-1 — a fresh clone needed the receipt's complete bytes

- **Severity**: blocking (CAL-4; load-bearing for PRD FR-6 and DOD-1)
- **Lens**: feasibility-completeness
- **Evidence**: prior D-4 retained only the receipt digest in the lock record, so a fresh clone could lose the fields needed by `prd:local-git-codehost/FR-6`; [review discussion 4126115982](https://github.com/agenticlab-ai/pyxis/pull/1312#discussion_r4126115982).
- **Acceptance condition**: keep the canonical receipt on a clone-visible ref and embed its full bytes in the atomically published final proposal record, then verify after deleting the review worktree.
- **Disposition**: applied
- **Applied in**: D-3/D-4/D-5/D-6, including a dedicated receipt ref, embedded bytes and digest, and fresh-clone recovery fixtures.

No blocking finding survives at this digest. The separate feature-draft contract migration remains a prerequisite to enabling the local adapter.

## Verdict history

| Round | Verdict | Date |
|---|---|---|
| 1 | approve | 2026-09-28 |

## Escalation brief

none

## Measurements

- **Rounds**: 1
- **Findings**: 1 blocking · 0 advisory
- **Dispositions**: 1 applied · 0 declined · 0 withheld-as-violation

## Overlay corrections

none
