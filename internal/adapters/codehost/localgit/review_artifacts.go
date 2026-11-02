package localgit

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/codehost"
)

type ReviewArtifactStore struct {
	proposals *ProposalStore
	commit    func(context.Context, string, []byte) error
}

func NewReviewArtifacts(options ProposalOptions) *ReviewArtifactStore {
	return &ReviewArtifactStore{proposals: NewProposals(options)}
}

var _ codehost.ReviewArtifacts = (*ReviewArtifactStore)(nil)

func (s *ReviewArtifactStore) Scope(ctx context.Context) (codehost.ProposalScope, error) {
	scope, err := s.proposals.Scope(ctx)
	if err == nil && (scope.Kind != "implementation" || (scope.HeadRef == "" && scope.TargetRef == scope.MainlineRef) || (scope.HeadRef != "" && (scope.TargetRef != scope.MainlineRef || scope.HeadRef == scope.TargetRef))) {
		err = invalidProposal("review artifacts require a task or final-feature workflow")
	}
	return scope, err
}

func (s *ReviewArtifactStore) generation(ctx context.Context, g codehost.ProposalGeneration, current bool) (codehost.ProposalScope, error) {
	scope, err := s.Scope(ctx)
	if err != nil {
		return scope, err
	}
	p := g.Proposal
	if p.Kind != "implementation" || p.State != "prepared" {
		return scope, codehost.ErrReviewInvalid
	}
	if scope.HeadRef == "" {
		if p.Task == "" || p.TaskPreparation == nil || p.TaskPreparation.Validate() != nil || p.CandidatePreparation != nil {
			return scope, codehost.ErrReviewInvalid
		}
	} else if p.Task != "" || p.TaskPreparation != nil || p.CandidatePreparation == nil || p.CandidatePreparation.Validate() != nil || p.SourceRef != scope.HeadRef {
		return scope, codehost.ErrReviewInvalid
	}
	var actual codehost.ProposalGeneration
	if current {
		actual, err = s.proposals.Read(ctx, p.ID)
	} else {
		actual, err = s.proposals.read(ctx, scope.Origin, p.ID, g.OID)
	}
	if err == nil && !reflect.DeepEqual(actual, g) {
		err = codehost.ErrProposalConflict
	}
	if err == nil && current {
		refs, readErr := proposalRefs(ctx, scope.Origin, proposalPrefix+p.ID, p.Branch, p.TargetRef, p.SourceRef)
		if readErr != nil {
			err = readErr
		} else if refs[proposalPrefix+p.ID] != g.OID || refs[p.Branch] != p.Head || refs[p.TargetRef] != p.TargetOID || (p.SourceRef != "" && refs[p.SourceRef] != p.Head) {
			err = codehost.ErrProposalConflict
		}
	}
	if err == nil && p.Task != "" {
		if _, ancestryErr := proposalGit(ctx, scope.Origin, nil, nil, "merge-base", "--is-ancestor", p.TargetOID, p.Head); ancestryErr != nil {
			err = fmt.Errorf("%w: reviewed task head must descend from its feature base: %w", codehost.ErrReviewInvalid, ancestryErr)
		}
	}
	return scope, err
}

func artifactLocation(g codehost.ProposalGeneration, kind, digest string) (string, string, error) {
	switch kind {
	case codehost.TaskAggregate:
		if g.Proposal.Task == "" {
			return "", "", codehost.ErrReviewInvalid
		}
		return "refs/pyxis/review-aggregates/" + g.Proposal.ID + "/" + strings.TrimPrefix(digest, "sha256:"), "aggregate.json", nil
	case codehost.TaskRequest:
		if g.Proposal.Task == "" {
			return "", "", codehost.ErrReviewInvalid
		}
		return "refs/pyxis/review-requests/" + g.Proposal.ID + "/" + g.OID, "request.json", nil
	case codehost.CandidateEvidence:
		if g.Proposal.Task != "" {
			return "", "", codehost.ErrReviewInvalid
		}
		return "refs/pyxis/candidate-evidence/" + g.Proposal.ID + "/" + strings.TrimPrefix(digest, "sha256:"), "evidence.json", nil
	case codehost.CandidateRequest:
		if g.Proposal.Task != "" {
			return "", "", codehost.ErrReviewInvalid
		}
		return "refs/pyxis/review-requests/" + g.Proposal.ID + "/" + g.OID, "request.json", nil
	default:
		return "", "", codehost.ErrReviewInvalid
	}
}

