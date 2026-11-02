# Amendment 002 — universal-review-floor

| Field | Value |
|---|---|
| **Amends** | planner-driven-model-routing *(chain through 001)* |
| **Status** | Approved |
| **Owner** | @mdarabi |
| **Date** | 2026-07-26 |
| **Learned-from** | operator-decision:design-brainstorm:planner-driven-model-routing-tech-design-2#D-1 |
| **Blast radius** | tier: contract-changing; touches: FR-13, FR-18, FR-25..FR-27, NR-2, NR-9, UB-5, UB-9, UB-12..UB-14, DOD-1..DOD-2, DOD-6..DOD-9, DOD-10..DOD-15, EVT-7, EVT-9..EVT-10, §0, §1, §3.2, §3.3, §4.3, §4.4, §5.1, §5.4, §5.5, §6.1, §6.2, §6.3, §6.4, §6.5, §6.7, §7.3, §8; reason: replace zero-pass proposal review with a universal model-backed floor, define testable mutually blind assurance passes and conservative aggregation, and make their depth, count, independence, and outcome observable end to end |
| **Template version** | amendment-template/v2 |

## Operations

### op: supersede FR-13

Reason: every implementation proposal must receive model-backed review, and higher planned assurance must add review depth or independent assurance passes rather than merely forbid a skip at the critical tier.

| ID | Requirement (EARS) | Rationale | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|
| FR-25 | When proposal review demand is computed, the review path shall join the actual `skip`, `standard`, or `deep` band projected by impl-review from the merge lane's risk class with the planned assurance under this closed floor: `ordinary` requires at least one `standard` model-backed assurance pass, `elevated` requires at least one `deep` model-backed assurance pass, and `critical` requires at least two mutually blind `deep` model-backed assurance passes; an actual `deep` band shall raise an `ordinary` pass to `deep`, and no actual band shall reduce planned depth or pass count. An assurance pass is a top-level review of one exact proposal revision and is distinct from any scope-locked remediation re-review inside that pass. | model-backed review is a universal quality floor, while greater consequence must monotonically add scrutiny without creating a second proposal-risk classifier or miscounting remediation as independent assurance | Must | critical | the complete assurance×band matrix yields `ordinary×skip→standard×1`, `ordinary×standard→standard×1`, `ordinary×deep→deep×1`, every `elevated` cell→`deep×1`, and every `critical` cell→`deep×2`; zero cells produce zero passes; every pass resolves through the review-role mapping; critical passes bind the same exact revision, use distinct top-level context identities, receive no other pass's session, candidates, findings, verdict, or output, and a remediation re-review never satisfies the second-pass requirement | Test + Demo | G1, G2, J3, F3 |

### op: add FR

Reason: multiple independent assurance passes need one fail-closed aggregation rule so disagreement, absence, and post-review mutation cannot be resolved by selecting the most favorable result.

| ID | Requirement (EARS) | Rationale | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|
| FR-26 | When more than one assurance pass is required, the review path shall keep the passes mutually blind until each has produced a revision-bound terminal result, union their surviving findings, and derive one aggregate result using the most restrictive pass verdict; any proposal change shall stale every pass and require the full assurance-pass set on the new exact revision before acceptance. | independent scrutiny is defeated if one pass can influence another, a favorable verdict can be cherry-picked, or one stale pass can certify a changed proposal | Must | critical | fixtures prove pass-order invariance, union all surviving findings, make any non-accepting or absent pass prevent aggregate acceptance, reject output leakage and favorable-verdict selection, and force every required pass to rerun after head change | Test + Failure injection | G1, G2, J3, F3 |

### op: supersede FR-18

Reason: strongest-profile fallback must preserve the live FR-25/FR-26 review process rather than cite superseded FR-13.

| ID | Requirement (EARS) | Rationale | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|
| FR-27 | Where the strongest-profile kill switch is engaged, each harness role shall prefer its strongest approved native profile only when that profile satisfies every requested semantic minimum; otherwise the system shall apply FR-10 before claim or invocation, while preserving FR-25's assurance-pass floor and FR-26's aggregation behavior. | fallback may overprovide but can never weaken the frozen contract or bypass required review process | Must | critical | kill-switch drills prove capability, deliberation, context, and assurance dominance for every resolved cell; unsatisfiable fixtures fail under FR-10; harness selection, assurance-pass depth/count, mutual blindness, and aggregate behavior remain unchanged | Test + Demo | G2, J2, J3, F2, F3 |

### op: supersede NR-2

Reason: the merge lane's classifier remains authoritative, but its lowest-risk output can no longer be interpreted as authorization for zero model-backed review.

| ID | Existing behavior protected | Why protected | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|
| NR-9 | The merge lane remains the sole classifier of actual proposal risk and keeps its lane/risk-class output unchanged; impl-review remains the sole projector of that class to the `skip`, `standard`, or `deep` review band; planner-driven routing consumes the projected band only through FR-25, where `skip` denotes the lowest band and never authorizes zero model-backed review. | planned assurance may join actual proposal risk but must neither relocate either existing predicate nor erase the universal review floor | critical | merge-lane lane/risk-class fixtures and impl-review class-to-band fixtures remain byte-for-byte unchanged; the complete FR-25 join suite consumes every projected band and clamps `skip` to at least `standard×1` | Test + Inspection | J3, FR-25 |

### op: supersede UB-5

Reason: failure handling must reject both a weakened review depth and a collapsed or missing independent-pass floor.

| ID | If… (trigger) | Then the system shall… | User sees | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|---|
| UB-12 | computed review depth or pass count falls below FR-25, a proposal reaches a zero-pass terminal, or required critical passes share a context/session/output lineage, review different revisions, or count a remediation re-review as an assurance pass | refuse review completion and require the complete correctly floored assurance-pass set before the proposal can be accepted | proposal remains unreviewed; the violated depth, pass count, revision binding, blindness, or pass-type condition is named | Must | critical | depth-lowering, zero-pass, single-pass-critical, shared-context, output-leakage, mismatched-revision, and remediation-miscount fixtures produce no accepted review evidence and name the exact violated floor | Test | FR-14, FR-25 |

### op: add UB

Reason: independent passes can validly disagree or fail, and the contract must forbid favorable-result selection or partial acceptance.

