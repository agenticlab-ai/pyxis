# Forgejo feature drafts — design-brainstorm record

| Field | Value |
|---|---|
| **In one line** | Add a draft-owned host seam with native Forgejo and GitHub adapters; retain the existing compiler, human lock, Git, and complete final gates. |
| **Relevant-when** | Implementing the installed Forgejo draft lifecycle, its credential boundary, human protection, preparation, lock recorder, or stage-aware source push policy. |
| **Basis** | draft:forgejo-feature-draft/prd.md |
| **Authority** | Argus, engineering principal under authority-contract/v1 tier-2; operator-directed working corrections are attributed separately in problem A-5. |
| **Date(s)** | 2026-10-01 |
| **Status** | gated: ready-for-design |
| **Tracking** | https://github.com/agenticlab-ai/pyxis/pull/1341 |
| **Feeds** | drafts/forgejo-feature-draft/tech-spec.md |
| **Prior records** | brainstorm:forgejo-feature-draft; design-brainstorm:feature-draft-tech-design; design-brainstorm:feature-draft-tech-design-2 |
| **Template version** | design-brainstorm-record/v1 |

## Orientation

| Source | Why loaded |
|---|---|
| drafts/forgejo-feature-draft/prd.md; problems.md | Current requirement basis and A-5's bounded repair; the human-only routing section supplies no machine input. |
| docs/tech-design/index.md; docs/prds/README.md | Routed the existing draft and attribution contracts before exploration. |
| docs/tech-design/feature-draft-v1.md §§3.1–3.10,3.14 | Existing state, one proposal, preparation, review, compile, and lock obligations; source-only scope is the explicitly proposed compatibility extension. |
| docs/tech-design/feature-draft-reopen.md §§3.1,3.6 | Exact restored text and version-relative compilation remain settled. |
| docs/tech-design/argus-draft-attribution.md §§3.2–3.4 | Content-bound stamps, supported rebase, fixed first-PR scope, and honest shared-account guarantee. |
| docs/architecture-principles.md AP-1/3/6/10/11/12/13/14/15/17/22/26/27/30; RW-3 | Neutral typed consumers, one predicate, observed artifacts, leases, additive adapters, configuration, and refusal boundaries. |
| docs/observability/incidents.md; docs/wiki/glossary.md | Matched I2, I8, I10, I5/I11; retained draft, lock, and produced-authorship vocabulary. |
| drafts/forgejo-feature-draft/brainstorm/001.md; matched feature-draft technical records | Product perimeter and prior workflow direction, not new engineering rulings. |
| internal/draft/lifecycle, lock, approval; internal/executor/port/codehost.go | Current GitHub process calls, separate approval selector, and task-shaped shared code-host port. |
| internal/adapters/common/forgejo; internal/adapters/codehost/forgejo; internal/gitcredential; internal/integrations | Existing bounded HTTP client, observed native draft state, scoped Git credentials, and independently selected bindings. |
| Forgejo 16.0.5 Swagger and official v16 configuration/protection documentation | Actual response/write fields and title-prefix mechanism; settings existence is not evidence of privileged enforcement. |

**Overlaps surfaced:** The frozen feature-draft designs already decide lifecycle, review freshness, literal compilation, and exact-head approval. Those decisions are extended to an additional configured host, not reopened. Existing task Forgejo adapters cover task proposals, not contract authoring. #1344/#1345 repair validation; #1346 removes new-scaffold example rows; none supplies this host seam. The following questions concern the extension's engineering arrangement.

## Design questions

- **DQ-1**: Where should neutral proposal, identity, protection, and review operations live so a Forgejo contract proposal does not require executor task objects?
- **DQ-2**: How should API and Git credentials be bound before the first authenticated request, including when the draft agent differs from the task actor?
- **DQ-3**: How should the existing state machine reconcile one proposal and preserve retry, round, reopen, and abandonment semantics?
- **DQ-4**: How should preparation publish the known prepared head without losing valid Argus bindings or replacing it with a stale readback?
- **DQ-5**: How should native protection keep compiled records off mainline until the configured human approves that exact head?
- **DQ-6**: How should the recorder independently prove the human review, compiled text, mainline content, and annotated tag after a squash merge?
- **DQ-7**: How should opening/round hooks defer only final readiness while every final path retains complete memory integrity and the same readiness predicate?

