# Coordinate one inherited Learning repair — plan 001

| Field | Value |
|---|---|
| **Feature** | inherited-learning-repair |
| **PRD** | prd/inherited-learning-repair |
| **Design** | design/inherited-learning-repair |
| **Basis** | lock/inherited-learning-repair/v1 |
| **Plan version** | 001 |
| **Dialect** | harbormaster/fs-spec:v2 |
| **Epic title** | Coordinate one repair for inherited Learning evidence failures |
| **Status** | Draft — freezes at merge |
| **Template version** | plan-template/v2 |

## Tasks

### T1 — Share the existing evidence report and witness predicate

| Field | Value |
|---|---|
| **Blocked by** | — |
| **Size band** | 90–180 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Requirements** | {"schema_version":"task-requirements/v1","implementation":{"assessment":{"reasoning":"complex","coupling":"composed","novelty":"known","verification":"composed","impact_scope":"multi_surface","architectural_role":"shared_substrate","citations":[{"id":"task:acceptance"},{"id":"design:inherited-learning-repair#D-1"}]},"rationale":"Extract the existing report codec and exact witness predicate without moving orchestration into evidence validation or changing verdicts."},"review":{"assessment":{"reasoning":"difficult","coupling":"composed","novelty":"known","verification":"cross_boundary","impact_scope":"multi_surface","architectural_role":"shared_substrate","citations":[{"id":"task:acceptance"},{"id":"prd:inherited-learning-repair/NR-2"}]},"rationale":"The review must prove that extraction preserves every existing evidence refusal and the narrow verified-successor exception."}} |
| **Pattern** | — |
| **Touches** | internal/memory/cli/evidence_verify.go, internal/memory/cli/evidence_successor_test.go, internal/memory/evidence/ |

**Goal:** Orchestration can consume the existing typed evidence report and witness-mismatch predicate without duplicating or weakening them.

**Current behavior:** The evidence report types and witness predicate are private to the memory CLI.

**Desired behavior:** A reusable typed codec and predicate retain the current report schema, exact mismatch requirements, and verified-successor classification; the ordinary verifier consumes the same definitions.

**Scope:**
- Extract the report types, closed decoding boundary, and witness predicate as the prerequisite refactor.
- Preserve report fields, serialization, classification, and existing gate behavior.
- Keep fetching, task creation, and model execution outside the evidence verifier.

**Out of scope:**
- New evidence exceptions, new record kinds, repair orchestration, or a corpus migration.

**Acceptance criteria:**
- [ ] Existing evidence report fixtures remain byte-equivalent and the current verified-successor refusal suite passes.
- [ ] The shared predicate accepts only invalid content or membership mismatches with a witnessed snapshot and rejects empty or mixed unsupported findings.
- [ ] The shared report decoder rejects malformed or unknown fields and exposes the same predecessor and verified successor identities used by the CLI.

**Key interfaces:**
- memory-evidence-report/v1, record identity and digest, evidence entries, witness mismatch, and verified successor fields.

**Traces:** prd:inherited-learning-repair/FR-2 · prd:inherited-learning-repair/NR-1 · prd:inherited-learning-repair/NR-2 · design:inherited-learning-repair#D-1 · design:inherited-learning-repair#D-6

### T2 — Verify a freshly fetched target with its pinned tool

| Field | Value |
|---|---|
| **Blocked by** | T1 |
| **Size band** | 220–300 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Requirements** | {"schema_version":"task-requirements/v1","implementation":{"assessment":{"reasoning":"difficult","coupling":"cross_boundary","novelty":"adapt","verification":"cross_boundary","impact_scope":"multi_surface","architectural_role":"feature_local","citations":[{"id":"task:acceptance"},{"id":"design:inherited-learning-repair#D-2"}]},"rationale":"Compose isolated Git history, target-owned source or release pins, strict tool resolution, and typed verification without touching caller state."},"review":{"assessment":{"reasoning":"difficult","coupling":"cross_boundary","novelty":"adapt","verification":"cross_boundary","impact_scope":"multi_surface","architectural_role":"feature_local","citations":[{"id":"task:acceptance"},{"id":"prd:inherited-learning-repair/UB-2"}]},"rationale":"A stale cache, ambient tool fallback, or mixed target snapshot could authorize an incorrect repair or false adoption result."}} |
| **Pattern** | — |
| **Touches** | internal/memoryrepair/target.go, internal/memoryrepair/target_test.go, internal/adapters/codehost/git/repair_target.go, internal/adapters/codehost/git/repair_target_test.go |

**Goal:** A caller obtains one current target snapshot and its compatible evidence verdict without altering its working checkout.

**Current behavior:** Local memory checks use the caller's available checkout and verifier; there is no repair-specific fresh-target inspection boundary.

**Desired behavior:** A context-aware inspection port returns the freshly observed target commit, canonical repository identity, retained evidence, and verifier identity from isolated storage, or an explicit unavailable-prerequisite result.

**Scope:**
- Fetch the configured target and required retained history into isolated storage, with cleanup owned by the invocation.
- Run the target's source-mode or verified release-mode tool through strict existing resolution and parse its typed evidence report.
- Bind all inspection data to one target commit and verifier identity; require a fresh remote observation even when reusable proof exists.

