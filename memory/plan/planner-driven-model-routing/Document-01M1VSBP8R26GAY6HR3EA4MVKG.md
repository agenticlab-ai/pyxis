# Planner-driven model routing — plan 002

| Field | Value |
|---|---|
| **Feature** | planner-driven-model-routing |
| **PRD** | docs/prds/planner-driven-model-routing |
| **Design** | docs/tech-design/native-routing-policy-v1.md |
| **Plan version** | 002 |
| **Dialect** | harbormaster/fs-spec:v1 |
| **Epic title** | Resolve semantic demand to qualified harness-native profiles |
| **Status** | Draft — freezes at merge |
| **Template version** | plan-template/v1 |

## Tasks

### T1 — Admit only closed, current, operator-qualified native catalogs

| Field | Value |
|---|---|
| **Blocked by** | — |
| **Size band** | 240–300 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | rung=3 |
| **Pattern** | — |
| **Touches** | internal/nativerouting catalog schema, internal/nativerouting qualification schema and validator, native-routing policy and qualification fixtures |

**Goal:** One closed validator admits a harness-role catalog only when every native profile is currently qualified for the exact adapter generation by operator-authorized evidence over all four semantic components.

**Current behavior:** Each runtime adapter carries an in-code rung ladder whose entries need no qualification identity, validity window, component claims, operator authorization, or generation binding, and no shared catalog schema exists.

**Desired behavior:** A catalog and every referenced qualification parse closed and validate as one unit against a trusted clock, the exact adapter generation, and repository authorization. Duplicate identities or cost ranks, unknown native values or controls, raw arguments, missing or insufficient claims, self-declared evidence, expiry, invalidation, and an ambiguous strongest profile invalidate the complete harness-role catalog with typed diagnostics and no partial candidate set.

**Scope:**
- Define the versioned catalog, entry, qualification, per-component claim, validity, invalidation, authorization, and strongest-profile contracts.
- Validate native values and typed controls through a consumer-owned adapter capability without importing any adapter implementation.
- Bind qualification to harness, role, profile, launch capture, adapter generation, evaluator versions, evidence digests, trusted time, and operator proof.
- Prove the complete malformed, stale, insufficient, injected, self-declared, and ambiguous policy corpus.

**Out of scope:**
- Dominance, candidate selection, repository override precedence, native launch construction, and evidence retention.
- Automatically qualifying a newly released model or trusting provider/model self-description.

**Acceptance criteria:**
- [ ] A complete qualified catalog for each role validates only when every entry has four sufficient component claims, a unique identity and cost rank, a current validity window, an exact adapter generation, a launch-capture digest, and operator authorization.
- [ ] Unknown keys, enum values, native values, typed controls, qualification methods, evaluator versions, and raw launch arguments each reject the entire affected catalog and name harness, role, profile, qualification, and defect.
- [ ] Missing, expired, invalidated, wrong-generation, component-insufficient, self-declared, duplicate-rank, and ambiguous-strongest fixtures return zero usable candidates.
- [ ] Reordered catalog fields, entries, and qualification claims produce the same canonical policy identity and validation result.

**Key interfaces:**
- `Catalog`, `CatalogEntry`, `Qualification`, `QualificationClaim`, `Validity`, `AuthorizationProof`
- `NativeCapability.ValidateProfile(CatalogEntry)`
- `ValidateCatalog(Catalog, QualificationSet, NativeCapability, TrustedClock, AuthorizationPolicy)`
- `native-routing-catalog/v1`, `native-profile-qualification/v1`

**Traces:** prd:planner-driven-model-routing/FR-11 · prd:planner-driven-model-routing/FR-12 · prd:planner-driven-model-routing/UB-4 · prd:planner-driven-model-routing/ABUSE-1 · prd:planner-driven-model-routing/ABUSE-4 · prd:planner-driven-model-routing/NFR-3 · design:native-routing-policy-v1#3.2 · design:native-routing-policy-v1#3.9 · design:native-routing-policy-v1#DINV-3 · design:native-routing-policy-v1#A-1 · design:native-routing-policy-v1#A-4

