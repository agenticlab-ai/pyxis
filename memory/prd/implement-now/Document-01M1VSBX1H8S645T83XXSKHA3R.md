# Amendment 001 — consume-execution-demand

| Field | Value |
|---|---|
| **Amends** | implement-now *(chain through 000)* |
| **Status** | Approved |
| **Owner** | @mdarabi |
| **Date** | 2026-07-27 |
| **Learned-from** | policy-change:prd:planner-driven-model-routing/FR-7 · policy-change:prd:planner-driven-model-routing/FR-8 · policy-change:prd:planner-driven-model-routing/FR-10 · policy-change:prd:planner-driven-model-routing/FR-15 · policy-change:prd:planner-driven-model-routing/FR-17 |
| **Blast radius** | tier: contract-changing; touches: FR-6, FR-21..FR-27, NR-5..NR-7, UB-9..UB-13, ABUSE-5, NFR-7..NFR-8, EVT-2, EVT-5, EVT-7..EVT-11, DOD-1..DOD-4, DOD-6, DOD-7, DOD-10..DOD-18, EVAL-1..EVAL-3, §0, §1, §3.2, §3.3, §4.1, §4.2, §4.3, §4.4, §5.1, §5.4, §5.5, §5.7, §5.9, §6.1, §6.2, §6.3, §6.4, §6.5, §7.2, §7.3, §8; reason: replace optional frozen-rung routing with mandatory lossless plan/v2 demand consumption, conservative plan/v1 translation, harness-adapter-native implementation resolution, immutable evidence, and test-then-direct production activation |
| **Template version** | amendment-template/v2 |

## Operations

### op: supersede FR-6

Reason: a coarse optional rung cannot carry the merged plan/v2 execution-demand contract and permits downstream reinterpretation.

| ID | Requirement (EARS) | Rationale | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|
| FR-21 | When a ready plan/v2 task is considered for implementation, the executor shall recover the task's complete frozen assessment, evidence citations, execution profile, derivation-policy version, and planner-raise evidence from the projected carrier, compare them field-for-field with the frozen plan record, and pass them into the implementation attempt without reinterpretation. | planning owns task semantics; execution transports and consumes the frozen minimum rather than inferring complexity from charter prose | Must | critical | every supported tracker projection round-trips every demand field byte-identically; missing, changed, or native dispatch data produces zero task claims and names the boundary and field; a mechanical task follows the same path | Test | G1, J1, NR-1 |

### op: add FR

Reason: frozen plan/v1 remains executable only through a typed conservative translation that never invents provenance or rewrites the source artifact.

| ID | Requirement (EARS) | Rationale | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|
| FR-22 | When a ready frozen plan/v1 task is considered for implementation, the executor shall translate an explicit rung through the applicable versioned compatibility table or classify an absent rung as `legacy_unassessed` with frontier capability, maximum deliberation, extended context, and an assurance floor that existing risk signals may raise, while preserving the original plan bytes and a typed legacy origin. | merged v1 plans cannot be reassessed or rewritten, but execution must fail safe rather than default cheap | Must | high | explicit-rung corpus entries translate exactly; absent-rung entries receive the conservative profile; unknown rungs fail under UB-11; every source artifact remains byte-identical | Test + Inspection | G1, J1, NR-5 |

### op: add FR

Reason: native selection belongs to the selected harness adapter and must satisfy every independent semantic minimum at least cost.

| ID | Requirement (EARS) | Rationale | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|
| FR-23 | When the already-selected harness receives a plan/v2 or translated plan/v1 execution profile for the implementation role, that harness adapter shall select the least-cost qualified native profile satisfying every capability, deliberation, context, and assurance minimum under the effective closed, version-compatible routing policy; an exact result is preferred and a stronger result is permitted only as a recorded promotion naming every promoted component. | harness catalogs differ, while the frozen semantic floor and least-powerful-that-suffices policy must remain stable | Must | critical | exhaustive harness×derivable-profile implementation-role conformance proves every resolved cell selects the cheapest qualified dominating native profile; exact and promoted fixtures record the expected result; native identifiers appear only in adapter policy and attempt evidence | Test + Analysis | G1, G3, FR-5 |

### op: add FR

Reason: unsupported or invalid mappings must stop before side effects and may not silently switch harnesses.

| ID | Requirement (EARS) | Rationale | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|
| FR-24 | If the effective implementation-role routing policy is missing, malformed, unqualified, weakening, or has no native profile satisfying the requested execution profile, the executor shall emit a named unsupported or policy-invalid result and stop that task before claim or model invocation without trying another harness. | an underpowered launch or silent backend switch violates both the frozen plan and the operator's credential/egress decision | Must | critical | every negative policy and unsatisfied-profile fixture performs zero claims and zero model invocations, retains the selected harness, and names harness, role, policy version, reason, and unmet components | Test | G1, §3.3, NR-6 |

### op: add FR

Reason: routing and worker outcomes need one immutable evidence chain that distinguishes plan/v2 demand from translated plan/v1 origin.

| ID | Requirement (EARS) | Rationale | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|
| FR-25 | For every implementation routing attempt, the executor shall record exactly one plan/v2 frozen-demand or typed plan/v1 translation origin, the selected harness and implementation role, requested profile, effective policy version, and exactly one resolution terminal: an exact/promoted EVT-7 with native profile identity and exactly one connected EVT-10 verified, failed, or blocked worker outcome with available usage, or an unsupported/policy-invalid EVT-8 with no worker outcome; the bounded records shall be immutable and contain no prompt or code payload. | native choices need auditability without fabricating a worker outcome for a pre-claim failure or pretending legacy provenance | Must | high | every conformance and dogfood attempt joins exactly one origin to exactly one EVT-7 or EVT-8; every EVT-7 joins exactly one EVT-10 and every EVT-8 joins zero EVT-10; missing, duplicate, and cross-class terminals fail; retries append rather than overwrite; payload inspection is clean | Test + Analysis + Inspection | G2, J3, NFR-4 |

### op: add FR

Reason: the production fallback must overprovide within the selected harness without becoming an unauthorized bypass.

| ID | Requirement (EARS) | Rationale | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|
| FR-26 | Where the strongest-profile kill switch is engaged for implementation, the selected harness adapter shall prefer its strongest approved native implementation profile only when that profile satisfies every requested semantic minimum; otherwise FR-24 applies, with harness selection and frozen demand unchanged. | fallback may spend more but may never weaken or escape the task contract | Must | critical | per-harness drills prove the fallback dominates every resolved request, never switches harness, and produces an unsupported result with zero claim/invocation when even the strongest qualified profile is insufficient | Test + Demo | G1, §6.5, NR-6 |

### op: add FR

Reason: the operator chose full pre-release proof followed by one production cutover rather than shadow or canary routing.

| ID | Requirement (EARS) | Rationale | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|
| FR-27 | When the parent routing contract's complete pre-release gate is green, `/implement-now` shall activate the validated execution-demand resolver for every supported production implementation path in one switch with no shadow or canary cohort, and shall retire the hardcoded rung/model/effort path only after the direct-cutover and strongest-profile fallback rehearsals pass. | mixed routing semantics would make production outcomes incomparable and preserve the hardcoded pin indefinitely | Must | critical | one production configuration covers the complete supported harness/profile implementation domain; inspection finds no cohort logic or reachable hardcoded selection path after activation; cutover and fallback drills pass | Inspection + Demo | G1, §6.5 |

### op: add NR

Reason: the routing transition must preserve frozen legacy artifacts, upstream harness choice, and the executor's zero-work cost invariant.

| ID | Existing behavior protected | Why protected | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|
| NR-5 | Frozen plan/v1 artifacts remain byte-immutable and executable through only FR-22's versioned compatibility path. | merge freezes planning judgment; post-freeze reassessment would invent provenance | critical | the v1 corpus executes or fails with its typed compatibility result and remains byte-identical | Test + Inspection | FR-22, UB-11 |
| NR-6 | Harness selection remains the invoking configuration's explicit upstream decision; semantic routing never changes harnesses or scans installed CLIs to choose one. | credentials, availability, and egress policy are not model-capacity decisions | critical | exact, promoted, unsupported, and kill-switch fixtures all retain the same selected harness | Test + Inspection | FR-23, FR-24, FR-26 |
| NR-7 | An empty ready frontier performs no routing resolution, claim, or model invocation. | every executable task is model-backed, but zero work remains zero inference | high | empty-frontier fixtures emit zero EVT-7/EVT-8 records, zero claims, and zero model launches | Test | FR-12 |

### op: add UB

Reason: demand corruption, mapping failure, legacy incompatibility, and orphaned evidence require distinct fail-closed outcomes.

