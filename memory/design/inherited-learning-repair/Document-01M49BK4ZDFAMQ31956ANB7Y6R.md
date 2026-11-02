# Inherited Learning repair — tech spec

This draft implements only the incident boundary in the sibling PRD. Engineering choices are authored by Argus under authority-contract/v1 tier-2, following problem:inherited-learning-repair#A-1 through A-5 and delivery:inherited-learning-repair#E-3. No implementation or contract approval is claimed.

## Principles

AP-5/AP-6: versioned boundary data and observable outcomes. AP-17: refuse unverifiable identity or freshness. AP-22: shared ownership across callers. AP-25: one vendor-neutral command path. AP-27: configured adapters and consumer-owned interfaces. Preserve the exact, local evidence semantics of design:memory-durable-evidence-v1; network and model work are outside that verifier. No governing principle is changed.

### D-1 — Admit one verified inherited Learning identity

**Decision:** Add `memory repair` as a Go orchestration verb. Accept one `--record Learning-<ULID>`, one `--report <JSON-file>` containing a unique matching Learning, or `--query <text>` containing one literal Learning identity. Reject conflicting operands, multiple identities and text with no usable identity. Do not infer an identity through an LLM, a summary hash or task-description similarity.

**Serves:** FR-1 · FR-2 · FR-3 · UB-1 · NFR-1 · NR-1

#### Concrete layout

Keep argument handling in the existing memory CLI and compose dependencies at `cmd/memory`. Add the orchestration core under `internal/memoryrepair`, importing only consumer interfaces and existing memory/Git logic. Extract a reusable typed view of the evidence verifier as needed without changing its existing decision rules. Its report already names `record_id`, `record_sha256`, classification, source paths and mismatch codes. No adapter imports enter engine packages.

The admitted evidence predicate requires: Learning kind; unchanged record bytes between the reporting committed snapshot and the configured shared target; a durable evidence binding; complete retained history; no malformed store or invalid identity/relations; and only `evidence-content-mismatch` and/or `evidence-membership-mismatch` findings already reproducible for that record on the shared target. Use the existing `witnessMismatch` semantics from #1437, not a second looser interpretation. A newly introduced caller record, changed bytes under the same ID, missing history, another kind or another evidence code refuses.

The reporting snapshot is a full caller commit plus record digest captured by the command, not accepted blindly from prose. A scan supplies its pinned target as the reporting snapshot. The identity is the configured canonical repository plus Learning ID; the fixed namespace names the one supported repair class. Individual witnesses and caller commits are evidence, not key components. Existing malformed/foreign ref content refuses rather than being adopted.

#### Observability

Report the exact inspected record, caller commit, target commit and outcome. Preserve detailed verification evidence on the canonical task; keep raw query out of routine structured events.

#### Security and privacy

Bound input files/text to 64 KiB, validate IDs before constructing refs, and pass process arguments without shell interpolation. Report text is quoted diagnostic data in the charter and cannot supply a command, binding, policy or authority. Verify caller objects locally; arbitrary report paths cannot select another repository.

### D-2 — Inspect fresh shared state independently of the caller

**Decision:** Fetch the configured code-host target and verify one pinned snapshot before admission. Use the verifier selected by that snapshot's existing source/release tool configuration. The caller's checkout and installed verifier are not evidence of current shared health.

**Serves:** FR-1 · FR-7 · FR-8 · UB-2 · UB-6 · NR-3

#### Concrete layout

Use isolated temporary Git storage/workspace with the retained history the existing evidence verifier requires. Read the target configuration and run its compatible pinned memory verifier: source-mode uses that fetched source snapshot; release-mode uses the existing verified release pin. Record the verifier identity with the target commit. Reuse existing installer/exec resolution rather than introduce another version negotiation or update service. Missing tool/history/fetch prerequisites return `verifier_unavailable` or `freshness_unknown` before shared mutation.

