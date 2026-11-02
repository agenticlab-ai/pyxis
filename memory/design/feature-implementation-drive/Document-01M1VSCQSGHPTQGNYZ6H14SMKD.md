# Feature implementation drive — technical design

| Field | Value |
|---|---|
| **Status** | Approved — operator sign-off 2026-07-25 (doc-review session; zero annotations) *(merge is the freezing event; "superseded" is computed by the index, never written here)* |
| **Owner** | @mdarabi |
| **Approver(s)** | @mdarabi (operator — factory-control, identity, execution-routing, and delivery-evidence surface) |
| **Date** | 2026-07-25 |
| **Relevant-when** | designing or changing feature-scoped implementation ownership, local/remote execution routing, DAG-to-feature-branch integration, candidate-merge e2e evidence, repair loops, cancellation/handoff, or verified final delivery; implementing the feature-drive core or its Git/GitHub adapters; triaging split ownership, premature DAG advancement, stale candidate authorization, or false delivery; superseding this shape |
| **Scope** | The one-repository feature integration drive: its durable aggregate and reconciler; exclusive owner epochs and routing; task landing/DAG progression; deterministic candidate construction and evidence; owner-routed wake/resume; cancellation, handoff, repair, final review, delivery verification, and cleanup ordering. **Not**: executor Worker internals, automatic merge, e2e scenario authoring, generic standalone issue execution, multi-repository DAGs, fleet-wide scheduling, or retention-duration values. |
| **Tracking** | — *(the introducing PR)* |
| **Builds-on** | docs/tech-design/implement-now.md · docs/tech-design/merge-lane-v1.md · docs/tech-design/fleet-bridge-v1.md · docs/tech-design/wake-channel-v1.md · docs/tech-design/plan-new-v1.md |
| **Supersedes** | — |
| **Serves** | `prd:feature-implementation-drive/FR-1` · `prd:feature-implementation-drive/FR-2` · `prd:feature-implementation-drive/FR-3` · `prd:feature-implementation-drive/FR-4` · `prd:feature-implementation-drive/FR-5` · `prd:feature-implementation-drive/FR-6` · `prd:feature-implementation-drive/FR-7` · `prd:feature-implementation-drive/FR-8` · `prd:feature-implementation-drive/FR-9` · `prd:feature-implementation-drive/FR-10` · `prd:feature-implementation-drive/FR-11` · `prd:feature-implementation-drive/FR-12` · `prd:feature-implementation-drive/FR-13` · `prd:feature-implementation-drive/FR-14` · `prd:feature-implementation-drive/FR-15` · `prd:feature-implementation-drive/FR-16` · `prd:feature-implementation-drive/FR-17` · `prd:feature-implementation-drive/FR-18` · `prd:feature-implementation-drive/FR-19` · `prd:feature-implementation-drive/FR-20` · `prd:feature-implementation-drive/FR-21` · `prd:feature-implementation-drive/FR-22` · `prd:feature-implementation-drive/FR-23` · `prd:feature-implementation-drive/FR-24` · `prd:feature-implementation-drive/FR-25` · `prd:feature-implementation-drive/FR-26` · `prd:feature-implementation-drive/FR-27` · `prd:feature-implementation-drive/FR-28` · `prd:feature-implementation-drive/FR-29` · `prd:feature-implementation-drive/FR-30` · `prd:feature-implementation-drive/NR-1` · `prd:feature-implementation-drive/NR-2` · `prd:feature-implementation-drive/NR-3` · `prd:feature-implementation-drive/NR-4` · `prd:feature-implementation-drive/NR-5` · `prd:feature-implementation-drive/NFR-1` · `prd:feature-implementation-drive/NFR-2` · `prd:feature-implementation-drive/NFR-3` · `prd:feature-implementation-drive/NFR-4` · `prd:feature-implementation-drive/NFR-5` · `prd:feature-implementation-drive/NFR-6` · `prd:feature-implementation-drive/NFR-7` · `prd:feature-implementation-drive/NFR-8` |
| **Security/privacy impact** | yes — durable machine/domain identity, protected coordination refs, execution-routing authority, code-host credentials, candidate publication, and e2e runner/actor evidence; no new user-data class · reviewer: @mdarabi |
| **Template version** | tech-design-template/v1 |

## 0. Read to rule — the human reviewer's section (~10 min)

*The body is the contract; this section only routes attention — on any conflict the body governs; machines ignore it.*

**Judge these**

| # | The call | Could have gone | If you disagree |
|---|---|---|---|
| 1 | One versioned feature aggregate plus deterministic reconciler is the lifecycle authority | Event-sourced journal (`#ALT-2`) or platform-derived coordinator (`#ALT-3`) | Veto the state and idempotency boundary → §2, §3.1–§3.3 |
| 2 | Exclusive ownership is fenced by a CAS record and monotonically increasing epoch | Coordination issue (`#ALT-5`) or central service (`#ALT-6`) | Require a different cross-machine consistency/identity contract → §3.4 |
| 3 | Task completion requires verified landing on the feature branch before an explicit tracker transition | Badge/default-branch closure semantics | Accept premature DAG advancement risk or replace the goal-object proof → §3.6, `#DINV-7` |
| 4 | E2e targets one canonical synthetic candidate on an immutable content-addressed ref | Code-host test merge (`#ALT-8`) or per-runner reconstruction (`#ALT-9`) | Replace candidate reproducibility and stale-evidence semantics → §3.7–§3.8 |
| 5 | Owner-routed instruction-free touches animate one reconciler in local and remote modes | Separate local/remote workflows | Accept behavior drift or replace the wake/backstop composition → §3.10 |
| 6 | Cancellation preserves; delivery releases only after main ancestry proof | Immediate release/cleanup or completion on merged badge | Weaken artifact safety or goal-object delivery → §3.11, `#DINV-12`–`#DINV-14` |

