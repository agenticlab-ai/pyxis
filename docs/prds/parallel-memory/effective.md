# Independent parallel memory changes — PRD *(effective view)*

---

**GENERATED — DO NOT EDIT.** Sole writer: `docgate fold`. As of: **000**. Content fingerprint: `sha256:ec36a1dad75e9d7cd6507cd0cfd045cbcdfbfd836dc1f7a62637977eb8fbff41`. Tool version (recorded, never hashed): `docgate/v1`.

---


| Field | Value |
|---|---|
| **In one line** | Independent changes can preserve their memory records and accepted-debt policy through integration without artificial shared-file edits. |
| **Relevant-when** | planning, implementing, or reviewing reconciliation debt policy and memory admission; investigating conflicts or record reissuance across parallel proposals |
| **Status** | Draft — contract review pending; problem and technical direction approved by the operator on 2026-09-17 |
| **Tier** | T3 — merge controls, immutable history, and accepted-debt migration |
| **Owner** | @mdarabi |
| **Approver(s)** | @mdarabi via the introducing proposal's operator review |
| **Contributors** | Codex — source audit, contract synthesis, and automated review |
| **Informed** | Pyxis memory maintainers and parallel proposal authors |
| **Date** | 2026-09-17 |
| **Appetite** | Bounded to debt-policy contention and unrelated record-age refusals using existing memory machinery; operator attention covers contract review and delivery acceptance; no new service, dependency, merge policy, or provenance redesign. No monetary or token cap was supplied. |
| **Target release** | Reader compatibility before explicit policy cutover; causal admission can ship independently after its safety checks pass |
| **Tracking** | Operator request of 2026-09-16 and approval to continue on 2026-09-17; no external work item yet |
| **Artifacts** | Source/history audit at commit 8b9c79c246ef025f981155aca93b567fe513b663; generated requirement projection and introducing review evidence |
| **Template version** | prd-template/v2 |
| **Design coverage** | requirements-first/v1 |

## 0. Read to rule — the human reviewer's section (~4 min)

*The body is the contract; this section only routes attention — on any conflict the body governs; machines ignore it.*

**Judge these**

| # | The call | Could have gone | If you disagree |
|---|---|---|---|
| 1 | Preserve exact accepted debt and existing merge safeguards. | Relax policy to reduce merge work. | Change the trust boundary → FR-3, NR-1, NR-2. |
| 2 | Independent work needs no shared baseline refresh or record reissuance. | Automate repeated refreshes and reissuance. | Retain integration churn → FR-1, FR-2, FR-6. |
| 3 | Ship compatibility before a separate explicit migration. | Change policy and readers together. | Change rollout and recovery scope → FR-4, §6.5. |

**Sanity-check the numbers** — zero baseline edits for unchanged findings; zero record-byte changes due only to unrelated chronology; ten siblings, 45 unordered pairs exercised in both integration orders → M1, M2, FR-1, FR-6.

**The author's open questions** — formal review of this requirement contract remains pending; the approved technical direction is unchanged → Q1.

**Skip the rest** — structure and traceability → docgate; substance → prd-review; behavior and regression proof → §5.8.

## 1. Summary — the one-pager

Parallel Pyxis authors currently compete over a memory baseline even when
their actual records and source changes are disjoint. A branch can also need
new record identities after unrelated, newer records reach its base. Both
behaviors consume integration effort without adding knowledge.

The operator asked to remove this avoidable contention and approved the
technical direction on 2026-09-17. This contract makes that existing scope
available to the planner; it adds no new capability beyond that scope.
Success means independent changes integrate with zero shared baseline edits
and unchanged candidate record bytes. Exact debt controls, causal relations,
immutable history, and historical-effect protections remain release guards.
The principal risks are granting new debt during migration, reviving an old
invalid act, and mistaking genuinely overlapping work for independent work.

## 2. Problem space

### 2.1 Problem statement

Authors of unrelated proposals repeatedly reconcile a shared memory artifact
and may recreate immutable records after rebasing. The overhead increases
with parallel work, although the information contributed by each proposal
does not overlap. A clean textual merge alone is insufficient if a later gate
still demands record reissuance or whole-repository inventory refreshes.

