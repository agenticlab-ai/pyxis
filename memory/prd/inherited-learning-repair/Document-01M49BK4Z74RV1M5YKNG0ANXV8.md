# Inherited Learning repair — PRD

| Field | Value |
|---|---|
| **In one line** | Coordinate one repair when an inherited frozen Learning's invalid evidence blocks concurrent agents. |
| **Relevant-when** | Building shared admission for the Learning-evidence failure demonstrated by PR #1437 or handling stale callers of that repair. |
| **Status** | Draft — independent review and exact-head operator lock approval pending |
| **Tier** | T3 — shared task dispatch and evidence feeding factory gates |
| **Owner** | Operator |
| **Approver(s)** | Operator through the prepared draft proposal |
| **Contributors** | Argus, authority-contract/v1 tier-2 |
| **Informed** | Agents blocked by inherited Learning evidence |
| **Date** | 2026-10-05 |
| **Appetite** | One observed failure class; reuse the existing executor; exclude crash recovery and general memory diagnosis. No numeric spending allowance inferred. |
| **Target release** | After the five-caller reproduction and stale-caller proof pass |
| **Tracking** | Feature draft inherited-learning-repair; https://github.com/agenticlab-ai/pyxis/pull/1437 |
| **Artifacts** | problem:inherited-learning-repair; delivery:inherited-learning-repair |
| **Template version** | prd-template/v2 |
| **Design coverage** | requirements-first/v1 |

## 0. Read to rule — the human reviewer's section (~4 min)

*The body is the contract; this section only routes attention — on any conflict the body governs; machines ignore it.*

**Judge these**

| # | The call | Could have gone | If you disagree |
|---|---|---|---|
| 1 | Only #1437's inherited Learning failure | General memory repair | Reopen NG-1 and the operator's scope ruling |
| 2 | One repair, then caller adoption | Independent repairs in every branch | Reopen FR-3 through FR-8 |
| 3 | Defer abandoned-owner and uncertain-effect recovery | Automatic retries and takeover | Reopen WR-1 |

**Sanity-check the numbers** — five callers, one task and one dispatch; zero predecessor rewrites: FR-3, FR-5, NR-1.

**The author's open questions** — no new product ruling identified; reviews and exact-head approval remain pending.

**Skip the rest** — draft readiness checks structure; independent reviews assess substance; implementation acceptance proves behavior.

## 1. Summary — the one-pager

An inherited frozen Learning with invalid evidence blocked unrelated tasks. The operator reported five agents independently trying to repair it. This feature supplies one shared repair and directs old checkouts to an already-landed fix. Five concurrent requests must yield one task and one implementation invocation; stale callers must create neither. Preserve the historical record and use the verified-successor behavior introduced by PR #1437. Other memory problems and crash recovery are outside this release.

## 2. Problem space

### 2.1 Problem statement

Agents on different branches can encounter the same inherited memory gate failure, independently investigate it and make competing edits. An old checkout can repeat that work even after a fix lands.

### 2.2 Users and segments

Primary users are agents implementing unrelated tasks; the operator needs one understandable repair. Reporters of incorrect advice, other record defects or new branch-local failures are outside this version.

### 2.3 Jobs and desired outcomes

| # | When… (situation) | I want to… (motivation) | So I can… (outcome) | Today's workaround | Evidence |
|---|---|---|---|---|---|
| J1 | Inherited Learning evidence blocks my task | Join one repair | Resume without independently editing memory | Each agent investigates | Operator's incident report |
| J2 | My checkout reports an already-fixed blocker | Obtain the fixing state | Update and revalidate | Rediscover the fix | problem:inherited-learning-repair#A-3 |

### 2.4 Evidence

