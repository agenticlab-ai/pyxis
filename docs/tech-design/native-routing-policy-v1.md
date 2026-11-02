# Harness-native routing policy — technical design

| Field | Value |
|---|---|
| **Status** | Draft → Approved *(merge requires Approved; merge is the freezing event)* |
| **Owner** | @mdarabi |
| **Approver(s)** | @mdarabi (operator and security reviewer) |
| **Date** | 2026-07-28 |
| **Relevant-when** | designing or changing how a selected agent harness maps vendor-neutral implementation or review demand to native model and effort; qualifying a native profile; adding a harness/role mapping or repository override; triaging an underpowered, promoted, unsupported, or fallback route; superseding native routing policy |
| **Scope** | Role-qualified harness-adapter catalogs, native-profile qualification, componentwise dominance, deterministic least-cost selection, exact/promotion/unsupported results, full-catalog overrides, and strongest satisfying same-harness fallback. **Not:** harness selection, demand derivation/transport, route-event retention, review pass count, or production activation. |
| **Tracking** | — *(the introducing PR)* |
| **Builds-on** | docs/tech-design/execution-demand-contract-v1.md · docs/tech-design/config-driven-portability-v1.md |
| **Supersedes** | — |
| **Serves** | prd:planner-driven-model-routing/FR-8 · prd:planner-driven-model-routing/FR-9 · prd:planner-driven-model-routing/FR-10 · prd:planner-driven-model-routing/FR-11 · prd:planner-driven-model-routing/FR-12 · prd:planner-driven-model-routing/FR-14 · prd:planner-driven-model-routing/FR-27 · prd:planner-driven-model-routing/NR-1 · prd:planner-driven-model-routing/NR-7 · prd:planner-driven-model-routing/UB-4 · prd:planner-driven-model-routing/UB-6 · prd:planner-driven-model-routing/ABUSE-1 · prd:planner-driven-model-routing/ABUSE-4 · prd:planner-driven-model-routing/NFR-1 · prd:planner-driven-model-routing/NFR-3 · prd:planner-driven-model-routing/NFR-5 · prd:planner-driven-model-routing/NFR-7 · prd:planner-driven-model-routing/NFR-8 · prd:planner-driven-model-routing/EVAL-2 · prd:planner-driven-model-routing/EVT-4 · prd:implement-now/FR-23 · prd:implement-now/FR-24 · prd:implement-now/FR-26 · prd:implement-now/NR-6 · prd:implement-now/UB-18 · prd:implement-now/ABUSE-5 · prd:implement-now/NFR-7 · prd:implement-now/NFR-9 · prd:implement-now/EVAL-2 · prd:impl-review/FR-25 · prd:impl-review/FR-28 · prd:impl-review/NR-7 · prd:impl-review/UB-15 · prd:impl-review/NFR-12 · prd:impl-review/EVAL-11 |
| **Security/privacy impact** | yes — this policy selects model/provider capacity, effort, spend, and egress behavior for autonomous implementation and review. Closed operator-authorized catalogs expose no credentials and admit no raw launch arguments. reviewer: @mdarabi. |
| **Template version** | tech-design-template/v1 |

## 0. Read to rule — the human reviewer's section (~5 min)

*The body is the contract; this section only routes attention — on any conflict the body governs; machines ignore it.*

**Judge these**

| # | The call | Could have gone | If you disagree |
|---|---|---|---|
| 1 | The caller fixes harness and role before one shared resolver sees demand | global model routing or harness fallback | you want capacity policy to choose credentials/egress substrate → §3.1, #DINV-1 |
| 2 | Native profiles prove four-component dominance and the resolver chooses the unique lowest cost rank | scalar tiers or adapter-specific algorithms | you accept hidden downgrades or divergent selection → §3.3, #DINV-4 |
| 3 | Repository overrides replace one complete compatible harness-role catalog | field-level merge | you accept ambiguous partial policy provenance → §3.5, #DINV-7 |
| 4 | Fallback is only the strongest qualified satisfying profile on the same harness and role | return to the hardcoded route | you accept semantic downgrade or a substrate switch → §3.6, #DINV-8 |

