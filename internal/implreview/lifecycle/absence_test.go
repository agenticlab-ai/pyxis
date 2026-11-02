package lifecycle

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/implreview/persist"
)

// fakeRecorder records absences into memory — the hermetic AbsenceRecorder the
// tests inject (no network). It counts calls and can be told to fail (the
// recorder-itself-fails leg).
type fakeRecorder struct {
	calls    int
	proposal string
	cause    error
	fail     bool
}

func (f *fakeRecorder) RecordAbsence(proposal string, cause error) (persist.Ref, error) {
	f.calls++
	f.proposal = proposal
	f.cause = cause
	if f.fail {
		return persist.Ref{}, fmt.Errorf("fake recorder: cannot write absence marker")
	}
	return persist.Ref{Medium: persist.MediumGitHub, Locator: "absent-marker-1"}, nil
}

// attemptFailingNTimes returns an Attempt that FAILS its first n calls and then
// succeeds — the "fails then recovers within the bound" fixture. If n is huge it
// effectively always fails.
func attemptFailingNTimes(n int, calls *int) Attempt {
	return func(attempt int) error {
		*calls++
		if attempt <= n {
			return fmt.Errorf("lens backend error on attempt %d", attempt)
		}
		return nil
	}
}

// TestRecordAbsence is the UB-1 contract: a reviewer failing past its bounded
// retry records a review-absence (never a silent pass); a reviewer that recovers
// within the bound records NO absence; the retry is bounded by the attempt count
// (never a non-terminating wait).
func TestRecordAbsence(t *testing.T) {
	cfg := Config{AbsentTimeout: time.Minute, MaxAttempts: 3}

	t.Run("fails every attempt past the bound → review-absence recorded (never a silent pass)", func(t *testing.T) {
		var calls int
		rec := &fakeRecorder{}
		attempt := attemptFailingNTimes(1000, &calls) // always fails
		err := RecordAbsence("github:pyxis#412", attempt, rec, cfg)

		// It is an ERROR (the review did not happen) — never nil (a nil here would
		// be the silent pass UB-1 forbids).
		if err == nil {
			t.Fatal("RecordAbsence = nil on total failure — that is the silent pass UB-1 forbids")
		}
		// It is the recorded-absence outcome, reusing T2's sentinel.
		if !IsRecordedAbsence(err) {
			t.Fatalf("RecordAbsence error = %v, want persist.ErrRecordedAbsence", err)
		}
		if !errors.Is(err, persist.ErrRecordedAbsence) {
			t.Errorf("errors.Is(err, persist.ErrRecordedAbsence) = false; err = %v", err)
		}
		// The absence was actually recorded on the medium.
		var are *persist.AbsenceRecordedError
		if !errors.As(err, &are) {
			t.Fatalf("error is not *persist.AbsenceRecordedError: %v", err)
		}
		if !are.Recorded {
			t.Errorf("AbsenceRecordedError.Recorded = false, want true (the absence was recorded)")
		}
		if are.Attempts != cfg.MaxAttempts {
			t.Errorf("AbsenceRecordedError.Attempts = %d, want %d", are.Attempts, cfg.MaxAttempts)
		}
		if are.AlternateSignal.Locator != "absent-marker-1" {
			t.Errorf("AlternateSignal = %v, want the recorded marker ref", are.AlternateSignal)
		}
		if rec.calls != 1 {
			t.Errorf("recorder called %d times, want exactly 1 (record the absence once)", rec.calls)
		}
		// BOUNDED: exactly MaxAttempts reviewer attempts, no more — the wait cannot
		// exceed the bound (no non-terminating wait).
		if calls != cfg.MaxAttempts {
			t.Errorf("reviewer attempted %d times, want exactly %d (bounded retry)", calls, cfg.MaxAttempts)
		}
	})

	t.Run("succeeds on the first attempt → no absence, no recorder call", func(t *testing.T) {
		var calls int
		rec := &fakeRecorder{}
		attempt := attemptFailingNTimes(0, &calls) // succeeds immediately
		if err := RecordAbsence("github:pyxis#412", attempt, rec, cfg); err != nil {
			t.Fatalf("RecordAbsence = %v, want nil (the reviewer succeeded)", err)
		}
		if calls != 1 {
			t.Errorf("reviewer attempted %d times, want 1 (succeeded first try)", calls)
		}
		if rec.calls != 0 {
			t.Errorf("recorder called %d times, want 0 (no absence to record)", rec.calls)
		}
	})

	t.Run("recovers within the bound → no absence recorded", func(t *testing.T) {
		var calls int
		rec := &fakeRecorder{}
		attempt := attemptFailingNTimes(cfg.MaxAttempts-1, &calls) // fails, fails, then succeeds on the last
		if err := RecordAbsence("github:pyxis#412", attempt, rec, cfg); err != nil {
			t.Fatalf("RecordAbsence = %v, want nil (recovered within the bound)", err)
		}
		if calls != cfg.MaxAttempts {
			t.Errorf("reviewer attempted %d times, want %d (recovered on the last)", calls, cfg.MaxAttempts)
		}
		if rec.calls != 0 {
			t.Errorf("recorder called %d times, want 0 (recovered — no absence)", rec.calls)
		}
	})

	t.Run("recording the absence ITSELF fails → still not silent (Recorded=false, both causes)", func(t *testing.T) {
		var calls int
		rec := &fakeRecorder{fail: true}
		attempt := attemptFailingNTimes(1000, &calls)
		err := RecordAbsence("github:pyxis#412", attempt, rec, cfg)
		if err == nil {
			t.Fatal("RecordAbsence = nil when even the recorder failed — never silent")
		}
		if !IsRecordedAbsence(err) {
			t.Fatalf("error = %v, want persist.ErrRecordedAbsence (still the absence outcome)", err)
		}
		var are *persist.AbsenceRecordedError
		if !errors.As(err, &are) {
			t.Fatalf("error is not *persist.AbsenceRecordedError: %v", err)
		}
		if are.Recorded {
			t.Errorf("AbsenceRecordedError.Recorded = true, want false (the recorder failed)")
		}
		if are.Cause == nil {
			t.Error("AbsenceRecordedError.Cause = nil, want both the reviewer and recorder failures")
		}
	})

	t.Run("MaxAttempts == 1 → one attempt, then absence (a reviewer is attempted at least once)", func(t *testing.T) {
		var calls int
		rec := &fakeRecorder{}
		oneShot := Config{AbsentTimeout: time.Minute, MaxAttempts: 1}
		attempt := attemptFailingNTimes(1000, &calls)
		err := RecordAbsence("github:pyxis#412", attempt, rec, oneShot)
		if !IsRecordedAbsence(err) {
			t.Fatalf("error = %v, want recorded-absence", err)
		}
		if calls != 1 {
			t.Errorf("reviewer attempted %d times, want 1", calls)
		}
	})
}

