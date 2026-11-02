# Feature implementation drive failure contract — technical design

| Field | Value |
|---|---|
| **Status** | Approved |
| **Owner** | @mdarabi |
| **Approver(s)** | @mdarabi (operator — factory-control failure and recovery surface) |
| **Date** | 2026-07-25 |
| **Relevant-when** | implementing or changing feature-drive failure classification, fail-closed routing, recovery actions, or unwanted-behavior fixtures; triaging invalid kickoff, malformed ownership, stale owners, blocked handoff, unverifiable landing, candidate conflict, blocked e2e, stale evidence, wrong-main delivery, or active cancellation; superseding this failure contract |
| **Scope** | The closed failure-disposition contract for `feature-implementation-drive`: how UB-1 through UB-11 map observed triggers to aggregate blocked/transition states, routing effects, preserved artifacts, user-visible remedies, and recovery preconditions. **Not**: the lifecycle architecture, aggregate/store schema, candidate algorithm, e2e scenario authoring, or merge authority; those remain owned by the design this builds on. |
| **Tracking** | PR #706 |
| **Builds-on** | docs/tech-design/feature-implementation-drive.md |
| **Supersedes** | — |
| **Serves** | `prd:feature-implementation-drive/UB-1` · `prd:feature-implementation-drive/UB-2` · `prd:feature-implementation-drive/UB-3` · `prd:feature-implementation-drive/UB-4` · `prd:feature-implementation-drive/UB-5` · `prd:feature-implementation-drive/UB-6` · `prd:feature-implementation-drive/UB-7` · `prd:feature-implementation-drive/UB-8` · `prd:feature-implementation-drive/UB-9` · `prd:feature-implementation-drive/UB-10` · `prd:feature-implementation-drive/UB-11` · `prd:feature-implementation-drive/NR-6` |
| **Security/privacy impact** | yes — classifies ownership/evidence failures and controls whether autonomous routing, final eligibility, or destructive recovery may proceed; uses the predecessor's identities and credentials without adding a new secret or egress class · reviewer: @mdarabi |
| **Template version** | tech-design-template/v1 |

## 0. Read to rule — the human reviewer's section (~6 min)

*The body is the contract; this section only routes attention — on any conflict the body governs; machines ignore it.*

**Judge these**

| # | The call | Could have gone | If you disagree |
|---|---|---|---|
| 1 | Every unwanted behavior resolves through one closed failure-disposition table over the existing aggregate | Independent adapter-specific recovery logic | Veto the single failure predicate and require another ownership boundary → §2, §3.1 |
| 2 | Recovery clears a failure only after its positive precondition is re-observed; retries and human commands never bypass the failed gate | Clear-on-retry or clear-on-operator-assertion | Accept stale or unverifiable recovery authority → §3.2, `#DINV-2` |
| 3 | Failure handling removes future routing before any uncertain state can execute, while preserving durable artifacts | Best-effort continuation or destructive reset | Weaken fail-closed and artifact-preservation guarantees → §3.1, `#DINV-1`, `#DINV-3` |

**Sanity-check the numbers** — eleven UB rows map one-to-one to eleven fixture families; every blocking disposition produces zero newly routed attempts and zero unauthorized final actions; cancellation/handoff overlap remains zero (A-1–A-6).

**The author's open questions** — none — all recovery authority and product behavior are fixed by the merged PRD and predecessor design (§7).

**Skip the rest** — lifecycle mechanics → `design:feature-implementation-drive`; structure/joins → docgate; substance/coherence → `tech-design-review`; implementation defects → codegate and A-1–A-6.

## 1. Problem

The merged feature-drive design defines the lifecycle and substantively describes every required failure path, but its traceability header did not serve the PRD's Must unwanted-behavior rows. The planning-entry contract gate correctly refuses to infer that coverage from nearby prose: UB-1 through UB-11 have no live design join, so a planner cannot prove that the executable plan covers invalid kickoff, split ownership, quiescence failure, landing failure, candidate conflict, blocked verification, stale evidence, wrong delivery, and active cancellation.

