package localgit

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/codehost"
)

type DraftLandingStore struct {
	options Options
	push    func(context.Context, string, ...string) error
}

func NewDraftLandings(options Options) *DraftLandingStore {
	return &DraftLandingStore{options: options}
}

var _ codehost.DraftLandings = (*DraftLandingStore)(nil)

// LandDraft is the operator adapter operation. The workflow must recheck the
// compiled contract and memory first; this adapter checks and leases Git proof.
func (s *DraftLandingStore) LandDraft(ctx context.Context, request codehost.DraftLandingRequest) (codehost.DraftLandingResult, error) {
	proposals := NewProposals(ProposalOptions{Git: s.options, Kind: "draft"})
	r, err := proposals.Mainline(ctx)
	if err != nil {
		return codehost.DraftLandingResult{}, err
	}
	g, err := proposals.Read(ctx, request.Binding.ProposalID)
	if err != nil {
		return codehost.DraftLandingResult{}, err
	}
	p := g.Proposal
	if p.State == "locked" && p.DraftLock != nil {
		lock, err := s.VerifyDraftLock(ctx, p.Feature, p.DraftLock.Version)
		if err == nil && (lock.Review.Receipt.Binding != request.Binding || lock.Version != request.Version || lock.ProposalsSinceOpened != request.ProposalsSinceOpened) {
			err = codehost.ErrReviewInvalid
		}
		return codehost.DraftLandingResult{Lock: lock}, err
	}
	if p.State != "prepared" || p.Sequence == math.MaxUint64 || request.Version < 1 || request.ProposalsSinceOpened < 1 ||
		p.ID != fmt.Sprintf("draft-%s-v%d", p.Feature, request.Version) || request.Binding.TargetRef != r.MainlineRef {
		return codehost.DraftLandingResult{}, codehost.ErrReviewInvalid
	}
	receipts := NewReceipts(s.options)
	review, err := receipts.Verify(ctx, request.Binding)
	if err != nil {
		return codehost.DraftLandingResult{}, err
	}
	latest, err := originLockVersion(ctx, r.Origin, p.Feature)
	if err != nil {
		return codehost.DraftLandingResult{}, err
	}
	if latest+1 != request.Version {
		return codehost.DraftLandingResult{}, invalidProposal("lock version changed; prepare and review the current version")
	}
	for _, oid := range []string{g.OID, p.Head, p.TargetOID, review.OID} {
		if _, err := proposalGit(ctx, s.options.Root, nil, nil, "fetch", "--no-tags", "--no-write-fetch-head", "--no-recurse-submodules", r.Origin, oid); err != nil {
			return codehost.DraftLandingResult{}, err
		}
	}
	tree, err := proposalGit(ctx, s.options.Root, nil, nil, "rev-parse", p.Head+"^{tree}")
	if err != nil {
		return codehost.DraftLandingResult{}, err
	}
	landing, err := proposalGit(ctx, s.options.Root, []byte("docs(draft): lock "+p.Feature+"\n\nDraft-Digest: "+request.Binding.Digest+"\n"), lockGitIdentity(), "commit-tree", strings.TrimSpace(string(tree)), "-p", p.TargetOID)
	if err != nil {
		return codehost.DraftLandingResult{}, err
	}
	lock := codehost.LocalDraftLock{SchemaVersion: codehost.DraftLockSchema, Feature: p.Feature, Version: request.Version,
		MergeCommit: strings.TrimSpace(string(landing)), Review: review, ProductApprover: review.Receipt.Reviewer,
		EngineeringApprover: review.Receipt.Reviewer, Lineless: review.Receipt.CaptureLine == "", ProposalsSinceOpened: request.ProposalsSinceOpened}
	if err := lock.Validate(); err != nil {
		return codehost.DraftLandingResult{}, err
	}
	tagRef := fmt.Sprintf("refs/tags/lock/%s/v%d", p.Feature, lock.Version)
	tag, err := prepareLockTag(ctx, s.options.Root, tagRef, lock)
	if err != nil {
		return codehost.DraftLandingResult{}, err
	}
	p.Sequence++
	p.PreviousOID, p.State, p.DraftLock = g.OID, "locked", &lock
	p.ArchiveRef = archiveRef(p)
	if err := validateProposal(ctx, s.options.Root, p); err != nil {
		return codehost.DraftLandingResult{}, err
	}
	finalOID, err := prepareProposal(ctx, s.options.Root, p)
	if err != nil {
		return codehost.DraftLandingResult{}, err
	}
	// Re-read after object preparation. The real branch deletion below is
	// leased; a no-op branch push would miss a move between this read and push.
	current, err := proposals.inspect(ctx)
	if err != nil {
		return codehost.DraftLandingResult{}, err
	}
	if current.Origin != r.Origin || current.MainlineOID != p.TargetOID {
		return codehost.DraftLandingResult{}, codehost.ErrProposalConflict
	}
	if verified, err := receipts.Verify(ctx, request.Binding); err != nil {
		return codehost.DraftLandingResult{}, err
	} else if verified != review {
		return codehost.DraftLandingResult{}, codehost.ErrReviewInvalid
	}
	args := []string{"push", "--atomic", "--no-recurse-submodules",
		"--force-with-lease=" + p.TargetRef + ":" + p.TargetOID,
		"--force-with-lease=" + proposalPrefix + p.ID + ":" + g.OID,
		"--force-with-lease=" + p.Branch + ":" + p.Head,
		"--force-with-lease=" + tagRef + ":", "--force-with-lease=" + p.ArchiveRef + ":",
		r.Origin, lock.MergeCommit + ":" + p.TargetRef, finalOID + ":" + proposalPrefix + p.ID,
		tag + ":" + tagRef, finalOID + ":" + p.ArchiveRef, ":" + p.Branch}
	push := s.push
	if push == nil {
		push = func(ctx context.Context, root string, args ...string) error {
			_, err := proposalGit(ctx, root, nil, nil, args...)
			return err
		}
	}
	// The receipt is unchanged by publication, so a no-op push lease would
	// not protect it. Hold its real Git ref lock through the atomic push.
	if err := withUnchangedRefs(ctx, r.Origin, []unchangedRef{{Ref: receiptRef(request.Binding), OID: review.OID}}, func() error {
		return push(ctx, s.options.Root, args...)
	}); err != nil {
		// A transport can lose the reply after success. Only the complete exact
		// retained result certifies that publication happened.
		if existing, verifyErr := s.VerifyDraftLock(ctx, p.Feature, lock.Version); verifyErr == nil && existing == lock {
			return codehost.DraftLandingResult{Lock: existing}, nil
		}
		if strings.Contains(err.Error(), "does not support --atomic push") {
			return codehost.DraftLandingResult{}, fmt.Errorf("%w: %v", codehost.ErrAtomicUnavailable, err)
		}
		return codehost.DraftLandingResult{}, fmt.Errorf("%w: atomic lock refused; prepare and review again: %v", codehost.ErrProposalConflict, err)
	}
	verified, err := s.VerifyDraftLock(ctx, p.Feature, lock.Version)
	return codehost.DraftLandingResult{Lock: verified, Created: true}, err
}

