# Amendment 001 — remove-no-inference

| Field | Value |
|---|---|
| **Amends** | planner-driven-model-routing *(000)* |
| **Status** | Approved |
| **Owner** | @mdarabi |
| **Date** | 2026-07-26 |
| **Learned-from** | operator-decision:design-brainstorm:planner-driven-model-routing-tech-design#D-5 |
| **Blast radius** | tier: contract-changing; touches: FR-1, FR-2, FR-16, FR-19, NR-5, UB-1, NG-8, §0, §1, §3.2, §4.1, §4.2, §4.3, §5.2, §5.3, §5.4, §5.5, §5.7, §5.8, §5.9, §6.1, §7.3, §8; reason: remove a task mode that `/implement-now` cannot execute, make the full model-backed demand contract mandatory for every task entering that engine, and close the affected evidence chain from freeze or legacy translation through runtime resolution |
| **Template version** | amendment-template/v2 |

## Operations

### op: supersede FR-1

Reason: every plan/v2 task projected for `/implement-now` is model-backed; the contract must not retain a conditional class that permits an unassessed executor task.

| ID | Requirement (EARS) | Rationale | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|
| FR-22 | When the planner freezes a plan/v2 task for `/implement-now`, the planner shall record `reasoning`, `coupling`, `novelty`, `verification`, `impact_scope`, and `architectural_role` using the normative assessment vocabulary. | every executor task needs complete inputs so dispatch cannot fall through to an absent, cheapest, or hardcoded route | Must | critical | every `/implement-now` plan/v2 golden carries exactly one accepted value for every field; missing and unknown values fail validation | Test + Eval (EVAL-1) | G1, J1, F1 |

### op: retire FR-2

Reason: `/implement-now` has no deterministic recipe or command contract, and a profile-free task cannot satisfy the required proposal-review join; task-level `no_inference` is removed rather than invented during design.

### op: supersede FR-19

Reason: assessment evidence is mandatory for every `/implement-now` plan/v2 task now that the task-level deterministic exception is removed.

| ID | Requirement (EARS) | Rationale | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|
| FR-23 | When the planner freezes a plan/v2 task for `/implement-now`, the planner shall record at least one resolvable evidence citation supporting its assessment values. | the assessment must preserve why every executor task was classified, not only its labels | Must | high | uncited or unresolved evidence fails plan validation before freeze; EVAL-1 proves that every recorded assessment value is supported by at least one cited anchor and rejects irrelevant, contradictory, or only partially supporting citation sets without requiring a distinct citation per axis | Test + Eval (EVAL-1) | G1, J1, J4, F1, F4 |

### op: supersede FR-16

Reason: the universal routing-evidence promise must admit both native plan/v2 demand and no-rewrite plan/v1 translation; otherwise legacy attempts have no typed origin event from which the promised terminal chain can start.

| ID | Requirement (EARS) | Rationale | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|
| FR-24 | For every implementation and review routing attempt, the system shall record either the frozen plan/v2 assessment, derived profile, evidence citations, and policy version or a typed no-rewrite plan/v1 origin containing the original rung presence/value, compatibility-table version, translation classification, and translated profile; it shall also record the selected harness and role, connected task/proposal identity, and an `exact`, `promoted`, or `unsupported` resolution, with native profile identity on resolved routes. | operators and successor routing need one complete routing decision without pretending legacy plans contain fields they never froze | Must | high | every attempt links by `demand_id` to exactly one EVT-1 or EVT-8 origin and by `attempt_id` to exactly one EVT-2 or EVT-3 terminal; plan/v1 explicit-rung, absent-rung, retry, and review fixtures remain unambiguous, every EVT-2 has a native profile, every EVT-3 has none, and source plan bytes remain unchanged | Test + Analysis | G1, G3, J4, F2, F3, F4, F5 |

### op: retire NR-5

Reason: task-level deterministic execution is not an existing `/implement-now` behavior and cannot be protected as a non-regression contract.

### op: add NR

Reason: removing a task-level `no_inference` mode must not weaken the executor's real zero-work, zero-inference behavior.

| ID | Existing behavior protected | Why protected | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|
| NR-8 | An `/implement-now` run with no ready task performs no model invocation. | every executable task is model-backed, but an empty frontier remains zero-work and must not spend inference | high | empty-frontier fixtures emit zero model-launch events and zero task claims | Test | NG-8 |

### op: supersede UB-1

Reason: missing-assessment failure applies to every plan/v2 task entering `/implement-now`, with no deterministic exception.

| ID | If… (trigger) | Then the system shall… | User sees | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|---|
| UB-11 | an `/implement-now` plan/v2 task omits an assessment field or uses an unknown value | reject the plan before freeze and name the field and accepted vocabulary | named plan-contract error | Must | critical | every missing- and unknown-value fixture is rejected before freeze and projection, and the diagnostic asserts the offending field plus its complete accepted vocabulary | Test | FR-22 |

### op: add NG

Reason: the removed task mode is a product non-goal, and adding it as a typed row preserves its amendment provenance in the folded source map.