func (s *ReviewArtifactStore) Retain(ctx context.Context, g codehost.ProposalGeneration, kind string, body []byte) (codehost.ReviewArtifact, error) {
	scope, err := s.generation(ctx, g, true)
	if err != nil {
		return codehost.ReviewArtifact{}, err
	}
	if len(body) == 0 || len(body) > codehost.MaxReviewArtifactBytes {
		return codehost.ReviewArtifact{}, codehost.ErrReviewInvalid
	}
	digest := fmt.Sprintf("sha256:%x", sha256.Sum256(body))
	ref, file, err := artifactLocation(g, kind, digest)
	if err != nil {
		return codehost.ReviewArtifact{}, err
	}
	var evidence codehost.ArtifactRef
	if kind == codehost.TaskRequest || kind == codehost.CandidateRequest {
		var proofKind string
		evidence, proofKind, err = requestEvidence(g, kind, body)
		if err != nil {
			return codehost.ReviewArtifact{}, err
		}
		if _, err := s.ReadRetained(ctx, g, proofKind, evidence); err != nil {
			return codehost.ReviewArtifact{}, err
		}
	}
	root := s.proposals.options.Git.Root
	if _, err := proposalGit(ctx, root, nil, nil, "fetch", "--no-tags", "--no-write-fetch-head", "--no-recurse-submodules", scope.Origin, g.OID); err != nil {
		return codehost.ReviewArtifact{}, err
	}
	oid, err := prepareReviewArtifact(ctx, root, g.OID, kind, file, body)
	if err != nil {
		return codehost.ReviewArtifact{}, err
	}
	identity := codehost.ArtifactRef{Ref: ref, OID: oid, Digest: digest}
	refs, err := proposalRefs(ctx, scope.Origin, ref)
	if err != nil {
		return codehost.ReviewArtifact{}, err
	}
	if existing := refs[ref]; existing != "" {
		if existing != oid {
			return codehost.ReviewArtifact{}, fmt.Errorf("%w: immutable review artifact conflicts", codehost.ErrReviewInvalid)
		}
		if _, err := s.generation(ctx, g, true); err != nil {
			return codehost.ReviewArtifact{}, err
		}
		return s.ReadRetained(ctx, g, kind, identity)
	}
	localRoot, err := filepath.Abs(root)
	if err != nil {
		return codehost.ReviewArtifact{}, err
	}
	if _, err := proposalGit(ctx, scope.Origin, nil, nil, "fetch", "--no-tags", "--no-write-fetch-head", "--no-recurse-submodules", localRoot, oid); err != nil {
		return codehost.ReviewArtifact{}, err
	}
	if _, err := s.generation(ctx, g, true); err != nil {
		return codehost.ReviewArtifact{}, err
	}
	p := g.Proposal
	transaction := fmt.Sprintf("start\nverify %s%s %s\nverify %s %s\nverify %s %s\n", proposalPrefix, p.ID, g.OID, p.Branch, p.Head, p.TargetRef, p.TargetOID)
	if p.SourceRef != "" {
		transaction += fmt.Sprintf("verify %s %s\n", p.SourceRef, p.Head)
	}
	if evidence.Ref != "" {
		transaction += fmt.Sprintf("verify %s %s\n", evidence.Ref, evidence.OID)
	}
	transaction += fmt.Sprintf("create %s %s\nprepare\ncommit\n", ref, oid)
	commit := s.commit
	if commit == nil {
		commit = func(ctx context.Context, origin string, body []byte) error {
			_, err := proposalGit(ctx, origin, body, nil, "update-ref", "--stdin")
			return err
		}
	}
	if err := commit(ctx, scope.Origin, []byte(transaction)); err != nil {
		return codehost.ReviewArtifact{}, fmt.Errorf("%w: retain review artifact: %w", codehost.ErrProposalConflict, err)
	}
	if _, err := s.generation(ctx, g, true); err != nil {
		return codehost.ReviewArtifact{}, err
	}
	return s.ReadRetained(ctx, g, kind, identity)
}

func prepareReviewArtifact(ctx context.Context, root, parent, kind, file string, body []byte) (string, error) {
	blob, err := proposalGit(ctx, root, body, nil, "hash-object", "-w", "--stdin")
	if err != nil {
		return "", err
	}
	tree, err := proposalGit(ctx, root, []byte("100644 blob "+strings.TrimSpace(string(blob))+"\t"+file+"\n"), nil, "mktree")
	if err != nil {
		return "", err
	}
	env := []string{"GIT_AUTHOR_NAME=Pyxis retained review", "GIT_COMMITTER_NAME=Pyxis retained review", "GIT_AUTHOR_EMAIL=local-review@pyxis.invalid", "GIT_COMMITTER_EMAIL=local-review@pyxis.invalid", "GIT_AUTHOR_DATE=2000-01-01T00:00:00Z", "GIT_COMMITTER_DATE=2000-01-01T00:00:00Z"}
	oid, err := proposalGit(ctx, root, []byte("Pyxis retained "+kind+"\n"), env, "commit-tree", strings.TrimSpace(string(tree)), "-p", parent)
	return strings.TrimSpace(string(oid)), err
}

