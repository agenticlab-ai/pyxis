# Contract gate (mechanical stage-6) — plan 001

| Field | Value |
|---|---|
| **Feature** | contract-gate |
| **PRD** | docs/prds/contract-gate |
| **Design** | docs/tech-design/contract-gate-v1.md |
| **Plan version** | 001 |
| **Dialect** | harbormaster/fs-spec:v1 |
| **Epic title** | Implement the mechanical stage-6 contract gate (docgate contract-gate) |
| **Status** | Draft — freezes at merge |
| **Template version** | plan-template/v1 |

## Tasks

### T1 — Enrich the fold's machine face with per-requirement fields

| Field | Value |
|---|---|
| **Blocked by** | — |
| **Size band** | 60–150 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | — |
| **Pattern** | — |
| **Touches** | internal/docgate/fold · internal/docgate/ledger · the source-map schema · fold golden fixtures |

**Goal:** The PRD source map exposes, per requirement unit, its priority, whether it carries a fit criterion, and its declared verification method — projected additively from the already-parsed row cells — so existing consumers are unaffected.

**Current behavior:** The source map carries lineage fields only (status, introduced_by, supersedes, superseded_by, source_location, current_location); the row cells are parsed but never exposed on the machine face.

**Desired behavior:** Each requirement unit additionally carries priority (where a Priority column exists), has-fit-criterion, and verify-method, resolved by header-column name so per-table column order does not matter; re-folding an existing ledger adds the fields and changes no existing field's value.

**Scope:**
- project the new fields from the parsed row cells, keyed to each table's header by column name
- add the fields to the source-map schema additively; a re-fold regenerates existing projections
- leave every existing machine-face field and its consumers byte-unchanged

**Out of scope:**
- the gate's consumption of these fields (later tasks); any change to the human-face projection

**Acceptance criteria:**
- [ ] a fixture PRD's folded source map carries priority, has-fit-criterion, and verify-method per requirement, resolved by header name across the requirement tables (FR, UB, NR, NFR)
- [ ] re-folding an existing ledger adds the fields and alters no existing field's value
- [ ] the docgate fold and gate suites stay green — no existing consumer is perturbed

**Key interfaces:**
- the source-map unit state (additive fields); the parsed row cells and the table header columns

**Traces:** prd:contract-gate/FR-3 · prd:contract-gate/FR-5 · prd:contract-gate/NR-2 · design:contract-gate-v1#3.6 · design:contract-gate-v1#DINV-7

### T2 — The contract-gate verb shell: resolution, verdict schema, fail-closed edges

| Field | Value |
|---|---|
| **Blocked by** | — |
| **Size band** | 150–300 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | — |
| **Pattern** | — |
| **Touches** | cmd/docgate · a new internal contractgate package · the verdict schema |

**Goal:** `docgate contract-gate <feature>` resolves a feature's assembled contract from the committed machine faces and returns one closed-schema verdict — clean when no check fails, and a fail-closed error or configuration-error on missing or malformed input — writing nothing.

**Current behavior:** docgate runs tree-wide checks and emits a defect stream; there is no feature-scoped, verdict-producing mode and no contract resolution.

**Desired behavior:** The verb resolves one feature's PRD projection, serving designs, and DoD; emits a closed verdict (class enum including a reserved not-evaluated semantic slot, plus the input content fingerprints and tool version); refuses fail-closed on missing or unreadable input, a malformed machine face, and an unmerged contract; makes zero mutations on any path.

**Scope:**
- the verb plus feature-scoped contract resolution from the machine faces
- the closed verdict schema — class enum, reserved semantic slot, fingerprint identity
- the fail-closed edges before any check runs; read-only by construction

**Out of scope:**
- the individual checks (T3, T4) — the shell runs an empty check set; the defect report (T5)

**Acceptance criteria:**
- [ ] the verb on a well-formed fixture returns a clean verdict recording the input fingerprints, with zero filesystem, git, or tracker writes
- [ ] missing input yields configuration-error, a malformed machine face yields error, an unmerged contract yields configuration-error — each with zero checks run
- [ ] the verdict validates closed and the reserved semantic slot is present and marked not-evaluated

**Key interfaces:**
- the contract-gate verb; the verdict schema; the contract-inputs resolver

