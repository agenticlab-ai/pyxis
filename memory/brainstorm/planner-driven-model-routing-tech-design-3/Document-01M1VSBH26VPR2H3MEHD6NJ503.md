# Planner-driven model routing — second reopened design-brainstorm record

| Field | Value |
|---|---|
| **In one line** | Preserve the approved routing semantics but bounce the design partition until contract-gate can distinguish an actually incompatible frozen dependency from a reviewed binding-compatible re-anchor. |
| **Relevant-when** | designing or changing plan/v2 execution-demand transport, harness-native implementation/review resolution, immutable routing evidence, the implement-now or impl-review consuming seams, their supersession graph, or direct-cutover mechanics; checking the final amended-basis rulings after both earlier routing design-brainstorm records gated needs-more-product |
| **Basis** | prd:planner-driven-model-routing (merged ledger as of amendment 003; fingerprint `sha256:1f6fa412f99da42f4b9aa0ca277dd912c011bd4acbe9b8c9881706f03f74f6f4`) |
| **Authority** | @mdarabi |
| **Date(s)** | 2026-07-27 — 2026-07-29 |
| **Status** | gated: needs-more-product |
| **Tracking** | — *(the introducing PR, once filed)* |
| **Feeds** | — *(the five draft designs are parked; resume after the contract-gate amendment merges)* |
| **Prior records** | brainstorm:planner-driven-model-routing · design-brainstorm:planner-driven-model-routing-tech-design · design-brainstorm:planner-driven-model-routing-tech-design-2 · design-brainstorm:implement-now-tech-design · design-brainstorm:impl-review-tech-design |
| **Template version** | design-brainstorm-record/v1 |

## Orientation

| Source | Why loaded |
|---|---|
| docs/prds/planner-driven-model-routing/effective.md | merged basis as of amendment 003: universal model-backed review, the exact assurance×band process, role-specific native resolution, append-first immutable route/outcome/process evidence, and direct activation |
| docs/prds/plan-new/effective.md · docs/prds/implement-now/effective.md · docs/prds/impl-review/effective.md | the three formerly missing consuming contracts are now merged as of 004, 003, and 003 respectively; their fingerprints fix the design-handoff basis |
| docs/brainstorms/planner-driven-model-routing-tech-design.md | frozen predecessor: D-1..D-4 choose existing-product continuity, declarative demand derivation, role-qualified native catalogs, and sealed plan transport; DQ-5..DQ-8 were stopped by product gaps |
| docs/brainstorms/planner-driven-model-routing-tech-design-2.md | frozen first reopen: removes `no_inference`, records the universal review-floor ruling, and bounces the exact process mapping now resolved by both amendment-002 ledgers |
| docs/tech-design/index.md | live-design routing surface: no live design Serves the parent routing PRD; `routing-envelope-v1`, `plan-new-v1`, `implement-now`, and `impl-review-v1` are the current overlap/currency set |
| docs/tech-design/routing-envelope-v1.md | scalar optional-rung design directly replaced by mandatory plan/v2 semantic demand |
| docs/tech-design/implement-now.md | reusable Worker/ports/fresh-process design whose binding `TierMap(rung)` launch and pre-routing backend description are stale |
| docs/tech-design/implement-now-e2e-tasks.md | newly merged executor child design; its pre-claim e2e basis resolution and two admissions remain live and must compose unchanged with the routed executor successor |
| docs/prds/contract-gate/effective.md · docs/tech-design/contract-gate-v1.md | FR-2 makes every `stale-serves`/`stale-builds-on` flag fatal but defines no typed compatibility disposition; this is the product contract that must move before a broad hub supersession is designable |
| docs/tech-design/index.json · `docgate contract-gate` output | the feature-scoped failures first exposed three dependents, while the complete reverse-dependency closure of the three superseded hubs spans dozens of live designs across multiple layers |
| docs/tech-design/impl-review-v1.md | reusable finding grammar, fresh-sibling lenses, attribution, and proposal rendering; its single-process envelope, proposal-only authority, dormant second-instance verify, and advisory production assumptions are now false |
| docs/brainstorms/impl-review-tech-design.md | prior impl-review artifact/runtime rulings and alternatives; valid decisions must be carried forward rather than re-asked |
| docs/architecture-principles.md · docs/observability/incidents.md · docs/wiki/glossary.md | AP-1/3/5/6/10/11/13/15/17/20/21/24/28/29/30 and RW-4/5/6; I1–I5; established execution-demand, assurance-floor, harness-adapter, and impl-review vocabulary |

**Overlaps surfaced:**

- The two earlier routing design-brainstorm records are frozen provenance. Their product blockers are closed: parent amendment 001 removed `no_inference`; parent and impl-review amendment 002 establish the universal one-pass floor and critical blind two-pass process; coordinated amendment 003 makes canonical append/exact-read the sole post-resolution pre-launch network boundary; the plan-new, implement-now, and impl-review consuming amendments are all merged.
- No live design covers the amended parent contract. `routing-envelope-v1` must be superseded; `implement-now` must be superseded because rung-based native selection is false; `impl-review-v1` must now be superseded because amendments 002 and 003 contradict multiple binding decisions rather than merely making its dependency header stale.
- `implement-now-e2e-tasks` landed after this session opened. Its pre-claim e2e resolution and admissions remain semantically valid, but its frozen `Builds-on: implement-now` edge becomes mechanically fatal when the executor successor lands; the same shape recurs across many unrelated dependents.
- The initial contract-gate run showed three immediate blockers (`plan-new-v1`, `code-gate-mirror`, and `implement-now-e2e-tasks`) and led to the operator's preliminary eight-record ruling. The full reverse-dependency audit then proved that repairing only those three would green the current feature while leaving the broader graph mechanically stale.
- Whole-document successors for every compatible dependent would create a replacement cone of roughly 38 records. Folding separately revisable Carina, slice-coverage, e2e, and code-gate decisions into a few mega-successors would instead violate RW-5. The conventions already anticipate an “explicit review re-anchor,” but contract-gate has no data class or acceptance rule for one.
- Still-valid decisions are not open ground: the planner owns evidence-backed semantic judgment; plangate mechanically derives and freezes demand; the selected harness remains upstream; one shared resolver uses role-qualified adapter catalogs; merge lane remains the actual-proposal classifier; impl-review remains the band projector and fresh independent reviewer.
- The open work is the final design partition/supersession graph, exact legacy translation, one retained evidence owner and crash boundary, the review process realization over that substrate, and the dependency/activation sequence.

