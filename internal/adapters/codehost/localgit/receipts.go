package localgit

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/codehost"
)

const receiptPrefix = "refs/pyxis/receipts/"

func receiptRef(binding codehost.ReviewBinding) string {
	return receiptPrefix + binding.ProposalID + "/" + binding.Nonce
}

type ReceiptStore struct {
	options         Options
	tasks           *ProposalOptions
	commit          func(context.Context, string, []byte) error
	reviewValidator codehost.ReviewValidator
	requireReview   bool
	final           bool
}

func NewReceipts(options Options) *ReceiptStore { return &ReceiptStore{options: options} }

func taskReceiptStore(options ProposalOptions) *ReceiptStore {
	return &ReceiptStore{options: options.Git, tasks: &options}
}

func NewTaskReceipts(options ProposalOptions, validator codehost.ReviewValidator) *ReceiptStore {
	s := taskReceiptStore(options)
	s.reviewValidator, s.requireReview = validator, true
	return s
}

func candidateReceiptStore(options ProposalOptions) *ReceiptStore {
	return &ReceiptStore{options: options.Git, tasks: &options, final: true}
}

func NewCandidateReceipts(options ProposalOptions, validator codehost.ReviewValidator) *ReceiptStore {
	s := candidateReceiptStore(options)
	s.reviewValidator, s.requireReview = validator, true
	return s
}

var _ codehost.Receipts = (*ReceiptStore)(nil)

// Binding reads the request retained by the prepared generation. It never
// mints a nonce, advances the generation, or infers approval from a branch.
func (s *ReceiptStore) Binding(ctx context.Context, id string) (codehost.ReviewBinding, error) {
	_, binding, err := s.current(ctx, id)
	if err == nil {
		err = s.validateReview(ctx, binding)
	}
	return binding, err
}

func (s *ReceiptStore) validateReview(ctx context.Context, binding codehost.ReviewBinding) error {
	if !s.requireReview {
		return nil
	}
	if s.reviewValidator == nil {
		return codehost.ErrReviewInvalid
	}
	return s.reviewValidator.ValidateReview(ctx, binding)
}

func (s *ReceiptStore) current(ctx context.Context, id string) (codehost.Readiness, codehost.ReviewBinding, error) {
	options := ProposalOptions{Git: s.options, Kind: "draft"}
	if s.tasks != nil {
		options = *s.tasks
		scope, err := NewReviewArtifacts(options).Scope(ctx)
		if err != nil {
			return codehost.Readiness{}, codehost.ReviewBinding{}, err
		}
		if (scope.HeadRef != "") != s.final {
			return codehost.Readiness{}, codehost.ReviewBinding{}, codehost.ErrReviewInvalid
		}
	}
	store := NewProposals(options)
	r, err := store.inspect(ctx)
	if err != nil {
		return r, codehost.ReviewBinding{}, err
	}
	generation, err := store.Read(ctx, id)
	if err != nil {
		return r, codehost.ReviewBinding{}, err
	}
	p := generation.Proposal
	if s.tasks != nil {
		binding, err := s.taskBinding(ctx, options, generation)
		return r, binding, err
	}
	if p.Kind != "draft" || p.State != "prepared" || p.ReviewRequest == nil || p.ReviewRequest.Validate() != nil {
		return r, codehost.ReviewBinding{}, codehost.ErrReviewInvalid
	}
	if p.TargetRef != r.MainlineRef || p.TargetOID != r.MainlineOID {
		return r, codehost.ReviewBinding{}, fmt.Errorf("%w: mainline moved; prepare and review again", codehost.ErrProposalConflict)
	}
	request := p.ReviewRequest
	binding := codehost.ReviewBinding{SchemaVersion: codehost.ReviewBindingSchema, ProposalID: p.ID, Kind: p.Kind, ProposalOID: generation.OID, Sequence: p.Sequence,
		Operation: request.Operation, Branch: p.Branch, Head: p.Head, TargetRef: p.TargetRef,
		TargetOID: p.TargetOID, Digest: request.Digest, Nonce: request.Nonce}
	return r, binding, nil
}

func (s *ReceiptStore) taskBinding(ctx context.Context, options ProposalOptions, generation codehost.ProposalGeneration) (codehost.ReviewBinding, error) {
	artifacts := NewReviewArtifacts(options)
	retained, err := artifacts.ReadRequest(ctx, generation)
	if err != nil {
		return codehost.ReviewBinding{}, err
	}
	var binding codehost.ReviewBinding
	proofKind := codehost.TaskAggregate
	if s.final {
		request, parseErr := codehost.ParseCandidateRequest(retained.Body)
		if parseErr != nil {
			return codehost.ReviewBinding{}, parseErr
		}
		binding, proofKind = request.Binding, codehost.CandidateEvidence
	} else {
		request, parseErr := codehost.ParseTaskRequest(retained.Body)
		if parseErr != nil {
			return codehost.ReviewBinding{}, parseErr
		}
		binding = request.Binding
	}
	if _, err := artifacts.ReadRetained(ctx, generation, proofKind, binding.Evidence); err != nil {
		return codehost.ReviewBinding{}, err
	}
	binding.Request = retained.Identity
	return binding, binding.Validate()
}

