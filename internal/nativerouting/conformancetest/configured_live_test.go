package conformancetest

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	_ "github.com/agenticlab-ai/pyxis/internal/adapters/harness/claude"
	_ "github.com/agenticlab-ai/pyxis/internal/adapters/harness/codex"
	_ "github.com/agenticlab-ai/pyxis/internal/adapters/harness/cursor"
	_ "github.com/agenticlab-ai/pyxis/internal/adapters/harness/opencode"
	"github.com/agenticlab-ai/pyxis/internal/entity"
	"github.com/agenticlab-ai/pyxis/internal/executiondemand"
	"github.com/agenticlab-ai/pyxis/internal/executor/port"
	"github.com/agenticlab-ai/pyxis/internal/executor/runtime"
	"github.com/agenticlab-ai/pyxis/internal/nativerouting"
)

type configuredRegistry struct {
	registry runtime.Registry
	profiles map[entity.Harness]map[nativerouting.Role][]nativerouting.ProfileSpec
}

func (r configuredRegistry) Harnesses() []entity.Harness { return r.registry.Harnesses() }

func (r configuredRegistry) Resolve(harness entity.Harness) (port.AgentRuntime, error) {
	return r.registry.ResolveConfigured(harness, r.profiles[harness])
}

func TestConfiguredLiveRegistryIgnoresAmbientCatalogAndPassesConformance(t *testing.T) {
	ambient := []byte("harness-config:\n  codex:\n    implementation:\n      ambient-only: [malformed\n")
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".pyxis"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".pyxis", "config.yaml"), ambient, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Chdir(root)

	profiles := explicitProfiles()
	want := cloneHarnessProfiles(profiles)
	registry := configuredRegistry{registry: runtime.LiveRegistry(), profiles: profiles}
	if got := registry.Harnesses(); len(got) != 4 {
		t.Fatalf("live configured registry harnesses = %v, want four adapters", got)
	}
	owners := make(map[entity.Harness]port.CatalogProvider, len(registry.Harnesses()))
	for _, harness := range registry.Harnesses() {
		rt, err := registry.Resolve(harness)
		if err != nil {
			t.Fatalf("ResolveConfigured(%s): %v", harness, err)
		}
		owner, ok := rt.(port.CatalogProvider)
		if !ok {
			t.Fatalf("%s configured runtime has no catalog", harness)
		}
		owners[harness] = owner
	}

	for harness, roles := range profiles {
		for role := range roles {
			profiles[harness][role][0].ID = "mutated"
			profiles[harness][role][0].Model = "ambient/model"
			profiles[harness][role] = append(profiles[harness][role], nativerouting.ProfileSpec{ID: "late-entry"})
		}
	}
	for harness, owner := range owners {
		var identities []string
		for _, role := range []nativerouting.Role{nativerouting.RoleImplementation, nativerouting.RoleReview} {
			catalog, qualifications, err := owner.Catalog(role)
			if err != nil {
				t.Fatalf("%s/%s Catalog: %v", harness, role, err)
			}
			if len(catalog.Entries) != 1 {
				t.Fatalf("%s/%s inherited ambient or later entries: %+v", harness, role, catalog.Entries)
			}
			entry := catalog.Entries[0]
			supplied := want[harness][role][0]
			if entry.NativeProfileID != supplied.ID || entry.NativeModel != supplied.Model || entry.NativeEffort != supplied.Effort {
				t.Fatalf("%s/%s catalog entry = %+v, want supplied identity/model/effort %+v", harness, role, entry, supplied)
			}
			validated, diagnostics := nativerouting.ValidateCatalog(catalog, qualifications, owner.(nativerouting.NativeCapability), fakeClock{time.Date(2026, 8, 5, 12, 0, 0, 0, time.UTC)}, liveAuthorization{})
			if len(diagnostics) != 0 {
				t.Fatalf("%s/%s explicit catalog invalid: %v", harness, role, diagnostics)
			}
			identities = append(identities, validated.Identity)
		}
		if identities[0] == identities[1] {
			t.Fatalf("%s implementation and review catalogs share identity", harness)
		}
	}

	registry.profiles = want
	domain, err := GenerateDomain(executiondemand.DefaultPolicy, registry, Roles{nativerouting.RoleImplementation, nativerouting.RoleReview})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := Check(domain, registry, fakeClock{time.Date(2026, 8, 5, 12, 0, 0, 0, time.UTC)}, liveAuthorization{}); err != nil {
		t.Fatalf("configured live conformance: %v", err)
	}
}

