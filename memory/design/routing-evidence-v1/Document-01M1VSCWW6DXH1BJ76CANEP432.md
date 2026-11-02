# Routing evidence protocol — technical design

| Field | Value |
|---|---|
| **Status** | Draft → Approved *(merge requires Approved; merge is the freezing event)* |
| **Owner** | @mdarabi |
| **Approver(s)** | @mdarabi (operator and security reviewer) |
| **Date** | 2026-07-28 |
| **Relevant-when** | designing or changing canonical demand, route, outcome, assurance-pass, review-process, invalidation, or activation evidence; implementing the append/read/reduce port; recovering an interrupted routed attempt; triaging missing, duplicate, stale, or conflicting routing proof; superseding evidence retention |
| **Scope** | One closed append-only git event family, domain-separated identities and joins, compare-and-swap append, exact-read launch admission, terminal completeness, invalidation, projections, and crash recovery. **Not:** demand derivation, route choice, review policy/verdict computation, proposal landing, or metrics policy. |
| **Tracking** | — *(the introducing PR)* |
| **Builds-on** | docs/tech-design/execution-demand-contract-v1.md · docs/tech-design/native-routing-policy-v1.md |
| **Supersedes** | — |
| **Serves** | prd:planner-driven-model-routing/FR-20 · prd:planner-driven-model-routing/FR-21 · prd:planner-driven-model-routing/FR-24 · prd:planner-driven-model-routing/UB-7 · prd:planner-driven-model-routing/UB-10 · prd:planner-driven-model-routing/UB-14 · prd:planner-driven-model-routing/NFR-4 · prd:planner-driven-model-routing/NFR-8 · prd:planner-driven-model-routing/EVT-1 · prd:planner-driven-model-routing/EVT-2 · prd:planner-driven-model-routing/EVT-3 · prd:planner-driven-model-routing/EVT-5 · prd:planner-driven-model-routing/EVT-8 · prd:planner-driven-model-routing/EVT-9 · prd:planner-driven-model-routing/EVT-10 · prd:implement-now/FR-25 · prd:implement-now/UB-12 · prd:implement-now/UB-13 · prd:implement-now/UB-18 · prd:implement-now/NFR-4 · prd:implement-now/NFR-9 · prd:implement-now/EVT-7 · prd:implement-now/EVT-8 · prd:implement-now/EVT-9 · prd:implement-now/EVT-10 · prd:implement-now/EVT-11 · prd:impl-review/FR-6 · prd:impl-review/FR-27 · prd:impl-review/FR-29 · prd:impl-review/FR-30 · prd:impl-review/UB-10 · prd:impl-review/UB-11 · prd:impl-review/UB-17 · prd:impl-review/NFR-10 · prd:impl-review/NFR-11 · prd:impl-review/NFR-12 · prd:impl-review/EVT-2 · prd:impl-review/EVT-3 · prd:impl-review/EVT-7 · prd:impl-review/EVT-8 · prd:impl-review/EVT-9 · prd:impl-review/EVT-10 · prd:impl-review/EVT-11 |
| **Security/privacy impact** | yes — canonical evidence authorizes model launch and later acceptance, is written through a git network boundary, and records provider/native-profile and usage metadata. Events exclude prompt/code/secret payloads and are validated before trust. reviewer: @mdarabi. |
| **Template version** | tech-design-template/v1 |

## 0. Read to rule — the human reviewer's section (~5 min)

*The body is the contract; this section only routes attention — on any conflict the body governs; machines ignore it.*

**Judge these**

| # | The call | Could have gone | If you disagree |
|---|---|---|---|
| 1 | One immutable file per event on one dedicated fetched git evidence ref is canonical | mutable ledger or proposal authority (`#ALT-9`) | you accept append contention, rewrites, or a second truth → §3.1, #DINV-1 |
| 2 | An exact-read durable resolved event is required before task claim or reviewer launch | launch then commit (`#ALT-8`) | you accept side effects with no reconstructable route → §3.4, #DINV-5 |
| 3 | Every resolved attempt gets exactly one verified/failed/blocked outcome, including claim loss or reaped crash | infer absence from missing outcomes | you accept permanently ambiguous evidence chains → §3.5, #DINV-7 |
| 4 | Proposal/report/ticket faces are checked projections, never authorities | medium-local result stores | you accept drift between audit and acceptance → §3.7, #DINV-9 |
| 5 | Multi-schema terminal facts and pass findings become reachable only as atomic composites | tolerate partial canonical prefixes | a crash could leave a favorable but incomplete chain → §3.3, §3.5, §3.6, #DINV-13 |
| 6 | Semantic producer authority is independently signed and revalidated by every reader | trust generic ref-write credentials | direct pushes could mint invalidation or activation authority → §3.10, §3.12, #DINV-10 |