func (s *ReceiptStore) expected(ctx context.Context, binding codehost.ReviewBinding) (codehost.Readiness, error) {
	if err := binding.Validate(); err != nil {
		return codehost.Readiness{}, err
	}
	if err := s.validateReview(ctx, binding); err != nil {
		return codehost.Readiness{}, err
	}
	r, current, err := s.current(ctx, binding.ProposalID)
	if err == nil && current != binding {
		err = fmt.Errorf("%w: request no longer matches prepared generation; prepare and review again", codehost.ErrReviewInvalid)
	}
	return r, err
}

func (s *ReceiptStore) Verify(ctx context.Context, binding codehost.ReviewBinding) (codehost.ReceiptRecord, error) {
	r, err := s.expected(ctx, binding)
	if err != nil {
		return codehost.ReceiptRecord{}, err
	}
	record, err := s.read(ctx, r.Origin, binding)
	if err != nil {
		return codehost.ReceiptRecord{}, err
	}
	// A read racing a revision or target movement is not current approval.
	if _, err := s.expected(ctx, binding); err != nil {
		return codehost.ReceiptRecord{}, err
	}
	return record, nil
}

func (s *ReceiptStore) WithReview(ctx context.Context, binding codehost.ReviewBinding, mutate func() error) error {
	lease, ok := s.reviewValidator.(codehost.ReviewLease)
	if !s.requireReview || s.tasks == nil || !ok || mutate == nil {
		return codehost.ErrReviewInvalid
	}
	return lease.WithReview(ctx, binding, mutate)
}

func (s *ReceiptStore) Record(ctx context.Context, binding codehost.ReviewBinding, reviewer, capture string) (codehost.ReceiptRecord, error) {
	r, err := s.expected(ctx, binding)
	if err != nil {
		return codehost.ReceiptRecord{}, err
	}
	receipt := codehost.ReviewReceipt{SchemaVersion: codehost.ReceiptSchema, Binding: binding, Reviewer: reviewer, CaptureLine: capture}
	if strings.TrimSpace(reviewer) == "" || len(reviewer) > 128 || strings.ContainsAny(reviewer, "\r\n\x00") || len(capture) > 8192 || strings.ContainsRune(capture, '\x00') {
		return codehost.ReceiptRecord{}, codehost.ErrReviewInvalid
	}
	existing, err := s.read(ctx, r.Origin, binding)
	if err == nil {
		if existing.Receipt != receipt {
			return codehost.ReceiptRecord{}, codehost.ErrReceiptConflict
		}
		return s.Verify(ctx, binding)
	}
	if !errors.Is(err, codehost.ErrReceiptNotFound) {
		return codehost.ReceiptRecord{}, err
	}
	if _, err := proposalGit(ctx, s.options.Root, nil, nil, "fetch", "--no-tags", "--no-write-fetch-head", "--no-recurse-submodules", r.Origin, binding.ProposalOID); err != nil {
		return codehost.ReceiptRecord{}, err
	}
	body, _ := json.Marshal(receipt)
	body = append(body, '\n')
	oid, err := prepareReceipt(ctx, s.options.Root, binding.ProposalOID, body)
	if err != nil {
		return codehost.ReceiptRecord{}, err
	}
	root, err := filepath.Abs(s.options.Root)
	if err != nil {
		return codehost.ReceiptRecord{}, err
	}
	// Transfer objects without advancing a ref. Publication then uses one
	// local Git transaction: all observed refs must still match, and create
	// imposes the receipt ref's zero-old-OID lease. No hook or author counts
	// as authentication in this shared-account protocol.
	if _, err := proposalGit(ctx, r.Origin, nil, nil, "fetch", "--no-tags", "--no-write-fetch-head", "--no-recurse-submodules", root, oid); err != nil {
		return codehost.ReceiptRecord{}, err
	}
	if _, err := s.expected(ctx, binding); err != nil {
		return codehost.ReceiptRecord{}, err
	}
	transaction := fmt.Sprintf("start\nverify %s%s %s\nverify %s %s\nverify %s %s\n",
		proposalPrefix, binding.ProposalID, binding.ProposalOID, binding.Branch, binding.Head,
		binding.TargetRef, binding.TargetOID)
	if binding.Kind == "implementation" {
		transaction += fmt.Sprintf("verify %s %s\nverify %s %s\n", binding.Evidence.Ref, binding.Evidence.OID, binding.Request.Ref, binding.Request.OID)
	}
	if s.final {
		transaction += fmt.Sprintf("verify %s %s\n", s.tasks.HeadRef, binding.Candidate.FeatureHead)
	}
	transaction += fmt.Sprintf("create %s %s\nprepare\ncommit\n", receiptRef(binding), oid)
	commit := s.commit
	if commit == nil {
		commit = func(ctx context.Context, origin string, input []byte) error {
			_, err := proposalGit(ctx, origin, input, nil, "update-ref", "--stdin")
			return err
		}
	}
	var recorded codehost.ReceiptRecord
	publish := func() error {
		if _, err := s.expected(ctx, binding); err != nil {
			return err
		}
		if err := commit(ctx, r.Origin, []byte(transaction)); err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return fmt.Errorf("%w: receipt transaction refused: %v", codehost.ErrReceiptConflict, err)
		}
		var err error
		recorded, err = s.Verify(ctx, binding)
		return err
	}
	if s.requireReview {
		err = s.WithReview(ctx, binding, publish)
	} else {
		err = publish()
	}
	return recorded, err
}

