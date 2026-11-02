package routingevidence

import (
	"context"
	"errors"
	"fmt"
	"github.com/agenticlab-ai/pyxis/internal/entity"
	"sort"
)

const OperatorDiagnosticSchema = "routing-evidence-operator-diagnostic/v1"

// OperatorDiagnostic is intentionally identity-only. Canonical event payloads
// remain available to the reducer, but finding prose, code-bearing fields,
// signatures, reasons, and native output never enter this surface.
type OperatorDiagnostic struct {
	Schema                 string             `json:"schema"`
	Subject                SubjectID          `json:"subject"`
	TaskID                 string             `json:"task_id,omitempty"`
	ProposalID             string             `json:"proposal_id,omitempty"`
	Harness                entity.Harness     `json:"harness,omitempty"`
	Role                   string             `json:"role,omitempty"`
	PolicyVersion          string             `json:"policy_version,omitempty"`
	SemanticComponents     []string           `json:"semantic_components"`
	AttemptID              string             `json:"attempt_id,omitempty"`
	ReviewProcessID        string             `json:"review_process_id,omitempty"`
	State                  string             `json:"state"`
	CurrentOwnerGeneration int                `json:"current_owner_generation,omitempty"`
	EvidenceRef            string             `json:"evidence_ref,omitempty"`
	ExactGitObject         string             `json:"exact_git_object,omitempty"`
	ProofDigest            string             `json:"proof_digest,omitempty"`
	ProofEntries           int                `json:"proof_entries"`
	Accepting              bool               `json:"accepting"`
	MissingEdge            string             `json:"missing_edge,omitempty"`
	Invalidations          []InvalidationView `json:"invalidations"`
	ProjectionDiff         *ProjectionError   `json:"projection_diff,omitempty"`
	Chain                  []EventView        `json:"chain"`
	Measurements           []MeasurementView  `json:"measurements"`
}

type FreshCloneVerifier struct{ reader Reader }

func NewFreshCloneVerifier(reader Reader) (FreshCloneVerifier, error) {
	if reader == nil {
		return FreshCloneVerifier{}, fmt.Errorf("fresh-clone verifier: reader is required")
	}
	return FreshCloneVerifier{reader: reader}, nil
}

// Verify reconstructs from the reader on every call and then checks every
// supplied disposable medium face against that one exact head.
func (v FreshCloneVerifier) Verify(ctx context.Context, subject SubjectID, projections ...Projection) (OperatorDiagnostic, error) {
	views, err := v.reader.ReadViews(ctx, subject)
	if err != nil {
		report := OperatorDiagnostic{Schema: OperatorDiagnosticSchema, Subject: subject, SemanticComponents: []string{}, Invalidations: []InvalidationView{}, Chain: []EventView{}, Measurements: []MeasurementView{}}
		var reducerErr *ReducerError
		if errors.As(err, &reducerErr) {
			report.State = string(reducerErr.Code)
			report.MissingEdge = reducerErr.Edge
		}
		return report, err
	}
	report := diagnosticFor(views)
	for _, projection := range projections {
		if err := VerifyProjection(projection, views); err != nil {
			var diff *ProjectionError
			if errors.As(err, &diff) {
				copy := *diff
				report.ProjectionDiff = &copy
			}
			return report, err
		}
	}
	return report, nil
}

func EncodeOperatorDiagnostic(report OperatorDiagnostic) ([]byte, error) {
	if report.Schema != OperatorDiagnosticSchema || report.Subject.ID == "" || report.SemanticComponents == nil || report.Invalidations == nil || report.Chain == nil || report.Measurements == nil {
		return nil, fmt.Errorf("routing evidence operator diagnostic: incomplete report")
	}
	return canonicalJSON(report)
}

func diagnosticFor(views SubjectViews) OperatorDiagnostic {
	report := OperatorDiagnostic{Schema: OperatorDiagnosticSchema, Subject: views.Subject.Subject,
		EvidenceRef: views.EvidenceHead.EvidenceRef, ExactGitObject: views.EvidenceHead.Commit,
		ProofEntries: len(views.Proof.Entries), Accepting: acceptingProjection(views.Acceptance),
		Invalidations:      append([]InvalidationView{}, views.Subject.Invalidations...),
		SemanticComponents: []string{}, Chain: append([]EventView{}, views.Events...),
		Measurements: append([]MeasurementView{}, views.Measurements...)}
	report.ProofDigest, _ = digestValue(proofDigestDomain, views.Proof)
	report.State, report.CurrentOwnerGeneration = subjectState(views.Subject)
	for _, event := range views.Events {
		if !eventMatchesSubject(event, views.Subject) {
			continue
		}
		setFirst(&report.TaskID, event.TaskID)
		setFirst(&report.ProposalID, event.ProposalID)
		if report.Harness == "" {
			report.Harness = event.Harness
		}
		setFirst(&report.Role, event.Role)
		setFirst(&report.PolicyVersion, event.PolicyVersion)
		setFirst(&report.AttemptID, event.AttemptID)
		setFirst(&report.ReviewProcessID, event.ProcessID)
		report.SemanticComponents = append(report.SemanticComponents, event.SemanticComponents...)
	}
	report.SemanticComponents = uniqueStrings(report.SemanticComponents)
	if report.CurrentOwnerGeneration == 0 && report.AttemptID != "" {
		for _, attempt := range views.State.Attempts {
			if attempt.AttemptID == report.AttemptID {
				report.CurrentOwnerGeneration = attempt.OwnerGeneration
				break
			}
		}
	}
	return report
}

func subjectState(view SubjectView) (string, int) {
	switch {
	case view.Demand != nil:
		if len(view.Demand.InvalidationIDs) > 0 {
			return string(StateInvalidated), 0
		}
		return "demand_frozen", 0
	case view.Attempt != nil:
		return string(view.Attempt.State), view.Attempt.OwnerGeneration
	case view.Process != nil:
		return string(view.Process.State), 0
	case view.Invalidation != nil:
		return string(view.Invalidation.State), 0
	case view.Activation != nil:
		if len(view.Activation.InvalidationIDs) > 0 {
			return string(StateInvalidated), 0
		}
		if view.Activation.ActivationEligible {
			return "activation_eligible", 0
		}
		return "activation_non_accepting", 0
	default:
		return "unknown", 0
	}
}

func eventMatchesSubject(event EventView, view SubjectView) bool {
	switch {
	case view.Demand != nil:
		return event.DemandID == view.Demand.DemandID
	case view.Attempt != nil:
		return event.AttemptID == view.Attempt.AttemptID || event.DemandID == view.Attempt.DemandID
	case view.Process != nil:
		if event.ProcessID == view.Process.ReviewProcessID {
			return true
		}
		for _, pass := range view.Process.Passes {
			if event.AttemptID == pass.AttemptID {
				return true
			}
		}
		return false
	case view.Invalidation != nil:
		return event.EventID == view.Invalidation.EventID
	case view.Activation != nil:
		return event.EventID == view.Activation.EventID
	default:
		return false
	}
}

func setFirst(target *string, value string) {
	if *target == "" && value != "" {
		*target = value
	}
}

func uniqueStrings(values []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		if value != "" && !seen[value] {
			seen[value] = true
			out = append(out, value)
		}
	}
	sort.Strings(out)
	return out
}
