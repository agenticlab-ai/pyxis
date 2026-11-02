package featuredrive

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"
)

const (
	ScenarioPlanSchema       = "scenario-plan/v1"
	CandidateEvidenceSchema  = "candidate-evidence/v1"
	VerificationRepairSchema = "verification-repair/v1"
)

var (
	ErrEvidenceMalformed = errors.New("feature drive: malformed candidate evidence")
	ErrEvidenceConflict  = errors.New("feature drive: conflicting candidate evidence")
	ErrManualRequired    = errors.New("feature drive: manual human attestation required")
	ErrRepairEvidence    = errors.New("feature drive: verification repair evidence invalid")
)

type ScenarioRoute string

const (
	RouteAutomatic   ScenarioRoute = "automatic"
	RouteSpecialized ScenarioRoute = "specialized"
	RouteManual      ScenarioRoute = "manual-human"
)

type ScenarioOutcome string

const (
	OutcomePassed         ScenarioOutcome = "passed"
	OutcomeFailed         ScenarioOutcome = "failed"
	OutcomeManualAccepted ScenarioOutcome = "manual-accepted"
	OutcomeManualRejected ScenarioOutcome = "manual-rejected"
	OutcomePending        ScenarioOutcome = "pending"
	OutcomeSkipped        ScenarioOutcome = "skipped"
	OutcomeUnavailable    ScenarioOutcome = "unavailable"
	OutcomeInconclusive   ScenarioOutcome = "inconclusive"
)

var validOutcomes = map[ScenarioOutcome]bool{
	OutcomePassed: true, OutcomeFailed: true,
	OutcomeManualAccepted: true, OutcomeManualRejected: true,
	OutcomePending: true, OutcomeSkipped: true,
	OutcomeUnavailable: true, OutcomeInconclusive: true,
}

type ScenarioSpec struct {
	ID                string        `json:"id"`
	Route             ScenarioRoute `json:"route"`
	SpecializedRunner string        `json:"specialized_runner,omitempty"`
}

type ScenarioPlan struct {
	Schema                 string            `json:"schema"`
	FeatureID              string            `json:"feature_id"`
	Candidate              CandidateIdentity `json:"candidate"`
	ScenarioSetFingerprint string            `json:"scenario_set_fingerprint"`
	Scenarios              []ScenarioSpec    `json:"scenarios"`
}

func (p ScenarioPlan) Validate() error {
	if p.Schema != ScenarioPlanSchema || p.FeatureID == "" || !validCandidateIdentity(p.Candidate) || p.Candidate.FeatureID != p.FeatureID || p.ScenarioSetFingerprint == "" || len(p.Scenarios) == 0 {
		return ErrEvidenceMalformed
	}
	seen := make(map[string]bool, len(p.Scenarios))
	for _, s := range p.Scenarios {
		if s.ID == "" || seen[s.ID] {
			return ErrEvidenceMalformed
		}
		seen[s.ID] = true
		switch s.Route {
		case RouteAutomatic, RouteManual:
			if s.SpecializedRunner != "" {
				return ErrEvidenceMalformed
			}
		case RouteSpecialized:
			if s.SpecializedRunner == "" {
				return ErrEvidenceMalformed
			}
		default:
			return ErrEvidenceMalformed
		}
	}
	return nil
}

type ScenarioExecution struct {
	FeatureID   string
	Candidate   CandidateIdentity
	Scenario    ScenarioSpec
	CandidateID string
}

type ScenarioRunner interface {
	Identity() string
	Run(context.Context, ScenarioExecution) (ScenarioResult, error)
}

type ScenarioResult struct {
	ScenarioID     string          `json:"scenario_id"`
	CandidateID    string          `json:"candidate_id"`
	Route          ScenarioRoute   `json:"route"`
	RunnerIdentity string          `json:"runner_identity,omitempty"`
	ActorIdentity  string          `json:"actor_identity,omitempty"`
	StartedAt      time.Time       `json:"started_at"`
	CompletedAt    time.Time       `json:"completed_at"`
	Outcome        ScenarioOutcome `json:"outcome"`
	EvidenceRef    string          `json:"evidence_ref"`
	ImplicatedRefs []string        `json:"implicated_refs,omitempty"`
}

func (r ScenarioResult) accepted() bool {
	return r.Outcome == OutcomePassed || r.Outcome == OutcomeManualAccepted
}

type ManualAttestation struct {
	ScenarioID    string    `json:"scenario_id"`
	CandidateID   string    `json:"candidate_id"`
	ActorIdentity string    `json:"actor_identity"`
	Human         bool      `json:"human"`
	Accepted      bool      `json:"accepted"`
	EvidenceRef   string    `json:"evidence_ref"`
	CompletedAt   time.Time `json:"completed_at"`
}

