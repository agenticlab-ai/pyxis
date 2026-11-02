package lifecycle

import (
	"context"
	"fmt"

	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/entity"
	"github.com/agenticlab-ai/pyxis/internal/executor/port"
	"github.com/agenticlab-ai/pyxis/internal/harnessconfig"
	"github.com/agenticlab-ai/pyxis/internal/nativerouting"
)

// PrototypeRoute retains the selected native profile and authenticated runtime
// for the build, so launch never needs to rediscover the catalog or route.
type PrototypeRoute struct {
	Source  string
	Harness entity.Harness
	Profile nativerouting.ProfileSpec
	Runtime port.AgentRuntime
}

func (r PrototypeRoute) String() string {
	return fmt.Sprintf("%s %s %s", r.Source, r.Harness, r.Profile.ID)
}

type PrototypeRouteResolver func(context.Context) (PrototypeRoute, error)

// ResolvePrototypeRoute uses only implementation profiles from the supplied
// effective snapshot. The caller constructs the runtime from that same snapshot.
func ResolvePrototypeRoute(ctx context.Context, configured *config.PrototypeSettings, catalog harnessconfig.Config, ambientBackend, ambientModel string, build func(entity.Harness) (port.AgentRuntime, error)) (PrototypeRoute, error) {
	route := PrototypeRoute{Source: "configured"}
	profileID := ""
	if configured != nil {
		route.Harness, profileID = configured.Harness, configured.Profile
	} else {
		route.Source = "ambient"
		if ambientBackend == "" || ambientModel == "" {
			return route, fmt.Errorf("prototype config block is absent: supply both --ambient-backend and --ambient-model, or set the prototype config block")
		}
		h, err := entity.ParseHarness(ambientBackend)
		if err != nil {
			return route, fmt.Errorf("resolve ambient backend (or set the prototype config block): %w", err)
		}
		route.Harness = h
	}
	for _, profile := range catalog.Profiles(route.Harness)[nativerouting.RoleImplementation] {
		if configured != nil {
			if profile.ID == profileID {
				route.Profile = profile
				break
			}
		} else if profile.Model == ambientModel && (route.Profile.ID == "" || profile.CostRank < route.Profile.CostRank) {
			route.Profile = profile
		}
	}
	if route.Profile.ID == "" {
		if configured != nil {
			return route, fmt.Errorf("prototype config block: harness %s implementation catalog does not hold profile %q", route.Harness, profileID)
		}
		return route, fmt.Errorf("ambient harness %s implementation catalog has no entry for model %q; set the prototype config block", route.Harness, ambientModel)
	}
	runtime, err := build(route.Harness)
	if err != nil {
		return route, fmt.Errorf("construct prototype runtime for %s: %w", route, err)
	}
	if runtime == nil {
		return route, fmt.Errorf("construct prototype runtime for %s: constructor returned no runtime", route)
	}
	if err := runtime.AuthCheck(ctx); err != nil {
		return route, fmt.Errorf("authenticate prototype route %s: %w", route, err)
	}
	route.Runtime = runtime
	return route, nil
}
