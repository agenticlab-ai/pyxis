package localgit

import (
	"bytes"
	"context"
	"fmt"
	"math"
	"reflect"
	"slices"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/codehost"
)

type TaskLandingStore struct {
	options  ProposalOptions
	push     func(context.Context, string, ...string) error
	run      func(context.Context, string, []byte, []string, ...string) ([]byte, error)
	receipts codehost.ReceiptReader
}

func NewTaskLandings(options ProposalOptions, receipts codehost.ReceiptReader) *TaskLandingStore {
	return &TaskLandingStore{options: options, run: proposalGit, receipts: receipts}
}

var _ codehost.TaskLandings = (*TaskLandingStore)(nil)

// LandTask is the operator adapter. The consumer revalidates the complete
// current ReviewGate first; this operation verifies and leases the origin proof.
func (s *TaskLandingStore) LandTask(ctx context.Context, binding codehost.ReviewBinding) (codehost.TaskLandingResult, error) {
	artifacts := NewReviewArtifacts(s.options)
	scope, err := artifacts.Scope(ctx)
	if err != nil {
		return codehost.TaskLandingResult{}, err
	}
	proposals := artifacts.proposals
	g, err := proposals.Read(ctx, binding.ProposalID)
	if err != nil {
		return codehost.TaskLandingResult{}, err
	}
	p := g.Proposal
	if p.State == "landed" {
		result, err := s.VerifyTaskLanding(ctx, p.ID)
		if err == nil && result.Landing.Review.Receipt.Binding != binding {
			err = codehost.ErrReviewInvalid
		}
		return result, err
	}
	if p.State != "prepared" || p.Sequence == math.MaxUint64 || p.TargetRef != scope.TargetRef {
		return codehost.TaskLandingResult{}, codehost.ErrReviewInvalid
	}
	if s.receipts == nil {
		return codehost.TaskLandingResult{}, codehost.ErrReviewInvalid
	}
	receipts := s.receipts
	review, err := receipts.Verify(ctx, binding)
	if err != nil {
		return codehost.TaskLandingResult{}, err
	}
	request, err := artifacts.ReadRetained(ctx, g, codehost.TaskRequest, binding.Request)
	if err != nil {
		return codehost.TaskLandingResult{}, err
	}
	aggregate, err := artifacts.ReadRetained(ctx, g, codehost.TaskAggregate, binding.Evidence)
	if err != nil {
		return codehost.TaskLandingResult{}, err
	}
	root := s.options.Git.Root
	for _, oid := range []string{g.OID, p.Head, p.TargetOID, review.OID, request.Identity.OID, aggregate.Identity.OID} {
		if _, err := s.run(ctx, root, nil, nil, "fetch", "--no-tags", "--no-write-fetch-head", "--no-recurse-submodules", scope.Origin, oid); err != nil {
			return codehost.TaskLandingResult{}, err
		}
	}
	if _, err := s.run(ctx, root, nil, nil, "merge-base", "--is-ancestor", p.TargetOID, p.Head); err != nil {
		return codehost.TaskLandingResult{}, fmt.Errorf("%w: task must descend from its reviewed feature base", codehost.ErrReviewInvalid)
	}
	tree, err := s.run(ctx, root, nil, nil, "rev-parse", p.Head+"^{tree}")
	if err != nil {
		return codehost.TaskLandingResult{}, err
	}
	env := []string{"GIT_AUTHOR_NAME=Pyxis local task", "GIT_COMMITTER_NAME=Pyxis local task", "GIT_AUTHOR_EMAIL=local-task@pyxis.invalid", "GIT_COMMITTER_EMAIL=local-task@pyxis.invalid", "GIT_AUTHOR_DATE=2000-01-01T00:00:00Z", "GIT_COMMITTER_DATE=2000-01-01T00:00:00Z"}
	oid, err := s.run(ctx, root, []byte("Pyxis land task "+p.Feature+"/"+p.Task+"\n"), env, "commit-tree", strings.TrimSpace(string(tree)), "-p", p.TargetOID)
	if err != nil {
		return codehost.TaskLandingResult{}, err
	}
	landing := codehost.TaskLanding{SchemaVersion: codehost.TaskLandingSchema, Feature: p.Feature, Task: p.Task, MergeCommit: strings.TrimSpace(string(oid)), Aggregate: aggregate, Request: request, Review: review}
	p.Sequence++
	p.PreviousOID, p.State, p.TaskPreparation, p.TaskLanding = g.OID, "landed", nil, &landing
	p.ArchiveRef = archiveRef(p)
	if err := validateProposal(ctx, root, p); err != nil {
		return codehost.TaskLandingResult{}, err
	}
	finalOID, err := prepareProposal(ctx, root, p)
	if err != nil {
		return codehost.TaskLandingResult{}, err
	}
	currentScope, err := artifacts.Scope(ctx)
	if err != nil {
		return codehost.TaskLandingResult{}, err
	}
	if currentScope != scope {
		return codehost.TaskLandingResult{}, codehost.ErrProposalConflict
	}
	lease, ok := receipts.(codehost.ReviewLease)
	if !ok {
		return codehost.TaskLandingResult{}, codehost.ErrReviewInvalid
	}
	args := []string{"push", "--atomic", "--no-recurse-submodules",
		"--force-with-lease=" + p.TargetRef + ":" + p.TargetOID,
		"--force-with-lease=" + proposalPrefix + p.ID + ":" + g.OID,
		"--force-with-lease=" + p.Branch + ":" + p.Head,
		"--force-with-lease=" + p.ArchiveRef + ":",
		scope.Origin, landing.MergeCommit + ":" + p.TargetRef, finalOID + ":" + proposalPrefix + p.ID,
		finalOID + ":" + p.ArchiveRef, ":" + p.Branch}
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
		return withUnchangedRefs(ctx, scope.Origin, []unchangedRef{
			{Ref: binding.Evidence.Ref, OID: binding.Evidence.OID}, {Ref: binding.Request.Ref, OID: binding.Request.OID},
			{Ref: receiptRef(binding), OID: review.OID},
		}, func() error { return push(ctx, root, args...) })
	})
	if err != nil {
		if existing, verifyErr := s.VerifyTaskLanding(ctx, p.ID); verifyErr == nil && reflect.DeepEqual(existing.Landing, landing) {
			return existing, nil
		}
		if strings.Contains(err.Error(), "does not support --atomic push") {
			return codehost.TaskLandingResult{}, fmt.Errorf("%w: %w", codehost.ErrAtomicUnavailable, err)
		}
		return codehost.TaskLandingResult{}, fmt.Errorf("%w: atomic task landing refused: %w", codehost.ErrProposalConflict, err)
	}
	result, err := s.VerifyTaskLanding(ctx, p.ID)
	result.Created = err == nil
	return result, err
}

