# Amendment 001 — the usage-ownership boundary against the routed plane

| Field | Value |
|---|---|
| **Amends** | worker-transcript-metrics *(chain through 000)* |
| **Status** | Approved |
| **Owner** | @mdarabi |
| **Date** | 2026-08-06 |
| **Learned-from** | operator-decision:design-brainstorm:worker-transcript-metrics-tech-design#D-1 |
| **Blast radius** | tier: contract-changing; touches: Q1, §0, §8; reason: no requirement row's behavior changes — the ops resolve one open question and refresh the two sections that assert its state. The tier is declared contract-changing because §0 is superseded wholesale, which the read-to-rule contract requires at that tier even when the change is informative in scope. |
| **Template version** | amendment-template/v2 |

## Operations

### op: answer Q1

Reason: the stage-3 design-brainstorm gated `needs-more-product` on this question rather than designing around it, and its verification established that the two planes do not in fact contest the same artifacts.

| Field | Value |
|---|---|
| **Answer** | **This feature produces usage figures; the routed plane consumes them by reference.** The routed plane owns route selection and launch fidelity — which native profile was chosen, and whether the process launched with it. This feature owns per-process transcript metrics, usage figures included. Where `prd:planner-driven-model-routing` FR-20 connects "available usage measurements" to a routing decision, those measurements are this feature's records, referenced by identity rather than restated: FR-20 positions itself as a consumer of whatever usage is available, not as a second producer of it. No requirement of this contract changes, and none is ceded. |
| **Evidence** | design-brainstorm:worker-transcript-metrics-tech-design#D-1 (the ruling to bounce rather than design around it) · #P-1 (the parked gap and its basis anchors) · #F-4 · #F-8 · #F-9 · #F-10 (the verification: the native-routing half is implemented, but `port.LaunchCapture` carries `NativeModel`, `NativeEffort` and `Controls` only — no tokens, turns, duration or tool events — and nothing persists it; `routing-evidence-v1` is wholly unimplemented and the evidence ref its §2 calls the authority does not exist) · verified 2026-08-06 against origin/main: no routed-outcome or usage-capture code exists, and neither routing plan (001, 002 — both complete) covers `prd:planner-driven-model-routing` FR-20 or FR-21, so the claim is contractual but undecomposed |
| **Decided-by** | @mdarabi (operator) |
| **Effects** | none — no requirement, non-goal, metric, event or DoD row is added, superseded or retired by this answer. It resolves §7.2 Q1 and refreshes the two sections that assert Q1's state (§0, §8). The residual is recorded rather than closed: `prd:planner-driven-model-routing` FR-20 and EVT-5 still claim usage measurements in their own words, and this amendment binds only this contract — if that plane is later built to capture usage independently rather than reference these records, the duplication surfaces there and is that contract's to resolve. |

### op: supersede §0

Reason: §0's "author's open questions" block routes the reviewer to Q1 as unresolved; answering Q1 makes that routing false. The read-to-rule contract requires §0 be replaced wholesale in the same op set rather than patched, and the change is informative in scope — §0 binds nothing and no machine consumer reads it.

*The body is the contract; this section only routes attention — on any conflict the body governs; machines ignore it.*

**Judge these**

| # | The call | Could have gone | If you disagree |
|---|---|---|---|
| 1 | Capture is **use-agnostic and maximal** — no metric is filtered out because no analysis wants it yet | Capture only what a named consumer asks for | The perimeter collapses to whatever the first consumer needs, and every later question needs a schema change → NG-2, FR-2 |
| 2 | A **stored task rollup** exists beside the per-process records, restating their totals | Derive the task total at read time from the process records | You accept arithmetic at every read in exchange for one fewer figure that can disagree with itself → FR-6, NR-2, R1 |
| 3 | **All four harnesses in v1**, including the one that persists nothing and so needs capture arranged at launch | The three readable-after-the-fact harnesses first, the ephemeral one behind them | v1 ships without ever proving the schema against a streaming source, and the executor launch path stays untouched → FR-8, NG-4, R2 |
| 4 | A figure a harness cannot expose is **declared unavailable**, never omitted and never zero | Omit the field and let readers infer | Readers cannot distinguish "not measurable here" from "measured as zero" — the failure this contract is built to prevent → FR-4, UB-1, DOD-3 |
| 5 | **FR-8 derives in flight and persists no raw output**, rather than retaining the run's stream and governing it | Retain the raw stream as a governed artifact with an owner, lifetime and access boundary | You accept a file holding the prompts, commands, output and secrets that FR-5 exists to exclude — and the T3 privacy treatment that comes with it → FR-8, ABUSE-1, §5.4 |
| 6 | **Identifiers yes, free text no** — tool names and exit statuses are captured, command strings and outputs are not | Capture full transcript content | You take on prompt logging, secret redaction, and a T3 privacy surface; or, going the other way, you lose the ability to say *what* to improve → FR-3, NG-5 |
| 7 | **This feature produces usage figures; the routed plane consumes them by reference** — the two planes split by artifact, not by intent | Cede usage aggregation to the routed plane and capture only durations, tool events and context growth | You reopen FR-2, FR-6 and FR-14, and defer the figures to a plane with no code and no plan for them → §7.2 Q1, FR-2, FR-6, FR-14 |

**Sanity-check the numbers** — the completeness bar is **100% accounted-for** (M1): every launched process yields either a record or a capture-failure entry, and the two must partition the launched set. Not a sampling bar, which would make a missing record unfalsifiable; and not a bare record-share, which UB-2's no-record-on-failure rule makes unreachable. The failure side is bounded separately by **M4** at **1%**, a pre-registered guess awaiting first measurement. The capture-overhead ceiling is **NFR-1**'s p95, a fraction of the measured run rather than an absolute, because run lengths vary by orders of magnitude — and it is measured and trended, never gated on (AP-29: wall-clock is a measurement, not a correctness assertion; DOD-6).