**Sanity-check the numbers** — one owner epoch is current; zero external effects occur before their intent is CAS-persisted; zero dependents advance without feature-branch ancestry; every required scenario has one accepted current candidate-bound result; 100 race repetitions must produce zero overlapping owners (`#DINV-1`–`#DINV-14`, A-1–A-10).

**The author's open questions** — Q1: repository retention durations remain operator/config policy; ordering and confirmation are fixed (§7 Q1).

**Skip the rest** — schema and citations → docgate; substance/coherence/reuse → `tech-design-review`; implementation defects and race fixtures → codegate plus A-1–A-12.

## 1. Problem

`implement-now` deliberately stops at a green task proposal. It walks one ready frontier, keeps every Worker feature-blind, never merges, and leaves landing and re-invocation to its driver. That is the right reusable execution primitive, but it cannot provide the feature-level promise in `prd:feature-implementation-drive`: one human-selected local or remote execution owner must carry a frozen task DAG through individually reviewed integration, terminal e2e against current main, repair, and one verified delivery.

The missing authority is not another Worker or a shared worktree. It is a durable lifecycle above the executor. Task claims serialize one currently ready issue; they cannot reserve future tasks to a machine, express a quiescing handoff, remember that evidence became stale, or distinguish cancellation from destructive cleanup. GitHub's default-branch closing behavior cannot prove a task landed on a non-default feature branch, and a passing feature head cannot prove integration with a concurrently advancing main. Without one explicit contract, local and remote drivers reconstruct intent differently, dependents can unlock from a badge, and a final PR can be raised from evidence for the wrong pair of heads.

This is the cheapest moment to decide the feature-drive shape: the Worker, task lease, merge lane, wake channel, and plan projection seams already exist; the e2e line has fixed the scenario contract while leaving execution to this follow-on. The design composes those seams and creates only the missing feature aggregate, ownership store, landing transition, and candidate/evidence contracts.

## 2. Decision

**Decision:** One versioned, medium-neutral `FeatureDrive` aggregate is the durable authority for a feature's integration lifecycle, and one deterministic reconciler is its only behavior. Short-lived local commands and remote wake workers load the same aggregate, verify external goal objects, plan typed idempotent effects, persist each intent by compare-and-swap, execute through ports, and reconcile the observed result; the feature-blind executor remains an effect target, never the lifecycle owner (`design-brainstorm:feature-implementation-drive-tech-design#D-1`).

Ownership is part of that aggregate and is fenced by a monotonically increasing epoch. The first binding stores each aggregate as closed versioned data on a dedicated remote Git ref; CAS elects one transition, and every routing or implementation effect presents the current epoch. Task proposals target one remote feature branch and advance the DAG only after their landed revision is verified there. Once all work lands, one canonical synthetic merge of exact feature and main heads is published by immutable identity; every automated result and manual attestation binds to it. Accepted current evidence authorizes one final human-reviewed PR, never a merge. Delivery is complete only after the delivered revision is verified on main (`design-brainstorm:feature-implementation-drive-tech-design#D-2`, `#D-3`, `#D-4`, `#D-5`, `#D-6`).

The core is a hexagon. It owns state transitions and predicates; adapters own Git, tracker, code-host, executor, e2e, identity, and notification mechanics. Medium callbacks carry hints only. All authority is re-derived from the aggregate, current epoch, and observed goal objects at reconciliation time.

## 3. The design

### 3.1 The aggregate and state model

**Decision:** `feature-drive/v1` is a closed, additive schema. Unknown fields, unknown enum values, a missing basis fingerprint, or an impossible state combination refuse before any effect. Its logical fields are:

| Block | Required content |
|---|---|
| Identity | schema version; repository identity; feature/plan identity and frozen-plan fingerprint; target-main ref; feature-integration ref |
| Revision | aggregate version/CAS token; created transition; last successful transition |
| Ownership | state (`unowned`, `active`, `quiescing`, `released`); domain (`local`, `remote`, or absent); stable owner id; fencing epoch; transition evidence |
| Phase | `integrating`, `candidate`, `verifying`, `repairing`, `final-review`, `delivery-verifying`, `cancel-pending`, `cancelled`, `abandoned`, `delivered`, or `blocked` plus a typed reason |
| Tasks | frozen task ids and dependency edges; proposal/head/base; review state; landed revision/evidence; repair lineage |
| Candidate | feature head; main head; merge-policy version; candidate commit/tree/ref; scenario-set fingerprint; evidence verdict |
| Delivery | final proposal identity/head/base; accepted candidate identity; main ancestry evidence |
| Effects | zero or more typed pending effects with idempotency key, required owner epoch, status, attempts, and observed goal-object evidence |

The aggregate stores the minimum durable facts needed to resume without model memory. Branches, proposals, checks, and tracker records remain authoritative for their own facts; their identities and last verified evidence are joined here so a cold diagnostic can state owner, phase, next action, and evidence currency. A cached observation never authorizes an effect: reconciliation re-reads its goal object.

The state machine separates ownership from phase so cancellation and handoff cannot be encoded as accidental phase names. `cancelled` and `abandoned` have no owner and no executable routing; `delivered` is terminal. A `blocked` reason is closed (`ownership-conflict`, `landing-verification`, `candidate-conflict`, `verification`, `delivery-verification`, `malformed-state`, `effect-exhausted`) and carries the evidence and human remedy.

### 3.2 The reconciler and durable effect protocol

**Decision:** Every touch follows one transaction/effect loop; no adapter may bypass it:

1. load and closed-validate the aggregate;
2. verify the caller is the current owner domain/id and holds the current epoch when the touch may execute work;
3. observe only the goal objects required by the current phase;
4. deterministically derive the next state and at most the bounded set of independent effects;
5. CAS-persist the new state and each effect intent before performing any external write;
6. execute effects through their ports using stable idempotency keys and the owner epoch;
7. re-read each effect's goal object and CAS-record its evidence;
8. stop on a human wait, no frontier, blocked state, ownership loss, or the configured per-touch bound.