| ID | Non-goal | Why refused |
|---|---|---|
| NG-8 | A task-level `no_inference` arm or deterministic recipe language in plan/v2 or `/implement-now` | the executor has no deterministic task contract, and profile-free tasks conflict with model-backed proposal-review semantics; deterministic commands remain internal to gates, generators, and tooling (`design-brainstorm:planner-driven-model-routing-tech-design#D-5`) |

### op: supersede §0 Read to rule — the human reviewer's section

Reason: §0's informative routing is refreshed wholesale because the amendment removes one task class and changes the denominator routed to FR-22/FR-23.

*The body is the contract; this section only routes attention — on any conflict the body governs; machines ignore it.*

**Judge these**

| # | The call | Could have gone | If you disagree |
|---|---|---|---|
| 1 | Every `/implement-now` plan/v2 task carries the complete assessment and checked profile; there is no task-level `no_inference` arm | introduce a deterministic recipe/task contract inside the executor | the planner/executor boundary and review semantics change → FR-22, FR-23, NG-8, NR-8 |
| 2 | Plans freeze all six assessment dimensions **and** a checked four-part execution profile; no native model identifiers | one scalar complexity tier, raw dimensions only, or native settings in the plan | the planner/runtime boundary changes → §4.1, FR-3..FR-4, FR-22 |
| 3 | Harness adapters independently map implementation and review demand to the least-cost qualified native profile | one global model map or the external CLI interpreting Pyxis demand | portability, qualification ownership, and role separation change → FR-8..FR-12 |
| 4 | Assurance is mechanically floored, planner-raise-only, and monotonically joined with the actual proposal lane for review | planner-scored risk or merge-lane-only review | critical-task scrutiny can be lowered or a second risk classifier appears → FR-5, FR-6, FR-13 |
| 5 | Missing exact profiles promote upward or fail before work; they never downgrade or silently switch harnesses | clamp down or cross-harness fallback | judge-once routing, credentials, and egress boundaries change → FR-9, FR-10, NR-1 |
| 6 | Tested routing activates directly for every supported production path; there is no shadow or canary stage | staged production exposure | the operator's explicit rollout ruling changes → §6.5, FR-17 |

**Sanity-check the numbers** — 100% of `/implement-now` plan/v2 candidates carry complete demand contracts, zero under-routing, zero critical-floor violations, and at least three distinct native profiles appear in pre-release dogfood (§3.2–§3.3).

**The author's open questions** — none at product level; Q1–Q3 (§7.2) are design-owned mechanics bounded by requirements.

**Skip the rest** — structure/schema → docgate; substance/coherence → unmanned PRD review; implementation and mapping defects → technical-design, plan, conformance, and independent-implementation-review gates.

### op: supersede §1 Summary — the one-pager

Reason: the summary's success denominator and execution-path statement must no longer imply a deterministic task exception.

- **Problem** — Planning can identify that tasks differ, but the execution path neither preserves enough of that judgment nor consistently turns it into implementation and review capacity. Routing data is optional or lost, runtime scales disagree, one worker path is temporarily pinned to a single strongest setting, and proposal review launches without an explicit model or effort.
- **For whom** — Primarily the model-backed planner→implementation→review path on every planned `/implement-now` task; secondarily the repository operator who must understand and constrain native model selection.
- **Why now** — The coarse routing seam already exists in the plan and runtime contracts, but its gaps now force hardcoding and silent defaults. Adaptive routing cannot learn safely until the factory can first produce, preserve, resolve, and observe an explicit execution-demand contract.
- **Direction** — The planner records a closed workload-and-consequence assessment and its deterministically derived, vendor-neutral execution profile for every plan/v2 task entering `/implement-now`. The already-selected harness adapter maps that minimum profile to the least-cost qualified implementation or review launch settings; it may promote visibly but may never downgrade or silently change harnesses. Finished-proposal risk can increase review assurance but never erase the planned floor.
- **Appetite** — One vertical routing-contract increment, including legacy translation, role-specific mappings, complete telemetry, conformance, dogfood, and a strongest-profile kill switch; no self-tuning, fallback, general authoring routing, automatic model qualification, or deterministic executor recipe language.
- **Success** — At the pre-release checkpoint, 100% of `/implement-now` plan/v2 candidates carry valid evidence-backed assessments and derived profiles, the full required routing domain conforms, zero dispatches under-route, and dogfood proves at least three distinct native profiles with verified outcomes.
- **Guardrail** — No critical assurance floor is lowered or skipped, no frozen v1 plan becomes unexecutable, and an empty execution frontier still launches no model.
- **Top risks** — A plausible but wrong planner assessment under-routes work; a stale native mapping overstates a model's qualification; transport or review code silently discards the frozen demand.

### op: supersede §3.2 Goals → signals → metrics

Reason: M1's denominator now covers every plan/v2 task presented for `/implement-now`; no deterministic task class is excluded.

