package githooks

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
)

// Active verifies the installed dispatcher and its source without changing Git
// state. Merely pointing hooksPath at a directory is insufficient: the old
// guard-only installation had that path but silently omitted pre-push.
func Active(ctx context.Context, root, expectedSource string) (bool, error) {
	r := repository{root: root}
	active, err := r.hookConfig(ctx, "core.hooksPath")
	if err != nil {
		return false, err
	}
	if filepath.Base(active) != guardDirName {
		return false, nil
	}
	common, err := r.commonGitDir(ctx)
	if err != nil {
		return false, err
	}
	dir := filepath.Join(common, guardDirName)
	if !sameHookDir(root, active, dir) {
		return false, nil
	}
	source, err := r.hookConfig(ctx, hookSourceKey)
	if err != nil {
		return false, err
	}
	if source != expectedSource {
		return false, fmt.Errorf("managed hooks source is %q, expected %q; re-run hooks-setup", source, expectedSource)
	}
	for _, hook := range forwardedHooks {
		path := filepath.Join(dir, hook)
		body, err := os.ReadFile(path)
		if err != nil {
			return false, fmt.Errorf("read managed hook %s: %w; re-run hooks-setup", hook, err)
		}
		info, err := os.Stat(path)
		if err != nil {
			return false, fmt.Errorf("stat managed hook %s: %w", hook, err)
		}
		if string(body) != hookDispatcher(source, hook) || info.Mode().Perm()&0o111 == 0 {
			return false, fmt.Errorf("managed hook %s is stale or not executable; re-run hooks-setup", hook)
		}
	}
	return true, nil
}
