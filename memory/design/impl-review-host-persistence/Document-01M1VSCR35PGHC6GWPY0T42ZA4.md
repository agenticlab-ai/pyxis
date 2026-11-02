# impl-review host persistence — technical design

| Field | Value |
|---|---|
| **Status** | Draft → Approved *(merge requires Approved; merge is the freezing event)* |
| **Owner** | @mdarabi |
| **Approver(s)** | @mdarabi (operator and security reviewer) |
| **Date** | 2026-08-20 |
| **Relevant-when** | designing or changing impl-review report publication, reviewer-to-host handoff, proposal-medium credentials, publication idempotency, native receipts, exact-read verification, EVT-13, cross-contract publication joins, or host-side recovery |
| **Scope** | The complete impl-review successor: exact-incorporates the routed-assurance process and adds one impl-review-owned host persistence adapter that accepts the outer process aggregate, publishes exactly once to GitHub or Carina, returns and exact-reads a native receipt, emits EVT-13, and joins a host terminal without granting proposal or evidence-write authority to reviewer execution. **Not:** implement-now claim/capacity/recovery scheduling, native catalog contents, report judgment/calibration, merge authority, or a distinct code-host reviewer principal. |
| **Tracking** | issue #1029 · PR #1056 |
| **Builds-on** | docs/tech-design/execution-demand-contract-v1.md · docs/tech-design/native-routing-policy-v1.md · docs/tech-design/routing-evidence-v1.md · docs/tech-design/codegate-v1.md · docs/tech-design/prd-review-artifacts.md · docs/tech-design/merge-lane-v1.md · docs/tech-design/context-pack-v1.md |
| **Supersedes** | docs/tech-design/impl-review-routed-assurance.md |
| **Serves** | prd:impl-review/FR-3 · prd:impl-review/FR-4 · prd:impl-review/FR-5 · prd:impl-review/FR-6 · prd:impl-review/FR-7 · prd:impl-review/FR-12 · prd:impl-review/FR-14 · prd:impl-review/FR-15 · prd:impl-review/FR-16 · prd:impl-review/FR-18 · prd:impl-review/FR-19 · prd:impl-review/FR-20 · prd:impl-review/FR-21 · prd:impl-review/FR-22 · prd:impl-review/FR-23 · prd:impl-review/FR-24 · prd:impl-review/FR-25 · prd:impl-review/FR-26 · prd:impl-review/FR-27 · prd:impl-review/FR-28 · prd:impl-review/FR-29 · prd:impl-review/FR-30 · prd:impl-review/FR-31 · prd:impl-review/FR-32 · prd:impl-review/FR-33 · prd:impl-review/FR-34 · prd:impl-review/FR-35 · prd:impl-review/FR-36 · prd:impl-review/FR-37 · prd:impl-review/NR-1 · prd:impl-review/NR-2 · prd:impl-review/NR-3 · prd:impl-review/NR-5 · prd:impl-review/NR-6 · prd:impl-review/NR-7 · prd:impl-review/NR-8 · prd:impl-review/UB-3 · prd:impl-review/UB-9 · prd:impl-review/UB-10 · prd:impl-review/UB-11 · prd:impl-review/UB-12 · prd:impl-review/UB-13 · prd:impl-review/UB-14 · prd:impl-review/UB-15 · prd:impl-review/UB-16 · prd:impl-review/UB-17 · prd:impl-review/UB-18 · prd:impl-review/UB-19 · prd:impl-review/UB-20 · prd:impl-review/UB-21 · prd:impl-review/UB-22 · prd:impl-review/UB-23 · prd:impl-review/ABUSE-3 · prd:impl-review/ABUSE-4 · prd:impl-review/ABUSE-5 · prd:impl-review/ABUSE-6 · prd:impl-review/ABUSE-7 · prd:impl-review/NFR-6 · prd:impl-review/NFR-7 · prd:impl-review/NFR-8 · prd:impl-review/NFR-10 · prd:impl-review/NFR-11 · prd:impl-review/NFR-12 · prd:impl-review/EVAL-1 · prd:impl-review/EVAL-2 · prd:impl-review/EVAL-3 · prd:impl-review/EVAL-4 · prd:impl-review/EVAL-5 · prd:impl-review/EVAL-6 · prd:impl-review/EVAL-7 · prd:impl-review/EVAL-8 · prd:impl-review/EVAL-9 · prd:impl-review/EVAL-10 · prd:impl-review/EVAL-11 · prd:impl-review/EVT-2 · prd:impl-review/EVT-3 · prd:impl-review/EVT-6 · prd:impl-review/EVT-7 · prd:impl-review/EVT-8 · prd:impl-review/EVT-9 · prd:impl-review/EVT-10 · prd:impl-review/EVT-11 · prd:impl-review/EVT-12 · prd:impl-review/EVT-13 · prd:planner-driven-model-routing/FR-14 · prd:planner-driven-model-routing/FR-25 · prd:planner-driven-model-routing/FR-26 · prd:planner-driven-model-routing/NR-6 · prd:planner-driven-model-routing/NR-9 · prd:planner-driven-model-routing/UB-12 · prd:planner-driven-model-routing/UB-13 |
| **Security/privacy impact** | yes — the host adapter holds proposal-write authority and durable publication state; reviewer execution and pass contexts receive neither. Receipt and proof references are non-secret, raw proof material remains outside every child, and code-host author identity is non-authoritative. reviewer: @mdarabi. |
| **Template version** | tech-design-template/v1 |

