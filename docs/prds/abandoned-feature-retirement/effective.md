# Abandoned feature retirement — PRD *(effective view)*

---

**GENERATED — DO NOT EDIT.** Sole writer: `docgate fold`. As of: **000**. Content fingerprint: `sha256:34b935dbe9ceca4bddebf94f288d31c12d9f08b926a90b3c6bb37479c69e603f`. Tool version (recorded, never hashed): `docgate/v1`.

---


| Field | Value |
|---|---|
| **In one line** | Operators can remove an explicitly abandoned feature's implementation and active authoring surface while preserving its historical records and unrelated behavior. |
| **Relevant-when** | planning, building, or reviewing cleanup after a feature is abandoned; triaging unexpected source or code deletion; changing feature lifecycle, memory reconciliation, or repository-retention behavior |
| **Status** | Problem + solution gates signed (2026-09-21, via operator approval after PRD review) — launch review pending, post-implementation; merge freezes the base |
| **Tier** | T3 — destructive repository changes and factory-control evidence |
| **Owner** | @mdarabi |
| **Approver(s)** | @mdarabi via the introducing proposal's operator review |
| **Contributors** | Codex — source audit, requirements synthesis, and automated review |
| **Informed** | Pyxis maintainers and feature owners |
| **Date** | 2026-09-21 |
| **Appetite** | A bounded, deterministic, offline repository capability using existing Git, lifecycle, and memory machinery; one human judgment for abandonment and one reviewed authority publication before destructive cleanup. No external service, model inference, historical-record deletion, shared-artifact retirement, or automatic product cancellation. |
| **Target release** | Before cleanup of the abandoned E2E test-generation feature resumes |
| **Tracking** | Operator request and scope rulings of 2026-09-21; no external work item yet |
| **Artifacts** | Operator instruction to remove the E2E test-generation feature while preserving historical records and unrelated E2E tests; `prd:feature-implementation-drive/FR-29` |
| **Template version** | prd-template/v2 |
| **Design coverage** | requirements-first/v1 |

## 0. Read to rule — the human reviewer's section (~6 min)

*The body is the contract; this section only routes attention — on any conflict the body governs; machines ignore it.*

**Judge these**

| # | The call | Could have gone | If you disagree |
|---|---|---|---|
| 1 | Historical contracts, decisions, plans, reviews, incidents, and evidence remain byte-identical during retirement. | Delete every artifact carrying the abandoned feature's name. | Change the preservation boundary → FR-1, FR-10, NR-2. |
| 2 | Destructive cleanup begins only after the feature's live product contract and governing designs have a typed terminal disposition. | Treat an abandonment statement as sufficient while the feature remains required. | Permit contradictory current state → FR-2, UB-1. |
| 3 | Retirement authority lands before, and separately from, the deletion it authorizes. | Let the deletion proposal publish its own permission. | Accept candidate self-authorization → FR-7, UB-4. |
| 4 | Only explicit, exclusively owned mutable sources and code units can be retired. | Infer a perimeter from feature names or admit shared artifacts. | Widen destructive scope → FR-3, FR-4, UB-3. |
| 5 | Authority binds the exact current generation, including code-unit absence and restoration inside a persistent file. | Treat matching bytes or a stable file history as the same generation forever. | Permit stale authority reuse → FR-6, UB-5. |

**Sanity-check the numbers** — two landed proposals from authority to deletion; zero unexplained residual surface; zero protected or unrelated-test identities changed; zero unselected or shared identities deleted; zero baseline widening; an exact clean retry writes zero records → M1–M4, FR-5–FR-15.

**The author's open questions** — none — product behavior is closed; implementation mechanics belong to technical design → §7.2.

**Skip the rest** — structure and traceability → docgate; contract substance → prd-review; architecture and lineage algorithms → technical design; implementation defects → codegate and the acceptance corpus.

## 1. Summary — the one-pager

An operator can deliberately abandon a feature whose implementation has
already entered the repository. Today the integrity machinery permits an
authored source to disappear only after feature completion has been proven.
An abandoned feature cannot truthfully make that completion claim, leaving
the operator between retaining a rejected surface and weakening the evidence
gate that protects unrelated work.

This contract gives feature owners a separate destructive lifecycle. The
feature's current product obligation must first reach a typed terminal state.
The operator then proves a complete active-surface inventory and publishes
exact retirement authority while the selected implementation is intact; a
later proposal removes only those exclusively owned identities. Merged PRDs,
designs, brainstorms, plans, reviews,
incidents, and evidence remain as historical records, and unrelated behavior
continues unchanged.

The known pilot is the abandoned E2E test-generation surface. Success means
the pilot's selected active surface is absent while every protected historical
artifact and unrelated E2E test remains unchanged, with local, hosted, and
fresh-clone validation agreeing. Any protected or unselected deletion, live
contract, candidate-authored authority, stale authority, shared ownership, or
baseline widening stops the release.

## 2. Problem space

### 2.1 Problem statement

Feature owners sometimes reject an implementation after its product and
engineering artifacts have entered the repository. Keeping the rejected
runtime and authoring surface advertises unsupported behavior and creates
ongoing maintenance cost. Removing it under the completion path would make a
false claim and can erase evidence or unrelated work if ownership is guessed.

### 2.2 Users and segments

| Segment | Role and context | Domain exp. | Tech exp. | Key goals | Pain today | Evidence |
|---|---|---|---|---|---|---|
| **P1 (primary)** | Feature owner deciding the fate of rejected repository work | master | journeyman | remove unsupported surfaces; preserve decisions and unrelated work | no truthful destructive terminal path | Operator request, 2026-09-21 |
| P2 (secondary) | Maintainer reviewing repository integrity | master | master | verify exact authority and containment offline | completion evidence is the only existing source-retirement authority | Current memory-gate behavior and `prd:parallel-memory/NR-1` |
| **Anti-segment** | Maintainer decommissioning a successfully delivered feature or deleting shared infrastructure | — | — | — | those lifecycles need different contracts | NG-4, WR-1, WR-2 |

### 2.3 Jobs and desired outcomes

| # | When… (situation) | I want to… (motivation) | So I can… (outcome) | Today's workaround | Evidence |
|---|---|---|---|---|---|
| J1 | I permanently abandon an incomplete feature | remove its active implementation without claiming completion | stop advertising and maintaining rejected behavior | retain the surface or propose integrity exceptions | Operator request, 2026-09-21 |
| J2 | I review a destructive cleanup | see exact prior authority, ownership, and preserved history | distinguish intended retirement from accidental deletion | reconstruct intent across commits and memory failures | Failed source-retirement and code-transition checks on the E2E removal candidate |
| J3 | A previously abandoned feature is restored | require fresh authority for any later removal | prevent an old decision from deleting a new generation | manually reason about Git history | Existing stale-evidence protections |

