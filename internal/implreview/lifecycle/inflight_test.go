package lifecycle

import (
	"errors"
	"testing"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/implreview/report"
)

// baseTime is a fixed instant the in-flight tests measure from — no wall-clock;
// `now` is computed as baseTime + an offset so the boundary is exact.
var baseTime = time.Date(2026, 7, 17, 12, 0, 0, 0, time.UTC)

// TestInFlight is the reap contract (UB-8): a started-round-no-verdict review is
// flagged in-flight/orphaned PAST the configured absent-timeout and NOT within
// it; a terminal review is never flagged. The clock is injected via `now`.
func TestInFlight(t *testing.T) {
	const timeout = 30 * time.Minute
	cfg := Config{AbsentTimeout: timeout, MaxAttempts: 3}

	cases := []struct {
		name    string
		report  *report.Report
		started time.Time
		now     time.Time
		want    bool
	}{
		{
			name:    "started round, no verdict, PAST timeout → in-flight (orphan)",
			report:  reportWith("", 1),
			started: baseTime,
			now:     baseTime.Add(timeout + time.Second),
			want:    true,
		},
		{
			name:    "started round, no verdict, WITHIN timeout → NOT in-flight (slow deep-band live review)",
			report:  reportWith("", 1),
			started: baseTime,
			now:     baseTime.Add(timeout - time.Second),
			want:    false,
		},
		{
			name:    "started round, no verdict, EXACTLY at timeout → NOT in-flight (strict boundary, still within)",
			report:  reportWith("", 1),
			started: baseTime,
			now:     baseTime.Add(timeout),
			want:    false,
		},
		{
			name:    "started round, no verdict, one nanosecond PAST → in-flight",
			report:  reportWith("", 1),
			started: baseTime,
			now:     baseTime.Add(timeout + time.Nanosecond),
			want:    true,
		},
		{
			name:    "terminal verdict (approve), long past timeout → NOT in-flight (completed, nothing to reap)",
			report:  reportWith(report.VerdictApprove, 1),
			started: baseTime,
			now:     baseTime.Add(100 * timeout),
			want:    false,
		},
		{
			name:    "terminal verdict (request-changes), past timeout → NOT in-flight (review completed, bounced)",
			report:  reportWith(report.VerdictRequestChanges, 1),
			started: baseTime,
			now:     baseTime.Add(timeout + time.Hour),
			want:    false,
		},
		{
			name:    "terminal verdict (escalate), past timeout → NOT in-flight",
			report:  reportWith(report.VerdictEscalate, 2),
			started: baseTime,
			now:     baseTime.Add(timeout + time.Hour),
			want:    false,
		},
		{
			name:    "no started round (zero rounds), past timeout → NOT in-flight (nothing started)",
			report:  reportWith("", 0),
			started: baseTime,
			now:     baseTime.Add(timeout + time.Hour),
			want:    false,
		},
		{
			name:    "nil report → NOT in-flight",
			report:  nil,
			started: baseTime,
			now:     baseTime.Add(timeout + time.Hour),
			want:    false,
		},
		{
			name:    "started, no verdict, but zero RoundStartedAt → NOT in-flight (no round-start to measure from)",
			report:  reportWith("", 1),
			started: time.Time{},
			now:     baseTime.Add(timeout + time.Hour),
			want:    false,
		},
		{
			name:    "deep-band-slow multi-round, no verdict yet, within timeout → NOT in-flight",
			report:  reportWith("", 2),
			started: baseTime,
			now:     baseTime.Add(timeout / 2),
			want:    false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			state := InFlightState{Report: tc.report, RoundStartedAt: tc.started}
			if got := InFlight(state, cfg, tc.now); got != tc.want {
				t.Errorf("InFlight = %t, want %t (elapsed %v, timeout %v)",
					got, tc.want, tc.now.Sub(tc.started), timeout)
			}
		})
	}
}

