# Config-driven portability — design-brainstorm record

| Field | Value |
|---|---|
| **In one line** | Design shape for making the engine's target-facing surfaces resolve from repo config — the central question is how the non-Go surfaces (the installed CI workflow + the shell git-hooks) receive config-driven values; the Go-side lifts extend the existing config-read seam. |
| **Relevant-when** | designing or changing how config reaches non-Go surfaces (generated workflow, git-hooks), the ownership class of the installed workflow/hooks, doc-path/branch/remote resolution in the engine, or the surface-pointer derivation; checking prior design rulings on config-driven portability |
| **Basis** | prd:config-driven-portability |
| **Authority** | @mdarabi |
| **Date(s)** | 2026-07-22, 2026-07-23 |
| **Status** | gated: ready-for-design (2026-07-23) — after gating, the record changes only through the introducing PR's review loop, and freezes at merge |
| **Tracking** | — |
| **Feeds** | docs/tech-design/config-driven-portability-v1.md |
| **Prior records** | brainstorm:config-driven-portability |
| **Template version** | design-brainstorm-record/v1 |

## Orientation

| Source | Why loaded |
|---|---|
| `prd:config-driven-portability` (effective.md) | the merged basis — FR-1..FR-6, UB-1..3, ABUSE-1, NR-1..2, the coherence-invariant framing, P-1/§7.2 Q1 |
| `brainstorm:config-driven-portability` | the gated stage-1 record — D-1..D-8, O-1..O-5, F-1..F-10, P-1, the target-facing-surface concept |
| `design:installer-v1` §60 (ownership classes), §3.5 (seed/substitution), §3.10 (installed-workflow variant) | the parent design: the engine-verbatim / repo-owned / generated class model and the substitution mechanism this design extends |
| `design:code-gate-mirror` (via I5) | the single-source precedent — one per-repo list run by one verb mirrors CI, never two hand-lists |
| `design:repo-layout-and-system-footprint` | the system-footprint / removal-test frame (`.pyxis/` tree, the hooks + workflow overlay targets) |
| `docs/architecture-principles.md` | AP-1 (no privileged substrate), AP-11 (one declared source), AP-25 (vendor pointers hold glue, not content), AP-24/AP-27 (authored class-policy → derived manifest) |
| `docs/observability/incidents.md` I5 (2026-07-21) | the `executor.gate` two-list-drift lesson — the binding incident for any CI-mirroring surface |
| `docs/tech-design/index.md` | Serves scan: no live design decides this basis (installer-v1 deferred it to WR-9/WR-10) |

**Overlaps surfaced:**
- **No live design covers this basis** — the index shows installer-v1 (and its siblings) Serve the installer PRD and deliberately deferred non-`main` branch (WR-9) and path honoring (WR-10); none decides the config-driven-portability questions. Not `covered`.
- **installer-v1 supplies the frame, not the answer** — its ownership-class model (engine-verbatim / repo-owned / generated, §60) and its seed-time substitution (§3.5) are the mechanisms this design reuses; it did not decide how a *config change* re-reaches the non-Go surfaces (its manual is substitute-once-then-repo-owned, which is wrong for a value that must track config).
- **code-gate-mirror / I5** — the single-source principle (AP-11) binds any surface that mirrors CI; this design must not create a second hand-list of branch/steps beside the config.

## Design questions