An effect identity is derived from `(repository, feature, aggregate epoch, effect kind, subject, input fingerprint)`. Retrying the same intent converges; a different input is a different effect. A crash before step 5 did nothing. A crash after step 5 replays the pending intent. A crash after the external write but before step 7 observes the goal object and records completion without duplicating it. CAS loss restarts from the winner's state; it is a no-op race, not permission to continue from stale memory.

Independent ready tasks may produce independent dispatch effects in one touch up to the configured concurrency cap. State transitions that invalidate authority—handoff, cancellation, evidence staleness, final proposal creation, and delivery—are serialized as single effects.

### 3.3 Ports and partition

**Decision:** The core defines consumer-side contracts at the highest stable seams:

| Port | Core need | First binding |
|---|---|---|
| `FeatureDriveStore` | create/load/CAS one aggregate; retain transition lineage | remote Git ref + commit object |
| `OwnerIdentity` | stable local-machine, remote-domain, and human-transition identity; sign/verify canonical transition envelopes | local registration key + remote worker identity + authenticated human command |
| `PlanReader` | resolve merged frozen plan, DAG, task routing dialect | existing plan projection/tracker adapters |
| `WorkTracker` | observe tasks/blockers; explicitly record verified landing; create linked repair work; project routing | existing tracker port with additive landing/repair operations |
| `CodeHost` | refs, proposal/review/check reads, proposal creation, landed revision, ancestry | Git/GitHub adapter |
| `Executor` | run one explicit assignment over eligible task ids, base ref, routing scope, and epoch | existing Dispatcher/Worker core |
| `CandidateBuilder` | deterministically construct and publish one candidate | Git plumbing adapter |
| `E2EVerifier` | resolve required scenarios, dispatch routes, read results/attestations | reviewed e2e contract and configured runners |
| `TouchRouter` | route instruction-free touches/backstop probes to owner domains | wake-channel adapters |
| `Reporter` | events, action-required notifications, minimal diagnostic | attended stream + existing notification surfaces |

The core imports no adapter package. The executor assignment contains no feature semantics: an explicit eligible task set, target base, routing signal, and fencing epoch are ordinary execution parameters. The Worker still claims one task, uses one isolated worktree, runs a fresh agent, drives it green, and proposes. The feature drive never shares a worktree and never gives the Worker merge, e2e, handoff, or delivery responsibilities.

### 3.4 Ownership, fencing, and the Git binding

**Decision:** Feature execution ownership is an explicit CAS transition over `{domain, stable_owner_id, epoch, state}` (`design-brainstorm:feature-implementation-drive-tech-design#D-2`). The first store adapter uses `refs/pyxis/feature-drives/<feature-id>`; the ref points to a commit whose tree contains canonical `drive.json`. Each successful update preserves the prior commit as its parent, giving transition lineage without making event sourcing the state model. Creation and updates use force-with-lease against the observed object id; a missing or mismatched expected id loses cleanly and reloads.

Owner ids are persistent registrations, not process ids: local uses a machine registration key generated once in the user's Pyxis config home; remote uses the configured execution-domain identity; an explicit human transition is authenticated through the medium-bound human identity. Every commit carries a canonical signed transition envelope over repository, feature, prior object id, prior/new epoch, prior/new owner, transition kind, aggregate digest, and effect ids. The adapter verifies the signature against the registered actor, verifies that actor is authorized for the transition class, and validates schema, parent continuity, expected epoch, and aggregate digest before honoring the record. A ref deletion or unverifiable replacement is `blocked: malformed-state`, never an unowned feature.

Repository policy restricts the feature-drive and candidate namespaces where the code host supports that capability, but it is defense in depth rather than the authentication root: arbitrary-ref protection is not portable and is not assumed. CAS supplies concurrency; signed transition envelopes supply positive authorship/authorization; expected-parent lineage makes deletion or rewrite detectable. v1 trusts registered owner/human keys and the configured registration process rather than claiming Byzantine consensus among those principals. The store credential may update only the coordination/candidate namespace, while executor and runner credentials retain their narrower existing scopes.

Assignment increments the epoch and installs executable routing for that owner. Every dispatch, feedback repair, and routing mutation carries the epoch; an adapter re-reads the aggregate immediately before action. An old process waking after handoff observes a different epoch and performs zero work. Heartbeats and last-seen timestamps are diagnostics only—absence never changes ownership.

Handoff is `active → quiescing`: remove future routing, wait for every active attempt to reach a durable proposal/checkpoint or terminal result, verify no task claim remains in flight, then CAS the new owner and increment the epoch before installing new routing. Cancellation uses the same quiescence barrier but lands ownerless `cancelled`. No automatic preemption or timeout takeover exists.

### 3.5 Routing and executor composition

**Decision:** Feature tasks are routable only through projections derived from the current aggregate. On each frontier reconciliation, the drive computes `ready = open ∧ all blockers RecordLanding-complete ∧ no open proposal ∧ no active claim`, then projects routing only for those task ids to the current owner domain. Local ownership projects the registered machine's routing signal; remote ownership projects the configured remote-domain signal. An unowned, quiescing, cancelled, abandoned, delivered, or blocked drive projects none.

The general execution engine remains feature-blind: a remote machine consumes ordinary assignments or its configured routing signal and does not parse the feature aggregate. Correctness lives one level higher—the drive is the only producer of executable routing for feature-managed tasks, and the assignment includes the selected task ids/base/epoch. Generic standalone issues retain their ordinary route and never acquire a feature-drive aggregate.

Each task proposal targets the remote feature integration branch fixed at drive creation. Feedback updates the same task branch/proposal under the same owner epoch; a changed head invalidates head-bound checks and review evidence according to repository policy. Work outside the task charter becomes a dependency-linked repair/scope task through the tracker port, never a silent charter expansion.

### 3.6 Human review, landing verification, and DAG progression