| ID | If… (trigger) | Then the system shall… | User sees | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|---|
| UB-9 | a plan/v2 projected task is missing any frozen demand field, differs from the frozen plan, contains an unknown semantic value, or contains native dispatch data | refuse the task before claim and identify the carrier, field, expected value or vocabulary, and remedy | task remains unclaimed with a plan-boundary integrity error | Must | critical | one fixture per missing/changed/unknown/native class performs zero claims and names every offending field; mixed task sets continue only independent valid tasks | Test | FR-21, NFR-7 |
| UB-10 | an effective native mapping or qualification is missing, malformed, incompatible, weakening, injected, or unable to satisfy the task profile | apply FR-24 and record EVT-8 before any claim or invocation | named policy-invalid or unsupported result with selected harness and unmet components | Must | critical | every trigger produces zero claims/invocations and no harness substitution; independent satisfiable tasks may continue | Test | FR-23, FR-24, NFR-8 |
| UB-11 | a frozen plan/v1 explicit rung is unknown to the applicable compatibility table or the table version is unavailable | stop the task before claim without rewriting or guessing, and name the plan version, original rung, compatibility version, and remedy | legacy compatibility error; plan unchanged | Must | high | unknown-rung and missing-version fixtures perform zero claims/invocations and zero source writes | Test | FR-22, NR-5 |
| UB-12 | a routing decision or worker outcome cannot connect to exactly one frozen/translated origin and attempt identity | treat the attempt as failed and non-evidentiary, preserve its artifacts, and surface an audit-integrity escalation | task is not counted succeeded; orphaned identity and missing edge named | Must | high | orphan, duplicate-terminal, and missing-origin fixtures never count toward M1/M4/M6 or release and never overwrite prior evidence | Test + Analysis | FR-25 |
| UB-13 | an effective implementation mapping or native-profile qualification is found defective after production activation | engage FR-26 for new work, stop new affected claims, append EVT-11 marking every affected in-flight or unaccepted attempt `rerun_required`, preserve unaffected work, and require replacement evidence on the exact affected task/proposal revision; for already accepted artifacts, preserve shipped state and emit an operator remediation brief without destructive rollback | exact policy/profile versions, attempts, tasks, proposals, revisions, and disposition are named; affected open work cannot be accepted until replacement evidence passes | Must | critical | a seeded defect marks every affected open attempt non-evidentiary, permits unaffected work to continue, reruns the exact affected revision successfully, and produces one complete operator brief for an accepted artifact without mutating it | Failure injection + Inspection | FR-25, FR-26, §6.5 |

### op: add ABUSE

Reason: native routing policy is a privileged execution-control surface that a contributor or worker could try to weaken or inject.

| ID | Actor and intent | Vector | Impact | Severity | Mitigated by | Verify |
|---|---|---|---|---|---|---|
| ABUSE-5 | repository contributor, compromised worker, or mistaken operator seeking cheaper or arbitrary execution | adds a weakening/unqualified mapping, unknown launch key, raw CLI argument, or self-declared native capability to routing policy | a worker launches below demand or with attacker-controlled egress/permissions | critical | FR-23 requires qualified dominance; FR-24/UB-10 fail before claim; NFR-7 closed parsing; operator-authorized policy and reviewed qualification evidence | contributor-only, worker-authored, weakening, unknown-key, argument-injection, and self-qualification fixtures all produce zero claims/invocations |

### op: add NFR

Reason: native policy must remain a deterministic, portable, closed data boundary rather than executable or vendor-shaped plan content.

| ID | Area | Requirement | Fit criterion | Verify |
|---|---|---|---|---|
| NFR-7 | Portability / integrity | The executor shall parse plan/v2 demand, plan/v1 compatibility, default mappings, and repository overrides as closed versioned schemas; frozen semantic demand shall contain no harness, vendor product, native model, native effort, provider-specific context, or raw launch-argument identifier, and native identifiers shall exist only in harness-adapter policy and attempt evidence. | every unknown key/enum, incompatible version, native-in-plan, and raw-argument fixture fails before claim; every harness adapter consumes the same semantic profile schema | Test + Inspection |
| NFR-8 | Determinism / cost | For identical frozen or translated demand, selected harness, implementation role, effective mapping policy, and qualification evidence, routing shall return one identical least-cost exact/promoted/unsupported result using zero model inference and no network call before the selected worker invocation. | repeated and reordered-input fixtures are byte-identical; exhaustive candidate-order tests always select the same cheapest qualified dominating result; dependency inspection records zero pre-invocation inference/network call | Test + Analysis + Inspection |

### op: supersede EVT-2

Reason: the old dispatch event records only backend+rung and cannot prove which semantic demand produced the native launch.

| ID | Event name | Fires when | Properties (typed) | Source | Feeds |
|---|---|---|---|---|---|
| EVT-7 | `routing_resolved` | an implementation route resolves before claim | `attempt_id:string`, `demand_id:string`, `task_id:string`, `harness:string`, `role:implementation`, `requested_profile:closed object`, `native_profile_id:string`, `resolution:enum{exact,promoted}`, `promoted_components:enum[]`, `policy_version:string` | selected harness adapter | M3, M4, M5, M6 |

### op: add EVT

Reason: unsupported, legacy, and policy-invalid paths need typed terminals before any worker side effect.

| ID | Event name | Fires when | Properties (typed) | Source | Feeds |
|---|---|---|---|---|---|
| EVT-8 | `routing_resolution_failed` | implementation routing stops before claim/invocation | `attempt_id:string`, `demand_id:string`, `task_id:string`, `harness:string`, `role:implementation`, `requested_profile:closed object`, `policy_version:string`, `unmet_components:enum[]`, `reason:enum{policy_invalid,unsupported,integrity_error,legacy_incompatible}` | executor + harness adapter | M4, M6 + health alert |
| EVT-9 | `legacy_execution_demand_translated` | a frozen plan/v1 task translates without rewrite | `demand_id:string`, `task_id:string`, `plan_version:string`, `rung_present:boolean`, `original_rung:string?`, `compatibility_version:string`, `translation:enum{explicit_rung,legacy_unassessed}`, `translated_profile:closed object`, `policy_version:string` | legacy translator | M4, M6 |
| EVT-11 | `implementation_routing_evidence_invalidated` | UB-13 identifies implementation evidence produced under a defective policy/profile | `policy_version:string`, `native_profile_id:string`, `attempt_ids:string[]`, `task_ids:string[]`, `proposal_refs:string[]`, `disposition:enum{rerun_required,operator_remediation}`, `evidence_ref:string` | executor + evidence owner | UB-13 audit + health alert |

### op: supersede EVT-5

Reason: a task outcome must connect to the exact resolved routing attempt rather than only the task identifier.

| ID | Event name | Fires when | Properties (typed) | Source | Feeds |
|---|---|---|---|---|---|
| EVT-10 | `routed_task_outcome` | a resolved implementation attempt reaches a terminal worker class | `attempt_id:string`, `demand_id:string`, `task_id:string`, `proposal_ref:string?`, `class:enum{succeeded,failed,blocked}`, `verification:enum{verified,failed,not_reached}`, `usage:closed object`, `turns:int`, `evidence_ref:string` | executor | M1, M2, M5, M6 |

### op: supersede DOD-1

Reason: functional completion must include mandatory demand transport, compatibility, native resolution, evidence, fallback, and cutover.

| ID | Done criterion | Source | Verify | Required evidence | Blocking? |
|---|---|---|---|---|---|
| DOD-10 | Every live Must functional requirement passes its fit criterion, including field-identical plan/v2 transport, no-rewrite plan/v1 translation, least-cost satisfying implementation resolution, immutable evidence, fallback, and direct activation. | FR-1..FR-5, FR-7..FR-10, FR-12..FR-18, FR-20..FR-27 | Test + Demo + Inspection + Analysis | per-requirement evidence index including exact, promoted, unsupported, legacy, and cutover cases | yes |

### op: supersede DOD-6

Reason: four-CLI coverage must prove the full derivable implementation-routing domain, not one unspecified native profile per CLI.

| ID | Done criterion | Source | Verify | Required evidence | Blocking? |
|---|---|---|---|---|---|
| DOD-11 | Every supported harness and derivable execution profile resolves through that harness's implementation-role adapter to its qualified least-cost satisfying native profile or the explicit unsupported result, with launch/failure capture. | FR-5, FR-23, FR-24, EVAL-2 | Test + Human judgment — @mdarabi; rubric: every required cell has current qualification evidence and its expected exact/promoted/unsupported outcome | complete implementation-role conformance matrix and operator sign-off | yes |

### op: supersede DOD-7

Reason: dogfood must falsify the hardcoded-pin failure by exercising meaningfully different resolved native profiles.

| ID | Done criterion | Source | Verify | Required evidence | Blocking? |
|---|---|---|---|---|---|
| DOD-12 | Dogfood: `/implement-now` implements one real pyxis feature end-to-end locally using at least three distinct resolved native implementation profiles that each connect to a succeeded EVT-10 with `verification=verified`; failed and blocked attempts remain in the evidence chain but do not count toward the three-profile threshold, and the operator only reviews/merges. | M1, M5, M6, FR-7, FR-25, EVAL-3 | Demo + Analysis | dogfood run, proposal links, joined routing/outcome report, and verified profile-diversity count | yes |

### op: supersede DOD-2

Reason: the original non-regression gate predates the routing transition and excludes its compatibility, harness-selection, and empty-frontier protections.

| ID | Done criterion | Source | Verify | Required evidence | Blocking? |
|---|---|---|---|---|---|
| DOD-15 | No live non-regression row is broken. | NR-1..NR-7 | Test + Inspection | one result per live NR row, including v1 byte identity, selected-harness retention, and empty-frontier zero work | yes |

### op: supersede DOD-3

