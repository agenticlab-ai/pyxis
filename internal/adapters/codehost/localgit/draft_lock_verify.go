package localgit

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"
	"strconv"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/codehost"
)

// VerifyDraftLock reads retained origin evidence without the review worktree.
// Descendant mainline commits are allowed; the original landing must remain
// on that configured mainline with its exact reviewed tree and base parent.
func (s *DraftLandingStore) VerifyDraftLock(ctx context.Context, feature string, version int) (codehost.LocalDraftLock, error) {
	if !proposalID(feature) || version < 1 {
		return codehost.LocalDraftLock{}, codehost.ErrReviewInvalid
	}
	proposals := NewProposals(ProposalOptions{Git: s.options, Kind: "draft"})
	r, err := proposals.inspect(ctx)
	if err != nil {
		return codehost.LocalDraftLock{}, err
	}
	ref := fmt.Sprintf("refs/tags/lock/%s/v%d", feature, version)
	lock, tagOID, err := readLocalLockTag(ctx, r.Origin, ref)
	if err != nil {
		return lock, err
	}
	b := lock.Review.Receipt.Binding
	if lock.Feature != feature || lock.Version != version || b.TargetRef != r.MainlineRef || b.ProposalID != fmt.Sprintf("draft-%s-v%d", feature, version) {
		return lock, codehost.ErrReviewInvalid
	}
	final, err := proposals.Read(ctx, b.ProposalID)
	if err != nil {
		return lock, err
	}
	p := final.Proposal
	if p.State != "locked" || p.DraftLock == nil || *p.DraftLock != lock {
		return lock, codehost.ErrReviewInvalid
	}
	prepared, err := proposals.read(ctx, r.Origin, b.ProposalID, b.ProposalOID)
	if err != nil {
		return lock, err
	}
	old := prepared.Proposal
	if old.State != "prepared" || old.ReviewRequest == nil || old.Sequence != b.Sequence || old.Branch != b.Branch || old.Head != b.Head || old.TargetRef != b.TargetRef || old.TargetOID != b.TargetOID || old.ReviewRequest.Nonce != b.Nonce || old.ReviewRequest.Digest != b.Digest || old.ReviewRequest.Operation != b.Operation {
		return lock, codehost.ErrReviewInvalid
	}
	receipt, err := NewReceipts(s.options).read(ctx, r.Origin, b)
	if err != nil {
		return lock, err
	}
	if receipt != lock.Review {
		return lock, codehost.ErrReviewInvalid
	}
	parents, err := proposalGit(ctx, r.Origin, nil, nil, "rev-list", "--parents", "-n", "1", lock.MergeCommit)
	if err != nil {
		return lock, err
	}
	if !slices.Equal(strings.Fields(string(parents)), []string{lock.MergeCommit, b.TargetOID}) {
		return lock, codehost.ErrReviewInvalid
	}
	landedTree, err := proposalGit(ctx, r.Origin, nil, nil, "rev-parse", lock.MergeCommit+"^{tree}")
	if err != nil {
		return lock, err
	}
	reviewedTree, err := proposalGit(ctx, r.Origin, nil, nil, "rev-parse", b.Head+"^{tree}")
	if err != nil {
		return lock, err
	}
	if !bytes.Equal(landedTree, reviewedTree) {
		return lock, codehost.ErrReviewInvalid
	}
	// Import the exact verified landing object for contract reads. The origin
	// tag is authoritative; a checkout's unrelated cached tag must neither be
	// overwritten nor prevent recovery of an already completed landing.
	if _, err := proposalGit(ctx, s.options.Root, nil, nil, "fetch", "--no-tags", "--no-write-fetch-head", "--no-recurse-submodules", r.Origin, lock.MergeCommit); err != nil {
		return lock, err
	}
	// Evidence reads and object import can outlive the first mainline view.
	// Certify inclusion against a fresh snapshot, allowing ordinary descendants.
	current, err := proposals.inspect(ctx)
	if err != nil {
		return lock, err
	}
	if current.Origin != r.Origin {
		return lock, fmt.Errorf("%w: configured origin changed during lock verification", codehost.ErrReviewInvalid)
	}
	// Hold the complete retained proof through the final inclusion check. A
	// terminal draft must also keep its source branch absent.
	err = withUnchangedRefs(ctx, current.Origin, []unchangedRef{
		{current.MainlineRef, current.MainlineOID},
		{ref, tagOID},
		{proposalPrefix + b.ProposalID, final.OID},
		{p.ArchiveRef, final.OID},
		{receiptRef(b), receipt.OID},
		{b.Branch, strings.Repeat("0", len(current.MainlineOID))},
	}, func() error {
		if _, err := proposalGit(ctx, current.Origin, nil, nil, "merge-base", "--is-ancestor", lock.MergeCommit, current.MainlineOID); err != nil {
			return fmt.Errorf("%w: recorded lock is absent from current mainline", codehost.ErrReviewInvalid)
		}
		return nil
	})
	if err != nil {
		return lock, fmt.Errorf("%w: retained lock evidence changed during verification: %w", codehost.ErrReviewInvalid, err)
	}
	return lock, nil
}

func readLocalLockTag(ctx context.Context, origin, ref string) (codehost.LocalDraftLock, string, error) {
	var lock codehost.LocalDraftLock
	rawOID, err := proposalGit(ctx, origin, nil, nil, "rev-parse", "--verify", ref)
	oid := strings.TrimSpace(string(rawOID))
	if err != nil || !objectID(oid) {
		return lock, oid, fmt.Errorf("%w: lock tag is missing or invalid", codehost.ErrReviewInvalid)
	}
	// Decode one immutable object even if the tag ref moves during its read.
	size, err := proposalGit(ctx, origin, nil, nil, "cat-file", "-s", oid)
	if err != nil {
		return lock, oid, fmt.Errorf("%w: lock tag is unreadable", codehost.ErrReviewInvalid)
	}
	n, err := strconv.Atoi(strings.TrimSpace(string(size)))
	if err != nil || n > 64<<10 {
		return lock, oid, codehost.ErrReviewInvalid
	}
	tag, err := proposalGit(ctx, origin, nil, nil, "cat-file", "-p", oid)
	if err != nil || len(tag) > 64<<10 {
		return lock, oid, codehost.ErrReviewInvalid
	}
	header, body, ok := bytes.Cut(tag, []byte("\n\n"))
	if !ok {
		return lock, oid, codehost.ErrReviewInvalid
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&lock); err != nil {
		return lock, oid, codehost.ErrReviewInvalid
	}
	if err := decoder.Decode(new(any)); !errors.Is(err, io.EOF) {
		return lock, oid, codehost.ErrReviewInvalid
	}
	canonical, _ := json.Marshal(lock)
	expectedHeader := "object " + lock.MergeCommit + "\ntype commit\ntag " + strings.TrimPrefix(ref, "refs/tags/") + "\ntagger Pyxis local lock <local-lock@pyxis.invalid> 946684800 +0000"
	if lock.Validate() != nil || string(header) != expectedHeader || !bytes.Equal(body, append(canonical, '\n')) {
		return lock, oid, codehost.ErrReviewInvalid
	}
	return lock, oid, nil
}
