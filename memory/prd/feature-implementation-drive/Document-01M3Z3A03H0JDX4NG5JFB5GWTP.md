# Amendment 001 — operator landings on local Git

| Field | Value |
|---|---|
| **Amends** | feature-implementation-drive *(chain through 000)* |
| **Status** | Approved |
| **Owner** | @mdarabi |
| **Date** | 2026-10-02 |
| **Learned-from** | operator-decision:https://github.com/agenticlab-ai/pyxis/pull/1312 |
| **Blast radius** | tier: contract-changing; touches: FR-7, FR-22, NR-5, DOD-1, DOD-2, DOD-3, ABUSE-2, R8, R9, §0, §1, §3.3, §3.4, §4.2, §4.3, §4.4, §5.3, §5.4, §5.5, §6.1, §6.4, §6.5, §6.6, §7.1, §8; reason: permits operator-invoked local task and final landing with exact retained evidence under the accepted shared-account limit, while retaining hosted enforcement |
| **Tracking** | https://github.com/agenticlab-ai/pyxis/issues/1337; activation and landing implementation follow in issues 1329 through 1333 |
| **Template version** | amendment-template/v2 |

**Approval state:** This is proposed merge-target content. The envelope's
Approved value takes effect only after the operator reviews and approves this
exact source in the PR and merges the independent contract proposal; no prior
approval is asserted. T13's independently approved feature-draft contract
landed in PR #1361 at `7e6d8bb52318ae05fcf26bbc94faf94327ce7fae`.

## Operations

### op: supersede FR-7

Reason: The local origin cannot authenticate a human merge actor, but each task still requires operator review and complete current task evidence before the normal landing route may update the feature branch.

| ID | Requirement (EARS) | Rationale | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|
| FR-31 | When an implementation or repair task becomes proposal-ready, the feature drive shall present a distinct proposal for operator review and landing into the feature branch through the selected code host's review route. | Keep task review and integration separate from implementation authority. | Must | critical | Hosted mode retains authenticated human review and human merge with all existing required checks. Explicit local-git mode requires a complete satisfied ReviewGate for the current task, proposal generation, head, and feature base; an operator-declared once-only receipt binds that aggregate's retained origin ref, object id, and digest. Only the separate operator route may land; it atomically publishes the reviewed task tree at the leased feature base, final proposal/evidence/archive state, and real leased proposal-branch deletion. No task or dependent advances until the retained proof and landed revision are verified on the freshly fetched feature branch. Local output discloses unauthenticated actor and shared-account limits. | Test, Demo, Inspection | G2, J2, F1 |

### op: supersede FR-22

Reason: Accepted candidate tests authorize a final proposal in both media; local landing uses the separate operator route with atomic leases instead of claiming host-enforced merge authorization.

| ID | Requirement (EARS) | Rationale | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|
| FR-32 | When all required candidate evidence is current and accepted, the feature drive shall raise one final operator-reviewed proposal from the feature branch to main without approving or landing it. | Present one complete result while keeping landing outside the drive. | Must | critical | Exactly one final proposal identifies current F, M, C and the scenario-set fingerprint. Hosted mode retains authenticated human merge and the required candidate verdict with current-base enforcement. Explicit local-git mode requires the complete accepted candidate-evidence/v1 envelope and every required scenario result, retained at an immutable origin ref and embedded in the final proposal. The declared once-only receipt binds proposal generation, head, target, F/M/C, scenario set, and evidence ref/object id/digest. The operator route atomically leases main M to a child L with tree(C), feature F to its retained child marker with tree(C), and a distinct final-proposal branch at F to deletion, together with final proposal/archive evidence. Any moved ref, absent/altered evidence, unaccepted result, or inconsistent receipt refuses landing and post-crash completion. The drive invokes zero local landing operations. | Test, Inspection | G3, J5, F2 |

### op: supersede NR-5

Reason: A solo shared-account origin supplies consistency inside the operator procedure, not authenticated human authorship or independently protected mainline.

| ID | Existing behavior protected | Why protected | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|
| NR-7 | Hosted human merge authority and required review/check/current-base enforcement remain unchanged; explicit local-git task and final landings use only the operator procedure in FR-31/FR-32 with the accepted unauthenticated shared-account limit. | The drive gains no approval or landing operation; the local route must not claim guarantees its origin cannot enforce. | critical | Hosted authorization and ruleset fixtures retain prior outcomes. Local fixtures prove generation/evidence/ref consistency and zero drive-invoked landings, while status and landing output state that the operator and agent share an OS account that can directly edit hooks and refs. No independent approval signing key, copied-origin identity, or host-level mainline protection is required. Existing signed execution ownership, task claims, e2e runner authentication, and manual-human candidate attestations retain their contracts; a local receipt cannot stand in for them. No implicit local fallback or re-enabling of Pyxis source's disabled legacy document checks occurs. | Test, Inspection | FR-31, FR-32, FR-18 |

