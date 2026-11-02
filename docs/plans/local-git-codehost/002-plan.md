# Local Git code host — plan 002

| Field | Value |
|---|---|
| **Feature** | local-git-codehost |
| **PRD** | prd/local-git-codehost |
| **Design** | design/local-git-codehost |
| **Basis** | lock/local-git-codehost/v1 |
| **Plan version** | 002 |
| **Dialect** | harbormaster/fs-spec:v2 |
| **Epic title** | Deliver exact-head local Git drafts and feature landings for AEC |
| **Status** | Draft — freezes at merge |
| **Template version** | plan-template/v2 |

## Tasks

### T1 — Select and diagnose the local Git code host

| Field | Value |
|---|---|
| **Blocked by** | T13 |
| **Size band** | 140–250 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Requirements** | {"schema_version":"task-requirements/v1","implementation":{"assessment":{"reasoning":"complex","coupling":"cross_boundary","novelty":"adapt","verification":"composed","impact_scope":"multi_surface","architectural_role":"shared_substrate","citations":[{"id":"task:goal"},{"id":"task:acceptance"}]},"rationale":"Selection and doctor cross configuration, draft, implementation, and code-host composition roots."},"review":{"assessment":{"reasoning":"complex","coupling":"cross_boundary","novelty":"adapt","verification":"composed","impact_scope":"multi_surface","architectural_role":"shared_substrate","citations":[{"id":"task:acceptance"}]},"rationale":"Review checks every composition path for a hidden hosted call and an honest trust disclosure."}} |
| **Pattern** | — |
| **Touches** | internal/docgate/config · internal/adapters/codehost · internal/draft · internal/executor · cmd/pyxis · cmd/doctor |

**Goal:** An installed repository can select `local-git`, and every affected command sees one code-host semantic port and a diagnostic of the configured bare origin.

**Current behavior:** Draft and implementation paths invoke hosted code-host operations, and a bare `origin` makes draft status fail through `gh`.

**Desired behavior:** After T13's feature-draft amendment and successor design have merged, the explicit local binding resolves a configured bare origin, refuses an absent or mismatched origin before writes, and routes code-host semantics through consumer-side interfaces. Doctor reports mainline and proposal readiness plus the shared-account, unauthenticated-approver, and copied-origin limits. Hosted bindings retain their existing behavior.

**Scope:**
- explicit local code-host selection and closed configuration validation
- consumer-side code-host operations wired through composition roots
- bare-origin and configured-mainline diagnostics with trust disclosures

**Out of scope:**
- publishing proposal records or approval receipts (T2, T3)

**Acceptance criteria:**
- [ ] after T13's approved amendment and successor design have merged, selecting local mode with a valid bare origin resolves the same configured origin across draft, implementation, and doctor commands
- [ ] an absent or mismatched origin refuses before a write and names the configuration defect
- [ ] a command-spy fixture records zero `gh`, Forgejo, or network calls in local mode
- [ ] hosted adapter conformance and pinned-execution tests keep their prior expected results

**Key interfaces:**
- configured code-host binding and consumer-side proposal, review, and landing ports
- doctor result fields for origin, configured mainline, and accepted trust limits

**Traces:** prd:local-git-codehost/FR-1 · prd:local-git-codehost/FR-10 · prd:local-git-codehost/UB-3 · prd:local-git-codehost/NR-1 · prd:local-git-codehost/NFR-1 · prd:local-git-codehost/DOD-5 · design:local-git-codehost#D-1 · design:local-git-codehost#D-7

### T2 — Publish discoverable local proposal records atomically

| Field | Value |
|---|---|
| **Blocked by** | T1 |
| **Size band** | 200–300 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Requirements** | {"schema_version":"task-requirements/v1","implementation":{"assessment":{"reasoning":"difficult","coupling":"composed","novelty":"first_of_kind","verification":"composed","impact_scope":"multi_surface","architectural_role":"shared_substrate","citations":[{"id":"task:goal"},{"id":"task:acceptance"}]},"rationale":"The local proposal format and paired Git ref transaction establish a new durable code-host medium."},"review":{"assessment":{"reasoning":"difficult","coupling":"composed","novelty":"first_of_kind","verification":"composed","impact_scope":"multi_surface","architectural_role":"shared_substrate","citations":[{"id":"task:acceptance"}]},"rationale":"Race, crash, and clone fixtures must prove the branch and record never diverge."}} |
| **Pattern** | — |
| **Touches** | internal/adapters/codehost/localgit · internal/draft/lifecycle |

**Goal:** A clone can list each local draft or implementation proposal with its exact branch head and workflow-derived target.

**Current behavior:** A bare Git origin has branches but no proposal identity or reviewable proposal state.

**Desired behavior:** A closed proposal record names kind, identity, branch, current head, workflow-derived target ref and old SHA, state, and monotonic event sequence. Publication and revision update branch and proposal refs in one atomic push with expected old OIDs. A stale writer, unsupported atomic push, or interruption leaves the prior pair intact; a completed push is recoverable from a clone.

**Scope:**
- versioned local proposal object and dedicated origin ref
- paired atomic branch and proposal publication with leases
- clone-visible listing, revisions, abandonment, and idempotent recovery

