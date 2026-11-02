# Argus committed plan scope — bounded downstream documentation

| Field | Value |
|---|---|
| **Status** | Approved |
| **Owner** | @mdarabi |
| **Approver(s)** | @mdarabi (operator) — attribution and factory-control surface |
| **Date** | 2026-10-05 |
| **Relevant-when** | changing Argus committed-problem scope, numbered plan authoring, or native problem captures |
| **Scope** | Add bounded same-problem numbered plan documents and native problem captures after a valid delivery ledger is committed; retain the predecessor's first-PR grammar, binding refresh and single-user assurance. Planning readiness, protected-path classes, positive human attestation and historical exact-path ownership remain unchanged. |
| **Tracking** | AEC company-memory delivery E-67..E-69 and brief-scope-transition.md |
| **Builds-on** | — |
| **Supersedes** | argus-problem-capture-attribution |
| **Serves** | prd:argus-boundary-enforcement/FR-9 · prd:argus-boundary-enforcement/FR-10 · prd:argus-boundary-enforcement/FR-13 · prd:argus-boundary-enforcement/FR-14 · prd:argus-boundary-enforcement/NR-4 · prd:argus-boundary-enforcement/UB-4 · prd:argus-boundary-enforcement/UB-5 · prd:argus-boundary-enforcement/ABUSE-2 · prd:argus-boundary-enforcement/ABUSE-3 · prd:argus-boundary-enforcement/ABUSE-4 · prd:argus-boundary-enforcement/NFR-1 · prd:argus-boundary-enforcement/NFR-2 · prd:argus-boundary-enforcement/NFR-3 |
| **Security/privacy impact** | yes — changes attribution scope and states the actual single-user assurance: local Git stamps are audit evidence, not an isolation boundary against a drive that can run Git; no new credential or network surface · reviewer: @mdarabi |
| **Template version** | tech-design-template/v1 |

This is proposed merge-target content. The Approved header takes effect only after reviewed merge and asserts no prior operator approval. All unaffected binding content from argus-problem-capture-attribution is carried forward below. The committed-problem scope adds only numbered same-feature plan documents and the native problem-capture namespace; planning readiness remains a separate required predicate.

## 0. Read to rule — the human reviewer's section

The body governs. Judge the committed-problem additions in §3.3 and A-6. They admit numbered documentation and existing native captures for the same problem; they do not authorize planning before its binding contract lock or admit application code, configuration, other problems, authority or governance. No prior operator approval or installed release verification is asserted.

## 1. Problem

The feature-draft flow pushes working files under the configured drafts root, then compiles them into `memory/prd/<slug>/` and `memory/design/<slug>/` while preparing the lock. The earlier first-PR scope lists only documentation paths, so its pre-push attribution gate refuses an otherwise valid Argus draft round. Preparation also rebases the draft onto current mainline. Since `Argus-Bind` includes the commit's parent, replaying an old stamp without refreshing it makes the rebased commit underivable. Both failures block the ordinary single-user draft path.

Native reconciliation also captures the required canonical problem and delivery ledgers into lossless Document JSON/body pairs under `memory/problem/<slug>/`. In the installed v0.1.5 AEC drive, those same-slug records pass memory validation but fail the first-PR attribution gate, preventing normal push and preparation (issue 1439; AEC PR 6, E-62). That namespace is the active problem's own context, not another problem or a policy grant.

After a delivery ledger has merged, the scope resolver stops using the first-proposal grammar and derives only exact paths from past journal effects. AEC company-memory's contract is genuinely locked, and its six-task plan passes native fold, verify, coverage and render. Nevertheless, the installed v0.1.6 shared scope predicate denies its new numbered plan source/JSON and subsequent native problem captures. The actual refusal is retained at AEC delivery E-67..E-69. Past exact effects cannot name future artifact identities, and a candidate ledger cannot grant itself new paths. This blocks the documented transition from contract to plan.

The predecessor also describes a supervisor-owned, drive-inaccessible commit surface. In the current Pyxis/Argus workflow, the drive can invoke Git and set its environment. That isolation guarantee is not implemented and must not be presented as a property of these stamps.

