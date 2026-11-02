# Local Git code host — PRD

| Field | Value |
|---|---|
| **In one line** | Drive one operator's feature drafts and implementation proposals through a configured local bare Git origin, with an exact-head review receipt and explicit trust limits. |
| **Relevant-when** | Planning, building, or reviewing local Git proposal and lock routes; triaging draft failures on bare origins; changing Argus, Carina, code-host adapters, or approval verification. |
| **Status** | Draft; common-case product rulings recorded from the repository operator on 2026-09-28; exact-head lock review pending. |
| **Tier** | T3 — factory control and operator-approval boundary. |
| **Owner** | Pyxis draft agent (`mdarabiassistant`) |
| **Approver(s)** | Repository operator for problem, solution, and exact-head lock. |
| **Contributors** | Pyxis engineering agent; repository operator for security choices and acceptance. |
| **Informed** | AEC test operator. |
| **Date** | 2026-09-28 |
| **Appetite** | One verified release for the common local draft and code-proposal route, including installed-repository evidence. Human budget: review and approval of each exact head. |
| **Target release** | First release whose local adapter passes the installed AEC end-to-end test. |
| **Tracking** | https://github.com/agenticlab-ai/pyxis/issues/1302 |
| **Artifacts** | AEC problem ledgers and local-origin test report in `pyxis_testing`. |
| **Template version** | prd-template/v2 |
| **Design coverage** | requirements-first/v1 |

## 0. Read to rule — the human reviewer's section (~5 min)

*The body is the contract; this section only routes attention — on any conflict the body governs; machines ignore it.*

**Judge these**

| # | The call | Could have gone | If you disagree |
|---|---|---|---|
| 1 | Record a local approval receipt bound to the prepared head and landing context. | Require an independent signing key. | Review the identity limitation in FR-3, ABUSE-1, and tech-spec D-3. |
| 2 | Keep the existing bare origin and add a local proposal adapter. | Move the AEC project to a hosted PR service. | Review FR-1, FR-2, and tech-spec D-1/D-2. |
| 3 | Use the configured bare origin without a separate repository-ID protocol; disclose the shared-account and copied-origin limits. | Add an independently controlled origin identity. | Review `problems.md#P-5`, Q-1, and tech-spec D-1/D-3. |

**Sanity-check the numbers** — one local AEC feature must reach a consistent lock and code landing; zero stale or malformed receipts accepted (M1, M2). Actor identity is not machine-verifiable in this mode.

**The author's open questions** — The common-case origin scope is ruled in Q-3, and P-13 requires an initialized mainline before drafts. Exact-head review of this draft remains open.

**Skip the rest** — implementation detail lives in `tech-spec.md`; gates and tests verify the release.

## 1. Summary — the one-pager

