# Shared reviewer source protection — technical design

| Field | Value |
|---|---|
| **Status** | Draft → Approved *(operator source review and human merge required)* |
| **Owner** | @mdarabi |
| **Approver(s)** | @mdarabi (operator and security reviewer) |
| **Date** | 2026-10-01 |
| **Relevant-when** | changing reviewer tool permissions, native sandbox settings, or the shared reviewer process guard |
| **Scope** | Source and Git write protection for reviewer processes, with writable private runtime state |
| **Tracking** | [Issue #1340](https://github.com/agenticlab-ai/pyxis/issues/1340) · replacement for closed [PR #1343](https://github.com/agenticlab-ai/pyxis/pull/1343) |
| **Builds-on** | docs/tech-design/native-routing-policy-v1.md |
| **Supersedes** | docs/tech-design/implement-now-authenticated-review-obligations.md · docs/tech-design/impl-review-host-persistence.md — scoped replacement; incorporation below preserves the other decisions |
| **Serves** | prd:impl-review/FR-3 · prd:impl-review/FR-4 · prd:impl-review/FR-5 · prd:impl-review/FR-6 · prd:impl-review/FR-7 · prd:impl-review/FR-12 · prd:impl-review/FR-14 · prd:impl-review/FR-15 · prd:impl-review/FR-16 · prd:impl-review/FR-18 · prd:impl-review/FR-19 · prd:impl-review/FR-20 · prd:impl-review/FR-21 · prd:impl-review/FR-22 · prd:impl-review/FR-23 · prd:impl-review/FR-24 · prd:impl-review/FR-25 · prd:impl-review/FR-26 · prd:impl-review/FR-27 · prd:impl-review/FR-28 · prd:impl-review/FR-29 · prd:impl-review/FR-30 · prd:impl-review/FR-31 · prd:impl-review/FR-32 · prd:impl-review/FR-33 · prd:impl-review/FR-34 · prd:impl-review/FR-35 · prd:impl-review/FR-36 · prd:impl-review/FR-37 · prd:impl-review/NR-1 · prd:impl-review/NR-2 · prd:impl-review/NR-3 · prd:impl-review/NR-5 · prd:impl-review/NR-6 · prd:impl-review/NR-7 · prd:impl-review/NR-8 · prd:impl-review/UB-3 · prd:impl-review/UB-9 · prd:impl-review/UB-10 · prd:impl-review/UB-11 · prd:impl-review/UB-12 · prd:impl-review/UB-13 · prd:impl-review/UB-14 · prd:impl-review/UB-15 · prd:impl-review/UB-16 · prd:impl-review/UB-17 · prd:impl-review/UB-18 · prd:impl-review/UB-19 · prd:impl-review/UB-20 · prd:impl-review/UB-21 · prd:impl-review/UB-22 · prd:impl-review/UB-23 · prd:impl-review/ABUSE-3 · prd:impl-review/ABUSE-4 · prd:impl-review/ABUSE-5 · prd:impl-review/ABUSE-6 · prd:impl-review/ABUSE-7 · prd:impl-review/NFR-6 · prd:impl-review/NFR-7 · prd:impl-review/NFR-8 · prd:impl-review/NFR-10 · prd:impl-review/NFR-11 · prd:impl-review/NFR-12 · prd:impl-review/EVAL-1 · prd:impl-review/EVAL-2 · prd:impl-review/EVAL-3 · prd:impl-review/EVAL-4 · prd:impl-review/EVAL-5 · prd:impl-review/EVAL-6 · prd:impl-review/EVAL-7 · prd:impl-review/EVAL-8 · prd:impl-review/EVAL-9 · prd:impl-review/EVAL-10 · prd:impl-review/EVAL-11 · prd:impl-review/EVT-2 · prd:impl-review/EVT-3 · prd:impl-review/EVT-6 · prd:impl-review/EVT-7 · prd:impl-review/EVT-8 · prd:impl-review/EVT-9 · prd:impl-review/EVT-10 · prd:impl-review/EVT-11 · prd:impl-review/EVT-12 · prd:impl-review/EVT-13 · prd:planner-driven-model-routing/FR-14 · prd:planner-driven-model-routing/FR-25 · prd:planner-driven-model-routing/FR-26 · prd:planner-driven-model-routing/NR-6 · prd:planner-driven-model-routing/NR-9 · prd:planner-driven-model-routing/UB-12 · prd:planner-driven-model-routing/UB-13 · prd:implement-now/FR-1 · prd:implement-now/FR-2 · prd:implement-now/FR-3 · prd:implement-now/FR-4 · prd:implement-now/FR-5 · prd:implement-now/FR-7 · prd:implement-now/FR-8 · prd:implement-now/FR-9 · prd:implement-now/FR-10 · prd:implement-now/FR-11 · prd:implement-now/FR-13 · prd:implement-now/FR-14 · prd:implement-now/FR-15 · prd:implement-now/FR-16 · prd:implement-now/FR-17 · prd:implement-now/FR-18 · prd:implement-now/FR-19 · prd:implement-now/FR-20 · prd:implement-now/FR-21 · prd:implement-now/FR-22 · prd:implement-now/FR-23 · prd:implement-now/FR-24 · prd:implement-now/FR-25 · prd:implement-now/FR-26 · prd:implement-now/FR-28 · prd:implement-now/FR-29 · prd:implement-now/FR-30 · prd:implement-now/FR-31 · prd:implement-now/FR-32 · prd:implement-now/FR-35 · prd:implement-now/FR-40 · prd:implement-now/FR-41 · prd:implement-now/FR-42 · prd:implement-now/FR-43 · prd:implement-now/FR-44 · prd:implement-now/FR-45 · prd:implement-now/FR-46 · prd:implement-now/FR-47 · prd:implement-now/FR-48 · prd:implement-now/FR-49 · prd:implement-now/NR-1 · prd:implement-now/NR-2 · prd:implement-now/NR-3 · prd:implement-now/NR-4 · prd:implement-now/NR-5 · prd:implement-now/NR-6 · prd:implement-now/NR-8 · prd:implement-now/NR-10 · prd:implement-now/UB-1 · prd:implement-now/UB-2 · prd:implement-now/UB-3 · prd:implement-now/UB-4 · prd:implement-now/UB-5 · prd:implement-now/UB-6 · prd:implement-now/UB-7 · prd:implement-now/UB-8 · prd:implement-now/UB-9 · prd:implement-now/UB-11 · prd:implement-now/UB-12 · prd:implement-now/UB-13 · prd:implement-now/UB-14 · prd:implement-now/UB-15 · prd:implement-now/UB-16 · prd:implement-now/UB-17 · prd:implement-now/UB-18 · prd:implement-now/UB-20 · prd:implement-now/UB-21 · prd:implement-now/ABUSE-1 · prd:implement-now/ABUSE-2 · prd:implement-now/ABUSE-3 · prd:implement-now/ABUSE-4 · prd:implement-now/ABUSE-5 · prd:implement-now/ABUSE-6 · prd:implement-now/NFR-1 · prd:implement-now/NFR-2 · prd:implement-now/NFR-3 · prd:implement-now/NFR-4 · prd:implement-now/NFR-5 · prd:implement-now/NFR-6 · prd:implement-now/NFR-7 · prd:implement-now/NFR-9 · prd:implement-now/EVT-1 · prd:implement-now/EVT-3 · prd:implement-now/EVT-4 · prd:implement-now/EVT-7 · prd:implement-now/EVT-8 · prd:implement-now/EVT-9 · prd:implement-now/EVT-10 · prd:implement-now/EVT-11 · prd:implement-now/EVT-12 · prd:implement-now/EVT-15 · prd:implement-now/EVT-16 · prd:implement-now/EVT-17 · prd:implement-now/EVAL-3 · prd:planner-driven-model-routing/FR-17 · prd:planner-driven-model-routing/NR-8 |
| **Security/privacy impact** | yes — changes native reviewer permissions and explicitly narrows the filesystem guarantee to writes; reviewer: @mdarabi |
| **Template version** | tech-design-template/v1 |

## 0. Read to rule — the human reviewer's section (~2 min)

*The body is the contract; this section only routes attention — on any conflict the body governs; machines ignore it.*

**Judge these** — Use one host process guard for source writes, with ordinary harness tools (§2), instead of composing native sandboxes or adding a command broker (§4).

**Sanity-check the numbers** — One private runtime directory is writable for each review (§3.1); no unguarded retry is allowed (§3.2).

**Open before locking** — none; the operator has approved the implementation approach. Source review and human PR approval remain required.

**Skip map** — §1 and §4 explain the failure and alternatives; §2, §3, and §6 govern the replacement.

## 1. Context

The installed Codex review route attempted a native Seatbelt sandbox inside Pyxis's existing process sandbox on macOS. Ordinary source reads failed before producing a grounded verdict. Issue #1340 retains the reproduction. The command broker proposed in #1343 introduced a separate execution path and a host endpoint.

The operator asked for a universal mechanism that lets reviewers read and inspect code while preventing code changes, then approved implementing the shared process guard in this session: “ok, like this approach more. implement this; i will close 1343 then”. This decision captures that source protection scope for the operator's single-user deployment. Reconsider it before treating untrusted reviewers as isolated from host secrets or other users.

## 2. Decision

**Decision:** Pyxis owns reviewer filesystem write permissions at the OS process boundary. Each reviewer and its descendants receive a read-only exact-revision checkout and read-only Git administration, with writable private runtime state outside the checkout. Harnesses use ordinary command and inspection tools under this guard. Native permissions must allow unattended inspection and must not install a conflicting nested sandbox.

This successor incorporates `implement-now-authenticated-review-obligations.md` at Git blob `22354a96c09d4658cca33076e9dbb3e360d20e86` and `impl-review-host-persistence.md` at Git blob `740ef89b282718247bf13c8f1105b78b3cfb1194`. Their contracts continue to apply except for the reviewer filesystem/read isolation claims and native tool restrictions replaced in §3. In particular, the read allowlist in the former's §3.6/DINV-7 and the latter's §3.7 is replaced by §3.3. All lifecycle, routing, artifact verification, publisher ownership, and human merge rules remain binding.

## 3. Mechanism

### 3.1 Write authority

The host prepares a disposable checkout at the captured revision and a separate private runtime directory. The shared guard permits file writes only inside that runtime directory, plus the device access required for process IO. It denies source writes, Git metadata writes, and writes to other host filesystem paths. Canonical path validation rejects overlapping checkout, runtime, and Git administration roots.

Temporary files, harness state, and build caches use the private directory through the closed launch environment. A command that normally generates files in the checkout must redirect its output there or fail. Source immutability does not imply every repository's test command can run unchanged.

### 3.2 Launch and lifetime

Every registered adapter passes review purpose to the same runtime guard before starting its CLI. The runtime tests whether the OS can apply that exact policy before inference. Missing or unusable confinement is a terminal infrastructure failure; it never triggers an unguarded launch. Platform support follows the shared runtime's supported confinement implementations.

The reviewer is a fresh session. All command children remain inside the confined process tree; a harness must not delegate review execution to a preexisting unconfined daemon. The host owns cancellation, descendant reaping, the final unchanged-checkout check, and cleanup. Native control captures and adapter generations must describe the actual review launch.

### 3.3 Security & privacy

**Decision:** This guard establishes filesystem write ownership. It does not establish a host read allowlist, prevent reading arbitrary host files, restrict network destinations, or protect against another process running as the same OS user. Native permission flags must not be represented as those additional guarantees.

The launch environment still excludes inherited code-host and SSH credentials, redirects user settings and caches to private state, and snapshots only the selected provider login material. The reviewer receives no host command endpoint or publisher interface. Its report crosses the existing bounded terminal handoff; the host coordinator validates it and the host persistence adapter publishes it. Environment exclusion is not a guarantee that host credentials cannot be discovered through filesystem reads or OS services.

- **DINV-1** — Reviewer processes and descendants cannot modify source, Git administration, another workspace, or other host filesystem paths outside their private runtime state and required IO devices.
- **DINV-2** — Guard admission failure starts no reviewer and has no unguarded fallback.
- **DINV-3** — Tool permissions are paired with the shared review guard; ordinary tool access grants no filesystem write authority outside private runtime state.
- **DINV-4** — Review completion still requires the inherited exact-revision handoff, coordinator validation, authenticated receipt, and reciprocal terminal predicates.

### 3.4 Observability

Guard setup, launch, cancellation, and unchanged-checkout failures use the inherited infrastructure failure and review terminal paths. This decision adds no events, counters, or command broker logs. Native command results remain inside the bounded review execution output; they do not become canonical completion evidence by themselves.

## 4. Alternatives

- **ALT-1 — Native read-only modes plus the shared guard:** rejected for this route because the observed nested sandbox prevents source reads; native mode semantics also differ across harnesses.
- **ALT-2 — A host command broker:** rejected in favor of the operator-approved common process guard. It adds another tool transport and execution policy to maintain.
- **ALT-3 — Instructions or file mode bits alone:** rejected because a reviewer running as the file owner can bypass them; write denial must be enforced by the OS.

## 5. Risks

| ID | Risk | Response |
|---|---|---|
| R1 | A CLI introduces execution outside its guarded process tree | Disable such delegation and exercise installed command execution before accepting the new launch shape |
| R2 | Repository checks generate files alongside source | Use private output/cache paths; report a check that requires source writes instead of relaxing the guard |
| R3 | Source write protection is mistaken for full host isolation | Preserve §3.3's limit and require a separate approved isolation design when that guarantee is needed |

## 6. Acceptance

| ID | Required observation |
|---|---|
| A-1 | Every registered harness launch retains review purpose and the shared access policy while permitting ordinary tools |
| A-2 | Real OS tests allow source/Git reads and private scratch writes, and deny source overwrite, creation, deletion, rename, permission changes, descendant writes, and Git writes |
| A-3 | The installed Codex command executor reads exact fixture source and revision without inference or a Pyxis command broker; its mutation attempts fail |
| A-4 | The registered Codex review adapter produces a grounded finding from a disposable fixture; host checks prove source bytes and HEAD unchanged |
| A-5 | The end-to-end implement-now path can complete review after a denied source write; confinement failures remain terminal before model launch |

## 7. Open questions

None for the approved source write protection scope.

## Principles

- **AP-17 — honored:** unusable write enforcement refuses launch before inference.
- **AP-20 — honored for filesystem writes:** private runtime state is the reviewer's sole writable filesystem scope, apart from process IO devices. Host read and network isolation require a separate decision.
- **AP-25 — honored:** the shared runtime owns write protection; each adapter maps its CLI's native controls to that boundary.
- **RW-5 — honored:** this successor decides reviewer source protection and incorporates the other contracts without changing their lifecycle or publication predicates.