// TestInFlightGarbageTimeoutNeverTrips: a zero/negative timeout must never
// fabricate an orphan — the predicate fails SAFE (the config is Validated
// separately; the predicate itself will not trip on garbage).
func TestInFlightGarbageTimeoutNeverTrips(t *testing.T) {
	state := InFlightState{Report: reportWith("", 1), RoundStartedAt: baseTime}
	for _, to := range []time.Duration{0, -time.Second, -time.Hour} {
		cfg := Config{AbsentTimeout: to, MaxAttempts: 1}
		if InFlight(state, cfg, baseTime.Add(1000*time.Hour)) {
			t.Errorf("InFlight with garbage timeout %v = true, want false (fail safe)", to)
		}
	}
}

// TestTimeUntilAbsent exercises the countdown a janitor logs: positive while
// live, non-positive once orphaned, zero for a non-candidate.
func TestTimeUntilAbsent(t *testing.T) {
	const timeout = 30 * time.Minute
	cfg := Config{AbsentTimeout: timeout, MaxAttempts: 3}
	inflight := InFlightState{Report: reportWith("", 1), RoundStartedAt: baseTime}

	// Within the window: positive remaining.
	if got := TimeUntilAbsent(inflight, cfg, baseTime.Add(10*time.Minute)); got != 20*time.Minute {
		t.Errorf("TimeUntilAbsent within window = %v, want %v", got, 20*time.Minute)
	}
	// Past the window: non-positive (overdue).
	if got := TimeUntilAbsent(inflight, cfg, baseTime.Add(timeout+5*time.Minute)); got != -5*time.Minute {
		t.Errorf("TimeUntilAbsent past window = %v, want %v", got, -5*time.Minute)
	}
	// A terminal review is not a candidate → 0.
	terminal := InFlightState{Report: reportWith(report.VerdictApprove, 1), RoundStartedAt: baseTime}
	if got := TimeUntilAbsent(terminal, cfg, baseTime); got != 0 {
		t.Errorf("TimeUntilAbsent for terminal = %v, want 0", got)
	}
	// A zero round-start is not a candidate → 0.
	nostart := InFlightState{Report: reportWith("", 1), RoundStartedAt: time.Time{}}
	if got := TimeUntilAbsent(nostart, cfg, baseTime); got != 0 {
		t.Errorf("TimeUntilAbsent for zero round-start = %v, want 0", got)
	}
}

// TestConfigValidate pins the fail-closed config contract: the absent-timeout
// must be positive and the retry bound at least one; a garbage config is
// rejected with ErrInvalidConfig so the lifecycle never runs with a config that
// would let a review stall forever or never bound its retry.
func TestConfigValidate(t *testing.T) {
	cases := []struct {
		name    string
		cfg     Config
		wantErr bool
	}{
		{"valid", Config{AbsentTimeout: time.Minute, MaxAttempts: 1}, false},
		{"valid larger", Config{AbsentTimeout: time.Hour, MaxAttempts: 5}, false},
		{"zero timeout → invalid", Config{AbsentTimeout: 0, MaxAttempts: 1}, true},
		{"negative timeout → invalid", Config{AbsentTimeout: -time.Second, MaxAttempts: 1}, true},
		{"zero attempts → invalid", Config{AbsentTimeout: time.Minute, MaxAttempts: 0}, true},
		{"negative attempts → invalid", Config{AbsentTimeout: time.Minute, MaxAttempts: -1}, true},
		{"both bad → invalid", Config{}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.cfg.Validate()
			if tc.wantErr {
				if err == nil {
					t.Fatal("Validate = nil, want ErrInvalidConfig")
				}
				if !errors.Is(err, ErrInvalidConfig) {
					t.Errorf("Validate error = %v, want ErrInvalidConfig", err)
				}
			} else if err != nil {
				t.Errorf("Validate = %v, want nil", err)
			}
		})
	}
}