| Claim | Source | Confidence |
|---|---|---|
| Learning-01M44G5XQQAJDWSVCW81V1SYMD failed both content and membership evidence checks | PR #1406 diagnostics; https://github.com/agenticlab-ai/pyxis/actions/runs/37269590734; PR #1437 | High: inspected diagnostics and source |
| Five agents independently encountered the blocker | Operator's report in the problem ledger | Medium: direct report, not an independent session count |
| A verified successor can resolve blocking use while preserving failed history | PR #1437, merged commit 0d03872f278291595a009cd09e94ecade6818575 | High: existing implementation and refusal fixtures |

### 2.5 The journey today

An agent encounters a gate failure, investigates historical evidence and frozen-record restrictions, then attempts its own correction. Others may do the same. After merge, older callers still need to adopt the correction and compatible verifier.

### 2.6 Opportunity and why now

The existing successor mechanism supplies the correction semantics; shared coordination is the missing behavior. The operator accepted the problem and directed this feature in A-5. That does not approve the future prepared head.

| Reviewer | Role | Date | Verdict |
|---|---|---|---|
| Operator | Problem provider | 2026-10-05 | Problem and bounded direction accepted in conversation; contract lock pending |

## 3. Goals, metrics, and guardrails

### 3.1 Goals as falsifiable hypotheses

| # | We believe that… | Will result in… | Measured by… |
|---|---|---|---|
| G1 | Shared ownership replaces independent repairs | One repair for concurrent callers | M1 |
| G2 | Checking fetched target state detects existing fixes | No new repair for stale callers | M2 |

### 3.2 Goals → signals → metrics

| Metric | Goal | Signal | Baseline | Target (at checkpoint) | Feeds from |
|---|---|---|---|---|---|
| M1 | G1 | Task and dispatch receipts | Five attempts reported; exact effects unmeasured | Five concurrent requests, one task, one dispatch | EVT-1 and fixture receipts |
| M2 | G2 | Effects after a landed correction | No shared admission guard | Zero task creations and dispatches for stale callers | EVT-1 and fixture receipts |

### 3.3 Guardrails and decision rule

Zero predecessor byte changes, evidence exemptions or automatic merges. Any violation fails acceptance and prevents automatic routing activation. Fixture success does not establish a production incident rate.

### 3.4 Non-goals and waiting room

| ID | Non-goal | Why refused |
|---|---|---|
| NG-1 | Other memory failures, semantic correctness, fuzzy task matching and general triage | Explicit incident-only scope |
| NG-2 | Repair of new or rewritten caller-branch records | Not the inherited frozen-record case |
| NG-3 | Automatic merge or modification of callers' working branches | Existing workflow ownership remains |
| NG-4 | A new scheduler or central service | A bounded invocation fits an existing daily trigger |

| ID | Waiting room | Revisit when |
|---|---|---|
| WR-1 | Crash recovery, owner takeover, redispatch and uncertain-effect reconciliation | Operator reopens recovery design |
| WR-2 | Additional failure classes | Operator selects another observed incident |

## 4. Solution space

### 4.1 Approach and alternatives considered

The operator selected a Go entry, configured task and Git-ref coordination, and implement-now execution. Ref reservation precedes task creation; verified Learning identity replaces wording as the duplicate key. Independent repairs repeat the incident; semantic matching cannot prove equivalence; a general framework exceeds A-4.

### 4.2 Scope perimeter

Include incident classification, shared admission, task creation, executor dispatch, waiting, stale-caller results and a bounded daily scan. Input narrowing and scan composition are attributed assumptions in delivery:inherited-learning-repair#E-3.

### 4.3 Experience: flows and states

F1: report one Learning; receive a shared handle, an existing-fix instruction, or an explicit refusal. Waiters follow the winning operation without an agent repair loop. F2: run a scan; supported unresolved Learnings enter F1. Empty scans return no work. Dependency failures expose their step and preserve the report. Cancelling a waiter leaves the repair unchanged.

### 4.4 Scenarios

Five agents reporting different witnesses of one Learning share a repair. A pre-fix caller after merge receives update_required. A different record kind receives unsupported_case. The conversation authorizes this direction; exact-head approval remains outstanding.