## 0. Read to rule — the human reviewer's section (~6 min)

*The body is the contract; this section only routes attention — on any conflict the body governs; machines ignore it.*

| # | The call | Could have gone | If you disagree |
|---|---|---|---|
| 1 | Impl-review's host persistence adapter is the sole proposal publisher acting for the reviewer program | reviewer child or implement-now publishes | publication ownership forks and credentials enter an untrusted execution boundary → §3.2, #DINV-12 |
| 2 | GitHub comment and Carina record publication is one durable idempotent state machine that returns the exact medium receipt | clean exit, body search, or blind POST retry | a crash can create duplicate or unlocatable evidence → §3.4, #DINV-14 |
| 3 | A counted aggregate requires one verified EVT-13 and, at a hosted seam, one reciprocal terminal joined by seven exact field pairs | accept a plausible-looking comment or infer similar IDs | neither corpus can prove it observed the same publication → §3.6, #DINV-16 |
| 4 | Native receipt and attempt-proof reference remain separate; publication-attempt ID is not the host obligation | overload one string as all three | artifact location, producer proof, and retry identity collapse into replayable ambiguity → §3.3, §3.6 |
| 5 | Artifact author may equal the implementation/operator account and never proves completion | provision and distribute a shared reviewer bot credential | onboarding gains secret-sharing risk without strengthening receipt/proof verification → §3.7, #DINV-17 |

**Sanity-check the proof** — one accepted aggregate produces at most one medium object; every verified publication has one exact-readable receipt, one distinct proof reference, one EVT-13, and—when hosted—one exact reciprocal terminal; zero/surplus/malformed handoff publishes nothing; no child can reach proposal or persistence credentials.

**The author's open questions** — none. The medium adapters, retry policy, receipt grammars, and reciprocal implement-now EVT-17 are closed below.

## 1. Problem

The routed-assurance design made the proposal report a checked projection of canonical review evidence but left the writer inside the reviewer flow. That is unsafe once implement-now launches the reviewer as an isolated, untrusted child: a child with proposal credentials can manufacture its own completion evidence, while a caller-side publisher would silently transfer ownership of impl-review's FR-6 artifact contract.

The persistence boundary must also survive the failure window around a remote write. GitHub comment creation does not provide a reusable idempotency key, so repeating an ambiguous POST can duplicate the sole review artifact. A clean process exit, proposal-wide search, comment author, or similar-looking event fields cannot prove which publication belonged to the launched review attempt.

## 2. Decision

**Decision:** The routed-assurance process is exact-incorporated unchanged. Its outer isolated execution returns one typed `ProcessHandoff` through a bounded one-way channel; the trusted host coordinator alone validates it and materializes the canonical completed aggregate. An impl-review-owned host persistence adapter, running outside every reviewer/pass context, preflights its medium capability, durably admits one publication attempt, performs the sole proposal write, exact-reads only the returned native receipt, and appends one EVT-13. The invoking host supplies the obligation/process/proposal/revision binding and distinct attempt-proof reference, consumes the receipt, and appends its separately contracted reciprocal terminal; it never writes the proposal or redefines the report.

The adapter never retries a GitHub write whose acknowledgement is indeterminate. It either reconciles a previously persisted exact receipt or terminates non-accepting, so response loss cannot create a second comment. The Carina adapter uses the dedicated proposal-record slice defined here—not `WorkTracker.appendDecisionRecord`—and exposes the same closed result through a local atomic create-if-absent. Verification ignores code-host author and requires the exact receipt, protected attempt proof, report binding, and field-identical EVT-13↔host terminal join.

