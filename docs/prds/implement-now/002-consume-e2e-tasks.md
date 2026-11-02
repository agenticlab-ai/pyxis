# Amendment 002 — consume-e2e-tasks

| Field | Value |
|---|---|
| **Amends** | implement-now *(chain through 001)* |
| **Status** | Approved |
| **Owner** | @mdarabi |
| **Date** | 2026-07-28 |
| **Learned-from** | dependency-change:prd:e2e-verification/WR-1 · dependency-change:design:e2e-verification-skills#3.3 · dependency-change:design:e2e-verification-artifacts#3.1 · incident:I5 · review-thread:review:implement-now-002#FND-1 · review-thread:review:implement-now-002#FND-2 · review-thread:review:implement-now-002#FND-3 · review-thread:review:implement-now-002#FND-4 · operator-decision:2026-07-28 — land the task-consumption half of e2e-verification/WR-1 in implement-now, leaving systematic run-time anti-gaming and the harder app shapes deferred |
| **Blast radius** | tier: contract-changing; touches: FR-28..FR-32, NR-8, UB-14..UB-17, EVT-12, DOD-19, §0, §1, §3.4, §5.4, §5.5, §6.2, §8; reason: the executor gains a second task-consumption class — e2e-planned tasks recognized from a recorded plan reference, briefed from the feature's approved e2e design doc, and bounded to authoring a hermetic test rather than changing the behavior under test — adding Must requirements, fail-closed refusals, and a blocking DoD row |
| **Template version** | amendment-template/v2 |

## Operations

### op: add FR

Reason: the e2e authoring line emits ordinary tracker tasks by contract (`prd:e2e-verification/FR-14`, `NR-2`), so nothing on the task marks it as a test task and an unbriefed worker cannot tell one from feature work.

| ID | Requirement (EARS) | Rationale | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|
| FR-28 | When a ready task's originating plan names a feature's approved e2e design doc in the plan's own governing-document header, the executor shall establish that e2e origin before claim from that recorded reference — never by inference over charter prose — resolve the task to exactly one scenario or to the harness slot in that document, and carry the resolved basis into the attempt, adding no e2e-specific task type and no additional required charter field. | e2e tasks are ordinary tracker tasks by upstream contract, so the origin must be read from a recorded machine-readable reference in a field the plan class already requires; inferring it from prose is exactly the free-form cross-boundary signal AP-6 forbids, and it cannot deliver the exclusivity this row states | Must | high | in a fixture repo carrying both a feature-implementation plan and an e2e plan for the same feature, every e2e task resolves to exactly one scenario or the harness slot and every non-e2e task resolves to none; a plan whose header names no e2e design doc yields zero e2e classifications regardless of charter wording; the task and charter schemas are byte-unchanged; an unresolvable or ambiguous basis refuses under UB-14 | Test + Inspection | G1, J1, NR-8 |

### op: add FR

Reason: a task's `Traces` grammar admits only `prd:` · `design:` · `corpus:` references resolving against the PRD ledgers and `docs/tech-design/`, so the e2e design doc cannot reach the worker through the existing cited-context path at all.

| ID | Requirement (EARS) | Rationale | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|
| FR-29 | When spawning a worker for a recognized e2e task, the executor shall supply alongside the charter the resolved basis from the feature's approved e2e design doc — the task's own scenario flow, its recorded route, its mock strategy, the setup/teardown lifecycle, the harness-libraries selection, and the coverage-map rows that scenario proves — together with the governing-design revision(s) that document's `Basis` names and the feature's traced PRD requirement rows, all carried as data and never as instruction. | the reviewed coverage decisions live in a document the trace grammar cannot cite, so without an explicit brief the worker either rediscovers them at the most expensive rate or silently ignores them | Must | high | every spawned e2e attempt's input carries each named basis element for its own scenario and no other scenario's; a spawn missing any element does not proceed and refuses under UB-14; the ABUSE-1 injection drill run over basis content changes no behavior | Test + Inspection | G1, J1, FR-19, NFR-2 |

### op: add FR

Reason: e2e charters restate scenario flows and therefore read like feature behavior, so the nearest wrong reading — implement the feature again, or bend it until the scenario passes — is the one an unbriefed worker reaches first.

