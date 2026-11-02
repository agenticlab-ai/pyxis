package ciparity

import (
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/integrations"
)

// Setup is deliberately a closed set, never a prefix match or a workflow-wide
// exemption. Adding a test to one of these commands makes it fail parity.
// New actions require review because they can hide executable checks themselves.
const linuxSetup = `sudo apt-get update -qq
sudo apt-get install -y bubblewrap apparmor zsh
sudo install -m 0644 .pyxis/security/bwrap.apparmor /etc/apparmor.d/pyxis-bwrap
sudo apparmor_parser --replace /etc/apparmor.d/pyxis-bwrap`

// Some hosted runners (for example Blacksmith VMs) boot kernels without
// AppArmor, so there is no user-namespace restriction to lift and
// apparmor_parser cannot load the profile. This variant loads it only when
// AppArmor is active; installed workflows keep the unconditional form above.
const linuxSetupOptionalAppArmor = `sudo apt-get update -qq
sudo apt-get install -y bubblewrap apparmor zsh
sudo install -m 0644 .pyxis/security/bwrap.apparmor /etc/apparmor.d/pyxis-bwrap
if sudo aa-enabled --quiet; then sudo apparmor_parser --replace /etc/apparmor.d/pyxis-bwrap; fi`

// This fixed hosted-runner SDK directory is unused by the Go test job. Keep
// cleanup bounded so resource setup cannot hide another verification command.
const linuxCoverageDiskSetup = "sudo rm -rf -- /usr/local/lib/android"

const replayFetch = "git fetch --no-tags origin 3ed3ffb1cf6e4a593bf9990178bb10afe88dce02"
const replayCondition = "github.event.pull_request.base.sha == '10561e22b0ad2ebceb84ac8c5b8913588b0096cc' || github.event.before == '10561e22b0ad2ebceb84ac8c5b8913588b0096cc'"

func setupStep(s step, provider string) bool {
	if s.Run != "" {
		if len(s.Env) != 0 || len(s.With) != 0 {
			return false
		}
		switch strings.TrimSpace(s.Run) {
		case linuxSetup, linuxSetupOptionalAppArmor, linuxCoverageDiskSetup:
			return s.If == ""
		case replayFetch:
			return s.If == hostedExpression(replayCondition, provider)
		default:
			return false
		}
	}
	if len(s.Env) != 0 {
		return false
	}
	uses := s.Uses
	if provider == integrations.Forgejo {
		// Forgejo source workflows pin the checkout implementation and resolve
		// reviewed setup actions by explicit origin, not the server's default.
		switch uses {
		case "https://data.forgejo.org/actions/checkout@v4":
			uses = "actions/checkout@v4"
		case "https://github.com/actions/setup-go@v5", "https://github.com/actions/setup-python@v5", "https://github.com/actions/setup-node@v4", "https://github.com/actions/cache/restore@v4", "https://github.com/actions/cache/save@v4":
			uses = strings.TrimPrefix(uses, "https://github.com/")
		default:
			return false
		}
	}
	switch uses {
	// useblacksmith/checkout is a drop-in actions/checkout with a runner-local
	// git mirror; it takes the same single input.
	case "actions/checkout@v4", "useblacksmith/checkout@v1":
		return s.If == "" && len(s.With) == 1 && s.With["fetch-depth"] == "0"
	case "actions/setup-go@v5":
		return s.If == "" && s.With["go-version-file"] == "go.mod" && onlyKeys(s.With, "go-version-file", "cache")
	case "actions/setup-python@v5":
		return s.If == "" && len(s.With) == 1 && s.With["python-version"] == "3.x"
	case "actions/setup-node@v4":
		return s.If == "" && len(s.With) == 1 && s.With["node-version"] == "20"
	case "actions/cache/restore@v4", "actions/cache/save@v4":
		return s.With["path"] != "" && s.With["key"] != "" && onlyKeys(s.With, "path", "key", "restore-keys")
	default:
		return false
	}
}

func onlyKeys(values map[string]string, allowed ...string) bool {
	for key := range values {
		found := false
		for _, candidate := range allowed {
			found = found || key == candidate
		}
		if !found {
			return false
		}
	}
	return true
}