## 3. The design

### 3.1 Exact incorporation of routed assurance

`impl-review-routed-assurance.md` is incorporated at immutable Git blob `6b147d3bb5f1073d87596e862bba1cf9eadbe66b`. Its assurance×band join, process/pass/context identities, review-role routing, append-first route admission, cacheable base pack, mutually blind lens/pass topology, complete-set aggregation, staleness, finding grammar, harvest, downgrade-only merge-lane use, `DINV-1..DINV-10`, risks `R1..R6`, and acceptance `A-1..A-11` remain binding and executable without semantic change.

This successor replaces only the predecessor's report-persistence performer and completion predicate. A generated conformance manifest pins the blob and maps every carried header anchor, invariant, risk, and acceptance row to its prior check/digest. The predecessor's frozen dependency on `e2e-verification-skills` is retired rather than carried: live `e2e-verification-skills-v2` is a downstream consumer whose impl-review conformance suite runs against this successor, not an upstream `Builds-on` edge. That direction is cycle-free and lets the consumer replace its stale predecessor edge independently. The manifest rejects the obsolete edge rather than carrying it forward. Missing, mutated, stale-dependency, or favorable-subset incorporation fails before activation.

### 3.2 Capability and ownership boundary

The outer reviewer execution and every blind pass context run without proposal-medium credentials, canonical evidence producer authority, persistence-store write access, or an IPC endpoint capable of arbitrary host calls. The outer execution's only outward result is one bounded `ProcessHandoff` on a host-owned pipe after its complete-set reducer settles; only the trusted host coordinator may validate that untrusted handoff and produce a canonical `AggregateHandoff`.

```go
type PublicationBinding struct {
	ReviewObligationID string
	ReviewProcessID    string
	ProposalID         string
	ReviewedRevision   string
	AttemptProofRef    string
}

type VerifiedPublicationBinding struct {
	PublicationBinding
	ProofEvidenceRef string
}

type ProcessCoordinator interface {
	Prepare(ctx context.Context, subject ReviewSubject) (AdmittedProcess, error)
	Complete(ctx context.Context, process AdmittedProcess, handoff ProcessHandoff) (AggregateHandoff, error)
}

type CompletedAggregate struct {
	ProcessID, ProposalID, ReviewedRevision string
	Report                                  implreview.ReportV2
}

type ProcessHandoff struct {
	Status        HandoffStatus // completed|missing|surplus|malformed|mismatched
	PassTerminals []PassTerminal
	Aggregate     *ProcessAggregate
	DiagnosticDigest string
}

type AggregateHandoff struct {
	Status           HandoffStatus // completed|missing|surplus|malformed|mismatched
	Aggregate        *CompletedAggregate
	DiagnosticDigest string
}

type PublicationTerminal struct {
	PublicationAttemptID      string
	IdempotencyID             string
	Status                    PublicationStatus // verified|failed
	ArtifactRef               *string
	NativeReceipt             *string
	AttemptProofRef           *string
	FailureClass, Reason      *string
	EVT13Ref                  string
}

type HostPersistenceAdapter interface {
	Preflight(ctx context.Context, repository string) error
	Complete(ctx context.Context, binding VerifiedPublicationBinding, handoff AggregateHandoff) (PublicationTerminal, error)
	ExactRead(ctx context.Context, nativeReceipt string) (implreview.ReportV2, error)
}

type CarinaReviewRecordStore interface {
	Preflight(ctx context.Context, repository string) error
	CreateIfAbsent(ctx context.Context, record CarinaReviewRecord) (CarinaReviewReceipt, error)
	ExactRead(ctx context.Context, receipt CarinaReviewReceipt) (CarinaReviewRecord, error)
}
```

`ProcessCoordinator` is the trusted host realization of the carried routed-assurance allocator. `Prepare` CAS-allocates or resumes the exact subject generation, resolves and append/exact-reads the complete route set before launch, and returns already-admitted launch inputs plus the authoritative `review_process_id`. A hosting executor adopts that returned value as its `review_attempt_id`; it never invents a parallel process identity. The isolated outer execution receives only those admitted inputs and returns one closed `ProcessHandoff` containing its pass/outcome terminals and aggregate—never an evidence-store handle. `Complete` validates that handoff, appends/exact-reads the carried route-outcome/pass/process events under host authority, and returns one canonical `AggregateHandoff` whose completed arm carries exact event references. Missing, surplus, malformed, or mismatched process handoff persists a non-accepting process terminal and returns the corresponding invalid arm for failed EVT-13 with zero proposal write.

