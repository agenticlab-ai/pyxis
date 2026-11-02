# Amendment 004 — observable impl-review terminal

| Field | Value |
|---|---|
| **Amends** | implement-now *(chain through 003)* |
| **Status** | Approved |
| **Owner** | @mdarabi |
| **Date** | 2026-08-06 |
| **Learned-from** | review-thread:pyxis#1034 · review-thread:pyxis#1041 · review-thread:issue-1029 |
| **Blast radius** | tier: contract-changing; touches: FR-12, FR-33..FR-39, NR-7, NR-9, UB-19, EVT-6, EVT-13, EVT-14, DOD-10, DOD-15, DOD-20..DOD-26, M7, §0, §3.2, §3.3, §4.3, §5.4, §5.5, §5.7, §6.1, §6.2, §6.3, §6.4, §6.5, §8; reason: a configured advisory reviewer gains a durable proposal-revision obligation, a separate append-only attempt terminal, closed failure classes, review-only re-invocation semantics, and an explicit driver contract while implementation task outcomes remain unchanged |
| **Tracking** | issue:1029 · PR #1034 |
| **Template version** | amendment-template/v2 |

## Operations

### op: add FR

Reason: issue 1029 and its implementation review proved that a green implementation can silently omit its configured review unless review success is artifact-verified and review absence is run-significant.

| ID | Requirement (EARS) | Rationale | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|
| FR-33 | When an explicitly configured review applies to a genuinely-green proposal, the executor shall establish exactly one durable pending review obligation keyed to the task, proposal, current revision, backend, and effective review policy, then release the implementation claim and dispatcher width slot before any reviewer launch; inability to persist or recover that obligation shall still release both resources and fail the run as review infrastructure while leaving the proposal discoverable for reconciliation. | review latency must not suppress implementation throughput, and a crash after green must not erase the obligation | Must | critical | crash-point fixtures before and after resource release recover one pending obligation for the exact revision, never hold a claim or width slot during review, and classify an obligation-store failure as run-red infrastructure without redispatching the implementation | Test + Failure injection | G2, J1, FR-10, FR-15 |
| FR-34 | When a pending review obligation is eligible for an attempt, the executor shall use the already-selected harness's qualified review-role catalog and policy, preflight the configured review backend before avoidable implementation spend, and pass the resolved native model and effort without implementation-only controls or another harness's settings. | review is a separately qualified role, not an implementation rung or a reason to switch credential/egress boundaries | Must | critical | every supported adapter resolves the expected review-role profile for a non-pyxis repository and a future injected clock, propagates model/effort into launch, rejects unauthorized policy, omits worker-only controls, and reports missing auth before task dispatch where preflight can know it | Test + Analysis + Failure injection | G2, G3, FR-5, FR-16, NR-6 |
| FR-35 | When a pending review obligation has no exact current artifact at its pre-launch read, the executor shall launch at most one bounded sibling reviewer attempt for that obligation in one invocation, assign a fresh attempt identity, and deduplicate concurrent or repeated invocations against the stable obligation identity. | retries are legitimate across invocations, but duplicate reviewers in one invocation or after a late artifact appears waste spend and race persistence | Must | high | duplicate-invocation and replica-lag fixtures produce at most one launch per obligation per invocation, a late exact artifact suppresses a second launch, and each actual retry has one distinct attempt ID joined to the same obligation | Test + Failure injection | G2, J1, FR-9 |
| FR-36 | When a configured review attempt settles, the executor shall mark its obligation verified only after exact-reading one schema-valid review artifact bound to the obligation's proposal and revision; process exit, output text, a malformed or foreign artifact, and a different revision shall never establish completion. | the artifact is the review's goal object and clean exit is not evidence that it landed | Must | critical | CRLF, malformed, foreign-proposal, stale-revision, duplicate, pagination, and replica-lag fixtures accept only the exact current artifact; clean process exit without it leaves the obligation pending | Test + Failure injection | G2, FR-10 |
| FR-37 | If review setup or an attempt cannot verify its obligation, the executor shall append an escalated EVT-13 terminal with `failure_class=artifact_unverified` when the reviewer completed but no exact artifact can be verified, or `failure_class=infrastructure` for obligation-store, auth, routing, launch, runtime, truncation, cancellation, or read-transport failure; it shall preserve the implementation's succeeded EVT-10, increment the matching failure count and `review_failed`, retain every unresolved obligation in `review_pending`, and exit non-zero. | operators and unattended consumers must distinguish missing review output from broken review machinery without rewriting green implementation truth | Must | critical | one fixture per failure family retains a succeeded EVT-10, emits one typed escalated EVT-13, increments only its matching class plus `review_failed`, reports the unresolved obligation in `review_pending`, and exits 1; a verified attempt has no failure class and exits 0 when no other work is red | Test + Failure injection | G2, J1, FR-15, FR-16 |
| FR-38 | When the executor or its unattended driver is re-invoked, it shall reconcile every in-scope pending review obligation before a clean terminal even when the implementation frontier is empty: exact-read first, otherwise perform only the bounded review attempt allowed by FR-35; the driver shall treat `review_pending>0` as review-only incomplete work, schedule bounded review-only re-invocations under its retry budget, never redispatch the succeeded implementation, and keep the drive red and escalated when that budget exhausts. | a one-run red signal is not durable if the next empty implementation sweep launders it to green | Must | critical | a kill-after-green fixture restarts with an empty implementation frontier, launches zero implementation workers, retries the exact proposal revision, remains exit 1 while pending, clears to exit 0 only after artifact verification, and remains visibly red after driver retry exhaustion | Test + Failure injection + Demo | G1, G2, J1, FR-14, UB-2 |

### op: supersede FR-12

Reason: a frontier with no implementation tasks is not idle while a durable review obligation remains pending.

| ID | Requirement (EARS) | Rationale | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|
| FR-39 | While both the ready implementation frontier and the pending review-obligation frontier are empty, the executor shall consume no model tokens; when only review obligations are pending, it may perform only FR-35 review attempts and shall perform zero implementation routing, claims, or worker launches. | zero work stays free without misclassifying durable review repair as an empty run | Must | med | combined-empty fixtures emit zero routing, claim, review, or model activity; review-only fixtures emit only the bounded reviewer activity and zero implementation activity | Test | §3.3, NR-9, FR-38 |

### op: supersede NR-7

Reason: the protected zero-work boundary must distinguish a genuinely empty run from a review-only reconciliation run.

| ID | Existing behavior protected | Why protected | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|
| NR-9 | A run with no ready implementation task and no pending review obligation performs no routing resolution, claim, reviewer launch, or model invocation; a review-only run never routes or claims implementation work. | zero combined work remains zero inference, while review repair cannot be laundered as idle | high | combined-empty fixtures emit zero work; pending-review fixtures launch no implementation worker and leave every task/proposal identity unchanged | Test + Inspection | FR-38, FR-39 |

