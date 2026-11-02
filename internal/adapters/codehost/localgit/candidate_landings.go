package localgit

import (
	"context"
	"fmt"
	"math"
	"reflect"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/codehost"
)

type CandidateLandingStore struct {
	options  ProposalOptions
	push     func(context.Context, string, ...string) error
	run      func(context.Context, string, []byte, []string, ...string) ([]byte, error)
	receipts codehost.ReceiptReader
}

func NewCandidateLandings(options ProposalOptions, receipts codehost.ReceiptReader) *CandidateLandingStore {
	return &CandidateLandingStore{options: options, run: proposalGit, receipts: receipts}
}

var _ codehost.CandidateLandings = (*CandidateLandingStore)(nil)

// LandCandidate is an operator adapter. Its consumer refreshes the complete
// current scenario policy and runner/manual attribution before invoking it.
func (s *CandidateLandingStore) LandCandidate(ctx context.Context, binding codehost.ReviewBinding) (codehost.CandidateLandingResult, error) {
	artifacts := NewReviewArtifacts(s.options)
	scope, err := artifacts.Scope(ctx)
	if err != nil {
		return codehost.CandidateLandingResult{}, err
	}
	if scope.HeadRef == "" || scope.TargetRef != scope.MainlineRef || binding.Validate() != nil || binding.Operation != codehost.FeatureFinalMerge {
		return codehost.CandidateLandingResult{}, codehost.ErrReviewInvalid
	}
	proposals := artifacts.proposals
	g, err := proposals.Read(ctx, binding.ProposalID)
	if err != nil {
		return codehost.CandidateLandingResult{}, err
	}
	p := g.Proposal
	if p.State == "landed" {
		result, err := s.VerifyCandidateLanding(ctx, p.ID)
		if err == nil && result.Landing.Review.Receipt.Binding != binding {
			err = codehost.ErrReviewInvalid
		}
		return result, err
	}
	if p.State != "prepared" || p.Task != "" || p.Sequence == math.MaxUint64 || p.SourceRef != scope.HeadRef || p.TargetRef != scope.TargetRef {
		return codehost.CandidateLandingResult{}, codehost.ErrReviewInvalid
	}
	if s.receipts == nil {
		return codehost.CandidateLandingResult{}, codehost.ErrReviewInvalid
	}
	receipts := s.receipts
	review, err := receipts.Verify(ctx, binding)
	if err != nil {
		return codehost.CandidateLandingResult{}, err
	}
	request, err := artifacts.ReadRetained(ctx, g, codehost.CandidateRequest, binding.Request)
	if err != nil {
		return codehost.CandidateLandingResult{}, err
	}
	evidence, err := artifacts.ReadRetained(ctx, g, codehost.CandidateEvidence, binding.Evidence)
	if err != nil {
		return codehost.CandidateLandingResult{}, err
	}
	root, candidate := s.options.Git.Root, binding.Candidate
	for _, oid := range []string{g.OID, p.Head, p.TargetOID, candidate.Commit, review.OID, request.Identity.OID, evidence.Identity.OID} {
		if _, err := s.run(ctx, root, nil, nil, "fetch", "--no-tags", "--no-write-fetch-head", "--no-recurse-submodules", scope.Origin, oid); err != nil {
			return codehost.CandidateLandingResult{}, err
		}
	}
	if err := verifyCandidateObjects(ctx, root, candidate); err != nil {
		return codehost.CandidateLandingResult{}, err
	}
	merge, err := s.prepare(ctx, root, p.TargetOID, candidate.Tree, "deliver feature "+p.Feature)
	if err != nil {
		return codehost.CandidateLandingResult{}, err
	}
	marker, err := s.prepare(ctx, root, p.Head, candidate.Tree, "retain feature "+p.Feature)
	if err != nil {
		return codehost.CandidateLandingResult{}, err
	}
	landing := codehost.CandidateLanding{SchemaVersion: codehost.CandidateLandingSchema, Candidate: candidate, MergeCommit: merge, FeatureMarker: marker, Evidence: evidence, Request: request, Review: review}
	p.Sequence++
	p.PreviousOID, p.State, p.CandidatePreparation, p.CandidateLanding = g.OID, "landed", nil, &landing
	p.ArchiveRef = archiveRef(p)
	if err := validateProposal(ctx, root, p); err != nil {
		return codehost.CandidateLandingResult{}, err
	}
	finalOID, err := prepareProposal(ctx, root, p)
	if err != nil {
		return codehost.CandidateLandingResult{}, err
	}
	currentScope, err := artifacts.Scope(ctx)
	if err != nil {
		return codehost.CandidateLandingResult{}, err
	}
	if currentScope != scope {
		return codehost.CandidateLandingResult{}, codehost.ErrProposalConflict
	}
	lease, ok := receipts.(codehost.ReviewLease)
	if !ok {
		return codehost.CandidateLandingResult{}, codehost.ErrReviewInvalid
	}
	args := []string{"push", "--atomic", "--no-recurse-submodules",
		"--force-with-lease=" + p.TargetRef + ":" + p.TargetOID, "--force-with-lease=" + p.SourceRef + ":" + p.Head,
		"--force-with-lease=" + proposalPrefix + p.ID + ":" + g.OID, "--force-with-lease=" + p.Branch + ":" + p.Head, "--force-with-lease=" + p.ArchiveRef + ":",
		scope.Origin, landing.MergeCommit + ":" + p.TargetRef, landing.FeatureMarker + ":" + p.SourceRef, finalOID + ":" + proposalPrefix + p.ID, finalOID + ":" + p.ArchiveRef, ":" + p.Branch}
	push := s.push
	if push == nil {
		push = func(ctx context.Context, root string, args ...string) error {
			_, err := s.run(ctx, root, nil, nil, args...)
			return err
		}
	}
	err = lease.WithReview(ctx, binding, func() error {
		currentReview, err := receipts.Verify(ctx, binding)
		if err != nil {
			return err
		}
		if currentReview != review {
			return codehost.ErrReviewInvalid
		}
		return withUnchangedRefs(ctx, scope.Origin, []unchangedRef{{Ref: binding.Evidence.Ref, OID: binding.Evidence.OID}, {Ref: binding.Request.Ref, OID: binding.Request.OID}, {Ref: receiptRef(binding), OID: review.OID}}, func() error { return push(ctx, root, args...) })
	})
	if err != nil {
		if existing, verifyErr := s.VerifyCandidateLanding(ctx, p.ID); verifyErr == nil && reflect.DeepEqual(existing.Landing, landing) {
			return existing, nil
		}
		if strings.Contains(err.Error(), "does not support --atomic push") {
			return codehost.CandidateLandingResult{}, fmt.Errorf("%w: %w", codehost.ErrAtomicUnavailable, err)
		}
		return codehost.CandidateLandingResult{}, fmt.Errorf("%w: atomic candidate landing refused: %w", codehost.ErrProposalConflict, err)
	}
	result, err := s.VerifyCandidateLanding(ctx, p.ID)
	result.Created = err == nil
	return result, err
}

func (s *CandidateLandingStore) prepare(ctx context.Context, root, parent, tree, message string) (string, error) {
	env := []string{"GIT_AUTHOR_NAME=Pyxis local feature", "GIT_COMMITTER_NAME=Pyxis local feature", "GIT_AUTHOR_EMAIL=local-feature@pyxis.invalid", "GIT_COMMITTER_EMAIL=local-feature@pyxis.invalid", "GIT_AUTHOR_DATE=2000-01-01T00:00:00Z", "GIT_COMMITTER_DATE=2000-01-01T00:00:00Z"}
	oid, err := s.run(ctx, root, []byte("Pyxis "+message+"\n"), env, "commit-tree", tree, "-p", parent)
	return strings.TrimSpace(string(oid)), err
}