## Design questions

- **DQ-1**: Given the merged parent and consuming contracts, what independently revisable technical-design partition and Supersedes/Builds-on graph should own plan demand, native routing policy, immutable evidence, implementation consumption, and review orchestration—especially now that both `implement-now` and `impl-review-v1` contain binding statements made false by the amendments?
- **DQ-2**: Within the execution-demand contract, what exact immutable plan/v1 rung translation and domain-separated `demand_id` derivation preserve historical intent, distinguish true absence from an explicit unknown rung, and remain stable across implementation, retry, and review attempts without rewriting the frozen source?
- **DQ-3**: Within the routing-evidence seam, must a canonical resolution/failure event become durably observable in git before claim or reviewer launch, and what append/idempotency/crash protocol preserves one evidence chain without a second ticket store or append-hot file?
- **DQ-4**: Within the impl-review successor, should a multi-pass process launch its independently admitted top-level passes concurrently behind a completion barrier or sequence/fail-fast them, and how do pass identity, internal remediation, head changes, and aggregation converge without leaking sibling output or accepting stale evidence?
- **DQ-5**: What exact dependency, pre-release proof, and one-switch activation sequence lets all five designs land safely, removes the hardcoded/rung paths without shadow or canary production behavior, and leaves only a same-harness strongest-profile fallback after cutover?
- **DQ-6**: When superseding the plan, executor, and reviewer hub designs makes a broad set of still-compatible frozen dependents mechanically stale, should the system mint successors for the entire reverse-dependency cone, collapse independent seams into a bounded successor set, or add a typed binding-level compatibility disposition that contract-gate can verify?

## Panel log

- **DQ-1**: opens/replaces — fired — the partition creates or replaces five seams and must be compared under minimal migration, independent-revisability, and evidence/cutover-integrity constraints before one graph is recommended.
- **DQ-2**: does not open/replace — not fired — D-1 already locates legacy translation inside the execution-demand contract; this question fixes that contract's compatibility data and identity rule without creating another owner, port, or side-effect boundary.
- **DQ-3**: does not open/replace — not fired — D-1 already establishes S-3 as the sole retained-evidence port and S-5/S-4 as its consumers; this question fixes S-3's admission and recovery semantics rather than adding another store or caller boundary.
- **DQ-4**: does not open/replace — not fired — the merged impl-review contract and D-1 already establish S-4, the aggregate result, independent top-level pass set, and internal remediation lineage; this question fixes orchestration order and identity/recovery rules inside that seam.
- **DQ-5**: does not open/replace — not fired — D-1 already assigns one union activation predicate to S-5 and the merged PRDs fix direct production activation; this question orders the five selected designs and fixes the predicate/cutover mechanics without adding a rollout service or cohort seam.
- **DQ-6**: opens/replaces — fired — the answer either creates a new contract-gate input seam, expands the supersession boundary across the complete dependent cone, or collapses existing document boundaries; three orthogonal panels tested minimal-record, independent-revisability, and zero-stale-integrity constraints.

## Decisions

### D-1 — Five decision records, with three exact successors

**Question:** DQ-1 — What independently revisable design partition and supersession graph owns the amended routing system?

**Answer:** Adopt ALT-1. Author five coordinated decision records, not five new product surfaces:

1. `execution-demand-contract-v1` supersedes `routing-envelope-v1`, builds on `plan-new-v1` and `local-work-tracker-v2`, and owns plan/v2 assessment/profile derivation, validation, sealed transport, and no-rewrite plan/v1 translation.
2. `native-routing-policy-v1` builds on the execution-demand contract and the repository configuration-policy boundary; it owns role-qualified harness catalogs, qualification, dominance, least-cost resolution, exact/promotion/unsupported results, and strongest satisfying same-harness fallback.
3. `routing-evidence-v1` builds on the first two and owns the one canonical append-only git event family, identifiers, joins, invalidation, and projections; it owns neither route choice nor review policy.
4. An impl-review successor supersedes `impl-review-v1`, builds on the three shared contracts plus its still-live dependencies, carries forward its valid finding/lens/remediation behavior, and owns assurance×band process computation, mutually blind top-level pass orchestration, conservative aggregation, and a typed aggregate result. It does not depend on implement-now.
5. An implement-now successor supersedes `implement-now`, builds on the four preceding records plus its still-live dependencies, carries forward the upstream harness-selection cascade and Worker lifecycle, consumes verified demand for `role=implementation`, invokes the impl-review aggregate boundary, and owns the one union activation predicate for the complete implementation-and-review route.

`plan-new-v1`, `implement-now-e2e-tasks`, `merge-lane-v1`, FeatureDrive, `local-work-tracker-v2`, context-pack, and codegate remain live. The executor successor builds on the e2e child so its basis and admission decisions stay binding. The successor records evolve the existing `implement-now` and `impl-review` products; they are not parallel “v2” runtimes. Native routing policy and retained evidence remain separate because they change for different reasons. Activation remains in the top-level implement-now composition rather than becoming a sixth one-time design.

**Reasoning:** This is the smallest partition that satisfies independent revisability, preserves one policy and one evidence authority, and retires every frozen binding statement made false by the merged amendments. ALT-2 couples policy evolution to evidence retention; ALT-3 creates four shallow one-consumer records and almost doubles landing coordination.

**Authority:** 2026-07-27 · “go for it”

**Status:** superseded by D-6 — the feature-scoped currency run proved five records leave same-feature consumers stale

