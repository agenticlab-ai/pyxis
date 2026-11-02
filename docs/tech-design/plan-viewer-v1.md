# plan-viewer v1 — one self-contained page per plan, rendered read-only — technical design

| Field | Value |
|---|---|
| **Status** | Approved (2026-07-09 — operator) *(merge is the freezing event; "superseded" is computed by the index, never written here)* |
| **Owner** | @mdarabi |
| **Approver(s)** | @mdarabi (operator) |
| **Date** | 2026-07-09 |
| **Relevant-when** | designing or changing the plan viewer (the render verb, its page, the diagram embedding); triaging rendering defects (missing charter fields, wrong graph, dead coverage links, external-asset leaks); superseding this design |
| **Scope** | The plan-viewer PRD's mechanics as one unit: the render verb's home, the page's composition (graph, cards, coverage, flags), the diagram technology, output placement, and the refusal path. **Not**: commenting or editing (PRD NG-1), serving/watching (NG-2), report-aware enrichment or diff views (PRD WR-1/WR-2). |
| **Tracking** | introducing PR (filed on approval) — serves and merges after the plan-viewer PRD's PR |
| **Builds-on** | docs/tech-design/plan-new-v1.md (the plan/v1 machine face and fold verdicts this consumes) · docs/tech-design/repo-layout-and-system-footprint.md (bucket taxonomy — the page is regenerable scratch) · docs/tech-design/docgate-v1.md (the verb/defect shape) |
| **Supersedes** | — |
| **Serves** | prd:plan-viewer/FR-1 · prd:plan-viewer/FR-2 · prd:plan-viewer/FR-3 · prd:plan-viewer/FR-4 · prd:plan-viewer/FR-5 · prd:plan-viewer/FR-6 · prd:plan-viewer/FR-7 · prd:plan-viewer/UB-1 · prd:plan-viewer/UB-2 · prd:plan-viewer/NR-1 · prd:plan-viewer/NR-2 |
| **Security/privacy impact** | none — read-only rendering of repo-local files into a local page; no network, identity, or egress surface (see §3.5) |
| **Template version** | tech-design-template/v1 |

## 1. Problem

The plan-viewer PRD is gate-signed but mechanism-free: it demands one
self-contained reviewable page per plan and leaves three questions open —
where the page lives (Q1), where the verb lives (Q2), and how the graph is
drawn (Q3). Until decided, the fleet cannot build it, and the checkpoint
that motivated it (a real plan PR is open right now) keeps being run on raw
markdown. The cheapest moment is now: the plan machine face, the fold
verdicts, the pinned diagram bundle, and the gitignored scratch home all
exist — the viewer only has to compose them.

## 2. Decision

**Decision:** The viewer is **`plangate render`** — a read-side verb on the
existing plan binary — that folds the plan in memory, **refuses with the
fold defects when red**, and otherwise emits **one self-contained HTML page
into the gitignored scratch directory** (`.pyxis/scratch/`),
composing: a header summary, the dependency graph as an inlined
**vendored-mermaid** diagram, one anchored card per task carrying every
§4.5 charter field with badges, the coverage matrix as in-page links, and
a flags panel. The page opens directly in a browser; the operator reviews
it in the conversation. The viewer contains no commenting or editing path.

Why these three answers: the verb belongs on plangate because the plan's
grammar, fold verdicts, and model already live there — a second binary
would duplicate the read path (AP-11); the page belongs in the review
scratch because it is regenerable output, not content (bucket 3); mermaid
inlined from the **pinned vendor bundle** is the one diagram technology the
repo already trusts offline, so
FR-1's self-containment and FR-2's graph come from proven, pinned data
rather than new dependencies (PRD NR-2).

## 3. The design

### 3.1 The render pipeline

**Decision:** `plangate render <plan-dir> [--version NNN] [--out <path>]`
runs the existing in-memory fold (parse + full gate suite) on the named
(default highest) plan version. Fold-red → **no page**: the defects print
exactly as `fold` prints them (FR-7/DINV-2). Fold-green → the page renders
from the **folded document model** (the same struct the machine face
serializes — AP-6: the contract renders, never a re-parse), written to
`--out` or the default `.pyxis/scratch/plan-<feature>-<NNN>.html`. The verb is
read-only: no tracker access, no merged-plan check — drafts on open PRs
are its primary input (FR-6).

