# Rebase-stable memory admission — technical design

| Field | Value |
|---|---|
| **Status** | Approved — operator sign-off 2026-09-17; merge is the freezing event |
| **Owner** | @mdarabi |
| **Approver(s)** | @mdarabi (operator; immutable memory and merge-control surface) |
| **Date** | 2026-09-16 |
| **Relevant-when** | changing memory record admission, candidate/base comparison, record ordering, or historical-effect guards; diagnosing record reissuance after an unrelated rebase |
| **Scope** | Remove repository-wide record-age admission while retaining explicit causal relations, immutable history, and historical-effect protection. Baseline storage, evidence locators, code-anchor granularity, and merge policy are separate decisions. |
| **Tracking** | Operator request of 2026-09-16 to develop the parallel-PR memory proposal; no issue or PR created |
| **Builds-on** | — |
| **Supersedes** | — |
| **Serves** | prd:parallel-memory/FR-6 · prd:parallel-memory/FR-7 · prd:parallel-memory/FR-8 · prd:parallel-memory/FR-9 · prd:parallel-memory/NR-2 · prd:parallel-memory/UB-4 · prd:parallel-memory/UB-5 · prd:parallel-memory/UB-6 · prd:parallel-memory/NFR-1 · prd:parallel-memory/NFR-3 |
| **Security/privacy impact** | yes — changes record admission feeding merge controls; preserves relation causality, immutable bytes, and historical-effect checks · reviewer: @mdarabi |
| **Template version** | tech-design-template/v1 |

## 0. Read to rule — the human reviewer's section (~4 min)

*The body is the contract; this section only routes attention — on any conflict
the body governs; machines ignore it.*

**Judge these**

| # | The call | Could have gone | If you disagree |
|---|---|---|---|
| 1 | An unrelated later record cannot force an earlier branch's records to be reissued. | Keep the global age floor and automate reissuance, ALT-1. | Retain identity and reference churn on rebase → §2. |
| 2 | Preserve ordering on explicit relations and refuse changes that activate previously rejected historical acts. | Remove chronology checks wholesale, ALT-2. | Choose a different causal resolver → §3.1 and §3.6. |
| 3 | Share historical-transition validation between admission and the merge packet. | Relax only the packet check, ALT-3. | Leave writer and gate with different validity rules → §3.6. |

**Sanity-check the numbers** — zero existing-record byte/ID changes for an unrelated rebase; zero new graph defects or revived historical acts accepted; no new shared counter → DINV-1 through DINV-5.

**The author's open questions** — admission change approved on 2026-09-17 → Q1; package-anchor coordination is explicitly outside the guarantee → §3.2.

**Skip the rest** — structure/pairing → docgate; substance → tech-design-review; actual behavior → the positive and adversarial fixtures in §6; alternatives → §4.

## 1. Problem

The current packet rejects a new record whose ULID sorts before the latest
record at the merge ancestor. Writer admission also rejects records older
than any record in its current tree. These global comparisons add a dependency
between otherwise unrelated subjects.

For example: branches A and B start at M. A issues a record, then B issues an
unrelated newer record and merges. A's initial packet can be valid while its
original merge ancestor remains M. Rebasing A onto B's mainline changes that
ancestor, and the unchanged record can fail `record-history-insertion`.
Reissuing it changes references and baseline hashes without adding knowledge.

At inspected commit `8b9c79c246ef025f981155aca93b567fe513b663`, the global
checks are in `internal/memory/packet/packet.go` and
`internal/memory/cli/cli.go`. The packet tests already distinguish legitimate
concurrent chronology from retroactively enabling an invalid revocation.
The repair must preserve that distinction after integration too.

This is a new infrastructure design grounded in the vision's parallel-memory
commitment. No frozen memory-specific design in the inspected index is being
partially overridden. The existing implementation README must be updated only
when the approved behavior is implemented.

## 2. Decision

**Decision:** Admit records according to their explicit dependencies and their
effect on existing history, not their age relative to unrelated records.
Remove the repository-wide record-age floor at writer and packet admission.
Keep immutable IDs and bytes across unrelated rebases and supported merge
forms, while preserving the existing ordering of every explicit relation.