Editing the merged design is forbidden. A successor would be disproportionate because no binding statement is false and no lifecycle decision changes. The independently revisable missing contract is the failure classifier itself: one closed mapping from observed defect to routing posture, durable state, evidence, and recovery gate. This additive design names that mapping explicitly and builds on every lifecycle mechanism rather than duplicating or revising it.

## 2. Decision

**Decision:** The feature drive has one deterministic `FailureDisposition(trigger, observed state) → disposition` predicate whose closed rows are §3.1. A disposition is durable aggregate state, not log prose: it names the blocked/transition state, removes or preserves routing as specified, records exact evidence and one user remedy, and may clear only after §3.2's positive recovery precondition is observed through the predecessor's reconciler.

The classifier adds no port and performs no external act. It consumes the predecessor's already-normalized observations and emits state/effect intent back to that reconciler. Unknown triggers, incomplete evidence, impossible state combinations, or adapter disagreement classify as `malformed-state`: remove future routing, preserve ownership and artifacts, and require human recovery. The table is the exhaustive executable meaning of `prd:feature-implementation-drive/UB-1..UB-11`; adapters may enrich diagnostics but never alter disposition.

## 3. The design

### 3.1 Closed disposition matrix

**Decision:** These rows are the complete v1 failure vocabulary and are evaluated before any ordinary next-effect planning:

| Trigger | Durable disposition | Routing/authority effect | Evidence and user remedy | Traces |
|---|---|---|---|---|
| `invalid-feature` | no aggregate created, or existing unassigned aggregate remains inert | no ownership; no task routing; no execution artifact | failed plan/precheck anchor + exact remediation; rerun kickoff only after merged/frozen contract is observed | UB-1 |
| `ownership-invalid` | `blocked: malformed-state` | remove future routing; honor no owner/epoch from invalid data | raw ref/object id, parse/signature/lineage defect; restore or explicitly reconstruct from verified transition lineage | UB-2 |
| `owner-unavailable` | ownership unchanged; `blocked: stale-owner` diagnostic | remove new routing for the unavailable owner; never route elsewhere | owner id/epoch + last successful touch; same owner resume or explicit handoff/cancel/abandon | UB-3 |
| `handoff-not-quiescent` | old owner remains `active` or `quiescing`; transfer intent blocked | no new old-owner routing while quiescing; no new-owner routing | active attempt/claim/checkpoint reference; finish/checkpoint it, then retry handoff | UB-4 |
| `landing-unverified` | `blocked: landing-verification`; task/dependents incomplete | remove newly derived frontier routing; never call `RecordLanding` | proposal/head/base/landed-ref/ancestry defect; correct or dispose proposal, then re-observe | UB-5 |
| `candidate-conflict` | `blocked: candidate-conflict` until repair item exists, then `repairing` | no candidate ref, green evidence, or final eligibility | `F`, `M`, builder/policy, conflict report, repair task; land reviewed repair then rebuild | UB-6 |
| `verification-unaccepted` | phase remains `verifying`, verdict unaccepted | no final-proposal intent | exact scenario/route/runner/outcome; pass, supply valid manual attestation, or repair failed behavior | UB-7 |
| `candidate-stale` | prior evidence marked stale; phase returns `candidate` | revoke/mark stale final verdict before any final-proposal effect | old/new feature/main heads and evidence id; construct and run full new candidate | UB-8 |
| `final-feedback-head-change` | prior evidence stale; phase returns `candidate` after reviewed repair integration | final proposal cannot become merge-ready | old/new final head, feedback/proposal refs; land feedback repair and run full suite | UB-9 |
| `delivery-unverified` | `blocked: delivery-verification`; owner retained | no delivered transition, ownership release, or cleanup | final proposal, accepted `M/C`, landed `L`, tree/ancestry mismatch; human delivery repair/disposition | UB-10 |
| `cancel-active` | `cancel-pending` until all attempts durably stop, then ownerless `cancelled` | de-route future tasks first; never delete active workspace/branch; increment epoch only after quiescence | active attempt/checkpoint refs; wait/finish/checkpoint, then complete cancellation | UB-11 |

