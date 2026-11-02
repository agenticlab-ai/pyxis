# Review: forgejo-feature-draft — design-review report

| Field | Value |
|---|---|
| **In one line** | approve: the revised contract closes both round-1 findings without a surviving changed-span regression. |
| **Relevant-when** | Reviewing the Forgejo feature-draft tech spec or auditing this design verdict |
| **Subject** | design:forgejo-feature-draft · shape: none · supersedes: — |
| **Moment** | draft |
| **Reviewer** | tech-design-review · scope-locked round 2 |
| **Date(s)** | 2026-10-02 |
| **Verdict** | approve |
| **Tracking** | https://github.com/agenticlab-ai/pyxis/pull/1365 |
| **Template version** | design-review-report/v1 |

**Review subject:** tech-spec  
**Draft digest:** sha256:ade0bfd49a5a0b21f3c61701a2c2e79e07904a2f952b031af579d85c3a92ec7e

## Round 1 lineage

[Report 006](../reviews/006.md) reviewed committed head `b3135835369ce664803bbd69934d1d8969bea865` with digest `sha256:04f7283633c658e2456e4a230212b6df6a2802f559ceffa984faaf5fb91a7f91` and returned **request-changes**. Its findings and recorded author dispositions carry forward:

### FND-1 — Review IDs do not establish submission order

- **Severity**: blocking (CAL-1)
- **Lens:** coherence
- **Evidence:** D-6 required the configured operator’s latest submitted review, but the shared selector chose the greatest numeric review ID. A lower-ID pending review submitted as a rejection after a higher-ID approval could leave the approval selected, contrary to FR-6, UB-4, and DOD-3.
- **Acceptance condition:** Establish submission order or refuse uncertain order; specify a real-schema lower-ID, later-rejection case that produces no lock or tag while preserving pending and comment-only behavior.
- **Disposition**: applied
- **Author evidence**: the corresponding D-3/D-6 design guarantee and real-schema regression checks are revised; runtime implementation evidence remains required after the lock.

### FND-2 — An empty read after an ambiguous create does not prove absence

- **Severity**: blocking (CAL-4; load-bearing for DOD-1)
- **Lens:** feasibility-completeness
- **Evidence:** D-3 allowed enumeration before retry after an ambiguous Create without establishing that an empty result proved the first POST had not committed. A second POST could violate FR-3, UB-6, and DOD-1’s one-proposal obligation.
- **Acceptance condition:** Forbid a second POST until authoritative negative evidence or a verified provider uniqueness or idempotency guarantee makes it safe; specify a lost-response, temporarily empty-enumeration case that retains one proposal.
- **Disposition**: applied
- **Author evidence**: the corresponding D-3/D-6 design guarantee and real-schema regression checks are revised; runtime implementation evidence remains required after the lock.

## Round 2

I reviewed exact current head `d5dc34d3e4e33016bde9812712b32d9499a7d130`; the tracked tree was clean. This re-review used report 006, the supplied computed tech-spec delta, and the supplied affected PRD slice. The PRD is byte-identical to round 1. I ran no tests, gates, network actions, or native server probes.

| Finding | Closure and changed-span regression conclusion |
|---|---|
| FND-1 | **Closed.** D-6 now orders submitted terminal operator reviews by validated native `submitted_at` instants in the shared selector, with no ID fallback. Missing, malformed, conflicting, or equally latest timestamps refuse approval. Both native reader fixtures specify the lower-ID review submitted as a later rejection and require no lock or tag; they also cover pending and comment-only reviews. No changed-span regression survives against FR-6, UB-4, DOD-3, or NR-1. |
| FND-2 | **Closed.** D-3 now publishes a recoverable pending-create intent before Create and keeps ambiguous outcomes pending through empty or stale enumeration. Another POST requires proved negative evidence or a verified provider guarantee; the initial implementation assumes neither. The specified real-schema lost-response fixture requires one total POST across same-process, restart, and fresh-clone recovery. No changed-span regression survives against FR-3, UB-3, UB-6, or DOD-1. |

The corrections retain the existing lifecycle and approval seams. This is a contract verdict; runtime behavior remains for post-lock implementation and acceptance evidence.

## Verdict history

| Round | Verdict | Date |
|---|---|---|
| 1 | request-changes | 2026-10-02 |
| 2 | approve | 2026-10-02 |

## Escalation brief

none

## Measurements

- **Rounds:** 2
- **Historical round-1 findings:** 2 blocking · 0 advisory; both closed in round 2
- **New round-2 findings:** 0 blocking · 0 advisory
- **Dispositions:** 2 applied · 0 declined · 0 withheld-as-violation · 0 author-pending

## Overlay corrections

none