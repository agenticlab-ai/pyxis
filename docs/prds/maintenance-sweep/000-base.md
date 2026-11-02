# Maintenance sweep — PRD

| Field | Value |
|---|---|
| **In one line** | A read-only, on-demand + schedulable maintenance sweep — stage 11's first mechanical slice — that detects operational rot (first-party packages no shipped binary reaches, frozen-plan task surfaces that never materialized, stale living rollups) and emits typed findings with named remedies, never fixes. |
| **Relevant-when** | planning, building, or reviewing periodic-maintenance / drift-detection machinery (the maintenance sweep, its detectors, waivers, findings schema, scheduling); triaging a rot finding, a false positive, or a missed detection; changing living-rollup freshness conventions, plan Touches semantics, or binary/package wiring the sweep audits. |
| **Status** | Draft — problem and solution gates signed by Argus (decision principal, tier-2; delivery:maintenance-sweep#E-3); the introducing PR lands by session merge under problem:maintenance-sweep#A-1, with the operator's review arriving through the normal overturn channels |
| **Tier** | T2 — a new read-only capability with no factory-control, security, or data surface; blast radius is advisory output. (Not T1: multiple detectors, a waiver policy surface, and a scheduling story deserve the standard sections; not T3: nothing here gates, mutates, or grants.) |
| **Owner** | @mdarabi (operator) |
| **Approver(s)** | @mdarabi (operator) — the introducing PR is session-merged under problem:maintenance-sweep#A-1; the operator's review is the standing overturn channel (template rule 15 — the "How to use this template" rules — adapted to the delegated-drive case). |
| **Contributors** | @mdarabi (operator / product / engineering); Argus (AI Engineer, decision principal at tier-2 — authored this contract from the gated record `brainstorm:maintenance-sweep`). |
| **Informed** | — |
| **Date** | 2026-07-19 created · 2026-07-19 last updated |
| **Appetite** | Small and bounded (brainstorm:maintenance-sweep#D-7). Machine work: one implementation plan of at most six tasks; three mechanical detectors + findings schema + shim; fixtures incl. historical-rot replays. Human attention: review of this PRD, one technical design, and occasional finding dispositions. Rollout burden: none (local read-only tooling; the operator installs the schedule if desired). Excludes: new third-party dependencies; semantic/LLM detectors; any mutation surface. |
| **Target release** | No calendar target. Checkpoint-bound: v1 done when the DoD's replay + live bars hold (§5.8). |
| **Tracking** | — *(the introducing PR / epic, once filed)* |
| **Artifacts** | Brainstorm record `docs/brainstorms/maintenance-sweep.md` (the gated stage-1 session); the drive ledgers `docs/problems/maintenance-sweep/`. |
| **Template version** | prd-template/v2 |

## 0. Read to rule — the human reviewer's section (~4 min)

*The body is the contract; this section only routes attention — on any conflict the body governs; machines ignore it.*

**Judge these**

| # | The call | Could have gone | If you disagree |
|---|---|---|---|
| 1 | The sweep is its own periodic moment, never a per-PR gate | fold the detectors into docgate/codegate's gates | veto reopens amendment 017's retired shape — strike NG-2 and rewrite §4.1 → §4.1, NG-2 |
| 2 | Findings, never fixes — zero mutation, zero tracker writes in v1 | auto-file issues per finding | veto adds a write surface and a dedup discipline — supersede FR-1/NR-1, reopen G-3 → §5.1 FR-1, NG-1 |
| 3 | Exactly three detectors in v1; map-row drift and symbol-level dead code deliberately parked | ship a wider detector set now | veto raises the appetite and the dependency posture — supersede FR-2..FR-4, revisit WR-1/WR-2 → §5.1, §3.4 |
| 4 | Zero-false-positive live bar (a noisy sweep dies of being ignored) | ship findings best-effort, tune later | veto trades trust for coverage — rewrite M2 and DOD-3 → §3.2, §5.8 |

**Sanity-check the numbers** — three detectors (§5.1 FR-2..FR-4); replay bar 3/3 = two historical replays + one constructed staleness fixture (M1, §3.2); live false positives 0 (M2); full-sweep runtime ≤ 60s on this repo class (NFR-1); plan ≤ 6 tasks (header Appetite).

**The author's open questions** — Q1 map-row drift mechanization (§7.2, from brainstorm G-1); Q2 symbol-level dead-code dependency decision (§7.2, from G-2); Q3 finding→issue auto-filing policy (§7.2, from G-3).

**Skip the rest** — structure/immutability/grammar → docgate; substance and coherence → the unmanned prd-review (its report rides this PR); implementation shape → the technical design (P-1..P-5 routed there); proposal verification → codegate at stage 8.

## 1. Summary — the one-pager

- **Problem** — Operational rot in the factory goes unnoticed until a human trips over it: a whole feature's implementation sat on the mainline reachable from no binary, and the pipeline map misstated two stages' reality for days. Nothing watches the gap between what landed and what is actually wired, claimed, and current.
- **For whom** — The factory's operator and its delegated drives (Argus), who today discover rot only by accident; downstream, the engine's stage-11 report.
- **Why now** — The rot class just materialized concretely (#514, #515, PR #519) and the pipeline's stage 11 is the last front-half stage with no first slice; the wake-tick pattern makes "periodic" cheap and safe to ship.
- **Direction** — A read-only sweep with a closed set of three mechanical detectors — unwired packages, frozen-plan Touches under-delivery, living-rollup staleness — emitting typed findings with named remedies; one bounded pass, on demand or host-scheduled; consumers dispose findings.
- **Appetite** — One small plan (≤ 6 tasks), stdlib + go/git subprocess reads only; excludes semantic detectors, mutation, new dependencies.
- **Success** — M1: 3/3 replay detections — the two attested historical instances (unwired packages; Touches under-delivery) plus the named constructed staleness fixture; M2: zero false positives on the live sweep at the adoption checkpoint.
- **Guardrail** — Zero mutations of any kind, ever (repo, tracker, network) — the sweep is structurally read-only.
- **Top risks** — False positives erode trust and the sweep gets ignored; `go list` reachability misclassifies deliberately library-only packages (waiver surface exists for exactly this); Touches' planning-metadata semantics drift makes the under-delivery signal misread as a breach claim.

## 2. Problem space

### 2.1 Problem statement

When implementation lands out of step with its contracts and surfaces — organs merged but never wired, planned surfaces never shipped, living rollups left describing an older system — nobody notices until the divergence bites (a feature silently absent, a map row directing a drive wrong). The cost of doing nothing is recurring: this session alone spent an operator-scale audit rediscovering by hand what a mechanical pass finds in seconds, and the factory's trust surfaces (maps, rollups) quietly rot in between. (brainstorm:maintenance-sweep#D-1; problem:maintenance-sweep#A-1)

### 2.2 Users and segments

| Segment | Role and context | Domain exp. | Tech exp. | Key goals | Pain today | Evidence |
|---|---|---|---|---|---|---|
| **P1 (primary)** | The operator (@mdarabi) reviewing factory health between features | master | master | know about rot before it bites; dispose findings cheaply | rot found only by accident or hand-audit | #514/#515/PR #519 (brainstorm:maintenance-sweep#F-5) |
| P2 | An Argus drive at a touchpoint, turning findings into issues/problems | master | master | a mechanical rot feed to dispose | hand-auditing wiring during drives | delivery:maintenance-sweep#E-1 |
| P3 (future) | The engine's stage-11 report consumer | — | — | a machine-readable findings feed | stage 11 unbuilt | docs/product-vision.md §4 row 11 (F-1) |
| **Anti-segment** | PR authors mid-proposal — the sweep never gates or evaluates a proposal | — | — | — | — | brainstorm:maintenance-sweep#D-5, O-2 |

### 2.3 Jobs and desired outcomes

| # | When… (situation) | I want to… (motivation) | So I can… (outcome) | Today's workaround | Evidence |
|---|---|---|---|---|---|
| J1 | after features land | know if any first-party package is reachable from no shipped binary | catch dead-feature rot before users or drives assume the feature works | hand-run `go list -deps` and diff by eye (this session) | brainstorm:maintenance-sweep#F-5 |
| J2 | after a frozen plan's tasks close | know if task surfaces the plan named never materialized | catch charter under-delivery early | none — job goes undone (T4/T8 gap sat unnoticed) | F-5 |
| J3 | periodically | know if the living rollups lag the shipped structure | trust docs/techspec.md and docs/prd.md as current | the retired FR-10 gate's vacancy; git-history spot checks | F-2 |
| J4 | on any finding | get a named remedy with evidence | dispose it (fix, waive, or file) without re-deriving the analysis | re-investigate from scratch | D-3 |

### 2.4 Evidence

| Claim | Source (research, data, tickets, interviews) | Confidence |
|---|---|---|
| The rot class is real and recurring: a feature's full implementation landed unwired; plan Touches named surfaces that never shipped; map rows lagged for days | #514, #515, PR #519; delivery:maintenance-sweep#E-1 (brainstorm:maintenance-sweep#F-5) | High |
| Rollup freshness is an unowned convention since amendment 017 retired the per-PR gate | docs/prds/docgate/effective.md FR-10 *(retired (017))* (F-2) | High |
| No existing mechanism scans factory state periodically: codegate is proposal-scoped; docgate gates documents at push/CI | internal/verify/checks/; internal/docgate/gate/ (F-3) | High |
| One bounded, nothing-resident pass with a host-scheduler binding is a proven, cheap pattern here | wake-channel-v1 §3.2 and its landed scheduler shim (F-6) | High |

### 2.5 The journey today

| Phase | User does | User feels / pain | Frontstage (what user sees) | Backstage (systems, people, processes) |
|---|---|---|---|---|
| suspect | operator senses something is off (a feature "should exist") | unease | nothing — no surface | none |
| audit | hand-runs go list / git log / plan diffs | expensive, ad-hoc | terminal archaeology | this session's #514/#515 audit |
| discover | finds rot days-to-weeks late | distrust of maps/rollups | stale surfaces | none |
| dispose | files issues, fixes by hand | reactive | issues | the normal fix path |

### 2.6 Opportunity and why now

- **Size** — every landed feature × every plan × the two living rollups; one missed wiring gap cost this session an operator-scale audit plus days of a feature being silently absent.
- **Alternatives users have** — hand-audits (unreliable, unscheduled); waiting for breakage (the status quo); folding checks into per-PR gates (retired shape, wrong moment — 017).
- **Why us / strategic fit** — stage 11 is the vision's own row; the detectors mechanize exactly the audits the factory already performs by hand; the sweep seeds the engine-era drift sentinel.
- **Why now** — fresh concrete evidence (#514/#515/#519); the scheduling substrate (wake-tick pattern) just landed; the front half of the pipeline is otherwise built.
- **Cost of doing nothing** — recurring hand-audits, silently absent features, decaying trust in the map and rollups.

**Decision:** Problem gate.

| Reviewer | Role | Date | Verdict |
|---|---|---|---|
| Argus | decision principal (tier-2) — `sign-gate-verdict` granted; journaled at delivery:maintenance-sweep#E-3; the operator overturns through the normal channels | 2026-07-19 | approved |

*capture: 0m · items: none — agent-principal gate; the operator's review arrives via the introducing PR's overturn channel (problem:maintenance-sweep#A-1)*

## 3. Goals, metrics, and guardrails

### 3.1 Goals as falsifiable hypotheses

| # | We believe that… | Will result in… | Measured by… |
|---|---|---|---|
| G1 | a mechanical periodic sweep over the three rot classes | rot surfacing as findings before humans trip over it | M1 (replay), M2 (live precision) |
| G2 | typed findings with named remedies | cheap dispositions (fix/waive/file) instead of re-derived audits | M3 |

### 3.2 Goals → signals → metrics

| Metric | Goal | Signal | Baseline | Target (at checkpoint) | Feeds from |
|---|---|---|---|---|---|
| M1 (primary) | G1 | replay detections on pinned fixtures: the two attested historical instances (FR-2 — the #514/#515 unwired trees; FR-3 — the T4/T8 Touches gap) plus the named constructed rollup-staleness fixture (FR-4) | 0/3 (nothing detects them) | 3/3 at the DoD checkpoint (§5.8 DOD-2) | EVT-1 |
| M2 | G1 | false positives among live-sweep emissions (evaluated state: a current mainline checkout, §4.1) | n/a (no sweep) | 0 at the adoption checkpoint (first five scheduled sweeps) | EVT-1 |
| M3 | G2 | findings carrying a remedy that a consumer can act on without re-analysis | n/a | 100% of emitted findings carry detector, subject, evidence, remedy | EVT-1 |

### 3.3 Guardrails and decision rule

| Guardrail metric | Threshold | Action if breached |
|---|---|---|
| Mutations performed by the sweep (repo content, tracker, network writes) | 0, structurally | any breach: stop use, fix before any further run (NR-1 is the contract) |
| Full-sweep runtime on this repo class | ≤ 60s (NFR-1) | profile and bound the offending detector; never background-daemonize |

**Ship rule (pre-registered):** M1 = 3/3 AND M2 = 0 through the adoption checkpoint → adopt (schedule it); any M2 false positive → fix or waiver-classify before the next scheduled run; M1 < 3/3 → the missing detector is not done.

### 3.4 Non-goals and waiting room

| ID | Non-goal | Why refused |
|---|---|---|
| NG-1 | No fixes, no tracker writes, no mutation of any kind from the sweep | findings are advisory by construction; mutation is the consumer's (brainstorm:maintenance-sweep#O-1, O-5, D-3) |
| NG-2 | No per-PR gating moment — the sweep never blocks or evaluates a proposal | wrong actor and moment; amendment 017 retired exactly that shape for freshness (O-2, D-1, D-5) |
| NG-3 | No semantic/LLM detectors in v1 — mechanical decision procedures only | trust is earned mechanically first; the sentinel program is a later slice (O-3, D-1) |
| NG-4 | No new third-party dependencies | repo dependency posture; the three detectors need none (O-6, D-7) |

| ID | Waiting room | Revisit when / intended version |
|---|---|---|
| WR-1 | pipeline-map row drift detection (semantic row→mechanism mapping) | when a mechanical mapping exists or the LLM-era sentinel slice opens (brainstorm:maintenance-sweep#G-1) |
| WR-2 | symbol-level dead-code detection (unused exported identifiers) | with the analysis-tooling dependency decision (G-2) |
| WR-3 | findings auto-filing tracker issues (dedup discipline included) | stage-12/engine-era policy decision (G-3) |

## 4. Solution space

### 4.1 Approach and alternatives considered

The sweep is one bounded, read-only pass: evaluate a closed set of mechanical detectors over the **evaluated state** — defined once as the committed state (HEAD) of the invoked repository's checkout, the one definition every detector, metric, and bar shares; mainline semantics are obtained by invoking on a current mainline checkout, as the live bars do — emit one typed finding per detected condition (detector, subject, evidence, named remedy) on a deterministic dual-face output, and exit — invokable on demand and bindable to the host scheduler exactly like the wake tick. Waivers make deliberate exceptions visible rather than silent. (brainstorm:maintenance-sweep#D-1..D-4)

| Option | Sketch | Why not chosen |
|---|---|---|
| A (chosen) | standalone periodic sweep, three mechanical detectors, findings-only | — |
| B | fold detectors into docgate/codegate gates | wrong moment (per-PR punishes the wrong actor for repo state); 017 retired that shape (NG-2) |
| C | the full drift-sentinel program (design-vs-architecture, capability-vs-tests, …) | semantic judgment, engine-era substrate; parked as the program this slice seeds (D-1) |
| Do nothing | — | recurring hand-audits and silently absent features (§2.6) |

### 4.2 Scope perimeter

- **In (plan of record)** — the sweep pass; the three detectors (unwired packages, plan-Touches under-delivery, rollup staleness); the typed findings schema + dual-face output; the waiver surface for unwired-packages; the scheduler shim + docs; replay fixtures for the historical instances.
- **Future (believed next, not committed)** — WR-1..WR-3; more detectors as evidence accumulates; the engine-era stage-11 report.
- **Out** — everything in §3.4.

### 4.3 Experience: flows and states

**Flow F1 — on-demand sweep (serves J1–J4)**

| Step | User does | System responds | States covered | Design |
|---|---|---|---|---|
| 1 | invokes the sweep on a repo root | one bounded pass; per-detector findings stream to the human face; machine face carries the typed report; exit code states the outcome | ideal (findings) · empty (clean — explicit "0 findings" line, never silence) · partial (one detector faulted, named; others complete) · error (unreadable repo state — named defect, no partial silent output) | tech design (P-2) |

**Flow F2 — scheduled sweep (serves J1–J3)**

| Step | User does | System responds | States covered | Design |
|---|---|---|---|---|
| 1 | operator installs the host schedule (shim, wake-tick pattern) | each tick = one F1 pass; capture/retention of scheduled output is the scheduler binding's concern, routed to the design with P-5 — the sweep itself stays stateless (§5.4) | as F1 | tech design (P-5) |

- **Empty** — a clean sweep prints an explicit zero-findings line per detector; silence is never the success signal.
- **Loading** — n/a (a bounded batch pass; progress is per-detector output order).
- **Partial** — a faulted detector is a named defect in the output; remaining detectors still complete and report (resilient pass).
- **Error** — an unreadable repo (not a git repo, go toolchain absent) is a named defect and a distinct exit code; no finding output is emitted that could be mistaken for a clean sweep.
- **Accessibility / Localization** — n/a: developer CLI tooling, English-only like the rest of the factory's tools.

### 4.4 Scenarios

**The dead feature.** A feature train merges its organs but the wiring PR never lands. The next sweep emits `unwired-package` findings naming the packages, the evidence (`no cmd/ binary reaches them`), and the remedy (wire or waive). The operator files the wiring issue in minutes instead of discovering the gap weeks later — the #514/#515 story, mechanized.

**The honest waiver.** A conformance-fixture package is deliberately library-only. The first sweep flags it; the operator adds it to the waiver surface with a reason; every later sweep reports the waiver as applied — visible, not silent.

**The stale rollup.** Structure ships for two weeks with no techspec touch. The sweep emits a `stale-rollup` finding with the age delta and the newest structural change as evidence. A one-line freshness PR closes it.

**The under-delivered charter.** A frozen plan's task named a shim in Touches; the task's PR merged without it. The sweep emits an advisory `touches-underdelivery` finding citing the plan and task — a signal to reconcile, framed per Touches' planning-metadata semantics, never a breach verdict.

**Decision:** Solution gate — zero TBDs.

| Reviewer | Role | Date | Verdict |
|---|---|---|---|
| Argus | decision principal (tier-2) — `sign-gate-verdict` granted; journaled at delivery:maintenance-sweep#E-3; the operator overturns through the normal channels | 2026-07-19 | approved |

*capture: 0m · items: none — agent-principal gate; the operator's review arrives via the introducing PR's overturn channel (problem:maintenance-sweep#A-1)*

## 5. System requirements

### 5.1 Functional requirements

| ID | Requirement (EARS) | Rationale | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|
| FR-1 | When invoked, the sweep shall evaluate exactly its closed detector set over the evaluated state (§4.1) and emit one typed finding per detected condition, each carrying detector, subject, evidence, and a named remedy. | the findings contract is the product (brainstorm:maintenance-sweep#D-3) | Must | low | on the fixture corpus, every emitted finding carries all four fields and cites a real condition; no detector outside the closed set runs | Test | G1, G2, J4, F1 |
| FR-2 | When the unwired-packages detector runs, the sweep shall report each first-party package that no shipped binary's import graph reaches, excluding waived packages. | the dead-feature class (#514/#515) (D-2) | Must | low | on the pinned replay fixture of the pre-wiring mainline, the lane and wake package trees are reported; on a fixture with full wiring, zero unwired findings | Test | G1, J1, F1, M1 |
| FR-3 | When the touches-underdelivery detector runs, the sweep shall report, for each frozen plan task, every Touches surface absent from the evaluated state (§4.1), phrased as an advisory reconciliation signal. | the charter under-delivery class (T4/T8) (D-2; F-4's semantics) | Must | low | on the pinned replay fixture, the two never-shipped wiring surfaces are reported citing plan + task; a fully-delivered plan fixture yields zero | Test | G1, J2, F1, M1 |
| FR-4 | When the rollup-staleness detector runs, the sweep shall report each living rollup whose last change predates the newest shipped-structure change by more than the configured bound. | the post-017 freshness vacancy (D-2; F-2) | Must | low | on a fixture where structure shipped after the rollup's last touch beyond the bound, the rollup is reported with the age delta; within the bound, zero | Test | G1, J3, F1, M1 |
| FR-5 | Where a waiver names a package, the sweep shall suppress that package's unwired finding and report the waiver as applied with its recorded reason; a waiver entry carrying no reason shall refuse to load under UB-2's closed parse. | deliberate exceptions stay visible — and justified — never silent (D-2) | Must | low | a waived package produces an applied-waiver line surfacing the waiver's reason and no finding; removing the waiver restores the finding; a reasonless waiver entry faults the load (UB-2) | Test | J1, F1 |
| FR-6 | The sweep shall produce a byte-identical machine face and identical exit codes for identical repository state and configuration; volatile wall-clock fields (duration) carry on the human face only, never the machine face. | findings feed dispositions and diffs; nondeterminism kills trust | Must | low | two consecutive runs on a pinned fixture produce byte-identical machine-face output, and the machine face contains no wall-clock field | Test | G2, F1 |
| FR-7 | The sweep shall complete each invocation as one bounded pass with no resident process, and shall be bindable to the host scheduler through a delegate-only shim. | the wake-tick pattern; stage 11's "periodic" without a daemon (D-4; F-6) | Must | low | process table shows zero sweep processes between runs; the shim passes the repo's shim test conventions (delegate-only, bash-3.2-clean) | Test | F2 |
| FR-8 | The sweep shall expose its outcome as distinct exit codes: clean, findings-present, and fault. | scriptability for schedules and drives | Must | low | fixture matrix: clean fixture → clean code; finding fixture → findings code; broken-repo fixture → fault code | Test | F1, F2 |

**Non-regression (NR-n)**

| ID | Existing behavior protected | Why protected | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|
| NR-1 | The repository, tracker, and network state are untouched by a sweep: no content writes, no tracker writes, no network writes | the read-only contract is the safety property (NG-1) | low | across the full test corpus, repo `git status` + worktree bytes are identical before and after; the sweep holds no credentialed client and opens no write path | Test | G1 |
| NR-2 | docgate and codegate behavior is unchanged by the sweep's existence | disjoint moments stay disjoint (NG-2) | low | docgate's and codegate's own suites pass unchanged with the sweep present; no shared mutable state exists | Test | — |

### 5.2 Unwanted behavior and failure handling

| ID | If… (trigger) | Then the system shall… | User sees | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|---|
| UB-1 | If the repository state is unreadable (not a git repo; go toolchain absent) | refuse the pass with a named defect and the fault exit code, emitting no finding output | a named defect, nothing mistakable for a clean sweep | Must | low | broken-repo fixtures produce the fault code and zero finding lines | Test | F1 |
| UB-2 | If the waiver or configuration surface is malformed or carries unknown keys | refuse to load it with a named defect (closed parse) and treat the pass as faulted rather than silently sweeping without waivers | the named parse defect | Must | low | malformed-waiver fixtures fault with the defect naming the line; no finding output emitted | Test | F1 |
| UB-3 | If one detector faults mid-pass | complete the remaining detectors and report the faulted one as a named defect alongside their findings | partial results + the named fault | Must | low | fault-injected fixture: remaining detectors' findings present, the fault named, findings-present-or-fault exit code honest | Test | F1 |
| UB-4 | If a frozen plan's machine face is unparsable | skip that plan loudly as a named defect and continue the detector over the rest | the named defect naming the plan | Must | low | corrupted-plan fixture: defect present, other plans' results present | Test | F1 |
| UB-5 | If the invocation carries an unknown flag or a flag-like positional | refuse with a usage defect and the fault exit code before evaluating any detector (incident I3: consequential CLIs fail closed on misparse) | the usage refusal | Must | low | misparse fixtures (unknown flag; flag-like positional) produce the fault code and zero finding lines | Test | F1, F2 |

### 5.3 Permissions and abuse cases

No new roles, permissions, or credentialed surfaces exist — the sweep is read-only local tooling holding no credential (NR-1). The one new surface is the waiver file, whose abuse case (waiving away real rot) is mitigated contractually rather than by permission: every waiver requires a recorded reason and every applied waiver is surfaced on each sweep's output (FR-5), so suppression is always visible and attributable in review; no further permission machinery is warranted for a T2 read-only surface.

### 5.4 Data

Skipped: the sweep creates no state beyond its emitted report (stdout/exit codes); no entity, retention, migration, or privacy surface exists. The waiver surface is configuration read, not data created. AP-28 posture: findings output is a regenerable projection of repository state (FR-6's determinism) — the judgment worth keeping lives in the committed waiver surface, so no committed event record is owed by v1; a committed drift-snapshot record is engine-era ground (WR-territory), decided there.

### 5.5 Interfaces and integrations

| Interface | Direction | Purpose | Contract (format, ranges, timing) | Failure modes | Owner |
|---|---|---|---|---|---|
| go toolchain + git (subprocess reads) | out (read-only) | package reachability; history ages | `go list` / `git log` invocations; bounded by NFR-1 | toolchain absent → UB-1 fault | operator's host |
| frozen plans' machine face (docs/plans/*/NNN-plan.json) | in | Touches per task | the plan dialect's committed JSON | unparsable → UB-4 named skip | plangate |
| the config/waiver surface | in | waivers; staleness bound | closed parse; unknown keys are errors (UB-2) | malformed → UB-2 fault | this feature (design names the home — §7.2 routes P-3) |
| host scheduler (cron/launchd) | in | periodic invocation | the shim's delegate-only contract (FR-7) | schedule absent → on-demand only | operator |

### 5.6 Non-functional requirements

| ID | Area | Requirement | Fit criterion | Verify |
|---|---|---|---|---|
| NFR-1 | Performance | The sweep shall complete a full pass on a repository of this repo's class within its stated bound. | full pass ≤ 60s on this repo at the DoD checkpoint | Test |
| NFR-2 | Cost / operability | The sweep shall state its steady-state cost: one bounded pass per invocation, zero between invocations. | the statement present in the sweep's docs; process table zero between runs | Inspection |
| NFR-3 | Operability | The sweep shall emit both a human-readable face and a machine-readable face of the same findings. | on the fixture corpus both faces carry the same findings; the machine face parses under its published schema | Test |

### 5.7 Instrumentation — the tracking plan

| ID | Event name | Fires when | Properties (typed) | Source | Feeds |
|---|---|---|---|---|---|
| EVT-1 | sweep_completed | each pass ends | per-detector finding counts: int; waivers applied: int; faults: int; outcome: enum(clean·findings·fault) — deterministic fields only; wall-clock duration appears on the human face only, excluded from the machine face by FR-6 | the sweep's machine face (the report is the event record; no second telemetry plane) | M1, M2, M3 |

### 5.8 Definition of Done

| ID | Done criterion | Source | Verify | Required evidence | Blocking? |
|---|---|---|---|---|---|
| DOD-1 | Every Must FR passes its fit criterion | FR-1..FR-8 | Test | the test-suite run | yes |
| DOD-2 | The replay bar: the two historical rot instances and the constructed staleness fixture detected on their pinned fixtures (M1 = 3/3) | §3.2 M1; FR-2..FR-4 | Test | the replay-fixture suite run | yes |
| DOD-3 | The live bar: a full sweep whose evaluated state (§4.1) is a current mainline checkout completes; every emission is a verified true condition or an explicitly waiver-classified one; zero mutations observed (M2 = 0 at this moment) | §3.2 M2; §3.3; NR-1 | Test + Analysis | the live sweep report + the per-emission verification notes riding the introducing PR/epic | yes |
| DOD-4 | No non-regression row broken | NR-1..NR-2 | Test | suite results | yes |
| DOD-5 | The scheduler shim exists, is delegate-only, and its shim test is green; the scheduling binding is documented | FR-7 | Test + Inspection | the shim test run + the doc | yes |

### 5.9 AI behavior and evals

Skipped: no model-driven behavior ships in v1 (NG-3).

## 6. Delivery

### 6.1 Phasing and milestones

| Phase | Purpose | Scope | Exit criterion |
|---|---|---|---|
| Build + verify (single phase) | ship the slice whole | §4.2 "In" | §5.8 all green |
| Adoption checkpoint | earn scheduled status | first five scheduled sweeps on this repo | M2 = 0 across them; §6.7's verdict recorded |

### 6.2 Dependencies

| Dependency | Type (team / system / vendor / approval) | Needed by | Status | Owner |
|---|---|---|---|---|
| go toolchain + git on the host | system | build+verify | confirmed (repo baseline) | operator |
| frozen plans' machine face (the plan dialect) | system | FR-3 | confirmed (plangate ships it) | plangate |
| the technical design resolving P-1..P-5 | approval (stage 3–5) | build | pending | this drive |

### 6.3 Risks, assumptions, and rabbit holes

| # | Risk / assumption / rabbit hole | Likelihood | Impact | Mitigation or test | Owner |
|---|---|---|---|---|---|
| R1 | false positives erode trust; the sweep gets ignored | M | H | the zero-FP live bar (M2, DOD-3); the waiver surface makes exceptions first-class (FR-5) | Argus |
| R2 | reachability misclassifies deliberately library-only packages | M | M | FR-5's waiver + the applied-waiver visibility; first live sweep triages the initial set (DOD-3) | Argus |
| R3 | Touches' planning-metadata semantics get misread as delivery breach claims | L | M | FR-3's advisory phrasing is contract; the glossary term (T-2) fixes the vocabulary | Argus |
| R4 | rabbit hole: perfect staleness semantics (what counts as "structure") | M | M | v1 pins a simple mechanical definition in the design; refinement is a later amendment, not scope creep now | Argus |

### 6.4 Release criteria

| Lens | Criterion | Evaluated by |
|---|---|---|
| Functionality | all Must FRs pass fit criteria (DOD-1) | the test suite |
| Usability | a finding is disposable without re-analysis (M3 = 100%) | the fixture corpus + the live report |
| Reliability | UB-1..UB-5 drills pass; faults never masquerade as clean | the failure + misparse fixtures |
| Performance | NFR-1 within bound | the timed suite |
| Supportability | NFR-2/NFR-3 (stated cost; dual faces) | inspection + tests |

### 6.5 Rollout and reversibility

- **Flag** — n/a: local read-only tooling with no staged exposure; "rollout" is the operator choosing to install the schedule (its absence is the off state).
- **Kill switch** — n/a beyond removing the schedule/not invoking: the sweep holds no persistent or acting state to kill (NR-1).
- **Experiment** — n/a: no user-facing variant surface.
- **Cutover/migration** — n/a: no data, no predecessor system.

| Stage | Audience | Minimum evidence | Advance when | Roll back when |
|---|---|---|---|---|
| on-demand | operator + drives | DOD-1..DOD-5 green (the §5.8 DoD set) | §6.7 trigger reached | any M2 false positive → fix/waive before next run |
| scheduled | the host schedule | five scheduled sweeps | M2 = 0 across them → adopt | repeated FPs → unschedule, keep on-demand |

### 6.6 Launch readiness

| Area | Item | Owner | Status |
|---|---|---|---|
| Docs | the sweep's runbook section (invocation, waivers, schedule binding) | Argus | ☐ |
| Support / Enablement / Legal / Pricing / Marketing / Platform / Localization | n/a — internal factory tooling; no external users, terms, or billing | — | n/a |

**Decision:** Launch gate — collapses into §5.8 + §6.7 for internal tooling (rule 15): the DoD is the bar; the adoption verdict is the launch review.

| Reviewer | Role | Date | Verdict |
|---|---|---|---|
| Argus | decision principal (tier-2); the operator overturns through the normal channels | 2026-07-19 | go (conditional on §5.8 green at build time) |

### 6.7 Post-launch review

- **Review trigger** — five scheduled sweeps completed on this repo (observable exposure, no dates) · **Owner** — @mdarabi (operator), with the drive assembling the evidence
- **Verdict recorded** — adopt / iterate / kill with M1/M2/M3 evidence — on the header's Tracking ref

## 7. Alignment

### 7.1 FAQ

**Customer** *(the operator and drives are the customer)*

- **Q:** Will this block my PRs or add CI time? — **A:** No. The sweep never gates a proposal (NG-2); it runs in its own moment, on demand or on the host schedule.
- **Q:** What happens when it flags something deliberate? — **A:** Add a waiver; the sweep reports the waiver as applied from then on — visible, never silent (FR-5).

**Internal**

- **Q:** Why not put these checks in docgate/codegate? — **A:** Wrong moment and actor: per-PR gates punish the proposal author for pre-existing repo state; amendment 017 retired exactly that shape for freshness (NG-2, F-2).
- **Q:** Why only three detectors? — **A:** Each shipped detector must clear the mechanical-honesty and zero-FP bars; the parked ones (WR-1..WR-2) currently cannot (§7.3 decision-log r2; brainstorm:maintenance-sweep#D-2).
- **Q:** Top failure modes? — **A:** FP-driven ignoring (R1); waiver rot — waivers accumulating past their reasons (mitigated by applied-waiver visibility); under-delivery findings read as accusations (R3).

### 7.2 Open questions

| # | Question | Owner | Needed by | Status |
|---|---|---|---|---|
| Q1 | Can map-row drift be detected mechanically (row→mechanism mapping), and is it worth a slice? | operator + a later brainstorm | WR-1's revisit | open (brainstorm:maintenance-sweep#G-1) |
| Q2 | Symbol-level dead code: is an analysis-tooling dependency acceptable? | operator (dependency posture) | WR-2's revisit | open (G-2) |
| Q3 | Should findings ever auto-file issues, under what dedup discipline? | operator (policy) | WR-3's revisit | open (G-3) |
| Q4 | The waiver/config surface's concrete home and the findings schema's face formats | the technical design | stage 4 | open (routed: brainstorm P-2, P-3) |

### 7.3 Decision log

| Date | Decision | Alternatives considered | Why | Who |
|---|---|---|---|---|
| 2026-07-19 | one feature: the stage-11 first mechanical slice, its own moment | the sentinel program; docgate-gate extension | evidence is mechanical; 017 retired the per-PR shape (brainstorm:maintenance-sweep#D-1) | Argus (principal, tier-2) |
| 2026-07-19 | exactly three detectors, closed | wider sets incl. map-row drift, dead symbols | mechanical-honesty bar; appetite (D-2) | Argus |
| 2026-07-19 | findings, never fixes; typed, deterministic | auto-fix; auto-filing | blast radius zero; AP-17 (D-3) | Argus |
| 2026-07-19 | on-demand + schedulable, nothing resident | daemon; CI-coupled | the wake-tick pattern (D-4) | Argus |
| 2026-07-19 | consumers: operator + drives + future engine; anti-segment: PR authors | per-PR surfacing | stage-11's Factory actor; 017 (D-5) | Argus |
| 2026-07-19 | success = replay 3/3 + live zero-FP | best-effort tuning later | trust economics of advisory tooling (D-6) | Argus |
| 2026-07-19 | appetite: ≤ 6 tasks, stdlib-only | richer tooling | dependency posture (D-7) | Argus |

### 7.4 Changelog

| Date | Change | Author |
|---|---|---|
| 2026-07-19 | 000-base authored from the gated record brainstorm:maintenance-sweep | Argus (decision principal, tier-2) |

## 8. Contract readiness — the executability gate

| Check | Pass? | Notes |
|---|---|---|
| Problem basis accepted — §2 problem gate signed | ✅ | Argus-signed (tier-2), journaled delivery:maintenance-sweep#E-3; operator overturn channel open |
| Zero solution-blocking TBDs or open questions (§7.2) | ✅ | Q1–Q3 are waiting-room policy questions; Q4 is design-owned (stage 4), not solution-blocking |
| Every Must FR has a fit criterion and a verification method (§5.1) | ✅ | FR-1..FR-8 |
| Every requirement row carries a risk tier; high/critical rows have a declared safety plan | ✅ | all rows low risk (read-only advisory tooling); no high/critical rows exist |
| Non-goals drawn (§3.4); non-regression rows named (§5.1) | ✅ | NG-1..NG-4; NR-1..NR-2 |
| Every primary job (§2.3) is covered by a requirement, a non-goal, or a waiting-room entry | ✅ | J1→FR-2/FR-5; J2→FR-3; J3→FR-4; J4→FR-1 |
| Every key flow (§4.3) has functional, failure, and non-regression coverage | ✅ | F1→FR-1..FR-8/UB-1..UB-5/NR-1; F2→FR-7/FR-8/UB-5 |
| Every primary metric (§3.2) has instrumentation (§5.7) | ✅ | M1..M3 ← EVT-1 |
| Every blocking DoD row traces to a requirement, guardrail, release criterion, or rubric | ✅ | DOD-1..DOD-5 sourced by reference (the glossary landing is not a DoD row: already landed in this write set and journaled at delivery:maintenance-sweep#E-2) |
| Risks and assumptions have owners (§6.3) | ✅ | R1..R4 |
| Human-judgment criteria name an owner and a rubric (§5.8) | ✅ | vacuously — no human-judgment rows (all DoD rows machine-verifiable by design) |
| DoD complete: every blocking row sourced and evidenced; every n/a explained | ✅ | no n/a rows |
