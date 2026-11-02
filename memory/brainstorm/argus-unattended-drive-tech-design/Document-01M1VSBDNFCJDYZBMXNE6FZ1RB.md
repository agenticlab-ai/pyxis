# Argus unattended drive — design-brainstorm record

| Field | Value |
|---|---|
| **In one line** | Solution-space session for the merged unattended-drive contract: how the AUTO/HUMAN merge lane, the stage-8 fleet bridge, and the wake channel are actually built — the design partition, the lane's execution home and machine identity, the risk-policy/verdict data classes, the wake transport and backstop, the await registration on the delivery ledger, the session-posture seam ruling (additive, no supersession), and the unattended-materialization mechanics. |
| **Relevant-when** | designing or changing the merge lane (classifier, policy, machine identity, activation), the wake channel (awaits, backstop, resume delivery), or the fleet bridge (unattended materialization, the stage-8 map binding); checking prior design rulings or refused shapes about unattended-drive machinery — the rejected lane-service/daemon/drive-embedded shapes, the no-supersession posture ruling |
| **Basis** | prd:argus-unattended-drive (merged ledger — effective as of 000, fingerprint `sha256:56aaae62…`, frozen at mainline) |
| **Authority** | Argus (decision principal, tier-2) — rulings are legality-tested assumptions journaled per the drive guide (delivery:argus-unattended-drive); the operator overturns through the normal channels |
| **Date(s)** | 2026-07-13 |
| **Status** | gated: ready-for-design |
| **Tracking** | — |
| **Feeds** | docs/tech-design/merge-lane-v1.md · docs/tech-design/fleet-bridge-v1.md · docs/tech-design/wake-channel-v1.md |
| **Prior records** | brainstorm:argus-unattended-drive (the stage-1 basis record — its P-1..P-4 are this session's question set) · design-brainstorm:argus-tech-design (the argus-pair stage-3 precedent — partition ruling style, the session-posture rulings D-6/D-7 this session must not contradict) · design-brainstorm:argus-boundary-enforcement-tech-design (targeted: its F-8 content-purity constraint and the shipped enforcement-arm inventory) |
| **Template version** | design-brainstorm-record/v1 |

## Orientation

| Source | Why loaded |
|---|---|
| docs/prds/argus-unattended-drive/effective.md | the basis: FR-1..17, NR-1..5, UB-1..9, ABUSE-1..4, NFR-1..5, EVT-1..4, Q3..Q6 (the design-owned handoffs), §3.3 ship rule, §6.1 phasing |
| docs/tech-design/index.md | the live decision graph — which designs' Serves/scope this basis touches (argus pair, plan-new, repo-layout, docgate, solution-shape) |
| docs/tech-design/tech-design-conventions.md | supersession semantics (whole-document; the contradiction boundary), header schema, binding elements, the generated index duty |
| docs/tech-design/argus-agent-v1.md | the frozen behavior design the wake touches: §3.2 session posture, §3.3 touchpoints/parks, §3.4 front door, DINV-4/DINV-5 |
| docs/tech-design/argus-artifacts.md | the frozen data design: §3.2 derived-state block content, §3.8 template-evolution rule, DINV-6 (journal wins) |
| docs/tech-design/plan-new-v1.md | the materialization contract the bridge rides: DINV-1/3/5, staged modes, the concurrent-projection race posture |
| docs/tech-design/repo-layout-and-system-footprint.md | the config surface (closed-parse config, `paths` keys), bucket taxonomy, content-purity rule A-2 |
| docs/tech-design/solution-shape-v1.md | designs carry no concrete shape in-body; the paired shape class, witness/trace checks — the lane's schema lands as a shape |
| docs/architecture-principles.md | AP-1/2/6/8/10/11/15/17/18/19/21/22/24/26, RW-5 — the scoring vocabulary |
| docs/observability/incidents.md | I1 (Serves cites committed FRs that predate the design), I2 (badge vs mainline — graduated AP-26), I3 (fail-closed on CLI misparse) |
| docs/wiki/glossary.md | the ruled terms this session must use: AUTO/HUMAN lane, floor-clearing, wake signal, activation; the Argus term set |
| docs/brainstorms/argus-unattended-drive.md · argus-tech-design.md | the stage-1 record (P-1..P-4, F-1..F-12, O-set); the stage-3 precedent (partition-question panel call, D-6/D-7 posture rulings) |

**Overlaps surfaced:** no live design serves prd:argus-unattended-drive (fresh basis). Adjacent deciders honored, not re-litigated: plan-new-v1 owns projection consent/idempotency/wiring (the bridge changes only the trigger timing — its NR-4); argus-artifacts owns the ledger classes (the await registration lands additively under its own evolution rule); argus-agent-v1 owns the drive protocol (this session rules whether its posture prose must move — DQ-5); solution-shape-v1 owns where concrete schema lives; the shipped enforcement arms (write hook, attribution gate, structure arm, ledger arms) sit beneath the lane unchanged. None covers the lane, the wake channel, or the unattended-materialization timing — that ground is open.

## Design questions

- **DQ-1**: How many design documents carry the three-cluster contract (the lane FR-1..9 · the bridge FR-10..12 · the wake FR-13..17), and what partitions them?
- **DQ-2**: Where does the lane execute — the classification and the AUTO acts: what computes verdicts, what holds and exercises the machine identity, and how does it interplay with branch protection and required contexts (basis Q4)?
- **DQ-3**: What data shape and home do the risk policy, the activation state, and the lane-verdict records take — schema, ratification enforcement, kill switch, shadow-evidence home (basis Q5)?
- **DQ-4**: What transports the wake — how are awaited completions observed, normalized, and delivered to the front door per CLI/host, and what guarantees the latency bound (basis Q3, mechanism half)?
- **DQ-5**: Does the wake require moving frozen argus-agent-v1 session-posture prose (supersession through channels), or is it additive — and through which channel does the await registration reach the delivery-ledger class (basis Q3 seam half; FR-13; NG-6)?
- **DQ-6**: What are the unattended-materialization mechanics — the plan-freeze observation point, projection idempotency under racing wakes, and the item-verification loop at the goal object (basis Q6)?

## Panel log

- **DQ-1**: extends (the design-corpus partitioning convention — RW-5's independently-revisable-question test applied to a ruled cluster set) — skipped — a decomposition ruling with inline options, no structural shape to diverge on; the stage-3 precedent's exact call (design-brainstorm:argus-tech-design DQ-1), audited fresh here: the three candidate partitions differ in document count, not in mechanism shape
- **DQ-2**: opens (the merge-lane execution seam — new machinery computing lane verdicts and exercising a machine identity against the code host) — fired — three orthogonal shapes: repo-native verb + platform workflow, external lane service, drive-embedded lane
- **DQ-3**: opens (two new data classes — the risk policy/activation state, and the lane-verdict record) — fired — three orthogonal homes: dedicated policy file at the config surface, a block inside the system config, platform-native protection rules as the policy
- **DQ-4**: opens (the wake-channel transport seam — how completions become front-door touches) — fired — three orthogonal shapes: ledger-registry + bounded backstop with push-preferred delivery, push-only hosted sessions, resident host listener
- **DQ-5**: potentially replaces the session-posture seam (the NG-6 crux — whether frozen behavior-design prose must move) — fired — two structurally distinct evolution shapes: additive `Builds-on` design vs whole-document supersession (an agent v2); the conventions' contradiction test is the decider, walked statement-by-statement in the scoring
- **DQ-6**: extends (plan-new-v1's own projection contract + the wake's signal class — the deciders already exist) — skipped — the deltas are parameters on ruled ground: trigger timing (a wake-delivered touch), race behavior (plan-new's convergent projection absorbs racing wakes — F-4), and the verification loop (prd:argus FR-9's confirm arm, already granted at tier-2); no structurally distinct alternative to riding the stage's own contract exists without violating basis NR-4

## Decisions

### D-1 — three designs: the lane, the bridge, the wake

- **Question**: DQ-1 — how many design documents carry the three-cluster contract, and what partitions them?
- **Answer**: **Three** — `merge-lane-v1` (the AUTO/HUMAN lane: classifier, risk-policy/verdict data, machine identity, shadow, activation gate, kill switch — FR-1..9 with their UB/ABUSE/NR/NFR rows), `fleet-bridge-v1` (the stage-8 binding correction + unattended materialization + the drive-to-landing closure loop — FR-10..12), `wake-channel-v1` (await registration, the wake transport + backstop, park/staleness semantics, the drive's additive duties — FR-13..17). Options weighed inline: one combined doc; two (lane + a merged bridge-and-wake); three (chosen).
- **Reasoning**: The partition line is the conventions' own test — one independently-revisable question per document: lane mechanics revise without touching wake transport and vice versa; the bridge's timing/verification loop revises without either (it *uses* both — `Builds-on`, not co-location). One doc bundles three revisable questions (the conventions' R5 churn: superseding lane mechanics would force re-approving wake+bridge); two couples the projection-verification loop to transport revisions. Three also matches the basis's phasing (A: bridge+wake · B: lane) and its "up to three designs" appetite.
- **Anchor**: 2026-07-13 · Argus assumption (delivery:argus-unattended-drive#E-24)
- **Status**: live

### D-2 — the lane executes repo-natively: one deterministic classifier verb, platform-workflow acts, secret-held machine identity

- **Question**: DQ-2 — where does the lane execute, what holds the machine identity, and how does it interplay with protection and required contexts?
- **Answer**: **ALT-1** — the classifier as one hermetic in-repo verb (the single lane predicate: diff + policy + medium-read check states → typed verdict, fail-closed per UB-1); a thin platform-workflow adapter invokes it per proposal event, records the verdict, and — enforce mode only, activation present (FR-9) — approves + merges as the dedicated machine identity at the evaluated head revision, credential in the platform secret store, never reachable from any drive session (NFR-3). Protection interplay: the lane never bypasses a required context — it acts only when the medium reports the full required set green (read from the medium, never a private copy — AP-11); the org backstop making a machine approval sufficient is the operator's activation act, outside this design. Idempotent/raceable: re-runs converge; a post-evaluation push voids the verdict (re-evaluate before any act); an already-merged proposal is a recorded no-op (AP-22).
- **Reasoning**: the only shape satisfying NFR-3 + NR-1 + the appetite's no-new-egress line simultaneously; the one-predicate rule kills the twin-predicate drift class; ALT-2/ALT-3 rejected on the scores above.
- **Anchor**: 2026-07-13 · Argus assumption (delivery:argus-unattended-drive#E-25)
- **Status**: live

### D-3 — policy and verdicts as data: a dedicated ratified policy file; verdicts ride the proposal; acts self-describe; the agreement report is committed

- **Question**: DQ-3 — the risk policy's schema/home, ratification and kill mechanics, and the verdict-record/shadow-evidence home.
- **Answer**: **ALT-4 (ALT-5's path-key piece absorbed)** — one versioned closed-schema policy file at the repo's config surface (floor classes · closed AUTO-class set · thresholds · activation block · kill switch), own `schema_version`, shared validator verb, unknown keys are defects (UB-2), every change requiring the operator trailer (the existing gate-policy ratification mechanism, widened); the system config carries only the path key. Verdict records ride the proposal as structured versioned blocks (append-only per evaluation); AUTO acts self-describe in the approval/merge history (identity · head revision · policy version — NFR-4's cold-read); the shadow agreement report is a committed artifact computed from records alone (FR-8). Concrete schema lands in merge-lane-v1's paired solution-shape, never the design body (F-7).
- **Reasoning**: ratification as a visible one-file diff with an enforced human trailer is AP-24 made mechanical; the kill switch is one field (NFR-5); ALT-6's platform-native shape cannot express whole-diff classes or shadow mode.
- **Anchor**: 2026-07-13 · Argus assumption (delivery:argus-unattended-drive#E-25)
- **Status**: live

### D-4 — the wake channel: ledger-registered awaits, push-preferred delivery, the bounded backstop as the guarantee

- **Question**: DQ-4 — how completions are observed, normalized, and delivered within the latency bound, per CLI/host.
- **Answer**: **ALT-7** — awaits are registered in the delivery ledger at every stop (FR-13: goal object + completion condition; the journal's stop entry is authoritative, the derived block surfaces it); delivery is push-preferred (adapter events normalize to slug + reason and re-enter the front door — AP-2) with the **bounded backstop tick on the drive host as the always-on guarantee**: read the awaits, probe each goal object exactly, invoke the CLI once per due touch, exit — interval = the configured bound, idle cost ~0 and stated (NFR-2/UB-7), parks excluded by construction (FR-16/NR-5), staleness readable from the same registry (UB-9). Sequential fallback: manual invocation (DINV-5; EVT-3 `delivered-by: manual`). Hosted push-only and resident-listener shapes rejected on scored grounds.
- **Reasoning**: FR-17's no-lost-completion guarantee must hold *without* push (signals are lossy); only the registry+tick shape guarantees the bound at ~0 idle with no new credential home; push stays a latency optimization per adapter, exactly AP-2's posture.
- **Anchor**: 2026-07-13 · Argus assumption (delivery:argus-unattended-drive#E-26)
- **Status**: live

### D-5 — the posture seam: additive, no supersession — and the await registration lands additively under the artifacts design's own evolution rule

- **Question**: DQ-5 — must frozen argus-agent-v1 session-posture prose move (supersession through channels), and through which channel does the await registration reach the delivery-ledger class?
- **Answer**: **ALT-10** — no supersession: wake-channel-v1 `Builds-on` argus-agent-v1 + argus-artifacts, purely additive. The contradiction test (F-1) was walked against every binding statement (F-2): none goes false — an await-stop completes a touch, it never protocol-ends a session (close and park remain the only protocol ends); the wake is a new caller of the existing front door; DINV-4/DINV-5 preserved verbatim. Basis Q3's "if the prose must move" resolves **negative**, with ALT-11 (an agent v2) recorded as the sanctioned channel should a future evolution falsify a binding statement. The await registration reaches the ledger **without a template bump**: the journal stop entry names the await (the ledger *is* the registration — FR-13's re-derivable fit), and the derived block surfaces it inside its existing Open-items element under a typed grammar the wake design defines — argus-artifacts' §3.2 content list stays true, DINV-6 (journal wins) untouched; a first-class Awaits row is deferred to a future template version bump under §3.8's successor-design rule (recorded as fog → the design's Q-n).
- **Reasoning**: the conventions reserve supersession for contradiction; re-issuing ~20 anchors for additive duties is churn (ALT-11's score); the grammar-in-open-items route satisfies FR-13/UB-9 for *live* v1 ledgers immediately — no migration, no re-pinning, journal-authoritative.
- **Anchor**: 2026-07-13 · Argus assumption (delivery:argus-unattended-drive#E-26)
- **Status**: live

### D-6 — the bridge: plan-freeze as an await, projection through the stage's own contract, verification at goal objects, the map row corrected

- **Question**: DQ-6 — the unattended-materialization mechanics: observation point, racing-wake behavior, the verification loop, and FR-10's map correction.
- **Answer**: (1) **Observation point**: when the drive stops at a plan's introducing proposal, it registers the await "plan PR merged at mainline" — the plan-freeze completion is an ordinary wake-signal class, no bespoke observer (basis Q6's first arm). (2) **Trigger**: the resumed touch verifies the freeze at the goal object (ancestry), then triggers projection **through the planning stage's own contract** — consent, staged modes, wiring order untouched (NR-4; the act is granted at tier-2). (3) **Races**: concurrent or duplicate wakes converge inside plan-new's own idempotency (probe-first, per-task-id at-most-once, both-miss detection with routing withheld — F-4); the drive's re-trigger is a recorded no-op (UB-6). (4) **Verification loop**: per created item, confirm at the goal object — exists + carries the repo-configured automated-implementation tag — journal the confirmation (EVT-4 with `trigger: wake`); discrepancies route back through the stage, never by editing items (prd:argus FR-9's confirm arm). (5) **FR-10**: fleet-bridge-v1 binds the map's stage-8 row to the operational external-fleet binding with the honest footer; the drive-guidance residue ("defer because not built") dies with it.
- **Reasoning**: every mechanism already has a ruled owner — the bridge design adds timing, the verification loop's shape, and the map correction; inventing a second projection path would violate NR-4 and AP-11.
- **Anchor**: 2026-07-13 · Argus assumption (delivery:argus-unattended-drive#E-27)
- **Status**: live

### D-7 — the design-set perimeter: operator acts stay outside every design; enforcement composes, never replaces

- **Question**: What may none of the three designs do — the inherited refusals, confirmed at design grain?
- **Answer**: No design performs, automates, or schedules the operator's activation acts (the org backstop change, the class-set ratification, any tier grant) — they appear only as FR-9's fail-closed precondition reads and the activation brief's content (basis NG-1/NG-2). No design weakens, replaces, or re-implements an existing mechanical check (NG-3; NR-1's byte-identical bar); the lane composes *over* the check surface. No design touches fleet internals (NG-4), builds engine-era machinery (NG-5), or edits frozen content in place (NG-6 — held by construction after D-5). The authority contract, ladder, and never-list are byte-untouched (NR-3).
- **Reasoning**: the perimeter is inherited from the frozen basis and A-1, not invented; confirming it at design grain gives each design's Boundaries section its seed (the O-set below).
- **Anchor**: 2026-07-13 · Argus assumption (delivery:argus-unattended-drive#E-27)
- **Status**: live

## Alternatives

### ALT-1 — the repo-native lane: deterministic verb + platform workflow + secret-held machine identity

- **Shape**: the lane classifier is an in-repo, hermetic Go verb (diff + policy + check states → one typed verdict; the docgate check/defect pattern), invoked by a code-host workflow on proposal events; in enforce mode the workflow — holding the dedicated machine identity's credential in the platform's secret store — submits the approval and the merge bound to the evaluated head revision; shadow mode records verdicts and acts on nothing.
- **Trade-offs**: wins — no new infrastructure or egress (rides the CI surface the repo already binds); the classifier is the *one* lane predicate, runnable identically in CI, at desks, and on fixtures; credential structurally out of the drive's reach (the secret store's boundary). Costs — workflow latency on busy queues; the medium binding lives in workflow glue (kept thin, adapter-shaped).
- **Principles**: AP-10 honored (deterministic code, no prompt); AP-11 honored (one predicate, one implementation everywhere); AP-1 honored (classifier medium-neutral; the workflow is the adapter); AP-19 honored (fires per event; idle = zero); AP-17/I3 honored (misparse/ambiguity → HUMAN, fail closed); AP-26 honored (acts verified at the goal object after).
- **Incidents**: I2 matched — post-merge ancestry verification is the act's acceptance, never the platform badge; I3 matched — the verb refuses flag-like positionals and malformed policy before any action.
- **Verdict**: chosen — the only shape that satisfies NFR-3 (credential unreachable by the drive), NR-1 (checks untouched), and the appetite's no-new-egress line at once (D-2)
- **From**: DQ-2 · panel

### ALT-2 — the external lane service

- **Shape**: a resident service (platform App with a webhook endpoint) receives proposal events, classifies, and acts — the gen-1 reviewer-bot topology.
- **Trade-offs**: wins — lowest act latency; central multi-repo posture for the engine era. Costs — always-on hosting + a new egress/webhook surface (the appetite excludes new egress); an idle service spends while nothing happens; a second home for policy evaluation invites predicate drift against any desk-side read.
- **Principles**: AP-19 strained (resident idle cost); AP-11 strained (service twin of the desk predicate); AP-25/portability strained (a repo-installable factory should not require hosted infrastructure); AP-1 nominally honored.
- **Incidents**: none checked beyond I2's generic act-verification (registry as of 2026-07-13).
- **Verdict**: rejected — new egress/infrastructure against the appetite, idle cost against AP-19; re-openable at the engine era's multi-repo surface (basis WR-2) (D-2)
- **From**: DQ-2 · panel

### ALT-3 — the drive-embedded lane

- **Shape**: Argus computes lane verdicts at drive touchpoints and performs (or requests) the acts itself — the lane as drive behavior, no separate machinery.
- **Trade-offs**: wins — zero new components. Costs — enforce mode puts an approving/merging credential in the drive's hands (NFR-3 and the never-list's outward-surface boundary both violated); proposals arriving while no session lives (fleet PRs) go unclassified — shadow evidence gets holes; conflates the installation's do-dial with the delegate's decide-dial (the basis D-3 boundary).
- **Principles**: NFR-3/NR-3 violated (credential + ladder confusion); AP-19 strained (classification waits for touches); AP-21 strained (verdicts re-derived per touch instead of judged once at the event).
- **Incidents**: none checked beyond the basis's ABUSE rows (registry as of 2026-07-13).
- **Verdict**: rejected — structurally illegal against NFR-3 and the ratified decide/do split (D-2)
- **From**: DQ-2 · panel

### ALT-4 — the dedicated risk-policy file at the config surface + on-proposal verdict records

- **Shape**: one versioned, closed-schema policy file at the repo's config surface (its own `schema_version`; floor classes as path/class globs, the closed AUTO-class set, thresholds, activation block, kill switch field) with a shared validator verb; changes require the operator trailer (the existing gate-policy ratification mechanism widened to the policy home). Verdict records: a structured, versioned record per evaluation riding the proposal (append-only), AUTO acts additionally self-describing (identity, head revision, policy version) in the approval/merge history; the shadow agreement report is a committed artifact computed from the records.
- **Trade-offs**: wins — ratification is a visible diff on one file with an enforced human trailer (AP-24); the kill switch is one data change (NFR-5); the policy's own diff is floor-classed (UB-3's self-unlock guard has a crisp target). Costs — one more governed file; verdict-on-proposal is medium-held data (durable with the proposal, read via the adapter — mitigated by acts self-describing in git history and the committed shadow report).
- **Principles**: AP-6 honored (closed schema, versioned); AP-8/AP-24 honored (sanctioned, human-ratified extension surface); AP-21 honored (classes judged once by the operator, read verbatim); AP-10 honored (validator is code); AP-15 honored (enforce reads are exact at act time).
- **Incidents**: I3 matched — unknown keys/malformed policy refuse to load and the lane goes all-HUMAN (UB-2); I1 none-applicable (data class, not a design).
- **Verdict**: chosen — with ALT-5's one good piece absorbed: the config file carries only the *path key* to the policy home, never policy content (D-3)
- **From**: DQ-3 · panel

### ALT-5 — a policy block inside the system config file

- **Shape**: the risk policy as a `lane:` block inside the existing closed-parse system config, like plan-new's `plan:` block.
- **Trade-offs**: wins — zero new files; the config's closed parse is already enforced. Costs — the system config is install-managed (manifest-matched, upgraded by the installer) while the policy is operator-ratified governance data on its own cadence: one file, two owners; ratification diffs entangle with install churn; the floor's "the risk policy itself is hard-gated" rule would hard-gate every unrelated config edit.
- **Principles**: AP-6 honored; AP-24 strained (governance data inside an install-managed artifact); AP-20 strained (blast radius of config edits widens to the merge gate).
- **Incidents**: none checked beyond I3's closed-parse posture (registry as of 2026-07-13).
- **Verdict**: hybridized into ALT-4 — the config surface contributes the *path key* only; policy content gets its own file and cadence (D-3)
- **From**: DQ-3 · panel

### ALT-6 — platform-native policy (branch protection as the floor)

- **Shape**: encode the floor and classes in the code host's own protection rules (protected paths, required reviews, rulesets); no in-repo policy artifact.
- **Trade-offs**: wins — enforcement by the medium itself, no code. Costs — the policy stops being versioned repo data (AP-24's home lost; audit years-later weakens against NFR-4); whole-diff class logic (FR-3) exceeds path-rule expressiveness; no shadow mode exists (FR-8 unimplementable); the shape is medium-locked (AP-1).
- **Principles**: AP-24 violated (governance data outside the repo's human-authored surface); AP-1 violated (no adapter boundary); FR-8's shadow requirement unimplementable.
- **Incidents**: none checked (registry as of 2026-07-13).
- **Verdict**: rejected — though the *org backstop itself* stays exactly this class of platform act, owned by the operator at activation (complementary boundary, not the policy home) (D-3)
- **From**: DQ-3 · panel

### ALT-7 — the awaits registry + bounded backstop, push-preferred delivery

- **Shape**: awaits live in the delivery ledgers (each stop registers goal object + completion condition — FR-13, the registry); delivery is two-layered: **push where the adapter provides it** (a completion event normalizes to `(slug, reason)` and re-enters the front door) and **always the bounded backstop** — a scheduled tick on the drive host that reads the registered awaits, probes each awaited goal object with exact reads, invokes the agentic CLI once per due touch, and exits. Interval = the configured latency bound; parked problems are not awaits (excluded by construction); an answer append is a repo event ridable by the push path, else the human's own touch (today's path). Manual `/argus <slug>` is the stated sequential fallback (EVT-3 `delivered-by: manual`).
- **Trade-offs**: wins — the latency bound is *guaranteed* by the backstop and merely *improved* by push (FR-17 satisfied by construction; a lost signal costs one interval, never a drive); idle cost is one cheap registry read (~0, statable — NFR-2); no new credential home (the CLI runs where drives already run). Costs — worst-case latency is the tick interval when push is unwired; the host must be on (stated honestly: an offline host defers to its next tick — the same truth manual resumption had).
- **Principles**: AP-2 honored (normalized signal, no carried logic); AP-19 honored (stated ~0 idle; zero for parks); AP-15/AP-18 honored (exact goal-object probes, observed not declared); AP-26 honored (the probe reads the goal object, never a status feed); DINV-4/DINV-5 of the agent design preserved (no recurring process *for a park*; sequential fallback stated).
- **Incidents**: I2 matched — completion is detected by goal-object observation (ancestry, record existence), never a badge.
- **Verdict**: chosen — the only shape meeting FR-17's guarantee, AP-19's cost line, and the appetite's no-new-credential line at once (D-4)
- **From**: DQ-4 · panel

### ALT-8 — push-only: hosted wake sessions on the platform

- **Shape**: completion events trigger platform-hosted runner sessions of the agentic CLI (model credential in platform secrets); no host backstop.
- **Trade-offs**: wins — lowest latency, no dependence on a drive host. Costs — a model credential and agent egress move into CI (a new egress + credential home — the appetite excludes both; UB-5-of-basis escalation territory); a lost/undelivered event is a lost drive (FR-17 unmet without adding the backstop anyway); runner cost per touch.
- **Principles**: AP-2 honored; the appetite boundary violated (new egress/credential); AP-19 nominally honored but per-touch runner spend is a new recurring cost class.
- **Incidents**: none checked beyond the injection class already contracted (EVAL-2) (registry as of 2026-07-13).
- **Verdict**: rejected — breaches the ratified no-new-egress/credential boundary and still needs ALT-7's backstop for FR-17; re-openable at the engine era (D-4)
- **From**: DQ-4 · panel

### ALT-9 — the resident host listener

- **Shape**: a long-lived daemon on the drive host holds subscriptions (webhook forward or long-poll) and invokes the CLI on events.
- **Trade-offs**: wins — push latency without platform credentials. Costs — a resident process spending while nothing is awaited (the exact daemon shape argus-agent-v1's alternatives table already rejected for parks/fleet-watching); fragile across laptop sleep/restarts; a second thing to install, monitor, and kill.
- **Principles**: AP-19 strained (recurring resident cost; the tick delivers the same bound at ~0); DINV-4's spirit strained (a standing watcher); AP-22 neutral.
- **Incidents**: none checked (registry as of 2026-07-13); the agent design's own daemon rejection is the in-house precedent.
- **Verdict**: rejected — the bounded tick meets the same bound with zero resident cost; push remains an optional adapter layer, not a resident one (D-4)
- **From**: DQ-4 · panel

### ALT-10 — the wake as an additive design: `Builds-on` the argus pair, no supersession

- **Shape**: wake-channel-v1 `Builds-on` argus-agent-v1 + argus-artifacts, adding: the await-registration duty at stop points (journal stop entry names the await; the derived block surfaces it under Open items in a typed grammar), the wake channel (ALT-7), the touchpoint staleness duty (UB-9), and the wake-surface injection eval (EVAL-2). The session posture is untouched: the drive still never protocol-ends a session except at close or park — an await-stop *completes the touch* and the runtime ends the session; a wake starts the next touch through the existing front door ("an input resolving to an existing problem directory resumes it").
- **Trade-offs**: wins — zero re-approval of the frozen protocol's ~20 served anchors; the touch vocabulary is already the system's own (touchpoints, front-door touches, resume touches); NG-6 held trivially. Costs — the continuous-session *preference* now reads against a wake-architected reality (mitigated: the preference was always within-touch — the design says so explicitly).
- **Principles**: the conventions' contradiction test walked against every binding statement of F-2: §3.2's ended-only-by enumeration stays true (close/park are the only protocol-decided ends; an await-stop ends no session — the runtime does); §3.3 park semantics untouched; §3.4's door semantics untouched (the wake is a new *caller*); DINV-4 preserved verbatim (FR-16/NR-5); DINV-5 preserved (manual fallback). No binding statement goes false → `Builds-on` narrowing is legal and is not even narrowing — it is extension.
- **Incidents**: none checked beyond the posture rulings' own lineage (registry as of 2026-07-13).
- **Verdict**: chosen — no contradiction exists once the touch model is stated; supersession stays the named escape hatch the day a wake evolution *does* falsify the posture (D-5)
- **From**: DQ-5 · panel

### ALT-11 — argus-agent-v2: wholesale supersession carrying the protocol forward

- **Shape**: a successor behavior design (`Supersedes: argus-agent-v1`) restating the whole drive protocol around the touch model — awaits, wake-started sessions, registration and staleness duties woven in; everything else carried forward.
- **Trade-offs**: wins — one document owns the posture story end-to-end; no two-document read for the drive's session semantics. Costs — re-issues ~140 lines and ~20 served anchors for a delta that is purely additive duties; the review burden triples against the basis's three-review appetite (v2 + lane + bridge would consume it with the wake channel still needing a home); the whole-document supersession rule makes every carried-forward clause re-approvable surface.
- **Principles**: AP-7 honored (supersession is the sanctioned channel); RW-5 strained (the "one decision" being revised is not actually revised — no binding statement of v1 is false); the conventions themselves reserve supersession for contradiction, which the walk in ALT-10 does not find.
- **Incidents**: none checked (registry as of 2026-07-13).
- **Verdict**: rejected — supersession without contradiction is churn; explicitly recorded as the sanctioned channel if a future wake evolution falsifies a binding posture statement (NG-6's standing answer)
- **From**: DQ-5 · panel

## Seam map

- **S-1**: the merge-lane execution seam (classifier verb · workflow adapter · machine identity) — opens — behind it: FR-1..FR-9 routing and acts, UB-1..UB-4/UB-8, ABUSE-1/2/4, NR-1..NR-3, NFR-1/NFR-3/NFR-5 — feeds: merge-lane-v1 Scope + Serves
- **S-2**: the risk-policy + lane-verdict data classes (ratified policy file · on-proposal verdict records · committed agreement report) — opens — behind it: FR-2/FR-3/FR-7/FR-8, the kill switch, shadow evidence — feeds: merge-lane-v1 Scope (+ its paired solution-shape)
- **S-3**: the wake channel (awaits registry read · push-preferred delivery · bounded backstop · CLI invocation) — opens — behind it: FR-14/FR-15/FR-17, UB-5/UB-7, M4's bound — feeds: wake-channel-v1 Scope + Serves
- **S-4**: the await registration on the delivery ledger (journal stop entries + the typed Open-items grammar) — extends (delivery-ledger/v1's derived block, additively, under argus-artifacts §3.8's evolution rule) — behind it: FR-13, UB-9's readable ages — feeds: wake-channel-v1 Scope
- **S-5**: the drive's touchpoint duty set (await registration at stops · staleness surfacing · wake-content-as-data) — extends (argus-agent-v1's touchpoint duties, additively — D-5's no-contradiction walk) — behind it: FR-13/UB-9, EVAL-2 — feeds: wake-channel-v1 Scope + `Builds-on`
- **S-6**: the materialization timing + verification loop (plan-freeze await → stage-contract trigger → goal-object confirmation) — extends (plan-new-v1's projection contract; prd:argus FR-9's confirm arm) — behind it: FR-11/FR-12, UB-6, NR-4 — feeds: fleet-bridge-v1 Scope + Serves
- **S-7**: the pipeline map's stage-8 row (living descriptive surface, PR-corrected) — extends — behind it: FR-10's honest binding — feeds: fleet-bridge-v1 Scope

## Facts

- **F-1**: Supersession is whole-document in v1; `Builds-on` narrowing is legal only while both documents can be simultaneously true — "if the new decision makes any binding statement of a live design false, it must supersede it; two live designs never disagree". — source: tech-design-conventions §2
- **F-2**: argus-agent-v1's binding session-posture statements: §3.2 "one problem, one session, held as long as the runtime allows — ended only by close, park, or the runtime itself"; §3.3 a brief "parks the problem fail-closed and ends the session" and "There is no daemon; a parked problem costs zero" (scoped to the drive's park/touchpoint model); §3.4 the front door — "an input resolving to an existing problem directory resumes it"; DINV-4 (parked = no live session, no recurring process; bumps only at touchpoints); DINV-5 (sequential+files+shell only; harness primitives optional with stated fallback). — source: docs/tech-design/argus-agent-v1.md
- **F-3**: argus-artifacts binds the derived-state block's content (status enum · stage position · open items · park state with ages; regenerated on append, never authoritative — DINV-6 journal wins) and pre-authorizes evolution: "Template versions are pinned at instantiation; evolution is a template version bump decided by a successor design, never an in-place redefinition" (§3.8). — source: docs/tech-design/argus-artifacts.md
- **F-4**: plan-new-v1's projection contract: `project` reads only a merged plan (DINV-1); projection is stateless-convergent per plan-task-id — probe exact reality first, at most one task ever created, re-runs no-op (DINV-3); routing signals apply only after hierarchy + edges (DINV-5); two concurrent projections that both probe-miss are detected, the report names both and routing is withheld for the pair; staged modes exist (dry-run · no-routing · full). — source: docs/tech-design/plan-new-v1.md §3.7, §3.5
- **F-5**: A ratification-enforcement mechanism already exists in the gate suite: gate-policy config keys require the operator trailer on the changing commit (`check-config-policy`). — source: the docgate registered-check set
- **F-6**: The content-purity gate (`check-content-paths`, repo-layout A-2) checks wiki + the flat corpus always, and ledger entries + designs when newly added since the merge base: content cites templates by version pin and skills/guides by role, never a system-footprint path. The solution-shape home is outside the purity set — concrete paths belong in shapes, not design bodies. — source: the docgate registered-check set; repo-layout §3.1; design-brainstorm:argus-boundary-enforcement-tech-design#F-8
- **F-7**: A design body may carry no concrete solution-shape (`check-design-shape`); shapes live at the config-keyed shape home, at most one per design, named by the design's slug, with witness/trace consistency checks; a shape is `authoritative` until its code lands. — source: docs/tech-design/solution-shape-v1.md §3.1–§3.8; the docgate registered-check set
- **F-8**: The proven gen-1 lane shape (parity source, not mechanism): deterministic floor with hard-gate path globs + self-unlock guard, narrow AUTO classes over the whole diff, downgrade-only intelligence, bot-identity approvals at the exact head revision, policy as versioned closed-schema data with validator + kill switch, shadow-first; idempotent raceable merge automation honoring review-gated repos. — source: docs/prds/quality-and-merge.md GATE-4/GATE-5 (via brainstorm:argus-unattended-drive#F-5)
- **F-9**: The basis hands the designs exactly Q3..Q6 (wake mechanism + posture seam · lane mechanics · policy schema/home · materialization mechanics), Phase A needing Q3/Q6 and Phase B needing Q4/Q5; Q1/Q2 are operator-owned at their own steps and touch no design here. — source: prd:argus-unattended-drive §7.2
- **F-10**: I1 binds Serves hygiene (≥1 committed FR anchor that predates the design — satisfied: the basis froze at mainline before this session); I2 graduated into AP-26 (goal-object verification — already contract rows FR-6/UB-4/FR-12); I3 binds the classifier/validator CLIs to fail closed on misparse (UB-1/UB-2's mechanism-side twin). — source: docs/observability/incidents.md
- **F-11**: The authority contract today grants tier-2 (`dispose-blocking-finding` · `sign-gate-verdict` · `trigger-materialization`); `approve-merge` sits at tier-4, activating only after the operator changes org governance — the lane must compose with that boundary untouched (basis NR-3). — source: the authority contract at the agent's home (authority-contract/v1)
- **F-12**: The glossary already rules the feature's terms — AUTO lane / HUMAN lane, floor-clearing, wake signal, activation — plus the Argus set (drive touchpoint, decision principal); this session and the designs use them verbatim, never synonyms. — source: docs/wiki/glossary.md

## Fog

- **G-1** (non-blocking — blocks: wake-channel-v1's own Q-n): per-adapter push wiring — which completion events each code-host/CLI pairing can deliver and how the host receives them; the backstop guarantees the bound regardless, so the direction stands without it
- **G-2** (non-blocking — blocks: merge-lane-v1's own Q-n): the shadow agreement report's exact format and computation cadence — the records it reads from are decided (D-3); the report's shape is implementation-grade
- **G-3** (non-blocking — blocks: wake-channel-v1's own Q-n): when the await grammar graduates to a first-class derived-block row via a delivery-ledger template version bump (argus-artifacts §3.8's successor-design rule) — deferred on volume evidence, same staging every record class took

## Out of scope

- **O-1**: performing or automating the org backstop flip, the AUTO-class ratification, or any tier grant — operator acts, read as preconditions (FR-9) and surfaced as the activation brief (from D-7; basis NG-1/NG-2; problem:argus-unattended-drive#A-1)
- **O-2**: weakening, replacing, or re-implementing any existing mechanical check or the HUMAN lane — the lane composes over the same floor (from D-7; basis NG-3, NR-1)
- **O-3**: modifying the executing fleet's internals — the tracker convention stays the whole interface (from D-6, D-7; basis NG-4)
- **O-4**: engine-era machinery — a pyxis-native stage-8 engine, hosted lane services, platform-hosted wake sessions (from D-2, D-4, D-7; basis NG-5, WR-2)
- **O-5**: editing frozen designs or ledger classes in place — D-5's additive ruling holds NG-6 by construction; supersession stays the named channel on real contradiction (from D-5, D-7)
- **O-6**: a second telemetry plane for lane/wake/materialization events — verdict records, journal entries, and history self-description are the surfaces (from D-3, D-4; basis §5.7's posture)

## Candidate glossary terms

- **T-1**: **await** — the registered pair (goal object, completion condition) a drive publishes when it stops on an external completion; the wake channel's unit of work and UB-9's ageing subject · avoid: *pending item* (open items are broader), *subscription* (implies a broker)
- **T-2**: **backstop tick** — the bounded, scheduled delivery pass that reads the awaits, probes goal objects exactly, and delivers due touches; the latency guarantee under push-preferred delivery, ~0 when idle · avoid: *poller* as the mechanism's identity (polling is the guarantee layer, push is the optimization), *daemon* (nothing is resident)
- **T-3**: **shadow window** — the pre-activation measurement period, bound to classified-proposal count (≥ 20), over which lane-agreement and floor-violation evidence accumulates; closes by the ship rule, never by calendar · avoid: *trial period* (implies time), *beta* (implies exposure — shadow acts on nothing)

## Parked product gaps

none — the basis is freshly reviewed (terminal approve at the cap); no requirement hole surfaced during this session

## Gate

| Field | Value |
|---|---|
| **Verdict** | ready-for-design |
| **Date** | 2026-07-13 |
| **Authority** | Argus (decision principal, tier-2) — `sign-gate-verdict` granted per the authority contract, grant-checked at act time and journaled at delivery:argus-unattended-drive#E-28; not a human sign-off — the operator overturns through the normal channels |
| **Covering design** | — |
| **Coverage check** | pass — core four covered (§2 Decision direction: D-1/D-2/D-4/D-5/D-6, every DQ panel-logged · §4 Alternatives: ALT-1..ALT-11, every fired panel ≥ 2 scored entries, every entry verdicted · Serves/Scope: S-1..S-7 + the Basis anchors enumerated · §3.2 Boundaries: O-1..O-6), zero blocking fog (G-1..G-3 all non-blocking, each riding a named design Q-n) |
| **Amend route** | — |