// ExecuteScenario selects the declared specialized runner before the owner
// runner. Manual routes never execute through a machine runner.
func ExecuteScenario(ctx context.Context, plan ScenarioPlan, scenarioID string, owner ScenarioRunner, specialized map[string]ScenarioRunner) (ScenarioResult, error) {
	if err := plan.Validate(); err != nil {
		return ScenarioResult{}, err
	}
	spec, ok := scenarioByID(plan, scenarioID)
	if !ok {
		return ScenarioResult{}, ErrEvidenceMalformed
	}
	if spec.Route == RouteManual {
		return ScenarioResult{}, ErrManualRequired
	}
	runner := owner
	if spec.Route == RouteSpecialized {
		runner = specialized[spec.SpecializedRunner]
	}
	if runner == nil || runner.Identity() == "" || (spec.Route == RouteSpecialized && runner.Identity() != spec.SpecializedRunner) {
		return ScenarioResult{}, ErrEvidenceMalformed
	}
	candidateID := plan.Candidate.Digest()
	result, err := runner.Run(ctx, ScenarioExecution{FeatureID: plan.FeatureID, Candidate: plan.Candidate, Scenario: spec, CandidateID: candidateID})
	if err != nil {
		return ScenarioResult{}, err
	}
	if err := validateResult(plan, spec, result); err != nil {
		return ScenarioResult{}, err
	}
	return result, nil
}

func ResultFromManualAttestation(plan ScenarioPlan, att ManualAttestation) (ScenarioResult, error) {
	if err := plan.Validate(); err != nil {
		return ScenarioResult{}, err
	}
	spec, ok := scenarioByID(plan, att.ScenarioID)
	if !ok || spec.Route != RouteManual || !att.Human || att.ActorIdentity == "" || att.CandidateID != plan.Candidate.Digest() || att.EvidenceRef == "" || att.CompletedAt.IsZero() {
		return ScenarioResult{}, ErrEvidenceMalformed
	}
	outcome := OutcomeManualRejected
	if att.Accepted {
		outcome = OutcomeManualAccepted
	}
	result := ScenarioResult{ScenarioID: att.ScenarioID, CandidateID: att.CandidateID, Route: RouteManual, ActorIdentity: att.ActorIdentity, CompletedAt: att.CompletedAt, Outcome: outcome, EvidenceRef: att.EvidenceRef}
	if err := validateResult(plan, spec, result); err != nil {
		return ScenarioResult{}, err
	}
	return result, nil
}

type CandidateEvidence struct {
	Schema                 string            `json:"schema"`
	FeatureID              string            `json:"feature_id"`
	Candidate              CandidateIdentity `json:"candidate"`
	ScenarioSetFingerprint string            `json:"scenario_set_fingerprint"`
	Results                []ScenarioResult  `json:"results"`
}

func NewCandidateEvidence(plan ScenarioPlan, results []ScenarioResult) (CandidateEvidence, error) {
	if err := plan.Validate(); err != nil {
		return CandidateEvidence{}, err
	}
	e := CandidateEvidence{Schema: CandidateEvidenceSchema, FeatureID: plan.FeatureID, Candidate: plan.Candidate, ScenarioSetFingerprint: plan.ScenarioSetFingerprint, Results: append([]ScenarioResult(nil), results...)}
	if err := e.Validate(plan); err != nil {
		return CandidateEvidence{}, err
	}
	return e, nil
}

func (e CandidateEvidence) Validate(plan ScenarioPlan) error {
	if err := plan.Validate(); err != nil || e.Schema != CandidateEvidenceSchema || e.FeatureID != plan.FeatureID || e.Candidate.Digest() != plan.Candidate.Digest() || e.ScenarioSetFingerprint != plan.ScenarioSetFingerprint {
		return ErrEvidenceMalformed
	}
	for _, r := range e.Results {
		spec, ok := scenarioByID(plan, r.ScenarioID)
		if !ok {
			return ErrEvidenceMalformed
		}
		if err := validateResult(plan, spec, r); err != nil {
			return err
		}
	}
	return nil
}

type EvidenceBlock struct {
	ScenarioID string
	Outcome    ScenarioOutcome
	Reason     string
}

type EvidenceVerdict struct {
	FeatureID   string
	CandidateID string
	ScenarioSet string
	Accepted    bool
	Blocks      []EvidenceBlock
}

