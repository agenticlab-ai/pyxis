# Workbench UI (the reading room) — PRD

| Field | Value |
|---|---|
| **In one line** | A read-only, locally served web UI over the repo's governed document corpus — browse every class, search including anchor-ID lookup, trace citations as links, and read per-feature pipeline progress. |
| **Relevant-when** | planning, building, or reviewing local web access to the document system; triaging defects in corpus browsing, search, citation resolution, or the progress view; changing the document classes, machine faces, or their readable presentations |
| **Status** | Draft — problem review pending |
| **Tier** | T2 |
| **Owner** | @mdarabi |
| **Approver(s)** | @mdarabi (operator) |
| **Contributors** | operator (product) · factory agents (authoring + implementation) |
| **Informed** | operator review users; engineers onboarding future repos |
| **Date** | 2026-07-10 created · 2026-07-10 last updated |
| **Appetite** | small-medium: one Go served-app surface in the managed system footprint (ephemeral localhost) plus its templates, fixtures, and tests; zero rollout/support surface (local, read-only). Excludes tracker/network integration, auth beyond loopback, persisted index artifacts, and any write path. (brainstorm:workbench-ui#D-8) |
| **Target release** | trigger-bound: first exercised answering real doc-system questions during a live review week — the same acceptance style plan-viewer's checkpoint used |
| **Tracking** | introducing PR (filed on approval) |
| **Artifacts** | brainstorm:workbench-ui (the gated stage-1 record, riding this PR) · the plan-viewer page contract · the operator conversation |
| **Template version** | prd-template/v1 |

## 1. Summary — the one-pager

- **Problem** — The document corpus *is* the product right now, but reading it means terminals and raw files: finding a document, following a citation, or answering "what stage is this feature at" takes grep, path knowledge, and cross-referencing by hand. The operator asked for it plainly: another way to access the information in the repo. (brainstorm:workbench-ui idea, verbatim in the record)
- **For whom** — the operator today (sole reviewer and daily reader); any engineer on an onboarded repo later. Explicitly not for external consumers or CI.
- **Why now** — the corpus has grown past casual navigability (ledgers with projections, a design index, plans, review reports, incidents, brainstorm records, a wiki), and every human gate in the pipeline is a reading task; terminal spelunking is machinery-operation time — a factory gap under the inversion test.
- **Direction** — a reading room: an on-demand, loopback-only Go server in the managed system footprint that presents the corpus by class, searches it (anchor IDs first-class), resolves the citation grammars as navigable links, and derives a per-feature pipeline view from the committed corpus alone. It uses a safe read-only document presentation and the plan viewer, while review remains in the operator conversation. It never authors or writes. (brainstorm:workbench-ui#D-2/#D-3/#D-4/#D-6)
- **Appetite** — as header; excludes tracker/network state, auth, persisted indexes, writes, and renderer absorption.
- **Success** — the operator completes real doc-system tasks (find, trace, answer stage questions) using only the UI — no raw markdown, no grep; judged as §5.8's human-judgment row. (brainstorm:workbench-ui#D-7)
- **Guardrail** — every write-side contract stays byte-identical: fold/gate/plan tooling and the gate surface are untouched (NR-1..3).
- **Top risks** — scope creep toward a control tower (monitoring/tracker state); search quality expectations outrunning a deliberately simple index; the read-only server growing an authoring path.

## 2. Problem space

### 2.1 Problem statement

Every pipeline stage produces or consumes documents, and every human touch is a reading task — yet the corpus is only reachable as raw files. The reader must know the layout, the class conventions, and the citation grammars to move around; a stage question ("does this feature have an approved design? a plan? a review verdict?") is answered by listing directories and opening files one by one. The cost is paid at exactly the moments the factory needs the human sharp: reviews and gate decisions. Doing nothing means every future document class adds surface the reader must navigate by hand.

### 2.2 Users and segments

| Segment | Role and context | Domain exp. | Tech exp. | Key goals | Pain today | Evidence |
|---|---|---|---|---|---|---|
| **P1 (primary)** | the operator — daily reader, sole reviewer, on the dev machine | master | master | answer corpus questions quickly; review with context at hand | path knowledge and grep as the only index | the seed request (record idea) |
| P2 (secondary) | an engineer on a future onboarded repo, post workbench install | journeyman | journeyman | learn and navigate an unfamiliar corpus | no map at all — conventions unknown | product-vision onboarding posture |
| **Anti-segment** | external/public consumers; CI and automated jobs | — | — | — | — | deliberate: local, loopback-only, read-only (brainstorm:workbench-ui#D-6) |

### 2.3 Jobs and desired outcomes

| # | When… (situation) | I want to… (motivation) | So I can… (outcome) | Today's workaround | Evidence |
|---|---|---|---|---|---|
| J1 | I need a specific document or don't know if one exists | browse the corpus by class and open it rendered | read it as a document, not source | shell listing + editor | record idea |
| J2 | I hold an ID or term (a requirement row, a principle, a decision anchor) | search the corpus with IDs as exact matches | land on the defining spot | grep + path knowledge | record idea |
| J3 | I'm reading a doc that cites another (requirement rows, record anchors, principles) | follow the citation as a link | trace provenance without hand-resolving grammars | manual lookup per citation | citation grammars in the class contracts |
| J4 | I want the factory's state | see per-feature pipeline progress and pending human gates | know what's in flight and what awaits me | directory archaeology across classes | record idea ("see the progress") |

### 2.4 Evidence

| Claim | Source (research, data, tickets, interviews) | Confidence |
|---|---|---|
| The operator wants browser access to the corpus (browse, search, progress) | the seed request, verbatim in the record header | High |
| Rendered review beats raw markdown for human gates in this repo | plan-viewer's accepted problem basis and its human-judged done criterion, exercised on a real checkpoint | High |
| The corpus exposes machine faces sufficient to derive browse/search/progress without new authority | the class contracts: projections + source maps, the design index's machine face, plan folds, review reports, the incident registry (brainstorm:workbench-ui#F-4) | High |

### 2.5 The journey today

Skipped: single-user internal tooling; the journey is one actor's terminal loop (list → open → grep → cross-reference), already captured by §2.1/§2.3 — a phase map would add no design-changing information.

### 2.6 Opportunity and why now

- **Size** — every human gate in the pipeline is a reading task; the reader count grows with each onboarded engineer and repo.
- **Alternatives users have** — terminal + editor (today's workaround); the code host's file view (renders files, but blind to the doc system: no projections-as-primary, no anchor-ID search, no citation resolution, no stage derivation; and not offline).
- **Why us / strategic fit** — the corpus's machine faces already exist for the factory's own consumption; making them legible to humans is composition, not new authority.
- **Why now** — corpus growth crossed casual navigability while the review cadence went daily.
- **Cost of doing nothing** — reader toil scales with corpus size; the inversion test books it as a standing factory gap.

**Decision:** Problem gate.

| Reviewer | Role | Date | Verdict |
|---|---|---|---|
| @mdarabi | operator | pending | via the introducing PR's operator review |

## 3. Goals, metrics, and guardrails

### 3.1 Goals as falsifiable hypotheses

| # | We believe that… | Will result in… | Measured by… |
|---|---|---|---|
| G1 | a read-only reading room over the corpus (browse · search · trace · progress) | doc-system questions answered in the browser without raw files or grep | §5.8's human-judgment row (real tasks completed UI-only) |
| G2 *(qualitative)* | one class registry and one read-only presentation path | document classes stay consistent as the corpus grows | inspection: every class routes through its declared presentation mode (NR-class evidence) |

### 3.2 Goals → signals → metrics

Skipped: no instrumentation for local read-only tooling (§5.7); G1's measurement is the human-judged done criterion with its rubric — an explicit measurement plan in place of telemetry.

### 3.3 Guardrails and decision rule

Skipped: the §1 guardrail is carried as non-regression rows (NR-1..3) whose breach blocks release via §5.8; no staged rollout exists to gate (§6.5).

### 3.4 Non-goals and waiting room

| ID | Non-goal | Why refused |
|---|---|---|
| NG-1 | UI-native comment or edit machinery | review stays in the operator conversation; the reading room remains read-only (brainstorm:workbench-ui#O-1) |
| NG-2 | Any write path from the UI process — repo edits, verb execution, gate runs | read-only charter; a write surface on a served app needs a safety story v1 has no appetite for (#O-2) |
| NG-3 | Live tracker/PR/network state | the dashboard derives from the committed corpus only; no second source of truth beside the docs (#O-3) |
| NG-4 | Re-implementing the plan viewer or adding an authoring renderer | each tool keeps one job (#O-4) |
| NG-5 | Serving beyond loopback, auth surfaces, or any deployment story | local tool for the repo's own humans; anything wider is a different product (#O-5) |

| ID | Waiting room | Revisit when / intended version |
|---|---|---|
| WR-1 | Tracker/PR enrichment (execution state beside the corpus-derived stages) | the work-tracker port ships and a real review week shows the corpus-derived view insufficient |
| WR-2 | Actions from the UI (running verbs, applying edits) | an explicit operator charter under the earned-autonomy posture; never implied by v1 |
| WR-3 | In-place comments or edits | a separately approved write-surface design exists; until then review stays in the operator conversation |
| WR-4 | Multi-repo / aggregated reading room | a second repo onboards and cross-repo questions actually occur |

## 4. Solution space

### 4.1 Approach and alternatives considered

The chosen shape: an on-demand Go server in the managed system footprint that scans the working tree at request time, presents documents through a class registry, resolves citation grammars into links, and derives the progress view from the corpus's machine faces plus git history. Nothing is persisted, nothing is written, and nothing is served beyond loopback.

| Option | Sketch | Why not chosen |
|---|---|---|
| A (chosen) | ephemeral Go server using class-registered read-only presentations over a live corpus scan | — |
| B | static site generation (a committed or scratch-built site per state) | staleness by construction; search and stage derivation want request-time state; a committed site is a giant derived artifact the repo's own rules refuse |
| C | extend a document-review application into the corpus shell | couples navigation to a separate application and gives a read-only tool an unnecessary write-oriented dependency |
| Do nothing | terminal + code-host file view | the standing toil of §2.1; blind to the doc system's structure |

### 4.2 Scope perimeter

- **In (plan of record)** — corpus catalog by class; safe readable document views plus plan-viewer pages; search with anchor-ID exact lookup; citation-grammar link resolution; per-feature progress derived from the committed corpus; hand-off of the exact document path to the operator conversation and links to tracked references; loopback-only ephemeral serving.
- **Future (believed next, not committed)** — the waiting room (WR-1..4).
- **Out** — NG-1..5.

### 4.3 Experience: flows and states

| Flow | Serves | User does | System responds | States covered |
|---|---|---|---|---|
| F1 browse | J1 | opens the reading room; picks a class; opens a doc | catalog grouped by class; rendered view via the codified transform | ideal · empty (class with no docs reads as such) · error (per-doc defect card, corpus stays browsable) |
| F2 search | J2 | types a query or an ID | results with document, heading/anchor context, open-at-match; ID hits ranked first as exact matches | ideal · empty (honest zero-results) · partial (a class unparseable → named in results as degraded) |
| F3 trace | J3 | clicks a citation in a rendered doc | navigates to the cited document at the cited anchor; a dangling citation shows named-unresolved | ideal · error (unresolved named, never silent) |
| F4 progress | J4 | opens the pipeline view | per-feature stage row: furthest artifact, pending human gates, links into the artifacts | ideal · empty (feature with only a record) · partial (contested/ambiguous state rendered as such, never guessed) |

Loading states are sub-perceptual or spinner-free by budget (NFR-1); no flow accepts input that could be destroyed. Accessibility: keyboard-navigable list/detail and search (the two humans include no screen-reader requirement yet; revisit at WR-4 scale). Localization: none — the corpus is English by convention.

### 4.4 Scenarios

**The stage question.** Mid-review, the operator wonders whether a feature's design landed and whether its review verdict was clean. Reading room: open progress, find the feature row, see design present + review verdict linked, click through to the report — three interactions, zero terminal.

**The cold start.** A future engineer, day one on an onboarded repo, opens the reading room and reads the vision rendered, browses the principles registry, and searches the two IDs a PR review cited at them. The corpus teaches itself; nobody wrote a tour.

**Decision:** Solution gate.

| Reviewer | Role | Date | Verdict |
|---|---|---|---|
| @mdarabi | operator | pending | via the introducing PR's operator review |

## 5. System requirements

### 5.1 Functional requirements

| ID | Requirement (EARS) | Rationale | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|
| FR-1 | When launched in a governed repo, the reading room shall present the document corpus grouped by class, with every governed document appearing exactly once and opening into a rendered view. | the catalog is the map the corpus lacks | Must (rank 1) | low | fixture corpus: every committed doc of every class appears once and opens; a doc outside the classes does not appear | Test | G1, J1, F1 |
| FR-2 | When presenting a document, the reading room shall route it through the class registry: safe read-only presentation for markdown classes and the Go plan viewer for plans. | one declared presentation path per class; no drift between views | Must (rank 2) | med | golden fixtures pin each class's output; a plan document routes through the plan viewer; hostile markup remains inert | Test | G2, F1 |
| FR-3 | When given a search query, the reading room shall return matches across the corpus with document, heading/anchor context, and open-at-match navigation; a query matching an anchor-ID grammar shall rank its exact matches first. | search is the index humans lack; IDs are the corpus's native names | Must (rank 3) | med | fixture queries: free-text hits carry doc + context and open at the match; ID queries land their defining row first; a no-hit query returns an explicit empty state | Test | G1, J2, F2 |
| FR-4 | When rendering a document containing corpus citation grammars, the reading room shall render each citation as navigation resolving to the cited document and anchor; an unresolvable citation shall render as named-unresolved. | provenance is the doc system's spine; hand-resolving it is the toil | Must (rank 4) | med | fixture docs: each grammar class resolves to its target anchor; a dangling citation renders named-unresolved and is never silently plain text | Test | G1, J3, F3 |
| FR-5 | The reading room shall present a per-feature progress view derived solely from the committed corpus and git history, naming each feature's furthest pipeline artifact and its pending human gates, with each cell linking to its artifact. | "what stage is X at" is today a multi-directory investigation | Must (rank 5) | med | fixture corpus with features at distinct stages: each row names the expected stage and gates; adding an artifact to a fixture feature advances its row; the view is identical with networking disabled | Test | G1, J4, F4 |
| FR-6 | Where a document is presented, the reading room should expose its exact repository path and tracked reference for use in the operator conversation, while itself performing no comment capture, edit, or write. | "interact" means carrying precise context to the flow that owns review | Should | low | demo on a live doc: the hand-off supplies the exact path and reference without retyping; a write-audit of the UI process across the demo shows zero repo writes | Demo | J1, F1 |
| FR-7 | The reading room process shall create and modify no files inside the repository, writing at most regenerable scratch under the gitignored render directory. | read-only is the charter, not a preference | Must (rank 6) | med | filesystem audit across a browse/search/trace/progress session: zero repo writes; any scratch confined to the gitignored path | Test | G1 |
| FR-8 | When the operator ends the session, the reading room shall terminate its server process, leaving no background process or daemon. | idle costs nothing; no watch modes | Must (rank 7) | low | process audit after session end: the server and children are gone; a repeated launch/end cycle accumulates nothing | Test | G1 |
| FR-9 | When the corpus changes on disk, the reading room should reflect the changed state on subsequent requests without restart. | reviews happen against moving drafts | Should | low | edit a fixture doc mid-session: the next catalog/search/progress request reflects it | Test | J4 |

**Non-regression (NR-n)**

| ID | Existing behavior protected | Why protected | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|
| NR-1 | The write-side doc tooling (fold, gate, plan verbs) behaves byte-identically with the reading room present | the UI is read-side only; the write side is frozen contract | med | existing suites pass unchanged; fold/gate outputs on the fixture corpus byte-identical before and after | Test | G2 |
| NR-2 | Reading-room launch and document presentation require no secondary language runtime or package installation | the workbench must follow the Go-only system policy | low | a clean Go build launches and serves the fixture corpus without any package-install step | Test | G2 |
| NR-3 | The CI gate surface is unchanged by v1 | a reading tool must not grow gate authority | low | the docs-gate workflow and check set are diff-identical | Inspection | G2 |

### 5.2 Unwanted behavior and failure handling

| ID | If… (trigger) | Then the system shall… | User sees | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|---|
| UB-1 | If launched outside a governed repo or the corpus root is missing | refuse with a defect naming the path, operation, reason, and remedy, leaving no server running | named defect in the terminal, nothing served | Must | low | fixture matrix: each case yields the quadruple and a process audit shows no listener | Test | FR-1 |
| UB-2 | If a class presentation or the plan viewer is unavailable | serve the catalog with that view degraded to a named remedy per document, never a blank page or dead server | degraded document view naming the remedy | Must | med | unavailable-view fixture: catalog and search still serve; affected views name the remedy | Test | FR-2 |
| UB-3 | If a document fails its class parse (unfoldable ledger, malformed machine face) | render that entry as its named defects while the rest of the corpus stays fully browsable | a defect card for the broken doc | Must | med | corrupted-fixture run: the broken entry shows its defects; all other flows unaffected | Test | FR-1, FR-5 |
| UB-4 | If the server cannot bind its port, or another session is already serving this repo | refuse with a defect naming the path, operation, reason, and remedy, leaving no half-served state | named defect in the terminal | Must | low | occupied-port and second-launch fixtures: each yields the quadruple; a process audit shows no partial listener | Test | FR-8 |

### 5.3 Permissions and abuse cases

Access model: the local user, unauthenticated, over loopback — the same trust boundary as filesystem access to the clone; there are no roles and no remote access by construction (NG-5, NFR-2).

| ID | Actor and intent | Vector | Impact | Severity | Mitigated by | Verify |
|---|---|---|---|---|---|---|
| ABUSE-1 | any local process reading the served corpus | loopback HTTP while a session runs | none beyond existing filesystem privilege; no write path exists | low | loopback-only bind; zero write endpoints (NFR-2); ephemeral lifetime (FR-8) | Test |
| ABUSE-2 | hostile document content attempting script execution or egress from a rendered view | crafted markdown/fixture in the corpus | exfiltration or UI-driven action | med | rendering through the codified transforms only (FR-2); no external assets and no egress (NFR-3); no write endpoints (NFR-2) | Test (hostile fixture) |
| ABUSE-3 | a hostile web page in the local browser probing the loopback server | cross-origin requests / DNS rebinding from an arbitrary web origin | corpus content read by a remote origin | med | NFR-2's session secret and Host validation; FR-8's ephemeral lifetime shrinks the window | Test (rebinding/cross-origin fixture) |

### 5.4 Data

No state is created or changed: the reading room holds request-time in-memory structures only, persists no index (appetite exclusion), and writes at most gitignored scratch (FR-7). **Egress: none** — no content leaves the machine (NFR-3); nothing to retain, migrate, or decommission; no privacy surface beyond the repo's own files.

### 5.5 Interfaces and integrations

| Interface | Direction | Purpose | Contract (format, ranges, timing) | Failure modes | Owner |
|---|---|---|---|---|---|
| the ledger projections and source maps | in | catalog, search, progress inputs | each class's machine face as committed | missing/diverged face → UB-3 defect card | the amendment model |
| the design index's machine face | in | design list + progress joins | the index contract as committed | malformed → UB-3 | the design-conventions doc |
| plan sources and folds | in | plan rows in catalog and progress | the plan class contract | unfoldable → UB-3 | the plan class design |
| class registry + Go plan viewer | in | readable document views | one declared presentation mode per class; plan pages use the plan-viewer contract | unavailable → UB-2 degradation | this feature / plan-viewer |
| git history | in | freshness and stage recency | read-only queries of the local clone | absent history → progress renders without recency, stated | — |
| operator conversation | out (handoff) | FR-6 review context | exact repository path + tracked reference; no UI-side review machinery | unavailable → path and reference remain copyable | operator |

### 5.6 Non-functional requirements

| ID | Area | Requirement | Fit criterion | Verify |
|---|---|---|---|---|
| NFR-1 | Performance | The reading room shall reach a usable catalog within 3 seconds of launch and shall answer each navigation or search request within 300 milliseconds at the 95th percentile, on the reference dev machine at a corpus of up to one thousand documents. | p95 launch-to-catalog ≤ 3s; p95 request ≤ 300ms; p99 ≤ 1s on the fixture corpus scaled to 1,000 docs | Test |
| NFR-2 | Security | The server shall bind only the loopback interface, shall expose zero mutating endpoints, and shall refuse requests that lack its session secret or carry an unexpected Host. | bind audit shows loopback only; the route table contains no mutating route; a secret-less and a rebound-Host request are each refused; a write-audit across mutation attempts shows zero effects | Test |
| NFR-3 | Offline integrity | The reading room shall function fully with no network egress, all assets local. | a session under a deny-all egress filter completes every flow; an asset audit of served pages finds zero external references | Test |
| NFR-4 | Maintainability | Where a new document class enters the corpus's contracts, the reading room shall admit it without modification of the surfaces serving existing classes, beyond registering the new class. | the class-onboarding conformance fixture adds a synthetic class: catalog, search, and progress admit it with zero edits to existing class surfaces (diff inspection) | Test + Inspection |

### 5.7 Instrumentation — the tracking plan

Skipped: no metrics ship (§3.2); success is §5.8's human-judgment row with a rubric — the explicit measurement plan for local tooling.

### 5.8 Definition of Done

| ID | Done criterion | Source | Verify | Required evidence | Blocking? |
|---|---|---|---|---|---|
| DOD-1 | All Must FRs pass their fit criteria | FR-1..5, FR-7..8 | Test | CI suite run | yes |
| DOD-2 | No unwanted-behavior or non-regression row broken | UB-1..3, NR-1..3 | Test + Inspection | suite results + diff audit | yes |
| DOD-3 | Abuse and NFR rows hold | ABUSE-1..2, NFR-1..3 | Test | suite results incl. hostile fixture + egress-off run | yes |
| DOD-4 | The operator completes real doc-system tasks using only the reading room | G1 | Human judgment — @mdarabi; rubric: across one normal review week, five real tasks (find/trace/stage questions) reach their answer with zero raw-file or grep fallbacks | task log on the Tracking ref | yes |
| DOD-5 | Shipped through the factory pipeline from an approved technical design | header Target release; the repo's design-first rule | Demo | the design, its plan, and the merged task PRs | yes |

### 5.9 AI behavior and evals

Skipped: deterministic rendering, search, and derivation; no model-driven behavior ships.

## 6. Delivery

### 6.1 Phasing and milestones

Skipped: one shippable surface at v1 scope; §5.8 is the exit bar (§6.4 folds its lenses into the blocking rows); no pilot/beta staging exists for a single-operator local tool.

### 6.2 Dependencies

| Dependency | Type | Needed by | Status | Owner |
|---|---|---|---|---|
| approved technical design (design-first rule; resolves Q1–Q5) | approval | implementation | not started | operator gate |
| Go toolchain in source mode or released workbench binary | system | first run | standard system-release path | installer |

### 6.3 Risks, assumptions, and rabbit holes

| # | Risk / assumption / rabbit hole | Likelihood | Impact | Mitigation or test | Owner |
|---|---|---|---|---|---|
| R1 | scope creep toward a control tower (tracker state, monitoring) | M | M | NG-3/WR-1 perimeter; appetite exclusions; review against §4.2 | @mdarabi |
| R2 | search-quality expectations outrun the simple index | M | M | anchor-ID exact match first-class (FR-3); fit criteria pin behavior; ranking beyond that is waiting-room material | tech design |
| R3 | class presentation grows into a second authoring path | L | H | NG-1/NG-2, GET-only route table, and write-audit fixtures | @mdarabi |
| R4 | class presentation contracts drift under the UI | M | M | NR-1/NR-2 plus conformance fixtures presenting each class through its registered path | tech design |

### 6.4 Release criteria

| Lens | Criterion | Evaluated by |
|---|---|---|
| Functionality | DOD-1 | CI |
| Usability | DOD-4's rubric | operator task log |
| Reliability | UB rows green incl. degraded-deps and hostile fixtures (DOD-2/3) | CI |
| Performance | NFR-1 within targets (DOD-3) | CI on the scaled fixture |
| Supportability | degraded modes name their remedy (UB-2) — the tool self-explains | fixture assertions |

### 6.5 Rollout and reversibility

Skipped: local-only tooling, no staged exposure; reversibility is not launching the process — no flag, experiment, or migration exists. (No high/critical-risk rows exist in §5; the §8 safety-plan check passes on that basis.)

### 6.6 Launch readiness

Skipped: solo operator, local tool; the usage note lands with implementation in the workbench documentation — no support, legal, pricing, marketing, or localization surface exists.

### 6.7 Post-launch review

- **Review trigger** — DOD-4's rubric window completes (one normal review week with five real tasks) · **Owner** — @mdarabi
- **Verdict recorded** — adopt / iterate / kill, with the task log as evidence, on the header's Tracking ref

## 7. Alignment

### 7.1 FAQ

**Internal**

- **Q:** Why a Go server? — **A:** It follows the repository's one-language policy, ships through the system release, and needs no secondary runtime or package-install step.
- **Q:** Why not the code host's web view? — **A:** It renders files, not the doc system: no projections-as-primary, no anchor-ID search, no citation-grammar resolution, no stage derivation — and it isn't offline or draft-aware.
- **Q:** The idea says "interact" — why read-only? — **A:** Interaction is carrying the exact document path and tracked reference into the operator conversation (FR-6). Writes have owners already; duplicating them into a served app is the refused NG-1/NG-2 ground.

### 7.2 Open questions

| # | Question | Owner | Needed by | Status |
|---|---|---|---|---|
| Q1 | Search mechanics: live scan + in-memory index shape; ranking; anchor-ID lookup as a distinct path (brainstorm:workbench-ui#G-1) | tech design | implementation | open |
| Q2 | Server lifecycle: port allocation, single-instance semantics, shutdown/idle behavior (#G-2) | tech design | implementation | open |
| Q3 | Launch story: the Go verb/entry point and its home in the system release (#G-3) | tech design | implementation | open |
| Q4 | Progress derivation: the stage function over corpus signals + git freshness; honest rendering of contested states (#G-4) | tech design | implementation | open |
| Q5 | Conversation hand-off mechanics without UI-side write machinery (#G-5) | tech design | implementation | open |

### 7.3 Decision log

| Date | Decision | Alternatives considered | Why | Who |
|---|---|---|---|---|
| 2026-07-10 | Go server, homed in the managed footprint (brainstorm:workbench-ui#D-1) | separate frontend runtime | follows the one-language policy and system-release path | @mdarabi |
| 2026-07-10 | Read-only with precise context hand-offs (#D-2) | UI-native comments; read-write verbs | no write surface without a safety story | @mdarabi |
| 2026-07-10 | Progress derives from the committed corpus only (#D-3) | live tracker/PR state; both tiered | offline, drift-free, one source of truth; enrichment parked (WR-1) | @mdarabi |
| 2026-07-10 | Register one read-only presentation per class (#D-4) | ad hoc per-route rendering | one declared route per class | @mdarabi |
| 2026-07-10 | Slug `workbench-ui` (#D-5) | doc-portal; repo-ui | ties to the durable capability area | @mdarabi |
| 2026-07-10 | Reading-room framing (#D-6) | control tower; workbench portal | access surface with zero new authority — the seed's own words | @mdarabi |
| 2026-07-10 | Bet: kill terminal-only access; success human-judged on real tasks (#D-7) | review-speed framing; visibility framing | inversion test; acceptance style already exercised by plan-viewer | @mdarabi |
| 2026-07-10 | Appetite small-medium with named exclusions (#D-8) | small (no progress view); medium (Should-tier now) | serves all three seed jobs without speculative surface | @mdarabi |

### 7.4 Changelog

| Date | Change | Author |
|---|---|---|
| 2026-07-10 | 000-base drafted from the gated brainstorm record for review | @mdarabi (factory-assisted) |
| 2026-07-10 | prd-review round-1 advisories applied: ABUSE-3 + NFR-2 hardening, UB-4, FR-2 fit bar, NFR-1 restated, NFR-4 class extensibility (review:workbench-ui-000 FND-1..5) | prd-review (unmanned) |

## 8. Contract readiness — the executability gate

| Check | Pass? | Notes |
|---|---|---|
| Problem basis accepted — §2 problem gate signed (T2) | pending | awaits the introducing PR's operator review (§2.6) |
| Zero solution-blocking TBDs or open questions (§7.2) | pass | Q1–Q5 block implementation via the tech design, not the solution shape |
| Every Must FR has a fit criterion and a verification method (§5.1) | pass | FR-1..9 each carry both |
| Every requirement row carries a risk tier; high/critical rows have a declared safety plan | pass | no high/critical rows — local, read-only, loopback (§6.5 note) |
| Non-goals drawn (§3.4); non-regression rows named (§5.1) | pass | NG-1..5; NR-1..3 |
| Every primary job (§2.3) is covered by a requirement, a non-goal, or a waiting-room entry | pass | J1→FR-1/2, J2→FR-3, J3→FR-4, J4→FR-5/9; interaction beyond reading → FR-6 + NG-1/2 |
| Every key flow (§4.3) has functional, failure, and non-regression coverage (§5.1–5.2) | pass | F1→FR-1/2/UB-2/3 · F2→FR-3/UB-3 · F3→FR-4 · F4→FR-5/UB-3; NR-1..3 span all |
| Every primary metric (§3.2) has instrumentation or an explicit measurement plan | pass | §3.2 skipped; DOD-4 rubric is the measurement plan |
| Every blocking DoD row traces to a requirement, guardrail, release criterion, or human-judgment rubric (§5.8) | pass | DOD-1..5 sourced; DOD-4 names owner + rubric |
| Risks and assumptions have owners (§6.3) | pass | R1..R4 owned |
| Human-judgment criteria name an owner and a rubric (§5.8) | pass | DOD-4 |
| DoD complete: every blocking row sourced and evidenced; every n/a explained (§5.8) | pass | no n/a rows; skips carry reasons |
