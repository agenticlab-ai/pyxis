# Feature implementation drive — design-brainstorm record

| Field | Value |
|---|---|
| **In one line** | Choose the durable orchestration, ownership, integration, candidate-verification, and delivery seams for one feature drive above the feature-blind executor. |
| **Relevant-when** | designing or changing feature-scoped implementation ownership, DAG-to-feature-branch integration, local/remote drive resumption, candidate-merge e2e evidence, repair loops, or final verified delivery |
| **Basis** | `prd:feature-implementation-drive` (merged ledger) |
| **Authority** | @mdarabi |
| **Date(s)** | 2026-07-25 |
| **Status** | gated: ready-for-design |
| **Tracking** | — |
| **Feeds** | `docs/tech-design/feature-implementation-drive.md` |
| **Prior records** | `brainstorm:feature-implementation-drive` |
| **Template version** | design-brainstorm-record/v1 |

## Orientation

| Source | Why loaded |
|---|---|
| `docs/prds/feature-implementation-drive/effective.md` | merged requirement basis and open design-owned mechanics |
| `docs/brainstorms/feature-implementation-drive.md` | feeding product rulings and parked technical constraints |
| `docs/tech-design/implement-now.md` | existing feature-blind Worker, Dispatcher, frontier-sweep, workspace, lease, and proposal boundaries |
| `docs/tech-design/merge-lane-v1.md` | existing human/automatic proposal-landing authority and goal-object verification boundary |
| `docs/tech-design/fleet-bridge-v1.md` | existing unattended progression, wake, and ancestry-verified closure discipline |
| `docs/architecture-principles.md` | governing seams, evidence, ownership, isolation, and goal-object principles |
| `docs/observability/incidents.md` | incident check, especially I2's side-branch false-delivery class |
| `docs/wiki/glossary.md` | canonical feature-drive, execution-ownership, and candidate-merge terms |

**Overlaps surfaced:** no live design serves `prd:feature-implementation-drive`; adjacent designs own the executor below the drive, proposal landing, unattended wake/progression, and ancestry verification, so this design must compose rather than replace them.

## Design questions

- **DQ-1**: What is the durable core shape that owns one feature drive's state transitions and composes local and remote execution without teaching the Worker feature semantics?
- **DQ-2**: What medium-neutral ownership protocol and Git/GitHub binding provide exclusive domain assignment, local machine affinity, checkpointed handoff, and fail-closed stale-owner recovery?
- **DQ-3**: How does the drive verify task-PR landing on a non-default feature branch and advance the frozen DAG without relying on default-branch issue-closing semantics?
- **DQ-4**: What deterministic candidate-merge and evidence contracts bind both input heads, the candidate identity, runners, attestations, and scenario outcomes?
- **DQ-5**: How are review, merge, remote-pool, and e2e events converted into resumable drive touches while preserving one behavior across attended and unattended modes?
- **DQ-6**: What state transitions and retained artifacts govern cancellation, explicit resume or abandonment, final-main ancestry verification, ownership release, and cleanup?

## Panel log

- **DQ-1**: opens — fired — introduces the feature-drive orchestration and durable-state seam above the existing feature-blind Dispatcher.
- **DQ-2**: opens — fired — introduces the cross-machine feature-ownership and aggregate-consistency contract; task leases cannot express future-task affinity.
- **DQ-3**: extends — skipped — composes the existing CodeHost goal-object verification and WorkTracker completion seams for a non-default feature base; no new boundary is required.
- **DQ-4**: opens — fired — introduces the candidate-construction identity, publication, and SHA-bound evidence seam consumed by independently placed runners.
- **DQ-5**: extends — skipped — reuses the existing instruction-free push-preferred, backstop-guaranteed touch channel; the additive work is owner-domain routing and aggregate reconciliation.
- **DQ-6**: extends — skipped — completes the lifecycle states of the chosen feature-drive aggregate without introducing another seam.

## Decisions

### D-1 — durable aggregate reconciled by short-lived touches

