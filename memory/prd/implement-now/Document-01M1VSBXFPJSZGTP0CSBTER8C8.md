# Amendment 005 — authenticated review attribution

| Field | Value |
|---|---|
| **Amends** | implement-now *(chain through 004)* |
| **Status** | Approved |
| **Owner** | @mdarabi |
| **Date** | 2026-08-06 |
| **Learned-from** | review-thread:pyxis#1034 · review-thread:pyxis#1041 · review-thread:pyxis#1054 |
| **Blast radius** | tier: contract-changing; touches: FR-33, FR-34, FR-36..FR-39, FR-40..FR-47, NR-9→NR-10, UB-19..UB-21, ABUSE-6, EVT-13..EVT-16, DOD-17, DOD-21..DOD-26, DOD-27..DOD-33, M7, §0, §1, §3.2..§3.4, §4.1..§4.4, §5.2..§5.5, §5.7..§5.8, §6.1..§6.5, §7.2, §8; reason: a review artifact becomes acceptable only with attempt-scoped producer attribution unavailable to the implementing worker, the exact green revision is captured without sweep drift, configured review factory/auth admission is separated from per-obligation routing, the impl-review-owned persistence broker is fixed as the proposal-artifact publisher, its performer authority and receipt-returning exactly-once capability are an explicit sibling-contract prerequisite rather than a design assumption, canonical review evidence is outside every worker write surface, reviewers gain isolated bounded execution, the driver can submit one closed selected recovery frontier, and observability distinguishes untrusted artifacts from absence and infrastructure failure |
| **Tracking** | issue:1029 · PR #1034 |
| **Template version** | amendment-template/v2 |

## Operations

### op: supersede FR-33

Reason: the exact reviewed revision and review obligation must be captured at the task's green boundary, not re-derived after the sweep, while release-before-launch remains mechanically evidenced.

| ID | Requirement (EARS) | Rationale | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|
| FR-40 | When a configured task proposal is verified genuinely green, the executor shall atomically capture the exact verified head revision into one durable review obligation before emitting the implementation terminal, then release that task's implementation claim and dispatcher width slot and admit the obligation to the review frontier without waiting for the implementation sweep to finish; no later proposal-head lookup may change the obligation's revision, and inability to persist or recover it shall still release both resources, preserve the succeeded implementation truth, and fail the run as review infrastructure. | per-task capture removes the sweep-long drift window and delayed outcomes without allowing review latency to consume implementation capacity | Must | critical | an ordering trace proves green verification → exact-revision obligation persistence → succeeded EVT-10 → claim/width release → review launch for each task; concurrent head movement after green never changes the reviewed revision; crash points at every edge recover one obligation, release both resources, and never redispatch implementation | Test + Failure injection | G2, J1, FR-10, FR-15, FR-35 |

### op: supersede FR-36

Reason: proposal/revision binding alone lets the implementing worker forge the sole evidence that a review occurred.

| ID | Requirement (EARS) | Rationale | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|
| FR-41 | When a configured review attempt is admitted, the executor shall mint fresh attempt-scoped attribution material only after the implementation worker has terminated and its resources are released and bind its non-secret proof to the obligation, attempt identity, proposal, and revision. The launched reviewer process shall return exactly one schema-valid completed report to the **review artifact broker**: the impl-review-owned host-side persistence adapter that performs `prd:impl-review/FR-6`'s medium-appropriate proposal write and returns the exact native receipt; implement-now shall invoke and verify through that boundary but shall neither publish the artifact itself nor redefine impl-review's artifact contract. The review path shall remain inactive and implementation shall not proceed unless the live impl-review contract and design explicitly authorize that host-side adapter to perform FR-6 on the reviewer program's behalf and expose exactly-one publication returning that receipt; absent or ambiguous performer authority, receipt capability, or exactly-once semantics requires a sibling impl-review amendment and design first. The executor shall mark the obligation verified only after exact-reading that receipted artifact and authenticating its launched-attempt proof against canonical obligation/attempt/receipt state whose producer-signing authority is unavailable to every implementation and reviewer child process and workspace; generic repository/ref write credentials shall never authorize canonical state. The code-host author is transport metadata, need not differ from the implementation identity, and shall not participate in verification; raw attribution material shall never enter the implementation charter, workspace, event stream, or repository; and process exit, output text, a pre-existing or worker-authored artifact without that fresh proof, replayed proof, malformed artifact, a different proposal/revision, or worker-substituted evidence state shall never establish completion. | the artifact is the goal object only if the impl-review persistence boundary publishes it and its protected receipt proves which fresh reviewer invocation produced it, independent of the account used to transport it; transport affordances cannot substitute for sibling-contract authority and capability | Must | critical | sibling-contract inspection blocks implementation and activation when host-adapter performer authority, receipt-returning publication, or exactly-once semantics is absent or ambiguous; once explicit, the launched reviewer has no direct proposal credential and hands exactly one completed report to the impl-review-owned broker; one broker proposal write returns one native receipt and its exact launched-attempt artifact verifies under either the same or a different code-host author; implement-now contains no second publisher; syntactically valid comments prewritten by the implementation worker, copied across attempts, replayed across proposal/revision, not matching the receipt, or paired with attempted obligation/attempt/receipt-store mutation fail as `artifact_untrusted`; missing/malformed/stale artifacts fail as `artifact_unverified`; write/read transport failure is infrastructure; capability and secret-scan fixtures find no raw attribution material or canonical producer-signing authority in either child launch | Test + Failure injection + Inspection + Security review | G2, FR-10, ABUSE-6, prd:impl-review/FR-6 |

### op: supersede FR-37

Reason: run significance must distinguish an absent or malformed artifact, an artifact that fails launched-attempt proof, and infrastructure that prevented verification.

| ID | Requirement (EARS) | Rationale | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|
| FR-42 | If configured-review setup or an attempt cannot authenticate and verify its obligation, the executor shall append an escalated EVT-15 terminal with `failure_class=artifact_unverified` when the launched reviewer settles but no exact schema-valid artifact exists, `failure_class=artifact_untrusted` when a structurally eligible artifact fails broker-receipt, attempt-proof, or protected-state authentication, or `failure_class=infrastructure` for obligation-store, factory, auth, routing, launch, runtime, truncation, cancellation, attribution-service, broker-publication, or read-transport failure; it shall preserve every succeeded EVT-10, increment only the matching class plus `review_failed`, retain any created obligation in `review_pending`, and exit 1 rather than a backend-specific code. A factory/auth preflight failure for a frontier with no existing obligation shall emit one run-level EVT-15 with `obligation_status=not_created`, invent no pending work, spawn no implementation/reviewer, and fail no task; the same failure while selected persisted obligations already exist shall emit one infrastructure EVT-15 per selected obligation with `obligation_status=pending` and no invented attempt identity, retain them pending, and launch nothing. | opposite evidence failures need different repair paths, while setup failure before obligation creation and setup failure during durable recovery must not be conflated | Must | critical | one fixture per failure family preserves EVT-10, emits typed escalated EVT-15, increments only infrastructure/unverified/untrusted plus `review_failed`, retains every created obligation, and exits 1; new-work preflight failure is run-level/not-created with zero task terminals/launches/pending; selected-pending preflight failure is obligation-scoped/pending with zero launch and exact pending count; same-author and different-author transport behave identically when proof matches; a verified attempt has no failure class | Test + Failure injection | G2, J1, FR-15, FR-16, UB-20, UB-21 |

### op: supersede FR-34

Reason: one row cannot be triggered by a pending obligation while also requiring factory/auth failure before the implementation work that could create that obligation.

| ID | Requirement (EARS) | Rationale | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|
| FR-44 | After discovering non-empty implementation or selected review work, when and only when an `executor.review` block is present, the executor shall determine every distinct effective review backend needed by that frontier—the explicit `review.backend` for new tasks, each task's selected implementation backend when inherited, and each selected persisted obligation's frozen backend—then build its typed factory and authenticate its reviewer service identity exactly once per distinct backend before any implementation dispatch or review attempt. An absent block installs no review seam/preflight. Factory/auth/config failure takes FR-42's run-level `not_created` path for new work and its obligation-scoped `pending` path for selected persisted obligations, spawning no task/reviewer. A persisted obligation never switches backend. | optional review remains uninstalled by default; invalid setup fails before avoidable task/review spend; recovery cannot erase or switch a frozen backend | Must | critical | absent-config/combined-empty perform zero build/auth; explicit and mixed inherited new-work fixtures preflight each backend once before task dispatch; pending A plus current B preflights the union without switching A; new-work failure emits one run-level not-created terminal and zero task/pending work; selected-pending failure emits one pending infrastructure terminal per obligation and zero launch | Test + Failure injection + Inspection | G2, FR-16, FR-42, FR-47, NR-10 |

### op: supersede FR-38

Reason: one driver round must be able to submit a closed eligible subset so exhausted obligations do not relaunch and N obligations do not cause N competing full-frontier sweeps.

| ID | Requirement (EARS) | Rationale | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|
| FR-46 | When the executor is invoked for review-only recovery, it shall require a `drive_round_id` and closed non-empty set of selected in-scope review-obligation IDs, exact-read only that selected frontier, perform at most the FR-35 attempt for each selected obligation, and reconcile every selected member before a clean terminal; IDs that are unknown, verified, out of scope, duplicated, or not pending shall fail the invocation closed without widening the scan. The unattended driver owns durable recovery counts/bounds and issues at most one such invocation per round; the executor shall never infer eligibility from exit code, scan unselected pending obligations, redispatch succeeded implementation, or report clean while any selected obligation remains pending. | the provider owns attempts but not driver spend/exhaustion; an unbounded scan defeats mixed eligible/exhausted recovery and creates N-sweep races | Must | critical | multi-obligation fixtures prove one invocation/round, one attempt maximum per selected member, zero attempts for unselected/exhausted members, exact failure for empty/duplicate/unknown/verified/out-of-scope IDs, zero implementation activity, exit 1 while any selected member remains pending, and exit 0 only when every selected member verifies | Test + Failure injection + Demo | G1, G2, J1, FR-35, FR-47, UB-21 |

### op: supersede FR-39

Reason: the combined-empty rule must distinguish ordinary configured-review processing from the driver's closed selected recovery frontier and cannot cite retired FR-38.