// EvaluateCandidateEvidence is the sole eligibility predicate. Every required
// scenario must have exactly one accepted result on the current candidate;
// duplicate terminal observations fail closed even when one is accepted.
func EvaluateCandidateEvidence(plan ScenarioPlan, evidence CandidateEvidence) (EvidenceVerdict, error) {
	if err := evidence.Validate(plan); err != nil {
		return EvidenceVerdict{}, err
	}
	byScenario := make(map[string][]ScenarioResult, len(plan.Scenarios))
	for _, r := range evidence.Results {
		byScenario[r.ScenarioID] = append(byScenario[r.ScenarioID], r)
	}
	verdict := EvidenceVerdict{FeatureID: plan.FeatureID, CandidateID: plan.Candidate.Digest(), ScenarioSet: plan.ScenarioSetFingerprint, Accepted: true}
	for _, spec := range plan.Scenarios {
		results := byScenario[spec.ID]
		switch len(results) {
		case 0:
			verdict.Accepted = false
			verdict.Blocks = append(verdict.Blocks, EvidenceBlock{ScenarioID: spec.ID, Outcome: OutcomePending, Reason: "missing result"})
		case 1:
			if !results[0].accepted() {
				verdict.Accepted = false
				verdict.Blocks = append(verdict.Blocks, EvidenceBlock{ScenarioID: spec.ID, Outcome: results[0].Outcome, Reason: "outcome is not accepted"})
			}
		default:
			return EvidenceVerdict{}, fmt.Errorf("%w: scenario %s has %d results", ErrEvidenceConflict, spec.ID, len(results))
		}
	}
	return verdict, nil
}

type VerificationRepair struct {
	Schema         string   `json:"schema"`
	ID             string   `json:"id"`
	FeatureID      string   `json:"feature_id"`
	CandidateID    string   `json:"candidate_id"`
	ScenarioID     string   `json:"scenario_id"`
	Outcome        string   `json:"outcome"`
	EvidenceRef    string   `json:"evidence_ref"`
	ImplicatedRefs []string `json:"implicated_refs,omitempty"`
}

type VerificationRepairCreator interface {
	CreateVerificationRepair(context.Context, VerificationRepair) (string, error)
}

func CreateVerificationRepair(ctx context.Context, creator VerificationRepairCreator, plan ScenarioPlan, result ScenarioResult) (string, error) {
	if creator == nil || result.accepted() || result.Outcome == OutcomePending {
		return "", ErrRepairEvidence
	}
	spec, ok := scenarioByID(plan, result.ScenarioID)
	if !ok || validateResult(plan, spec, result) != nil {
		return "", ErrRepairEvidence
	}
	refs := append([]string(nil), result.ImplicatedRefs...)
	sort.Strings(refs)
	r := VerificationRepair{Schema: VerificationRepairSchema, FeatureID: plan.FeatureID, CandidateID: plan.Candidate.Digest(), ScenarioID: result.ScenarioID, Outcome: string(result.Outcome), EvidenceRef: result.EvidenceRef, ImplicatedRefs: refs}
	b, _ := json.Marshal(r)
	h := sha256.Sum256(b)
	r.ID = hex.EncodeToString(h[:])
	return creator.CreateVerificationRepair(ctx, r)
}

func scenarioByID(plan ScenarioPlan, id string) (ScenarioSpec, bool) {
	for _, s := range plan.Scenarios {
		if s.ID == id {
			return s, true
		}
	}
	return ScenarioSpec{}, false
}

func validateResult(plan ScenarioPlan, spec ScenarioSpec, r ScenarioResult) error {
	if r.ScenarioID != spec.ID || r.CandidateID != plan.Candidate.Digest() || r.Route != spec.Route || !validOutcomes[r.Outcome] || r.EvidenceRef == "" || r.CompletedAt.IsZero() {
		return ErrEvidenceMalformed
	}
	switch spec.Route {
	case RouteManual:
		if r.RunnerIdentity != "" || r.ActorIdentity == "" || (r.Outcome != OutcomeManualAccepted && r.Outcome != OutcomeManualRejected) {
			return ErrEvidenceMalformed
		}
	case RouteSpecialized:
		if r.RunnerIdentity != spec.SpecializedRunner || r.Outcome == OutcomeManualAccepted || r.Outcome == OutcomeManualRejected {
			return ErrEvidenceMalformed
		}
	case RouteAutomatic:
		if r.RunnerIdentity == "" || r.Outcome == OutcomeManualAccepted || r.Outcome == OutcomeManualRejected {
			return ErrEvidenceMalformed
		}
	default:
		return ErrEvidenceMalformed
	}
	if !r.StartedAt.IsZero() && r.CompletedAt.Before(r.StartedAt) {
		return ErrEvidenceMalformed
	}
	return nil
}

func validCandidateIdentity(c CandidateIdentity) bool {
	return c.FeatureID != "" && c.FeatureHead != "" && c.MainHead != "" && c.Commit != "" && c.Tree != "" && c.MergePolicyVersion != "" && c.BuilderVersion != ""
}
