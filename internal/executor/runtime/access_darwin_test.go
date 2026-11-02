package runtime

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/executor/port"
)

func TestSeatbeltPolicyKeepsGitAndAncestorWriteDenials(t *testing.T) {
	for _, readOnly := range []bool{false, true} {
		paths := accessPaths{work: "/work", state: "/state", protected: []string{"/work/.git"}, readOnly: readOnly,
			ancestors: []accessPaths{{work: "/parent", state: "/parent-state"}, {work: "/review", state: "/review-state", readOnly: true}}}
		cmd := exec.Command("/usr/bin/true")
		if err := confineNativeProcess(cmd, paths); err != nil {
			t.Fatal(err)
		}
		if cmd.Path != "/usr/bin/sandbox-exec" || len(cmd.Args) != 4 || cmd.Args[1] != "-p" || cmd.Args[3] != "/usr/bin/true" {
			t.Fatal("command was not bound to its Seatbelt policy")
		}
		profile := cmd.Args[2]
		for _, rule := range []string{`(deny file-write*)`, `(allow file-write* (subpath "/state"))`, `(deny file-write* (literal "/work/.git") (subpath "/work/.git"))`, `(require-any (subpath "/parent-state") (subpath "/parent")`, `(require-any (subpath "/review-state") (literal "/dev/null")`} {
			if !strings.Contains(profile, rule) {
				t.Fatalf("lost a required policy rule: %s", rule)
			}
		}
		if writable := strings.Contains(profile, `(allow file-write* (subpath "/work"))`); writable == readOnly {
			t.Fatal("policy did not preserve the requested worktree permissions")
		}
	}
}

func TestOfflineSeatbeltWrapperKeepsHostWritePolicy(t *testing.T) {
	for _, tc := range []struct {
		name, profile string
		fold          bool
	}{
		{"network-only", "(version 1) (allow default) (deny network*)", true},
		{"unrelated-profile", "(version 1) (allow default)", false},
		{"different-network-policy", "(version 1) (allow default) (deny network-outbound)", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			paths := accessPaths{work: "/work", state: "/state", protected: []string{"/work/.git"}, readOnly: true,
				ancestors: []accessPaths{{work: "/parent", state: "/parent-state"}}}
			cmd := exec.Command("/usr/bin/sandbox-exec", "-p", tc.profile, "/usr/bin/true", "retained-arg")
			if err := confineNativeProcess(cmd, paths); err != nil {
				t.Fatal(err)
			}
			profile := cmd.Args[2]
			for _, rule := range []string{`(deny file-write*)`, `(allow file-write* (subpath "/state"))`, `(deny file-write* (literal "/work/.git") (subpath "/work/.git"))`, `(require-any (subpath "/parent-state") (subpath "/parent")`} {
				if !strings.Contains(profile, rule) {
					t.Fatalf("lost host rule: %s", rule)
				}
			}
			if strings.Contains(profile, `(allow file-write* (subpath "/work"))`) {
				t.Fatal("offline child regained source writes")
			}
			if tc.fold {
				if cmd.Args[3] != "/usr/bin/true" || len(cmd.Args) != 5 || cmd.Args[4] != "retained-arg" || !strings.Contains(profile, "(deny network*)") {
					t.Fatal("offline child lost its network denial or original argv")
				}
			} else if cmd.Args[3] != "/usr/bin/sandbox-exec" || len(cmd.Args) != 8 || cmd.Args[5] != tc.profile {
				t.Fatal("rewrote an unrecognized sandbox profile")
			}
		})
	}
}

// Opt-in offline startup check against installed CLIs, including Cursor's real
// PTY wrapper. It never sends a prompt or makes a model call.
func TestProcessAccessInstalledHarnessStartup(t *testing.T) {
	if os.Getenv("PYXIS_TEST_INSTALLED_HARNESSES") != "1" {
		t.Skip("set PYXIS_TEST_INSTALLED_HARNESSES=1 for installed CLI startup checks")
	}
	for _, name := range []string{"codex", "claude", "cursor-agent"} {
		t.Run(name, func(t *testing.T) {
			binary, err := exec.LookPath(name)
			if err != nil {
				t.Fatal(err)
			}
			policy, _ := accessFixture(t, true)
			for _, dir := range []string{"home", "xdg/config", "xdg/cache", "xdg/state"} {
				if err := os.MkdirAll(filepath.Join(policy.RuntimeStateDir, dir), 0o700); err != nil {
					t.Fatal(err)
				}
			}
			cmd := exec.CommandContext(t.Context(), binary, "--version")
			if name == "cursor-agent" {
				cmd = exec.CommandContext(t.Context(), "/usr/bin/script", "-q", "/dev/null", binary, "--version")
			}
			cmd.Dir = policy.Workdir
			cmd.Env = LaunchEnvironment(port.LaunchSpec{Purpose: policy.Purpose, Workdir: policy.Workdir, RuntimeStateDir: policy.RuntimeStateDir})
			if err := ConfineCommand(t.Context(), cmd, &policy); err != nil {
				t.Fatal(err)
			}
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("offline confined startup: %v: %s", err, out)
			}
		})
	}
}
