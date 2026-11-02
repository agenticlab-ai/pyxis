# Amendment 003 — append-first-routing-evidence

| Field | Value |
|---|---|
| **Amends** | implement-now *(chain through 002)* |
| **Status** | Approved |
| **Owner** | @mdarabi |
| **Date** | 2026-07-28 |
| **Learned-from** | operator-decision:2026-07-28-canonical-git-evidence-before-launch |
| **Blast radius** | tier: contract-changing; touches: NFR-8, NFR-9, UB-10, UB-18, DOD-16, DOD-20, §0, §3.3, §5.3, §5.5, §6.1, §6.2, §6.4, §6.5, §8; reason: keep native route selection deterministic, offline, and inference-free; land a backward-compatible consumer boundary first; let the coordinated parent amendment activate mandatory canonical git evidence append/exact-read before claim or worker invocation; and move every live failure, release, dependency, and readiness reference to the replacement rows |
| **Template version** | amendment-template/v2 |

## Operations

### op: supersede NFR-8

Reason: append-first admission requires durable canonical routing evidence before claim or worker invocation, while the current fit criterion forbids the evidence store itself in that interval.

| ID | Area | Requirement | Fit criterion | Verify |
|---|---|---|---|---|
| NFR-9 | Determinism / cost | For identical frozen or translated demand, selected harness, implementation role, effective mapping policy, and qualification evidence, routing resolution shall return one identical least-cost exact/promoted/unsupported result using zero model inference and zero network I/O. Until the coordinated parent append-first policy is effective, the compatibility path shall retain the existing zero-network pre-claim boundary. Once that parent policy is effective, after every terminal result exists and before task claim or selected worker invocation, the executor shall durably append that terminal to the configured canonical git evidence store and exact-read it successfully. The append and exact-read are then the only network operations permitted in that interval: no policy lookup, qualification lookup, harness probe, model call, or other outbound operation may occur. | repeated and reordered-input fixtures are byte-identical; exhaustive candidate-order tests select the same cheapest qualified dominating result; dependency inspection and boundary traces record zero inference/network I/O through resolution and prove the pre-parent compatibility path remains zero-network; after parent activation every healthy-path trace contains a successful canonical append and exact-read before claim or invocation and no other network operation; a zero-evidence healthy path fails, and unavailable or unverifiable evidence produces zero claim and zero worker invocation | Test + Analysis + Inspection + Failure injection |

### op: supersede UB-10

Reason: invalid or unsupported native resolution must retain its pre-claim refusal while allowing its canonical failure terminal to cross the sole permitted evidence boundary.

| ID | If… (trigger) | Then the system shall… | User sees | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|---|
| UB-18 | an effective native mapping or qualification is missing, malformed, incompatible, weakening, injected, or unable to satisfy the task profile | apply FR-24 and stop before any claim or worker invocation; before coordinated parent activation, retain the existing zero-network refusal-evidence path; after activation, append and exact-read EVT-8 through the canonical git evidence store before reporting the refusal; if append or exact-read fails, assert no verified canonical EVT-8, report a routing-evidence-admission failure, leave the task unclaimed and retryable, and perform no other inference or network operation | named policy-invalid or unsupported result with selected harness and unmet components, or after activation a named evidence-admission failure with the task and attempted terminal identity | Must | critical | before parent activation every trigger records its refusal with zero claims/invocations and zero network I/O; after activation, with a healthy evidence store every trigger records exactly one verified EVT-8 before refusal, performs zero claims/invocations and no harness substitution, and records no network activity except canonical append/exact-read; append-failure and exact-read-failure fixtures assert no verified canonical EVT-8, produce zero claims/invocations, expose the named evidence-admission failure, and leave the task retryable; a successful retry appends exactly one immutable EVT-8; independent satisfiable tasks may continue | Test + Failure injection | FR-23, FR-24, NFR-9 |

### op: supersede DOD-16

Reason: the complete unwanted-behavior proof must cover the live append-first failure row and every already-live e2e consequence row.

| ID | Done criterion | Source | Verify | Required evidence | Blocking? |
|---|---|---|---|---|---|
| DOD-20 | Every live unwanted-behavior row is handled. | UB-1..UB-9, UB-11..UB-18 | Test + Failure injection + Analysis + Inspection | failure-injection matrix with one result per live UB row, including canonical-evidence unavailability and zero omitted critical path | yes |