**Decision:** A candidate must not make a previously invalid or dangling
historical act silently effective. Apply the existing before/after history
protection through one shared deterministic validator at both writing and
merge checking; weakening chronology is not permission to reinterpret history.

The proposed functions, call sites, and concrete adversarial fixtures are in
the paired [solution shape](../solution-shapes/memory-causal-admission-v1.md).

## 3. The design

### 3.1 Identity, causality, and independence

Keep the record schema and ULID generation. An ID is stable identity and the
existing resolver's deterministic ordering key; it is not proof of when an
act entered mainline or proof that the claim is true. Existing relations
continue to require a present, strictly older target with the valid kind,
subject, family, and lifecycle constraints for that relation. A forward edge
still fails, even when its two files were added in the same proposal.

An otherwise valid relationless record can be admitted with an ID older than
an unrelated record. The same holds for a record whose dependencies all
precede it. Rebase does not edit the ID, timestamps, provenance, or relations.
Clock skew against a real dependency produces a precise causal refusal;
unrelated future-dated records do not impose a global clock floor.

Independence is judged by graph and projection effects, not merely by different
filenames or subjects. Competing heads, answers, revocations, or replacements
of the same anchor remain conflicts. A backdated record that changes the
validity of an old act is not independent.

### 3.2 Boundaries and non-goals

- This is not a new identifier, vector clock, event-log order, or resolver.
  Per-edge chronology and deterministic projection remain intact.
- It does not make simultaneous changes to one claim family or code anchor
  automatically compatible. Package-wide anchors retain their current scope.
- It does not repair topic-only provenance after squash, invent missing source
  evidence, or modify any base record. The separately drafted durable-evidence
  proposal remains separate and unapproved by this document.
- The companion baseline proposal removes a different cause of integration
  churn. This chronology repair can ship against the current baseline format;
  its own implementation still follows that format until migration.
- No counter file, reservation service, shared issuance ledger, model call,
  or new module is introduced. Stored history and protected-mainline authority
  remain the source of record identity and admission context.

### 3.3 Invariants

- **DINV-1** — Integrating unrelated memory changes cannot require changing any existing candidate record's ID, bytes, timestamps, or references solely because its ID is older than records newly present on the base.
- **DINV-2** — Every explicit relation retains the current target-presence, strict causal-order, subject/kind, and lifecycle rules; newly introduced cycles, forward references, dangling targets, and competing exclusive claims fail.
- **DINV-3** — A previously rejected or dangling historical act cannot become effective through insertion of prerequisites or older records. Repair retires the invalid carrier and issues a fresh valid act; it never grants retrospective effect.
- **DINV-4** — Previously accepted records remain immutable. Add/add identity collisions, rewritten/deleted records, moved supersession targets, and competing answers/revocations retain their existing checks.
- **DINV-5** — Writer admission and packet comparison use the same pure historical-transition predicate. A failed admission writes nothing; fixed before/after inputs yield identical findings regardless of read order or ambient refs.
- **DINV-6** — Existing valid history keeps its projection without migration. No repository-wide reissuance, shared sequence allocator, new record schema, or provenance rewrite is required.
- **DINV-7** — The shared gate applies these rules locally and in hosted checks using its existing pinned base and candidate contract; no model or network call participates in the decision.

### 3.4 Observability

Remove `record-history-insertion` as a refusal for unrelated chronology.
Keep specific causal, immutability, moved-head, conflict, and
`historical-act-reinterpreted` findings. A historical-effect finding names the
old carrier, its previously rejected relation/defect, and the candidate
prerequisite that would change its effect. The same diagnostic data feeds CLI
admission and the packet's human/JSON presentation.

The required memory gate is the alert surface; no daemon or new telemetry
store is needed. Compare runs by pinned base/candidate and record IDs. For
acceptance, record the number of candidate bytes/IDs changed by integration
and the exact refusal codes, not wall-clock thresholds. Failure lessons are
retained in their owning review or incident; successful checks add no log or
counter to shared memory.

### 3.5 Security & privacy

Records are untrusted inputs to merge controls. Relaxing a global age check
must not create a way to revive an old invalid revocation, fulfill an old
dangling answer implicitly, change a document's interpretation mode, or
overwrite an existing identity. The shared transition check compares the
trusted before state with the proposed after state, including invalid and
dangling records; it cannot discard them before checking.