### 2.2 Users and segments

| Segment | Role and context | Domain exp. | Tech exp. | Key goals | Pain today | Evidence |
|---|---|---|---|---|---|---|
| **P1 (primary)** | Human or agent author integrating an independent proposal | journeyman | journeyman | Preserve valid work through integration | Shared baseline churn and chronology refusals | Operator request and source audit, §2.4 |
| P2 (secondary) | Maintainer reviewing memory integrity | master | master | Verify debt and history without weakening gates | Must distinguish real conflicts from artificial contention | Existing reconciliation and packet safeguards |
| **Anti-segment** | Authors concurrently replacing the same exclusive claim or code unit | — | — | — | Real coordination remains necessary | NG-1 |

### 2.3 Jobs and desired outcomes

| # | When… (situation) | I want to… (motivation) | So I can… (outcome) | Today's workaround | Evidence |
|---|---|---|---|---|---|
| J1 | Another unrelated proposal lands first | Integrate my existing contribution unchanged | Continue without memory-only rework | Refresh the baseline and sometimes reissue records | §2.4 |
| J2 | My change resolves accepted debt | Record just that reduction | Avoid touching another author's debt policy | Rewrite the shared inventory | §2.4 |
| J3 | Memory validation changes | Prove existing trust rules survive | Adopt the improvement without granting extra authority | Source inspection and existing adversarial fixtures | §2.4 |

### 2.4 Evidence

| Claim | Source (research, data, tickets, interviews) | Confidence |
|---|---|---|
| Routine unrelated record additions touch a shared baseline | At inspected commit 8b9c79c246ef025f981155aca93b567fe513b663, all ten preceding first-parent PR commits changed the baseline; they added 171 distinct memory paths with no repeated record path | High — dated Git audit; overlap evidence, not a measured manual-conflict rate |
| Unrelated newer records can invalidate an older candidate after rebase | Source audit of writer admission and packet comparison at the same commit; current tests explicitly refuse an older relationless record | High — executable rules and fixtures |
| Relaxing chronology must preserve historical-act safety | Existing packet fixtures cover a backdated document activating a rejected revocation and supplied targets activating dangling acts | High — exact counterexamples already retained |
| The operator wants the proposed remedy pursued | Request to develop the proposal, followed by “the design is approved. move on” on 2026-09-17 | High — direct operator instruction |

### 2.5 The journey today

| Phase | User does | User feels / pain | Frontstage (what user sees) | Backstage (systems, people, processes) |
|---|---|---|---|---|
| Author | Adds source and explanatory memory | Work itself is independent | Separate source and record files | Baseline identities still cover the whole tree |
| Integrate | Rebases after another proposal | Repeats work unrelated to intent | Baseline differences or age refusal | Global identity and chronological comparisons |
| Verify | Rebuilds inventory and reissues affected records | Must revalidate changed identities | Gate passes only after churn | Exact checks are valid, but coupled to shared artifacts |

### 2.6 Opportunity and why now

The affected audience is concurrent proposal authors; frequency depends on
the actual proposal stream. The inspected ten-proposal sample establishes a
shared-file hotspot, not a production failure-rate estimate. Continuing the
current process spends the parallelism required by vision §12. The existing
immutable records already provide independent paths, making this a bounded
repair of surrounding validation rather than a new memory product.

**Decision:** The problem and remedy direction were accepted in the operator's
2026-09-17 approval. The requirement document itself remains a draft until
its review is recorded; approval of that document is not asserted here.

| Reviewer | Role | Date | Verdict |
|---|---|---|---|
| @mdarabi | Operator and affected author | 2026-09-17 | Problem accepted through the proposal conversation; this predates this document's review surface, so no review-duration measurement is claimed |

## 3. Goals, metrics, and guardrails

### 3.1 Goals as falsifiable hypotheses

| # | We believe that… | Will result in… | Measured by… |
|---|---|---|---|
| G1 | Removing artificial shared memory maintenance preserves parallel authorship | Independent proposals integrate without memory-only edits | M1 and M2 |
| G2 | Preserving exact trust checks permits safe adoption | Existing invalid transitions remain refused | M3 |

