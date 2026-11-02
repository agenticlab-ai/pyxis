# Config-driven portability — brainstorm record

| Field | Value |
|---|---|
| **In one line** | Make Pyxis's target-facing paths, surfaces, toolchain/stack, and branch/remote fully config-driven (extending `.pyxis/config.yaml`'s closed schema) so the engine honors any repo's layout without collision — phase 1 of existing-repo adoption, provable by self-hosting the pyxis repo on a non-default config. |
| **Relevant-when** | brainstorming or contracting the engine's per-repo parameterization — making paths/surfaces/toolchain/branch config-driven so an arbitrary target repo's layout is honored; the phase-1 predecessor to existing-repo adoption (installer WR-1); checking prior rulings on portability, the `.pyxis/` config surface, path honoring (WR-10), or non-`main` branch support (WR-9) |
| **Idea** | Operator (2026-07-21), verbatim: "my suggestion would be, first, making all the paths and surfaces, toolchain, etc in pyxis config-driven (to avoid collision and honoring target's layout and structure) and then think about the onboarding phase" — the phase-1-first decomposition of adding Pyxis to an existing repository, preceded by "let's audit all the hardcoded values first (check for path, usernames, technologies, etc)". |
| **Operator** | @mdarabi |
| **Date(s)** | 2026-07-21, 2026-07-22 |
| **Status** | gated: ready-for-prd (2026-07-22) — after gating, the record changes only through the introducing PR's review loop, and freezes at merge |
| **Tracking** | — |
| **Feeds** | docs/prds/config-driven-portability/ |
| **Prior records** | brainstorm:installer |
| **Template version** | brainstorm-record/v1 |

## Orientation

| Source | Why loaded |
|---|---|
| `docs/prds/installer/000-base.md` | the merged basis: WR-1 (existing-repo adoption / J4), WR-9 (non-`main` branch), WR-10 (path honoring), Q-4 (install-epoch grandfathering), FR-17/NG-2 (binaries-not-source), FR-19 (preview) |
| `docs/tech-design/installer-v1.md` | ownership-class model (engine-verbatim / repo-owned / generated), §3.5 target-check refusal of non-empty trees, §3.7 config surface, DINV-3 (repo-owned never rewritten) |
| `docs/brainstorms/installer.md` | the direct ancestor: Framing C ("portability forcing function"), D-7 (`.pyxis/` not `pyxis-config/`), D-3 (adopt-existing deferred to J4), the parity slot (SCAF-1/2/5) |
| `docs/product-vision.md` §8 | `pyxis-config/` per-repo onboarding surface (the sanctioned extension point) — vision-level, undated future ground |
| `docs/domain-model.md` | `PyxisConfig` entity (status "Port + extend"), INV-32 (closed schema), INV-27 (key-level merge), INV-40 (skill/hook portability, AP-4), the "binding is user configuration" model |
| `docs/wiki/glossary.md` | verification-manifest term (already contracted to the config surface, materialized under `.pyxis/`), routing-surface, term hygiene |
| 4-part hardcoding audit of `origin/main` (this session) | the empirical inventory driving the Facts below (paths/surfaces · toolchain · identity · git+config-baseline) |

**Overlaps surfaced:**
- **installer Framing C** (`brainstorm:installer` Framings) · "un-hardcode the pyxis-repo assumptions, make every gate honor config paths, prove the offline floor" was offered as a distinct installer framing, then **folded into Framing A (converge) as acceptance criteria**; the actual path/surface honoring was then deferred to WR-10. **This feature extracts that deferred work as its own contract.** Not a duplicate — the honoring was never built.
- **installer WR-9 / WR-10** (`prd:installer#WR-9`, `#WR-10`) · non-`main` branch and non-default path honoring were explicitly captured-but-deferred in v1 (config admits values, code refuses non-defaults fail-closed). **This feature's phase-1 delivers exactly WR-9 + WR-10, generalized to all surfaces.**
- **installer WR-1 / Q-4** (`#WR-1`, `#Q-4`) · existing-repo adoption (converge at a non-empty tree, epoch baselines) and the install-epoch grandfathering shape are the **phase-2** successor — deliberately out of this record's scope.
- **installer D-7** (`brainstorm:installer#D-7`) · one `.pyxis/` (manifest-marked ownership); `pyxis-config/` **not** built — vision §8's directory is "future ground." Settled: "config-driven" here means **extend `.pyxis/config.yaml`'s schema**, not create a directory. Not reopened.
- **domain-model** · `PyxisConfig` ("Port + extend"), INV-32 (closed schema, unknown key = hard error), INV-27 (shared config merged key-level, never byte-replace — the phase-2 collision model), INV-40 (skill/hook portability). The governing models this feature extends.
- **SCAF-2 adopt-existing** (`docs/prds/scaffolding-and-onboarding.md`) · parity corpus row (two-phase permission split, `--update` idempotent, required-checks calibration) — mostly phase-2, noted.
- No prior portability/adoption brainstorm record exists → not covered.

