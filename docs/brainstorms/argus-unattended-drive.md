# Argus unattended drive — brainstorm record

| Field | Value |
|---|---|
| **In one line** | Make a delegated Argus drive run unattended end-to-end on this repo — an AUTO-approve/merge lane for floor-clearing PRs over the existing mechanical checks, the stage-8 fleet-binding correction with unattended materialization, and a wake mechanism that resumes the drive when an awaited external gate completes — while the org backstop flip and every tier grant stay operator acts. |
| **Relevant-when** | brainstorming or contracting unattended/autonomous drive mechanics (AUTO merge lanes, risk-tiered approval, wake/resume triggers, unattended tracker materialization); checking prior rulings about the org merge backstop, Argus tier grants, or the stage-8 fleet binding; triaging why a delegated drive stalls on operator toil. |
| **Idea** | for me the biggest missing piece right now is, making argus to go through all the steps by itself without waiting for me to approve/merge prs and prompting it to create tickets in github and tag them with local-agent for the implementation. by solving this problem with can implement all of the missing pieces much faster given we have governance documents in place. *(verbatim; the ratified scope pre-ruling is problem:argus-unattended-drive#A-1)* |
| **Operator** | @mdarabi (operator) — the decision authority of record |
| **Date(s)** | 2026-07-13 |
| **Status** | gated: ready-for-prd |
| **Tracking** | — *(the introducing PR, once filed)* |
| **Feeds** | docs/prds/argus-unattended-drive/ |
| **Prior records** | brainstorm:autonomous-developer, brainstorm:argus-boundary-enforcement, brainstorm:argus-attribution-hardening, brainstorm:argus-ledger-lint |
| **Template version** | brainstorm-record/v1 |

*Authoring note — agent principal (feature-brainstorm FR-16/FR-17). This record was driven by Argus as decision principal at tier-2, not by a live human interview. Per `.pyxis/authoring/argus.md`, each decision below was resolved by a **recorded, attributed assumption** that passed the legality tests (tier · channel · regression) and is cited to its `assumption-recorded` journal entry in `delivery:argus-unattended-drive`, OR would have been escalated as breaking had no legal assumption remained. The intake pre-ruling `problem:argus-unattended-drive#A-1` is a live operator ruling and binds throughout. Every `D-n` carries its authority anchor, standing until a human overturns it through the normal channels. No governance-tier content was authored or modified.*

## Orientation

| Source | Why loaded |
|---|---|
| docs/product-vision.md | §4 pipeline (human stages 2/5/9), §5 quality-and-merge capability row ("risk-tiered human/auto lanes"), §6 trust model — Level 3 names the risk-tiered AUTO merge lane; §11 push triggers + polling backstop |
| docs/argus-vision.md *(draft v0.2 — the operator's direction of record for the Argus program, under the product vision)* | §1 two stop classes (requests graduate; breaking escalations permanent), §2 the two composing dials (mechanical autonomy vs decision authority), §5 graduation re-tiers scrutiny, §9 the merge row activates only after the operator moves the org backstop |
| docs/prds/argus/effective.md | NG-1 (org governance = operator act), FR-9 (materialization through plan-new's contract + tag confirmation), FR-14 (promotions only at checkpoints), FR-15 (park/aging alarms at touchpoints), UB-1/UB-5, §6.5 ladder ("+ merge approval — operator changes the org backstop first") |
| docs/prds/quality-and-merge.md | GATE-4 (risk-tiered two-lane approval, as built in gen-1: deterministic floor → narrow classes → downgrade-only intelligence; bot identity; policy as data; shadow-first) + GATE-5 (idempotent merge automation honoring review-gated repos) — parity requirement sources |
| docs/tech-design/argus-agent-v1.md · docs/tech-design/argus-artifacts.md | the frozen drive protocol (session posture, touchpoints, DINV-4 "a parked problem holds no live session and no recurring process", DINV-5 any-CLI), the ledger contracts, §3.3 writer discipline |
| docs/architecture-principles.md | AP-2 (normalized wake signals), AP-10/AP-21 (codify; judge once, freeze as data), AP-17 (fail closed), AP-19 (idle costs nothing; recurring paths state their cost), AP-22 (idempotent/raceable), AP-24 (policy is human-authored input), AP-26 (goal-object verification, never badges) |
| docs/observability/incidents.md | I1 (a class's first gate can't gate its introducing PR), I2 (merged badges ≠ mainline — the auto-merge hazard class), I3 (consequential CLIs fail closed on misparse) |
| .pyxis/pipeline.md | the stage table — stage 8 reads `⛔ not built`; the footer already names "an external factory" |
| docs/problems/argus-ledger-lint/delivery.md · docs/problems/argus-boundary-enforcement/delivery.md · docs/problems/argus-attribution-hardening/delivery.md | the empirical toil record: E-36's materialization deferral on the stale `⛔` row; every artifact PR operator-merged; projection operator-directed; fleet PRs #193–#196 and #206–#210 later landed both plans |
| AGENTS.md · .pyxis/config.yaml | the standing repo merge convention ("every PR needs a human approval and a human merge") and the closed config surface the lane's policy data would join |
| docs/wiki/glossary.md | the established vocabulary; candidate terms checked against it |

**Overlaps surfaced:**
- **GATE-4 / GATE-5 (quality-and-merge baseline)** — the AUTO/HUMAN lane and merge automation exist as *parity requirement sources* (gen-1 as-built inventory), not as shipped pyxis capability; the placeholder corpus is exactly what a real PRD cites (`AGENTS.md` rule 8). *Requirement source, not covering capability — proceed.*
- **prd:argus NG-1 + §6.5 ladder** — the org backstop and the `approve-merge` tier row are *reserved operator ground*; this feature must build around them, never through them. A-1 re-affirms the same boundary. *Boundary honored, not moved.*
- **prd:argus FR-9 + granted `trigger-materialization`** — the materialization *behavior* is already contracted and granted; what is missing is the unattended *timing* (no operator prompt) and the corrected stage-8 map that made prior drives defer it (delivery:argus-ledger-lint#E-36). *Extends, does not duplicate.*
- **prd:argus WR-1/WR-2** — binding to pyxis-native execution stages and the tracker-item front door stay parked there; this feature corrects the *map* to the operational external binding and wires the wake, without building either WR item. *Disjoint.*
- **product-vision §6 Level 3** — the AUTO lane is the vision's own trust-ladder rung, not new ground. No covering source exists in pyxis (nothing implements it here).

*Conclusion: no covering source; proceed to framings.*

## Framings

- **Framing A — Three separate features (a program).** The merge lane, the stage-8 correction + unattended materialization, and the wake each get their own PRD. *Implications:* maximal decomposition, three contracts and review cycles; but the three gaps are one delegation failure (the drive stalls on operator toil at three different joints), the ratified scope names them as one problem, and fragmenting loses the end-to-end fit criterion (a whole problem delivered unattended) that only the composition can carry.
- **Framing B — One feature: the unattended drive loop (chosen).** The unit is *the set of non-owned human stop points in a delegated drive*; one PRD contracts closing all three (lane, fleet-bridge correction, wake) with one end-to-end success measure, decomposed into separate technical designs downstream (RW-5). *Implications:* one contract carries the honest ownership partition (toil removed vs operator ground preserved); the composition is testable as a whole; matches the one-brainstorm→one-PRD intake shape used by every prior Argus drive.
- **Framing C — Promote Argus instead (tier-3/tier-4 now).** Read the idea as a trust problem: grant `rule-acceptance`/`approve-merge` and let Argus merge its own work. *Rejected as a category error:* tier moves are operator acts at evidenced checkpoints (prd:argus FR-14; the never-list forbids self-grant), A-1 explicitly keeps every tier grant an operator act, and even a tier-4 Argus could not remove the org backstop (NG-1). The missing piece is factory machinery, not a trust jump.

**Destination:** **Framing B** — one PRD contracting the unattended drive loop on this repo: (1) the AUTO-approve/merge lane for floor-clearing PRs over the existing mechanical checks, (2) the stage-8 binding correction + unattended materialization, (3) the wake mechanism — with the org backstop flip and all tier grants remaining operator acts, surfaced as briefs. *Rationale (Argus, tier-2 assumption D-1):* the three clusters share one subject (the delegated drive's non-owned stops) and one governing direction (A-1 + the delegation test); the end-to-end fit criterion exists only at the composition; design-stage decomposition preserves RW-5.

## Decisions

### D-1 — Destination: one feature, three clusters — the unattended drive loop

- **Question**: Is this one feature (one PRD, three requirement clusters) or a program of three?
- **Answer**: ONE feature — Framing B. Clusters: the AUTO/HUMAN merge lane; the stage-8 binding correction + unattended materialization; the wake mechanism. Downstream designs decompose (each cluster likely its own technical design, RW-5).
- **Reasoning**: A-1 ratifies the three as one scope ("unattended end-to-end on this repo"); the delegation test (argus-vision §1) charges all three stops to one gap class; the end-to-end success measure (a delegated problem delivered with zero non-owned operator moments) is a property of the composition, not of any part. Mirrors the argus-boundary-enforcement precedent (one PRD, three clusters, three designs).
- **Anchor**: 2026-07-13 · Argus assumption (delivery:argus-unattended-drive#E-2)
- **Status**: live

### D-2 — The unattended boundary is drawn by ownership, not ambition: toil stops close; owned stops stay forever

- **Question**: Which human moments does this feature remove, and which must it structurally preserve?
- **Answer**: It removes only **non-owned toil stops**: (a) approve/merge of *floor-clearing* PRs (mechanical-verdict work), (b) prompting/directing plan materialization after the plan freezes, (c) re-invoking the drive after an awaited external completion. It **preserves, permanently**: breaking-decision briefs (`A-n` answers), the org backstop flip, every tier grant/promotion, HUMAN-lane review of every non-floor-clearing PR, overturns and audit-by-sample. Unattended = zero *toil*, never zero *authority*.
- **Reasoning**: Largely determined by A-1 (the flip and grants "remain operator acts, surfaced as briefs") and argus-vision §1's two stop classes (requests graduate; breaking escalations are permanent at every tier). The assumption content is the precise partition of today's observed stops (the ledgers' record) into the two classes — each of (a)/(b)/(c) is a request-class stop backed by a mechanical or already-granted act, none is operator ground.
- **Anchor**: 2026-07-13 · Argus assumption (delivery:argus-unattended-drive#E-3)
- **Status**: live

### D-3 — Cluster 1 shape: the AUTO lane is the factory's mechanical lane (GATE-4/GATE-5 shape), never an Argus grant; activation is the operator's

- **Question**: What is the AUTO-approve/merge lane — an Argus tier-4 act, or factory machinery — and what makes it safe?
- **Answer**: The lane is **the installation's mechanical-autonomy dial** (product-vision §6 Level 3), in the GATE-4/GATE-5 shape: a **deterministic floor** (versioned policy data; any hit → HUMAN, non-negotiable — control-plane paths, governance docs, the lane's own policy, the authority contract), **narrow closed AUTO classes** on floor-clearing PRs, optional **downgrade-only** intelligence, approval+merge by a **dedicated machine identity** at the exact head revision, **post-merge goal-object verification** (mainline ancestry, I2/AP-26), policy with validator + kill switch, **shadow mode first**. Argus's own `approve-merge` class stays withheld and untouched; the lane's *enforcement activation* requires the operator's recorded acts (the org backstop flip + ratification of the initial AUTO-eligible class set) and the feature **refuses enforcement without them** (fail-closed activation gate). Which artifact classes are AUTO-eligible is **policy data the operator ratifies at activation** (recommendation recorded: floor-clearing fleet task PRs, and drive doc PRs carrying green gates + an approving reviewer verdict).
- **Reasoning**: A-1's wording ("an AUTO-approve/merge lane … over the existing mechanical checks" while "all tier grants remain operator acts") selects the factory-lane reading; argus-vision §2 makes the two dials compose without substituting; GATE-4/GATE-5 are the paid-for as-built shape (parity source); AP-21 puts the eligible-set judgment at the richest-context site (the operator, at activation) frozen as data; AP-24 keeps the policy human-ratified input. **Escalation honesty:** not breaking — everything contracted here is machinery + shadow evidence, inert until the operator's own acts move the boundary; the brief for the flip is owed at the activation step, exactly as A-1 directs.
- **Anchor**: 2026-07-13 · Argus assumption (delivery:argus-unattended-drive#E-4)
- **Status**: live

### D-4 — Cluster 2 shape: correct the stage-8 map to the operational external-fleet binding; materialize unattended through plan-new's own contract

- **Question**: What unblocks the fleet bridge — and is editing `.pyxis/pipeline.md` legal?
- **Answer**: Two parts. (a) **The map correction**: the stage-8 row moves from `⛔ not built` to an honest ◐ — *implementation runs today through the external fleet binding: the frozen plan projects onto the tracker with the repo-configured automated-implementation tag; an external fleet claims and implements; the pyxis-native engine remains not built*. (b) **Unattended materialization**: when a plan freezes at merge, the drive triggers projection through plan-new's own contract (consent, idempotency, wiring order stay the stage's) without an operator prompt, then verifies each created item at the goal object (exists + carries the tag) — prd:argus FR-9's exact behavior, minus the waiting-for-direction convention.
- **Reasoning**: The `⛔` row is empirically stale: it drove E-36's deferral ("orphan issues no fleet claims"), yet the fleet has since claimed and implemented both projected plans (PRs #193–#196; epic #200 → #206–#210) — F-2/F-3. **Legality of the map edit:** the map's own header says it is descriptive and "corrected to match"; product-vision §4 stage 8 (actor: Factory) is not contradicted — an external factory is a factory actor and the map's footer already names it; the map is not a frozen design (no supersession owed). The materialization behavior is already contracted (FR-9) and granted (`trigger-materialization` ∈ tier-2); this feature contracts its *unattended timing* — a new-feature channel, breaking no unamended contract (projection stays gated on the *merged* plan, plangate DINV-1).
- **Anchor**: 2026-07-13 · Argus assumption (delivery:argus-unattended-drive#E-5)
- **Status**: live

### D-5 — Cluster 3 shape: the wake is a normalized, instruction-free signal that re-enters the front door; parked problems keep zero steady-state cost

- **Question**: What is the wake mechanism at contract level, and how does it avoid the daemon the frozen behavior design rejects?
- **Answer**: Contract the **behavior**, not the mechanism: while a drive is open and awaiting a named external completion (an artifact PR merged, checks settled, a fleet item claimed/landed, an `A-n` answer appended), the completion reaches the drive as a **normalized wake signal** — slug + touch reason only, never instruction content — and the drive resumes at the front door through the existing resume protocol (replay + goal-object verification) within a configured latency bound. **Parked** problems wake only on the answer append or a human touch (their zero steady-state cost is preserved); a **bounded, stated-cost backstop** (e.g. a scheduled touch) guarantees no awaited completion is lost to a missed signal. Mechanism selection (webhook/CI-completion trigger vs bounded poll vs harness scheduler, per CLI) is a stage-3/4 design decision, parked as P-2.
- **Reasoning**: AP-2 sanctions exactly this shape (triggers deliver a normalized wake signal, never logic); product-vision §11 names push triggers with the polling backstop; AP-19 demands the stated steady-state cost — the contract binds outcome + cost envelope, not mechanism. The frozen argus-agent-v1 DINV-4 protects *parked* problems (no live session, no recurring process) — honored verbatim; where the design-stage mechanism needs to touch the frozen behavior design's session-posture prose, it evolves by supersession through channels (P-2 carries the seam), never by silent contradiction.
- **Anchor**: 2026-07-13 · Argus assumption (delivery:argus-unattended-drive#E-6)
- **Status**: live

### D-6 — Segments and anti-segments

- **Question**: Who is this for, and who must it deliberately not touch?
- **Answer**: **Primary:** the operator (the toil stops disappear; what remains is genuinely theirs) and the delegated drive itself (Argus — the end-to-end thread completes). **Secondary:** the executing fleet (tagged items arrive at plan-freeze, not at operator convenience). **Anti-segments:** the self-driving human engineer (the human-driven path stays byte-identical — HUMAN lane, human merges, nothing forces AUTO), and every non-floor-clearing PR (control-plane, governance, policy, high-risk classes — always HUMAN).
- **Reasoning**: argus-vision §3's segment set carried to this feature; the anti-segment guarantee is what keeps the tier test clean (no standing human path degrades). Determined largely by the governing docs; the assumption is only the enumeration.
- **Anchor**: 2026-07-13 · Argus assumption (delivery:argus-unattended-drive#E-7)
- **Status**: live

### D-7 — Scope perimeter and non-goals

- **Question**: What is in, and what are the deliberate refusals/deferrals?
- **Answer**: **In:** the three clusters as one behavioral contract, scoped to this repo. **Non-goals:** (a) performing the org backstop flip or any branch-protection/AGENTS.md merge-rule change — operator acts, surfaced as briefs (A-1; prd:argus NG-1); the AGENTS.md merge-rule row is re-cut only by the operator's own activation change; (b) granting, changing, or activating any Argus authority tier (incl. `approve-merge`) — operator acts at checkpoints (FR-14; never-list); (c) weakening, removing, or bypassing any existing mechanical check or the human lane — the AUTO lane adds a lane over the same floor; (d) modifying the executing fleet's internals (prd:argus NG-3 — the tracker convention is the whole interface); (e) building the pyxis-native stage-8 engine (stays prd:argus WR-1 / engine era); (f) editing any frozen design in place — where the wake mechanism needs the frozen behavior design to move, it supersedes through channels.
- **Reasoning**: Every refusal quotes a standing boundary (A-1, NG-1, NG-3, FR-14, the never-list, AP-7) — the perimeter is inherited, not invented. NG-(c) is the load-bearing safety refusal: unattended must never mean less-checked.
- **Anchor**: 2026-07-13 · Argus assumption (delivery:argus-unattended-drive#E-7)
- **Status**: live

### D-8 — Appetite: bounded factory machinery on settled substrate; shadow-first; named rabbit holes

- **Question**: What total spend is this problem worth?
- **Answer**: Bounded. **Machine work:** the lane as policy-data + deterministic classifier + merge automation over the *existing* check surface (docs-gate CI, plangate, the argus gate arms, CI contexts — nothing new invented at the floor), the one-row map correction with its honest footer, the wake glue (repo-internal trigger wiring + the bounded backstop), the unattended-materialization convention change, fixtures/drills for all three. **Human attention:** one PRD review, up to three downstream design reviews, and the operator's own activation acts (the flip + policy ratification) when they choose. **Rollout burden:** shadow mode first for the lane (classify + log, act on nothing) with agreement measurement; enforcement only after the operator's acts. No new egress, identity-in-Argus's-hands, or outward surface — anything needing one escalates instead (UB-5).
- **Reasoning**: The floor reuses settled substrate (the mechanical checks exist and are green today); the genuinely new costs are the lane's policy/identity mechanics and the wake's cross-CLI wiring — both named rabbit holes: **R-a** badge-trust (I2 — goal-verify every AUTO merge), **R-b** idle cost (AP-19 — the wake must state its steady-state cost, ~0), **R-c** self-unlock (the lane's own policy riding the AUTO lane — always HUMAN).
- **Anchor**: 2026-07-13 · Argus assumption (delivery:argus-unattended-drive#E-8)
- **Status**: live

## Facts

- **F-1**: `.pyxis/pipeline.md` stage 8 reads "the engine / worker fleet · ⛔"; its footer already states stages 8–11 "run outside pyxis today (by hand or an external factory)". — source: .pyxis/pipeline.md
- **F-2**: The external fleet has twice claimed and implemented this repo's projected plans: argus-ledger-lint plan (merged #187) → implementation PRs #193/#194/#195/#196 (items #189–#192); argus-boundary-enforcement plan (merged #199) → epic #200 → PRs #206/#208/#209/#210 (items #201/#203–#205). — source: git history (commits 23311ae, 77e8fdb, b3c7023, 367f3bc, d335981, 7b46c53, 03e5485, d36064a); delivery:argus-attribution-hardening#E-18
- **F-3**: delivery:argus-ledger-lint#E-36 deferred real materialization *because* the map row read `⛔` ("a real projection would create orphan `local-agent` issues no fleet would claim") — the exact prompting-toil moment the idea names; projection then happened under operator direction and was claimed. — source: docs/problems/argus-ledger-lint/delivery.md
- **F-4**: product-vision §6 Level 3: "Auto-merge low-risk changes once gates pass … the risk-tiered AUTO merge lane (low-risk classes only; everything else stays human)"; §5: "risk-tiered human/auto lanes". — source: docs/product-vision.md
- **F-5**: GATE-4 (as built, gen-1): deterministic floor with hard-gate path globs + **self-unlock guard** (changes to the risk policy/classifier are always HUMAN) + foundation floor; narrow AUTO classes; downgrade-only LLM; reviewer **bot identity** approvals at the exact head revision; policy = versioned closed-schema data with validator + kill switch; shadow-mode first. GATE-5: idempotent, raceable merge automation; review-gated repos honored (bot merges only once approval + green checks present). — source: docs/prds/quality-and-merge.md
- **F-6**: prd:argus NG-1: changing org/repo governance is "an operator act, never the feature's; the graduated merge row activates only after the operator changes it"; §6.5 ladder: "+ merge approval … operator changes the org backstop first (NG-1)". — source: docs/prds/argus/effective.md
- **F-7**: argus-vision §5: "Graduation re-tiers scrutiny; it never removes it. A human gate that graduates away is replaced by an adversarial machine check, never by nothing"; §2: the autonomy-level dial (what an installation may **do**) and the authority-contract dial (what a delegate may **decide**) compose, neither substitutes. *(Draft v0.2, operator-authored; under the product vision.)* — source: docs/argus-vision.md
- **F-8**: AP-2 — triggers deliver a normalized wake signal and never carry logic; AP-19 — idle costs ~nothing and every recurring path states its steady-state cost; AP-26 — a merged badge certifies the PR's base, never the mainline (incident I2's lesson, graduated). — source: docs/architecture-principles.md; docs/observability/incidents.md I2
- **F-9**: argus-agent-v1 DINV-4: "A parked problem holds no live session and no recurring process; bumps evaluate only at touchpoints"; its §4 rejects a daemon/scheduler watching parks (AP-19); DINV-5 requires any harness primitive to have a stated sequential fallback. — source: docs/tech-design/argus-agent-v1.md
- **F-10**: `trigger-materialization` is in tier-2's bundle (granted today); prd:argus FR-9 contracts materialization through the planning stage's own contract with tag confirmation at the goal object. — source: .pyxis/skills/argus/authority-contract.md; docs/prds/argus/effective.md FR-9
- **F-11**: AGENTS.md carries the standing repo convention "Every PR needs a human approval and a human (write-access) merge — do not merge your own work"; the lane's enforcement therefore activates only together with the operator's own change to that convention (part of the flip, NG-(a)). — source: AGENTS.md
- **F-12**: The argus write hook, attribution gate, structure arm, and ledger arms are shipped and enforce Argus's boundaries mechanically at desk and merge boundary — the enforcement layer the unattended loop rides on already exists. — source: internal/argus/* · internal/docgate/authoritycontract · docs/tech-design/argus-write-hook.md · argus-attribution-gate.md · argus-authority-contract-structure-arm.md

## Fog

- **G-1** (non-blocking — blocks: stage-3/4 design detail): how the lane composes with a *future* operator-granted Argus `approve-merge` (tier-4) — precedence, attribution, and whether the lane's machine identity remains the sole merge actor. Nothing now: tier-4 is inactive and stays operator ground; lands as a PRD open question.
- **G-2** (non-blocking — blocks: activation step): the exact initial AUTO-eligible class set the operator ratifies at activation (recommendation recorded in D-3); until ratified, shadow mode measures against the recommended set. Lands as a PRD open question + rollout-stage input.

## Out of scope

- **O-1**: Performing the org backstop flip (branch protection, required human approval) or the AGENTS.md merge-rule change — operator acts, surfaced as briefs at the activation step. (from D-7; A-1, prd:argus NG-1)
- **O-2**: Granting, changing, or activating any Argus authority tier, including `approve-merge` — operator acts at evidenced checkpoints; the ladder and never-list are untouched by this feature. (from D-3, D-7)
- **O-3**: Weakening, removing, or bypassing any existing mechanical check, required context, or the HUMAN lane — the AUTO lane is additive over the same floor. (from D-7)
- **O-4**: Modifying the executing fleet's internals — the tracker convention (projected, tagged items) remains the whole interface. (from D-7; prd:argus NG-3)
- **O-5**: Building the pyxis-native stage-8 engine/worker fleet — stays in prd:argus WR-1 / the engine era; the corrected map names the external binding honestly instead. (from D-4, D-7)
- **O-6**: Editing any frozen design in place — the wake's session-posture seam, if the design stage needs it, moves by supersession through channels. (from D-5, D-7)

## Candidate glossary terms

- **T-1**: **AUTO lane / HUMAN lane** — the two merge lanes of the risk-tiered approval split: AUTO = a dedicated machine identity approves and merges a floor-clearing PR once every mechanical check is green; HUMAN = the default lane, a human approval + merge. Avoid: "auto-merge" as a bare verb (names the act, not the lane), "fast lane" (implies speed, not risk class).
- **T-2**: **Floor-clearing** — of a PR: clearing the deterministic floor — zero hard-gate hits (control-plane paths, governance docs, the lane's own policy, the authority contract), an AUTO-eligible change class, and every required mechanical check green. The floor is versioned policy data; any hit routes HUMAN, non-negotiable. Avoid: "low-risk" as a synonym (the floor is computed, not felt).
- **T-3**: **Wake signal** — the normalized, instruction-free notification (slug + touch reason) that an awaited external completion has occurred, re-entering a drive at its front door; the drive then trusts only its own resume protocol (replay + goal-object verification), never the signal's content. Avoid: "trigger with payload", "callback" (both imply carried logic — AP-2 forbids it).
- **T-4**: **Activation (of the AUTO lane)** — the operator's recorded enforcement flip: the org backstop change plus ratification of the initial AUTO-eligible class set; before it, the lane runs shadow-only. Avoid: "rollout" (activation is one owned act, not a percentage).

## Parked technical constraints

- **P-1**: **Lane mechanics per medium.** How a dedicated machine identity approves and squash-merges under this repo's branch protection (bot App vs deploy identity; the exact required-context set read from the medium; idempotent/raceable per GATE-5; "awaiting review" logged distinctly). Medium-neutral vocabulary at contract level; the GitHub binding is the first adapter. (stages 3–4)
- **P-2**: **Wake mechanism + the frozen-design seam.** Push (webhook/CI-completion) vs bounded poll backstop vs harness scheduler, per CLI, with DINV-5 sequential fallback; the steady-state cost statement (AP-19); and the supersession seam if argus-agent-v1's session-posture prose must move to admit mechanical touchpoints. (stages 3–4)
- **P-3**: **Risk-policy data schema.** The floor's closed schema (hard-gate globs, AUTO classes, self-unlock set), its validator, kill switch, and home on the config surface (AP-8/AP-24 — operator-ratified content); how shadow-mode verdicts are recorded as data for the agreement measurement. (stages 3–4)
- **P-4**: **Unattended materialization timing.** Where the plan-freeze completion is observed (the wake's own signal class), how projection idempotency behaves under racing wakes (AP-22), and the item-verification loop at the goal object (FR-9's confirm arm). (stages 3–4)

## Gate

| Field | Value |
|---|---|
| **Verdict** | ready-for-prd |
| **Date** | 2026-07-13 |
| **Operator** | Argus (decision principal, tier-2) — `sign-gate-verdict` GRANTED at tier-2; verdict signed by Argus per the authority contract, journaled at delivery:argus-unattended-drive#E-10. Not a human sign-off; the operator overturns through the normal channels. |
| **Covering source** | — |
| **Coverage check** | pass — all six decision-shaped inputs covered (§2.1 problem via Destination/D-1/D-2; §2.2 segments via D-6; §2.3 jobs via D-2/D-3/D-4/D-5 — the three toil stops as jobs; §3.1 goals direction via D-2/D-3 and the end-to-end fit; §3.4 non-goals via the O-1..O-6 set; header appetite via D-8), zero blocking fog (G-1, G-2 both non-blocking). |