Use the strict existing tool resolver/exec path. A convenience resolver that falls back to an ambient PATH binary after a pin-resolution failure does not satisfy this decision.

Inspect the requested Learning's result, not merely aggregate gate color. If #1437's verifier produces `historical-unresolved` with `verified_successor`, confirm that successor and predecessor relation in the same pinned target. Return `update_required` with the inspected target commit and successor ID to an older caller. Returning the full verified target avoids guessing the minimum combination of verifier and memory commits. If the caller already incorporates that target, rerun its relevant check with its required verifier: pass returns `healthy`, otherwise `unsupported_case` with remaining diagnostics. Do not silently rebase, reset, cherry-pick or modify caller state.

An absent record or a record whose bytes changed is not this recovery. It returns an explicit refusal. A closed task or green/open PR does not establish resolution. Wait checks fetch and pin again; only verified target content authorizes an adoption result. If target verification still fails after an operation is terminal, return `unresolved` on the existing handle; a new attempt is outside v1.

Recheck the fresh predicate after winning the ref and before creating a task. If the fix landed during intake, record the no-work outcome and return adoption instructions. The worker also verifies the starting target before making a correction; it closes out as no-work if the same correction has already landed.

#### Observability

Results bind `target_commit`, `verifier_identity`, optional successor and the revalidation instruction. Never label caller work unblocked solely because shared state is fixed.

#### Security and privacy

Only the repository-configured target supplies tool policy. Query URLs cannot override bindings or execute code. Existing trusted source/release execution rules apply; this command does not weaken artifact verification.

### D-3 — Reserve the shared operation before creating a task

**Decision:** Atomically create a persistent repair ref on the configured code host before any tracker mutation. One live winner owns task creation and execution. Losing callers only observe it. Do not implement crash recovery, expiry or automatic takeover.

**Serves:** FR-3 · FR-4 · FR-6 · UB-3 · UB-4 · UB-6 · NFR-2 · NFR-3

#### Concrete layout

The ref is `refs/pyxis/memory-repairs/inherited-learning-evidence/<Learning-ID>`. Its commit contains a closed `learning-repair/v1` JSON record: schema, canonical repository identity, Learning ID and SHA-256, inspected target, verifier identity, owner token, phase, optional canonical task/proposal references and diagnostic code. Ref existence denotes the operation, not a liveness guarantee. Keep the terminal record; do not delete it and permit another caller to repeat completed work.

Implement the consumer's `RepairStore` through existing Git-ref compare-and-swap machinery (`internal/gitrefstore`) with create-if-absent and updates conditional on the observed head. Serialize all mutations under the winning owner token. A local mutex may optimize contention, but the remote atomic update supplies correctness. Validate loaded identity, allowed phase transitions and owner on every write. Do not base ownership on a tracker search index.

The ordinary phases are `reserved`, `creating_task`, `task_ready`, `dispatching`, `running`, `awaiting_merge`, `resolved`, `failed`, and `outcome_unknown`. Persist `creating_task` before the single create request and `dispatching` before the single executor launch. A definite error records failed; an ambiguous response records outcome_unknown when possible. Both remain on the same handle and subsequent callers do not retry them. A stopped process can leave the prior phase indefinitely; command help states this limitation. Resolving it is WR-1, not a hidden claim of recovery.

The live owner calls a context-aware `TaskCreator` consumer interface and stores the returned exact source reference. Its adapters wrap existing tracker creation capabilities, including the projection adapters' create-issue operation and Carina's Creator; they do not introduce a new tracker implementation. The task contains a machine-readable operation marker and self-contained evidence/acceptance. Do not add a generic fleet-discovery label that could launch a second implementation outside the selected direct-issue path. Honor any labels required by that selected path's actual contract.

Waiters poll the ref at an injected, bounded cadence (default five seconds), with cancellable context and no model calls. Caller cancellation affects only its own wait. The owner runs implementation as an owned foreground process; it does not start an unowned detached goroutine. A completed executor returning a proposal advances awaiting_merge. The wait/adoption path determines landed resolution using D-2.

