# Amendment 001 — declared local Git draft approvals

| Field | Value |
|---|---|
| **Amends** | feature-draft *(chain through 000)* |
| **Status** | Approved |
| **Owner** | @mdarabi |
| **Date** | 2026-09-30 |
| **Learned-from** | operator-decision:https://github.com/agenticlab-ai/pyxis/pull/1312 |
| **Blast radius** | tier: contract-changing; touches: FR-17, FR-35, FR-46, FR-47, FR-27, FR-30, WR-4, NR-3, UB-3, UB-11, UB-5, NFR-1, DOD-1, DOD-2, DOD-3, DOD-4, DOD-5, DOD-7, §0, §1, §2.2, §4.2, §4.3, §4.4, §5.3, §5.4, §5.5, §6.1, §6.3, §6.4, §6.5, §6.6, §7.1, §8; reason: authorizes declared local approvals under the accepted shared-account boundary while retaining authenticated hosted approvals and exact-content checks |
| **Tracking** | https://github.com/agenticlab-ai/pyxis/issues/1336; local activation follows in issues 1324 and 1326 |
| **Template version** | amendment-template/v2 |

This is proposed merge-target content. The envelope's Approved value takes
effect only after the operator reviews and approves this exact source in the
PR and merges the independent contract proposal; no prior approval is asserted.

## Operations

### op: supersede FR-17

Reason: Every lock still needs the operator's review, but a shared-account
local origin cannot prove that a human issued the approval.

| ID | Requirement (EARS) | Rationale | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|
| FR-48 | When a ready feature draft's lock is requested, the system shall request approval from the human operator and lock only with the approval evidence required by the selected approval surface in FR-50. | Retains one operator review per lock and states the evidence boundary for each medium | Must | critical | Hosted fixtures require an authenticated human approval and refuse agent approval; local fixtures require a matching declared receipt and refuse a missing or inconsistent one; the local output states that human authorship is unauthenticated. | Test + Inspection | G2, J2, F3 |

### op: supersede FR-35

Reason: Argus drives the draft and requests review in both media. Its lack of
local approval authority is a workflow rule, not an authenticated OS boundary.

| ID | Requirement (EARS) | Rationale | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|
| FR-49 | Where Argus drives a feature draft, the system shall let Argus run rounds and request the lock, and the Argus workflow shall leave approval and local landing invocation to the human operator. | Preserves the operator's decision while distinguishing workflow authority from actor authentication | Must | critical | An Argus-driven fixture reaches readiness and a review request without invoking approval or landing; a hosted agent approval is refused; local inspection establishes that an actor with the shared account could invoke the operator command and that Pyxis makes no contrary authentication claim. | Test + Inspection | J1, J2, F1, F3 |

### op: supersede FR-46

Reason: The operator accepted a shared-account origin and waived a separate
signing key in the reviewed local-git-codehost contract. Hosted protection
remains the authenticated route.

| ID | Requirement (EARS) | Rationale | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|
| FR-50 | Where the approval surface is hosted, the system shall accept a lock approval only from an authenticated human identity that agent sessions cannot assume. Where the repository explicitly selects local-git, the system shall accept the operator command's declared review receipt satisfying FR-52 and report review authorship and independent origin identity as unauthenticated, with no host-level mainline protection. | A local Git consistency record cannot establish who wrote it or prevent direct writes by the shared account | Must | critical | Hosted self-declared and agent approvals remain refused. A valid local receipt is accepted by the normal route without a signing key; doctor, receipt output, and clone verification disclose the shared-account, unauthenticated-actor, unprotected-mainline, and copied-origin limits. Inspection confirms that a shared-account actor can forge a consistent receipt or write refs directly. | Test + Inspection | G2, J2, F3 |

### op: supersede FR-47

Reason: A local head restored after a revision is a different proposal
generation even when its SHA or contract digest equals the earlier one.

| ID | Requirement (EARS) | Rationale | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|
| FR-51 | The system shall bind a lock approval to the exact draft version presented and void it when the draft changes before lock. In local-git mode it shall also bind approval to the current proposal generation, workflow-derived target ref and existing target SHA, readiness digest, and request nonce, and refuse any mismatch at landing. | Certifies the content and landing context that were actually presented, including a revision that restores an old head | Must | critical | Hosted change-after-request fixtures retain their verdicts. Local changed-head, changed-target-ref, moved-target, changed-digest, changed-proposal-generation, and H1→H2→H1 fixtures refuse the old receipt before any landing write and require fresh preparation and review. | Test | G2, J2, F3 |

### op: add FR

Reason: A once-only, generation-bound receipt makes local approval checkable
from the configured origin without inventing identity authentication.

| ID | Requirement (EARS) | Rationale | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|
| FR-52 | When the operator records a local draft-lock review, the system shall retain a closed, versioned receipt binding proposal identity and kind, proposal-record full OID and monotonic sequence, operation draft-lock, branch ref, exact full prepared-head SHA, workflow-derived fully qualified mainline target ref and its existing full SHA, readiness digest, and a unique request nonce. It shall publish that receipt once, revalidate it against the current pre-landing proposal, and retain its canonical bytes and digest with the final proposal and lock evidence. | Prevents normal-route stale and same-origin replay errors and preserves clone-visible evidence after the review worktree is gone | Must | critical | Real bare-origin fixtures reject missing, malformed, wrong-proposal, wrong-operation, wrong-branch, wrong-head, wrong-target, wrong-digest, and wrong-generation receipts without advancing landing refs; a nonce cannot be overwritten; retry after a completed landing recognizes the original result instead of creating a second lock; a fresh clone detects missing or changed retained receipt bytes and reports consistency with unauthenticated actor and origin identity. | Test + Failure injection | G2, G4, J2, F3 |

### op: rescope WR-4

Reason: The accepted local Git contract requires feature drafts in an
initialized installed repository such as AEC. This admits that scoped route;
it does not migrate existing document-pipeline features or add a general
installed-memory rollout.

| ID | Requirement (EARS) | Rationale | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|
| FR-53 | Where an installed repository explicitly selects local-git and carries the feature-draft memory and readiness capabilities required by its pinned Pyxis release, the system shall permit new feature drafts only after the configured bare origin's mainline exists, and shall apply the same compile, readiness, memory-integrity, and locked-version rules as in the source repository. | Supplies the accepted local AEC workflow without inventing a separate base-free draft path | Must | high | A compatible initialized installed fixture reaches draft preparation and lock; missing-mainline open and preparation refuse before writes with the separate first-mainline-push remedy; an unsupported installed capability names the missing prerequisite; no existing document-pipeline feature is migrated by selecting local-git. | Test | G1, G2, J2, F1, F3 |

