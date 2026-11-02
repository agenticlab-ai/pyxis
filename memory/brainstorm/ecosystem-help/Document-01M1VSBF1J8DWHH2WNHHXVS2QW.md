# Pyxis ecosystem help — brainstorm record

| Field | Value |
|---|---|
| **In one line** | A post-install discovery skill (working name `/help`) that lets a newly onboarded user explore what the pyxis ecosystem offers and learn how and when to use each feature. |
| **Relevant-when** | brainstorming or contracting post-install onboarding, feature discovery, or help/guide surfaces; checking prior rulings on how new users learn the pyxis pipeline and skills |
| **Idea** | "as part of distributing pyxis to other people, after installation we need to enable them to explore and learn about the pyxis ecosystem and how to use it. i was thinking of a help skill (or similar name) that whenever is triggered list the features of pyxis and how/when to use them." *(operator, 2026-07-23, verbatim)* |
| **Operator** | @mdarabi |
| **Date(s)** | 2026-07-23 |
| **Status** | gated: ready-for-prd *(after gating, the record changes only through the introducing PR's review loop — decisions added or superseded with review anchors — and freezes entirely at merge)* |
| **Tracking** | agenticlab-ai/pyxis#630 (the introducing PR) |
| **Feeds** | `docs/prds/ecosystem-help/` |
| **Prior records** | — |
| **Template version** | brainstorm-record/v1 |

## Orientation

| Source | Why loaded |
|---|---|
| `docs/product-vision.md` | governing scope check — §3 onboarding posture ("one install command plus a design doc — not a wiki crawl"), §5 capability map + litmus test, §9 distribution |
| `.pyxis/pipeline.md` | the existing distributable map of stages → skills — the closest artifact to what the idea describes |
| `docs/prds/README.md` | baseline index routing; SCAF area + built-vs-not status |
| `docs/prds/scaffolding-and-onboarding.md` | SCAF-5 welcome path — the baseline onboarding requirement this idea sits beside |
| `docs/prds/vnext-requirements.md` | adjacency check — VNEXT-6 (repo intelligence), VNEXT-14 (proportional ceremony) |
| `docs/prds/workbench-ui/effective.md` | overlap check — the chartered "reading room" (browse · search · trace · progress) |
| `docs/wiki/README.md` · `docs/wiki/glossary.md` | wiki rules (never authoritative; pointer-heavy) + established vocabulary |
| `docs/wiki/ai-ready-repository.md` | the permanent-newcomer model — the "learn the ecosystem" frame at agent altitude |

**Overlaps surfaced:**

- `.pyxis/pipeline.md` — already ships with the distributable `.pyxis/` surface as the *static* map of every stage, its skill, and where a problem enters; a help skill that hand-authors its own catalog would duplicate it and become a drift vector. Overlap is content, not capability: no interactive surface exists.
- SCAF-5 (`docs/prds/scaffolding-and-onboarding.md`) — the baseline "welcome path" requirement (install + first design doc = productive, no wiki crawl); built in harbormaster, **not built in pyxis**. This idea is its interactive sibling, not a duplicate.
- workbench-ui PRD (reading room) — a chartered browser surface over the *document corpus* (browse/search/trace/progress); different modality and different object (the repo's documents, not the ecosystem's capabilities). Adjacent, not covering.
- Vision §3/§5 — governing posture the idea serves; no conflict found. No prior record or NG/WR row refuses a help/discovery surface.

## Framings

- **A — The living index (router over live truth)** — help is *derived, never authored*: the skill enumerates the live surfaces that already exist (`.pyxis/pipeline.md`, the skills' own descriptions, `.pyxis/authoring/*` headers, wiki routing headers) and renders "what exists, what to use when, where your problem enters" from them. Zero hand-authored catalog → zero drift by construction (the wiki lesson; codify-over-prompt). Implication: most of the work is making the existing surfaces enumerable as data; the skill stays thin. Opens: the same enumeration later feeds agents (repo intelligence), not just humans.
- **B — The guided tour (onboarding curriculum)** — help is *pedagogy, not lookup*: a curated, sequenced walk ("here's the factory, here's the pipeline; now try `/feature-brainstorm` on a toy idea") that takes a fresh user to their first shipped feature. Implication: authored, maintained content with a success metric like time-to-first-feature; directly realizes SCAF-5's welcome path. Risk: curated content is exactly the class the wiki rules distrust — it rots unless something enforces freshness.
- **C — The situational advisor ("what now?")** — help is *stateful*: it reads the repo's current pipeline position (which features have records, PRDs, designs, plans; what awaits a human gate) and answers "given where you are, the next command is X". Implication: needs state derivation from the corpus — the conversational twin of workbench-ui's progress view (J4); strongest for the returning user, weakest for day-1 discovery.

**Destination:** hybrid **A + thin B entry path** — the living index is the founding thesis: the catalog of "what exists, when to use it" is derived from live surfaces (`.pyxis/pipeline.md`, skill descriptions, authoring-guide headers, wiki routing headers), never hand-authored; layered on top is only a small curated "new here? start with X" entry path (which `pipeline.md`'s "where a problem enters" section already half-provides). C (situational advisor) is later work, not the founding thesis. — *operator: "go with your recommendation" (D-1)*

## Decisions

### D-1 — Destination framing

- **Question**: Which framing is the feature — A living index, B guided tour, C situational advisor, or a hybrid?
- **Answer**: The recommended hybrid: A as the core (catalog derived from live surfaces, zero hand-authored content), plus a thin B-style "first steps" entry path; C deferred.
- **Reasoning**: Operator adopted the recommendation — derivation kills the drift risk by construction (the wiki lesson); the thin entry layer covers day-1 discovery without maintaining a curriculum; C needs corpus-state derivation and serves returning users, so it earns its own moment later.
- **Anchor**: 2026-07-23 · turn 2
- **Status**: live

### D-2 — Users and segments

- **Question**: Who is the feature for — primary segment, secondary, anti-segment?
- **Answer**: Primary: the human newcomer on a freshly pyxis-onboarded repo (engineer or operator who just received the `.pyxis/` surface). Secondary: the returning user who forgets which skill fits the moment. Anti-segment: agents.
- **Reasoning**: Operator confirmed the recommendation — agents already navigate by routing headers, `AGENTS.md`, and `pipeline.md`; agent-facing scope would blur into VNEXT-6 repo intelligence. The derived enumeration stays machine-readable as a byproduct, but the feature promises nothing to agents.
- **Anchor**: 2026-07-23 · turn 3
- **Status**: live

### D-3 — Jobs to be done

- **Question**: Which jobs is the feature hired for?
- **Answer**: Three: **J1 orient** ("I just got `.pyxis/` — what's in it?" → what exists, one line each, with when-to-use); **J2 route** ("I have a concrete problem — where does it enter?" → the right skill/stage with a why, `pipeline.md`'s entry section made interactive); **J3 first steps** ("what do I literally do first?" → the D-1 thin entry path). No more.
- **Reasoning**: Operator confirmed the recommendation — all three fall out of the destination; stateful "what's in flight" queries are framing C (deferred by D-1), and deep how-does-X-work reading belongs to the corpus and workbench-ui, not help.
- **Anchor**: 2026-07-23 · turn 4
- **Status**: live

### D-4 — Goals direction

- **Question**: What is the bet, and what does observable success look like?
- **Answer**: The bet: discoverability is the adoption gate for distributed pyxis — a newcomer who can't see what the ecosystem offers never reaches the pipeline; help turns an installed surface into a used one. Success: a newcomer with zero pyxis context, given only an onboarded repo, uses `/help` alone to (a) name what the ecosystem offers, (b) identify the right entry command for a real problem, and (c) take their first productive action — no corpus reading, no human. Bound to an exposure checkpoint (first outside user or fresh-machine dogfood), never a date.
- **Reasoning**: Operator confirmed the recommendation — the vision's no-wiki-crawl posture made measurable; the checkpoint style follows the template rule (targets bind to exposure, not calendar). Stated caveat: until outside users exist, success is provable only by proxy (a fresh session/machine simulating the newcomer) — the PRD should say so rather than invent an unmeasurable metric.
- **Anchor**: 2026-07-23 · turn 5
- **Status**: live

### D-5 — Non-goals and the waiting room

- **Question**: Which refusals are deliberate (NG candidates), and what waits with a revisit trigger (WR candidate)?
- **Answer**: Four refusals: (1) no hand-authored feature catalog — content derives from live surfaces; (2) no stateful pipeline-position advice — workbench-ui's progress view owns that question; (3) no agent-facing promises — agents keep navigating routing surfaces directly; (4) no teaching corpus *content* — help says what exists and when, never restates normative text. One waiting-room entry: the situational advisor (framing C) — revisit when workbench-ui's progress derivation exists to reuse, or when returning-user friction shows up in real use.
- **Reasoning**: Operator confirmed the recommendation — each refusal makes an earlier ruling explicit (D-1's drift kill, D-1's C deferral, D-2's anti-segment, the wiki's pointer-heavy rule) so it lands as a PRD NG/WR row instead of staying implicit.
- **Anchor**: 2026-07-23 · turn 6
- **Status**: live

### D-6 — Appetite

- **Question**: What total spend is this problem worth?
- **Answer**: Small: one skill on the vendor-neutral surface (`.pyxis/skills/help/` + `.pyxis/authoring/help.md` + vendor symlinks), reading only surfaces that already exist. No new server, renderer, document class, or docgate change. Explicitly refused: building an enumeration API or generated index for the surfaces — if reading them as-is proves fragile, that is a follow-up amendment finding, not license to grow this scope. Rollout/support burden ≈ zero (ships with `.pyxis/`).
- **Reasoning**: Operator confirmed the recommendation — the derived-index destination keeps the skill thin by construction; budget expressed as scope, never time, per the template's appetite rule.
- **Anchor**: 2026-07-23 · turn 7
- **Status**: live

### D-7 — One skill, not two; once-ness belongs to the install event

- **Question**: One skill (`/help` containing the first-steps entry path) or two (`/help` + a separate onboarding skill)? And how does onboarding run exactly once on first install?
- **Answer**: One skill. `/help` is stateless and always repeatable; the one-time onboarding moment belongs to the install event — the installer's last act leaves a durable welcome pointer ("run `/help`") per SCAF-5's welcome-artifact pattern. No skill-side once-tracking.
- **Reasoning**: Operator adopted the recommendation — newcomers only ever try `/help` (a second skill is invisible to its own audience); D-1 sized first-steps as a thin layer and a dedicated skill invites curriculum growth; "onboarding" collides with repo onboarding (vision §8, SCAF-2); skills can't self-fire, so a separate skill faces the identical trigger problem; once-state has no good home (repo-tracked denies the second human, machine-tracked breaks on shared clones) and repeatability serves both segments. Splitting later on evidence stays cheap.
- **Anchor**: 2026-07-23 · turn 10
- **Status**: live

## Facts

- **F-1**: The distributable agent surface is `.pyxis/`, and it already carries the pipeline map — "a repo that installs pyxis gets the skills that run these stages *and* the map they belong to" — source: `.pyxis/pipeline.md` (header).
- **F-2**: SCAF-5 — the welcome path ("install + write a design doc — no wiki crawl") — is a baseline parity requirement, shipped in harbormaster, implemented in pyxis: none yet — source: `docs/prds/scaffolding-and-onboarding.md` · `docs/prds/README.md` (status line).
- **F-3**: workbench-ui charters the read-only reading room (browse · search · trace · per-feature progress) over the governed corpus, browser-based and loopback-only — source: `docs/prds/workbench-ui/effective.md` §1.
- **F-4**: The vision's litmus for admitting a new capability: every repo (of its type) needs it; it must be uniform for agents to operate without special-casing; its absence pulls an engineer away from design — source: `docs/product-vision.md` §5.

- **F-5**: Discovered at introducing-PR assembly (post-gate, pre-merge): the install story has materialized upstream — the installer contract is merged (`prd:installer`; its FR-15 charters the one-time welcome work item at `pyxis init`, SCAF-5 parity) and implementation is under way (`cmd/pyxis` on main). The D-7/P-2 seam holds exactly as ruled — the installer owns the once-moment, help owes the pointable first-steps entry; G-1's residue narrows to integration timing. — source: `docs/prds/installer/effective.md` (FR-15) · origin/main at d1a9eae

## Fog

- **G-1** (non-blocking — blocks: stage-3 mechanism/channel choice): what "installation" concretely is today — pyxis has no built installer (SCAF-1/2 are unbuilt parity requirements); distribution currently means the `.pyxis/` surface traveling with a repo. The help skill's trigger point ("after installation") inherits whatever the install story becomes.

## Out of scope

- **O-1**: No hand-authored feature catalog — a maintained duplicate list is the drift vector the living-index destination exists to kill (from D-1, D-5)
- **O-2**: No stateful pipeline-position advice ("what's in flight, what awaits me") — chartered home is workbench-ui's progress view; waiting-room revisit per D-5 (from D-1, D-5)
- **O-3**: No agent-facing promises — agents navigate routing surfaces directly; smarter agent orientation is VNEXT-6 ground (from D-2, D-5)
- **O-4**: No restating corpus content — help points at what exists and when to use it; normative text stays in its governing documents (from D-5)

## Candidate glossary terms

- **T-1**: **onboarding** — in pyxis, *repo* onboarding: bringing a repository under factory management (vision §8; SCAF-2). Never used for the human-facing learn-the-ecosystem path — that is the **welcome path** (SCAF-5) / the `/help` first-steps layer · avoid: "onboarding skill", "user onboarding" (overloads the term on the person least equipped to disambiguate)

## Parked technical constraints

- **P-1**: The skill must live on the vendor-neutral agent surface — content under `.pyxis/skills/` + `.pyxis/authoring/`, vendor dirs as symlinks only (AP-25) — like every existing skill.
- **P-2**: The one-time onboarding moment is the installer's, not the skill's: whatever the install story becomes (G-1), its last act leaves a durable welcome pointer to `/help` (SCAF-5's welcome-artifact pattern). Help owes only an invokable, pointable first-steps entry; it holds no once-state (from D-7).

## Gate

| Field | Value |
|---|---|
| **Verdict** | ready-for-prd |
| **Date** | 2026-07-23 |
| **Operator** | @mdarabi |
| **Covering source** | — |
| **Coverage check** | pass — all six inputs covered (D-1 problem framing · D-2 segments · D-3 jobs · D-4 goals direction · D-5/O-1..O-4 non-goals · D-6 appetite), zero blocking fog (G-1 is non-blocking) |