## Framings

*Three distinct theses of what this feature is — differing in what DRIVES the scope. Operator picks or hybridizes.*

- **A — Finish the config schema (mechanical completion).** Thesis: *this is a bounded schema-completion* — lift the five hardcoded clusters (F-2..F-6) into the existing closed schema, defaults preserved, done when the pyxis repo self-hosts on a non-default config. Implications: narrowest scope, pure engine plumbing, no new concepts. Risk: a checklist that misses coherence (toolchain-stack is more than a path key; a gate that reads config but a hook that doesn't leaves a hole). Opens: phase-2 adoption sits directly on top.
- **B — Portability as a coherence property (the vision's portability, proven).** Thesis: *the deliverable is portability itself* — every gate, verb, hook, and workflow honors config, making "no privileged substrate" (AP-1) real; self-hosting on a non-default config is the acceptance harness. This is installer Framing C, finally extracted. Implications: broader than plumbing — a coherence invariant across surfaces, includes generalizing the toolchain-stack model, closed-schema growth stays deliberate. Opens: adoption (phase-2), org-portability (FLEET-9), fork/mirror distribution.
- **C — The adoption enabler (phase-1 of WR-1, scoped by its successor).** Thesis: *this is explicitly the first half of existing-repo adoption* — scope is driven by what a real foreign repo needs to not-collide and be-honored, and the collision residue (AGENTS.md, hooks) is consciously drawn as the phase-2 seam. Implications: the phase-1/phase-2 boundary is a first-class deliverable; forces early ownership-class/merge decisions. Risk: scoping against a not-yet-built phase-2 can over- or under-shoot.

**Recommended:** **B as the thesis, A as the concrete work, C as the scoping discipline** — the coherence property (B) supplies the acceptance criteria that keep the schema-completion (A) from being an incoherent checklist, while the adoption use-case (C) supplies the discipline of lifting *only what an adopter must vary* (the complexity valve), not every internal constant.

**Destination:** **B (portability as a coherence property) as the thesis · A (finish the config schema) as the concrete work · C (adoption enabler) as the scoping discipline.** Operator confirmed the recommendation (2026-07-22): the coherence bar — every target-facing surface honors config, provable by self-hosting the pyxis repo on a non-default config — keeps the schema-completion from becoming an incoherent checklist, while the adoption use-case disciplines scope to only what an adopter must actually vary (the complexity valve), never every internal constant.

## Decisions

### D-1 — segments: engine-is-the-user, adopter-is-the-audience

- **Question**: Who is phase-1 for, given it onboards no one — it only makes the engine parameterizable?
- **Answer**: Primary beneficiary = the pyxis engine's own portability (the self-host proof is the user; phase-1 serves no new external end-user). Design audience = the phase-2 external adopter (installer D-2's solo / 2–5-person team on their own infra), so every added config key is shaped stranger-proof. Anti-segment = installer D-2's (org-wide adoption sweeps, non-GitHub hosts) **plus the onboarding UX itself** (phase-2).
- **Reasoning**: "engine is the user, adopter is the audience" keeps scope honest — lift what an adopter *will* vary (audience-driven), prove it on ourselves (beneficiary-driven), with no pressure to build onboarding UX yet.
- **Anchor**: 2026-07-22 · turn 1 (segments)
- **Status**: live

### D-2 — jobs: one "declare → obey, or refuse by name" job

- **Question**: What job(s) does phase-1 deliver — one, or several?
- **Answer**: ONE primary job — declare a repo's actual paths / branch / remote / toolchain-stack / surfaces in `.pyxis/config.yaml`, and every engine part (gates, verbs, hooks, the installed CI workflow) obeys, no hardcoded "this repo" assumption left; exercised in phase-1 by the maintainer via the self-host config. Config-coherence validation (cross-field: a declared stack must resolve to runnable verbs; a relocated path must stay in-repo) folds **into** this job — the engine obeys or names why it can't (fail-closed). No second user-facing job.
- **Reasoning**: a single "declare → obey (or refuse by name)" job makes the coherence property (Destination) testable as one thing, not a bag of loosely related capabilities; upgrade / onboarding UX / org-sweep / drift-repair belong to other features.
- **Anchor**: 2026-07-22 · turn 2 (jobs)
- **Status**: live

### D-3 — the bet + observable success (two legs, no foreign fixture)

- **Question**: What's the bet, and what observable proves phase-1 done?
- **Answer**: **Bet** — making the engine fully config-driven now (before phase-2 onboarding) turns adoption into a thin flow later and makes every future engine feature portable-by-default (installer Bet-1 applied to internals; AP-1 made real). **Observable — two legs**: (1) *coherence/self-host* — the pyxis repo runs green (all docgate checks + local `gate` + `doctor`) under a `.pyxis/config.yaml` setting non-default values for every liftable surface (relocated doc paths, renamed branch, explicit remote, non-default surface pointers, explicitly-declared stack); (2) *stack* — a hermetic test proving codegate/`gate` executes a non-Go stack's verbs (python or node) entirely from config. **Not** a full foreign fixture repo running the pipeline (that's phase-2).
- **Reasoning**: leg 1 tests the coherence invariant end-to-end for free (our own repo); leg 2 is the non-Go witness for the one cluster self-host structurally can't cover (pyxis is Go). Strong but bounded — no phase-2 pulled forward.
- **Anchor**: 2026-07-22 · turn 3 (bet + observable)
- **Status**: superseded by D-5 — D-4 deferred toolchain from phase-1, so the observable's stack legs (leg-1's declared-stack, leg-2's non-Go proof) no longer apply; the bet and the self-host coherence proof carry into D-5 unchanged

