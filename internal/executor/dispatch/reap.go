package dispatch

import (
	"context"
	"fmt"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/executor/port"
	"github.com/agenticlab-ai/pyxis/internal/implreview/lifecycle"
)

// This file is the host-side impl-review reap (impl-review-v1 §3.4 lifecycle,
// §3.10 orphaned-review reap; plan T14, prd:impl-review/UB-8). It is ADDITIVE to
// the Dispatcher: Sweep is byte-untouched, and the reaper is a separate host
// entry the /impl-review integration wires. The AP-12 reaper-LOCATION deviation
// lands here — the reaper executes host-side because the review lives on the
// host's proposal surface — while the in-flight PREDICATE and the absent-timeout
// remain DEFINED by the reviewer (internal/implreview/lifecycle, T9): this file
// CONSUMES lifecycle.InFlight, it does not redefine it. Nothing here changes the
// frontier sweep; a Dispatcher whose host never calls ReapOrphanedReviews
// behaves exactly as before.

// ReviewReinvoker re-invokes the impl-review reviewer for an ORPHANED in-flight
// review the reap identified — the host-side half of the AP-12 reaper-location
// deviation (§3.4). It is a consumer-side seam mirroring the worker's
// ReviewInvoker: the Dispatcher declares only that it needs "a reviewer
// re-invoked for this orphaned proposal", and the /impl-review integration
// injects an implementation. The reviewer runtime it fronts RESUMES from the
// persisted round state (idempotent, lifecycle.Decide — T9/UB-8), so a re-invoke
// of an orphan continues the review rather than starting a fresh round 1.
type ReviewReinvoker interface {
	// Reinvoke re-runs the reviewer for one proposal whose in-flight review was
	// reaped as an orphan. It is handed only the proposal identity — the reviewer
	// runtime reads the persisted round state and resumes (T9). An error is
	// returned for the caller to record (a re-invoke that itself fails records a
	// review-absence per the reviewer's UB-1 contract); the reap continues over
	// the other candidates regardless (one orphan's re-invoke failure never
	// strands its siblings).
	Reinvoke(ctx context.Context, proposal port.Proposal) error
}

// OrphanCandidate is one proposal whose review MIGHT be an orphaned in-flight
// review the reap should re-invoke. The host supplies the candidate set (it
// enumerates the proposals it dispatched and reads each one's persisted review
// state); the reap applies lifecycle.InFlight to decide which are genuinely
// orphaned. It pairs the proposal identity (what the reviewer re-invoke needs)
// with the lifecycle.InFlightState the predicate reads — the persisted review
// and the moment its current round started (the host knows when it invoked the
// reviewer; the persisted payload carries no wall-clock, pack determinism).
type OrphanCandidate struct {
	// Proposal identifies the proposal whose review is a reap candidate — what
	// ReviewReinvoker.Reinvoke is handed when this candidate is an orphan.
	Proposal port.Proposal
	// State is the lifecycle in-flight input for this proposal: the persisted
	// review report as the host found it, and when the current round started. The
	// reap passes it verbatim to lifecycle.InFlight — the predicate the reviewer
	// defines (T9), consumed here (the AP-12 deviation is the reaper's location,
	// never the predicate's).
	State lifecycle.InFlightState
}

// ReapReport is the classed result of one ReapOrphanedReviews pass — which
// candidates were live (within their absent-timeout, left alone), which were
// orphaned and re-invoked, and any re-invoke that failed. It is the observable
// surface a caller logs/monitors, mirroring the sweep's RunReport posture.
type ReapReport struct {
	// Live is the count of candidates the in-flight predicate judged LIVE — a
	// started-round-no-verdict review still WITHIN its absent-timeout (a possibly
	// slow deep-band review, not an orphan). These are left untouched.
	Live int
	// Reaped is the count of orphaned reviews the predicate flagged and the reaper
	// re-invoked (whether or not the re-invoke itself then succeeded).
	Reaped int
	// ReinvokeErrs pairs each proposal number whose re-invoke returned an error
	// with that error, so a caller can surface the failed re-invokes. A reaped
	// orphan whose re-invoke errored is still counted in Reaped (it WAS reaped);
	// its error is recorded here for visibility.
	ReinvokeErrs map[int]error
}

