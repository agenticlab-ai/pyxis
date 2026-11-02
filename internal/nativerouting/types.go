// Package nativerouting owns the harness-native catalog and qualification
// boundary. It deliberately depends on the vendor-neutral execution profile,
// but never on a runtime adapter implementation.
package nativerouting

import (
	"fmt"
	"github.com/agenticlab-ai/pyxis/internal/entity"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/executiondemand"
)

const (
	CatalogSchema       = "native-routing-catalog/v1"
	QualificationSchema = "native-profile-qualification/v1"
)

type Role string
type Component string
type QualificationMethod string
type EvaluatorVersion string
type EvidenceOrigin string
type AuthorizationKind string
type ControlType string

const (
	RoleImplementation Role = "implementation"
	RoleReview         Role = "review"

	ComponentCapability   Component = "capability"
	ComponentDeliberation Component = "deliberation"
	ComponentContext      Component = "context"
	ComponentAssurance    Component = "assurance"

	MethodDeterministicConformance QualificationMethod = "deterministic_conformance"
	MethodDecodePinnedEval         QualificationMethod = "decode_pinned_eval"
	MethodOperatorInspection       QualificationMethod = "operator_inspection"

	EvaluatorDeterministicV1 EvaluatorVersion = "deterministic-conformance/v1"
	EvaluatorDecodePinnedV1  EvaluatorVersion = "decode-pinned-eval/v1"
	EvaluatorInspectionV1    EvaluatorVersion = "operator-inspection/v1"

	EvidenceIndependentEvaluator EvidenceOrigin = "independent_evaluator"
	EvidenceOperatorRepository   EvidenceOrigin = "operator_repository"
	EvidenceWorker               EvidenceOrigin = "worker"
	EvidenceProvider             EvidenceOrigin = "provider"

	AuthorizationRepositoryOperator AuthorizationKind = "repository_operator"
	AuthorizationContributor        AuthorizationKind = "contributor"
	AuthorizationWorker             AuthorizationKind = "worker"

	ControlString  ControlType = "string"
	ControlInteger ControlType = "integer"
	ControlBoolean ControlType = "boolean"
)

// NativeControl is a closed tagged union. Exactly the value named by Type may
// be populated; an adapter decides which names and values it supports.
type NativeControl struct {
	Name         string      `json:"name"`
	Type         ControlType `json:"type"`
	StringValue  string      `json:"string_value,omitempty"`
	IntegerValue *int64      `json:"integer_value,omitempty"`
	BooleanValue *bool       `json:"boolean_value,omitempty"`
}

type Provides struct {
	Capability   executiondemand.Capability   `json:"capability"`
	Deliberation executiondemand.Deliberation `json:"deliberation"`
	Context      executiondemand.Context      `json:"context"`
	Assurance    executiondemand.Assurance    `json:"assurance"`
}

type CatalogEntry struct {
	NativeProfileID     string          `json:"native_profile_id"`
	NativeModel         string          `json:"native_model"`
	NativeEffort        string          `json:"native_effort"`
	Controls            []NativeControl `json:"controls"`
	Provides            Provides        `json:"provides"`
	CostRank            int             `json:"cost_rank"`
	QualificationID     string          `json:"qualification_id"`
	QualificationDigest string          `json:"qualification_digest"`
	AdapterGeneration   string          `json:"adapter_generation"`
	Strongest           bool            `json:"strongest"`
}

type Catalog struct {
	SchemaVersion     string         `json:"schema_version"`
	CatalogID         string         `json:"catalog_id"`
	Harness           entity.Harness `json:"harness"`
	Role              Role           `json:"role"`
	AdapterGeneration string         `json:"adapter_generation"`
	Entries           []CatalogEntry `json:"entries"`
}

type QualificationClaim struct {
	Component            Component           `json:"component"`
	AssertedLevel        string              `json:"asserted_level"`
	Method               QualificationMethod `json:"method"`
	FixtureDigest        string              `json:"fixture_digest"`
	ResultDigest         string              `json:"result_digest"`
	Threshold            string              `json:"threshold"`
	Result               string              `json:"result"`
	Satisfied            bool                `json:"satisfied"`
	EvidenceReference    string              `json:"evidence_reference"`
	EvaluatorVersion     EvaluatorVersion    `json:"evaluator_version"`
	ResponsibleEvaluator string              `json:"responsible_evaluator"`
	EvidenceOrigin       EvidenceOrigin      `json:"evidence_origin"`
	RubricResult         string              `json:"rubric_result,omitempty"`
}

type Invalidation struct {
	EventID string    `json:"event_id"`
	At      time.Time `json:"at"`
	Reason  string    `json:"reason"`
}

type Validity struct {
	ValidFrom     time.Time      `json:"valid_from"`
	ValidUntil    time.Time      `json:"valid_until"`
	Invalidations []Invalidation `json:"invalidations"`
}

type AuthorizationProof struct {
	Kind           AuthorizationKind `json:"kind"`
	Repository     string            `json:"repository"`
	OperatorID     string            `json:"operator_id"`
	EvidenceDigest string            `json:"evidence_digest"`
	AuthorizedAt   time.Time         `json:"authorized_at"`
}

type Qualification struct {
	SchemaVersion       string               `json:"schema_version"`
	QualificationID     string               `json:"qualification_id"`
	Harness             entity.Harness       `json:"harness"`
	Role                Role                 `json:"role"`
	NativeProfileID     string               `json:"native_profile_id"`
	NativeModel         string               `json:"native_model"`
	NativeEffort        string               `json:"native_effort"`
	Controls            []NativeControl      `json:"controls"`
	AdapterGeneration   string               `json:"adapter_generation"`
	CatalogSchema       string               `json:"catalog_schema"`
	LaunchCaptureDigest string               `json:"launch_capture_digest"`
	Claims              []QualificationClaim `json:"claims"`
	Validity            Validity             `json:"validity"`
	Authorization       AuthorizationProof   `json:"authorization"`
}

type QualificationSet []Qualification

// NativeCapability is implemented by a harness adapter. Its error must name
// an unsupported native value or control; the core imports no adapter.
type NativeCapability interface {
	ValidateProfile(CatalogEntry) error
}

// NativeProfileDefect lets adapters retain a stable defect code and field at
// the consumer boundary. Ordinary errors remain fail-closed generic defects.
type NativeProfileDefect struct {
	Defect string
	Field  string
	Detail string
}

func (d NativeProfileDefect) Error() string { return d.Detail }

type TrustedClock interface {
	Now() time.Time
}

type AuthorizationPolicy interface {
	Authorize(AuthorizationProof) error
}

type Diagnostic struct {
	Harness         entity.Harness `json:"harness"`
	Role            Role           `json:"role"`
	CatalogID       string         `json:"catalog_id"`
	ProfileID       string         `json:"profile_id"`
	QualificationID string         `json:"qualification_id"`
	Defect          string         `json:"defect"`
	Field           string         `json:"field"`
	Detail          string         `json:"detail"`
}

func (d Diagnostic) Error() string {
	return fmt.Sprintf("native routing %s/%s catalog %s profile %s qualification %s: %s at %s: %s", d.Harness, d.Role, d.CatalogID, d.ProfileID, d.QualificationID, d.Defect, d.Field, d.Detail)
}

// ValidatedCatalog is constructed only when the complete catalog validates.
// Entries are canonicalized; no partial candidate set is ever returned.
type ValidatedCatalog struct {
	Catalog  Catalog
	Identity string
}

func (v ValidatedCatalog) Entries() []CatalogEntry {
	return append([]CatalogEntry(nil), v.Catalog.Entries...)
}
