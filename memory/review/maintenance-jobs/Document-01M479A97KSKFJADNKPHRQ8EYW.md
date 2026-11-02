# Review: maintenance-jobs draft — operator revision design

| Field | Value |
|---|---|
| **In one line** | Request changes: enforce host credential isolation and name the missing historical CI evidence capability. |
| **Relevant-when** | Auditing the revised maintenance design and its bounded engineering corrections. |
| **Subject** | design:maintenance-jobs · shape: none · supersedes: — |
| **Moment** | draft |
| **Reviewer** | tech-design-review · repository guide at 0eaa1ec84fd8ecec98205bbd804cb363432fb486 |
| **Date(s)** | 2026-10-05 |
| **Verdict** | request-changes |
| **Tracking** | https://github.com/agenticlab-ai/pyxis/pull/1443 |
| **Template version** | design-review-report/v1 |

**Review subject:** tech-spec
**Draft digest:** sha256:8b4d0936966796c19673469e33c6ff170cd2f426cf35aa5520e9cb5c81671ee3

Reviewed committed head 0eaa1ec84fd8ecec98205bbd804cb363432fb486. This is
round 1 of the operator-revision episode. Four parallel lenses assessed
contract quality, coherence, upstream alignment, and feasibility/completeness;
the lead independently screened candidates against the contract and code.
Earlier reports retain their old subject revisions. No human-only PRD routing
section or gate-floor requirement was used as review evidence.

## Round 1

### FND-1 — Write confinement does not enforce the promised credential boundary

- **Severity**: blocking (CAL-4; load-bearing for FR-12 and DOD-7)
- **Lens**: feasibility-completeness
- **Evidence**: design:maintenance-jobs#D-5 says “Model authentication is the only selected credential grant; host Git/code-host credentials and mutable user settings remain outside the harness” and relies on an “enforced process/credential boundary.” The reused Darwin policy starts with `(allow default)` and denies writes; Linux read-only binds `/`. Environment filtering and private HOME do not prevent absolute reads of host secrets. The Codex implementation path also bypasses its native sandbox. Those primary-code observations contradict the mechanical supervisor authority promised by prd:maintenance-jobs/FR-12, ABUSE-3, and DOD-7; P-3 remains the accepted isolation ruling.
- **Acceptance condition**: Define an additive host-owned read/credential policy in the shared runtime, including credential-bearing clone Git configuration and host credential services. Carry its restrictions through descendants/delegated launches, allow only the selected private model-auth snapshot, and refuse before claim when unsupported. Darwin/Linux probes through all four harness launch paths must deny injected host secrets and protected writes while permitting required source/tool reads and selected model authentication.
- **Disposition**: applied

D-5 specifies the restrictive shared-runtime policy and positive pre-claim
probes; D-7 adds four-harness credential-read and delegated-launch conformance.
Round 2 must verify closure at the new committed digest. Existing
non-maintenance runtime behavior remains the compatibility baseline.

### FND-2 — Historical CI evidence is attributed to an API that lacks that capability

- **Severity**: advisory (CAL-4)
- **Lens**: feasibility-completeness
- **Evidence**: design:maintenance-jobs#D-5 says “The existing code-host integration supplies bounded recent-CI and pending-PR data.” The current executor CodeHost exposes proposal opening and aggregate current checks, with separate proposal inspection and failed-log capabilities; it does not expose the successful historical workflow/job/step timings required by D-6. Because the harness intentionally receives no code-host authority, the source and schema of those inputs need an explicit owner. prd:maintenance-jobs/FR-13 and EVAL-1 require honest measurable CI evidence; AP-1 requires a consumer-owned medium boundary.
- **Acceptance condition**: Name the consumer-owned historical-evidence capability, its bounded versioned result and completeness limits, and its implementation on the existing code-host adapter axis. Reuse supervisor authentication and proposal/check readers instead of introducing agent-side credentials or a second code-host client. Cover pagination/cutoff and unavailable required-check settings with hermetic adapter fixtures.
- **Disposition**: applied

D-1 names the maintenance-owned evidence port; D-5 defines its bounded schema
and adapter reuse; D-7 adds evidence completeness/limit fixtures. This clarifies
a missing engineering seam without changing FR-13 or the initial prompt's
measurements.

## Verdict history

| Round | Verdict | Date |
|---|---|---|
| 1 | request-changes | 2026-10-05 |

## Escalation brief

none

## Measurements

- **Rounds**: 1 in this episode
- **Findings**: 1 blocking · 1 advisory
- **Dispositions**: 2 applied · 0 declined · 0 withheld-as-violation

## Overlay corrections

none
