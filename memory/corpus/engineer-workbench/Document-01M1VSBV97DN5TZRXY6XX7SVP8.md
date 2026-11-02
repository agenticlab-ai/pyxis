# PRD — Engineer Workbench

**Status: draft v0.1 (baseline inventory — as built in harbormaster) · 2026-07-01**
Part of the [Pyxis PRD set](README.md); serves the [product vision](../product-vision.md). Northstar category: C3 (authoring
environment). Pipeline stages 1–5 + 9 (the human-facing surfaces).

## Scope

What a human engineer sits inside: a one-command native workbench with the
factory's skills, gates, and review surfaces, on the engineer's own identity
and choice of agentic CLI. Users: **engineers**.

## Features

### BENCH-1 — Workbench install
**What:** Engineer onboarding is one command (`dev-install.sh`).
**Requirements (as built):**
- Native install (not a cloud devcontainer), **own identity** (never the fleet bot's credentials or key), clones all accessible org repos, links the factory's skills/scripts/commands, pins tool versions from the single source (versions.env) with preflight checks.
- A dedicated doctor mode (`doctor --engineer`) verifies the workbench: tool versions vs pins, symlink integrity (self-healing), hook presence, ambient identity is human, clean main.
- Coexists safely with a fleet node on the same machine (separate scripts, no shared auth).
**Origin:** engineer-workbench.md (epic #763).

### BENCH-2 — Pre-push gate
**What:** The quality gate rides git itself, regardless of which CLI edited files.
**Requirements (as built):**
- A git pre-push hook runs the same local CI mirror (GATE-2) before anything leaves the machine — the universal chokepoint that makes multi-CLI safe: no matter which agent or editor produced the change, the gate runs.
**Origin:** engineer-workbench.md §pre-push.

### BENCH-3 — Multi-CLI authoring environment
**What:** The engineer chooses the agentic CLI; the factory wires whatever is installed.
**Requirements (as built):**
- `--cli auto` detects installed CLIs and wires each: Claude Code and Cursor (CLI + IDE) supported today; guard hooks translated per CLI with **fail-closed** enforcement (Cursor's fail-open default is explicitly overridden); canonical instruction files (AGENTS.md standard, CLAUDE.md for Claude) so repo knowledge is CLI-neutral.
- *(Pyxis hardens this into AP-4: skills/hooks authored once, portable across all agentic CLIs and to user machines.)*
**Origin:** engineer-workbench-cursor.md (epic #789); cross-CLI portability verdict.

### BENCH-4 — Rendered review surface
**What:** Substantial documents are reviewed in a browser with inline
annotations, not as terminal text.
**Requirements (as built):**
- A vendored, version-pinned review server (never fetched-and-executed at runtime; deps installed at workbench setup from a committed lockfile) renders markdown via a codified transform and serves an annotate→apply→reply loop the agent drives end-to-end — zero manual commands for the reviewer.
- Degrades gracefully (static-open → render-only) with the reason logged; wire-format contract tests pin the API against version bumps.
- Hard-won constraints honored: vendor un-bundled + install-time deps (never runtime bundling), bash-3.2 floor for driver scripts, exec-verify vendored artifacts.
**Origin:** design-review-self-host.md (epic #797, #855); lavish skill.

### BENCH-5 — Session handoff (`/transfer`)
**What:** Distill a live working session into a single, self-contained,
paste-ready handoff prompt for a fresh agent instance — same or another machine.
**Requirements (as built):**
- Gathers live state read-only (host, cwd, branch, uncommitted work, recent commits, stash), every probe failure-tolerant; folds the load-bearing specifics — issues/proposals in flight, decisions made, dead ends ruled out, the single immediate next action — into one block a context-free reader can act on.
- Optional focus/target arguments; optional save-to-file mode; distributed to every machine as a user-level command.
**Origin:** transfer-command.md (#253).
