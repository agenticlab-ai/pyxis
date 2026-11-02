# Amendment 001 — codegate seed verb

| Field | Value |
|---|---|
| **Amends** | implementation-verification *(chain through 000)* |
| **Status** | Approved |
| **Owner** | @mdarabi |
| **Date** | 2026-07-28 |
| **Learned-from** | operator-decision:`design-brainstorm:init-hardening-tech-design#D-6` |
| **Blast radius** | tier: contract-changing; touches: §0, FR-20 (new), UB-10 (new), UB-11 (new); reason: Must-priority additions contracting the one-shot manifest seed verb; §0's "never detected" row re-worded to bind detection to runtime, which the verb never touches |
| **Template version** | amendment-template/v2 |

## Operations

### op: add FR

Reason: `design-brainstorm:init-hardening-tech-design#D-6` assigned first-use manifest seeding to the manifest's owner — the init-hardening truth surfaces (welcome, doctor's advisory) route users to this verb by name, so it is contract, not convenience.

| ID | Requirement (EARS) | Rationale | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|
| FR-20 | Where a repository holds recognized stack marker files (the ecosystem defaults' marker set — one detection source) and no verification manifest, the codegate seed verb shall write a closed-schema-valid template manifest declaring the detected stack — verb commands filled from the ecosystem defaults where real defaults exist, and as named to-be-edited placeholders otherwise — for the operator to own and edit; the harness's runtime reading is unchanged (FR-5). | the authoring aid that closes the fresh-operand gap while the manifest stays declared, human-owned data | Must | med | on manifest-less fixtures per supported stack, seed writes a manifest that closed-parses (FR-5's parser), declares the marker-matched stack, carries real defaults where the ecosystem ships them and named placeholders otherwise; the harness consumes the seeded manifest identically to a hand-authored one | Test | G3, J4, F3 |

### op: add UB

Reason: the verb fails closed on both edges — an existing manifest is operator-owned data it never overwrites; an undetectable or ambiguous tree gets the hand-authoring remedy, never a guess.

| ID | If… (trigger) | Then the system shall… | User sees | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|---|
| UB-10 | the seed verb runs where a verification manifest already exists | refuse by name and change nothing — a declared manifest is operator-owned, never overwritten | a named refusal citing the existing manifest path | Must | med | on a manifest-bearing fixture the verb exits non-zero naming the path; the manifest is byte-identical | Test | FR-20 |
| UB-11 | the seed verb finds no recognized marker files, or markers matching multiple stacks ambiguously | write nothing and name the hand-authoring remedy (the manifest path and the declared-stack requirement) | a named refusal carrying the authoring instruction | Must | low | on an empty fixture and on a mixed-marker fixture the verb writes nothing; the remedy names the manifest path | Test | FR-20 |

### op: supersede §0

Reason: the op set changes content §0 routes — judge row 5's "never detected" slogan must bind detection to runtime (the displaced alternative it names) so the authoring-time seed verb cannot read as its reversal; §0-only informative scope, replaced wholesale at the computed contract-changing tier.


*The body is the contract; this section only routes attention — on any conflict the body governs; machines ignore it.*

**Judge these**

| # | The call | Could have gone | If you disagree |
|---|---|---|---|
| 1 | **Agent-free harness; drive-side stays in implement-now** — this feature runs checks and returns findings; launching/prompting the agent, fix caps, and escalation belong to the host | The harness embeds the fix loop itself (verify → prompt → re-verify as one component) | The seam moves: implement-now's contract gains the loop, the remote gate becomes a stripped variant instead of the identical artifact → §4.1, NG-1 |
| 2 | **Anti-gaming checks ship in v1** — falsification, test-delta audit, machinery protection are rungs, not future work | Parity minimum (mirror repo lint/build/test only), novel checks later | "Green that isn't green" — implement-now's named top risk — stays open through its dogfood; rung 3 rows fall out → FR-11, FR-12, FR-8, M2 |
| 3 | **v1 is fully deterministic** — zero LLM judgment in any rung | Include judgment lenses (scope/spec fidelity) from the start | Verdicts gain a non-reproducible component before trust exists; cost per cycle stops being ~zero → §3.4 WR-2, NFR-5 |
| 4 | **Remote CI is design-binding, not delivered** — the harness must be portable unchanged, proven by one advisory dry-run; enforcement ships later | Ship the remote required-check now | The bypass hole (hand-written PRs landing unchecked) stays open longer — that is the accepted cost → §3.4 WR-5, FR-15, M4 |
| 5 | **Manifest is declared, never runtime-detected** — hand-authored closed schema + ecosystem defaults; absent verb ⇒ skip-with-record; the seed verb (FR-20) is a one-shot authoring aid writing the declared artifact for the operator to own | Runtime stack detection (zero-config) | Runtime guessing changes what green means without a human act (judge-once violated); ambiguity has no resolution surface → FR-5, FR-6, FR-20 |
| 6 | **No-new-failures baseline from day one** — red-main repos bind honestly; ratchet is shrink-only, baseline writes are human-lane | Require a green main to adopt (all-or-nothing) | Brownfield repos get no test gate at all until someone cleans up first → FR-10, ABUSE-3 |
| 7 | **The implementing agent is P1** — latency ordering, compact findings, incremental re-runs optimize the machine's loop; the operator is P2 | Operator-first segmentation (implement-now's choice) | Efficiency trade-offs flip toward human-facing polish; the "minimize back-and-forth" requirement loses its tiebreaker → §2.2, D-7 in §7.3 |

**Sanity-check the numbers** — the catch test demands **3/3 seed families blocked** (M2); fix-cycle economy demands **p50 re-verification wall-time ≤ 50% of a full ladder run** on the dogfood repo (M3); rung-0 feedback lands in **p95 ≤ 10 s** on the dogfood repo (NFR-1); the guardrail tolerates **zero unresolved wrong-blocks** at the dogfood checkpoint (§3.3). Retry caps, turn caps, and parallelism are the host's numbers, not this document's.

**The author's open questions** — whether a repo with no bindable exec-probe should ever count as more than green-degraded long-term (§7.2 Q5, launch-gate-bound); the remote gate's enforcement semantics are deliberately unresolved (§7.2 Q1, remote-phase-bound). The remaining §7.2 rows are design-owned mechanics.

**Skip the rest** — structure/ID/fold correctness → docgate; substance and coherence → the unmanned `/prd-review`; finding-schema, incremental strategy, weakened-test detection depth → the technical design (§7.2 design-owned rows); fix loops, caps, escalation → implement-now's PRD; merge safety and landing → merge-lane.

