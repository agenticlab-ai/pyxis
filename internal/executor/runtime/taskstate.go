package runtime

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/entity"
)

// TaskStateRoot is outside the task's publishable tree. The supervisor owns
// this directory; a harness may write only its own child directory within it.
func TaskStateRoot(worktree string) string { return worktree + "-runtime" }

// PrepareTaskState preserves native conversation files across implementation,
// repair, remediation, and feedback launches of the same task. Review state
// remains disposable. A host-owned marker outside the writable child prevents
// an interrupted initialization from being mistaken for a usable session.
func PrepareTaskState(ctx context.Context, worktree string, harness entity.Harness) (state string, fresh bool, err error) {
	if err := ctx.Err(); err != nil {
		return "", false, err
	}
	name := harness.String()
	if name == "" {
		// Legacy port callers and runtime doubles may omit the harness. They
		// receive isolated state with no copied provider credential.
		name = "default"
	}
	if name == "." || name == ".." || filepath.Base(name) != name || strings.ContainsAny(name, "\\\x00\r\n") {
		return "", false, fmt.Errorf("runtime: invalid task harness path %q", harness)
	}
	work, err := accessDirectory(worktree)
	if err != nil {
		return "", false, fmt.Errorf("runtime: task worktree: %w", err)
	}
	root := TaskStateRoot(work)
	if err := privateStateDirectory(root); err != nil {
		return "", false, err
	}
	state = filepath.Join(root, name)
	marker := filepath.Join(root, name+".ready")
	if err := os.Mkdir(state, 0o700); errors.Is(err, os.ErrExist) {
		info, statErr := os.Lstat(state)
		if statErr != nil || !info.IsDir() {
			return "", false, fmt.Errorf("runtime: task state is not a directory: %s", state)
		}
		info, statErr = os.Lstat(marker)
		if statErr != nil || !info.Mode().IsRegular() {
			return "", false, fmt.Errorf("runtime: task state initialization is incomplete: %s", state)
		}
		return state, false, nil
	} else if err != nil {
		return "", false, fmt.Errorf("runtime: create task state: %w", err)
	}
	// Only the newly-created directory is removable on failed preparation.
	defer func() {
		if err != nil {
			if removeErr := os.RemoveAll(state); removeErr != nil {
				err = errors.Join(err, fmt.Errorf("runtime: remove failed task state: %w", removeErr))
			}
		}
	}()
	if err = prepareStateDirectories(state); err != nil {
		return state, false, err
	}
	// Registered extensions receive isolated state too. Only the built-in
	// adapters have a host credential snapshot format; custom adapters own
	// their authentication without being rejected by a second registry here.
	switch harness {
	case entity.Codex, entity.Claude, entity.Cursor, entity.OpenCode:
		if err = PrepareReviewAuthentication(ctx, state, harness); err != nil {
			return state, false, err
		}
	}
	if err = os.WriteFile(marker, []byte("task-runtime/v1\n"), 0o600); err != nil {
		return state, false, fmt.Errorf("runtime: mark task state ready: %w", err)
	}
	return state, true, nil
}

func privateStateDirectory(path string) error {
	if err := os.Mkdir(path, 0o700); err != nil && !errors.Is(err, os.ErrExist) {
		return fmt.Errorf("runtime: create task state root: %w", err)
	}
	info, err := os.Lstat(path)
	if err != nil || !info.IsDir() {
		return fmt.Errorf("runtime: task state root is not a directory: %s", path)
	}
	return nil
}

func prepareStateDirectories(state string) error {
	for _, dir := range []string{"tmp", "home", "no-codehost-credentials", "xdg/config", "xdg/cache", "xdg/state"} {
		if err := os.MkdirAll(filepath.Join(state, dir), 0o700); err != nil {
			return fmt.Errorf("runtime: prepare state directory: %w", err)
		}
	}
	return nil
}