### 3.2 Goals → signals → metrics

| Metric | Goal | Signal | Baseline | Target (at checkpoint) | Feeds from |
|---|---|---|---|---|---|
| M1 (primary) | G1 | Intersection of baseline paths changed by independent siblings | One shared baseline in all ten inspected proposals | Zero shared policy paths across ten sibling fixtures and all 45 pairs in both integration orders before cutover | EVT-1 |
| M2 | G1 | Candidate record bytes or IDs changed solely after unrelated integration | Current age-floor fixture refuses an older relationless record | Zero changed candidate bytes/IDs in both issuance orders, merge and squash fixtures before causal rollout | EVT-1 |
| M3 | G2 | Invalid transition accepted in the regression matrix | Existing refusal fixtures | Zero accepted invalid transitions at each rollout checkpoint | EVT-1 |

### 3.3 Guardrails and decision rule

| Guardrail metric | Threshold | Action if breached |
|---|---|---|
| Accepted-debt widening, lost retirement checks, or changed trusted records | Zero occurrences | Stop rollout; preserve prior policy and evidence; fix and rerun the exact failing fixture |
| Historical invalid act made effective by a candidate | Zero occurrences | Stop causal rollout and retain the historical guard |
| Unrelated dirty files changed by synchronization | Zero files | Stop writer rollout and repair its preflight |

**Ship rule:** Each increment advances only when its functional and safety
matrix passes. Combined completion additionally requires M1 and M2 together.
Runtime measurements are advisory under AP-29 and never substitute for these
correctness checks.

### 3.4 Non-goals and waiting room

| ID | Non-goal | Why refused |
|---|---|---|
| NG-1 | Automatically compose competing claims, code anchors, or changes to the same allowance | Those changes are not independent; authority conflicts remain visible |
| NG-2 | Change merge authority, require serialized proposals, or change squash support | Avoidable memory contention is the problem; merge governance remains intact |
| NG-3 | Redesign provenance retention, code-unit granularity, record identity, or other document indexes | Separate decisions with separate acceptance evidence |
| NG-4 | Add a service, model dependency, shared issuance counter, or success ledger | Deterministic existing repository machinery suffices |

| ID | Waiting room | Revisit when / intended version |
|---|---|---|
| WR-1 | Broader memory coordination improvements | A measured conflict remains outside the disjoint unit/family/address boundary after this change |

## 4. Solution space

### 4.1 Approach and alternatives considered

Independent source work retains independently editable memory obligations.
Validation derives current evidence, retains exact accepted-debt authority,
and judges candidate records by their actual relations and effects. An
explicit compatibility transition preserves existing accepted policy.

| Option | Sketch | Why not chosen |
|---|---|---|
| Independent maintenance with preserved trust checks | The approved direction; only the actual affected obligations need updates | Chosen |
| Automate refresh and record reissuance | Reduce manual commands while keeping shared maintenance | Still changes unrelated artifacts and record identities |
| Serialize proposals | Avoid concurrent integration attempts | Spends parallelism instead of removing contention |
| Relax accepted-debt or history checks | Make more candidates pass | Violates G2 and the operator-approved safety boundary |
| Do nothing | Keep current workflow | Retains J1/J2 rework |

### 4.2 Scope perimeter

In scope: independent debt maintenance, preserved record identity after
unrelated integration, explicit policy migration, shared admission/gate
checks, diagnostics, and reproducible acceptance evidence. Future or excluded
work is named in NG-1 through NG-4 and WR-1; none is implicitly authorized.

### 4.3 Experience: flows and states

| Flow | User does | System responds | States covered |
|---|---|---|---|
| F1 — independent integration (J1) | Integrates another independent proposal and validates | Preserves existing candidate records and unchanged policy | Valid, real conflict, missing input, invalid history |
| F2 — debt reduction (J2) | Commits a repair and explicitly synchronizes its policy | Previews and applies only authorized policy changes | Unchanged, reduction, regression, interrupted/retry, dirty refusal |
| F3 — migration (J3) | Requests migration after reader compatibility is available | Proves exact trusted input and allowance preservation | Valid legacy policy, malformed/unsupported policy, failed replay, interrupted/retry |

