# Planner-driven model routing — brainstorm record

| Field | Value |
|---|---|
| **In one line** | Replace hardcoded execution models with a planner-frozen, vendor-neutral task-demand decision that each implementation and review runtime resolves to a qualified harness-native model and effort. |
| **Relevant-when** | brainstorming or contracting task-complexity assessment, plan-time capability routing, per-harness model/effort selection, implementation/review runtime profiles, or the path from coarse rungs to VNEXT-7 routing intelligence; checking prior rulings about `AgentRuntime.tiermap`, review depth bands, risk-adjusted routing, or frozen dispatch decisions |
| **Idea** | "currently we have hardcoded the implementation engine to be Sonnet 5. the main idea was at the planning time, we can decide on the complexity of the task, and the execution engine (implementation engine and review engine) will decide what model and effort to use based on the task complexity as well as the underlying harness. for example if the task is marked as high complexity and we are using codex we should use gpt-5.6 sol with xhigh effort but if we are on cursor cli we should end up using different model. first question, figure out the different level of complexity and tagging that planning agent supports"; "understood, so what needs to be changed to move on to the world that planner decides on some complexity scale that executino engine would use to decide on the model and effort? what should be the dimensions of measuing the \"complexity\" ? how to translate that to required model /effort?" (operator, 2026-07-25) |
| **Operator** | @mdarabi |
| **Date(s)** | 2026-07-25 · 2026-07-26 |
| **Status** | gated: ready-for-prd |
| **Tracking** | — |
| **Feeds** | docs/prds/planner-driven-model-routing/ |
| **Prior records** | brainstorm:implement-now, brainstorm:impl-review |
| **Template version** | brainstorm-record/v1 |

## Orientation

| Source | Why loaded |
|---|---|
| `docs/product-vision.md` §4 · `docs/prds/README.md` | governing stage-7/8/9 boundary and the capability baseline: IMPL-7 coarse rungs are shipped; fine-grained routing remains backlog |
| `docs/prds/vnext-requirements.md` VNEXT-7 · VNEXT-9 | direct overlap: backend scorecards + routing intelligence, frozen per-task decisions, multi-agent review for risky code, and expanded risk affecting backend eligibility |
| `.pyxis/model-routing.md` | current human policy: deterministic/mechanical/authoring/judgment complexity classes, risk adjustment, capability names rather than product IDs |
| `docs/prds/plan-new/effective.md` FR-7 · FR-14 · FR-16 | planner contract: mechanical quarantine; optional frozen routing proposal; ambiguity must bounce rather than enter a task |
| `docs/prds/implement-now/effective.md` FR-6 · EVT-2 · §5.5 | executor contract: frozen rung resolves per backend to model+effort through one four-CLI runtime port |
| `docs/prds/impl-review/effective.md` FR-2 · FR-9 · FR-10 | reviewer contract: fresh runtime; depth comes from the merge lane's one risk classification as skip/standard/deep |
| `docs/domain-model.md` Spec · AgentRuntime · RoutingPolicy · INV-31 | existing domain seam: plan-frozen capability rung and adapter-native `tiermap(rung → model,effort)`, judged once at richest context |
| `docs/tech-design/routing-envelope-v1.md` §3 · §7 | current coarse envelope and explicit successor gaps: per-task risk, exact routing decision, tier→model lineup, and distinct no-model representation |
| `docs/brainstorms/implement-now.md` D-1 · `docs/brainstorms/impl-review.md` D-4 | prior operator rulings: all four CLIs share one `AgentRuntime`; proposal review reuses merge-lane classes and never invents a second risk classifier |
| `docs/wiki/glossary.md` | vocabulary check: `rung` already primarily names a verification-ladder layer, so routing should avoid overloading it |

**Overlaps surfaced:**

