// Package executiondemand derives vendor-neutral execution minima from a
// closed, evidence-backed task assessment. It deliberately has no runtime or
// provider dependencies.
package executiondemand

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

type Reasoning string
type Coupling string
type Novelty string
type Verification string
type ImpactScope string
type ArchitecturalRole string
type Capability string
type Deliberation string
type Context string
type Assurance string

const (
	ReasoningRoutine          Reasoning         = "routine"
	ReasoningComplex          Reasoning         = "complex"
	ReasoningDifficult        Reasoning         = "difficult"
	CouplingContained         Coupling          = "contained"
	CouplingComposed          Coupling          = "composed"
	CouplingCrossBoundary     Coupling          = "cross_boundary"
	NoveltyKnown              Novelty           = "known"
	NoveltyAdapt              Novelty           = "adapt"
	NoveltyFirstOfKind        Novelty           = "first_of_kind"
	VerificationLocal         Verification      = "local"
	VerificationComposed      Verification      = "composed"
	VerificationCrossBoundary Verification      = "cross_boundary"
	ImpactContained           ImpactScope       = "contained"
	ImpactMultiSurface        ImpactScope       = "multi_surface"
	ImpactSystemWide          ImpactScope       = "system_wide"
	RoleFeatureLocal          ArchitecturalRole = "feature_local"
	RoleSharedSubstrate       ArchitecturalRole = "shared_substrate"
	RoleSystemCore            ArchitecturalRole = "system_core"
	CapabilityEconomy         Capability        = "economy"
	CapabilityStandard        Capability        = "standard"
	CapabilityStrong          Capability        = "strong"
	CapabilityFrontier        Capability        = "frontier"
	DeliberationLight         Deliberation      = "light"
	DeliberationStandard      Deliberation      = "standard"
	DeliberationDeep          Deliberation      = "deep"
	DeliberationMaximum       Deliberation      = "maximum"
	ContextStandard           Context           = "standard"
	ContextLarge              Context           = "large"
	ContextExtended           Context           = "extended"
	AssuranceOrdinary         Assurance         = "ordinary"
	AssuranceElevated         Assurance         = "elevated"
	AssuranceCritical         Assurance         = "critical"
)

type Assessment struct {
	Reasoning         Reasoning         `json:"reasoning"`
	Coupling          Coupling          `json:"coupling"`
	Novelty           Novelty           `json:"novelty"`
	Verification      Verification      `json:"verification"`
	ImpactScope       ImpactScope       `json:"impact_scope"`
	ArchitecturalRole ArchitecturalRole `json:"architectural_role"`
	Citations         []Citation        `json:"citations"`
}
type Citation struct {
	ID string `json:"id"`
}
type EvidenceSet struct{ Resolved map[string]struct{} }
type AssuranceRaise struct {
	Value     Assurance  `json:"value"`
	Citations []Citation `json:"citations"`
}
type Profile struct {
	Capability    Capability    `json:"capability"`
	Deliberation  Deliberation  `json:"deliberation"`
	Context       Context       `json:"context"`
	Assurance     Assurance     `json:"assurance"`
	Applied       []AppliedRule `json:"applied"`
	PolicyVersion string        `json:"policy_version"`
	PolicyDigest  string        `json:"policy_digest"`
}
type AppliedRule struct {
	ID   string `json:"id"`
	Kind string `json:"kind"`
}
type Floor struct {
	ID      string         `json:"id"`
	Axis    string         `json:"axis"`
	Value   string         `json:"value"`
	Profile ProfileMinimum `json:"profile"`
}
type Promotion struct {
	ID      string          `json:"id"`
	When    AssessmentMatch `json:"when"`
	Profile ProfileMinimum  `json:"profile"`
}
type ProfileMinimum struct {
	Capability   Capability   `json:"capability"`
	Deliberation Deliberation `json:"deliberation"`
	Context      Context      `json:"context"`
	Assurance    Assurance    `json:"assurance"`
}
type AssessmentMatch struct {
	Coupling          Coupling          `json:"coupling,omitempty"`
	Novelty           Novelty           `json:"novelty,omitempty"`
	ImpactScope       ImpactScope       `json:"impact_scope,omitempty"`
	ArchitecturalRole ArchitecturalRole `json:"architectural_role,omitempty"`
}
type DerivationPolicy struct {
	Schema     string      `json:"schema"`
	Version    string      `json:"version"`
	Floors     []Floor     `json:"floors"`
	Promotions []Promotion `json:"promotions"`
}
type Diagnostic struct {
	Field   string
	Code    string
	Message string
}