**The author's open questions** — Q2 (whether the rollup is written at task end only, or amended as processes complete — bears on FR-6 and R1); Q3 (retention: this contract sets none, and the volume implied by FR-2 is unbounded over time); Q4 (what "token usage attributed to a tool use" means, given tokens are consumed per turn — FR-3 binds to whatever the design fixes). Q1 is resolved by amendment 001 and no longer routes attention here; the residual it left — that the routed plane's own contract still claims usage in its own words — is recorded in that amendment's Effects, not carried as an open question of this document.

**Skip the rest** — structure and ledger legality → docgate; substance and coherence → the unmanned PRD reviewer's report and the human overlay recorded beside it; requirement-to-code fidelity → the technical design and its review; §4.3 flow states, §5.9 evals, and most of §6 → skipped with stated reasons, since this feature ships no user-facing surface and no model-driven behavior. §5.3 is **not** skipped: FR-8 puts this feature in the path of a run's raw output, which is an abuse surface even though no new role is introduced.

### op: supersede §8

Reason: §8's open-questions row asserts that Q1 is solution-blocking and unresolved, which the `answer Q1` op makes false; a readiness gate stating a false readiness is worse than one stating a failing one. The problem-gate row is **deliberately left unchecked** — this amendment does not rule on whether a draft-phase gate table can be signed retroactively, and asserting a signature that was never recorded would be the same defect in the other direction. Every other row is carried forward verbatim.

| Check | Pass? | Notes |
|---|---|---|
| Problem basis accepted — §2 problem gate signed (T2) | ☐ | The §2.6 and §4.4 gate tables merged with their Date and Verdict cells reading *(pending)*; the introducing PR (#1035) was reviewed and merged, but no verdict was recorded in the tables before merge froze them. Whether a draft-phase gate may be signed retroactively by amendment is undecided and deliberately out of amendment 001's scope, so this row stays unchecked rather than being quietly passed |
| Zero solution-blocking TBDs or open questions (§7.2) | ☑ | Q1 — the sole solution-blocking question — is resolved by amendment 001. Q2, Q3 and Q4 remain open and owned; none is solution-blocking (they bear on the technical design and on a retention policy already deferred to WR-1) |
| Every Must FR has a fit criterion and a verification method (§5.1) | ☑ | the Must set is FR-1..FR-9 and FR-11..FR-15 (FR-10 is the sole Should); every one carries a fit criterion and a Verify method |
| Every requirement row carries a risk tier; high/critical rows have a declared safety plan (§5.1, §6.5) | ☑ | high: FR-4, FR-5, FR-8, FR-9, FR-12, FR-13, FR-15 · critical: NR-1. Safety plans: the disable path is a requirement (FR-12) with its own blocking proof (DOD-9), not a prose assurance; content escape is bounded by FR-5/NFR-5 and adversarially tested by ABUSE-1; and the two reconciliations **fail closed** — UB-7 requires FR-13 and FR-15 to report themselves *unavailable* when their independent source is missing or unreadable, so an absent denominator can never read as a clean result. No staged rollout applies, with the reason stated in §6.5 |
| Non-goals drawn (§3.4); non-regression rows named (§5.1) | ☑ | NG-1..NG-5, WR-1..WR-3; NR-1..NR-3 |
| Every primary job (§2.3) is covered by a requirement, a non-goal, or a waiting-room entry | ☑ | J1 → FR-6/FR-7; J2 → FR-3/FR-10; J3 → FR-11 (which process was rework) + FR-6/FR-7 (what it cost). FR-11 exists because FR-7's role alone cannot separate a first-pass process from a CI-fix one — both carry the same role |
| Every key flow (§4.3) has functional, failure, and non-regression coverage | ☑ | vacuous for flows — §4.3 skipped with reason. The failure space is covered by UB-1..UB-7, every row blocking-proved by DOD-4; non-regression by NR-1..NR-3 via DOD-2 |
| Every primary metric (§3.2) has instrumentation (§5.7) | ☑ | M1 → EVT-1/EVT-4/EVT-5 (its two terminal outcomes plus the unaccounted case; EVT-3 is the rollup event and feeds nothing here) · M2 → EVT-2/EVT-6 · M3 → EVT-1 · M4 → EVT-4. Each denominator is author-independent *and* each metric has an event that fires on the silent case: EVT-5 for a launch in the launch record with no outcome, EVT-6 for a required figure neither measured nor declared. Neither can be satisfied by capture emitting nothing |
| Every blocking DoD row traces to a requirement, guardrail, release criterion, or rubric (§5.8) | ☑ | the blocking set is DOD-1, DOD-2, DOD-3, DOD-4, DOD-5, DOD-7, DOD-9, DOD-10 — each citing its sources; DOD-4 covers the whole UB-1..UB-7 failure set. DOD-6 is deliberately **non-blocking** (AP-29: wall-clock is a measurement, not a correctness assertion) and DOD-8 is n/a with its reason stated in the row |
| Risks and assumptions have owners (§6.3) | ☑ | R1..R5 owned |
| Human-judgment criteria name an owner and a rubric (§5.8) | ☑ | vacuous — no DoD row uses the Human-judgment method. The methods in use are Test (DOD-1..DOD-5, DOD-7, DOD-9, DOD-10) and Analysis (DOD-6, a recorded measurement reviewed against NFR-1's ceiling); DOD-8 carries none, being n/a |
| DoD complete: every blocking row sourced and evidenced; every n/a explained (§5.8) | ☑ | DOD-8's n/a states its reason |
