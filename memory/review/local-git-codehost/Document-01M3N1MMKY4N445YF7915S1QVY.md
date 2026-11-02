# Review: local-git-codehost tech spec — design-review report

| Field | Value |
|---|---|
| **In one line** | Approve the scoped configured-origin design as a reviewable proposal; the product-side scope ruling remains pending. |
| **Relevant-when** | Auditing the local Git adapter's configured-origin, receipt, and landing design after the common-case revision. |
| **Subject** | `design:local-git-codehost` · shape: none · supersedes: — |
| **Moment** | draft |
| **Reviewer** | Codex using tech-design-review guide; same session as draft author, not an independent identity |
| **Date(s)** | 2026-09-28 |
| **Verdict** | approve |
| **Tracking** | https://github.com/agenticlab-ai/pyxis/pull/1312 |
| **Template version** | design-review-report/v1 |

**Review subject:** tech-spec
**Draft digest:** sha256:1d85f040d154c58cdda2878b558084ac420d13da8482db6f73caf584f0274622

## Round 1

Reviewed the committed PRD and D-1 through D-7 at `f3a83caf2a96fc589087b7434885d6e5b5a91cf1` through contract-quality, coherence, upstream-alignment, and feasibility-completeness lenses. D-1 no longer requires an origin-ID initialization protocol; D-2/D-3 omit the ID field; D-3/D-4 retain the clone-visible receipt bytes and exact-head and target checks required by PRD FR-3/FR-4/FR-6/FR-7. D-6 removes distinct-origin and copied-origin identity fixtures while keeping normal and parallel-agent stale-ref fixtures. The design discloses the accepted shared-account limits and the newly proposed lack of independent repository identity. Its configured-origin check is a local configuration check, not authentication of an origin. No substantive design finding survives. This verdict does not grant P-5 or Q-3, approve a draft head, or replace the separate feature-draft contract migration.

## Verdict history

| Round | Verdict | Date |
|---|---|---|
| 1 | approve | 2026-09-28 |

## Escalation brief

none

## Measurements

- **Rounds**: 1
- **Findings**: 0 blocking · 0 advisory
- **Dispositions**: 0 applied · 0 declined · 0 withheld-as-violation

## Overlay corrections

none