**Out of scope:**
- Another installer or compatibility service, shared repair mutations, and automatic updates to caller branches.

**Acceptance criteria:**
- [ ] A stale local remote-tracking ref and older caller tool still produce the current remote target verdict through the target-selected verifier.
- [ ] Fetch or retained-history failure returns freshness_unknown; unavailable or incompatible pinned tooling returns verifier_unavailable; neither path invokes a tracker or executor.
- [ ] Source-mode and release-mode fixtures reject ambient-tool fallback and report the exact target and verifier used.
- [ ] Success, failure, and cancellation preserve caller HEAD, index, tracked modifications, and untracked files and clean up owned temporary state.

**Key interfaces:**
- Context-aware target inspection, canonical repository binding, strict tool resolution, target commit, verifier identity, and typed evidence report.

**Traces:** prd:inherited-learning-repair/FR-1 · prd:inherited-learning-repair/UB-2 · prd:inherited-learning-repair/NR-3 · design:inherited-learning-repair#D-2

### T3 — Admit only the verified inherited Learning incident

| Field | Value |
|---|---|
| **Blocked by** | T2 |
| **Size band** | 190–280 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Requirements** | {"schema_version":"task-requirements/v1","implementation":{"assessment":{"reasoning":"difficult","coupling":"composed","novelty":"adapt","verification":"composed","impact_scope":"contained","architectural_role":"feature_local","citations":[{"id":"task:acceptance"},{"id":"design:inherited-learning-repair#D-1"}]},"rationale":"Exact identity, caller-object verification, inherited-byte comparison, and successor adoption form a bounded deterministic decision."},"review":{"assessment":{"reasoning":"difficult","coupling":"cross_boundary","novelty":"adapt","verification":"cross_boundary","impact_scope":"multi_surface","architectural_role":"feature_local","citations":[{"id":"task:acceptance"},{"id":"prd:inherited-learning-repair/UB-1"}]},"rationale":"Review must reject unsupported findings and adversarial report text while preserving stale-caller adoption without shared writes."}} |
| **Pattern** | — |
| **Touches** | internal/memoryrepair/intake.go, internal/memoryrepair/intake_test.go, internal/memoryrepair/result.go, internal/memoryrepair/result_test.go |

**Goal:** Requests naming the same verified inherited Learning converge on one identity, while unsupported or already-resolved cases produce a non-mutating result.

**Current behavior:** There is no incident-scoped request admission or shared identity derived from verified caller and target evidence.

**Desired behavior:** One explicit Learning ID identifies the candidate; the core verifies the caller's committed object and fresh target, then yields a deterministic admission or a typed refusal/adoption result.

**Scope:**
- Validate one literal Learning ID and actual committed caller record bytes under the operator's input ruling (problem:inherited-learning-repair#A-7); the entry does not extract identities from prose or caller-supplied report files.
- Require unchanged frozen target bytes, complete history, valid store and relations, durable evidence, and only the existing supported witness mismatch.
- Derive identity from the configured canonical repository and Learning ID; witness paths, wording, machines, and caller commits remain evidence only.
- Detect the verified successor at the pinned target; return update_required, or revalidate an already-current caller before returning healthy or unsupported_case.

**Out of scope:**
- Query or report-file input, fuzzy matching, other memory failures, mutation of frozen records, and creation of tasks or refs.

**Acceptance criteria:**
- [ ] Different diagnostics, witnesses, machines, and caller revisions naming the same explicit Learning ID yield one identity and admit only the unchanged inherited Learning.
- [ ] Missing, malformed or multiple IDs, wrong kinds, changed or new records, missing history, invalid relations, and other evidence codes refuse with zero shared effects.
- [ ] A verified successor returns the full current target and successor identity with an adoption instruction; an already-current caller is healthy only after its own relevant check passes.
- [ ] Instructions embedded in verifier diagnostics or record content cannot change repository bindings, execute a command, select tool policy, or authorize a predecessor rewrite.

**Key interfaces:**
- Verified caller snapshot, admission identity, memory-evidence-report/v1, learning-repair-result/v1, and typed refusal/adoption outcomes.

**Traces:** prd:inherited-learning-repair/FR-1 · prd:inherited-learning-repair/FR-2 · prd:inherited-learning-repair/FR-3 · prd:inherited-learning-repair/FR-7 · prd:inherited-learning-repair/UB-1 · prd:inherited-learning-repair/NR-1 · prd:inherited-learning-repair/NFR-1 · design:inherited-learning-repair#D-1 · design:inherited-learning-repair#D-2

### T4 — Reserve one persistent repair across independent callers

