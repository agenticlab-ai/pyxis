# AGENTS.md — operating manual for agents working in this repository

Canonical, CLI-neutral agent instructions for `agenticlab-ai/pyxis`.
`CLAUDE.md` is a symlink to this file; Cursor, Codex, and OpenCode read it
directly. Everything here is binding for any automated author.

## What this repo is

**Pyxis** is an open-source software factory: a core engine that orchestrates
agent workers to turn product ideas into merged, verified software, with every
integration (issue tracker, code host, CI, agentic CLI, trigger) behind an
adapter — no privileged substrate.

The pipeline that turns a raw idea into a verified feature — each stage, the
skill or mechanism that runs it, the human gates, and what is built today — is
mapped in `.pyxis/pipeline.md` (the operational projection of
`docs/product-vision.md` §4).

**Current phase: product definition and design; the only code is doc-system tooling (docgate).**
The documents under `docs/` *are* the deliverable; treat them with code-level
discipline. Do not scaffold engine/application code — implementation starts
only from an approved technical design, and the open architecture questions
(e.g. engine state) are decided in design docs, never silently in code.

**Go is the implementation language — for everything, not just the engine.**
The engine ships as a single static Go binary plus adapters, and tooling,
scripts, glue, gates, and tests are Go too. **Bash is permitted only where a
shell is a hard, demonstrated requirement** (see the language policy below);
"it's quicker in bash" is never that. When implementation begins, the rules in
"Go — language rules" below are binding.

## Durable knowledge — record it once where it belongs

Do not create mandatory per-session logs or "nothing happened" records.
Record decisions, constraints, and lessons that change future work once, in
the artifact they belong to: the relevant requirement, design, issue, pull
request, incident, or typed memory record. Keep the evidence needed to
understand the decision. Use the existing amendment and supersession rules
when the owning artifact is frozen.

A session with nothing durable to add creates no documentation. Operational
logs are optional, local, and ignored by Git. Never automatically import
session logs or transcripts into durable memory. Existing substantive logs
are historical evidence; they do not impose a logging obligation or govern
future work. Preserve an unresolved decision in its owning work item rather
than creating a separate session diary.

## Shared records and local execution logs

Keep project state and decision evidence in Git so another user can understand
and resume the same work. This includes Carina work items and their history,
Argus authority and delivery ledgers, review reports and feedback dispositions,
review/recovery/routing evidence refs, and retained evaluation results.

Execution diagnostics belong under ignored `.pyxis/logs/<producer>/`:
cli, executor, watch, phasemeter, codegate, agents, reviews,
contract-gate, sessions, and tests. Never stage these files or include them in
a release payload. Optional session notes use `.pyxis/logs/sessions/`.
Logs must not be the only record of a shared decision or outcome.

Credentials, locks, retry queues, caches, and rate-limit state retain their
appropriate state directories. Local runtime state is not execution logging.

## Layout — where to find what