| ID | If… (trigger) | Then the system shall… | User sees | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|---|
| UB-13 | one required assurance pass is absent, stale, invalid, non-accepting, or disagrees with another pass | apply FR-26's most-restrictive aggregation, preserve every surviving finding and pass result, and prevent acceptance until the exact revision completes a satisfying full pass set | aggregate review remains non-accepting and names every contributing pass outcome | Must | critical | missing-pass, stale-pass, split-verdict, and conflicting-finding fixtures never accept from a favorable subset, preserve all pass evidence, and converge only after the full exact-revision set satisfies FR-25/FR-26 | Test + Failure injection | FR-25, FR-26 |

### op: supersede UB-9

Reason: post-cutover defect handling must cite the live kill switch and invalidate any affected multi-pass review process, not only individual routing attempts.

| ID | If… (trigger) | Then the system shall… | User sees | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|---|
| UB-14 | an effective mapping or native-profile qualification is found defective after production activation | engage FR-27's strongest-profile kill switch, stop new affected claims, invalidate every affected in-flight or unaccepted routing and review-process evidence artifact, and require re-execution or the complete FR-25/FR-26 assurance-pass set on the exact revision before acceptance | exact affected policy/profile versions, attempts, tasks, proposals, assurance passes, and review processes marked `rerun_required` | Must | critical | a seeded post-cutover qualification defect prevents acceptance of every affected open attempt and proposal until replacement evidence passes; all required assurance passes rerun on the exact head; unaffected routes continue | Failure injection | FR-20, FR-24..FR-27 |

### op: supersede DOD-1

Reason: completion must cover the live universal-review requirement and its failure row rather than their superseded predecessors.

| ID | Done criterion | Source | Verify | Required evidence | Blocking? |
|---|---|---|---|---|---|
| DOD-10 | Every live Must functional and failure requirement passes its own fit criterion. | FR-3..FR-12, FR-14..FR-15, FR-17, FR-20..FR-27, UB-2..UB-4, UB-6..UB-8, UB-10..UB-14 | Test + Eval + Demo + Inspection + Analysis + Failure injection | per-requirement evidence index linked from the proposal | yes |

### op: supersede DOD-2

Reason: non-regression evidence must protect the clarified merge-lane classifier boundary and universal review-floor interpretation.

| ID | Done criterion | Source | Verify | Required evidence | Blocking? |
|---|---|---|---|---|---|
| DOD-11 | No protected planner, harness-selection, merge-lane classifier, legacy-plan, empty-frontier, independent-review, or config-authorization behavior regresses. | NR-1, NR-3..NR-4, NR-6..NR-9 | Test + Inspection | non-regression suite, byte-identical merge-lane classification fixtures, and boundary inspection | yes |

### op: supersede DOD-7

Reason: cutover rehearsal must prove the full review-depth and independent-pass floor, not only absence of a critical skip.

| ID | Done criterion | Source | Verify | Required evidence | Blocking? |
|---|---|---|---|---|---|
| DOD-12 | The direct-cutover configuration and strongest-profile kill switch pass rehearsal without a harness switch, any FR-25 assurance-pass loss, any FR-26 aggregation change, or any semantic downgrade; an unsatisfiable fallback still fails before claim. | FR-10, FR-17, FR-25..FR-27, NR-1, NR-9 | Test + Demo | cutover and fallback drill record including ordinary×skip, conflicting critical passes, and critical×deep review cases | yes |

### op: supersede DOD-6

Reason: the review-process guardrail needs a typed production denominator and immutable proof, not only a pre-release narrative report.

| ID | Done criterion | Source | Verify | Required evidence | Blocking? |
|---|---|---|---|---|---|
| DOD-13 | Every §3.3 guardrail is clean before activation, including 100% of proposal review processes satisfying FR-25/FR-26 with an unambiguous EVT-9 terminal. | §3.3, M6, EVT-9 | Analysis + Test | pre-release decision report joined to the complete EVT-9 proposal denominator, including ordinary×skip and critical split-verdict cases | yes |

### op: supersede DOD-9

Reason: the qualification-defect drill must exercise the live UB-14 path and prove full assurance-pass invalidation and rerun behavior.

| ID | Done criterion | Source | Verify | Required evidence | Blocking? |
|---|---|---|---|---|---|
| DOD-14 | A post-cutover qualification-defect drill invalidates every affected open routing and review-process artifact, preserves unaffected work, reruns the complete required assurance-pass set on the exact head, and produces an operator brief for already accepted artifacts. | UB-10, UB-14, FR-25, FR-26, §3.3 | Failure injection + Inspection | defect-disposition drill record including one affected critical proposal and its replacement EVT-9 terminal | yes |

### op: supersede DOD-8

Reason: the operator audit must cite the live invalidation event and include the new aggregate review-process evidence.

| ID | Done criterion | Source | Verify | Required evidence | Blocking? |
|---|---|---|---|---|---|
| DOD-15 | Routing diagnostics and immutable git evidence are complete, connected, payload-free, and usable by the operator to explain one exact, one promoted, one unsupported, one legacy-origin, one aggregated review-process, and one invalidation case. | FR-20, FR-21, FR-24, NFR-4, EVT-1..EVT-6, EVT-8..EVT-10 | Demo + Inspection | six-case audit walkthrough plus committed event records | yes |

### op: add EVT

Reason: review depth, pass count, mutual blindness, and aggregate outcome must be immutable typed evidence for release and post-launch guardrails.

| ID | Event name | Fires when | Properties (typed) | Source | Feeds |
|---|---|---|---|---|---|
| EVT-9 | `review_process_completed` | the assurance-pass set for one exact proposal revision reaches an aggregate terminal, including a floor violation | `review_process_id:string`, `demand_id:string`, `task_id:string`, `proposal_id:string`, `reviewed_revision:string`, `planned_assurance:enum{ordinary,elevated,critical}`, `proposal_band:enum{skip,standard,deep}`, `effective_depth:enum{standard,deep}`, `required_passes:int`, `completed_passes:int`, `pass_attempt_ids:string[]`, `top_level_context_ids:string[]`, `pass_verdicts:enum[]`, `aggregate_verdict:enum`, `result:enum{satisfied,floor_violation,conflict_nonaccepting}`, `evidence_ref:string` | review orchestrator | M6, M5, §3.3 guardrail + health alert |

### op: supersede EVT-7

Reason: invalidation evidence must point to the live defect path and identify affected aggregate review processes as well as routing attempts.