Reason: the original failure gate excludes the new demand-integrity, mapping, legacy, evidence, and post-cutover defect paths.

| ID | Done criterion | Source | Verify | Required evidence | Blocking? |
|---|---|---|---|---|---|
| DOD-16 | Every live unwanted-behavior row is handled. | UB-1..UB-13 | Test + Failure injection + Analysis + Inspection | failure-injection matrix with one result per live UB row and zero omitted critical path | yes |

### op: supersede DOD-4

Reason: the original security proof excludes native-policy weakening, qualification forgery, and launch-argument injection.

| ID | Done criterion | Source | Verify | Required evidence | Blocking? |
|---|---|---|---|---|---|
| DOD-17 | The complete local-runtime and native-routing security posture is proven. | ABUSE-1..ABUSE-5 | Test + Inspection | abuse-drill report covering every actor/vector, including contributor-only and worker-authored mapping changes | yes |

### op: add DOD

Reason: direct production cutover and both legacy/fallback compatibility paths need blocking evidence separate from ordinary conformance.

| ID | Done criterion | Source | Verify | Required evidence | Blocking? |
|---|---|---|---|---|---|
| DOD-13 | The one-switch production activation and strongest-profile fallback pass rehearsal for every supported harness without a harness switch or semantic downgrade; an unsatisfiable strongest profile still fails before claim. | FR-24, FR-26, FR-27, NR-6, §6.5 | Test + Demo | direct-cutover configuration, fallback drill, and unreachable hardcoded-path inspection | yes |
| DOD-14 | The frozen plan/v1 corpus translates through the versioned compatibility table, absent-rung cases receive the conservative profile, unknown cases fail before claim, and all source bytes remain unchanged. | FR-22, NR-5, UB-11 | Test + Inspection | compatibility report and byte-identity manifest | yes |
| DOD-18 | A post-cutover qualification-defect drill stops affected claims, invalidates every affected in-flight or unaccepted implementation artifact, preserves unaffected work, reruns replacement evidence on the exact affected task/proposal revision, and produces an operator remediation brief for already accepted artifacts without destructive rollback. | UB-13, FR-25, FR-26, §6.5 | Failure injection + Inspection | defect-disposition drill record plus EVT-11 evidence, exact-revision replacement result, and accepted-artifact brief | yes |

### op: supersede EVAL-1

Reason: permitting bounded escalation for every nominal fixture allows an implementation worker with a zero-success rate to pass.

| ID | Behavior | Eval set / harness | Threshold | Negative examples |
|---|---|---|---|---|
| EVAL-3 | A worker turns a positive task charter into a genuinely-green proposal under the resolved native profile and reaches only the designated bounded terminal for an adversarial/failure case. | versioned positive and adversarial task corpus through the real runtime per harness; at least five decode-pinned positive runs per supported harness per release candidate | 100% of required positive runs reach green with launch matching EVT-7; 100% of designated adversarial cases reach their expected bounded terminal; zero hollow-as-succeeded or cross-task contamination; the report records required/executed counts and per-case outcomes | nominal task always escalates; hollow counted green; another task's diff; unbounded loop; launch differs from route; adversarial case falsely succeeds |

### op: add EVAL

Reason: deterministic routing needs exhaustive conformance over the supported implementation domain and negative mapping classes.

| ID | Behavior | Eval set / harness | Threshold | Negative examples |
|---|---|---|---|---|
| EVAL-2 | Each supported harness's implementation adapter maps every derivable execution profile to its qualified least-cost satisfying native profile or the explicit unsupported result, while preserving the selected harness and using no task-level `no_inference`, hardcoded global pin, or charter-derived complexity inference. | exhaustive harness×derivable-profile implementation-role corpus plus exact, promotion, override, strongest-fallback, legacy, and policy-invalid launch/failure capture | every required cell produces its declared result; every resolved cell launches the recorded native profile; zero omitted cells, unqualified mappings, semantic downgrades, harness switches, `no_inference` tasks, or reachable hardcoded pins | omitted harness/profile; cheaper qualified candidate skipped; underpowered mapping; missing effort; native ID in plan; bad override; unsupported demand; attempted harness fallback; absent-rung v1; unknown-rung v1; mechanical task bypassing inference |

### op: supersede §0 Read to rule — the human reviewer's section (~6 min)

Reason: the human routing section must expose the replacement of coarse rung mapping with mandatory semantic demand while remaining informative-only under AGENTS.md rule 9.

*The body is the contract; this section only routes attention — on any conflict the body governs; machines ignore it.*

**Judge these**

| # | The call | Could have gone | If you disagree |
|---|---|---|---|
| 1 | Opens PRs, **never merges** — landing is out of scope | Drive each task to *merged* end-to-end | The executor would hold merge authority, colliding with merge-lane DINV-5 and the vision's trust level 2 → §3.4 NG-1, FR-7 |
| 2 | **Full tracker-mediated** — projects via the planning stage and claims the same tagged items as the external fleet | A tracker-bypass local loop reading only the plan JSON | A second projection path and loss of idempotency/coexistence with the fleet → §4.1, FR-1, FR-2 |
| 3 | **Fresh non-interactive session per task**, no warm-session reuse | Reuse a warm session across tasks to amortize startup | Re-imports the same-session charter-bleed failure class → FR-5, UB-3 |
| 4 | Autonomous bypass-permission workers run on the **same hardware under ambient/local policy** | A sandbox / hardware-separated / policy-governed runtime | The security posture rests on worktree isolation + the never-merge boundary, not host isolation → §5.3, §3.4 NG-2/NG-3 |
| 5 | On unrecoverable task failure: **skip-and-continue + escalate** | Halt the whole run on first failure | Independent tasks continue and the failed work stays recoverable → FR-15, UB-1 |
| 6 | Build the **durable engine core** (worker + ports), local runtime first | A throwaway local script, remote rebuilt later | The remote/Docker fleet becomes another adapter set, not a rewrite → §4.1, §3.1 G3 |
| 7 | **Bounded-parallel** local dispatch by default | Sequential single-worker | Parallelism exercises claim/isolation seams; the cap remains policy → FR-11 |
| 8 | Every non-empty task consumes frozen semantic demand; the **already-selected harness adapter** chooses least-cost satisfying native model/effort and never another harness | Keep the hardcoded pin; let the executor infer from charter prose; put native settings in the plan | Richest-context judgment stays frozen and portable, while credentials/egress choice stays upstream → FR-21..FR-24, NR-6 |
| 9 | Frozen plan/v1 translates conservatively without rewrite; new plan/v2 demand is never inferred after freeze | Backfill old artifacts or default missing values cheaply | Merge-frozen judgment and audit provenance remain honest → FR-22, NR-5, UB-11 |
| 10 | Full pre-release proof, then **one direct production cutover** with strongest-profile fallback | Shadow or canary production routing | The operator's rollout ruling is explicit; fail-closed resolution and rehearsed fallback bound it → FR-26, FR-27, §6.5 |

**Sanity-check the proof** — model routing is not a scalar complexity guess. The planner freezes six assessment axes and four independent semantic minima. `implement-now` proves field-identical transport, a complete harness×derivable-profile implementation-role matrix, at least three distinct native profiles in dogfood, no-rewrite v1 translation, and exact/promoted/unsupported evidence. The routing resolver itself spends zero inference (NFR-8).

**The author's open questions** — whether Claude Code's headless mode fully covers an autonomous multi-tool run remains open (§7.2 Q-1). Harness *selection* mechanics remain design-owned (§7.2 Q-4); demand resolution never answers that question or scans for another harness. The other open rows are implementation mechanics behind settled requirements.

**Skip the rest** — structure/ID/fold correctness → docgate; substance and coherence → `/prd-review`; implementation mechanisms → the superseding technical design; review model/process routing → the impl-review sibling amendment; merge safety → merge-lane; plan judgment and projection → plan-new.

### op: supersede §1 Summary

Reason: the executor summary must state the new frozen-demand, adapter-resolution, compatibility, and direct-cutover boundaries.

- **Problem** — Once a plan is frozen and projected, pyxis can execute it locally, but the implementation route is still coupled to an optional coarse rung and hardcoded/implicit native settings; plan/v2's richer demand can be lost or reinterpreted before launch, while legacy plans have no explicit fail-safe translation contract.
- **For whom** — The operator with a pyxis-installed repo who has just planned in-session (attended), and argus driving the same core unattended.
- **Why now** — Plan-new now freezes complete semantic demand on every plan/v2 task, and the parent routing contract requires `/implement-now` to consume it through harness-specific native mappings before production activation.
- **Direction** — Preserve the existing single-task worker and upstream harness choice. Before claim, recover plan/v2 demand field-identically or translate plan/v1 conservatively; resolve the implementation profile through the selected harness adapter to the least-cost qualified satisfying native profile; fail unsupported/invalid cases without claim or invocation; then run the unchanged claim → workspace → agent → genuinely-green proposal path and retain a connected evidence chain.
- **Appetite** — The native local execution core plus the complete implementation-routing seam and dogfood proof; excludes harness selection policy, review-process routing, the remote runtime, hardware separation, umbrella policies, and landing.
- **Success** — `/implement-now` implements a real feature with every plan demand preserved, every routing cell conformance-proven, at least three native profiles each reaching a succeeded verified outcome, and every other resolved attempt retained as a genuinely-green or bounded-failure outcome.
- **Guardrail** — It never merges, mutates the parent checkout, double-claims, downgrades semantic demand, switches harnesses, rewrites plan/v1, or launches under invalid/unsupported routing.
- **Top risks** — stale/unqualified native mappings; tracker transport dropping demand; legacy translation becoming mutable inference; direct activation exposing a mapping defect; the existing charter-bleed, unbounded-loop, and false-green failures.

