# Amendment 001 — no-surface stop and conditional harness

| Field | Value |
|---|---|
| **Amends** | e2e-verification *(chain through 000)* |
| **Status** | Approved |
| **Owner** | @mdarabi |
| **Date** | 2026-07-24 |
| **Learned-from** | operator-decision:design-brainstorm:e2e-verification-tech-design#P-2 · operator-decision:design-brainstorm:e2e-verification-tech-design#P-3 |
| **Blast radius** | tier: contract-changing; touches: UB-6→UB-7, FR-12→FR-16, FR-13→FR-17; reason: supersedes three Must rows — reversing e2e-design's no-automatable-surface behavior (produce-nothing-and-stop, not a manual-only design + checklist) and making e2e-plan's harness task conditional |
| **Template version** | amendment-template/v2 |

## Operations

### op: supersede UB-6

Reason: the operator ruled during technical design that a feature with no automatable e2e surface should stop the pipeline cleanly, not emit a manual-only design + checklist nobody asked for (`design-brainstorm:e2e-verification-tech-design#P-2`).

| ID | If… (trigger) | Then the system shall… | User sees | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|---|
| UB-7 | the feature has no meaningfully-automatable e2e surface — every required behavior routes to manual, or the feature is pure-doc/refactor with no end-to-end behavior to exercise | produce no e2e documents and signal the operator that the feature has no automatable e2e surface (naming the reason) and that the downstream e2e steps (review, plan) must not run | a refusal-style signal naming the no-automatable-surface reason; zero e2e documents produced; the pipeline does not advance to review or plan | Must | med | the no-automatable-surface fixtures (an all-manual feature and a pure-doc feature) each yield the stop-signal naming the reason and produce zero e2e documents; zero fabricated scenarios; zero plan produced | Test | F1, FR-1 |

### op: supersede FR-12

Reason: reuse an existing e2e harness rather than always emitting a redundant build-the-harness task (`design-brainstorm:e2e-verification-tech-design#P-3`).

| ID | Requirement (EARS) | Rationale | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|
| FR-16 | When e2e-plan is invoked on an approved e2e design doc, the skill shall decompose it into an ordered plan of per-scenario tasks and — only when the feature's e2e harness does not already exist — a harness task, and project it onto the tracker as ordinary tasks. | the plan is the buildable output; ordinary tasks so the existing executor consumes them; no redundant harness task when a shared or already-built harness is present | Must | med | on the dogfood, e2e-plan produces one task per scenario (or grouped scenarios) plus a harness task iff no harness already exists, projected onto the tracker; the tasks validate against the existing task/charter schema; a fixture with a pre-existing harness yields zero harness tasks | Demo, Test | G1, J3, F3 |

### op: supersede FR-13

Reason: the harness dependency is conditional — scenarios wait on a newly-built harness, but not when the harness already exists (`design-brainstorm:e2e-verification-tech-design#P-3`).

| ID | Requirement (EARS) | Rationale | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|
| FR-17 | When e2e-plan emits a harness task, each scenario task shall carry an explicit blocked-by dependency on it; when the feature's e2e harness already exists, scenario tasks shall carry no harness-build dependency; and no scenario task shall be routable before its harness is available. | harness-before-scenarios where a harness is built; the executor already builds unblocked-first, and a pre-existing (merged) harness leaves scenarios immediately buildable | Must | high | in a produced plan with a harness task, every scenario task is blocked-by it (native relation); in a plan with a pre-existing harness, scenario tasks carry no harness-build edge; a scenario routable before its harness exists is a plan defect | Test | G1, J3, F3 |
