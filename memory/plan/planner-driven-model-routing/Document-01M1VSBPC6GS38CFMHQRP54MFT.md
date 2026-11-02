# Planner-driven model routing — plan 003

| Field | Value |
|---|---|
| **Feature** | planner-driven-model-routing |
| **PRD** | docs/prds/planner-driven-model-routing |
| **Design** | docs/tech-design/routing-evidence-v1.md |
| **Plan version** | 003 |
| **Dialect** | harbormaster/fs-spec:v1 |
| **Epic title** | Retain canonical routing evidence before model launch |
| **Status** | Draft — freezes at merge |
| **Template version** | plan-template/v1 |

## Tasks

### T1 — Canonicalize and identify every routing event

| Field | Value |
|---|---|
| **Blocked by** | — |
| **Size band** | 240–300 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | rung=3 |
| **Pattern** | — |
| **Touches** | internal/routingevidence event schemas, canonical codec, identity preimages, event-family golden fixtures |

**Goal:** Every routing-evidence event kind has one closed canonical envelope, one domain-separated semantic identity, and one content digest that independent producers reproduce byte for byte.

**Current behavior:** Execution-demand and native-route values have their own codecs, while runtime journals and review stores serialize unrelated payloads; no `routing-evidence/v1` event family, shared preimage grammar, or canonical event identity exists.

**Desired behavior:** One codec constructs and strictly decodes every demand, route, outcome, ownership, assurance, finding, disposition, invalidation, and activation event. Identity, content, and unsigned-envelope stages follow the design's exact exclusions and ordering, reject non-canonical input, and make an identical semantic retry reproduce the same path and bytes.

**Scope:**
- Define the closed envelope and payload union for every event kind named by the design, preserving each upstream event name and bounded payload.
- Implement the canonical JSON codec plus identity, content-digest, unsigned-envelope, and deterministic identity-derived path functions.
- Prove every type-specific identity tuple, set ordering, exclusion, mutation-sensitivity, retry, and non-canonical failure through cross-producer goldens.

**Out of scope:**
- Producer authorization, chain reduction, git append, route selection, review verdict computation, and proposal projection.

**Acceptance criteria:**
- [ ] Independent producers generate byte-identical canonical bytes, event IDs, content digests, unsigned envelopes, and paths for every event kind and time-varying retry fixture.
- [ ] Mutating any identity-bearing field changes the event ID, while changing any other admitted payload field changes the content digest but not the event ID.
- [ ] Unknown keys, kinds, versions, nulls, floats, duplicate fields, non-canonical ordering, invalid UTF-8, and recursive digest/proof inputs fail with no accepted event.
- [ ] Inspection finds no prompt, raw model output, source, diff, credential, session transcript, ambient clock, git metadata, or producer-local Unicode normalization in canonical payloads or preimages.

**Key interfaces:**
- `Event`, `Envelope`, `EventKind`, `IdentityPreimage`, `ContentPreimage`, `UnsignedEnvelope`
- `EncodeCanonical(Event)`, `DecodeCanonical(bytes)`, `Identify(Event)`, `Digest(Event)`, `EventPath(EventID)`
- `routing-evidence/v1`, `routing-evidence-canonical/v1`, `routing-evidence-event-id/v1`

**Traces:** prd:planner-driven-model-routing/FR-21 · prd:planner-driven-model-routing/FR-24 · prd:planner-driven-model-routing/NFR-4 · prd:planner-driven-model-routing/EVT-1 · prd:planner-driven-model-routing/EVT-2 · prd:planner-driven-model-routing/EVT-3 · prd:planner-driven-model-routing/EVT-5 · prd:planner-driven-model-routing/EVT-8 · prd:planner-driven-model-routing/EVT-9 · prd:planner-driven-model-routing/EVT-10 · design:routing-evidence-v1#3.1 · design:routing-evidence-v1#DINV-1 · design:routing-evidence-v1#DINV-2 · design:routing-evidence-v1#DINV-11 · design:routing-evidence-v1#A-1

### T2 — Authenticate event producers and ref genesis

| Field | Value |
|---|---|
| **Blocked by** | T1 |
| **Size band** | 220–290 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | rung=3 |
| **Pattern** | — |
| **Touches** | internal/routingevidence producer policy and proof, evidence-ref genesis policy, signer and verifier fixtures |

**Goal:** A routing event becomes trusted only when its exact kind, repository, ref, producer class, implementation generation, content, and authorization are proven by the configured least-privilege authority.

**Current behavior:** Git credentials can write generic evidence refs and the review path records local identity strings, but no signed semantic producer policy distinguishes demand, route, outcome, transfer, review, invalidation, or activation authority.

