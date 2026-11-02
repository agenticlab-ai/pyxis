package nativerouting

import (
	"fmt"
	"github.com/agenticlab-ai/pyxis/internal/entity"
	"sort"

	"github.com/agenticlab-ai/pyxis/internal/executiondemand"
)

const (
	RequestSchema    = "native-route-request/v1"
	ResolutionSchema = "native-route-resolution/v1"
)

type Mode string
type ResolutionClass string
type ResolutionReason string

const (
	ModeNormal              Mode             = "normal"
	ModeStrongestSatisfying Mode             = "strongest_satisfying"
	ResolutionExact         ResolutionClass  = "exact"
	ResolutionPromoted      ResolutionClass  = "promoted"
	ResolutionUnsupported   ResolutionClass  = "unsupported"
	ResolutionPolicyInvalid ResolutionClass  = "policy_invalid"
	ReasonUnsupported       ResolutionReason = "unsupported"
	ReasonPolicyInvalid     ResolutionReason = "policy_invalid"
)

// Request is a closed request for one already-selected harness and role. The
// caller verifies the execution demand before constructing it; the resolver
// checks that the carried minimum is complete and uses no other demand source.
type Request struct {
	SchemaVersion    string                         `json:"schema_version"`
	AttemptID        string                         `json:"attempt_id"`
	DemandID         executiondemand.DemandID       `json:"demand_id"`
	Harness          entity.Harness                 `json:"harness"`
	Role             Role                           `json:"role"`
	RequestedProfile executiondemand.ProfileMinimum `json:"requested_profile"`
	Mode             Mode                           `json:"mode"`
}

// SelectedProfile is the complete immutable launch selection copied from one
// validated catalog entry. Adapters receive this value; they do not receive
// semantic demand and therefore cannot choose a different native profile.
type SelectedProfile struct {
	CatalogID           string          `json:"catalog_id"`
	PolicyIdentity      string          `json:"policy_identity"`
	Harness             entity.Harness  `json:"harness"`
	Role                Role            `json:"role"`
	NativeProfileID     string          `json:"native_profile_id"`
	NativeModel         string          `json:"native_model"`
	NativeEffort        string          `json:"native_effort"`
	Controls            []NativeControl `json:"controls"`
	Provides            Provides        `json:"provides"`
	CostRank            int             `json:"cost_rank"`
	QualificationID     string          `json:"qualification_id"`
	QualificationDigest string          `json:"qualification_digest"`
	AdapterGeneration   string          `json:"adapter_generation"`
}

// UnmetComponent compares one requested minimum with the greatest provision
// in the catalog. A validated catalog has one componentwise-greatest entry, so
// this reports exactly every reason no candidate can dominate the request.
type UnmetComponent struct {
	Component Component `json:"component"`
	Requested string    `json:"requested"`
	Available string    `json:"available"`
}

// PolicyDefect is a stable, data-only explanation for a policy-invalid
// terminal. It deliberately carries no adapter error or incidental input order.
type PolicyDefect struct {
	Code  string `json:"code"`
	Field string `json:"field"`
}

// Resolution is the closed terminal grammar. Exactly one NativeProfile is
// present for exact/promoted; unsupported and policy-invalid always carry nil.
// Slice fields are always non-nil so canonical JSON never varies between null
// and an empty list.
type Resolution struct {
	SchemaVersion      string                         `json:"schema_version"`
	AttemptID          string                         `json:"attempt_id"`
	DemandID           executiondemand.DemandID       `json:"demand_id"`
	Harness            entity.Harness                 `json:"harness"`
	Role               Role                           `json:"role"`
	RequestedProfile   executiondemand.ProfileMinimum `json:"requested_profile"`
	CatalogID          string                         `json:"catalog_id"`
	PolicyIdentity     string                         `json:"policy_identity"`
	Mode               Mode                           `json:"mode"`
	Class              ResolutionClass                `json:"class"`
	NativeProfile      *SelectedProfile               `json:"native_profile"`
	PromotedComponents []Component                    `json:"promoted_components"`
	UnmetComponents    []UnmetComponent               `json:"unmet_components"`
	Reason             ResolutionReason               `json:"reason,omitempty"`
	PolicyDefects      []PolicyDefect                 `json:"policy_defects"`
}

type Resolver struct{}

var componentOrder = [...]Component{
	ComponentCapability,
	ComponentDeliberation,
	ComponentContext,
	ComponentAssurance,
}

func requestedProvides(p executiondemand.ProfileMinimum) Provides {
	return Provides{Capability: p.Capability, Deliberation: p.Deliberation, Context: p.Context, Assurance: p.Assurance}
}