### T2 — Resolve one harness-role catalog by componentwise dominance

| Field | Value |
|---|---|
| **Blocked by** | T1 |
| **Size band** | 230–295 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | rung=3 |
| **Pattern** | — |
| **Touches** | internal/nativerouting resolver and result contract, semantic-order and dominance fixtures, internal/executor/port native-routing contracts, strongest-satisfying resolver fixtures |

**Goal:** A pure shared resolver converts verified semantic demand into the unique least-cost qualified profile for one already-selected harness and role, or a typed unsupported result, without inference or network access.

**Current behavior:** The executor asks an adapter's `TierMap` to clamp one integer rung directly to a model and effort pair; the adapters use different ladders and cannot express independent semantic minima, promotion, unsupported components, or role separation.

**Desired behavior:** Normal resolution filters one validated catalog by four-component dominance, selects its unique cheapest rank, and returns exact or promoted with every surplus component named. Strongest-satisfying mode changes only candidate preference inside the same harness-role catalog. No satisfying candidate returns unsupported with every unmet component and no native profile; input order never changes the canonical result.

**Scope:**
- Implement the closed request and exact, promoted, unsupported, and policy-invalid result grammar over verified execution demand.
- Implement componentwise dominance, unique least-cost ordering, promotion reporting, unmet-component reporting, and strongest-satisfying mode.
- Define the consumer-side catalog-provider and typed launch-builder contracts that every runtime adapter implements.
- Prove resolver purity, determinism, role separation, harness immutability, and zero partial resolution.

**Out of scope:**
- Selecting or probing a harness, reading task prose, persisting route evidence, orchestrating review passes, and invoking a model.
- Supplying default or repository catalogs; later tasks implement the data owners.

**Acceptance criteria:**
- [ ] Exhaustive profile fixtures select the unique lowest-cost qualified candidate that dominates capability, deliberation, context, and assurance; no surplus component compensates for a deficit.
- [ ] Exact and promoted results carry the complete requested profile, policy identity, harness, role, native profile, and precisely the promoted components; unsupported carries every unmet component and no native profile.
- [ ] Candidate, field, map, and input reordering produce byte-identical results with zero inference and zero network operations through the terminal result.
- [ ] Strongest mode never changes demand, harness, role, or caller process metadata, and returns unsupported when the catalog's strongest qualified profile misses any minimum.

**Key interfaces:**
- `Request`, `Resolution`, `ResolutionClass`, `UnmetComponent`, `Mode`
- `Resolver.Resolve(Request, ValidatedCatalog)`
- `CatalogProvider.Catalog(role)`, `LaunchBuilder.Build(SelectedProfile)`, `LaunchCapture`
- `native-route-resolution/v1`

**Traces:** prd:planner-driven-model-routing/FR-8 · prd:planner-driven-model-routing/FR-9 · prd:planner-driven-model-routing/FR-10 · prd:planner-driven-model-routing/FR-14 · prd:planner-driven-model-routing/FR-27 · prd:planner-driven-model-routing/NR-1 · prd:planner-driven-model-routing/UB-6 · prd:planner-driven-model-routing/NFR-1 · prd:planner-driven-model-routing/NFR-5 · prd:planner-driven-model-routing/NFR-8 · design:native-routing-policy-v1#3.1 · design:native-routing-policy-v1#3.3 · design:native-routing-policy-v1#3.4 · design:native-routing-policy-v1#3.6 · design:native-routing-policy-v1#DINV-1 · design:native-routing-policy-v1#DINV-2 · design:native-routing-policy-v1#DINV-4 · design:native-routing-policy-v1#DINV-5 · design:native-routing-policy-v1#DINV-6 · design:native-routing-policy-v1#DINV-8 · design:native-routing-policy-v1#A-2 · design:native-routing-policy-v1#A-3 · design:native-routing-policy-v1#A-7

