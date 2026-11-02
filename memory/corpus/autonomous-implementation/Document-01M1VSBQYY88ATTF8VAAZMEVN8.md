# PRD — Autonomous Implementation

**Status: draft v0.1 (baseline inventory — as built in harbormaster) · 2026-07-01**
Part of the [Pyxis PRD set](README.md); serves the [product vision](../product-vision.md). Northstar categories: C2 (execution), C3
(agent runtime). Pipeline stage 8.

## Scope

The heart of the factory: unattended implementation of planned work by agent
workers — scheduling, cross-machine mutual exclusion, isolated execution,
resumability, multi-CLI backends, and self-healing. Users: the **factory**
itself; the **operator** observes.

## Features

### IMPL-1 — Gated supervisor invocation (idle ≈ 0 tokens)
**What:** Intelligence fires only when there is work; idling is mechanical and free.
**Requirements (as built):**
- A mechanical tick (every ~5m) runs maintenance + a deterministic gate predicate; the LLM pass fires only on a non-empty work signal.
- **Claim-then-fire:** the gate acquires the per-task leases *before* firing intelligence, so losing racers exit at ~0 cost; acquired claims are binding (spawn, release, or audited reap — never silently dropped).
- Repeat no-op firings are deduped (hash of the candidate set); a **forced-pass liveness floor** (default 3h, unconditional 24h) guarantees every ready task is eventually attempted even under quiet gates, with a pre-probe that skips at 0 tokens when truly clean.
**Origin:** script-gated-supervisor.md; claim-in-gate.md; supervisor-token-efficiency.md.

### IMPL-2 — Supervisor pass
**What:** One orchestrator iteration over claimed work.
**Requirements (as built):**
- Spawn one worker per claimed task; handle triage judgment; end with a structured **pass report**: spawned / succeeded / hollow / failed / skipped + budget.
- **Artifact-verified success:** a task counts succeeded only when its verified proposal exists; a worker that returned cleanly but shipped nothing is reported as *hollow*, never succeeded.
**Origin:** feature-supervisor skill; pass_report (#475); supervisor-spend-attribution.md.

### IMPL-3 — Cross-machine lease (claim) system
**What:** At most one worker anywhere works one task; correctness without a coordinator.
**Requirements (as built):**
- Per-task atomic compare-and-set lease with recorded holder identity; first-writer-wins; losers skip free.
- **Dual TTL reaping:** own never-spawned claims reaped fast (~15m); any claim with no closing proposal reaped at ~2h. No lease outlives its purpose silently.
- **Presence-anchored validity:** a lease is valid only while its holder's heartbeat is alive; a vanished node's leases lapse and the work returns to the pool.
- **Fencing:** a lease lost mid-flight writes a tombstone; the worker aborts at its next checkpoint and its pushed WIP survives for the next claimant; a push/proposal by a fenced worker is blocked.
- Leases with an open closing proposal are immune to reaping; a proposal closing a task leased to *another* machine fails the gate (explicit override only).
- **Interactive claim guard:** a human session hand-taking a pool task is intercepted and redirected (de-label, take the lock the machine way, or open a draft proposal).
**Origin:** claim-in-gate.md; uniform-node-leases.md; interactive-claim-guard.md; claim-ref-single-source.md.

### IMPL-4 — Isolated worker workspaces
**What:** Each worker runs in its own workspace, never the parent checkout.
**Requirements (as built):**
- One isolated working copy per task with a deterministic branch name; guards block any mutation of the parent clone/engine checkout; workspaces for closed tasks are TTL-pruned.
**Origin:** local-agents.md; worktree isolation (#63); clone-isolation guard.

### IMPL-5 — WIP checkpointing & resume
**What:** Worker progress survives any interruption — process, machine, or lease loss.
**Requirements (as built):**
- First push within minutes; checkpoint commits at each milestone plus a small progress note (done / next / gotchas); deterministic branch ⇒ a respawn resumes where the last worker stopped (resume-check is a worker's opening step).
- **Checkpoint-and-exit:** a turn-threshold guard triggers a clean resumable exit; the supervisor respawns up to a cap, then leaves a resumable orphan for the next tick (bounded spend on over-scoped tasks).
- The final proposal is clean: checkpoints squashed, progress note removed.
**Origin:** worker-checkpoint-exit.md; pass-and-worker-slimming.md.

### IMPL-6 — Multi-CLI worker backends
**What:** Workers run on any supported agentic CLI, behind one contract.
**Requirements (as built):**
- Adapter contract per backend: launch / await / kill / auth-check / guard-shim / instructions-filename / tier-map. Backends: Claude Code (warm interactive session), Codex, OpenCode, Cursor (headless).
- Routing is data: (phase, type) → (backend, rung), with precedence per-task override > repo config > routing table.
- Guard behavior is uniform across CLIs (one denylist, per-CLI shims).
**Origin:** multi-CLI redesign (#669); adapters/; single-issue-worker-runtime.md.

### IMPL-7 — Capability rungs (model/effort routing)
**What:** Task difficulty maps to model/effort as data, judged once.
**Requirements (as built):**
- A cost-ranked rung ladder; the planner judges a task's rung at plan time (richest context) and freezes it in the spec; dispatch clamps it; each backend maps rung → native (model, effort) in its own tier-map — native names never leak into the engine.
- Supervisor pass effort is independently configurable per node.
**Origin:** routing.json + tiermaps; two-way picker (#320); SUPERVISOR_EFFORT (#318).

### IMPL-8 — Single-issue worker runtime (scheduler)
**What:** Per-leaf dispatch as an alternative to epic-scoped passes.
**Requirements (as built):**
- A scheduler tick dispatches individual ready leaves to worker slots — **K-way parallel** — running the plan phase (spec generation) or implement phase per leaf state; backend availability/auth is preflighted before dispatch (fast-fail instead of burning the rung ladder); structured events record every dispatch and verified ship; the same leases, specs, and gates apply. Shipped as the baked fleet default (legacy path retired).
**Origin:** single-issue-worker-runtime.md; scheduler.sh; K-way dispatch (#950); cutover (#1026).

### IMPL-9 — Janitors & self-healing
**What:** The system cleans up after every failure mode without a human.
**Requirements (as built):**
- Reapers: never-spawned claims, foreign-TTL claims, lease-expired claims (vanished node), crashed-worker recovery (claim + marker + no proposal → recovery draft).
- Pruning: stale workspaces, dead heartbeats/telemetry, branch heads of closed proposals.
- Recovery: fenced-issue recovery (wedged claim + draft cleared, WIP preserved), orphaned own-claim release, design-approval unparking, epic auto-close, rig-clone freeze detection + auto-restore.
**Origin:** feature-supervisor.sh cleanup family; fleet-hardening.md.

### IMPL-10 — Fair scheduling & liveness protection
**What:** No project starves; no silent stalls.
**Requirements (as built):**
- Per-repo rotation cursor (fairness across projects); epic-level dependency gate (downstream epics held until foundations close); starvation warnings when ready work sits unclaimed; battery policy (a node on battery finishes in-flight work but refuses new leases, configurable).
**Origin:** claim-in-gate.md §fair-rotation; fleet-hardening-system1.md.
