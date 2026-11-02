# Independent memory debt allowances — technical design

| Field | Value |
|---|---|
| **Status** | Approved — operator sign-off 2026-09-17; merge is the freezing event |
| **Owner** | @mdarabi |
| **Approver(s)** | @mdarabi (operator; memory integrity and merge-control surface) |
| **Date** | 2026-09-16 |
| **Relevant-when** | changing memory reconciliation baselines, debt reduction, baseline migration, or the shared local and hosted memory gate; diagnosing conflicts between unrelated memory changes |
| **Scope** | The representation and validation of accepted reconciliation debt, its migration, and its write footprint. Record chronology, provenance retention, code-anchor granularity, and general document indexes are separate decisions. |
| **Tracking** | Operator request of 2026-09-16 to develop the parallel-PR memory proposal; no issue or PR created |
| **Builds-on** | — |
| **Supersedes** | — |
| **Serves** | prd:parallel-memory/FR-1 · prd:parallel-memory/FR-2 · prd:parallel-memory/FR-3 · prd:parallel-memory/FR-4 · prd:parallel-memory/FR-5 · prd:parallel-memory/FR-8 · prd:parallel-memory/FR-9 · prd:parallel-memory/NR-1 · prd:parallel-memory/NR-3 · prd:parallel-memory/UB-1 · prd:parallel-memory/UB-2 · prd:parallel-memory/UB-3 · prd:parallel-memory/UB-6 · prd:parallel-memory/NFR-1 · prd:parallel-memory/NFR-2 · prd:parallel-memory/NFR-3 |
| **Security/privacy impact** | yes — changes the accepted-debt representation used by merge controls; preserves trusted-base authority and offline evidence checks · reviewer: @mdarabi |
| **Template version** | tech-design-template/v1 |

## 0. Read to rule — the human reviewer's section (~5 min)

*The body is the contract; this section only routes attention — on any conflict
the body governs; machines ignore it.*

**Judge these**

| # | The call | Could have gone | If you disagree |
|---|---|---|---|
| 1 | Store one accepted finding address per file; ordinary PRs never rewrite a global inventory. | Keep the list together and remove only hashes, ALT-2. | Retain contention between unrelated debt reductions → §2, §3.1. |
| 2 | Recompute exact input identities during each check; keep debt policy separately reviewable. | Continue committing whole-repository hashes, ALT-1. | Keep refresh work on every source or memory change → §3.1. |
| 3 | Preserve exact inventory matching and a monotonic comparison with the trusted base. | Use a permanent allowance ceiling alone, ALT-3. | Allow resolved debt to return or require another retirement mechanism → §3.6. |
| 4 | Migrate explicitly, once, with an exact v1-to-v2 allowance comparison. | Silently rebootstrap missing or old policy, ALT-5. | Weaken the existing authority boundary → §3.7. |

**Sanity-check the numbers** — zero shared baseline edits for disjoint finding addresses; zero new allowances during migration; zero network/model calls; ten sibling branches exercised in both orders for each pair → DINV-1, DINV-4, DINV-6, A-1.

**The author's open questions** — representation and migration approved on 2026-09-17 → Q1; no implementation-blocking question remains.

**Skip the rest** — structure and pairing → docgate; substance → tech-design-review; implementation proof → §6; alternatives → §4.

## 1. Problem

The current baseline mixes accepted debt with four identities of the entire
repository's reconciliation inputs. Adding a record in an unrelated subject
changes one of those identities. A production change elsewhere changes
another. Both PRs therefore rewrite the same lines, and neither branch's hash
describes their combined tree.

At inspected commit `8b9c79c246ef025f981155aca93b567fe513b663`, all ten
most recent first-parent PR commits changed the baseline. Those commits added
171 distinct memory files with no repeated memory-record path. This dated
source/history observation establishes overlap, not a measured conflict rate.
The relevant implementation is the baseline model and identity comparison in
`internal/memory/reconcile/baseline.go`, the full-memory digest in
`internal/memory/reconcile/identity.go`, and the check orchestration in
`internal/memory/cli/reconcile.go`.

