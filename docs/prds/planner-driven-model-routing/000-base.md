# Planner-driven model routing — PRD

| Field | Value |
|---|---|
| **In one line** | Define the planner-frozen execution-demand contract that implementation and review runtimes resolve to qualified, harness-native model and effort settings. |
| **Relevant-when** | planning, building, or reviewing task-demand assessment, plan routing data, implementation or proposal-review model selection, harness-native routing policy, legacy plan compatibility, or routing telemetry and rollout |
| **Status** | Draft — problem review pending |
| **Tier** | T3 — the capability controls factory execution policy, review assurance, production model selection, and fail-closed dispatch |
| **Owner** | @mdarabi |
| **Approver(s)** | @mdarabi — problem, solution, and launch gates |
| **Contributors** | @mdarabi (product + engineering) · factory (synthesis) |
| **Informed** | Pyxis maintainers and repository operators |
| **Date** | 2026-07-26 created · 2026-07-26 last updated |
| **Appetite** | One complete plan-to-runtime routing seam: human attention at the three PRD gates; bounded machine work through contract, design, plan, implementation, independent review, and pre-release conformance; one direct production cutover with a tested strongest-profile fallback. Adaptive optimization and broader policy programs do not fit. |
| **Target release** | The routing-contract increment that replaces coarse or hardcoded worker selection and establishes trustworthy data before adaptive VNEXT-7 work; no calendar target. |
| **Tracking** | — *(the introducing PR, once filed)* |
| **Artifacts** | `brainstorm:planner-driven-model-routing` (gated record) |
| **Template version** | prd-template/v2 |

## 0. Read to rule — the human reviewer's section (~5 min)

*The body is the contract; this section only routes attention — on any conflict the body governs; machines ignore it.*

**Judge these**

| # | The call | Could have gone | If you disagree |
|---|---|---|---|
| 1 | Plans freeze all six assessment dimensions **and** a checked four-part execution profile; no native model identifiers | one scalar complexity tier, raw dimensions only, or native settings in the plan | the planner/runtime boundary changes → §4.1, FR-1..FR-4 |
| 2 | Harness adapters independently map implementation and review demand to the least-cost qualified native profile | one global model map or the external CLI interpreting Pyxis demand | portability, qualification ownership, and role separation change → FR-8..FR-12 |
| 3 | Assurance is mechanically floored, planner-raise-only, and monotonically joined with the actual proposal lane for review | planner-scored risk or merge-lane-only review | critical-task scrutiny can be lowered or a second risk classifier appears → FR-5, FR-6, FR-13 |
| 4 | Missing exact profiles promote upward or fail before work; they never downgrade or silently switch harnesses | clamp down or cross-harness fallback | judge-once routing, credentials, and egress boundaries change → FR-9, FR-10, NR-1 |
| 5 | Tested routing activates directly for every supported production path; there is no shadow or canary stage | staged production exposure | the operator's explicit rollout ruling changes → §6.5, FR-17 |

**Sanity-check the numbers** — 100% complete plan/v2 contracts, zero under-routing, zero critical-floor violations, and at least three distinct native profiles in pre-release dogfood (§3.2–§3.3).

**The author's open questions** — none at product level; Q1–Q3 (§7.2) are design-owned mechanics bounded by requirements.

**Skip the rest** — structure/schema → docgate; substance/coherence → unmanned PRD review; implementation and mapping defects → technical-design, plan, conformance, and independent-implementation-review gates.

## 1. Summary — the one-pager

- **Problem** — Planning can identify that tasks differ, but the execution path neither preserves enough of that judgment nor consistently turns it into implementation and review capacity. Routing data is optional or lost, runtime scales disagree, one worker path is temporarily pinned to a single strongest setting, and proposal review launches without an explicit model or effort.
- **For whom** — Primarily the model-backed planner→implementation→review path on every planned task; secondarily the repository operator who must understand and constrain native model selection.
- **Why now** — The coarse routing seam already exists in the plan and runtime contracts, but its gaps now force hardcoding and silent defaults. Adaptive routing cannot learn safely until the factory can first produce, preserve, resolve, and observe an explicit execution-demand contract.
- **Direction** — The planner records a closed workload-and-consequence assessment and its deterministically derived, vendor-neutral execution profile. The already-selected harness adapter maps that minimum profile to the least-cost qualified implementation or review launch settings; it may promote visibly but may never downgrade or silently change harnesses. Finished-proposal risk can increase review assurance but never erase the planned floor.
- **Appetite** — One vertical routing-contract increment, including legacy translation, role-specific mappings, complete telemetry, conformance, dogfood, and a strongest-profile kill switch; no self-tuning, fallback, general authoring routing, or automatic model qualification.
- **Success** — At the pre-release checkpoint, 100% of non-deterministic plan/v2 candidates carry valid evidence-backed assessments and derived profiles, the full required routing domain conforms, zero dispatches under-route, and dogfood proves at least three distinct native profiles with verified outcomes.
- **Guardrail** — No critical assurance floor is lowered or skipped, and no frozen v1 plan becomes unexecutable.
- **Top risks** — A plausible but wrong planner assessment under-routes work; a stale native mapping overstates a model's qualification; transport or review code silently discards the frozen demand.

## 2. Problem space

### 2.1 Problem statement

The planning system can express only an optional coarse routing hint, while downstream execution paths lose that value, use incompatible scales, or bypass explicit selection entirely. Consequently, tasks with materially different reasoning, context, consequence, and verification needs can receive the same worker setting, and an operator cannot reliably explain why a particular implementation or reviewer was chosen. Leaving the gap in place means the factory must choose between overspending on every task and silently under-powering high-consequence work.

### 2.2 Users and segments

| Segment | Role and context | Domain exp. | Tech exp. | Key goals | Pain today | Evidence |
|---|---|---|---|---|---|---|
| **P1 (primary)** | The model-backed planning, implementation, and proposal-review path for a frozen task | master | master | preserve richest-context judgment; obtain sufficient implementation and review capacity; fail before unsafe work | optional/lost routing, inconsistent scales, hardcoded and implicit launch settings | `brainstorm:planner-driven-model-routing#D-12`, `#F-3`, `#F-7` |
| P2 (secondary) | A repository operator configuring and auditing the factory's native model policy | master | master | control local availability and cost; inspect why promotion or failure occurred; retain harness choice | native choice is embedded in code or absent from review invocation | `brainstorm:planner-driven-model-routing#D-12`, `#D-13` |
| **Anti-segment** | Consumers expecting automatic scorecard learning, exploration, harness selection, or cross-harness fallback in this increment | — | — | — | those jobs require trustworthy routing outcomes first | `brainstorm:planner-driven-model-routing#D-1`, `#D-12`, `#O-1`, `#O-2` |

### 2.3 Jobs and desired outcomes