### D-4 — toolchain/stack selection is deferred out of phase-1

- **Question**: How far does config-driven toolchain/stack go in phase-1 — built-in-stacks-selectable-and-overridable, or deeper/shallower?
- **Answer**: Deferred entirely — phase-1 does **not** take on toolchain/stack selection (operator: "we should not worry about the toolchain selection in phase-1"). The existing `executor.gate` override already lets a repo supply its own build/test commands; a `stack`-selection abstraction + generalized non-Go verb defaults wait for a later phase (when a real non-Go adopter exists).
- **Reasoning**: it is the deepest/riskiest cluster, its value is speculative while pyxis is Go (self-host can't witness non-Go anyway), and the `executor.gate` override already covers the immediate need — deferring holds phase-1 tight (the complexity valve).
- **Anchor**: 2026-07-22 · turn 4 (toolchain)
- **Status**: live

### D-5 — observable success: leg-1 self-host only (supersedes D-3)

- **Question**: Given D-4 (toolchain deferred), what is the corrected observable success?
- **Answer**: Bet unchanged (carried from D-3). Observable = **self-host only**: the pyxis repo runs green (all docgate checks + the local `gate` + `doctor`) under a `.pyxis/config.yaml` that sets non-default values for every liftable surface — relocated doc paths, a renamed default branch, an explicit remote, and non-default surface pointers. No stack variation; no non-Go stack test.
- **Reasoning**: D-4 removed toolchain from phase-1, so D-3's stack legs no longer apply; the coherence proof stands on the four remaining liftable clusters, exercised end-to-end on our own repo.
- **Anchor**: 2026-07-22 · turn 5 (observable, post-toolchain-defer)
- **Status**: live

### D-6 — phase-1 scope boundary (the non-goals)

- **Question**: What is out of phase-1 — confirm the out-of-scope set?
- **Answer**: Four boundaries confirmed → O-2 the irreducible single-owner collision residue (AGENTS.md merge, `core.hooksPath`/`.githooks` chaining, `.github/workflows/` collision, CLAUDE.md) → phase-2 onboarding; O-3 Go module path + release host → no work (binaries-not-source, FR-17/NG-2; `artifact_source` overrides the host); O-4 `.pyxis/`-internal paths → out (own namespace, no collision); O-5 onboarding itself — non-empty-tree converge, epoch/grandfathering (Q-4), wizard/UX → phase-2.
- **Reasoning**: O-2 is the deliberate phase-1/phase-2 line (config solves relocation; merge/chaining is a harder, different problem); O-3/O-4 prevent wasted work on non-collisions; O-5 holds the phase boundary.
- **Anchor**: 2026-07-22 · turn 6 (non-goals)
- **Status**: live

### D-7 — problem statement (§2.1): the engine's self-assumption

- **Question**: What is the problem phase-1 solves — pin it as a decision for the PRD's §2.1?
- **Answer**: Pyxis's engine hardcodes "this repo" — its own doc paths, its `main`/`origin`, its `.claude`/`.codex` surface layout — so it can only run where it was developed; any repo with a different layout would collide with or be dishonored by those assumptions, blocking existing-repo adoption (WR-1) at the root. Phase-1 removes that block by making every target-facing surface honor config, proven by running green on a deliberately non-default config of its own.
- **Reasoning**: naming the problem as the engine's *self-assumption* (not "we lack an installer") is what makes the fix config-coherence and self-host a valid proof.
- **Anchor**: 2026-07-22 · turn 7 (problem statement)
- **Status**: live

### D-8 — appetite: a focused single-PRD feature (~5–8 tasks), smaller than the installer

- **Question**: How much do we spend on phase-1?
- **Answer**: A focused single-PRD feature, clearly smaller than the installer program (~10–15 tasks) — on the order of **~5–8 tasks**, with sustained operator attention at the review gates. The work: fill 4 doc-path keys, derive the 2 surface/skeleton maps from config, thread `branch`+`remote` through code/hooks/workflow refs, add cross-field config validation (fail-closed), and build the self-host acceptance harness. Cut-line: anything not needed to pass D-5's self-host observable is parked (O-1, O-2, O-5). Complexity valve: for the non-Go shell hooks + generated workflow, prefer generate-at-converge over a config-reader in the hook — never grow config-interpretation into bash.
- **Reasoning**: "finish + generalize" completes an existing schema, so a program-scale appetite would over-provision; the risk is coherence (no surface left behind), protected by the self-host harness, not by more tasks.
- **Anchor**: 2026-07-22 · turn 8 (appetite)
- **Status**: live

## Facts

- **F-1**: The repo config is already a **closed schema** (unknown keys are hard errors, INV-32) and already parameterizes: 15 `paths:` keys, the `system`/`knobs`/`epoch` blocks, `executor.gate.<command>` (per-repo local-CI commands), `system_maps`, `capability_paths`, grandfathering lists, and template-version pins. — source: `internal/docgate/config/config.go` + `.pyxis/config.yaml` (audit)
- **F-2**: `branch` is captured in config but **honored nowhere** — `main`/`origin` are hardcoded in `config.Defaults()`, `cmd/lane` (176, 259), `cmd/implement-now` (61–62), `heal.go:106`, `workspace.go:30–31`, `plangate:195`, the pre-push hook, and the installed `docs-gate.yml`. This is `prd:installer#WR-9`. — source: audit
- **F-3**: `internal/verify/ecosystem/defaults.go` already carries a **three-stack registry (go / python / node)**: Go ships default verbs (`gofmt`, `go vet`, `go build`, `go test -json`); python/node carry test-detection patterns but **empty verb sets** (they defer to `executor.gate`). The multi-language skeleton exists; only stack-selection + non-Go defaults are missing. — source: audit
- **F-4**: Four doc paths are hardcoded **outside** the `paths:` schema — `docs/brainstorms/`, `docs/problems/`, `docs/reviews/`, `docs/wiki/` (in gate checks + argus boundary), while 15 sibling doc paths are already config keys. — source: audit
- **F-5**: `SurfacePointers` (`.claude`/`.codex` → `.pyxis`) and `SkeletonTargets` (→ `docs/product-vision.md` etc.) are hardcoded literal maps, derivable from the `cli_surfaces` knob + `paths:`. — source: `internal/converge/doctor.go`, `internal/payload/payload.go` (audit)
- **F-6**: The installed CI workflow `docs-gate.yml` bakes `ubuntu-latest`, Go toolchain steps, and `origin/main`. — source: `.pyxis/dist/github-workflows/docs-gate.yml` (audit)
- **F-7**: The Go module path `github.com/agenticlab-ai/pyxis` and the release download base are **not** adopter concerns — adopters consume released binaries, never source (`prd:installer#FR-17`/NG-2), and `artifact_source` already overrides the release host. — source: `prd:installer` + audit
- **F-8**: Four surfaces are **irreducible single-owners** config can't relocate: `AGENTS.md` at repo root (CLI convention), git's single `core.hooksPath`/`.githooks`, `.github/workflows/` (fixed by Actions), `CLAUDE.md`. — source: audit
- **F-9**: The config surface already has an external consumer contracted to it — the **verification manifest** (`prd:implementation-verification#FR-5`), materialized under the system tree (`design:installer-v1 §3.7`). — source: glossary + installer
- **F-10**: All `.pyxis/`-internal paths (`severity-map.yaml`, `context-pack.yaml`, `contract-gate-reports/`, `executor-journal.jsonl`, vendor/dist/bin) live in Pyxis's own namespace and do not collide with a target repo. — source: audit

## Fog

none

## Out of scope

- **O-1**: Toolchain/stack selection — a `stack` knob, generalized non-Go verb defaults, and the ecosystem registry becoming config-fed — is out of phase-1. The `executor.gate` override already lets a repo supply its own commands, and the generalization's value is speculative until a real non-Go adopter (phase-2+). (from D-4)
- **O-2**: The irreducible single-owner collision residue — `AGENTS.md` at repo root (merge with an existing one), git's single `core.hooksPath`/`.githooks` (chain into existing hooks), the `.github/workflows/` path (additive + name-collision), `CLAUDE.md` — is phase-2 onboarding's job; phase-1 leaves these where they are. Config cannot relocate them (F-8); their merge/chaining policy is a different, harder problem. (from D-6)
- **O-3**: The Go module path `github.com/agenticlab-ai/pyxis` and the release download base are no-work — adopters consume released binaries, never source (FR-17/NG-2), and `artifact_source` already overrides the host (F-7). (from D-6)
- **O-4**: `.pyxis/`-internal paths (`severity-map.yaml`, `context-pack.yaml`, `contract-gate-reports/`, `executor-journal.jsonl`, vendor/dist/bin) — out; they live in Pyxis's own namespace and never collide with a target (F-10). (from D-6)
- **O-5**: Onboarding itself — the non-empty-tree converge, the install-epoch/grandfathering shape (installer Q-4), and any wizard/UX — is the phase-2 successor feature, not this one. (from D-6)

## Candidate glossary terms

- **T-1**: target-facing surface — an engine-touched path, ref, or file that lives in the **adopter's** namespace (doc paths, the default branch/remote, root-level convention files like `AGENTS.md`, the `.claude`/`.codex` pointers) and therefore must honor repo config; distinct from `.pyxis/`-internal paths, which live in Pyxis's own namespace and need no config. · avoid: bare "surface" (overloaded — access surfaces, CLI surfaces, routing surface)

## Parked technical constraints

- **P-1**: Config delivery to non-Go surfaces — the shell git-hooks (pre-push) and the generated CI workflow cannot read `.pyxis/config.yaml` at runtime, yet must honor `branch`/`remote` (and any other lifted value). Candidate shape (per D-8's valve): generate/template these at converge time with the values baked in, regenerated when config changes, rather than a config-reader inside the hook. Stages 3–4 resolve the mechanism.

## Gate

| Field | Value |
|---|---|
| **Verdict** | ready-for-prd |
| **Date** | 2026-07-22 |
| **Operator** | @mdarabi |
| **Covering source** | — |
| **Coverage check** | pass — all six inputs covered (§2.1 D-7 · §2.2 D-1 · §2.3 D-2 · §3.1 D-5 · §3.4 O-1…O-5 · appetite D-8), zero blocking fog |
