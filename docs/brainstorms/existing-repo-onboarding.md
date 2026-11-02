# Existing-repo onboarding — brainstorm record

| Field | Value |
|---|---|
| **In one line** | Bringing a repository that already has its own source, conventions, hooks, CI, and doc corpus under factory management — converge onto a non-empty tree, the install-epoch baseline, and the single-owner collision residue. |
| **Relevant-when** | brainstorming or contracting existing-repo adoption (installer WR-1 / SCAF-2); checking prior rulings on non-empty-tree converge, the install-epoch or grandfathering baseline (installer Q-4), the single-owner collision residue (`AGENTS.md`, `core.hooksPath`, `.github/workflows/`, `CLAUDE.md`), toolchain/stack profiles (SCAF-3), or what pyxis promises never to mutate in an adopter's repo |
| **Idea** | "we should think about adding pyxis to an existing repository … with own source code and all of this stuff" → decomposed with the operator into phase-1 (config-driven portability, contracted and in build) and **phase-2, this feature: the onboarding flow itself**. |
| **Operator** | @mdarabi |
| **Date(s)** | 2026-07-23 |
| **Status** | gated: ready-for-prd *(→ freezes at the introducing PR's merge; until then operator review may add or supersede review-anchored decisions)* |
| **Tracking** | — |
| **Feeds** | `docs/prds/existing-repo-onboarding/` |
| **Prior records** | `brainstorm:installer`, `brainstorm:config-driven-portability` |
| **Template version** | brainstorm-record/v1 |

## Orientation

| Source | Why loaded |
|---|---|
| `docs/prds/installer/effective.md` | the merged basis — WR-1 (existing-repo adoption) is this feature's charter; NG-1/NG-2/NG-3 bound it; Q-4 (install-epoch) is the inherited open question |
| `docs/brainstorms/config-driven-portability.md` | the phase-1 record — its O-1/O-2/O-5 explicitly assign this work to phase-2 |
| `docs/prds/config-driven-portability/effective.md` | phase-1's contract: every target-facing surface now resolves from repo config (the precondition this feature builds on) |
| `docs/prds/scaffolding-and-onboarding.md` | the baseline requirement source — SCAF-2 (repo bootstrap / adopt an existing repo), SCAF-3 (language profiles), SCAF-5 (welcome path) |
| `docs/prds/README.md` | requirements index — located the SCAF area |
| `docs/product-vision.md` §8 | governing ground: per-repo onboarding + the `pyxis-config/` extension point + repo intelligence |
| `docs/wiki/glossary.md` | term check — `onboarding`, `target-facing surface` are already defined; disambiguation from `welcome path` |
| `docs/brainstorms/` (index scan) | overlap check — no prior onboarding/adoption record exists |

**Overlaps surfaced:**

- `prd:installer#WR-1` · "Existing-repo adoption (converge pointed at a non-empty tree, epoch baselines, path honoring)", triggered by "the first real existing repo that wants the factory". **This feature is that designated successor** — not a duplicate; the work was never built.
- `prd:installer#NG-3` · **"Merge intelligence in converge — three-way merges or conflict resolution inside repo-owned files" is a declared NON-GOAL** (the D-5 complexity valve: ownership classes stay simple; repo-owned content is never rewritten). This binds every collision-residue framing: whatever resolves an existing `AGENTS.md`, it is not converge performing a merge.
- `prd:installer#Q-4` · the install-epoch baseline shape (generalized grandfathering) is an **open, design-owned** question, deadline "design gate". This feature inherits it.
- `prd:installer#NG-1` · the installer writes governance **skeletons only, never substance** — generated governance is counterfeit.
- `prd:installer#NG-2` · binaries are the contract; no source distribution or clone-and-build install path.
- `brainstorm:config-driven-portability#O-2` · the irreducible single-owner collision residue (`AGENTS.md` merge, `core.hooksPath`/`.githooks` chaining, `.github/workflows/` collision, `CLAUDE.md`) → "phase-2 onboarding's job". Config cannot relocate them; their merge/chaining policy is "a different, harder problem".
- `brainstorm:config-driven-portability#O-5` · onboarding itself — non-empty-tree converge, the epoch/grandfathering shape, any wizard/UX → "the phase-2 successor feature".
- `brainstorm:config-driven-portability#O-1` · toolchain/stack selection deferred "until a real non-Go adopter (phase-2+)" — **eligible here, not mandated**; `executor.gate` already lets a repo supply its own commands.
- `prd:installer#WR-3`, `#WR-4`, `#WR-5` · adjacent but distinct: self-converging the pyxis repo (instance #0), uninstall, and the org-wide adoption sweep (SCAF-4) are separate deferred items.
- `docs/wiki/glossary.md` · **`onboarding` is already a defined term** — *repo* onboarding, bringing a repository under factory management; explicitly **not** the human-facing learn-the-ecosystem path, which is the **welcome path** (SCAF-5, `prd:ecosystem-help`). This record uses the settled sense.

## Framings

- **A — Converge onto a non-empty tree (the mechanical extension).** Thesis: onboarding is not a new subsystem — it is making the *existing* install path (`init` → `converge` → `doctor`) correct and safe when the tree already has content. The deliverable is converge's non-empty-tree semantics plus the install-epoch baseline that stops a pre-existing corpus from failing every gate. Implications: no wizard, no new verb, maximal reuse of the ownership-class machinery; the phase-1/phase-2 boundary stays clean. Opens: the cheapest credible path to a real adopter. Risk: leaves the human question unanswered — an adopter still must understand converge to know what will happen to their repo.
- **B — The adoption negotiation (inspect → propose → apply).** Thesis: onboarding is fundamentally a *negotiation* with a repo that already has opinions. The factory first reads the repo (stack, hooks, CI, conventions, an existing `AGENTS.md`), then proposes a concrete, reviewable plan — "here is exactly what I add, what I chain into, what I will never touch" — and applies only on approval. Implications: makes "no surprise mutation" a provable property; every collision becomes an explicit line item with a chosen resolution rather than an implicit behavior; generalizes SCAF-2's two-phase permission split and its non-admin hand-off. Opens: reversibility and partial adoption become natural. Risk: larger — introduces an artifact class and probably a verb.
- **C — Adoption is a profile problem (stack-first).** Thesis: what makes a repo adoptable is a *profile* — SCAF-3's data-not-code stack descriptor supplying toolchain verbs, the CI suite, and templates. Onboarding = detect or pick the profile, render it, and calibrate required checks to what the repo actually runs (SCAF-2). Implications: puts the deferred toolchain half (O-1) at the centre and directly serves the first non-Go adopter. Opens: language profiles as a durable extension point. Risk: phase-1 D-4 deferred exactly this as speculative until a real non-Go adopter exists — centring it now may over-build against an imagined user.

**Destination:** **B — the adoption negotiation.** Onboarding is a negotiation with a repo that already has opinions: inspect → propose a reviewable plan → apply only on approval. Operator's rationale: `prd:installer#NG-3` forbids converge merging repo-owned files, so *something* must decide per collision — and an explicit, approved proposal is the only honest place for that decision to live. Framing A's mechanics (non-empty-tree converge semantics + the epoch baseline) are the engine underneath, not a competing thesis.

## Decisions

### D-1 — What is existing-repo onboarding, as a feature?

- **Question**: Three framings offered — A (converge onto a non-empty tree: mechanical extension, no new verb), B (the adoption negotiation: inspect → propose → apply on approval), C (a profile problem: stack descriptors first). Which is the feature?
- **Answer**: **B — the adoption negotiation.** The factory reads the repo, proposes a concrete reviewable plan naming what it adds, what it chains into, what collides and how that resolves, and what it will never touch; it applies only on approval, idempotently.
- **Reasoning**: `prd:installer#NG-3` forecloses merge intelligence inside converge, so collision resolution cannot be an implicit converge behavior — it must be an explicit decision surfaced for approval. This makes "no surprise mutation" a provable property rather than a promise, and generalizes SCAF-2's two-phase permission split. A's mechanics remain required as the engine beneath B; C is deferred-speculative per phase-1 D-4.
- **Anchor**: 2026-07-23 · turn 1
- **Status**: live

### D-2 — Primary adopter segment

- **Question**: Who is this contract written for — the operator's own existing repos (real fixture, Go stack, every collision exercised), a generic mid-size repo archetype, or any repo including large monorepos?
- **Answer**: **A generic mid-size repo archetype** — a repo of moderate size with conventional CI and some existing conventions, not fitted to the operator's own setup.
- **Reasoning**: Operator's ruling, chosen over the recommendation (design against a real owned repo). Avoids over-fitting the contract to one organisation's layout; keeps the adoption path honest for an arbitrary adopter. Accepted consequence: the acceptance proof is a constructed fixture repo rather than a real adoption, and the archetype is not assumed to be Go — which makes the toolchain boundary (`brainstorm:config-driven-portability#O-1`) a live question rather than a free one.
- **Anchor**: 2026-07-23 · turn 2
- **Status**: live

### D-3 — How far the contract goes on toolchain/stack

- **Question**: With a language-agnostic archetype, does this contract ship SCAF-3 language profiles, detect-and-surface only, or stay Go-only and refuse others?
- **Answer**: **Detect and surface only.** The adoption plan detects the repo's stack and names it as an explicit line item stating what pyxis will and will not run; it ships **no language profiles** and invents **no commands**. The adopter supplies build/test/lint through the existing `executor.gate.<command>` override, named as a required action in the plan.
- **Reasoning**: `executor.gate` already makes a non-Go repo functional, so profiles are an ergonomics layer rather than a functional gate; shipping SCAF-3's full profile (pinned toolchain + CI suite + release automation + CODEOWNERS + templates + labels) would roughly double the contract and would build against an adopter not yet met — the same reasoning phase-1 D-4 used to defer it. Framing B's honesty requirement is satisfied by naming the detected stack and the required action, with nothing silently assumed.
- **Anchor**: 2026-07-23 · turn 3
- **Status**: live

### D-4 — Admissible resolutions for the single-owner collision residue

- **Question**: When pyxis wants a path the adopter already owns (`AGENTS.md`, `core.hooksPath`/`.githooks`, `.github/workflows/`, `CLAUDE.md`), what resolutions are admissible — additive-adjacent-else-instruct, never-touch-anything, or adopt-and-own-with-backup?
- **Answer**: **Additive-adjacent where a genuine mechanism exists; instruct the human where it does not.** Hooks resolve by delegation (pyxis's hook calls the repo's pre-existing hook, or installs into the hooksPath the repo already uses) and the workflow lands under a distinct, non-colliding filename — both turnkey, neither touching an existing file. `AGENTS.md`/`CLAUDE.md` content has no honest mechanical merge, so pyxis writes nothing there and the plan prints the exact lines for the human to add. Pyxis never takes over a file the repo owns.
- **Reasoning**: `prd:installer#NG-3` forbids merge intelligence in converge, so "turnkey everywhere" is only reachable by mutating repo-owned content — which would also break framing B's no-surprise-mutation property. Chaining and distinct filenames are additive mechanisms that need no merge, so they are honest wins; where no such mechanism exists, the honest answer is an instruction, not an invented merge. Rejected adopt-and-own-with-backup: a preserved backup does not make the mutation unsurprising.
- **Anchor**: 2026-07-23 · turn 4
- **Status**: live

### D-5 — The install-epoch baseline shape (inherited `prd:installer#Q-4`)

- **Question**: How does an adopting repo's pre-existing corpus avoid failing every gate — an epoch baseline marker, enumerated per-check grandfather lists, or no grandfathering at all?
- **Answer**: **An epoch baseline marker.** Adoption records the adoption commit in the repo config; gates judge only content that changed after it, and everything present at adoption is grandfathered wholesale. Today's enumerated per-check lists (e.g. `grandfathered_citations`) remain available afterwards for targeted exceptions.
- **Reasoning**: Cost is constant whatever the corpus size, so a ten-file and a ten-thousand-file repo adopt identically; the repo is green on day one and gets stricter only as it changes. It is honest — it never claims the old corpus conforms, only that it is not yet judged. Enumerated lists were rejected as potentially thousands of rows plus a slow adoption scan; no-grandfathering was rejected because it would turn away the entire segment this feature exists to serve.
- **Anchor**: 2026-07-23 · turn 5
- **Status**: live

### D-6 — The negotiation's verb and artifact surface

- **Question**: Does adoption get its own verb and a durable plan artifact, become a mode of converge with terminal output, or use a new verb with an ephemeral plan?
- **Answer**: **A new verb plus a durable plan artifact.** The verb inspects the repo and writes a reviewable adoption plan file; a second, explicit step applies exactly that plan. The artifact persists as the record of what was agreed.
- **Reasoning**: Adoption is a one-time negotiation while converge is steady-state repair — one verb carrying both jobs would be a word doing two jobs. D-4 commits the plan to printing exact instructions the human must perform by hand; terminal output scrolls away, whereas a file persists, stays reviewable before anything is written, and gives later upgrade and uninstall a record to consult. Accepted cost: a new verb and a new artifact class.
- **Anchor**: 2026-07-23 · turn 6
- **Status**: live

### D-7 — The bet's observable success (and the owed existing-repo e2e)

- **Question**: What observable proves this works — a fixture-repo end-to-end adoption with an untouched-set proof, per-collision integration tests only, or adopting a real repository?
- **Answer**: **A fixture repo carrying every collision at once** — an existing `AGENTS.md`, a hook-manager `core.hooksPath`, a name-colliding workflow, a non-conforming doc corpus, and a non-Go stack — **adopted end to end (plan → apply)**. Success is: the repo is gate-green afterwards, the emitted manual instructions are the only human work, re-apply is idempotent, and the declared never-touch set is asserted **byte-identical before and after**.
- **Reasoning**: The byte-identity assertion is the falsifiable core — framing B's whole promise is no-surprise-mutation, and an unchecked promise is only a claim; this turns it into a test that fails if violated. A composed fixture exercises the interaction between resolutions, which is where adoption is most likely to break, unlike isolated per-collision tests. A real-repo proof was declined as non-hermetic, un-rerunnable in CI, and in tension with the generic-archetype segment (D-2). This discharges the existing-repo e2e that phase-1 deferred here.
- **Anchor**: 2026-07-23 · turn 7
- **Status**: live

### D-8 — The scope boundary: uninstall is in, five deferrals confirmed

- **Question**: Confirm the six-item out-of-scope set, or pull something into this contract?
- **Answer**: **Uninstall / reversibility is IN scope.** Adoption is reversible: the repo can be restored to its pre-adoption state. The other five stay out — org-wide adoption sweep, branch protection and bot-identity wiring, repo intelligence, upgrade of an already-adopted repo, and partial (subset) adoption.
- **Reasoning**: Operator's ruling. Reversibility is the trust lever that makes adoption safe to say yes to — "try it and back out" materially lowers the cost of the decision — and the plan artifact (D-6) already records exactly what was added and chained, so it is the natural undo input rather than new bookkeeping; a removal contract already exists as prior art (F-8). Accepted consequences: this contract now delivers `prd:installer#WR-4`, its appetite grows, and the D-7 byte-identity observable extends to a **round-trip** proof (adopt → undo → byte-identical to the pre-adoption tree). The other five each retain their own deferred home and trigger.
- **Anchor**: 2026-07-23 · turn 8
- **Status**: live

### D-9 — Appetite

- **Question**: Does roughly 10–14 tasks as a single contract match the appetite, or should it be tighter?
- **Answer**: **Confirmed — one contract, ~10–14 tasks**, with the cut-line that anything not needed to pass the D-7 round-trip observable is parked. Uninstall stays inside this contract rather than splitting into its own.
- **Reasoning**: Larger than phase-1's ~5–8 ("finish + generalize an existing schema") because this adds a verb, an artifact class, three collision resolutions, the epoch baseline, and undo; still below the installer program's ~10–15. Splitting uninstall out would land adoption without the back-out path that motivated including it (D-8), defeating the trust property that justified the scope.
- **Anchor**: 2026-07-23 · turn 9
- **Status**: live

### D-10 — Anti-segment (who this is not for)

- **Question**: D-2 named the primary segment (a generic mid-size repo archetype); who is explicitly *not* served by this contract?
- **Answer**: Two anti-segments. **(a) The greenfield/empty repo** — already served by `pyxis init` (installer v1); pointing `adopt` at an empty tree is a negotiation with nothing to negotiate, so it routes to init rather than running. **(b) The large multi-language monorepo with a bespoke CI empire** — declined at D-2; its collision surface and epoch scale are a materially different problem, and admitting it would silently reintroduce the scope ruled out there.
- **Reasoning**: Operator confirmed. The empty-repo boundary keeps `adopt` and `init` from overlapping (one word, one job); the monorepo boundary holds the D-2 segment line so the contract is not quietly stretched to a size D-9's appetite excludes.
- **Anchor**: 2026-07-23 · turn 10
- **Status**: live

### D-11 — Undo semantics on a diverged tree (review-born, refines D-8)

- **Question**: prd-review FND-1 surfaced that D-8's "reversible / restore to pre-adoption state" is undefined once the repo has diverged since adoption (post-adoption commits, or a pyxis-added artifact the operator edited). What does undo do then?
- **Answer**: **Remove the installation and preserve post-adoption work** — unchain hooks to restore the prior path, delete the paths adoption added, clear the epoch — and **fail closed on operator edits**: any pyxis-added artifact the operator modified is preserved-and-named, never deleted. Not revert-to-snapshot (which would clobber a week of the operator's own work), and not blanket-refuse-on-any-divergence (which would make the back-out lever rarely usable).
- **Reasoning**: the only option consistent with the feature's own no-surprise-mutation + reversibility thesis; a preserved-work removal is the honest meaning of "reversible" for a repo that has kept working. The M1 round-trip proof is unaffected — the fixture does an immediate undo, where preserve-work and revert-to-snapshot coincide. Refines D-8 (which put uninstall in scope) with the divergence semantics the review demanded; drove FR-9, FR-11, UB-6/UB-7 in the 000-base.
- **Anchor**: 2026-07-23 · review
- **Status**: live

## Facts

- **F-1**: `prd:installer#NG-3` forecloses merge intelligence inside converge (three-way merge / conflict resolution on repo-owned files) — repo-owned content is never rewritten. — source: `docs/prds/installer/effective.md`
- **F-2**: SCAF-2 (as built in the predecessor) split adoption along the **permission boundary**: phase 1 template-push needs write access (PR by default, `--direct` opt-in), phase 2 branch-protection needs admin; a non-admin gets phase 1 plus a clear hand-off. — source: `docs/prds/scaffolding-and-onboarding.md#SCAF-2`
- **F-3**: SCAF-2 also specified **per-repo required-checks calibration** — required CI contexts derived from what the repo actually runs (flag or repo-config override), "never a forced template superset". — source: `docs/prds/scaffolding-and-onboarding.md#SCAF-2`
- **F-4**: SCAF-2's `--update` re-applied **idempotently, only drifted files landing** — the direct ancestor of converge. — source: `docs/prds/scaffolding-and-onboarding.md#SCAF-2`
- **F-5**: SCAF-3 language profiles are **data, not engine code** ("adding a language = adding a profile directory, no engine change") and already carry a substitution-variable convention. — source: `docs/prds/scaffolding-and-onboarding.md#SCAF-3`
- **F-6**: Phase-1 (config-driven-portability) is contracted and in build: the closed `paths:` schema covers every target-facing doc path, `branch`+`remote` resolve from config, surface/skeleton maps are pure functions of config, the installed CI workflow is a **generated**-class file converge renders from a shipped template, and config load enforces lexical repository containment. — source: `docs/prds/config-driven-portability/effective.md`, `docs/tech-design/config-driven-portability-v1.md`
- **F-7**: Converge already classifies every managed path into engine-verbatim / repo-owned / generated, and already reports `WouldWrite` / `RefusedByName` without mutating — a dry-run reporting surface exists. — source: `internal/converge/converge.go`
- **F-8**: A removal contract already exists in the installer's e2e suite (`internal/installer/e2e/removal_test.go`), and uninstall is separately deferred as `prd:installer#WR-4`. — source: repo tree
- **F-9**: `onboarding` is a settled glossary term meaning *repo* onboarding; the human-facing first-run path is the distinct **welcome path** (SCAF-5). — source: `docs/wiki/glossary.md`
- **F-10**: Vision §8 grounds per-repo onboarding on a `pyxis-config/` extension point plus factory-maintained **repo intelligence** (a living, execution-verified model of the repo). Installer D-7 settled the concrete surface as a single config file rather than a directory. — source: `docs/product-vision.md` §8, `brainstorm:installer#D-7`

## Fog

none *(every decision-shaped input resolved this session; no blocking or non-blocking gaps parked)*

## Out of scope

- **O-1**: SCAF-3 **language profiles** — shipping per-stack descriptors (pinned toolchain verbs, CI suite, templates, labels) rendered at adoption — stay out. `executor.gate.<command>` already makes a non-Go repo functional, so profiles are ergonomics, not a functional gate; building them now repeats the speculation phase-1 D-4 declined. Adoption detects and names the stack instead. (from D-3)
- **O-2**: **Org-wide adoption sweep** (classify + approve + act across many repos) is out — this contract adopts one repo at a time. Its home is `prd:installer#WR-5` / SCAF-4, whose trigger is org-scale adoption demand. (from D-8)
- **O-3**: **Branch protection and bot-identity wiring** are out — SCAF-2's second phase needs admin permission and an App identity, a different permission boundary and concern; deferred at `prd:installer#WR-7`. Adoption stops at what write access can do. (from D-8)
- **O-4**: **Repo intelligence** — vision §8's living, execution-verified model of an onboarded repo's architecture, commands, conventions, and risks — is out. It is a program in its own right, not part of landing the factory in a repo. (from D-8)
- **O-5**: **Upgrade/migration of an already-adopted repo** is out — this contract covers the first adoption; changing installed files across releases is `prd:installer#WR-2` (upgrade UX), triggered by the first release that changes them. (from D-8)
- **O-6**: **Partial (subset) adoption** — taking the doc system without the gates, or the gates without the pipeline skills — is out. It introduces a configuration matrix every downstream behavior would have to honour and test, a large multiplier on a contract already carrying the collision set plus the epoch. Revisit on real demand. (from D-8)

## Candidate glossary terms

- **T-1**: **adopt / adoption** — bringing an *existing, non-empty* repository under factory management by negotiation: inspect → propose a reviewable plan → apply on approval. The verb `pyxis adopt`, distinct from `pyxis init` (which scaffolds a greenfield/empty repo). A specialization of the settled term *onboarding* (repo onboarding), naming specifically the existing-repo path. · avoid: *onboarding* used to mean only the existing-repo case (onboarding covers init too); *install* for the existing-repo path (install is the init-time act).
- **T-2**: **adoption plan** — the durable, reviewable artifact `pyxis adopt` writes before anything is applied: a per-path ledger of what pyxis will add, what it will chain into, which collisions resolve how, what the human must do by hand, and the declared never-touch set. The record of what was agreed; the input a later apply, re-apply, and undo consult. · avoid: *converge report* (converge's ephemeral `WouldWrite`/`RefusedByName` over its own managed set, not the adoption negotiation); bare *plan* (overloaded — the pipeline's stage-6 plan).
- **T-3**: **install epoch** — the adoption-commit baseline recorded in repo config; gates judge only content changed after it, grandfathering the pre-existing corpus wholesale (the generalized-grandfathering shape of `prd:installer#Q-4`). · avoid: *grandfather list* (the enumerated per-check exception surface, e.g. `grandfathered_citations` — the epoch is a single marker, the lists are targeted exceptions that remain available alongside it).
- **T-4**: **never-touch set** — the paths an adoption plan declares pyxis will not read into, write, or configure (the adopter's source, their CI, their docs, their owned convention files); the subject of D-7's byte-identical before/after assertion. · avoid: *repo-owned* (an ownership *class* converge manages; the never-touch set is content pyxis does **not** manage at all).

## Parked technical constraints

- **P-1**: The epoch (D-5) is mechanically uneven across the gate set. History-scoped checks get epoch honouring nearly free — they already compare against a ref, so the span becomes `epoch..HEAD`. **Tree-local checks judge the current tree with no history notion**, so honouring the epoch there needs a per-file "unchanged since epoch" test (a `git log` query per candidate path, or an equivalent snapshot), with a real cost and caching question. Stage-3/4 matter — the shape is settled, the mechanism is not.
- **P-2**: Hook chaining (D-4) must contend with git's **single `core.hooksPath`**. Two sub-mechanisms exist — pyxis's hook delegating to the previously-configured hook, or pyxis installing its hook *into* the directory the repo already uses — and they differ in who owns the file and what uninstall must undo. Which applies depends on whether the adopter uses a hook manager (husky sets `core.hooksPath=.husky`) or stock `.git/hooks`. Detection and the resolution rule are design matter.
- **P-3**: The adoption plan artifact's (D-6/T-2) **home, format, and lifecycle** are design matter: where it lives (a `.pyxis/`-internal path vs a repo-root file the adopter reviews), whether it is committed or transient-then-discarded, its schema (closed, per AP-5), and how apply/undo bind to it (does undo read the committed plan, or a separate applied-manifest?). The negotiation shape is settled (D-6); the artifact mechanics are not.
- **P-4**: The **byte-identity round-trip proof** (D-7/D-8) needs a hermetic mechanism to snapshot and compare the never-touch set (T-4) across adopt→apply→undo — a content hash over the complement of pyxis-managed paths — plus a definition of the never-touch set that is computable from the plan. Test-harness design matter; the observable is settled.
- **P-5**: **Detect-and-surface** (D-3) needs a stack-detection heuristic (marker files → named stack) whose *only* consequence is naming the stack and the required `executor.gate.<command>` action — it must invent no command and gate nothing on a wrong guess (an unknown stack degrades to "unknown; set your gate commands", never an error). The detection surface and its degradation are design matter.

## Gate

| Field | Value |
|---|---|
| **Verdict** | ready-for-prd |
| **Date** | 2026-07-23 |
| **Operator** | @mdarabi |
| **Covering source** | — |
| **Coverage check** | pass — all six inputs covered (§2.1 D-1 · §2.2 D-2/D-10 · §2.3 D-1/D-4/D-8 · §3.1 D-7 · §3.4 O-1…O-6 · appetite D-9), zero blocking fog |