- `corpus:VNEXT-7` already names the larger destination — learn backend×task-type performance, route simple work cheaply and architecture/risky work strongly, preserve judge-once frozen decisions, and support backend fallback. This idea is not a duplicate capability: VNEXT-7 is a placeholder requirement, while no feature PRD yet defines the planner assessment, runtime resolution contract, or rollout.
- `corpus:VNEXT-9` already requires task risk to influence eligible agent/backend. Risk therefore composes with routing but should not be silently redefined as task complexity.
- `prd:plan-new/FR-14` covers an optional coarse proposal whose absence is valid; the desired world requires deciding whether every model-backed task must carry a closed assessment and what freezes.
- `prd:implement-now/FR-6` covers the downstream seam in outline (`rung → backend-native model,effort`) but not the task dimensions, capability manifest, unsupported-profile behavior, or qualification/calibration policy.
- `prd:impl-review/FR-9` and `brainstorm:impl-review#D-4` already fix review risk/depth to the merge lane's one classifier. This feature may select the review model/effort, but must not introduce a competing proposal-risk predicate without an explicit superseding product ruling.
- `brainstorm:implement-now#D-1` already fixes Claude Code, Codex, Cursor CLI, and OpenCode behind one runtime port. The harness set and adapter boundary are covered and will not be re-asked.

## Framings

- **A — Routing contract completion.** Make plan-time task demand a required, evidence-backed, vendor-neutral contract; deterministically derive capability/deliberation requirements; let each implementation and review adapter resolve them through versioned qualified profiles. Include observability and a compatibility rollout, but keep automatic scorecard learning out of scope. Implication: removes the hardcoded Sonnet pin and completes the currently deferred planner→runtime seam with the smallest independently useful feature. Opens VNEXT-7's later learning loop over trustworthy routing/outcome data.
- **B — Full adaptive routing intelligence.** Implement VNEXT-7 as one feature: workload taxonomy, backend×task-type scorecards, exploration/minimum-sample rules, policy updates, planner decisions, runtime resolution, fallback, and review routing. Implication: the system can improve its mappings from evidence at launch, but the feature includes both the execution contract and an optimizer/calibration program.
- **C — Runtime capability resolver first.** Treat the feature primarily as executor infrastructure: repair rung transport, replace `tiermap` with backend capability manifests, route implementation and review correctly, and leave the planner on its existing mechanical/behavioral plus optional-rung vocabulary. Implication: removes hardcoding fastest, but deliberately postpones the requested complexity dimensions and keeps planning judgment coarse.

**Destination:** **A — Routing contract completion** — first establish the required planner-frozen execution-demand contract, its deterministic derivation, and qualified per-harness implementation/review resolution; retain observability for later learning but leave automatic scorecard-driven adaptation to a subsequent VNEXT-7 increment. The operator chose to "start with A," adopting the recommended foundation-first sequence while keeping B open as its successor.

## Decisions

### D-1 — destination: routing contract completion before adaptive learning

- **Question**: should this feature be A (routing contract completion), B (full adaptive routing intelligence), C (runtime resolver first), or a hybrid?
- **Answer**: **start with A** — complete the planner→runtime routing contract first; adaptive scorecard learning follows later.
- **Reasoning**: A is the smallest independently useful foundation that removes hardcoding and produces trustworthy routing/outcome data; the operator's "start with" preserves B as the intended successor rather than rejecting it.
- **Anchor**: 2026-07-25 · t1
- **Status**: live

### D-2 — freeze both assessed dimensions and derived execution requirements

- **Question**: should the frozen plan carry only a workload class, only raw dimensions for dispatch-time interpretation, or closed dimensions plus a deterministically derived execution profile?
- **Answer**: **both raw dimensions and the derived profile** — preserve option 2's executor freedom by letting each harness map the requirements to its own native model/effort, while retaining option 3's checked derivation.
- **Reasoning**: the dimensions preserve why the task was classified; the derived profile prevents dispatch from re-judging it; the executor remains free exactly where backends differ — mapping semantic capability/deliberation requirements to Codex, Cursor, Claude, or OpenCode launch settings.
- **Anchor**: 2026-07-25 · t2
- **Status**: live

