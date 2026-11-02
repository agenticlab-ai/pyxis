# Forgejo feature drafts — problem ledger

| Field | Value |
|---|---|
| **Slug** | forgejo-feature-draft |
| **Principal** | operator (acceptance-test request) |
| **Authority-contract version** | authority-contract/v1, tier-2 at intake |
| **Envelope override** | —; no per-problem override supplied |
| **Template version** | problem-ledger/v1 |

## Idea

Allow the current Pyxis feature-draft workflow to run against the configured Forgejo code host so a fresh installed AEC repository can progress through Argus requirements, design, review, exact-head operator approval, locking, planning, implementation, landing, and verified delivery.

You are preparing the focused Pyxis correction required by the operator's acceptance-test request at ../../acceptance/objective.txt. Follow this source repository's AGENTS.md and Argus drive guide, the current memory system, and the actual current draft workflow. The Pyxis source repository uses GitHub; the application acceptance repository uses Forgejo for both tracker and code host. Do not move the application's code or tracker to GitHub. Work only in this isolated Pyxis clone, not in the other source or AEC clone.

Concrete reproduction already established from source e848afcc92a381c6dc8773f42920b4237da6e6d9 in a verified v0.1.474-aecforgejo.1 installation: in aec_forgejo_repo, `pyxis draft status company-memory` invokes `gh api --paginate repos/{owner}/{repo}/pulls?state=open&per_page=100` and refuses because the configured origin is http://127.0.0.1:3330/argus/aec-rfp-reviewer.git. `lifecycle.Open`, Reopen, Abandon, Prepare/Round, and RecordPush also hardcode gh calls or GitHub push URLs; cmd/draft-lock-record selects only the GitHub approval reader. Forgejo already has native code-host, tracker, authenticated client, configured integration bindings, and Git credential adapters. Inspect and reuse them. Existing durable knowledge includes learning/forgejo-native-task-code-and-lifecycle-identity and learning/forgejo-fast-installed-feature-e2e; these previous tests did not test feature-draft authoring. Current memory CLI subjects can be located via scoped search.

The operator authorizes the simplest practical implementation and focused fixes through required review. This is authorization to prepare a correction, not approval of an unseen lock or code head. Keep the authenticated, exact-head human approval requirement; do not weaken it, spoof it, or treat synthetic test approval as actual operator approval. Retain GitHub behavior and fail closed on stale reviews, invalid identity, ambiguous proposals, origin mismatch, HTTP failure, and credential leakage. Support Forgejo lifecycle without a fake gh shim; use a consumer-owned neutral seam and native Forgejo adapter. Git operations remain Git. Preserve Argus content-bound trailers through preparation rebase. Lock tags and compiled records must be verified against mainline.

Use current authority at every permission act. Record your assumptions as Argus, with exact citations, and do not invent operator rulings. The first-push defect is already covered by existing green PR #1307 at ece4404df00dc18ecdbac648653edb0b6849a33a and needs its human merge; exclude that fix from this concern. Local-Git code-host implementation is also a separate effort; exclude it here. Treat changes to frozen source-only scope as an explicit compatibility extension in the proposed contract, not a silent rewrite of frozen documents. Source GitHub draft identity: mdarabiassistant; token is in environment PYXIS_DRAFT_AGENT_TOKEN. Configure these only in ignored .pyxis/config.local.yaml. Run the source hooks normally.

Complete all authorized work through a concrete reviewable contract proposal: overlap check, intake, requirements, design, PRD and tech-spec cold reviews with disposition, readiness, and prepare-lock on a clean pushed draft branch. Use all applicable stage protocols. One draft PR for this correction; do not start implementation or planning before the lock lands. Do not manufacture a gate approval. If an ungranted requirement ruling is essential, preserve it as an open problem while continuing independent work. If a routine scope decision is already determined by the original objective and compatible current contract, cite the determinant. Obtain independent reviews through Codex only, using model gpt-6-sol because the installed CLI account currently rejects the configured gpt-6.1-sol; retain the existing max reasoning setting. This native-route substitution is supported by the local account catalog and current model-routing policy. Do not use Claude, Cursor, or other harnesses.

Finish with the PR URL, exact prepared head, contract digest, readiness result, unresolved operator items, and accurate measurement availability. Your final response is captured in the acceptance logs. A final operator approval is pending until the human actually supplies it. No feature or correction is delivered merely because a proposal is prepared.

## Authority entries

### A-1 — Forgejo compatibility and exact-head lock boundary

- **Date**: 2026-09-30
- **Author**: operator (this invocation)
- **Authority-contract version**: authority-contract/v1, tier-2
- **Answers**: —
- **Ruling**: The correction targets the installed AEC repository's configured Forgejo code host while Pyxis source keeps GitHub. Retain authenticated human approval of the exact head, native Forgejo transport, GitHub compatibility, and fail-closed identity, origin, proposal, review, HTTP, and credential behavior. The operator has not approved a lock or head.

### A-2 — Scope and stage boundary

- **Date**: 2026-09-30
- **Author**: operator (this invocation)
- **Authority-contract version**: authority-contract/v1, tier-2
- **Answers**: —
- **Ruling**: Prepare one reviewable draft contract through cold reviews, readiness, and prepare-lock; do not start planning or implementation before the human-approved lock lands. Exclude PR #1307's first-push correction and the separate local-Git code-host implementation. State the frozen source-only boundary as an explicit compatibility extension.