| Metric | Goal | Signal | Baseline | Target (at checkpoint) | Feeds from |
|---|---|---|---|---|---|
| M1 (primary) | G1 | valid model-backed demand contracts ÷ every plan/v2 task presented for `/implement-now` freeze | contract absent | 100% at pre-release acceptance | EVT-6 |
| M2 | G1, G2 | requested profiles whose non-assurance components are field-identical to their frozen plan/v2 or translated plan/v1 demand and whose assurance is exactly that origin floor for implementation or its monotone join with the actual proposal band for review, before any visible native promotion; dispatch then satisfies every component | not measurable; current paths can lose, substitute, or override routing | 100% exact demand transport/join and satisfying dispatch; zero unauthorized substitution or downward resolution in conformance and dogfood | EVT-1, EVT-2, EVT-3, EVT-8 |
| M3 | G2 | required harness × role × derivable-profile cells producing their specified satisfying or unsupported result under adapter conformance | no common conformance proof | 100% of the required routing domain before release | EVT-4 |
| M4 | G2 | distinct native launch profiles exercised by verified dogfood tasks | implementation can converge on one pin | at least three before production activation | EVT-2, EVT-5 |
| M5 | G3 | terminal routing attempts with a complete evidence chain: frozen plan/v2 demand or translated plan/v1 origin→unsupported result, or origin→resolved route→worker outcome | incomplete and fragmented | 100% at pre-release acceptance and in production | EVT-1, EVT-2, EVT-3, EVT-5, EVT-8 |

### op: supersede §4.1 Approach and alternatives considered

Reason: the chosen approach now applies to every plan/v2 task entering `/implement-now` and explicitly rejects inventing a deterministic executor contract.

The chosen approach freezes two related but distinct artifacts on every plan/v2 task projected for `/implement-now`: a closed six-axis assessment that explains the work, and a deterministically derived four-part execution profile that states the minimum capability, deliberation, context, and assurance required. The planner supplies evidence and may raise assurance; a mechanical plan gate recomputes policy output and rejects disagreement. The selected harness adapter then resolves the semantic minimum through separate qualified implementation and review mappings.

| Option | Sketch | Why not chosen |
|---|---|---|
| A (chosen) | raw assessment + checked semantic profile; adapter-owned native resolution for every `/implement-now` task | preserves explainability, judge-once semantics, harness portability, and a usable review demand |
| B | one scalar complexity tier | collapses independent reasoning, context, consequence, and assurance needs |
| C | raw dimensions only; executor interprets them | moves planning judgment to dispatch and permits adapters to disagree about task meaning |
| D | native model and effort frozen in each plan | couples durable plans to volatile vendor catalogs and one harness |
| E | full adaptive routing now | combines the routing substrate with an optimizer before reliable outcomes exist |
| F | a task-level `no_inference` arm with deterministic executor recipes | `/implement-now` has no such task contract, and adding one would expand the product rather than remove model-routing hardcoding |

### op: supersede §4.2 Scope perimeter

Reason: scope now names the universal model-backed executor contract and excludes deterministic executor recipes.

- **In (plan of record)** — a complete closed evidence-backed task assessment and deterministically derived semantic profile for every plan/v2 task projected to `/implement-now`; assurance floor and planner raise; plan validation and transport; the full baseline harness/role/profile domain; role-specific adapter mappings with closed operator-authorized repository overrides; least-cost satisfying resolution, promotion, and named unsupported failure; independent-review join; frozen v1 translation; immutable routing/outcome evidence; conformance and model-behavior evals; direct production activation after the pre-release gate; strongest-profile kill switch; post-defect evidence invalidation and operator remediation brief.
- **Future** — adaptive scorecards and calibration; harness selection/fallback; broader authoring routing.
- **Out** — NG-1..NG-8.

### op: supersede §4.3 Experience: flows and states

Reason: F1 must cite the live universal missing-assessment failure row, and F5 must include the legacy evidence-chain contract that makes translated attempts observable without rewriting the plan.

| Flow | Happy path | States | Failure coverage |
|---|---|---|---|
| F1 — plan and freeze | planner records all workload and consequence dimensions with evidence; policy derives a matching profile | unassessed → assessed → validated/frozen | UB-11, UB-2 |
| F2 — implementation | executor reads frozen demand; selected implementation mapping resolves the least-cost qualified native profile | claimed → resolved/promoted → launched → verified | UB-3, UB-4, UB-5, UB-6 |
| F3 — independent review | reviewer reads frozen demand plus the actual proposal band; review mapping resolves independently | proposal-ready → joined → resolved/promoted → reviewed | UB-4, UB-5, UB-6 |
| F4 — operator audit/override | operator inspects the derivation and native result, then authorizes a constrained repository override if needed | observed → proposed → authorized → active | UB-7 |
| F5 — legacy execution | frozen v1 routing translates through the versioned compatibility table without rewriting the plan and emits a typed legacy origin | legacy-read → translated/recorded → resolved | UB-8 |

### op: supersede §5.2 Unwanted behavior and failure handling

Reason: failure rows that depend on the routing-evidence contract must cite FR-24, the live replacement that covers both plan/v2 and translated plan/v1 origins.