**Desired behavior:** A closed operator-authorized policy binds the evidence ref and genesis commit to trusted Ed25519 keys, allowed event classes, roles, and generations. Writers sign the exact unsigned envelope; appenders and fresh readers independently reject unsigned, replayed, cross-boundary, stale, revoked, or over-privileged events.

**Scope:**
- Define closed producer-policy, key-generation, authorization-reference, proof, rotation, revocation, and genesis-continuity contracts.
- Implement deterministic Ed25519 signing and independent verification over the canonical unsigned envelope and the design's producer matrix.
- Prove least privilege for every event class, including operator disposition, ownership transfer, invalidation, and activation.

**Out of scope:**
- Private-key provisioning UX, remote secret management, git CAS append, and deciding whether an event's domain payload is substantively correct.

**Acceptance criteria:**
- [ ] Every event kind accepts only its configured producer role, key, generation, repository, ref, and authorization proof against the exact unsigned bytes.
- [ ] Unsigned, forged, altered, revoked, stale-generation, cross-kind, cross-repository, cross-ref, and generic-ref-writer fixtures fail identically in the append path and a fresh reader.
- [ ] Key rotation becomes effective only at its exact policy activation point and never expands an older signature into a new generation or event class.
- [ ] Missing, changed, deleted, rewritten, or unauthorized genesis continuity makes the evidence ref unusable and produces a typed operator-visible integrity failure.

**Key interfaces:**
- `ProducerPolicy`, `ProducerClass`, `ProducerProof`, `KeyGeneration`, `GenesisCheckpoint`
- `Sign(UnsignedEnvelope, SigningCapability)`, `VerifyProducer(Event, ProducerPolicy)`, `VerifyContinuity(RefView, GenesisCheckpoint)`
- `routing-evidence-producer-policy/v1`, `routing-evidence-producer-proof/v1`

**Traces:** prd:planner-driven-model-routing/FR-21 · prd:planner-driven-model-routing/UB-7 · design:routing-evidence-v1#3.10 · design:routing-evidence-v1#3.12 · design:routing-evidence-v1#DINV-10 · design:routing-evidence-v1#A-9

### T3 — Reduce routing chains into closed structural states

| Field | Value |
|---|---|
| **Blocked by** | T1, T2 |
| **Size band** | 250–300 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | rung=3 |
| **Pattern** | — |
| **Touches** | internal/routingevidence reducer, chain and subject views, transition corpus, join-integrity fixtures |

**Goal:** One deterministic reducer recognizes every valid reachable routing state and refuses every missing, duplicate, conflicting, cross-subject, or partially complete chain before it can authorize a consequence.

**Current behavior:** Execution, review-obligation, publication, and report stores reduce their own records with unrelated identities; no authority joins demand, route, outcome, pass, process, invalidation, and activation events into one reconstructable state.

**Desired behavior:** The reducer verifies canonical bytes and producer authority, then enforces the closed state and join tables for demand origins, resolution terminals, owner generations, outcomes, pass/finding composites, dispositions, process terminals, invalidations, and activation checkpoints. Reordering events cannot change the result, and no absent child is inferred from a count or parent payload.

**Scope:**
- Implement the declared structural states, per-kind parent joins, identity cardinalities, current-generation rules, and exact subject views.
- Validate complete atomic-composite manifests and distinguish valid in-flight prefixes from integrity defects.
- Prove order independence and every duplicate, orphan, cross-repository, cross-task, cross-role, cross-revision, schema, terminal, and partial-composite failure.

**Out of scope:**
- Git transport, native route selection, task claim semantics, review aggregation policy, and repairing an invalid chain.

**Acceptance criteria:**
- [ ] Every valid prefix reduces to exactly one declared state, and identical event sets in any order produce byte-identical typed views.
- [ ] Each demand, attempt, pass, process, finding, disposition, invalidation, and checkpoint obeys the design's exact parent, identity, generation, and cardinality rules.
- [ ] Duplicate terminals, same-ID different bytes, missing parents, cross-boundary joins, partial composites, invalid proofs, and conflicting current owners fail closed and name the exact edge.
- [ ] Only complete declared states expose launch, process, projection, invalidation, or activation eligibility; no timestamp, file order, proposal badge, or missing event is interpreted as evidence.

**Key interfaces:**
- `Reducer`, `StructuralState`, `DemandView`, `AttemptView`, `ProcessView`, `InvalidationView`, `ActivationView`
- `Reduce([]Event)`, `ValidateTransition(State, EventOrComposite)`, `SubjectView(SubjectID)`
- `resolution_failed`, `resolved_pending_outcome`, `resolved_terminal`, `process_in_flight`, `process_terminal`, `invalidated`

**Traces:** prd:planner-driven-model-routing/FR-20 · prd:planner-driven-model-routing/FR-21 · prd:planner-driven-model-routing/FR-24 · prd:planner-driven-model-routing/UB-7 · design:routing-evidence-v1#3.2 · design:routing-evidence-v1#DINV-3 · design:routing-evidence-v1#DINV-4 · design:routing-evidence-v1#DINV-12 · design:routing-evidence-v1#A-2