### op: add NR

Reason: The local exception must not become a fallback that weakens hosted
approval or the contract's independent publication order.

| ID | Existing behavior protected | Why protected | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|
| NR-6 | Hosted authenticated review, code-owner protection, stale-approval dismissal, latest-head approval, and admin enforcement retain their behavior; local mode is explicit and never selected as an approval fallback | The operator accepted a reduced boundary only for the shared-account local Git workflow | critical | Hosted conformance fixtures retain their verdicts; a failed hosted approval cannot be satisfied by a local receipt; this contract-only amendment and its approved successor design merge independently before a later proposal enables local selection or receipts. | Test + Inspection | FR-48, FR-50, FR-51 |

### op: supersede UB-3

Reason: Refusal of agent identity is enforceable on the hosted surface. A
local origin enforces receipt consistency and discloses its actor limit.

| ID | If… (trigger) | Then the system shall… | User sees | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|---|
| UB-12 | If a hosted lock approval is attempted by an agent identity, Argus included, or a local lock lacks a valid FR-52 receipt | refuse the lock and request the required operator approval evidence | the refusal naming the missing or invalid evidence and, for local mode, its unauthenticated-actor limit | Must | critical | Every hosted agent-approval fixture and every invalid local receipt fixture refuses without advancing landing refs; no fixture claims that a consistent local receipt proves human authorship | Test + Inspection | FR-48, FR-49, FR-50, FR-52, F3 |

### op: supersede UB-11

Reason: Local freshness includes proposal generation and landing context,
so restoring a previous head cannot restore its previous approval.

| ID | If… (trigger) | Then the system shall… | User sees | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|---|
| UB-13 | If a draft changes after its approval request and before lock, or a local receipt's proposal generation, target, digest, or nonce no longer matches the current request | void the approval and require fresh preparation and operator review of the current request | the changed field, the stale approval, and the prepare-and-review remedy | Must | critical | Hosted change-after-request behavior is preserved; each local freshness mismatch, H1→H2→H1 included, refuses before landing writes, and a fresh matching receipt permits the normal route | Test | FR-51, FR-52, F3 |

### op: supersede NFR-1

Reason: The abuse matrix must distinguish authenticated hosted identity
checks from consistency checks within the accepted local route.

| ID | Area | Requirement | Fit criterion | Verify |
|---|---|---|---|---|
| NFR-5 | Security | The system shall refuse every normal-route lock, draft-born status change, and locked-version change that lacks its required evidence, and surface every unruled requirement change in the approval request. Hosted actor authority shall remain authenticated; local approval shall enforce FR-50 through FR-52 with the disclosed shared-account limit. | Hosted ABUSE-2 through ABUSE-8 fixtures remain refused; local readiness, stale/replay, status, and immutable-version fixtures refuse inconsistent evidence; local actor-forgery and direct-ref-write inspection records the accepted lack of protection; the ABUSE-1 silent rewrite remains marked unruled. | Test + Inspection |

### op: supersede DOD-1

Reason: The current Must requirements include the scoped local successors.

| ID | Done criterion | Source | Verify | Required evidence | Blocking? |
|---|---|---|---|---|---|
| DOD-11 | Every active Must FR passes its fit criterion | active FR-1 through FR-55 | Test + Inspection | acceptance corpus results for hosted and explicit local modes | yes |

### op: supersede DOD-3

Reason: A local actor-forgery inspection must not be reported as a successful
identity-refusal test.

| ID | Done criterion | Source | Verify | Required evidence | Blocking? |
|---|---|---|---|---|---|
| DOD-12 | Every active unwanted-behavior row and mode-scoped abuse row satisfies its stated refusal or accepted-limit criterion | active UB-1 through UB-14; ABUSE-1 through ABUSE-8; NFR-5 | Test + Failure injection + Inspection | hosted refusal matrix; local receipt/freshness matrix and shared-account limit inspection | yes |

### op: supersede DOD-4

Reason: The local end-to-end proof must name its declared evidence boundary
and the initialized-mainline prerequisite.

| ID | Done criterion | Source | Verify | Required evidence | Blocking? |
|---|---|---|---|---|---|
| DOD-13 | An end-to-end fixture goes from an opened draft through rounds, refused lock, ruled requirement change, ready lock with the selected medium's approval evidence, plan generation, projection, and executor pickup; explicit local mode starts with an existing mainline and verifies retained receipt and landing consistency from a fresh clone | active FR-1 through FR-25; active FR-37 through FR-53; NFR-3; NFR-4 | Test + Demo | hosted end-to-end regression; initialized bare-origin run and fresh-clone consistency verification with actor/origin authentication limits reported | yes |

### op: supersede DOD-7

Reason: The Argus acceptance reference follows its scoped successor.

| ID | Done criterion | Source | Verify | Required evidence | Blocking? |
|---|---|---|---|---|---|
| DOD-14 | Stage skills, Argus, agent instructions, pipeline map, and domain model work with or describe the feature draft and its selected approval boundary | FR-34; FR-49; FR-36; FR-50 | Inspection + Demo | skill runs and operator-reviewed diffs; local request without an Argus approval or landing invocation | yes |

### op: add DOD

Reason: The approved plan orders this independently reviewed contract before
the enabling implementation.

| ID | Done criterion | Source | Verify | Required evidence | Blocking? |
|---|---|---|---|---|---|
| DOD-15 | This scoped amendment and its successor design receive exact-source operator review and land in a contract-only proposal before any later proposal enables local code-host selection or receipt acceptance | FR-50; FR-52; NR-6 | Inspection | operator approval, independent mainline merge, and later implementation references to the merged contract | yes |

### op: supersede §0

Reason: The approval call now routes the operator to the local trust boundary and the contract-first publication order as well as the existing lifecycle.

*The body is the contract; this section only routes attention — on any conflict the body governs; machines ignore it.*

**Judge these**