| ID | If… | Then the system shall… | User sees | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|---|
| UB-11 *(001 ← UB-1)* | an `/implement-now` plan/v2 task omits an assessment field or uses an unknown value | reject the plan before freeze and name the field and accepted vocabulary | named plan-contract error | Must | critical | every missing- and unknown-value fixture is rejected before freeze and projection, and the diagnostic asserts the offending field plus its complete accepted vocabulary | Test | FR-22 |
| UB-2 | a frozen derived profile or policy version does not match mechanical recomputation | reject the plan before projection and show expected versus supplied semantic values | named derivation-mismatch error | Must | critical | every mismatch fixture fails with zero task projection | Test | FR-3, FR-4 |
| UB-3 | a projected or tracker-reconstructed task loses or changes routing data | refuse claim and name the boundary/field mismatch | task remains unclaimed; integrity error | Must | critical | corruption fixtures produce zero claims and name the changed field | Test | FR-7 |
| UB-4 | a default or repository mapping is missing, malformed, unqualified, or weakening | refuse the effective policy before any affected dispatch | named policy error with harness, role, and profile | Must | critical | each negative policy fixture fails closed with zero invocation | Test | FR-8, FR-11, FR-12 |
| UB-5 | review demand would fall below the planned assurance floor | refuse review completion and require a correctly joined re-invocation | proposal remains unreviewed; floor violation named | Must | critical | every lowering fixture produces no accepted review evidence | Test | FR-13, FR-14 |
| UB-6 | no native profile satisfies the task | stop before claim/model invocation without trying another harness | named unsupported-profile result and unmet components | Must | critical | FR-10 negative suite proves zero side effects | Test | FR-10, NR-1 |
| UB-7 | routing telemetry cannot connect a selection or outcome to its frozen task/proposal decision | refuse to treat the attempt as acceptance or calibration evidence and surface an audit-integrity failure | attempt is visibly non-evidentiary | Must | high | orphaned-event fixtures never count toward M2–M5 or release | Test + Analysis | FR-24 |
| UB-8 | a frozen v1 rung is unknown to the applicable compatibility table | fail before claim and name the plan version and unsupported value | legacy compatibility error; plan unchanged | Must | high | unknown-rung fixtures perform zero claim/invocation and zero rewrite | Test | FR-15, NR-3 |
| UB-9 | an effective mapping or native-profile qualification is found defective after production activation | engage the strongest-profile kill switch, stop new affected claims, invalidate every affected in-flight or unaccepted evidence artifact, and require re-execution or re-review under a satisfying profile before acceptance | exact affected policy/profile versions, attempts, tasks, and proposals marked `rerun_required` | Must | critical | a seeded post-cutover qualification defect prevents acceptance of every affected open attempt until replacement evidence passes; unaffected routes continue | Failure injection | FR-18, FR-20, FR-24 |
| UB-10 | defective routing evidence has already authorized an accepted or merged proposal | preserve the shipped state, emit an operator decision brief listing every exact affected artifact, and require an explicit incident/remediation disposition | no automatic destructive rollback; exact accepted artifacts are named for human disposition | Must | critical | a seeded late discovery produces one complete brief and never silently retains a valid-routing claim or mutates accepted code | Failure injection + Inspection | FR-21, FR-24, NR-1 |

### op: supersede §5.3 Permissions and abuse cases

Reason: the adapter-bypass abuse control must cite the live routing-evidence requirement after FR-16's supersession.

| Function / data | Planner | Repository operator | Worker/reviewer | Unauthenticated |
|---|---|---|---|---|
| Write task assessment and evidence before freeze | write | review | none | none |
| Raise assurance before freeze | write with evidence | review | none | none |
| Configure repository native mappings | none | write | read effective decision | none |
| Resolve a frozen profile | none | inspect | execute through adapter only | none |
| Read routing/outcome evidence | read | read | read own attempt context | none |

| ID | Actor and intent | Vector | Impact | Severity | Mitigated by | Verify |
|---|---|---|---|---|---|---|
| ABUSE-1 | repository contributor seeking cheaper execution | labels an underpowered native profile as satisfying stronger demand | high-consequence work runs below its floor | critical | FR-12, EVAL-2, reviewed qualification evidence | negative config tests + policy review |
| ABUSE-2 | compromised or mistaken planner minimizing scrutiny | supplies low consequence values or attempts to lower derived assurance | implementation/review under-routed | critical | FR-4, FR-5, FR-6, EVAL-1, human plan checkpoint | eval + tamper tests + plan review |
| ABUSE-3 | adapter or tracker bypassing the contract | discards fields, substitutes settings, or reports a false decision | unaudited or underpowered worker launch | critical | FR-7, FR-24, UB-3, conformance suite | boundary and launch-capture tests |
| ABUSE-4 | operator override containing unrecognized or injected launch data | exploits permissive parsing to add unsupported settings | arbitrary CLI behavior or egress change | high | FR-12 and closed-schema parsing | unknown-key and argument-injection tests |

### op: supersede §5.4 Data

Reason: the plan data contract is complete on every `/implement-now` plan/v2 task and no longer admits a profile-free task variant.