### T4 — Append immutable events through one canonical git ref

| Field | Value |
|---|---|
| **Blocked by** | T3 |
| **Size band** | 250–300 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | rung=3 |
| **Pattern** | — |
| **Touches** | internal/routingevidence store port, internal/routingevidence/git adapter, internal/gitrefstore multi-path CAS, local bare-git conformance fixtures |

**Goal:** A fully validated event or atomic composite becomes canonical through one fetched fast-forward evidence ref, or the append fails without making a partial or conflicting fact reachable.

**Current behavior:** `gitrefstore` can compare-and-swap one mutable file path, but it does not create immutable identity paths, validate routing state and producer proofs, append multiple files atomically, return a typed exact-read proof, or establish protected genesis.

**Desired behavior:** The routing-evidence adapter fetches one configured ref, validates its complete current state, no-ops byte-identical repeats, commits one immutable path or a complete composite, pushes by compare-and-swap with bounded revalidation, and exact-reads the winning event set. Conflicts, losing races, partial existing sets, unavailable refs, and authorization defects fail closed.

**Scope:**
- Define the consumer-owned append/read port, observed-head lease, exact proof, immutable-path, and atomic-composite contracts.
- Extend the git-ref substrate for multi-path commits, bounded CAS replay, exact same-head reads, protected genesis, and safe ambient-git handling.
- Run the real local bare-git adapter conformance over fresh clones, concurrent writers, interruption boundaries, repeat, conflict, and ref-protection cases.

**Out of scope:**
- Hosted-provider administration automation, route selection, task claim, reviewer launch, and projection rendering.

**Acceptance criteria:**
- [ ] A valid event or complete composite wins one compare-and-swap that advances the observed head and returns ref, commit, path set, event IDs, and digests exact-read from the winning head.
- [ ] Byte-identical replay creates no commit; same-ID different bytes, partial existing composites, invalid current state, unavailable ref, and exhausted races create no trusted result.
- [ ] Concurrent non-conflicting writers converge after bounded fetch, reduction, and retry, while conflicting writers preserve the original canonical event and return a typed integrity error.
- [ ] Fresh-clone and interruption tests prove no atomic outcome or pass composite is partially reachable and that force, delete, rewrite, or unreviewed initialization makes the ref non-authoritative.

**Key interfaces:**
- `Store`, `AppendRequest`, `AtomicComposite`, `ObservedHead`, `EvidenceProof`, `ExactRead`
- `Append(ctx, AppendRequest)`, `ReadSubject(ctx, SubjectID)`, `ReadProof(ctx, EvidenceProof)`
- the routing-evidence git adapter conformance suite

**Traces:** prd:planner-driven-model-routing/FR-21 · prd:planner-driven-model-routing/NFR-8 · design:routing-evidence-v1#3.3 · design:routing-evidence-v1#3.10 · design:routing-evidence-v1#DINV-1 · design:routing-evidence-v1#DINV-3 · design:routing-evidence-v1#DINV-13 · design:routing-evidence-v1#A-1 · design:routing-evidence-v1#A-2 · design:routing-evidence-v1#A-9

### T5 — Admit launches only from exact routing proofs

| Field | Value |
|---|---|
| **Blocked by** | T4 |
| **Size band** | 240–300 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | rung=3 |
| **Pattern** | — |
| **Touches** | internal/routingevidence admission, executor pre-claim route boundary, reviewinvoke pre-launch route boundary, boundary network tracer fixtures |

**Goal:** An implementation claim or reviewer model launch occurs only after its offline route terminal is canonically appended and exact-read, with no hidden network operation between resolution and the consequential boundary.

**Current behavior:** Native routing returns in-memory resolution values and the review invocation seam can journal outcomes, but no canonical `routing_resolved` proof mechanically gates task claim or reviewer launch and no tracer enforces the permitted network interval.

**Desired behavior:** Each caller reconstructs the exact demand origin and effective policy, resolves without I/O, signs and appends one resolved or failed terminal, exact-reads its proof, and presents the verified permit to the claim or review-launch boundary. Unsupported or unverifiable routes authorize nothing; duplicate review admission records the losing failure without launching.

**Scope:**
- Implement the shared admission predicate and boundary trace over demand, selected harness/role, resolution terminal, and exact evidence proof.
- Bind the implementation pre-claim and review pre-launch seams to the same permit without moving harness selection or review-band ownership.
- Prove offline resolution, the append/read-only network interval, duplicate review admission, claim loss, and every unavailable or invalid proof path.

**Out of scope:**
- Choosing a harness, changing native routing policy, task-claim mutual exclusion, review-pass aggregation, and production activation.

