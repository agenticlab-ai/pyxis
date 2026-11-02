# Argus journal type recovery — append a correction without rewriting history

| Field | Value |
|---|---|
| **Status** | Pending review; Approved only on exact-head reviewed merge |
| **Owner** | @mdarabi |
| **Approver(s)** | @mdarabi (operator), pending exact-head review |
| **Date** | 2026-10-08 |
| **Relevant-when** | validating new delivery events or recovering a committed delivery ledger with an unsupported event heading |
| **Scope** | Executable append-only type corrections and early validation. Carry forward bounded Argus draft, plan and capture attribution, stamp refresh, protected classes and shared-account assurance. |
| **Tracking** | AEC company-memory epic #20, comment 6069919910; merged delivery E-110/E-111 |
| **Builds-on** | argus-artifacts §3.6 |
| **Supersedes** | argus-committed-plan-attribution |
| **Serves** | prd:argus-boundary-enforcement/FR-9 · prd:argus-boundary-enforcement/FR-10 · prd:argus-boundary-enforcement/FR-13 · prd:argus-boundary-enforcement/FR-14 · prd:argus-boundary-enforcement/NR-4 · prd:argus-boundary-enforcement/UB-4 · prd:argus-boundary-enforcement/UB-5 · prd:argus-boundary-enforcement/ABUSE-2 · prd:argus-boundary-enforcement/ABUSE-3 · prd:argus-boundary-enforcement/ABUSE-4 · prd:argus-boundary-enforcement/NFR-1 · prd:argus-boundary-enforcement/NFR-2 · prd:argus-boundary-enforcement/NFR-3 |
| **Security/privacy impact** | yes — exact historical correction validation and early refusal of unknown new types; committed-input scope and all protected classes remain unchanged |
| **Template version** | tech-design-template/v1 |

This proposal asserts no prior operator approval. Its decisions take effect only after reviewed merge. All unaffected predecessor requirements remain binding below.

## 1. Problem

The native company-memory plan merged two events named `finding-disposition`, outside delivery-ledger/v1's closed vocabulary. The introducing native gates passed. The committed-ledger scope resolver subsequently rejects the journal's typed shape, so native reconciliation's required same-problem Document pair passes memory validation but fails attribution. Rewriting either frozen entry would destroy the audit contract. Broadening the event vocabulary to accept an improvised name would conceal the producer error.

Argus artifacts §3.6 already requires a correction to be a new entry citing what it corrects. The current typed reader has no executable interpretation for that correction. Recovery must preserve original bytes and actors, provide an explicit corrected classification, and prevent newly authored unknown event names from reaching main.

## 2. Decision

**Decision:** Keep the ten canonical event types unchanged. A native writer appends typed `reconciliation` entries with a closed, versioned body gist that identifies the original event, its exact bytes, the erroneous type and its canonical replacement. Readers validate the complete correction chain and compute the effective type without modifying the original entry.

New unknown events fail before merge. A correction can recover only an unsupported type already present in the trusted committed input. Recovery adds no permission inferred from a candidate path list or external-effect claim. The correction first lands as an ordinary same-problem ledger PR. Subsequent source capture and planning use the corrected trusted committed ledger.

## 3. The design

### 3.1 Correction data and writer

**Decision:** The original journal entries remain byte-identical. A correction is a new dense `E-n` of canonical type `reconciliation`, carrying the actual principal, date and authority-contract version, and a reason. Its first labeled body line reads `journal-type-correction/v1; target=delivery:<slug>#E-n; sha256=<original entry digest>; from=<original unsupported token>; to=<canonical token>`, with exactly those four keys. The remainder explains the error and intended classification. The digest hashes the original entry block from its heading through its last content line, including original line endings and trailing spaces, excluding inter-entry blank lines and horizontal-rule separators. This exact-byte view is distinct from the existing normalized lint view.

The target must be a prior event in the same ledger, its digest and original token must match, and `to` must belong to the unchanged closed vocabulary. Corrections of already canonical types, forward references, cross-problem references, duplicate/conflicting corrections, unknown keys and malformed gists refuse. Every original date, principal, authority version, body and external-effect identity remains subject to the existing shape rules; the corrected type's specific rules apply to the original event. A correction cannot turn an intermediate event into a terminal delivery or bypass required brief/answer fields.

