// Package reporoot locates the root of a pyxis-operated repository from the
// current working directory — the one discovery predicate every pyxis binary
// shares (AP-11), so the pyxis orchestrator and the tools it execs agree on
// where the operand is.
//
// The root is the nearest ancestor carrying .pyxis/config.yaml (the operand
// config, repo-owned from birth by pyxis init), falling back to the git
// working-tree root. It deliberately does NOT key on go.mod: an installed
// operand ships no Go source (a founding invariant of the release — targets
// consume released binaries, never source), so a go.mod walk finds nothing
// and the operand's own gate path — the hooks and CI that run
// `pyxis exec docgate -- gate` — dead-ends before the gate ever runs. This
// package is that dead-end's fix.
package reporoot

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Find returns the pyxis-operated repo root above the current working
// directory: the nearest ancestor holding .pyxis/config.yaml, else the git
// working-tree root. It errors when neither locates a root — the caller runs
// outside any pyxis-operated repository.
func Find() (string, error) {
	start, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("getwd: %w", err)
	}
	return From(start)
}

// From locates the operand above start using the same predicate as Find.
// Git discovery runs at start, so callers with an explicit directory do not
// accidentally resolve the process's own repository instead.
func From(start string) (string, error) {
	if root, ok := findConfigRoot(start); ok {
		return root, nil
	}
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	cmd.Dir = start
	if out, err := cmd.Output(); err == nil {
		if root := strings.TrimSpace(string(out)); root != "" {
			return root, nil
		}
	}
	return "", fmt.Errorf("no repo root found above %s — no .pyxis/config.yaml on the ancestor path and not inside a git repository; run inside a pyxis-operated repo", start)
}

// findConfigRoot walks from start to the filesystem root looking for the
// nearest ancestor that carries .pyxis/config.yaml.
func findConfigRoot(start string) (string, bool) {
	for dir := start; ; {
		if _, err := os.Stat(filepath.Join(dir, ".pyxis", "config.yaml")); err == nil {
			return dir, true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", false
		}
		dir = parent
	}
}
