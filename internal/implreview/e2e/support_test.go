package e2e

import (
	"errors"
	"fmt"
)

// Shared test-only sentinels and helpers for the in-memory persist.Port fake.

// errWriteFailed is the simulated code-host write failure the memPort injects for
// the UB-5 path — a plain error the caller's bounded-retry/absence handling acts
// on (the memPort itself is a dumb store; the retry contract lives in the
// production adapter and in lifecycle.RecordAbsence).
var errWriteFailed = errors.New("e2e memport: simulated code-host write failure")

// errNoArtifact is returned when a Read addresses a locator the memPort never
// stored — a programmer error in the test (never a silent empty payload).
var errNoArtifact = errors.New("e2e memport: no artifact at that locator")

// locatorFor builds a stable, opaque locator for the Nth stored artifact — the
// memPort's stand-in for a GitHub comment id (opaque to consumers, §3.2).
func locatorFor(n int) string { return fmt.Sprintf("impl-review-comment-%d", n) }

// operatorLogin is the operator's medium identity the harvest resolves an
// operator disposition's attribution against (FR-16/DINV-4). A disposition whose
// artifact author is anything else (a worker, the bot) is forged and excluded.
const operatorLogin = "mdarabi"
