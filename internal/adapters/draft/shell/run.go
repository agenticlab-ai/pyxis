// Package draftshell implements the feature-draft lifecycle process port.
package draftshell

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// Run executes a Git or code-host command in the requested repository. Env
// overrides are values, never command arguments, so a credential cannot enter
// process listings or the error's printed argv.
func Run(ctx context.Context, dir string, env []string, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), env...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("%s %s: %w: %s", name, strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return out, nil
}
