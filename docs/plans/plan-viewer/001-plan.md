# Plan viewer — the checkpoint's review surface — plan 001

| Field | Value |
|---|---|
| **Feature** | plan-viewer |
| **PRD** | docs/prds/plan-viewer |
| **Design** | docs/tech-design/plan-viewer-v1.md |
| **Plan version** | 001 |
| **Dialect** | harbormaster/fs-spec:v1 |
| **Epic title** | Plan viewer — the checkpoint's review surface |
| **Status** | Draft — freezes at merge |
| **Template version** | plan-template/v1 |

## Tasks

### T1 — The render spine: verb, refusal, and the page skeleton

| Field | Value |
|---|---|
| **Blocked by** | — |
| **Size band** | 180–260 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | — |
| **Pattern** | — |
| **Touches** | internal/plangate/render, cmd/plangate |

**Goal:** The render verb exists end to end: a green plan becomes one self-contained page skeleton at the default scratch location, and a red or unusable plan refuses with named defects.

**Current behavior:** Plans have no rendered form; the render verb does not exist and reviewers read raw source and machine faces.

**Desired behavior:** Invoking render on a plan directory folds the named or highest version in memory. A fold-red plan yields no page and presents every fold defect. A green plan yields exactly one file — the page with its header region (feature, version, dialect, status, totals) and a footer stating the source fingerprint and tool version — at the default scratch location or the named output path. A missing directory, unknown version, stale machine face, or missing diagram bundle each refuse with a defect naming file, operation, reason, and remedy. Draft plans render; the verb reads repo files only and never touches the tracker.

**Scope:**
- the render verb with version and output selection over the in-memory fold
- the refusal paths: fold defects presented, environment defects named, no partial page
- the page skeleton: header region, fingerprint footer, inline styling, template escaping
- the write discipline: exactly one output file, nothing else created or modified

**Out of scope:**
- the graph, task-card, coverage, and flags regions (later tasks)

**Acceptance criteria:**
- [ ] a green fixture plan renders one file whose header region and fingerprint footer carry the plan's values
- [ ] a fold-red fixture yields no page and every fold defect in the output
- [ ] a missing directory, unknown version, and stale machine face each refuse with the named quadruple and zero writes
- [ ] a filesystem audit across a render shows exactly one file created and none modified

**Key interfaces:**
- the render verb — version and output selection, defect rendering, exit codes
- the page template — the skeleton the region tasks extend
- the folded plan model — the only rendering input (the machine-face contract)

**Traces:** prd:plan-viewer/FR-1 · prd:plan-viewer/FR-6 · prd:plan-viewer/FR-7 · prd:plan-viewer/UB-1 · prd:plan-viewer/UB-2 · design:plan-viewer-v1#3.1 · design:plan-viewer-v1#DINV-1 · design:plan-viewer-v1#DINV-2

### T2 — The graph region: the dependency diagram

| Field | Value |
|---|---|
| **Blocked by** | T1 |
| **Size band** | 100–160 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | — |
| **Pattern** | — |
| **Touches** | internal/plangate/render |

**Goal:** The page shows the plan's dependency shape: a diagram with every task exactly once, every edge exactly once, and foundation levels visually distinct.

**Current behavior:** The page skeleton renders header and footer only; the dependency shape is invisible.

**Desired behavior:** The graph region draws the task graph from the folded edge set — node labels carry the task identifier and title, styling distinguishes the foundation levels, and the diagram engine is the pinned vendored bundle inlined into the page at render time, keeping the page self-contained with zero external references.

**Scope:**
- the diagram definition generated from the folded model's tasks and edges
- foundation-level styling on nodes
- inlining the pinned vendored diagram bundle into the page

**Out of scope:**
- any layout tuning beyond the engine's defaults

**Acceptance criteria:**
- [ ] the fixture plan's diagram definition contains each task node exactly once and each edge exactly once
- [ ] distinct foundation levels render with distinct styles
- [ ] an asset audit over the rendered bytes finds zero external references

**Key interfaces:**
- the graph-region template and the diagram-definition generator over the folded model

**Traces:** prd:plan-viewer/FR-2 · design:plan-viewer-v1#3.6 · design:plan-viewer-v1#DINV-3 · design:plan-viewer-v1#DINV-4

### T3 — The task cards: every charter field, anchored

| Field | Value |
|---|---|
| **Blocked by** | T2 |
| **Size band** | 140–220 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | — |
| **Pattern** | — |
| **Touches** | internal/plangate/render |

**Goal:** Each task renders as one anchored card carrying its complete charter, so the reviewer reads the contract the worker will execute.

**Current behavior:** The page shows the plan's shape but none of its content; charters are only readable in the raw source.