### D-2 — Immutable monotone plan/v1 translation; plan/v2 remains multidimensional

**Question:** DQ-2 — How are frozen plan/v1 rungs translated, and what makes their origin stable across attempts?

**Answer:** Adopt ALT-4. The permanently immutable `plan-v1-rung/v1` compatibility table is:

| Frozen source | Translated semantic profile |
|---|---|
| explicit `rung=1` | `capability=economy`, `deliberation=light`, `context=standard`, `assurance=ordinary` |
| explicit `rung=2` | `capability=standard`, `deliberation=standard`, `context=large`, `assurance=ordinary` |
| explicit `rung=3` | `capability=frontier`, `deliberation=maximum`, `context=extended`, `assurance=critical` |
| truly absent routing proposal | the same maximum tuple as rung 3, classified `legacy_unassessed` |

Any other explicitly present value—including literal `rung=0`—is unknown and fails before claim or invocation; the raw parser must not collapse it into absence. Rung 2 retains ordinary assurance because its historical meaning was behavioral complexity, not elevated consequence. Rung 3 and absence translate conservatively because the former was the strongest/risk-raised route and the latter froze no routing judgment.

For either plan/v1 or plan/v2, `demand_id` is a domain-separated digest over the canonical demand-origin schema/version, repository + frozen-plan + task identity, tagged source-content digest, raw plan/v1 rung presence/value or the complete plan/v2 demand, applicable immutable policy/compatibility identity, and resulting four-part semantic profile. Identical source and policy bytes reproduce one `demand_id`; any source, policy, or translation change produces another. Implementation retries, review passes, and remediation attempts reuse the demand identity and mint distinct `attempt_id`s. Tracker text, proposal text, current policy aliases, and runtime re-assessment never enter the identity.

This compatibility path exists only for frozen plan/v1. A new plan/v2 task freezes both the six raw assessment axes and the four independently derived semantic minima; it never emits one combined complexity value or any native model/effort identifier.

**Reasoning:** A versioned mechanical table preserves known historical intent without pretending old plans carried the new dimensions. Domain-separated source binding proves immutability without another durable translation object. Full-maximum translation is reserved for genuine absence; runtime re-assessment would violate freeze and make retries disagree.

**Authority:** 2026-07-28 · “yes,” after confirming that plan/v2 carries both the six-axis assessment and the four-part derived profile rather than one combined value

**Status:** decided

### D-3 — Canonical git append is the launch-admission boundary

**Question:** DQ-3 — When must routing evidence become durable, and how does append/retry converge?

**Answer:** Adopt ALT-7. The canonical evidence adapter stores every origin, resolution/failure, routed outcome, assurance-pass terminal, review-process aggregate, and invalidation as one closed-schema immutable file on a dedicated fetched git evidence ref. Each logical event has a content-addressed, domain-separated identity and an identity-derived path. Writers compare-and-swap the observed ref head; on a concurrent non-conflicting append they exact-read the new head and retry to a bounded cap. The same identity plus byte-identical content is an idempotent success; the same identity plus different content is an integrity failure. No mutable JSONL, proposal payload, or routing-ticket store is authoritative.

For a routed attempt, the evidence sequence is:

1. exact-read the canonical demand origin and effective policy;
2. mint the attempt identity and resolve through the already-selected harness/role;
3. durably append exactly one `routing_resolved` or `routing_resolution_failed` terminal and exact-read it from the canonical ref;
4. only a durably observed resolved terminal permits task claim or reviewer launch; a failure terminal permits neither;
5. append exactly one `verified`, `failed`, or `blocked` outcome for every resolved attempt, including a post-resolution claim loss or a bounded crash/reap terminal; append no outcome for a resolution failure;
6. derive proposal/report faces from canonical events and reject any mismatch.

The resolved event is the permit, not proof that launch occurred. Recovery reuses its `attempt_id`, exact-reads the claim/runtime state, and resumes or reattaches only when absence or ownership is positively established; ambiguity remains non-accepting and never launches a duplicate. Git evidence unavailability therefore stops the affected attempt before claim/invocation. A new attempt appends new identities and never overwrites the old chain.

**Reasoning:** Committing after launch admits an unreconstructable side-effect window and then needs a second durable ticket to recover safely. One file per event avoids append-hot contention, CAS makes concurrency explicit, and exact-read admission turns the PRD's evidence promise into a mechanical precondition.

**Authority:** 2026-07-28 · “yes”

**Status:** decided

### D-4 — Concurrent blind pass set over one cacheable context pack

**Question:** DQ-4 — How are required review passes scheduled and isolated while reusing their common context efficiently?

**Answer:** Adopt ALT-10 with the operator's context-pack refinement. For each exact proposal revision, the orchestrator derives one `review_process_id` from the demand, task/proposal/revision, authoritative lane-verdict identity and projected band, process-policy version, and selected review harness. It derives the required `pass_id`s from that process identity plus stable pass indices, assigns a distinct runtime `attempt_id` and fresh top-level `context_id` to each pass, and constructs the complete set before launch.

The context-pack compiler builds one deterministic review base pack from the exact revision: proposal diff/tree, task charter and frozen demand, codegate verdict, served PRD/design binding elements, required ABUSE/incident context at deep depth, and calibration. Both passes receive the same content-addressed pack bytes and record the same pack digest. Pass-specific identity/instructions are a small private overlay appended after the shared bytes so provider prefix caching remains available. The pack contains no pass output, conversation state, mutable orchestration notes, timestamps, or sibling identity. Thus shared bytes reduce repeated context loading without creating shared judgment.

All required pass routes must satisfy D-3's durable admission before any pass launches. The orchestrator then launches the passes concurrently under one bounded owner, never cancels a sibling because another pass is non-accepting, and waits at a full-set barrier. Only after every required pass has a revision-bound terminal may it union surviving findings and compute the most restrictive verdict; only then may the worker see the union.

