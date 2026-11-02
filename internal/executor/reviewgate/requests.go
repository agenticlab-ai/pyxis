package reviewgate

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"reflect"

	"github.com/agenticlab-ai/pyxis/internal/codehost"
)

const PacketSchema = "local-task-review-packet/v1"

type Packet struct {
	SchemaVersion string                  `json:"schema_version"`
	Binding       codehost.ReviewBinding  `json:"binding"`
	Snapshot      Snapshot                `json:"snapshot"`
	Aggregate     codehost.ReviewArtifact `json:"aggregate"`
	Request       codehost.ReviewArtifact `json:"request"`
	Trust         codehost.Trust          `json:"trust"`
}

// TaskRequests coordinates preparation and retained evidence. Its ports
// expose no declaration of approval, target mutation, or tracker completion.
type TaskRequests struct {
	proposals codehost.ScopedProposals
	artifacts codehost.ReviewArtifacts
	reader    Reader
	bound     codehost.ProposalScope
}

func NewTaskRequests(ctx context.Context, proposals codehost.ScopedProposals, artifacts codehost.ReviewArtifacts, reader Reader) (*TaskRequests, error) {
	if proposals == nil || artifacts == nil || reader == nil {
		return nil, fmt.Errorf("%w: missing task review ports", ErrIncomplete)
	}
	s := &TaskRequests{proposals: proposals, artifacts: artifacts, reader: reader}
	scope, err := s.scope(ctx)
	if err != nil {
		return nil, err
	}
	s.bound = scope
	return s, nil
}

func (s *TaskRequests) scope(ctx context.Context) (codehost.ProposalScope, error) {
	scope, err := s.proposals.Scope(ctx)
	if err != nil {
		return scope, err
	}
	retention, err := s.artifacts.Scope(ctx)
	if err != nil {
		return scope, err
	}
	if scope.SchemaVersion != codehost.ProposalScopeSchema || scope.Kind != "implementation" || scope.Origin == "" || scope.TargetRef == scope.MainlineRef || scope.HeadRef != "" || retention != scope || (s.bound.Origin != "" && scope != s.bound) {
		return scope, fmt.Errorf("%w: task review ports do not share the bound workflow", ErrIncomplete)
	}
	return scope, nil
}

// Prepare resumes a complete retained request without rerunning review or
// changing its nonce. An interrupted generation with no retained request, or
// valid proof under obsolete policy, requires a new generation and nonce.
func (s *TaskRequests) Prepare(ctx context.Context, id string) (Packet, error) {
	scope, err := s.scope(ctx)
	if err != nil {
		return Packet{}, err
	}
	g, err := s.proposals.Read(ctx, id)
	if err != nil {
		return Packet{}, err
	}
	p := g.Proposal
	if p.Kind != "implementation" || p.Task == "" || p.TargetRef != scope.TargetRef || (p.State != "open" && p.State != "prepared") {
		return Packet{}, fmt.Errorf("%w: expected an active task proposal", ErrIncomplete)
	}
	if p.State == "prepared" {
		request, err := s.artifacts.ReadRequest(ctx, g)
		if err == nil {
			packet, packetErr := s.packet(ctx, scope, g, request)
			if !errors.Is(packetErr, ErrPolicyStale) {
				return packet, packetErr
			}
		}
		if err != nil && !errors.Is(err, codehost.ErrReviewArtifactNotFound) {
			return Packet{}, err
		}
	}
	preparation, err := codehost.NewTaskPreparation()
	if err != nil {
		return Packet{}, err
	}
	g, err = s.proposals.Publish(ctx, codehost.ProposalChange{
		ID: p.ID, Feature: p.Feature, Task: p.Task, Branch: p.Branch, Head: p.Head, State: "prepared",
		ExpectedProposalOID: g.OID, ExpectedBranchOID: p.Head, ExpectedTargetOID: p.TargetOID, TaskPreparation: &preparation,
	})
	if err != nil {
		return Packet{}, err
	}
	binding, err := Bind(scope.Origin, g)
	if err != nil {
		return Packet{}, err
	}
	policy, err := s.reader.Policy(ctx, binding)
	if err != nil {
		return Packet{}, err
	}
	snapshot, err := s.reader.Snapshot(ctx, binding)
	if err != nil {
		return Packet{}, err
	}
	if err := Validate(binding, snapshot, policy); err != nil {
		return Packet{}, err
	}
	body, err := Canonical(snapshot)
	if err != nil {
		return Packet{}, err
	}
	aggregate, err := s.artifacts.Retain(ctx, g, codehost.TaskAggregate, body)
	if err != nil {
		return Packet{}, err
	}
	p = g.Proposal
	request := codehost.TaskReviewRequest{SchemaVersion: codehost.TaskRequestSchema, Binding: codehost.ReviewBinding{
		SchemaVersion: codehost.ReviewBindingSchema, ProposalID: p.ID, Kind: p.Kind, ProposalOID: g.OID, Sequence: p.Sequence,
		Operation: preparation.Operation, Branch: p.Branch, Head: p.Head, TargetRef: p.TargetRef, TargetOID: p.TargetOID,
		Digest: aggregate.Identity.Digest, Nonce: preparation.Nonce, Evidence: aggregate.Identity,
	}}
	body, err = request.Canonical()
	if err != nil {
		return Packet{}, err
	}
	retained, err := s.artifacts.Retain(ctx, g, codehost.TaskRequest, body)
	if err != nil {
		return Packet{}, err
	}
	return s.packet(ctx, scope, g, retained)
}