func (d Diagnostic) Error() string { return fmt.Sprintf("%s (%s): %s", d.Field, d.Code, d.Message) }

func valid[T ~string](v T, values []T) bool {
	for _, x := range values {
		if v == x {
			return true
		}
	}
	return false
}
func (a Assessment) validate() error {
	checks := []struct {
		f  string
		v  string
		ok bool
	}{{"reasoning", string(a.Reasoning), valid(a.Reasoning, []Reasoning{ReasoningRoutine, ReasoningComplex, ReasoningDifficult})}, {"coupling", string(a.Coupling), valid(a.Coupling, []Coupling{CouplingContained, CouplingComposed, CouplingCrossBoundary})}, {"novelty", string(a.Novelty), valid(a.Novelty, []Novelty{NoveltyKnown, NoveltyAdapt, NoveltyFirstOfKind})}, {"verification", string(a.Verification), valid(a.Verification, []Verification{VerificationLocal, VerificationComposed, VerificationCrossBoundary})}, {"impact_scope", string(a.ImpactScope), valid(a.ImpactScope, []ImpactScope{ImpactContained, ImpactMultiSurface, ImpactSystemWide})}, {"architectural_role", string(a.ArchitecturalRole), valid(a.ArchitecturalRole, []ArchitecturalRole{RoleFeatureLocal, RoleSharedSubstrate, RoleSystemCore})}}
	for _, c := range checks {
		if !c.ok {
			return Diagnostic{c.f, "unknown_value", fmt.Sprintf("unknown value %q", c.v)}
		}
	}
	if len(a.Citations) == 0 {
		return Diagnostic{"citations", "missing_citation", "at least one assessment citation is required"}
	}
	return nil
}