### A-3 — Review route and source identity

- **Date**: 2026-09-30
- **Author**: operator (this invocation)
- **Authority-contract version**: authority-contract/v1, tier-2
- **Answers**: —
- **Ruling**: Use Codex gpt-6-sol with max reasoning for independent reviews, the agent's GitHub account mdarabiassistant with PYXIS_DRAFT_AGENT_TOKEN only in ignored local config, and the normal source hooks. Do not use a fake gh shim or treat synthetic approval as the operator's review.

### A-4 — Scoped intermediate-document publication exception

- **Date**: 2026-10-01
- **Author**: operator (conversation answer; recorded by Argus)
- **Authority-contract version**: authority-contract/v1, tier-2
- **Answers**: delivery:forgejo-feature-draft#E-7
- **Answer verbatim**: “continue”
- **Context**: The pending question presented the concrete proposal at `9c180533b12a3db26245675355a00c47c2bd0c33` and its E-7 brief, and requested explicit `git push --no-verify` for intermediate documentation rounds on this source draft branch. It disclosed incomplete draft readiness and the two missing lossless ledger bindings, with document/attribution and other applicable checks retained. The operator answered “continue” immediately after that question. Argus stated the scoped interpretation in the conversation before recording this answer.
- **Ruling**: Continue with that intermediate-document publishing exception for `forgejo-feature-draft` only. This narrowly supersedes A-3's normal-push-hook instruction for these rounds. Strict final readiness and memory integrity must be resolved before locking. Exact-head human contract-lock approval, source implementation checks, human merge, application hooks, requirement rulings, and authority promotion remain outside this exception.

### A-5 — Operator-directed repair of PR #1341's working contract

- **Date**: 2026-10-01
- **Author**: operator (conversation instruction; recorded and interpreted by Argus)
- **Authority-contract version**: authority-contract/v1, tier-2; no promotion
- **Answers**: draft requirement problems P-1..P-4 and their explicitly proposed acceptance-evidence edits
- **Instruction verbatim**: “1341 ci failed. fix it”
- **Context**: After the prior CI diagnosis identified pending P-1..P-4 and retained their concrete corrective patch, the operator instructed repair of this PR. Argus stated the bounded interpretation in the conversation: complete the working draft and apply the four corrections already proposed, while retaining separate final contract approval and merge.
- **Ruling**: Apply the already-proposed P-1..P-4 corrective clarifications and their UB-1, DOD-3, DOD-5, and new DOD-6 acceptance evidence to this unmerged working draft; complete its design and current-digest reviews so CI can judge a finished proposal. This is the scope Argus interprets from the operator's repair instruction, not a claim that the operator separately signed every sentence. Preserve authenticated exact-head human contract approval, full final memory integrity, ordinary source implementation review, and human merge. No unseen head, lock, implementation, authority promotion, general acceptance grant, or memory-scope exception is approved.

### A-6 — Operator-confirmed human review and merge of Source #1420

- **Date**: 2026-10-04
- **Author**: operator (conversation answer; recorded by Argus)
- **Authority-contract version**: authority-contract/v1, tier-2; no promotion
- **Answers**: delivery:forgejo-feature-draft#E-255
- **Answer verbatim**: “merged”
- **Context**: E-255 requested personal review and write-access merge of the exact qualified Source custody PR #1420 at42cfe53ca1b9e3315cdc75aa66faec6abfb88104. The operator replied “merged”; native mainline verification records the actual human squash10101196016e5a66ca77616c3d5bc4f386246b76 bymdarabi at2026-10-04T19:48:35Z and all three reviewed file blobs. E-256 records receipt of that answer; E-257 corrects the later binding-preflight inference without retracting the verified human merge.
- **Ruling**: This answer resolves only the personal Source1420 review/merge boundary. Intermediate draft maintenance remains within the existing A-4 exception and operator instruction to rebase with latest main. It does not approve an unseen #1365 v2 contract head, a final lock, implementation, another Source merge, a memory-scope exception or an authority promotion.

### A-7 — Operator-confirmed human review and merge of Source #1423

- **Date**: 2026-10-04
- **Author**: operator (conversation answer; recorded by Argus)
- **Authority-contract version**: authority-contract/v1, tier-2; no promotion
- **Answers**: delivery:forgejo-feature-draft#E-266
- **Answer verbatim**: “1423 merged”
- **Context**: E-266 requested personal review and write-access merge of the exact qualified parser-fix PR #1423 at1f1be4bfaee33040e9ae28125fd3e522cdb5e556. The operator replied “1423 merged”; native mainline verification records the actual human squash429d5b5696936007fbb5bb501f319870d6c8c6ef bymdarabi at2026-10-04T21:28:54Z and all five reviewed Go blobs. E-267 records receipt of that answer.
- **Ruling**: This answer resolves only the personal Source1423 review/merge boundary. It permits continuation of the existing authorized acceptance work with the landed parser fix, while retaining normal qualification, the supported prepare-lock route and separate exact-head human Source v2 approval/merge. It establishes no formal GitHub review record, unseen-head approval, future merge, memory-scope exception or authority promotion.