### op: add EVT

Reason: review attempts have a retryable lifecycle independent of the immutable implementation task outcome and therefore require their own append-only terminal event.

| ID | Event name | Fires when | Properties (typed) | Source | Feeds |
|---|---|---|---|---|---|
| EVT-13 | `impl_review_terminal` | configured-review setup fails or one review-obligation attempt reaches a verified or failed terminal | `review_attempt_id:string`, `review_obligation_id:string?`, `task_id:string?`, `proposal_ref:string?`, `revision:string?`, `backend:string`, `native_profile:string?`, `catalog:string?`, `policy:string?`, `model:string?`, `effort:string?`, `phase:enum{setup,obligation,routing,launch,await,artifact}`, `status:enum{verified,failed}`, `obligation_status:enum{verified,pending,not_created}`, `failure_class:enum{artifact_unverified,infrastructure}?`, `artifact_ref:string?`, `reason:string?`, `escalated:boolean` | executor | M7, FR-33..FR-38, UB-19 |

### op: supersede EVT-6

Reason: unattended consumers need a run-level denominator that distinguishes verified reviews from failed or absent configured reviews.

| ID | Event name | Fires when | Properties (typed) | Source | Feeds |
|---|---|---|---|---|---|
| EVT-14 | `run_finished` | a run ends after implementation work and configured-review reconciliation settle for that invocation | `succeeded:int`, `failed:int`, `blocked:int`, `tokens:int`, `routing_exact:int`, `routing_promoted:int`, `routing_unsupported:int`, `routing_invalid:int`, `review_completed:int`, `review_failed:int`, `review_pending:int`, `review_artifact_unverified:int`, `review_infrastructure_failed:int` | executor | M1, M2, M4, M6, M7, FR-37, FR-38 |

### op: add UB

Reason: review failure and interruption need a durable recovery path that cannot be erased by an empty implementation frontier.

| ID | If… (trigger) | Then the system shall… | User sees | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|---|
| UB-19 | a configured review is interrupted, fails infrastructure, produces no verifiable artifact, or remains pending when a later invocation has no ready implementation task | preserve or re-derive the exact proposal-revision obligation, release all implementation resources, append the typed failed attempt when one occurred, exact-read before any retry, and let the driver perform only bounded review-only re-invocations; never redispatch the succeeded task or report a clean run while the obligation remains pending | the implementation stays succeeded, the review obligation stays pending and run-red until exact artifact verification, and retry-budget exhaustion remains an explicit escalation | Must | critical | kill, auth, route, runtime, truncation, artifact-lag, empty-frontier restart, and exhausted-driver-budget fixtures preserve one obligation, zero duplicate proposals or implementation launches, closed failure classification, and non-zero exit until verified | Test + Failure injection | FR-33..FR-39, EVT-13, EVT-14 |

### op: add DOD

Reason: the review terminal is safe to ship only when the full attended/unattended significance chain and resource-release ordering are regression-proven.

| ID | Done criterion | Source | Verify | Required evidence | Blocking? |
|---|---|---|---|---|---|
| DOD-21 | Every configured green proposal materializes one durable exact-revision review obligation; the implementation claim and width slot release before review; one invocation launches at most one uniquely identified attempt for that obligation and deduplicates concurrent/repeated starts. | FR-33, FR-35 | Test + Failure injection + Inspection | obligation persistence/recovery record, claim/width ordering trace, crash-point matrix, duplicate-invocation fixture, and distinct retry-attempt identities | yes |
| DOD-22 | Every supported adapter resolves and launches the qualified review-role model/effort without implementation controls; only an exact current artifact verifies; every failed attempt emits EVT-13 with the correct closed failure class and EVT-14 carries exact class, failure, and pending counts without changing EVT-10. | FR-34, FR-36, FR-37, EVT-10, EVT-13, EVT-14 | Test + Failure injection + Inspection | four-adapter route/launch matrix, non-pyxis/future-clock/auth fixtures, malformed/foreign/stale/paginated/lagged artifact matrix, attended+JSONL wire assertions, task-outcome non-regression, tally, and exit assertions | yes |
| DOD-23 | A killed or failed review remains durably pending; an empty-implementation re-invocation exact-reads then performs only bounded review work; the unattended driver consumes `review_pending`, retries review-only under a bound, never redispatches implementation, exits 0 only after verification, and stays visibly red when the bound exhausts. | FR-38, FR-39, NR-9, UB-19, EVT-14 | Test + Failure injection + Demo | kill/restart and empty-frontier fixtures, no-implementation-launch trace, exact-revision retry, driver retry-budget drill, pending-to-verified transition, and exhausted-budget escalation | yes |

### op: supersede DOD-20

Reason: the all-unwanted-behavior proof must include the new durable review-recovery row.

| ID | Done criterion | Source | Verify | Required evidence | Blocking? |
|---|---|---|---|---|---|
| DOD-24 | Every live unwanted-behavior row is handled. | UB-1..UB-9, UB-11..UB-19 | Test + Failure injection + Analysis + Inspection | failure-injection matrix with one result per live UB row, including canonical-evidence unavailability, durable review recovery, driver exhaustion, and zero omitted critical path | yes |

### op: supersede DOD-10

Reason: the all-Must proof must include the split review obligations and the combined idle frontier.

| ID | Done criterion | Source | Verify | Required evidence | Blocking? |
|---|---|---|---|---|---|
| DOD-25 | Every live Must functional requirement passes its fit criterion, including field-identical plan/v2 transport, no-rewrite plan/v1 translation, least-cost satisfying implementation routing, immutable evidence, fallback/direct activation, durable review obligations, exact-artifact completion, closed failure significance, review-only recovery, and combined-frontier zero work. | FR-1..FR-5, FR-7..FR-10, FR-13..FR-18, FR-20..FR-39 | Test + Demo + Inspection + Analysis + Failure injection | per-requirement evidence index including exact, promoted, unsupported, legacy, cutover, obligation/retry, artifact/failure-class, driver-exhaustion, and combined-empty cases | yes |

### op: supersede DOD-15

Reason: the all-non-regression proof must follow NR-7's replacement and include the e2e and review-only frontier boundaries.

| ID | Done criterion | Source | Verify | Required evidence | Blocking? |
|---|---|---|---|---|---|
| DOD-26 | No live non-regression row is broken. | NR-1..NR-6, NR-8..NR-9 | Test + Inspection | one result per live NR row, including v1 byte identity, selected-harness retention, e2e path integrity, combined-frontier zero work, and review-only zero implementation work | yes |