### op: supersede DOD-1

Reason: The release criterion must include the replacement task and final landing obligations.

| ID | Done criterion | Source | Verify | Required evidence | Blocking? |
|---|---|---|---|---|---|
| DOD-9 | All active Must functional and failure requirements pass their fit criteria in the applicable selected code-host mode. | FR-1..FR-6, FR-8..FR-21, FR-23..FR-32, UB-1..UB-11 | Test, Demo, Inspection | conformance and acceptance-suite report, including local retained-evidence and atomic-landing fixtures | yes |

### op: supersede DOD-2

Reason: Hosted non-regression and the explicit local exception must be verified together.

| ID | Done criterion | Source | Verify | Required evidence | Blocking? |
|---|---|---|---|---|---|
| DOD-10 | Existing executor, task-claim, generic-pool, signed ownership, e2e-authoring/attribution, and mainline-isolation behavior remains intact; hosted enforcement is unchanged and local code-host selection is explicit. | NR-1..NR-4, NR-6, NR-7 | Test, Inspection | non-regression suite, composition inspection, and local trust-limit disclosure | yes |

### op: supersede DOD-3

Reason: Local execution ownership is independent of code-host selection; the new local code host needs its own evidence in addition to the original ownership fixture.

| ID | Done criterion | Source | Verify | Required evidence | Blocking? |
|---|---|---|---|---|---|
| DOD-11 | The local-owned six-task dogfood completes through reviewed task proposals, seeded feedback, seeded e2e failure/repair, accepted candidate, and verified delivery with no guardrail breach. The explicit local-git binding additionally proves operator task/final landing, parallel roots, origin-retained proof after clone/process loss, and all-or-nothing ref leases. | M1–M4, §3.3, FR-1..FR-6, FR-8..FR-21, FR-23, FR-24, FR-31, FR-32, NR-7 | Demo, Analysis, Test | drive journal, proposal/evidence refs, metric rollup, local transaction and fresh-clone transcripts | yes |

### op: add DOD

Reason: Plan T14 requires operator source review and an independent contract merge before any T6 local landing capability can activate.

| ID | Done criterion | Source | Verify | Required evidence | Blocking? |
|---|---|---|---|---|---|
| DOD-12 | The operator reviews the precise amendment and successor source and its contract-only PR merges independently before local task or final landing is enabled. | FR-31, FR-32, NR-7; [local Git D-5](../../../memory/design/local-git-codehost/Decision-01M3N1MMJP92P3D7Q6C1CNBXN9.json); [local Git D-7](../../../memory/design/local-git-codehost/Decision-01M3N1MMJR02WDP45HVH4HS9Q1.json); issue:1337 | Human judgment — @mdarabi, rubric: exact source approves the operator route, current retained evidence, shared-account limit, and unchanged hosted enforcement; Inspection | source approval record and independently merged contract revision cited by enabling code | yes |

### op: supersede ABUSE-2

Reason: Replace the hosted-only abuse mitigation with the scoped local operator boundary while preserving typed supersession lineage.

| ID | Actor and intent | Vector | Impact | Severity | Mitigated by | Verify |
|---|---|---|---|---|---|---|
| ABUSE-6 | Drive bypasses review/progression | lands or marks complete from proposal/receipt alone | premature integration | critical | FR-31, FR-32, FR-10, NR-7 inside normal route; direct shared-account writes remain outside its guarantee | composition, retained-proof, and wrong-base fixtures |

### op: add R

Reason: Add the local trust and retained-proof risks through the typed row operation; existing risk rows remain unchanged.

| # | Risk / assumption / rabbit hole | Likelihood | Impact | Mitigation or test | Owner |
|---|---|---|---|---|---|
| R8 | A shared-account receipt is mistaken for authenticated human approval or protected mainline. | H | H | accepted explicit local limit; procedural operator boundary; no identity/protection claim; hosted unchanged | operator/code-host design |
| R9 | Missing retained proof or a failed lease is treated as recoverable success. | M | H | verify exact canonical proof before landing and post-crash progress; atomic rejection and idempotent result observation | local code-host design |

### op: supersede §0

Reason: Make this section consistent with the scoped local operator route, current retained evidence, and unchanged hosted/adjacent contracts.

*The body is the contract; this section only routes attention. Machines ignore it.*

