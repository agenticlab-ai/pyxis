package band

import (
	"context"
	"fmt"

	"github.com/agenticlab-ai/pyxis/internal/implreview/report"
	"github.com/agenticlab-ai/pyxis/internal/implreview/runtime"
)

// The dormant deep-band second-instance verify (A-8, §3.6-seam).
//
// A DORMANT, DATA-GATED seam: OFF by default, flipped at adoption on precision
// evidence (design D-4; the always-on alternative was rejected — it pre-empts the
// operator's dial before any data). When ON, it re-judges a DEEP-band BLOCKING
// finding through a SECOND INDEPENDENT subagent (a distinct reviewer root, so the
// second instance is a fresh context, not the first's) and DOWNGRADES a finding
// the second instance does not confirm (blocking → advisory). When OFF it never
// runs — Verify returns the findings verbatim and spawns nothing.
//
// It REUSES T4's runtime.LensRunner seam (the four-subagent spawn machinery) for
// the second instance rather than a new spawn path: the verifier is a
// runtime.LensRunner whose Run re-judges one finding's claim and returns a
// confirming candidate iff the second instance agrees (AP-11, reuse not rebuild).
// The Go core models the seam and the downgrade; the real second-instance spawn
// is the skill's job at runtime (a hermetic test injects a fake verifier).

// VerifyConfig is the second-instance verify's data gate (A-8). The zero value
// is OFF — the dormant default — so a caller that never sets Enabled ships the
// seam off by construction. Enabled is the ONE flag; flipping it to true at
// adoption is a data change, never a code edit.
type VerifyConfig struct {
	// Enabled is the data flag. false (the zero value) = dormant: Verify is a
	// no-op. true = the second instance re-judges deep-band blocking findings.
	Enabled bool
}

// Verifier is the second-instance re-judge seam. It is a runtime.LensRunner (the
// SAME T4 seam) so the second instance reuses the four-subagent spawn machinery:
// its Run re-evaluates a single finding's claim in a fresh context and returns a
// CONFIRMING candidate (a non-empty finding set) iff the second instance agrees
// the finding is a real blocking defect, or an EMPTY set when it does not confirm
// (the signal to downgrade). A real Verifier spawns a second independent subagent
// at runtime; a test injects a fake.
type Verifier = runtime.LensRunner

// VerifyRequest is the input to one second-instance verify pass over a round's
// findings (A-8). It carries the flag, the reviewer roots (the second instance's
// root must be a SIBLING of both the implementer and the first-round reviewer —
// a fresh independent context), the shared pack the second instance re-reads, and
// the Verifier seam.
type VerifyRequest struct {
	// Config is the data gate. Config.Enabled==false makes Verify a no-op.
	Config VerifyConfig

	// Band is the review depth. The verify runs ONLY in the deep band (A-8: "a
	// deep-band blocking finding"); any other band is a no-op even when enabled.
	Band Band

	// SecondInstanceRoot is the second instance's session root — a fresh sibling,
	// distinct from both the implementer and the first reviewer (the second
	// instance is INDEPENDENT, §3.4/DINV-5). Provenance-checked against
	// ImplementerRoot below.
	SecondInstanceRoot string
	// ImplementerRoot is the implementing session's root; Verify fails closed if
	// the second instance's context is inherited from it (reusing the runtime's
	// CheckFreshInstance — the second instance must be as independent as the
	// first).
	ImplementerRoot string
	// FirstReviewerRoot is the FIRST round's reviewer root; the second instance
	// must NOT share it either (a second instance that reused the first's context
	// would not be independent). Verify fails closed if it does.
	FirstReviewerRoot string

	// Pack is the shared review context pack the second instance re-reads (the
	// same read-only input; the second instance forms its own judgment over it).
	Pack []byte

	// Verifier is the second-instance re-judge seam (real: a second independent
	// subagent; test: a fake). Required when Config.Enabled is true.
	Verifier Verifier
}

// VerifyResult is what one Verify pass produces: the (possibly downgraded)
// findings and the ids of the findings the second instance did not confirm
// (downgraded blocking → advisory), surfaced for the debugging/observability
// surface. When the verify is a no-op (disabled, or not the deep band), Findings
// is the input verbatim and Downgraded is empty.
type VerifyResult struct {
	// Findings is the findings after the second-instance pass — deep-band
	// blocking findings the second instance did not confirm are downgraded to
	// advisory; every other finding is unchanged.
	Findings []report.Finding
	// Downgraded is the FND ids the second instance downgraded (did not confirm),
	// in input order — the observable a test and the harvest read.
	Downgraded []string
	// Ran reports whether the second instance actually ran (true only when
	// enabled AND deep band AND there was at least one blocking finding to
	// re-judge) — so a caller can distinguish "ran, confirmed all" from "did not
	// run".
	Ran bool
}

