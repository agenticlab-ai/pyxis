//go:build !unix

package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
)

// execTool on platforms without execve: run the verified binary as a child
// wired to this process's std streams and relay its exit code (a
// signal-killed child relays 1). This is the documented fallback the T4
// charter names — the design's platforms are darwin/linux, where
// exec_unix.go's true execve applies instead. argv keeps execve semantics:
// argv[0] is the program name, argv[1:] the arguments.
func execTool(path string, argv, env []string) (int, error) {
	cmd := exec.Command(path, argv[1:]...)
	cmd.Env = env
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	err := cmd.Run()
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		if code := ee.ExitCode(); code >= 0 {
			return code, nil
		}
		return 1, nil // killed by a signal: no exit code to relay — nonzero, honestly
	}
	if err != nil {
		return 0, fmt.Errorf("run %s: %w", path, err)
	}
	return 0, nil
}