### D-3 — retain impact scope and architectural role as distinct modifiers

- **Question**: should the assessment add the change's affected reach and how fundamental its target is, without overloading the existing `Blast radius` and `Foundation` terms?
- **Answer**: yes — keep `impact_scope: contained | multi_surface | system_wide` and `architectural_role: feature_local | shared_substrate | system_core`.
- **Reasoning**: a locally implemented change can affect the whole system, while a cross-component change can remain feature-contained; architectural centrality raises the cost of a miss independently of reasoning difficulty. Distinct names preserve the glossary's amendment-specific `Blast radius` and plan-DAG `Foundation` meanings.
- **Anchor**: 2026-07-25 · t3
- **Status**: live

### D-4 — retain all workload axes plus both consequence modifiers

- **Question**: should the assessment retain any or all of `reasoning`, `coupling`, `novelty`, and `verification` alongside `impact_scope` and `architectural_role`?
- **Answer**: retain **all workload + consequences**: the four workload axes and both consequence modifiers as one complete assessment.
- **Reasoning**: the workload axes independently describe execution difficulty; the consequence modifiers describe affected reach and architectural centrality. Keeping both groups avoids routing a cognitively simple but system-critical change cheaply, or treating a novel but contained change as system-wide risk.
- **Anchor**: 2026-07-25 · t4
- **Status**: live

### D-5 — require the complete assessment on every model-backed task

- **Question**: should every model-backed `plan/v2` task carry all six assessment fields and its derived execution profile, or may routing remain absent/defaulted?
- **Answer**: require the complete assessment for every model-backed task; deterministic execution declares no inference explicitly, and frozen v1 plans use a compatibility translation.
- **Reasoning**: an omitted or zero-valued route is the current path to accidental cheapest-model behavior and the temporary all-Sonnet pin. Closed required data makes dispatch deterministic while preserving old plans without rewriting them.
- **Anchor**: 2026-07-25 · t5
- **Status**: live

### D-6 — derive assurance mechanically; planner may only raise it

- **Question**: should risk be a seventh planner-scored axis, or should policy derive an assurance floor from consequence and existing risk signals with planner authority only to raise it?
- **Answer**: adopt the **derived, planner-raise-only assurance floor** (`ordinary | elevated | critical`).
- **Reasoning**: deterministic floors prevent optimistic under-routing, while a reasoned planner raise preserves richest-context judgment. The floor governs plan-time execution demand; the merge lane remains the sole classifier of actual proposal risk and may raise review assurance from the finished diff.
- **Anchor**: 2026-07-25 · t6
- **Status**: live

### D-7 — derive four independent semantic execution requirements

- **Question**: should the derived execution profile carry independent capability, deliberation, context, and assurance requirements rather than collapse them into another scalar rung?
- **Answer**: yes — freeze `capability: economy | standard | strong | frontier`, `deliberation: light | standard | deep | maximum`, `context: standard | large | extended`, and `assurance: ordinary | elevated | critical`.
- **Reasoning**: model strength, reasoning effort, context capacity, and scrutiny do not rise together for every task. Independent requirements preserve that distinction while letting each harness resolve their tuple to one qualified native launch profile.
- **Anchor**: 2026-07-25 · t7
- **Status**: live

### D-8 — aggregate by component floors plus named promotions

- **Question**: should assessment→profile derivation use a weighted score, or component-wise maximum floors followed by a small closed set of named interaction promotions?
- **Answer**: use **max-of-floors plus named promotions**; `plangate` recomputes the frozen profile and rejects a mismatch.
- **Reasoning**: no load-bearing input can be averaged away; independent components still diverge appropriately; explicit promotions capture combinations such as `compose + adapt` or `critical assurance → frontier capability` without hidden arithmetic or false precision.
- **Anchor**: 2026-07-25 · t8
- **Status**: live

### D-9 — harness adapters own layered native-profile mappings