**Traces:** prd:contract-gate/FR-1 · prd:contract-gate/FR-7 · prd:contract-gate/FR-10 · prd:contract-gate/FR-11 · prd:contract-gate/FR-12 · prd:contract-gate/UB-1 · prd:contract-gate/UB-2 · prd:contract-gate/UB-3 · prd:contract-gate/UB-4 · prd:contract-gate/NFR-4 · design:contract-gate-v1#3.1 · design:contract-gate-v1#3.7 · design:contract-gate-v1#DINV-1 · design:contract-gate-v1#DINV-4

### T3 — The two new checks: structural completeness and compilability

| Field | Value |
|---|---|
| **Blocked by** | T1, T2 |
| **Size band** | 100–250 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | — |
| **Pattern** | — |
| **Touches** | internal/docgate/gate (two new check files) · the check registry · the contract-scope marker |

**Goal:** Two new checks, self-registered per-file with a contract-scope marker, catch structural and compilability defects: a requirement missing a fit criterion or verification method (or a missing DoD or §8) reds incomplete, and a malformed join reds malformed.

**Current behavior:** No structural or compilability check exists; the registry holds the tree-wide checks.

**Desired behavior:** The structural check reads the enriched fields and fails naming each requirement missing a fit criterion or verification method, or a missing DoD or §8; the compilability check validates the source map and Serves joins and fails naming each malformed join (orphan Serves, broken supersession, dependency cycle); both self-register from their own file carrying the contract-scope marker.

**Scope:**
- the structural-completeness check over the enriched machine face
- the compilability check over the source map and Serves joins
- both self-registered with the contract-scope descriptor marker

**Out of scope:**
- feature-scoping the reused checks (T4); the verb's aggregation of the subset (T2 and T4)

**Acceptance criteria:**
- [ ] seeded fixtures — a Must missing a fit criterion, one missing a verify method, a missing DoD — each red incomplete naming the requirement; a complete fixture passes
- [ ] seeded fixtures — orphan Serves, dangling supersession, a dependency cycle — each red malformed naming the join; a well-formed fixture passes
- [ ] each check is one new file touching no shared enumeration, dispatch, or help, and both carry the contract-scope marker

**Key interfaces:**
- the two check descriptors; the contract-scope marker field; the enriched unit state

**Traces:** prd:contract-gate/FR-5 · prd:contract-gate/FR-6 · prd:contract-gate/NFR-5 · design:contract-gate-v1#3.9 · design:contract-gate-v1#DINV-3

### T4 — Feature-scope the three reused checks: currency, coverage, link

| Field | Value |
|---|---|
| **Blocked by** | T2 |
| **Size band** | 80–200 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | — |
| **Pattern** | — |
| **Touches** | internal/docgate/gate (citation, coverage, stale-flag readers) · the verb's subset assembly |

**Goal:** The verb runs the three existing checks feature-scoped — currency, inverse coverage, link resolution — via verb-side unit selection, producing stale, incomplete, or unresolved verdicts scoped to one feature, with the checks' tree-wide behavior for existing callers byte-unchanged.

**Current behavior:** The citation, coverage, and stale-flag computations are tree-wide; nothing runs them scoped to one feature's contract or gates on them.

**Desired behavior:** The verb selects the feature's units from the machine faces and runs the three reused checks over that subset, marking them contract-scope; a stale serve, an uncovered Must, and a dangling citation each red the feature verdict; a feature with no live designs reds incomplete; the checks' existing tree-wide behavior for their current callers is unchanged.

**Scope:**
- feature-scoped selection of the reused checks via verb-side unit selection
- mark the three reused checks contract-scope; aggregate their results into the verdict
- preserve each reused check's existing tree-wide behavior for its current callers

**Out of scope:**
- the new checks (T3); changing any reused check's tree-wide logic

**Acceptance criteria:**
- [ ] seeded stale-serve reds stale, uncovered-Must reds incomplete, dangling-citation reds unresolved — each scoped to the feature and naming the offender
- [ ] a feature with a PRD but no live serving designs reds incomplete (every Must unserved)
- [ ] the existing docgate gate and citation verdicts on the corpus are byte-unchanged

**Key interfaces:**
- the feature-unit selector; the contract-scope subset assembly; the coverage per-feature family

**Traces:** prd:contract-gate/FR-2 · prd:contract-gate/FR-3 · prd:contract-gate/FR-4 · prd:contract-gate/UB-5 · prd:contract-gate/NR-2 · prd:contract-gate/NR-3 · design:contract-gate-v1#3.9

