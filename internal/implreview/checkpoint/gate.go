package checkpoint

import (
	"fmt"

	"github.com/agenticlab-ai/pyxis/internal/implreview/report"
)

// State is the adoption ramp's two states — the whole authority ladder of the
// advisory→adopted ramp (FR-11). It is deliberately a CLOSED two-value set: a
// verdict either gates nothing (advisory) or may gate a raise (adopted). There
// is no third "partial" state — promotion is one recorded operator flip.
type State string

const (
	// StateAdvisory is the DEFAULT, un-promoted state: the verdict gates NOTHING.
	// Findings deliver and persist, but PR-raise and lane routing proceed
	// regardless of the verdict (FR-11). The reviewer ramps here until the ship
	// rule is met AND the operator records a promotion.
	StateAdvisory State = "advisory"
	// StateAdopted is the promoted state: a downgrade-only verdict MAY now gate a
	// raise (request-changes / escalate blocks the raise; escalate forces the
	// human lane). Reached ONLY by a recorded operator [Gate.Promote] act — never
	// automatically (§3.3: promotion is an explicit operator act).
	StateAdopted State = "adopted"
)

// Gate is the advisory ramp for ONE repo — the state a caller consults before
// letting a reviewer verdict block a PR-raise or route a merge lane (FR-11). It
// is PER REPO (§6.5: promotion is per-repo — the anti-segment, a repo without
// calibration data, stays advisory) and DEFAULT OFF (the zero value is
// [StateAdvisory], so a Gate a caller never promoted gates nothing).
//
// The gate models FR-11's rule as a decision a caller reads, not an effect this
// package performs: [Gate.RaiseBlocked] and [Gate.LaneRoutingBlocked] answer
// "may this verdict block?" and the HOST acts on the answer. In advisory both
// answer false for EVERY verdict; only after a recorded [Gate.Promote] does a
// downgrade-only verdict block.
//
// Promotion is a RECORDED operator act ([Gate.Promote]) — the gate carries who
// promoted it and the checkpoint that justified it, so a promotion is auditable,
// never a silent default flip. A guardrail breach never lands here (this type
// holds no metric); the checkpoint ([Evaluate]) is where a breach HOLDS the ramp
// at advisory by refusing to report the ship rule met — a caller must not
// promote a Gate whose checkpoint did not clear.
type Gate struct {
	// repo is the repository this ramp governs — promotion is per-repo, so a Gate
	// is bound to one repo and never speaks for another.
	repo string
	// state is the ramp state — StateAdvisory until a recorded Promote. The zero
	// value ("") is treated as StateAdvisory (see effectiveState), so a
	// freshly-constructed or zero Gate gates nothing by default.
	state State
	// promotion records the operator act that promoted this gate — nil while
	// advisory. It is the audit trail: who promoted, and the checkpoint that
	// justified it. A promoted gate always has a non-nil promotion.
	promotion *Promotion
}

// Promotion is the RECORDED operator promotion act (§3.3): who flipped the ramp
// to adopted, and the checkpoint that justified the flip. It is the audit trail
// FR-11 requires — a promotion is "an explicit operator act recorded", never a
// default. The checkpoint's [Record] is retained so a later reader can see the
// metrics that were on the table when the operator promoted.
type Promotion struct {
	// Operator is the identity of the operator who recorded the promotion (a
	// medium login). It is required — a promotion with no operator is not a
	// recorded operator act.
	Operator string
	// Checkpoint is the checkpoint record that justified the promotion — the
	// coverage/precision/nit/engagement reading the operator saw. Retained for
	// audit; a promotion should cite a checkpoint whose ShipRuleMet is true, but
	// the operator is the principal (the gate records the act; it does not
	// re-adjudicate the operator's judgment — §3.3's "the operator's recorded
	// promotion act attests genuine engagement").
	Checkpoint Record
}

// NewGate builds the DEFAULT-OFF advisory gate for a repo (FR-11): a fresh gate
// is always [StateAdvisory] — it gates nothing until a recorded promotion. The
// repo binds the ramp; promotion is per-repo (§6.5).
func NewGate(repo string) *Gate {
	return &Gate{repo: repo, state: StateAdvisory}
}

// Repo is the repository this ramp governs.
func (g *Gate) Repo() string { return g.repo }

// effectiveState treats the zero value as advisory, so a zero Gate (never
// constructed via NewGate) still gates nothing by default — the fail-safe
// default FR-11 demands (nothing gates until an explicit promotion).
func (g *Gate) effectiveState() State {
	if g.state == StateAdopted {
		return StateAdopted
	}
	return StateAdvisory
}

// State reports the ramp's current state — StateAdvisory until a recorded
// Promote (the zero value reads as StateAdvisory).
func (g *Gate) State() State { return g.effectiveState() }

// Adopted reports whether the ramp has been promoted — a convenience over
// State() == StateAdopted.
func (g *Gate) Adopted() bool { return g.effectiveState() == StateAdopted }

// Promotion returns the recorded promotion act, or nil while advisory. It is the
// audit trail (who promoted, on which checkpoint).
func (g *Gate) Promotion() *Promotion { return g.promotion }