#### Observability

Emit the shared handle and canonical task as soon as known, then progress/results through versioned JSON events when `--json` is selected. Diagnostics distinguish waiting, failed and unknown; a stalled phase is never reported as active-worker proof.

#### Security and privacy

Use the configured credential and repository binding. Do not serialize credentials or local user paths. Fail on foreign identity, malformed JSON, unexpected fields or a provider lacking required atomic-ref capability.

### D-4 — Execute the proven successor correction through implement-now

**Decision:** Invoke the existing direct-issue executor with a frozen structured charter for the canonical task. Reuse #1437's successor mechanism and preserve all existing evidence and review controls.

**Serves:** FR-4 · FR-5 · FR-8 · UB-5 · NR-1 · NR-2 · EVAL-1

#### Concrete layout

Reuse issue inspection and `issue-task-input/v1` admission from `internal/executor/issueentry` and `cmd/implement-now`. The charter includes the exact tracker source fingerprint, repository and Learning identity, current evidence report, intended lesson, scope, and acceptance. Derive independent implementation and review requirements through the existing requirements codec; configuration chooses harness/model/effort. A free-text prompt alone is insufficient admission.

The repair instruction is: preserve predecessor ID, bytes and original witness; verify the intended lesson against retained source; issue an evidence-verified Learning successor with the honored supersedes relation using the coded memory writer; prove the original inherited mismatch becomes historical-unresolved under the existing bounded predicate; and pass ordinary required checks and independent review. Do not broaden allowances, rewrite history, change branch protection or fabricate evidence. If retained evidence cannot support the correction, report the task blocked rather than manufacture a successor. Any new semantic conclusion outside that bounded correction is out of scope.

Execute from an isolated workspace based on the inspected target, not the reporting task branch. Reuse the existing per-task executor claim in addition to the earlier repair-ref reservation. The existing issue contract guards duplicate worker admission for that canonical task; the repair ref prevents duplicate tasks. Carry the repair handle as trusted invocation context so a failure inside this repair returns to this same task, never recursively calls the repair entry.

The owner records the exact published proposal and executor result. Pending checks/review are not verified completion; human merge remains the repo's existing procedure. A valid candidate alone cannot change the operation to resolved. Adoption requires D-2's fetched target check.

#### Observability

Keep the executor's existing verification/review receipts and link them from the canonical task. State a blocked correction precisely. No claims of model-backed success are inferred from hermetic adapter tests.

#### Security and privacy

Treat report prose and record content as quoted evidence. Trusted charter instructions define the scope. The repair worker has no additional approval, policy-editing or frozen-record mutation authority.

### D-5 — Route the original worker failure and the daily scan to the same entry

**Decision:** Add a narrow branch in the existing worker's memory-gate failure handling that delegates only this supported inherited Learning case. Add a bounded scan mode to the same command for daily triggering.

**Serves:** FR-6 · FR-9 · FR-10 · UB-1 · UB-4 · UB-5 · NR-2 · NR-3 · EVT-1 · NFR-3

#### Concrete layout

At the existing local/CI repair boundary, obtain the structured memory evidence report and pass the Learning identity and caller revision to this entry. Classification remains D-1's responsibility. On admitted/joined, suspend the original memory-fixing loop and observe the shared operation. On update_required, return a typed task blocker carrying the target revision and rerun instruction to the normal task integration workflow. Do not perform another memory edit, silently mark the original gate green or turn a generic checker failure into this case. A repair-task context returns blocked directly under D-4.

Expose `memory repair --scan [--json]` as a mutually exclusive alternative to request operands. Fetch one target, enumerate only unresolved inherited Learning witness mismatches and invoke the same admission core for each; intake rechecks freshness before effects. Report per-record results, including joins and refusals. Empty scans succeed without effects. Scan mode waits only for its own foreground dispatch completion, not for human merge. Existing repository triggers can invoke it daily; no new scheduler implementation or automatic schedule registration is included.