- **Question**: what durable core should own one feature drive while keeping the Worker feature-blind?
- **Answer**: choose ALT-1: one persisted `FeatureDrive` aggregate and deterministic reconciler, invoked identically by short-lived local commands and remote wake workers through typed ports.
- **Reasoning**: the authority approved the recommendation; it gives local and remote drives one resumable behavior without the complexity of full event sourcing or ambiguous platform-state reconstruction.
- **Anchor**: 2026-07-25 · “go for it”
- **Status**: live

### D-2 — CAS aggregate with owner epochs and explicit handoff

- **Question**: how should exclusive local/remote ownership be stored and transferred?
- **Answer**: choose ALT-4: a medium-neutral CAS `FeatureDriveStore`, first bound to one protected remote Git ref per feature; ownership carries domain, stable owner id, state, and a monotonically increasing fencing epoch.
- **Reasoning**: the authority approved the recommendation; quiesce-then-CAS handoff and epoch checks prevent stale owners from resuming, while heartbeats remain diagnostic and never authorize automatic takeover.
- **Anchor**: 2026-07-25 · “yes” after DQ-2 recommendation
- **Status**: live

### D-3 — explicit tracker completion only after feature-branch goal verification

- **Question**: how should a task merge unlock its dependents without relying on default-branch closing semantics?
- **Answer**: bind the proposal to its reviewed head and feature-branch base, resolve the resulting landed revision, verify that revision is an ancestor of the current feature branch, then idempotently `RecordLanding`; any mismatch advances nothing and blocks the drive.
- **Reasoning**: the authority approved the recommendation; this supports merge, squash, and rebase results while preserving human review and making the existing tracker blocker predicate the sole readiness derivation.
- **Anchor**: 2026-07-25 · “yes” after DQ-3 recommendation
- **Status**: live

### D-4 — deterministic synthetic candidate published by immutable identity

- **Question**: how should the exact candidate be constructed and made available to e2e runners?
- **Answer**: choose ALT-7: deterministically merge exact feature and main heads into a canonical synthetic two-parent commit, publish it under an immutable content-addressed candidate ref, and bind every result or attestation to its complete identity.
- **Reasoning**: the authority approved the recommendation; one fetchable revision works across local, remote, CI, specialized, and manual routes without mutating either source branch or creating the final proposal prematurely.
- **Anchor**: 2026-07-25 · “yes” after DQ-4 recommendation
- **Status**: live

### D-5 — instruction-free touches routed to the fenced owner domain

- **Question**: how should review, merge, and e2e events resume the correct local or remote drive?
- **Answer**: reuse the wake channel with `{feature id, reason}` signals; route by the aggregate's current owner, run a bounded owner-domain backstop, then reload, verify goal objects, and reconcile only under the current fencing epoch.
- **Reasoning**: the authority approved the recommendation; local and remote modes share one behavior, duplicate or stale delivery converges to CAS no-ops, and review waits retain zero model or worker processes.
- **Anchor**: 2026-07-25 · “yes” after DQ-5 recommendation
- **Status**: live

### D-6 — quiesce before release; prove delivery before cleanup

- **Question**: how should cancellation, abandonment, final review, delivery verification, ownership release, and cleanup transition?
- **Answer**: cancellation quiesces before releasing and preserving; resume returns to unowned and requires a fresh assignment; abandonment is explicit and terminal but non-destructive; delivery releases only after verified main ancestry, then policy-controlled cleanup may begin.
- **Reasoning**: the authority approved the full recommended ordering; stale candidate evidence returns to verification, wrong-main delivery stays blocked and owned, and no stop or terminal transition silently destroys durable artifacts.
- **Anchor**: 2026-07-25 · “yes” after DQ-6 recommendation
- **Status**: live

## Alternatives

### ALT-1 — Durable reconciler over a feature-drive aggregate

