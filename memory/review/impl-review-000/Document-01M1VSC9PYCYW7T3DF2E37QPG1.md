# Review: impl-review 000-base — review report

| Field | Value |
|---|---|
| **In one line** | Round 1: request-changes — 14 blocking + 14 advisory (the measurement layer blind to absence, M2's undefined denominator, the AP-13 verdict-staleness seam, disposition forgery, the lane-forcing vacuum, and D-9's dropped kinship among them); all 28 applied in revision; round 2: approve. |
| **Relevant-when** | auditing or consuming the review of prd:impl-review/000; asking why it was approved after a request-changes round; tuning prd-review calibration against real findings |
| **Subject** | prd:impl-review/000 |
| **Moment** | new-base |
| **Reviewer** | prd-review · `.pyxis/authoring/prd-review.md` (2026-07-16) |
| **Date(s)** | 2026-07-16 (rounds 1 and 2) |
| **Verdict** | approve |
| **Tracking** | — |
| **Template version** | review-report/v1 |

## Round 1

*Process note: four lenses ran as independent parallel reviewers over the stage context pack, the served basis, the feeding record, and the sibling PRDs (codegate, implement-now, prd-review). Independent convergence: the measurement-layer defects (M1 absence-blindness · M2 denominator · the `out` guardrail token) surfaced across three lenses; D-9's dropped kinship across two; the AP-13 staleness seam is this session's third appearance of that class. Findings are the deduplicated survivors.*

### FND-1 — §5.9 has no eval family for the correctness-critic lens

