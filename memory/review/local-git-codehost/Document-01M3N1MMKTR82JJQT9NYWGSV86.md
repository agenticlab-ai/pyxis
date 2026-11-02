# Review: local-git-codehost tech spec — design-review report

| Field | Value |
|---|---|
| **In one line** | Approve the revised local adapter design after resolving atomic publication, release ordering, origin identity, and target-ref findings. |
| **Relevant-when** | Auditing local Git proposal transactions, receipt consistency, or the review-driven design changes. |
| **Subject** | `design:local-git-codehost` · shape: none · supersedes: — |
| **Moment** | draft |
| **Reviewer** | Codex using tech-design-review guide to verify disposition of GitHub review comments; same session as draft author, not an independent identity |
| **Date(s)** | 2026-09-28 |
| **Verdict** | approve |
| **Tracking** | https://github.com/agenticlab-ai/pyxis/pull/1312 |
| **Template version** | design-review-report/v1 |

**Review subject:** tech-spec
**Draft digest:** sha256:979aa76fe1c5712c7fdace9ca487a863c17bbe25f9b47c11c40ba0b464cdbe35

## Round 1

The GitHub review of prepared head `1155baf70aa080e58bb32251c4cf6574d5d6d193` raised the findings below. This report verifies their disposition in the revised contract at the digest above through coherence, upstream-alignment, contract-quality, and feasibility-completeness lenses. It does not substitute for an independent reviewer or the operator's exact-head approval.

### FND-1 — branch and proposal records could disagree

- **Severity**: blocking (CAL-1)
- **Lens**: coherence
- **Evidence**: prior D-2's branch-first push and later proposal CAS could publish a record naming a replaced branch head, violating `prd:local-git-codehost/FR-2` and `UB-4`; [review discussion 4125900289](https://github.com/agenticlab-ai/pyxis/pull/1312#discussion_r4125900289).
- **Acceptance condition**: atomically publish or revise both refs with old-OID leases and refuse a remote without atomic support; landing must also lease a real branch ref update.
- **Disposition**: applied
- **Applied in**: D-2 and D-4, including leased branch deletion and reviewed-head archive at landing.

### FND-2 — release sequence was impossible

- **Severity**: blocking (CAL-4; load-bearing for DOD-4)
- **Lens**: feasibility-completeness
- **Evidence**: prior D-7 required AEC to pin the final release before that release could be cut, contrary to `design:installer-v1#§3.1`; [review discussion 4125900303](https://github.com/agenticlab-ai/pyxis/pull/1312#discussion_r4125900303).
- **Acceptance condition**: build and verify candidate artifacts from the intended release source, use them for DOD-4, then publish the identical set at a tag on that source and repin AEC.
- **Disposition**: applied
- **Applied in**: D-6/D-7 and PRD DOD-4.

### FND-3 — repository identity was unspecified

- **Severity**: blocking (CAL-3; receipt decision was untestable)
- **Lens**: contract-quality
- **Evidence**: prior D-2/D-3 used “repository identity” without creation, storage, comparison, or fork semantics, leaving `prd:local-git-codehost/UB-1` unverifiable; [review discussion 4125900312](https://github.com/agenticlab-ai/pyxis/pull/1312#discussion_r4125900312).
- **Acceptance condition**: specify initialization, ref and config pin, clone behavior, fork re-identification, and the accepted full-copy limitation.
- **Disposition**: applied
- **Applied in**: D-1/D-3/D-6; the identity is a once-initialized random ID in an origin ref, with the copy limit disclosed.

### FND-4 — task receipts did not bind the task landing destination

- **Severity**: blocking (CAL-2)
- **Lens**: upstream-alignment
- **Evidence**: prior D-3 named only mainline while `design:feature-implementation-drive#§3.5–3.6` uses feature-integration branches for task proposals; [review discussion 4125958223](https://github.com/agenticlab-ai/pyxis/pull/1312#discussion_r4125958223).
- **Acceptance condition**: derive the target from the workflow, bind its full ref and old SHA in proposal and receipt, refuse alternate targets, and CAS the target during landing.
- **Disposition**: applied
- **Applied in**: D-2/D-3/D-5/D-6 and PRD FR-4/FR-7/UB-2.

D-1 through D-7 now describe one transaction path and its conformance cases. They continue to disclose unauthenticated receipt authorship and absent host protection in this shared-account mode. The separate feature-draft amendment and successor design remain prerequisites before implementation is enabled. No blocking finding survives at this digest.

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
