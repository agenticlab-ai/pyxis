# Direct issue entry for implement-now — technical design

| Field | Value |
|---|---|
| **Status** | Approved |
| **Owner** | @mdarabi |
| **Approver(s)** | @mdarabi (operator and security reviewer) |
| **Date** | 2026-10-01 |
| **Relevant-when** | implementing or changing exact issue intake, issue contract admission, issue-scoped execution, or execution attribution in implement-now |
| **Scope** | A bounded issue input to the existing executor and a basic execution summary on its proposal. Continuous watching, decomposition, full harness telemetry, planning, and merging remain outside this decision. |
| **Tracking** | — |
| **Builds-on** | docs/tech-design/implement-now-authenticated-review-obligations.md · docs/tech-design/native-routing-policy-v1.md · docs/tech-design/impl-review-host-persistence.md |
| **Supersedes** | — |
| **Serves** | prd:implement-now/FR-50 · prd:implement-now/FR-51 · prd:implement-now/FR-52 · prd:implement-now/FR-53 · prd:implement-now/FR-54 · prd:implement-now/FR-55 · prd:implement-now/NR-11 · prd:implement-now/UB-22 · prd:implement-now/DOD-37 |
| **Security/privacy impact** | yes — untrusted issue content becomes an execution input; exact selection controls repository/tracker authority; shared evidence and public attribution must exclude credentials and personal local paths. reviewer: @mdarabi. |
| **Template version** | tech-design-template/v1 |

Operator approval: @mdarabi, 2026-10-01 — "yes, go for coding", in response to the linked design and amendment approval request.

## 0. Read to rule — the human reviewer's section (~6 min)

*The body is the contract; this section only routes attention — on any conflict the body governs; machines ignore it.*

**Judge these**

