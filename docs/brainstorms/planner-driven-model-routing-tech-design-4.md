# Planner-driven model routing — third reopened design-brainstorm record

| Field | Value |
|---|---|
| **In one line** | Finalize the six runtime design boundaries that carry planner-measured demand into harness-native implementation and AI-review model/effort choices. |
| **Relevant-when** | designing or changing plan/v2 execution demand, harness-native model/effort resolution, routed-attempt evidence, AI-review process selection, or implement-now's routed composition |
| **Basis** | prd:planner-driven-model-routing · prd:plan-new · prd:implement-now · prd:impl-review *(merged ledgers on 2026-07-29)* |
| **Authority** | @mdarabi |
| **Date(s)** | 2026-07-29 |
| **Status** | gated: ready-for-design |
| **Tracking** | — |
| **Feeds** | docs/tech-design/execution-demand-contract-v1.md · docs/tech-design/plan-new-routed-demand.md · docs/tech-design/native-routing-policy-v1.md · docs/tech-design/routing-evidence-v1.md · docs/tech-design/impl-review-routed-assurance.md · docs/tech-design/implement-now-routed-execution.md |
| **Prior records** | design-brainstorm:planner-driven-model-routing-tech-design-3 · design-brainstorm:planner-driven-model-routing-tech-design-2 · design-brainstorm:planner-driven-model-routing-tech-design |
| **Template version** | design-brainstorm-record/v1 |

## Orientation

| Source | Why loaded |
|---|---|
| docs/prds/planner-driven-model-routing/effective.md | merged runtime contract: planner-owned raw workload/consequence measurements, four semantic demand minima, role-specific harness-native resolution, universal review floor, canonical routing evidence, and direct activation |
| docs/prds/plan-new/effective.md | merged producer contract: assessment evidence and the raw plus derived demand freeze into plan/v2 without native model identifiers |
| docs/prds/implement-now/effective.md | merged implementation consumer: selected harness is upstream, the implementation role resolves natively, append-first evidence gates claim/launch, and review is an aggregate admission boundary |
| docs/prds/impl-review/effective.md | merged review consumer: at least one model-backed pass for all work and two mutually blind concurrent passes for critical/high-band work over one cacheable shared prefix |
| docs/brainstorms/planner-driven-model-routing-tech-design-3.md | frozen predecessor; D-2..D-5 already decide immutable plan/v1 translation, append-before-launch evidence, concurrent blind passes with shared-prefix caching, and full pre-release proof followed by one direct production switch |
| docs/tech-design/routing-envelope-v1.md · docs/tech-design/plan-new-v1.md · docs/tech-design/implement-now.md · docs/tech-design/impl-review-v1.md | live predecessors whose scalar/plan-v1/hardcoded or older review bindings are made false by the merged runtime contracts and therefore require successors |
| docs/tech-design/execution-demand-contract-v1.md · docs/tech-design/native-routing-policy-v1.md · docs/tech-design/routing-evidence-v1.md · docs/tech-design/impl-review-routed-assurance.md · docs/tech-design/implement-now-routed-execution.md | parked draft material from the prior session; reusable only after dependency direction, current Serves coverage, and the six-record union are corrected |
| docs/tech-design/index.json · docs/architecture-principles.md · docs/observability/incidents.md | live design graph, AP/RW rules, and I1–I5 integrity checks |

**Overlaps surfaced:** The execution-demand schema is shared by its plan producer, implementation consumer, and review consumer, so it remains independent of plan-new. Native resolution policy changes for catalog/model reasons while routing evidence changes for retention/recovery reasons, so neither belongs inside the other. Impl-review remains a standalone product consumed by implement-now; implement-now remains the top-level routed composition and owns the one activation predicate. A separate repository-governance prerequisite is tracked in GitHub issue #806 and is not part of these runtime boundaries.

## Design questions

- **DQ-1**: What is the smallest independently revisable runtime design and supersession graph that carries semantic demand from planning through harness-native implementation and AI review without cycles or duplicated policy/evidence ownership?

## Panel log

