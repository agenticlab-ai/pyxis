# Forgejo repository policy — v2 design-brainstorm record

| Field | Value |
|---|---|
| **In one line** | Extend the selected draft Host with the existing abandonment repository-policy query; preserve v1 directions and failure behavior. |
| **Relevant-when** | Completing the Forgejo draft host capability surface, mapping repository settings, or checking abandonment before mutation. |
| **Basis** | draft:forgejo-feature-draft/prd.md |
| **Authority** | Argus, engineering principal under authority-contract/v1 tier-2; engineering assumption delivery:forgejo-feature-draft#E-112. |
| **Date(s)** | 2026-10-02 |
| **Status** | gated: ready-for-design |
| **Tracking** | https://github.com/agenticlab-ai/pyxis/pull/1365 |
| **Feeds** | drafts/forgejo-feature-draft/tech-spec.md |
| **Prior records** | Frozen v1 brainstorm Documents 01M3WDAD7DXV57X6X9GQPKBJMY and 01M3WDAD7EAGY7VG48W6QA6WG4 under memory/brainstorm/forgejo-feature-draft/. |
| **Template version** | design-brainstorm-record/v1 |

## Orientation

| Source | Why loaded |
|---|---|
| drafts/forgejo-feature-draft/prd.md, FR-4/NR-1/NR-2/NR-3 | Existing abandonment semantics and compatibility perimeter; no requirement behavior is changed. |
| memory/brainstorm/forgejo-feature-draft/Document-01M3WDAD7EAGY7VG48W6QA6WG4.md | Frozen v1 direction and alternatives, retained below as facts rather than new rulings. |
| internal/draft/lifecycle/abandon.go | Reads delete_branch_on_merge before abandonment artifacts, PR updates, or branch publication. |
| docs/plans/forgejo-feature-draft/001-plan.md, T1/T5 | Frozen plan already names the missing repository-policy capability and native field mappings. |
| docs/architecture-principles.md, AP-1/AP-3/AP-17/AP-27; docs/observability/incidents.md | Consumer ownership, existing seam, typed refusal, additive adapters; I8's retry lesson applies. |
| docs/prds/feature-draft/001-local-git-receipts.md | Approved successor provenance FR-48/50/51 retains hosted human approval and freshness. |

**Overlaps surfaced:** V1 chose the draft-owned Host, separate approval reader and common lifecycle. D-1 preserves that placement and abandonment's existing policy precondition. Operator-approved P-9/D-2 narrows inherited F-5's enforced-protection direction for installed Forgejo only; its history remains below. Local-Git implementation and existing GitHub behavior remain separate.

## Design questions

- **DQ-1**: How should the selected draft Host expose the repository's automatic head-branch-deletion policy without adding another port or changing abandonment behavior?
- **DQ-2**: P-9's operator-resolved choice: should installed Forgejo enforce privileged/direct/force/path write controls or trust repository governance while verifying exact-head human approval and merged contents?

## Panel log

- **DQ-1**: extends — skipped — add one read operation and typed value to the already selected Host seam; no new or replacement interface or module boundary.
- **DQ-2**: scoped trust-model revision — panel skipped — product choice already ruled by the operator at P-9; retain the existing approval-reader seam and omit a new Forgejo protection-policy capability.

## Decisions

### D-1 — Complete the selected Host's repository-policy capability

- **Question**: DQ-1, where does the existing abandonment policy query belong?
- **Answer**: Argus extends Host with RepositoryPolicy(context.Context) (RepositoryPolicy, error), whose AutoDeleteHeadBranchesOnMerge value is returned only after an unambiguous native observation. Abandonment requires affirmative true before any mutation.
- **Reasoning**: The selected Host owns repository observations; a separate seam or direct core gh/HTTP call would undo the v1 placement. Missing, malformed, inaccessible and false settings retain the fail-closed precondition.
- **Anchor**: 2026-10-02 · live Argus engineering assumption, delivery:forgejo-feature-draft#E-112; v1 placement is prior fact F-1.
- **Status**: live