| # | When… | I want to… | So I can… | Today's workaround | Evidence |
|---|---|---|---|---|---|
| J1 | a planner freezes a model-backed task | describe its workload and consequences once, with evidence | downstream workers consume the same minimum demand without re-judging it | optional scalar hint or no route | `brainstorm:planner-driven-model-routing#D-2`..`#D-8` |
| J2 | an implementation worker is about to launch on the selected harness | resolve semantic demand to a qualified native model and effort | use the least-cost setting that is still sufficient | hardcoded model or mismatched numeric tier | `brainstorm:planner-driven-model-routing#D-9`, `#D-10`, `#F-7` |
| J3 | an independent reviewer is about to inspect the finished proposal | preserve planned assurance and join it with actual proposal risk | review consequence-sensitive work deeply without inventing another risk classifier | review launches without model/effort; depth sees only the proposal lane | `brainstorm:planner-driven-model-routing#D-11`, `#F-6`, `#F-7` |
| J4 | an operator changes or audits routing policy | see the assessment, derived demand, selected profile, promotion, and outcome | control spend and diagnose behavior without editing plans or adapter code | inspect scattered code and worker logs | `brainstorm:planner-driven-model-routing#D-9`, `#D-13`, `#D-14` |

### 2.4 Evidence

| Claim | Source | Confidence |
|---|---|---|
| The existing global policy already separates deterministic, mechanical, authoring, and judgment/adversarial work and keeps product identifiers outside capability names. | `brainstorm:planner-driven-model-routing#F-2` | High |
| The current planner makes routing optional and otherwise distinguishes tasks only mechanically. | `brainstorm:planner-driven-model-routing#F-3` (`prd:plan-new/FR-7`, `FR-14`) | High |
| The current runtime contract intends to resolve a frozen task rung to native model and effort behind the shared runtime port. | `brainstorm:planner-driven-model-routing#F-5` (`prd:implement-now/FR-6`) | High |
| The proposal-review contract already binds depth to the merge lane and forbids a second review-side risk model. | `brainstorm:planner-driven-model-routing#F-6` (`prd:impl-review/FR-9`, `FR-10`) | High |
| Current tracker reads discard projected routing, planner and adapter scales disagree, one implementation adapter is pinned, and review launches without model or effort. | `brainstorm:planner-driven-model-routing#F-7` (code audit) | High |
| The larger adaptive destination already depends on frozen per-task decisions and backend/task outcome evidence. | `brainstorm:planner-driven-model-routing#F-1` (`corpus:VNEXT-7`) | High |

### 2.5 The journey today

| Phase | User/system action | Pain | Observable consequence |
|---|---|---|---|
| Plan | planner may attach a coarse numeric hint or omit it | the hint cannot express independent workload and consequence needs | absence remains valid and intent is ambiguous |
| Project/claim | tracker adapters reconstruct executable work | the frozen hint can be dropped | executor sees no reliable task demand |
| Implement | selected harness launches a worker | scales disagree or code overrides the route | tasks converge on one setting or accidental defaults |
| Review | proposal lane chooses skip/standard/deep | model and effort are not explicitly resolved | review depth and worker capacity are disconnected |
| Audit | operator investigates cost or failure | no single decision record connects plan to outcome | selection cannot be calibrated or defended |

### 2.6 Opportunity and why now

- **Opportunity** — Close an already-declared planner/runtime seam so each inference-bearing task spends only the capacity it needs while high-consequence work cannot be averaged down.
- **Why now** — The factory is entering execution work with a temporary strongest-model pin and known transport defects; multiplying tasks multiplies either unnecessary cost or under-routing risk.
- **Strategic fit** — Vendor-neutral semantic demand preserves the common core while adapter-owned native mappings honor the no-privileged-substrate promise.
- **Cost of doing nothing** — Hardcoding persists, independent review remains only partially routed, and later adaptive policy has no trustworthy decision/outcome corpus.

**Decision:** Problem gate — do not proceed to §3–§4 until signed.