**Sanity-check the numbers** — four semantic minima must all dominate (#DINV-4); implementation and review are distinct roles (#DINV-2); resolution performs zero inference and zero network calls (#DINV-5).

**The author's open questions** — none — exact native profile inventories and prices are reviewed policy data, not frozen design prose.

**Skip the rest** — semantic demand → `execution-demand-contract-v1`; route/outcome retention → `routing-evidence-v1`; review process → `impl-review-routed-assurance`; activation → `implement-now-routed-execution`; current catalog cells → live policy and conformance output.

## 1. Problem

The planner can say that a task needs frontier capability, maximum deliberation, extended context, and critical assurance, but those words do not name a valid Codex, Cursor, Claude Code, or OpenCode launch. Each harness exposes different models, effort controls, and context behavior, and implementation and review can need different mappings on the same harness. A global hardcoded model cannot preserve the planner's meaning across those substrates.

The native mapping must therefore be owned at the harness boundary while its selection semantics remain common and auditable. Otherwise every adapter becomes a second resolver and a missing cell silently becomes a cheap default (`design-brainstorm:planner-driven-model-routing-tech-design#D-3`; `design-brainstorm:planner-driven-model-routing-tech-design-4#D-1`).

## 2. Decision

**Decision:** After an upstream caller has selected a harness and declared `role=implementation|review`, one pure resolver reads that exact harness-role's closed, qualified native-profile catalog and returns the unique least-cost profile whose declared semantic provision componentwise dominates the requested execution profile. It returns `exact`, `promoted`, or `unsupported`; it never selects a harness, invokes a model, consults the network, infers from task prose, or reuses another role's result.

Harness adapters own the native catalog and typed translation from the selected native profile to their CLI/API launch contract. The shared resolver owns dominance, ordering, qualification, override validation, and result grammar. This makes “high complexity on Codex” and the same task on Cursor separate policy cells over one unchanged demand, exactly as intended.

## 3. The design

### 3.1 Inputs and ownership

The resolver accepts:

- the verified `execution-demand/v2` profile and demand ID;
- one already-selected harness identity from the invoking configuration;
- one closed role, `implementation` or `review`;
- an exact effective `native-routing-catalog/v1` identity and digest;
- a resolution mode, `normal` or `strongest_satisfying`.

The harness decision remains upstream and is never a candidate set. The role is explicit; review cannot inherit the implementation result. The resolver returns data only. The adapter later turns the chosen typed model/effort/context controls into a launch spec.

- **DINV-1** — Resolution never changes, scans for, or falls back to another harness.
- **DINV-2** — Implementation and review catalogs, qualification, resolution, and evidence are independently complete even when their native values happen to match.

### 3.2 Closed catalog and qualification

**Decision:** Each supported harness publishes separate role catalogs under `native-routing-catalog/v1`. A catalog entry contains:

- stable `native_profile_id` scoped by harness and role;
- typed native model and effort values plus only adapter-recognized typed launch controls;
- `provides`: capability, deliberation, context, and assurance levels in the shared semantic orders;
- an integer `cost_rank` unique within that harness-role catalog;
- qualification identity and digest, qualified implementation generation, validity state, and operator approval;
- an explicit strongest-profile marker derived from the greatest qualified provision and tie rules, never free text.

Each qualification identity resolves to one closed `native-profile-qualification/v1` artifact. It binds the harness, role, native profile/model/effort and typed controls, exact adapter implementation generation, catalog schema/evaluator versions, and one claim for each of capability, deliberation, context, and assurance. A claim names its asserted level, method `deterministic_conformance|decode_pinned_eval|operator_inspection`, fixture/corpus and result digests, threshold/result, evidence event or repository reference, evaluator version, and responsible evaluator. The envelope records the native launch capture digest, operator-authorization proof, `valid_from`, required `valid_until`, and any additive invalidation event. Resolution evaluates validity against the invocation's trusted clock and exact implementation generation; expiry or invalidation removes the profile from the candidate set.

Provider/model self-description, worker output, free-form assertions, and unbound URLs are never qualification. Operator inspection is accepted only with a named rubric result for each claimed component; it cannot stand in for a runnable conformance/eval method where the harness exposes one. Unknown keys, duplicate IDs/ranks, incomparable or ambiguous strongest profiles, missing or stale/component-insufficient qualification, unqualified entries, raw CLI arguments, and native values unknown to the adapter make the whole affected catalog invalid before resolution.

- **DINV-3** — Only operator-authorized, currently qualified profiles participate; contributor/worker-authored claims never become effective by presence alone.

### 3.3 Dominance and least-cost resolution

**Decision:** `provides` satisfies `requested` iff every component is greater than or equal to the requested component in that component's closed order. In normal mode the resolver filters to qualified satisfying candidates, sorts by `(cost_rank, native_profile_id)`, and requires the first rank to be unique. Equal component tuples are `exact`; any strictly greater component is `promoted` and the result names every promoted component. No candidate yields `unsupported` with every unmet component.

Catalog order, map order, and input serialization never affect the result. A duplicate cheapest rank is policy-invalid rather than resolved by incidental ordering.

- **DINV-4** — A resolved profile dominates all four requested minima; no surplus in one component compensates for a deficit in another.
- **DINV-5** — Identical demand, harness, role, effective catalog, qualification evidence, and mode yield byte-identical results with zero model inference and zero network calls.
- **DINV-6** — `unsupported` carries no native profile and authorizes no claim or model invocation.

### 3.4 Result contract and launch fidelity

The closed `native-route-resolution/v1` result contains attempt/demand IDs supplied by the caller, harness, role, requested profile, catalog/policy identity, resolution class, selected native profile and promoted components when resolved, or unmet components and a typed reason when failed. The adapter must build the launch specification only from the referenced catalog entry.

After launch, the adapter captures the actual native model, effort, and relevant controls reported or observed by the harness. Any difference from the selected entry makes the attempt failed and invalid for qualification, dogfood, or acceptance. The evidence design retains both route and outcome; this design defines their equality predicate.

### 3.5 Repository overrides

**Decision:** A repository override may replace exactly one complete harness-role catalog when its schema and resolver compatibility identities match the runtime, every entry passes the same qualification rules, and existing repository authorization approves it. There is no field merge between defaults and override, no inherited candidate, and no raw argument escape hatch.

Replacement provenance records default identity, override identity, authorization evidence, and effective digest. An invalid override invalidates the entire affected harness-role policy before any affected dispatch; it never falls back silently to defaults.

- **DINV-7** — One resolution reads one complete effective catalog with one provenance chain; no result is assembled from partially merged policy sources.

### 3.6 Strongest satisfying fallback

**Decision:** In `strongest_satisfying` mode the resolver considers only the already-selected harness-role catalog and selects its unique strongest qualified profile if, and only if, that profile satisfies every requested minimum. Otherwise it returns `unsupported`. The mode changes candidate preference, not demand, harness, role, required review depth/count, or aggregation.

- **DINV-8** — Fallback may overprovide native capacity but can never weaken semantic demand, reduce review process, switch harness, or restore a retired hardcoded route.

### 3.7 Boundaries and non-goals

- **Not solved here** — planning judgment, tracker carrier, event storage, pass orchestration, cost budgets, and activation timing.
- **Owned elsewhere** — demand by `execution-demand-contract-v1`; canonical evidence by `routing-evidence-v1`; review process by `impl-review-routed-assurance`; cutover by `implement-now-routed-execution`.
- **Implementation must not infer** — harness, role, capability, qualification, or fallback from installed tools, environment, charter, model name, or provider marketing.
- **Must remain unchanged** — upstream harness selection and authorization, general authoring-stage routing, merge-lane risk classification, and impl-review band projection.

### 3.8 Observability

- **Events/logs** — every call produces a canonical resolution/failure payload for the evidence port; policy validation logs exact harness, role, catalog identity, profile ID, and typed defect.
- **Metrics** — exact/promoted/unsupported/invalid counts, promotions by component, selected native profiles, qualification age, launch mismatches, and descriptive cost/latency by harness-role.
- **Alerts** — any launch mismatch, qualification invalidation, or repeated policy-invalid result pages the repository operator; unsupported demand is surfaced as a task/process result.
- **Debugging surface** — a dry resolver explains candidates, dominance comparisons, ordering, override provenance, and unmet components without launching.
- **Trace/correlation** — attempt ID and demand ID join resolution to origin and outcome; catalog and native-profile identities reproduce the choice.

### 3.9 Security & privacy

Catalogs influence billing, provider egress, and autonomous capability. Existing config authorization gates defaults and repository replacements; closed typed native controls and the raw-argument prohibition block launch injection. Qualification must originate outside worker-controlled output, bind the exact adapter generation, and be invalidated when that generation or declared native behavior changes (#DINV-3). Routing data contains provider/product identifiers and usage metadata but no credentials, prompts, or code. Adapters obtain credentials through their existing runtime boundary after resolution; policy never stores them.

### 3.10 Conformance and failure handling

The test generator enumerates the required domain from the live AgentRuntime harness registry, both roles, and every profile derivable by the demand policy. Each cell has a qualified satisfying result or explicit unsupported result; omission is a failure, not absence of support. Candidate-order property tests prove determinism. Each adapter runs launch-fidelity tests using its real typed launch builder and decode-pinned qualification fixtures.

| Failure | Behavior | Recovery |
|---|---|---|
| catalog/override invalid | fail affected policy before claim/invocation | operator lands corrected complete catalog |
| no satisfying candidate | durable unsupported result; no harness switch | qualify capacity or revise future planning policy, never current demand |
| actual launch differs | fail attempt and invalidate qualification | stop affected work, requalify adapter/profile, rerun exact attempt/review process |
| strongest profile insufficient | unsupported, preserving process | no semantic fallback exists |

## 4. Alternatives considered

| Alternative | Why rejected |
|---|---|
| Role-qualified adapter catalogs with one pure resolver | selected because policy can vary per harness while dominance and evidence remain uniform (`design-brainstorm:planner-driven-model-routing-tech-design#D-3` · `#ALT-11`) |
| Fold native policy into retained routing evidence (`design-brainstorm:planner-driven-model-routing-tech-design-4#ALT-2`) | route choice and evidence retention change for different reasons; coupling them forces policy roster changes through storage/recovery review |
| One global vendor-neutral-to-model table | model and effort capabilities differ by harness and role; the table either leaks substrate assumptions or lies |
| Let each adapter implement its own resolver | creates divergent dominance, tie, failure, and override behavior |
| Merge partial repository overrides into defaults | makes effective provenance and qualification ambiguous |
| Switch harness when unsupported | overrides credentials, availability, and egress policy fixed upstream |
| Retain the hardcoded model as fallback (#ALT-15) | can underprovide and preserves two routing truths (`design-brainstorm:planner-driven-model-routing-tech-design-3#ALT-15`) |

## 5. Risks

| ID | Risk | Impact | Mitigation | Verification / evidence |
|---|---|---|---|---|
| R1 | Declared semantics overstate actual native behavior | underpowered launch | generation-bound qualification plus launch capture | real-adapter conformance and invalidation drill |
| R2 | Cost ranks encode volatile prices as frozen prose | cheapest selection becomes stale | ranks live in reviewed catalogs; design freezes only selection rule | policy review and reordered-candidate tests |
| R3 | A role catalog is omitted | support silently shrinks | generated full-domain matrix treats omission as failure | exhaustive harness×role×profile report |
| R4 | Strongest mode bypasses review floor | critical review becomes one shallow pass | fallback changes native candidate only; process invariants remain external | end-to-end critical fallback fixture |

## 6. Acceptance

| ID | Acceptance criterion | Verify | Required evidence | Traces to |
|---|---|---|---|---|
| A-1 | Every live harness × role × derivable profile cell resolves to its declared qualified least-cost satisfying profile or explicit unsupported result; every resolved cell has a current `native-profile-qualification/v1` proving all four components and no cell is omitted. | Test + Human judgment — @mdarabi, rubric: each component claim's method/evidence/threshold supports the declared level and binds the exact launch generation | generated conformance/qualification matrix and sign-off | `prd:planner-driven-model-routing/FR-11` · `prd:implement-now/EVAL-2` · `prd:impl-review/EVAL-11` |
| A-2 | Candidate and field reordering produce byte-identical exact/promoted/unsupported results with zero inference and zero network access through the terminal result. | Test + Analysis | property suite and dependency inspection | `prd:planner-driven-model-routing/NFR-8` · #DINV-5 |
| A-3 | Every resolved profile dominates all four minima, selects the unique lowest cost rank, and records exactly the promoted components. | Test | exhaustive dominance/tie/promotion fixtures | `prd:planner-driven-model-routing/FR-8` · `prd:planner-driven-model-routing/FR-9` · #DINV-4 |
| A-4 | Missing, malformed, expired, invalidated, self-declared, component-insufficient, wrong-generation, weakening, incompatible, ambiguous-rank, and raw-argument policy/qualification cases yield zero claims/invocations and name harness, role, catalog, qualification, and defect. | Test + Inspection | negative policy/qualification/abuse corpus | `prd:planner-driven-model-routing/FR-12` · `prd:implement-now/ABUSE-5` · #DINV-3 · #DINV-6 |
| A-5 | Implementation and review fixtures with intentionally different catalogs launch their own declared native profiles and never reuse the other role's result. | Test | cross-role adapter fixture | `prd:planner-driven-model-routing/FR-14` · #DINV-2 |
| A-6 | A complete authorized override replaces exactly one catalog; partial/unauthorized overrides never become effective and never fall back silently. | Test + Inspection | config authorization and replacement provenance report | `prd:planner-driven-model-routing/NR-7` · #DINV-7 |
| A-7 | Strongest-mode drills stay on the selected harness/role, satisfy every minimum, preserve critical process depth/count, and fail unsupported when strongest is insufficient. | Test + Demo | fallback matrix and end-to-end critical review record | `prd:planner-driven-model-routing/FR-27` · `prd:implement-now/FR-26` · `prd:impl-review/FR-28` · #DINV-8 |
| A-8 | Actual launch capture exactly matches the selected native model, effort, and typed controls for every adapter; mismatches invalidate the attempt and qualification. | Test | real-runtime launch-fidelity report | `prd:implement-now/EVAL-2` · `prd:impl-review/EVAL-11` |

## 7. Open questions

None. Native inventory, cost rank, qualification evidence, and support status remain live operator-reviewed policy data; changing resolver semantics or schema requires a successor design.

## Principles

- **AP-1 — honored.** Harnesses are adapters selected upstream; core routing imports no adapter implementation.
- **AP-3 — honored.** One semantic demand and resolution grammar spans all harnesses and roles.
- **AP-5 — honored.** Every selection, promotion, failure, policy identity, and launch mismatch is observable.
- **AP-6 — honored.** Catalog, override, qualification, and result schemas parse closed.
- **AP-10 — honored.** Mapping and ranks are policy-as-data, not hardcoded model choice.
- **AP-11 — honored.** One resolver owns dominance and ordering across every adapter.
- **AP-13 — honored.** Qualification and actual-launch evidence precede trust.
- **AP-17 — honored.** Invalid or unsatisfied policy fails before claim/invocation.
- **AP-20 — honored.** Harness selection, semantic routing, adapter launch, evidence, and review process have distinct owners.
- **AP-21 — honored.** Native mapping consumes planner judgment and never reinterprets task semantics.
- **AP-24 — honored.** Capability is explicit at the adapter boundary rather than assumed from a product name.
- **AP-28 — honored.** The effective catalog and canonical events form one reproducible decision chain.
- **RW-5 — honored.** This document owns only native resolution policy.
- **Deviation:** none.
