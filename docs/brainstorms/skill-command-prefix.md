# Pyxis skill and command prefix — brainstorm record

| Field | Value |
|---|---|
| **In one line** | Rename every pyxis harness skill and slash-command with a `pyxis-` prefix so factory commands are unambiguous inside Codex, Claude Code, Cursor Agent, and other agentic CLIs. |
| **Relevant-when** | brainstorming or contracting agent-surface naming, harness skill registration, slash-command collisions with native CLI skills, or the `pyxis-` namespace; checking prior rulings on AP-25 vendor surfaces, ecosystem help routing, or skill portability |
| **Idea** | "renaming all the pyxis skills and command (those that will be used in side harnesses such as codex, claude code, cursor agent, etc) and add a prefix of "pyxis-" to all of them" *(operator, 2026-07-27, chat, verbatim)* |
| **Operator** | @mdarabi |
| **Date(s)** | 2026-07-27, 2026-07-28, 2026-07-29 |
| **Status** | gated: ready-for-prd |
| **Tracking** | agenticlab-ai/pyxis#808 |
| **Feeds** | `docs/prds/skill-command-prefix/` |
| **Prior records** | — |
| **Template version** | brainstorm-record/v1 |

## Orientation

| Source | Why loaded |
|---|---|
| `docs/architecture-principles.md` (AP-25) | vendor-neutral agent surface — knowledge in `.pyxis/`, vendor dirs pointer-only |
| `docs/techspec.md` § Agent surface | live inventory of built skills and the symlink layout |
| `.pyxis/pipeline.md` | operational map — every stage cites unprefixed slash commands today |
| `docs/brainstorms/ecosystem-help.md` | adjacent discovery surface — `/help` routes to unprefixed commands; derivation purity |
| `docs/tech-design/ecosystem-help-v1.md` | DINV-1 purity + live-surface derivation contract |
| `docs/brainstorms/config-driven-portability.md` | INV-40 skill/hook portability; vendor surface knobs |
| `docs/wiki/glossary.md` | Argus persona vs skill naming; onboarding/help vocabulary |
| `.pyxis/skills/*/SKILL.md` *(tree scan)* | current skill count, frontmatter `name:` fields, invocation examples in descriptions |

**Overlaps surfaced:**

- **ecosystem-help** (`brainstorm:ecosystem-help`, `prd:ecosystem-help`) — `/help` is the derived front door; routing eval fixtures and `help.md` cite unprefixed commands (`/prd-new`, `/argus`, `/feature-brainstorm`, …). Overlap is **content and routing targets**, not capability: help does not own naming policy; it must reflect whatever names the live surface carries after this feature lands.
- **AP-25 / config-driven-portability** (`brainstorm:config-driven-portability`, INV-40) — vendor dirs (`.claude/skills`, `.codex/skills`, `.agents/skills`, `.opencode/skills`) are symlinks into `.pyxis/skills/`; renaming is a surface change, not a knowledge port. Adjacent portability work — not covering this rename.
- **Glossary Argus entry** (`docs/wiki/glossary.md`) — defines **Argus** as the persona/skill name for the AI Engineer. A rename of `/argus` → `/pyxis-argus` touches persona vocabulary, not just invocation — needs an explicit ruling here, not silent drift.
- **pipeline.md + authoring guides** — hardcoded unprefixed slash references across the distributable surface. Mechanical fallout of any rename, not a separate feature.
- No prior brainstorm, NG/WR row, or merged PRD refuses a `pyxis-` namespace → **not covered**.

## Framings