| Entity / field | Meaning | Source of truth | Retention | Integrity rules | Privacy class |
|---|---|---|---|---|---|
| Task assessment | closed workload and consequence values plus resolvable evidence citations | frozen plan ledger | with the plan | complete on every plan/v2 task projected to `/implement-now`; immutable after freeze | internal |
| Execution profile | vendor-neutral minimum tuple and derivation-policy version | mechanically validated frozen plan | with the plan | present for every `/implement-now` plan/v2 task; reproducible from assessment; no native product identifiers | internal |
| Native routing policy | qualified harness×role mappings and optional repository overrides | released defaults plus validated repo policy | current policy plus normal repository history | closed schema; version compatible; no weakening | internal |
| Routing decision | exact/promoted/unsupported resolution and native profile identity where resolved | immutable git event record; runtime event stream is its transport | forever in repository history | connected to frozen plan/v2 demand or typed plan/v1 translation, policy, harness, and role; a prior record is never overwritten | internal |
| Routing outcome | completion, verification/review result, and usage measurements already exposed by the harness | immutable git event record connected to the routing decision | forever in repository history | exactly one terminal outcome per resolved route; never used if orphaned or invalidated | internal |

- **Migration** — No frozen artifact is rewritten. Plan/v2 adopts the required model-backed demand contract for every task projected to `/implement-now`; frozen v1 reads through FR-15's versioned translator and emits FR-24's typed legacy-origin record.
- **Parallel-run / cutover** — No production parallel-run, shadow, or canary path exists. Conformance and dogfood run before the one direct cutover.
- **Rollback** — The strongest-profile kill switch changes native resolution only; semantic plan data and emitted decisions remain readable. UB-9 invalidates affected unaccepted evidence, and UB-10 routes already accepted work to explicit human disposition rather than destructive rollback.
- **Decommission** — The hardcoded implementation pin, incompatible numeric ladder, and implicit review launch settings are removed only after the direct-cutover gate and fallback drill pass.
- **Privacy check** — No DPIA trigger: the feature adds operational classification and routing metadata, not personal or sensitive-data profiling.
- **Egress** — The already-selected harness remains the only model boundary. This feature changes its native launch profile but adds no provider, no cross-harness fallback, and no new source-content egress; routing telemetry records metadata rather than prompt or code content.

### op: supersede §5.5 Interfaces and integrations

Reason: the planner interface now requires the complete assessment/profile contract for every `/implement-now` plan/v2 task.

| Interface | Direction | Purpose | Contract | Failure modes | Owner |
|---|---|---|---|---|---|
| Planner → frozen plan | out | publish assessment and derived demand for every `/implement-now` task | versioned closed plan schema; complete assessment and profile | missing/unknown values, derivation mismatch | plan-new |
| Frozen plan → tracker/executor | out | preserve demand to claim and dispatch | lossless versioned projection/read | field loss, legacy-version mismatch | plan projection + tracker adapters |
| Executor/reviewer → harness adapter | out | resolve semantic minimum for a selected harness and role | qualified effective mapping; exact/promoted/unsupported result | missing cell, incompatible override, no satisfying profile | runtime adapters |
| Merge lane → review routing | in | supply actual proposal skip/standard/deep band | monotone join with planned assurance | missing band, attempted lowering | impl-review + merge lane |
| Runtime/review → evidence stream | out | connect route, promotion/failure, and outcome | typed events joined by task/proposal and attempt identity | orphan, partial write, schema mismatch | execution/review runtime |

### op: supersede §5.7 Instrumentation — the tracking plan

Reason: validation and freeze events now cover every `/implement-now` plan/v2 task rather than excluding a deterministic class.

