// Package runlogs owns repository-local execution diagnostic paths. Shared
// workflow state and retained decision evidence do not belong in this tree.
package runlogs

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	Root         = ".pyxis/logs"
	CLI          = Root + "/cli"
	Executor     = Root + "/executor"
	Watch        = Root + "/watch"
	Phasemeter   = Root + "/phasemeter"
	Codegate     = Root + "/codegate"
	Agents       = Root + "/agents"
	Reviews      = Root + "/reviews"
	ContractGate = Root + "/contract-gate"
)

// Resolve returns an absolute log path. Relative overrides are relative to the
// repository, and every execution-log destination must remain inside Root.
// Callers exporting shared evidence use their artifact writer instead.
func Resolve(ctx context.Context, repo, fallback, override string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	root, err := filepath.Abs(repo)
	if err != nil {
		return "", fmt.Errorf("resolve log repository: %w", err)
	}
	path := override
	if strings.TrimSpace(path) == "" {
		path = fallback
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(root, filepath.FromSlash(path))
	}
	path = filepath.Clean(path)
	rel, err := filepath.Rel(filepath.Join(root, filepath.FromSlash(Root)), path)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("execution log path must be inside %s: %s", filepath.Join(root, Root), path)
	}
	// Reject symlinked descendants so an ignored log cannot escape to a tracked
	// file or another directory. A symlink to the repository itself is fine.
	for p := path; p != root; p = filepath.Dir(p) {
		info, err := os.Lstat(p)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return "", fmt.Errorf("inspect log path: %w", err)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return "", fmt.Errorf("execution log path contains a symlink: %s", p)
		}
	}
	return path, nil
}
