# Review: planner-driven-model-routing amendment 002 — review report

| Field | Value |
|---|---|
| **In one line** | Approve after defining top-level mutually blind assurance passes, preserving the merge-lane→impl-review→routing ownership chain, adding conservative aggregation, and making the universal review floor measurable over every proposal. |
| **Relevant-when** | auditing or consuming amendment 002; asking why critical assurance requires two top-level passes rather than one review's remediation round; tuning prd-review calibration against universal-review findings |
| **Subject** | `prd:planner-driven-model-routing/002` |
| **Moment** | amendment |
| **Reviewer** | prd-review · 2026-07-26 |
| **Date(s)** | 2026-07-26 (rounds 1–2) |
| **Verdict** | approve |
| **Tracking** | — *(introducing PR not filed)* |
| **Template version** | review-report/v1 |

## Round 1

### FND-1 — the proposed independent rounds were not mechanically distinguishable

- **Severity**: blocking (CAL-3)
- **Lens**: contract-quality
- **Evidence**: the initial FR-25 draft required two “mutually independent deep model-backed rounds” and its fit criterion checked only distinct fresh-context provenance — vs — `prd:impl-review/FR-1`, where a post-change scope-locked remediation run is itself a review round, so distinct contexts alone could not prove two independent top-level assurance passes.
- **Acceptance condition**: Define an assurance pass as a top-level exact-revision review distinct from internal remediation re-review; require same-revision binding, distinct top-level contexts, no cross-pass session/output leakage, and negative fixtures for shared context, mismatched revision, and remediation miscount.
- **Disposition**: applied

### FND-2 — the normative assurance vocabulary contradicted the exact floor

- **Severity**: blocking (CAL-1)
- **Lens**: coherence
- **Evidence**: the folded §5.1 vocabulary still said ordinary review “may follow the actual proposal lane,” elevated was merely “at least standard,” and critical was only “deep and cannot be skipped” — vs — initial FR-25's exact `standard×1`, `deep×1`, and `deep×2` mapping.
- **Acceptance condition**: Supersede §5.1 so every assurance value names its FR-25 floor and critical names FR-26 aggregation; leave no zero-review interpretation in the live vocabulary.
- **Disposition**: applied

### FND-3 — live flows and readiness rows still cited superseded contract units

- **Severity**: blocking (CAL-1)
- **Lens**: coherence
- **Evidence**: the first fold left F2/F3 pointing to UB-5, FR-18 and NR-2 in delivery/readiness paths, DOD-7 in supportability, and a singular F3 review lifecycle — vs — the amendment's own FR-25/UB-12 replacements and the PRD-ledger rule that live prose consumes the folded effective contract rather than superseded rows.
- **Acceptance condition**: Update every affected flow, scenario, dependency, release, rollout, post-launch, and readiness span to live identifiers; expose band projection, flooring, pass-set execution, aggregation, and exact-revision rerun as distinct F3 states; keep old identifiers only in lineage and generated superseded history.
- **Disposition**: applied

### FND-4 — release could not prove review depth, pass count, blindness, or aggregation

- **Severity**: blocking (CAL-4)
- **Lens**: completeness-critic
- **Evidence**: the first fold relied on prose-level “review reports” while §5.7 had no typed process terminal and §3.2 had no complete proposal denominator — vs — §8's claim that every primary metric had instrumentation and AP-29's requirement that a gate consume explicit measurement evidence.
- **Acceptance condition**: Add one immutable exact-revision review-process entity and typed EVT-9 terminal carrying required/completed passes, top-level contexts, per-pass verdicts, and aggregate result; add M6 over every implementation proposal and make it blocking in guardrails, DoD, release, and post-launch checks.
- **Disposition**: applied

### FND-5 — two valid passes could disagree without a fail-closed aggregate

- **Severity**: blocking (CAL-4)
- **Lens**: completeness-critic
- **Evidence**: initial FR-25 required two critical rounds but specified no rule for an absent, stale, invalid, or non-accepting pass, conflicting findings, or favorable-result selection — vs — AP-15's exact-state binding and §8's safety-readiness claim for the critical review surface.
- **Acceptance condition**: Keep passes blind until terminal, union surviving findings, select the most restrictive verdict, make any missing/non-accepting pass prevent acceptance, stale the whole set after head change, and cover pass-order, split-verdict, and favorable-subset fixtures.
- **Disposition**: applied

### FND-6 — the amendment blurred risk classification and band-projection ownership

- **Severity**: blocking (CAL-2)
- **Lens**: upstream-alignment
- **Evidence**: the initial NR-9 and §5.5 treated the merge lane as supplying the final `skip|standard|deep` band directly to review routing — vs — `prd:impl-review/FR-9`, which reads the merge lane's risk class and solely applies the review band, and AP-11's single-mechanical-rule constraint.
- **Acceptance condition**: Preserve the merge lane as sole actual-risk classifier, impl-review as sole risk-class→band projector, FR-25 as the only planned-assurance join, and harness adapters as native resolvers only; require unchanged classifier and projector fixtures.
- **Disposition**: applied

### FND-7 — the decision log attributed an unresolved exact table to D-1

- **Severity**: blocking (CAL-2)
- **Lens**: upstream-alignment
- **Evidence**: the initial §7.3 row cited `design-brainstorm:planner-driven-model-routing-tech-design-2#D-1` as the source of ordinary `standard×1`, elevated `deep×1`, and critical `deep×2` — vs — D-1's explicit statement that only the universal monotone floor was decided and “the exact higher-risk mapping … remains unresolved until the governing PRD is amended.”
- **Acceptance condition**: Attribute only the universal floor and monotone direction to D-1, and identify amendment 002's operator-approved contract as the resolution of the exact mapping and aggregation policy.
- **Disposition**: applied

## Round 2

### FND-8 — FR-25's inline enum corrupted its generated source metadata

- **Severity**: blocking (CAL-3)
- **Lens**: contract-quality
- **Evidence**: the first corrected FR-25 spelled the proposal band as an escaped-pipe inline enum inside a Markdown table cell, causing `effective.source.json` to read part of the requirement as `priority` and `critical` as its verification method — vs — AP-6's requirement that cross-boundary contract data remain schema-versioned and mechanically trustworthy.
- **Acceptance condition**: Express closed enum values without literal table separators, refold, and prove FR-25's generated metadata reports `priority: Must` and `verify_method: Test + Demo`; apply the same safe spelling to the new event and interface rows.
- **Disposition**: applied

No further findings survived the scope-locked re-review of FND-1..FND-8 and the corrected spans. The final fold distinguishes top-level assurance passes from remediation, preserves all three ownership seams, contains no live stale references, derives a conservative exact-revision aggregate, connects M6 to EVT-9 and every release gate that claims universal-review conformance, and retains correct typed source metadata for FR-25.

## Verdict history

| Round | Verdict | Date |
|---|---|---|
| 1 | request-changes | 2026-07-26 |
| 2 | approve | 2026-07-26 |

## Escalation brief

none

## Measurements

- **Rounds**: 2
- **Findings**: 8 blocking · 0 advisory
- **Dispositions**: 8 applied · 0 declined · 0 withheld-as-violation

## Overlay corrections

none