- **DQ-1**: opens/replaces — fired — the partition creates or replaces six runtime seams; minimal-record, integrity-first, and seam-pure panels compared the selected six records against folding demand into plan-new and adding a terminal activation composition.

## Decisions

### D-1 — Six runtime records; adapters map semantic demand natively

- **Question**: What is the minimum honest model-routing design graph once repository-governance mechanics are excluded?
- **Answer**: Use four true successors—execution demand, plan-new routed demand, impl-review routed assurance, and implement-now routed execution—plus new native-routing-policy and routing-evidence records. Planner outputs raw measurements and four semantic minima only; each selected harness adapter maps role-qualified demand to its native model/effort profile.
- **Reasoning**: Each record owns a distinct reason to change. Folding demand into plan-new hides a shared multi-consumer contract; folding policy into evidence couples choice to retention; folding review into execution erases an independently amended product; a separate activation record has no independent lifecycle because implement-now is already the top-level composition.
- **Anchor**: 2026-07-29 · “yes”
- **Status**: live

## Alternatives

### ALT-1 — Six independently revisable runtime records

- **Shape**: `execution-demand-contract-v1` supersedes `routing-envelope-v1`; `plan-new-routed-demand` supersedes `plan-new-v1` and produces that contract; separate native policy and routing evidence consume it; the impl-review successor owns routed AI-review assurance; the implement-now successor consumes plan demand, implementation routing, evidence, and the review aggregate.
- **Trade-offs**: six coordinated designs and a union proof, but schema, producer, model-policy, evidence, reviewer, and executor evolution remain isolated and every contradicted predecessor gets exactly one successor.
- **Principles**: honors AP-1, AP-3, AP-5, AP-6, AP-10, AP-11, AP-13, AP-15, AP-17, AP-20, AP-21, AP-22, AP-24, AP-28, AP-29, AP-30, RW-4, and RW-5.
- **Incidents**: I1 merged requirements precede design; I2 requires graph-coherent mainline proof; I3 closed demand/catalog/event parsing; I4 six-record union coverage; I5 one hermetic derivation/resolution/evidence predicate across local and CI.
- **Verdict**: chosen by D-1 — smallest partition that keeps every real cross-boundary contract and independently changing policy/runtime owner visible.
- **From**: DQ-1 · panel

### ALT-2 — Five records by folding demand into plan-new

- **Shape**: Let the plan-new successor own the raw assessment, four semantic minima, plan/v2 carrier, validation, demand identity, and plan/v1 translation; retain native policy, evidence, reviewer, and executor records.
- **Trade-offs**: removes one document, but makes the implementation/review input contract an internal planner detail and forces consumers to depend on producer orchestration for their own boundary validation.
- **Principles**: strains AP-1, AP-11, AP-20, AP-21, AP-28, and RW-5 by collapsing a multi-consumer contract into one producer.
- **Incidents**: I4 loses an independently auditable demand boundary; I5 risks producer/consumer validation drift.
- **Verdict**: rejected — it hides a seam rather than removing one.
- **From**: DQ-1 · panel

### ALT-3 — Seven records with terminal activation composition

- **Shape**: Split implement-now into a routed executor core with consumer-owned review/verification ports and a terminal composition record wiring impl-review, code gates, e2e, FeatureDrive, and the direct activation predicate.
- **Trade-offs**: pure dependency topology and independently revisable integration, but adds a one-consumer record for responsibilities already owned by the top-level implement-now product and reopens the prior direct-cutover placement with no new requirement.
- **Principles**: strongly honors AP-1/AP-20/RW-5 seam purity but strains AP-20/AP-27 with an extra coordination surface.
- **Incidents**: I2/I4 gain another landing and union object; I5 still needs the same one activation predicate.
- **Verdict**: rejected — keep its consumer-owned-port discipline inside the implement-now successor until composition has a second lifecycle.
- **From**: DQ-1 · panel

## Seam map