The native writer accepts a batch so both historical errors can be corrected atomically. `pyxis exec docgate -- correct-journal <slug> --correction E-n=<canonical-type> --date YYYY-MM-DD --principal <actual-actor> --authority-version <version> --reason <explanation>` appends the correction; repeat `--correction` for each target. `--dry-run` returns the exact proposed bytes without writing. It reads the actual journal, computes target digests, validates the complete result and appends only the new entries in one atomic replacement preserving file permissions. It refuses missing final newlines, nonregular files, symlink traversal, escaping configured roots and changes detected before replacement. It changes no Git ref, index, original entry, authority file or grant. Caller-supplied attribution must identify the actual author; the writer is not human attestation. The derived state remains owned by the drive.

### 3.2 Early validation

**Decision:** The normal document gate's registered `check-delivery-types` check validates changed delivery ledgers against their trusted base. A newly introduced unknown event refuses even if a later same-proposal correction tries to cover it. A historical unsupported event is tolerated only when a valid appended correction resolves it. Unresolved historical errors receive a specific recovery diagnostic rather than an unexplained capture-path refusal. Unchanged legacy ledgers do not block unrelated application changes. Pushed heads are read from Git rather than another session's working tree. The Source legacy-document exemption retains this Argus integrity check, as it retains attribution and history.

The native correction writer validates a full batch before writing. The write hook and attribution gate continue to use the same committed-ledger parser and effective typed shape. Gate checks still verify the complete carried bindings, append-only history and protected-path classification. No producer may silently substitute a canonical heading for the original bytes.

### 3.3 Sequential recovery through the existing scope

**Decision:** For a fully valid committed delivery ledger, retain the predecessor's exact historical paths, own problem subtree, three-digit numbered plan Markdown/JSON, and flat native Document JSON/body grammar. With malformed committed input, no numbered plan scope is added.

The existing resolver retains the active problem's own subtree even when its historical typed shape is invalid. The drive uses that scope to append the corrections and submit a ledger-only PR with ordinary independent review, exact-head approval and protected merge. The candidate supplies no additional capture or plan scope. Original entries retain their order and exact bytes, every historical unsupported type is resolved, all new entries use canonical types, and the complete corrected shape must pass. Any unresolved event, edited entry, malformed correction or missing historical field refuses.

After the ledger PR lands, native reconciliation captures its current literal source in a subsequent PR. Both the write hook and attribution gate then derive normal bounded plan and capture scope from the corrected trusted committed ledger. The preceding malformed ledger grants neither grammar. Native record admission separately verifies source content and provenance. Boundary data version advances to `argus-boundary/v6` for the effective-type interpretation; the path grammars do not change.

### 3.4 Unaffected bindings carried forward

**Decision:** First-proposal scope remains a fixed function of configuration and the active slug, including its draft, compiled PRD/design and native problem-record subtrees, and the predecessor's fixed shared allowlist. It admits no application code, another slug, authority or governance.

**Decision:** Stamped draft rebases remain exclusively `draft prepare-lock` operations: verify original bindings, refresh existing complete stamps for the new tree/parent, preserve unstamped source commits, refuse partial/stale stamps, keep the generated lock commit unstamped and share the pinned cache in the temporary installed worktree. The normal per-commit drive context and single-parent binding remain unchanged.

**Decision:** Positive Argus precedence, protected-path refusal, positive human attestation and underivable fail-closed behavior remain in force. No correction creates an operator attestation, action-class grant, lock approval, accepted plan or application acceptance. Planning still requires the actual binding lock and native precheck; first-proposal scope still excludes numbered plans. Configured roots relocate the same fixed grammars.

**Decision:** Shared-account Git stamps remain audit evidence, not adversarial isolation from a drive able to invoke Git. Exact-head approval under trusted-mainline policy, memory/code/document predicates and host protection govern merge. Pyxis Source continues its documented legacy-check exemption while retaining attribution; installed consumers retain the full document gate.

### 3.5 Invariants

- **DINV-1** — Original event bytes, numbering, actor, authority version and effects remain permanent.
- **DINV-2** — Canonical event vocabulary stays closed; no new unknown event can be introduced through recovery.
- **DINV-3** — A correction binds one earlier same-ledger unsupported event by exact digest and validates its effective type's rules.
- **DINV-4** — A candidate correction confers no new scope; normal bounded plan and native Document grammars resume only from a valid corrected committed ledger.
- **DINV-5** — Both scope consumers use the same predicate; no candidate external-effect claim widens ownership.
- **DINV-6** — Predecessor stamp/rebase/lock, grant, readiness, protection and assurance bindings remain unchanged.