| # | The call | Could have gone | If you disagree |
|---|---|---|---|
| 1 | New feature contracts use drafts when the release lands, with no pilot or switch back. The approved Pyxis-source legacy-gate bypass remains; enabled document-pipeline repositories retain applicable automated checks. | Pilot one feature first, keep an operator-only switch to reopen it. | A draft-process defect blocks all new feature contracting until fixed → NG-6, UB-9, R2, §6.5. |
| 2 | This PRD overrides eight rows in five other features' PRDs for draft-born features; those ledgers stay unamended. | One amendment per sibling ledger, or one governing ruling. | Readers of those ledgers see stale rules and PRD review will flag it → FR-31, FR-32, FR-33, NG-5, R1. |
| 3 | The repository memory becomes the authoritative home of a draft-born contract; no document source is required. | Keep documents as the source and memory as their mirror. | The memory gate keeps refusing native contracts → FR-3, FR-29, NR-4, ABUSE-4. |
| 4 | PRD and tech spec lock together after readiness and operator review. Hosted review is authenticated; explicit local-git accepts a declared receipt whose actor is unauthenticated. | Require a separate local signing key or protected host; keep per-stage freezes. | The accepted shared-account workflow changes → FR-48–FR-53, NR-6, §5.3; the plan still follows the lock. |
| 5 | Inside a draft, only a product-side ruling changes a requirement; engineering-side sessions record problems. | Let design sessions edit requirements and review changes at the lock. | Requirements drift silently from product intent → FR-8, FR-9, FR-10, UB-1, ABUSE-1. |
| 6 | Day one: new features start as drafts; existing features are not migrated in this release, and their documents stay frozen history. | Migrate existing features when they next change; convert all at once. | Existing-feature documents keep their authoring rules; automatic migration remains excluded → NG-4, WR-5, R3, FR-54, NR-7. |
| 7 | A feature cannot be reopened while its current plan has unfinished tasks. | Mark each unfinished task keep, replace, or cancel inside the reopen. | A task known to be wrong is built or cancelled by hand first → FR-43, NG-8, R8. |

**Sanity-check the numbers** — baseline median 3 proposals to a first plan, maximum 9, across 25 features → target exactly 1 proposal per draft for each of the first three draft-born features; zero reopens caused by gaps the lock should have caught; zero new document-corpus files for draft-born features; 100% of in-draft requirement changes carrying a ruling → M1, M2, M3, M4, §6.7.

**The author's open questions** — Q1 (the `docs/techspec.md` naming clash), Q3 (whether gates also refuse new brainstorm and review documents), Q11 (what abandoning an open draft does); Q4–Q10 carry the brainstorm's technical constraints to the technical design → §7.2.

**Skip the rest** — structure and citation resolution → docgate; requirement quality and coherence → prd-review; mechanisms (proposal, merge, memory schema, projection input) → technical design; implementation defects → code-gate and the acceptance corpus.

The local exception requires an existing origin mainline (FR-53); it preserves generation, head, target, digest, and once-only nonce checks (FR-51, FR-52), while copied-origin identity and protection against direct shared-account writes remain unavailable (FR-50). Its contract-only proposal lands before enabling code (DOD-15).

### op: supersede §4.4

Reason: The scenario must not promise actor authentication that the accepted local origin cannot supply.

The operator starts a model-routing feature. In round 1 the PRD requires an
explicit no-inference mode; in the same round a tech-spec brainstorm session
finds that the executor has no such mode and records a requirement problem.
The operator rules the requirement out and keeps real zero-work behavior as a
guarantee; the PRD changes in the same draft. In round 2 the tech spec
takes shape and both reviewers run. The first lock request is refused:
two Must requirements have no tech-spec decision. The tech spec is revised, the
reviewers re-run on the new version, readiness passes, and the approval request
lists the one requirement change with its ruling. The operator approves, v1
locks, and a plan generated from it is projected and built by today's executor
— one contract proposal and one plan proposal where the same feature once
needed nine.

Weeks later, with v1's tasks all delivered, another feature's change affects
this one. The operator reopens it; v1 stays binding while the reopened draft
runs one round and locks as v2. A second open draft that depends on a decision
v2 changed is refused at its own lock attempt, because its readiness runs
against the mainline that now holds v2, and is fixed in its own draft.

On a hosted approval surface, an agent session's approval is refused. In an
explicit local-git repository, Argus presents the prepared review request and
the operator records a declared receipt and invokes landing. A missing,
inconsistent, or stale receipt is refused by that route. The shared OS account
cannot establish which actor issued a consistent receipt or prevent a direct
Git write; output discloses both limits. Restoring an earlier head after a
revision still requires a fresh receipt for the new proposal generation.