- **A — Invocation-only prefix** — the feature renames only what the operator types in harness chat (`/pyxis-help`, `/pyxis-prd-new`); skill directories and YAML `name:` stay short (`help`, `prd-new`). Each harness adapter maps directory → prefixed slash. Implication: smallest directory churn; risk of permanent directory-name ≠ invocation-name drift and help must learn the mapping layer.
- **B — Canonical unified rename** — one identity everywhere: `.pyxis/skills/pyxis-help/`, frontmatter `name: pyxis-help`, and every authored reference (`pipeline.md`, authoring guides, eval fixtures, wiki pointers) updated to the prefixed form. Vendor symlinks follow automatically. Implication: one mechanical sweep, help derivation picks up new names from the surface; old names die unless a shim is explicitly ruled in.
- **C — Collision-guard prefix (partial)** — prefix only where a pyxis skill would collide with or be confused by a harness-native skill; keep distinctive pyxis identities short (`/argus` stays, generic names like `/help` become `/pyxis-help`). Implication: smallest prefix set; cognitive load of a mixed catalog and a maintained collision matrix.

**Destination:** **B — canonical unified rename** — one `pyxis-` identity across skill directories, frontmatter `name:`, and every live authored reference; vendor symlinks follow; no alias mapping layer. — *operator: "B" (2026-07-27)*

## Decisions

### D-1 — Destination framing

- **Question**: Which framing is the feature — A invocation-only, B canonical unified rename, or C collision-guard partial?
- **Answer**: B — canonical unified rename.
- **Reasoning**: Operator chose B directly — one namespace everywhere, consistent with AP-25's single vendor-neutral surface and ecosystem-help's live derivation (F-4); avoids permanent directory≠invocation drift (A) and the mixed catalog of partial prefixing (C).
- **Anchor**: 2026-07-27 · turn 2
- **Status**: live

### D-2 — Scope boundary

- **Question**: What carries the `pyxis-` prefix in this feature's contract boundary?
- **Answer**: Prefix the distributable agent surface and live routing docs: `.pyxis/skills/*` directories and `name:` fields, matching `.pyxis/authoring/*` guide names/references, `.pyxis/pipeline.md`, help fixtures/tests, and editable wiki routing entries that teach invocation. Exclude engine CLI binaries/verbs (`pyxis`, `pyxis-release`, `docgate`) and frozen merged docs.
- **Reasoning**: Operator confirmed the recommended boundary — the idea targets harness slash-command disambiguation across vendor CLIs, while engine CLIs are a separate namespace and frozen docs are immutable by rule.
- **Anchor**: 2026-07-27 · turn 3
- **Status**: live

### D-3 — Cutover posture

- **Question**: Do unprefixed skill/command names keep working after the rename, or is the cut hard?
- **Answer**: Hard cut — prefixed names only; no compatibility aliases for the old unprefixed forms.
- **Reasoning**: Operator chose hard cut; matches Destination B (canonical unified rename) and avoids a dual-name window that would reintroduce the ambiguity this feature exists to remove.
- **Anchor**: 2026-07-28 · turn 4
- **Status**: live

### D-4 — Users and segments

- **Question**: Who is the feature for — primary segment, secondary, anti-segment?
- **Answer**: Primary: humans and agents invoking pyxis skills inside shared harnesses (Claude Code, Codex, Cursor Agent, OpenCode) where unprefixed names collide with native skills. Secondary: newcomers discovering the surface via `/pyxis-help` / the pipeline map after install. Anti-segment: engine CLI users renaming `pyxis` / `docgate` verbs.
- **Reasoning**: Operator confirmed the recommendation — the pain is namespace collision on the harness skill list; discoverability is secondary (ecosystem-help already owns that path); engine CLIs stay outside per O-1.
- **Anchor**: 2026-07-28 · turn 5
- **Status**: live

### D-5 — Jobs to be done

- **Question**: Which jobs is the feature hired for?
- **Answer**: Two only: **J1 disambiguate** — every pyxis harness skill is visibly namespaced (`/pyxis-*`) so it cannot be confused with a harness-native skill of the same short name; **J2 keep one identity** — directory, frontmatter `name:`, slash invocation, and live routing docs all share the same `pyxis-<skill>` string.
- **Reasoning**: Operator confirmed — teaching the ecosystem stays with ecosystem-help; engine CLI rename and frozen-doc rewrite stay out per O-1/O-2; the two jobs are exactly Destination B + hard cut made observable.
- **Anchor**: 2026-07-28 · turn 6
- **Status**: live