// Observe only reads an already retained request. It cannot prepare a new
// generation, repair absent evidence, or infer approval from a green gate.
func (s *TaskRequests) Observe(ctx context.Context, id string) (Packet, error) {
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

func (s *TaskRequests) ValidateReview(ctx context.Context, binding codehost.ReviewBinding) error {
	if binding.Validate() != nil || binding.Operation != codehost.TaskMerge {
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

func (s *TaskRequests) WithReview(ctx context.Context, binding codehost.ReviewBinding, mutate func() error) error {
	if mutate == nil || binding.Validate() != nil || binding.Operation != codehost.TaskMerge {
		return codehost.ErrReviewInvalid
	}
	lease, ok := s.reader.(SnapshotLease)
	if !ok {
		return fmt.Errorf("%w: task landing requires a complete review snapshot lease", ErrIncomplete)
	}
	packet, err := s.Observe(ctx, binding.ProposalID)
	if err != nil {
		return err
	}
	if packet.Binding != binding {
		return codehost.ErrReviewInvalid
	}
	return lease.WithSnapshot(ctx, packet.Snapshot.Binding, packet.Snapshot, func() error {
		if err := s.ValidateReview(ctx, binding); err != nil {
			return err
		}
		return mutate()
	})
}

func (s *TaskRequests) packet(ctx context.Context, scope codehost.ProposalScope, g codehost.ProposalGeneration, retained codehost.ReviewArtifact) (Packet, error) {
	request, err := codehost.ParseTaskRequest(retained.Body)
	if err != nil {
		return Packet{}, err
	}
	if err := request.Matches(g); err != nil {
		return Packet{}, err
	}
	aggregate, err := s.artifacts.ReadRetained(ctx, g, codehost.TaskAggregate, request.Binding.Evidence)
	if err != nil {
		return Packet{}, err
	}
	snapshot, err := Parse(aggregate.Body)
	if err != nil {
		return Packet{}, err
	}
	binding, err := Bind(scope.Origin, g)
	if err != nil {
		return Packet{}, err
	}
	policy, err := s.reader.Policy(ctx, binding)
	if err != nil {
		return Packet{}, err
	}
	if err := Validate(binding, snapshot, snapshot.Policy); err != nil {
		return Packet{}, err
	}
	if !reflect.DeepEqual(snapshot.Policy, policy) {
		return Packet{}, ErrPolicyStale
	}
	currentSnapshot, err := s.reader.Snapshot(ctx, binding)
	if err != nil {
		return Packet{}, err
	}
	if err := Validate(binding, currentSnapshot, policy); err != nil {
		return Packet{}, err
	}
	currentBody, err := Canonical(currentSnapshot)
	if err != nil {
		return Packet{}, err
	}
	if !bytes.Equal(aggregate.Body, currentBody) {
		return Packet{}, ErrPolicyStale
	}
	// All objects must still be the presented origin objects after policy and
	// proof reads. This rejects movement at the last presentation boundary.
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
	if _, err := s.artifacts.ReadRetained(ctx, g, codehost.TaskAggregate, aggregate.Identity); err != nil {
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
	return Packet{PacketSchema, request.Binding, snapshot, aggregate, retained, codehost.Trust{SharedAccount: true}}, nil
}
