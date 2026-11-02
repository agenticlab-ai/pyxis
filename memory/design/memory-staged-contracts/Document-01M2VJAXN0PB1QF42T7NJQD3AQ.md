# Memory integrity during staged contract changes — technical design

| Field | Value |
|---|---|
| **Status** | Approved — operator approval, 2026-09-18 |
| **Owner** | @mdarabi |
| **Approver(s)** | @mdarabi |
| **Date** | 2026-09-18 |
| **Relevant-when** | a PRD amendment must merge before its replacement design without fabricating design coverage or weakening source integrity |
| **Scope** | Separate incomplete design coverage from corrupt memory evidence in the existing reconciler; retain strict completion at planning and closeout. No source-deletion exception. |
| **Tracking** | https://github.com/agenticlab-ai/pyxis/pull/1199 |
| **Builds-on** | docs/tech-design/contract-gate-v1.md · docs/tech-design/memory-debt-partitions-v1.md |
| **Supersedes** | — |
| **Serves** | doc:cmd/memory/README.md |
| **Security/privacy impact** | yes — changes which reconciliation findings stop a proposal; no network or credential access |
| **Template version** | tech-design-template/v1 |

## 1. Problem

The document gate requires separate contract, design, and implementation PRs. The memory packet also describes undesigned claims and reopened designs as warnings. Nevertheless, reconciliation treats both incomplete design coverage and a frozen design's references to deliberately retired requirements as blocking integrity failures.

The approved installer amendment reproduces the cycle after all exact source captures, new typed claims, the FR-12 revocation, and the R8 risk disposition have been written through the memory writer. The ratchet reports eight new addresses: four missing targets in installer-v1 (FR-12, NFR-1, UB-2, UB-7) and four undesigned successor requirements (FR-31, FR-32, UB-14, UB-15). There are 699 total findings against the 700-item baseline, but the different addresses correctly prevent using the count as a loophole. The successor design would resolve these states, but check-standalone-docs prevents adding it to the contract PR.

Evidence: `internal/docgate/gate/reg_check_standalone_docs.go`, `internal/memory/reconcile/analyze.go`, `internal/memory/reconcile/baseline.go`, `internal/memory/packet/packet.go`, and `cmd/memory/README.md` steps 3 and 6. The existing contract-gate design §3.11 explicitly allows incomplete inverse coverage on contract-document PRs because a PRD legitimately merges first.

## 2. Decision

**Decision:** Reconciliation distinguishes integrity defects from unfinished design work. Exact captured, live typed requirements that have no serving design are reported as undesigned; an unchanged frozen design whose exact declared source requirement has been superseded or retired is reported as reopened. These states remain visible and prevent feature closeout, but do not become debt allowances or block the preceding contract PR solely because design work is pending.

All source, provenance, schema, native-record, declared-live-coverage, code-association, and deletion checks remain blocking. A missing or unknown requirement is never classified as a deliberately retired requirement. This corrects the workflow conflict in the existing memory and contract gates, without adding a bypass flag or an installer-specific exception.

## 3. Design

### 3.1 Classification

Use the existing folded PRD statuses, immutable record projection, design index, and exact captured design source. An unserved requirement becomes an explicit completion finding only after its active memory claim is present and exactly matches the live folded unit. A design reference becomes reopened only when its exact declared address exists in the retained PRD ledger with an explicit superseded or retired status and its memory transition is valid.

Unknown addresses, absent live claims, source drift, stale active claims on retired rows, invalid revocations, invented serves edges, missing native design decisions, and absent edges for a design's declared live requirements remain integrity defects. Newly authored decisions cannot claim current coverage of a retired address. Historical reopened edges are diagnostic history, never current coverage.

### 3.2 Reporting and enforcement

Add a distinct, closed, versioned completion-finding section to the reconciliation report. It names the exact requirement/design addresses, their state, and the remaining action. Existing status, delivery, and packet views retain their visible undesigned/reopened semantics. No successful proposal or report asserts that implementation or feature completion has occurred.

The debt ratchet inventories integrity findings only. Completion findings are not allowances and cannot mask unrelated integrity failures. Planning continues to use the existing contract-gate severity map and planning-entry checks. Feature closeout and its offline replay require both integrity and completion findings to be empty for the feature, preserving the completed-feature seal's stronger claim.

