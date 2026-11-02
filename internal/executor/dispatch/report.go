package dispatch

import (
	"fmt"

	"github.com/agenticlab-ai/pyxis/internal/executor/port"
	"github.com/agenticlab-ai/pyxis/internal/executor/worker"
)

// Class is the terminal classification of one task within a sweep. It is the
// Dispatcher's normalization of a Worker's Outcome plus the two run-model states
// a Worker never sees — a task still gated behind an open blocker (blocked) and a
// task another worker already holds (skipped) (docs/tech-design/implement-now.md
// §3.6, §3.4).
type Class int

const (
	// ClassBlocked is the zero value: a candidate that never became ready
	// because a blocker in its BlockedBy set is still open. It is not a run
	// failure — it is reported so the driver knows what to land next (§3.6 step
	// 5). Zero-valued so an un-run task defaults to "not attempted", never a
	// spurious success.
	ClassBlocked Class = iota
	// ClassSucceeded is a worker.OutcomeSucceeded task: a proposal exists and
	// every required check is green (DINV-3, AP-13). The only success class.
	ClassSucceeded
	// ClassFailed is a task that ran but could not be verified green — a
	// worker.OutcomeFailed, a worker.OutcomeHollow (clean run, no proposal —
	// never succeeded, DINV-3), or an operational error during the cycle. The
	// raw Outcome is kept on the TaskResult so hollow stays distinguishable for
	// the hollow-rate metric (§3.4).
	ClassFailed
	// ClassSkipped is a task whose claim was already held by another worker — a
	// local sibling or the external fleet (port.ErrClaimLost). It is a skip, not
	// a failure: the losing racer no-ops (DINV-5, AP-22). It is reported so the
	// driver knows the task is in flight elsewhere, but it is never counted as
	// this run's success or failure.
	ClassSkipped
	// ClassAwaitingProposal is an unblocked task that already has an open
	// proposal on its deterministic head branch. The work has been proposed;
	// re-running it would not advance it — it would push fresh commits onto a
	// live proposal — so the sweep classes it here, before any claim, and
	// spends no lease, worktree, or model call on it (DINV-2, DINV-8).
	//
	// This holds for EVERY open proposal, whatever its checks say. A red
	// proposal is not evidence that its Worker died: far more often it is one
	// that exhausted its bounded self-heal, recorded the failure and released
	// its claim. Restarting on check colour alone resets the per-sweep bounds
	// and retries forever across sweeps (the unbounded retry DINV-9 forbids),
	// so resuming an open proposal is left to an explicit, durable recovery
	// authorization that weighs attempt, outcome, claim, branch and proposal
	// together — never inferred here (AP-17).
	ClassAwaitingProposal
	// ClassProbeUnavailable is a ready task the pre-claim preflight could not
	// evaluate: the proposal probe failed, so the sweep has no evidence about
	// whether the task is already proposed. It withholds — a dispatch act
	// requires positive evidence (AP-17) — but this is a FAILURE, not healthy
	// awaiting: it is counted, it reddens the run's exit status, and it reaches
	// the unattended escalation surface, so a code-host outage can never read as
	// a quiet, successful, empty sweep (AP-16). The probe error rides the
	// TaskResult's Err as the outcome's reason.
	ClassProbeUnavailable
)

// String renders the class for logs and test output.
func (c Class) String() string {
	switch c {
	case ClassSucceeded:
		return "succeeded"
	case ClassFailed:
		return "failed"
	case ClassBlocked:
		return "blocked"
	case ClassSkipped:
		return "skipped"
	case ClassAwaitingProposal:
		return "awaiting-proposal"
	case ClassProbeUnavailable:
		return "probe-unavailable"
	default:
		return fmt.Sprintf("Class(%d)", int(c))
	}
}

