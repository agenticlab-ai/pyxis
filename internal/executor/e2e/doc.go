// Package e2e is the pyxis executor's end-to-end verification and reusability
// witness — task T14 of the implement-now plan
// (docs/tech-design/implement-now.md §3.10, A-9, A-12). It composes the REAL
// engine — dispatch.Dispatcher + worker.Worker + drivetogreen.Driver +
// report (both sinks) + config — and drives a multi-task fixture plan through
// frontier-sweeps. Nothing here is production code: the whole package is the
// verification harness, so its behavior lives in the _test.go files.
//
// The stub boundary is TWO of the five ports, not five. Stubbed are the media
// that cannot run in-session: GitHub (WorkTracker's issue API and CodeHost's
// pull-request + Checks API) and the coding-agent CLI (AgentRuntime — the model
// call). Everything else runs as the PRODUCTION adapter over a real local git
// world (a bare origin plus the executor's parent checkout, on disk, no network):
//
//   - workspace.Workspace cuts a genuine linked worktree per task off the
//     freshly-fetched mainline tip, under the real parent-checkout commit guard,
//     and publishes with a real `git push` (DINV-1, DINV-2);
//   - lease.Lease claims by a real create-or-fail ref CAS on the shared origin
//     and releases by a real guarded ref delete (DINV-5);
//   - the drive-to-green re-push is a real force-with-lease push, and the code
//     host answers every question by reading the pushed head.
//
// That last point is what the git substrate buys: "green" means the artifact on
// a branch's real published head, so work that was committed but never pushed
// cannot read green, a task branch cut from a sibling instead of mainline is
// visible as real ancestry, and a released claim is a ref that is actually gone.
// The only external requirement is a `git` binary on PATH — the same dependency
// the workspace and lease adapter suites already carry.
//
// What the drill proves (hermetic, in-session — the strongest dogfood proxy A-12
// admits without a live CLI/network):
//
//   - The whole stack COMPOSES end to end. A frozen fixture plan (a foundation
//     plus two parallel leaves with a blocked_by edge) runs through real
//     frontier-sweeps: the foundation runs first, the driver lands it, the next
//     sweep runs the leaves concurrently to the configured width, and each task
//     reaches a genuinely-green proposal — one of them by the real bounded
//     drive-to-green loop (a red proposal healed to green through the real Gate +
//     self-heal state machine). The classed RunReport is correct at every sweep.
//   - ONE normalized event stream reaches BOTH sinks (the operator live view and
//     the argus JSONL journal) identically (AP-2): run/task lifecycle events land
//     on both, and a hollow task is classed failed-never-succeeded on both
//     (DINV-3).
//   - Re-invocation is IDEMPOTENT (AP-22): a repeated sweep over the same ready
//     frontier resumes from the persisted worktree and opens NO duplicate
//     proposal and fires NO new self-heal attempt — the killed-and-resumed
//     convergence the frontier-sweep model rests on (§3.6 step 5).
//   - The Worker core is RUNTIME-AGNOSTIC (A-9, DINV-10, NFR-5). The same
//     worker.Worker and the same five ports run unchanged behind a second,
//     "remote-shaped" dispatcher (a lane-pool driver that is deliberately NOT
//     dispatch.Dispatcher). The local↔remote reusability bet is witnessed: only
//     the driver around the Worker differs, never the Worker or its ports.
//
// What still requires a real, gated LIVE run (NOT covered hermetically — see
// live_test.go and the honest scope note in the T14 report):
//
//   - The four real AgentRuntime CLI adapters spawning fresh headless processes
//     that actually mutate code (A-4 — each adapter's own conformance test drives
//     that; here the AgentRuntime is a stub that "implements" a task by writing
//     and committing a file in its worktree). The stub stands at the SAME port the
//     real CLI adapters implement, so the composition it proves is real; the model
//     call itself is not.
//   - The real gh WorkTracker/CodeHost adapters against a live code host and CI
//     (their own packages test them; here they are stubbed at the port boundary,
//     answering from real local git state rather than from invented state).
//   - A true /implement-now dogfood turning a real pyxis feature into merged,
//     green PRs with the operator only reviewing/merging (A-12 proper). That is
//     the gated live drill in live_test.go, which SKIPS CLEANLY in the normal run
//     and documents exactly how a real dogfood is driven.
//
// In short: this package proves the engine's parts COMPOSE into a correct whole
// over real git; it does not — and cannot, in-session — prove the live external
// media (real CLIs, real GitHub, real CI) behave. That is §7's standing open
// question, discharged by the per-adapter conformance suites and the gated live
// drill, not by this harness.
package e2e