### T3 — Replace one complete catalog through authorized repository policy

| Field | Value |
|---|---|
| **Blocked by** | T1, T2 |
| **Size band** | 190–260 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | rung=3 |
| **Pattern** | — |
| **Touches** | internal/executor/config native-routing policy, internal/nativerouting effective-catalog selection and provenance, repository policy authorization integration, catalog-override fixtures |

**Goal:** An authorized repository may replace exactly one complete compatible harness-role catalog while every partial, weakening, injected, or unauthorized override fails before resolution.

**Current behavior:** Repository executor config can choose a backend and routing category but cannot declare a qualified native catalog; native model ladders live inside adapter code with no effective-policy provenance.

**Desired behavior:** The closed repository config names a complete replacement catalog for a single harness and role. Effective-policy selection verifies schema and resolver compatibility, applies the same qualification and native-capability checks as a default catalog, and records default, override, authorization, and effective identities. Invalid overrides never field-merge and never silently fall back to defaults.

**Scope:**
- Extend the closed repository policy with complete harness-role catalog replacement references and authorization proof.
- Select exactly one default or replacement catalog and produce its deterministic provenance and digest.
- Reuse catalog validation and the shared resolver over the selected complete policy.
- Prove valid replacement, authorization, incompatibility, partial-merge, weakening, and fallback-refusal cases.

**Out of scope:**
- Editing frozen plans, selecting a harness, accepting raw CLI arguments, and changing unrelated executor routing or cap configuration.
- Updating native profile inventories automatically.

**Acceptance criteria:**
- [ ] One compatible operator-authorized complete catalog replacement becomes the sole effective policy for its exact harness and role and records default, override, authorization, and effective identities.
- [ ] Partial, field-merged, unknown-key, incompatible-version, unqualified, weakening, contributor-only, worker-authored, and raw-argument overrides fail before resolution or launch.
- [ ] An invalid override never falls back to the released default and leaves zero effective candidate set for the affected harness-role pair.
- [ ] Existing backend selection, review configuration, caps, gates, and closed-parse fixtures remain byte-identical outside the new policy block.

**Key interfaces:**
- `CatalogOverride`, `EffectiveCatalog`, `ReplacementProvenance`
- `SelectEffectiveCatalog(DefaultCatalog, CatalogOverride, AuthorizationPolicy)`
- the closed `executor.native_routing` repository-policy block

**Traces:** prd:planner-driven-model-routing/FR-12 · prd:planner-driven-model-routing/NR-7 · prd:planner-driven-model-routing/UB-4 · prd:planner-driven-model-routing/ABUSE-4 · prd:planner-driven-model-routing/NFR-3 · prd:planner-driven-model-routing/NFR-7 · design:native-routing-policy-v1#3.5 · design:native-routing-policy-v1#3.9 · design:native-routing-policy-v1#DINV-7 · design:native-routing-policy-v1#A-6

### T4 — Publish and launch Codex role catalogs faithfully

| Field | Value |
|---|---|
| **Blocked by** | T3 |
| **Size band** | 180–250 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | rung=3 |
| **Pattern** | — |
| **Touches** | internal/executor/runtime/codex native catalogs and qualifications, Codex typed launch builder and launch capture, Codex role-routing conformance fixtures |

**Goal:** Codex independently resolves implementation and review demand through qualified catalogs and launches exactly the selected model, reasoning effort, and typed controls.

**Current behavior:** Codex clamps one rung through an in-code ladder and emits model plus `model_reasoning_effort`; the ladder is not role-qualified or evidence-bound and launch capture cannot prove equality to a semantic route.

**Desired behavior:** The adapter publishes complete implementation and review catalogs whose native values and controls it recognizes, provides current qualification evidence for every resolved cell, builds launch settings only from the selected profile, and captures actual model, effort, and controls for exact comparison. Any mismatch fails the attempt and invalidates the affected qualification.