| ID | Requirement (EARS) | Rationale | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|
| FR-47 | While the ready implementation frontier and applicable review frontier are both empty, the executor shall consume no model tokens. A normal invocation may process the durable in-scope review frontier created during that invocation; a review-only recovery invocation shall process only FR-46's non-empty selected set and shall perform zero implementation routing, claims, workers, or proposal operations. An empty selected recovery set is invalid rather than idle success. | zero work stays free without letting a recovery request widen, redispatch, or misclassify an invalid empty selection | Must | high | combined-empty normal fixtures emit zero work; normal pending-review fixtures reconcile only applicable obligations; selected recovery fixtures touch only members and no implementation path; empty selection fails before preflight/model work | Test + Failure injection | §3.3, FR-35, FR-46, NR-10 |

### op: supersede NR-9

Reason: the non-regression row cites retired FR-38 and does not protect selected-frontier confinement.

| ID | Existing behavior protected | Why protected | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|
| NR-10 | A normal run with no ready implementation and no applicable review obligation performs no routing, claim, reviewer, or model work; a review-only recovery run never routes/claims implementation and never scans or attempts an unselected obligation. | zero combined work remains zero inference, while recovery cannot launder pending work or widen the driver's bound | critical | combined-empty fixtures emit zero work; selected/unselected/exhausted fixtures prove byte-identical task/proposal identities, zero implementation launches, and zero unselected review reads/attempts | Test + Inspection + Failure injection | FR-46, FR-47 |

### op: supersede DOD-26

Reason: the complete non-regression proof must include NR-10 and drop retired NR-9.

| ID | Done criterion | Source | Verify | Required evidence | Blocking? |
|---|---|---|---|---|---|
| DOD-33 | No live non-regression row is broken. | NR-1..NR-6, NR-8, NR-10 | Test + Inspection + Failure injection | one result per live NR row, including v1 byte identity, selected-harness retention, e2e integrity, normal combined-empty zero work, selected-recovery zero implementation, and zero unselected review access | yes |

### op: add FR

Reason: per-obligation qualified route resolution and launch fidelity remain necessary after setup/auth admission is split into FR-44.

| ID | Requirement (EARS) | Rationale | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|
| FR-45 | When a pending review obligation is admitted after FR-44 succeeds, the executor shall use that configured backend's qualified review-role catalog and policy, resolve the obligation's assurance-band demand without a hardcoded weakest profile, and pass the resolved native model and effort without implementation-only controls, another backend's settings, repository-specific qualification assumptions, or wall-clock expiry of compiled policy. | review is a separately qualified semantic role, not an implementation rung, a universal strongest-profile request, or a reason to switch credential/egress boundaries | Must | critical | every supported adapter resolves every required review assurance band for a non-pyxis repository and an injected future clock, propagates model/effort into launch, rejects unauthorized policy, omits worker-only controls, and never switches the configured backend | Test + Analysis + Failure injection | G2, G3, FR-5, FR-35, NR-6 |

### op: add FR

Reason: a reviewer that shares the operator's parent clone and unbounded concurrency can mutate live state and race other reviewers even after implementation claims release.

| ID | Requirement (EARS) | Rationale | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|
| FR-43 | When launching a configured reviewer, the executor shall run it in a fresh disposable workspace pinned to the obligation's exact revision, separate from the parent checkout and every implementation/reviewer workspace, with a review-only brief and only the one-way completed-report handoff required by FR-41; reviewer attempts shall use a separately bounded review capacity and shall never consume implementation width, receive proposal credentials or canonical-evidence producer-signing authority, or mutate repository code, the parent checkout, or another workspace. | release-before-launch is insufficient if reviewers remain mutually concurrent in the live operator clone or can write around the protected persistence boundary | Must | critical | concurrent-review fixtures never exceed the review bound, hold zero implementation slots, see the exact captured revision, produce no cross-attempt file change, and leave parent/implementation workspaces byte-unchanged; direct proposal/store mutation attempts fail closed while proposal-artifact persistence remains available only through the impl-review-owned broker | Test + Failure injection + Inspection | G2, G3, FR-4, FR-35, ABUSE-2 |

### op: add UB

Reason: an artifact that looks current but lacks authentic launched-reviewer provenance must have a closed fail-closed outcome.

| ID | If… (trigger) | Then the system shall… | User sees | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|---|
| UB-20 | a candidate review artifact is pre-existing, worker-authored without the fresh launched-attempt proof, bound to another attempt/proposal/revision, not the broker-receipted artifact, or carries missing, replayed, mismatched, or otherwise unauthentic attribution proof | reject it without clearing the obligation, append EVT-15 with `failure_class=artifact_untrusted`, preserve the succeeded EVT-10, retain review-only pending work, and expose the expected versus observed non-secret receipt/proof identities without exposing raw attribution material | implementation remains succeeded; review remains pending and run-red with an attributable security failure | Must | critical | one adversarial fixture per trigger is rejected, same-author and different-author transport are accepted when receipt/proof match, no secret is emitted, `review_artifact_untrusted` increments, implementation never relaunches, and verification occurs only after a newly launched reviewer produces an authentic exact-revision artifact | Test + Failure injection + Security review | FR-41, FR-42, EVT-15, EVT-16 |

### op: supersede UB-19

Reason: interrupted and failed review recovery must use the immutable captured obligation, the three live failure classes, and the replacement terminal events; it must never re-derive a proposal revision.

| ID | If… (trigger) | Then the system shall… | User sees | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|---|
| UB-21 | a configured review is interrupted, fails infrastructure, produces no authenticated exact artifact, or remains pending when a later invocation has no ready implementation task | retain the immutable exact-green obligation, release all implementation resources, append the typed three-way failed attempt when one occurred, exact-read before any selected retry, and let the driver perform only consumer-bounded FR-46 review-only invocations; never re-derive a head, widen the selected frontier, redispatch succeeded implementation, or report clean while a selected obligation remains pending | implementation stays succeeded; the review obligation stays pending/run-red as `artifact_unverified`, `artifact_untrusted`, or `infrastructure` until authenticated verification; consumer recovery exhaustion remains explicit outside the provider | Must | critical | kill, auth, factory, route, runtime, truncation, malformed/absent artifact, forgery/replay, transport, selected empty-frontier restart, and exhausted-driver-budget fixtures retain exact obligations, launch zero implementation, never touch unselected obligations, keep one closed failure class, and exit 1 until every selected member verifies | Test + Failure injection | FR-35, FR-40..FR-47, EVT-15, EVT-16 |

### op: add ABUSE

Reason: the implementing worker is an adversarially relevant artifact writer because the code-host medium is shared.

| ID | Actor and intent | Vector | Impact | Severity | Mitigated by | Verify |
|---|---|---|---|---|---|---|
| ABUSE-6 | implementing worker, reviewer child, or repository contributor seeking to manufacture review completion | writes a schema-valid proposal comment before or after green, copies a prior proof, substitutes an artifact outside the broker receipt, or mutates/substitutes canonical obligation, attempt, or receipt state | a proposal appears independently reviewed when no launched reviewer produced the accepted artifact | critical | FR-41 fresh attribution plus an impl-review-owned single-write broker; canonical review evidence producer-signing capability owned only by the executor-side evidence adapter and absent from every child process/workspace, so generic ref-write access has no semantic authority; FR-43 isolated reviewer workspace and one-way report handoff; UB-20 fail-closed classification; raw attribution material excluded from worker/repository/event surfaces | worker-authored prewrite, direct reviewer proposal write, receipt/state substitution, unsigned/generic-ref store mutation, replay, and cross-revision drills all remain pending and emit `artifact_untrusted`; same-author transport through the broker with the launched proof verifies |

### op: supersede EVT-13

Reason: the review terminal needs authenticated producer evidence and a closed untrusted-artifact failure class.

| ID | Event name | Fires when | Properties (typed) | Source | Feeds |
|---|---|---|---|---|---|
| EVT-15 | `impl_review_terminal` | configured-review preflight/setup fails or one review-obligation attempt reaches an authenticated-verified or failed terminal | `drive_round_id:string?`, `review_attempt_id:string?`, `review_obligation_id:string?`, `task_id:string?`, `proposal_ref:string?`, `revision:string?`, `backend:string`, `reviewer_principal:string?`, `native_profile:string?`, `catalog:string?`, `policy:string?`, `model:string?`, `effort:string?`, `phase:enum{preflight,obligation,routing,launch,await,artifact,attribution}`, `status:enum{verified,failed}`, `obligation_status:enum{verified,pending,not_created}`, `failure_class:enum{artifact_unverified,artifact_untrusted,infrastructure}?`, `artifact_ref:string?`, `artifact_proof_ref:string?`, `reason:string?`, `escalated:boolean` | executor | M7, FR-35, FR-40..FR-47, UB-20, UB-21 |

### op: supersede EVT-14

Reason: the run terminal needs a distinct count for artifacts rejected on producer attribution.

| ID | Event name | Fires when | Properties (typed) | Source | Feeds |
|---|---|---|---|---|---|
| EVT-16 | `run_finished` | a run ends after implementation work and configured-review reconciliation settle for that invocation | `succeeded:int`, `failed:int`, `blocked:int`, `tokens:int`, `routing_exact:int`, `routing_promoted:int`, `routing_unsupported:int`, `routing_invalid:int`, `review_completed:int`, `review_failed:int`, `review_pending:int`, `review_artifact_unverified:int`, `review_artifact_untrusted:int`, `review_infrastructure_failed:int` | executor | M1, M2, M4, M6, M7, FR-42, FR-46 |

### op: supersede DOD-21

Reason: the obligation/order proof must cover per-task exact-revision capture and all release edges instead of relying on structural code shape.

| ID | Done criterion | Source | Verify | Required evidence | Blocking? |
|---|---|---|---|---|---|
| DOD-27 | Every configured green proposal captures one durable obligation at the verified revision before its EVT-10; that task's claim and width slot release before review admission; no sweep-end head re-read occurs; one invocation launches at most one uniquely identified attempt and deduplicates concurrent/repeated starts. | FR-35, FR-40 | Test + Failure injection + Inspection | executable green→persist→EVT-10→release→launch ordering trace, moving-head fixture, obligation persistence/recovery record, crash-point matrix, duplicate-invocation fixture, and distinct retry-attempt identities | yes |

### op: supersede DOD-22

Reason: review completion now requires authentic launched-reviewer provenance and isolated bounded execution in addition to exact artifact binding.

