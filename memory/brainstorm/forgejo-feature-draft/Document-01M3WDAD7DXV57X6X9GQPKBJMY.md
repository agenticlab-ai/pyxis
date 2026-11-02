# Forgejo feature drafts — brainstorm record

| Field | Value |
|---|---|
| **In one line** | Extend the feature-draft lifecycle to the configured Forgejo code host while preserving the human exact-head lock. |
| **Relevant-when** | contracting native Forgejo draft opening, rounds, approval, lock recording, or installed-repository authoring; checking the prior source-only boundary |
| **Idea** | The operator's exact idea is recorded verbatim in `docs/problems/forgejo-feature-draft/problem.md` §Idea. |
| **Operator** | operator supplied A-1..A-3; Argus is the stage decision principal under authority-contract/v1 tier-2 |
| **Date(s)** | 2026-09-30 |
| **Status** | gated: ready-for-prd |
| **Tracking** | draft/forgejo-feature-draft (PR pending publication) |
| **Feeds** | drafts/forgejo-feature-draft/prd.md |
| **Prior records** | brainstorm:feature-draft · design-brainstorm:feature-draft-tech-design |
| **Template version** | brainstorm-record/v1 |

## Orientation

| Source | Why loaded |
|---|---|
| docs/product-vision.md §§1–4 | The factory's medium-independent pipeline and human judgment boundary. |
| docs/prds/README.md; docs/prds/feature-draft/effective.md | Capability index and existing feature-draft requirements. |
| docs/tech-design/feature-draft-v1.md §§3.1–3.9 | Current lifecycle, source-only boundary, and exact-head approval port. |
| .pyxis/pipeline.md; .pyxis/authoring/argus.md | Current draft route and its single operator lock gate. |
| internal/draft/lifecycle; internal/draft/lock; cmd/draft-lock-record | Reproduced GitHub-only process calls and lock recorder. |
| internal/integrations; internal/adapters/common/forgejo; internal/gitcredential | Existing configured Forgejo binding, authenticated transport, and Git credential helper. |
| learning/forgejo-native-task-code-and-lifecycle-identity; learning/forgejo-fast-installed-feature-e2e | Existing Forgejo execution knowledge and its draft-authoring test gap. |
| docs/tech-design/local-git-codehost draft in memory | Separate local-Git medium under development, excluded here. |

**Overlaps surfaced:** `prd:feature-draft` and `design:feature-draft-v1` govern the lifecycle and explicitly exclude repositories other than Pyxis in §3.2. This is a versioned compatibility extension, not a rewrite of frozen rows. The local-Git code-host draft covers a different medium; PR #1307 covers the separate installed first-push defect. No merged contract or open PR covers native Forgejo feature-draft authoring (problem:forgejo-feature-draft#A-1, #A-2; delivery:forgejo-feature-draft#E-1).

## Framings

- **Transport swap** — replace gh invocations with Forgejo-only calls; reaches the acceptance repository but regresses Pyxis source GitHub and leaves approval/lock seams split.
- **Compatibility extension** — use the configured code-host binding and a draft-owned neutral seam with GitHub and native Forgejo adapters; preserve the same draft stages and exact-head operator lock.
- **New Forgejo pipeline** — fork the draft workflow for Forgejo; duplicates readiness, history, and review rules.