// TaskResult is the classed outcome of one task the sweep considered. Every task
// the Dispatcher saw — run, blocked, or skipped — gets exactly one TaskResult, so
// the report classes the whole frontier (prd:implement-now/FR-15).
type TaskResult struct {
	// Task is the task this result is for.
	Task port.Task

	// Class is the terminal classification (the headline the report counts).
	Class Class

	// Outcome is the Worker's raw outcome, valid only when Ran is true. It keeps
	// the hollow-vs-hard-failed distinction that Class folds into ClassFailed
	// (DINV-3, the §3.4 hollow-rate metric).
	Outcome worker.Outcome

	// Ran reports whether a Worker actually executed this task. False for a
	// blocked task (never ready) or a skipped task (claim lost before the cycle
	// did any work).
	Ran                   bool
	VerificationAttempted bool

	// Err is the operational error from the Worker's cycle, if any: nil on a
	// clean succeeded/hollow run, the wrapped port.ErrClaimLost on a skip, or the
	// cycle's failure on an operational error. It is advisory context for the
	// driver, never the class itself.
	Err error

	// Tokens is implementation runtime usage across every bounded respawn for
	// this task. It is telemetry only; artifacts determine Class.
	Tokens int

	// Proposal is populated on the genuinely-green path. Review is present only
	// when executor.review was configured and that proposal reached its independent
	// reviewer; a failed Review never rewrites Class or Outcome but does red the run.
	Proposal      port.Proposal
	ReviewCapture *worker.ReviewCapture
	Review        *worker.ReviewResult

	// PrePublish is the local-first convergence loop's record for this task
	// (worker/prepublish.go), present when a pre-publish reviewer was installed
	// and the loop was reached. Covered=true means the accepted review binds
	// the exact published head, so no post-green Review is owed.
	PrePublish *worker.PrePublishOutcome
}

// RunReport is the classed result of one frontier-sweep: the per-task results in
// a deterministic order plus the headline counts the driver and the event stream
// read (docs/tech-design/implement-now.md §3.6 step 4, §3.4 run_finished). One
// task's failure never removes it or its siblings from the report — every
// considered task is classed (AP-16).
type RunReport struct {
	// Results holds one TaskResult per task the sweep considered, in the order
	// the tracker returned them (stable across runs for testability).
	Results []TaskResult
	// ReviewResults holds recovered configured-review attempts that are not a
	// new implementation task in this sweep (for example handoff repair after a
	// crash). They affect only review tallies.
	ReviewResults []worker.ReviewResult

	// Succeeded, Failed, Blocked and Skipped are the headline counts, each the
	// number of Results in the matching Class.
	Succeeded int
	Failed    int
	Blocked   int
	Skipped   int

	// AwaitingProposal is the headline count of unblocked tasks left alone
	// because they already carry an open proposal. Like Blocked, it is work the
	// sweep deliberately did not start, so it is never counted succeeded or
	// failed and never makes NothingReady false.
	AwaitingProposal int

	// ProbeUnavailable is the headline count of ready tasks the pre-claim
	// preflight could not evaluate. Unlike AwaitingProposal it is a failure
	// signal: ready work was found and could not be judged, so it makes
	// NothingReady false and reddens the run.
	ProbeUnavailable int
	Tokens           int

	// ReviewCompleted/ReviewFailed are orthogonal to implementation task
	// classes. A task remains Succeeded while its review failure reddens the run
	// and escalates explicitly.
	ReviewCompleted            int
	ReviewFailed               int
	ReviewPending              int
	ReviewArtifactUnverified   int
	ReviewArtifactUntrusted    int
	ReviewInfrastructureFailed int

	// PrePublishReviewed counts succeeded tasks whose pre-publish review covers
	// the exact published head — reviewed before the push, no post-green
	// review owed. It is orthogonal to ReviewCompleted (the post-green path).
	PrePublishReviewed int

	// FeedbackRounds counts the human-review feedback rounds this sweep ran
	// (tasks admitted with new human activity on their open proposal).
	FeedbackRounds int
}

// NothingReady reports whether the sweep found no ready work — an empty frontier
// (no candidates, or every candidate still blocked). It is the DINV-8 signal: a
// run that reports NothingReady spawned no Worker and made zero model calls
// (prd:implement-now/FR-12, AP-19). It is true exactly when no task passed the
// readiness predicate: nothing ran, nothing was skipped, and no ready task was
// A probe-unavailable task counts as ready work found: the sweep could not
// judge it, so claiming the frontier was empty would be a lie that reads as a
// clean, idle run (AP-16).
func (r RunReport) NothingReady() bool {
	return r.Succeeded+r.Failed+r.Skipped+r.ProbeUnavailable+r.ReviewCompleted+r.ReviewFailed+r.ReviewPending == 0
}