**Scope:**
- Supply separate qualified Codex implementation and review catalogs over every derivable semantic profile.
- Translate a selected profile into the adapter's typed model, reasoning-effort, and supported control contract.
- Capture the actual native launch settings and enforce exact selected-versus-actual equality.
- Run the shared role-routing and launch-fidelity conformance suite with hermetic decode-pinned fixtures.

**Out of scope:**
- Harness selection, task claim, model invocation orchestration, route-event retention, and review-pass count.

**Acceptance criteria:**
- [ ] Every Codex implementation and review domain cell returns its declared qualified satisfying profile or explicit unsupported result, with no omitted profile and no implicit role reuse.
- [ ] The launch builder emits only the selected typed model, reasoning effort, and recognized controls; raw arguments and unknown settings cannot enter the invocation.
- [ ] Captured model, effort, and controls equal the selected entry for every resolved fixture; each seeded difference fails and invalidates qualification.
- [ ] Changing roster data and qualification evidence requires no frozen-plan edit or resolver-code change.

**Key interfaces:**
- the Codex `CatalogProvider`, `NativeCapability`, and `LaunchBuilder` implementations
- Codex launch capture decoded into the shared `LaunchCapture` contract

**Traces:** prd:planner-driven-model-routing/FR-11 · prd:planner-driven-model-routing/FR-14 · prd:planner-driven-model-routing/NFR-7 · prd:planner-driven-model-routing/EVAL-2 · design:native-routing-policy-v1#3.2 · design:native-routing-policy-v1#3.4 · design:native-routing-policy-v1#3.10 · design:native-routing-policy-v1#DINV-2 · design:native-routing-policy-v1#DINV-3 · design:native-routing-policy-v1#A-1 · design:native-routing-policy-v1#A-5 · design:native-routing-policy-v1#A-8

### T5 — Publish and launch Claude Code role catalogs faithfully

| Field | Value |
|---|---|
| **Blocked by** | T3 |
| **Size band** | 180–250 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | rung=3 |
| **Pattern** | — |
| **Touches** | internal/executor/runtime/claude native catalogs and qualifications, Claude Code typed launch builder and launch capture, Claude Code role-routing conformance fixtures |

**Goal:** Claude Code independently resolves implementation and review demand through qualified catalogs and launches exactly the selected model, effort, and typed controls.

**Current behavior:** Claude Code ignores its rung ladder and pins every task to Sonnet 5 at maximum effort; the override is not role-qualified or evidence-bound and launch capture cannot prove equality to a semantic route.

**Desired behavior:** The adapter publishes complete implementation and review catalogs whose native values and controls it recognizes, provides current qualification evidence for every resolved cell, builds launch settings only from the selected profile, and captures actual model, effort, and controls for exact comparison. Any mismatch fails the attempt and invalidates the affected qualification.

**Scope:**
- Supply separate qualified Claude Code implementation and review catalogs over every derivable semantic profile.
- Translate a selected profile into the adapter's typed model, effort, and supported control contract.
- Capture the actual native launch settings and enforce exact selected-versus-actual equality.
- Run the shared role-routing and launch-fidelity conformance suite with hermetic decode-pinned fixtures.

**Out of scope:**
- Harness selection, task claim, model invocation orchestration, route-event retention, and review-pass count.

**Acceptance criteria:**
- [ ] Every Claude Code implementation and review domain cell returns its declared qualified satisfying profile or explicit unsupported result, with no omitted profile and no implicit role reuse.
- [ ] The launch builder emits only the selected typed model, effort, and recognized controls; raw arguments and unknown settings cannot enter the invocation.
- [ ] Captured model, effort, and controls equal the selected entry for every resolved fixture; each seeded difference fails and invalidates qualification.
- [ ] The Sonnet-5 maximum-effort pin is no longer a route truth, while changing roster data and qualification evidence requires no frozen-plan edit or resolver-code change.