Removing only the hashes would solve ordinary refresh churn while leaving
unrelated debt reductions sharing an allowance list. Both are sets whose
members can be addressed independently. The vision's parallel-authorship
requirement and AP-27 supply the governing direction; this document proposes
the mechanism. No existing memory-specific technical design is present in
the inspected design index, so this is a new design rather than a supersession
of an implementation README.

## 2. Decision

**Decision:** Represent accepted reconciliation debt as a set of separately
addressed policy files, one per stable finding address. Derive the aggregate
inventory, counts, and input identities during validation. A normal change
with unchanged debt writes no baseline file; reducing debt changes only the
allowance files for the addresses it actually improves.

**Decision:** Preserve the existing exactness and monotonicity requirements.
The candidate's allowances must exactly match its recomputed findings and
must not widen the allowances already trusted on its base. The existing
explicit, one-way finding-phase advances remain the only address-change
exceptions; their predecessor must disappear at the same scope and feature,
and multiplicity cannot grow. Splitting storage
does not authorize new debt, relaxed evidence checks, or source deletion.

The concrete layout, closed schemas, CLI extension, and existing package seams
are in the paired [solution shape](../solution-shapes/memory-debt-partitions-v1.md).

## 3. The design

### 3.1 Policy members and derived evidence

A finding address consists of the existing code, scope, and feature identity.
An allowance retains the existing count, severity, and repairability semantics.
The filename derives deterministically from the address; mutable allowance
values do not participate in that address. Different addresses never share a
policy file. Duplicate addresses or inconsistent filenames are invalid.

The collection has a stable version marker but no authored list of members,
aggregate count, or current-tree hash. Discovery reads its fixed directory
from the pinned Git tree. The marker changes only for an explicit format or
evaluator transition. A missing marker is an error after adoption; an empty
collection with a valid marker means zero accepted debt.

The validator still computes exact source, code, design, and memory identities.
They identify the actual inputs in its result, including the pinned Git
commits. They are evidence about that invocation, not policy every future
author must refresh. Existing record-level source hashes, code-unit witnesses,
and closeout checks retain their force.

### 3.2 Boundaries and non-goals

- The zero-overlap claim concerns baseline maintenance for different finding
  addresses. It does not declare competing edits to one allowance independent.
- Independent source work here means disjoint authored-source units, code
  units, and exclusive claim families. A Go package remains one code unit;
  changing separate files in that package may still require anchor coordination.
- Record chronology is governed by the companion proposal
  `memory-causal-admission-v1`, which can be approved and implemented separately.
- Durable provenance after squash, automatic merge resolution, general design
  indexes, branch protection, and merge scheduling are outside this change.
  Source deletion and feature closeout retain their existing authority rules.
- Implementation uses the existing memory CLI, reconciliation, store, and Git
  adapter. It introduces no service, database, custom Git merge driver, or new
  adapter interface.

### 3.3 Invariants

- **DINV-1** — Changing source or memory without changing findings modifies zero baseline policy files. Reducing findings at disjoint addresses produces disjoint baseline paths.
- **DINV-2** — Candidate allowances exactly equal the recomputed finding addresses, multiplicities, severities, and repairability; comparison retains the existing no-widening rules and only their explicitly recognized one-way phase advances.
- **DINV-3** — Checks read one pinned candidate and one pinned trusted base under explicit evaluator versions. Derived identities name all actual inputs; malformed or unavailable inputs cannot produce success.
- **DINV-4** — Migration preserves the trusted allowance inventory exactly and proves the legacy input binding before switching representations. It cannot add debt, hide deletion, or restart the lineage.
- **DINV-5** — Code-transition, source-retirement, record immutability, record-evidence, and strict feature-closeout checks remain independent of allowance storage and retain their existing refusal conditions.
- **DINV-6** — Parsing, discovery, normalization, comparison, and synchronization are deterministic Go operations with no model or network dependency. Re-running a completed synchronization changes zero files.
- **DINV-7** — A validator never writes policy. An explicit writer validates the complete proposed change and protects unrelated working-tree files before its first mutation; a partial write cannot pass the gate.
- **DINV-8** — Local, pull-request, push, and scheduled checks use the same policy parser and comparator. A clean pass creates no new tracked report or shared current-state receipt.

