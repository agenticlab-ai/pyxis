// Package taskrequirements owns the portable planning contract. It records
// task assessments and their derived requirements, never a provider selection.
package taskrequirements

import (
	"fmt"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/executiondemand"
	"github.com/agenticlab-ai/pyxis/internal/taskrouting"
)

type SchemaVersion string

const Schema SchemaVersion = "task-requirements/v1"

// Role contains six typed assessment axes and four derived requirement axes.
// Profile may be omitted by authors; Compile fills it. Stored tasks require it.
type Role struct {
	Assessment executiondemand.Assessment `json:"assessment"`
	Profile    *executiondemand.Profile   `json:"profile,omitempty"`
	Rationale  string                     `json:"rationale"`
}

type Spec struct {
	Schema         SchemaVersion `json:"schema_version"`
	Implementation Role          `json:"implementation"`
	Review         Role          `json:"review"`
}

// Sources names the task's charter sections and its declared requirement/design
// traces. The caller owns validation of those traces against the source contract.
func Sources(traces []string) executiondemand.EvidenceSet {
	e := executiondemand.EvidenceSet{Resolved: map[string]struct{}{
		"task:goal": {}, "task:scope": {}, "task:acceptance": {},
	}}
	for _, trace := range traces {
		e.Resolved[trace] = struct{}{}
	}
	return e
}

// Parse reads the closed authoring object; compilation occurs once the complete
// task charter and its traces are known. Unknown and duplicate keys are errors.
func Parse(b []byte) (Spec, error) {
	var s Spec
	if err := taskrouting.Decode(b, &s); err != nil {
		return Spec{}, fmt.Errorf("task requirements: %w", err)
	}
	return s, nil
}

// Compile derives omitted profiles and verifies supplied ones, including their
// applied rules and policy identity. It does not mutate the authored object.
func (s Spec) Compile(e executiondemand.EvidenceSet) (Spec, error) {
	if s.Schema != Schema {
		return Spec{}, fmt.Errorf("task requirements: unsupported schema %q", s.Schema)
	}
	for _, r := range []struct {
		name  string
		value *Role
	}{
		{"implementation", &s.Implementation}, {"review", &s.Review},
	} {
		if strings.TrimSpace(r.value.Rationale) == "" {
			return Spec{}, fmt.Errorf("task requirements: %s rationale is required", r.name)
		}
		p, err := executiondemand.Evaluate(r.value.Assessment, e, executiondemand.AssuranceRaise{}, executiondemand.DefaultPolicy)
		if err != nil {
			return Spec{}, fmt.Errorf("task requirements: %s: %w", r.name, err)
		}
		if r.value.Profile != nil {
			if err := executiondemand.VerifyProfile(r.value.Assessment, e, executiondemand.AssuranceRaise{}, executiondemand.DefaultPolicy, *r.value.Profile); err != nil {
				return Spec{}, fmt.Errorf("task requirements: %s: %w", r.name, err)
			}
		}
		r.value.Profile = &p
	}
	return s, nil
}

// Validate is the persisted-data boundary: an omitted derived profile cannot be
// repaired on readback, and mismatched or stale derivation provenance is refused.
func (s Spec) Validate(e executiondemand.EvidenceSet) error {
	if s.Implementation.Profile == nil || s.Review.Profile == nil {
		return fmt.Errorf("task requirements: both derived profiles are required")
	}
	_, err := s.Compile(e)
	return err
}

// Summary is plain text; HTML consumers must escape it at their output boundary.
func (r Role) Summary() string {
	a := r.Assessment
	var s strings.Builder
	fmt.Fprintf(&s, "reasoning=%s; coupling=%s; novelty=%s; verification=%s; impact_scope=%s; architectural_role=%s", a.Reasoning, a.Coupling, a.Novelty, a.Verification, a.ImpactScope, a.ArchitecturalRole)
	if p := r.Profile; p != nil {
		fmt.Fprintf(&s, "; capability=%s; deliberation=%s; context=%s; assurance=%s; policy=%s (%s)", p.Capability, p.Deliberation, p.Context, p.Assurance, p.PolicyVersion, p.PolicyDigest)
	}
	for _, c := range a.Citations {
		s.WriteString("; evidence=" + c.ID)
	}
	s.WriteString("; rationale=" + r.Rationale)
	return s.String()
}
