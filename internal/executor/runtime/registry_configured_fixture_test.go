package runtime

import (
	"slices"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/entity"
	"github.com/agenticlab-ai/pyxis/internal/executor/port"
	"github.com/agenticlab-ai/pyxis/internal/nativerouting"
)

const configuredFixture entity.Harness = "configured-fixture"

func init() {
	Register(
		configuredFixture,
		func() (port.AgentRuntime, error) { return &stubRuntime{}, nil },
		func(map[nativerouting.Role][]nativerouting.ProfileSpec) (port.AgentRuntime, error) {
			return &stubRuntime{}, nil
		},
	)
}

func TestAdditionalConfiguredRegistrationNeedsNoCentralSwitch(t *testing.T) {
	if _, err := ResolveConfigured(configuredFixture, configuredProfiles()); err != nil {
		t.Fatal(err)
	}
	names := Harnesses()
	if !slices.IsSorted(names) || !slices.Contains(names, configuredFixture) {
		t.Fatalf("Harnesses() = %v, want sorted set containing %q", names, configuredFixture)
	}
}
