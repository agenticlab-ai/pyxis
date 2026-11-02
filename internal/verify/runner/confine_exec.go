package runner

import (
	"fmt"
	"os"
	"os/exec"
)

// ConfineExecArg is cmd/codegate's hidden re-exec marker. main() checks
// os.Args[1] against it, before any normal subcommand parsing, and
// dispatches to RunConfinedExec instead — an internal implementation detail,
// never a documented CLI surface.
const ConfineExecArg = "__codegate_confine_exec__"

// confineWithLandlock rewrites cmd so that starting it launches this codegate binary in
// the hidden confinement mode instead of the real verb directly: argv
// becomes [self, ConfineExecArg, allowedPaths..., "--", cmd.Path,
// cmd.Args...] — the confinement wrapper (RunConfinedExec) splits on "--",
// restricts itself to allowedPaths, and execve's into (cmd.Path, cmd.Args).
// cmd.Path is carried through as a distinct token (rather than folded into
// cmd.Args[0]) so the real verb sees EXACTLY the argv it would have without
// confinement, including whatever argv[0] spelling exec.Command resolved.
func confineWithLandlock(cmd *exec.Cmd, allowedPaths []string, resolveExecutable func() (string, error)) (bool, error) {
	if !landlockEnforced() {
		// The Landlock write-scope leg (A-6) is OPT-IN, off by default, pending
		// Linux-CI-matrix verification of the toolchain-write allow-set (§3.5,
		// WriteAllowedRoots): a real `go`/`vet`/`test` verb writes its build cache
		// and temp files outside the workspace, and getting that allow-set exactly
		// right needs the Linux enforcement matrix the design pins A-6 to — not a
		// blind allow-list. Until then the Linux runner matches the macOS residual
		// (confine_other.go): the verb runs unconfined and no write-scope violation
		// is detected. The ruleset machinery below is complete and compiled; only
		// its default activation is deferred. Set CODEGATE_LANDLOCK=1 to enable it.
		return false, nil
	}
	self, err := resolveExecutable()
	if err != nil {
		return false, fmt.Errorf("landlock: resolve codegate's own executable path: %w", err)
	}

	wrapperArgs := append([]string{self, ConfineExecArg}, allowedPaths...)
	wrapperArgs = append(wrapperArgs, "--", cmd.Path)
	wrapperArgs = append(wrapperArgs, cmd.Args...)

	cmd.Path = self
	cmd.Args = wrapperArgs
	return true, nil
}

// runConfinedExec implements the hidden re-exec protocol: argv is everything
// after ConfineExecArg — one or more allowed-root paths, a bare "--", the
// resolved path to exec, then the real verb's own argv (argv[0] included).
// It restricts itself to the allowed roots and hands off via execve, which
// inherits the restriction. Successful execve replaces this process image;
// errors return to the Linux entry point, which reports and terminates the
// child. Setup failure never falls back to running the verb unconfined.
func runConfinedExec(argv []string, restrictPaths func([]string) error, execProcess func(string, []string, []string) error) error {
	roots, rest, ok := splitConfineArgv(argv)
	if !ok || len(rest) < 2 {
		return fmt.Errorf("malformed internal invocation: %q", argv)
	}
	execPath, realArgv := rest[0], rest[1:]

	if err := restrictPaths(roots); err != nil {
		return err
	}

	err := execProcess(execPath, realArgv, os.Environ())
	// syscall.Exec returns only on failure: success replaces this process
	// image and never returns control here.
	return fmt.Errorf("exec %q: %w", execPath, err)
}

// splitConfineArgv separates argv into the allowed roots and (the resolved
// exec path, the real verb's own argv), split on the first bare "--".
func splitConfineArgv(argv []string) (roots, rest []string, ok bool) {
	for i, a := range argv {
		if a == "--" {
			return argv[:i], argv[i+1:], true
		}
	}
	return nil, nil, false
}
