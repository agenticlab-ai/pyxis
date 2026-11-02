package ciparity

import (
	"fmt"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/integrations"
)

func workflowProvider(root string) (string, error) {
	cfg, err := integrations.Load(root)
	if err != nil {
		return "", err
	}
	if cfg.Bindings.CodeHost == (integrations.Binding{}) {
		return integrations.GitHub, nil
	}
	host, err := cfg.CodeHost("")
	if err != nil {
		return "", err
	}
	if host.Provider == integrations.LocalGit {
		if cfg.CI.Mode != "local" {
			return "", fmt.Errorf("local-git CI parity requires integrations.ci.mode: local; hosted actions are unavailable for this code host")
		}
		// There is no hosted workflow inventory in local-only mode. Check
		// still validates the shared check list and tracked pre-push hook.
		return "", nil
	}
	return host.Provider, nil
}

// Only fixed reviewed context expressions pass through here. A connection's
// validated provider selects the context; workflow-authored values never do.
func hostedExpression(expression, provider string) string {
	return strings.ReplaceAll(expression, "github.", provider+".")
}
