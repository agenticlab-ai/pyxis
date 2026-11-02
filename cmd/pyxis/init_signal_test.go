//go:build !windows

package main

import (
	"context"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/installer"
)

// The child sends a real signal exactly when logging is about to create the
// first directory. Before the fix it dies from that signal; afterward init
// handles it and passes a cancelled context to the installer. A subprocess
// keeps a missing handler from terminating the parent test runner.
func TestInitHandlesSignalBeforeLogSetup(t *testing.T) {
	const signalEnv = "PYXIS_TEST_INIT_LOG_SIGNAL"
	const closedOutputEnv = "PYXIS_TEST_INIT_CLOSED_OUTPUT"
	if name := os.Getenv(signalEnv); name != "" {
		target := t.TempDir()
		t.Chdir(target)
		t.Setenv("XDG_CONFIG_HOME", t.TempDir())
		sig := syscall.SIGTERM
		if name == "SIGINT" {
			sig = syscall.SIGINT
		}
		var stderr io.Writer = os.Stderr
		if os.Getenv(closedOutputEnv) == "true" {
			stderr = closedInitOutput{}
		}
		swap(t, &initRunLogger, func(root string, stderr io.Writer, verb string, verbose bool) (*slog.Logger, string) {
			if err := syscall.Kill(os.Getpid(), sig); err != nil {
				t.Fatalf("send %s: %v", name, err)
			}
			return runLogger(root, stderr, verb, verbose)
		})
		cancelled := false
		swap(t, &installerInit, func(ctx context.Context, _ installer.Options) (installer.Report, error) {
			select {
			case <-ctx.Done():
				cancelled = true
				return installer.Report{}, ctx.Err()
			case <-time.After(5 * time.Second):
				t.Fatal("init did not handle the signal sent before log setup")
				return installer.Report{}, nil
			}
		})
		code := initVerb([]string{"--yes", "--system-version", "v1.0.0", "--manifest-sha256", strings.Repeat("ab", 32)}, io.Discard, stderr)
		if code != 1 || !cancelled {
			t.Fatalf("interrupted init = %d, cancelled = %t", code, cancelled)
		}
		if _, err := os.Stat(filepath.Join(target, ".pyxis", "logs", "cli")); err != nil {
			t.Fatalf("log setup did not finish after the handled signal: %v", err)
		}
		return
	}
	for _, tc := range []struct{ signal, closedOutput string }{
		{"SIGTERM", "false"}, {"SIGINT", "false"},
		{"SIGTERM", "true"}, {"SIGINT", "true"},
	} {
		t.Run(tc.signal+"/closed-output="+tc.closedOutput, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestInitHandlesSignalBeforeLogSetup$")
			cmd.Env = append(os.Environ(), signalEnv+"="+tc.signal, closedOutputEnv+"="+tc.closedOutput)
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("init must handle %s before creating log directories: %v\n%s", tc.signal, err, out)
			}
		})
	}
}

// A failed terminal must not stop the signal watcher before it cancels init.
type closedInitOutput struct{}

func (closedInitOutput) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }
