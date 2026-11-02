# Amendment 007 — reciprocal review publication terminal

| Field | Value |
|---|---|
| **Amends** | implement-now *(chain through 006)* |
| **Status** | Approved |
| **Owner** | @mdarabi |
| **Date** | 2026-08-20 |
| **Learned-from** | review-thread:pr-1092 |
| **Blast radius** | tier: contract-changing; touches: FR-49, EVT-17, DOD-35→DOD-36, §0, §3.2, §3.3, §6.2; reason: define implement-now's reciprocal terminal for the merged impl-review publication contract, make its seven-field join and receipt/proof distinction evidentiary, place it inside the existing metric and release guard, and record that amendment 004 satisfies the sibling contract dependency while its successor design remains required |
| **Tracking** | issue #1029 |
| **Template version** | amendment-template/v2 |

## Operations

### op: add FR

Reason: the merged impl-review contract requires a separately contracted host terminal for the same publication moment, but implement-now currently exposes only a broader lifecycle terminal whose optional, differently named fields cannot prove that join.

| ID | Requirement (EARS) | Rationale | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|
| FR-49 | When an impl-review-owned host persistence adapter emits `prd:impl-review/EVT-13` for a completed-report publication attempt initiated from an implement-now review obligation, the executor shall append exactly one EVT-17 for the same publication moment. The terminal statuses and, when failed, failure classes shall agree, and the terminals shall carry these field-identical pairs with the same grammar and value: `review_process_id = review_attempt_id`, `review_obligation_id = review_obligation_id`, `proposal_id = proposal_ref`, `reviewed_revision = revision`, `artifact_ref = artifact_ref`, `native_receipt = native_receipt`, and `attempt_proof_ref = attempt_proof_ref`. The first four pairs shall be present on every paired terminal; the last three shall have identical presence, and all three shall be present when status is `verified`. `publication_attempt_id` shall remain impl-review's distinct child identity with no implement-now alias. EVT-15 shall remain the broader review-lifecycle terminal: its `artifact_proof_ref`, when present, shall equal EVT-17 `attempt_proof_ref`, shall never carry a native receipt, and shall not substitute for EVT-17 in cross-contract evidence. A missing, duplicate, orphaned, status/failure-class disagreement, field mismatch, or receipt/proof-overloaded pair shall remain non-evidentiary, preserve the review obligation as pending, and take FR-42's infrastructure class for absent/append failure or artifact-untrusted class for conflicting/substituted evidence. | exact field identity is the only reliable proof that impl-review's artifact publication and implement-now's review outcome describe one real reviewer attempt rather than two similar records | Must | critical | verified and each failed-class fixture produce exactly one EVT-13↔EVT-17 pair with the same status, the same failure class when failed, the four required identities, and equal optional-field presence/value; every cross-class mismatch while status and all seven pairs match, plus missing, duplicate, orphaned, status/grammar/value mismatch, optional-presence mismatch, receipt/proof overload, and EVT-15-only fixtures never count toward M7 or clear pending and emit the required FR-42 class; verified pairs carry separate native receipt and attempt proof; `publication_attempt_id` never appears as an obligation or attempt alias | Test + Analysis + Failure injection + Inspection | G2, M7, FR-35, FR-41, FR-42, prd:impl-review/FR-35, prd:impl-review/EVT-13 |

### op: add EVT

Reason: implement-now needs one terminal dedicated to the exact publication moment so the sibling event can be joined by contract rather than inferred from the broader review lifecycle.

| ID | Event name | Fires when | Properties (typed) | Source | Feeds |
|---|---|---|---|---|---|
| EVT-17 | `hosted_review_publication_terminal` | the executor records the same completed-report publication terminal emitted as `prd:impl-review/EVT-13` | `review_attempt_id:string`, `review_obligation_id:string`, `proposal_ref:string`, `revision:string`, `status:enum{verified,failed}`, `artifact_ref:string?`, `native_receipt:string?`, `attempt_proof_ref:string?`, `failure_class:enum{artifact_unverified,artifact_untrusted,infrastructure}?`, `reason:string?`, `escalated:boolean`; the first four identity fields are always required; `verified` requires all three distinct artifact/receipt/proof fields and forbids failure fields; `failed` requires failure fields, the same failure class, and exact optional-field presence from EVT-13; every field pair and grammar is governed by FR-49 | executor evidence adapter | M7, FR-49, DOD-36, cross-contract publication audit |

