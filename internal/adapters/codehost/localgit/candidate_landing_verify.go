package localgit

import (
	"bytes"
	"context"
	"fmt"
	"reflect"
	"slices"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/codehost"
)

// VerifyCandidateLanding checks delivered L rather than the feature marker.
// Later main and feature descendants are allowed; retained proof is mandatory.
func (s *CandidateLandingStore) VerifyCandidateLanding(ctx context.Context, id string) (codehost.CandidateLandingResult, error) {
	artifacts := NewReviewArtifacts(s.options)
	scope, err := artifacts.Scope(ctx)
	if err != nil {
		return codehost.CandidateLandingResult{}, err
	}
	if scope.HeadRef == "" || scope.TargetRef != scope.MainlineRef {
		return codehost.CandidateLandingResult{}, codehost.ErrReviewInvalid
	}
	proposals := artifacts.proposals
	final, err := proposals.Read(ctx, id)
	if err != nil {
		return codehost.CandidateLandingResult{}, err
	}
	p := final.Proposal
	if p.State != "landed" || p.CandidateLanding == nil || p.Task != "" {
		return codehost.CandidateLandingResult{}, codehost.ErrReviewInvalid
	}
	landing := *p.CandidateLanding
	b := landing.Review.Receipt.Binding
	prepared, err := proposals.read(ctx, scope.Origin, id, b.ProposalOID)
	if err != nil {
		return codehost.CandidateLandingResult{}, err
	}
	request, err := codehost.ParseCandidateRequest(landing.Request.Body)
	if err != nil || request.Matches(prepared) != nil {
		return codehost.CandidateLandingResult{}, codehost.ErrReviewInvalid
	}
	for _, embedded := range []codehost.ReviewArtifact{landing.Evidence, landing.Request} {
		retained, err := artifacts.ReadRetained(ctx, prepared, embedded.Kind, embedded.Identity)
		if err != nil {
			return codehost.CandidateLandingResult{}, err
		}
		if !reflect.DeepEqual(retained, embedded) {
			return codehost.CandidateLandingResult{}, codehost.ErrReviewInvalid
		}
	}
	review, err := candidateReceiptStore(s.options).read(ctx, scope.Origin, b)
	if err != nil {
		return codehost.CandidateLandingResult{}, err
	}
	if review != landing.Review {
		return codehost.CandidateLandingResult{}, codehost.ErrReviewInvalid
	}
	if err := verifyCandidateObjects(ctx, scope.Origin, landing.Candidate); err != nil {
		return codehost.CandidateLandingResult{}, err
	}
	for _, object := range [][2]string{{landing.MergeCommit, b.TargetOID}, {landing.FeatureMarker, b.Head}} {
		parents, err := s.run(ctx, scope.Origin, nil, nil, "rev-list", "--parents", "-n", "1", object[0])
		if err != nil || !slices.Equal(strings.Fields(string(parents)), []string{object[0], object[1]}) {
			return codehost.CandidateLandingResult{}, codehost.ErrReviewInvalid
		}
		tree, err := s.run(ctx, scope.Origin, nil, nil, "rev-parse", object[0]+"^{tree}")
		if err != nil || !bytes.Equal(tree, []byte(landing.Candidate.Tree+"\n")) {
			return codehost.CandidateLandingResult{}, codehost.ErrReviewInvalid
		}
	}
	refs, err := proposalRefs(ctx, scope.Origin, p.TargetRef, p.SourceRef)
	if err != nil {
		return codehost.CandidateLandingResult{}, err
	}
	main, feature := refs[p.TargetRef], refs[p.SourceRef]
	for _, pair := range [][2]string{{landing.MergeCommit, main}, {landing.FeatureMarker, feature}} {
		if _, err := s.run(ctx, scope.Origin, nil, nil, "merge-base", "--is-ancestor", pair[0], pair[1]); err != nil {
			return codehost.CandidateLandingResult{}, fmt.Errorf("%w: delivered candidate ancestry is absent", codehost.ErrReviewInvalid)
		}
	}
	current, err := proposals.Read(ctx, id)
	if err != nil {
		return codehost.CandidateLandingResult{}, err
	}
	latest, err := proposalRefs(ctx, scope.Origin, p.TargetRef, p.SourceRef)
	if err != nil || current.OID != final.OID || latest[p.TargetRef] != main || latest[p.SourceRef] != feature {
		return codehost.CandidateLandingResult{}, codehost.ErrProposalConflict
	}
	return codehost.CandidateLandingResult{Landing: landing, Prepared: prepared, MainHead: main, FeatureHead: feature}, nil
}
