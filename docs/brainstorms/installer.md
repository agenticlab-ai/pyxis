# installer — brainstorm record

| Field | Value |
|---|---|
| **In one line** | Install Pyxis into a target repository: a config-driven installer that materializes the factory — gates, skills, doc system, hooks, CI — in a brand-new repo, with existing-repo adoption as the designed-for follow-up constraint. |
| **Relevant-when** | brainstorming or contracting installation, distribution, releases/versioning of the engine, repo scaffolding/onboarding, the engine↔instance split, or the `pyxis-config/` per-repo surface; checking prior rulings on portability, offline/local floor, or what an installed repo must contain |
| **Idea** | "Now at the last step to make sure whatever we are building in Pyxis can be ported to other repositories, we need to build an installer for the Pyxis. So right now, the Pyxis has been evolved with inside the Pyxis repo. So now we should think about, okay, we are building a new repo from scratch, how we can install Pyxis into that. The next step would be how should we install Pyxis on existing repository, which is a separate piece as a follow-up, but it should be a guiding constraint when we are designing it for the new repository. So the idea would be a user follows a installation script, and then Pyxis will be installed on that repo, assuming it's a new repo, which gives them all the skills, all the tools, such as the docgate, everything we have built. And, obviously, this requires a config driven approach, meaning that when they're installing, they wanna decide, oh, I'm using a GitHub or I'm using Carina, how I'm gonna push the docgate… where I'm gonna run the local CI or the remote CI? If I am gonna do it as a remote CI, how to install it automatically on my GitHub account so on and so forth. So think about that perspective and come up with a list of things that needs to happen for this installer to work properly." (operator, 2026-07-19, voice transcript; spelling of product names normalized) |
| **Operator** | @mdarabi |
| **Date(s)** | 2026-07-19 |
| **Status** | gated: ready-for-prd |
| **Tracking** | — |
| **Feeds** | docs/prds/installer/ |
| **Prior records** | — |
| **Template version** | brainstorm-record/v1 |

## Orientation