**Out of scope:**
- receipt publication and landing authorization (T3, T5, T8, T10)

**Acceptance criteria:**
- [ ] a fresh clone lists proposal identity, kind, exact head, and target from the configured origin
- [ ] concurrent writers leave one consistent branch/proposal pair and refuse the stale writer
- [ ] an origin without atomic push support refuses without advancing either ref
- [ ] a crash before publication leaves the old pair; a retry after publication recognizes the new pair once

**Key interfaces:**
- closed proposal record with monotonic event sequence
- atomic Git ref transaction with explicit expected old OIDs

**Traces:** prd:local-git-codehost/FR-2 · prd:local-git-codehost/UB-4 · prd:local-git-codehost/NFR-3 · design:local-git-codehost#D-2

### T3 — Record exact-head receipts under the local approval contract

| Field | Value |
|---|---|
| **Blocked by** | T2, T13 |
| **Size band** | 220–300 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Requirements** | {"schema_version":"task-requirements/v1","implementation":{"assessment":{"reasoning":"difficult","coupling":"cross_boundary","novelty":"first_of_kind","verification":"cross_boundary","impact_scope":"system_wide","architectural_role":"system_core","citations":[{"id":"task:goal"},{"id":"task:acceptance"}]},"rationale":"Receipt code crosses the local adapter and draft approval boundary governed by the already merged T13 contract."},"review":{"assessment":{"reasoning":"difficult","coupling":"cross_boundary","novelty":"first_of_kind","verification":"cross_boundary","impact_scope":"system_wide","architectural_role":"system_core","citations":[{"id":"task:acceptance"}]},"rationale":"Review verifies the predecessor contract is merged and adversarial replay cases refuse before local approval is enabled."}} |
| **Pattern** | — |
| **Touches** | internal/adapters/approval/localgit · internal/draft/approval |

**Goal:** The operator can record one declared review of a prepared head, while Pyxis rejects stale or malformed receipts and states that local actor identity is unauthenticated.

**Current behavior:** No local receipt protocol is implemented; T13 must first land the approved exception to the hosted review contract.

**Desired behavior:** After T13 lands the scoped amendment and successor design, the operator command publishes a once-only receipt binding proposal record OID and sequence, operation, branch, exact head, target ref and SHA, digest, and nonce. A changed generation, head, target, or digest refuses, including `H1→H2→H1`; the retained receipt can be checked from a clone without an identity-authentication claim. Hosted review identity behavior remains.

**Scope:**
- once-only local receipt publication and canonical field validation
- same-origin replay, proposal-generation, and target-ref checks

**Out of scope:**
- user identity authentication under the accepted shared OS account
- draft or implementation landing transactions (T5, T8, T10)

**Acceptance criteria:**
- [ ] T13's approved amendment and successor design are merged before any local receipt or approval route is enabled
- [ ] missing, malformed, wrong-head, wrong-target, and changed-generation receipts refuse with no landing mutation
- [ ] `H1→H2→H1` cannot reuse the first `H1` receipt, and a new matching receipt succeeds
- [ ] a fresh clone checks the retained receipt and reports approval actor and repository identity as unauthenticated

**Key interfaces:**
- local review-request and receipt record schemas with proposal OID, sequence, and nonce
- approval port's local consistency result and hosted identity-preserving result

**Traces:** prd:local-git-codehost/FR-3 · prd:local-git-codehost/FR-4 · prd:local-git-codehost/UB-1 · prd:local-git-codehost/DOD-5 · design:local-git-codehost#D-3

### T4 — Drive draft status and rounds from the local origin

| Field | Value |
|---|---|
| **Blocked by** | T3 |
| **Size band** | 170–270 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Requirements** | {"schema_version":"task-requirements/v1","implementation":{"assessment":{"reasoning":"complex","coupling":"cross_boundary","novelty":"adapt","verification":"composed","impact_scope":"multi_surface","architectural_role":"shared_substrate","citations":[{"id":"task:goal"},{"id":"task:acceptance"}]},"rationale":"Draft lifecycle verbs must consume local proposal state and the configured mainline instead of GitHub."},"review":{"assessment":{"reasoning":"complex","coupling":"cross_boundary","novelty":"adapt","verification":"composed","impact_scope":"multi_surface","architectural_role":"shared_substrate","citations":[{"id":"task:acceptance"}]},"rationale":"Review checks both the missing-mainline refusal and the existing first-push gate against a real bare origin."}} |
| **Pattern** | — |
| **Touches** | internal/draft/lifecycle · internal/adapters/draft · cmd/pyxis · internal/codegate |

**Goal:** Draft status, open, round, reopen, and abandon work on a configured local bare origin after its first mainline push.

**Current behavior:** Status and open call hosted APIs; an empty origin needs the separate base-free first-push gate before a draft has a mainline base.

**Desired behavior:** The lifecycle reads local proposal refs through the code-host port, creates or revises paired refs, and preserves the existing draft event state. Open and preparation refuse an absent configured mainline with the first-push remedy. The already approved init-epoch proof supplies that separate first push; subsequent mainline pushes retain history gates, including a non-`main` configured branch.

**Scope:**
- local draft status, open, round, reopen, and abandonment paths
- configured-mainline prerequisite and first-push gate integration
- local proposal identity in draft event and status output