func prepareReceipt(ctx context.Context, root, parent string, body []byte) (string, error) {
	blob, err := proposalGit(ctx, root, body, nil, "hash-object", "-w", "--stdin")
	if err != nil {
		return "", err
	}
	tree, err := proposalGit(ctx, root, []byte("100644 blob "+strings.TrimSpace(string(blob))+"\treceipt.json\n"), nil, "mktree")
	if err != nil {
		return "", err
	}
	env := []string{"GIT_AUTHOR_NAME=Pyxis declared review", "GIT_COMMITTER_NAME=Pyxis declared review", "GIT_AUTHOR_EMAIL=local-review@pyxis.invalid", "GIT_COMMITTER_EMAIL=local-review@pyxis.invalid", "GIT_AUTHOR_DATE=2000-01-01T00:00:00Z", "GIT_COMMITTER_DATE=2000-01-01T00:00:00Z"}
	oid, err := proposalGit(ctx, root, []byte("Pyxis declared local review\n"), env, "commit-tree", strings.TrimSpace(string(tree)), "-p", parent)
	return strings.TrimSpace(string(oid)), err
}

func (s *ReceiptStore) read(ctx context.Context, origin string, binding codehost.ReviewBinding) (codehost.ReceiptRecord, error) {
	ref := receiptRef(binding)
	refs, err := proposalRefs(ctx, origin, ref)
	if err != nil {
		return codehost.ReceiptRecord{}, err
	}
	oid := refs[ref]
	if oid == "" {
		return codehost.ReceiptRecord{}, codehost.ErrReceiptNotFound
	}
	size, err := proposalGit(ctx, origin, nil, nil, "cat-file", "-s", oid+":receipt.json")
	if err != nil {
		return codehost.ReceiptRecord{}, codehost.ErrReviewInvalid
	}
	n, err := strconv.Atoi(strings.TrimSpace(string(size)))
	if err != nil || n > 64<<10 {
		return codehost.ReceiptRecord{}, codehost.ErrReviewInvalid
	}
	body, err := proposalGit(ctx, origin, nil, nil, "show", oid+":receipt.json")
	if err != nil || len(body) > 64<<10 {
		return codehost.ReceiptRecord{}, codehost.ErrReviewInvalid
	}
	var receipt codehost.ReviewReceipt
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&receipt); err != nil {
		return codehost.ReceiptRecord{}, codehost.ErrReviewInvalid
	}
	if err := decoder.Decode(new(any)); !errors.Is(err, io.EOF) {
		return codehost.ReceiptRecord{}, codehost.ErrReviewInvalid
	}
	canonical, _ := json.Marshal(receipt)
	if !bytes.Equal(body, append(canonical, '\n')) || receipt.SchemaVersion != codehost.ReceiptSchema || receipt.Binding != binding || strings.TrimSpace(receipt.Reviewer) == "" || len(receipt.Reviewer) > 128 || strings.ContainsAny(receipt.Reviewer, "\r\n\x00") || len(receipt.CaptureLine) > 8192 || strings.ContainsRune(receipt.CaptureLine, '\x00') {
		return codehost.ReceiptRecord{}, codehost.ErrReviewInvalid
	}
	parents, err := proposalGit(ctx, origin, nil, nil, "rev-list", "--parents", "-n", "1", oid)
	if err != nil {
		return codehost.ReceiptRecord{}, err
	}
	if !slices.Equal(strings.Fields(string(parents)), []string{oid, binding.ProposalOID}) {
		return codehost.ReceiptRecord{}, codehost.ErrReviewInvalid
	}
	digest := sha256.Sum256(body)
	return codehost.ReceiptRecord{OID: oid, Digest: "sha256:" + hex.EncodeToString(digest[:]), Receipt: receipt, Trust: codehost.Trust{SharedAccount: true}}, nil
}
