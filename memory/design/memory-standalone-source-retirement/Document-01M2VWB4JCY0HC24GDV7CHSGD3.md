# Standalone source retirement — technical design

| Field | Value |
|---|---|
| **Status** | Approved — operator approval, 2026-09-18 |
| **Owner** | @mdarabi |
| **Approver(s)** | @mdarabi |
| **Date** | 2026-09-18 |
| **Relevant-when** | retiring an imported skill entry or vendor README that has no feature-closeout owner |
| **Scope** | Exact, offline retirement evidence for standalone mutable source documents, through the existing memory writer and reconciliation validator. No bypass for feature-owned or frozen governing documents. |
| **Tracking** | https://github.com/agenticlab-ai/pyxis/pull/1199 |
| **Builds-on** | — |
| **Supersedes** | — |
| **Serves** | doc:cmd/memory/README.md |
| **Security/privacy impact** | yes — a gate decision authorizes deletion of a captured authored source; no network or secret access |
| **Template version** | tech-design-template/v1 |

## 0. Read to rule

*Human-only routing; the body governs.*

Approve the narrow additional retirement path in §2. Its purpose is to delete imported skill/vendor documents whose exact history is preserved but which cannot acquire a feature closeout. The alternative is to preserve the files indefinitely. Publication order is specified in §3.5; the candidate deletion cannot authorize itself.

## 1. Problem

The authored-source ratchet protects every prior source path. Its only retirement exception comes from a feature closeout already present at the trusted base. The ownership resolver recognizes PRD, design, brainstorm, plan, review, e2e, and problem artifacts. It does not assign skill entries or vendor READMEs to a feature. Consequently the vendor-neutral `doc-review` skill entry and the `lavish-axi` vendor README can never satisfy the existing exception, even after the operator explicitly requests removal and their contents are retained in immutable memory.

## 2. Decision

**Decision:** Extend the existing memory CLI and reconciler with an exact standalone-source retirement seal. Eligible paths are canonical `skills/<slug>/SKILL.md` and `vendor/<slug>/README.md` regular files under Pyxis's vendor-neutral system root, already captured as active literal Documents and owned by no feature. This is an additional evidence type, not a new exemption list or a relaxed feature closeout.

The writer accepts one or more source paths and an explicit retirement reason. It derives every identity from the same configured trusted-mainline snapshot, requires HEAD to equal that mainline, and writes one immutable Document receipt per source through the existing store. The entire batch is preflighted before the first write. It never deletes the sources. A later deletion is allowed only after its receipt is present in the trusted base and passes the same offline validator used by reconciliation.

## 3. Design

### 3.1 Receipt and writer

The versioned receipt binds the source path, regular Git mode, raw SHA-256, retained-mainline commit, active literal Document ID and body hash, and the explicit retirement reason. It lives in a deterministic source-retirement subject keyed by the source path's SHA-256, with family `seal`, using the existing Document envelope and literal-body storage. Parsing is closed and the encoding canonical. Repeating the identical request is a no-op; a changed source requires a new capture and superseding seal.

The command follows the existing closeout shape: `memory retire-source --source <path> [--source <path> ...] --reason <text> --check|--write [--actor <actor>] [--json]`. It requires a nonempty reason, unique source paths, and exactly one action. Unknown flags, flag-like paths, unsupported paths, symlinks, missing captures, inconsistent captures, feature owners, unrelated dirty source/memory state, and topic-only HEADs refuse before writing. The write lock and HEAD recheck use the existing writer discipline.

An identical retry may recognize the complete, exact pending output of the same batch before the ordinary clean-memory check and return already-sealed without writing. This exception admits only those newly written receipt records and their literal bodies, verified against the clean pinned mainline inputs and requested paths/reason; no source changes, unrelated records, altered receipt bytes, or incomplete batches qualify. A fresh batch requires clean memory. This permits issuing both Lavish seals together and immediately retrying without permitting sequential topic-only issuance or extending a dirty batch.

### 3.2 One offline validator

Validate the receipt's record identity, canonical encoding, exact historical Git blob and mode, retained-mainline ancestry, and its bound active capture. Both the issuance snapshot and trusted-base snapshot must classify the path as an eligible standalone source with no feature owner. Malformed or unverifiable receipts fail closed. A candidate-authored receipt cannot authorize a deletion in its own diff.

The trusted-base deletion check additionally requires the base path's bytes and mode to equal the seal. A source edited after sealing needs replacement evidence. An edit-delete-reintroduction sequence cannot revive stale authority: introduction identity is checked against retained history. Existing feature-closeout verification remains unchanged.

