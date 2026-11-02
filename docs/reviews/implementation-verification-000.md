# Review: implementation-verification 000-base — review report

| Field | Value |
|---|---|
| **In one line** | Round 1: request-changes — 10 blocking + 19 advisory findings (the FR-11 test-only contradiction, undefined no-baseline rung-2 semantics, a sibling-contract conflict on the local-gate slot, unbound verdict identity, and uninstrumentable guardrails among them); all 29 applied in revision; round 2: approve. |
| **Relevant-when** | auditing or consuming the review of prd:implementation-verification/000; asking why it was approved after a request-changes round; tuning prd-review calibration against real findings |
| **Subject** | prd:implementation-verification/000 |
| **Moment** | new-base |
| **Reviewer** | prd-review · `.pyxis/authoring/prd-review.md` (2026-07-16) |
| **Date(s)** | 2026-07-16 (rounds 1 and 2) |
| **Verdict** | approve |
| **Tracking** | agenticlab-ai/pyxis#356 |
| **Template version** | review-report/v1 |

## Round 1

*Process note: the four lenses ran as independent parallel reviewers over the folded subject and the stage context pack; the completeness-critic lens was re-executed inline after two infrastructure failures (server 529s). Candidates crossed adversarial screening before shipping; independent convergence occurred on FND-1 and FND-3 (contract-quality and coherence each raised them unprompted).*

### FND-1 — FR-11 mandates a falsification failure on the test-only diffs its own fit requires to pass

- **Severity**: blocking (CAL-1)
- **Lens**: coherence *(independently also raised by contract-quality)*
- **Evidence**: "When an implementation diff adds or modifies test files, rung 3 shall verify the new or changed tests fail with the diff's non-test half reverted; if they pass without the implementation, the rung shall fail" (§5.1 FR-11) — vs — its own fit: "the legitimate-diff fixtures (test-only change, …) pass without a falsification finding". A test-only diff has an empty non-test half; the revert is a no-op, the tests pass "without the implementation", and the letter of the rule fires. The escape reading ("implementation diff" = diff containing implementation) breaks FR-8, which uses the same undefined term for any proposal diff.
- **Acceptance condition**: scope FR-11's trigger to diffs with a non-empty non-test half; state the test-only disposition explicitly; define the diff term once so FR-8's trigger provably covers test-only and machinery-only diffs.
- **Disposition**: applied — FR-11 rescoped; new FR-18 records not-applicable for unfalsifiable diffs; "proposal diff / non-test half" defined in §5's terms note; FR-8, FR-12, ABUSE-3 migrated to the defined term.

### FND-2 — FR-10 states rung 2's semantics only for the baseline-bound state; the healthy-repo default is unstated and unverified

- **Severity**: blocking (CAL-3: makes a Must row unverifiable)
- **Lens**: contract-quality
- **Evidence**: "execute the test verb; while a baseline is bound … evaluate as no-new-failures" (§5.1 FR-10), all three fit legs baseline-bound — vs — Appendix A (complete · verifiable); §4.4's common case and M1's dogfood path run the unbound state, where the row required only "execute the test verb" with no failure semantics.
- **Acceptance condition**: state the default (fail on any test failure) in the statement and add healthy-repo fit legs.
- **Disposition**: applied — FR-10 rewritten with the default-first form; clean-fixture and seeded-failure fit legs added.

### FND-3 — FR-12's three-class Must floor is fit-verified for only one class; M2's seed wording disagrees with the fit

- **Severity**: blocking (CAL-1)
- **Lens**: coherence *(independently also raised by contract-quality)*
- **Evidence**: "deletes a test, adds a skip/exclusion marker, or removes assertions" (§5.1 FR-12) — vs — fit verifying only "a deleted failing test"; M2 naming seed 2 "deleted/weakened test" (§3.2). Two of three Must trigger classes — ABUSE-2's "skips, or hollows out" — had no verifying case.
- **Acceptance condition**: extend the fit and catch suite with skip-marker and assertion-removal cases; align M2's seed naming.
- **Disposition**: applied — FR-12 fit covers all three classes; M2 renamed to seed families (tautology · silenced test [deletion, skip-marker, assertion-removal] · machinery); EVT-4 gains seed_family+case; ABUSE-2 verify cell aligned.

### FND-4 — NR-2's "no amendment required" contradicts implement-now's own contract, and R4 cites a row in its §7.2 that does not exist