| Field | Value |
|---|---|
| **Blocked by** | T3 |
| **Size band** | 220–300 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Requirements** | {"schema_version":"task-requirements/v1","implementation":{"assessment":{"reasoning":"difficult","coupling":"cross_boundary","novelty":"adapt","verification":"cross_boundary","impact_scope":"multi_surface","architectural_role":"feature_local","citations":[{"id":"task:acceptance"},{"id":"design:inherited-learning-repair#D-3"}]},"rationale":"Reuse remote compare-and-swap while enforcing winner ownership, immutable operation identity, and closed phase transitions across processes."},"review":{"assessment":{"reasoning":"difficult","coupling":"cross_boundary","novelty":"adapt","verification":"cross_boundary","impact_scope":"multi_surface","architectural_role":"feature_local","citations":[{"id":"task:acceptance"},{"id":"prd:inherited-learning-repair/NFR-2"}]},"rationale":"The critical failure is two live owners or a retry that repeats an external effect; review requires independent-process contention and corrupt-state cases."}} |
| **Pattern** | — |
| **Touches** | internal/memoryrepair/store.go, internal/memoryrepair/store_test.go, internal/adapters/codehost/git/learning_repair.go, internal/adapters/codehost/git/learning_repair_test.go |

**Goal:** Independent callers reserve or join the same durable operation before any tracker or executor effect.

**Current behavior:** The reusable Git-ref store supplies compare-and-swap, but there is no Learning repair record or ownership protocol.

**Desired behavior:** RepairStore atomically creates the incident's fixed ref and validates the closed learning-repair/v1 state on every read and owner-authorized transition; losing callers observe the winner.

**Scope:**
- Implement the exact incident ref namespace and versioned record with repository, Learning digest, target, verifier, owner token, phase, and optional canonical references.
- Reuse Git-ref compare-and-swap for create-if-absent and conditional updates; re-read a winning generation after contention.
- Validate identity, owner, and allowed transitions, retaining terminal and uncertain operations indefinitely.
- Provide read-only joining and preserve pre-effect phases when execution stops; expose no lease expiry, takeover, or retry policy.

**Out of scope:**
- Tracker creation, executor dispatch, crash recovery, and trusting task-search similarity for ownership.

**Acceptance criteria:**
- [ ] Five independent processes using separate checkouts and one bare remote produce one ref and one owner; every loser reads that same operation.
- [ ] Foreign repository or Learning identity, digest mismatch, unknown JSON fields, malformed records, wrong owner, and illegal transitions refuse without mutation.
- [ ] A stale compare-and-swap cannot overwrite the winner, and an unsupported atomic-ref capability refuses rather than using a local mutex for correctness.
- [ ] Failed, uncertain, terminal, and indefinitely reserved operations remain observable and cannot cause another reservation or owner takeover.

**Key interfaces:**
- RepairStore, learning-repair/v1, create-if-absent, observed-head compare-and-swap, owner token, and the contract's phase enum.

**Traces:** prd:inherited-learning-repair/FR-3 · prd:inherited-learning-repair/FR-6 · prd:inherited-learning-repair/UB-3 · prd:inherited-learning-repair/UB-6 · prd:inherited-learning-repair/NFR-2 · design:inherited-learning-repair#D-3

### T5 — Create the canonical tracker task once

| Field | Value |
|---|---|
| **Blocked by** | T4 |
| **Size band** | 220–300 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Requirements** | {"schema_version":"task-requirements/v1","implementation":{"assessment":{"reasoning":"complex","coupling":"cross_boundary","novelty":"adapt","verification":"cross_boundary","impact_scope":"multi_surface","architectural_role":"feature_local","citations":[{"id":"task:acceptance"},{"id":"design:inherited-learning-repair#D-3"}]},"rationale":"Compose configured existing tracker creation adapters with the reserved operation, a last fresh-target check, and one self-contained task."},"review":{"assessment":{"reasoning":"difficult","coupling":"cross_boundary","novelty":"adapt","verification":"cross_boundary","impact_scope":"multi_surface","architectural_role":"feature_local","citations":[{"id":"task:acceptance"},{"id":"prd:inherited-learning-repair/UB-3"}]},"rationale":"Review must prove no duplicate creation after ambiguity and no discovery label that launches a second worker outside direct admission."}} |
| **Pattern** | — |
| **Touches** | internal/memoryrepair/task.go, internal/memoryrepair/task_test.go, internal/adapters/tracker/github/learning_repair.go, internal/adapters/tracker/forgejo/learning_repair.go, internal/adapters/tracker/carina/learning_repair.go, internal/adapters/tracker/repairconformance/ |

**Goal:** The live owner creates one understandable task through the configured tracker, and every joining caller receives its canonical reference.

**Current behavior:** Tracker adapters can create work items, but creation is not guarded by a shared Learning repair reservation.

**Desired behavior:** After a fresh post-reservation predicate check, the owner persists creating_task before one create request and records task_ready with the exact returned source reference; failures remain on that operation.

**Scope:**
- Introduce the consumer-owned context-aware TaskCreator and adapt the existing GitHub, Forgejo, and Carina creation capabilities.
- Render identity, target evidence, intended lesson, bounded correction scope, acceptance, and an operation marker into the canonical task.
- Recheck the target immediately before creation; a concurrently landed fix produces the no-work/adoption result without creating a task.
- Distinguish definite failure from outcome uncertainty; never retry creation from a subsequent caller or add generic fleet-discovery routing.

**Out of scope:**
- New tracker implementations, task-description similarity, dispatch, and uncertain-effect recovery.