**Judge these**

| # | The call | Could have gone | If you disagree |
|---|---|---|---|
| 1 | Tasks integrate on a feature branch before one final proposal to main | Task proposals land directly on main | Change isolation → FR-3, FR-15 |
| 2 | Hosted merges remain authenticated; explicit local-git landings follow a declared operator procedure with retained exact evidence | Independent signing key and protected local host | Change the accepted trust limit → FR-31, FR-32, NR-7, §5.3 |
| 3 | The human explicitly assigns exclusive local or remote execution ownership | Timeout assignment or racing executors | Change routing authority → FR-1, FR-2, FR-11 |
| 4 | Terminal e2e verifies exact feature and current-main inputs; manual routes retain human attribution | Feature-head-only tests or receipt as manual attestation | Change verification → FR-16..FR-19 |
| 5 | The drive requests and observes proposals; the operator invokes landing | Drive automatically lands green proposals | Change the command boundary → FR-31, FR-32, NR-7 |
| 6 | Only verified delivered revision, candidate tree, and current-main ancestry complete the feature | Complete on proposal or receipt | Change delivery proof → FR-23, FR-24 |

**Sanity-check the numbers** — both execution-owner dogfoods still use a six-task DAG; zero invalid dependent pickups; every required scenario has accepted current evidence. Local task and final evidence survives a fresh clone; each receipt nonce is consumed once; any leased-ref movement rejects the entire landing transaction.

**Open review** — exact T14 source approval remains pending. This amendment enables no runtime path; its independent merge precedes T6.

**Skip the rest** — substantive contract review and later adapter acceptance establish these claims. The approved Pyxis source legacy-document-gate bypass remains in force.

### op: supersede §1

Reason: Make this section consistent with the scoped local operator route, current retained evidence, and unchanged hosted/adjacent contracts.

- **Problem** — Task execution alone cannot carry a dependency graph through reviewed integration, current-main candidate tests, repair, and verified delivery. The original hosted merge contract also cannot describe the accepted local bare origin honestly.
- **For whom** — A human engineer driving one feature, with an autonomous driver requesting and observing reviewable work. Standalone tasks remain generic executor work.
- **Direction** — Keep one durable drive, exclusive execution owner, feature-blind task executor, feature integration branch, candidate-evidence contract, and delivery proof. Hosted proposals retain authenticated human review/merge and enforced checks. Explicit local-git proposals use operator-declared receipts and an operator-invoked atomic landing route with retained current task or candidate evidence.
- **Appetite** — One repository and feature; local and remote execution ownership remain separate from code-host selection. No automatic merges, takeover, e2e redesign, or shared worker worktree.
- **Success** — Both execution-owner dogfoods satisfy M1–M4 and the selected medium's review route; the local binding also proves parallel-root integration, leased atomic updates, and recovery with origin-retained proof.
- **Guardrail** — The drive performs no approval or landing action. No dependent unlocks or delivered transition occurs from receipt, badge, or local cache alone. Every required scenario remains current and accepted.
- **Accepted local limit** — The operator and agent share an OS account able to edit hooks and refs. The normal route verifies consistency; it cannot authenticate human authorship or prevent direct outside-route Git writes. No independent signing key or protected host is required.
- **Contract order** — Exact source review and an independent T14 merge precede enabling code. Pyxis source's approved disabled legacy document checks remain disabled; applicable installed checks and frozen-document authoring rules retain their existing scope.

### op: supersede §3.3

Reason: Make this section consistent with the scoped local operator route, current retained evidence, and unchanged hosted/adjacent contracts.

| Guardrail metric | Threshold | Action if breached |
|---|---|---|
| Cross-owner implementation attempts | any attempt in a domain other than the current recorded owner, or any overlapping handoff | stop the drive, preserve artifacts, require human recovery |
| Premature DAG advancement | any dependent becomes executable before every blocker is verified landed on the feature branch | stop routing and fail the release fixture |
| Stale verification authorization | any final proposal raised after feature head or main head changed from accepted evidence | close/draft-block the proposal, invalidate evidence, rerun verification |
| Unauthorized merge | any task or final proposal merged by the drive | disable the drive and escalate as a factory-control defect |

**Ship rule (pre-registered):** both dogfood modes satisfy M1–M4, the seeded e2e failure is repaired through a reviewed PR, and no guardrail is breached; otherwise do not ship.

These guardrails apply to the drive and selected landing route. In explicit local-git mode an operator-declared receipt is not evidence of authenticated human authorship, and direct writes made through the shared OS account can bypass the route. The route still refuses inconsistent generation, missing or altered retained proof, unsatisfied current ReviewGate, unaccepted candidate result, and moved target/proposal refs. Hosted enforcement and manual-human e2e attribution are unchanged (NR-7).

