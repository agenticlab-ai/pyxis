package lifecycle

import (
	"errors"
	"fmt"

	"github.com/agenticlab-ai/pyxis/internal/implreview/persist"
)

// Attempt is one bounded reviewer attempt (UB-1): the closure the lifecycle runs
// under [RecordAbsence]. It returns nil when the reviewer produced its review,
// or an error when the lens backend errored / timed out / exceeded its bound —
// which [RecordAbsence] retries up to the bound and then records as an absence.
// It is a closure (not a value) so each retry is a FRESH attempt (a new lens
// run), never a re-check of a stale result — the same shape T8's RoundAttempt
// uses for the void-and-retry.
//
// The `attempt` count passed in is the 1-based attempt number (1 for the first
// try), so an attempt that wants to log/adjust per-try can, though it need not.
type Attempt func(attempt int) error

// RecordAbsence runs a reviewer Attempt under a BOUNDED retry and, when it never
// succeeds within the bound, RECORDS a review-absence on the proposal — the UB-1
// contract: a reviewer that errors or exceeds its bound after bounded retry
// records a review-absence, NEVER a silent pass, NEVER an indefinite stall
// (§3.10, the "lens backend error/timeout past bound" row).
//
// On success (an attempt returns nil within cfg.MaxAttempts) it returns nil —
// the review was produced, no absence. On failure past the bound it RECORDS the
// absence through the injected `recorder` and returns a
// *persist.AbsenceRecordedError (which is persist.ErrRecordedAbsence to
// errors.Is) — REUSING T2's absence outcome so a runtime-failure absence is the
// SAME distinguishable, harvestable signal a persist-failure absence is (the
// plan's "reuse the T2 absence outcome where it fits"). It is an ERROR (the
// review did not happen) but a NON-silent one: the absence is recorded, the
// cause is carried, and it is never nil-on-failure.
//
// The retry is BOUNDED by cfg.MaxAttempts (≥ 1) — there is NO clock wait in this
// package; the bound is the attempt COUNT, so the loop terminates in at most
// MaxAttempts iterations regardless of the clock (an "indefinite stall" is
// impossible by construction — the wait cannot exceed the bound). Any real
// per-attempt timeout lives INSIDE the Attempt (the runtime bounds one lens run
// via its own ctx); this package bounds the NUMBER of attempts.
//
// `recorder` records the absence on the proposal's medium (T2's alternate-signal
// mechanism, or a host equivalent) and returns the Ref of the recorded absence
// marker — reused so the absence is visible and harvestable exactly like a
// persist absence. A nil recorder is a programmer error (the absence MUST be
// recordable — a review-absence that cannot be recorded is the silent pass this
// contract forbids), so RecordAbsence refuses it. If the recorder ITSELF fails,
// the returned AbsenceRecordedError carries Recorded=false and both causes —
// still never a silent loss (T2's own fallback shape).
func RecordAbsence(proposal string, attempt Attempt, recorder AbsenceRecorder, cfg Config) error {
	if attempt == nil {
		return fmt.Errorf("record absence: nil attempt")
	}
	if recorder == nil {
		// A review-absence that cannot be recorded would be the silent pass UB-1
		// forbids — fail closed rather than run with no way to record the absence.
		return fmt.Errorf("record absence: nil recorder — a review-absence must be recordable (never a silent pass)")
	}
	if err := cfg.Validate(); err != nil {
		return err
	}

	// The bounded retry: attempt up to cfg.MaxAttempts times. A success within the
	// bound returns nil (the review happened — no absence). The bound is the
	// COUNT, so this loop cannot run forever — no indefinite stall.
	var lastErr error
	for n := 1; n <= cfg.MaxAttempts; n++ {
		err := attempt(n)
		if err == nil {
			return nil // the reviewer produced its review — no absence.
		}
		lastErr = err
	}

	// Past the bound: the reviewer never succeeded. Record the absence on the
	// proposal (UB-1) — never a silent pass. lastErr is the failure that exhausted
	// the bound.
	cause := fmt.Errorf("reviewer failed after %d attempts: %w", cfg.MaxAttempts, lastErr)
	ref, rerr := recorder.RecordAbsence(proposal, cause)
	if rerr != nil {
		// Even recording the absence failed. Still not silent: return the recorded
		// error with Recorded=false and BOTH causes (T2's fallback shape).
		return &persist.AbsenceRecordedError{
			Proposal: proposal,
			Attempts: cfg.MaxAttempts,
			Recorded: false,
			Cause:    fmt.Errorf("%w; recording the absence also failed: %v", cause, rerr),
		}
	}
	return &persist.AbsenceRecordedError{
		Proposal:        proposal,
		Attempts:        cfg.MaxAttempts,
		AlternateSignal: ref,
		Recorded:        true,
		Cause:           cause,
	}
}

// AbsenceRecorder records a review-absence on a proposal's medium and returns the
// Ref of the recorded absence marker (UB-1). It is the SEAM the runtime-failure
// absence shares with the persist-failure absence: the GitHub adapter's
// alternate-signal write is one implementation (a distinct absence-marker
// comment), a host janitor another. It is defined on the CONSUMER side here (the
// lifecycle declares what it needs to record an absence), and an adapter — or a
// tiny wrapper over persist.Port's own absence path — satisfies it; a hermetic
// test injects a fake. Keeping it an interface is what makes RecordAbsence
// testable with no network.
type AbsenceRecorder interface {
	// RecordAbsence records that the review for `proposal` is ABSENT (the reviewer
	// failed past its bound), with `cause` the underlying failure. It returns the
	// Ref of the recorded absence marker on the medium (so the absence is visible
	// and harvestable), or an error if even the absence could not be recorded
	// (which RecordAbsence surfaces with Recorded=false — never a silent loss).
	RecordAbsence(proposal string, cause error) (persist.Ref, error)
}

// AbsenceRecorderFunc adapts a plain function to AbsenceRecorder — the ergonomic
// form a caller/test uses to supply a recorder without declaring a whole type.
type AbsenceRecorderFunc func(proposal string, cause error) (persist.Ref, error)

// RecordAbsence implements AbsenceRecorder.
func (f AbsenceRecorderFunc) RecordAbsence(proposal string, cause error) (persist.Ref, error) {
	return f(proposal, cause)
}

// IsRecordedAbsence reports whether an error is the recorded review-absence
// outcome (UB-1) — persist.ErrRecordedAbsence, whether it came from a persist
// failure (T2) or a runtime failure recorded here. A caller branches on it to
// proceed FLAGGED in advisory / route HUMAN post-adoption (the §3.10 recovery
// column) rather than treat the failure as a pass. It is a thin errors.Is
// wrapper so a caller need not import persist just to test the sentinel.
func IsRecordedAbsence(err error) bool {
	return errors.Is(err, persist.ErrRecordedAbsence)
}
