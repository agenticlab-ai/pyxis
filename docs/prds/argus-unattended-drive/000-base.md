# Argus unattended drive — PRD

| Field | Value |
|---|---|
| **In one line** | Make a delegated Argus drive run unattended end-to-end on this repo: a risk-tiered AUTO-approve/merge lane for floor-clearing proposals over the existing mechanical checks, the corrected stage-8 fleet binding with unattended plan materialization, and a wake mechanism that resumes a drive when an awaited external completion occurs — with the org backstop flip and every Argus tier grant remaining operator acts, surfaced as briefs. |
| **Relevant-when** | planning, building, or reviewing unattended-drive machinery (the AUTO/HUMAN merge lanes, the risk-policy floor, wake/resume triggers, unattended materialization); triaging a delegated drive stalled on operator toil, a wrong-lane routing, a missed wake, or an unverified auto-merge; changing merge conventions, branch protection, the risk policy, or the stage-8 fleet binding. |
| **Status** | Draft — problem and solution gates signed by Argus (decision principal, tier-2; delivery:argus-unattended-drive#E-13); the introducing PR's operator review + merge is the freezing event |
| **Tier** | T3 — factory-control surface (the merge gate, risk policy, and activation state are exactly the "governance, gates, policy" critical-risk class); full rigor including abuse cases and evals. |
| **Owner** | @mdarabi (operator) |
| **Approver(s)** | @mdarabi (operator) — factory-control surface; the introducing PR's operator review is the single real gate at this team size (Appendix A rule 15). |
| **Contributors** | @mdarabi (operator / product / engineering); Argus (AI Engineer, decision principal at tier-2 — authored this contract from the gated record `brainstorm:argus-unattended-drive`). |
| **Informed** | — |
| **Date** | 2026-07-13 created · 2026-07-13 last updated |
| **Appetite** | Bounded. Machine work: the lane as versioned policy data + a deterministic classifier + merge automation over the existing mechanical check surface; the one-row pipeline-map correction with its honest footer; the wake glue (normalized signal + bounded backstop); the unattended-materialization timing change; fixtures and live drills for all three clusters. Human attention: operator review of this PRD, up to three downstream technical designs, and the operator's own activation acts (the org backstop flip + ratification of the initial AUTO-eligible class set) whenever they choose. Rollout burden: shadow mode first for the lane (classify + record, act on nothing); enforcement only after activation. Excludes: any new egress or any credential in Argus's hands; performing the activation acts themselves (operator-owned); the pyxis-native stage-8 engine. |
| **Target release** | No calendar target. Gated on the shipped Argus enforcement arms (merged) and the twice-proven external fleet binding (observed). Checkpoints, not dates, bind the metrics (§3.2); the enforce-mode targets bind to the operator's activation, which this feature never performs. |
| **Tracking** | — *(the introducing PR / epic, once filed)* |
| **Artifacts** | Brainstorm record `docs/brainstorms/argus-unattended-drive.md` (the gated stage-1 session); the drive ledgers `docs/problems/argus-unattended-drive/`. |
| **Template version** | prd-template/v1 |

## 1. Summary — the one-pager

- **Problem** — A delegated drive today stalls on the operator at three joints that are pure toil, not judgment: every artifact PR waits for a human approve + merge even when its verdict is fully mechanical; a frozen plan waits for the operator to prompt its projection onto the tracker (the stale stage-8 map row taught the drive to defer it); and after every awaited completion the drive waits for the operator to re-invoke it. The operator is the factory's clock and button — the delegation test charges every one of those moments as a gap.
- **For whom** — The operator (whose non-owned touches drop to zero) and the delegated drive itself (which completes end-to-end); explicitly not for: the self-driving human engineer, whose path stays byte-identical, and any non-floor-clearing proposal, which always gets a human.
- **Why now** — The governance layer this depends on is in place: the boundary-enforcement and attribution-hardening arms are merged (mechanical, author-independent gates), the unmanned reviewers produce complete verdicts, the fleet binding is twice-proven live (plans #187 and #199 both implemented by the external fleet through tagged projection), and the operator has ratified the scope (problem:argus-unattended-drive#A-1). Every additional drive run attended re-pays the same toil.
- **Direction** — Three clusters, one loop. (1) A risk-tiered **AUTO/HUMAN merge lane** in the proven GATE-4/GATE-5 shape: a deterministic, versioned policy floor (any hit → HUMAN, non-negotiable), narrow closed AUTO classes, optional downgrade-only intelligence, approval + merge by a dedicated machine identity at the exact head revision, post-merge goal-object verification, shadow mode first, and a fail-closed activation gate — enforcement is impossible until the operator's own recorded acts. (2) The **stage-8 binding correction**: the pipeline map states the operational external-fleet binding, and the drive materializes a frozen plan through the planning stage's own contract without a prompt, verifying items and tags at goal objects. (3) The **wake**: a normalized, instruction-free signal re-enters a drive at its front door when an awaited completion occurs, with a bounded, stated-cost backstop — parked problems keep zero steady-state cost.
- **Appetite** — Policy data + classifier + merge automation over existing checks, a one-row map correction, wake glue, drills; operator attention = this review, up to three design reviews, and the activation acts at their choosing. Excludes new egress, credentials in Argus's hands, and the activation acts themselves.
- **Success** — Operator toil touches per delegated problem (non-owned moments outside the north star's four) drop from the observed baseline of five or more per drive to **zero** for a floor-clearing problem, at the checkpoint "first delegated problem delivered end-to-end with the lane enforcing"; the shadow-stage proxy is lane-agreement ≥ 95% over the shadow window with zero would-have-been-AUTO floor violations.
- **Guardrail** — Zero floor-violating AUTO merges, ever (zero-tolerance: one occurrence trips the kill switch and an incident); every existing mechanical check's verdict stays byte-identical; the human-driven path is untouched.
- **Top risks** — (R1) badge-trust: an auto-merge trusted from a platform status while the mainline lacks the content (incident I2's class) — mitigated by mandatory goal-object verification and the halt rule. (R2) floor evasion: a crafted diff presents as AUTO-eligible while touching protected surface — mitigated by the deterministic path-derived floor, whole-diff classification, fail-closed ambiguity, and the shipped attribution/write-hook arms beneath. (R3) the wake re-imports idle cost or a daemon — mitigated by contracting outcome + cost envelope (AP-19), event-driven-first with a bounded stated-cost backstop, and park semantics preserved verbatim.

## 2. Problem space

### 2.1 Problem statement

The operator sits inside the delegated drive's loop at three places where no judgment of theirs is being exercised. First, artifact PRs: in the three drives run to date, every brainstorm/PRD/design/plan/ledger PR waited for the operator to approve and merge — including PRs whose entire verdict was already computed by the mechanical gates and the unmanned reviewers. Second, materialization: the drive's own map told it stage 8 was not built, so it deferred projection and the operator had to prompt it out-of-band — twice — after which the external fleet promptly claimed and implemented everything. Third, resumption: after each of those waits, the drive sat until the operator re-invoked it by hand. Each wait costs hours-to-days of latency per artifact, multiplies across the pipeline's stages, and — by the argus-vision's delegation test — is a recorded gap, not a chore. The cost of doing nothing is the whole difference between a factory and a very patient assistant: the missing pieces of pyxis itself are queued behind exactly this toil.

### 2.2 Users and segments

| Segment | Role and context | Domain exp. | Tech exp. | Key goals | Pain today | Evidence |
|---|---|---|---|---|---|---|
| **P1 (primary) — the operator** | @mdarabi; grants authority, answers briefs, reviews what matters; currently also the merge button, the projection prompter, and the resume clock | master | master | spend attention only on genuinely-owned calls (briefs, activation, audit); stop being the factory's clock | five-plus non-owned touches per drive; days of artifact latency; drives silently waiting on them | the three drive ledgers (§2.4); brainstorm:argus-unattended-drive#D-2 |
| **P2 (primary) — the delegated drive (Argus)** | the AI Engineer driving a problem end-to-end as decision principal at tier-2 | master | master | complete the thread problem→delivered without stalling on non-owned stops; keep every act journaled and verified | must stop at every artifact merge, defer materialization, and wait for re-invocation | delivery:argus-ledger-lint#E-36; delivery:argus-boundary-enforcement (stage waits) |
| P3 (secondary) — the executing fleet | the external fleet claiming tagged tracker items | — | — | receive items at plan-freeze, not at operator convenience | items arrive only when the operator prompts projection | brainstorm:argus-unattended-drive#F-2/#F-3 |
| **Anti-segment** | the **self-driving human engineer** and every **non-floor-clearing proposal** | — | — | — | — | the human-driven path stays byte-identical (HUMAN lane default, human merges untouched); control-plane/governance/policy diffs are always HUMAN (§5.1 FR-2) |

### 2.3 Jobs and desired outcomes

| # | When… (situation) | I want to… (motivation) | So I can… (outcome) | Today's workaround | Evidence |
|---|---|---|---|---|---|
| J1 | a drive's artifact or fleet proposal is floor-clearing — zero policy hits, every required mechanical check green, an approving reviewer verdict where one applies | have it approved and merged by the factory's machine lane, attributably, at the exact evaluated revision | stop paying a human approve/merge for a verdict already computed mechanically | the operator merges every PR by hand (observed in all three drives) | quality-and-merge baseline GATE-4/GATE-5; product-vision §6 Level 3 |
| J2 | a plan freezes at its merge | have the drive project it through the planning stage's own contract and verify the tagged items, with no prompt | get the fleet building at plan-freeze, not at operator convenience | the operator prompts/directs projection out-of-band (twice observed) | delivery:argus-ledger-lint#E-36; prd:argus FR-9 |
| J3 | an awaited external completion occurs (artifact PR merged, checks settled, fleet items landed, an answer appended) | have the drive resume itself at the front door through its resume protocol | keep the thread moving without the operator as the clock | the operator re-invokes `/argus <slug>` by hand after each completion | the three drive ledgers' reconciliation entries |
| J4 | a proposal is NOT floor-clearing (control-plane, governance, policy, unratified class) | have it routed to a human, non-negotiably | keep human authority exactly where risk demands it | today everything is human (over-broad, but safe) | brainstorm:argus-unattended-drive#D-2/#D-3 |

### 2.4 Evidence

| Claim | Source (research, data, tickets, interviews) | Confidence |
|---|---|---|
| Every artifact PR of the three completed/ongoing drives was operator-approved and operator-merged | the drive ledgers: delivery:argus-ledger-lint (#184–#187 waits at E-11/E-19/E-31), delivery:argus-boundary-enforcement (#197–#199 waits at E-13/E-28), delivery:argus-attribution-hardening (open plan-PR wait) | High |
| The drive deferred real materialization because the stage-8 map row read "not built", and the operator had to direct projection out-of-band | delivery:argus-ledger-lint#E-36 (the deferral, verbatim reasoning); brainstorm:argus-unattended-drive#F-3 | High |
| The external fleet twice claimed and implemented this repo's projected plans end-to-end | git history: plan #187 → implementation PRs #193/#194/#195/#196; plan #199 → epic #200 → PRs #206/#208/#209/#210 (brainstorm:argus-unattended-drive#F-2) | High |
| The AUTO merge lane is the product vision's own trust-ladder rung, not new governance ground | product-vision §6 Level 3 ("Auto-merge low-risk changes once gates pass … risk-tiered AUTO merge lane"); §5 quality-and-merge row | High |
| A proven as-built shape for the lane exists (deterministic floor, narrow classes, downgrade-only intelligence, bot identity, policy-as-data, shadow-first, idempotent merge automation) | quality-and-merge baseline GATE-4 + GATE-5 (gen-1 as-built inventory) | High |
| The org backstop and every Argus tier grant are operator ground; the ratified scope keeps them so | problem:argus-unattended-drive#A-1; prd:argus NG-1 + §6.5 ladder; argus-vision §9 | High |
| The mechanical enforcement layer beneath the lane is shipped (write hook, attribution gate, authority-contract structure arm, ledger arms) | the merged boundary-enforcement implementation (brainstorm:argus-unattended-drive#F-12) | High |
| A platform merged badge can be true while the mainline lacks the content — the auto-merge hazard class | incident I2; AP-26 | High |
| Parked problems must hold no live session and no recurring process | argus-agent-v1 DINV-4; AP-19 | High |

### 2.5 The journey today

| Phase | User does | User feels / pain | Frontstage (what user sees) | Backstage (systems, people, processes) |
|---|---|---|---|---|
| Author | the drive authors an artifact, gates green, opens/queues a PR | (drive) done, verified, waiting | a PR with green checks and a reviewer verdict | docgate/plangate/CI have already computed the verdict |
| Wait: merge | the operator approves + merges, whenever they next look | (operator) button-pressing; (drive) stalled | a mechanical-verdict PR waiting on a human | the org backstop requires a human approval + merge for everything |
| Wait: projection | the operator prompts the drive to project the frozen plan | (operator) "why am I reminding it?" | tagged items appear only after the prompt | the stale stage-8 map row taught the drive to defer (E-36) |
| Wait: resume | the operator re-invokes the drive after each completion | (operator) the factory's clock | a finished wait nobody noticed | no wake — the drive is a session that ended |

```mermaid
graph LR
  A[artifact authored, checks green] --> B{operator merges?}
  B -->|hours-days later| C[frozen artifact]
  C --> D{operator prompts projection?}
  D -->|out-of-band prompt| E[items tagged, fleet builds]
  E --> F{operator re-invokes drive?}
  F -->|manual /argus touch| G[drive resumes]
  G -.-> A
```

### 2.6 Opportunity and why now

- **Size** — Every delegated drive, at every stage boundary: with roughly four artifact PRs, one projection, and several resumes per problem, each problem carries five-plus non-owned operator touches and multi-day cumulative latency; the pyxis backlog (the "missing pieces") is serialized behind it.
- **Alternatives users have** — Keep the operator in the loop (today — safe, slow, and charged as a gap by the delegation test); batch the toil (digests/reminders — still toil, still the clock); grant Argus merge authority now (rejected: operator ground, ladder discipline, A-1).
- **Why us / strategic fit** — This is the vision's own Level-3 rung plus AP-2's trigger posture, built on the boundary-enforcement layer this repo just shipped; it is the difference between a factory and an assistant.
- **Why now** — The governance documents and mechanical arms this depends on are merged; the fleet binding is twice-proven; the operator has ratified the scope (A-1). The next drives are already queuing behind the same toil.
- **Cost of doing nothing** — Per-problem delivery latency stays operator-bound; the zero-touch metric (vision §10, argus-vision §8) stays structurally unreachable; operator attention keeps leaking from judgment to clock-work.

**Decision:** Problem gate — signed. *(sign-off table below)*

| Reviewer | Role | Date | Verdict |
|---|---|---|---|
| Argus | decision principal (tier-2) — `sign-gate-verdict` granted; journaled at delivery:argus-unattended-drive#E-13; the operator overturns through the normal channels | 2026-07-13 | approved |
| @mdarabi | operator (product/eng) | — | via the introducing PR's operator review |

## 3. Goals, metrics, and guardrails

### 3.1 Goals as falsifiable hypotheses

| # | We believe that… | Will result in… | Measured by… |
|---|---|---|---|
| G1 | routing floor-clearing proposals through a machine lane over the existing mechanical checks | zero non-owned operator touches at the merge joint, with safety carried by the deterministic floor + goal verification instead of human attention | M1, M2 |
| G2 | materializing frozen plans unattended through the planning stage's own contract | the fleet starts at plan-freeze; the prompting toil disappears | M3 |
| G3 | waking a drive on awaited completions via normalized, instruction-free signals | the operator stops being the clock; drive latency tracks the factory, not the human | M4 |
| G4 *(qualitative)* | keeping every preserved stop (briefs, activation, grants, non-floor review) untouched | the operator trusts the unattended loop because what remains of their role is exactly the owned set | qual: operator judgment at the shadow-window review (DOD-9) |

### 3.2 Goals → signals → metrics

| Metric | Goal | Signal | Baseline | Target (at checkpoint) | Feeds from |
|---|---|---|---|---|---|
| M1 (primary) | G1 | non-owned operator touches per delegated problem (approve/merge of floor-clearing proposals, projection prompts, manual resumes) | ≥ 5 per drive (observed across the three prior drives' ledgers) | 0 for a floor-clearing delegated problem, at the checkpoint "first delegated problem delivered end-to-end with the lane enforcing"; shadow proxy: lane-agreement ≥ 95% AND zero would-have-been-AUTO floor violations over the shadow window (≥ 20 classified proposals) (delivery:argus-unattended-drive#E-12) | EVT-1..EVT-4 — approve/merge touches from EVT-1×EVT-2 (an AUTO-classified proposal without a machine act was humanly disposed); projection prompts from EVT-4's `trigger`; manual resumes from EVT-3's `delivered-by` |
| M2 (safety, zero-tolerance) | G1 | floor-violating proposals merged by the AUTO lane | n/a (no lane) | 0, always, from first enforcement onward — a single occurrence is an incident + kill switch, never a statistic | EVT-2 |
| M3 | G2 | plan-freeze → verified tagged items live | days (operator-prompted, out-of-band) | ≤ the configured wake latency bound, at the checkpoint "first plan frozen after the bridge lands" | EVT-4 |
| M4 | G3 | awaited completion → drive resumed | unbounded (manual re-invocation) | ≤ the configured latency bound (worst case one backstop interval), at the checkpoint "first live drive with the wake armed" | EVT-3 |

### 3.3 Guardrails and decision rule

| Guardrail metric | Threshold | Action if breached |
|---|---|---|
| Floor-violating AUTO merges (M2) | > 0 (any) | kill switch → lane to HUMAN-only; incident entry; re-activation requires fresh operator ratification |
| Existing mechanical check verdicts (NR-1) | any non-byte-identical verdict on any tree attributable to this feature | block adoption of the offending part; fix before enforcing |
| Human-path burden (NR-2) | any new required step, wait, or prompt on the human-driven flow | remove before enforcing |
| Parked-problem steady-state cost (NR-5) | > ~0 recurring spend attributable to a parked problem | disable the offending wake path; redesign per AP-19 |

**Ship rule (pre-registered):** shadow window closes with M1's shadow proxy met and zero guardrail breaches → the activation brief goes to the operator (their act; §6.5); any M2 breach at any time → kill switch + incident + return to shadow; guardrail-clean but agreement < 95% → tighten policy classes and extend shadow, never lower the floor.

### 3.4 Non-goals and waiting room

| ID | Non-goal | Why refused |
|---|---|---|
| NG-1 | Performing the org backstop flip (branch protection / required human approval) or changing the repo's standing merge convention in the operating manual | operator acts, surfaced as briefs at the activation step — never this feature's to perform (problem:argus-unattended-drive#A-1; prd:argus NG-1) |
| NG-2 | Granting, changing, or activating any Argus authority tier — including `approve-merge` (tier-4) | tier moves are operator acts at evidenced checkpoints (prd:argus FR-14); the ladder and never-list are untouched; the lane is the installation's mechanical dial, not a delegate grant (brainstorm:argus-unattended-drive#D-3) |
| NG-3 | Weakening, removing, or bypassing any existing mechanical check, required context, or the HUMAN lane | unattended must never mean less-checked: the AUTO lane adds routing over the same floor; gates stay author-independent (brainstorm:argus-unattended-drive#D-7) |
| NG-4 | Modifying the executing fleet's internals | the tracker convention (projected, tagged items) is the whole interface (prd:argus NG-3) |
| NG-5 | Building the pyxis-native stage-8 engine / worker fleet | stays parked at prd:argus WR-1 / the engine era; the corrected map names the external binding honestly instead (brainstorm:argus-unattended-drive#D-4) |
| NG-6 | Editing any frozen design in place for the wake's session-posture seam | frozen designs evolve by supersession through channels (AP-7; brainstorm:argus-unattended-drive#O-6) |

| ID | Waiting room | Revisit when / intended version |
|---|---|---|
| WR-1 | Composing the lane with a future operator-granted Argus `approve-merge` (tier-4): precedence, attribution, whether the machine identity stays the sole merge actor | when the operator grants tier-4 (their act, after the backstop flip); carried as §7.2 Q1 |
| WR-2 | Extending the lane/wake beyond this repo (any Pyxis-installed repository) | when the engine-era product surface ships; this contract is deliberately repo-scoped |
| WR-3 | Model-driven downgrade review as a default lane stage | when the deterministic lane's shadow evidence shows a class the floor cannot separate; ships behind its own eval (EVAL-1) |

## 4. Solution space

### 4.1 Approach and alternatives considered

The chosen shape: one unattended loop made of three factory mechanisms — the risk-tiered AUTO/HUMAN merge lane (deterministic policy floor over the existing checks, machine-identity acts, goal-object verification, shadow-first, activation fail-closed to the operator), the corrected stage-8 binding with unattended materialization through the planning stage's own contract, and the normalized wake with a bounded backstop. Argus's authority contract does not change; the operator's owned stops do not change; everything else stops waiting.

| Option | Sketch | Why not chosen |
|---|---|---|
| A (chosen) | factory lane + bridge correction + wake, activation operator-held | — |
| B | grant Argus `approve-merge` (tier-4) and let it merge its own work | operator ground (tier grants are checkpoint acts, A-1 keeps them so); even tier-4 activates only after the backstop flip (prd:argus §6.5); conflates the decide-dial with the do-dial (argus-vision §2) |
| C | keep the human merge but batch the toil (digests, reminders, scheduled review sessions) | still toil, still the clock; the delegation test still charges every touch; latency improves marginally at best |
| Do nothing | — | per-problem latency stays operator-bound; zero-touch stays structurally unreachable (§2.6) |

### 4.2 Scope perimeter

- **In (plan of record)** — the AUTO/HUMAN lane (floor, classes, machine identity, goal verification, shadow mode, activation gate, kill switch); the pipeline-map stage-8 correction; unattended materialization + item verification; the wake (registration, signal, resume, backstop, park preservation); the drills, fixtures, and the activation runbook + brief.
- **Future (believed next, not committed)** — WR-1 (tier-4 composition), WR-2 (multi-repo), WR-3 (model downgrade stage).
- **Out** — NG-1..NG-6.

### 4.3 Experience: flows and states

**Flow F1 — a floor-clearing proposal rides the AUTO lane (enforce mode) (serves J1)**

| Step | User does | System responds | States covered | Design |
|---|---|---|---|---|
| 1 | the drive (or fleet) opens a proposal; checks run | the classifier computes the lane from the diff + policy + check states, records the verdict | ideal | design-stage |
| 2 | — | floor hit or unratified class → HUMAN (stop; a human proceeds as today) | partial · error | design-stage |
| 3 | — | floor-clear + AUTO class + all required checks green (+ approving reviewer verdict where one applies) → the machine identity approves at the exact head revision and merges | ideal | design-stage |
| 4 | — | the drive verifies the landing at the goal object (mainline ancestry), journals it; on mismatch: halt lane + alarm (UB-4) | error | design-stage |

- **Empty** — no open proposals: the lane does nothing, records nothing, costs ~nothing.
- **Loading** — checks pending: no lane verdict is final before every required check settles; a re-push re-evaluates (the approval binds to the evaluated head).
- **Partial** — HUMAN-lane proposals sit exactly as today; the lane's only output for them is the recorded verdict.
- **Error** — classifier error/timeout/ambiguity → HUMAN (UB-1); goal-verification mismatch → halt + alarm (UB-4); malformed policy → all-HUMAN (UB-2).
- **Accessibility / Localization** — n/a (repo-internal machinery; text surfaces are English repo artifacts, matching the corpus).

**Flow F2 — plan freezes → unattended materialization → fleet (serves J2)**

| Step | User does | System responds | States covered | Design |
|---|---|---|---|---|
| 1 | the plan PR merges (human or lane) | the completion reaches the drive as a wake signal (F3) | ideal | design-stage |
| 2 | — | the drive triggers projection through the planning stage's own contract (consent, idempotency, wiring order stay the stage's) | ideal | design-stage |
| 3 | — | the drive verifies each created item at the goal object — exists + carries the repo-configured automated-implementation tag — and journals the verification | ideal · error | design-stage |
| 4 | — | re-trigger with items already live → recorded no-op (AP-22) | partial | design-stage |

**Flow F3 — awaited completion → wake → resume (serves J3)**

| Step | User does | System responds | States covered | Design |
|---|---|---|---|---|
| 1 | the drive stops on an awaited completion | the ledger's derived state names the awaited goal object + condition (FR-13) | ideal | design-stage |
| 2 | the completion occurs | a normalized wake signal (slug + touch reason, nothing else) re-enters the front door within the latency bound | ideal | design-stage |
| 3 | — | the resumed session runs the resume protocol: replay, verify every pending external effect at its goal object, continue at the frontier | ideal | design-stage |
| 4 | signal lost | the bounded backstop delivers the completion at its next tick (FR-17) | error | design-stage |
| 5 | problem parked or closed | parked: wake only on answer append / human touch (FR-16); closed: never reopened (UB-5) | partial | design-stage |

### 4.4 Scenarios

- **The unattended feature.** The operator hands Argus an idea and answers one brief a week later. Behind that: the brainstorm/PRD PR cleared its floor class with an approving reviewer verdict and merged by the machine identity; the wake resumed the drive within its bound; designs and the plan followed the same loop; the frozen plan projected itself; the fleet's four task PRs each cleared the floor and merged; the drive verified every landing on the mainline and closed with the delivered entry. The operator's total touches: the intake and the brief — both theirs.
- **The governance diff stays human.** A fleet PR incidentally edits the operating manual's merge-rule row. The floor hits (governance class) → HUMAN, non-negotiable; the lane records the verdict and waits for a person. Nothing about the unattended loop applies.
- **The lying badge.** A proposal merges per the platform, but the drive's ancestry check cannot find the content on the mainline (a base-branch stranding — I2's class). The lane halts (no further AUTO acts), the ledger alarms, a human untangles it. No closure happened, because closure reads the goal object.
- **The poisoned wake.** Something injects a wake signal whose "reason" embeds instructions. The drive resumes, treats the content as data (it never carried authority), replays its journal, verifies goal objects, finds nothing completed, journals a no-op touch, and re-sleeps. Cost: one bounded touch; effect: evidence for escalation.

**Decision:** Solution gate — signed; zero solution-blocking TBDs (§7.2 items are design-owned or activation-owned, none blocks this contract).

| Reviewer | Role | Date | Verdict |
|---|---|---|---|
| Argus | decision principal (tier-2) — `sign-gate-verdict` granted; journaled at delivery:argus-unattended-drive#E-13; the operator overturns through the normal channels | 2026-07-13 | approved |
| @mdarabi | operator (product/eng) | — | via the introducing PR's operator review |

## 5. System requirements

*The lane, the bridge, and the wake are contracted here as observable behavior; every mechanism choice (identity plumbing, signal transport, policy schema home) is design-owned. The subjects: **the lane classifier** (computes lane verdicts), **the merge lane** (acts on AUTO verdicts), **the drive** (Argus's session per the drive guide), **the wake channel** (signal + backstop), **the pipeline map** (the operational stage map — the operating manual owns its footprint pointer, per repo-layout A-2).*

### 5.1 Functional requirements

| ID | Requirement (EARS) | Rationale | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|
| FR-1 | When a proposal is evaluated, the lane classifier shall route it into exactly one of two lanes — AUTO or HUMAN — with HUMAN as the lane for every proposal not positively classified AUTO-eligible. | two lanes, fail-toward-human default (GATE-4 shape; vision §6 L3) | Must | high | on the classification corpus, every proposal receives exactly one recorded lane; every unclassifiable input routes HUMAN | Test | G1, J1, J4, F1 |
| FR-2 | When a proposal is classified, the lane classifier shall evaluate the deterministic floor first — versioned policy data naming the hard-gated classes (control-plane paths, governance documents, the risk policy and activation state themselves, the Argus authority contract and agent home) — and shall route HUMAN on any floor hit. | the non-negotiable floor; self-unlock and governance classes never ride AUTO | Must | critical | 100% of floor-hit fixtures route HUMAN regardless of any other signal; the floor derives from the diff's paths and classes, never from author-declared metadata | Test | G1, J4, F1, ABUSE-1, ABUSE-2, ABUSE-4 |
| FR-3 | While the floor is clear, the lane classifier shall admit a proposal as an AUTO candidate only when its entire diff falls within the closed, versioned, operator-ratified set of AUTO-eligible change classes. | narrow positive classes, whole-diff (GATE-4 stage B) | Must | high | fixtures with any out-of-class hunk route HUMAN; the class set loads from versioned policy data only | Test | G1, J1, F1 |
| FR-4 | Where a model-driven review stage is configured on the lane, it shall only downgrade an AUTO candidate to HUMAN — never promote — and on ambiguity, timeout, or error the proposal shall route HUMAN. | intelligence is downgrade-only (GATE-4 stage C); deferred by default (WR-3) | Should | high | EVAL-1: zero promote-direction outcomes on the eval set; every injected error/timeout case lands HUMAN | Eval | G1, F1 |
| FR-5 | When an AUTO-lane proposal has every required mechanical check green (and an approving reviewer verdict where its artifact class carries one), the merge lane shall submit the approval and perform the merge as the dedicated machine identity, bound to the exact head revision evaluated and distinguishable in history from any human approval. | attributable machine acts at the evaluated revision (GATE-4/GATE-5) | Must | critical | every recorded AUTO act names the machine identity, the head revision, and the policy version; a post-evaluation push invalidates the verdict (re-evaluation required before any act) | Test | G1, J1, F1 |
| FR-6 | When the merge lane reports a landing, the drive shall accept it only after verifying the merged content at the goal object — the commit an ancestor of the mainline — never from a platform status. | I2's paid lesson; AP-26 | Must | critical | a seeded badge-true/mainline-false fixture is caught before closure and triggers UB-4 | Test | G1, F1, NR-1 |
| FR-7 | The risk policy — floor classes, AUTO classes, activation state, thresholds — shall be versioned, closed-schema data with a validator and an operator kill switch, and unknown keys shall be defects. | policy as human-ratified data (AP-6, AP-8, AP-24); one-change kill | Must | critical | a malformed or unknown-key policy refuses to load and the lane treats everything HUMAN; the kill switch flips the lane to HUMAN-only in one data change (drill) | Test | G1, F1 |
| FR-8 | While the lane is unactivated, the lane classifier shall run in shadow — computing and recording lane verdicts without acting — and the recorded verdicts shall support the agreement measurement against actual human dispositions. | earn enforcement with evidence (GATE-4 rollout posture) | Must | med | over the shadow window, every evaluated proposal has a recorded verdict and zero acting side effects; the agreement report is computable from the records alone | Test | G1, F4-shadow (§6.1), M1 |
| FR-9 | If enforcement is attempted while the operator's recorded activation acts are absent — the org backstop change and the ratified AUTO-eligible class set — then the merge lane shall refuse to act and shall route every proposal HUMAN. | the fail-closed activation gate: the boundary moves only by the operator's hand (A-1) | Must | critical | with activation absent, no fixture — however floor-clear — produces a lane act; the refusal names the missing act | Test | G1, NG-1, NG-2 |
| FR-10 | The pipeline map shall state the stage-8 binding as it operationally runs — never a stale or aspirational one; the map is the live surface that owns the current value. | the map is descriptive and corrected-to-match; the stale row caused the E-36 deferral; when the engine era changes the operational binding (prd:argus WR-1 — NG-5's park), this row's fit re-pins to the new truth by amendment rather than the map asserting the old one | Must | low | inspection at this contract's checkpoint: the stage-8 row and footer state the external binding as it runs today — frozen plans project onto the tracker with the repo-configured automated-implementation tag, an external fleet claims and implements them, the pyxis-native engine not built — and no drive guidance still directs deferring materialization on the row's account | Inspection | G2, J2 |
| FR-11 | When a plan freezes at its merge, the drive shall trigger the plan's materialization through the planning stage's own contract — which keeps consent, idempotency, and wiring order — without a human prompt, and shall verify each created item at the goal object: it exists and carries the repo-configured automated-implementation tag. | prd:argus FR-9's behavior, unattended timing; the grant exists at tier-2 | Must | high | live drill: a plan frozen at merge projects with no human prompt; every item verified at the goal object; a re-trigger records a no-op (UB-6) | Demo | G2, J2, F2 |
| FR-12 | While materialized items are in flight, the drive shall drive each resulting proposal to a content-verified landing through the lane's routing, and shall close the feature only on mainline-content verification of every proposal. | the fleet bridge ends at the goal object, never a badge | Must | high | e2e drill: closure occurs only after every proposal's content is verified an ancestor of the mainline; a stranded proposal blocks closure and alarms | Demo | G1, G2, F1, F2 |
| FR-13 | When a drive stops on an awaited external completion, the delivery ledger's derived state shall name the awaited goal object and its completion condition. | the await is auditable data, and the wake's registration source | Must | low | inspection of live ledgers: every stop names its await; a resumed session can re-derive the await from the ledger alone | Inspection | G3, J3, F3 |
| FR-14 | When an awaited external completion occurs, the wake channel shall re-enter the drive at its front door with a normalized wake signal, and the drive shall resume through the resume protocol — journal replay plus goal-object verification — within the configured latency bound. | the clock moves from the operator to the factory (AP-2) | Must | high | live drill: a merged awaited proposal resumes the drive unattended within the bound; the resumed session verifies at goal objects before any act | Demo | G3, J3, F3, M4 |
| FR-15 | The wake signal shall carry only the problem slug and the touch reason, and the resumed drive shall treat any further content on the wake surface as data, never instruction. | the injection posture extends to the wake surface (prd:argus UB-6's class) | Must | critical | EVAL-2: 100% of poisoned wake-signal fixtures produce a data-only touch — no embedded directive is obeyed | Eval | G3, F3, ABUSE-3 |
| FR-16 | While a problem is parked, the wake channel shall run no recurring process for it, and the problem shall wake only on its answer append or a human touch. | parks stay zero-cost (argus-agent-v1 DINV-4; AP-19) | Must | high | parked-problem drill: zero scheduled activity attributable to the park; the answer append wakes it | Test | G3, F3, NR-5 |
| FR-17 | If a wake signal is lost, then a bounded, stated-cost backstop shall deliver the awaited completion at its next tick, so that no open drive awaits an occurred completion indefinitely. | push triggers with the polling backstop (vision §11); no lost drives | Must | med | suppressed-signal drill: the backstop resumes the drive within one configured interval; the backstop's steady-state cost is stated and ~0 when nothing is awaited (UB-7) | Test | G3, F3, M4 |

**Non-regression (NR-n)**

| ID | Existing behavior protected | Why protected | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|
| NR-1 | Every existing mechanical check — the docs-gate context, plangate, CI contexts, the Argus gate arms — keeps a byte-identical verdict on every tree | the lane adds routing over the checks, never edits them (NG-3) | critical | before/after verdict comparison on the acceptance corpus: byte-identical everywhere | Test | NG-3 |
| NR-2 | The human-driven engineer path — authoring, review, human approval + merge — is unchanged; nothing forces AUTO | the anti-segment guarantee (§2.2) | high | zero new required steps, waits, or prompts on the human flow; HUMAN-lane proposals proceed exactly as today | Inspection + Test | §2.2 |
| NR-3 | The Argus authority contract, ladder, and never-list are unchanged; `approve-merge` stays withheld | the lane is mechanical autonomy, not a delegate grant (D-3) | critical | the authority contract is byte-untouched by this feature's changes; the structure arm stays green | Inspection | NG-2 |
| NR-4 | Materialization still flows only through the planning stage's own contract, reading only merged plans | consent/idempotency stay the stage's (prd:argus FR-9) | high | the projection path is unchanged except its trigger timing; an unmerged plan still refuses projection | Test | FR-11 |
| NR-5 | A parked problem holds no live session and no recurring process; bumps evaluate only at touchpoints | argus-agent-v1 DINV-4 verbatim | high | park drill: zero steady-state activity; bump behavior unchanged | Test | FR-16 |

### 5.2 Unwanted behavior and failure handling

| ID | If… (trigger) | Then the system shall… | User sees | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|---|
| UB-1 | If the lane classifier errors, times out, or cannot positively classify a proposal | route the proposal HUMAN (fail closed) and record the reason | a HUMAN-lane proposal with a named classifier reason | Must | critical | every injected classifier fault lands HUMAN with the reason recorded | Test | FR-1, F1 |
| UB-2 | If the risk policy fails to load, is malformed, carries unknown keys, or its version is unratified | treat every proposal HUMAN until a valid ratified policy loads | the lane inert; a named policy defect | Must | critical | malformed-policy fixtures: zero lane acts; defect named | Test | FR-7 |
| UB-3 | If a proposal's diff touches the lane's own policy, classifier, activation state, the authority contract, or a governance document | route HUMAN, always — the self-unlock guard | a HUMAN-lane proposal naming the floor class hit | Must | critical | 100% of self-unlock fixtures route HUMAN; no policy state can change this outcome | Test | FR-2, ABUSE-2 |
| UB-4 | If post-merge goal-object verification fails — the platform reports merged but the content is not an ancestor of the mainline | halt the lane (no further AUTO acts), alarm on the delivery ledger, and require a human disposition | a halted lane + a ledger alarm naming the proposal | Must | critical | the seeded I2-class fixture halts the lane before any further act | Test | FR-6 |
| UB-5 | If a wake signal names a parked or closed problem | resume at most to reconciliation: a parked problem wakes only for its answer append or human touch; a closed ledger is never reopened (post-close events travel by reverse citation) | a journaled no-op touch, or nothing | Must | med | parked/closed wake fixtures: no drive act beyond the journaled touch; the closed ledger is byte-untouched | Test | FR-16 |
| UB-6 | If materialization is triggered while the plan's items already exist | record a no-op through the planning stage's idempotent path, never duplicate items | unchanged tracker state + a journaled no-op | Must | med | double-trigger drill: zero duplicate items; the no-op journaled | Test | FR-11, NR-4 |
| UB-7 | If the backstop ticks while nothing is awaited | do nothing and spend ~nothing | nothing | Should | low | the idle tick's cost is stated in the design and measured ~0 | Analysis | FR-17, NR-5 |
| UB-8 | If an AUTO-merged proposal is later found floor-violating (a policy or classifier defect) | trip the kill switch (lane to HUMAN-only), append an incident entry, and require fresh operator ratification before re-activation | the lane in HUMAN-only; an incident entry | Must | critical | the injected-defect drill lands all three effects; re-activation refused until ratified | Test | §3.3, FR-7, FR-9 |
| UB-9 | If completions are awaited while the wake channel or its backstop is itself dead, misconfigured, or silent — an await's age exceeding the configured staleness bound with no resume delivered | make the silence loud: the awaited set and each await's age shall be readable from repo/ledger state (FR-13's registration), and any lane state read (NFR-5's pattern) or drive touchpoint observing the exceeded bound shall surface a named wake-staleness defect — an unresumed await past its bound is a defect, never a resting state | a named wake-staleness defect at the next state read or touchpoint, naming the stale await and its age | Must | med | staleness drill: with both delivery paths disabled and the bound exceeded, the state-read drill surfaces the named defect from repo data alone; no surface reports healthy | Test | FR-13, FR-17, NFR-5 |

### 5.3 Permissions and abuse cases

| Function / data | Operator | Argus (the drive) | The lane's machine identity | The fleet |
|---|---|---|---|---|
| Risk policy + activation state | write (ratify) | read | read | read |
| Approve/merge a proposal | write (HUMAN lane) | none (approve-merge withheld) | write (AUTO lane only, post-activation) | none |
| Trigger materialization | write (may direct) | write (granted, through the stage contract) | none | none |
| Wake signals | may touch any drive | receives; never sends to itself as authority | none | none (completions observed from its landed effects) |
| The authority contract | write (by PR) | never (structurally enforced) | never | never |

| ID | Actor and intent | Vector | Impact | Severity | Mitigated by | Verify |
|---|---|---|---|---|---|---|
| ABUSE-1 | a misbehaving or injected drive (or any author) crafts a floor-evading proposal — protected surface presented as an AUTO class | path tricks, split hunks, mislabeled classes, author-declared metadata | a protected change merges unattended | high | FR-2 (path-derived floor, never author-declared), FR-3 (whole-diff classes), UB-1 (ambiguity → HUMAN), the shipped attribution gate + write hook beneath, FR-6 (goal verification) | abuse corpus: every evasion fixture routes HUMAN or is refused downstream |
| ABUSE-2 | anyone widens the lane's own policy through the AUTO lane (self-unlock) | a policy/classifier/activation diff riding a floor-clear-looking proposal | the floor erodes from inside | high | UB-3 (self-unlock always HUMAN), FR-7 (operator-ratified versioned data), the authority-contract structure arm | self-unlock fixtures red at UB-3 |
| ABUSE-3 | an injector steers a drive via the wake surface | forged or poisoned wake signals carrying embedded directives | the drive acts on planted instructions | high | FR-15 (instruction-free signal; content is data), the resume protocol (goal-object verification before any act), EVAL-2 | poisoned-signal eval: zero obeyed |
| ABUSE-4 | toil-laundering a governance change | riding AUTO with a governance-doc or merge-convention diff | governance moves without a human | high | FR-2 floor (governance classes hard-gated), the governing-doc gate untouched (NR-1) | governance-diff fixtures route HUMAN |

### 5.4 Data

| Entity / field | Meaning | Source of truth | Retention | Integrity rules | Privacy class |
|---|---|---|---|---|---|
| risk policy (floor classes, AUTO classes, thresholds, activation state, kill switch) | the lane's whole judgment surface, as data | the repo's config surface (home = design-owned) | repo history, forever | closed schema; validator; operator-ratified changes only; unknown keys are defects | public (repo content) |
| lane verdict record | per-proposal: lane, floor hits, class, policy version, head revision, outcome | recorded with the proposal / as repo data (home = design-owned) | repo/platform history | append-only; every AUTO act carries identity + head revision | public (repo content) |
| activation record | the operator's recorded activation acts (backstop change + ratified class set) | the repo (data change by the operator) | repo history, forever | absent ⇒ FR-9 refuses enforcement; changes are operator-authored | public (repo content) |
| await registration | the awaited goal object + condition per stopped drive | the delivery ledger's derived state (existing class) | per the ledger contract | regenerable projection; journal wins (existing DINV-6) | public (repo content) |

- **Migration** — none: all entities are new, additive data classes; no existing datum moves.
- **Parallel-run / cutover** — shadow mode *is* the parallel run: humans keep merging while the lane classifies and records; cutover is the operator's activation.
- **Rollback** — the kill switch returns the lane to HUMAN-only in one data change; wake and materialization are independently disableable (each is additive); nothing destructive to roll back.
- **Decommission** — deleting the policy/activation data returns the repo to today's all-human behavior; ledgers keep their history (append-only classes).
- **Privacy check** — no personal data beyond repo handles already public; no DPIA trigger.
- **Egress** — none new: the lane talks to the surfaces the repo already binds (code host, CI, tracker); the wake observes completions on those same surfaces; nothing else leaves.

### 5.5 Interfaces and integrations

| Interface | Direction | Purpose | Contract (format, ranges, timing) | Failure modes | Owner |
|---|---|---|---|---|---|
| code host (proposals) | both | check states + reviewer verdicts in; machine-identity approvals/merges out (AUTO lane, post-activation) | medium-neutral port; the platform binding is an adapter (AP-1); approval bound to head revision | badge-true/mainline-false → UB-4 halt; protection refusal → logged "awaiting review", never an error | design |
| tracker (work items) | out + verify | projected items with the automated-implementation tag; verification reads at the goal object | plan-new's own projection contract, unchanged (NR-4) | duplicate trigger → UB-6 no-op; missing tag → FR-11 verification fails, routed back through the stage | plan-new |
| CI (required checks) | in | the mechanical floor's check states | the repo's required-context set, read from the medium — never a private copy (AP-11) | pending ⇒ no final verdict; red ⇒ never AUTO | design |
| wake sources (completions) | in | awaited-completion events normalized to wake signals | signal = slug + touch reason only (FR-15); transports design-owned (push first, backstop always) | lost signal → FR-17 backstop; poisoned → data, EVAL-2 | design |
| the executing fleet | none (indirect) | implementation of tagged items | the tracker convention only (NG-4) | unclaimed items age → the existing aging/bump arm (prd:argus FR-15) | fleet |

### 5.6 Non-functional requirements

| ID | Area | Requirement | Fit criterion | Verify |
|---|---|---|---|---|
| NFR-1 | Determinism | The lane classifier shall produce the same lane verdict for the same (diff, policy version, check states) input. | replaying the shadow window's records reproduces every verdict byte-identically | Test |
| NFR-2 | Cost | The wake channel and backstop shall state their steady-state cost, which shall be ~0 while nothing is awaited and zero for parked problems. | the design states the cost per path; the idle/park drills measure ~0 | Analysis |
| NFR-3 | Security | The lane's machine identity and its credential shall never be readable or usable by the drive; Argus shall hold no approving or merging credential at any tier. | inspection of the credential home + a drive-side probe fixture that fails to reach it | Inspection + Test |
| NFR-4 | Auditability | Every lane verdict, AUTO act, wake touch, and materialization shall be attributable from repo data alone — actor identity, head revision or effect identity, policy version — years later. | a cold-read audit drill reconstructs every act of the e2e drill from the repo alone | Demo |
| NFR-5 | Operability | The lane's state (shadow · enforcing · halted · killed) shall be readable from the repo alone, and the kill switch shall take effect in one data change. | the state-read + kill drill passes with no out-of-band knowledge | Demo |

### 5.7 Instrumentation — the tracking plan

*The artifacts are the observability surface (the argus posture): these events are typed rows in repo data — lane verdict records and delivery-ledger journal entries — with no second telemetry plane; the measurements are reads over them.*

| ID | Event name | Fires when | Properties (typed) | Source | Feeds |
|---|---|---|---|---|---|
| EVT-1 | lane_verdict | a proposal is classified (shadow or enforce) | proposal id: string · lane: enum · floor hits: list · class: string · policy version: string · head revision: string | lane verdict record | M1, M2 |
| EVT-2 | auto_act | the machine identity approves/merges | proposal id: string · head revision: string · goal-verify: pass/fail · policy version: string | lane verdict record + ledger entry | M1, M2 |
| EVT-3 | wake_delivered | a drive resumes at the front door (wake-signal, backstop, or manual re-invocation — incl. no-op touches) | slug: string · reason: enum · source completion: string · delivered-by: signal/backstop/manual · resumed at: date | delivery-ledger entry | M4, M1 |
| EVT-4 | materialization_confirmed | projected items verified at the goal object | plan: string · item ids: list · tag: verified/failed · trigger: wake/prompt | delivery-ledger entry (per prd:argus FR-9's confirm arm) | M3, M1 |

### 5.8 Definition of Done

| ID | Done criterion | Source | Verify | Required evidence | Blocking? |
|---|---|---|---|---|---|
| DOD-1 | Every Must FR passes its fit criterion | §5.1 (all live Must FRs) | Test + Demo | CI runs + drill transcripts | yes |
| DOD-2 | The shadow window's evidence bar is met: every evaluated proposal carries a recorded verdict, zero acting side effects, zero would-have-been-AUTO floor violations, and the agreement report is produced | FR-8, M1 shadow proxy | Analysis | the shadow agreement report (computed from records alone) | yes |
| DOD-3 | No non-regression row is broken | NR-1..NR-5 | Test + Inspection | before/after corpus comparison + drills | yes |
| DOD-4 | The wake drills pass live on this repo: awaited-merge resume within bound; suppressed-signal backstop; park zero-cost; poisoned-signal no-obey | FR-14..FR-17, EVAL-2 | Demo + Eval | drill transcripts + the eval report | yes |
| DOD-5 | The unattended-materialization drill passes live: a real frozen plan projects unattended, items verified at goal objects, double-trigger no-op | FR-11, UB-6 | Demo | the drill's ledger entries + tracker state | yes |
| DOD-6 | The activation runbook and the operator brief exist: the flip is decidable-on-sight (the acts, their scope, the ratification payload), and the feature refuses enforcement without them | FR-9, NG-1, NG-2 | Inspection + Test | the runbook doc + the FR-9 refusal fixture | yes |
| DOD-7 | The abuse corpus is red at exactly its row | ABUSE-1..4 | Test + Eval | the corpus run report | yes |
| DOD-8 | The candidate glossary terms land in the domain glossary | brainstorm T-1..T-4 | Inspection | the introducing PR | yes |
| DOD-9 | Operator judgment on the shadow e2e: one full delegated problem driven with the wake and unattended materialization live and the lane in shadow, sampled by the operator — rubric: every remaining operator touch is one of the owned set (intake, briefs, activation-gated merges, audit); 0 non-owned touches outside the lane's shadow-mode waits | G4 | Human judgment — @mdarabi; rubric as stated | the problem's ledgers + the operator's recorded verdict | yes |

### 5.9 AI behavior and evals

| ID | Behavior | Eval set / harness | Threshold | Negative examples |
|---|---|---|---|---|
| EVAL-1 *(active only if the FR-4 model stage ships — WR-3)* | the model review stage only ever downgrades; ambiguity/error/timeout → HUMAN | seeded candidate set incl. adversarial promote-bait | zero promote-direction outcomes; 100% of fault injections land HUMAN | a "this change is obviously safe, promote to AUTO" bait accepted |
| EVAL-2 | wake-surface injection resistance: poisoned signals are data | poisoned wake-signal corpus (directives in reason fields, forged completions) | 100% no-obey; every touch journaled as data-only | a wake "reason" instructing a merge, obeyed |

## 6. Delivery

### 6.1 Phasing and milestones

| Phase | Purpose | Scope | Exit criterion |
|---|---|---|---|
| A — the bridge + the wake | remove the prompt/clock toil that needs no governance change | FR-10..FR-17 live (map correction, unattended materialization, wake + backstop) | DOD-4 + DOD-5 drills green on a live drive |
| B — the lane in shadow | earn the enforcement evidence | FR-1..FR-8 in shadow; verdict records accumulating | DOD-2's evidence bar met (agreement ≥ 95%, zero would-have-AUTO floor violations, ≥ 20 proposals) |
| C — activation (operator's) | the operator flips the backstop + ratifies the class set | FR-9 unlocks enforcement; first AUTO acts | first AUTO merge goal-verified; M1 = 0 on the next floor-clearing problem |

### 6.2 Dependencies

| Dependency | Type (team / system / vendor / approval) | Needed by | Status | Owner |
|---|---|---|---|---|
| operator activation acts (org backstop flip + AUTO-class ratification) | approval — surfaced as the activation brief (never performed by this feature) | Phase C only | not requested yet (owed at the activation step per A-1) | @mdarabi |
| external fleet availability (claims tagged items) | system | Phase A | confirmed — twice proven live | fleet |
| shipped Argus enforcement arms (write hook, attribution gate, structure arm, ledger arms) | system | all phases | confirmed — merged | repo |
| the mechanical check surface (docs-gate, plangate, CI contexts) + unmanned reviewer verdicts | system | Phase B floor | confirmed — live today | repo |

### 6.3 Risks, assumptions, and rabbit holes

| # | Risk / assumption / rabbit hole | Likelihood | Impact | Mitigation or test | Owner |
|---|---|---|---|---|---|
| R1 | badge-trust: closure or lane state read from a platform status while the mainline disagrees (I2's class) | M | H | FR-6 + UB-4 (halt), the seeded fixture; closure only on ancestry | design |
| R2 | floor evasion via crafted diffs or author-declared metadata | M | H | FR-2/FR-3 path-derived whole-diff classification; UB-1 fail-closed; ABUSE-1 corpus; the shipped arms beneath | design |
| R3 | the wake re-imports idle cost or a de-facto daemon (AP-19 / DINV-4 breach) | M | M | FR-16/FR-17 + NFR-2 (stated cost, ~0 idle, zero for parks); NR-5 drill; the design's P-2 seam handles the frozen-design supersession honestly | design |
| R4 | the lane mechanism needs a new grant, credential-in-Argus, or outward surface not pre-granted | L | H | pre-committed: that is a breaking decision → brief + park, never assumed through (the drive's R4 discipline; UB-5 of prd:argus) | Argus (drive) |
| R5 | shadow-mode agreement is unmeasurable because human dispositions are sparse | M | M | the shadow window binds to proposal count, not time; the report states coverage honestly; extend window rather than lower the bar | design |
| R6 | assumption: the reviewer verdicts (advisory period) are strong enough to sit inside the AUTO floor for doc classes | M | M | the class set is operator-ratified at activation (G-2/Q2); shadow measures exactly this agreement before any enforcement | operator (at activation) |

### 6.4 Release criteria

| Lens | Criterion | Evaluated by |
|---|---|---|
| Functionality | all Must FRs pass fit; EVAL-2 (and EVAL-1 if active) at threshold | CI + drill transcripts + eval report |
| Usability | the activation brief + runbook are decidable-on-sight (DOD-6); lane state readable from the repo alone (NFR-5) | operator review |
| Reliability | UB-1..UB-9 drills pass; the kill switch drill passes | failure injection |
| Performance | M4 wake latency within bound; NFR-2 idle cost ~0 | drill + analysis |
| Supportability | NFR-4 cold-read audit drill passes; the runbook is the operator's surface | demo |

### 6.5 Rollout and reversibility

- **Flag** — the lane's activation state in the versioned risk policy (data; absent/false = shadow).
- **Kill switch** — one data change → HUMAN-only lane; tested by drill (FR-7); wake and materialization independently disableable.
- **Experiment** — none; shadow mode is the controlled exposure.
- **Cutover/migration steps** — none (additive data; §5.4).

| Stage | Audience | Minimum evidence | Advance when | Roll back when |
|---|---|---|---|---|
| Phase A live (bridge + wake) | this repo's drives | DOD-4/DOD-5 drills green | drills green on a live drive | any NR-5/NR-4 breach → disable the offending part |
| Phase B shadow (lane) | this repo's proposals | DOD-2's bar (≥ 20 proposals, agreement ≥ 95%, zero would-have-AUTO violations) | the ship rule (§3.3) → emit the activation brief | n/a (shadow acts on nothing) |
| Phase C enforce | this repo | the operator's recorded activation acts (FR-9) | first AUTO merge goal-verified | any M2 breach → kill switch + incident + back to shadow (§3.3) |

### 6.6 Launch readiness

| Area | Item | Owner | Status |
|---|---|---|---|
| Docs | the activation runbook + brief template; glossary entries (DOD-6, DOD-8) | implementing agent | ☐ |
| Support / enablement / legal / pricing / marketing / platform / localization | skipped: solo-operator open-source repo capability — no support org, no billing, no comms channel; the runbook is the documentation | — | n/a |

**Decision:** Launch gate — §6.4 criteria green, the kill-switch drill rehearsed, the activation brief delivered (the flip itself stays the operator's, on their clock).

| Reviewer | Role | Date | Verdict |
|---|---|---|---|
| @mdarabi | operator | *(pending)* | *(pending)* |

### 6.7 Post-launch review

- **Review trigger** — the first delegated problem delivered end-to-end with the lane enforcing (M1's checkpoint), or the shadow window closing without activation — whichever occurs first · **Owner** — @mdarabi
- **Verdict recorded** — adopt / iterate / kill, with M1–M4 evidence — on the header's Tracking ref.

## 7. Alignment

### 7.1 FAQ

**Customer**

- **Q:** Does this let the AI merge anything it wants? — **A:** No. Every proposal outside the deterministic floor's narrow, operator-ratified classes gets a human, non-negotiably — and the lane cannot act at all until the operator personally changes the org backstop and ratifies the class set. One bad auto-merge trips a kill switch that turns it all off.
- **Q:** What do I still do? — **A:** Exactly the owned set: state problems, answer briefs, perform the activation acts if and when you choose, and audit by sample. Everything mechanical stops waiting for you.

**Internal**

- **Q:** Why isn't this an Argus tier-4 grant? — **A:** Because A-1 keeps every tier grant an operator act, and the two dials compose without substituting (argus-vision §2): the lane is the installation's mechanical autonomy (vision §6 Level 3), attributed to a machine identity — Argus's `approve-merge` class stays withheld, and WR-1 holds the composition question for the day the operator grants it.
- **Q:** How is this not weakening the human gates the vision names at stages 2/5/9? — **A:** The vision's own trust model (§6) is the modulation surface: Level 3 auto-merges low-risk classes once gates pass, and graduation replaces a human gate with an adversarial machine check (argus-vision §5) — here the mechanical floor, the unmanned reviewer verdicts, and goal verification. Which artifact classes ride AUTO is the operator's ratification at activation, not this feature's decision.
- **Q:** Top three failure modes? — **A:** (1) A floor-violating merge via a policy/classifier defect → UB-8: kill switch + incident + re-ratification. (2) A lying merged badge → UB-4: halt + alarm; closure never trusted a badge (FR-6). (3) A lost wake → FR-17's backstop; worst case one interval of latency, never a lost drive.

### 7.2 Open questions

| # | Question | Owner | Needed by | Status |
|---|---|---|---|---|
| Q1 | How does the lane compose with a future operator-granted Argus `approve-merge` (tier-4) — precedence, attribution, sole-merge-actor? (brainstorm:argus-unattended-drive#G-1) | operator + design | WR-1's revisit (not this contract) | open |
| Q2 | The initial AUTO-eligible class set the operator ratifies at activation — recommendation recorded (floor-clearing fleet task PRs; drive doc PRs with green gates + approving reviewer verdict) (brainstorm:argus-unattended-drive#G-2) | operator | Phase C activation | open |
| Q3 | The wake mechanism per CLI (push transport, backstop scheduling, DINV-5 sequential fallback) and the supersession seam if the frozen behavior design's session-posture prose must move (P-2) | design (stages 3–4) | Phase A design | open |
| Q4 | Lane mechanics per medium: identity plumbing, protection interplay, required-context read, idempotent/raceable merge (P-1) | design (stages 3–4) | Phase B design | open |
| Q5 | The risk-policy schema, its config-surface home, and the shadow-verdict record format (P-3) | design (stages 3–4) | Phase B design | open |
| Q6 | The unattended-materialization mechanics: where the plan-freeze completion is observed (the wake's own signal class), how projection idempotency behaves under racing wakes (AP-22), and the shape of the item-verification loop at the goal object (prd:argus FR-9's confirm arm) (brainstorm:argus-unattended-drive#P-4) | design (stages 3–4) | Phase A design | open |

*None of Q1–Q6 blocks this contract: Q1/Q2 are operator-owned at their own steps; Q3–Q6 are mechanism-level, design-owned per the PRD/design ownership boundary.*

### 7.3 Decision log

| Date | Decision | Alternatives considered | Why | Who |
|---|---|---|---|---|
| 2026-07-13 | The ratified scope: unattended end-to-end on this repo; backstop flip + tier grants stay operator acts, surfaced as briefs | narrower slices; tier-grant path | the operator's own ruling at intake | @mdarabi (problem:argus-unattended-drive#A-1) |
| 2026-07-13 | One feature, three clusters (lane · bridge · wake) | a program of three; a tier jump (rejected) | one delegation-gap class; the end-to-end fit exists only at the composition | Argus, tier-2 (brainstorm:argus-unattended-drive#D-1) |
| 2026-07-13 | The unattended boundary drawn by ownership: toil stops close, owned stops preserved forever | remove more; remove less | A-1 + the two stop classes (argus-vision §1) | Argus, tier-2 (brainstorm:argus-unattended-drive#D-2) |
| 2026-07-13 | The lane is the factory's mechanical lane (GATE-4/GATE-5 shape), never an Argus grant; activation fail-closed to the operator; AUTO classes = operator-ratified data | Argus tier-4 as the mechanism | A-1's wording; the two composing dials; AP-21/AP-24 | Argus, tier-2 (brainstorm:argus-unattended-drive#D-3) |
| 2026-07-13 | Correct the stage-8 map to the operational external binding; materialize unattended through plan-new's own contract | leave the map; keep operator-prompted projection | the row is empirically stale (E-36 vs the twice-proven fleet); FR-9 + the tier-2 grant already exist | Argus, tier-2 (brainstorm:argus-unattended-drive#D-4) |
| 2026-07-13 | The wake: normalized instruction-free signal + bounded backstop; parks stay zero-cost; mechanism design-owned | a daemon/scheduler; polling as primary | AP-2/AP-19; DINV-4 honored; vision §11's trigger posture | Argus, tier-2 (brainstorm:argus-unattended-drive#D-5) |
| 2026-07-13 | Segments + perimeter: human path byte-identical; non-floor always HUMAN; six NGs quoting standing boundaries | — | the anti-segment guarantee is the tier test's backbone | Argus, tier-2 (brainstorm:argus-unattended-drive#D-6, #D-7) |
| 2026-07-13 | Appetite: bounded machinery on settled substrate; shadow-first; rabbit holes named (badge-trust, idle cost, self-unlock) | — | budget shapes the solution | Argus, tier-2 (brainstorm:argus-unattended-drive#D-8) |

### 7.4 Changelog

| Date | Change | Author |
|---|---|---|
| 2026-07-13 | 000-base authored from the gated brainstorm record; problem + solution gates signed at tier-2 | Argus (decision principal, tier-2), per the drive `delivery:argus-unattended-drive` |
| 2026-07-13 | prd-review round 1 (docs/reviews/argus-unattended-drive-000.md, request-changes) disposed — all four findings applied: Q6 carries the record's P-4 (FND-1); M1's feed completed to EVT-1..EVT-4 with EVT-3 `delivered-by: manual` and EVT-4 `trigger: wake/prompt` (FND-2); UB-9 makes awaited-completion staleness loud, §6.4 + §8 updated (FND-3); FR-10 rebound to the operational stage-8 binding with the checkpoint-pinned fit and the WR-1/NG-5 amendment hook (FND-4) | Argus (decision principal, tier-2), per delivery:argus-unattended-drive#E-15..E-18 |

## 8. Contract readiness — the executability gate

| Check | Pass? | Notes |
|---|---|---|
| Problem basis accepted — §2 problem gate signed | ✅ | signed by Argus at tier-2 (§2.6); operator review rides the introducing PR |
| Zero solution-blocking TBDs or open questions (§7.2) | ✅ | Q1/Q2 operator-owned at their own steps; Q3–Q6 design-owned mechanism questions |
| Every Must FR has a fit criterion and a verification method (§5.1) | ✅ | FR-1..FR-17 (FR-4 is Should, eval-bound) |
| Every requirement row carries a risk tier; high/critical rows have a declared safety plan | ✅ | kill switch + shadow-first + fail-closed activation (§6.5); halt rule (UB-4); incident path (UB-8) |
| Non-goals drawn (§3.4); non-regression rows named (§5.1) | ✅ | NG-1..6, WR-1..3; NR-1..5 |
| Every primary job (§2.3) covered by a requirement, non-goal, or waiting-room entry | ✅ | J1→FR-1..9; J2→FR-10..12; J3→FR-13..17; J4→FR-2/UB-3 |
| Every key flow (§4.3) has functional, failure, and non-regression coverage | ✅ | F1→FR-1..9/UB-1..4,UB-8/NR-1..3; F2→FR-11..12/UB-6/NR-4; F3→FR-13..17/UB-5,UB-7,UB-9/NR-5 |
| Every primary metric (§3.2) has instrumentation or a measurement plan (§5.7) | ✅ | EVT-1..4 as artifact-riding records; measurements are reads |
| Every blocking DoD row traces to a requirement, guardrail, release criterion, or rubric | ✅ | DOD-1..9 sourced by reference |
| Risks and assumptions have owners (§6.3) | ✅ | R1..R6 |
| Human-judgment criteria name an owner and a rubric (§5.8) | ✅ | DOD-9 (@mdarabi; owned-set rubric) |
| DoD complete: every blocking row sourced and evidenced; every n/a explained | ✅ | no n/a rows |