**Out of scope:**
- compile, readiness, and reviewed lock landing (T5)

**Acceptance criteria:**
- [ ] a real initialized bare-origin fixture opens, revises, lists, reopens, and abandons a draft without hosted calls
- [ ] before first configured-mainline publication, open and prepare refuse with the installed first-push remedy
- [ ] the pinned init-epoch first push succeeds base-free; wrong epoch and later history violations refuse
- [ ] a configured mainline branch other than `main` is used for draft targets and history checks

**Key interfaces:**
- lifecycle proposal-store port and configured-mainline resolver
- first-push init-epoch proof from issue #1305 and PR #1307

**Traces:** prd:local-git-codehost/FR-2 · prd:local-git-codehost/FR-9 · prd:local-git-codehost/UB-3 · design:local-git-codehost#D-2 · design:local-git-codehost#D-4

### T5 — Prepare and lock a reviewed local feature draft

| Field | Value |
|---|---|
| **Blocked by** | T3, T4 |
| **Size band** | 220–300 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Requirements** | {"schema_version":"task-requirements/v1","implementation":{"assessment":{"reasoning":"difficult","coupling":"cross_boundary","novelty":"first_of_kind","verification":"cross_boundary","impact_scope":"system_wide","architectural_role":"system_core","citations":[{"id":"task:goal"},{"id":"task:acceptance"}]},"rationale":"Locking joins compile, memory readiness, approval, atomic Git landing, and durable version identity."},"review":{"assessment":{"reasoning":"difficult","coupling":"cross_boundary","novelty":"first_of_kind","verification":"cross_boundary","impact_scope":"system_wide","architectural_role":"system_core","citations":[{"id":"task:acceptance"}]},"rationale":"Review must prove prepared bytes match the landed tree across stale-ref and crash cases."}} |
| **Pattern** | — |
| **Touches** | internal/draft/lock · internal/adapters/codehost/localgit · internal/memory · cmd/pyxis |

**Goal:** One reviewed local draft becomes one durable lock whose exact PRD and tech-spec bytes a fresh clone can verify.

**Current behavior:** Preparation, approval reading, and lock recording assume a hosted pull request and cannot land on a bare origin.

**Desired behavior:** Preparation reuses rebase, compile, readiness, memory, and change-report gates against the pinned configured mainline. Operator-invoked lock rechecks the exact head, proposal generation, target and receipt, then atomically publishes the squash landing, lock tag, final proposal with embedded receipt, archive, and leased branch deletion. A crash before publication leaves the proposal; one after publication is recognized without a second lock.

**Scope:**
- local preparation with hosted-equivalent readiness and memory predicates
- atomic reviewed squash lock, tag, proposal finalization, and archive publication
- clone-visible lock verification and crash-safe idempotent recovery

**Out of scope:**
- task and final feature implementation proposals (T6–T10)

**Acceptance criteria:**
- [ ] every hosted readiness negative fixture also refuses local preparation or lock
- [ ] a matching receipt lands exactly the reviewed PRD and tech-spec bytes once, with one versioned tag
- [ ] moved branch or mainline and missing or altered receipt ref refuse the whole transaction
- [ ] a fresh clone verifies lock bytes, receipt consistency, and reviewed head after deleting the review worktree

**Key interfaces:**
- prepared draft request and lock record with receipt digest and readiness digest
- atomic target, tag, final proposal, archive, and branch-deletion transaction

**Traces:** prd:local-git-codehost/FR-4 · prd:local-git-codehost/FR-5 · prd:local-git-codehost/FR-6 · prd:local-git-codehost/UB-1 · prd:local-git-codehost/UB-2 · design:local-git-codehost#D-4

### T6 — Authorize operator-run local feature-drive landings

| Field | Value |
|---|---|
| **Blocked by** | T3, T14 |
| **Size band** | 120–210 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Requirements** | {"schema_version":"task-requirements/v1","implementation":{"assessment":{"reasoning":"difficult","coupling":"cross_boundary","novelty":"adapt","verification":"cross_boundary","impact_scope":"system_wide","architectural_role":"system_core","citations":[{"id":"task:goal"},{"id":"task:acceptance"}]},"rationale":"Feature-drive local proposal and landing boundaries must implement the already merged T14 contract across the host adapter and drive."},"review":{"assessment":{"reasoning":"difficult","coupling":"cross_boundary","novelty":"adapt","verification":"cross_boundary","impact_scope":"system_wide","architectural_role":"system_core","citations":[{"id":"task:acceptance"}]},"rationale":"Review checks the predecessor contract is merged and local operator invocation never grants Argus merge authority or weakens hosted rules."}} |
| **Pattern** | — |
| **Touches** | internal/featuredrive · internal/adapters/codehost/localgit |

**Goal:** The feature drive can request and observe local task and final proposals while only an operator procedure invokes their landing command.

**Current behavior:** Its PRD requires human code-host merge and host-enforced final-candidate checks; a shared-account bare origin cannot provide that protection.

**Desired behavior:** After T14 lands the scoped amendment and successor design, the drive opens or updates local proposals and verifies landed results but never calls the operator-only landing operation. Hosted mode retains required review/check enforcement.