### op: supersede §3.2 Goals → signals → metrics

Reason: the configured review terminal needs a complete denominator and the metric plan must point only to the live replacement task/run events.

| Metric | Goal | Signal | Baseline | Target (at checkpoint) | Feeds from |
|---|---|---|---|---|---|
| M1 (primary) | G1 | a real pyxis feature's tasks reach green PRs via `/implement-now` | 0 features under demand-aware routing | 1 dogfood feature fully implemented locally, operator only reviewing/merging | EVT-10, EVT-14 |
| M2 | G2 | tasks counted done that are genuinely green | n/a | 100% of succeeded tasks carry a green proposal; zero hollow-as-succeeded | EVT-4, EVT-10, EVT-14 |
| M3 | G3 | worker core and runtime adapters retain portability | one coarse rung mapper | the worker passes unchanged behind fake/local dispatch; every supported harness passes implementation-role routing conformance | EVT-7, EVT-8, EVT-14 |
| M4 | G1 | requested semantic profiles whose plan/v2 fields are identical to frozen demand or whose plan/v1 origin is a typed FR-22 translation, followed by a satisfying or explicit unsupported result | optional rung can be absent and demand can be lost | 100% exact origin/transport/resolution; zero inferred, downgraded, native-in-plan, or cross-harness substitutions | EVT-7, EVT-8, EVT-9, EVT-14 |
| M5 | G1 | distinct native implementation profiles exercised by dogfood attempts that succeed with `verification=verified` | hardcoded/implicit route can converge on one pin | at least 3 before production activation; failed/blocked attempts never count | EVT-7, EVT-10 |
| M6 | G2 | routing attempts with exactly one origin, one resolution terminal, and—when resolved—one worker outcome | no complete decision chain | 100% in conformance and dogfood before activation; remains 100% over production attempts after activation; zero orphan/duplicate/cross-class terminals | EVT-7..EVT-10, EVT-14 |
| M7 *(004)* | G2 | configured green proposals with one durable exact-revision review obligation and append-only attempt terminals: verified only with an exact artifact, otherwise closed-class failed and still pending | configured review can be silently absent, process-exit-derived, or red for only one invocation | 100% of configured green proposals; zero absent obligations, duplicate same-invocation attempts, artifact-free verified terminals, unclassified failures, pending-with-exit-zero runs, implementation redispatches, or pending obligations laundered by a later empty frontier | EVT-13, EVT-14 |

### op: supersede §3.3 Guardrails and decision rule

Reason: release must fail when a configured review is absent or artifact-unverified, and the ship rule must include the new blocking proof row.

| Guardrail metric | Threshold | Action if breached |
|---|---|---|
| Parent-checkout mutation | any mutation of the parent clone / engine checkout outside a worktree | fail-closed; halt the offending worker and alarm |
| Double-claim with the external fleet | any task worked concurrently by both a local worker and a fleet node | fail-closed; block release until the claim mechanism is fixed |
| Self-merge | any merge performed by the executor | critical defect; executor holds no merge credential |
| Idle cost | any implementation routing/claim/model spend when its frontier is empty, any reviewer spend when no review obligation is pending, or any model spend when both frontiers are empty | investigate the combined-frontier gate and block release on an unauthorized launch |
| Semantic downgrade or harness switch | any native launch fails to satisfy one requested component, uses an unqualified mapping, or changes the selected harness | stop affected claims; block/rollback immediately; repair policy and rerun affected attempts |
| Frozen-plan mutation or post-freeze inference | any plan/v1 rewrite or plan/v2 assessment/profile reinterpretation | invalidate affected evidence; require no-rewrite translation or field-identical transport before retry |
| Routing evidence break | any attempt lacks exactly one origin/terminal or a resolved attempt lacks exactly one outcome | do not count it succeeded or toward release; preserve artifacts and repair the evidence chain |
| Configured review terminal break | any configured green proposal lacks one durable exact-revision obligation, verifies without an exact current artifact, fails without a closed class/escalation/tally/non-zero exit, duplicates an attempt within one invocation, or becomes clean while still pending | preserve the green EVT-10, block release or activation, and reconcile the same obligation/revision through the bounded review-only path |

**Ship rule (pre-registered):** M1–M7 meet their pre-release targets, every live blocking DoD row (DOD-5, DOD-8..DOD-9, DOD-11..DOD-14, DOD-17..DOD-19, DOD-21..DOD-26) is green, and every guardrail stays clean through complete pre-release proof → activate every supported production implementation and configured review path in one cutover; post-launch M4–M7 and every guardrail remain continuously evaluated; any breach → no activation or immediate strongest-profile fallback, affected-attempt invalidation, and exact-revision review-only reconciliation where applicable.

### op: supersede §4.3 Experience: flows and states

Reason: the user-visible run has two independently derived work frontiers, and a pending review must not masquerade as an idle or implementation-failed sweep.

**Flow F1 — an in-session run (serves J1, J3)**

| Step | User does | System responds | States covered | Design |
|---|---|---|---|---|
| 1 | invokes `/implement-now` on a frozen, projected plan with an explicitly selected/ambient harness | reports the ready implementation frontier and pending review-obligation frontier; only a combined-empty run exits with zero routing, claim, review, or model work | ideal · empty · review-only · error | superseding tech design |
| 2 | watches (attended) or supplies the journal (argus) | for each ready task, validates field-identical plan/v2 demand or emits typed plan/v1 translation, resolves through that harness's implementation mapping, and refuses invalid/unsupported tasks before claim | ideal · unsupported · integrity error · legacy error | superseding tech design |
| 3 | — | claims only resolved tasks, then shows claimed → implementing → proposal → green while independent valid tasks continue; each configured green proposal establishes one durable exact-revision review obligation and releases implementation resources | loading · partial · error | superseding tech design |
| 4 | — | exact-reads each pending obligation, otherwise performs at most one bounded qualified review attempt for it; verified artifacts clear obligations, typed failures leave them pending and run-red without changing EVT-10 | review-only · partial · error | superseding tech design + driver consumer amendment |
| 5 | reviews the final report | receives succeeded/failed/blocked/skipped implementation classes, exact/promoted/unsupported routing evidence, proposal links, review completed/failed/pending counts, and closed review failure classes; the driver consumes pending as review-only incomplete work | ideal · partial · review-pending · error | superseding tech design + driver consumer amendment |