// Verify runs the dormant second-instance verify over a round's findings (A-8).
//
// When Config.Enabled is FALSE (the dormant default), it is a NO-OP: it returns
// the findings verbatim, runs no verifier, and Ran is false — the seam is
// present but off. It is likewise a no-op for any band other than deep, and when
// there is no blocking finding to re-judge.
//
// When ENABLED in the DEEP band, it re-judges every BLOCKING finding through the
// second-instance Verifier: a finding the second instance CONFIRMS (Run returns a
// non-empty candidate set) stays blocking; a finding it does NOT confirm (Run
// returns no candidates) is DOWNGRADED to advisory and recorded in Downgraded.
// Advisory findings are never re-judged (only a blocking finding can be
// downgraded). The second instance's independence is provenance-checked first
// (reusing runtime.CheckFreshInstance against both the implementer and the first
// reviewer); a failed check is fail-closed (no re-judge over a non-independent
// second instance).
//
// It NEVER upgrades a finding — the second instance can only downgrade an
// unconfirmed blocking finding, never promote an advisory one (the seam is a
// precision lever, one-directional). The escalated CalLookup a downgraded
// finding is later validated against must resolve the finding's (possibly
// unchanged) cal_row to advisory; because the shipped critic advisory row maps to
// advisory in the standard sense, a downgraded finding keeps its cal_row and the
// caller validates it with the NON-escalated (or standard-band) lookup — the
// downgrade is a severity change the round records, and validation is the
// caller's step over the mutated round.
func Verify(ctx context.Context, findings []report.Finding, req VerifyRequest) (*VerifyResult, error) {
	// Dormant by default, and deep-band only: a no-op that spawns nothing.
	if !req.Config.Enabled || req.Band != report.BandDeep {
		return &VerifyResult{Findings: findings, Ran: false}, nil
	}

	// Is there anything to re-judge? Only blocking findings are re-judged; if
	// none, the enabled pass is still a no-op (nothing to downgrade).
	anyBlocking := false
	for i := range findings {
		if findings[i].Severity == report.SeverityBlocking {
			anyBlocking = true
			break
		}
	}
	if !anyBlocking {
		return &VerifyResult{Findings: findings, Ran: false}, nil
	}

	if req.Verifier == nil {
		return nil, fmt.Errorf("%w", ErrNilVerifier)
	}

	// The second instance must be INDEPENDENT — a fresh sibling of both the
	// implementer and the first reviewer. Reuse the runtime's provenance check
	// against each; a failed check fails closed (never re-judge over a
	// non-independent second instance).
	if err := runtime.CheckFreshInstance(req.SecondInstanceRoot, req.ImplementerRoot); err != nil {
		return nil, fmt.Errorf("impl-review band: second-instance vs implementer: %w", err)
	}
	if req.FirstReviewerRoot != "" {
		if err := runtime.CheckFreshInstance(req.SecondInstanceRoot, req.FirstReviewerRoot); err != nil {
			return nil, fmt.Errorf("impl-review band: second-instance vs first reviewer: %w", err)
		}
	}

	out := make([]report.Finding, len(findings))
	copy(out, findings)
	var downgraded []string

	for i := range out {
		f := &out[i]
		if f.Severity != report.SeverityBlocking {
			continue // advisory findings are never re-judged (one-directional).
		}
		confirmed, err := secondInstanceConfirms(ctx, req.Verifier, f, req.Pack)
		if err != nil {
			return nil, fmt.Errorf("impl-review band: second-instance re-judge of %q: %w", f.ID, err)
		}
		if !confirmed {
			// The second instance did not confirm — downgrade blocking → advisory.
			f.Severity = report.SeverityAdvisory
			downgraded = append(downgraded, f.ID)
		}
	}

	return &VerifyResult{Findings: out, Downgraded: downgraded, Ran: true}, nil
}

// secondInstanceConfirms asks the second-instance Verifier to re-judge one
// finding's claim over the shared pack and reports whether it CONFIRMS the
// finding (a non-empty candidate set). The Verifier is a runtime.LensRunner: its
// Run is invoked with the finding's own lens and its rule identity as the
// "instruction" (the concrete claim to re-check), the shared pack, and returns a
// confirming candidate iff the second instance agrees. An empty return is the
// no-confirm signal (downgrade).
func secondInstanceConfirms(ctx context.Context, v Verifier, f *report.Finding, pack []byte) (bool, error) {
	// The re-judge instruction is the finding's own claim — the second instance
	// re-checks THIS finding, not the whole diff fresh. Its rule identity + message
	// name what to confirm.
	instruction := "re-judge whether this blocking finding is a real defect: " + f.RuleID + " — " + f.Message
	candidates, err := v.Run(ctx, f.Lens, pack, instruction)
	if err != nil {
		return false, err
	}
	// Confirmed iff the second instance returned at least one candidate (it agrees
	// the finding is real). No candidates = not confirmed = downgrade.
	return len(candidates) > 0, nil
}
