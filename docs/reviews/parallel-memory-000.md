# Review: parallel-memory/000 — review report

| Field | Value |
|---|---|
| **In one line** | Approved after clarifying that legacy debt-policy behavior stays unchanged while causal admission can ship separately. |
| **Relevant-when** | reviewing the parallel-memory requirement contract, compatibility scope, and readiness for implementation planning |
| **Subject** | prd:parallel-memory/000 |
| **Moment** | new-base |
| **Reviewer** | prd-review · repository guide inspected 2026-09-17 |
| **Date(s)** | 2026-09-17 |
| **Verdict** | approve |
| **Tracking** | Local introducing proposal; external PR not yet created |
| **Template version** | review-report/v1 |

## Round 1

The base was folded before review. Four lenses ran concurrently: independent
contract-quality, coherence, and upstream-alignment agents, with the
coordinator examining completeness against writer failures, adversarial
history, migration/recovery, and the named acceptance matrices. The subject's
human-only §0 was excluded. `docgate gate` and the feature's mechanical
contract check passed; their enforced floor is not a review finding.

### FND-1 — Scope legacy compatibility to debt-policy behavior

- **Severity**: blocking (CAL-1)
- **Lens**: coherence
- **Evidence**: NR-3 originally promised “Existing-format repositories retain their behavior until explicit migration” and “Existing-format fixtures remain unchanged”; §6.5 repeated that scope. FR-6 intentionally changes admission of older unrelated records, while §6.1 stages that change before policy cutover. This contradicts the intended independent rollout expressed by the approved causal-admission proposal in §3.2; vision §4 requires contradictory contracts to return with named defects before planning.
- **Acceptance condition**: Scope NR-3 and §6.5 to existing-format debt policy and validation, explicitly allowing independently staged causal admission before policy migration.
- **Disposition**: applied — NR-3 now preserves debt-policy fixtures while allowing the causal age-floor expectations to change in their own increment; §6.5 states the same independent rollout boundary. No new scope or operator decision was introduced.

No other findings survived adversarial screening. The contract states honest
T3/critical risk, executable fit criteria, deterministic evidence checkpoints,
and the distinction between prior direction approval and pending review of
this requirement artifact. The missing ledger discovered at the initial
planning precheck is now present; the current `plangate precheck
parallel-memory` passes its mechanical checks.

## Round 2

The coherence lens checked only FND-1, the revised NR-3 and §6.5, and their
implicated rollout requirements. The revised text consistently permits causal
admission before policy migration while preserving existing-format debt-policy
behavior. FND-1 is closed; no changed-span regression was found. No new
findings. This is the automated review verdict, not a human sign-off on the
new requirement artifact or a claim that its designs have merged.

## Verdict history

| Round | Verdict | Date |
|---|---|---|
| 1 | request-changes | 2026-09-17 |
| 2 | approve | 2026-09-17 |

## Escalation brief

none

## Measurements

- **Rounds**: 2
- **Findings**: 1 blocking · 0 advisory
- **Dispositions**: 1 applied · 0 declined · 0 withheld-as-violation

## Overlay corrections

none
