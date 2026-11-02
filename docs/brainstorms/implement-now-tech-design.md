# implement-now — design-brainstorm record

| Field | Value |
|---|---|
| **In one line** | The solution shape for pyxis's native local execution engine (`/implement-now`): the worker/runtime/dispatch/lease/workspace ports and their local↔remote swap seam, the drive-to-green loop, dependency ordering under never-merge, CLI routing, and the attended/unattended surface — grounded in the merged PRD. |
| **Relevant-when** | designing or changing the pyxis-native executor's internals — the single-task worker, the AgentRuntime / dispatch / lease / workspace ports, the drive-to-green loop, dependency handling, per-task CLI routing, the attended/unattended surface; checking prior design rulings on the executor shape or the local↔remote seam |
| **Basis** | prd:implement-now |
| **Authority** | @mdarabi (operator) |
| **Date(s)** | 2026-07-16 |
| **Status** | gated: ready-for-design |
| **Tracking** | — |
| **Feeds** | docs/tech-design/implement-now.md |
| **Prior records** | brainstorm:implement-now · brainstorm:autonomous-developer |
| **Template version** | design-brainstorm-record/v1 |

## Orientation

| Source | Why loaded |
|---|---|
| `docs/prds/implement-now/effective.md` (the merged basis) | the frozen contract — FR-1..20, NR-1..4, UB-1..8, NFR-1..6, §7.2 design-owned questions |
| `brainstorm:implement-now` (stage-1 record) | the settled shape decisions this design instantiates (D-1 four-CLI runtime, D-3 drive-to-green, D-4 tracker-mediated, D-5 worker+dispatch, P-1..P-6 parked design constraints) |
| `brainstorm:autonomous-developer` | the fleet-seam rulings this executor binds under (D-8 adapter-neutral seam, O-4 stage-8 nativization) |
| `docs/domain-model.md` §3.2–3.4 | the stage-8 ports already named for transformation (Worker/Pass/Lease/AgentRuntime; Node kind-blind) |
| `docs/tech-design/{fleet-bridge,argus-agent,merge-lane,plan-new,routing-envelope}-v1.md` | frozen upstream contracts + the plan/charter consumed; routing-envelope owns the plan-side rung, scopes out the executor's dispatch clamp |
| `docs/architecture-principles.md` (AP-1..27, RW-1..6) | the principle registry every `ALT-n` scores against by ID |
| `docs/observability/incidents.md` (I1–I3) | pyxis's own incident registry for panel "incidents matched" |
| `docs/tech-design/{design-brainstorm-record-model,brainstorm-record-model}.md` + templates | this record's own contract |

**Overlaps surfaced:**
- **No live design Serves `implement-now` or decides the executor's shape** — this is the first executor design (not `covered`).
- `routing-envelope-v1` — **adjacent, clean boundary**: it owns the *plan-time* rung envelope check and explicitly scopes out "the executor's downstream capability clamp at dispatch (AP-21)" — that clamp is this design's (DQ-4). No conflict; the frozen rung is an input.
- `plan-new-v1` — the projection + charter this executor consumes (unchanged; NR-1).
- `fleet-bridge-v1` / `argus-agent-v1` / `merge-lane-v1` — the binding, principal/hands, and landing contracts this executor honors (F-4).

## Design questions

- **DQ-1**: What is the port decomposition of the executor core, and where exactly does the local↔remote swap seam sit? *(the §2 direction)*
- **DQ-2**: Under never-merge, how does one run sequence a **dependent** task whose blocker's PR isn't merged yet — stack the dependent PR on the blocker's branch · pause-for-merge-then-resume in-session · frontier-only-then-reinvoke?
- **DQ-3**: What is the AgentRuntime completion/status/token contract, and is it uniform (fresh-headless, exit-code) across all four CLIs — or does Claude Code need a distinct adapter mode? *(pending the headless-Claude fact, F-8)*
- **DQ-4**: How is a task's CLI/backend chosen (default + per-repo config + optional per-task override), given the charter carries a `rung` but no CLI hint?
- **DQ-5**: How does the same worker core surface to the operator (attended: live progress + intervention) vs argus (unattended: journal + fail-closed escalation)?
- **DQ-6**: Where do the executor's caps live (worker turn bound, CI self-heal attempt cap, parallel-dispatch width) and what governs them?

