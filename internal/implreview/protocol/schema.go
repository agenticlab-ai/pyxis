package protocol

import (
	"errors"
	"fmt"

	"github.com/agenticlab-ai/pyxis/internal/implreview/report"
)

// RoundAttempt produces one candidate report state for a round — the fresh
// evaluation the caller runs (round 1 or the scope-locked round 2). It is a
// closure so the void-and-retry (UB-2) re-runs a FRESH attempt rather than
// re-validating the same bytes: a schema-invalid round is VOIDED and the caller
// evaluates AGAIN. It returns the candidate *report.Report to validate, or an
// error if the attempt itself could not run (which the retry treats like an
// invalid round — a failed attempt is not a passing round).
type RoundAttempt func() (*report.Report, error)

// RecordRound records a round with the schema-invalid void-retry-once-then-absent
// contract (UB-2, §3.10): it runs an attempt, VALIDATES the resulting report
// with report.Validate (T1) against the calibration lookup, and:
//
//   - if the attempt is SCHEMA-VALID → returns the validated report (the round
//     is recorded);
//   - if the attempt is SCHEMA-INVALID → VOIDS it and RETRIES ONCE (a fresh
//     attempt); if the retry is valid → returns it; if the retry is ALSO invalid
//     → returns ErrReviewAbsent, the terminal a caller maps to the T2/T9
//     review-absence recording (never a silent pass — §3.10).
//
// This reuses report.Validate as the schema-validity oracle (the plan's "reuse
// report.Validate / the T2 absence outcome if it fits"): a round is
// schema-invalid exactly when the report carrying it fails report.Validate
// (DINV-1/2/3/7, the decline-reason and round rules). cal is the calibration
// lookup report.Validate needs to certify DINV-3; a nil cal is a programmer
// error (report.Validate would fail closed) and RecordRound refuses it.
//
// The retry budget is EXACTLY ONE (MaxSchemaRetries): void, retry once, then
// absent — never an unbounded retry-until-green (which §3.5's eval protocol also
// forbids for a different reason). An attempt() that ERRORS is treated as an
// invalid round (a failed evaluation is not a passing round), so it too voids +
// retries, then goes absent — a failing attempt never yields a silent pass.
func RecordRound(attempt RoundAttempt, cal report.CalLookup) (*report.Report, error) {
	if attempt == nil {
		return nil, fmt.Errorf("record round: nil attempt")
	}
	if cal == nil {
		return nil, fmt.Errorf("record round: nil CalLookup — a round's schema validity cannot be certified without the calibration table")
	}

	// The first attempt.
	r, err := tryRound(attempt, cal)
	if err == nil {
		return r, nil
	}
	firstErr := err

	// Void and retry ONCE. The retry is a FRESH attempt (the caller re-evaluates),
	// not a re-validation of the same bytes.
	r, err = tryRound(attempt, cal)
	if err == nil {
		return r, nil
	}

	// The void-and-retry both failed: the terminal is review-absent (UB-2). Name
	// both failures so a reader can see WHY the round is absent; the caller maps
	// ErrReviewAbsent to the T2/T9 absence recording (never a silent pass).
	return nil, fmt.Errorf("%w: first attempt %v; retry %v", ErrReviewAbsent, firstErr, err)
}

// MaxSchemaRetries is the void-and-retry budget for a schema-invalid round
// (UB-2): exactly ONE retry after the initial attempt, then review-absent. It is
// a constant, not a knob — the protocol's fixed "void, retry once, then absent"
// shape.
const MaxSchemaRetries = 1

// tryRound runs one attempt and validates its report with report.Validate. It
// returns the validated report on success, or an error when the attempt itself
// failed OR the resulting report is schema-invalid — both of which the
// void-and-retry treats as a failed round.
func tryRound(attempt RoundAttempt, cal report.CalLookup) (*report.Report, error) {
	r, err := attempt()
	if err != nil {
		return nil, fmt.Errorf("round attempt: %w", err)
	}
	if err := report.Validate(r, cal); err != nil {
		return nil, fmt.Errorf("round schema-invalid: %w", err)
	}
	return r, nil
}

// IsReviewAbsent reports whether an error is the review-absent terminal (UB-2) —
// the void-retry-once-then-absent outcome a caller maps to the T2/T9
// review-absence recording. A caller branches on it with this predicate (or
// errors.Is(err, ErrReviewAbsent)) to record the absence rather than treat the
// failure as a pass.
func IsReviewAbsent(err error) bool {
	return errors.Is(err, ErrReviewAbsent)
}