| ID | Event name | Fires when | Properties (typed) | Source | Feeds |
|---|---|---|---|---|---|
| EVT-10 | `routing_evidence_invalidated` | UB-10 or UB-14 identifies evidence produced under a defective policy/profile | `policy_version:string`, `native_profile_id:string`, `attempt_ids:string[]`, `review_process_ids:string[]`, `artifact_refs:string[]`, `disposition:enum{rerun_required,operator_remediation}` | execution/review evidence owner | health alert + UB-10/UB-14 audit |

### op: supersede §0 Read to rule — the human reviewer's section

Reason: §0's informative routing must expose the newly approved universal review floor and exact assurance-to-process mapping.

*The body is the contract; this section only routes attention — on any conflict the body governs; machines ignore it.*

**Judge these**

| # | The call | Could have gone | If you disagree |
|---|---|---|---|
| 1 | Every `/implement-now` plan/v2 task carries the complete assessment and checked profile; there is no task-level `no_inference` arm | introduce a deterministic recipe/task contract inside the executor | the planner/executor boundary and review semantics change → FR-22, FR-23, NG-8, NR-8 |
| 2 | Plans freeze all six assessment dimensions **and** a checked four-part execution profile; no native model identifiers | one scalar complexity tier, raw dimensions only, or native settings in the plan | the planner/runtime boundary changes → §4.1, FR-3..FR-4, FR-22 |
| 3 | Harness adapters independently map implementation and review demand to the least-cost qualified native profile | one global model map or the external CLI interpreting Pyxis demand | portability, qualification ownership, and role separation change → FR-8..FR-12 |
| 4 | Every proposal receives model-backed review: ordinary is at least standard×1, elevated deep×1, and critical deep×2 mutually blind assurance passes; impl-review projects the merge lane's risk class to a band, which may only raise depth | permit ordinary×skip to terminate with zero review, leak one pass into another, or select the favorable result | the universal review floor, blindness, and aggregation change → FR-25, FR-26, NR-9, UB-12, UB-13 |
| 5 | Missing exact profiles promote upward or fail before work; they never downgrade or silently switch harnesses | clamp down or cross-harness fallback | judge-once routing, credentials, and egress boundaries change → FR-9, FR-10, NR-1 |
| 6 | Tested routing activates directly for every supported production path; there is no shadow or canary stage | staged production exposure | the operator's explicit rollout ruling changes → §6.5, FR-17 |

**Sanity-check the numbers** — 100% of `/implement-now` plan/v2 candidates carry complete demand contracts, zero under-routing, zero zero-pass proposal reviews, zero FR-25/FR-26 process violations, and at least three distinct native profiles appear in pre-release dogfood (§3.2–§3.3).

**The author's open questions** — none at product level; Q1–Q3 (§7.2) are design-owned mechanics bounded by requirements.

**Skip the rest** — structure/schema → docgate; substance/coherence → unmanned PRD review; implementation and mapping defects → technical-design, plan, conformance, and independent-implementation-review gates.

### op: supersede §1 Summary — the one-pager

Reason: the summary must state the universal review floor and consequence-scaled review process rather than only a critical no-skip guardrail.

- **Problem** — Planning can identify that tasks differ, but the execution path neither preserves enough of that judgment nor consistently turns it into implementation and review capacity. Routing data is optional or lost, runtime scales disagree, one worker path is temporarily pinned to a single strongest setting, and proposal review launches without an explicit model or effort.
- **For whom** — Primarily the model-backed planner→implementation→review path on every planned `/implement-now` task; secondarily the repository operator who must understand and constrain native model selection.
- **Why now** — The coarse routing seam already exists in the plan and runtime contracts, but its gaps now force hardcoding and silent defaults. Adaptive routing cannot learn safely until the factory can first produce, preserve, resolve, and observe an explicit execution-demand contract.
- **Direction** — The planner records a closed workload-and-consequence assessment and its deterministically derived, vendor-neutral execution profile for every plan/v2 task entering `/implement-now`. The already-selected harness adapter maps that minimum profile to the least-cost qualified implementation or review launch settings; it may promote visibly but may never downgrade or silently change harnesses. Every proposal receives at least one model-backed assurance pass; elevated assurance adds depth, while critical assurance requires two mutually blind deep passes aggregated conservatively under FR-25/FR-26.
- **Appetite** — One vertical routing-contract increment, including legacy translation, role-specific mappings, complete telemetry, conformance, dogfood, and a strongest-profile kill switch; no self-tuning, fallback, general authoring routing, automatic model qualification, or deterministic executor recipe language.
- **Success** — At the pre-release checkpoint, 100% of `/implement-now` plan/v2 candidates carry valid evidence-backed assessments and derived profiles, the full required routing domain conforms, every proposal has an EVT-9-proven FR-25/FR-26 review process, zero dispatches under-route, and dogfood proves at least three distinct native profiles with verified outcomes.
- **Guardrail** — No proposal reaches a zero-pass terminal, no review depth, pass count, blindness, or aggregate result weakens FR-25/FR-26, no frozen v1 plan becomes unexecutable, and an empty execution frontier still launches no model.

### op: supersede §3.2 Goals → signals → metrics

Reason: the universal review-process floor needs its own complete proposal denominator and typed evidence path rather than being implied by per-attempt routing metrics.

| Metric | Goal | Signal | Baseline | Target (at checkpoint) | Feeds from |
|---|---|---|---|---|---|
| M1 (primary) | G1 | valid model-backed demand contracts ÷ every plan/v2 task presented for `/implement-now` freeze | contract absent | 100% at pre-release acceptance | EVT-6 |
| M2 | G1, G2 | requested profiles whose non-assurance components are field-identical to their frozen plan/v2 or translated plan/v1 demand and whose assurance is exactly that origin floor for implementation or its FR-25 join for review, before any visible native promotion; dispatch then satisfies every component | not measurable; current paths can lose, substitute, or override routing | 100% exact demand transport/join and satisfying dispatch; zero unauthorized substitution or downward resolution in conformance and dogfood | EVT-1, EVT-2, EVT-3, EVT-8 |
| M3 | G2 | required harness × role × derivable-profile cells producing their specified satisfying or unsupported result under adapter conformance | no common conformance proof | 100% of the required routing domain before release | EVT-4 |
| M4 | G2 | distinct native launch profiles exercised by verified dogfood tasks | implementation can converge on one pin | at least three before production activation | EVT-2, EVT-5 |
| M5 | G3 | terminal routing and review-process attempts with a complete evidence chain: frozen plan/v2 demand or translated plan/v1 origin→unsupported result, or origin→resolved route→worker outcome; proposal reviews additionally join to one aggregate EVT-9 | incomplete and fragmented | 100% at pre-release acceptance and in production | EVT-1, EVT-2, EVT-3, EVT-5, EVT-8, EVT-9 |
| M6 | G1, G2 | exact proposal revisions whose single EVT-9 terminal proves effective depth, required/completed assurance-pass count, distinct mutually blind top-level contexts, and conservative aggregation ÷ every implementation proposal | zero-pass review permitted and process evidence absent | 100% satisfied; zero missing, floor-violating, or favorably cherry-picked terminals before activation and in production | EVT-9 |
- **Top risks** — A plausible but wrong planner assessment under-routes work; a stale native mapping overstates a model's qualification; transport or review code silently discards the frozen demand.