Expose `--wait` for callers that want to follow a known operation through target verification. Interruption preserves the operation. Unsupported failures remain ordinary explicit errors; the integration never interprets them as permission to bypass evidence policy. Command and worker routing are initially opt-in; use a closed optional boolean `memory.inherited_learning_repair.enabled`, default false, to activate worker delegation. Explicit command invocation remains available. Do not enable daily scheduling by default.

#### Observability

Produce `learning-repair-result/v1` results with enum outcome, record identity, target commit, optional repair/task/proposal/successor references and action. Emit EVT-1 via structured logging. Document the distinction between shared resolution and caller adoption, and the deferred abandoned-operation behavior.

#### Steady-state work

Each explicit scan fetches a target snapshot and runs deterministic evidence verification over its corpus and required retained history; each eligible Learning then incurs admission's fresh-state check and shared-ref read. A scan with no supported findings makes no tracker or executor calls. Its cadence belongs to the existing daily trigger, not a loop in this command.

Each wait poll reads the small operation ref at the injected five-second default. While awaiting merge, it also reads the fresh remote target identity. Reuse a verification result only for the identical target commit and verifier identity within that invocation; a changed target requires fetching and revalidation before an adoption result. A failed remote read still returns freshness_unknown, never cached success. This avoids rerunning a full historical verifier on every unchanged-target poll while retaining D-2's freshness requirement. Deterministic work depends on corpus/history size and target changes; this version makes no latency or monetary ceiling claim.

Only the winning admitted task incurs model work, bounded by the existing configured implement-now execution/repair budgets. Scan and wait perform zero model calls. No abandoned-operation retry service or new background process is implied.

#### Security and privacy

Parse configuration closed and preserve unrelated local settings. Use the existing integration bindings and credentials. Logs contain identifiers and bounded diagnostics, not credentials or raw reports.

### D-6 — Prove the incident, stale adoption and preservation

**Decision:** Make the #1437 reproduction the primary acceptance fixture and use the existing successor refusal cases as the negative boundary. Prove coordination with independent processes, not only a shared in-process mock.

**Serves:** DOD-1 · DOD-2 · DOD-3 · DOD-4 · DOD-5 · DOD-6 · NFR-1 · NFR-2 · NFR-3 · NR-1 · NR-2 · NR-3

#### Concrete layout

Build a hermetic retained-history repository with the same sequence as the incident: introduce the frozen Learning with a mismatching witness, advance unrelated task branches, then land a valid successor using the unchanged predecessor. Reuse the existing memory-evidence test helpers and assert exact classifications before and after. Do not require hosted historical object availability in unit tests.

Run five CLI processes against a local bare remote and a shared test tracker/executor adapter. Vary prose, witness paths and checkout SHAs; require exactly one task and one launch. Race a scan with those callers. After the correction lands, run a pre-fix caller with a stale local remote-tracking ref and an older verifier; require the target-pinned check and update_required with no new effects. Separately prove an incompatible/unavailable verifier returns UB-2.

Exercise task/dispatch failure, outcome uncertainty, cancelled waiter, recursive repair suppression and an unresolved terminal ref without implementing takeover. Assert caller HEAD/index/files and predecessor bytes remain unchanged. Run required Go race/vet/lint checks and adapter conformance. Use injected time/process execution; no live network in unit tests.

Finally run one bounded model-backed repair trial through the configured implement-now entry and retain candidate, review and verification evidence. A fixture-only green result does not satisfy EVAL-1. Document opt-in and disabled worker routing behavior and the invocation an existing daily trigger uses.

#### Observability

Retain deterministic effect receipts in tests and revision-specific validation at the owning proposal. Operational diagnostics remain ignored; no test inventory is copied into semantic memory.

#### Security and privacy

Test report injection, wrong repository and unsupported records. Use synthetic credentials and isolated test remotes. Hosted proof, if collected, states its exact repository and revision without exposing secrets.
