package runtime

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func configureVerificationTools(ctx context.Context, cmd *exec.Cmd, inherited bool) error {
	// Apple's Git and compiler launchers write an xcrun cache outside our
	// write roots. Resolve the actual tools before entering the boundary.
	for _, tool := range []string{"git", "clang"} {
		if path, err := exec.LookPath(tool); err != nil || path != "/usr/bin/"+tool {
			continue
		}
		out, err := exec.CommandContext(ctx, "/usr/bin/xcrun", "--find", tool).Output()
		if err != nil {
			return fmt.Errorf("runtime: resolve verification %s toolchain: %w", tool, err)
		}
		dir := filepath.Dir(strings.TrimSpace(string(out)))
		for i, entry := range cmd.Env {
			if value, ok := strings.CutPrefix(entry, "PATH="); ok {
				cmd.Env[i] = "PATH=" + dir + string(os.PathListSeparator) + value
			}
		}
	}
	// The real compiler also needs the SDK selection the xcrun launcher
	// normally supplies. Preserve it through nested declarative verb envs.
	sdk := os.Getenv("SDKROOT")
	if !inherited || sdk == "" {
		out, err := exec.CommandContext(ctx, "/usr/bin/xcrun", "--show-sdk-path").Output()
		if err != nil {
			return fmt.Errorf("runtime: resolve verification SDK: %w", err)
		}
		sdk = strings.TrimSpace(string(out))
	}
	var toolEnv []string
	for _, entry := range cmd.Env {
		if !strings.HasPrefix(entry, "SDKROOT=") {
			toolEnv = append(toolEnv, entry)
		}
	}
	cmd.Env = append(toolEnv, "SDKROOT="+sdk)
	return nil
}