### op: supersede §3.2 Goals → signals → metrics

Reason: success now includes demand fidelity, native-profile diversity, and complete routing-to-outcome evidence.

| Metric | Goal | Signal | Baseline | Target (at checkpoint) | Feeds from |
|---|---|---|---|---|---|
| M1 (primary) | G1 | a real pyxis feature's tasks reach green PRs via `/implement-now` | 0 features under demand-aware routing | 1 dogfood feature fully implemented locally, operator only reviewing/merging | EVT-10, EVT-6 |
| M2 | G2 | tasks counted done that are genuinely green | n/a | 100% of succeeded tasks carry a green proposal; zero hollow-as-succeeded | EVT-4, EVT-10, EVT-6 |
| M3 | G3 | worker core and runtime adapters retain portability | one coarse rung mapper | the worker passes unchanged behind fake/local dispatch; every supported harness passes implementation-role routing conformance | EVT-7, EVT-8, EVT-6 |
| M4 | G1 | requested semantic profiles whose plan/v2 fields are identical to frozen demand or whose plan/v1 origin is a typed FR-22 translation, followed by a satisfying or explicit unsupported result | optional rung can be absent and demand can be lost | 100% exact origin/transport/resolution; zero inferred, downgraded, native-in-plan, or cross-harness substitutions | EVT-7, EVT-8, EVT-9 |
| M5 | G1 | distinct native implementation profiles exercised by dogfood attempts that succeed with `verification=verified` | hardcoded/implicit route can converge on one pin | at least 3 before production activation; failed/blocked attempts never count | EVT-7, EVT-10 |
| M6 | G2 | routing attempts with exactly one origin, one resolution terminal, and—when resolved—one worker outcome | no complete decision chain | 100% in conformance and dogfood before activation; remains 100% over production attempts after activation; zero orphan/duplicate/cross-class terminals | EVT-7..EVT-10 |

### op: supersede §3.3 Guardrails and decision rule

Reason: under-routing, harness switching, legacy mutation, and evidence loss must block release alongside the executor's original safety boundaries.

| Guardrail metric | Threshold | Action if breached |
|---|---|---|
| Parent-checkout mutation | any mutation of the parent clone / engine checkout outside a worktree | fail-closed; halt the offending worker and alarm |
| Double-claim with the external fleet | any task worked concurrently by both a local worker and a fleet node | fail-closed; block release until the claim mechanism is fixed |
| Self-merge | any merge performed by the executor | critical defect; executor holds no merge credential |
| Idle cost | any routing resolution or model spend on an empty ready frontier | investigate the frontier gate and block release on a model launch |
| Semantic downgrade or harness switch | any native launch fails to satisfy one requested component, uses an unqualified mapping, or changes the selected harness | stop affected claims; block/rollback immediately; repair policy and rerun affected attempts |
| Frozen-plan mutation or post-freeze inference | any plan/v1 rewrite or plan/v2 assessment/profile reinterpretation | invalidate affected evidence; require no-rewrite translation or field-identical transport before retry |
| Routing evidence break | any attempt lacks exactly one origin/terminal or a resolved attempt lacks exactly one outcome | do not count it succeeded or toward release; preserve artifacts and repair the evidence chain |

**Ship rule (pre-registered):** M1–M6 meet their pre-release targets, every live blocking DoD row (DOD-5, DOD-8..DOD-18) is green, and every guardrail stays clean through complete pre-release proof → activate every supported production implementation path in one cutover; post-launch M4–M6 and every guardrail remain continuously evaluated; any breach → no activation or immediate strongest-profile fallback and affected-attempt invalidation.

### op: supersede §4.1 Approach and alternatives considered

Reason: the chosen shape now includes the semantic-demand-to-native adapter seam while preserving the original worker architecture.

The chosen shape remains a **durable execution core run in its simplest runtime**, extended by a deterministic routing boundary before claim. A self-contained single-task worker consumes one task charter plus its frozen plan/v2 demand—or a typed no-rewrite plan/v1 translation—and repo access. The invoking configuration has already selected one of the four harnesses. That harness's adapter resolves the implementation-role semantic minimum through closed qualified policy to the least-cost satisfying native profile, records exact/promotion/unsupported evidence, and only a resolved route may proceed to claim → isolated worktree → fresh agent → genuinely-green proposal → release. The Dispatcher remains separate and bounded-parallel; native resolution does not choose a harness, change the Worker, or create another projection path.

| Option | Sketch | Why not chosen |
|---|---|---|
| A (chosen) | frozen/translated demand → selected harness adapter → qualified native profile → durable worker | preserves plan meaning and harness ownership while reusing the worker core |
| B | keep one hardcoded native model/effort per harness or globally | collapses independent task needs, strands planner judgment, and cannot satisfy least-powerful-that-suffices |
| C | let runtime infer complexity from charter/repo at claim time | re-judges with less context after freeze and produces unauditable drift |
| D | put native model/effort or harness choice in the plan | couples frozen artifacts to vendor catalogs and credential/egress policy |
| E | switch to another harness when selected harness is unsupported | violates the operator's upstream harness, credential, and egress decision |
| F | a throwaway local script or tracker-bypass loop | remote runtime becomes a rewrite or a second projection path |
| Do nothing | — | hardcoded/implicit routing remains and the parent routing contract cannot activate |

### op: supersede §4.2 Scope perimeter

Reason: implementation routing and legacy compatibility enter scope, while harness choice and review orchestration remain explicitly downstream siblings.

- **In (plan of record)** — the single-task worker; four-CLI backend-agnostic runtime with fresh sessions; claim + worktree isolation; bounded-parallel frontier dispatch; drive-to-green; plan/v2 demand integrity and lossless consumption; no-rewrite plan/v1 translation; selected-harness implementation-role native mappings and qualification; least-cost satisfying resolution, visible promotion, unsupported refusal before claim; immutable routing/outcome evidence; strongest-profile fallback; test-then-direct production activation.
- **Future (believed next, not committed)** — the remote/Docker dispatch adapter; hardware isolation; umbrella policy; auto-landing (WR-1, WR-2).
- **Out** — merging; hardware separation; fleet umbrella policy; choosing or switching harnesses; planner assessment/derivation; proposal-risk classification; review-role resolution, pass count, blindness, or aggregation; adaptive routing or cross-harness optimization.

### op: supersede §4.3 Experience: flows and states

Reason: the attended/unattended flow must expose pre-claim demand validation, native resolution, and unsupported failure.

**Flow F1 — an in-session run (serves J1, J3)**

| Step | User does | System responds | States covered | Design |
|---|---|---|---|---|
| 1 | invokes `/implement-now` on a frozen, projected plan with an explicitly selected/ambient harness | reports the ready frontier; an empty frontier exits with zero translation, routing, claim, or model work | ideal · empty · error | superseding tech design |
| 2 | watches (attended) or supplies the journal (argus) | for each ready task, validates field-identical plan/v2 demand or emits typed plan/v1 translation, resolves through that harness's implementation mapping, and refuses invalid/unsupported tasks before claim | ideal · unsupported · integrity error · legacy error | superseding tech design |
| 3 | — | claims only resolved tasks, then shows claimed → implementing → proposal → green while independent valid tasks continue | loading · partial · error | superseding tech design |
| 4 | reviews the final report | receives succeeded/failed/blocked/skipped plus exact/promoted/unsupported routing evidence and proposal links | ideal · partial · error | superseding tech design |

- **Empty** — no ready tasks: report frontier state and exit with zero EVT-7/EVT-8, claims, or model calls.
- **Loading** — a resolved worker is implementing or awaiting CI; live status names selected harness and native profile identity but no prompt/code payload.
- **Partial** — green, blocked, claimed-elsewhere, failed, integrity-invalid, legacy-incompatible, and unsupported tasks are classed separately; independent valid work continues.
- **Error** — policy-invalid/unsupported stops before claim; execution failure after claim follows the existing bounded escalation path.
- **Accessibility / localization** — unchanged terminal/agent-driven, English-only surface.

### op: supersede §4.4 Scenarios

Reason: common and failure scenarios must demonstrate multiple semantic profiles, promotion, unsupported behavior, legacy translation, and direct-cutover fallback.

**Common case.** A plan/v2 foundation task and two leaves carry different semantic profiles. Codex is already selected by ambient configuration. Its adapter resolves the foundation to a stronger native model with deep effort, a local leaf to a cheaper qualified model with standard effort, and promotes the third because no exact native context profile exists. Each EVT-7 names the same selected harness, requested profile, native profile, and promotion; only then are tasks claimed and driven to genuinely-green proposals. The run's three EVT-10 outcomes join to their origins.

