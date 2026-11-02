package lifecycle

import (
	"fmt"

	"github.com/agenticlab-ai/pyxis/internal/implreview/report"
)

// Action is what a re-invocation of the reviewer should do given the persisted
// review state — the closed outcome set of [Decide]. It makes the
// idempotent-resume contract (UB-8) a value the caller switches on, so a
// double-invocation CANNOT accidentally start a fresh round 1: the only way to
// reach ActionStart is an ABSENT (or unstarted) persisted state.
type Action string

const (
	// ActionStart — no persisted review exists (or one exists with no started
	// round). The reviewer starts a fresh round 1. This is the ONLY action that
	// begins evaluation from scratch; a re-invocation over an existing review
	// never reaches it (the no-fresh-round-1 guarantee).
	ActionStart Action = "start"

	// ActionResume — a persisted review has a started round but NO terminal
	// verdict (T4 recorded a round; T7 has not stamped a verdict). The reviewer
	// RESUMES from the persisted round state — the persisted rounds are
	// authoritative, and the next work continues from the latest recorded round
	// (Decision.AtRound), never a new round 1 (UB-8).
	ActionResume Action = "resume"

	// ActionComplete — a persisted review has a started round AND a terminal
	// verdict. The review is DONE for this invocation; a re-invocation is a NO-OP
	// that produces nothing new (the double-invocation idempotence — a completed
	// review is never re-run into a fresh round). A between-rounds fix that must
	// force a new round is the two-round protocol's job (T8/T7), not a lifecycle
	// restart.
	ActionComplete Action = "complete"
)

// Decision is the deterministic outcome of reading a persisted review's state
// (UB-8): the [Action] to take, and — for ActionResume/ActionComplete — the
// index of the LATEST recorded round the persisted state left off at (AtRound),
// so the caller resumes relative to the authoritative persisted rounds rather
// than guessing. AtRound is the 1-based round number of the latest round (round
// 1 is the first); it is 0 for ActionStart (no round has been recorded).
type Decision struct {
	// Action is what to do: start / resume / complete.
	Action Action
	// AtRound is the 1-based number of the latest recorded round (0 when none) —
	// the round the persisted state is authoritative through. For ActionResume it
	// is the round to resume from; for ActionComplete it is the round the terminal
	// verdict certifies; for ActionStart it is 0.
	AtRound int
}

// String renders a Decision for logs — never parsed back.
func (d Decision) String() string {
	return fmt.Sprintf("%s@round%d", d.Action, d.AtRound)
}

// Decide reads a persisted review's round state as AUTHORITATIVE and returns the
// idempotent resume decision (UB-8, DINV-1). It is a PURE function over the
// persisted report — no clock, no IO — so a re-invocation always maps the SAME
// persisted state to the SAME action:
//
//   - a NIL persisted report (never persisted) or one with NO started round →
//     [ActionStart]: begin a fresh round 1. This is the only path to a fresh
//     round 1.
//   - a persisted report with a started round and NO terminal verdict →
//     [ActionResume] at the latest recorded round: continue from the persisted
//     round state, never a new round 1 (the persisted rounds are authoritative).
//   - a persisted report with a started round AND a terminal verdict →
//     [ActionComplete] at the latest recorded round: the review is done, and a
//     re-invocation is a NO-OP (the double-invocation idempotence — no fresh
//     round is started).
//
// The single source of the "resume vs start vs complete" decision is the
// persisted state's (started-round, terminal-verdict) pair — exactly the two
// bits DINV-1's one-payload-per-review makes unambiguous. Because Decide never
// returns ActionStart for a report that already has a round, a caller that
// starts ONLY on ActionStart cannot double-start a review — the no-fresh-round-1
// guarantee is structural in the returned Action, not a caller convention.
func Decide(persisted *report.Report) Decision {
	if !HasStartedRound(persisted) {
		// Never persisted, or persisted with no round yet: start round 1. This is
		// the only path that begins evaluation.
		return Decision{Action: ActionStart, AtRound: 0}
	}

	// A round exists — the review has started. The persisted rounds are
	// authoritative; the latest recorded round is where the state left off.
	latest := len(persisted.Rounds) // 1-based: len == the latest round number.

	if TerminalVerdict(persisted) {
		// A terminal verdict is stamped: the review completed. A re-invocation is
		// a no-op — resume yields nothing new, and no fresh round is started.
		return Decision{Action: ActionComplete, AtRound: latest}
	}

	// A started round but no terminal verdict: resume from the persisted round
	// state — continue the review, never a fresh round 1.
	return Decision{Action: ActionResume, AtRound: latest}
}

// ShouldStartFresh reports whether the reviewer should begin a fresh round 1 for
// this invocation — true ONLY for [ActionStart]. It is the one-liner a caller
// uses at the top of an invocation to enforce the no-fresh-round-1 guarantee:
// resume/complete both return false, so a re-invocation over an existing review
// never starts fresh (UB-8). A double-invocation therefore does not begin a new
// round 1 — the second call resumes or is a no-op.
func ShouldStartFresh(persisted *report.Report) bool {
	return Decide(persisted).Action == ActionStart
}