**Key interfaces:**
- the Claude Code `CatalogProvider`, `NativeCapability`, and `LaunchBuilder` implementations
- Claude Code launch capture decoded into the shared `LaunchCapture` contract

**Traces:** prd:planner-driven-model-routing/FR-11 · prd:planner-driven-model-routing/FR-14 · prd:planner-driven-model-routing/NFR-7 · prd:planner-driven-model-routing/EVAL-2 · design:native-routing-policy-v1#3.2 · design:native-routing-policy-v1#3.4 · design:native-routing-policy-v1#3.10 · design:native-routing-policy-v1#DINV-2 · design:native-routing-policy-v1#DINV-3 · design:native-routing-policy-v1#A-1 · design:native-routing-policy-v1#A-5 · design:native-routing-policy-v1#A-8

### T6 — Publish and launch Cursor CLI role catalogs faithfully

| Field | Value |
|---|---|
| **Blocked by** | T3 |
| **Size band** | 180–250 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | rung=3 |
| **Pattern** | — |
| **Touches** | internal/executor/runtime/cursor native catalogs and qualifications, Cursor CLI typed launch builder and launch capture, Cursor CLI role-routing conformance fixtures |

**Goal:** Cursor CLI independently resolves implementation and review demand through qualified catalogs and launches exactly the selected model-parameter form and typed controls.

**Current behavior:** Cursor clamps one rung through an in-code ladder and folds effort and context into model strings because the CLI exposes no separate effort flag; the ladder is not role-qualified or evidence-bound and launch capture cannot prove equality to a semantic route.

**Desired behavior:** The adapter publishes complete implementation and review catalogs over only model-parameter forms it recognizes, provides current qualification evidence for every resolved cell, builds launch settings only from the selected profile, and captures actual model parameters and controls for exact comparison. Any mismatch fails the attempt and invalidates the affected qualification.

**Scope:**
- Supply separate qualified Cursor CLI implementation and review catalogs over every derivable semantic profile.
- Translate a selected profile into the adapter's typed model-parameter and supported control contract without inventing an effort flag.
- Capture the actual native launch settings and enforce exact selected-versus-actual equality.
- Run the shared role-routing and launch-fidelity conformance suite with hermetic decode-pinned fixtures.

**Out of scope:**
- Harness selection, task claim, model invocation orchestration, route-event retention, and review-pass count.

**Acceptance criteria:**
- [ ] Every Cursor CLI implementation and review domain cell returns its declared qualified satisfying profile or explicit unsupported result, with no omitted profile and no implicit role reuse.
- [ ] The launch builder emits only the selected recognized model-parameter form and typed controls and never fabricates an unsupported effort flag or accepts raw arguments.
- [ ] Captured model parameters and controls equal the selected entry for every resolved fixture; each seeded difference fails and invalidates qualification.
- [ ] Changing roster data and qualification evidence requires no frozen-plan edit or resolver-code change.

**Key interfaces:**
- the Cursor CLI `CatalogProvider`, `NativeCapability`, and `LaunchBuilder` implementations
- Cursor CLI launch capture decoded into the shared `LaunchCapture` contract

**Traces:** prd:planner-driven-model-routing/FR-11 · prd:planner-driven-model-routing/FR-14 · prd:planner-driven-model-routing/NFR-7 · prd:planner-driven-model-routing/EVAL-2 · design:native-routing-policy-v1#3.2 · design:native-routing-policy-v1#3.4 · design:native-routing-policy-v1#3.10 · design:native-routing-policy-v1#DINV-2 · design:native-routing-policy-v1#DINV-3 · design:native-routing-policy-v1#A-1 · design:native-routing-policy-v1#A-5 · design:native-routing-policy-v1#A-8

### T7 — Publish and launch OpenCode role catalogs faithfully