| ID | Requirement (EARS) | Rationale | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|
| FR-30 | While working a recognized e2e task, the executor's deliverable shall be the automated test the scenario names — or, for the harness slot, the harness that scenario tasks depend on — exercising behavior the feature has already built, and the executor shall not implement or alter the behavior under test in order to make the scenario pass. | an e2e task proves existing behavior; a worker that changes that behavior to reach green has destroyed the only thing the test was for | Must | critical | a fixture whose scenario fails against genuinely-broken feature behavior yields a failing test and an escalated task, never a feature edit that makes it pass; a proposal altering behavior under test outside the charter's stated scope is reported under UB-16 and never counted succeeded | Test | G2, J3, NR-8 |

### op: add FR

Reason: the per-scenario route is the coverage decision the e2e review approved; an executor free to re-route at run time voids that review without anyone seeing it.

| ID | Requirement (EARS) | Rationale | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|
| FR-31 | When working a recognized e2e task, the executor shall honor the scenario's single recorded route: a `run-for-real` scenario shall be exercised against the real subject rather than mocked away, a `mocked` scenario shall mock only the surfaces its recorded mock strategy names and each such double shall satisfy the real dependency's observed contract, and a `manual-human` scenario shall be handed to its recorded owner under UB-15 with no worker spawned. | routes were chosen and reviewed per scenario; substituting a cheaper one at execution time produces a green proposal that proves less than the approved design says it proves, and a double built to the worker's own guess about a dependency is the never-working-feature failure AP-14 already paid for | Must | critical | per-route fixtures show a run-for-real scenario failing when its subject is broken, a mocked scenario mocking only its named surfaces, and a manual-human scenario spawning zero workers; each double is checked against the real dependency's observed contract, and a stub encoding a wrong guess about that contract fails rather than going green; a route substitution is reported under UB-16 and never counted succeeded | Test | G2, J3, FR-30 |

### op: add FR

Reason: every e2e task adds a check that can fail a change, and `incident:I5` — tagged `implement-now` and `hermeticity` — already paid for a non-hermetic check that passed locally and stranded CI red, with the record noting a command-level mirror would not have caught it.

| ID | Requirement (EARS) | Rationale | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|
| FR-32 | Before counting a recognized e2e task succeeded, the executor shall establish that the test it authored is hermetic — a pure function of its declared inputs, yielding the same verdict on any machine and any run — and shall route a test that cannot be made hermetic to UB-17 rather than to the self-heal loop. | the executor's whole deliverable here is new blocking checks, and a check whose verdict depends on wall-clock, network, load, or ambient repository state is a measurement rather than a correctness claim; neither the e2e authoring line nor FR-8's CI mirror carries this duty | Must | high | each authored test returns the same verdict across repeated runs on differently-loaded machines before the task is counted succeeded; seeded wall-clock, network, ordering, and ambient-git fixtures are detected and routed to UB-17 with zero self-heal iterations consumed | Test | G2, J3, FR-7, FR-8 |

### op: add NR

Reason: the upstream line deliberately shaped e2e work as ordinary tasks so the executor could consume it unchanged; recognition must stay additive to the executor rather than becoming a schema demand on the planner.

| ID | Existing behavior protected | Why protected | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|
| NR-8 | e2e tasks reach the executor through the unchanged frontier, claim, task, and charter path; no e2e-specific task type, required charter field, or second projection path is introduced, and plan-new and plangate are not edited — FR-28's origin reference is carried in the governing-document header the plan class already requires of every plan. | the upstream contract makes e2e tasks ordinary tracker tasks precisely so the executor consumes them unchanged; an e2e-specific schema would fork the planner and the projection it already owns, and an already-required field carries the signal without one | critical | the task and charter schemas and plan-new's pipeline are byte-unchanged; the e2e origin reference occupies an existing required plan header field and adds no new one; an e2e plan's tasks pass the existing frontier and claim mechanisms unmodified; a frontier mixing e2e and feature-implementation tasks dispatches through one path | Test + Inspection | FR-1, FR-2, FR-28 |

### op: add UB

Reason: recognizing an e2e task the executor cannot then brief is worse than not recognizing it, so an unresolvable basis must stop the task rather than fall through to an ordinary spawn.