| ID | Done criterion | Source | Verify | Required evidence | Blocking? |
|---|---|---|---|---|---|
| DOD-28 | The live sibling contract and design explicitly authorize impl-review's host adapter to perform FR-6 for the reviewer program through exactly-one publication returning a native receipt; every supported adapter then preflights and launches the qualified review-role model/effort without worker controls; reviewers run in isolated exact-revision workspaces under separate bounded capacity and return one completed report to that broker; only its exact proposal artifact authenticated to the launched attempt and protected canonical receipt verifies, independent of code-host author; every failure emits EVT-15 with the correct three-way class and EVT-16 carries exact completed/failed/pending/class counts without changing EVT-10. | FR-41..FR-45, EVT-10, EVT-15, EVT-16, prd:impl-review/FR-6 | Test + Failure injection + Inspection + Security review | sibling-contract capability inspection plus absent/ambiguous-performer, missing-receipt, and missing-exactly-once refusal fixtures; absent/empty/factory/auth admission matrix; four-adapter assurance-band route/launch matrix; isolated-workspace/concurrency trace; exactly-one broker publish/native-receipt trace with no implement-now or child publisher; worker-prewrite/direct-write/receipt-or-state-substitution/store-mutation/replay/same-author/different-author/secret-leak matrix; malformed/stale/paginated/lagged/transport matrix; attended+JSONL wire assertions; task-outcome non-regression, tally, and exit assertions | yes |

### op: supersede DOD-17

Reason: the complete security posture must include review-artifact forgery and reviewer workspace isolation.

| ID | Done criterion | Source | Verify | Required evidence | Blocking? |
|---|---|---|---|---|---|
| DOD-29 | The complete local-runtime, native-routing, and configured-review security posture is proven. | ABUSE-1..ABUSE-6 | Test + Failure injection + Inspection + Security review | abuse-drill report covering every actor/vector, including worker-authored routing policy, artifact forgery/replay/receipt or canonical-state substitution, denied obligation/attempt/receipt-store mutation by both child roles, attribution-secret exclusion, parent/workspace mutation, and reviewer-capability confinement | yes |

### op: supersede DOD-24

Reason: the all-unwanted-behavior proof must include unauthentic review artifacts.

| ID | Done criterion | Source | Verify | Required evidence | Blocking? |
|---|---|---|---|---|---|
| DOD-30 | Every live unwanted-behavior row is handled. | UB-1..UB-9, UB-11..UB-18, UB-20, UB-21 | Test + Failure injection + Analysis + Inspection + Security review | failure-injection matrix with one result per live UB row, including canonical-evidence unavailability, durable review recovery, driver exhaustion, artifact forgery/provenance rejection, and zero omitted critical path | yes |

### op: supersede DOD-25

Reason: the all-Must proof must include exact-green capture, launched-attempt attribution, closed untrusted failures, and isolated bounded review execution.

| ID | Done criterion | Source | Verify | Required evidence | Blocking? |
|---|---|---|---|---|---|
| DOD-31 | Every live Must functional requirement passes its fit criterion, including field-identical plan transport, least-cost satisfying routing, immutable implementation evidence, exact-green durable review obligations, authenticated exact-artifact completion, three-way failure significance, isolated bounded reviewers, selected-frontier review-only recovery, and combined-frontier zero work. | FR-1..FR-5, FR-7..FR-10, FR-13..FR-18, FR-20..FR-32, FR-35, FR-40..FR-47 | Test + Demo + Inspection + Analysis + Failure injection + Security review | per-requirement evidence index including exact/promoted/unsupported/legacy/cutover, ordering/revision, setup/auth, assurance-band routing, attempt-attribution/receipt-substitution, isolation/concurrency, failure-class, selected-frontier/driver-exhaustion, and combined-empty cases | yes |

### op: supersede DOD-23

Reason: durable recovery evidence must use the captured obligation, authenticated completion, live three-way failure classes, and replacement events.

| ID | Done criterion | Source | Verify | Required evidence | Blocking? |
|---|---|---|---|---|---|
| DOD-32 | A killed, failed, or unauthentic review remains durably pending on its captured revision; an empty-implementation recovery invocation exact-reads only the driver's closed selected frontier, performs at most one attempt per selected member, touches no unselected/exhausted obligation, and performs zero implementation work. The consumer owns durable recovery counts/bounds, issues at most one invocation per round, exits clean only when every selected member authenticates, and keeps exhaustion visibly red. | FR-41, FR-42, FR-46, FR-47, NR-10, UB-20, UB-21, EVT-15, EVT-16 | Test + Failure injection + Demo + Security review | multi-obligation selected/unselected/exhausted truth table; kill/restart/forgery/setup-failure fixtures; empty/duplicate/unknown/verified/out-of-scope selection rejection; `drive_round_id` joins; zero-implementation trace; exact-revision retry; consumer-budget/exhaustion evidence; authenticated selected-to-verified transition; three-class counts | yes |

### op: supersede §1 Summary

Reason: configured review is now a durable authenticated completion obligation, so the executive summary cannot keep presenting review routing as excluded or omit it from success and risk.

- **Problem** — Once a plan is frozen and projected, pyxis can execute it locally, but implementation routing can still lose semantic demand and a configured post-green review can be skipped, forged, drift to a later proposal head, consume implementation capacity, or disappear after one red invocation.
- **For whom** — The operator with a pyxis-installed repo who has just planned in-session (attended), and argus driving the same core unattended.
- **Why now** — Plan-new freezes complete semantic demand on every plan/v2 task; the parent routing contract requires harness-specific native mappings; the e2e line projects ordinary tasks; and live configured review must now produce durable, attributable evidence without changing green implementation truth.
- **Direction** — Preserve the single-task worker and upstream harness choice. Recover or conservatively translate demand before claim; resolve qualified implementation and review roles through the selected harness; drive implementation to a genuinely-green exact revision; persist its review obligation before EVT-10, release implementation resources, and reconcile it through an isolated bounded reviewer whose exact artifact authenticates the launched attempt. Fail unsupported routing and review evidence in closed typed classes while retaining the implementation terminal.
- **Appetite** — The native local execution core, implementation-routing seam, e2e task consumption, and configured-review obligation/admission/verification/driver-consumer seam. Excludes harness selection policy, impl-review's internal pass/blindness/aggregation process, the remote runtime, hardware separation, umbrella policies, landing, and systematic e2e anti-gaming beyond the named scope.
- **Success** — `/implement-now` implements a real feature with every plan demand preserved, at least three native profiles reaching verified EVT-10 outcomes, one real e2e plan reaching genuine green, and every configured green proposal retaining one exact-revision obligation that becomes authenticated verified or remains visibly pending under one of three closed failure classes without implementation redispatch.
- **Guardrail** — It never merges, mutates the parent or another workspace, double-claims, downgrades demand, switches harnesses, rewrites plan/v1, launches under invalid routing, changes e2e subject behavior, re-derives a review revision, accepts worker-authored/replayed evidence, lets review consume implementation width, or launders pending review to green.
- **Top risks** — stale/unqualified native mappings; demand loss; mutable legacy inference; e2e anti-patterns and flake; charter bleed and unbounded loops; false green; lost/duplicated review obligations; forged or unreadable artifacts; shared reviewer state; and unattended retry laundering.

### op: supersede §4.1 Approach and alternatives considered

Reason: the chosen solution now includes a per-task durable review frontier and authenticated reviewer admission, not only implementation routing before claim.

The chosen shape remains a **durable execution core run in its simplest runtime**, extended by deterministic routing boundaries and a second durable review frontier. A self-contained single-task worker consumes one task charter plus its frozen plan/v2 demand—or a typed no-rewrite plan/v1 translation—and repo access. The invoking configuration has already selected one of the four harnesses. That harness's adapter resolves the implementation-role semantic minimum through closed qualified policy to the least-cost satisfying native profile, records exact/promotion/unsupported evidence, and only a resolved route may proceed to claim → isolated worktree → fresh agent → genuinely-green proposal.

At each configured green boundary, the executor captures the verified revision into a durable obligation before EVT-10, releases the task's claim and implementation slot, and admits that obligation immediately under separately bounded review capacity. A fresh isolated reviewer process receives attribution material minted only after the implementation worker terminates and returns one completed report through a one-way handoff. The **review artifact broker** is impl-review's host-side persistence adapter: as part of the reviewer program it performs impl-review FR-6's medium-appropriate proposal write, returns the native receipt, and exact-reads that object for authentication; implement-now integrates this boundary but does not become a second publisher. Only the broker-receipted exact artifact with matching attempt proof and protected canonical state clears the obligation. Code-host author identity is transport metadata and is not required to differ from the implementation identity. Missing evidence, unauthentic evidence, and broken infrastructure remain distinct, pending, and run-red. The Dispatcher still owns implementation capacity; the review frontier owns its own capacity; neither path chooses or switches harnesses, and impl-review continues to own its internal pass/blindness/aggregation/persistence program.

| Option | Sketch | Why not chosen |
|---|---|---|
| A (chosen) | frozen demand → qualified implementation → exact-green obligation → released implementation resources → isolated attributed reviewer → impl-review-owned broker publish/receipt → authenticated artifact | preserves plan meaning, implementation throughput, exact revision, sibling publication ownership, and independent review evidence |
| B | hardcoded native model/effort | collapses independent needs and cannot satisfy least-powerful-that-suffices |
| C | infer complexity from charter/repo at claim time | re-judges with less context after freeze and produces unauditable drift |
| D | store native settings or harness choice in the plan | couples frozen artifacts to vendor catalogs and credential/egress policy |
| E | switch harnesses when unsupported | violates the operator's credential and egress decision |
| F | launch all reviews after the implementation sweep in the parent clone | delays outcomes, re-derives mutable heads, and races reviewers in operator state |
| G | accept any matching proposal comment | lets the implementing worker forge the sole evidence that independent review occurred |
| H | treat review exit code as success or failure of the implementation task | confuses process health with artifact truth and rewrites green implementation history |
| Do nothing | — | routing and configured review remain silent, unauthenticated, and unrecoverable |

### op: supersede §4.2 Scope perimeter

Reason: implement-now owns configured review obligation/admission/routing and must distinguish that seam from impl-review's internal assurance process.

