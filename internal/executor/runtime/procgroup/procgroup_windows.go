//go:build windows

package procgroup

import (
	"errors"
	"os"
	"os/exec"
)

// Setpgid is a no-op on windows: os/exec exposes no process-group equivalent
// here, so a worker that spawns its own children may leave orphans behind a
// Kill. Windows is not an execution-target platform for the engine; this file
// exists only so `go build ./...` and `go vet ./...` stay green there.
func Setpgid(cmd *exec.Cmd) {}

func SetOwnedSession(cmd *exec.Cmd) {}

// Kill terminates the direct child only: Go exposes no SIGTERM on windows
// (Process.Signal accepts only os.Kill), so force is accepted for signature
// symmetry with the unix implementation but does not change the behavior.
// Best-effort, like the unix sibling — an already-gone process is the desired
// end state.
func Kill(cmd *exec.Cmd, force bool) {
	_ = KillChecked(cmd, force) // existing best-effort teardown API
}

func KillChecked(cmd *exec.Cmd, force bool) error {
	if cmd.Process == nil {
		return nil
	}
	if err := cmd.Process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
		return err
	}
	return nil
}
