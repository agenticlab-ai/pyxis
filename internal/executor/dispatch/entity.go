package dispatch

import (
	"github.com/agenticlab-ai/pyxis/internal/entity"
	"github.com/agenticlab-ai/pyxis/internal/executor/port"
	"github.com/agenticlab-ai/pyxis/internal/executor/report"
	"github.com/agenticlab-ai/pyxis/internal/executor/worker"
)

// sweepItem is one unit of work a sweep considers. Frontier candidates, handoff
// recoveries, and pair repairs are the same value, so each stage advances this
// item instead of keeping a parallel slice.
//
// Impl and Review stay separate. A review failure must not rewrite the
// implementation class: the run report counts the two axes independently.
type sweepItem struct {
	Task   port.Task
	Origin itemOrigin

	Impl   implState
	Review reviewState

	// Runtime and Reviewer are bound for this sweep. A Worker is not stored:
	// each implementation attempt builds a fresh one.
	Runtime  port.AgentRuntime
	Reviewer worker.ReviewService
}

// itemOrigin is how the item entered the sweep.
type itemOrigin int

const (
	// originFrontier is a candidate read from the tracker.
	originFrontier itemOrigin = iota
	// originHandoff is an already-green implementation whose review obligation
	// was saved and whose review has not started.
	originHandoff
	// originPair is a pair_verified obligation whose lifecycle terminal is
	// still missing.
	originPair
)

// implState is the implementation half of a sweep item. Class is the terminal
// headline TaskResult already reports. Phase is how the item reached it.
type implState struct {
	Phase    implPhase
	Class    Class
	Outcome  worker.Outcome
	Ran      bool
	Err      error
	Tokens   int
	Attempts int

	Proposal   port.Proposal
	PrePublish *worker.PrePublishOutcome
	// Capture is the durable review obligation, once the green implementation
	// has written it.
	Capture *worker.ReviewCapture
}

// implPhase is the implementation position of one sweep item.
type implPhase int

const (
	implCandidate implPhase = iota
	implBlocked
	implReady
	implAwaitingProposal
	implProbeUnavailable
	// implFeedbackAdmitted is still runnable. Task.Feedback carries the round.
	implFeedbackAdmitted
	implClaimLost
	implRunning
	implSucceeded
	implFailed
)

// reviewState is the review half of a sweep item. The zero phase means this
// sweep does not owe a review for the item.
type reviewState struct {
	Phase        reviewPhase
	ObligationID string
	Harness      entity.Harness
	ProposalRef  string
	Revision     string
	Result       *worker.ReviewResult
}

// reviewPhase is the review position of one sweep item.
type reviewPhase int

const (
	_ reviewPhase = iota
	// reviewNotOwed means no reviewer is configured, or a pre-publish review
	// already covers the exact published head.
	reviewNotOwed
	_
	// reviewHandoffPending means the obligation exists and its handoff marker
	// is not committed.
	reviewHandoffPending
	// reviewHandoffReady means the handoff marker is committed and Invoke is
	// allowed.
	reviewHandoffReady
	// reviewPairPending means EVT-13 and EVT-17 agree and EVT-15 is missing.
	reviewPairPending
	reviewInvoked
	reviewVerified
	reviewFailed
)

// applyTaskResult copies a classed implementation outcome onto the item.
// Review is left untouched: a later review result must not rewrite Class.
func (it *sweepItem) applyTaskResult(res TaskResult) {
	it.Task = res.Task
	it.Impl.Class = res.Class
	it.Impl.Outcome = res.Outcome
	it.Impl.Ran = res.Ran
	it.Impl.Err = res.Err
	it.Impl.Tokens = res.Tokens
	it.Impl.Proposal = res.Proposal
	it.Impl.PrePublish = res.PrePublish
	it.Impl.Capture = res.ReviewCapture
	switch res.Class {
	case ClassSucceeded:
		it.Impl.Phase = implSucceeded
	case ClassSkipped:
		it.Impl.Phase = implClaimLost
	case ClassAwaitingProposal:
		it.Impl.Phase = implAwaitingProposal
	case ClassProbeUnavailable:
		it.Impl.Phase = implProbeUnavailable
	case ClassBlocked:
		it.Impl.Phase = implBlocked
	default:
		it.Impl.Phase = implFailed
	}
}

// setReviewResult records the review half. The implementation class stays as
// applyTaskResult left it.
func (it *sweepItem) setReviewResult(result worker.ReviewResult) {
	it.Review.Result = &result
	if result.ObligationID != "" {
		it.Review.ObligationID = result.ObligationID
	}
	if result.Harness != "" {
		it.Review.Harness = result.Harness
	}
	if result.ProposalRef != "" {
		it.Review.ProposalRef = result.ProposalRef
	}
	if result.Revision != "" {
		it.Review.Revision = result.Revision
	}
	if result.Status == report.ReviewStatusFailed || result.Err != nil {
		it.Review.Phase = reviewFailed
		return
	}
	it.Review.Phase = reviewVerified
}

// taskResult is the implementation report row for this item, with the review
// attached only when this sweep recorded one on the same item.
func (it sweepItem) taskResult() TaskResult {
	return TaskResult{
		Task:          it.Task,
		Class:         it.Impl.Class,
		Outcome:       it.Impl.Outcome,
		Ran:           it.Impl.Ran,
		Err:           it.Impl.Err,
		Tokens:        it.Impl.Tokens,
		Proposal:      it.Impl.Proposal,
		PrePublish:    it.Impl.PrePublish,
		ReviewCapture: it.Impl.Capture,
		Review:        it.Review.Result,
	}
}

// getTaskResults projects tracker-order implementation rows.
func getTaskResults(items []sweepItem) []TaskResult {
	if items == nil {
		return nil
	}
	out := make([]TaskResult, len(items))
	for i := range items {
		out[i] = items[i].taskResult()
	}
	return out
}

// getReviewResults projects review rows in group order, skipping items that have
// not recorded a review result.
func getReviewResults(groups ...[]sweepItem) []worker.ReviewResult {
	var out []worker.ReviewResult
	for _, group := range groups {
		for i := range group {
			if group[i].Review.Result == nil {
				continue
			}
			out = append(out, *group[i].Review.Result)
		}
	}
	return out
}