**Decision:** Review state is observed but never equated with completion. `open`, `changes-requested`, `approved-but-unmerged`, and `merged-unverified` all leave the task and every dependent incomplete. For a proposal to complete a task, the reconciler must verify: its reviewed head is the current proposal head; its base is the feature branch; required checks/reviews bind that head; the code host reports a landed revision for the merge/squash/rebase result; and that landed revision is an ancestor of the freshly fetched feature-branch head (`design-brainstorm:feature-implementation-drive-tech-design#D-3`).

Only then does the core call idempotent `RecordLanding(task, proposal, reviewed_head, landed_revision, feature_head)`. The tracker adapter writes the explicit completion evidence and closes/transitions the task according to its dialect; readiness continues to use the tracker's one blocker predicate. Wrong base, stale head, absent landed revision, ancestry failure, or tracker write ambiguity moves the drive to `blocked: landing-verification`, projects no newly ready work, and preserves every artifact. This is the feature-branch instance of AP-26/I2: a badge is never the goal object.

Parallel root proposals may land in either human-chosen order. Each independently verified landed revision remains an ancestor of the advancing feature head; a conflict or review-head update remains on that proposal and cannot unlock dependents until it passes the same proof.

### 3.7 Candidate construction and immutable publication

**Decision:** Once every planned and repair task is RecordLanding-complete, the `CandidateBuilder` fetches exact feature head `F` and exact target-main head `M`, verifies both refs resolve to those objects, and computes one canonical synthetic merge `C` (`design-brainstorm:feature-implementation-drive-tech-design#D-4`). It uses the repository's declared merge policy and a version-pinned builder algorithm to produce a merge tree, then a two-parent commit `(M, F)` with canonical author/committer identity, message, and deterministic timestamp rule. The candidate identity records builder version, merge-policy version, `F`, `M`, `C`, and tree id.

Publication is create-only at `refs/pyxis/candidates/<feature-id>/<candidate-id>`, where `candidate-id` is the digest of the complete identity. A pre-existing ref is accepted only if every object and identity field matches; otherwise construction fails closed. Neither source branch moves. A merge conflict creates no candidate and no green evidence; the drive creates conflict-repair work carrying both heads and the conflict report, then returns through human-reviewed feature-branch integration.

The immutable ref is the checkout target for local, remote, CI, specialized, and manual-human routes. Candidate refs are disposable but never moved; cleanup is permitted only after the lifecycle ordering in §3.11 and Q1's retention policy.

### 3.8 E2e evidence, staleness, and repair

**Decision:** `candidate-evidence/v1` is immutable per candidate and scenario-set fingerprint. Its closed identity is `{feature, F, M, C, tree, builder_version, merge_policy_version, scenario_set_fingerprint}`; each required scenario result adds `{scenario_id, route, runner_identity, actor_identity, started_at, completed_at, outcome, evidence_ref}`. Outcomes are closed: `passed`, `failed`, `manual-accepted`, `manual-rejected`, `pending`, `skipped`, `unavailable`, `inconclusive`. Only `passed` and `manual-accepted` are accepted.

The e2e contract owns scenario authoring and route classification. The feature owner coordinates execution: a declared specialized runner wins; absent one, automatic scenarios run in the current execution-owner domain. Every adapter checks out `C` and reports `C`; a result for any other revision is rejected. Manual attestation names the human actor and exact candidate. The aggregate's candidate verdict is accepted only when every required scenario has exactly one current accepted result and no conflicting terminal result.

Before final-proposal intent is persisted, the reconciler re-fetches feature and main. If either differs from `F` or `M`, evidence becomes stale and the drive constructs and fully verifies a new candidate. A failed/negative route creates one idempotent feature-scoped repair task carrying candidate identity, failed evidence, scenario, and implicated task/proposal links. Its human-reviewed landing invalidates the old candidate and reruns the complete required suite; partial reruns never restore eligibility.

### 3.9 Final proposal and delivery proof

**Decision:** Accepted current evidence authorizes exactly one final proposal from the feature branch to target main. The proposal body/check surface identifies `F`, `M`, `C`, the scenario-set fingerprint, and the evidence record. The drive opens or updates the proposal but has no approval or merge credential. A required `feature-candidate` verdict binds the proposal's current head **and exact target-main head** to accepted evidence. Activation requires the code-host adapter to prove the target ruleset both requires that verdict and refuses a merge whose base has advanced beyond the verdict's bound main head (GitHub binding: strict/up-to-date required checks plus a base-push-triggered verdict refresh). If the medium cannot enforce both, final delivery refuses activation rather than relying on notification timing.

The verdict publisher runs on proposal-head changes, target-main changes, and the owner-domain backstop. It publishes green only for the exact accepted `(F,M,C)` and red/stale for every other pair. Immediately before final-proposal creation or update, the reconciler re-reads both heads; branch protection closes the later human-merge race by requiring the same current pair at merge time.

Final-review feedback is ordinary feature work: the current owner updates the feature branch through reviewable repair as required, invalidates candidate evidence, and returns to §3.7. A platform "merged" state starts `delivery-verifying`; it does not complete the drive. The reconciler resolves the final proposal's landed revision `L` and verifies: `L` is an ancestor of freshly fetched target main; the proposal's recorded base input is accepted `M`; and `tree(L) == tree(C)`. The tree equality makes “corresponds” merge-strategy-neutral: merge commit, squash, and rebase may produce different commit graphs, but the delivered feature at `L` must be byte-identical to the accepted candidate tree. Failure leaves ownership intact in `blocked: delivery-verification` and emits a human action brief. Success CAS-transitions exactly once to `delivered`, removes executable routing, releases ownership, and only then permits cleanup.

### 3.10 Touches, waits, and local/remote equivalence

**Decision:** The existing wake channel carries feature-drive touches as the instruction-free pair `{feature_id, reason}`. Push adapters normalize review, merge, check, runner, and manual-attestation events; the bounded backstop scans only aggregates assigned to its owner domain and probes registered waits. Signals assert no completion and carry no authority. The router reads the aggregate only to choose the recorded owner domain; the reconciler independently verifies state and goal objects under the current epoch (`design-brainstorm:feature-implementation-drive-tech-design#D-5`).