### D-6 — Goals direction

- **Question**: What is the bet, and what does observable success look like?
- **Answer**: Bet: a shared harness skill list without a pyxis namespace will keep colliding as more skills ship; prefixing once is cheaper than per-collision renames later. Success: after the cut, (a) every skill under `.pyxis/skills/` has directory + `name:` + slash form `pyxis-<stem>`; (b) live routing surfaces (`pipeline.md`, help fixtures/eval expectations, editable wiki invocation pointers) cite only prefixed forms; (c) no unprefixed pyxis skill remains invocable on the vendor surface. Bound to the introducing PR merge — not a calendar date.
- **Reasoning**: Operator confirmed — mechanical, merge-bound success criteria match Destination B + hard cut (D-1, D-3) without inventing unmeasurable adoption metrics.
- **Anchor**: 2026-07-28 · turn 7
- **Status**: live

### D-7 — Non-goals and in-iteration additions

- **Question**: Which refusals are deliberate (NG), and what from the proposed waiting room lands in this iteration?
- **Answer**: **NG:** engine CLI rename; rewriting frozen docs for historical command strings; compatibility aliases / soft cutover; per-harness divergent names. **In this iteration (not deferred):** (1) a mechanical gate that new skills under `.pyxis/skills/` must be named `pyxis-*`; (2) a glossary amendment that keeps **Argus** as the persona and names the harness skill `/pyxis-argus`.
- **Reasoning**: Operator redirected both proposed WR items into this iteration — the rename should leave a prevent-regression gate and close the Argus persona-vs-skill vocabulary seam in the same contract, not park them.
- **Anchor**: 2026-07-28 · turn 8
- **Status**: live

### D-8 — Appetite / delivery shape

- **Question**: How large is the appetite, and how does the work land?
- **Answer**: Small-batch appetite delivered as **one PR per skill** (not a single mega-PR that renames all fourteen at once).
- **Reasoning**: Operator redirected the one-PR recommendation to a per-skill PR train — keeps each change reviewable and bisectable while staying a small mechanical batch per landing, not a multi-milestone program.
- **Anchor**: 2026-07-29 · turn 9
- **Status**: live

### D-9 — PR train composition

- **Question**: Where do the mechanical gate and glossary amendment land relative to the per-skill rename PRs?
- **Answer**: Per-skill PRs rename that skill’s directory/`name:` and its skill-local live refs only (hard cut for that skill on merge). A **closing PR** enables the mechanical `pyxis-*` skill-name gate, lands the Argus persona vs `/pyxis-argus` glossary split, and finishes any remaining shared live refs (e.g. `pipeline.md`) not fully updated skill-by-skill.
- **Reasoning**: Operator confirmed the recommendation — avoids mid-train gate false failures on still-unprefixed skills (P-1) while keeping D-7 in-iteration obligations and D-3 hard-cut-per-skill.
- **Anchor**: 2026-07-29 · turn 10
- **Status**: live

### D-10 — Naming stem rule

- **Question**: How is each skill's new canonical identity formed?
- **Answer**: `pyxis-` + current basename unchanged — no shortening, no reordering. Same string for directory, frontmatter `name:`, slash command, and matching authoring guide filename (e.g. `help` → `pyxis-help`, `feature-brainstorm` → `pyxis-feature-brainstorm`, `argus` → `pyxis-argus`).
- **Reasoning**: Operator confirmed ("yes") — keeps the 14-PR train mechanical and preserves J2 one-identity; per-skill shortening would reopen naming debates and fight Destination B.
- **Anchor**: 2026-07-29 · turn 11
- **Status**: live