### op: supersede §3.4

Reason: Make this section consistent with the scoped local operator route, current retained evidence, and unchanged hosted/adjacent contracts.

| ID | Non-goal | Why refused |
|---|---|---|
| NG-1 | Automatically merge task proposals | Hosted human review/merge and the local operator procedure remain the selected review boundary (FR-31, NR-7). |
| NG-2 | Send task proposals directly to main | The feature branch is the integration and isolation boundary. |
| NG-3 | Share one literal worker worktree | Workers stay isolated; committed state is reproducible and portable. |
| NG-4 | Redesign the e2e authoring line | This feature consumes its reviewed design and ordered plan. |
| NG-5 | Orchestrate generic standalone issues | Generic work remains in the ordinary executor pool. |
| NG-6 | Automatically take over stale local ownership | Recovery and handoff are explicit human acts. |
| NG-7 | Automatically merge the final feature proposal | Final acceptance and landing remain operator acts through the selected review route (FR-32, NR-7). |
| NG-8 | Coordinate multi-repository feature DAGs | V1 is one feature in one repository. |
| NG-9 | Optimize fleet-wide fairness and capacity | That is umbrella fleet policy, not feature lifecycle behavior. |
| NG-10 | Build a unified feature dashboard | Native action surfaces plus minimal recovery diagnostics suffice for v1. |

| ID | Waiting room | Revisit when / intended version |
|---|---|---|
| WR-1 | Automatic low-risk task-PR admission | A future autonomy-level policy explicitly grants it and evidence shows human review can graduate safely. |
| WR-2 | Automatic stale-owner failover | A recovery policy can prove no split brain and the operator authorizes automatic transfer. |
| WR-3 | Multi-repository feature drives | The single-repository lifecycle is stable and a real cross-repository contract exists. |
| WR-4 | Unified feature progress surface | Native surfaces produce repeated, measured navigation or recovery failures. |
| WR-5 | Fleet-wide scheduling and fairness | Multiple remote feature drives demonstrate starvation or capacity contention beyond existing fleet policy. |

### op: supersede §4.2

Reason: Make this section consistent with the scoped local operator route, current retained evidence, and unchanged hosted/adjacent contracts.

- **In** — feature resolution; durable origin feature branch; exclusive local/remote execution ownership and explicit quiesced handoff; resumable sweeps; feature-blind assignment; distinct task/repair proposals; same-proposal feedback; verified tracker/DAG progression; canonical candidate e2e with existing manual-human attestations; repair/full rerun; final proposal; verified main delivery; cancellation/resume/abandonment and ordered cleanup.
- **Code-host routes** — hosted authenticated operator review/merge with required checks and current-base candidate enforcement; explicit local-git operator-declared review and separately invoked atomic landing, complete current origin-retained evidence, once-only receipts, and shared-account disclosure. Code-host selection does not assign execution ownership.
- **Future** — risk-governed automatic admission, automatic stale-owner recovery, multi-repository drives, unified status surface, fleet fairness.
- **Out** — drive approval/landing, direct-to-main task proposals, shared worker worktree, e2e-authoring redesign, implicit timeout routing, authenticated local landing actor, independent copied-origin identity, and host-level local mainline protection.

### op: supersede §4.3

Reason: Make this section consistent with the scoped local operator route, current retained evidence, and unchanged hosted/adjacent contracts.

**Flow F1 — Kick off and integrate (serves J1, J2)**

| Step | User does | System responds | States covered | Design |
|---|---|---|---|---|
| 1 | explicitly selects local or remote execution for a frozen feature | records exclusive owner and feature-branch target, or refuses conflict | ideal, conflict, invalid feature | design:feature-implementation-drive-local-git |
| 2 | reviews a distinct current task/repair proposal | keeps dependents blocked; same-proposal feedback invalidates old head-bound evidence | awaiting review, changes requested, approved | selected review surface |
| 3 | merges on the hosted surface, or invokes the local operator landing command | verifies current review evidence and landed revision on the feature branch before RecordLanding and new routing | landing requested, landed-unverified, partial/complete DAG, failure | design:feature-implementation-drive-local-git#3.6 |

Local task landing requires the complete satisfied ReviewGate for the current proposal generation/head/base, including the exact code gate and every required green check. Its immutable origin aggregate and declared receipt are verified before the atomic leased transaction. Concurrent root proposals at the same old base cannot both land: the second rebases, reruns the complete aggregate, and obtains a fresh receipt. Review waits persist and exit with zero live workers/model sessions. Errors preserve artifacts and block progression; status names the remedy and local trust limit. CLI/action-surface accessibility and English-only operator text remain unchanged.