### 3.4 Observability

The existing human and JSON reconciliation outputs gain the policy format,
pinned candidate/base, evaluator identities, allowance-member identities,
derived input hashes, and exact changed addresses. Findings distinguish
malformed policy, unsupported migration, inventory mismatch, widened debt,
source/code transition failure, and insufficient history. Ordering is stable.

Counts of policy files changed and allowance deltas come from the result;
there is no new mutable metrics file. Gate failures remain the alert to the
proposal author. Local diagnostics stay in the existing ignored log surface.
When a failure or judgment supplies durable improvement evidence, retain it
in its owning review, incident, or work item as a distinct event, per AP-28;
routine successful projections need no tracked artifact. Correlation uses
the candidate/base identities and the existing gate execution identity.

### 3.5 Security & privacy

This is a merge-control change. Trusted bases retain the event/configured
authority supplied by the existing gate; candidate files cannot redirect the
policy root or select a weaker base. Directory members are regular Git blobs,
parsed closed, with exact filenames, no duplicate JSON keys, no traversal,
symlinks, submodules, or unknown entries. A hash-derived filename is an
address, not authorization; the decoded address and trusted comparison decide.

Missing policy never falls back to an empty allowance set or a new bootstrap.
The writer refuses increases as well as policy-root redirection, and never
derives a new grant from an existing defect. Diagnostic output includes paths
and digests, not document bodies or credentials. Checks perform no network
fetch, model call, hook, or execution of repository content. These protections
implement DINV-2 through DINV-7.

### 3.6 Validation and synchronization

**Decision:** Keep the existing trusted-base/committed-candidate contract.
This proposal changes what must be committed, not which unmerged tree is
authorized. A branch behind the required base still integrates it before the
gate; the policy representation must make that integration mechanically clean
for disjoint work.

On a proposal or push, pin both commits, load policy and configuration from
their own trees, and recompute candidate findings. Load the trusted set,
verify its reproduction under a recognized evaluator, and apply the existing
monotonic comparison and source/code transition predicates. Candidate
allowances must exactly equal candidate findings. A scheduled inventory pass
has no invented predecessor: it verifies the exact checked-in inventory and
current inputs, just as the existing inventory mode does.

Resolving a finding removes or tightens its member in the same change. If
that finding returns after the reduction lands, absence or tightening at the
trusted base prevents a candidate from granting it again. An address may not
move to a new filename to reset its history; identity comes from decoded
content, not the pathname alone.

An explicit synchronization previews all changes, refuses regression, and
writes only required member changes under the existing repository write lock.
It checks the pinned HEAD and baseline working files before writing. All
nonpolicy work must already be represented by the pinned HEAD. An exact
rerun is a no-op. Interrupted output is a refused incomplete policy; rerunning
the same synchronization repairs only its planned paths. The existing checker
does not become an automatic author of allowances.

### 3.7 Compatibility, migration, and rollback

**Decision:** Adopt the representation through a dedicated, explicit migration
with dual-format reading available first. Existing v1 repositories retain
their current behavior until that migration. There is no permissive fallback
from a failed v2 parse to v1 or to bootstrap.

The migration requires a clean, pinned mainline-derived snapshot whose v1
baseline is valid, plus the existing trusted-base proof. It reproduces legacy
input identities using the existing bounded replay where needed, compares
the entire allowance set before and after, and changes only the policy
representation. Code needed for the migration lands before the data-only
cutover. The original legacy baseline remains recoverable in retained Git
history; the v2 reader does not depend on a topic-only commit.

Storage format and evaluator version are separate. Changing the policy layout
alone does not change source classification or the evaluator identifier.

Known evaluator changes use explicit, tested compatibility mappings; unknown
versions refuse with a named remedy. They cannot silently classify away old
sources or code. An in-flight v1 branch integrates the migration, keeps its
records, and synchronizes only its actual debt reductions; stale global hash
edits disappear rather than becoming v2 policy input.

