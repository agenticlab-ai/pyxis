# Existing-repo onboarding — design-brainstorm record

| Field | Value |
|---|---|
| **In one line** | Shaping the `adopt` mechanism: how inspect→plan→apply→undo relates to converge's existing scan/report engine, where the adoption plan lives, how the install-epoch scopes the gates, and how the collision resolutions + stack detection + round-trip proof are built. |
| **Relevant-when** | designing or changing existing-repo adoption (the `adopt` verb, the adoption plan artifact, install-epoch gate-scoping, hook-path chaining, stack detection, the round-trip proof); checking prior design rulings on reusing converge for a non-empty tree or on where the epoch is consumed |
| **Basis** | `prd:existing-repo-onboarding` (000-base merged/frozen #651) |
| **Authority** | @mdarabi |
| **Date(s)** | 2026-07-23 |
| **Status** | gated: ready-for-design *(→ freezes at the introducing PR's merge; until then the doc-review loop may add/supersede review-anchored rulings)* |
| **Tracking** | — |
| **Feeds** | `docs/tech-design/existing-repo-onboarding-v1.md` |
| **Prior records** | `brainstorm:existing-repo-onboarding`, `design-brainstorm:config-driven-portability-tech-design` |
| **Template version** | design-brainstorm-record/v1 |

## Orientation

| Source | Why loaded |
|---|---|
| `docs/prds/existing-repo-onboarding/effective.md` | the basis — 11 FR / 7 UB / 2 ABUSE / 6 DoD; Q1–Q5 are the design-owned open questions this record settles |
| `docs/brainstorms/existing-repo-onboarding.md` | the frozen stage-1 record — D-1..D-11, O-1..O-6, P-1..P-5 (the parked constraints = the DQ set) |
| `docs/tech-design/config-driven-portability-v1.md` | phase-1 design (live) — the generated-workflow + runtime-resolving-hooks split this phase extends |
| `docs/tech-design/installer-v1.md` | the ownership-class model (engine-verbatim/repo-owned/generated), converge, substitution, the release manifest, `pyxis exec` |
| `docs/tech-design/index.md` | the Serves/Builds-on graph — config-driven-portability-v1 Builds-on installer-v1·repo-layout·code-gate-mirror; ecosystem-help-v1 Builds-on config-driven-portability-v1 |
| `internal/converge/converge.go` | the converge engine — `Report{WouldWrite,RefusedByName,Repaired,Regenerated}`, `Action`, `Refusal`, preview mode (the plan precedent) |
| `internal/docgate/config/config.go` | the config schema — the `epoch:` block (`Epoch{SystemVersion,Commit}`) already exists, written once by init |
| `docs/architecture-principles.md` | AP registry — AP-11 (single source), AP-24, AP-25, the same-principal threat model |
| `docs/observability/incidents.md` | incident registry — I5 (executor.gate two-list drift, single-source lesson) and adjacents |
| `docs/wiki/glossary.md` | terms — adopt/adoption, adoption plan, install epoch, never-touch set, ownership class, converge, payload, release manifest |

**Overlaps surfaced:**

- `design:config-driven-portability-v1` · phase-1 (live) already made the installed workflow a generated-class file and the hooks runtime-resolve branch/remote from config. Phase-2 EXTENDS these seams (the adopt-time chaining of an existing hook path; the epoch as a new gate-scoping input) — not a re-litigation. No live design decides the adopt verb, the plan artifact, or epoch-gate-consumption.
- `design:installer-v1` · owns converge, the ownership-class model, and `pyxis init` for the EMPTY tree. `adopt` is the non-empty-tree sibling; the seam question is how much of converge's machinery it reuses (DQ-1). installer NG-3 (no merge intelligence in converge) binds the collision design.
- No prior design-brainstorm or live design covers existing-repo adoption, the epoch's gate-consumption, or the adoption-plan artifact — this is new ground.

## Design questions

*The open solution-space questions in scope (PRD §7.2 Q1–Q5 + the central engine seam), phrased precisely — the panel log keys off these.*

- **DQ-1**: The `adopt` engine — does inspect/plan/apply **reuse converge's ownership-class scan + `Report` machinery** (the plan = a persisted, extended converge preview over a non-empty tree), or is it a **parallel inspector/applier** beside converge? (the central seam — everything else plugs into the answer)
- **DQ-2**: The **adoption plan artifact** (PRD Q3) — its home, format (schema), and lifecycle, and how `apply` and `undo` bind to it (is it converge's `Report` persisted, or a distinct document class?).
- **DQ-3**: **Install-epoch honouring in the gates** (PRD Q1) — the mechanism by which tree-local checks (which judge the current tree with no history notion) scope to content changed since `epoch.commit`, without a per-file cost that grows with the corpus.
- **DQ-4**: **Hook-path collision resolution** (PRD Q2) — chaining into the adopter's existing `core.hooksPath` (delegate-to-prior vs install-into-existing-dir) and how `undo` reverses it, given git's single hook path.
- **DQ-5**: **Stack detection** (PRD Q5) — the marker→stack heuristic whose only output is naming the stack and the required `executor.gate` action, degrading to "unknown" without error.
- **DQ-6**: **The byte-identity round-trip harness** (PRD Q4) — the hermetic mechanism that snapshots and compares the never-touch set across adopt→apply→undo.

## Panel log

*One line per DQ-n (DINV-7) — filled as each resolves.*

- **DQ-1**: opens (the `adopt` engine — a new module boundary over converge) — **fired** — three structurally-distinct shapes (reuse-classifier-hybrid · drive-converge-extended · parallel-inspector) scored against AP-11, I5, NFR-4, NR-2; hybrid (ALT-1) chosen.
- **DQ-2**: opens (the adoption plan — a new document class + its lifecycle) — **fired** — three shapes (durable-receipt · transient+manifest-driven-undo · repo-root) scored against AP-11 (single record) + the namespace-non-collision principle; durable-receipt (ALT-4) chosen.
- **DQ-3**: extends (the gates' scan-set construction — no new seam) — **skipped** — the `epoch.commit` field already exists; honouring it is a `git diff --name-only epoch..HEAD` filter at scan-set construction. Mechanism confirmed with the authority; the per-file and snapshot alternatives were named and declined on the R1/I5 axes, not panelled.
- **DQ-4**: extends (the phase-1 delegating-hook seam + the adopt collision layer) — **skipped** — chaining reuses the engine-verbatim delegating hook; the delegate-forward vs install-into-dir sub-mechanisms (PRD P-2) were named, delegate-forward chosen for clean ownership + undo, not panelled (no new port).
- **DQ-5**: extends (the existing `internal/verify/ecosystem` marker registry) — **skipped** — detection reuses the go/python/node marker patterns; the design choice was ambiguity handling (name-all vs dominant vs any-ambiguity→unknown), name-all-detected chosen, not panelled (no new seam; D-3 already bound detect-and-surface).
- **DQ-6**: extends (the `installer/e2e` fixture + removal_test harness) — **skipped** — the proof is git-native (diff over the non-managed complement after apply; tree-equality to the pre-adoption commit after undo) in the git-init'd fixture; the content-hash alternative was named and declined on the I5 axis, not panelled.

## Decisions

### D-1 — The `adopt` engine's relationship to converge

- **Question**: Does inspect/plan/apply reuse converge's ownership-class scan + `Report` machinery (extend converge / persist its Report as the plan), run as a parallel inspector beside converge, or a hybrid (reuse the classifier, adopt owns plan+apply+undo)?
- **Answer**: **Hybrid (ALT-1).** `adopt` consumes converge's scan/classification as the single source of tree-vs-manifest truth, but owns the adoption-specific layer — collision resolution, manual instructions, the durable plan, the epoch, and undo — on top; converge's steady-state `Run` is not overloaded with one-time adoption semantics.
- **Reasoning**: One classifier keeps a single source of truth (AP-11) and avoids the two-list drift incident I5, which the parallel shape (ALT-3) would reintroduce and NFR-4 forbids. Not overloading converge's `Run` protects its steady-state contract (NR-2), the risk in extend-in-place (ALT-2), whose plan-schema-coupled-to-converge-internals cost the authority also declined. Accepted cost: a clean seam must expose converge's scan independently of its write.
- **Anchor**: 2026-07-23 · turn 1
- **Status**: live

### D-2 — The adoption plan artifact: home, format, lifecycle, undo-input

- **Question**: Is the durable record a plan-as-receipt (one `.pyxis/`-internal artifact, draft→applied→undo), a transient plan with manifest-driven undo (reconstruct from release manifest + epoch + a minimal receipt), or a repo-root reviewable file?
- **Answer**: **Plan-as-durable-receipt (ALT-4).** One closed-schema `adoption-plan/v1` record in the `.pyxis/` engine namespace, rendered human-readable for review; lifecycle draft (`adopt`) → applied (`apply` stamps it with the resolved undo-data: the prior hook path it chained, the exact paths it added) → consumed (`undo`). The agreement and the receipt are one artifact.
- **Reasoning**: a single record is the single source of what-was-agreed and what-was-done (AP-11), where the manifest-driven shape (ALT-5) splits it across manifest + config + receipt (a coherence/drift surface). The `.pyxis/` home keeps it in the engine namespace with no collision, which the repo-root shape (ALT-6) forfeits. The operator commits it with the adoption (consistent with apply creating no commit of its own).
- **Anchor**: 2026-07-23 · turn 2
- **Status**: live

### D-3 — Install-epoch honouring mechanism in tree-local gates

- **Question**: How do tree-local gates scope to content changed since `epoch.commit` without a per-file cost growing with the corpus (R1) — a single `git diff` filter, a per-file `git log` query, or a content-hash snapshot?
- **Answer**: **A `git diff --name-only epoch.commit..HEAD` (∪ working-tree) filter computed once per gate run and intersected with the scan-set** — only changed-since-epoch files are checked; the grandfathered corpus is never visited. History-scoped checks (immutability/governing) already ref-compare and are unaffected. An unreachable epoch commit (shallow clone) fails closed — gate everything, name the cause.
- **Reasoning**: cost scales with the changed set, not the corpus (NFR-1 exactly), on one git call — where the per-file query is the O(files) cost R1 flagged, and the snapshot is a second "what changed" source (I5 drift) that re-implements `git diff`. The epoch lives where it already is (`epoch.commit`); this only extends the gate's file-selection, opening no new seam.
- **Anchor**: 2026-07-23 · turn 3
- **Status**: live

### D-4 — Hook-path collision resolution and its undo

- **Question**: The adopter already sets `core.hooksPath` (git allows one). Does pyxis own the hooksPath and delegate forward to the prior hook, install its hook into the adopter's existing dir, or not chain and instruct the human?
- **Answer**: **pyxis owns `core.hooksPath` (its own `.githooks`) and its hook delegates forward** — runs its checks, then invokes the adopter's previously-configured hook. The prior hooksPath is stamped into the applied plan; `undo` restores it (a one-value restore).
- **Reasoning**: reuses phase-1's already-delegating engine-verbatim hook, writes no foreign file into the adopter's tooling dir (the install-into-dir option edges toward NG-2 and husky-regeneration drops it), and gives undo a clean single-value restore. Chaining is an honest mechanism, so falling back to instruct (reserved for no-honest-mechanism cases, D-4) would forfeit a safe turnkey win. Accepted operability risk: a later husky re-install may reset hooksPath → doctor detects, converge repairs (rides the design as Q-1).
- **Anchor**: 2026-07-23 · turn 4
- **Status**: live

### D-5 — Stack detection and its degradation

- **Question**: Reusing the existing ecosystem marker registry, what does detection do on an ambiguous (polyglot) or unrecognized repo — name all detected, name the single dominant, or degrade any-ambiguity to unknown?
- **Answer**: **Name every detected stack** and the required `executor.gate.<command>` action for each; if no marker matches, one plan line "unknown stack — set your gate commands." Never an error, never a fabricated command.
- **Reasoning**: naming all detected stacks keeps the "nothing silently assumed" promise (M2) that dominant-only dents by dropping a second stack's action, and it preserves correct information that any-ambiguity→unknown discards. Reuses the existing `internal/verify/ecosystem` marker patterns (no new registry), and honours D-3's detect-and-surface (name, never wire or invent).
- **Anchor**: 2026-07-23 · turn 5
- **Status**: live

### D-6 — The byte-identity round-trip harness mechanism

- **Question**: How does the harness express the byte-identity proof in the git-init'd fixture — git-native (diff/tree-equality) or an independent content-hash map?
- **Answer**: **git-native.** Never-touch: `git diff` over paths outside the plan's managed set is empty after apply. Round-trip: after undo, the tree equals the pre-adoption commit (git diff empty; git's tree SHA is the whole-tree hash). Reuses the `installer/e2e` fixture + `removal_test` patterns.
- **Reasoning**: the fixture is already a git repo, so git is the hermetic differ D-7/D-11 need for free; the content-hash map re-implements `git diff`/tree-SHA and adds a second "what changed" computation (the I5 flavour DQ-3 avoided). Directly expresses the never-touch byte-identity and the round-trip.
- **Anchor**: 2026-07-23 · turn 6
- **Status**: live

## Alternatives

### ALT-1 — Reuse-classifier hybrid (chosen)

- **Shape**: `adopt` calls converge's scan → per-path ownership-class findings (the one classifier); an adopt layer consumes those findings to build the plan (collision/manual/never-touch classes), and `apply` calls converge's write primitives for the engine-verbatim/generated classes while owning collision + epoch + undo itself.
- **Trade-offs**: wins one source of truth + isolated adoption logic + untouched steady-state Run; costs a refactor to expose converge's scan⟂write boundary.
- **Principles**: AP-11 (single source — one classifier); NFR-4 honored (reuse the ownership machinery); NR-2 honored (converge Run untouched).
- **Incidents**: I5 (executor.gate two-list drift) — averted by a single classifier, checked `docs/observability/incidents.md`.
- **Verdict**: chosen — single-source reuse without overloading converge's steady-state contract.
- **From**: DQ-1 · panel

### ALT-2 — Drive converge, plan = extended Report

- **Shape**: extend converge's classification in place with the collision classes and persist its `Report` as the plan; `apply` = `converge.Run` scoped to the approved plan; `undo` reverses recorded actions.
- **Trade-offs**: wins maximal reuse and never-touch = "not in manifest"; costs surgery on a load-bearing component built for a tree it owns, plan schema coupled to converge's internal types, steady-state risk.
- **Principles**: AP-11 honored (one engine) but NR-2 strained (Run carries one-time adoption semantics).
- **Incidents**: I5 — not triggered (one engine), checked `docs/observability/incidents.md`.
- **Verdict**: rejected — overloads converge's steady-state Run and couples the plan artifact to converge internals.
- **From**: DQ-1 · panel

### ALT-3 — Parallel adopt inspector/applier

- **Shape**: `adopt` is a standalone component that reads manifest + tree, classifies, builds its own plan schema, and applies; converge stays entirely untouched.
- **Trade-offs**: wins full isolation + a clean adoption-owned plan schema + undisturbed converge; costs two engines classifying tree-vs-manifest.
- **Principles**: NFR-4 violated (parallelizes rather than reuses); AP-11 strained (two classifiers).
- **Incidents**: I5 (two-list drift) — directly reintroduced by two independent classifiers, checked `docs/observability/incidents.md`.
- **Verdict**: rejected — the exact two-list drift I5 warns against; duplicates converge's ownership walk.
- **From**: DQ-1 · panel

### ALT-4 — Plan-as-durable-receipt (chosen)

- **Shape**: one `adoption-plan/v1` closed-schema record under `.pyxis/`, rendered for review; `adopt` writes the draft, `apply` stamps it applied with the resolved undo-data (prior hook path, added paths), `undo` reads it. One artifact carries agreement + receipt.
- **Trade-offs**: wins a single source of what-was-agreed-and-done + engine-namespace non-collision; costs keeping the artifact committed and valid for undo to work.
- **Principles**: AP-11 (one record); AP-5 (closed schema-versioned struct); the namespace-non-collision principle (lives under `.pyxis/`).
- **Incidents**: I5 — averted (one record, not a split), checked `docs/observability/incidents.md`.
- **Verdict**: chosen — single durable record, no collision, undo reads the same artifact it reviewed.
- **From**: DQ-2 · panel

### ALT-5 — Transient plan + manifest-driven undo

- **Shape**: the plan is ephemeral (review only, uncommitted); `apply` records what it did across the release manifest (added paths), the config epoch, and a minimal receipt (chained hook, manual items); `undo` reconstructs from those three.
- **Trade-offs**: wins no new durable artifact + manifest reuse; costs "what adoption did" split three ways.
- **Principles**: AP-11 strained (three records of one fact); reuses the manifest (NFR-4-adjacent).
- **Incidents**: I5 (two-/multi-list drift) — reintroduced across manifest+config+receipt, checked `docs/observability/incidents.md`.
- **Verdict**: rejected — splits the adoption record across three surfaces, a drift surface.
- **From**: DQ-2 · panel

### ALT-6 — Repo-root reviewable plan

- **Shape**: same draft→applied→undo lifecycle but the artifact is a committed repo-root file (e.g. `ADOPTION.md`), prominent to the adopter.
- **Trade-offs**: wins maximum reviewer visibility; costs a pyxis artifact in the adopter's root namespace + an unasked-for root file.
- **Principles**: the namespace-non-collision principle strained (pyxis writes to the adopter's root).
- **Incidents**: none checked `docs/observability/incidents.md`.
- **Verdict**: rejected — forfeits engine-namespace containment for visibility the rendered review already gives.
- **From**: DQ-2 · panel

## Seam map

- **S-1**: converge's **scan ⟂ write** boundary — **opens** (converge's scan/classification must be callable independently of its write, so `adopt` can consume findings without invoking `Run`) — behind it: the ownership-class classifier as a reusable read surface — feeds: the design's Scope (converge refactor) + §2 Decision.
- **S-2**: the **`adopt` module** (inspect/plan/apply/undo) — **opens** — behind it: the adoption layer that consumes S-1 and owns the plan, collisions, epoch, undo — feeds: Serves prd:existing-repo-onboarding/FR-1..FR-11.
- **S-3**: the **`adoption-plan/v1` document class** — **opens** — behind it: the closed-schema durable receipt (draft→applied→consumed) under `.pyxis/`, rendered for review — feeds: Serves prd:existing-repo-onboarding/FR-1 · prd:existing-repo-onboarding/FR-2 · prd:existing-repo-onboarding/FR-10 + the design's Scope (a new artifact contract).
- **S-4**: the **gate scan-set epoch filter** — **extends** — behind it: docgate's file-selection gains a `git diff epoch..HEAD` intersection reading `epoch.commit` — feeds: Serves prd:existing-repo-onboarding/FR-8 · prd:existing-repo-onboarding/NFR-1 (no new seam, a gate-layer extension).
- **S-5**: the **delegating-hook chain** — **extends** — behind it: the phase-1 engine-verbatim hook gains a forward-delegation to the adopter's prior hook; adopt owns `core.hooksPath`, the plan stamps the prior path for undo — feeds: Serves prd:existing-repo-onboarding/FR-4 · prd:existing-repo-onboarding/FR-9.
- **S-6**: **stack detection** — **extends** — behind it: `internal/verify/ecosystem`'s marker registry read to name (not wire) the detected stack(s) + the `executor.gate` action — feeds: Serves prd:existing-repo-onboarding/FR-7.
- **S-7**: the **round-trip proof harness** — **extends** — behind it: the `installer/e2e` fixture + `removal_test` patterns, git-native diff/tree-equality — feeds: DOD-1/M1 verification (not a Serves seam; the acceptance harness).

## Facts

- **F-1**: converge already produces a `Report` with `WouldWrite` (the exact write set a real run would make), `RefusedByName` (`Refusal{Path,Class,Reason}`), `Repaired`, and `Regenerated`, plus a preview mode — a "what would happen" surface without mutating. — source: `internal/converge/converge.go`
- **F-2**: The config schema already carries an `epoch:` block (`Epoch{SystemVersion, Commit}`, keys `epoch.commit`/`epoch.system_version`), written once by init (installer-v1 §3.5 step 8), zero until then — the install-epoch has a home; the open work is *consuming* it in the gates. — source: `internal/docgate/config/config.go`
- **F-3**: converge acts only on manifest-listed paths; a path not in the release manifest (the adopter's own source) is never touched — the never-touch property is partly inherent, and the collision is only where a manifest path (AGENTS.md, the workflow) already carries adopter content. — source: `internal/converge/converge.go`, `installer-v1` ownership-class model
- **F-4**: Phase-1 made the installed workflow generated-class (converge renders from a config-substituted template) and the git-hooks runtime-resolve branch/remote from config, staying engine-verbatim. — source: `design:config-driven-portability-v1#3.1`
- **F-5**: installer-v1 NG-3 forecloses merge intelligence in converge; repo-owned content is never rewritten. — source: `prd:installer/NG-3`

## Fog

- **G-1** (non-blocking — blocks: the authored design's own Q-n): a later hook-manager re-install (e.g. `husky install`) may reset `core.hooksPath`, silently displacing pyxis's chained entry (D-4). Detection/repair posture (doctor names it, converge restores) rides the design as an operability open question — not blocking the direction.

## Out of scope

- **O-1**: The converge change is bounded to **exposing the scan⟂write boundary (S-1)** — making the ownership-class classifier callable independently of `Run`. Extending `converge.Run` itself with adoption semantics (collision classes, the plan, non-empty-tree apply) is refused: adoption logic lives in the adopt layer, keeping converge's steady-state contract intact (NR-2). (from D-1)
- **O-2**: The adopt module reuses converge's classifier and git-native primitives rather than re-implementing tree-vs-manifest classification, drift computation, or byte-diffing — no parallel engine (DQ-1/DQ-3/DQ-6 all declined a second "what changed" source on the I5 axis). (from D-1)

## Candidate glossary terms

none *(the PRD already coined adopt/adoption, adoption plan, install epoch, never-touch set; the design adds only the `adoption-plan/v1` schema label, an artifact name not a new domain term)*

## Parked product gaps

none *(no basis defect discovered yet)*

## Gate

| Field | Value |
|---|---|
| **Verdict** | ready-for-design |
| **Date** | 2026-07-23 |
| **Authority** | @mdarabi |
| **Covering design** | — |
| **Coverage check** | pass — core four covered (§2 Decision: D-1/D-2 · §4 Alternatives: ALT-1..6 · Serves/Scope: S-1..S-7 + basis · §3.2 Boundaries: O-1/O-2), zero blocking fog |
| **Amend route** | — |
