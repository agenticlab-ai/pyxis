package runtime

import (
	"errors"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/entity"
	"github.com/agenticlab-ai/pyxis/internal/executor/port"
	"github.com/agenticlab-ai/pyxis/internal/nativerouting"
)

func configuredProfiles() map[nativerouting.Role][]nativerouting.ProfileSpec {
	return map[nativerouting.Role][]nativerouting.ProfileSpec{
		nativerouting.RoleImplementation: {{ID: "explicit", Model: "model", Effort: "high"}},
	}
}

func TestRegistryConfiguredResolutionRefusals(t *testing.T) {
	wantErr := errors.New("configured failure")
	tests := []struct {
		name     string
		register func(*registry, *int)
		harness  string
		profiles map[nativerouting.Role][]nativerouting.ProfileSpec
		want     string
	}{
		{name: "unknown", register: func(*registry, *int) {}, harness: "missing", profiles: configuredProfiles(), want: "unknown harness"},
		{name: "nil profiles", register: configuredTestRegistration, harness: "configured", want: "profile catalog is empty"},
		{name: "empty profiles", register: configuredTestRegistration, harness: "configured", profiles: map[nativerouting.Role][]nativerouting.ProfileSpec{nativerouting.RoleImplementation: {}}, want: "profile catalog is empty"},
		{name: "legacy only", register: func(r *registry, calls *int) { r.register("configured", countingFactory(calls)) }, harness: "configured", profiles: configuredProfiles(), want: "does not support configured resolution"},
		{name: "factory error", register: func(r *registry, calls *int) {
			r.register("configured", countingFactory(calls), func(map[nativerouting.Role][]nativerouting.ProfileSpec) (port.AgentRuntime, error) {
				return nil, wantErr
			})
		}, harness: "configured", profiles: configuredProfiles(), want: "configured failure"},
		{name: "nil runtime", register: func(r *registry, calls *int) {
			r.register("configured", countingFactory(calls), func(map[nativerouting.Role][]nativerouting.ProfileSpec) (port.AgentRuntime, error) { return nil, nil })
		}, harness: "configured", profiles: configuredProfiles(), want: "nil runtime"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newRegistry()
			legacyCalls := 0
			tt.register(r, &legacyCalls)
			_, err := r.resolveConfigured(entity.Harness(tt.harness), tt.profiles)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("resolveConfigured error = %v, want containing %q", err, tt.want)
			}
			if tt.name == "factory error" && !errors.Is(err, wantErr) {
				t.Fatalf("resolveConfigured error = %v, want wrapped configured error", err)
			}
			if legacyCalls != 0 {
				t.Fatalf("legacy factory called %d times", legacyCalls)
			}
		})
	}
}

func configuredTestRegistration(r *registry, calls *int) {
	r.register("configured", countingFactory(calls), func(map[nativerouting.Role][]nativerouting.ProfileSpec) (port.AgentRuntime, error) {
		return &stubRuntime{}, nil
	})
}

func countingFactory(calls *int) Factory {
	return func() (port.AgentRuntime, error) {
		(*calls)++
		return &stubRuntime{}, nil
	}
}

func TestRegistryConfiguredDoesNotChangeLegacyResolution(t *testing.T) {
	r := newRegistry()
	legacyCalls := 0
	configuredCalls := 0
	r.register("configured", countingFactory(&legacyCalls), func(map[nativerouting.Role][]nativerouting.ProfileSpec) (port.AgentRuntime, error) {
		configuredCalls++
		return &stubRuntime{}, nil
	})
	if _, err := r.resolve("configured"); err != nil {
		t.Fatal(err)
	}
	if legacyCalls != 1 || configuredCalls != 0 {
		t.Fatalf("legacy calls=%d configured calls=%d", legacyCalls, configuredCalls)
	}
}

func TestConfiguredZeroRegistryFailsClosed(t *testing.T) {
	var registry Registry
	if _, err := registry.ResolveConfigured("codex", configuredProfiles()); !errors.Is(err, ErrUnknownHarness) {
		t.Fatalf("zero registry ResolveConfigured error = %v, want ErrUnknownHarness", err)
	}
}

func TestRegistryRejectsInvalidConfiguredRegistration(t *testing.T) {
	for _, register := range []func(*registry){
		func(r *registry) { r.register("configured", countingFactory(new(int)), nil) },
		func(r *registry) {
			r.register("configured", countingFactory(new(int)),
				func(map[nativerouting.Role][]nativerouting.ProfileSpec) (port.AgentRuntime, error) {
					return &stubRuntime{}, nil
				},
				func(map[nativerouting.Role][]nativerouting.ProfileSpec) (port.AgentRuntime, error) {
					return &stubRuntime{}, nil
				},
			)
		},
	} {
		func() {
			defer func() {
				if recover() == nil {
					t.Error("invalid configured registration did not panic")
				}
			}()
			register(newRegistry())
		}()
	}
}
