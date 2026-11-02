package procgroup

import (
	"context"
	"errors"
	"os/exec"
)

// WaitOwned cleans a private session before reaping its direct child, keeping
// the session ID reserved throughout cleanup. The separate stop error lets a
// caller distinguish a failed acknowledgement from the command's exit status.
func WaitOwned(ctx context.Context, cmd *exec.Cmd, owner interface{ Kill() error }) (waitErr, stopErr error) {
	if exit, ok := owner.(interface{ WaitExit(context.Context) error }); ok {
		waitErr = exit.WaitExit(ctx)
		stopErr = owner.Kill()
		if waitErr != nil && stopErr != nil {
			return waitErr, stopErr // no proof of exit: never enter an unbounded Wait
		}
		reapErr := cmd.Wait()
		if waitErr == nil {
			return reapErr, stopErr
		}
		return errors.Join(waitErr, reapErr), stopErr
	}
	// Other platforms and injected ownership seams retain the ordinary wait.
	waitErr = cmd.Wait()
	if owner != nil {
		stopErr = owner.Kill()
	}
	return waitErr, stopErr
}
