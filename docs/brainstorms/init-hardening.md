# Init hardening — brainstorm record

| Field | Value |
|---|---|
| **In one line** | Hardening `pyxis init`'s first contact: content-safe admission, a preflight gate before the first question, honest init↔adopt routing, truthful knobs (`ci_mode`, the code gate), and a question surface a stranger can answer — closing the G1–G25 register from the 2026-07-28 init-flow audit. |
| **Relevant-when** | brainstorming or contracting changes to `pyxis init`'s admission, prerequisite/preflight checking, interactive prompts, preview/plan, mid-pipeline failure recovery, or routing between init and adopt; checking prior rulings on the five knobs, `ci_mode` semantics, install-time collision policy, or seeding the verification manifest (`.pyxis/codegate.json`) at install |
| **Idea** | "init hardening — close the G1–G25 gap register from the init-flow audit (admission/content safety, preflight gate, init↔adopt routing, knob semantics, question surface)" |
| **Operator** | @mdarabi |
| **Date(s)** | 2026-07-28 |
| **Status** | gated: ready-for-prd *(→ freezes at the introducing PR's merge; until then operator review may add or supersede review-anchored decisions)* |
| **Tracking** | — |
| **Feeds** | `docs/prds/init-hardening/` |
| **Prior records** | `brainstorm:installer`, `brainstorm:existing-repo-onboarding` |
| **Template version** | brainstorm-record/v1 |

## Orientation

| Source | Why loaded |
|---|---|
| `docs/prds/installer/effective.md` (origin/main) | the merged basis this feature amends — FR-3/FR-4 (five knobs), FR-19 (preview), UB-1 (admission), UB-7 (prerequisites), UB-10 (identity), G4 (stranger-shaped refusals), R6 (a sixth knob is an amendment, not a redesign) |
| `docs/tech-design/installer-v1.md` | Q-1 parked the prompt surface as "implementation — flags are the contract; prompts are UX"; §3.5's stage order is where the late checks live |
| `docs/prds/existing-repo-onboarding/effective.md` (origin/main) | the shipped negotiation counterpart — FR-2 (five-class plan), FR-4 (hook chaining), UB-1 (adopt on an empty tree routes to init), UB-5 (non-git refusal), NR-1 (init non-regression guardrail) |
| `docs/brainstorms/existing-repo-onboarding.md` | D-1 (negotiation framing), D-4 (collision residue), D-10 (anti-segments), O-1 (stack profiles out) — rulings this session must not silently relitigate |
| `docs/brainstorms/installer.md` | D-6 (the asked-knob set: four, argus fifth via D-11; "hard dependencies don't get opt-outs; paths are config, not questions") — the prior knob rulings any `ci_mode`/stack question touches |
| `docs/tech-design/brainstorm-record-model.md` | this record's contract |
| `docs/wiki/glossary.md` (origin/main) | term check — `onboarding` is settled as *repo* onboarding; no `preflight`/`admission` terms exist yet |
| live init-flow audit (2026-07-28 session) | the G1–G25 register: every row verified against the v0.1.2 release binary (live installs) and origin/main@a2787d6 (source) |

**Overlaps surfaced:**

- `prd:existing-repo-onboarding#UB-1` · adopt→init routing exists ("adopt on an empty tree routes to init"). The **reverse edge is unowned**: init's non-empty refusal names no verb and still reads "adoption is the deferred follow-up WR-1" — stale in the very binary that ships `adopt` (F-11). Closing it is amendment ground, not new product.
- `prd:existing-repo-onboarding#UB-5` + installer `UB-1` as built · adopt refuses non-git targets; init admits a non-git directory *with content* (F-1) — the destructive shape falls between the two contracts; an owner must be ruled.
- `prd:existing-repo-onboarding#FR-2` (with `prd:installer#NG-3`) · the five-class collision plan is **merged capability** — init's missing pre-write collision diff (F-2) has an existing mechanism to reuse rather than a policy to invent.
- `prd:installer#FR-19` · preview exists but is contracted only as "report and perform none" — it plans neither wiring nor, on a cold cache, the write set (F-5). Strengthening it amends FR-19 rather than duplicating it.
- `brainstorm:installer#D-6` · the asked-knob set was a deliberate operator ruling. Any `ci_mode` re-ruling (F-9) or sixth stack knob (F-10) supersedes that ruling explicitly — `prd:installer` R6 pre-authorizes the amendment path.
- `prd:implementation-verification#FR-5` (via `brainstorm:installer#G-1`) · the verification manifest (as built: `.pyxis/codegate.json`) belongs to that contract, with a re-homing obligation already on record — the code-gate-at-init question crosses feature lines and needs a scoping ruling here.
- `prd:installer#G4` · "stranger-shaped defaults, docs, and refusals — an external adopter completes an install unaided" — the register is direct evidence G4 is unmet; this feature is its enforcement arm.
- **Not covered anywhere found**: a pre-question preflight gate, content-based admission, cross-knob validation, interactive re-ask, recovery from mid-pipeline failure, prompt glosses. Clear new ground.

## Framings

- **A — The safe front door (plan-then-apply init).** Thesis: every register row traces to one missing subsystem — a **gate that runs before the first question**: environment preflight (gh auth/scopes when GitHub mode is implied, git/node floors, remote/slug, artifact reachability), content-based admission, a full write-set/collision diff, cross-knob validation, and a confirmable plan — making init plan→confirm→apply, the shape `adopt` already has. Implications: one design, mostly installer amendments; the question surface rides the same pass; a standalone readiness check (`pyxis preflight` or a real preview) falls out; routing to adopt falls out of admission. Opens: symmetric front doors; a machine-checkable "no mutation before a confirmed plan" invariant. Risk: pulls product decisions (`ci_mode`, a stack knob) along as passengers — they need their own rulings, not incidental resolution.
- **B — Init is adopt's empty case (unify the doors).** Thesis: v0.1.2 shipped a negotiation engine (inspect→plan→apply→undo); init is its degenerate case. Fold init onto the adoption machinery: one admission, one plan artifact, one collision policy, one preflight; `init` survives as the empty-tree route. Implications: the routing and inconsistency gaps become structurally impossible; but it relitigates a just-merged contract that deliberately kept the verbs distinct (onboarding D-1/D-6) and is guarded by NR-1 ("init does not regress"); the largest appetite. Opens: one mental model — "pyxis manages a tree". Risk: architecture churn on fresh code for gaps A closes more cheaply.
- **C — Truthful first contact (the product-surface framing).** Thesis: the register is one product failure — the first contact **lies**: an inert knob, a misnamed one, unstated prerequisites, a refusal citing a deferral that shipped, success reported over destroyed files, a welcome advertising a dead verify ladder. The feature is truthfulness: every question real and glossed, every prerequisite stated-and-checked, every outcome reported honestly, every dead end routed. Measured as installer G4 made testable. Implications: prioritizes the knob-semantics and code-gate rows; safety mechanics enter as supporting requirements. Risk: the data-loss rows deserve a hard mechanical guarantee, not a theme.

**Destination:** **A + C hybrid — the safe front door, held to truthful first contact.** The mechanism is A: a gate before the first question (environment preflight, content-based admission, write-set/collision diff, cross-knob validation) feeding a confirmable plan — init becomes plan→confirm→apply. C's truthfulness rows (knob honesty, routing, reporting, glosses) are binding requirements on that surface, measured as installer G4 made testable. B is declined (→ O-1). Operator adopted the recommendation. *(Mechanism narrowed by D-7: checks-then-go — front-loaded fail-fast checks and the empty/route rule replace plan→confirm→apply ceremony.)*

## Decisions

### D-1 — What is init hardening, as a feature?

- **Question**: Three framings — A (the safe front door: a pre-question gate + plan→confirm→apply), B (fold init onto the shipped adoption machinery), C (truthful first contact: the G4 product-surface framing). Which is the feature?
- **Answer**: **A + C hybrid** — the front-door gate is the mechanism; the truthfulness rows ride it as requirements; B is declined.
- **Reasoning**: A gives the data-loss rows a hard invariant ("no mutation before a confirmed plan") instead of a theme; C's honesty rows get a measurable home on that same surface (G4 made testable); B relitigates the just-merged onboarding contract (its D-1/D-6 chose a distinct verb; NR-1 guards init non-regression) for gaps A closes more cheaply. Recommendation adopted.
- **Anchor**: 2026-07-28 · turn 1
- **Status**: live

### D-2 — The vehicle: one new feature PRD

- **Question**: What carries the register to contract — a new feature PRD, an amendment train on `prd:installer` alone, or a two-feature split (safety now, knob truth later)?
- **Answer**: **One new feature PRD** at `docs/prds/init-hardening/`, fed by this record; installer amendments only where an existing row becomes false (FR-19, UB-1, the refusal text, `ci_mode`'s meaning); the `ci_mode` and code-gate rulings land in this session and ride the same effort.
- **Reasoning**: the front-door gate is a coherent new subsystem, not a correction; a pure amendment train smears one capability across ~10 single-amendment PRs (docgate FR-27 gates one amendment per PR); precedent — config-driven-portability and existing-repo-onboarding were new features that changed installer behavior. Recommendation adopted.
- **Anchor**: 2026-07-28 · turn 2
- **Status**: live

### D-3 — `ci_mode` becomes real; the key survives

- **Question**: `ci_mode` today changes nothing (F-9). Wire it keeping the key name, wire it under a renamed key (`docs_gate_mode`), or delete the knob?
- **Answer**: **Wire it, keep the name** — the docs-gate workflow is installed only when `actions|both`; hooks stay unconditional (the desk gate is a D-6 hard dependency); the prompt gloss states "where the docs gate runs", aligning with the generated AGENTS.md's already-honest wording.
- **Reasoning**: real semantics without a closed-schema key migration; deleting would force an Actions workflow onto offline/carina repos — the exact wrong behavior F-9 documents. Migration of already-installed repos parks to design (P-5). Recommendation adopted.
- **Anchor**: 2026-07-28 · turn 3
- **Status**: live

### D-4 — The code gate: first-use seeding, truth at init

- **Question**: How does a fresh operand get its code gate (`.pyxis/codegate.json`) — first-use seeding, a sixth init-time stack knob, or truth-only?
- **Answer**: **First-use seeding, truth now.** No sixth question. This feature contracts the truth surface at init — welcome/AGENTS.md state the docs-gate-vs-code-gate boundary and the manifest step; doctor advises when code files exist without a manifest. At first implement-time the stack is detected (`MarkerFiles` — files exist by then) and the manifest is seeded or instructed; verb content stays with `prd:implementation-verification` FR-5.
- **Reasoning**: init's canonical target is empty — nothing to detect, and a stack question asks about a changeable future, against `brainstorm:installer#D-6`'s "config, not questions"; first use has real files to detect against. Recommendation adopted.
- **Anchor**: 2026-07-28 · turn 4
- **Status**: live

### D-5 — Failure recovery: full rollback

- **Question**: What does a failed init leave behind — full rollback, a resumable pipeline, or a debris-aware refusal that names the cleanup?
- **Answer**: **Full rollback.** The invariant: a failed init leaves the target exactly as it found it, or fully installed — nothing between. Failure undoes the enumerated write set (and the `git init`, when init created the repository); refusals name the failed stage and reason.
- **Reasoning**: under plan-then-apply the write set is enumerated by construction, so staged undo is mechanical; `adopt`'s shipped `undo` is precedent; fail-closed on destructive paths is the house posture (AP-17/INV-6). Resume is stateful complexity for a seconds-scale pipeline; refusal-only still leaves manual cleanup (F-8's stranding, softened not solved). Recommendation adopted.
- **Anchor**: 2026-07-28 · turn 5
- **Status**: live

### D-6 — Confirmation gates interactive runs only

- **Question**: When does init stop for confirmation before applying the plan — interactive runs only, every run, or never?
- **Answer**: **Interactive only.** The resolved plan (knobs, write set, wiring operations) renders and one y/N gates mutation. Promptless runs (any flag, answers file, `--yes`) print the same plan and proceed — FR-4's zero-prompt contract stays intact; automation inspects without applying via the readiness/plan surface, not a prompt.
- **Reasoning**: preserves SCAF-1 CI parity unbroken while giving the interactive stranger — this feature's audience — the moment of judgment the plan exists to serve. Recommendation adopted.
- **Anchor**: 2026-07-28 · turn 6
- **Status**: superseded by D-7 — the confirmation gate presumed a rendered plan; D-7 removes plan ceremony entirely (checks pass ⇒ proceed)

### D-7 — Checks-then-go: no plan, no confirm, no readiness verb; the simple routing rule

- **Question**: Does the readiness gate become a standalone verb (`pyxis preflight`), and does init carry plan→confirm ceremony at all?
- **Answer**: **Neither** (operator redirect, their words): "run the check before starting the init action. if there is any issue, error out and stop." No plan artifact, no confirmation gate, no new verb — "do not mix the init with adopt/apply. the rule is simple, if the repo is empty, run init otherwise tell the user to run adopt/apply."
- **Reasoning**: the plan/confirm/verb apparatus would re-grow adopt's negotiation inside init, and empty-by-content admission makes it unnecessary — an empty target has nothing to collide with, so front-loaded checks + fail-fast named refusals deliver the same safety invariant with zero ceremony. Supersedes D-6; narrows D-1's mechanism to checks-then-go; declines the standalone readiness verb (O-3); fixes admission and routing in one rule (empty → init; anything else → the refusal names `adopt`/`apply`).
- **Anchor**: 2026-07-28 · turn 7
- **Status**: live

### D-8 — GitHub coordinate: init asks when it has nothing to parse

- **Question**: GitHub mode needs `owner/name` before the checks can pass (F-3/F-4: today it is parsed from `origin` six stages in). Require origin pre-set, ask for it, or drop `repo=create`?
- **Answer**: **Init asks.** In GitHub mode with no `origin` remote, init elicits `owner/name` (a conditional exchange, not a sixth knob; promptless runs supply it via flag/answers key), sets `origin` itself, and under `create` creates the repository there. With an origin present it is parsed as today and nothing is asked. Promptless with neither origin nor coordinate → named up-front refusal.
- **Reasoning**: smoothest brand-new-project flow — the operator's own first-run scenario — at the cost of one conditional exchange; closes F-4's create-from-nothing impossibility for real; checks-then-go still fail-fasts when automation omits the coordinate (FR-4 parity intact).
- **Anchor**: 2026-07-28 · turn 8
- **Status**: live

### D-9 — Segments: the first-contact installer; content-holders redirected

- **Question**: Who is the hardened init's primary user, and who is explicitly not served?
- **Answer**: **P1 = the first-contact installer** — anyone at an empty folder standing up a new factory repo, no pyxis vocabulary assumed (the operator this morning; the external adopter by design — installer P2 elevated to this feature's P1). CI/automation is served-secondary through promptless parity (FR-4). **Anti-segment: any repo with existing content** — adopt's P1; init redirects by name, never serves (D-7).
- **Reasoning**: every register failure is a first-contact failure — G4's "install unaided" is the unmet goal this segment choice makes primary; content-holding repos have a shipped door. Recommendation adopted.
- **Anchor**: 2026-07-28 · turn 9
- **Status**: live

### D-10 — Four jobs

- **Question**: Which jobs does the hardened init contract?
- **Answer**: **Four.** J1 — install ends green, or refuses *before any write* with the fix named. J2 — a failed install leaves the folder exactly as found; re-running just works. J3 — every question answerable without reading source; every answer changes real behavior. J4 — the wrong door walks you to the right one ("content here → `pyxis adopt`"; "no repo yet → `git init`, then adopt").
- **Reasoning**: each traces to verified audit facts (J1←F-3/F-7 · J2←F-8/D-5 · J3←F-9/F-12 · J4←F-11); routing stays a distinct job because the redirect is its own journey with its own acceptance. Recommendation adopted.
- **Anchor**: 2026-07-28 · turn 10
- **Status**: live

### D-11 — The bet's observable: the unaided-install trial

- **Question**: What observable proves the bet — a contracted fixture matrix replaying the audit failures, register-closure traceability, or an unaided-install trial?
- **Answer**: **The unaided-install trial.** The metric is a session, not a suite: someone who has never seen pyxis, fresh machine, empty folder → green factory, zero questions asked of the operator — recorded as a usage verbatim. E2e fixtures are added opportunistically, not contractually.
- **Reasoning**: operator chose the human observable over the mechanical one (the fixture-matrix recommendation declined as the gate). This is installer G4's own metric shape made primary ("usage verbatims once external adopters exist"); register closure is scope, not a metric.
- **Anchor**: 2026-07-28 · turn 11
- **Status**: live

### D-12 — Appetite: one contract, ~8–12 tasks

- **Question**: How much build before the cut-line — one contract (~8–12 tasks), a minimal ~4–6-task sweep, or two waves (safety core, then truth surface)?
- **Answer**: **One contract, ~8–12 tasks** — one PRD, one plan, one PR train, the scale existing-repo-onboarding landed at. Everything ruled here ships together; e2e fixtures ride opportunistically (D-11); the installer amendment train (FR-19, UB-1, `ci_mode`, refusal text) is its own task.
- **Reasoning**: the rulings interlock — empty-by-content admission is what keeps the checks and rollback simple — so wave-splitting doubles plan/review ceremony without reducing risk at this size. Recommendation adopted.
- **Anchor**: 2026-07-28 · turn 12
- **Status**: live

## Facts

- **F-1**: `admitTarget` admits any target lacking `.git/` without reading directory content — a non-git directory full of files proceeds straight to mutation — source: `internal/installer/helpers.go:84-89` (unchanged at origin/main@a2787d6); verified live on v0.1.2.
- **F-2**: on such a target init silently overwrote pre-existing `AGENTS.md`, `.gitignore`, `docs/observability/incidents.md`, and the design index — no warning, no backup, no git history to recover from — then reported "green ✓"; only a file squatting a *symlink* target refuses (`tail.go:295`), and that fires mid-converge, after `.gitignore` is already gone — verified live on v0.1.2.
- **F-3**: GitHub-mode wiring derives the repo slug solely from the `origin` remote; with none, the pre-action guard refuses at stage 7 of 9 — after git init, the config writes, converge, dependency install, and hooks — the operator's first real install failed exactly here — source: `internal/wire/execute.go:164`, `tail.go:440` (origin/main); live run 2026-07-28.
- **F-4**: `wiring repo=create` sources its slug from that same absent origin — creating the repo from a bare directory is impossible on every path — source: `internal/wire/wire.go` Table + the slug guard.
- **F-5**: `--preview` performs no wiring planning (no slug, no wire table) and on a cold cache reports the write set "computable only after that fetch" — the dry run is blind to the likeliest failures — source: `internal/installer/installer.go:386-403`; verified live.
- **F-6**: the gh preflight is `gh --version` only — authentication and scopes are never probed anywhere; failures classify post-hoc from stderr patterns, including GitHub's 404-on-admin-endpoints case — source: `wire/execute.go:226-241`, `wire/runner.go:83-115`.
- **F-7**: prerequisite checks run mid-pipeline — package-runtime probes at stage 5, gh at stage 7, gate-green at stage 9 — all after mutations begin; git presence/version and artifact-host reachability are never checked by init at all — source: `installer.go:236-254`, `tail.go:356-395`.
- **F-8**: a mid-pipeline failure strands the target: re-running init trips UB-1 on init's *own debris*; the only recovery is manual deletion — verified live, twice (including the operator's real first install).
- **F-9**: `ci_mode` has exactly one behavioral consumer — a converge-carried wire row that executes nothing — hooks and the docs-gate workflow install unconditionally (a `local` install still commits `.github/workflows/docs-gate.yml`); the knob's answer changes no installed behavior — source: `wire.go:181` (origin/main), `.pyxis/class-policy.yaml:41`; verified live.
- **F-10**: init establishes no code gate: no stack question, no `.pyxis/codegate.json` seeded; the verify ladder refuses ("manifest not found", `invoke.go:230`) on every fresh operand while the welcome text advertises `/implement-now`; python/node stack defaults ship empty verb sets (go's are real); upstream added `MarkerFiles` detection groundwork — source: `ecosystem/defaults.go` (origin/main); live operand inspection.
- **F-11**: `adopt`/`apply`/`undo` are shipped capability (v0.1.2 binary; origin/main): adopt routes an empty tree to init and refuses non-git targets; init's non-empty refusal still says "adoption is the deferred follow-up WR-1" and never names `adopt` — source: `pyxis help` (v0.1.2), `prd:existing-repo-onboarding#UB-1`/`#UB-5`, `helpers.go:71-72` (unchanged upstream).
- **F-12**: the interactive prompts are byte-identical to the flag help strings; one out-of-enum answer aborts the whole run discarding every prior answer; `cli_surfaces` is the only question that hides its legal values — source: `helpers.go:220-336` ≡ `cmd/pyxis/main.go:187-192`; `helpers.go:280,306`.
- **F-13**: `installer-v1` Q-1 parked the interactive prompt surface as "implementation — flags are the contract; prompts are UX" — still open — source: `docs/tech-design/installer-v1.md:180`.
- **F-14**: onboarding FR-2 contracts the five-class adoption plan (add / chain / collide-with-resolution / manual-instruction / never-touch) with apply-on-approval — merged collision machinery honoring `prd:installer#NG-3` — source: `docs/prds/existing-repo-onboarding/effective.md`.
- **F-15**: installer G4 declares "stranger-shaped defaults, docs, and refusals — an external adopter completes an install unaided" as a qualitative goal; R6 pre-authorizes a sixth knob as "an amendment, not a redesign" — source: `docs/prds/installer/effective.md`.
- **F-16**: the consolidated G1–G25 register (question surface G1–G6 · preflight G7–G16 · admission & content safety G17–G21 · routing G22–G23 · knob semantics & code gate G24–G25; each row with evidence, severity, candidate solution) is the audit's output — source: init-flow audit session 2026-07-28, verified against the v0.1.2 release binary + origin/main@a2787d6.

## Fog

- **G-1** (non-blocking — blocks: stage-3 design / the fed PRD carries it as an open question): which surface mechanically owns first-use code-gate seeding — `implement-now`'s preflight, a doctor remedy, or `plan-new`'s projection — crosses `prd:implement-now` and `prd:implementation-verification`; the seeding ruling (D-4) fixes the *when*, not the owner.

## Out of scope

- **O-1**: Folding init onto the adoption machinery (framing B) is out — it relitigates the just-merged existing-repo-onboarding contract (`brainstorm:existing-repo-onboarding#D-1`/`#D-6` chose a distinct verb; its NR-1 guards init non-regression) for gaps the front-door gate closes more cheaply. Revisit only if the two doors' admission/plan surfaces drift apart in practice. (from D-1)
- **O-2**: Real python/node default verb sets (today empty in the ecosystem defaults while go's are real) are out — that content belongs to `prd:implementation-verification` as its own amendment; this feature's truth surface only names the gap. (from D-4)
- **O-3**: A standalone readiness verb, a plan artifact for init, and any confirmation ceremony are out — the checks are init-internal, front-loaded, and fail-fast; init never negotiates. (from D-7)
- **O-4**: Init serving any non-empty target — an "adopt-lite" mode, partial adoption, collision handling of any kind — is out, permanently: the rule is *empty → init; anything else → the refusal names `adopt`/`apply`*. Collision policy lives in adopt alone. (from D-7)

## Candidate glossary terms

none

## Parked technical constraints

- **P-1**: adopt's five-class plan classifier is the natural engine for init's pre-write collision diff — reuse over reinvention keeps `prd:installer#NG-3` (no merge intelligence) honored once, not twice — context: F-2, F-14. *(Mooted by D-7: empty-by-content admission leaves init nothing to diff; collision machinery stays adopt's alone.)*
- **P-6**: a non-git directory *with* content routed to adopt (D-7's "otherwise") meets onboarding UB-5's non-git refusal — either init's refusal names the two-step remedy (`git init`, then `pyxis adopt`) or onboarding amends UB-5 to admit a non-git target by initializing; stage 3 (or a one-line onboarding amendment) decides — context: D-7, F-11.
- **P-7**: `gh auth status` is a reliable presence/auth probe, but per-operation *scope* prediction across token types (classic PAT, fine-grained, App installation) may not be — design decides probe depth; a late wiring permission failure is already survivable under D-5's rollback, so scope checks may degrade to advisory without breaching J1 — context: D-5, F-6.
- **P-2**: a version-floor probe pattern already exists (`internal/verify/invoke`'s `gitFloorVersion`) — reusable for init's git/node floors — context: F-7.
- **P-3**: preview's cold-cache blindness is a fetch-contract question (FR-19's "performs none of them" vs needing the manifest to enumerate the write set) — stage 3 decides read-only manifest fetch vs declared incompleteness — context: F-5.
- **P-4**: upstream `MarkerFiles` stack detection (ecosystem defaults) is the groundwork any install-time stack question or detection would consume — context: F-10.
- **P-5**: wiring `ci_mode` (D-3) changes converge's reconciliation of `.github/workflows/docs-gate.yml` on repos installed before the change — a local-mode repo already carrying the workflow needs a ruled disposition (remove vs leave-with-advisory) in the class policy — context: D-3, F-9.

## Gate

| Field | Value |
|---|---|
| **Verdict** | ready-for-prd |
| **Date** | 2026-07-28 |
| **Operator** | @mdarabi |
| **Covering source** | — |
| **Coverage check** | pass — all six inputs covered (§2.1 ← D-1/D-7 · §2.2 ← D-9 · §2.3 ← D-10 · §3.1 ← D-11 · §3.4 ← O-1..O-4 · appetite ← D-12), zero blocking fog (G-1 is non-blocking, carried to the PRD's open questions) |