func lockGitIdentity() []string {
	return []string{"GIT_AUTHOR_NAME=Pyxis local lock", "GIT_COMMITTER_NAME=Pyxis local lock", "GIT_AUTHOR_EMAIL=local-lock@pyxis.invalid", "GIT_COMMITTER_EMAIL=local-lock@pyxis.invalid", "GIT_AUTHOR_DATE=2000-01-01T00:00:00Z", "GIT_COMMITTER_DATE=2000-01-01T00:00:00Z"}
}

func prepareLockTag(ctx context.Context, root, ref string, lock codehost.LocalDraftLock) (string, error) {
	body, err := json.Marshal(lock)
	if err != nil {
		return "", err
	}
	annotation := "object " + lock.MergeCommit + "\ntype commit\ntag " + strings.TrimPrefix(ref, "refs/tags/") + "\ntagger Pyxis local lock <local-lock@pyxis.invalid> 946684800 +0000\n\n"
	oid, err := proposalGit(ctx, root, append([]byte(annotation), append(body, '\n')...), nil, "mktag")
	return strings.TrimSpace(string(oid)), err
}

func originLockVersion(ctx context.Context, origin, feature string) (int, error) {
	if feature == "" || !proposalID(feature) {
		return 0, codehost.ErrReviewInvalid
	}
	refs, err := proposalGit(ctx, origin, nil, nil, "for-each-ref", "--format=%(refname) %(objecttype)", "refs/tags/lock/"+feature+"/")
	if err != nil {
		return 0, err
	}
	versions := map[int]bool{}
	latest := 0
	for line := range strings.SplitSeq(strings.TrimSpace(string(refs)), "\n") {
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) != 2 || fields[1] != "tag" {
			return 0, codehost.ErrReviewInvalid
		}
		raw := strings.TrimPrefix(fields[0], "refs/tags/lock/"+feature+"/v")
		version, err := strconv.Atoi(raw)
		if err != nil || version < 1 || strconv.Itoa(version) != raw || versions[version] {
			return 0, codehost.ErrReviewInvalid
		}
		versions[version], latest = true, max(latest, version)
	}
	for version := 1; version <= latest; version++ {
		if !versions[version] {
			return 0, codehost.ErrReviewInvalid
		}
	}
	return latest, nil
}
