# Deterministic durable memory evidence — technical design

| Field | Value |
|---|---|
| **Status** | Approved — operator approval 2026-09-16 |
| **Owner** | @mdarabi |
| **Approver(s)** | @mdarabi (operator; stored-record and merge-control surface) |
| **Date** | 2026-09-14 |
| **Relevant-when** | capturing memory provenance; validating evidence before or after squash merges; repairing references after rebases; changing the memory writers or shared memory gate |
| **Scope** | Durable repository evidence for memory records: capture, historical resolution, validation, and draft repair. Excludes deciding whether a prose claim is true, changing the merge policy, and rewriting frozen records. |
| **Tracking** | Motivated by [PR #1192](https://github.com/agenticlab-ai/pyxis/pull/1192); this draft proposes the systemic fix. |
| **Builds-on** | — |
| **Supersedes** | — |
| **Serves** | — |
| **Security/privacy impact** | yes — validation of repository evidence feeding merge controls; reads untrusted record paths and Git data; no additional network or model access · reviewer: @mdarabi |
| **Template version** | tech-design-template/v1 |

## 0. Read to rule — the human reviewer's section (~5 min)

*The body is the contract; this section only routes attention — on any conflict
the body governs; machines ignore it.*

**Judge these**

| # | The call | Could have gone | If you disagree |
|---|---|---|---|
| 1 | Preserve squash support; bind evidence to retained source bytes. | Require history-preserving merges, ALT-1. | Reopen the recorded merge-policy ruling → §2. |
| 2 | Resolve new evidence through the mainline snapshot introducing its record. | Copy every source into a separate evidence store, ALT-3. | Choose a different retention mechanism → §3.1–§3.2. |
| 3 | Every new repository evidence reference passes one deterministic gate; inherited legacy references remain explicitly unverified. | Retroactively block all old records, ALT-5. | Choose a separate historical migration requirement → §3.7. |
| 4 | Capture and draft repair use exact bytes; no model or semantic inference participates. | AI-assisted reference repair, ALT-6. | Reopen the recorded determinism ruling → §2 and §3.6. |

**Sanity-check the numbers** — zero model calls and zero network fetches in evidence processing; zero allowance-based exemptions for new invalid evidence; exact byte equality, without fuzzy matching → DINV-1, DINV-3, DINV-5.

**The author's open questions** — the operator approved the mechanism → Q1. Required-check deployment verification remains an activation obligation → A-10.

**Skip the rest** — structural checks → docgate; implementation correctness → the deterministic acceptance fixtures in §6; alternatives and rationale → §4.

## 1. Problem

A record can name a commit that exists in an author's checkout while being
unavailable in a fresh clone after the proposal is squashed. Checking ancestry
against the proposal head does not solve this: an ancestor of that head can
still be absent from the squash result's ancestry.

The motivating experiment inspected PR #1192 at head
`fb8ccbc0851f4530e60bd8b808959f102d7a8c20`, based on
`3085177722469830eaa6fc48dd38607904b5ceb8`. Its three new Learning records
referenced `706089c715cc71d2217a74f215ca846f70fae01c`; its three new code
anchors referenced `adfa9806ec04762bf076cd34f17a06e56ce0610b`. Both commits
were ancestors of the inspected head. A disposable single-parent commit with
the identical final tree was then cloned through Git's transport with only
mainline history, no tags, no alternates, and no shallow boundary. All six
record files and their cited source files survived unchanged; neither cited
commit existed in the clone; the existing shared memory gate returned success.
This is a retained description of that experiment, not a claim that the PR's
future state or GitHub's retained objects are fixed.

The existing code-anchor check compares the current code inventory digest;
ordinary provenance is not a general historical-reachability proof. Closeout
receipts already require mainline source commits. This design extends durable
evidence validation through the existing memory subsystem while preserving
the closeout rule. It is document-system infrastructure, hence `Serves: —`.

## 2. Decision

**Decision:** Preserve squash merges. Represent repository evidence as exact,
typed source bindings whose bytes can be retrieved from retained mainline
history. A binding either names an already-retained mainline commit or the
mainline snapshot introducing the immutable record; it never depends on a
topic-only commit surviving the merge.

**Decision:** Capture, repair, historical resolution, validation, and reporting
are deterministic Go operations. They require no AI, agent CLI, model API,
semantic classifier, probabilistic match, network lookup, or reviewer judgment.
The operator's rulings in this conversation are to keep squash support and
make the solution independent of AI. Human approval of this implementation
contract does not become a per-record runtime dependency.

The content witness is the tuple of source path, file mode, and content hash,
with exact file-set membership for a code unit. Git's retained tree stores the
bytes. The future merge commit need not be guessed or embedded into its own
tree: the immutable record's introduction provides the historical locator.

## 3. The design

### 3.1 Evidence contract and existing seams

**Decision:** Extend the existing record provenance with a closed, versioned
evidence binding. Keep schema validation in `internal/memory/record`, Git
reads in `internal/memory/gitsrc`, source/unit classification in the existing
source and code inventories, and orchestration in `internal/memory/cli`.
No new service, database, model integration, or abstract adapter is required.

The binding has an explicit format version and a locator. File and unit
evidence carries a nonempty sorted file manifest; a commit-identity-only
reference has no file manifest. Each file entry carries its canonical repository-relative
path, regular Git file mode, and SHA-256 of the raw Git blob bytes. A unit
witness also carries the existing inventory's kind, scope, digest domain, and
unit digest; the manifest includes every production member selected by that
versioned inventory. Named test files are additional entries. Unknown fields,
versions, locator values, duplicate paths, malformed hashes, unsupported
entry types, and incomplete unit membership are errors.

| Locator | Stored meaning | Validation rule |
|---|---|---|
| `mainline-commit` | Existing `provenance.commit` names the retained historical snapshot. | It is a full commit ID reachable from the trusted base; verify the manifest against that exact commit. |
| `record-introduction` | The evidence is present in the snapshot that introduces this record to mainline. | `provenance.commit` is absent. Validate candidate evidence before merge; resolve introduction from mainline history afterward (§3.2). |

A commit-only reference is legal when it uses `mainline-commit` and satisfies
the ancestry rule; it proves commit identity, not a particular source claim.
Any declared file or code scope additionally requires its content witness.
The record cannot change the meaning of a durable commit field into an
unresolvable attribution hint. No extra origin-commit hint is introduced.

Source selection is mechanical: the declared repository source, explicit
repository file references, and code scopes obtained through the existing
typed association rules. Literal repository paths in `learned_from` are
explicit file references; other strings remain narrative attribution and are
never interpreted as executable commands or proof. Every declared repository
file is required, not merely one representative file. A code scope expands
through the existing inventory, including resolved typed `about` associations.
A malformed or unresolved declared repository target refuses capture.

`memory add` and deterministic reconciliation generate bindings using this
same selector and encoder. Handwritten records undergo the same checks.
Removing a commit field, changing the record kind, or superseding the record
within the proposal does not waive a declared source's evidence obligation.
Literal Document body hashes and closeout receipt rules retain their existing
meaning; they are not silently relabelled as evidence of arbitrary other files.

### 3.2 Snapshot resolution before and after landing

**Decision:** Resolve references using pinned candidate and trusted-base
snapshots supplied through the existing gate boundary. A new record is an
identity absent from the trusted base. All new identities are checked,
including ones that are already superseded or revoked in the candidate.

Before merge, a `record-introduction` binding must match the exact proposed
landing tree. For the existing workflow this is the gate's committed candidate
snapshot with its validated event base; a local run must satisfy the same
base relationship and candidate-tree checks. A stale base or mismatching
integration tree requires refresh and revalidation, never a guessed pass.
Any hosting integration that computes a different merge result must validate
that result before accepting the proposal.

After merge, resolve introduction along the trusted mainline's first-parent
history, with rename following and Git replacement objects disabled. Enumerate
additions of the exact record path, including a root commit and additions by
a merge commit; inspect the full parent trees where Git's path simplification
would otherwise omit an addition. The introduction is the earliest such
snapshot whose stored record bytes equal the immutable record being checked.
Reject a path/identity rewrite, removal and reintroduction, or ambiguous binding;
an arbitrary later commit with matching source bytes is not a substitute.
Read and hash the manifest's files from this introduction snapshot. Subsequent
source changes, renames, and deletion do not invalidate its historical witness.

For an inherited binding, discovery stops at the trusted base. A proposal
cannot manufacture older evidence or use a sibling branch, tag, reflog,
replacement ref, alternate object store, or GitHub pull-request ref as its
retention root. Missing history is an explicit failure to verify, distinct
from proven absence; the verifier never fetches more history on its own.

For a code-change acknowledgment, historical evidence alone is insufficient:
the existing code-transition check must require that the fresh semantic
record's unit witness matches the current changed unit. Historical lessons
may cite older retained snapshots, but cannot thereby acknowledge unrelated
new code. This is content association, not semantic evaluation of the prose.

### 3.3 Invariants

- **DINV-1** — Given the same record bytes, trusted base, candidate tree, evidence format, inventory version, and retained history, evidence capture and verdict output are byte-identical. No wall clock, randomness, model response, mutable ref lookup after pinning, or ambient branch inventory affects them.
- **DINV-2** — Every accepted new repository evidence binding can be reproduced in a normal full clone of mainline after a squash, without fetching a topic commit or consulting local execution logs.
- **DINV-3** — A witness matches exact file paths, modes, bytes, and required file membership. Equal filenames, commit existence, ancestry of the proposal head, or similar text are insufficient.
- **DINV-4** — All consumers use the same validation implementation. Record admission, draft repair, direct memory checks, the shared gate, and offline historical verification cannot disagree for the same pinned inputs.
- **DINV-5** — New invalid or absent required evidence is blocking independently of the reconciliation debt inventory. No baseline generation, legacy classification, supersession, or record-kind change can excuse it.
- **DINV-6** — Capture and repair never modify an identity already present at the trusted base. Re-running a completed draft repair writes zero files and preserves record IDs and issuance fields.
- **DINV-7** — Historical replay finds the record's actual introduction and does not reinterpret a later snapshot as its source. Incomplete history or a failed Git read cannot produce a verified result.
- **DINV-8** — Evidence operations make zero network or model calls and execute no source text, record text, hook, filter, or agent instruction. They work with AI tools and credentials absent.

### 3.4 Observability

Evidence checks return a versioned, canonical JSON report plus a human rendering
of the same data. Entries sort by record path, source path, and defect code.
Each includes the record ID, locator, pinned base and candidate identities,
resolved introduction where applicable, expected/observed mode and digest,
classification, and a specific recovery command. Success, invalid evidence,
legacy-unverified evidence, and insufficient history remain distinct.

Proposed defect categories are `missing-durable-evidence`,
`non-durable-evidence-commit`, `evidence-content-mismatch`,
`evidence-membership-mismatch`, and `evidence-history-unavailable`; schema
defects reuse the record validator. Blocking defects propagate a failing
exit status. Counts are derived from report entries, not stored rollups.
The required CI check exposes failure to the proposal author. Execution
diagnostics use the existing ignored memory/gate log homes and correlate
through the pinned input tuple. No daemon, new alert service, or model-based
diagnostic is part of this change.

The shared memory gate owns retention of blocking `memory-evidence-report/v1`
events through the existing store's exclusive atomic publisher. The CLI guide
binds the shared evidence home; `<canonical-report-sha256>.json` names a
single immutable event for the pinned input tuple and findings; identical
failures deduplicate, and subsequent repairs or clean passes cannot overwrite
it. The proposal author or existing delivery caller stages and commits this
Git-trackable event before revalidation. Direct check/preview stays read-only
and prints the exact retention command; `memory evidence --retain` explicitly
publishes the event. Draft repair retains its failing preimage before any
replacement. These commands never commit or push automatically. Tests remove
ignored logs and verify that committed failure events survive a later clean
run and a fresh clone. This is the AP-28 retention boundary.

### 3.5 Security & privacy

The boundary reads untrusted repository records feeding merge controls.
Parse closed, use argument arrays through the Git adapter, validate canonical
paths before access, and read regular blobs from Git trees. Reject absolute
paths, traversal, Git revision syntax in paths, newline/NUL injection,
symlinks, submodules, and self-references to the record envelope. Disable
replacement objects and lazy fetching. Do not invoke checkout filters,
textconv, hooks, shell evaluation, external diff drivers, or model tools.

Working-tree cleanliness uses raw file comparisons and index-only Git reads,
because refreshing Git's worktree index can invoke clean filters. Checkouts
must preserve the pinned blob bytes and file modes; transformations are never
executed as part of evidence verification.

A candidate cannot select its own trusted base or grandfather itself.
Hosted bases retain event-supplied provenance; local bases remain explicit
or configured through the existing mainline resolution. Parse failures,
cancellation, missing objects, and ambiguous history fail verification.
SHA-256 proves byte identity, not authorship, semantic truth, or runtime test
success. A stored test source proves which test was cited; a claim that a test
ran requires a separately retained result artifact. Existing human approval
and protected-mainline controls remain the authority for landing changes.
These controls implement DINV-3, DINV-5, DINV-7, and DINV-8.

### 3.6 Deterministic capture and draft repair

**Decision:** Add evidence operations under the existing memory CLI, with
read-only check/preview and an explicit draft-binding write mode. Inputs are
the exact base and source snapshot plus explicit record IDs or paths. Binding
is a pure transformation of fixed record metadata and selected source bytes;
it does not allocate new IDs, timestamps, or entropy. Ordinary creation keeps
the existing record issuer's supplied identity/time separate from this
transformation; changing that metadata is a different input.

Capture reads a pinned source snapshot. It refuses selected source edits that
are not represented there. For topic sources it emits `record-introduction`
only after proving the same selected bytes and membership are present in the
candidate. Stable historical sources may emit `mainline-commit`. An existing
topic commit reference is repaired only when the original declared evidence
can be read and exactly matches the selected retained snapshot; no semantic
guess or automatic substitution of a nearby commit is allowed. Missing or
changed original evidence produces a named failure and writes nothing for
that record.

Before any write, validate the whole requested batch, its unchanged expected
record bytes, selected source bytes, and base/head pins. Use the existing
memory writer's lock and atomic file replacement for draft files, preserving
all non-evidence fields and leaving base records untouched. A failed file
write reports the exact applied/pending set; retry converges. The batch is
not claimed to be a cross-file transaction, and no gate certifies a partially
repaired set. A changed pin or file causes refusal, not overwriting concurrent
work. No Git commit, push, merge, or repository-setting mutation is performed.

The writer lock uses a persistent kernel-held sidecar, with no age-based
takeover of a live owner. Release is idempotent and closes only the holder's
descriptor; it never removes another owner's lock path. The kernel releases
ownership on process exit. An old directory-style lock refuses the upgrade
until its old writer is quiescent and the directory is removed. A deterministic
two-writer fixture pauses a holder, proves that age cannot transfer ownership,
then releases it and proves that a repeated old release cannot unlock its
successor. Expected-byte checks and atomic replacement run within this
exclusion boundary.

### 3.7 Adoption, historical debt, and integration

**Decision:** New bindings and new source-bearing records are fully enforced;
unchanged legacy records already present at the trusted base are reported as
legacy-unverified where durable evidence cannot be proven. This classification
comes from exact trusted-base record bytes, not an author-controlled tag,
clock, cut-off date, or allowance entry. New invalid references cannot join it.
Existing valid mainline references can be verified without rewriting them.

Upgrade the strict readers before enabling new-format writes in installed
flows; an older reader may reject the added field and must be upgraded, never
silently drop it. No automatic corpus-wide rewrite accompanies adoption.
Repairs to frozen records use new successor records under the existing memory
rules, with an honest distinction between the successor's verified evidence
and the predecessor's unresolved historical reference. The existing baseline
allowance set is not widened for this feature.

Wire validation into the shared memory predicate already used by local hooks
and hosted CI, ahead of any debt-baseline acceptance. Direct memory checks and
offline verification call the same implementation. Admission's source pin is
rechecked at the final gate; admission alone is not a merge certificate.
Scheduled and mainline inventory runs verify all new-format historical
bindings and report legacy coverage explicitly. Source tree, trusted base,
verifier format/version, and relevant history identities belong in the
existing gate cache key; insufficient-history results are never cached as
success. The parity check covers every hosted template and required input.

Deployment includes checking that the hosted memory result is a required
merge check, plus the normal protected-mainline invariant. A workflow merely
running successfully is not proof that its failure would block a merge.
The check must cover the current integration tree: on GitHub, require branches
to be up to date for the required context, or use an equivalent validated
merge queue. A previously green result against an older base is insufficient.
Repository administration remains a separate authorized operation; this
design neither changes squash settings nor grants merge authority.

### 3.8 Testing strategy and boundaries

Pure parsers, selectors, manifest encoders, and reducers are tested directly.
Git is local-substitutable: use real repositories and transport clones, never
a mock ancestry function. Fixtures may use local bare repositories; they
cannot depend on GitHub retaining objects after branch deletion. The core
works with provider tools and AI executables removed from PATH and network
access denied. Result equality is an exact assertion, not a timing threshold.

This mechanism proves evidence identity and retention. It does not prove the
truth of arbitrary prose, discover undeclared dependencies, recreate missing
historical source bytes, or certify test execution from test source code.
Sources unavailable in the intended landing tree require an explicit retained
artifact or an already-retained historical snapshot; v1 does not create a
second general-purpose evidence store. Governance, frozen-record immutability,
closeout's mainline-only rule, and the chosen squash policy remain in force.

## 4. Alternatives considered

| ID | Alternative | Verdict and reason |
|---|---|---|
| ALT-1 | Require history-preserving merges and check ancestry. | Rejected by operator: squash support is required. Also does not validate the cited source bytes. |
| ALT-2 | Validate commit existence or ancestry against the proposal head. | Rejected: the motivating transport-clone experiment passes before squash and loses both commit objects afterward. |
| ALT-3 | Store every cited source in a separate immutable evidence archive. | Viable when intermediate bytes must be retained, but deferred here: ordinary candidate evidence already survives in Git trees; a second store adds duplication and another retention contract. |
| ALT-4 | Rewrite evidence to the final merge SHA after landing. | Rejected as the primary mechanism: there is an unverifiable interval, frozen records cannot be edited, and a repair commit cannot embed its own future hash. Mainline references remain available for historical sources. |
| ALT-5 | Reject every historical record whose commit is no longer reachable. | Rejected for adoption: unrelated inherited debt would stop all changes. Report that debt explicitly while blocking every new invalid binding. |
| ALT-6 | Ask an AI reviewer to detect or repair broken references. | Rejected by operator: the solution must be independent of AI and deterministic. Exact comparisons completely decide this property. |

## 5. Risks

| ID | Risk | Owner | Impact | Mitigation | Verification / evidence |
|---|---|---|---|---|---|
| R1 | Shallow/partial history looks like a missing introduction. | implementation | False pass or misleading failure. | Distinguish unavailable history; fail verification; no lazy fetch. | A-5. |
| R2 | Capture omits unit members or cited tests. | implementation | A witness certifies less than the declared scope. | Closed source selection plus exact membership and explicit test paths. | A-3. |
| R3 | Existing readers reject new evidence fields. | release owner | Installed flows fail until upgraded. | Upgrade readers first; preserve closed parsing and fail visibly. | A-8. |
| R4 | Gate bypass through legacy classification or debt allowances. | implementation | New broken evidence lands. | Trusted-base identity comparison outside the baseline ratchet. | A-6. |
| R5 | Historical resolution repeatedly scans too much history. | implementation | Verification cost grows with the corpus. | Batch exact path additions and file reads; cache within one pinned run, never by moving refs. | A-9. |
| R6 | Passing workflow is optional at merge. | operator | Checks diagnose but do not enforce. | Verify required-check protection at deployment. | A-10. |

## 6. Acceptance

| ID | Acceptance criterion | Verify | Required evidence | Traces to |
|---|---|---|---|---|
| A-1 | Repeated capture, repair preview, and verification with identical pinned inputs emit byte-identical manifests/reports with AI tools absent and network disabled. | Test | Golden outputs; denied-network and missing-AI executable fixture; zero model/network invocation counters. | §2; DINV-1; DINV-8 |
| A-2 | A repaired equivalent of the six-record motivating case passes after squash, topic-ref deletion, and a transport clone containing only full mainline history; the old commit-only form fails. | Test | Real Git E2E output proving identical final source bytes, absent topic commit objects, and expected gate verdicts. | DINV-2; DINV-5 |
| A-3 | Wrong byte, mode, source path, omitted cited test, removed/added unit member, duplicate path, symlink, and unknown schema each produce the named failure; complete exact witnesses pass. | Test | Table-driven mutation fixtures for capture and gate, exercising the same selector. | DINV-3; DINV-4 |
| A-4 | Sibling-commit, stale-rebase, and post-capture source-change cases cannot pass as commit-only evidence; exact draft rebinding succeeds, changed original evidence refuses without guessing. | Test | Real Git ancestry fixtures and record-byte comparisons before/after repair. | DINV-2; DINV-3; DINV-6 |
| A-5 | Later source edits, rename, and deletion retain historical verification; rewritten/reintroduced record paths and unavailable history fail explicitly. | Test | Mainline history fixtures, merge-addition fixture, root-addition fixture, shallow and missing-object fixtures. | DINV-7 |
| A-6 | Baseline regeneration, a newly superseded/revoked record, changed record kind, and a forged legacy tag cannot suppress new evidence failures; unchanged legacy debt remains visibly unverified. | Test | Direct-check, shared-gate, and offline-verification fixtures with the original allowance inventory unchanged. | DINV-4; DINV-5 |
| A-7 | Draft repair preserves IDs, issuance fields, prose, and all base-record bytes; repeated repair is a no-op; races and write failures are recoverable with explicit applied/pending results. | Test | File hashes, second-run zero-write result, pin-race and write-failure injection; paused live holder, rejected takeover, and idempotent old release with two writers; immutable committed failure event survives log deletion, later repair, and a fresh clone. | DINV-1; DINV-6; §3.4; §3.6 |
| A-8 | Existing valid stable-commit evidence remains verifiable; legacy records are not silently upgraded; supported installed writers/readers generate and validate the new binding without AI. | Test | Compatibility fixtures and installed-CLI E2E through the public command path. | §3.7; DINV-4; DINV-8 |
| A-9 | Local and hosted invocations use one predicate; relevant base/tree/history/version changes invalidate cached verdicts; source/history reads are batched rather than one full-history traversal per record. | Test | Parity and cache-invalidation tests; deterministic Git-operation counts at two fixture corpus sizes. | DINV-1; DINV-4; R5 |
| A-10 | A failing evidence result prevents acceptance through the deployed required-check configuration, while a valid squash proposal remains mergeable through the ordinary authorized workflow. | Test + Inspection | Required-check/rules configuration readback and a controlled failure exercise before activation is declared complete. | §3.7; DINV-5; R6 |

## 7. Open questions

| ID | Question | Owner | Blocks | Status | Resolution |
|---|---|---|---|---|---|
| Q1 | Approve this evidence contract and its enforcement/adoption boundary? | @mdarabi | — | resolved | Operator approved the proposal on 2026-09-16: "ok, proposal approved". Squash support and AI independence remain binding. |

## Principles

- **AP-1 / AP-4 — honored.** Evidence semantics use the existing Git and memory surfaces; provider workflows supply pinned inputs without owning the predicate.
- **AP-5 — honored.** Canonical reports identify each failed witness and its pinned inputs.
- **AP-6 — honored.** Closed, versioned evidence data and exact hashes cross the boundary; no free-text matching contract is introduced.
- **AP-7 — honored.** Frozen records remain immutable; historical correction is an explicit successor.
- **AP-10 / AP-19 — honored.** Evidence operations are ordinary deterministic code with zero inference and no background service.
- **AP-11 — honored.** Admission, repair, direct checks, offline checks, and local/hosted gates share one validator.
- **AP-13 / AP-14 — honored.** A transport clone after squash is the retention proof; verdicts are bound to exact candidate and base snapshots.
- **AP-15 — honored.** Pins are resolved once and checked again at the write/acceptance boundary; incomplete history never becomes a pass.
- **AP-20 / AP-22 — honored.** Draft writes are explicitly scoped, guarded against concurrent changes, and converge on retry.
- **AP-24 — honored.** The governed candidate cannot regenerate its own retention policy or inherited-debt exemption.
- **AP-28 — honored.** The shared gate and draft writer retain immutable failure events in Git-trackable files; the delivery caller commits them before revalidation. Read-only checks expose the explicit retention command.
- **RW-5 — honored.** This document decides one mechanism: deterministic durable evidence under history-rewriting merges. Implementation follows approval.
- **Deviation:** none.
