// Package ghcodehost implements the executor's CodeHost port against GitHub
// (docs/tech-design/implement-now.md §3.1, §3.10 — the S-6 seam). It opens a
// pull request from a task's isolated head branch and reads that proposal's
// required-check states through the Checks API — an exact, fresh read of the
// head commit's check runs, never a status rollup badge (AP-13, AP-15).
//
// The executor NEVER merges (prd:implement-now/NR-3, DINV-6): this adapter only
// opens proposals and reads their state. OpenProposal is idempotent — an
// existing open proposal for the head is returned as-is, never duplicated
// (prd:implement-now/FR-14, AP-22) — and returns port.ErrNoProposal when the
// run produced nothing to propose, which the Worker classifies as hollow
// (DINV-3). Publishing the worktree branch to the remote is the Workspace's
// responsibility (§3.1); this adapter assumes the head branch is already
// pushed.
//
// All GitHub interaction sits behind a narrow, consumer-side client interface
// (client). The production implementation shells an authenticated `gh api`
// (client.go); the adapter's own logic — the idempotent-open flow, the
// nothing-to-propose detection, and the fail-closed check aggregation — is
// unit-tested against a fake client with recorded responses, so it is hermetic.
package ghcodehost