Deploy a reader that supports both formats before cutover. Rollback of
unrelated implementation changes keeps that reader and the v2 policy. A full
return to v1 is a separately reviewed reverse migration from current retained
inputs, not a deletion of v2 files or automatic rebootstrap. Older binaries
must refuse the unknown format.

### 3.8 Verification and implementation sequence

Pure parsing/addressing/comparison uses in-process fixtures. Git trees,
rebases, merges, squash integration, and fresh transport clones use real
temporary repositories through the existing CLI E2E harness. Git is a local
substitutable dependency; no remote service or new mock port is needed.

After approval: extend the baseline reader/comparator and its structured
output; add the explicit writer and migration preflight; integrate the shared
gate; then land the data-only cutover. Tests assert write footprints and
refusals, not timing. The companion chronology change is needed for the full
parallel-memory acceptance scenario but not for the baseline-only fixtures.
Required Go checks, shared code gate, and hosted checks apply when code is
proposed; this draft does not claim those results.

## 4. Alternatives considered

| ID | Alternative | Verdict and tradeoff |
|---|---|---|
| ALT-1 | Keep the whole-tree baseline and automate regeneration during rebase | Rejected: fewer manual commands, but every sibling still edits the same policy lines and repeats validation. |
| ALT-2 | Remove input hashes but retain one allowance list | Rejected: smallest implementation, but unrelated debt reductions still share the central list; fails the declared disjoint-address guarantee. |
| ALT-3 | Keep an immutable allowance ceiling and compare only live base/candidate findings | Rejected for this increment: avoids policy writes, but permanent retirement, scheduled auditing, and evaluator migration need a new historical-floor mechanism. Exact per-address policy already provides those properties. |
| ALT-4 | One file per finding address, with aggregates derived on read | Proposed: more small files during one migration, in return for bounded per-address writes and reuse of exact ratchet semantics. |
| ALT-5 | Delete the baseline and accept or regenerate all current findings | Rejected: turns observed defects into new authority and loses the existing no-widening guarantee. |
| ALT-6 | Serialize PRs or use a merge driver to pick a hash | Rejected: serialization spends parallelism; either branch's hash is generally wrong for the combined tree. |

## 5. Risks

| ID | Risk | Impact | Mitigation | Verification / evidence |
|---|---|---|---|---|
| R1 | Migration changes the accepted set | Hidden debt or false refusal | Exact normalized comparison and data-only cutover; owner: implementer | A-3 and A-4 negative fixtures |
| R2 | Parser skips an unexpected file | Missing debt policy silently accepted | Closed directory grammar; owner: implementer | A-5 malformed-tree fixtures |
| R3 | Evaluator change reclassifies old debt | Apparent improvement hides regression | Explicit version compatibility and both-tree transition checks; owner: memory maintainer | A-6 version-transition fixtures |
| R4 | Many small members raise IO cost | Slower validation | Batch Git reads and derive the set once; owner: implementer | A-8 operation-count evidence; duration advisory only |
| R5 | The promised concurrency boundary is misunderstood | Same-package conflicts remain surprising | State unit/family/address boundaries in output and documentation; owner: design author | A-1 contrasting same-address case; companion integration test |
| R6 | A failed multi-file writer leaves partial output | A later action uses incomplete policy | Full preflight, narrow owned paths, rerunnable synchronization, gate refusal; owner: implementer | A-7 interruption and retry fixtures |

## 6. Acceptance