| Reviewer | Role | Date | Verdict |
|---|---|---|---|
| @mdarabi | operator | — | — *(via the introducing PR's operator review)* |

## 3. Goals, metrics, and guardrails

### 3.1 Goals as falsifiable hypotheses

| # | We believe that… | Will result in… | Measured by… |
|---|---|---|---|
| G1 | requiring a complete, checked execution-demand contract on every model-backed task | plan intent survives unchanged to implementation and review | M1, M2 |
| G2 | resolving vendor-neutral demand through qualified role-specific adapter policy | tasks use diverse native profiles without under-routing | M2, M3, M4 |
| G3 | recording each derivation, resolution, promotion, and outcome | routing becomes explainable and supplies trustworthy successor evidence | M5 |

### 3.2 Goals → signals → metrics

| Metric | Goal | Signal | Baseline | Target (at checkpoint) | Feeds from |
|---|---|---|---|---|---|
| M1 (primary) | G1 | valid model-backed demand contracts ÷ every plan/v2 task presented for freeze that is not explicitly deterministic | contract absent | 100% at pre-release acceptance | EVT-6 |
| M2 | G1, G2 | dispatch decisions that exactly satisfy or visibly promote every profile component | not measurable; current paths can lose or override routing | 100% satisfying; zero downward resolution in conformance and dogfood | EVT-2, EVT-3 |
| M3 | G2 | required harness × role × derivable-profile cells producing their specified satisfying or unsupported result under adapter conformance | no common conformance proof | 100% of the required routing domain before release | EVT-4 |
| M4 | G2 | distinct native launch profiles exercised by verified dogfood tasks | implementation can converge on one pin | at least three before production activation | EVT-2, EVT-5 |
| M5 | G3 | terminal routing attempts with a complete evidence chain: frozen demand→unsupported result, or frozen demand→resolved route→worker outcome | incomplete and fragmented | 100% at pre-release acceptance and in production | EVT-1, EVT-2, EVT-3, EVT-5 |

### 3.3 Guardrails and decision rule

| Guardrail metric | Threshold | Action if breached |
|---|---|---|
| Downward resolution of any execution-profile component | zero | block release; after activation, engage the strongest-profile kill switch, stop new routed claims, and disposition affected evidence under UB-9/UB-10 |
| Critical assurance lowered or review skipped | zero | block/rollback immediately; invalidate affected review evidence and re-review |
| Frozen v1 plan made unexecutable or rewritten | zero | block release; restore compatibility translation before proceeding |
| Routing-selected task fails verification because its launch profile was under-qualified | zero in required conformance and pre-release dogfood | block activation; after activation quarantine the mapping, use the strongest-profile fallback, and disposition affected evidence under UB-9/UB-10 |

**Ship rule (pre-registered):** M1–M5 meet their targets, every guardrail is clean, and §6.4 is green → activate routing directly in production. Any guardrail breach → no activation, or immediate strongest-profile fallback after activation.

### 3.4 Non-goals and waiting room

| ID | Non-goal | Why refused |
|---|---|---|
| NG-1 | Scorecard-driven policy changes, self-tuning, or exploration traffic | trustworthy decision/outcome data must exist first; successor VNEXT-7 scope (`brainstorm:planner-driven-model-routing#O-1`) |
| NG-2 | Automatic harness selection or cross-harness fallback | changes credentials and egress beyond the already-selected harness (`#O-2`) |
| NG-3 | Routing general authoring-stage skills | this increment closes plan-backed implementation and proposal review (`#O-3`) |
| NG-4 | A unified expanded-risk or policy engine | VNEXT-9/VNEXT-10 remain independently revisable (`#O-4`) |
| NG-5 | Automatically qualifying newly released models | native profiles require reviewed policy and conformance evidence (`#O-5`) |
| NG-6 | Native model identifiers or effort settings in frozen plans | native values remain adapter data (`#O-6`) |
| NG-7 | Shadow or canary production rollout | the operator chose test-then-direct production activation (`#O-7`) |

| ID | Waiting room | Revisit when / intended version |
|---|---|---|
| WR-1 | Backend×task scorecards, guarded policy learning, and exploration | successor VNEXT-7 increment after M5 supplies trustworthy outcomes |
| WR-2 | Cross-harness fallback and automatic harness choice | successor policy explicitly governs credentials, egress, availability, and fallback |
| WR-3 | Broader authoring-stage routing | plan-backed execution and review have shipped and their semantic vocabulary proves reusable |

## 4. Solution space

### 4.1 Approach and alternatives considered

The chosen approach freezes two related but distinct artifacts on every model-backed plan/v2 task: a closed six-axis assessment that explains the work, and a deterministically derived four-part execution profile that states the minimum capability, deliberation, context, and assurance required. The planner supplies evidence and may raise assurance; a mechanical plan gate recomputes policy output and rejects disagreement. The selected harness adapter then resolves the semantic minimum through separate qualified implementation and review mappings.

| Option | Sketch | Why not chosen |
|---|---|---|
| A (chosen) | raw assessment + checked semantic profile; adapter-owned native resolution | preserves explainability, judge-once semantics, and harness portability |
| B | one scalar complexity tier | collapses independent reasoning, context, consequence, and assurance needs |
| C | raw dimensions only; executor interprets them | moves planning judgment to dispatch and permits adapters to disagree about task meaning |
| D | native model and effort frozen in each plan | couples durable plans to volatile vendor catalogs and one harness |
| E | full adaptive routing now | combines the routing substrate with an optimizer before reliable outcomes exist |

### 4.2 Scope perimeter

- **In (plan of record)** — closed evidence-backed task assessment; deterministic semantic-profile derivation; assurance floor and planner raise; plan validation and transport; the full baseline harness/role/profile domain; role-specific adapter mappings with closed operator-authorized repository overrides; least-cost satisfying resolution, promotion, and named unsupported failure; independent-review join; frozen v1 translation; immutable routing/outcome evidence; conformance and model-behavior evals; direct production activation after the pre-release gate; strongest-profile kill switch; post-defect evidence invalidation and operator remediation brief.
- **Future** — adaptive scorecards and calibration; harness selection/fallback; broader authoring routing.
- **Out** — NG-1..NG-7.

### 4.3 Experience: flows and states

| Flow | Happy path | State transitions | Failure coverage |
|---|---|---|---|
| F1 — plan and freeze | planner records all workload and consequence dimensions with evidence; policy derives a matching profile | unassessed → assessed → validated/frozen | UB-1, UB-2 |
| F2 — implement | selected harness resolves the frozen profile to the least-cost qualified implementation setting and launches | ready → resolved → claimed → completed | UB-3, UB-4, UB-6 |
| F3 — review | merge-lane band joins the planned assurance floor; review mapping resolves the joined demand independently | proposal-ready → joined → resolved → reviewed | UB-3, UB-5, UB-6 |
| F4 — audit and override | operator inspects the full decision chain or supplies a closed, qualified repository override | default-policy → override-validated → effective-policy | UB-4, UB-7 |
| F5 — legacy execution | frozen v1 routing translates through the versioned compatibility table without rewriting the plan | legacy-read → translated → resolved | UB-8 |

### 4.4 Scenarios

- **Novel but contained implementation.** A task requires first-of-kind reasoning and adversarial verification while touching a feature-local surface. Its profile demands strong capability and deep deliberation without falsely declaring system-wide consequence; the selected harness resolves it to its own qualified settings.
- **Simple change on system core.** A mechanically straightforward edit targets a system-core policy surface. Consequence floors raise assurance and the named interaction rules prevent a cheap cognitive classification from lowering scrutiny.
- **Review after a riskier-than-planned diff.** Planned assurance is elevated, but the finished proposal enters the deep merge lane. The monotone join raises review demand; the review-role mapping selects independently from the implementation worker.
- **No exact native profile.** A harness cannot meet the tuple exactly but has a stronger qualified profile. Resolution promotes and records the reason. If no satisfying profile exists, execution stops before claim or model invocation and names the unmet component.
- **Frozen v1 plan with no rung.** The reader classifies it as legacy-unassessed and applies the conservative compatibility profile without modifying the plan artifact.

**Decision:** Solution gate — this shape at this appetite is the bet. Zero product-level TBDs may pass.

| Reviewer | Role | Date | Verdict |
|---|---|---|---|
| @mdarabi | operator | — | — *(via the introducing PR's operator review)* |

## 5. System requirements

### 5.1 Functional requirements

**Normative assessment vocabulary**

| Axis | Closed values and meanings |
|---|---|
| `reasoning` | `apply` — execute one established pattern; `compose` — combine established patterns and resolve their tradeoffs; `solve` — derive a solution where no established pattern decides the task |
| `coupling` | `local` — one cohesive component with no coordinated contract change; `multi_component` — coordinated changes across components within one system boundary; `cross_boundary` — a change crosses a process, trust, repository, or external-integration boundary |
| `novelty` | `known` — directly covered by governing precedent; `adapt` — precedent exists but must change for new constraints; `first_of_kind` — no governing implementation precedent exists in the repository |
| `verification` | `direct` — one deterministic observation decides the behavior; `composed` — multiple layers or states must jointly prove it; `adversarial` — negative, abuse, fault, or invariant-challenge evidence is required |
| `impact_scope` | `contained` — failure is confined to the task's feature surface; `multi_surface` — failure can affect multiple features, consumers, or interfaces; `system_wide` — failure can alter factory-wide behavior or policy |
| `architectural_role` | `feature_local` — implementation owned by one feature; `shared_substrate` — reused infrastructure serving multiple features; `system_core` — control-plane, governance, or globally load-bearing behavior |

**Normative execution-profile vocabulary**

| Component | Closed values and meanings |
|---|---|
| `capability` | `economy` — established, bounded application; `standard` — ordinary composition and adaptation; `strong` — difficult ambiguity, novelty, or cross-component synthesis; `frontier` — the maximum semantic capability requirement for first-of-kind, system-core, or otherwise highest-judgment work |
| `deliberation` | `light` — short application path; `standard` — bounded multi-step reasoning; `deep` — sustained comparison, synthesis, or critique; `maximum` — the maximum semantic reasoning depth for problems requiring exhaustive search or adversarial synthesis |
| `context` | `standard` — the task charter and its direct dependencies; `large` — multiple relevant components or artifacts; `extended` — system-wide contract, architecture, implementation, and evidence context needed for the broadest task |
| `assurance` | `ordinary` — review may follow the actual proposal lane; `elevated` — review is at least standard; `critical` — review is deep and cannot be skipped |

The **required routing domain** is every harness in the existing `AgentRuntime` contract—Claude Code, Codex, Cursor CLI, and OpenCode—crossed with implementation and review roles and every execution profile the derivation policy can emit. A later harness added to that runtime contract joins this domain automatically. Each cell must resolve to a qualified satisfying native profile or to the explicit unsupported result required by FR-10; a harness or derivable profile cannot disappear from conformance by omission.

| ID | Requirement (EARS) | Rationale | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|
| FR-1 | For every model-backed plan/v2 task, the planner shall record `reasoning`, `coupling`, `novelty`, `verification`, `impact_scope`, and `architectural_role` using the normative assessment vocabulary. | complete inputs prevent absent/default routing | Must | critical | every model-backed golden plan carries exactly one accepted value for every field; missing and unknown values fail validation | Test + Eval (EVAL-1) | G1, J1, F1 |
| FR-2 | For every deterministic plan/v2 task, the planner shall record an explicit no-inference execution mode. | deterministic work should spend no model inference and must not masquerade as missing routing | Must | high | deterministic fixtures validate without an execution profile and never enter model resolution | Test | G1, J1, F1 |
| FR-3 | When a task assessment is complete, the planning system shall derive `capability`, `deliberation`, `context`, and `assurance` minimums from the normative execution-profile vocabulary by a versioned max-of-floors policy plus named interaction promotions. | independent requirements avoid false scalar precision | Must | critical | every derivation golden produces only accepted values plus the applicable policy version; no input is reduced by another input | Test + Eval (EVAL-1) | G1, J1, F1 |
| FR-4 | When a plan is validated, the plan gate shall recompute the execution profile and reject any frozen value that differs from the applicable policy result. | dispatch must not trust unchecked planner arithmetic | Must | critical | each tampered-profile fixture is rejected by name before freeze | Test | G1, J1, F1 |
| FR-5 | When closed consequence or risk-policy signals require scrutiny, the planning system shall derive an `ordinary`, `elevated`, or `critical` assurance floor. | simple work on a dangerous surface must not route cheaply | Must | critical | every consequence/risk golden meets its declared floor; no cognitive axis lowers it | Test + Eval (EVAL-1) | G1, J1, F1 |
| FR-6 | When the planner raises a derived assurance floor, the plan shall preserve the raised value and a non-empty evidence citation. | richest-context judgment may add safety but not remove it | Must | high | a justified raise validates; an attempted reduction or uncited raise is rejected | Test | G1, J1, F1 |
| FR-7 | When a frozen task crosses planning, projection, tracker, claim, implementation, or review boundaries, the system shall preserve its assessment, assessment-evidence citations, derived profile, policy version, and planner-raise evidence without reinterpretation. | current tracker reads lose routing data, and uncoupled evidence would make the judgment unauditable | Must | critical | boundary round-trip fixtures are field-identical for every supported tracker path, including every evidence reference | Test | G1, J1, J4, F1, F2, F3, F4 |
| FR-8 | When a selected harness and worker role receive an execution profile, the harness adapter shall choose the least-cost qualified native profile that satisfies every semantic minimum. | native catalogs differ while task meaning must remain stable | Must | critical | every declared mapping cell resolves to the lowest-cost qualifying profile and passes role conformance | Test + Analysis | G2, J2, J3, F2, F3 |
| FR-9 | If no exact native profile exists but a stronger qualified profile satisfies every minimum, the harness adapter shall select the stronger profile and record each promoted component. | visible overprovision preserves correctness | Must | high | promotion fixtures select only a dominating profile and emit EVT-2 with the promoted components | Test | G2, J2, F2, F3 |
| FR-10 | If the selected harness has no qualified profile satisfying every minimum, the execution engine shall fail before task claim or model invocation with a named unsupported-profile result. | under-routing and silent backend switches violate the frozen contract | Must | critical | negative fixtures perform zero claims and zero invocations and name each unmet minimum | Test | G2, J2, F2, F3 |
| FR-11 | The routing policy shall specify an independent implementation-role and review-role result for every cell in the required routing domain. | review must not inherit implementation capacity accidentally, and support cannot shrink by declaration | Must | critical | conformance enumerates the required domain from the runtime contract and derivation policy; every cell has a qualified satisfying mapping or the FR-10 unsupported result; no required harness or profile is omitted | Test + Inspection | G2, J2, J3, F2, F3 |
| FR-12 | Where a repository supplies routing overrides, the system shall accept only closed, version-compatible mappings whose native profiles carry reviewed qualification evidence. | local availability and cost policy must not weaken minimums | Must | critical | valid overrides replace defaults; unknown keys, incompatible versions, missing evidence, and weakening entries are rejected before dispatch | Test + Inspection | G2, J4, F4 |
| FR-13 | When proposal review demand is computed, the review path shall monotonically join the planned assurance floor with the merge lane's actual proposal band so ordinary permits skip, elevated requires at least standard, and critical requires deep review. | preserve consequence while keeping one actual-diff classifier | Must | critical | the complete floor×band table returns the greater review demand; zero cells lower planned assurance | Test | G1, G2, J3, F3 |
| FR-14 | When review is required, the review runtime shall resolve the joined demand through the selected harness's review-role mapping independently of the implementation selection. | fresh independent review needs its own qualified capacity | Must | critical | fixtures with differing role mappings launch the declared review profile and never reuse an implementation decision implicitly | Test | G2, J3, F3 |
| FR-15 | When a frozen v1 task is read, the system shall translate its explicit rung through a versioned compatibility table or classify an absent rung as `legacy_unassessed` with frontier capability, maximum deliberation, extended context, and an assurance floor that existing risk signals may raise. | old plans allowed no rung and cannot be rewritten | Must | high | rung fixtures translate exactly; absent-rung fixtures receive the conservative profile; source bytes remain unchanged | Test | G1, J2, F5 |
| FR-16 | For every implementation and review routing attempt, the system shall record the frozen assessment, derived profile, policy version, selected harness and role, connected task/proposal identity, and an `exact`, `promoted`, or `unsupported` resolution; a resolved route shall also record its native profile identity. | operators and successor routing need a complete routing decision | Must | high | every routing attempt joins EVT-1 to exactly one EVT-2 or EVT-3; every EVT-2 has a native profile and every EVT-3 has none | Test + Analysis | G3, J4, F2, F3, F4 |
| FR-17 | When all pre-release criteria are green, the release shall activate planner-driven routing for every supported production implementation and review path without a shadow or canary stage. | direct cutover is the operator-approved rollout | Must | critical | one production configuration enables the validated policy for the full declared support matrix; no staged cohort logic exists | Inspection + Demo | G1, G2, J2, J3 |
| FR-18 | Where the strongest-profile kill switch is engaged, each harness role shall prefer its strongest approved native profile only when that profile satisfies every requested semantic minimum; otherwise the system shall apply FR-10 before claim or invocation. | fallback may overprovide but can never weaken the frozen contract | Must | critical | kill-switch drills prove capability, deliberation, context, and assurance dominance for every resolved cell; unsatisfiable fixtures fail under FR-10; harness selection and FR-13 remain unchanged | Test + Demo | G2, J2, J3, F2, F3 |
| FR-19 | For every model-backed plan/v2 task, the planner shall record at least one resolvable evidence citation supporting its assessment values. | the assessment must preserve why the task was classified, not only its labels | Must | high | a cited assessment validates and round-trips; an uncited assessment or unresolved citation fails before freeze | Test + Eval (EVAL-1) | G1, J1, J4, F1, F4 |
| FR-20 | When a resolved implementation or review route reaches a terminal worker state, the system shall connect exactly one `verified`, `failed`, or `blocked` outcome and its available usage measurements to that routing decision. | resolved routes need an achievable outcome chain distinct from pre-claim failures | Must | high | every EVT-2 in conformance and dogfood joins exactly one EVT-5; EVT-3 terminal failures require no EVT-5 | Test + Analysis | G3, J4, F2, F3, F4 |
| FR-21 | When a routing attempt reaches its terminal resolution or worker outcome, the system shall retain its bounded decision/outcome evidence as an immutable, append-only event record in git. | routing judgments and failures are the improvement fuel for successor policy | Must | high | exact, promoted, unsupported, verified, failed, and blocked fixtures each create a new committed evidence record; reruns never overwrite an earlier record; records contain no prompt or code payload | Test + Inspection | G3, J4, F4 |

**Non-regression (NR-n)**

| ID | Existing behavior protected | Why protected | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|
| NR-1 | Harness selection remains an upstream explicit decision; routing never switches harnesses. | credentials, egress, and availability policy are outside this feature | critical | all unsupported-profile fixtures retain the selected harness and end in FR-10 | Test | NG-2, FR-10 |
| NR-2 | The merge lane remains the sole classifier of actual proposal risk and keeps its skip/standard/deep meanings. | review routing may join but must not duplicate risk judgment | critical | proposal lane fixtures are byte-for-byte unchanged; FR-13 consumes their result only | Test + Inspection | J3, FR-13 |
| NR-3 | Frozen v1 plans remain immutable and executable. | merge freezes plan artifacts | high | FR-15 compatibility fixtures execute with zero source mutation | Test | J2, F5 |
| NR-4 | Existing general authoring-stage model routing remains governed by its current policy. | authoring routing is outside this increment | medium | inspection finds no new execution-demand dependency in non-plan authoring paths | Inspection | NG-3 |
| NR-5 | Deterministic tasks continue without model invocation. | least-powerful-that-suffices begins at no inference | high | FR-2 fixtures emit zero model-launch events | Test | FR-2 |
| NR-6 | Independent review retains fresh context and reviewer separation. | model selection must not weaken review independence | critical | existing impl-review independence/conformance suites remain green | Test | J3, FR-14 |
| NR-7 | The existing repository-config policy authorization and closed-parse guarantees remain in force for routing overrides. | a routing override is critical governing policy, not an ordinary worker-authored preference | critical | an operator-authorized override can become effective; contributor-only, worker-authored, unknown-key, and unauthenticated changes cannot; existing config-policy suites remain green | Test + Inspection | J4, F4, FR-12 |

### 5.2 Unwanted behavior and failure handling

| ID | If… | Then the system shall… | User sees | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|---|
| UB-1 | a model-backed plan/v2 task omits an assessment field or uses an unknown value | reject the plan before freeze and name the field and accepted vocabulary | named plan-contract error | Must | critical | every missing/unknown-field fixture is rejected before projection | Test | FR-1 |
| UB-2 | a frozen derived profile or policy version does not match mechanical recomputation | reject the plan before projection and show expected versus supplied semantic values | named derivation-mismatch error | Must | critical | every mismatch fixture fails with zero task projection | Test | FR-3, FR-4 |
| UB-3 | a projected or tracker-reconstructed task loses or changes routing data | refuse claim and name the boundary/field mismatch | task remains unclaimed; integrity error | Must | critical | corruption fixtures produce zero claims and name the changed field | Test | FR-7 |
| UB-4 | a default or repository mapping is missing, malformed, unqualified, or weakening | refuse the effective policy before any affected dispatch | named policy error with harness, role, and profile | Must | critical | each negative policy fixture fails closed with zero invocation | Test | FR-8, FR-11, FR-12 |
| UB-5 | review demand would fall below the planned assurance floor | refuse review completion and require a correctly joined re-invocation | proposal remains unreviewed; floor violation named | Must | critical | every lowering fixture produces no accepted review evidence | Test | FR-13, FR-14 |
| UB-6 | no native profile satisfies the task | stop before claim/model invocation without trying another harness | named unsupported-profile result and unmet components | Must | critical | FR-10 negative suite proves zero side effects | Test | FR-10, NR-1 |
| UB-7 | routing telemetry cannot connect a selection or outcome to its frozen task/proposal decision | refuse to treat the attempt as acceptance or calibration evidence and surface an audit-integrity failure | attempt is visibly non-evidentiary | Must | high | orphaned-event fixtures never count toward M2–M5 or release | Test + Analysis | FR-16 |
| UB-8 | a frozen v1 rung is unknown to the applicable compatibility table | fail before claim and name the plan version and unsupported value | legacy compatibility error; plan unchanged | Must | high | unknown-rung fixtures perform zero claim/invocation and zero rewrite | Test | FR-15, NR-3 |
| UB-9 | an effective mapping or native-profile qualification is found defective after production activation | engage the strongest-profile kill switch, stop new affected claims, invalidate every affected in-flight or unaccepted evidence artifact, and require re-execution or re-review under a satisfying profile before acceptance | exact affected policy/profile versions, attempts, tasks, and proposals marked `rerun_required` | Must | critical | a seeded post-cutover qualification defect prevents acceptance of every affected open attempt until replacement evidence passes; unaffected routes continue | Failure injection | FR-16, FR-18, FR-20 |
| UB-10 | defective routing evidence has already authorized an accepted or merged proposal | preserve the shipped state, emit an operator decision brief listing every exact affected artifact, and require an explicit incident/remediation disposition | no automatic destructive rollback; exact accepted artifacts are named for human disposition | Must | critical | a seeded late discovery produces one complete brief and never silently retains a valid-routing claim or mutates accepted code | Failure injection + Inspection | FR-16, FR-21, NR-1 |

### 5.3 Permissions and abuse cases

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
| ABUSE-3 | adapter or tracker bypassing the contract | discards fields, substitutes settings, or reports a false decision | unaudited or underpowered worker launch | critical | FR-7, FR-16, UB-3, conformance suite | boundary and launch-capture tests |
| ABUSE-4 | operator override containing unrecognized or injected launch data | exploits permissive parsing to add unsupported settings | arbitrary CLI behavior or egress change | high | FR-12 and closed-schema parsing | unknown-key and argument-injection tests |

### 5.4 Data

| Entity / field | Meaning | Source of truth | Retention | Integrity rules | Privacy class |
|---|---|---|---|---|---|
| Task assessment | closed workload and consequence values plus resolvable evidence citations | frozen plan ledger | with the plan | complete on model-backed tasks; immutable after freeze | internal |
| Execution profile | vendor-neutral minimum tuple and derivation-policy version | mechanically validated frozen plan | with the plan | reproducible from assessment; no native product identifiers | internal |
| Native routing policy | qualified harness×role mappings and optional repository overrides | released defaults plus validated repo policy | current policy plus normal repository history | closed schema; version compatible; no weakening | internal |
| Routing decision | exact/promoted/unsupported resolution and native profile identity where resolved | immutable git event record; runtime event stream is its transport | forever in repository history | connected to frozen task, policy, harness, and role; a prior record is never overwritten | internal |
| Routing outcome | completion, verification/review result, and usage measurements already exposed by the harness | immutable git event record connected to the routing decision | forever in repository history | exactly one terminal outcome per resolved route; never used if orphaned or invalidated | internal |

- **Migration** — No frozen artifact is rewritten. Plan/v2 adopts the new required contract; frozen v1 reads through FR-15's versioned translator.
- **Parallel-run / cutover** — No production parallel-run, shadow, or canary path exists. Conformance and dogfood run before the one direct cutover.
- **Rollback** — The strongest-profile kill switch changes native resolution only; semantic plan data and emitted decisions remain readable. UB-9 invalidates affected unaccepted evidence, and UB-10 routes already accepted work to explicit human disposition rather than destructive rollback.
- **Decommission** — The hardcoded implementation pin, incompatible numeric ladder, and implicit review launch settings are removed only after the direct-cutover gate and fallback drill pass.
- **Privacy check** — No DPIA trigger: the feature adds operational classification and routing metadata, not personal or sensitive-data profiling.
- **Egress** — The already-selected harness remains the only model boundary. This feature changes its native launch profile but adds no provider, no cross-harness fallback, and no new source-content egress; routing telemetry records metadata rather than prompt or code content.

### 5.5 Interfaces and integrations

| Interface | Direction | Purpose | Contract | Failure modes | Owner |
|---|---|---|---|---|---|
| Planner → frozen plan | out | publish assessment and derived demand | versioned closed plan schema; complete or explicit deterministic mode | missing/unknown values, derivation mismatch | plan-new |
| Frozen plan → tracker/executor | out | preserve demand to claim and dispatch | lossless versioned projection/read | field loss, legacy-version mismatch | plan projection + tracker adapters |
| Executor/reviewer → harness adapter | out | resolve semantic minimum for a selected harness and role | qualified effective mapping; exact/promoted/unsupported result | missing cell, incompatible override, no satisfying profile | runtime adapters |
| Merge lane → review routing | in | supply actual proposal skip/standard/deep band | monotone join with planned assurance | missing band, attempted lowering | impl-review + merge lane |
| Runtime/review → evidence stream | out | connect route, promotion/failure, and outcome | typed events joined by task/proposal and attempt identity | orphan, partial write, schema mismatch | execution/review runtime |

### 5.6 Non-functional requirements

| ID | Area | Requirement | Fit criterion | Verify |
|---|---|---|---|---|
| NFR-1 | Determinism | For identical assessment, policy version, selected harness, role, and override, the system shall produce an identical semantic and native routing decision. | repeated and reordered-input fixtures are byte-identical | Test |
| NFR-2 | Portability | The semantic assessment and execution-profile schemas shall contain no harness or vendor product identifiers. | schema inspection finds zero native identifiers; every supported adapter consumes the same semantic values | Inspection + Test |
| NFR-3 | Security / integrity | The system shall parse assessment, profile, compatibility, and native-policy data as closed schemas. | every unknown-key, unknown-enum, and incompatible-version fixture fails before dispatch | Test |
| NFR-4 | Operability | Every failed or promoted resolution shall identify the plan task, harness, role, policy version, and affected semantic component without exposing prompt or code content. | diagnostic golden files contain all named metadata and no payload content | Test + Inspection |
| NFR-5 | Cost | Among qualified native profiles that satisfy every semantic minimum, the resolver shall choose the declared least-cost profile. | exhaustive mapping tests prove no more costly satisfying candidate is selected when a cheaper qualified candidate exists | Test + Analysis |
| NFR-6 | Availability / cost | Routing resolution shall consume zero model inference and require no network call beyond the model invocation already owned by the selected harness. | dependency inspection and the offline resolution suite show zero model-launch event and zero outbound call before the selected worker invocation | Inspection + Test |
| NFR-7 | Maintainability | A native model roster change shall be expressible as reviewed routing-policy data plus qualification evidence without changing frozen plans. | a roster-change fixture updates mappings and passes conformance with zero plan mutation | Demo + Inspection |

### 5.7 Instrumentation — the tracking plan

| ID | Event name | Fires when | Properties (typed) | Source | Feeds |
|---|---|---|---|---|---|
| EVT-1 | `execution_demand_frozen` | a model-backed task passes plan validation | `task_id:string`, `plan_version:string`, `assessment:closed object`, `assessment_evidence_refs:string[]`, `profile:closed object`, `policy_version:string`, `assurance_raise:boolean` | plan gate | M5 |
| EVT-2 | `routing_resolved` | an implementation or review route resolves | `attempt_id:string`, `task_id:string`, `proposal_id:string?`, `harness:string`, `role:enum`, `requested_profile:closed object`, `native_profile_id:string`, `resolution:exact|promoted`, `promoted_components:enum[]`, `policy_version:string` | runtime adapter | M2, M4, M5 |
| EVT-3 | `routing_resolution_failed` | routing stops before claim/invocation | `attempt_id:string`, `task_id:string`, `proposal_id:string?`, `harness:string`, `role:enum`, `requested_profile:closed object`, `policy_version:string`, `unmet_components:enum[]`, `reason:enum` | executor/reviewer | M2, M5 + health alert |
| EVT-4 | `routing_conformance_checked` | one required routing-domain cell finishes conformance | `harness:string`, `role:enum`, `semantic_profile_id:string`, `expected:resolved|unsupported`, `result:pass|fail`, `evidence_ref:string` | conformance harness | M3 |
| EVT-5 | `routed_attempt_completed` | implementation verification or independent review completes | `attempt_id:string`, `task_id:string`, `proposal_id:string?`, `outcome:verified|failed|blocked`, `usage:closed object`, `evidence_ref:string` | executor/reviewer | M4, M5 |
| EVT-6 | `execution_demand_checked` | every plan/v2 task not explicitly deterministic reaches validation, before acceptance or rejection | `task_id:string`, `plan_version:string`, `result:pass|fail`, `violations:enum[]` | plan gate | M1 |
| EVT-7 | `routing_evidence_invalidated` | UB-9 or UB-10 identifies evidence produced under a defective policy/profile | `policy_version:string`, `native_profile_id:string`, `attempt_ids:string[]`, `artifact_refs:string[]`, `disposition:rerun_required|operator_remediation` | execution/review evidence owner | health alert + UB-9/UB-10 audit |

### 5.8 Definition of Done

| ID | Done criterion | Source | Verify | Required evidence | Blocking? |
|---|---|---|---|---|---|
| DOD-1 | Every Must functional and failure requirement passes its own fit criterion. | FR-1..FR-21, UB-1..UB-10 | Test + Eval + Demo + Inspection + Analysis + Failure injection | per-requirement evidence index linked from the proposal | yes |
| DOD-2 | No protected planner, harness-selection, merge-lane, legacy-plan, deterministic, independent-review, or config-authorization behavior regresses. | NR-1..NR-7 | Test + Inspection | non-regression suite and boundary inspection | yes |
| DOD-3 | Every required harness/role/derivable-profile cell passes conformance and qualification review. | M3, FR-8, FR-11, FR-12, EVAL-2 | Test + Human judgment — @mdarabi; rubric: every required cell has the expected satisfying or unsupported result, passing launch/failure capture, and current qualification evidence | conformance matrix and operator sign-off | yes |
| DOD-4 | Pre-release dogfood exercises at least three distinct native profiles with verified outcomes and complete evidence chains. | M4, M5, FR-16 | Analysis | joined routing/outcome report | yes |
| DOD-5 | Planner assessment and derivation evals pass, with zero under-assessed safety floors in the adversarial set. | EVAL-1, FR-1, FR-3, FR-5 | Eval | versioned eval report | yes |
| DOD-6 | Every §3.3 guardrail is clean before activation. | §3.3 | Analysis | pre-release decision report | yes |
| DOD-7 | The direct-cutover configuration and strongest-profile kill switch pass rehearsal without a harness switch, review-floor loss, or any semantic downgrade; an unsatisfiable fallback still fails before claim. | FR-10, FR-17, FR-18, NR-1, NR-2 | Test + Demo | cutover and fallback drill record | yes |
| DOD-8 | Routing diagnostics and immutable git evidence are complete, connected, payload-free, and usable by the operator to explain one exact, one promoted, one unsupported, and one review-join case. | FR-16, FR-20, FR-21, NFR-4, EVT-1..EVT-7 | Demo + Inspection | four-case audit walkthrough plus committed event records | yes |
| DOD-9 | A post-cutover qualification-defect drill invalidates every affected open artifact, preserves unaffected work, and produces an operator brief for already accepted artifacts. | UB-9, UB-10, §3.3 | Failure injection + Inspection | defect-disposition drill record | yes |

### 5.9 AI behavior and evals

| ID | Behavior | Eval set / harness | Threshold | Negative examples |
|---|---|---|---|---|
| EVAL-1 | The planner assigns values from the normative assessment vocabulary, cites resolvable evidence, and never produces a semantic profile below deterministic policy floors. | versioned clear-case and adversarial task corpus covering every accepted assessment/profile value, consequence floor, named promotion, deterministic task, and planner raise | 100% structurally valid; 100% exact on unambiguous golden cases; zero unresolved evidence citations and zero consequence/assurance under-floor outcomes | simple-looking system-core change; first-of-kind contained change; cross-boundary composed verification; attempted assurance reduction; missing evidence; unknown enum |
| EVAL-2 | Each required routing-domain cell produces a qualified satisfying native profile or the explicit unsupported result appropriate to that cell. | harness adapter conformance corpus with representative implementation and independent-review tasks plus launch/failure capture | every required cell produces its specified result; every resolved cell produces a verified outcome; zero harnesses or derivable profiles are omitted; zero cells qualify by self-declaration alone | omitted harness; omitted derivable profile; mapping to an unavailable model; effort omitted; review reuses implementation setting; cheaper profile fails claimed demand; unreviewed new model |

## 6. Delivery

### 6.1 Phasing and milestones

| Phase | Purpose | Scope | Exit criterion |
|---|---|---|---|
| A — contract | make demand explicit and mechanically trustworthy | plan/v2 vocabulary, evidence-backed assessment, derivation, validation, transport, legacy translation | FR-1..FR-7, FR-15, and FR-19 green; EVAL-1 green |
| B — resolution | complete implementation and review consumption | required-domain mappings, overrides, resolver, review join, failure paths, immutable evidence | FR-8..FR-14, FR-16, FR-20, FR-21 and adapter conformance green |
| C — pre-release proof | falsify under-routing before any production activation | full non-regression, failure/abuse suites, dogfood diversity, operator audit, kill-switch rehearsal | M1–M5 and every §3.3 guardrail green; DOD-1..DOD-8 evidenced |
| D — production | replace hardcoded/implicit routing in one cutover | every supported implementation and review path | direct activation succeeds; production events remain connected and guardrails clean |

### 6.2 Dependencies

| Dependency | Type | Needed by | Status | Owner |
|---|---|---|---|---|
| Typed amendments to the frozen plan-new, implement-now, and impl-review contracts | approval / document system | Phase A design handoff | required (`brainstorm:planner-driven-model-routing#P-1`) | @mdarabi + factory |
| Existing plan projection and tracker-adapter boundary | system | Phase A | available; known transport defects | plan/executor owners |
| Existing shared runtime port and supported harness registry | system | Phase B | available; native policies incomplete/inconsistent | runtime owners |
| Existing merge-lane proposal band | system | Phase B | available; semantics protected by NR-2 | review/merge-lane owners |
| Qualified native model roster and cost metadata per harness/role | vendor / policy | Phase B and release | design-owned; evidence required by FR-12/EVAL-2 | @mdarabi + runtime owners |

### 6.3 Risks, assumptions, and rabbit holes

| # | Risk / assumption / rabbit hole | Likelihood | Impact | Mitigation or test | Owner |
|---|---|---|---|---|---|
| R1 | A semantically plausible but wrong planner assessment passes structural validation. | M | H | EVAL-1 adversarial corpus, evidence citations, mechanical consequence floors, human plan checkpoint | plan-new owner |
| R2 | A native mapping goes stale as harness model catalogs change. | H | H | closed versioned mappings, operator authorization, qualification evidence, unsupported failure, EVAL-2, strongest-profile fallback | runtime owners |
| R3 | Transport fixes one tracker path but leaves another dropping fields. | M | H | consumer-side round-trip conformance for every declared tracker path (FR-7/UB-3) | executor owners |
| R4 | Review routing accidentally creates a second proposal-risk classifier. | M | H | protect merge-lane semantics (NR-2); only monotone join in FR-13 | impl-review owner |
| R5 | Direct activation exposes a mapping defect broadly. | M | H | all-path pre-release proof, fail before claim, strongest-profile kill switch rehearsed before cutover, UB-9 invalidation, UB-10 remediation brief | @mdarabi |
| R6 | The feature expands into adaptive scorecards or cross-harness policy and exceeds the appetite. | M | M | enforce NG-1/NG-2 and route successor work to WR-1/WR-2 | @mdarabi |
| R7 | Legacy translation becomes a second mutable inference policy. | L | H | versioned explicit translation table, conservative absent-rung sentinel, no source rewrite | plan/executor owners |

### 6.4 Release criteria

| Lens | Criterion | Evaluated by |
|---|---|---|
| Functionality | Every Must FR and UB fit criterion passes; M1 and M3 reach target over their complete denominators. | CI, eval, conformance matrix |
| Usability | Operator explains exact, promoted, unsupported, and joined-review decisions from emitted evidence without reading adapter code. | DOD-8 audit walkthrough |
| Reliability | Zero §3.3 guardrail breaches across conformance, adversarial fixtures, dogfood, direct-cutover rehearsal, and kill-switch rehearsal; the post-defect disposition drill passes. | test/eval reports and DOD-9 drill record |
| Performance / cost | Resolution remains local and deterministic; every successful selection is the least-cost qualified satisfying profile. | NFR-1, NFR-5, NFR-6 suites |
| Supportability | Named failures identify harness, role, policy, task, and unmet component; strongest-profile fallback and runbook are operator-approved. | diagnostic goldens, runbook review, DOD-7 |

### 6.5 Rollout and reversibility

- **Flag** — one production routing-policy switch, owned by @mdarabi, changes native resolution only after Phase C is green.
- **Kill switch** — strongest-profile fallback per harness role; tested before release; preserves selected harness, every semantic minimum, review join, and evidence; an unsatisfiable demand still fails under FR-10.
- **Experiment** — none; this increment makes no adaptive or statistical-optimization claim.
- **Cutover/migration steps** — validate policy and overrides; pass the complete support matrix, evals, dogfood, and fallback drill; activate all supported implementation and review paths together; remove the temporary hardcoded/implicit paths only after successful activation.

| Stage | Audience | Minimum evidence | Advance when | Roll back when |
|---|---|---|---|---|
| Pre-release only | conformance fixtures and repository dogfood; no production routing | DOD-1..DOD-8 and §6.4 evidence | every metric target and guardrail is green | any failed criterion blocks activation |
| 100% production | every required implementation and review path | successful direct-cutover rehearsal and effective-policy snapshot | post-launch trigger reached with M5 complete and guardrails clean | any under-routing, critical-floor violation, qualification defect, or evidence-integrity breach engages the kill switch, stops affected claims, and invokes UB-9/UB-10 |

### 6.6 Launch readiness

| Area | Item | Owner | Status |
|---|---|---|---|
| Support | routing failure/promotion diagnostics and strongest-profile fallback runbook | @mdarabi | ☐ |
| Docs | operator documentation for semantic profiles, native mappings, overrides, and audit fields | @mdarabi | ☐ |
| Legal & privacy | confirm no new provider or payload egress and no DPIA trigger | @mdarabi | ☐ |
| Platform | supported harness/role policy and qualification evidence current for the release | runtime owners | ☐ |
| Cost | least-cost order and native-profile cost metadata reviewed | @mdarabi | ☐ |
| Marketing & external comms | skipped — internal factory control-plane change with no end-user launch surface | @mdarabi | n/a |
| Localization & accessibility | skipped — no human interface is introduced | @mdarabi | n/a |

**Decision:** Launch gate — §6.4 criteria green, §6.6 owned and checked, rollback rehearsed.

| Reviewer | Role | Date | Verdict |
|---|---|---|---|
| @mdarabi | operator | — | — *(via the release proposal's operator review)* |

### 6.7 Post-launch review

- **Review trigger** — production has completed verified attempts using at least three distinct native profiles, including one deep review produced by a critical assurance floor, with complete immutable M5 evidence · **Owner** — @mdarabi
- **Verdict recorded** — adopt / iterate / kill, with M1–M5 and guardrail evidence — **on the header's Tracking ref**. Contract changes travel as typed amendments; adaptive-policy proposals route to WR-1.

## 7. Alignment

### 7.1 FAQ

**Customer / operator**

- **Q:** Does the planner choose a Codex, Claude, Cursor, or OpenCode model? — **A:** No. It freezes vendor-neutral execution demand; the adapter for the already-selected harness chooses qualified native settings.
- **Q:** Can a repository prefer a different qualified model? — **A:** Yes, through a closed reviewed override. An override can change native selection but cannot weaken the semantic minimum.
- **Q:** What happens when the selected harness cannot satisfy a task? — **A:** The task fails before claim or model invocation with the unmet requirements. Pyxis does not silently switch harnesses.

**Internal**

- **Q:** Why not keep a single low/medium/high complexity label? — **A:** Reasoning, context, and consequence do not move together. Independent assessment and profile components prevent a simple system-core change or a novel contained change from being misrepresented.
- **Q:** Does this replace merge-lane risk? — **A:** No. Plan-time assurance supplies a floor; the merge lane remains the sole classifier of the actual finished proposal and may only raise review demand.
- **Q:** Why direct production instead of shadowing? — **A:** The operator explicitly chose full pre-release conformance and dogfood followed by one production cutover. Fail-closed dispatch and a rehearsed strongest-profile kill switch bound the production risk.
- **Q:** Is lower model spend a launch promise? — **A:** No. Least-cost satisfying selection and routing diversity are contractual; statistically proven optimization and self-tuning are successor work.

### 7.2 Open questions

| # | Question | Owner | Needed by | Status |
|---|---|---|---|---|
| Q1 | What exact per-axis floor table and named interaction-promotion rules implement FR-3/FR-5 without hiding a weighted score? | technical design | Phase A design gate | open — design-owned; product aggregation and safety behavior are fixed |
| Q2 | What policy schema, qualification-evidence format, and precedence implement shipped defaults plus closed repository overrides? (`brainstorm:planner-driven-model-routing#P-2`) | technical design | Phase B design gate | open — design-owned; FR-11/FR-12 and EVAL-2 fix the product bar |
| Q3 | Which compatibility values map each historical explicit rung while keeping absent v1 routing conservative? (`brainstorm:planner-driven-model-routing#P-3`) | technical design | Phase A design gate | open — design-owned; FR-15 fixes the absent-rung behavior and no-rewrite rule |

### 7.3 Decision log

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

### 7.4 Changelog

| Date | Change | Author |
|---|---|---|
| 2026-07-26 | Created 000-base from gated `brainstorm:planner-driven-model-routing`. | @mdarabi + factory |

## 8. Contract readiness — the executability gate

| Check | Pass? | Notes |
|---|---|---|
| Problem basis accepted — §1 problem accepted (T1); §2 problem gate signed (T2/T3) | ☐ | pending introducing PR operator review |
| Zero solution-blocking TBDs or open questions (§7.2) | ☑ | Q1–Q3 are design mechanics with product behavior fixed by cited requirements |
| Every Must FR has a fit criterion and a verification method (§5.1) | ☑ | FR-1..FR-21 carry both |
| Every requirement row carries a risk tier; high/critical rows have a declared safety plan | ☑ | fail-before-claim behavior, pre-release full-domain proof, direct-cutover and unsatisfiable-fallback rehearsal, UB-9/UB-10 post-defect disposition, operator-authorized policy, immutable evidence, and no-rewrite migration cover the high/critical surfaces |
| Non-goals drawn (§3.4); non-regression rows named (§5.1) | ☑ | NG-1..NG-7, WR-1..WR-3, NR-1..NR-7 |
| Every primary job (§2.3) is covered by a requirement, a non-goal, or a waiting-room entry | ☑ | J1→FR-1..FR-7/FR-19; J2→FR-8..FR-12/FR-15/FR-18; J3→FR-13/FR-14/FR-18; J4→FR-12/FR-16/FR-20/FR-21/NR-7 |
| Every key flow (§4.3) has functional, failure, and non-regression coverage (§5.1–5.2) | ☑ | F1→FR-1..FR-6/FR-19/UB-1..UB-2/NR-5; F2/F3→FR-7..FR-14/FR-16/FR-18/FR-20/UB-3..UB-6/UB-9..UB-10/NR-1..NR-2/NR-6; F4→FR-12/FR-16/FR-19..FR-21/UB-4/UB-7/UB-9..UB-10/NR-7; F5→FR-15/UB-8/NR-3 |
| Every primary metric (§3.2) has instrumentation (§5.7) or an explicit measurement plan | ☑ | M1→EVT-6 (pass ÷ all non-deterministic candidates); M2→EVT-2/3; M3→EVT-4 over the required routing domain; M4→EVT-2/5; M5→EVT-1 plus terminal EVT-3 or EVT-2+EVT-5 |
| Every blocking DoD row traces to a requirement, guardrail, release criterion, or human-judgment rubric (§5.8) | ☑ | DOD-1..DOD-9 each cite reviewed source rows |
| Risks and assumptions have owners (§6.3) | ☑ | R1..R7 owned |
| Human-judgment criteria name an owner and a rubric (§5.8) | ☑ | DOD-3 names @mdarabi and the three-part qualification rubric |
| DoD complete: every blocking row sourced and evidenced; every n/a explained (§5.8) | ☑ | all DoD rows block and name evidence; §6.6 n/a rows state the absent surface |
