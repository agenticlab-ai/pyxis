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

## Session logs — every session, every stage, no exceptions

**Before you do anything else in a session, open a session log.** This binds
every automated author, in every CLI (Claude, Codex, Cursor, OpenCode), at
every step of the pipeline — a skill run, a plan task, a gate fix, a one-off
chore. It is a prerequisite, not a closing chore: open the log at your first
real step, write to it the moment you have to put something to a human, and
leave it behind when you stop.

**The file.** One file per session, at
`logs/<YYYY-MM-DD>-<HHMM>-<short-slug>.md` — the date and time you created it
first (take them from `date '+%Y-%m-%d-%H%M'`, local time), then a few words
naming the job, e.g. `logs/2026-07-30-1742-e2e-budget-fix.md`. Commit it by
explicit path along with your work. Entries are append-only: once you have
written one, add to it, never rewrite it.

**What belongs in it — only what a human had to decide.** If you hit a problem
and worked it out yourself, it does **not** go in the log, however annoying it
was. Solving things is the job, not a log entry. The log records only the ones
you could not settle on your own: the problem stopped you, you had to put it to
a human, and a human made the call.

These are the kinds of problem that reach that bar — but only once you have
actually escalated one:

- a skill's own steps contradict each other, the repo, or a doc, and you
  cannot tell which side is right
- a documented path, command, flag, or artifact is not there, and there is no
  safe way to guess what was meant
- a gate, hook, or tool blocks you for a reason unrelated to your change, and
  the only way past it would be to override a guard
- two rules disagree, and following either one breaks the other
- an input you were told to consume is missing, so no later step can run
  honestly
- an unknown you genuinely cannot settle — any answer you pick might be the
  wrong one, and the cost of being wrong is real
- a limit, permission, or access wall you cannot get past

**What never belongs in it:**

- anything you settled yourself — however much time it cost you
- review findings — `/prd-review`, `/tech-design-review`, `/impl-review`, and
  `/e2e-review` write their own reports; those are not session-log entries
- bugs in the feature you are building or the bug you were asked to fix —
  that is the work itself and it belongs to the work's own artifacts
- failures caused by your own half-finished change
- opinions, ideas, and things you would do differently

The test: **did a human have to make the call? If not, it does not go in.**

**How to write it.** Plain English, short sentences, no jargon, no strings of
IDs. Keep every reference — file paths, requirement or doc IDs, PR and issue
numbers, commands, error text — out of the prose and down on the `Refs` line,
so a human can read the problem and the decision straight through without
decoding anything. Each entry carries the time you hit the problem and the time
the human decided, says plainly what you asked, and says plainly what they
answered.

Copy this skeleton:

```markdown
# Session log — <what this session was doing>

Started: 2026-07-30 17:42 · CLI: Claude Code · Stage or skill: /e2e-design

## P-1 — <the problem in one plain line>

- **Found:** 2026-07-30 17:42
- **Problem:** What went wrong, in two or three plain sentences.
- **Blocked:** What this stopped me from doing, and why I could not settle it
  myself.
- **Asked:** 2026-07-30 17:50 — what I put to the human, in plain words.
- **Decided:** 2026-07-30 17:58 — what the human decided.  (or: Still waiting)
- **How:** What was done to carry that decision out.
- **Refs:** paths, IDs, PR numbers, commands, error text.
```

Number entries `P-1`, `P-2`, … within the file. If a decision is still
outstanding when you stop, leave it as `Still waiting` and say so in your
hand-off — an open question recorded honestly is worth more than a tidy log.

**If you escalated nothing**, still leave the file, with the header and one
line: `Nothing escalated this session.` A run that needed no human is worth
recording too, and the habit of opening the file is what makes you notice when
one does.

The log is the record of the calls a human had to make for the factory. When
one turns out to be a lesson worth keeping forever, graduate it separately
through `/incident` into `docs/observability/incidents.md` — the registry is
for permanent lessons, the log is for what needed a human today.

## Layout — where to find what