Empty policy means zero accepted debt only when its format is valid. A missing
required policy or predecessor is an error, not a first-use success. Validation
is synchronous; partial output names completed work and never certifies an
incomplete result. Errors preserve unrelated files and identify a remedy.
Existing CLI text and machine output remain the interface; there is no new
visual accessibility or localization surface.

### 4.4 Scenarios

Two authors add memory for disjoint units. The later-issued record lands first.
The other author rebases and keeps the original record bytes; unchanged debt
requires no policy edit. Separately, two authors resolve different debt
addresses and each updates only the affected policy. By contrast, supplying
an old invalid act's missing prerequisite still refuses with that act named.

**Decision:** The operator approved the technical direction on 2026-09-17.
Formal acceptance of this requirement contract is pending; the review must
check that it faithfully records that scope and introduces no new promises.

| Reviewer | Role | Date | Verdict |
|---|---|---|---|
| @mdarabi | Operator | pending | Requirement-contract review pending; existing design approval remains recorded separately |

## 5. System requirements

### 5.1 Functional requirements

Must priority is ranked by row order; safety rows retain critical risk even
when their functional rank follows a user outcome. §6.5 supplies the safety
plan for every high/critical row.

| ID | Requirement (EARS) | Rationale | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|
| FR-1 | When an otherwise valid source or memory change leaves reconciliation findings unchanged, the system shall validate the change without requiring baseline-policy edits. | Remove routine shared maintenance | Must | critical | Ten disjoint sibling fixtures; all 45 unordered pairs in both integration orders pass with zero baseline edits | Test | G1, J1, F1 |
| FR-2 | When proposals reduce debt at disjoint finding addresses, the system shall confine each proposal's policy changes to its affected addresses. | Preserve independent reduction work | Must | critical | Changed-policy path intersections are empty for disjoint reductions; same-address disagreement remains visible | Test | G1, J2, F2 |
| FR-3 | When validating candidate debt policy, the system shall enforce an exact current inventory under the trusted base's existing monotonic rules and recognized one-way phase advances. | Preserve accepted-debt authority | Must | critical | Exact matches, reductions, and existing phase advances pass; new addresses outside those advances, increased counts/severity, forbidden repairability changes, reversal, and reintroduced retired debt refuse | Test | G2, J3, F1, F2 |
| FR-4 | When explicitly migrating accepted policy after reader compatibility is available, the system shall preserve its exact normalized allowance inventory with reproduced legacy input evidence. | Prevent migration from granting authority | Must | critical | Before/after inventories are identical; changed allowances or unavailable replay refuse; migration changes policy representation only | Test | G2, J3, F3 |
| FR-5 | When explicitly synchronizing policy, the system shall apply only a fully preflighted authorized write set against pinned repository inputs. | Protect unrelated work and permit recovery | Must | critical | Dirty/unowned input and moved-HEAD fixtures write zero paths; valid retry converges; a completed retry writes zero paths | Test | G1, G2, J2, F2, F3 |
| FR-6 | When integrating unrelated records, the system shall preserve otherwise valid candidate record identities, bytes, timestamps, and references irrespective of their order relative to unrelated records. | Remove reissuance on unrelated rebase | Must | critical | Both issuance/integration orders, merge and squash, and fresh mainline-only transport clones preserve exact candidate bytes/IDs | Test | G1, J1, F1 |
| FR-7 | When admitting or merge-checking a candidate record, the system shall enforce the same causal and historical-effect constraints before accepting its effects. | Prevent writer/gate disagreement | Must | critical | Every writer entry and packet path accepts valid independent cases and refuses explicit causal defects or activation of prior rejected/dangling acts | Test | G2, J3, F1 |
| FR-8 | When equivalent pinned inputs are checked locally or in hosted gate modes, the system shall produce equivalent debt and admission classifications. | Keep gate authority consistent | Must | critical | Local, PR, push, and scheduled fixture matrix agrees for equivalent inputs; snapshot-only output makes no historical-transition certification | Test | G2, J3, F1, F3 |
| FR-9 | When validation or synchronization reports a result, the system shall identify its input scope and the affected debt addresses or record relations with a specific remedy for refusal. | Make residual conflicts actionable | Must | high | Human/JSON fixture results identify pinned inputs, actual defects, policy changes or historical carriers without exposing document bodies or credentials | Test | G1, G2, J1, J2, J3 |

