# Review: prd:argus-unattended-drive/000 — review report

| Field | Value |
|---|---|
| **In one line** | Approve after two rounds: round 1 bounced the base on two blocking findings (the record's parked P-4 dropped from the design handoff; M1's stated feed unable to compute two of its three touch classes) plus two advisories (wake-channel death never loud; FR-10 freezing a volatile truth into a Must); the revision closed all four against their acceptance conditions — Q6 carries P-4, M1 feeds from EVT-1..EVT-4, UB-9 makes await-staleness a named defect, FR-10 rebound to the operational binding — with no regression in the changed spans and no new findings. |
| **Relevant-when** | auditing or consuming the review of the argus-unattended-drive PRD; asking why its 000-base was bounced at round 1; tuning prd-review calibration against a T3 factory-control-surface finding set |
| **Subject** | prd:argus-unattended-drive/000 |
| **Moment** | new-base |
| **Reviewer** | prd-review · guide 2026-07-13 (`.pyxis/authoring/prd-review.md`) |
| **Date(s)** | 2026-07-13 |
| **Verdict** | approve |
| **Tracking** | — *(the subject's introducing PR, once filed)* |
| **Template version** | review-report/v1 |

## Round 1

*Subject verified: a post-fold draft 000-base (`effective.md` as of **000**, fingerprint `sha256:646d2bf5…`; the introducing PR not yet raised — the prd-new step-6b pre-PR moment; base and projection differ only by the generated banner and the fold-appended Superseded & retired section). Floor excluded per protocol step 2: `docgate gate` exit 0 on this tree (base-free); nothing the suite enforces is raised below. Advisory period: this verdict blocks nothing; findings accumulate as measurement. Four lenses ran in parallel; each survivor cleared independent scrutiny.*

*Principal-parametric checks (the drive was authored by Argus as decision principal at tier-2): the problem/solution gate signatures are legal — `sign-gate-verdict` ∈ tier-2 per `.pyxis/skills/argus/authority-contract.md` (operator-directed promotion 2026-07-11), journaled at delivery:argus-unattended-drive#E-13, with the operator row honestly deferred to the introducing PR per template rule 15; `trigger-materialization` ∈ tier-2 backs FR-11's granted-behavior claim; attribution is typed everywhere (every §7.3 row names Argus tier-2 or the operator; the M1 crystallization cites its assumption entry #E-12 at the load-bearing spot). Basis fidelity: D-1..D-8 all carried (D-1→§4.1/§7.3 · D-2→§2.2–2.3/§3.4 · D-3→FR-1..9/NG-2 · D-4→FR-10..12 · D-5→FR-13..17 · D-6/D-7→§2.2/NG-1..6 (O-1..6 each seeded) · D-8→header Appetite); fog kept (G-1→Q1, G-2→Q2); T-1..T-4 confirmed landed in `docs/wiki/glossary.md` ("The unattended drive"); A-1 honored throughout (NG-1, FR-9, §6.2); F-1/F-3 verified against `.pyxis/pipeline.md` (stage-8 `⛔`) and delivery:argus-ledger-lint#E-36. P-1/P-2/P-3 carried as Q4/Q3/Q5 — but not P-4 (FND-1). Incidents fidelity: I2 explicitly bound (FR-6/UB-4/R1); I3's fail-closed-on-misparse class covered by UB-1/UB-2. The stage-2/5 human-gate tension with vision §4 is not raised: A-1 and D-3 are recorded rulings, the modulation rides vision §6 Level 3 + argus-vision §5, and the residual (advisory-period reviewer verdicts inside the AUTO floor for doc classes) is honestly carried as R6 + Q2 + shadow evidence. Injection sweep of subject, record, and ledgers: no planted directives. Scope lock recorded for any round 2: the named defects FND-1..FND-4 plus regressions within changed spans, nothing else; two rounds maximum.*

### FND-1 — The record's parked constraint P-4 (unattended-materialization timing) is dropped, not passed downstream

- **Severity**: blocking (CAL-2)
- **Lens**: upstream-alignment
- **Evidence**: §7.2 carries Q3 "(P-2)", Q4 "(P-1)", Q5 "(P-3)" and closes "None of Q1–Q5 blocks this contract…"; §6.2 lists four dependencies; no row anywhere cites `brainstorm:argus-unattended-drive#P-4` — vs — the record's P-4 ("Unattended materialization timing. Where the plan-freeze completion is observed…, how projection idempotency behaves under racing wakes (AP-22), and the item-verification loop at the goal object (FR-9's confirm arm). (stages 3–4)") and the prd-new synthesis rule "`P-n` constraints pass to the technical design via §6.2/§7.2, never resolved in the PRD". The drive's own journal believed it was carried — delivery:argus-unattended-drive#E-13 claims "Q1..Q5 from G-1/G-2 + P-1..P-4" — but five questions carry only five of the six anchors. UB-6 and F2 absorb parts behaviorally (double-trigger no-op; plan-freeze as a wake-signal class), yet the racing-wake idempotency mechanics and the verification-loop shape reach no design-owned handoff, so a stage-3/4 author reading this contract alone never receives them.
- **Acceptance condition**: P-4 is carried with its citation — either a fresh §7.2 row (Q6, owner design (stages 3–4), needed by Phase A design, citing brainstorm:argus-unattended-drive#P-4) or Q3 extended to name P-4's constraint set alongside P-2.
- **Disposition**: applied — §7.2 gains **Q6** citing brainstorm:argus-unattended-drive#P-4 with its full constraint set (plan-freeze observation point · racing-wake idempotency per AP-22 · the goal-object item-verification loop's shape), owner design (stages 3–4), needed by Phase A design; the §7.2 closing note and the §8 readiness row now span Q1–Q6 (Argus, decision principal, tier-2; delivery:argus-unattended-drive#E-15)

### FND-2 — M1's stated feed cannot compute two of the metric's three touch classes

- **Severity**: blocking (CAL-1)
- **Lens**: coherence
- **Evidence**: §3.2 M1 defines its signal as "non-owned operator touches per delegated problem (approve/merge of floor-clearing proposals, **projection prompts**, **manual resumes**)" with target 0, yet its Feeds cell names only "EVT-1, EVT-2" — the lane rows (`lane_verdict`, `auto_act`), which evidence the merge joint alone. The prompt-free projection joint is evidenced by EVT-4 (`materialization_confirmed`, feeds M3 only) and the unattended-resume joint by EVT-3 (`wake_delivered` with `delivered-by: signal/backstop`, feeds M4 only) — vs — the template §3.2 rule "every metric names the event that feeds it (§5.7)" and the subject's own §8 claim "EVT-1..4 as artifact-riding records; measurements are reads". As wired, M1 = 0 is not computable from its named feed; the goal→signal→metric→event chain for the primary metric is broken at the last hop.
- **Acceptance condition**: M1's Feeds cell names the complete feeding set — EVT-1..EVT-4 (or EVT-1/EVT-2 plus the delivery-ledger touch reads) — such that the touch count across all three classes is computable from the named sources alone.
- **Disposition**: applied — M1's Feeds cell now names **EVT-1..EVT-4** with the per-class mapping (approve/merge ← EVT-1×EVT-2 join; projection prompts ← EVT-4's new `trigger: wake/prompt` property; manual resumes ← EVT-3's `delivered-by`, extended `signal/backstop/manual` with fires-when widened to any front-door resume); EVT-3 and EVT-4 now also feed M1 in §5.7 — all three touch classes computable from the named sources alone, no second telemetry plane (Argus, decision principal, tier-2; delivery:argus-unattended-drive#E-16)

### FND-3 — Nothing makes the wake channel's own death loud while a drive awaits

- **Severity**: advisory (CAL-4 — not §8-load-bearing: F3's named failure rows (lost signal, parked/closed wake, idle tick) are covered; this is the recovery path's meta-failure)
- **Lens**: completeness-critic
- **Evidence**: FR-17 covers a *lost signal* (the backstop delivers); UB-7 covers the idle tick; NFR-5 makes the *lane's* state readable — no row states what happens when the wake channel/backstop itself is dead or misconfigured while completions are awaited: both delivery paths fail and the drive stalls indefinitely with nothing alarming, at exactly the joint this feature removes the operator-as-clock from (prd:argus FR-15's aging bumps fire only "at a drive touchpoint", and the wake is now what produces touchpoints) — vs — AP-16 ("nothing loops or skips silently") and argus-vision §6 ("Fail closed, get louder… Silence is a defect class, not a resting state").
- **Acceptance condition**: a UB or NFR row making awaited-completion staleness observable-or-alarmed — e.g. the awaited set and its oldest age readable from repo/ledger state, with exceeding a configured bound a named defect surfaced at the next touchpoint or state-read drill (NFR-5's pattern) — or an explicit reasoned park of backstop-liveness into Q3.
- **Disposition**: applied — new **UB-9** (Must, med): the awaited set and each await's age readable from repo/ledger state (FR-13's registration), an await past the configured staleness bound a **named wake-staleness defect** surfaced at any state read (NFR-5's pattern) or touchpoint, fit = the both-paths-dead drill surfaces it from repo data alone; §6.4 Reliability extended to UB-1..UB-9 and §8's F3 failure coverage updated (Argus, decision principal, tier-2; delivery:argus-unattended-drive#E-17)

### FND-4 — FR-10 freezes a volatile truth ("engine not built") into an immutable Must row

- **Severity**: advisory (CAL-3 — the row is verifiable today; no Must becomes unverifiable)
- **Lens**: contract-quality
- **Evidence**: FR-10 (Must) binds "…while keeping the pyxis-native engine explicitly not built", and its fit pins "the stage-8 row and footer state the external binding and the not-built engine" — vs — template rule 14 ("Frozen prose states no volatile facts… name the concept and point at the live surface that owns the current value"). The ledger itself expects the fact to expire (NG-5 parks the native engine at prd:argus WR-1 "the engine era"), yet nothing flags FR-10's expiry: the day the engine ships, this frozen Must demands the descriptive map state a falsehood until someone remembers to amend. This does not dispute D-4's recorded content (the map correction and today's honest ◐ stand); it concerns only how the requirement binds over time.
- **Acceptance condition**: pre-freeze, rephrase FR-10 to bind the map to stating the operational stage-8 binding *as it actually runs* (the fit criterion pinning today's expected external-binding content at this checkpoint), and/or name the engine-era supersession hook (a WR/NG-5 note that FR-10 is amended when prd:argus WR-1 lands).
- **Disposition**: applied — both arms: FR-10 now binds the map to "the stage-8 binding as it operationally runs — never a stale or aspirational one" (the map named as the live surface owning the current value), the fit criterion pins today's expected content **at this contract's checkpoint** (external binding · not-built engine · no deferral guidance), and the rationale names the engine-era hook (prd:argus WR-1 — NG-5's park: the fit re-pins by amendment) (Argus, decision principal, tier-2; delivery:argus-unattended-drive#E-18)

## Round 2 *(scope-locked: FND-1..FND-4 + regressions within changed spans; the cap round)*

*Subject re-verified: the refolded projection (`effective.md` as of **000**, fingerprint `sha256:cf5ef1e93525cfd931b990c226f433912ae3afcabf797496016d7be4caaf1084` — matching delivery:argus-unattended-drive#E-19's journaled identity); projection↔base consistency confirmed (the diff is exactly the generated banner plus the fold-appended Superseded & retired section — no hand-edit of the projection). Floor excluded: `docgate gate` exit 0 on this tree; nothing the suite enforces is raised. Advisory period unchanged: this verdict blocks nothing. Principal-parametric checks on the dispositions: all four were disposed by Argus as decision principal at tier-2 — `dispose-blocking-finding` and `dispose-advisory-finding` ∈ tier-2 per `.pyxis/skills/argus/authority-contract.md` (operator-directed promotion 2026-07-11), grant-checked at act time and journaled (delivery:argus-unattended-drive#E-14..E-18); revisions applied by direct edit of the draft `000-base.md` (still pre-PR, legally editable), refolded + gate green both paths (#E-19); the authoring flow's report writes confined to the four Disposition elements, the contract's one designated surface (#E-20) — verdict history and reviewer prose untouched. Injection sweep of the changed spans, the disposition lines, and ledger entries E-14..E-20: no planted directives. Scope lock honored: only the four named defects and the changed spans below were examined.*

Closure verification against each finding's acceptance condition:

- **FND-1 — held (applied; verified closed).** §7.2 gains **Q6** citing `brainstorm:argus-unattended-drive#P-4` — the acceptance condition's first arm exactly: the full constraint set carried (the plan-freeze observation point as the wake's own signal class · racing-wake projection idempotency per AP-22 · the item-verification loop's shape at the goal object, prd:argus FR-9's confirm arm), owner **design (stages 3–4)**, needed by **Phase A design**; the §7.2 closing note and §8's readiness row now span Q1–Q6. The citation resolves against the record's P-4 verbatim, and all six record anchors now reach a carry (G-1→Q1 · G-2→Q2 · P-1→Q4 · P-2→Q3 · P-3→Q5 · P-4→Q6) — a stage-3/4 author reading this contract alone now receives the constraint set.
- **FND-2 — held (applied; verified closed).** M1's Feeds cell names the complete set **EVT-1..EVT-4** with a per-class mapping — approve/merge touches from the EVT-1×EVT-2 join, projection prompts from EVT-4's new `trigger: wake/prompt` property, manual resumes from EVT-3's `delivered-by` (extended `signal/backstop/manual`, its fires-when honestly widened to any front-door resume) — and EVT-3/EVT-4 each name M1 in their §5.7 Feeds column: the goal→signal→metric→event chain closes at its last hop, in both directions. No second telemetry plane appeared: the §5.7 preamble and §8's "EVT-1..4 as artifact-riding records" row hold unchanged.
- **FND-3 — held (advisory taken; applied; verified closed).** New **UB-9** (Must, med) is the acceptance condition's first arm: the awaited set and each await's age readable from repo/ledger state (FR-13's registration as the data), an await past the configured staleness bound a **named wake-staleness defect** surfaced at any state read (NFR-5's pattern) or drive touchpoint, fit = the both-paths-dead drill surfaces it from repo data alone with no surface reporting healthy. §6.4 Reliability extends to UB-1..UB-9 and §8's F3 failure coverage adds UB-9. The read-side shape adds no recurring process — the AP-19/DINV-4 posture is intact (parked problems register no awaits).
- **FND-4 — held (advisory taken; applied both arms; verified closed).** FR-10 now binds the concept — "the stage-8 binding as it operationally runs — never a stale or aspirational one" — naming the map as the live surface that owns the current value (template rule 14's exact prescription); the fit criterion pins today's expected content **at this contract's checkpoint** (external binding · not-built engine · no deferral guidance); the rationale names the engine-era hook (prd:argus WR-1 — NG-5's park; the fit re-pins by amendment). D-4's recorded content stands untouched — nothing re-judged.

Changed-span regression sweep (§7.2 Q6 + closing note, §8 readiness/F3 rows, §3.2 M1, §5.7 EVT-3/EVT-4, §5.2 UB-9, §6.4 Reliability, §5.1 FR-10, §7.4 changelog — the E-15..E-18 write set): Q6 is design-owned at P-4's own stage park, so §7.2's "none blocks this contract" and §8's zero-blocking-TBD row stay true; EVT-3's widening keeps M4 computable (`delivered-by` · `reason` · `source completion` separate wake-latency rows from manual and no-op touches); UB-9's staleness bound joins the existing configured-bound family with its home design-owned (Q3/Q5); FR-10 stays EARS-ubiquitous with Verify/Traces intact; the changelog row is accurate and attributed (Argus tier-2; E-15..E-18). No residue — **zero new findings**; no out-of-scope candidates arose, so nothing was withheld.

**Terminal verdict at the cap: approve** — both blocking dispositions applied and verified, both advisories taken and verified, no declines (no auto-escalation path), no new findings.

## Verdict history

| Round | Verdict | Date |
|---|---|---|
| 1 | request-changes | 2026-07-13 |
| 2 | approve | 2026-07-13 |

## Escalation brief

none

## Measurements

- **Rounds**: 2 (the cap; terminal)
- **Findings**: 2 blocking · 2 advisory (all round 1; round 2 delivered none)
- **Dispositions**: 4 applied · 0 declined · 0 withheld-as-violation
- **Round-2 closure**: 4/4 round-1 findings verified closed against their acceptance conditions

## Overlay corrections

*Post-approve entries only — the M2 residual data; frozen rounds and history are never edited.*

none