- **Shape**: One medium-neutral `FeatureDrive` aggregate persists desired ownership plus observed lifecycle state. Short-lived attended commands and remote wake workers both invoke the same deterministic reconciler, which emits typed effects through existing tracker, code-host, executor, e2e, and notification ports.
- **Trade-offs**: One behavior and resumable state across machines; explicit idempotency and effect journal cost more design work up front.
- **Principles**: AP-2 honored by one normalized touch path; AP-6 honored by a closed aggregate/effect schema; AP-21 honored by consuming prior human judgments as data; AP-23 honored by one aggregate per feature.
- **Incidents**: I2 matched — closure and landing effects must verify ancestry at the goal branch, never trust a merged badge.
- **Verdict**: chosen — authority approved the one-behavior, resumable aggregate/reconciler shape.
- **From**: DQ-1 · panel

### ALT-2 — Event-sourced feature-drive journal

- **Shape**: Every observed event and requested act appends to an immutable per-feature event stream; current state is folded from that stream, and local or remote processors claim stream positions before emitting effects.
- **Trade-offs**: Maximum audit and replayability; introduces event ordering, schema migration, snapshotting, and split-brain complexity beyond the v1 repository slice.
- **Principles**: AP-5 strongly honored by complete provenance; AP-6 honored only with a strict event schema; AP-16 strained by replay/reconciliation bounds; AP-23 honored by per-feature streams.
- **Incidents**: I2 matched — ancestry evidence still must be an event payload verified at the goal object; replay cannot turn badge state into delivery proof.
- **Verdict**: rejected — audit power exceeds the product's v1 need and enlarges the state seam.
- **From**: DQ-1 · panel

### ALT-3 — Branch-and-tracker-derived stateless coordinator

- **Shape**: Persist no first-class drive aggregate; each invocation reconstructs phase and ownership from the feature branch, task issues/PRs, labels, checks, and git refs, then invokes the next action.
- **Trade-offs**: Minimal new storage and naturally remote-visible state; distributed platform artifacts cannot cleanly express checkpointed handoff, cancellation, stale evidence, or atomic ownership transitions, making inference and recovery ambiguous.
- **Principles**: AP-18 honored by reading observed reality; AP-6 strained because one contract is scattered across media; AP-11 strained by duplicated derivation predicates; AP-21 strained by repeatedly re-judging reconstructed intent.
- **Incidents**: I2 directly matched — platform badges and side-branch topology can misrepresent the goal object, making reconstruction unsafe without a durable contract.
- **Verdict**: rejected — insufficiently explicit for fail-closed ownership and candidate-evidence state.
- **From**: DQ-1 · panel

### ALT-4 — Versioned ownership inside a CAS feature-drive record

- **Shape**: A medium-neutral `FeatureDriveStore` exposes compare-and-swap load/create/transition over one versioned aggregate per feature. Ownership is `{domain, stable owner id, epoch, state}`; Git/GitHub binds the record to a dedicated remote git ref whose object carries closed versioned data. Handoff quiesces, then atomically advances the epoch and owner.
- **Trade-offs**: Reuses the repository's proven shared git-ref CAS and keeps state beside the repo; requires careful ref authorization, immutable transition evidence, and bounded retry on CAS loss.
- **Principles**: AP-6 honored by a closed schema; AP-11 honored by one ownership predicate; AP-17 honored by CAS-loss no-op and fail-closed owner mismatch; AP-22 honored by convergent retries.
- **Incidents**: I2 matched — the aggregate records evidence, but landing/delivery still verifies branch ancestry at the named goal object.
- **Verdict**: chosen — authority approved the Git-first CAS binding and fencing-epoch protocol.
- **From**: DQ-2 · panel

### ALT-5 — Code-host issue as ownership ledger

- **Shape**: A feature coordination issue carries owner labels and machine/domain metadata in comments or fields; conditional issue updates serialize assignment and handoff, while branch and PR state carry the remaining drive state.
- **Trade-offs**: Highly visible and requires no hidden refs; code-host fields/comments lack a portable atomic transition contract and invite partial updates, human edits, and duplicated derivation logic.
- **Principles**: AP-2 honored by native action visibility; AP-6 and AP-11 strained by splitting one contract across labels, fields, and prose; AP-17 strained on ambiguous edits.
- **Incidents**: I2 matched — platform presentation state has already proven insufficient as goal-object truth.
- **Verdict**: rejected — visibility is useful as a projection, not as the ownership oracle.
- **From**: DQ-2 · panel