`HostPersistenceAdapter` is consumer-owned by impl-review and implemented by its GitHub and Carina packages. The host constructs it and supplies only the proof-verified exact-read process binding and the closed aggregate handoff terminal; a hosting executor obtains `VerifiedPublicationBinding` from its protected-state verifier before this call, and proof failure never reaches publication. The adapter owns rendering, schema, medium write, receipt grammar, exact-read, artifact-validation predicate, and EVT-13. `missing|surplus|malformed|mismatched` allocates the publication terminal but performs zero medium writes and returns failed `artifact_unverified`; `mismatched` is the explicit arm for a structurally valid aggregate whose process, publication attempt, proposal, or revision differs from the expected binding. A committed EVT-13—verified or failed—is returned/replayed as `PublicationTerminal`, so the caller can append the field-identical EVT-17 without reconstructing data from an error string. A side-effect-free `Preflight` proves the authenticated actor and current repository permission can read and create the medium object and that durable adapter state/event sinks are writable; an API that cannot prove those capabilities is unsupported and launches no reviewer. Implement-now contains no comment/decision-record publisher and no second artifact parser.

Carina's implementation is a narrow ProposalService-owned review-record slice, not an extension of the frozen WorkTracker port or its closed `triage-disposition|charter|foundation` namespaces. It stores one immutable `impl-review-report/v2` typed decision record beneath the configured proposal-store root, addressed by repository, proposal, and the idempotency ID. The record schema is closed: schema version, idempotency ID, publication attempt, obligation, process, proposal, revision, artifact digest, canonical artifact bytes, created timestamp supplied by the injected clock, and record digest. `CreateIfAbsent` uses an identity-derived file and local exclusive-create/atomic-rename discipline; exact replay byte-compares the complete record and returns the existing receipt, while any different content at that identity is a conflict. `ExactRead` opens only the identity named by the receipt, validates repository/proposal containment plus the record digest, and returns no latest-wins or proposal-wide search result. This slice and its conformance kit are owned by this design; `local-work-tracker-v2` remains unchanged and its `appendDecisionRecord` is never called.

- **DINV-11** — Exactly one component—the impl-review host persistence adapter—can perform FR-6 for a completed process.
- **DINV-12** — Reviewer execution, every pass/lens child, repository content, and the invoking host's implementation worker hold no proposal/evidence/persistence write capability; all predecessor admission/outcome/process writes execute in the trusted host coordinator around the one-way process handoff.

### 3.3 Identities and grammars

The invoking host echoes the exact-read impl-review `review_process_id`, supplies `review_obligation_id`, proposal, revision, and a non-secret `attempt_proof_ref` whose protected source is outside every child write surface. The host mints that proof only after implementation resources are released and binds it to the returned process identity; predictability of the non-secret process ID never weakens proof. `publication_handoff_id = sha256("impl-review-handoff/v1" || review_obligation_id || review_process_id || proposal_id || reviewed_revision || handoff_status || aggregate_or_diagnostic_digest)` is the stable retry/concurrency lookup key. Under one compare-and-swap, impl-review allocates a fresh `publication_attempt_id` only when that handoff identity has no state; duplicate or restarted calls exact-read and reuse the allocated ID and terminal, while a different handoff for the same review process is a conflict. The publication attempt is a child of the review process and never aliases the stable host obligation or review-process identity.

`idempotency_id = sha256("impl-review-publication/v1" || review_obligation_id || review_process_id || publication_attempt_id || proposal_id || reviewed_revision || publication_handoff_id)`. Because `publication_handoff_id` includes the closed status and aggregate-or-diagnostic digest, the formula is total for completed, missing, surplus, malformed, and mismatched handoffs. `artifact_ref` is `sha256:<lowercase-hex>` over the canonical rendered-plus-machine artifact bytes. GitHub native receipts use `github-comment:<repository-node-id>:<comment-database-id>`; Carina uses `carina-review-record:<repository-id>:<proposal-id>:<idempotency-id>`, with each component encoded by the ProposalService grammar and the final component equal to the admitted idempotency ID. `attempt_proof_ref` uses the canonical evidence-reference grammar supplied by the host and is opaque to the medium adapter; the host verifier resolves it. Unknown versions, empty components, wrong grammar, equality between receipt and proof, or a second handoff identity for one review process fails before acceptance.