**Scope:**
- drive-facing local proposal and observer interfaces
- local landing command separated from Argus's request path

**Out of scope:**
- task aggregate and landing mechanics (T7, T8)
- final candidate and landing mechanics (T9, T10)

**Acceptance criteria:**
- [ ] T14's approved feature-drive amendment and successor design are merged before local task or final landing is enabled
- [ ] an Argus drive can request a local task or final proposal and observe status without invoking the landing command
- [ ] the landing surface presents the exact head, workflow-derived target, and unauthenticated actor boundary to the operator
- [ ] hosted feature-drive authorization and required-verdict fixtures retain their prior outcomes

**Key interfaces:**
- feature-drive proposal request and landed-result observer ports
- operator landing command boundary and local-mode policy disclosure

**Traces:** prd:local-git-codehost/FR-7 · prd:local-git-codehost/FR-8 · prd:local-git-codehost/DOD-6 · design:local-git-codehost#D-5 · design:local-git-codehost#D-7

### T7 — Retain current task review evidence on the origin

| Field | Value |
|---|---|
| **Blocked by** | T6 |
| **Size band** | 170–270 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Requirements** | {"schema_version":"task-requirements/v1","implementation":{"assessment":{"reasoning":"difficult","coupling":"composed","novelty":"first_of_kind","verification":"composed","impact_scope":"multi_surface","architectural_role":"shared_substrate","citations":[{"id":"task:goal"},{"id":"task:acceptance"}]},"rationale":"Task review aggregates must remain reachable and generation-bound across a lost worktree or process."},"review":{"assessment":{"reasoning":"difficult","coupling":"composed","novelty":"first_of_kind","verification":"composed","impact_scope":"multi_surface","architectural_role":"shared_substrate","citations":[{"id":"task:acceptance"}]},"rationale":"Review checks aggregate completeness, evidence ref immutability, and recovery from a clone."}} |
| **Pattern** | — |
| **Touches** | internal/adapters/codehost/localgit · internal/executor/reviewgate · internal/featuredrive |

**Goal:** A local task proposal carries the complete satisfying review aggregate for its current head on a durable origin ref.

**Current behavior:** The hosted drive can inspect proposal reviews, but a local proposal has no clone-visible task ReviewGate aggregate.

**Desired behavior:** Before the operator review request, the adapter exact-reads a complete satisfied aggregate and current codegate/check verdicts, then publishes canonical aggregate bytes with proposal OID and sequence under an immutable ref. The request and receipt bind that ref, object OID, and digest; a clone can recover it after the review worktree vanishes. Missing, partial, stale, or altered evidence blocks landing and later `RecordLanding`.

**Scope:**
- current task ReviewGate and codegate/check predicate
- immutable aggregate envelope publication and receipt binding
- fresh-clone recovery and missing-ref refusal

**Out of scope:**
- constructing the task squash landing revision (T8)

**Acceptance criteria:**
- [ ] missing, partial, stale, or invalidated aggregates refuse before any target ref update
- [ ] the immutable aggregate ref retains canonical bytes, proposal generation, OID, and digest after deleting the review worktree
- [ ] an existing evidence ref is idempotent only when its object and bytes match exactly
- [ ] missing or altered retained evidence blocks both landing and post-crash `RecordLanding`

**Key interfaces:**
- `ReviewGate` aggregate reader and `review-aggregates` origin evidence envelope
- receipt evidence-ref, object-OID, and digest fields

**Traces:** prd:local-git-codehost/FR-7 · prd:local-git-codehost/UB-1 · prd:local-git-codehost/NFR-3 · design:local-git-codehost#D-5

### T8 — Land reviewed tasks without losing parallel work

| Field | Value |
|---|---|
| **Blocked by** | T7 |
| **Size band** | 180–290 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Requirements** | {"schema_version":"task-requirements/v1","implementation":{"assessment":{"reasoning":"difficult","coupling":"cross_boundary","novelty":"first_of_kind","verification":"cross_boundary","impact_scope":"system_wide","architectural_role":"system_core","citations":[{"id":"task:goal"},{"id":"task:acceptance"}]},"rationale":"Squash landing must coordinate Git target leases, retained review evidence, and feature DAG progression."},"review":{"assessment":{"reasoning":"difficult","coupling":"cross_boundary","novelty":"first_of_kind","verification":"cross_boundary","impact_scope":"system_wide","architectural_role":"system_core","citations":[{"id":"task:acceptance"}]},"rationale":"Parallel-root and stale-target fixtures must prove earlier landed revisions remain ancestors."}} |
| **Pattern** | — |
| **Touches** | internal/adapters/codehost/localgit · internal/featuredrive · internal/executor |

**Goal:** An operator-reviewed task proposal lands once on the feature-integration branch and preserves every earlier landed task revision.

**Current behavior:** No local task merge exists; the feature drive expects a hosted merged revision and fresh feature-branch ancestry proof.

**Desired behavior:** With reviewed head `H` descending from receipt-bound target base `B`, the local operator command prepares squash revision `L` with parent `B` and `tree(H)`. One atomic push leases the feature target, final proposal, reviewed-head archive, and proposal-branch deletion. `RecordLanding` verifies `L` and current feature ancestry. A parallel proposal from old `B` refuses after the first landing and must rebase, rerun checks/review, and obtain a new receipt before its child squash lands.

