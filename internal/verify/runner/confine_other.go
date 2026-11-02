//go:build !linux

// Package runner: confine_other.go is the named residual (design
// codegate-v1 §3.5, risk R6) for every platform besides Linux: no
// self-contained, kernel-level write-scope enforcement is available here.
// macOS (the operator's attended dogfood surface) has no Landlock
// equivalent this harness can call without an external sandbox runtime,
// which NFR-3 rules out (design §4, "External sandbox runtime (bwrap-class)
// ... rejected"); windows was already outside the verification target
// platforms before this design (see runner_windows.go). A verb here can
// write anywhere its own OS-level permissions allow — DINV-5's env-strip and
// wall-clock bound still apply unconditionally, but the write-scope
// guarantee is Linux-only, and A-6's write-scope leg is pinned to the Linux
// CI matrix accordingly (design §3.5, §6 R6).
package runner

import (
	"fmt"
	"os"
	"os/exec"
)

var unsupportedConfineExit = os.Exit

// confine is a no-op here: cmd is returned unrewritten, and the caller must
// never scan its output for a write-scope violation signature (confined ==
// false) since no enforcement was attempted to violate.
func confine(cmd *exec.Cmd, allowedPaths []string) (bool, error) {
	return false, nil
}

// RunConfinedExec would apply write-scope confinement and re-exec into the
// real verb; on this platform that is structurally impossible (no
// self-contained enforcement — see the package doc), so reaching this
// function at all means something invoked the internal re-exec marker
// directly, which is not a supported entry point here.
func RunConfinedExec(argv []string) {
	fmt.Fprintln(os.Stderr, "codegate: internal confinement re-exec is not supported on this platform")
	unsupportedConfineExit(1)
}
