# Review: prd:forgejo-feature-draft/draft — review report

| Field | Value |
|---|---|
| **In one line** | request-changes — the blocking PRD acceptance evidence does not cover direct writes that could put compiled contract records on mainline without human review. |
| **Relevant-when** | Reviewing the Forgejo feature-draft v2 PRD at the stated digest or closing FND-1. |
| **Subject** | prd:forgejo-feature-draft/draft |
| **Moment** | draft |
| **Reviewer** | prd-review |
| **Date(s)** | 2026-10-03 |
| **Verdict** | request-changes |
| **Tracking** | https://github.com/agenticlab-ai/pyxis/pull/1365 |
| **Template version** | review-report/v1 |

**Review subject:** prd  
**Draft digest:** sha256:ade0bfd49a5a0b21f3c61701a2c2e79e07904a2f952b031af579d85c3a92ec7e

## Round 1

I inspected the PRD and tech spec from committed head `d2f0e5640858cf1e9f0fce3736f6acc3a84c0ef1`, excluding human-only §0. I checked the draft’s brainstorm 003 and P-8 ruling, the PRD template, relevant vision, principles, incident and frozen feature-draft contract slices, and the earlier review reports as historical evidence. I performed read-only inspection; I did not run gates, tests, or network checks. Reports 004/005 and 006/007 do not supply this episode’s verdict.

The contract-quality lens found observable fit criteria for the Must rows. The coherence lens found blocking DoD coverage for their stated flows, including P-8’s abandonment criteria. The upstream-alignment and completeness-critic lenses identified the same surviving mainline-write gap below; it is recorded once.

### FND-1 — Blocking acceptance covers merges but not direct mainline writes

- **Severity**: blocking (CAL-2)
- **Lens**: upstream-alignment
- **Evidence**: PRD FR-5 requires enforcement “before permitting a prepared contract to merge,” and its fit criterion and DOD-3 require negative *merge* attempts. A policy could pass those cases while allowing an agent or privileged actor to push compiled records directly to mainline, including through a path exemption. FR-6 could refuse a lock tag afterward, but the records would already have landed. Frozen `design:feature-draft-v1 #DINV-1` requires open-draft records to stay off mainline before lock; its §3.9 requires enforced mainline protection. The current tech spec D-5 explicitly requires no direct-push or file-pattern bypass for compiled records, but the PRD’s blocking evidence does not test that obligation.
- **Acceptance condition**: Make FR-5’s protection requirement and blocking DOD-3 evidence cover direct and force-push paths, privileged actors, and path-rule exemptions that could land compiled contract records without the configured human’s exact-head approval. Require selected-host native negative fixtures showing mainline remains unchanged, and refusal of preparation when effective protection cannot be established.
- **Disposition**: declined — operator · P-9 on 2026-10-04 accepts the explicitly scoped installed-Forgejo trusted-governance model in place of bypass-enforcement proof; authenticated exact-head human review and verified merged artifacts remain required. The original acceptance condition is not claimed implemented, GitHub behavior remains, and separate human lock approval is pending.
- **Author evidence**: Operator P-9 ruling on2026-10-04 (“ok, your proposa is aproved.”) selects the explicitly narrower installed-Forgejo trusted-governance model instead of the proposed bypass-enforcement tests. FR-5/DOD-3, PRD scope/risk, tech-spec D-5 and affected references, and brainstorm003 D-2 now declare that boundary; authenticated exact-head review and merged-artifact verification remain required, and GitHub behavior is retained. The original acceptance condition is not claimed implemented. Decline auto-escalation and the operator's supplied conflict resolution are recorded at P-9; the independent round2 still decides its verdict. Original native008 bytes/blob/hash/EOF remain archived in Source journal E-197 and the ignored native final.

## Verdict history

| Round | Verdict | Date |
|---|---|---|
| 1 | request-changes | 2026-10-03 |

## Escalation brief

none

## Measurements

- **Rounds**: 1
- **Findings**: 1 blocking · 0 advisory
- **Dispositions**: 0 applied · 1 declined · 0 withheld-as-violation · 0 pending

## Overlay corrections

none
