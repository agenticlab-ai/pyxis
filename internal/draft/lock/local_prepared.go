package lock

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/codehost"
	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/draft/compile"
	"github.com/agenticlab-ai/pyxis/internal/draft/lifecycle"
	"github.com/agenticlab-ai/pyxis/internal/draft/readiness"
	"github.com/agenticlab-ai/pyxis/internal/memory/gitsrc"
	"github.com/agenticlab-ai/pyxis/internal/memory/record"
	"github.com/agenticlab-ai/pyxis/internal/memory/resolve"
	"github.com/agenticlab-ai/pyxis/internal/memory/store"
)

type preparedLocalReview struct {
	SourceHead string
	Compiled   compile.Result
	Sources    readiness.Sources
	Verdict    readiness.Verdict
	Documents  map[string]string
}

// inspectPreparedLocal shares source/readiness and literal-text verification
// between packet resumption and landing. It writes no request or receipt.
func inspectPreparedLocal(ctx context.Context, root string, cfg config.Config, p codehost.Proposal, version int, run lifecycle.RunCommand, local LocalPreparation) (out preparedLocalReview, err error) {
	if p.ReviewRequest == nil || p.ReviewRequest.Validate() != nil {
		return out, codehost.ErrReviewInvalid
	}
	if _, err := run(ctx, root, nil, "git", "fetch", "--no-tags", "--no-write-fetch-head", cfg.Remote, p.Head); err != nil {
		return out, err
	}
	parents, err := command(ctx, run, root, nil, "git", "rev-list", "--parents", "-n", "1", p.Head)
	if err != nil {
		return out, err
	}
	fields := strings.Fields(parents)
	if len(fields) != 2 || fields[0] != p.Head {
		return out, fmt.Errorf("prepared draft must have one source parent")
	}
	out.SourceHead = fields[1]
	if _, err := run(ctx, root, nil, "git", "merge-base", "--is-ancestor", p.TargetOID, out.SourceHead); err != nil {
		return out, fmt.Errorf("prepared source is not rebased on its reviewed target: %w", err)
	}
	if err := verifySourceStamps(ctx, root, p.TargetOID, out.SourceHead, run); err != nil {
		return out, err
	}
	repo, err := gitsrc.Open(ctx, root)
	if err != nil {
		return out, err
	}
	out.Compiled, err = compile.CompileBranch(ctx, repo, p.TargetOID, out.SourceHead, cfg.Drafts, p.Feature)
	if err != nil {
		return out, fmt.Errorf("recompile prepared source: %w", err)
	}
	if out.Compiled.Digest != p.ReviewRequest.Digest {
		return out, fmt.Errorf("prepared source digest changed; prepare and review again")
	}
	out.Sources, err = readiness.LoadSources(ctx, repo, p.TargetOID, cfg)
	if err != nil {
		return out, err
	}
	out.Verdict = readiness.Judge(out.Compiled, out.Sources)
	latest, err := latestLockedVersion(ctx, root, cfg.Remote, p.Feature, run)
	if err != nil {
		return out, err
	}
	if latest != version-1 {
		return out, fmt.Errorf("binding lock version changed; prepare and review again")
	}
	if version > 1 {
		locked, err := local.Locked(ctx, root, cfg.Remote, p.Feature, version-1, run)
		if err != nil {
			return out, err
		}
		unmet, err := guardReopenedWithReader(ctx, root, cfg.Remote, repo, run, p.TargetOID, out.SourceHead, cfg.Drafts, p.Feature, locked, out.Compiled, local.Locked)
		if err != nil {
			return out, err
		}
		out.Verdict.Unmet = append(out.Verdict.Unmet, unmet...)
		out.Verdict.Sort()
	}
	if !out.Verdict.Ready {
		return out, nil // The caller records the readiness-specific refusal.
	}
	if _, err := contractAt(ctx, root, p.Feature, version, p.Head, p.ReviewRequest.Digest); err != nil {
		return out, err
	}
	set, err := store.Load(ctx, gitsrc.TreeSource{Repo: repo, Ref: p.Head, Prefix: store.Dir})
	if err != nil {
		return out, err
	}
	projection := resolve.New(set)
	out.Documents = map[string]string{}
	for _, entry := range out.Compiled.Records {
		if entry.Draft.Kind != record.Document {
			continue
		}
		document := projection.Head(entry.Draft.Subject, entry.Draft.Family)
		if document == nil {
			return out, fmt.Errorf("prepared memory lacks Document %s#%s", entry.Draft.Subject, entry.Draft.Family)
		}
		setDocumentPath(out.Documents, p.Feature, document.Subject, document.Family, filepath.ToSlash(filepath.Join(store.Dir, document.BodyPath())))
	}
	return out, nil
}