- **Severity**: blocking (CAL-3: FR-3's per-lens fit is unverifiable for one of the four lenses it binds; M3/DOD-2 would green with the critic never validated)
- **Lens**: contract-quality
- **Evidence**: FR-3 fit "the §5.9 catch evals demonstrate each lens catching its seeded family" (§5.1) — vs — §5.9 supplies EVAL-1/2/3 (spec/design/security) + cross-cutting 4/5/6; no seeded critic-defect family exists, though §4.4 illustrates critic ground ("a missing error-path test suggestion").
- **Acceptance condition**: a critic eval family (seeded logic/edge/error-path defect in a charter-faithful diff, threshold + negative example).
- **Disposition**: applied — EVAL-7 added (seeded correctness defect; deep-band leg for FR-10).

### FND-2 — three compound Must rows carry obligations their fits never measure

- **Severity**: blocking (CAL-3: each named obligation is a Must commitment with no deciding fit)
- **Lens**: contract-quality *(the adversarial-screen half independently: completeness)*
- **Evidence**: FR-3's "candidates surviving adversarial screening" (no screen fixture) · FR-8's "recorded with its reason" (EVT-3 types `reason` optional; no reason-required fixture) · FR-10's "the critic's calibration shall escalate" (fit tests catching, not escalation) — vs — Appendix A's fit-decides-pass/fail rule.
- **Acceptance condition**: each orphan obligation gets a fit — a screened-out-candidate fixture, a dispute-without-reason-is-invalid validation, a deep-band assertion that a seeded load-bearing critic finding ships blocking.
- **Disposition**: applied — FR-3 fit gains the screen fixture; EVT-3/FR-5 make decline-reason mandatory (schema-enforced); FR-10 fit gains the escalation leg via EVAL-7's deep leg.

### FND-3 — M1 is not computable from its declared feeds; §8 certifies the wrong source

- **Severity**: blocking (CAL-1)
- **Lens**: coherence
- **Evidence**: M1 "Feeds from: EVT-1, EVT-2" + §8 "M1←EVT-1/2" — vs — EVT-1 "Fires when a review round completes": a never-invoked proposal emits nothing, yet non-invocation is M1's one job (R3, §7.1 obituary 3); FR-13's harvest is the only mechanism that sees absence; EVT-5 carries no coverage/eval field for the ship rule's "M1 met AND M3 green".
- **Acceptance condition**: M1 feeds from the proposal-enumerating harvest (events as supplements); §8 corrected; EVT-5 gains coverage + eval-status fields.
- **Disposition**: applied — M1 feed rewritten to the FR-13 harvest over the tracker's proposal set; §8 cell corrected; EVT-5 gains `coverage_pct`, `m3_green`.

### FND-4 — M2's denominator is undefined exactly where the flows collide

- **Severity**: blocking (CAL-1)
- **Lens**: coherence *(independently: contract-quality small-n; completeness checkpoint population)*
- **Evidence**: M2 "≥70% of blocking findings accepted" — vs — F1 step 3 "the worker applies blocking findings warm" (no operator disposition) + §4.4 (operator merges without disposing the applied finding); 0/0 windows undefined; abandoned-proposal and undisposed-finding population unstated; the ship rule calls this "pre-registered."
- **Acceptance condition**: M2 numerator/denominator defined exactly, with a minimum-disposition floor and the zero-blocking case; the dogfood obligation that the operator disposes every blocking finding.
- **Disposition**: applied — M2 = operator-accepted ÷ operator-disposed blocking findings; a ≥N-disposition floor per checkpoint (else the window extends); zero-blocking window = held, not vacuous-pass; a dogfood-period operator-disposition obligation added (FR-15); EVT-3 distinguishes worker-apply from operator-accept.

### FND-5 — the nit guardrail counts a disposition value (`out`) its feed cannot represent

- **Severity**: blocking (CAL-1)
- **Lens**: coherence *(independently: upstream vocabulary)*
- **Evidence**: §3.3 "rejected/`out` dispositions per review" — vs — EVT-3 enum `applied·declined·accepted·rejected` (no `out`), FR-5 defers values to design; the harvest (FR-13) must compute the guardrail.
- **Acceptance condition**: align the vocabulary — add `out` with meaning, or strike it.
- **Disposition**: applied — `out` struck; the guardrail counts operator `rejected` dispositions.

### FND-6 — UB-5's post-adoption behavior mandates the indefinite stall its own guardrail forbids

- **Severity**: blocking (CAL-1)
- **Lens**: coherence
- **Evidence**: UB-5 "post-adoption the raise waits for the persisted report" (no bound) — vs — §3.3 guardrail 2 "never an indefinite stall"; AP-16 (bounded self-healing loops escalate).
- **Acceptance condition**: UB-5 gains a retry bound and a terminal path to the review-absent outcome.
- **Disposition**: applied — UB-5 bounds retries then falls to UB-1's recorded-absence via an alternate signal; post-adoption a persistent write failure routes HUMAN, never waits.

### FND-7 — the lane-forcing job has no outbound carrier

- **Severity**: blocking (CAL-1)
- **Lens**: coherence
- **Evidence**: FR-7 "may route a proposal to the HUMAN lane"; F3; UB-1 fit "routes HUMAN" — vs — §5.5 lists the lane only inbound (band selection); no outbound verdict→lane interface, owner, or failure mode; §6.2 has no lane consumer.
- **Acceptance condition**: an outbound interface row (downgrade signal → the lane's GATE-4 downgrade-only slot) with owner and failure mode.
- **Disposition**: applied — §5.5 gains the outbound row (the verdict payload is what the lane's existing downgrade-only classifier reads; no lane-side redesign — NR-4 added protecting it); §6.2 notes the read.

### FND-8 — D-3's findings-use half has no requirement carrier

- **Severity**: blocking (CAL-1)
- **Lens**: coherence
- **Evidence**: D-3 "the worker still applies blocking-class findings … advisory bounds the verdict's authority, not the findings' use" — vs — FR-11 obliges nothing; FR-8 triggers only on dispute; the apply duty lives only in §4 prose.
- **Acceptance condition**: a requirement obliging apply-or-dispute-with-reason of every blocking finding before PR-raise (a host protocol duty, distinct from verdict authority).
- **Disposition**: applied — FR-15 (the disposition obligation): during advisory and after, every blocking finding carries a recorded worker disposition before raise; the duty is the host's, traced to the amendment wave.

### FND-9 — D-9's grammar-kinship clause is dropped from the product bars

- **Severity**: blocking (CAL-2)
- **Lens**: upstream-alignment *(independently: coherence)*
- **Evidence**: D-9 "a shared finding-grammar core with codegate's finding schema + review-only fields" — vs — FR-5 states no relation; Q2's fixed bars omit it; NR-1 demands zero diff to codegate's schema (frozen as implement-now's joint contract) — the un-carried constraint intersects both.
- **Acceptance condition**: carry the kinship as a product bar (read-only reuse of codegate's finding core + review-only fields; no codegate change).
- **Disposition**: applied — FR-5 gains the kinship clause; Q2's framing cites it; NR-1's read-only relation made explicit.

### FND-10 — the verdict is never bound to the revision that ships (AP-13 seam)

- **Severity**: blocking (CAL-4: load-bearing for §8 flow coverage and the post-adoption ship rule)
- **Lens**: completeness-critic
- **Evidence**: F1 step 4 raises with no check that head = last-reviewed revision; a worker can apply round-1 findings, skip round 2, edit post-approve — the verdict rides stale into stage 9 — vs — AP-13 ("a gate's PASS certifies the exact revision that ships"), AP-26. This session's third instance of the class.
- **Acceptance condition**: the report records the reviewed revision; at raise, head ≠ reviewed ⇒ stale — recorded visibly (advisory) / re-review-or-block (post-adoption); round 2 forced by any post-review change.
- **Disposition**: applied — UB-6 added (review-staleness: the verdict binds the reviewed revision; a post-review change marks it `review-stale`, non-gating in advisory, HUMAN-routing/re-review post-adoption); FR-1's round-2 trigger reworded to "any post-review change," making round 2 forced not optional.

### FND-11 — no honest-coverage rule for a diff exceeding the band's budget/context

- **Severity**: blocking (CAL-4: silent truncation → approve certifies unread code, on the fat-tail population §2.4 cites)
- **Lens**: completeness-critic
- **Evidence**: bands are risk-derived only (FR-9); NFR-1 forbids silent overrun but not under-reading; no size-skip class — vs — §2.4's own 525-PR fat-tail evidence guarantees oversized diffs.
- **Acceptance condition**: a UB row — input exceeding band capacity ⇒ a recorded outcome (size-escalation / recorded skip / review-absent), never silent truncation; the report attests input coverage; a §5.9 truncation-temptation leg.
- **Disposition**: applied — UB-7 (oversize: recorded size-escalation to HUMAN, never truncation); the report's machine payload attests full-diff coverage; EVAL-8 pins the truncation temptation.

### FND-12 — disposition records carry no actor-identity binding; a worker can forge operator accepts

- **Severity**: blocking (CAL-4: poisons M2 and the adoption ground truth — the metric gamed at its source)
- **Lens**: completeness-critic
- **Evidence**: EVT-3 `actor: worker·operator` unauthenticated; the GitHub marker block is one bot comment editable by any write-capable worker; the harvest validates schema, not attribution — vs — the vision's "bot and human identities never confusable"; ABUSE-1/2/3 model this adversary.
- **Acceptance condition**: an ABUSE row + an integrity rule — operator-class dispositions valid only when attributable to the operator's medium identity; the harvest drops unattributable ones; a forgery fixture.
- **Disposition**: applied — ABUSE-5 (disposition forgery) + FR-16 (dispositions are attribution-bound: operator-class dispositions derive from the operator's own medium act, never a bot-writable field; the harvest excludes unattributable ones from M2).

### FND-13 — NR-2/DOD-6 freeze an event-identity bind on sibling timing this feature doesn't control

- **Severity**: blocking (CAL-2)
- **Lens**: upstream-alignment
- **Evidence**: NR-2 fit "confined to the same amendment set as the VerificationGate seam"; DOD-6 (blocking) — vs — that wave is unexecuted and codegate-owned (implement-now header "As of: 000"); codegate's epic is in flight now. The exact premise-class that went stale once (codegate's own NR-2, implement-now merged un-edited within a day). If codegate integrates before this feature reaches Phase B, the fit is unsatisfiable; the scheduling dependency is recorded nowhere.
- **Acceptance condition**: rebind NR-2/DOD-6 to the amendment *class* (one additive amendment adding only the invocation slot, drive-side untouched; joins the VerificationGate wave when phase alignment holds, else a separate same-class amendment), or record the scheduling decision with its owner.
- **Disposition**: applied — NR-2 rebound to the class; DOD-6 reworded to the class; §6.3 R5 records the timing race and its dogfood-window consequence.

### FND-14 — the program's escalate semantics (brief + fail-closed park) are not carried

- **Severity**: blocking (CAL-2)
- **Lens**: upstream-alignment
- **Evidence**: FR-7 lists `escalate`; FR-8 "a declined blocking finding shall escalate" — and that is the whole surface; "brief"/"park" appear nowhere in §5 — vs — the record's F-2 ("escalate = brief + fail-closed park") and prd-review FR-11's PRD-level definition.
- **Acceptance condition**: define escalate's output and the proposal's held state per period, or record the per-class deviation with its reason.
- **Disposition**: applied — FR-17 (escalate = a decision brief recorded on the proposal, addressed to stage 9; advisory: non-gating; post-adoption: HUMAN-routed, raise held), with a §7.3 note that for this class stage 9 *is* the park — the brief rides the proposal rather than a separate tracker item.

### FND-15 — stochastic eval execution has no protocol (runs, flake rule, pinning, overlap judge)

- **Severity**: advisory (CAL-3)
- **Lens**: contract-quality
- **Evidence**: EVAL-1 "100% caught" gates DOD-2 (blocking) — vs — §5.9 states no runs-per-fixture, pass rule, decode pinning, or flake disposition (rerun-until-green silently voids the gate); EVAL-4/5 need unstated overlap/invariance judges; EVT-4 lacks the provenance NFR-3 demands for reviews.
- **Acceptance condition**: a §5.9 execution-protocol preamble (runs + pass rule, pinned provenance on EVT-4, scoring method, flake-is-a-defect); family removal/threshold-relaxation governed as data PRs.
- **Disposition**: applied — §5.9 preamble added; EVT-4 gains provenance; the flake-is-defect and data-PR-governance rules stated.

### FND-16 — three §5.9 threshold cells fail the definedness bar

- **Severity**: advisory (CAL-3)
- **Lens**: contract-quality
- **Evidence**: EVAL-1 "zero sev-1 misses" (sev-1 undefined); EVAL-3 "documents the miss honestly" (banned adverb); EVAL-6 "nit guardrail's threshold" (dispositions don't exist on goldens) — vs — Appendix A blacklist + the fit-decides rule.
- **Acceptance condition**: "sev-1"→"blocking-class"; restate EVAL-3 observably; give EVAL-6 a fixture-unit threshold.
- **Disposition**: applied — all three reworded.

### FND-17 — EARS grammar sweep (pronoun subject, non-EARS rider, compounds)

- **Severity**: advisory (CAL-3)
- **Lens**: contract-quality
- **Evidence**: FR-5 subject "it"; FR-12 clause 2 shall-less; FR-1/2/7/11 multi-shall — vs — Appendix A (named subject, singular, EARS patterns).
- **Acceptance condition**: reword FR-5's subject; give FR-12's rider a shall form; split the load-bearing compounds.
- **Disposition**: applied — FR-5/FR-12 reworded; the compounds carrying orphan obligations split under FND-2; the fit-covered compounds left (draft economy).

### FND-18 — §8's F2 flow-coverage cell cites an instrumentation event, not a §5.1–5.2 row

- **Severity**: advisory (CAL-1)
- **Lens**: coherence
- **Evidence**: §8 "F2→FR-6/13+EVT-3" — vs — EVT-3 lives in §5.7, outside the sections the check names; no §5.2 row covers disposition-write failure.
- **Acceptance condition**: cite F2's failure coverage from §5.2 (UB-5, dispositions-as-report-writes) or add a UB row.
- **Disposition**: applied — §8 F2 cell re-cited to UB-5; UB-5's scope clarified to cover disposition writes.

### FND-19 — M4's "stage-9 capture lines" names a mechanism defined nowhere

- **Severity**: advisory (CAL-1: G4 qualitative, M4 has no gate)
- **Lens**: coherence
- **Evidence**: M4 "Feeds from: stage-9 capture lines" — vs — no §5.7 event, interface, or owner; the ledger is future/out.
- **Acceptance condition**: name the mechanism or downgrade M4 to "measurement plan owed at Phase B (Q1)".
- **Disposition**: applied — M4 reworded to the read-to-rule capture-line convention with the Phase-B/Q1 measurement-plan owner named.

### FND-20 — host/worker death mid-protocol has no resume/orphan semantics

- **Severity**: advisory (CAL-4)
- **Lens**: completeness-critic
- **Evidence**: NFR-4 (state survives) — vs — no rule for who re-invokes round 2 when the worker dies; resume-at-round-2 vs restart; double-invocation idempotence; UB-5 backfill ownership after the session ends.
- **Acceptance condition**: a clause — re-invocation is an idempotent resume keyed off the persisted report (rounds authoritative there); the backfill/orphan reaper named.
- **Disposition**: applied — UB-8 (re-invocation resumes from the persisted report's round state, idempotent; an orphaned in-flight review is reaped/re-invoked by the host's janitor, traced to the amendment wave).

### FND-21 — worker self-certifies "applied" with no round-2 verification requirement

- **Severity**: advisory (CAL-4; largely closed by FND-10)
- **Lens**: completeness-critic
- **Evidence**: F1 step 3 + EVT-3 "applied" — vs — round 2 optional (FR-1 parenthetical); nothing states an "applied" blocking finding is verified only by round-2 re-review.
- **Acceptance condition**: "applied" on a blocking finding renders verified only after round 2 confirms closure; else the report labels it unverified-claim.
- **Disposition**: applied — folded into FR-1's forced-round-2 (FND-10) + a report labeling rule: an unverified "applied" cannot upgrade the verdict.

### FND-22 — operator disposition fatigue (rubber-stamp accepts) inflates M2

- **Severity**: advisory (CAL-4)
- **Lens**: completeness-critic
- **Evidence**: §2.1's own thesis (the human gate collapses under load) predicts the calibration oracle bulk-accepting — vs — no guardrail/ABUSE/assumption names disposition *quality* (A1 covers friction, not honesty).
- **Acceptance condition**: name it — an assumption + a cheap integrity signal (disposition timing/burst stratification at the checkpoint).
- **Disposition**: applied — A2 (disposition-quality assumption) + EVT-5 gains a disposition-engagement signal (median dwell / burst flag) surfaced at the checkpoint.

### FND-23 — the adversarial screen is unspecified, unverified, and unobserved

- **Severity**: advisory (CAL-4)
- **Lens**: completeness-critic
- **Evidence**: FR-3's screen — vs — no fit tests it discriminates (a no-op screen passes); no candidate/kill data, so a miss can't be localized (never-generated vs killed-in-screen); the calibration loop iterates blind.
- **Acceptance condition**: a screen eval leg (seeded plausible-but-wrong candidates must die) + bounded candidate/kill counters on EVT-1.
- **Disposition**: applied — the FR-3 screen fixture (FND-2) doubles as the discrimination leg; EVT-1 gains `candidates`/`killed` counts.

### FND-24 — no live false-negative capture path (the eval-growth loop's unstated input)

- **Severity**: advisory (CAL-4)
- **Lens**: completeness-critic
- **Evidence**: §5.9 "families grow as evasions are found"; R2 "every found evasion becomes an eval family" — vs — an operator/stage-10 catching a missed defect on a reviewed proposal lands nowhere (no EVT, disposition class, or metric); live recall unmeasured.
- **Acceptance condition**: a miss-capture path tying an operator-found defect to the review that missed it, feeding the eval-family data PRs.
- **Disposition**: applied — EVT-6 (review_miss: an escaped defect tagged to its review) + a §3.2 note that live recall is the miss corpus feeding §5.9 growth.

### FND-25 — recurring-path steady-state costs unstated (AP-19)

- **Severity**: advisory (CAL-4)
- **Lens**: completeness-critic
- **Evidence**: NFR-1/2 (per-review) — vs — round-2 budget inclusion, fleet-aggregate concurrent spend, harvest recompute at scale, eval-suite growth — none stated.
- **Acceptance condition**: state the four as data/bounds (round-2 inclusion; a fleet ceiling or explicit non-goal; harvest incrementality note; eval-suite budget).
- **Disposition**: applied — NFR-1 states round-2 rides the band budget; NFR-5 (fleet spend visibility, no per-tick ceiling in v1 — stated); a harvest-incrementality note; §5.9 eval-suite budget-as-data.

### FND-26 — "fresh contexts" (FR-2) is unverifiable by the stated inspection method

- **Severity**: advisory (CAL-4)
- **Lens**: completeness-critic
- **Evidence**: FR-2 fit inspects the contract's *shape*; a host running lenses inline in its own session passes both checks while violating the requirement — vs — §8's every-Must-verifiable claim.
- **Acceptance condition**: a behavioral leg — provenance attests execution-context identity, or in-session invocation is structurally refused at the seam.
- **Disposition**: applied — NFR-3 provenance gains a fresh-instance identity attestation; FR-2's fit gains the runtime leg.

### FND-27 — a present-but-defective charter has no stated recourse

- **Severity**: advisory (CAL-4)
- **Lens**: completeness-critic
- **Evidence**: FR-14 handles absence only — vs — an ambiguous/self-contradictory/design-contradicting charter leaves spec-fidelity findings unadjudicable and pollutes M2; no stage-8-tail analog of stage-6's bounce.
- **Acceptance condition**: escalate with a named upstream defect class, withholding findings against the defective span.
- **Disposition**: applied — FR-14 extended: a charter defect material to review escalates (FR-17) with an upstream-defect class, findings withheld against the defective span.

### FND-28 — the record's glossary terms (proposal boundary, impl-review) aren't landed, and "proposal" collides

- **Severity**: advisory (CAL-2 downgraded: accompanying-artifact, not contract-body; no Must unverifiable)
- **Lens**: upstream-alignment
- **Evidence**: T-1/T-2 defined in the record; the PRD leans on "proposal boundary" — vs — the glossary carries no impl-review section, and implement-now pins "proposal = the PR," which pre-raise usage stretches.
- **Acceptance condition**: land the two glossary entries in the introducing PR, or state where/when in §6.6.
- **Disposition**: applied as deferral-with-reason — §6.6 Docs row names the two glossary entries as owed in the introducing PR; not landed in this doc-review pass because the glossary file carries a concurrent session's uncommitted WIP (the recurring shared-file hazard); recorded so it isn't lost.

## Round 2

*Scope-locked re-review of the revision: each of FND-1..28's acceptance conditions verified met at its anchor. The new rows (FR-15/16/17, UB-6/7/8, ABUSE-5, EVAL-7/8, EVT-6, NR-4, A2) carry complete EARS statements, fit criteria, risk tiers, and traces; the measurement layer is now internally consistent (M1←harvest, M2 denominator defined with the disposition-attribution and floor rules, the guardrail computable from EVT-3); the AP-13 seam is closed by UB-6 + FR-1's forced round 2; NR-2's class-rebind removes the timing-race dependency. No finding against unchanged content; no new findings on changed spans.*

## Verdict history

| Round | Verdict | Date |
|---|---|---|
| 1 | request-changes | 2026-07-16 |
| 2 | approve | 2026-07-16 |

## Escalation brief

none

## Measurements

- **Rounds**: 2
- **Findings**: 14 blocking · 14 advisory
- **Dispositions**: 28 applied · 0 declined · 0 withheld-as-violation *(FND-28 applied as a recorded deferral with reason, not a decline)*

## Overlay corrections

none