### ALT-6 — Central transactional coordination service

- **Shape**: A network service/database stores feature aggregates, owner registrations, heartbeats, epochs, and transitions transactionally; local and remote drivers call it before every effect.
- **Trade-offs**: Strong transactions, querying, and fleet observability; adds mandatory infrastructure, availability, identity, migration, and operator burden that violates the one-repository minimal vertical slice.
- **Principles**: AP-6 honored by an API schema; AP-14 strained because local use now depends on deployed infrastructure; AP-19 strained by service steady-state cost; AP-23 honored by per-feature rows.
- **Incidents**: none checked `docs/observability/incidents.md` 2026-07-25 beyond I2, which remains orthogonal to service storage.
- **Verdict**: rejected — disproportionate infrastructure for v1 and weakens local-first operation.
- **From**: DQ-2 · panel

### ALT-7 — Canonical synthetic candidate commit on an immutable candidate ref

- **Shape**: Fetch exact feature and main heads; use deterministic Git plumbing to produce their merge tree and a synthetic two-parent commit with canonical metadata. Publish the object at a content-addressed candidate ref that never moves; evidence keys on feature head, main head, candidate commit, tree, suite version, and outcomes.
- **Trade-offs**: Any local, remote, CI, or specialized runner can fetch one exact revision; adds a namespaced disposable-ref lifecycle and requires canonical conflict/error handling.
- **Principles**: AP-6 honored by a closed evidence schema; AP-13 honored by one verifiable candidate artifact; AP-22 honored by content-addressed idempotent publication; AP-23 honored by per-candidate immutable refs.
- **Incidents**: I2 matched — both inputs and the published candidate are verified by object identity, never inferred from a branch badge.
- **Verdict**: chosen — authority approved canonical construction and immutable candidate-ref publication.
- **From**: DQ-4 · panel

### ALT-8 — Code-host-generated test merge

- **Shape**: Open or update the final feature-to-main proposal first and consume the code host's synthetic merge ref/check-suite revision as the candidate; e2e evidence attaches to that platform revision.
- **Trade-offs**: Native CI integration and no custom publication; reverses the required order by creating the final proposal before accepted evidence, platform merge refs can regenerate or disappear, and semantics are medium-specific.
- **Principles**: AP-2 honored through native checks; AP-13 strained by a mutable platform-generated artifact; AP-3 strained by privileged code-host semantics; AP-11 strained when local and remote paths differ.
- **Incidents**: I2 matched directly — platform status and branch topology cannot be accepted without independent goal-object verification.
- **Verdict**: rejected — conflicts with the basis's evidence-before-final-proposal gate and medium neutrality.
- **From**: DQ-4 · panel

### ALT-9 — Independent per-runner merge reconstruction

- **Shape**: Publish only the two input heads and merge policy; every runner independently fetches and constructs its own candidate, reporting the resulting tree/commit with its outcome. The drive accepts evidence only if every runner reports one identity.
- **Trade-offs**: No candidate-ref cleanup and no centralized build artifact; tool/version/environment differences can produce mismatches, manual-human routes lack a simple checkout target, and every runner duplicates conflict handling.
- **Principles**: AP-14 honored by exercising each real runner; AP-11 strained by multiple candidate constructors; AP-13 strained until cross-runner identity convergence completes; AP-16 strained by mismatch recovery.
- **Incidents**: none checked `docs/observability/incidents.md` 2026-07-25 beyond I2; I2 still requires exact-object verification rather than runner claims.
- **Verdict**: rejected — distributes the most safety-critical derivation and complicates manual evidence.
- **From**: DQ-4 · panel

## Seam map