All durable binding/idempotency/receipt state lives in the host-owned adapter store with producer authority unavailable to repository credentials and every worker/reviewer process. The adapter stores no raw proof secret; only the non-secret reference crosses the boundary.

- **DINV-13** — Obligation, process, publication attempt, artifact, native receipt, and attempt proof are distinct typed identities with one declared derivation or authority each.

### 3.4 Exactly-once publication state machine

One append-only `review-publication/v1` state machine reduces by idempotency ID:

```text
accepted → armed → receipt_recorded → exact_read → evt13_verified
                   ↘ evt13_failed
armed/unknown_ack  → evt13_failed
```

Before any remote call the adapter resolves `publication_handoff_id`, then append/exact-reads `accepted` with that identity, the complete binding, handoff status/digest, artifact digest when valid, medium, policy generation, and its once-allocated publication attempt. A non-completed handoff goes directly to failed EVT-13 with zero remote calls. A completed handoff append/exact-reads `armed`; only the owner of that exact fence may issue one create call. A returned receipt is append/exact-read before parsing the remote object. Exact replay at any persisted state returns the same publication terminal; a second caller races through the same CAS and performs zero remote calls; conflicting replay is `artifact_untrusted`.

GitHub comment creation has no trusted server-side idempotency key. Therefore a timeout, connection loss, cancellation, or host death after `armed` but before durable receipt makes the attempt `unknown_ack`; the adapter never issues another create call for that idempotency ID and emits/replays a failed infrastructure EVT-13 when its event sink is available. This may leave one unverified comment, but can never manufacture a duplicate. A later review uses a new process/publication attempt; it does not adopt or overwrite the ambiguous object. Carina's design-owned review-record store uses the identity-derived exclusive create above; exact replay returns the same receipt after full-record comparison, and conflicting bytes fail without replacement.

After `receipt_recorded`, recovery performs only exact-read by that receipt, never proposal-wide body/author search. Event-append or caller-ack failure replays from durable receipt state without another medium write. A closed policy bounds state/event reconciliation attempts and elapsed time; exhaustion is a failed infrastructure terminal, never success.

- **DINV-14** — One publication attempt performs at most one externally observable create; acknowledgement ambiguity can reduce availability but never authorize a second create.
- **DINV-15** — Crash/restart after a durable receipt performs zero new proposal writes and converges by exact-read/event replay.

### 3.5 Artifact validation

The adapter renders the unchanged `impl-review-report/v2` human and machine faces from the typed aggregate, normalizing LF before render and parse. It rejects zero, surplus, malformed, or binding-mismatched handoffs before publication. On exact-read it normalizes CRLF to LF before both fence extraction and byte-fidelity parsing, then requires one schema-valid report whose process, proposal, revision, aggregate digest, and artifact digest match the admitted binding.

Unrelated proposal comments/records are never candidates because exact-read begins from the native receipt. Comment author is retained only as operational telemetry. Same-author and different-author fixtures follow identical verification paths; FR-16's later operator-disposition author check remains unchanged.

### 3.6 EVT-13 and reciprocal host terminal

The persistence adapter emits exactly one `review_artifact_publication_terminal` EVT-13 per publication attempt. Verified requires `artifact_ref`, `native_receipt`, and `attempt_proof_ref` and forbids failure fields. Failed requires one of `artifact_unverified|artifact_untrusted|infrastructure` plus reason; its three evidence fields remain schema-optional and never establish acceptance.

For implement-now hosting, EVT-13 joins exactly one EVT-17 for the same publication moment:

| impl-review EVT-13 | implement-now EVT-17 | Rule |
|---|---|---|
| `review_process_id` | `review_attempt_id` | required; identical grammar/value |
| `review_obligation_id` | `review_obligation_id` | required; identical grammar/value |
| `proposal_id` | `proposal_ref` | required; identical grammar/value |
| `reviewed_revision` | `revision` | required; identical grammar/value |
| `artifact_ref` | `artifact_ref` | identical presence/value; required when verified |
| `native_receipt` | `native_receipt` | identical presence/value; required when verified |
| `attempt_proof_ref` | `attempt_proof_ref` | identical presence/value; required when verified |