For local ownership, the registered machine's adapter or explicit `implement-now <feature>` invocation supplies the touch. For remote ownership, the remote-domain queue/backstop invokes the same core. Review and manual waits persist their awaited goal object in the aggregate and exit with zero model sessions and zero Worker processes. Duplicate push/backstop/manual touches race on CAS: one advances; the rest reload and become recorded no-ops. A local machine offline past the configured wake bound surfaces stale-owner action required but never transfers ownership.

### 3.11 Handoff, cancellation, abandonment, delivery, and cleanup

**Decision:** Terminal and ownership-changing transitions preserve artifacts and order destructive acts last (`design-brainstorm:feature-implementation-drive-tech-design#D-6`):

| Request/observation | Required transition |
|---|---|
| explicit handoff | de-route future tasks → quiesce/checkpoint active attempts → verify no in-flight claim → increment epoch and assign new owner → install its routing |
| explicit cancel | de-route future tasks → quiesce/checkpoint → increment epoch, clear owner → `cancelled`; preserve branches, commits, proposals, tasks, and evidence |
| resume cancelled | `cancelled → unowned`; remain inert until a new explicit assignment increments the epoch and installs routing |
| abandon cancelled | `cancelled → abandoned`; terminal and inert; preserve durable artifacts until separately confirmed cleanup |
| final head/base changes | invalidate evidence → return to candidate construction/full verification |
| final proposal reports merged | `final-review → delivery-verifying`; verify goal-object ancestry |
| delivery verified | remove routing → `delivered` and release owner → apply retention policy to disposable state |
| delivery unverified | retain owner/artifacts → `blocked: delivery-verification`; require repair/disposition |

Cleanup is a typed effect, never an incidental defer. Disposable worktrees and expired routing are reaped after positive terminal evidence. Candidate refs, the feature branch, aggregate transition lineage, and e2e evidence follow repository retention policy; cancellation and abandonment alone authorize no deletion. Destructive cleanup verifies the terminal state, epoch, retained evidence, and policy/confirmation immediately before each deletion.

### 3.12 Boundaries and non-goals

- **Not solved here** — e2e scenario authoring/build adaptation; automatic task or final merge; generic issue execution; the executor's Worker/AgentRuntime internals; a unified dashboard.
- **Owned by a future design** — multi-repository feature drives; fleet-wide fairness/capacity policy; retention durations and archive destinations if they outgrow configuration (Q1); stronger adversarial authorization for coordination refs if repository writers cease to be trusted.
- **Implementation must not infer** — that a signal, label, proposal badge, cached aggregate observation, heartbeat absence, passing subset, or final-PR creation grants authority; that feature ownership constrains a configured specialized e2e runner; that cancellation/abandonment permits deletion; that candidate acceptance permits merge.
- **Must remain unchanged** — the feature-blind Worker and isolated-worktree model; task-level claim semantics; human review/merge authority; the frozen plan's DAG and task charters; the e2e author's scenario contract; generic standalone routing.

### 3.13 Invariants

- **DINV-1** — At most one authenticated ownership epoch is current for a feature; every transition has a valid authorized signature, every executable routing or implementation effect names the epoch, and stale or unverifiable epochs perform zero work.
- **DINV-2** — No external write occurs before its typed effect intent is CAS-persisted; every persisted intent is idempotently observable and resumable after process loss.
- **DINV-3** — CAS loss, duplicate touch, duplicate effect delivery, or pre-existing identical artifact converges to reload/no-op; it never authorizes a second effect.
- **DINV-4** — Feature-managed tasks have executable routing only while ownership is active and only for the current ready frontier; unowned, quiescing, cancelled, abandoned, delivered, or blocked drives route none.
- **DINV-5** — The executor receives task ids, base, routing, and epoch but no feature lifecycle authority; it never merges, advances the DAG, constructs candidates, or releases ownership.
- **DINV-6** — Every task proposal targets the feature branch and remains incomplete through all review states until landing verification succeeds.
- **DINV-7** — `RecordLanding` occurs only after the proposal's current reviewed head/base and landed revision are verified, and the landed revision is an ancestor of the feature branch; failure advances zero dependents.
- **DINV-8** — A candidate identity is a deterministic function of exact `F`, `M`, merge policy, and builder version; its publication ref is immutable and create-only.
- **DINV-9** — Candidate evidence is accepted only when every required scenario has a current accepted outcome bound to the same candidate and scenario-set fingerprint.
- **DINV-10** — Any feature-head, main-head, scenario-set, or final-proposal-head/base change invalidates prior eligibility before a final-proposal effect may execute; final-merge activation requires a medium-enforced current-base and required-verdict gate.
- **DINV-11** — E2e failure creates new traceable repair work and a landed repair requires full candidate reconstruction and full-suite verification; merged task history is never rewritten.
- **DINV-12** — Handoff and cancellation de-route before quiescing and change owner/epoch only after every active attempt is durably stopped; no transition overlaps implementation owners.
- **DINV-13** — Final proposal creation and merged status are not completion; only a landed revision whose accepted base is `M`, whose tree equals accepted `tree(C)`, and which is an ancestor of current main transitions to `delivered`.
- **DINV-14** — Ownership/routing release precedes cleanup; cancellation and abandonment preserve durable artifacts, and every destructive act requires positive terminal evidence plus policy or confirmation.

### 3.14 Observability