### 3.2 Boundaries and non-goals

- **Not solved here** — commenting or editing (review occurs in the operator
  conversation); serving, live
  reload, or a viewer daemon; report-aware enrichment and plan-version
  diffs (PRD WR-1/WR-2, revisited when those inputs exist).
- **Implementation must not infer** — that the page may fetch anything at
  display time (DINV-3 forbids every external reference); that render may
  grow write-side behavior (it never touches fold outputs, the tracker, or
  any plan file); that the mermaid bundle may be bumped as a side effect
  (the pin has its own refresh procedure).
- **Must remain unchanged** — plan grammar, fold, verify, and projection
  (PRD NR-1); the pinned Mermaid asset (PRD NR-2 — this design reads it
  byte-identical and needs no server or package installation).

### 3.3 Invariants

- **DINV-1** — render is a pure function of the plan's source and folded
  model: given one plan state, the page bytes are deterministic, and the
  filesystem gains exactly one file (the page) and loses none.
- **DINV-2** — a fold-red plan yields no page; the refusal carries every
  fold defect verbatim. A page's existence is therefore evidence the plan
  folded green at render time.
- **DINV-3** — the page is self-contained: zero external references
  (scripts, styles, fonts, images); the only script is the pinned vendored
  mermaid bundle inlined at render time.
- **DINV-4** — the graph section renders every task exactly once and every
  blocked-by edge exactly once, derived from the folded model's edge set —
  never from prose.

### 3.4 Observability

- **Events/logs** — one stdout line per render (`rendered <plan> → <page>`)
  or the defect list on refusal; `--defects-json` rides along as on every
  plangate verb.
- **Metrics / Alerts** — none: an on-demand local tool; a red fixture suite
  or a refusal is the signal.
- **Debugging surface** — the page itself (view-source shows the inlined
  mermaid definition); `--out` to render anywhere; the fold defects name
  document/rule/reason/remedy.
- **Trace/correlation** — the page footer states the plan's source
  fingerprint and the tool version, so a review verdict can be tied to the
  exact plan bytes reviewed.

### 3.5 Security & privacy

none — matches the header row: the verb reads repo-local files and writes
one local page; no network at render or display time (DINV-3 is also the
security property: a review page that phones nowhere), no identity,
secrets, or new surface. The page contains only plan content already in
the repository.

### 3.6 Page composition

**Decision:** one `html/template` (stdlib) page, inline CSS, five regions
in reading order: **(1) header** — feature, plan version, dialect, status,
source fingerprint, totals (task count, band sum, largest-task share);
**(2) graph** — a mermaid `graph TD` of the task DAG, nodes labeled
`T<n> — <title>` and styled by foundation level, edges from the folded
`blocked_by` set (DINV-4); **(3) task cards** — one per task, anchored
`id="T<n>"`, rendering every §4.5 layer-2 charter field plus badges
(mechanical, size band, foundation, routing, size/overlap acceptances) and
the task's traces; **(4) coverage** — the matrix with each task reference
an in-page link to its card, deferred rows visually distinct (PRD FR-4);
**(5) flags** — every recorded size/overlap acceptance with task and
reason (PRD FR-5). Charter text renders as text (HTML-escaped by the
template engine); the page never interprets plan content as markup.

## 4. Alternatives considered

| Alternative | Why rejected |
|---|---|
| add a JavaScript renderer beside the Go plan tools | violates Go-by-default and duplicates plan knowledge already held by plangate |
| a separate viewer binary | duplicates plangate's parse/fold read path — the twin-predicate seed AP-11 exists to prevent |
| static SVG generation in Go (no script at all) | DAG layout is real work; the pinned mermaid bundle is already the repo's trusted offline diagram engine — same self-containment, none of the layout code |
| render from `NNN-plan.md` directly | re-parses what the fold already validated and risks divergence from the machine face; the folded model is the contract (AP-6) |
| write the page beside the plan (bucket 1) | regenerable output committed beside frozen content invites drift and PR noise; scratch is the established home for renders |