- **In (plan of record)** — the single-task worker; four-CLI backend-agnostic runtime with fresh sessions; claim + worktree isolation; bounded implementation dispatch; drive-to-green; demand integrity/translation; selected-harness qualified implementation and review-role resolution; immutable routing/outcome evidence; per-task exact-green review obligations; isolated attributed reviewer admission under separate capacity; authenticated exact-artifact reconciliation; three review failure classes; review-only driver recovery; strongest-profile fallback; test-then-direct activation.
- **Future (believed next, not committed)** — the remote/Docker dispatch adapter; hardware isolation; umbrella policy; auto-landing (WR-1, WR-2).
- **Out** — merging; hardware separation; fleet umbrella policy; choosing or switching harnesses; planner assessment/derivation; proposal-risk classification; impl-review pass count, blindness, lens prompts, candidate screening, and aggregation; adaptive routing or cross-harness optimization.

### op: supersede §4.3 Experience: flows and states

Reason: the flow must expose per-task capture/release/admission ordering, authenticated completion, and all three review failure classes.

**Flow F1 — an in-session run (serves J1, J3)**

| Step | User does | System responds | States covered | Design |
|---|---|---|---|---|
| 1 | invokes `/implement-now` on a frozen, projected plan with an explicitly selected/ambient harness | preflights configured implementation/review backends, reports ready implementation plus pending review frontiers, and exits idle only when both are empty | ideal · empty · review-only · infrastructure | superseding tech design |
| 2 | watches (attended) or supplies the journal (argus) | validates/translates demand, resolves the selected harness's implementation mapping, and refuses invalid/unsupported tasks before claim | ideal · unsupported · integrity error · legacy error | superseding tech design |
| 3 | — | claims only resolved tasks and drives them to green; for each configured proposal, atomically captures the exact verified revision, emits EVT-10, releases the claim and implementation width, then admits review without waiting for the sweep | loading · partial · infrastructure | superseding tech design |
| 4 | — | exact-reads pending obligations; otherwise launches at most one qualified reviewer attempt in an isolated exact-revision workspace under separate capacity with fresh attribution; the reviewer returns one completed report to impl-review's persistence broker, and only that broker's receipted exact artifact with matching attempt proof clears pending, independent of code-host author | review-only · verified · artifact-unverified · artifact-untrusted · infrastructure | superseding tech design + driver consumer amendment |
| 5 | reviews the final report | receives independent implementation classes plus review completed/failed/pending and three failure counts; the unattended driver consumes pending as bounded review-only incomplete work | ideal · partial · review-pending · error | superseding tech design + driver consumer amendment |

- **Empty** — both frontiers empty: report and exit with zero routing, claim, reviewer launch, or model work.
- **Review-only** — pending review with no ready implementation: exact-read/review only; zero implementation route, claim, worker, or proposal activity.
- **Loading** — a resolved worker implements or awaits CI; live status names harness/native profile but no prompt/code payload.
- **Partial** — implementation terminal classes and review verified/pending classes remain separate; independent valid work continues.
- **Error** — implementation routing/execution follows its existing path; review preserves EVT-10 and emits exactly `artifact_unverified`, `artifact_untrusted`, or `infrastructure`, leaving the obligation pending and exit 1 through bounded recovery.
- **Accessibility / localization** — unchanged terminal/agent-driven, English-only surface.

### op: supersede §4.4 Scenarios

Reason: the scenario set must include exact-revision admission, forgery rejection, and durable review-only recovery rather than ending at implementation routing.

**Common implementation case.** A plan/v2 foundation task and two leaves carry different semantic profiles. The selected harness resolves exact or promoted qualified native profiles, emits joined EVT-7/EVT-10 evidence, and drives each to genuine green.

**Configured review case.** At each green boundary, the executor captures that verified head before EVT-10, releases the task claim and implementation slot, then reviews the captured revision in an isolated workspace while later implementation tasks continue. The launched reviewer process returns exactly one completed report to the review artifact broker. That broker is the impl-review-owned persistence adapter executing impl-review FR-6: it publishes the report on the proposal and returns the native receipt. Implement-now supplies the attempt binding and consumes the receipt but neither performs nor reimplements the write. Matching attempt proof exact-read verifies it and clears only that obligation regardless of which authenticated code-host account transported it.

**Moving head and forgery.** A contributor pushes after green while the implementing worker has already written a schema-valid comment. The obligation remains pinned to the original green revision; the prewritten comment fails `artifact_untrusted`; no later head lookup or implementation redispatch occurs; only a fresh launched reviewer artifact can verify.

**Interrupted review-only recovery.** The reviewer is killed after implementation succeeds. EVT-10 remains succeeded, EVT-15 records infrastructure, EVT-16 reports pending, and exit is 1. The next invocation has an empty implementation frontier, exact-reads the same obligation, performs only bounded review work, and cannot exit 0 until authenticated verification.

**Unsupported implementation task.** The selected harness cannot satisfy demand. EVT-8 fires before claim/invocation; another harness is never tried and independent work continues.

**Legacy task.** A frozen plan/v1 task with no rung translates as `legacy_unassessed` without rewriting source bytes; an unknown explicit rung fails before claim.

**Cutover and fallback.** After the full implementation/review conformance, dogfood, forgery, moving-head, isolation, failure-class, and recovery matrices pass, one switch activates all supported paths. A defective mapping uses the strongest qualified profile within the selected harness or fails before claim/admission.

**Decision:** Solution gate — zero TBDs may pass.

