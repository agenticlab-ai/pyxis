# Amendment 008 — direct issue entry

| Field | Value |
|---|---|
| **Amends** | implement-now *(chain through 007)* |
| **Status** | Approved |
| **Owner** | @mdarabi |
| **Date** | 2026-10-01 |
| **Learned-from** | operator-decision:2026-10-01-direct-issue-phase-1 |
| **Blast radius** | tier: contract-changing; touches: NG-4, FR-50..FR-55, NR-1→NR-11, UB-22, DOD-37, §0, §2.3, §3.1; frozen consumers: docs/plans/implement-now/001-plan.md T5/coverage and implement-now-authenticated-review-obligations Serves cite historical NR-1 and need legal successor/compatibility treatment before current-contract recompilation; reason: admit one explicitly selected issue without planning artifacts while preserving the worker, claims, review evidence, and activation controls |
| **Template version** | amendment-template/v2 |

Operator approval: @mdarabi, 2026-10-01 — "yes, go for coding", in response to the linked amendment and design approval request.

## Operations

### op: rescope NG-4

Reason: the operator requested a bounded issue entry using the existing trackers and executor; plan projection remains unchanged, but is no longer the only way to supply an executable charter.

| ID | Requirement (EARS) | Rationale | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|
| FR-50 | When an operator invokes implement-now for one issue on the repository's configured GitHub, Forgejo, or Carina tracker, the invoking agent shall read that issue and prepare a bounded goal, scope, acceptance criteria, and independent implementation/review requirements; the executor shall accept that intake without requiring a draft, PRD, technical design, or plan for the issue. | a small existing issue should enter the reusable worker directly | Must | high | each supported tracker admits a bounded issue with none of the planning artifacts present; unresolved product decisions or work requiring decomposition are reported before implementation spend; plan invocation remains available | Test + Demo + Inspection | G1, G3, J1 |

### op: add FR

Reason: direct entry needs an exact authorization boundary, stable contract, honest completion, and a small attribution record rather than a second workflow engine.

| ID | Requirement (EARS) | Rationale | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|
| FR-51 | When direct issue entry is selected, the executor shall verify the tracker instance, repository or Carina root, issue identity, open executable state, and every native dependency's readiness through exact reads; it shall repair, claim, implement, or review only that issue and obligations belonging to its admitted contract. Read-only exact source-comment and native-dependency facts, plus shared metadata/index reads needed to select that scope, are permitted; unrelated proposal/artifact lookups are prohibited. An unreadable or open blocker shall block entry even when outside the selected set or agent label. | filtering a label frontier can launch unrelated work or mistake an omitted blocker for completed work | Must | critical | same-number cross-provider/repository issues, pull requests, closed items, Carina rollups, missing items, open/unreadable blockers, unrelated malformed task capsules, and unrelated review obligations produce zero unauthorized launches or repairs; ready exact selection works without an agent label | Test + Failure injection | FR-50, FR-2, NR-10 |
| FR-52 | Before claiming a directly selected issue, the executor shall compile the intake through the current native task requirements codec, bind it to a canonical issue snapshot and provider-qualified identity, and persist an immutable, digest-addressed contract in shared Git evidence. Retries of the same contract shall retain one task, claim, branch, and proposal identity. An existing different native capsule or changed intake/source shall stop with an explicit conflict rather than silently replacing an admitted contract. | ordinary issue text is mutable and is not a frozen execution authority | Must | critical | restart and fresh-clone fixtures reproduce identical contract/task identities; cross-provider identity, changed title/body/cited comments/dependencies/intake, conflicting capsules, and failed persistence refuse claim; no synthetic plan or native model choice appears in portable requirements | Test + Inspection + Failure injection | FR-50, FR-51, FR-5, FR-14, NR-6 |
| FR-53 | After direct issue admission, the executor shall reuse the existing bounded implementation worker, isolated workspaces, claims, routing, verification, code-host publication, and authenticated review machinery. Direct entry shall require a configured, usable independent reviewer before implementation spend, and shall report a verified PR only when the published proposal's current revision equals both the verified local/CI revision and its authenticated successful review revision. Implementation success and pending review shall retain their existing distinct truth. | a new input should not create a weaker executor or success predicate | Must | critical | selected issue traverses the existing worker on each tracker and supported code host; disabled/unusable review fails before implementation; stale head/check/review or pending review never yields a verified-PR result; no merge occurs; existing activation controls are neither enabled nor bypassed | Test + Demo + Failure injection + Inspection | FR-50, FR-7, FR-35, FR-40..FR-49, NR-3 |
| FR-54 | For a direct issue proposal, the host shall publish a schema-versioned execution summary correlated to the admitted contract, attempts, and exact proposal revision, containing observed harness/model/effort by execution role, available token totals, attempt counts, and command-start-to-terminal elapsed time with that boundary stated. Every unavailable counter, including tool counts and intake-session usage when unobservable, shall be explicitly unavailable; unknown shall never be rendered as zero. | the host can report a useful baseline without reconstructing harness transcripts or inventing full end-to-end attribution | Must | high | GitHub and Forgejo summaries round-trip closed typed JSON and readable text; observed and unavailable counter fixtures remain distinct; launch choices match captured execution rather than declared preferences; elapsed-time boundary is explicit | Test + Inspection | FR-53, G2, J1 |
| FR-55 | When direct issue entry encounters ambiguous scope, changed source, blocked dependencies, unsupported routing evidence, exhausted implementation/review bounds, or a publication/verification failure, the executor shall preserve existing work and shared evidence, return a typed non-success result, and stop within the existing budgets. It shall not author a planning pipeline, recursively split work, watch later comments, or merge as an implicit recovery action. | bounded first delivery can grow later without hiding additional agents or authority | Must | high | failure fixtures terminate with a reason and resumable references; no unselected work, watcher, decomposition, planning artifacts, or merge action is created; unsupported activated routing evidence refuses before claim | Test + Failure injection + Inspection | FR-50..FR-54, FR-9, FR-14, NG-1 |