**Sanity-check the numbers** — one resolution terminal per attempt (#DINV-4); one outcome per resolved attempt and zero outcomes per failed resolution (#DINV-7); one current fenced owner generation (#DINV-14); same ID plus different bytes is always an integrity error (#DINV-3).

**The author's open questions** — none — the concrete ref name and sharded path layout are config/implementation details; one fetched ref and identity-derived immutable paths are binding.

**Skip the rest** — route selection → `native-routing-policy-v1`; pass/verdict policy → `impl-review-routed-assurance`; proposal/worker lifecycle and activation predicate → `implement-now-routed-execution`; event correctness → schema, reducer, fault tests, and exact-read gate below.

## 1. Problem

Routing is consequential before a proposal exists: an unsupported route stops before claim, a resolved route spends model capacity, and a reviewer route can authorize acceptance. Proposal payloads therefore cannot be the record of truth, and a local buffer committed after execution loses the exact evidence needed after a host crash. A mutable JSONL file adds an append-hot conflict and lets retries rewrite history.

The system needs one reconstructable route from frozen demand through native resolution and terminal outcome, plus review-pass, aggregate, invalidation, and activation proof. The append itself must be the admission boundary, not merely after-the-fact telemetry (`design-brainstorm:planner-driven-model-routing-tech-design-4#D-1`; `design-brainstorm:planner-driven-model-routing-tech-design-3#D-3`).

## 2. Decision

**Decision:** Routing evidence is one closed `routing-evidence/v1` event family stored as one immutable content-addressed file per logical event on a dedicated fetched git evidence ref. Writers validate the full existing reducer state, compare-and-swap the ref head, and exact-read the newly reachable event; only a durably observed `routing_resolved` event authorizes task claim or reviewer launch, and every resolved attempt must later append exactly one bounded outcome.

Git is both transport and authority. Proposals, reports, tracker text, runtime journals, and local caches are projections referencing canonical event IDs. The event owner validates structure and joins but never derives demand, chooses a native route, computes a review verdict, or decides acceptance.

## 3. The design

### 3.1 Canonical store and event envelope

**Decision:** The repository configuration names one dedicated evidence ref that every production reader fetches explicitly. The ref contains only immutable `routing-evidence/v1` event files at deterministic identity-derived paths; no append-hot aggregate is authoritative. The implementation may shard paths by identity prefix without changing logical identity.

Every event envelope contains schema/type version, event ID, type, subject identities, predecessor/reference IDs required by its type, canonical payload, producer role and implementation generation, producer proof, and content digest. One shared `routing-evidence-canonical/v1` Go codec produces every preimage and file: UTF-8 JSON, object keys sorted by UTF-8 bytes, integers in minimal base-10, schema-declared arrays ordered as declared, set-valued arrays sorted by canonical element bytes, absent fields omitted, and `null` rejected unless the closed schema admits it. Strings remain exact valid UTF-8 bytes with minimal JSON escaping; no producer-local Unicode normalization, map iteration, float, or ambient clock enters the encoding.

Identity and content use three closed, non-recursive encoding stages:

1. `identity_preimage` contains only domain `routing-evidence-event-id/v1`, repository identity, evidence-ref identity, event type/schema version, and the type-specific tuple below. It excludes `event_id`, `content_digest`, `producer_proof`, every non-identity payload field, and all git metadata. `event_id = sha256(canonical(identity_preimage))`.
2. `content_preimage` is the complete schema-valid envelope with the computed `event_id`, producer class/generation, references, and payload, but without `content_digest` or `producer_proof`. `content_digest = sha256(canonical(content_preimage))`.
3. `unsigned_envelope` is `content_preimage` plus the computed `content_digest`, still without `producer_proof`. The producer signs the domain-separated canonical `unsigned_envelope`; the stored event file is that envelope plus the closed proof object.

The type-specific identity tuples are exhaustive:

| Event kind | Semantic-identity fields |
|---|---|
| `execution_demand_frozen`, `legacy_execution_demand_translated` | `demand_id` |
| `routing_resolved`, `routing_resolution_failed` | `attempt_id` |
| `routed_attempt_completed`, `routed_task_outcome` | `attempt_id` |
| `attempt_owner_transferred` | `attempt_id`, `new_owner_generation` |
| `assurance_pass_completed` | `review_process_id`, `process_generation`, `pass_id` |
| `finding_persisted` | `pass_id`, `finding_id` |
| `disposition_recorded` | `finding_id`, `native_medium_act_ref` |
| `review_process_completed` | `review_process_id`, `process_generation` |
| `routing_evidence_invalidated` | `invalidation_id` |
| `activation_checkpoint` | `activation_policy_version`, `exact_mainline_object`, `proof_manifest_digest` |

`invalidation_id` is itself the domain-separated digest of policy/profile generation, sorted affected event/artifact identities, disposition, and authorization reference; none of those fields may change behind that ID. Locally generated timestamps are forbidden in canonical payloads: git commit metadata may describe append time but never feeds reduction. A source contract that requires a time carries the exact source-owned opaque value only where that event's table row admits it. Thus a retry reconstructed from canonical parents produces the same ID and bytes without a second draft store. The same semantic identity with a different content digest or stored file is an integrity conflict, never a later version.

The closed event kinds preserve the upstream canonical names and payloads:

- `execution_demand_frozen` — exact plan/v2 origin and demand ID (parent EVT-1);
- `legacy_execution_demand_translated` — typed no-rewrite plan/v1 origin (`prd:planner-driven-model-routing/EVT-8`);
- `routing_resolved` or `routing_resolution_failed` — exactly one terminal resolution for an attempt;
- `routed_attempt_completed` — verified, failed, or blocked for a resolved implementation-or-review attempt (parent EVT-5);
- `routed_task_outcome` — the implementation consumer's exact terminal schema, digest-bound to its parent `routed_attempt_completed` event rather than inferred from it (implement-now EVT-10);
- `attempt_owner_transferred` — a fenced recovery handoff to one higher owner generation;
- `assurance_pass_completed` — a valid exact-revision top-level pass terminal carrying its bounded normalized finding set;
- `finding_persisted` — one normalized finding exact-bound to its enclosing pass terminal;
- `disposition_recorded` — an attributed finding disposition exact-bound to its native medium author act;
- `review_process_completed` — complete-set aggregate or typed non-accepting process terminal;
- `routing_evidence_invalidated` — append-only disposition of affected events/artifacts;
- `activation_checkpoint` — union predicate inputs/result and exact mainline object.

- **DINV-1** — A logical event has one immutable canonical file on one evidence ref; no mutable ledger, proposal, report, or ticket is a second authority.
- **DINV-2** — Canonical events contain bounded identity, normalized findings/dispositions, decision, outcome, usage, and provenance data only—never prompt, raw model output, source code, diff, credential, or session transcript.
- **DINV-11** — Every producer and fresh reader computes byte-identical unsigned bytes, event ID, and content digest from identical closed inputs; retries never depend on a regenerated timestamp or lost local draft.

### 3.2 Identity and joins

Demand IDs come from `execution-demand-contract-v1`. A caller mints a distinct attempt ID for each intended implementation or review launch; recovery reuses it. Review process and pass IDs come from `impl-review-routed-assurance`. Every ID is domain separated by kind and repository.

The reducer recognizes closed structural states rather than requiring a terminal chain after every append:

| State | Required reachable events | May authorize |
|---|---|---|
| `resolution_failed` | one failure terminal; no resolved terminal or outcome | nothing |
| `resolved_pending_outcome` | one resolved terminal; no outcome composite yet | implementation claim or review launch only after §3.4 admission |
| `resolved_terminal` | one resolved terminal plus one complete outcome composite | downstream pass/process reduction according to outcome |
| `process_in_flight` | exact process/pass set; each pass absent, admitted, or non-accepting; no aggregate | pass orchestration only |
| `process_terminal` | complete required pass composites plus one aggregate, or one typed non-accepting terminal naming every incomplete contributor | proposal/report projection; satisfaction only for the complete valid set |
| `invalidated` | any prior structural state plus an authorized invalidation naming exact affected identities | no current acceptance from invalidated evidence |

The reducer enforces within those states:

- one demand origin per demand ID;
- one resolution terminal per attempt ID, connected to one demand, subject, harness, role, and policy;
- at most one complete outcome composite for each resolved attempt and none for a failed resolution;
- one atomic pass composite per pass ID, connected to its resolved review attempt outcome and containing its complete finding-event set;
- at most one admitted unresolved attempt per `(review_process_id, pass_id)`; a later attempt requires the earlier one to have a terminal and an explicit new process generation;
- one process terminal per process ID, naming the complete required pass set;
- one disposition event per finding/medium act, whose native author proof validates against the declared actor class;
- invalidations reference existing event/artifact identities and never mutate them;
- activation checkpoints name an exact mainline object and every proof identity they evaluated.

Cross-repository, cross-task, cross-role, cross-revision, duplicate-terminal, missing-parent, and schema/version joins are invalid.

- **DINV-3** — Re-appending the same ID and byte-identical content is success without a new commit; the same ID with different bytes is an integrity error.
- **DINV-4** — An attempt has exactly one resolution terminal; event order or git commit grouping cannot change that result.
- **DINV-12** — Every reachable prefix is exactly one declared structural state; only the declared complete states can authorize their downstream consequence, and no missing child event is inferred from a parent count or payload.

### 3.3 Compare-and-swap append

The append port accepts a fully formed event or schema-declared atomic composite and an observed evidence-ref head:

1. fetch and exact-read the configured evidence ref;
2. validate canonical bytes, producer proof, identity, referenced parents, the transition from the current structural state, and every composite completeness rule;
3. if every identity path already contains identical bytes, return the existing proof; if any path differs or only a strict subset exists, fail integrity;
4. create a commit adding only the new event path or all paths of the closed atomic composite, using the observed head as parent;
5. compare-and-swap the remote ref from observed head to new commit;
6. on a non-conflicting losing race, fetch, revalidate against the new head, and retry to a configured bound;
7. exact-read the event bytes from the winning ref and return a proof containing ref, commit, path, ID, and digest.

A conflicting event, partial composite, invalid existing chain, unavailable ref, authorization failure, or exhausted retry bound fails closed. Multiple unrelated events may share a commit as an optimization only if each retains its own identity/path and the transaction validates the same invariants. Atomic outcome and pass composites are not optional batching: none of their constituent paths becomes reachable unless the complete set wins one compare-and-swap.

### 3.4 Launch admission

**Decision:** For each intended route the caller exact-reads the demand origin and effective policy, resolves through the already-selected harness/role, appends exactly one resolution terminal, and exact-reads its proof. Only a verified `routing_resolved` proof permits claim (implementation) or model launch (review). A `routing_resolution_failed` proof permits neither.

Resolution itself performs zero inference and zero network I/O. From the instant its terminal exists until claim or selected worker/reviewer invocation, canonical append and exact-read are the only permitted network operations; policy/qualification lookup, harness probing, model calls, and every other outbound operation are forbidden. A boundary tracer owned by the caller records the full interval and the evidence proof, and the same predicate gates both implementation and review.

The resolved event is a permit, not evidence that launch occurred. Two executors may resolve the same ready task concurrently; the ordinary claim remains the mutual-exclusion gate. A losing claimant appends a blocked outcome for its own resolved attempt. For review, the reducer admits only the first unresolved route for a process-generation/pass. A concurrent loser exact-reads the winner and appends `routing_resolution_failed(reason=duplicate_admission)` for its distinct attempt, authorizing no launch and requiring no routed outcome.

- **DINV-5** — No canonical resolved event, no task claim and no reviewer model invocation.
- **DINV-6** — A failed resolution has zero claims, zero model invocations, and zero routed outcome events.
- **DINV-15** — Between offline resolution terminal and claim/invocation, the only network operations are the canonical append and exact-read that admit that terminal.

### 3.5 Outcome completeness and crash recovery

**Decision:** Every resolved attempt reaches exactly one `routed_attempt_completed`: `verified`, `failed`, or `blocked`. For an implementation attempt, that parent event and the implement-now `routed_task_outcome` are one atomic two-file composite with identical `attempt_id`, `demand_id`, `task_id`, optional proposal identity, usage, and evidence proof. The only valid terminal mappings are:

| Parent outcome | implement-now class | implement-now verification |
|---|---|---|
| `verified` | `succeeded` | `verified` |
| `failed` | `failed` | `failed` |
| `blocked` | `blocked` | `not_reached` |

Every other combination or shared-field difference is invalid. Review attempts emit only the parent event. Outcome composites record available usage and launch fidelity, but no prompt/code. Post-resolution claim loss, credential/preflight failure, cancellation, and bounded crash reaping are typed blocked/failed outcomes rather than missing evidence.

The canonical evidence chain and runtime adapter jointly own attempt fencing; neither a tracker claim nor an unfenced process-local check is authority. A resolved event binds `owner_generation=1`. The runtime adapter activates that exact resolution/generation and issues an unforgeable generation-scoped launch capability. Every model invocation, including review without a tracker claim, must cross the adapter's `LaunchFenced(attempt_id, owner_generation, resolution_proof, capability)` boundary. At the actual spawn boundary the adapter atomically consumes the capability only if that generation is still current and unrevoked; validation before the boundary is insufficient.

A takeover first calls the same adapter's `FenceAndObserve(attempt_id, prior_generation)`. That operation atomically revokes every unconsumed launch capability for the prior generation before returning a signed, nonce-bound `runtime_fence_proof` and one closed `not_started | running(handle) | terminal | absent_proven | unknown` observation. A stale owner whose spawn races the fence therefore loses either the capability consumption or the revocation and its actual spawn fails; it cannot pass a check and launch later. `running` reattaches without transfer. `unknown` launches nothing and remains non-accepting. Only a revocation proof paired with `not_started`, `terminal`, or `absent_proven` permits the recovery coordinator to append an authorized `attempt_owner_transferred` event naming the prior and next generations. After exact-read of that event, the adapter may activate and issue the next generation's capability. A crash after revocation but before append leaves the prior generation fenced and is recovered by replaying the nonce-bound proof, not by restoring its authority.

Every launch and outcome carries the current fencing generation. Tracker claim/proposal state is supporting evidence, never the sole oracle. Operator-authorized blocked reap may terminal an `unknown` attempt but never authorizes a replacement launch. A revived old owner is rejected at `LaunchFenced` and at terminal append. Terminal retry stays with the current generation; a fresh retry after a terminal gets a new attempt ID and preserves the earlier chain.

- **DINV-7** — Every resolved attempt eventually has one outcome; every failed resolution has none.
- **DINV-8** — Recovery never launches a second model invocation when the first invocation's absence is not positively established.
- **DINV-13** — An implementation outcome and a pass plus its findings become reachable only as complete atomic composites; no partial prefix can authorize a process or projection.
- **DINV-14** — Exactly one current fenced owner generation may launch or terminal an attempt; ambiguous liveness never transfers authority.

### 3.6 Review and invalidation events

An assurance-pass terminal is accepted only after its review routing attempt has a verified outcome and its payload passes the exact-revision/pass identity rules of `impl-review-routed-assurance`. It carries the complete bounded normalized finding records—grammar, calibration, severity, acceptance, and provenance—not raw candidates or model output. The pass terminal and exactly one `finding_persisted` child for every declared finding ID are one atomic composite; child IDs, normalized bytes, count, pass identity, and revision must match bijectively. A later `disposition_recorded` event carries the finding ID, disposition, native proposal-medium act/reference, native author identity proof, and attribution-verification result. A review-process terminal references the complete required pass composites and records the deterministic aggregate. Missing or non-accepting inputs can produce a typed non-accepting process terminal but never `satisfied`.

Invalidation appends a new event naming defective policy/profile generations, exact affected event IDs and artifacts, and disposition `rerun_required` or `operator_remediation`. Reducers exclude invalidated evidence from current acceptance without deleting history. Open work must produce replacement attempt/process evidence on the exact subject revision; accepted artifacts receive a human brief rather than destructive rollback.

### 3.7 Projections and read models

**Decision:** Proposal/report/tracker payloads and measurement datasets are deterministic projections from canonical events. A projection carries event IDs, evidence-ref proof, and its own digest. Acceptance exact-reads canonical events and rejects missing, stale, extra, or field-different faces.

The read API returns typed subject views—demand chain, attempt chain, review process, invalidation disposition, activation checkpoint—by running the same reducer over a fetched exact ref. Local caches are disposable and keyed by evidence-ref head; no cache result can authorize launch or acceptance without exact-read proof.

- **DINV-9** — A medium projection either losslessly matches its canonical event set or is rejected; a favorable projection cannot override a non-accepting or invalidated chain.

### 3.8 Boundaries and non-goals

- **Not solved here** — derivation, native selection, finding quality, review aggregation policy, task claim semantics, or merge authority.
- **Owned elsewhere** — origin schema and resolver/process payload semantics remain with their sibling designs; this design validates and retains them.
- **Implementation must not infer** — outcomes from absence, ordering from timestamps, acceptance from proposal badges, or route choice from event history.
- **Must remain unchanged** — tracker claims, proposal media, merge-lane authority, and append-only git history.

### 3.9 Observability

- **Events/logs** — the canonical family is the primary evidence; append/read/reduce operations log ref head, event ID/type, attempt/process IDs, retries, and typed failure without payload secrets.
- **Metrics** — append latency/retries/conflicts, evidence-ref availability, incomplete resolved attempts, invalid joins, projection mismatches, exact/promoted/unsupported and terminal counts, usage sums, and invalidations.
- **Alerts** — unavailable/corrupt evidence ref, identity conflict, launch without admission, overdue resolved attempt, projection mismatch, or qualification invalidation alerts the repository operator.
- **Debugging surface** — a fresh-clone verifier renders each chain, missing edge, invalidation, projection diff, and exact git object; it never needs session state.
- **Trace/correlation** — demand, attempt, pass, process, proposal/task/revision, event, and activation identities join end to end.

### 3.10 Security & privacy

The evidence ref is a production-control and audit boundary. Its adapter conformance requires an explicit genesis checkpoint, fetch for readers, fast-forward compare-and-swap for the sole writer identity, and remote denial of non-fast-forward update, deletion, and unreviewed initialization. Ref protection alone is not semantic authority: every fresh reader revalidates the canonical bytes, producer proof, complete reducer chain, and continuity from genesis before trust. A direct push that bypasses the append port therefore cannot create accepted evidence unless every event is independently authorized and the full transition is valid; force/delete/rewrite makes the ref unusable and alerts rather than replacing history.

Content limits (#DINV-2) prevent prompt/code/secret retention while still recording provider/native profile and bounded usage. Tracker or proposal contributors cannot forge authority because those faces are exact-checked projections. A malicious git contributor cannot weaken an existing record: events are immutable, identity conflicts fail, invalidation is additive, and current acceptance depends on verified ref continuity.

### 3.11 Test strategy

The schema, identity, reducer, producer-proof verifier, and projection functions are in-process. A local bare git repository with the same fast-forward/delete/initialization guards is the real substitutable evidence adapter for append/CAS/fetch/restart/concurrency tests. The hosted git remote is remote-but-owned behind the same port and conformance suite; a recorded rehearsal proves its protection behavior. Fault injection stops before and after every append/CAS/exact-read/claim/launch/composite/ownership-transfer boundary and asserts #DINV-5..#DINV-15. Fresh-clone tests reconstruct every accepted chain, complete finding set, attributed disposition, producer authority, and genesis continuity with caches and proposal state absent.

### 3.12 Producer authorization

**Decision:** `routing-evidence-producer-policy/v1` is a closed matrix over event kind, producer role, repository identity class, implementation generation, and required authorization proof. Operator-authorized repository config exact-identifies the policy digest, trusted Ed25519 public keys, allowed event classes and generations for each key, the evidence ref, and its genesis commit. A short-lived producer receives only the private signing capability for its declared class/generation. `routing-evidence-producer-proof/v1` is the key ID, algorithm, and deterministic Ed25519 signature over domain `routing-evidence-producer-proof/v1` plus the exact canonical `unsigned_envelope` from §3.1. Repository, ref, event ID/type, content digest, producer class, and generation are already closed fields in that signed envelope; the proof object itself is excluded from all three preimages. Replay across event kind, repository, ref, generation, or content therefore fails.

The append port validates the proof before schema/join admission, and every reducer/fresh-clone read repeats the same verification independently of the write path:

| Event class | Authorized producer |
|---|---|
| demand origin | exact plan gate or version-pinned legacy translator generation |
| routing terminal | selected resolver/orchestrator generation for the declared role |
| routed outcome | the admitted attempt owner/reaper generation |
| ownership transfer | configured recovery-coordinator generation, with a valid nonce-bound `runtime_fence_proof`; forced blocked reap additionally requires configured operator authorization |
| assurance pass/finding/process | the admitted reviewer/orchestrator generation |
| disposition | proposal-medium adapter after native author proof; operator class requires the configured operator identity |
| invalidation | evidence-owner generation plus a current qualification-defect or operator-remediation authorization reference |
| activation checkpoint | release-gate generation plus the operator-signed activation manifest and exact human-merged mainline proof |

Workers and ordinary outcome producers cannot authorize invalidation, process satisfaction, operator disposition, ownership transfer, or activation events. Unknown/revoked keys, producer generations, actor classes, authorization references, signatures, and policy versions fail before the event becomes trusted. Key rotation is additive policy with an exact activation point; it never makes an old valid signature authorize a new generation.

- **DINV-10** — Event authority is least-privilege by kind; possession of generic evidence-ref write credentials alone never authorizes a semantic event.

## 4. Alternatives considered

| Alternative | Why rejected |
|---|---|
| Append-first canonical git evidence (#ALT-7) | selected because the durable exact-read route becomes the mechanical launch permit and a fresh clone can reconstruct acceptance (`design-brainstorm:planner-driven-model-routing-tech-design-3#ALT-7`) |
| Buffer locally and commit after terminal outcome (#ALT-8) | a crash can spend inference or mutate work with no retained route; safe recovery then needs a second ticket |
| Mutable ledger or proposal payload as authority (#ALT-9) | creates append contention, rewritable history, and no home for pre-proposal failures |
| Fold evidence into the native routing-policy record (`design-brainstorm:planner-driven-model-routing-tech-design-4#ALT-2`) | couples model-roster policy changes to append/recovery semantics and obscures the launch-admission boundary |
| One route ticket store plus later git archive | introduces a second durable authority and reconciliation protocol |
| Cancel a resolved attempt by deleting its event | destroys provenance and makes old projections unverifiable; invalidation must append |

## 5. Risks

| ID | Risk | Impact | Mitigation | Verification / evidence |
|---|---|---|---|---|
| R1 | Evidence-ref outage stops all model-backed work | availability loss | fail closed by contract; surface health and preserve retry state | outage fault matrix and alert drill |
| R2 | Concurrent append retries livelock | delayed fleet | one-file non-conflicting commits and bounded CAS retry | concurrent writer stress test |
| R3 | Resolved attempt remains terminal-less after host loss | audit and acceptance stall | canonical fenced ownership transfer, typed runtime observation, bounded reap, and overdue alert | two-owner crash-at-every-boundary suite |
| R4 | Git history accumulates many small files | clone/storage cost | deterministic sharding and measurement; never trade away immutability | scale/load report, non-blocking cost trend |
| R5 | Projection reducer differs across media | favorable stale face accepted | one reducer/codec and exact canonical comparison | GitHub/Carina golden and mismatch corpus |

## 6. Acceptance

| ID | Acceptance criterion | Verify | Required evidence | Traces to |
|---|---|---|---|---|
| A-1 | Exact, promoted, unsupported, verified, failed, blocked, pass, aggregate, invalidation, and activation fixtures each append immutable canonical events reconstructable from a fresh clone; independent producers match every per-kind identity-preimage, content-preimage, unsigned-envelope, byte, ID, digest, and exclusion golden, including time-varying retries. | Test + Inspection | event-family, preimage/exclusion, and cross-producer canonical golden corpus plus fresh-clone report | `prd:planner-driven-model-routing/FR-21` · #DINV-1 · #DINV-11 |
| A-2 | Concurrent non-conflicting writers converge; byte-identical repeats no-op; same-ID/different-byte and conflicting joins fail without overwriting; every declared in-flight prefix reduces to its exact state; partial outcome/pass composites are never reachable; concurrent attempts for one review pass admit exactly one route and terminal every loser without launch. | Test + Failure injection | state-transition, atomic-batch, CAS/concurrency, and pass-admission report | #DINV-3 · #DINV-4 · #DINV-12 · #DINV-13 |
| A-3 | Resolution spends zero inference and zero network I/O; every implementation/review terminal is appended and exact-read before claim/invocation; the complete interval contains no network operation except that append/read; every failure or unavailable/unverifiable-evidence case performs zero claim/invocation. | Test + Analysis + Failure injection | implementation/review boundary network trace matrix | `prd:planner-driven-model-routing/NFR-8` · `prd:implement-now/NFR-9` · `prd:impl-review/NFR-12` · #DINV-5 · #DINV-6 · #DINV-15 |
| A-4 | Every resolved attempt, including claim loss, preflight failure, cancellation, and reaped crash, reaches one outcome; failed resolutions reach none; every implementation outcome matches the §3.5 table and both files appear atomically; every pass/finding bijection appears atomically. | Test + Failure injection | value-combination, composite-interruption, and crash/recovery matrix | `prd:planner-driven-model-routing/FR-20` · `prd:implement-now/FR-25` · #DINV-7 · #DINV-13 |
| A-5 | Runtime observation and canonical fenced ownership handle two owners, old-owner revival, host loss, review without a tracker claim, terminal retry, and ambiguous launch without duplicate invocation or conflicting terminal; only proven absence transfers authority. The two-owner and revival fixtures race the stale owner's actual `LaunchFenced` spawn against revocation and require that spawn—not merely its later evidence—to fail. | Test + Failure injection | ownership-generation, capability-consumption, and launch-ambiguity traces | #DINV-8 · #DINV-14 |
| A-6 | Proposal and both tracker/report media losslessly round-trip the canonical chain, complete atomic pass/finding sets, and attribution-bound dispositions from a fresh clone; every missing/extra/stale/changed projection is non-accepting. | Test | media/finding/disposition goldens and negative mismatch corpus | `prd:impl-review/FR-27` · `prd:impl-review/NFR-10` · #DINV-9 · #DINV-13 |
| A-7 | A seeded qualification defect invalidates all exact affected open evidence, preserves unaffected work, requires replacement evidence, and briefs accepted artifacts without rewriting history. | Failure injection + Inspection | invalidation disposition report | `prd:planner-driven-model-routing/UB-14` · `prd:implement-now/UB-13` · `prd:impl-review/UB-17` |
| A-8 | Inspection confirms canonical events contain bounded normalized findings/dispositions but no prompt, raw model output/candidates, source code, diff, credential, or session transcript, while usage remains summable by harness/profile/process. | Inspection + Analysis | payload audit and usage reconstruction | `prd:planner-driven-model-routing/NFR-4` · `prd:impl-review/NFR-11` · #DINV-2 |
| A-9 | Every event kind trusts only its §3.12 signed producer identity/generation/proof; fresh readers reject direct-push bypass, replay, cross-kind/repository/ref use, stale/revoked generation, forged operator, ordinary-writer invalidation, unsigned activation, broken genesis continuity, force update, and delete; the local adapter and one hosted rehearsal prove the same fast-forward-only policy. | Test + Security review + Demo | producer-policy abuse matrix, fresh-reader corpus, and hosted-ref protection rehearsal | #DINV-10 |

## 7. Open questions

None. The configured ref name, sharding depth, and batching are replaceable implementation details so long as one fetched ref, immutable identity-derived files, exact-read admission, and the same reducer remain true.

## Principles

- **AP-5 — honored.** Routing decisions, outcomes, failures, invalidations, and activation are first-class evidence.
- **AP-1 — honored.** The reducer depends on a typed evidence port; local bare and hosted git are conforming adapters, never core branches.
- **AP-3 — honored.** One adapter conformance suite covers append, ref protection, fresh reads, continuity, and failure behavior.
- **AP-6 — honored.** Every event and projection uses a closed versioned schema.
- **AP-11 — honored.** One canonical reducer decides join validity for every surface.
- **AP-13 — honored.** Exact git evidence is observed before launch or acceptance.
- **AP-15 — honored.** Idempotent append and terminal recovery converge without rewriting history.
- **AP-17 — honored.** Evidence unavailability or ambiguity stops consequential side effects.
- **AP-20 — honored.** Storage/reduction is separate from demand, route, and verdict policy.
- **AP-22 — honored.** Every admitted attempt and concurrent writer has bounded ownership and recovery.
- **AP-23 / AP-27 — honored.** One identity-derived file per event and derived read models remove the append-hot authored aggregate.
- **AP-26 — honored.** Readers verify the exact fetched ref, genesis continuity, and exact mainline activation object rather than trusting a medium status.
- **AP-28 — honored.** Canonical retained events are the one authority; all other faces derive from them.
- **AP-29 — honored.** Correctness admission is an exact predicate; latency/cost remain measurements.
- **RW-5 — honored.** This document owns only retained evidence and its crash boundary.
- **Deviation:** none.