- **S-1**: feature-drive aggregate and reconciler — opens — behind it: durable lifecycle state plus deterministic next-effect selection — feeds: design Decision and core Scope
- **S-2**: feature-drive store and ownership transition contract — opens — behind it: CAS persistence, owner epoch fencing, quiesce/handoff/release — feeds: design ownership and consistency sections
- **S-3**: task landing evidence to tracker completion — extends — behind it: reviewed-head/base binding, landed-revision ancestry, idempotent `RecordLanding` — feeds: integration and DAG progression
- **S-4**: candidate builder and evidence contract — opens — behind it: deterministic merge identity, immutable publication, scenario and attestation binding — feeds: e2e orchestration and final-proposal eligibility
- **S-5**: owner-routed drive touch — extends — behind it: instruction-free signals, owner-domain backstop, epoch verification, duplicate convergence — feeds: attended/unattended resumption and observability
- **S-6**: feature-drive lifecycle transitions — extends — behind it: quiesce, cancel/resume/abandon, final review, ancestry-verified delivery, release-before-cleanup — feeds: state model, failure modes, migration and cleanup

## Facts

- **F-1**: The merged basis requires one feature-aware lifecycle above a feature-blind executor, with explicit exclusive local or remote ownership and no automatic takeover or merge. — source: `prd:feature-implementation-drive/FR-1..FR-7`, `NR-1`
- **F-2**: `implement-now` is already a frontier-sweep Dispatcher over a portable single-task Worker; it never merges, stacks, or owns feature lifecycle state. — source: `docs/tech-design/implement-now.md` §2, §3.2, §3.6
- **F-3**: Existing task claims prevent duplicate execution of one current issue but do not reserve future-unblocked tasks of a feature to one execution domain. — source: `brainstorm:feature-implementation-drive#F-4`; `prd:feature-implementation-drive` §2.1
- **F-4**: The existing merge and unattended-drive designs require goal-object verification, including mainline ancestry, rather than trusting proposal badges. — source: `docs/tech-design/merge-lane-v1.md`; `docs/tech-design/fleet-bridge-v1.md` #DINV-4
- **F-5**: The basis requires process-free local review waits, independently routed e2e runners, candidate evidence stale on either input-head change, and preserved artifacts on cancellation. — source: `prd:feature-implementation-drive/FR-13`, `FR-16..FR-19`, `FR-25..FR-30`

## Fog

- **G-1** (non-blocking — blocks: design Q-1): exact repository retention durations for delivered/abandoned feature branches, candidate refs, and evidence; ordering and explicit-confirmation constraints are fixed, values remain operator/config policy per `prd:feature-implementation-drive` Q5.

## Out of scope

- **O-1**: Automatic merge of task or final proposals — the drive proposes and verifies but never gains merge authority. (from D-1, D-3, D-6)
- **O-2**: Feature semantics inside the executor Worker — the reconciler translates lifecycle state into ordinary executor assignments. (from D-1)
- **O-3**: Automatic stale-owner takeover — availability signals diagnose; only explicit handoff, cancel, resume, or abandonment changes ownership. (from D-2)
- **O-4**: A central coordination service — the v1 binding is repository-native CAS behind a medium-neutral store port. (from D-2)
- **O-5**: Code-host-generated candidate merges as the delivery-evidence oracle — candidates are independently constructed and content-addressed before a final proposal exists. (from D-4)
- **O-6**: Multi-repository feature DAGs and fleet-wide fairness — v1 is one feature in one repository and consumes existing remote scheduling policy. (from D-1)
- **O-7**: A unified feature dashboard — action-required state stays on proposal, tracker, command, and notification surfaces with a minimal diagnostic. (from D-5)
- **O-8**: Destructive cancellation or abandonment — durable artifacts survive until a separate confirmed cleanup act. (from D-6)

## Candidate glossary terms

none

## Parked product gaps

none

## Gate

| Field | Value |
|---|---|
| **Verdict** | ready-for-design |
| **Date** | 2026-07-25 |
| **Authority** | @mdarabi |
| **Covering design** | — |
| **Coverage check** | pass — core four covered, zero blocking fog |
| **Amend route** | — |
