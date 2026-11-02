// Package runtime hosts the machinery every AgentRuntime backend adapter reuses:
// a self-registration registry (name → factory, no central switch — AP-27), the
// ephemeral brief assembly handed to each fresh process
// (prd:implement-now/FR-19), and a black-box conformance harness that drives an
// adapter through the whole port.AgentRuntime contract
// (docs/tech-design/implement-now.md §3.7).
//
// The concrete backends live under internal/adapters/harness/{claude,codex,
// cursor,opencode} and self-register from their own init, so onboarding a CLI
// is a new package — never an edit to a shared dispatch table (AP-27). Each
// Launch spawns a fresh non-interactive process per task; no agent session is
// ever reused (DINV-11, the charter-bleed guard).
//
// This package imports no adapter package: the dependency is always
// adapter → runtime, so the resolver never grows a central import of every
// backend.
package runtime