### op: supersede DOD-35

Reason: the blanket live-Must proof must include the reciprocal publication-terminal contract and its adversarial join matrix.

| ID | Done criterion | Source | Verify | Required evidence | Blocking? |
|---|---|---|---|---|---|
| DOD-36 | Every live Must functional requirement passes its fit criterion, including field-identical plan transport, least-cost satisfying routing, immutable implementation evidence, exact-green durable review obligations, authenticated exact-artifact completion, the exact EVT-13↔EVT-17 reciprocal publication join with distinct native-receipt and attempt-proof fields, three-way failure significance, isolated bounded reviewers, selected-frontier review-only recovery, combined-frontier zero work, and direct activation. | FR-1..FR-5, FR-7..FR-10, FR-13..FR-18, FR-20..FR-26, FR-28..FR-32, FR-35, FR-40..FR-49 | Test + Demo + Inspection + Analysis + Failure injection + Security review | per-requirement evidence index including exact/promoted/unsupported/legacy/cutover, ordering/revision, setup/auth, assurance-band routing, attempt-attribution/receipt-substitution, EVT-13↔EVT-17 required/optional/status/failure-class/grammar/value joins, receipt-versus-proof non-alias, missing/duplicate/orphan/mismatch refusal, isolation/concurrency, failure-class, combined-empty, selected-frontier/driver-exhaustion, direct-cutover, fallback, and unreachable-hardcoded-path cases | yes |

### op: supersede §0 Read to rule — the human reviewer's section (~6 min)

Reason: the human routing section must name the reciprocal publication terminal and the now-available sibling contract while keeping the remaining design dependency explicit.

*The body is the contract; this section only routes attention — on any conflict the body governs; machines ignore it.*

**Judge these**

| # | The call | Could have gone | If you disagree |
|---|---|---|---|
| 1 | Opens PRs, **never merges** | Drive to merged | Merge authority collision → §3.4 NG-1, FR-7 |
| 2 | **Full tracker-mediated** projection/claims | Tracker-bypass local loop | Second projection and broken idempotency → §4.1, FR-1/FR-2 |
| 3 | **Fresh session per task** | Warm reuse | Charter bleed → FR-5, UB-3 |
| 4 | Autonomous workers run on the same hardware under ambient policy, but implementation and review attempts each receive **separate isolated workspaces and capabilities** | Parent-clone/shared reviewer or fleet-grade hardware sandbox | Shared-state mutation is forbidden here; hardware separation remains fleet-owned → FR-4, FR-43, NG-2/NG-3 |
| 5 | Unrecoverable task failure **continues + escalates** | Halt run | Independent work continues → FR-15, UB-1 |
| 6 | Build the **durable worker core** | Throwaway script | Remote runtime reuses it → §4.1, G3 |
| 7 | **Bounded-parallel** implementation plus separately bounded review | One pool or sequential | Review never holds implementation capacity or races in one clone → FR-11, FR-40, FR-43 |
| 8 | Frozen semantic demand; selected harness maps least-cost satisfying profile | Hardcoded pin/inference/native plan data | Portable judgment and explicit egress → FR-21..FR-24, NR-6 |
| 9 | Plan/v1 conservative no-rewrite translation | Backfill/default cheap | Frozen judgment remains honest → FR-22, NR-5, UB-11 |
| 10 | Full pre-release proof then direct cutover/fallback | Shadow/canary | One auditable semantics → FR-26/FR-48, §6.5 |
| 11 | e2e origin from recorded plan reference | e2e type/prose inference | No schema fork or false classification → FR-28/FR-29, NR-8 |
| 12 | e2e worker writes test, never feature, honoring route | Bend behavior/route | Test keeps its meaning → FR-30/FR-31, UB-15/UB-16 |
| 13 | e2e test hermetic before green | Ordinary self-heal handles flake | Measurement is not a gate → FR-32, UB-17, AP-29 |
| 14 | Routing offline/inference-free; only canonical implementation evidence may network pre-claim after parent activation | Hidden lookup/buffer | No orphan route/hidden dispatch dependency → NFR-9, UB-18, §6.2 |
| 15 | Configured review is advisory to EVT-10 but durable and **authenticated to the launched reviewer invocation**: absent by default; setup/auth before task spend; assurance-band route; exact-green revision captured per task; implementation resources released; isolated reviewer child; impl-review-owned broker performs FR-6's proposal write; exact protected receipt+attempt-proof verification independent of code-host author; the publication is proven by one field-identical EVT-13↔EVT-17 pair; three failure classes; consumer-selected review-only recovery | Silent default install; factory/task coupling; weakest-profile shortcut; sweep-end head lookup; parent-clone review; child or implement-now direct publication; worker-writable receipt state; accept any matching comment; infer the sibling join from similar fields or EVT-15; overload receipt and proof; process-exit success; rerun implementation; unbounded/widened recovery | Independent review evidence cannot be forged, silently skipped, under-routed, drifted, mismatched across contracts, or laundered without changing impl-review's publication contract → FR-35/FR-40..FR-49, UB-20/UB-21, EVT-10/EVT-15..EVT-17, DOD-27/DOD-28/DOD-32/DOD-36 |