| ID | Event name | Fires when | Properties (typed) | Source | Feeds |
|---|---|---|---|---|---|
| EVT-1 | `execution_demand_frozen` | an `/implement-now` plan/v2 task passes plan validation | `demand_id:string`, `task_id:string`, `plan_version:string`, `assessment:closed object`, `assessment_evidence_refs:string[]`, `profile:closed object`, `policy_version:string`, `assurance_raise:boolean` | plan gate | M2, M5 |
| EVT-2 | `routing_resolved` | an implementation or review route resolves | `attempt_id:string`, `demand_id:string`, `task_id:string`, `proposal_id:string?`, `harness:string`, `role:implementation\|review`, `requested_profile:closed object`, `proposal_band:skip\|standard\|deep\|null`, `assurance_join_raise:boolean`, `native_profile_id:string`, `resolution:exact|promoted`, `promoted_components:enum[]`, `policy_version:string` | runtime adapter | M2, M4, M5 |
| EVT-3 | `routing_resolution_failed` | routing stops before claim/invocation | `attempt_id:string`, `demand_id:string`, `task_id:string`, `proposal_id:string?`, `harness:string`, `role:implementation\|review`, `requested_profile:closed object`, `proposal_band:skip\|standard\|deep\|null`, `assurance_join_raise:boolean`, `policy_version:string`, `unmet_components:enum[]`, `reason:enum` | executor/reviewer | M2, M5 + health alert |
| EVT-4 | `routing_conformance_checked` | one required routing-domain cell finishes conformance | `harness:string`, `role:enum`, `semantic_profile_id:string`, `expected:resolved|unsupported`, `result:pass|fail`, `evidence_ref:string` | conformance harness | M3 |
| EVT-5 | `routed_attempt_completed` | implementation verification or independent review completes | `attempt_id:string`, `task_id:string`, `proposal_id:string?`, `outcome:verified|failed|blocked`, `usage:closed object`, `evidence_ref:string` | executor/reviewer | M4, M5 |
| EVT-6 | `execution_demand_checked` | every plan/v2 task presented for `/implement-now` reaches validation, before acceptance or rejection | `task_id:string`, `plan_version:string`, `result:pass|fail`, `violations:enum[]` | plan gate | M1 |
| EVT-7 | `routing_evidence_invalidated` | UB-9 or UB-10 identifies evidence produced under a defective policy/profile | `policy_version:string`, `native_profile_id:string`, `attempt_ids:string[]`, `artifact_refs:string[]`, `disposition:rerun_required|operator_remediation` | execution/review evidence owner | health alert + UB-9/UB-10 audit |
| EVT-8 | `legacy_execution_demand_translated` | a frozen plan/v1 task is translated for `/implement-now` without rewrite | `demand_id:string`, `task_id:string`, `plan_version:string`, `rung_present:boolean`, `original_rung:string?`, `compatibility_version:string`, `translation:explicit_rung\|legacy_unassessed`, `translated_profile:closed object`, `policy_version:string` | legacy translator | M2, M5 |

### op: supersede §5.8 Definition of Done

Reason: completion evidence removes the nonexistent deterministic-task suite, cites the live replacement requirements, and protects real empty-frontier behavior.

| ID | Done criterion | Source | Verify | Required evidence | Blocking? |
|---|---|---|---|---|---|
| DOD-1 | Every Must functional and failure requirement passes its own fit criterion. | FR-3..FR-15, FR-17..FR-18, FR-20..FR-24, UB-2..UB-11 | Test + Eval + Demo + Inspection + Analysis + Failure injection | per-requirement evidence index linked from the proposal | yes |
| DOD-2 | No protected planner, harness-selection, merge-lane, legacy-plan, empty-frontier, independent-review, or config-authorization behavior regresses. | NR-1..NR-4, NR-6..NR-8 | Test + Inspection | non-regression suite and boundary inspection | yes |
| DOD-3 | Every required harness/role/derivable-profile cell passes conformance and qualification review. | M3, FR-8, FR-11, FR-12, EVAL-2 | Test + Human judgment — @mdarabi; rubric: every required cell has the expected satisfying or unsupported result, passing launch/failure capture, and current qualification evidence | conformance matrix and operator sign-off | yes |
| DOD-4 | Pre-release dogfood exercises at least three distinct native profiles with verified outcomes and complete evidence chains. | M4, M5, FR-24 | Analysis | joined routing/outcome report | yes |
| DOD-5 | Planner assessment and derivation evals pass, with zero under-assessed safety floors in the adversarial set. | EVAL-1, FR-3, FR-5, FR-22 | Eval | versioned eval report | yes |
| DOD-6 | Every §3.3 guardrail is clean before activation. | §3.3 | Analysis | pre-release decision report | yes |
| DOD-7 | The direct-cutover configuration and strongest-profile kill switch pass rehearsal without a harness switch, review-floor loss, or any semantic downgrade; an unsatisfiable fallback still fails before claim. | FR-10, FR-17, FR-18, NR-1, NR-2 | Test + Demo | cutover and fallback drill record | yes |
| DOD-8 | Routing diagnostics and immutable git evidence are complete, connected, payload-free, and usable by the operator to explain one exact, one promoted, one unsupported, one legacy-origin, and one review-join case. | FR-20, FR-21, FR-24, NFR-4, EVT-1..EVT-8 | Demo + Inspection | five-case audit walkthrough plus committed event records | yes |
| DOD-9 | A post-cutover qualification-defect drill invalidates every affected open artifact, preserves unaffected work, and produces an operator brief for already accepted artifacts. | UB-9, UB-10, §3.3 | Failure injection + Inspection | defect-disposition drill record | yes |

### op: supersede §5.9 AI behavior and evals

Reason: the planner eval no longer includes or permits a task-level deterministic execution mode.

