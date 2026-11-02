package lifecycle

import (
	"fmt"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/implreview/report"
)

// Config is the lifecycle's DATA — the tunables the reap and absence contracts
// read, carried as config rather than hard-coded (UB-8, UB-1; design §3.4: the
// absent-timeout is "config data, not hard-coded"; §0 sanity-check: the
// absent-timeout is config, not frozen here). The zero value is NOT a usable
// config — a caller MUST supply the timeout and the retry bound (Validate
// rejects a zero one) so a missing config fails closed rather than defaulting to
// a silent "never times out" / "never retries".
type Config struct {
	// AbsentTimeout is how long a STARTED round may sit with NO terminal verdict
	// before the in-flight predicate flags it as an orphan the janitor reaps
	// (UB-8). WITHIN this window a verdict-less started round is a (possibly slow
	// deep-band) LIVE review, not an orphan; PAST it, it is orphaned. It is
	// DATA — a deep-band review's honest budget sets it; this package never
	// picks a value. Must be > 0.
	AbsentTimeout time.Duration

	// MaxAttempts is the BOUNDED reviewer-attempt budget before a persistent
	// failure records a review-absence (UB-1): the attempt runs up to this many
	// times, then the absence is recorded — never an unbounded retry-until-green
	// (which §3.5's eval protocol also forbids), never an indefinite stall. Must
	// be ≥ 1 (a reviewer is always attempted at least once). It mirrors the
	// GitHub adapter's defaultMaxWriteAttempts posture: a small const-like bound
	// carried as data.
	MaxAttempts int
}

// Sentinel error for a malformed config — part of the package contract so a
// caller/test can branch with errors.Is.
var ErrInvalidConfig = fmt.Errorf("impl-review lifecycle: invalid config")

// Validate reports whether the config is usable: a positive absent-timeout and a
// retry bound of at least one. A zero/negative timeout or a bound below one is
// rejected (ErrInvalidConfig) — the lifecycle never runs with a config that
// would let a review stall forever (zero timeout) or never attempt/never bound
// (zero attempts). This is the closed-parse posture (AP-6) applied to the
// lifecycle's own data.
func (c Config) Validate() error {
	if c.AbsentTimeout <= 0 {
		return fmt.Errorf("%w: absent-timeout must be > 0 (a review must time out into an orphan, never sit forever)", ErrInvalidConfig)
	}
	if c.MaxAttempts < 1 {
		return fmt.Errorf("%w: max-attempts must be >= 1 (a reviewer is attempted at least once)", ErrInvalidConfig)
	}
	return nil
}

// InFlightState is the input to the in-flight predicate: the persisted review
// (for the started-round + terminal-verdict bits) and the wall-clock moment the
// review's CURRENT round STARTED. The persisted report.Report is a
// content-addressed payload with NO wall-clock (pack.go's determinism posture),
// so the round-start time is carried HERE — the host supplies it (it knows when
// it invoked the reviewer), the same way it supplies the proposal head to the
// staleness predicate (verdict.IsStale). This keeps the predicate a pure
// function of its inputs with the clock INJECTED.
type InFlightState struct {
	// Report is the persisted review as the host found it (possibly nil / not yet
	// persisted). HasStartedRound and TerminalVerdict read its state.
	Report *report.Report
	// RoundStartedAt is when the current (latest) round's evaluation STARTED — the
	// moment the absent-timeout is measured FROM. Supplied by the host (it invoked
	// the reviewer); the zero time means "no started round is being timed" and the
	// predicate treats it as not-in-flight (there is no round-start to have
	// elapsed from).
	RoundStartedAt time.Time
}

// InFlight reports whether a review is an ORPHANED in-flight review the janitor
// (T14) should reap (UB-8) — the reap contract this package defines so the
// resource is reap-able BY CONTRACT even though the reaper executes host-side
// (the AP-12 deviation, §3.4). A review is in-flight/orphaned iff ALL of:
//
//   - it has a STARTED round (HasStartedRound), AND
//   - it has NO terminal verdict (the envelope verdict is unset — the review
//     never finalized), AND
//   - the CONFIGURED absent-timeout has ELAPSED since the round started
//     (now.Sub(RoundStartedAt) > cfg.AbsentTimeout).
//
// The predicate is FALSE — the review is a LIVE, still-running review, not an
// orphan — when it is WITHIN the timeout (a slow deep-band review is exactly a
// started-round-no-verdict review inside its budget), when it has a TERMINAL
// verdict (the review completed — nothing to reap), when it has NO started round
// (nothing has started), or when RoundStartedAt is the zero time (no round-start
// to measure from).
//
// It is a PURE predicate over (state, cfg, now): the clock is INJECTED as `now`,
// so a test drives "within" vs "past" the timeout with no wall-clock. A zero/
// negative cfg.AbsentTimeout is treated as "never in-flight" (fail SAFE: the
// predicate never fabricates an orphan from an unset config — a caller Validates
// the config separately; the predicate itself will not trip on a garbage
// timeout).
//
// The boundary is STRICT: elapsed == AbsentTimeout is still WITHIN (not yet an
// orphan); the timeout must be strictly EXCEEDED. This makes "does not flag one
// within it" true at the exact boundary.
func InFlight(state InFlightState, cfg Config, now time.Time) bool {
	// A garbage timeout never trips the predicate (fail safe — do not invent an
	// orphan from an unset/negative config).
	if cfg.AbsentTimeout <= 0 {
		return false
	}
	// Must have a started round with no terminal verdict; otherwise not in-flight.
	if !HasStartedRound(state.Report) {
		return false
	}
	if TerminalVerdict(state.Report) {
		return false
	}
	// No round-start to measure the elapsed window from → not (yet) an orphan.
	if state.RoundStartedAt.IsZero() {
		return false
	}
	// Orphaned only once the configured timeout is STRICTLY exceeded; at or within
	// the window it is a live (possibly slow deep-band) review.
	elapsed := now.Sub(state.RoundStartedAt)
	return elapsed > cfg.AbsentTimeout
}

// TimeUntilAbsent returns how long remains before a started-round-no-verdict
// review would trip the in-flight predicate at `now` — a positive duration while
// the review is still LIVE (within the timeout), and a non-positive duration
// once it is orphaned (the amount by which it is already past). It is exposed so
// a janitor can log/schedule "reap in T" rather than only poll the boolean, and
// so a caller can distinguish "live, 30s of budget left" from "orphaned, 5s
// overdue".
//
// For a review that is NOT a candidate for in-flight (no started round, a
// terminal verdict, a zero round-start, or a non-positive timeout) it returns 0
// — there is no meaningful countdown for a review that will never be flagged.
func TimeUntilAbsent(state InFlightState, cfg Config, now time.Time) time.Duration {
	if cfg.AbsentTimeout <= 0 ||
		!HasStartedRound(state.Report) ||
		TerminalVerdict(state.Report) ||
		state.RoundStartedAt.IsZero() {
		return 0
	}
	deadline := state.RoundStartedAt.Add(cfg.AbsentTimeout)
	return deadline.Sub(now)
}