func TestConfiguredCatalogDefectsAndUnsupportedDemandRefuseAtNativeBoundary(t *testing.T) {
	profiles := explicitProfiles()[entity.Codex]
	profiles[nativerouting.RoleImplementation][0].ID = ""
	rt, err := runtime.ResolveConfigured(entity.Codex, profiles)
	if err != nil {
		t.Fatal(err)
	}
	owner := rt.(interface {
		port.CatalogProvider
		nativerouting.NativeCapability
	})
	catalog, qualifications, err := owner.Catalog(nativerouting.RoleImplementation)
	if err != nil {
		t.Fatal(err)
	}
	if _, diagnostics := nativerouting.ValidateCatalog(catalog, qualifications, owner, fakeClock{time.Date(2026, 8, 5, 12, 0, 0, 0, time.UTC)}, liveAuthorization{}); len(diagnostics) == 0 {
		t.Fatal("invalid explicit catalog passed native validation")
	}

	weak := explicitProfiles()[entity.Codex]
	weak[nativerouting.RoleImplementation][0].Provides = nativerouting.Provides{
		Capability: executiondemand.CapabilityEconomy, Deliberation: executiondemand.DeliberationLight,
		Context: executiondemand.ContextStandard, Assurance: executiondemand.AssuranceOrdinary,
	}
	rt, err = runtime.ResolveConfigured(entity.Codex, weak)
	if err != nil {
		t.Fatal(err)
	}
	owner = rt.(interface {
		port.CatalogProvider
		nativerouting.NativeCapability
	})
	catalog, qualifications, err = owner.Catalog(nativerouting.RoleImplementation)
	if err != nil {
		t.Fatal(err)
	}
	validated, diagnostics := nativerouting.ValidateCatalog(catalog, qualifications, owner, fakeClock{time.Date(2026, 8, 5, 12, 0, 0, 0, time.UTC)}, liveAuthorization{})
	if len(diagnostics) != 0 {
		t.Fatalf("weak explicit catalog invalid: %v", diagnostics)
	}
	resolution := (nativerouting.Resolver{}).Resolve(nativerouting.Request{
		SchemaVersion: nativerouting.RequestSchema, AttemptID: "unsupported-explicit", DemandID: "unsupported-explicit",
		Harness: entity.Codex, Role: nativerouting.RoleImplementation, Mode: nativerouting.ModeNormal,
		RequestedProfile: executiondemand.ProfileMinimum{
			Capability: executiondemand.CapabilityFrontier, Deliberation: executiondemand.DeliberationMaximum,
			Context: executiondemand.ContextExtended, Assurance: executiondemand.AssuranceCritical,
		},
	}, validated)
	if resolution.Class != nativerouting.ResolutionUnsupported || resolution.NativeProfile != nil {
		t.Fatalf("unsupported request resolved as %+v", resolution)
	}
}

func explicitProfiles() map[entity.Harness]map[nativerouting.Role][]nativerouting.ProfileSpec {
	strongest := nativerouting.Provides{
		Capability: executiondemand.CapabilityFrontier, Deliberation: executiondemand.DeliberationMaximum,
		Context: executiondemand.ContextExtended, Assurance: executiondemand.AssuranceCritical,
	}
	profile := func(id, model, effort string) []nativerouting.ProfileSpec {
		return []nativerouting.ProfileSpec{{ID: id, Model: model, Effort: effort, Provides: strongest, CostRank: 1, Strongest: true}}
	}
	return map[entity.Harness]map[nativerouting.Role][]nativerouting.ProfileSpec{
		entity.Claude: {
			nativerouting.RoleImplementation: profile("captured-claude-implementation", "claude-opus-5", "max"),
			nativerouting.RoleReview:         profile("captured-claude-review", "claude-opus-5", "high"),
		},
		entity.Codex: {
			nativerouting.RoleImplementation: profile("captured-codex-implementation", "gpt-5.6-sol", "xhigh"),
			nativerouting.RoleReview:         profile("captured-codex-review", "gpt-5.6-sol", "high"),
		},
		entity.Cursor: {
			nativerouting.RoleImplementation: profile("captured-cursor-implementation", "cursor-grok-4.5-high", "high"),
			nativerouting.RoleReview:         profile("captured-cursor-review", "cursor-grok-4.5-medium", "medium"),
		},
		entity.OpenCode: {
			nativerouting.RoleImplementation: profile("captured-opencode-implementation", "anthropic/claude-opus-4-8", "none"),
			nativerouting.RoleReview:         profile("captured-opencode-review", "anthropic/claude-sonnet-5", "none"),
		},
	}
}

func cloneHarnessProfiles(in map[entity.Harness]map[nativerouting.Role][]nativerouting.ProfileSpec) map[entity.Harness]map[nativerouting.Role][]nativerouting.ProfileSpec {
	out := make(map[entity.Harness]map[nativerouting.Role][]nativerouting.ProfileSpec, len(in))
	for harness, profiles := range in {
		out[harness] = nativerouting.CloneProfiles(profiles)
	}
	return out
}