**Acceptance criteria:**
- [ ] Every implementation claim and reviewer invocation presents a valid exact-read `routing_resolved` proof bound to its demand, attempt, subject, harness, role, policy, and requested/native profiles.
- [ ] `routing_resolution_failed`, unsupported, duplicate-admission, unavailable, stale, altered, or unverifiable proof fixtures perform zero claim and zero model invocation.
- [ ] Boundary traces prove zero inference and zero network I/O during resolution and only canonical append/exact-read networking from terminal creation through claim or launch.
- [ ] The existing selected harness, merge-lane risk class, impl-review band projection, empty-frontier behavior, and caller process metadata remain unchanged.

**Key interfaces:**
- `AdmissionPermit`, `AdmissionVerifier`, `BoundaryTrace`, `ResolutionTerminalWriter`
- `AdmitImplementation(ctx, RouteIntent)`, `AdmitReview(ctx, RouteIntent)`, `VerifyPermit(EvidenceProof)`
- `routing_resolved`, `routing_resolution_failed`

**Traces:** prd:planner-driven-model-routing/FR-24 · prd:planner-driven-model-routing/NFR-8 · prd:planner-driven-model-routing/EVT-2 · prd:planner-driven-model-routing/EVT-3 · prd:planner-driven-model-routing/NR-1 · prd:planner-driven-model-routing/NR-8 · design:routing-evidence-v1#3.4 · design:routing-evidence-v1#DINV-5 · design:routing-evidence-v1#DINV-6 · design:routing-evidence-v1#DINV-15 · design:routing-evidence-v1#A-3

### T6 — Fence every runtime spawn by owner generation

| Field | Value |
|---|---|
| **Blocked by** | T5 |
| **Size band** | 260–300 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | rung=3 |
| **Pattern** | — |
| **Touches** | executor AgentRuntime fencing port, claude/codex/cursor/opencode spawn boundaries, routing ownership events, two-owner recovery fixtures |

**Goal:** Exactly one current attempt owner generation can cross the actual model-process spawn boundary or append its terminal, including after host loss and concurrent recovery.

**Current behavior:** Runtime adapters launch from ordinary in-process calls and review recovery uses local/store-specific leases; no generation-scoped capability is atomically consumed at spawn and no signed runtime observation can fence a stale owner across hosts.

**Desired behavior:** A resolved generation activates one unforgeable launch capability. Every adapter consumes it atomically at the real spawn boundary; `FenceAndObserve` revokes prior authority before returning a nonce-bound observation, and only proven non-start or terminal absence permits an exact-read ownership-transfer event and a new generation. Unknown liveness never relaunches.

**Scope:**
- Extend the consumer runtime port with generation activation, `LaunchFenced`, revocation/observation, capability consumption, and signed fence-proof contracts.
- Enforce the same atomic spawn predicate in Claude Code, Codex, Cursor CLI, and OpenCode without exposing native credentials to the evidence layer.
- Implement ownership-transfer validation and race actual stale-owner spawn against revocation, takeover, revival, terminal retry, and ambiguous observation.

**Out of scope:**
- Distributed scheduler election, killing a positively running process, inventing a second attempt after terminal, and model-specific routing changes.

**Acceptance criteria:**
- [ ] Across all four adapters, exactly one current generation can consume its capability at actual spawn; a stale, replayed, revoked, altered, or already-consumed capability spawns nothing.
- [ ] `running` reattaches, `unknown` remains non-accepting, and only a nonce-bound proof of `not_started`, `terminal`, or `absent_proven` admits one higher ownership generation.
- [ ] Two-owner, host-loss, crash-after-revocation, old-owner revival, review-without-claim, terminal-retry, and operator-reap fixtures never duplicate a model invocation or conflicting terminal.
- [ ] Every launch and outcome remains bound to its exact attempt, resolution proof, owner generation, and capability, while tracker/proposal state is supporting rather than sole authority.

**Key interfaces:**
- `FencedRuntime`, `LaunchCapability`, `OwnerGeneration`, `RuntimeFenceProof`, `RuntimeObservation`
- `Activate(attempt, generation, proof)`, `LaunchFenced(ctx, spec, capability)`, `FenceAndObserve(ctx, attempt, generation)`
- `attempt_owner_transferred`

**Traces:** prd:planner-driven-model-routing/FR-21 · prd:planner-driven-model-routing/UB-7 · design:routing-evidence-v1#3.5 · design:routing-evidence-v1#DINV-8 · design:routing-evidence-v1#DINV-14 · design:routing-evidence-v1#A-5

### T7 — Close every resolved attempt with one exact outcome

| Field | Value |
|---|---|
| **Blocked by** | T4, T6 |
| **Size band** | 250–300 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | rung=3 |
| **Pattern** | — |
| **Touches** | routing outcome writer and reaper, executor routed-task outcome composite, review attempt terminals, outcome recovery fixtures |