func baseResolution(request Request, catalog ValidatedCatalog) Resolution {
	return Resolution{
		SchemaVersion: ResolutionSchema, AttemptID: request.AttemptID, DemandID: request.DemandID,
		Harness: request.Harness, Role: request.Role, RequestedProfile: request.RequestedProfile,
		CatalogID: catalog.Catalog.CatalogID, PolicyIdentity: catalog.Identity, Mode: request.Mode,
		PromotedComponents: []Component{}, UnmetComponents: []UnmetComponent{}, PolicyDefects: []PolicyDefect{},
	}
}

func requestDefects(request Request) []PolicyDefect {
	var defects []PolicyDefect
	if request.SchemaVersion != RequestSchema {
		defects = append(defects, PolicyDefect{Code: "unknown_schema_version", Field: "request.schema_version"})
	}
	if request.AttemptID == "" {
		defects = append(defects, PolicyDefect{Code: "missing_identity", Field: "request.attempt_id"})
	}
	if request.DemandID == "" {
		defects = append(defects, PolicyDefect{Code: "missing_identity", Field: "request.demand_id"})
	}
	if request.Harness == "" {
		defects = append(defects, PolicyDefect{Code: "missing_identity", Field: "request.harness"})
	}
	if !oneOf(request.Role, RoleImplementation, RoleReview) {
		defects = append(defects, PolicyDefect{Code: "unknown_role", Field: "request.role"})
	}
	if !validProvides(requestedProvides(request.RequestedProfile)) {
		defects = append(defects, PolicyDefect{Code: "unknown_semantic_value", Field: "request.requested_profile"})
	}
	if !oneOf(request.Mode, ModeNormal, ModeStrongestSatisfying) {
		defects = append(defects, PolicyDefect{Code: "unknown_mode", Field: "request.mode"})
	}
	return defects
}

func catalogDefects(request Request, catalog ValidatedCatalog) []PolicyDefect {
	var defects []PolicyDefect
	c := catalog.Catalog
	if catalog.Identity == "" {
		defects = append(defects, PolicyDefect{Code: "missing_policy_identity", Field: "catalog.policy_identity"})
	}
	if c.SchemaVersion != CatalogSchema || c.CatalogID == "" || c.AdapterGeneration == "" || len(c.Entries) == 0 {
		defects = append(defects, PolicyDefect{Code: "invalid_catalog_identity", Field: "catalog"})
	}
	if c.Harness != request.Harness {
		defects = append(defects, PolicyDefect{Code: "harness_mismatch", Field: "catalog.harness"})
	}
	if c.Role != request.Role {
		defects = append(defects, PolicyDefect{Code: "role_mismatch", Field: "catalog.role"})
	}
	profiles, ranks := map[string]int{}, map[int]int{}
	var strongest []int
	for i, entry := range c.Entries {
		profiles[entry.NativeProfileID]++
		ranks[entry.CostRank]++
		if entry.Strongest {
			strongest = append(strongest, i)
		}
	}
	for _, entry := range c.Entries {
		field := fmt.Sprintf("catalog.entries[%s]", entry.NativeProfileID)
		if entry.NativeProfileID == "" || profiles[entry.NativeProfileID] > 1 {
			defects = append(defects, PolicyDefect{Code: "duplicate_profile_identity", Field: field + ".native_profile_id"})
		}
		if entry.CostRank <= 0 || ranks[entry.CostRank] > 1 {
			defects = append(defects, PolicyDefect{Code: "duplicate_cost_rank", Field: field + ".cost_rank"})
		}
		if !validProvides(entry.Provides) {
			defects = append(defects, PolicyDefect{Code: "unknown_semantic_value", Field: field + ".provides"})
		}
		if entry.NativeModel == "" || entry.NativeEffort == "" || entry.QualificationID == "" || entry.QualificationDigest == "" || entry.AdapterGeneration != c.AdapterGeneration {
			defects = append(defects, PolicyDefect{Code: "incomplete_validated_entry", Field: field})
		}
		if err := validateControls(entry.Controls); err != nil {
			defects = append(defects, PolicyDefect{Code: "invalid_typed_control", Field: field + ".controls"})
		}
	}
	if len(strongest) != 1 {
		defects = append(defects, PolicyDefect{Code: "ambiguous_strongest_profile", Field: "catalog.entries.strongest"})
	} else {
		for i, entry := range c.Entries {
			if i != strongest[0] && (!dominates(c.Entries[strongest[0]].Provides, entry.Provides) || c.Entries[strongest[0]].Provides == entry.Provides) {
				defects = append(defects, PolicyDefect{Code: "ambiguous_strongest_profile", Field: "catalog.entries.strongest"})
				break
			}
		}
	}
	return canonicalPolicyDefects(defects)
}

