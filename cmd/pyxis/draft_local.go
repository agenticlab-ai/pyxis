package main

import (
	"context"
	"fmt"
	"io"
	"strings"

	localcodehost "github.com/agenticlab-ai/pyxis/internal/adapters/codehost/localgit"
	draftshell "github.com/agenticlab-ai/pyxis/internal/adapters/draft/shell"
	"github.com/agenticlab-ai/pyxis/internal/codehost"
	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/draft/lifecycle"
	draftlock "github.com/agenticlab-ai/pyxis/internal/draft/lock"
)

var localDraftCommand = lifecycle.RunCommand(draftshell.RunLocalGit)

// Implemented local draft verbs bypass the full-workflow admission. Execution
// and watch keep their independent gates until their local adapters are ready.
func localDraftVerb(args []string, stdout, stderr io.Writer) (int, bool) {
	root, err := draftRepoRoot()
	if err != nil {
		return 0, false
	}
	cfg, defects := config.Load(root)
	if len(defects) > 0 {
		return 0, false
	}
	options, selected, err := localCodeHostOptions(root, cfg)
	if !selected && err == nil {
		return 0, false
	}
	if err != nil {
		terminalPrintf(stderr, "pyxis: draft: code host: %v\n", err)
		return 1, true
	}
	ctx := context.Background()
	proposals := localcodehost.NewProposals(localcodehost.ProposalOptions{Git: options, Kind: "draft"})
	r, err := proposals.Mainline(ctx)
	if err != nil {
		terminalPrintf(stderr, "pyxis: draft: code host: %v\n", err)
		return 1, true
	}
	// Pending events and lock reads use this already verified local destination.
	cfg.Remote = r.Origin
	options.Origin = r.Origin
	l := lifecycle.ProposalLifecycle{Root: root, Config: cfg, Store: proposals, Run: localDraftCommand, Locked: localBindingReader(options)}
	feature := ""
	if len(args) > 1 {
		feature = args[1]
	}
	var result lifecycle.ProposalResult
	switch args[0] {
	case "digest":
		head, digest, digestErr := draftDigest(ctx, root, cfg, feature)
		if digestErr != nil {
			err = digestErr
			break
		}
		terminalPrintf(stdout, "%s %s\n", head, digest)
		return 0, true
	case "prepare-lock":
		local := draftlock.LocalPreparation{Proposals: proposals, Locked: l.Locked, Memory: localDraftMemory}
		if tasks, readerErr := configuredTaskReader(root, cfg); readerErr != nil {
			err = readerErr
			break
		} else if tasks != nil {
			local.Unfinished = func(ctx context.Context) ([]string, error) {
				return lifecycle.UnfinishedTasksWithTracker(ctx, root, cfg, feature, r.MainlineOID, tasks, localDraftCommand)
			}
			if a, ok := tasks.(draftlock.AppendOnlyTasks); ok {
				local.AppendOnly = a.AppendOnly()
			}
		}
		prepared, prepareErr := draftlock.PrepareLocal(ctx, root, cfg, feature, localDraftCommand, local)
		if prepareErr != nil {
			err = prepareErr
			break
		}
		p := prepared.Proposal
		terminalPrintf(stdout, "%s: prepared %s at %s (generation %d; %s)\n\n%s\n", feature, p.Proposal.ID, prepared.Head, p.Proposal.Sequence, p.OID, prepared.Request)
		terminalPrintf(stdout, "Save the exact request with: pyxis draft review-request %s --generation %s > request.json\n", p.Proposal.ID, p.OID)
		terminalPrintln(stdout, "Before record-review, display the complete saved binding and review its nonce, target ref/OID, generation, head and digest:\ncat request.json")
		terminalPrintln(stderr, "Local review uses a shared OS account. Reviewer and independent origin identity are unauthenticated; mainline has no host-level protection.")
		return 0, true
	case "status":
		s, all, inspectErr := l.Inspect(ctx)
		if inspectErr != nil {
			err = inspectErr
			break
		}
		shown := false
		for _, state := range lifecycle.States(s) {
			if feature != "" && feature != state.Feature {
				continue
			}
			terminalPrintf(stdout, "%s: %s\n", state.Feature, renderDraftState(state))
			shown = true
			for _, g := range all {
				if g.Proposal.Feature == state.Feature && g.Proposal.Kind == "draft" && g.Proposal.State != "abandoned" {
					terminalPrintf(stdout, "  refs/pyxis/proposals/%s at %s (generation %d, head %s)\n", g.Proposal.ID, g.OID, g.Proposal.Sequence, g.Proposal.Head)
				}
			}
		}
		for _, g := range all {
			if g.Proposal.Kind == "draft" && g.Proposal.State == "abandoned" && (feature == "" || feature == g.Proposal.Feature) {
				terminalPrintf(stdout, "%s: abandoned refs/pyxis/proposals/%s at %s — %s\n", g.Proposal.Feature, g.Proposal.ID, g.OID, g.Proposal.AbandonmentReason)
				shown = true
			}
		}
		if !shown && feature != "" {
			err = fmt.Errorf("feature %q has no contract state", feature)
		}
		if !shown && feature == "" {
			terminalPrintln(stdout, "no features")
		}
	case "open":
		result, err = l.Open(ctx, feature)
	case "reopen":
		cause, causeRef := "other", ""
		if len(args) == 6 {
			cause, causeRef = args[3], args[5]
		}
		result, err = l.Reopen(ctx, feature, cause, causeRef)
	case "round":
		snapshot, all, inspectErr := l.Inspect(ctx)
		if inspectErr != nil {
			err = inspectErr
			break
		}
		prepared := false
		proposalID := ""
		for _, g := range all {
			if g.Proposal.Kind == "draft" && g.Proposal.Feature == feature && g.Proposal.State == "prepared" {
				prepared = true
				proposalID = g.Proposal.ID
			}
			if g.Proposal.Kind == "draft" && g.Proposal.Feature == feature && g.Proposal.State == "open" {
				message, readErr := localDraftCommand(ctx, root, nil, "git", "show", "-s", "--format=%s", "HEAD")
				if readErr != nil {
					err = readErr
					break
				}
				prepared = strings.TrimSpace(string(message)) == "docs(draft): prepare lock "+feature
				proposalID = g.Proposal.ID
			}
		}
		if err == nil && prepared {
			var restored draftlock.RoundResult
			restored, err = draftlock.RoundWithProposals(ctx, root, cfg, feature, localDraftCommand, proposals)
			if err == nil {
				result.Generation, err = proposals.Read(ctx, proposalID)
				result.Version = snapshot.LockedVersions[feature] + 1
				if err == nil && (result.Generation.Proposal.State != "open" || result.Generation.Proposal.Head != restored.Head) {
					err = codehost.ErrProposalConflict
				}
			}
		}
		if err == nil && !prepared {
			result, err = l.PublishRound(ctx, feature)
		}
	case "abandon":
		result, err = l.Abandon(ctx, feature, args[3])
	default:
		err = fmt.Errorf("local draft verb %q is not implemented", args[0])
	}
	if err != nil {
		terminalPrintf(stderr, "pyxis: draft %s: %v\n", args[0], err)
		return 1, true
	}
	if result.Generation.OID != "" {
		p := result.Generation.Proposal
		terminalPrintf(stdout, "%s: refs/pyxis/proposals/%s (%s, generation %d, head %s)\n", feature, p.ID, p.State, p.Sequence, p.Head)
	}
	terminalPrintln(stderr, "Local Git uses a shared OS account; review actors and independent origin identity are unauthenticated, and mainline has no host-level protection.")
	return 0, true
}