## 2. Decision

**Decision:** Keep the content-bound `Argus-Drive`/`Argus-Bind` pair on every drive-authored commit. Add only the active slug's working draft and compiled PRD/design and problem record subtrees to the author-independent first-PR grammar. During `draft prepare-lock`, refresh bindings on replayed commits that already carry a valid stamp, retain unstamped commits as unstamped, and leave the Pyxis-generated lock commit unstamped. For an already committed valid delivery ledger, additionally admit only that problem's numbered plan Markdown/JSON and native problem captures. The planning-entry precheck still requires the actual binding lock and current readiness before drafting, freezing or projecting a plan. Treat stamps as auditable authorship evidence in the current single-user workflow; exact-head approval by the configured principal under trusted-mainline policy and host protection governs merge.

## 3. The design

### 3.1 First-PR path grammar

**Decision:** `ResolveFirstPR` admits the configured drafts root plus `/<slug>/`, and the fixed `memory/prd/<slug>/`, `memory/design/<slug>/`, and `memory/problem/<slug>/` subtrees, in addition to its existing slug-derived documents and fixed named allowlist. A different slug, a code path, an authority contract, or a governance document remains outside the drive's permitted scope. The scope never consults a same-PR ledger entry or author-provided path list. The versioned boundary policy changes when this grammar changes.

The added problem prefix supports the native writer's existing record paths; it neither moves generated files nor expands authority. The fixed active-slug calculation and protected-path checks remain the same. Boundary data version `argus-boundary/v5` identifies this grammar and the downstream extension in §3.3. Positive coverage includes native problem Document JSON/body pairs and both ledgers' captures. Negative coverage includes other slugs, exact-prefix lookalikes, traversal, code, authority and governance.

### 3.2 Rebase and generated commit

**Decision:** Before rebase, `draft prepare-lock` compares each source stamp to its original commit tree and parents; a stale or replayed binding refuses preparation. It then runs its temporary-worktree rebase with an explicit rebase-stamp context and an empty ambient drive slug. The prepare-commit-msg hook reads the replayed commit's final trailer block: a complete `Argus-Drive` plus well-formed `Argus-Bind` supplies the existing slug, and the hook replaces the binding using the new staged tree and parent. An unstamped source commit stays unstamped. A partial or malformed stamp aborts the rebase. This operation refreshes an existing audit signal; it does not assign a new drive to an unstamped commit. The generated compilation commit runs with the rebase context and drive slug cleared, even if the caller exported them. In an installed repository, the temporary worktree shares the original checkout's ignored `.pyxis/bin` cache so its pinned hook tool remains available offline.

The hook's normal per-commit mode still requires the drive author to supply `ARGUS_DRIVE_SLUG=<slug>` for each Git commit. The guide checks the resulting trailers and uses reset-and-recommit to repair an unpushed omission. Rebase is limited to single-parent draft commits because the binding producer computes one parent.

The guide sends stamped draft rebases through `draft prepare-lock`, which verifies source bindings before it enables refresh. It does not offer the raw rebase hook mode as a manual repair command.

### 3.3 Attribution gate carried forward

**Decision:** The gate still reads commit metadata from Git without a code-host credential, compares each carried binding to the commit's tree and parents, and resolves protected-path diffs as positively Argus, positively human, or underivable. A positively Argus protected-path diff is refused; an underivable protected-path diff fails closed; a positive human attestation can admit a human diff but cannot launder a positively Argus diff. The authority-contract and governance classes remain fixed. After the first PR, the valid committed delivery ledger still supplies its exact historically owned paths and own problem subtree. It also admits the configured plans root plus `/<slug>/NNN-plan.md` and `NNN-plan.json`, where NNN is exactly three decimal digits, and the fixed `memory/problem/<slug>/` namespace already admitted during birth. No candidate path list, journal claim or title changes this grammar. The committed input is the active problem's `delivery.md` (`delivery-ledger/v1`), not its `problem.md` (`problem-ledger/v1`). A valid problem ledger alone grants no downstream plan scope. Both consumers refuse the new numbered plans when the committed delivery ledger is absent or malformed, even if a valid delivery ledger appears only in the candidate. With absent committed delivery input, the attribution gate retains its first-proposal grammar, including birth-time problem captures but excluding plans; malformed committed delivery input fails closed. Relocating the configured plans root moves the grammar instead of retaining the default location. The same pure resolver supplies both the write hook and merge-attribution gate.