**Unsupported task.** Cursor is selected for a task whose semantic minimum has no qualified satisfying native profile. The executor emits EVT-8 with unmet components, performs zero claim/invocation, never tries Codex or another installed CLI, and continues an independent satisfiable task.

**Legacy task.** A frozen plan/v1 task with no rung emits EVT-9 as `legacy_unassessed` with frontier/maximum/extended conservative demand; the source bytes remain unchanged. An unknown explicit rung instead fails before claim under UB-11.

**Cutover and fallback.** After the complete conformance matrix, three-profile dogfood, evidence audit, and fallback drill are green, one switch activates demand-aware implementation routing for every supported production path. A seeded mapping defect engages the strongest-profile fallback within the selected harness; if that profile cannot satisfy demand, the task remains unclaimed.

**Decision:** Solution gate — zero TBDs may pass.

| Reviewer | Role | Date | Verdict |
|---|---|---|---|
| @mdarabi | operator (via the introducing PR's review) | — | — |

### op: supersede §5.1 Functional requirements

Reason: the live requirement table and priority bar must replace optional rung routing with mandatory demand transport, compatibility, native resolution, evidence, fallback, and cutover.

| ID | Requirement (EARS) | Rationale | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|
| FR-1 | When invoked on a frozen plan, the executor shall build the ready frontier of tasks that are open, dependency-unblocked, and unclaimed. | the DAG defines what may run | Must | med | given a fixture plan+tracker state, the computed frontier equals the open∧unblocked∧unclaimed set | Test | G1, J1, F1 |
| FR-2 | Before working a task, the executor shall acquire an at-most-one-worker claim through the same automated-implementation claim mechanism the external fleet uses. | one worker per task, and coexistence with the fleet | Must | high | in a concurrent local+fleet race fixture, exactly one worker ever holds a task; the loser skips | Test | G1, NR-2 |
| FR-3 | The executor shall run each task as a self-contained worker that, given one task charter and repo access, completes the task with no dependence on the invoking session's memory or on any sibling task. | the worker is the portable, remote-reusable unit | Must | high | the worker runs to a green proposal in a harness supplying only a task charter + repo; no sibling/session state is read | Test | G3, J1 |
| FR-4 | The executor shall run each worker in an isolated workspace branched from the freshly-fetched mainline, and shall not mutate the parent checkout. | isolation + correct base | Must | high | a worker's writes appear only in its workspace; the base is the fetched mainline tip; a parent-checkout write is blocked | Test | G1, §3.3 |
| FR-5 | The executor shall launch the implementing agent through one backend-agnostic runtime contract supporting Claude Code, Codex, Cursor, and OpenCode, each as a fresh non-interactive session per task. | one contract, four backends; fresh sessions avoid charter bleed | Must | high | each backend drives a task to a green proposal through the identical runtime contract; no session is reused across tasks | Test | G1, DOD-11, UB-3 |
| FR-21 *(001 ← FR-6)* | When a ready plan/v2 task is considered for implementation, the executor shall recover the task's complete frozen assessment, evidence citations, execution profile, derivation-policy version, and planner-raise evidence from the projected carrier, compare them field-for-field with the frozen plan record, and pass them into the implementation attempt without reinterpretation. | planning owns task semantics; execution transports and consumes the frozen minimum rather than inferring complexity from charter prose | Must | critical | every supported tracker projection round-trips every demand field byte-identically; missing, changed, or native dispatch data produces zero task claims and names the boundary and field; a mechanical task follows the same path | Test | G1, J1, NR-1 |
| FR-22 *(001)* | When a ready frozen plan/v1 task is considered for implementation, the executor shall translate an explicit rung through the applicable versioned compatibility table or classify an absent rung as `legacy_unassessed` with frontier capability, maximum deliberation, extended context, and an assurance floor that existing risk signals may raise, while preserving the original plan bytes and a typed legacy origin. | merged v1 plans cannot be reassessed or rewritten, but execution must fail safe rather than default cheap | Must | high | explicit-rung corpus entries translate exactly; absent-rung entries receive the conservative profile; unknown rungs fail under UB-11; every source artifact remains byte-identical | Test + Inspection | G1, J1, NR-5 |
| FR-23 *(001)* | When the already-selected harness receives a plan/v2 or translated plan/v1 execution profile for the implementation role, that harness adapter shall select the least-cost qualified native profile satisfying every capability, deliberation, context, and assurance minimum under the effective closed, version-compatible routing policy; an exact result is preferred and a stronger result is permitted only as a recorded promotion naming every promoted component. | harness catalogs differ, while the frozen semantic floor and least-powerful-that-suffices policy must remain stable | Must | critical | exhaustive harness×derivable-profile implementation-role conformance proves every resolved cell selects the cheapest qualified dominating native profile; exact and promoted fixtures record the expected result; native identifiers appear only in adapter policy and attempt evidence | Test + Analysis | G1, G3, FR-5 |
| FR-24 *(001)* | If the effective implementation-role routing policy is missing, malformed, unqualified, weakening, or has no native profile satisfying the requested execution profile, the executor shall emit a named unsupported or policy-invalid result and stop that task before claim or model invocation without trying another harness. | an underpowered launch or silent backend switch violates both the frozen plan and the operator's credential/egress decision | Must | critical | every negative policy and unsatisfied-profile fixture performs zero claims and zero model invocations, retains the selected harness, and names harness, role, policy version, reason, and unmet components | Test | G1, §3.3, NR-6 |
| FR-25 *(001)* | For every implementation routing attempt, the executor shall record exactly one plan/v2 frozen-demand or typed plan/v1 translation origin, the selected harness and implementation role, requested profile, effective policy version, and exactly one resolution terminal: an exact/promoted EVT-7 with native profile identity and exactly one connected EVT-10 verified, failed, or blocked worker outcome with available usage, or an unsupported/policy-invalid EVT-8 with no worker outcome; the bounded records shall be immutable and contain no prompt or code payload. | native choices need auditability without fabricating a worker outcome for a pre-claim failure or pretending legacy provenance | Must | high | every conformance and dogfood attempt joins exactly one origin to exactly one EVT-7 or EVT-8; every EVT-7 joins exactly one EVT-10 and every EVT-8 joins zero EVT-10; missing, duplicate, and cross-class terminals fail; retries append rather than overwrite; payload inspection is clean | Test + Analysis + Inspection | G2, J3, NFR-4 |
| FR-26 *(001)* | Where the strongest-profile kill switch is engaged for implementation, the selected harness adapter shall prefer its strongest approved native implementation profile only when that profile satisfies every requested semantic minimum; otherwise FR-24 applies, with harness selection and frozen demand unchanged. | fallback may spend more but may never weaken or escape the task contract | Must | critical | per-harness drills prove the fallback dominates every resolved request, never switches harness, and produces an unsupported result with zero claim/invocation when even the strongest qualified profile is insufficient | Test + Demo | G1, §6.5, NR-6 |
| FR-27 *(001)* | When the parent routing contract's complete pre-release gate is green, `/implement-now` shall activate the validated execution-demand resolver for every supported production implementation path in one switch with no shadow or canary cohort, and shall retire the hardcoded rung/model/effort path only after the direct-cutover and strongest-profile fallback rehearsals pass. | mixed routing semantics would make production outcomes incomparable and preserve the hardcoded pin indefinitely | Must | critical | one production configuration covers the complete supported harness/profile implementation domain; inspection finds no cohort logic or reachable hardcoded selection path after activation; cutover and fallback drills pass | Inspection + Demo | G1, §6.5 |
| FR-7 | The executor shall drive each task to a proposal whose local gate and required remote CI are green before counting the task done, and shall not merge any proposal. | "done" = mergeable; landing stays downstream | Must | high | a task counts done only when its proposal exists and all required checks are green; the executor performs zero merges | Test | G2, NG-1, J3 |
| FR-8 | The executor's local gate shall run the same deterministic checks required by CI. | prevent "passed local, failed CI" | Must | high | a change failing a required CI check also fails the local gate on the same input | Test | G2, UB-5 |
| FR-9 | When a proposal's checks are red, the executor shall attempt recovery through a bounded self-heal loop and, on exhausting the bound, escalate. | recover routine CI failures without unbounded looping | Must | high | red→fix→re-check converges or, at the bound, stops and escalates; the attempt count is bounded and observable | Test | G2, UB-1 |
| FR-10 | The executor shall count a task succeeded only on a verified proposal artifact, and shall report a worker that returned without one as hollow. | success is an artifact, never self-report | Must | high | a worker that exits without a proposal is reported hollow, never succeeded | Test | G2, UB-4 |
| FR-11 | The executor shall dispatch tasks through a dispatch layer separate from the worker; the local runtime shall run a bounded, configurable number of workers concurrently without changing the worker. | concurrency is a dispatch concern, not a worker property | Should | med | changing the local concurrency bound changes only dispatch config; the worker code is unchanged | Inspection | G3 |
| FR-12 | While the ready frontier is empty, the executor shall consume no model tokens. | idle is free | Must | med | a run over an empty frontier emits no routing resolution or model call | Test | §3.3, NR-7 |
| FR-13 | The executor shall bound each worker's turns mechanically; a non-converging worker shall checkpoint, exit resumably, and be respawned up to a cap, then escalated. | bound per-task spend; a model cannot count its own turns | Must | high | a worker exceeding the turn bound checkpoints and exits; respawns are capped; at the cap it escalates | Test | UB-6 |
| FR-14 | When re-invoked or resumed after interruption, the executor shall re-probe the tracker and continue from pushed work, producing no duplicate proposal for a task. | crash/rerun resumability without duplication | Must | high | after a mid-run kill, a re-invocation resumes in-flight tasks from their pushed branches; no task yields two proposals | Test | G1, UB-2 |
| FR-15 | If a task cannot reach green within the self-heal bound, the executor shall mark it failed, escalate it, preserve its work, continue the other independent ready tasks, and end with a succeeded/failed/blocked report. | one failure never stalls the whole run | Must | med | in a fixture where one task fails, independent tasks still complete and the report classes every task | Test | J1, UB-1 |
| FR-16 | Before consuming an attempt, the executor shall preflight each backend's auth and inject the backend's credential into the worker session; on a missing credential it shall degrade with a stated, actionable reason and not count a success. | an unauthenticated child looks like a fast success | Must | high | with a backend unauthenticated, the run reports the missing-credential reason and spawns no counted worker | Test | UB-7 |
| FR-17 | The executor shall be drivable by the operator or by argus over the same worker core. | both consumers, one core | Must | med | the same run completes under an attended invocation and under an unattended journal-only invocation | Demo | J1, J2 |
| FR-18 | The executor shall evaluate each run-control predicate through a single implementation. | drifted duplicate predicates cause phantom loops | Must | high | each predicate has exactly one implementation | Inspection | §3.3, UB-1 |
| FR-19 | The executor shall provide each worker its full task charter and the relevant context inline at spawn. | a context-starved worker re-discovers at the most expensive rate | Should | med | the spawn input carries the charter and cited context; the worker performs no avoidable context re-discovery | Inspection | NFR-10 |
| FR-20 | The executor's stage-8 binding shall be introduced by moving the pipeline map's stage-8 row and re-pinning the binding through the sanctioned hook, not by a second pipeline. | the operational binding lives on the map | Must | med | the pipeline map's stage-8 row states the native-local binding; the change rides this feature's implementation | Inspection | NR-1 |

*Priority bar: Must = the executor does not ship without it. FR-11 and FR-19 are the only Should rows. Demand transport, compatibility, native resolution, evidence, fallback, and cutover (FR-21..FR-27) are Must because any silent absence or downgrade changes autonomous execution.*

**Non-regression (NR-n)**

| ID | Existing behavior protected | Why protected | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|
| NR-1 | The plan→tracker projection contract remains the single projection path. | no second projection path | high | every executor read uses the existing projection and FR-21 proves demand round-trip | Inspection + Test | FR-1, FR-20, FR-21 |
| NR-2 | The external fleet keeps working; local and fleet share one claim mechanism. | coexistence without double-grab | high | shared claim fixture shows no task worked by both | Test | FR-2 |
| NR-3 | The merge-lane landing path is unchanged; the executor holds no merge credential. | landing is a separate authority | critical | zero approving/merging credential; landing routes as before | Inspection | FR-7, NG-1 |
| NR-4 | The argus principal/hands split remains: executor implements, argus never self-implements. | preserve agency boundary | high | argus-driven fixture shows all task work in executor attempts | Inspection | FR-17, J2 |
| NR-5 *(001)* | Frozen plan/v1 artifacts remain byte-immutable and executable through only FR-22's versioned compatibility path. | merge freezes planning judgment | critical | v1 corpus executes or fails with typed compatibility result and remains byte-identical | Test + Inspection | FR-22, UB-11 |
| NR-6 *(001)* | Harness selection remains the invoking configuration's explicit upstream decision; semantic routing never changes harnesses or scans installed CLIs to choose one. | credentials, availability, and egress policy are not model-capacity decisions | critical | exact, promoted, unsupported, and kill-switch fixtures retain the selected harness | Test + Inspection | FR-23, FR-24, FR-26 |
| NR-7 *(001)* | An empty ready frontier performs no routing resolution, claim, or model invocation. | zero work remains zero inference | high | empty fixtures emit zero routing records, claims, and launches | Test | FR-12 |

### op: supersede §5.4 Data

Reason: the executor now retains demand origins, policies, routing terminals, and connected outcomes while preserving no-rewrite migration.

| Entity / field | Meaning | Source of truth | Retention | Integrity rules | Privacy class |
|---|---|---|---|---|---|
| plan/v2 execution demand | frozen assessment, citations, semantic profile, derivation version, raise evidence | frozen plan; projected copy is transport | with plan | field-identical through executor boundary; immutable; no native identifiers | internal |
| plan/v1 legacy origin | original rung presence/value plus compatibility version and translated profile | immutable source plan + typed EVT-9 | forever with routing evidence | source never rewritten; explicit or `legacy_unassessed`; unknown fails | internal |
| native routing policy | qualified harness×implementation-role mappings plus authorized repo overrides | released defaults + validated repo policy | current + repository history | closed/version-compatible; qualification evidence; no weakening/raw arguments | internal |
| routing attempt | selected harness/role, requested profile, policy version, exact/promoted/unsupported result, native identity if resolved | immutable EVT-7 or EVT-8 | forever in git evidence | exactly one terminal per attempt; connected to one origin | internal |
| worker outcome | verified/failed/blocked terminal and usage for a resolved attempt | immutable EVT-10 + proposal evidence | forever in git evidence | exactly one per resolved attempt; absent for unsupported pre-claim attempts | internal |
| evidence invalidation | affected implementation attempts/artifacts and `rerun_required` or `operator_remediation` disposition | immutable EVT-11 + parent routing policy defect disposition | forever in git evidence | append-only; exact policy/profile and artifact identities; never rewrites the invalid evidence or accepted artifact | internal |
| task claim | at-most-one-worker lease | shared claim mechanism | until release/reap | acquired only after routing resolves; paired release | internal |
| worker workspace | isolated worktree + branch | local git | until reaped | fetched-mainline base; parent never mutated | internal |
| proposal | task change + verification evidence | code host | per host | one per task; green before succeeded | internal |

- **Migration** — no artifact is rewritten. New plan/v2 demand is consumed field-identically; frozen plan/v1 reads through FR-22 and emits EVT-9.
- **Parallel-run / cutover** — no production shadow/canary. Pre-release conformance and dogfood exercise the full implementation domain before one direct cutover.
- **Rollback** — FR-26 changes native resolution within the selected harness only. Invalid/unsupported demand remains unclaimed; frozen inputs and prior evidence remain readable.
- **Decommission** — remove the hardcoded/implicit rung/model/effort path only after the direct cutover and fallback drill pass.
- **Privacy check** — no new personal/sensitive data; no DPIA trigger.
- **Egress** — the already-selected harness remains the only model boundary. Routing adds metadata, no new provider or payload egress; prompts/code are excluded from routing evidence.

### op: supersede §5.5 Interfaces and integrations

Reason: the interfaces must distinguish frozen-demand transport, compatibility translation, selected-harness resolution, and evidence emission.

| Interface | Direction | Purpose | Contract | Failure modes | Owner |
|---|---|---|---|---|---|
| frozen plan/v2 + projected task | in | supply charter and complete semantic demand | versioned closed schema; projected fields identical to frozen record | missing/changed/native field → UB-9 before claim | plan-new + tracker adapters |
| frozen plan/v1 | in | legacy execution without rewrite | explicit rung through versioned table or absent-rung `legacy_unassessed` | unknown rung/version → UB-11 | compatibility owner + executor |
| harness selection | in | choose the one credential/egress/runtime boundary | explicit config or ambient harness; no installed-CLI inference | absent/unauthenticated → FR-16; unsupported remains same harness | operator/invoker |
| executor → selected harness adapter | out | resolve implementation semantic minimum | qualified implementation-role mapping; exact/promoted/unsupported | invalid mapping or no satisfying profile → FR-24/UB-10 | runtime adapters |
| selected harness adapter → AgentRuntime launch | out | invoke the resolved native profile | native model/effort/context arguments derived only from adapter policy | argument injection, launch mismatch → UB-10/ABUSE-5 | runtime adapter |
| runtime → evidence stream | out | preserve origin, route, and worker outcome | EVT-7..EVT-10, immutable and payload-free | orphan/duplicate/missing edge → UB-12 | executor + evidence owner |
| code host | both | proposals + CI | existing proposal/check surface | lag bounded; malformed is not green | this feature |
| merge-lane | out | landing downstream | opens proposals; never merges | no landing behavior owned here | merge-lane |

### op: supersede §5.7 Instrumentation — the tracking plan

Reason: the normalized executor stream must carry routing origins, decisions, failures, and connected outcomes rather than coarse rung metadata.

| ID | Event name | Fires when | Properties (typed) | Source | Feeds |
|---|---|---|---|---|---|
| EVT-1 | `run_started` | a run begins | `plan_ref:string`, `frontier_size:int`, `driver:enum{operator,argus}` | executor | — |
| EVT-7 *(001 ← EVT-2)* | `routing_resolved` | an implementation route resolves before claim | `attempt_id:string`, `demand_id:string`, `task_id:string`, `harness:string`, `role:implementation`, `requested_profile:closed object`, `native_profile_id:string`, `resolution:enum{exact,promoted}`, `promoted_components:enum[]`, `policy_version:string` | selected harness adapter | M3, M4, M5, M6 |
| EVT-8 *(001)* | `routing_resolution_failed` | routing stops before claim/invocation | `attempt_id:string`, `demand_id:string`, `task_id:string`, `harness:string`, `role:implementation`, `requested_profile:closed object`, `policy_version:string`, `unmet_components:enum[]`, `reason:enum{policy_invalid,unsupported,integrity_error,legacy_incompatible}` | executor + harness adapter | M4, M6 + health alert |
| EVT-9 *(001)* | `legacy_execution_demand_translated` | a frozen plan/v1 task translates without rewrite | `demand_id:string`, `task_id:string`, `plan_version:string`, `rung_present:boolean`, `original_rung:string?`, `compatibility_version:string`, `translation:enum{explicit_rung,legacy_unassessed}`, `translated_profile:closed object`, `policy_version:string` | legacy translator | M4, M6 |
| EVT-11 *(001)* | `implementation_routing_evidence_invalidated` | UB-13 identifies implementation evidence produced under a defective policy/profile | `policy_version:string`, `native_profile_id:string`, `attempt_ids:string[]`, `task_ids:string[]`, `proposal_refs:string[]`, `disposition:enum{rerun_required,operator_remediation}`, `evidence_ref:string` | executor + evidence owner | UB-13 audit + health alert |
| EVT-3 | `ci_fix_attempt` | a self-heal iteration runs | `task_id:string`, `attempt:int`, `bound:int` | executor | — |
| EVT-4 | `task_verified` | a proposal is verified green or hollow | `task_id:string`, `proposal_ref:string`, `green:boolean`, `hollow:boolean` | executor | M2 |
| EVT-10 *(001 ← EVT-5)* | `routed_task_outcome` | a resolved attempt reaches a terminal worker class | `attempt_id:string`, `demand_id:string`, `task_id:string`, `proposal_ref:string?`, `class:enum{succeeded,failed,blocked}`, `verification:enum{verified,failed,not_reached}`, `usage:closed object`, `turns:int`, `evidence_ref:string` | executor | M1, M2, M5, M6 |
| EVT-6 | `run_finished` | a run ends | `succeeded:int`, `failed:int`, `blocked:int`, `tokens:int`, `routing_exact:int`, `routing_promoted:int`, `routing_unsupported:int`, `routing_invalid:int` | executor | M1, M2, M4, M6 |

### op: supersede §5.9 AI behavior and evals

Reason: worker behavior remains model-driven, while deterministic native routing needs a separate exhaustive conformance eval.

*The worker's implementation remains model-driven and is accepted behaviorally. Demand validation, compatibility translation, and native resolution are deterministic and exhaustively tested; they never use an inference call to decide routing.*

| ID | Behavior | Eval set / harness | Threshold | Negative examples |
|---|---|---|---|---|
| EVAL-3 *(001 ← EVAL-1)* | a worker turns a positive task charter into a genuinely-green proposal under the resolved native profile and reaches only the designated bounded terminal for an adversarial/failure case | versioned positive and adversarial task corpus through the real runtime per harness; at least five decode-pinned positive runs per supported harness per release candidate | 100% of required positive runs reach green with launch matching EVT-7; 100% of designated adversarial cases reach their expected bounded terminal; zero hollow-as-succeeded or cross-task contamination; the report records required/executed counts and per-case outcomes | nominal task always escalates; hollow counted green; another task's diff; unbounded loop; launch differs from route; adversarial case falsely succeeds |
| EVAL-2 *(001)* | each supported harness's implementation adapter maps every derivable execution profile to its qualified least-cost satisfying native profile or explicit unsupported result, preserving harness and avoiding `no_inference`, hardcoded global pins, or charter-derived inference | exhaustive harness×profile implementation corpus plus exact, promotion, override, fallback, legacy, and invalid-policy capture | every required cell yields its declared result; every resolved launch matches; zero omitted/unqualified/downgraded/switched/no-inference/hardcoded cases | omitted cell; cost inversion; underpowered mapping; missing effort; native plan ID; bad override; harness fallback; invalid/absent legacy rung; mechanical inference bypass |

### op: supersede §6.1 Phasing and milestones

Reason: implementation routing needs contract, resolution, and pre-release proof before the original worker dogfood becomes production evidence.

| Phase | Purpose | Scope | Exit criterion |
|---|---|---|---|
| Demand boundary | consume without re-judging | plan/v2 integrity/transport, plan/v1 translation, origin identities, negative fixtures | FR-21/FR-22, NR-5, UB-9/UB-11 green; frozen input byte-identity proven |
| Native resolution | resolve through every selected harness | implementation-role mappings, qualification, overrides, least-cost resolver, promotions, unsupported, evidence | FR-23..FR-26, NFR-7/NFR-8, EVAL-2, DOD-11/DOD-14 green |
| Worker integration | preserve execution semantics | connect resolved native settings to fresh sessions, claims, drive-to-green, routed outcomes | existing worker suite plus FR-25 and EVT-7..EVT-10 green; no claim precedes resolution |
| Pre-release dogfood | falsify the full seam | one real feature, at least three successfully verified native profiles, exact/promotion/unsupported/legacy audit, fallback, post-defect, and cutover drills | M1–M6 pre-release targets, every live blocking DoD row (DOD-5, DOD-8..DOD-18), and every §3.3 guardrail green |
| Production | replace the hardcoded/implicit path | every supported `/implement-now` implementation route | one direct cutover succeeds; evidence remains connected; unreachable old pin confirmed |

### op: supersede §6.2 Dependencies

Reason: plan-new demand is now available, while implementation requires the parent policy and a superseding design before code changes.

| Dependency | Type | Needed by | Status | Owner |
|---|---|---|---|---|
| plan-new plan/v2 demand and checkpoint contract (`prd:plan-new/FR-28..FR-30`) | approval / data contract | Demand boundary | available through merged amendment 004 | plan-new owner |
| planner-driven routing semantics, compatibility, resolver, evidence, and rollout (`prd:planner-driven-model-routing/FR-7..FR-12`, `FR-15`, `FR-17`, `FR-20..FR-24`, `FR-27`) | policy / contract | all phases | available through merged amendments | routing policy owner |
| superseding implement-now technical design | docs | Demand boundary | required after this amendment; must replace `tiermap(rung)` and hardcoded/implicit launch selection without changing Worker/Dispatcher ownership | @mdarabi |
| impl-review sibling amendment and superseding design | approval / docs | parent direct-cutover gate | required for full production activation; not owned by implement-now | impl-review owner |
| shared claim mechanism + automated-implementation tag | system | Worker integration | confirmed | plan-new / fleet convention |
| qualified current native model roster and cost metadata per harness implementation role | vendor / policy | Native resolution + release | required; mappings without reviewed qualification fail | @mdarabi + runtime adapter owners |
| four agent CLIs installed/authenticated | vendor | Worker integration | operator-provisioned | operator |
| merge-lane | system | post-run | as-is; review floor changes land through impl-review sibling | merge-lane |

### op: supersede §6.3 Risks, assumptions, and rabbit holes

Reason: routing adds stale-policy, transport, legacy, and direct-cutover risks that must be owned beside the original worker risks.

| # | Risk / assumption / rabbit hole | Likelihood | Impact | Mitigation or test | Owner |
|---|---|---|---|---|---|
| R1 | same-session charter bleed if any state leaks across tasks | M | H | fresh session per task (FR-5); no cross-task reuse; UB-3 drill | design |
| R2 | unbounded loop / token runaway from a drifted predicate | M | H | single-source predicates (FR-18); idle-free gate (FR-12/NR-7); bounded loops (FR-9/FR-13) | design |
| R3 | "green" is red or absent | M | H | artifact verification (FR-10); local gate mirrors CI (FR-8); EVAL-3 | design |
| R4 | selected harness adapter mapping goes stale as native catalogs or costs change | H | H | closed versioned policy, current qualification evidence, exhaustive EVAL-2, unsupported refusal, strongest-profile fallback | runtime adapter owners |
| R5 | one tracker carrier drops or mutates plan/v2 demand | M | H | consumer-side field-identity conformance for every supported carrier (FR-21/UB-9) | plan projection + executor owners |
| R6 | legacy translation becomes mutable inference or rewrites source | L | H | one versioned table, conservative absent-rung classification, unknown fail, byte-identity manifest (FR-22/NR-5/DOD-14) | compatibility owner |
| R7 | direct activation exposes a broad mapping defect | M | H | all-path pre-release proof, no-claim unsupported behavior, strongest-profile drill, and UB-13/DOD-18 exact-artifact invalidation and disposition | @mdarabi |
| R8 | native routing accidentally becomes harness selection or review orchestration | M | H | NR-6 and scope boundary; selected harness is input; impl-review sibling owns proposal-band/pass logic | @mdarabi + design reviewer |
| R9 | headless Claude cannot fully cover an autonomous run | M | M | Q-1 spike before adapter commitment; no weakening or harness fallback if unsupported | design |

### op: supersede §6.4 Release criteria

Reason: release must prove the complete implementation routing domain, no-rewrite compatibility, connected evidence, and direct-cutover fallback.

| Lens | Criterion | Evaluated by |
|---|---|---|
| Functionality | DOD-10 and every live Must fit criterion green; every non-empty task follows plan/v2 or typed plan/v1 demand | CI + compatibility suite |
| Routing conformance | every supported harness×derivable-profile implementation cell produces its qualified least-cost exact/promoted result or explicit unsupported terminal | EVAL-2 + DOD-11 operator rubric |
| Reliability | UB-1..UB-13 drills; no claim before route; no claim leak, duplicate proposal, downgrade, harness switch, orphan evidence, or accepted affected open artifact | failure injection + joined event/invalidation report |
| Security | existing abuse drills plus ABUSE-5; no merge credential, parent mutation, unqualified mapping, raw argument, or native plan identifier | tests + inspection |
| Compatibility | DOD-14 plan/v1 translation and byte identity; plan/v2 carrier identity across every tracker adapter | corpus and round-trip conformance |
| Dogfood | one real feature exercises at least three native profiles that each succeed with verified outcomes; all other resolved outcomes remain connected but do not count toward diversity | DOD-12 report |
| Rollout | direct cutover and strongest-profile fallback satisfy every semantic minimum without changing harness; old pin unreachable; post-cutover qualification defects trigger exact-artifact invalidation and disposition | DOD-13/DOD-18 drills + inspection |

### op: supersede §6.5 Rollout and reversibility

Reason: the routing parent requires pre-release proof followed by one production cutover, not the base capability's audience-stage rollout.

- **Flag** — one production implementation-routing policy switch, owned by @mdarabi, changes native resolution only after the full parent pre-release gate—including the impl-review sibling—is green.
- **Kill switch** — FR-26's strongest qualified profile within each already-selected harness. It may overprovide but never downgrade, change harness, bypass evidence, or claim an unsatisfiable task.
- **Experiment** — none; no adaptive optimization, shadow production decisions, or canary cohort.
- **Cutover** — validate closed defaults/overrides and qualification; pass every implementation conformance cell, v1 compatibility case, three-profile dogfood, evidence audit, direct-cutover rehearsal, and fallback drill; then activate every supported production `/implement-now` route in one switch. Remove the hardcoded/implicit rung/model/effort path only after successful activation.
- **Rollback** — engage the strongest-profile fallback and stop affected new claims; preserve frozen demand and immutable prior evidence. Unsatisfied demand remains unsupported. Under UB-13, append EVT-11 for exact affected in-flight/unaccepted attempts as `rerun_required`, preserve unaffected work, and rerun replacement evidence on the exact affected task/proposal revision; already accepted work receives an `operator_remediation` brief and never automatic destructive rollback.

| Stage | Audience | Minimum evidence | Advance when | Roll back when |
|---|---|---|---|---|
| Pre-release only | conformance fixtures and repository dogfood; no production routing | every live blocking DoD row (DOD-5, DOD-8..DOD-18) plus §6.4 report and parent full-seam gate | every M1–M6 pre-release target and guardrail is green | any failed criterion blocks activation |
| 100% production | every supported `/implement-now` implementation path | effective policy snapshot + successful direct-cutover/fallback/post-defect rehearsal | remains active while post-launch M4–M6 and every guardrail stay complete | any downgrade, harness switch, mapping defect, unsupported claim, or evidence break engages fallback and UB-13 invalidation |

### op: supersede §7.2 Open questions

Reason: Q-4 must preserve harness selection as a separate mechanism now that per-task native settings come from frozen semantic demand.

| # | Question | Owner | Needed by | Status |
|---|---|---|---|---|
| Q-1 | Does Claude Code's headless mode fully cover an autonomous multi-tool run under the native profile/effort settings its adapter must expose? | superseding tech design | design gate | open — must not weaken or switch harness if unsupported |
| Q-2 | The concrete worker turn, CI self-heal, and parallel-dispatch caps and their config home. | superseding tech design | design gate | open — orthogonal to semantic routing |
| Q-3 | Dependency ordering under never-merge: pause/reinvoke versus another non-stacking mechanism. | superseding tech design | design gate | open — never-merge remains fixed |
| Q-4 | How the invoker chooses the explicit/ambient harness and optional repo default without installed-CLI inference; plan/v2 intentionally contains no harness hint. | superseding tech design | design gate | open — selection is upstream of FR-23 and cannot be changed by resolution |
| Q-5 | Attended live progress versus unattended journal rendering over the same enriched event stream. | superseding tech design | design gate | open — behavior and evidence semantics are identical |

*None is solution-blocking: each selects a mechanism behind FR-21..FR-27, NR-5..NR-7, and the existing Worker boundaries. A mechanism that weakens those rows requires a new amendment, not a design choice.*

### op: supersede §7.3 Decision log

Reason: the immutable decision ledger must record the replacement routing and rollout boundary.

| Date | Decision | Alternatives considered | Why | Who |
|---|---|---|---|---|
| 2026-07-15 | Build durable worker core + swappable dispatch, local runtime first | throwaway script; tracker bypass | remote fleet reuses the core | @mdarabi (`brainstorm:implement-now#D-5`, `#D-11`) |
| 2026-07-15 | Four fresh-headless CLIs behind one runtime | Claude-only; warm sessions | portable contract; no charter bleed | @mdarabi (`brainstorm:implement-now#D-1`) |
| 2026-07-15 | Stop at genuinely-green PRs and never merge | drive to merged | preserves merge authority | @mdarabi (`brainstorm:implement-now#D-3`) |
| 2026-07-15 | Full tracker-mediated execution | tracker bypass; hybrid | one projection and shared claims | @mdarabi (`brainstorm:implement-now#D-4`) |
| 2026-07-15 | Operator and argus drive one core | operator-only | attendedness is a mode | @mdarabi (`brainstorm:implement-now#D-6`) |
| 2026-07-15 | Hardware separation and umbrella policy stay in fleet | build them here | separate runtime concern | @mdarabi (`brainstorm:implement-now#D-7`) |
| 2026-07-15 | Unrecoverable task failure skips-and-continues with escalation | halt whole run | independent work continues | @mdarabi (`brainstorm:implement-now#D-12`) |
| 2026-07-27 | Consume frozen plan/v2 demand verbatim; translate plan/v1 without rewrite; selected harness implementation adapter maps to least-cost qualified native profile and may promote but never downgrade or switch harness. (`prd:planner-driven-model-routing/FR-7`, `FR-8`, `FR-10`, `FR-15`) | keep optional rung/hardcoded pin; infer complexity at claim; store native settings in plan; cross-harness fallback | preserves richest-context judgment, artifact portability, explicit credential/egress choice, and fail-closed execution | @mdarabi |
| 2026-07-27 | Test the complete seam pre-release, then activate every supported production path directly with strongest-profile fallback. (`prd:planner-driven-model-routing/FR-17`, `FR-27`) | shadow/canary production; mixed old/new paths | one auditable semantics after full proof; bounded emergency overprovision without weakening | @mdarabi |

### op: supersede §8 Contract readiness — the executability gate

Reason: readiness must point to the live routing, compatibility, evidence, and rollout contract and its required design supersession.

| Check | Pass? | Notes |
|---|---|---|
| Problem basis accepted — §2 problem gate signed (T3) | ☐ | base gate remains pending the introducing review; amendment does not rewrite its historical status |
| Zero solution-blocking TBDs or open questions (§7.2) | ☑ | Q-1..Q-5 are design mechanisms constrained by live requirements; unsupported cannot weaken or switch harness |
| Every live Must FR has a fit criterion and verification method (§5.1) | ☑ | FR-1..FR-5, FR-7..FR-10, FR-12..FR-18, FR-20..FR-27 |
| Every requirement row carries risk; high/critical rows have a safety plan | ☑ | pre-claim validation, no-rewrite translation, exhaustive conformance, selected-harness refusal, immutable evidence, direct-cutover/fallback drill |
| Non-goals drawn; non-regression rows named | ☑ | NG-1..NG-4; NR-1..NR-7; routing does not absorb harness selection or review orchestration |
| Every primary job covered | ☑ | J1→FR-1/7/15/21..27; J2→FR-17/NR-4; J3→FR-7/8/9/25 |
| Every key flow has functional, failure, and non-regression coverage | ☑ | F1→FR-1..FR-27, UB-1..UB-13, NR-1..NR-7; unsupported, legacy, and post-cutover defect states are explicit |
| Every primary metric has instrumentation or plan | ☑ | M1–M6→EVT-4, EVT-6..EVT-10 with complete origin/terminal/outcome denominators |
| Every blocking DoD row traces to source/rubric | ☑ | DOD-5, DOD-8..DOD-18; DOD-11 names @mdarabi and complete-cell rubric; DOD-15..DOD-18 cover all new consequence and post-defect rows |
| Risks and assumptions have owners | ☑ | R1..R9 owned, including mapping staleness, transport, legacy, cutover, and ownership creep |
| Human-judgment criteria name owner and rubric | ☑ | DOD-11 names @mdarabi and its cell qualification rubric; other rows are mechanical/demo/analysis |
| DoD complete; n/a explained | ☑ | routing domain, legacy compatibility, successfully verified dogfood diversity, all consequence rows, direct cutover, fallback, and post-cutover defect disposition are separately blocking |
