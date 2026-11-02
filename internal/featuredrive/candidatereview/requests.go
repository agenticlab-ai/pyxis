package candidatereview

import (
	"context"
	"errors"
	"reflect"

	"github.com/agenticlab-ai/pyxis/internal/codehost"
)

const PacketSchema = "local-candidate-review-packet/v1"

type Packet struct {
	SchemaVersion string                  `json:"schema_version"`
	Binding       codehost.ReviewBinding  `json:"binding"`
	Snapshot      Snapshot                `json:"snapshot"`
	Evidence      codehost.ReviewArtifact `json:"evidence"`
	Request       codehost.ReviewArtifact `json:"request"`
	Trust         codehost.Trust          `json:"trust"`
}

// Requests retains candidate proof before presenting an operator request.
// It provides neither an approval declaration nor a landing operation.
type Requests struct {
	proposals codehost.ScopedProposals
	artifacts codehost.ReviewArtifacts
	reader    Reader
	bound     codehost.ProposalScope
}

func NewRequests(ctx context.Context, proposals codehost.ScopedProposals, artifacts codehost.ReviewArtifacts, reader Reader) (*Requests, error) {
	if proposals == nil || artifacts == nil || reader == nil {
		return nil, ErrIncomplete
	}
	s := &Requests{proposals: proposals, artifacts: artifacts, reader: reader}
	scope, err := s.scope(ctx)
	if err != nil {
		return nil, err
	}
	s.bound = scope
	return s, nil
}

func (s *Requests) scope(ctx context.Context) (codehost.ProposalScope, error) {
	scope, err := s.proposals.Scope(ctx)
	if err != nil {
		return scope, err
	}
	retention, err := s.artifacts.Scope(ctx)
	if err != nil {
		return scope, err
	}
	if scope.SchemaVersion != codehost.ProposalScopeSchema || scope.Kind != "implementation" || scope.Origin == "" || scope.TargetRef != scope.MainlineRef || scope.HeadRef == "" || scope.HeadRef == scope.TargetRef || retention != scope || (s.bound.Origin != "" && s.bound != scope) {
		return scope, ErrIncomplete
	}
	return scope, nil
}

// Prepare reuses a complete matching request without rerunning scenarios.
// Changed inputs or a generation interrupted before its request require a
// fresh prepared generation and nonce before review can be presented.
func (s *Requests) Prepare(ctx context.Context, id string, candidate codehost.FinalCandidate, scenarios string) (Packet, error) {
	scope, err := s.scope(ctx)
	if err != nil {
		return Packet{}, err
	}
	g, err := s.proposals.Read(ctx, id)
	if err != nil {
		return Packet{}, err
	}
	p := g.Proposal
	if p.Kind != "implementation" || p.Task != "" || p.SourceRef != scope.HeadRef || p.TargetRef != scope.TargetRef || (p.State != "open" && p.State != "prepared") || candidate.FeatureID != p.Feature || candidate.FeatureHead != p.Head || candidate.MainHead != p.TargetOID {
		return Packet{}, ErrIncomplete
	}
	if p.State == "prepared" && p.CandidatePreparation != nil && p.CandidatePreparation.Candidate == candidate && p.CandidatePreparation.ScenarioSetFingerprint == scenarios {
		request, err := s.artifacts.ReadRequest(ctx, g)
		if err == nil {
			packet, err := s.packet(ctx, scope, g, request)
			if !errors.Is(err, ErrEvidenceChanged) {
				return packet, err
			}
		}
		if err != nil && !errors.Is(err, codehost.ErrReviewArtifactNotFound) {
			return Packet{}, err
		}
	}
	preparation, err := codehost.NewCandidatePreparation(candidate, scenarios)
	if err != nil {
		return Packet{}, err
	}
	g, err = s.proposals.Publish(ctx, codehost.ProposalChange{
		ID: p.ID, Feature: p.Feature, Branch: p.Branch, Head: p.Head, State: "prepared",
		ExpectedProposalOID: g.OID, ExpectedBranchOID: p.Head, ExpectedTargetOID: p.TargetOID, CandidatePreparation: &preparation,
	})
	if err != nil {
		return Packet{}, err
	}
	binding, err := Bind(scope.Origin, g)
	if err != nil {
		return Packet{}, err
	}
	snapshot, err := Capture(ctx, binding, s.reader)
	if err != nil {
		return Packet{}, err
	}
	body, err := snapshot.Canonical()
	if err != nil {
		return Packet{}, err
	}
	evidence, err := s.artifacts.Retain(ctx, g, codehost.CandidateEvidence, body)
	if err != nil {
		return Packet{}, err
	}
	p = g.Proposal
	request := codehost.CandidateReviewRequest{SchemaVersion: codehost.CandidateRequestSchema, Binding: codehost.ReviewBinding{
		SchemaVersion: codehost.ReviewBindingSchema, ProposalID: p.ID, Kind: p.Kind, ProposalOID: g.OID, Sequence: p.Sequence,
		Operation: preparation.Operation, Branch: p.Branch, Head: p.Head, TargetRef: p.TargetRef, TargetOID: p.TargetOID,
		Digest: evidence.Identity.Digest, Nonce: preparation.Nonce, Evidence: evidence.Identity, Candidate: candidate, ScenarioSetFingerprint: scenarios,
	}}
	body, err = request.Canonical()
	if err != nil {
		return Packet{}, err
	}
	retained, err := s.artifacts.Retain(ctx, g, codehost.CandidateRequest, body)
	if err != nil {
		return Packet{}, err
	}
	return s.packet(ctx, scope, g, retained)
}