- **Question**: should harness-native model/effort mappings be release-owned, repository-owned, or layered as Pyxis-shipped defaults plus closed repository overrides?
- **Answer**: use **shipped harness-adapter defaults plus optional closed repository overrides**, with distinct implementation and review mappings.
- **Reasoning**: shipped defaults provide qualified portable behavior; repo overrides express local availability/cost policy without putting model IDs into plans. The Pyxis-owned harness adapter performs the translation—the external CLI does not interpret Pyxis execution-demand fields.
- **Anchor**: 2026-07-25 · t9
- **Status**: live

### D-10 — resolve least-cost satisfying; promote upward or fail closed

- **Question**: when a selected harness has no exact native profile, should it clamp down, promote up, switch harnesses, or fail?
- **Answer**: select the least-cost qualified profile satisfying all requirements; an upward promotion is allowed and recorded; a downgrade is forbidden; no satisfying profile fails before claim/model invocation; framing A never silently switches harnesses.
- **Reasoning**: the frozen profile is a minimum contract, so under-powering violates judge-once routing. Promotion preserves correctness visibly; named failure preserves credential/egress and backend-selection boundaries. Cross-harness fallback remains VNEXT-7 successor scope.
- **Anchor**: 2026-07-25 · t10
- **Status**: live

### D-11 — join planned assurance with actual proposal review band

- **Question**: should review use only the merge-lane band, or the monotone join of the planner-frozen assurance floor and the actual proposal's lane band?
- **Answer**: use the **planned-floor plus actual-band monotone join**: ordinary permits skip, elevated requires at least standard, critical requires deep; the actual lane may raise but never lower it.
- **Reasoning**: the task's consequence floor survives into independent review, while the merge lane remains the sole classifier of finished-diff risk. The reviewer resolves the joined demand through the review-role harness mapping, never inheriting the implementation model.
- **Anchor**: 2026-07-25 · t11
- **Status**: live

### D-12 — segments: machine path primary, operator secondary, adaptive consumers later

- **Question**: who should this feature optimize for, and who is explicitly not served by framing A?
- **Answer**: primary = the model-backed planner→implementation/review execution path; secondary = the repository operator configuring overrides and auditing selections; anti-segment = consumers expecting automatic scorecard learning, exploration, or cross-harness fallback.
- **Reasoning**: deterministic production and consumption of routing data is the direct high-frequency job; operator control and explainability remain required. Adaptive optimization depends on trustworthy outcomes from this foundation and stays in VNEXT-7 successor scope.
- **Anchor**: 2026-07-25 · t12
- **Status**: live

### D-13 — four jobs across planning, implementation, review, and operation

- **Question**: should the feature's job set cover planner classification, implementation resolution, independent review resolution, and operator audit/override?
- **Answer**: yes — those four jobs define the feature.
- **Reasoning**: together they close the full contract: evidence-backed demand is produced once, consumed without re-judgment for implementation, conservatively joined with actual risk for review, and remains explainable/configurable to the operator. Omitting any one recreates either hardcoding, silent under-routing, or an unauditable selection.
- **Anchor**: 2026-07-25 · t13
- **Status**: live

### D-14 — success proves the routing contract, not adaptive optimization

- **Question**: should framing A launch on contract/conformance proof and real routing diversity, reserving statistically demonstrated optimization for VNEXT-7?
- **Answer**: yes — require complete plan/v2 assessments, all-four-adapter implementation/review conformance, zero under-routing, critical-floor preservation, at least three native profiles in dogfood with verified outcomes, and complete routing/outcome telemetry.
- **Reasoning**: those signals falsify the hardcoding and transport failures this feature owns and create the calibration corpus. Cost/quality trends are measured during rollout, but a statistically guarded policy learner is explicitly the successor rather than a launch claim.
- **Anchor**: 2026-07-25 · t14
- **Status**: live

### D-15 — appetite: one complete routing seam, bounded before adaptation

