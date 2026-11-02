# implement-now — plan 001

| Field | Value |
|---|---|
| **Feature** | implement-now |
| **PRD** | docs/prds/implement-now |
| **Design** | docs/tech-design/implement-now.md |
| **Plan version** | 001 |
| **Dialect** | harbormaster/fs-spec:v1 |
| **Epic title** | Build the pyxis-native local execution engine (/implement-now) |
| **Status** | Draft — freezes at merge |
| **Template version** | plan-template/v1 |

## Tasks

### T1 — Define the five ports and the single-task Worker cycle

| Field | Value |
|---|---|
| **Blocked by** | — |
| **Size band** | 200–290 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | — |
| **Pattern** | — |
| **Touches** | internal/executor/port (the five interfaces), internal/executor/worker (the cycle + hollow), internal/executor/worker/worker_test.go (inline fakes) |

**Goal:** A single-task Worker runs the whole cycle — claim → workspace → run agent → open+verify proposal → release — end to end against inline fakes, hermetically.

**Current behavior:** No executor exists; pyxis plans and projects but has no code that turns a claimed task into a verified proposal.

**Desired behavior:** Given one task charter + repo access, the Worker claims, prepares a workspace, invokes the runtime, opens a proposal, verifies success by artifact (a proposal exists and is green — else `hollow`, never succeeded), and releases; it reads no sibling or session state.

**Scope:**
- The five consumer-side port interfaces (WorkTracker, Lease, Workspace, AgentRuntime, CodeHost) as the engine's declared contracts.
- The Worker composition of those ports into the one-task cycle, with hollow detection at verify.
- Inline test fakes for all five ports proving the cycle in-process.

**Out of scope:**
- Any real adapter (git, gh, or a CLI) — those are their own tasks.
- Concurrency and the frontier walk — the Dispatcher's task.

**Acceptance criteria:**
- [ ] Driving the Worker with inline fakes runs claim→workspace→run→verify→release for one task and returns `succeeded`.
- [ ] A fake run that produces no proposal is reported `hollow`, never `succeeded`.
- [ ] The Worker reads only its one task charter — a test asserting no sibling/session state is consulted passes.

**Key interfaces:**
- `WorkTracker`, `Lease`, `Workspace`, `AgentRuntime`, `CodeHost` port interfaces; `Worker` with a `Run(ctx, charter)` contract returning a typed outcome.

**Traces:** prd:implement-now/FR-3 · prd:implement-now/FR-10 · design:implement-now#3.1 · design:implement-now#DINV-3 · design:implement-now#DINV-10

### T2 — The Dispatcher and the frontier-sweep run model

| Field | Value |
|---|---|
| **Blocked by** | T1 |
| **Size band** | 210–290 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | — |
| **Pattern** | — |
| **Touches** | internal/executor/dispatch (frontier + scheduler + report), internal/executor/dispatch/dispatch_test.go |

**Goal:** One run computes the ready frontier over the frozen plan's DAG, dispatches Workers claim-serially then run-in-parallel, and ends with a classed report of succeeded, failed, and blocked tasks — consuming zero model cost when the frontier is empty.

**Current behavior:** The Worker (T1) runs one task; nothing walks the plan's DAG or bounds concurrency.

**Desired behavior:** The Dispatcher reads open∧unblocked∧unclaimed tasks through one predicate, claims up to the parallel-width bound one at a time (no two Workers per task), runs the claimed Workers concurrently, and reports each task's terminal class; an empty frontier reports and exits before any model call; one task's failure never halts the run.

**Scope:**
- The single frontier predicate (open ∧ unblocked ∧ unclaimed) over the DAG.
- Claim-serially / run-concurrently scheduling to a parallel-width bound.
- The classed run report (succeeded / failed / blocked) and the empty-frontier zero-cost exit.

**Out of scope:**
- Real tracker/runtime adapters (uses the T1 fakes/ports).
- Turn caps, CI self-heal, and resume — later robustness tasks.