| ID | Existing behavior protected | Why protected | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|
| NR-1 | Code transitions, authored-source retirement, record evidence, and strict closeout retain existing checks | Policy representation cannot bypass semantic obligations | critical | Existing changed-code, stale-anchor, source-deletion, missing-evidence, and closeout negative fixtures still refuse through the new policy path | Test | G2, F2, F3 |
| NR-2 | Record immutability, identity collisions, moved heads, competing claims/answers/revocations, and relation ordering remain protected | Independent work must not authorize conflicting work | critical | Exact existing malicious fixtures retain refusal; valid-history projections remain identical | Test | G2, F1 |
| NR-3 | Existing-format debt policy and validation retain their behavior until explicit policy migration | Reader deployment must not silently cut over policy; causal admission is staged independently | critical | Existing-format debt-policy fixtures remain unchanged; old readers refuse the new format; rollback compatibility is rehearsed; the separate causal rollout may change age-floor expectations before policy migration | Test | G2, F3 |

### 5.2 Unwanted behavior and failure handling

| ID | If… (trigger) | Then the system shall… | User sees | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|---|
| UB-1 | If policy is malformed, missing after adoption, mixed, irregular, or uses an unsupported evaluator | Refuse without granting substitute policy | Named format/input refusal | Must | critical | Unknown keys/versions, duplicates, wrong address identity, unexpected entries, symlinks/submodules, and missing marker all fail | Test | FR-3, FR-4, F3 |
| UB-2 | If a candidate widens authority or redirects policy lineage | Refuse the candidate | Address and regression reason | Must | critical | Added debt, wrong trusted base, unauthorized root redirection, and resurrection after reduction fail | Test | FR-3, F2 |
| UB-3 | If policy synchronization is interrupted | Report completed/planned work and permit only an exact authorized retry | Explicit incomplete outcome | Must | critical | Injected interruption never certifies a partial policy; retry repairs only planned paths and converges | Test | FR-5, F2, F3 |
| UB-4 | If a candidate would make a previously rejected or dangling historical act effective | Refuse the reinterpretation while preserving valid explicit retirement as the repair path | Carrier, relation, and prerequisite context | Must | critical | Exact backdated-document/revocation and dangling-target fixtures refuse even when another defect is present; a different rejection reason is not mistaken for activation | Test | FR-7, F1 |
| UB-5 | If single-record admission fails validation | Preserve the pre-admission record/body files | Causal, loader, projection, or history refusal | Must | critical | Add, revise, ask, answer, revoke, reconciliation, and closeout refuse without writing the rejected record/body; earlier valid batch appends are reported accurately | Test | FR-7, F1 |
| UB-6 | If required pinned history or evidence is unavailable | Refuse transition certification | Missing-input remedy and snapshot/transition scope | Must | critical | Missing base/replay fixtures fail; no ambient-ref, model, or network fallback supplies authority | Test | FR-4, FR-8, F1, F3 |

### 5.3 Permissions and abuse cases

Existing repository write and human merge authority remain unchanged. Local
validation grants no merge permission. A candidate's own files never establish
the authority against which that candidate is checked.

| ID | Actor and intent | Vector | Impact | Severity | Mitigated by | Verify |
|---|---|---|---|---|---|---|
| ABUSE-1 | Candidate author hides new debt | Alter policy or select a weaker lineage | Unauthorized acceptance | critical | FR-3, UB-1, UB-2 | Test |
| ABUSE-2 | Candidate author changes prior meaning | Insert an older prerequisite for a rejected act | Retroactive effect | critical | FR-7, UB-4, NR-2 | Test |
| ABUSE-3 | Writer touches another author's work | Dirty inputs or path redirection during retry | Lost unrelated changes | high | FR-5, UB-3 | Test |

