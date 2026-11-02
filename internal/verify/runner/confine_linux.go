//go:build linux

// Package runner: confine_linux.go is the Linux write-scope enforcement leg
// (design codegate-v1 §3.5, DINV-5): a verb's process may write only beneath
// the allowed paths, confined by Landlock (kernel >=5.13) — self-contained,
// no external sandbox binary (NFR-3; §4 rejects the bwrap-class alternative).
//
// Landlock restricts a PROCESS, and the restriction is permanent and
// inherited across exec for as long as that process lives (landlock_lsm(7)).
// Applying it to codegate's own long-lived process would confine every
// later write THIS invocation makes for the rest of its life — its own
// scratch, its verdict, a falsification worktree several rungs later. So
// confinement runs in a throwaway child instead: confine() rewrites cmd to
// re-exec this same codegate binary into a hidden mode (ConfineExecArg,
// dispatched by cmd/codegate's main() before its normal subcommand switch)
// that applies Landlock to ITSELF alone, then execve's into the real verb —
// never codegate's own parent process.
package runner

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
)

func confine(cmd *exec.Cmd, allowedPaths []string) (bool, error) {
	return confineWithLandlock(cmd, allowedPaths, os.Executable)
}

// RunConfinedExec applies Linux confinement before replacing the child process.
func RunConfinedExec(argv []string) {
	kernel := nativeLandlockKernel()
	confineFail(runConfinedExec(argv, kernel.restrictSelfToPaths, syscall.Exec))
}

// confineFail reports reason on stderr with confineWrapperFailMarker
// (confine.go) and exits confineWrapperFailExitCode. This process never
// returns from here — the parent runner.Exec's finalizeConfinement
// recognizes exactly this exit-code-plus-marker pair as "the harness's own
// confinement machinery broke" (ClassInfra), never the verb's own red and
// never a silent unconfined fallback.
func confineFail(reason error) {
	fmt.Fprintf(os.Stderr, "%s%v\n", confineWrapperFailMarker, reason)
	os.Exit(confineWrapperFailExitCode)
}