**Acceptance criteria:**
- [ ] Adapter conformance for all configured tracker families shows one creation after creating_task, exact task readback, required evidence/acceptance, and zero creations from joiners.
- [ ] A fix landing after reservation but before creation yields adoption instructions and zero task creation.
- [ ] A definite create error records failed; an ambiguous result records outcome_unknown when persistence is available, otherwise retains creating_task; later callers never retry.
- [ ] Provider credentials and local user paths stay out of persisted state and task text, and labels cannot start an independent fleet attempt.

**Key interfaces:**
- TaskCreator, configured tracker binding, canonical task source reference, operation marker, and creating_task/task_ready transitions.

**Traces:** prd:inherited-learning-repair/FR-4 · prd:inherited-learning-repair/FR-6 · prd:inherited-learning-repair/FR-7 · prd:inherited-learning-repair/UB-3 · prd:inherited-learning-repair/NR-3 · design:inherited-learning-repair#D-2 · design:inherited-learning-repair#D-3 · design:inherited-learning-repair#D-4

### T6 — Dispatch one bounded successor correction through implement-now

| Field | Value |
|---|---|
| **Blocked by** | T5 |
| **Size band** | 240–300 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Requirements** | {"schema_version":"task-requirements/v1","implementation":{"assessment":{"reasoning":"difficult","coupling":"cross_boundary","novelty":"adapt","verification":"cross_boundary","impact_scope":"multi_surface","architectural_role":"feature_local","citations":[{"id":"task:acceptance"},{"id":"design:inherited-learning-repair#D-4"}]},"rationale":"Bind an exact tracker snapshot to a frozen native charter, configured role requirements, isolated execution, and one owner-controlled launch."},"review":{"assessment":{"reasoning":"difficult","coupling":"cross_boundary","novelty":"adapt","verification":"cross_boundary","impact_scope":"multi_surface","architectural_role":"shared_substrate","citations":[{"id":"task:acceptance"},{"id":"prd:inherited-learning-repair/NR-2"}]},"rationale":"The executor boundary must preserve independent review and merge authority while preventing prompt injection, duplicate launch, and recursive repair."}} |
| **Pattern** | — |
| **Touches** | internal/memoryrepair/dispatch.go, internal/memoryrepair/dispatch_test.go, internal/memoryrepair/charter.go, internal/memoryrepair/charter_test.go, cmd/memory/repair_executor.go, internal/executor/issueentry/repair_context.go, cmd/implement-now/repair_context.go |

**Goal:** The canonical task reaches the existing direct-issue executor exactly once under a frozen successor-repair charter.

**Current behavior:** Direct-issue admission and per-task claims exist, but no repair owner creates their incident-specific input or records their outcome.

**Desired behavior:** The owner inspects the exact task fingerprint, builds issue-task-input/v1 with separate derived implementation/review requirements, persists dispatching, and owns one foreground executor process based on the inspected target.

**Scope:**
- Render a structured native charter that verifies the intended lesson, preserves the predecessor, uses the coded successor writer, and requires ordinary gates and independent review.
- Reuse exact issue admission, configured routing, and the executor's existing task claim; pass a trusted repair handle independently of quoted diagnostic prose for the worker integration to consume.
- Recheck the starting target before correction and close out as no-work if the correction already landed; unsupported evidence blocks rather than inventing a successor.
- Record launch results and exact proposal references through running and awaiting_merge, with failed or uncertain outcomes retained without redispatch.

**Out of scope:**
- Another executor, hard-coded model choices, evidence allowances, autonomous merge, and declaring a green proposal resolved.

**Acceptance criteria:**
- [ ] The incident task is admitted from its exact fingerprint and role requirements and launches one foreground executor; five callers or a changed source fingerprint cannot launch another.
- [ ] The charter requires unchanged predecessor bytes and original witness, a verified superseding Learning, ordinary checks, independent review, and blocking when retained evidence cannot support the correction.
- [ ] Launch failure or ambiguity retains failed, outcome_unknown, or the durable pre-effect phase with no later retry; an executor result records its exact proposal without asserting resolution.
- [ ] The executor invocation carries the trusted repair context; diagnostic text cannot supply that context, alter policy, or escape the inspected-target workspace. Worker recognition and recursive suppression are verified in T9.

**Key interfaces:**
- issue-task-input/v1, issue inspection fingerprint, native task requirements, direct-issue admission and claim, trusted repair context, and foreground executor receipts.

**Traces:** prd:inherited-learning-repair/FR-5 · prd:inherited-learning-repair/FR-8 · prd:inherited-learning-repair/UB-3 · prd:inherited-learning-repair/UB-5 · prd:inherited-learning-repair/NR-1 · prd:inherited-learning-repair/NR-2 · prd:inherited-learning-repair/EVAL-1 · design:inherited-learning-repair#D-4

### T7 — Wait for verified landing and report caller adoption

