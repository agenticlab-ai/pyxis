//go:build unix

package main

import (
	"fmt"
	"syscall"
)

// execTool replaces the process image with the verified binary — a true
// execve (installer-v1 §3.4: resolve → verify → exec). On success it never
// returns: the tool owns the process, its streams, its signals, and its
// exit code with zero relay. The (exit-code, error) shape exists for the
// paths that DO return — an execve failure here, and the !unix fallback in
// exec_other.go, which runs a child and relays its exit code. Stdlib
// syscall.Exec is the whole mechanism on the design's platforms
// (darwin/linux); no x/sys dependency.
func execTool(path string, argv, env []string) (int, error) {
	err := syscall.Exec(path, argv, env)
	// Reached only on failure — on success the image is already replaced.
	return 0, fmt.Errorf("execve %s: %w", path, err)
}