**Acceptance criteria:**
- [ ] A fixture plan with a foundation task and two file-disjoint leaves runs the foundation, then both leaves, and reports 3 succeeded.
- [ ] A run whose frontier is empty emits zero model calls and reports nothing-ready.
- [ ] With one leaf failing, the other independent leaf still completes and the report classes every task.

**Key interfaces:**
- `Dispatcher` with a `Sweep(ctx, plan)` contract; the frontier predicate; the `RunReport` shape.

**Traces:** prd:implement-now/FR-1 · prd:implement-now/FR-11 · prd:implement-now/FR-12 · prd:implement-now/FR-15 · prd:implement-now/FR-18 · design:implement-now#3.6 · design:implement-now#DINV-4 · design:implement-now#DINV-8

### T3 — Workspace adapter: worktree isolation off freshly-fetched mainline

| Field | Value |
|---|---|
| **Blocked by** | T1 |
| **Size band** | 170–250 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | — |
| **Pattern** | — |
| **Touches** | internal/executor/workspace (git worktree adapter + parent guard + reaper), internal/executor/workspace/workspace_test.go |

**Goal:** Each task gets an isolated git worktree branched off the freshly-fetched mainline, with the parent checkout mechanically protected from mutation and worktrees reaped when done.

**Current behavior:** The Workspace port exists (T1) with only a temp-dir fake; no real git isolation.

**Desired behavior:** Preparing a task fetches the mainline and creates a per-task worktree on a fresh branch off that tip; any attempt to mutate the parent clone is blocked by a guard; a completed (merged ∧ closed) or aged worktree is reaped; a task is never based off an unmerged blocker's branch.

**Scope:**
- Worktree creation per task, branched off the freshly-fetched mainline tip.
- A mechanical guard blocking mutation of the parent checkout.
- Worktree reaping on completion or age.