## Facts

- **F-1**: Thirteen skills ship under `.pyxis/skills/` today: `argus`, `feature-brainstorm`, `help`, `implement-now`, `impl-review`, `incident`, `plan-new`, `prd-amend`, `prd-new`, `prd-review`, `tech-design-brainstorm`, `tech-design-new`, `tech-design-review` — each with a `SKILL.md` frontmatter `name:` matching the directory basename (unprefixed). — source: `.pyxis/skills/*/SKILL.md` *(workspace, 2026-07-27)*
- **F-2**: Vendor integration points are symlinks into `.pyxis/`: `.agents/skills`, `.codex/skills`, `.claude/skills`, `.claude/scripts`, `.githooks` — AP-25 pointer model; renaming skill directories renames what every harness sees. — source: `docs/techspec.md` § Agent surface; `AGENTS.md`
- **F-3**: `.pyxis/pipeline.md` cites unprefixed slash commands for every built stage and cross-cutting skill (e.g. `/feature-brainstorm`, `/prd-new`, `/argus`, `/help`). — source: `.pyxis/pipeline.md` *(workspace)*
- **F-4**: ecosystem-help DINV-1 binds help to live-surface derivation — catalog lines trace to `.pyxis/pipeline.md`, skill frontmatter, and authoring headers; `test-help-purity.sh` harvests skill names from the surface at test time. A rename changes help output with zero help-file edits if names change on the surface. — source: `docs/tech-design/ecosystem-help-v1.md` § DINV-1; `.pyxis/scripts/tests/test-help-purity.sh`
- **F-5**: Glossary defines **Argus** as "the persona/skill name instantiating the AI Engineer" — distinct from informal "autonomous developer." — source: `docs/wiki/glossary.md`

## Fog

none

## Out of scope

- **O-1**: Renaming engine CLI binaries or subcommands (`pyxis`, `pyxis-release`, `docgate`) — command namespace differs from harness slash-commands; not required to satisfy this idea (from D-2).
- **O-2**: Rewriting frozen merged records/designs/PRDs solely to replace historical unprefixed command mentions — immutable artifacts stay as provenance; only live surfaces update (from D-2).
- **O-3**: Compatibility aliases or a soft dual-name cutover for unprefixed skills — hard cut only (from D-3).
- **O-4**: Per-harness divergent skill names — one canonical `pyxis-<stem>` identity everywhere (from D-1, D-5, D-7).

## Candidate glossary terms

- **T-1**: **Argus** (persona) vs **`/pyxis-argus`** (harness skill) — Argus remains the AI Engineer persona; the invocable skill identity is the prefixed form. Avoid: treating "Argus" and the slash-command as interchangeable after the rename (from D-7).

## Parked technical constraints

- **P-1**: Under a one-PR-per-skill train (D-8), the `pyxis-*` skill-name gate (D-7) cannot enforce "all skills prefixed" until the last rename lands without failing mid-train unprefixed directories — resolved product-side by D-9 (gate in closing PR); mechanism remains stage-3/4. — context: D-7, D-8, D-9; hard cut (D-3) still means no aliases once a given skill's PR merges.

## Gate

| Field | Value |
|---|---|
| **Verdict** | ready-for-prd |
| **Date** | 2026-07-29 |
| **Operator** | @mdarabi |
| **Covering source** | — |
| **Coverage check** | pass — all six inputs covered, zero blocking fog |

| prd-template/v1 target | Required record source | Covered? |
|---|---|---|
| §2.1 problem statement | D-1 (Destination B) | yes |
| §2.2 users and segments | D-4 | yes |
| §2.3 jobs | D-5 (J1, J2) | yes |
| §3.1 goals direction | D-6 (plus D-7 in-iteration additions) | yes |
| §3.4 non-goals | O-1…O-4 | yes |
| header appetite | D-8 (plus D-9 train shape) | yes |
