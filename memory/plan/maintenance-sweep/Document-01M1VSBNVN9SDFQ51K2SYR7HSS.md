# Maintenance sweep — the read-only rot detector — plan 001

| Field | Value |
|---|---|
| **Feature** | maintenance-sweep |
| **PRD** | docs/prds/maintenance-sweep |
| **Design** | docs/tech-design/maintenance-sweep-v1.md |
| **Plan version** | 001 |
| **Dialect** | harbormaster/fs-spec:v1 |
| **Epic title** | Build the maintenance sweep — one read-only pass, three mechanical rot detectors, typed findings with waivers, schedulable like the wake tick |
| **Status** | Draft — freezes at merge |
| **Template version** | plan-template/v1 |

## Tasks

### T1 — The policy surface and the report contract

| Field | Value |
|---|---|
| **Blocked by** | — |
| **Size band** | 150–280 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | the config surface gains one additive path key; concurrent additive key edits merge cleanly |
| **Routing** | — |
| **Pattern** | — |
| **Touches** | internal/sweep/policy.go · internal/sweep/report.go · internal/docgate/config/config.go |

**Goal:** The sweep's two data contracts exist as one focused package's foundation: the closed-parse policy (waivers + staleness bound) every detector shares, and the deterministic report/finding schema every pass emits.

**Current behavior:** No sweep package exists; nothing parses a maintenance policy, and no report or finding type exists.

**Desired behavior:** The policy loads through one closed parse: schema version, the staleness bound (default fourteen days when absent), package waivers, and plan waivers — every waiver entry requires a recorded reason, and a reasonless entry, an unknown key, or a malformed file refuses to load with a named defect; an absent policy file is the legal empty policy with the default bound. The configuration surface resolves the policy's location through one new path key with the shape's default. The report and finding types realize the shape's closed schemas: fixed detector order, lexicographically ordered subjects, four contracted finding fields, applied waivers carrying their reasons, in-band per-detector faults, the evaluated-state pin with its dirty flag, and no wall-clock field anywhere on the machine face — marshalling is byte-deterministic for equal input.

**Scope:**
- the closed policy parse with reason-required waiver entries and the named-defect refusals
- the absent-file = empty-policy + default-bound legal state
- the report/finding types with deterministic ordering and marshalling
- the one additive configuration path key resolving the policy home

**Out of scope:**
- the detectors themselves (T2–T4) and the pass/binary that emits the report (T5)
- the seeded live policy file for this repository (T6 triages it from the first live sweep)

**Acceptance criteria:**
- [ ] a malformed policy, an unknown key, and a reasonless waiver each refuse to load with a named defect; an absent file loads as the empty policy with the default bound
- [ ] a well-formed policy exposes the bound, the package waivers, and the plan waivers with their reasons to a reader
- [ ] marshalling the same report value twice yields byte-identical output, and no machine-face field carries wall-clock content
- [ ] finding values refuse construction or marshalling without all four contracted fields (detector, subject, evidence, remedy)

**Key interfaces:**
- the policy type, its loader, and the named-defect surface it shares with the house defect shape
- the report and finding types (`maintenance-report/v1`, `finding/v1`) and their deterministic marshal
- the configuration key naming the policy's location

**Traces:** prd:maintenance-sweep/FR-10 · prd:maintenance-sweep/UB-2 · prd:maintenance-sweep/FR-6 · design:maintenance-sweep-v1#3.3 · design:maintenance-sweep-v1#3.4 · design:maintenance-sweep-v1#DINV-2 · design:maintenance-sweep-v1#DINV-3 · design:maintenance-sweep-v1#DINV-5

### T2 — The unwired-packages detector

| Field | Value |
|---|---|
| **Blocked by** | T1 |
| **Size band** | 120–240 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | — |
| **Pattern** | — |
| **Touches** | internal/sweep/unwired.go |

**Goal:** The dead-feature rot class is detected mechanically: every first-party package no shipped binary reaches becomes one typed finding, waivers applied and surfaced.

**Current behavior:** Nothing computes shipped reachability; a feature's organs can land reachable from no binary and stay invisible (the historical #514/#515 instances were found by hand).