### 2.4 Evidence

| Claim | Source (research, data, tickets, interviews) | Confidence |
|---|---|---|
| The E2E test-generation implementation is intentionally abandoned and should be removed while its historical records and unrelated E2E tests remain | Direct operator request and scope rulings, 2026-09-21 | High — authoritative product decision |
| Existing completion closeout cannot truthfully represent abandonment | Current closeout contract requires aligned live requirements, designs, and implementation completeness | High — executable contract and refusal behavior |
| The lifecycle already distinguishes abandonment from cleanup | `prd:feature-implementation-drive/FR-29` makes abandonment terminal while preserving durable artifacts until separately confirmed cleanup | High — merged Must requirement |
| The current E2E removal candidate is rejected by authored-source and code-transition memory checks | Reproduced local memory gate: exact removal candidate reports source-retirement and code-transition regressions | High — executable current-tree evidence |
| Path or name matching is insufficient to prove exclusive ownership | Source audit found E2E behavior crossing skills, templates, commands, packages, living documentation, and shared executor seams | High — repository inspection |

### 2.5 The journey today

| Phase | User does | User feels / pain | Frontstage (what user sees) | Backstage (systems, people, processes) |
|---|---|---|---|---|
| Decide | marks an incomplete feature abandoned | decision is clear | terminal feature intent | durable artifacts remain protected |
| Select | identifies active surface to remove | ownership is uncertain at shared seams | repository paths and code units | typed ownership is incomplete for mutable sources |
| Propose | removes the selected surface | valid intent still fails | source-retirement and code-transition findings | completion closeout is the only accepted feature-level authority |
| Recover | considers exceptions or a false closeout | safety and truth are in tension | blocked proposal | baseline and trusted-history rules correctly reject self-authorization |

### 2.6 Opportunity and why now

- **Size** — one current high-confidence pilot spans the installed agent surface, commands, code packages, and living documentation; broader frequency is not yet measured.
- **Alternatives users have** — retain unsupported behavior, falsely close out the feature as complete, weaken the baseline, or reverse old commits and risk unrelated changes.
- **Why us / strategic fit** — Pyxis already retains lifecycle, Git-history, ownership, and immutable evidence needed to make destructive intent reviewable.
- **Why now** — the E2E implementation has been rejected and its prepared removal is blocked by a legitimate gap in the lifecycle contract.
- **Cost of doing nothing** — Pyxis continues distributing an unsupported feature surface and carrying its integration hooks, while the prepared cleanup cannot pass the repository's own integrity gate.

**Decision:** The problem is accepted; the requirement contract and final solution gate remain part of the introducing proposal review.

| Reviewer | Role | Date | Verdict |
|---|---|---|---|
| @mdarabi | Operator and affected feature owner | 2026-09-21 | Problem accepted through the direct removal request; capture: none — pre-document ruling, duration not measured · items: [in] preserve historical records · [in] retain unrelated E2E tests |

## 3. Goals, metrics, and guardrails

### 3.1 Goals as falsifiable hypotheses

| # | We believe that… | Will result in… | Measured by… |
|---|---|---|---|
| G1 | exact prior authority can distinguish deliberate abandoned-feature cleanup from accidental deletion | the selected pilot surface can be removed without weakening unrelated integrity checks | M1, M3 |
| G2 | typed product-state, lifecycle, inventory, and ownership prerequisites can contain destructive scope | protected history and unrelated E2E test sources remain byte-identical, shared identities are never retired, and unrelated behavior keeps passing | M2, M3, M4 |

### 3.2 Goals → signals → metrics

| Metric | Goal | Signal | Baseline | Target (at checkpoint) | Feeds from |
|---|---|---|---|---|---|
| M1 (primary) | G1 | Complete authorized active surface accepted as absent by every gate mode | Current pilot removal is refused | 100% of exact selected pilot identities accepted after prior authority lands, zero unexplained residual surface, and 0% accepted in the same proposal | EVT-1 |
| M2 | G2 | Protected historical files and the exact unrelated-E2E inventory retain path, mode, bytes, and digest | Candidate already intends preservation; no authoritative retirement proof exists | 100% exact identity for both inventories in the verified removal | EVT-1 |
| M3 | G1, G2 | Invalid destructive transitions refused | Existing gates refuse all source retirement without completion closeout | 100% refusal across live-contract, unowned, shared, protected, stale, reintroduced, candidate-authored, dirty, and unavailable-history fixtures | EVT-1 |
| M4 | G2 | Unrelated E2E behavior passes its unchanged test inventory | The current unrelated E2E suites pass before retirement | 100% of the same unrelated E2E tests pass on the verified cleanup candidate and in the fresh-clone reproduction | EVT-1 |

### 3.3 Guardrails and decision rule

| Guardrail metric | Threshold | Action if breached |
|---|---|---|
| Protected, shared, or unselected identity removed | Zero identities | Stop cleanup, restore the candidate, and correct selection or ownership evidence |
| Active attempt or execution routing remains for the abandoned feature | Zero active attempts and zero assigned routes | Stop before authority publication and complete cancellation and abandonment |
| Live requirement or governing design left active for the abandoned feature | Zero live obligations contradicted by cleanup | Stop before authority publication and complete the typed product transition |
| Debt baseline or ordinary reconciliation weakened | Zero widened allowances or bypassed changed-unit checks | Stop rollout and retain existing gate behavior |
| Stale authority accepted for a restored generation | Zero accepted stale transitions | Stop rollout and retain the restored surface |

**Ship rule:** The pilot advances from authority publication to cleanup only
when M3 is perfect, every §3.3 guardrail is clean, and the authority is
already represented in the trusted base. Completion additionally requires M1,
M2, and M4 at target in a fresh transport clone.

### 3.4 Non-goals and waiting room