## 5. System requirements

### 5.1 Functional requirements

| ID | Requirement (EARS) | Rationale | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|
| FR-1 | When a request identifies one Learning, the entry shall classify its evidence against a freshly fetched configured target snapshot. | Freshness | Must | high | Target SHA is fetched during intake; stale remote-tracking state cannot authorize a task | Test | G2, J2, F1 |
| FR-2 | When the target contains the byte-identical frozen Learning with the supported inherited witness mismatch, the entry shall admit that Learning for repair. | Incident boundary | Must | critical | #1437 reproduction admits; wrong-kind, rewritten, missing-history and new-record cases refuse | Test | G1, J1, F1 |
| FR-3 | When five callers concurrently report an admitted Learning, the entry shall assign them the same repair identity. | Shared ownership | Must | high | Wording, witness paths, caller SHAs and machines do not split the operation | Test | G1, J1, F1 |
| FR-4 | When a caller wins a new repair, the entry shall create one task on the configured tracker describing its evidence and acceptance. | Understandable work | Must | high | One task contains identity, inspected target, scope and postcondition | Test | G1, J1, F1 |
| FR-5 | When the canonical task is ready, the owning entry shall invoke implement-now once under configured execution and review policy. | Existing executor | Must | critical | Five live concurrent callers produce one admitted charter and one implementation invocation | Test | G1, J1, F1 |
| FR-6 | While a repair exists, a joining caller shall receive its task and proposal references as available. | Shared progress | Must | med | Join and wait use the winning operation with no task creation or dispatch | Test | G1, F1 |
| FR-7 | When the fetched target resolves the reported defect through a verified successor, the entry shall return the verified target revision to adopt. | Stale callers | Must | high | Pre-fix callers get update_required with zero shared writes and dispatches | Test | G2, J2, F1 |
| FR-8 | When the correction lands, the waiting interface shall direct callers to incorporate the verified target and rerun their failed check. | Caller readiness | Must | high | An open or merely green PR never proves resolution; verified target content does | Test | G2, J2, F1 |
| FR-9 | When a bounded scan runs, the entry shall submit supported unresolved Learnings through the agent admission path. | Daily use | Must | high | Scan plus agent callers converge; empty scan creates nothing | Test | G1, F2 |
| FR-10 | When the existing worker recognizes this inherited Learning blocker, the worker shall route its diagnostic to the shared entry instead of independently correcting memory. | Prevent duplicate fixing | Must | critical | Reporting workers make no local memory repair edit in the incident reproduction | Test | J1, F1 |

| ID | Existing behavior protected | Why protected | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|
| NR-1 | Frozen predecessor identity, bytes and evidence | Retained history | critical | Exact pre/post equality; failed witness remains visible | Test | FR-2, FR-5 |
| NR-2 | Evidence refusals, review gates and merge authority | No bypass | critical | Existing #1437 refusal suite passes; no allowance widening or autonomous merge | Test | FR-5, FR-8 |
| NR-3 | Caller work and configured integration choices | Isolation and portability | high | Intake preserves caller HEAD/index/worktree; adapters honor explicit bindings | Test | FR-1, FR-4 |

### 5.2 Unwanted behavior and failure handling

| ID | If… (trigger) | Then the system shall… | User sees | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|---|
| UB-1 | If input identifies no unique Learning or an unsupported failure | return a non-mutating refusal | invalid_input or unsupported_case | Must | high | Zero ref/task/dispatch effects | Test | FR-2 |
| UB-2 | If fresh history or a compatible verifier is unavailable | report the unavailable prerequisite | freshness_unknown or verifier_unavailable | Must | high | Cached evidence cannot produce admission or a fixed verdict | Test | FR-1 |
| UB-3 | If task creation or dispatch fails or is uncertain | preserve the same operation and report the step | failed or outcome_unknown | Must | high | Subsequent callers neither duplicate nor retry these effects | Test | FR-4, FR-5, WR-1 |
| UB-4 | If a waiter is cancelled | stop only that wait | cancelled with repair handle | Must | med | Shared operation remains observable | Test | FR-6 |
| UB-5 | If the repair worker encounters its own blocker | retain the failure on its task | blocked without recursive dispatch | Must | critical | Invocation count remains one | Test | FR-5, FR-10 |
| UB-6 | If a terminal operation exists but target verification still fails | expose the unresolved operation | unresolved with task reference | Must | high | No false resolution or automatic new attempt | Test | FR-7, WR-1 |