| ID | If… (trigger) | Then the system shall… | User sees | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|---|
| UB-14 | a task originates from an e2e plan but its feature has no e2e design doc, that document is not `approved`, it is malformed against its class checklist, or the task resolves to no scenario or to more than one | refuse the task before claim, name the feature, the expected document, and the defect, spawn no worker, and continue the other independent ready tasks | task unclaimed with a named e2e-basis error | Must | high | one fixture per missing, not-approved, malformed, unresolvable, and ambiguous class performs zero claims and zero model invocations and names the defect; independent valid tasks in the same run still complete | Test | FR-28, FR-29 |

### op: add UB

Reason: e2e-plan emits a task for every scenario including `manual-human` ones, so those tasks land on the executor's frontier and would otherwise be driven to an automated green that contradicts the approved route.

| ID | If… (trigger) | Then the system shall… | User sees | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|---|
| UB-15 | a recognized e2e task's scenario carries the `manual-human` route | spawn no worker, perform no claim and no model invocation, class the task as human-execution-required naming the route's recorded owner, and report it in the run's final classes | task listed as awaiting its named human, never as succeeded or failed | Must | high | a manual-human fixture spawns zero workers, consumes zero tokens, names the recorded owner, and is never counted succeeded; repeated runs report it identically without claim churn | Test | FR-31, NFR-3 |

### op: add UB

Reason: FR-7's green-checks test cannot see a proposal that reached green by changing the subject or swapping the route, so the mandate needs its own consequence row.

| ID | If… (trigger) | Then the system shall… | User sees | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|---|
| UB-16 | a recognized e2e task's proposal changes the feature behavior under test outside the charter's stated scope, or exercises a route other than the scenario's recorded one | withhold the succeeded class, mark the task failed naming the violated mandate or route, preserve its work, escalate, and continue the independent tasks | task in the failed class with the scenario, the recorded route, and the offending change named | Must | critical | seeded feature-edit and route-substitution fixtures are never counted succeeded even with every required check green; each violation names the scenario, the recorded route, and the offending change | Test + Inspection | FR-30, FR-31, FR-10 |

### op: add UB

Reason: a flaky test is red-then-green on identical input, so the ordinary self-heal loop reads it as a fixable failure and burns its whole bound converging on nothing — the loop `incident:I5` describes.

| ID | If… (trigger) | Then the system shall… | User sees | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|---|
| UB-17 | an authored e2e test's verdict varies across runs on identical input, or depends on wall-clock, network, load, ordering, or ambient repository state | stop that task before consuming further self-heal iterations, mark it failed naming the non-hermetic dependency, preserve its work, escalate, and never count it succeeded or wire it into required checks | task in the failed class with the non-hermetic dependency named | Must | high | seeded flaky and environment-dependent fixtures stop with the dependency named, consume no self-heal iterations beyond detection, and are never counted succeeded; a hermetic test in the same run is unaffected | Test + Failure injection | FR-32, FR-9, FR-13 |

### op: add EVT

Reason: DOD-19 and UB-14..UB-16 are only auditable if each ready task's e2e origin decision — including the refusals and the not-an-e2e-task outcome — is recorded before claim.

| ID | Event name | Fires when | Properties (typed) | Source | Feeds |
|---|---|---|---|---|---|
| EVT-12 | `e2e_task_basis_resolved` | a ready task's e2e origin is established or refused, before claim | `task_id:string`, `feature_id:string`, `plan_ref:string`, `basis_ref:string?`, `slot:enum{scenario,harness}?`, `scenario_id:string?`, `route:enum{run_for_real,mocked,manual_human}?`, `resolution:enum{resolved,refused,not_e2e}`, `reason:enum{missing_doc,not_approved,malformed,unresolvable,ambiguous}?` | executor | DOD-19 audit + M2 |

### op: add DOD

Reason: the consumption path is only proven by a real e2e plan running end to end — a fixture cannot show that a briefed worker writes a test that actually fails when its subject breaks.

