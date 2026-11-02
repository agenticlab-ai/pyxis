# Maintenance sweep v1 — one read-only pass, three mechanical detectors, one policy file — technical design

| Field | Value |
|---|---|
| **Status** | Approved |
| **Owner** | @mdarabi (operator) |
| **Approver(s)** | @mdarabi (operator) — the introducing PR is session-merged under problem:maintenance-sweep#A-1; the operator's review is the standing overturn channel |
| **Date** | 2026-07-19 |
| **Relevant-when** | designing or changing the maintenance sweep — its binary/package home, detectors, findings schema, policy file, exit codes, or scheduler binding; implementing against it; triaging a sweep false positive, a missed detection, or a policy-parse fault; superseding it |
| **Scope** | The stage-11 first slice's realization: the sweep's home and pass structure, the three detectors' mechanical decision procedures, the findings/report contract, the policy surface (waivers + staleness bound), fixtures, and the scheduling shim. **Not**: additional detectors (the basis's waiting room WR-1..WR-3); the engine-era stage-11 report; any mutation or tracker surface (NG-1); per-proposal gating (NG-2). |
| **Tracking** | — *(the introducing PR, once filed)* |
| **Builds-on** | docs/tech-design/wake-channel-v1.md (the bounded-pass + delegate-only shim pattern this reuses) · docs/tech-design/docgate-v1.md (the dual-face + closed-parse + defects-as-data conventions) · docs/tech-design/solution-shape-v1.md (the design↔shape partition this instantiates) |
| **Supersedes** | — |
| **Serves** | prd:maintenance-sweep/FR-1 · prd:maintenance-sweep/FR-2 · prd:maintenance-sweep/FR-4 · prd:maintenance-sweep/FR-6 · prd:maintenance-sweep/FR-7 · prd:maintenance-sweep/FR-8 · prd:maintenance-sweep/UB-1 · prd:maintenance-sweep/UB-2 · prd:maintenance-sweep/UB-3 · prd:maintenance-sweep/UB-4 · prd:maintenance-sweep/UB-5 · prd:maintenance-sweep/NR-1 · prd:maintenance-sweep/NR-2 · prd:maintenance-sweep/NFR-1 · prd:maintenance-sweep/NFR-2 · prd:maintenance-sweep/NFR-3 · prd:maintenance-sweep/FR-9 · prd:maintenance-sweep/FR-10 |
| **Security/privacy impact** | none — read-only local tooling; no credential, no network, no identity/secret/user-data surface (see §3.5) |
| **Template version** | tech-design-template/v1 |

## 0. Read to rule — the human reviewer's section (~3 min)

*The body is the contract; this section only routes attention — on any conflict
the body governs; machines ignore it.*

**Judge these**

| # | The call | Could have gone | If you disagree |
|---|---|---|---|
| 1 | A new dedicated binary + one focused package, not a docgate or wake-tick verb | design-brainstorm:maintenance-sweep-tech-design#ALT-2, #ALT-3 | veto re-couples the sweep's faults to a gate or the wake guarantee — rewrite §2, §3.1; NR-2's disjointness stops being structural |
| 2 | Findings ride a NEW closed report schema; docgate defects carry only faults | #ALT-5 | veto conflates advisory findings with gate defects (R3's misread risk becomes structural) — rewrite §3.3's schema decision, DINV-5 |
| 3 | One policy file owns waivers (package + plan) AND the staleness bound, reason-required | split homes (config key per knob) | veto scatters one feature's policy across surfaces — rewrite §3.4; FR-10's visibility contract weakens |
| 4 | Touches detection reads only path-like tokens and is plan-waivable | interpret every token; no plan waivers | veto either invents semantics for tokens like "pipeline-map" or floods the live sweep with in-flight-plan noise (M2 dies) — rewrite §3.2's procedure, DINV-3 |

**Sanity-check the numbers** — exit codes 0/1/2 (§3.3, A-6); staleness default 14 days, policy-overridable (§3.4); full pass ≤ 60s (A-7, prd NFR-1); fixture bar 3/3 replays (A-2..A-4).