Statuses and failed failure classes are equal; failed EVT-17 sets `escalated=true`, while verified sets it false. `publication_attempt_id` stays impl-review-only. The adapter returns the exact EVT-13 projection/result to the caller; a missing or mismatched EVT-17 never causes republish. Instead, the caller replays its terminal append from the receipt-bound result and leaves the obligation pending until the pair verifies. Inferred joins and EVT-15-only evidence count nowhere.

- **DINV-16** — A counted hosted review has one satisfying EVT-9, one verified EVT-13, one verified reciprocal terminal, and exact equality across all seven declared pairs.

### 3.7 Security & privacy

The adapter store, event writer, code-host credential, and receipt cache are outside every implementation/reviewer workspace and denied by the child runtime's filesystem and process sandbox. The reviewer receives an allowlisted read-only repository root and only the provider authentication material required for inference; it cannot read the operator checkout, credential stores, agent socket, adapter IPC, or evidence keys. A backend unable to prove that confinement is unsupported.

`artifact_unverified` covers missing/surplus/malformed/binding-invalid aggregate or exact-receipt content. Adapter-emitted `artifact_untrusted` covers replay, conflicting idempotency state, or receipt/proof alias/substitution visible within the verified binding and adapter state. Protected-state rejection occurs in the hosting verifier before this adapter is called; cross-terminal mismatch occurs after EVT-17 and invalidates the pair without rewriting the immutable adapter terminal. `infrastructure` covers capability/auth, durable-store, remote write/read, unknown acknowledgement, event append, cancellation, or bounded-recovery failure. None becomes a satisfied process.

The adapter's code-host principal may equal or differ from the implementation/operator principal. Author identity neither establishes nor refutes completion; only receipt, protected proof, binding, and immutable terminals do.

- **DINV-17** — Transport identity is observational, while child capability denial and protected-state authority are structural completion inputs.
- **DINV-18** — Every failure leaves the review non-accepting and preserves enough durable state to prevent duplicate publication or inferred recovery.

### 3.8 Observability and operations

- **Events/logs** — EVT-13 plus process/publication/medium/state/receipt/proof-reference correlations; no artifact body, prompt, code, credential, raw proof, or model output.
- **Metrics** — preflight failures, accepted handoffs, create calls, acknowledgement ambiguity, receipt persistence, exact-read attempts, EVT-13 status/class, cross-terminal join outcomes, and reconciliation duration.
- **Alerts** — unknown acknowledgement, duplicate/conflicting state, protected-store access, direct child publication, receipt/proof alias, missing reciprocal terminal, or event mismatch.
- **Debugging** — an exact publication inspector reads durable adapter state, exact receipt, EVT-13, and reciprocal terminal without scanning proposal history.

### 3.9 Compatibility and activation

Historical v1/v2 reports remain readable but cannot satisfy a new publication attempt without the new receipt/proof/terminal chain. The successor lands inactive. Activation requires both this design and `implement-now-authenticated-review-obligations`, green GitHub and design-owned Carina ProposalService conformance, protected-state and child-capability inspection, the complete reciprocal EVT-13↔EVT-17 mismatch matrix, and the predecessor exact-incorporation manifest.

The activation input also carries one independently reconstructed corpus-wide inventory of addressable rows rendered in an `effective.md` but absent from its `effective.source.json`. The known pre-transition baseline is exactly two rows—`prd:impl-review/ABUSE-8` and `prd:e2e-verification/ABUSE-5`, both introduced through legacy §5.3 Permissions-and-abuse section supersedes. The live docgate projection-congruence path must lawfully repair both without retroactively rewriting frozen entries, and activation requires the fresh inventory to be empty; a one-row repair, a new source-missing row, or a grandfather entry remaining anywhere keeps the predicate false. The repaired impl-review source map must additionally make ABUSE-8 available to inverse `Serves`/index coverage. There is no reviewer-child publisher, caller publisher, WorkTracker-namespace workaround, proposal-wide verifier, or legacy success fallback after activation.

## 4. Alternatives considered

| Alternative | Why rejected |
|---|---|
| Reviewer process publishes directly | grants the untrusted subject the credential needed to manufacture its own evidence |
| Implement-now publishes the report | transfers FR-6 ownership and forks impl-review's schema/media contract |
| Retry an ambiguous GitHub comment POST | GitHub supplies no reusable idempotency key; retry can duplicate the completed report |
| Find the comment by body, marker, or author | proposal-wide similarity is forgeable and cannot recover an exact server receipt |
| Require a separate bot/user principal | adds onboarding and shared-secret risk; author identity does not replace attempt proof |
| Sign the report with a distributed private key | creates a new key lifecycle on every engineer machine while protected host proof already authenticates the attempt |