**Sanity-check the proof** — routing remains semantic and inference-free. Configured-review proof now joins exact-green capture, green→persist→EVT-10→release→launch ordering, separate review capacity/workspace, fresh post-worker attribution, exactly one impl-review-owned broker publish, canonical obligation/attempt/receipt state unavailable to child writes, receipt plus attempt proof independent of code-host author, one exact seven-pair EVT-13↔EVT-17 publication join, three failure classes/counts, bounded review-only recovery, and zero implementation redispatch (M7, DOD-27..DOD-29, DOD-36).

**The author's open questions** — harness selection, worker bounds, dependency ordering, progress rendering, the concrete obligation store, attribution primitive, workspace adapter, and driver retry number remain design/config choices. Publisher ownership, receipt capability, exactly-once semantics, and the reciprocal terminal contract are now fixed by `prd:impl-review/FR-35` and FR-49. The sibling and implement-now successor designs must realize that boundary before code: impl-review's host persistence adapter performs FR-6, while implement-now supplies binding, consumes the receipt, and records EVT-17 without becoming a second publisher. FR-40..FR-49 prohibit choosing mechanisms that silently install review, re-derive the head, expose raw attribution or canonical-store writes to a child process, share the parent workspace, bypass receipt+attempt-proof authentication, infer the cross-contract join, overload receipt and proof, or clear pending work without exact verification. A distinct code-host reviewer principal is explicitly deferred by WR-4 to a later amendment, not required by this one.

**Skip the rest** — structure/fold → docgate; substance → `/prd-review`; mechanism → superseding design; internal assurance process → impl-review sibling; merge → merge-lane; e2e authoring → e2e-design/e2e-review.

### op: supersede §3.2 Goals → signals → metrics

Reason: counted review coverage must consume the exact paired publication terminals rather than infer publication from the review-lifecycle event.

