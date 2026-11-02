package wake

import (
	"context"
	"fmt"
)

// The closure discipline over in-flight proposals (docs/tech-design/
// fleet-bridge-v1.md §3.2, DINV-4; prd:argus-unattended-drive FR-6, FR-12).
// Closure is **mainline-content closure**: a feature closes only when every
// resulting proposal's content is verified an ancestor of the mainline — never
// on a platform badge (AP-26/I2). A stranded proposal — a merged badge with
// absent mainline content — blocks the feature's close and alarms on the
// delivery ledger. This package owns the landing acceptance, the full-set gate,
// and the stranded alarm; it does NOT route proposals (the lane's job, T2/T4)
// and introduces no new watcher over unclaimed items (they age under the
// existing bump arm — nothing new watches them).

// LandingChecker reads a proposal's landing at the goal object — the exact reads
// that decide acceptance (§3.2). It carries no credential and mutates nothing:
// `merged` is the platform's merged badge, `ancestor` is the goal-object truth
// (the merged content is an ancestor of the mainline). A merged badge whose
// content is not an ancestor is a stranded proposal — a lying badge (I2's
// class). The code-host/git implementation is the host adapter's; the tests
// substitute a stub.
type LandingChecker interface {
	Landing(ctx context.Context, proposal string) (merged bool, ancestor bool, err error)
}

// ProposalState is one resulting proposal's landing state.
type ProposalState string

const (
	// StateLanded — merged and its content verified an ancestor of the mainline;
	// the landing is accepted (FR-6).
	StateLanded ProposalState = "landed"
	// StateInFlight — not yet merged; the proposal is still being driven to a
	// landing through the lane's routing. Blocks closure, without alarm — it has
	// simply not landed yet.
	StateInFlight ProposalState = "in-flight"
	// StateStranded — a merged badge with content absent from the mainline; a
	// lying badge (I2's class). Blocks closure AND alarms (DINV-4).
	StateStranded ProposalState = "stranded"
)

// ProposalStatus is one proposal's resolved landing state.
type ProposalStatus struct {
	Proposal string
	State    ProposalState
}

// StrandedAlarm is a stranded-proposal alarm the drive raises on the affected
// delivery ledger — the notification layered over the load-bearing closure block
// (the block is the guarantee; the alarm surfaces it). It names the affected
// ledger, the stranded proposal, and why, so it is cold-readable years later.
type StrandedAlarm struct {
	Ledger   string
	Proposal string
	Reason   string
}

// ClosureResult is the closure gate's cold-readable outcome. CanClose is true
// only when every resulting proposal is a verified mainline-ancestry landing
// (DINV-4); Statuses is the per-proposal resolution; Stranded carries the
// stranded-proposal alarms the drive raises on the delivery ledger; InFlight
// names the proposals still being driven to a landing.
type ClosureResult struct {
	CanClose bool
	Statuses []ProposalStatus
	Stranded []StrandedAlarm
	InFlight []string
}

// EvaluateClosure gates a feature's close on the FULL set of resulting proposals:
// it accepts each landing only on verified mainline ancestry, and the feature
// closes only when every proposal is a verified landing (DINV-4). A stranded
// proposal (merged badge, absent content) blocks closure and produces an alarm;
// an in-flight proposal blocks closure without alarm. It returns a hard error
// only for an infrastructure fault (a landing read that errored); a stranded or
// in-flight proposal is a named state in the result, not an error.
//
// The proposals are the feature's resulting proposals — one per materialized
// item that has been claimed. Unclaimed items produce no proposal and are not
// this gate's concern: they age under the existing bump arm, and this gate reads
// only the proposals it is handed, introducing no new watcher over them.
func EvaluateClosure(ctx context.Context, ledger string, proposals []string, checker LandingChecker) (ClosureResult, error) {
	res := ClosureResult{CanClose: true}
	for _, p := range proposals {
		merged, ancestor, err := checker.Landing(ctx, p)
		if err != nil {
			return ClosureResult{}, fmt.Errorf("wake: reading the landing of proposal %q at the goal object: %w", p, err)
		}
		switch {
		case ancestor:
			// Accepted only at the goal object — content an ancestor of the
			// mainline, never a badge (FR-6, AP-26).
			res.Statuses = append(res.Statuses, ProposalStatus{Proposal: p, State: StateLanded})
		case merged && !ancestor:
			// A merged badge with absent mainline content — stranded (I2's class):
			// block closure and alarm.
			res.CanClose = false
			res.Statuses = append(res.Statuses, ProposalStatus{Proposal: p, State: StateStranded})
			res.Stranded = append(res.Stranded, StrandedAlarm{
				Ledger:   ledger,
				Proposal: p,
				Reason:   "proposal " + quote(p) + " carries a merged badge but its content is not an ancestor of the mainline — a stranded proposal; a landing is accepted only at the goal object, never on a badge (fleet-bridge-v1 DINV-4, incident I2)",
			})
		default:
			// Not merged yet — still being driven to a landing through the lane.
			res.CanClose = false
			res.Statuses = append(res.Statuses, ProposalStatus{Proposal: p, State: StateInFlight})
			res.InFlight = append(res.InFlight, p)
		}
	}
	return res, nil
}