**Scope:**
- operator-invoked task squash and atomic update of target, proposal, and archive refs
- landed-revision, tree, and feature-branch ancestry verification
- parallel-root stale refusal and reviewed rebase recovery

**Out of scope:**
- final feature-to-main candidate landing (T9, T10)

**Acceptance criteria:**
- [ ] the accepted task landing has `parent(L) == B`, `tree(L) == tree(H)`, and `L` ancestor of the fetched feature branch
- [ ] a moved target or proposal branch rejects the complete transaction and retains review evidence
- [ ] two parallel roots land in either operator-chosen order only after the second rebases and obtains fresh checks, aggregate, and receipt
- [ ] `RecordLanding` refuses a missing or non-ancestor revision and advances zero dependents

**Key interfaces:**
- task landing result with reviewed head, base, squash revision, and feature head
- `RecordLanding` verification boundary and atomic Git ref lease

**Traces:** prd:local-git-codehost/FR-7 · prd:local-git-codehost/UB-2 · design:local-git-codehost#D-5

### T9 — Retain final candidate evidence for local review

| Field | Value |
|---|---|
| **Blocked by** | T8 |
| **Size band** | 160–260 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Requirements** | {"schema_version":"task-requirements/v1","implementation":{"assessment":{"reasoning":"difficult","coupling":"composed","novelty":"first_of_kind","verification":"composed","impact_scope":"multi_surface","architectural_role":"shared_substrate","citations":[{"id":"task:goal"},{"id":"task:acceptance"}]},"rationale":"Candidate identity, all required scenarios, and proposal generation must survive a deleted evidence worktree."},"review":{"assessment":{"reasoning":"difficult","coupling":"composed","novelty":"first_of_kind","verification":"composed","impact_scope":"multi_surface","architectural_role":"shared_substrate","citations":[{"id":"task:acceptance"}]},"rationale":"Review verifies current accepted scenario coverage and immutable origin evidence before the operator sees a request."}} |
| **Pattern** | — |
| **Touches** | internal/adapters/codehost/localgit · internal/featuredrive/candidate · internal/featuredrive/evidence |

**Goal:** The final proposal's accepted candidate and every required scenario result remain verifiable from a fresh clone.

**Current behavior:** The hosted feature drive can publish candidate/evidence refs, but the local final proposal has no durable receipt-bound evidence envelope.

**Desired behavior:** The adapter exact-reads accepted `candidate-evidence/v1` for `(F,M,C,scenario_set_fingerprint)`, requires one current accepted result for each required scenario, and publishes its complete canonical envelope with proposal OID/sequence on an immutable origin ref before operator review. The receipt and final proposal pin ref, OID, digest, candidate, and scenario-set fingerprint. A clone rechecks the same bytes; missing, negative, conflicting, or stale evidence refuses.

**Scope:**
- current candidate and required-scenario eligibility predicate
- immutable final-candidate evidence envelope and receipt binding
- clone recovery after evidence-worktree loss

**Out of scope:**
- constructing and publishing the final mainline landing (T10)

**Acceptance criteria:**
- [ ] a failed, skipped, missing, conflicting, or stale scenario result blocks final review eligibility
- [ ] complete accepted evidence is fetchable from the immutable origin ref after deleting the producing worktree
- [ ] a changed feature head, main head, proposal generation, or scenario set invalidates prior evidence and receipt
- [ ] an absent or altered evidence ref blocks landing and post-crash delivery verification

**Key interfaces:**
- `candidate-evidence/v1` result set and immutable local evidence envelope
- final receipt's candidate identity, scenario fingerprint, and evidence-ref fields

**Traces:** prd:local-git-codehost/FR-7 · prd:local-git-codehost/UB-1 · prd:local-git-codehost/NFR-3 · design:local-git-codehost#D-5

### T10 — Land and verify the final feature on local mainline

| Field | Value |
|---|---|
| **Blocked by** | T9 |
| **Size band** | 200–300 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Requirements** | {"schema_version":"task-requirements/v1","implementation":{"assessment":{"reasoning":"difficult","coupling":"cross_boundary","novelty":"first_of_kind","verification":"cross_boundary","impact_scope":"system_wide","architectural_role":"system_core","citations":[{"id":"task:goal"},{"id":"task:acceptance"}]},"rationale":"Final delivery joins accepted candidate bytes, leased main and feature refs, and the drive's delivery proof."},"review":{"assessment":{"reasoning":"difficult","coupling":"cross_boundary","novelty":"first_of_kind","verification":"cross_boundary","impact_scope":"system_wide","architectural_role":"system_core","citations":[{"id":"task:acceptance"}]},"rationale":"Review checks stale-main refusal, distinct proposal/feature refs, and accepted-tree ancestry after crashes."}} |
| **Pattern** | — |
| **Touches** | internal/adapters/codehost/localgit · internal/featuredrive · internal/draft/lock |

**Goal:** The operator's exact-head final feature review yields one mainline revision with the accepted candidate tree and a verifiable delivery result.