| ID | Behavior | Eval set / harness | Threshold | Negative examples |
|---|---|---|---|---|
| EVAL-1 | The planner assigns values from the normative assessment vocabulary, cites evidence that substantively supports every recorded value, never emits a task-level `no_inference` mode, and never produces a semantic profile below deterministic policy floors. | versioned clear-case and adversarial task corpus covering every accepted assessment/profile value, consequence floor, named promotion, planner raise, and mechanical-looking agent task; support rubric: every axis has explicit support in one or more cited anchors, one anchor may support multiple axes, and any contradiction or unsupported axis fails | 100% structurally valid; 100% exact on unambiguous golden cases; zero unresolved, irrelevant, contradictory, or partially supporting citation sets; zero `no_inference` outputs; zero consequence/assurance under-floor outcomes | simple-looking system-core change; mechanical-looking agent task; attempted `no_inference` output; first-of-kind contained change; cross-boundary composed verification; attempted assurance reduction; missing or irrelevant evidence; contradictory evidence; partially supporting citation set; unknown enum |
| EVAL-2 | Each required routing-domain cell produces a qualified satisfying native profile or the explicit unsupported result appropriate to that cell. | harness adapter conformance corpus with representative implementation and independent-review tasks plus launch/failure capture | every required cell produces its specified result; every resolved cell produces a verified outcome; zero harnesses or derivable profiles are omitted; zero cells qualify by self-declaration alone | omitted harness; omitted derivable profile; mapping to an unavailable model; effort omitted; review reuses implementation setting; cheaper profile fails claimed demand; unreviewed new model |

### op: supersede §6.1 Phasing and milestones

Reason: Phase A acceptance now cites the live universal-demand requirements and has no deterministic-task branch.

| Phase | Purpose | Scope | Exit criterion |
|---|---|---|---|
| A — contract | make demand explicit and mechanically trustworthy | plan/v2 vocabulary, evidence-backed assessment, derivation, validation, transport, legacy translation | FR-3..FR-7, FR-15, FR-22, and FR-23 green; EVAL-1 green |
| B — resolution | complete implementation and review consumption | required-domain mappings, overrides, resolver, review join, failure paths, immutable evidence | FR-8..FR-14, FR-20, FR-21, FR-24 and adapter conformance green |
| C — pre-release proof | falsify under-routing before any production activation | full non-regression, failure/abuse suites, dogfood diversity, operator audit, kill-switch rehearsal | M1–M5 and every §3.3 guardrail green; DOD-1..DOD-9 evidenced |
| D — production | replace hardcoded/implicit routing in one cutover | every supported implementation and review path | direct activation succeeds; production events remain connected and guardrails clean |

### op: supersede §7.3 Decision log

Reason: record the operator's design-brainstorm ruling that removed the task-level deterministic arm before technical design.

| Date | Decision | Alternatives considered | Why | Who |
|---|---|---|---|---|
| 2026-07-25 | Complete the routing contract before adaptive learning. (`brainstorm:planner-driven-model-routing#D-1`) | full adaptive routing now; runtime resolver only | smallest independently useful foundation and trustworthy data first | @mdarabi |
| 2026-07-25 | Freeze raw dimensions and a checked derived profile. (`brainstorm:planner-driven-model-routing#D-2`) | class only; raw only; derived only | preserve why and minimum demand without dispatch re-judgment | @mdarabi |
| 2026-07-25 | Keep impact scope and architectural role distinct. (`brainstorm:planner-driven-model-routing#D-3`) | reuse blast radius/foundation; one consequence axis | affected reach and architectural centrality differ and existing terms are occupied | @mdarabi |
| 2026-07-25 | Retain all workload axes plus both consequence modifiers. (`brainstorm:planner-driven-model-routing#D-4`) | selected subset; scalar complexity | each carries independent routing information | @mdarabi |
| 2026-07-25 | Require the complete assessment on every model-backed plan/v2 task. (`brainstorm:planner-driven-model-routing#D-5`) | optional routing/defaults | omissions recreate accidental cheapest or hardcoded behavior | @mdarabi |
| 2026-07-25 | Mechanically derive assurance; planner may only raise with evidence. (`brainstorm:planner-driven-model-routing#D-6`) | planner-scored risk; fixed risk only | prevents optimistic lowering while retaining richest-context caution | @mdarabi |
| 2026-07-25 | Derive independent capability, deliberation, context, and assurance requirements. (`brainstorm:planner-driven-model-routing#D-7`) | one scalar rung | the capacities do not rise together | @mdarabi |
| 2026-07-25 | Aggregate by component floors plus named promotions. (`brainstorm:planner-driven-model-routing#D-8`) | weighted score | no input can be averaged away; interactions remain explicit | @mdarabi |
| 2026-07-25 | Put layered, role-specific native mappings in harness adapters. (`brainstorm:planner-driven-model-routing#D-9`) | release-only map; repository-only map; external CLI interpretation | portable defaults plus controlled local policy at the correct boundary | @mdarabi |
| 2026-07-25 | Select least-cost satisfying; promote visibly or fail closed. (`brainstorm:planner-driven-model-routing#D-10`) | clamp down; silently switch harness | the profile is a minimum contract and harness choice has external consequences | @mdarabi |
| 2026-07-25 | Join planned assurance with the actual proposal band monotonically. (`brainstorm:planner-driven-model-routing#D-11`) | merge-lane only; new review risk model | preserve consequence without duplicating actual-diff classification | @mdarabi |
| 2026-07-25 | Serve the model-backed machine path first and operator audit second. (`brainstorm:planner-driven-model-routing#D-12`) | adaptive/fallback consumers in v1 | focuses the direct, repeated job while preserving control | @mdarabi |
| 2026-07-25 | Cover planner classification, implementation resolution, review resolution, and operator audit/override. (`brainstorm:planner-driven-model-routing#D-13`) | omit one seam | all four are required to remove hardcoding without losing explainability | @mdarabi |
| 2026-07-25 | Prove contract completeness, adapter conformance, zero under-routing, profile diversity, critical floors, and telemetry—not adaptive optimization. (`brainstorm:planner-driven-model-routing#D-14`) | cost/quality learning as launch claim | acceptance must falsify this substrate's risks | @mdarabi |
| 2026-07-25 | Bound appetite to the complete vertical seam and originally include a shadow rollout. (`brainstorm:planner-driven-model-routing#D-15`) | partial seam; adaptive program | vertical completeness retained; rollout portion superseded by D-17 | @mdarabi |
| 2026-07-25 | Translate unassessed frozen v1 tasks conservatively without rewriting them. (`brainstorm:planner-driven-model-routing#D-16`) | infer from prose; refuse all old plans; rewrite | compatibility without invented judgment or mutation | @mdarabi |
| 2026-07-26 | Test the full seam pre-release, then activate directly in production with no shadow/canary. (`brainstorm:planner-driven-model-routing#D-17`) | shadow/canary; testing in production | explicit operator ruling; fail-closed paths and strongest fallback remain | @mdarabi |
| 2026-07-26 | Every `/implement-now` plan/v2 task is model-backed; remove task-level `no_inference` while preserving empty-frontier zero-token behavior. (`design-brainstorm:planner-driven-model-routing-tech-design#D-5`) | invent a deterministic executor recipe contract; let profile-free tasks reach proposal review | `/implement-now` has no deterministic task execution seam, and proposal review requires a planned semantic profile | @mdarabi |