- **Events/logs** — one normalized stream: `feature_drive_created`, `ownership_changed`, `drive_touch`, `effect_planned`, `effect_observed`, `task_routed`, `task_landed`, `candidate_published`, `scenario_outcome`, `candidate_verdict`, `repair_created`, `final_proposal_opened`, `delivery_verified`, `drive_cancelled`, `drive_abandoned`, `cleanup_applied`. Every event carries feature id, aggregate version, owner epoch where applicable, effect id, actor/domain, and goal-object refs.
- **Metrics** — ownership-conflict and stale-epoch attempts; ready→proposal and merge→next-frontier latency; invalid landing count; candidate rebuild/staleness count; scenario route outcomes; final-proposal stale blocks; delivery-verification failures; idle model/worker count. M1–M4 derive from these records and medium timestamps.
- **Alerts** — split-owner/stale-epoch attempts, malformed aggregate/ref lineage, landing ancestry failure, effect retry exhaustion, wake staleness, stale evidence at final action, wrong-main delivery, and cleanup refusal emit action-required notifications to the owner/operator; no alert silently changes ownership.
- **Debugging surface** — `feature-drive status <feature>` reads the aggregate and named goal objects to show owner/epoch, phase, task frontier, pending effects, waits, candidate/evidence currency, blockers, and remedy. Native proposal/tracker/check surfaces remain where humans act.
- **Trace/correlation** — feature id → aggregate transition/effect id → task/proposal/landed revision → candidate `F/M/C` → scenario evidence → final proposal → main ancestry proof. The chain is cold-readable without session memory.

### 3.15 Security & privacy

Yes — matches the header. The design handles stable machine/domain identity, repository-write and proposal credentials, protected custom refs, autonomous agent execution routing, source-code publication to existing repository refs, e2e runner identities, and human attestation identity. It introduces no user-content class beyond repository artifacts and no egress destination beyond configured code-host, agent-runtime, notification, and e2e adapters.

Threats and controls: a remote executor stealing local work is stopped by owner-derived routing plus epoch checks (`#DINV-1`, `#DINV-4`); a stale process acting after handoff is fenced (`#DINV-1`, `#DINV-12`); forged/malformed state refuses on canonical signature, authorization, closed schema, ref lineage, parent, digest, and epoch validation (`#DINV-1`, `#DINV-2`); poisoned wake/tracker/PR text remains data and is independently verified at goal objects (`#DINV-3`, `#DINV-7`, `#DINV-13`); credential overreach is bounded by separating store-signing, executor, runner, and human merge credentials (`#DINV-5`, A-11); stale or fabricated evidence cannot authorize a final proposal (`#DINV-8`–`#DINV-10`); destructive cleanup fails closed on positive evidence (`#DINV-14`).

The Git-first binding provides concurrency integrity and authenticated transition records, not Byzantine consensus. A repository writer without a registered signing key can deny service by deleting a custom ref but cannot produce an accepted owner transition; deletion is loud and fail-closed. Compromise of an authorized owner/human signing key remains inside v1's trust boundary and follows credential-revocation recovery. Private keys never enter aggregate/evidence payloads or logs. Runner/actor identities are audit metadata subject to repository retention, not authentication secrets.

### 3.16 Failure modes

| Failure | System behavior | Detection | Recovery |
|---|---|---|---|
| concurrent local/remote kickoff | one create/assign CAS wins; loser reports owner and routes nothing | ownership-conflict event | explicit handoff/cancel only |
| owner process/machine disappears | ownership remains; no foreign routing | stale-owner action-required | same owner resumes, or human handoff/cancel/abandon |
| crash around external effect | pending intent or goal-object observation reconstructs outcome | pending-effect diagnostic | idempotent reconcile |
| task PR reports merged on wrong base or without ancestry | no `RecordLanding`; drive blocks | landing-verification alert | human corrects/disposes proposal, then reconcile |
| candidate merge conflicts | no candidate/evidence; conflict repair created | candidate-conflict event | reviewed repair to feature branch, rebuild |
| runner unavailable/skipped/inconclusive | candidate remains unaccepted | exact blocking scenario shown | restore runner or provide allowed manual route |
| feature/main changes after green | evidence and final eligibility become stale | head comparison / required verdict | new candidate and full rerun |
| final badge true but content absent or tree differs from candidate | ownership retained; drive blocks | base/tree/ancestry verification | human delivery repair/disposition |
| cancel during task execution | de-route; wait for checkpoint/finish; then ownerless cancelled | cancel-pending state | explicit resume assignment or abandon |
| cleanup target ambiguous or policy absent | delete nothing | cleanup refusal | human supplies policy/confirmation |

### 3.17 Compatibility, rollout, and rollback

This is additive. Generic issues and existing `implement-now` invocations retain their current behavior and create no aggregate. Feature-managed plans opt in at kickoff; their task projection must mark them as drive-managed so only the drive projects executable owner routing. Local and remote modes ship behind the same core and conformance suite; v1 dogfood enables local first only as a rollout cohort, not as a different contract, then exercises the remote binding before the product claim is made.

Cutover order is structural: land aggregate/store schemas and hermetic state-machine tests; land Git/ref and identity adapters in shadow diagnostics; land explicit tracker landing evidence; enable owner-scoped routing for a fixture feature; add candidate/e2e evidence; then final delivery and cleanup. A partially deployed adapter refuses a phase whose required capability is absent and leaves the aggregate recoverable.

Rollback disables new kickoff and new routing, then uses the same quiesce/cancel path for active drives. It never deletes coordination refs or rewrites tracker/proposal history. Because every external effect has an idempotency key and goal-object evidence, a corrected binary can resume existing v1 aggregates; a breaking schema change requires an additive reader/migration or a successor design, never in-place reinterpretation.

### 3.18 Performance, cost, and testing strategy

Idle cost is zero model sessions and zero Worker processes. Push is preferred; bounded owner-domain backstops perform exact reads only for assigned aggregates with registered waits. A touch is bounded by configured task concurrency, effect-attempt, and wall-clock caps. Candidate construction is one Git merge per distinct `(F,M,policy,builder)`; content-addressed publication deduplicates retries. No central service or repository-wide hot lock exists; one ref per feature keeps parallel drives disjoint.

Testing follows the seams: aggregate validation, transition legality, effect planning, evidence verdicts, and property/race tests are **in-process**; Git refs, ancestry, candidate construction, crash windows, and worktree behavior are **local-substitutable** fixture repositories with bare remotes; tracker/code-host/runner/touch ports run shared conformance suites against hermetic adapters and the GitHub binding; the two six-task dogfoods are **remote-but-owned** live drills. No true-external mock proves correctness—the production adapters must pass the same conformance suite, while network faults are injected at their transport boundary.