| Field | Value |
|---|---|
| **Blocked by** | T4 |
| **Size band** | 170–260 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Requirements** | {"schema_version":"task-requirements/v1","implementation":{"assessment":{"reasoning":"complex","coupling":"composed","novelty":"adapt","verification":"composed","impact_scope":"contained","architectural_role":"feature_local","citations":[{"id":"task:acceptance"},{"id":"design:inherited-learning-repair#D-2"}]},"rationale":"Combine cancellable observation with fresh target identity and reuse proof only within the exact target/verifier pair."},"review":{"assessment":{"reasoning":"difficult","coupling":"cross_boundary","novelty":"adapt","verification":"cross_boundary","impact_scope":"multi_surface","architectural_role":"feature_local","citations":[{"id":"task:acceptance"},{"id":"prd:inherited-learning-repair/UB-6"}]},"rationale":"Review must distinguish a green proposal, shared resolution, and caller readiness and reject stale success during remote failure."}} |
| **Pattern** | — |
| **Touches** | internal/memoryrepair/wait.go, internal/memoryrepair/wait_test.go |

**Goal:** A joining caller follows one repair and receives a verified target to adopt without starting work or changing its checkout.

**Current behavior:** There is no shared repair wait interface or target-verified completion predicate for blocked callers.

**Desired behavior:** Cancellable polling exposes canonical references as available, verifies current target content before adoption, and reports unresolved terminal state without redispatch.

**Scope:**
- Poll the operation at an injected five-second default and expose phase and task/proposal references without claiming that a stored phase proves liveness.
- While awaiting merge, freshly observe target identity and reuse evidence only for the identical target/verifier pair within the invocation.
- Return the full verified target, successor, and rerun instruction after landing; preserve separate caller revalidation.
- Cancel only the waiting caller and retain terminal, failed, uncertain, or stalled operations without takeover.

**Out of scope:**
- Modifying caller branches, automatic retries, owner cancellation by a waiter, and treating host status as resolution.

**Acceptance criteria:**
- [ ] Open or green proposals and closed tasks never resolve a repair; a fetched target with the verified successor returns update_required and a rerun instruction.
- [ ] Unchanged target/verifier pairs reuse verification after a fresh remote read; changed targets reverify, and remote-read failure returns freshness_unknown instead of cached success.
- [ ] Cancelled wait returns its handle promptly, leaves the operation intact, and permits another waiter to continue without extra tasks or launches.
- [ ] A terminal operation whose target still fails returns unresolved with the same task; failed, uncertain, and stalled records never trigger another attempt.

**Key interfaces:**
- Cancellable waiter, injected poll cadence, fresh target identity, exact-target verification cache, repair handle, and adoption result.

**Traces:** prd:inherited-learning-repair/FR-6 · prd:inherited-learning-repair/FR-7 · prd:inherited-learning-repair/FR-8 · prd:inherited-learning-repair/UB-2 · prd:inherited-learning-repair/UB-4 · prd:inherited-learning-repair/UB-6 · prd:inherited-learning-repair/NR-3 · design:inherited-learning-repair#D-2 · design:inherited-learning-repair#D-3 · design:inherited-learning-repair#D-5

### T8 — Expose explicit repair and bounded scan through the memory CLI

| Field | Value |
|---|---|
| **Blocked by** | T6, T7 |
| **Size band** | 220–300 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Requirements** | {"schema_version":"task-requirements/v1","implementation":{"assessment":{"reasoning":"complex","coupling":"cross_boundary","novelty":"adapt","verification":"composed","impact_scope":"multi_surface","architectural_role":"feature_local","citations":[{"id":"task:acceptance"},{"id":"design:inherited-learning-repair#D-5"}]},"rationale":"Wire the bounded orchestration into the existing command and configured adapters, including scan composition and machine-readable results."},"review":{"assessment":{"reasoning":"difficult","coupling":"cross_boundary","novelty":"adapt","verification":"cross_boundary","impact_scope":"multi_surface","architectural_role":"feature_local","citations":[{"id":"task:acceptance"},{"id":"prd:inherited-learning-repair/FR-9"}]},"rationale":"Review must prove explicit, installed, and scan entry points share admission and never silently select a provider, retry an effect, or wait for human merge during a scan."}} |
| **Pattern** | — |
| **Touches** | cmd/memory/main.go, cmd/memory/repair.go, internal/memory/cli/cli.go, internal/memory/cli/repair.go, internal/memory/cli/repair_test.go, internal/memoryrepair/scan.go, internal/memoryrepair/scan_test.go |

**Goal:** Agents and existing daily triggers invoke the same bounded repair behavior through the advertised memory command.

**Current behavior:** The memory CLI exposes evidence checks but no explicit repair, shared wait, or incident scan.

**Desired behavior:** Under the operator's input ruling (problem:inherited-learning-repair#A-7), the command composes configured ports, accepts either one `--record Learning-<ULID>` or `--scan`, supports wait and JSON results, and reports supported no-work and refusal outcomes without another agent loop.

**Scope:**
- Wire the existing memory dispatch and source/installed tool flow to the repair core, keeping adapter imports in composition roots.
- Expose one explicit Learning ID or a mutually exclusive scan, wait, help, and learning-repair-result/v1 output with the contract's outcome and reference fields.
- Scan one pinned target for supported unresolved Learnings, then use the same fresh admission path for each and await only its own foreground dispatch completion.
- Emit EVT-1 through structured logging without raw diagnostics, credentials, or user paths.

**Out of scope:**
- Query or report-file input, a scheduler, automatic schedule registration, broader diagnosis, and automatic worker delegation.