Do not infer authority from timestamps, ID order, issuer text, or branch
names. Existing merge approval supplies authority. Preserve closed parsing,
regular-file checks, explicit relation constraints, and the base's provenance.
No personal data beyond existing record metadata is introduced; no network or
model egress is added. DINV-2 through DINV-5 are the required controls.

### 3.6 One historical-transition predicate

Extract the packet's historical-violation and dangling-carrier comparisons
into the existing resolver package. The function accepts before/after
projections and returns stable typed findings. It compares each historical
carrier's prior rejected effect and dangling edges with the proposed result;
a valid retirement of the old carrier is the existing explicit repair route.
Check whether the previously refused relation is actually honored in the new
projection; disappearance of a diagnostic alone is not proof of activation.
It also retains the existing comparison that attributes new projection defects
to candidate changes rather than treating them as inherited warnings.

Writer admission builds a trial projection, including the validated envelope
and paired body for a Document, and applies both relation validation
and the shared historical predicate before storing any record, for every
writer path. Reconciliation, answers, questions, revocations, and document
revision reach the same admission seam. Packet comparison applies the same
predicate to the trusted base and its existing three-way memory union, while
retaining its additional Git immutability and moved-head checks.

The snapshot-only resolver remains deterministic and does not invent an
admission history from ULIDs. Historical-transition certification requires
the before state. An offline snapshot check may report current integrity;
it cannot claim to prove a transition without its trusted predecessor. The
shared merge gate retains that predecessor requirement.

Do not suppress this guard because another defect exists. Multiple causes may
be reported deterministically; fixing one defect must not hide a pending
historical reinterpretation. Projection validity, historical-effect safety,
and merge authorization remain distinct predicates.

### 3.7 Migration, rollout, and testing

There is no stored-data migration. First make the shared historical guard and
all-writer admission coverage explicit, retaining existing behavior. Then
remove the two global age comparisons and update the obsolete expectations
that reject harmless relationless insertion. Preserve the exact malicious
counterexamples; they must still fail through the relevant causal or
historical-effect predicate.

Use in-process graph fixtures for individual rules and the existing real-Git
CLI E2E harness for branch/rebase/merge behavior. Controlled test issuance
provides chosen ID order; tests never sleep for a newer clock tick. Build one
CLI for the fixture batch and run without AI tools or credentials. No new port
is needed. Required Go/shared-gate/hosted checks apply to implementation, not
to this proposal's authoring.

Rollback before adoption is ordinary code rollback. After an older independent
record has legitimately landed, restoring the global floor can reject later
valid proposals; such a rollback must preserve the shared historical guard
and does not reissue or delete landed records. Its compatibility must be
demonstrated against the same retained-history fixtures.

## 4. Alternatives considered

| ID | Alternative | Verdict and tradeoff |
|---|---|---|
| ALT-1 | Keep the global age floor and automate record reissuance after rebase | Rejected: preserves a simple rule but churns immutable identity, references, and evidence for unrelated changes. |
| ALT-2 | Replace ULID chronology with a new graph clock or drop all ordering checks | Rejected for this increment: requires a broader resolver/schema decision; the existing per-edge order already enforces acyclicity. |
| ALT-3 | Relax only merge admission and leave writers unchanged | Rejected: the same record becomes acceptable by hand but refused by the supported writer. |
| ALT-4 | Preserve per-edge chronology and share historical-effect validation | Proposed: permits independent records to survive integration while preserving the existing safety model at both admission boundaries. |
| ALT-5 | Use a global sequence allocator or require creation in merge order | Rejected: adds a new shared dependency and reduces the parallelism this change is intended to recover. |

## 5. Risks