## 5. Risks

| ID | Risk | Impact | Mitigation | Verification / evidence |
|---|---|---|---|---|
| R7 | GitHub acknowledgement is lost after comment creation | real artifact remains unverified | one-shot armed fence, never retry unknown write, failed EVT-13 | response-loss and kill-after-write fixture proves ≤1 comment |
| R8 | Adapter state is writable by a child or repository credential | forged receipt/proof/terminal | host-only store and producer key, sandbox path denial, mutation fixture | capability graph plus direct-write attack suite |
| R9 | EVT-13 lands but host terminal does not | cross-corpus drift | durable receipt/result replay; no republish; pending until exact EVT-17 | crash/event-append matrix |
| R10 | Receipt and proof are accidentally aliased | location masquerades as producer evidence | distinct types, grammars, resolvers, and equality rejection | compile/schema and substitution matrix |
| R11 | Strict ambiguity handling creates false negatives | review remains pending after a comment landed | typed infrastructure status and later fresh attempt; never false acceptance | operator-facing recovery drill |
| R12 | New persistence changes routed-assurance semantics | calibration or pass topology regresses | exact blob incorporation and full prior conformance manifest | carried-check digest comparison |
| R13 | Carina review records are accidentally routed through WorkTracker's latest-wins decision namespace | duplicate or overwritten proposal evidence | separate ProposalService record root, closed receipt grammar, exclusive create, and import-boundary check | seeded `appendDecisionRecord` reachability defect plus conformance suite |
| R14 | Either known §5.3 source-missing row remains, or a new one appears | inverse coverage stays incomplete and the congruence repair is falsely declared finished | pin the exact two-row baseline (`impl-review/ABUSE-8`, `e2e-verification/ABUSE-5`), then require an independently reconstructed empty corpus inventory plus repaired impl-review `Serves` visibility before activation | baseline-equality and zero-row generated-source/index assertions in activation manifest |

## 6. Acceptance

| ID | Acceptance criterion | Verify | Required evidence | Traces to |
|---|---|---|---|---|
| A-12 | The routed-assurance blob and every carried Serves/DINV/risk/acceptance mapping are exact; mutation or omission fails activation. | Test + Inspection | exact-incorporation manifest and negative corpus | #DINV-1..#DINV-10 |
| A-13 | Each supported medium preflights current authenticated read/create capability and durable state/event access before reviewer spend; absence or ambiguity launches and writes nothing. | Test + Failure injection | GitHub/Carina capability matrix | `prd:impl-review/UB-20` |
| A-14 | The host coordinator append/exact-reads every carried route admission before child launch and every returned route-outcome/pass/process terminal before publication. Outer execution returns one closed process handoff; every pass/lens child and implement-now worker has no proposal, adapter-store, or evidence-producer capability. Missing/surplus/malformed/mismatched handoffs deterministically reuse the same publication-attempt and idempotency identities under concurrent/restarted replay, write zero medium objects, and produce exactly one failed EVT-13↔EVT-17 pair. | Test + Inspection + Security review + Failure injection | append-order trace, topology/capability graph, handoff matrix, and invalid-handoff concurrency/replay fixture | `prd:impl-review/FR-27` · `prd:impl-review/FR-32` · `prd:impl-review/FR-34` · `prd:impl-review/UB-22` |
| A-15 | GitHub and Carina publish at most one object per publication attempt, return/exact-read the native receipt, and never body/author-search; exact replay returns the same receipt or stays non-accepting. Carina uses the design-owned ProposalService record slice and never WorkTracker's latest-wins decision namespace. | Test + Failure injection + Inspection | both-media conformance including concurrent calls, unknown acknowledgement, exact-record conflict, and forbidden WorkTracker reachability | `prd:impl-review/FR-33` · `prd:impl-review/UB-21` · #DINV-14 |
| A-16 | Kill/restart before write, after write/before receipt persistence, after receipt/before EVT-13, and after EVT-13/before caller acknowledgement converges to one exact artifact/receipt/terminal set or one failed EVT-13 without a second write. | Failure injection + Security review | boundary crash matrix | `prd:impl-review/UB-23` · #DINV-15 |
| A-17 | Every hosted verified/failed publication produces exactly one EVT-13↔EVT-17 pair with equal status/failure class and all seven exact fields; the host adopts the coordinator's exact process ID as its attempt ID, while a host-invented/second process ID, missing, duplicate, orphaned, grammar/value/presence mismatch, receipt/proof overload, publication-attempt alias, and EVT-15-only case is non-evidentiary. | Test + Analysis + Failure injection | process-identity ownership fixture plus full reciprocal-terminal truth table | `prd:impl-review/FR-35` · `prd:impl-review/EVT-13` · `prd:implement-now/FR-49` · `prd:implement-now/EVT-17` |
| A-18 | Same-author and different-author artifacts verify identically only with matching receipt/proof/binding; FR-16 operator dispositions still require native author attribution. | Test + Security review | author-neutral artifact matrix plus unchanged disposition fixtures | `prd:impl-review/FR-37` |
| A-19 | CRLF, foreign/malformed comments, duplicate bodies, pagination, replica lag, replay, receipt substitution, proof substitution, protected-state mutation, and transport failures take their declared exact-receipt or failure paths. | Test + Failure injection | artifact abuse and transport matrix | `prd:impl-review/UB-21` · `prd:impl-review/ABUSE-8` |
| A-20 | One production-inactive activation manifest proves predecessor incorporation, both media, child confinement, bounded recovery, EVT-13/17 join, absence of every legacy publisher/verifier path, exact agreement with the two-row pre-repair source-missing baseline, a zero-row corpus inventory after lawful repair, and generated source-map/index visibility for impl-review ABUSE-8 before the joint executor cutover. A fixture repairing only one of `impl-review/ABUSE-8` and `e2e-verification/ABUSE-5` remains red. | Test + Inspection + Demo | signed activation manifest, reachability report, independent before/after corpus inventories, one-row-repair negative fixture, and generated-provenance assertion | `prd:impl-review/DOD-18` · `prd:impl-review/ABUSE-8` · `prd:e2e-verification/ABUSE-5` |