// TestRecordAbsenceGuards pins the fail-closed programmer-error paths: a nil
// attempt, a nil recorder (an unrecordable absence IS the silent pass UB-1
// forbids), and an invalid config are all refused before any run.
func TestRecordAbsenceGuards(t *testing.T) {
	cfg := Config{AbsentTimeout: time.Minute, MaxAttempts: 3}
	okAttempt := func(int) error { return nil }
	okRec := AbsenceRecorderFunc(func(string, error) (persist.Ref, error) {
		return persist.Ref{Medium: persist.MediumGitHub, Locator: "x"}, nil
	})

	if err := RecordAbsence("p", nil, okRec, cfg); err == nil {
		t.Error("RecordAbsence(nil attempt) = nil, want an error")
	}
	if err := RecordAbsence("p", okAttempt, nil, cfg); err == nil {
		t.Error("RecordAbsence(nil recorder) = nil, want an error (an unrecordable absence is a silent pass)")
	}
	// An invalid config is refused (zero timeout / zero attempts).
	if err := RecordAbsence("p", okAttempt, okRec, Config{}); err == nil {
		t.Error("RecordAbsence(invalid config) = nil, want ErrInvalidConfig")
	} else if !errors.Is(err, ErrInvalidConfig) {
		t.Errorf("RecordAbsence(invalid config) error = %v, want ErrInvalidConfig", err)
	}
}

// TestRecordAbsenceViaFuncAdapter confirms the AbsenceRecorderFunc ergonomic
// adapter drives the same path a struct recorder does — reusing the persist
// alternate-signal shape through a plain function.
func TestRecordAbsenceViaFuncAdapter(t *testing.T) {
	cfg := Config{AbsentTimeout: time.Minute, MaxAttempts: 2}
	var recorded bool
	rec := AbsenceRecorderFunc(func(proposal string, cause error) (persist.Ref, error) {
		recorded = true
		if proposal != "github:pyxis#99" {
			t.Errorf("recorder got proposal %q, want the failing proposal", proposal)
		}
		if cause == nil {
			t.Error("recorder got nil cause, want the reviewer failure")
		}
		return persist.Ref{Medium: persist.MediumGitHub, Locator: "m"}, nil
	})
	var calls int
	err := RecordAbsence("github:pyxis#99", attemptFailingNTimes(1000, &calls), rec, cfg)
	if !IsRecordedAbsence(err) {
		t.Fatalf("error = %v, want recorded-absence", err)
	}
	if !recorded {
		t.Error("the func recorder was never called")
	}
}
