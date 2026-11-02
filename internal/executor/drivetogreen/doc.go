// Package drivetogreen waits for pending CI to settle without inference, then
// drives an explicitly-red proposal to genuinely-green: a local gate that
// mirrors required CI, and a bounded CI self-heal loop that repairs remote check
// failures, capped, then escalates
// (docs/tech-design/implement-now.md §3.5, §3.6).
//
// Two pieces, one predicate:
//
//   - The local gate (Gate) runs the SAME deterministic checks CI requires — the
//     check commands are injected config, so one source feeds both the gate and
//     CI, and a change failing a required CI check fails the gate on the same
//     input (prd:implement-now/FR-8, DINV-4, AP-11). It is one implementation,
//     shared by the pre-push filter and each re-push of the self-heal loop.
//   - The self-heal loop (Driver) first waits through pending/no-check states,
//     then on red fetches + REDACTS + clamps failed-CI logs before they reach a
//     fixer run (a fresh agent process — DINV-11), gates locally, revalidates the
//     proposal's landing/head identity, re-pushes, and waits for CI to settle, to
//     a configured attempt cap; at the cap it stops, flags the task failed, and
//     escalates (prd:implement-now/FR-7, FR-9, DINV-9, AP-16). Nothing loops
//     silently.
//
// The frozen executor ports (port.CodeHost, port.AgentRuntime, port.Workdir) are
// untouched: the CI-log source and the branch re-push are small INJECTED
// dependencies (LogFetcher, Pusher) so the loop stays hermetic-testable and the
// port surface does not grow. Redaction of the failed-CI logs before a fixer sees
// them is this package's responsibility (ABUSE-3, §3.5): CI content is data fed to
// a fixer, and a secret in a build log must never ride that egress.
//
// The Worker wires this in additively through worker.WithGreenDriver: a Worker
// built with a Driver settles pending CI or drives a red proposal to green before
// terminal classification; a Worker built without one keeps the base verify
// unchanged.
package drivetogreen
