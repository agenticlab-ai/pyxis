# PRD — Fleet Operations

**Status: draft v0.1 (baseline inventory — as built in harbormaster) · 2026-07-01**
Part of the [Pyxis PRD set](README.md); serves the [product vision](../product-vision.md). Northstar category: C11 (substrate), C3
(runtime). The machinery under pipeline stages 8–11.

## Scope

Provisioning, running, observing, and retiring the machines that execute the
factory: containerized nodes, warm supervisor pools, liveness, health, and
multi-machine collaboration. Users: the **operator**.

## Features

### FLEET-1 — Containerized execution node
**What:** A node runs entirely inside a container; one command stands it up.
**Requirements (as built):**
- Multi-arch (amd64+arm64) published image + build-local fallback; **one-line fresh-machine bootstrap**: pull image → credential wizard (terminal-native) → long-lived auto-restart container.
- State persists on a named volume across container recreation (identity, credentials, clones) — same node identity, no duplicate work after an upgrade; secrets injected at runtime, never baked into the image.
- First-run agent-CLI gates (onboarding/trust/permissions) are pre-seeded so the pool runs unattended from boot.
- Host-bound observability directory (events/logs readable host-native); in-container **monitor** dashboard + log tailing; `upgrade --pull` refreshes the image keeping state.
- Graceful teardown: release held leases + deregister presence *before* container removal; state volume kept unless purged.
**Origin:** containerized-node.md; node-build-time-install.md; node-container.sh.

### FLEET-2 — Supervisor pool + watchdog
**What:** N always-ready execution slots per node, self-healing.
**Requirements (as built):**
- N warm interactive supervisor sessions idle at prompt (~0 tokens); a mechanical watchdog every ≤5 min: revives crashed or limit-blocked members, runs maintenance + the gate, handles auth outages (reset + alert instead of context-flooding retries).
- Per-node launch policy from config: model pin, pass effort, permission mode, remote-control opt-in; re-asserted across restarts and session clears.
- **Quota- and rate-limit-aware:** a usage-quota floor is checked before listing work, transient rate-limit errors fast-path a session nudge rather than a full cycle, and limit-blocked sessions are revived when quota returns.
- Managed repos are cloned **on demand** the first time they have ready work (an empty workspace root on a fresh node is by design).
**Origin:** interactive-supervisor-pool.md; supervisor-tmux.sh; quota-aware iterations (#162); ensure-clone (#95/#109).

### FLEET-3 — Node identity, presence & telemetry
**What:** Every node is identifiable, observable, and safely retirable.
**Requirements (as built):**
- Identity: stable machine id + volume-seeded instance id (the lease/heartbeat key, container-recreate-stable) + operator-set display name.
- A per-node liveness beacon updated every tick carrying an identity+state snapshot; stale-warn (~30m) and reap (~48h) thresholds; graceful deregister on stop/uninstall so retired nodes vanish rather than linger.
- Richer telemetry snapshot (activity, resources, throughput, health, queue) published per tick; a fleet-silence observer files a deduped operator issue when the whole fleet goes quiet.
- **Kind-blind uniformity:** no code path distinguishes laptop vs always-on vs container — behavior keys on observed state only (presence freshness, power, slots).
- **Multiple isolated node instances per host:** per-instance volume, observability path, and name; startup reclaim of the instance's own prior leases; the provenance tuple (instance id, container id) is stamped into events and worker proposal trailers.
- Telemetry is an **optional sink** (a dedicated telemetry repo): the heartbeat is the degenerate snapshot when the sink is absent and everything degrades gracefully; full snapshots include platform resource probes (cpu / mem / disk).
**Origin:** fleet-telemetry.md; fleet-heartbeat-lifecycle.md; uniform-node-leases.md; multi-instance nodes (#820); telemetry sink (#931/#1065).

### FLEET-4 — Fleet status
**What:** One read-only command answers "what is the fleet doing?" (`/rig-status`).
**Requirements (as built):**
- Heartbeat freshness, lifecycle-aware fleet table of live nodes, idle-vs-working token-spend per pool member, active leases per repo, pending routed work, flagged proposals (worker-failed / merge-conflict / verify-pending), one-line count footer; `--fleet` renders the full telemetry dashboard. Mutates nothing.
**Origin:** rig-status skill.

### FLEET-5 — Doctor (node & fleet health)
**What:** Continuous verification that a node is actually able to work.
**Requirements (as built):**
- Cheap per-tick checks + a daily full audit: code parses, credential helper reachable (self-healing repoint), key present with correct mode, watchdog fresh, pool members alive, lease-lag (expired-but-unreaped), peer liveness, frozen rig clone (detect + auto-restore), work-level signals (unclaimed-ready, starvation).
- Engineer-workbench variant: tool versions vs pins, symlink integrity (self-healing), pre-push hook present, ambient identity is human, clean main.
- Output classes PASS / WARN / NEEDS-OPERATOR; repeated non-mechanical findings file a deduped operator issue.
**Origin:** doctor subcommand; fleet-hardening.md.

### FLEET-6 — Install / uninstall lifecycle
**What:** Machine onboarding and retirement are one command each, reversible,
and honest.
**Requirements (as built):**
- Install: one-liner node bootstrap (container) or workbench install (engineer); shared library; single-source version pins for every pinned tool; idempotent re-runs converge.
- Uninstall: conservative by construction — removes only what install placed (symlinks verified by target before removal, newest backup restored), graceful fleet deregistration, credentials deleted only after a second explicit confirmation; `--dry-run` previews everything; re-run is a safe no-op.
**Origin:** install-parameterization.md; node-bootstrap.sh; uninstall.sh.

### FLEET-7 — Multi-machine collaboration
**What:** Any number of nodes share one org's work safely with no coordinator.
**Requirements (as built):**
- Correctness from three primitives: the per-task lease mutex, the presence namespace, and idempotent+raceable merges. Machines join/leave freely; work redistributes via lease lapse + reap; no central scheduler exists.
**Origin:** multi-machine-rig.md.

### FLEET-8 — Rig self-update
**What:** Nodes keep their own factory code current.
**Requirements (as built):**
- Every tick fast-forwards the factory clone (so skill/gate updates reach running loops); a clone left off-main (e.g. by a crashed worker) is detected, frozen-state alarmed, and auto-restored when safe; org policy syncs fleet-wide the same way.
**Origin:** sync-rig (harbormaster #8); uniform-node-leases.md §4.8.

### FLEET-9 — Org portability (zero-edit fork)
**What:** The whole factory installs into **any** GitHub org without source edits.
**Requirements (as built):**
- The org is resolved from the install origin (overridable per flag/config); the bot-App slug and installation IDs are resolved dynamically at runtime (dormant-safe on machines with no App key); org-level instruction files are templated with org substitution, never copied verbatim.
- No hardcoded org literals anywhere — enforced by a portability-guard test suite wired into CI.
- A move-to-another-org runbook covers migrating an existing install.
**Origin:** install-parameterization.md (#377); portability hardening (#958–964).