### op: supersede §8 Contract readiness — the executability gate

Reason: the readiness proof now cites only live requirements and treats every `/implement-now` plan/v2 task as part of M1/EVT-6.

| Check | Pass? | Notes |
|---|---|---|
| Problem basis accepted — §1 problem accepted (T1); §2 problem gate signed (T2/T3) | ☐ | pending introducing PR operator review |
| Zero solution-blocking TBDs or open questions (§7.2) | ☑ | Q1–Q3 are design mechanics with product behavior fixed by cited requirements |
| Every Must FR has a fit criterion and a verification method (§5.1) | ☑ | FR-3..FR-15, FR-17..FR-18, and FR-20..FR-24 carry both |
| Every requirement row carries a risk tier; high/critical rows have a declared safety plan | ☑ | fail-before-claim behavior, pre-release full-domain proof, direct-cutover and unsatisfiable-fallback rehearsal, UB-9/UB-10 post-defect disposition, operator-authorized policy, immutable evidence, no-rewrite migration, and empty-frontier non-regression cover the high/critical surfaces |
| Non-goals drawn (§3.4); non-regression rows named (§5.1) | ☑ | NG-1..NG-8, WR-1..WR-3, NR-1..NR-4 and NR-6..NR-8 |
| Every primary job (§2.3) is covered by a requirement, a non-goal, or a waiting-room entry | ☑ | J1→FR-3..FR-7/FR-22/FR-23; J2→FR-8..FR-12/FR-15/FR-18; J3→FR-13/FR-14/FR-18; J4→FR-12/FR-20/FR-21/FR-23/FR-24/NR-7 |
| Every key flow (§4.3) has functional, failure, and non-regression coverage (§5.1–5.2) | ☑ | F1→FR-3..FR-6/FR-22/FR-23/UB-2/UB-11; F2/F3→FR-7..FR-14/FR-18/FR-20/FR-24/UB-3..UB-6/UB-9..UB-10/NR-1..NR-2/NR-6; F4→FR-12/FR-20..FR-24/UB-4/UB-7/UB-9..UB-10/NR-7; F5→FR-15/FR-24/UB-8/NR-3; empty frontier→NR-8 |
| Every primary metric (§3.2) has instrumentation (§5.7) or an explicit measurement plan | ☑ | M1→EVT-6 (pass ÷ all `/implement-now` plan/v2 candidates); M2→EVT-1/8 joined to EVT-2/3 with non-assurance identity and role-appropriate assurance join checked before native promotion; M3→EVT-4 over the required routing domain; M4→EVT-2/5; M5→EVT-1 or EVT-8 plus terminal EVT-3 or EVT-2+EVT-5 |
| Every blocking DoD row traces to a requirement, guardrail, release criterion, or human-judgment rubric (§5.8) | ☑ | DOD-1..DOD-9 each cite reviewed source rows |
| Risks and assumptions have owners (§6.3) | ☑ | R1..R7 owned |
| Human-judgment criteria name an owner and a rubric (§5.8) | ☑ | DOD-3 names @mdarabi and the three-part qualification rubric |
| DoD complete: every blocking row sourced and evidenced; every n/a explained (§5.8) | ☑ | all DoD rows block and name evidence; §6.6 n/a rows state the absent surface |
