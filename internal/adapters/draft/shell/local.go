package draftshell

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// RunLocalGit keeps the draft workflow on file transport and removes ambient
// repository overrides. Hosted commands have no executable fallback here.
func RunLocalGit(ctx context.Context, dir string, env []string, name string, args ...string) ([]byte, error) {
	if name != "git" {
		return nil, fmt.Errorf("local draft process refuses %q", name)
	}
	cmd := exec.CommandContext(ctx, "git", append([]string{"-c", "protocol.allow=never", "-c", "protocol.file.allow=always"}, args...)...)
	cmd.Dir = dir
	for _, item := range os.Environ() {
		key, _, _ := strings.Cut(item, "=")
		if !strings.HasPrefix(key, "GIT_") {
			cmd.Env = append(cmd.Env, item)
		}
	}
	cmd.Env = append(cmd.Env, "GIT_TERMINAL_PROMPT=0", "GIT_NO_REPLACE_OBJECTS=1")
	// These two workflow overrides are supplied by preparation to preserve
	// verified rebase stamps and avoid attributing its generated commit to a
	// drive. Repository and transport overrides remain stripped.
	for _, item := range env {
		key, _, _ := strings.Cut(item, "=")
		if key == "ARGUS_STAMP_REBASE" || key == "ARGUS_DRIVE_SLUG" {
			cmd.Env = append(cmd.Env, item)
		}
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if err != nil {
		return nil, fmt.Errorf("local Git: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	return stdout.Bytes(), nil
}