**Flow F2 — Verify, repair, and deliver (serves J3–J5)**

| Step | User does | System responds | States covered | Design |
|---|---|---|---|---|
| 1 | completes all task reviews and verified landings | constructs candidate C from exact feature F and main M without moving either branch | ready, conflict, stale input | retained candidate builder |
| 2 | supplies any required manual-human result | retains the existing authenticated, candidate-bound attestation contract; blocks on every non-accepted route | running, manual pending, failed, accepted | existing e2e evidence surface |
| 3 | reviews and lands repair proposals | rebuilds candidate and reruns the entire required scenario set | repair pending, re-verifying | selected task review route |
| 4 | reviews final proposal and invokes its selected hosted/local landing route | checks accepted current proof; verifies L's accepted base, candidate tree, and current-main ancestry before delivery, release, and cleanup | final review, feedback, landed-unverified, delivered | design:feature-implementation-drive-local-git#3.9 |

Local final landing atomically leases main, feature, the distinct final-proposal branch, and proposal/archive state. Any moved input or absent/altered retained evidence refuses all changes. The local receipt is approval metadata, never a replacement for a required scenario result or manual-human attestation. Neither an open final proposal nor an unverified landed record completes the feature.

### op: supersede §4.4

Reason: Make this section consistent with the scoped local operator route, current retained evidence, and unchanged hosted/adjacent contracts.

**Execution ownership with hosted review.** The local-owned and remote-owned six-task fixtures retain the same task review, feedback, integrated failure/repair, current candidate, and authenticated human final merge contract. Local ownership does not imply a local code host.

**Explicit local-git common case.** The operator selects local-git against an initialized bare origin with mainline. Argus presents two root proposals. After current ReviewGate evidence and operator-declared review, the operator lands the first. The second's old-base lease refuses all updates; it rebases, reruns the complete ReviewGate, obtains a fresh receipt, and lands without losing the first root. Dependents unlock only from verified landed revisions. All tasks land, every required scenario on C is accepted, and the operator reviews then lands one final proposal with exact retained candidate proof and atomic ref leases. Delivery is verified at L on main.

**Recovery and stale inputs.** Losing the review worktree or restarting cannot replace origin-retained evidence with a green badge. A fresh clone verifies canonical retained and embedded envelopes. Changed feature/main/proposal generation or missing proof blocks landing and progress; candidate input changes require reconstruction and a full rerun. Retry of an already committed exact transaction returns the same L without another target update.

**Solution review for this amendment** — pending exact T14 source approval and its independently merged contract-only PR. Runtime acceptance is not claimed complete.

### op: supersede §5.3

Reason: Make this section consistent with the scoped local operator route, current retained evidence, and unchanged hosted/adjacent contracts.

| Function / data | Human operator | Current execution owner / Argus drive | Non-owner executor | Unauthenticated |
|---|---|---|---|---|
| Select execution ownership; handoff/cancel/abandon | existing authenticated authorization | read/checkpoint | none | none |
| Execute/repair feature tasks | observe/review | assigned execution | none | none |
| Request/observe task or final proposals | review | propose/update/observe | none | none |
| Hosted approval/merge | authenticated review/merge | no approval or merge operation | none | none |
| Local declared approval and landing | separate operator procedure | request/observe only; no landing operation in drive composition | none through supported route | no authenticated local actor proof |
| Manual-human e2e attestation | existing authenticated, candidate-bound attestation | record/verify only | none | rejected by existing e2e contract |

The local operator command boundary is procedural: the shared OS account can invoke commands, forge declarations, or write refs directly. Receipt validity proves consistency of the normal route, not who ran it. The drive cannot call landing through its supported ports, but this is not an OS-level capability barrier (NR-7).

| ID | Actor and intent | Vector | Impact | Severity | Mitigated by | Verify |
|---|---|---|---|---|---|---|
| ABUSE-1 | Remote executor tries to steal a local-owned feature | claims a newly ready issue before local resume | locality breach and split feature history | critical | FR-2, FR-4, FR-11, NFR-1 | concurrent ownership drill |
| ABUSE-6 *(001 ← ABUSE-2)* | Drive bypasses review/progression | lands or marks complete from proposal/receipt alone | premature integration | critical | FR-31, FR-32, FR-10, NR-7 inside normal route; direct shared-account writes remain outside its guarantee | composition, retained-proof, and wrong-base fixtures |
| ABUSE-3 | Actor replays passing e2e evidence for another revision | copies verdict without current input identities | unverified feature reaches main | critical | FR-16–FR-19, NFR-2 | replay/race corpus |
| ABUSE-4 | Stale owner resumes after handoff | uses old local state or task ref | overlapping implementation | critical | FR-11–FR-13, NFR-1 | stale-owner recovery drill |
| ABUSE-5 | Machine submits a human-route attestation | forges or omits acting identity | manual requirement falsely passes | high | FR-18, NFR-2 | attribution-negative tests |