func canonicalPolicyDefects(defects []PolicyDefect) []PolicyDefect {
	sort.Slice(defects, func(i, j int) bool {
		if defects[i].Field != defects[j].Field {
			return defects[i].Field < defects[j].Field
		}
		return defects[i].Code < defects[j].Code
	})
	out := make([]PolicyDefect, 0, len(defects))
	for _, defect := range defects {
		if len(out) == 0 || out[len(out)-1] != defect {
			out = append(out, defect)
		}
	}
	return out
}

func cloneControls(controls []NativeControl) []NativeControl {
	if len(controls) == 0 {
		return []NativeControl{}
	}
	out := make([]NativeControl, len(controls))
	for i, control := range controls {
		out[i] = control
		if control.IntegerValue != nil {
			value := *control.IntegerValue
			out[i].IntegerValue = &value
		}
		if control.BooleanValue != nil {
			value := *control.BooleanValue
			out[i].BooleanValue = &value
		}
	}
	return canonicalControls(out)
}

func selectedProfile(catalog ValidatedCatalog, entry CatalogEntry) SelectedProfile {
	return SelectedProfile{
		CatalogID: catalog.Catalog.CatalogID, PolicyIdentity: catalog.Identity,
		Harness: catalog.Catalog.Harness, Role: catalog.Catalog.Role,
		NativeProfileID: entry.NativeProfileID, NativeModel: entry.NativeModel, NativeEffort: entry.NativeEffort,
		Controls: cloneControls(entry.Controls), Provides: entry.Provides, CostRank: entry.CostRank,
		QualificationID: entry.QualificationID, QualificationDigest: entry.QualificationDigest,
		AdapterGeneration: entry.AdapterGeneration,
	}
}

func componentDifferences(available, requested Provides) (promoted []Component, unmet []UnmetComponent) {
	promoted = []Component{}
	unmet = []UnmetComponent{}
	for _, component := range componentOrder {
		have, want := providedLevel(available, component), providedLevel(requested, component)
		switch {
		case levelIndex(component, have) > levelIndex(component, want):
			promoted = append(promoted, component)
		case levelIndex(component, have) < levelIndex(component, want):
			unmet = append(unmet, UnmetComponent{Component: component, Requested: want, Available: have})
		}
	}
	return promoted, unmet
}

// Resolve deterministically resolves one already-selected harness-role policy.
// It performs no I/O and has no injectable inference, clock, or network seam.
func (Resolver) Resolve(request Request, catalog ValidatedCatalog) Resolution {
	resolution := baseResolution(request, catalog)
	defects := canonicalPolicyDefects(append(requestDefects(request), catalogDefects(request, catalog)...))
	if len(defects) != 0 {
		resolution.Class, resolution.Reason, resolution.PolicyDefects = ResolutionPolicyInvalid, ReasonPolicyInvalid, defects
		return resolution
	}

	requested := requestedProvides(request.RequestedProfile)
	entries := catalog.Entries()
	var strongest CatalogEntry
	for _, entry := range entries {
		if entry.Strongest {
			strongest = entry
			break
		}
	}

	candidates := make([]CatalogEntry, 0, len(entries))
	if request.Mode == ModeStrongestSatisfying {
		if dominates(strongest.Provides, requested) {
			candidates = append(candidates, strongest)
		}
	} else {
		for _, entry := range entries {
			if dominates(entry.Provides, requested) {
				candidates = append(candidates, entry)
			}
		}
		sort.Slice(candidates, func(i, j int) bool {
			if candidates[i].CostRank != candidates[j].CostRank {
				return candidates[i].CostRank < candidates[j].CostRank
			}
			return candidates[i].NativeProfileID < candidates[j].NativeProfileID
		})
	}

	if len(candidates) == 0 {
		_, resolution.UnmetComponents = componentDifferences(strongest.Provides, requested)
		resolution.Class, resolution.Reason = ResolutionUnsupported, ReasonUnsupported
		return resolution
	}

	selected := selectedProfile(catalog, candidates[0])
	resolution.NativeProfile = &selected
	resolution.PromotedComponents, _ = componentDifferences(selected.Provides, requested)
	if len(resolution.PromotedComponents) == 0 {
		resolution.Class = ResolutionExact
	} else {
		resolution.Class = ResolutionPromoted
	}
	return resolution
}