## Panel log

- **DQ-1**: opens — fired — two parallel, read-only Codex design alternatives under orthogonal constraints: minimize new interfaces by widening shared codehost versus contain draft capabilities in a new consumer-owned seam. Same example: open `company-memory`, prepare head H, human squash merge M, record the lock. Recommendations were presented to Argus and resolved live in D-1; panel agents had no ruling authority.
- **DQ-2**: extends — skipped — use the existing integration resolver, common HTTP client, and scoped credential helper; credential-role selection extends those capabilities.
- **DQ-3**: extends — skipped — retain the settled draft state machine and its existing file/Git transitions behind D-1's selected host seam.
- **DQ-4**: extends — skipped — retain the supported preparation worktree/rebase path and replace its host-specific calls.
- **DQ-5**: extends — skipped — protection observation is part of D-1's selected capability set; this question chooses its refusal and proof policy under FR-5.
- **DQ-6**: extends — skipped — extend the existing approval port and recorder's content verification, not introduce another authority or storage class.
- **DQ-7**: extends — skipped — extend the existing code-gate stage selection and retain its existing checks, rather than add a parallel gate system.

## Decisions

### D-1 — Consumer-owned draft host
- **Question**: DQ-1, shared task port or draft-owned composite?
- **Answer**: Argus selects ALT-2: a typed draft host port and provider-registered GitHub/Forgejo adapters; existing approval.Port stays separate and reusable.
- **Reasoning**: The task port lacks draft-only capabilities; containing them protects NR-2 while reusing common transport. Two production adapters and a hermetic conformance adapter justify the seam (AP-1/AP-3/AP-27).
- **Anchor**: 2026-10-01 · live Argus design decision; legality and authority recorded in delivery:forgejo-feature-draft#E-29.
- **Status**: live

### D-2 — Bound destination and actor
- **Question**: DQ-2, what authorizes the first credential-bearing request?
- **Answer**: Argus selects offline exact remote/binding validation before constructing authenticated host operations; draft credentials remain an ephemeral actor selection, with an explicit separate operator login.
- **Reasoning**: A wrong read destination leaks credentials even without writes; binding the child Git credential environment to the draft actor avoids changing task credentials (FR-1/UB-1/UB-2, AP-15/AP-17/AP-30).
- **Anchor**: 2026-10-01 · live Argus design decision; delivery:forgejo-feature-draft#E-29.
- **Status**: live

### D-3 — Reconcile lifecycle from exact observations
- **Question**: DQ-3, how does one logical proposal survive retries and rounds?
- **Answer**: Argus selects complete native enumeration plus exact reads, deterministic branch identity, known-head leases, and the unchanged Git/file state machine; native draft state is observed after title-prefix changes.
- **Reasoning**: Reusing a branch without checking repository/base/head can silently reuse the wrong proposal; restore and abandonment remain governed by the existing contract (AP-13/AP-22, I8).
- **Anchor**: 2026-10-01 · live Argus design decision; delivery:forgejo-feature-draft#E-29.
- **Status**: live

### D-4 — Existing preparation with a known publication target
- **Question**: DQ-4, what head is checked, published, and requested for review?
- **Answer**: Argus selects the existing detached worktree and supported stamp-refreshing rebase, full final gates and exact compilation, then leased publication of the locally created H followed by bounded readback of H.
- **Reasoning**: A post-push response cannot redefine the expected head; preserve original source-stamp verification and fail on stale/replayed bindings (AP-11/AP-13/AP-15, I10).
- **Anchor**: 2026-10-01 · live Argus design decision; delivery:forgejo-feature-draft#E-29.
- **Status**: live

