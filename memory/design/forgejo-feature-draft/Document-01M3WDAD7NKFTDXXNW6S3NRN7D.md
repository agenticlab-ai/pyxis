# Forgejo feature drafts — tech spec

This working design extends the existing feature-draft lifecycle to a configured Forgejo code host. The current PRD is its basis. The frozen feature-draft and reopen designs continue to govern compilation, states, numbering, and human locking; this proposal explicitly extends their Pyxis-only host perimeter. All seven live direction decisions were made by Argus in design-brainstorm:forgejo-feature-draft-tech-design and delivery:forgejo-feature-draft#E-29. Implementation and planning start only after this contract is human-locked.

### D-1 — A consumer-owned draft host with native adapters

**Decision:** Add a neutral, typed host port owned by internal/draft; keep the existing lifecycle, compiler, readiness, Git operations, and approval selector as the common core. Register GitHub and Forgejo adapters by provider without growing the task codehost port. (design-brainstorm:forgejo-feature-draft-tech-design#D-1/#ALT-2)

**Serves:** FR-1 · FR-2 · FR-3 · FR-4 · FR-5 · FR-6 · UB-3 · NR-1 · NR-2 · NR-3 · NFR-3

#### Concrete layout

Add internal/draft/host for the consuming interface and value types; add internal/adapters/draft/github and internal/adapters/draft/forgejo. Keep internal/adapters/draft/shell for the process runner. Commands resolve and inject the selected host and existing approval.Port; lifecycle/lock packages import consumer ports only, never native adapters or executor Task/Workdir. Existing task/tracker adapters and their ports are unchanged.

The proposed capability surface is:

```go
type Host interface {
    Identity(context.Context) (Actor, error)
    Proposals(context.Context, Query) ([]Proposal, error)
    Proposal(context.Context, int) (Proposal, error)
    Create(context.Context, NewProposal) (Proposal, error)
    Update(context.Context, int, ProposalChange) (Proposal, error)
    RequestReview(context.Context, int, string) error
    Protection(context.Context, string) (Protection, error)
}
```

Actor carries authenticated numeric ID/login. Proposal carries number, URL, author, state, draft flag, head/base repository identity, branch and full SHA, and merge commit. Query explicitly selects open/all history and exact branch/base/repository filters. NewProposal and ProposalChange carry title/body/base/head and intended draft state; adapters translate that intent into a supported native operation and return the observed state. Protection carries the effective branch/rule, approval eligibility/count, stale policy, privileged coverage, direct-push restrictions, and required check configuration; unavailable or ambiguous observations return a typed refusal. These are internal typed values; adapter HTTP wire types carry explicit field tags and bounded decoding. No token is a field of a serializable proposal or event.

A draft host constructor registry follows the existing integration-provider registration pattern. Each medium supplies a registration file and a command-side blank-import file of its own, so adding a future adapter is additive rather than editing a shared switch/list. Duplicate keys and unregistered selected providers refuse. Constructor output includes the host and an approval.Port reader for the same resolved repository. Configuration and selection do not fall back from Forgejo to gh. GitHub wraps its existing authenticated gh operations and exact-head reader; Forgejo uses the existing common Forgejo HTTP client with its additional draft checks.

Publish an adapter guide and one consumer-owned conformance kit alongside the port. It executes the same lifecycle cases against both production adapters using controlled transport and Git fixtures. A hermetic fake obeys the actual provider schemas, rather than implementing an invented draft API. The rejected shared-executor-port shape and its wider task blast radius are retained in design-brainstorm:forgejo-feature-draft-tech-design#ALT-1.

#### Observability

Use existing draft events and structured diagnostics for feature, verb, provider, bound repository, proposal number, expected/observed head, refusal code, and remedy. Provider is evidence, not a new stage. Preserve normal stdout/JSON command contracts and pending event reconciliation.

#### Security and privacy

The port exposes no agent approve/merge operation. Human approval remains the configured operator's native review. Native response text, tokens, full credential environments, and untrusted HTTP bodies never enter returned errors or logs. Capability discovery cannot silently select a different repository or host.

#### Verification

Run both adapters' conformance suites and existing draft lifecycle/lock/E2E packages; protect existing GitHub and Forgejo task/tracker tests. Assert no gh invocation in every Forgejo draft case. A new adapter registration fixture must load with additive files and no core provider switch change.

### D-2 — Validate the bound destination before credentials and authenticate the draft actor

**Decision:** Resolve the code-host binding independently, validate the configured Git remote offline before any credential-bearing API/Git operation, then authenticate the explicit draft agent and require a separately configured operator; contain credentials across API redirects and Git requests. (design-brainstorm:forgejo-feature-draft-tech-design#D-2)

**Serves:** FR-1 · FR-3 · UB-1 · UB-2 · NFR-1 · ABUSE-2 · ABUSE-3 · DOD-5

**Builds-on:** D-1

#### Concrete layout

Load integrations through the existing closed configuration parser and select integrations.bindings.code_host, never the tracker/task binding. Read cfg.Remote locally with git remote get-url; reject multiple conflicting fetch/push URLs, userinfo, query/fragment, unsafe escapes/traversal, and unsupported transports before resolving an authenticated client. Compare normalized scheme, server, effective port, installation base path, owner, and repository against the selected binding. Preserve only documented normalization, such as an optional final .git and trailing server slash; never collapse a different port/base path/repository into a match. For Forgejo, use HTTPS or the existing loopback-only HTTP exception. SSH/scp Forgejo transport is refused with an explicit HTTPS-origin remedy in this first implementation; it is not silently converted before validation. GitHub retains its currently supported transport through its adapter's identity validator.

Every network-capable draft verb starts through this shared validator, including status, ls-remote/fetch, preparation, and lock recording. Revalidate the bound remote before each consequential Git action. Only then read the configured token environment and construct the selected native client. All credential-bearing API endpoints must be on the bound server/base path and repository; identity endpoints are the explicit same-server /user exception. The draft HTTP client refuses write redirects and refuses all redirects that would carry authorization outside that allowed endpoint scope, including another repository on the same server. Pagination links undergo the same checks before any request; cycles, partial pages, malformed identity, and configured bounds refuse.

Use draft.agent_login and draft.agent_token_env for the actual author identity. Add the closed draft.operator_login setting for the Forgejo human. Require nonempty, valid, distinct logins; resolve authenticated /user before branch or PR creation and compare its login/ID with the declared agent. Operator eligibility is confirmed through native user/reviewer/protection observations, never inferred from repo owner or token login. GitHub preserves its existing configured/source operator selection; this proposal does not silently change that source workflow to the Forgejo operator.

Reuse internal/gitcredential for Git's credential protocol, path scoping, and authenticated login. When the draft token differs from the task connection token, create a child-only credential environment selecting that draft token for the already validated connection; do not rewrite the connection configuration, parent environment, task credential, or remote URL. Set credential.useHttpPath=true, clear ambient helpers for the operation, and use the Pyxis scoped helper without a token in argv. Set http.followRedirects=false for credential-bearing Git operations. The helper checks the same bound repository and authenticated draft actor before emitting credentials directly to Git. GitHub's current adapter credential route remains subject to the same pre-request validation boundary.

#### Observability

Refusals name the missing setting or mismatched destination component and repair, with secrets redacted. Successful identity evidence contains login and numeric ID, not a token. Never include the entire child environment or a credential-protocol response in diagnostic capture.

#### Security and privacy

Validation happens before authenticated reads as well as writes. A hostile remote receives neither a token-bearing Git attempt nor a token-bearing API call. The configured code-host actor cannot become the human approver merely by changing the token. Unexpected response bodies and HTTP headers are not copied into errors.

#### Verification

Use recording HTTP servers and fake Git/helper invocations for wrong server/port/base path/owner/repo/scheme, multiple URLs, userinfo, encoded traversal, missing binding/token, wrong actor, equal agent/operator, and cross-server/same-server-other-repo redirects. Assert zero credential-bearing requests and zero writes to every unbound destination. Canary tokens must be absent from stdout, stderr, events, logs, argv, URLs, files, and API error surfaces. Keep existing output-canary cases in addition to destination containment.

### D-3 — One proposal and the existing lifecycle state machine

**Decision:** Derive status and reconcile lifecycle mutations from complete selected-host observations, deterministic draft branch identity, exact working bytes, and leased remote heads; translate Forgejo draft intent through native title-prefix behavior and verify actual draft state. (design-brainstorm:forgejo-feature-draft-tech-design#D-3)

**Serves:** FR-2 · FR-3 · FR-4 · UB-3 · UB-5 · UB-6 · NR-3 · NFR-2 · ABUSE-4 · DOD-1

**Builds-on:** D-1 · D-2

#### Concrete layout

Retain lifecycle.States and its PRD-ledger precedence. Inspect combines configured PRD ledgers, verified remote lock versions, and selected-host proposals. Match same head/base repository, configured base branch, and the exact deterministic draft branch/version. A fork or duplicated matching proposal is an explicit ambiguity refusal, not a silently filtered successful state. No contract returns the existing explicit absent state; reopened N+1 still reports N as binding. Existing document-pipeline features remain document pipeline, without migration.

Open/Reopen retain their scaffold, clean-worktree, branch, numbering, and previous-lock-byte checks. Before creating a PR, enumerate matching proposals completely. Zero permits one creation; one is reconciled only if its repository/base/branch/head/author/draft intent agrees; more than one refuses. If a create response is lost or ambiguous, perform a bounded exact native enumeration/read before retry, never blindly POST a second proposal. A pushed branch with no confirmed PR remains a named recoverable draft event. Reopen's unfinished-task test still reads the independent task binding and does not replace that tracker with the code host.

Forgejo 16.0.5's actual Create/EditPullRequestOption has no draft write field. Use the configured native draft title prefix (existing WIP: default behavior) for opening/rounds, then exact-read and require observed PullRequest.draft=true. Preparation removes that prefix only after the final gates and protection checks, then requires draft=false. A custom server prefix must be declared through the existing provider setting; if the title does not produce the required observed state, refuse with a named prefix/capability remedy. No invented JSON draft field, body marker, or AGit draft option substitutes for native draft state.

Round uses the supported restore path to byte-restore working PRD/techspec and preserve the same PR. Read the expected remote SHA R and refuse movement; updates publish with an explicit branch lease and verify the known local SHA. An approved prepared head is invalidated by the new push through D-5's protection. Abandon retains its required reason Learning on the same proposal and existing review/abandon transition; it cannot simply delete the branch or lose the reason. Task-plan restoration and proposal identity checks remain as in the frozen reopen design.

#### Observability

Existing draft-open/reopen/round/abandon events retain feature/version/proposal/branch/head and pending-operation identity. A failed native write is not a success event. Cancellation preserves enough known local/remote identity for the existing queue to reconcile on resume.

#### Security and privacy

All native operations use D-2's destination/actor checks. Status does not interpret a fork, wrong base, arbitrary branch, or unverified tag as an authoritative draft. No agent review is used to satisfy abandonment's human obligation.

#### Verification

Exercise no contract, new draft, locked version, reopened draft with prior lock binding, and document-pipeline status. Test duplicate/fork/wrong-base/wrong-actor/wrong-head/missing PR, lost POST response, malformed/partial pagination, unsupported prefix, and moved-head lease refusal. Interrupt after scaffold, push, PR creation, and body update, then resume to one PR without losing exact working bytes or required abandonment reason. Retain GitHub lifecycle and independent tracker cases.

### D-4 — Prepare the exact rebased and compiled head

**Decision:** Keep the supported detached-worktree preparation and content-bound stamp refresh, require full final readiness/memory integrity, compile exact reviewed texts, then publish the locally known prepared head under its remote-head lease and update that same selected-host proposal. (design-brainstorm:forgejo-feature-draft-tech-design#D-4)

**Serves:** FR-5 · FR-7 · UB-3 · UB-5 · UB-6 · NFR-2 · ABUSE-4 · DOD-1 · DOD-6

**Builds-on:** D-2 · D-3 · D-5 · D-7

#### Concrete layout

Retain lock.Prepare's clean author-branch, local/remote equality, current-main fetch, original source-stamp verification, temporary detached worktree, internally authorized stamp-refreshing rebase, and complete readiness pipeline. The host seam replaces only native discovery/body/reviewer operations and supplies the already validated Git credential context. No manual rebase mode or external environment override can substitute for source binding validation.

Check readiness against the fetched mainline at the rebased tree with both latest reviews bound to the exact combined PRD/tech-spec digest. Compile through the existing draft compiler into literal PRD/design Documents and typed records, remove working draft files in the prepared landing tree, and run full final memory integrity over that exact tree. For reopened versions, retain the verified seed and existing version-relative retire/supersede rules. Readiness, compile, final memory gate, and D-5 protection must all pass before publication or making the proposal mergeable.

Let R be the observed old branch head and H the locally created final prepared commit. Push only refs/heads/<configured-draft-branch> with --force-with-lease=<that-ref>:R; Git cannot fall back to an unleased force. Refresh valid Argus source commits solely through the existing preparation mechanism. The generated compiler commit retains the existing system-authorship convention, not a fabricated Argus or human stamp. A lease or final gate refusal leaves the author branch/draft intact and records the refusal.

Bounded native/read-ref reconciliation must find H; an eventually consistent response does not redefine the expected head. Update the same PR's compact lock-request body with H, digest, mainline, compiled-record refs, and exact ruled changes. Only after D-5 protection is confirmed does the adapter transition its native draft state and request the configured separate human. A partial update/reviewer failure is a pending approval-request event for that same H, not a new PR, merge authorization, or successful preparation receipt. Cleanup/reaping uses the existing preparation worktree owner and queue; no new unowned scratch process or daemon is introduced.

#### Observability

Prepared outcome and existing event identify R, H, mainline, digest, PR, and requested human. Retain exact checked/published revision rather than a head rediscovered through status. Final gate output preserves every unmet condition and remedy.

#### Security and privacy

No synthetic review, body assertion, successful CI badge, or agent identity grants human approval. A new publication must invalidate any older review. A-4 is an intermediate source-document exception and is unavailable to this final path.

#### Verification

Exercise current-main advancement, valid/replayed/stale Argus trailers, wrong parent/tree binding, missing/later-stale reviews, changed contract digest, moved remote head, interruption and stale post-push readback. Verify the rebased bindings, exact compiled text/digest, H on the real remote, same PR, and separate reviewer request. Inject final memory/readiness/protection failures and assert no prepared publication or mergeable transition.

### D-5 — Enforce the human gate before merge

**Decision:** Require an observed effective branch policy that enforces the configured human's current-head approval, stale dismissal, rejection blocking, and privileged coverage before making prepared contracts mergeable; refuse unsupported protection with a concrete remedy and retain independent post-merge verification. (design-brainstorm:forgejo-feature-draft-tech-design#D-5)

**Serves:** FR-5 · UB-2 · UB-4 · ABUSE-1 · ABUSE-2 · DOD-3

**Builds-on:** D-1 · D-2

#### Concrete layout

The Forgejo draft adapter reads the base branch's effective protection rather than reusing the task adapter's narrower protection model. Its wire model includes required_approvals, enable_approvals_whitelist, approvals_whitelist_username (the actual singular Swagger key), dismiss_stale_approvals, ignore_stale_approvals, block_on_rejected_reviews, apply_to_admins, push/merge whitelists, status checks, and protected/unprotected patterns. Require at least one approval exclusively eligible from the separately configured human, dismissal after new commits, no ignored stale approval, rejection blocking, privileged enforcement, and no direct-push or file-pattern bypass for compiled records. Required final checks must apply to the same prepared head. Multiple matching rules or precedence that cannot be proven effective are refused.

The server enforces this policy on UI/API merges and Git mainline writes; the agent does not perform a merge. Read-only preparation verifies the current policy and names settings the operator must fix; it neither grants permissions nor auto-disables protection. Existing GitHub preparation keeps its code-owner approval, dismiss-stale, latest-push, admin-bound protection semantics. Platform capability support must be established in disposable native fixtures for the supported version, including any privileged/forced-merge endpoint. Merely exposing apply_to_admins or a whitelist field in Swagger is insufficient. If the platform cannot enforce the rule, leave the proposal unprepared and name the missing enforcement capability and remedy; do not call post-merge tag refusal an equivalent mechanism.

Capture normalized policy observations with the tested provider version and the prepared receipt. Any relevant setting change requires renewed policy verification and fresh head review through the normal round/preparation path. This is within the existing trusted-server/governance boundary: an operator's later intentional removal of protections is a governance change, not an agent permission to bypass the recorded policy.

#### Observability

Protection refusals name the effective branch/rule and missing human, stale, privileged, direct-push, or check condition. Successful inspection is policy evidence, explicitly separate from an actual human review or runtime acceptance result.

#### Security and privacy

Agent, unlisted user/team, stale review, and privileged force attempts cannot put compiled contract records on mainline. No operator token is requested or reused by the agent. Review request/eligibility does not count as approval.

#### Verification

In disposable Forgejo and GitHub fixtures, attempt merge with no review, agent review, another human, pending/comment-only review, rejection, approval followed by a new push, and privileged bypass; verify mainline contains no compiled records in every negative case. Fresh configured-human approval of exact H permits the positive test merge. Tests use declared synthetic identities; actual source and application approvals must be real operator actions. A changed rule, ambiguous rule, inaccessible policy, or unsupported forced-merge behavior refuses preparation.

### D-6 — Verify the merged artifact and publish a byte-idempotent lock

**Decision:** Select exactly one native merged proposal, verify the configured human's current-head review independently, establish mainline ancestry and exact compiled-record content/digest through Git, and publish or raw-byte-verify the annotated lock tag with race reconciliation. (design-brainstorm:forgejo-feature-draft-tech-design#D-6)

**Serves:** FR-6 · UB-2 · UB-3 · UB-4 · UB-5 · UB-6 · NFR-2 · ABUSE-1 · ABUSE-5 · DOD-3 · DOD-4

**Builds-on:** D-2 · D-4 · D-5

#### Concrete layout

cmd/draft-lock-record resolves the selected code-host binding and configured credentials instead of universally requiring GH_TOKEN or a hardcoded GitHub reader. Keep the existing push-range discovery and lock schema; selected native observations find exactly one same-repository PR with the configured base, draft version branch, prepared head H, and merge commit M. Enumerate all relevant pages before choosing; zero/multiple/fork/wrong-base/wrong-merge cases refuse without a tag. Repository identities are verified before consuming host fields.

Add a Forgejo approval.Port reader. Exact-read the proposal's head before and after complete review enumeration; both must be H, with merged association M unchanged. Map native review ID/user login+ID/state/commit_id/body and dismissed/stale flags into the existing approval.Select predicate. Dismissed/stale observations cannot normalize to APPROVED. Unknown/malformed review state or actor, partial pagination, changed head, missing commit ID, or conflicting latest review fails closed. Only the configured operator's latest submitted approving review for H counts; a newer rejection/dismissal blocks it, while comments/pending drafts do not replace an otherwise valid submitted approval, as in the shared selector. Preserve the exact optional capture body and explicit lineless case.

Fetch the configured mainline using D-2's Git context, require M to be its ancestor, and read the merged compiled PRD/design Documents and their typed records directly from M. Validate the compiled graph, active records, exact stored text, approved-head/readiness trailer, and recomputed combined digest using the same digest routine as draft compilation. Check those texts/digest against the verified prepared H, rather than reconstructing approval from the platform badge. Squash merges are supported by checking H's prepared contents against M and M's mainline ancestry; H itself need not be an ancestor after squash. Existing version-relative unchanged records must resolve from the validated prior lock, without deleting/replacing prior obligations.

Construct the deterministic draft-lock/v1 annotation through the existing serializer, retaining feature/version, H, M, digest, real PR/review/operator, capture/lineless fields, and proposal count. Publish the annotated tag at M through Git. Existing tags must be annotated, target M, and have the exact annotation bytes produced by that serializer; decoded-field equality alone is insufficient. Fetch and read back the remote tag/object after publication. If another recorder wins a non-force tag race, fetch its tag and succeed only if target and raw annotation bytes match; otherwise refuse a conflict. Never overwrite or advance a conflicting lock version.

#### Observability

Record exact proposal/head/review/mainline/digest/tag receipt only after readback. Failures identify the PR, missing proof, or conflicting tag without emitting token or untrusted response body. A lock is not delivered merely because native merge status is true.

#### Security and privacy

Post-merge review, content and ancestry are independent of the pre-merge policy check. No synthetic acceptance can authorize a real lock. A tag that points to a side branch, stale content, or conflicting annotation cannot authorize planning.

#### Verification

Exercise no/multiple/fork/wrong-base proposals, stale/agent/unknown/dismissed/rejected reviews, head movement during review pagination, mismatched typed records/text/digest, nonexistent or non-ancestor M, valid squash, malformed/lightweight/conflicting tags, annotation-byte differences, repeated invocation, and identical/conflicting racing recorders. Verify from a fresh clone that only the valid mainline artifact yields one annotated lock with exact H/digest/review/PR/M.

### D-7 — Stage-aware hooks with complete final integrity and readiness

**Decision:** Extend the existing source push gate to observe working-draft authoring stage and defer only final draft-ready there; required PR readiness continues to judge the same contract predicate, and every preparation, prepared-head push, and lock uses complete memory integrity plus current-digest readiness. (design-brainstorm:forgejo-feature-draft-tech-design#D-7)

**Serves:** FR-7 · UB-5 · UB-6 · DOD-6 · NR-1 · NFR-3

**Builds-on:** D-1

#### Concrete layout

Use the existing code-gate check inventory and readiness implementation. The hook derives an intermediate authoring stage only from a configured draft branch and that feature's actual working draft files, without a compiled prepared landing tree. An ambiguous/mixed stage refuses rather than accepting an author-supplied skip flag. Opening and round pushes run every other applicable check, including memory integrity and attribution; the production policy is not git push --no-verify. Prepared trees cannot select that deferral because compilation removes the working files and publishes the contract-record tree.

Required PR draft-ready still produces the shared verdict, including unmet conditions while a proposal is intentionally unfinished; deferring an intermediate hook does not turn an incomplete proposal into a green merge candidate. The final hook and preparation path run that same readiness predicate against current mainline, latest PRD and tech-spec reports at the current combined digest, all Must coverage, answered product problems, valid links/relations, and blocking dispositions. There is no independent weaker CI readiness definition.

Complete memory integrity remains required at intermediate production pushes and all final paths. Lossless problem/delivery bindings, evidence validity, trusted policy, and the native compiled PRD/design records must satisfy the existing memory gate. The current source drive's first-PR scope conflict is a prerequisite that must be resolved through a sanctioned human/bootstrap or separately reviewed compatibility route before final publication; this design grants no memory/problem scope extension, debt/baseline mutation, stamp removal, relabeling, or suppressed binding check. A missing binding or out-of-scope capture refuses the final path with the actual remedy. Problem A-4 remains only this source draft's temporary intermediate-document publishing exception and does not implement or test the production policy.

Ship a fresh verified release from reviewed mainline after implementation, verify its manifest and installed artifacts, and repeat the lifecycle from an installed Forgejo repository with no gh available. Installed demo, GitHub source conformance, native protection tests, fresh-clone tag verification, and normal hook transcripts provide DOD-1..DOD-6 evidence; contract preparation alone supplies none of that implementation evidence.

#### Observability

Gate results retain stage, exact checked tree/head/base, check names, and complete unmet/remedy output. Record readiness deferral as an explicit intermediate-stage event. Never represent a skipped check as a pass or a diagnostic compiler binary as an installed release.

#### Security and privacy

No caller flag, body claim, branch rename alone, or earlier passing digest may waive final readiness or memory integrity. Current full source attribution and protected-path checks remain in force. Approval and human merge remain outside the agent's gate permissions.

#### Verification

Test open/round stage detection, branch/file disagreement, mixed compiled/working trees, normal intermediate pushes with only readiness deferred, and all remaining checks invoked. Test final preparation/prepared push/lock with missing lossless bindings, invalid evidence, stale reviews, changed digest, uncovered Must rows, and invalid Argus attribution; every negative must refuse before publication. The positive final head passes the complete memory gate and identical local/CI readiness predicate. Run the installed end-to-end proof and existing code-gate/CI parity suites; retained historical failures are not new passes.
