package reviewinvoke

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// The running implementation host contains the canonical assembler. Its exact
// executable remains visible in the review sandbox's read-only filesystem;
// neither the proposal clone nor its ignored binary cache is used to resolve it.
func trustedAssemblerPath(ctx context.Context, workdir, state string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	executable, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("review assembler: host executable: %w", err)
	}
	return validateAssemblerPath(executable, workdir, state)
}

func validateAssemblerPath(executable, workdir, state string) (string, error) {
	path, err := filepath.EvalSymlinks(executable)
	if err != nil {
		return "", fmt.Errorf("review assembler: resolve host executable: %w", err)
	}
	if !filepath.IsAbs(path) {
		return "", fmt.Errorf("review assembler: host executable must be absolute")
	}
	for _, root := range []string{workdir, state} {
		root, err = filepath.EvalSymlinks(root)
		if err != nil {
			return "", fmt.Errorf("review assembler: resolve runtime boundary: %w", err)
		}
		rel, err := filepath.Rel(root, path)
		if err != nil || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))) {
			return "", fmt.Errorf("review assembler must be outside proposal and writable runtime roots")
		}
	}
	info, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("review assembler: stat host executable: %w", err)
	}
	if !info.Mode().IsRegular() || info.Mode().Perm()&0o111 == 0 {
		return "", fmt.Errorf("review assembler: host path must be a regular executable")
	}
	return path, nil
}