### D-5 — Effective protection before making a prepared proposal mergeable
- **Question**: DQ-5, how is the human boundary enforced before merge?
- **Answer**: Argus selects exact effective branch-protection inspection, exclusively configured-human approval eligibility, stale-dismissal and privileged enforcement, with disposable native negative-merge proof; unsupported or ambiguous protection refuses preparation.
- **Reasoning**: Post-merge tag refusal cannot undo mainline records; schema fields alone do not prove enforcement. No agent merge method or automatic protection weakening is introduced (FR-5/DOD-3, AP-14/AP-17).
- **Anchor**: 2026-10-01 · live Argus design decision; delivery:forgejo-feature-draft#E-29.
- **Status**: live

### D-6 — Independent mainline and byte-idempotent lock proof
- **Question**: DQ-6, what does a successful lock tag certify?
- **Answer**: Argus selects complete native merged-PR/review observation, exact-head shared approval selection, Git ancestry and compiled-text/digest checks, and raw annotated-tag target/body verification with racing-winner reconciliation.
- **Reasoning**: A merged badge and decoded tag fields are weaker than the promised exact mainline artifact; content proof also supports squash merges (AP-13/AP-22/AP-26, I2).
- **Anchor**: 2026-10-01 · live Argus design decision; delivery:forgejo-feature-draft#E-29.
- **Status**: live

### D-7 — Readiness-only intermediate hook deferral
- **Question**: DQ-7, which check can differ during a working round?
- **Answer**: Argus selects observed working-draft stage selection in the existing hook gate; only draft-ready is deferred there. Required PR readiness still reports the same verdict; preparation, compiled-head push, and lock retain all checks and lossless memory bindings.
- **Reasoning**: This implements the operator-directed FR-7 correction and makes the stage exception explicit; no memory debt, allowlist, policy, stamp, or CI weakening is authorized (AP-10/AP-11/AP-29, I5/I11).
- **Anchor**: 2026-10-01 · live Argus design decision; delivery:forgejo-feature-draft#E-29.
- **Status**: live

## Alternatives

### ALT-1 — Generalize the shared executor codehost port
- **Shape**: Add generic actor, exact proposal, ensure/edit/reviewer, and protection primitives to executor CodeHost; the draft lifecycle composes those with approval.Port and Git. Opening `company-memory` uses generic creation instead of fabricating a Task; preparation and recording consume the widened shared host.
- **Trade-offs**: Shares more adapter code and adds fewer interfaces; expands task adapter/fake/conformance obligations for capabilities the executor does not consume and couples this repair to task-port generalization.
- **Principles**: AP-1/AP-3 honored through neutral types/conformance; AP-11/AP-13/AP-15/AP-17/AP-22 honored through shared predicates and exact fail-closed observations; AP-27 requires additive registration; AP-30 configured inputs. Consumer ownership is strained by draft-only obligations on an executor port.
- **Incidents**: I2 ancestry; I8 retry/idempotence; I10 known pushed head; I5/I11 gate parity. Registry checked 2026-10-01.
- **Verdict**: rejected — feasible, but broadens NR-2's protected task surface unnecessarily for this focused correction.
- **From**: DQ-1 · parallel panel, minimal-interface constraint, Codex draft_design_minimal; recommendation only.

### ALT-2 — Draft-owned host and separate approval reader
- **Shape**: Add internal/draft/host with capability-specific typed proposal/protection operations; provider-registered draft adapters reuse common clients and credentials. Opening `company-memory` needs no task; existing core preparation and recording compose the selected host, approval reader, and Git.
- **Trade-offs**: Keeps task interfaces stable and draft-specific evidence explicit; costs a second capability-specific proposal model and two adapters, whose shared transport and approval predicates remain reused.
- **Principles**: AP-1/AP-3 consumer-owned port and conformance; AP-11 one readiness/approval/digest predicate; AP-13/AP-15 exact known-head readback; AP-17 ambiguous protection refuses; AP-22 leased reconciliation; AP-27 additive adapters; AP-30 configured paths/identities; RW-3 prefix workarounds stay inside Forgejo.
- **Incidents**: I2 mainline proof and I10 expected-head readback; registry checked 2026-10-01. I5/I11 constrain the common gate paths.
- **Verdict**: chosen — the smaller task blast radius satisfies the compatibility perimeter without duplicating the draft state machine.
- **From**: DQ-1 · parallel panel, containment constraint, Codex draft_design_contained; resolved by Argus in D-1.