| ID | Non-goal | Why refused |
|---|---|---|
| NG-1 | Delete, rewrite, move, or hide merged PRDs, designs, brainstorms, plans, reviews, incidents, governing documents, or memory evidence | They explain the decision and remain historical truth |
| NG-2 | Infer ownership or selection from a feature name, directory, branch, proposal, or commit range | Naming and commit boundaries do not prove exclusive current ownership |
| NG-3 | Count abandonment as successful feature completion | The statements and evidence are materially different |
| NG-4 | Decommission a delivered feature, archive historical records, delete remote branches or proposals, or clean external services | Each has a different lifecycle and authority boundary |
| NG-5 | Retire shared or unowned code and sources | The first version is confined to exclusive identities |
| NG-6 | Remove or redesign unrelated E2E tests | They remain part of repository verification |
| NG-7 | Add a service, model decision, network dependency, or mutable success ledger | Destructive authority must remain deterministic and offline-verifiable |

| ID | Waiting room | Revisit when / intended version |
|---|---|---|
| WR-1 | Shared-artifact decomposition and retirement | A real abandoned feature cannot be cleaned up without changing a shared identity |
| WR-2 | Delivered-feature decommissioning | A shipped feature requires removal under an approved product lifecycle |
| WR-3 | Historical-record archival or access boundaries | Repository scale or policy establishes a distinct archival requirement |

## 4. Solution space

### 4.1 Approach and alternatives considered

Use a two-stage destructive lifecycle. First, the feature's current product
obligation reaches a typed terminal state and a reviewed repository change
publishes authority over exact, exclusively owned current identities while
they still exist. A later change may remove only those identities; the same
offline checks replay authority, ownership, generation, and preservation.

| Option | Sketch | Why not chosen |
|---|---|---|
| A (chosen) | Typed product retirement, then prior exact authority, then separately reviewed deletion | — |
| B | Treat abandonment as completion closeout | Produces a false completeness statement and loses lifecycle meaning |
| C | Add deletion paths to the accepted-debt baseline | Lets a destructive candidate weaken its own evaluator and records no exact operator authority |
| D | Revert original feature commits | Later commits and shared files make commit boundaries an unsafe current ownership perimeter |
| E | Infer all matching feature paths | Can delete shared work and miss active surfaces with different names |
| Do nothing | Keep the rejected feature indefinitely | Retains unsupported behavior and maintenance cost |

### 4.2 Scope perimeter

- **In (plan of record)** — typed abandonment prerequisite; explicit selection; exclusive mutable-source and code-unit ownership; exact current-generation authority; separate authority and deletion proposals; protected historical classes; offline parity; fresh-clone proof; one E2E test-generation pilot.
- **Future (believed next, not committed)** — shared-artifact decomposition, delivered-feature decommissioning, and general archive policy after evidence in WR-1–WR-3.
- **Out** — automatic product cancellation, completion semantics changes, historical-record removal, path-derived ownership, remote-service cleanup, branch deletion, shared-unit retirement, unrelated E2E changes, and model inference.

### 4.3 Experience: flows and states

**F1 — establish terminal product and execution state.** The feature owner
first cancels the owning feature drive, waits for every active attempt to
quiesce, and releases execution ownership and routing before explicitly
abandoning it. A legacy feature with no drive state records an equivalent
human-authorized terminal transition. The owner then uses the owning
immutable-evolution mechanisms to remove its current product obligation and
governing-design liveness. Frozen plans receive an explicit disposition;
their source remains history.

**F2 — publish retirement authority.** From a trusted clean mainline, the
operator derives the complete active feature surface, then selects every
eligible exclusive mutable source and code unit. The system reports their
current generation, owner evidence, protected-class status, all shared
identities requiring ordinary reconciliation, and the complete proposed
authority without deleting anything. The operator reviews and lands that
authority as its own proposal.

**F3 — remove the selected surface.** A later proposal removes the selected
identities. Validation reads authority only from the trusted base, proves the
candidate matched it exactly, and verifies preserved history and unrelated
behavior before accepting the transition.

**F4 — restore and retire again.** Restoring any selected identity creates a
new generation. A later deletion remains blocked until the lifecycle and
authority flow is repeated for that generation.

- **Ideal** — the exact exclusive selection is previewed, authority lands, and the later removal passes with preservation proof.
- **Empty** — no eligible identity produces no authority and no write.
- **Loading** — not applicable; each command is a bounded synchronous repository operation.
- **Partial** — a mixed eligible and ineligible selection refuses the whole authority batch; deletion remains an ordinary proposal until every gate passes.
- **Error** — the result names the failed identity, predicate, trusted input, and repair without modifying selected sources or publishing partial authority.
- **Accessibility** — CLI and proposal text remain screen-reader-readable and do not rely on color or a browser-only view.
- **Localization** — machine values and operator CLI text are English-only; no customer-facing strings ship.

### 4.4 Scenarios

An operator retires the E2E test-generation feature's live contract while
keeping its merged PRD, designs, plans, brainstorms, and reviews in place.
The operator previews exact skill, authoring, template, command, and code-unit
selection; a shared executor seam refuses selection. After the exclusive
authority proposal lands, a rebased removal deletes only authorized identities
and updates any changed shared units through ordinary reconciliation.

By contrast, a deletion that adds its authority in the same proposal fails.
So does an attempt made while the E2E contract is live, an unowned wiki entry,
a shared helper, a protected design, a changed file mode, an identical code
unit restored inside a file that never disappeared, or a clone missing the
retained history needed to replay authority.

**Decision:** The two-stage, exact, preservation-first direction was approved
by the operator on 2026-09-21; the introducing proposal reviews this full
requirement contract.

| Reviewer | Role | Date | Verdict |
|---|---|---|---|
| @mdarabi | Operator | 2026-09-21 | Initial solution direction approved; capture: none — pre-document ruling, duration not measured · items: [in] keep historical records · [in] keep unrelated E2E tests |

## 5. System requirements

### 5.1 Functional requirements

Must priority is ranked by row order. All destructive-authority and
preservation requirements carry critical risk regardless of implementation
size.