### 5.4 Data

| Entity / field | Meaning | Source of truth | Retention | Integrity rules | Privacy class |
|---|---|---|---|---|---|
| Accepted debt | Previously authorized reconciliation findings | Trusted repository policy and retained history | Policy reductions and prior versions retained in Git | Exact inventory and monotonic transition | repository-internal |
| Memory records | Immutable claims and acts with stable identity | Existing record store | Existing retention rules | Causality, immutable bytes, historical-effect checks | existing classification unchanged |
| Validation result | Evidence about one invocation | Pinned inputs and derived output | Failures/judgments in owning artifacts; routine diagnostics ignored | Deterministic, scoped, no tracked success receipt | paths and digests; no new body disclosure |

Migration preserves accepted authority exactly. Compatible readers precede
data-only cutover. Rollback keeps the ability to read already adopted policy;
reverting its representation requires a separately reviewed exact reverse
migration. Accepted older independent records remain unchanged during rollback.
No new personal-data category, profiling, external processor, or network egress
is introduced; a new DPIA is not applicable to this local representation and
validation change. Existing repository privacy controls continue to apply.

### 5.5 Interfaces and integrations

| Interface | Direction | Purpose | Contract (format, ranges, timing) | Failure modes | Owner |
|---|---|---|---|---|---|
| Existing memory CLI and shared gate | both | Read, validate, and explicitly synchronize repository memory policy | Existing typed CLI/report conventions with versioned output evolution | Specific nonzero refusals; zero implicit writes | Memory maintainer |
| Local Git history | input | Supply pinned trusted and candidate evidence | Exact retained commits, paths, modes, and bytes | Missing or irregular evidence refuses; no fetch | Memory maintainer |

### 5.6 Non-functional requirements

| ID | Area | Requirement | Fit criterion | Verify |
|---|---|---|---|---|
| NFR-1 | Determinism | The system shall derive validation and synchronization results deterministically from declared inputs. | Fixed-input output bytes agree under shuffled input enumeration; no ambient clock/randomness, AI, or network determines a verdict | Test |
| NFR-2 | Work per check | The policy reader shall batch member reads for each pinned tree. | Operation counters show one batch policy read per pinned tree rather than one Git subprocess per member; elapsed time is advisory | Test |
| NFR-3 | Privacy | The new result fields shall expose input identities and defect context without introducing document-body or credential disclosure. | Output fixtures contain required paths/digests and no fixture secret/body payload | Test |

### 5.7 Instrumentation — the tracking plan

| ID | Event name | Fires when | Properties (typed) | Source | Feeds |
|---|---|---|---|---|---|
| EVT-1 | memory_acceptance_result | Acceptance harness completes a branch-order or adversarial case | case: string, pinned inputs: strings, changed paths: string[], record hashes: string[], refusal codes: string[], accepted: bool | Existing test/report evidence; no new runtime event service | M1, M2, M3 |

Required evidence travels with the implementing proposal. Failures and review
judgments are retained in their owning shared artifacts. Routine gate passes
create no new tracked record, counter, or mutable metric file.

### 5.8 Definition of Done

| ID | Done criterion | Source | Verify | Required evidence | Blocking? |
|---|---|---|---|---|---|
| DOD-1 | Independent policy maintenance and record preservation satisfy their fit criteria together | FR-1, FR-2, FR-6, M1, M2 | Test | Real-Git branch-pair results, changed-path manifests, before/after record digests, fresh transport-clone results | yes |
| DOD-2 | Accepted-debt validation, migration, and writer recovery satisfy every positive and negative case | FR-3, FR-4, FR-5, UB-1, UB-2, UB-3 | Test | Exact migration inventories, refusal evidence, interruption/retry/no-op manifests | yes |
| DOD-3 | Every admission entry and gate mode preserves causal/history safety | FR-7, FR-8, UB-4, UB-5, UB-6, M3 | Test | Writer matrix, exact malicious fixtures, mode parity, snapshot/transition scope evidence | yes |
| DOD-4 | Adjacent integrity behavior and deterministic/private output remain protected | NR-1, NR-2, NR-3, FR-9, NFR-1, NFR-2, NFR-3 | Test + Inspection | Existing regression suite, canonical output comparisons, operation counters, output fixtures | yes |
| DOD-5 | Staged adoption and recovery are executable and documented | §6.4, §6.5, §6.6 | Test + Inspection | Compatibility and rollback rehearsal results, CLI documentation, implementation review, required shared-gate results for the proposed revision | yes |