- **Empty** — both frontiers empty: report state and exit with zero EVT-7/EVT-8, claims, reviewer launches, or model calls.
- **Review-only** — no implementation task is ready but one or more review obligations are pending: perform exact-read/review work only, with zero implementation route, claim, worker, or proposal activity.
- **Loading** — a resolved worker is implementing or awaiting CI; live status names selected harness and native profile identity but no prompt/code payload.
- **Partial** — green, blocked, claimed-elsewhere, failed, integrity-invalid, legacy-incompatible, unsupported, review-verified, and review-pending states are classed separately; independent valid work continues.
- **Error** — implementation policy-invalid/unsupported stops before claim; execution failure follows its bounded escalation path; review failure preserves implementation success, names `artifact_unverified` or `infrastructure`, and remains pending/run-red through bounded review-only recovery.
- **Accessibility / localization** — unchanged terminal/agent-driven, English-only surface.

### op: supersede §5.4 Data

Reason: durable review obligations and append-only review attempts are separate data from the immutable implementation outcome and need explicit ownership, retention, and integrity rules.

| Entity / field | Meaning | Source of truth | Retention | Integrity rules | Privacy class |
|---|---|---|---|---|---|
| plan/v2 execution demand | frozen assessment, citations, semantic profile, derivation version, raise evidence | frozen plan; projected copy is transport | with plan | field-identical through executor boundary; immutable; no native identifiers | internal |
| plan/v1 legacy origin | original rung presence/value plus compatibility version and translated profile | immutable source plan + typed EVT-9 | forever with routing evidence | source never rewritten; explicit or `legacy_unassessed`; unknown fails | internal |
| e2e scenario basis | the scenario flow, route, mock strategy, setup/teardown, harness selection, and coverage rows a recognized e2e task must satisfy | the feature's approved `e2e-design/v1` document, at the revision the plan's recorded reference names | with the feature's e2e artifacts | read-only to the executor and never edited by it (NG-5); resolved from the recorded reference, never inferred; exactly one scenario or the harness slot per task; a non-approved, malformed, or ambiguous basis refuses under UB-14 | internal |
| native routing policy | qualified harness×role mappings plus authorized repo overrides | released defaults + validated repo policy | current + repository history | closed/version-compatible; qualification evidence; no weakening/raw arguments | internal |
| routing attempt | selected harness/role, requested profile, policy version, exact/promoted/unsupported result, native identity if resolved | immutable EVT-7 or EVT-8 | forever in git evidence | exactly one terminal per attempt; connected to one origin | internal |
| e2e origin decision | the resolved, refused, or not-e2e outcome of a ready task's origin check, with its basis reference, slot, and route | immutable EVT-12 | forever in git evidence | append-only; exactly one per ready task per run; a refusal names its reason and precedes any claim | internal |
| worker outcome | verified/failed/blocked terminal and usage for a resolved implementation attempt | immutable EVT-10 + proposal evidence | forever in git evidence | exactly one per resolved attempt; absent for unsupported pre-claim attempts; never rewritten by review state | internal |
| review obligation | the requirement to obtain one exact review artifact for a configured green task/proposal/revision under the recorded backend and review policy | durable medium-neutral obligation record joined to authoritative task/proposal state | until verified, then forever with proposal/run evidence | stable obligation identity; state is `pending` or `verified`; only exact current artifact verification clears pending; recoverable/re-derivable after process death | internal |
| review attempt terminal | one append-only attempt or setup result against a review obligation, including closed failure class and goal-object evidence | immutable EVT-13 + proposal artifact when verified | forever with review evidence | fresh attempt identity; stable obligation join; at most one launch per obligation per invocation; failure never mutates EVT-10 or clears pending | internal |
| evidence invalidation | affected implementation attempts/artifacts and `rerun_required` or `operator_remediation` disposition | immutable EVT-11 + parent routing policy defect disposition | forever in git evidence | append-only; exact policy/profile and artifact identities; never rewrites the invalid evidence or accepted artifact | internal |
| task claim | at-most-one-worker lease | shared claim mechanism | until release/reap | acquired only after routing resolves; paired release; never held by review | internal |
| worker workspace | isolated worktree + branch | local git | until reaped | fetched-mainline base; parent never mutated | internal |
| proposal | task change + verification/review evidence | code host | per host | one per task; green before succeeded; review artifacts bind exact proposal revision | internal |

- **Migration** — no artifact is rewritten. Existing green proposals under configured review derive one obligation from their current proposal revision on first reconciliation; new proposals persist it at green. Plan/v2 demand remains field-identical; frozen plan/v1 reads through FR-22 and emits EVT-9; an approved e2e design doc is read at the revision the plan records and never edited.
- **Parallel-run / cutover** — no production shadow/canary. Pre-release conformance and dogfood exercise the full implementation and configured-review obligation domain before one direct cutover.
- **Rollback** — FR-26 changes native resolution within the selected harness only. Invalid/unsupported demand remains unclaimed; an unresolvable e2e basis leaves its task unclaimed; pending review obligations and append-only attempt evidence survive rollback and remain exact-revision work.
- **Decommission** — remove the hardcoded/implicit rung/model/effort path and silent/one-invocation-only review path only after direct cutover, fallback, and durable review-recovery drills pass.
- **Privacy check** — no new personal/sensitive data; no DPIA trigger. Review obligations contain repository/task/proposal routing metadata already present in executor evidence, never prompt/code payload.
- **Egress** — the already-selected harness remains the only model boundary. Routing and obligation records add metadata, no new provider or payload egress; prompts/code remain excluded from routing and review-terminal evidence.

### op: supersede §5.5 Interfaces and integrations

Reason: review persistence and the unattended driver's pending-work consumer are cross-boundary contracts, not implications of process exit.