| Path | What it is |
|---|---|
| `logs/` | Historical session evidence retained where substantive. New operational logs are optional and ignored by Git; durable decisions belong in their owning artifacts. |
| `docs/product-vision.md` | The **abstract governing layer**: north star, pipeline, capability map, trust model. Append-only once approved. Links to **no** concrete document. |
| `.pyxis/logs/` | Local execution diagnostics grouped by producer; ignored and excluded from releases. Shared workflow records stay in their owning Git artifacts. |
| `.pyxis/pipeline.md` | The **operational pipeline map**: every stage (raw idea → verified feature), the skill/mechanism that runs it, the human gates, and built-vs-not status. The distributable projection of `product-vision.md` §4 — ships with `.pyxis/`; §4 governs on conflict. |
| `.pyxis/model-routing.md` | The **model-routing policy** (routing-as-data): planning records independent implementation and review requirements; execution selects qualified native harness/model/effort choices. Deterministic tooling spends no inference. |
| `docs/architecture-principles.md` | The citable principle registry (**AP-n** product + engineering tiers, **RW-n** rewrite-era; medium-independence, contractual data, doc mutability/link rules, …). Every technical design cites the principles it honors or deviates from. |
| `docs/prds/README.md` | The **requirements index**: the parity baseline (67 features, stable IDs like `IMPL-3`, `GATE-4`) + pointers to the area PRDs. Requirements, *not* implemented capability. |
| `docs/prds/*.md` | Ten area PRDs (baseline requirements, as-built provenance) + `vnext-requirements.md` (**VNEXT-1..13**, the new ground). |
| `.pyxis/templates/` | Instantiable document templates: `prd-template.md` (the PRD skeleton — routing header, tiers, gates, EARS + fit-criterion requirements, Appendix A writing rules), `prd-amendment-template.md` (the typed-amendment envelope + op blocks for evolving an approved PRD per `docs/tech-design/prd-amendment-model.md`), `tech-design-template.md` (the design skeleton — header incl. `Supersedes`/`Serves`, §1–§7 + Principles, per `docs/tech-design/tech-design-conventions.md`), `prd-review-report-template.md` + `design-review-report-template.md` (the unmanned reviewers' evidence packages, per `docs/tech-design/prd-review-artifacts.md` · `tech-design-review-artifacts.md`), `brainstorm-record-template.md` (the stage-1 session record — typed sections, `D/F/G/O/T/P-n` anchors, checklist + citation-grammar appendices, per `docs/tech-design/brainstorm-record-model.md`), `design-brainstorm-record-template.md` (the stage-3 session record — the four design organs: Basis, ALT-n ledger, panel log, seam map, per `docs/tech-design/design-brainstorm-record-model.md`), and the **Argus data-class templates** — `problem-ledger-template.md` · `delivery-ledger-template.md` · `authority-contract-template.md` (per `docs/tech-design/argus-artifacts.md`). Copy, fill, delete the guidance italics. |
| `docs/domain-model.md` | The engine's entity catalog, invariant registry (INV-*), ports/adapters map, and open design questions (§10). |
| `docs/tech-design/` | **Technical designs** (immutable once merged — merge is the freezing event): factory header template + `## Principles` section citing AP/RW IDs. |
| `docs/design-principles.md` | Compatibility **pointer** to `docs/architecture-principles.md` for factory tooling — no content of its own. |
| `docs/observability/incidents.md` | The **incident registry** (append-only, `I1…`, four-field entries; lessons graduate into principles). |
| `docs/wiki/` | Distilled, **descriptive** knowledge (flat entries, routing headers, freely editable via PR) — **never authoritative**: on conflict, the sources each entry cites govern. Rules + index in its README. |
| `docs/research/` | **Research records** — descriptive landscape/market studies informing product direction (e.g. the AI-software-engineer study behind the Argus vision). Wiki-class posture: never authoritative — on conflict the cited sources govern; dated snapshots, superseded by newer studies rather than rewritten. |
| `docs/brainstorms/` | **Brainstorm records, both classes** — stage-1 session outputs (`<slug>.md`, from `brainstorm-record-template.md`; produced by `/feature-brainstorm`) and stage-3 outputs (`<slug>-tech-design.md`, from `design-brainstorm-record-template.md`; produced by `/tech-design-brainstorm`). Provenance artifacts, frozen at merge — a reopened effort starts a fresh suffixed record, never an edit. Contracts: `docs/tech-design/brainstorm-record-model.md` · `docs/tech-design/design-brainstorm-record-model.md`. |
| `docs/reviews/` | **Review reports** — the unmanned reviewers' evidence packages (one report per subject per moment: `<feature>-<NNN>.md` from `prd-review-report-template.md` via `/prd-review`; `<design-slug>.md` for the design class). Ride the subject's introducing PR; append-only after each verdict. Contracts: `docs/tech-design/prd-review-artifacts.md` · `docs/tech-design/tech-design-review-artifacts.md`. |
| `.pyxis/skills/argus/` | The **`/argus` skill** — the AI Engineer (drive guide `.pyxis/authoring/argus.md`): hand it a problem, it drives the pipeline as decision principal. Holds `authority-contract.md` (the versioned tier ladder — policy as data, human-changed only; born at `floor`) and `fixtures/` (the EVAL-1..4 corpus + DINV defect fixtures). Designs: `docs/tech-design/argus-agent-v1.md` (behavior) · `argus-artifacts.md` (data); PRD: `docs/prds/argus/`. |
| `docs/problems/<slug>/` | **Problem instances** — per problem a `problem.md` (`problem-ledger/v1`: idea verbatim + append-only-forever `A-n` authority entries) and `delivery.md` (`delivery-ledger/v1`: the typed `E-n` drive journal + derived state, closing once at delivery). Written by `/argus`; cited `problem:<slug>#A-n` · `delivery:<slug>#E-n`. Contract: `docs/tech-design/argus-artifacts.md`. |
| `.pyxis/skills/` + `.pyxis/scripts/` + `.pyxis/authoring/` | The **vendor-neutral agent surface** (AP-25); `.agents/skills` (current Codex), `.codex/skills` (Codex compatibility), `.claude/skills`, and `.claude/scripts` are symlinks into it — vendor dirs hold pointers, never content. |
| `.pyxis/vendor/mermaid/` | The pinned Mermaid browser bundle the Go plan viewer inlines for offline diagrams. Re-vendor via `.pyxis/vendor/mermaid/refresh.sh`. |
| `.pyxis/vendor/litellm-prices/` | The pinned LiteLLM per-model price + context-window map the pipeline-measurement tool's cost derivation reads (`docs/tech-design/pipeline-measurement-v2.md` §3.12). Re-vendor via `.pyxis/vendor/litellm-prices/refresh.sh`. |
| `internal/adapters/` | **Port-axis medium implementations** — `adapters/<port>/<medium>/` (harness, spend, codehost, tracker) plus `adapters/common/` for shared medium support (GitHub auth/identity). Ports stay on the consumer side (`internal/executor/port`, `internal/worktracker`, `internal/phasemeter/spend`). Engine packages import ports only; they may import `adapters/common`. `cmd/` wires port adapters. |
| `cmd/docgate` + `internal/docgate/` | **docgate** — the PRD-ledger fold + the gate/writer verb set (enumerated by `docgate` usage and `docs/techspec.md`, grown by amendment) (PRD: `docs/prds/docgate/`; design: `docs/tech-design/docgate-v1.md`). CI: `.github/workflows/docs-gate.yml`; desk: `.pyxis/hooks/` shims through `.githooks` (a symlink), activated with the shared `go run ./cmd/hooks-setup` installer. |
| `.pyxis/scripts/tests/` | Compatibility-shim tests; run the applicable suite after changing a shim. |
| `.pyxis/scratch/` | Regenerable plan previews and authoring scratch; ignored by Git. |

## The document system — rules that gate your edits

1. **Link direction is concrete → abstract, never the reverse.** PRDs and
   technical designs cite the vision/principles that govern them. The vision
   and principles link to **no** concrete document — do not add such links,
   and do not add requirement-ID citations (VNEXT-n, GATE-n, …) to the vision.
2. **Mutability classes — the prompt-level gate.** As an automated author you
   must **never modify**: `product_vision`, `architecture_principles`, or
   incident entries once merged (append-only — add entries, never rewrite;
   the one sanctioned mutation is an entry's `Links:` line growing at
   graduation), nor any per-feature PRD ledger entry or technical design
   once **merged** — merge is the freezing event. Frozen designs evolve by
   supersession (a *new* document citing the old); frozen PRDs evolve only
   by typed amendments (rule 8). While a document's PR is still open it is
   a draft and may be edited. docgate enforces this at push and CI
   (`check-immutable`, `check-governing`, `lint-incidents`); the
   pre-commit hook runs only the tree-local checks — history checks need
   a merge base. This rule is the prompt-side half.
3. **No living rollups before code.** `docs/prd.md` and `docs/techspec.md`
   (the live "what the system does" docs) must not exist until the first
   capability actually ships; they are then seeded empty and kept current as
   shipped structure changes — a convention since docgate amendment 017
   retired FR-10's per-PR gate; freshness reads from git history. A
   capability claimed with no code behind it is documentation drift.
4. **Requirement IDs are stable.** Never renumber SCAF/PLAN/IMPL/GATE/FLEET/
   IDEN/GOV/KNOW/OBS/BENCH/VNEXT IDs; supersede or append instead.
5. **The vision stays standalone.** No references to prior or external
   systems in `product-vision.md` — a new reader must need no outside context.
   (Internal engineering docs keep their provenance notes.)
6. **Governing docs are human-authored inputs.** Never derive, regenerate, or
   "reconcile" the vision/principles from other content; changes to them come
   from the operator.
7. **Technical designs follow the factory convention** (operator ruling,
   2026-07-01, so the existing factory can drive pyxis's implementation):
   designs live at `docs/tech-design/<slug>.md` (path re-ruled 2026-07-06; formerly `docs/design/`, freeing the plain name for the upcoming `docs/ux-design/`), instantiated from
   `.pyxis/templates/tech-design-template.md` — header table incl.
   `Supersedes`/`Serves`, required `## Principles` section citing AP/RW IDs
   (deviation-with-reason, never silent omission); evolution and index
   conventions per `docs/tech-design/tech-design-conventions.md`. Incidents append to `docs/observability/incidents.md`
   (stable IDs `I1…`, four-field entries, graduation → `AP-<n>`).
   `docs/design-principles.md` is a **compatibility pointer** to the
   principles registry — never put principle content there. These fixed paths
   are a bootstrap-era binding: they become per-repo configurable via the
   pyxis-config surface (issue #4), with these paths as defaults.
8. **Per-feature PRDs are ledgers** (decided in
   `docs/tech-design/prd-amendment-model.md`): a real PRD lives at
   `docs/prds/<feature>/` — `000-base.md` instantiated from
   `.pyxis/templates/prd-template.md`, evolving **only** by typed amendments
   (`.pyxis/templates/prd-amendment-template.md`; dense-numbered, immutable
   once merged) folded into the generated projection. **Never hand-edit
   `effective.md` or `effective.source.json`** — if the projection is wrong,
   fix the amendment op or the fold, never the output. The gate is
   enforced: the `docs-gate` CI context and the pre-push hook (activate once
   per clone: `go run ./cmd/hooks-setup`) run `docgate gate`;
   regenerate projections with `go run ./cmd/docgate fold docs/prds/<feature>`. The flat files directly under `docs/prds/` are the
   **placeholder requirement corpus** (operator ruling): requirement
   *sources* the real PRDs will cite, exempt from these conventions, never
   retrofitted.
9. **§0 "Read to rule" is human-only.** PRDs born under `prd-template/v2`
   open with §0 — an informative routing section for the gate-signing
   human reviewer. It binds nothing: on any conflict the body governs, and
   automated consumers (reviewers per their ledgers' adoption, the
   compiler, traceability, workers, stage flows) never read, cite, or obey
   it. Amendments whose ops change content §0 routes to update it
   wholesale in the same op set (`op: supersede §0`, full replacement
   body, declared at the computed contract-changing tier). Contract:
   `docs/prds/read-to-rule/` · `docs/tech-design/read-to-rule-v1.md`.

## Default implementation rules

- **Work on the current branch.** Never create or switch branches unless
  explicitly asked; a PR needs a `<type>/<slug>` feature branch.
- **Stage by explicit path — never `git add -A` / `-u`.** Multiple agent
  sessions share this clone; add-all sweeps another session's untracked WIP
  into your commit (this has already happened here once).
- **One concern per PR**, conventional-commit title. Every PR needs a human
  approval and a human (write-access) merge — do not merge your own work.
  The ledger half is machine-enforced: docgate's `check-single-amendment`
  (FR-27) fails any PR that adds more than one `docs/prds/*/NNN-*.md` entry.
- **Stacked PR trains: delete each head branch at merge, then verify the
  train landed.** A dependent PR retargets to main only when its base
  branch is *deleted*; a surviving base swallows the child's merge into an
  orphaned branch while every badge reads "merged" — the badge certifies
  the PR's base, never main (AP-13; incident I2). Merge bottom-up, delete
  on merge, and finish with `git merge-base --is-ancestor <head-tip>
  origin/main` for every PR in the train before reporting it landed.
- **Substantial doc changes require operator review** of the source document
  and proposed edits before the PR is raised. Record approval in the owning
  artifact; use the existing plan viewer when an HTML plan preview helps.
- **Ambiguity → ask.** If a rule above and an instruction conflict, or product
  intent is unclear, escalate to the operator with options; never guess a
  silent resolution.

## Go — language rules (binding once engine code exists)

**Language policy — Go by default, bash only on proven necessity**
- **Every new executable artifact is written in Go** — engine, CLIs, tooling,
  janitors, gates, test harnesses, one-off utilities. The predecessor's core
  was tens of thousands of lines of untestable shell; escaping that is a
  founding reason this rewrite exists. Do not re-grow it here.
- **The burden of proof is on bash.** A shell script is admissible only when a
  shell is a *hard requirement* of the integration point itself — e.g. a git
  hook entry, a curl-able bootstrap line that must run before any binary
  exists, or a third-party interface that is shell-only. Convenience, habit,
  or "it's just a small script" never qualify.
- **Admissible bash stays a thin shim**: minimal lines, no logic beyond
  locating and delegating to a Go binary (or fetching one), bash-3.2-clean,
  and covered by a test. If a shim starts accumulating logic, that logic moves
  to Go.
- **Existing bash is bootstrap-era, not precedent.** Maintain existing shims,
  do not extend their logic, and move behavior into Go as capabilities evolve.

**Toolchain & hygiene**
- Latest stable Go; modules with `go.mod`/`go.sum` at the repo root.
- `gofmt` is non-negotiable; `go vet` and `golangci-lint` must be clean;
  `go test -race ./...` green before any proposal.
- Standard library first. Every new dependency is a justified decision (small
  supply-chain surface is part of the security posture); no dependency for
  what twenty lines of stdlib can do.

**Structure — the hexagon in Go**
- Thin mains under `cmd/<binary>/`; everything else under `internal/`
  (`pkg/` only for an intentionally public API, never by default).
- Small, focused packages named for what they *provide*; no `util`, `common`,
  or `helpers` packages.
- **Ports are interfaces defined on the consumer side** (the engine declares
  what it needs): `internal/executor/port`, `internal/worktracker`,
  `internal/phasemeter/spend`, and the rest. Adapters live under
  `internal/adapters/<port>/<medium>/` and implement exactly one port. The
  engine core imports **no port-adapter package** — `go test ./internal/adapters`
  enforces that (`adapters/common` is shared medium support and is allowed);
  composition roots under `cmd/` blank-import port adapters so they
  self-register (AP-27). Accept interfaces, return concrete types.

**Errors & control flow**
- Wrap with `fmt.Errorf("…: %w", err)` adding context at each hop; branch with
  `errors.Is`/`errors.As`; sentinel errors are part of a package's contract.
- Every error is handled or returned — a discarded error (`_ =`) requires a
  comment saying why. No panics across package boundaries; `panic` is for
  programmer error only. Fail-closed on destructive paths (INV-6).

**Concurrency**
- `context.Context` is the first parameter of anything that blocks, does IO,
  or can be cancelled; honor cancellation promptly.
- No goroutine without an owner: who starts it, how it stops, and what happens
  on error must be explicit (prefer `errgroup`/worker pools over
  fire-and-forget).
- Guard shared state by construction (channels or small mutex-owning types);
  the race detector always runs in CI and locally.

**Data & observability (AP-5 / AP-6)**
- Cross-boundary data is schema-versioned structs with explicit field tags;
  additive evolution only; no `map[string]any` contracts.
- Config and policy files parse **closed**: unknown keys are errors.
- Structured logging via `log/slog` with trace context propagated end-to-end;
  no `fmt.Println` debugging in committed code.

**Testing**
- Table-driven tests; golden files for rendered/serialized artifacts.
- Unit tests are hermetic (no network, no clock/randomness without injection).
- The minimal-local adapters double as the engine's test harness; every port
  ships a conformance suite that all its adapters must pass.

## Tooling rules

- Shell scripts hold a **bash 3.2 floor** (macOS stock bash): no bash-4-only
  syntax (`;&`, `${var^^}`, …).
- After touching a shim or vendor tree, run its consumer tests and the
  applicable checks in `.pyxis/scripts/tests/`.
