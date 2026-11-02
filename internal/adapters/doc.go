// Package adapters is the port-axis home for medium implementations.
//
// Each adapter lives at adapters/<port>/<medium> and implements exactly one
// consumer-side port. Engine packages import ports only. Composition roots
// under cmd/ (and test composition roots) import adapters so they
// self-register — never a central vendor switch in core code (AP-1, AP-3,
// AP-27).
//
// Current tree:
//
//	adapters/harness/{claude,codex,cursor,opencode}  — AgentRuntime
//	adapters/spend/{claudecode,codex,cursor,opencode} — spend.Source
//	adapters/codehost/{github,forgejo}               — CodeHost
//	adapters/tracker/{github,forgejo,carina}         — executor WorkTracker + plan projection
//	adapters/persist/forgejo                         — impl-review persist + harvest
//	adapters/release/forgejo                         — Forgejo release/attachment host
//	adapters/common/github/{githubauth,ghidentity}   — shared GitHub support
//	adapters/common/forgejo                          — shared Forgejo HTTP transport
//
// Ports stay on the consumer side, not in this tree:
//
//   - AgentRuntime, CodeHost, executor WorkTracker: internal/executor/port
//   - spend.Source: internal/phasemeter/spend
//   - worktracker.WorkTracker (the local item store): internal/worktracker
//
// Not extracted yet: CI/checks still switch providers inside ciparity;
// Argus write-hook adapters are still files under argus/writehook;
// GitHub/Carina persist adapters still live under implreview/persist;
// worktracker/localfs stays beside its consumer-side port. Shared GitHub
// support is adapters/common/github; shared Forgejo transport is
// adapters/common/forgejo — neither is a port implementation.
package adapters