### D-2 — Operator-approved installed-Forgejo trust model

- **Question**: P-9, should native privileged/direct/force/path-bypass prevention be a blocking acceptance prerequisite for this single-user Forgejo route?
- **Answer**: Operator selects trusted repository governance for those write controls, while authenticated configured-human exact-head approval, stale/agent/uncertain review refusal and independent merged-content/ancestry proof remain mandatory for a lock.
- **Reasoning**: The operator approved the simpler model after its inherited stronger guarantee and additional native-fixture cost were explained. The original acceptance objective requests the simplest practical ordinary single-user workflow. This explicitly scoped successor to F-5 for installed Forgejo changes the enforcement boundary, not the required human review or GitHub behavior.
- **Anchor**: 2026-10-04 · operator conversation “ok, your proposa is aproved.” · draft problems.md P-9; recorded by Argus at authority-contract/v1 tier-2.
- **Status**: live; product ruling applied to the draft, separate exact-head lock approval pending

## Alternatives

### ALT-1 — Extend the existing draft Host
- **Shape**: Add the typed RepositoryPolicy read to Host; GitHub maps delete_branch_on_merge and Forgejo maps default_delete_branch_after_merge. Decode field presence and type before returning the consumer value.
- **Trade-offs**: Completes the selected capability surface without another port; both native adapters and the consumer conformance suite must implement the query.
- **Principles**: AP-1/AP-3 consumer ownership; AP-17 unknown policy refuses; AP-27 additive native adapters.
- **Incidents**: I8's observed retry/idempotence lesson retained; registry checked 2026-10-02.
- **Verdict**: chosen — preserves the existing precondition within the already selected seam.
- **From**: DQ-1 · hand-authored extension; panel skipped.

### ALT-2 — Trust installed-Forgejo repository governance
- **Shape**: Verify configured-human authenticated exact-head approval and merged artifacts; repository owners govern privileged/direct/force/path writes. GitHub retains existing checks.
- **Trade-offs**: Omits the inherited mandatory bypass-enforcement capability and proof suite; software cannot prevent or undo an out-of-band unapproved mainline write.
- **Verdict**: chosen by operator P-9/D-2 over inherited F-5's enforced-protection alternative; separate human lock approval remains required.
- **From**: DQ-2 · recorded product ruling, not a new engineering assumption.

## Engineering elaborations — review010, 2026-10-04

The current technical review's two findings clarify the existing F-4 preparation/retry and F-6 lock-recording directions. Argus reread authority-contract/v1 tier-2 and applied both blocking findings as engineering dispositions. FR-6, UB-6, DOD-4 and DOD-5 already require these outcomes; this record changes no acceptance criterion, P-9 ruling, frozen record or human lock boundary.

- **Installed recorder**: Keep the shipped draft-lock-record tool and GitHub workflow; define the installed Forgejo command with explicit verified mainline before/after IDs. The acceptance drive invokes it after the human merge and proves the resulting remote tag. Existing CI may invoke the same command, but a new daemon or mandatory Forgejo Actions deployment is unnecessary.
- **Preparation recovery**: Reuse the existing pending Git-ref/CAS transport for a typed preparation intent, retained original/prepared commits and exact body/phase evidence. Re-entry validates the same R/H/digest/mainline/PR before completing observed missing effects. The closed draft-event/v1 schema stays intact; the retained-parent initialization is an explicit extension of the current Git store. This extends an existing seam rather than replacing a module or adding a host port, so no new design panel is triggered.
- **Trade-off**: The explicit post-merge command requires one ordinary installed invocation. Durable preparation evidence adds one small typed sidecar and CAS recovery path; it prevents recompilation or duplicate uncertain writes after a crash without adding an orchestration service. Those details feed tech-spec D-4/D-6 and the same lifecycle verification.