func validateMinimum(m ProfileMinimum, field string) error {
	if m.Capability != "" && !valid(m.Capability, []Capability{CapabilityEconomy, CapabilityStandard, CapabilityStrong, CapabilityFrontier}) {
		return Diagnostic{field + ".capability", "unknown_value", "unknown capability"}
	}
	if m.Deliberation != "" && !valid(m.Deliberation, []Deliberation{DeliberationLight, DeliberationStandard, DeliberationDeep, DeliberationMaximum}) {
		return Diagnostic{field + ".deliberation", "unknown_value", "unknown deliberation"}
	}
	if m.Context != "" && !valid(m.Context, []Context{ContextStandard, ContextLarge, ContextExtended}) {
		return Diagnostic{field + ".context", "unknown_value", "unknown context"}
	}
	if m.Assurance != "" && !valid(m.Assurance, []Assurance{AssuranceOrdinary, AssuranceElevated, AssuranceCritical}) {
		return Diagnostic{field + ".assurance", "unknown_value", "unknown assurance"}
	}
	return nil
}
func validateCitations(e EvidenceSet, cs []Citation, field string) error {
	if len(cs) == 0 {
		return Diagnostic{field, "missing_citation", "at least one citation is required"}
	}
	seen := map[string]bool{}
	for _, c := range cs {
		if c.ID == "" || seen[c.ID] || e.Resolved == nil {
			return Diagnostic{field, "unresolved_citation", "citation is missing, duplicated, or unresolved"}
		}
		seen[c.ID] = true
		if _, ok := e.Resolved[c.ID]; !ok {
			return Diagnostic{field, "unresolved_citation", c.ID}
		}
	}
	return nil
}
func (p DerivationPolicy) validate() error {
	if p.Schema != "execution-demand-policy/v1" || p.Version == "" {
		return Diagnostic{"policy", "unknown_policy", "unsupported or incomplete policy"}
	}
	ids := map[string]string{}
	for i, f := range p.Floors {
		if f.ID == "" || f.Axis == "" || f.Value == "" || (f.Profile == ProfileMinimum{}) {
			return Diagnostic{fmt.Sprintf("floors[%d]", i), "unknown_policy", "incomplete floor"}
		}
		if err := validateMinimum(f.Profile, fmt.Sprintf("floors[%d].profile", i)); err != nil {
			return err
		}
		if previous, exists := ids[f.ID]; exists {
			return Diagnostic{fmt.Sprintf("floors[%d].id", i), "duplicate_policy_id", fmt.Sprintf("duplicates %s entry %s", previous, f.ID)}
		}
		ids[f.ID] = fmt.Sprintf("floors[%d]", i)
		if !valid(f.Axis, []string{"reasoning", "coupling", "novelty", "verification", "impact_scope", "architectural_role"}) {
			return Diagnostic{fmt.Sprintf("floors[%d].axis", i), "unknown_policy", "unknown assessment axis"}
		}
	}
	for i, r := range p.Promotions {
		if r.ID == "" || (r.Profile == ProfileMinimum{}) {
			return Diagnostic{fmt.Sprintf("promotions[%d]", i), "unknown_policy", "incomplete promotion"}
		}
		if err := validateMinimum(r.Profile, fmt.Sprintf("promotions[%d].profile", i)); err != nil {
			return err
		}
		if previous, exists := ids[r.ID]; exists {
			return Diagnostic{fmt.Sprintf("promotions[%d].id", i), "duplicate_policy_id", fmt.Sprintf("duplicates %s entry %s", previous, r.ID)}
		}
		ids[r.ID] = fmt.Sprintf("promotions[%d]", i)
		if r.When.Coupling != "" && !valid(r.When.Coupling, []Coupling{CouplingContained, CouplingComposed, CouplingCrossBoundary}) {
			return Diagnostic{fmt.Sprintf("promotions[%d].when.coupling", i), "unknown_policy", "unknown coupling"}
		}
		if r.When.Novelty != "" && !valid(r.When.Novelty, []Novelty{NoveltyKnown, NoveltyAdapt, NoveltyFirstOfKind}) {
			return Diagnostic{fmt.Sprintf("promotions[%d].when.novelty", i), "unknown_policy", "unknown novelty"}
		}
		if r.When.ImpactScope != "" && !valid(r.When.ImpactScope, []ImpactScope{ImpactContained, ImpactMultiSurface, ImpactSystemWide}) {
			return Diagnostic{fmt.Sprintf("promotions[%d].when.impact_scope", i), "unknown_policy", "unknown impact scope"}
		}
		if r.When.ArchitecturalRole != "" && !valid(r.When.ArchitecturalRole, []ArchitecturalRole{RoleFeatureLocal, RoleSharedSubstrate, RoleSystemCore}) {
			return Diagnostic{fmt.Sprintf("promotions[%d].when.architectural_role", i), "unknown_policy", "unknown role"}
		}
	}
	return nil
}
func policyDigest(p DerivationPolicy) string {
	p.Floors = append([]Floor(nil), p.Floors...)
	p.Promotions = append([]Promotion(nil), p.Promotions...)
	sort.Slice(p.Floors, func(i, j int) bool { return p.Floors[i].ID < p.Floors[j].ID })
	sort.Slice(p.Promotions, func(i, j int) bool { return p.Promotions[i].ID < p.Promotions[j].ID })
	b, _ := json.Marshal(p)
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

// DefaultPolicy is the portable provider-neutral policy shipped by this package.
var DefaultPolicy = DerivationPolicy{Schema: "execution-demand-policy/v1", Version: "1.0.0", Floors: []Floor{
	{ID: "reasoning-complex", Axis: "reasoning", Value: "complex", Profile: ProfileMinimum{Capability: CapabilityStandard, Deliberation: DeliberationStandard}},
	{ID: "reasoning-difficult", Axis: "reasoning", Value: "difficult", Profile: ProfileMinimum{Capability: CapabilityStrong, Deliberation: DeliberationDeep}},
	{ID: "coupling-composed", Axis: "coupling", Value: "composed", Profile: ProfileMinimum{Capability: CapabilityStandard, Context: ContextLarge}},
	{ID: "coupling-cross-boundary", Axis: "coupling", Value: "cross_boundary", Profile: ProfileMinimum{Capability: CapabilityStrong, Context: ContextExtended}},
	{ID: "novelty-adapt", Axis: "novelty", Value: "adapt", Profile: ProfileMinimum{Capability: CapabilityStandard, Deliberation: DeliberationStandard}},
	{ID: "novelty-first-of-kind", Axis: "novelty", Value: "first_of_kind", Profile: ProfileMinimum{Capability: CapabilityStrong, Deliberation: DeliberationDeep}},
	{ID: "verification-composed", Axis: "verification", Value: "composed", Profile: ProfileMinimum{Context: ContextLarge}},
	{ID: "verification-cross-boundary", Axis: "verification", Value: "cross_boundary", Profile: ProfileMinimum{Capability: CapabilityStrong, Context: ContextExtended}},
	{ID: "impact-multi-surface", Axis: "impact_scope", Value: "multi_surface", Profile: ProfileMinimum{Capability: CapabilityStrong, Assurance: AssuranceElevated}},
	{ID: "impact-system-wide", Axis: "impact_scope", Value: "system_wide", Profile: ProfileMinimum{Capability: CapabilityFrontier, Context: ContextExtended, Assurance: AssuranceCritical}},
	{ID: "role-shared-substrate", Axis: "architectural_role", Value: "shared_substrate", Profile: ProfileMinimum{Capability: CapabilityStrong, Assurance: AssuranceElevated}},
	{ID: "role-system-core", Axis: "architectural_role", Value: "system_core", Profile: ProfileMinimum{Capability: CapabilityFrontier, Deliberation: DeliberationMaximum, Assurance: AssuranceCritical}},
}, Promotions: []Promotion{
	{ID: "compose-adapt", When: AssessmentMatch{Coupling: CouplingComposed, Novelty: NoveltyAdapt}, Profile: ProfileMinimum{Deliberation: DeliberationDeep}},
	{ID: "critical-frontier", When: AssessmentMatch{ImpactScope: ImpactSystemWide}, Profile: ProfileMinimum{Capability: CapabilityFrontier}},
}}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
func capAt(v Capability, x Capability) Capability {
	order := []Capability{CapabilityEconomy, CapabilityStandard, CapabilityStrong, CapabilityFrontier}
	i, j := 0, 0
	for n, z := range order {
		if v == z {
			i = n
		}
		if x == z {
			j = n
		}
	}
	return order[max(i, j)]
}
func delAt(v, x Deliberation) Deliberation {
	o := []Deliberation{DeliberationLight, DeliberationStandard, DeliberationDeep, DeliberationMaximum}
	i, j := 0, 0
	for n, z := range o {
		if v == z {
			i = n
		}
		if x == z {
			j = n
		}
	}
	return o[max(i, j)]
}
func ctxAt(v, x Context) Context {
	o := []Context{ContextStandard, ContextLarge, ContextExtended}
	i, j := 0, 0
	for n, z := range o {
		if v == z {
			i = n
		}
		if x == z {
			j = n
		}
	}
	return o[max(i, j)]
}
func assAt(v, x Assurance) Assurance {
	o := []Assurance{AssuranceOrdinary, AssuranceElevated, AssuranceCritical}
	i, j := 0, 0
	for n, z := range o {
		if v == z {
			i = n
		}
		if x == z {
			j = n
		}
	}
	return o[max(i, j)]
}
func assuranceRank(v Assurance) int {
	switch v {
	case AssuranceElevated:
		return 1
	case AssuranceCritical:
		return 2
	default:
		return 0
	}
}
func (m AssessmentMatch) matches(a Assessment) bool {
	return (m.Coupling == "" || m.Coupling == a.Coupling) && (m.Novelty == "" || m.Novelty == a.Novelty) && (m.ImpactScope == "" || m.ImpactScope == a.ImpactScope) && (m.ArchitecturalRole == "" || m.ArchitecturalRole == a.ArchitecturalRole)
}

func Evaluate(a Assessment, e EvidenceSet, r AssuranceRaise, p DerivationPolicy) (Profile, error) {
	if err := a.validate(); err != nil {
		return Profile{}, err
	}
	if err := validateCitations(e, a.Citations, "citations"); err != nil {
		return Profile{}, err
	}
	if err := p.validate(); err != nil {
		return Profile{}, err
	}
	if r.Value != "" && !valid(r.Value, []Assurance{AssuranceOrdinary, AssuranceElevated, AssuranceCritical}) {
		return Profile{}, Diagnostic{"assurance_raise.value", "unknown_value", "unknown assurance"}
	}
	if r.Value != "" {
		if err := validateCitations(e, r.Citations, "assurance_raise.citations"); err != nil {
			return Profile{}, err
		}
	}
	o := Profile{Capability: CapabilityEconomy, Deliberation: DeliberationLight, Context: ContextStandard, Assurance: AssuranceOrdinary, PolicyVersion: p.Version, PolicyDigest: policyDigest(p)}
	rules := []AppliedRule{}
	for _, f := range p.Floors {
		match := (f.Axis == "reasoning" && f.Value == string(a.Reasoning)) || (f.Axis == "coupling" && f.Value == string(a.Coupling)) || (f.Axis == "novelty" && f.Value == string(a.Novelty)) || (f.Axis == "verification" && f.Value == string(a.Verification)) || (f.Axis == "impact_scope" && f.Value == string(a.ImpactScope)) || (f.Axis == "architectural_role" && f.Value == string(a.ArchitecturalRole))
		if match {
			o.Capability = capAt(o.Capability, f.Profile.Capability)
			o.Deliberation = delAt(o.Deliberation, f.Profile.Deliberation)
			o.Context = ctxAt(o.Context, f.Profile.Context)
			o.Assurance = assAt(o.Assurance, f.Profile.Assurance)
			rules = append(rules, AppliedRule{f.ID, "floor"})
		}
	}
	for _, x := range p.Promotions {
		if x.When.matches(a) {
			o.Capability = capAt(o.Capability, x.Profile.Capability)
			o.Deliberation = delAt(o.Deliberation, x.Profile.Deliberation)
			o.Context = ctxAt(o.Context, x.Profile.Context)
			o.Assurance = assAt(o.Assurance, x.Profile.Assurance)
			rules = append(rules, AppliedRule{x.ID, "promotion"})
		}
	}
	if r.Value != "" {
		before := o.Assurance
		if assuranceRank(r.Value) < assuranceRank(before) {
			return Profile{}, Diagnostic{"assurance_raise.value", "reduction", "raise cannot lower derived assurance"}
		}
		o.Assurance = assAt(o.Assurance, r.Value)
		rules = append(rules, AppliedRule{"assurance-raise", "raise"})
	}
	sort.Slice(rules, func(i, j int) bool { return rules[i].ID < rules[j].ID })
	o.Applied = rules
	return o, nil
}
func VerifyProfile(a Assessment, e EvidenceSet, r AssuranceRaise, p DerivationPolicy, want Profile) error {
	got, err := Evaluate(a, e, r, p)
	if err != nil {
		return err
	}
	gb, _ := json.Marshal(got)
	wb, _ := json.Marshal(want)
	if string(gb) != string(wb) {
		return Diagnostic{"profile", "mismatch", "supplied profile does not match derived profile"}
	}
	return nil
}
func CanonicalJSON(v any) ([]byte, error) { return json.Marshal(v) }
func NormalizeCitations(c []Citation) []Citation {
	out := append([]Citation(nil), c...)
	sort.Slice(out, func(i, j int) bool { return strings.Compare(out[i].ID, out[j].ID) < 0 })
	return out
}
