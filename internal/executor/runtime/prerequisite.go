package runtime

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

// ProbeProcessAccess checks the actual OS facility, not just the executable's
// presence. It runs only a trusted no-op and performs no filesystem writes,
// installs, privilege escalation, or network access, so init and doctor share
// the same read-only prerequisite evaluator as the launch policy.
func ProbeProcessAccess(ctx context.Context) error {
	return probeProcessAccess(ctx, confineProcess)
}

func probeProcessAccess(ctx context.Context, wrap func(*exec.Cmd, accessPaths) error) error {
	cmd := exec.CommandContext(ctx, "/usr/bin/true")
	cmd.Env = []string{"PATH=" + os.Getenv("PATH")}
	if err := wrap(cmd, accessPaths{work: "/", state: os.TempDir(), readOnly: true}); err != nil {
		return err
	}
	if out, err := cmd.CombinedOutput(); err != nil {
		diagnostic := strings.TrimSpace(string(out))
		if len(diagnostic) > 1024 {
			diagnostic = diagnostic[:1024]
		}
		return fmt.Errorf("%w: %v: %s", ErrAccessUnsupported, err, diagnostic)
	}
	return nil
}

// ProcessAccessRemedy is shared by installation, doctor, and native launch
// errors. Host package/security policy remains an explicit operator action;
// Pyxis never silently installs privileged packages or disables host guards.
func ProcessAccessRemedy() string {
	return processAccessRemedy(runtime.GOOS)
}

func processAccessRemedy(platform string) string {
	switch platform {
	case "linux":
		return "install Bubblewrap with your distribution package manager (Debian/Ubuntu: sudo apt-get install bubblewrap); enable user namespaces for bwrap. On AppArmor hosts, allow userns for /usr/bin/bwrap using the profile in .pyxis/security/bwrap.apparmor, then re-run pyxis doctor"
	case "darwin":
		return "ensure /usr/bin/sandbox-exec can start child processes under the host security policy, then re-run pyxis doctor"
	default:
		return "run native implementation and review on macOS or Linux with a supported process boundary"
	}
}