An internal scope-locked remediation re-review stays inside its originating pass lineage and never satisfies another top-level pass. Any code/head change makes the complete process non-accepting and mints a new exact-revision process with the full required set, fresh top-level contexts, and the newly built byte-deterministic base pack. Old events remain append-only evidence. Missing, failed, stale, mismatched, or conflicting passes prevent a satisfying aggregate.

**Reasoning:** Concurrent collect-all is order-invariant and preserves all required independent evidence. A deterministic shared prefix captures the context-pack cost benefit already proven by impl-review without sharing sessions or conclusions; sequential fail-fast would make critical coverage and findings depend on pass order.

**Authority:** 2026-07-28 · “yes, but use the same idea as context packing for two concurrent passes to ensure caching of shared context”

**Status:** decided

### D-5 — Complete pre-release proof, then one atomic production activation

**Question:** DQ-5 — How do the five designs land and activate without a shadow/canary or a retained hardcoded rollback path?

**Answer:** Adopt ALT-13. Author and review the five coordinated decision records as one union-audited design set so their headers, `Serves`, ports, and supersession edges freeze compatibly. Implement them from fresh main in dependency order: execution demand → native routing policy → routing evidence → impl-review successor → implement-now successor/integration. Incomplete combinations remain production-inactive; explicit conformance, failure-injection, and dogfood entry points exercise the real implementations without authorizing production acceptance.

One hermetic activation predicate exact-reads proof already landed on `origin/main`: all five implementation generations and compatibility identities; exhaustive harness × implementation/review role × derivable-profile conformance and current qualification; D-2 plan/v1 byte-identity/translation cases; D-3 append/admission/round-trip/crash cases; D-4 one-pass and critical concurrent two-pass independence, caching, staleness, and aggregation cases; at least three verified native-profile dogfood routes; every blocking DOD; actual-launch fidelity; strongest-profile fallback and unsatisfied-demand refusal; linked proposal/git evidence; and inspection that the hardcoded pin, scalar rung selector, implicit review settings, and zero-review terminal cannot be reached after activation. Cost, latency, token, and cache-hit measurements are recorded but never decide correctness readiness.

After the predicate is green, one fresh-main, human-merged activation change enables the validated semantic route for every supported production implementation and review path at once and makes the retired paths unreachable. There is no production shadow, canary, harness cohort, dual decision, or favorable-result comparison. The activation checkpoint is appended idempotently only after exact mainline verification; recovery derives whether activation landed from the goal object on main rather than a badge or journal claim.

Post-cutover fallback is only the strongest qualified native profile satisfying the complete request on the already-selected harness and role. It preserves review depth/count/blindness/aggregation. If none satisfies, the attempt fails before claim/invocation. A discovered qualification defect stops affected work, append-invalidates open evidence, reruns the complete exact-revision process under satisfying policy, and briefs already accepted artifacts; it never restores the hardcoded route or destructively rewrites shipped state.

**Reasoning:** This is the operator's “test completely, then production” ruling expressed as one auditable predicate and switch. It prevents incomplete dependency states from becoming live while avoiding every production cohort/shadow mechanism the contract excludes. The strongest-profile fallback preserves semantics; the retired path would not.

**Authority:** 2026-07-28 · “yes”

**Status:** superseded by D-7 — the activation semantics stand, but the five-record dependency set cannot reach a current contract without the compatibility amendment

### D-6 — Preliminary eight-record repair of the feature-scoped failures

**Question:** DQ-6 — How should the partition respond to the three contract-gate failures first exposed for plan-new and implement-now?

**Answer:** Add three narrow successors to the five routing records: one for current plan-new demand production, one for the code-gate mirror's executor binding, and one for e2e task consumption. Preserve their existing behavior and re-anchor them to the routed executor graph rather than introducing new product surfaces.

**Reasoning:** The first feature-scoped contract-gate output named exactly those three stale designs, so eight appeared to be the smallest set that could keep plan-new and implement-now mechanically current without folding code-gate or e2e semantics into the routed executor.

**Authority:** 2026-07-29 · “yes”

**Status:** superseded by D-7 — the complete reverse-dependency closure proved the feature-scoped repair was not graph-complete

### D-7 — Amend contract-gate with an explicit binding-compatibility disposition before resuming design

**Question:** DQ-6 — What graph-wide mechanism preserves immutable design integrity without manufacturing successors for still-compatible dependents?

**Answer:** Adopt ALT-16's product direction and bounce this record to `/prd-amend contract-gate`. Contract-gate must accept a closed, versioned, independently reviewed compatibility disposition only when it maps the exact binding anchors a frozen dependent consumed to exact successor anchors and cites conformance evidence. An absent, incomplete, stale, or incompatible disposition remains a blocking currency defect; a real binding contradiction still requires a successor. After that amendment merges, resume this record, author the required plan-new successor, and re-evaluate which dependents need behavior-changing successors versus mechanical re-anchors.

This ruling changes no routing semantics: the six raw planning measurements, four derived minima, harness/role-native resolver, append-first evidence, concurrent blind review passes over one cacheable shared prefix, at-least-one review floor, and one direct production switch all stand. It changes only how the immutable design graph proves a carried dependency remains valid.

**Reasoning:** ALT-17 repairs only the current feature and leaves other contracts stale; ALT-18 is mechanically pure but spends roughly 38 successors re-approving unchanged decisions. A typed disposition realizes the explicit-review path already named by the design conventions, keeps contract-gate fail-closed, and reserves supersession for actual contradictions.

**Authority:** 2026-07-29 · “go fo rit”

**Status:** decided

## Alternatives