| Metric | Goal | Signal | Baseline | Target (at checkpoint) | Feeds from |
|---|---|---|---|---|---|
| M1 (primary) | G1 | a real pyxis feature's tasks reach green PRs via `/implement-now` | 0 features under demand-aware routing | 1 dogfood feature fully implemented locally, operator only reviewing/merging | EVT-10, EVT-16 |
| M2 | G2 | tasks counted done that are genuinely green | n/a | 100% of succeeded tasks carry a green proposal; zero hollow-as-succeeded | EVT-4, EVT-10, EVT-16 |
| M3 | G3 | worker core and runtime adapters retain portability | one coarse rung mapper | the worker passes unchanged behind fake/local dispatch; every supported harness passes implementation-role routing conformance | EVT-7, EVT-8, EVT-16 |
| M4 | G1 | requested semantic profiles whose plan/v2 fields are identical to frozen demand or whose plan/v1 origin is a typed FR-22 translation, followed by a satisfying or explicit unsupported result | optional rung can be absent and demand can be lost | 100% exact origin/transport/resolution; zero inferred, downgraded, native-in-plan, or cross-harness substitutions | EVT-7, EVT-8, EVT-9, EVT-16 |
| M5 | G1 | distinct native implementation profiles exercised by dogfood attempts that succeed with `verification=verified` | hardcoded/implicit route can converge on one pin | at least 3 before production activation; failed/blocked attempts never count | EVT-7, EVT-10 |
| M6 | G2 | routing attempts with exactly one origin, one resolution terminal, and—when resolved—one worker outcome | no complete decision chain | 100% in conformance and dogfood before activation; remains 100% over production attempts after activation; zero orphan/duplicate/cross-class terminals | EVT-7..EVT-10, EVT-16 |
| M7 *(007 ← 005 ← 004)* | G2 | configured green proposals with one exact-green-revision obligation and append-only attempts: verified only by an exact artifact authenticated to the launched reviewer and one field-identical EVT-13↔EVT-17 publication pair, otherwise three-way closed-class failed and still pending | configured review can be silently absent, forged, process-exit-derived, drifted to a later head, or represented by similar but unjoinable publication records | 100% of configured green proposals; zero absent/drifted obligations, duplicate attempts, worker-authored or replayed accepted artifacts, artifact-free or unpaired verified terminals, receipt/proof overloads, unclassified failures, pending-with-exit-zero runs, implementation redispatches, or pending obligations laundered by a later empty frontier | prd:impl-review/EVT-13, EVT-15..EVT-17 |

### op: supersede §3.3 Guardrails and decision rule

Reason: release must reject inferred, incomplete, duplicated, or receipt/proof-overloaded reciprocal publication evidence and cite the new blanket proof row.

| Guardrail metric | Threshold | Action if breached |
|---|---|---|
| Parent/workspace mutation | any implementation or reviewer mutation of the parent clone, engine checkout, another task workspace, or repository code outside its own implementation charter | fail-closed; halt the offending process, retain evidence, and alarm |
| Double-claim with the external fleet | any task worked concurrently by both a local worker and a fleet node | fail-closed; block release until the claim mechanism is fixed |
| Self-merge | any merge performed by the executor | critical defect; executor holds no merge credential |
| Idle cost | any implementation routing/claim/model spend when its frontier is empty, any reviewer spend when no review obligation is pending, or any model spend when both frontiers are empty | investigate the combined-frontier gate and block release on an unauthorized launch |
| Semantic downgrade or harness switch | any native launch fails to satisfy one requested component, uses an unqualified mapping, or changes the selected harness | stop affected claims; block/rollback immediately; repair policy and rerun affected attempts |
| Frozen-plan mutation or post-freeze inference | any plan/v1 rewrite or plan/v2 assessment/profile reinterpretation | invalidate affected evidence; require no-rewrite translation or field-identical transport before retry |
| Routing evidence break | any attempt lacks exactly one origin/terminal or a resolved attempt lacks exactly one outcome | do not count it succeeded or toward release; preserve artifacts and repair the evidence chain |
| Configured review integrity break | any configured green proposal lacks one exact-green-revision obligation, re-derives a later head, launches before claim/width release, runs in the parent/shared workspace or beyond review capacity, verifies without the exact launched-attempt receipt and proof, lacks exactly one field-identical EVT-13↔EVT-17 publication pair, infers that join from EVT-15 or similar values, aliases native receipt with attempt proof, fails without a three-way class/escalation/tally/exit 1, duplicates an attempt, or becomes clean while still pending | preserve EVT-10, reject unauthentic or unjoinable artifacts, block release/activation, and reconcile the same obligation/revision through the bounded review-only path |