| Field | Value |
|---|---|
| **Blocked by** | T3 |
| **Size band** | 180–250 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | rung=3 |
| **Pattern** | — |
| **Touches** | internal/executor/runtime/opencode native catalogs and qualifications, OpenCode typed launch builder and launch capture, OpenCode role-routing conformance fixtures |

**Goal:** OpenCode independently resolves implementation and review demand through qualified catalogs and launches exactly the selected provider/model and typed controls.

**Current behavior:** OpenCode clamps one rung through an in-code provider/model ladder and carries effort only as descriptive metadata because the CLI exposes no effort flag; the ladder is not role-qualified or evidence-bound and launch capture cannot prove equality to a semantic route.

**Desired behavior:** The adapter publishes complete implementation and review catalogs over provider/model values and controls it recognizes, provides current qualification evidence for every resolved cell, builds launch settings only from the selected profile, and captures actual provider/model and controls for exact comparison. Any mismatch fails the attempt and invalidates the affected qualification.

**Scope:**
- Supply separate qualified OpenCode implementation and review catalogs over every derivable semantic profile.
- Translate a selected profile into the adapter's typed provider/model and supported control contract without inventing an effort flag.
- Capture the actual native launch settings and enforce exact selected-versus-actual equality.
- Run the shared role-routing and launch-fidelity conformance suite with hermetic decode-pinned fixtures.

**Out of scope:**
- Harness selection, task claim, model invocation orchestration, route-event retention, and review-pass count.

**Acceptance criteria:**
- [ ] Every OpenCode implementation and review domain cell returns its declared qualified satisfying profile or explicit unsupported result, with no omitted profile and no implicit role reuse.
- [ ] The launch builder emits only the selected recognized provider/model and typed controls and never fabricates an unsupported effort flag or accepts raw arguments.
- [ ] Captured provider/model and controls equal the selected entry for every resolved fixture; each seeded difference fails and invalidates qualification.
- [ ] Changing roster data and qualification evidence requires no frozen-plan edit or resolver-code change.

**Key interfaces:**
- the OpenCode `CatalogProvider`, `NativeCapability`, and `LaunchBuilder` implementations
- OpenCode launch capture decoded into the shared `LaunchCapture` contract

**Traces:** prd:planner-driven-model-routing/FR-11 · prd:planner-driven-model-routing/FR-14 · prd:planner-driven-model-routing/NFR-7 · prd:planner-driven-model-routing/EVAL-2 · design:native-routing-policy-v1#3.2 · design:native-routing-policy-v1#3.4 · design:native-routing-policy-v1#3.10 · design:native-routing-policy-v1#DINV-2 · design:native-routing-policy-v1#DINV-3 · design:native-routing-policy-v1#A-1 · design:native-routing-policy-v1#A-5 · design:native-routing-policy-v1#A-8

### T8 — Enumerate the complete live routing domain

| Field | Value |
|---|---|
| **Blocked by** | T4, T5, T6, T7 |
| **Size band** | 220–290 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | rung=3 |
| **Pattern** | — |
| **Touches** | internal/nativerouting/conformancetest domain generator and matrix, live harness-role catalog qualification fixtures, routing-conformance report and event payload fixtures |

**Goal:** One generated matrix proves that no supported harness, role, or derivable semantic profile can disappear from routing support by omission.

**Current behavior:** Runtime conformance enumerates caller-supplied integer rungs per adapter; it has no derivation-policy domain, role dimension, qualification matrix, or explicit unsupported cell.

**Desired behavior:** The conformance generator derives every possible execution profile from the live demand policy, crosses it with every runtime-registry harness and both roles, and requires each cell to produce its declared qualified satisfying resolution or explicit unsupported result. Each completed cell emits one closed conformance result carrying its evidence reference, and the matrix refuses omissions, duplicate cells, stale qualifications, or an unregistered catalog owner.