- **ALT-16 — typed binding-level compatibility disposition** *(from DQ-6 · panel)*
  - **Shape:** add one closed `design-dependency-compatibility/v1` datum consumed by contract-gate. It names the dependent, superseded target, current successor, every consumed predecessor Decision/DINV/A/port anchor, each replacement anchor, conformance evidence, reviewer identity, and exact graph/content fingerprints. Only a complete current `compatible` disposition clears that edge's currency defect; contradictions still require a successor.
  - **Trade-offs:** avoids an O(N) successor wave and preserves independently revisable seams while keeping unknown mappings fail-closed. The new datum is a governance surface whose false-positive risk must be controlled by closed parsing, exact fingerprints, human review, and conformance evidence.
  - **Principles:** honors AP-6, AP-7, AP-10, AP-11, AP-13, AP-17, AP-20, AP-23, AP-24, AP-26, AP-27, AP-29, and RW-5; it operationalizes the conventions' explicit-review re-anchor rather than weakening immutability.
  - **Incidents:** I1 requires the amendment and its design before implementation; I2 requires exact mainline graph proof; I3 requires closed parsing; I4 requires exact per-edge and union coverage; I5 requires the same hermetic resolver locally and in CI.
  - **Verdict:** selected by D-7 as a product-amendment direction; the PRD must define its acceptance semantics before technical design chooses its concrete home and codec.
- **ALT-17 — bounded eight-record same-feature repair** *(from DQ-6 · panel)*
  - **Shape:** retain the five routing drafts and add plan-new, code-gate-mirror, and e2e-consumption successors; either leave unrelated stale edges for later or fold same-feature child decisions into the new hub successors.
  - **Trade-offs:** a small current PR and no new governance datum, but it greens only the feature-scoped slice. Folding Carina/slice/e2e/code-gate decisions violates their proven independent revisability; leaving them live leaves other contracts mechanically stale.
  - **Principles:** superficially honors AP-20 through a small change set but strains AP-11, AP-13, AP-20, AP-23, AP-27, and RW-5 once the complete graph is considered.
  - **Incidents:** repeats I2's intermediate-object trap by treating a clean current-feature verdict as proof of mainline graph health; I4 exposes the omitted dependent coverage.
  - **Verdict:** preliminarily selected by D-6, then rejected by D-7 after the full graph audit.
- **ALT-18 — complete reverse-dependency successor cone** *(from DQ-6 · panel)*
  - **Shape:** author one carry-forward successor for every live design in the reverse-dependency closure of `plan-new-v1`, `implement-now`, and `impl-review-v1`, re-pointing every edge topologically and adding a final activation composition; the audited graph is roughly 38 records over four layers.
  - **Trade-offs:** strongest literal zero-stale graph and no new compatibility mechanism, but re-approves mostly unchanged decisions, creates a large landing/review train, and couples unrelated product work to routing solely for header currency.
  - **Principles:** strongly honors AP-6, AP-11, AP-13, AP-17, AP-26, and literal whole-document immutability; materially strains AP-20, AP-23, AP-27, and the intent of RW-5 through graph-hygiene churn.
  - **Incidents:** I1/I4 multiply review and coverage obligations across every successor; I2 makes the long train itself a delivery risk; I5 keeps one hermetic gate but at disproportionate document cost.
  - **Verdict:** reject unless the operator later requires zero stale flags without any compatibility-disposition class.

- **ALT-1 — five independently revisable decisions, three forced supersessions** *(from DQ-1 · panel)*
  - **Shape:** `execution-demand-contract-v1` supersedes `routing-envelope-v1`; separate `native-routing-policy-v1` and `routing-evidence-v1` build on it; an `impl-review-v1` successor builds on all three; an `implement-now` successor builds on those four and owns the one union activation predicate. `plan-new-v1`, `merge-lane-v1`, FeatureDrive, tracker, context-pack, and codegate remain live.
  - **Trade-offs:** minimum legal supersession blast and no duplicated policy/store, while model-policy churn, evidence-schema churn, reviewer-process churn, and executor churn stay isolated. Five coordinated designs and four typed contracts still require an ordered union coverage audit; placing activation in the top-level implement-now composition couples a one-time release concern to the runtime owner.
  - **Principles:** honors AP-1, AP-3, AP-5, AP-6, AP-10, AP-11, AP-13, AP-15, AP-17, AP-20, AP-21, AP-22, AP-24, AP-28, AP-29, AP-30, RW-4, and RW-5; records the operator-authorized direct-production deviation from AP-9/RW-6 while retaining exhaustive pre-release proof, fail-closed activation, and fallback rehearsal.
  - **Incidents:** I1 satisfied by the now-merged four-ledger basis; I2 requires bottom-up landing, branch deletion, and mainline ancestry proof; I3 requires closed parsing before claim/invocation; I4 requires per-design plus five-design-union `Serves`; I5 requires the same hermetic derivation, resolver, join, evidence, and activation predicates locally and in CI.
  - **Verdict:** selected by D-1 — five is the smallest partition that keeps routing policy separate from durable evidence and gives each contradicted frozen design exactly one successor.
- **ALT-2 — append-first routed-attempt spine plus terminal activation composition** *(from DQ-1 · panel)*
  - **Shape:** keep the execution-demand contract and both consumer successors, combine native resolution plus canonical event admission in `routed-attempt-protocol-v1`, and add `planner-routing-activation-v1` as the fifth design over the complete graph.
  - **Trade-offs:** strongest crash boundary—durable `routing_resolved` admission is the launch permit—and a narrow owner for direct-cutover proof. It couples resolver-policy evolution to event identity/retention/invalidation and adds a terminal composition whose predicate can instead live in the already top-level executor composition.
  - **Principles:** strongly honors AP-1, AP-3, AP-5, AP-6, AP-10, AP-11, AP-13, AP-15, AP-17, AP-20, AP-21, AP-22, AP-23, AP-24, AP-26, AP-27, AP-28, AP-29, AP-30, RW-4, and RW-5; same explicit AP-9/RW-6 rollout deviation and compensating proof as ALT-1.
  - **Incidents:** I1–I5 checked; append-first admission best addresses I3 and crash convergence, and the terminal composition makes I2/I4 union proof explicit.
  - **Verdict:** viable integrity-maximizing alternative, but reject unless launch admission cannot be expressed as a typed evidence-port precondition: policy and evidence otherwise change for different reasons.