**Current behavior:** The drive requires a host-enforced current-candidate verdict and hosted human merge; a local origin has neither route.

**Desired behavior:** A distinct local final proposal ref at `F` and current evidence permit the operator command to prepare mainline revision `L` as child of accepted main `M` with `tree(L) == tree(C)`, plus retained feature marker `F'` as child of `F` with the same tree. One atomic push leases main, feature, proposal deletion, final proposal, and archive. Delivery verifies fresh mainline ancestry, `M`, candidate tree, and retained evidence before marking the feature complete.

**Scope:**
- local final landing object and feature marker construction
- atomic multi-ref lease and stale-input refusal
- post-crash candidate-tree, evidence, and mainline-ancestry proof

**Out of scope:**
- a claim that the shared-account origin prevents direct writes outside Pyxis

**Acceptance criteria:**
- [ ] accepted delivery has `parent(L) == M`, `tree(L) == tree(C)`, and `L` ancestor of freshly fetched mainline
- [ ] movement of main, feature, or distinct proposal branch rejects the entire push
- [ ] a crash after publication recovers the final proposal, receipt, evidence, and landing without a second delivery
- [ ] a direct write outside the normal route is disclosed as outside local enforcement, not certified as approved

**Key interfaces:**
- final landing result `(F,M,C,L,F')` and retained candidate evidence
- atomic mainline, feature-marker, final-proposal, archive, and branch-deletion transaction

**Traces:** prd:local-git-codehost/FR-7 · prd:local-git-codehost/UB-2 · prd:local-git-codehost/NFR-1 · design:local-git-codehost#D-5

### T11 — Report local proposal identities to Argus and Carina

| Field | Value |
|---|---|
| **Blocked by** | T5, T8, T10 |
| **Size band** | 140–240 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Requirements** | {"schema_version":"task-requirements/v1","implementation":{"assessment":{"reasoning":"complex","coupling":"cross_boundary","novelty":"adapt","verification":"cross_boundary","impact_scope":"multi_surface","architectural_role":"shared_substrate","citations":[{"id":"task:goal"},{"id":"task:acceptance"}]},"rationale":"Argus and Carina must consume typed local refs instead of hosted URLs across draft and implementation events."},"review":{"assessment":{"reasoning":"complex","coupling":"cross_boundary","novelty":"adapt","verification":"cross_boundary","impact_scope":"multi_surface","architectural_role":"shared_substrate","citations":[{"id":"task:acceptance"}]},"rationale":"Review checks journal recovery, actual code-host claims, and metric refusal counts across tracker and adapter surfaces."}} |
| **Pattern** | — |
| **Touches** | Argus guide · internal/adapters/tracker/carina · internal/featuredrive · internal/observability · cmd/pyxis |

**Goal:** Argus and Carina identify local drafts, task proposals, and landings by their real Git refs and full heads, with honest diagnostic events.

**Current behavior:** Their proposal records expect hosted URLs or hosted status; a bare Git proposal cannot be reported accurately.

**Desired behavior:** Argus journals local proposal identities and observed landing results without inventing PR URLs, and Carina links the same typed ref/head. Doctor, status, logs, and metrics label local mode, the current origin/prerequisite, receipt refusals, and the accepted trust limits. A tracker outage preserves a replayable local result and does not create a false delivery claim.

**Scope:**
- local proposal and landing identities in Argus and Carina records
- local-mode doctor, status, structured logs, and metric events
- tracker retry and recovery from retained Git state

**Out of scope:**
- changing hosted proposal URL or review behavior

**Acceptance criteria:**
- [ ] an Argus-driven local draft and code proposal name real ref/full-head identities in the journal and Carina item
- [ ] no local status or log claims a hosted PR review, authenticated approver, or protected origin
- [ ] refused approvals emit a typed reason and proposal identity; successful locks and landings emit distinct events
- [ ] tracker unavailability leaves a retryable local result and advances no unverified feature state

**Key interfaces:**
- typed local proposal identity and observed landing result in drive/tracker adapters
- local diagnostic and event schemas for lock, landing, and approval refusal

**Traces:** prd:local-git-codehost/FR-8 · prd:local-git-codehost/FR-10 · prd:local-git-codehost/NFR-1 · design:local-git-codehost#D-1 · design:local-git-codehost#D-5 · design:local-git-codehost#D-7

### T12 — Prove the candidate release in AEC

| Field | Value |
|---|---|
| **Blocked by** | T4, T5, T8, T10, T11 |
| **Size band** | 100–200 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Requirements** | {"schema_version":"task-requirements/v1","implementation":{"assessment":{"reasoning":"complex","coupling":"cross_boundary","novelty":"adapt","verification":"cross_boundary","impact_scope":"system_wide","architectural_role":"shared_substrate","citations":[{"id":"task:goal"},{"id":"task:acceptance"}]},"rationale":"A candidate built from the intended release commit must be exercised through a separately reviewed and landed AEC proof, then verified from its owning origin."},"review":{"assessment":{"reasoning":"difficult","coupling":"cross_boundary","novelty":"adapt","verification":"cross_boundary","impact_scope":"system_wide","architectural_role":"shared_substrate","citations":[{"id":"task:acceptance"}]},"rationale":"Review checks artifact hashes, the independently landed AEC commit and report, actual human review, local-only calls, Carina state, and release-source equivalence."}} |
| **Pattern** | — |
| **Touches** | cmd/pyxis-release · internal/release · internal/adapters/codehost/localgit · internal/selfhost |