### 5.9 AI behavior and evals

Skipped: no model-driven behavior is introduced; deterministic tests prove
the relevant behavior.

## 6. Delivery

### 6.1 Phasing and milestones

| Phase | Purpose | Scope | Exit criterion |
|---|---|---|---|
| Compatible validation | Preserve behavior while introducing new policy support and shared history checks | Reader/writer compatibility and causal guard | Relevant existing and new safety matrices pass |
| Independent admission | Remove unrelated age refusals after shared safety coverage exists | Causal change | FR-6/FR-7 and their adversarial/rollback fixtures pass |
| Policy cutover | Adopt independent policy only after compatible code lands | Separate explicit data migration | Exact inventory proof and combined DOD-1 through DOD-5 pass |

### 6.2 Dependencies

| Dependency | Type (team / system / vendor / approval) | Needed by | Status | Owner |
|---|---|---|---|---|
| Approved requirement ledger and merged serving designs | approval | Frozen implementation planning | Technical direction approved; this contract review and merges pending | @mdarabi |
| Existing memory/Git/shared-gate machinery | system | Implementation | Present in inspected source | Memory maintainer |
| Dual-format reader in retained mainline | system | Policy cutover | Implementation pending | Implementer |

### 6.3 Risks, assumptions, and rabbit holes

| # | Risk / assumption / rabbit hole | Likelihood | Impact | Mitigation or test | Owner |
|---|---|---|---|---|---|
| R1 | Migration accidentally grants or erases accepted debt | M | H | Exact trusted inventory proof and malformed/widening fixtures | Implementer |
| R2 | An older insertion makes a rejected historical act effective | M | H | Shared before/after predicate and exact existing malicious cases | Implementer |
| R3 | Independent filenames are mistaken for independent semantics | M | H | Contrast disjoint units/families/addresses with competing cases | Design owner |
| R4 | Partial policy output becomes authoritative | M | H | Pinned preflight, interrupted-output refusal, exact retry proof | Implementer |
| R5 | Runtime cost grows with member count | M | M | Batch-read operation evidence; record durations without a timing gate | Memory maintainer |

### 6.4 Release criteria

| Lens | Criterion | Evaluated by |
|---|---|---|
| Functionality | DOD-1 through DOD-4 pass at the combined checkpoint | Acceptance and shared gates |
| Usability | FR-9 output and documented commands distinguish independent work from real conflicts | CLI fixtures and documentation inspection |
| Reliability | UB-1 through UB-6 and recovery rehearsals pass | Failure-injection and real-Git tests |
| Performance | NFR-2 operation evidence is recorded; timings remain advisory | Operation counters and optional measurements |
| Supportability | Migration, retry, compatibility, and rollback commands are documented | Maintainer inspection |

### 6.5 Rollout and reversibility

This is a local repository capability with an explicit data-adoption boundary,
so percentage traffic flags do not apply. Policy format controls debt-policy
adoption; existing-format debt policy and validation keep their behavior until
explicit policy migration. Causal admission is a separate increment and may
ship against the existing policy format once its safety matrix passes.
All critical requirements use staged rollout and a rollback rehearsal.

Compatible readers/writers land before the data-only cutover. Before cutover,
code rollback preserves existing policy. After cutover, rollback retains the
compatible reader; reverse migration is a separate reviewed operation with
exact current-inventory proof. A causal rollback keeps the historical guard
and must accept legitimately landed records without changing their bytes.
Any §3.3 breach pauses advancement and retains evidence for remediation.

### 6.6 Launch readiness

