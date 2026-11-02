# Review: prd:contract-gate/000 — review report

| Field | Value |
|---|---|
| **In one line** | approve-with-advisories after a round-2 close: one blocking finding (a primary metric true-by-construction) and four advisories (currency-scope blur, an under-scoped gaming residual, a missing failure mode, an understated gate risk tier) all applied. |
| **Relevant-when** | auditing or consuming the review of the contract-gate PRD; asking why it was approved; tuning prd-review calibration against real findings on a deterministic-gate feature |
| **Subject** | prd:contract-gate/000 |
| **Moment** | new-base |
| **Reviewer** | prd-review · 2026-07-17 |
| **Date(s)** | 2026-07-17 |
| **Verdict** | approve-with-advisories |
| **Tracking** | agenticlab-ai/pyxis — the introducing PR (filed with this base) |
| **Template version** | review-report/v1 |

## Round 1

### FND-1 — the primary metric M1 is true-by-construction, so it cannot lose

- **Severity**: blocking (CAL-1)
- **Lens**: coherence
- **Evidence**: §3.2 M1 as drafted — "100% of planning-entry invocations … clear the gate before any draft; enforced by construction" — vs — prd-template §3.1 ("state each goal so it can lose: tentative, falsifiable, testable") and rule 7 (metrics are derived and targeted). Once FR-1 is enforced, plan-new cannot draft without a clean verdict, so "100% cleared the gate" is definitional, not a hypothesis; it verifies wiring, not the bet. The falsifiable failure of G1 is a *check missing a defect it should have caught* — an escape — which M1 as drafted does not measure.
- **Acceptance condition**: reframe M1 to a signal that can lose (the gate's escape/miss rate — a mechanically-flawed contract that passes yet carries one of the five defect classes, surfaced downstream), and demote the by-construction guarantee to a mechanism note.
- **Disposition**: applied — M1 reframed to the escape rate (target zero, "a non-zero escape falsifies the check set's completeness"); the guarantee restated as a wiring property in a note under §3.2.

### FND-2 — the currency check conflates two distinct properties (stale-serves vs §12 invalidation)

- **Severity**: advisory (CAL-3)
- **Lens**: contract-quality
- **Evidence**: FR-2 (§5.1) is defined as internal stale-serves / stale-builds-on, but its rationale and §1/§2.4/appetite lean on "the §12 staleness contract" — `brainstorm:contract-compiler-2#F-4` (§12 derived-artifact invalidation: a plan/charter pinned against an old fingerprint while main moved) vs `#F-5` (stale-serves: a live design serving a superseded requirement). These are different mechanisms, and §12's applies only when a *prior* derived artifact exists — which it does not, pre-planning. Blurring them leaves FR-2's actual contract ambiguous relative to its own framing.
- **Acceptance condition**: state FR-2's planning-entry check as the stale-serves state made gating, and mark §12 derived-artifact invalidation as the related-but-distinct downstream concern (the plan's own fingerprint pin), so the requirement and its rationale agree.
- **Disposition**: applied — FR-2's rationale rewritten to lead with stale-serves (`#F-5`) and explicitly distinguish §12 (`#F-4`) as governing a prior plan/charter, "with no derived artifact to check pre-planning."

### FND-3 — the present-but-substanceless residual is named for coverage only, not the structural check

- **Severity**: advisory (CAL-4)
- **Lens**: completeness-critic
- **Evidence**: ABUSE-1 (§5.3) names the gaming residual only for inverse coverage (a stub design that Serves every Must with no body). The identical mechanical/semantic gap applies to FR-5's structural check — a bogus fit criterion or a verification method naming no real proof passes field-presence just as a hollow design passes coverage. The abuse surface the section opens is half-covered; the what-would-stage-6-bounce pre-check would catch the analog.
- **Acceptance condition**: name the structural-field analog of the present-but-substanceless residual alongside ABUSE-1, closed at the same follow-up (WR-1/NG-2).
- **Disposition**: applied — ABUSE-1's mitigation extended to state the FR-5 structural analog explicitly and route it to WR-1.

### FND-4 — no failure mode is specified for a malformed severity-map policy

- **Severity**: advisory (CAL-4)
- **Lens**: completeness-critic
- **Evidence**: FR-13 (§5.1) introduces the closed-schema severity-map policy ("an unknown policy key fails closed-parse"), but §5.2's UB set specifies no row for the policy being malformed or absent — the sibling gate PRD specifies exactly this class (implementation-verification UB-3/UB-5: malformed manifest/baseline → configuration-error, fail-closed). A configuration surface with no stated failure behavior is an implied-but-unspecified failure mode.
- **Acceptance condition**: add a UB row for a malformed severity-map (configuration-error, fail-closed) and state the missing-policy fallback (the built-in default map).
- **Disposition**: applied — UB-6 added: malformed → configuration-error naming the defect; missing → falls back to the default `#D-4` split.

### FND-5 — the planning-entry gate row (FR-1) is risk-tiered high on a surface the template flags critical

- **Severity**: advisory (CAL-3)
- **Lens**: contract-quality
- **Evidence**: FR-1 (§5.1) wires a hard gate into planning and was tiered `high`; prd-template §5.1's risk rules name `critical` for "anything that can compromise factory control (governance, gates, policy)." FR-1 is the control-plane gate itself. `high` is defensible for a read-only, fail-closed row, but the tier decides downstream review-lane and rollback rigor, and understating the gate-core row is precisely the failure the risk-honesty rule guards.
- **Acceptance condition**: either upgrade FR-1 to `critical` with its safety plan named, or record why `high` is honest for a read-only fail-closed gate.
- **Disposition**: applied — FR-1 upgraded to `critical`; its safety plan (advisory-first staged rollout, fail-closed UBs, severity-demotion kill switch) named in the §8 readiness row.

## Round 2

*Scope-locked to the five changed spans (§3.2 M1 + note, FR-1 risk, FR-2 rationale, ABUSE-1, UB-6, §8 readiness row); the unchanged remainder is out of scope by protocol.*

- **FND-1** — closed: M1 now measures the escape rate, which can lose; the guarantee is a mechanism note. No regression: G1→M1 trace holds, §1 Success stays coherent (zero escapes = no plan on a flawed contract).
- **FND-2** — closed: FR-2 rationale distinguishes stale-serves from §12; the requirement and framing agree. No regression: `#F-4`/`#F-5` citations resolve (gate green).
- **FND-3** — closed: the structural analog is named at ABUSE-1, routed to WR-1. No regression: NG-2/WR-1 unchanged.
- **FND-4** — closed: UB-6 added; FR-13 now has its failure mode. No regression: UB numbering dense, DoD unaffected.
- **FND-5** — closed: FR-1 critical with a named safety plan; §8's high/critical row updated. No regression: §6.5 rollout already carried the plan.

No new finding on unchanged content (would be a protocol violation). All five round-1 findings dispositioned `applied`.

## Verdict history

| Round | Verdict | Date |
|---|---|---|
| 1 | request-changes | 2026-07-17 |
| 2 | approve-with-advisories | 2026-07-17 |

## Escalation brief

none

## Measurements

- **Rounds**: 2
- **Findings**: 1 blocking · 4 advisory
- **Dispositions**: 5 applied · 0 declined · 0 withheld-as-violation

## Overlay corrections

none