**Goal:** A checksum-verified candidate from the current integration source demonstrates one AEC feature from local draft through reviewed code landing and verified delivery without hosted calls.

**Current behavior:** AEC is pinned to a provisional local test release; five Argus drives park before brainstorm because draft status calls GitHub.

**Desired behavior:** Pack and verify a candidate artifact set from the current Pyxis integration commit. In the separate AEC repository, prepare an operator-reviewed local proposal that pins this candidate and commits a tracked test report with one feature's draft lock, plan projection, code proposal, task/final landing, Carina state, gate and phase evidence, exact refs, and candidate hashes. Land that AEC proposal on its own mainline before T12 claims success. The Pyxis task exact-reads the landed AEC commit and report from the configured AEC origin, checks the pin and evidence against the candidate manifest, and carries their full commit/OID and digest in its own review package; it never writes AEC files through a Pyxis task proposal. A separate installed AEC fixture completes the deterministic local Git/Carina path with networking disabled and zero outbound calls; live Codex model traffic is reported separately. After T12 and the final feature land, repeat the installed proof from the final source commit before tagging, publishing the same verified artifacts, and repinning AEC; changed source or hashes require a new proof.

**Scope:**
- candidate pack and checksum verification in Pyxis
- verification of the separately landed AEC pin, report, and local bare-origin proof from AEC origin
- network-disabled installed local-route fixture
- exact AEC commit/report evidence in the Pyxis review package and post-merge release handoff

**Out of scope:**
- OCR, general language understanding, or a production RFP submission workflow
- authoring or landing AEC repository files from the Pyxis task proposal
- final tag, publication, and published-source repin before T12 and the final feature land

**Acceptance criteria:**
- [ ] all local lifecycle, stale-receipt, crash, and hosted-regression fixtures pass with zero hosted calls in local mode
- [ ] a separate operator-reviewed AEC proposal containing the candidate pin and tracked test report lands on AEC mainline; the Pyxis task independently reads its full commit/OID and report digest from AEC origin and verifies one exact-head reviewed draft lock and code landing with Git, Carina, gate, and phase evidence
- [ ] an installed AEC fixture with networking disabled completes the deterministic local Git/Carina draft-to-delivery path and records zero outbound calls
- [ ] `pyxis-release verify` proves the candidate manifest and artifact hashes from the named integration commit; the landed AEC report pins those values, and the Pyxis review package requires a fresh installed proof from the final source commit before tag, publication, and AEC repin

**Key interfaces:**
- candidate artifact manifest, pin, and release-source identity
- AEC origin commit/OID and report digest, checked against the candidate manifest and local Git/Carina evidence

**Traces:** prd:local-git-codehost/FR-1 · prd:local-git-codehost/FR-7 · prd:local-git-codehost/FR-8 · prd:local-git-codehost/FR-9 · prd:local-git-codehost/FR-10 · prd:local-git-codehost/NR-2 · prd:local-git-codehost/NFR-2 · prd:local-git-codehost/DOD-1 · prd:local-git-codehost/DOD-2 · prd:local-git-codehost/DOD-3 · prd:local-git-codehost/DOD-4 · design:local-git-codehost#D-6 · design:local-git-codehost#D-7

### T13 — Approve the local feature-draft receipt contract

| Field | Value |
|---|---|
| **Blocked by** | — |
| **Size band** | 100–200 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Requirements** | {"schema_version":"task-requirements/v1","implementation":{"assessment":{"reasoning":"difficult","coupling":"cross_boundary","novelty":"adapt","verification":"cross_boundary","impact_scope":"system_wide","architectural_role":"system_core","citations":[{"id":"task:goal"},{"id":"task:acceptance"}]},"rationale":"An offline local receipt changes the feature-draft review guarantee and needs its own approved, merged contract before code uses it."},"review":{"assessment":{"reasoning":"difficult","coupling":"cross_boundary","novelty":"adapt","verification":"cross_boundary","impact_scope":"system_wide","architectural_role":"system_core","citations":[{"id":"task:acceptance"}]},"rationale":"Operator review verifies the exact local exception and that hosted approval retains authenticated review."}} |
| **Pattern** | — |
| **Touches** | docs/prds/feature-draft · docs/tech-design/feature-draft-local-git.md |

**Goal:** The feature-draft contract explicitly authorizes the accepted single-user local receipt boundary before any local approval implementation lands.

**Current behavior:** Feature-draft approval requires authenticated hosted review, while a shared-account bare origin can verify only exact-head receipt consistency.

**Desired behavior:** A scoped PRD amendment and reviewed successor design authorize operator-declared local receipts with no actor authentication or host-level mainline protection, retain hosted identity requirements, and define the proposal generation, exact head, target, digest, nonce, and replay checks T3 will implement. The contract PR lands independently before T1 enables local mode and before T3 implements local approval.

**Scope:**
- feature-draft amendment for the accepted local review boundary
- successor design for receipt fields, freshness checks, and operator procedure
- operator review and merged contract evidence