| ID | Requirement (EARS) | Rationale | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|
| FR-1 | When a feature owner explicitly abandons a feature, the system shall preserve its merged product contracts, designs, brainstorms, plans, reviews, incidents, governing documents, and durable evidence as historical records. | Abandonment must retain the reason and decision trail. | Must | critical | Every protected-class fixture and the pilot retain identical tracked bytes before and after cleanup. | Test + Inspection | G2, J1, F1, F3 |
| FR-2 | Before retirement authority can be published, the system shall require observed proof that the owning feature lifecycle is explicitly abandoned after cancellation, every active attempt is quiesced, execution ownership and routing are released, the current product obligation and governing-design liveness have typed terminal dispositions, and every affected frozen-plan trace has an explicit disposition. | The repository must express one terminal product and execution state before destruction. | Must | critical | Active, cancellation-pending, cancelled-but-resumable, stale, malformed, live-requirement, live-design, missing-supersession, and unresolved-plan fixtures all refuse; a valid drive terminal and an explicit legacy-equivalent control pass. | Test | G2, J1, F1 |
| FR-3 | When an operator selects a mutable authored source for retirement, the system shall require closed, replayable, typed evidence that assigns the source exclusively to the abandoned feature. | Names and paths are not ownership authority. | Must | critical | Eligible-class controls pass; missing, inactive, shared, inferred, and conflicting owner evidence all refuse. | Test | G2, J2, F2 |
| FR-4 | When an operator selects a production code unit for retirement, the system shall require active semantic evidence that assigns the exact unit exclusively to the abandoned feature. | Shared implementation must remain protected. | Must | critical | Each supported unit-kind control passes; zero-owner, multi-owner, changed-owner, and ambiguous-unit fixtures refuse. | Test | G2, J2, F2 |
| FR-5 | When retirement authority is published, the system shall bind the explicit feature, reason, actor, trusted source state, complete selector set, ownership evidence, preservation inventory, baseline identity, and exact current identities in immutable repository evidence. | Reviewers need one replayable statement of intent and scope. | Must | critical | Canonical fixed-input output is byte-identical and any changed field produces a different valid identity or refusal. | Test | G1, J2, F2 |
| FR-6 | When authority identifies mutable source or code, the system shall bind the exact current generation so removal and later restoration create identities that old authority cannot cover, including a code unit that disappears and returns inside a persistent file. | Matching bytes alone cannot distinguish restored work. | Must | critical | Real-Git add, edit, rename, copy, delete/re-add, root, merge-add, multi-file, and same-file unit restoration fixtures agree across every verifier; second deletion refuses old authority. | Test | G1, J3, F2, F4 |
| FR-7 | When a candidate removes a selected identity, the system shall accept retirement authority only when the exact authority is already active in the trusted comparison base and unchanged in the candidate. | A destructive proposal cannot authorize itself. | Must | critical | Same-proposal and candidate-modified authority refuse; the identical deletion passes only after authority lands in the base. | Test | G1, J2, F3 |
| FR-8 | While issuing retirement authority, the system shall leave every selected source and code unit unchanged and publish the complete batch atomically, with exact retries writing nothing. | Selection and destruction require separate review events. | Must | critical | Writer manifests show zero selected-source changes, all-or-nothing publication under each injected interruption, and zero writes on exact retry. | Test + Failure injection | G1, J2, F2 |
| FR-9 | When an authorized cleanup proposal is evaluated, the system shall accept only disappearance of exact selected identities and shall apply ordinary change reconciliation to every present changed or unselected identity. | Retirement authority must not become edit authority. | Must | critical | Mixed deletion/change fixtures accept exact authorized absences and refuse changed, unselected, or partially removed identities without fresh ordinary evidence. | Test | G1, G2, J2, F3 |
| FR-10 | When cleanup completes, the system shall prove that every protected historical record and every unrelated E2E test in the pre-cleanup inventory retains its exact path, mode, bytes, and digest, and that the unchanged unrelated E2E inventory passes on the cleanup candidate. | Containment includes source integrity and behavior, not mere test presence. | Must | critical | Pilot inventory reports zero protected or unrelated-test identity changes and the exact pre-cleanup unrelated E2E set passes on the candidate and fresh clone. | Test + Inspection | G2, J1, F3 |
| FR-11 | When equivalent pinned inputs are validated locally, in hosted gates, offline, or from a fresh transport clone, the system shall produce the same retirement classification and evidence identity. | Authority must survive squash, branch deletion, and environment changes. | Must | critical | Mode matrix agrees for every positive and negative fixture; fresh mainline-only clone verifies the pilot after branch deletion. | Test | G1, J2, F3 |
| FR-12 | When a retirement operation refuses, the system shall identify the action, trusted inputs, selected identity, ownership state, lifecycle state, and either one deterministic repair transition or an explicit irreparable outcome without exposing source bodies or credentials. | Operators must repair a refusal without guessing, following a dead end, or leaking content. | Must | high | Human and machine output identify the same transition; for every repairable closed refusal code, applying the emitted repair removes that predicate on exact retry without weakening policy; irreparable controls remain refused; no seeded body or credential appears. | Test + Demo | G1, G2, J2, F2, F3 |
| FR-13 | Before authority publication or cleanup acceptance, the system shall require the checked-in integrity policy to admit the pinned repository state without widening, replacing, or bypassing existing debt. | Retirement cannot hide a new regression. | Must | critical | Exact admitted baseline passes; every added allowance, evaluator change, missing input, and new finding refuses. | Test | G1, G2, F2, F3 |
| FR-14 | Before retirement authority can be published, the system shall derive a complete active-surface inventory for the abandoned feature from typed ownership and capability references, require every eligible exclusively owned identity in that inventory to be selected, and assign every shared identity an explicit preserved or ordinary-reconciliation disposition. | Exact selectors are safe only when omissions are also detectable. | Must | critical | The pilot and fixture features report zero unexplained active surfaces; omitted exclusive source, omitted code unit, unresolved entry point, unowned live reference, and undispositioned shared identity all refuse. | Test + Inspection | G1, G2, J1, J2, F2 |
| FR-15 | When a retirement result carries a failure or judgment signal, the system shall retain one bounded immutable Git event linked to the action, pinned inputs, selector or authority identity, refusal, and disposition before a later action for that identity can be accepted; exact clean rechecks shall create no event. | Destructive failures are improvement evidence under AP-28. | Must | high | Every non-clean fixture produces exactly one schema-valid event in retained history before repair acceptance; exact clean retries produce zero events; duplicate and mutable event fixtures refuse. | Test | G1, G2, J2, F2, F3 |

**Non-regression (NR-n)**

| ID | Existing behavior protected | Why protected | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|
| NR-1 | Completion closeout remains the only feature receipt that asserts implementation completeness | Abandonment must not make a false delivery claim | critical | Existing incomplete, missing-design, and misaligned-requirement closeout fixtures retain refusal; valid completion controls remain unchanged | Test | NG-3, F1, F2, F4 |
| NR-2 | Frozen document mutability, typed PRD amendment, whole-design supersession, incident append-only behavior, and governing-document protections remain unchanged | Retirement must use existing evolution rules and preserve history | critical | Existing document gates remain green; protected-class mutation fixtures refuse before retirement evaluation | Test | FR-1, FR-2, F1, F2 |
| NR-3 | Ordinary source, code, record, and accepted-debt reconciliation remains active for every transition outside exact retirement authority | Destructive authority is narrow | critical | Existing changed-code, stale-anchor, missing-evidence, debt-widening, and unselected-deletion fixtures retain refusal | Test | FR-9, FR-13, F2, F3, F4 |
| NR-4 | Unrelated E2E tests retain their exact source identity and execution behavior | The retirement decision excludes them | high | Pilot inventory shows zero path, mode, byte, or digest changes to unrelated E2E tests, and the identical test set has zero new failures on candidate and fresh clone | Test + Inspection | NG-6, FR-10, F3 |

### 5.2 Unwanted behavior and failure handling

| ID | If… (trigger) | Then the system shall… | User sees | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|---|
| UB-1 | If any feature requirement, governing design, or affected frozen-plan trace lacks a typed terminal disposition | Refuse authority publication and cleanup | lifecycle precondition with exact unresolved anchor | Must | critical | One fixture per live or missing disposition writes zero authority and accepts zero deletions | Test | FR-2, F1 |
| UB-2 | If abandonment lacks explicit human authority, an owning feature drive is active, cancellation is pending, an attempt is not quiesced, ownership or routing remains assigned, the feature remains resumable, or legacy-equivalent terminal evidence is absent | Preserve the surface and refuse destructive transition | lifecycle predicate and exact terminal transition required | Must | critical | Missing, inferred, agent-only, active, cancellation-pending, non-quiesced, routed, cancelled-but-resumable, stale, malformed, conflicting, and legacy-without-equivalent fixtures all refuse | Test | FR-2, F1 |
| UB-3 | If any selection is protected, unowned, shared, ambiguous, inferred, unsupported, or absent | Refuse the entire batch before writing | selector and failed ownership predicate | Must | critical | Mixed batches publish zero authority for every failure class | Test | FR-1, FR-3, FR-4, F2 |
| UB-4 | If authority is introduced, changed, revoked, malformed, inactive, or unavailable in the deletion candidate rather than trusted at its base | Refuse the deletion | authority and trusted-base remedy | Must | critical | Every candidate-authority and unavailable-history fixture accepts zero selected absences | Test | FR-7, FR-11, F3 |
| UB-5 | If selected bytes, mode, unit digest, owner set, generation, or preservation inventory differs from authority | Refuse the stale or over-broad transition | mismatched identity and fresh-authority remedy | Must | critical | Mutation matrix and restore-then-delete matrix agree across local, hosted, offline, and fresh-clone modes | Test | FR-5, FR-6, FR-11, F3, F4 |
| UB-6 | If the trusted source is dirty, moved, unconfigured, not admitted by policy, or not the configured mainline | Write no authority and accept no cleanup; retain only FR-15's bounded failure event | pinned-input refusal and recovery action | Must | critical | Each source-state fixture changes zero source, authority, or policy paths, produces no accepted retirement, and retains exactly the permitted failure event | Test | FR-8, FR-13, FR-15, F2 |
| UB-7 | If publication is interrupted or only part of a batch exists | Treat no partial batch as authority, retain the failure event, and permit only an exact authorized retry | explicit incomplete result and pending repair | Must | critical | Failure injection at every write boundary never authorizes a deletion; the retained event is singular and retry converges exactly once | Test + Failure injection | FR-8, FR-15, F2 |
| UB-8 | If a verifier cannot replay ownership, lifecycle, Git history, evidence, or policy from retained inputs | Fail closed without ambient, network, or model fallback | unavailable evidence and exact missing input | Must | critical | Missing-object and unknown-schema corpus produces zero accepted transitions in every mode | Test | FR-3, FR-4, FR-6, FR-11, FR-13 |
| UB-9 | If complete active-surface discovery is unavailable or reports an omitted exclusive identity, unresolved entry point, unowned live reference, or shared identity without a disposition | Refuse the whole authority batch | residual surface and required inventory repair | Must | critical | Every omission and unavailable-inventory fixture writes zero authority; complete controls report zero unexplained residual surface | Test | FR-14, F2 |

### 5.3 Permissions and abuse cases

| Function / data | Role: feature owner/operator | Role: automated author or gate | Unauthenticated |
|---|---|---|---|
| Decide abandonment and approve product-contract retirement | authorize | propose and verify only | none |
| Publish retirement authority | review and merge | preview, write proposal, and verify | none |
| Delete selected repository identities | review and merge | propose and verify only | none |
| Read historical records and retirement evidence | read | read and verify | repository policy only |

| ID | Actor and intent | Vector | Impact | Severity | Mitigated by | Verify |
|---|---|---|---|---|---|---|
| ABUSE-1 | Candidate author hides unwanted code by authorizing its deletion in the same change | candidate-authored authority | evidence and implementation disappear together | high | FR-7, UB-4, NFR-2 | two-proposal real-Git drill |
| ABUSE-2 | Feature owner over-selects shared or historical artifacts | broad selectors or inferred ownership | unrelated behavior or decision history is lost | high | FR-1, FR-3, FR-4, UB-3 | protected/shared selection corpus |
| ABUSE-3 | Stale authority is replayed against restored work | exact byte restoration or history manipulation | new implementation is deleted under an old decision | high | FR-6, UB-5, NFR-2 | generation replay corpus |
| ABUSE-4 | Candidate weakens the baseline to make cleanup pass | policy change beside deletion | unrelated integrity regression is hidden | high | FR-13, NR-3 | mixed baseline/deletion drill |

### 5.4 Data

| Entity / field | Meaning | Source of truth | Retention | Integrity rules | Privacy class |
|---|---|---|---|---|---|
| Feature abandonment disposition | Human-authorized terminal product state | owning product-contract and feature-lifecycle artifacts | permanent history | typed immutable evolution; one current state | internal |
| Retirement authority | Exact approved scope and identity evidence for later cleanup | immutable repository evidence | permanent with repository history | canonical, content-addressed, trusted-base-only, replayable | internal |
| Active feature surface inventory | Complete current set of exclusive, shared, protected, and unresolved feature-associated identities and their disposition | typed ownership and capability projections at a pinned revision | with the authority and its verification evidence | complete at publication; zero unexplained residual identity | internal |
| Mutable-source ownership evidence | Feature relation for an eligible authored source generation | repository memory projection | while the source generation or its retirement evidence is retained | closed schema; active-state and exclusivity rules | internal |
| Code-unit ownership and generation evidence | Feature relation and lineage for an exact production unit | repository memory and retained Git history | while the unit generation or its retirement evidence is retained | exact unit identity, owner set, and history replay | internal |
| Retirement failure event | Bounded failure or judgment signal from one retirement action | immutable Git event record | permanent history | append-only, canonical action/input links, no clean-pass record | internal |

- **Migration** — existing completion closeouts remain unchanged; the new authority class begins empty and grants nothing retroactively.
- **Parallel-run / cutover** — readers and gates land before the first authority; existing closeout-only behavior remains until a reviewed authority is present in the trusted base.
- **Rollback** — before any cleanup, disabling the new authority reader restores the prior refusal behavior; after cleanup, rollback restores removed identities through an ordinary reviewed proposal while retaining the authority history.
- **Decommission** — remove the publication surface only after no pending cleanup depends on it; retained dispositions and authority remain readable evidence.
- **Privacy check** — no DPIA trigger: evidence stores repository identities, digests, owner handles, and operator reason, with no new personal or sensitive data category.
- **Egress** — none new; all validation is offline and repository-local.

### 5.5 Interfaces and integrations

| Interface | Direction | Purpose | Contract (format, ranges, timing) | Failure modes | Owner |
|---|---|---|---|---|---|
| Product-contract lifecycle | in | prove current feature obligations are terminal | typed current-state projection at pinned revision | live, ambiguous, stale, unsupported | Product contract owner |
| Feature-drive lifecycle | in | prove cancellation, quiescence, released routing, and explicit abandonment | typed terminal state or explicit legacy-equivalent transition at pinned revision | active attempt, pending cancellation, resumable, stale, malformed | Feature lifecycle owner |
| Design index and plan traces | in | prove governing designs and frozen work have dispositions | typed live/superseded and trace state at pinned revision | live dependent, missing disposition, stale index | Document system owner |
| Memory evidence | both | read ownership and publish immutable retirement authority | closed versioned records under existing writer rules | malformed, partial, inactive, conflicting | Memory maintainer |
| Git history and configured mainline | in | derive generations and trusted-base ancestry | retained repository objects at explicit revisions | missing object, ambiguous lineage, moved head | Repository maintainer |
| Integrity policy and gates | both | require admitted source state and classify cleanup | exact checked-in evaluator and policy identity | widening, unsupported version, mode disagreement | Gate owner |

### 5.6 Non-functional requirements

| ID | Area | Requirement | Fit criterion | Verify |
|---|---|---|---|---|
| NFR-1 | Determinism | The system shall derive authority bytes and retirement classifications only from declared repository inputs and explicit operator decisions. | Fixed inputs produce byte-identical output and verdicts under shuffled enumeration, with zero clock, randomness, network, or model influence. | Test |
| NFR-2 | Security and integrity | The system shall fail closed on ambiguous authority, ownership, lifecycle, lineage, history, or policy state. | The complete ambiguity and abuse corpus accepts zero destructive transitions. | Test |
| NFR-3 | Atomicity and recovery | The system shall make authority publication all-or-nothing and exact retries convergent. | Failure injection at every boundary yields either the complete expected batch or no usable authority; retry writes no duplicate. | Test + Failure injection |
| NFR-4 | Portability | The retirement decision shall remain equivalent across supported local and hosted gate modes and a fresh transport clone. | Mode matrix returns the same normalized classification and evidence identity for every fixture. | Test |
| NFR-5 | Operability | The system shall name the failed predicate and deterministic repair transition or explicit irreparable outcome for every refusal without requiring source-body inspection. | Every repairable negative fixture succeeds after its emitted repair and exact retry without policy weakening; irreparable fixtures remain refused; human and machine forms agree. | Test + Demo |
| NFR-6 | Privacy | The system shall expose no source body, credential, or secret in retirement evidence or diagnostics. | Seeded-secret and body fixtures produce zero matching payload fragments. | Test |
| NFR-7 | Work per check | For P pinned snapshots and N inventoried identities, the system shall keep Git process launches O(P), independent of N, and repository-object processing O(P + N). | With P fixed, inventories of N and 10N launch the same number of Git processes and perform no more than linear object work; elapsed time is advisory. | Test + Analysis |
| NFR-8 | Cost | The system shall spend no model inference on publication, ownership, lineage, or gate verdicts. | Instrumentation attributes zero tokens and zero network requests to every deterministic retirement path. | Analysis |

### 5.7 Instrumentation — the tracking plan

| ID | Event name | Fires when | Properties (typed) | Source | Feeds |
|---|---|---|---|---|---|
| EVT-1 | feature_retirement_result | Authority preview/publication or cleanup validation completes | action: enum, accepted: bool, feature: string, pinned_inputs: string[], selected_count: int, residual_surface_count: int, protected_changed_count: int, unrelated_changed_count: int, unrelated_suite_pass: bool, refusal_code: string, authority_id: string | Existing CLI/gate result and retained test evidence; no new event service | M1, M2, M3, M4 |
| EVT-2 | feature_retirement_failure | A retirement result carries failure or judgment signal | action: enum, feature: string, pinned_inputs: string[], selector_or_authority_id: string, refusal_code: string, disposition: string, repair_transition: string, irreparable: bool | Immutable Git event record | AP-28, DOD-5 |

### 5.8 Definition of Done