### op: supersede §3.3 Guardrails and decision rule

Reason: the release guardrail must reject every zero-pass or under-floor review, not only a skipped critical review.

| Guardrail metric | Threshold | Action if breached |
|---|---|---|
| Downward resolution of any execution-profile component | zero | block release; after activation, engage the strongest-profile kill switch, stop new routed claims, and disposition affected evidence under UB-10/UB-14 |
| Proposal review below FR-25's minimum depth, pass count, or mutual-blindness floor, or aggregated contrary to FR-26 | zero | block/rollback immediately; invalidate affected review evidence and rerun the complete assurance-pass set on the exact revision |
| Frozen v1 plan made unexecutable or rewritten | zero | block release; restore compatibility translation before proceeding |
| Routing-selected task fails verification because its launch profile was under-qualified | zero in required conformance and pre-release dogfood | block activation; after activation quarantine the mapping, use the strongest-profile fallback, and disposition affected evidence under UB-10/UB-14 |

**Ship rule (pre-registered):** M1–M6 meet their targets, every guardrail is clean, and §6.4 is green → activate routing directly in production. Any guardrail breach → no activation, or immediate strongest-profile fallback and FR-25/FR-26-compliant re-review after activation.

### op: supersede §4.3 Experience: flows and states

Reason: the implementation and review flows must cite live failures and show band projection, the universal assurance-pass floor, and conservative aggregation as separate states.

| Flow | Happy path | States | Failure coverage |
|---|---|---|---|
| F1 — plan and freeze | planner records all workload and consequence dimensions with evidence; policy derives a matching profile | unassessed → assessed → validated/frozen | UB-11, UB-2 |
| F2 — implementation | executor reads frozen demand; selected implementation mapping resolves the least-cost qualified native profile | claimed → resolved/promoted → launched → verified | UB-3, UB-4, UB-6 |
| F3 — independent review | reviewer reads frozen demand; impl-review projects the merge lane's risk class to a proposal band; FR-25 joins the band with planned assurance; the review-role mapping resolves every required assurance pass independently; FR-26 aggregates the terminal pass set | proposal-ready → band-projected → floored → assurance-pass set → aggregated → reviewed | UB-4, UB-6, UB-12, UB-13 |
| F4 — operator audit/override | operator inspects the derivation and native result, then authorizes a constrained repository override if needed | observed → proposed → authorized → active | UB-7 |
| F5 — legacy execution | frozen v1 routing translates through the versioned compatibility table without rewriting the plan and emits a typed legacy origin | legacy-read → translated/recorded → resolved | UB-8 |

### op: supersede §4.4 Scenarios

Reason: scenarios must demonstrate an actual-risk raise from the ordinary floor and the complete critical multi-pass lifecycle rather than imply that elevated assurance starts below deep.

- **Novel but contained implementation.** A task requires first-of-kind reasoning and adversarial verification while touching a feature-local surface. Its profile demands strong capability and deep deliberation without falsely declaring system-wide consequence; the selected harness resolves it to its own qualified settings.
- **Simple change on system core.** A mechanically straightforward edit targets a system-core policy surface. Consequence floors raise assurance and the named interaction rules prevent a cheap cognitive classification from lowering scrutiny.
- **Review after a riskier-than-planned diff.** Planned assurance is ordinary, but impl-review projects the finished proposal's merge-lane risk class to `deep`. FR-25 raises the proposal from `standard×1` to `deep×1`; the review-role mapping still selects independently from the implementation worker.
- **Critical review with disagreement and remediation.** A critical-assurance proposal starts two mutually blind top-level deep assurance passes on the same exact head revision. Each pass receives a distinct context and none of the other's session or output. FR-26 unions their surviving findings and selects the most restrictive verdict. If remediation changes the head, every prior pass becomes stale and the complete two-pass set reruns; an internal remediation re-review never counts as the second assurance pass.
- **No exact native profile.** A harness cannot meet the tuple exactly but has a stronger qualified profile. Resolution promotes and records the reason. If no satisfying profile exists, execution stops before claim or model invocation and names the unmet component.
- **Frozen v1 plan with no rung.** The reader classifies it as legacy-unassessed and applies the conservative compatibility profile without modifying the plan artifact.

**Decision:** Solution gate — this shape at this appetite is the bet. Zero product-level TBDs may pass.