### op: supersede §0 Read to rule — the human reviewer's section (~6 min)

Reason: §0's routing summary must distinguish offline native resolution from the one permitted append-first evidence network boundary and cite live NFR-9.

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
| 11 | e2e tasks are recognized from a **reference the plan already records** in a required header field, never inferred from charter prose | Add an e2e task type or charter field upstream; or let the worker infer the origin itself | The planner would fork its schema and its projection; inference cannot deliver "no feature task is ever misread as e2e" → FR-28, FR-29, NR-8 |
| 12 | An e2e worker **writes the test, never the feature**, and honors the scenario's recorded route — a `manual-human` scenario spawns nothing | Let the worker change behavior under test to reach green, or pick a cheaper route at run time | A green proposal would prove less than the approved e2e design says it proves → FR-30, FR-31, UB-15, UB-16 |
| 13 | An authored e2e test must be **hermetic before it counts green**, and a flaky one leaves the self-heal loop immediately | Let the ordinary red→fix loop handle it, trusting the CI mirror | A check whose verdict depends on the machine is a measurement, not a gate — the loop `incident:I5` burned → FR-32, UB-17, AP-29 |
| 14 | Native route selection is offline and inference-free; this consumer lands with the existing zero-network boundary, then the coordinated parent activates canonical git append/exact-read as the only pre-claim/launch network operation | Activate the consumer prematurely, buffer evidence locally after parent activation, or allow arbitrary preflight/network lookups | Consumer-first landing avoids a cyclic contract change; parent activation prevents an orphan route or hidden dispatch dependency → NFR-9, UB-18, §6.2 |

**Sanity-check the proof** — model routing is not a scalar complexity guess. The planner freezes six assessment axes and four independent semantic minima. `implement-now` proves field-identical transport, a complete harness×derivable-profile implementation-role matrix, at least three distinct native profiles in dogfood, no-rewrite v1 translation, and exact/promoted/unsupported evidence. The routing resolver itself spends zero inference and zero network I/O; only canonical evidence append/exact-read may occur between resolution and invocation (NFR-9). For the e2e class the proof is a real e2e plan run end to end, with one deliberately-broken-subject falsification and one repeat-run verdict-stability record per automatable scenario (DOD-19) — the cheapest evidence that a green e2e proposal is neither hollow nor flaky.

**The author's open questions** — whether Claude Code's headless mode fully covers an autonomous multi-tool run remains open (§7.2 Q-1). Harness *selection* mechanics remain design-owned (§7.2 Q-4); demand resolution never answers that question or scans for another harness. *How* the executor joins a recorded e2e reference to the individual scenario is likewise design-owned — FR-28 fixes the signal and the outcome and NR-8 fixes the constraint, not the join mechanism. The other open rows are implementation mechanics behind settled requirements.

**Skip the rest** — structure/ID/fold correctness → docgate; substance and coherence → `/prd-review`; implementation mechanisms → the superseding technical design; review model/process routing → the impl-review sibling amendment; merge safety → merge-lane; plan judgment and projection → plan-new; what an e2e scenario, route, or mock strategy should be → e2e-design and e2e-review, already settled by the doc's approval.

### op: supersede §3.3 Guardrails and decision rule

Reason: the release predicate must include replacement DOD-20 while preserving every already-live e2e and routing guardrail.

| Guardrail metric | Threshold | Action if breached |
|---|---|---|
| Parent-checkout mutation | any mutation of the parent clone / engine checkout outside a worktree | fail-closed; halt the offending worker and alarm |
| Double-claim with the external fleet | any task worked concurrently by both a local worker and a fleet node | fail-closed; block release until the claim mechanism is fixed |
| Self-merge | any merge performed by the executor | critical defect; executor holds no merge credential |
| Idle cost | any routing resolution or model spend on an empty ready frontier | investigate the frontier gate and block release on a model launch |
| Semantic downgrade or harness switch | any native launch fails to satisfy one requested component, uses an unqualified mapping, or changes the selected harness | stop affected claims; block/rollback immediately; repair policy and rerun affected attempts |
| Frozen-plan mutation or post-freeze inference | any plan/v1 rewrite or plan/v2 assessment/profile reinterpretation | invalidate affected evidence; require no-rewrite translation or field-identical transport before retry |
| Routing evidence break | any attempt lacks exactly one origin/terminal or a resolved attempt lacks exactly one outcome | do not count it succeeded or toward release; preserve artifacts and repair the evidence chain |