### op: supersede §5.4

Reason: Make this section consistent with the scoped local operator route, current retained evidence, and unchanged hosted/adjacent contracts.

| Entity / field | Meaning | Source of truth | Retention | Integrity rules | Privacy class |
|---|---|---|---|---|---|
| Feature drive identity | stable join from frozen feature plan to lifecycle state | configured work tracker / drive state adapter | through delivery or explicit abandonment; audit according to repo policy | one active drive per feature/repository | internal |
| Execution ownership | domain, registered machine when local, state, transition evidence | shared ownership store adapter | active through verified delivery/cancel; transition history retained by policy | exclusive; holder/transition authenticated; fail closed on ambiguity | internal |
| Feature integration ref | durable committed integration state | code host / git adapter | until verified delivery plus branch-retention policy | target and head identities explicit; never inferred from checkout state | internal |
| Task/proposal state | charter, blockers, review/head/landing evidence | work tracker + code host adapters | existing medium policy | completion only after verified feature-branch landing | internal |
| Candidate evidence | feature head, main head, candidate identity, scenario outcomes, runner/actor identity | e2e evidence adapter | through delivery/abandonment plus audit policy | immutable per candidate; any input change creates new evidence | internal |
| Delivery proof | final proposal and mainline ancestry result | code host adapter | existing proposal/audit policy | goal-object verified before completion | internal |

- **Migration** — none; feature drives begin only for newly kicked-off frozen features in v1. Existing in-flight task pools remain on their current path.
- **Parallel-run / cutover** — generic executor behavior remains available in parallel; a feature enters the new path only through explicit kickoff.
- **Rollback** — disable new feature-drive kickoff, preserve all durable branches/proposals/evidence, and return unstarted work to its prior explicit routing only by human action.
- **Decommission** — remove the drive only after no active ownership remains and every retained artifact has an explicit disposition.
- **Privacy check** — no DPIA trigger identified: no personal/sensitive data, profiling, monitoring of people, or new external data use.
- **Egress** — code/task/e2e content follows the configured executor, code-host, tracker, and runner adapters already selected by the repository; this feature adds no destination and records runner identity so operators can inspect where verification ran.

- **Local review proof** — complete canonical task ReviewGate or candidate-evidence/v1 plus every required result is retained on a create-only origin evidence ref. The request and receipt bind ref, full object id, digest, and proposal generation; final proposal state embeds identical bytes and pins that ref. Both retained and embedded proof are verified before landing and post-crash progression. Proposal lineage, receipt, evidence, and archive records remain reachable after real proposal-branch deletion; local cache/logs are not their source of truth. Cleanup cannot remove proof still needed for RecordLanding or delivery recovery.
- **Local identity limit** — declared receipt actor is unauthenticated metadata; it does not change authenticated execution ownership, runner identity, or manual-human attestation requirements. Independent copied-origin identity is not supplied.

### op: supersede §5.5

Reason: Make this section consistent with the scoped local operator route, current retained evidence, and unchanged hosted/adjacent contracts.

| Interface | Direction | Purpose | Contract (format, ranges, timing) | Failure modes | Owner |
|---|---|---|---|---|---|
| Frozen feature-plan resolver | in | resolve feature identity and exact task DAG | one feature/repository; rejects ungated/non-frozen input | missing, ambiguous, stale projection | planning adapter owner |
| Feature ownership store | both | exclusive assignment, handoff, release, cancellation | durable cross-machine state; one owner; transition evidence | conflict, stale owner, malformed state | feature-drive design |
| Work tracker | both | read DAG/readiness; record completion/repair work | existing task and blocked-by dialect; state advances after verified landing | non-default-base closure mismatch, stale read | tracker adapter owner |
| Code host / git | both | feature refs, task/final proposals, review and ancestry evidence | revision-pinned proposals/reviews; remote branch required | conflict, wrong base, head race, auth failure | code-host adapter owner |
| implement-now executor | out/in | implement one ready frontier in selected domain | existing task/label/base/runtime policies; feature-blind worker | claim loss, worker failure, red proposal | executor owner |
| E2e design/plan/evidence | in/both | execute required scenarios and decide final eligibility | reviewed design + ordered tasks; outcomes bound to candidate and runner/actor | unavailable route, stale/replayed evidence | e2e-verification owner |
| Notification/action surfaces | out | surface human review, manual scenario, ownership, and recovery actions | existing medium-native notifications; instruction-free wake content | lost signal, duplicate signal | trigger/medium adapter owner |