- **Problem** — AEC's installed Pyxis works with Carina and local CI but cannot open a feature draft on its local bare Git origin. `draft status` invokes `gh api` and exits before Argus can brainstorm a feature.
- **For whom** — A single repository operator using Pyxis with a configured local Git origin per installation, beginning with the five AEC RFP reviewer features.
- **Why now** — The requested end-to-end Pyxis test is parked at feature definition on this missing code-host surface (issue #1302; AEC delivery briefs).
- **Direction** — Add a selected local Git proposal route for drafts and implementation after the configured mainline has been pushed once. Bind each approval receipt to a specific prepared head and workflow-derived landing ref, and verify their consistency before landing. Retain Carina tracker evidence and pinned release execution. Do not claim independent repository identity for a copied origin.
- **Appetite** — One release plus an installed-repository proof; per-head operator review is required.
- **Success** — From zero to at least one AEC feature whose draft and code proposal land through local Git with consistent exact-head receipts and zero hosted calls (M1).
- **Guardrail** — Zero stale-head or malformed receipts accepted by the normal Pyxis route (M2). Actor identity cannot be verified from the local record.
- **Top risks** — A shared OS account can edit the bare origin, hooks, and receipts; a claimed operator approval can be forged; stale mainline or draft heads can invalidate readiness.

## 2. Problem space

### 2.1 Problem statement

An operator choosing a local bare Git origin cannot drive a new feature through the installed Pyxis draft and delivery flow. The first status read fails on a GitHub API call, so the AEC test stops before brainstorming despite a healthy local installation.

### 2.2 Users and segments

| Segment | Role and context | Domain exp. | Tech exp. | Goal | Pain today | Evidence |
|---|---|---|---|---|---|---|
| Primary | Repository operator with local Git, Carina, and Codex | master | journeyman | Review and lock exact feature heads locally | No draft review object exists | issue #1302; AEC E-briefs |
| Secondary | Argus agent on an installed repo | journeyman | master | Drive a feature to an approval request | Draft CLI fails at status | AEC reproduction |
| Anti-segment | Agent seeking to approve its own work | — | — | — | Must remain unauthorized | `prd:feature-draft/FR-17` |

### 2.3 Jobs and desired outcomes

| # | When… (situation) | I want to… (motivation) | So I can… (outcome) | Today's workaround | Evidence |
|---|---|---|---|---|---|
| J1 | A local feature starts | Open and review one draft on the local origin | Lock one coherent contract after inspection | Park the feature | issue #1302 |
| J2 | A prepared head changes | Know the old approval is void | Avoid locking unreviewed bytes | Manual comparison | feature-draft contract |
| J3 | A plan produces code | Review and land the exact code proposal locally | Verify delivered work | Hosted adapter only | issue #1302 |

### 2.4 Evidence

| Claim | Source | Confidence |
|---|---|---|
| `draft status` fails with a local bare origin because it calls `gh api`. | issue #1302 reproduction and AEC test log | High |
| Existing draft lifecycle and lock recorder call hosted PR operations directly. | Pyxis source `internal/draft/lifecycle` and `internal/draft/lock` | High |
| The AEC bare origin and agent currently share an OS account. | Local environment inspection; tech-spec P-1 | High |

### 2.5 The journey today

| Phase | User does | Pain | Frontstage | Backstage |
|---|---|---|---|---|
| Configure | Installs pinned Pyxis with Carina and local CI | None | Doctor reports healthy with governing skeleton advisories | Local bare origin exists |
| Define | Starts Argus feature | Draft status fails | GitHub host error | Lifecycle invokes `gh` |
| Park | Records a decision brief | No contract can lock | Carina hold and intake branch | No review object |

### 2.6 Opportunity and why now

Five AEC features are parked at the same prerequisite. The operator chose to retain local bare Git and authorized Pyxis work on 2026-09-28. A supported route removes that shared block while preserving exact-head human control.

**Problem gate:** The operator approved the proposed local-route direction in the AEC session on 2026-09-28. The operator later selected the shared-account origin and waived the independent signing-key requirement. Neither statement approves a draft head.

## 3. Goals, metrics, and guardrails

### 3.1 Goals as falsifiable hypotheses

| # | We believe that… | Will result in… | Measured by… |
|---|---|---|---|
| G1 | A local proposal and approval route serves bare Git | At least one AEC feature reaches consistency-checked delivery | M1 |
| G2 | Exact-head receipt checks run at each normal landing | No stale or malformed receipt is accepted by Pyxis | M2 |

### 3.2 Goals → signals → metrics

| Metric | Goal | Signal | Baseline | Target (at checkpoint) | Feeds from |
|---|---|---|---|---|---|
| M1 | G1 | AEC feature has a consistency-checked local draft lock and code landing | 0 | At least 1 after installed-repository run | EVT-1, EVT-2 |
| M2 | G2 | Stale or malformed receipt accepted in negative fixtures | Unknown; no local route | 0 across full conformance matrix | EVT-3 |

### 3.3 Guardrails and decision rule

A release is refused if any stale-head, moved-target-ref, or malformed receipt fixture lands through the normal Pyxis route. The record checks consistency for the configured origin; it does not authenticate the approver, protect refs from direct edits by the shared account, or establish that a copied origin is distinct. A local mode must never report a hosted PR review or merge as its evidence.

### 3.4 Non-goals and waiting room

- No GitHub, Forgejo, or network call is part of an AEC local-mode run.
- Draft open and preparation against an origin without its configured mainline are outside this adapter; FR-9 provides the first-push path.
- The operator procedure reserves review for the operator; the shared-account origin cannot enforce this actor boundary.
- No repository-ID ref, independent origin authentication, or cross-origin receipt-replay guarantee is part of this mode. A clone of the configured origin can still inspect its retained records.
- Migration of existing hosted draft proposals is deferred; hosted repos retain their configured adapter.

## 4. Solution space

### 4.1 Approach and alternatives considered

Use local Git proposal refs and a versioned exact-head approval receipt. A hosted code host would alter the operator's local-origin choice. The receipt and mainline are writable by the shared OS account, so a clone of the configured origin can check their consistency but cannot authenticate the approver or prove that origin's independent identity. The operator accepted the shared-account route and waived the independent signing-key requirement for this test.

### 4.2 Scope perimeter

In scope: one configured bare origin per installation, installed config selection, draft lifecycle after the first mainline push, review and lock evidence, implementation proposals, Carina references, the separate first-mainline-push proof, doctor, logs, tests, release. Existing hosted adapter behavior is protected by NR-1.

### 4.3 Experience: flows and states

`open → drafting → prepared → awaiting operator review receipt → locked` for contracts. A changed head or named landing target ref returns to `prepared` with the prior receipt invalid. Implementation proposals follow `open → checks → awaiting operator review receipt → landed`. Every refusal names the head and the unmet condition.

### 4.4 Scenarios

| ID | Scenario | Expected state |
|---|---|---|
| Case 1 | Bare origin with configured mainline already pushed, and Carina, first feature | Draft opens without hosted calls |
| Case 2 | Agent presents a prepared head | Operator sees digest, diff, and exact SHA |
| Case 3 | Agent pushes after approval | Old approval is refused |
| Case 4 | Operator reviews code proposal | Consistency-checked landing records the exact reviewed head |

**Solution gate:** The operator approved the common-case proposal in the AEC session on 2026-09-28 at PR #1312 head `78b13bbeb974cede10d4ae5119bf1a354dd9dacb` (digest `sha256:1d85f040d154c58cdda2878b558084ac420d13da8482db6f73caf584f0274622`). P-3 through P-12 attribute its requirement changes, and Q-3 records the scope choice. A separate exact-head lock review remains pending.

## 5. System requirements

### 5.1 Functional requirements

| ID | Requirement (EARS) | Rationale | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|
| FR-1 | When local Git mode is selected, Pyxis shall run proposal and approval operations without calling a hosted code host. | Preserve the selected surface | Must | high | Installed fixture records zero hosted or network calls | Test | G1, J1 |
| FR-2 | When a local feature starts, Pyxis shall create a discoverable reviewable proposal for its pushed branch. | Give Argus a draft object | Must | high | A clone of the configured origin lists the feature and exact head | Test | G1, J1 |
| FR-3 | When a lock is requested, Pyxis shall require an explicit local review receipt naming the prepared head and landing context. | Preserve an exact review checkpoint | Must | critical | Missing, malformed, or wrong-head receipts refuse; record is labeled unauthenticated | Test | G2, J2 |
| FR-4 | If a proposal branch head or its receipt's named landing target ref changes after approval, Pyxis shall refuse the approval and request a new review. | No stale approval on mainline locks or feature-integration task landings | Must | critical | Moved branch and moved target-ref fixtures refuse before landing | Test | G2, J2 |
| FR-5 | Before local lock approval is accepted, Pyxis shall apply the same draft readiness and memory gates as hosted mode. | Contract parity | Must | critical | Every hosted readiness negative fixture also refuses locally | Test | G2, J1 |
| FR-6 | When a local lock lands, Pyxis shall retain a versioned record that a clone of the configured origin can check against the reviewed head and landing commit. | Durable consistency evidence | Must | critical | Clone validates bytes, receipt fields, version, and landing commit without claiming approver or independent repository identity | Test | G1, J1 |
| FR-7 | When an implementation proposal lands locally, Pyxis shall require an exact-head review receipt naming its workflow-derived target ref and verify that ref's landing result. | Delivery checkpoint for feature-integration branches as well as mainline | Must | critical | Missing, stale, or proposal-chosen wrong-target receipts refuse; consistent receipt lands once through Pyxis | Test | G1, J3 |
| FR-8 | While Argus drives a local feature, Pyxis shall make proposal identities and statuses available to Argus and Carina without a hosted URL. | Honest records | Must | high | Ledgers and tracker name local ref and head; no fabricated PR URL | Test | G1, J1, J3 |
| FR-9 | When an empty local origin receives its first configured-mainline push, Pyxis shall support a base-free proof; until that push completes, local draft open and preparation shall refuse with a first-push remedy; later pushes shall require normal history gates. | Bootstrap before drafts | Must | high | First-push fixture succeeds, pre-push draft open/prepare name the remedy, initialized-origin draft proceeds, and subsequent violating push refuses | Test | G1 |
| FR-10 | When local mode is configured or run, Pyxis shall report its prerequisites and actual outcomes through doctor, logs, and metrics. | Operability | Must | medium | Missing origin is named; shared-account and unauthenticated-approval limits are visible | Test | G1 |

**Non-regression (NR-n)**

| ID | Existing behavior protected | Why protected | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|
| NR-1 | Hosted draft and implementation routes retain their exact-head checks. | Existing users | critical | Existing conformance suites pass with zero changed expected results | Test | FR-1 |
| NR-2 | `pyxis exec` verifies pinned artifacts. | Supply-chain boundary | critical | Tampered release fixture still refuses | Test | FR-10 |

### 5.2 Unwanted behavior and failure handling

| ID | If… (trigger) | Then the system shall… | User sees | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|
| UB-1 | Receipt has missing fields or wrong head, proposal, or target ref | Refuse without mutation | Specific mismatch | Must | critical | All in-scope consistency mismatch fixtures refuse | Test | FR-3 |
| UB-2 | Proposal branch or named target ref moves during landing | Refuse and retain review evidence | New approval needed | Must | critical | Atomic race fixtures land zero unapproved bytes | Test | FR-4 |
| UB-3 | Bare origin or local approval policy is unavailable | Refuse the operation | Missing prerequisite named | Must | high | No partial lock or merge | Test | FR-10 |
| UB-4 | Atomic proposal publication fails or is interrupted | Leave the prior branch and proposal pair intact and support idempotent retry | Recovery state named | Must | high | Retry creates one consistent branch/proposal pair | Test | FR-2 |

### 5.3 Permissions and abuse cases

| Function / data | Agent | Operator | Unauthenticated |
|---|---|---|---|
| Proposal branch and review packet | write | read | none |
| Approval receipt | prepare/read | record after exact-head review | none |
| Mainline lock or implementation landing | request review only | record receipt and invoke the landing command after exact-head review | none |

| ID | Actor and intent | Vector | Impact | Severity | Mitigated by | Verify |
|---|---|---|---|---|---|---|
| ABUSE-1 | Agent impersonates approval | Writes a local receipt as the shared account | Unreviewed lock | critical | Accepted residual risk under Q-2; log receipt as unauthenticated | Inspection of trust disclosure |
| ABUSE-2 | Agent replays a receipt on the configured origin | Reuses it after a push | Stale contract or code | critical | FR-4 | Same-origin replay fixture |
| ABUSE-3 | Agent bypasses the normal Pyxis route | Edits hook or main ref under shared OS account | Mainline changes outside approval | critical | Accepted residual risk under Q-1 | Inspection of trust disclosure |

### 5.4 Data

| Entity / field | Meaning | Source of truth | Retention | Integrity rules | Privacy class |
|---|---|---|---|---|---|
| Local proposal | Branch, head, target ref and old target SHA, state, kind | Git object/ref on configured origin | Git history | Versioned, atomic compare-and-swap with branch ref | internal |
| Approval receipt | Declared review of exact head and landing context | Local review record | With proposal and lock | Field and ref consistency; actor identity unauthenticated | internal |
| Lock record | Binding contract version in local mode | Annotated Git tag plus review receipt | Permanent | Same-origin clone consistency check | internal |

No existing hosted records migrate. Rollback disables the opt-in mode for new proposals while retaining consistency-checkable historical records. No AEC code or diff leaves the local machine in local mode. The declared reviewer name is audit metadata, not authenticated identity.

### 5.5 Interfaces and integrations

| Interface | Direction | Purpose | Contract | Failure modes | Owner |
|---|---|---|---|---|---|
| Bare Git origin | both | Proposal and landing refs | Full SHA and versioned records | Missing or moved ref refuses | Pyxis local adapter |
| Carina | both | Feature/task status and proposal pointers | Local ref plus head identity | Tracker unavailable leaves journaled retry | Carina adapter |
| Operator review surface | both | Exact-head review and landing | Canonical request, explicit local receipt recording, and operator-invoked landing | Absent or stale receipt refuses | Repository operator |

### 5.6 Non-functional requirements

| ID | Area | Requirement | Fit criterion | Verify |
|---|---|---|---|---|
| NFR-1 | Security | Local mode shall label approval identity, origin protection, and repository identity as unauthenticated under the accepted shared-account policy. | Doctor, receipts, and clone verification disclose the limits; no signed or independent-origin claim appears | Test, Inspection |
| NFR-2 | Portability | Local mode shall work without a network service. | Installed fixture completes with network disabled | Test |
| NFR-3 | Recoverability | A failed write shall not hide or duplicate proposal or lock evidence. | Crash and retry fixtures converge to one state | Test |

### 5.7 Instrumentation — the tracking plan

| ID | Event name | Fires when | Properties (typed) | Source | Feeds |
|---|---|---|---|---|---|
| EVT-1 | local_draft_locked | Consistent lock recorded | feature: string, head: sha, version: int | Pyxis lock | M1 |
| EVT-2 | local_proposal_landed | Consistent implementation landing | task: string, head: sha | Pyxis local adapter | M1 |
| EVT-3 | local_approval_refused | Invalid approval rejected | reason: enum, proposal: string | Pyxis verifier | M2 |

### 5.8 Definition of Done

| ID | Done criterion | Source | Verify | Required evidence | Blocking? |
|---|---|---|---|---|---|
| DOD-1 | All local lifecycle requirements and failures pass in a real bare-origin fixture. | FR-1 through FR-10; UB-1 through UB-4 | Test | Conformance run | yes |
| DOD-2 | Stale and malformed receipts refuse through the normal Pyxis route, and the accepted impersonation risk is visible. | FR-3, FR-4; ABUSE-1 through ABUSE-3; M2 | Test, Inspection | Negative fixtures and trust disclosure | yes |
| DOD-3 | Existing hosted routes and pinned execution pass. | NR-1, NR-2 | Test | Regression suite | yes |
| DOD-4 | AEC pins a checksum-verified candidate artifact set from the intended release source and completes at least one exact-head reviewed feature and code landing without hosted calls before final publication. | M1; FR-8; NR-2 | Test, Demo | Candidate manifest and pin, local refs, lock tag, Carina item, logs | yes |
| DOD-5 | Before local mode is enabled, the feature-draft contract has an approved scoped amendment and superseding design for unauthenticated shared-account approval. | prd:feature-draft/FR-17; prd:feature-draft/FR-46; prd:feature-draft/UB-3 | Inspection | Merged amendment, folded projection, and reviewed successor design | yes |
| DOD-6 | Before local task or final feature landing is enabled, the feature-drive contract has an approved scoped amendment and successor design for operator-invoked local landing and its shared-account enforcement limit. | prd:feature-implementation-drive/FR-7; prd:feature-implementation-drive/FR-22; prd:feature-implementation-drive/NR-5 | Inspection | Merged amendment, folded projection, and reviewed successor design | yes |

### 5.9 AI behavior and evals

Skipped: This release changes deterministic proposal and approval mechanisms; it adds no model-driven output requirement.

## 6. Delivery

### 6.1 Phasing and milestones

| Phase | Purpose | Scope | Exit criterion |
|---|---|---|---|
| Contract review | Set trust boundary | Q-1, Q-2, and exact-head protocol | Rulings recorded; draft reviewed |
| Adapter and gates | Implement local lifecycle | Drafts after initialized mainline, proposals, locks, and first-push integration | Conformance suite passes |
| Installed proof | Test AEC | Pinned candidate artifact set, Carina, Codex | DOD-4 evidence exists before final release publication |

### 6.2 Dependencies

Local bare origin and recorded shared-account trust choice; Carina own-item scope fix tracked separately by issue #1309. No independent signing key is required for the accepted mode. The first configured-mainline push uses the installed init-epoch proof from issue #1305/PR #1307 before a local draft opens; AEC's mainline already exists. The existing feature-draft contract requires authenticated human approval (`prd:feature-draft/FR-17`, `FR-46`, `UB-3`; `design:feature-draft-v1#DINV-4`), so a scoped PRD amendment and successor design must be approved before local mode is enabled. The feature-implementation-drive PRD requires human task merge (FR-7), drive-authored final proposal without drive merge (FR-22), and unchanged merge authority and repository gates (NR-5); its design §3.9 requires host-enforced current-candidate verdicts. A scoped amendment and successor must authorize the operator-invoked local task and final landing, retained aggregate and candidate-evidence checks, and atomic-ref predicate under the accepted shared-account limitation. The drive itself still does not invoke landing. Local task and final landing remain disabled until those contracts land. Hosted mode keeps its existing contract.

### 6.3 Risks, assumptions, and rabbit holes

| Risk | Consequence | Response |
|---|---|---|
| Shared OS account controls origin hook and refs | Host-level branch protection is unavailable | Disclose accepted Q-1 limitation |
| Agent can write an approval receipt | Actor identity cannot be proven from the record | Disclose accepted Q-2 limitation and require procedural review |
| Configured origin is copied with refs and objects | Copied records can appear consistent at another origin | Do not claim cross-origin binding; check only configured-origin record and object consistency |
| Named landing target ref moves after readiness | Approval applies to stale context | Bind receipt to target ref and full SHA, then retry |

### 6.4 Release criteria

All DOD rows pass and the operator reviews the exact draft head. A passing test fixture or locally authored receipt is not itself proof of that human review. The installed AEC report cites the operator's actual exact-head review statement separately from the local receipt.

### 6.5 Rollout and reversibility

Opt in per repository. Preserve existing hosted bindings. A disabled local mode leaves historical refs and lock records readable. No automatic migration.

### 6.6 Launch readiness

Doctor can diagnose a missing origin and report the selected trust policy. The installed test report records release pin, local refs, Carina state, gates, and actual metrics. Operator instructions explain exact-head review, receipt recording, and recovery.

### 6.7 Post-launch review

After the first three local feature deliveries, inspect M1 and M2 and review every refusal and recovery. A guardrail breach blocks further local locks until fixed.

## 7. Alignment

### 7.1 FAQ

**Can the agent supply approval?** The procedure requires operator review, but the shared-account local record cannot prove the actor. An agent could forge it. This is an explicit accepted limitation, so output must never label the actor as cryptographically verified.

**Does an editable local hook protect mainline?** No. Q-1 records the operator's shared-account choice.

### 7.2 Open questions

| ID | Question | Owner | Blocks | Status | Resolution |
|---|---|---|---|---|---|
| Q-1 | Will the bare origin run under a separately controlled principal with protected receive enforcement, or does the operator accept a shared-account origin with a narrower consistency guarantee? | Repository operator | Solution gate and lock | resolved | Operator chose the shared-account origin on 2026-09-28; host-level mainline protection is out of scope. |
| Q-2 | Will the operator provide a signing key outside agent reach for approval authentication? | Repository operator | Production verification | resolved | Operator said to ignore this requirement on 2026-09-28; local receipts cannot authenticate the approver. |
| Q-3 | Should the common-case local route omit a separate repository-ID protocol while retaining atomic stale-head checks for parallel agent writers? | Repository operator | Solution gate and lock | resolved | Operator approved the proposal at PR #1312 head `78b13bbeb974cede10d4ae5119bf1a354dd9dacb` on 2026-09-28: keep atomic stale-head checks, verify the configured origin's record and object consistency, and make no cross-origin identity claim. |
| Q-4 | Must a local draft open or prepare before the configured mainline's first push? | Repository operator | Draft lifecycle and lock | resolved | Operator chose pushing mainline first on 2026-09-28; the AEC origin already has mainline. P-13 records the ruling; FR-9 retains the separate first-push proof. |

### 7.3 Decision log

| Date | Decision | Source |
|---|---|---|
| 2026-09-28 | Keep the local bare origin and build exact-head operator review. | `brainstorm:local-git-codehost#D-1`; AEC operator ruling A-3 in each feature ledger |
| 2026-09-28 | Keep the shared-account origin and accept its lack of host-level protection. | `brainstorm:local-git-codehost#D-2` |
| 2026-09-28 | Waive an independent operator signing key for this test. | `brainstorm:local-git-codehost#D-3` |
| 2026-09-28 | Request a common-case revision for a single Pyxis operator. | Operator message on 2026-09-28; `problems.md#P-5` |
| 2026-09-28 | Approve the revised common-case proposal, including atomic stale-head checks and no separate origin-ID protocol. | Operator reply “approved” to PR #1312 head `78b13bbeb974cede10d4ae5119bf1a354dd9dacb`; `problems.md#P-3` through `#P-12`; Q-3 |
| 2026-09-28 | Require first mainline publication before opening or preparing local drafts; retain the separate base-free first-push proof. | Operator answer “ok, push something to the mainline”; `problems.md#P-13`; Q-4 |

### 7.4 Changelog

| Date | Change |
|---|---|
| 2026-09-28 | Opened the local code-host contract draft and recorded the trust questions. |
| 2026-09-28 | Resolved both trust questions and narrowed the local approval guarantee to record consistency. |
| 2026-09-28 | Proposed a common-case scope that checks one configured origin's records without a separate repository-ID protocol. |
| 2026-09-28 | Recorded the operator's P-3–P-8 product rulings and resolved Q-3; the exact-head lock approval remains pending. |
| 2026-09-28 | Added P-9–P-12 to attribute the already approved FR-2, FR-6, ABUSE-2, and NFR-1 row changes surfaced by the first lock packet. |
| 2026-09-28 | Recorded P-13/Q-4 and clarified FR-9's first-push and draft-ordering cases. |
| 2026-09-28 | Made the inherited feature-drive FR-7, FR-22, and NR-5 migration prerequisite explicit after prepared-head review, including DOD-6. |

## 8. Contract readiness — the executability gate

- Problem evidence, direction, and the common-case solution gate are recorded. The exact-head lock review remains pending.
- Q-1 through Q-4 and P-3 through P-13 are resolved by operator rulings; no exact-head approval is inferred.
- Every Must requirement has a measurable fit criterion and a technical decision in `tech-spec.md`.
- Release completion requires the installed AEC evidence in DOD-4.