### 3.3 Baseline compatibility

The generator version changes for the new classification. Trusted older baselines, including partitioned v2 policy under the v3 evaluator, are reproduced under their exact historical classification before comparison; the candidate always uses the new rules. The migration can remove old completion allowances, but cannot add, broaden, or rename an integrity allowance. The partitioned policy layout and coded baseline synchronization remain the owning storage mechanisms. Source and code transitions remain checked against the actual trusted base as well as any replay snapshot. Replaying an old baseline never selects the candidate's enforcement mode.

The requirements-first marker introduced before this design covers only a new ledger before its first design. This successor classification also permits subsequent contract amendments to precede their replacement designs. It does not edit or retrofit the immutable marker in old PRD bases; historical v3 evaluation continues to interpret that marker exactly as before.

### 3.4 Observability

Machine and human reports show integrity and completion counts separately. Gate output states that the contract is admitted with named design work outstanding. Closeout names each outstanding completion finding and refuses. Immutable records remain the evidence; execution logs remain optional and ignored.

### 3.5 Security & privacy

No model, network, time-based grace period, branch-name exception, candidate-selected trust root, blanket advisory switch, or path whitelist participates. Explicit source lineage and native evidence govern classification. The source-retirement ratchet is unchanged by this design, including the requirement for evidence already landed at the trusted base.

### 3.6 Invariants

- **DINV-1** — incomplete coverage cannot fabricate a serves relation or certify feature completion.
- **DINV-2** — an unknown, absent-live, mismatched, or unverifiable target remains a blocking defect.
- **DINV-3** — no integrity allowance grows during generator migration or subsequent staging.
- **DINV-4** — planning and closeout retain their explicit completion checks.
- **DINV-5** — candidate source deletion cannot authorize itself or use completion classification to avoid the retirement check.

## 4. Alternatives

| Alternative | Disposition |
|---|---|
| Bundle PRD and design into one PR | contradicts the existing standalone-contract rule and requires changing a separate governed contract |
| Add the eight addresses to the baseline | hides newly introduced findings and violates the monotonic debt inventory |
| Invent a design Decision before the indexed source exists | supplies false coverage and is already rejected by the exact serves validator |
| Add a branch flag or temporary bypass | lets the candidate select weaker enforcement and leaves staging correctness unproved |

## 5. Risks

| Risk | Mitigation and evidence |
|---|---|
| Corruption is misclassified as unfinished work | negative fixtures for absent live targets, unknown IDs, edited source, wrong family, fake or stale serves edges, and invalid revocations |
| A feature closes while coverage is incomplete | closeout and offline replay tests refuse the same completion states admitted for staged authoring |
| Existing debt is hidden during migration | exact old-generator replay and per-address monotonic comparison; independent assertions for unchanged integrity findings |
| Staging works only in a topic checkout | real-Git contract → design → implementation sequence, squash each prerequisite, delete branches, then verify in a fresh transport clone |

## 6. Acceptance

| ID | Criterion | Evidence |
|---|---|---|
| A-1 | An exact new or amended PRD can merge before its design; undesigned and reopened states remain visible | real-Git staged contract fixture and packet/report assertions |
| A-2 | The successor design closes only its declared current coverage after exact native memory is written | design-stage positive and wrong-target negative tests |
| A-3 | All source/provenance/code/deletion and declared-live-coverage integrity failures still stop the gate | adversarial mutation matrix |
| A-4 | Incomplete coverage refuses planning under the existing hard trigger and refuses feature closeout/offline completion | production entrypoint tests |
| A-5 | Old baselines replay exactly and migrate without any new integrity allowance | generator compatibility and ratchet tests |
| A-6 | The approved installer contract/design sequence passes after separate squash merges; both Lavish source deletions still require separate trusted retirement evidence | fresh transport-clone regression |

## 7. Open questions

The operator approved the integrity-versus-completion boundary in §2 on 2026-09-18. The separately approved standalone-source seal remains required to delete the two Lavish files; this correction does not grant that authority. No open decision remains.

## Principles

AP-6, AP-10, AP-11, AP-13, AP-17, AP-24, and AP-28 are honored through typed reports, one deterministic classification, exact historical evidence, fail-closed integrity, and visible retained decisions. AP-7 preserves frozen sources and immutable records. RW-5 requires approval before implementation. No other gate or contract is weakened.
