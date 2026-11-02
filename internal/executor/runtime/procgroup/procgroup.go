// Package procgroup runs a worker subprocess in its own process group so the
// engine can signal the WHOLE subtree — a Node/Bun CLI such as `claude` spawns
// its own children — as a unit, not just the direct child it started.
//
// os/exec's default kill (and exec.CommandContext's async cancel) signals only
// the direct child PID. When a Worker is cancelled or bounded out, that leaves
// the CLI's own grandchildren orphaned — the concrete failure this package
// exists to prevent (a headless `claude` outliving the engine that spawned it).
// Placing the child in its own group and killing the group (a negative PID)
// reaches the entire tree.
//
// This is intentionally the same shape as the pattern internal/verify/runner
// already uses for code-gate check subprocesses (that package keeps its own
// unexported copy; the executor runtime cannot import it). Windows has no
// process-group equivalent here, so Setpgid is a no-op there and Kill falls
// back to the direct child — windows is not an execution-target platform for
// the engine; the build-tagged file exists only so `go build ./...` stays green.
package procgroup

import "time"

// DefaultGrace bounds the post-kill pipe drain (used as cmd.WaitDelay): after
// the group is signalled, cmd.Wait waits at most this long for the captured
// stdout/stderr pipes to close before forcing them shut, so an escaped
// grandchild that re-grouped and still holds a pipe write-end cannot wedge
// cmd.Wait forever. Mirrors the grace internal/verify/runner applies.
const DefaultGrace = 5 * time.Second
