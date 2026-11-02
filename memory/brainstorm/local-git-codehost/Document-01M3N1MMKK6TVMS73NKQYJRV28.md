# Local Git code host — brainstorm record

| Field | Value |
|---|---|
| **In one line** | Keep AEC on its local bare Git origin and define the Pyxis proposal and exact-head review route needed to run its Argus features. |
| **Relevant-when** | Contracting Pyxis local Git proposal, approval, or landing behavior; checking the AEC operator's local-origin and trust rulings. |
| **Idea** | AEC operator's Pyxis × AEC test request with Codex-only Argus, Carina, and a local bare Git origin; durable test context: issue #1302 and the five AEC `docs/problems/<slug>/` intake ledgers. |
| **Operator** | Repository operator (@mdarabi) |
| **Date(s)** | 2026-09-28 |
| **Status** | gated: ready-for-prd; exact-head draft lock approval pending |
| **Tracking** | https://github.com/agenticlab-ai/pyxis/pull/1312 |
| **Feeds** | `drafts/local-git-codehost/prd.md` |
| **Prior records** | — |
| **Template version** | brainstorm-record/v1 |

## Orientation

| Source | Why loaded |
|---|---|
| [Pyxis issue #1302](https://github.com/agenticlab-ai/pyxis/issues/1302) | The installed draft CLI asks GitHub for a review object that AEC's bare origin cannot supply. |
| `docs/tech-design/feature-draft-v1.md` | Existing locks require an authenticated human review of the exact prepared head. |
| AEC `docs/problems/<slug>/problem.md` and `delivery.md` for five RFP reviewer features | The selected Codex, Carina, local Git route is recorded in the intake and A-3 operator rulings. |
| `docs/tech-design/installer-v1.md` | AEC runs a pinned, verified Pyxis release; any test candidate must preserve that execution boundary. |

**Overlaps surfaced:** The existing hosted feature-draft route owns lock semantics and remains in force for hosted repositories. A local adapter must fit those lifecycle ports and identify where the existing approval contract needs amendment.

## Framings

- **Local Pyxis adapter** — represent a reviewable proposal in the bare origin and keep exact-head review and readiness in the Pyxis lifecycle; the implementation can serve any opt-in local Git repository.
- **Hosted code host for AEC** — use a GitHub or Forgejo PR and its native review, which changes the operator's selected local-origin test surface.
- **AEC-only test shim** — imitate a PR inside the test repository, leaving the installed Pyxis draft lifecycle unable to serve another local Git repository.

**Destination:** Keep AEC's bare origin and develop the Pyxis local route needed for the installed test. This gists the operator's accepted proposal, not a claim that a specific draft head or broadened trust guarantee was approved.

## Decisions

### D-1 — Which code-host route should the AEC test use?

- **Question**: Keep the selected local bare Git origin and develop the needed Pyxis route, or move AEC to a hosted code host?
- **Answer**: Keep local bare Git and proceed with the proposed Pyxis work needed to run the Codex/Carina AEC test; create a new verified release if the test needs it.
- **Reasoning**: The installed draft command's GitHub dependency is the blocker; changing AEC's selected medium would skip that development gap. The immediate user is the AEC operator on local Git; existing hosted users are outside this opt-in route.
- **Anchor**: 2026-09-28 · AEC operator message “ok, i agree with your proposal. go for them,” following the local-route proposal; earlier AEC test and release instructions.
- **Status**: live

### D-2 — What mainline protection does the local origin provide?

- **Question**: Move the origin behind a separate principal with protected receive enforcement, or keep the shared-account origin with narrower assurance?
- **Answer**: “Keep the shared-account origin and accept signed-record verification without host-level mainline protection.”
- **Reasoning**: The operator selected the existing local setup. Because the agent can edit its refs and hooks, the local path must disclose that host-level protection is absent.
- **Anchor**: 2026-09-28 · AEC operator answer to the local-origin protection question.
- **Status**: live

### D-3 — Must an independent operator signing key authenticate receipts?

- **Question**: Keep a signing key outside the agent-accessible filesystem for operator approvals, or waive that requirement for this test?
- **Answer**: “ignore this requirement.”
- **Reasoning**: Under the shared account and without that key, a local receipt can check exact-head consistency but cannot prove who wrote it. The earlier “signed-record” phrase in D-2 is narrowed to an unsigned consistency record.
- **Anchor**: 2026-09-28 · AEC operator answer to the independent-key question.
- **Status**: live

### D-4 — Who reviews each prepared head, and what is the human budget?

- **Question**: May an agent approve its own draft or code proposal, or does the operator review every exact head despite the local record's inability to authenticate the actor?
- **Answer**: The operator reviews and approves each exact prepared draft and code head. Argus drives the work to a review request but does not approve its own output. The accepted appetite is one verified local-route release and installed AEC proof, with an operator review act for every head that lands.
- **Reasoning**: The existing feature-draft human gate remains a product decision even though D-2 and D-3 weaken machine-enforced identity in the shared-account local mode. A local receipt records the declared act and checks consistency; it cannot prove who performed it.
- **Anchor**: 2026-09-28 · AEC operator reply “approved” to the common-case proposal at PR #1312 head `78b13bbeb974cede10d4ae5119bf1a354dd9dacb`, whose PRD header, summary, and FR-3/FR-7 require per-head operator review; `prd:feature-draft/FR-17`.
- **Status**: live

## Facts

- **F-1**: The current draft lifecycle and lock recorder invoke hosted PR operations for listing, opening, preparation, review verification, and post-merge recording — source: issue #1302 and `internal/draft/`.
- **F-2**: The merged feature-draft contract requires authenticated human approval of the exact prepared head — source: `prd:feature-draft/FR-17`, `FR-46`, `UB-3`; `design:feature-draft-v1#DINV-4`.
- **F-3**: A bare Git origin stores refs and objects but provides no built-in PR review actor — source: AEC reproduction and Git adapter inspection.
- **F-4**: The AEC origin and agent share an OS account, so its hooks and refs are agent-writable — source: local origin permissions inspection and D-2.

## Fog

none

## Out of scope

- **O-1**: Switching AEC to a hosted GitHub or Forgejo PR route — the local-origin test choice in D-1 would be lost.
- **O-2**: Claiming host-level mainline protection from a same-account hook — D-2 accepted its absence.
- **O-3**: Claiming approval identity from an unsigned local record — D-3 waived the independent signer.

## Candidate glossary terms

- **T-1**: local review receipt — a versioned statement bound to a proposal head and landing target whose fields can be checked from Git; avoid: authenticated signature.

## Parked technical constraints

- **P-1**: A local Git adapter must preserve the draft lifecycle's readiness and exact-head behavior through a code-host port — the host medium differs, while the contract stages remain.
- **P-2**: The shared-account route can prove record/object consistency through the normal Pyxis path but cannot make the bare origin tamper-proof — D-2 and D-3.
- **P-3**: Before the local adapter is enabled, the merged feature-draft approval rows need a scoped PRD amendment and successor design — F-2.

## Gate

| Field | Value |
|---|---|
| **Verdict** | ready-for-prd |
| **Date** | 2026-09-28 |
| **Operator** | Repository operator (@mdarabi), by the accepted local-route proposal and subsequent trust choices; this is not exact-head lock approval. |
| **Covering source** | — |
| **Coverage check** | pass — D-1 fixes the local-origin problem framing and primary operator; D-4 names the agent seeking self-approval as the anti-segment and fixes the per-head human budget; F-1/F-2 supply the observed job and constraint; D-1 and D-4 set the observable goal; O-1–O-3 cover non-goals; zero blocking fog. |