| Path | What it is |
|---|---|
| `logs/` | **Session logs** — one file per agent session, `<YYYY-MM-DD>-<HHMM>-<slug>.md`, recording only the problems that session had to escalate to a human, and what the human decided (see "Session logs" above). Mandatory for every session in every CLI. Not things the agent solved itself, not review findings, not product bugs. |
| `docs/product-vision.md` | The **abstract governing layer**: north star, pipeline, capability map, trust model. Append-only once approved. Links to **no** concrete document. |
| `.pyxis/pipeline.md` | The **operational pipeline map**: every stage (raw idea → verified feature), the skill/mechanism that runs it, the human gates, and built-vs-not status. The distributable projection of `product-vision.md` §4 — ships with `.pyxis/`; §4 governs on conflict. |
| `.pyxis/model-routing.md` | The **model-routing policy** (routing-as-data): task complexity → model tier, adjusted by subject risk. Least-powerful-that-suffices — cheapest for mechanical work, strongest for judgment on a high-risk surface; deterministic work spends no inference. The bootstrap policy an orchestrator reads today; a mechanical picker consumes it in the engine era. |
| `docs/architecture-principles.md` | The citable principle registry (**AP-n** product + engineering tiers, **RW-n** rewrite-era; medium-independence, contractual data, doc mutability/link rules, …). Every technical design cites the principles it honors or deviates from. |
| `docs/prds/README.md` | The **requirements index**: the parity baseline (67 features, stable IDs like `IMPL-3`, `GATE-4`) + pointers to the area PRDs. Requirements, *not* implemented capability. |
| `docs/prds/*.md` | Ten area PRDs (baseline requirements, as-built provenance) + `vnext-requirements.md` (**VNEXT-1..13**, the new ground). |
| `.pyxis/templates/` | Instantiable document templates: `prd-template.md` (the PRD skeleton — routing header, tiers, gates, EARS + fit-criterion requirements, Appendix A writing rules), `prd-amendment-template.md` (the typed-amendment envelope + op blocks for evolving an approved PRD per `docs/tech-design/prd-amendment-model.md`), `tech-design-template.md` (the design skeleton — header incl. `Supersedes`/`Serves`, §1–§7 + Principles, per `docs/tech-design/tech-design-conventions.md`), `prd-review-report-template.md` + `design-review-report-template.md` (the unmanned reviewers' evidence packages, per `docs/tech-design/prd-review-artifacts.md` · `tech-design-review-artifacts.md`), `brainstorm-record-template.md` (the stage-1 session record — typed sections, `D/F/G/O/T/P-n` anchors, checklist + citation-grammar appendices, per `docs/tech-design/brainstorm-record-model.md`) , `design-brainstorm-record-template.md` (the stage-3 session record — the four design organs: Basis, ALT-n ledger, panel log, seam map, per `docs/tech-design/design-brainstorm-record-model.md`), the **Argus data-class templates** — `problem-ledger-template.md` · `delivery-ledger-template.md` · `authority-contract-template.md` (per `docs/tech-design/argus-artifacts.md`) — and `e2e-design-template.md` (the **`e2e-design/v1`** doc class: six required sections + header `Basis` join, per `docs/tech-design/e2e-verification-artifacts.md` §3.1). Copy, fill, delete the guidance italics. |
| `docs/domain-model.md` | The engine's entity catalog, invariant registry (INV-*), ports/adapters map, and open design questions (§10). |
| `docs/tech-design/` | **Technical designs** (immutable once merged — merge is the freezing event): factory header template + `## Principles` section citing AP/RW IDs. |
| `docs/design-principles.md` | Compatibility **pointer** to `docs/architecture-principles.md` for factory tooling — no content of its own. |
| `docs/observability/incidents.md` | The **incident registry** (append-only, `I1…`, four-field entries; lessons graduate into principles). |
| `docs/wiki/` | Distilled, **descriptive** knowledge (flat entries, routing headers, freely editable via PR) — **never authoritative**: on conflict, the sources each entry cites govern. Rules + index in its README. |
| `docs/research/` | **Research records** — descriptive landscape/market studies informing product direction (e.g. the AI-software-engineer study behind the Argus vision). Wiki-class posture: never authoritative — on conflict the cited sources govern; dated snapshots, superseded by newer studies rather than rewritten. |
| `docs/brainstorms/` | **Brainstorm records, both classes** — stage-1 session outputs (`<slug>.md`, from `brainstorm-record-template.md`; produced by `/feature-brainstorm`) and stage-3 outputs (`<slug>-tech-design.md`, from `design-brainstorm-record-template.md`; produced by `/tech-design-brainstorm`). Provenance artifacts, frozen at merge — a reopened effort starts a fresh suffixed record, never an edit. Contracts: `docs/tech-design/brainstorm-record-model.md` · `docs/tech-design/design-brainstorm-record-model.md`. |
| `docs/reviews/` | **Review reports** — the unmanned reviewers' evidence packages (one report per subject per moment: `<feature>-<NNN>.md` from `prd-review-report-template.md` via `/prd-review`; `<design-slug>.md` for the design class). Ride the subject's introducing PR; append-only after each verdict. Contracts: `docs/tech-design/prd-review-artifacts.md` · `docs/tech-design/tech-design-review-artifacts.md`. |
| `docs/e2e/<feature>/` | **e2e design artifacts** — per-feature home co-locating `design.md` (the **`e2e-design/v1`** doc class: six required sections + header `Basis` join, frozen at operator approval) and `review.md` (its `e2e-review-report/v1` review). A lightweight, ungoverned class — no `docgate` fold/lint, no technical-design-index entry, no `Serves`/`Supersedes` semantics. Template: `.pyxis/templates/e2e-design-template.md`; well-formedness checker: `internal/e2e/docclass`. The feature's e2e plan is not homed here — it is an ordinary plan-new `plan/v1` at `docs/plans/<feature>/`. Contract: `docs/tech-design/e2e-verification-artifacts.md`. |
| `.pyxis/skills/doc-review/` | The `/doc-review` skill: render a doc → inline browser annotation → propose-and-confirm edits. |
| `.pyxis/skills/argus/` | The **`/argus` skill** — the AI Engineer (drive guide `.pyxis/authoring/argus.md`): hand it a problem, it drives the pipeline as decision principal. Holds `authority-contract.md` (the versioned tier ladder — policy as data, human-changed only; born at `floor`) and `fixtures/` (the EVAL-1..4 corpus + DINV defect fixtures). Designs: `docs/tech-design/argus-agent-v1.md` (behavior) · `argus-artifacts.md` (data); PRD: `docs/prds/argus/`. |
| `docs/problems/<slug>/` | **Problem instances** — per problem a `problem.md` (`problem-ledger/v1`: idea verbatim + append-only-forever `A-n` authority entries) and `delivery.md` (`delivery-ledger/v1`: the typed `E-n` drive journal + derived state, closing once at delivery). Written by `/argus`; cited `problem:<slug>#A-n` · `delivery:<slug>#E-n`. Contract: `docs/tech-design/argus-artifacts.md`. |
| `.pyxis/skills/` + `.pyxis/scripts/` + `.pyxis/authoring/` | The **vendor-neutral agent surface** (AP-25); `.agents/skills` (current Codex), `.codex/skills` (Codex compatibility), `.claude/skills`, and `.claude/scripts` are symlinks into it — vendor dirs hold pointers, never content. |
| `.pyxis/scripts/` | `lavish-driver.sh` (open/listen/reply/end loop) and `render-doc.mjs` (the codified markdown→HTML transform). |
| `.pyxis/vendor/lavish-axi/` | The pinned, **un-bundled** review-surface runtime. Deps install via `.pyxis/scripts/setup-lavish.sh`. |
| `.pyxis/vendor/mermaid/` | The pinned mermaid browser bundle `render-doc.mjs` inlines so ` ```mermaid ` fences draw as diagrams. Re-vendor via `.pyxis/vendor/mermaid/refresh.sh`. |
| `.pyxis/vendor/litellm-prices/` | The pinned LiteLLM per-model price + context-window map the pipeline-measurement tool's cost derivation reads (`docs/tech-design/pipeline-measurement-v2.md` §3.12). Re-vendor via `.pyxis/vendor/litellm-prices/refresh.sh`. |
| `cmd/docgate` + `internal/docgate/` | **docgate** — the PRD-ledger fold + the gate/writer verb set (enumerated by `docgate` usage and `docs/techspec.md`, grown by amendment) (PRD: `docs/prds/docgate/`; design: `docs/tech-design/docgate-v1.md`). CI: `.github/workflows/docs-gate.yml`; desk: `.pyxis/hooks/` shims via `core.hooksPath .githooks` (a symlink). |
| `.pyxis/scripts/tests/` | `test-lavish-driver.sh`, `test-lavish-api-contract.sh`, `test-render-doc.mjs`, `test-docgate-hook.sh` — run after touching the driver, the renderer, or a vendor tree. |
| `.lavish/` | Regenerable render scratch. Gitignored — never commit it. |

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
   per clone: `git config core.hooksPath .githooks`) run `docgate gate`;
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
- **Substantial doc changes go through `/doc-review`** (rendered inline
  annotation with the operator) before the PR is raised.
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
- **Existing bash is bootstrap-era, not precedent.** The doc-review/lavish
  scripts predate the engine; maintain them, do not extend them, and replace
  them with Go equivalents as the engine's capabilities arrive.

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
  what it needs); adapters implement them in their own packages. The engine
  core imports **no adapter package** — enforce the boundary with a lint rule,
  not convention. Accept interfaces, return concrete types.

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

## Tooling rules (doc-review / lavish)

- Never run the vendored CLI via `npx`, and never esbuild-bundle it — it runs
  via local `node` with deps installed once by `bash .pyxis/scripts/setup-lavish.sh`
  (lockfile-pinned). See `.pyxis/vendor/lavish-axi/README.md` for the history.
- Drive the review loop only through `.pyxis/scripts/lavish-driver.sh`
  (`open` → background `listen` → `reply` → `end`); render markdown only via
  `.pyxis/scripts/render-doc.mjs` — never hand-author HTML for a doc.
- Shell scripts hold a **bash 3.2 floor** (macOS stock bash): no bash-4-only
  syntax (`;&`, `${var^^}`, …).
- After touching the driver, renderer, or a vendor tree, run the tests in
  `.pyxis/scripts/tests/` and keep them green.