## 5. Risks

| ID | Risk | Impact | Mitigation | Verification / evidence |
|---|---|---|---|---|
| R1 | the graph misleads (missing/duplicated node or edge) | a wrong review verdict — worse than no viewer | DINV-4 derives the diagram from the folded edge set; the fixture asserts node/edge multiplicity against the model | A-2 |
| R2 | schema evolution drifts past the renderer (new charter field silently unrendered) | hidden content at the checkpoint | the golden fixture renders a maximal plan (every field populated); adding a field to the model without the template fails the golden | A-3 golden |
| R3 | plan prose injects markup into the page | broken or misleading page | stdlib template auto-escaping; a fixture task carries hostile markup and renders inert | A-6 |
| R4 | the vendored bundle is missing in a checkout | no diagram | render refuses with a named defect and the re-vendor remedy (fail closed, not a silently graphless page) | UB fixture |

## 6. Acceptance

| ID | Acceptance criterion | Verify | Required evidence | Traces to |
|---|---|---|---|---|
| A-1 | a rendered page opens offline with zero external references (asset audit over the bytes) | Test | audit fixture | prd:plan-viewer/FR-1 · #DINV-3 |
| A-2 | the fixture plan's diagram definition contains each task node exactly once, each edge exactly once, and distinct foundation styles | Test | fixture | prd:plan-viewer/FR-2 · #DINV-4 |
| A-3 | every §4.5 charter field and badge of every fixture task appears in the page (maximal-plan golden) | Test | golden suite | prd:plan-viewer/FR-3 |
| A-4 | every coverage row renders; every task reference resolves to an in-page anchor; deferred rows carry their distinct marking | Test | fixture | prd:plan-viewer/FR-4 |
| A-5 | the flags panel lists exactly the machine face's recorded acceptances with task and reason | Test | fixture | prd:plan-viewer/FR-5 |
| A-6 | hostile markup in charter prose renders inert (escaped), and a filesystem audit across a render shows exactly one file created | Test | fixture | prd:plan-viewer/FR-6 · #DINV-1 |
| A-7 | a fold-red fixture yields no page and every fold defect in the output; missing dir / bad version / stale machine face each refuse with the named quadruple | Test | fixture matrix | prd:plan-viewer/FR-7 · prd:plan-viewer/UB-1 · prd:plan-viewer/UB-2 · #DINV-2 |
| A-8 | plan write-side suites are byte-identically green with the viewer present; rendering works without a server or package installation and reads the pinned Mermaid asset byte-identically | Test + Inspection | suite run + dependency audit | prd:plan-viewer/NR-1 · prd:plan-viewer/NR-2 |
| A-9 | the operator reviews a real open plan PR from the page alone | Human judgment — @mdarabi, rubric: verdict reached without opening raw md/json | review record | prd:plan-viewer/DOD-3 (via the PRD's §5.8) |

## 7. Open questions

| ID | Question | Owner | Blocks | Status | Resolution |
|---|---|---|---|---|---|
| Q1 | Exact page styling/layout beyond the five regions | implementation | nothing | open | decided against the golden fixtures; the regions and their content are the binding part |
| Q2 | Whether `render` should also emit a markdown summary face for terminal review | operator | nothing | open | deferred — the page is the contract; revisit on demonstrated need |

## Principles

- **AP-6 — honored.** The page renders the folded model — the schema-versioned contract — never a re-parse of prose; the footer pins the source fingerprint.
- **AP-10 / AP-11 — honored.** Rendering rules live in one Go template over plangate's one fold path; no second parser, no prompt-side rendering rules.
- **AP-13 — honored.** A page's existence certifies a green fold at render time (DINV-2); the fingerprint ties the review to exact bytes.
- **AP-17 — honored.** Every failure refuses loudly with the named quadruple (fold-red, missing bundle, stale face); there is no degraded silently-partial page.
- **AP-19 — honored.** On-demand, local, nothing recurring.
- **AP-25 / NR-2 — honored.** The viewer writes only to the shared scratch directory and reuses pinned vendor data without a separate runtime.
- **RW-5 — honored.** This document precedes the implementation; the fleet builds from it.
- **Deviation:** none.
