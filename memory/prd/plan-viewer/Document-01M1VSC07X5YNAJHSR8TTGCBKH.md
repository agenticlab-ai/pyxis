# Plan viewer — the checkpoint's review surface — PRD

| Field | Value |
|---|---|
| **In one line** | A read-only renderer that turns one plan document (source + folded machine face) into one self-contained page — dependency graph, task charters, coverage, and recorded flags — so the plan checkpoint is reviewed visually instead of from raw markdown and JSON. |
| **Relevant-when** | planning, building, or reviewing the plan viewer or its rendered page; triaging defects in plan rendering (missing fields, wrong graph, dead coverage links); changing the plan schema or fold outputs it consumes |
| **Status** | Draft — review pending |
| **Tier** | T1 |
| **Owner** | @mdarabi |
| **Approver(s)** | @mdarabi (operator) |
| **Contributors** | operator (product) · factory agents (authoring + implementation) |
| **Informed** | plan reviewers; the plan-new skill's users |
| **Date** | 2026-07-09 created · 2026-07-09 last updated |
| **Appetite** | small: one read-side verb on the existing plan tooling, its templates and fixtures; zero rollout/support surface (local, read-only). Excludes any annotation engine, served app, or tracker enrichment. |
| **Target release** | before the next plan checkpoint — its first real use is reviewing an open plan PR; it is also the first feature planned and built end-to-end by the factory (the pipeline's live test). |
| **Tracking** | introducing PR (filed on approval) |
| **Artifacts** | prd:plan-new §4.5 (the task contract it renders) · design plan-new-v1 (the plan/v1 face it consumes) · the doc-review loop it composes with |
| **Template version** | prd-template/v1 |

## 1. Summary — the one-pager

- **Problem** — The plan checkpoint is the one human gate before work fans out to the fleet, and today it means reading a five-hundred-line markdown file and its JSON twin: the dependency shape, the per-task charters, the coverage matrix, and the recorded acceptances are all present but none of them are *visible*. The operator said it plainly: plans are really hard to review as md and json.
- **For whom** — the operator (today's only plan reviewer); later, any human running the FR-17 checkpoint on a plan PR.
- **Why now** — the first factory-authored plan is sitting in an open PR awaiting exactly this review, and every future feature ships through the same checkpoint; the viewer is also deliberately the first feature the factory plans and builds for itself end to end.
- **Direction** — a read-only render verb over the existing plan tooling: given a plan directory, emit one self-contained page (all assets inline, offline-openable) showing the header summary, the dependency graph as a diagram, one card per task with every charter field and its badges, the coverage matrix with links into the cards, and a flags panel aggregating every recorded acceptance. Review and annotation ride the existing doc-review loop unchanged — the viewer renders, it never grows its own review machinery.
- **Appetite** — as header: one verb, templates, fixtures; nothing served, nothing written beyond the output page.
- **Success** — the operator completes a plan-PR checkpoint review using only the rendered page (no raw markdown or JSON consulted), first exercised on a real open plan PR.
- **Guardrail** — the write side and the review machinery stay untouched: plan grammar, fold, and projection behavior are byte-identical, and the bootstrap-era doc-review scripts are not extended.
- **Top risks** — the graph rendering misleads (a wrong or partial DAG is worse than none); drift between rendered fields and the plan schema as it evolves; scope creep toward a served app or annotation engine.

## 2. Problem space

Skipped: T1 — the §1 problem statement is the accepted basis (the operator's direct request, one sentence of evidence); no segments, journey, or opportunity analysis would change the decision.

## 3. Goals, metrics, and guardrails

### 3.1 Goals as falsifiable hypotheses

Skipped: T1 — §1's Success and Guardrail lines carry the acceptance-style criteria; no instrumented metrics for local read-only tooling.

### 3.2 Goals → signals → metrics

Skipped: T1 — as §3.1.

### 3.3 Guardrails and decision rule

Skipped: T1 — the §1 Guardrail line is enforced by NR-1/NR-2 below; breach of either blocks the release (§5.8).

### 3.4 Non-goals and waiting room

| ID | Non-goal | Why refused |
|---|---|---|
| NG-1 | An annotation or comment engine of its own | the doc-review loop already owns annotate→apply; the viewer only renders (one review machinery, not two) |
| NG-2 | A served application, daemon, or watch mode | the page is a static artifact; serving is the review loop's concern where needed |
| NG-3 | Rendering arbitrary documents | plans only; the doc renderer owns general markdown |
| NG-4 | Editing plans from the page | plans change by editing the source and re-folding; the page is read-only by definition |

| ID | Waiting room | Revisit when / intended version |
|---|---|---|
| WR-1 | Report-aware enrichment (issue numbers/links per task once a plan is projected) | first review of a projected plan's execution state |
| WR-2 | Plan-version diff view (001 vs 002 of one feature) | the first plan supersession (plan-new WR-5) |
| WR-3 | Governing-PRD inverse view (requirement → covering tasks across plans) | when multiple plans per feature exist |

## 4. Solution space

Skipped: T1 — the §1 Direction paragraph is the shape; the render mechanics, output placement, and verb naming are the technical design's decisions (§7.2).

## 5. System requirements

### 5.1 Functional requirements

| ID | Requirement (EARS) | Rationale | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|
| FR-1 | When invoked on a plan directory, the viewer shall render the named (default: highest-numbered) plan version into one self-contained page whose display requires no network access and no external asset. | the checkpoint happens anywhere — a laptop on a plane reviews the same page | Must | low | the page opens from a local file with networking disabled and renders completely; an asset audit finds zero external references | Test | §1 |
| FR-2 | When rendering a plan, the viewer shall draw the dependency graph as a diagram containing every task exactly once and every blocked-by edge exactly once, with each task's foundation level visually distinguished. | the DAG shape is the review's first question and is invisible in raw markdown | Must | med | on a fixture plan, the generated diagram definition contains each task node once and each edge once, and distinct foundation levels render distinctly; a wrong-edge fixture fails | Test | §1 |
| FR-3 | When rendering a plan, the viewer shall render one card per task carrying every charter field of the task contract (prd:plan-new §4.5 layer 2) plus its badges: mechanical, size band, foundation, routing proposal, and any recorded size or overlap acceptance. | the charter is what the reviewer approves; omitted fields hide exactly what needs eyes | Must | med | golden fixture: every field value and badge of every task appears in the page; a field added to the fixture and not rendered fails | Test | §1 |
| FR-4 | When rendering a plan, the viewer shall render the coverage matrix with each requirement's task references linking to the corresponding task cards, and shall render deferred rows distinctly. | coverage is the completion criterion; the reviewer verifies it by following links, not by cross-referencing IDs by hand | Must | med | fixture: every coverage row renders, every task reference resolves to an in-page card anchor, deferred rows are visually distinct | Test | §1 |
| FR-5 | When rendering a plan, the viewer shall aggregate every recorded acceptance (size and overlap) into one flags panel naming the task and the recorded reason. | recorded decisions are the checkpoint's attention surface; scattering them across cards buries them | Must | low | fixture: the panel lists exactly the acceptances present in the machine face, with task and reason | Test | §1 |
| FR-6 | The viewer shall operate read-only over the plan's source and folded machine face — including a draft (unmerged) plan — writing nothing but the output page. | the checkpoint reviews drafts on open PRs; a viewer that mutates or demands a merged plan cannot serve it | Must | low | a filesystem audit across a render shows exactly one file created and zero modified; a draft plan renders | Test | §1 |
| FR-7 | If the plan does not fold green, then the viewer shall refuse to emit a page and shall present the fold defects instead. | a pretty rendering of an invalid plan launders it past review | Must | med | a defect-carrying fixture yields no page and every fold defect in the output | Test | §1 |

**Non-regression (NR-n)**

| ID | Existing behavior protected | Why protected | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|
| NR-1 | Plan grammar, fold, verify, and projection behavior are byte-identical with the viewer present | the viewer is read-side only; the write side is frozen contract | med | the existing plan tooling suites pass unchanged; fold output on the fixture corpus is byte-identical before and after | Test | §1 |
| NR-2 | The bootstrap-era doc-review scripts and their vendored runtime are untouched | maintained, never extended (repo language policy); the viewer composes with them, not into them | low | zero diff under the review-tooling scripts and vendor trees | Inspection | §1 |

### 5.2 Unwanted behavior and failure handling

| ID | If… | Then the system shall… | User sees | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|---|
| UB-1 | If the plan directory is missing, empty, or the named version does not exist | refuse with a defect naming the path, the operation, the reason, and the remedy, writing nothing | named defect, no output file | Must | low | fixture matrix: each case yields the quadruple and zero writes | Test | FR-6 |
| UB-2 | If the committed machine face is missing or diverges from the source | refuse and name the divergence, directing the reviewer to re-fold | named defect | Must | low | stale-face fixture refused with the re-fold remedy | Test | FR-7 |

### 5.3 Permissions and abuse cases

Skipped: T1 — read-only local rendering of repo files; no new surface, identity, or egress.

### 5.4 Data

Skipped: T1 — no state created or changed; the output page is a regenerable artifact of its inputs (its placement is a design question, §7.2).

### 5.5 Interfaces and integrations

Skipped: T1 — consumes the plan class's two faces as-is; composes with the doc-review loop unchanged (NR-2); no new contract crosses a boundary.

### 5.6 Non-functional requirements

Skipped: T1 — rendering a plan of realistic size is interactive-instant by construction (one local file in, one out); the repo's Go rules apply to the implementation as to everything.

### 5.7 Instrumentation — the tracking plan

Skipped: T1 — no metrics; §1's Success is a human-judged checkpoint outcome recorded in §5.8.

### 5.8 Definition of Done

| ID | Done criterion | Source | Verify | Required evidence | Blocking? |
|---|---|---|---|---|---|
| DOD-1 | All Must FRs pass their fit criteria | FR-1..7 | Test | CI suite run | yes |
| DOD-2 | No unwanted-behavior or non-regression row broken | UB-1..2, NR-1..2 | Test + Inspection | suite results + diff audit | yes |
| DOD-3 | The operator completes a real plan-PR checkpoint review using only the rendered page | §1 Success | Human judgment — @mdarabi, rubric: the review reaches a verdict without opening the raw markdown or JSON | review record on the plan PR | yes |
| DOD-4 | The feature itself shipped through the factory pipeline: factory-authored plan, fleet-executed tasks | §1 Target release | Demo | the plan, its projection report, and the merged task PRs | yes |

### 5.9 AI behavior and evals

Skipped: T1 — the viewer is deterministic rendering; no model-driven behavior.

## 6. Delivery

Skipped: T1 — §5.8 is the executable bar (§6.4 folds into its blocking rows); no phases, dependencies beyond the frozen plan-new contract, or rollout surface for a local read-only tool.

## 7. Alignment

### 7.1 FAQ

Skipped: T1 — no open stakeholder questions; the §7.2 rows carry the genuine unknowns.

### 7.2 Open questions

| # | Question | Owner | Needed by | Status |
|---|---|---|---|---|
| Q1 | Output placement and lifecycle (regenerable scratch beside the review tooling's, or beside the plan?) and its ignore rule | tech design | implementation | open |
| Q2 | Verb naming and home (a render verb on the existing plan binary vs a separate viewer binary) | tech design | implementation | open |
| Q3 | Diagram technology: reuse the pinned vendored diagram bundle the doc renderer embeds, or generate static markup | tech design | implementation | open |

### 7.3 Decision log

| Date | Decision | Alternatives considered | Why | Who |
|---|---|---|---|---|
| 2026-07-09 | The viewer renders; the existing doc-review loop annotates — no second review machinery | an annotatable viewer app | one review loop in the repo (the guardrail); composition over duplication | @mdarabi |
| 2026-07-09 | This feature is the factory pipeline's first end-to-end test (planned and built by the factory), displacing the larger local-work-tracker plan as the guinea pig | dispatch local-work-tracker first | smaller blast radius; its output improves the operator's own checkpoint immediately | @mdarabi |

### 7.4 Changelog

| Date | Change | Author |
|---|---|---|
| 2026-07-09 | 000-base drafted for review | @mdarabi (factory-assisted) |

## 8. Contract readiness — the executability gate

| Check | Pass? | Notes |
|---|---|---|
| Problem basis accepted — §1 problem accepted (T1) | pending | awaits operator review |
| Zero solution-blocking TBDs or open questions (§7.2) | pass | Q1–Q3 block implementation via the tech design, not the solution |
| Every Must FR has a fit criterion and a verification method (§5.1) | pass | FR-1..7 each carry both |
| Every requirement row carries a risk tier; high/critical rows have a declared safety plan (§5.1, §6.5) | pass | no high/critical rows — read-only local tooling; skips reasoned |
| Non-goals drawn (§3.4); non-regression rows named (§5.1) | pass | NG-1..4; NR-1..2 |
| Every primary job (§2.3) is covered by a requirement, a non-goal, or a waiting-room entry | pass | vacuous — §2 skipped at T1; the §1 problem maps to FR-1..7 |
| Every key flow (§4.3) has functional, failure, and non-regression coverage (§5.1–5.2) | pass | vacuous — §4 skipped at T1 |
| Every primary metric (§3.2) has instrumentation or an explicit measurement plan | pass | vacuous — §3.2 skipped; Success is DOD-3's human judgment |
| Every blocking DoD row traces to a requirement, guardrail, release criterion, or human-judgment rubric (§5.8) | pass | DOD-1..4 sourced; DOD-3 names owner + rubric |
| Risks and assumptions have owners (§6.3) | pass | vacuous — §6 skipped; §1 top risks carried by FR-2/FR-7/NG rows |
| Human-judgment criteria name an owner and a rubric (§5.8) | pass | DOD-3 |
| DoD complete: every blocking row sourced and evidenced; every n/a explained (§5.8) | pass | no n/a rows; every skip carries its reason |