| ID | Done criterion | Source | Verify | Required evidence | Blocking? |
|---|---|---|---|---|---|
| DOD-19 | One real pyxis feature's approved e2e plan runs through `/implement-now`: every task resolves to its scenario or the harness slot from the plan's recorded e2e reference, every automatable scenario reaches a genuinely-green proposal that fails when its subject is deliberately broken and returns the same verdict on repeat runs across differently-loaded machines, every `manual-human` scenario is handed to its named owner with zero worker spawns, and no proposal changes the behavior under test. | FR-28..FR-32, NR-8, UB-14..UB-17, EVT-12 | Demo + Test + Inspection | the e2e-plan run record, per-task basis resolution from EVT-12, proposal links, one broken-subject falsification result per automatable scenario, a repeat-run verdict-stability record per authored test, and a diff inspection showing no change to behavior under test | yes |

### op: supersede §0 Read to rule — the human reviewer's section (~6 min)

Reason: the human routing section must expose the executor's second task-consumption class and the calls that bound it, while remaining informative-only under AGENTS.md rule 9.

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

**Sanity-check the proof** — model routing is not a scalar complexity guess. The planner freezes six assessment axes and four independent semantic minima. `implement-now` proves field-identical transport, a complete harness×derivable-profile implementation-role matrix, at least three distinct native profiles in dogfood, no-rewrite v1 translation, and exact/promoted/unsupported evidence. The routing resolver itself spends zero inference (NFR-8). For the e2e class the proof is a real e2e plan run end to end, with one deliberately-broken-subject falsification and one repeat-run verdict-stability record per automatable scenario (DOD-19) — the cheapest evidence that a green e2e proposal is neither hollow nor flaky.

**The author's open questions** — whether Claude Code's headless mode fully covers an autonomous multi-tool run remains open (§7.2 Q-1). Harness *selection* mechanics remain design-owned (§7.2 Q-4); demand resolution never answers that question or scans for another harness. *How* the executor joins a recorded e2e reference to the individual scenario is likewise design-owned — FR-28 fixes the signal and the outcome and NR-8 fixes the constraint, not the join mechanism. The other open rows are implementation mechanics behind settled requirements.

**Skip the rest** — structure/ID/fold correctness → docgate; substance and coherence → `/prd-review`; implementation mechanisms → the superseding technical design; review model/process routing → the impl-review sibling amendment; merge safety → merge-lane; plan judgment and projection → plan-new; what an e2e scenario, route, or mock strategy should be → e2e-design and e2e-review, already settled by the doc's approval.

### op: supersede §1 Summary

Reason: the summary must state the executor's second consumption class, since a reader who stops at §1 would otherwise not learn that e2e-planned tasks are handled at all.

- **Problem** — Once a plan is frozen and projected, pyxis can execute it locally, but the implementation route is still coupled to an optional coarse rung and hardcoded/implicit native settings; plan/v2's richer demand can be lost or reinterpreted before launch, legacy plans have no explicit fail-safe translation contract, and the e2e authoring line now projects test tasks the executor cannot tell apart from feature work or brief from the design that justified them.
- **For whom** — The operator with a pyxis-installed repo who has just planned in-session (attended), and argus driving the same core unattended.
- **Why now** — Plan-new now freezes complete semantic demand on every plan/v2 task and the parent routing contract requires `/implement-now` to consume it through harness-specific native mappings before production activation; separately, the e2e authoring line ships and its plans are projected onto the same frontier with no executor that understands them.
- **Direction** — Preserve the existing single-task worker and upstream harness choice. Before claim, recover plan/v2 demand field-identically or translate plan/v1 conservatively; resolve the implementation profile through the selected harness adapter to the least-cost qualified satisfying native profile; read whether the originating plan records an approved e2e design doc and, if so, resolve and carry its scenario basis; fail unsupported, invalid, and unresolvable cases without claim or invocation; then run the unchanged claim → workspace → agent → genuinely-green proposal path and retain a connected evidence chain.
- **Appetite** — The native local execution core, the complete implementation-routing seam and dogfood proof, and the e2e task-consumption half — recognition, briefing, mandate, route fidelity, and hermeticity; excludes harness selection policy, review-process routing, the remote runtime, hardware separation, umbrella policies, landing, and the systematic e2e anti-gaming and hard-app-shape work that stays in the waiting room.
- **Success** — `/implement-now` implements a real feature with every plan demand preserved, every routing cell conformance-proven, at least three native profiles each reaching a succeeded verified outcome, every other resolved attempt retained as a genuinely-green or bounded-failure outcome, and one real e2e plan run to green tests that fail when their subjects break and hold their verdict across machines.
- **Guardrail** — It never merges, mutates the parent checkout, double-claims, downgrades semantic demand, switches harnesses, rewrites plan/v1, launches under invalid/unsupported routing, changes the behavior an e2e task exists to prove, substitutes a scenario's approved route, or counts a non-hermetic check green.
- **Top risks** — stale/unqualified native mappings; tracker transport dropping demand; legacy translation becoming mutable inference; direct activation exposing a mapping defect; an e2e worker re-implementing or bending the feature it was meant to test; a flaky authored check burning the self-heal loop and stranding CI red (`incident:I5`); the existing charter-bleed, unbounded-loop, and false-green failures.