**Goal:** Every canonically resolved attempt reaches one verified, failed, or blocked terminal, and an implementation terminal becomes reachable only together with its field-identical task-outcome projection event.

**Current behavior:** Executor and review journals infer or emit terminal classes from separate stores, and a crash can leave a route, process, or task face without one canonical outcome joined to native launch fidelity and available usage.

**Desired behavior:** The current fenced owner or authorized reaper signs one `routed_attempt_completed`. Implementation attempts append it atomically with the exact `routed_task_outcome` mapping; review attempts append only the parent. Claim loss, preflight failure, cancellation, and bounded crash reaping become typed outcomes, while failed resolutions never gain an outcome.

**Scope:**
- Implement current-generation outcome creation, validation, idempotent retry, bounded usage/fidelity fields, and overdue resolved-attempt discovery.
- Append the implementation two-event composite with the exact class/verification and shared-field table; append review outcomes as one parent event.
- Prove every terminal cause, mapping, interruption, reaper, duplicate, stale-generation, and failed-resolution case.

**Out of scope:**
- Review finding quality, review-process aggregation, report publication, and inferring a terminal from elapsed time alone.

**Acceptance criteria:**
- [ ] Every resolved implementation and review attempt, including claim loss, preflight failure, cancellation, and reaped crash, converges on exactly one current-generation outcome.
- [ ] Every implementation outcome and `routed_task_outcome` appear in one atomic append with identical identities, usage, and proof and exactly the permitted outcome, class, and verification combination.
- [ ] Failed resolutions emit no outcome; stale owners, conflicting repeats, invalid mappings, missing usage provenance, and partial composites cannot become canonical.
- [ ] Fresh recovery finds every overdue resolved attempt and either reattaches, proves absence before ownership transfer, or remains visibly non-accepting without a replacement launch.

**Key interfaces:**
- `OutcomeWriter`, `OutcomeComposite`, `OutcomeClass`, `AttemptReaper`
- `CompleteAttempt(ctx, AttemptTerminal)`, `ReapResolved(ctx, AttemptID)`
- `routed_attempt_completed`, `routed_task_outcome`

**Traces:** prd:planner-driven-model-routing/FR-20 · prd:planner-driven-model-routing/FR-21 · prd:planner-driven-model-routing/EVT-5 · prd:implement-now/FR-25 · prd:implement-now/EVT-10 · design:routing-evidence-v1#3.5 · design:routing-evidence-v1#DINV-7 · design:routing-evidence-v1#DINV-13 · design:routing-evidence-v1#A-4 · design:routing-evidence-v1#A-5

### T8 — Persist complete review evidence atomically

| Field | Value |
|---|---|
| **Blocked by** | T7 |
| **Size band** | 260–300 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | rung=3 |
| **Pattern** | — |
| **Touches** | internal/routingevidence review composites, implreview normalized findings and dispositions, review-process reducer, review projection fixtures |

**Goal:** A review pass, its complete normalized finding set, every attributed disposition, and the process aggregate become trustworthy only through exact revision-bound canonical composites.

**Current behavior:** Impl-review reports and host comments carry findings and verdicts, while executor review terminals use separate obligation and publication stores; no canonical event chain bijectively joins the routed reviewer attempt, pass, findings, native author acts, and aggregate process.

**Desired behavior:** A verified review attempt can append one pass terminal together with exactly one child per normalized finding in a single CAS. Dispositions exact-bind native medium act and author proof. The process terminal names the complete required pass set and can be satisfied only when every revision, context, verdict, finding, and aggregate join is valid; incomplete inputs yield a typed non-accepting terminal.

**Scope:**
- Define and validate assurance-pass, finding, disposition, and review-process payloads while reusing impl-review's owned pass and aggregate semantics.
- Build atomic pass/finding composites and exact native-act disposition verification through consumer-owned medium ports.
- Reduce complete, missing, invalid, stale, conflicting, and non-accepting pass sets without relocating merge-lane, band, or verdict policy.

**Out of scope:**
- Generating review findings, choosing review depth/pass count, computing a new verdict policy, and publishing proposal comments.

**Acceptance criteria:**
- [ ] A pass becomes reachable only with its complete bounded normalized finding set, whose IDs, bytes, count, pass, attempt, process, context, proposal, and exact revision match bijectively.
- [ ] Every disposition binds one finding to one native medium act and independently verified actor proof; fabricated, quoted, cross-finding, or unattributed acts are rejected.
- [ ] A process terminal names the complete required pass set and deterministic aggregate; missing, stale, invalid, non-accepting, leaked-context, or favorable-subset inputs cannot produce `satisfied`.
- [ ] Proposal/report projection fixtures round-trip the exact canonical pass, finding, disposition, and process identities without raw candidates or model output.

