package stamp

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/argus/attribution"
)

// GitBinding computes the in-tree content binding for the commit the producer
// is about to author, from the worktree's current git state: the tree it is
// about to commit (`git write-tree` over the staged index — the same tree git
// will record) and its parent (`git rev-parse HEAD`, or none on a root commit /
// unborn branch). The pair is fed to the single-sourced attribution.ComputeBinding
// so the produced binding and the gate's re-derivation are byte-identical (§3.2,
// DINV-4).
//
// This is the producer's one git read — the cmd/argus-stamp I/O layer calls it
// and passes the result into the pure Stamp core, keeping Stamp hermetic. A
// write-tree failure is a real error (an unmergeable index, a broken repo): the
// caller fails the commit closed rather than emit an unbindable trailer, exactly
// as a malformed slug aborts. `git write-tree` honors the GIT_INDEX_FILE git
// sets for a partial commit, so the tree matches even then.
func GitBinding(worktree string) (string, error) {
	tree, err := gitField(worktree, "write-tree")
	if err != nil {
		return "", fmt.Errorf("stamp: computing the commit tree for the content binding: %w", err)
	}
	if tree == "" {
		return "", fmt.Errorf("stamp: git write-tree produced no tree for the content binding")
	}
	var parents []string
	// A root commit / unborn branch has no HEAD — that is not an error, it is a
	// parentless commit; the gate reads the same empty `%P`.
	if head, err := gitField(worktree, "rev-parse", "HEAD"); err == nil && head != "" {
		parents = []string{head}
	}
	return attribution.ComputeBinding(tree, parents), nil
}

// gitField runs a git command in worktree and returns its trimmed stdout.
func gitField(worktree string, args ...string) (string, error) {
	out, err := exec.Command("git", append([]string{"-C", worktree}, args...)...).Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}
