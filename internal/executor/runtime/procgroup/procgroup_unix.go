//go:build !windows

package procgroup

import (
	"errors"
	"fmt"
	"os/exec"
	"syscall"
)

// Setpgid places the child in its own process group (its PGID becomes its own
// PID) so a later Kill can reach the whole subtree, not just the direct child.
// Call it after every other cmd field is set and before cmd.Start.
func Setpgid(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

// SetOwnedSession gives an invocation a private session as well as a private
// process group. Children that create another group remain in that session even
// if an intermediate parent exits before the lifetime observer sees it.
func SetOwnedSession(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
}

// Kill signals the child's entire process group: SIGTERM for a graceful stop
// (force == false), SIGKILL once any grace has elapsed (force == true). A group
// with no living members yields ESRCH, which is ignored — Kill is a best-effort
// teardown, not a certification that anything died. It requires the child to
// have been started with Setpgid; without it, the negative-PID signal would
// escape to the caller's own group, so Kill no-ops when there is no process.
func Kill(cmd *exec.Cmd, force bool) {
	_ = KillChecked(cmd, force) // existing best-effort teardown API
}

// KillChecked reports a failed signal so a supervisor cannot checkpoint while
// a known-live process may still be writing. An already absent group is stopped.
func KillChecked(cmd *exec.Cmd, force bool) error {
	if cmd.Process == nil {
		return nil
	}
	if cmd.Process.Pid <= 0 {
		return fmt.Errorf("refuse to signal invalid process group %d", cmd.Process.Pid)
	}
	sig := syscall.SIGTERM
	if force {
		sig = syscall.SIGKILL
	}
	if err := syscall.Kill(-cmd.Process.Pid, sig); err != nil && !errors.Is(err, syscall.ESRCH) {
		return fmt.Errorf("signal process group %d: %w", cmd.Process.Pid, err)
	}
	return nil
}