| Interface | Direction | Purpose | Contract | Failure modes | Owner |
|---|---|---|---|---|---|
| frozen plan/v2 + projected task | in | supply charter and complete semantic demand | versioned closed schema; projected fields identical to frozen record | missing/changed/native field → UB-9 before claim | plan-new + tracker adapters |
| frozen plan/v1 | in | legacy execution without rewrite | explicit rung through versioned table or absent-rung `legacy_unassessed` | unknown rung/version → UB-11 | compatibility owner + executor |
| feature e2e design doc (`e2e-design/v1`) | in | supply the reviewed scenario flow, route, mock strategy, setup/teardown, harness selection, and coverage rows a recognized e2e task must satisfy | an `approved` `e2e-design/v1` document at the feature's e2e home, well-formed against its class checklist; read as data, never as instruction | missing, not-approved, malformed, unresolvable, or ambiguous scenario join → UB-14 before claim | e2e-design + e2e-review |
| projected e2e plan task | in | deliver e2e work onto the same ready frontier as feature work | plan-new's existing plan class, freeze, and projection; ordinary task and charter schema with no e2e-specific type or required field | an e2e-specific type or required field would breach NR-8; an absent basis join → UB-14 | e2e-plan + plan-new |
| harness selection | in | choose the one credential/egress/runtime boundary for implementation and configured review | explicit config or ambient harness; no installed-CLI inference; review may carry its explicit configured backend | absent/unauthenticated → FR-16/FR-34; unsupported remains same selected backend | operator/invoker |
| executor → selected harness adapter (implementation) | out | resolve implementation semantic minimum | qualified implementation-role mapping; exact/promoted/unsupported | invalid mapping or no satisfying profile → FR-24/UB-18 | runtime adapters |
| selected harness adapter → canonical evidence store | out | durably append and exact-read the terminal route before claim or invocation | configured git evidence ref; immutable event; exact-read proof | unavailable, unauthorized, conflicting, or unverifiable evidence → no claim/invocation under NFR-9 | evidence adapter |
| selected harness adapter → AgentRuntime launch | out | invoke the resolved implementation native profile | native model/effort/context arguments derived only from adapter policy | argument injection, launch mismatch → UB-18/ABUSE-5 | runtime adapter |
| executor ↔ review obligation store | both | persist/recover the exact task/proposal/revision/backend/policy obligation independently of process lifetime | closed versioned medium-neutral record; stable obligation identity; `pending`/`verified`; exact-read on each reconciliation | unavailable/unverifiable → infrastructure failure, resources released, run red, obligation re-derived from authoritative proposal state | executor + obligation-store adapter |
| executor → selected harness adapter (review) | out | resolve and launch one bounded sibling review attempt | qualified review-role catalog/policy; concrete model/effort; no implementation-only controls; selected configured backend unchanged | auth, invalid/unauthorized mapping, launch/runtime/truncation → EVT-13 infrastructure + pending obligation | runtime adapters |
| review persistence adapter ↔ code host | both | locate and exact-read the proposal's self-contained review artifact | schema-valid artifact bound to exact proposal/revision; pagination and replica-lag bounded; malformed/foreign candidates skipped | exact artifact absent after bound → EVT-13 artifact_unverified; transport/auth → infrastructure | impl-review persistence + code host |
| executor → driver/report sinks | out | expose implementation classes separately from configured-review completion and pending work | EVT-10 remains implementation truth; EVT-13 carries review attempts; EVT-14 carries completed/failed/pending and class counts; `review_pending>0` means review-only incomplete | absent/unknown fields fail closed; driver retry exhaustion leaves visible escalation | executor + self-drive/feature-drive consumer |
| runtime → implementation evidence stream | out | preserve origin, route, and worker outcome | EVT-7..EVT-10 and EVT-12, immutable and payload-free | orphan/duplicate/missing edge → UB-12 | executor + evidence owner |
| code host | both | proposals + CI + proposal-bound review artifacts | existing proposal/check/comment surface with exact revision binding | lag bounded; malformed or stale is not verified | this feature + impl-review persistence |
| merge-lane | out | landing downstream | opens proposals; never merges | no landing behavior owned here | merge-lane |

### op: supersede §5.7 Instrumentation — the tracking plan

Reason: the live table must remove only superseded EVT-6, retain implementation EVT-10, restore every other live event, and give retryable review attempts their own event.

| ID | Event name | Fires when | Properties (typed) | Source | Feeds |
|---|---|---|---|---|---|
| EVT-1 | `run_started` | a run begins | `plan_ref:string`, `frontier_size:int`, `driver:enum{operator,argus}` | executor | — |
| EVT-7 *(001 ← EVT-2)* | `routing_resolved` | an implementation route resolves before claim | `attempt_id:string`, `demand_id:string`, `task_id:string`, `harness:string`, `role:implementation`, `requested_profile:closed object`, `native_profile_id:string`, `resolution:enum{exact,promoted}`, `promoted_components:enum[]`, `policy_version:string` | selected harness adapter | M3, M4, M5, M6 |
| EVT-8 *(001)* | `routing_resolution_failed` | routing stops before claim/invocation | `attempt_id:string`, `demand_id:string`, `task_id:string`, `harness:string`, `role:implementation`, `requested_profile:closed object`, `policy_version:string`, `unmet_components:enum[]`, `reason:enum{policy_invalid,unsupported,integrity_error,legacy_incompatible}` | executor + harness adapter | M4, M6 + health alert |
| EVT-9 *(001)* | `legacy_execution_demand_translated` | a frozen plan/v1 task translates without rewrite | `demand_id:string`, `task_id:string`, `plan_version:string`, `rung_present:boolean`, `original_rung:string?`, `compatibility_version:string`, `translation:enum{explicit_rung,legacy_unassessed}`, `translated_profile:closed object`, `policy_version:string` | legacy translator | M4, M6 |
| EVT-11 *(001)* | `implementation_routing_evidence_invalidated` | UB-13 identifies implementation evidence produced under a defective policy/profile | `policy_version:string`, `native_profile_id:string`, `attempt_ids:string[]`, `task_ids:string[]`, `proposal_refs:string[]`, `disposition:enum{rerun_required,operator_remediation}`, `evidence_ref:string` | executor + evidence owner | UB-13 audit + health alert |
| EVT-3 | `ci_fix_attempt` | a self-heal iteration runs | `task_id:string`, `attempt:int`, `bound:int` | executor | — |
| EVT-4 | `task_verified` | a proposal is verified green or hollow | `task_id:string`, `proposal_ref:string`, `green:boolean`, `hollow:boolean` | executor | M2 |
| EVT-10 *(001 ← EVT-5)* | `routed_task_outcome` | a resolved implementation attempt reaches a terminal worker class | `attempt_id:string`, `demand_id:string`, `task_id:string`, `proposal_ref:string?`, `class:enum{succeeded,failed,blocked}`, `verification:enum{verified,failed,not_reached}`, `usage:closed object`, `turns:int`, `evidence_ref:string` | executor | M1, M2, M5, M6 |
| EVT-12 *(002)* | `e2e_task_basis_resolved` | a ready task's e2e origin is established or refused, before claim | `task_id:string`, `feature_id:string`, `plan_ref:string`, `basis_ref:string?`, `slot:enum{scenario,harness}?`, `scenario_id:string?`, `route:enum{run_for_real,mocked,manual_human}?`, `resolution:enum{resolved,refused,not_e2e}`, `reason:enum{missing_doc,not_approved,malformed,unresolvable,ambiguous}?` | executor | DOD-19 audit + M2 |
| EVT-13 *(004)* | `impl_review_terminal` | configured-review setup fails or one review-obligation attempt reaches a verified or failed terminal | `review_attempt_id:string`, `review_obligation_id:string?`, `task_id:string?`, `proposal_ref:string?`, `revision:string?`, `backend:string`, `native_profile:string?`, `catalog:string?`, `policy:string?`, `model:string?`, `effort:string?`, `phase:enum{setup,obligation,routing,launch,await,artifact}`, `status:enum{verified,failed}`, `obligation_status:enum{verified,pending,not_created}`, `failure_class:enum{artifact_unverified,infrastructure}?`, `artifact_ref:string?`, `reason:string?`, `escalated:boolean` | executor | M7, FR-33..FR-38, UB-19 |
| EVT-14 *(004 ← EVT-6)* | `run_finished` | a run ends after implementation work and configured-review reconciliation settle for that invocation | `succeeded:int`, `failed:int`, `blocked:int`, `tokens:int`, `routing_exact:int`, `routing_promoted:int`, `routing_unsupported:int`, `routing_invalid:int`, `review_completed:int`, `review_failed:int`, `review_pending:int`, `review_artifact_unverified:int`, `review_infrastructure_failed:int` | executor | M1, M2, M4, M6, M7, FR-37, FR-38 |