Explicit local-git replaces only code-host proposal/review/landing transport. Its operator command is outside the drive/executor composition. The drive can request and observe local proposal state; no port lets it invoke approval or landing. Evidence publication, ref leases, and atomic-push capability are deterministic normal-route prerequisites. A local adapter with missing capabilities refuses activation or landing; it never silently falls back to hosted transport or a non-atomic write sequence.

### op: supersede §6.1

Reason: Make this section consistent with the scoped local operator route, current retained evidence, and unchanged hosted/adjacent contracts.

| Phase | Purpose | Scope | Exit criterion |
|---|---|---|---|
| Contract — T14 | authorize the local exception before code | precise amendment and successor source; independent contract-only PR after T13 | DOD-12; no local activation in this phase |
| A — deterministic lifecycle | retain state, ownership, and executor boundaries | existing aggregate/ownership/race/cancellation conformance | NFR-1..NFR-4 and UB-1..UB-5 green |
| B — hosted local-owned dogfood | retain hosted review, integration, candidate tests, repair, and delivery | hosted adapters and local execution affinity | applicable DOD-11 and DOD-5 paths green |
| C — remote-owned parity | retain equivalent lifecycle under separate execution owner | remote binding, handoff, runner routing | DOD-4 and all remaining applicable blocking rows green |
| Local binding — T6..T10 | implement the independently approved local route | distinct operator boundary; retained task/candidate proof; atomic leases and recovery | FR-31/FR-32, NR-7, DOD-9..DOD-12 and local successor acceptance green before activation |

### op: supersede §6.4

Reason: Make this section consistent with the scoped local operator route, current retained evidence, and unchanged hosted/adjacent contracts.

| Lens | Criterion | Evaluated by |
|---|---|---|
| Functionality | All active Must FRs and UBs and every active blocking DoD pass for the applicable selected medium: FR-1..FR-6, FR-8..FR-21, FR-23..FR-32; UB-1..UB-11; DOD-4..DOD-12. No retired row stands in for its replacement. | acceptance, conformance, and exact source/contract-merge evidence |
| Compatibility | NR-1..NR-4, NR-6, NR-7 pass; hosted review/check/current-base enforcement and existing manual-human attestation remain unchanged. | regression and composition inspection |
| Usability | Operator can review exact proposals, retained evidence, target refs and local trust limits, then invoke landing/recovery through documented action surfaces. | dogfood observation against J1–J5 |
| Reliability | Race, restart, evidence, parallel-root, and ref-lease fixtures produce zero premature advancement, stale authorization inside the normal route, partial transaction, or evidence loss. | failure injection and fresh-clone drills |
| Performance/cost | Review waits hold zero model/Worker processes and deterministic transitions consume zero inference. | process and token attribution |
| Supportability | Status names exact phase, owner, blocker, retained evidence, next action, and the selected medium's guarantee. | recovery drill |

### op: supersede §6.5

Reason: Make this section consistent with the scoped local operator route, current retained evidence, and unchanged hosted/adjacent contracts.

- **Flag** — feature-drive kickoff is an explicit per-feature choice; generic execution remains unchanged and is the fallback for unstarted work.
- **Kill switch** — disable new feature-drive kickoff and new task routing; preserve active artifacts and require explicit cancel/handoff/complete disposition.
- **Experiment** — none; deterministic dogfood fixtures and real local/remote drives are the evidence.
- **Cutover/migration steps** — no migration of in-flight work; only newly kicked-off frozen features enter the drive.

| Stage | Audience | Minimum evidence | Advance when | Roll back when |
|---|---|---|---|---|
| Hermetic | maintainers | all Phase-A race, failure, and conformance fixtures | deterministic contracts and guardrails green | any split ownership, destructive path, or worker-boundary breach |
| Local dogfood | @mdarabi | one full six-task local-owned drive including feedback and repair | DOD-11 and local failure drills green | any §3.3 breach or hidden manual machinery step |
| Remote dogfood | @mdarabi + configured remote engine | equivalent remote-owned drive and explicit handoff drill | ship rule and all DOD rows green | semantic mismatch, foreign execution, stale evidence, or unbounded idle cost |