### op: supersede §3.4 Non-goals and waiting room

Reason: without an explicit waiting-room row, absorbing the e2e task-consumption half reads as absorbing all of `prd:e2e-verification/WR-1`, including work this amendment deliberately does not take.

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

### op: supersede §5.4 Data

Reason: the resolved e2e basis and its EVT-12 origin decision are the only evidence-bearing entities this contract consumes with no data row, leaving the brief the worker acts on without a stated source of truth, retention, or integrity rule.

| Entity / field | Meaning | Source of truth | Retention | Integrity rules | Privacy class |
|---|---|---|---|---|---|
| plan/v2 execution demand | frozen assessment, citations, semantic profile, derivation version, raise evidence | frozen plan; projected copy is transport | with plan | field-identical through executor boundary; immutable; no native identifiers | internal |
| plan/v1 legacy origin | original rung presence/value plus compatibility version and translated profile | immutable source plan + typed EVT-9 | forever with routing evidence | source never rewritten; explicit or `legacy_unassessed`; unknown fails | internal |
| e2e scenario basis | the scenario flow, route, mock strategy, setup/teardown, harness selection, and coverage rows a recognized e2e task must satisfy | the feature's approved `e2e-design/v1` document, at the revision the plan's recorded reference names | with the feature's e2e artifacts | read-only to the executor and never edited by it (NG-5); resolved from the recorded reference, never inferred; exactly one scenario or the harness slot per task; a non-approved, malformed, or ambiguous basis refuses under UB-14 | internal |
| native routing policy | qualified harness×implementation-role mappings plus authorized repo overrides | released defaults + validated repo policy | current + repository history | closed/version-compatible; qualification evidence; no weakening/raw arguments | internal |
| routing attempt | selected harness/role, requested profile, policy version, exact/promoted/unsupported result, native identity if resolved | immutable EVT-7 or EVT-8 | forever in git evidence | exactly one terminal per attempt; connected to one origin | internal |
| e2e origin decision | the resolved, refused, or not-e2e outcome of a ready task's origin check, with its basis reference, slot, and route | immutable EVT-12 | forever in git evidence | append-only; exactly one per ready task per run; a refusal names its reason and precedes any claim | internal |
| worker outcome | verified/failed/blocked terminal and usage for a resolved attempt | immutable EVT-10 + proposal evidence | forever in git evidence | exactly one per resolved attempt; absent for unsupported pre-claim attempts | internal |
| evidence invalidation | affected implementation attempts/artifacts and `rerun_required` or `operator_remediation` disposition | immutable EVT-11 + parent routing policy defect disposition | forever in git evidence | append-only; exact policy/profile and artifact identities; never rewrites the invalid evidence or accepted artifact | internal |
| task claim | at-most-one-worker lease | shared claim mechanism | until release/reap | acquired only after routing resolves; paired release | internal |
| worker workspace | isolated worktree + branch | local git | until reaped | fetched-mainline base; parent never mutated | internal |
| proposal | task change + verification evidence | code host | per host | one per task; green before succeeded | internal |