**Desired behavior:** The detector computes the module's first-party package list and the union of the import graphs of every main under the binaries' tree, both through an injectable runner seam; each first-party non-main package outside that union yields one finding carrying the package path as subject, the empty reaching set as evidence, and the wire-or-waive remedy — unless a package waiver names it, in which case the finding is suppressed and the applied waiver surfaces with its recorded reason. The detector is a pure function of the evaluated state and the policy.

**Scope:**
- the reachability procedure over the toolchain's package listings behind the runner seam
- package-waiver application with applied-waiver surfacing
- the hermetic replay fixture reproducing the historical dead-feature shape by its real package names, plus a fully-wired fixture

**Out of scope:**
- test-only package auto-exclusion (design Q1: waivers-only in v1)
- the pass orchestration and exit mapping (T5)

**Acceptance criteria:**
- [ ] on the pinned replay fixture the historical package trees are reported; on the fully-wired fixture zero unwired findings emit
- [ ] a waived package produces an applied-waiver line with its reason and no finding; removing the waiver restores the finding
- [ ] every emitted finding carries all four contracted fields
- [ ] the detector performs no write and consults nothing but the evaluated state and the policy

**Key interfaces:**
- the detector's pure function signature over (evaluated state, policy) shared by all three detectors
- the runner seam the toolchain reads sit behind

**Traces:** prd:maintenance-sweep/FR-2 · prd:maintenance-sweep/FR-10 · design:maintenance-sweep-v1#3.2 · design:maintenance-sweep-v1#DINV-1 · design:maintenance-sweep-v1#DINV-5

### T3 — The touches-underdelivery detector

| Field | Value |
|---|---|
| **Blocked by** | T1 |
| **Size band** | 160–300 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | — |
| **Pattern** | — |
| **Touches** | internal/sweep/touches.go |

**Goal:** The charter under-delivery rot class is detected mechanically: every frozen plan task's path-like Touches surface absent from the evaluated state becomes one advisory finding, in-flight plans waivable.

**Current behavior:** Nothing cross-checks frozen plan charters against shipped surfaces; a task can merge without its contracted wiring and stay invisible (the historical T4/T8 instances).

**Desired behavior:** The detector reads each frozen plan's committed machine face, splits each task's Touches on the house separator, and evaluates only path-like tokens (containing a slash or a dot); a token matching no file and no directory prefix in the evaluated state yields one finding citing plan and task, phrased as the advisory reconciliation signal the contract binds — never a breach verdict; non-path tokens are skipped, never interpreted. A plan waiver suppresses that plan's findings and surfaces its reason. An unparsable plan face is a named in-band skip and the detector continues over the rest.

**Scope:**
- the plan-face read, the token split, and the path-like predicate with the stat/prefix check
- plan-waiver application with applied-waiver surfacing; the advisory phrasing as contract
- the unparsable-plan named skip
- the hermetic replay fixture naming the two never-shipped historical wiring surfaces verbatim, plus a fully-delivered fixture

