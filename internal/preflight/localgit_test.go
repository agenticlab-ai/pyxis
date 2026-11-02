package preflight_test

import (
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/adapters/common/forgejo"
	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/integrations"
	"github.com/agenticlab-ai/pyxis/internal/preflight"
)

func TestLocalGitPreflightNeverSelectsHostedTransport(t *testing.T) {
	profile := integrations.Config{SchemaVersion: integrations.SchemaVersion, Bindings: integrations.Bindings{CodeHost: integrations.Binding{Provider: integrations.LocalGit, Root: "/configured/origin.git"}, Tasks: integrations.Binding{Provider: integrations.Carina}}, CI: integrations.CI{Mode: "local"}}
	runner := newHappyRunner(t)
	runner.failOnCall = true
	factory := func(integrations.ResolvedRepository) (*forgejo.Client, error) {
		t.Fatal("local code host selected Forgejo")
		return nil, nil
	}
	knobs := config.Knobs{Tracker: "carina", CIMode: "local", Wiring: config.Wiring{Repo: "use", Protection: "none"}}
	rows, err := preflight.IntegrationChecks(t.Context(), preflight.Input{Integrations: profile, Knobs: knobs, WireRunner: runner, ForgejoClientFactory: factory})
	if err != nil || len(rows) != 0 {
		t.Fatalf("local mode selected hosted probes: %+v, %v", rows, err)
	}
	tools, err := preflight.PrerequisitesForIntegrations(t.Context(), knobs, runner, profile)
	if err != nil {
		t.Fatal(err)
	}
	for _, tool := range tools {
		if tool.Name == "gh" {
			t.Fatal("local Git requires gh")
		}
	}
	for _, wiring := range []config.Wiring{{Repo: "create"}, {Protection: "required-checks"}, {Protection: "checks+review"}} {
		knobs.Wiring = wiring
		if _, err := preflight.PrerequisitesForIntegrations(t.Context(), knobs, runner, profile); err == nil {
			t.Fatalf("hosted administration accepted for local Git: %+v", wiring)
		}
	}
}
