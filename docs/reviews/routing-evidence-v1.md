# Review: routing-evidence-v1 — design-review report

| Field | Value |
|---|---|
| **In one line** | Approved: append-first evidence now has a closed reducer, reproducible identity, independently verifiable producers, launch-time fencing, and a traceable sole-network boundary. |
| **Relevant-when** | auditing or consuming the review of routing-evidence-v1; asking why its append, reducer, recovery, or authority protocol was approved or bounced; tuning tech-design-review calibration |
| **Subject** | design:routing-evidence-v1 · shape: none · supersedes: — |
| **Moment** | new-design |
| **Reviewer** | tech-design-review · 2026-07-28 guide |
| **Date(s)** | 2026-07-28 |
| **Verdict** | approve |
| **Tracking** | — *(the subject's introducing PR, once filed)* |
| **Template version** | design-review-report/v1 |

## Round 1

Amendment 003 changed the governing basis after an earlier unfrozen review, so this report was re-instantiated rather than recording an illegal third round. The earlier local corrections remain in the subject: producer-kind permissions, single unresolved review-pass admission, complete normalized finding/disposition evidence, and exact upstream event names. The amendment itself closed the former outbound-call contradiction and restored typed EVT-8 provenance. Those paid-for corrections were rechecked but are not restated as current defects.

### FND-1 — reducer states and multi-file terminal facts are not closed

- **Severity**: blocking (CAL-1)
- **Lens**: coherence
- **Evidence**: design: §3.2 requires one outcome for each resolved attempt while §3.4 must expose a resolved-without-outcome launch permit; §3.5 and §3.6 split one implementation outcome and one pass/finding set across multiple canonical files without a mapping, atomicity rule, or partial-state predicate — vs — `prd:planner-driven-model-routing/FR-20`, `prd:implement-now/FR-25`, and `prd:impl-review/FR-27`.
- **Acceptance condition**: define closed in-flight/terminal reducer states, the exact parent↔implement-now outcome mapping, atomic completeness for outcome and pass/finding composites, and crash/prefix fixtures proving partial sets never authorize a process or projection.
- **Disposition**: applied

### FND-2 — canonical identity cannot be reproduced after local-state loss

- **Severity**: blocking (CAL-4)
- **Lens**: feasibility-completeness
- **Evidence**: design: §3.1 hashes semantic identity and canonical payload while permitting descriptive timestamps but defines neither a canonical byte codec nor per-type stable identity preimage — vs — #DINV-3 and A-1/A-2 idempotency.
- **Acceptance condition**: specify one versioned canonical encoding, exact event-ID/content-digest preimages, treatment of absent/null/set/order/string/time fields, and cross-producer/time-varying retry goldens.
- **Disposition**: applied

### FND-3 — producer authority is not independently verifiable

- **Severity**: blocking (CAL-3)
- **Lens**: contract-quality
- **Evidence**: design: §3.10 trusts protected-git authorization and §3.12 names logical producer roles/proofs without a trust root or a proof bound to event bytes; a direct Git write can bypass append-port validation — vs — #DINV-10 and A-9, which say generic ref-write credentials cannot confer semantic authority.
- **Acceptance condition**: define the producer-proof format and trust roots, bind proof to repository/ref/kind/generation/event bytes, require fresh-reader verification, define fast-forward/genesis continuity protection, and exercise direct-push/replay/stale-generation/force/delete abuse cases on local and hosted adapters.
- **Disposition**: applied

### FND-4 — crash recovery has no fenced ownership-transfer protocol

- **Severity**: blocking (CAL-4)
- **Lens**: feasibility-completeness
- **Evidence**: design: §3.5 assigns terminal responsibility to an admitted owner/reaper but defines no durable generation, takeover proof, review-attempt observation contract, or old-owner fence — vs — #DINV-7/#DINV-8 and A-4/A-5 exactly-one-outcome/no-duplicate-launch claims.
- **Acceptance condition**: define canonical owner generations, typed runtime observations for implementation and review, positive transfer evidence, stale-owner fencing, terminal ownership, and two-owner/owner-revival/host-loss/unknown-state fixtures.
- **Disposition**: applied

### FND-5 — the sole permitted network boundary is not a binding proof

- **Severity**: blocking (CAL-2)
- **Lens**: upstream-alignment
- **Evidence**: design: §3.4/A-3 require durable evidence before claim/invocation but do not assert or test that resolution is offline and append/exact-read are the only network operations afterward — vs — `prd:planner-driven-model-routing/NFR-8`, `prd:implement-now/NFR-9`, and `prd:impl-review/NFR-12`.
- **Acceptance condition**: bind the offline-resolution/post-terminal network interval as an invariant and require implementation/review boundary traces plus outage fixtures proving zero inference and no other network operation.
- **Disposition**: applied

## Round 2 — scope-locked

*Terminal round. Scope was limited to whether the five round-1 acceptance conditions were met and whether their changed spans introduced a regression; unchanged content was not re-reviewed. All five findings are closed and no changed-span regression survived adversarial filtering. `git diff --check`, `docgate lint-tech-designs`, `docgate check-citations`, and `docgate verify-index` are green on the reviewed tree.*

- **FND-1 — closed.** §3.2 now enumerates the valid in-flight and terminal reducer states. §§3.3/3.5/3.6 and DINV-13 define atomic outcome and pass/finding composites, the exact three parent-to-implement outcome mappings, and non-acceptance of every partial prefix. A-2/A-4/A-6 own crash, prefix, mapping, and bijection fixtures.
- **FND-2 — closed.** §3.1 defines one versioned canonical codec and a non-recursive three-stage identity/content/proof construction. Its exhaustive per-kind identity table and explicit exclusions make bytes, IDs, digests, retries, and same-identity conflicts independently reproducible; A-1 owns the preimage/exclusion goldens.
- **FND-3 — closed.** §3.12 closes event-class authorization over configured Ed25519 trust roots and generation-scoped producers, with proofs bound to the exact unsigned envelope. §3.10 requires every fresh reader to validate proof and genesis/ref continuity; A-9 owns bypass, replay, stale-generation, force, and delete cases.
- **FND-4 — closed.** §§3.1/3.12 make ownership transfer a canonical, authorized event. §3.5's `LaunchFenced` consumes a generation capability at the actual spawn boundary while `FenceAndObserve` atomically revokes it before producing transfer proof, so a stale spawn racing transfer must fail. A-5 exercises the actual spawn race for implementation and claimless review.
- **FND-5 — closed.** §3.4 and DINV-15 bind offline resolution and the post-terminal interval in which canonical append/exact-read are the only network operations. A-3 requires full implementation/review boundary traces and fail-closed outage cases against the exact three amended NFRs.

The subject is approved on its reviewed basis.

## Verdict history

| Round | Verdict | Date |
|---|---|---|
| 1 | request-changes | 2026-07-28 |
| 2 | approve | 2026-07-28 |

## Escalation brief

none

## Measurements

- **Rounds**: 2
- **Findings**: 5 blocking · 0 advisory
- **Dispositions**: 5 applied · 0 declined · 0 withheld-as-violation

## Overlay corrections

- **2026-07-29 — tech-design-review:** inspected the post-approve §1 provenance re-anchor and §4 partition alternative. Both are explanatory only: append-first admission, canonical git evidence, recovery, and projection contracts are unchanged. Verdict remains **approve**.
