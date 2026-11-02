package runtime

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestVerificationResolvesAppleLaunchersAndPinsSDK(t *testing.T) {
	t.Setenv("PATH", "/usr/bin")
	cmd := exec.Command("/usr/bin/true")
	cmd.Env = []string{"PATH=/usr/bin", "SDKROOT=/unowned-sdk", "KEEP=value"}
	if err := configureVerificationTools(t.Context(), cmd, false); err != nil {
		t.Fatal(err)
	}
	env := strings.Join(cmd.Env, "\n")
	if strings.Contains(env, "/unowned-sdk") || !strings.Contains(env, "SDKROOT=/") || !strings.Contains(env, "KEEP=value") {
		t.Fatal("lost the host SDK selection or a declared setting")
	}
	var toolPath string
	for _, entry := range cmd.Env {
		if value, ok := strings.CutPrefix(entry, "PATH="); ok {
			toolPath = value
		}
	}
	for _, tool := range []string{"git", "clang"} {
		found := false
		for _, dir := range filepath.SplitList(toolPath) {
			if dir == "/usr/bin" {
				continue
			}
			if info, err := os.Stat(filepath.Join(dir, tool)); err == nil && !info.IsDir() {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("%s still resolves through the system launcher", tool)
		}
	}
}

func TestVerificationToolchainResolutionFailsClosed(t *testing.T) {
	t.Setenv("PATH", "/usr/bin") // force Apple's xcrun launcher
	ctx, cleanup, err := VerificationContext(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	ctx, cancel := context.WithCancel(ctx)
	cancel()
	cmd := exec.Command("must-not-start")
	cmd.Env = []string{"PATH=/usr/bin"}
	if err := ConfineVerificationCommand(ctx, cmd); err == nil || !strings.Contains(err.Error(), "resolve verification git toolchain") || cmd.Process != nil {
		t.Fatalf("toolchain failure started a command: %v", err)
	}
	t.Setenv("PATH", t.TempDir()) // bypass launchers and exercise SDK selection
	t.Setenv("SDKROOT", "")
	if err := ConfineVerificationCommand(ctx, cmd); err == nil || !strings.Contains(err.Error(), "resolve verification SDK") || cmd.Process != nil {
		t.Fatalf("SDK failure started a command: %v", err)
	}
}