### 5.3 Permissions and abuse cases

Configured credentials authorize existing provider operations. Report text is data, not commands or authority. Test injected instructions to change repositories, rewrite the predecessor or bypass checks; FR-2, NR-1, NR-2 and UB-1 must hold.

### 5.4 Data

Repair identity, target evidence and task/proposal references are shared operational state. Diagnostic text does not become semantic memory. No corpus migration occurs. Credentials never enter refs, tasks or logs; existing record retention stays unchanged.

### 5.5 Interfaces and integrations

Use configured code-host, tracker and executor bindings. Keep the evidence checker local and deterministic. The scan is callable by an existing daily trigger; this feature does not install a new scheduler.

### 5.6 Non-functional requirements

| ID | Area | Requirement | Fit criterion | Verify |
|---|---|---|---|---|
| NFR-1 | Determinism | The entry shall derive identity from verified repository and Learning identity without model inference. | Wording variants yield identical keys; zero model calls before dispatch | Test |
| NFR-2 | Concurrency | The entry shall coordinate independent processes sharing only configured remote state. | Five-process fixture passes under race detection | Test |
| NFR-3 | Operability | The entry shall expose machine-readable outcome, inspected revision and canonical references. | CLI golden cases cover admission, join, update and refusals | Test |

### 5.7 Instrumentation — the tracking plan

| ID | Event name | Fires when | Properties (typed) | Source | Feeds |
|---|---|---|---|---|---|
| EVT-1 | learning_repair_outcome | Intake or wait returns | outcome:string; record_id:string; target_commit:string; repair_ref:string; task_ref:string; proposal_ref:string | Repair entry, excluding raw query and credentials | M1, M2 |

### 5.8 Definition of Done

| ID | Done criterion | Source | Verify | Required evidence | Blocking? |
|---|---|---|---|---|---|
| DOD-1 | Five callers converge on one task and implementation | FR-1, FR-2, FR-3, FR-4, FR-5, FR-6, FR-10, NFR-1, NFR-2 | Test | Multi-process incident fixture and adapter receipts | yes |
| DOD-2 | Old callers obtain the existing fix; landed content proves resolution | FR-7, FR-8, UB-2, UB-6 | Test | Stale checkout and verifier fixtures | yes |
| DOD-3 | Scan and agents share admission, including empty scan | FR-9 | Test | Concurrent scan/intake fixture | yes |
| DOD-4 | Bounded failures and cancellation expose the specified result without duplicate effects | UB-1, UB-3, UB-4, UB-5, NFR-3, EVT-1 | Test | Negative fixtures and CLI goldens | yes |
| DOD-5 | Frozen evidence, existing gates and caller work remain protected | NR-1, NR-2, NR-3, §3.3, §5.3 | Test | Existing refusal suite, transport-clone proof and caller-isolation assertions | yes |
| DOD-6 | Bounded repair charter yields an accepted successor through the executor | FR-5, EVAL-1 | Eval | Retained worker trial and independent implementation review | yes |

### 5.9 AI behavior and evals

Coordination makes no model calls. The existing implementation worker performs the correction under a bounded charter.

| ID | Behavior | Eval set / harness | Threshold | Negative examples |
|---|---|---|---|---|
| EVAL-1 | Preserve the predecessor and produce a valid successor for the #1437 reproduction | One retained incident scenario through implement-now, deterministically verified | One accepted correction; zero rewrites, bypasses, recursive dispatches or unrelated changes | Injected bypass text, unavailable evidence |

