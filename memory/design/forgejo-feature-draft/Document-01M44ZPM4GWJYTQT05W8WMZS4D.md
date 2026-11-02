# Forgejo feature drafts — tech spec

This working design extends the existing feature-draft lifecycle to a configured Forgejo code host. The current PRD is its basis. The frozen feature-draft and reopen designs continue to govern compilation, states, numbering, and human locking; this proposal explicitly extends their Pyxis-only host perimeter and adopts the operator-approved installed-Forgejo trust model recorded in draft problem P-9. P-9 supersedes the inherited F-5 protection direction for Forgejo only; existing GitHub approval/protection behavior remains binding. The seven v1 directions remain attributed prior facts in the published design-brainstorm working file drafts/forgejo-feature-draft/brainstorm/003.md (design-brainstorm:forgejo-feature-draft-tech-design#F-1; delivery:forgejo-feature-draft#E-29). This v2 adds that record's Host repository-policy extension (design-brainstorm:forgejo-feature-draft-tech-design#D-1), preserving the existing abandonment precondition, and makes the existing one-proposal and latest-submitted-review guarantees explicit within the same lifecycle and approval seams. Implementation and planning start only after this contract is human-locked.

### D-1 — A consumer-owned draft host with native adapters

**Decision:** Add a neutral, typed host port owned by internal/draft; keep the existing lifecycle, compiler, readiness, Git operations, and approval selector as the common core. Register GitHub and Forgejo adapters by provider without growing the task codehost port. The same Host exposes the read-only repository-policy query that supplies abandonment's affirmative auto-delete precondition. (design-brainstorm:forgejo-feature-draft-tech-design#F-1; design-brainstorm:forgejo-feature-draft-tech-design#F-9; design-brainstorm:forgejo-feature-draft-tech-design#D-1)

**Serves:** FR-1 · FR-2 · FR-3 · FR-4 · FR-5 · FR-6 · UB-3 · NR-1 · NR-2 · NR-3 · NFR-3

#### Concrete layout

Add internal/draft/host for the consuming interface and value types; add internal/adapters/draft/github and internal/adapters/draft/forgejo. Keep internal/adapters/draft/shell for the process runner. Commands resolve and inject the selected host and existing approval.Port; lifecycle/lock packages import consumer ports only, never native adapters or executor Task/Workdir. Existing task/tracker adapters and their ports are unchanged.

The proposed capability surface is:

```go
type Host interface {
    Identity(context.Context) (Actor, error)
    Proposals(context.Context, Query) ([]Proposal, error)
    Proposal(context.Context, int) (Proposal, error)
    RepositoryPolicy(context.Context) (RepositoryPolicy, error)
    Create(context.Context, NewProposal) (Proposal, error)
    Update(context.Context, int, ProposalChange) (Proposal, error)
    RequestReview(context.Context, int, string) error
}

type RepositoryPolicy struct {
    AutoDeleteHeadBranchesOnMerge bool
}
```

Actor carries authenticated numeric ID/login. Proposal carries number, URL, author, state, draft flag, head/base repository identity, branch and full SHA, merge commit, exact body/title, and the completely observed requested-reviewer identities. Missing or incomplete reviewer membership is an error, never an empty-list assertion. Query explicitly selects open/all history and exact branch/base/repository filters. NewProposal and ProposalChange carry title/body/base/head and intended draft state; adapters translate that intent into a supported native operation and return the observed state. GitHub's existing protection handling remains within its adapter; the shared Host carries no new protection-policy model. These are internal typed values; adapter HTTP wire types carry explicit field tags and bounded decoding. No token is a field of a serializable proposal or event.

RepositoryPolicy observes the bound repository's automatic head-branch-deletion setting through this same Host. GitHub maps the native boolean delete_branch_on_merge; Forgejo maps the native boolean default_delete_branch_after_merge (design-brainstorm:forgejo-feature-draft-tech-design#F-10; design-brainstorm:forgejo-feature-draft-tech-design#F-11). Native wire decoding distinguishes absent/malformed fields from an explicit false value: missing, wrong-type, inaccessible, failed or ambiguous observations return an error, never a guessed default. An unambiguous false returns AutoDeleteHeadBranchesOnMerge=false. The operation is read-only, uses D-2's bound-destination/actor checks, and creates no new port, setting write or host fallback.

A draft host constructor registry follows the existing integration-provider registration pattern. Each medium supplies a registration file and a command-side blank-import file of its own, so adding a future adapter is additive rather than editing a shared switch/list. Duplicate keys and unregistered selected providers refuse. Constructor output includes the host and an approval.Port reader for the same resolved repository. Configuration and selection do not fall back from Forgejo to gh. GitHub wraps its existing authenticated gh operations and exact-head reader; Forgejo uses the existing common Forgejo HTTP client with its additional draft checks.

Publish an adapter guide and one consumer-owned conformance kit alongside the port. It executes the same lifecycle cases against both production adapters using controlled transport and Git fixtures. A hermetic fake obeys the actual provider schemas, rather than implementing an invented draft API. The rejected shared-executor-port shape and its wider task blast radius are retained as an attributed v1 alternative in design-brainstorm:forgejo-feature-draft-tech-design#F-8; the same-seam repository-policy extension is selected by design-brainstorm:forgejo-feature-draft-tech-design#ALT-1.

#### Observability

Use existing draft events and structured diagnostics for feature, verb, provider, bound repository, proposal number, expected/observed head, refusal code, and remedy. Provider is evidence, not a new stage. Preserve normal stdout/JSON command contracts and pending event reconciliation.

#### Security and privacy

The port exposes no agent approve/merge operation. Human approval remains the configured operator's native review. Native response text, tokens, full credential environments, and untrusted HTTP bodies never enter returned errors or logs. Capability discovery cannot silently select a different repository or host.

#### Verification

The common conformance kit covers repository-policy true, false, absent, wrong-type, inaccessible and ambiguous native observations for both adapters. Assert correct native field mapping, typed refusal for unknown observations, and no policy writes. Run both adapters' conformance suites and existing draft lifecycle/lock/E2E packages; protect existing GitHub and Forgejo task/tracker tests. Assert no gh invocation in every Forgejo draft case. A new adapter registration fixture must load with additive files and no core provider switch change.

### D-2 — Validate the bound destination before credentials and authenticate the draft actor

**Decision:** Resolve the code-host binding independently, validate the configured Git remote offline before any credential-bearing API/Git operation, then authenticate the explicit draft agent and require a separately configured operator; contain credentials across API redirects and Git requests. (design-brainstorm:forgejo-feature-draft-tech-design#F-2)

**Serves:** FR-1 · FR-3 · UB-1 · UB-2 · NFR-1 · ABUSE-2 · ABUSE-3 · DOD-5

**Builds-on:** D-1

#### Concrete layout

Load integrations through the existing closed configuration parser and select integrations.bindings.code_host, never the tracker/task binding. Read cfg.Remote locally with git remote get-url; reject multiple conflicting fetch/push URLs, userinfo, query/fragment, unsafe escapes/traversal, and unsupported transports before resolving an authenticated client. Compare normalized scheme, server, effective port, installation base path, owner, and repository against the selected binding. Preserve only documented normalization, such as an optional final .git and trailing server slash; never collapse a different port/base path/repository into a match. For Forgejo, use HTTPS or the existing loopback-only HTTP exception. SSH/scp Forgejo transport is refused with an explicit HTTPS-origin remedy in this first implementation; it is not silently converted before validation. GitHub retains its currently supported transport through its adapter's identity validator.

Every network-capable draft verb starts through this shared validator, including status, ls-remote/fetch, preparation, and lock recording. Revalidate the bound remote before each consequential Git action. Only then read the configured token environment and construct the selected native client. All credential-bearing API endpoints must be on the bound server/base path and repository; identity endpoints are the explicit same-server /user exception. The draft HTTP client refuses write redirects and refuses all redirects that would carry authorization outside that allowed endpoint scope, including another repository on the same server. Pagination links undergo the same checks before any request; cycles, partial pages, malformed identity, and configured bounds refuse.

Use draft.agent_login and draft.agent_token_env for the actual author identity. Add the closed draft.operator_login setting for the Forgejo human. Require nonempty, valid, distinct logins; resolve authenticated /user before branch or PR creation and compare its login/ID with the declared agent. Operator identity is confirmed through native user and review observations, never inferred from repo owner or token login. GitHub preserves its existing configured/source operator selection; this proposal does not silently change that source workflow to the Forgejo operator.

Reuse internal/gitcredential for Git's credential protocol, path scoping, and authenticated login. When the draft token differs from the task connection token, create a child-only credential environment selecting that draft token for the already validated connection; do not rewrite the connection configuration, parent environment, task credential, or remote URL. Set credential.useHttpPath=true, clear ambient helpers for the operation, and use the Pyxis scoped helper without a token in argv. Set http.followRedirects=false for credential-bearing Git operations. The helper checks the same bound repository and authenticated draft actor before emitting credentials directly to Git. GitHub's current adapter credential route remains subject to the same pre-request validation boundary.

#### Observability

Refusals name the missing setting or mismatched destination component and repair, with secrets redacted. Successful identity evidence contains login and numeric ID, not a token. Never include the entire child environment or a credential-protocol response in diagnostic capture.

#### Security and privacy

Validation happens before authenticated reads as well as writes. A hostile remote receives neither a token-bearing Git attempt nor a token-bearing API call. The configured code-host actor cannot become the human approver merely by changing the token. Unexpected response bodies and HTTP headers are not copied into errors.

#### Verification

Use recording HTTP servers and fake Git/helper invocations for wrong server/port/base path/owner/repo/scheme, multiple URLs, userinfo, encoded traversal, missing binding/token, wrong actor, equal agent/operator, and cross-server/same-server-other-repo redirects. Assert zero credential-bearing requests and zero writes to every unbound destination. Canary tokens must be absent from stdout, stderr, events, logs, argv, URLs, files, and API error surfaces. Keep existing output-canary cases in addition to destination containment.

### D-3 — One proposal and the existing lifecycle state machine

**Decision:** Derive status and reconcile lifecycle mutations from complete selected-host observations, deterministic draft branch identity, exact working bytes, and leased remote heads; translate Forgejo draft intent through native title-prefix behavior and verify actual draft state. Abandonment requires affirmative repository policy before its mutations. (design-brainstorm:forgejo-feature-draft-tech-design#F-3)

**Serves:** FR-2 · FR-3 · FR-4 · UB-3 · UB-5 · UB-6 · NR-3 · NFR-2 · ABUSE-4 · DOD-1

**Builds-on:** D-1 · D-2

#### Concrete layout

Retain lifecycle.States and its PRD-ledger precedence. Inspect combines configured PRD ledgers, verified remote lock versions, and selected-host proposals. Match same head/base repository, configured base branch, and the exact deterministic draft branch/version. A fork or duplicated matching proposal is an explicit ambiguity refusal, not a silently filtered successful state. Before returning the absent state or authorizing Create, Inspect also reads the bound remote draft branch's pending-operation evidence. A pending or unconfirmed earlier create attempt requires reconciliation, never an absent/no-contract result; an inaccessible or incomplete intent read refuses. With neither a contract nor a pending attempt, retain the existing explicit absent state; reopened N+1 still reports N as binding. Existing document-pipeline features remain document pipeline, without migration.

Open/Reopen retain their scaffold, clean-worktree, branch, numbering, and previous-lock-byte checks. Before creating a PR, enumerate matching proposals completely. One is reconciled only if its repository/base/branch/head/author/draft intent agrees; more than one refuses. Zero permits an initial creation only when no earlier creation for that feature/version is confirmed or unresolved. Establish one pending-create intent in the existing draft events on the owned draft branch, committed and published under the known remote-head lease before sending Create. It records feature/version, bound repository, actor, head/base branches and source head; its introducing Git commit supplies the attempt identity without a self-referential hash. A restart or fresh clone must recover that shared intent rather than allocate another attempt.

A lost response, cancellation after possible dispatch, or otherwise ambiguous Create leaves that intent pending. Recovery performs bounded exact native enumeration/read and reuses the single matching observed proposal; a temporarily empty or eventually stale enumeration is not an authoritative negative observation and never permits a second POST. The host adapter and its underlying HTTP client must not implicitly retry an ambiguous Create either. Another Create is allowed only after provider-specific evidence proves the earlier attempt did not create a proposal, or a fixture-verified provider uniqueness/idempotency guarantee covers the complete one-proposal-per-version obligation. The initial implementation assumes neither guarantee: unresolved outcomes remain a named pending reconciliation with read-only retries and a remedy, never an absent/no-contract success. A definite negative must be evidenced, not inferred from a generic error, timeout or empty list. A pushed branch with no confirmed PR retains its exact working bytes and pending event. Reopen's unfinished-task test still reads the independent task binding and does not replace that tracker with the code host.

Forgejo 16.0.5's actual Create/EditPullRequestOption has no draft write field. Use the configured native draft title prefix (existing WIP: default behavior) for opening/rounds, then exact-read and require observed PullRequest.draft=true. Preparation removes that prefix only after the final gates and D-5's provider-specific preparation conditions, then requires draft=false. A custom server prefix must be declared through the existing provider setting; if the title does not produce the required observed state, refuse with a named prefix/capability remedy. No invented JSON draft field, body marker, or AGit draft option substitutes for native draft state.

Round uses the supported restore path to byte-restore working PRD/techspec and preserve the same PR. Read the expected remote SHA R and refuse movement; updates publish with an explicit branch lease and verify the known local SHA. A new prepared head makes older-head reviews ineligible through the exact-head selector in D-5/D-6; existing GitHub stale-dismissal behavior is retained. Abandon retains its required reason Learning on the same proposal and existing review/abandon transition; it cannot simply delete the branch or lose the reason. Task-plan restoration and proposal identity checks remain as in the frozen reopen design.

Before abandonment writes its reason Learning, pushes the draft branch or updates the proposal, the common lifecycle queries Host.RepositoryPolicy and requires AutoDeleteHeadBranchesOnMerge=true. An explicit false or query error refuses before those mutations and preserves the prior draft/PR state. This retains GitHub's existing fail-closed precondition for both native adapters; every other reason/review/transition condition remains required (design-brainstorm:forgejo-feature-draft-tech-design#D-1; design-brainstorm:forgejo-feature-draft-tech-design#F-10).

#### Observability

Existing draft-open/reopen/round/abandon events retain feature/version/proposal/branch/head and pending-operation identity. A failed native write is not a success event. Cancellation preserves enough known local/remote identity for the existing queue to reconcile on resume.

#### Security and privacy

All native operations use D-2's destination/actor checks. Status does not interpret a fork, wrong base, arbitrary branch, or unverified tag as an authoritative draft. No agent review is used to satisfy abandonment's human obligation.

#### Verification

Exercise no contract, new draft, locked version, reopened draft with prior lock binding, and document-pipeline status. Test duplicate/fork/wrong-base/wrong-actor/wrong-head/missing PR, lost POST response, malformed/partial pagination, unsupported prefix, and moved-head lease refusal. Use a real-schema fixture in which the first POST creates the PR, its response is lost, and the next complete enumeration temporarily returns zero: same-process recovery, restart and fresh-clone recovery must retain the pending intent and perform exactly one total POST, then reuse that PR when it becomes observable. Interrupt after pending-intent publication but before dispatch; an uncertain recovery remains pending and sends no second POST. Exercise a proved definite-negative outcome separately from generic HTTP/transport failures, and assert no hidden HTTP-client write retry. Interrupt after scaffold, push, PR creation, and body update, then resume to one PR without losing exact working bytes or required abandonment reason. For abandonment, exercise repository-policy true, false, absent, malformed and inaccessible responses; false/error cases must perform no Learning, branch or proposal mutation, and true still requires the existing reason/review/transition checks. Retain GitHub lifecycle and independent tracker cases.

### D-4 — Prepare the exact rebased and compiled head

**Decision:** Keep the supported detached-worktree preparation and content-bound stamp refresh, require full final readiness/memory integrity, compile exact reviewed texts, then publish the locally known prepared head under its remote-head lease and update that same selected-host proposal. (design-brainstorm:forgejo-feature-draft-tech-design#F-4)

**Serves:** FR-5 · FR-7 · UB-3 · UB-5 · UB-6 · NFR-2 · ABUSE-4 · DOD-1 · DOD-6

**Builds-on:** D-2 · D-3 · D-5 · D-7

#### Concrete layout

Retain lock.Prepare's clean author-branch, current-main fetch, original source-stamp verification, temporary detached worktree, internally authorized stamp-refreshing rebase, and complete readiness pipeline. Before applying the normal local/remote equality requirement, fetch and reconcile the preparation intent described below; only its independently verified R-to-H transition permits the recovery exception. With no valid intent, equality remains required. The host seam replaces native discovery/body/reviewer operations and supplies the already validated Git credential context. No manual rebase mode or external environment override can substitute for source binding validation.

Check readiness against the fetched mainline at the rebased tree with both latest reviews bound to the exact combined PRD/tech-spec digest. Compile through the existing draft compiler into literal PRD/design Documents and typed records, remove working draft files in the prepared landing tree, and run full final memory integrity over that exact tree. For reopened versions, retain the verified seed and existing version-relative retire/supersede rules. Readiness, compile, final memory gate, and D-5's provider-specific preparation conditions must all pass before publication or making the proposal mergeable.

Let R be the clean original author/observed remote head, B the exact fetched mainline, S the supported rebased source head, D the combined contract digest, and H the locally created final prepared commit. Push only refs/heads/<configured-draft-branch> with --force-with-lease=<that-ref>:R; Git cannot fall back to an unleased force. Refresh valid Argus source commits solely through the existing preparation mechanism. The generated compiler commit retains the existing system-authorship convention, not a fabricated Argus or human stamp. A lease or final gate refusal leaves the author branch/draft intact and records the refusal.

Before publishing H, create and remote-read-back a typed draft-preparation-intent/v1 sidecar on refs/pyxis/draft-preparations/<feature>/vN. Its operation key is a deterministic hash of the validated provider/repository identity, feature/version, PR number, branch, R, B, D and configured operator. Store the immutable identity fields, R/S/B/H, expected remote R, compiled-record references, exact lock-request body bytes and their SHA256, requested operator identity, and a phase from prepared, head-observed, body-confirmed, ready-confirmed, reviewer-dispatching, reviewer-confirmed or complete. Unknown schema, fields, phase, identity or missing content refuses recovery. At most one unresolved operation may exist for that version; another operation cannot silently replace it.

Reuse internal/gitrefstore's fetched evidence-ref transport, isolated index, system-authored metadata commits, exact reads and leased compare-and-swap updates. Add a consumer-specific retained-object initialization capability: the first metadata commit for an operation retains both R and H as additional Git parents, with later metadata commits preserving the prior evidence history. H already retains S/B through its normal prepared ancestry. The metadata tree contains only intent/body evidence; these metadata commits never enter the draft branch or change authoring content. The existing store does not yet support extra retained parents, so this is an explicit implementation extension, not an assumed API. It ensures a fresh clone fetching the intent ref obtains the original and prepared commits even after branch replacement or scratch reaping. Create the sidecar after H exists; neither H nor its compiled event text contains a self-referential H value. Keep the closed draft-event/v1 schema unchanged: existing draft_step_failed with step, error_class and version digest supplies failure diagnostics, while the typed sidecar supplies preparation identity. No new event name is invented.

Re-entry exact-fetches the intent generation, retained R/S/H and bound draft/mainline refs, then exact-reads the same native PR. Verify D-2's repository/actor/operator identities, branch/version/PR association, unchanged B, clean local R or H, original R stamps, supported rebased S bindings, current-digest reviews/readiness, compiled H records/text/digest and final memory integrity. An intent is recovery evidence, never a gate waiver or approval. A changed mainline, unrelated local/remote head, different actor/PR/digest, missing retained object or uncertain read refuses with the known heads and preserves all working bytes and evidence. A fresh clone can reconstruct a clean author checkout at retained R through the existing worktree owner before reconciliation; it must not guess source bytes from H.

If the actual remote still equals R, reuse the exact retained H after those checks and retry only the original R lease. If it equals H while the clean author checkout is still R, skip compilation and branch publication; reconcile the remaining host effects for that exact H and same PR. A lost push response is resolved by exact bound-ref readback, never by producing another H. No other remote head is accepted. CAS phase updates cannot overwrite another operation; a lost update response is resolved by fetching the exact current metadata generation.

Bounded native/read-ref reconciliation must find H; an eventually consistent response does not redefine the expected head. Exact-read before and after each subsequent effect. Update the same PR's compact lock-request body with the retained exact bytes naming H, D, B, compiled-record refs and ruled changes, then persist body-confirmed only after exact body readback. If the body already matches, skip that write. After D-5's preparation conditions pass, transition the native draft state, verify draft=false on H, and persist ready-confirmed; an already observed ready state needs no repeat transition.

Read complete native requested-reviewer membership through the same Host. An observed request for the configured operator, or that operator's authenticated submitted review of H through the existing approval reader, fulfills the request step; it does not authorize the lock or presume an approving verdict. Otherwise, durably persist reviewer-dispatching before the first request. Verify native membership afterward before recording reviewer-confirmed. A lost response with no positive native observation remains pending and permits bounded read-only retries; a stale/empty list never permits a second uncertain request. A proved undispatched/definite-negative outcome can resume the same request only with provider-specific evidence. No adapter or HTTP client silently retries an ambiguous write.

Complete only after exact H, body, ready state and reviewer fulfillment are verified; then align the clean author branch to retained H using the supported owner and record the successful preparation receipt. A failure or crash at any boundary retains the same intent, H and PR, rather than declaring preparation successful. Cleanup/reaping uses the existing preparation worktree owner and queue; retained-object evidence survives scratch cleanup until existing terminal lifecycle cleanup permits removal. No new process, daemon or host port is introduced. These engineering details respond to draft review010 FND-2 under existing UB-6/DOD-5; their basis is recorded in brainstorm/003's engineering elaborations.

#### Observability

Preparation diagnostics and typed intent identify R, S, H, B, D, PR, requested human and the last verified phase. Existing failure events retain their existing fields. Retain exact checked/published revision rather than a head rediscovered through status. Final gate output preserves every unmet condition and remedy; an unknown host effect remains pending.

#### Security and privacy

No synthetic review, body assertion, successful CI badge, or agent identity grants human approval. A new publication must invalidate any older review. A-4 is an intermediate source-document exception and is unavailable to this final path.

#### Verification

Exercise current-main advancement, valid/replayed/stale Argus trailers, wrong parent/tree binding, missing/later-stale reviews, changed contract digest, moved remote head, interruption and stale post-push readback. Verify the rebased bindings, exact compiled text/digest, H on the real remote, same PR, and separate reviewer request. Kill after intent publication, branch push, body update, ready transition, reviewer dispatch and final confirmation, including loss of each response and phase update; resume in a new process and fresh clone using the actual retained ref. Assert the same H and PR, exact original working bytes retained, no second branch publication when remote H is observed, no duplicate effective body/ready transition and no second uncertain reviewer request. Exercise incomplete/malformed intents, missing objects, unrelated heads, changed B/D/actor/PR, racing CAS writers and scratch reaping; unsafe cases retain evidence and refuse. A valid remote-R retry reuses H under the original lease. Inject final memory/readiness failures and inherited GitHub protection failures and assert no prepared publication or ready transition.

### D-5 — Exact-head human approval under the selected host's trust model

**Decision:** For installed Forgejo, use the operator-approved trusted-repository model in draft problem P-9: prepare and request review of the exact head, then authorize a lock only from the configured human's latest authenticated review of that head and independently verified merged contents. The operator governs direct/force pushes, privileged merges and path exceptions. Preserve existing GitHub protection and approval behavior. (design-brainstorm:forgejo-feature-draft-tech-design#D-2)

**Serves:** FR-5 · UB-2 · UB-4 · ABUSE-1 · ABUSE-2 · DOD-3

**Builds-on:** D-1 · D-2

#### Concrete layout

The shared lifecycle retains the configured separate human, exact prepared head H and digest, native review request and D-6 approval selector. For Forgejo, preparation requires valid destination/actor/proposal state, complete final memory integrity and current-digest readiness. It does not query branch-protection rules or require proof of administrator, direct/force-push or path-exemption enforcement. There is no new protection configuration switch, policy engine or permissions writer.

The Forgejo operator is responsible for repository access and for keeping unapproved compiled contract records off mainline. This is an explicit scoped compatibility change to frozen design:feature-draft-v1 §3.9 and DINV-1's enforced mainline boundary for installed Forgejo, authorized by P-9. The software's narrower guarantee is authenticated lock authorization: it never authorizes a lock or planning from a merge badge, an agent/synthetic review, or an approval of another head. An out-of-band write may place unapproved records on mainline; that is outside the trusted-governance assumption and is not represented as prevented by Pyxis. Independent D-6 review/content/ancestry verification still refuses a false lock; it is not described as undoing or preventing such a write.

GitHub's existing code-owner, stale-dismissal, latest-push and administrator-bound repository policy remains unchanged. Its current adapter/preparation behavior and existing tests are retained; this Forgejo ruling creates no Source merge permission or GitHub policy exception. Both hosts use the same native approval reader/selector interface and exact-head artifact proof.

A round or new push changes H, so an older-head approval cannot satisfy the subsequent lock. Native dismissed/stale/rejected observations remain ineligible. D-6 selects the uniquely latest submitted terminal review before checking its state/head and refuses uncertain identity, ordering or incomplete reads. The agent requests review, publishes only the draft branch under a lease, and never approves or merges the proposal.

#### Observability

Preparation names the selected provider and exact requested head/digest/human. Refusal identifies the missing review, wrong head, uncertain actor/order or unmatched merged artifact. A successful Forgejo preparation receipt is a request for human review, not evidence of server-enforced bypass protection.

#### Security and privacy

Credentials and destinations remain scoped by D-2. The configured agent cannot impersonate the human or manufacture acceptance through configuration or a PR body. Operator-governed Git/API writes are an explicit trust boundary; no guarantee of resistance to a privileged actor or policy exception is claimed for Forgejo.

#### Verification

Use controlled native-schema review fixtures for no approval, agent/other-human review, pending/comment-only state, rejection, dismissed/stale state, changed head and ambiguous identity/submission ordering. Every invalid approval produces zero lock-tag writes and no authorized planning; a valid latest configured-human review of H plus matching mainline artifacts permits the lock. Preparation succeeds in an otherwise valid Forgejo fixture whose protection endpoint is unavailable, proving bypass-policy inspection is not a prerequisite. Retain existing GitHub protection/approval regression tests. Synthetic identities remain confined to tests; actual Source and application locks require the real operator's authenticated review. Native administrator/direct/force/path-bypass prevention experiments are outside this contract's blocking DoD.

### D-6 — Verify the merged artifact and publish a byte-idempotent lock

**Decision:** Select exactly one native merged proposal, verify the configured human's current-head review independently, establish mainline ancestry and exact compiled-record content/digest through Git, and publish or raw-byte-verify the annotated lock tag with race reconciliation. (design-brainstorm:forgejo-feature-draft-tech-design#F-6)

**Serves:** FR-6 · UB-2 · UB-3 · UB-4 · UB-5 · UB-6 · NFR-2 · ABUSE-1 · ABUSE-5 · DOD-3 · DOD-4

**Builds-on:** D-2 · D-4 · D-5

#### Concrete layout

cmd/draft-lock-record resolves the selected code-host binding and configured credentials instead of universally requiring GH_TOKEN or a hardcoded GitHub reader. The installed Forgejo post-merge entrypoint is pyxis exec draft-lock-record -- --before <B> --after <A>, invoked from the installed bound checkout after the actual human merge. Both flags are explicit full commit IDs: B is the previously verified mainline tip before the merge/range and A is the exact fetched configured-mainline tip covering that merge. Supplying the pair activates recording without PYXIS_LOCK_RECORD_RUN or any GitHub environment. Reject a missing member, malformed or zero ID, unknown object, non-ancestor B, after that is not the fetched mainline tip, or range inconsistent with the bound repository before tag writes. Fetch through D-2's verified remote/credential context; never guess the range from the draft branch or a merge badge. The existing distributed tool registry, release manifest/payload and installed frontdoor already include draft-lock-record and must continue to do so.

This acceptance drive performs that explicit command after each human merge and retains its native receipt. An operator or existing CI can invoke the same command with verified mainline range inputs; no Forgejo Actions installation, daemon or new service is a prerequisite. Document the post-merge step in the installed Forgejo workflow guide. Keep GitHub's existing mainline-push workflow, PYXIS_LOCK_RECORD_RUN=1 and before/after environment route compatible; that route validates its resolved range through the same core recorder. Retain the existing lock schema and range discovery within the explicitly supplied range. Selected native observations find exactly one same-repository PR for each compiled merge, with the configured base, draft version branch, prepared head H, and merge commit M. Enumerate all relevant pages before choosing; zero/multiple/fork/wrong-base/wrong-merge cases refuse without a tag. Repository identities are verified before consuming host fields. These engineering details respond to draft review010 FND-1 under existing FR-6/DOD-4; their basis is recorded in brainstorm/003's engineering elaborations.

Add a Forgejo approval.Port reader and retain the same approval.Port interface and shared selector for both native adapters. Exact-read the proposal's head before and after complete review enumeration; both must be H, with merged association M unchanged. Map native review ID/user login+ID/state/commit_id/body, submitted_at and dismissed/stale flags into shared typed Review observations. Extend Review with a validated native submission timestamp and extend approval.Select to order terminal submitted operator reviews by that timestamp, not by numeric ID, created_at, updated_at, list position or pagination order. Both GitHub and Forgejo readers decode their actual submitted_at field with RFC3339-compatible timezone/precision handling and compare instants. Review IDs identify the chosen native review only.

Pending and comment-only reviews remain non-authoritative and do not replace a submitted approval. For every otherwise eligible terminal operator review, absent/malformed submission time, conflicting duplicates, or an equal latest timestamp on distinct reviews leaves ordering uncertain and refuses approval; there is no numeric-ID fallback. Select the uniquely latest submitted terminal operator review before requiring that it approves H, so a later rejection, dismissal/stale observation or other-head review cannot revive an older approval. Dismissed/stale observations cannot normalize to APPROVED. Unknown/malformed review state or actor, partial pagination, changed head or missing commit ID continues to fail closed. Preserve the exact optional capture body and explicit lineless case. This corrects the shared selector's proof of its existing latest-submitted obligation without a second approval pipeline or a new port.

Fetch the configured mainline using D-2's Git context, require M to be its ancestor, and read the merged compiled PRD/design Documents and their typed records directly from M. Validate the compiled graph, active records, exact stored text, approved-head/readiness trailer, and recomputed combined digest using the same digest routine as draft compilation. Check those texts/digest against the verified prepared H, rather than reconstructing approval from the platform badge. Squash merges are supported by checking H's prepared contents against M and M's mainline ancestry; H itself need not be an ancestor after squash. Existing version-relative unchanged records must resolve from the validated prior lock, without deleting/replacing prior obligations.

Construct the deterministic draft-lock/v1 annotation through the existing serializer, retaining feature/version, H, M, digest, real PR/review/operator, capture/lineless fields, and proposal count. Publish the annotated tag at M through Git. Existing tags must be annotated, target M, and have the exact annotation bytes produced by that serializer; decoded-field equality alone is insufficient. Fetch and read back the remote tag/object after publication. If another recorder wins a non-force tag race, fetch its tag and succeed only if target and raw annotation bytes match; otherwise refuse a conflict. Never overwrite or advance a conflicting lock version.

#### Observability

Record the explicit B/A range, selected repository/provider and exact proposal/head/review/mainline/digest/tag receipt only after readback. Failures identify the PR, missing proof, or conflicting tag without emitting token or untrusted response body. A lock is not delivered merely because native merge status is true.

#### Security and privacy

Post-merge review, content and ancestry remain independently verified under D-5's provider-specific trust model. No synthetic acceptance can authorize a real lock. A tag that points to a side branch, stale content, or conflicting annotation cannot authorize planning.

#### Verification

Exercise no/multiple/fork/wrong-base proposals, stale/agent/unknown/dismissed/rejected reviews, head movement during review pagination, mismatched typed records/text/digest, nonexistent or non-ancestor M, valid squash, malformed/lightweight/conflicting tags, annotation-byte differences, repeated invocation, and identical/conflicting racing recorders. Both native reader fixtures must include a lower-ID pending review created before a higher-ID approval, then submitted as a rejection afterward: the later submission must yield no lock/tag. Also verify that a uniquely later lower-ID approval can succeed, a later other-head or dismissed/stale review refuses, and pending/comment-only observations do not replace a valid submitted approval. Test absent/malformed submission times, timezone-equivalent instants and equal latest timestamps on distinct reviews; unknown ordering produces zero tag writes. The fixture schemas use actual native submitted_at fields and states, and existing GitHub capture/lineless and exact-head cases remain covered. Test explicit missing/malformed/zero/out-of-repository/non-ancestor/stale-A ranges and unbound credentials with zero tag writes; retain the normal GitHub workflow route. The installed no-gh Forgejo proof must perform a real human merge, invoke this exact installed entrypoint with verified B/A, invoke it again idempotently, and fresh-clone the remote tag to verify one annotated lock with exact H/digest/review/PR/M and raw annotation bytes before planning.

### D-7 — Stage-aware hooks with complete final integrity and readiness

**Decision:** Extend the existing source push gate to observe working-draft authoring stage and defer only final draft-ready there; required PR readiness continues to judge the same contract predicate, and every preparation, prepared-head push, and lock uses complete memory integrity plus current-digest readiness. (design-brainstorm:forgejo-feature-draft-tech-design#F-7)

**Serves:** FR-7 · UB-5 · UB-6 · DOD-6 · NR-1 · NFR-3

**Builds-on:** D-1

#### Concrete layout

Use the existing code-gate check inventory and readiness implementation. The hook derives an intermediate authoring stage only from a configured draft branch and that feature's actual working draft files, without a compiled prepared landing tree. An ambiguous/mixed stage refuses rather than accepting an author-supplied skip flag. Opening and round pushes run every other applicable check, including memory integrity and attribution; the production policy is not git push --no-verify. Prepared trees cannot select that deferral because compilation removes the working files and publishes the contract-record tree.

Required PR draft-ready still produces the shared verdict, including unmet conditions while a proposal is intentionally unfinished; deferring an intermediate hook does not turn an incomplete proposal into a green merge candidate. The final hook and preparation path run that same readiness predicate against current mainline, latest PRD and tech-spec reports at the current combined digest, all Must coverage, answered product problems, valid links/relations, and blocking dispositions. There is no independent weaker CI readiness definition.

Complete memory integrity remains required at intermediate production pushes and all final paths. Lossless problem/delivery bindings, evidence validity, trusted policy, and the native compiled PRD/design records must satisfy the existing memory gate. The current source drive's first-PR scope conflict is a prerequisite that must be resolved through a sanctioned human/bootstrap or separately reviewed compatibility route before final publication; this design grants no memory/problem scope extension, debt/baseline mutation, stamp removal, relabeling, or suppressed binding check. A missing binding or out-of-scope capture refuses the final path with the actual remedy. Problem A-4 remains only this source draft's temporary intermediate-document publishing exception and does not implement or test the production policy.

Ship a fresh verified release from reviewed mainline after implementation, verify its manifest and installed artifacts, and repeat the lifecycle from an installed Forgejo repository with no gh available. Installed demo, GitHub source conformance, native review/lock tests, fresh-clone tag verification, and normal hook transcripts provide DOD-1..DOD-6 evidence; contract preparation alone supplies none of that implementation evidence.

#### Observability

Gate results retain stage, exact checked tree/head/base, check names, and complete unmet/remedy output. Record readiness deferral as an explicit intermediate-stage event. Never represent a skipped check as a pass or a diagnostic compiler binary as an installed release.

#### Security and privacy

No caller flag, body claim, branch rename alone, or earlier passing digest may waive final readiness or memory integrity. Current full source attribution and protected-path checks remain in force. Approval and human merge remain outside the agent's gate permissions.

#### Verification

Test open/round stage detection, branch/file disagreement, mixed compiled/working trees, normal intermediate pushes with only readiness deferred, and all remaining checks invoked. Test final preparation/prepared push/lock with missing lossless bindings, invalid evidence, stale reviews, changed digest, uncovered Must rows, and invalid Argus attribution; every negative must refuse before publication. The positive final head passes the complete memory gate and identical local/CI readiness predicate. Run the installed end-to-end proof and existing code-gate/CI parity suites; retained historical failures are not new passes.
