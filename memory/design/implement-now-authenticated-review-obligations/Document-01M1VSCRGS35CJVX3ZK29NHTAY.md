# implement-now authenticated review obligations — technical design

| Field | Value |
|---|---|
| **Status** | Draft → Approved *(merge requires Approved; merge is the freezing event)* |
| **Owner** | @mdarabi |
| **Approver(s)** | @mdarabi (operator and security reviewer) |
| **Date** | 2026-08-20 |
| **Relevant-when** | designing or changing implement-now dispatch, exact-green handoff, configured reviewer setup, review routing, reviewer workspace isolation, proposal-artifact attribution, review recovery, or run significance; implementing PR #1034; superseding the executor composition |
| **Scope** | The complete implement-now Worker/Dispatcher composition, with a durable per-proposal review obligation created at exact green, claim/implementation-width release before review, typed preflight, role-qualified routing, isolated reviewer execution, authenticated artifact reconciliation, independent review failure classes, and review-only recovery. **Not:** impl-review lens/pass/aggregation internals, native catalog values, the canonical evidence store's git mechanics, task authoring, landing, or merge authority. |
| **Tracking** | issue #1029 · PR #1056 |
| **Builds-on** | docs/tech-design/plan-new-routed-demand.md · docs/tech-design/execution-demand-contract-v1.md · docs/tech-design/native-routing-policy-v1.md · docs/tech-design/routing-evidence-v1.md · docs/tech-design/impl-review-host-persistence.md · docs/tech-design/implement-now-e2e-tasks.md · docs/tech-design/codegate-v1.md · docs/tech-design/feature-implementation-drive.md · docs/tech-design/fleet-bridge-v1.md · docs/tech-design/argus-agent-v1.md · docs/tech-design/merge-lane-v1.md |
| **Supersedes** | docs/tech-design/implement-now-routed-execution.md |
| **Serves** | prd:implement-now/FR-1 · prd:implement-now/FR-2 · prd:implement-now/FR-3 · prd:implement-now/FR-4 · prd:implement-now/FR-5 · prd:implement-now/FR-7 · prd:implement-now/FR-8 · prd:implement-now/FR-9 · prd:implement-now/FR-10 · prd:implement-now/FR-11 · prd:implement-now/FR-13 · prd:implement-now/FR-14 · prd:implement-now/FR-15 · prd:implement-now/FR-16 · prd:implement-now/FR-17 · prd:implement-now/FR-18 · prd:implement-now/FR-19 · prd:implement-now/FR-20 · prd:implement-now/FR-21 · prd:implement-now/FR-22 · prd:implement-now/FR-23 · prd:implement-now/FR-24 · prd:implement-now/FR-25 · prd:implement-now/FR-26 · prd:implement-now/FR-28 · prd:implement-now/FR-29 · prd:implement-now/FR-30 · prd:implement-now/FR-31 · prd:implement-now/FR-32 · prd:implement-now/FR-35 · prd:implement-now/FR-40 · prd:implement-now/FR-41 · prd:implement-now/FR-42 · prd:implement-now/FR-43 · prd:implement-now/FR-44 · prd:implement-now/FR-45 · prd:implement-now/FR-46 · prd:implement-now/FR-47 · prd:implement-now/FR-48 · prd:implement-now/FR-49 · prd:implement-now/NR-1 · prd:implement-now/NR-2 · prd:implement-now/NR-3 · prd:implement-now/NR-4 · prd:implement-now/NR-5 · prd:implement-now/NR-6 · prd:implement-now/NR-8 · prd:implement-now/NR-10 · prd:implement-now/UB-1 · prd:implement-now/UB-2 · prd:implement-now/UB-3 · prd:implement-now/UB-4 · prd:implement-now/UB-5 · prd:implement-now/UB-6 · prd:implement-now/UB-7 · prd:implement-now/UB-8 · prd:implement-now/UB-9 · prd:implement-now/UB-11 · prd:implement-now/UB-12 · prd:implement-now/UB-13 · prd:implement-now/UB-14 · prd:implement-now/UB-15 · prd:implement-now/UB-16 · prd:implement-now/UB-17 · prd:implement-now/UB-18 · prd:implement-now/UB-20 · prd:implement-now/UB-21 · prd:implement-now/ABUSE-1 · prd:implement-now/ABUSE-2 · prd:implement-now/ABUSE-3 · prd:implement-now/ABUSE-4 · prd:implement-now/ABUSE-5 · prd:implement-now/ABUSE-6 · prd:implement-now/NFR-1 · prd:implement-now/NFR-2 · prd:implement-now/NFR-3 · prd:implement-now/NFR-4 · prd:implement-now/NFR-5 · prd:implement-now/NFR-6 · prd:implement-now/NFR-7 · prd:implement-now/NFR-9 · prd:implement-now/EVT-1 · prd:implement-now/EVT-3 · prd:implement-now/EVT-4 · prd:implement-now/EVT-7 · prd:implement-now/EVT-8 · prd:implement-now/EVT-9 · prd:implement-now/EVT-10 · prd:implement-now/EVT-11 · prd:implement-now/EVT-12 · prd:implement-now/EVT-15 · prd:implement-now/EVT-16 · prd:implement-now/EVT-17 · prd:implement-now/EVAL-3 · prd:planner-driven-model-routing/FR-17 · prd:planner-driven-model-routing/NR-8 |
| **Security/privacy impact** | yes — this design separates implementation and reviewer process capabilities, mints attempt identities, pins completion to impl-review's exact receipted publication plus protected attempt proof and reciprocal terminals, constrains reviewer filesystem and egress capability, and makes review failure affect unattended run control. A distinct code-host reviewer principal is not required. reviewer: @mdarabi. |
| **Template version** | tech-design-template/v1 |

## 0. Read to rule — the human reviewer's section (~9 min)

*The body is the contract; this section only routes attention — on any conflict the body governs; machines ignore it.*

**Judge these**

| # | The call | Could have gone | If you disagree |
|---|---|---|---|
| 1 | Exact green creates one durable obligation and implementation terminal before claim/width release; review begins only after both releases | sweep-end review or review inside Worker (`#ALT-1`, `#ALT-2`) | you accept drift or scarce-resource retention → §3.3, #DINV-3 |
| 2 | Impl-review allocates the process ID that implement-now adopts as its attempt ID; implement-now separately mints the protected post-worker proof; completion requires that identity, impl-review's receipt, EVT-13, and reciprocal EVT-17 | invent a second attempt ID, accept any matching comment, or infer a cross-contract join (`#ALT-3`, `#ALT-4`) | you accept two identity authorities or worker-forgeable/unrelated terminal records → §3.4, §3.7, #DINV-8 |
| 3 | Obligations and attempts are immutable events on the canonical evidence ref | process-local state or mutable tracker fields (`#ALT-5`) | interrupted review can disappear on the next sweep → §3.4, #DINV-4 |
| 4 | Review uses a separately bounded, read-only disposable clone of the exact revision | operator clone or implementation worktree (`#ALT-6`) | a reviewer may mutate live/shared repository state → §3.6, #DINV-7 |
| 5 | Installed review backends are typed and authenticated before any task dispatch; attempt failures remain orthogonal to implementation success | lazy setup or task failure (`#ALT-7`) | green work can be spent before a known setup failure or rewritten red → §3.2, §3.9, #DINV-10 |