**Key interfaces:**
- `PassComposite`, `FindingEvent`, `DispositionEvent`, `ProcessTerminal`, `NativeActVerifier`
- `CompletePass(ctx, PassComposite)`, `RecordDisposition(ctx, DispositionEvent)`, `CompleteProcess(ctx, ProcessTerminal)`
- `assurance_pass_completed`, `finding_persisted`, `disposition_recorded`, `review_process_completed`

**Traces:** prd:planner-driven-model-routing/FR-21 · prd:planner-driven-model-routing/FR-24 · prd:planner-driven-model-routing/EVT-9 · prd:impl-review/FR-27 · prd:impl-review/FR-29 · prd:impl-review/FR-30 · design:routing-evidence-v1#3.6 · design:routing-evidence-v1#DINV-9 · design:routing-evidence-v1#DINV-13 · design:routing-evidence-v1#A-6

### T9 — Invalidate defective evidence without rewriting history

| Field | Value |
|---|---|
| **Blocked by** | T4, T8 |
| **Size band** | 210–280 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | rung=3 |
| **Pattern** | — |
| **Touches** | routing invalidation authorizer and reducer, affected-chain discovery, operator remediation brief, qualification-defect fixtures |

**Goal:** A discovered policy or qualification defect additively revokes every exact affected open evidence chain, preserves unaffected work and history, and briefs rather than silently rewrites already accepted artifacts.

**Current behavior:** Routing qualifications can be invalidated in native-policy memory and review obligations can fail, but no canonical event names the exact affected route, outcome, pass, process, and artifact set or removes it from current acceptance without deletion.

**Desired behavior:** An authorized evidence owner derives a domain-separated invalidation identity from the defective generation, sorted affected identities, disposition, and authorization. The reducer excludes exact invalidated evidence, requires replacement work for open subjects, preserves unaffected chains, and emits a complete operator decision brief for accepted artifacts.

**Scope:**
- Implement affected-chain discovery, closed invalidation payload/identity, authorization validation, append, reduction, and current-acceptance exclusion.
- Distinguish `rerun_required` open work from `operator_remediation` accepted work without mutating or deleting prior events.
- Prove exact blast radius, replacement evidence, unaffected continuity, repeat, conflict, and late-discovery behavior.

**Out of scope:**
- Automatically reverting accepted code, deciding the human remediation, changing qualification policy, and launching replacement work without ordinary admission.

**Acceptance criteria:**
- [ ] A seeded defect produces one authorized invalidation naming every and only affected policy/profile generation, event, attempt, process, pass, and artifact identity.
- [ ] Invalidated open evidence cannot authorize claim, review satisfaction, projection acceptance, or activation until exact replacement evidence passes; unaffected routes continue.
- [ ] Accepted or merged artifacts remain unchanged and yield one complete operator decision brief rather than a silent valid-routing claim or destructive rollback.
- [ ] Repeat is byte-identical and idempotent; altered blast radius, disposition, authorization, or same-ID content conflict fails without weakening the original history.

**Key interfaces:**
- `Invalidation`, `InvalidationAuthorizer`, `AffectedSet`, `RemediationBrief`
- `DiscoverAffected(State, QualificationDefect)`, `Invalidate(ctx, Invalidation)`, `CurrentAcceptance(View)`
- `routing_evidence_invalidated`

**Traces:** prd:planner-driven-model-routing/FR-21 · prd:planner-driven-model-routing/UB-10 · prd:planner-driven-model-routing/UB-14 · prd:planner-driven-model-routing/EVT-10 · design:routing-evidence-v1#3.6 · design:routing-evidence-v1#DINV-1 · design:routing-evidence-v1#A-7

### T10 — Checkpoint activation against exact merged evidence

| Field | Value |
|---|---|
| **Blocked by** | T4, T9 |
| **Size band** | 200–270 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | rung=3 |
| **Pattern** | — |
| **Touches** | routing activation manifest and checkpoint, release-gate producer, exact-mainline proof verifier, activation abuse fixtures |

**Goal:** Production routing can reference one canonical activation checkpoint only when an operator-signed manifest proves the complete required evidence set at one exact human-merged mainline object.

**Current behavior:** Repository configuration keeps authenticated review inactive by comment and routing policy can be loaded independently, but no canonical checkpoint binds activation policy, exact mainline object, proof manifest, operator authorization, and current non-invalidated evidence.

**Desired behavior:** The release gate verifies a closed signed manifest against exact merged mainline and the canonical reducer, then appends one signed activation checkpoint carrying the evaluated proof identities and result. Missing, stale, invalidated, unsigned, unmerged, cross-repository, or changed proof input cannot authorize an activation consumer.

