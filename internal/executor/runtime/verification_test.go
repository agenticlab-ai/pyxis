package runtime

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"testing/synctest"
	"time"
)

func TestVerificationContextDoesNotBlockVirtualClockBeforeLaunch(t *testing.T) {
	t.Setenv(launchCapabilityEnv, os.Getenv(launchCapabilityEnv))
	if err := os.Unsetenv(launchCapabilityEnv); err != nil {
		t.Fatal(err)
	}
	synctest.Test(t, func(t *testing.T) {
		ctx, cleanup, err := VerificationContext(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		defer cleanup()
		if !VerificationAccessRequired(ctx) {
			t.Fatal("verification boundary was not attached")
		}
		start := time.Now()
		time.Sleep(time.Hour)
		if elapsed := time.Since(start); elapsed != time.Hour {
			t.Fatalf("virtual deadline advanced by %s", elapsed)
		}
	})
}

func TestVerificationOwnerChecksCapabilityBeforePolicyProbe(t *testing.T) {
	// Exercise the supervisor's policy setup while retaining any outer OS
	// boundary. The injected probe rejects before starting a command.
	t.Setenv(launchCapabilityEnv, os.Getenv(launchCapabilityEnv))
	if err := os.Unsetenv(launchCapabilityEnv); err != nil {
		t.Fatal(err)
	}
	for _, state := range []string{"active", "closed", "revoked"} {
		t.Run(state, func(t *testing.T) {
			policy, _ := accessFixture(t, false)
			ctx, cleanup, err := VerificationContext(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			defer cleanup()
			switch state {
			case "closed":
				cleanup()
			case "revoked":
				broker, err := ctx.Value(launchBrokerKey{}).(*launchBrokerOwner).get(ctx)
				if err != nil {
					t.Fatal(err)
				}
				broker.cancel()
			}
			cmd := exec.Command("must-not-start")
			cmd.Dir, cmd.Env = policy.Workdir, []string{"PATH=" + os.Getenv("PATH")}
			probeFailure := errors.New("policy probe rejected")
			probed := false
			err = confineCommand(ctx, cmd, &policy, func(*exec.Cmd, accessPaths) error {
				probed = true
				return probeFailure
			})
			if cmd.Process != nil {
				t.Fatal("policy setup started a command")
			}
			if state == "active" {
				if !probed || !errors.Is(err, probeFailure) || !strings.Contains(strings.Join(cmd.Env, "\n"), launchCapabilityEnv+"=") {
					t.Fatalf("active owner lost its launch capability: %v", err)
				}
			} else if err == nil || probed {
				t.Fatalf("inactive owner reached the policy probe: %v", err)
			}
		})
	}
}

func TestVerificationContextOwnsScratchAndPreservesNestedBoundary(t *testing.T) {
	// Exercise creation by the owning supervisor even when this test itself
	// runs inside verification. No command is executed through this new owner.
	t.Setenv(launchCapabilityEnv, os.Getenv(launchCapabilityEnv))
	if err := os.Unsetenv(launchCapabilityEnv); err != nil {
		t.Fatal(err)
	}
	ctx, cleanup, err := VerificationContext(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	state := ctx.Value(verificationAccessKey{}).(string)
	nested, nestedCleanup, err := VerificationContext(ctx)
	if err != nil || nested != ctx || !VerificationAccessRequired(ctx) {
		t.Fatalf("nested context: %v", err)
	}
	nestedCleanup()
	if _, err := os.Stat(state); err != nil {
		t.Fatalf("nested cleanup removed owner's scratch: %v", err)
	}
	cleanup()
	if _, err := os.Stat(state); !os.IsNotExist(err) {
		t.Fatalf("scratch survived owner: %v", err)
	}
	cmd := exec.Command("unconfined-standalone")
	if err := ConfineVerificationCommand(context.Background(), cmd); err != nil || cmd.Path != "unconfined-standalone" {
		t.Fatalf("standalone command changed: %v", err)
	}
}

func TestVerificationScratchFailureAndMalformedRepoNeverLaunch(t *testing.T) {
	t.Run("invalid inherited capability", func(t *testing.T) {
		t.Setenv(launchCapabilityEnv, "{}")
		if _, cleanup, err := VerificationContext(t.Context()); err == nil || cleanup != nil {
			t.Fatal("accepted forged capability")
		}
	})
	t.Run("scratch unavailable", func(t *testing.T) {
		t.Setenv("TMPDIR", filepath.Join(t.TempDir(), "missing"))
		if _, cleanup, err := VerificationContext(t.Context()); err == nil || cleanup != nil {
			t.Fatalf("scratch failure accepted: %v", err)
		}
	})
	ctx, cleanup, err := VerificationContext(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".git"), []byte("broken"), 0o600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("must-not-start")
	cmd.Dir, cmd.Env = dir, []string{"HOME=/host", "GH_TOKEN=secret", "GITHUB_TOKEN=secret", "SSH_AUTH_SOCK=/host/agent", "GIT_DIR=/host/.git", "LD_PRELOAD=/host/inject.so", "DYLD_INSERT_LIBRARIES=/host/inject.dylib", "GCONV_PATH=/host/code", "HTTPS_PROXY=proxy"}
	if err := ConfineVerificationCommand(ctx, cmd); err == nil || cmd.Process != nil {
		t.Fatalf("invalid Git metadata accepted: %v", err)
	}
	env := strings.Join(cmd.Env, "\n")
	if strings.Contains(env, "secret") || strings.Contains(env, "/host") || !strings.Contains(env, "HTTPS_PROXY=proxy") {
		t.Fatalf("unsafe verification environment: %s", env)
	}
	home := filepath.Join(ctx.Value(verificationAccessKey{}).(string), "home")
	if err := os.Remove(home); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(home, []byte("blocked"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := ConfineVerificationCommand(ctx, cmd); err == nil || !strings.Contains(err.Error(), "verification home") {
		t.Fatalf("unavailable home accepted: %v", err)
	}
}

func TestVerificationPreservesOwnedScratchThroughStrippedVerbEnv(t *testing.T) {
	_, ref, _ := testLaunchBroker(t, func(*exec.Cmd, accessPaths) error {
		t.Fatal("inherited verification tried to apply another sandbox")
		return nil
	})
	capability := strings.TrimPrefix(withLaunchCapability(nil, ref)[0], launchCapabilityEnv+"=")
	t.Setenv(launchCapabilityEnv, capability)
	for _, tc := range []struct {
		name string
		path string
	}{
		{"local tools", os.Getenv("PATH")},
		{"hosted tools", "/opt/hostedtoolcache/go/1.26.8/x64/bin:/usr/bin:/bin"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cmd := exec.Command("/usr/bin/true")
			cmd.Env = []string{"HOME=/host", "TMPDIR=/host", "GOCACHE=/host", "GH_TOKEN=secret", "SDKROOT=/ignored-sdk", "PATH=" + tc.path}
			if err := ConfineVerificationCommand(context.Background(), cmd); err != nil {
				t.Fatal(err)
			}
			env := make(map[string]string)
			for _, entry := range cmd.Env {
				key, value, _ := strings.Cut(entry, "=")
				env[key] = value
			}
			for key, want := range map[string]string{
				"HOME":              filepath.Join(ref.State, "home"),
				"TMPDIR":            filepath.Join(ref.State, "tmp"),
				"GOCACHE":           filepath.Join(ref.State, "go-build"),
				launchCapabilityEnv: capability,
			} {
				if env[key] != want {
					t.Fatalf("lost owned verification setting %q", key)
				}
			}
			if _, present := env["GH_TOKEN"]; present {
				t.Fatal("inherited verification exposed host credentials")
			}
			// Apple toolchain resolution may prepend directories. The declared
			// PATH must survive, even when it contains the substring /host.
			if env["PATH"] != tc.path && !strings.HasSuffix(env["PATH"], string(os.PathListSeparator)+tc.path) {
				t.Fatal("inherited verification lost its tool paths")
			}
			if cmd.Path != "/usr/bin/true" {
				t.Fatal("inherited verification changed its boundary")
			}
		})
	}
	t.Setenv(launchCapabilityEnv, "{}")
	if err := ConfineVerificationCommand(context.Background(), exec.Command("/usr/bin/true")); err == nil {
		t.Fatal("invalid inherited boundary was ignored")
	}
}

func TestProcessAccessPrerequisiteChecksKernelAndRemedies(t *testing.T) {
	if err := ProbeProcessAccess(t.Context()); err != nil {
		if os.Getenv("PYXIS_REQUIRE_PROCESS_CONFINEMENT") == "1" {
			t.Fatal(err)
		}
		t.Skipf("OS boundary unavailable: %v", err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if err := ProbeProcessAccess(ctx); !errors.Is(err, ErrAccessUnsupported) {
		t.Fatalf("cancelled probe = %v", err)
	}
	for _, platform := range []string{"linux", "darwin", "windows"} {
		if remedy := processAccessRemedy(platform); remedy == "" {
			t.Fatalf("no remedy for %s", platform)
		}
	}
}

func TestProcessPrerequisiteReportsUnavailableWrapperAndBoundsDiagnostics(t *testing.T) {
	want := errors.New("missing wrapper")
	if err := probeProcessAccess(t.Context(), func(*exec.Cmd, accessPaths) error { return want }); !errors.Is(err, want) {
		t.Fatalf("missing wrapper cause lost: %v", err)
	}
	err := probeProcessAccess(t.Context(), func(cmd *exec.Cmd, _ accessPaths) error {
		cmd.Path = os.Args[0]
		cmd.Args = []string{os.Args[0], "-test.run=^TestProcessPrerequisiteDiagnosticHelper$"}
		cmd.Env = []string{"PYXIS_PREREQUISITE_DIAGNOSTIC=1"}
		return nil
	})
	if !errors.Is(err, ErrAccessUnsupported) || !strings.Contains(err.Error(), "blocked") || len(err.Error()) > 1200 {
		t.Fatalf("unbounded or missing wrapper diagnostic: %v", err)
	}
}

func TestProcessPrerequisiteDiagnosticHelper(t *testing.T) {
	if os.Getenv("PYXIS_PREREQUISITE_DIAGNOSTIC") != "1" {
		return
	}
	// Test helper diagnostics are best-effort; the parent checks captured bytes.
	_, _ = os.Stderr.WriteString(strings.Repeat("blocked", 250))
	os.Exit(1)
}