If a defect in the draft process appears after release, new feature
contracting waits for its fix: the document pipeline does not reopen
(brainstorm:feature-draft#D-17).

**Decision:** Solution gate — contributors sign that this shape, at this appetite, is the bet. Zero TBDs may pass this gate.

| Reviewer | Role | Date | Verdict |
|---|---|---|---|
| @mdarabi | Operator — product and engineering | 2026-09-22 | approved — the same review event as §2.6, whose row carries its one capture line |
| @mdarabi | Operator — product and engineering | 2026-09-23 | approved — the same review event as §2.6's 2026-09-23 row |

### op: supersede §5.3

Reason: Scope identity-refusal claims to hosted approvals and state the accepted limits of local workflow permissions.

| Function / data | Role: operator | Role: agent session or Argus | Unauthenticated |
|---|---|---|---|
| Open or revise a feature draft | read/write | read/write, for the operator | none |
| Rule on a requirement problem | write | Argus under grant only; other agents none | none |
| Approve a lock | hosted: authenticated human review; explicit local-git: declared operator receipt and operator-invoked landing | none under the workflow; local OS-account separation is not enforced | hosted: none; local actor identity is unauthenticated |
| Change a locked version | only through a reopen | only through a reopen | none |
| Change which features are draft-born | write, through a reviewed mainline change | none | none |
| Edit an existing document-pipeline document | none | none | none |

| ID | Actor and intent | Vector | Impact | Severity | Mitigated by | Verify |
|---|---|---|---|---|---|---|
| ABUSE-1 | An agent session wants an easier tech spec or plan | silently rewrites a requirement during a round | the contract drifts from product intent | high | FR-8, FR-9, FR-18, UB-1 | approval-request listing test: a silent rewrite appears marked unruled |
| ABUSE-2 | An agent, or Argus beyond its grant, wants its own work locked | approves its own lock | an unread contract reaches execution | high | hosted: FR-48, FR-49, UB-12; local: FR-49, FR-50 | hosted agent-identity refusal fixtures; local inspection records that the workflow does not authenticate a shared-account actor |
| ABUSE-3 | A session wants a quick lock | revises the draft after its reviews and readiness passed | a stale review certifies different content | high | FR-16, UB-2, UB-8 | post-review revision fixtures |
| ABUSE-4 | A proposal wants to escape document checks or memory authorization | declares itself draft-born in the same proposal that relies on it | candidate-selected weaker enforcement | high | UB-6, FR-29, FR-54, NR-4 | same-proposal status fixtures |
| ABUSE-5 | A session wants to skip a reopen's readiness and approval | edits a locked version in place | an unapproved contract change binds execution | high | FR-19, UB-4 | in-place edit fixtures on locked parts |
| ABUSE-6 | A proposal wants the overrides for an existing feature | cites FR-31 through FR-33 for a feature still in the document pipeline | the feature's existing contract or applicable document checks are bypassed | med | FR-54, NR-7, NR-5 | existing-feature override fixtures in enabled repositories; source-bypass inspection |
| ABUSE-7 | A session wants work started before approval | plans or executes from an open draft | execution acts on an unapproved contract | high | FR-5, FR-6 | open-draft planning and execution fixtures |
| ABUSE-8 | An agent session wants a lock without the operator | writes an approval under the operator's identity, such as a record whose self-declared author is the operator | an unread contract reaches execution under a forged approval | high | hosted: FR-50, FR-51, UB-13; local: FR-50–FR-52 | hosted forged-approval refusal fixtures; local inconsistent-receipt refusal fixtures and accepted-limit inspection for a consistently forged receipt |

In explicit local-git mode these permissions describe the Pyxis workflow. The
operator command accepts a declared review, not an authenticated human
identity. The shared account can edit origin hooks and refs or forge internally
consistent records; no signing key or host-level mainline protection is
required. A copied origin can preserve the same records. Verification proves
consistency within the configured origin, not human authorship or independent
origin identity (FR-50). Readiness, memory integrity, receipt, and applicable document checks continue inside the normal
Pyxis route; the source repository's legacy document checks remain disabled (FR-54, NR-7), and they make no claim to prevent direct writes outside it.

### op: supersede §5.4

Reason: The approval data dictionary must distinguish authenticated hosted review from the retained local declaration.

| Entity / field | Meaning | Source of truth | Retention | Integrity rules | Privacy class |
|---|---|---|---|---|---|
| feature draft | the open workspace of one feature's PRD and tech spec, with their brainstorm records and review reports, until its lock | working files on the draft's branch, off the mainline until the lock | until its lock or abandonment | parts revisable only while open | internal |
| locked version | the PRD and tech spec, with their brainstorm records and review reports, as approved at one lock | the repository memory on the mainline, each file's exact text included | permanent; replaced versions stay as history | immutable; numbered in sequence per feature | internal |
| requirement problem | an engineering-side finding against one requirement | the repository memory | permanent with the locked version | answered by one ruling before the lock | internal |
| ruling | a product-side decision that changes a requirement | the repository memory | permanent | records author, reason, and triggering problem | internal |
| lock approval | the operator's review of a ready draft, with medium-specific evidence | hosted: repository memory and authenticated approval surface; local: configured origin receipt, final proposal, and lock evidence | permanent | hosted authenticated identity; local declared reviewer in both roles, unauthenticated actor/origin identity, exact proposal generation/head/target/digest/nonce binding; capture line or lineless mark | internal |
| draft-born status | whether a feature is contracted through feature drafts | the repository mainline | permanent | changes only through a reviewed mainline change that lands before use | internal |

- **Migration** — none in this release (NG-4): existing features keep their frozen documents, and moving them into drafts waits in WR-5.
- **Parallel-run / cutover** — no parallel run for new contract work: the authoring workflow moves to drafts at release, with automated legacy refusals only where that gate remains enabled (FR-55); existing documents stay as frozen history (NR-7).
- **Rollback** — none by the operator's ruling (NG-6); defects are fixed forward, and locked versions stay valid regardless.
- **Decommission** — at release, Pyxis's own repository stops producing PRD ledgers, technical designs, and brainstorm and review documents for new contract work, and plans stay plan documents.
- **Privacy check** — no DPIA trigger: no personal data beyond operator identities already recorded with commits and approvals.
- **Egress** — no new egress: agent sessions and reviewers send draft parts to the same model providers the document pipeline already sends documents to, under unchanged settings.

### op: supersede §5.5

Reason: Make the consumer approval port's two evidence contracts explicit without a fallback between them.

| Interface | Direction | Purpose | Contract (format, ranges, timing) | Failure modes | Owner |
|---|---|---|---|---|---|
| Repository memory | both | holds draft parts, requirement problems, rulings, approvals, and locked versions | typed immutable records; mainline history only after a lock | unreadable or invalid part → UB-7 | Pyxis memory |
| Approval surface | in | carries the operator's lock approval | hosted: authenticated human identity; explicit local-git: closed once-only declared receipt bound to current proposal generation, head, workflow-derived target, readiness digest, and nonce | hosted agent identity or invalid local receipt → UB-12; stale approval → UB-13; local actor/origin authentication unavailable → FR-50 disclosure | operator |
| Plan projection to the tracker | out | hands a plan generated from a locked draft to execution | today's task form (NR-1) | projection refusal → UB-10; the plan is checked against the projection's rules when generated (FR-37) | Pyxis planning |
| Unmanned reviewers | both | review the draft's PRD and tech spec per round | verdicts and findings stored as review documents in the draft | a missing final review → readiness refuses (FR-16) | reviewer program |
| Argus | both | drives rounds, rules under grant, requests the lock | grant checked at act time | ungranted act → requested from the operator (FR-10) | Argus |
| Agentic CLIs | both | run the stage skills inside a draft | skills read and write the draft only (FR-34) | a failing step → UB-9 | stage skills |

### op: supersede §8

Reason: Refresh the approval and DoD references and distinguish the pending local amendment review from historical gate signatures.

| Check | Pass? | Notes |
|---|---|---|
| Problem basis accepted — §1 problem accepted (T1); §2 problem gate signed (T2/T3) | yes | §2.6 signed 2026-09-22 by the operator |
| Zero solution-blocking TBDs or open questions (§7.2) | yes | Q1–Q11 are owned by the technical design and block no solution-gate call; the operator signed the solution gate with them open |
| Every Must FR has a fit criterion and a verification method (§5.1) | yes | every FR row carries both; FR-23 is the one Should row |
| Every requirement row carries a risk tier; high/critical rows have a declared safety plan — staged rollout, rollback rehearsal, kill switch, migration fallback, manual recovery, or an explicit non-applicability rationale (§5.1, §6.5) | yes | every FR, NR, and UB row carries a tier; staged rollout and kill switch are ruled out (brainstorm:feature-draft#D-17) — the declared plan is pre-release proof (all active blocking §5.8 rows) plus fix-forward with draft content preserved (UB-9) |
| Non-goals drawn (§3.4); non-regression rows named (§5.1) | yes | NG-1 through NG-8; active NR-1 through NR-7 |
| Every primary job (§2.3) is covered by a requirement, a non-goal, or a waiting-room entry | yes | J1 by FR-8–FR-10, FR-33; J2 by active FR-11–FR-19, FR-31, FR-37–FR-42, FR-48, FR-50–FR-53; J3 by FR-20–FR-22, FR-43, FR-44; J4 by FR-23, FR-24, FR-38 |
| Every key flow (§4.3) has functional, failure, and non-regression coverage (§5.1–5.2) | yes | F1: FR-1–FR-5, FR-34, UB-9, NR-4; F2: FR-8, FR-9, UB-1, NR-4; F3: FR-11–FR-19, FR-37–FR-51, UB-2, UB-12, UB-7, UB-10, UB-13, NR-1; F4: FR-20–FR-22, FR-43, FR-44, UB-4, NR-5; F5: FR-23, FR-24, UB-8, NR-4; F6: FR-55, FR-54, UB-14, NR-7, NR-5 |
| Every primary metric (§3.2) has instrumentation (§5.7) or an explicit measurement plan | yes | M1 by EVT-1 and EVT-5; M2 by EVT-6; M3 by EVT-7; M4 by EVT-3 |
| Every blocking DoD row traces to a requirement, guardrail, release criterion, or human-judgment rubric (§5.8) | yes | all active blocking §5.8 rows cite their sources |
| Risks and assumptions have owners (§6.3) | yes | R1 through R8 name owners |
| Human-judgment criteria name an owner and a rubric (§5.8) | yes | DOD-9 and DOD-10 name @mdarabi and their rubrics |
| DoD complete: every blocking row sourced and evidenced; every n/a explained (§5.8) | yes | DOD-10 is non-blocking by design, recorded after release |

The original gate signatures above remain historical. This amendment's exact
source review is pending; it authorizes no implementation until it and the
successor design have independently merged (NR-6, DOD-15). Local approval
readiness includes the FR-50 accepted-limit disclosure and FR-51/FR-52
consistency checks, never a claim of actor authentication.

### op: supersede FR-30

Reason: Preserve the operator-approved source-repository legacy-gate bypass while retaining applicable checks in installed document-pipeline repositories.

| ID | Requirement (EARS) | Rationale | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|
| FR-54 | While the switch-over is in effect, the system shall apply automated document-pipeline checks only to existing documents in repositories where those checks remain enabled, and shall apply none to draft-born contract parts. In the Pyxis source repository it shall preserve the approved memory-migration bypass; selecting local-git shall not restore legacy document checks. | Frozen-document authoring rules survive independently of the operator-disabled source gate | Must | critical | Enabled document-pipeline fixtures retain existing-document immutability refusals and produce zero such findings for draft-born parts. A trusted Pyxis-source fixture continues to bypass legacy document checks at push and CI in both hosted and local mode; retained non-document checks still run. Inspection confirms that source authoring rules for frozen documents remain binding. | Test + Inspection | G3, F6 |

### op: supersede FR-27

Reason: Distinguish the required new-feature authoring path from automated enforcement that the operator disabled in Pyxis source.

| ID | Requirement (EARS) | Rationale | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|
| FR-55 | While the switch-over is in effect, the feature-authoring workflow shall use feature drafts for new feature contracts. Where the legacy document gate remains enabled for that cutover, the system shall refuse new-feature contract work proposed as PRD ledger entries, technical designs, brainstorm records, or review reports with the feature-draft remedy, while accepting plan documents and the permitted evolution of existing document-pipeline features. The Pyxis-source bypass in FR-54 shall remain in effect. | Move new contracts to memory without reinstating the retired source gate or changing existing-feature authoring rules | Must | critical | An enabled-cutover new-feature fixture refuses each corpus-document kind, including a proposal opened before cutover, and accepts a plan; a permitted existing-feature amendment or successor remains accepted. Source inspection and gate fixtures establish that the workflow directs new features to drafts without activating automated legacy refusals. Existing merged features retain permitted typed amendments and design supersession under their authoring rules; no feature is automatically migrated. | Test + Inspection | G3, F6 |

### op: supersede NR-3

Reason: Frozen bytes and prompt-level mutability rules are preserved; automated refusal applies only where the legacy pipeline remains enabled.

| ID | Existing behavior protected | Why protected | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|
| NR-7 | Existing document-pipeline documents keep their bytes through the switch-over and their merged-document authoring rules, permitted typed amendments, supersession, and sanctioned retirement paths. Automated legacy immutability and append-only checks remain active only in repositories where that pipeline is enabled; Pyxis source keeps its approved bypass. | Preserve historical authority without restoring checks the operator disabled during memory migration | critical | Existing documents are byte-identical through cutover. Enabled-repository immutability fixtures still refuse edits and sanctioned retirement fixtures still pass. Source gate fixtures retain the bypass, while source instructions continue to forbid edits to frozen documents and permit their established evolution routes. | Test + Inspection | FR-54, FR-55, F6 |

### op: supersede UB-5

Reason: Scope the automated new-document refusal to repositories where that enforcement is enabled.

| ID | If… (trigger) | Then the system shall… | User sees | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|---|
| UB-14 | If new-feature contract work is proposed as a PRD ledger entry, technical design, brainstorm record, or review report during cutover in a repository whose legacy document gate remains enabled, including a proposal opened before cutover | refuse it, naming the feature draft as the path; preserve the Pyxis-source bypass and permitted existing-feature evolution | in an enabled repository, the refusal with the draft remedy; in Pyxis source, no restored legacy-gate finding | Must | high | Enabled-cutover new-feature fixtures refuse each corpus-document kind with the remedy and accept plans; permitted existing-feature amendments and successors remain accepted. A Pyxis-source fixture does not produce an automated legacy finding for those files, and source authoring instructions still direct new features to drafts. | Test + Inspection | FR-54, FR-55, F6 |

### op: supersede DOD-2

Reason: Non-regression evidence includes the scoped local exception and the existing source-gate bypass.

| ID | Done criterion | Source | Verify | Required evidence | Blocking? |
|---|---|---|---|---|---|
| DOD-16 | Every active non-regression row passes its scoped fit criterion | active NR-1 through NR-7 | Test + Inspection | hosted conformance, existing-feature and memory-integrity fixtures, enabled-document-pipeline regressions, and source-bypass inspection | yes |

### op: supersede DOD-5

Reason: The switch-over proof must not treat the disabled source gate as an active release requirement.

| ID | Done criterion | Source | Verify | Required evidence | Blocking? |
|---|---|---|---|---|---|
| DOD-17 | New feature authoring uses drafts, applicable enabled-repository cutover checks pass, and the approved Pyxis-source legacy-gate bypass remains in force | FR-54, FR-55, UB-14, NR-7 | Test + Inspection | enabled-cutover refusal fixtures, source gate fixtures, and inspection of new-feature guidance and frozen-document authoring rules | yes |

### op: supersede §1

Reason: The summary must admit the narrow installed-repository exception already authorized by FR-53.

- **Problem** — A feature's contract is authored as a relay of separately landed stages — brainstorm, PRD, design brainstorm, design, reviews, plan — and each stage freezes when it lands. When design or planning shows that a requirement was wrong or missing, the correction costs another proposal after the earlier stage was already treated as done, and every authored document is also mirrored into the repository memory, whose own checks refuse contract content written there directly.
- **For whom** — The Pyxis operator, one human holding both the product and the engineering sides of a feature and working with agent sessions; secondarily Argus when it drives a feature for the operator.
- **Why now** — The repository memory now holds typed, immutable claims with supersession, rendering, and proposal views of product and design deltas; the document pipeline keeps growing workarounds around it (staged-contract rules, byte-for-byte source captures); every new feature pays at least one proposal per stage.
- **Direction** — One feature draft per feature holds the PRD and tech spec, with their brainstorm records and review reports. Rounds revise any part; a requirement changes only by a product-side ruling. Readiness checks gate one human lock of the PRD and tech spec together, which writes them into the repository memory; a plan is generated from the locked draft afterwards, when wanted, and stays the plan document that connects it to execution. Any later change reopens the feature as a new draft.
- **Appetite** — The complete process for new features in Pyxis's own repository, plus the FR-53 exception for compatible initialized installed repositories explicitly selecting local-git, on existing machinery; excludes changes to the execution handoff, other intake lanes, migrating existing features, a general installed-memory rollout, and governing documents.
- **Success** — Proposals from a draft's opening to its lock: today a median of 3 to a first plan (maximum 9) → exactly 1 for each of the first three draft-born features (M1).
- **Guardrail** — Zero reopens of those three features caused by a gap the readiness checks should have caught, judged once all three are implemented (M2); memory integrity never weakened (§3.3).
- **Top risks** — a draft-process defect blocks all new feature contracting, since nothing can switch back (R2); the unamended sibling ledgers keep stating the old rules and PRD review flags the contradiction (R1); existing features require their existing authoring and evolution rules until migration is designed (R3).

### op: supersede §2.2

Reason: Keep the original audience while removing the blanket exclusion of the accepted AEC route.

| Segment | Role and context | Domain exp. | Tech exp. | Key goals | Pain today | Evidence |
|---|---|---|---|---|---|---|
| **P1 (primary)** | The Pyxis operator — one human holding both the product and the engineering sides of each feature, taking part in the rounds while agent sessions write the artifacts | master | master | define the right contract; correct it as soon as design or planning teaches something; approve once | at least one proposal per stage; a further proposal per correction after a stage froze | brainstorm:feature-draft#D-4 · §2.4 rows 1–3 |
| P2 (secondary) | Argus, the AI engineer, driving a feature for the operator under its authority contract | journeyman | master | run the contract loop with rulings where granted; escalate breaking decisions; request approval | a stage-by-stage drive in which every change must travel an amendment, supersession, or reopen-record channel | brainstorm:feature-draft#D-4 · prd:argus/FR-4 |
| **Anti-segment** | Authors of bug fixes, performance work, or other non-feature changes (later intake lanes); installed repositories outside FR-53's compatible initialized local-git exception; teams that require two different approvers (not in the first version) | — | — | — | — | brainstorm:feature-draft#D-8, brainstorm:feature-draft#D-26, brainstorm:feature-draft#O-5 |

FR-53 also serves the same solo operator and Argus in a compatible initialized installed local-git repository. It adds no team approval model or general memory rollout.

### op: supersede §4.2

Reason: Align the scope perimeter with installed local drafts and the medium-specific approval and gate boundaries.

- **In (plan of record)** — the feature draft and its rounds; brainstorm records and review reports as separate documents inside the draft; storage of every part in the repository memory at the lock, each file's exact text included; draft-born PRDs held to the PRD template, §0 included; requirement problems and product rulings, retirement included; readiness checks — the planning gate's currency, link, structural, and consistency checks included — computed against the mainline each lock lands on; the operator-reviewed lock with the selected medium's version-bound evidence and its capture line: authenticated hosted approval, or FR-50–FR-52's declared local receipt; locked versions; reopen, refused while the current plan has unfinished tasks; memory's in-flight notice as an advisory heads-up; plans generated from locked drafts, checked for coverage and projection rules and fed to today's projection; the new-feature authoring switch-over and FR-54/FR-55's gate applicability; compatible initialized installed local-git repositories under FR-53; memory integrity that treats draft-born contracts as authoritative; the overrides of the enumerated sibling rows; reworked stage skills; Argus as a driver; updated agent instructions, pipeline map, and domain model.
- **Future (believed next, not committed)** — the whole-pack execution handoff (WR-1); the intake step and other lanes (WR-2); two-approver teams (WR-3); a general installed-memory rollout beyond the scoped FR-53 exception; migrating existing features (WR-5).
- **Out** — governing-document edits (NG-1); automatic locks (NG-2); amendments for draft-born features (NG-3); bulk conversion or a permanent dual pipeline (NG-4); sibling-ledger amendments (NG-5); a pilot stage or a switch back (NG-6); overlap tracking between open drafts (NG-7); in-reopen task dispositions (NG-8).

Terms used below: a **feature draft** is the open, revisable workspace of one feature's contract until its lock; a **round** is one pass of feedback in which any part may be revised; the **lock** is the single human approval that ends a draft; a **locked version** (v1, v2, …) is what a lock produces; a **reopen** starts a new feature draft from a locked version; the **tech spec** is the per-feature technical design; a **draft-born feature** is any feature contracted through a feature draft — every feature started after the switch-over; the **switch-over** is the moment the release carrying this feature lands in Pyxis's own repository; the **document pipeline** is the per-stage process in which brainstorm records, PRD ledgers, technical designs, review reports, and plans are separate documents, each landing in its own proposal; **readiness** is the set of lock preconditions in FR-12 through FR-14, FR-16, and FR-38 through FR-42.

The local exception requires the pinned release's feature-draft memory/readiness capabilities and an existing configured-origin mainline. Its approval is a declared receipt, not authenticated human authorship. Existing features are not migrated. The approved source legacy-gate bypass remains; local selection changes the code-host medium, not document-gate policy.

### op: supersede §4.3

Reason: The main lock flow must use medium-specific evidence; the cutover flow must preserve source-gate policy.

**F1 — draft a new feature (serves J1, J2).** The operator, or an agent
session or Argus acting for the operator, opens a feature draft. Brainstorm
sessions, the PRD, tech-spec brainstorm sessions, the tech spec, and review
rounds all write into that one draft; each brainstorm session and each
review round is its own document. Any part may change in any round, and none
of it reaches the mainline, planning, or execution until the lock.

**F2 — correct a requirement during a draft (serves J1).** A tech-spec
session finds that a requirement is wrong, missing, or unbuildable.
The session records a requirement problem against it, and the requirement
stays unchanged. The operator — or Argus where its authority contract grants
it — rules; the ruled change — a revision, an addition, or a retirement — is
applied to the PRD in the same draft with its author, reason, and triggering
problem, and the tech spec adjusts in
later rounds.

**F3 — lock (serves J2).** A lock is requested. Readiness runs against the
mainline the lock would land on and either refuses, naming every unmet
condition, or passes; a passing draft produces an approval request that shows
the PRD with its §0, the readiness result, and every requirement change made
during the draft with its ruling. Hosted review requires an authenticated
human identity. Explicit local-git mode presents the current proposal generation,
full prepared head, workflow-derived target ref and existing SHA, readiness
digest, and request nonce. The operator records a matching declared receipt
and invokes landing; its output discloses unauthenticated actor/origin identity
and no host-level mainline protection. The approval binds the presented version
and records the capture line when one is given. Any local binding mismatch,
including a revision that restores an old head, requires fresh preparation and
review before any landing write. The PRD and tech spec become the feature's
next locked version together and are written into the repository memory; a
plan generated from it afterwards goes to today's plan projection.

**F4 — change a locked feature (serves J3).** A change opens a new feature
draft starting from the current locked version — refused while that version's
plan has unfinished tasks, which finish or are cancelled first; finished tasks
keep tracing the version they delivered. The locked version stays binding for
planning and execution meanwhile. The reopened draft runs rounds and locks
under the same rules; its locked version replaces the previous one, which
stays readable as history. A reopen can retire requirements, which is how an
abandoned feature's requirements reach a typed terminal state.

**F5 — parallel drafts (serves J4).** Open drafts do not track each other.
Each lock's readiness runs against the mainline the lock lands on, so the
second draft to lock is checked against everything the first locked; a
conflict refuses it and is fixed in its own draft. Memory's existing notice of
other open proposals touching the same requirements or decisions gives an
optional early heads-up.

**F6 — switch-over (serves J1, J2).** New feature contracts use drafts.
In repositories where the legacy document gate remains enabled for cutover,
new-feature work proposed as PRD ledger entries, technical designs, brainstorm
records, or review reports is refused with that remedy, including proposals
already open at cutover; plans and permitted existing-feature evolution remain
accepted. The approved Pyxis-source bypass stays in
force, with frozen-document authoring rules and permitted existing-feature
evolution unchanged (FR-54, FR-55, NR-7). Existing documents remain readable,
and existing features are not automatically moved into drafts (WR-5).

- **Ideal** — each flow completes with its parts, rulings, readiness result, and locked version retrievable from the repository memory.
- **Empty** — a feature with no draft and no locked version reports no contract state; a newly opened draft reports each part as not yet written, and readiness names them all.
- **Loading** — not applicable; each step is a bounded synchronous repository operation or an agent session the operator runs.
- **Partial** — a draft with some parts written reports the missing parts and unmet readiness conditions; nothing locks partially.
- **Error** — every refusal names the failing condition, the exact item, and the next action, and leaves the draft's content unchanged.
- **Accessibility** — command output, approval requests, and rendered parts are plain text readable by screen readers and never rely on color alone.
- **Localization** — operator-facing text is English-only; no customer-facing strings ship.

### op: supersede §6.1

Reason: Build and release completion must cover all active requirements and DoD rows, including the local exception.

| Phase | Purpose | Scope | Exit criterion |
|---|---|---|---|
| Contract | Approve this scoped extension through the existing feature's typed amendment and successor design | this amendment and successor; no runtime activation | exact-source operator approval and independent contract-only mainline merge precede local selection and receipts (NR-6, DOD-15) |
| Build | Deliver every active Must requirement on a release candidate | all active Must FRs through FR-55, active non-regression and unwanted-behavior rows, and their scoped fixtures | every active blocking §5.8 DoD row is green, with hosted and local evidence at their declared boundaries |
| Release | Ship the new-feature draft workflow and compatible installed local-git capability | source authoring switch-over, retained source-gate bypass, and FR-53-compatible initialized installed repositories | all §6.4 criteria and active blocking DoDs hold on the released tree; DOD-17 confirms gate applicability |
| Checkpoint | Judge the original bet | the first three draft-born features | DOD-10 recorded |

### op: supersede §6.3

Reason: Risk mitigations must use active evidence rows and distinguish authoring rules from disabled automated checks.

| # | Risk / assumption / rabbit hole | Likelihood | Impact | Mitigation or test | Owner |
|---|---|---|---|---|---|
| R1 | PRD review flags the override of eight rows in five sibling ledgers as a contradiction, and readers of those ledgers still see the old rules | H | M | FR-31 through FR-33 name each row and scope the override to draft-born features; a flagged finding is declined with brainstorm:feature-draft#D-15 as the ruling | @mdarabi |
| R2 | With no pilot and no switch back, a draft-process defect blocks all new feature contracting until fixed | M | H | DOD-13 end-to-end proof and fresh-clone verification before release; UB-9 keeps draft content intact; fix forward as top priority | @mdarabi |
| R3 | Existing features stay in the document pipeline and must use its established evolution rules until migration is designed | H | M | Existing documents keep their authoring rules and bindings; automated legacy checks remain only where enabled, and the approved Pyxis-source bypass remains (FR-54, NR-7, NR-5); moving existing features into drafts waits in WR-5 | @mdarabi |
| R4 | Long-lived drafts meet the memory gate's ancestry and evidence rules | M | M | Q5 — the technical design settles re-evidencing at the lock | tech spec author |
| R5 | Review rounds on a moving draft consume agent sessions repeatedly | M | M | FR-16 binds the required review to the exact locked version; intermediate reviews stay optional; Q10 | tech spec author |
| R6 | Assumption: one proposal per draft is reachable when a draft spans many sessions and rounds | M | M | Measured at the checkpoint (M1); a miss iterates the process | @mdarabi |
| R7 | Several sessions editing one draft at once overwrite each other | M | M | Q9 and Q10 — the technical design settles concurrent editing | tech spec author |
| R8 | With reopens refused while tasks run, a task known to be wrong is built, or cancelled by hand, before the change | M | M | The refusal names each unfinished task and the remedy (FR-43); cancel promptly | @mdarabi |

### op: supersede §6.4

Reason: Release criteria must include every active blocking DoD and the new local receipt and installed-repository proof.

| Lens | Criterion | Evaluated by |
|---|---|---|
| Release completeness | Every active blocking §5.8 DoD row passes; no retired row substitutes for an active criterion | the evidence recorded for every active blocking row, including instrumentation, memory integrity, gate applicability, and independent contract publication |
| Functionality | DOD-11, DOD-13, and DOD-16 pass, covering every active Must FR and non-regression row in the applicable mode | hosted acceptance and initialized installed local-git end-to-end corpus |
| Usability | DOD-9 passes | operator review of the complete approval request and local trust disclosure |
| Reliability | DOD-12 passes with failure injection; NFR-4 and DOD-13 hold from a fresh clone | hosted refusal matrix; local generation/head/target/digest/nonce and replay matrix; retained-receipt and landing consistency verification |
| Performance | Not applicable — steps run at human-paced moments under existing check budgets | — |
| Supportability | NFR-2 holds and DOD-14 passes; the missing-mainline prerequisite has its separate first-push remedy | refusal fixtures and reviewed instruction diffs |

### op: supersede §6.5

Reason: Rollout must describe the scoped installed audience and current evidence without restoring the source gate.

The original no-pilot and no-switch-back ruling remains (NG-6). New features
use drafts in Pyxis source. FR-53 additionally admits compatible initialized
installed repositories explicitly selecting local-git; this selects the
code-host medium, not a way back to the document pipeline or a general memory
rollout. Existing features are not migrated (NG-4, WR-5).

Safety rests on every active blocking §5.8 row passing before release and
fix-forward afterward, with UB-9 preserving draft content through failures.
The reviewed contract-only amendment and successor land independently before
T1 selection and T3 receipt code (NR-6, DOD-15). Candidate proof covers hosted
regressions and the initialized installed local route, including receipt
freshness/replay, retained canonical evidence, and fresh-clone consistency
under the disclosed unauthenticated actor/origin limit.

- **Flag** — no draft-process pilot or switch-back flag (NG-6); local code-host selection is explicit.
- **Kill switch** — none (NG-6); recovery is fix-forward.
- **Experiment** — none; the checkpoint (§6.7) judges the original bet.
- **Cutover/migration steps** — new feature authoring uses drafts; enabled-cutover document repositories apply FR-55, and Pyxis source preserves its approved legacy-gate bypass (FR-54). Existing documents retain their authoring rules and bindings, with no automatic migration.

| Stage | Audience | Minimum evidence | Advance when | Roll back when |
|---|---|---|---|---|
| Release | Pyxis source and FR-53-compatible initialized installed local-git repositories | every active blocking §5.8 DoD row on the release candidate, including DOD-11 through DOD-17, DOD-6, DOD-8, and DOD-9 | all active blocking rows and §6.4 criteria pass; exact-source contract publication precedes activation | never — fix forward on any §3.3 breach (NG-6) |

### op: supersede §6.6

Reason: The installed local exception now has an explicit platform prerequisite and proof owner.

| Area | Item | Owner | Status |
|---|---|---|---|
| Support | refusal diagnostics (NFR-2) | @mdarabi | ☐ |
| Docs | agent instructions, pipeline map, domain model (FR-36); glossary terms; changelog entry | @mdarabi | ☐ |
| Enablement | not applicable — no sales or customer-success surface | — | n/a |
| Legal & privacy | not applicable — no new data class (§5.4) | — | n/a |
| Pricing & billing | not applicable | — | n/a |
| Marketing & comms | release notes state the switch-over and its fix-forward policy | @mdarabi | ☐ |
| Platform | FR-53 compatible installed release pin and initialized origin mainline; local receipt and clone proof; unchanged hosted route | @mdarabi | ☐ |
| Localization | not applicable — English-only operator text | — | n/a |

**Decision:** Launch gate — §6.4 criteria green, §6.6 owned and checked, fix-forward readiness confirmed in place of a rollback rehearsal (NG-6).

| Reviewer | Role | Date | Verdict |
|---|---|---|---|
| @mdarabi | Operator — product and engineering | — | pending, post-implementation |

### op: supersede §7.1

Reason: The explanatory FAQ must not promise an automatic existing-feature migration or contradict its permitted evolution.

**Customer**

- **Q:** What happens to my existing PRDs and designs? — **A:** They stay frozen and readable; their typed amendments, design supersession, and sanctioned retirement rules remain. This release does not migrate them into drafts (WR-5), and the Pyxis-source legacy-gate bypass remains (FR-54, NR-7).
- **Q:** Can I still amend a PRD? — **A:** A draft-born feature changes by reopening its locked version. An existing document-pipeline feature retains its typed-amendment and design-supersession authoring rules; selecting local-git does not migrate it.
- **Q:** Who approves a lock? — **A:** The operator. One approval covers both the product and the engineering roles.
- **Q:** What if the draft process breaks? — **A:** It is fixed forward; there is no switch back to documents.

**Internal**

- **Q:** Doesn't overriding other features' rows break the ledger rules? — **A:** It departs from them knowingly (brainstorm:feature-draft#D-15). FR-31 through FR-33 name each overridden row, and PRD review is expected to flag it (R1).
- **Q:** How does this fit the vision's stage order? — **A:** Design starts from the product side's in-draft rulings and the lock is the single approval; the operator appends a note to vision §4 (brainstorm:feature-draft#D-13).
- **Q:** What are the top ways this fails? — **A:** R2 (a defect blocks all new contracting), R1 (stale sibling rules), R3 (existing features must use their established document-pipeline evolution rules until migration is designed).