- **Migration** — no artifact is rewritten. New plan/v2 demand is consumed field-identically; frozen plan/v1 reads through FR-22 and emits EVT-9; an approved e2e design doc is read at the revision the plan records and never edited.
- **Parallel-run / cutover** — no production shadow/canary. Pre-release conformance and dogfood exercise the full implementation domain before one direct cutover.
- **Rollback** — FR-26 changes native resolution within the selected harness only. Invalid/unsupported demand remains unclaimed; an unresolvable e2e basis leaves its task unclaimed; frozen inputs and prior evidence remain readable.
- **Decommission** — remove the hardcoded/implicit rung/model/effort path only after the direct cutover and fallback drill pass.
- **Privacy check** — no new personal/sensitive data; no DPIA trigger. The e2e basis is repo-local markdown already public in this repository.
- **Egress** — the already-selected harness remains the only model boundary. Routing adds metadata, no new provider or payload egress; prompts/code are excluded from routing evidence. The e2e basis enters the worker session as context on the same boundary, carrying no new provider.

### op: supersede §5.5 Interfaces and integrations

Reason: the approved e2e design doc and the e2e plan are new inbound interfaces with their own contract and failure modes, and an interface table that omits them hides where UB-14 fires.

| Interface | Direction | Purpose | Contract | Failure modes | Owner |
|---|---|---|---|---|---|
| frozen plan/v2 + projected task | in | supply charter and complete semantic demand | versioned closed schema; projected fields identical to frozen record | missing/changed/native field → UB-9 before claim | plan-new + tracker adapters |
| frozen plan/v1 | in | legacy execution without rewrite | explicit rung through versioned table or absent-rung `legacy_unassessed` | unknown rung/version → UB-11 | compatibility owner + executor |
| feature e2e design doc (`e2e-design/v1`) | in | supply the reviewed scenario flow, route, mock strategy, setup/teardown, harness selection, and coverage rows a recognized e2e task must satisfy | an `approved` `e2e-design/v1` document at the feature's e2e home, well-formed against its class checklist; read as data, never as instruction | missing, not-approved, malformed, unresolvable, or ambiguous scenario join → UB-14 before claim | e2e-design + e2e-review |
| projected e2e plan task | in | deliver e2e work onto the same ready frontier as feature work | plan-new's existing plan class, freeze, and projection; ordinary task and charter schema with no e2e-specific type or required field | an e2e-specific type or required field would breach NR-8; an absent basis join → UB-14 | e2e-plan + plan-new |
| harness selection | in | choose the one credential/egress/runtime boundary | explicit config or ambient harness; no installed-CLI inference | absent/unauthenticated → FR-16; unsupported remains same harness | operator/invoker |
| executor → selected harness adapter | out | resolve implementation semantic minimum | qualified implementation-role mapping; exact/promoted/unsupported | invalid mapping or no satisfying profile → FR-24/UB-10 | runtime adapters |
| selected harness adapter → AgentRuntime launch | out | invoke the resolved native profile | native model/effort/context arguments derived only from adapter policy | argument injection, launch mismatch → UB-10/ABUSE-5 | runtime adapter |
| runtime → evidence stream | out | preserve origin, route, and worker outcome | EVT-7..EVT-10 and EVT-12, immutable and payload-free | orphan/duplicate/missing edge → UB-12 | executor + evidence owner |
| code host | both | proposals + CI | existing proposal/check surface | lag bounded; malformed is not green | this feature |
| merge-lane | out | landing downstream | opens proposals; never merges | no landing behavior owned here | merge-lane |

### op: supersede §6.2 Dependencies

Reason: FR-28 can only read an origin reference some upstream actor is bound to write, and FR-29's brief can only be assembled from an approved e2e design doc — neither obligation appears in the dependency table that exists to name exactly these preconditions.