Reconciliation and offline verification share this predicate. When the source is absent, the exact retained capture satisfies its source-memory binding. Additional active captures with different bytes, a revoked/superseded capture, or a revoked receipt refuse retirement. The validator does not erase historical records or suppress unrelated findings.

### 3.3 Observability

`--check` and `--json` report ready, already-sealed, or a named refusal with the exact path and failed identity. Reconciliation findings distinguish missing trusted receipt, changed source, unavailable history, and invalid memory binding. The committed receipt is the durable decision evidence; optional execution diagnostics remain under ignored logs.

### 3.4 Security & privacy

No network, model inference, fuzzy matching, environment-selected trust root, or candidate allowlist participates. The configured mainline and event-supplied trusted base retain authority. Frozen governing documents, feature context, code units, arbitrary source paths, and unknown future receipt versions stay outside this narrowly admitted path. The existing debt inventory is not widened.

### 3.5 Rollout

Land the reader/writer gate repair first. At the resulting mainline commit, issue the exact two source seals in one batch on a receipt-only branch, keeping both original files present. Land that evidence, refresh mainline, then delete the skill and vendor directory in the Lavish removal PR. Human review and merge remain required; no implementation PR may certify its own base or auto-merge its prerequisites.

### 3.6 Invariants

- **DINV-1** — a candidate deletion never supplies its own retirement authority.
- **DINV-2** — every accepted retirement binds one exact path, mode, historical introduction, body, and immutable capture.
- **DINV-3** — any feature owner, unsupported path class, or conflicting capture refuses this standalone route.
- **DINV-4** — the writer does not delete source files, mutate existing memory, or widen baseline allowances.
- **DINV-5** — one deterministic predicate governs writer checks, reconciliation, and offline verification.

## 4. Alternatives

| Alternative | Disposition |
|---|---|
| Retain placeholder files indefinitely | rejected by the operator's explicit deletion instruction |
| Invent feature ownership for these paths | rejected: misrepresents their lifecycle and still requires unrelated feature completion |
| Allow any missing source with a revocation or free-text reason | rejected: no exact trusted-base binding; an incorrect capture revocation is not retirement authority |
| Add path exceptions or baseline allowances | rejected: lets the deletion authorize itself and hides debt |

## 5. Risks

| Risk | Mitigation and evidence |
|---|---|
| Candidate receipt laundering | actual Git transition test rejects same-PR receipt plus deletion |
| Stale bytes or reintroduced path | edit-after-seal and delete/re-add fixtures refuse |
| Squash loses provenance | issue only from mainline; verify the sequence in a fresh transport clone after squash and branch deletion |
| Standalone path becomes feature-owned | derive owner sets at issuance and trusted base; added-owner fixture refuses |
| Gate paths drift | one shared validator, mutation tests at CLI, reconciliation, and offline verification boundaries |

## 6. Acceptance

| ID | Criterion | Evidence |
|---|---|---|
| A-1 | Two exact eligible sources can be sealed in one mainline batch, an identical immediate retry produces no duplicate, and source bytes remain present and unchanged | production writer integration test |
| A-2 | Topic-only source, unrelated dirty state, altered or partial retry output, symlink, unsupported path, malformed schema, missing/revoked/conflicting capture, or feature owner refuses the whole batch with zero writes | table-driven refusal tests |
| A-3 | Same-diff receipt/deletion refuses; a landed exact seal permits a later deletion without new allowances | real-Git two-PR transition test |
| A-4 | Changed bytes/mode, reintroduced path, revoked seal, or unavailable history refuses consistently across all readers | mutation tests |
| A-5 | Squash and branch deletion preserve evidence in a fresh transport clone | standalone offline clone verification |
| A-6 | Both Lavish paths are absent from the final source tree and release payload, while their immutable captures remain verifiable | removal integration evidence |

## 7. Open questions

None. Expansion to other source classes is outside this repair.

## Principles

AP-6, AP-10, AP-11, AP-13, AP-15, AP-17, AP-20, AP-22, AP-24, and AP-28 are honored through a closed receipt, one deterministic predicate, exact trusted evidence, confined writes, idempotency, and retained history. AP-7 remains intact: this path never authorizes deleting frozen governing documents. AP-30 permits fixed paths inside Pyxis's own namespace; repository branch and remote resolve from existing configuration. RW-5 is honored by approving this decision before implementation. No deviation.
