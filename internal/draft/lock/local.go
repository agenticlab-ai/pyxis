package lock

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/codehost"
	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/draft/lifecycle"
)

// LockLocal rechecks the saved prepared binding, compiler, readiness, and
// installed memory gate before asking the operator-only adapter to publish.
// It does not record a review or infer approval from an author identity.
func LockLocal(ctx context.Context, root string, cfg config.Config, binding codehost.ReviewBinding, run lifecycle.RunCommand, local LocalPreparation, reviews codehost.ReceiptReader, landings codehost.DraftLandings) (out codehost.DraftLandingResult, err error) {
	if root == "" || run == nil || local.Proposals == nil || local.Locked == nil || local.Memory == nil || reviews == nil || landings == nil {
		return out, fmt.Errorf("local lock requires repository, process, proposal, review, memory, and landing adapters")
	}
	g, err := local.Proposals.Read(ctx, binding.ProposalID)
	if err != nil {
		return out, err
	}
	p := g.Proposal
	if p.State == "locked" && p.DraftLock != nil {
		if p.DraftLock.Review.Receipt.Binding != binding {
			return out, codehost.ErrReviewInvalid
		}
		verified, err := landings.VerifyDraftLock(ctx, p.Feature, p.DraftLock.Version)
		if err != nil {
			return out, err
		}
		if verified != *p.DraftLock {
			return out, codehost.ErrReviewInvalid
		}
		if _, err := LoadLocalContract(ctx, root, p.Feature, verified.Version, landings); err != nil {
			return out, err
		}
		return codehost.DraftLandingResult{Lock: verified}, nil
	}
	if !featureName.MatchString(p.Feature) || p.State != "prepared" || g.OID != binding.ProposalOID {
		return out, codehost.ErrReviewInvalid
	}
	raw := strings.TrimPrefix(p.ID, "draft-"+p.Feature+"-v")
	version, err := strconv.Atoi(raw)
	if err != nil || version < 1 || strconv.Itoa(version) != raw {
		return out, codehost.ErrReviewInvalid
	}
	branch := "refs/heads/" + cfg.Draft.BranchPrefix + p.Feature
	if version > 1 {
		branch += "-v" + strconv.Itoa(version)
	}
	if p.Branch != branch {
		return out, codehost.ErrReviewInvalid
	}
	refused := false
	defer func() {
		if err != nil && !refused && !out.Created {
			err = recordStepFailure(ctx, root, cfg, p.Feature, version, binding.Digest, "lock", err)
		}
	}()
	r, err := local.Proposals.Mainline(ctx)
	if err != nil {
		return out, err
	}
	if r.MainlineOID != binding.TargetOID || r.MainlineRef != binding.TargetRef {
		return out, codehost.ErrProposalConflict
	}
	cfg.Remote = r.Origin
	if _, err := reviews.Verify(ctx, binding); err != nil {
		return out, err
	}
	prepared, err := inspectPreparedLocal(ctx, root, cfg, p, version, run, local)
	if err != nil {
		return out, err
	}
	verdict := prepared.Verdict
	if !verdict.Ready {
		refused = true
		cause := fmt.Errorf("current readiness refuses local lock: %+v", verdict.Unmet)
		return out, recordReadinessRefusal(ctx, root, cfg, p.Feature, version, verdict, cause)
	}
	tmp, err := os.MkdirTemp("", "pyxis-local-lock-*")
	if err != nil {
		return out, err
	}
	if _, err := run(ctx, root, nil, "git", "worktree", "add", "--detach", tmp, p.Head); err != nil {
		return out, err
	}
	defer func() {
		cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
		defer cancel()
		if _, cleanupErr := run(cleanupCtx, root, nil, "git", "worktree", "remove", "--force", tmp); cleanupErr != nil {
			err = errors.Join(err, cleanupErr)
		}
	}()
	if err := local.Memory(ctx, tmp, binding.TargetOID); err != nil {
		return out, fmt.Errorf("memory gate at reviewed tree: %w", err)
	}
	all, err := local.Proposals.List(ctx)
	if err != nil {
		return out, err
	}
	count := 0
	for _, generation := range all {
		if generation.Proposal.Kind == "draft" && generation.Proposal.Feature == p.Feature && generation.Proposal.Branch == p.Branch {
			count++
		}
	}
	return landings.LandDraft(ctx, codehost.DraftLandingRequest{Binding: binding, Version: version, ProposalsSinceOpened: count})
}

// LoadLocalContract requires the selected origin's complete retained lock
// verification before its literal Documents can seed another draft.
func LoadLocalContract(ctx context.Context, root, feature string, version int, reader codehost.DraftLockReader) (lifecycle.LockedContract, error) {
	if reader == nil {
		return lifecycle.LockedContract{}, fmt.Errorf("local lock requires a verifier")
	}
	verified, err := reader.VerifyDraftLock(ctx, feature, version)
	if err != nil {
		return lifecycle.LockedContract{}, err
	}
	if verified.Validate() != nil || verified.Feature != feature || verified.Version != version {
		return lifecycle.LockedContract{}, codehost.ErrReviewInvalid
	}
	return contractAt(ctx, root, feature, version, verified.MergeCommit, verified.Review.Receipt.Binding.Digest)
}