### op: supersede §6.1 Phasing and milestones

Reason: the pre-release phase must exercise review obligation persistence, attempt routing, artifact verification, failure classification, and review-only driver recovery before activation.

| Phase | Purpose | Scope | Exit criterion |
|---|---|---|---|
| Demand boundary | consume without re-judging | plan/v2 integrity/transport, plan/v1 translation, origin identities, negative fixtures | FR-21/FR-22, NR-5, UB-9/UB-11 green; frozen input byte-identity proven |
| Native resolution | resolve through every selected harness | implementation-role mappings, review-role mappings, qualification, overrides, least-cost resolver, promotions, unsupported, append-first canonical evidence | FR-23..FR-26/FR-34, NFR-7/NFR-9, EVAL-2, DOD-11/DOD-14/DOD-22 green |
| Worker integration | preserve execution and review truth separately | connect resolved implementation settings to fresh sessions, claims, drive-to-green and EVT-10; establish durable post-green review obligations, release resources, and append EVT-13 attempts without rewriting task outcomes | existing worker suite plus FR-25/FR-33..FR-37 and EVT-7..EVT-13 green; canonical route evidence precedes claim; one obligation persists before review; implementation resources release before any review launch |
| Driver integration | make review-red durable across invocations | combined frontier derivation, exact-read-first review reconciliation, EVT-14 pending/class counts, bounded review-only retry, retry exhaustion | FR-38/FR-39, NR-9, UB-19, DOD-23 green; empty-implementation restart performs zero implementation work and cannot launder pending review to exit 0 |
| Pre-release dogfood | falsify the full seam | one real feature, at least three successfully verified native profiles, exact/promotion/unsupported/legacy audit, fallback, post-defect, verified-review, both review-failure classes, interruption/retry/exhaustion, and cutover drills | M1–M7 pre-release targets, every live blocking DoD row (DOD-5, DOD-8..DOD-9, DOD-11..DOD-14, DOD-17..DOD-19, DOD-21..DOD-26), and every §3.3 guardrail green |
| Production | replace the hardcoded/implicit and ephemeral-review paths | every supported `/implement-now` implementation and configured review route plus its driver consumer | one direct cutover succeeds; implementation and review evidence remain separate/connected; old pin, silent-review, and next-sweep-laundering paths are unreachable |

### op: supersede §6.2 Dependencies

Reason: implement-now owns durable review obligation/reconciliation and attempt observation, impl-review retains artifact production and internal assurance, and the watcher/feature driver must explicitly consume pending review work.

| Dependency | Type | Needed by | Status | Owner |
|---|---|---|---|---|
| plan-new plan/v2 demand and checkpoint contract (`prd:plan-new/FR-28..FR-30`) | approval / data contract | Demand boundary | available through merged amendment 004 | plan-new owner |
| planner-driven routing semantics, compatibility, resolver, evidence, and rollout (`prd:planner-driven-model-routing/FR-7..FR-12`, `FR-15`, `FR-17`, `FR-20..FR-24`, `FR-27`) | policy / contract | all phases | parent through amendment 002 is approved; this implement-now amendment must land first with the zero-network compatibility path, then `planner-driven-model-routing/003-append-first-routing-evidence` activates mandatory canonical admission; ancestry is proven against main before the parent PR is merged | routing policy owner |
| superseding implement-now technical design | docs | Demand boundary + review integration | required after this amendment; must replace `tiermap(rung)` and hardcoded/implicit launch selection, define the medium-neutral durable review-obligation store and combined frontier, keep EVT-10 independent from EVT-13, and preserve Worker/Dispatcher ownership | @mdarabi |
| e2e-plan records the feature's approved e2e design doc as the plan's governing-document header reference | data contract | e2e consumption | required before FR-28 can resolve any origin; uses the plan class's existing required header field and needs no plan-new or plangate change | e2e-plan owner |
| approved `e2e-design/v1` documents at their per-feature home (`prd:e2e-verification/FR-1`, `FR-4`, `FR-5`) | data contract | e2e consumption | available through the merged e2e authoring line; a non-approved or malformed document refuses under UB-14 | e2e-design + e2e-review owners |
| impl-review amendment 003 and superseding design | approval / docs | review-role resolution and exact-revision artifact production | available as the sibling contract for band projection, pass orchestration, artifact schema, and proposal-medium persistence; implement-now owns obligation identity/state, configured sibling launch, resource ordering, exact-read reconciliation, EVT-13, and run consequence under FR-33..FR-38 | impl-review + implement-now owners |
| self-drive-wake consumer amendment + superseding watch design (or feature-drive absorption with the same contract) | approval / docs | unattended review-only recovery | required before FR-38 production activation; consumes EVT-14 `review_pending`, schedules bounded review-only re-invocations, preserves retry exhaustion as red/escalated, and never redispatches the succeeded implementation | self-drive-wake / feature-drive owner |
| shared claim mechanism + automated-implementation tag | system | Worker integration | confirmed | plan-new / fleet convention |
| qualified current native model roster and cost metadata per harness implementation and review roles | vendor / policy | Native resolution + release | required; mappings without reviewed qualification fail | @mdarabi + runtime adapter owners |
| four agent CLIs installed/authenticated | vendor | Worker + review integration | operator-provisioned and preflighted before a configured run spends avoidable task work | operator |
| merge-lane | system | post-run | as-is; review floor changes land through impl-review sibling | merge-lane |

### op: supersede §6.3 Risks, assumptions, and rabbit holes

Reason: a durable review obligation creates a new cross-invocation loss/duplication risk and changes the idle-bound mitigation.