**Out of scope:**
- any semantic interpretation of non-path tokens (a future design's ground)
- the pass orchestration and exit mapping (T5)

**Acceptance criteria:**
- [ ] on the pinned replay fixture the two never-shipped surfaces are reported citing plan and task in advisory phrasing; a fully-delivered plan yields zero
- [ ] a plan waiver suppresses that plan's findings, surfaces its reason, and its removal restores them; non-path tokens appear in no finding
- [ ] a corrupted plan face yields a named skip while other plans' results are present
- [ ] every emitted finding carries all four contracted fields

**Key interfaces:**
- the detector's pure function over (evaluated state, policy)
- the frozen plan machine face (the plan dialect's committed JSON) as the read contract

**Traces:** prd:maintenance-sweep/FR-9 · prd:maintenance-sweep/FR-10 · prd:maintenance-sweep/UB-4 · design:maintenance-sweep-v1#3.2 · design:maintenance-sweep-v1#DINV-5

### T4 — The rollup-staleness detector

| Field | Value |
|---|---|
| **Blocked by** | T1 |
| **Size band** | 120–240 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | — |
| **Pattern** | — |
| **Touches** | internal/sweep/staleness.go |

**Goal:** The stale-rollup rot class is detected mechanically: each living rollup whose last change trails the newest shipped-structure change beyond the policy bound becomes one finding with the age delta as evidence.

**Current behavior:** Rollup freshness is an unowned convention; nothing measures a rollup's age against shipped structure.

**Desired behavior:** The detector reads, through the runner seam, the newest committed timestamp under the configured capability paths and the newest committed timestamp of each configured rollup — the rollup set and capability paths come from the existing configuration surface's keys, not new policy — and emits one finding per rollup whose lag exceeds the policy's staleness bound, with the age delta and the newest structural change as evidence and the refresh-or-retune remedy. Ages derive from committed timestamps only, so the result is deterministic per evaluated state.

**Scope:**
- the committed-timestamp reads over the configured capability paths and rollup keys
- the bound comparison with the policy's staleness bound
- the constructed staleness fixture matrix: beyond-bound reports with the delta; within-bound zero; the bound read from policy

**Out of scope:**
- any richer definition of "structure" than the configured capability paths (a later amendment's ground)
- the pass orchestration and exit mapping (T5)

**Acceptance criteria:**
- [ ] on the constructed fixture where structure shipped beyond the bound after the rollup's last touch, the rollup is reported with the age delta; within the bound, zero findings
- [ ] the bound is read from the policy and its override changes the verdict on the same fixture
- [ ] two runs on the same fixture yield identical findings (committed timestamps only, no wall clock)
- [ ] every emitted finding carries all four contracted fields

**Key interfaces:**
- the detector's pure function over (evaluated state, policy)
- the existing configuration keys naming the rollups and the capability paths

**Traces:** prd:maintenance-sweep/FR-4 · design:maintenance-sweep-v1#3.2 · design:maintenance-sweep-v1#DINV-2

### T5 — The pass, the binary, and the exit ranking

| Field | Value |
|---|---|
| **Blocked by** | T1, T2, T3, T4 |
| **Size band** | 180–300 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | — |
| **Pattern** | — |
| **Touches** | cmd/maintenance-sweep/main.go · internal/sweep/pass.go |

**Goal:** One invocation is one bounded, read-only pass: fail-closed invocation parse, policy load, the three detectors in fixed order each isolated, one dual-face report, and the pinned exit ranking.

**Current behavior:** The detectors and contracts exist as a package (T1–T4) with no executable surface.

**Desired behavior:** The thin binary parses its invocation fail-closed — an unknown flag or flag-like positional refuses with a usage defect and the fault exit code before any detector runs; loads the policy (malformed = fault; absent = legal empty); runs the three detectors in fixed order, each isolated so one detector's fault is named in-band while the rest complete; emits the machine face on stdout and the human face on stderr with an explicit per-detector zero-findings line — never silence; pins the evaluated state's commit with the dirty annotation on a non-clean tree; and exits by the pinned ranking — any finding one, zero findings with any fault two, otherwise zero — with the machine face's outcome field mapping identically. An unreadable repository or absent toolchain faults before any detector claims cleanliness. Two consecutive runs on a pinned fixture produce byte-identical machine faces.

**Scope:**
- the thin main and the pass orchestration over the T1–T4 organs
- the fail-closed invocation contract and the unreadable-state fault
- the dual faces with face parity, explicit-zero lines, and the dirty annotation
- the exit ranking and its outcome-field mirror

**Out of scope:**
- the scheduler shim and the live-repo evidence (T6)
- any new detector or verb beyond the one pass

**Acceptance criteria:**
- [ ] the exit matrix holds across the fixture corpus: clean zero, findings one, fault two — including misparse, malformed policy, reasonless waiver, and unreadable repo, each with zero finding output
- [ ] a fault-injected detector leaves the remaining detectors' findings present with the fault named, exiting one with findings present and two with zero findings
- [ ] two consecutive runs on a pinned fixture produce byte-identical machine faces; both faces carry the same findings and a clean fixture prints each detector's explicit zero-findings line
- [ ] a dirty working tree sets the machine-face dirty annotation and the human-face warning while the commit names the evaluated base

**Key interfaces:**
- the pass function composing policy load, detectors, report assembly, and exit mapping
- the CLI contract (the shape's invocation grammar and exit codes)

**Traces:** prd:maintenance-sweep/FR-1 · prd:maintenance-sweep/FR-6 · prd:maintenance-sweep/FR-8 · prd:maintenance-sweep/UB-1 · prd:maintenance-sweep/UB-3 · prd:maintenance-sweep/UB-5 · prd:maintenance-sweep/NFR-3 · prd:maintenance-sweep/EVT-1 · design:maintenance-sweep-v1#3.1 · design:maintenance-sweep-v1#3.3 · design:maintenance-sweep-v1#DINV-3 · design:maintenance-sweep-v1#DINV-4

### T6 — The scheduler shim, the seeded policy, and the non-regression proofs

| Field | Value |
|---|---|
| **Blocked by** | T5 |
| **Size band** | 100–220 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | — |
| **Pattern** | — |
| **Touches** | the scheduler shim + its conventions test (agent-surface scripts; deliberately non-path — the surface is cited by role) · docs/policy/maintenance-sweep.yaml · docs/techspec.md |

**Goal:** The sweep is schedulable and proven harmless: the delegate-only shim with its conventions test, the repository's seeded reason-carrying policy from the first live triage, and the read-only, performance, and neighbour-suite proofs.

**Current behavior:** The binary (T5) runs on demand only; no shim, no seeded policy, and the read-only and performance bounds are contracted but unproven on the live repository.

**Desired behavior:** A delegate-only, shell-floor-clean shim binds the sweep to the host scheduler exactly like the wake tick's, its header documenting the scheduler binding and the steady-state cost statement — one bounded pass per invocation, zero between; its conventions test covers delegation, thinness, the shell floor, and one hermetic dry pass. The repository's policy file is seeded from the first live sweep's triage — every entry reason-carrying, so the live bar's emissions are each a verified truth or a visible waiver. The proofs land: before/after state comparison across the corpus shows zero mutations and the package imports no write-capable client; the full pass meets the runtime bound on this repository; the neighbouring document- and code-gate suites pass unchanged; the living rollup gains the shipped capability per the house convention.

**Scope:**
- the shim and its conventions test, with the cost statement homed in the shim header
- the seeded, reason-carrying policy file from the first live triage
- the mutation-audit, import-audit, timing, and neighbour-suite proofs
- the living-rollup update for the shipped capability

**Out of scope:**
- installing any host schedule (the operator's act; its absence is the off state)
- new detectors or policy knobs beyond the contract

**Acceptance criteria:**
- [ ] the shim test passes the conventions rows (delegate-only, thinness, shell floor, hermetic dry pass) and the header carries the binding and cost statement
- [ ] a live sweep on a current mainline checkout emits only verified-true or visibly-waived findings, with every waiver reason recorded in the seeded policy
- [ ] the corpus-wide before/after comparison shows zero repository mutations and the import audit shows no write-capable client
- [ ] the full pass completes within the contracted bound on this repository and the neighbouring gates' suites pass unchanged

**Key interfaces:**
- the shim's delegate-only contract (the house scripts-tests conventions)
- the seeded policy file under the configured policy key

**Traces:** prd:maintenance-sweep/FR-7 · prd:maintenance-sweep/NR-1 · prd:maintenance-sweep/NR-2 · prd:maintenance-sweep/NFR-1 · prd:maintenance-sweep/NFR-2 · design:maintenance-sweep-v1#3.9 · design:maintenance-sweep-v1#DINV-1

## Coverage

| Requirement | Tasks |
|---|---|
| prd:maintenance-sweep/FR-1 | T1, T5 |
| prd:maintenance-sweep/FR-2 | T2 |
| prd:maintenance-sweep/FR-4 | T4 |
| prd:maintenance-sweep/FR-6 | T1, T5 |
| prd:maintenance-sweep/FR-7 | T6 |
| prd:maintenance-sweep/FR-8 | T5 |
| prd:maintenance-sweep/FR-9 | T3 |
| prd:maintenance-sweep/FR-10 | T1, T2, T3 |
| prd:maintenance-sweep/UB-1 | T5 |
| prd:maintenance-sweep/UB-2 | T1 |
| prd:maintenance-sweep/UB-3 | T5 |
| prd:maintenance-sweep/UB-4 | T3 |
| prd:maintenance-sweep/UB-5 | T5 |
| prd:maintenance-sweep/NR-1 | T6 |
| prd:maintenance-sweep/NR-2 | T6 |
| prd:maintenance-sweep/NFR-1 | T6 |
| prd:maintenance-sweep/NFR-2 | T6 |
| prd:maintenance-sweep/NFR-3 | T5 |
| prd:maintenance-sweep/EVT-1 | T5 |