This scope is permission to author those bounded artifacts, not contract acceptance or planning readiness. `plangate precheck` remains required before drafting/freezing/projection and verifies the latest binding lock against current mainline. A missing, corrupt, stale or unmerged lock still refuses that stage. Plan folds, citation validation and byte comparison still gate content. Arbitrary Markdown, nested paths, code files and plan files for another slug remain outside the added grammar. Configuration remains outside it; enabling repository execution controls remains an independently reviewed setup act. First-proposal scope does not admit plans, and all protected classes are evaluated before scope ownership. The extension supplies no new action-class grant, operator attestation or application acceptance.

While Pyxis moves its own contracts to memory, its docgate invocation skips legacy document checks but still runs this attribution check on a branch with a merge base. Installed repositories retain their full document gate.

The Serves header carries forward every active predecessor target and replaces retired FR-12 with FR-14, so supersession does not drop the gate's existing requirement coverage.

### 3.4 Single-user assurance

**Decision:** In the single-user shared-account workflow, a drive that can run Git can also omit the environment slug or alter local hooks. The stamp and its binding make honestly labeled commits checkable and detect accidental stale bindings or replay. They are not an adversarial isolation boundary against that drive. Human review of the exact prepared head, Argus attribution, memory, and code gates remain merge controls. A stronger supervisor-owned commit path or operator-held signing key requires a separate design and deployment. This change adds no key, credential, network call, or privileged service.

### 3.5 Security & privacy

The commit hook and gate read local Git state only. The rebase context is an
environment flag for local stamp refresh, not an authentication token. The
shared-account limit in §3.4 applies to every hook and stamp verdict. No new
identity, secret, network, or external service is introduced.

### 3.6 Observability

The hook reports malformed stamps as commit failures; `prepare-lock` wraps
rebase errors with the named step. The attribution gate reports refused paths
and their protected class. No new diagnostic stream is required.

### 3.7 Invariants

- **DINV-1** — First-PR draft and compiled-record scope is a fixed function of configuration and the active slug, with no code path or other slug admitted.
- **DINV-2** — A rebased stamped commit carries a binding for its new tree and parent; an unstamped source stays unstamped; malformed, stale, or replayed source stamps abort.
- **DINV-3** — The generated lock commit has no Argus stamp, regardless of the caller's drive environment.
- **DINV-4** — The attribution gate's positive Argus precedence, protected-path refusal, positive human path, and underivable fail-closed behavior remain in force.
- **DINV-5** — A valid committed delivery ledger adds only its own three-digit plan Markdown/JSON and native problem-capture namespace, using the configured plan root. Candidate ledger effects cannot widen this grammar. First-proposal scope still excludes plans; scope permission does not replace planning readiness.

## 4. Alternatives considered

| ID | Alternative | Reason declined |
|---|---|---|
| ALT-1 | Keep the old first-PR grammar and make the operator commit draft rounds | Adds a manual step to every ordinary draft round and leaves Argus's own authored commits without their required stamp. |
| ALT-2 | Rebase without refreshing the old binding | Parent changes invalidate the binding and may make protected-path authorship underivable. |
| ALT-3 | Claim the Git hook is outside the drive's reach | The single shared account lets the drive invoke Git and set the hook environment; the claim would be false. |
| ALT-4 | Have the operator commit each numbered plan and new capture | Technically feasible, but transfers ordinary Argus documentation authoring to the operator after every lock and on every new capture identity. It does not support the intended autonomous planning transition. |
| ALT-5 | Derive future plan scope from the candidate journal | Lets the drive grant itself arbitrary paths in the same proposal; violates the committed-input boundary. |
| ALT-6 | Admit the complete configured plan root or all memory records | Supports the transition but also admits other problems and unrelated records. The narrower slug and artifact grammar supplies the required paths. |

