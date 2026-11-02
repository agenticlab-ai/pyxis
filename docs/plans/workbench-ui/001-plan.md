# Workbench UI — the reading room — plan 001

| Field | Value |
|---|---|
| **Feature** | workbench-ui |
| **PRD** | docs/prds/workbench-ui |
| **Design** | docs/tech-design/workbench-ui-v1.md |
| **Plan version** | 001 |
| **Dialect** | harbormaster/fs-spec:v1 |
| **Epic title** | Workbench UI — the reading room |
| **Status** | Draft — freezes at merge |
| **Template version** | plan-template/v1 |

## Tasks

### T1 — The serving spine: lifecycle, guards, and the complete route table

| Field | Value |
|---|---|
| **Blocked by** | — |
| **Size band** | 220–300 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | — |
| **Pattern** | — |
| **Touches** | cmd/workbench-ui/main.go · internal/workbenchui/server.go · internal/workbenchui/testdata/lifecycle |

**Goal:** The reading room's server exists end to end: launch to a guarded secret URL, the shape's complete route table stubbed through per-module handler seams, and the whole lifecycle — refusals, exits, and audits — proven.

**Current behavior:** No reading-room executable exists in the system release.

**Desired behavior:** The released Go binary mints a session secret, binds an ephemeral loopback port, writes the statefile, and opens the catalog stub at the secret URL. Every route of the shape's table exists from birth, each delegating to a handler seam (view tasks fill handlers, never the entry point). Requests lacking the secret, carrying an unexpected Host, using a non-GET method, or traversing document paths are refused. A second launch against a live session, or an unbindable port, refuses with the named quadruple and no half-served state; a dead statefile is replaced after a liveness probe. The process exits on terminal end and after the idle window; unknown flags or flag-like positionals refuse usage before any action.

**Scope:**
- the entry and argument grammar; the session lifecycle (secret, port, statefile) and browser open
- the guard layer: secret, Host allowlist, GET-only, document-path bounds
- the complete route table with per-view handler seams and stub responses
- exit paths (terminal, idle) and the refusal matrix

**Out of scope:**
- any real view content (the handlers land in later tasks)

**Acceptance criteria:**
- [ ] launching serves the catalog stub at the secret URL; secret-less, rebound-Host, non-GET, and traversal requests are each refused
- [ ] a second launch and an occupied port each refuse with the named quadruple; a dead statefile is replaced after the liveness probe
- [ ] terminal end and idle expiry leave no process and no repo write (filesystem audit: scratch only)
- [ ] an unknown flag or flag-like positional refuses with usage and exit 2 before any action