**Scope:**
- Define the closed activation manifest, exact-mainline human-merge proof, union-predicate input, checkpoint, and proof-digest contracts.
- Implement release-gate verification and least-privilege checkpoint append against the current non-invalidated evidence view.
- Prove exact object and proof-set binding, operator signature, repeat, conflict, replay, invalidation, and mainline movement cases.

**Out of scope:**
- Turning on production configuration, choosing rollout cohorts, shadow/canary behavior, and changing the PRD release predicate.

**Acceptance criteria:**
- [ ] A checkpoint is accepted only for one exact human-merged mainline object, activation-policy version, operator-signed manifest digest, complete proof set, and current canonical evidence head.
- [ ] Missing, extra, stale, invalidated, unsigned, forged, unmerged, moved-mainline, cross-repository/ref, and generic-writer checkpoints are non-authoritative.
- [ ] Byte-identical replay is idempotent; changing any policy, mainline, manifest, proof identity, predicate result, producer, or generation yields a different identity or a typed conflict.
- [ ] Inspection finds no production config mutation or staged cohort logic in this task; activation remains a downstream human-merged change consuming this proof.

**Key interfaces:**
- `ActivationManifest`, `ActivationCheckpoint`, `MainlineProof`, `ActivationPredicate`
- `VerifyActivation(State, ActivationManifest, MainlineProof)`, `AppendCheckpoint(ctx, ActivationCheckpoint)`
- `activation_checkpoint`

**Traces:** prd:planner-driven-model-routing/FR-21 · design:routing-evidence-v1#3.1 · design:routing-evidence-v1#3.2 · design:routing-evidence-v1#3.12 · design:routing-evidence-v1#DINV-10 · design:routing-evidence-v1#A-9

### T11 — Derive every operator face from canonical evidence

| Field | Value |
|---|---|
| **Blocked by** | T8, T9, T10 |
| **Size band** | 230–295 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | rung=3 |
| **Pattern** | — |
| **Touches** | routing subject read API and cache, proposal/report/tracker projectors, fresh-clone verifier, operator diagnostics fixtures |

**Goal:** Proposal, report, tracker, audit, and measurement faces are reproducible checked projections of one exact canonical evidence head and can never override a missing, non-accepting, or invalidated chain.

**Current behavior:** Executor journals, operator reports, proposal comments, and tracker bodies carry partially overlapping review and routing facts, and no fresh-clone reader can render the complete chain or reject a favorable stale face against one authority.

**Desired behavior:** One typed read API fetches and reduces an exact ref head into demand, attempt, process, invalidation, and activation views. Disposable caches key by that head. Each medium projection records canonical event IDs, proof, and digest and is rejected when missing, extra, stale, changed, or disconnected; a fresh-clone verifier explains every edge and defect without session state.

**Scope:**
- Implement exact-head typed reads, disposable cache keys, deterministic projections, projection verification, and bounded measurement views.
- Bind existing proposal, report, and tracker faces to canonical identities without making those media launch or acceptance authorities.
- Build an operator verifier that renders chain, state, current owner, missing edge, invalidation, projection diff, and exact git object without payload secrets.

**Out of scope:**
- Changing proposal or tracker ownership, retaining prompts/code, deriving route or verdict policy, and accepting from a cache without exact-read proof.

**Acceptance criteria:**
- [ ] Fresh clones reconstruct byte-identical demand, route, outcome, pass, finding, disposition, process, invalidation, and activation views from the exact fetched ref.
- [ ] Proposal, report, and both tracker faces round-trip the canonical event set and reject every missing, extra, stale, changed, favorable-subset, non-accepting, or invalidated projection.
- [ ] Cache reuse is possible only for the exact evidence-ref head and never authorizes launch or acceptance without a current exact-read proof.
- [ ] Operator diagnostics name task or proposal, harness, role, policy, semantic component, event, attempt, process identity, state, and proof without prompt, code, secret, raw model output, or transcript.

**Key interfaces:**
- `Reader`, `SubjectViews`, `Projection`, `ProjectionDigest`, `FreshCloneVerifier`
- `ReadViews(ctx, SubjectID)`, `Project(View, Medium)`, `VerifyProjection(Projection, View)`
- demand, attempt, review-process, invalidation, and activation read models

**Traces:** prd:planner-driven-model-routing/FR-21 · prd:planner-driven-model-routing/FR-24 · prd:planner-driven-model-routing/UB-7 · prd:planner-driven-model-routing/NFR-4 · design:routing-evidence-v1#3.7 · design:routing-evidence-v1#3.9 · design:routing-evidence-v1#DINV-2 · design:routing-evidence-v1#DINV-9 · design:routing-evidence-v1#A-6 · design:routing-evidence-v1#A-8

### T12 — Falsify the complete routing-evidence boundary

