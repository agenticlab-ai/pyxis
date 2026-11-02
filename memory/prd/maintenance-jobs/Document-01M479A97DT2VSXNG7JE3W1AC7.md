# Requirement problems

The following operator review rulings revise the open contract before its
exact-head approval. They do not approve a lock, implementation, or migration.

### P-1 — FR-15 — Reuse all existing harnesses and native routing

**Raised by:** Operator review, 2026-10-05.
**Problem:** The initial v1 scope excludes other agent backends and freezes a Codex model/effort/flag profile even though Pyxis already supports four harnesses.
**Ruling:** @mdarabi · Pyxis already supports “4 different harness” and “a job can simply have the config for the harness and the routing”; the operator accepted the proposed capability/deliberation/context/assurance shape with “sounds good”. · Add FR-15 for Claude, Codex, Cursor, and OpenCode through existing runtime adapters and native routing. A job records its harness and semantic routing requirements; native model/effort selection and invocation translation reuse existing owners. Update scope, FR-2/FR-10, integrations, and verification consistently. Codex remains an explicit choice for the initial job.

### P-2 — FR-6 — Daily identity is job ID plus date

**Raised by:** Operator review, 2026-10-05.
**Problem:** Minute-occurrence evaluation and a startup grace window add scheduling machinery that daily-only v1 jobs do not need for deduplication.
**Ruling:** @mdarabi · Daily jobs in the near future need only “date+job_id” as their unique key. · Restrict v1 schedules to once per day. Scope the retained claim key to the repository remote, job ID, and date in the configured job timezone, derived from the invocation's captured start time. Remove minute matching and startup grace. Manual runs use the same daily key; attempted dates remain consumed and the next date stays eligible. Update FR-6/FR-7, UB-4, flows, and deferred scheduling scope accordingly.

### P-3 — FR-9 — Isolate configuration reads and all repository work

**Raised by:** Operator review, 2026-10-05.
**Problem:** Fetching published policy without switching the operator checkout does not by itself exclude writes to its shared refs or Git metadata.
**Ruling:** @mdarabi · A job modifying code or raising a PR outside its own worktree “can cause issue”. · Strengthen FR-8/FR-9 so policy fetches and claim transport use runner-owned Git storage before launch; only the winner receives a private independent clone for edits, tests, and supervisor-owned publication. Preserve the enrolled checkout's files, branch, index, configuration, and refs.

### P-4 — FR-11 — Required local JSONL audit journal

**Raised by:** Operator review, 2026-10-05.
**Problem:** Scattered run diagnostics do not provide the operator's requested single per-repository success/failure/outcome audit stream.
**Ruling:** @mdarabi · “ensure capturing the logs under .pyxis/maintenance/logs.jsonl” including success, failure, outcomes, and useful debug/audit information. · Require an append-only, schema-versioned local JSONL journal at that exact enrolled-repository path. Include pre-claim failures, installation and execution lifecycle, correlation/ownership, configuration and resolved route, terminal outcomes, publication and retained-evidence references. Serialize writers and exclude credentials. This explicitly requested path is the exception to the usual .pyxis/logs producer layout; it remains Git-ignored local diagnostics, with compact shared outcomes retained in the Git claim ref.