// ErrAlreadyPromoted is returned by [Gate.Promote] when the gate is already
// adopted — a promotion is a one-way flip per ramp; a second promotion is a
// caller error (revert-to-advisory is the kill switch, [Gate.Revert], §6.5).
var ErrAlreadyPromoted = fmt.Errorf("checkpoint: gate already promoted")

// Promote records an operator promotion act, flipping the ramp to [StateAdopted]
// (§3.3, FR-11). This is the ONLY path to adoption — there is no automatic
// promotion; a caller records the operator's own act here. The operator identity
// is required (a promotion with no operator is not a recorded operator act), and
// the justifying checkpoint is retained for audit.
//
// The gate does NOT re-adjudicate whether the ship rule was met — the operator
// is the principal (§3.3). But a defensive contract holds: promoting on a
// checkpoint whose ShipRuleMet is false is a caller mistake the gate refuses,
// because FR-11's promotion is "recorded PER THE §3.3 SHIP RULE" — a promotion
// with no clearing checkpoint is not that. A guardrail breach thus HOLDS the
// ramp at advisory by construction: [Evaluate] never reports the ship rule met on
// a breach, so Promote refuses.
func (g *Gate) Promote(operator string, justifiedBy Record) error {
	if g.effectiveState() == StateAdopted {
		return fmt.Errorf("%w: %s", ErrAlreadyPromoted, g.repo)
	}
	if operator == "" {
		return fmt.Errorf("checkpoint: Promote requires an operator identity (a promotion is a recorded operator act)")
	}
	if !justifiedBy.ShipRuleMet {
		return fmt.Errorf("checkpoint: refusing to promote %s on a checkpoint whose ship rule is not met (%s) — a guardrail breach holds the ramp at advisory (§3.3)", g.repo, justifiedBy.HoldReason)
	}
	g.state = StateAdopted
	g.promotion = &Promotion{Operator: operator, Checkpoint: justifiedBy}
	return nil
}

// Revert returns the ramp to advisory — the §6.5 kill switch ("revert the
// adoption flip, back to advisory"). It is idempotent: reverting an already-
// advisory gate is a no-op. After a revert the gate gates nothing again, and a
// later Promote may re-adopt on a fresh clearing checkpoint.
func (g *Gate) Revert() {
	g.state = StateAdvisory
	g.promotion = nil
}

// RaiseBlocked reports whether a proposal's PR-raise is BLOCKED by the given
// live verdict under this ramp (FR-11). It is the exact FR-11 predicate:
//
//   - In [StateAdvisory] (the default) it returns FALSE for EVERY verdict — the
//     verdict gates nothing, so a request-changes does not block a raise. This is
//     the advisory contract: findings deliver and persist, but raise proceeds
//     regardless.
//   - In [StateAdopted] it returns true iff the verdict is a downgrade
//     (request-changes or escalate) — a downgrade-only verdict now blocks the
//     raise (post-adoption; UB-6's "a raise on a stale/blocking verdict is
//     refused"). approve / approve-with-advisories never block (they withhold a
//     downgrade — they do not grant AUTO, they simply do not force one).
//
// The HOST consults this and acts; this package performs no raise and no block.
func (g *Gate) RaiseBlocked(verdict string) bool {
	if g.effectiveState() != StateAdopted {
		// Advisory: the verdict gates nothing — raise proceeds regardless (FR-11).
		return false
	}
	return blocks(verdict)
}

// LaneRoutingBlocked reports whether the given live verdict forces a lane
// downgrade (routes the proposal to the HUMAN lane / holds it off AUTO) under
// this ramp (FR-11, F3). Like [Gate.RaiseBlocked]:
//
//   - Advisory (default): FALSE for every verdict — lane routing proceeds
//     regardless (the verdict annotates without gating).
//   - Adopted: true iff the verdict is a downgrade (request-changes forces
//     return-to-worker; escalate forces the human lane). approve /
//     approve-with-advisories never route (they withhold a downgrade).
//
// Consuming the downgrade signal into the merge lane's Downgrader seam is the
// owed lane amendment (design §3.6 Q4); this predicate only tells a caller
// whether, under the ramp, the verdict is ALLOWED to gate lane routing at all.
func (g *Gate) LaneRoutingBlocked(verdict string) bool {
	if g.effectiveState() != StateAdopted {
		// Advisory: the verdict routes nothing — lane routing proceeds regardless.
		return false
	}
	return blocks(verdict)
}

// blocks reports whether a live verdict is a DOWNGRADE that may gate a raise /
// force a lane once adopted — mirroring the verdict package's downgrade-only
// authority (request-changes returns-to-worker; escalate forces-human) WITHOUT
// importing it (the checkpoint reads the persisted verdict string a report
// carries; it does not depend on the verdict computation). approve and
// approve-with-advisories withhold a downgrade and never block; an unknown
// verdict string fails SAFE (does not block), because a value this ramp cannot
// recognize must not silently gate the hottest path.
func blocks(verdict string) bool {
	switch verdict {
	case report.VerdictRequestChanges, report.VerdictEscalate:
		return true
	case report.VerdictApprove, report.VerdictApproveWithAdvisories:
		return false
	default:
		// An unrecognized verdict never gates — fail safe, never fail blocking.
		return false
	}
}
