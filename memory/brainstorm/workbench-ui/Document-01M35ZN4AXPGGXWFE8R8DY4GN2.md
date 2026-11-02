# Workbench UI (the reading room) — brainstorm record

| Field | Value |
|---|---|
| **In one line** | A read-only, locally served web UI over the repo's document corpus — browse presented docs, search them, trace citations, and see per-feature pipeline progress from one Go service. |
| **Relevant-when** | brainstorming or contracting web/browser access to the document system; checking prior rulings on served apps, corpus search, progress dashboards, or UI-native writes; revisiting plan-viewer's served-app refusal |
| **Idea** | Build a web UI within the managed system footprint that offers another way to read generated documents, search the corpus, and see progress. *(operator, 2026-07-10; implementation language normalized to the repository's Go policy)* |
| **Operator** | @mdarabi |
| **Date(s)** | 2026-07-10 |
| **Status** | gated: ready-for-prd |
| **Tracking** | — *(the introducing PR, once filed)* |
| **Feeds** | docs/prds/workbench-ui/ |
| **Prior records** | — |
| **Template version** | brainstorm-record/v1 |

## Orientation

| Source | Why loaded |
|---|---|
| docs/product-vision.md | §5 capability map — the engineer-workbench row's "rendered review surfaces" and the knowledge-system row; §1 inversion test (terminal toil is a factory gap); the capability litmus test |
| docs/prds/README.md | coverage scan: no baseline area row or vNext row contracts a corpus UI, search, or progress surface |
| docs/prds/plan-viewer/000-base.md | nearest neighbor: its NG-1 (no own annotation engine), NG-2 (no served application, daemon, or watch mode), NG-3 (plans only; the doc renderer owns general markdown), and its appetite's "excludes … served app, or tracker enrichment" |
| docs/prds/engineer-workbench.md | the workbench area PRD (header scan): dev-environment install surface; no served-UI requirement exists there |
| docs/prds/vnext-requirements.md + vnext-self-improvement.md | scan for UI/dashboard/search rows: none |
| docs/wiki/glossary.md | vocabulary check before coining terms (ledger, projection, routing surface, tier) |
| docs/brainstorms/ (headers) | prior-record scan: no record covers UI/browser access to the corpus |
| AGENTS.md | the Go-by-default language policy and the document-review boundary the UI must respect |

**Overlaps surfaced:** plan-viewer NG-2 refused "a served application, daemon, or watch mode" *for its own scope* (a plan page is a static artifact) — this idea deliberately revisits serving as a corpus-level concern; the operator charters it here without disturbing plan-viewer's frozen boundary. plan-viewer NG-1 (annotation engine) is **upheld**: the UI presents documents read-only and returns exact source references to the operator conversation.

## Framings

- **Reading room** — an access surface over the document system: browse the rendered corpus, search it, trace citations, read per-feature pipeline status. Zero new authority — the docs and gates stay the truth; the UI makes them legible. Opens: legibility now, later repos inherit it at onboarding.
- **Control tower** — progress-first: the dashboard is the product, documents are the drill-down. Pulls toward factory monitoring and tracker/execution state (excluded from v1 by D-3), so the charter would immediately strain its own perimeter.
- **Workbench portal** — the UI half of the engineer workbench, chartered to eventually front tooling actions (launch reviews, run verbs). v1 would still be read-only but the thesis promises interaction — a bigger contract than the idea needs.

**Destination:** the **reading room** (D-6) — operator confirmed the access-surface thesis: another way to *access* the information, for the operator today and any engineer on an onboarded repo later; anti-segment external/public consumers and CI. Interaction beyond reading is hand-offs to existing flows, not new machinery.

## Decisions

### D-1 — runtime and home: one Go service inside the managed footprint

- **Question**: implicit in the seed — what runtime and where does it live?
- **Answer**: one Go service in the managed system footprint, shipped with the system release and using standard-library HTTP and templates.
- **Reasoning**: the repository's Go policy applies to every new executable artifact; a single Go service avoids an additional package runtime and shares the release path used by the rest of the tooling.
- **Anchor**: 2026-07-10 · seed message
- **Status**: live

### D-2 — write surface: read-only, with launch-points into existing flows

- **Question**: what may v1 do beyond rendering — read-only + launch-points, UI-native annotation, or read-write verbs?
- **Answer**: read-only + launch-points (recommendation confirmed): browse/search/progress are pure reads; "interact" means handing the exact document path or tracked reference back to the operator conversation, or opening the tracked PR — no UI-native writes.
- **Reasoning**: one human review channel, not two (plan-viewer NG-1's logic, upheld); the tooling layer's zero-git-writes posture; a write surface on a served app needs a safety story v1 has no appetite for.
- **Anchor**: 2026-07-10 · round 1 (batched ask, recommendations attached)
- **Status**: live

### D-3 — progress scope: pipeline dashboard derived from the corpus only

- **Question**: what does "see the progress" mean for v1?
- **Answer**: a per-feature stage view derived purely from the committed corpus (ledgers + status ladders, designs and their index, plans, review reports, incidents) plus git history for freshness — no tracker or network calls (recommendation confirmed).
- **Reasoning**: works offline, nothing to drift, no second source of truth beside the docs; live tracker/PR enrichment is deferred with a trigger (→ the PRD's waiting room).
- **Anchor**: 2026-07-10 · round 1 (batched ask, recommendations attached)
- **Status**: live

### D-4 — reuse posture: compose the existing surfaces

- **Question**: how does the UI relate to document presentation and plan-viewer's pages?
- **Answer**: compose them (recommendation confirmed): the UI is the corpus shell — index, search, navigation, dashboard — presenting registered document classes and plan-viewer output, then returning the exact source reference to the operator conversation for review.
- **Reasoning**: reuse-not-rebuild; each surface keeps one job; independent re-rendering would fork the transform layer the repo already codified.
- **Anchor**: 2026-07-10 · round 1 (batched ask, recommendations attached)
- **Status**: live

### D-5 — feature slug: workbench-ui

- **Question**: the permanent kebab-case slug naming the ledger, branch, and citations?
- **Answer**: `workbench-ui` (recommendation confirmed over `doc-portal`, `repo-ui`).
- **Reasoning**: ties the feature to the engineer-workbench capability area ("rendered review surfaces"); the slug is permanent so the tie should be to the durable capability, not to one surface's nickname.
- **Anchor**: 2026-07-10 · round 1 (batched ask, recommendations attached)
- **Status**: live

### D-6 — destination: the reading-room framing

- **Question**: which framing is the thesis — reading room, control tower, or workbench portal?
- **Answer**: reading room (recommendation confirmed): an access surface over the document system with zero new authority; primary segment the operator, later any engineer on an onboarded repo; anti-segment external/public consumers and CI.
- **Reasoning**: matches the seed's own words ("another way to access the information"); the other framings enlarge the charter toward monitoring or interaction that D-2/D-3 already scoped out.
- **Anchor**: 2026-07-10 · round 2 (batched ask, recommendations attached)
- **Status**: live

### D-7 — goals direction: kill terminal-only access

- **Question**: what is the bet, and what observable outcome makes v1 a success?
- **Answer**: the bet: corpus questions ("where is X decided", "what governs Y", "what stage is feature Z at") get answered in the browser without opening raw files. Success, human-judged: the operator completes a real doc-system navigation/review task using only the UI — no raw markdown, no grep (recommendation confirmed).
- **Reasoning**: the inversion test — terminal spelunking through the corpus is machinery-operation time, a factory gap; the acceptance style mirrors plan-viewer's human-judged done criterion, which the operator has already exercised.
- **Anchor**: 2026-07-10 · round 2 (batched ask, recommendations attached)
- **Status**: live

### D-8 — appetite: small-medium, with named exclusions

- **Question**: the spend/scope bound for v1, machine-native?
- **Answer**: small-medium (recommendation confirmed): one ephemeral loopback Go service in the managed footprint plus its templates, fixtures, and tests; reuses the Go plan presenter and provides safe read-only presentation for other classes; zero rollout/support surface (local, read-only). Excludes: tracker/network integration, auth beyond loopback, persistence/index artifacts, and any write path.
- **Reasoning**: big enough to serve all three seed jobs (browse, search, progress) — the small option left "see the progress" unserved; small enough to refuse the Should-tier speculation (saved views, traceability graphs) before any code exists.
- **Anchor**: 2026-07-10 · round 2 (batched ask, recommendations attached)
- **Status**: live

## Facts

- **F-1**: plan-viewer refused "a served application, daemon, or watch mode" (NG-2) and excluded "any annotation engine, served app, or tracker enrichment" from its appetite — scoped to plan rendering — source: docs/prds/plan-viewer/000-base.md §3.4 + header
- **F-2**: the managed footprint and system release already ship Go binaries and pinned data assets without a runtime package-install step — source: installer and repository-layout designs
- **F-3**: the language policy makes every new executable artifact Go by default — source: AGENTS.md Go rules
- **F-4**: the corpus already exposes machine-readable faces the UI can consume without new authority: PRD projections and source maps, the tech-design index's machine face, plan folds, review reports, the incident registry, and the wiki — sources: the ledger/projection glossary entries and each class's contract doc
- **F-5**: content rules forbid frozen PRD/design prose from citing footprint paths or volatile counts — the contract must name the home abstractly — source: the docgate ledger (its content-purity requirement) + prd-template rule 14

## Fog

- **G-1** (non-blocking — blocks: tech design): search mechanics — live scan with an in-memory index vs any prebuilt artifact; ranking; whether anchor-ID lookup (FR-n, AP-n, D-n, I-n) is a distinct exact-match path
- **G-2** (non-blocking — blocks: tech design): server lifecycle — port allocation, single-instance semantics, shutdown/idle behavior consistent with idle-costs-nothing
- **G-3** (non-blocking — blocks: tech design): launch story — the Go binary or verb, its release registration, and its source-mode entry
- **G-4** (non-blocking — blocks: tech design): progress derivation — how stage status per feature is computed from corpus signals (ledger status, design index rows, review reports, plan presence) and git freshness, and how contested states render honestly
- **G-5** (non-blocking — blocks: tech design): hand-off mechanics to the operator conversation — how "review this doc" returns the exact source reference without the UI growing review machinery or a write path

## Out of scope

- **O-1**: UI-native annotation or comment machinery — review and confirmed edits stay in the operator conversation (from D-2)
- **O-2**: any write path from the UI process — no repo edits, no verb execution, no gate runs (from D-2)
- **O-3**: live tracker/PR/network state in v1 — the dashboard derives from the committed corpus only (from D-3)
- **O-4**: reimplementing the plan-viewer's graph semantics (from D-4)
- **O-5**: serving beyond loopback, any auth surface, or any deployment story (from D-8)

## Candidate glossary terms

- **T-1**: reading room — the read-only corpus access surface (browse · search · trace · progress) chartered as workbench-ui; avoid: portal, dashboard (names one panel, not the surface), viewer (taken by plan-viewer's page)

## Parked technical constraints

- **P-1**: the reading room is a Go executable shipped through the ordinary system release, with no language-policy exception (from D-1 + F-3)
- **P-2**: loopback-only bind, no auth surface, ephemeral process — idle costs nothing
- **P-3**: zero GitHub writes and zero git mutations from the UI process — the posture shared with the review tooling
- **P-4**: no persistent index or derived artifacts in the repo — in-memory or gitignored scratch only, so nothing new can drift or conflict
- **P-5**: frozen contract prose names the UI's home abstractly (the managed system footprint) per the content-purity rule (F-5)

## Gate

| Field | Value |
|---|---|
| **Verdict** | ready-for-prd |
| **Date** | 2026-07-10 |
| **Operator** | @mdarabi |
| **Covering source** | — |
| **Coverage check** | pass — all six inputs covered, zero blocking fog (§2.1 ← D-6 · §2.2 ← D-6 · §2.3 ← D-6/D-7 · §3.1 ← D-7 · §3.4 ← O-1..O-5 · appetite ← D-8) |
