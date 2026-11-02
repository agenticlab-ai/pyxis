# implement-now — brainstorm record

| Field | Value |
|---|---|
| **In one line** | The first pyxis-native stage-8 executor: after a plan is projected, `/implement-now` claims its `local-agent`-tagged tasks and implements them into PRs synchronously, in the same session, on this machine — a second concrete binding of the adapter-neutral fleet seam harbormaster's external nodes also bind. |
| **Relevant-when** | brainstorming or contracting the pyxis-native implementation executor (pipeline stage 8, local/synchronous shape); deciding how projected work items are claimed and executed on the same machine as the pipeline; checking prior rulings on the fleet seam, the principal/hands split, or nativizing stages 8–10 |
| **Idea** | "given we have the planning phase ready, the next natural phase would be implementation. we have the similar idea in ~/agenticlab-ai/harbormaster/ already implemented and we have a tons of learning on what could go wrong (read through github issues for harbormaster and prs) the implementation can have two different shapes (but same core implementation). the implementation can happen within the separate machine (this is what harbormaster does; where we launch it in the docker container and it reads from github issues list and implements them). or it can happen locally on the same harness cli that design happened. in our first implementation, we want to focus on the local implementation on the same session as the design. from the user perspective after the plan-new is kicked off, they can run the `/implement local` skill and the tasks will be implemented on the same machine by starting a proper set of harness cli with specific task for the implementation" (operator, 2026-07-15) |
| **Operator** | @mdarabi |
| **Date(s)** | 2026-07-15 |
| **Status** | gated: ready-for-prd |
| **Tracking** | — |
| **Feeds** | docs/prds/implement-now/ |
| **Prior records** | brainstorm:autonomous-developer |
| **Template version** | brainstorm-record/v1 |

## Orientation

| Source | Why loaded |
|---|---|
| `.pyxis/pipeline.md` (stage 8) + `docs/product-vision.md` §4/§137/§115 | the governing pipeline: stage 8 = Factory, per-task implement+test+doc+PR with evidence package; trust level 2 = "create proposals, never merge"; stage-8 status today = "◐ external fleet; pyxis-native engine not built" — this feature's slot |
| `docs/brainstorms/autonomous-developer.md` (gated ready-for-prd → argus) | the fleet seam's prior rulings: O-4 (nativize stages 8–10 = separate future work), D-8 (adapter-neutral seam; agent never self-implements), O-3 (fleet consumed via tracker), the principal/hands split this feature sits under |
| `docs/prds/autonomous-implementation.md` (IMPL-1…10) | the stage-8 requirement corpus, as-built-in-harbormaster — the placeholder requirement source this feature's PRD will cite |
| `docs/prds/design-and-planning.md` (PLAN-6) | the naming overlap: `/feature-implement-local` already means "label sub-issues for the autonomous pool" (async) — a collision to navigate |
| `docs/domain-model.md` §3.2–3.4 | the stage-8 entity catalog marked for engine transformation (Worker/Pass/Lease/AgentRuntime as Ports; Node kind-blind); `local-agent` = routing + merge-authorization |
| `docs/tech-design/{fleet-bridge,argus-agent,merge-lane,wake-channel,plan-new}-v1.md` | the frozen contracts this executor must honor (DINV-6 principal/hands, DINV-5 never-self-merge, DINV-1 one-projection-path, FR-10 re-pin hook) + the plan/charter it consumes |
| `~/agenticlab-ai/harbormaster/docs/{observability/incidents.md, design-principles.md, design/*}` | prior art + paid-for learnings (I2–I19, P0–P11): the design contract of what goes wrong when a fleet implements planned tasks |
| `docs/wiki/glossary.md` · `docs/tech-design/brainstorm-record-model.md` + template | established vocabulary (for the naming challenge) + this record's own contract |