## Seam map

- **S-1**: internal/draft/host — extends — typed same-repository policy query; feeds FR-4/NR-1/NR-2 and tech-spec D-1/D-3.
- **S-2**: selected native adapters and common abandonment lifecycle — extends — field observation and affirmative precondition before mutation; feeds FR-4/UB-5/UB-6.

## Facts

The following F-1..F-9 are attributed v1 evidence from the frozen design-brainstorm Document 01M3WDAD7EAGY7VG48W6QA6WG4 and its 2026-10-01 Argus session (delivery E-29). They are not new answers in this session.

- **F-1**: V1 D-1 selected a draft-owned typed Host, provider-registered GitHub/Forgejo adapters and separate reusable approval.Port.
- **F-2**: V1 D-2 selected offline remote/binding validation before credential-bearing requests and child-only draft credential selection with a separate operator.
- **F-3**: V1 D-3 selected complete native observations, deterministic proposal identity, known-head leases and the existing state machine.
- **F-4**: V1 D-4 selected supported preparation/stamp refresh, complete final gates, locally known prepared H and bounded exact-head readback.
- **F-5**: V1 D-5 selected effective configured-human protection, stale and privileged enforcement, native negative proof and refusal for unsupported protection.
- **F-6**: V1 D-6 selected independent exact-head review, mainline/compiled-content proof and raw annotated-tag byte verification with race reconciliation.
- **F-7**: V1 D-7 selected readiness-only intermediate hook deferral; full memory, final readiness and operator approval remain mandatory.
- **F-8**: V1 ALT-1 rejected widening the executor task codehost port because draft-only capabilities broaden NR-2's task surface.
- **F-9**: V1 ALT-2 selected the contained draft-owned host and separate approval reader, reusing common transports.
- **F-10**: Current GitHub abandonment requires native delete_branch_on_merge to equal true before its first mutation — source: internal/draft/lifecycle/abandon.go.
- **F-11**: Native Forgejo 16.0.5 Repository schema exposes boolean default_delete_branch_after_merge — source: captured Swagger Repository projection acceptance/forgejo-abandonment-repository-policy-schema.json in the acceptance controller workspace; frozen plan001 T5 records the same mapping. Runtime adapter/protection proof remains implementation work.
- **F-12**: Approved feature-draft amendment001 supersedes FR-17/46/47 with FR-48/50/51; their hosted clauses preserve authenticated operator approval and changed-head invalidation — source: docs/prds/feature-draft/001-local-git-receipts.md.

## Fog

none — field absence or ambiguous observation is a refusal to test, not an unanswered direction.

## Out of scope

- **O-1**: No new port, task/tracker redesign, repository-setting write, host fallback or weaker abandonment precondition — from D-1.
- **O-2**: D-2 changes only installed-Forgejo protection enforcement under P-9's operator ruling; no local-Git implementation, frozen record/plan edit, GitHub policy change or human lock approval is included.

## Candidate glossary terms

none — RepositoryPolicy names the existing policy observation, not another authority or stage.

## Parked product gaps

none — no EARS, fit criterion, priority or requirement ID changes; active provenance refresh follows F-12.

## Gate

| Field | Value |
|---|---|
| **Verdict** | ready-for-design |
| **Date** | 2026-10-02 |
| **Authority** | Argus; authority-contract/v1 tier-2 sign-gate-verdict reread on 2026-10-04 at current main 2a2125c2; D-2's product choice is operator P-9, not Argus acceptance or human lock approval. |
| **Covering design** | — |
| **Coverage check** | pass — D-1/D-2, logged DQ-1/DQ-2 skips, chosen ALT-1/ALT-2, retained basis/seams S-1/S-2 and scoped O-1/O-2 cover the core four; inherited F-5 remains history with explicit operator-scoped successor D-2; all sections present, zero blocking fog. |
| **Amend route** | — |