| # | Risk / assumption / rabbit hole | Likelihood | Impact | Mitigation or test | Owner |
|---|---|---|---|---|---|
| R1 | same-session charter bleed if any state leaks across tasks | M | H | fresh session per task (FR-5); no cross-task reuse; UB-3 drill | design |
| R2 | unbounded loop / token runaway from a drifted predicate | M | H | single-source predicates (FR-18); combined-idle gate (FR-39/NR-9); per-task and per-review-invocation bounds (FR-9/FR-13/FR-35/FR-38) | design |
| R3 | "green" is red or absent | M | H | artifact verification (FR-10); local gate mirrors CI (FR-8); EVAL-3 | design |
| R4 | selected harness adapter mapping goes stale as native catalogs or costs change | H | H | closed versioned policy, current qualification evidence, exhaustive EVAL-2, unsupported refusal, strongest-profile fallback | runtime adapter owners |
| R5 | one tracker carrier drops or mutates plan/v2 demand | M | H | consumer-side field-identity conformance for every supported carrier (FR-21/UB-9) | plan projection + executor owners |
| R6 | legacy translation becomes mutable inference or rewrites source | L | H | one versioned table, conservative absent-rung classification, unknown fail, byte-identity manifest (FR-22/NR-5/DOD-14) | compatibility owner |
| R7 | direct activation exposes a broad mapping defect | M | H | all-path pre-release proof, no-claim unsupported behavior, strongest-profile drill, and UB-13/DOD-18 exact-artifact invalidation and disposition | @mdarabi |
| R8 | native routing accidentally becomes harness selection or review orchestration | M | H | NR-6 and scope boundary; selected harness is input; impl-review sibling owns proposal-band/pass logic | @mdarabi + design reviewer |
| R9 | headless Claude cannot fully cover an autonomous run | M | M | Q-1 spike before adapter commitment; no weakening or harness fallback if unsupported | design |
| R10 *(004)* | a crash, store failure, concurrent invocation, or driver restart loses or duplicates a pending review, or launders it after implementation becomes non-ready | M | H | stable proposal-revision obligation identity; durable persist/re-derive; exact-read-first; one launch per obligation per invocation; EVT-13 append-only attempts; EVT-14 pending denominator; UB-19 kill/restart/concurrency/exhaustion drills | executor obligation owner + self-drive/feature-drive consumer owner |

### op: supersede §6.4 Release criteria

Reason: the release criteria must make review obligation durability, attempt semantics, failure classes, and driver consumption independently falsifiable.

| Lens | Criterion | Evaluated by |
|---|---|---|
| Functionality | DOD-25 and every live Must fit criterion green; every non-empty task follows plan/v2 or typed plan/v1 demand; every configured green proposal persists one obligation, preserves EVT-10, and reaches verified or typed-pending review evidence | CI + compatibility suite + DOD-21..DOD-23 reports |
| Routing conformance | every supported harness×derivable-profile implementation cell and required review-role cell produces its qualified least-cost exact/promoted result or explicit unsupported terminal; resolution uses zero inference/network I/O, the pre-parent compatibility path remains zero-network, and after parent activation canonical implementation evidence is exact-read before claim while review completion exact-reads its proposal artifact | EVAL-2 + DOD-11 operator rubric + NFR-9 boundary traces + DOD-22 |
| Reliability | DOD-24's UB-1..UB-9/UB-11..UB-19 drills; after parent activation no claim precedes exact-read route evidence; no claim leak, duplicate proposal/attempt, downgrade, harness switch, orphan evidence, accepted affected open artifact, silent configured review, artifact-free verification, pending-review laundering, or implementation redispatch during review repair | failure injection + implementation/review/invalidation joins + driver report |
| Security | existing abuse drills plus ABUSE-5; no merge credential, parent mutation, unqualified mapping, raw argument, native plan identifier, or unauthorized pre-launch network operation | tests + inspection |
| Compatibility | DOD-14 plan/v1 translation and byte identity; plan/v2 carrier identity across every tracker adapter | corpus and round-trip conformance |
| Dogfood | one real feature exercises at least three native profiles that each succeed with verified EVT-10 outcomes; every configured review is exact-revision artifact-verified or closed-class pending/run-red, and one killed review recovers through the review-only driver path | DOD-12 and DOD-21..DOD-23 reports |
| Rollout | direct cutover and strongest-profile fallback satisfy every semantic minimum without changing harness; old pin, silent-review, duplicate-attempt, and pending-laundering paths are unreachable; post-cutover qualification defects trigger exact-artifact invalidation/disposition | DOD-13/DOD-18/DOD-21..DOD-23 drills + inspection |

### op: supersede §6.5 Rollout and reversibility

Reason: direct cutover and rollback must preserve durable review obligations, separate attempt evidence, and the driver consumer as well as implementation routing.

- **Flag** — one production implementation-routing policy switch, owned by @mdarabi, changes native implementation and configured review resolution only after the full parent pre-release gate—including DOD-21..DOD-26 and the driver consumer dependency—is green.
- **Kill switch** — FR-26's strongest qualified profile within each already-selected harness and role. It may overprovide but never downgrade, change harness, bypass evidence, or claim an unsatisfiable task; pending review remains review-only run-red until an exact-revision artifact verifies.
- **Experiment** — none; no adaptive optimization, shadow production decisions, or canary cohort.
- **Cutover** — validate closed defaults/overrides, qualification, configured-review auth, obligation persistence, and driver consumption; pass every implementation/review conformance cell, v1 compatibility case, three-profile dogfood, evidence audit, both review-failure classes, kill/restart/retry/exhaustion drills, direct-cutover rehearsal, and fallback drill; then activate every supported production `/implement-now` route in one switch. Remove the hardcoded/implicit rung/model/effort, silent-review, and invocation-local-only review paths only after successful activation.
- **Rollback** — engage the strongest-profile fallback and stop affected new claims; preserve frozen demand, immutable prior evidence, durable review obligations, and append-only attempt terminals. Unsatisfied demand remains unsupported. Under UB-13, append EVT-11 for exact affected in-flight/unaccepted attempts as `rerun_required`, preserve unaffected work, and rerun replacement evidence on the exact affected task/proposal revision; already accepted work receives an `operator_remediation` brief and never automatic destructive rollback. A failed configured review preserves implementation success but remains pending/escalated/run-red and is reconciled review-only until exact-revision verification.

| Stage | Audience | Minimum evidence | Advance when | Roll back when |
|---|---|---|---|---|
| Pre-release only | conformance fixtures and repository dogfood; no production routing | every live blocking DoD row (DOD-5, DOD-8..DOD-9, DOD-11..DOD-14, DOD-17..DOD-19, DOD-21..DOD-26) plus §6.4 report, parent full-seam gate, and driver-consumer proof | every M1–M7 pre-release target and guardrail is green | any failed criterion blocks activation |
| 100% production | every supported `/implement-now` implementation/configured-review path and its unattended consumer | effective policy/obligation snapshots + successful direct-cutover/fallback/post-defect/review-terminal/kill-retry-exhaustion rehearsals | remains active while post-launch M4–M7 and every guardrail stay complete | any downgrade, harness switch, mapping defect, unsupported claim, evidence break, silent review, artifact-free verification, duplicate attempt, or pending laundering engages fallback, UB-13 invalidation where applicable, and exact-revision review-only reconciliation |