**Acceptance criteria:**
- [ ] Built CLI and installed exec-flow fixtures cover admission, join, update_required, healthy, refusals, cancellation, failed, uncertain, and unresolved outcomes with valid versioned JSON and non-JSON help.
- [ ] Empty scan makes zero tracker or executor calls; a scan racing agent intake joins the same operation and does not wait for human merge.
- [ ] Missing or conflicting record/scan operands, malformed or multiple IDs, removed `--query`/`--report` flags, other unknown flags, and unavailable configured adapters refuse before shared effects.
- [ ] The CLI uses the configured host, tracker, and executor, emits bounded EVT-1 fields, and performs no inference before the single admitted dispatch.

**Key interfaces:**
- memory repair operand grammar, scan and wait modes, CLI dependency injection, learning-repair-result/v1, and EVT-1.

**Traces:** prd:inherited-learning-repair/FR-4 · prd:inherited-learning-repair/FR-5 · prd:inherited-learning-repair/FR-6 · prd:inherited-learning-repair/FR-9 · prd:inherited-learning-repair/UB-1 · prd:inherited-learning-repair/UB-4 · prd:inherited-learning-repair/NFR-1 · prd:inherited-learning-repair/NFR-3 · prd:inherited-learning-repair/EVT-1 · design:inherited-learning-repair#D-1 · design:inherited-learning-repair#D-3 · design:inherited-learning-repair#D-5

### T9 — Delegate the worker's known blocker without a local repair loop

| Field | Value |
|---|---|
| **Blocked by** | T8 |
| **Size band** | 220–300 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Requirements** | {"schema_version":"task-requirements/v1","implementation":{"assessment":{"reasoning":"difficult","coupling":"cross_boundary","novelty":"adapt","verification":"cross_boundary","impact_scope":"system_wide","architectural_role":"system_core","citations":[{"id":"task:acceptance"},{"id":"design:inherited-learning-repair#D-5"}]},"rationale":"A narrowly guarded branch changes the executor's local and hosted CI recovery path, so preserving the existing factory loop requires system-level assessment."},"review":{"assessment":{"reasoning":"difficult","coupling":"cross_boundary","novelty":"adapt","verification":"cross_boundary","impact_scope":"system_wide","architectural_role":"system_core","citations":[{"id":"task:acceptance"},{"id":"prd:inherited-learning-repair/FR-10"}]},"rationale":"Review must establish that unrelated failures retain recovery, supported failures cannot launch independent memory edits, and repair tasks cannot recurse or bypass a red gate."}} |
| **Pattern** | — |
| **Touches** | internal/executor/drivetogreen/local.go, internal/executor/drivetogreen/heal.go, internal/executor/drivetogreen/memory_repair.go, internal/executor/drivetogreen/memory_repair_test.go, internal/executor/worker/, internal/executor/config/, internal/configlayer/, cmd/implement-now/memory_repair.go |

**Goal:** An opted-in worker blocked by this inherited Learning delegates to the shared operation instead of independently editing memory.

**Current behavior:** Local and hosted CI recovery can launch independent fixers for a memory-gate failure.

**Desired behavior:** A closed optional memory.inherited_learning_repair.enabled boolean defaults false; when enabled, structured incident diagnostics reach shared admission, admitted/joining workers suspend their memory-fixing loop, and adoption returns a typed task blocker.

**Scope:**
- Add the closed opt-in setting without enabling it in repository defaults or changing unrelated local configuration.
- Integrate structured evidence at both local-gate and hosted-CI recovery boundaries, retaining the original caller revision and configured repository.
- Wait on admitted or joined operations and return the verified target/rerun instruction through normal task integration without mutating the original branch or marking its gate green.
- Suppress recursive repair under trusted repair-task context; retain ordinary handling for unsupported failure classes.

**Out of scope:**
- General CI triage changes, automatic branch integration, scheduler enrollment, and activation before the incident and model-backed proofs.

**Acceptance criteria:**
- [ ] With routing disabled, existing local and hosted recovery fixtures retain their behavior; malformed or unknown configuration values refuse.
- [ ] With routing enabled, the supported local and hosted memory failures call shared admission and produce zero independent memory-fixer edits while following the winning operation.
- [ ] update_required preserves the target revision, verified successor, and rerun instruction through the local or hosted driver, worker outcome, and caller-visible result; caller work remains unchanged and its failed check is not silently marked successful.
- [ ] Unsupported checker failures use the ordinary path, while a repair worker's own supported blocker remains on its task with no recursive launch.

**Key interfaces:**
- Closed memory.inherited_learning_repair.enabled setting, structured gate evidence, shared repair delegation, trusted repair context, and typed caller-adoption blocker.

**Traces:** prd:inherited-learning-repair/FR-10 · prd:inherited-learning-repair/FR-6 · prd:inherited-learning-repair/UB-1 · prd:inherited-learning-repair/UB-5 · prd:inherited-learning-repair/NR-2 · prd:inherited-learning-repair/NR-3 · design:inherited-learning-repair#D-4 · design:inherited-learning-repair#D-5

### T10 — Prove five callers, concurrent scan, and stale adoption