The first matching normalized trigger is terminal for that reconcile pass. Coexisting defects are retained as secondary evidence. Cross-cutting triggers have this closed priority: `ownership-invalid` → `invalid-feature` → `cancel-active` → `handoff-not-quiescent` → `owner-unavailable` → the phase-local trigger. Exactly one phase-local trigger is legal for the aggregate's current phase; observing two, or observing one outside its legal phase, classifies `malformed-state`. Thus adapter order never selects behavior, and every compound observation has one testable result.

`NR-6` is enforced by the same downstream barrier: no task landing effect ever targets main, and no final delivery effect exists until candidate acceptance. A seeded task revision reachable from main before final human merge is a guardrail observation, not a twelfth recoverable trigger: it enters the existing unknown/impossible-state fallback as `malformed-state`, disables new routing, and raises a factory-control escalation; the drive never attempts to rewrite main.

### 3.2 Recovery protocol

**Decision:** A failure clears only by re-observing its positive recovery precondition at the goal object. A human command selects an allowed recovery path but is not itself evidence that recovery completed.

| Failure family | Positive clear condition |
|---|---|
| invalid feature | merged folded PRD + Approved live serving designs + frozen projected plan pass `plangate precheck` |
| ownership invalid/unavailable | valid signed lineage/current epoch restored, or explicit quiesced transition records a new ownerless/owner state |
| handoff/cancel active | every named attempt has terminal/checkpoint evidence and no current task claim remains |
| landing | reviewed proposal's landed result passes base/head/check/ancestry proof and `RecordLanding` is observed |
| candidate conflict | repair task exists, reviewed repair lands, and a different input pair constructs successfully |
| verification | all required routes have accepted evidence on one current candidate |
| stale/final feedback | a new exact-head candidate completes the full required suite |
| delivery | accepted base + candidate-tree + current-main ancestry proof succeeds, or a human explicitly abandons/cancels without claiming delivery |

Recovery itself follows persist-intent-before-effect and fencing from `design:feature-implementation-drive#DINV-1/#DINV-2`. A repeated recovery request with unchanged observations is a recorded no-op. If recovery evidence conflicts across adapters, the disposition stays blocked and names both observations; the classifier never chooses the more permissive one.

### 3.3 Boundaries and non-goals

- **Not solved here** — aggregate/store schemas, effect execution, owner identity, routing projection, candidate construction, scenario routing, or delivery mechanics; the predecessor remains authoritative.
- **Owned by a future design** — new failure classes introduced by future PRD requirements; they extend this closed vocabulary through a successor design rather than an adapter-local case.
- **Implementation must not infer** — that `blocked` means ownerless, that a human retry clears evidence, that timeout grants takeover, that repair authorizes partial e2e, or that cancellation permits deletion.
- **Must remain unchanged** — every predecessor invariant, human merge authority, feature-blind Worker boundary, isolated workspaces, feature-branch integration, candidate identity, and retention ordering.

### 3.4 Invariants

- **DINV-1** — Every UB-1..UB-11 trigger maps to exactly one durable primary disposition before ordinary effect planning; unknown or ambiguous triggers fail to `blocked: malformed-state` with zero new routing.
- **DINV-2** — No disposition clears from elapsed time, signal content, retry, badge, or human assertion alone; its positive goal-object precondition must be re-observed.
- **DINV-3** — Failure handling preserves branches, commits, proposals, task history, aggregate lineage, and evidence unless the predecessor's separately authorized terminal cleanup contract applies.
- **DINV-4** — A blocked ownership, landing, candidate, verification, delivery, handoff, or cancellation state cannot emit an effect downstream of the failed gate.
- **DINV-5** — Adapter-specific diagnostics may enrich evidence but cannot change trigger priority, state, routing effect, recovery paths, or clear conditions.
- **DINV-6** — Before final human merge, no feature task revision is accepted as reachable from main; a detected breach disables routing and escalates without rewriting history.

