package nativerouting

import (
	"encoding/json"
	"fmt"
	"reflect"
)

// ParseRequest closed-decodes and validates native-route-request/v1.
func ParseRequest(data []byte) (Request, error) {
	var request Request
	if err := closedDecode(data, &request); err != nil {
		return Request{}, fmt.Errorf("parse native route request: %w", err)
	}
	if defects := requestDefects(request); len(defects) != 0 {
		return Request{}, fmt.Errorf("invalid native route request: %s at %s", defects[0].Code, defects[0].Field)
	}
	return request, nil
}

// CanonicalResolutionJSON emits the stable wire representation returned by
// the resolver. A malformed or caller-fabricated terminal is rejected.
func CanonicalResolutionJSON(resolution Resolution) ([]byte, error) {
	if err := validateResolution(resolution); err != nil {
		return nil, err
	}
	return json.Marshal(resolution)
}

// ParseResolution closed-decodes native-route-resolution/v1 and requires the
// same mutually exclusive terminal invariants as CanonicalResolutionJSON.
func ParseResolution(data []byte) (Resolution, error) {
	var resolution Resolution
	if err := closedDecode(data, &resolution); err != nil {
		return Resolution{}, fmt.Errorf("parse native route resolution: %w", err)
	}
	if err := validateResolution(resolution); err != nil {
		return Resolution{}, err
	}
	return resolution, nil
}

func validateResolution(r Resolution) error {
	if r.SchemaVersion != ResolutionSchema || r.AttemptID == "" || r.DemandID == "" || r.Harness == "" || !oneOf(r.Role, RoleImplementation, RoleReview) || !validProvides(requestedProvides(r.RequestedProfile)) || !oneOf(r.Mode, ModeNormal, ModeStrongestSatisfying) {
		return fmt.Errorf("invalid native route resolution envelope")
	}
	switch r.Class {
	case ResolutionExact:
		if r.CatalogID == "" || r.PolicyIdentity == "" || r.NativeProfile == nil || len(r.PromotedComponents) != 0 || len(r.UnmetComponents) != 0 || r.Reason != "" || len(r.PolicyDefects) != 0 {
			return fmt.Errorf("invalid exact native route resolution")
		}
	case ResolutionPromoted:
		if r.CatalogID == "" || r.PolicyIdentity == "" || r.NativeProfile == nil || len(r.PromotedComponents) == 0 || len(r.UnmetComponents) != 0 || r.Reason != "" || len(r.PolicyDefects) != 0 {
			return fmt.Errorf("invalid promoted native route resolution")
		}
	case ResolutionUnsupported:
		if r.CatalogID == "" || r.PolicyIdentity == "" || r.NativeProfile != nil || len(r.PromotedComponents) != 0 || len(r.UnmetComponents) == 0 || r.Reason != ReasonUnsupported || len(r.PolicyDefects) != 0 {
			return fmt.Errorf("invalid unsupported native route resolution")
		}
	case ResolutionPolicyInvalid:
		if r.NativeProfile != nil || len(r.PromotedComponents) != 0 || len(r.UnmetComponents) != 0 || r.Reason != ReasonPolicyInvalid || len(r.PolicyDefects) == 0 {
			return fmt.Errorf("invalid policy-invalid native route resolution")
		}
	default:
		return fmt.Errorf("unknown native route resolution class %q", r.Class)
	}
	if r.NativeProfile != nil {
		selected := r.NativeProfile
		if selected.CatalogID != r.CatalogID || selected.PolicyIdentity != r.PolicyIdentity || selected.Harness != r.Harness || selected.Role != r.Role {
			return fmt.Errorf("native profile does not belong to resolution policy")
		}
		if selected.NativeProfileID == "" || selected.NativeModel == "" || selected.NativeEffort == "" || selected.CostRank <= 0 || selected.QualificationID == "" || selected.QualificationDigest == "" || selected.AdapterGeneration == "" || !validProvides(selected.Provides) || validateControls(selected.Controls) != nil {
			return fmt.Errorf("native profile is incomplete")
		}
		if !controlsInCanonicalOrder(selected.Controls) {
			return fmt.Errorf("native profile controls are not canonical")
		}
		promoted, unmet := componentDifferences(selected.Provides, requestedProvides(r.RequestedProfile))
		if len(unmet) != 0 || !reflect.DeepEqual(promoted, r.PromotedComponents) {
			return fmt.Errorf("native profile does not exactly account for requested minima")
		}
	}
	lastComponent := -1
	for _, unmet := range r.UnmetComponents {
		position := componentPosition(unmet.Component)
		if position <= lastComponent || unmet.Requested != providedLevel(requestedProvides(r.RequestedProfile), unmet.Component) || levelIndex(unmet.Component, unmet.Available) < 0 || levelIndex(unmet.Component, unmet.Available) >= levelIndex(unmet.Component, unmet.Requested) {
			return fmt.Errorf("unmet components are incomplete or non-canonical")
		}
		lastComponent = position
	}
	if !reflect.DeepEqual(r.PolicyDefects, canonicalPolicyDefects(append([]PolicyDefect(nil), r.PolicyDefects...))) {
		return fmt.Errorf("policy defects are not canonical")
	}
	return nil
}

func componentPosition(component Component) int {
	for i, candidate := range componentOrder {
		if component == candidate {
			return i
		}
	}
	return -1
}

func controlsInCanonicalOrder(controls []NativeControl) bool {
	for i := 1; i < len(controls); i++ {
		if controls[i-1].Name >= controls[i].Name {
			return false
		}
	}
	return true
}
