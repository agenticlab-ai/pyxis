//go:build !windows

package runner

import (
	"os/exec"
	"syscall"
)

// setpgid places the child in its own process group so a later killGroup can
// reach the whole subtree (e.g. `go build`'s compiler subprocesses), not just
// the direct child.
func setpgid(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

// killGroup signals the child's entire process group: SIGTERM for a graceful
// stop at the bound (force == false), SIGKILL once the grace elapses (force
// == true). A group with no living members returns ESRCH, which is ignored —
// killGroup is a best-effort nudge, not a certification that anything died.
func killGroup(cmd *exec.Cmd, force bool) {
	if cmd.Process == nil {
		return
	}
	sig := syscall.SIGTERM
	if force {
		sig = syscall.SIGKILL
	}
	_ = syscall.Kill(-cmd.Process.Pid, sig)
}
