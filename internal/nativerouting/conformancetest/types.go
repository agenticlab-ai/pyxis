// Package conformancetest derives and checks the complete live native-routing
// domain. It contains no adapter roster: harnesses come only from the runtime
// registry and semantic profiles come only from the demand derivation policy.
package conformancetest

import (
	"encoding/json"
	"github.com/agenticlab-ai/pyxis/internal/entity"

	"github.com/agenticlab-ai/pyxis/internal/executiondemand"
	"github.com/agenticlab-ai/pyxis/internal/executor/port"
	"github.com/agenticlab-ai/pyxis/internal/nativerouting"
)

const (
	DomainSchema              = "native-routing-conformance-domain/v1"
	MatrixSchema              = "native-routing-conformance-matrix/v1"
	CellResultSchema          = "native-routing-conformance-cell-result/v1"
	QualificationReportSchema = "native-routing-qualification-report/v1"
	ConformanceEvent          = "routing_conformance_checked"
)

type Expected string
type Result string
type Roles = []nativerouting.Role

const (
	ExpectedResolved    Expected = "resolved"
	ExpectedUnsupported Expected = "unsupported"
	ResultPass          Result   = "pass"
	ResultFail          Result   = "fail"
)

// RuntimeRegistry is the read-only runtime composition used by dispatch.
type RuntimeRegistry interface {
	Harnesses() []entity.Harness
	Resolve(entity.Harness) (port.AgentRuntime, error)
}

type Domain struct {
	SchemaVersion  string                           `json:"schema_version"`
	PolicyVersion  string                           `json:"policy_version"`
	PolicyDigest   string                           `json:"policy_digest"`
	DomainIdentity string                           `json:"domain_identity"`
	Harnesses      []string                         `json:"harnesses"`
	Roles          []nativerouting.Role             `json:"roles"`
	Profiles       []executiondemand.ProfileMinimum `json:"profiles"`
	Cells          []Cell                           `json:"cells"`
}

type Cell struct {
	CellID            string                         `json:"cell_id"`
	Harness           entity.Harness                 `json:"harness"`
	Role              nativerouting.Role             `json:"role"`
	SemanticProfileID string                         `json:"semantic_profile_id"`
	RequestedProfile  executiondemand.ProfileMinimum `json:"requested_profile"`
}

type CellResult struct {
	SchemaVersion       string                         `json:"schema_version"`
	CellID              string                         `json:"cell_id"`
	Harness             entity.Harness                 `json:"harness"`
	Role                nativerouting.Role             `json:"role"`
	SemanticProfileID   string                         `json:"semantic_profile_id"`
	RequestedProfile    executiondemand.ProfileMinimum `json:"requested_profile"`
	Expected            Expected                       `json:"expected"`
	Result              Result                         `json:"result"`
	ResolutionClass     nativerouting.ResolutionClass  `json:"resolution_class"`
	CatalogID           string                         `json:"catalog_id"`
	PolicyIdentity      string                         `json:"policy_identity"`
	NativeProfileID     string                         `json:"native_profile_id,omitempty"`
	QualificationID     string                         `json:"qualification_id,omitempty"`
	QualificationDigest string                         `json:"qualification_digest,omitempty"`
	AdapterGeneration   string                         `json:"adapter_generation"`
	UnmetComponents     []nativerouting.UnmetComponent `json:"unmet_components"`
	EvidenceReference   string                         `json:"evidence_ref"`
}

// RoutingConformanceChecked is the closed EVT-4 payload emitted from a result.
type RoutingConformanceChecked struct {
	Harness           entity.Harness     `json:"harness"`
	Role              nativerouting.Role `json:"role"`
	SemanticProfileID string             `json:"semantic_profile_id"`
	Expected          Expected           `json:"expected"`
	Result            Result             `json:"result"`
	EvidenceReference string             `json:"evidence_ref"`
}

func (r CellResult) Event() RoutingConformanceChecked {
	return RoutingConformanceChecked{Harness: r.Harness, Role: r.Role, SemanticProfileID: r.SemanticProfileID, Expected: r.Expected, Result: r.Result, EvidenceReference: r.EvidenceReference}
}

type Matrix struct {
	SchemaVersion  string       `json:"schema_version"`
	DomainIdentity string       `json:"domain_identity"`
	MatrixIdentity string       `json:"matrix_identity"`
	Results        []CellResult `json:"results"`
}

type QualificationReport struct {
	SchemaVersion  string                        `json:"schema_version"`
	DomainIdentity string                        `json:"domain_identity"`
	MatrixIdentity string                        `json:"matrix_identity"`
	Rows           []QualificationRow            `json:"rows"`
	Qualifications []QualificationEvidenceRecord `json:"qualifications"`
	Signoff        SignoffInput                  `json:"operator_signoff"`
}

type QualificationRow struct {
	CellID              string             `json:"cell_id"`
	Harness             entity.Harness     `json:"harness"`
	Role                nativerouting.Role `json:"role"`
	SemanticProfileID   string             `json:"semantic_profile_id"`
	NativeProfileID     string             `json:"native_profile_id"`
	QualificationID     string             `json:"qualification_id"`
	QualificationDigest string             `json:"qualification_digest"`
	AdapterGeneration   string             `json:"adapter_generation"`
}

// QualificationEvidenceRecord is normalized by qualification identity. Rows
// join to it by ID and digest, avoiding copies of identical human evidence
// while keeping every resolved cell auditable.
type QualificationEvidenceRecord struct {
	Harness             entity.Harness          `json:"harness"`
	Role                nativerouting.Role      `json:"role"`
	NativeProfileID     string                  `json:"native_profile_id"`
	QualificationID     string                  `json:"qualification_id"`
	QualificationDigest string                  `json:"qualification_digest"`
	AdapterGeneration   string                  `json:"adapter_generation"`
	Validity            nativerouting.Validity  `json:"validity"`
	Claims              []QualificationEvidence `json:"claims"`
}

type QualificationEvidence struct {
	Component            nativerouting.Component           `json:"component"`
	AssertedLevel        string                            `json:"asserted_level"`
	Method               nativerouting.QualificationMethod `json:"method"`
	FixtureDigest        string                            `json:"fixture_digest"`
	ResultDigest         string                            `json:"result_digest"`
	Threshold            string                            `json:"threshold"`
	Result               string                            `json:"result"`
	EvidenceReference    string                            `json:"evidence_reference"`
	EvaluatorVersion     nativerouting.EvaluatorVersion    `json:"evaluator_version"`
	ResponsibleEvaluator string                            `json:"responsible_evaluator"`
}

type SignoffInput struct {
	Status            string `json:"status"`
	RequiredEvaluator string `json:"required_evaluator"`
	Rubric            string `json:"rubric"`
	MatrixIdentity    string `json:"matrix_identity"`
}

func (d Domain) JSON() ([]byte, error)              { return json.Marshal(d) }
func (m Matrix) JSON() ([]byte, error)              { return json.Marshal(m) }
func (r QualificationReport) JSON() ([]byte, error) { return json.Marshal(r) }