**Out of scope:**
- receipt producer, verifier, or landing code (T3 and T5)
- an operator-held signing key or authenticated local approver

**Acceptance criteria:**
- [ ] the amendment and successor design state the shared-account limit and preserve authenticated hosted review
- [ ] the contract defines once-only, generation-bound exact-head receipts and refusal of stale or replayed approvals
- [ ] operator review approves the precise contract source before its independent PR merges
- [ ] T1 and T3 can cite the merged amendment and design without enabling local mode in this task

**Key interfaces:**
- approved local receipt and review-request contract for T3
- explicit hosted and local assurance statements

**Traces:** prd:local-git-codehost/FR-3 · prd:local-git-codehost/FR-4 · prd:local-git-codehost/DOD-5 · design:local-git-codehost#D-3

### T14 — Approve the local feature-drive landing contract

| Field | Value |
|---|---|
| **Blocked by** | T13 |
| **Size band** | 100–200 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Requirements** | {"schema_version":"task-requirements/v1","implementation":{"assessment":{"reasoning":"difficult","coupling":"cross_boundary","novelty":"adapt","verification":"cross_boundary","impact_scope":"system_wide","architectural_role":"system_core","citations":[{"id":"task:goal"},{"id":"task:acceptance"}]},"rationale":"Local task and final landing changes the feature-drive human-merge and candidate-verdict boundary and must land as a contract before enabling code."},"review":{"assessment":{"reasoning":"difficult","coupling":"cross_boundary","novelty":"adapt","verification":"cross_boundary","impact_scope":"system_wide","architectural_role":"system_core","citations":[{"id":"task:acceptance"}]},"rationale":"Operator review verifies the operator-only local landing route, retained review evidence, and unchanged hosted requirements."}} |
| **Pattern** | — |
| **Touches** | docs/prds/feature-implementation-drive · docs/tech-design/feature-implementation-drive-local-git.md |

**Goal:** The feature-drive contract authorizes local task and final landings through the operator procedure before any local landing implementation is enabled.

**Current behavior:** Feature drive requires human hosted merge and host-enforced candidate checks, which the accepted bare origin cannot independently provide.

**Desired behavior:** A scoped amendment to FR-7, FR-22, and NR-5 and a reviewed successor design define operator-invoked local landing, current task ReviewGate and final candidate evidence, and the shared-account limit. The drive may request and observe proposals but cannot invoke landing; hosted mode keeps required review and check enforcement. The contract PR lands independently before T6 code.

**Scope:**
- feature-drive amendment for local task and final landing
- successor design for the operator command boundary and evidence requirements
- operator review and merged contract evidence

**Out of scope:**
- drive, review aggregate, or landing implementation (T6–T10)
- authenticated local identity or host-protected mainline

**Acceptance criteria:**
- [ ] the amendment and successor design define the operator-only local landing procedure and accepted trust limit
- [ ] task ReviewGate and final candidate evidence remain required at the exact proposal head
- [ ] hosted human-merge and required-check guarantees remain unchanged
- [ ] operator review approves the precise contract source before its independent PR merges and T6 can cite it

**Key interfaces:**
- approved drive-facing proposal and operator landing boundaries for T6
- task and final evidence obligations for T7–T10

**Traces:** prd:local-git-codehost/FR-7 · prd:local-git-codehost/FR-8 · prd:local-git-codehost/DOD-6 · design:local-git-codehost#D-5 · design:local-git-codehost#D-7

## Coverage

| Requirement | Tasks |
|---|---|
| prd:local-git-codehost/FR-1 | T1, T12 |
| prd:local-git-codehost/FR-2 | T2, T4 |
| prd:local-git-codehost/FR-3 | T3, T13 |
| prd:local-git-codehost/FR-4 | T3, T5, T13 |
| prd:local-git-codehost/FR-5 | T5 |
| prd:local-git-codehost/FR-6 | T5 |
| prd:local-git-codehost/FR-7 | T6, T7, T8, T9, T10, T12, T14 |
| prd:local-git-codehost/FR-8 | T6, T11, T12, T14 |
| prd:local-git-codehost/FR-9 | T4, T12 |
| prd:local-git-codehost/FR-10 | T1, T11, T12 |
| prd:local-git-codehost/UB-1 | T3, T5, T7, T9 |
| prd:local-git-codehost/UB-2 | T5, T8, T10 |
| prd:local-git-codehost/UB-3 | T1, T4 |
| prd:local-git-codehost/UB-4 | T2 |
| prd:local-git-codehost/NR-1 | T1 |
| prd:local-git-codehost/NR-2 | T12 |
| prd:local-git-codehost/NFR-1 | T1, T10, T11 |
| prd:local-git-codehost/NFR-2 | T12 |
| prd:local-git-codehost/NFR-3 | T2, T7, T9 |
| prd:local-git-codehost/DOD-1 | T12 |
| prd:local-git-codehost/DOD-2 | T3, T12 |
| prd:local-git-codehost/DOD-3 | T1, T12 |
| prd:local-git-codehost/DOD-4 | T12 |
| prd:local-git-codehost/DOD-5 | T1, T3, T13 |
| prd:local-git-codehost/DOD-6 | T6, T14 |