| Reviewer | Role | Date | Verdict |
|---|---|---|---|
| @mdarabi | operator | — | — *(via the introducing PR's operator review)* |

### op: supersede §5.1 Functional requirements

Reason: the normative assurance vocabulary and live requirement tables must describe the same exact floor, ownership seams, assurance-pass definition, and aggregation behavior.

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
| `assurance` | `ordinary` — the lowest semantic assurance, mapped only through FR-25 and never authorization for zero review; `elevated` — higher semantic assurance, mapped through FR-25 to at least one deep assurance pass; `critical` — the highest semantic assurance, mapped through FR-25 to two mutually blind deep assurance passes aggregated by FR-26 |

The **required routing domain** is every harness in the existing `AgentRuntime` contract—Claude Code, Codex, Cursor CLI, and OpenCode—crossed with implementation and review roles and every execution profile the derivation policy can emit. A later harness added to that runtime contract joins this domain automatically. Each cell must resolve to a qualified satisfying native profile or to the explicit unsupported result required by FR-10; a harness or derivable profile cannot disappear from conformance by omission.

| ID | Requirement (EARS) | Rationale | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|
| FR-22 *(001 ← FR-1)* | When the planner freezes a plan/v2 task for `/implement-now`, the planner shall record `reasoning`, `coupling`, `novelty`, `verification`, `impact_scope`, and `architectural_role` using the normative assessment vocabulary. | every executor task needs complete inputs so dispatch cannot fall through to an absent, cheapest, or hardcoded route | Must | critical | every `/implement-now` plan/v2 golden carries exactly one accepted value for every field; missing and unknown values fail validation | Test + Eval (EVAL-1) | G1, J1, F1 |
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
| FR-25 *(002 ← FR-13)* | When proposal review demand is computed, the review path shall join the actual `skip`, `standard`, or `deep` band projected by impl-review from the merge lane's risk class with the planned assurance under this closed floor: `ordinary` requires at least one `standard` model-backed assurance pass, `elevated` requires at least one `deep` model-backed assurance pass, and `critical` requires at least two mutually blind `deep` model-backed assurance passes; an actual `deep` band shall raise an `ordinary` pass to `deep`, and no actual band shall reduce planned depth or pass count. An assurance pass is a top-level review of one exact proposal revision and is distinct from any scope-locked remediation re-review inside that pass. | model-backed review is a universal quality floor, while greater consequence must monotonically add scrutiny without creating a second proposal-risk classifier or miscounting remediation as independent assurance | Must | critical | the complete assurance×band matrix yields `ordinary×skip→standard×1`, `ordinary×standard→standard×1`, `ordinary×deep→deep×1`, every `elevated` cell→`deep×1`, and every `critical` cell→`deep×2`; zero cells produce zero passes; every pass resolves through the review-role mapping; critical passes bind the same exact revision, use distinct top-level context identities, receive no other pass's session, candidates, findings, verdict, or output, and a remediation re-review never satisfies the second-pass requirement | Test + Demo | G1, G2, J3, F3 |
| FR-26 *(002)* | When more than one assurance pass is required, the review path shall keep the passes mutually blind until each has produced a revision-bound terminal result, union their surviving findings, and derive one aggregate result using the most restrictive pass verdict; any proposal change shall stale every pass and require the full assurance-pass set on the new exact revision before acceptance. | independent scrutiny is defeated if one pass can influence another, a favorable verdict can be cherry-picked, or one stale pass can certify a changed proposal | Must | critical | fixtures prove pass-order invariance, union all surviving findings, make any non-accepting or absent pass prevent aggregate acceptance, reject output leakage and favorable-verdict selection, and force every required pass to rerun after head change | Test + Failure injection | G1, G2, J3, F3 |
| FR-14 | When review is required, the review runtime shall resolve the joined demand through the selected harness's review-role mapping independently of the implementation selection. | fresh independent review needs its own qualified capacity | Must | critical | fixtures with differing role mappings launch the declared review profile and never reuse an implementation decision implicitly | Test | G2, J3, F3 |
| FR-15 | When a frozen v1 task is read, the system shall translate its explicit rung through a versioned compatibility table or classify an absent rung as `legacy_unassessed` with frontier capability, maximum deliberation, extended context, and an assurance floor that existing risk signals may raise. | old plans allowed no rung and cannot be rewritten | Must | high | rung fixtures translate exactly; absent-rung fixtures receive the conservative profile; source bytes remain unchanged | Test | G1, J2, F5 |
| FR-24 *(001 ← FR-16)* | For every implementation and review routing attempt, the system shall record either the frozen plan/v2 assessment, derived profile, evidence citations, and policy version or a typed no-rewrite plan/v1 origin containing the original rung presence/value, compatibility-table version, translation classification, and translated profile; it shall also record the selected harness and role, connected task/proposal identity, and an `exact`, `promoted`, or `unsupported` resolution, with native profile identity on resolved routes. | operators and successor routing need one complete routing decision without pretending legacy plans contain fields they never froze | Must | high | every attempt links by `demand_id` to exactly one EVT-1 or EVT-8 origin and by `attempt_id` to exactly one EVT-2 or EVT-3 terminal; plan/v1 explicit-rung, absent-rung, retry, and review fixtures remain unambiguous, every EVT-2 has a native profile, every EVT-3 has none, and source plan bytes remain unchanged | Test + Analysis | G1, G3, J4, F2, F3, F4, F5 |
| FR-17 | When all pre-release criteria are green, the release shall activate planner-driven routing for every supported production implementation and review path without a shadow or canary stage. | direct cutover is the operator-approved rollout | Must | critical | one production configuration enables the validated policy for the full declared support matrix; no staged cohort logic exists | Inspection + Demo | G1, G2, J2, J3 |
| FR-27 *(002 ← FR-18)* | Where the strongest-profile kill switch is engaged, each harness role shall prefer its strongest approved native profile only when that profile satisfies every requested semantic minimum; otherwise the system shall apply FR-10 before claim or invocation, while preserving FR-25's assurance-pass floor and FR-26's aggregation behavior. | fallback may overprovide but can never weaken the frozen contract or bypass required review process | Must | critical | kill-switch drills prove capability, deliberation, context, and assurance dominance for every resolved cell; unsatisfiable fixtures fail under FR-10; harness selection, assurance-pass depth/count, mutual blindness, and aggregate behavior remain unchanged | Test + Demo | G2, J2, J3, F2, F3 |
| FR-23 *(001 ← FR-19)* | When the planner freezes a plan/v2 task for `/implement-now`, the planner shall record at least one resolvable evidence citation supporting its assessment values. | the assessment must preserve why every executor task was classified, not only its labels | Must | high | uncited or unresolved evidence fails plan validation before freeze; EVAL-1 proves that every recorded assessment value is supported by at least one cited anchor and rejects irrelevant, contradictory, or only partially supporting citation sets without requiring a distinct citation per axis | Test + Eval (EVAL-1) | G1, J1, J4, F1, F4 |
| FR-20 | When a resolved implementation or review route reaches a terminal worker state, the system shall connect exactly one `verified`, `failed`, or `blocked` outcome and its available usage measurements to that routing decision. | resolved routes need an achievable outcome chain distinct from pre-claim failures | Must | high | every EVT-2 in conformance and dogfood joins exactly one EVT-5; EVT-3 terminal failures require no EVT-5 | Test + Analysis | G3, J4, F2, F3, F4 |
| FR-21 | When a routing attempt reaches its terminal resolution or worker outcome, the system shall retain its bounded decision/outcome evidence as an immutable, append-only event record in git. | routing judgments and failures are the improvement fuel for successor policy | Must | high | exact, promoted, unsupported, verified, failed, and blocked fixtures each create a new committed evidence record; reruns never overwrite an earlier record; records contain no prompt or code payload | Test + Inspection | G3, J4, F4 |

**Non-regression (NR-n)**

| ID | Existing behavior protected | Why protected | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|
| NR-1 | Harness selection remains an upstream explicit decision; routing never switches harnesses. | credentials, egress, and availability policy are outside this feature | critical | all unsupported-profile fixtures retain the selected harness and end in FR-10 | Test | NG-2, FR-10 |
| NR-9 *(002 ← NR-2)* | The merge lane remains the sole classifier of actual proposal risk and keeps its lane/risk-class output unchanged; impl-review remains the sole projector of that class to the `skip`, `standard`, or `deep` review band; planner-driven routing consumes the projected band only through FR-25, where `skip` denotes the lowest band and never authorizes zero model-backed review. | planned assurance may join actual proposal risk but must neither relocate either existing predicate nor erase the universal review floor | critical | merge-lane lane/risk-class fixtures and impl-review class-to-band fixtures remain byte-for-byte unchanged; the complete FR-25 join suite consumes every projected band and clamps `skip` to at least `standard×1` | Test + Inspection | J3, FR-25 |
| NR-3 | Frozen v1 plans remain immutable and executable. | merge freezes plan artifacts | high | FR-15 compatibility fixtures execute with zero source mutation | Test | J2, F5 |
| NR-4 | Existing general authoring-stage model routing remains governed by its current policy. | authoring routing is outside this increment | medium | inspection finds no new execution-demand dependency in non-plan authoring paths | Inspection | NG-3 |
| NR-6 | Independent review retains fresh context and reviewer separation. | model selection must not weaken review independence | critical | existing impl-review independence/conformance suites remain green | Test | J3, FR-14 |
| NR-7 | The existing repository-config policy authorization and closed-parse guarantees remain in force for routing overrides. | a routing override is critical governing policy, not an ordinary worker-authored preference | critical | an operator-authorized override can become effective; contributor-only, worker-authored, unknown-key, and unauthenticated changes cannot; existing config-policy suites remain green | Test + Inspection | J4, F4, FR-12 |
| NR-8 *(001)* | An `/implement-now` run with no ready task performs no model invocation. | every executable task is model-backed, but an empty frontier remains zero-work and must not spend inference | high | empty-frontier fixtures emit zero model-launch events and zero task claims | Test | NG-8 |

### op: supersede §5.4 Data

Reason: review-process conformance requires a durable entity that binds the proposal revision, required passes, isolated contexts, per-pass outcomes, and aggregate result.

| Entity / field | Meaning | Source of truth | Retention | Integrity rules | Privacy class |
|---|---|---|---|---|---|
| Task assessment | closed workload and consequence values plus resolvable evidence citations | frozen plan ledger | with the plan | complete on every plan/v2 task projected to `/implement-now`; immutable after freeze | internal |
| Execution profile | vendor-neutral minimum tuple and derivation-policy version | mechanically validated frozen plan | with the plan | present for every `/implement-now` plan/v2 task; reproducible from assessment; no native product identifiers | internal |
| Native routing policy | qualified harness×role mappings and optional repository overrides | released defaults plus validated repo policy | current policy plus normal repository history | closed schema; version compatible; no weakening | internal |
| Routing decision | exact/promoted/unsupported resolution and native profile identity where resolved | immutable git event record; runtime event stream is its transport | forever in repository history | connected to frozen plan/v2 demand or typed plan/v1 translation, policy, harness, and role; a prior record is never overwritten | internal |
| Routing outcome | completion, verification/review result, and usage measurements already exposed by the harness | immutable git event record connected to the routing decision | forever in repository history | exactly one terminal outcome per resolved route; never used if orphaned or invalidated | internal |
| Review process | exact proposal revision, planned assurance, projected proposal band, required effective depth/pass count, top-level pass/context identities, per-pass verdicts, and aggregate result | immutable EVT-9 event backed by the review report | forever in repository history | exactly one terminal per process; every required pass reviews the same exact head in a distinct mutually blind top-level context; pass and aggregate evidence is append-only; a head change stales the complete prior set | internal |

- **Migration** — No frozen artifact is rewritten. Plan/v2 adopts the required model-backed demand contract for every task projected to `/implement-now`; frozen v1 reads through FR-15's versioned translator and emits FR-24's typed legacy-origin record.
- **Parallel-run / cutover** — No production parallel-run, shadow, or canary path exists. Conformance and dogfood run before the one direct cutover.
- **Rollback** — The strongest-profile kill switch changes native resolution only; semantic plan data and emitted decisions remain readable. UB-14 invalidates affected unaccepted routing and review-process evidence, and UB-10 routes already accepted work to explicit human disposition rather than destructive rollback.
- **Decommission** — The hardcoded implementation pin, incompatible numeric ladder, and implicit review launch settings are removed only after the direct-cutover gate and fallback drill pass.
- **Privacy check** — No DPIA trigger: the feature adds operational classification and routing metadata, not personal or sensitive-data profiling.
- **Egress** — The already-selected harness remains the only model boundary. This feature changes its native launch profile but adds no provider, no cross-harness fallback, and no new source-content egress; routing telemetry records metadata rather than prompt or code content.

### op: supersede §5.5 Interfaces and integrations

Reason: ownership must preserve the merge lane's risk classification, impl-review's band projection, the review orchestrator's assurance-pass lifecycle, and the harness adapter's native resolution as distinct seams.

| Interface | Direction | Purpose | Contract | Failure modes | Owner |
|---|---|---|---|---|---|
| Planner → frozen plan | out | publish assessment and derived demand for every `/implement-now` task | versioned closed plan schema; complete assessment and profile | missing/unknown values, derivation mismatch | plan-new |
| Frozen plan → tracker/executor | out | preserve demand to claim and dispatch | lossless versioned projection/read | field loss, legacy-version mismatch | plan projection + tracker adapters |
| Executor → harness adapter | out | resolve implementation demand for the selected harness | qualified implementation-role mapping; exact/promoted/unsupported result | missing cell, incompatible override, no satisfying profile | runtime adapters |
| Merge lane → impl-review band projector | in | supply the existing actual proposal lane/risk class and project it to `skip`, `standard`, or `deep` | merge-lane classification remains unchanged; impl-review solely owns class-to-band projection | missing class, changed classification semantics, invalid band | merge lane + impl-review |
| Review orchestrator → harness adapter | out | resolve each FR-25 assurance pass independently after joining the projected band with planned assurance | qualified review-role mapping; exact/promoted/unsupported result per pass | attempted lowering, shared context, missing cell, no satisfying profile | impl-review + runtime adapters |
| Review process → evidence stream | out | preserve exact-revision pass provenance, outcomes, and conservative aggregate | one typed EVT-9 terminal joined to immutable pass/report evidence | absent/stale pass, output leakage, partial write, favorable-subset aggregation, schema mismatch | impl-review + execution/review evidence owner |
| Runtime/review → evidence stream | out | connect route, promotion/failure, and worker outcome | typed events joined by task/proposal and attempt identity | orphan, partial write, schema mismatch | execution/review runtime |

### op: supersede §6.2 Dependencies

Reason: sibling contract changes must explicitly reconcile impl-review's existing remediation rounds and skip vocabulary with the new top-level assurance-pass floor.

| Dependency | Type | Needed by | Status | Owner |
|---|---|---|---|---|
| Typed amendments to the frozen plan-new, implement-now, and impl-review contracts | approval / document system | Phase A design handoff | required (`brainstorm:planner-driven-model-routing#P-1`); the impl-review amendment must preserve merge-lane classification, retain impl-review's class-to-band projection, distinguish its existing scope-locked remediation re-reviews from top-level assurance passes, replace zero-review `skip` terminal behavior, and extend the report/evidence schema for FR-25/FR-26 | @mdarabi + factory |
| Existing plan projection and tracker-adapter boundary | system | Phase A | available; known transport defects | plan/executor owners |
| Existing shared runtime port and supported harness registry | system | Phase B | available; native policies incomplete/inconsistent | runtime owners |
| Existing merge-lane proposal risk class and impl-review class-to-band projection | system | Phase B | available; ownership and output semantics protected by NR-9 | review/merge-lane owners |
| Qualified native model roster and cost metadata per harness/role | vendor / policy | Phase B and release | design-owned; evidence required by FR-12/EVAL-2 | @mdarabi + runtime owners |

### op: supersede §6.4 Release criteria

Reason: release proof must include the complete proposal denominator, typed review-process evidence, and the live replacement DoD rows.

| Lens | Criterion | Evaluated by |
|---|---|---|
| Functionality | Every live Must FR and UB fit criterion passes; M1, M3, and M6 reach target over their complete denominators. | CI, eval, conformance matrix, EVT-9 process report |
| Usability | Operator explains exact, promoted, unsupported, and joined/aggregated review decisions from emitted evidence without reading adapter code. | DOD-15 audit walkthrough |
| Reliability | Zero §3.3 guardrail breaches across conformance, adversarial fixtures, dogfood, direct-cutover rehearsal, and kill-switch rehearsal; the post-defect disposition drill and universal-review process gate pass. | test/eval reports, DOD-13 decision report, DOD-14 drill record |
| Performance / cost | Resolution remains local and deterministic; every successful selection is the least-cost qualified satisfying profile. | NFR-1, NFR-5, NFR-6 suites |
| Supportability | Named failures identify harness, role, policy, task, unmet component, and any review-process violation; strongest-profile fallback and runbook are operator-approved. | diagnostic goldens, runbook review, DOD-12 |

### op: supersede §6.1 Phasing and milestones

Reason: the resolution phase and release proof must cite the live universal-review requirement and replacement DoD rows.

| Phase | Purpose | Scope | Exit criterion |
|---|---|---|---|
| A — contract | make demand explicit and mechanically trustworthy | plan/v2 vocabulary, evidence-backed assessment, derivation, validation, transport, legacy translation | FR-3..FR-7, FR-15, FR-22, and FR-23 green; EVAL-1 green |
| B — resolution | complete implementation and review consumption | required-domain mappings, overrides, resolver, universal review floor and process join, failure paths, immutable evidence | FR-8..FR-12, FR-14, FR-20, FR-21, FR-24..FR-27 and EVT-9 adapter/review-process conformance green |
| C — pre-release proof | falsify under-routing and under-review before any production activation | full non-regression, failure/abuse suites, dogfood diversity, operator audit, kill-switch rehearsal | M1–M6 and every §3.3 guardrail green; DOD-3..DOD-5 and DOD-10..DOD-15 evidenced |
| D — production | replace hardcoded/implicit routing in one cutover | every supported implementation and review path | direct activation succeeds; production events remain connected and guardrails clean |

### op: supersede §6.3 Risks, assumptions, and rabbit holes

Reason: review-routing risk must name the universal floor and the independence requirement while preserving the merge lane as the sole classifier.

| # | Risk / assumption / rabbit hole | Likelihood | Impact | Mitigation or test | Owner |
|---|---|---|---|---|---|
| R1 | A semantically plausible but wrong planner assessment passes structural validation. | M | H | EVAL-1 adversarial corpus, evidence citations, mechanical consequence floors, human plan checkpoint | plan-new owner |
| R2 | A native mapping goes stale as harness model catalogs change. | H | H | closed versioned mappings, operator authorization, qualification evidence, unsupported failure, EVAL-2, strongest-profile fallback | runtime owners |
| R3 | Transport fixes one tracker path but leaves another dropping fields. | M | H | consumer-side round-trip conformance for every declared tracker path (FR-7/UB-3) | executor owners |
| R4 | Review routing accidentally relocates risk or band ownership, permits zero review, collapses critical independence, or accepts a favorable subset. | M | H | preserve merge-lane classification and impl-review band projection (NR-9); apply only FR-25's closed join; reject weakened depth, pass count, provenance, or aggregation under UB-12/UB-13 | merge-lane + impl-review owners |
| R5 | Direct activation exposes a mapping defect broadly. | M | H | all-path pre-release proof, fail before claim, strongest-profile kill switch rehearsed before cutover, UB-14 invalidation, UB-10 remediation brief | @mdarabi |
| R6 | The feature expands into adaptive scorecards or cross-harness policy and exceeds the appetite. | M | M | enforce NG-1/NG-2 and route successor work to WR-1/WR-2 | @mdarabi |
| R7 | Legacy translation becomes a second mutable inference policy. | L | H | versioned explicit translation table, conservative absent-rung sentinel, no source rewrite | plan/executor owners |

### op: supersede §6.5 Rollout and reversibility

Reason: direct cutover and rollback must rehearse and preserve the universal review-process floor across all assurance levels.

- **Flag** — one production routing-policy switch, owned by @mdarabi, changes native resolution only after Phase C is green.
- **Kill switch** — strongest-profile fallback per harness role; tested before release; preserves selected harness, every semantic minimum, FR-25's assurance-pass depth/count, FR-26 aggregation, and evidence; an unsatisfiable demand still fails under FR-10.
- **Experiment** — none; this increment makes no adaptive or statistical-optimization claim.
- **Cutover/migration steps** — validate policy and overrides; pass the complete support matrix, evals, review-process conformance, dogfood, and fallback drill; activate all supported implementation and review paths together; remove the temporary hardcoded/implicit paths only after successful activation.

| Stage | Audience | Minimum evidence | Advance when | Roll back when |
|---|---|---|---|---|
| Pre-release only | conformance fixtures and repository dogfood; no production routing | DOD-3..DOD-5 and DOD-10..DOD-15 plus §6.4 evidence | every metric target and guardrail is green | any failed criterion blocks activation |
| 100% production | every required implementation and review path | successful direct-cutover rehearsal and effective-policy snapshot | post-launch trigger reached with M5 and M6 complete and guardrails clean | any under-routing, FR-25/FR-26 review-process violation, qualification defect, or evidence-integrity breach engages the kill switch, stops affected claims, and invokes UB-10/UB-14 plus a full exact-revision compliant assurance-pass set |

### op: supersede §6.7 Post-launch review

Reason: the post-launch trigger must exercise the critical two-mutually-blind-deep-pass process, not merely one deep review.

- **Review trigger** — production has completed verified attempts using at least three distinct native profiles, including one critical-assurance proposal with two mutually blind deep top-level assurance passes on the same exact revision and complete immutable M5/M6 evidence through EVT-9 · **Owner** — @mdarabi
- **Verdict recorded** — adopt / iterate / kill, with M1–M6 and guardrail evidence — **on the header's Tracking ref**. Contract changes travel as typed amendments; adaptive-policy proposals route to WR-1.

### op: supersede §7.3 Decision log

Reason: the product decision ledger must preserve the operator's universal-review-floor ruling and its exact process mapping.

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
| 2026-07-26 | Every implementation receives model-backed proposal review and actual proposal risk may only raise the floor. (`design-brainstorm:planner-driven-model-routing-tech-design-2#D-1`) Amendment 002 resolves the previously open exact mapping as ordinary `standard×1`, elevated `deep×1`, and critical `deep×2` mutually blind top-level assurance passes with conservative aggregation. | permit ordinary×skip to terminate with zero review; use one uniform review process; count remediation re-review as an independent pass; allow favorable-result selection | D-1 establishes the universal monotone direction; the operator-approved amendment supplies the exact process contract so greater consequence adds independently testable scrutiny | @mdarabi |

### op: supersede §8 Contract readiness — the executability gate

Reason: readiness references must point to the live review-floor, failure, non-regression, and DoD rows after this amendment.

| Check | Pass? | Notes |
|---|---|---|
| Problem basis accepted — §1 problem accepted (T1); §2 problem gate signed (T2/T3) | ☐ | pending introducing PR operator review |
| Zero solution-blocking TBDs or open questions (§7.2) | ☑ | Q1–Q3 are design mechanics with product behavior fixed by cited requirements |
| Every live Must FR has a fit criterion and a verification method (§5.1) | ☑ | FR-3..FR-12, FR-14..FR-15, FR-17, and FR-20..FR-27 carry both |
| Every requirement row carries a risk tier; high/critical rows have a declared safety plan | ☑ | fail-before-claim behavior, pre-release full-domain proof, universal model-backed review, direct-cutover and unsatisfiable-fallback rehearsal, UB-10/UB-14 post-defect disposition, UB-12/UB-13 floor and aggregation enforcement, operator-authorized policy, immutable EVT-9 process evidence, no-rewrite migration, and empty-frontier non-regression cover the high/critical surfaces |
| Non-goals drawn (§3.4); non-regression rows named (§5.1) | ☑ | NG-1..NG-8, WR-1..WR-3, NR-1, NR-3..NR-4, and NR-6..NR-9 |
| Every primary job (§2.3) is covered by a requirement, a non-goal, or a waiting-room entry | ☑ | J1→FR-3..FR-7/FR-22/FR-23; J2→FR-8..FR-12/FR-15/FR-27; J3→FR-14/FR-25/FR-26/FR-27; J4→FR-12/FR-20/FR-21/FR-23/FR-24/NR-7 |
| Every key flow (§4.3) has functional, failure, and non-regression coverage (§5.1–5.2) | ☑ | F1→FR-3..FR-6/FR-22/FR-23/UB-2/UB-11; F2→FR-7..FR-12/FR-20/FR-24/FR-27/UB-3..UB-4/UB-6/UB-10/UB-14/NR-1; F3→FR-7..FR-12/FR-14/FR-20/FR-24..FR-27/UB-4/UB-6/UB-10/UB-12..UB-14/NR-6/NR-9; F4→FR-12/FR-20..FR-24/UB-4/UB-7/UB-10/UB-14/NR-7; F5→FR-15/FR-24/UB-8/NR-3; empty frontier→NR-8 |
| Every primary metric (§3.2) has instrumentation (§5.7) or an explicit measurement plan | ☑ | M1→EVT-6; M2→EVT-1/EVT-8 plus EVT-2/EVT-3; M3→EVT-4; M4→EVT-2/EVT-5; M5→EVT-1 or EVT-8 plus terminal EVT-3 or EVT-2+EVT-5, with proposal review joined to EVT-9; M6→EVT-9 over the complete proposal denominator |
| Every blocking DoD row traces to a requirement, guardrail, release criterion, or human-judgment rubric (§5.8) | ☑ | DOD-3..DOD-5 and DOD-10..DOD-15 each cite reviewed source rows |
| Risks and assumptions have owners (§6.3) | ☑ | R1..R7 owned |
| Human-judgment criteria name an owner and a rubric (§5.8) | ☑ | DOD-3 names @mdarabi and the three-part qualification rubric |
| DoD complete: every blocking row sourced and evidenced; every n/a explained (§5.8) | ☑ | all live DoD rows (DOD-3..DOD-5, DOD-10..DOD-15) block and name evidence; §6.6 n/a rows state the absent surface |