### op: supersede NR-1

Reason: preserve the single plan projection and shared executor while explicitly allowing a separately admitted issue charter.

| ID | Existing behavior protected | Why protected | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|
| NR-11 | The existing plan→tracker projection remains the only plan projection; direct issue admission uses the current native task codec and the same claims, worker, and provider bindings. | add an input without a parallel execution substrate or changing planning semantics | high | existing plan projection and execution fixtures remain unchanged; issue mode constructs no plan and instantiates no second worker or tracker binding | Inspection + Test | FR-1, FR-20, FR-21, FR-50..FR-53 |

### op: add UB

Reason: issue selection, input freshness, and missing review are safety failures rather than permission to broaden execution.

| ID | If… (trigger) | Then the system shall… | User sees | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|---|
| UB-22 | a direct issue reference mismatches configured authority, readiness cannot be proved, the admitted source/contract differs, or independent review cannot be verified | stop the selected run with a typed refusal or pending-review result, retain existing immutable evidence and work, and perform zero unrelated proposal/artifact lookups, repairs, claims, or model launches; permit only exact selected-source/comment/blocker facts and bounded shared metadata/index reads needed for selection | exact reason, issue/contract reference, and recovery action; never a verified-PR claim | Must | critical | negative selection, dependency, source mutation, capsule conflict, review setup, exact-head, and pending-obligation fixtures cannot widen execution or count as verified | Test + Failure injection | FR-51..FR-55 |

### op: add DOD

Reason: direct issue entry requires meaningful provider and negative-boundary proof in addition to inherited worker checks.

| ID | Done criterion | Source | Verify | Required evidence | Blocking? |
|---|---|---|---|---|---|
| DOD-37 | Direct issue entry passes every new Must fit criterion and the inherited executor/review gates without weakening plan mode or activating a previously inactive evidence plane. | FR-50..FR-55, NR-11, UB-22 | Test + Demo + Inspection + Failure injection | exact-read conformance for GitHub/Forgejo/Carina; GitHub/Forgejo publication fixtures; restart/fresh-clone identity proof; real CLI-to-worker proof; authenticated exact-revision review and CI proof; unavailable-usage golden; existing plan regression checks; explicit record of any production-readiness prerequisite still unmet | yes |