func (s *ReviewArtifactStore) ReadRequest(ctx context.Context, g codehost.ProposalGeneration) (codehost.ReviewArtifact, error) {
	scope, err := s.generation(ctx, g, true)
	if err != nil {
		return codehost.ReviewArtifact{}, err
	}
	ref := "refs/pyxis/review-requests/" + g.Proposal.ID + "/" + g.OID
	refs, err := proposalRefs(ctx, scope.Origin, ref)
	if err != nil {
		return codehost.ReviewArtifact{}, err
	}
	if refs[ref] == "" {
		return codehost.ReviewArtifact{}, codehost.ErrReviewArtifactNotFound
	}
	kind := codehost.TaskRequest
	if g.Proposal.Task == "" {
		kind = codehost.CandidateRequest
	}
	return s.read(ctx, scope.Origin, g, kind, codehost.ArtifactRef{Ref: ref, OID: refs[ref]})
}

func (s *ReviewArtifactStore) ReadRetained(ctx context.Context, g codehost.ProposalGeneration, kind string, expected codehost.ArtifactRef) (codehost.ReviewArtifact, error) {
	scope, err := s.generation(ctx, g, false)
	if err != nil {
		return codehost.ReviewArtifact{}, err
	}
	if err := expected.Validate(); err != nil {
		return codehost.ReviewArtifact{}, err
	}
	return s.read(ctx, scope.Origin, g, kind, expected)
}

func (s *ReviewArtifactStore) read(ctx context.Context, origin string, g codehost.ProposalGeneration, kind string, expected codehost.ArtifactRef) (codehost.ReviewArtifact, error) {
	ref, file, err := artifactLocation(g, kind, expected.Digest)
	if err != nil || ref != expected.Ref || !objectID(expected.OID) {
		return codehost.ReviewArtifact{}, codehost.ErrReviewInvalid
	}
	refs, err := proposalRefs(ctx, origin, ref)
	if err != nil {
		return codehost.ReviewArtifact{}, err
	}
	if refs[ref] == "" {
		return codehost.ReviewArtifact{}, codehost.ErrReviewArtifactNotFound
	}
	if refs[ref] != expected.OID {
		return codehost.ReviewArtifact{}, codehost.ErrReviewInvalid
	}
	size, err := proposalGit(ctx, origin, nil, nil, "cat-file", "-s", expected.OID+":"+file)
	if err != nil {
		return codehost.ReviewArtifact{}, errors.Join(codehost.ErrReviewInvalid, err)
	}
	n, err := strconv.Atoi(strings.TrimSpace(string(size)))
	if err != nil || n > codehost.MaxReviewArtifactBytes {
		return codehost.ReviewArtifact{}, codehost.ErrReviewInvalid
	}
	body, err := proposalGit(ctx, origin, nil, nil, "show", expected.OID+":"+file)
	if err != nil || len(body) != n {
		return codehost.ReviewArtifact{}, errors.Join(codehost.ErrReviewInvalid, err)
	}
	digest := fmt.Sprintf("sha256:%x", sha256.Sum256(body))
	if expected.Digest != "" && expected.Digest != digest {
		return codehost.ReviewArtifact{}, codehost.ErrReviewInvalid
	}
	parents, err := proposalGit(ctx, origin, nil, nil, "rev-list", "--parents", "-n", "1", expected.OID)
	if err != nil || !slices.Equal(strings.Fields(string(parents)), []string{expected.OID, g.OID}) {
		return codehost.ReviewArtifact{}, errors.Join(codehost.ErrReviewInvalid, err)
	}
	files, err := proposalGit(ctx, origin, nil, nil, "ls-tree", "--name-only", expected.OID)
	if err != nil || !bytes.Equal(files, []byte(file+"\n")) {
		return codehost.ReviewArtifact{}, errors.Join(codehost.ErrReviewInvalid, err)
	}
	if kind == codehost.TaskRequest || kind == codehost.CandidateRequest {
		if _, _, err := requestEvidence(g, kind, body); err != nil {
			return codehost.ReviewArtifact{}, err
		}
	}
	expected.Digest = digest
	return codehost.ReviewArtifact{Identity: expected, Kind: kind, Body: body, Trust: codehost.Trust{SharedAccount: true}}, nil
}

func requestEvidence(g codehost.ProposalGeneration, kind string, body []byte) (codehost.ArtifactRef, string, error) {
	if kind == codehost.CandidateRequest {
		request, err := codehost.ParseCandidateRequest(body)
		if err == nil {
			err = request.Matches(g)
		}
		return request.Binding.Evidence, codehost.CandidateEvidence, err
	}
	request, err := codehost.ParseTaskRequest(body)
	if err == nil {
		err = request.Matches(g)
	}
	return request.Binding.Evidence, codehost.TaskAggregate, err
}