**Overlaps surfaced:**
- `brainstorm:autonomous-developer#O-4` — **not covered; explicitly deferred**: "nativizing stages 8–10 is separate future work; Argus binds to them by role when they ship." This feature *is* the O-4 stage-8 nativization.
- `brainstorm:autonomous-developer#D-8` + Parked "fleet-seam adapter boundary" — the fleet seam is **adapter-neutral**; GitHub + `local-agent` + harbormaster = one concrete binding. This feature is a **second concrete binding** of the same seam.
- `brainstorm:autonomous-developer#O-3` / `#D-8` — the executor is the **hands**, never the Argus principal ("the agent never self-implements fleet-tagged work — a known gen-1 race class"); coexists with the external fleet via the shared tracker claim.
- `PLAN-6` (`/feature-implement-local`) — **naming collision only**: the corpus term means the *async* node pool; this synchronous same-session shape needs a distinct name (open decision).
- `IMPL-1…10` (autonomous-implementation) — the stage-8 requirement precedent; this feature is a **local-synchronous specialization** of it (reuses the single-machine parts, drops cross-machine leases/fairness).

## Framings

*Three genuinely distinct theses of what `/implement local` is. The operator picks or hybridizes; the Destination records the choice.*

- **A — The local fleet node.** `/implement local` is a *second concrete binding of the adapter-neutral fleet seam* (per `brainstorm:autonomous-developer#D-8`): a pyxis-native node that happens to be this laptop, claiming `local-agent`-tagged WorkItems exactly like harbormaster's nodes, only running synchronously in the foreground on demand. Thesis: *the domain model already says nodes are kind-blind (laptop/box/container indistinguishable) — "local" is just a node kind; the sole novelty is the synchronous, in-session, on-demand runtime.* Implications: maximal reuse of the IMPL-* corpus + AgentRuntime port; the "local vs fleet" split nearly vanishes; the feature is "the first pyxis-native executor, in its simplest deployment." Opens the pyxis-native Pass/Worker/Lease/AgentRuntime engine, with harbormaster as the parallel binding.
- **B — The same-session dev loop.** `/implement local` is a *developer-facing, foreground, synchronous implementation command*: having just designed and planned in this session, the operator types `/implement local` and watches tasks get built here and now — live progress, their own machine and identity, immediate feedback. Thesis: *the point is the tight, observable, same-session loop — design → plan → implement without leaving the session or waiting on an async fleet.* Implications: emphasis on foreground UX, live reporting, operator identity/credentials, human-at-hand; closer to harbormaster's "engineer workbench" than its unattended fleet. Opens a synchronous drive-and-report loop.
- **C — The engine's execution core, bootstrapped locally.** `/implement local` is the *first increment of pyxis's native stage-8 engine* (the O-4 work): build the real Pass/Worker/Lease/Workspace/AgentRuntime engine in Go, and "local" is merely its first, simplest runtime (single machine, synchronous) before distributed/container runtimes arrive. Thesis: *don't build a throwaway local script — build the engine's execution core and run it locally first.* Implications: invests in durable ports, conformance suites, and resumability from the start; "local" is a deployment of the engine, not a separate feature; the distributed fleet becomes another runtime later. Opens the full stage-8 engine.

**Destination:** **C grounded by A** — build the pyxis-native stage-8 **execution core** (the `Pass / Worker / Lease / AgentRuntime / Workspace` ports the domain model already names for transformation), shipped with only the **local-synchronous adapters** (in-process lease, fresh-headless runtime, this machine, foreground), structured as a **kind-blind node binding** of the adapter-neutral fleet seam — so the external/distributed/container fleet later becomes another adapter set behind the same core, never a rewrite. B's same-session `/implement-now` command + live progress is the operator-facing *surface*, not a separate architecture. — Operator endorsed the framing after a worked example ("i like the idea") and confirmed it covers the four-CLI MVP requirement through the `AgentRuntime` port (D-1).

## Decisions

### D-1 — MVP backend scope: all four CLIs behind one AgentRuntime port

- **Question**: does C-grounded-by-A cover an MVP that must support Claude Code, Codex, Cursor CLI, and OpenCode?
- **Answer**: yes — and the four-CLI set is fixed MVP scope. All four run behind one `AgentRuntime` contract (`launch / await / kill / auth_check / tiermap(rung→model,effort)`), the seam the domain model already prescribes (IMPL-6, "the proven seam; generalize the pattern"); the engine core never learns which CLI ran. Standardize on **fresh-headless** across all four (codex `exec`, opencode `run`, cursor `-p --force`; Claude via `claude -p`/SDK) so every adapter is the clean exit-code shape and no warm-TUI settle-heuristic (the I18/I19 charter-bleed class) enters.
- **Reasoning**: operator-stated MVP requirement; the port is exactly what makes N backends an adapter/config concern rather than N engine code paths; uniform headless keeps the four adapters uniform and drops harbormaster's worst, most-recent bug class. Resolves the open backend-scope question toward multi-adapter-from-day-one.
- **Anchor**: 2026-07-15 · t2
- **Status**: live