- **ALT-3 — nine seam-pure decision records** *(from DQ-1 · panel)*
  - **Shape:** split demand policy, plan/v2 carrier, legacy translation, native routing, routing evidence, review-assurance policy, implement-now, impl-review, and activation into separate documents; implement-now defines a consumer-side `ReviewGate` and impl-review implements it to avoid a dependency cycle.
  - **Trade-offs:** maximum independent revisability and five singular mechanical authorities, but nine reviews and five dependency waves. The legacy translator and pure review join each have one present consumer and would be shallow standalone decision records; their splits buy hypothetical isolation at substantial coordination cost.
  - **Principles:** strongest literal RW-5/AP-20 partition; honors AP-1, AP-3, AP-5, AP-6, AP-10, AP-11, AP-13, AP-15, AP-17, AP-20, AP-21, AP-24, AP-28, AP-29, AP-30, RW-4, and RW-5; same explicit AP-9/RW-6 deviation.
  - **Incidents:** I1–I5 checked; exhaustive I4 coverage and I2 mainline landing proof become materially more expensive across nine records.
  - **Verdict:** reject as over-partitioned for the known system; preserve its pure-authority and consumer-owned-port tests inside ALT-1.
- **ALT-4 — immutable monotone plan/v1 compatibility table** *(from DQ-2 · session)*
  - **Shape:** one permanently versioned table maps explicit historical rungs `1→economy/light/standard/ordinary`, `2→standard/standard/large/ordinary`, and `3→frontier/maximum/extended/critical`; true absence becomes `legacy_unassessed→frontier/maximum/extended/critical`. A domain-separated digest binds source bytes, repository/plan/task identity, raw rung presence/value, compatibility version, and translated profile as `demand_id`.
  - **Trade-offs:** preserves the historical distinction that rung 2 represented behavioral complexity rather than elevated consequence, while translating old strongest/risk-raised work and unassessed work conservatively. Some legacy work will over-route, but no mutable reassessment or source rewrite is introduced.
  - **Principles:** AP-5/AP-6 closed identity and schema, AP-10 mechanical table, AP-13 source-bound proof, AP-15 retry-stable origin, AP-21 no downstream re-judgment, AP-28 one durable origin.
  - **Incidents:** I3 requires raw-presence parsing and fail-closed unknown rungs; I5 requires the same table/digest goldens locally and in CI.
  - **Verdict:** selected by D-2 — it is deterministic, conservative where history is ambiguous, and keeps one stable demand across all attempts.
- **ALT-5 — translate every plan/v1 task to the maximum profile** *(from DQ-2 · session)*
  - **Shape:** ignore the semantic distinction among known explicit rungs and translate all plan/v1 tasks to `frontier/maximum/extended/critical`; retain raw rung only as evidence.
  - **Trade-offs:** safest against under-routing and simplest table, but discards frozen planner judgment, makes the compatibility-table requirement largely ceremonial, and forces two deep blind review passes for every legacy task.
  - **Principles:** AP-13 source evidence is retained, but AP-21 and least-powerful-that-suffices are strained by deliberate overprovision.
  - **Incidents:** I5 cost remains non-blocking, but this creates systematic cost/latency rather than measuring it.
  - **Verdict:** reject — absence warrants maximum conservatism; known historical rungs do not.
- **ALT-6 — re-assess the legacy charter into plan/v2 dimensions at read time** *(from DQ-2 · session)*
  - **Shape:** run a planner-like assessment over the frozen plan/v1 charter and derive a new semantic profile on every read, using the old rung only as a floor.
  - **Trade-offs:** may route more precisely but creates mutable post-freeze inference, unstable demand identity, a second planner in the executor, and retry/review disagreement.
  - **Principles:** violates AP-10, AP-13, AP-15, AP-21, and the no-rewrite/no-rejudgment contract.
  - **Incidents:** conflicts directly with I3 fail-closed consequential parsing and I5 one hermetic predicate.
  - **Verdict:** reject — forbidden by the merged contract, not merely less attractive.
- **ALT-7 — append-first canonical git evidence as launch admission** *(from DQ-3 · session)*
  - **Shape:** each canonical origin, resolution/failure, outcome, review-process, and invalidation event is one closed immutable file committed to a dedicated fetched evidence ref. Writers compare-and-swap the ref head and retry concurrent non-conflicting appends; an exact-read durably observed `routing_resolved` event is the launch permit, while a durable failure terminal authorizes no side effect.
  - **Trade-offs:** a fresh clone can reconstruct every accepted chain and a crash cannot leave an authorized launch with no retained route. Git/evidence availability becomes an execution prerequisite and concurrent writers need bounded CAS retry, but one-file events avoid an append-hot ledger and no second ticket object exists.
  - **Principles:** AP-5/AP-6 closed append schema, AP-11 one admission predicate, AP-13 evidence before trust, AP-15 retry convergence, AP-17 fail before side effect, AP-22 bounded ownership, AP-28 canonical retained evidence.
  - **Incidents:** I2 exact-reads the canonical ref rather than trusting a status; I3 rejects identity/schema conflicts; I5 uses one append/reduce implementation across desk, CI, and production.
  - **Verdict:** selected by D-3 — strongest integrity shape and the only one that makes “no durable route, no launch” mechanically true.
- **ALT-8 — launch after local buffering; commit evidence at terminal outcome** *(from DQ-3 · session)*
  - **Shape:** buffer origin/resolution locally, launch immediately, and commit the full chain after the worker reaches an outcome; unsupported attempts commit their failure directly.
  - **Trade-offs:** lower launch latency and less remote-ref contention, but a host crash can spend inference or mutate a workspace with no reconstructable route. Recovery cannot distinguish never-launched from launched-with-lost-buffer without another durable ticket.
  - **Principles:** strains AP-13, AP-15, AP-17, and AP-28 because evidence follows the consequential act.
  - **Incidents:** fails the I2 exact-object lesson and makes I5 fresh-clone parity impossible during the crash window.
  - **Verdict:** reject — availability is improved by accepting the exact orphan-evidence state the contract forbids.
