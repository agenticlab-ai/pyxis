package preflight

import (
	"context"
	"time"
)

// SetProbeSleepForTest swaps the GitHub-mode existence probe's bounded-retry
// sleeper for a no-op (hermetic, zero real delay) — the external
// preflight_test package's seam onto this package's otherwise-unexported
// clock, mirroring internal/installer's own SetWireExecuteForTest pattern.
// Returns a restore function.
func SetProbeSleepForTest() (restore func()) {
	old := probeSleep
	probeSleep = func(context.Context, time.Duration) error { return nil }
	return func() { probeSleep = old }
}
