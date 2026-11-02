# Channel inbox — brainstorm record

| Field | Value |
|---|---|
| **In one line** | Surface distributor→operator notices inside the agentic CLI — session-start hooks run a `pyxis inbox` verb that relays unseen channel messages (a new-version advisory as the first kind), deduped by a user-global seen-ledger; the notification leg of installer WR-2. |
| **Relevant-when** | brainstorming or contracting update notification, distributor→operand messaging, session-start hook surfaces, upgrade UX (installer WR-2), or force-upgrade/revocation posture; checking prior rulings on notification dedupe, seen-state storage, or channel message schemas |
| **Idea** | "how should we know the user a newer version is avaialble when they are using pyxis? ( given the only surface they come in touch with pyxis ecosystem is whitin their agentic cli). Also how can we have a 'force-upgrade' in case of bad version rollout?" … "ok, makes sense on no path forward for force-upgrade. going back to the notification process. can we use hooks in each agentic cli to check for a new message (this could be a new version as the first use case) and show it only once (we need a local storage along the pyxis-installer binary (the one that installs in the ~/.local/bin)" *(operator, 2026-07-23, chat, verbatim)* |
| **Operator** | @mdarabi |
| **Date(s)** | 2026-07-23 |
| **Status** | gated: ready-for-prd *(after gating, the record changes only through the introducing PR's review loop — decisions added or superseded with review anchors — and freezes entirely at merge)* |
| **Tracking** | — *(the introducing PR, once filed)* |
| **Feeds** | `docs/prds/channel-inbox/` |
| **Prior records** | `brainstorm:installer` |
| **Template version** | brainstorm-record/v1 |

## Orientation

| Source | Why loaded |
|---|---|
| `docs/product-vision.md` §9/§11 | governing scope — the vision's distribution ground names auto-updaters, staged rollout, beta channels, and "the platform loop"; this idea must sit inside it without becoming it |
| `docs/prds/README.md` | index — "Auto-update channels / A/B cohorts / global telemetry" listed as new ground, no covering ledger |
| `docs/prds/installer/effective.md` *(origin/main)* | the adjacent contract: WR-2 parks upgrade UX; FR-13 offline floor; FR-14 doctor read-only; FR-22 artifact-source override |
| `docs/tech-design/installer-v1.md` *(origin/main)* | frozen constraints: pin-only execution, `pyxis exec` grammar, doctor as read-only face, the epoch block |
| `docs/tech-design/repo-layout-and-system-footprint.md` | the no-unpinned-execution rule, the release channel, gitignored `.pyxis/bin/` cache |
| `docs/brainstorms/installer.md` *(origin/main)* | prior rulings O-2 (upgrade UX → WR), O-7 (update channels = reserved config keys; platform loop its own program) |
| `docs/brainstorms/ecosystem-help.md` *(gated, unmerged)* | adjacency check — the pull-based post-install discovery skill |
| `docs/wiki/glossary.md` | term hygiene; the wake-signal term's instruction-free posture as adjacent precedent for channel-carried content |
| `README.md` § Releases & distribution *(origin/main)* | live state: pyxis-dist mirror, pinned-tag `install.sh` consumer path |
| `install.sh` *(origin/main)* | the bootstrap target the idea names: `~/.local/bin` default, `PYXIS_INSTALL_DIR` override |

**Overlaps surfaced:**

- **`prd:installer` WR-2 / `brainstorm:installer#O-2`** — upgrade UX (channels, changelogs, migration notes) is parked with revisit trigger "the first release that changes installed files"; releases v0.1.0/v0.1.1 have shipped and pyxis-dist is live, so the trigger has effectively fired. This idea is that WR row's notification leg — parked ground being picked up, not covered capability.
- **`brainstorm:installer#O-7`** — telemetry, **update channels**, and the autonomy dial are *reserved config keys only*; the platform loop (vision §9/§11) is its own future program. A message inbox must stay distinct from rollout channels/cohorts — or consciously reopen O-7.
- **`brainstorm:ecosystem-help`** *(gated, unmerged)* — the `/help` discovery skill is the *pull* surface for learning the ecosystem; this idea is the *push* surface for time-sensitive notices. Adjacent, not covering; the boundary deserves a ruling.
- **`docs/product-vision.md` §9** — the vision's endgame includes auto-updaters; the frozen installer design rules execution stays pin-only (DINV-2). Notification-not-mutation is the compatible near-term step, not a conflict.

## Framings

- **A — The upgrade nudge** — the feature *is* WR-2's notification leg and nothing more: `pyxis` derives pin-vs-latest locally from the cached release stamp and surfaces it once per (repo, version) at session start, doctor, and CI. No authored messages, no channel file, no schema. Smallest build; a bulletin channel would be a later, separate feature.
- **B — The channel bulletin (system inbox)** — the feature *is* an authenticated distributor→operator message channel: a signed, append-only, closed-schema message file in the release channel, typed message kinds rendered through pyxis-owned templates, severity tiers governing dedupe (advisory shows once via the seen-ledger; blocking re-fires until cleared). The new-version advisory is the first kind — *derived* locally, not authored — and deprecation/security notices follow as authored kinds. The inbox verb + hooks + seen-ledger are the product.
- **C — The session briefing** — the feature *is* a session-start operand briefing: a doctor-lite pass relaying everything actionable (manifest drift, unauthored skeletons, update available, channel notices) through the agent each session. The inbox is one feed into a broader health surface.

**Destination:** **B — the channel bulletin (system inbox)**, holding A's discipline inside it: v1 ships exactly one authored message kind plus the locally-derived version advisory; rollout channels/cohorts stay out (`brainstorm:installer#O-7` undisturbed). — operator adopted the recommendation: the machinery (hook wiring, storage, injection hardening) is needed even for the nudge alone, so the generality should be paid for once; the operator's own articulation was bulletin-shaped ("a new version as the first use case").

## Decisions

### D-1 — destination: the channel bulletin, nudge-disciplined

- **Question**: Which framing is the feature — A (upgrade nudge only), B (authenticated channel bulletin with the version advisory as first kind), or C (session briefing)?
- **Answer**: B, as recommended.
- **Reasoning**: A builds all the machinery yet forecloses generality; C balloons into doctor UX the frozen installer design owns. Scope valve: one authored kind + the derived advisory in v1; rollout channels/cohorts remain reserved (O-7).
- **Anchor**: 2026-07-23 · "go with your recommendation"
- **Status**: live

### D-2 — segments: operand operator via agent relay; drives and the source repo excluded

- **Question**: Who is the bulletin for — and who explicitly not?
- **Answer**: Confirmed as recommended: primary = the operand operator (human in an agentic CLI on an installed repo; the agent is relay medium, never audience); producer = the distributor, authoring thin via the release runbook (no tooling in v1). Anti-segments: unattended drives (argus/headless executors never act on a bulletin autonomously — relay-to-human only) and the pyxis source repo (source mode; learns of releases by making them).
- **Reasoning**: The relay-not-audience split keeps decision authority human; the drive anti-segment turns P-1's injection hardening from mitigation into policy — channel content is never carried logic (wake-signal posture).
- **Anchor**: 2026-07-23 · "confirmed"
- **Status**: live

### D-3 — v1 jobs: version advisory + authored notices + never-pay-for-it; no read receipts

- **Question**: What are the v1 jobs?
- **Answer**: Confirmed as recommended: (1) the derived version advisory — pin-vs-latest per repo, relayed at session start, deduped on (repo, pin, latest), remedy attached; (2) distributor-authored can't-wait notices — severity-tiered, `advisory` dedupes via the seen-ledger, `blocking` re-fires until cleared; (3) the negative job, contract-grade — fail-silent, exit-0, cache-only offline, bounded fetch, zero repo writes. Shaped out: fleet read-receipts/telemetry — delivery is best-effort local; must-land enforcement stays CI (the revocation future).
- **Reasoning**: The three jobs cover discover/reach/trust; the shape-down keeps the O-7 platform-loop boundary and the no-phone-home posture intact.
- **Anchor**: 2026-07-23 · "confirmed. also this feature should have a toggle…"
- **Status**: live

### D-4 — a mute toggle exists

- **Question**: *(operator-initiated)* The feature must offer a toggle to turn notifications off — the operator can opt out of being distracted.
- **Answer**: A user-settable mute toggle is in scope for v1.
- **Reasoning**: Notification surfaces earn trust by being silenceable; an operator who cannot opt out routes around the whole channel. Shape (scope, tier semantics) resolved in D-5.
- **Anchor**: 2026-07-23 · "this feature should have a toggle that the user can set to avoid being distracted"
- **Status**: live

### D-5 — mute shape: user-global, advisory-tier only, snooze-first

- **Question**: The mute toggle's shape — scope, what it silences, and whether snooze exists?
- **Answer**: Confirmed as recommended: user-global (stored beside the seen-ledger, never repo config — a committed mute would silence teammates); silences the `advisory` tier only — `blocking` still fires and CI surfacing is untouched; `mute --for <duration>` snooze is the recommended default gesture, bare `mute` the permanent form; state visible in `doctor` / inbox status.
- **Reasoning**: Distraction is a property of the human, not the repo; a channel whose emergency lane can be muted has no emergency lane (the revocation future leans on it); snooze acknowledges "not now" without the operator silently falling off the channel forever.
- **Anchor**: 2026-07-23 · "confirmed"
- **Status**: live

### D-6 — the bet and observable success

- **Question**: What is the bet, and how do we see it working?
- **Answer**: Confirmed as recommended. Bet: operands drift out of date because the channel has no voice on the surface operators actually live in (their agentic CLI); an authenticated, severity-tiered voice there lands pin-bumps without polling and reaches the fleet in one session-start when a bad version ships. Observable success: (1) fleet freshness — median installed pin moves within N sessions of the advisory first firing, derivable from operand git history, no telemetry; (2) zero-cost holds — the negative job stays provable as an e2e gate (no session wounded, gate blocked, or repo write; offline silent); (3) the bad-rollout drill — a staged known-bad-version notice reaches a test operand's next session and is relayed with its remedy. Direction only: v1 commits to no numeric N; the PRD baselines it from our own fleet.
- **Reasoning**: Ties success to fleet behavior we can already observe without a phone-home; the drill makes the emergency lane acceptance evidence before it's needed in anger.
- **Anchor**: 2026-07-23 · "confirmed"
- **Status**: live

### D-7 — appetite: feature-scale, cut-line explicit

- **Question**: What is the appetite?
- **Answer**: Confirmed feature-scale (well below installer's program-scale). In: the `pyxis inbox` verb (relay/hook mode + mute/unmute/status), user-global XDG state (seen-ledger + snapshot + mute), the locally-derived version advisory, one signed closed-schema authored kind, the two severity tiers + dedupe rule, session-start hook wiring for the vendor CLIs pyxis already targets, the distributor's thin runbook-append authoring path, the fail-silent/offline/zero-write contract, and the bad-rollout drill. Cut-line (parked, not built): message-authoring tooling, the revocation/force-refuse machinery (its own security contract, referenced), rollout channels/cohorts/telemetry (O-7 reserved), the framing-C session briefing, any host beyond those already wired. Complexity valve: cut toward fewer typed template-rendered fields, never free-form channel text (the P-1 injection line).
- **Reasoning**: A verb, a state file, a hook, and a schema — the machinery framing B pays for, nothing more; the valve protects the injection boundary under scope pressure.
- **Anchor**: 2026-07-23 · "confirmed"
- **Status**: live

### D-8 — hook coverage: wire-where-possible + manual fallback, one verb; per-CLI enumeration deferred

- **Question**: Which agentic-CLI surfaces does v1 wire, and what's the fallback where a CLI has no session-start hook?
- **Answer**: Confirmed as recommended. The contract, not a fixed CLI list: wire a thin per-CLI session-start adapter fragment (`pyxis init` merges it, exactly like the writehook) for every vendor CLI that exposes a session-start lifecycle hook — Claude Code definitively; where a CLI lacks one, fall back to the payload-shipped operand manual / AGENTS.md instructing the agent to run `pyxis inbox` at session start. One verb (`pyxis inbox --hook`) regardless of caller — wiring is per-surface, logic is not. The per-CLI hook-surface enumeration is a stage-3 constraint (P-3), not pinned now.
- **Reasoning**: A fixed list would freeze against vendor capabilities that shift; the contract keeps every surface covered (wired or manual) with no capability left dark, and the tech design finalizes coverage against each vendor's actual hook surface.
- **Anchor**: 2026-07-23 · "confirmed"
- **Status**: live

## Facts

- **F-1**: The bootstrap installs `pyxis` to `~/.local/bin` by default, overridable via `PYXIS_INSTALL_DIR` — so "storage next to the binary" is a moving target; user-global state needs its own home. — source: `install.sh:19,42` *(origin/main)*
- **F-2**: Vendor-hook precedent exists: `.pyxis/writehook/claude.settings.json` ships a Claude Code hook config that invokes a pyxis binary — the wiring pattern the inbox hook would generalize. — source: `.pyxis/writehook/claude.settings.json` *(origin/main)*
- **F-3**: FR-13 contracts the offline floor: init, converge/repair, doctor, resolution, the hook gate, and skill invocation complete with **zero network operations** post-fetch in the carina+local-CI combo — a session-start network check must not break this. — source: `docs/prds/installer/effective.md` FR-13
- **F-4**: Session-start hook stdout lands in the **agent's context**, not on the human's screen — the human hears a notice only if the agent relays it; channel-carried text is therefore injected into every fleet agent's context (a prompt-injection surface by construction). — source: vendor CLI hook semantics (Claude Code SessionStart; the `.pyxis/writehook` pattern)
- **F-5**: The installer brainstorm reserved `update channels` as config keys only (`#O-7`) and parked upgrade UX as WR-2 with the revisit trigger "first release that changes installed files" (`#O-2`); v0.1.0/v0.1.1 have shipped, pyxis-dist is live. — source: `brainstorm:installer#O-2,#O-7`; `README.md` § Releases & distribution *(origin/main)*
- **F-6**: Force-upgrade decomposes to revocation-not-push under DINV-2 (no unpinned execution): the achievable guarantee is "a revoked version is refused within one channel contact or one CI run" — the operator accepted this posture in-session ("makes sense on no path forward for force-upgrade"). — source: `docs/tech-design/installer-v1.md` DINV-2; operator chat 2026-07-23
- **F-7**: The only agentic-CLI hook wired today is Claude Code's `PreToolUse` (`.pyxis/writehook/claude.settings.json` — a pointer fragment merged into `.claude/settings.json`, delegating to a per-CLI adapter `cmd/argus-writehook`/`internal/argus/writehook/adapter_claude.go`). Session-*start* lifecycle hooks are a Claude Code capability; Codex/OpenCode equivalents are not established in the repo. — source: `.pyxis/writehook/claude.settings.json`, `cmd/argus-writehook` *(origin/main)*
- **F-8**: pyxis wires operand hooks by the payload-overlay model: fragments ship under `.pyxis/dist/operand-hooks/**` (engine-verbatim class), and `pyxis init` installs them by name, thin (locate `pyxis` on PATH → delegate → propagate exit) — the pattern an inbox session-start adapter would follow. Recognized agent surfaces: `.claude/`, `.codex/`, `.agents/`, `.opencode/` (pointers into `.pyxis/`, AP-25). — source: `.pyxis/dist/operand-hooks/README.md` *(origin/main)*

## Fog

none

## Out of scope

- **O-1**: Fleet read-receipts, acknowledgment, and delivery telemetry — delivery is best-effort local; anything that *must* land is enforced at CI (the revocation future), not the inbox. Keeps the no-phone-home posture. (from D-3)
- **O-2**: The revocation / force-refuse machinery — its own security contract (severity, who may append, signing), referenced by this feature's channel but not built here. (from D-7; F-6)
- **O-3**: Rollout channels, A/B cohorts, and the platform loop — remain `brainstorm:installer#O-7` reserved config keys; a message inbox is not a rollout channel. (from D-7)
- **O-4**: The framing-C session briefing / doctor-merge — a general session-start health surface is out; the inbox is a distinct push channel, not a doctor front end. (from D-1)
- **O-5**: Message-authoring tooling — the distributor appends a schema-valid entry via the release runbook; no authoring UI/tooling in v1. (from D-7)
- **O-6**: Hosts/CLIs beyond those pyxis already wires — no new vendor surface stood up for the inbox; unwireable surfaces fall back to the manual instruction (D-8), not a new integration. (from D-7, D-8)

## Candidate glossary terms

- **T-1**: **channel bulletin** — a signed, append-only, closed-schema notice the distributor publishes on the release channel for operators to receive in their agentic CLI; the message unit of the inbox. Avoid: *notification* (the delivery act), *update channel* (a rollout cohort — `brainstorm:installer#O-7`, a different thing).
- **T-2**: **severity tier** — a bulletin's dedupe-governing class: `advisory` (shows once via the seen-ledger, muteable) vs `blocking` (re-fires every session until the condition clears, not muteable — the emergency lane). Avoid: *priority*, *level* (imply an ordinal scale, not the two-valued dedupe/mute contract).
- **T-3**: **seen-ledger** — the user-global record of which advisory bulletins an operator has already been shown, so each fires once per operator; lives beside the mute state and the channel snapshot in XDG state, never in repo config. Avoid: *read receipt* (implies a report back to the sender — explicitly out, O-1).

## Parked technical constraints

- **P-1**: Channel-carried message text flows into every fleet agent's context at session start — the schema must stay closed, rendering must come from pyxis-owned templates (messages supply typed fields only), and the message file wants the same signing treatment as a future revocation list. — context: F-4; stage-3 security section material.
- **P-2**: The inbox hook must be fail-silent and always exit 0 — a notification check may never block or wound a session; offline means cache-only, silent when empty (F-3's floor). — context: hook UX + FR-13.
- **P-3**: Per-CLI session-start hook-surface enumeration — for each recognized vendor CLI (`.claude/`, `.codex/`, `.agents/`/`.opencode/`), does it expose a session-start lifecycle hook and what is the fragment shape; the manual-fallback wording for those that don't. The stage-3 finalization of D-8's contract. — context: D-8, F-7, F-8.

## Gate

| Field | Value |
|---|---|
| **Verdict** | ready-for-prd |
| **Date** | 2026-07-23 |
| **Operator** | @mdarabi |
| **Covering source** | — |
| **Coverage check** | pass — all six inputs covered (D-1/D-6 problem · D-2 segments · D-3 jobs · D-6 goals · O-1..O-6 non-goals · D-7 appetite), zero blocking fog |