| Source | Why loaded |
|---|---|
| `docs/product-vision.md` §2/§3/§5/§8/§9/§11 | the governing anchors: open-source, "engineers simply install it" + offline floor ("simplest form: a local git repository and a plain task file"); "one install command plus a design doc"; capability map row "a repo becomes factory-managed in one command"; §8 `pyxis-config/` per onboarded repo; §9 auto-update/A-B/telemetry; §11 "the platform loop" |
| `docs/prds/README.md` | requirements index + "not in the baseline" list (org-installer program, auto-update channels = new ground); confirms no per-feature PRD ledger exists for installation |
| `docs/prds/scaffolding-and-onboarding.md` (SCAF-1..5) | the parity corpus this feature's PRD will cite: scaffolding wizard (<60s, idempotent, wizard+flags, dry-run), adopt-existing (two-phase permission split, --update idempotent, required-checks calibration), profiles-as-data, org sweep, welcome path |
| `docs/prds/vnext-requirements.md` (VNEXT-8/10) | install-adjacent new ground: autonomy level is a per-repo dial set at onboarding; policy engine = org > repo > defaults policy files an installer must seed |
| `docs/wiki/glossary.md` | term hygiene + a live contract dependency: **verification manifest** is already contracted to live in `pyxis-config/` (prd:implementation-verification FR-5) — a directory that does not exist on disk yet |
| `AGENTS.md` | layout table (what `.pyxis/` vs `docs/` contain), language policy (Go; bash only as bootstrap/hook shims), tooling rules (un-bundled vendor), rule 7 (fixed doc paths are bootstrap-era, deferred to "the pyxis-config surface", issue #4) |
| repo inventory (this session, 2026-07-19; cmd/, .pyxis/**, .github/workflows/, resolver, config) | code reality of what "installing Pyxis" must materialize — entered below as F-n facts with file:line sources |
| `docs/brainstorms/` listing | no prior record covers installation/distribution/onboarding; `implement-now.md` read for record conventions only |

**Overlaps surfaced:**
- `SCAF-1/SCAF-2/SCAF-5` (scaffolding-and-onboarding.md) — **not covered; this is the parity slot.** The area PRD is placeholder requirement corpus ("nothing here is built in Pyxis yet"); this feature is its pyxis-native successor. SCAF-2 (adopt existing) is the operator's stated follow-up-but-constraint.
- `prds/README.md` "Not in the baseline" — "Two-mode restructure + org-installer program… designed, in flight" refers to the **harbormaster-side** program; prior art, not coverage. Likewise "Auto-update channels / A/B cohorts / global telemetry — only seeds exist."
- `product-vision.md` §8 — `pyxis-config/` is **specified** as every onboarded repo's extension point but unimplemented; `prd:implementation-verification` FR-5 already binds the verification manifest to it (glossary). This feature likely forces the surface into existence.
- Adjacent parity rows this feature will touch, not absorb: BENCH-1 (workbench install), FLEET-6 (install/uninstall lifecycle), FLEET-9 (org portability), GOV-1/2/4 (primitive manifest, drift repair, sanctioned customization) — per the README index.
- `VNEXT-8` — the autonomy-level dial is set per repo "so adoption can start conservative"; an installer is where it first gets set.

## Framings

*Three genuinely distinct theses of what the installer is. The operator picks or hybridizes; the Destination records the choice.*

- **A — Converge onto a release.** The installer is the visible face of the **engine↔instance split**: Pyxis becomes a *versioned, released product* (binaries + a `.pyxis/` payload of skills/templates/hooks/guides), and installing = **converging a target repo onto a pinned release** with recorded install-time answers. Thesis: *install, re-run, upgrade, and (later) adopt-existing are one converge-to-manifest code path; the wizard is a thin front end over it.* Implications: distribution machinery first (release all engine artifacts, generalize the docgate resolver); idempotency and drift-repair fall out structurally; existing-repo support is the same verb pointed at a non-empty tree. Opens: auto-update channel (§9), GOV-2-style drift repair, fork/mirror distribution.
- **B — Stage-0 onboarding wizard.** The installer is the **SCAF-1/SCAF-5 successor**: a <60s question-driven wizard whose product is the *day-1 experience* — answer tracker/CI/CLI questions, get a green factory-managed repo, land in a welcome path pointing at the first `/feature-brainstorm`. Thesis: *the feature is onboarding UX; distribution is plumbing bought minimally (even copy-from-a-clone would do for v1).* Implications: ships fastest; converge/manifest/upgrade deferred to a later feature; risk that copy-once semantics get rebuilt when upgrade and adopt-existing arrive.
- **C — Portability forcing function.** The installer is an **engine program with a test harness attached**: un-hardcode the pyxis-repo assumptions, make every gate honor config paths, prove the offline floor (vision §2: local git repo + plain task file, zero GitHub calls), and the install script is merely the *acceptance harness* that proves "no privileged substrate" (AP-1) is real. Thesis: *the deliverable is portability itself; the installer is its proof.* Implications: engine work first, UX later; closest to FLEET-9's zero-edit-fork spirit; slowest route to a usable day-1 experience.

**Destination:** **A — converge onto a release.** Pyxis becomes a versioned, released product (engine binaries + the `.pyxis/` payload); installing = converging a target repo onto a pinned release with recorded install-time answers, so install, re-run, upgrade, and the existing-repo follow-up are **one converge code path**. B's wizard is A's front end, not a separate architecture; C's portability items (un-hardcoding, config-honoring gates, offline floor) become A's acceptance criteria. — Operator confirmed the recommendation (2026-07-19): converge-to-manifest is the single decision that makes the stated guiding constraint (existing-repo adoption) fall out structurally instead of becoming a second product.

## Decisions

### D-1 — the problem: the factory is trapped in its own repo

- **Question**: what problem does the installer solve?
- **Answer**: adopted as written — *Pyxis has no existence outside its own repository.* Every capability shipped so far (gates, skills, doc system, executor) runs only where it was developed; there is no way to stand up a second factory-managed repo, so the factory's compounding value is trapped in one repo and the product claim ("engineers simply install it", vision §2) is unproven. The installer converts Pyxis from a repo that governs itself into a product that can govern any repo.
- **Reasoning**: matches the idea's own framing ("the last step to make sure whatever we are building can be ported"); grounded in F-1..F-8 (no distribution beyond docgate, no init verb, fresh repo fails the gates).
- **Anchor**: 2026-07-19 · t2
- **Status**: live

### D-2 — segments: operator-first, adopter-shaped

- **Question**: who is the primary segment, who is the design audience, and who is the anti-segment?
- **Answer**: primary = the operator standing up their own next repos (the first real installs, dogfooding). Design audience = the external open-source adopter — a solo engineer or 2–5-person team on their own infra (vision §3) — so defaults, docs, and the wizard are written stranger-proof even though v1 validates on the operator's repos. Anti-segment (v1): org-wide adoption sweeps across many existing repos (SCAF-4 territory) and non-GitHub hosts (GitLab/Bitbucket/Jira).
- **Reasoning**: dogfood validates the artifact while it stays shaped for the product's real audience; org sweep and host-adapter breadth are separate later features.
- **Anchor**: 2026-07-19 · t3
- **Status**: live

### D-3 — v1 jobs: install + idempotent repair; upgrade ready-not-polished; adopt-existing deferred

- **Question**: which jobs are in the v1 contract — J1 stand-up-new-repo, J2 re-run-to-repair, J3 upgrade, J4 adopt-existing?
- **Answer**: **J1 + J2 ship v1** — install, and idempotent re-run that repairs a half-finished or hand-damaged install. **J3** must be structurally ready in v1 (manifest, version pins, converge) but its UX (channels, changelogs, migration notes) is a waiting-room item. **J4** (existing-repo adoption) is the waiting-room follow-up with an explicit revisit trigger, per the idea's own framing.
- **Reasoning**: re-run/repair is the same converge code path — shipping it is the proof the Destination's semantics are real, not copy-once in disguise; upgrade polish before a second consumer exists is premature; J4 stays the designed-for constraint.
- **Anchor**: 2026-07-19 · t4
- **Status**: live

### D-4 — the bet and observable success

- **Question**: what is the bet, and what observable success does it bind to?
- **Answer**: adopted — **bet**: making Pyxis a versioned release + converge installer *now* (rather than after more capabilities ship) turns every subsequent feature into distributable product by default, and the next new repo pays it back immediately. **Observable success**: (1) a brand-new repo goes from nothing to all-gates-green, pipeline-runnable via one command + wizard answers, target ≤5 minutes end-to-end; (2) checkpoint: the next real project repo is created via the installer and runs a feature through the pipeline with zero hand-copying from the pyxis repo; (3) deleting/corrupting an installed file and re-running converge repairs it to green.
- **Reasoning**: (2) is a checkpoint (observable exposure condition), not a date; (3) is the direct falsifier of copy-once-in-disguise; (1) keeps SCAF-1's speed spirit at realistic v1 granularity.
- **Anchor**: 2026-07-19 · t5
- **Status**: live

### D-5 — appetite: program-scale, observable-bounded

- **Question**: what budget is this problem worth?
- **Answer**: program-scale, observable-bounded — full pipeline treatment as **one feature PRD** (a split only if review forces it), sustained operator attention at the review gates, machine work on the order of the contract-gate / impl-review programs (~10–15 tasks). Cut-line: anything not needed to pass D-4's three observables stays parked (upgrade UX, adoption, org sweep, App-creation flows, telemetry). Complexity valve: if converge semantics balloon, cut toward simpler verbatim-ownership file classes — never grow merge cleverness in v1.
- **Reasoning**: the feature touches release machinery, a new engine surface, and dogfood migration — genuinely program-sized; the observables give it a hard edge so it cannot sprawl into the platform loop (§9), which stays future work.
- **Anchor**: 2026-07-19 · t6
- **Status**: live

### D-6 — the v1 decision surface: four asked knobs; hard deps and paths are config, not questions

- **Question**: which questions does the v1 installer actually ask (each also a non-interactive flag, answers persisted as repo config)?
- **Answer**: **four knobs asked**: (1) work tracker github|carina; (2) CI mode local-hooks-only|github-actions|both (defaulted by tracker); (3) GitHub wiring depth — use-or-create repo, labels, branch-protection level none|required-checks|checks+review; (4) agent CLI surfaces auto-detect|explicit set. **Not knobs, by ruling**: default branch is *assumed `main`* for now; **every directory Pyxis needs (doc roots, prds, tech-design, plans, reviews, brainstorms, templates, workitems, …) is captured in the generated config with its default value** — explicit config, never implicit paths. Reserved-not-asked keys: telemetry, update channel, autonomy level.
- **Reasoning**: operator ruling (redirected the six-knob recommendation): hard dependencies don't get opt-outs; branch plurality is not v1's problem; complete path capture keeps config the single source of truth and holds the door open for existing-repo layouts without building path-asking UX now.
- **Anchor**: 2026-07-19 · t7
- **Status**: live

### D-7 — one `.pyxis/`, manifest-marked ownership; `pyxis-config/` not built in v1

- **Question**: what is the engine-owned vs repo-owned split on disk — where does the captured config live?
- **Answer**: **single `.pyxis/` directory, manifest-marked** — one tree holds both the engine payload and the repo-owned config; the release manifest marks each file's ownership class (engine-verbatim, repairable by converge · repo-owned, validated-never-rewritten · generated). `pyxis-config/` is **not** built in v1. Chosen over the recommended two-directory physical split.
- **Reasoning**: operator's call — no new top-level directory; ownership is manifest bookkeeping, and converge reads the manifest anyway. Consequences accepted knowingly: vision §8's `pyxis-config/` remains *future ground* (the vision is undated — deferral, not contradiction), and prd:implementation-verification FR-5's contracted home for the verification manifest needs re-ruling → carried as fog G-1.
- **Anchor**: 2026-07-19 · t8
- **Status**: live

### D-8 — green-with-skeletons seeding + welcome path

- **Question**: how does install satisfy the gates on a fresh repo (F-4), given governing docs are human-authored inputs the installer must never write content for?
- **Answer**: the installer seeds the four governing/system docs as **fill-me skeletons** (template structure + guidance markers, zero invented content), creates the empty-but-valid registries (tech-design index, incidents header, taxonomy dirs; rollups correctly absent pre-first-ship), activates hooks, runs `carina init` / pushes the CI workflow per the knobs. **Invariant: install ends green** — docgate passes on the seeded tree as-is (structure-checked, not content-checked). The **welcome path** (SCAF-5 parity) carries the authorship duty: printed next-steps plus a welcome work item in the chosen tracker — "author your vision + principles, then run /feature-brainstorm". No new docgate states; no blocking on authorship.
- **Reasoning**: keeps AGENTS.md rule 6 intact (the installer authors no governing content), preserves the ≤5-minute one-command flow, and avoids building gate machinery for a transient bootstrap condition.
- **Anchor**: 2026-07-19 · t9
- **Status**: live

### D-9 — the product face: one `pyxis` command

- **Question**: what does the user actually type — given the engine is nine separate binaries today (F-1)?
- **Answer**: a single **`pyxis`** CLI is the product face: the curl-able bootstrap line (the one admissible bash, P-1) fetches the pinned `pyxis` binary and hands off to `pyxis init`; doctor, converge/repair, and (later) upgrade live under the same command. Whether the nine engine binaries merge into it, sit behind it, or become subcommands is a stage-3/4 packaging decision — v1 commits only that the *user surface* is one command.
- **Reasoning**: SCAF-1 parity (one entry point); avoids two product faces; keeps the packaging trade-space open for design without letting it leak into UX.
- **Anchor**: 2026-07-19 · t10
- **Status**: live

### D-10 — v1 surface: offline floor + doctor in; self-converge and uninstall deferred

- **Question**: which of the surface candidates ship in v1 — offline floor, doctor, pyxis-repo self-converge, uninstall?
- **Answer**: **offline floor** (carina + local-CI with zero GitHub/network calls after release fetch — vision §2's "local git repo and a plain task file" as an acceptance case) and **doctor** (`pyxis doctor`: manifest drift via converge dry-run + environment checks — deps, hooks active, tracker reachable, CLI surfaces intact) both ship v1. **Self-converge of the pyxis repo** (instance #0) and **uninstall** are deferred to the waiting room.
- **Reasoning**: the offline floor is the direct test of medium-independence; doctor is converge's natural read-only face and FLEET-5/BENCH-1 parity (F-5: none exists). Self-converge is unification value with dogfood-migration risk that shouldn't gate new-repo installs; uninstall follows FLEET-6's conservative-off later.
- **Anchor**: 2026-07-19 · t11
- **Status**: live

### D-11 — payload: everything pipeline-serving; argus as a fifth, opt-in knob

- **Question**: what is the v1 payload boundary, and where does argus sit in it?
- **Answer**: **everything pipeline-serving ships** — all skills, authoring guides, templates, the gate/executor binaries (docgate, plangate, codegate, carina, implement-now, argus-stamp/writehook, feedback tools), hook shims, and pinned data assets such as Mermaid. Instance-specific data never ships: the pyxis repo's own `docs/` content, `.workitems/`, stamp authority config, grandfather lists, benchmarks. The release manifest is the single authority on in vs out. **Argus is a fifth wizard knob: opt-in, off by default** — strangers get the full pipeline without delegated decision authority; enabling the AI Engineer is an explicit consent moment.
- **Reasoning**: honors the idea's "everything we have built" while keeping delegated authority conservative-by-default (VNEXT-8's spirit: autonomy is climbed into, not defaulted). Extends D-6: the asked-knob set becomes five.
- **Anchor**: 2026-07-19 · t12
- **Status**: live

## Facts

- **F-1**: Of the 9 engine binaries under `cmd/` (docgate, plangate, codegate, carina, implement-now, argus-stamp, argus-writehook, feedback-capture, feedback-dispose), **only docgate has release machinery** — `.github/workflows/docgate-release.yml`: tag `docgate/v*`, darwin/linux × arm64/amd64 matrix, SHA256SUMS. No goreleaser, Makefile, or install script exists. — source: repo scan 2026-07-19
- **F-2**: `.pyxis/scripts/docgate-resolve.sh` is the existing distribution seam: fetch-verify-cache of the pinned binary (version from `.pyxis/config.yaml`, SHA256-verified) with a `go run ./cmd/docgate` source-mode fallback and a hardcoded release URL `https://github.com/agenticlab-ai/pyxis/releases/download/…` (line 27). CI, the pre-push hook, and skills all invoke docgate through it. — source: docgate-resolve.sh; .github/workflows/docs-gate.yml
- **F-3**: Target repos would consume **released binaries, never Go source** — so the ~226 `github.com/agenticlab-ai/pyxis` module-path imports are not portability blockers. The true un-hardcode list is short: pre-push hook `origin/main` (`.pyxis/hooks/pre-push:4`), argus-writehook base env-default, implement-now `--base main` + label `automated-implementation` defaults (`cmd/implement-now/run.go:30,61`), plangate `git show origin/main:` (no override flag), and the ratification trailer const `Governing-Change: operator` (`internal/lane/policy/load.go`). — source: repo grep 2026-07-19
- **F-4**: A fresh empty repo **fails docgate**: the governing-doc checks require `docs/product-vision.md` + `docs/architecture-principles.md`, and `check-maps` requires the `system_maps` files (`docs/domain-model.md`, `docs/system-design.md` per `.pyxis/config.yaml`). The living rollups are validly *absent* pre-first-ship (amendment-017 convention). Install must seed skeletons; governing docs are human-authored inputs (AGENTS.md rule 6), so seeds are templates the human fills, never generated content. — source: internal/docgate checks; .pyxis/config.yaml
- **F-5**: Tracker selection is **flag-only** today (`implement-now --tracker gh|carina`; no `.pyxis/config.yaml` key). `carina init` is the only installer-adjacent verb in any binary; **no doctor verb exists anywhere**. — source: cmd/* argument parsing scan 2026-07-19
- **F-6**: `pyxis-config/` is contracted but absent: vision §8 specifies it as every onboarded repo's extension point; `prd:implementation-verification` FR-5 places the verification manifest in it (glossary entry); AGENTS.md rule 7 defers fixed doc paths to "the pyxis-config surface (issue #4)". On disk, per-repo config is `.pyxis/config.yaml`, inside the same directory that carries engine-owned scripts/templates/skills. — source: product-vision.md §8; glossary; AGENTS.md rule 7
- **F-7**: The cross-CLI skill surface is already solved by symlinks — `.claude/skills`, `.codex/skills`, `.agents/skills`, `.opencode/` → `.pyxis/skills` (AP-25: vendor dirs hold pointers, never content). — source: repo tree; AGENTS.md layout table
- **F-8**: The installable engine surface in `.pyxis/` is ~56 files (config, 3 hook shims, scripts+tests, 12 templates, 11 authoring guides, 12 skills, stamp config, ui machinery) plus vendor trees; hooks activate via a manual one-time `git config core.hooksPath .githooks`. — source: .pyxis/ recursive listing 2026-07-19

## Fog

- **G-1** (non-blocking — blocks: stage-2 synthesis carries it as an open question): FR-5 re-homing — `prd:implementation-verification` FR-5 contracts the verification manifest into `pyxis-config/`, which D-7 rules out of v1. Either a typed amendment re-homes that manifest into `.pyxis/` (repo-owned class), or `pyxis-config/` arrives with a later feature and FR-5 stands for that date. Not this feature's call to make; the PRD must carry the obligation.

## Out of scope

- **O-1**: Org-wide adoption sweeps (SCAF-4 territory) and non-GitHub hosts (GitLab/Bitbucket/Jira) are out of v1 — separate later features; host breadth waits on adapters existing at all. (from D-2)
- **O-2**: Upgrade **UX** — release channels, changelogs, migration notes — is deferred; v1 must only be structurally upgrade-ready (manifest, pins, converge). WR candidate: revisit at the first release that changes installed files. (from D-3)
- **O-3**: Existing-repo adoption is the deferred follow-up, WR with revisit trigger (first real existing repo that wants the factory); v1's converge semantics and namespacing are chosen so it stays structurally cheap. (from D-3)
- **O-4**: GitHub App creation / bot-identity wiring is out of v1 — installed workflows run on the default `GITHUB_TOKEN`; App-manifest-flow creation belongs to the org-installer future. (from D-6)
- **O-5**: Self-converging the pyxis repo onto its own release manifest (instance #0) — WR; revisit once external installs have proven converge. (from D-10)
- **O-6**: Uninstall — WR (FLEET-6 parity, conservative off). (from D-10)
- **O-7**: Telemetry, update channels, and the autonomy-level dial are reserved config keys only; the platform loop (vision §9/§11) is its own future program. (from D-6)

## Candidate glossary terms

- **T-1**: **release manifest** — the versioned, hash-carrying inventory a Pyxis release ships: every installed file with its ownership class; what converge reads and doctor diffs. · avoid: bare *manifest* (collides with GOV-1's primitive manifest and FR-5's verification manifest)
- **T-2**: **converge** — the idempotent reconcile of a repo against its pinned release manifest, acting per ownership class (repair engine-verbatim files, validate-never-rewrite repo-owned ones, regenerate generated ones); install, re-run/repair, and upgrade are all converge. · avoid: *sync* (directionless), *scaffold* (SCAF-1's one-shot-creation sense), *copy*
- **T-3**: **ownership class** — the per-file marking in the release manifest: engine-verbatim · repo-owned · generated. · avoid: *mutability class* (AP-7's document classes)
- **T-4**: **payload** *(candidate)* — the engine-owned artifact set of a release: binaries + the `.pyxis/` tree the manifest enumerates.
- **T-5**: **offline floor** *(candidate)* — the guaranteed-working minimal binding: local git + carina + local hooks, zero network after release fetch (vision §2's "simplest form").

## Parked technical constraints

- **P-1**: The installer itself is **Go** under the language policy; admissible bash is exactly the curl-able bootstrap line (explicitly listed admissible in AGENTS.md) and git-hook shims — any logic beyond locate-and-delegate moves to Go.
- **P-2**: The pinned Mermaid browser asset ships as data and introduces no install-time Node dependency.
- **P-3**: The resolver's source-mode fallback (`go run ./cmd/…`) cannot exist in a target repo (no Go source present): installed mode must be binary-only, with the release URL overridable for forks/enterprise mirrors.
- **P-4**: Release/versioning scheme — one system version pinning binaries + payload together vs per-binary tags (today: `docgate/v*` only, F-1) — is a stage-3 decision; either way the pins are captured in the repo config (D-6).
- **P-5**: Epoch-aware gating — generalizing the grandfather pattern (e.g. `grandfathered_citations`) into an install-epoch baseline so gates can enforce from-install-forward — is the design-now-or-regret-later constraint that keeps O-3 (existing-repo adoption) structurally cheap; conveyed to stages 3–4.
- **P-6**: The F-3 un-hardcode list (pre-push `origin/main`, writehook base default, implement-now label/base defaults, plangate live-ref, ratification trailer) becomes config-read plumbing; branch stays `main`-only in v1 (D-6) but reads from captured config.
- **P-7**: The installer must be **exec-verified end-to-end in CI** — a matrix over tracker × CI-mode on scratch repos asserting the D-4 observables (green-on-install, converge-repair) — never structure-verified only (the green-merge ≠ working-artifact lesson).
- **P-8**: Skill invocation surfaces inside the payload (resolver scripts, `.claude`/`.codex`/`.agents`/`.opencode` symlinks, hook shims) must work identically in a target repo that has no Go toolchain and no pyxis source — audit every skill's invocation line for source-tree assumptions.

## Gate

| Field | Value |
|---|---|
| **Verdict** | ready-for-prd |
| **Date** | 2026-07-19 |
| **Operator** | @mdarabi |
| **Covering source** | — |
| **Coverage check** | pass — all six inputs covered (problem D-1 · segments D-2 · jobs D-3 · goals D-4 · non-goals O-1..7 · appetite D-5), zero blocking fog |
