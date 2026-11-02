package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	repoconfig "github.com/agenticlab-ai/pyxis/internal/docgate/config"
	executorconfig "github.com/agenticlab-ai/pyxis/internal/executor/config"
	"github.com/agenticlab-ai/pyxis/internal/executor/reviewobligation"
	"github.com/agenticlab-ai/pyxis/internal/watch"
)

var (
	newReviewRearmStore  = reviewobligation.NewGitStore
	newReviewRearmLedger = watch.NewGitRecoveryLedger
	newReviewRearmRound  = watch.NewRoundID
	performReviewRearm   = func(reader watch.ReviewStateReader, ctx context.Context, scope, obligation, principal, reason string, policy watch.RecoveryPolicy) (string, error) {
		return reader.Rearm(ctx, scope, obligation, principal, reason, policy)
	}
	snapshotReviewRearm = func(reader watch.ReviewStateReader, ctx context.Context, scope string) (watch.ReviewWakeState, error) {
		return reader.SnapshotStatus(ctx, scope)
	}
)

func reviewRearmVerb(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("review-rearm", flag.ContinueOnError)
	fs.SetOutput(stderr)
	repo := fs.String("repo", ".", "local repo checkout")
	scope := fs.String("scope", "", "frozen plan scope (feature/NNN)")
	obligation := fs.String("obligation", "", "exhausted review obligation identity")
	principal := fs.String("principal", "", "attributable operator identity")
	reason := fs.String("reason", "", "why recovery is safe to rearm")
	if err := fs.Parse(args); err != nil {
		return statusUsage
	}
	if len(fs.Args()) != 0 || strings.TrimSpace(*scope) == "" || strings.TrimSpace(*obligation) == "" || strings.TrimSpace(*principal) == "" || strings.TrimSpace(*reason) == "" {
		fmt.Fprintln(stderr, "pyxis: review-rearm: --scope, --obligation, --principal, and --reason are required; positional arguments are not accepted")
		return statusUsage
	}
	executorCfg, err := executorconfig.Load(*repo)
	if err != nil {
		fmt.Fprintf(stderr, "pyxis: review-rearm: load executor config: %v\n", err)
		return statusUsage
	}
	if !executorCfg.ReviewConfigured() {
		fmt.Fprintln(stderr, "pyxis: review-rearm: executor.review is not configured")
		return statusCannotBuild
	}
	thresholds, err := watch.LoadThresholds(*repo)
	if err != nil {
		fmt.Fprintf(stderr, "pyxis: review-rearm: load wake config: %v\n", err)
		return statusUsage
	}
	repoCfg, defects := repoconfig.Load(*repo)
	if len(defects) != 0 {
		fmt.Fprintf(stderr, "pyxis: review-rearm: load repo config: %v\n", defects)
		return statusCannotBuild
	}
	provider, err := newReviewRearmStore(*repo, repoCfg.Remote, executorCfg.ReviewEvidenceRef())
	if err != nil {
		fmt.Fprintf(stderr, "pyxis: review-rearm: review store: %v\n", err)
		return statusCannotBuild
	}
	ledger, err := newReviewRearmLedger(*repo, repoCfg.Remote, thresholds.ReviewRecoveryRef)
	if err != nil {
		fmt.Fprintf(stderr, "pyxis: review-rearm: recovery ledger: %v\n", err)
		return statusCannotBuild
	}
	reader := watch.ReviewStateReader{Provider: reviewProviderAdapter{store: provider}, Recovery: ledger}
	roundID, err := newReviewRearmRound()
	if err != nil {
		fmt.Fprintf(stderr, "pyxis: review-rearm: create evidence round: %v\n", err)
		return statusCannotBuild
	}
	evidence, err := performReviewRearm(reader, context.Background(), strings.TrimSpace(*scope), strings.TrimSpace(*obligation), strings.TrimSpace(*principal), strings.TrimSpace(*reason), watch.RecoveryPolicy{Bound: thresholds.ReviewRecoveryBound})
	if err != nil {
		fmt.Fprintf(stderr, "pyxis: review-rearm: %v\n", err)
		return statusCannotBuild
	}
	if state, snapErr := snapshotReviewRearm(reader, context.Background(), strings.TrimSpace(*scope)); snapErr == nil && state.Status == watch.ReviewStateKnown {
		for _, row := range state.Rows {
			if row.ID != strings.TrimSpace(*obligation) {
				continue
			}
			attempts, count, bound := row.AttemptCount, row.RecoveryCount, row.RecoveryBound
			watch.NewSink(filepath.Join(*repo, watch.DefaultEventsDir), 0, nil).Append(watch.Event{Kind: watch.EventReviewRecoveryDecision, DriveRoundID: roundID, ReviewObligationID: row.ID, TaskID: row.TaskID, ProposalRef: row.ProposalRef, Revision: row.Revision, Action: "rearmed", ProviderAttemptCount: &attempts, RecoveryEpoch: row.RecoveryEpoch, RecoveryCount: &count, RecoveryBound: &bound, LastFailureClass: row.LastFailureClass, EvidenceRef: evidence, Reason: strings.TrimSpace(*reason)})
			break
		}
	}
	fmt.Fprintf(stdout, "review rearmed: obligation=%s scope=%s evidence=%s\n", strings.TrimSpace(*obligation), strings.TrimSpace(*scope), evidence)
	return statusClean
}
