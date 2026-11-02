package main

import (
	"context"
	"errors"
	"flag"
	"io"
	"net"
	"os"
	osexec "os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/executor/port"
	"github.com/agenticlab-ai/pyxis/internal/executor/runtime"
)

// This is an offline-operation check, not the Linux zero-syscall proof.
// Seatbelt denies network access while the same complete lifecycle executes.
// A positive control and denied TCP/UDP probes keep a broken policy from
// silently turning the check into an ordinary unconstrained lifecycle run.
func TestOfflineLifecycle(t *testing.T) {
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = listener.Close() }() // test owns the loopback canary
	state, coverageArgs := offlineTestState(t)
	store, err := os.MkdirTemp(state, "carina-offline-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(store); err != nil {
			t.Error(err)
		}
	})
	workdir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	runChild := func(t *testing.T, test string, sandboxed bool, env ...string) {
		t.Helper()
		ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
		defer cancel()
		args := []string{"-test.v", "-test.count=1", "-test.run=^" + test + "$"}
		args = append(args, coverageArgs...)
		name := os.Args[0]
		if sandboxed {
			args = append([]string{"-p", "(version 1) (allow default) (deny network*)", name}, args...)
			name = "/usr/bin/sandbox-exec"
		}
		cmd := osexec.CommandContext(ctx, name, args...)
		cmd.Env = append(append(os.Environ(), "TMPDIR="+state), env...)
		if sandboxed {
			cmd.Dir = workdir
			// Reuse the owned launcher when verification already has a write
			// guard. The host combines its policy with the network denial; source
			// stays read-only and the lifecycle writes only its private state.
			access := &runtime.CommandAccess{Purpose: port.LaunchPurposeReview, Workdir: workdir, RuntimeStateDir: state, RepositoryOptional: true}
			if err := runtime.ConfineCommand(ctx, cmd, access); err != nil {
				t.Fatal(err)
			}
		}
		out, err := cmd.CombinedOutput()
		if err != nil || !strings.Contains(string(out), "--- PASS: "+test) || strings.Contains(string(out), "--- SKIP") {
			t.Fatalf("child %s (sandboxed=%v): %v\n%s", test, sandboxed, err, out)
		}
	}
	probeEnv := []string{"WT_OFFLINE_PROBE=1", "WT_OFFLINE_ADDRESS=" + listener.Addr().String()}
	t.Run("network available without policy", func(t *testing.T) {
		runChild(t, "TestOfflineNetworkProbe", false, append(probeEnv, "WT_OFFLINE_DENIED=0")...)
	})
	t.Run("network denied by policy", func(t *testing.T) {
		runChild(t, "TestOfflineNetworkProbe", true, append(probeEnv, "WT_OFFLINE_DENIED=1")...)
	})
	t.Run("lifecycle succeeds without network", func(t *testing.T) {
		runChild(t, "TestHermeticLifecycleChild", true, hermeticRootEnv+"="+filepath.Join(store, "store"))
	})
}

// Exercise the complete offline proof inside the same owned write guard used
// by the executor, including when this test already inherits such a guard.
func TestOfflineLifecycleProtectedVerification(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 90*time.Second)
	defer cancel()
	ctx, cleanup, err := runtime.VerificationContext(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	workdir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	state, coverageArgs := offlineTestState(t)
	args := append([]string{"-test.v", "-test.count=1", "-test.run=^TestOfflineLifecycle$"}, coverageArgs...)
	cmd := osexec.CommandContext(ctx, os.Args[0], args...)
	cmd.Dir, cmd.Env = workdir, append(os.Environ(), "TMPDIR="+state)
	access := &runtime.CommandAccess{Purpose: port.LaunchPurposeReview, Workdir: workdir, RuntimeStateDir: state, RepositoryOptional: true}
	if err := runtime.ConfineCommand(ctx, cmd, access); err != nil {
		t.Fatal(err)
	}
	out, err := cmd.CombinedOutput()
	if err != nil || !strings.Contains(string(out), "--- PASS: TestOfflineLifecycle") || strings.Contains(string(out), "--- SKIP") {
		t.Fatalf("protected offline proof: %v\n%s", err, out)
	}
}

// Preserve Go's owned coverage directory so every child contributes to the
// parent's profile. Without forwarding the flag, testing creates another
// output directory outside the child's write scope at process exit.
func offlineTestState(t *testing.T) (string, []string) {
	t.Helper()
	if coverageDir := flag.Lookup("test.gocoverdir"); coverageDir != nil && coverageDir.Value.String() != "" {
		state := coverageDir.Value.String()
		return state, []string{"-test.gocoverdir=" + state}
	}
	return t.TempDir(), nil
}

func TestOfflineNetworkProbe(t *testing.T) {
	if os.Getenv("WT_OFFLINE_PROBE") != "1" {
		t.Skip("child entrypoint for TestOfflineLifecycle")
	}
	denied := os.Getenv("WT_OFFLINE_DENIED") == "1"
	for _, tc := range []struct {
		name string
		open func() (io.Closer, error)
	}{
		{"TCP connect", func() (io.Closer, error) {
			return net.DialTimeout("tcp4", os.Getenv("WT_OFFLINE_ADDRESS"), time.Second)
		}},
		{"TCP listen", func() (io.Closer, error) {
			return net.Listen("tcp4", "127.0.0.1:0")
		}},
		{"UDP listen", func() (io.Closer, error) {
			return net.ListenPacket("udp4", "127.0.0.1:0")
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			conn, err := tc.open()
			if err == nil {
				if closeErr := conn.Close(); closeErr != nil {
					t.Fatal(closeErr)
				}
			}
			if denied {
				if !errors.Is(err, syscall.EPERM) && !errors.Is(err, syscall.EACCES) {
					t.Fatalf("expected network permission denial, got %v", err)
				}
			} else if err != nil {
				t.Fatalf("unconfined network control failed: %v", err)
			}
		})
	}
}
