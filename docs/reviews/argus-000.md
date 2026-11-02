# Review: argus 000-base — review report

| Field | Value |
|---|---|
| **In one line** | Approve-with-advisories after two rounds: round 1 bounced the base (7 blocking + 2 advisory — sibling collisions on stage-7 projection ownership and the stage-1 operator-only contract, the AP-26 "landed" gap, a dropped D-4 sub-clause, and unwired park-alarm/EVAL-4/FR-4-audit chains); the revision closed all nine against their acceptance conditions, leaving one new advisory (stale volatile FR ranges after FR-15's addition). |
| **Relevant-when** | auditing or consuming the review of the argus base PRD; asking why it was bounced; tuning prd-review calibration against real findings |
| **Subject** | prd:argus/000 |
| **Moment** | new-base |
| **Reviewer** | prd-review · guide as merged 2026-07-10 |
| **Date(s)** | 2026-07-10 |
| **Verdict** | approve-with-advisories |
| **Tracking** | the subject's introducing PR (this PR) |
| **Template version** | review-report/v1 |

*Advisory period: this verdict blocks nothing — the reviewer is pre-adoption (prd:prd-review §6.5; the pipeline map's advisory note); findings and verdict accumulate as measurement, and the operator gate + merge remain the stage-2 gate.*

## Round 1

### FND-1 — FR-9 re-assigns the tracker projection that the merged plan-new sibling owns

- **Severity**: blocking (CAL-2)
- **Lens**: upstream-alignment
- **Evidence**: FR-9 "Argus shall materialize its tasks as tracker work items tagged for automated implementation" and §5.5's tracker row ("materialize plan as work items … **Owner: Argus**") — vs — prd:plan-new (merged, T3): §4.1 "deterministic tooling then projects the frozen plan onto the tracker", FR-17 (consent/freeze, critical), FR-18 (create-at-most-once idempotency, critical), FR-19 (wire-before-route, critical — the I15-class race). The subject's own FR-3 already drives stage 7 by role, making FR-9 read as a second, guarantee-free materialization path; AP-11 (one predicate, one place) forbids the twin. The record's D-8 substance (plan → tagged items → fleet) holds under either mechanism, so this disputes no ruling — it targets the PRD's assignment of the mechanics.
- **Acceptance condition**: FR-9 and §5.5's tracker row name stage-7's existing projection as the mechanism — Argus satisfies FR-9 by driving the planner's checkpoint and projection as decision principal — or the bypass is recorded explicitly with its own consent/idempotency/wire-before-route safety story equal to plan-new FR-17..19.
- **Disposition**: applied — FR-9 and §5.5's tracker row reworded: the planning stage's own contract (consent, idempotency, wiring order) is the materialization mechanism; Argus triggers and confirms as decision principal, never re-implements

### FND-2 — the "additive" principal seam silently breaks feature-brainstorm's merged contract at stage 1

- **Severity**: blocking (CAL-2)
- **Lens**: upstream-alignment
- **Evidence**: §4.2 "the principal seam landing in stage documents as additive amendments at implementation time" and A1 "the stage guides accept a principal seam as a small additive amendment each" (fit: "NR-1/NR-2 prove no behavior change" — NR-2 checks reviewer fixtures only) — vs — prd:feature-brainstorm (merged): NG-4 ("An autonomous / AFK mode … stage-1 decisions are the operator's **by definition**; an unmanned interview is the sev-1 failure mode"), FR-6 ("only from an explicit operator answer given in the live session"), EVAL-2 (zero-tolerance), §3.3 guardrail. The basis itself flags this: record F-6 — stage 1 is "the exact surface the AD inverts; an AD-ruled session needs its own anchor semantics". Driving stage 1 as agent principal breaks an unamended contract — the subject's own FR-5 legality test.
- **Acceptance condition**: the PRD names the stage-1 reconciliation — a contract-changing amendment through feature-brainstorm's sanctioned channel (rescoping NG-4, re-anchoring FR-6/EVAL-2 to D-6's principal/anchor semantics) landed before Argus drives stage 1 unmanned, or stage 1 excluded from the unmanned drive until then — and A1/NR-2 restated to cover the stage-1 eval fixtures.
- **Disposition**: applied — §4.2/A1/NR-2 restated: seams land through each stage's own evolution channel (contract-changing amendment where a stage binds operator-only — stage 1's live-answer contract), each landed before Argus drives that stage unmanned; NR-2's fit now covers every touched stage's own eval fixtures, including stage 1's zero-tolerance set

### FND-3 — "landed" is undefined; the drive-home loop repeats incident I2 as written

- **Severity**: blocking (CAL-2)
- **Lens**: upstream-alignment
- **Evidence**: FR-10 "drive them to landed" (fit: "dogfood PRs reach landed"), EVT-3 "the delivery ledger closes with evidence", F1 step 5 — no row requires observing the goal object — vs — AP-26 ("Goal-state is accepted only by observing the goal object itself — the commit is an ancestor of the mainline … never by reading the upstream status") and incident I2 (2026-07-09, this repo, Graduated: AP-26): a stacked train showed every badge merged and CI green while nothing reached main. Argus closes delivery ledgers over exactly this surface; a badge-trusting close makes EVT-3/M1 count undelivered features.
- **Acceptance condition**: "landed"/"delivered" defined as mainline-observed (ancestry of the shipped commit against the mainline, per AP-26), binding FR-10's fit and EVT-3's close condition, with the dogfood evidence carrying that verification per PR.
- **Disposition**: applied — landed/delivered defined as mainline-content-verified (AP-26) in FR-10's statement and fit, EVT-3's close condition, and DOD-4's evidence (per-PR mainline-ancestry verification)

### FND-4 — D-4's per-problem grant override and optional pre-rulings are not carried

- **Severity**: blocking (CAL-2)
- **Lens**: upstream-alignment
- **Evidence**: record D-4 clause (2): "the grant (D-3 row 4's envelope) defaults from per-repo config, **overridable per problem** — never a required intake field; **optional pre-rulings stay available**, never required" — vs — the subject, which carries only the repo-scoped half: §3.3 "Cost per problem | within the repo grant envelope", UB-4, NFR-3 "the repo's grant envelope", header appetite "within the per-repo grant envelope", and a front door admitting "no required structure beyond the idea" (§5.5). The "never required" half survives; the ruled availability of a per-problem override and pre-rulings appears nowhere — a silent narrowing of the ruled intake (basis fidelity on a record-fed subject).
- **Acceptance condition**: the intake contract (FR-1 and/or §5.5's front-door row) admits the optional per-problem envelope override and optional pre-rulings as recorded problem-ledger fields, and §3.3's cost guardrail + UB-4 read the problem's effective envelope.
- **Disposition**: applied — FR-1 and §5.5's front-door row admit the optional per-problem envelope override and pre-rulings, recorded as problem-ledger fields binding as rulings-in-advance; §3.3, UB-4, NFR-3, and the header appetite now read the problem's effective envelope (repo default, per-problem override)

### FND-5 — the park alarm has no requirement, no owner, and no stated steady-state cost

- **Severity**: blocking (CAL-1)
- **Lens**: coherence
- **Evidence**: F2 step 2 "the park alarm gets louder with age, never quieter" and §5.5's fleet row "unclaimed items age → alarm on delivery ledger" — vs — §5.1/§5.2, where no FR or UB implements an alarm for either spine level; §8's flow-coverage row maps F2 to FR-6..8, which cover emit/park/resume only, so the row is trusted, not verified. The inherited convention doesn't apply: in prd:prd-review/UB-6 the alarm is owned by *the surrounding flow* — Argus is the outermost flow for its own human-level briefs, so no owner exists; and AP-19 requires every recurring path to state its steady-state cost, which the parked state never does (a session-form agent that isn't running cannot make anything louder).
- **Acceptance condition**: a requirement naming the alarm mechanism and owner for both spine levels (e.g., alarm data on the ledger surfaced by a named bootstrap convention — a labeled tracker item or next-invocation surfacing) and the parked problem's steady-state cost, restoring §8's F2 coverage claim.
- **Disposition**: applied — FR-15 added: ledger-exposed age plus touchpoint-driven visibility bumps for parked briefs and aging driven items at both spine levels, owner Argus-at-touchpoints, steady-state cost stated as zero (event-driven; a non-running session spends nothing); §8's F2 coverage row cites FR-15

### FND-6 — EVAL-4 feeds nothing: a threshold with no consumer