- **Question**: should the appetite cover the full plan→transport→implementation/review adapter seam, compatibility and safe rollout, while excluding the adaptive-routing program?
- **Answer**: yes — include plan/v2, derivation policy, end-to-end transport, four harness adapters and role mappings, repo overrides, review join, v1 compatibility, telemetry, shadow rollout, and kill switch; exclude the named adaptive and general-policy surfaces.
- **Reasoning**: this is the minimum end-to-end scope that can actually remove hardcoding safely. Adding learning, exploration, backend selection/fallback, broad authoring routing, unified risk policy, or automatic model qualification would combine independently revisable programs before the routing substrate has evidence.
- **Anchor**: 2026-07-25 · t15
- **Status**: superseded by D-17 — scope and non-goals retained; shadow rollout replaced by test-then-direct-production activation

### D-16 — conservatively translate unassessed frozen v1 tasks

- **Question**: should frozen v1 tasks with no rung be inferred, refused/replanned, or translated to a conservative compatibility profile?
- **Answer**: preserve them through `legacy_unassessed`: frontier capability, maximum deliberation, extended context, ordinary assurance until existing risk signals raise it; explicit v1 rungs use a versioned compatibility table; no v1 plan is rewritten.
- **Reasoning**: the missing dimensions were never judged, so inference invents history and refusal breaks frozen-plan executability. Conservative execution spends more only on legacy work, avoids under-powering, and makes every compatibility dispatch auditable while v2 carries optimized routing.
- **Anchor**: 2026-07-26 · t16
- **Status**: live

### D-17 — test before release, then activate directly in production

- **Question**: should D-15's shadow/canary rollout stand, or should the full routing seam be tested pre-release and then enabled directly in production?
- **Answer**: retain D-15's full vertical scope and non-goals, but after its tests pass, **roll routing directly into production** for all supported profiles; no shadow or canary stages.
- **Reasoning**: operator ruling after resolving the distinction between testing *in* production and testing *before* release. Pre-deployment conformance and dogfood remain the launch gate; fail-closed resolution and the strongest-profile kill switch remain production safeguards.
- **Anchor**: 2026-07-26 · t17
- **Status**: live

## Facts

- **F-1**: VNEXT-7 already requires backend×task-type scorecards, routing simple work cheaply and architecture/risky work strongly, multi-agent review for risky code, backend fallback, and judge-once frozen task decisions — source: `docs/prds/vnext-requirements.md` VNEXT-7
- **F-2**: the current global policy has four broad classes — deterministic, mechanical, authoring, judgment/adversarial — with risk adjustment and model IDs deliberately outside the tier names — source: `.pyxis/model-routing.md`
- **F-3**: plan-new currently makes the per-task routing proposal optional (`Should`) and accepts absence; its only general task-work distinction is `Mechanical: yes|no` — source: `docs/prds/plan-new/effective.md` FR-7 · FR-14
- **F-4**: routing-envelope-v1 explicitly defers per-task risk, exact deterministic routing, tier→concrete-model mapping, and a distinct no-model proposal to successor work — source: `docs/tech-design/routing-envelope-v1.md` §3.4 · §7 Q1–Q3
- **F-5**: implement-now already requires each frozen task rung to resolve to a backend-native model+effort, with all four CLIs behind one runtime port — source: `docs/prds/implement-now/effective.md` FR-6 · §5.5; `brainstorm:implement-now#D-1`
- **F-6**: impl-review's depth is already bound to the merge lane's skip/standard/deep band and forbids a second review-side risk model — source: `docs/prds/impl-review/effective.md` FR-9 · FR-10; `brainstorm:impl-review#D-4`
- **F-7**: the projected routing value is currently lost on both executor tracker reads; the planner exposes a 0–3 routing ladder while runtime adapters carry 0–4 arrays; Claude has a temporary all-task Sonnet-5/max override; and review invocation launches without model or effort — source: `internal/executor/ghtracker/tracker.go`; `internal/executor/carinatracker/carinatracker.go`; `internal/plangate/plan/reg_routing.go`; `internal/executor/runtime/{claude,codex,cursor,opencode}`; `internal/executor/reviewinvoke/reviewinvoke.go`
- **F-8**: plan-new already derives `Foundation` as task-DAG position (`root / has-dependents / leaf`) for proposal-risk classification; `Touches` exists only for overlap sequencing, while the glossary reserves `Blast radius` for a PRD amendment's declared impact tier — source: `docs/prds/plan-new/effective.md` §4.5 row 15 · FR-11 · FR-22; `internal/plangate/plan/gates.go`; `docs/wiki/glossary.md`

