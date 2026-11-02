package band

import "errors"

// Sentinel errors — part of the package contract, one per fail-closed condition,
// so a caller/test can branch with errors.Is.
var (
	// ErrMissingDeepContext — a deep-band review was asked to enrich its pack but
	// the DeepContext carried neither the ABUSE rows nor the incident registry
	// (FR-10: the deep-band context is MANDATORY). The band fails closed rather
	// than run a deep review missing its mandatory context.
	ErrMissingDeepContext = errors.New("impl-review band: deep band requires its mandatory context (served ABUSE rows + incident registry)")

	// ErrOversize — the diff exceeds the band's honest read budget (UB-7). The
	// caller records a size-escalation routing the proposal HUMAN; it NEVER emits
	// an approve over a partially-read diff.
	ErrOversize = errors.New("impl-review band: diff exceeds the band's honest read budget (size-escalation, route HUMAN)")

	// ErrNilVerifier — the second-instance verify was enabled (flag on) with no
	// verifier runner supplied; it fails closed rather than silently skip the
	// re-judge it was told to perform.
	ErrNilVerifier = errors.New("impl-review band: second-instance verify enabled but no verifier runner supplied")
)