### 3.5 Observability

- **Events/logs** — `feature_failure_classified` on every primary disposition and `feature_failure_cleared` only on positive re-observation; fields: feature, aggregate version, owner epoch, trigger, disposition, evidence refs, blocked effect class, remedy, and clear evidence.
- **Metrics** — count/time-in-state per disposition; repeat/no-op recovery count; conflicting-observation count; main-before-final breach count; UB fixture pass rate.
- **Alerts** — ownership invalid, overlap risk, main-before-final breach, stale-authorized final action, and wrong delivery are critical operator alerts; other blocked states route action-required notices to the current owner/human surface.
- **Debugging surface** — the feature status diagnostic prints the primary disposition, retained secondary defects, exact evidence, blocked effects, and the positive clear condition.
- **Trace/correlation** — feature id + aggregate version + owner epoch + triggering goal object tie classification to recovery and the resumed lifecycle transition.

### 3.6 Security & privacy

Yes — matches the header. This design controls whether autonomous execution, ownership transfer, final eligibility, delivery, or cleanup proceeds. It introduces no identity, credential, secret, user-data, or egress mechanism; it consumes the predecessor's authenticated observations and signed transitions. Untrusted tracker, proposal, wake, runner, and log text is evidence data only and cannot select a disposition or clear condition. Fail-closed priority prevents a permissive adapter observation from overriding malformed ownership or stale evidence (`#DINV-1`, `#DINV-2`, A-5). Diagnostics redact credentials and preserve only actor/runner identities already authorized for audit.

### 3.7 Testing strategy

The classifier, priority order, transition legality, blocked-effect set, and clear predicates are **in-process** table/property tests. Each UB row has positive, negative, duplicate, and conflicting-observation fixtures. Git ancestry, signed lineage, claims, candidate conflicts, and wrong-tree delivery are **local-substitutable** fixture repositories. Tracker/code-host/e2e adapters pass a shared failure-observation conformance suite; the two six-task dogfoods are **remote-but-owned** demonstrations. Restart injection brackets classification persistence and every recovery effect. The generic executor regression and a main-before-final ancestry fixture pin NR-6.

## 4. Alternatives considered

| Alternative | Why rejected |
|---|---|
| Supersede `feature-implementation-drive` solely to add UB joins | no binding statement changed; wholesale restatement would reapprove a large live design and obscure that the independently revisable missing contract is failure disposition |
| One additive design with only a `Serves` header and no new binding content | would satisfy syntax while failing Serves substance; planners and workers still could not compile the UB rows into tasks and tests |
| Encode failure behavior independently in each adapter | creates multiple predicates for the same safety gate, permits medium drift, and violates the feature-drive core's medium-neutrality |
| Treat all failures as one generic `blocked` state | loses user remedy, positive recovery precondition, routing effect, severity, and testable one-to-one UB coverage |

## 5. Risks

| ID | Risk | Impact | Mitigation | Verification / evidence |
|---|---|---|---|---|
| R1 | this design drifts into a second lifecycle state machine | predecessor and failure contract disagree | classifier emits only predecessor-declared states/effects; conformance checks every row against predecessor invariants | state-vocabulary and invariant cross-check |
| R2 | priority hides a lower-ranked defect permanently | recovery clears primary then unknowingly proceeds into another failure | preserve secondary defects and reclassify from fresh observations before ordinary planning | compound-defect fixture for every priority boundary |
| R3 | a human recovery command is mistaken for proof | stale owner/evidence resumes unsafely | #DINV-2 and explicit positive clear table; command selects path only | assertion-without-goal-proof negative fixtures |
| R4 | adapter disagreement chooses a permissive observation | unauthorized execution or delivery | conflict remains blocked and records both observations | conformance conflict fixtures |
| R5 | future UB rows are added without extending the closed vocabulary | contract gate passes another design but runtime has no disposition | successor-design and plan coverage review; unknown trigger fails malformed | unknown-trigger fixture and contract coverage inspection |

