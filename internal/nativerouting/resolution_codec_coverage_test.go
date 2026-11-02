package nativerouting

import (
	"slices"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/executiondemand"
)

func TestResolutionValidationRejectsEveryTerminalShape(t *testing.T) {
	requested := executiondemand.ProfileMinimum{
		Capability: executiondemand.CapabilityEconomy, Deliberation: executiondemand.DeliberationLight,
		Context: executiondemand.ContextStandard, Assurance: executiondemand.AssuranceOrdinary,
	}
	base := (Resolver{}).Resolve(resolverRequest(requested), promotionCatalog())
	mutations := []func(*Resolution){
		func(r *Resolution) { r.Class = ResolutionClass("unknown") },
		func(r *Resolution) { r.Class, r.PromotedComponents = ResolutionExact, []Component{ComponentCapability} },
		func(r *Resolution) { r.Class, r.NativeProfile = ResolutionUnsupported, nil },
		func(r *Resolution) {
			r.Class, r.Reason, r.PolicyDefects = ResolutionPolicyInvalid, ReasonPolicyInvalid, []PolicyDefect{{Code: "z"}}
		},
		func(r *Resolution) { r.NativeProfile.Harness = "other" },
		func(r *Resolution) { r.NativeProfile.NativeModel = "" },
		func(r *Resolution) {
			slices.Reverse(r.NativeProfile.Controls)
			r.NativeProfile.Controls = append(r.NativeProfile.Controls, NativeControl{Name: "a"})
		},
		func(r *Resolution) { r.PromotedComponents = nil },
	}
	for i, mutate := range mutations {
		candidate := base
		profile := *base.NativeProfile
		profile.Controls = append([]NativeControl(nil), profile.Controls...)
		candidate.NativeProfile = &profile
		mutate(&candidate)
		if _, err := CanonicalResolutionJSON(candidate); err == nil {
			t.Fatalf("mutation %d was accepted: %+v", i, candidate)
		}
	}
	if got := componentPosition(Component("unknown")); got != -1 {
		t.Fatalf("unknown component position = %d", got)
	}
	if controlsInCanonicalOrder([]NativeControl{{Name: "b"}, {Name: "a"}}) {
		t.Fatal("accepted non-canonical controls")
	}
}