**Scope:**
- Generate the semantic profile domain from the live derivation policy rather than a hand-maintained list.
- Enumerate the runtime registry, both closed roles, and every profile into one deterministic matrix.
- Run shared catalog, resolution, qualification, and adapter conformance for every cell and emit the closed result payload.
- Produce a reviewable qualification matrix and operator-signoff input without embedding prompts or code.

**Out of scope:**
- Persisting the conformance payload as canonical routing evidence, changing model rosters, and activating production routing.

**Acceptance criteria:**
- [ ] The generated matrix contains every registered harness × implementation/review role × derivable profile cell exactly once and fails when any harness, role, profile, catalog, or result is omitted.
- [ ] Every resolved cell names a current qualification proving all four components and the exact adapter generation; every other cell is explicitly unsupported with unmet components.
- [ ] Reordering registry, policy, catalog, and fixture inputs produces a byte-identical matrix and cell payload order.
- [ ] The reviewable report contains the qualification method, evidence digest, threshold/result, validity, and responsible evaluator for every resolved cell and no prompt, code, credential, or raw launch argument.

**Key interfaces:**
- `Domain`, `Cell`, `CellResult`, `Matrix`, `QualificationReport`
- `GenerateDomain(DerivationPolicy, RuntimeRegistry, Roles)`
- `routing_conformance_checked` payload

**Traces:** prd:planner-driven-model-routing/FR-11 · prd:planner-driven-model-routing/EVAL-2 · prd:planner-driven-model-routing/EVT-4 · design:native-routing-policy-v1#3.10 · design:native-routing-policy-v1#A-1

### T9 — Prove fail-closed routing, fallback, and launch fidelity end to end

| Field | Value |
|---|---|
| **Blocked by** | T3, T8 |
| **Size band** | 190–260 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | rung=3 |
| **Pattern** | — |
| **Touches** | internal/nativerouting acceptance and abuse corpus, cross-adapter launch-capture fidelity harness, resolver network and inference guard, strongest-satisfying fallback matrix |

**Goal:** Cross-adapter acceptance proves that every policy defect, unsupported demand, fallback, and launch mismatch preserves semantic demand and performs no unauthorized claim or invocation.

**Current behavior:** Existing ladders clamp all rungs to a native value, including out-of-range inputs, so the runtime has no cross-adapter proof for unsupported demand, qualification abuse, role separation, offline resolution, or selected-versus-actual launch equality.

**Desired behavior:** One acceptance corpus runs exact, promoted, unsupported, invalid-policy, override, role-divergence, strongest-fallback, and launch-mismatch cases across every adapter. Resolution is offline and deterministic; every non-resolved case returns zero launch authority; strongest mode stays within the selected harness and role while preserving all four minima and caller review-process metadata; actual launch capture must exactly match the selected profile.

**Scope:**
- Exercise the complete invalid catalog, qualification, override, dominance, unsupported, role, and fallback corpus over every adapter.
- Trace the resolver boundary to prove zero inference and zero network operations through its terminal result.
- Compare selected and captured native model, effort, and typed controls through the shared equality predicate.
- Emit reproducible diagnostics and acceptance evidence for exact, promoted, unsupported, fallback, and mismatch cases.

**Out of scope:**
- Task claim orchestration, canonical evidence-store append, assurance-pass aggregation, production activation, and cross-harness fallback.

**Acceptance criteria:**
- [ ] Every malformed, missing, stale, unqualified, insufficient, wrong-generation, weakening, ambiguous-rank, injected, and unauthorized case yields zero launch authority and names harness, role, catalog, qualification, and defect.
- [ ] Exact, promoted, unsupported, and strongest-fallback cases are byte-identical across repeats and reordered inputs with zero inference and zero network operations through the terminal result.
- [ ] Every adapter's captured native model, effort representation, and typed controls equal its selected profile; each seeded mismatch fails the attempt and invalidates that qualification.
- [ ] Strongest mode stays on the selected harness and role, dominates every requested minimum or returns unsupported, and leaves caller-supplied review depth, pass count, blindness, and aggregate metadata byte-identical.