- **ALT-9 — one mutable attempt ledger or proposal payload as authority** *(from DQ-3 · session)*
  - **Shape:** update one JSONL/ledger file per repository or treat the proposal's impl-review payload as the durable attempt record, with git events derived later.
  - **Trade-offs:** fewer files and a familiar human surface, but concurrent appends contend on one object, retries can rewrite history, unsupported pre-proposal attempts have no home, and the proposal/git faces can disagree.
  - **Principles:** violates AP-15/AP-20/AP-28 and the selected S-3 one-authority boundary.
  - **Incidents:** I3 makes mutable marker parsing consequential; I5 would require competing reducers rather than one.
  - **Verdict:** reject — both shapes create a second or mutable authority explicitly retired by the amended PRDs.
- **ALT-10 — concurrent blind pass set with a completion barrier** *(from DQ-4 · session)*
  - **Shape:** derive one exact-revision process and all required pass IDs up front; build one deterministic content-addressed base context pack whose byte-identical shared prefix feeds every pass for provider-cache reuse; append private pass overlays; admit every route durably; launch one fresh top-level context per pass concurrently; aggregate once behind a full-set barrier before exposing the finding union.
  - **Trade-offs:** lowest critical-review wall time, strongest structural blindness, and complete finding evidence even when one pass is non-accepting. It may spend the second pass after the first has already found a blocker and requires bounded fan-out/reap logic.
  - **Principles:** AP-5 exact identity, AP-13 exact-revision trust, AP-15 full-set retry convergence, AP-17 admission before launch, AP-22 owned bounded concurrency, AP-28 append-only process evidence.
  - **Incidents:** I3 rejects shared/unknown context identities; I5 requires order-invariant aggregation and the same barrier reducer in local/CI paths.
  - **Verdict:** selected by D-4 with shared-pack caching — required passes are independent evidence, so collecting the complete set is more honest than treating one pass as a gate on whether another exists.
- **ALT-11 — sequential passes with fail-fast cancellation** *(from DQ-4 · session)*
  - **Shape:** run pass 1, stop when it is non-accepting, and launch pass 2 only after pass 1 accepts; aggregate the available prefix.
  - **Trade-offs:** saves spend on already-red changes and simplifies concurrency, but pass order affects which findings survive, critical work can repeatedly miss the second independent lens, and an available prefix cannot satisfy the complete-set contract.
  - **Principles:** strains AP-11/AP-13/AP-28 because required evidence becomes order-dependent and routinely partial.
  - **Incidents:** violates I5's hermetic order-invariant predicate and weakens I4 coverage of the critical two-pass contract.
  - **Verdict:** reject — a failed first pass is a reason not to accept, not permission to omit required corroboration evidence.
- **ALT-12 — one top-level reviewer with two internal rounds** *(from DQ-4 · session)*
  - **Shape:** reuse one reviewer context and count its scope-locked remediation round as critical pass two.
  - **Trade-offs:** cheapest execution and easiest state machine, but no independent context or corroboration exists.
  - **Principles:** violates AP-13 and the explicit amended product contract.
  - **Incidents:** directly recreates the pass/remediation identity confusion guarded by I3-style closed typing and the merged EVAL-10 corpus.
  - **Verdict:** reject — forbidden by FR-24/FR-26 and `prd:impl-review/FR-20` · `prd:impl-review/FR-24`.
- **ALT-13 — inactive dependency wave, exhaustive proof, then one atomic production switch** *(from DQ-5 · session)*
  - **Shape:** land demand, policy, evidence, impl-review, and implement-now integration in dependency order with production activation false; exercise them only through explicit pre-release conformance/dogfood entry points; after one union predicate exact-reads all proof from main, one fresh-main activation change enables every supported implementation/review route and removes reachability of hardcoded/rung paths.
  - **Trade-offs:** keeps incomplete combinations out of production and makes the operator's direct cutover one auditable act. It requires a temporary inactive code path and a broad final gate, but creates no production cohort, shadow decision, or parallel outcome authority.
  - **Principles:** AP-9/RW-6 mechanism deviated by explicit operator ruling; their evidence-before-replacement intent remains. AP-11 one predicate, AP-13 exact proof, AP-17 fail closed, AP-26 mainline ancestry, AP-29 hermetic release evaluation.
  - **Incidents:** I2 requires each landed prerequisite and activation tip to be proven on `origin/main`; I4 requires per-design and union `Serves`; I5 keeps cost/latency measurements outside the correctness predicate.
  - **Verdict:** selected by D-5 — it implements “test fully, then production” exactly without using production as the test environment.
- **ALT-14 — activate each harness or role as its implementation lands** *(from DQ-5 · session)*
  - **Shape:** enable Codex implementation first, then other harnesses and review paths incrementally as their mapping cells become ready.
  - **Trade-offs:** smaller individual cutovers and earlier feedback, but this is a production cohort/canary rollout, creates mixed routing semantics, and contradicts the one-switch contract.
  - **Principles:** violates the operator-authorized rollout and strains AP-11 with multiple live predicates.
  - **Incidents:** I2/I5 proof becomes dependent on which partial production state is observed.
  - **Verdict:** reject — explicitly outside the merged PRD.
- **ALT-15 — retain the hardcoded route as rollback after activation** *(from DQ-5 · session)*
  - **Shape:** switch to semantic routing but keep the old Sonnet/rung path selectable if the new mapping fails.
  - **Trade-offs:** familiar operational rollback, but it can silently underprovide, bypass multidimensional demand and review floors, and preserve two production truths indefinitely.
  - **Principles:** violates AP-11/AP-17/AP-21 and the selected one-authority graph.
  - **Incidents:** recreates I5 local/CI/production predicate divergence.
  - **Verdict:** reject — fallback is the strongest qualified profile on the already-selected harness or a fail-closed stop, never the retired route.

## Seam map

