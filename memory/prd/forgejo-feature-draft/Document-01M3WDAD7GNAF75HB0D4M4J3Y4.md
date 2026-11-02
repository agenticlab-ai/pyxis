# Forgejo feature drafts — PRD

| Field | Value |
|---|---|
| **In one line** | Run the current feature-draft workflow on a configured Forgejo code host with authenticated exact-head human approval and retained GitHub behavior. |
| **Relevant-when** | planning, building, or reviewing draft status/open/reopen/round/prepare-lock/abandon, approval, lock recording, or draft push gates for installed Forgejo repositories |
| **Status** | Draft — Argus authoring gate under tier-2; independent reviews and human lock approval pending |
| **Tier** | T3 — identity, credentials, branch protection, and immutable contract lock |
| **Owner** | Argus, principal for problem:forgejo-feature-draft |
| **Approver(s)** | human operator for the exact prepared head; Argus tier-2 for authoring gates only |
| **Contributors** | operator acceptance request; Argus; independent Codex reviewers |
| **Informed** | Pyxis maintainers and installed Forgejo repository operator |
| **Date** | 2026-09-30 created · 2026-10-01 last updated |
| **Appetite** | One focused compatibility correction plus stage-correct draft push gating; one human contract-lock review, cold PRD and design reviews, and one installed end-to-end proof. No token/cost budget or override was supplied; this CLI exposes no dependable spend receipt. |
| **Target release** | First verified release after this draft is human-locked and implemented, before resuming the AEC feature drafts. |
| **Tracking** | one draft/forgejo-feature-draft proposal; problem:forgejo-feature-draft |
| **Artifacts** | drafts/forgejo-feature-draft/brainstorm/001.md; ../../acceptance/objective.txt |
| **Template version** | prd-template/v2 |
| **Design coverage** | requirements-first/v1 |

## 0. Read to rule — the human reviewer's section (~6 min)

*The body is the contract; this section only routes attention — on any conflict the body governs; machines ignore it.*

**Judge these**

| # | The call | Could have gone | If you disagree |
|---|---|---|---|
| 1 | Extend the frozen Pyxis-only draft scope to installed Forgejo repositories (§4.2) | Keep the existing boundary and halt the AEC acceptance path | Refuse this compatibility extension before lock; do not edit the frozen design. |
| 2 | Require a separate human approval identity and exact prepared head (§5.3, FR-5, FR-6) | Guess from repo owner or accept the agent review | Refuse lock authorization until the identity rule is settled. |
| 3 | Use the configured native host and retain GitHub (§5.5, FR-1, NR-1) | Route Forgejo through gh or move AEC to GitHub | Refuse the medium route; keep AEC on Forgejo. |
| 4 | Defer only readiness during production opening/round pushes; require complete memory integrity and readiness at preparation/lock (FR-7, DOD-6) | Skip additional gates or demand final readiness at opening | Inspect the scoped policy and final lossless-binding evidence. |

**Sanity-check the numbers** — one PR per feature version (§3.2); zero stale/agent approvals (§3.3, DOD-3); one verified mainline lock tag per version (DOD-4).

**The author's open questions** — none in the requirements; actual operator lock approval remains a separate pending act (§7.2).

**Skip the rest** — structure → draft readiness/memory gate; coherence and security → cold reviews; code → code gate and installed proof; final permission → the human's exact-head code-host review.

## 1. Summary — the one-pager

