# Review: prd:worker-transcript-metrics/000 — review report

| Field | Value |
|---|---|
| **In one line** | approve-with-advisories — round 1's four blocking findings (an unreachable primary metric, an uncovered job, a self-certifying Must row, an undeclared sibling departure) were all applied; one advisory survives on the round-2 span. |
| **Relevant-when** | auditing or consuming the review of prd:worker-transcript-metrics; asking why its 000-base was bounced and what changed; tuning prd-review calibration against real findings |
| **Subject** | prd:worker-transcript-metrics/000 |
| **Moment** | new-base |
| **Reviewer** | prd-review · .pyxis/authoring/prd-review.md @ 88eb521 |
| **Date(s)** | 2026-08-05 |
| **Verdict** | approve-with-advisories |
| **Tracking** | — *(the subject's introducing PR)* |
| **Template version** | review-report/v1 |

## Round 1

*Reviewed at fold `sha256:9613133f…`.*

### FND-1 — The 100% capture bar is unreachable in the presence of the contract's own capture-failure path, and the ship rule does not cover it

- **Severity**: blocking (CAL-1)
- **Lens**: coherence
- **Evidence**: §3.2 M1 targets "100% of launched processes" and §5.8 DOD-3 requires "every launched process yields a schema-valid record" (blocking) — vs — §5.2 UB-2, which requires that when capture fails the system "leave no partial or invalid record". A capture failure therefore produces no record, so M1 cannot reach 100% and DOD-3 cannot pass. §3.3's ship rule anticipates only one shortfall: "M1 below 100% for a reason the record itself declares (a harness that provably cannot expose the figure)" — which is the FR-4 unavailability case, not the UB-2 failure case. The two paths are distinct: FR-4 yields a record *with* a declared gap; UB-2 yields no record at all. NFR-4 and EVT-4 record the failure but do not produce the record M1 counts.
- **Acceptance condition**: Either M1/DOD-3 are restated over a denominator that excludes UB-2 failures (e.g. "every launched process yields a record or a capture-failure entry, and the two sets partition the launched set"), or §3.3's ship rule gains an explicit UB-2 branch with its own bar. Whichever is chosen, M1, DOD-3 and the ship rule must agree on what a capture failure does to the number.
- **Disposition**: applied — M1 restated as an accounted-for partition (record ∪ capture-failure entry, no process in neither); M4 added to bound the failure side with a §3.3 guardrail; the ship rule rewritten to give the two shortfalls distinct rules; DOD-3 restated to match and now sources UB-2. Recorded in §7.3.

### FND-2 — J3 is marked covered, but no requirement captures what distinguishes rework from first-pass work

- **Severity**: blocking (CAL-1)
- **Lens**: coherence
- **Evidence**: §2.3 J3 states the outcome "so I can judge whether planned tasks are sliced too large", and §8's row "Every primary job (§2.3) is covered by a requirement, a non-goal, or a waiting-room entry" is marked ☑ citing "J3 → FR-6/FR-7". But FR-7 captures "the role it ran in", and the record fixes role as implementation-or-review (`brainstorm:worker-transcript-metrics#D-2`); an initial implementation process and a CI-fix process share that role. FR-6 yields only a count and a sum. So the record set can show *that* a task took several processes, but not which were rework and of what kind — and §4.4 S1, the PRD's own worked scenario, narrates exactly that distinction ("CI is red, so a second process runs… a review-remediation process follows"). §8 asserts coverage the requirements do not deliver, which is the row the gate exists to verify rather than trust.
- **Acceptance condition**: Either add a requirement capturing each process's purpose or ordinal within its task (so first-pass, CI-fix and review-remediation are distinguishable), or narrow J3's stated outcome to what FR-6/FR-7 actually support and correct the §8 row's justification.
- **Disposition**: applied — FR-11 added (ordinal + launch occasion per process, Must), with DOD-10 as its blocking proof; §8's job-coverage row rewritten to cite it and to state why role alone was insufficient. Recorded in §7.3.

### FND-3 — FR-2's fit criterion certifies itself: the artifact it verifies against is produced by the effort under test

- **Severity**: blocking (CAL-3 — escalated from advisory: the defect makes a Must row unverifiable)
- **Lens**: contract-quality
- **Evidence**: §5.1 FR-2 requires "every measurement its harness exposes", with the fit criterion "for each supported harness, every measurement **the capability matrix records as exposed** appears in the record or as an FR-4 unavailability". §6.2 lists that capability matrix as a dependency owned by "this feature's technical design", status at-risk. The acceptance therefore tests the build against a description the same build authors: any measurement not captured can be omitted from the matrix, and the row passes. The stated Verify method (Inspection) cannot disagree with the implementation. This is the failure incident **I8** names — "a test can encode the bug and stay green forever… when a test's FIXTURE does not match the state its acceptance names, it certifies the opposite behavior while reading as coverage" — with the capability matrix in the fixture's role. FR-2 is also the row carrying `brainstorm:worker-transcript-metrics#D-4`'s maximal-capture ruling, so an unfalsifiable FR-2 leaves the contract's central commitment unverified.
- **Acceptance condition**: Bind FR-2's fit criterion to an author-independent source of what a harness exposes — the harness's own published output surface, or a matrix frozen and reviewed before the capture path is built — so the row can fail. Alternatively, state FR-2's ambition in the narrative and move the verifiable part into per-harness rows whose figures are enumerated in the contract.
- **Disposition**: applied — FR-2's fit criterion rebound to each harness's own published output contract, explicitly "not any description this feature authors", with the enumeration required to be frozen and reviewed before that harness's capture path is built; §6.2's dependency row now carries the same constraint and names I8 as the failure it avoids. Recorded in §7.3.

### FND-4 — The departure from the adjacent merged contract's execution-agnostic posture is never stated in the body

- **Severity**: blocking (CAL-2)
- **Lens**: upstream-alignment
- **Evidence**: §5.1 FR-8 requires that "where a harness does not persist its own run data, the system shall retain that data as the run produces it" — capture arranged on the launch path — vs — the merged sibling `prd:pipeline-measurement` NG-3, whose reason states "measurement is execution-agnostic, reading whatever accounting the run produced". FR-8 does not read whatever the run produced; it requires the run to be arranged so the data survives. The record acknowledges this as a deliberate departure (`brainstorm:worker-transcript-metrics#D-5`), and §0 row 3 flags it — but §0 is informative and excluded from review by AGENTS.md rule 9, so it cannot carry a contract-level claim. In the body the departure appears only as R2, which names the *risk to the launch path*, not the *conflict with the sibling's stated posture*. A reader of the contract alone cannot see that an adjacent merged contract's rationale is being departed from.
- **Acceptance condition**: State the departure in the body — as an NG/WR row, an FR-8 rationale clause, or a §7.1 internal FAQ entry — naming `prd:pipeline-measurement` NG-3 and why this contract diverges. §0 may continue to route to it, but the claim must live somewhere normative.
- **Disposition**: applied — FR-8's rationale now names the departure and the sibling row explicitly and states its bound (NR-1/NR-3 forbid behavioral change); a §7.1 internal FAQ entry answers the question directly.

### FND-5 — NR-1 is critical and its safety plan rests on a disable path that no blocking row proves independently

- **Severity**: advisory (CAL-4)
- **Lens**: completeness-critic
- **Evidence**: §5.1 NR-1 carries risk **critical** ("A task's terminal outcome class is unchanged by the presence of capture"), and §8's row on high/critical safety plans is marked ☑ citing "§6.5's stated reversibility plus NR-1/NR-3/UB-2". §6.5 skips staged rollout and offers "capture can be disabled, at which point runs proceed exactly as before". That disable path is the whole safety plan, yet it is named nowhere as a requirement and is exercised only incidentally — NR-1's fit criterion compares enabled against disabled runs, which touches the path but tests outcome parity, not that disabling is reliable, complete, or leaves no residue. The template's rollout guidance is explicit that "an untested fallback is a second incident".
- **Acceptance condition**: Either add a requirement (and a blocking DoD row) for the disable path itself — that capture can be turned off and that doing so leaves runs and the store in a defined state — or record in §6.5 why NR-1's parity test is considered sufficient proof of the fallback.
- **Disposition**: applied — FR-12 added (capture disabled writes no record and no capture-failure entry, Must, risk high) with DOD-9 as its blocking proof; §8's safety-plan row rewritten to cite the requirement rather than prose. Recorded in §7.3.

### FND-6 — Must rows are unranked, against the template's explicit rank-within-class rule

- **Severity**: advisory (CAL-3)
- **Lens**: contract-quality
- **Evidence**: §5.1 carries nine Must rows (FR-1..FR-9) with no ranking — vs — the template's priority bar: "rank within each class, because cuts arrive mid-build and unranked lists cut themselves". The risk is concrete here rather than formal: §6.1 phases the work across harnesses, and FR-8 (the ephemeral-harness retention, the row `brainstorm:worker-transcript-metrics#D-5` describes as the schema-constraining hard case) is the row most likely to be deferred under pressure — which would deliver precisely the file-shaped schema D-5 exists to prevent.
- **Acceptance condition**: Rank the Must rows, or state in §5.1 why ranking is withheld for this contract.
- **Disposition**: applied — §5.1 gained a Must-ranking paragraph ordering all eleven Must rows, placing FR-8 deliberately above the last three with the D-5 reason stated.

## Round 2

*Scope-locked to the six round-1 defects and regressions within their changed spans; reviewed at fold `sha256:403f4db1…`. Every round-1 finding verified applied at its cited anchor. One new finding, on changed content and therefore in scope.*

### FND-7 — FR-11's "closed set" of launch occasions is never enumerated, leaving a consumer unable to know the values

- **Severity**: advisory (CAL-3)
- **Lens**: contract-quality
- **Evidence**: FR-11's fit criterion requires "an occasion drawn from a closed set", but no section names that set's members — vs — Appendix A's banned-vagueness list, which strikes open-ended clauses, and the §2.2 primary segment ("machine consumers… process many runs programmatically"), which cannot branch on an enumeration it is never given. The PRD's own §4.4 S1 already names three occasions in prose (first-pass, CI-fix, review-remediation), so the set is knowable at contract level; leaving it implicit forces every consumer to discover it from data. This does not make FR-11 unverifiable — the fit criterion still fails if the field is absent or open-valued — so it stays advisory under CAL-3 rather than escalating.
- **Acceptance condition**: Enumerate the occasion values in FR-11 or in §5.4's data table, or state that the set is fixed by the technical design and cite where a consumer reads it.
- **Disposition**: *(open — advisory; the author may take or leave it)*

## Verdict history

| Round | Verdict | Date |
|---|---|---|
| 1 | request-changes | 2026-08-05 |
| 2 | approve-with-advisories | 2026-08-05 |

## Escalation brief

none

## Measurements

- **Rounds**: 2 *(the protocol cap; no third round is legal)*
- **Findings**: 4 blocking · 3 advisory *(round 1: 4 blocking, 2 advisory · round 2: 1 advisory)*
- **Dispositions**: 6 applied · 0 declined · 0 withheld-as-violation · 1 open advisory
- **Overlay corrections (M2 residual)**: 8 applied post-approve — 2 of them residuals of the reviewer's own round-1 fixes (OC-1, OC-6)
- **Note**: every round-1 blocking finding was applied rather than declined, so no auto-escalation occurred. The subject's own declared open question Q1 (the routed-execution boundary) was assessed, not re-raised: it is disclosed in §7.2 and §8 already records that readiness row as failing, so it is visible rather than hidden and needs no finding.

## Overlay corrections

*Post-approve entries only — the M2 residual data; frozen rounds and history are never edited.*

**2026-08-05 · @mdarabi (human overlay, post-approve).** Eight corrections against the round-2 subject (`sha256:403f4db1…`), all applied; subject refolded to `sha256:e325cd99…`. These are overlay data, **not** a third round: the protocol caps at two, and the round-2 verdict stands as recorded. Two of the eight (OC-1, OC-6) are residuals of the reviewer's own round-1 fixes — the reviewer changed a metric and its DoD row but missed the requirement stating the same bar, and it accepted an unavailability rule that let capture defects pass as capability gaps. That is the M2 residual this section exists to measure.

| # | Correction | Lens it belongs to | Why the rounds missed it |
|---|---|---|---|
| OC-1 | FR-1's own fit criterion still required "records equal launches", contradicting UB-2 — the round-1 FND-1 fix reached M1, M4, the ship rule and DOD-3 but not the requirement itself, and DOD-1 inherited the contradiction | coherence | an incomplete application of the reviewer's own accepted finding — the fix was verified at the anchors the finding cited, not across every row asserting the same bar |
| OC-2 | FR-8's "retain that data" created an ungoverned artifact holding the prompts, commands, output and secrets FR-5 excludes — with no owner, lifetime, access boundary, privacy class, or crash/cancellation handling, and arguably T3 territory | completeness-critic | both rounds read FR-5's no-content property as holding for the record and never asked what FR-8's retention step itself stores; the surface was created by a row neither round examined for data governance |
| OC-3 | M1 and M2 were fed only by capture's own events, so a process whose capture never ran was invisible to the metric measuring exactly that silence | coherence | the round-1 fix made M1's *partition* correct without asking where its *denominator* came from; instrumentation was checked for existence, not independence |
| OC-4 | DOD-6 gated release on p95/p99 wall-clock, against AP-29 ("a gate asserts correctness; a measurement only records") and incident I5 | upstream-alignment | the alignment lens checked sibling PRDs and the basis record but did not sweep the principle registry against the *verification methods*, only against the requirements |
| OC-5 | FR-6 required an exact sum while FR-4 and UB-2 both permit missing parts, leaving rollup missingness undefined | coherence | the interaction sits across three rows in two sections; each is individually coherent |
| OC-6 | UB-5 mapped malformed input to unavailability, letting this feature's own parser and capture defects pass as honest degradation and evade M4; UB-1 mapped every zero to unavailability, destroying legitimately measured zeros | upstream-alignment | the reviewer treated the unavailability path as the safe default without asking what it lets through — the same silent-zero discipline it was praising, inverted |
| OC-7 | DOD-7 proved only FR-9's direction of schema compatibility, not NFR-3's, and proved parsing rather than stable field meaning | coherence | the DoD row named both sources, which read as covering both |
| OC-8 | FR-7 verified attribution fields as non-empty, so records with swapped task identities would pass | contract-quality | "carries all five, non-empty" reads as a fit criterion; that it cannot fail the misattribution it targets needs the adversarial question the lens did not ask of it |

**2026-08-05 · @mdarabi (human overlay, second pass).** A re-check of the eight found **four closed and four carrying blocking residuals** — OC-2, OC-3, OC-4, OC-5. Every residual was the same defect: the correction was applied to the *requirement row the finding cited* and not to the other sections restating the same claim. Subject refolded to `sha256:a3ec0ea9…`.

| # | Residual | Where the claim survived |
|---|---|---|
| OC-2 | FR-8 required in-flight derivation while seven other places still specified retention | §4.1 approach · §4.2 scope · §4.4 S2 · §5.5 two interface rows · §6.1 phase · §6.2 dependency · §6.3 R2 — plus the §2.4 evidence row saying the caller "must retain" the stream. Separately, FR-8's own proof was **overbroad**: forbidding the seeded content in "any file, buffer, or store" would fail against the task's worktree, which legitimately holds the code the agent wrote. Now scoped to capture-owned artifacts |
| OC-3 | The denominators were added, but M2 still could not observe its own failure: it fed from EVT-2, which fires only when an unavailability is *successfully declared*, so a silently dropped figure emitted nothing | §3.2 M2's feed. Closed by FR-15 — a reconciliation over the required-figure set emitting EVT-6, mirroring what FR-13/EVT-5 do for launches. The launch record, required-figure set, capture-failure entry and discrepancy also joined §5.4/§5.5, which had none of them |
| OC-4 | DOD-6 stopped gating, but three other places still did | §1 Guardrail · §3.3 guardrail table (feeding "no guardrail breached" in the ship rule) · §6.4 release criterion. The guardrail table now states explicitly *why* overhead is absent from it |
| OC-5 | FR-6 defined incomplete rollups; §5.4's data contract still said every rollup figure equals the sum of its parts, and named process records alone as the source of truth even where a missing process must be identified | §5.4 task-rollup row — now carrying the measured/incomplete states and the launch record as a second source |

Also closed: **FND-7** — FR-11 now enumerates its occasion set as {first-pass, ci-fix, review-remediation, resume}.

**2026-08-05 · @mdarabi (human overlay, third pass).** §8 — the contract-readiness gate — was certifying **stale evidence**, because every correction in the two passes above changed the sets §8 summarizes without §8 being regenerated. Subject refolded to `sha256:2cf0a784…`.

| # | Stale §8 claim | Reality after the earlier passes |
|---|---|---|
| OC-9 | "Every Must FR has a fit criterion" citing FR-1..FR-9 | the Must set had grown to FR-1..FR-9 + FR-11..FR-15 (14 rows); five were uncertified by the gate that claims to check them |
| OC-10 | the safety-plan row omitted the new high-risk FR-13 and FR-15, and named no fail-closed treatment for them | both are reconciliations whose failure mode — a missing independent source reading as a clean result — had no contract rule at all. Closed by **UB-7**: a reconciliation whose source is unavailable reports *unavailable*, never clean |
| OC-11 | M1 fed from EVT-3 (the rollup event, irrelevant to process accounting) while EVT-4 did not list M1 among its consumers | M1's two terminal outcomes are the record (EVT-1) and the capture-failure entry (EVT-4), plus EVT-5 for the unaccounted case |
| OC-12 | the blocking-DoD row cited DOD-1..DOD-7 — counting the now-non-blocking DOD-6 and omitting the new blocking DOD-9/DOD-10 | blocking set is DOD-1..5, 7, 9, 10; DOD-6 non-blocking per AP-29; DOD-8 n/a |
| OC-13 | "every DoD row is Test or Inspection" | DOD-6 became Analysis when OC-4 unblocked it |
| OC-14 *(advisory)* | ABUSE-1 still claimed the seeded material appears in *no* file, buffer or store globally, after FR-8's proof had been narrowed | now mirrors FR-8's capture-owned scope |
| OC-15 *(advisory)* | the §5.4 note said "the table has three rows and not four" (six now); DOD-6 pointed at a §3.3 guardrail action that OC-4 had removed; R1 and the §7.1 FAQ still described exact-sum verification only | all three restated against the current content |

**2026-08-05 · @mdarabi (human overlay, fourth pass).** Three residuals, every one created by the third pass itself. Subject refolded to `sha256:3275cac1…`.

| # | Residual | Origin |
|---|---|---|
| OC-16 | UB-7 — added in pass 3 as a **high-risk Must** failure behavior — had no blocking completion coverage: DOD-4 still cited UB-1..UB-6, and §8's failure-coverage row said the same. A rule written specifically so a missing reconciliation source cannot read as clean was itself unproven | added a UB row without extending the two places that enumerate the UB set |
| OC-17 | EVT-3 still claimed it feeds M1, after pass 3 removed EVT-3 from M1's feeds — the mapping was corrected forward and left stale in reverse | a bidirectional relationship edited on one side |
| OC-18 | §8's failure row likewise enumerated UB-1..UB-6 | same as OC-16 |

**This is where discipline stopped being the answer.** Five passes, each finding the same class of defect, three of them finding defects the previous fix created. The remedy is no longer "sweep more carefully" — it is a script. `check-sync.py`, beside this ledger, extracts the sets a PRD *derives* and diffs them against the sections that *summarise* them:

- §8's Must set, high/critical set, and blocking/non-blocking DoD split, against the requirement tables
- the full UB set against DOD-4 and §8's failure-coverage row
- metric↔event mapping in **both** directions

It earned its place immediately: run against the pass-4 fixes it caught a defect in the fix itself — EVT-3's Feeds cell had been rewritten to explain *why* it feeds no metric, and the explanation mentioned M1, so the cell still named M1. A human reading that row would have called it correct.

**The pattern worth keeping.** Four consecutive passes made the same mistake in two escalating forms. First: fixing at the finding's cited anchor and treating the anchor as the extent of the claim — a PRD restates its commitments across §1, §3, §4, §5 and §6 by design, so any correction to a requirement is a **document-wide sweep**, not an edit. Second, and worse: **§8 is not prose, it is a derived index** — it enumerates the Must set, the high-risk set, the blocking-DoD set, the verification methods and the metric-to-event mapping. Every requirement, event or DoD change silently falsifies it, and because it is the *readiness gate*, a stale §8 certifies a contract nobody checked. It must be **regenerated from the current sets, never edited**.

That is now mechanically checkable rather than a discipline: the four sets §8 claims can be extracted from the requirement tables and diffed against §8's own citations, which is how OC-9..OC-13 were confirmed closed rather than by re-reading. A `docgate` verb doing this at gate time would have caught all five before any reviewer saw them — the natural amendment once this ledger is live.
