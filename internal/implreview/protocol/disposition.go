package protocol

import (
	"errors"
	"fmt"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/implreview/report"
	"github.com/agenticlab-ai/pyxis/internal/implreview/verdict"
)

// WorkerDisposition is the worker's disposition of one blocking finding, before
// it is folded into the payload (FR-8). A worker may record exactly two things:
//
//   - APPLIED — the worker applied a fix for the finding. This is an UNVERIFIED
//     claim: it does NOT resolve the blocker until a round-2 re-review confirms
//     the named defect closed (FR-15; see raise.go). verdict.resolvedAway
//     already refuses to let a worker `applied` resolve a blocker, so an applied
//     claim HOLDS the verdict at request-changes until verified.
//   - DISPUTED — the worker disputes the finding. A dispute MUST carry a reason;
//     a reason-less dispute VOIDS its round (UB-2). A disputed BLOCKING finding
//     is the FR-8 → FR-17 escalate condition: the fix is a spec/judgment
//     decision, not a code change, so it ESCALATES (Escalation below), surfaced
//     to the caller to feed verdict.Compute.
//
// The worker never records accepted/rejected — those are OPERATOR verdicts on a
// separate operator-authored artifact (§3.1/DINV-4), out of this worker path.
type WorkerDisposition struct {
	// FindingID is the FND-n the disposition is against.
	FindingID string
	// Applied is true when the worker applied a fix (an unverified claim); false
	// when the worker disputes the finding.
	Applied bool
	// Reason is the dispute reason — MANDATORY when Applied is false. A reason-less
	// dispute voids its round (ErrDisputeNoReason).
	Reason string
	// EscalationBrief is the FR-17 decision brief the worker attaches to a DISPUTE
	// of a blocking finding (the single conflict, the options, the decision
	// needed). Required for a disputed blocking finding — a dispute that escalates
	// with no actionable brief is refused downstream by verdict.Compute; ToRecord
	// carries it so the caller can build the verdict.Escalation.
	EscalationBrief report.EscalationBrief
}

// RecordResult is the outcome of recording a worker disposition against a
// finding (FR-8): the report.Disposition to fold into the payload, and — when
// the worker disputed a BLOCKING finding — the verdict.Escalation the caller
// feeds to verdict.Compute (FR-8 → FR-17). This package DETECTS the escalate
// condition; verdict (T7) ACTS on it.
type RecordResult struct {
	// Disposition is the report.Disposition to fold onto the finding in the
	// payload. For an applied claim it is a worker/applied disposition; for a
	// dispute it is a worker/declined disposition carrying the reason (which
	// report.Validate requires and which the reason-less-dispute rule guarantees
	// is non-empty here).
	Disposition report.Disposition
	// Escalation is non-nil iff the worker DISPUTED a BLOCKING finding — the
	// FR-8 → FR-17 escalate condition the caller hands to verdict.Compute as
	// Signals.Escalation. It is nil for an applied claim and for a dispute of an
	// advisory finding (an advisory dispute does not escalate — it holds nothing).
	Escalation *verdict.Escalation
}

// RecordDisposition records a worker's disposition of one finding (FR-8),
// enforcing the reason-less-dispute void (UB-2) and detecting the declined-
// blocking escalate condition (FR-8 → FR-17).
//
// finding is the finding being disposed (its severity decides whether a dispute
// escalates). wd is the worker's disposition. It returns:
//
//   - a report.Disposition to fold onto the finding, and
//   - a verdict.Escalation when the worker disputed a BLOCKING finding (nil
//     otherwise) — the caller feeds it to verdict.Compute.
//
// It fails closed:
//
//   - ErrDisputeNoReason when a dispute (Applied == false) carries no reason —
//     the round is VOID (the caller drops it; see VoidRound). A reason-less
//     dispute is not a disposition at all, so it never yields a payload
//     disposition.
//   - it never records an operator verdict (accepted/rejected) — this is the
//     worker path (§3.1/DINV-4).
//
// An APPLIED claim yields a worker/applied disposition and NO escalation — it is
// unverified and holds the verdict until a round-2 re-review confirms it
// (FR-15). A DISPUTE of a blocking finding yields a worker/declined disposition
// (Reason mandatory, carried) and an Escalation the caller acts on. A dispute of
// an ADVISORY finding yields a worker/declined disposition and no escalation (an
// advisory finding holds nothing).
func RecordDisposition(finding report.Finding, wd WorkerDisposition) (RecordResult, error) {
	if wd.Applied {
		// An applied claim: a worker/applied disposition, unverified, no escalate.
		return RecordResult{
			Disposition: report.Disposition{
				Actor:   report.ActorWorker,
				Verdict: report.VerdictApplied,
			},
		}, nil
	}

	// A dispute. It MUST carry a reason; a reason-less dispute voids its round.
	if strings.TrimSpace(wd.Reason) == "" {
		return RecordResult{}, fmt.Errorf("%w: finding %q", ErrDisputeNoReason, wd.FindingID)
	}

	// The worker/declined disposition (report.Validate requires the reason, which
	// the check above guarantees non-empty).
	disp := report.Disposition{
		Actor:   report.ActorWorker,
		Verdict: report.VerdictDeclined,
		Reason:  wd.Reason,
	}

	// A disputed BLOCKING finding is the FR-8 → FR-17 escalate condition: the fix
	// is a spec/judgment decision, not a code change. A disputed advisory finding
	// escalates nothing (it holds nothing).
	if finding.Severity == report.SeverityBlocking {
		brief := wd.EscalationBrief
		return RecordResult{
			Disposition: disp,
			Escalation:  &verdict.Escalation{Brief: brief},
		}, nil
	}

	return RecordResult{Disposition: disp}, nil
}

// VoidRound reports whether a recording error means the round is VOID (must be
// dropped, never counted) — the reason-less-dispute void (UB-2). A void round is
// not a recorded round; the caller discards it and does not advance the round
// count. Any other error is a different failure the caller handles per its
// contract.
//
// This is the predicate a caller uses after RecordDisposition returns an error
// to decide "void this round" vs "some other failure": errors.Is(err,
// ErrDisputeNoReason) is the void condition.
func VoidRound(err error) bool {
	return errors.Is(err, ErrDisputeNoReason)
}