The race matrix repeats concurrent kickoff, duplicate touch, handoff/resume, cancel/dispatch, and evidence/final-action boundaries at least 100 times under the race detector. Restart injection occurs before/after every CAS and external effect. Seeded wrong-base, lying-badge, squash/rebase landing, merge-conflict, stale-head, unavailable-runner, manual-pending, stale-candidate, wrong-main, and cleanup-ambiguity fixtures each pin their corresponding invariant.

## 4. Alternatives considered

| Alternative | Why rejected |
|---|---|
| Event-sourced feature-drive journal (`design-brainstorm:feature-implementation-drive-tech-design#ALT-2`) | complete replay is attractive, but ordering, snapshotting, and migration enlarge v1; aggregate commit lineage supplies audit without making event replay the state model |
| Stateless reconstruction from branches/tracker (`design-brainstorm:feature-implementation-drive-tech-design#ALT-3`) | cannot express atomic ownership, cancellation, pending effects, or evidence currency without ambiguous inference |
| Code-host issue as ownership ledger (`design-brainstorm:feature-implementation-drive-tech-design#ALT-5`) | labels/comments are visible but not a portable atomic ownership transition; retained as reporting projection only |
| Central transactional service (`design-brainstorm:feature-implementation-drive-tech-design#ALT-6`) | strong transactions do not justify mandatory hosting, identity, availability, and idle infrastructure for one-repository v1 |
| Code-host-generated test merge (`design-brainstorm:feature-implementation-drive-tech-design#ALT-8`) | creates the final proposal before evidence, depends on mutable platform merge refs, and makes local/remote semantics medium-specific |
| Independent per-runner candidate reconstruction (`design-brainstorm:feature-implementation-drive-tech-design#ALT-9`) | duplicates the safety-critical merge derivation and makes manual/specialized routes prove identity convergence before testing |

## 5. Risks

| ID | Risk | Impact | Mitigation | Verification / evidence |
|---|---|---|---|---|
| R1 | custom-ref policy cannot enforce a writer boundary on a configured code host | repository writer can erase state or attempt forgery | signed authorized transitions are the trust root; policy is defense in depth; deletion/malformed lineage blocks rather than releases | forged signature, unauthorized signer, deletion, non-parent, and malformed-record fixtures |
| R2 | aggregate grows into a duplicate tracker/event store | drift and expensive CAS contention | store identities and verified joins, not full external artifacts; one transition model; schema review against §3.1 | golden schema and state-size/duplicate-field inspection |
| R3 | crash windows duplicate task, proposal, repair, or final-PR effects | duplicated human work or delivery surfaces | persist intent first; stable idempotency keys; goal-object reconciliation | restart injection before/after every effect boundary |
| R4 | squash/rebase merge identity is interpreted incorrectly | valid task never completes or wrong content unlocks DAG | code-host adapter resolves landed revision by merge strategy; ancestry and reviewed-head/base checks remain invariant | merge/squash/rebase conformance fixtures plus lying-badge negative |
| R5 | candidate construction differs across Git versions | runners test a non-canonical object | single version-pinned builder; canonical metadata/policy; publish one object for all runners | cross-version fixture corpus and golden candidate identities |
| R6 | main advances continuously and repeatedly stales expensive e2e | delivery starvation and cost | surface staleness; serialize candidate effect; allow human scheduling but never weaken currency | advancing-main stress drill and measured rebuild count |
| R7 | owner-scoped routing labels leak or remain after release | foreign/post-terminal execution | derive routing from aggregate; epoch check at dispatch; release-before-cleanup invariant | stale-label, handoff, cancel, delivered routing fixtures |
| R8 | specialized runner identity or manual attestation is forged | unverified feature becomes eligible | adapter-authenticated runner actor; exact candidate binding; immutable evidence; human identity from medium | forged/wrong-candidate evidence security fixtures |
| R9 | retention policy deletes evidence needed for audit or leaves refs forever | lost proof or unbounded repository state | terminal ordering fixed; Q1 owner; dry-run cleanup and explicit confirmation for destructive classes | retention-policy conformance and cleanup refusal fixtures |
| R10 | code host cannot atomically keep final eligibility current with an advancing base | human merges evidence for stale main | activation capability probe requires current-base enforcement plus required candidate verdict; otherwise final delivery unavailable | missing/partial-ruleset refusal and base-advance race drill |

## 6. Acceptance