| # | The call | Could have gone | If you disagree |
|---|---|---|---|
| 1 | The invoking agent derives the short intake; Go validates and admits it | Dedicated intake agent/service (#ALT-1) | add orchestration before the worker → §3.1 |
| 2 | Exact issue reads include native blockers; issue mode never scans ordinary task/review frontiers | Filter the label frontier (#ALT-2) | accept omitted blockers or unrelated work → §3.2, #DINV-1 |
| 3 | Digest-addressed contracts use the existing Git ref store and native task codec | Mutable issue text or new database (#ALT-3, #ALT-4) | replace the recovery authority → §3.3, #DINV-2 |
| 4 | Reuse the worker; require usable independent review and exact current revision before a verified-PR result | Second executor or green-only result (#ALT-5) | change review/completion authority → §3.4, #DINV-3 |
| 5 | Publish observed execution data with explicit unavailable counters and elapsed-time boundary | Full transcript telemetry (#ALT-6) | expand instrumentation → §3.5, #DINV-4 |

**Sanity-check the numbers** — one selected issue (#DINV-1); retries retain one contract/task/branch/proposal identity (#DINV-2); published/local/CI/review revisions agree (#DINV-3); unknown counters remain unavailable (#DINV-4).

**The author's open questions** — none about the entry mechanism. Existing configured-review and routing-evidence production readiness remain prerequisites, independently verified before release (§3.8, A-7).

**Skip the rest** — structure → docgate; substance/reuse → technical-design review; worker/reviewer internals → the existing executor and impl-review designs; defects/provider behavior → §6 tests; alternatives → §4.

## 1. Problem

The advertised implement-now entry expects a frozen projected plan even though the executable already supports native task requirements and the worker operates on one portable task. An ordinary issue cannot safely be handled by filtering a normal sweep: adapters parse unrelated task bodies first, and the dispatcher determines blockers from that returned frontier. Omitting an outside blocker can make blocked work appear ready. Review handoff repair also precedes task selection and can perform unrelated work.

A directly admitted issue needs a small immutable charter, exact authorization and readiness, and a stable recovery identity. The same worker already owns workspaces, implementation bounds, verification, publication, and review handoff. The useful initial attribution data are its observed launches, available token totals, attempts, and measured execution duration.

## 2. Decision

**Decision:** Add an explicit one-issue entry to the existing implement-now command and skill. The invoking agent prepares a short typed intake; the Go entry exact-reads and freezes it, creates one current native task, and supplies only that task and its own review obligations to the existing dispatcher. The entry requires usable configured independent review and publishes a basic typed execution summary; verified completion requires the current proposal revision to match local verification, CI, and authenticated review.

This adds task authoring/admission ahead of the composition defined by `implement-now-authenticated-review-obligations`. It does not replace that composition or alter the single plan→tracker projection, implementation outcome, reviewer publication authority, or activation controls.

## 3. The design

### 3.1 Invoking-agent intake and command boundary

**Decision:** Extend the skill with an issue branch and the Go command with mutually exclusive `--issue <native-number>` plus `--issue-input <file>` operands. The configured task binding selects the tracker instance and repository/root; a URL supplied to the skill must match that binding before conversion to a number. Plan mode remains its existing branch. Issue mode rejects plan operands, review-only operands, or missing paired issue/intake operands.

The invoking agent reads the issue and relevant code, resolves routine implementation choices within the issue's scope, and writes an ignored scratch input. Genuine unresolved product decisions or work requiring decomposition stop intake with a concise reason. There is no additional model process for intake.

Use the read-only `--inspect-issue <number>` command to obtain the exact source identity, fingerprint, and comment citation digests. On restart it also returns the admitted contract from shared Git; reuse that exact input instead of deriving a different charter. The bounded charter records the chosen implementation scope and acceptance basis. This operator-authorized lane does not claim that all six planning stages ran or were mechanically certified.

The closed `issue-task-input/v1` object contains:

- `schema_version` and a typed source binding: provider, opaque binding digest, native issue number, source fingerprint, and cited comment identities/digests;
- `title`, `goal`, `scope`, and non-empty `acceptance` entries;
- current `taskrequirements.Spec`, with independent implementation and review requirements and resolvable source citations.

Factor the portable charter compilation in `cmd/carina/native_task.go` into the native taskspec package so both Carina task creation and direct issue intake use the same validation and requirements compilation. This compiler chooses no harness/model/effort. Code references identify the existing integration seams; they do not freeze their file layout.

### 3.2 Exact selection and native readiness

**Decision:** Add one consumer-owned exact-issue reader contract implemented by the existing tracker adapters. Its typed snapshot returns native identity, title/body, cited source comments, executable/open state, and native dependency identities/states. GitHub and Forgejo reject pull-request objects. Carina rejects closed/non-executable/rollup items and uses the same native dependency semantics as its ready command.

Read the selected issue, cited source comments, and dependencies directly. Do not call the ordinary label frontier and then filter its result. An open or unreadable dependency blocks even if it has no agent label, is outside the selection, or is not itself an executable task. The caller's explicit selection authorizes this issue without mutating an agent label; configured credentials and native claim authorization still apply.

After admission, a scoped tracker exposes exactly one task to the dispatcher and restricts comments to that native issue. The selected task carries the same source key and native number used by existing fleet claims. Readiness is computed from exact native dependency states; the ordinary dispatcher's candidate-set blocker heuristic must not override it.

Review lifecycle repair, pending-frontier construction, retries, and backend-union calculation are restricted by the admitted contract identity before any unrelated proposal/artifact lookup, repair, backend authentication, claim, or model launch. Fetching and decoding the existing shared metadata/index to select that scope is permitted; it grants no execution authority over another row. Exact selected-source comments and native dependency facts are also permitted read-only exceptions. The existing review-only path provides the relevant selected-obligation pattern. An issue invocation must not repair or dispatch unrelated review state merely because it shares a repository or scope label.

### 3.3 Immutable admission and reuse

**Decision:** Store canonical admitted native task bytes and bounded origin metadata through `internal/gitrefstore`, using a dedicated implement-now issue evidence ref and digest-addressed paths. Reuse its fetch, exact-read, CAS, and idempotent-update mechanics; do not add a database or a general amendment engine.

The source fingerprint covers title/body and native dependency identities. The contract separately binds each explicitly cited source comment identity and body digest. Labels, timestamps, generated execution comments, and later uncited comments are excluded. This is a bounded snapshot; later issue discussion is future watcher work. A cited comment edit/deletion or changed semantic source is a conflict on re-entry.

The canonical contract contains the source identity/fingerprint, charter, both compiled role requirements, schema version, and compiler/policy identities needed to reproduce compilation. Its ID is a domain-separated SHA-256 digest of those canonical bytes. The task ID is a stable `adhoc/issue-<source-identity-digest>`; the native carrier's existing fingerprint field holds the contract digest and its plan reference remains empty. No plan document, plan version, or legacy demand-origin event is fabricated.

One source identity admits one contract in this phase. A repeat with identical bytes is a no-op; a different admitted contract, conflicting pre-existing native capsule, or partial/unverifiable record refuses before claim. The stored bytes, not a local cache, reconstruct the task on restart. A fresh clone with the same resolved binding reconstructs the same contract; changing the authoritative Carina root changes the binding rather than silently aliasing it.

For public metadata, use hosted issue URLs or `carina:<opaque-binding-digest>#<number>`. Do not publish a Carina absolute root or source text. The current resolved source key remains available locally for the shared claim mechanism; public serialization uses only its digest.

### 3.4 Existing worker and exact completion

**Decision:** Dispatch the admitted task through the existing implementation path in `cmd/implement-now/run.go`, `internal/executor/dispatch`, and `internal/executor/worker`. Reuse backend resolution, native requirements routing, lease, deterministic branch, disposable workspace, fresh harness invocation, local gate, bounded repair, duplicate-proposal association, code-host adapters, checks, and durable review handoff.

Issue entry requires a present, usable configured review block before claim or implementation launch. It does not silently install a reviewer or change plan mode's absent-review behavior. Authenticated review remains a fresh separately bounded process in an isolated workspace; impl-review's host broker remains the sole publisher of its report. Preserve the exact receipt/attempt proof and EVT-13↔EVT-17 join.

Implementation success remains the existing implementation result. Pending review preserves that success and makes the direct-entry result non-success. After the selected review completes, exact-read proposal state and checks again. A `verified_pr` result requires one proposal association for the contract and equality of current published, local-verified, CI-verified, and reviewed revisions. Head drift or pending/failed checks is an explicit resumable non-success; no loop watches new comments or merges the PR.

Require at least one configured required local code-gate check before spend. Retain the proposal association and revision-specific local gate proof on the issue evidence ref. A resumed invocation exact-reads those facts and the contract's existing review state; it invokes at most one selected pending review attempt and never redispatches implementation from an associated proposal's check color alone. Bounded local/CI repair remains inside the originally admitted worker; interrupted or exhausted implementation returns pending verification and needs independently authorized recovery. A changed published head cannot borrow an earlier local or review proof.

### 3.5 Basic execution attribution

**Decision:** Add a closed `issue-execution-summary/v1` record and a consumer-owned summary-publication capability on the existing code hosts. A host-owned marker in the PR body holds readable text and typed JSON; updates replace only that owned section, preserving the task association and authored body. Repeated publication is idempotent for the same proposal/contract/attempt summary.

The record carries contract and public source references, proposal/revision, terminal class, and per-role attempt records with observed harness/model/effort. Capture actual launch values and completion usage through existing runtime/results and review events. Sum only reported usage, avoiding double-counting review-owned child records. Tokens, turns, and tool counts use explicit observed/unavailable states; available token totals need not imply an input/output/cache breakdown. Missing role/launch observations remain unavailable rather than inferred from config. Unavailable counters carry a reason.

Retain bounded observations in shared Git by host-generated command/launch identity so review or metadata retries preserve earlier role observations without counting replayed usage twice. `usage_scope` distinguishes the current invocation from retained invocations; partial totals are labeled partial. Duration continues to describe the latest command, not the sum of previous command durations.

Measure duration with an injected monotonic clock from Go command entry through selected terminal verification, before final summary serialization/publication. Label it `execution_elapsed_ms`, with boundary `command_start_to_terminal`; the final metadata write and intake in the already-running invoking session are excluded. Intake time/tokens are unavailable unless a future harness adapter supplies a bounded measurement. This phase does not promise full request-to-PR telemetry.

The proposal opens with a running/provisional section and receives a final summary after the selected implementation/review attempt terminates. Final summary publication failure retains the verified artifact but returns a publication failure with references; it cannot silently claim a fully delivered result. Summary data never authorizes review success or replaces canonical evidence.

### 3.6 Boundaries and non-goals

- **Not solved here** — continuous comment/CI watching, issue decomposition, tracker-independent planning, merge driving, cost estimation, per-tool token attribution, or transcript ingestion.
- **Owned elsewhere** — the existing worker/review/routing/activation designs own their process and evidence semantics; harness adapters own observations; human authority owns landing.
- **Implementation must not infer** — source authority from a matching issue number, readiness from a truncated frontier, routing requirements from prose inside Go, successful review from process exit, or missing usage as zero.
- **Must remain unchanged** — plan projection and execution, shared fleet claims, e2e task classification/admission, reviewer publisher ownership, implementation truth, and activation/release authority.

### 3.7 Invariants

- **DINV-1** — Only the selected provider-qualified issue and its admitted-contract obligations can be repaired, claimed, implemented, or reviewed; all blockers are exact-verified before claim.
- **DINV-2** — Identical admitted bytes retain one recovery identity; different source/contract bytes cannot overwrite that authority or silently alias another provider/root.
- **DINV-3** — A verified-PR result joins the exact current proposal revision to local verification, CI, and authenticated independent review without redefining implementation success.
- **DINV-4** — Attribution describes observed execution, preserves unavailable values, and states its timing boundary; it never becomes review or routing authority.
- **DINV-5** — Issue entry preserves inherited runtime/review budgets, workspace isolation, source-key claims, and activation controls; no merge authority is introduced.

### 3.8 Compatibility and release prerequisites

This is an additive entry design: its upstream task-authoring scope was explicitly outside the existing executor composition. There remains one plan projection and one executor. Existing plan fixtures must pass unchanged.

Amendment 008 supersedes NR-1 with NR-11. The frozen `docs/plans/implement-now/001-plan.md` T5/coverage rows and `implement-now-authenticated-review-obligations` Serves header cite historical NR-1. Those artifacts remain immutable. Their old NR-1 liveness checks require a legally authored successor/compatibility treatment before a current-contract plan recompilation is advertised; passing runtime plan fixtures does not prove their unchanged fold verifies against the newly amended PRD. Existing FR-6/FR-12 retired-reference failures in that old plan predate this amendment. This phase does not rewrite delivered tasks or historical designs.

Compiler/provider tests start with a supplied charter and do not measure the invoking agent's semantic intake quality. A calibrated issue-to-charter admit/refuse corpus remains a release-evaluation advisory; no additional intake agent or evaluation service is introduced in this phase.

An activated routing-evidence plane currently recognizes a closed demand-origin contract. Issue admission must not forge a plan-origin event or disable that verifier. If the configured plane cannot verify an admitted native issue source, reject before claim with `unsupported_issue_origin`. A production release using that plane requires its owning contract/design to authorize and validate the source first. That authorization is not smuggled into this entry design.

The existing authenticated-review setup, source ref initialization/protection, backend conformance, and activation proofs remain required. Implementation fixtures may prove the entry path without claiming those prerequisites have shipped. The delivery record must distinguish code/tests completed from a usable production configuration; unresolved prerequisites prevent calling the feature production-ready.

### 3.9 Observability

- **Events/logs** — structured intake/admission/refusal and terminal diagnostics correlate source digest, contract ID, task, attempt, proposal, and revision; runtime diagnostics remain ignored logs.
- **Metrics** — bounded per-role observed usage, attempts, and execution elapsed time appear in the typed proposal summary; missing observations are explicit. No separate dashboard is introduced.
- **Alerts** — no new daemon or alert service; selected non-success returns non-zero with the typed reason through the existing report sink.
- **Debugging surface** — admission records and existing attempt/review references explain conflicts, pending review, or stale verification; raw issue prompts/transcripts are not execution-summary fields.
- **Trace/correlation** — propagate contract/task/attempt identity through existing events and review handoff; summary references that evidence rather than creating another success record.

### 3.10 Security & privacy

Issue text and comments are untrusted data, never permission to switch repository, shell operands, harness flags, or credentials. Closed decoding and comparison with the resolved configured binding enforce #DINV-1/#DINV-2. Native issue numbers are validated before adapter calls. Git storage remains host-owned; the implementation/review child receives the charter but no contract-store mutation capability.

Persist the operator-derived bounded charter and digests needed for recovery, not an indiscriminate issue/comment archive. Review retains its existing isolated capability and protected proof. Public attribution contains model/harness identifiers and aggregate observations but no credentials, environment values, personal paths, raw prompts, or session transcripts (#DINV-4/#DINV-5). Public source identity uses opaque Carina binding digests.

### 3.11 Testing strategy and change boundaries

The native charter compiler, closed codecs, identity, summary renderer, and scoped selection are in-process and table-tested. Exact-read adapters and code-host summary publishers receive provider conformance suites using existing fake HTTP/CLI transports. Native Carina fixtures exercise the real ready/show semantics. The shared Git ref store uses a local bare remote for concurrent/restart/fresh-clone admission proof. Harnesses are true external dependencies behind the current runtime fake; one installed-flow rehearsal proves launch capture when configured readiness permits it.

Expected implementation touches are the native compiler, exact-read tracker adapters, command/skill branch, dispatch scoping, a small admission record wrapper over gitrefstore, attempt observation/summary codec, and the existing code-host publishers. No new watcher, scheduling service, general workflow, or harness transcript parser is required.

## 4. Alternatives considered

| ID | Alternative | Why rejected |
|---|---|---|
| ALT-1 | Dedicated intake model/service before dispatch | the invoking session can perform the same bounded reasoning; a new process introduces handoff and telemetry scope without an initial capability gain |
| ALT-2 | Filter the ordinary label frontier by issue number | unrelated capsules are parsed, native outside blockers can disappear, and review repair can run before filtering |
| ALT-3 | Treat current issue text as the execution contract | mutable text cannot reproduce admitted requirements or stable retry authority |
| ALT-4 | Introduce a new execution database/amendment service | existing Git ref CAS and native task serialization provide bounded immutable recovery; amendment/multi-contract lifecycle can be designed when needed |
| ALT-5 | Implement another issue-specific worker or count green CI alone | duplicates isolation/recovery/publication and weakens independent exact-revision review |
| ALT-6 | Instrument every harness transcript and tool call immediately | useful baseline observations already exist; full instrumentation is independently scoped by the worker telemetry feature |

## 5. Risks

Owner: @mdarabi for scope, readiness, and security acceptance; the implementing worker supplies the named verification evidence.

| ID | Risk | Impact | Mitigation | Verification / evidence |
|---|---|---|---|---|
| R1 | Same issue number or omitted blocker widens authority/readiness | wrong or blocked work executes | exact binding and native dependency reads before scoping | A-2 provider/refusal matrix |
| R2 | Retry recomputes a different task from mutable text | duplicate PR or silent scope change | canonical immutable contract and source conflict refusal | A-3 restart/concurrency/fresh-clone proof |
| R3 | Existing review repair still walks all repository obligations | unrelated reviews or mutations | restrict before repair/backend-union/read, not only before model launch | A-2 instrumented zero-unselected operations |
| R4 | Metadata reports configured choices or zero-filled unknown usage | false attribution and cost comparisons | actual launch capture and typed unavailable states | A-5 observed/unavailable goldens |
| R5 | Native issue origin or authenticated review is not production-ready | tests pass but advertised flow remains unusable | explicit inherited prerequisite checks, no bypass or activation | A-7 readiness refusal and release evidence |
| R6 | Source text or Carina paths escape into public metadata | public disclosure | bounded intake, source digests, opaque public binding identity | A-5 payload audit |

## 6. Acceptance

| ID | Acceptance criterion | Verify | Required evidence | Traces to |
|---|---|---|---|---|
| A-1 | A bounded issue on each configured tracker enters with a short portable intake and no planning artifacts; plan mode remains unchanged. | Test + Demo | CLI-to-worker fixtures and plan regression suite | prd:implement-now/FR-50 · prd:implement-now/NR-11 |
| A-2 | Cross-provider/repository/root references, closed/non-task/PR items, open/unreadable blockers, unrelated malformed capsules, and unrelated review obligations never widen reads/repairs/claims/launches. | Test + Failure injection | exact-read conformance and instrumented negative matrix | prd:implement-now/FR-51 · prd:implement-now/UB-22 · #DINV-1 |
| A-3 | Repeated, concurrent, restarted, and fresh-clone entry with the same binding/contract preserves task/claim/branch/proposal identity; changed source/intake or persistence failure refuses claim. | Test + Failure injection | canonical goldens and local bare-remote recovery tests | prd:implement-now/FR-52 · #DINV-2 |
| A-4 | Missing/unusable independent review stops before implementation; a verified result requires current proposal/local/CI/authenticated-review revision equality through the existing worker and impl-review publisher. | Test + Demo + Failure injection | selected-flow proof, head-drift/pending/stale checks, authenticated receipt join | prd:implement-now/FR-53 · #DINV-3 |
| A-5 | Both code hosts publish idempotent readable/closed typed summaries with actual role launch capture, available usage, explicit unavailable values, and stated command timing; public metadata contains no secrets/personal paths/raw input. | Test + Inspection | publication and usage goldens, repeated-update preservation tests, payload audit | prd:implement-now/FR-54 · #DINV-4 |
| A-6 | Failure/exhaustion remains within inherited budgets and preserves recovery evidence/work; no watcher, implicit planning, decomposition, unselected dispatch, or merge is created. | Test + Inspection | bounded failure fixtures and mutation audit | prd:implement-now/FR-55 · #DINV-5 |
| A-7 | Unsupported activated issue origins and unmet review/evidence prerequisites refuse before claim; production readiness is reported only with inherited release proof and installed-flow evidence. | Test + Demo + Inspection | preflight refusal matrix and explicit prerequisite/readiness record | prd:implement-now/FR-53 · prd:implement-now/FR-55 · prd:implement-now/DOD-37 |

## 7. Open questions

None about the selected entry mechanism. Existing production-readiness dependencies are explicit in §3.8; they are release prerequisites rather than choices the worker may infer.

## Principles

- **AP-5 — honored.** Observed execution, refusal, and pending-review evidence are visible from the start.
- **AP-6 — honored.** Cross-boundary intake, snapshots, admission, and summary data use closed versioned structs.
- **AP-25 — honored.** The issue branch lives on the vendor-neutral skill surface; portable requirements carry no executor choice.
- **AP-27 — honored.** Provider capabilities extend their existing adapters without introducing a new central provider enumeration; any needed aggregate is derived from the configured bindings.
- **AP-29 — honored.** Existing e2e classification and hermetic admission are unchanged; tests inject time and external observations.
- **RW-5 — honored.** One additive entry decision is reviewable before implementation; contradictory source/activation policy changes require their own owning contracts.
- **Operator ruling:** direct issue entry freezes a bounded charter without separate per-issue planning artifacts. It makes no claim that the complete vision planning workflow executed for that issue; the existing verified implementation/review and human landing boundaries continue to apply.