## Panel log

*One line per `DQ-n`, filled as each resolves (DINV-7).*

- **DQ-1**: extends — skipped — the worker↔dispatch seam was opened + ruled at `brainstorm:implement-now#D-5`; the domain model names these ports (F-1) and RW-4 fixes the substrate-first hexagon, so this DQ instantiates proven contracts with no structural fork left to diverge. Operator confirmed, did not force a panel.
- **DQ-2**: opens — fired — the base-selection / dependency-handling seam carries a genuine three-way structural fork (stack · pause-resume · frontier-sweep), with I2 bearing on the stack option; three scored `ALT-n` produced, ALT-3 chosen.
- **DQ-3**: extends — skipped — the AgentRuntime seam (S-2) was opened at DQ-1; F-8 + F-9 confirm all four CLIs are uniform fresh-headless with exit-code completion, so the runtime contract instantiates the seam with no structural fork. The interactive-tmux alternative (ALT-4, hand-authored) was raised by the operator and withdrawn for simplicity — logged, not a fired panel.
- **DQ-4**: extends — skipped — backend selection extends the pyxis-config surface (AP-8) + the routing concept `routing-envelope-v1` established plan-side (which scoped out executor-side selection, F-2); a config-table lookup, no structural fork. Operator confirmed, did not force.
- **DQ-5**: extends — skipped — AP-5 + AP-2 + the PRD's EVT tracking plan (§5.7) determine "one normalized event stream, two sinks"; the unattended sink reuses argus's frozen journal/escalation/wake surfaces — no structural fork. Operator adopted, did not force.
- **DQ-6**: extends — skipped — the caps extend the pyxis-config surface (AP-8 closed-schema); config values with mechanical enforcement (AP-16/AP-19/AP-21), no structural fork. Operator adopted, did not force.

## Decisions

### D-1 — the port decomposition: a portable Worker of five ports, dispatched by a swappable Dispatcher

- **Question**: what is the executor's port decomposition, and where does the local↔remote swap seam sit?
- **Answer**: a **Dispatcher** (the swap seam — local in-process bounded-parallel scheduler now, remote fleet scheduler later) walks the frozen plan's frontier and allocates **Workers**; the Worker is the portable single-task unit composing five ports — **WorkTracker** (frontier/claim/verify/comment), **Lease** (git-ref CAS + local in-process guard), **Workspace** (worktree per task off freshly-fetched mainline + parent guard), **AgentRuntime** (`launch/await/kill/authcheck/tiermap` — four fresh-headless CLI adapters), **CodeHost** (open PR / read checks). Worker + all ports are shared below the Dispatcher (the reusability bet, NFR-5).
- **Reasoning**: operator adopted. Instantiates the stage-1 worker+dispatch ruling (`brainstorm:implement-now#D-5`), the domain model's named ports (F-1), and RW-4's substrate-first hexagon; ports harbormaster's adapter *behaviors*, not its bash (RW-1/RW-3); honors AP-1 (the agent runtime is a swappable medium).
- **Anchor**: 2026-07-16 · t1
- **Status**: live

### D-2 — dependency ordering: frontier-sweep, the driver re-invokes (never stack, never in-session-wait)