// Observe is read-only. A missing origin record is never repaired from a
// cached request, approval, or a passing scenario summary.
func (s *Requests) Observe(ctx context.Context, id string) (Packet, error) {
	scope, err := s.scope(ctx)
	if err != nil {
		return Packet{}, err
	}
	g, err := s.proposals.Read(ctx, id)
	if err != nil {
		return Packet{}, err
	}
	request, err := s.artifacts.ReadRequest(ctx, g)
	if err != nil {
		return Packet{}, err
	}
	return s.packet(ctx, scope, g, request)
}

func (s *Requests) ValidateReview(ctx context.Context, binding codehost.ReviewBinding) error {
	if binding.Validate() != nil || binding.Operation != codehost.FeatureFinalMerge {
		return codehost.ErrReviewInvalid
	}
	packet, err := s.Observe(ctx, binding.ProposalID)
	if err != nil {
		return err
	}
	if packet.Binding != binding {
		return codehost.ErrReviewInvalid
	}
	return nil
}

func (s *Requests) WithReview(ctx context.Context, binding codehost.ReviewBinding, mutate func() error) error {
	if mutate == nil || binding.Validate() != nil || binding.Operation != codehost.FeatureFinalMerge {
		return codehost.ErrReviewInvalid
	}
	lease, ok := s.reader.(EvidenceLease)
	if !ok {
		return ErrIncomplete
	}
	packet, err := s.Observe(ctx, binding.ProposalID)
	if err != nil {
		return err
	}
	if packet.Binding != binding {
		return codehost.ErrReviewInvalid
	}
	return lease.WithEvidence(ctx, packet.Snapshot.Binding, packet.Snapshot, func() error {
		if err := s.ValidateReview(ctx, binding); err != nil {
			return err
		}
		return mutate()
	})
}

func (s *Requests) packet(ctx context.Context, scope codehost.ProposalScope, g codehost.ProposalGeneration, retained codehost.ReviewArtifact) (Packet, error) {
	request, err := codehost.ParseCandidateRequest(retained.Body)
	if err != nil {
		return Packet{}, err
	}
	if err := request.Matches(g); err != nil {
		return Packet{}, err
	}
	evidence, err := s.artifacts.ReadRetained(ctx, g, codehost.CandidateEvidence, request.Binding.Evidence)
	if err != nil {
		return Packet{}, err
	}
	snapshot, err := Parse(evidence.Body)
	if err != nil {
		return Packet{}, err
	}
	binding, err := Bind(scope.Origin, g)
	if err != nil {
		return Packet{}, err
	}
	if err := Verify(ctx, binding, snapshot, s.reader); err != nil {
		return Packet{}, err
	}
	if _, err := s.scope(ctx); err != nil {
		return Packet{}, err
	}
	current, err := s.proposals.Read(ctx, g.Proposal.ID)
	if err != nil {
		return Packet{}, err
	}
	if !reflect.DeepEqual(current, g) {
		return Packet{}, codehost.ErrProposalConflict
	}
	if _, err := s.artifacts.ReadRetained(ctx, g, codehost.CandidateEvidence, evidence.Identity); err != nil {
		return Packet{}, err
	}
	currentRequest, err := s.artifacts.ReadRequest(ctx, g)
	if err != nil {
		return Packet{}, err
	}
	if !reflect.DeepEqual(currentRequest, retained) {
		return Packet{}, codehost.ErrReviewInvalid
	}
	request.Binding.Request = retained.Identity
	if err := request.Binding.Validate(); err != nil {
		return Packet{}, err
	}
	return Packet{PacketSchema, request.Binding, snapshot, evidence, retained, codehost.Trust{SharedAccount: true}}, nil
}