**Destination:** Compatibility extension — the operator explicitly requires native Forgejo, retained GitHub behavior, a consumer-owned seam, and the current workflow (problem:forgejo-feature-draft#A-1, #A-2). The source-only boundary is named for review, not silently removed.

## Decisions

### D-1 — installed repository and user

- **Question**: Which repository and user journey does this correction serve?
- **Answer**: The installed AEC repository uses Forgejo for code host and tracker; Argus must move a new feature through the existing draft and delivery stages. The Pyxis source keeps GitHub.
- **Reasoning**: Direct operator scope in problem:forgejo-feature-draft#A-1 and the acceptance objective; no repository migration is authorized.
- **Anchor**: 2026-09-30 · operator invocation A-1
- **Status**: live

### D-2 — compatibility and boundary

- **Question**: Does the proposed contract replace the source workflow or extend its code-host support?
- **Answer**: Extend the current workflow through a native Forgejo adapter behind a consumer-owned neutral seam, retaining GitHub and Git operations; explicitly name the frozen source-only scope as an extension.
- **Reasoning**: Operator A-1/A-2; `design:feature-draft-v1` §3.2 is frozen and cannot be silently edited.
- **Anchor**: 2026-09-30 · operator invocation A-1/A-2
- **Status**: live

### D-3 — approval and failure boundary

- **Question**: What evidence authorizes a lock?
- **Answer**: Only an authenticated human operator review of the exact prepared head; fail closed on stale or invalid reviews, wrong origin, ambiguous proposals, transport failures, or leaked credentials. Synthetic review evidence is test-only.
- **Reasoning**: Operator A-1/A-3 and existing `prd:feature-draft/FR-17`, `FR-46`, `FR-47`.
- **Anchor**: 2026-09-30 · operator invocation A-1/A-3
- **Status**: live

### D-4 — correction perimeter

- **Question**: What work remains in this correction?
- **Answer**: Prepare one reviewed draft contract, then wait for the human lock; implementation and planning begin only after it lands. PR #1307 and local-Git code-host delivery remain separate.
- **Reasoning**: Operator A-2; an unseen head is not approved by an authoring gate.
- **Anchor**: 2026-09-30 · operator invocation A-2
- **Status**: live

### D-5 — Forgejo operator identity source

- **Question**: How should a Forgejo installation identify the human whose review may authorize a lock?
- **Answer**: Proposed assumption: require an explicit configured operator login distinct from the draft agent and compare it with the native review actor; no fallback from owner or agent token. (delivery:forgejo-feature-draft#E-3)
- **Reasoning**: `cmd/draft-lock-record` currently hardcodes a source GitHub reviewer; an installed host needs a repository-specific human identity, and guessing one would weaken `FR-46`.
- **Anchor**: 2026-09-30 · Argus assumption E-3, authority-contract/v1 tier-2
- **Status**: live

## Facts

- **F-1**: `pyxis draft status company-memory` failed at a `gh api` call in the installed Forgejo repository — source: problem:forgejo-feature-draft §Idea, reproduction from e848afcc / v0.1.474-aecforgejo.1.
- **F-2**: Open, Reopen, Abandon, Prepare/Round, RecordPush, and cmd/draft-lock-record contain GitHub-specific operations — source: `internal/draft/lifecycle`, `internal/draft/lock`, `cmd/draft-lock-record` inspected at e848afcc.
- **F-3**: Configured integration bindings, Forgejo REST client and code-host adapter, and URL-scoped Git credential helper exist — source: `internal/integrations/config.go`, `internal/adapters/common/forgejo/client.go`, `internal/adapters/codehost/forgejo/host.go`, `internal/gitcredential/credential.go`.
- **F-4**: The source pre-push code gate refuses the intentionally unready initial scaffold through `memory ready --all`; `draft open` retained a local scaffold commit after push failure — source: delivery:forgejo-feature-draft#E-2; `.pyxis/config.yaml` `draft-ready` check, `internal/memory/cli/ready.go`.

## Fog

none — operator A-1/A-2 fixes the destination, segment, job, guardrails, and scope. Implementation tradeoffs pass to stage 3.

## Out of scope

- **O-1**: Do not move the AEC application to GitHub or alter its tracker — operator A-1 (D-1).
- **O-2**: Do not fold PR #1307 or local-Git code-host implementation into this correction — operator A-2 (D-4).
- **O-3**: Do not approve, lock, plan, or implement this correction before the exact prepared head receives the human review — operator A-2/A-3 (D-3/D-4).

## Candidate glossary terms

none — the existing glossary already defines feature draft, round, lock, and locked version.

## Parked technical constraints

- **P-1**: Preserve the current prepared-head digest, Argus content-bound commit trailers across rebase, and lock tags whose merge commits and compiled records are verified on mainline — operator A-1 and `design:feature-draft-v1` §§3.7–3.9.
- **P-2**: Source pre-push must permit an incomplete draft round to be published while still running the applicable hook checks; readiness remains mandatory at preparation/lock — F-4 and operator A-3.
- **P-3**: Forgejo API request/response and review fields need validation against the installed service and native adapter; uncertain or incomplete state refuses — learning/forgejo-native-task-code-and-lifecycle-identity.

## Gate

| Field | Value |
|---|---|
| **Verdict** | ready-for-prd |
| **Date** | 2026-09-30 |
| **Operator** | Argus, authority-contract/v1 tier-2 `sign-gate-verdict` (delivery:forgejo-feature-draft#E-4); human lock approval remains pending |
| **Covering source** | — |
| **Coverage check** | pass — problem framing D-1/D-2; primary and anti-segments D-1/O-1; job D-1; success direction D-3; non-goals O-1..O-3; appetite D-4; zero blocking fog |
