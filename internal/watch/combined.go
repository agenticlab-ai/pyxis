package watch

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"
)

var (
	ErrReviewExhausted    = errors.New("watch: configured review recovery exhausted")
	ErrReviewStateUnknown = errors.New("watch: configured review state remains unknown")
)

// RunCombinedLive extends RunLive with the configured-review frontier. The
// existing implementation transition/cadence behavior is retained; review
// recovery is admitted independently and can never call the ordinary sweep.
func RunCombinedLive(
	ctx context.Context,
	source PlanSource,
	reader Reader,
	reviews ReviewStateReader,
	policy RecoveryPolicy,
	scope string,
	implementationSweep Sweep,
	reviewSweep ReviewSweep,
	roundIDs RoundIDSource,
	sink *Sink,
	clock Clock,
	hints <-chan struct{},
) (Summary, error) {
	if clock == nil {
		clock = realClock{}
	}
	if source == nil || reader == nil || reviews.Provider == nil || reviews.Recovery == nil || implementationSweep == nil || reviewSweep == nil || roundIDs == nil || scope == "" {
		return Summary{}, errors.New("watch: combined source, readers, sweeps, round source, and scope are required")
	}
	if err := policy.validate(); err != nil {
		return Summary{}, err
	}
	var sum Summary
	var previous string
	var previousHandoff string
	forceSweep := false
	unknownReads := 0
	seenRounds := map[string]bool{}

	for {
		if err := ctx.Err(); err != nil {
			return sum, err
		}
		plan, err := source(ctx)
		if err != nil {
			return sum, fmt.Errorf("watch: plan source: %w", err)
		}
		set, err := Derive(ctx, plan, reader)
		if err != nil {
			return sum, err
		}
		reviewState, err := reviews.DeriveReadiness(ctx, scope, policy)
		if err != nil {
			return sum, err
		}
		if reviewState.Status == ReviewStateUnknown {
			unknownReads++
			roundID, idErr := nextUniqueRoundID(roundIDs, seenRounds)
			if idErr != nil {
				return sum, idErr
			}
			retry := reader.Cadence(ctx)
			if retry <= 0 {
				return sum, fmt.Errorf("watch: reader returned non-positive cadence %s", retry)
			}
			retryMS := int(retry / time.Millisecond)
			sink.Append(Event{Kind: EventReviewStateUnknown, DriveRoundID: roundID, Scope: scope, Source: reviewState.Unknown.Source, FailureKind: reviewState.Unknown.Kind, Action: "defer", RetryAfterMS: &retryMS, Reason: reviewState.Unknown.Reason})
			if unknownReads >= 2 {
				return sum, fmt.Errorf("%w: %s: %s", ErrReviewStateUnknown, reviewState.Unknown.Source, reviewState.Unknown.Reason)
			}
			select {
			case <-ctx.Done():
				return sum, ctx.Err()
			case <-clock.After(retry):
			case <-hints:
			}
			continue
		}
		unknownReads = 0
		sum.Awaits = len(set.Awaits) + len(reviewState.Rows)
		if set.Complete() && reviewState.Complete() {
			sum.Completed = true
			return sum, nil
		}

		key := set.key()
		readyKey := set.readyKey()
		handoffKey := ""
		for _, row := range reviewState.Rows {
			if row.Eligibility == ReviewHandoffRepair {
				handoffKey += row.ID + "\x00"
			}
		}
		if key != previous || handoffKey != previousHandoff || (forceSweep && (readyKey != "" || handoffKey != "")) {
			previous = key
			previousHandoff = handoffKey
			forceSweep = false
			if err := implementationSweep(ctx); err != nil {
				return sum, fmt.Errorf("watch: implementation sweep: %w", err)
			}
			sum.Sweeps++
			continue
		}
		previous = key
		previousHandoff = handoffKey
		forceSweep = false

		var eligible, exhausted []ReviewWakeRow
		for _, row := range reviewState.Rows {
			switch row.Eligibility {
			case ReviewEligible:
				eligible = append(eligible, row)
			case ReviewExhausted:
				exhausted = append(exhausted, row)
			}
		}
		sort.Slice(eligible, func(i, j int) bool { return eligible[i].ID < eligible[j].ID })
		sort.Slice(exhausted, func(i, j int) bool { return exhausted[i].ID < exhausted[j].ID })

		decisionRoundID := ""
		if len(exhausted)+len(eligible) > 0 {
			var idErr error
			decisionRoundID, idErr = nextUniqueRoundID(roundIDs, seenRounds)
			if idErr != nil {
				return sum, idErr
			}
		}
		for _, row := range exhausted {
			evidence, recordErr := reviews.RecordExhaustion(ctx, scope, decisionRoundID, row)
			if recordErr != nil {
				return sum, fmt.Errorf("watch: persist review exhaustion: %w", recordErr)
			}
			emitReviewDecision(sink, decisionRoundID, row, "exhausted", evidence, "consumer recovery bound exhausted")
		}
		if len(eligible) == 0 && len(exhausted) > 0 {
			return sum, ErrReviewExhausted
		}

		if len(eligible) > 0 {
			request := ReviewRecoveryRequest{DriveRoundID: decisionRoundID, ObligationIDs: make([]string, 0, len(eligible))}
			for _, row := range eligible {
				admitted, admitErr := reviews.Admit(ctx, scope, decisionRoundID, row)
				if admitErr != nil {
					return sum, fmt.Errorf("watch: append-first review admission: %w", admitErr)
				}
				request.ObligationIDs = append(request.ObligationIDs, admitted.ID)
				emitReviewDecision(sink, decisionRoundID, admitted, "invoke_review_only", "", "")
			}
			sum.Sweeps++
			sum.ReviewSweeps++
			// A non-zero review-only child is a settled pending attempt, not an
			// implementation failure. Re-derive provider truth and spend another
			// consumer admission only if the frozen bound still permits it.
			if err := reviewSweep(ctx, request); err != nil && ctx.Err() != nil {
				return sum, ctx.Err()
			}
			continue
		}

		// No actionable review exists. Preserve RunLive's cadence behavior for
		// an unchanged implementation frontier.
		sum.Polls++
		cadence := reader.Cadence(ctx)
		if cadence <= 0 {
			return sum, fmt.Errorf("watch: reader returned non-positive cadence %s", cadence)
		}
		select {
		case <-ctx.Done():
			return sum, ctx.Err()
		case <-clock.After(cadence):
		case <-hints:
		}
		forceSweep = true
	}
}

func nextUniqueRoundID(source RoundIDSource, seen map[string]bool) (string, error) {
	id, err := source()
	if err != nil {
		return "", err
	}
	if id == "" || seen[id] {
		return "", errors.New("watch: round ID source returned an empty or duplicate identity")
	}
	seen[id] = true
	return id, nil
}

func emitReviewDecision(sink *Sink, roundID string, row ReviewWakeRow, action, evidence, reason string) {
	attempts, count, bound := row.AttemptCount, row.RecoveryCount, row.RecoveryBound
	sink.Append(Event{Kind: EventReviewRecoveryDecision, DriveRoundID: roundID, ReviewObligationID: row.ID, TaskID: row.TaskID, ProposalRef: row.ProposalRef, Revision: row.Revision, Action: action, ProviderAttemptCount: &attempts, RecoveryEpoch: row.RecoveryEpoch, RecoveryCount: &count, RecoveryBound: &bound, LastFailureClass: row.LastFailureClass, EvidenceRef: evidence, Reason: reason})
}
