package localgit

import (
	"context"
	"fmt"

	"github.com/agenticlab-ai/pyxis/internal/codehost"
	"github.com/agenticlab-ai/pyxis/internal/featuredrive"
)

type featureProposals struct {
	feature, integration, mainline string
	tasks, final                   codehost.Proposals
}

// NewProposals binds immutable destinations from an already verified
// drive. Caller changes carry observed OIDs, never a caller-selected target ref.
// Neither the returned interface nor its implementation has a landing method.
func NewProposals(ctx context.Context, drive featuredrive.FeatureDrive, tasks, final codehost.ScopedProposals) (featuredrive.LocalProposals, error) {
	if err := drive.Validate(); err != nil {
		return nil, fmt.Errorf("bind local feature proposals: %w", err)
	}
	if tasks == nil || final == nil {
		return nil, invalidProposal("task and final proposal ports are required")
	}
	taskScope, err := tasks.Scope(ctx)
	if err != nil {
		return nil, fmt.Errorf("bind task proposal scope: %w", err)
	}
	finalScope, err := final.Scope(ctx)
	if err != nil {
		return nil, fmt.Errorf("bind final proposal scope: %w", err)
	}
	if taskScope.SchemaVersion != codehost.ProposalScopeSchema || finalScope.SchemaVersion != codehost.ProposalScopeSchema ||
		taskScope.Origin == "" || taskScope.Origin != finalScope.Origin || taskScope.Kind != "implementation" || finalScope.Kind != "implementation" ||
		taskScope.MainlineRef != drive.TargetMainRef || finalScope.MainlineRef != drive.TargetMainRef ||
		taskScope.TargetRef != drive.IntegrationRef || taskScope.HeadRef != "" || finalScope.TargetRef != drive.TargetMainRef || finalScope.HeadRef != drive.IntegrationRef || drive.IntegrationRef == drive.TargetMainRef {
		return nil, invalidProposal("drive destinations contradict configured mainline or feature isolation")
	}
	return &featureProposals{
		feature: drive.FeatureID, integration: drive.IntegrationRef, mainline: drive.TargetMainRef,
		tasks: tasks, final: final,
	}, nil
}

func (s *featureProposals) RequestTask(ctx context.Context, change codehost.ProposalChange) (featuredrive.LocalProposalView, error) {
	return s.request(ctx, s.tasks, change, featuredrive.LocalTaskMerge)
}

func (s *featureProposals) RequestFinal(ctx context.Context, change codehost.ProposalChange) (featuredrive.LocalProposalView, error) {
	return s.request(ctx, s.final, change, featuredrive.LocalFinalMerge)
}

func (s *featureProposals) ObserveTask(ctx context.Context, id string) (featuredrive.LocalProposalView, error) {
	return s.observe(ctx, s.tasks, id, featuredrive.LocalTaskMerge)
}

func (s *featureProposals) ObserveFinal(ctx context.Context, id string) (featuredrive.LocalProposalView, error) {
	return s.observe(ctx, s.final, id, featuredrive.LocalFinalMerge)
}

func (s *featureProposals) request(ctx context.Context, store codehost.Proposals, change codehost.ProposalChange, operation string) (featuredrive.LocalProposalView, error) {
	if change.Feature != s.feature || (change.Task == "") != (operation == featuredrive.LocalFinalMerge) ||
		change.Branch == s.integration || change.Branch == s.mainline || change.State != "open" || change.ReviewRequest != nil || change.TaskPreparation != nil || change.CandidatePreparation != nil {
		return featuredrive.LocalProposalView{}, invalidProposal("drive can request only its own open task or distinct final proposal")
	}
	generation, err := store.Publish(ctx, change)
	if err != nil {
		return featuredrive.LocalProposalView{}, fmt.Errorf("request %s proposal: %w", operation, err)
	}
	return s.view(generation, operation)
}

func (s *featureProposals) observe(ctx context.Context, store codehost.Proposals, id, operation string) (featuredrive.LocalProposalView, error) {
	generation, err := store.Read(ctx, id)
	if err != nil {
		return featuredrive.LocalProposalView{}, fmt.Errorf("observe %s proposal: %w", operation, err)
	}
	return s.view(generation, operation)
}

func invalidProposal(message string) error {
	return fmt.Errorf("%w: %s", codehost.ErrProposalInvalid, message)
}

func (s *featureProposals) view(generation codehost.ProposalGeneration, operation string) (featuredrive.LocalProposalView, error) {
	p := generation.Proposal
	target := s.integration
	if operation == featuredrive.LocalFinalMerge {
		target = s.mainline
	}
	if p.Feature != s.feature || p.Kind != "implementation" || (p.Task == "") != (operation == featuredrive.LocalFinalMerge) || p.TargetRef != target || p.Branch == s.integration || p.Branch == s.mainline {
		return featuredrive.LocalProposalView{}, invalidProposal("origin proposal contradicts feature, purpose, or workflow destination")
	}
	return featuredrive.LocalProposalView{
		SchemaVersion: featuredrive.LocalProposalSchema, Operation: operation, Generation: generation,
		Trust: codehost.Trust{SharedAccount: true},
	}, nil
}