- **S-1**: execution-demand contract — replaces — behind it: six raw assessment axes, four semantic minima, assessment evidence, canonical demand identity, plan/v2 carrier/validation, and immutable plan/v1 translation — feeds: execution-demand successor Scope/Serves
- **S-2**: plan-new demand producer — replaces — behind it: planner-owned measurement/evidence and freeze/projection of raw assessment plus semantic minima, with no native model/effort names — feeds: plan-new successor Scope/Serves
- **S-3**: harness-native routing policy — opens — behind it: role-qualified native catalogs, qualification, dominance, least-cost resolution, exact/promotion/unsupported results, and same-harness strongest fallback — feeds: native-routing-policy Scope/Serves
- **S-4**: canonical routing evidence — opens — behind it: append-first resolution/failure permit, outcomes, review-process events, invalidation, exact-read, and recovery — feeds: routing-evidence Scope/Serves
- **S-5**: routed AI-review assurance — replaces — behind it: assurance×band pass topology, universal one-pass floor, concurrent mutually blind passes over one cacheable shared prefix, aggregation, and remediation lineage — feeds: impl-review successor Scope/Serves
- **S-6**: routed executor composition — replaces — behind it: implementation resolution, Worker lifecycle, evidence admission, review aggregate admission, union proof, and one direct production activation — feeds: implement-now successor Scope/Serves

## Facts

- **F-1**: planner assessment carries workload dimensions `reasoning`, `coupling`, `novelty`, and `verification`, plus consequence dimensions `impact_scope` and `architectural_role`; plan/v2 preserves both evidence-backed raw values and the derived semantic profile — source: prd:planner-driven-model-routing/FR-22 · FR-3..FR-7 · prd:plan-new/FR-28..FR-30
- **F-2**: the derived demand is four independent minima—capability, deliberation, context, and assurance—not one combined complexity scalar and never a native model name — source: prd:planner-driven-model-routing/FR-3..FR-6
- **F-3**: the already-selected harness and role are resolver inputs; each harness adapter owns its native model/effort catalog and mapping while the shared engine owns qualification/dominance/resolution semantics — source: prd:planner-driven-model-routing/FR-8..FR-13
- **F-4**: all work receives at least one model-backed review pass; critical assurance at high band receives two mutually blind passes, and the shared context prefix is reused for cache efficiency without shared top-level context identity — source: prd:planner-driven-model-routing amendment 002 · prd:impl-review
- **F-5**: route/failure evidence must be durably appended and exact-read before implementation claim or reviewer launch — source: prd:planner-driven-model-routing amendment 003 · prd:implement-now

## Fog

none

## Out of scope

- **O-1**: repository contract-currency and compatibility-approval mechanics live in GitHub issue #806 and must not enter routing data, ports, catalogs, evidence, or runtime control flow (from D-1)
- **O-2**: harness selection remains upstream; routing selects a native model/effort only within the already-selected harness and role (from D-1)
- **O-3**: production shadowing, canaries, cohorts, or restoration of the hardcoded/rung path remain excluded; activation follows full pre-release proof and one direct switch, with only same-harness strongest-profile fallback (from prior D-5)

## Candidate glossary terms

- **T-1**: execution demand — the frozen harness-neutral contract containing raw planner assessment, evidence, and the four semantic minima required by downstream implementation and review · avoid: complexity score, model tier
- **T-2**: harness model adapter — the selected-harness implementation of native catalog enumeration and role-qualified model/effort realization under shared resolver semantics · avoid: approval adapter, planner router
- **T-3**: resolved route — the exact harness, role, native model, effort/profile, policy, demand, and attempt identity durably admitted before launch · avoid: suggested model

## Parked product gaps

none

## Gate

| Field | Value |
|---|---|
| **Verdict** | ready-for-design |
| **Date** | 2026-07-29 |
| **Authority** | @mdarabi (operator) |
| **Covering design** | — |
| **Coverage check** | pass — core four covered, zero blocking fog. §2 direction: D-1. §4 alternatives: ALT-1..ALT-3, all verdicted and the fired panel has three scored alternatives. Scope/Serves: S-1..S-6 against the four-PRD basis. Boundaries: O-1..O-3. Fog: none. |
| **Amend route** | — |

*Verdict anchor: 2026-07-29 · “kick off the design” — the authority invoked the `tech-design-new` handoff on the recommended ready-for-design gate.*
