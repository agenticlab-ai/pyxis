# Review: forgejo-feature-draft — design-review report

| Field | Value |
|---|---|
| **In one line** | request-changes: review ordering and ambiguous-create recovery lack the guarantees needed for an exact-head lock and one proposal per version. |
| **Relevant-when** | Reviewing or revising the Forgejo feature-draft tech spec; auditing this design verdict |
| **Subject** | design:forgejo-feature-draft · shape: none · supersedes: — |
| **Moment** | draft |
| **Reviewer** | tech-design-review · draft-born guide at the reviewed Git head |
| **Date(s)** | 2026-10-02 |
| **Verdict** | request-changes |
| **Tracking** | https://github.com/agenticlab-ai/pyxis/pull/1365 |
| **Template version** | design-review-report/v1 |

**Review subject:** tech-spec  
**Draft digest:** sha256:04f7283633c658e2456e4a230212b6df6a2802f559ceffa984faaf5fb91a7f91

## Round 1

I reviewed exact committed head `b3135835369ce664803bbd69934d1d8969bea865` on `draft/forgejo-feature-draft-v2`. The tracked tree was clean. The committed PRD and tech-spec bytes recompute to the digest above.

The subject was `drafts/forgejo-feature-draft/tech-spec.md`. I read its committed PRD basis with the human-only §0 excluded, the current `brainstorm/003.md`, shared problem rulings and relevant delivery entries, and completed PRD report 005 as context. I loaded the product vision, architecture principles, and incident registry, then inspected only source slices needed to assess the proposed host reads, approval selector, and retry behavior. I ran no tests, gates, native server probes, or network actions.

| Lens | Round 1 inspection |
|---|---|
| contract-quality | Checked D-1 through D-7 against their concrete layouts and verification clauses. The two findings below identify checks needed to make binding outcomes decidable. |
| coherence | Checked decision-to-layout and cross-decision agreement, especially D-3’s single-proposal recovery and D-6’s latest-submitted-review claim. |
| upstream-alignment | Checked the PRD’s Must and blocking DoD substance, the v2 brainstorm’s repository-policy ruling, relevant principles, and incident lessons. The design preserves the attributed abandonment ruling; neither finding disputes it. |
| feasibility-completeness | Checked native observation boundaries, partial effects, credentials, protection, review selection, tag proof, and recovery. No implementation or live provider behavior is claimed as verified. |

### FND-1 — Review IDs do not establish submission order

- **Severity**: blocking (CAL-1)
- **Lens**: coherence
- **Evidence**: Tech spec D-6 says the configured operator’s *latest submitted* review for H controls the lock, a newer rejection blocks it, and the Forgejo reader maps native reviews into the existing `approval.Select`. That selector chooses the greatest numeric review ID, not the latest submission (`internal/draft/approval/port.go:37-59`); its `Review` value has no submission time. The existing Forgejo native review shape carries both `id` and `submitted_at`, as well as a `PENDING` state (`internal/adapters/codehost/forgejo/state.go:24-35`). D-6 supplies no native guarantee that ID order equals submission order. A lower-ID pending review submitted as a rejection after a higher-ID approval would leave the selector choosing the approval. PRD FR-6, UB-4, and blocking DOD-3 require refusal when a later operator rejection supersedes approval.
- **Acceptance condition**: Specify in D-6 how the adapter establishes submission order and makes uncertain ordering refuse, or provide tested native evidence that IDs order submitted reviews on the supported Forgejo version. Bind a real-schema sequence in which a lower-ID pending review is submitted as a rejection after a higher-ID approval; it must produce no lock or tag while the existing pending/comment-only behavior remains intact.
- **Disposition**: applied — the corresponding D-3/D-6 design guarantee and real-schema regression checks are revised; runtime implementation evidence remains required after the lock.

### FND-2 — An empty read after an ambiguous create does not prove absence

- **Severity**: blocking (CAL-4; load-bearing for DOD-1)
- **Lens**: feasibility-completeness
- **Evidence**: Tech spec D-3 says that, after a lost or ambiguous Create response, bounded native enumeration/read precedes retry, while its decision promises one proposal. D-4 explicitly anticipates eventually stale native readback; D-3 states no stronger absence guarantee for the create path. The reusable Forgejo `ListExact` can complete with an empty result when a page has no continuation or count (`internal/adapters/common/forgejo/client.go:242-295`); that result alone cannot establish whether an earlier POST committed but is not yet visible. Retrying Create on that basis can violate PRD FR-3 and UB-6’s one-PR recovery obligation and blocking DOD-1’s one-proposal evidence. AP-15 and AP-22 require consequential reads and retries to preserve that invariant.
- **Acceptance condition**: Specify that an ambiguous Create cannot cause a second POST until an authoritative negative observation or a verified provider-side uniqueness/idempotency guarantee makes it safe; otherwise retain a pending reconciliation state. Bind a fixture where the first POST creates the proposal, its response is lost, and the next enumeration temporarily returns zero; recovery must retain exactly one proposal.
- **Disposition**: applied — the corresponding D-3/D-6 design guarantee and real-schema regression checks are revised; runtime implementation evidence remains required after the lock.

These are contract and acceptance gaps. I did not establish that either failure occurs on a live Forgejo installation.

## Verdict history

| Round | Verdict | Date |
|---|---|---|
| 1 | request-changes | 2026-10-02 |

## Escalation brief

none

## Measurements

- **Rounds**: 1
- **Findings**: 2 blocking · 0 advisory
- **Dispositions**: 2 applied · 0 declined · 0 withheld-as-violation · 0 author-pending

## Overlay corrections

none