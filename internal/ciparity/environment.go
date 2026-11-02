package ciparity

import (
	"fmt"
	"maps"
	"slices"

	"github.com/agenticlab-ai/pyxis/internal/codegatemirror"
	"github.com/agenticlab-ai/pyxis/internal/integrations"
)

func checkGateEnv(env map[string]string, cfg codegatemirror.Config, provider string) error {
	// These are the reviewed hosted inputs. Other overrides can affect either
	// the gate launcher itself (for example GOFLAGS) or a passed-through check.
	trusted := map[string]string{
		"PYXIS_CI_BASE":                     hostedExpression(baseExpression, provider),
		"PYXIS_CI_EVENT":                    hostedExpression(eventExpression, provider),
		"GITHUB_ACTIONS":                    "true",
		"PYXIS_REQUIRE_PROCESS_CONFINEMENT": "1",
		"GH_TOKEN":                          "${{ github.token }}",
		"PYXIS_PUSH_BEFORE":                 "${{ github.event.before }}",
		"PYXIS_LOCK_RECORD_RUN":             "1",
		"PYXIS_PR_DESCRIPTION_RUN":          "1",
	}
	if provider == integrations.Forgejo {
		trusted["FORGEJO_ACTIONS"] = "true"
	}
	for _, name := range slices.Sorted(maps.Keys(env)) {
		if name == "PATH" || name == "HOME" {
			return fmt.Errorf("shared gate may not override PATH or HOME")
		}
		want, ok := trusted[name]
		if !ok {
			return fmt.Errorf("unregistered shared gate environment override %s", name)
		}
		if env[name] != want {
			return fmt.Errorf("shared gate requires %s=%q", name, want)
		}
	}
	for _, check := range cfg.Checks {
		if slices.Contains(check.Env, "PYXIS_REQUIRE_PROCESS_CONFINEMENT") && env["PYXIS_REQUIRE_PROCESS_CONFINEMENT"] != "1" {
			return fmt.Errorf("shared check %s requires PYXIS_REQUIRE_PROCESS_CONFINEMENT=\"1\"", check.Name)
		}
	}
	return nil
}