**The author's open questions** — Q1 (test-only harness packages: waivers-only in v1, auto-exclusion revisited on waiver-volume evidence) — §7.

**Skip the rest** — schema/structure → docgate (lint-tech-designs, check-design-shape, check-shape-consistency); substance/coherence/reuse → the unmanned tech-design-review (its report rides this PR); implementation defects → codegate at stage 8; rationale → §4.

## 1. Problem

The frozen PRD (prd:maintenance-sweep, as of 000) contracts stage 11's first slice — three mechanical rot detectors, typed findings, read-only, on-demand + schedulable — and deliberately routes the realization here (its Q4; the stage-1 record's P-1..P-5): where the sweep lives, what its findings concretely look like, where waivers and bounds live, the detectors' exact decision procedures, how the historical replays are pinned, and the scheduler binding. Undecided, an implementer either invents these (unreviewable drift) or stalls. The design questions were ground through the gated stage-3 record (design-brainstorm:maintenance-sweep-tech-design — two panels fired, six rulings); this document freezes the decisions and points at its non-frozen shape for every concrete schema.

## 2. Decision

**Decision:** The maintenance sweep is **its own thin binary over one focused package** — never a verb on the document gate or the wake tick — running **one bounded, read-only pass**: load the single closed-parse **policy file** (waivers and bounds; reason-required entries), evaluate the **three detectors** as pure functions of the evaluated state (prd §4.1), and emit **one deterministic report** — a new closed machine-face schema on stdout, a human face on stderr, exit codes clean/findings/fault — then exit, nothing resident. Concrete schemas, paths, and the CLI grammar live in the paired non-frozen shape `shape:maintenance-sweep-v1`; this document carries only the decisions and invariants. (design-brainstorm:maintenance-sweep-tech-design#D-1, #D-2, #D-3)

## 3. The design

### 3.1 The pass and its home

**Decision:** One binary, named for the glossary term, delegating to one focused package that owns the policy load, the three detectors, the report assembly, and the exit-code mapping (#D-1). The pass order is fixed: parse invocation (fail-closed on misparse — incident I3's lesson, prd UB-5) → load policy (closed parse; absent = empty policy + default bound, a legal advisory-tool state; malformed = fault, prd UB-2) → run detectors in fixed order, each isolated (a detector fault is named in the report and the pass continues — prd UB-3) → emit the report + exit code. The concrete package and binary names, flags, and layout are the shape's (shape:maintenance-sweep-v1).

### 3.2 The three detectors — mechanical decision procedures

**Decision:** Each detector is a pure function of (evaluated state, policy) with a closed procedure (#D-4):

- **unwired-packages** — first-party packages are the module's own package list; shipped reachability is the union of the import graphs of every main under the binaries' tree; a first-party non-main package outside that union is unwired unless a package waiver names it. Evidence: the package path + the empty reaching set; remedy: wire or waive.
- **touches-underdelivery** — for each frozen plan's machine face, each task's Touches entries split on the house separator; only **path-like tokens** (containing a slash or a dot) are evaluated; a token naming no file and no directory prefix in the evaluated state is reported, citing plan + task, phrased as the advisory reconciliation signal the PRD binds (prd FR-9). Non-path tokens are skipped, never interpreted. A **plan waiver** (reason-required, contracted at prd FR-10 with FR-9's exclusion clause — amendment 001) suppresses a plan's findings and surfaces the reason — the in-flight-plan control (the concrete case: a freshly frozen plan mid-execution).
- **rollup-staleness** — a living rollup is stale iff the newest commit touching the configured capability paths is newer than the rollup's newest commit by more than the policy bound (default 14 days). The inputs' homes are the existing config surface, not new policy: the rollup set is the `paths.rollup_prd` + `paths.rollup_techspec` keys and the capability paths are the existing `capability_paths` list — the pin R4's mitigation relies on. Ages derive from committed timestamps — deterministic per evaluated state, no wall clock (prd FR-6).

An unparsable plan is a named skip (prd UB-4); an unreadable repo or absent toolchain faults the pass before any detector claims cleanliness (prd UB-1).

### 3.3 The report contract

**Decision:** The pass emits exactly one report per invocation: the **machine face** — one JSON object of a new closed, versioned schema (report + finding elements; the schema is the shape's) — on stdout; the **human face** on stderr (#D-2). Findings carry the PRD's four fields (detector, subject, evidence, remedy — prd FR-1); applied waivers are surfaced with their reasons (prd FR-10); a faulted detector appears as a named fault inside the report (prd UB-3). The **exit ranking is pinned**: any finding emitted → exit **1** (a coexisting detector fault stays named inside the report, never silently absorbed — the findings are the pass's actionable product); zero findings with any fault — pass-level (usage, misparse, unreadable state, malformed policy) or detector-level — → exit **2**; zero findings, zero faults → exit **0**; the machine face's `outcome` field maps by the same ranking (findings > fault > clean). The **human face is explicit on emptiness**: each clean detector prints its explicit "0 findings" line — never silence (the PRD's F1 empty state, held in this frozen half). The report pins the evaluated state honestly: `evaluated_state.commit` names HEAD, and a non-clean working tree sets a named `dirty` annotation on the machine face plus a human-face warning — the commit then names the base of the evaluated tree, not its exact bytes; the §3.7 correlation claim and every live bar run on clean checkouts (AP-13's clean-tree corollary). Ordering is fixed (detector order, then lexicographic subjects), and no machine-face field carries wall-clock content — the report is the event record (prd EVT-1) and a regenerable projection of state (the AP-28 posture the PRD states).

### 3.4 The policy surface

**Decision:** All sweep policy lives in **one** closed-parse file — schema version, the staleness bound, package waivers, plan waivers — homed in the repository's policy directory and located through **one new `paths.*` configuration key**, the same pattern the merge lane's risk policy landed (#D-3). Every waiver entry requires a reason; a reasonless entry refuses to load (prd FR-10 · UB-2). Unknown keys are errors (the closed-parse house rule). The concrete file grammar and key name are the shape's.

### 3.5 Boundaries and non-goals

- **Not solved here** — additional detectors (map-row drift, symbol-level dead code — the basis's WR-1/WR-2); auto-filing findings (WR-3); the engine-era stage-11 report.
- **Owned by a future design** — any committed drift-snapshot record (the PRD's §5.4 routes it engine-era); a semantic Touches interpreter.
- **Implementation must not infer** — that a finding licenses a fix (findings are advisory; mutation is the consumer's — prd NG-1); that skipping non-path Touches tokens licenses interpreting them elsewhere; that the sweep may share a binary, exit surface, or schedule with the wake tick (pattern reuse ≠ cohabitation — #ALT-3's rejection).
- **Must remain unchanged** — docgate's and codegate's behavior and suites (prd NR-2); the wake tick's contract; every existing gate moment.

### 3.6 Invariants

- **DINV-1** — A sweep performs no write beyond its own stdout/stderr: no repository content, tracker, or network mutation, under any input including faults (prd NR-1's contract made structural — the package holds no credentialed client and no write-path import).
- **DINV-2** — The machine face is byte-identical for identical (evaluated state, policy): fixed ordering, no wall-clock field, no environment leakage (prd FR-6).
- **DINV-3** — Every input surface parses closed and fails loud: unknown invocation tokens, unknown policy keys, reasonless waivers, and unreadable state each produce a named defect and the fault exit code — never a silent skip, never output mistakable for a clean sweep (prd UB-1/UB-2/UB-5; incident I3).
- **DINV-4** — A detector fault never masquerades: the remaining detectors complete, the fault is named inside the report, and the exit maps by §3.3's pinned ranking — any finding → 1 (a coexisting fault named, never absorbed); zero findings with any fault → 2; zero findings, zero faults → 0 (prd UB-3/UB-4).
- **DINV-5** — Every emitted finding carries detector, subject, evidence, and remedy; every applied waiver is surfaced with its recorded reason (prd FR-1/FR-10 — suppression is always visible).

### 3.7 Observability

- **Events/logs** — the report IS the event record (prd EVT-1): per-detector finding counts, waivers applied, faults, outcome — one record per pass on the machine face; no second telemetry plane.
- **Metrics** — the PRD's M1..M3 are computed from reports and fixtures, not runtime instrumentation.
- **Alerts** — none (advisory tooling; the operator's schedule surfaces output).
- **Debugging surface** — re-run the pass: determinism (DINV-2) makes any report reproducible; the human face names each finding's evidence; a fault names its input line.
- **Trace/correlation** — the report pins the evaluated state's commit, so any report correlates to exact repository state.

### 3.8 Security & privacy

none — read-only local tooling holding no credential, opening no network path, touching no identity, secret, or user data (echoes the header row). The one judgment surface — waivers — is a committed, reason-required, review-routed file (§3.4), so suppression of findings is visible and attributable in history.

### 3.9 Testing strategy

Seams and bins: the **policy parse and report schema** are in-process (table-driven unit tests; golden machine-face fixtures for DINV-2's byte-identity). The **detectors** are local-substitutable — hermetic temp-git fixture repos built in-test (the house pattern), including the three replay fixtures (#D-5): the unwired fixture reproducing the historical dead-feature shape with its real package names; the Touches fixture reproducing the historical charter gap verbatim (a frozen-plan face naming the two never-shipped wiring surfaces); the constructed staleness fixture (prd FR-2..FR-4, DOD-2). The **toolchain reads** (go/git subprocesses) sit behind an injectable runner seam with the production runner integration-covered by the fixture repos. The **shim** is covered by the standard scripts-tests conventions (delegation, thinness, the shell floor, one hermetic dry pass — prd FR-7, DOD-5).

## 4. Alternatives considered

| Alternative | Why rejected |
|---|---|
| A docgate verb hosting the detectors (design-brainstorm:maintenance-sweep-tech-design#ALT-2) | moment/actor conflation with the document gate; NR-2's disjointness stops being structural; the basis's own D-1 refused the gate-fold framing |
| A second verb on the wake tick (#ALT-3) | binary cohabitation fuses two independently-revisable contracts (delivery guarantee vs state analysis), shares an exit surface, and forbids separate schedules — pattern reuse is the shim discipline, not cohabitation |
| Findings as docgate defect lines (#ALT-5) | semantic conflation (defect = gate failure; finding = advisory), and the defect shape lacks the finding's four contracted fields |
| Unstructured human-only output (#ALT-6) | fails the machine-face requirement, disposability, and the engine-era consumer |

## 5. Risks

| ID | Risk | Impact | Mitigation | Verification / evidence |
|---|---|---|---|---|
| R1 | false positives erode trust (the PRD's R1) | sweep ignored | the zero-FP live bar; waivers first-class + reason-surfaced (DINV-5); the plan-waiver control for in-flight plans | the live-sweep triage evidence at DoD; A-5 |
| R2 | reachability misclassifies deliberately library-only or test-harness packages | noisy unwired findings | package waivers; Q1 tracks auto-exclusion on evidence | the first live triage; A-2's fixture pair |
| R3 | Touches findings read as breach claims | mistrust of the detector | the PRD-bound advisory phrasing; path-token-only evaluation; plan waivers | A-3's fixture phrasing assertion |
| R4 | staleness semantics dispute ("what counts as structure") | contested findings | the capability-paths definition pinned here; the bound is policy data, not code | A-4's fixture matrix |

## 6. Acceptance

| ID | Acceptance criterion | Verify | Required evidence | Traces to |
|---|---|---|---|---|
| A-1 | On the fixture corpus every emitted finding carries detector, subject, evidence, remedy; no detector outside the closed set runs | Test | suite run | prd:maintenance-sweep/FR-1 · #DINV-5 |
| A-2 | The unwired replay fixture reports the historical package trees; the fully-wired fixture reports zero; a waived package yields an applied-waiver line and no finding, and removing the waiver restores it | Test | replay suite | prd:maintenance-sweep/FR-2 · prd:maintenance-sweep/FR-10 |
| A-3 | The Touches replay fixture reports the two never-shipped surfaces citing plan + task in advisory phrasing; a fully-delivered plan yields zero; a plan waiver suppresses with the reason surfaced; non-path tokens are skipped | Test | replay suite | prd:maintenance-sweep/FR-9 · prd:maintenance-sweep/FR-10 · #DINV-5 |
| A-4 | The staleness fixture matrix: beyond-bound reports with the age delta; within-bound zero; the bound read from policy | Test | fixture matrix | prd:maintenance-sweep/FR-4 |
| A-5 | Two consecutive runs on a pinned fixture produce byte-identical machine faces containing no wall-clock field | Test | golden comparison | prd:maintenance-sweep/FR-6 · #DINV-2 |
| A-6 | The exit-code matrix holds: clean 0, findings 1, fault 2 — incl. misparse (unknown flag, flag-like positional), malformed policy, reasonless waiver, unreadable repo, each with zero finding output | Test | failure-fixture matrix | prd:maintenance-sweep/FR-8 · prd:maintenance-sweep/UB-1 · prd:maintenance-sweep/UB-2 · prd:maintenance-sweep/UB-5 · #DINV-3 |
| A-7 | A fault-injected detector: remaining detectors' findings present, the fault named in the report, exit per §3.3's ranking — incl. the pinned variant where the fault coexists with zero findings (exit 2, `outcome: fault`) and with findings present (exit 1, fault named); an unparsable plan is a named skip with other plans' results present | Test | fault fixtures | prd:maintenance-sweep/UB-3 · prd:maintenance-sweep/UB-4 · #DINV-4 |
| A-8 | Before/after state comparison across the full corpus shows zero repository mutations; the package imports no write-capable client; the full pass meets the PRD's runtime bound on this repo class | Test + Analysis | corpus run + import audit + timing | prd:maintenance-sweep/NR-1 · prd:maintenance-sweep/NFR-1 · #DINV-1 |
| A-9 | The delegate-only shim passes the conventions test (thinness, bash-3.2 floor, delegation) and its header documents the scheduler binding and the steady-state cost statement (one bounded pass per invocation, zero between) | Test + Inspection | the shim test run + the header | prd:maintenance-sweep/FR-7 · prd:maintenance-sweep/NFR-2 |
| A-10 | On the fixture corpus both faces carry the same findings, and on a clean fixture the human face prints each detector's explicit "0 findings" line — never silence | Test | face-parity fixtures | prd:maintenance-sweep/NFR-3 |
| A-11 | docgate's and codegate's own suites pass unchanged with the sweep present | Test | the neighbouring suites' runs | prd:maintenance-sweep/NR-2 |

## 7. Open questions

| ID | Question | Owner | Blocks | Status | Resolution |
|---|---|---|---|---|---|
| Q1 | Should test-only harness packages be auto-excluded from unwired detection, or stay waivers-only? | the operator, on waiver-volume evidence from the first live sweeps | none (v1 ships waivers-only) | open | revisit at the adoption checkpoint |

## Principles

- **AP-5 — honored.** The report is the event record; observability is the design, not a bolt-on (§3.7).
- **AP-6 — honored.** One closed, versioned machine-face schema; dual faces; additive evolution (the shape owns the concrete grammar).
- **AP-10 — honored.** The sweep mechanizes exactly the judgment-free audits the factory performed by hand; everything semantic stays parked (basis WR rows).
- **AP-17 — honored.** Every uncertain input fails closed and loud (DINV-3); a fault never masquerades as clean (DINV-4).
- **AP-19 — honored.** One bounded pass, nothing resident, cost stated; the schedule is the host's (§3.1, prd NFR-2).
- **AP-25 — honored.** The scheduler binding is a delegate-only shim on the vendor-neutral scripts surface; the binary assumes only a shell and the repo (§3.9, shape).
- **AP-26 — honored.** Every detector reads goal objects (import graphs, files, commits) — never badges or declared state.
- **AP-28 — honored.** The report is a regenerable projection (DINV-2); the judgment worth keeping (waivers) lives in a committed, reviewed file (§3.4).
- **RW-5 — honored.** One independently-revisable decision: the sweep's realization. The detectors' subordinate procedures ride §3.2 because they only ever change with the sweep; future detectors arrive by amendment + successor designs.
- **Deviation:** none.
