# Amendment 001 — plan-waivers

| Field | Value |
|---|---|
| **Amends** | maintenance-sweep *(first amendment; chains from 000-base)* |
| **Status** | Approved |
| **Owner** | @mdarabi (operator) |
| **Date** | 2026-07-19 |
| **Learned-from** | review-thread:docs/reviews/maintenance-sweep-v1.md#FND-1 |
| **Blast radius** | tier: contract-changing; touches: FR-3, FR-5, §4.2, DOD-1, DOD-2, §6.5, §8; reason: two Must rows superseded so the design's plan-waiver mechanism has requirement ground — suppression of a Must-mandated output must be contracted, never extended by analogy; the DoD source cells and the readiness/rollout enumerations follow so no normative enumeration excludes a live Must row (review:maintenance-sweep-001#FND-2) |
| **Template version** | amendment-template/v2 |

## Operations

### op: supersede FR-3

Reason: the touches detector needs a contracted exclusion for waived in-flight plans (the design-review finding review:maintenance-sweep-v1#FND-1 — the stage-3 record's D-4 control was sound in judgment but had no requirement ground; without an exclusion clause every finding of a deliberately in-flight plan is mandated noise).

| ID | Requirement (EARS) | Rationale | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|
| FR-9 | When the touches-underdelivery detector runs, the sweep shall report, for each frozen plan task, every Touches surface absent from the evaluated state (§4.1), phrased as an advisory reconciliation signal, excluding tasks of plans a plan waiver names (FR-10). | the charter under-delivery class (T4/T8) (D-2; F-4's semantics), with the in-flight-plan control contracted rather than extended by analogy (design-brainstorm:maintenance-sweep-tech-design#D-4) | Must | low | on the pinned replay fixture, the two never-shipped wiring surfaces are reported citing plan + task; a fully-delivered plan fixture yields zero; a fixture whose in-flight plan carries a plan waiver yields zero touches findings for that plan and an applied-waiver line | Test | G1, J2, F1, M1 |

### op: supersede FR-5

Reason: the waiver contract widens from packages to both waiver kinds — package and frozen plan — so DOD-3's waiver-classified leg is reachable for touches emissions and every suppression stays reason-carried and surfaced (review:maintenance-sweep-v1#FND-1's acceptance, route (a)).

| ID | Requirement (EARS) | Rationale | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|
| FR-10 | Where a waiver names a package or a frozen plan, the sweep shall suppress that package's unwired finding or that plan's touches findings, and report each applied waiver with its recorded reason; a waiver entry carrying no reason shall refuse to load under UB-2's closed parse. | deliberate exceptions stay visible — and justified — never silent (D-2), for both waiver kinds the detectors carry | Must | low | a waived package produces an applied-waiver line surfacing the waiver's reason and no finding; a waived plan produces an applied-waiver line and zero touches findings for its tasks; removing either waiver restores the findings; a reasonless waiver entry faults the load (UB-2) | Test | G1, J1, J2, F1 |

### op: supersede §4.2 Scope perimeter

Reason: the In-perimeter waiver-surface wording names only unwired-packages; the surface contracted by FR-10 carries both waiver kinds (review:maintenance-sweep-v1#FND-1's acceptance names this edit).

- **In (plan of record)** — the sweep pass; the three detectors (unwired packages, plan-Touches under-delivery, rollup staleness); the typed findings schema + dual-face output; the waiver surface for unwired-packages and in-flight plans (FR-10); the scheduler shim + docs; replay fixtures for the historical instances.
- **Future (believed next, not committed)** — WR-1..WR-3; more detectors as evidence accumulates; the engine-era stage-11 report.
- **Out** — everything in §3.4.

### op: supersede DOD-1

Reason: the source cell's range (FR-1..FR-8) excludes the live Must rows FR-9/FR-10 minted above and still binds the superseded FR-3/FR-5 (review:maintenance-sweep-001#FND-2 — no normative enumeration may exclude a live Must row).

| ID | Done criterion | Source | Verify | Required evidence | Blocking? |
|---|---|---|---|---|---|
| DOD-6 | Every Must FR passes its fit criterion | the live Must set as of 001: FR-1, FR-2, FR-4, FR-6..FR-10 | Test | the test-suite run | yes |

### op: supersede DOD-2

Reason: the source cell's range (FR-2..FR-4) binds the superseded FR-3; the touches leg of the replay bar now lives at FR-9 (review:maintenance-sweep-001#FND-2).

| ID | Done criterion | Source | Verify | Required evidence | Blocking? |
|---|---|---|---|---|---|
| DOD-7 | The replay bar: the two historical rot instances and the constructed staleness fixture detected on their pinned fixtures (M1 = 3/3) | §3.2 M1; FR-2, FR-4, FR-9 | Test | the replay-fixture suite run | yes |

### op: supersede §6.5 Rollout and reversibility

Reason: the on-demand stage's minimum-evidence cell enumerates DOD-1..DOD-5; DOD-1/DOD-2 are superseded above, so the cell repoints at the live DoD set (review:maintenance-sweep-001#FND-2).

- **Flag** — n/a: local read-only tooling with no staged exposure; "rollout" is the operator choosing to install the schedule (its absence is the off state).
- **Kill switch** — n/a beyond removing the schedule/not invoking: the sweep holds no persistent or acting state to kill (NR-1).
- **Experiment** — n/a: no user-facing variant surface.
- **Cutover/migration** — n/a: no data, no predecessor system.

| Stage | Audience | Minimum evidence | Advance when | Roll back when |
|---|---|---|---|---|
| on-demand | operator + drives | the §5.8 DoD set green (DOD-3..DOD-7 as of 001) | §6.7 trigger reached | any M2 false positive → fix/waive before next run |
| scheduled | the host schedule | five scheduled sweeps | M2 = 0 across them → adopt | repeated FPs → unschedule, keep on-demand |

### op: supersede §8 Contract readiness — the executability gate

Reason: three enumeration cells bind superseded IDs (the Must-fit row's FR-1..FR-8; the jobs row's J1→FR-5/J2→FR-3; the flows row's FR-1..FR-8) and the DoD-trace row's range predates the DoD supersedes above (review:maintenance-sweep-001#FND-2).

| Check | Pass? | Notes |
|---|---|---|
| Problem basis accepted — §2 problem gate signed | ✅ | Argus-signed (tier-2), journaled delivery:maintenance-sweep#E-3; operator overturn channel open |
| Zero solution-blocking TBDs or open questions (§7.2) | ✅ | Q1–Q3 are waiting-room policy questions; Q4 is design-owned (stage 4), not solution-blocking |
| Every Must FR has a fit criterion and a verification method (§5.1) | ✅ | the live Must set as of 001: FR-1, FR-2, FR-4, FR-6..FR-10 |
| Every requirement row carries a risk tier; high/critical rows have a declared safety plan | ✅ | all rows low risk (read-only advisory tooling); no high/critical rows exist |
| Non-goals drawn (§3.4); non-regression rows named (§5.1) | ✅ | NG-1..NG-4; NR-1..NR-2 |
| Every primary job (§2.3) is covered by a requirement, a non-goal, or a waiting-room entry | ✅ | J1→FR-2/FR-10; J2→FR-9; J3→FR-4; J4→FR-1 |
| Every key flow (§4.3) has functional, failure, and non-regression coverage | ✅ | F1→FR-1, FR-2, FR-4, FR-6..FR-10/UB-1..UB-5/NR-1; F2→FR-7/FR-8/UB-5 |
| Every primary metric (§3.2) has instrumentation (§5.7) | ✅ | M1..M3 ← EVT-1 |
| Every blocking DoD row traces to a requirement, guardrail, release criterion, or rubric | ✅ | DOD-3..DOD-7 sourced by reference (the glossary landing is not a DoD row: already landed in this write set and journaled at delivery:maintenance-sweep#E-2) |
| Risks and assumptions have owners (§6.3) | ✅ | R1..R4 |
| Human-judgment criteria name an owner and a rubric (§5.8) | ✅ | vacuously — no human-judgment rows (all DoD rows machine-verifiable by design) |
| DoD complete: every blocking row sourced and evidenced; every n/a explained | ✅ | no n/a rows |