| Field | Value |
|---|---|
| **Blocked by** | T5, T6, T7, T8, T9, T10, T11 |
| **Size band** | 250–300 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | rung=3 |
| **Pattern** | — |
| **Touches** | routing-evidence acceptance harness, crash-at-boundary matrix, producer abuse corpus, hosted-ref rehearsal record, evidence audit report |

**Goal:** One end-to-end proof demonstrates that canonical evidence, admission, recovery, projection, invalidation, and activation remain correct across every event kind, race, crash boundary, and authority attack.

**Current behavior:** Package tests cover execution demand, native routing, generic git-ref updates, review obligations, and publication separately; there is no fresh-clone acceptance corpus spanning one complete route and review process or mutation proof for the design's fifteen invariants.

**Desired behavior:** A shared harness runs exact, promoted, unsupported, implementation, review, recovery, invalidation, and activation chains against the local real git adapter, crashes before and after every durable and spawn boundary, mutates every proof, join, and projection, and produces a reviewable requirement and invariant evidence index. One hosted-ref rehearsal proves the same protection predicate without making the hosted provider part of core logic.

**Scope:**
- Drive the complete event family through canonical encode, producer proof, CAS append, exact-read admission, fenced launch, atomic terminal, reducer, projection, invalidation, and checkpoint paths.
- Inject interruption, concurrent writer/owner, ref outage/rewrite, producer abuse, projection drift, partial composite, ambiguous launch, and stale evidence faults.
- Emit the bounded conformance, crash, security, payload, hosted-ref, and fresh-clone reports required by design A-1 through A-9.

**Out of scope:**
- Production activation, adaptive routing, model-quality judgment, cross-harness fallback, and live provider inference in hermetic CI.

**Acceptance criteria:**
- [ ] The acceptance matrix covers every design event kind, structural state, producer class, invariant, and A-1 through A-9 with a deterministic evidence reference and no omitted cell.
- [ ] Crash and race injection before/after append, CAS, exact-read, claim, spawn, composite, transfer, projection, invalidation, and checkpoint boundaries never permits duplicate launch, partial authority, or silent success.
- [ ] Reverting each launch-proof, owner-fence, atomicity, producer-authority, projection, invalidation, and activation guard makes its designated acceptance case fail.
- [ ] Local bare-git and hosted-ref rehearsal reports prove non-rewriting ref continuity, fresh-read reconstruction, bounded retries, payload exclusion, and operator-visible failure under outage, force, delete, or rewrite.

**Key interfaces:**
- the routing-evidence acceptance and mutation corpus
- `FaultPoint`, `BoundaryTrace`, `AcceptanceMatrix`, `HostedRefRehearsal`, `EvidenceAudit`
- design acceptance A-1 through A-9

**Traces:** prd:planner-driven-model-routing/FR-20 · prd:planner-driven-model-routing/FR-21 · prd:planner-driven-model-routing/FR-24 · prd:planner-driven-model-routing/UB-7 · prd:planner-driven-model-routing/UB-10 · prd:planner-driven-model-routing/UB-14 · prd:planner-driven-model-routing/NFR-4 · prd:planner-driven-model-routing/NFR-8 · design:routing-evidence-v1#3.11 · design:routing-evidence-v1#A-1 · design:routing-evidence-v1#A-2 · design:routing-evidence-v1#A-3 · design:routing-evidence-v1#A-4 · design:routing-evidence-v1#A-5 · design:routing-evidence-v1#A-6 · design:routing-evidence-v1#A-7 · design:routing-evidence-v1#A-8 · design:routing-evidence-v1#A-9

## Coverage

| Requirement | Tasks |
|---|---|
| prd:planner-driven-model-routing/FR-20 | T3, T7, T12 |
| prd:planner-driven-model-routing/FR-21 | T1, T2, T3, T4, T6, T7, T8, T9, T10, T11, T12 |
| prd:planner-driven-model-routing/FR-24 | T1, T3, T5, T8, T11, T12 |
| prd:planner-driven-model-routing/UB-7 | T2, T3, T6, T11, T12 |
| prd:planner-driven-model-routing/UB-10 | T9, T12 |
| prd:planner-driven-model-routing/UB-14 | T9, T12 |
| prd:planner-driven-model-routing/NFR-4 | T1, T11, T12 |
| prd:planner-driven-model-routing/NFR-8 | T4, T5, T12 |
| prd:planner-driven-model-routing/EVT-1 | T1 |
| prd:planner-driven-model-routing/EVT-2 | T1, T5 |
| prd:planner-driven-model-routing/EVT-3 | T1, T5 |
| prd:planner-driven-model-routing/EVT-5 | T1, T7 |
| prd:planner-driven-model-routing/EVT-8 | T1 |
| prd:planner-driven-model-routing/EVT-9 | T1, T8 |
| prd:planner-driven-model-routing/EVT-10 | T1, T9 |