## Fog

none

## Out of scope

- **O-1**: scorecard-driven policy changes, self-tuning, and exploration traffic — VNEXT-7 successor work after this feature produces trustworthy outcomes (from D-17)
- **O-2**: automatic harness selection or cross-harness fallback — framing A resolves within an already-selected harness; fallback changes credentials/egress and belongs to VNEXT-7 (from D-17)
- **O-3**: routing for general authoring-stage skills — this increment serves plan-backed implementation and proposal review (from D-17)
- **O-4**: a unified expanded-risk/policy engine — VNEXT-9/VNEXT-10 remain separate programs; this feature consumes existing closed signals (from D-17)
- **O-5**: automatically treating a newly released model as qualified — harness mappings change only through reviewed data and conformance evidence (from D-17)
- **O-6**: native model IDs or effort settings in frozen plans — those remain harness-adapter data (from D-17)
- **O-7**: shadow or canary production rollout — after the pre-release test gate passes, routing activates directly in production (from D-17)

## Candidate glossary terms

- **T-1**: **execution demand** — the vendor-neutral capability, deliberation, context, and assurance requirements frozen for a model-backed task; resolved by a runtime adapter into native launch settings · avoid: bare *complexity* (it conflates cognitive difficulty, consequence, size, and runtime capacity)
- **T-2**: **impact scope** — the breadth of consumers or system behavior that can be affected if a task's change is wrong (`contained | multi_surface | system_wide`) · avoid: *blast radius* (already the PRD-amendment impact tier)
- **T-3**: **architectural role** — how central the changed surface is to the product architecture (`feature_local | shared_substrate | system_core`) · avoid: *Foundation* (already the task's dependency-graph position)
- **T-4**: **assurance floor** — the minimum scrutiny required by consequence and closed risk-policy signals (`ordinary | elevated | critical`), mechanically derived and planner-raise-only; the proposal classifier may raise it from the actual diff · avoid: bare *risk tier* (the merge lane owns proposal-risk classification)
- **T-5**: **execution profile** — the frozen tuple of minimum semantic requirements (`capability`, `deliberation`, `context`, `assurance`) derived from a task's assessment and resolved by one harness adapter into native launch settings · avoid: *rung* (collapses independent requirements and conflicts with the verification-ladder term)
- **T-6**: **harness adapter** — the Pyxis-owned `AgentRuntime` implementation for one external CLI (Codex, Cursor, Claude Code, or OpenCode), including semantic-profile→native-model/effort resolution · avoid: *harness owns routing* (the external CLI knows nothing about Pyxis demand fields)

## Parked technical constraints

- **P-1**: merged PRDs/designs cannot be edited in place; the chosen product direction requires typed amendments before successor technical designs — context: repository mutability rules and routing-envelope-v1's explicit successor boundaries
- **P-2**: native model and effort values differ by harness and change with the model roster; frozen plans must carry semantic requirements and policy versions, never product IDs — context: AP-1/AP-21 and the existing AgentRuntime boundary
- **P-3**: frozen v1 plans commonly omit a rung because absence is contractually valid; migration needs an explicit compatibility translation rather than rewriting old plans — context: plan-new FR-14 and routing-envelope-v1 DINV-1

## Gate

| Field | Value |
|---|---|
| **Verdict** | ready-for-prd |
| **Date** | 2026-07-26 |
| **Operator** | @mdarabi |
| **Covering source** | — |
| **Coverage check** | pass — all six inputs covered, zero blocking fog |