| Area | Item | Owner | Status |
|---|---|---|---|
| Docs and support | CLI migration/retry/rollback and independence-boundary documentation | Implementer | pending implementation |
| Platform integrity | Exact revision passes required shared gates and implementation review | Implementer | pending implementation |
| Adoption | Compatible code landed before explicit policy cutover; recovery rehearsed | Memory maintainer | pending implementation |
| Privacy | No new data category or egress; output assertions cover NFR-3 | Implementer | pending validation |

Sales, billing, localization, and marketing have no changed surface in this
repository-maintenance capability.

**Decision:** Launch requires §6.4 and the applicable readiness rows to pass.

| Reviewer | Role | Date | Verdict |
|---|---|---|---|
| @mdarabi | Delivery acceptance | pending | No launch claim; implementation has not started |

### 6.7 Post-launch review

Trigger: both increments and the explicit policy cutover have landed, and
the combined acceptance matrix has been reproduced from retained mainline.
Owner: @mdarabi. Record adopt, iterate, or rollback with M1–M3 and actual
proposal evidence in the owning work item. Future contract changes use the
ledger's amendment process; they never rewrite this frozen base.

## 7. Alignment

### 7.1 FAQ

Independent changes means disjoint authored-source units, code units,
exclusive claim families, and debt addresses. Different files in one Go
package may still share a code anchor. Same-address and same-family changes
retain visible coordination. Squash support remains; redesigning evidence
retention is a separate concern. These boundaries implement vision §12 and
§13 while retaining AP-7, AP-11, AP-13, AP-24, AP-27, and AP-29.

### 7.2 Open questions

| # | Question | Owner | Needed by | Status |
|---|---|---|---|---|
| Q1 | Does this requirement ledger faithfully formalize the already approved scope? | @mdarabi | Requirement-contract sign-off before frozen planning | open — review pending; no new technical decision is requested |

### 7.3 Decision log

| Date | Decision | Alternatives considered | Why | Who |
|---|---|---|---|---|
| 2026-09-17 | Pursue independent debt maintenance and causal/history-based admission within existing machinery | Repeated regeneration/reissuance, serialization, relaxed integrity | Remove artificial contention while retaining trust boundaries | @mdarabi, explicit proposal approval in conversation |
| 2026-09-17 | Formalize one requirement contract serving the two separately revisable technical decisions | Leave only vision citations, or invent unrelated feature contracts | Planning precheck requires a feature ledger; one outcome has two implementation decisions | Codex, authoring within approved scope; contract review pending |

### 7.4 Changelog

| Date | Change | Author |
|---|---|---|
| 2026-09-17 | Drafted the requirement ledger from the operator's problem, source evidence, and approved scope after planning exposed the missing feature basis | Codex |

## 8. Contract readiness — the executability gate

| Check | Pass? | Notes |
|---|---|---|
| Problem basis accepted | yes | §2.6 attributes the existing operator decision without inventing a document review |
| Zero solution-blocking TBDs or open questions | pending | Q1 requires contract sign-off; technical direction is already approved |
| Every Must FR has fit and Verify | yes | FR-1 through FR-9 have executable criteria |
| Requirement risk and safety plans declared | yes | FR/NR/UB risk fields; §6.5 covers staged adoption and rollback |
| Boundaries and non-regression named | yes | NG-1 through NG-4; NR-1 through NR-3 |
| Primary jobs covered | yes | J1 by FR-1/FR-6; J2 by FR-2/FR-5; J3 by FR-3/FR-4/FR-7/FR-8 |
| Flow success/failure/non-regression coverage | yes | F1 through F3 map to FR/UB/NR rows |
| Metrics have a measurement plan | yes | EVT-1 supplies M1 through M3 from acceptance evidence |
| Blocking DoD rows are sourced | yes | DOD-1 through DOD-5 cite requirements and delivery criteria |
| Risks and assumptions have owners | yes | R1 through R5 |
| Human judgment has an owner and rubric | yes | Q1: @mdarabi checks faithful scope without new promises; delivery acceptance uses the supplied evidence |
| DoD is complete and evidence classes declared | yes | §5.8; concrete commands and test ownership belong in the plan |

## Superseded & retired

None — no unit has been superseded or retired; the ledger is its base.