## 6. Acceptance

| ID | Acceptance criterion | Verify | Required evidence | Traces to |
|---|---|---|---|---|
| A-1 | invalid-plan and malformed/conflicting/unverifiable-ownership fixtures create zero executable routing or task attempts and name the exact recovery precondition | Test | UB-1/UB-2 matrix | `prd:feature-implementation-drive/UB-1` · `UB-2` · #DINV-1–#DINV-2 |
| A-2 | owner disappearance and unquiesced handoff preserve the old epoch/owner, route zero foreign/new-owner attempts, and clear only on same-owner resume or verified quiesced transition | Test, race/failure injection | UB-3/UB-4 race report | `prd:feature-implementation-drive/UB-3` · `UB-4` · #DINV-2–#DINV-4 |
| A-3 | wrong-base, stale-head, missing-revision, candidate-conflict, and every unaccepted e2e outcome advance zero dependents/final actions while preserving exact evidence and remedy | Test | UB-5/UB-6/UB-7 fixture report | `prd:feature-implementation-drive/UB-5` · `UB-6` · `UB-7` · #DINV-1–#DINV-5 |
| A-4 | head changes at every candidate/final-action boundary and final-feedback updates invalidate prior evidence before action; only a full accepted new candidate clears | Test, race/failure injection | UB-8/UB-9 race matrix | `prd:feature-implementation-drive/UB-8` · `UB-9` · #DINV-2, #DINV-4 |
| A-5 | wrong-base, wrong-tree, partial, and non-ancestor final-delivery fixtures retain ownership, release/clean nothing, and emit delivery escalation; only full delivery proof clears | Test + security review | UB-10 delivery matrix | `prd:feature-implementation-drive/UB-10` · #DINV-2–#DINV-4 |
| A-6 | cancellation injected around every active-effect boundary de-routes first, preserves all artifacts, mutates no owner epoch until quiescence, and permits zero post-cancel pickup | Test, race/failure injection | UB-11 cancellation matrix | `prd:feature-implementation-drive/UB-11` · #DINV-1–#DINV-4 |
| A-7 | before final human merge, both dogfoods and a seeded breach fixture show zero accepted task revision on main; a detected breach disables routing and escalates without rewriting main | Test + Demo | ancestry audit and breach transcript | `prd:feature-implementation-drive/NR-6` · #DINV-6 |

## 7. Open questions

| ID | Question | Owner | Blocks | Status | Resolution |
|---|---|---|---|---|---|
| — | none — the PRD and predecessor fully decide the v1 failure vocabulary, authority, and recovery gates | — | — | closed | — |

## Principles

- **AP-5 — honored.** Every failure and clear has a typed event, evidence, remedy, age, and end-to-end correlation.
- **AP-6 — honored.** Trigger, disposition, blocked effects, priority, evidence, and clear condition form a closed contractual table.
- **AP-11 — honored.** One deterministic classifier owns failure meaning; adapters only normalize observations.
- **AP-13 — honored.** Recovery requires positive goal-object evidence, never a retry, assertion, status badge, or elapsed time.
- **AP-17 — honored.** Safety-authorizing paths fail closed; duplicate recovery is a no-op and preserves progress.
- **AP-21 — honored.** The merged PRD and predecessor decisions are consumed as judged facts; this design does not re-decide them.
- **AP-22 — honored.** Repeated classification and recovery converge; compound defects reclassify without duplicate external effects.
- **AP-26 / I2 — honored.** Landing and delivery failures are verified at their named goal branches; platform badges cannot clear them.
- **RW-5 — honored.** One independently revisable decision—the closed failure-disposition contract—builds on the lifecycle design without restating or superseding it.
- **Deviation:** none.