## Seam map

- **S-1**: internal/draft/host — opens — typed proposal, actor, reviewer request, protection observations; feeds FR-1..FR-6, UB-1..UB-5, NR-1/NR-2.
- **S-2**: internal/draft/approval and native approval adapters — extends — exact-head configured-human observation; feeds FR-5/FR-6, UB-2/UB-4.
- **S-3**: lifecycle/lock and Git credential composition — extends — existing states, working files, stamped rebase, leases, tag/mainline verification; feeds FR-2..FR-6, UB-3/UB-5/UB-6, NR-3.
- **S-4**: source code-gate stage selection — extends — intermediate readiness-only deferral with unchanged complete final checks; feeds FR-7, DOD-6.

## Facts

- **F-1**: lifecycle Inspect/Open and lock Prepare/RecordPush currently invoke gh; draft-lock-record selects GitHub approval and requires GH_TOKEN — source: internal/draft/lifecycle/source.go/open.go; internal/draft/lock/prepare.go/push.go; cmd/draft-lock-record/main.go.
- **F-2**: executor CodeHost is task-shaped; existing approval.Port exposes exact-head Approval and a shared Select predicate — source: internal/executor/port/codehost.go; internal/draft/approval/port.go.
- **F-3**: Forgejo 16.0.5 exposes PullRequest.draft, but no draft field in Create/EditPullRequestOption; existing task adapter uses a title prefix then observes Draft — source: actual Swagger and internal/adapters/codehost/forgejo/host.go.
- **F-4**: Official v16 configuration defines case-insensitive WIP title prefixes; branch protections and supplied Swagger expose approval/stale/admin controls, whose runtime enforcement still needs native proof — source: [configuration](https://forgejo.org/docs/v16.0/admin/config-cheat-sheet/), [protection](https://forgejo.org/docs/v16.0/user/repository/protection/), captured Swagger SHA-256 `25f869a447c8c5a81e8745bbfb5dfa2c363f1faa6e36af793b061109f679836a`.
- **F-5**: The common Forgejo client contains credentials at the configured server/base path; scoped Git helper chooses the selected connection token and authenticated actor — source: internal/adapters/common/forgejo/client.go; internal/gitcredential/credential.go. Draft-specific repository redirect checks and actor selection are additional obligations.
- **F-6**: Existing recorder additions/trailer/review checks do not establish all FR-6 mainline/text/tag-byte promises; these need explicit core verification — source: internal/draft/lock/push.go/record.go.

## Fog

none — no unresolved direction or design input. Native privileged-merge/protection semantics are a mandatory implementation and release capability test in D-5, with a specified refusal when unsupported; no optimistic acceptance is assumed.

## Out of scope

- **O-1**: No parallel Forgejo pipeline, gh shim, or application migration — D-1; existing core workflow and operator host perimeter govern.
- **O-2**: No task codehost/tracker redesign, bundled local-Git implementation, or replay of #1307 — D-1/D-3; NR-2/NR-3 and operator A-2.
- **O-3**: No agent human approval/merge, authority promotion, memory-scope widening, or final-gate waiver — D-5/D-7; operator A-1..A-5.
- **O-4**: No product implementation or plan before this contract lock lands — D-4; operator A-2 and existing workflow.

## Candidate glossary terms

none — Draft host is the capability consumer, not a new workflow stage or authority. Existing draft, lock, proposal, and produced-authorship terms remain.

## Parked product gaps

none — current P-1..P-4 corrective edits were applied under A-5 before this session; no additional requirement change is inferred from design.

## Gate

| Field | Value |
|---|---|
| **Verdict** | ready-for-design |
| **Date** | 2026-10-01 |
| **Authority** | Argus, authority-contract/v1 tier-2; sign-gate-verdict read from current main c080733c at this act; not human lock approval. |
| **Covering design** | — |
| **Coverage check** | pass — direction D-1 with DQ-1 panel; ALT-1/ALT-2 fully scored; S-1..S-4 enumerate basis/scope; O-1..O-4 bound non-goals; all required sections present and zero blocking fog. |
| **Amend route** | — |