| Dependency | Type | Needed by | Status | Owner |
|---|---|---|---|---|
| plan-new plan/v2 demand and checkpoint contract (`prd:plan-new/FR-28..FR-30`) | approval / data contract | Demand boundary | available through merged amendment 004 | plan-new owner |
| planner-driven routing semantics, compatibility, resolver, evidence, and rollout (`prd:planner-driven-model-routing/FR-7..FR-12`, `FR-15`, `FR-17`, `FR-20..FR-24`, `FR-27`) | policy / contract | all phases | available through merged amendments | routing policy owner |
| superseding implement-now technical design | docs | Demand boundary | required after this amendment; must replace `tiermap(rung)` and hardcoded/implicit launch selection without changing Worker/Dispatcher ownership | @mdarabi |
| e2e-plan records the feature's approved e2e design doc as the plan's governing-document header reference | data contract | e2e consumption | required before FR-28 can resolve any origin; uses the plan class's existing required header field and needs no plan-new or plangate change | e2e-plan owner |
| approved `e2e-design/v1` documents at their per-feature home (`prd:e2e-verification/FR-1`, `FR-4`, `FR-5`) | data contract | e2e consumption | available through the merged e2e authoring line; a non-approved or malformed document refuses under UB-14 | e2e-design + e2e-review owners |
| impl-review sibling amendment and superseding design | approval / docs | parent direct-cutover gate | required for full production activation; not owned by implement-now | impl-review owner |
| shared claim mechanism + automated-implementation tag | system | Worker integration | confirmed | plan-new / fleet convention |
| qualified current native model roster and cost metadata per harness implementation role | vendor / policy | Native resolution + release | required; mappings without reviewed qualification fail | @mdarabi + runtime adapter owners |
| four agent CLIs installed/authenticated | vendor | Worker integration | operator-provisioned | operator |
| merge-lane | system | post-run | as-is; review floor changes land through impl-review sibling | merge-lane |

### op: supersede §8 Contract readiness — the executability gate

Reason: the readiness enumerations name explicit ID ranges that this amendment's new requirement, non-regression, unwanted-behavior, and DoD rows fall outside of.

| Check | Pass? | Notes |
|---|---|---|
| Problem basis accepted — §2 problem gate signed (T3) | ☐ | base gate remains pending the introducing review; amendments do not rewrite its historical status |
| Zero solution-blocking TBDs or open questions (§7.2) | ☑ | Q-1..Q-5 are design mechanisms constrained by live requirements; unsupported cannot weaken or switch harness; e2e-origin derivation is likewise a mechanism behind FR-28 and NR-8 |
| Every live Must FR has a fit criterion and verification method (§5.1) | ☑ | FR-1..FR-5, FR-7..FR-10, FR-12..FR-18, FR-20..FR-32 |
| Every requirement row carries risk; high/critical rows have a safety plan | ☑ | pre-claim validation, no-rewrite translation, exhaustive conformance, selected-harness refusal, immutable evidence, direct-cutover/fallback drill, e2e basis refusal, mandate/route consequence rows, and hermeticity before green |
| Non-goals drawn; non-regression rows named | ☑ | NG-1..NG-5; NR-1..NR-8; routing does not absorb harness selection or review orchestration, and execution does not absorb e2e coverage authoring |
| Every primary job covered | ☑ | J1→FR-1/7/15/21..27, FR-28/29; J2→FR-17/NR-4; J3→FR-7/8/9/25, FR-30/31/32 |
| Every key flow has functional, failure, and non-regression coverage | ☑ | F1→FR-1..FR-32, UB-1..UB-17, NR-1..NR-8; unsupported, legacy, post-cutover defect, e2e-basis, and non-hermetic states are explicit |
| Every primary metric has instrumentation or plan | ☑ | M1–M6→EVT-4, EVT-6..EVT-10 with complete origin/terminal/outcome denominators; EVT-12 carries the e2e origin decision |
| Every blocking DoD row traces to source/rubric | ☑ | DOD-5, DOD-8..DOD-19; DOD-11 names @mdarabi and complete-cell rubric; DOD-15..DOD-18 cover all consequence and post-defect rows; DOD-19 covers the e2e consumption path |
| Risks and assumptions have owners | ☑ | R1..R9 owned, including mapping staleness, transport, legacy, cutover, and ownership creep; the e2e re-implementation risk is bounded by FR-30/FR-31 and UB-16, and the flaky-check risk `incident:I5` paid for by FR-32 and UB-17 |
| Human-judgment criteria name owner and rubric | ☑ | DOD-11 names @mdarabi and its cell qualification rubric; UB-15 routes manual-human scenarios to the owner the approved design doc records; other rows are mechanical/demo/analysis |
| DoD complete; n/a explained | ☑ | routing domain, legacy compatibility, successfully verified dogfood diversity, all consequence rows, direct cutover, fallback, post-cutover defect disposition, and the real-e2e-plan run are separately blocking |
