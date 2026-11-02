# Fleet bridge v1 — unattended materialization and the honest stage-8 binding — technical design

| Field | Value |
|---|---|
| **Status** | Draft → Approved *(merge is the freezing event; "superseded" is computed by the index, never written here)* |
| **Owner** | @mdarabi |
| **Approver(s)** | @mdarabi (operator) — drive-behavior + operational-map surface |
| **Date** | 2026-07-13 |
| **Relevant-when** | designing or changing when and how a frozen plan reaches the executing fleet unattended — the plan-freeze trigger, projection re-triggers, the item-verification loop, the drive-to-landing closure discipline; correcting the pipeline map's stage-8 row; triaging a deferred materialization, duplicate items under racing wakes, or a closure claimed off a badge |
| **Scope** | The bridge's three joints: the unattended trigger timing (plan-freeze as an ordinary await), the item-verification loop at the goal object, and the closure discipline over in-flight proposals — plus the pipeline map's stage-8 correction duty. **Not**: the planning stage's own projection contract (consent, idempotency, wiring order — unchanged, plan-new-v1 owns them); the wake transport (wake-channel-v1); lane mechanics (merge-lane-v1); the executing fleet's internals. |
| **Tracking** | — *(the introducing PR)* |
| **Builds-on** | docs/tech-design/plan-new-v1.md (the projection contract this rides: merged-plan-only, convergent per task id, staged modes) · docs/tech-design/argus-agent-v1.md (the drive loop whose materialization confirm-arm this schedules) · docs/tech-design/wake-channel-v1.md (the await/delivery mechanics the plan-freeze trigger uses) · docs/tech-design/merge-lane-v1.md (the routing every resulting proposal lands through) |
| **Supersedes** | — |
| **Serves** | prd:argus-unattended-drive/FR-6 · prd:argus-unattended-drive/FR-10 · prd:argus-unattended-drive/FR-11 · prd:argus-unattended-drive/FR-12 · prd:argus-unattended-drive/UB-6 · prd:argus-unattended-drive/NR-4 · prd:argus-unattended-drive/NFR-4 |
| **Security/privacy impact** | none — already-granted acts on already-bound surfaces (the projection grant exists at the drive's current tier; the tracker convention is the whole fleet interface); no new identity, credential, egress, or data class |
| **Template version** | tech-design-template/v1 |

## 1. Problem

The one live deferral this feature exists to kill is recorded: a drive refused a real projection because the pipeline map's stage-8 row read "not built", and the operator had to direct materialization out-of-band — twice — after which the external fleet promptly claimed and implemented everything (the basis's evidence rows). Two defects compose there: the map states a stale binding, and materialization timing hangs on a human prompt even though the act itself is already contracted and granted (prd:argus FR-9; the drive's current tier carries `trigger-materialization`). The basis adds the closure discipline: a feature closes only when every resulting proposal's content is verified on the mainline — never on a badge (FR-12; incident I2's class).

## 2. Decision

**Decision:** The bridge changes **when**, never **how**: plan-freeze becomes an ordinary await — when the drive stops at a frozen plan's introducing proposal it registers "plan merged at mainline" with the wake channel; the resumed touch verifies the freeze at the goal object, then triggers projection **through the planning stage's own contract** — consent, staged modes, idempotency, and wiring order exactly as that design binds them — with no human prompt; each created item is verified at the goal object (it exists and carries the repo-configured automated-implementation tag) and journaled; re-triggers and racing wakes land the guarantee the projection contract actually gives, split by case — a re-trigger with the items already live records a journaled no-op and creates nothing (UB-6's case), and a true concurrent both-probe-miss race lands the stage's own **detected halt**: the report names both items, routing is withheld for the pair, and the drive surfaces it loudly (journaled, alarmed per the stage's posture) — never silent duplication, never auto-repair, never a bespoke lock around the stage. In-flight proposals are driven to **content-verified landings** through the lane's routing, and the feature closes only when every proposal's content is an ancestor of the mainline. The pipeline map's stage-8 row is corrected to state the binding **as it operationally runs** — the map is the live surface that owns that value, and no drive guidance may direct deferring materialization on the row's account. (design-brainstorm:argus-unattended-drive-tech-design#D-6)

## 3. The design

### 3.1 The trigger: plan-freeze as an await

**Decision:** When the drive's frontier reaches a frozen plan's introducing proposal, the stop registers the await exactly as any other (wake-channel-v1 §3.1): goal object = the plan's introducing proposal landed on the mainline; condition = merged content an ancestor. The resumed touch re-verifies that condition itself (never trusting the wake's reason — the resume protocol's own rule), then triggers projection through the planning stage's contract as its decision principal, journaling the trigger with its effect identities. The projection reads **only the merged plan** (the stage's own invariant — an unmerged plan still refuses, unchanged), so unattended timing adds no new authority: the plan's merge *is* the consent event the stage already binds, and the trigger act is already granted at the drive's current tier. This closes the seam the projection design itself left open and pre-scoped — plan-new-v1 §7 **Q2** (auto-fire on plan-PR merge vs stay operator-invoked; owner: operator; "revisit with the Trigger design; DINV-1 holds either way"): this design is that anticipated revisit, the answer (the drive triggers on the wake-delivered plan-freeze touch) landing under FR-11's operator-approved authority — approved at the basis PRD's freeze and again at this design's introducing PR — with the projection contract's DINV-1 holding exactly as Q2 pre-stated.

### 3.2 The verification loop and the closure discipline

**Decision:** After the trigger, the drive verifies **each created item at the goal object** — it exists on the tracker and carries the repo-configured automated-implementation tag — and journals the confirmation (the basis's EVT-4 row, carried per §3.6's entry-type and gist-grammar home — `trigger: wake` for unattended runs, `prompt` preserved for directed ones); a discrepancy routes back through the planning stage's own contract as a reconciliation, never by the drive editing items. While materialized items are in flight, each resulting proposal lands through the lane's routing (AUTO where floor-clearing and activated; HUMAN otherwise), and the drive accepts each landing only at the goal object — commit an ancestor of the mainline (FR-6). **Closure blocks on the full set**: any stranded proposal — merged badge, absent content — blocks the feature's close and alarms on the delivery ledger. Unclaimed items age under the existing aging/bump arm; nothing new watches them.

### 3.3 The map correction

**Decision:** The pipeline map's stage-8 row and footer state the external-fleet binding as it operationally runs — frozen plans project onto the tracker with the repo-configured automated-implementation tag and an external fleet claims and implements them — and the map remains the live, PR-corrected surface owning that value (its own header already binds it descriptive and corrected-to-match). This design deliberately freezes no copy of the row's wording: the binding duty is FR-10's, the current value is the map's, and when the engine era changes the operational binding, the map moves and FR-10's fit re-pins by amendment (the basis's own hook). The correction lands with this design's implementation; its whole footprint is the map row and footer. What taught the deferral was the stale row itself, never a guidance directive — no guidance surface carries one today (the drive guide already directs the unconditional trigger) — so the durable duty is the standing negative one DINV-5 binds and A-4 inspects: no drive guidance may direct deferring materialization on the row's account, a duty that holds vacuously today.

### 3.4 Boundaries and non-goals

- **Not solved here** — the projection mechanics themselves (consent, staged modes, per-task idempotency keys, dialect rendering — the planning stage's design owns every one, unchanged); wake delivery (the sibling design).
- **Owned by a future design** — the engine-era stage-8 binding (the basis's NG-5 keeps the native engine parked; the map + FR-10's re-pin hook absorb that transition).
- **Implementation must not infer** — that unattended timing relaxes any projection precondition (merged-plan-only holds; a racing wake never yields a silently *routed* second item set — the sequential re-trigger no-ops, and the true both-create window resolves through the stage's detected named-both halt, an operator resolution, never by relaxing a precondition or serializing around the stage); that verifying items authorizes implementing them (the drive never implements fleet-tagged work — the behavior design's own invariant); that the map row's *current wording* is frozen here (the duty is; the value lives on the map).
- **Must remain unchanged** — plan-new-v1's contract in every part (NR-4's fit: the projection path unchanged except trigger timing); the fleet's interface (the tracker convention only); the existing aging/bump arm.

### 3.5 Invariants

- **DINV-1** — Materialization flows only through the planning stage's own contract reading only a merged plan; this design introduces no second projection path and relaxes no precondition of the existing one.
- **DINV-2** — Re-triggers and racing wakes converge to the projection contract's own composed guarantee: with items already live, a trigger records a journaled no-op and creates nothing; a true concurrent both-probe-miss race is the stage's detected halt — both items named, routing withheld for the pair, surfaced loudly through the drive — never silent duplication and never a bridge-side repair, lock, or serialization; duplicate detection and routing-withhold behavior remain the projection contract's own.
- **DINV-3** — Every created item is verified at the goal object — exists + the repo-configured automated-implementation tag — and journaled before the bridge advances; discrepancies route back through the stage, never by item edits.
- **DINV-4** — Closure is mainline-content closure: the feature closes only when every resulting proposal's content is verified an ancestor of the mainline; a stranded proposal blocks closure and alarms.
- **DINV-5** — The pipeline map states the stage-8 binding as it operationally runs; no drive guidance directs deferring materialization on the row's account.

### 3.6 Observability

- **Events/logs** — the journal rows the basis contracts, landing on the delivery ledger's **declared entry types**: the trigger as the act's own `ruling-recorded` entry, each item confirmation as an `item-materialized` entry, discrepancies as `reconciliation` entries routed through the stage — with EVT-4's properties (plan, item ids, tag verified/failed, `trigger: wake|prompt`) riding the entry's **gist body in a typed grammar this design defines**: the house route the behavior design fixed for mid-drive spend and the wake sibling's await grammar reuses — never an invented structured field on the frozen entry classes, first-class fields deferred to the artifacts design's own template-version-bump rule; landings and the closure's verification evidence as the existing entry classes; no new surface.
- **Metrics** — M3 (plan-freeze → verified items live, against the wake bound) is a read over the plan's merge timestamp and the confirmation entry.
- **Alerts** — the stranded-proposal closure block alarms on the delivery ledger (DINV-4); verification discrepancies surface as reconciliations routed through the stage.
- **Debugging surface** — the projection's own report (the stage's artifact) plus the journal's confirmation entries reconstruct any materialization cold (NFR-4).
- **Trace/correlation** — plan id → item ids → proposal ids → mainline commits, end-to-end from repo data.

### 3.7 Security & privacy

none — matches the header row: the bridge exercises an already-granted act through an existing contract on already-bound surfaces; the fleet interface remains the tracker convention; no new identity, credential, egress, or data class appears. The injection posture is inherited: tracker content and PR bodies the drive reads while verifying are data, never instruction (the behavior design's standing rule).

### 3.8 Testing strategy

All seams are existing contracts exercised at new moments, so the strategy is drills over fixtures: the **unattended drill** (a real plan frozen at merge projects with no human prompt; items verified at goal objects; the confirmation journaled — the basis's DOD-5); the **double-trigger drill** (re-trigger with items live → recorded no-op, zero duplicates — DINV-2); the **racing-wake case** (two deliveries converge; the projection's own duplicate posture observed — the no-op, or the detected named-both halt with routing withheld); the **tag-discrepancy fixture** (item live, tag absent/misapplied → the tag arm fails, `tag: failed` journaled, routed back through the stage, no item edit — DINV-3's failure arm, A-5); the **stranded-proposal fixture** (badge-true/mainline-false blocks closure and alarms — DINV-4, shared with the lane's halt fixture class); the **map inspection** (FR-10's fit at this contract's checkpoint, walked as a checklist row).

## 4. Alternatives considered

| Alternative | Why rejected |
|---|---|
| A bespoke plan-freeze observer (the bridge watches merges itself) | duplicates the wake channel's delivery mechanics for one signal class; two observers of one event drift (AP-11); the await shape already fits exactly (design-brainstorm:argus-unattended-drive-tech-design#D-6) |
| A second, bridge-owned projection path ("just create the items directly") | violates the basis's NR-4 and the projection contract's single-writer discipline; re-imports every idempotency and consent question that contract already answers (#D-6) |
| Leave the map row and encode the binding in drive guidance instead | replays the recorded failure — guidance taught deferral precisely because the map was stale; the map is the descriptive surface whose *job* is the current value (#D-6) |
| Prompt-confirmed materialization (keep a human ack, just faster) | the prompt is the toil the basis charges (its J2); the act is already granted and the stage's consent model already binds the merge as the consent event — a second ack adds no authority, only a clock (#D-6) |

## 5. Risks

| ID | Risk | Impact | Mitigation | Verification / evidence |
|---|---|---|---|---|
| R1 | racing wakes both trigger projection | duplicate item sets, or a half-routed pair | DINV-2's split: the sequential re-trigger no-ops inside the projection's per-task-id convergence; the true both-miss window is the stage's detected named-both halt with routing withheld, surfaced loudly through the drive — no serialization added around the stage | the double-trigger + racing-wake drills (A-2) |
| R2 | items created but the tag misapplied — the fleet never claims | silent stall | DINV-3's per-item verification fails the tag arm and routes back through the stage; unclaimed items age under the existing bump arm | the tag-discrepancy fixture (A-5) |
| R3 | closure claimed while a proposal is stranded on a side branch | I2's class — false delivery | DINV-4 blocks closure on ancestry and alarms | the stranded-proposal fixture (A-3) |
| R4 | the map re-stales when the engine era changes the binding | a future deferral class | DINV-5 binds the duty to the live surface, and FR-10's own fit re-pins by amendment at that transition | the FR-10 inspection row at each checkpoint (A-4) |

## 6. Acceptance

| ID | Acceptance criterion | Verify | Required evidence | Traces to |
|---|---|---|---|---|
| A-1 | Live drill: a plan frozen at its merge projects through the stage's own contract with no human prompt; every created item verified at the goal object (exists + the automated-implementation tag) and journaled with `trigger: wake` | Demo | the drill's journal entries + tracker state | prd:argus-unattended-drive/FR-11 · #DINV-1 · #DINV-3 |
| A-2 | Double-trigger and racing-wake drills: the re-trigger with items live records a journaled no-op with zero duplicates; the true both-miss race lands the stage's named-both detected halt with routing withheld, surfaced through the drive — zero silent or routed duplicates either way, and no serialization added around the stage | Test | drill transcripts | prd:argus-unattended-drive/UB-6 · prd:argus-unattended-drive/NR-4 · #DINV-2 |
| A-3 | E2E: closure occurs only after every resulting proposal's content is verified an ancestor of the mainline; a seeded stranded proposal blocks closure and alarms | Demo + Test | the e2e drill + the stranded fixture | prd:argus-unattended-drive/FR-12 · prd:argus-unattended-drive/FR-6 · #DINV-4 |
| A-4 | Inspection at this contract's checkpoint: the map's stage-8 row and footer state the operational external binding, and no drive guidance directs deferring materialization on the row's account | Inspection | the map + guidance read-through | prd:argus-unattended-drive/FR-10 · #DINV-5 |
| A-5 | Seeded tag-discrepancy fixture: an item exists with the automated-implementation tag absent or misapplied — the confirmation's tag arm fails (`tag: failed` journaled), the reconciliation routes back through the planning stage's own contract, and no item is edited by the drive | Test | the fixture transcript + the routed reconciliation entry | prd:argus-unattended-drive/FR-11 · #DINV-3 |

## 7. Open questions

| ID | Question | Owner | Blocks | Status | Resolution |
|---|---|---|---|---|---|
| Q1 | Verification retry shape when the tracker read lags item creation (bounded re-probe vs immediate discrepancy) | implementation | nothing | open | decided at implementation against the tracker adapter's consistency behavior; bounded either way (AP-16) |
| Q2 | The map row's wording at the engine-era transition | the future stage-8 design + the operator | nothing | open | the map moves by PR; FR-10's fit re-pins by amendment (the basis's hook) |

## Principles

- **AP-26 / I2 — honored.** Items, landings, and closure are accepted only at goal objects — tracker records and mainline ancestry — never a platform badge; the stranded case blocks and alarms.
- **AP-22 — honored.** Triggers, re-triggers, and racing wakes converge to the deferred-to contract's own posture: the losing racer records a no-op — or, in the true both-create window, the stage's detected loud halt — never an error, a silent duplicate, or silent divergence (the projection design's own AP-22 wording, carried whole).
- **AP-11 — honored.** One projection path, one duplicate-detection home — the planning stage's; the bridge adds a trigger moment, not a twin.
- **AP-2 — honored.** The trigger arrives as a normalized wake touch; pull (a directed prompt) and push (the wake) produce identical projection behavior, distinguished only in the recorded `trigger` property.
- **AP-19 — honored.** Nothing watches plans or items between touches; verification and bumps ride touchpoints; idle cost is zero.
- **AP-21 — honored.** The plan's merge is the consent judgment, made once at the richest-context site (the plan review); the bridge reads it as data and adds no second ack.
- **AP-18 — honored.** The map states the binding as observed reality runs, corrected to match — never a declared aspiration; behavior keys on the merged plan's existence, not on stage-status declarations.
- **RW-5 — honored.** One decision — the bridge's timing, verification, and map duty — in one document; projection mechanics and wake transport stay with their owning designs per the record's partition (design-brainstorm:argus-unattended-drive-tech-design#D-1).
- **Deviation:** none.