- **S-1 — execution-demand contract:** planner judgment enters as cited closed assessment; one mechanical derivation/validator freezes the semantic profile, and tracker adapters preserve the canonical capsule. Owns plan/v1 translation but no native selection.
- **S-2 — native routing policy:** after the caller fixes harness and role, one pure resolver reads that harness-role catalog and returns `exact`, `promoted`, or `unsupported`. Harness adapters own native catalogs; the resolver owns no harness selection or durable event state.
- **S-3 — routing evidence:** consumers append and exact-read one canonical immutable event family through a typed port. The evidence owner validates identities/joins and derives projections; it never recomputes demand, selects a route, or decides a review verdict.
- **S-4 — impl-review successor:** consumes planned assurance, unchanged impl-review band projection, native review resolution, and evidence ports; owns top-level pass/remediation distinction, independent-pass lifecycle, and one aggregate terminal. It exposes the aggregate contract without importing the executor.
- **S-5 — implement-now successor:** the already-selected harness and verified demand enter before claim; the still-live e2e child may refuse or classify an e2e task first, then the Worker launches only an admitted implementation route and calls the review aggregate boundary before proposal acceptance. This top-level composition owns the single direct-production activation predicate over S-1..S-4.

## Facts

- **F-1**: the admitted parent basis is merged as of amendment 003 with fingerprint `sha256:1f6fa412f99da42f4b9aa0ca277dd912c011bd4acbe9b8c9881706f03f74f6f4`; its product-level review mapping, append-first evidence boundary, and rollout questions are closed — source: docs/prds/planner-driven-model-routing/effective.md
- **F-2**: the formerly blocking sibling contracts are merged: plan-new as of 004, implement-now as of 003, and impl-review as of 003 — source: their generated effective-view headers on origin/main after PRs #778–#780
- **F-3**: predecessor D-1..D-4 selected existing-product continuity, declarative compiler-derived demand, role-qualified native catalogs with one resolver, and sealed self-contained tracker transport verified against the frozen plan; those choices remain consistent with the amended basis — source: design-brainstorm:planner-driven-model-routing-tech-design#D-1..D-4
- **F-4**: the parent and impl-review contracts require the same closed process: ordinary yields at least `standard×1`, elevated `deep×1`, critical `deep×2`; critical passes are mutually blind top-level contexts on one exact revision and aggregate most-restrictively — source: prd:planner-driven-model-routing/FR-25, FR-26; prd:impl-review/FR-24, FR-26
- **F-5**: `impl-review-v1` binds one payload whose rounds are one reviewer lineage, proposal-resident payload authority, a dormant optional deep second instance, and advisory production; the amended basis instead requires top-level pass identity, immutable git event authority plus proposal projection, mandatory critical two-pass independence, and direct activation — source: docs/tech-design/impl-review-v1.md; prd:impl-review/FR-19, FR-23..FR-30
- **F-6**: the canonical parent evidence chain is one demand origin, one resolution or failure per attempt, exactly one routed outcome only for resolved routes, one exact-revision aggregate process terminal, and append-only invalidation; prompt/code payload is excluded — source: prd:planner-driven-model-routing/FR-20, FR-21, FR-24, EVT-1..EVT-3, EVT-5, EVT-8..EVT-10
- **F-7**: I1 requires merged requirements before design; I2/AP-26 requires mainline ancestry rather than merge badges; I3 requires closed consequential parsing; I4 requires exhaustive per-design and union Serves; I5/AP-29 requires one hermetic gate predicate and keeps measurements non-blocking — source: docs/observability/incidents.md
- **F-8**: contract-gate FR-2 makes any live serving design's `stale-serves` or `stale-builds-on` flag a blocking currency defect, while tech-design-conventions §3.2 says those flags remain non-fatal for other readers and may be cleared by a successor or an explicit review; no machine-readable explicit-review disposition exists — source: prd:contract-gate/FR-2 · docs/tech-design/tech-design-conventions.md §3.2 · docs/tech-design/contract-gate-v1.md §3.9
- **F-9**: the live reverse-dependency closure of `plan-new-v1`, `implement-now`, and `impl-review-v1` spans multiple product families and dozens of designs; the three feature-scoped failures were not the complete graph — source: docs/tech-design/index.json on origin/main plus the five routing drafts, audited 2026-07-29
- **F-10**: `local-work-tracker-v2` review already adjudicated one `stale-builds-on` edge tolerable because no binding statement became false, proving compatibility and supersession are distinct judgments even though contract-gate cannot currently encode that distinction — source: docs/reviews/local-work-tracker-v2.md Round 2

## Fog

- **G-1**: the exact compatibility-disposition schema, authority, invalidation rule, and contract-gate effect are blocking design inputs owned by the contract-gate PRD amendment — blocks: DQ-6's final design partition and every parked technical design

## Out of scope

- changing upstream harness selection, credentials, egress, or availability policy
- routing non-plan authoring stages or introducing task-level `no_inference`
- automatic model discovery/qualification, self-tuning, cross-harness optimization/fallback, or cost/quality learning
- changing merge-lane risk classification, codegate's deterministic ladder, or human approval authority
- a second routing ticket/store, proposal-authoritative evidence, mutable event ledger, or committed context-pack cache
- production shadowing, canaries, per-harness cohorts, testing-in-production, or restoring the hardcoded/rung route after activation

## Candidate glossary terms

none — existing vocabulary is sufficient

## Parked product gaps

- **P-1**: contract-gate has no typed, verifiable way to distinguish a frozen dependent whose consumed binding anchors are carried unchanged by a successor from one whose assumption actually became false; `prd:contract-gate/FR-2` therefore forces a broad successor cascade. **Disposition:** bounced — see Gate; amend `prd:contract-gate/FR-2` and its failure/DoD/eval surfaces before design resumes.

## Gate

`needs-more-product` — on 2026-07-29 the operator approved the full-graph audit's recommendation to amend contract-gate before resuming design: “go fo rit”. Amend route: `/prd-amend contract-gate`; blocking gap: P-1 (`prd:contract-gate/FR-2`). The prior ready-for-design gate is superseded by this pre-merge review-loop ruling.