// VerifyTaskLanding permits later feature descendants, but requires the exact
// original squash, reviewed predecessor, and retained/embedded origin proof.
func (s *TaskLandingStore) VerifyTaskLanding(ctx context.Context, id string) (codehost.TaskLandingResult, error) {
	artifacts := NewReviewArtifacts(s.options)
	scope, err := artifacts.Scope(ctx)
	if err != nil {
		return codehost.TaskLandingResult{}, err
	}
	proposals := artifacts.proposals
	final, err := proposals.Read(ctx, id)
	if err != nil {
		return codehost.TaskLandingResult{}, err
	}
	p := final.Proposal
	if p.State != "landed" || p.TaskLanding == nil {
		return codehost.TaskLandingResult{}, codehost.ErrReviewInvalid
	}
	landing := *p.TaskLanding
	b := landing.Review.Receipt.Binding
	prepared, err := proposals.read(ctx, scope.Origin, id, b.ProposalOID)
	if err != nil {
		return codehost.TaskLandingResult{}, err
	}
	request, err := codehost.ParseTaskRequest(landing.Request.Body)
	if err != nil || request.Matches(prepared) != nil {
		return codehost.TaskLandingResult{}, codehost.ErrReviewInvalid
	}
	for _, embedded := range []codehost.ReviewArtifact{landing.Aggregate, landing.Request} {
		retained, err := artifacts.ReadRetained(ctx, prepared, embedded.Kind, embedded.Identity)
		if err != nil {
			return codehost.TaskLandingResult{}, err
		}
		if !reflect.DeepEqual(retained, embedded) {
			return codehost.TaskLandingResult{}, codehost.ErrReviewInvalid
		}
	}
	review, err := taskReceiptStore(s.options).read(ctx, scope.Origin, b)
	if err != nil {
		return codehost.TaskLandingResult{}, err
	}
	if review != landing.Review {
		return codehost.TaskLandingResult{}, codehost.ErrReviewInvalid
	}
	parents, err := s.run(ctx, scope.Origin, nil, nil, "rev-list", "--parents", "-n", "1", landing.MergeCommit)
	if err != nil || !slices.Equal(strings.Fields(string(parents)), []string{landing.MergeCommit, b.TargetOID}) {
		return codehost.TaskLandingResult{}, codehost.ErrReviewInvalid
	}
	landedTree, err := s.run(ctx, scope.Origin, nil, nil, "rev-parse", landing.MergeCommit+"^{tree}")
	if err != nil {
		return codehost.TaskLandingResult{}, err
	}
	reviewedTree, err := s.run(ctx, scope.Origin, nil, nil, "rev-parse", b.Head+"^{tree}")
	if err != nil || !bytes.Equal(landedTree, reviewedTree) {
		return codehost.TaskLandingResult{}, codehost.ErrReviewInvalid
	}
	refs, err := proposalRefs(ctx, scope.Origin, p.TargetRef)
	if err != nil {
		return codehost.TaskLandingResult{}, err
	}
	featureHead := refs[p.TargetRef]
	for _, pair := range [][2]string{{b.TargetOID, b.Head}, {landing.MergeCommit, featureHead}} {
		if _, err := s.run(ctx, scope.Origin, nil, nil, "merge-base", "--is-ancestor", pair[0], pair[1]); err != nil {
			return codehost.TaskLandingResult{}, fmt.Errorf("%w: task landing ancestry is absent", codehost.ErrReviewInvalid)
		}
	}
	current, err := proposals.Read(ctx, id)
	if err != nil {
		return codehost.TaskLandingResult{}, err
	}
	latest, err := proposalRefs(ctx, scope.Origin, p.TargetRef)
	if err != nil || current.OID != final.OID || latest[p.TargetRef] != featureHead {
		return codehost.TaskLandingResult{}, codehost.ErrProposalConflict
	}
	return codehost.TaskLandingResult{Landing: landing, Prepared: prepared, FeatureHead: featureHead}, nil
}
