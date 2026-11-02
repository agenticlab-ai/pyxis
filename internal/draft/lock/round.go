package lock

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/codehost"
	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/draft/compile"
	"github.com/agenticlab-ai/pyxis/internal/draft/lifecycle"
	"github.com/agenticlab-ai/pyxis/internal/memory/gitsrc"
)

type RoundResult struct {
	Head   string
	Digest string
}

// Round reverts the latest preparation commit in a temporary worktree. The
// draft branch changes only after the restored files reproduce its digest.
func Round(ctx context.Context, root string, cfg config.Config, feature string, run lifecycle.RunCommand) (out RoundResult, err error) {
	return round(ctx, root, cfg, feature, run, nil)
}

// RoundWithProposals restores the reviewed working bytes while atomically
// advancing the local proposal and clearing its old review request.
func RoundWithProposals(ctx context.Context, root string, cfg config.Config, feature string, run lifecycle.RunCommand, proposals codehost.DraftProposals) (RoundResult, error) {
	if proposals == nil {
		return RoundResult{}, fmt.Errorf("local round requires a proposal store")
	}
	return round(ctx, root, cfg, feature, run, proposals)
}

func round(ctx context.Context, root string, cfg config.Config, feature string, run lifecycle.RunCommand, proposals codehost.DraftProposals) (out RoundResult, err error) {
	if !featureName.MatchString(feature) || run == nil {
		return out, fmt.Errorf("resume round: a valid feature and process adapter are required")
	}
	branch := cfg.Draft.BranchPrefix + feature
	mainline := cfg.MainlineRef()
	var generation codehost.ProposalGeneration
	var readiness codehost.Readiness
	if proposals != nil {
		readiness, err = proposals.Mainline(ctx)
		if err != nil {
			return out, err
		}
		mainline = readiness.MainlineOID
		all, listErr := proposals.List(ctx)
		if listErr != nil {
			return out, listErr
		}
		for _, g := range all {
			if g.Proposal.Kind == "draft" && g.Proposal.Feature == feature && (g.Proposal.State == "prepared" || g.Proposal.State == "open") {
				if generation.OID != "" {
					return out, fmt.Errorf("multiple prepared drafts for %s", feature)
				}
				generation, branch = g, strings.TrimPrefix(g.Proposal.Branch, "refs/heads/")
			}
		}
		if generation.OID == "" {
			return out, fmt.Errorf("%s has no prepared proposal", feature)
		}
		raw := strings.TrimPrefix(generation.Proposal.ID, "draft-"+feature+"-v")
		version, parseErr := strconv.Atoi(raw)
		if parseErr != nil || version < 1 || strconv.Itoa(version) != raw {
			return out, fmt.Errorf("local round has no canonical draft version")
		}
		defer func() {
			if err != nil {
				err = recordStepFailure(ctx, root, cfg, feature, version, out.Digest, "draft-round", err)
			}
		}()
	}
	checkedOut, err := command(ctx, run, root, nil, "git", "branch", "--show-current")
	if err != nil {
		return out, fmt.Errorf("inspect current branch: %w", err)
	}
	if checkedOut != branch {
		return out, fmt.Errorf("resume round on %s, not %s", branch, checkedOut)
	}
	dirty, err := command(ctx, run, root, nil, "git", "status", "--porcelain")
	if err != nil {
		return out, fmt.Errorf("inspect working tree: %w", err)
	}
	if dirty != "" {
		return out, fmt.Errorf("resume round requires a clean working tree")
	}
	message, err := command(ctx, run, root, nil, "git", "show", "-s", "--format=%B", "HEAD")
	if err != nil {
		return out, fmt.Errorf("read preparation commit: %w", err)
	}
	lines := strings.Split(message, "\n")
	if lines[0] != "docs(draft): prepare lock "+feature {
		return out, fmt.Errorf("latest commit is not the preparation of %s", feature)
	}
	for _, line := range lines[1:] {
		if value, ok := strings.CutPrefix(line, "Draft-Digest: "); ok {
			out.Digest = strings.TrimSpace(value)
		}
	}
	if !strings.HasPrefix(out.Digest, "sha256:") {
		return out, fmt.Errorf("preparation commit has no draft digest")
	}
	oldHead, err := command(ctx, run, root, nil, "git", "rev-parse", "HEAD")
	if err != nil {
		return out, fmt.Errorf("resolve prepared head: %w", err)
	}
	if proposals != nil && generation.Proposal.State == "open" {
		p := generation.Proposal
		if p.PreviousOID == "" {
			return out, fmt.Errorf("open proposal has no prior generation to recover")
		}
		if _, err := run(ctx, root, nil, "git", "fetch", "--no-tags", "--no-write-fetch-head", readiness.Origin, p.Head); err != nil {
			return out, fmt.Errorf("fetch published round: %w", err)
		}
		parents, err := command(ctx, run, root, nil, "git", "rev-list", "--parents", "-n", "1", p.Head)
		if err != nil || parents != p.Head+" "+oldHead {
			return out, errors.Join(err, fmt.Errorf("published round does not directly restore the local preparation"))
		}
		repo, err := gitsrc.Open(ctx, root)
		if err != nil {
			return out, err
		}
		base, err := command(ctx, run, root, nil, "git", "merge-base", p.Head, p.TargetOID)
		if err != nil {
			return out, err
		}
		result, err := compile.CompileBranch(ctx, repo, base, p.Head, cfg.Drafts, feature)
		if err != nil || result.Digest != out.Digest {
			return out, errors.Join(err, fmt.Errorf("published round differs from the local preparation digest"))
		}
		out.Head = p.Head
		if _, err := run(ctx, root, nil, "git", "reset", "--hard", out.Head); err != nil {
			return out, fmt.Errorf("align published round: %w", err)
		}
		return out, nil
	}
	if proposals != nil && (oldHead != generation.Proposal.Head || generation.Proposal.ReviewRequest == nil || generation.Proposal.ReviewRequest.Digest != out.Digest) {
		return out, fmt.Errorf("prepared head or digest differs from retained proposal")
	}
	remoteHead, err := command(ctx, run, root, nil, "git", "ls-remote", cfg.Remote, "refs/heads/"+branch)
	if err != nil {
		return out, fmt.Errorf("inspect remote draft branch: %w", err)
	}
	remoteOID, _, ok := strings.Cut(remoteHead, "\t")
	if !ok || remoteOID != oldHead {
		return out, fmt.Errorf("remote draft branch moved since preparation")
	}
	var agentEnv []string
	if proposals == nil {
		if cfg.Draft.AgentLogin == "" || cfg.Draft.AgentTokenEnv == "" {
			return out, fmt.Errorf("resume round: configure the agent identity")
		}
		token := os.Getenv(cfg.Draft.AgentTokenEnv)
		if token == "" {
			return out, fmt.Errorf("resume round: %s is not set", cfg.Draft.AgentTokenEnv)
		}
		agentEnv = []string{"GH_TOKEN=" + token, "GIT_TERMINAL_PROMPT=0"}
		identity, err := command(ctx, run, root, agentEnv, "gh", "api", "user", "--jq", ".login")
		if err != nil {
			return out, fmt.Errorf("verify agent identity: %w", err)
		}
		if !strings.EqualFold(identity, cfg.Draft.AgentLogin) {
			return out, fmt.Errorf("authenticated account %q differs from configured draft agent %q", identity, cfg.Draft.AgentLogin)
		}
	}
	tmp, err := os.MkdirTemp("", "pyxis-draft-round-*")
	if err != nil {
		return out, fmt.Errorf("create round worktree: %w", err)
	}
	defer func() {
		if _, cleanupErr := run(ctx, root, nil, "git", "worktree", "remove", "--force", tmp); cleanupErr != nil {
			err = errors.Join(err, fmt.Errorf("remove round worktree: %w", cleanupErr))
		}
	}()
	if _, err := run(ctx, root, nil, "git", "worktree", "add", "--detach", tmp, oldHead); err != nil {
		return out, fmt.Errorf("create round worktree: %w", err)
	}
	if _, err := run(ctx, tmp, nil, "git", "revert", "--no-edit", "HEAD"); err != nil {
		return out, fmt.Errorf("revert lock preparation: %w", err)
	}
	repo, err := gitsrc.Open(ctx, tmp)
	if err != nil {
		return out, fmt.Errorf("open restored draft: %w", err)
	}
	out.Head, err = repo.Head(ctx)
	if err != nil {
		return out, fmt.Errorf("resolve restored draft: %w", err)
	}
	base, err := command(ctx, run, tmp, nil, "git", "merge-base", "HEAD", mainline)
	if err != nil {
		return out, fmt.Errorf("resolve draft mainline: %w", err)
	}
	result, err := compile.CompileBranch(ctx, repo, base, out.Head, cfg.Drafts, feature)
	if err != nil {
		return out, fmt.Errorf("compile restored draft: %w", err)
	}
	if result.Digest != out.Digest {
		return out, fmt.Errorf("restored draft digest %s differs from prepared %s", result.Digest, out.Digest)
	}
	if proposals != nil {
		p := generation.Proposal
		_, err := proposals.Publish(ctx, codehost.ProposalChange{ID: p.ID, Feature: p.Feature, Branch: p.Branch, Head: out.Head, State: "open", ExpectedProposalOID: generation.OID, ExpectedBranchOID: oldHead, ExpectedTargetOID: readiness.MainlineOID})
		if err != nil {
			return out, fmt.Errorf("publish restored round: %w", err)
		}
	} else {
		repoURL, err := command(ctx, run, tmp, agentEnv, "gh", "repo", "view", "--json", "url", "--jq", ".url")
		if err != nil {
			return out, fmt.Errorf("resolve code-host repository: %w", err)
		}
		url, err := pushURL(repoURL)
		if err != nil {
			return out, err
		}
		if _, err := run(ctx, tmp, agentEnv, "git", "-c", "credential.helper=", "-c", "credential.helper=!gh auth git-credential", "push", "--force-with-lease=refs/heads/"+branch+":"+oldHead, url, out.Head+":refs/heads/"+branch); err != nil {
			return out, fmt.Errorf("push restored round: %w", err)
		}
		number, err := command(ctx, run, tmp, agentEnv, "gh", "pr", "view", branch, "--json", "number", "--jq", ".number")
		if err != nil {
			return out, fmt.Errorf("find draft pull request: %w", err)
		}
		pr, err := strconv.Atoi(number)
		if err != nil || pr < 1 {
			return out, fmt.Errorf("invalid draft pull request number %q", number)
		}
		if _, err := run(ctx, tmp, agentEnv, "gh", "pr", "ready", "--undo", number); err != nil {
			return out, fmt.Errorf("return PR #%d to drafting: %w", pr, err)
		}
		if _, err := run(ctx, tmp, agentEnv, "gh", "pr", "edit", number, "--body", "Draft round resumed for "+feature+". The preparation commit was reverted; PRD and tech spec working files are restored."); err != nil {
			return out, fmt.Errorf("update draft PR #%d: %w", pr, err)
		}
	}
	if _, err := run(ctx, root, nil, "git", "reset", "--hard", out.Head); err != nil {
		return out, fmt.Errorf("align restored working tree: %w", err)
	}
	return out, nil
}