**Key interfaces:**
- the cross-adapter acceptance corpus and `LaunchFidelity` equality predicate
- a resolver-side effect recorder for inference and network operations
- the strongest-satisfying acceptance matrix

**Traces:** prd:planner-driven-model-routing/FR-8 · prd:planner-driven-model-routing/FR-9 · prd:planner-driven-model-routing/FR-10 · prd:planner-driven-model-routing/FR-12 · prd:planner-driven-model-routing/FR-14 · prd:planner-driven-model-routing/FR-27 · prd:planner-driven-model-routing/NR-1 · prd:planner-driven-model-routing/NR-7 · prd:planner-driven-model-routing/UB-4 · prd:planner-driven-model-routing/UB-6 · prd:planner-driven-model-routing/ABUSE-1 · prd:planner-driven-model-routing/ABUSE-4 · prd:planner-driven-model-routing/NFR-1 · prd:planner-driven-model-routing/NFR-3 · prd:planner-driven-model-routing/NFR-5 · prd:planner-driven-model-routing/NFR-8 · prd:planner-driven-model-routing/EVAL-2 · design:native-routing-policy-v1#3.3 · design:native-routing-policy-v1#3.4 · design:native-routing-policy-v1#3.5 · design:native-routing-policy-v1#3.6 · design:native-routing-policy-v1#3.10 · design:native-routing-policy-v1#DINV-1 · design:native-routing-policy-v1#DINV-2 · design:native-routing-policy-v1#DINV-3 · design:native-routing-policy-v1#DINV-4 · design:native-routing-policy-v1#DINV-5 · design:native-routing-policy-v1#DINV-6 · design:native-routing-policy-v1#DINV-7 · design:native-routing-policy-v1#DINV-8 · design:native-routing-policy-v1#A-2 · design:native-routing-policy-v1#A-3 · design:native-routing-policy-v1#A-4 · design:native-routing-policy-v1#A-5 · design:native-routing-policy-v1#A-6 · design:native-routing-policy-v1#A-7 · design:native-routing-policy-v1#A-8

## Coverage

| Requirement | Tasks |
|---|---|
| prd:planner-driven-model-routing/FR-8 | T2, T9 |
| prd:planner-driven-model-routing/FR-9 | T2, T9 |
| prd:planner-driven-model-routing/FR-10 | T2, T9 |
| prd:planner-driven-model-routing/FR-11 | T1, T4, T5, T6, T7, T8 |
| prd:planner-driven-model-routing/FR-12 | T1, T3, T9 |
| prd:planner-driven-model-routing/FR-14 | T2, T4, T5, T6, T7, T9 |
| prd:planner-driven-model-routing/FR-27 | T2, T9 |
| prd:planner-driven-model-routing/NR-1 | T2, T9 |
| prd:planner-driven-model-routing/NR-7 | T3, T9 |
| prd:planner-driven-model-routing/UB-4 | T1, T3, T9 |
| prd:planner-driven-model-routing/UB-6 | T2, T9 |
| prd:planner-driven-model-routing/ABUSE-1 | T1, T9 |
| prd:planner-driven-model-routing/ABUSE-4 | T1, T3, T9 |
| prd:planner-driven-model-routing/NFR-1 | T2, T9 |
| prd:planner-driven-model-routing/NFR-3 | T1, T3, T9 |
| prd:planner-driven-model-routing/NFR-5 | T2, T9 |
| prd:planner-driven-model-routing/NFR-7 | T3, T4, T5, T6, T7 |
| prd:planner-driven-model-routing/NFR-8 | T2, T9 |
| prd:planner-driven-model-routing/EVAL-2 | T4, T5, T6, T7, T8, T9 |
| prd:planner-driven-model-routing/EVT-4 | T8 |