### T5 — The defect report: one dual-face event record

| Field | Value |
|---|---|
| **Blocked by** | T3, T4 |
| **Size band** | 100–250 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | — |
| **Pattern** | — |
| **Touches** | the contractgate report emitter and template · the report schema |

**Goal:** On any failing verdict the verb emits exactly one structured, dual-face template-governed defect report bundling every failed check's named defects; a clean verdict emits none; the emission is read-only, so the host commits it.

**Current behavior:** No defect-report artifact exists.

**Desired behavior:** A bounce produces one report enumerating every failed check's defects (check id, offender, unblock) in a machine-readable face and a human rendering, governed by a template and schema; the report is emitted to a path the host reads — the gate never commits it; a clean verdict emits nothing.

**Scope:**
- the dual-face report schema, template, and emitter
- bundle every failed check's named defects (offender plus unblock) into one report
- read-only emission to a path; nothing on a clean verdict

**Out of scope:**
- the host's git-commit of the report (T6 and T7 own that); the report's exact home path, decided here at implementation

**Acceptance criteria:**
- [ ] a bouncing fixture emits exactly one report whose both faces validate and enumerate all failed checks with offender and unblock
- [ ] a clean fixture emits zero report and zero artifact (filesystem audit)
- [ ] the gate performs no git or tracker mutation while emitting (read-only audit)

**Key interfaces:**
- the defect-report schema; the report template; the read-only emitter

**Traces:** prd:contract-gate/FR-8 · design:contract-gate-v1#3.8 · design:contract-gate-v1#DINV-1 · design:contract-gate-v1#DINV-2 · design:contract-gate-v1#DINV-6

### T6 — The severity-map policy and the continuous CI context

| Field | Value |
|---|---|
| **Blocked by** | T3, T4 |
| **Size band** | 80–200 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | — |
| **Pattern** | — |
| **Touches** | the pyxis-config severity-map schema · the docgate CI context wiring · the workflow entry |

**Goal:** The continuous advisory surface runs the contract-scope subset on PRD and design PRs with a hard-or-advisory split read from a closed-schema versioned severity-map policy — currency, link, and structural blocking, inverse-coverage advisory — with a missing policy falling back to the default split.

**Current behavior:** No continuous contract check and no severity-map policy exist.

**Desired behavior:** A closed-schema versioned severity-map policy on the pyxis-config surface encodes the per-trigger split (hard at planning entry; coverage advisory continuously); a docgate CI context runs the subset on contract-doc PRs applying it; a malformed policy fails closed with configuration-error, a missing one uses the built-in default.

**Scope:**
- the closed-schema versioned severity-map policy (unknown key fails closed; missing uses the default)
- the continuous CI context running the contract-scope subset with the split
- coverage advisory continuously; currency, link, and structural blocking

**Out of scope:**
- the planning-entry hard gate (T7); the checks themselves (T3/T4)

**Acceptance criteria:**
- [ ] a fixture PR seeding a currency, link, and structural defect fails the context; one whose only gap is an uncovered Must reports advisory and does not block
- [ ] a malformed severity-map yields configuration-error; an absent one runs on the default split
- [ ] editing the severity-map flips a fixture's hard-or-advisory outcome with zero code change

**Key interfaces:**
- the severity-map policy schema; the CI context entry; the pyxis-config surface

**Traces:** prd:contract-gate/FR-9 · prd:contract-gate/FR-13 · prd:contract-gate/UB-6 · design:contract-gate-v1#3.10

### T7 — The plan-new planning-entry precondition (cross-feature amendment)