### D-2 — the name: `/implement-now`, reserving "fleet" for the async path

- **Question**: what is the command called, and how do we disambiguate from the existing "local = async node pool" sense (F-7)?
- **Answer**: **`/implement-now`** — the sharper word. The distinguishing axis versus the harbormaster path is **synchronicity/attendedness**, not machine location (its pool is *also* local-machine): `/implement-now` = synchronous, foreground, in-session, on the invoking machine. The async/unattended/distributed path is named **fleet execution** (already a domain-model plane). The `local-agent` label keeps its name — it tags a work item automation-eligible, and both `/implement-now` and the fleet claim it (the coexistence hinge).
- **Reasoning**: operator chose `/implement-now` over keeping `/implement local`; "now" encodes foreground/attended/synchronous unambiguously and sidesteps F-7 entirely — "local" never cleanly distinguished the two paths because the fleet pool is also local-machine; the real property is *when/how it runs*, not *where*.
- **Anchor**: 2026-07-15 · t3
- **Status**: live

### D-3 — the run boundary: stop at genuinely-green PRs (fix local + remote CI), never merge

- **Question**: where does an `/implement-now` run stop, and what does "done" mean per task?
- **Answer**: **Option A** — claim → implement → open a PR per task → **drive it to genuinely green** → report; never merge (landing is downstream: the human, merge-lane AUTO once activated, or argus — see O-1). "Green" is the real bar (P6): the run fixes failures on **both the local gate and remote CI** before a task counts done — a local build/test/lint gate that **mirrors required CI** (I9/I17), plus a **bounded** remote-CI self-heal loop (fix → re-push → re-check), capped then escalated to the human (P5/P7).
- **Reasoning**: operator's ruling; A honors trust-level-2 (F-1) + merge-lane DINV-5 (F-8) and keeps the attended run fast on landing, while "done = CI-green" avoids the merely-opened-but-red class (I9 red-unowned PR blocking a critical path ~2h; I17 red-classifier merged) — verified-done ≠ works.
- **Anchor**: 2026-07-15 · t4
- **Status**: live

### D-4 — tracker relationship: full tracker-mediated (an executor binding, not a new work substrate)

- **Question**: where does the executor sit relative to the GitHub tracker — bypass, hybrid, or full tracker-mediated like the external fleet?
- **Answer**: **full tracker-mediated** — `/implement-now` projects the frozen plan via `plangate project` (unchanged, fleet-bridge DINV-1) and **claims the `local-agent`-tagged GitHub issues exactly like the external fleet** (git-ref claim CAS + the projected `fs-spec:v1` markers). The **only** difference from harbormaster is synchronous same-session execution on this machine. So it's a new *executor binding* of the adapter-neutral automated-implementation-tag→executor seam (`brainstorm:autonomous-developer#D-8`), not a new work substrate — and it coexists with the external fleet for free (the shared claim CAS prevents double-grab).
- **Reasoning**: operator's ruling (pre-session AskUserQuestion). Maximally invariant-preserving — reuses the one projection path (DINV-1), the per-`plan_task_id` idempotency/verification, and the tracker convention that *is* the whole fleet interface; the executor becomes literally another consumer of the same tagged items.
- **Anchor**: 2026-07-15 · pre-session ruling (AskUserQuestion)
- **Status**: live

### D-5 — reusability structure: a self-contained single-task worker + a separable dispatch layer