**Sanity-check the numbers** — one obligation per exact task/proposal/revision (#DINV-4); one active attempt per obligation (#DINV-5); one setup/auth per distinct effective backend per run (§3.2); review capacity is a separate positive configured bound (§3.5); every admitted attempt has exactly one EVT-15 lifecycle terminal, and every publication moment has exactly one field-identical EVT-13↔EVT-17 pair (§3.9).

**The author's open questions** — none — configured capacity, evidence-ref name, live catalog cells, and retry budget are closed data owned by their existing configuration/policy surfaces (§7).

**Skip the rest** — impl-review judgment process → sibling review design; evidence CAS/producer proof → `routing-evidence-v1`; native catalog contents → live adapter policy; rationale → §4; implementation defects → tests and release evidence in §6.

## 1. Problem

The prior composition made a successful implementation wait inside its claim and implementation-width slot for review. Moving review to a sweep-end batch released those resources but introduced a whole-sweep head-drift window, delayed every task terminal, and left interrupted review only in process memory. Its artifact verifier accepted a matching proposal/revision without proving that the artifact came from the launched post-worker attempt, so the implementing worker could satisfy its own independent-review gate by prewriting or replaying evidence.

Setup and failure semantics were also conflated. An absent review block could install a reviewer, the configured review backend and artifact broker could escape preflight, setup could fail after implementation spend, and a review factory failure could fail the implementation task. Run redness was therefore neither durable nor diagnostically useful. The successor must preserve the proven implementation path while making review a durable, separately owned obligation whose success is authenticated artifact evidence rather than exit status.

## 2. Decision

**Decision:** Implement-now commits one exact-revision `review-obligation/v1` beside the succeeded implementation outcome while the task claim is still held, then releases the claim and implementation capacity before admitting that obligation to a separate review frontier. Each review attempt uses one pre-authenticated effective backend, the obligation's verified semantic demand, a fresh read-only disposable workspace, and the exact process identity allocated by impl-review, which implement-now records as `review_attempt_id`. Implement-now separately mints unpredictable post-worker proof bound to that identity. Completion requires impl-review's exact native receipt, the distinct protected proof, and one verified field-identical EVT-13↔EVT-17 pair. The code-host author may be the same as or different from the implementation identity and does not participate in verification.

Review state is append-only canonical evidence. Review failure never rewrites the implementation class: it emits a typed EVT-15 terminal, leaves the obligation pending, makes EVT-16 and exit status red, and is recoverable by a later driver-selected review-only invocation. Configuration/setup failure before new work creates no obligation; the same failure during selected recovery names each existing obligation and leaves it pending without inventing an attempt.

## 3. The design

### 3.1 Carried implementation composition

The Dispatcher remains the sole frontier walker and bounded implementation scheduler. The Worker remains context-first and single-task over consumer-owned WorkTracker, Lease, Workspace, AgentRuntime, CodeHost, VerificationGate, e2e basis/hermeticity/mandate, and green-driver seams. Demand is exact-verified before native resolution; a resolved implementation route is durably admitted before claim; work starts from freshly fetched mainline in an isolated workspace; every model process is fresh and bounded; a proposal is succeeded only from an exact green artifact; the executor never merges.

The review change does not reintroduce `TierMap`, an implicit backend, a hardcoded native model, or a second implementation success predicate. The e2e pre-claim design, deterministic head branch, duplicate-proposal guard, bounded recovery, and canonical routing evidence remain binding as restated here and verified through §6's successor conformance manifest. Direct activation, fallback, and invalidation are carried explicitly in §3.16 rather than delegated to the now-superseded target. All implementation-only result fields remain on EVT-10; EVT-15 never changes them.

### 3.2 Typed configured-review admission

**Decision:** Configuration parsing distinguishes an absent `executor.review` block from a present block whose optional `backend` is empty. Absence returns `ReviewInstall{Enabled:false}` and performs no build, auth, catalog read, or model work. Presence creates a closed `ReviewInstall` with a positive independent width, attempt budget, and either one explicit backend or inherited backend selection.

Before constructing any backend union, the composition exact-reads and completes every in-scope `pair_verified` lifecycle tail through only `ReviewObligationStore.Finish` and EVT-15 append/exact-read; those members contribute no backend and cannot receive a setup-failure terminal. After that repair, the applicable implementation/ordinary-pending review frontier is known and the composition computes a union of distinct effective review backends. In a normal invocation, every exact `handoff_pending` obligation being repaired contributes its persisted backend and every new ready task contributes the current explicit review backend, or its already-selected implementation backend when the field is absent; pre-existing ordinary `review_pending` obligations do not contribute or enter this path. In a review-only recovery invocation only selected members still ordinary-pending after lifecycle repair contribute; unselected pending obligations are not read, preflighted, routed, or attempted. Thus a config change never switches a frozen obligation: selected pending backend A plus current explicit backend B preflights both when both work classes are present. For each distinct name the composition calls one typed factory and one auth preflight. The factory returns the runtime, review-role catalog provider/builder/capture interfaces, impl-review's process coordinator and host persistence adapter defined by `impl-review-host-persistence`, and the executor-side protected-proof/reciprocal-terminal verifier; no `any`, variadic positional constructor, silent type switch, always-pass legacy verifier, executor-side artifact parser, or executor-owned proposal publisher exists.

```go
type ReviewSetupFactory interface {
	Build(ctx context.Context, backend string) (ReviewService, error)
}

type ReviewService struct {
	Backend     string
	Runtime     ReviewRuntime
	Processes   implreview.ProcessCoordinator
	Persistence implreview.HostPersistenceAdapter
	Workspace   ReviewWorkspace
	Verifier    ReviewTerminalVerifier
}

type ReviewAuthenticator interface {
	Authenticate(ctx context.Context) error
}

type ReviewRecoveryRequest struct {
	DriveRoundID  string
	ObligationIDs []string
}

type ReviewTerminalVerifier interface {
	VerifyAttemptProof(ctx context.Context, obligation ReviewObligation, processID, proofRef string) (implreview.VerifiedPublicationBinding, error)
	VerifyPair(ctx context.Context, publication implreview.PublicationTerminal, host EVT17) error
}
```

`ReviewRecoveryRequest` is either absent (normal invocation) or closed-required review-only input. Its round ID is non-empty and its obligation IDs are non-empty, unique, in-scope, exact-known, and in `pending|pair_verified`; empty, duplicate, unknown, terminally verified, or out-of-scope input fails before backend preflight and never widens to a repository scan. A selected `pair_verified` member takes the lifecycle-only repair path before backend preflight and performs no model/persistence-medium work. Authentication for ordinary pending work proves that the configured runtime and impl-review persistence adapter are ready, including a side-effect-free proof of current proposal-medium read/create permission and durable publication/event-store access. It does not mint a completion identity or require the adapter's code-host account to differ from any implementation account. An adapter may expose its authenticated actor as optional operational telemetry, but that value never enters the verifier predicate.

Any factory, runtime-auth, code-host-auth, catalog, or config failure exits 1 with zero task terminals, claims, implementation launches, and reviewer launches; no native auth exit code escapes. For affected new work it emits one run-level EVT-15 `phase=preflight`, `failure_class=infrastructure`, `obligation_status=not_created`. For each affected selected obligation that remains ordinary-pending after lifecycle repair it emits an obligation-scoped EVT-15 with `drive_round_id`, `obligation_status=pending`, and no attempt ID. A repaired `pair_verified` member already has its one verified EVT-15 and never receives this failure projection. A mixed frontier emits both projections as applicable without fabricating work.

### 3.3 Exact-green handoff and release ordering

**Decision:** A Worker invokes a typed `GreenHandoff` after exact green and after its implementation process group is quiesced, but while its claim remains held. Before the handoff, the CodeHost adapter attaches the VerificationGate's immutable revision-bound verdict to the proposal revision and exact-reads it; this `implementation-green-capture/v1` native evidence is the crash-recovery source and is never inferred from a mutable head. The handoff receives identities already proven in that attempt—task, demand, implementation attempt, proposal, exact verified revision, semantic profile, effective review backend, and implementation terminal—then (when review is configured) persists/exact-reads the obligation and only afterward appends/exact-reads the existing EVT-10 routed-outcome composite. An unconfigured run appends EVT-10 alone.

```go
type GreenCandidate struct {
	TaskID, DemandID, ImplementationAttemptID string
	ProposalRef, Revision, Backend             string
	RequestedProfile                          executiondemand.ProfileMinimum
}

type GreenHandoff interface {
	Commit(ctx context.Context, candidate GreenCandidate) (ReviewObligation, error)
}

type ReviewObligationStore interface {
	Create(ctx context.Context, obligation ReviewObligation) (ObligationProof, error)
	Recoverable(ctx context.Context, repository string) ([]ReviewObligation, error) // pending|pair_verified
	ExactSelected(ctx context.Context, repository string, obligationIDs []string) ([]ReviewObligation, error)
	Begin(ctx context.Context, obligationID, attemptID, runID string) (AttemptProof, error)
	Finish(ctx context.Context, terminal ReviewTerminal) (TerminalProof, error)
}
```

The ordered observable sequence is:

```mermaid
sequenceDiagram
  participant W as Worker
  participant E as Evidence
  participant C as Claim
  participant I as Implementation slot
  participant R as Review frontier
  W->>W: verify exact proposal revision green
  W->>E: attach/exact-read immutable green capture
  W->>W: quiesce implementation process group
  W->>E: obligation append/exact-read
  W->>E: EVT-10 append/exact-read
  W-->>C: return; deferred release completes
  C-->>I: runOne returns; implementation slot releases
  I->>R: enqueue exact persisted obligation
  R->>R: acquire independent review slot
```

The Worker's claim-release defer completes before `Run` returns; the Dispatcher releases its implementation semaphore before enqueueing. A trace recorder used by conformance tests observes all six edges. The reviewer never receives a `port.Task` with an implementation head branch and never runs through `runtime.AssembleBrief`'s implementation delivery protocol.

If obligation persistence cannot commit after its bounded CAS retry, the immutable green capture preserves implementation-success evidence but no succeeded EVT-10 is projected from an absent canonical terminal. The run emits EVT-15 `phase=obligation`, `obligation_status=not_created`, `failure_class=infrastructure`, releases both resources, exits 1, and the next combined-frontier scan discovers the captured-but-unterminalled proposal through a typed batched CodeHost read. If EVT-10 append fails after obligation persistence, the obligation remains in a `handoff_pending` reducer state and is not review-eligible. A later invocation reconstructs either case only from the exact attached VerificationGate verdict and exact revision checks—never from current head—then idempotently creates the obligation if absent and appends EVT-10 before any review launch. No cross-store atomicity is assumed.

- **DINV-1** — Exact green is captured once from verified attempt evidence; review never re-resolves a branch or proposal head.
- **DINV-2** — No review launch occurs until the implementation process group is terminated/reaped and its capability is revoked.
- **DINV-3** — Every review launch occurs after its task claim and implementation-width slot are both observably released.

### 3.4 Obligation and attempt evidence

**Decision:** `review-obligation/v1`, `review-attempt/v1`, and `review-terminal/v1` form a separate closed `implement-now-review-state/v1` event family behind `ReviewObligationStore`. Its production adapter uses one configured fetched git ref, the canonical identity/encoding and producer-proof primitives supplied by `routing-evidence-v1`, immutable identity-derived files, fast-forward compare-and-swap, exact-read proofs, and fresh-clone reduction. It does not add an undeclared event kind to the sibling's closed `routing-evidence/v1` schema, duplicate EVT-10, or authorize review from a report projection. The ref name and sharding are repository configuration/implementation data, not literals in this design.

The obligation identity is the domain-separated digest of repository, task ID, demand ID, implementation attempt ID, proposal ref, exact revision, semantic profile, effective backend, and review-policy generation. Its state is derived: `pending` from one valid obligation without a matching verified EVT-13↔EVT-17 publication pair; `pair_verified` from that authenticated pair plus its exact-readable artifact while the lifecycle terminal is incomplete; and `verified` only after the same attempt also append/exact-reads its canonical `review-terminal/v1` and projects exactly one verified EVT-15. EVT-15 cannot clear an obligation without the pair, and the pair cannot make the obligation terminally clean without EVT-15. Failures append attempts and terminals but never mutate or clear pending.

An attempt is admitted only for a pending exact-read obligation with no active attempt. After #DINV-2 and #DINV-3 hold, the executor asks impl-review's `ProcessCoordinator` to CAS-allocate or resume the exact subject generation and exact-reads its authoritative `review_process_id`. The executor adopts that value unchanged as `review_attempt_id`; concurrent callers receive the same unterminated process identity, and a retry after a terminal receives the next linked generation. It then reads cryptographic randomness, derives protected proof bound to obligation/process/proposal/revision, destroys the raw nonce, and persists only the non-secret `attempt_proof_ref`. Duplicate attempt admission against the same process is non-launching; a different active process for the obligation is an integrity failure. Every paired EVT-13/EVT-17 repeats the process/attempt value field-identically. Impl-review separately allocates `publication_attempt_id`, which has no executor alias and never substitutes for the obligation or review attempt.

Each attempt reaches exactly one canonical `review-terminal/v1`, from which EVT-15 is projected. Each completed-report publication moment also reaches one impl-review EVT-13 and one implement-now EVT-17. A verified review terminal references the obligation, attempt, review route/outcome evidence, exact native receipt, distinct attempt-proof reference, parsed report digest, and verified terminal pair. A failed terminal leaves the obligation pending. Canonical idempotence makes replay of the same terminal a no-op; a different terminal for the same attempt is an integrity error. A landed EVT-13 with a missing EVT-17 is replayed into EVT-17 from the durable receipt/result and never republished. A landed verified pair with a missing review terminal remains `pair_verified`, stays discoverable by both combined and exact-selected recovery, append/exact-reads the same verified `review-terminal/v1`, and projects the same EVT-15 without launching or publishing again. The reducer admits at most one attempt for an obligation in one invocation, even after that attempt fails; retry requires a later run ID and a fresh attempt identity.

- **DINV-4** — One exact task/proposal/revision/policy tuple has one obligation identity and one derived state on the canonical evidence ref.
- **DINV-5** — One obligation has at most one active attempt and each admitted attempt has exactly one terminal.
- **DINV-6** — Repository code enters only the disposable reviewer repository snapshot; raw attempt entropy, credentials, prompt, and raw model output never enter obligation/attempt/event payloads, repository files, or proposal artifacts. Only admitted process inputs enter reviewer execution and one closed `ProcessHandoff` leaves it; the non-secret proof reference crosses only the trusted host→persistence/evidence seam, and raw proof material stays host-only.

### 3.5 Immediate separate-capacity scheduling

The Dispatcher owns two pools: implementation width and configured review width. When one green handoff returns and both implementation resources are released, its exact-read obligation is offered immediately to the review pool; it does not wait for other implementation tasks. Review saturation backpressures only the review queue. Implementation tasks continue until their own pool drains.

At normal invocation start, the handoff-recovery reader reconstructs exact `handoff_pending` rows, appends/exact-reads their missing EVT-10 terminals, and also completes every exact `pair_verified` lifecycle tail before considering backend setup. It admits only those handoff-repaired obligations plus obligations created by newly green work to the normal review frontier; it never scans or attempts an ordinary pre-existing `review_pending` obligation. At review-only recovery start, `ExactSelected` validates the closed request and only its members enter the frontier; `pair_verified` members complete their lifecycle tail first and are removed from the model-work set, validation failure stops before preflight, and no repository-wide pending scan occurs. Eligibility always requires exact-read proofs for the obligation and referenced succeeded EVT-10. One run may admit at most one attempt per obligation; a failed attempt remains pending for a later driver round rather than re-entering its current queue. A review-only run performs lifecycle repair and, for still-pending members, setup, routing, workspace, launch, verification, events, and final reporting, but zero implementation route, claim, Worker, proposal open/update, or implementation model action. The run finishes after the selected review frontier settles; unselected pending obligations neither block its terminal nor become clean by implication.

Cancellation stops new admission, kills and reaps owned reviewer process groups under a fresh cleanup context, appends an infrastructure terminal for every admitted attempt, and leaves their obligations pending. Queue entries without an admitted attempt remain pending without a fabricated terminal.

### 3.6 Review demand, route, workspace, and launch

**Decision:** A review attempt resolves `role=review` through only its effective backend's current qualified catalog. Its requested minimum is the exact `ProfileMinimum` retained from the verified execution demand, including the assurance band; normal least-cost-satisfying resolution selects exact or promoted capacity. The executor never supplies a weakest placeholder, `strongest_satisfying` as the normal mode, a numeric rung, another backend's policy, or an implementation-only native control.

Qualification is checked against the invocation repository and operator-authorized current policy data. Adapter source may ship decode fixtures, but no compiled repository slug or wall-clock validity cutoff is authorization. Expiry/invalidation is an operator policy input whose trusted clock is injected. The selected model, effort, and controls pass through the adapter's typed native builder and launch-capture equality check and then into `LaunchSpec`; surplus `worker_turns`/`max_turns`, implementation head branch, and implementation delivery protocol are rejected.

After review-route resolution, `ReviewWorkspace.Prepare` creates a fresh private temporary clone, fetches the obligation's exact revision by object ID from the repository-configured remote, verifies object identity, and detaches it from the operator clone and every implementation/reviewer directory. The repository tree is permissioned read-only for the reviewer. The launch environment redirects code-host configuration, removes code-host tokens, SSH-agent access, Git credential redirections, and repository path overrides, while retaining only the selected model provider's ordinary authentication path. Cleanup kills/reaps the process group, verifies the repository tree unchanged, and removes only that validated temporary root. Non-default remote fixtures exercise the typed repository configuration required by AP-30.

The review-only brief names the proposal, exact revision, obligation ID, exact review-process identity, and process-handoff grammar. It explicitly forbids implementation delivery, repository mutation, and direct proposal writes. The reviewer receives no code-host credential, canonical evidence authority, persistence-store path, or host-call endpoint: its one permitted artifact handoff is exactly one bounded typed terminal-output payload. After the reviewer is reaped and the workspace is proven unchanged, the host wrapper decodes that untrusted payload into the closed `ProcessHandoff` without treating a clean exit as completion. Implement-now first calls `ReviewService.Processes.Complete(admittedProcess, processHandoff)`; the trusted coordinator validates the binding and complete-set shape, append/exact-reads route-outcome, pass, and process evidence, and returns the canonical `AggregateHandoff`. Before publication, `ReviewTerminalVerifier.VerifyAttemptProof` resolves the proof against protected obligation/attempt state and returns impl-review's typed `VerifiedPublicationBinding`; a proof failure publishes nothing and takes `artifact_untrusted`. Only that verified binding and canonical handoff are passed to `HostPersistenceAdapter.Complete`, which performs the sole medium write under its durable idempotency fence, returns the native receipt, exact-reads and validates only that receipt, and emits EVT-13. Implement-now mirrors the returned terminal into EVT-17 and then checks only reciprocal equality through `VerifyPair`; it never parses the artifact or publishes it. Any wrapper text, zero/surplus/malformed/mismatched process handoff, process-completion evidence failure, protected-proof failure, write ambiguity, or receipt mismatch fails closed. Inability to enforce the filesystem/process sandbox, read-only repository, credential-redacted environment, terminal-output boundary, or closed review launch controls makes the attempt infrastructure-failed.

- **DINV-7** — A reviewer can read only its exact disposable repository snapshot and allowlisted model-provider authentication, and can hand the host only one bounded `ProcessHandoff`; only the trusted coordinator can turn it into a canonical `AggregateHandoff`, and the reviewer holds no proposal, persistence-store, canonical-evidence, or arbitrary host-call capability.

### 3.7 Authenticated artifact reconciliation

**Decision:** Process exit never verifies review. After a clean, non-truncated reviewer exit and successful `ProcessCoordinator.Complete`, implement-now accepts only the typed impl-review persistence result for the bound obligation/process/proposal/revision. The adapter is the single owner of artifact rendering, CRLF normalization, schema/render/binding validation, and exact-read of the native object named by its returned receipt; implement-now performs no artifact parsing, proposal-wide pagination, body search, or author selection.

The executor-side `ReviewTerminalVerifier` requires only:

1. before publication, a distinct `attempt_proof_ref` that resolves against protected canonical obligation/attempt evidence outside every child write surface and yields a typed binding for impl-review;
2. after publication, exactly one EVT-13 and one EVT-17 with equal status/failure class and all seven FR-49 field pairs equal in presence, grammar, and value;
3. verified status plus `artifact_ref`, `native_receipt`, and `attempt_proof_ref` present on both terminals.

Impl-review alone classifies malformed, absent, or process/proposal/revision/digest-binding-invalid handoff or exact-receipt content as `artifact_unverified`; implement-now preserves that class unchanged in EVT-17 and EVT-15. Protected-proof failure before publication or a conflicting/orphaned/duplicate reciprocal terminal is `artifact_untrusted`; the former emits no publication terminal, while the latter makes the observed EVT-13/EVT-17 records non-paired and lifecycle-failed without changing either immutable record. Missing terminal transport/append, capability/auth/store/write/read transport, acknowledgement ambiguity, runtime, or bounded reconciliation is `infrastructure`. No concrete mismatch is classified by both components: process-handoff and exact-object artifact validity belongs to impl-review; protected proof and reciprocal-event consistency belongs to implement-now. An unrelated or duplicate comment is never a candidate because no list/search path exists. Code-host author is retained as telemetry only.

The accepted receipt, attempt-proof reference, artifact digest, EVT-13 reference, and EVT-17 reference are stored in the canonical verified terminal. A later comment deletion or replica lag cannot erase that already exact-read terminal; a fresh verifier revalidates the stored object and immutable pair according to evidence-retention policy.

- **DINV-8** — A verified obligation has one schema-valid exact-revision artifact at impl-review's exact native receipt, protected proof bound to the launched attempt, and one field-identical verified EVT-13↔EVT-17 pair; code-host author identity is not part of this predicate.
- **DINV-9** — A prewritten, receipt-substituted, stale, replayed, malformed, terminal-mismatched, or merely process-successful artifact never verifies review; same-author and different-author transport behave identically when receipt and proof match.

### 3.8 Recovery and attempt fencing

The combined frontier exact-reads canonical obligations before consulting proposal projections. A pending obligation retains its captured revision, demand, backend, and policy generation; recovery never reads the current branch/head to reconstruct them. A valid active attempt is reattached only with the runtime-fence proof defined by `routing-evidence-v1`; otherwise bounded reaping appends its infrastructure terminal before a new attempt is admitted. Attempt IDs are never reused across retries.

If the process dies before obligation persistence, handoff recovery uses the already attached exact green/codegate evidence named in §3.3. If it dies after obligation persistence but before EVT-10, the reducer exposes `handoff_pending` and recovery appends the exact existing implementation terminal before review eligibility. If it dies after EVT-10 but before queueing, the next combined frontier finds the pending obligation. If it dies after launch but before `ProcessCoordinator.Complete`, fencing determines reattach/kill/terminal before retry; if it dies during coordinator completion, recovery exact-reads the same admitted process and completes or replays its canonical process terminal before any publication call. If it dies after publication receipt persistence but before EVT-13 or EVT-17, reconciliation exact-reads the same impl-review-receipted attempt-bound artifact and replays only the missing event append; it performs no second proposal write. If it dies after the verified EVT-13↔EVT-17 pair but before `ReviewObligationStore.Finish`, after `Finish` but before EVT-15, or during EVT-15 append, the reducer exposes `pair_verified` rather than ordinary pending: recovery append/exact-reads the same verified review terminal and same EVT-15, performs zero launch/publication, and gives this completion precedence over active-attempt infrastructure reaping. An acknowledgement-indeterminate GitHub write remains non-accepting and is never retried under the same publication identity. Thus no crash point can produce a second implementation dispatch, duplicate publication, lost lifecycle terminal, or launder pending review into exit 0.

The unattended driver consumes `review-wake-state/v1`, the exact join specified by the self-drive recovery contract between this reducer's obligations/provider attempts and the consumer-owned append-first `review-recovery/v1` ledger. Provider attempt count is observational only; consumer `recovery_epoch`, `recovery_count`, and frozen `recovery_bound` meter every admitted review-only invocation, including setup failure that creates no provider attempt. `age_s` derives from consumer `first_observed_at_utc`, never an undeclared provider timestamp or git metadata.

For one durable driver round, the consumer selects every eligible in-scope obligation, append-first increments each recovery count, and submits one non-empty `ReviewRecoveryRequest`. Every selected terminal carries the round ID; the provider never scans an unselected or exhausted obligation. An all-exhausted or whole-store-unknown round launches no executor and remains red through the consumer's immutable evidence/event path. Pull cadence, rearm, exhaustion evidence, and push transport remain owned by the companion state-reader/watch designs; they must supersede the frozen dependents before §3.16 activates.

### 3.9 Events, failure classes, and exit semantics

EVT-10 remains the canonical implementation terminal. EVT-15 is the broader review-lifecycle event `impl_review_terminal`; EVT-17 is the executor's `review_artifact_publication_terminal` for the same publication moment as impl-review EVT-13; EVT-16 remains `run_finished`. None is smuggled onto serialized `task_outcome`. Report package comments, JSON discriminators, operator sink, argus sink, escalation flag, summary, and exit mapping use those live names and schemas.

EVT-15 carries exactly the effective PRD's fields and phase enum; it adds no undeclared demand, usage, class, receipt, or discriminator value. Its optional `drive_round_id` joins selected recovery decisions; `review_attempt_id` joins the separate canonical review routing/outcome evidence that owns demand and usage. Its `artifact_proof_ref`, when present, equals EVT-17 `attempt_proof_ref`, never carries a native receipt, and never substitutes for EVT-17. New-work setup failure omits task/obligation/attempt fields and uses `obligation_status=not_created`; selected-recovery setup failure names round/task/obligation/proposal/revision/backend, omits attempt identity, and uses `obligation_status=pending`.

Whenever impl-review emits EVT-13, the executor append/exact-reads exactly one EVT-17 with the same status and failed failure class. `review_process_id=review_attempt_id`, both obligation IDs are equal, `proposal_id=proposal_ref`, `reviewed_revision=revision`, and artifact/receipt/proof have identical presence and values; all three evidence fields are present when verified. Impl-review's `publication_attempt_id` has no executor alias. Missing, duplicate, orphaned, grammar/value/presence mismatch, status/class disagreement, or receipt/proof overload is non-evidentiary and leaves the obligation pending.

The three failure classes are disjoint:

| Failure class | Evidence meaning | Obligation | Operator action |
|---|---|---|---|
| `artifact_unverified` | impl-review rejected the process handoff or exact receipted artifact as missing, malformed, surplus, or binding-invalid | pending | retry/reconcile artifact production |
| `artifact_untrusted` | protected attempt proof failed before publication, adapter identity state conflicted, or the reciprocal terminal pair was substituted/mismatched | pending | investigate substitution/replay/event drift, then reconcile or start a fresh attempt |
| `infrastructure` | setup/store/routing/launch/runtime/truncation/cancel/read service failed | pending if created; otherwise not-created | repair named infrastructure, then retry |

A failed review sets EVT-15 `escalated=true`, increments exactly its class, `review_failed`, and (when an obligation exists) `review_pending`; it exits 1. A verified EVT-15 increments `review_completed`, but only the conjunction of a verified EVT-13↔EVT-17 pair, its canonical verified `review-terminal/v1`, and that exact EVT-15 clears pending. Implementation succeeded/failed/blocked counts never change because of review. Review setup or attempt failure never returns a native backend exit code and never becomes a task error. An absent review block emits no review terminals/counts and preserves the implementation-only exit contract.

- **DINV-10** — Review setup/attempt state is orthogonal to the implementation class yet cannot be invisible to run summary, escalation, exit status, or the reciprocal publication-evidence corpus.

### 3.10 Boundaries and non-goals

- **Not solved here** — review lens prompts, blind-session topology, pass count, finding screening/aggregation, proposal-medium idempotency mechanics, model catalog contents, git evidence CAS implementation, retry timing policy, or merge/landing.
- **Owned elsewhere** — `impl-review-host-persistence` owns report content, process verdict, sole publication, native receipt, EVT-13, and publication recovery; native routing owns catalog/qualification semantics; routing evidence owns route/outcome append/read/fencing; invocation adapters own pull/push cadence while consuming this design's normalized pending-review readiness protocol.
- **Implementation must not infer** — revision from a branch, completion identity from code-host author/config text/runtime exit, review demand from a numeric rung, success from exit/comment presence, installed config from an empty backend string, or recovery from proposal head/check colour.
- **Must remain unchanged** — one plan→tracker projection, external-fleet claims, e2e schemas/admissions, implementation success truth, argus principal/hands split, no executor merge credential, and human landing authority.

### 3.11 Invariants

- **DINV-11** — Empty combined frontier performs zero review build/auth/routing/model work; absent review configuration installs no seam.
- **DINV-12** — Every selected review launch exactly matches one qualified review-role catalog row on its persisted effective backend and semantic demand.
- **DINV-13** — Every durable state transition is reconstructable from a fresh clone of canonical evidence; a favorable projection or next invocation cannot override pending/untrusted state.
- **DINV-14** — One task failure never stops independent implementation/review work, but a configured review failure keeps the run non-zero.
- **DINV-15** — No implementation/reviewer process, workspace, claim, capability, or semaphore slot survives its owner; cleanup targets exact owned identities, never a recycled PID or broad path.
- **DINV-19** — A review-only recovery invocation is authorized only by one closed non-empty selected obligation set and never reads, preflights, routes, or attempts an unselected obligation.

### 3.12 Observability

- **Events/logs** — EVT-10, EVT-15, EVT-16, and EVT-17 carry run/driver-round/task/demand/implementation-attempt/review-obligation/review-attempt/proposal/revision/backend/route/artifact/receipt/proof correlations; EVT-17 exact-joins impl-review EVT-13. Optional authenticated-actor telemetry is observational only. Structured logs name every selection, setup, release, queue, launch, publish, reconcile, event-pair, and cleanup boundary without secrets.
- **Metrics** — setup/auth by effective backend, handoff latency/failure, pending age, review queue depth/width, route class/profile, launch fidelity, artifact visibility retries, three failure counts, completion latency, usage, and recovery count.
- **Alerts** — untrusted artifact, parent/workspace mutation, launch before release, missing attempt terminal, overdue pending obligation, setup outage, evidence corruption, or cleanup failure alerts the operator/argus.
- **Debugging surface** — the attended report and fresh-clone verifier render the exact obligation chain, release trace, native route, workspace identity/digest, impl-review receipt, attempt proof, EVT-13↔EVT-17 comparison, artifact disposition, and recovery recommendation.
- **Trace/correlation** — driver round→selected obligation→run→task→demand→implementation attempt→proposal revision→review attempt/process→native route→artifact→EVT-13↔EVT-17→EVT-15→EVT-16 joins end to end.

### 3.13 Security & privacy

The threat actors are repository contributors, the autonomous implementing worker, malicious repository content executed/read during review, a stale/replayed reviewer, and a process outside its attempt. The protected assets are operator/implementation checkouts, impl-review's host persistence credential/state, routing policy, canonical evidence producer authority, review independence, and run-control truth.

The implementing process receives no review attempt identity or proposal-artifact capability; the identity is minted only after its entire process group is gone. The reviewer receives the derived non-secret identity inside a read-only disposable snapshot but receives no code-host credential, host-store path, evidence key, or write endpoint. The host accepts only its bounded terminal output and invokes impl-review's persistence adapter, which pins verification to its exact native receipt; protected attempt/revision binding and the EVT-13↔EVT-17 pair defeat prewrite, receipt substitution, replay, and inferred joins. The adapter's code-host account may be the same account used elsewhere on the engineer's machine: identity separation is deliberately not a completion factor. Credentials and raw proof never enter briefs, repository files, events, or logs. Exact temp-root validation, filesystem/process sandboxing, read-only enforcement, environment redaction, process-group ownership, post-run workspace comparison, and cleanup prevent repository mutation and cross-attempt bleed. Any inability to prove these controls is infrastructure failure, not best-effort review.

### 3.14 Test strategy

Core identity, reducer, scheduling, failure classification, report/event codecs, and exact-receipt selection are in-process and table/property tested. A local bare git remote and temporary clones are local-substitutable for evidence, proposal revision, read-only workspace, process-group cleanup, moving-head, and crash recovery. Impl-review's persistence/receipt surface and each agent CLI are remote-but-owned ports with hermetic transcript adapters plus the same conformance suite; one authenticated dogfood run supplies production evidence.

The release matrix includes:

- an ordered trace proving exact green → obligation+EVT-10 → claim release → implementation-slot release → review admission, plus a width-saturation case proving immediate per-task review and independent capacities;
- absent config, present-empty inheritance, explicit backend, mixed inherited backends, empty combined frontier, review-only frontier, and every setup failure, all asserting factory/auth call counts and zero forbidden work;
- closed recovery-request validation for empty, duplicate, unknown, verified, out-of-scope, selected, unselected, and exhausted IDs; one multi-obligation invocation per driver round; selected-pending preflight failures carry round/obligation identity and no attempt;
- every supported backend × derivable assurance-band review route, non-pyxis repository, injected future clock, model/effort/control propagation, launch capture, and no worker-only control;
- exact revision under a moving head; same-author and different-author transport, worker prewrite, native-receipt substitution, replay, wrong proof, CRLF, malformed/foreign comments, duplicate bodies, visibility lag, acknowledgement ambiguity, and exact-read transport failure;
- parent/implementation/reviewer workspace byte digests, mutation denial, review-width cap, process-group reaping, cancellation, truncation on every backend, and PID-reuse-safe cleanup;
- crash injection before/after handoff append, EVT-10, both releases, attempt admission, launch, `ProcessCoordinator.Complete`, publication fence/write/receipt/exact-read, EVT-13, EVT-17, `ReviewObligationStore.Finish`, and EVT-15, followed by review-only recovery with zero implementation work, lifecycle loss, or duplicate proposal write;
- exact EVT-13/EVT-17 pair, EVT-15 lifecycle, and EVT-16 wire goldens; required/optional/status/failure-class/grammar/value mismatch truth table; both sinks, escalation, three class counts, review-pending durability, implementation-count preservation, and exit 1/0 mapping.

### 3.15 Compatibility and cutover

No proposal report-body fork is required: impl-review retains its v2 aggregate/report schema while the host binding and publication terminals carry the obligation, process, receipt, and proof identities. Existing unauthenticated comments never auto-verify; an obligation created for an already-green proposal uses its authoritative captured green revision and a fresh attempt/process. Old process-local review results are non-authoritative.

The new obligation/review-event generations and `impl-review-host-persistence` land production-inactive and pass §3.14 plus all live blocking DoD rows. One fresh-main human-merged activation replaces sweep-end review, parent-clone launch, child/caller publication, generic `task_outcome` review fields, untyped factories/constructors, compatibility-ladder review routing, inferred event joins, and unauthenticated verification together. There is no shadow/canary/dual success predicate and no fallback to the old seam. A defective native mapping may use only the strongest qualified profile on the already-selected backend/role when the governing policy explicitly requests fallback; otherwise it fails before launch.

### 3.16 Activation proof, fallback, and defect response

**Decision:** One closed `routing-activation-proof/v2` manifest is committed on main before activation. It pins the exact candidate mainline object; compatible demand, native-policy, routing-evidence, implementation executor, review-state, impl-review process/persistence, event, driver-consumer, and adapter generations; effective catalog/qualification digests; both configured evidence-ref heads and every relied-on event ID/digest, including the EVT-13↔EVT-17 schema pair; conformance/abuse/crash/dogfood/DoD evidence digests; operator qualification and release signatures; fallback/invalidation drill results; and a retired-path reachability report. Unknown, missing, stale, incompatible, unsigned, non-mainline, or cross-generation input invalidates the whole manifest.

One hermetic union predicate exact-reads that manifest, the named mainline object, both evidence refs, and every pinned proof. It requires:

- exhaustive supported backend × implementation/review role × derivable semantic-profile conformance, launch fidelity, current operator qualification, and same-backend strongest fallback;
- plan/v1 translation goldens, plan/v2 carrier identity, e2e non-regression, implementation Worker/Dispatcher/claim/workspace/process conformance, and exact EVT-7..EVT-12 outcomes;
- the §3.14 setup, handoff ordering, moving-head, isolated workspace/capacity, persistence/receipt/proof, EVT-13↔EVT-17 mismatch, three-failure-class, event/sink/exit, crash/fencing, and review-only driver matrices;
- one authenticated dogfood feature with the PRD's required implementation-profile diversity and every live blocking DoD evidence row;
- a deterministic static/runtime reachability result proving hardcoded model pins, scalar review rung/TierMap, untyped review factory/constructor, legacy verifier, sweep-end revision resolution, parent-clone reviewer, child/caller publisher, proposal-wide body/author verifier, generic task-outcome review fields, inferred terminal joins, unauthenticated acceptance, and pending-review laundering are unreachable.

Identical manifest/mainline/evidence objects produce byte-identical verdicts; timing, cost, queue latency, and usage remain measurements. Only a green predicate permits an `activation_checkpoint` bound to the manifest and exact human-merged mainline. One fresh-main human-merged config change activates every supported implementation/review/driver path together; no cohort, canary, shadow result, dual success predicate, or dormant old seam remains.

The only kill-switch behavior is an explicit policy request for the strongest qualified profile on the already-selected backend and role. It may overprovide but never weaken semantic demand, switch backend/role, reduce review, bypass attribution/isolation, or clear pending work; insufficient capacity stops before claim/launch. A qualification or generation defect stops new affected claims/reviews, append-invalidates open evidence, preserves unaffected work and all pending obligations, reruns the exact task/proposal revision through the full current path, and emits one operator remediation brief for already accepted artifacts. It never deletes history or restores the retired seam.

- **DINV-16** — Production activation is one hermetic exact-object predicate over the complete implementation/review/driver generation set; partial activation is unrepresentable.
- **DINV-17** — The activation checkpoint and human-merged mainline are the only authorities that enable the new path; proposal status or passing subsets cannot.
- **DINV-18** — Fallback stays within selected backend/role and preserves every semantic, isolation, attribution, and pending-state obligation or fails closed.

## 4. Alternatives considered

| Alternative | Why rejected |
|---|---|
| Review inside Worker until completion (`#ALT-1`) | holds the task claim and implementation width after green and couples an advisory failure to task ownership |
| Batch all reviews after the implementation sweep (`#ALT-2`) | creates sweep-long head drift, delays every implementation terminal, and loses per-task crash recovery |
| Accept any exact proposal/revision artifact (`#ALT-3`) | the implementing worker or contributor can forge the sole independent-review evidence |
| Infer publication identity from EVT-15 or similar-looking report fields (`#ALT-4`) | EVT-15 has no native receipt and is not the publication terminal; only the contracted EVT-13↔EVT-17 pair supplies reciprocal evidence |
| Store pending review in process memory, tracker labels, or a mutable file (`#ALT-5`) | next invocation can launder failure; projections are contributor-mutable or non-atomic |
| Run reviewer in operator clone or implementation worktree (`#ALT-6`) | review can mutate shared/live repository state and races sibling reviewers |
| Build/auth reviewer lazily per green task (`#ALT-7`) | a known bad configuration burns all preceding implementation work and multiplies auth side effects |
| Treat review failure as implementation task failure | rewrites a verified artifact class and encourages implementation redispatch into an open proposal |
| Sign report payload in a new private-key scheme | adds per-engineer key provisioning and rotation while impl-review's exact receipt plus protected post-worker attempt proof already closes prewrite, substitution, and replay |

## 5. Risks

| ID | Risk | Impact | Mitigation | Verification / evidence |
|---|---|---|---|---|
| R1 | Protected attempt proof becomes visible before implementation process quiescence | worker can forge its reviewer artifact | allocate/adopt process and mint proof only after #DINV-2/#DINV-3; no proof capability in implementation environment | hostile background-child/prewrite fixture |
| R2 | Code-host consistency delay hides a receipted artifact | false `artifact_unverified` and exit 1 | bounded exact-receipt visibility retry on a separate verification budget; durable pending recovery | delayed-object/replica transcript tests |
| R3 | Read-only controls differ across operating systems/backends | repository mutation or unsupported production path | one ReviewWorkspace conformance plus native launch-control equality; unsupported fails closed | mutation matrix on supported platforms/backends |
| R4 | Evidence outage blocks green handoff | green proposal lacks terminal/obligation | ordered append/exact-read with immutable green capture and typed handoff-recovery frontier; no false cross-store atomicity | every append boundary crash/outage fixture |
| R5 | Multiple effective inherited backends make setup partial | some tasks spend before a later auth failure | compute complete set and preflight each once before first dispatch | mixed-backend ordering trace |
| R6 | The same account transports unrelated comments | unrelated content mistaken for this attempt | exact impl-review receipt plus proposal/revision/process/proof, schema/render validation, and reciprocal terminals | same-author wrong-receipt/wrong-attempt/replay fixtures |
| R9 | EVT-13 lands while EVT-17 append fails | publication exists but cross-contract evidence is incomplete | replay EVT-17 from durable receipt/result; never republish; keep pending | event-boundary crash and orphan-terminal fixtures |
| R7 | Separate review pool overloads provider/code host | resource exhaustion | positive configured width, bounded queue/attempt budget, no implementation-slot borrowing | saturation and cancellation tests |
| R8 | A recovery request widens beyond the consumer's eligible set | exhausted/unselected obligations spend or race | closed non-empty selection validation before preflight; `ExactSelected`; #DINV-19 | invalid-selection and selected/unselected access traces |

## 6. Acceptance

| ID | Acceptance criterion | Verify | Required evidence | Traces to |
|---|---|---|---|---|
| A-1 | A closed `implement-now-conformance/v2` manifest maps every live requirement in this design's `Serves` and every live NR/UB/ABUSE/DoD row to at least one named executable check ID, its test binary/package identity, input fixture digest, result digest, and retained evidence reference; the manifest verifier rejects missing, duplicated, stale-generation, non-executable, or favorable-subset mappings. Its green instance proves the carried Worker/Dispatcher demand, claim, workspace, process, persistence, e2e, event-pair, recovery, and no-merge behaviors against the successor composition. | Test + Inspection | manifest schema/golden/negative corpus plus green successor conformance manifest | `prd:implement-now/DOD-30` · `prd:implement-now/DOD-33` · `prd:implement-now/DOD-36` · #DINV-11..#DINV-15 · #DINV-19 |
| A-2 | For each configured exact-green task, the obligation is append/exact-read and then EVT-10 is append/exact-read while claim is held; neither is claimed atomic across stores. Review admission occurs only after process quiescence, claim release, and implementation-slot release, and starts without waiting for the sweep. | Test + Failure injection | ordered edge trace, every append-boundary crash matrix, moving-head fixture, width-saturation fixture | `prd:implement-now/FR-40` · #DINV-1..#DINV-5 |
| A-3 | Absent config performs no review setup; `pair_verified` lifecycle tails repair before any backend work; explicit/mixed configurations then preflight each applicable ordinary-pending backend once before dispatch/attempt. New-work failure emits one run-level not-created terminal; selected persisted failure emits one pending infrastructure terminal per still-pending obligation with round ID and no attempt; both launch nothing and never fail a task. A selected `pair_verified` member completes the same verified terminal/EVT-15 even when its backend/auth is unavailable and receives no preflight failure terminal. | Test + Failure injection | closed new-work/selected/mixed config and setup matrix including backend-unavailable lifecycle repair | `prd:implement-now/FR-42` · `prd:implement-now/FR-44` · #DINV-10 · #DINV-11 |
| A-4 | Every admitted review uses its persisted backend and exact semantic demand against the qualified review-role catalog, propagates selected model/effort/controls exactly, and succeeds for non-pyxis repositories and injected future clocks without worker controls. | Test + Analysis | all-adapter × demand-domain native conformance | `prd:implement-now/FR-45` · #DINV-12 |
| A-5 | Every reviewer runs under separate capacity in a read-only exact-revision disposable clone with no parent/implementation/cross-review mutation; attempts cannot outlive owned process/capability/workspace cleanup. | Test + Security review + Failure injection | concurrency, mutation, process-group, cancellation, and cleanup report | `prd:implement-now/FR-43` · #DINV-7 · #DINV-15 |
| A-6 | Only one schema-valid report at impl-review's exact native receipt, bound to the launched process and a pre-publication protected proof, verifies. Impl-review's adapter is the sole schema/render/binding predicate; implement-now validates only proof and reciprocal terminals. Same-author and different-author transport both succeed when receipt/proof match, while prewritten, receipt-substituted, replayed, stale, malformed, CRLF, acknowledgement-ambiguous, and duplicate-body cases take the one owning predicate's specified verified or failure path without a second publication. | Test + Security review + Failure injection | artifact-authentication ownership test plus publication-boundary abuse matrix | `prd:implement-now/FR-41` · `prd:implement-now/ABUSE-6` · `prd:impl-review/FR-32` · `prd:impl-review/FR-33` · #DINV-8 · #DINV-9 |
| A-7 | Every admitted review adopts impl-review's exact process identity as `review_attempt_id` and emits exactly one EVT-15 lifecycle terminal; every impl-review EVT-13 publication terminal pairs to exactly one EVT-17 with equal status/failure class and all seven FR-49 fields. Every pre-attempt setup failure emits only the run-level or selected-obligation EVT-15 §3.2 defines. A host-invented/second process ID, missing, duplicate, orphaned, presence/grammar/value mismatch, receipt/proof overload, publication-attempt alias, or EVT-15-only publication evidence remains pending and takes the declared class. Crash fixtures immediately after EVT-17, after `Finish`, and before EVT-15 recover the same verified lifecycle terminal without launch/publication; all failures preserve implementation counts, escalate, increment only their exact EVT-16 count plus review_failed, and exit 1; only the verified pair plus its exact lifecycle terminal clears pending. | Test + Failure injection + Analysis | process-ID ownership fixture, reciprocal-terminal truth table, three lifecycle-boundary crash fixtures, and lifecycle/sink/summary/exit golden matrix | `prd:implement-now/FR-42` · `prd:implement-now/FR-49` · `prd:implement-now/EVT-15` · `prd:implement-now/EVT-16` · `prd:implement-now/EVT-17` · `prd:impl-review/EVT-13` · #DINV-10 |
| A-8 | A fresh normal invocation reconstructs handoff/pending/`pair_verified` state, while a review-only invocation validates one non-empty selected frontier, fences interrupted selected attempts, and touches no unselected/exhausted obligation or implementation path. Process-completion recovery precedes publication, and `pair_verified` completion precedes active-attempt reaping and performs only the missing review-terminal/EVT-15 appends. Invalid selections fail before preflight; selected members alone determine exit; no next round launders pending state. | Test + Failure injection | fresh-clone crash matrix including process-completion and pair-to-lifecycle boundaries, plus multi-obligation selected/unselected/exhausted/invalid request and access traces | `prd:implement-now/FR-46` · `prd:implement-now/FR-47` · `prd:implement-now/NR-10` · `prd:implement-now/UB-21` · #DINV-4 · #DINV-5 · #DINV-13 · #DINV-19 |
| A-9 | A deterministic `legacy-review-reachability/v1` checker enumerates registered dispatcher options/factories, constructors, event kinds/fields, runtime launch/persistence paths, workspace roots, revision resolvers, verifier implementations, and forbidden legacy symbols, and fails unless every old seam—including child/caller publication and inferred event joins—is absent or unreachable under the candidate activation generation; runtime branch-coverage fixtures exercise every registered review path. | Test + Inspection + Demo | checker golden/seeded-defect corpus, zero-reachable-path report, branch-coverage report, and authenticated dogfood proposal | `prd:implement-now/DOD-28` · `prd:implement-now/DOD-29` · `prd:implement-now/DOD-30` · `prd:implement-now/DOD-36` |
| A-10 | The `routing-activation-proof/v2` union predicate rejects every individually missing/stale/mismatched/unsigned generation, policy, evidence ref/event, manifest mapping, DoD result, driver result, retired-path report, or non-mainline object; identical exact inputs produce one byte-identical verdict and only an exact human-merged checkpoint activates all paths. | Test + Failure injection + Inspection | one negative fixture per manifest field plus green activation record and direct-mainline proof | #DINV-16 · #DINV-17 |
| A-11 | Every supported backend/role strongest-profile drill preserves full semantic demand, isolation, attribution, pending state, and backend/role; insufficient capacity stops before claim/launch, and a seeded qualification defect invalidates/reconciles exact affected work without restoring the retired seam. | Test + Demo + Failure injection | fallback matrix and defect-disposition drill | `prd:implement-now/FR-26` · `prd:implement-now/UB-13` · #DINV-18 |

## 7. Open questions

None. Review width, attempt/reconciliation bounds, evidence-ref name/path sharding, supported backends, catalog cells, and trusted qualification policy are closed live configuration/policy data. Changing the obligation identity/state machine, handoff order, attribution primitive, isolation boundary, failure classes, or run-control semantics requires a successor design.

## Principles

- **AP-1 — honored.** GreenHandoff, ReviewSetupFactory, ReviewWorkspace, and the executor-side proof/terminal verifier are consumer-owned; impl-review owns process coordination, artifact validation, persistence, and its medium adapters outside executor core.
- **AP-2 — honored.** Attended and argus modes consume one canonical implementation/review state and event stream.
- **AP-3 — honored.** Runtime, workspace, artifact, code-host, and evidence adapters share conformance suites.
- **AP-5 — honored.** Exact identities join implementation, release, review, artifact, recovery, and run significance end to end.
- **AP-6 — honored.** Obligations, attempts, setup, events, native launch, and artifacts use typed closed versioned data; existing report schema is reused deliberately.
- **AP-10 — honored.** Setup, route, attribution, state reduction, failure class, and exit are mechanical predicates.
- **AP-11 — honored.** One implementation truth, one obligation reducer, and one review-failure classifier exist.
- **AP-12 — honored.** Claims, slots, process groups, workspaces, capabilities, obligations, and attempts have explicit owners/reapers.
- **AP-13 — honored.** Implementation and review success require exact verified artifacts, never exits, badges, or comment presence.
- **AP-15 — honored.** Immutable idempotent evidence and fencing converge every interruption without duplicate implementation or review attempts.
- **AP-16 — honored.** Implementation and review capacities, attempts, visibility retry, CAS retry, and cleanup are mechanically bounded then escalated.
- **AP-17 — honored.** Setup fails before dispatch; route/isolation fails before review launch; artifact distrust fails before verification.
- **AP-19 — honored.** Absent config and empty combined frontier spend zero review inference.
- **AP-20 — honored.** Implementation process, reviewer process, workspace, evidence, impl-review persistence, and merge capabilities remain explicitly bounded; neither child receives the post-worker proof or proposal-write capability, while no distinct code-host account is required.
- **AP-21 — honored.** Review consumes the verified semantic demand and never re-judges or maps a numeric rung.
- **AP-22 — honored.** CAS, claim, attempt admission, and fencing make races idempotent and typed.
- **AP-25 — honored.** Backend-specific auth, catalog, controls, sandbox, and artifact details end at adapters.
- **AP-26 — honored.** Exact captured revision and detached clone prevent stacked or moving-head review.
- **AP-28 — honored.** Canonical evidence governs; events, comments, summaries, and journals are projections or native proofs.
- **AP-29 — honored.** Correctness/authentication gates bind completion; usage/cost/latency are measurements.
- **AP-30 — honored.** Review/evidence remotes, default branch, comparison base, and operand repository identity come from the one typed repository-config surface; no pyxis-authored prompt or adapter hardcodes the operand's values.
- **RW-4 — deviated, with reason.** Real code-host and agent CLI adapters ship with local hermetic substitutes because this approved executor must perform real proposal review; ports and conformance land before network dependence.
- **RW-5 — honored.** This successor owns the one executor composition/handoff question; impl-review judgment, native policy, evidence mechanics, and driver wake remain independent designs.
- **RW-6 — deviated, with operator authorization.** The seam uses one evidence-gated direct replacement with no canary/dual success path; pre-activation failure and abuse matrices preserve evidence-before-replacement.
- **Deviation:** RW-4 and RW-6 as stated; none other.