**Out of scope:**
- Claim coordination (the Lease adapter's task).
- Any stacked-branch base selection — the base is always fetched mainline.

**Acceptance criteria:**
- [ ] A Worker's writes appear only in its worktree; a parent-checkout mutation attempt is blocked.
- [ ] A dependent task's worktree is based off freshly-fetched mainline, never an unmerged blocker branch; no stacked PR is created.
- [ ] A worktree for a merged-and-closed task is reaped.

**Key interfaces:**
- The `Workspace` port's git adapter; a worktree prepare/reap contract; the parent-guard.

**Traces:** prd:implement-now/FR-4 · design:implement-now#3.1 · design:implement-now#DINV-1 · design:implement-now#DINV-2

### T4 — Lease adapter: the git-ref CAS claim shared with the fleet

| Field | Value |
|---|---|
| **Blocked by** | T1 |
| **Size band** | 170–250 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | — |
| **Pattern** | — |
| **Touches** | internal/executor/lease (git-ref CAS adapter + in-proc guard), internal/executor/lease/lease_test.go |

**Goal:** At most one Worker anywhere holds a task, via the same git-ref compare-and-swap the external fleet uses, plus a local in-process guard — every claim released on every terminal path.

**Current behavior:** The Lease port exists (T1) with an in-memory fake; no cross-process claim.

**Desired behavior:** Acquiring a claim is a create-or-fail git-ref CAS keyed by (repo, task); a local in-process guard prevents two local Workers racing before the CAS; a losing racer skips with no error; every terminal outcome releases the claim (fire-or-release); the executor coexists with the external fleet with no double-grab.

**Scope:**
- The git-ref CAS acquire-or-skip claim, keyed by (repo, task), with holder identity.
- A local in-process guard against intra-run double-grab.
- Structural release on every terminal path.

**Out of scope:**
- Cross-machine heartbeat/lease-expiry reaping (fleet concern, not the local runtime).
- Workspace isolation (its own task).

**Acceptance criteria:**
- [ ] In a concurrent local+fleet claim race on one task, exactly one Worker ever holds it; the loser skips.
- [ ] Every terminal outcome (success, failure, crash-path) leaves no claim held.
- [ ] A lost race returns a distinct skip, never a hard error.

**Key interfaces:**
- The `Lease` port's git-ref adapter; an acquire/release contract with holder identity; the in-proc guard.

**Traces:** prd:implement-now/FR-2 · prd:implement-now/NR-2 · design:implement-now#3.1 · design:implement-now#DINV-5

### T5 — gh adapters: WorkTracker frontier/claim/verify + CodeHost proposal/checks

| Field | Value |
|---|---|
| **Blocked by** | T1 |
| **Size band** | 200–290 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | — |
| **Pattern** | — |
| **Touches** | internal/executor/ghtracker (WorkTracker gh adapter + tracker_test.go), internal/executor/ghcodehost (CodeHost gh adapter + codehost_test.go) |

**Goal:** Real GitHub adapters read the ready frontier and item tags, mark claims and comment, open a proposal from a task's head branch, and read its required-check states — all via exact, fresh reads.

**Current behavior:** WorkTracker and CodeHost ports exist (T1) with fakes; nothing talks to GitHub.

**Desired behavior:** The WorkTracker gh adapter enumerates open tagged items, verifies an item's automated-implementation tag, and comments; the CodeHost gh adapter opens a PR from the exact head branch and reads required-check states through the Checks API; every decision-critical read is exact and fresh (never a search index or cacheable conditional GET); the plan-new projection contract is consumed unchanged.

**Scope:**
- The WorkTracker gh adapter: frontier read, tag verify, claim-mark, comment.
- The CodeHost gh adapter: open a proposal from the head branch, read required-check states via the Checks API.
- Exact/fresh reads for every consequential lookup.

**Out of scope:**
- The claim CAS mechanism (the Lease adapter's task).
- The CI self-heal loop (the drive-to-green task).

**Acceptance criteria:**
- [ ] The WorkTracker adapter returns exactly the open, automated-implementation-tagged items for a fixture repo state.
- [ ] The CodeHost adapter opens a proposal from the exact head branch and reports its required-check states.
- [ ] A stale-read fixture proves consequential reads use the fresh path, not a cache.

**Key interfaces:**
- The `WorkTracker` and `CodeHost` gh adapters; the Checks-API read contract; the automated-implementation tag.

**Traces:** prd:implement-now/FR-1 · prd:implement-now/FR-7 · prd:implement-now/NR-1 · prd:implement-now/NR-3 · design:implement-now#3.1 · design:implement-now#3.10

### T6 — AgentRuntime contract and the Claude adapter

| Field | Value |
|---|---|
| **Blocked by** | T1 |
| **Size band** | 200–290 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | — |
| **Pattern** | — |
| **Touches** | internal/executor/runtime (the contract + registry + brief + authcheck + runtime_test.go), internal/executor/runtime/claude (adapter + claude_test.go) |

**Goal:** A fresh headless Claude process implements a task from an inline brief in its worktree, its completion read from the process exit and its token/cost parsed from JSON — behind the one runtime contract, with a conformance harness other adapters reuse.

**Current behavior:** The AgentRuntime port exists (T1) as an interface with a fake.

**Desired behavior:** `launch` spawns a fresh non-interactive process per task (no session reuse) with the full charter + relevant context inline; `await` reads completion from the exit code; the adapter parses the CLI's JSON for tokens/turns; `authcheck` preflights install+auth and degrades loudly with a stated remedy on a missing credential, consuming no attempt; `tiermap` maps the frozen rung to (model, effort) with native IDs kept inside the adapter; a self-registration registry (no central switch) admits adapters.

**Scope:**
- The runtime contract (`launch`/`await`/`kill`/`authcheck`/`tiermap`) with exit-code completion and JSON token parsing.
- The Claude adapter, self-registered, spawning a fresh headless process with the inline brief.
- The auth preflight + loud degrade; a reusable adapter conformance harness.

**Out of scope:**
- The other three CLI adapters (their own tasks).
- Turn caps and the CI self-heal loop (later tasks).

**Acceptance criteria:**
- [ ] The Claude adapter drives a fixture task to a green proposal through the runtime contract, with a fresh process (no reuse).
- [ ] A missing/unauthenticated backend degrades loudly with a stated remedy and spawns no counted Worker.
- [ ] Parsed token/turn usage is reported for a completed run.

**Key interfaces:**
- The `AgentRuntime` contract; the self-registration registry; the ephemeral brief assembly; the Claude adapter + conformance harness.

**Traces:** prd:implement-now/FR-5 · prd:implement-now/FR-16 · prd:implement-now/FR-19 · design:implement-now#3.7 · design:implement-now#DINV-11 · design:implement-now#A-11

### T7 — Codex AgentRuntime adapter

| Field | Value |
|---|---|
| **Blocked by** | T6 |
| **Size band** | 100–170 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | — |
| **Pattern** | — |
| **Touches** | internal/executor/runtime/codex (adapter + conformance), codex_test.go |

**Goal:** A fresh headless Codex process (`codex exec`) implements a task through the one runtime contract, self-registered, passing the adapter conformance test.

**Current behavior:** The runtime contract + registry + conformance harness exist (T6); no Codex adapter.

**Desired behavior:** The Codex adapter runs `codex exec` non-interactively with approval/sandbox bypass, reads completion from exit, parses its JSON for usage, and self-registers; it passes the same conformance test the other adapters do.

**Scope:**
- The Codex adapter: headless `exec` launch, exit-code completion, JSON usage parse, `tiermap`.
- Self-registration (no central switch).

**Out of scope:**
- The runtime contract itself (T6).
- Routing which task uses Codex (the config task).

**Acceptance criteria:**
- [ ] The Codex adapter drives a fixture task to a green proposal through the runtime contract.
- [ ] The adapter self-registers and passes the shared conformance test.

**Key interfaces:**
- The Codex `AgentRuntime` adapter; its `tiermap`.

**Traces:** prd:implement-now/FR-5 · design:implement-now#3.7 · design:implement-now#A-4

### T8 — Cursor AgentRuntime adapter (under a pty)

| Field | Value |
|---|---|
| **Blocked by** | T6 |
| **Size band** | 110–180 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | — |
| **Pattern** | — |
| **Touches** | internal/executor/runtime/cursor (adapter + pty wrap + conformance), cursor_test.go |

**Goal:** A fresh headless Cursor process (`cursor-agent -p` under a pseudo-terminal) implements a task through the one runtime contract, self-registered, passing conformance.

**Current behavior:** The runtime contract + registry + conformance harness exist (T6); no Cursor adapter.

**Desired behavior:** The Cursor adapter runs `cursor-agent -p --output-format json --force` inside a pseudo-terminal (the CLI requires a TTY), reads completion from exit, parses JSON usage, and self-registers; it passes the shared conformance test.

**Scope:**
- The Cursor adapter: headless print-mode launch wrapped in a pty, autonomy flag, exit-code completion, JSON usage parse, `tiermap`.
- Self-registration.

**Out of scope:**
- The runtime contract (T6).
- Backend routing (the config task).

**Acceptance criteria:**
- [ ] The Cursor adapter drives a fixture task to a green proposal through the runtime contract, running under a pty.
- [ ] The adapter self-registers and passes the shared conformance test.

**Key interfaces:**
- The Cursor `AgentRuntime` adapter; the pty wrapper; its `tiermap`.

**Traces:** prd:implement-now/FR-5 · design:implement-now#3.7 · design:implement-now#A-4

### T9 — OpenCode AgentRuntime adapter

| Field | Value |
|---|---|
| **Blocked by** | T6 |
| **Size band** | 100–170 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | — |
| **Pattern** | — |
| **Touches** | internal/executor/runtime/opencode (adapter + conformance), opencode_test.go |

**Goal:** A fresh headless OpenCode process (`opencode run`) implements a task through the one runtime contract, self-registered, passing conformance.

**Current behavior:** The runtime contract + registry + conformance harness exist (T6); no OpenCode adapter, and OpenCode is not installed on the authoring machine.

**Desired behavior:** The OpenCode adapter runs `opencode run --format json` non-interactively, reads completion from exit, parses JSON usage, uses `provider/model` naming in `tiermap`, and self-registers; the exact current headless flags are confirmed at implementation and locked by the conformance test.

**Scope:**
- The OpenCode adapter: headless `run` launch, exit-code completion, JSON usage parse, `provider/model` `tiermap`.
- Self-registration; flags confirmed against the live CLI at implementation.

**Out of scope:**
- The runtime contract (T6).
- Backend routing (the config task).

**Acceptance criteria:**
- [ ] The OpenCode adapter drives a fixture task to a green proposal through the runtime contract.
- [ ] The adapter self-registers and passes the shared conformance test against the live CLI.

**Key interfaces:**
- The OpenCode `AgentRuntime` adapter; its `provider/model` `tiermap`.

**Traces:** prd:implement-now/FR-5 · design:implement-now#3.7 · design:implement-now#A-4

### T10 — Drive-to-green: local gate mirrors CI + bounded CI self-heal

| Field | Value |
|---|---|
| **Blocked by** | T5, T6 |
| **Size band** | 220–300 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | — |
| **Pattern** | — |
| **Touches** | internal/executor/drivetogreen (local gate + self-heal state machine), drivetogreen_test.go |

**Goal:** A Worker drives its proposal to genuinely green — a local gate that runs the same checks required by CI, then a bounded self-heal loop that fixes remote-CI failures, capped, then escalates.

**Current behavior:** A Worker opens a proposal (T1/T5) but does not verify or repair its checks beyond a green/not-green read.

**Desired behavior:** Before push, a local gate runs the same deterministic checks required by CI (one predicate — a change failing a required CI check also fails locally); on a red proposal, a bounded state machine reads the Checks API, feeds redacted+clamped failed-CI logs to a fixer run, re-pushes, and re-checks, to a configured cap; at the cap it stops and escalates.

**Scope:**
- The local gate that mirrors required CI (one shared predicate).
- The bounded CI self-heal loop (read checks, redact+clamp logs to a fixer, re-push, re-check).
- The cap-then-escalate terminal.

**Out of scope:**
- The turn cap / checkpoint-resume of a single Worker run (its own task).
- Merging or landing the green proposal (never in scope — the driver lands).

**Acceptance criteria:**
- [ ] A change that fails a required CI check also fails the local gate on the same input.
- [ ] A red proposal is driven to green within the cap in a fixture, with failed-CI logs redacted before reaching the fixer.
- [ ] At the self-heal cap, the task stops, is flagged failed, and escalates.

**Key interfaces:**
- The drive-to-green loop; the local-gate-mirrors-CI predicate; the bounded self-heal state machine.

**Traces:** prd:implement-now/FR-7 · prd:implement-now/FR-8 · prd:implement-now/FR-9 · design:implement-now#3.6 · design:implement-now#DINV-4

### T11 — Bounded execution and resume

| Field | Value |
|---|---|
| **Blocked by** | T2, T6 |
| **Size band** | 190–280 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | — |
| **Pattern** | — |
| **Touches** | internal/executor/worker (checkpoint-exit + turn cap enforcement + worker resume_test), internal/executor/dispatch (respawn + resume-from-tracker + dispatch resume_test) |

**Goal:** A Worker's turns are mechanically bounded and a non-converging Worker checkpoints, exits resumably, and is respawned up to a cap then escalated; a killed run resumes in-flight tasks from their pushed branches with no duplicate proposal.

**Current behavior:** Workers run unbounded (T1); the Dispatcher (T2) has no resume path.

**Desired behavior:** Each Worker's turns are capped by a native flag where present and a uniform wall-clock/budget cap + kill where absent; a Worker over the bound checkpoints and exits resumably; the Dispatcher respawns it up to a cap, then escalates; a re-invocation re-probes the tracker and continues in-flight tasks from their pushed branches, producing no duplicate proposal.

**Scope:**
- Mechanical per-Worker turn bound (native flag or uniform wall-clock/budget + kill).
- Checkpoint-exit + bounded respawn + escalate.
- Crash/re-invocation resume from the tracker + pushed WIP, idempotent (no duplicates).

**Out of scope:**
- The CI self-heal loop (its own task).
- The in-session wake-driven auto-resume layer (a future design).

**Acceptance criteria:**
- [ ] A Worker exceeding the turn bound checkpoints, exits, is respawned up to the cap, then escalates.
- [ ] A killed mid-run re-invocation resumes in-flight tasks from pushed branches with no duplicate proposal.
- [ ] The turn bound holds uniformly across a native-flag backend and a wall-clock-enforced backend.

**Key interfaces:**
- The turn-bound enforcement; the checkpoint-exit + respawn contract; the resume-from-tracker probe.

**Traces:** prd:implement-now/FR-13 · prd:implement-now/FR-14 · design:implement-now#3.7 · design:implement-now#DINV-9

### T12 — Backend routing and the caps config

| Field | Value |
|---|---|
| **Blocked by** | T6, T2 |
| **Size band** | 120–200 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | — |
| **Pattern** | — |
| **Touches** | internal/executor/config (routing table + caps, closed-schema), config_test.go |

**Goal:** Backend selection and the caps are closed-schema policy on the pyxis-config surface — a default backend plus an optional per-repo table, and the turn, self-heal, and parallel-width caps — with unknown keys erroring and enforcement mechanical.

**Current behavior:** Backends are self-registered (T6–T9) but nothing selects one per task; caps are hardcoded or absent.

**Desired behavior:** An executor config block carries a default backend plus an optional per-repo `(task type | phase) → backend` table, and the worker-turn / CI-self-heal / parallel-width caps, with sensible defaults + per-repo override; config parses closed (unknown keys error); the frozen rung is clamped to (model, effort) within the chosen backend; caps are enforced mechanically, never model-judged.

**Scope:**
- The default-backend + per-repo routing table on the closed-schema config surface.
- The three caps as config with defaults + override.
- The downstream rung clamp to (model, effort).

**Out of scope:**
- Per-task CLI override (deferred to a future plan-charter field).
- The plan-time rung envelope (routing-envelope owns it).

**Acceptance criteria:**
- [ ] A repo config selects the default backend and a per-repo table routes a matching task to its backend.
- [ ] An unknown config key is a parse error.
- [ ] The caps read from config are enforced mechanically in a fixture.

**Key interfaces:**
- The executor config block schema; the routing lookup; the caps; the rung clamp.

**Traces:** prd:implement-now/FR-6 · prd:implement-now/FR-11 · design:implement-now#3.8

### T13 — The /implement-now surface and two-sink reporting

| Field | Value |
|---|---|
| **Blocked by** | T2, T12 |
| **Size band** | 190–280 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | — |
| **Pattern** | — |
| **Touches** | cmd/implement-now (thin main), internal/executor/report (event stream + sinks + report_test.go), the implement-now skill + authoring guide (the vendor-neutral agent surface) |

**Goal:** The operator (attended) and argus (unattended) drive the same frontier-sweep through one normalized event stream — a live progress view in the session, or argus's delivery journal with fail-closed escalation — via the vendor-neutral `/implement-now` surface.

**Current behavior:** The Dispatcher runs a sweep (T2) but emits no normalized events and has no operator/argus surface.

**Desired behavior:** A run emits one normalized event stream (dispatched → verified → outcome → finished) delivered to two sinks: attended → a live in-session progress view + a classed final summary; unattended → argus's existing delivery journal, a stuck/failed task fail-closed-escalating to argus; intervention is coarse (cancel a run / kill a Worker); the surface is a vendor-neutral skill + authoring guide with a thin `cmd` main.

**Scope:**
- The one normalized event stream and the two sinks (operator live view · argus journal + escalation).
- The vendor-neutral `/implement-now` skill + authoring guide + thin main.
- Coarse intervention (cancel/kill).

**Out of scope:**
- The argus drive protocol itself (argus owns it; this feeds its journal).
- Landing/merge (merge-lane's).

**Acceptance criteria:**
- [ ] The same run completes and reports under an attended invocation (live view) and an unattended (journal-only) invocation.
- [ ] The event stream carries the per-task lifecycle events, and a stuck task escalates fail-closed to the argus sink.
- [ ] Cancelling a run kills its in-flight Workers and releases their claims.

**Key interfaces:**
- The normalized event stream; the two sink adapters; the `/implement-now` skill + authoring guide + `cmd` main.

**Traces:** prd:implement-now/FR-17 · design:implement-now#3.9 · design:implement-now#DINV-7

### T14 — End-to-end verification, dogfood, and the stage-8 map re-pin

| Field | Value |
|---|---|
| **Blocked by** | T3, T4, T5, T7, T8, T9, T10, T11, T13 |
| **Size band** | 130–220 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | — |
| **Pattern** | — |
| **Touches** | internal/executor/e2e (end-to-end drill + reusability check), the operational pipeline map's stage-8 row |

**Goal:** A real small pyxis feature is implemented end to end by `/implement-now` locally, the Worker core proves runtime-agnostic behind a second (fake/remote-shaped) Dispatcher adapter, and the pipeline map's stage-8 row states the native-local binding.

**Current behavior:** The engine's parts exist and are unit/conformance tested; no whole-system drill or map correction has run.

**Desired behavior:** An end-to-end drill runs a real frozen plan through `/implement-now` to green PRs (operator only reviewing/merging); the Worker core + ports pass conformance behind a fake Dispatcher adapter with zero Worker change (the reusability check); the pipeline map's stage-8 row is moved to state the native-local runtime binding alongside the external fleet.

**Scope:**
- The end-to-end drill (a real plan → green PRs via `/implement-now`).
- The reusability check (the Worker core behind a second Dispatcher adapter, no Worker change).
- The pipeline-map stage-8 row correction.

**Out of scope:**
- The remote/Docker Dispatcher binding itself (a future design).
- Any PRD amendment content (the FR-10/FR-20 re-pin is a follow-up amendment, not this task).

**Acceptance criteria:**
- [ ] `/implement-now` implements one real small pyxis feature end to end locally, with the operator only reviewing/merging.
- [ ] The Worker core passes conformance behind a second (fake/remote-shaped) Dispatcher adapter with no Worker change.
- [ ] The pipeline map's stage-8 row states the native-local binding.

**Key interfaces:**
- The end-to-end drill; the second Dispatcher adapter (the reusability witness); the pipeline-map stage-8 row.

**Traces:** prd:implement-now/FR-20 · design:implement-now#A-9 · design:implement-now#A-12

## Coverage

| Requirement | Tasks |
|---|---|
| prd:implement-now/FR-1 | T2, T5 |
| prd:implement-now/FR-2 | T4 |
| prd:implement-now/FR-3 | T1 |
| prd:implement-now/FR-4 | T3 |
| prd:implement-now/FR-5 | T6, T7, T8, T9 |
| prd:implement-now/FR-6 | T12 |
| prd:implement-now/FR-7 | T5, T10 |
| prd:implement-now/FR-8 | T10 |
| prd:implement-now/FR-9 | T10 |
| prd:implement-now/FR-10 | T1 |
| prd:implement-now/FR-11 | T2, T12 |
| prd:implement-now/FR-12 | T2 |
| prd:implement-now/FR-13 | T11 |
| prd:implement-now/FR-14 | T11 |
| prd:implement-now/FR-15 | T2 |
| prd:implement-now/FR-16 | T6 |
| prd:implement-now/FR-17 | T13 |
| prd:implement-now/FR-18 | T2 |
| prd:implement-now/FR-19 | T6 |
| prd:implement-now/FR-20 | T14 |
| prd:implement-now/NR-1 | T5 |
| prd:implement-now/NR-2 | T4 |
| prd:implement-now/NR-3 | T5, T13 |
| prd:implement-now/NR-4 | T13 |
| prd:implement-now/UB-1 | T2, T10 |
| prd:implement-now/UB-2 | T11 |
| prd:implement-now/UB-3 | T6 |
| prd:implement-now/UB-4 | T1 |
| prd:implement-now/UB-5 | T10 |
| prd:implement-now/UB-6 | T11 |
| prd:implement-now/UB-7 | T6 |
| prd:implement-now/UB-8 | T2, T4 |