- **Question**: sequential or bounded-parallel for the MVP — which keeps the code most reusable toward the remote/Docker fleet?
- **Answer**: decide by **portability, not a concurrency count** (operator delegated: "do whatever still gives reusable code that turns cleanly into the remote/Docker implementation"). The reusable core is a **self-contained, single-task, stateless worker** — given one task ref + repo access it runs the whole claim → worktree (off fresh `origin/main`) → implement → drive-to-green-PR → release cycle with no reliance on session memory or sibling tasks (truth on disk + tracker, IMPL-5); it ports to a remote node / Docker container **unchanged** (the fleet is just N of them, one task each — harbormaster's single-issue-worker-runtime). **Concurrency lives in a separable dispatch layer, never in the worker**: local = in-process K-slot scheduler; remote = N nodes. Local default = **bounded-parallel** (config; sequential a pure flip), so the local build exercises the claim-CAS + worktree-isolation seams the remote fleet reuses (dogfooding concurrency-correctness).
- **Reasoning**: operator's reusability steer, recorded as a delegation with my recommendation as content (skill rule). Keeping the worker concurrency-agnostic + self-contained is what makes local the first runtime and remote another dispatch adapter, not a rewrite (Destination). The claim is the same git-ref CAS the fleet uses (D-4); a local in-process guard is only an optimization against two local workers racing before the CAS.
- **Anchor**: 2026-07-15 · t5
- **Status**: live

### D-6 — consumers & segments: operator (attended) and argus (unattended) drive one core; the anti-segment needs hardware/fleet separation

- **Question**: is `/implement-now` only for attended operator use, or also unattended — and who is the anti-segment?
- **Answer**: **both** (operator correction). Primary segments: the **operator directly** (attended, watching), and **argus** (unattended, driving `/implement-now` as its **hands** — argus is principal, never self-implements, DINV-6). Attendedness is orthogonal — the workers are always autonomous (headless, bypass-permission); "attended" means only that a human observes/can intervene, not that a worker needs input. **Anti-segment**: a user/workload that **requires hardware separation or fleet umbrella policies** (isolated/other machines, container security posture, org/fleet governance) — served by the remote/Docker fleet, not `/implement-now` (D-7).
- **Reasoning**: operator's correction — unattended is served via argus, another executor binding argus drives through the tracker convention exactly like the external fleet (D-4). The anti-segment is defined by the hardware/policy separation the remote runtime provides, not by attendedness.
- **Anchor**: 2026-07-15 · t6
- **Status**: live

### D-7 — the local↔remote distinction: hardware separation + umbrella policies are the remote fleet's value-add (out of scope here)

- **Question**: what actually distinguishes `/implement-now` from the remote/Docker/harbormaster approach — and what is therefore out of scope?
- **Answer**: the remote/harbormaster-node approach adds two things `/implement-now` deliberately does **not**: (1) **hardware separation** — isolated/other machines or containers, the deny-by-omission container security posture, dedicated resources/host isolation; (2) **umbrella policies** — org/fleet-level governance a harbormaster-style node layer enforces (managed-set/org policy, risk & size caps, resource limits, node leases/heartbeats). `/implement-now` runs on the **same hardware under ambient/local policy**; both layers are the remote runtime's job, wrapped over the *same shared worker core* (D-5). → O-2, O-3.
- **Reasoning**: operator's framing of the real delta. Sharpens the non-goals precisely and confirms the reusability bet — the remote fleet = shared worker core + dispatch layer + hardware isolation + umbrella policy, added additively, never a rewrite.
- **Anchor**: 2026-07-15 · t6
- **Status**: live

### D-8 — the problem: no pyxis-native executor; the only path is the hardware-separated, policy-governed fleet

- **Question**: what problem does `/implement-now` solve?
- **Answer**: pyxis has **no native executor** — the only way to turn a frozen, projected plan into green PRs is to stand up the **external harbormaster fleet**, which requires hardware-separated nodes + fleet umbrella policies. There is no lightweight, pyxis-native way to execute a plan **on the same hardware as the session, under ambient/local policy** — whether the operator drives it attended or argus drives it unattended. `/implement-now` is that native execution core (stage 8, local runtime).
- **Reasoning**: operator adopted as written; grounded in F-1 (stage 8 = "◐ external fleet; native not built") and the D-7 boundary (the fleet's value is hardware separation + umbrella policy, not the execution itself).
- **Anchor**: 2026-07-15 · t7
- **Status**: live

### D-9 — the core job: turn a frozen plan's tasks into green reviewable PRs, on this machine, now

- **Question**: what is the primary job-to-be-done?
- **Answer**: *"I have a frozen, projected plan — turn its tasks into green, reviewable PRs on this machine, now, without provisioning a fleet or leaving the session."* For argus: the same job, driven unattended.
- **Reasoning**: operator adopted as written; flows from D-8 (the need) and D-3 (deliverable = genuinely-green PRs).
- **Anchor**: 2026-07-15 · t7
- **Status**: live

### D-10 — the bet & observable success

- **Question**: what does the program bet on, and how will we know it worked?
- **Answer**: the bet — pyxis closes the last big pipeline gap by turning a frozen plan into green PRs with its **own** execution core, starting local, with a worker that ports to the remote/Docker fleet unchanged. Observable success: (a) **dogfood** — `/implement-now` implements a real small pyxis feature end-to-end locally (every task reaches a genuinely-green PR), operator only reviewing/merging; (b) each of the **four CLIs** drives a task to green behind the one `AgentRuntime` contract; (c) **reusability check** — the worker core is exactly what a later remote/Docker runtime wraps, no rewrite.
- **Reasoning**: operator adopted. Dogfood-prove mirrors the argus program's D-10 and the plan-viewer precedent (a real feature as DoD evidence); the four-CLI + reusability checks bind D-1 and D-5's bets to observable outcomes.
- **Anchor**: 2026-07-15 · t8
- **Status**: live

### D-11 — appetite: the native local execution core; remote runtime & landing deferred

- **Question**: how much is in the MVP?
- **Answer**: build the **native local execution core** well — single-task worker + `AgentRuntime` (4 CLIs) + `LeaseStore` (git-ref CAS) + `Workspace` + a bounded-parallel local dispatcher; tracker-mediated (D-4); stop-at-green-PRs with local + CI self-heal (D-3); crash-resumable (D-5). **Deferred**: the remote/Docker runtime, hardware separation + umbrella policies (O-2/O-3), and merging/landing (O-1).
- **Reasoning**: operator adopted. Scope bounded by the reusability bet (build the portable core; defer the runtimes it later plugs into) and the frozen contracts (landing is merge-lane's — O-1).
- **Anchor**: 2026-07-15 · t8
- **Status**: live

### D-12 — failure disposition: skip-and-continue + escalate, never halt the whole run

- **Question**: when a task can't reach genuinely-green after the bounded local+CI self-heal cap (P-4), what does the run do?
- **Answer**: **skip-and-continue + escalate** — mark the task `worker-failed`, escalate (surface to the attended operator / report to argus's journal when unattended), leave its WIP branch + draft PR for a human or a later resumed run, and **continue the other independent ready tasks** (its dependents stay blocked per the DAG). The run finishes what it can and ends with a clear **succeeded / failed / blocked** summary; it never halts the whole run on one task's failure.
- **Reasoning**: operator adopted recommendation. Matches harbormaster (`worker-failed` + escalate, never halt the pass) and P7 (fail-closed on the task, not the run); preserves the independent work and gives the attended operator / argus a precise recovery surface.
- **Anchor**: 2026-07-15 · t9
- **Status**: live

## Facts

*Looked up, never asked — each with its source.*

- **F-1**: Stage 8 in the governing pipeline = Factory, per-task "implementation + testing + document updating + raising the PR — each proposal carrying its evidence package"; the operator's trust level 2 is "create branches and proposals; **never merge** (every merge stays human)"; stage-8 status today is "◐ external fleet; pyxis-native engine not built." — source: `docs/product-vision.md` §4/§137, `.pyxis/pipeline.md` stage 8
- **F-2**: This feature is the explicitly-deferred O-4 work — "nativizing stages 8–10 is separate future work; Argus binds to them by role when they ship." Not covered by any shipped capability. — source: `brainstorm:autonomous-developer#O-4`
- **F-3**: The fleet seam is adapter-neutral: a plan materializes as tracker items tagged for automated implementation; GitHub + `local-agent` + harbormaster is *one* concrete binding, carried by per-repo config. The principal (Argus) never self-implements fleet-tagged work — the fleet is the hands. — source: `brainstorm:autonomous-developer#D-8`, `#O-3`
- **F-4**: The stage-8 requirement corpus exists as-built-in-harbormaster (IMPL-1…10): gated idle≈0 tokens; supervisor pass with artifact-verified success + `hollow` detection; per-task cross-machine lease (CAS, dual-TTL, presence-anchored, fencing); isolated worker workspaces; WIP checkpoint/resume with checkpoint-and-exit cap; multi-CLI backends behind one adapter contract; capability rungs judged once; single-issue scheduler (K-way); janitors/self-healing; fair scheduling. — source: `docs/prds/autonomous-implementation.md`
- **F-5**: The domain model catalogs the stage-8 entities marked for engine transformation — **Worker** (Port, via AgentRuntime), **Pass** (→ engine loop), **Lease/Claim** (→ LeaseStore port), **AgentRuntime** (`launch/await/kill/auth_check/guard_shim/instructions_filename/tiermap` — "the proven seam"), **Node** ("kind-blind — laptop, always-on box, container; no code path may distinguish"). — source: `docs/domain-model.md` §3.2–3.4
- **F-6**: `local-agent` is *both* a routing signal *and* an **authorization token** ("= merge token") in the domain model's Signal entity. — source: `docs/domain-model.md:117`
- **F-7**: The predecessor's `/feature-implement-local` (PLAN-6) means "label sub-issues for the autonomous pool" — the *async* node fleet — a direct naming collision with this synchronous same-session shape. — source: `docs/prds/design-and-planning.md` PLAN-6
- **F-8**: Frozen pyxis contracts the executor must honor: **argus DINV-6** (principal never implements fleet-tagged work — executor is a distinct hands role); **merge-lane DINV-5** (no drive/session holds merge credentials; the classifier is author-agnostic, so a locally-authored PR routes identically); **fleet-bridge DINV-1** (one projection path — reuse `plangate project`) + §3.4's **FR-10 re-pin hook** as the sanctioned way to introduce a new stage-8 binding. — source: `docs/tech-design/{argus-agent,merge-lane,fleet-bridge}-v1.md`
- **F-9**: The consumption contract is the frozen plan JSON (`plan/v1`): per-task charter = `goal, current/desired, scope, out_of_scope, acceptance[], key_interfaces[], traces[], blocked_by[]` (a DAG), `foundation.level, size_band, plan_task_id` (idempotency key). `plangate project` renders each task to a GitHub issue + `fs-spec:v1` spec marker + route label, idempotent per `plan_task_id`, duplicate → halt (never auto-repair). Frontier = open ∧ unblocked ∧ unclaimed. — source: `docs/tech-design/plan-new-v1.md`, `internal/plangate/*`
- **F-10**: Harbormaster's paid-for learnings (incidents I2–I19; principles P0–P11): warm-session reuse causes same-session charter-bleed (I18/I19 — its current hardening frontier); two predicates that should be one drift into phantom loops (P1; I2/I3 ≈2M tok/hr); idle must gate at ≈0 tokens (P9); `cache_read` ≈86% of spend and grows with turn count → mechanical turn caps + checkpoint-resume; verify by artifact, not self-report (P3, `hollow`); every self-healing retry bounded then fail-closed escalate (P5/P7); risk-tiered landing, "verified-done ≠ works" (P6); inject + preflight credentials, else an unauthenticated child looks like a fast success (I8). — source: `~/agenticlab-ai/harbormaster/docs/observability/incidents.md`, `docs/design-principles.md`

## Fog

none *(session open)*

## Out of scope

- **O-1 — merging / landing**: `/implement-now` never merges; landing is a downstream concern (the human, merge-lane AUTO once activated, or argus). Opening genuinely-green PRs is the run's boundary. (from D-3)
- **O-2 — hardware separation**: running on isolated/other machines or in containers (host isolation, the deny-by-omission security posture, dedicated resources) is the remote/Docker fleet's value-add, not `/implement-now`'s — it executes on the same hardware as the invoking session. (from D-7)
- **O-3 — fleet umbrella policies**: org/fleet-level governance provided by harbormaster-style nodes (managed-set/org policy, risk & size caps, resource limits, node leases/heartbeats) is the remote fleet's layer, not `/implement-now`'s — it runs under ambient/local policy. (from D-7)

## Candidate glossary terms

- **T-1**: **in-session run** (the `/implement-now` command) — **synchronous, same-session, same-hardware** execution of a frozen plan's ready tasks by the invoking harness session under ambient/local policy; the invoker may be the operator (attended) or argus (unattended). Contrasted with **fleet execution** — asynchronous pickup by hardware-separated, umbrella-policy-governed nodes. · avoid: *local* (the fleet pool is also local-machine-capable) and treating *attended* as the defining trait (it's a mode — D-6); the axis is same-session/same-hardware vs. hardware-separated fleet.
- **T-2**: **umbrella policy** — fleet/org-level governance a harbormaster-style node layer enforces over workers (managed-set/org policy, risk & size caps, resource limits, node leases/heartbeats) — the remote runtime's value-add, distinct from a task's own **charter** and from argus's **authority contract**. · avoid: bare *policy* (overloaded — charter, authority contract, and fleet governance are all "policy").

## Parked technical constraints

- **P-1**: Per-CLI headless viability — codex (`exec`), opencode (`run`), and cursor (`-p --force`, but requires a pty) are natively headless one-shot with exit-code completion; Claude Code's headless mode (`claude -p` / Agent SDK) exists and supports autonomous runs (`--permission-mode bypassPermissions`, `--model`, tools, MCP, `--output-format stream-json`), **but harbormaster deliberately used the warm TUI and forbade `claude -p`** (reason unconfirmed — likely auth/priming amortization) → the design stage must confirm headless-Claude fully covers an autonomous multi-tool implementation run. The `AgentRuntime` adapter absorbs each CLI's quirks. — context: D-1
- **P-2**: CLI routing — the plan charter carries `rung` (capability tier) but **no per-task CLI hint** (F-9); "which of the four CLIs runs task T" is a routing decision (default backend + per-repo config + optional per-task override), like harbormaster's `routing.json (phase,type)→(backend,rung)`; native model IDs live only inside each adapter's `tiermap`. — context: D-1
- **P-3**: Dependency ordering under never-merge — how one run sequences a dependent task whose blocker's PR isn't merged yet (stack the PR on the blocker's branch · pause for the attended human to merge, then resume in-session · implement only the ready frontier and continue on re-invocation); a design-stage choice that doesn't move D-3's boundary. If stacking is chosen, the stacked-PR-train discipline applies (land bottom-up, delete each base branch on merge; incident I2). — context: D-3
- **P-4**: The "drive to green" machinery — the worker's **local gate must run the same deterministic checks as required CI** (I9/I17); the remote-CI self-heal is a **bounded state machine** (read CI via the Checks REST API not `statusCheckRollup` #829; feed **redacted + clamped** failed-CI logs to the fixer #989; cap attempts → flag failed + escalate #988/#707, P5/P7). The synchronous run's wall-clock includes **remote-CI round-trips** (minutes per cycle) — an attended await (poll or wake-channel). — context: D-3
- **P-5**: The dispatch/scheduler layer is the **local↔remote swap seam**. Shared below it: the single-task Worker, `AgentRuntime`, `LeaseStore` (git-ref CAS, per D-4), `Workspace`. Swapped above it: a local in-process K-slot scheduler ↔ the remote fleet scheduler across nodes. The local concurrency cap and the in-process anti-double-dispatch guard are local-dispatcher config, not core. — context: D-5
- **P-6**: Invocation/surface differs by driver over the *same* worker core — operator-attended: live progress + an intervention affordance; argus-unattended: headless, reports to the delivery journal, fail-closed escalation to argus. The worker's own autonomy (headless, bypass-permission) is identical in both. — context: D-6

## Gate

| Field | Value |
|---|---|
| **Verdict** | ready-for-prd |
| **Date** | 2026-07-15 |
| **Operator** | @mdarabi |
| **Covering source** | — |
| **Coverage check** | pass — all six inputs covered (§2.1 D-8 · §2.2 D-6 · §2.3 D-9 · §3.1 D-10 · §3.4 O-1/O-2/O-3 · appetite D-11), zero blocking fog |