func resumePreparedLocal(ctx context.Context, root string, cfg config.Config, generation codehost.ProposalGeneration, version int, oldHead, branch string, run lifecycle.RunCommand, local LocalPreparation) (out PrepareResult, err error) {
	p := generation.Proposal
	prepared, err := inspectPreparedLocal(ctx, root, cfg, p, version, run, local)
	if err != nil {
		return out, err
	}
	out.Branch, out.Head, out.Proposal, out.Verdict = branch, p.Head, &generation, prepared.Verdict
	if !out.Verdict.Ready {
		return out, fmt.Errorf("retained preparation is no longer ready: %+v", out.Verdict.Unmet)
	}
	if oldHead != p.Head && oldHead != prepared.SourceHead {
		return out, fmt.Errorf("checkout differs from retained preparation and its source; reconcile before resuming")
	}
	other, err := otherChanges(ctx, root, p.TargetOID, p.Head, p.Feature, cfg.Plans, run)
	if err != nil {
		return out, err
	}
	out.Request, err = localRequestWith(prepared.Compiled, prepared.Sources, out.Verdict, p.Head, prepared.Documents, other)
	if err != nil {
		return out, err
	}
	if err := checkPreparedLocalOrigin(ctx, generation, local); err != nil {
		return out, err
	}
	if oldHead != p.Head {
		if err := alignPreparedLocal(ctx, root, generation, oldHead, branch, run, local); err != nil {
			return out, err
		}
	}
	if err := checkPreparedLocalOrigin(ctx, generation, local); err != nil {
		return out, err
	}
	return out, nil
}

// Origin reads surround the destructive alignment. If the published packet
// moves during reset, restore the original clean source only after confirming
// that nobody edited or switched the newly aligned checkout.
func alignPreparedLocal(ctx context.Context, root string, generation codehost.ProposalGeneration, oldHead, branch string, run lifecycle.RunCommand, local LocalPreparation) error {
	if err := checkPreparationCheckout(ctx, root, oldHead, branch, run); err != nil {
		return err
	}
	if err := checkPreparedLocalOrigin(ctx, generation, local); err != nil {
		return err
	}
	if _, err := run(ctx, root, nil, "git", "reset", "--hard", generation.Proposal.Head); err != nil {
		return fmt.Errorf("align prepared working tree: %w", err)
	}
	if err := checkPreparedLocalOrigin(ctx, generation, local); err != nil {
		if checkoutErr := checkPreparationCheckout(ctx, root, generation.Proposal.Head, branch, run); checkoutErr != nil {
			return errors.Join(err, checkoutErr)
		}
		if _, restoreErr := run(ctx, root, nil, "git", "reset", "--hard", oldHead); restoreErr != nil {
			return errors.Join(err, fmt.Errorf("restore source checkout after origin moved: %w", restoreErr))
		}
		return err
	}
	return nil
}

func checkPreparedLocalOrigin(ctx context.Context, generation codehost.ProposalGeneration, local LocalPreparation) error {
	p := generation.Proposal
	current, err := local.Proposals.Read(ctx, p.ID)
	if err != nil {
		return err
	}
	mainline, err := local.Proposals.Mainline(ctx)
	if err != nil {
		return err
	}
	if current.OID != generation.OID || mainline.MainlineRef != p.TargetRef || mainline.MainlineOID != p.TargetOID {
		return codehost.ErrProposalConflict
	}
	return nil
}

func checkPreparationCheckout(ctx context.Context, root, expectedHead, expectedBranch string, run lifecycle.RunCommand) error {
	head, headErr := command(ctx, run, root, nil, "git", "rev-parse", "HEAD")
	branch, branchErr := command(ctx, run, root, nil, "git", "branch", "--show-current")
	dirty, statusErr := command(ctx, run, root, nil, "git", "status", "--porcelain")
	if headErr != nil || branchErr != nil || statusErr != nil || head != expectedHead || branch != expectedBranch || dirty != "" {
		return errors.Join(headErr, branchErr, statusErr, fmt.Errorf("checkout changed during preparation; reconcile before retrying"))
	}
	return nil
}