**Ship rule (pre-registered):** M1–M6 meet their pre-release targets, every live blocking DoD row (DOD-5, DOD-8..DOD-15, DOD-17..DOD-20) is green, and every guardrail stays clean through complete pre-release proof → activate every supported production implementation path in one cutover; post-launch M4–M6 and every guardrail remain continuously evaluated; any breach → no activation or immediate strongest-profile fallback and affected-attempt invalidation.

### op: supersede §5.3 Permissions and abuse cases

Reason: abuse mitigation must cite the live append-first failure row rather than superseded UB-10.

*Access: `/implement-now` runs under the invoking identity (the operator's, attended; argus's grant, unattended). Workers run headless with bypass-permission autonomy inside their worktrees. The executor holds no merge/approve credential (NR-3).*

| Function / data | Operator | argus (granted) | Unauthenticated |
|---|---|---|---|
| invoke a run | invoke | invoke (under grant) | none |
| worker: modify code in its worktree | via worker | via worker | none |
| merge a proposal | none (executor never merges) | none | none |

| ID | Actor and intent | Vector | Impact | Severity | Mitigated by | Verify |
|---|---|---|---|---|---|---|
| ABUSE-1 | tracker/repo/CI content bends a worker into acting on injected instructions | untrusted text read during a task treated as instruction | worker does attacker-directed work | high | FR-3, FR-19 (charter is the instruction; read content is data) + NG-1 (can't merge) + FR-4 (worktree-bounded) | drill: injected content in a fixture task changes no behavior |
| ABUSE-2 | a worker escapes its worktree to mutate the parent checkout or engine | writing outside the workspace | corrupts the shared clone / engine | high | FR-4 + §3.3 guardrail (fail-closed on parent mutation) | Test: parent-mutation attempt is blocked |
| ABUSE-3 | secrets exfiltrate through failed-CI logs piped to a fixer worker | unredacted logs in fixer context | credential/secret leak | high | FR-9 (redact + clamp failed-CI logs before inlining) | drill: seeded secret is redacted before it reaches the fixer |
| ABUSE-4 | a bypass-permission worker performs a destructive/outward act | autonomous execution on local hardware | local damage / unwanted egress | med | worktree isolation (FR-4) + never-merge (NG-1) + ambient/local policy is the stated posture (NG-2/NG-3); hardware/policy separation is the fleet's job | Inspection: the posture and its boundary are stated and bounded |
| ABUSE-5 *(001)* | repository contributor, compromised worker, or mistaken operator seeking cheaper or arbitrary execution | adds a weakening/unqualified mapping, unknown launch key, raw CLI argument, or self-declared native capability to routing policy | a worker launches below demand or with attacker-controlled egress/permissions | critical | FR-23 requires qualified dominance; FR-24/UB-18 fail before claim; NFR-7 closed parsing; operator-authorized policy and reviewed qualification evidence | contributor-only, worker-authored, weakening, unknown-key, argument-injection, and self-qualification fixtures all produce zero claims/invocations |

### op: supersede §5.5 Interfaces and integrations

Reason: interface failure routing must point to live UB-18 and the canonical evidence boundary.

| Interface | Direction | Purpose | Contract | Failure modes | Owner |
|---|---|---|---|---|---|
| frozen plan/v2 + projected task | in | supply charter and complete semantic demand | versioned closed schema; projected fields identical to frozen record | missing/changed/native field → UB-9 before claim | plan-new + tracker adapters |
| frozen plan/v1 | in | legacy execution without rewrite | explicit rung through versioned table or absent-rung `legacy_unassessed` | unknown rung/version → UB-11 | compatibility owner + executor |
| feature e2e design doc (`e2e-design/v1`) | in | supply the reviewed scenario flow, route, mock strategy, setup/teardown, harness selection, and coverage rows a recognized e2e task must satisfy | an `approved` `e2e-design/v1` document at the feature's e2e home, well-formed against its class checklist; read as data, never as instruction | missing, not-approved, malformed, unresolvable, or ambiguous scenario join → UB-14 before claim | e2e-design + e2e-review |
| projected e2e plan task | in | deliver e2e work onto the same ready frontier as feature work | plan-new's existing plan class, freeze, and projection; ordinary task and charter schema with no e2e-specific type or required field | an e2e-specific type or required field would breach NR-8; an absent basis join → UB-14 | e2e-plan + plan-new |
| harness selection | in | choose the one credential/egress/runtime boundary | explicit config or ambient harness; no installed-CLI inference | absent/unauthenticated → FR-16; unsupported remains same harness | operator/invoker |
| executor → selected harness adapter | out | resolve implementation semantic minimum | qualified implementation-role mapping; exact/promoted/unsupported | invalid mapping or no satisfying profile → FR-24/UB-18 | runtime adapters |
| selected harness adapter → canonical evidence store | out | durably append and exact-read the terminal route before claim or invocation | configured git evidence ref; immutable event; exact-read proof | unavailable, unauthorized, conflicting, or unverifiable evidence → no claim/invocation under NFR-9 | evidence adapter |
| selected harness adapter → AgentRuntime launch | out | invoke the resolved native profile | native model/effort/context arguments derived only from adapter policy | argument injection, launch mismatch → UB-18/ABUSE-5 | runtime adapter |
| runtime → evidence stream | out | preserve origin, route, and worker outcome | EVT-7..EVT-10 and EVT-12, immutable and payload-free | orphan/duplicate/missing edge → UB-12 | executor + evidence owner |
| code host | both | proposals + CI | existing proposal/check surface | lag bounded; malformed is not green | this feature |
| merge-lane | out | landing downstream | opens proposals; never merges | no landing behavior owned here | merge-lane |

### op: supersede §6.1 Phasing and milestones

Reason: the native-resolution and pre-release exits must cite live NFR-9 and replacement DOD-20.

| Phase | Purpose | Scope | Exit criterion |
|---|---|---|---|
| Demand boundary | consume without re-judging | plan/v2 integrity/transport, plan/v1 translation, origin identities, negative fixtures | FR-21/FR-22, NR-5, UB-9/UB-11 green; frozen input byte-identity proven |
| Native resolution | resolve through every selected harness | implementation-role mappings, qualification, overrides, least-cost resolver, promotions, unsupported, append-first canonical evidence | FR-23..FR-26, NFR-7/NFR-9, EVAL-2, DOD-11/DOD-14 green |
| Worker integration | preserve execution semantics | connect resolved native settings to fresh sessions, claims, drive-to-green, routed outcomes | existing worker suite plus FR-25 and EVT-7..EVT-10 green; canonical route evidence precedes claim |
| Pre-release dogfood | falsify the full seam | one real feature, at least three successfully verified native profiles, exact/promotion/unsupported/legacy audit, fallback, post-defect, and cutover drills | M1–M6 pre-release targets, every live blocking DoD row (DOD-5, DOD-8..DOD-15, DOD-17..DOD-20), and every §3.3 guardrail green |
| Production | replace the hardcoded/implicit path | every supported `/implement-now` implementation route | one direct cutover succeeds; evidence remains connected; unreachable old pin confirmed |

### op: supersede §6.2 Dependencies

Reason: this consumer amendment must land as a dormant compatibility prerequisite before the parent policy activates append-first admission, removing the cyclic landing dependency.

| Dependency | Type | Needed by | Status | Owner |
|---|---|---|---|---|
| plan-new plan/v2 demand and checkpoint contract (`prd:plan-new/FR-28..FR-30`) | approval / data contract | Demand boundary | available through merged amendment 004 | plan-new owner |
| planner-driven routing semantics, compatibility, resolver, evidence, and rollout (`prd:planner-driven-model-routing/FR-7..FR-12`, `FR-15`, `FR-17`, `FR-20..FR-24`, `FR-27`) | policy / contract | all phases | parent through amendment 002 is approved; this implement-now amendment must land first with the zero-network compatibility path, then `planner-driven-model-routing/003-append-first-routing-evidence` activates mandatory canonical admission; ancestry is proven against main before the parent PR is merged | routing policy owner |
| superseding implement-now technical design | docs | Demand boundary | required after this amendment; must replace `tiermap(rung)` and hardcoded/implicit launch selection without changing Worker/Dispatcher ownership | @mdarabi |
| e2e-plan records the feature's approved e2e design doc as the plan's governing-document header reference | data contract | e2e consumption | required before FR-28 can resolve any origin; uses the plan class's existing required header field and needs no plan-new or plangate change | e2e-plan owner |
| approved `e2e-design/v1` documents at their per-feature home (`prd:e2e-verification/FR-1`, `FR-4`, `FR-5`) | data contract | e2e consumption | available through the merged e2e authoring line; a non-approved or malformed document refuses under UB-14 | e2e-design + e2e-review owners |
| impl-review amendment 003 and superseding design | approval / docs | parent direct-cutover gate | consumer amendment 003 must land before the coordinated planner amendment; full review activation and design remain required and are not owned by implement-now | impl-review owner |
| shared claim mechanism + automated-implementation tag | system | Worker integration | confirmed | plan-new / fleet convention |
| qualified current native model roster and cost metadata per harness implementation role | vendor / policy | Native resolution + release | required; mappings without reviewed qualification fail | @mdarabi + runtime adapter owners |
| four agent CLIs installed/authenticated | vendor | Worker integration | operator-provisioned | operator |
| merge-lane | system | post-run | as-is; review floor changes land through impl-review sibling | merge-lane |

### op: supersede §6.4 Release criteria

Reason: release proof must cover live UB-18 and the offline-resolution/append-first boundary.

| Lens | Criterion | Evaluated by |
|---|---|---|
| Functionality | DOD-10 and every live Must fit criterion green; every non-empty task follows plan/v2 or typed plan/v1 demand | CI + compatibility suite |
| Routing conformance | every supported harness×derivable-profile implementation cell produces its qualified least-cost exact/promoted result or explicit unsupported terminal; resolution uses zero inference/network I/O, the pre-parent compatibility path remains zero-network, and after parent activation canonical evidence is exact-read before claim | EVAL-2 + DOD-11 operator rubric + NFR-9 boundary traces |
| Reliability | UB-1..UB-9 and UB-11..UB-18 drills; after parent activation no claim precedes exact-read route evidence; no claim leak, duplicate proposal, downgrade, harness switch, orphan evidence, or accepted affected open artifact | failure injection + joined event/invalidation report |
| Security | existing abuse drills plus ABUSE-5; no merge credential, parent mutation, unqualified mapping, raw argument, native plan identifier, or unauthorized pre-launch network operation | tests + inspection |
| Compatibility | DOD-14 plan/v1 translation and byte identity; plan/v2 carrier identity across every tracker adapter | corpus and round-trip conformance |
| Dogfood | one real feature exercises at least three native profiles that each succeed with verified outcomes; all other resolved outcomes remain connected but do not count toward diversity | DOD-12 report |
| Rollout | direct cutover and strongest-profile fallback satisfy every semantic minimum without changing harness; old pin unreachable; post-cutover qualification defects trigger exact-artifact invalidation and disposition | DOD-13/DOD-18 drills + inspection |

### op: supersede §6.5 Rollout and reversibility

Reason: the pre-release gate must include the live DOD set through replacement DOD-20.

- **Flag** — one production implementation-routing policy switch, owned by @mdarabi, changes native resolution only after the full parent pre-release gate—including the impl-review sibling—is green.
- **Kill switch** — FR-26's strongest qualified profile within each already-selected harness. It may overprovide but never downgrade, change harness, bypass evidence, or claim an unsatisfiable task.
- **Experiment** — none; no adaptive optimization, shadow production decisions, or canary cohort.
- **Cutover** — validate closed defaults/overrides and qualification; pass every implementation conformance cell, v1 compatibility case, three-profile dogfood, evidence audit, direct-cutover rehearsal, and fallback drill; then activate every supported production `/implement-now` route in one switch. Remove the hardcoded/implicit rung/model/effort path only after successful activation.
- **Rollback** — engage the strongest-profile fallback and stop affected new claims; preserve frozen demand and immutable prior evidence. Unsatisfied demand remains unsupported. Under UB-13, append EVT-11 for exact affected in-flight/unaccepted attempts as `rerun_required`, preserve unaffected work, and rerun replacement evidence on the exact affected task/proposal revision; already accepted work receives an `operator_remediation` brief and never automatic destructive rollback.

| Stage | Audience | Minimum evidence | Advance when | Roll back when |
|---|---|---|---|---|
| Pre-release only | conformance fixtures and repository dogfood; no production routing | every live blocking DoD row (DOD-5, DOD-8..DOD-15, DOD-17..DOD-20) plus §6.4 report and parent full-seam gate | every M1–M6 pre-release target and guardrail is green | any failed criterion blocks activation |
| 100% production | every supported `/implement-now` implementation path | effective policy snapshot + successful direct-cutover/fallback/post-defect rehearsal | remains active while post-launch M4–M6 and every guardrail stay complete | any downgrade, harness switch, mapping defect, unsupported claim, or evidence break engages fallback and UB-13 invalidation |

### op: supersede §8 Contract readiness — the executability gate

Reason: readiness accounting must name the live replacement UB and DOD rows so the amendment cannot disappear from the declared proof denominator.

| Check | Pass? | Notes |
|---|---|---|
| Problem basis accepted — §2 problem gate signed (T3) | ☐ | base gate remains pending the introducing review; amendments do not rewrite its historical status |
| Zero solution-blocking TBDs or open questions (§7.2) | ☑ | Q-1..Q-5 are design mechanisms constrained by live requirements; unsupported cannot weaken or switch harness; e2e-origin derivation is likewise a mechanism behind FR-28 and NR-8 |
| Every live Must FR has a fit criterion and verification method (§5.1) | ☑ | FR-1..FR-5, FR-7..FR-10, FR-12..FR-18, FR-20..FR-32 |
| Every requirement row carries risk; high/critical rows have a safety plan | ☑ | pre-claim validation, no-rewrite translation, exhaustive conformance, selected-harness refusal, append-first immutable evidence, direct-cutover/fallback drill, e2e basis refusal, mandate/route consequence rows, and hermeticity before green |
| Non-goals drawn; non-regression rows named | ☑ | NG-1..NG-5; NR-1..NR-8; routing does not absorb harness selection or review orchestration, and execution does not absorb e2e coverage authoring |
| Every primary job covered | ☑ | J1→FR-1/7/15/21..27, FR-28/29; J2→FR-17/NR-4; J3→FR-7/8/9/25, FR-30/31/32 |
| Every key flow has functional, failure, and non-regression coverage | ☑ | F1→FR-1..FR-32, UB-1..UB-9, UB-11..UB-18, NR-1..NR-8; unsupported, legacy, evidence-unavailable, post-cutover defect, e2e-basis, and non-hermetic states are explicit |
| Every primary metric has instrumentation or plan | ☑ | M1–M6→EVT-4, EVT-6..EVT-10 with complete origin/terminal/outcome denominators; EVT-12 carries the e2e origin decision |
| Every blocking DoD row traces to source/rubric | ☑ | DOD-5, DOD-8..DOD-15, DOD-17..DOD-20; DOD-11 names @mdarabi and complete-cell rubric; DOD-15/DOD-17..DOD-20 cover non-regression, security, consequence, post-defect, e2e, and append-first failure rows |
| Risks and assumptions have owners | ☑ | R1..R9 owned, including mapping staleness, transport, legacy, cutover, and ownership creep; the e2e re-implementation risk is bounded by FR-30/FR-31 and UB-16, and the flaky-check risk `incident:I5` paid for by FR-32 and UB-17 |
| Human-judgment criteria name owner and rubric | ☑ | DOD-11 names @mdarabi and its cell qualification rubric; UB-15 routes manual-human scenarios to the owner the approved design doc records; other rows are mechanical/demo/analysis |
| DoD complete; n/a explained | ☑ | routing domain, legacy compatibility, successfully verified dogfood diversity, all consequence rows, direct cutover, fallback, post-cutover defect disposition, append-first evidence admission, and the real-e2e-plan run are separately blocking |