**Key interfaces:**
- the entry's argument grammar and exit codes
- the handler seam every route delegates through (the view tasks' contract)
- the statefile schema and the liveness probe

**Traces:** prd:workbench-ui/FR-7 · prd:workbench-ui/FR-8 · prd:workbench-ui/UB-1 · prd:workbench-ui/UB-4 · prd:workbench-ui/NFR-2 · prd:workbench-ui/ABUSE-3 · design:workbench-ui-v1#3.6 · design:workbench-ui-v1#3.10 · design:workbench-ui-v1#DINV-1 · design:workbench-ui-v1#DINV-3

### T2 — The corpus reader and the class registry

| Field | Value |
|---|---|
| **Blocked by** | — |
| **Size band** | 240–300 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | — |
| **Pattern** | — |
| **Touches** | internal/workbenchui/registry.go · internal/workbenchui/reader.go · internal/workbenchui/testdata/classes |

**Goal:** Every governed document class reads into the uniform record through the class registry — machine faces first, the header/anchor scan for faceless classes — with class knowledge nowhere else.

**Current behavior:** No workbench corpus reader exists; the machine faces are consumed only by their owning Go tools.

**Desired behavior:** The registry enumerates every current document class, each entry naming its root, machine face (or none), anchor and citation grammars, render route, and progress role. The reader turns the working tree into uniform records: faced classes parse their generated JSON; faceless classes get the header-table, heading, and anchor scan — never semantic markdown parsing. A document failing its class parse yields a defect-carrying record while the rest of the corpus reads clean. Conformance fixtures pin the faceless grammars against golden documents produced from the live templates, and a synthetic-class fixture proves onboarding touches only the registry.

**Scope:**
- the registry with every current class in both lanes
- the reader: faces consumed as data, the faceless scan, defect records
- the golden-pinned faceless-grammar suite and the class-onboarding fixture

**Out of scope:**
- search, progress derivation, and document presentation (they consume records)

**Acceptance criteria:**
- [ ] on the fixture corpus every class reads into records with titles, anchors, and citations as pinned by the goldens
- [ ] a corrupted fixture document yields its defect record while every other record reads clean
- [ ] the synthetic-class fixture onboards with one registry entry and zero edits elsewhere (diff inspection)
- [ ] grammar drift injected into a golden turns the conformance suite red

**Key interfaces:**
- the registry entry schema (both lanes) — the shape's contract
- the uniform record (DocEntry) every downstream consumer reads
- the tree fingerprint the freshness layer keys on

**Traces:** prd:workbench-ui/FR-1 · prd:workbench-ui/UB-3 · prd:workbench-ui/NFR-4 · design:workbench-ui-v1#3.1 · design:workbench-ui-v1#DINV-4 · design:workbench-ui-v1#DINV-5

### T3 — Catalog and document views through registered Go presenters

| Field | Value |
|---|---|
| **Blocked by** | T1, T2 |
| **Size band** | 220–300 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | — |
| **Pattern** | — |
| **Touches** | internal/workbenchui/catalog.go · internal/workbenchui/document.go · internal/workbenchui/page.go |

**Goal:** The reading room reads: the catalog lists the corpus by class and every document opens through its registered presenter, with per-document loud degradation when a presenter cannot serve.

**Current behavior:** The spine serves stubs; records exist but nothing presents them.

**Desired behavior:** The catalog groups every record by class, each entry opening the document view: markdown classes through the safe Go presenter, plans through the in-process plan-viewer package. An unavailable class presenter degrades that document to a named remedy card; a defect-carrying record presents its defects; the catalog and every other view stay serviceable throughout. Pages are server-rendered from embedded templates and assets.

**Scope:**
- the catalog view over records grouped by class
- the document view delegating through the class-presentation registry
- per-document degradation and defect cards
- the shared page frame with local-only assets

**Out of scope:**
- citation link resolution inside rendered bodies (a later task)

**Acceptance criteria:**
- [ ] every fixture document appears exactly once in the catalog and opens through its registered presenter
- [ ] with the plan presenter unavailable, plan entries degrade to the named remedy while the catalog and other views serve
- [ ] a defect-carrying record presents its defect card; an asset audit of served pages finds zero external references

**Key interfaces:**
- the catalog and document handler modules behind the spine's seams
- the class-presentation registry and plan-presenter interface

**Traces:** prd:workbench-ui/FR-1 · prd:workbench-ui/FR-2 · prd:workbench-ui/UB-2 · prd:workbench-ui/UB-3 · prd:workbench-ui/NR-2 · design:workbench-ui-v1#3.7 · design:workbench-ui-v1#DINV-2

### T4 — Search with the anchor-ID exact lane

| Field | Value |
|---|---|
| **Blocked by** | T1, T2 |
| **Size band** | 160–240 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | — |
| **Pattern** | — |
| **Touches** | internal/workbenchui/search.go · internal/workbenchui/search_page.go |

**Goal:** A query answers from the live corpus: anchor-ID hits ranked first as exact matches, free text with document and heading context, opening at the match.

**Current behavior:** Records exist but nothing indexes or queries them.

**Desired behavior:** The in-memory index builds from records and rebuilds only when the tree fingerprint moves, so a mid-session edit is reflected on the next request without restart. A query matching an anchor-ID grammar returns its defining locations first; free text returns case-folded token matches with heading context and open-at-match navigation; a no-hit query returns an explicit empty state. No search dependency ships.

**Scope:**
- the fingerprint-keyed in-memory index over records
- the ID-exact lane and the token lane with context and anchors
- the search view and empty state

**Out of scope:**
- ranking heuristics beyond ID-first (the design defers them until evidence)

**Acceptance criteria:**
- [ ] fixture ID queries land their defining anchor first; free-text queries carry document + heading context and open at the match
- [ ] a no-hit query renders the explicit empty state
- [ ] editing a fixture document mid-session is reflected by the next search request without restart

**Key interfaces:**
- the index build/rebuild contract keyed on the tree fingerprint
- the search handler module behind the spine's seam

**Traces:** prd:workbench-ui/FR-3 · prd:workbench-ui/FR-9 · design:workbench-ui-v1#3.8 · design:workbench-ui-v1#DINV-5

### T5 — Citation tracing in rendered documents

| Field | Value |
|---|---|
| **Blocked by** | T3 |
| **Size band** | 120–180 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | — |
| **Pattern** | — |
| **Touches** | internal/workbenchui/citations.go · internal/workbenchui/document.go |

**Goal:** Every corpus citation grammar in a rendered document is a navigable link to its target anchor; a dangling citation shows named-unresolved.

**Current behavior:** Rendered documents display citations as plain text; the reader resolves them by hand.

**Desired behavior:** The document view post-processes rendered bodies: each citation grammar the registry declares (requirement anchors, record anchors, design anchors, principle and incident IDs, review findings) resolves against the record set and renders as navigation to the cited document at the cited anchor. A citation that resolves nowhere renders as named-unresolved — visibly distinct, never silent plain text, never a broken link.

**Scope:**
- citation detection over rendered bodies using the registry's grammars
- resolution against records and anchor maps
- the link and named-unresolved renderings

**Out of scope:**
- any change to the underlying committed transforms

**Acceptance criteria:**
- [ ] every citation grammar in the fixture set resolves to its target document and anchor as a working in-page navigation
- [ ] a dangling fixture citation renders named-unresolved and visibly distinct
- [ ] rendered-body bytes without citations are unchanged by the pass

**Key interfaces:**
- the citation resolution contract over records + anchor maps

**Traces:** prd:workbench-ui/FR-4 · design:workbench-ui-v1#3.1 · design:workbench-ui-v1#DINV-2

### T6 — The progress view: stage chain, gates, honest unknowns

| Field | Value |
|---|---|
| **Blocked by** | T1, T2 |
| **Size band** | 180–260 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | — |
| **Pattern** | — |
| **Touches** | internal/workbenchui/progress.go · internal/workbenchui/progress_page.go |

**Goal:** One view answers "what stage is each feature at, and what awaits a human" — derived from the corpus alone, honest where the corpus is silent.

**Current behavior:** Stage questions are answered by listing directories and opening documents one by one.

**Desired behavior:** The progress view joins records per feature slug: the brainstorm record and its gate verdict, the PRD ledger and status, designs serving the feature (from the index face) with their status, plan presence, and attached review verdicts. Each row names the furthest artifact and pending human gates, every cell linking to its artifact. Stages beyond the corpus render as unknown; a contested or unparseable signal renders as its defect, never a guess. The view reflects the working tree at each request.

**Scope:**
- the per-feature join over records and the index face
- the stage chain, pending-gates derivation, and honest-unknown states
- the progress view with links into artifacts

**Out of scope:**
- tracker, PR, or execution state (the PRD's waiting room)

**Acceptance criteria:**
- [ ] fixture features at distinct stages render their expected chain, gates, and links, offline
- [ ] adding an artifact to a fixture feature advances its row on the next request
- [ ] a contested fixture signal renders as its defect; beyond-corpus stages render unknown

**Key interfaces:**
- the progress row contract (the shape's schema) and the stage derivation over records

**Traces:** prd:workbench-ui/FR-5 · prd:workbench-ui/FR-9 · design:workbench-ui-v1#3.9 · design:workbench-ui-v1#DINV-5

### T7 — Hand-off launch points

| Field | Value |
|---|---|
| **Blocked by** | T5 |
| **Size band** | 60–120 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | — |
| **Pattern** | — |
| **Touches** | internal/workbenchui/document.go |

**Goal:** From a presented document, the reviewer can copy its exact source path or open its tracked reference in the owning interaction channel.

**Current behavior:** Reaching the operator conversation or tracked PR means leaving the UI and reconstructing paths by hand.

**Desired behavior:** The document view presents the open document's exact source path and stable reference as copyable text, and links the tracked reference where its record carries one. A write-audit across the interaction shows the UI process performing no comment capture, spawning no review process, and writing nothing to the repository.

**Scope:**
- the presented, copyable source path and stable reference per document
- the tracked-reference link where present
- the no-execution guarantee

**Out of scope:**
- running or embedding review or comment-capture machinery (the PRD's NG-1)

**Acceptance criteria:**
- [ ] the document view presents its copyable source path and stable reference and links the tracked reference when the record carries one
- [ ] a write-and-process audit across the interaction shows no execution and no repo write

**Key interfaces:**
- the hand-off fragment of the document view

**Traces:** prd:workbench-ui/FR-6 · design:workbench-ui-v1#3.2

### T8 — The proof pass: security, scale, goldens, and the shape flip

| Field | Value |
|---|---|
| **Blocked by** | T4, T6, T7 |
| **Size band** | 160–240 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | — |
| **Pattern** | — |
| **Touches** | internal/workbenchui/testdata/audits · docs/solution-shapes/workbench-ui-v1.md |

**Goal:** The finished reading room is proven whole — the standing security, degradation, freshness, and scale suites green, the non-regression evidence recorded, and the solution shape flipped to superseded-by-code.

**Current behavior:** Each task passes its own checks; no end-to-end proof surface or scale evidence exists, and the shape still reads authoritative.

**Desired behavior:** Standing suites cover the full session: the hostile-content fixture executes nothing and egresses nothing; the rebinding and secret-miss fixtures are refused; a deny-all-egress session completes every flow; the scaled fixture corpus holds the performance budgets; a full-session write audit shows scratch-only writes. The write-side document tooling and plan-viewer suites pass unchanged, and the CI gate surface is diff-identical. With the implementing code landed, the shape's status flips to superseded-by-code and the docs gate stays green — the lifecycle's first real exercise.

**Scope:**
- the standing security, degradation, and egress-off suites over full sessions
- the scaled-corpus benchmark against the PRD's budgets
- the non-regression evidence (write-side suites, plan viewer, gate surface)
- the shape status flip and its gate run

**Out of scope:**
- the operator's acceptance week (the PRD's human-judged done criterion runs on the tracker)

**Acceptance criteria:**
- [ ] hostile-content, rebinding, and secret-miss fixtures pass; a deny-all-egress session completes every flow
- [ ] the scaled fixture corpus meets the budgets of prd:workbench-ui/NFR-1's fit criterion
- [ ] write-side suites and plan-viewer suites pass unchanged; the gate surface shows zero diff
- [ ] the shape reads superseded-by-code and the docs gate is green

**Key interfaces:**
- the standing audit suites — the reading room's proof surface

**Traces:** prd:workbench-ui/NFR-1 · prd:workbench-ui/NFR-2 · prd:workbench-ui/NFR-3 · prd:workbench-ui/NR-1 · prd:workbench-ui/NR-2 · prd:workbench-ui/NR-3 · prd:workbench-ui/ABUSE-2 · prd:workbench-ui/ABUSE-3 · design:workbench-ui-v1#3.11

## Coverage

| Requirement | Tasks |
|---|---|
| prd:workbench-ui/FR-1 | T2, T3 |
| prd:workbench-ui/FR-2 | T3 |
| prd:workbench-ui/FR-3 | T4 |
| prd:workbench-ui/FR-4 | T5 |
| prd:workbench-ui/FR-5 | T6 |
| prd:workbench-ui/FR-6 | T7 |
| prd:workbench-ui/FR-7 | T1 |
| prd:workbench-ui/FR-8 | T1 |
| prd:workbench-ui/FR-9 | T4, T6 |
| prd:workbench-ui/UB-1 | T1 |
| prd:workbench-ui/UB-2 | T3 |
| prd:workbench-ui/UB-3 | T2, T3 |
| prd:workbench-ui/UB-4 | T1 |
| prd:workbench-ui/NFR-1 | T8 |
| prd:workbench-ui/NFR-2 | T1, T8 |
| prd:workbench-ui/NFR-3 | T3, T8 |
| prd:workbench-ui/NFR-4 | T2 |
| prd:workbench-ui/NR-1 | T8 |
| prd:workbench-ui/NR-2 | T3, T8 |
| prd:workbench-ui/NR-3 | T8 |
| prd:workbench-ui/ABUSE-2 | T8 |
| prd:workbench-ui/ABUSE-3 | T1, T8 |