### op: supersede §2.3 Jobs and desired outcomes

Reason: the first operator job now includes one bounded issue while preserving the plan and Argus jobs.

| # | When… | I want to… | So I can… | Today's workaround | Evidence |
|---|---|---|---|---|---|
| J1 | I have a frozen projected plan, or one bounded GitHub, Forgejo, or Carina issue | turn the selected work into green, independently reviewed PRs on this machine | ship without provisioning a fleet or manually authoring planning artifacts for a small issue | stand up the external fleet, create a plan, or hand-implement | `brainstorm:implement-now#D-9`; operator decision 2026-10-01 |
| J2 | argus has driven a plan to freeze and must materialize it | execute the plan's tasks unattended, as the hands, without self-implementing | keep the principal/hands split while still delivering | drive the external fleet only | `brainstorm:implement-now#D-6` |
| J3 | a task's proposal is red (local gate or CI) | have the executor fix it to green before counting it done | trust that "done" means a mergeable proposal | manual fix, or a task falsely counted done | `brainstorm:implement-now#D-3` |

### op: supersede §3.1 Goals as falsifiable hypotheses

Reason: extend the existing delivery goal to the additional issue input rather than adding a new metrics program.

| # | We believe that… | Will result in… | Measured by… |
|---|---|---|---|
| G1 | a native local executor accepts projected plans and directly admitted bounded issues | selected work becomes green, independently reviewed PRs on the invoking machine with no fleet | M1 |
| G2 | driving each task to genuinely-green (local + CI) before counting it done | "done" means a reviewable, mergeable proposal — not a red or absent one | M2 |
| G3 | a self-contained single-task worker behind swappable dispatch | the remote/Docker fleet later reuses the worker core unchanged | M3 |
| G4 *(qualitative)* | an attended run reads as "watch my work get built" | the operator trusts and stays with the in-session loop | interview / usage verbatims |

### op: supersede §0 Read to rule — the human reviewer's section (~7 min)

Reason: refresh the informative human route for the issue-entry boundary and preserve attention to existing review and release authority.

*The body is the contract; this section only routes attention — on any conflict the body governs; machines ignore it.*

**Judge these**

| # | The call | Could have gone | If you disagree |
|---|---|---|---|
| 1 | One bounded issue can enter without planning artifacts; the invoking agent supplies portable intake | Require the whole feature pipeline for every issue | retain plan-only entry → FR-50, J1 |
| 2 | Exact provider-qualified selection and native blocker reads scope every execution/review action | Filter the ordinary label frontier | accept unrelated work or omitted blockers → FR-51, UB-22 |
| 3 | Admission freezes one Git-backed contract; changed source stops | Execute mutable tracker text | accept silent contract drift → FR-52 |
| 4 | Reuse the worker and authenticated reviewer; verified PR means exact current revision | New issue executor or green-only completion | weaken review or change implementation truth → FR-53, FR-35/FR-40..FR-49 |
| 5 | Basic observed attribution; unknown counters are unavailable and command duration has a stated boundary | Promise full intake-to-delivery telemetry now | require harness transcript instrumentation → FR-54 |
| 6 | Existing activation, human landing, and no-merge authority remain | Activate or merge as recovery | enlarge release/landing authority → FR-55, FR-26/FR-48, NR-3 |

**Sanity-check the proof** — exactly one selected issue; zero unselected repairs/claims/launches; same admitted contract retains one identity; head/local/CI/review revisions must agree; no unknown-as-zero usage (FR-51..FR-54, DOD-37).

**The author's open questions** — the concrete issue-entry design and its existing production-readiness dependencies require operator approval. Existing configured-review publication/authentication and activation requirements still govern (FR-35/FR-40..FR-49, §6.2).

**Skip the rest** — structure/fold → docgate; requirements substance → `/prd-review`; entry mechanism → technical design; implementation/review mechanics → existing executor and impl-review contracts; merge → merge-lane.