## 7. Open questions

None. A distinct code-host reviewer principal remains explicitly deferred by the PRD and is not needed for this completion predicate. Adding one later requires a PRD amendment and successor design because it changes onboarding, credential ownership, and verifier behavior.

## Principles

- **AP-1 — honored.** Impl-review owns the persistence port; GitHub and the design-owned Carina ProposalService record slice remain adapters, while WorkTracker is unchanged.
- **AP-3 — honored.** Both proposal media run one persistence conformance suite.
- **AP-5 — honored.** Process, publication attempt, obligation, proposal, revision, artifact, receipt, proof, and both terminals join explicitly.
- **AP-6 — honored.** Handoff, state, result, receipt, and terminal schemas are typed, closed, and versioned.
- **AP-10 — honored.** Idempotency, exact-read, artifact validation, failure class, and terminal join are mechanical predicates.
- **AP-11 — honored.** Impl-review has one publisher and one artifact contract; the host has one reciprocal terminal.
- **AP-13 — honored.** Completion is the exact receipted, proof-bound artifact plus immutable terminal pair, never an exit or comment presence.
- **AP-15 — honored.** Durable fences and replay converge after interruption without duplicate publication.
- **AP-16 — honored.** Publication and reconciliation are closed-policy bounded; exhaustion stays non-accepting.
- **AP-17 — honored.** Missing capability fails before reviewer spend; invalid handoff fails before write; mismatched evidence fails before acceptance.
- **AP-20 — honored.** Reviewer execution, persistence adapter, invoking host, evidence producer, and human disposition authorities remain separate.
- **AP-22 — honored.** The host owns adapter calls, cancellation, durable state, and terminal completion.
- **AP-25 — honored.** GitHub and Carina receipt/idempotency behavior stays inside medium adapters; the Carina adapter exposes no WorkTracker operation.
- **AP-28 — honored.** Canonical EVT-13 and its reciprocal terminal govern; proposal rendering is a checked projection.
- **AP-29 — honored.** Receipt/proof/terminal correctness gates acceptance; latency and retries are measurements.
- **RW-4 — deviated, with operator authorization.** Real proposal adapters ship with hermetic substitutes because review must persist on the proposal; their conformance lands before network-dependent proof.
- **RW-5 — honored.** This successor owns the independently revisable impl-review persistence boundary and carries routed assurance unchanged.
- **Deviation:** RW-4 as stated; none other.