**Ship rule (pre-registered):** M1–M7 meet their pre-release targets, every live blocking DoD row (DOD-5, DOD-8..DOD-9, DOD-11..DOD-12, DOD-14, DOD-18..DOD-19, DOD-27..DOD-30, DOD-32..DOD-34, DOD-36) is green, every guardrail stays clean, every required §6.2 dependency is satisfied, the §6.6 support/docs/platform readiness rows are owned and complete, and §6.5 rollback is rehearsed through complete pre-release proof → activate every supported production implementation and configured review path in one cutover; post-launch M4–M7 and every guardrail remain continuously evaluated; any breach → no activation or immediate strongest-profile fallback, affected-attempt invalidation, and exact-revision review-only reconciliation where applicable. This is the only activation predicate; no narrower metric, DoD, dependency, stage, or launch subset may authorize production.

### op: supersede §6.2 Dependencies

Reason: merged impl-review amendment 004 now supplies the host-persistence performer authority, exactly-one native-receipt publication, and reciprocal join contract that the live dependency row still reports missing; only the successor design remains blocked.

| Dependency | Type | Needed by | Status | Owner |
|---|---|---|---|---|
| plan-new plan/v2 demand and checkpoint contract (`prd:plan-new/FR-28..FR-30`) | approval / data contract | Demand boundary | available through merged amendment 004 | plan-new owner |
| planner-driven routing semantics, compatibility, resolver, evidence, and rollout (`prd:planner-driven-model-routing/FR-7..FR-12`, `FR-15`, `FR-17`, `FR-20..FR-24`, `FR-27`) | policy / contract | all phases | approved parent contract; canonical append-first activation remains ancestry-gated | routing policy owner |
| superseding implement-now technical design | docs | Demand boundary + review integration | required after this amendment; must replace rung/hardcoded routing, define the obligation state machine and atomic green-boundary handoff, isolated review workspace/capacity, attribution primitive, three failure classes, EVT-10/EVT-15 independence, and authenticated exact-read recovery while preserving Worker/Dispatcher ownership; before planning or code it must also replace the frozen `design:implement-now-routed-execution` `prd:implement-now/FR-27` Serves and acceptance lineage with FR-48 | @mdarabi |
| e2e-plan governing-document reference + approved `e2e-design/v1` | data contract | e2e consumption | available through merged e2e authoring line; malformed/unapproved basis refuses under UB-14 | e2e-plan + e2e-design owners |
| impl-review amendment 004 and host-persistence successor design | approval / docs | review-role assurance and proposal-medium artifact production | contract available through merged amendment 004: FR-32 authorizes the impl-review-owned host adapter as the sole FR-6 performer acting for the reviewer program, FR-33 requires exactly-one publication returning the exact native receipt, and FR-35 defines the reciprocal terminal join that FR-49/EVT-17 complete on the host side. The remaining blocker is the approved impl-review host-persistence successor design realizing that authority, receipt, exactly-once, proof, and reciprocal-event boundary before integration code or activation | impl-review + implement-now owners |
| self-drive-wake consumer amendment + superseding watch design (or feature-drive absorption with the same contract) | approval / docs | unattended review-only recovery | required before production activation; consumes EVT-16 `review_pending` and three failure counts, schedules bounded review-only re-invocations, preserves exhaustion as red/escalated, and never redispatches implementation | self-drive-wake / feature-drive owner |
| shared claim mechanism + automated-implementation tag | system | Worker integration | confirmed | plan-new / fleet convention |
| qualified current native model roster and cost metadata per harness implementation and review roles | vendor / policy | Native resolution + release | required; mappings without reviewed qualification fail | @mdarabi + runtime adapter owners |
| four agent CLIs plus configured reviewer service identities authenticated | vendor / identity | Worker + review integration | operator-provisioned; configured review preflight occurs once before avoidable implementation spend and failures emit run-level EVT-15 infrastructure/exit 1 without failing a task | operator |
| merge-lane | system | post-run | as-is; executor never lands proposals | merge-lane |