- **Question**: under never-merge, how does one run sequence a dependent task whose blocker isn't merged?
- **Answer**: **ALT-3** — a run builds only the unblocked frontier, opens green PRs, reports the blocked set, and exits; **landing + re-invocation is the driver's job** (attended operator merges + re-runs; argus lands via merge-lane/human and re-invokes through the wake-channel). Every sweep branches off freshly-fetched `origin/main`. The executor **never bases a task off an unmerged blocker's branch** (no stacking — rejects I2) and **never blocks in-session on a merge** (ALT-2's wait is deferred to a future optional wake-driven layer).
- **Reasoning**: operator chose ALT-3. Simplest + correct-by-construction (AP-22/AP-26/AP-13); pushes landing/resume to the driver where the separation already lives (argus owns the wake-channel + merge-lane); rejects the stacking I2 punished.
- **Anchor**: 2026-07-16 · t2
- **Status**: live

### D-3 — the AgentRuntime contract: uniform fresh-headless across all four CLIs, exit-code completion, artifact-verified

- **Question**: what is the runtime completion/status/token contract, and does it hold uniformly across the four CLIs?
- **Answer**: **uniform fresh-headless** (FR-5). One fresh process per task in the task's worktree; **completion = process exit**; the adapter parses each CLI's JSON for tokens/cost + turns. Contract: `launch(brief,workspace)→handle` · `await(handle)→{status,tokens,turns}` · `kill` · `authcheck` (preflight install+auth, FR-16) · `tiermap(rung)→(model,effort)`. **AP-13 nuance**: a clean exit is preliminary — the Worker verifies by artifact (PR exists + green) before counting success; clean-exit-no-PR is hollow (FR-10). Each adapter absorbs its CLI's autonomy flag + quirks (F-9).
- **Reasoning**: operator confirmed headless `-p` over interactive tmux (ALT-4) "for simplicity" — keeping FR-5 intact, no amendment; verified all four support it (F-8 + F-9). Honors AP-1 (runtime = swappable medium), AP-13 (verified artifact).
- **Anchor**: 2026-07-16 · t3
- **Status**: live

### D-4 — CLI/backend routing: default backend + optional per-repo table; rung stays orthogonal

- **Question**: how does a task pick its CLI/backend, given the charter carries a `rung` but no CLI hint?
- **Answer**: a **default backend** (repo-configured on the pyxis-config surface, AP-8) + an optional **per-repo `(task type | phase) → backend` table** (harbormaster's `routing.json` behavior ported, RW-1). **Per-task override is deferred** (the charter has no CLI-hint field; a future plan-charter field or projected-issue label would carry it — O-2). The frozen **`rung` stays orthogonal** — it picks model/effort *within* the chosen backend (the adapter `tiermap`, D-3), not the backend.
- **Reasoning**: operator adopted. Extends the config/routing seam (AP-8 closed-schema + `routing-envelope-v1`'s plan-side rung, which scoped out this executor-side selection, F-2); a config lookup, no structural fork; native model IDs stay in each adapter's tiermap (AP-1).
- **Anchor**: 2026-07-16 · t4
- **Status**: live

### D-5 — the attended/unattended surface: one normalized event stream, two sinks, coarse intervention

- **Question**: how does the same worker core surface to the operator (attended) vs argus (unattended)?
- **Answer**: **one normalized event stream** from the executor (the PRD's EVT-1..6 per-task lifecycle + a final report), emitted identically for both drivers, over **two sinks** — attended → a live progress view in the session; unattended → argus's existing **delivery journal**, a stuck/failed task **fail-closed-escalating to argus** (which escalates to human per its authority contract), reusing argus's frozen journal/escalation-brief/wake-channel rather than new surfaces. **Intervention is coarse** (cancel a run / kill a worker) — a consequence of headless `-p` (D-3), no attachable TUI. One core + a thin two-sink reporting adapter, never two cores.
- **Reasoning**: operator adopted. Honors AP-2 (invocation-independent — identical run under either driver) + AP-5 (observability first-class, structured events); the unattended sink reuses argus's frozen surfaces; extends the observability seam, no structural fork.
- **Anchor**: 2026-07-16 · t5
- **Status**: live

### D-6 — the caps: closed-schema policy on the pyxis-config surface, mechanically enforced

- **Question**: where do the executor's caps (worker turn bound, CI self-heal attempt cap, parallel-dispatch width) live and what governs them?
- **Answer**: all three are **closed-schema policy on the pyxis-config surface** (AP-8) — an executor config block with sensible defaults + per-repo override; config parses **closed** (unknown keys error, AP-6). Enforced **mechanically** by the Worker/Dispatcher (AP-16 bounded-loops-then-escalate · AP-19 idle-free · AP-21 clamp), never model-judged. The turn bound maps to `--max-turns` (claude, cursor) + `--max-budget-usd` (claude); for codex/opencode the Worker enforces a uniform wall-clock/turn budget + kill — keeping the cap uniform across backends despite differing native flags.
- **Reasoning**: operator adopted. Extends the AP-8 closed-schema config surface; config values with mechanical enforcement, no structural fork; honors AP-16/AP-19/AP-21.
- **Anchor**: 2026-07-16 · t6
- **Status**: live

## Alternatives

### ALT-1 — Stack the dependent PR on the blocker's branch

- **Shape**: the dependent task branches off the blocker's *PR branch* (not mainline); a chain of stacked PRs, landed bottom-up.
- **Trade-offs**: one uninterrupted run builds the whole DAG; but fragile — stacked-train landing discipline, re-basing on blocker-PR changes, the chain entangled.
- **Principles**: strains **AP-26** (a merged badge certifies its base, not mainline); **AP-13** at risk (children land off orphaned bases).
- **Incidents**: **matches I2** — a stacked four-proposal train merged into orphaned side branches; every badge "merged" while nothing reached mainline; the fix re-stranded within the hour.
- **Verdict**: rejected — re-imports the exact I2 failure class the repo already paid for.
- **From**: DQ-2 · panel

### ALT-2 — Pause-for-merge, resume in-session

- **Shape**: build the frontier; when a dependent is blocked, the run waits in-session for the blocker to reach mainline, then resumes it off freshly-fetched `origin/main`.
- **Trade-offs**: one command finishes the DAG, always off merged mainline; but blocks on merge latency (human/async) — an unbounded in-session wait.
- **Principles**: honors **AP-26**/**AP-13** (branches off observed mainline); strains **AP-19** (a blocked run holds resources, not idle-free).
- **Incidents**: none matched (checked `docs/observability/incidents.md`, 2026-07-16) — avoids I2 by construction.
- **Verdict**: rejected as load-bearing (its in-session wait duplicates the driver's role); **hybridized into a future optional wake-driven auto-resume layer** over ALT-3.
- **From**: DQ-2 · panel

### ALT-3 — Frontier-sweep; the driver re-invokes

- **Shape**: a run builds only the currently-unblocked frontier, opens those PRs, reports "N blocked on #X landing," and exits. Landing + re-invocation is the driver's job (operator merges + re-runs; argus lands + re-invokes via the wake-channel). Each sweep branches off freshly-fetched `origin/main`.
- **Trade-offs**: simplest, stateless, never blocks, correct base every time; a dependency chain spans multiple sweeps (~2 for foundation-plus-leaves).
- **Principles**: honors **AP-22** (idempotent + raceable — re-invocation converges), **AP-19** (idle-free between sweeps), **AP-26**/**AP-13** (each sweep off merged mainline), **AP-20** (narrow blast — the executor doesn't own landing).
- **Incidents**: none matched (checked `docs/observability/incidents.md`, 2026-07-16) — it is harbormaster's proven frontier-gated model minus the async fleet.
- **Verdict**: chosen — simplest + correct; landing/resume belongs to the driver.
- **From**: DQ-2 · panel

### ALT-4 — Interactive CLI in a tmux session (per task)

- **Shape**: run each CLI's interactive TUI in a tmux session, drive it by send-keys; detect turn-end by pane-scrape + transcript settle (harbormaster's model).
- **Trade-offs**: attachable / watchable / interruptible — a rich attended UX; but loses the exit-code completion signal (settle-detection complexity), and warm reuse reintroduces charter-bleed.
- **Principles**: strains **AP-10** (a scrape/settle heuristic vs a clean signal); no correctness gain over headless; contradicts the basis (FR-5 "non-interactive").
- **Incidents**: none in pyxis's registry (checked `docs/observability/incidents.md`, 2026-07-16); the prior-art harbormaster **I18/I19** bear on it — the pane-scrape settle detection is the charter-bleed surface (F-7).
- **Verdict**: rejected — contradicts frozen **FR-5**, would require a `/prd-amend`; operator raised it, then **withdrew it for simplicity**, keeping headless.
- **From**: DQ-3 · hand-authored (operator-raised)

## Seam map

- **S-1**: Dispatcher — **opens** — behind it: a local in-process bounded-parallel scheduler (the remote fleet scheduler is a later adapter) — feeds: Scope "the dispatch layer" + NFR-5 (the local↔remote reusability boundary)
- **S-2**: AgentRuntime port — **opens** — behind it: four fresh-headless CLI adapters (`launch/await/kill/authcheck/tiermap`) — feeds: Serves FR-5/FR-6, Scope "the backend-agnostic runtime"
- **S-3**: Lease port — **opens** — behind it: the git-ref CAS claim (shared with the fleet) + a local in-process guard — feeds: FR-2, NR-2
- **S-4**: Workspace port — **opens** — behind it: a git worktree per task off freshly-fetched `origin/main` + a parent-checkout guard — feeds: FR-4, AP-20
- **S-5**: WorkTracker port — **extends** — behind it: a gh adapter for frontier/claim/verify/comment (may bind the existing `internal/worktracker` port later) — feeds: FR-1, NR-1
- **S-6**: CodeHost port — **opens** — behind it: a gh adapter (open PR, read CI check states) — feeds: FR-7
- **S-7**: base-selection / frontier-gating — **opens** — behind it: each sweep reads the frozen DAG and works only open∧unblocked∧unclaimed tasks off freshly-fetched mainline; the driver lands blockers + re-invokes (D-2) — feeds: Scope "the frontier-sweep run model" + FR-1, honors AP-26
- **S-8**: backend-routing — **extends** — behind it: a default backend + optional per-repo `(type|phase)→backend` table on the pyxis-config surface (AP-8) — feeds: Scope "backend routing," Serves FR-5
- **S-9**: reporting / event surface — **extends** — behind it: one normalized executor event stream (EVT-1..6) with two sinks (operator live view · argus delivery journal + fail-closed escalation) — feeds: Scope "the attended/unattended surface," Serves FR-17, honors AP-2/AP-5

## Facts

- **F-1**: The domain model already names the stage-8 ports for transformation — **Worker** (via AgentRuntime), **Pass** (engine loop), **Lease/Claim** (LeaseStore port), **AgentRuntime** (`launch/await/kill/auth_check/guard_shim/instructions_filename/tiermap` — "the proven seam; generalize the pattern"), **Node** ("kind-blind — laptop/box/container, no code path may distinguish"). — source: `docs/domain-model.md` §3.2–3.4
- **F-2**: The per-task `rung` is frozen at plan time and validated against the model-routing **envelope** by the `routing` plangate check (routing-envelope-v1); that design **explicitly scopes out** "the executor's downstream capability clamp at dispatch (AP-21)" and the tier→concrete-model map. So the executor consumes a frozen rung, clamps downstream, and maps rung→(model,effort) per adapter; **CLI/backend selection is separate from rung**. — source: `routing-envelope-v1` §Scope/§2
- **F-3**: The consumption contract (`plan/v1` JSON): per-task charter = `goal, current/desired, scope, out_of_scope, acceptance[], key_interfaces[], traces[], blocked_by[]` (DAG), `foundation.level, size_band, rung?`, `plan_task_id` (idempotency key). Projection renders each task → a GitHub issue + `fs-spec:v1` marker + `local-agent` label; frontier = open ∧ unblocked ∧ unclaimed; duplicate carrier → halt, never auto-repair (AP-17). — source: `plan-new-v1`, `internal/plangate/*`
- **F-4**: Frozen upstream invariants the executor honors: **argus DINV-6** (principal never implements fleet-tagged work — executor is the hands); **merge-lane DINV-5** (no drive/session holds merge credentials; classifier author-agnostic); **fleet-bridge DINV-1** (one projection path) + §3.4 **FR-10 re-pin hook** (the sanctioned way to move the stage-8 binding). — source: `docs/tech-design/{argus-agent,merge-lane,fleet-bridge}-v1.md`
- **F-5**: Governing principles bearing on the executor: **AP-1** (medium-independent core — the agent-runtime is a swappable medium), **AP-11** (one predicate one place; local gate mirrors CI exactly), **AP-12** (every acquisition accounted; janitor in the same change), **AP-13/AP-26** (success is a verified artifact; a medium's status certifies its own object — accept goal-state only by observing the goal object), **AP-16** (bounded loops + escalate), **AP-17** (fail closed on destructive), **AP-19** (idle costs nothing; gate secures work first), **AP-20** (narrowest blast radius; worker mutates only its workspace, mechanically guarded), **AP-21** (judge once, clamp downstream), **AP-22** (idempotent + raceable), **AP-25** (vendor-neutral agent surface), **AP-27** (extension by addition; a shared append point is a defect — aggregates derived, not authored), **RW-1/3/4** (port behaviors not mechanisms; medium workarounds die at the port; substrate-first: ports + minimal-local adapter + conformance kit before real adapters). — source: `docs/architecture-principles.md`
- **F-6**: pyxis incident matches available for panels — **I2** (a stacked-proposal train merged into orphaned side branches; every badge read "merged" while nothing reached mainline — AP-26) → matches the "stack the dependent PR" option in DQ-2; **I3** (consequential CLIs fail closed on misparse; never truncate a consequential run's output) → matches the CLI/dispatch fail-closed posture. — source: `docs/observability/incidents.md`
- **F-7**: Harbormaster prior-art **mechanisms** to port as behaviors, never copy (RW-1): the adapter contract (launch/await/kill/tiermap), the single-issue-worker-runtime, worktree-per-task off freshly-fetched `origin/main` (#982), the git-ref claim CAS, the bounded CI-fix state machine (Checks API + redacted logs + capped attempts), checkpoint-exit-then-resume. Paid-for lessons already graduated into pyxis APs: I18/I19 charter-bleed → fresh-headless; P1→AP-11; P9→AP-19; P3→AP-13; P10→AP-20. — source: `~/agenticlab-ai/harbormaster` (research)
- **F-8**: **Q-1 resolved — headless-Claude viability = YES** (verified against the installed CLI + live test + docs). `claude -p --permission-mode bypassPermissions --model <m> --effort <e> --output-format json` runs a full autonomous multi-tool agentic loop, exits with a status code (0=success), and emits token/cost usage as JSON (`usage`, `modelUsage`, `num_turns`, `terminal_reason`); **`--max-turns N`** (turn cap, print-mode only — exits with error at the limit) and `--max-budget-usd` bound it; `--resume <id>` / `--fork-session` resume, session-id keyed **per-directory** (natural with worktree-per-task). Tool + MCP parity with the TUI. **So all four CLIs are uniform fresh-headless with exit-code completion — Claude needs no distinct adapter mode; the prior tool's warm-TUI/tmux was an amortization choice, not a capability gap.** Design caveats: avoid auto-mode (it aborts after repeated blocks with no human — use `bypassPermissions`); background procs exit ~5s post-run (design tasks to finish in the foreground). — source: Claude-Code guide verification (CLI + docs), 2026-07-16
- **F-9**: **All four CLIs support headless/non-interactive autonomous runs** (operator's "ensure", verified 2026-07-16): **codex** `exec` ("Run Codex non-interactively") `[--json] -m <model>` with `--dangerously-bypass-approvals-and-sandbox` or `-s workspace-write|danger-full-access` + `resume` — installed, `codex exec --help` confirms; **cursor** the `cursor-agent -p --output-format json --model <m> --force|--yolo` binary ("non-interactive use, has access to all tools") — installed; harbormaster runs it under a **pty** (`script`); **opencode** `run --model provider/model --format json [--session|-c]` (non-interactive scripting mode) — docs-confirmed (opencode.ai/docs/cli), **not installed on this machine** to live-test, but harbormaster's production adapter drives it headlessly; **claude** per F-8. Each emits structured output for token/cost accounting and has session resume. Per-CLI autonomy flag + cursor's pty are **adapter-absorbed** quirks. — source: installed CLI `--help` (codex, cursor-agent) + F-8 (claude) + opencode.ai/docs/cli + harbormaster adapters

## Fog

none *(session open)*

## Out of scope

- **O-1**: PR stacking / basing a task off an unmerged blocker's branch — the executor always branches off freshly-fetched mainline; stacking re-imports I2. (from D-2)
- **O-2**: per-task CLI/backend override — deferred for MVP (default + per-repo table only); a future plan-charter CLI-hint field or projected-issue label would carry it. (from D-4)

## Candidate glossary terms

none *(pending)*

## Parked product gaps

none *(none discovered yet)*

## Gate

| Field | Value |
|---|---|
| **Verdict** | ready-for-design |
| **Date** | 2026-07-16 |
| **Authority** | @mdarabi |
| **Covering design** | — |
| **Coverage check** | pass — core four covered (§2 D-1 · §4 ALT-1/2/3 · Serves/Scope S-1..S-9 + Basis · §3.2 O-1/O-2), zero blocking fog |
| **Amend route** | — |