Argus cannot start the installed AEC feature-draft workflow: `pyxis draft status` invokes GitHub's CLI against a Forgejo origin. Other draft verbs and the lock recorder also assume GitHub, while the operator requires Forgejo for both the application's tracker and code host. Pyxis source keeps GitHub (problem:forgejo-feature-draft#A-1; brainstorm:forgejo-feature-draft#F-1/#F-2).

This contract proposes a visible compatibility extension of the current draft lifecycle. The configured code-host binding selects native Forgejo operations; Git transports refs; GitHub retains its current route. One draft PR moves through rounds, current-mainline readiness, exact-head human review, lock, planning, and verified delivery. A separate source pre-push conflict with unready scaffolds must also be corrected so the normal authoring route works without dropping readiness at lock (brainstorm:forgejo-feature-draft#F-4/#P-2).

Success at the release checkpoint is a fresh installed Forgejo repo completing the lifecycle with one PR and a verifiable mainline lock. The guardrail is zero locks from stale, agent, malformed, or unverified reviews and zero GitHub regression. An origin or identity mix-up, partial API read, or tag based only on a platform badge blocks release. This document grants no lock approval (problem:forgejo-feature-draft#A-2/#A-3).

## 2. Problem space

### 2.1 Problem statement

An operator who installs Pyxis on Forgejo cannot enter the feature-draft front door. Status targets GitHub and fails before requirements. Moving the application to GitHub would violate the acceptance request. Without this correction all five dependent AEC features remain blocked (problem:forgejo-feature-draft#A-1; ../../acceptance/objective.txt).

### 2.2 Users and segments

| Segment | Role and context | Domain exp. | Tech exp. | Key goals | Pain today | Evidence |
|---|---|---|---|---|---|---|
| P1 | Argus driving a new feature in an installed Forgejo repo | journeyman | master | Author one draft and reach a real lock | status fails at gh | problem:forgejo-feature-draft#A-1 |
| P2 | Human operator of that repo | master | journeyman | Review one exact prepared head and verify lock | cannot reach approval | problem:forgejo-feature-draft#A-1 |
| Anti-segment | Local-Git code-host work and migrations of existing document-pipeline features | — | — | Separate contracts | mechanism would be conflated | problem:forgejo-feature-draft#A-2; design:feature-draft-v1 §3.2 |

### 2.3 Jobs and desired outcomes

| # | When… (situation) | I want to… (motivation) | So I can… (outcome) | Today's workaround | Evidence |
|---|---|---|---|---|---|
| J1 | a new installed Forgejo feature begins | inspect status, open one draft, and iterate contracts | reach a reviewable exact head on Forgejo | blocked at gh status | brainstorm:forgejo-feature-draft#D-1/#F-1 |
| J2 | a ready draft is prepared | approve its exact current head under my identity | lock only the contract I saw | no Forgejo approval reader | brainstorm:forgejo-feature-draft#D-3/#F-2 |
| J3 | the prepared proposal merges | verify its mainline content and lock | plan from the binding version | GitHub-only recorder | brainstorm:forgejo-feature-draft#F-2 |

### 2.4 Evidence

| Claim | Source | Confidence |
|---|---|---|
| Installed `draft status company-memory` invokes gh and refuses the Forgejo origin. | Operator's e848afcc / v0.1.474-aecforgejo.1 reproduction in problem:forgejo-feature-draft §Idea | High for that installation |
| Other draft lifecycle and lock paths are GitHub-specific. | `internal/draft/lifecycle`, `internal/draft/lock`, `cmd/draft-lock-record` at e848afcc | High |
| Forgejo bindings, REST client, code-host, tracker, and Git credentials already exist; installed feature E2E did not author a draft. | `internal/integrations`, `internal/adapters/common/forgejo`, `internal/adapters/codehost/forgejo`, `internal/gitcredential`; learning/forgejo-fast-installed-feature-e2e | High |
| Source pre-push refuses an intentionally unready opening scaffold. | delivery:forgejo-feature-draft#E-2; `.pyxis/config.yaml` draft-ready; `internal/memory/cli/ready.go` | High for e848afcc |

### 2.5 The journey today

| Phase | User does | Pain | Frontstage | Backstage |
|---|---|---|---|---|
| Install | selects Forgejo bindings | none here | configured repo | native integration and Git helper |
| Inspect | requests status | immediate stop | gh refuses origin | `lifecycle.Inspect` hardcodes gh |
| Author | attempts open and rounds | cannot reach one PR | no native draft proposal | GitHub-only identity, push, PR verbs |
| Lock | requests human review | cannot certify it | no lock tag | GitHub-only approval and merged-PR lookup |

### 2.6 Opportunity and why now

One acceptance repository has five dependent capabilities waiting on this authoring path. Native Forgejo execution services exist, leaving a focused draft integration and exact-head lock proof. Doing nothing blocks the acceptance run; moving hosts is excluded (problem:forgejo-feature-draft#A-1; learning/forgejo-native-task-code-and-lifecycle-identity). Argus signs only the authoring problem framing at tier-2; the human still owns the lock (delivery:forgejo-feature-draft#E-4).

## 3. Goals, metrics, and guardrails

### 3.1 Goals as falsifiable hypotheses

| # | We believe that… | Will result in… | Measured by… |
|---|---|---|---|
| G1 | enabling native feature drafts for installed Forgejo repositories | one new feature reaches a mainline-verified lock and planning handoff through the current workflow | M1 |
| G2 | preserving exact-head approval and the GitHub route | zero non-human or stale-head locks while source GitHub remains usable | M2 |

### 3.2 Goals → signals → metrics

| # | Metric | Goal | Signal | Baseline | Target (at checkpoint) | Feeds from |
|---|---|---|---|---|---|---|
| M1 | Installed feature-draft completion | G1 | Retained run has PR identity, head/digest, mainline lock/tag, and handoff | status fails at first gh call | all draft stages pass with one PR per version after correction release | installed run report (§5.7) |
| M2 | Approval and host compatibility | G2 | Negative lock fixtures and source GitHub tests | Forgejo draft path absent; GitHub path present | zero false locks and existing GitHub suite green at implementation review | test and review reports (§5.7) |

### 3.3 Guardrails and decision rule

Any lock without an authenticated human review of the exact prepared head, a verified mainline merge, or matching compiled contract records blocks release. A credential in argv, URL, log, error, record, or approval request blocks release. A Forgejo failure cannot silently fall back to GitHub. All blocking DoD rows need evidence.

### 3.4 Non-goals and waiting room

| ID | Non-goal / waiting room | Why refused / revisit trigger |
|---|---|---|
| NG-1 | Move AEC code/tracker to GitHub or add a fake gh shim | Operator A-1 requires native selected-host behavior. |
| NG-2 | Implement local-Git code hosting | Separate draft; revisit through its own lock. |
| NG-3 | Duplicate PR #1307's installed first-push fix | Existing green PR needs a human merge; revisit only if it fails after merge. |
| NG-4 | Weaken approval or merge this draft during authoring | Operator A-1/A-2/A-3; only the actual human review counts. |

**Waiting room**

| ID | Deferred work | Reason and revisit trigger |
|---|---|---|
| WR-1 | Migrate existing document-pipeline features to drafts | `prd:feature-draft/WR-5` governs separately. |

## 4. Solution space

### 4.1 Approach and alternatives considered

Extend the configured code-host route and retain one draft lifecycle. Stage 1 compared a Forgejo-only replacement, a compatible native adapter, and a parallel pipeline. The operator's constraints select the compatible extension (brainstorm:forgejo-feature-draft#D-2). API and seam choices belong to the tech spec.

### 4.2 Scope perimeter

This contract **proposes a compatibility extension** to frozen `design:feature-draft-v1` §3.2, which excluded repositories other than Pyxis. It does not edit that design or `prd:feature-draft`; the extension binds only if this new exact-head draft is human-approved. It covers Forgejo status/open/reopen/round/prepare/abandon, approval reading, lock recording, origin validation, source GitHub compatibility, and stage-correct draft push gates. The existing locked-plan/execution handoff and task tracker remain unchanged (problem:forgejo-feature-draft#A-1/#A-2; brainstorm:forgejo-feature-draft#D-2/#D-4).

### 4.3 Experience: flows and states

| State | Action | Next observed state |
|---|---|---|
| no contract | status and open | one native draft PR on selected host |
| drafting | commit and push round | same PR, new head; old approval void |
| ready draft | prepare-lock | rebased, compiled head; human review requested |
| approved prepared head | human merge | verified mainline contract and annotated lock tag |
| locked | plan/project | locked version is planning basis |
| failure | reconcile branch/PR/tag and retry | no second proposal, false lock, or lost files |

### 4.4 Scenarios

- S1 — Fresh Forgejo install: status reports no contract, open creates one draft PR, rounds reuse its number.
- S2 — Head changes after approval: recording refuses until a fresh human review.
- S3 — A merge badge lacks mainline ancestry or matching compiled records: no tag is published.
- S4 — Origin points to a different server/repository: draft verb refuses before a host write.
- S5 — Source GitHub draft behavior remains green; an unfinished round can pass applicable pre-push checks while preparation still requires readiness.

## 5. System requirements

### 5.1 Functional requirements

| ID | Requirement (EARS) | Rationale | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|
| FR-1 | When a draft verb runs, the system shall select `integrations.bindings.code_host` independently of the tracker and verify that the configured Git remote names the same bound server/repository before any credential-bearing API or Git request or host mutation; refuse credential-bearing redirects to an unbound destination. | Keep AEC on Forgejo. | Must (rank 1) | critical | Matching Forgejo uses native host; a different server, port, base path, owner, repository, scheme, unsupported binding, or hostile redirect receives zero credential-bearing API/Git requests and zero writes; GitHub keeps its route. | Test | problem:forgejo-feature-draft#A-1; brainstorm:forgejo-feature-draft#D-1/#D-2 |
| FR-2 | When status is requested, the system shall derive open draft proposals, remote lock versions, and document-pipeline ledgers for the selected host without another host's CLI. | Status is entry gate. | Must (rank 2) | high | Forgejo fixtures cover no contract, new draft, locked version, reopened draft with the previous version still binding, and an unmigrated document-pipeline feature with zero gh calls; absent or ambiguous proposal state is named. | Test | brainstorm:forgejo-feature-draft#F-1; design:feature-draft-v1 §3.1 |
| FR-3 | When a new or reopened draft starts, the system shall authenticate the configured agent, use Git with selected-host credentials to publish a branch, and create exactly one native draft PR on the selected host. | One revisable proposal. | Must (rank 3) | high | Fresh/reopened Forgejo fixtures verify agent, branch/head/base/repo, draft state, URL, and one PR after retry; GitHub fixture retains behavior. | Test | brainstorm:forgejo-feature-draft#D-2; prd:feature-draft/FR-1, FR-20 |
| FR-4 | When a draft round resumes or is abandoned, the system shall update the same selected-host PR and branch under a remote-head lease, retaining round and abandonment semantics. | Prevent duplicates/misdirection. | Must (rank 4) | high | Round restores exact working bytes and same PR; abandonment leaves required reason record; moved remote head refuses. | Test | brainstorm:forgejo-feature-draft#F-2; design:feature-draft-v1 §§3.7,3.14 |
| FR-5 | When a ready draft is prepared, the system shall preserve valid Argus content-bound trailers across rebase, compile the contract against current mainline, publish the exact prepared head with a lease, update the same selected-host PR, and request separate human review of that head; before permitting a prepared contract to merge, enforce the configured human's authenticated approval of the exact current head, dismissal after a new push, and the same rule for privileged actors, or refuse with a named protection remedy. | Present exactly what may lock. | Must (rank 5) | critical | Source/rebased bindings validate; digest equals stored texts; PR head equals prepared SHA; reviewer differs from agent; unapproved, stale, agent-approved, or privileged-bypass merge attempts are refused before compiled records reach mainline; fresh configured-human approval of the exact head permits merge; post-merge review and mainline checks still apply. | Test | problem:forgejo-feature-draft#A-1; brainstorm:forgejo-feature-draft#P-1; prd:feature-draft/FR-17, FR-47 |
| FR-6 | When a prepared draft is merged, the system shall identify exactly one same-repository merged PR on the selected host, verify the operator's authenticated review of its exact prepared head, verify compiled records and ancestry on mainline, then publish or byte-verify an annotated lock tag for that merge. | Badge alone cannot authorize planning. | Must (rank 6) | critical | Missing/multiple PRs, stale/agent/rejected reviews, unmatched records/digest, non-ancestor merge, and conflicting tag refuse; valid squash merge yields one tag with head/digest/review/PR/mainline commit; rerun idempotent. | Test | problem:forgejo-feature-draft#A-1; prd:feature-draft/FR-19, FR-46, FR-47 |
| FR-7 | When an unfinished draft branch is pushed, only final-draft-readiness shall be deferred; all other applicable source checks, including memory integrity, shall run. When preparation, prepared-head publication, or lock is requested, the system shall require full memory integrity and current-digest draft readiness and refuse every unmet condition. The temporary problem A-4 exception is not a production gate policy. | Open scaffolds and rounds are intentionally unready. | Must (rank 7) | high | Normal opening/round push succeeds with the readiness-only deferral and all other checks green; final prepared head and lock pass the complete memory-integrity gate, including lossless problem/delivery bindings, plus current-digest reviews and every readiness condition; invalid evidence or missing bindings refuse without publication. | Test | brainstorm:forgejo-feature-draft#F-4/#P-2; problem:forgejo-feature-draft#A-3 |

| ID | Existing behavior protected | Why protected | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|
| NR-1 | Pyxis source GitHub draft lifecycle and approval remain working. | Source uses GitHub. | critical | GitHub conformance and exact-head negative tests pass without changed reviewer semantics. | Test | problem:forgejo-feature-draft#A-1 |
| NR-2 | Forgejo tracker, task execution, and selected credentials stay independent of draft host decisions. | No application migration. | high | Equal numeric IDs on separate servers remain separate; draft never calls task binding as host. | Test | learning/forgejo-native-task-code-and-lifecycle-identity |
| NR-3 | Frozen source documents, first-push PR #1307, and local-Git code-host work stay on their sanctioned paths. | Separate concerns. | medium | No frozen source edit or separate implementation in correction diff. | Inspection | problem:forgejo-feature-draft#A-2 |

### 5.2 Unwanted behavior and failure handling

| ID | If… (trigger) | Then the system shall… | User sees | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|---|
| UB-1 | If binding or remote origin is missing, malformed, unsupported, or mismatched | refuse before a credential-bearing API/Git request or host write and identify the repair. | named refusal | Must | critical | Wrong server, port, base path, owner, repo, scheme, and hostile redirect receive zero credential-bearing requests and zero writes. | Test | FR-1 |
| UB-2 | If agent token resolves to another login, operator login is absent/equal to agent, or review actor cannot be authenticated | refuse without inferring a human identity. | identity refusal | Must | critical | Mismatch fixtures create no PR/tag; native review actor compared with configured human. | Test | FR-3, FR-5, FR-6; delivery:forgejo-feature-draft#E-3 |
| UB-3 | If proposals are missing, forked, duplicated, moved, or inconsistent with selected branch/base/repo | refuse and preserve draft for reconciliation. | proposal refusal | Must | high | Zero/multiple/fork/moved-head fixtures produce no lock or second PR. | Test | FR-2..FR-6 |
| UB-4 | If operator review is stale, pending, commented, rejected, on another head, or superseded | refuse and request fresh exact-head human review. | approval refusal | Must | critical | All negative sequences yield no tag; latest valid operator approval at current head succeeds. | Test | FR-6; prd:feature-draft/FR-46, FR-47 |
| UB-5 | If Forgejo returns HTTP/transport failure, malformed/partial JSON, or incomplete pagination | fail closed without exposing token or claiming success. | safe retry context | Must | high | 401/403/5xx, timeout, malformed body, truncated pages, and redirects yield no credential/false result. | Test | FR-1..FR-6; NFR-1 |
| UB-6 | If branch push, PR update/create, or tag push fails after an earlier step | report observed identity and permit reconcile/retry without losing files or duplicating work. | recoverable step | Must | high | Kill/retry converges to one PR/tag; unchanged draft bytes survive failed publication. | Test | FR-3..FR-6; design:feature-draft-v1 #DINV-10 |

### 5.3 Permissions and abuse cases

| ID | Actor and intent | Vector | Impact | Mitigated by | Verify |
|---|---|---|---|---|---|
| ABUSE-1 | Agent or test attempts lock authorization. | Synthetic or agent review | False human approval | Separate configured human actor and authenticated exact-head review | Test |
| ABUSE-2 | Actor replays an old approval. | Push after approved head | Stale contract locks | Re-read current head around review enumeration and refuse | Test |
| ABUSE-3 | Actor redirects a draft operation. | Wrong or hostile Git origin | Cross-repository mutation or credential exposure | Exact selected-host remote match before credentials or mutation | Test |
| ABUSE-4 | Faulty provider exposes credentials. | API or Git error echoes token or body | Secret leakage | Sanitize errors and keep token out of argv, URL, record, and logs | Test |
| ABUSE-5 | Actor claims lock from a merge badge. | Non-mainline merge or self-declared record | Planning from false contract | Require ancestry, compiled record match, and tag readback | Test |

The Forgejo operator login is a separately configured human identity under Argus assumption (delivery:forgejo-feature-draft#E-3). Configuration cannot manufacture an operator review.

### 5.4 Data

Draft working files and literal compiled memory Documents retain current schemas. The closed `draft-lock/v1` payload keeps digest/head/review/merge identity. The provider-qualified binding identifies host and repository; token values stay in environment. Proposal and review IDs are interpreted inside that host identity. No application or tracker data migrates (learning/forgejo-native-task-code-and-lifecycle-identity).

### 5.5 Interfaces and integrations

`integrations.bindings.code_host` selects GitHub or Forgejo independently of `bindings.tasks`. Forgejo uses its authenticated REST client and native pull/review endpoints; Git pushes use the repository-scoped credential helper. GitHub keeps its adapter. The draft lifecycle consumes a neutral code-host seam for identity, proposal discovery/mutation, approval reads, and merged-PR resolution; Git remains the ref seam. The lock recorder selects the same binding instead of defaulting to GitHub event variables. Exact payloads and error checks belong to technical design.

### 5.6 Non-functional requirements

| ID | Area | Requirement | Fit criterion | Verify |
|---|---|---|---|---|
| NFR-1 | Security | Credentials stay scoped and absent from output. | Secret canary appears in no argv, URL, Git config value, log, error, PR body, event, tag, or compiled record. | Test |
| NFR-2 | Consistency | Consequential reads are fresh and complete. | Approval/merge use uncached exact reads, bounded complete pagination, and head recheck; uncertainty refuses. | Test |
| NFR-3 | Compatibility | Existing GitHub and Forgejo execution remain supported. | Source GitHub draft and native Forgejo code-host/tracker suites pass beside installed draft E2E. | Test |

### 5.7 Instrumentation — the tracking plan

Retain an installed run report with source version, release artifact hashes, each draft verb, PR/head/digest, actual operator review receipt, lock/mainline proof, plan and execution state, and available phase/token/cost fields. Count proposal numbers per draft version and false-lock negative fixtures. Reuse current draft events and lock payload rather than adding telemetry. Mark unavailable measurements explicitly; infer no cost number (problem:forgejo-feature-draft#A-1; design:feature-draft-v1 §3.4).

### 5.8 Definition of Done

| ID | Done criterion | Source | Verify | Required evidence | Blocking? |
|---|---|---|---|---|---|
| DOD-1 | Fresh installed Forgejo flow opens, iterates, prepares, and identifies one native PR without gh. | FR-1..FR-5, FR-7, UB-1..UB-3 | Test | Transcript with status/open/round/rebase, PR/head/digest, normal hook outcomes | yes |
| DOD-2 | Source GitHub and Forgejo task/code-host behavior do not regress. | NR-1..NR-3, NFR-3 | Test | Existing suites plus source GitHub draft conformance | yes |
| DOD-3 | Only real configured operator's latest authenticated exact-head review authorizes a lock. | FR-5/6, UB-2/4, ABUSE-1/2 | Test | Enforced pre-merge protection or an equivalent enforceable mechanism, positive human receipt, and unapproved/stale/agent/rejected/changed-head/privileged-bypass negative merges; independent post-merge review remains required; synthetic reviews only in tests | yes |
| DOD-4 | Lock tag and compiled contract are verified on mainline; recorder rerun is byte-idempotent. | FR-6, ABUSE-5 | Test | Fresh-clone ancestry, record digest/head, annotated tag/readback, rerun | yes |
| DOD-5 | Failures, ambiguity, origin mismatch, and credential canaries fail closed and recover. | UB-1..UB-6, NFR-1/2 | Test | HTTP/pagination/origin/hostile-redirect/kill/retry fixtures, with zero credential-bearing requests to unbound destinations, zero output-canary leaks, and zero false PR/tag | yes |
| DOD-6 | Final preparation, prepared-head publication, and lock pass full memory integrity and current-digest readiness; the opening/round policy defers only readiness. | FR-7; prd:feature-draft/NR-4; problem:forgejo-feature-draft#A-4 | Test | Passing complete memory gate with lossless problem/delivery bindings and current PRD/design reviews; missing-binding/invalid-evidence final-head refusals and intermediate readiness-only gate evidence | yes |

### 5.9 AI behavior and evals

Draft operations here are deterministic. Argus and Codex author/review the contract, but this correction introduces no model-driven product behavior. Synthetic review fixtures cannot stand in for actual human approval.

## 6. Delivery

### 6.1 Phasing and milestones

1. Human reviews and locks this draft's exact prepared PRD/tech-spec head.
2. Plan and implement neutral seam, native Forgejo adapter, approval/recorder path, and stage-correct source hook behavior.
3. Run code gate, independent implementation review, human merge, and verified release.
4. Resume installed AEC run after PR #1307 lands separately; report delivered or unfinished evidence.

### 6.2 Dependencies

Existing `integrations`, `forgejo.Client`, native code-host, and Git credential helper are reusable. The frozen `design:feature-draft-v1` §3.2 boundary is extended only if this draft locks. PR #1307 is a separate dependency for the full AEC run, not this diff. Local-Git code hosting remains independent (problem:forgejo-feature-draft#A-2).

### 6.3 Risks, assumptions, and rabbit holes

| ID | Risk / assumption / rabbit hole | Owner | Mitigation or test |
|---|---|---|---|
| R-1 | Forgejo review and merged-PR fields or pagination differ from GitHub. | implementer | Validate native response at installed version; malformed/missing identity refuses; installed test. |
| R-2 | Forgejo protection alone may not prove a specific operator review. | operator and implementer | Configure protection and independently read exact-head human review before tag. |
| R-3 | Source pre-push readiness blocks initial scaffold. | implementer | Stage-aware hook change under FR-7 with normal hook/prepare-lock regression. |
| R-4 | Operator login config is a provisional separate-identity choice. | Argus | delivery:forgejo-feature-draft#E-3; require explicit value and reject absent/equal login; operator may revise at lock review. |

### 6.4 Release criteria

DOD-1..DOD-6 must pass at the correction release candidate. The wider AEC acceptance run follows this release and PR #1307's separate merge; its outcome must be reported separately. Preparing or merging this contract cannot satisfy that run. No release tag is cut from an unmerged correction.

### 6.5 Rollout and reversibility

GitHub stays the source default. Forgejo activates only under its explicit code-host binding; incorrect/incomplete config refuses before host writes. Later releases may roll back the adapter without changing AEC application data. An already created lock remains immutable history read by its original version.

### 6.6 Launch readiness

The implementation proposal must carry installed proof, hook/CI parity, approval tests, tag/mainline verification, and release hashes. The operator checks its exact head before merge. This document requests only a future contract lock; no implementation or acceptance result is claimed.

### 6.7 Post-launch review

After the first AEC feature delivery, inspect the retained report for duplicate proposals, stale approvals, tag mismatches, leaks, and missing measurements. A new requirement or design change follows the next sanctioned draft version.

## 7. Alignment

### 7.1 FAQ

**Does Forgejo tracker selection force Forgejo code hosting?** No. The code-host binding is independent and must match Git remote (FR-1, NR-2).

**Can Argus or a test approve this draft?** No. Only an authenticated human review of the exact prepared head authorizes lock (FR-5/6, UB-4).

**Is the source-only design edited?** No. This proposal explicitly extends its scope after human lock (§4.2).

### 7.2 Open questions

none — operator A-1..A-3 sets the product perimeter. The exact Forgejo human login is an installation setting, deliberately required and fail-closed under Argus assumption E-3; it is not guessed here. Human approval of this contract remains pending, not an answered product question.

### 7.3 Decision log

| Date | Decision | Alternatives considered | Why | Who |
|---|---|---|---|---|
| 2026-09-30 | Native Forgejo compatibility extension, GitHub retained (brainstorm:forgejo-feature-draft#D-1/#D-2) | Forgejo-only replacement; parallel pipeline | Source stays GitHub; AEC stays Forgejo | operator A-1/A-2 |
| 2026-09-30 | Authenticated exact-head human lock (brainstorm:forgejo-feature-draft#D-3) | agent/synthetic approval | Existing FR-46/47 and operator A-1/A-3 | operator |
| 2026-09-30 | Explicit separate Forgejo operator login (brainstorm:forgejo-feature-draft#D-5) | infer owner; reuse agent | Avoid identity confusion | Argus assumption, tier-2 (delivery:forgejo-feature-draft#E-3) |
| 2026-09-30 | First-push and local-Git efforts separate (brainstorm:forgejo-feature-draft#D-4) | bundle adjacent work | One concern and prior ownership | operator A-2 |

### 7.4 Changelog

| Date | Change | Author |
|---|---|
| 2026-09-30 | Authored compatibility draft from operator intake and reproduced source seams. | Argus, authority-contract/v1 tier-2 |
| 2026-10-01 | Applied already-proposed P-1..P-4 corrections and companion acceptance evidence under operator-directed repair A-5; no final contract approval. | Argus recording operator instruction and bounded interpretation |

## 8. Contract readiness — the executability gate

| Check | Pass? | Notes |
|---|---|---|
| Problem basis accepted | yes | Operator A-1/A-2 fixes journey/scope; Argus stage-1 gate E-4 is not the lock. |
| Zero solution-blocking TBDs/open questions | yes | §7.2; R-4 is provisional with fail-closed outcome. |
| Every Must FR has fit criterion and verification | yes | FR-1..FR-7 have observable Test criteria. |
| Dangerous rows have safety plan | yes | UB-1..UB-6 and ABUSE-1..ABUSE-5. |
| Non-goals and non-regression named | yes | NG-1..NG-4, WR-1, NR-1..NR-3. |
| Every primary job covered | yes | J1→FR-1..FR-5; J2→FR-5/6; J3→FR-6. |
| Every key flow has failure coverage | yes | §4.3 and UB-1..UB-6. |
| Metrics have measurement plan | yes | §3.2 and §5.7 use retained installed evidence. |
| Blocking DoD traces to sources | yes | DOD-1..DOD-6 state source and evidence. |
| Risks and assumptions have owners | yes | §6.3 and delivery E-3. |
| Human judgment has owner and rubric | yes | Operator must approve exact prepared head; no synthetic review. |
| DoD complete and evidence sourced | yes | §5.8; the wider acceptance run is a downstream dependency in §6.2 and §6.4. |