| Reviewer | Role | Date | Verdict |
|---|---|---|---|
| @mdarabi | operator (via the introducing PR's review) | — | — |

### op: supersede §3.2 Goals → signals → metrics

Reason: M7 must measure authenticated launched-reviewer evidence and the new untrusted-artifact class through the live replacement events.

| Metric | Goal | Signal | Baseline | Target (at checkpoint) | Feeds from |
|---|---|---|---|---|---|
| M1 (primary) | G1 | a real pyxis feature's tasks reach green PRs via `/implement-now` | 0 features under demand-aware routing | 1 dogfood feature fully implemented locally, operator only reviewing/merging | EVT-10, EVT-16 |
| M2 | G2 | tasks counted done that are genuinely green | n/a | 100% of succeeded tasks carry a green proposal; zero hollow-as-succeeded | EVT-4, EVT-10, EVT-16 |
| M3 | G3 | worker core and runtime adapters retain portability | one coarse rung mapper | the worker passes unchanged behind fake/local dispatch; every supported harness passes implementation-role routing conformance | EVT-7, EVT-8, EVT-16 |
| M4 | G1 | requested semantic profiles whose plan/v2 fields are identical to frozen demand or whose plan/v1 origin is a typed FR-22 translation, followed by a satisfying or explicit unsupported result | optional rung can be absent and demand can be lost | 100% exact origin/transport/resolution; zero inferred, downgraded, native-in-plan, or cross-harness substitutions | EVT-7, EVT-8, EVT-9, EVT-16 |
| M5 | G1 | distinct native implementation profiles exercised by dogfood attempts that succeed with `verification=verified` | hardcoded/implicit route can converge on one pin | at least 3 before production activation; failed/blocked attempts never count | EVT-7, EVT-10 |
| M6 | G2 | routing attempts with exactly one origin, one resolution terminal, and—when resolved—one worker outcome | no complete decision chain | 100% in conformance and dogfood before activation; remains 100% over production attempts after activation; zero orphan/duplicate/cross-class terminals | EVT-7..EVT-10, EVT-16 |
| M7 *(005 ← 004)* | G2 | configured green proposals with one exact-green-revision obligation and append-only attempts: verified only by an exact artifact authenticated to the launched reviewer, otherwise three-way closed-class failed and still pending | configured review can be silently absent, forged, process-exit-derived, drifted to a later head, or red for only one invocation | 100% of configured green proposals; zero absent/drifted obligations, duplicate attempts, worker-authored or replayed accepted artifacts, artifact-free verified terminals, unclassified failures, pending-with-exit-zero runs, implementation redispatches, or pending obligations laundered by a later empty frontier | EVT-15, EVT-16 |

### op: supersede §3.3 Guardrails and decision rule

Reason: the review guardrail and ship rule must block forged evidence, parent/workspace mutation, sweep drift, and the live replacement DoD/event rows.

| Guardrail metric | Threshold | Action if breached |
|---|---|---|
| Parent/workspace mutation | any implementation or reviewer mutation of the parent clone, engine checkout, another task workspace, or repository code outside its own implementation charter | fail-closed; halt the offending process, retain evidence, and alarm |
| Double-claim with the external fleet | any task worked concurrently by both a local worker and a fleet node | fail-closed; block release until the claim mechanism is fixed |
| Self-merge | any merge performed by the executor | critical defect; executor holds no merge credential |
| Idle cost | any implementation routing/claim/model spend when its frontier is empty, any reviewer spend when no review obligation is pending, or any model spend when both frontiers are empty | investigate the combined-frontier gate and block release on an unauthorized launch |
| Semantic downgrade or harness switch | any native launch fails to satisfy one requested component, uses an unqualified mapping, or changes the selected harness | stop affected claims; block/rollback immediately; repair policy and rerun affected attempts |
| Frozen-plan mutation or post-freeze inference | any plan/v1 rewrite or plan/v2 assessment/profile reinterpretation | invalidate affected evidence; require no-rewrite translation or field-identical transport before retry |
| Routing evidence break | any attempt lacks exactly one origin/terminal or a resolved attempt lacks exactly one outcome | do not count it succeeded or toward release; preserve artifacts and repair the evidence chain |
| Configured review integrity break | any configured green proposal lacks one exact-green-revision obligation, re-derives a later head, launches before claim/width release, runs in the parent/shared workspace or beyond review capacity, verifies without the exact launched-attempt receipt and proof, fails without a three-way class/escalation/tally/exit 1, duplicates an attempt, or becomes clean while still pending | preserve EVT-10, reject unauthentic artifacts, block release/activation, and reconcile the same obligation/revision through the bounded review-only path |

**Ship rule (pre-registered):** M1–M7 meet their pre-release targets, every live blocking DoD row (DOD-5, DOD-8..DOD-9, DOD-11..DOD-14, DOD-18..DOD-19, DOD-27..DOD-33) is green, and every guardrail stays clean through complete pre-release proof → activate every supported production implementation and configured review path in one cutover; post-launch M4–M7 and every guardrail remain continuously evaluated; any breach → no activation or immediate strongest-profile fallback, affected-attempt invalidation, and exact-revision review-only reconciliation where applicable.

### op: supersede §3.4 Non-goals and waiting room

Reason: the deliberate deferral of a distinct code-host reviewer principal is a product decision and must be visible in the live contract, not only in its review overlay.

| ID | Non-goal | Why refused |
|---|---|---|
| NG-1 | Merging / landing proposals | landing is merge-lane's, gated by a human or the AUTO lane; the executor holds no merge credential (`brainstorm:implement-now#O-1`) |
| NG-2 | Hardware separation (isolated/other machines, containers, the deny-by-omission security posture) | the remote/Docker fleet's value-add; this runs on the same hardware (`brainstorm:implement-now#O-2`) |
| NG-3 | Fleet umbrella policies (org/managed-set governance, risk & size caps, resource limits, node leases/heartbeats) | the remote runtime's layer; this runs under ambient/local policy (`brainstorm:implement-now#O-3`) |
| NG-4 | A new work substrate or projection path | it reuses the planning stage's projection and the tracker convention unchanged (`brainstorm:implement-now#D-4`) |
| NG-5 | Authoring or revising e2e coverage — scenarios, routes, mock strategies, harness choice, or the coverage map | those are e2e-design's decisions, settled by e2e-review and the operator's approval; the executor consumes the approved document and never edits or reinterprets it (`prd:e2e-verification/FR-1`, `FR-4`) |

| ID | Waiting room | Revisit when / intended version |
|---|---|---|
| WR-1 | The remote/Docker runtime (a second dispatch adapter + hardware isolation + umbrella policy over the same worker core) | when hardware separation or fleet governance is needed; the worker core is built to receive it |
| WR-2 | Auto-landing (the executor triggers merge-lane AUTO after green) | when merge-lane AUTO is activated for the repo and the operator wants it |
| WR-3 | The rest of the e2e execution follow-on: systematic run-time anti-gaming (falsification sweeps and e2e test-delta beyond DOD-19's per-scenario broken-subject check), driving the harder app shapes such as web UI to green in CI, and the dedicated no-regression gate | when the e2e authoring line has produced plans across more than one app shape and this amendment's consumption path is proven on a real plan (`prd:e2e-verification/WR-1`, `WR-5`) |
| WR-4 | A distinct code-host reviewer principal, separate from the implementation/operator transport identity | when teams want identity-level separation and a provisioning, rotation, onboarding, author-verification, and local-only fallback contract is approved; this version deliberately authenticates the impl-review-owned broker receipt plus fresh attempt proof and treats author identity as non-authoritative transport metadata |

### op: supersede §5.3 Permissions and abuse cases

Reason: the security boundary must deny implementation workers the attempt attribution capability and isolate reviewer mutation authority.

*Access: `/implement-now` runs under the invoking identity (the operator's, attended; argus's grant, unattended). Implementing workers run headless with bypass-permission autonomy only inside their task worktrees. Reviewer child processes run in fresh exact-revision disposable workspaces with no repository-code mutation mandate, direct proposal credential, or canonical-evidence producer authority; they return one completed report to the impl-review-owned host broker. Attempt attribution is minted after implementation termination and withheld from implementation surfaces. Only the executor-side evidence adapter receives the producer-signing capability for canonical obligation/attempt/receipt state; generic repository/ref credentials cannot authorize it. The executor holds no merge/approve credential (NR-3).*

| Function / data | Operator | argus (granted) | Unauthenticated |
|---|---|---|---|
| invoke a run | invoke | invoke (under grant) | none |
| implementing worker: modify code | via worker, task worktree only | via worker, task worktree only | none |
| reviewer child: read exact revision + return attempt-bound completed report | via isolated reviewer and one-way broker handoff; no direct proposal, canonical producer-signing, code, or parent write authority | same under grant | none |
| impl-review persistence broker: publish completed report + exact-read receipt | via preflighted host adapter acting for the reviewer program under impl-review FR-6 | same under grant | none |
| canonical review obligation/attempt/receipt state | executor-side evidence adapter producer-signing authority only; generic repository/ref credentials held by a child cannot authorize state | same under grant | none |
| review attribution material | raw material executor-only; derived proof to the isolated reviewer attempt only; never implementation worker | same under grant | none |
| merge a proposal | none (executor never merges) | none | none |

| ID | Actor and intent | Vector | Impact | Severity | Mitigated by | Verify |
|---|---|---|---|---|---|---|
| ABUSE-1 | tracker/repo/CI content bends a worker into acting on injected instructions | untrusted text read during a task treated as instruction | worker does attacker-directed work | high | FR-3, FR-19 (charter is the instruction; read content is data) + NG-1 (can't merge) + FR-4 (worktree-bounded) | drill: injected content in a fixture task changes no behavior |
| ABUSE-2 | an implementation worker or reviewer escapes its workspace to mutate the parent checkout, engine, or another attempt | writing outside the isolated workspace | corrupts shared state or contaminates another result | high | FR-4, FR-43, configured review guardrail | Test: parent/cross-workspace mutation attempts are blocked |
| ABUSE-3 | secrets exfiltrate through failed-CI logs piped to a fixer worker | unredacted logs in fixer context | credential/secret leak | high | FR-9 (redact + clamp failed-CI logs before inlining) | drill: seeded secret is redacted before it reaches the fixer |
| ABUSE-4 | a bypass-permission worker performs a destructive/outward act | autonomous execution on local hardware | local damage / unwanted egress | med | worktree isolation (FR-4/FR-43) + never-merge (NG-1) + ambient/local policy is the stated posture (NG-2/NG-3); hardware/policy separation is the fleet's job | Inspection: the posture and its boundary are stated and bounded |
| ABUSE-5 *(001)* | repository contributor, compromised worker, or mistaken operator seeking cheaper or arbitrary execution | adds a weakening/unqualified mapping, unknown launch key, raw CLI argument, or self-declared native capability to routing policy | a worker launches below demand or with attacker-controlled egress/permissions | critical | FR-23 requires qualified dominance; FR-24/UB-18 fail before claim; NFR-7 closed parsing; operator-authorized policy and reviewed qualification evidence | contributor-only, worker-authored, weakening, unknown-key, argument-injection, and self-qualification fixtures all produce zero claims/invocations |
| ABUSE-6 *(005)* | implementing worker, reviewer child, or repository contributor seeking to manufacture review completion | writes a schema-valid proposal comment before or after green, copies a prior proof, substitutes an artifact outside the broker receipt, or mutates/substitutes canonical obligation, attempt, or receipt state | a proposal appears independently reviewed when no launched reviewer produced the accepted artifact | critical | FR-41 fresh attribution plus an impl-review-owned single-write broker; canonical review evidence producer-signing capability owned only by the executor-side evidence adapter and absent from every child process/workspace, so generic ref-write access has no semantic authority; FR-43 isolated reviewer workspace and one-way report handoff; UB-20 fail-closed classification; raw attribution material excluded from worker/repository/event surfaces | worker-authored prewrite, direct reviewer proposal write, receipt/state substitution, unsigned/generic-ref store mutation, replay, and cross-revision drills all remain pending and emit `artifact_untrusted`; same-author transport through the broker with the launched proof verifies |

### op: supersede §5.4 Data

Reason: the data contract needs immutable exact-green revision capture and an attribution-proof lifecycle that excludes raw material from implementation surfaces.

| Entity / field | Meaning | Source of truth | Retention | Integrity rules | Privacy class |
|---|---|---|---|---|---|
| plan/v2 execution demand | frozen assessment, citations, semantic profile, derivation version, raise evidence | frozen plan; projected copy is transport | with plan | field-identical through executor boundary; immutable; no native identifiers | internal |
| plan/v1 legacy origin | original rung presence/value plus compatibility version and translated profile | immutable source plan + typed EVT-9 | forever with routing evidence | source never rewritten; explicit or `legacy_unassessed`; unknown fails | internal |
| e2e scenario basis | the scenario flow, route, mock strategy, setup/teardown, harness selection, and coverage rows a recognized e2e task must satisfy | approved `e2e-design/v1` at the recorded revision | with feature e2e artifacts | read-only; exactly one scenario/harness slot; malformed or ambiguous refuses | internal |
| native routing policy | qualified harness×role mappings plus authorized repo overrides | released defaults + validated repo policy | current + repository history | closed/version-compatible; qualification evidence; no weakening/raw arguments | internal |
| routing attempt | selected harness/role, requested profile, policy version, result, native identity | immutable EVT-7 or EVT-8 | forever in git evidence | exactly one terminal; connected to one origin | internal |
| e2e origin decision | resolved/refused/not-e2e basis join | immutable EVT-12 | forever in git evidence | append-only; one per ready task/run; before claim | internal |
| worker outcome | verified/failed/blocked implementation terminal and usage | immutable EVT-10 + proposal evidence | forever in git evidence | one per resolved attempt; never rewritten by review state | internal |
| review obligation | requirement to obtain one authenticated review artifact for the exact revision observed by green verification | durable medium-neutral canonical obligation store + authoritative proposal verification result | pending until verified, then forever | stable identity; exact revision captured once before EVT-10/release; never re-derived; only the executor-side evidence adapter holds producer-signing authority; generic repository/ref credentials cannot authorize a mutation; only FR-41 verification clears pending | internal |
| review attempt attribution | attempt/proposal/revision binding, broker receipt, and one-way proof reference; only the derived non-secret proof crosses executor→isolated reviewer | durable canonical attempt/receipt store under the executor-side evidence adapter; raw minting entropy is ephemeral executor-only state | non-secret proof forever; raw entropy destroyed immediately after derivation | fresh per attempt after implementation termination; raw entropy unavailable to every child/repo/event; canonical producer-signing authority unavailable to every worker/reviewer child; unsigned/direct-push, receipt/proof replay, mismatch, mutation, or substitution fails; code-host author is non-authoritative transport metadata | secret while minting; internal after one-way derivation |
| review artifact broker receipt | exact native locator returned by the impl-review-owned host-side persistence write | impl-review persistence adapter + exact code-host object | forever with attempt evidence | exactly one schema-valid completed-report publish per attempt; bound to attempt/proposal/revision; implement-now and child processes cannot mint or replace it; exact-read before verification | internal |
| review attempt terminal | append-only authenticated-verified or three-way failed result | immutable EVT-15 + authenticated proposal artifact when verified | forever with review evidence | fresh attempt identity; stable obligation join; at most one launch/obligation/invocation; failure never mutates EVT-10 or clears pending | internal |
| evidence invalidation | affected implementation evidence and disposition | immutable EVT-11 | forever | append-only; exact identities; never rewrites accepted artifact | internal |
| task claim | at-most-one-worker lease | shared claim mechanism | until release/reap | paired release; never held by review | internal |
| implementation workspace | isolated worktree + branch | local git | until reaped | fetched-mainline base; parent never mutated | internal |
| reviewer workspace | disposable read-oriented workspace pinned to obligation revision | executor workspace adapter | until attempt settles/reaped | separate from parent and all attempts; no repository-code mutation; separate capacity | internal |
| proposal | task change + verification/review evidence | code host | per host | one per task; green before succeeded; review artifact binds exact revision and reviewer attempt | internal |

- **Migration** — no artifact is rewritten. Pending obligations created under 004 remain pending and receive fresh attribution on their next attempt; an old unauthenticated comment never auto-verifies. Existing green proposals derive one exact-revision obligation from authoritative green evidence, never the mutable current head.
- **Parallel-run / cutover** — no production shadow/canary. Pre-release conformance and dogfood exercise exact-green capture, isolated review, all three failure classes, and authenticated persistence before one direct cutover.
- **Rollback** — pending obligations, non-secret proof references, and append-only attempts survive rollback; raw attribution material is never retained or reconstructed.
- **Decommission** — remove sweep-end head re-derivation, parent-clone reviewer launch, unauthenticated artifact acceptance, and invocation-local-only review only after the replacement drills pass.
- **Privacy check** — no personal/sensitive data added; any reviewer principal retained in runtime telemetry is an existing operational identity and is not verifier input.
- **Egress** — the selected harness and proposal medium remain the only model/code-host boundaries; attribution adds non-secret proof metadata, never prompts/code or raw material.

### op: supersede §5.5 Interfaces and integrations

Reason: the review runtime and persistence interfaces need explicit isolated-workspace, broker-receipt, and attempt-proof contracts.

| Interface | Direction | Purpose | Contract | Failure modes | Owner |
|---|---|---|---|---|---|
| frozen plan/v2 + projected task | in | supply charter and complete semantic demand | versioned closed schema; projected fields identical to frozen record | missing/changed/native field → UB-9 before claim | plan-new + tracker adapters |
| frozen plan/v1 | in | legacy execution without rewrite | explicit rung through versioned table or absent-rung `legacy_unassessed` | unknown rung/version → UB-11 | compatibility owner + executor |
| feature e2e design doc (`e2e-design/v1`) | in | supply reviewed scenario basis | approved, well-formed, exact recorded revision; read as data | missing/not-approved/malformed/ambiguous → UB-14 | e2e-design + e2e-review |
| projected e2e plan task | in | deliver e2e work through ordinary frontier | unchanged plan/task/charter schema | absent basis → UB-14; schema fork breaches NR-8 | e2e-plan + plan-new |
| harness selection | in | choose implementation and optional review credential/egress/runtime boundaries | implementation backend explicit/ambient; review block absent means uninstalled, otherwise explicit backend or typed inheritance; no installed-CLI inference | absent/unauthenticated → FR-16/FR-44 | operator/invoker |
| executor → harness adapter (implementation) | out | resolve implementation profile | qualified implementation-role mapping | invalid/unsatisfied → FR-24/UB-18 | runtime adapters |
| harness adapter → canonical evidence store | out | append/exact-read implementation route terminal | configured git evidence ref; immutable exact-read proof | unavailable/unverifiable → no claim/invocation | evidence adapter |
| harness adapter → AgentRuntime (implementation) | out | launch resolved worker | native settings from adapter policy only | injection/mismatch → UB-18/ABUSE-5 | runtime adapter |
| executor ↔ review obligation/attempt/receipt store | both | persist/recover exact-green proposal revision, attempt proof, broker receipt, and terminal state | closed versioned canonical record; stable obligation; pending/verified; only executor-side evidence adapter receives producer-signing authority; no raw attribution material or producer key reaches a child, and generic repository/ref credentials cannot authorize a canonical event | unavailable/unverifiable → infrastructure, resources released; unsigned/direct-push mutation or substitution → artifact-untrusted; re-derive only from authoritative green evidence | executor + obligation-store adapter |
| executor → reviewer workspace adapter | out | create/reap isolated exact-revision workspace under separate review capacity | one disposable workspace per attempt; parent/all other workspaces immutable; no implementation slot | create/reap/mutation/concurrency failure → EVT-15 infrastructure + pending | executor workspace/dispatch owners |
| executor → harness adapter (review) | out | preflight, resolve, and launch one attributed reviewer child | qualified review-role policy; concrete model/effort; review-only brief; fresh derived non-secret attempt proof delivered only to isolated launch; raw minting entropy and proposal/canonical-store credentials remain host-local | auth/factory/policy/launch/runtime/truncation → EVT-15 infrastructure + pending; preflight before avoidable task spend | runtime adapters |
| review orchestration host → impl-review review artifact broker | out | hand one completed attempt-bound report to impl-review's medium-specific publisher and receive its exact native receipt | available only after the live sibling contract and design explicitly authorize the impl-review-owned persistence adapter to perform `prd:impl-review/FR-6` for the reviewer program and accept exactly one schema-valid completed report for the frozen attempt/proposal/revision while returning its native receipt; implement-now supplies binding and consumes receipt but neither owns nor reimplements publication | absent or ambiguous performer authority, receipt-returning capability, or exactly-once semantics → dependency block before implementation/activation; invalid/surplus report → artifact_unverified; write/receipt/auth failure → infrastructure; no second/direct publisher path | impl-review persistence |
| impl-review persistence adapter ↔ code host | both | publish then exact-read the broker-receipted review artifact | medium-appropriate proposal write under impl-review FR-6; schema-valid proposal/revision/attempt artifact pinned to returned native receipt; attempt proof matches protected canonical verifier state; code-host author ignored for completion; pagination/lag bounded; malformed/unrelated candidates skipped | missing/malformed → artifact_unverified; receipt/proof/replay/state mismatch → artifact_untrusted; transport/auth → infrastructure | impl-review persistence + code host |
| driver → executor review-only request | in | reconcile one consumer-selected eligible frontier | closed non-empty obligation-ID set + `drive_round_id`; exact-read selected members only; no widening/inference | empty/duplicate/unknown/verified/out-of-scope selection fails closed; setup failure emits selected pending infrastructure terminals | self-drive/feature-drive consumer + executor |
| executor → driver/report sinks | out | expose implementation truth separately from authenticated review state | EVT-10 implementation; EVT-15 carries optional `drive_round_id` and review terminal; EVT-16 completed/failed/pending/three classes; pending means review-only incomplete | absent/unknown fields fail closed; consumer exhaustion stays red | executor + self-drive/feature-drive consumer |
| runtime → implementation evidence stream | out | preserve origin, route, worker outcome | EVT-7..EVT-10 and EVT-12 | orphan/duplicate/missing edge → UB-12 | executor + evidence owner |
| code host | both | proposals, CI, authenticated review artifacts | existing proposal/check/comment surface; impl-review persistence is the artifact publisher under FR-6; exact revision, broker receipt, and attempt proof bind completion; author is observational only | lag bounded; stale/unauthentic never verifies | impl-review persistence + this feature's verifier |
| merge-lane | out | landing downstream | proposals only; never merges here | no landing behavior owned | merge-lane |

### op: supersede §5.7 Instrumentation — the tracking plan

Reason: superseded EVT-13/EVT-14 must leave the live table and the replacement events must carry attribution evidence and the untrusted count.

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
| EVT-15 *(005 ← EVT-13)* | `impl_review_terminal` | configured-review preflight/setup fails or one review-obligation attempt reaches an authenticated-verified or failed terminal | `drive_round_id:string?`, `review_attempt_id:string?`, `review_obligation_id:string?`, `task_id:string?`, `proposal_ref:string?`, `revision:string?`, `backend:string`, `reviewer_principal:string?`, `native_profile:string?`, `catalog:string?`, `policy:string?`, `model:string?`, `effort:string?`, `phase:enum{preflight,obligation,routing,launch,await,artifact,attribution}`, `status:enum{verified,failed}`, `obligation_status:enum{verified,pending,not_created}`, `failure_class:enum{artifact_unverified,artifact_untrusted,infrastructure}?`, `artifact_ref:string?`, `artifact_proof_ref:string?`, `reason:string?`, `escalated:boolean` | executor | M7, FR-35, FR-40..FR-47, UB-20, UB-21 |
| EVT-16 *(005 ← EVT-14 ← EVT-6)* | `run_finished` | a run ends after implementation work and configured-review reconciliation settle for that invocation | `succeeded:int`, `failed:int`, `blocked:int`, `tokens:int`, `routing_exact:int`, `routing_promoted:int`, `routing_unsupported:int`, `routing_invalid:int`, `review_completed:int`, `review_failed:int`, `review_pending:int`, `review_artifact_unverified:int`, `review_artifact_untrusted:int`, `review_infrastructure_failed:int` | executor | M1, M2, M4, M6, M7, FR-42, FR-46 |

### op: supersede §6.1 Phasing and milestones

Reason: integration and release phases must exercise exact-green capture, attribution, reviewer isolation, and the third failure class.

| Phase | Purpose | Scope | Exit criterion |
|---|---|---|---|
| Demand boundary | consume without re-judging | plan/v2 integrity/transport, plan/v1 translation, origin identities, negative fixtures | FR-21/FR-22, NR-5, UB-9/UB-11 green; frozen input byte-identity proven |
| Native resolution | resolve through every selected harness | implementation/review mappings, qualification, overrides, least-cost resolver, promotions, unsupported, append-first implementation evidence | FR-23..FR-26/FR-44/FR-45, NFR-7/NFR-9, EVAL-2, DOD-11/DOD-14/DOD-28 green |
| Worker integration | preserve implementation and review truth separately | exact-green obligation capture before EVT-10/release; isolated exact-revision reviewer admission; authenticated artifact persistence; EVT-15 without EVT-10 rewrite | FR-25/FR-35/FR-40..FR-45, EVT-7..EVT-12/EVT-15 green; DOD-27/DOD-28 ordering, attribution, isolation, and failure matrices complete |
| Driver integration | make review-red durable across invocations | combined frontier, consumer recovery ledger, one closed selected-frontier invocation per round, EVT-15 round joins, EVT-16 counts, exhaustion | FR-46/FR-47, NR-10, UB-20/UB-21, DOD-32/DOD-33 green; empty-implementation restart does zero implementation work and cannot launder pending/untrusted review |
| Pre-release dogfood | falsify the full seam | one real feature, three profiles, route/evidence/fallback, authenticated review, three failure classes, interruption/retry/exhaustion, forgery/replay, isolation/concurrency | M1–M7, every live blocking DoD row (DOD-5, DOD-8..DOD-9, DOD-11..DOD-14, DOD-18..DOD-19, DOD-27..DOD-33), and every §3.3 guardrail green |
| Production | replace implicit routing and unauthenticated ephemeral review | every supported implementation/configured-review path plus driver consumer | one direct cutover succeeds; old pin, sweep-end head derivation, parent-clone reviewer, unauthenticated acceptance, silent review, and next-sweep laundering are unreachable |

### op: supersede §6.2 Dependencies

Reason: the design and consumer dependencies must cite live events/requirements and explicitly account for authenticated attribution and isolated per-task review admission.

| Dependency | Type | Needed by | Status | Owner |
|---|---|---|---|---|
| plan-new plan/v2 demand and checkpoint contract (`prd:plan-new/FR-28..FR-30`) | approval / data contract | Demand boundary | available through merged amendment 004 | plan-new owner |
| planner-driven routing semantics, compatibility, resolver, evidence, and rollout (`prd:planner-driven-model-routing/FR-7..FR-12`, `FR-15`, `FR-17`, `FR-20..FR-24`, `FR-27`) | policy / contract | all phases | approved parent contract; canonical append-first activation remains ancestry-gated | routing policy owner |
| superseding implement-now technical design | docs | Demand boundary + review integration | required after this amendment; must replace rung/hardcoded routing, define the obligation state machine and atomic green-boundary handoff, isolated review workspace/capacity, attribution primitive, three failure classes, EVT-10/EVT-15 independence, and authenticated exact-read recovery while preserving Worker/Dispatcher ownership | @mdarabi |
| e2e-plan governing-document reference + approved `e2e-design/v1` | data contract | e2e consumption | available through merged e2e authoring line; malformed/unapproved basis refuses under UB-14 | e2e-plan + e2e-design owners |
| impl-review amendment 003 and superseding design | approval / docs | review-role assurance and proposal-medium artifact production | available for band projection, internal pass/blindness/aggregation, artifact schema, and generic FR-6 proposal persistence only; the live sibling contract/design does not yet explicitly authorize its host adapter to perform FR-6 for the reviewer program or expose exactly-one publication returning a native receipt, so review integration remains blocked. Absence or ambiguity of performer authority, receipt-returning capability, or exactly-once semantics—and any proposed ownership or performer transfer—requires a sibling impl-review amendment and design before code; a transport API returning an ID is not contract evidence | impl-review + implement-now owners |
| self-drive-wake consumer amendment + superseding watch design (or feature-drive absorption with the same contract) | approval / docs | unattended review-only recovery | required before production activation; consumes EVT-16 `review_pending` and three failure counts, schedules bounded review-only re-invocations, preserves exhaustion as red/escalated, and never redispatches implementation | self-drive-wake / feature-drive owner |
| shared claim mechanism + automated-implementation tag | system | Worker integration | confirmed | plan-new / fleet convention |
| qualified current native model roster and cost metadata per harness implementation and review roles | vendor / policy | Native resolution + release | required; mappings without reviewed qualification fail | @mdarabi + runtime adapter owners |
| four agent CLIs plus configured reviewer service identities authenticated | vendor / identity | Worker + review integration | operator-provisioned; configured review preflight occurs once before avoidable implementation spend and failures emit run-level EVT-15 infrastructure/exit 1 without failing a task | operator |
| merge-lane | system | post-run | as-is; executor never lands proposals | merge-lane |

### op: supersede §6.3 Risks, assumptions, and rabbit holes

Reason: review forgery, revision drift, and shared reviewer workspaces are now explicit release risks with executable mitigations.

| # | Risk / assumption / rabbit hole | Likelihood | Impact | Mitigation or test | Owner |
|---|---|---|---|---|---|
| R1 | same-session charter bleed if state leaks across tasks | M | H | fresh implementation/reviewer sessions and isolated workspaces; UB-3/ABUSE-2 drills | design |
| R2 | unbounded loop/token runaway | M | H | single predicates; combined-idle gate; per-task/review/driver bounds | design |
| R3 | green is red or absent | M | H | proposal artifact verification, CI mirror, EVAL-3 | design |
| R4 | selected harness mapping goes stale | H | H | closed policy, qualification, EVAL-2, unsupported refusal, fallback | runtime adapter owners |
| R5 | tracker carrier mutates demand | M | H | field-identity conformance | plan projection + executor owners |
| R6 | legacy translation becomes inference | L | H | versioned table, conservative absent rung, unknown fail, byte identity | compatibility owner |
| R7 | direct activation exposes mapping defect | M | H | pre-release proof, fallback, UB-13/DOD-18 disposition | @mdarabi |
| R8 | routing becomes harness selection/review orchestration | M | H | NR-6; selected harness input; impl-review owns internal assurance process | @mdarabi + design reviewer |
| R9 | headless harness cannot cover autonomous run | M | M | design spike; no weakening/harness fallback | design |
| R10 *(005 ← 004)* | crash, concurrent invocation, mutable proposal head, forged/replayed artifact, worker-accessible evidence state, shared reviewer workspace, or driver restart loses/corrupts/duplicates review truth | M | H | exact-green obligation capture; fresh attempt attribution after implementation termination; impl-review-owned single-write broker; canonical store outside child write surfaces; isolated bounded reviewer workspace; broker-receipt+proof verification; three failure classes; EVT-15/EVT-16; UB-20/UB-21 kill/restart/moving-head/forgery/store-mutation/replay/concurrency/exhaustion drills | executor obligation, runtime, persistence, and driver-consumer owners |

### op: supersede §6.4 Release criteria

Reason: release must require authenticated review evidence, isolated execution, and every live replacement DoD/event.

| Lens | Criterion | Evaluated by |
|---|---|---|
| Functionality | DOD-31 and every live Must fit criterion green; every configured green proposal captures one exact-green obligation before EVT-10/release and reaches authenticated verified or typed pending review evidence | CI + compatibility suite + DOD-27/DOD-28/DOD-31 reports |
| Routing conformance | every supported implementation/review cell resolves qualified least-cost/unsupported; review preflight occurs before avoidable task spend; resolved launches match events | EVAL-2 + DOD-11 rubric + DOD-28 matrix |
| Reliability | DOD-30 UB drills; no claim leak, head drift, duplicate attempt, pending laundering, implementation redispatch, or class/exit mismatch | failure injection + EVT-10/EVT-15/EVT-16 joins + driver report |
| Security | DOD-29; no merge credential, parent/cross-workspace mutation, child-accessible canonical-state write capability, attribution-secret exposure, worker-prewritten/direct-published/receipt-or-state-substituted/replayed accepted artifact, unqualified mapping, or raw argument | abuse matrix + capability/secret scan + security review |
| Compatibility | DOD-14 v1 translation/byte identity; plan/v2 carrier identity | corpus and round-trip conformance |
| Dogfood | one real feature uses three verified EVT-10 profiles; every configured review is authenticated exact-revision verified or three-way pending/run-red; one killed review recovers review-only and forged evidence is rejected | DOD-12, DOD-27..DOD-29, DOD-32 reports |
| Rollout | direct cutover/fallback pass; sweep-end derivation, parent/shared reviewer, unauthenticated acceptance, duplicate attempt, and pending laundering are unreachable | DOD-13/DOD-18/DOD-27..DOD-29/DOD-32 drills + inspection |

### op: supersede §6.5 Rollout and reversibility

Reason: cutover and rollback must preserve authenticated evidence and remove the specific unsafe review paths found in #1034.

- **Flag** — one production routing/review switch, owned by @mdarabi, activates only after DOD-27..DOD-33 and the driver consumer dependency are green.
- **Kill switch** — strongest qualified profile within the selected harness/role; never changes harness, weakens demand, bypasses attribution, or clears pending review.
- **Experiment** — none; no shadow/canary production decision.
- **Cutover** — validate policy/auth, exact-green obligation persistence, canonical-state denial to child processes, isolated reviewer workspaces/capacity, fresh attribution, exactly-one impl-review-owned broker publish, broker-receipt/attempt-proof verification independent of code-host author, driver consumption, three failure classes, moving-head/forgery/store-mutation/replay/secret-leak/kill/retry/exhaustion drills, and fallback; then activate all supported paths in one switch. Remove sweep-end revision lookup, parent-clone review, unauthenticated acceptance, and invocation-local review only after successful activation.
- **Rollback** — engage fallback and stop affected new claims; preserve frozen demand, EVT-10, exact-green obligations, non-secret proof references, EVT-15 attempts, and pending review. Destroy raw attribution material at settle. A failed/untrusted review remains pending/escalated/run-red and reconciles review-only.

| Stage | Audience | Minimum evidence | Advance when | Roll back when |
|---|---|---|---|---|
| Pre-release only | conformance fixtures + repository dogfood | every live blocking DoD row (DOD-5, DOD-8..DOD-9, DOD-11..DOD-14, DOD-18..DOD-19, DOD-27..DOD-33), §6.4 report, parent seam gate, driver proof | M1–M7 and all guardrails green | any failed criterion blocks activation |
| 100% production | all supported implementation/review/driver paths | policy/obligation/proof snapshots + cutover/fallback/moving-head/forgery/replay/kill-retry rehearsals | post-launch M4–M7 and guardrails remain complete | downgrade, switch, evidence break, unauthentic acceptance, workspace mutation, duplicate, or pending laundering engages fallback and exact-revision review-only reconciliation |

### op: supersede §7.2 Open questions

Reason: the design gate must resolve the new obligation, attribution, isolation, and retry mechanisms under the closed behavioral constraints.

| # | Question | Owner | Needed by | Status |
|---|---|---|---|---|
| Q-1 | Does Claude Code's headless mode fully cover an autonomous multi-tool run under the native profile/effort settings its adapter must expose? | superseding tech design | design gate | open — must not weaken or switch harness if unsupported |
| Q-2 | The concrete worker turn, CI self-heal, implementation width, review capacity, and retry caps and their config homes. | superseding tech design | design gate | open — review capacity is separate and bounded; neither model counts its own budget |
| Q-3 | Dependency ordering under never-merge: pause/reinvoke versus another non-stacking mechanism. | superseding tech design | design gate | open — never-merge remains fixed |
| Q-4 | How the invoker chooses the explicit/ambient implementation harness and resolves an explicit or inherited review backend without installed-CLI inference. | superseding tech design | design gate | open — selection is upstream of FR-23/FR-44/FR-45 and cannot be changed by resolution |
| Q-5 | Attended live progress versus unattended journal rendering over the same EVT-10/EVT-15/EVT-16 stream. | superseding tech design | design gate | open — behavior and evidence semantics are identical |
| Q-6 | Which medium-neutral durable store and atomic transition implement exact-green obligation capture, per-task release, concurrent deduplication, and crash recovery? | superseding tech design | design gate | open — revision is captured once; no later head read may alter it |
| Q-7 | Which attribution primitive authenticates the impl-review-owned broker's native receipt plus attempt/proposal/revision proof without exposing raw material or canonical-state write capability to either child role? Before that primitive can be chosen, the live sibling contract and design must explicitly authorize the host-side impl-review persistence adapter to perform `prd:impl-review/FR-6` for the reviewer program and expose exactly-one publication returning the native receipt; implement-now supplies binding and consumes the receipt but cannot transfer or duplicate that write. Code-host author identity is explicitly outside the predicate. | impl-review sibling amendment/design + superseding implement-now design | sibling contract gate → design gate | blocked — absent or ambiguous performer authority, receipt-returning capability, or exactly-once semantics, and any ownership or performer transfer, requires a sibling impl-review amendment and design before code; only then may design choose the proof primitive |
| Q-8 | Which workspace/capability adapter gives every reviewer an isolated exact-revision disposable workspace, blocks parent/cross-workspace/repo-code mutation, and still permits only proposal-artifact persistence? | superseding tech design | design gate | open — hardware separation remains out, but shared parent state is forbidden |
| Q-9 | Which driver owns bounded review-only retry and how does it consume pending/three-class EVT-16 without treating exit 1 as implementation work? | stacked self-drive-wake amendment 004 + superseding watch design | consumer design gate | answered at contract — consumer-owned git recovery ledger, one closed selected-frontier invocation per durable round, immutable exhaustion/rearm; design must realize it before activation |

*None is product-decision-blocking: each selects a mechanism behind FR-21..FR-27 and FR-35/FR-40..FR-47. A mechanism that re-derives revision, exposes attribution material to implementation, accepts unauthentic evidence, shares mutable reviewer state, consumes implementation width, widens a selected recovery frontier, or launders pending review requires a new amendment rather than a design choice.*

### op: supersede §8 Contract readiness — the executability gate

Reason: readiness must name authenticated attribution, reviewer isolation, the third failure class, all replacement IDs, and the unresolved sibling broker-capability dependency.

| Check | Pass? | Notes |
|---|---|---|
| Problem basis accepted — §2 problem gate signed (T3) | ☐ | base gate remains pending the introducing review; amendments do not rewrite history |
| Zero solution-blocking TBDs or open questions (§7.2) | ☐ | Q-7 is blocked until a sibling impl-review amendment and design explicitly authorize host-adapter FR-6 performance plus exactly-one receipt-returning publication; only after that are attribution primitive, storage, workspace adapter, and bounds design mechanisms constrained by FR-40..FR-46 |
| Every live Must FR has fit criterion/verification | ☑ | FR-1..FR-5, FR-7..FR-10, FR-13..FR-18, FR-20..FR-32, FR-35, FR-40..FR-47 |
| Every requirement carries risk; high/critical rows have safety plan | ☑ | includes exact-green capture, fresh attribution, three failure classes, isolated bounded review, durable recovery |
| Non-goals/non-regressions named | ☑ | NG-1..NG-5; NR-1..NR-6, NR-8, NR-10; review isolation is workspace/capability confinement, not fleet hardware policy |
| Every primary job covered | ☑ | J1→FR-1/7/15/21..29/34/35/39..46; J2→FR-17/NR-4; J3→FR-7/8/9/25/30..32/40..42/46 |
| Every key flow has functional/failure/non-regression coverage | ☑ | FR live set, UB-1..UB-9/UB-11..UB-18/UB-20..UB-21, NR live set; moving-head, auth/factory, forgery/replay, isolated mutation/concurrency, three failure classes, interruption/retry/exhaustion explicit |
| Every primary metric instrumented | ☑ | M1–M6→EVT-4/EVT-7..EVT-10/EVT-16; M7→EVT-15/EVT-16 with attempt/receipt/proof refs and three-way pending denominator |
| Every blocking DoD traces to source/rubric | ☑ | DOD-5, DOD-8..DOD-9, DOD-11..DOD-14, DOD-18..DOD-19, DOD-27..DOD-33 |
| Risks/assumptions owned | ☑ | R1..R10 include lost/forged/drifted review, shared workspace, and driver laundering with named owners |
| Human-judgment criteria name owner/rubric | ☑ | DOD-11 names @mdarabi; security review evidence is mechanical plus named PR approval |
| DoD complete; n/a explained | ☑ | exact-green ordering, attribution, isolation, three-class events, driver recovery, live UB/Must/NR/security proofs separately blocking |

### op: supersede §0 Read to rule — the human reviewer's section (~6 min)

Reason: amendment 005 changes a human-routed security and evidence decision, so §0 is refreshed wholesale in the same contract-changing op set.

*The body is the contract; this section only routes attention — on any conflict the body governs; machines ignore it.*

**Judge these**

| # | The call | Could have gone | If you disagree |
|---|---|---|---|
| 1 | Opens PRs, **never merges** | Drive to merged | Merge authority collision → §3.4 NG-1, FR-7 |
| 2 | **Full tracker-mediated** projection/claims | Tracker-bypass local loop | Second projection and broken idempotency → §4.1, FR-1/FR-2 |
| 3 | **Fresh session per task** | Warm reuse | Charter bleed → FR-5, UB-3 |
| 4 | Autonomous workers run on the same hardware under ambient policy, but implementation and review attempts each receive **separate isolated workspaces and capabilities** | Parent-clone/shared reviewer or fleet-grade hardware sandbox | Shared-state mutation is forbidden here; hardware separation remains fleet-owned → FR-4, FR-43, NG-2/NG-3 |
| 5 | Unrecoverable task failure **continues + escalates** | Halt run | Independent work continues → FR-15, UB-1 |
| 6 | Build the **durable worker core** | Throwaway script | Remote runtime reuses it → §4.1, G3 |
| 7 | **Bounded-parallel** implementation plus separately bounded review | One pool or sequential | Review never holds implementation capacity or races in one clone → FR-11, FR-40, FR-43 |
| 8 | Frozen semantic demand; selected harness maps least-cost satisfying profile | Hardcoded pin/inference/native plan data | Portable judgment and explicit egress → FR-21..FR-24, NR-6 |
| 9 | Plan/v1 conservative no-rewrite translation | Backfill/default cheap | Frozen judgment remains honest → FR-22, NR-5, UB-11 |
| 10 | Full pre-release proof then direct cutover/fallback | Shadow/canary | One auditable semantics → FR-26/FR-27, §6.5 |
| 11 | e2e origin from recorded plan reference | e2e type/prose inference | No schema fork or false classification → FR-28/FR-29, NR-8 |
| 12 | e2e worker writes test, never feature, honoring route | Bend behavior/route | Test keeps its meaning → FR-30/FR-31, UB-15/UB-16 |
| 13 | e2e test hermetic before green | Ordinary self-heal handles flake | Measurement is not a gate → FR-32, UB-17, AP-29 |
| 14 | Routing offline/inference-free; only canonical implementation evidence may network pre-claim after parent activation | Hidden lookup/buffer | No orphan route/hidden dispatch dependency → NFR-9, UB-18, §6.2 |
| 15 | Configured review is advisory to EVT-10 but durable and **authenticated to the launched reviewer invocation**: absent by default; setup/auth before task spend; assurance-band route; exact-green revision captured per task; implementation resources released; isolated reviewer child; impl-review-owned broker performs FR-6's proposal write; exact protected receipt+attempt-proof verification independent of code-host author; three failure classes; consumer-selected review-only recovery | Silent default install; factory/task coupling; weakest-profile shortcut; sweep-end head lookup; parent-clone review; child or implement-now direct publication; worker-writable receipt state; accept any matching comment; process-exit success; rerun implementation; unbounded/widened recovery | Independent review evidence cannot be forged, silently skipped, under-routed, drifted, or laundered without changing impl-review's publication contract → FR-35/FR-40..FR-47, UB-20/UB-21, EVT-10/EVT-15/EVT-16, DOD-27/DOD-28/DOD-32 |

**Sanity-check the proof** — routing remains semantic and inference-free. Configured-review proof now joins exact-green capture, green→persist→EVT-10→release→launch ordering, separate review capacity/workspace, fresh post-worker attribution, exactly one impl-review-owned broker publish, canonical obligation/attempt/receipt state unavailable to child writes, receipt plus attempt proof independent of code-host author, three failure classes/counts, bounded review-only recovery, and zero implementation redispatch (M7, DOD-27..DOD-29).

**The author's open questions** — harness selection, worker bounds, dependency ordering, progress rendering, the concrete obligation store, attribution primitive, workspace adapter, and driver retry number remain design/config choices. Publisher ownership is fixed, but the required broker seam is not assumed available: before code, a sibling impl-review amendment and design must explicitly authorize the host persistence adapter to perform FR-6 for the reviewer program and supply exactly-one publication returning a native receipt. Absent or ambiguous performer authority, receipt capability, or exactly-once semantics is a blocking dependency, not a primitive choice. FR-40..FR-45 prohibit choosing mechanisms that silently install review, re-derive the head, expose raw attribution or canonical-store writes to a child process, share the parent workspace, bypass receipt+attempt-proof authentication, or clear pending work without exact verification. A distinct code-host reviewer principal is explicitly deferred by WR-4 to a later amendment, not required by this one.

**Skip the rest** — structure/fold → docgate; substance → `/prd-review`; mechanism → superseding design; internal assurance process → impl-review sibling; merge → merge-lane; e2e authoring → e2e-design/e2e-review.