| Field | Value |
|---|---|
| **Blocked by** | T9 |
| **Size band** | 40–100 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Requirements** | {"schema_version":"task-requirements/v1","implementation":{"assessment":{"reasoning":"difficult","coupling":"cross_boundary","novelty":"adapt","verification":"cross_boundary","impact_scope":"multi_surface","architectural_role":"feature_local","citations":[{"id":"task:acceptance"},{"id":"design:inherited-learning-repair#D-6"}]},"rationale":"Construct a hermetic retained-history incident and independent-process effect receipts that exercise the real CLI and worker boundary."},"review":{"assessment":{"reasoning":"difficult","coupling":"cross_boundary","novelty":"adapt","verification":"cross_boundary","impact_scope":"multi_surface","architectural_role":"shared_substrate","citations":[{"id":"task:acceptance"},{"id":"prd:inherited-learning-repair/DOD-5"}]},"rationale":"The acceptance proof must detect process-local coordination, stale-tool false success, predecessor rewriting, and evidence-gate bypass rather than mirror the implementation."}} |
| **Pattern** | — |
| **Touches** | internal/memoryrepair/incident_test.go, internal/memoryrepair/process_test.go, internal/memoryrepair/testdata/, cmd/memory/repair_flow_test.go, internal/executor/drivetogreen/memory_repair_flow_test.go |

**Goal:** Reproduce the reported multi-agent failure and demonstrate one repair plus correct adoption across independent checkouts.

**Current behavior:** Individual seams can pass unit tests without proving the incident across processes, shared remote state, and stale caller tools.

**Desired behavior:** A retained-history fixture reproduces the unchanged inherited witness mismatch, observes one task and launch across five CLI processes and a scan, then proves fresh-target adoption after a valid successor lands.

**Scope:**
- Build a hermetic incident history using the existing evidence helpers, local bare remote, and shared recording tracker/executor adapters.
- Run five independent processes naming the same Learning ID from checkouts with different diagnostics, witnesses, and caller revisions while a scan competes through the same admission path.
- Land a valid successor in the fixture and exercise a stale checkout, stale tracking ref, older caller verifier, and missing/incompatible verifier variants.
- Exercise failure, ambiguity, cancellation, unresolved terminal state, injection, disabled routing, and recursive suppression while retaining exact preservation assertions.

**Out of scope:**
- Live network in unit tests, a generalized memory corpus, model-backed success claims, and implementation of recovery or takeover.

**Acceptance criteria:**
- [ ] Five independent callers plus concurrent scan share exactly one task and one executor invocation, and reporting workers make zero independent memory repair edits; the fixture passes with the configured Go test command without race instrumentation, as directed by the operator's replacement repository instructions (problem:inherited-learning-repair#A-7).
- [ ] After fixture landing, a stale caller receives update_required for the current verified target and successor with zero new tasks or dispatches; unavailable verification never reports fixed.
- [ ] Fault and abuse fixtures preserve operation identity, caller HEAD, index and files, predecessor bytes and failed witness, and ordinary gate refusal without retries, recursive work, or allowance changes.
- [ ] The advertised built and installed flows, adapter conformance, existing successor refusal suite, vet/lint, and fresh transport-clone validation pass at the reviewed revision with deterministic effect receipts.

**Key interfaces:**
- Retained-history incident fixture, independent process harness, shared effect receipts, installed command flow, and revision-bound verification evidence.

**Traces:** prd:inherited-learning-repair/FR-3 · prd:inherited-learning-repair/FR-7 · prd:inherited-learning-repair/FR-9 · prd:inherited-learning-repair/FR-10 · prd:inherited-learning-repair/NR-1 · prd:inherited-learning-repair/NR-2 · prd:inherited-learning-repair/NR-3 · prd:inherited-learning-repair/NFR-1 · prd:inherited-learning-repair/NFR-2 · prd:inherited-learning-repair/NFR-3 · prd:inherited-learning-repair/DOD-1 · prd:inherited-learning-repair/DOD-2 · prd:inherited-learning-repair/DOD-3 · prd:inherited-learning-repair/DOD-4 · prd:inherited-learning-repair/DOD-5 · design:inherited-learning-repair#D-6

### T11 — Retain a bounded worker trial and document opt-in operation

| Field | Value |
|---|---|
| **Blocked by** | T10 |
| **Size band** | 30–90 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Requirements** | {"schema_version":"task-requirements/v1","implementation":{"assessment":{"reasoning":"complex","coupling":"cross_boundary","novelty":"adapt","verification":"cross_boundary","impact_scope":"multi_surface","architectural_role":"feature_local","citations":[{"id":"task:acceptance"},{"id":"prd:inherited-learning-repair/EVAL-1"}]},"rationale":"Exercise the existing configured model-backed path once in an isolated incident trial and retain real candidate, review, and verifier evidence."},"review":{"assessment":{"reasoning":"difficult","coupling":"cross_boundary","novelty":"adapt","verification":"cross_boundary","impact_scope":"multi_surface","architectural_role":"feature_local","citations":[{"id":"task:acceptance"},{"id":"prd:inherited-learning-repair/DOD-6"}]},"rationale":"Review must distinguish model-backed correction evidence from fixture success and ensure the opt-in guidance grants no new recovery or merge authority."}} |
| **Pattern** | — |
| **Touches** | internal/memoryrepair/eval/, docs/evaluations/inherited-learning-repair/, docs/wiki/inherited-learning-repair.md, internal/memory/cli/repair_help.go |

