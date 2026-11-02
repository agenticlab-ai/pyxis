// Package toolbin resolves a sibling pyxis CLI to a runnable file so callers
// never depend on ambient PATH. That is the same acquisition story as
// `pyxis exec`: a verified release binary when the operand is pinned, or a
// one-shot `go build ./cmd/<tool>` when the checkout is in source mode.
//
// PATH is only a last resort — for a source tree that does not contain the
// tool, or an installed operand whose pins cannot be read. Debug sessions,
// worktrees, and `go run ./cmd/implement-now` therefore find carina from
// this checkout instead of requiring a globally installed binary.
package toolbin

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	pyxisconfig "github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/resolve"
)

var (
	lookPath    = exec.LookPath
	resolveTool = resolve.Tool
)

// Ensure returns an executable path for tool rooted at the operand checkout.
func Ensure(ctx context.Context, root, tool string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	tool = strings.TrimSpace(tool)
	root = strings.TrimSpace(root)
	if tool == "" {
		return "", errors.New("toolbin: empty tool name")
	}
	if root == "" {
		return "", errors.New("toolbin: empty repository root")
	}

	repo, defects := pyxisconfig.Load(root)
	if len(defects) == 0 {
		path, err := resolveTool(ctx, resolve.Config{Root: root, Repo: repo}, tool, "")
		if err == nil && path != "" {
			return path, nil
		}
		var sm *resolve.SourceMode
		if errors.As(err, &sm) {
			return buildFromSource(ctx, *sm)
		}
		if p, lerr := lookPath(tool); lerr == nil {
			return p, nil
		}
		var absent *resolve.SourceTreeAbsentError
		if errors.As(err, &absent) {
			return "", missingToolError(root, tool)
		}
		if err != nil {
			return "", fmt.Errorf("toolbin: resolve %s: %w", tool, err)
		}
	}

	cmdDir := filepath.Join(root, "cmd", tool)
	if st, err := os.Stat(cmdDir); err == nil && st.IsDir() {
		return buildFromSource(ctx, resolve.SourceMode{Tool: tool, RepoRoot: root, CmdDir: cmdDir})
	}
	if p, err := lookPath(tool); err == nil {
		return p, nil
	}
	return "", missingToolError(root, tool)
}

func missingToolError(root, tool string) error {
	return fmt.Errorf("toolbin: %s not on PATH and ./cmd/%s is not in %s — install %s or run inside the pyxis source tree", tool, tool, root, tool)
}

func buildFromSource(ctx context.Context, sm resolve.SourceMode) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	goBin, err := lookPath("go")
	if err != nil {
		return "", fmt.Errorf("toolbin: source mode needs the Go toolchain to build ./cmd/%s: %w", sm.Tool, err)
	}
	out := filepath.Join(sm.RepoRoot, ".pyxis", "bin", sm.Tool)
	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		return "", fmt.Errorf("toolbin: create %s: %w", filepath.Dir(out), err)
	}
	cmd := exec.CommandContext(ctx, goBin, "build", "-o", out, "./cmd/"+sm.Tool)
	cmd.Dir = sm.RepoRoot
	if b, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("toolbin: build ./cmd/%s: %v\n%s", sm.Tool, err, strings.TrimSpace(string(b)))
	}
	return out, nil
}