Local-git is an explicit code-host selection, independent of local/remote execution ownership. T13 and this T14 contract land independently before local landing code activates. Validate initialized origin/mainline, atomic-push support, retained proof, and required gates before selecting it. Unsupported capability refuses; hosted mode remains the default and has no local fallback. Rollback preserves proposal/receipt/evidence/archive refs and already landed revisions. This amendment does not restore the approved Pyxis source legacy document gate; installed checks apply only under their existing pipeline scope.

### op: supersede §6.6

Reason: Make this section consistent with the scoped local operator route, current retained evidence, and unchanged hosted/adjacent contracts.

| Area | Item | Owner | Status |
|---|---|---|---|
| Support | recovery states and explicit cancel/handoff/diagnostic guidance | @mdarabi | ☐ |
| Docs | command and lifecycle documentation, including human review and evidence semantics | @mdarabi | ☐ |
| Enablement | n/a — internal factory capability; operator dogfood is the enablement path | — | n/a |
| Legal & privacy | confirm no new egress destination or personal-data processing | @mdarabi | ☐ |
| Pricing & billing | n/a — no product packaging or billing surface | — | n/a |
| Marketing & comms | n/a — no external launch surface in v1 | — | n/a |
| Platform | adapter conformance and versioning notes | @mdarabi | ☐ |
| Localization | n/a — operator CLI and machine-state surface only | — | n/a |

**Decision:** Launch gate — pending §6.4 evidence, checked applicable readiness rows, and operator approval.

| Reviewer | Role | Date | Verdict |
|---|---|---|---|
| @mdarabi | operator/product/engineering | — | pending |

Local activation additionally needs DOD-12's exact source approval and independent contract merge, implemented successor acceptance, operator-procedure documentation with the shared-account disclosure, and retained-evidence/atomic-push conformance. All are launch evidence to be supplied later; this Draft records none as completed.

### op: supersede §7.1

Reason: Make this section consistent with the scoped local operator route, current retained evidence, and unchanged hosted/adjacent contracts.

- Each implementation and repair task still has a distinct reviewable proposal to the feature branch. Hosted review and merge are authenticated; local review and landing follow the separately invoked operator procedure.
- Local or remote execution is chosen explicitly and remains exclusive until a safe handoff. This choice is independent of GitHub or local-git code-host selection.
- Review waits checkpoint and exit; the selected execution owner resumes without a live terminal or Worker.
- Failed e2e creates reviewed repair work followed by a full new candidate run. A local receipt cannot approve a manual-human scenario.
- Main stays outside task integration. Final delivery requires exact accepted candidate evidence and verified L on current main, not a receipt or merged badge.
- Shared-account local mode does not authenticate the person issuing a receipt or stop direct writes to hooks/refs. Its normal route still checks proposal generation, current head/base, complete retained proof, once-only nonce, and atomic leases. A stronger host or independent signing key is outside this accepted scope.
- Frozen document features keep their existing evolution rules. Pyxis source's approved legacy-check bypass remains in force; this local amendment does not introduce a new document-gate ceremony.

### op: supersede §8

Reason: Make this section consistent with the scoped local operator route, current retained evidence, and unchanged hosted/adjacent contracts.

| Check | Pass? | Notes |
|---|---|---|
| Exact local source approved | ☐ | T14 source review pending; no runtime activation is authorized by Draft status |
| Independent contract merged before enabling local landing | ☐ | DOD-12 precedes T6 |
| Zero solution-blocking TBDs | ☑ | local trust limit accepted in the locked local-git contract; retained mechanics are specified by the successor; existing retention-policy question remains launch-owned |
| All active Must FRs have fit criteria and verification | ☑ | FR-1..FR-6, FR-8..FR-21, FR-23..FR-32 |
| Risks and permission boundaries explicit | ☑ | critical landing/evidence predicates, operator procedure, hosted enforcement, shared-account disclosure |
| Non-goals and non-regression named | ☑ | NG-1..NG-10; NR-1..NR-4, NR-6, NR-7 |
| Jobs, flows, metrics, and instrumentation retained | ☑ | J1–J5, F1/F2, M1–M4, EVT-1..EVT-8 |
| Every active blocking DoD sourced and evidenced | ☑ | DOD-4..DOD-12; required evidence is specified, not claimed complete |
| Human judgment has owner and rubric | ☑ | @mdarabi; retained DOD-8 and exact local source/contract DOD-12 |
| Existing e2e attribution and signed ownership unaffected | ☑ | FR-18, NFR-2, NR-7; local approval receipt is not those proofs |
| No implicit local fallback or disabled-gate restoration | ☑ | NR-7, §6.5 |