## 6. Delivery

### 6.1 Phasing and milestones

Lock the contract, plan implementation, prove the incident with hermetic adapters, then exercise the configured command flow and one bounded worker trial. No calendar release promise.

### 6.2 Dependencies

Verified-successor semantics, full retained history, configured adapters and implement-now direct issue admission are prerequisites. Missing capabilities refuse explicitly; no silent provider substitution.

### 6.3 Risks, assumptions, and rabbit holes

| # | Risk / assumption / rabbit hole | Likelihood | Impact | Mitigation or test | Owner |
|---|---|---|---|---|---|
| R1 | Old verifier rejects an adopted correction | M | H | Target-pinned verification and DOD-2 | Implementer |
| R2 | Text or witness paths split one incident | M | H | Fixed identity and DOD-1 | Implementer |
| R3 | Coordinator stops after an external effect | M | H | Explicit uncertainty, no automatic retry; WR-1 | Operator |

### 6.4 Release criteria

All blocking DoD rows and required repository checks pass, including race detection. Evidence distinguishes hermetic, installed-flow, model-backed and hosted checks.

### 6.5 Rollout and reversibility

Start with explicit command invocation. Enable worker routing and the existing daily trigger after the incident proof and worker trial. Disabling these entry points stops new automatic submissions while preserving existing repair records and ordinary evidence gates. Rehearse disabled routing before release. Percentage rollout is inapplicable to this bounded repository command.

### 6.6 Launch readiness

Ship help for the supported case, update_required, tool adoption and the absence of crash recovery. Ordinary implementation review and operator merge remain required.

### 6.7 Post-launch review

After the first real repair or stale-caller encounter, record whether callers shared one task and avoided independent edits on the owning task. Any broader scope needs another operator-selected incident.

## 7. Alignment

### 7.1 FAQ

Wording does not determine ownership. A green proposal does not prove landing. Shared resolution does not prove caller readiness. Automatic crash recovery is deferred.

### 7.2 Open questions

No new product decision is required by this initial bounded draft. Independent reviews, mechanical readiness and exact-head lock approval remain outstanding workflow obligations.

### 7.3 Decision log

| Date | Decision | Alternatives considered | Why | Who |
|---|---|---|---|---|
| 2026-10-05 | Shared Go entry, ref, task and existing executor | Independent repairs | One shared operation | Operator, problem:inherited-learning-repair#A-1 |
| 2026-10-05 | Defer crash recovery | Replay and takeover | Explicit scope | Operator, problem:inherited-learning-repair#A-2 |
| 2026-10-05 | Detect an existing fix | Trust local checkout | Stale callers expected | Operator, problem:inherited-learning-repair#A-3 |
| 2026-10-05 | Only #1437's case | General framework | Observed evidence | Operator, problem:inherited-learning-repair#A-4 |
| 2026-10-05 | Exact-identity intake and bounded scan | Semantic classifier, scheduler | Smallest composition | Argus, delivery:inherited-learning-repair#E-3 |

### 7.4 Changelog

| Date | Change | Author |
|---|---|---|
| 2026-10-05 | Initial contract from the conversation | Argus, authority-contract/v1 tier-2 |

## 8. Contract readiness — the executability gate

| Check | Pass? | Notes |
|---|---|---|
| Problem accepted | yes | A-5 |
| No blocking product questions | yes | Scope and assumptions explicit |
| Must requirements have fit criteria and methods | yes | §5.1–§5.2 |
| Risks and safety plan declared | yes | §3.3, §6.3, §6.5 |
| Non-goals and non-regression named | yes | §3.4, §5.1 |
| Jobs, flows and metrics covered | yes | Requirements and EVT-1 receipts |
| DoD sourced and evidenced | yes | DOD-1 through DOD-6 specify required proof, not completed implementation |
| Independent review and mechanical readiness | pending | Evaluate the committed digest |