| ID | Acceptance criterion | Verify | Required evidence | Traces to |
|---|---|---|---|---|
| A-1 | 100 repetitions each of concurrent kickoff, stale-owner resume, handoff, cancel/dispatch, and duplicate-touch races produce one current epoch, zero overlapping implementation owners, and zero foreign attempts | Test, race/failure injection | race-matrix report | `prd:feature-implementation-drive/FR-1` · `FR-2` · `FR-11` · `FR-12` · `NFR-1` · #DINV-1–#DINV-4 |
| A-2 | restart injection at every aggregate/effect boundary resumes to the same next action with zero duplicate tasks, proposals, repairs, candidates, or final proposals | Test | restart matrix and effect journal fixtures | `prd:feature-implementation-drive/FR-13` · `NFR-3` · #DINV-2–#DINV-3 |
| A-3 | local-owned and remote-owned six-task DAGs route every implementation/feedback attempt to the selected domain, target every task PR to the feature branch, and keep generic standalone issues unchanged | Demo + Test | two dogfood journals, PR set, generic executor regression | `prd:feature-implementation-drive/FR-2`–`FR-7` · `FR-14` · `FR-15` · `NR-1` · `NR-3` · #DINV-4–#DINV-6 |
| A-4 | merge, squash, and rebase landing fixtures advance exactly valid dependents; wrong-base, stale-head, absent-revision, and badge-true/non-ancestor fixtures advance none | Test | code-host/tracker conformance report | `prd:feature-implementation-drive/FR-8`–`FR-10` · #DINV-6–#DINV-7 |
| A-5 | identical `(F,M,policy,builder)` inputs reproduce one candidate commit/tree/ref; either-head changes produce a new identity; conflicts publish nothing and create traceable repair | Test | golden candidate corpus and conflict fixture | `prd:feature-implementation-drive/FR-16` · `FR-17` · `FR-20` · #DINV-8 |
| A-6 | every failed, skipped, unavailable, inconclusive, manual-pending, wrong-candidate, or partial-suite fixture blocks final eligibility; one accepted outcome for every required scenario on one current candidate unblocks it | Test | evidence verdict matrix | `prd:feature-implementation-drive/FR-18` · `FR-19` · `FR-26` · `FR-30` · #DINV-9–#DINV-10 |
| A-7 | seeded e2e failure creates one evidence-linked repair task; its reviewed landing invalidates old evidence and only a complete accepted rerun restores eligibility | Test + Demo | repair task/PR/evidence chain | `prd:feature-implementation-drive/FR-20` · `FR-21` · #DINV-11 |
| A-8 | exactly one final human-reviewed proposal is opened from current accepted evidence; the drive performs zero approvals/merges; head/base movement makes its required verdict stale; absent current-base/ruleset capability refuses activation | Test + Inspection | final-proposal fixture, authorization log, base-advance race drill | `prd:feature-implementation-drive/FR-22` · `NR-5` · #DINV-10 |
| A-9 | merged-badge, wrong-base, wrong-tree, and non-ancestor fixtures remain incomplete and owned; only accepted-base + candidate-tree + main-ancestry proof transitions once to delivered, removes routing, releases ownership, then permits cleanup | Test | delivery/base/tree/ancestry fixture report | `prd:feature-implementation-drive/FR-23` · `FR-24` · #DINV-13–#DINV-14 |
| A-10 | cancel in every active phase quiesces without artifact deletion; resume remains inert until explicit assignment; abandonment is terminal and non-destructive | Test | cancellation/resume/abandon matrix | `prd:feature-implementation-drive/FR-25` · `FR-28` · `FR-29` · #DINV-12–#DINV-14 |
| A-11 | core imports no medium adapter; Git/GitHub, local identity, remote identity, tracker, runner, and touch adapters pass conformance; forged/unauthorized transition signatures, epoch/state/evidence, deleted refs, and poisoned signals perform zero unauthorized effects | Test + Inspection + security review | dependency inspection, conformance suite, abuse corpus | `prd:feature-implementation-drive/NFR-2` · `NFR-4` · `NFR-6` · `NFR-7` · §3.15 |
| A-12 | full review waits record zero live model sessions and Worker processes; status reconstructs owner, phase, frontier, pending effect, and evidence currency from durable state after restart | Demo + Analysis | idle-cost capture and cold diagnostic transcript | `prd:feature-implementation-drive/FR-27` · `NFR-3` · `NFR-5` · §3.10, §3.14 |

## 7. Open questions

| ID | Question | Owner | Blocks | Status | Resolution |
|---|---|---|---|---|---|
| Q1 | What repository-configured retention durations and archive destinations apply to delivered/abandoned feature branches, candidate refs, aggregate lineage, and e2e evidence? | @mdarabi / repository policy | launch cleanup policy, not implementation direction | open | ordering is fixed by §3.11 and #DINV-14; default values are selected at launch against dogfood size/audit evidence |

## Principles

- **AP-2 — honored.** Local, remote, push, backstop, and manual entry all become one instruction-free touch and one reconciler behavior.
- **AP-5 — honored.** Ownership, effects, proposals, candidates, evidence, and delivery carry one cold-readable correlation chain; broken states have named events and alerts.
- **AP-6 — honored.** Aggregate, ownership, effects, candidate identity, evidence, and outcomes are closed versioned contracts with additive evolution.
- **AP-11 — honored.** Each predicate has one home: CAS ownership, tracker readiness, feature-branch landing proof, candidate identity, evidence acceptance, and main delivery ancestry.
- **AP-12 — honored.** Ownership, routing, task claims, workspaces, candidate refs, and cleanup are explicitly released, retained, or reaped; absence never grants takeover.
- **AP-13 — honored.** Task completion, candidate green, and delivery are verified artifacts, never process exits, badges, or declarations.
- **AP-14 — honored.** Both local and remote six-task dogfoods, real candidate execution, and failure drills are release evidence rather than simulated confidence.
- **AP-16 — honored.** Touch work, concurrency, retries, effect attempts, and backstop latency are mechanically bounded.
- **AP-17 — honored.** CAS/claim races fail open as no-ops; routing, ownership changes, evidence authorization, and cleanup fail closed.
- **AP-19 — honored.** Human waits hold no model or Worker process; recurring backstops state and measure their exact-read cost.
- **AP-20 — honored.** Workers retain isolated worktrees; the integration branch and immutable candidate ref are committed artifacts, never a shared mutable worker checkout.
- **AP-21 — honored.** The human chooses execution ownership and reviews each proposal once; the drive records and consumes those judgments rather than re-deciding them.
- **AP-22 — honored.** Duplicate touches/effects and CAS races converge to the same aggregate and goal objects; retries are no-ops, not errors or duplicate acts.
- **AP-23 — honored.** One aggregate and disjoint candidate namespace per feature permit parallel drives without shared-file conflicts.
- **AP-25 — honored.** Core contracts are vendor-neutral; Git/GitHub, local machine, remote domain, runner, and wake bindings remain adapters.
- **AP-26 / I2 — honored.** Task and final landing are accepted only by ancestry at the named goal branch; badges cannot advance or close the drive.
- **RW-5 — honored.** This document decides one independently revisable mechanism: the end-to-end feature integration drive; executor, lane, wake, plan, and e2e authoring remain separate designs.
- **Deviation:** none.