- **Severity**: blocking (CAL-2)
- **Lens**: upstream-alignment
- **Evidence**: "holds without amendment" (§5.1 NR-2), "the definition and executor of the 'genuinely-green' bar" (header), "both PRDs cite (§5.5, its §7.2)" (§6.3 R4) — vs — implement-now FR-8 (its local gate "shall run the same deterministic checks required by CI" — a mirror, which the ladder is not: docs-gate is a required context no ladder verb binds; AP-11), implement-now M2 (genuinely-green already defined there), and its §7.2 (no joint-contract row). The record's D-2 scoped "no mid-review amendment" to drive-side caps only.
- **Acceptance condition**: reword NR-2 to protect the drive-side only and admit the local-gate slot changes via a draft-phase edit to the sibling; reconcile genuinely-green to one owner; correct R4; address the mirror property.
- **Disposition**: applied — NR-2 rewritten (drive-side protected; integration = draft-phase edit to the sibling's local-gate rows); §5.5 gains the genuinely-green ownership row (defined in implement-now, extended at integration); header cells softened; R4 corrected; WR-7 (custom check bindings — ladder-green complements, never replaces, required CI until then) added citing this finding; FAQ row added.

### FND-5 — the verdict artifact never records what it certifies (AP-13 / incident I2 class)

- **Severity**: blocking (CAL-2)
- **Lens**: upstream-alignment
- **Evidence**: FR-4's verdict enumerates rung results but no revision identity; §5.4 sells it as "rides the proposal as evidence"; EVT-1 carries no commit — vs — AP-13 ("a gate's PASS certifies the exact revision that ships"), incident I2, argus-unattended-drive FR-5 (acts bound "to the exact head revision"). A fix-and-push after the last ladder run attaches a green verdict for a different tree; M4's same-commit fit was unverifiable from the artifacts.
- **Acceptance condition**: verdicts record workspace revision, diff base, manifest hash, baseline hash, harness version; a stale-revision verdict is treated as absent; mismatch fixture.
- **Disposition**: applied — FR-4 extended with the identity clause + mismatch fixture; §5.4 verdict integrity cell updated; §5.5 consumer rule added; EVT-1 gains head_sha/diff_base/manifest_hash/baseline_hash.

### FND-6 — no fail-closed rule for a misparsed invocation (incident I3's paid lesson)

- **Severity**: blocking (CAL-2)
- **Lens**: upstream-alignment
- **Evidence**: §5.2 UB-1..6 cover verbs, manifest, revert, baseline, interruption — no invocation-input row — vs — incident I3 ("consequential CLIs fail closed on misparse: any flag-like positional is a usage refusal before any action"). The diff base scopes FR-8's machinery detection and FR-11's revert; a silently mis-taken base yields a confident wrong-scope green.
- **Acceptance condition**: a UB row refusing invalid invocations before any rung executes, with malformed-invocation fixtures.
- **Disposition**: applied — UB-7 added (usage refusal, configuration-error only, zero verbs executed; I3 named).

### FND-7 — M3's gating hardness has three inconsistent answers (ship rule vs Phase B exit vs DOD-4)

- **Severity**: blocking (CAL-1)
- **Lens**: coherence
- **Evidence**: "M3 missed while M1/M2 hold → discuss" (§3.3) — vs — "Exit criterion: M1 + M3 met" (§6.1 Phase B) and DOD-4 "Blocking? yes" (§5.8).
- **Acceptance condition**: one status, cross-referenced.
- **Disposition**: applied — the ship rule governs: DOD-4 is non-blocking with the §3.3 reason stated in-row; Phase B's exit reads "M3 measured and dispositioned per §3.3".

### FND-8 — EVT-1 carries no task/workspace correlation key, so M1 and M3 are not computable from the instrumentation §8 certifies

- **Severity**: blocking (CAL-1)
- **Lens**: coherence
- **Evidence**: EVT-1's properties (run_id, mode, verdict, per-rung, verb_runs, wall_ms) join to nothing — vs — M1 denominated in tasks and M3 pairing "the same workspace's" runs; §8's "M1/M3←EVT-1 ✓".
- **Acceptance condition**: correlation properties on EVT-1 (preserving FR-15) or an explicit measurement plan naming the host-side join.
- **Disposition**: applied — EVT-1 gains workspace_id + opaque host-supplied invocation_context (+ FND-5's identity fields); M1's feed names the host's dogfood run record as the join; M3 pairs by workspace_id; §8 row updated to the stated plan.

### FND-9 — both guardrails count "unresolved" items the tracking plan cannot observe; stall dispositions don't fit EVT-3's enum

- **Severity**: blocking (CAL-1)
- **Lens**: coherence
- **Evidence**: "0 unresolved at the dogfood checkpoint" (§3.3, both rows) — vs — EVT-3 firing only when "the operator dispositions a disputed finding" with verdict ∈ {upheld, wrong_block}: an undispositioned dispute emits nothing (absence certifies the count), and guardrail 2's dispositions fit neither value.
- **Acceptance condition**: record dispute-opening as well as disposition; cover stall dispositions; re-cite §8/DOD-7 to a producible source.
- **Disposition**: applied — EVT-3 renamed disposition_record with status: opened·dispositioned and verdict extended (upheld · wrong_block · stall_defect_fixed · stall_upheld_error); §3.3 and DOD-7 cite both statuses.

### FND-10 — the wrong-block breach action depends on a per-check advisory mode no requirement provides, colliding with FR-17's definition of green

- **Severity**: blocking (CAL-1)
- **Lens**: coherence
- **Evidence**: "pause the offending check to advisory in the host's configuration" (§3.3) — vs — no FR grants check-level advisory execution; the manifest holds no such field (§5.4); FR-17 forbids green with a failed rung; M1/DOD-3 require "through the harness".
- **Acceptance condition**: define the pause in-system and state its M1 accounting.
- **Disposition**: applied — the action is a human-lane manifest change demoting the check to skip-with-record (green-degraded under FR-6/FR-17); tasks under demotion count toward M1 as green-degraded; the demotion rides the disposition record; FAQ and R1 aligned.

### FND-11 — NFR-1's requirement sentence is banned-class subjective language

- **Severity**: advisory (CAL-3)
- **Lens**: contract-quality
- **Evidence**: "quickly enough to serve as in-loop feedback" (§5.6 NFR-1) — vs — Appendix A's banned-vagueness blacklist (subjective language).
- **Acceptance condition**: move the bound into the statement.
- **Disposition**: applied — "Rung 0 shall complete within 10 s at p95 on the reference (dogfood) repo."

### FND-12 — FR-17 splices two requirements and phrases the safety half as a negative

- **Severity**: advisory (CAL-3)
- **Lens**: contract-quality
- **Evidence**: "…green-degraded if any skip-with-record occurred; a green or green-degraded verdict shall never coexist with a failed or errored rung" (§5.1 FR-17) — vs — Appendix A: singular; state the positive.
- **Acceptance condition**: split the invariant into its own positively-stated row.
- **Disposition**: applied — FR-17 keeps classification; new FR-19 states "While any rung result is fail or error, the harness shall report the overall verdict as red or error respectively" and inherits the schema-invariant fit.

### FND-13 — NFR-3 bakes an implementation choice ("Go binary") into a requirement

- **Severity**: advisory (CAL-3)
- **Lens**: contract-quality
- **Evidence**: "single static Go binary" (§5.6 NFR-3) — vs — template: requirements are implementation-free; the repo's language policy already binds Go at design level.
- **Acceptance condition**: restate the observable property.
- **Disposition**: applied — "single self-contained executable with no runtime dependencies beyond the repo's own declared verbs."

### FND-14 — UB-2's fit hinges on an undefined "grace" quantity

- **Severity**: advisory (CAL-3)
- **Lens**: contract-quality
- **Evidence**: "terminates within bound + grace" (§5.2 UB-2 fit), "grace" defined nowhere — vs — Appendix A: fit criteria decide pass/fail.
- **Acceptance condition**: pin the value or its home.
- **Disposition**: applied — "within its bound plus the runner's declared termination grace (a constant the verdict schema records)."

### FND-15 — ABUSE-5's "absence is visible" is false on the sibling AUTO lane

- **Severity**: advisory (CAL-2: the AUTO lane is shadow-only until an operator activation act; the defect is an overstated mitigation cell, not a live v1 hole)
- **Lens**: upstream-alignment
- **Evidence**: "a proposal's verdict artifact is its evidence — absence is visible" (§5.3 ABUSE-5) — vs — argus-unattended-drive FR-5 (AUTO merges on required-checks-green; verdict presence not among them).
- **Acceptance condition**: scope the visibility claim to the human lane and name the interim guard.
- **Disposition**: applied — cell rewritten: visible to the human lane; until WR-5, AUTO class ratification must exclude harness-hosted task proposals or verdict presence joins the floor.

### FND-16 — NFR-6 under-constrains against AP-27 (derived, never authored aggregates)

- **Severity**: advisory (CAL-2: the row permits a compliant reading; the design stage's mandatory principles section is a second net)
- **Lens**: upstream-alignment
- **Evidence**: "a new check unit plus its registration" (§5.6 NFR-6) — vs — AP-27 and its founding incident (sibling tasks each editing shared central lists).
- **Acceptance condition**: require extension by pure addition with a derived aggregate, cite AP-27, fit inspects the demonstration diffs.
- **Disposition**: applied — NFR-6 rewritten (self-registration/scan/build; no edit to any shared list; AP-27 cited; fit inspects for central-aggregate touches).

### FND-17 — Phase A's exit cites DOD-1 wholesale though two fits cannot run until Phases B/C

- **Severity**: advisory (CAL-1: phases' actual landing points are unambiguous from §3.2/§5.8; the fix is scoping, not a decision change)
- **Lens**: coherence
- **Evidence**: "Exit criterion: … (DOD-1/2/6/9)" (§6.1 Phase A) — vs — FR-15's fit (Phase C) and FR-3/FR-7's dogfood legs (Phase B): Phase A as written was formally unexitable.
- **Acceptance condition**: scope Phase A's exit to the fixture-verifiable subset.
- **Disposition**: applied — Phase A exits on the fixture-verifiable clauses of DOD-1 plus DOD-2/6/9, with the B/C-bound legs named.

### FND-18 — FR-4's trigger covers only completed full-ladder runs; unreached rungs have no named representation

- **Severity**: advisory (CAL-1: FR-4's fit already binds every-run behavior operationally)
- **Lens**: coherence
- **Evidence**: "When a full-ladder run completes" (§5.1 FR-4) — vs — FR-1 stopping every failing run early, §4.3's red state, EVT-1 firing on every invocation.
- **Acceptance condition**: widen the trigger to every completed invocation; name the unreached-rung representation.
- **Disposition**: applied — "When an invocation completes — at whatever rung it stopped —"; `skip:not-reached` added to the enumeration.

### FND-19 — G4's only measurement anchor (§6.7) records neither its observable nor its window

- **Severity**: advisory (CAL-1: G4 is qualitative and non-gating)
- **Lens**: coherence
- **Evidence**: "Measured by … (observed, §6.7)" (§3.1 G4) — vs — §6.7's evidence list omitting verdict-consultation and firing post-dogfood only.
- **Acceptance condition**: add G4's observable to §6.7's recorded evidence and align the window.
- **Disposition**: applied — §6.7 records "G4's observed verdict-consultation during dogfood and post-launch reviews"; G4's cell names both windows.

### FND-20 — §5.4 cites FR-10 as enforcing a "shrink-only ratchet" it does not enforce

- **Severity**: advisory (CAL-1: the residual is honestly declared in ABUSE-3; the defect is one integrity-cell citation)
- **Lens**: coherence
- **Evidence**: "Integrity rules: shrink-only ratchet (FR-10)" (§5.4) — vs — FR-10's text (evaluates and records; growth-rejection lives in FR-8 for proposal diffs and human-lane convention outside them).
- **Acceptance condition**: correct the cell to cite what holds.
- **Disposition**: applied — cell now cites FR-10 (evaluation), FR-8 (growth flagged in proposal diffs), and the human-lane convention until WR-5.

### FND-21 — FR-15's "sole outputs", EVT-1's log source, and NFR-2's timestamps clause cannot all hold as written

- **Severity**: advisory (CAL-1: every resolution preserves stated behavior)
- **Lens**: coherence
- **Evidence**: "exit code and verdict artifact as the sole outputs" (§5.1 FR-15) — vs — EVT sources "harness structured log + verdict artifact" (§5.7) and NFR-2's "modulo timestamps" fit with durations in the payload.
- **Acceptance condition**: name the run log as an output and widen the segregation clause to timing fields.
- **Disposition**: applied — FR-15 lists exit code, verdict artifact, and structured run log; NFR-2 segregates "timing fields — timestamps and durations".

### FND-22 — seventeen Must FRs with no intra-class rank despite the template's cut-order demand

- **Severity**: advisory (CAL-1: a partial cut order exists via §3.3/§6.1)
- **Lens**: coherence
- **Evidence**: every §5.1 Priority cell "Must" — vs — the template's rank-within-class rule.
- **Acceptance condition**: rank, or declare the set irreducible with the sanctioned cut named.
- **Disposition**: applied — declaration under §5.1: the set is the irreducible bet (D-9); the only sanctioned mid-build cut is §3.3's M3 branch.

### FND-23 — concurrent invocations on one workspace are unhandled

- **Severity**: advisory (CAL-4)
- **Lens**: completeness-critic
- **Evidence**: §5.2 covered verbs, config, interruption — nothing for a second invocation contending the same workspace and its incremental state.
- **Acceptance condition**: a UB row refusing concurrent same-workspace invocation.
- **Disposition**: applied — UB-8 (busy refusal; no interleaved rung execution or shared incremental state; contention fixture).

### FND-24 — a manifest or baseline change between runs silently met FR-3's "unchanged manifest" precondition with no stated behavior

- **Severity**: advisory (CAL-4)
- **Lens**: completeness-critic
- **Evidence**: "re-invoked … with an unchanged manifest" (§5.1 FR-3) — vs — no row stating what a *changed* manifest does to incremental state.
- **Acceptance condition**: state invalidation.
- **Disposition**: applied — FR-3 now invalidates incremental state on manifest/baseline change (next run full), with a changed-manifest fixture leg.

### FND-25 — unparseable test output under a bound baseline had no stated disposition, though no-new-failures needs failure identities

- **Severity**: advisory (CAL-4)
- **Lens**: completeness-critic
- **Evidence**: FR-10's evaluation presumes failure identities; §5.4's ecosystem table listed only "bindings and test globs" — vs — a test verb whose output the parser cannot read.
- **Acceptance condition**: fail closed on unparseable output with a baseline bound; name failure-identity parsers as ecosystem-table content.
- **Disposition**: applied — UB-9 (rung-2 error, never exit-code fallback, with the no-baseline path following FR-10); §5.4 ecosystem row gains failure-identity parsers.

### FND-26 — the zero-test-change diff's vacuous rung-3 pass was unstated, and the "code with no tests sails green in v1" residual unnamed

- **Severity**: advisory (CAL-4)
- **Lens**: completeness-critic
- **Evidence**: FR-11/FR-12 trigger only on test-file changes — a code-only diff passes rung 3 with nothing recorded, and no row admitted that v1 does not require new code to carry tests (deferred with WR-1's coverage rung).
- **Acceptance condition**: state the disposition and name the residual where the deferral lives.
- **Disposition**: applied — FR-18's code-only fit leg records not-applicable; WR-1's revisit cell names the residual explicitly.

### FND-27 — falsification's structural residual (a test that detects the revert and fails conditionally) was unowned

- **Severity**: advisory (CAL-4)
- **Lens**: completeness-critic
- **Evidence**: ABUSE-1's mitigation cited FR-11 without bounding what it proves — discrimination of presence/absence, not assertion quality.
- **Acceptance condition**: name the residual and its future owner.
- **Disposition**: applied — ABUSE-1's cell states the residual and points at mutation-depth checks (WR-3).

### FND-28 — generated-code sync is absent from the contract entirely — neither committed nor refused

- **Severity**: advisory (CAL-4)
- **Lens**: completeness-critic
- **Evidence**: no rung, NG, or WR row addressed hiding failures via stale generated code, though the dogfood repo itself carries generated projections.
- **Acceptance condition**: waiting-room it with a trigger.
- **Disposition**: applied — WR-8 (generated-code sync rung; revisit at a codegen repo's onboarding or the technical design), citing this finding.

### FND-29 — F3 binds a baseline whose provenance is never stated (who captures it, how, in v1)

- **Severity**: advisory (CAL-4)
- **Lens**: completeness-critic
- **Evidence**: "binds no-new-failures baseline" (§4.3 F3) and §5.4's baseline row — vs — no capture procedure anywhere; detection-assisted capture is deferred (WR-6) but the v1 path was a gap.
- **Acceptance condition**: state the v1 operator-run capture procedure.
- **Disposition**: applied — F3 and §5.4 name the documented operator-run capture (run the test verb, record failure identities), with WR-6 as the assisted future.

## Round 2

*Scope-locked re-review of the revision (fold `sha256:b1ebc5f0…`): each of FND-1..29's acceptance conditions verified met at its named anchor; the changed spans were re-read for regressions — the new rows (FR-18, FR-19, UB-7..9, WR-7, WR-8) carry complete EARS statements, fit criteria, risk tiers, and traces; the term migration ("proposal diff") is consistent across FR-8/11/12, §5.3, and §5.4; no finding against unchanged content arose. No new findings.*

## Verdict history

| Round | Verdict | Date |
|---|---|---|
| 1 | request-changes | 2026-07-16 |
| 2 | approve | 2026-07-16 |

## Escalation brief

none

## Measurements

- **Rounds**: 2
- **Findings**: 10 blocking · 19 advisory
- **Dispositions**: 29 applied · 0 declined · 0 withheld-as-violation

## Overlay corrections

none