| ID | Done criterion | Source | Verify | Required evidence | Blocking? |
|---|---|---|---|---|---|
| DOD-1 | Terminal product and execution lifecycle preconditions and protected-history preservation satisfy every positive and negative case | FR-1, FR-2, UB-1, UB-2, NR-2 | Test + Inspection | typed lifecycle fixture matrix, legacy-equivalent controls, and protected-byte manifests | yes |
| DOD-2 | Mutable-source and code-unit ownership admit only exact exclusive selections | FR-3, FR-4, UB-3 | Test | owner-state corpus covering every eligible class and supported unit kind | yes |
| DOD-3 | Exact authority, generation lineage, and trusted-base admission prevent self-authorization and stale replay | FR-5, FR-6, FR-7, UB-4, UB-5, ABUSE-1, ABUSE-3 | Test | real-Git lineage goldens and two-proposal transition matrix | yes |
| DOD-4 | Publication, baseline admission, failure handling, and retries are atomic and do not widen integrity policy | FR-8, FR-13, UB-6, UB-7, UB-8, NFR-1, NFR-3 | Test + Failure injection | write manifests, policy diffs, interruption and retry evidence | yes |
| DOD-5 | Every validation mode and fresh clone agree, diagnostics have proven repairs, and every non-clean signal is retained without private content | FR-11, FR-12, FR-15, NFR-4, NFR-5, NFR-6, NFR-7, NFR-8, EVT-2 | Test + Analysis | mode-parity report, repair-and-retry corpus, immutable failure events, operation counters, privacy corpus, fresh-clone proof | yes |
| DOD-6 | The complete E2E test-generation pilot surface is absent while historical records and the exact unrelated E2E test inventory remain unchanged and passing | FR-9, FR-10, FR-14, UB-9, NR-1, NR-3, NR-4, §3.3, §6.4 | Test + Inspection | complete active-surface inventory with zero residuals, final tree inventory, protected and unrelated-test digests, focused regression results, full required shared gates | yes |
| DOD-7 | The operator judges the final pilot diff to match the approved retirement perimeter with no unexplained deletion or residual active surface | G2, FR-14, NG-1–NG-6 | Human judgment — @mdarabi, rubric: every deleted identity appears in the prior authority; every exclusive active identity is deleted; every shared identity has its declared disposition; every retained class and unrelated E2E test is present | final proposal review record | yes |

### 5.9 AI behavior and evals

Skipped: the capability is deterministic and ships no model-driven behavior.

## 6. Delivery

### 6.1 Phasing and milestones

| Phase | Purpose | Scope | Exit criterion |
|---|---|---|---|
| Contract | Establish one coherent current product state and downstream authority contract | PRD approval, adjacent-contract reconciliation, technical-design supersession, plan-trace disposition | Every design-review finding is closed and planning can trace each Must row without a live-contract contradiction |
| Gate | Prove authority without deleting product surface | compatible readers, writer, typed ownership, generation replay, offline validator | DOD-1 through DOD-5 pass and the existing gate remains closeout-only when no authority is present |
| Pilot authority | Publish exact E2E retirement authority | authority-only proposal over intact E2E generation surface | authority is merged, branch-deletion and fresh-clone checks pass, selected bytes remain present |
| Pilot cleanup | Remove only the authorized E2E generation surface | rebased removal, shared-unit reconciliation, living-doc updates, unrelated E2E regression proof | DOD-6 and DOD-7 pass on the final proposal |

### 6.2 Dependencies

| Dependency | Type (team / system / vendor / approval) | Needed by | Status | Owner |
|---|---|---|---|---|
| Operator approval of this requirement contract | approval | Contract | pending introducing proposal review | @mdarabi |
| Typed E2E product-contract and design retirement | approval and document system | Pilot authority | pending | E2E feature owner |
| Terminal E2E feature-lifecycle evidence with quiesced attempts and released routing | system and approval | Pilot authority | pending | E2E feature owner |
| Reconciliation of the existing closeout-only memory contract | document system | Gate | pending | Memory maintainer |
| Retained Git history and configured trusted mainline | system | Gate | present; acceptance proof pending | Repository maintainer |
| Mutable-source ownership evidence for the pilot classes | system | Gate | absent; design and implementation required | Memory maintainer |

### 6.3 Risks, assumptions, and rabbit holes

| # | Risk / assumption / rabbit hole | Likelihood | Impact | Mitigation or test | Owner |
|---|---|---|---|---|---|
| R1 | A mutable E2E source has no typed feature owner | H | H | make ownership evidence a release prerequisite; unowned selection refuses | Memory design owner |
| R2 | A code unit disappears and returns inside an unchanged file | M | H | unit-generation history proof and exact same-file restoration fixture | Memory design owner |
| R3 | Product retirement leaves a live design or frozen-plan contradiction | M | H | typed lifecycle preflight and index/trace fixtures before authority | Document system owner |
| R4 | Shared executor or plan logic is mistaken for exclusive E2E code | M | H | positive owner plus zero other owners; mixed pilot fixture | E2E feature owner |
| R5 | Squash merge or branch deletion removes needed authority provenance | M | H | fresh mainline-only transport-clone proof at both proposal boundaries | Repository maintainer |
| R6 | The generic mechanism grows into an archive or remote cleanup system | L | M | enforce NG-1, NG-4, and the header appetite in design review | Product owner |
| R7 | The selector manifest is internally exact but omits an active feature entry point | M | H | complete typed active-surface inventory, residual fixtures, and operator rubric | E2E feature owner |

### 6.4 Release criteria

| Lens | Criterion | Evaluated by |
|---|---|---|
| Functionality | DOD-1 through DOD-7 pass at the applicable phase | Acceptance corpus, shared gates, and operator review |
| Usability | Every refusal and successful preview satisfies FR-12 without requiring source-body inspection | CLI fixture and operator inspection |
| Reliability | UB-1 through UB-9, ABUSE-1 through ABUSE-4, and the §3.3 guardrails accept zero invalid destructive transition | Failure injection and real-Git transition tests |
| Performance | NFR-7 operation bound is recorded for the pilot; elapsed timing remains advisory | Operation counters and analysis |
| Supportability | Publication, retry, verification, rollback, and restoration procedures are documented and reproduced offline | Maintainer inspection and fresh-clone rehearsal |

### 6.5 Rollout and reversibility

This capability advances by repository evidence rather than traffic
percentages. Reader and validator compatibility land first with no authority
present, so prior closeout-only behavior remains the fallback. The writer then
lands and the complete negative corpus runs before any real authority is
published.

The first authority is a proposal over the intact E2E generation surface.
Disabling publication after it lands prevents new authority but keeps the
existing evidence readable. Cleanup is a later human-reviewed proposal; its
kill switch is to leave that proposal unmerged. After cleanup, restoration is
an ordinary proposal that reintroduces code and sources under normal memory
rules; the historical retirement authority remains and cannot cover the new
generation.

| Stage | Audience | Minimum evidence | Advance when | Roll back when |
|---|---|---|---|---|
| Compatibility | repository gates with no authority records | existing suite plus complete new negative corpus | every prior verdict is unchanged and M3 is 100% | any existing transition changes |
| Pilot authority | E2E test-generation feature only | DOD-1 through DOD-5 and human review of the complete active-surface inventory and selector preview | authority is trusted in a fresh clone, zero residual surface is unexplained, and selected surface is byte-identical | any guardrail breach, residual surface, or selector disagreement |
| Pilot cleanup | E2E test-generation feature only | trusted authority plus final candidate evidence | DOD-6 and DOD-7 pass | any protected, shared, unrelated, or unselected change |