### 3.6 Observability

The correction entries retain the original target, exact digest, original and effective types, actual principal, date, authority version and reason. The native writer reports its batch result; `check-delivery-types` reports named defects with the offending event and correction remedy. Neither creates a mandatory session log or duplicates semantic knowledge. Existing runtime diagnostics remain ignored local evidence.

### 3.7 Security & privacy

Correction data is closed and versioned. Candidate corrections confer no scope, attestation or grant. Exact raw entry bytes and the actual trusted Git base prevent normalization or same-proposal laundering. Original metadata and effective type-specific obligations remain mandatory. The file writer confines replacement to the regular configured repository ledger and validates the entire batch before replacement. There are no credentials, new services or telemetry.

## 4. Alternatives considered

| ID | Alternative | Reason declined |
|---|---|---|
| ALT-1 | Rewrite the two merged headings | Violates permanent entry history and invalidates retained source/binding evidence. |
| ALT-2 | Add `finding-disposition` as an alias or new event type | Hides the producer defect and changes the closed vocabulary rather than correcting the record. |
| ALT-3 | Admit captures for every malformed ledger | Weakens the valid-journal precondition without an exact correction proof. |
| ALT-4 | Derive scope from candidate effects or admit all memory | Lets an author grant arbitrary paths; recovery needs only the existing same-problem capture grammar. |
| ALT-5 | Require an operator to capture every later journal snapshot | Transfers normal component work to the operator and does not restore enabled-human parity. |

## 5. Risks

| ID | Risk | Mitigation | Required evidence |
|---|---|---|---|
| R1 | A repair launders new unknown events or edits historical truth | Compare the retained prefix and trusted base; reject fresh unknown types and bind target digest | Adversarial pure and Git-backed gate tests |
| R2 | Candidate recovery expands unrelated scope | No candidate-based scope; same trusted-base resolver and protected classes | Both consumers refuse captures before corrected merge and retain all unrelated path denials |
| R3 | A corrected type loses subtype obligations or closes early | Apply existing type-specific rules to the original entry | Brief/answer/delivery negative tests |
| R4 | Recovery produces partial appends | Validate a full batch before one atomic file replacement | No-write-on-error and real multi-target writer tests |

## 6. Acceptance

| ID | Acceptance criterion | Verify | Required evidence | Traces to |
|---|---|---|---|---|
| A-1 | An exact append-only batch corrects both historical types, retains all original bytes and restores effective typed validity | Test | Pure resolver and writer results; exact before/after prefix comparison | §3.1 · #DINV-1 · #DINV-3 |
| A-2 | Newly authored unknown names fail before merge, including same-proposal attempted repair | Test | Real Git attribution refusal | §3.2 · #DINV-2 |
| A-3 | Before the correction lands, capture scope stays denied; a valid corrected committed ledger restores only normal same-problem scope through both consumers; malformed or incomplete repairs refuse | Test + failure injection | Failing-before/fixed-after pure, write-hook and real Git attribution fixtures | §3.3 · #DINV-4 · #DINV-5 |
| A-4 | Wrong digest/from/to, cross-ledger/forward/duplicate targets, changed original fields and invalid subtype obligations refuse without writer changes | Test | Closed-data and atomicity negatives | §3.1 · #DINV-3 |
| A-5 | All predecessor first-PR, valid committed plan/capture, protected path, rebase stamp and generated lock behavior continues passing | Test | Existing conformance suites | §3.4 · #DINV-6 |

## 7. Open questions

None for this bounded type-only repair. Other historical field errors, post-close overturns and broader artifact migration remain governed by their own contracts.

## Principles

- **AP-6 — honored:** A versioned closed correction gist, canonical vocabulary and pinned boundary policy govern interpretation.
- **AP-7 — honored:** Corrections append; original events and committed history are never rewritten.
- **AP-13 — honored:** Exact retained bytes and trusted-base objects govern recovery and review.
- **AP-17 — honored:** Unresolved history, malformed data and unsupported scope fail closed.
- **AP-25 — honored:** Native component interfaces and one shared predicate serve the supported harness surfaces.
- **AP-26 — honored:** Actual Git content supplies the base and target digests; no badge or author scope claim supplies permission.
- **Deviation:** None beyond the predecessor's retained shared-account assurance limitation.