- **Severity**: blocking (CAL-1)
- **Lens**: coherence
- **Evidence**: EVAL-4 "brief quality … operator-judged ≥ 4/5 per brief in the dogfood window" — vs — DOD-3 (covers only "breaking set, self-grant/outward set, injection corpus" = EVAL-1..3), DOD-4's rubric ("every ruling attributed, every assumption recorded, zero missed-breaking, floor respected" — no brief-quality item), and §6.4 ("EVAL-1..3 at threshold"). No FR fit cites EVAL-4 either, so the chain eval→gate is broken; sibling precedent binds §5.9 wholesale (prd:prd-review/DOD-3 cites "§5.9"). An eval defined for the dogfood window that nothing requires to run or pass is a dead branch in the contract.
- **Acceptance condition**: EVAL-4 wired into a blocking row (DOD-3, DOD-4's rubric, or §6.4) — or explicitly marked a non-blocking measurement with its consumer and revisit point named.
- **Disposition**: applied — EVAL-4 wired into DOD-4's blocking rubric: every emitted brief rated ≥ 4/5, vacuous when no brief occurred

### FND-7 — FR-4's fit criterion cannot fail on the failure it guards

- **Severity**: blocking (CAL-3 — escalated per its condition: the defect makes a Must row unverifiable)
- **Lens**: contract-quality
- **Evidence**: FR-4 fit "100% of dogfood assumptions recorded, attributed, at the artifact they shape | Inspection", with DOD-4's evidence being "the delivered feature's ledgers + PRs" — vs — prd-template/v1 Appendix A ("every requirement carries a measurement that **decides pass/fail**"). The denominator "dogfood assumptions" is observable only through the recording FR-4 mandates: an unrecorded assumption never enters the measured set, and the named evidence (ledgers + PRs) cannot surface it, so the central clause is vacuous as verified — while the unrecorded assumption is precisely the laundering-adjacent case (ABUSE-3, M2's blind spot) the row exists to prevent.
- **Acceptance condition**: FR-4's fit names its comparison base — audited against the drive session transcript and an artifact-derivation sweep (every judgment not determined by the problem ledger + governing documents appears as a recorded assumption) — with DOD-4's required evidence extended to include that audit.
- **Disposition**: applied — FR-4's fit re-based on the drive-session transcript plus an artifact-derivation sweep (every judgment not determined by the problem ledger + governing documents appears as a recorded, attributed ruling; denominator independent of the assumptions list); DOD-4's required evidence extended with that audit

### FND-8 — the floor's only granted act class (dispositions) has no eval family

- **Severity**: advisory (CAL-3)
- **Lens**: contract-quality
- **Evidence**: §6.5's floor stage — "floor (advisory dispositions only) … Minimum evidence: EVAL families green" — vs — §5.9, whose families cover breaking detection (EVAL-1), assumption legality (EVAL-2), injection (EVAL-3), and briefs (EVAL-4), but never disposition behavior: applying vs declining-with-reason seeded reviewer findings, honoring decline→auto-escalate (prd:prd-review/FR-8), holding the NG-4 never-re-judge boundary. The first authority Argus exercises is thus the one behavior its evidence gate never exercised; §5.9 adequacy on a T3 model-driven feature warrants the coverage.
- **Acceptance condition**: an eval family (or an EVAL-2 extension) over disposition behavior on seeded reviewer findings — apply/decline-with-reason quality, auto-escalate on decline honored, zero re-judging — green before the floor grant is exercised.
- **Disposition**: applied — EVAL-2 extended with disposition cases on seeded reviewer findings (apply and decline arms): 100% reasons recorded, decline-of-blocking auto-escalates, zero re-judging, green before the floor grant is exercised

### FND-9 — the pre-registered ship rule leaves N a free variable

- **Severity**: advisory (CAL-3)
- **Lens**: contract-quality
- **Evidence**: §3.3 "authority tiers promote only at **N-problem checkpoints** with M2 ≥ target", M2's target "≥ 90% **per checkpoint window**", §6.5 "one checkpoint window at M2/M3 targets" — N and the window length are never pinned (only the ten-problem post-launch trigger is, §6.7) — vs — Appendix A's fit discipline (a measurement that decides) applied to a decision rule: a pre-registered promotion rule with an unbound parameter can be satisfied by declaring a checkpoint opportunistically, softening D-7's promote-slow posture in exactly the direction the rule exists to prevent. Pinning N implements D-7; it disputes nothing.
- **Acceptance condition**: N pinned — as a number, or as a named versioned field of the authority contract with its initial value stated in this PRD — so the ship rule is decidable at any point in time.
- **Disposition**: applied — the checkpoint window pinned to ten problems in §3.3's ship rule and FR-14, aligned with §6.7's trigger

## Round 2 *(scope-locked: the nine named defects + regressions within changed spans)*

Closure verification against each finding's acceptance condition, on the re-folded revision (gate suite green):

- **FND-1 — closed.** FR-9 now reads "through the planning stage's own contract — which owns consent, idempotency, and wiring order"; §5.5's tracker row assigns the write path to the planning stage with Argus as trigger/confirm. The twin materialization path is gone; ownership matches the acceptance condition's first arm exactly.
- **FND-2 — closed.** §4.2 names the evolution channel per stage class — contract-changing amendments where a stage binds operator-only (the stage-1 live-answer contract named), additive elsewhere — **with each seam amendment landed before Argus drives that stage unmanned** (the sequencing arm); A1 restates the same; NR-2's fit now covers every touched stage's own eval fixtures *including stage 1's zero-tolerance set*.
- **FND-3 — closed.** "Landed" is defined in FR-10's statement ("verified present in mainline content, never platform status alone") and fit ("mainline content at the merge base"); EVT-3's close condition carries the AP-26 binding; DOD-4's evidence adds per-PR mainline-ancestry verification.
- **FND-4 — closed.** FR-1 admits the optional per-problem grant override and pre-rulings as recorded, binding ledger fields; §5.5's front-door row mirrors it; §3.3's cost guardrail, UB-4, NFR-3, and the header appetite all read the problem's effective envelope (repo default, per-problem override).
- **FND-5 — closed.** FR-15 names the mechanism (ledger-exposed age + threshold-triggered tracker visibility bump), the owner (Argus at drive touchpoints), both spine levels (unanswered briefs; stage escalations and unclaimed items), and the steady-state cost (zero — event-driven; the acceptance condition's "next-invocation surfacing" example is the shape chosen); §8's F2 coverage row now cites FR-15.
- **FND-6 — closed.** EVAL-4 is consumed by blocking DOD-4's rubric (≥ 4/5 per emitted brief, vacuous when none occurred).
- **FND-7 — closed.** FR-4's fit names the comparison base verbatim to the condition: the drive-session transcript plus an artifact-derivation sweep, denominator independent of the assumptions list; DOD-4's required evidence includes that audit.
- **FND-8 — closed.** EVAL-2 gains the disposition arm on seeded reviewer findings (apply and decline), with recorded-reasons, decline-of-blocking auto-escalation, zero re-judging, and the green-before-floor-grant sequencing in its threshold.
- **FND-9 — closed.** The window is pinned: ten-problem checkpoints in §3.3's ship rule and FR-14, aligned with §6.7's trigger.

Changed-span regression sweep (gate tables, status ladder, changelog, and every edited row examined): one residue delivered below; the signed gate tables and status advance are consistent with the template's solo-operator rule and the still-pending launch gate — no finding.

### FND-10 — the revision's new FR-15 leaves the volatile FR ranges stale

- **Severity**: advisory (CAL-3)
- **Lens**: contract-quality
- **Evidence**: the revision added FR-15 (a Must row) while DOD-1's Source column still cites "FR-1..14" and §8's row-3 note still reads "FR-1..14 all carry both" — vs — prd-template/v1 rule 14 ("never write a count, range, enumeration … a later change can falsify; name the concept and point at the live surface"). The chain itself holds — DOD-1's criterion text is universal ("**All** Must FRs pass their fit criteria"), so FR-15 remains bound and verifiable, which is why this is a stale-citation writing-rule slip (CAL-3's default), not a coherence chain break and not the CAL-3 escalation condition (no Must row becomes unverifiable).
- **Acceptance condition**: DOD-1's Source and §8's row-3 note reference the live Must-FR set in non-volatile form (e.g., "every Must FR row in §5.1") or are extended to include FR-15.
- **Disposition**: applied — DOD-1's Source and §8's row-3 note de-volatilized to "every Must FR row in §5.1" (template rule 14); changelog row added

## Verdict history

| Round | Verdict | Date |
|---|---|---|
| 1 | request-changes | 2026-07-10 |
| 2 | approve-with-advisories | 2026-07-10 |

## Escalation brief

none

## Measurements

- **Rounds**: 2 (the cap; terminal)
- **Findings**: 7 blocking · 3 advisory (FND-10 delivered in round 2 on a changed span)
- **Dispositions**: 9 applied · 0 declined · 0 withheld-as-violation · 1 pending (FND-10, advisory — the authoring flow's to take or leave)
- **Round-2 closure**: 9/9 round-1 findings verified closed against their acceptance conditions

## Overlay corrections

none