- **DQ-1**: How do the **non-Go surfaces** — the installed CI workflow and the shell git-hooks — receive config-driven values (branch/remote, and any path they reference), given neither can read the repo config the way a Go consumer can (a workflow's `on: push: branches:` trigger is a static literal Actions cannot compute; a shell hook has no config parser)?
- **DQ-2**: How do the Go-side consumers resolve the newly-lifted values — the four doc paths added to the closed `paths:` schema, and `branch`/`remote` — a single "target-facing resolver" port, or each consumer reading the config struct it already holds?
- **DQ-3**: How are the derived surface maps (the CLI-surface pointers and the governance-skeleton targets) computed — a pure function of the CLI-surface knob + the path schema, or cached/materialized state?
- **DQ-4**: Where does cross-field validation live (FR-5 — a relocated path stays in-repo; a referenced surface exists), and is the in-repo check lexical or symlink-resolved (the reviewer's TOCTOU concern)?
- **DQ-5**: How is the self-host acceptance harness expressed (M1/DOD-1) — a committed non-default config profile + a CI job that runs the engine against it?

## Panel log

- **DQ-1**: opens/replaces (a new config-delivery mechanism + an ownership-class shift for the workflow/hooks) — **fired** — the surfaces cannot read config directly, so the delivery mechanism is a genuine new seam with structurally distinct shapes (ALT-1/2/3).
- **DQ-2**: extends (the existing config-struct read seam — new fields on an existing surface) — **skipped** — adding keys + reading them is not a new seam.
- **DQ-3**: extends (config → derived value, a pure function) — **skipped**.
- **DQ-4**: extends (the existing closed-schema parser's fail-closed refusal path) — **skipped**.
- **DQ-5**: extends (a test fixture + a CI job over the existing gate/doctor) — **skipped**.

## Decisions

### D-1 — DQ-1: non-Go config delivery — split (workflow generated, hooks runtime-resolve)

- **Question**: How do the non-Go surfaces (the remote CI workflow + the local git-hooks) receive config-driven branch/remote, given neither reads config like a Go consumer?
- **Answer**: ALT-2 (split). The remote GitHub Actions workflow becomes **generated** — converge renders it from config (its `on: push: branches:` trigger is a static literal Actions cannot compute). The local git-hooks stay **engine-verbatim**, byte-identical everywhere, and drop their branch/remote literals: the pre-push hook invokes `pyxis exec …` without a baked merge-base and pyxis (Go, reads config) resolves branch/remote at runtime. Each surface uses the mechanism its constraint admits.
- **Reasoning**: most honest to each surface's real constraint (the workflow trigger is static → must generate; the hook already delegates to pyxis → runtime-resolve); minimizes generated files to only the workflow (which must be generated anyway); keeps hooks byte-identical (zero hook-drift surface); lands on I5's single-source remedy — the hook's branch is the one pyxis-read config, never a second list.
- **Anchor**: 2026-07-23 · turn 1 (DQ-1 panel)
- **Status**: live

### D-2 — DQ-2: Go-side resolution — extend the config struct, no new resolver port

- **Question**: How do Go consumers resolve the lifted values (the four doc paths + branch/remote) — a dedicated resolver port, or the `config.Config` they already hold?
- **Answer**: Extend the existing config-struct reads. Add the four doc-path keys to the closed `paths:` schema; add `remote` as a top-level scalar beside the existing `branch`; each consumer reads the field from the `config.Config` it already receives, and the few that hardcode without holding a Config get it threaded in (plumbing). No "target-facing resolver" port — the config struct + `Defaults()` is the single resolution surface (AP-11).
- **Reasoning**: the config struct is already the pattern for the ~15 existing path keys, so adding fields + reading them is the smallest coherent extension; a resolver port adds a layer with nothing cross-cutting to justify it; the coherence invariant (DOD-4) is met by consumers reading config fields, not literals.
- **Anchor**: 2026-07-23 · turn 2 (DQ-2)
- **Status**: live

### D-3 — DQ-3: derived surface maps — pure function, no cached state

- **Question**: How are the derived maps (CLI-surface pointers, governance-skeleton targets) computed — a pure function of the CLI-surface knob + path config, or materialized state?
- **Answer**: Pure function, no cached/materialized state. Both maps compute their entries at call time from the CLI-surface knob + the relevant path keys; deterministic, recomputed per use (converge/doctor/seed — infrequent); no memoization, no derived-state file to invalidate on config change.
- **Reasoning**: tiny derivations; a pure function keeps config the single source with nothing to sync; a cache of derived pointers would be a second thing that can drift from config (I5 in miniature). Exact key set is the design's detail; the shape is pure-derivation-no-state.
- **Anchor**: 2026-07-23 · turn 3 (DQ-3)
- **Status**: live

### D-4 — DQ-4: cross-field validation — fold into load, lexical containment, no symlink resolution

- **Question**: Where does FR-5 cross-field validation live, and is the in-repo check lexical or symlink-resolved?
- **Answer**: Fold the FR-5 checks into config load, fail-closed before any resolution, reusing the existing closed-schema refusal path (INV-32). The in-repo check is **lexical**: refuse a config path that is absolute, contains `..`, or isn't repo-relative; the engine only joins config paths under root, never follows a config value as absolute. **No symlink resolution** — a symlink inside the repo is the owner's own content, and the config author and repo author are the same principal (no trust boundary); resolving symlinks would be non-hermetic and outside this threat model.
- **Reasoning**: folding into load reuses the fail-closed parser (INV-32); lexical containment is deterministic + hermetic (no TOCTOU window — the check is on the config string at parse and the engine never treats a config path as absolute); symlink resolution conflates config validity with repo filesystem policy and has no privilege boundary to protect here.
- **Anchor**: 2026-07-23 · turn 4 (DQ-4)
- **Status**: live

### D-5 — DQ-5: acceptance harness — self-host per M1 + coherence inspection; existing-repo e2e → phase-2

- **Question**: How is the acceptance harness expressed, and does the phase-1 proof include an existing-repo install e2e?
- **Answer**: Option C. Phase-1's proof stays the PRD's M1/DOD-1 **self-host** — the pyxis repo run green (full gate + local `gate` + `doctor`) under a non-default config (its corpus relocated in a throwaway checkout) — plus the DOD-4 coherence inspection (zero target-facing literals) and hermetic per-consumer unit tests (DQ-2/3/4). The **existing-(non-empty)-repo install e2e is deferred to phase-2 onboarding** — it needs the non-empty converge (installer UB-1 refuses non-empty; O-5/WR-3), which phase-1 does not build. M1 stays as written — no PRD amendment.
- **Reasoning**: an existing-non-empty-repo install e2e cannot be built in phase-1 (the onboarding path is deferred); the self-host proof + coherence inspection already give a strong phase-1 coherence guarantee without pulling phase-2 forward.
- **Anchor**: 2026-07-23 · turn 5 (DQ-5)
- **Status**: live

### D-6 — design scope boundaries (the out-of-scope set)

- **Question**: What does this design not cover — confirm the boundary set?
- **Answer**: Not designed here (phase-2): toolchain/stack selection (PRD WR-1); the irreducible collision-residue merge/chaining — AGENTS.md merge, `core.hooksPath` chaining (PRD WR-2); onboarding as a whole — the non-empty-tree converge, the install-epoch/grandfathering, and the existing-repo install e2e (PRD WR-3 + D-5). No work: the Go module path + release host (PRD NG-2); the `.pyxis/`-internal paths (PRD NG-1). The design touches only the phase-1 clusters (config-schema extension, Go-side resolution, generated workflow + runtime-resolving hooks, cross-field validation, self-host proof).
- **Reasoning**: mirrors the merged PRD's NG/WR; the design implements phase-1's coherence property, deferring the harder, separate problems (merge, onboarding, toolchain) to their phases.
- **Anchor**: 2026-07-23 · turn 6 (scope boundaries)
- **Status**: live

## Alternatives

### ALT-1 — Uniform generated (both surfaces rendered at converge)

- **Shape**: The installed CI workflow and the operand git-hooks both become **generated**-class files. Converge renders each from a shipped template, substituting branch/remote (and any referenced path) from the repo config; a config change re-renders them on the next converge. Reuses the seed-time substitution mechanism (installer-v1 §3.5) and the generated class (converge regenerates; doctor reports drift — installer A-4 / this PRD's UB-3).
- **Trade-offs**: wins — one uniform mechanism for every non-Go surface, one mental model; the generated class + substitution + doctor-drift all already exist. costs — the hooks lose their byte-identical-everywhere property (each repo carries a rendered hook variant), adding a per-repo generated surface where none existed.
- **Principles**: AP-1 honored (surfaces honor config); AP-11 honored (both render from the one config — no second list); reuses AP-24/AP-27 (class-policy → derived manifest).
- **Incidents**: I5 matched (executor.gate two-list drift) — averted: both surfaces render from the ONE config, never a second hand-list.
- **Verdict**: rejected — the runner-up (one uniform mechanism), but it makes the hooks per-repo rendered files (a new hook-drift surface) where ALT-2 keeps them byte-identical; the hook's runtime pyxis-delegation is cheaper than rendering it (D-1).
- **From**: DQ-1 · panel.

### ALT-2 — Split: workflow generated, hooks runtime-resolve

- **Shape**: The workflow becomes **generated** — its `on: push: branches:` trigger is a static literal Actions cannot compute, so converge renders it from config. The git-hooks stay **engine-verbatim** (byte-identical across every repo) and drop their branch/remote literals: the pre-push hook invokes `pyxis exec …` without a baked `--merge-base origin/main`, and pyxis (Go, reads config) resolves the branch/remote at runtime. Each surface uses the mechanism its constraint admits.
- **Trade-offs**: wins — the hook stays universal (no per-repo render, zero hook-drift surface); only the workflow (which *must* be generated anyway) is a rendered file; runtime resolution is a tiny extension since hooks already invoke pyxis. costs — two delivery mechanisms (generated vs runtime), a larger concept surface; the workflow's trigger (generated) and run-step (delegates to pyxis) are handled differently.
- **Principles**: AP-1 honored; AP-11 honored (config is the single source — the workflow renders from it, the hook reads it live via pyxis); AP-25 honored (the hook stays thin glue, delegating to pyxis).
- **Incidents**: I5 matched — averted, and directly aligned with its remedy ("single-source both surfaces from one per-repo list run by one verb"): the hook's branch is the one pyxis-read config, never a second list.
- **Verdict**: **chosen** (D-1) — most honest to each surface's constraint; generated files bounded to the workflow; hooks stay byte-identical; on I5's single-source remedy.
- **From**: DQ-1 · panel.

### ALT-3 — Uniform runtime shim (both delegate at runtime)

- **Shape**: Both surfaces stay engine-verbatim and delegate every config-dependent value to pyxis at runtime. The hook resolves as in ALT-2; the workflow's run-step likewise — but its `on: push: branches:` trigger is evaluated by Actions from the file with zero computation, so a "runtime" trigger is impossible: the workflow must either trigger on ALL pushes and let pyxis decide, or bake a literal.
- **Trade-offs**: wins — maximal uniformity, zero generated files. costs — structurally blocked for the workflow: an all-pushes trigger runs the gate on every branch (wrong semantics, wasted CI), and a baked literal reintroduces the hardcoding this feature removes.
- **Principles**: AP-1 honored in intent; the all-pushes fallback strains AP-14 (deployment-context fidelity) and wastes CI.
- **Incidents**: none additional checked (incidents.md, 2026-07-22) beyond I5, which it also averts.
- **Verdict**: rejected — infeasible: the workflow-trigger constraint makes uniform-runtime impossible; retained as the panel's divergence floor.
- **From**: DQ-1 · panel.

## Seam map

- **S-1**: the non-Go config-delivery seam (the ownership class + render/resolve mechanism of the installed workflow and the git-hooks) — **opens/replaces** — behind it: whichever of ALT-1/2/3 the authority picks — feeds: the design's §2 Decision + Scope (the converge render/generate surface).
- **S-2**: the closed config schema — **extends** — behind it: the four new `paths:` keys + `remote`, added as deliberate closed-schema keys (INV-32) — feeds: Scope (config schema).
- **S-3**: the config-load validation path — **extends** — behind it: the FR-5 cross-field checks folded into the existing fail-closed parser — feeds: Scope (config load).

## Facts

- **F-1**: The engine already carries three ownership classes — engine-verbatim (converge repairs byte-exact) · repo-owned (validated, never rewritten) · generated (converge regenerates) — assigned by an authored class-policy, manifest-derived. — source: `design:installer-v1` §60 (AP-24/AP-27)
- **F-2**: A GitHub Actions `on: push: branches:` trigger is a static literal evaluated from the workflow file; Actions performs no computation or config read to determine it. — source: GitHub Actions workflow-trigger semantics
- **F-3**: The installed git-hooks already invoke `pyxis exec …` (e.g. `pyxis exec docgate -- pre-push --merge-base origin/main`), so a hook can delegate branch/remote resolution to pyxis without gaining a config parser. — source: the operand-hooks payload (`.pyxis/dist/operand-hooks/*`)
- **F-4**: Converge already renders a shipped template with config substitution at seed time (the operand manual, substitute-once-then-repo-owned) — the substitution engine exists; only the *re-render-on-config-change* posture (the generated class) differs. — source: `design:installer-v1` §3.5
- **F-5**: I5 (2026-07-21): a local gate hand-listed as a subset of required CI drifted from it; the lesson binds any CI-mirroring surface to a single source (AP-11) — "single-source both surfaces from one per-repo list run by one verb." — source: `docs/observability/incidents.md` I5
- **F-6**: code-gate-mirror is the standing precedent for single-sourcing a local gate and CI from one per-repo list (the docgate-resolve.sh pattern applied to code). — source: `design:code-gate-mirror` (I5 Links)

## Fog

none

## Out of scope

- **O-1**: Toolchain/stack selection (a `stack` knob, non-Go verb defaults) — phase-2; `executor.gate` override covers the interim (PRD WR-1). (from D-6)
- **O-2**: The irreducible collision-residue merge/chaining — AGENTS.md merge, `core.hooksPath` chaining, `.github/workflows/` name-collision, CLAUDE.md — phase-2 onboarding (PRD WR-2). (from D-6)
- **O-3**: Onboarding as a whole — the non-empty-tree converge, the install-epoch/grandfathering (installer Q-4), and the existing-(non-empty)-repo install e2e — phase-2 (PRD WR-3). (from D-6, D-5)
- **O-4**: The Go module path + release host — no work; adopters consume released binaries, `artifact_source` overrides the host (PRD NG-2). (from D-6)
- **O-5**: The `.pyxis/`-internal paths — no work; they live in Pyxis's own namespace and never collide with a target (PRD NG-1). (from D-6)

## Candidate glossary terms

none *(the target-facing-surface term landed with the PRD)*

## Parked product gaps

none *(the basis is a merged, prd-review-clean contract)*

## Gate

| Field | Value |
|---|---|
| **Verdict** | ready-for-design |
| **Date** | 2026-07-23 |
| **Authority** | @mdarabi |
| **Covering design** | — |
| **Coverage check** | pass — core four covered (§2 Decision D-1 · §4 Alternatives ALT-1/2/3 · Serves/Scope S-1/S-2/S-3 + Basis · §3.2 Boundaries O-1…O-5), zero blocking fog |
| **Amend route** | — |