| ID | Risk | Impact | Mitigation | Verification / evidence |
|---|---|---|---|---|
| R1 | A historical revocation becomes valid after an older insertion | Silent change to prior meaning | Shared before/after guard; owner: memory maintainer | A-3 exact existing malicious fixture preserved |
| R2 | A CLI writer bypasses trial projection | Writer/merge disagreement | Audit all paths through the existing store/admission seam; owner: implementer | A-4 writer matrix and zero-write assertions |
| R3 | A real dependency is misclassified as unrelated | Conflicting active memory | Keep per-edge and exclusive-family rules; owner: implementer | A-2 and A-5 negative cases |
| R4 | Base-less verification overclaims historical proof | Misleading green result | Name snapshot versus transition scope; owner: implementer | A-7 output and missing-base fixtures |
| R5 | Legacy warnings become a bypass | New defects treated as inherited | Compare exact prior defect identities and members; owner: implementer | A-3 dangling and expanded-conflict cases |

## 6. Acceptance

| ID | Acceptance criterion | Verify | Required evidence | Traces to |
|---|---|---|---|---|
| A-1 | A branch with an earlier record remains valid after an unrelated later record lands and the branch rebases. Exercise both issuance/integration orders, merge and squash integration, and a fresh mainline-only transport clone; candidate record bytes/IDs stay identical. | Test | Real-Git CLI fixture transcript, before/after digests, and gate results; baseline regenerated only when the old format requires it | DINV-1 · DINV-6 · vision §12 |
| A-2 | Relationless insertion older than unrelated history passes; the same ID order with a forward dependency, missing target, or invalid subject/kind relation refuses. | Test | Table-driven positive/negative cases through CLI and packet | DINV-2 · DINV-5 · vision §13 |
| A-3 | Preserve exact counterexamples for backdated document replacement enabling rejected revocation, concurrent-base revocation activation, newly supplied dangling-act targets, and widened inherited conflicts. Each still refuses for its real defect. | Test | Existing fixture inputs retained, expected causal/history diagnostics, no admitted effect | DINV-3 · DINV-4 · vision §13 |
| A-4 | All record-writing entry paths reach shared causal/history admission; every refusal writes zero record/body files. Existing valid fixtures remain accepted. | Test + Inspection | Call-site inventory and writer behavior matrix; batch reconciliation reports any earlier successful independent appends accurately | DINV-5 · DINV-7 · vision §13 |
| A-5 | Two successors to the same claim/anchor, competing answers/revocations, duplicate identities, changed base bytes, and a moved head still fail; disjoint families remain valid. | Test | Real-Git competing-branch cases and packet defect assertions | DINV-2 · DINV-4 · vision §12 |
| A-6 | Existing valid record trees resolve byte-identically before/after the change, no record schema or ID migration occurs, and fixed before/after inputs yield identical guard output under shuffled input order. | Test | Projection goldens, zero record-file changes, deterministic permutations | DINV-5 · DINV-6 · vision §13 |
| A-7 | Local and hosted gate modes share causal/history predicates; missing transition inputs cannot produce a historical certification. Tests run without clock sleeps, model calls, or network fetches. | Test | Gate-mode fixtures, explicit scope/refusal output, hermetic execution evidence | DINV-7 · vision §13 |

## 7. Open questions

| ID | Question | Owner | Blocks | Status | Resolution |
|---|---|---|---|---|---|
| Q1 | Approve removing the global age floor while retaining causal and historical-effect checks? | @mdarabi | nothing | resolved 2026-09-17 | Operator approved ALT-4 and continuation: “the design is approved. move on”. |

## Principles

- **AP-5 / AP-6 — honored.** Refusals retain typed evidence of the actual offending relation or historical effect.
- **AP-7 — honored.** Integration preserves immutable record bytes and identity; change remains an explicit new act.
- **AP-10 / AP-11 — honored.** One pure guard serves existing writer and packet seams; no duplicated policy or new service is introduced.
- **AP-13 / AP-17 — honored.** Real-Git proofs include malicious history cases; missing evidence fails closed.
- **AP-22 / AP-23 / AP-27 — honored.** Independent record additions remain separate files without a shared ordering allocator.
- **AP-24 — honored.** A candidate cannot grant retrospective authority by supplying a historical act's missing prerequisite.
- **AP-28 — honored.** Durable failures and decisions stay with owning artifacts; routine passes add no shared counters.
- **AP-29 — honored.** Controlled input IDs replace timing assumptions in correctness fixtures.
- **RW-5 — honored.** Record admission is one decision, separate from the baseline representation.
- **Deviation:** none.