// Ran is the number of tasks a Worker actually executed this sweep (succeeded
// plus failed). Blocked and skipped tasks are not counted, since no Worker ran
// for them.
func (r RunReport) Ran() int {
	return r.Succeeded + r.Failed
}

// tally rebuilds the headline counts from Results. It is called once, after all
// Results are collected, so a concurrent write never races a count.
func (r *RunReport) tally() {
	r.Succeeded, r.Failed, r.Blocked, r.Skipped = 0, 0, 0, 0
	r.AwaitingProposal = 0
	r.ProbeUnavailable = 0
	r.Tokens = 0
	r.ReviewCompleted, r.ReviewFailed, r.ReviewPending = 0, 0, 0
	r.ReviewArtifactUnverified, r.ReviewArtifactUntrusted, r.ReviewInfrastructureFailed = 0, 0, 0
	r.PrePublishReviewed, r.FeedbackRounds = 0, 0
	for _, review := range r.ReviewResults {
		r.tallyReview(review)
	}
	for _, res := range r.Results {
		r.Tokens += res.Tokens
		if res.Task.Feedback != nil && res.Ran {
			r.FeedbackRounds++
		}
		if res.PrePublish != nil {
			r.Tokens += res.PrePublish.Tokens
			if res.PrePublish.Covered && res.Class == ClassSucceeded {
				r.PrePublishReviewed++
			}
		}
		if res.Review != nil {
			r.tallyReview(*res.Review)
		}
		switch res.Class {
		case ClassAwaitingProposal:
			r.AwaitingProposal++
		case ClassProbeUnavailable:
			r.ProbeUnavailable++
		case ClassSucceeded:
			r.Succeeded++
		case ClassFailed:
			r.Failed++
		case ClassBlocked:
			r.Blocked++
		case ClassSkipped:
			r.Skipped++
		}
	}
}

func (r *RunReport) tallyReview(review worker.ReviewResult) {
	r.Tokens += review.Tokens
	if review.Failed() {
		r.ReviewFailed++
		if review.ObligationStatus == "pending" {
			r.ReviewPending++
		}
		switch review.FailureClass {
		case worker.ReviewFailureArtifactUnverified:
			r.ReviewArtifactUnverified++
		case worker.ReviewFailureArtifactUntrusted:
			r.ReviewArtifactUntrusted++
		case worker.ReviewFailureInfrastructure:
			r.ReviewInfrastructureFailed++
		}
		return
	}
	r.ReviewCompleted++
}

// classify maps one Worker cycle's return — its Outcome and error — to a
// TaskResult. It is the single place a run outcome becomes a report class
// (DINV-4): a lost claim is a skip keyed on the sentinel (not a failure, AP-22);
// any other error or a non-succeeded outcome (including hollow) is failed
// (fail-closed, DINV-3); only a verified-green outcome is succeeded.
func classify(task port.Task, outcome worker.Outcome, err error) TaskResult {
	switch {
	case isClaimLost(err):
		// The task is held elsewhere; the Worker did no real work. Advisory
		// outcome is discarded — the sentinel is the signal (worker.Run returns
		// OutcomeFailed here, which must not be read as a work failure).
		return TaskResult{Task: task, Class: ClassSkipped, Ran: false, Err: err}
	case err != nil:
		// An operational error during the cycle. One task's failure never halts
		// the run (AP-16); it is classed and the sweep continues.
		return TaskResult{Task: task, Class: ClassFailed, Outcome: worker.OutcomeFailed, Ran: true, Err: err}
	case outcome == worker.OutcomeSucceeded:
		return TaskResult{Task: task, Class: ClassSucceeded, Outcome: outcome, Ran: true}
	default:
		// OutcomeFailed or OutcomeHollow: ran, not verified green. Hollow is kept
		// in Outcome so it stays visible (DINV-3), but it is never a success.
		return TaskResult{Task: task, Class: ClassFailed, Outcome: outcome, Ran: true}
	}
}