| ID | Acceptance criterion | Verify | Required evidence | Traces to |
|---|---|---|---|---|
| A-1 | Ten sibling branches change disjoint source/code/claim units; every unordered pair merges in both orders without baseline edits or conflicts. Separate branches reducing disjoint debt addresses change disjoint policy files; competing edits to one address remain visible. | Test | Real-Git fixture results and exact per-branch changed-path sets; combined test also cites the companion chronology acceptance | DINV-1 · vision §12 |
| A-2 | No unrecognized address, count increase, severity increase, or newly non-writable allowance is admitted; a resolved finding cannot return under its deleted allowance. Existing one-way phase advances remain accepted only at the same scope/feature without growing multiplicity or retaining the predecessor. | Test | Positive reductions/phase advances and negative reversal, coexistence, reintroduction, and widening fixtures using the existing comparator semantics | DINV-2 · vision §13 |
| A-3 | Migration reproduces trusted v1 inputs and emits the identical normalized allowance inventory; a missing replay or one changed allowance refuses. | Test | Before/after canonical inventory comparison and refusal output | DINV-4 · vision §13 |
| A-4 | Source deletion without prior closeout, changed code without new semantic memory, stale anchors, and modified/deleted records still fail after migration. | Test | Existing boundary fixtures executed through the v2 shared gate | DINV-5 · vision §13 |
| A-5 | Unknown keys/versions, duplicate addresses, incorrect filenames, unexpected members, irregular Git entries, and missing markers refuse; a valid marker with zero members passes only for zero findings. | Test | Closed-format parser and CLI fixture report | DINV-2 · DINV-3 · vision §13 |
| A-6 | Local, PR, push, and scheduled modes produce equivalent classifications for equivalent pinned inputs; recognized evaluator transitions are explicit and unknown ones fail closed. A fresh mainline-only clone validates after squash without topic refs. | Test | Gate-mode matrix and transport-clone evidence | DINV-3 · DINV-8 · vision §13 |
| A-7 | Writer preflight rejects dirty/unowned policy, widening, or moved HEAD before changes; injected interruption cannot certify a partial set; a retry converges and a further retry writes zero files. | Test | Changed-path manifests, refused outputs, and deterministic failure-injection results | DINV-6 · DINV-7 · vision §12 |
| A-8 | Repeated fixed-input checks have identical canonical results, no model/network calls, no tracked success receipt, and one batch policy read per pinned tree rather than one Git subprocess per member. | Test + Inspection | Operation counters and result-byte comparisons; runtime measurements advisory | DINV-3 · DINV-6 · DINV-8 · vision §12 |

## 7. Open questions

| ID | Question | Owner | Blocks | Status | Resolution |
|---|---|---|---|---|---|
| Q1 | Approve the proposed per-address policy representation and explicit migration? | @mdarabi | nothing | resolved 2026-09-17 | Operator approved ALT-4 and continuation: “the design is approved. move on”. |

## Principles

- **AP-5 — honored.** Typed findings and pinned input identities make refusal and successful comparison inspectable.
- **AP-6 — honored.** Policy and result data are closed, versioned contracts with explicit migration.
- **AP-10 / AP-11 — honored.** Existing Go parsers, comparators, and the shared gate own the mechanics; no parallel authority is added.
- **AP-13 / AP-17 — honored.** Real-Git acceptance evidence and exact fail-closed comparisons establish the outcome.
- **AP-22 / AP-23 / AP-27 — honored.** Disjoint debt addresses have disjoint files; aggregates are derived and repeat writes converge.
- **AP-24 — honored.** Synchronization can only preserve or tighten trusted allowances; current defects cannot grant themselves acceptance.
- **AP-28 — honored.** Failure/judgment evidence belongs in durable owning artifacts; clean derived reports create no new shared file.
- **AP-29 — honored.** Concurrency, integrity, and operation counts are correctness assertions; elapsed time is advisory.
- **RW-5 — honored.** This document decides baseline representation and validation; record chronology remains independently revisable.
- **AP-30 — bounded deviation.** Retain the existing canonical policy location
  and place its independent members alongside it for this format transition.
  The root identifies one trusted policy lineage; adding candidate-controlled
  relocation during migration would expand the authority decision and require
  a separately verified transfer of that lineage. This change introduces no
  alternate root and preserves the existing refusal of redirected policy.
  Configurable policy placement is deferred until a repository-layout need
  justifies an explicit trusted relocation protocol and its migration tests.
  The deviation is limited to the existing baseline marker and its new member
  directory; it does not authorize hardcoding other operand-owned paths.
