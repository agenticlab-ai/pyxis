# Withdrawal of unlanded source captures — technical design

| Field | Value |
|---|---|
| **Status** | Approved only upon authenticated approval and merge of the introducing exact PR head |
| **Owner** | mdarabi |
| **Approver(s)** | mdarabi; approval of the exact introducing PR head |
| **Date** | 2026-10-06 |
| **Relevant-when** | recovering unpublished literal source captures after their source changes; changing proposal withdrawal or its retry behavior |
| **Scope** | Withdrawal of an explicit closed batch of unlanded source Documents and their exact source-only native Decision companions. No changes to accepted records, evidence verification, baseline allowances, or Git history. |
| **Tracking** | Introducing pull request |
| **Builds-on** | memory-durable-evidence-v1 |
| **Supersedes** | — |
| **Serves** | design:memory-durable-evidence-v1#DINV-2 · design:memory-durable-evidence-v1#DINV-5 · design:memory-durable-evidence-v1#DINV-6 |
| **Security/privacy impact** | yes — removal of proposal files and local archival of their exact content; reviewer: mdarabi |
| **Template version** | tech-design-template/v1 |

## 0. Read to rule

Skipped: the decision and executable acceptance criteria fit in the body below.

## 1. Problem

A driver can capture an evolving journal more than once before publishing it.
Each intermediate capture truthfully witnesses the source at capture time, but
its record-introduction witness cannot match the final landing tree. The
durable evidence validator correctly refuses every such new record, including
superseded records. Rebinding an intermediate capture to different bytes would
misstate its evidence. Requiring a history-preserving merge would violate the
supported squash workflow.

The missing operation is withdrawal of those unaccepted capture proposals,
followed by a fresh capture of the final source. This is a proposal lifecycle
operation, not an exception to the evidence predicate.

## 2. Decision

**Decision:** A native memory writer may withdraw an explicitly selected,
closed batch of committed, unlanded, literal source Documents and their exact
source-only native Decision companions. It first archives their exact
envelopes and any literal bodies with a pinned intent, then removes
only those working files. It never changes the index, refs, commits, accepted
identities, or remaining records. Final source capture uses a new native ID.

## 3. The design

### 3.1 Admission and pins

`memory discard-proposal --record ID … --reason TEXT --check|--write [--json]`
requires a nonempty reason and exactly one mode. IDs are deduplicated and
sorted. The configured mainline and current HEAD are pinned using the existing
full-history evidence session; callers cannot supply a substitute base.

**Decision:** Admission reads the committed candidate memory tree. Every
selected record must be a literal Document tagged `reconcile:source` with a
source path, or an exact source-only native Decision companion of a selected
design Document. A companion has only the native generated tags and fields,
the exact extracted statement, family, block hash and provenance, and one
`depends_on` edge to that selected Document. Validation uses the captured
body, not the currently edited source. Added semantic fields, code associations
and other relations refuse. Every selected identity must be absent from the
retained first-parent mainline history. A surviving relation cannot target a
selected ID. Authored semantic claims, feature locks, frozen records, incomplete history, uncommitted records,
invalid committed memory, and partial relation chains refuse.

The memory index must match HEAD. All unselected memory working files must
match HEAD. Each present selected file must be regular, with the committed
bytes and executable mode. Selected files already absent are admitted only
when a complete, byte-exact archive and matching intent prove a previous
attempt on the same pins and selection. Preview performs no writes.

### 3.2 Archive, mutation, and retry

The archive is under ignored `.pyxis/logs/memory/proposal-discard/`, addressed
by candidate commit and a digest of the complete sorted selection. Every
destination must actually be Git-ignored and untracked. Directory symlinks,
nonregular files, conflicting archive bytes, and redirected paths refuse.
Existing matching copies are reused; new copies are created exclusively and
synced. The versioned intent identifies the base, candidate, reason, IDs,
file paths, modes, digests, and archive location. It is written only after all
exact copies are retained.

**Decision:** Writes share the native memory writer lock. Pins and file state
are rechecked after archival and around removal. An observed failure restores
files removed by this attempt using exclusive creation; it never overwrites a
concurrent replacement. A restoration conflict reports the affected path and
retains the archive. Process interruption may leave a partial withdrawal;
rerunning the identical command on unchanged pins safely finishes it. Success
requires every selected file absent and the pins and unselected memory still
unchanged.

The command stages or commits nothing. Its caller stages the reported paths
explicitly, records the withdrawal reason in the existing delivery or review
artifact, and uses the normal required checks and approval workflow. The
archive is local recovery data; it is not evidence for accepted records and
is never published as semantic memory or included in a release payload.

### 3.3 Invariants

- **DINV-1** — No ID previously present on configured mainline is withdrawn.
- **DINV-2** — Withdrawal never alters a record's bytes, reuses its ID, edits
  source bytes, or weakens the validator or allowance inventory.