### op: supersede §8 Contract readiness — the executability gate

Reason: readiness must include the split review requirements, separate live event streams, durable recovery/consumer flow, replacement all-row proofs, and persistence risk.

| Check | Pass? | Notes |
|---|---|---|
| Problem basis accepted — §2 problem gate signed (T3) | ☐ | base gate remains pending the introducing review; amendments do not rewrite its historical status |
| Zero solution-blocking TBDs or open questions (§7.2) | ☑ | Q-1..Q-5 plus obligation-store placement and retry-bound values are design/config mechanisms constrained by FR-33..FR-39; unsupported cannot weaken/switch harness, and neither mechanism may erase pending work or redispatch implementation |
| Every live Must FR has a fit criterion and verification method (§5.1) | ☑ | FR-1..FR-5, FR-7..FR-10, FR-13..FR-18, FR-20..FR-39 |
| Every requirement row carries risk; high/critical rows have a safety plan | ☑ | pre-claim validation, no-rewrite translation, exhaustive conformance, selected-harness refusal, append-first immutable evidence, direct-cutover/fallback drill, e2e basis refusal, mandate/route consequence rows, hermeticity before green, durable exact-revision review obligations, closed failure classes, and bounded review-only recovery before run success |
| Non-goals drawn; non-regression rows named | ☑ | NG-1..NG-5; NR-1..NR-6, NR-8..NR-9; routing does not absorb harness selection or impl-review's band/pass orchestration, execution does not absorb e2e authoring, and review-only recovery never becomes implementation redispatch |
| Every primary job covered | ☑ | J1→FR-1/7/15/21..29/33..39; J2→FR-17/NR-4; J3→FR-7/8/9/25/30/31/32/37/38 |
| Every key flow has functional, failure, and non-regression coverage | ☑ | F1→FR-1..FR-39, UB-1..UB-9, UB-11..UB-19, NR-1..NR-6, NR-8..NR-9; unsupported, legacy, evidence-unavailable, post-cutover defect, e2e-basis, non-hermetic, obligation-store, both review-failure classes, interruption, retry, exhaustion, and combined-empty states are explicit |
| Every primary metric has instrumentation or plan | ☑ | M1–M6→EVT-4, EVT-7..EVT-10, EVT-14; M7→EVT-13/EVT-14 with exact-artifact verification, closed failed-attempt classes, and durable pending denominator; EVT-11/EVT-12 carry invalidation/e2e audit |
| Every blocking DoD row traces to source/rubric | ☑ | DOD-5, DOD-8..DOD-9, DOD-11..DOD-14, DOD-17..DOD-19, DOD-21..DOD-26; DOD-11 names @mdarabi and complete-cell rubric; DOD-21..DOD-23 separate obligation/order, route/artifact/event, and driver-recovery proofs; DOD-24..DOD-26 close every live UB/Must/NR row |
| Risks and assumptions have owners | ☑ | R1..R10 owned, including mapping staleness, transport, legacy, cutover, ownership creep, and lost/duplicated review obligations; the e2e re-implementation risk is bounded by FR-30/FR-31 and UB-16, flaky-check risk `incident:I5` is paid for by FR-32/UB-17, and review producer/obligation/consumer ownership is explicit in §6.2 |
| Human-judgment criteria name owner and rubric | ☑ | DOD-11 names @mdarabi and its cell qualification rubric; UB-15 routes manual-human scenarios to the owner the approved design doc records; other rows are mechanical/demo/analysis |
| DoD complete; n/a explained | ☑ | routing domain, legacy compatibility, successfully verified dogfood diversity, all consequence rows, direct cutover, fallback, post-cutover defect disposition, append-first evidence admission, the real-e2e-plan run, review obligation/order, route/artifact/failure evidence, review-only recovery, and complete live-row proofs are separately blocking |

### op: supersede §0 Read to rule — the human reviewer's section (~6 min)

Reason: the contract adds a durable post-green review obligation, separate append-only attempt evidence, and review-only driver recovery that the human reviewer must judge separately from implementation success.

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
| 15 | A configured impl-review is **advisory to EVT-10 but a durable obligation of the drive**: it has its own EVT-13 attempts, releases implementation resources before launch, verifies only by exact artifact, distinguishes artifact absence from infrastructure, and remains review-only run-red across invocations until verified | Overload the task outcome; treat clean reviewer exit as success; retry the implementation; or let the next empty task frontier erase the failure | Task truth stays immutable while unattended review absence remains recoverable and impossible to launder → FR-33..FR-39, UB-19, EVT-10, EVT-13, EVT-14, DOD-21..DOD-23 |

**Sanity-check the proof** — model routing is not a scalar complexity guess. The planner freezes six assessment axes and four independent semantic minima. `implement-now` proves field-identical transport, complete required implementation-role and review-role routing cells, at least three distinct native profiles in dogfood, no-rewrite v1 translation, and exact/promoted/unsupported evidence. The routing resolver itself spends zero inference and zero network I/O; only canonical implementation evidence append/exact-read may occur between resolution and invocation (NFR-9). For the e2e class the proof is a real e2e plan run end to end, with one deliberately-broken-subject falsification and one repeat-run verdict-stability record per automatable scenario (DOD-19). For configured post-green review, M7 and DOD-21..DOD-23 join one durable obligation, separate uniquely identified attempts, the selected review route, exact proposal revision/artifact, closed failure class, run counts, pending state, escalation, bounded driver retry, and exit code without changing EVT-10 or relaunching implementation.

**The author's open questions** — whether Claude Code's headless mode fully covers an autonomous multi-tool run remains open (§7.2 Q-1). Harness *selection* mechanics remain design-owned (§7.2 Q-4); demand resolution never answers that question or scans for another harness. *How* the executor joins a recorded e2e reference to the individual scenario is likewise design-owned. The durable obligation store's concrete adapter and the driver's numeric retry budget are also design/config choices; FR-33/FR-38 fix their identity, persistence, consumer behavior, and fail-closed outcomes, so neither choice may erase pending work or redispatch implementation.

**Skip the rest** — structure/ID/fold correctness → docgate; substance and coherence → `/prd-review`; implementation mechanisms → the superseding technical design; review assurance/process routing → the impl-review sibling contract; merge safety → merge-lane; plan judgment and projection → plan-new; what an e2e scenario, route, or mock strategy should be → e2e-design and e2e-review, already settled by the doc's approval.