| Field | Value |
|---|---|
| **Blocked by** | T4, T5 |
| **Size band** | 40–120 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | — |
| **Pattern** | — |
| **Touches** | docs/prds/plan-new (a typed amendment on plan-new's own ledger) · the plan-new precondition call |

**Goal:** plan-new refuses to draft, freeze, or project a plan until `docgate contract-gate <feature>` returns clean — landed as an additive typed amendment on plan-new's own frozen ledger.

**Current behavior:** plan-new drafts directly from the contract with no contract-gate precondition.

**Desired behavior:** An additive typed amendment on plan-new's ledger introduces the precondition requirement (call the gate before drafting; refuse on any non-clean verdict, surfacing the defect report); a bouncing contract yields zero plan output and a clean one proceeds; plan-new's other ledger rows are untouched. This is a cross-feature amendment on a different feature's frozen contract — additive only.

**Scope:**
- the additive typed amendment on plan-new's ledger introducing the precondition
- wire plan-new to call the verb before drafting and refuse on non-clean
- surface the emitted defect report to the planning session

**Out of scope:**
- any non-additive change to plan-new's existing behavior; the continuous surface (T6)

**Acceptance criteria:**
- [ ] a bouncing contract fixture produces zero draft or plan output and surfaces the defect report; a clean one proceeds
- [ ] the plan-new amendment is a single additive op and a diff shows its other ledger rows untouched
- [ ] plan-new's existing fixtures stay green

**Key interfaces:**
- the plan-new precondition requirement (amendment); the verb invocation seam

**Traces:** prd:contract-gate/FR-1 · prd:contract-gate/NR-1 · design:contract-gate-v1#3.10

### T8 — Fixtures, determinism, performance, and the dogfood sweep

| Field | Value |
|---|---|
| **Blocked by** | T3, T4, T5, T6, T7 |
| **Size band** | 100–300 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | — |
| **Pattern** | — |
| **Touches** | the contractgate fixture corpus · the determinism/perf tests · the dogfood sweep harness |

**Goal:** The gate is proven deterministic, fast enough, and honest on real contracts: the fixture suite is green, the verdict is byte-identical across runs with no network, p95 wall-time is within budget on the largest live contract, and the dogfood sweep over the repo's own live contracts surfaces the known drift with zero false bounces.

**Current behavior:** No fixture suite, determinism and performance checks, or dogfood run exist for the gate.

**Desired behavior:** A fixture suite covering clean and each defect class as permanent tests; a determinism test (byte-identical verdicts; a gate network attempt fails); a performance check (p95 within the PRD's budget on the largest live contract); and a dogfood sweep running the gate over every live contract, surfacing at least the known drift instance with zero unresolved false bounces.

**Scope:**
- the fixture suite (clean plus one per defect class) as permanent tests
- the determinism and performance checks
- the dogfood sweep over the repo's live contracts and its disposition record

**Out of scope:**
- the LLM semantic checks (issue #419); any real second-repo onboarding

**Acceptance criteria:**
- [ ] the fixture suite is green in CI; repeated runs yield byte-identical verdicts; a gate network attempt fails the determinism suite
- [ ] p95 gate wall-time is within the PRD's budget on the repo's largest live contract
- [ ] the dogfood sweep surfaces at least the known drift instance with zero unresolved false bounces, with a disposition record attached

**Key interfaces:**
- the fixture corpus; the determinism and performance test harness; the dogfood sweep runner

**Traces:** prd:contract-gate/NFR-1 · prd:contract-gate/NFR-2 · prd:contract-gate/NFR-3 · prd:contract-gate/NFR-6 · design:contract-gate-v1#3.11

## Coverage

| Requirement | Tasks |
|---|---|
| prd:contract-gate/FR-1 | T2, T7 |
| prd:contract-gate/FR-2 | T4 |
| prd:contract-gate/FR-3 | T1, T4 |
| prd:contract-gate/FR-4 | T4 |
| prd:contract-gate/FR-5 | T1, T3 |
| prd:contract-gate/FR-6 | T3 |
| prd:contract-gate/FR-7 | T2 |
| prd:contract-gate/FR-8 | T5 |
| prd:contract-gate/FR-9 | T6 |
| prd:contract-gate/FR-10 | T2 |
| prd:contract-gate/FR-11 | T2, T5 |
| prd:contract-gate/FR-12 | T2, T8 |
| prd:contract-gate/FR-13 | T6 |
| prd:contract-gate/NR-1 | T7 |
| prd:contract-gate/NR-2 | T1, T4 |
| prd:contract-gate/NR-3 | T4 |
| prd:contract-gate/UB-1 | T2 |
| prd:contract-gate/UB-2 | T2 |
| prd:contract-gate/UB-3 | T2 |
| prd:contract-gate/UB-4 | T2 |
| prd:contract-gate/UB-5 | T4 |
| prd:contract-gate/UB-6 | T6 |
| prd:contract-gate/NFR-1 | T8 |
| prd:contract-gate/NFR-2 | T2, T8 |
| prd:contract-gate/NFR-3 | T8 |
| prd:contract-gate/NFR-4 | T2 |
| prd:contract-gate/NFR-5 | T3 |
| prd:contract-gate/NFR-6 | T2, T8 |