### 6.6 Launch readiness

| Area | Item | Owner | Status |
|---|---|---|---|
| Contracts | requirement approval, adjacent amendment, design supersessions, and plan-trace disposition | @mdarabi | pending |
| Integrity | complete ownership, lineage, abuse, failure, and gate-parity corpus | Implementer | pending implementation |
| Docs and support | publication, retry, verification, rollback, and restoration commands | Memory maintainer | pending implementation |
| Pilot | exact E2E selector manifest and preserved-history inventory | E2E feature owner | prepared source audit; typed evidence pending |
| Platform | exact revision passes required shared gates and implementation review | Implementer | pending implementation |
| Privacy | evidence and diagnostics pass seeded-body and credential checks | Implementer | pending validation |

Sales, billing, legal, marketing, customer localization, and external support
have no changed surface for this repository-maintenance capability.

**Decision:** Launch requires §6.4 green, every applicable readiness row
complete, rollback rehearsed, and the final cleanup proposal approved by the
operator.

| Reviewer | Role | Date | Verdict |
|---|---|---|---|
| @mdarabi | Launch and destructive-change authority | pending | No launch claim; contract and implementation remain pending |

### 6.7 Post-launch review

- **Review trigger** — the E2E test-generation pilot cleanup is represented in retained mainline history and reproduced from a fresh transport clone · **Owner** — @mdarabi
- **Verdict recorded** — adopt, iterate, or kill with M1–M4, every guardrail, the final active-surface and selector manifests, and actual operator intervention on the header's Tracking ref.

## 7. Alignment

### 7.1 FAQ

**Customer**

- **Q:** Does retiring a feature erase its PRD, designs, plans, or review history? — **A:** No. Those records remain in Git and continue explaining what was attempted and why it ended.
- **Q:** Can retirement remove shared code or unrelated E2E tests? — **A:** No. The first version accepts only exact exclusively owned identities and proves unrelated E2E coverage remains.
- **Q:** Is an abandoned feature reported as completed? — **A:** No. Abandonment and completion remain separate terminal states with different evidence.

**Internal**

- **Q:** Why are authority and deletion separate proposals? — **A:** A candidate must not manufacture the evidence that permits its own destructive change; reviewers first inspect authority while the surface still exists.
- **Q:** Why is matching content insufficient for a restored feature? — **A:** The operator decision applied to one generation. Restoration creates new work even if its bytes match, so old authority cannot remove it.
- **Q:** Why not place the deleted paths in the accepted-debt baseline? — **A:** A baseline exception weakens the evaluator and carries neither exact lifecycle authority nor generation ownership.

### 7.2 Open questions

None. Concrete schemas, commands, generation algorithms, and supersession
layout are technical-design decisions constrained by FR-1 through FR-15.

### 7.3 Decision log

| Date | Decision | Alternatives considered | Why | Who |
|---|---|---|---|---|
| 2026-09-21 | Remove the abandoned E2E test-generation feature | keep the implementation; continue developing it | the current approach is unscalable and expected to cause downstream problems | @mdarabi |
| 2026-09-21 | Preserve historical feature records | delete every E2E-related document | the records remain evidence of requirements, decisions, and work history | @mdarabi |
| 2026-09-21 | Preserve unrelated E2E tests | remove all E2E testing | those tests verify other Pyxis features and are outside the abandoned project | @mdarabi |
| 2026-09-21 | Use prior exact authority for destructive cleanup | completion closeout; baseline exceptions; candidate-authored authority | preserve truthful lifecycle semantics and prevent self-authorized deletion | @mdarabi, initial direction approval |
| 2026-09-21 | Approve the abandoned-feature retirement requirement contract after automated review | revise the contract; reject the retirement path | the reviewed contract safely preserves history and unrelated behavior while authorizing a dedicated abandonment path | @mdarabi |

### 7.4 Changelog

| Date | Change | Author |
|---|---|---|
| 2026-09-21 | Drafted the requirement contract from the operator's retirement decision, source audit, reproduced gate refusal, and existing abandonment lifecycle | Codex |
| 2026-09-21 | Recorded operator approval after `prd-review` converged to `approve` | Codex |

## 8. Contract readiness — the executability gate

| Check | Pass? | Notes |
|---|---|---|
| Problem basis accepted — §1 problem accepted (T1); §2 problem gate signed (T2/T3) | yes | Direct operator request and scope rulings are recorded in §2.6 |
| Zero solution-blocking TBDs or open questions (§7.2) | yes | §7.2 leaves only design-owned mechanics constrained by requirements |
| Every Must FR has a fit criterion and a verification method (§5.1) | yes | FR-1 through FR-15 are measurable and testable |
| Every requirement row carries a risk tier; high/critical rows have a declared safety plan — staged rollout, rollback rehearsal, kill switch, migration fallback, manual recovery, or an explicit non-applicability rationale (§5.1, §6.5) | yes | Two-proposal rollout, fail-closed guards, unmerged-cleanup kill switch, and restoration path cover the destructive surface |
| Non-goals drawn (§3.4); non-regression rows named (§5.1) | yes | NG-1 through NG-7 and NR-1 through NR-4 define the perimeter |
| Every primary job (§2.3) is covered by a requirement, a non-goal, or a waiting-room entry | yes | J1 by FR-1/FR-2/FR-10/FR-14; J2 by FR-3–FR-15; J3 by FR-6/UB-5 |
| Every key flow (§4.3) has functional, failure, and non-regression coverage (§5.1–5.2) | yes | F1 through F4 trace through FR, UB, and NR rows |
| Every primary metric (§3.2) has instrumentation (§5.7) or an explicit measurement plan | yes | EVT-1 feeds M1 through M4 from retained CLI, gate, and test evidence; EVT-2 retains non-clean signal |
| Every blocking DoD row traces to a requirement, guardrail, release criterion, or human-judgment rubric (§5.8) | yes | DOD-1 through DOD-7 cite their sources |
| Risks and assumptions have owners (§6.3) | yes | R1 through R7 name accountable owners |
| Human-judgment criteria name an owner and a rubric (§5.8) | yes | DOD-7 names @mdarabi and an exact manifest rubric |
| DoD complete: every blocking row sourced and evidenced; every n/a explained (§5.8) | yes | All rows are blocking and name evidence classes |

## Superseded & retired

None — no unit has been superseded or retired; the ledger is its base.

## Superseded & retired

None — no unit has been superseded or retired; the ledger is its base.