**Goal:** Demonstrate that the bounded charter produces a valid successor through the configured executor and make safe explicit operation understandable.

**Current behavior:** Hermetic effect receipts do not prove a model-backed worker can make the correction, and users lack supported-case and adoption guidance.

**Desired behavior:** One isolated retained-history incident trial uses the configured direct-issue executor and retains the exact candidate, independent review, and deterministic evidence; help explains explicit repair, wait, bounded daily invocation, opt-in routing, and deferred recovery.

**Scope:**
- Run one model-backed incident trial through implement-now within existing configured execution and repair budgets, using isolated fixture history and the bounded charter.
- Retain sanitized revision-specific candidate, executor, independent review, and evidence-verifier receipts at the owning evaluation/proposal.
- Document explicit `--record Learning-<ULID>` repair and `--scan` for the known incident, caller adoption and tool revalidation, the opt-in setting, and disabled routing.
- Keep automatic routing and scheduling disabled in shared defaults; activation requires the blocking incident and worker-trial evidence, not an inferred fixture-only success.

**Out of scope:**
- New deployment hosts or schedules, broader repair cases, automatic merge, and abandoned-operation recovery.

**Acceptance criteria:**
- [ ] One configured model-backed trial yields an independently accepted successor candidate whose deterministic report retains the unchanged predecessor and original failed witness as historical-unresolved with a verified successor.
- [ ] The retained trial identifies actual tool/route and candidate revision, ordinary verification and review outcomes, one dispatch, and zero unrelated edits, evidence bypasses, recursive dispatches, or automatic merges.
- [ ] Published help demonstrates explicit request, shared wait, update_required with revalidation, bounded daily scan, and indefinitely stalled/uncertain operation limitations without claiming automatic recovery.
- [ ] Disabled-routing rehearsal prevents automatic submissions while explicit invocation and existing operation observation still work; shared defaults and unrelated scheduling remain unchanged.

**Key interfaces:**
- Configured implement-now trial, EVAL-1 receipts, independent implementation review, evidence verification, explicit command help, and opt-in/disabled routing.

**Traces:** prd:inherited-learning-repair/FR-5 · prd:inherited-learning-repair/FR-9 · prd:inherited-learning-repair/FR-10 · prd:inherited-learning-repair/NR-1 · prd:inherited-learning-repair/NR-2 · prd:inherited-learning-repair/EVAL-1 · prd:inherited-learning-repair/DOD-6 · design:inherited-learning-repair#D-4 · design:inherited-learning-repair#D-5 · design:inherited-learning-repair#D-6

## Coverage

| Requirement | Tasks |
|---|---|
| prd:inherited-learning-repair/FR-1 | T2, T3 |
| prd:inherited-learning-repair/FR-2 | T1, T3 |
| prd:inherited-learning-repair/FR-3 | T3, T4, T10 |
| prd:inherited-learning-repair/FR-4 | T5, T8 |
| prd:inherited-learning-repair/FR-5 | T6, T8, T11 |
| prd:inherited-learning-repair/FR-6 | T4, T5, T7, T8, T9 |
| prd:inherited-learning-repair/FR-7 | T3, T5, T7, T10 |
| prd:inherited-learning-repair/FR-8 | T6, T7 |
| prd:inherited-learning-repair/FR-9 | T8, T10, T11 |
| prd:inherited-learning-repair/FR-10 | T9, T10, T11 |
| prd:inherited-learning-repair/UB-1 | T3, T8, T9 |
| prd:inherited-learning-repair/UB-2 | T2, T7 |
| prd:inherited-learning-repair/UB-3 | T4, T5, T6 |
| prd:inherited-learning-repair/UB-4 | T7, T8 |
| prd:inherited-learning-repair/UB-5 | T6, T9 |
| prd:inherited-learning-repair/UB-6 | T4, T7 |
| prd:inherited-learning-repair/NR-1 | T1, T3, T6, T10, T11 |
| prd:inherited-learning-repair/NR-2 | T1, T6, T9, T10, T11 |
| prd:inherited-learning-repair/NR-3 | T2, T5, T7, T9, T10 |
| prd:inherited-learning-repair/NFR-1 | T3, T8, T10 |
| prd:inherited-learning-repair/NFR-2 | T4, T10 |
| prd:inherited-learning-repair/NFR-3 | T8, T10 |
| prd:inherited-learning-repair/EVT-1 | T8 |
| prd:inherited-learning-repair/DOD-1 | T10 |
| prd:inherited-learning-repair/DOD-2 | T10 |
| prd:inherited-learning-repair/DOD-3 | T10 |
| prd:inherited-learning-repair/DOD-4 | T10 |
| prd:inherited-learning-repair/DOD-5 | T10 |
| prd:inherited-learning-repair/DOD-6 | T11 |
| prd:inherited-learning-repair/EVAL-1 | T6, T11 |