## 5. Risks

| ID | Risk | Impact | Mitigation | Verification / evidence |
|---|---|---|---|---|
| R1 | Scope accidentally admits another problem or code | Out-of-scope change passes attribution | Fixed slug-specific prefixes; protected classes still checked first | Boundary and Git-backed gate tests (A-1) |
| R2 | Rebase copies or launders a stale binding, or stamps a human commit | Wrong authorship verdict | Preparation verifies original bindings; hook refreshes only a complete valid pair; generated commit clears context | Real-hook rebase and prepare-lock tests (A-2, A-3) |
| R3 | Audit stamp is mistaken for a security principal | Misleading merge decision | State shared-account limit in guide and design; operator reviews exact head | Guide and PR review (A-4) |
| R4 | Scope permission is mistaken for an accepted plan or contract lock | Planning starts from an unaccepted basis | Keep the separate native planning-entry precheck, lock validation and plan gates; document their distinct purpose | Planning-entry missing/corrupt/unmerged-lock refusal tests and bounded scope tests (A-6) |

## 6. Acceptance

| ID | Acceptance criterion | Verify | Required evidence | Traces to |
|---|---|---|---|---|
| A-1 | A stamped first-PR draft round and its compiled records pass attribution; other-slug and code paths remain out of scope | Test | Boundary and Git-backed attribution results | §3.1 · #DINV-1 |
| A-2 | After mainline advances, preparing a stamped draft yields a new valid binding; a source human commit remains unstamped; partial or stale source stamps abort; the installed hook reuses its pinned cache offline | Test | Real Git hook rebase and refusal results | §3.2 · #DINV-2 |
| A-3 | The generated lock commit remains unstamped even when the caller has a drive slug in its environment | Test | Prepare-lock integration result | §3.2 · #DINV-3 |
| A-4 | The guide names the per-commit context, repair path, and shared-account limit without promising inaccessible hooks | Inspection | Guide diff and operator review | §3.4 · #DINV-4 |

| A-5 | Native captures of the active problem and delivery ledgers pass the normal first-PR attribution path; the same source remains refused by the old installed grammar | Test + Failure injection | Failing-before/fixed-after boundary and real Git attribution fixtures; installed v0.1.5 refusal and unreleased candidate docgate pass at AEC source 4a541b1135d194f9dbbca435d6c83677d481d6b9 | §3.1 · #DINV-1 · prd:argus-boundary-enforcement/FR-9 |

| A-6 | A valid committed delivery ledger admits only its numbered plans and subsequent native problem captures through both canonical scope consumers; a valid problem ledger with absent/malformed committed delivery input still grants no numbered plans, and another slug, traversal, arbitrary/nested/code paths and protected classes remain refused; planning without a binding lock remains refused | Test + failure injection | Failing-before/fixed-after pure and Git-backed attribution fixtures, real Git write-hook fixtures, relocated-root cases and existing planning-entry refusal tests | §3.3 · #DINV-5 · prd:argus-boundary-enforcement/FR-9 · prd:argus-boundary-enforcement/FR-10 |

## 7. Open questions

None for the proposed bounded documentation transition. Native capture of additional standalone brief sources is a separately verified producer/ownership concern; no generic memory/source namespace is admitted by this proposal. The retained shared-account audit boundary supplies no stronger isolation guarantee.

## Principles

- **AP-6 — honored:** The producer and gate share trailer keys and binding derivation.
- **AP-7 — honored:** Preparation rebases a draft before lock; merged history is not rewritten.
- **AP-13 — honored:** Exact in-tree commit and prepared-head evidence governs review.
- **AP-17 — honored:** Malformed stamps abort rebase, and underivable protected paths fail closed.
- **AP-25 — honored:** The hook delegates to the same Go stamper for supported CLI surfaces.
- **AP-26 — honored:** Attribution reads committed Git metadata, not a mutable badge.
- **Deviation:** The predecessor's drive-inaccessible supervisor guarantee is withdrawn for this single-user workflow because that isolation does not exist. §3.4 states the resulting assurance.
