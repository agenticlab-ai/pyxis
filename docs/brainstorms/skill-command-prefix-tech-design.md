# skill-command-prefix — design-brainstorm record

| Field | Value |
|---|---|
| **In one line** | Design exploration for the `pyxis-` harness skill rename: gate mechanism (FR-4), interaction with generated harness skill views, and mechanical shape of the per-skill PR train + closing PR. |
| **Relevant-when** | designing or changing the skill-name prefix gate, the rename train's file/ref strategy, or how converge/generated harness views track canonical skill identity; checking prior design rulings about agent-surface naming |
| **Basis** | `prd:skill-command-prefix` (merged ledger, PR #808) |
| **Authority** | @mdarabi |
| **Date(s)** | 2026-07-30 |
| **Status** | gated: ready-for-design |
| **Tracking** | — |
| **Feeds** | `docs/tech-design/skill-command-prefix-v1.md` |
| **Prior records** | brainstorm:skill-command-prefix |
| **Template version** | design-brainstorm-record/v1 |

## Orientation

| Source | Why loaded |
|---|---|
| `docs/prds/skill-command-prefix/effective.md` | the basis — FR-1..5, UB-1..3, NG-1..4, Q1 (gate mechanism) |
| `docs/brainstorms/skill-command-prefix.md` | stage-1 rulings D-1..D-10; P-1 gate timing → closing PR |
| `docs/tech-design/index.md` | Serves scan — no live design serves `prd:skill-command-prefix` |
| `docs/tech-design/distributed-check-registration.md` | how new gate checks compose without shared-line conflicts (AP-23) |
| `docs/tech-design/ecosystem-help-v1.md` | help derives from live skill frontmatter + pipeline map — rename changes catalog by surface alone |
| `docs/tech-design/code-gate-mirror.md` (index row) | single-source code predicate — adjacent gate family, not content naming |
| `docs/tech-design/repo-layout-and-system-footprint.md` (via FR-31 / content-paths) | system tree + content-purity; PRD prose already constrained |
| `docs/architecture-principles.md` | AP-10/AP-21, AP-23, AP-25, AP-27 |
| `docs/observability/incidents.md` | I1–I3 scanned; no direct match to skill rename |
| `docs/wiki/glossary.md` | Argus persona vs `/pyxis-argus` already split on the PRD PR |
| `internal/converge/harnessskills.go` | generated harness skill views under `.pyxis/generated/harnesses/**` — rename may not be a plain tree symlink anymore |

**Overlaps surfaced:**

- No live design Serves `prd:skill-command-prefix` — space is open.
- **ecosystem-help-v1** — adjacent consumer: catalog/routing follow the live skills tree; no naming policy owned there (stage-1 overlap already recorded).
- **distributed-check-registration** — adjacent substrate if FR-4 lands as a docgate check: add one file, no shared enumeration edit.
- **code-gate-mirror** — adjacent but wrong family for tree-shape/content naming (it's the Go correctness predicate).
- **converge harness views** — potential covering machinery for *what harnesses see*; may extend or constrain how FR-1's "invocable on the vendor surface" is proven after rename. Not a prior refusal.

## Design questions

- **DQ-1**: What concrete mechanism implements the mechanical `pyxis-*` skill-name gate (FR-4 / basis Q1), and how does closing-PR activation work (absent mid-train per UB-2)?
- **DQ-2**: How does the canonical skills-tree rename interact with generated harness skill views (`converge` / `.pyxis/generated/harnesses/**`) so M1(c) "invocable on the vendor surface" stays true?
- **DQ-3**: What is the mechanical unit of a per-skill rename PR (directory, `name:`, authoring guide, skill-local refs) versus what the closing PR still owns?
- **DQ-4**: How are shared live refs (pipeline map, living rollup, cross-skill citations) updated without fighting the one-PR-per-skill appetite?
- **DQ-5**: How are "occurrences of that skill's basename" matched so short names (`help`, `incident`) do not rewrite unrelated English prose?

## Panel log

- **DQ-1**: opens a seam (new enforcement surface for skill directory/`name:` legality) — fired — mechanism chooses which umbrella owns the check and how activation is encoded; ≥ 2 orthogonal shapes exist
- **DQ-2**: extends the existing converge harness-view seam (`ReconcileHarnessSurfaces` already keys off canonical skill directory names) — skipped — no new interface; rename propagates by regenerating views from the renamed tree
- **DQ-3**: extends existing skill/authoring file layout (AP-25 glue + guide) — skipped — mechanical rename of known paths; no new interface
- **DQ-4**: extends DQ-3 (shared refs absorbed into per-skill all-occurrences under D-3/D-4) — skipped — no separate seam; closed by D-3/D-4
- **DQ-5**: opens a seam (occurrence-matching / rewrite safety rule for short basenames) — fired — naive whole-word replace collides with English; ≥ 2 orthogonal matching shapes exist

## Decisions

### D-1 — Gate mechanism: scripts-test / payload conformance (DQ-1)

- **Question**: What implements FR-4's `pyxis-*` skill-name gate — ALT-1 docgate check, or ALT-2 scripts-test/payload conformance?
- **Answer**: ALT-2 — a conformance test in the scripts-test or Go payload harness scans the skills tree for the `pyxis-*` rule; no docgate verb. Closing PR lands the test (absent mid-train ⇒ inactive per UB-2).
- **Reasoning**: Authority chose ALT-2 over the recommended ALT-1 — prefer the help-purity-style mechanical proof on the existing test harness over growing the docgate check set.
- **Anchor**: 2026-07-30 · turn 2
- **Status**: live

### D-2 — Harness views: canonical rename + converge regenerate (DQ-2)

- **Question**: How does the skills-tree rename interact with generated harness skill views so M1(c) stays true?
- **Answer**: Rename only the canonical skills tree; regenerate harness views via existing `ReconcileHarnessSurfaces` / converge — never hand-edit generated views as a second source of truth. Prefer regenerating on each skill PR that renames (so that skill is invocable mid-train); closing PR is the backstop for any remaining view drift.
- **Reasoning**: Authority confirmed the recommendation — F-6 already keys views off directory names; new machinery would duplicate converge.
- **Anchor**: 2026-07-30 · turn 3
- **Status**: live

### D-3 — Per-skill PR: all occurrences of that skill's basename (DQ-3)

- **Question**: What is the mechanical unit of a per-skill rename PR versus what the closing PR owns?
- **Answer**: Each skill PR renames **all occurrences** of that skill's basename — across file contents, file names, and directory names — not merely a skill-local subset. Closing PR still lands the prefix conformance test (D-1) and glossary confirm (FR-5); shared-ref deferral to closing is rejected for that skill's own identity strings.
- **Reasoning**: Authority redirected the skill-local recommendation — one PR per skill must leave no residual unprefixed identity for that skill anywhere the rename is allowed to touch.
- **Anchor**: 2026-07-30 · turn 4
- **Status**: live

### D-4 — P-1 proceed-anyway + live-only rewrite perimeter

- **Question**: How to dispose P-1 (§6.1 skill-local vs D-3 all-occurrences), and does "all files" include frozen docs?
- **Answer**: **Proceed-anyway** on P-1 — each skill PR owns all live occurrences of that skill's identity; closing PR keeps prefix test + glossary confirm. Rewrite perimeter is **live/mutable surfaces only** (skills, authoring, pipeline map, fixtures/evals, living rollups, generated views via converge, editable wiki); frozen merged docs stay provenance (NG-2).
- **Reasoning**: Authority confirmed both recommendations — delivery shape sharpens §6.1 without blocking design; NG-2 remains binding.
- **Anchor**: 2026-07-30 · turn 5
- **Status**: live

### D-5 — Occurrence matching: ALT-3 + full `.pyxis/` tree (DQ-5)

- **Question**: How are skill-basename occurrences matched so short names do not rewrite unrelated English?
- **Answer**: Start from **ALT-3** (structural identity forms: directories, authoring filenames, frontmatter `name:`, slash-commands, path segments in skills/authoring/fixtures/harness views). **Additionally**, within the live `.pyxis/` tree, rewrite **all references** to that skill's basename (content and paths under `.pyxis/`), not only the structural forms. Outside `.pyxis/`, keep ALT-3 structural-only (no bare-prose English replace). Frozen docs still excluded (D-4 / NG-2).
- **Reasoning**: Authority: "start with ALT-3 and include all the references under .pyxis as well" — structural safety globally, exhaustive within the system tree where skill identity is data.
- **Anchor**: 2026-07-30 · turn 6
- **Status**: live

## Alternatives

### ALT-1 — Docgate check (distributed registration)

- **Shape**: A new docgate check (e.g. `check-skill-prefix`) registers via `distributed-check-registration`: scans the skills tree; refuses directories/`name:` that are not `pyxis-*` (after the closing PR lands the check). Mid-train: check file absent → gate inactive (UB-2). Closing PR adds the check (+ glossary confirm + shared refs).
- **Trade-offs**: Wins — same CI/pre-push path as other content gates; AP-23-clean add; merge-base optional for pure tree shape. Costs — another docgate verb; must not fire on mid-train trees until the check exists.
- **Principles**: AP-10 honored (mechanical rule); AP-23 honored (one-file registration); AP-25 honored (checks the neutral tree, not vendor copies).
- **Incidents**: none checked matching (registry read 2026-07-30; I1–I3 are unrelated gate/CLI classes).
- **Verdict**: rejected — authority chose ALT-2 (D-1)
- **From**: DQ-1 · panel

### ALT-2 — Scripts-test / payload conformance (help-purity pattern)

- **Shape**: A conformance test in the scripts-test or Go payload harness scans the skills tree for `pyxis-*` (like help purity). Closing PR lands the test. No docgate verb.
- **Trade-offs**: Wins — proven pattern (ecosystem-help purity); simple. Costs — enforcement rides `go test` / code-gate, not the doc content gate operators expect for tree policy; easier to skip in a docs-only PR path unless wired into both.
- **Principles**: AP-10 honored; AP-25 honored; strains the content-vs-code gate split (naming policy is content/layout, not Go correctness — AP-21 judgment site stays clear but the *umbrella* is wrong-shaped).
- **Incidents**: none checked matching (registry read 2026-07-30).
- **Verdict**: chosen — authority ruling (D-1); closing-PR activation still holds
- **From**: DQ-1 · panel

### ALT-3 — Structural identity forms only

- **Shape**: Rewrite only structurally marked skill-identity forms: skill directories, authoring filenames, frontmatter `name:` / `name: <stem>`, slash-commands (`/<stem>`, `/pyxis-<stem>` after), paths under the skills/authoring trees, and harness/fixture paths that embed the stem as a path segment. Do **not** rewrite bare English words in prose.
- **Trade-offs**: Wins — safe for `help` / `incident`; mechanical and reviewable. Costs — may miss informal prose ("run help") that isn't a structural form; those stay until deliberately edited.
- **Principles**: AP-10 honored; NG-2 honored; AP-25 honored.
- **Incidents**: none checked matching (registry read 2026-07-30).
- **Verdict**: hybridized into D-5 — ALT-3 is the base; within `.pyxis/` expanded to all skill-basename references (D-5)
- **From**: DQ-5 · panel

### ALT-4 — Aggressive whole-token replace with allowlist/denylist

- **Shape**: Replace whole-token occurrences of the basename repo-wide on live surfaces, minus a denylist of known non-skill phrases; optional human review diff.
- **Trade-offs**: Wins — closer to literal "all occurrences" including prose. Costs — high false-positive risk on short stems; denylist becomes a maintained artifact; fights AP-10.
- **Principles**: strains AP-10 (judgment-heavy denylist); NG-2 honored if live-only.
- **Incidents**: none checked matching (registry read 2026-07-30).
- **Verdict**: rejected — authority chose ALT-3-based hybrid (D-5), not denylist replace
- **From**: DQ-5 · panel

## Seam map

- **S-1**: skill-prefix enforcement harness (scripts-test / payload conformance) — opens — behind it: FR-4 refuse non-`pyxis-*` skills after close — feeds: design Scope/Serves for FR-4, UB-2, UB-3
- **S-2**: converge harness-view reconcile — extends — behind it: vendor-visible skill names after rename — feeds: Serves for FR-1 / M1(c); Scope boundary with converge
- **S-3**: per-skill whole-repo identity rewrite (basename occurrences) — opens — behind it: how far a skill PR may rewrite content/paths — feeds: design Scope for FR-1/FR-3 delivery; must respect NG-2
- **S-4**: occurrence-matching rule for skill basenames — opens — behind it: what counts as an identity occurrence vs English prose — feeds: D-3 operationalization; Scope/safety — resolved by D-5 (ALT-3 + exhaustive under `.pyxis/`)

## Facts

- **F-1**: Basis Q1 parks gate mechanism to stage-3/4; product timing is closing-PR activation (UB-2 / stage-1 D-9). — source: `prd:skill-command-prefix` §7.2 Q1; `brainstorm:skill-command-prefix#D-9/#P-1`
- **F-2**: New docgate checks register from their own file via distributed-check-registration — no shared enumeration edit. — source: `docs/tech-design/distributed-check-registration.md` §2
- **F-3**: Help derives catalog from live skill frontmatter + pipeline map; renaming skills updates help output without help-file edits (DINV-1). — source: `docs/tech-design/ecosystem-help-v1.md`
- **F-4**: Operand harness skill discovery may use *generated* views under `.pyxis/generated/harnesses/**` reconciled by `converge.ReconcileHarnessSurfaces`, not only a direct symlink to the canonical skills tree. — source: `internal/converge/harnessskills.go`; doctor expected link targets
- **F-5**: Glossary Argus/`/pyxis-argus` split already landed with the PRD introducing PR; closing PR confirms the prefixed skill form (FR-5). — source: `prd:skill-command-prefix` FR-5; `docs/wiki/glossary.md`
- **F-6**: `ReconcileHarnessSurfaces` walks canonical `.pyxis/skills/*` directories and creates per-view symlinks named after those directories into `.pyxis/generated/harnesses/<view>/skills/`; `implement-now` is the rendered exception. A directory rename on the canonical tree is picked up on the next reconcile. — source: `internal/converge/harnessskills.go`

## Fog

- **G-1** (non-blocking — blocks: design Q-n): Exact home of the ALT-2 conformance test (shell under scripts-test vs Go under `internal/payload` / sibling) — either satisfies D-1; pick in the authored design.

## Out of scope

- **O-1**: Hand-editing generated harness skill views as a parallel identity source — regenerate via converge only (from D-2).
- **O-2**: Rewriting frozen merged documents solely to replace historical unprefixed skill identity strings — NG-2; live-only perimeter (from D-4).
- **O-3**: Bare-English whole-token replace outside `.pyxis/` (ALT-4) — rejected; outside `.pyxis/` only structural identity forms (from D-5).

## Candidate glossary terms

none

## Parked product gaps

- **P-1**: Basis §6.1 splits "skill-local live refs" (per-skill PR) from "shared live refs" (closing PR). D-3 requires every occurrence of that skill's basename in each skill PR — wider than §6.1's skill-local wording. — anchors: `prd:skill-command-prefix` §6.1 · FR-3 — disposition: proceed-anyway per D-4 (optional later `/prd-amend` to align §6.1 prose)

## Gate

| Field | Value |
|---|---|
| **Verdict** | ready-for-design |
| **Date** | 2026-07-30 |
| **Authority** | @mdarabi |
| **Covering design** | — |
| **Coverage check** | pass — core four covered, zero blocking fog |
| **Amend route** | — |

| tech-design-template/v1 target | Required record source | Covered? |
|---|---|---|
| §2 Decision | D-1…D-5 | yes |
| §4 Alternatives | ALT-1…4 verdicted (DQ-1, DQ-5 panels) | yes |
| header Serves + Scope | S-1…S-4 + Basis | yes |
| §3.2 Boundaries / non-goals | O-1…O-3 | yes |