**Desired behavior:** One card per task, anchored by task identifier, renders every charter field of the task contract — goal, current and desired behavior, scope, out-of-scope, acceptance criteria, key interfaces, traces — plus badges for mechanical, size band, foundation, routing proposal, and any recorded size or overlap acceptance. Charter prose renders as escaped text: hostile markup in a plan displays inert.

**Scope:**
- the task-card template over the charter field set with its badges
- in-page anchors per task
- escaping verification for charter prose

**Out of scope:**
- coverage links into the cards (the next task wires them)

**Acceptance criteria:**
- [ ] every charter field and badge of every fixture task appears in the page
- [ ] a maximal fixture (every field populated) renders with no omission, guarding schema drift
- [ ] a fixture task carrying hostile markup renders it inert

**Key interfaces:**
- the task-card template — the reviewer-facing form of the task contract

**Traces:** prd:plan-viewer/FR-3 · design:plan-viewer-v1#3.6 · design:plan-viewer-v1#DINV-1

### T4 — Coverage links and the flags panel

| Field | Value |
|---|---|
| **Blocked by** | T3 |
| **Size band** | 120–180 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | — |
| **Pattern** | — |
| **Touches** | internal/plangate/render |

**Goal:** The review's two attention surfaces render: the coverage matrix navigable by links into the task cards, and every recorded acceptance aggregated in one flags panel.

**Current behavior:** Coverage and recorded acceptances exist in the plan but the page does not show them; the reviewer cross-references identifiers by hand.

**Desired behavior:** The coverage region renders every matrix row with each task reference linking to that task's card anchor; deferred rows render visually distinct. The flags panel lists every recorded size and overlap acceptance with its task and reason — exactly those present in the machine face, no more and no fewer.

**Scope:**
- the coverage-region template with in-page links and distinct deferred styling
- the flags panel aggregating recorded acceptances

**Out of scope:**
- any inverse view across plans (the governing contract's waiting room)

**Acceptance criteria:**
- [ ] every coverage row renders and every task reference resolves to an in-page card anchor
- [ ] deferred rows carry their distinct marking
- [ ] the flags panel lists exactly the machine face's acceptances with task and reason

**Key interfaces:**
- the coverage-region and flags-panel templates

**Traces:** prd:plan-viewer/FR-4 · prd:plan-viewer/FR-5 · design:plan-viewer-v1#3.6

### T5 — The proof pass: goldens, audits, and the live render

| Field | Value |
|---|---|
| **Blocked by** | T4 |
| **Size band** | 100–160 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | — |
| **Pattern** | — |
| **Touches** | internal/plangate/render |

**Goal:** The finished viewer is proven whole: deterministic golden output, the self-containment and write audits as standing fixtures, the untouched-write-side evidence, and a live render of a real plan.

**Current behavior:** Each region passes its own checks, but no end-to-end golden, audit suite, or real-plan render exists.

**Desired behavior:** A maximal fixture plan renders byte-deterministically against a committed golden. The asset audit (zero external references) and the write audit (exactly one file) run as standing fixtures over the full page. The existing plan tooling suites pass unchanged beside the viewer, and the review-tooling scripts and vendor trees show zero diff. A real plan from the repository renders successfully as the demo evidence.

**Scope:**
- the end-to-end golden over a maximal fixture
- the standing asset and write audits
- the write-side and review-tooling non-regression evidence
- the live render of a real plan in the repository

**Out of scope:**
- the human checkpoint use itself (the operator's review is recorded on the plan PR it serves)

**Acceptance criteria:**
- [ ] two renders of one plan state are byte-identical and match the committed golden
- [ ] the asset and write audits pass over the full page as standing fixtures
- [ ] the plan tooling suites pass unchanged and the review-tooling scripts and vendor trees show zero diff
- [ ] a real plan in the repository renders to a page that opens offline

**Key interfaces:**
- the golden suite and the audit fixtures — the viewer's proof surface

**Traces:** prd:plan-viewer/NR-1 · prd:plan-viewer/NR-2 · prd:plan-viewer/FR-1 · prd:plan-viewer/FR-6 · design:plan-viewer-v1#DINV-1 · design:plan-viewer-v1#DINV-3

## Coverage

| Requirement | Tasks |
|---|---|
| prd:plan-viewer/FR-1 | T1, T5 |
| prd:plan-viewer/FR-2 | T2 |
| prd:plan-viewer/FR-3 | T3 |
| prd:plan-viewer/FR-4 | T4 |
| prd:plan-viewer/FR-5 | T4 |
| prd:plan-viewer/FR-6 | T1, T5 |
| prd:plan-viewer/FR-7 | T1 |
| prd:plan-viewer/UB-1 | T1 |
| prd:plan-viewer/UB-2 | T1 |
