//go:build windows

package runner

import "os/exec"

// setpgid is a no-op on windows: this package has no process-group
// equivalent here, so a verb that spawns its own children may leave orphans
// behind a kill — a named residual (design:codegate-v1#3.5 scopes enforced
// write-scope and full subtree control to the Linux CI matrix; the operator's
// attended macOS surface already carries a residual, and windows is not a
// verification target platform — this file exists only so `go build ./...`
// and `go vet ./...` stay green there).
func setpgid(cmd *exec.Cmd) {}

// killGroup kills the direct child only. Go exposes no SIGTERM equivalent on
// windows (Process.Signal accepts only os.Kill), so the bound's first phase
// is already a hard kill; force is accepted for signature symmetry with the
// unix implementation but does not change the behavior.
func killGroup(cmd *exec.Cmd, force bool) {
	if cmd.Process == nil {
		return
	}
	// Best-effort, like the unix sibling: Kill returns an error only if the
	// process is already gone, which is the desired end state — nothing to do
	// with it.
	_ = cmd.Process.Kill()
}
