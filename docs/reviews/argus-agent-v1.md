# Review: argus-agent-v1 — design-review report

| Field | Value |
|---|---|
| **In one line** | approve-with-advisories at round 2 (terminal) — all nine round-1 findings closed against their acceptance conditions within the frozen vocabularies (the request form, the envelope watch, UB-8's routing, FR-9's confirm arm, D-7's label, the intake homes, and the three invariant bindings); two advisories delivered on the changed spans (FND-10: the watch's mid-drive spend home is undeclared in the frozen entry contracts; FND-11: UB-8's drill bound in no acceptance row), dispositions pending. |
| **Relevant-when** | auditing or consuming the review of argus-agent-v1; asking why it was bounced at round 1; tuning tech-design-review calibration |
| **Subject** | design:argus-agent-v1 · shape: none · supersedes: — |
| **Moment** | new-design |
| **Reviewer** | tech-design-review · guide `.pyxis/authoring/tech-design-review.md` as of 2026-07-11 · **advisory period** (verdicts block nothing; findings accumulate as measurement) |
| **Date(s)** | 2026-07-11 |
| **Verdict** | approve-with-advisories |
| **Tracking** | the subject's introducing PR (unfiled at review time) |
| **Template version** | design-review-report/v1 |

## Round 1

*Review basis: post-lint draft (gate suite incl. `lint-tech-designs` and `verify-index` green), shape none — every evidence quote below is `design:`. Record-fed — basis record `design-brainstorm:argus-tech-design` is frozen on main (merged in #177): its rulings are read as frozen content, fidelity-checked and never disputed. Upstream corpus: prd:argus effective (as of 000), the frozen sibling argus-artifacts (joint consistency), plan-new-v1, prd-review-artifacts, tech-design-review-artifacts, architecture-principles, incidents I1–I3, the design index. Non-findings, recorded once: the Scope's exclusion of the data contracts is the record's D-1 partition, honored (RW-5 row cites it); D-6's two operator requirements (continuous session should · any-CLI must) are carried into §2, §3.2, DINV-5, and §4's rejections; the §4 alternatives are genuinely distinct, each verdicted with record citations — the record's ALT-1..6 ledger belongs to the sibling's questions (DQ-2/DQ-3) and is carried there, while this design's questions were extends-class panels with no ALT rows to carry; materialization ownership honors plan-new-v1's ground (trigger/confirm split, consent rows intact); the reviewers' disposition and escalation semantics are mirrored, not re-decided (decline-of-blocking auto-escalates, never-re-judge held); the sibling's entry vocabulary is used consistently everywhere except FND-6's intake sentence; incidents matched (I2 → AP-26 → the resume rule). Killed in adversarial survival: AP-17/AP-15 silent-omission candidates (their substance is PRD- and sibling-bound; a Principles selection is not dishonest for omitting them); the Security "none" header (no new surface class — every outward act crosses an already-bound contract, and the named-reviewer bar is carried); `fixtures/` as a volatile path (concept + relative name, the reviewer designs' precedented shape); resume-without-answer (implied by construction — a parked problem's resume routes through answer reconciliation, which requires the `A-n`); the bump's exactly-once phrasing (mirrors the PRD's own FR-15 fit).*

### FND-1 — the ungranted-act request has no defined form, and the UB-1/UB-4/UB-5 briefs cannot map onto the two-class spine

- **Severity**: blocking (CAL-1)
- **Lens**: coherence
- **Evidence**: design: "an ungranted decision/permission act is requested from the human and journaled as such, never queued invisibly" (§3.1) and design: "every beyond-floor act requested and journaled" (A-2) — vs — design: §3.3, where the only defined escalation machinery is "a breaking decision — `class: override` … or `class: deadlock` — emits the brief … parks the problem fail-closed and **ends the session**", and the frozen sibling's contracts (argus-artifacts §3.2): a closed entry vocabulary with no request type and `brief-emitted` carrying the closed enum `class: override | deadlock`. Under the graduated floor, requests are the dogfood's most-travelled edge (every gate signature and blocking disposition is beyond-floor), yet the design never states the request's journal form, whether the problem parks or the session holds while a request pends, how a pending request ages into FR-15's visibility path, or how the PRD's *required* briefs for prd:argus/UB-1 ("a decision brief naming the self-grant attempt"), UB-4 ("a brief with spend evidence"), and UB-5 ("a brief naming the surface") — none an override-of-governance/`A-n`, none a rounds-exhausted deadlock as §3.3 defines them — are representable in the spine at all.
- **Acceptance condition**: §3.1/§3.3 define the request-and-non-breaking-escalation mechanics: the journal form within the sibling's frozen vocabulary; the session/problem posture while a request pends (park vs hold, and how it differs from a breaking park); the request's aging/visibility path (FR-15's "any driven item" breadth is available); and the stated mapping of UB-1/UB-4/UB-5's briefs onto the spine's classes — or, if the frozen class enum genuinely cannot carry them, the named resolution routed through the sibling's sanctioned evolution channel, never silence.
- **Disposition**: applied — the ungranted act has a defined form (§3.1): not breaking, no park; a `ruling-recorded` entry + the request placed at the needed spot + a derived-block open item aging under FR-15's aging arm; human completion observed and journaled as `reconciliation`. The brief `class` enum maps the whole escalation space (§3.3): `override` = any human-owned boundary (governance, A-n, self-grant, envelope, outward — body names the row); `deadlock` = convergence exhaustion — no sibling change needed

### FND-2 — UB-4's envelope check is unimplemented, and §2's "never-list needs no check" claim is false for the envelope row

- **Severity**: blocking (CAL-2)
- **Lens**: upstream-alignment
- **Evidence**: design: "the never-list needs no check because the protocol contains no path that performs those acts" (§2, the Decision callout) and the word "envelope" appearing only in intake parsing ("optional envelope override and pre-rulings parsed into their fields", §3.4) — vs — prd:argus/UB-4 ("If the problem's grant envelope … would be exceeded → park the problem and escalate the budget question"; fit: "simulated envelope breach **parks within that drive step**, honoring the override when present") and the frozen sibling's never-list row "exceeding the problem's effective envelope" (argus-artifacts §3.3). Driving *is* the path that exceeds an envelope — spend accrues on every step — so only an accounting check can park in time; the design binds no check, no cadence, no mid-drive spend-evidence source (the sibling's Q-2 pins spend fields on the `delivered` close entry only, adapter-honest), and no budget-brief route. The Serves claim on UB-4 is not implemented by content.
- **Acceptance condition**: the drive states the envelope check — when it runs (bounding the breach window to UB-4's "within that drive step"), its mid-drive spend-evidence source (adapter-honest, consistent with the sibling's Q-2 posture), and the park + budget-brief route (with FND-1's class mapping) — and §2's no-check claim is carved to exclude the envelope row, where a check exists and is required.
- **Disposition**: applied — §3.2's envelope watch implements UB-4 (effective envelope read at intake; cumulative spend summed into the derived block per append; projected crossing stops the step, emits the budget brief, parks; adapter-honest degradation stated); §2's false claim corrected to name the envelope as the by-measurement exception

### FND-3 — UB-8's confirmed-missed-breaking flow is unimplemented

- **Severity**: blocking (CAL-2)
- **Lens**: upstream-alignment
- **Evidence**: design: the Serves row claims prd:argus/UB-8, yet no §3 subsection, A-row, or risk row states any behavior for a confirmed missed-breaking — §3.3 covers brief/park/bump, §3.9 covers birth and graduation — vs — prd:argus/UB-8 ("If a missed-breaking is confirmed after the fact → append an incident entry and demote the touched authority tier immediately"; fit: "drill: a seeded confirmation produces both effects") and §3.3's zero-tolerance guardrail. The sibling gives demotion its data form (a change-history entry, human-landed per its writer discipline) and the overturn citation its `missed-breaking: yes | no` field, but the *routing behavior* — who detects the confirmation at a touchpoint, prepares the incident entry and the demotion's evidence for the human's immediate data PR, and journals the act — is this design's ground and is absent; no other live design serves UB-8 (the index).
- **Acceptance condition**: the design states the confirmed-missed-breaking flow — the touchpoint/reconciliation behavior that routes a `missed-breaking: yes` overturn into the incident append and the immediate, human-landed demotion, journaled — and binds UB-8's drill in acceptance.
- **Disposition**: applied — UB-8's flow bound in §3.3: the confirming human lands the incident entry + the immediate demotion (writer discipline); Argus is the named backstop — an observed missed-breaking overturn without a landed demotion is bump-class at any touchpoint, raised immediately, never re-judged

### FND-4 — FR-9's confirm arm has no home: the design triggers materialization but never confirms the tag

- **Severity**: blocking (CAL-2)
- **Lens**: upstream-alignment
- **Evidence**: design: "bridge the approved plan through the planning stage's own materialization" (§2) and design: "plan-new's consent rows intact" (§3.5) — the trigger half only — vs — prd:argus/FR-9's second shall: "and shall **confirm** the resulting items carry the repo-configured automated-implementation tag" (fit: items "carry the configured tag, and are claimed by the fleet"). No design content performs confirmation: the resume rule verifies existence ("the tracker item exists", §3.2), not the tag, and no A-row traces FR-9. Ownership is correctly plan-new-v1's (non-finding above); the confirm duty is Argus's and is unstated.
- **Acceptance condition**: the bridge states the confirm step — after triggering materialization, verify the resulting items carry the repo-configured tag against the tracker (the goal object, AP-26's posture), journaled as the act's entry — in §2's bridge clause or a §3 sentence.
- **Disposition**: applied — FR-9's confirm arm in §3.1: post-trigger goal-object verification of each item (exists + configured tag), journaled; discrepancies routed back through the planning stage's own contract, never edited directly

### FND-5 — D-7's park-time parked-label clause is dropped (record fidelity)

- **Severity**: blocking (CAL-2)
- **Lens**: upstream-alignment
- **Evidence**: design: §3.3's park mechanics — brief entry, fail-closed park, session end, answer/wake path, threshold bumps — with no park-time tracker marking anywhere — vs — the frozen record's D-7 clause (2): "park flips the derived state block … and the session ends — no live session while parked; **where the repo binds a tracker, the problem's item carries a parked label pointing at the brief**" (design-brainstorm:argus-tech-design#D-7). The label is immediate at park and points at the brief (tracker-side discoverability); FR-15's bump is threshold-aged — distinct mechanisms, so the FR-15 citation does not carry the clause. A live ruling's sub-clause is silently narrowed on a record-fed subject (the argus-000 FND-4 class).
- **Acceptance condition**: §3.3 carries D-7's clause — at park, where the repo binds a tracker, the problem's item gains the parked label pointing at the brief — or records the deviation from the ruling explicitly with its reason.
- **Disposition**: applied — D-7's parked-label clause restored in §3.3's park mechanics (tracker item labeled, pointing at the brief, where the repo binds one)

### FND-6 — "journaling intake" misdirects intake recording to the delivery journal, whose frozen vocabulary has no intake type

- **Severity**: blocking (CAL-2)
- **Lens**: upstream-alignment
- **Evidence**: design: "minting the slug, instantiating both ledgers from their templates, **journaling intake** (idea verbatim; optional envelope override and pre-rulings parsed into their fields), and entering the drive" (§3.4, a Decision callout) — vs — the frozen sibling's contracts: the delivery journal's closed type vocabulary declares no intake entry (argus-artifacts §3.2), and intake-time facts are the *problem* ledger's ground — closed header fields, the Idea section verbatim, pre-rulings as `A-n` entries (argus-artifacts §3.1). "Journal" is a load-bearing term in this pair (the `E-n` sequence); as written, an implementer either mints an undeclared entry type (breaking the sibling's closed vocabulary) or coerces intake into a wrong-semantics type.
- **Acceptance condition**: §3.4 names where each intake datum lands per the sibling's frozen contracts (problem.md header fields, Idea, `A-n` pre-rulings) and re-words "journaling" so no delivery-journal intake entry is implied.
- **Disposition**: applied — §3.4 reworded: the problem ledger IS the intake record (idea verbatim, override + pre-rulings as its fields/A-n); the journal opens with the first `stage-entered` entry — no intake type needed in the frozen vocabulary

### FND-7 — DINV-5 (the any-CLI must), DINV-6, and DINV-3's mid-drive arm are bound by no verification

- **Severity**: blocking (CAL-4 — escalated: the gap is load-bearing for acceptance — A-1..A-6 are what DOD-4 and A1-of-basis consume, and an implementation can clear all six while hard-wiring one CLI's primitives, self-implementing a tagged item, or caching a grant across a mid-drive ladder change)
- **Lens**: feasibility-completeness
- **Evidence**: design: "**DINV-5** — The protocol requires only sequential agentic execution, files, and a shell …" and "**DINV-6** — Argus never implements fleet-tagged work items …" (§3.10) — vs — the acceptance table (A-1..A-6), which traces #DINV-1 (A-3) and #DINV-4 (A-6) but binds nothing to DINV-5 or DINV-6; DINV-3's mid-drive freshness rides only R4's split cite "A-4 (flip drill, with the sibling's A-4)", and the sibling's flip drill (argus-artifacts A-4) never states a mid-session flip. DINV-5 encodes the operator's *must* (design-brainstorm:argus-tech-design#D-6) stated in §2's first line ("vendor-neutral"); the PRD's own R5 row promises the principal/hands race class is "tested".
- **Acceptance condition**: each of DINV-3 (mid-drive ladder change), DINV-5, and DINV-6 gains a bound verification — an A-row or a named arm of A-3/A-4 (e.g. a second-CLI resume leg or a guide-fallback inspection for DINV-5; a seeded tagged-item temptation fixture in EVAL-2 for DINV-6; a mid-drive flip case for DINV-3) — the form is the author's, the binding is not.
- **Disposition**: applied — A-7 added (a scripted drive segment on a second agentic CLI — DINV-5 demonstrated); A-4 extended to name EVAL-2's fleet self-implement negative (DINV-6) and a mid-drive ladder-flip case (DINV-3's mid-session arm)

### FND-8 — R5's verification cites a nonexistent A-7, and its re-plan mitigation is bound nowhere

- **Severity**: advisory (CAL-4)
- **Lens**: feasibility-completeness
- **Evidence**: design: "R5 | fleet items sit unclaimed and the drive stalls silently | … | FR-15 aging + bumps at touchpoints; **re-plan disposition on persistent no-claim** | **A-7**" (§5) — vs — the acceptance table, which ends at A-6; the sibling's A-7 (per-DINV defect fixtures, which §3.6 correctly calls "its A-7") is unrelated to unclaimed items, so the anchor dangles either way. A mitigation without verification is intent (the template's own rule), and "re-plan disposition on persistent no-claim" appears in no binding element; A-6's drill covers the park arm of FR-15 only, leaving the aging-item arm untested.
- **Acceptance condition**: R5's verification cites an anchor that exists — extending A-6 (or a new row) with the aging-item arm is the natural closure — and the re-plan disposition either lands in a binding element or is dropped from the mitigation.
- **Disposition**: applied — R5's verification re-pointed at the extended A-6; the second-threshold disposition (re-plan or escalate) bound in §3.3's touchpoint duties; A-6 extended with the seeded aging unclaimed item

### FND-9 — the touchpoint's bump-evaluation scope is unstated

- **Severity**: advisory (CAL-4)
- **Lens**: feasibility-completeness
- **Evidence**: design: "At every touchpoint, parks and aging items past the repo-config threshold get exactly one `bump-issued` visibility bump" (§3.3) and design: "bumps evaluate only at touchpoints" (#DINV-4) — vs — the two readings both texts admit: the touched problem's own parks/items only (under which a parked problem's brief bumps only when the human already touches it), or a repo-wide sweep at any problem's touchpoint (under which one problem's session appends `bump-issued` to *another* problem's open journal — a cross-problem write the design never names, interacting with Q-1's shared-clone question). A-6's drill is decidable under either reading, so this is incompleteness, not contradiction — but the guide author must pick, and the pick has file-ownership consequences.
- **Acceptance condition**: §3.3 states the evaluation scope — the touched problem's own parks/items, or a repo-wide sweep — and, if repo-wide, names the cross-problem write path (whose journal carries the entry).
- **Disposition**: applied — touchpoint bump scope stated in §3.3: own-problem only, no cross-problem writes (AP-22/AP-27 disjointness); the repo-wide sweep is the checkpoint walk's job

## Round 2 — scope-locked re-review

*Scope: the nine named defects (each disposition verified against its finding's stated acceptance condition) plus regressions within the changed spans (§2's carve, §3.1's request form + confirm arm, §3.2's envelope watch, §3.3's rewrite, §3.4's intake rewording, R5, A-4/A-6/A-7). Frozen-vocabulary consistency re-checked against the sibling for every revision — the round-1 crux: `ruling-recorded` and `reconciliation` are declared types used within their semantics; the `class` mapping supplies semantics the frozen sibling never defined, contradicting nothing written; the intake rewording lands every datum on its sibling-declared home. Killed in adversarial survival: the request-vs-escalation terminology tension (§3.3's "every human-bound escalation" vs §3.1's non-escalation requests — the two texts define the distinction explicitly); A-4 extending EVAL-2's set beyond §3.6's descriptive enumeration (the binding row states the set; the description is not contradicted); the UB-8 backstop's raise-path for closed-problem overturns (own-problem scope routes those to the checkpoint walk by the stated rule, and the own-problem case follows the bump grammar — inferable by construction).*

- **FND-1 — closed.** The request form is defined within the frozen vocabulary (`ruling-recorded` + spot placement + derived-block open item aging under FR-15's aging arm; completion observed as `reconciliation`); park-vs-hold answered (no park); the UB-1/UB-4/UB-5 briefs are mapped onto the frozen enum with stated semantics (`override` = any human-owned boundary, body names the row) — the acceptance condition's every arm, no sibling change needed.
- **FND-2 — closed.** The envelope watch runs at every append and stops the crossing step (UB-4's "within that drive step" bounded); the park + budget-brief route lands with FND-1's mapping; §2's claim carved to name the envelope as the by-measurement exception. One narrow residue on the changed span re-scoped as FND-10 (the mid-drive spend home).
- **FND-3 — closed in substance; residue re-scoped.** The flow is stated with the routing the condition demanded (confirming human lands incident + demotion per the sibling's writer discipline; Argus the named backstop, never re-judging); the condition's second arm — the UB-8 drill bound in acceptance — remains unmet and is advisory-grade on its own (an unbound single verification, FND-8's calibration class): delivered as FND-11.
- **FND-4 — closed.** The confirm arm is stated in §3.1: post-trigger goal-object verification (exists + configured tag), journaled, discrepancies routed back through the planning stage's own contract.
- **FND-5 — closed.** D-7's parked-label clause restored verbatim in §3.3's park mechanics.
- **FND-6 — closed.** Intake lands on the problem ledger's declared homes (Idea verbatim, header field, `A-n` pre-rulings); the journal opens with `stage-entered`; no undeclared entry type implied.
- **FND-7 — closed.** DINV-5 bound by A-7 (second-CLI drive segment); DINV-6 and DINV-3's mid-drive arm bound by A-4's named EVAL-2 members (the fleet self-implement negative; the mid-drive ladder-flip case).
- **FND-8 — closed.** R5 cites the extended A-6 (which now drills the aging-item arm and the second-threshold disposition); the re-plan disposition is bound in §3.3's touchpoint duties.
- **FND-9 — closed.** Scope stated: own-problem only, no cross-problem writes; the repo-wide sweep assigned to the checkpoint walk.

### FND-10 — the envelope watch's mid-drive spend source has no declared home in the frozen entry contracts

- **Severity**: advisory (CAL-3)
- **Lens**: contract-quality
- **Evidence**: design: "cumulative spend is summed from the journal's **spend-bearing entries** … Where a CLI exposes no spend signal … **entries record `unavailable`**" (§3.2, changed span) — vs — the frozen sibling's entry contract: the generic entry carries type · date · principal + contract version · gist body · external-effect identity, with type-specific fields enumerated closed, and **spend fields pinned to the `delivered` entry only** (argus-artifacts §3.2; its Q-2 pins the spend-field vocabulary there). Mid-drive per-entry spend — the watch's data source — has no declared field; an implementer either invents an undeclared structured field (against the sibling's closed posture) or guesses that gist bodies carry it informally.
- **Acceptance condition**: §3.2 states where mid-drive spend lives — carried informally in entry gist bodies until the sibling's Q-2 vocabulary lands (with the watch binding on what is measurable, as already stated), or a named successor-design gap — so no implementer invents an undeclared field.
- **Disposition**: applied — the mid-drive spend home stated in §3.2: gist-body-informal until the sibling's Q-2 vocabulary lands (no invented structured field); the watch binds on measurable gists, the closing entry carries the structured fields

### FND-11 — UB-8's drill is bound in no acceptance row

- **Severity**: advisory (CAL-4)
- **Lens**: feasibility-completeness
- **Evidence**: design: §3.3's UB-8 flow (changed span) and the acceptance table A-1..A-7 — none names the seeded-confirmation drill — vs — prd:argus/UB-8's fit ("drill: a seeded confirmation produces both effects — incident entry + immediate demotion") and FND-3's acceptance condition, whose second arm ("binds UB-8's drill in acceptance") the revision leaves unmet. A stated flow without its drill is intent on the one zero-tolerance guardrail path (§3.3-of-basis).
- **Acceptance condition**: an acceptance row (extending A-6, or a new row) binds the UB-8 drill — a seeded `missed-breaking: yes` confirmation produces the incident entry and the immediate demotion, with Argus's backstop bump observed when the demotion is withheld.
- **Disposition**: applied — A-6 extended with the UB-8 drill: a seeded missed-breaking confirmation produces the incident entry and the immediate demotion, and the backstop bump is observed when the demotion is withheld

## Verdict history

| Round | Verdict | Date |
|---|---|---|
| 1 | request-changes | 2026-07-11 |
| 2 | approve-with-advisories | 2026-07-11 |

## Escalation brief

none

## Measurements

- **Rounds**: 2 (the cap; terminal)
- **Findings**: 7 blocking · 4 advisory (2 at round 1 · 2 at round 2)
- **Dispositions**: 9 applied · 0 declined · 0 withheld-as-violation · 2 pending (FND-10, FND-11 — the authoring flow's)
- **Closure**: 9/9 round-1 findings closed against their stated acceptance conditions (FND-3 closed in substance, its drill-binding residue re-scoped as FND-11)

## Overlay corrections

none
