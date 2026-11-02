// implement-now is the pyxis-native local execution engine's thin entrypoint —
// the /implement-now surface (docs/tech-design/implement-now.md §3.9). It loads
// the executor config, wires the five ports (gh tracker + code host, git
// worktree workspace, git-ref CAS lease) plus a per-task backend resolver and the
// drive-to-green driver, and runs ONE frontier-sweep over the frozen plan's ready
// tasks — opening genuinely-green proposals and reporting the blocked set. It
// never merges and never stacks; landing and re-invocation belong to the driver
// (the attended operator, or argus unattended). One normalized event stream feeds
// two sinks: a live operator view and the argus JSONL delivery journal with
// fail-closed escalation.
//
// The whole surface is vendor-neutral (AP-25): the four CLI backends self-register
// (backends.go), and the authoring guide + skill glue live under .pyxis/. This
// main is intentionally thin — the wiring is in run.go, all of it exercised
// hermetically by the report/dispatch tests.
package main

import "os"

var mainExit = os.Exit

func main() {
	mainExit(run(os.Args[1:], os.Stdout, os.Stderr))
}
