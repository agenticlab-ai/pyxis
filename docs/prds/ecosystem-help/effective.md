# ecosystem-help — the pyxis front door (`/help`) — PRD *(effective view)*

---

**GENERATED — DO NOT EDIT.** Sole writer: `docgate fold`. As of: **000**. Content fingerprint: `sha256:1de724a6124e7f2d8d900210e5489f4fa71e6af8ba68cd68357915b32beec7e3`. Tool version (recorded, never hashed): `docgate/v1`.

---


| Field | Value |
|---|---|
| **In one line** | A distributable, stateless `/help` skill — the ecosystem's front door: on invocation it derives a catalog of what the installed pyxis surface offers (each capability with its purpose and when-to-use), routes a concretely described problem to its entry command, and offers a thin first-steps path for newcomers — all content derived from live surfaces, never hand-authored. |
| **Relevant-when** | planning, building, or reviewing the help/discovery surface; triaging defects in ecosystem orientation, problem routing, or the first-steps path; changing the surfaces help derives from (the pipeline map, skill descriptions, authoring-guide headers, wiki routing headers); designing install-time welcome behavior that points at help |
| **Status** | Draft — problem review pending |
| **Tier** | T2 — user-facing product surface for distribution, but reversible, read-only, and stateless: no data, auth, money, or migration surface. |
| **Owner** | @mdarabi |
| **Approver(s)** | @mdarabi (operator) |
| **Contributors** | — *(solo operator; gate tables record the introducing PR's review, rule 15)* |
| **Informed** | future engineers on onboarded repos |
| **Date** | 2026-07-23 created · 2026-07-23 last updated |
| **Appetite** | Small: one skill on the vendor-neutral agent surface (skill + authoring guide + vendor pointers), reading only surfaces that already exist. No new server, renderer, document class, or docgate change. Explicitly excluded: an enumeration API or generated index over the derivation surfaces — if reading them as-is proves fragile, that is an amendment finding against the surface, never license to grow this scope. Rollout/support burden ≈ zero (ships with the distributable surface). (brainstorm:ecosystem-help#D-6) |
| **Target release** | trigger-bound: passes the newcomer-proxy protocol (§3.2 M1) before the first outside distribution of the pyxis surface |
| **Tracking** | agenticlab-ai/pyxis#630 — the introducing PR |
| **Artifacts** | `brainstorm:ecosystem-help` — the gated stage-1 record (rides this PR) · `review:ecosystem-help-000` — the unmanned reviewer's report (rides this PR) |
| **Template version** | prd-template/v2 |

## 0. Read to rule — the human reviewer's section (~4 min)

*The body is the contract; this section only routes attention — on any conflict the body governs; machines ignore it.*

**Judge these**

| # | The call | Could have gone | If you disagree |
|---|---|---|---|
| 1 | **Derived, never authored** — help's catalog is computed from live surfaces at invocation; it holds no per-feature prose of its own | A curated catalog or onboarding curriculum (rejected framing B) | The founding thesis falls: help becomes authored content with a freshness problem the wiki rules exist to prevent → FR-4, NG-1, `brainstorm:ecosystem-help#D-1` |
| 2 | **One skill, not two** — `/help` contains the first-steps path; there is no separate onboarding skill | A `/help` + `/onboarding` pair | A second front door is invisible to exactly its audience, and "onboarding" collides with repo onboarding — veto reshapes §4.1 before any build → §4.1 option D, `#D-7` |
| 3 | **Once-ness belongs to the install event** — the skill is stateless and repeatable; the installer leaves the one-time welcome pointer | Skill-side "have I run before" state | §5.4's skip reverses into a state design with no good home (whose once — the repo's or each human's?) → FR-6, Q1, `#D-7` |
| 4 | **Agents are the anti-segment** — help promises nothing to automated readers | Agent-facing orientation as a first-class job | Scope widens into repo-intelligence ground (corpus VNEXT-6); the appetite no longer covers the feature → §2.2, NG-3, `#D-2` |
| 5 | **Success = the newcomer-proxy protocol** until outside users exist | An instrumented adoption metric | §3.2/§5.7 would need a telemetry surface this appetite excludes; the honest measure today is the recorded proxy run → M1, §6.7, `#D-4` |

**Sanity-check the numbers** — EVAL-1's bar: 100% correct entry naming on the canonical scenario set, zero fabricated commands on the negative set (§5.9); NFR-1's budget: derivation reads routing surfaces only, ≤ 2 output lines per capability (§5.6); M1's rubric: 3 of 3 proxy tasks unaided (§3.2).

**The author's open questions** — Q1: the welcome-pointer integration — the installer contract exists (`prd:installer` FR-15 charters the welcome work item) with implementation in flight; help's trigger point inherits its timing (§7.2, `brainstorm:ecosystem-help#G-1`/`#F-5`).

**Skip the rest** — structure → docgate; substance/coherence → the unmanned reviewer's report riding this PR; implementation defects → downstream verification gates.

## 1. Summary — the one-pager

- **Problem** — Someone who just received pyxis in their repository has no way to learn what the ecosystem offers, which capability fits their situation, or what to do first. The corpus explains everything — but only to a reader who already knows where to look.
- **For whom** — the human newcomer on a freshly pyxis-onboarded repo (primary); the returning user who forgets which skill fits the moment (secondary). Explicitly not for agents.
- **Why now** — pyxis is being prepared for distribution beyond its home repo; the first outside install is the moment discoverability starts gating adoption. An installed surface nobody can explore never becomes a used one.
- **Direction** — a single stateless `/help` skill, the ecosystem's front door: a **living index** that derives "what exists, when to use it" from the live surfaces (the pipeline map, skill descriptions, authoring-guide headers, wiki routing headers), answers "where does my problem enter?", and carries a thin curated first-steps path. The one-time onboarding moment stays with the installer (a durable welcome pointer at install), never with the skill. (brainstorm:ecosystem-help#D-1/#D-7)
- **Appetite** — as header: one skill reading existing surfaces; no API, index, server, or new document class.
- **Success** — at the newcomer-proxy checkpoint: a zero-context session on an onboarded repo, using `/help` alone, names what the ecosystem offers, identifies the right entry command for a real problem, and takes its first productive action (M1: 3 of 3).
- **Guardrail** — derivation purity: zero catalog content without a resolvable live source; adding a capability to the surface requires zero help-side edits (FR-4).
- **Top risks** — the derivation surfaces prove too thin or inconsistent to read as-is (R1); the first-steps path grows into a maintained curriculum (R2); "onboarding" term confusion misroutes users (R3).

## 2. Problem space

### 2.1 Problem statement

A newcomer receiving the distributable pyxis surface faces a fully documented but undiscoverable ecosystem: the knowledge exists (the pipeline map, the skills' own descriptions, the corpus), yet reaching it requires knowing the layout, the conventions, and the vocabulary first. The cost lands at the exact moment of first contact — the install-to-first-use gap — where confusion means abandonment, not a support ticket. Doing nothing means distribution produces installs, not users. (brainstorm:ecosystem-help#D-1, the operator's seed request verbatim in the record)

### 2.2 Users and segments

| Segment | Role and context | Domain exp. | Tech exp. | Key goals | Pain today | Evidence |
|---|---|---|---|---|---|---|
| **P1 (primary)** | the human newcomer — engineer or operator on a repo that just received the pyxis surface | novice | journeyman | grasp what the ecosystem offers; find the right entry for their problem; act | no map they know to open; grep-and-guess or ask a human | brainstorm:ecosystem-help#D-2 |
| P2 (secondary) | the returning user — knows pyxis, forgets which skill fits the current moment | journeyman | journeyman | re-find the right command quickly | re-reading the pipeline map or the skills list by hand | brainstorm:ecosystem-help#D-2 |
| **Anti-segment** | agents / automated readers — they navigate by routing headers and the pipeline map directly; smarter agent orientation is repo-intelligence ground (corpus VNEXT-6) | — | — | — | — | brainstorm:ecosystem-help#D-2 |

### 2.3 Jobs and desired outcomes

| # | When… (situation) | I want to… (motivation) | So I can… (outcome) | Today's workaround | Evidence |
|---|---|---|---|---|---|
| J1 | I just got the pyxis surface on my repo and don't know what's in it | see what exists, one line each, with when-to-use | grasp the ecosystem without reading the corpus | read AGENTS.md + the pipeline map by hand, or ask a human | brainstorm:ecosystem-help#D-3 |
| J2 | I have a concrete idea, problem, or task and don't know which command or stage it enters at | ask where my problem enters and get the right skill with a why | start correctly instead of guessing | the pipeline map's entry section, if I know it exists | brainstorm:ecosystem-help#D-3 |
| J3 | I'm brand new and want to know what to literally do first | get a short first-steps path | reach my first productive action without a wiki crawl | none — the job goes undone for a true newcomer | brainstorm:ecosystem-help#D-3 |

### 2.4 Evidence

| Claim | Source (research, data, tickets, interviews) | Confidence |
|---|---|---|
| The distributable surface ships a static stage map but no interactive discovery capability | the pipeline map's own header ("a repo that installs pyxis gets the skills *and* the map") (brainstorm:ecosystem-help#F-1) | High |
| The welcome-path requirement (install + first design doc = productive, no wiki crawl) exists in the baseline and is unmet in pyxis | corpus SCAF-5 · the PRD index's implemented-in-pyxis status (brainstorm:ecosystem-help#F-2) | High |
| The chartered reading room covers corpus *documents*, not ecosystem *capability* discovery — adjacent, not covering | prd:workbench-ui §1 (brainstorm:ecosystem-help#F-3) | High |
| Newcomers need an interactive front door (demand) | the operator's seed request; no outside-user data yet | Med |

### 2.5 The journey today

| Phase | User does | User feels / pain | Frontstage (what user sees) | Backstage (systems, people, processes) |
|---|---|---|---|---|
| discover | opens the repo after install; finds the installed pyxis system tree and the agent manual | curiosity → overwhelm; no obvious start | a directory tree and long documents | the full corpus, built for those who know it |
| attempt | greps, opens files, maybe finds the pipeline map | unsure what's authoritative or current | raw markdown | routing headers exist but nothing surfaces them |
| resolve | asks a human, or abandons | dependence or drop-off | — | the operator answers by hand — a factory gap under the inversion test |

### 2.6 Opportunity and why now

- **Size** — every distributed install of the pyxis surface × the first hours of every new user on it; plus every returning-user "which skill?" moment.
- **Alternatives users have** — read the corpus by hand; ask whoever installed it; give up.
- **Why us / strategic fit** — the routing surfaces (headers, descriptions, the map) already exist by convention; pyxis is uniquely placed to derive discovery from them instead of authoring it.
- **Why now** — distribution is the stated next act; discoverability gates whether installs become use (brainstorm:ecosystem-help#D-4).
- **Cost of doing nothing** — installs that never become users; the operator remains the human help surface.

**Decision:** Problem gate.

| Reviewer | Role | Date | Verdict |
|---|---|---|---|
| @mdarabi | operator | pending | pending — via the introducing PR's operator review |

## 3. Goals, metrics, and guardrails

### 3.1 Goals as falsifiable hypotheses

| # | We believe that… | Will result in… | Measured by… |
|---|---|---|---|
| G1 | a derived, always-current front door (`/help`) for the newcomer on an onboarded repo | newcomers reaching their first productive action unaided | M1 |
| G2 *(qualitative)* | the same front door serving the returning user's "which skill?" moment | `/help` invocations replacing operator questions | qual: observed use during dogfood; no instrumentation |

### 3.2 Goals → signals → metrics

| Metric | Goal | Signal | Baseline | Target (at checkpoint) | Feeds from |
|---|---|---|---|---|---|
| M1 (primary) | G1 | the newcomer-proxy protocol: a fresh session (or human on a fresh machine) with zero pyxis context on an onboarded repo answers (a) "what does this factory offer?", (b) one real problem's entry command, (c) "what do I do first?" — via `/help` alone | fails today (no capability) | 3 of 3, unaided, at the checkpoint: before the first outside distribution; re-run at the first outside install | measurement plan §6.7 (no telemetry — §5.7 skip) |

### 3.3 Guardrails and decision rule

| Guardrail metric | Threshold | Action if breached |
|---|---|---|
| Derivation purity | zero catalog entries without a resolvable live source (no fabricated capabilities, commands, or stages) | fix the skill or the surface before any wider distribution |
| Output discipline (AP-6) | NFR-1's budget holds: routing-surface reads only; ≤ 2 output lines per capability in the catalog answer | trim the skill; never widen the budget silently |

**Ship rule (pre-registered):** M1 = 3/3 and both guardrails clean → ship with the distributable surface; any fabricated entry → fix before ship, no exceptions; M1 < 3/3 → iterate the skill (or file the surface defect) and re-run the protocol.

### 3.4 Non-goals and waiting room

| ID | Non-goal | Why refused |
|---|---|---|
| NG-1 | A hand-authored feature catalog inside the skill | a maintained duplicate list is the drift vector the living-index destination exists to kill (brainstorm:ecosystem-help#D-1/#D-5) |
| NG-2 | Stateful pipeline-position advice ("what's in flight, what awaits me") | chartered home is the reading room's progress view (prd:workbench-ui); see WR-1 (brainstorm:ecosystem-help#D-5) |
| NG-3 | Agent-facing promises | agents navigate routing surfaces directly; smarter agent orientation is repo-intelligence ground, corpus VNEXT-6 (brainstorm:ecosystem-help#D-2/#D-5) |
| NG-4 | Restating corpus content | help points at what exists and when to use it; normative text stays in its governing documents — the wiki's pointer-heavy rule applied to a skill (brainstorm:ecosystem-help#D-5) |
| NG-5 | Skill-side once-state or any install-time execution claim | the one-time onboarding moment is the installer's (welcome-pointer pattern); the skill is stateless and repeatable by design (brainstorm:ecosystem-help#D-7) |

| ID | Waiting room | Revisit when / intended version |
|---|---|---|
| WR-1 | The situational advisor ("given where this repo is, the next command is X") | when the reading room's progress derivation exists to reuse, or when returning-user friction shows up in real use (brainstorm:ecosystem-help#D-5) |

## 4. Solution space

### 4.1 Approach and alternatives considered

The chosen shape: a single stateless skill that, on invocation, reads the live routing surfaces — the pipeline map, the skills' own descriptions, the authoring-guide headers, the wiki entries' routing headers — and renders three answers from them: the catalog (J1), the entry route for a described problem (J2), and a thin curated first-steps path (J3). The only authored content is the first-steps framing; everything catalog-shaped is derived. (brainstorm:ecosystem-help#D-1)

| Option | Sketch | Why not chosen |
|---|---|---|
| A (chosen) | living index + thin first-steps layer, derived at invocation | — |
| B | curated onboarding curriculum (guided tour to first shipped feature) | authored content rots unless something enforces freshness — the exact class the wiki rules distrust (brainstorm:ecosystem-help#D-1) |
| C | situational advisor reading the repo's pipeline position | needs corpus-state derivation; serves returning users, not day-1 discovery; deferred to WR-1 (brainstorm:ecosystem-help#D-1/#D-5) |
| D | two skills: `/help` + a separate onboarding skill | a second front door is invisible to exactly its audience; "onboarding" collides with repo onboarding; skills can't self-fire so once-ness isn't solved by splitting (brainstorm:ecosystem-help#D-7) |
| Do nothing | corpus-only discovery | distribution produces installs, not users (§2.6) |

### 4.2 Scope perimeter

- **In (plan of record)** — catalog derivation from live surfaces; problem→entry routing; the first-steps path; vendor-neutral placement with pointers for every supported CLI; a directly invokable first-steps entry (the installer's future pointer target).
- **Future (believed next, not committed)** — the situational advisor (WR-1); the install-time welcome pointer itself (chartered as prd:installer FR-15's welcome work item — integration per Q1).
- **Out** — mirrors §3.4. Authority note: help's derived output is descriptive — on any conflict, the surfaces it derives from govern, and the divergence is a help defect (the wiki's authority rule applied to a skill).

### 4.3 Experience: flows and states

**Flow F1 — orient (serves J1).** Invoke `/help` bare → the catalog: capabilities grouped by pipeline position (entry stages, cross-cutting, tooling), each with its one-line purpose and when-to-use, closing with the pointer to the first-steps path and the pipeline map. **Degraded**: a missing surface is named, never silently skipped (UB-1).

**Flow F2 — route (serves J2).** Invoke `/help <described problem>` → the entry command for it, the reason (which stage owns that class of problem), and what to expect next. **Unroutable**: a problem no pyxis entry covers gets an honest "not a pyxis entry" plus the map pointer (UB-2) — never an invented command.

**Flow F3 — first steps (serves J3).** Invoke the first-steps entry (or ask "where do I start?") → a short path: what pyxis is in two sentences, the one thing to try first with their real problem, and where the map lives. Thin by contract — it routes into F1/F2, it does not teach the corpus (NG-4).

- **Accessibility / localization** — terminal/chat text, English-only, inherits the host CLI's surface; no separate UI. No further state design: the skill is conversational and stateless (NG-5).

### 4.4 Scenarios

1. **The new team.** A team installs pyxis on their service repo. An engineer runs `/help`, sees the catalog, recognizes their half-formed caching idea as "a raw idea whose decisions aren't settled," and is routed to the brainstorm entry — first productive action inside the first session.
2. **The returning user.** Weeks later the same engineer, holding an approved PRD, asks `/help` "PRD's approved — now what?" and is routed to the design-brainstorm entry with the why. No corpus re-read.
3. **The miss.** Someone asks `/help` to fix their SQL query. Help says that's not a pyxis entry point, shows where real entries are, and stops — no fabricated capability (UB-2).

**Decision:** Solution gate — zero TBDs pass this gate.

| Reviewer | Role | Date | Verdict |
|---|---|---|---|
| @mdarabi | operator | pending | pending — via the introducing PR's operator review |

## 5. System requirements

### 5.1 Functional requirements

| ID | Requirement (EARS) | Rationale | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|
| FR-1 | When invoked without a specific problem, the help skill shall present the ecosystem catalog: every skill on the installed surface and every pipeline stage from the shipped map, each with its one-line purpose, when-to-use, and the map's built-status, derived at invocation from the live surfaces. | J1 — orient without reading the corpus | Must | low | every skill present on the installed surface appears exactly once; every stage row of the shipped map appears carrying its status; zero entries lack a resolvable live source | Demo | G1, J1, F1 |
| FR-2 | When the user describes a concrete problem or goal, the help skill shall name the entry command or stage for it and the reason, consistent with the pipeline map's entry rules. | J2 — route instead of guess | Must | low | 100% correct entry naming on the canonical scenario set; zero fabricated commands (EVAL-1) | Eval | G1, J2, F2 |
| FR-3 | When the user signals they are new or asks where to start, the help skill shall present the first-steps path: a two-sentence orientation plus the concrete first action for their situation. | J3 — SCAF-5's no-wiki-crawl promise | Must | low | the newcomer-proxy reaches a recommended first action in a single invocation | Demo | G1, J3, F3 |
| FR-4 | The help skill shall derive all catalog content at invocation time from live surfaces, holding no authored per-capability descriptions of its own. | the founding thesis — drift-proof by construction | Must | med | adding or removing a skill on the surface changes help's catalog with zero help-side edits (verified by adding a throwaway skill in a sandbox copy) | Test | G1, NG-1 |
| FR-5 | Where the pyxis agent surface is installed in a repository, the help skill shall be invocable from every supported agentic CLI via the vendor pointer directories. | vendor neutrality (AP-25) | Must | low | the skill resolves through each vendor pointer present in the repo | Inspection | J1 |
| FR-6 | The help skill shall expose its first-steps path as a directly invokable entry, so an install-time welcome pointer can reference it without interactive navigation. | the installer's future pointer target (Q1) | Should | low | the documented invocation lands on the first-steps path directly | Demo | J3 |

**Non-regression (NR-n)**

| ID | Existing behavior protected | Why protected | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|
| NR-1 | Existing skills, authoring guides, and the pipeline map's existing content are unchanged by this feature's landing | help composes the surfaces; it never edits them | low | the implementation diff outside the new skill's own files is limited to vendor pointers, an additive pipeline-map row, and the living-rollup updates the shipped-capability convention requires | Inspection | F1 |

### 5.2 Unwanted behavior and failure handling

| ID | If… (trigger) | Then the system shall… | User sees | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|---|
| UB-1 | If a derivation surface is missing or unreadable (e.g. no wiki in a distributed repo) | present what the remaining surfaces yield and name the absent surface — never fabricate entries for it | the catalog plus an explicit "not present on this install" note | Must | low | with a surface removed in a sandbox copy, output contains zero entries derived from it and names the absence | Test | F1, FR-1 |
| UB-2 | If the described problem matches no pyxis entry point | say so and point at the pipeline map — never invent a command or stage | an honest "not a pyxis entry" plus where real entries are | Must | med | zero fabricated commands or stages across the negative scenario set (EVAL-1) | Eval | F2, FR-2 |
| UB-3 | If a cataloged or routed entry's stage has no invocable mechanism on the installed surface (an unbuilt stage on the shipped map) | present the stage with its map status — never recommend a command for it | the stage named with its status, plus the nearest built entry point | Must | med | across scenarios targeting unbuilt stages, zero commands recommended for them and the status stated every time (EVAL-1) | Eval | F1, F2, FR-1, FR-2 |

### 5.3 Permissions and abuse cases

Skipped: read-only local skill; no new surface, role, or privilege — nothing to permission and no abuse vector beyond what the host CLI already governs.

### 5.4 Data

Skipped: the skill is stateless by decision (NG-5, brainstorm:ecosystem-help#D-7) — no state created, read state is the repo's existing files; the one-time install state belongs to the future installer (Q1).

### 5.5 Interfaces and integrations

Skipped: no system boundary crossed — the skill reads repository files through the host CLI's ordinary file access.

### 5.6 Non-functional requirements

| ID | Area | Requirement | Fit criterion | Verify |
|---|---|---|---|---|
| NFR-1 | Cost / token-consciousness (AP-6) | The help skill shall derive its catalog from routing surfaces only (headers, descriptions, the pipeline map) — never loading full document bodies — and shall render the catalog compactly. | derivation reads no full corpus document beyond the pipeline map; the catalog answer spends ≤ 2 output lines per capability | Inspection |

*Other NFR areas (performance, availability, security, privacy, accessibility, localization, operability, maintainability) reviewed and dropped: a local, read-only, stateless text skill inherits all of them from the host CLI; maintainability is carried by FR-4's fit criterion.*

### 5.7 Instrumentation — the tracking plan

Skipped: no telemetry surface exists for skills and this appetite adds none; M1 is measured by the recorded newcomer-proxy protocol (§3.2, §6.7), not events.

### 5.8 Definition of Done

| ID | Done criterion | Source | Verify | Required evidence | Blocking? |
|---|---|---|---|---|---|
| DOD-1 | All Must FRs pass their fit criteria | FR-1..5 | Demo/Test per row | recorded run transcript riding the implementation PR | yes |
| DOD-2 | Failure behavior holds: missing surfaces named, unroutable problems refused, unbuilt stages never recommended as commands | UB-1, UB-2, UB-3 | Test/Eval | sandbox-run results in the implementation PR | yes |
| DOD-3 | The routing eval clears its bar | EVAL-1 | Eval | the scenario checklist with results (manual harness until eval machinery exists) | yes |
| DOD-4 | The newcomer-proxy protocol passes | M1 | Human judgment — @mdarabi; rubric: 3 of 3 tasks completed via help alone, unaided | the recorded proxy session | yes |
| DOD-5 | Non-regression holds | NR-1 | Inspection | the implementation PR's diff | yes |

### 5.9 AI behavior and evals

| ID | Behavior | Eval set / harness | Threshold | Negative examples |
|---|---|---|---|---|
| EVAL-1 | Problem→entry routing (FR-2), refusal honesty (UB-2), and built-status honesty (UB-3) | the canonical scenario set derived from the pipeline map's entry rules (one scenario per entry class, plus cross-cutting skills), a negative set of non-pyxis asks, and an unbuilt-stage set; run as a manual scenario checklist until eval machinery exists | 100% correct entry naming on the canonical set; zero fabricated commands or stages on any set; zero commands recommended for unbuilt stages | routing a raw idea straight to PRD authoring; inventing a command for "fix my SQL"; naming a stage that doesn't exist on the installed surface; recommending a command for a stage the map marks unbuilt |

## 6. Delivery

### 6.1 Phasing and milestones

| Phase | Purpose | Scope | Exit criterion |
|---|---|---|---|
| Land | build the skill per the approved design | §4.2 "In" | DOD-1..3, DOD-5 green |
| Dogfood | prove the newcomer path | fresh-session proxy runs on this repo | DOD-4 green (M1 = 3/3) |
| Distribute | real exposure | ships with the surface at the first outside install | §6.7 review trigger fires |

### 6.2 Dependencies

| Dependency | Type (team / system / vendor / approval) | Needed by | Status | Owner |
|---|---|---|---|---|
| the installer's welcome path (prd:installer FR-15 — the chartered welcome-pointer home) | system — sibling feature | Distribute (pointer only; help works without it) | contract merged; implementation in flight; not blocking (Q1) | operator |

### 6.3 Risks, assumptions, and rabbit holes

| # | Risk / assumption / rabbit hole | Likelihood | Impact | Mitigation or test | Owner |
|---|---|---|---|---|---|
| R1 | The derivation surfaces are too thin or inconsistent to read as-is — the rabbit hole is "just build an index" | M | M | each derivation gap is filed as a defect against the surface (its own PR/amendment); the appetite explicitly refuses the index (header, brainstorm:ecosystem-help#D-6) | operator |
| R2 | The first-steps path grows into a maintained curriculum | M | M | NG-1/NG-4 bound it; review discipline on help changes; FR-4's fit criterion keeps the catalog derived | operator |
| R3 | "Onboarding" term confusion — users expect help to onboard the repo | L | M | the glossary term landed with this PR reserves "onboarding" for repo onboarding; help's first-steps copy says "welcome path", never "onboarding" | operator |
| A1 | Assumption: newcomer demand behaves as the operator's seed request predicts (Med-confidence, §2.4) | — | M | tested by the proxy protocol now and the first outside install later (§6.7) | operator |

### 6.4 Release criteria

| Lens | Criterion | Evaluated by |
|---|---|---|
| Functionality | all Must FRs pass their fit criteria | DOD-1 evidence |
| Usability | the newcomer-proxy completes 3/3 unaided | DOD-4 protocol |
| Reliability | UB-1/UB-2 behavior proven in sandbox runs | DOD-2 evidence |
| Supportability | the skill's authoring guide documents the degraded modes and the derivation surfaces | Inspection |

### 6.5 Rollout and reversibility

- **Flag / kill switch** — n/a: a local skill shipped with the surface; reversal is reverting the PR (no runtime state to unwind — NG-5).
- **Stages** — n/a: no staged exposure surface exists; the honest stages are §6.1's phases (land → dogfood → distribute).

### 6.6 Launch readiness

| Area | Item | Owner | Status |
|---|---|---|---|
| Docs | the pipeline map gains its additive cross-cutting row at implementation landing (NR-1's carve-out) | operator | ☐ |
| Docs | glossary term ("onboarding" reservation) rides this introducing PR | operator | ☐ |

*Other areas (support, enablement, legal, pricing, marketing, platform, localization): n/a — solo-operator, local, free, English-only tooling.*

### 6.7 Post-launch review

- **Review trigger** — the first outside install of the pyxis surface completes its first week, or three fresh-machine proxy runs accumulate — whichever comes first. · **Owner** — @mdarabi
- **Verdict recorded** — adopt / iterate / kill, with the M1 protocol evidence — on the header's Tracking ref.

## 7. Alignment

### 7.1 FAQ

**Customer**

- **Q:** How do I find out what pyxis can do for me? — **A:** Run `/help`. It shows everything installed in your repo, what each thing is for, and where your problem enters. Ask it with your actual problem and it points at the right command.
- **Q:** Do I have to read the docs first? — **A:** No. Help's whole job is that you don't start with the docs — it routes you to the one thing worth reading or running next.

**Internal**

- **Q:** Why not a static document — the pipeline map already exists? — **A:** It ships and stays governing; help is the interactive front door *over* it. The map answers those who open it; help answers those who don't know it exists (brainstorm:ecosystem-help#F-1).
- **Q:** What stops help from rotting like every hand-written catalog? — **A:** It holds no catalog. Content derives from live surfaces at invocation (FR-4); a stale entry means a stale *surface*, which is where the fix lands.
- **Q:** Why one skill instead of `/help` + `/onboarding`? — **A:** A second door is invisible to exactly its audience; "onboarding" already means repo onboarding here; and skills can't self-fire, so once-ness isn't solved by splitting anyway (brainstorm:ecosystem-help#D-7).

### 7.2 Open questions

| # | Question | Owner | Needed by | Status |
|---|---|---|---|---|
| Q1 | When and how does the installer's welcome work item (prd:installer FR-15) point at help's first-steps entry? The installer contract is merged and its implementation in flight; the integration inherits its timing. Conveyed to the technical design; blocks no gate of this PRD. (brainstorm:ecosystem-help#G-1, #F-5) | operator | stage-3 design (context), the installer's welcome-path landing (resolution) | open |

### 7.3 Decision log

| Date | Decision | Alternatives considered | Why | Who |
|---|---|---|---|---|
| 2026-07-23 | Destination: living index + thin first-steps layer; advisor deferred | curated curriculum (B); situational advisor (C) | derivation kills drift by construction; thin layer covers day-1; C earns its own moment (brainstorm:ecosystem-help#D-1) | operator |
| 2026-07-23 | Segments: human newcomer primary, returning user secondary, agents anti-segment | agent-facing help | agents navigate routing surfaces directly; VNEXT-6 owns smarter agent orientation (brainstorm:ecosystem-help#D-2) | operator |
| 2026-07-23 | Jobs: orient, route, first-steps — and no more | stateful queries; corpus teaching | the three fall out of the destination; the rest is owned elsewhere (brainstorm:ecosystem-help#D-3) | operator |
| 2026-07-23 | Success: the newcomer-proxy protocol at an exposure checkpoint | an instrumented adoption metric | no telemetry surface exists; honest proxy over invented metric (brainstorm:ecosystem-help#D-4) | operator |
| 2026-07-23 | Non-goals NG-1..4 + the advisor to the waiting room | keeping boundaries implicit | each refusal makes an earlier ruling explicit and enforceable (brainstorm:ecosystem-help#D-5) | operator |
| 2026-07-23 | Appetite: small — one skill, existing surfaces only, no index/API | an enumeration API over the surfaces | fragility findings route to the surface, not into scope growth (brainstorm:ecosystem-help#D-6) | operator |
| 2026-07-23 | One skill; once-ness assigned to the install event (welcome-pointer pattern); skill stateless | two skills; skill-side once-state | the second door is invisible to its audience; once-state has no good home; repeatability serves both segments (brainstorm:ecosystem-help#D-7) | operator |

### 7.4 Changelog

| Date | Change | Author |
|---|---|---|
| 2026-07-23 | 000-base created from the gated brainstorm record | factory agent (operator-directed session) |
| 2026-07-23 | Round-1 review findings applied — NR-1 fit widened to the rollup convention, FR-1 scope aligned with its fit, UB-3 added for unbuilt-stage honesty (review:ecosystem-help-000#FND-1..3) | factory agent (review disposition) |
| 2026-07-23 | Upstream refresh at PR assembly: prd:installer discovered merged (FR-15 = the chartered welcome-pointer home) — Q1, §6.2, §4.2, §0 informative rows sharpened; no requirement semantics changed (brainstorm:ecosystem-help#F-5) | factory agent (operator-directed session) |
| 2026-07-23 | CI purity fix: §2.5's journey row reworded to drop a system-footprint path literal (check-content-paths, FR-31); no semantics changed | factory agent (operator-directed session) |

## 8. Contract readiness — the executability gate

| Check | Pass? | Notes |
|---|---|---|
| Problem basis accepted — §2 problem gate signed (T2) | ☐ | pending — signs via the introducing PR's operator review (rule 15) |
| Zero solution-blocking TBDs or open questions (§7.2) | ☑ | Q1 is context for downstream stages, blocks no gate here |
| Every Must FR has a fit criterion and a verification method (§5.1) | ☑ | |
| Every requirement row carries a risk tier; high/critical rows have a declared safety plan | ☑ | vacuously — no high/critical rows; highest is medium (FR-4, UB-2, UB-3) |
| Non-goals drawn (§3.4); non-regression rows named (§5.1) | ☑ | NG-1..5, WR-1; NR-1 |
| Every primary job (§2.3) is covered by a requirement, a non-goal, or a waiting-room entry | ☑ | J1→FR-1; J2→FR-2/UB-2; J3→FR-3/FR-6 |
| Every key flow (§4.3) has functional, failure, and non-regression coverage (§5.1–5.2) | ☑ | F1→FR-1/UB-1; F2→FR-2/UB-2; F3→FR-3/FR-6; NR-1 spans all |
| Every primary metric (§3.2) has instrumentation (§5.7) or an explicit measurement plan | ☑ | M1 → the proxy protocol (§3.2, §6.7); §5.7 skipped with reason |
| Every blocking DoD row traces to a requirement, guardrail, release criterion, or human-judgment rubric (§5.8) | ☑ | |
| Risks and assumptions have owners (§6.3) | ☑ | |
| Human-judgment criteria name an owner and a rubric (§5.8) | ☑ | DOD-4: @mdarabi, 3-of-3 rubric |
| DoD complete: every blocking row sourced and evidenced; every n/a explained (§5.8) | ☑ | no n/a rows |

## Superseded & retired

None — no unit has been superseded or retired; the ledger is its base.