// ReapOrphanedReviews reaps the orphaned in-flight reviews among the candidate
// set and re-invokes each through the ReviewReinvoker seam (UB-8, AP-12
// deviation, §3.4/§3.10). For every candidate it applies lifecycle.InFlight
// (state, cfg, now): a candidate the predicate judges in-flight/orphaned — a
// started round with no terminal verdict past the configured absent-timeout — is
// re-invoked; a live one (within the timeout) or a completed one (terminal
// verdict) is left alone. The clock is INJECTED as now, so the reap is a pure,
// hermetic function of its inputs (no wall-clock read here — the same discipline
// lifecycle.InFlight holds).
//
// It is ADDITIVE and host-driven: Sweep never calls it, so the frontier sweep is
// unchanged; a host that never wires a reviewer never calls this, and the base
// behavior is untouched. cfg is Validated first (a garbage absent-timeout would
// otherwise never trip the predicate — fail closed on a mis-set config rather
// than silently reaping nothing forever). The reinvoker is required (reaping is
// meaningless with nothing to re-invoke).
//
// One orphan's re-invoke failure never strands the others: an errored Reinvoke
// is recorded in the report and the reap continues over the remaining
// candidates (fail-open across siblings, the janitor posture — a single stuck
// re-invoke must not block reaping the rest).
func (d *Dispatcher) ReapOrphanedReviews(ctx context.Context, candidates []OrphanCandidate, reinvoker ReviewReinvoker, cfg lifecycle.Config, now time.Time) (ReapReport, error) {
	if reinvoker == nil {
		return ReapReport{}, fmt.Errorf("dispatch: nil ReviewReinvoker (reaping needs a reviewer to re-invoke)")
	}
	// Fail closed on a mis-set lifecycle config: a zero/garbage absent-timeout
	// would make InFlight never trip, silently reaping nothing. Validate up front
	// so a broken config is an error, not a silent no-op (AP-6, the closed-parse
	// posture the lifecycle Config already carries).
	if err := cfg.Validate(); err != nil {
		return ReapReport{}, fmt.Errorf("dispatch: invalid reap lifecycle config: %w", err)
	}

	report := ReapReport{}
	for _, cand := range candidates {
		// Honor cancellation promptly: stop reaping once the context is done. The
		// candidates not yet reached are simply left for the next reap pass.
		if err := ctx.Err(); err != nil {
			return report, fmt.Errorf("dispatch: reap cancelled: %w", err)
		}
		// The predicate the reviewer defines (T9) decides orphan-vs-live — this
		// package only CONSUMES it (the AP-12 deviation is the reaper's location).
		if !lifecycle.InFlight(cand.State, cfg, now) {
			// Live (within its timeout) or complete (terminal verdict): not an
			// orphan, leave it alone. Only count the live ones the predicate would
			// have timed (a completed or unstarted review is neither live nor reaped).
			if isTimedLiveReview(cand.State, cfg) {
				report.Live++
			}
			continue
		}
		// Orphaned: re-invoke the reviewer, which resumes from the persisted round
		// state (idempotent — T9). It WAS reaped whether or not the re-invoke then
		// succeeds; a re-invoke error is recorded but never strands the siblings.
		report.Reaped++
		if err := reinvoker.Reinvoke(ctx, cand.Proposal); err != nil {
			if report.ReinvokeErrs == nil {
				report.ReinvokeErrs = make(map[int]error)
			}
			report.ReinvokeErrs[cand.Proposal.Number] = fmt.Errorf("dispatch: re-invoke review for proposal #%d: %w", cand.Proposal.Number, err)
		}
	}
	return report, nil
}

// isTimedLiveReview reports whether a NON-orphaned candidate is a LIVE review the
// predicate was actively timing — a started round with no terminal verdict and a
// real round-start, i.e. one that InFlight judged still WITHIN its absent-timeout
// (as opposed to a completed review, an unstarted one, or one with no round-start
// to time). It is used only to classify the reap report's Live count; it never
// gates the reap (InFlight alone does that).
func isTimedLiveReview(state lifecycle.InFlightState, cfg lifecycle.Config) bool {
	return cfg.AbsentTimeout > 0 &&
		lifecycle.HasStartedRound(state.Report) &&
		!lifecycle.TerminalVerdict(state.Report) &&
		!state.RoundStartedAt.IsZero()
}