- **DINV-3** — Before the first removal, all selected committed bytes and a
  deterministic pinned intent exist in the ignored archive. Original Git
  objects, refs, and index bytes are unchanged by the operation.
- **DINV-4** — No surviving relation acquires a missing target. Dirty or
  conflicting bytes refuse rather than being overwritten or deleted.
- **DINV-5** — Retry on identical pins, selection, and reason converges;
  detected failures restore this attempt's removals or report restoration
  conflicts explicitly. No successful result follows a changed pin.
- **DINV-6** — Accepted replacement evidence verifies in a fresh full clone
  after squash without topic commits or the local archive.

### 3.4 Observability

The `memory-proposal-discard/v1` result contains the pinned request and an
`applied` flag. Preview exposes exact paths and hashes. Errors identify the
refused condition or restoration path; the archive's `intent.json` represents
prepared recovery data, never a claim of completed withdrawal. There are no
background metrics or alerts; the caller's owning workflow records the shared
decision and the ordinary memory gate records the acceptance verdict.

### 3.5 Security & privacy

This operation removes proposal files and copies their committed content
locally. Directory handles confine filesystem access and reject symlink
components; exclusive writes prevent archive or restoration overwrite. New
archive files are private to the local user. Git reads use the existing
hardened local adapter and execute no hooks, filters, source text, network
fetch, or model call. The selected subject does not grant authority over
mainline or permit feature-lock disposal. Arbitrary noncooperating filesystem
writers cannot be made transactional; detected races fail closed with exact
recovery copies rather than a fabricated completion verdict.

## 4. Alternatives considered

| Alternative | Why rejected |
|---|---|
| Rebind intermediate envelopes to final source | Changes the evidence behind an immutable assertion. |
| Exempt superseded records from validation | Leaves new unsupported claims accepted and weakens the shared predicate. |
| Require a normal merge | Removes squash support; topic ancestry is not durable record-introduction evidence. |
| Hand-delete generated records | Lacks a bounded writer, archive, closure check, or retry contract. |
| Retain every intermediate source as accepted snapshots | Introduces a separate source-retention mechanism when the proposals were never accepted. |

## 5. Risks

| ID | Risk | Impact | Mitigation | Verification |
|---|---|---|---|---|
| R1 | Accidental withdrawal of an accepted identity | Corrupts durable memory | Complete mainline identity inventory | Frozen and historically removed ID fixtures |
| R2 | Interrupted removal or competing edits | Partial proposal tree or lost bytes | Complete archive, exact retry, exclusive restoration | Failure and race injection with byte comparisons |
| R3 | Archive mistaken for accepted proof | Hidden dependency on local state | Unchanged validator and fresh transport-clone proof | Squash and topic deletion test |

## 6. Acceptance

| ID | Acceptance criterion | Verify | Required evidence | Traces to |
|---|---|---|---|---|
| A-1 | A stale capture chain, including exact source-only native Decision companions, refuses evidence before withdrawal; preview writes nothing; withdrawal preserves exact archive bytes, Git objects, refs, and index. | Test | Real Git command fixture | DINV-2; DINV-3 |
| A-2 | Frozen and historically introduced IDs, authored semantic records, malformed or unselected-source companions, code-associated companions, dirty bytes or modes, unsafe archive paths, uncommitted selections, and open reference chains refuse without removal. | Test | Adversarial fixtures | DINV-1; DINV-4 |
| A-3 | Archive and removal failures, cancellation, and changed pins cannot report success; retry after partial removal converges without overwriting a concurrent replacement. | Test | Injected failures and pin races | DINV-3; DINV-5 |
| A-4 | A fresh final capture passes the ordinary validator before and after squash and topic deletion in a transport clone with the archive absent; reusing or withdrawing accepted identities remains refused. | Test | Fresh clone with topic object absence | DINV-1; DINV-6 |

## 7. Open questions

None. Approval of this introducing contract and implementation remains the
normal exact-head review and approval event.

## Principles

- **AP-6 / AP-7** — A versioned pinned request preserves exact data; accepted
  memory remains immutable.
- **AP-11 / AP-13 / AP-14** — The existing evidence predicate still decides
  acceptance, including fresh-clone verification after squash.
- **AP-15 / AP-20 / AP-22 / AP-23** — Explicit selection, a shared writer
  lock, repeated pin checks, and exact recovery bound concurrent mutations.
- **AP-24** — A candidate cannot choose its own base or grant an exemption.
- **AP-28** — Original candidate history remains unchanged. The caller's
  existing shared workflow retains the withdrawal decision; local archive
  data has no authority to establish accepted evidence.
- **RW-5** — This document decides the single proposal-withdrawal lifecycle.
