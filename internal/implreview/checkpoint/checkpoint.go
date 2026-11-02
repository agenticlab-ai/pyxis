package checkpoint

import (
	"fmt"

	"github.com/agenticlab-ai/pyxis/internal/implreview/harvest"
)

// Engagement is the disposition-engagement signal the checkpoint surfaces as an
// ADVISORY FLAG (§3.3, EVT-5) — NEVER an auto-fail. It is the operator's own
// dwell/accept-burst reading, observed OUTSIDE the harvested payload corpus (the
// harvest measures findings + dispositions; dwell and accept-burst are
// observations about HOW the operator engaged), so it is a checkpoint input, not
// a harvested field.
//
// The §3.3 rule is explicit and this type enforces it structurally: a burst
// signal is "an advisory flag, not an auto-fail; a flagged checkpoint is
// discussed, not auto-promoted". So [Evaluate] carries DwellP50 and BurstFlag
// into the EVT-5 [Record] and NEVER lets BurstFlag hold the ramp — the operator's
// recorded promotion act attests genuine engagement (§3.3, A2), not this flag.
type Engagement struct {
	// DwellP50 is the median operator dwell (a duration signal, opaque units the
	// harvest's EVT-5 dwell_p50 carries) — surfaced at the checkpoint, gating
	// nothing.
	DwellP50 float64
	// BurstFlag is true when the operator's dispositions look like an accept-burst
	// (bulk rubber-stamp accepts — §3.3, A2). It is an ADVISORY flag: it is
	// surfaced in the EVT-5 record for discussion, and it NEVER causes a hold or a
	// fail (only a real guardrail breach — nit / coverage — holds).
	BurstFlag bool
}

// Inputs is everything a checkpoint reads. The harvested corpus ([harvest.Metrics])
// is the coverage / precision / nit substrate — the checkpoint CONSUMES it and
// never recomputes a metric (T11 owns the computation; this reads it). M3 (evals
// green) and the disposition engagement come from OUTSIDE the harvest (the eval
// suite runs in CI — T12; engagement is an operator observation), so they are
// separate inputs. The policy carries the ship-rule thresholds as data.
type Inputs struct {
	// Reviews is the number of reviews this checkpoint window covers — the "ten-
	// review checkpoint" cadence counter (compared to Policy.WindowSize). It is the
	// count of COMPLETED reviews harvested into this window (Metrics.Reviews is the
	// corpus's review count; a caller passes the window's review count here, which
	// at a full harvest equals Metrics.Reviews).
	Reviews int
	// Metrics is the harvested corpus (T11) — coverage, blocking dispositions,
	// precision, nit. The checkpoint reads these; it recomputes nothing.
	Metrics harvest.Metrics
	// M3Green is whether the §5.9 eval suite is green under its run protocol
	// (M3 — from CI's EVT-4, not the harvest). The ship rule requires it.
	M3Green bool
	// NitP50 is the operator `rejected`-per-review median across the window (§3.3's
	// nit guardrail signal). It is a WINDOW p50 the caller supplies (the guardrail
	// is "≤ 3 per review at p50 across any five consecutive reviews"); the
	// harvest's NitRate is the corpus mean, a related but distinct reading, so the
	// p50 the guardrail names is passed explicitly.
	NitP50 float64
	// Engagement is the advisory dwell/burst signal (§3.3, EVT-5) — surfaced,
	// never gating.
	Engagement Engagement
	// Policy is the ship-rule thresholds as data (§3.2/§3.3). Zero value is treated
	// as DefaultPolicy (a caller that passes no policy gets the pre-registered
	// starting values).
	Policy Policy
}

// Outcome is the checkpoint's verdict about the RAMP — the three ways a §3.3
// checkpoint can land. It is distinct from the EVT-5 [Record] (the emitted data):
// the Outcome is the DECISION, the Record is the observation.
type Outcome string

const (
	// OutcomeShipRuleMet — every ship-rule term cleared (M1 met, M3 green, nit
	// guardrail clean, M2 ≥ threshold over the floor). The operator MAY now record
	// a promotion ([Gate.Promote]); the checkpoint does not promote (§3.3).
	OutcomeShipRuleMet Outcome = "ship-rule-met"
	// OutcomeExtendWindow — the disposition floor is UNMET (fewer than
	// Policy.DispositionFloor operator-disposed blocking findings), so M2 is
	// INCONCLUSIVE: the window EXTENDS to gather more disposed blocking findings —
	// never a vacuous pass (§3.3: "the disposition floor unmet → the checkpoint
	// re-runs on the next five reviews"). A zero-blocking window is this case too:
	// it HOLDS (extends), never passes vacuously.
	OutcomeExtendWindow Outcome = "extend-window"
	// OutcomeHoldAdvisory — a metric FAILED (a guardrail breach, M2 below
	// threshold over a MET floor, coverage short, evals not green, or the window
	// cadence not yet reached): the ramp HOLDS at advisory (§3.3: "any guardrail
	// breach → the ramp holds at advisory; M2 below threshold → calibration
	// iterates by data PR; the checkpoint re-runs on the next five reviews"). The
	// [Record.HoldReason] names why.
	OutcomeHoldAdvisory Outcome = "hold-advisory"
)

// Record is the EVT-5 adoption_checkpoint record (§5.7) — the machine payload a
// checkpoint emits, one per checkpoint evaluation. Its fields are EXACTLY the
// EVT-5 typed properties (reviews, coverage_pct, m3_green, blocking_disposed,
// precision?, nit_p50, dwell_p50, burst_flag, promoted), plus the derived
// [Record.Outcome] and [Record.HoldReason] that make the §3.3 decision legible.
//
// Precision is a POINTER (EVT-5's `precision: float?`): it is nil when the
// disposition floor was unmet — precision is NOT YET A BASIS below the floor, so
// the record carries no precision value rather than a misleading zero (the
// window extends instead). PrecisionOverFloor==false mirrors this for a caller
// that reads the struct fields directly.
type Record struct {
	// Reviews is the review count this checkpoint covered (EVT-5 reviews).
	Reviews int `json:"reviews"`
	// Coverage is M1 — report-or-skip present ÷ the full universe (EVT-5
	// coverage_pct), read from the harvest, in [0,1].
	Coverage float64 `json:"coverage_pct"`
	// M3Green is whether the eval suite is green under protocol (EVT-5 m3_green).
	M3Green bool `json:"m3_green"`
	// BlockingDisposed is the count of attributable operator-disposed blocking
	// findings the precision is over (EVT-5 blocking_disposed) — the floor is
	// checked against this.
	BlockingDisposed int `json:"blocking_disposed"`
	// Precision is M2 over the floor (EVT-5 precision?) — nil when the floor was
	// unmet (precision is not yet a basis; the window extends). Non-nil only when
	// BlockingDisposed ≥ the floor.
	Precision *float64 `json:"precision,omitempty"`
	// NitP50 is the operator `rejected`-per-review p50 (EVT-5 nit_p50) — the nit
	// guardrail signal.
	NitP50 float64 `json:"nit_p50"`
	// DwellP50 is the operator dwell median (EVT-5 dwell_p50) — advisory.
	DwellP50 float64 `json:"dwell_p50"`
	// BurstFlag is the accept-burst advisory flag (EVT-5 burst_flag) — surfaced,
	// never gating.
	BurstFlag bool `json:"burst_flag"`
	// Promoted is whether the reviewer is promoted AS OF this checkpoint (EVT-5
	// promoted). The checkpoint EVALUATES eligibility; it never sets this true by
	// itself — a caller stamps it from the gate's state (see [Evaluate], which
	// leaves it false: promotion is the operator's recorded act, not a checkpoint
	// side effect).
	Promoted bool `json:"promoted"`

	// --- derived decision fields (not EVT-5 wire props; the §3.3 verdict) ---

	// ShipRuleMet is true iff every ship-rule term cleared (§3.3): M1 met, M3
	// green, nit clean, M2 ≥ threshold over the floor. It is the eligibility the
	// operator's [Gate.Promote] requires — a promotion on a Record whose
	// ShipRuleMet is false is refused (a guardrail breach holds the ramp).
	ShipRuleMet bool `json:"ship_rule_met"`
	// Outcome is the §3.3 decision: ship-rule-met, extend-window (floor unmet), or
	// hold-advisory (a breach / cadence-not-reached).
	Outcome Outcome `json:"outcome"`
	// HoldReason names WHY the ramp held or the window extended — an empty string
	// when ShipRuleMet. It is the legible cause a discussion reads (§3.3: "a
	// flagged checkpoint is discussed").
	HoldReason string `json:"hold_reason,omitempty"`
	// PrecisionOverFloor is true iff BlockingDisposed ≥ the floor (so Precision is
	// non-nil and M2 is a basis). Mirrors the nil-ness of Precision for a caller
	// reading fields directly.
	PrecisionOverFloor bool `json:"precision_over_floor"`
}

// Evaluate runs a ten-review adoption checkpoint over the harvested corpus and
// the out-of-harvest signals, returning the EVT-5 [Record] with the §3.3 ship-
// rule decision (FR-11, DOD-4). It COMPUTES the metrics FROM the harvest (never
// recomputing them — [harvest.Metrics] owns coverage/precision/nit; this reads
// them via the same [harvest.Metrics.Coverage]/[harvest.Metrics.Precision]
// methods T11 exposes), evaluates the pre-registered ship rule, and surfaces the
// engagement signal as an advisory flag.
//
// The decision ladder (§3.3), in order:
//
//  1. The window must have reached the checkpoint CADENCE (Reviews ≥
//     WindowSize). A window short of the cadence HOLDS (the ten-review
//     checkpoint has not arrived) — never a vacuous early pass.
//  2. The disposition FLOOR governs M2's evaluability. Below the floor (fewer
//     than DispositionFloor operator-disposed blocking findings — INCLUDING a
//     zero-blocking window) precision is NOT A BASIS: the window EXTENDS
//     ([OutcomeExtendWindow]), the Record carries a nil Precision, and the ship
//     rule is NOT met — never a vacuous pass on an empty precision denominator.
//  3. With the floor MET, every ship-rule term is checked: M1 (coverage == 100%),
//     M3 (evals green), the nit guardrail (nit p50 ≤ the bound), and M2
//     (precision ≥ the threshold). All clear → [OutcomeShipRuleMet]; any fails →
//     [OutcomeHoldAdvisory] naming the failing term.
//
// The engagement signal is ALWAYS carried into the record (dwell + burst) and
// NEVER affects the outcome — a burst flag is advisory (§3.3). The Record's
// Promoted stays false: promotion is the operator's recorded [Gate.Promote] act,
// not a checkpoint side effect.
func Evaluate(in Inputs) Record {
	pol := in.Policy
	if pol == (Policy{}) {
		pol = DefaultPolicy()
	}

	// Read the metrics FROM the harvest — never recompute. Coverage and precision
	// come through the harvest's own methods (the single source of the computation,
	// T11); the floor is applied via the harvest's Precision(floor) contract.
	m := in.Metrics
	coverage := m.Coverage()
	precisionVal, floorMet := m.Precision(pol.DispositionFloor)

	rec := Record{
		Reviews:          in.Reviews,
		Coverage:         coverage,
		M3Green:          in.M3Green,
		BlockingDisposed: m.BlockingDisposed,
		NitP50:           in.NitP50,
		DwellP50:         in.Engagement.DwellP50,
		BurstFlag:        in.Engagement.BurstFlag,
		Promoted:         false, // promotion is the operator's recorded act, never a checkpoint side effect
	}

	// Precision is a basis ONLY over the floor (EVT-5 precision?): carry it iff the
	// floor is met, else leave it nil (not a misleading zero — the window extends).
	if floorMet {
		p := precisionVal
		rec.Precision = &p
		rec.PrecisionOverFloor = true
	}

	// (1) The checkpoint cadence must be reached — a window short of it holds
	// (the ten-review checkpoint has not arrived), never a vacuous early pass.
	if in.Reviews < pol.WindowSize {
		rec.Outcome = OutcomeHoldAdvisory
		rec.ShipRuleMet = false
		rec.HoldReason = fmt.Sprintf("checkpoint cadence not reached: %d of %d reviews", in.Reviews, pol.WindowSize)
		return rec
	}

	// (2) The disposition FLOOR governs M2. Below it (including a zero-blocking
	// window) precision is inconclusive → EXTEND the window, never a vacuous pass.
	if !floorMet {
		rec.Outcome = OutcomeExtendWindow
		rec.ShipRuleMet = false
		rec.HoldReason = fmt.Sprintf("disposition floor unmet: %d of %d operator-disposed blocking findings — precision inconclusive, window extends (never a vacuous pass)", m.BlockingDisposed, pol.DispositionFloor)
		return rec
	}

	// (3) The floor is met — evaluate every ship-rule term (§3.3). Any failure
	// HOLDS the ramp at advisory, naming the failing term; a guardrail breach
	// (nit) is a hold too. Order: M1, M3, nit guardrail, M2.
	switch {
	case coverage < 1.0:
		// M1: 100% factory-proposal coverage (a report or a recorded skip). A
		// never-reviewed proposal counts against it (the harvest's contract).
		rec.Outcome = OutcomeHoldAdvisory
		rec.ShipRuleMet = false
		rec.HoldReason = fmt.Sprintf("M1 coverage below 100%%: %.1f%% (%d of %d proposals covered)", coverage*100, m.Covered, m.Universe)
	case !in.M3Green:
		// M3: the eval suite green under protocol.
		rec.Outcome = OutcomeHoldAdvisory
		rec.ShipRuleMet = false
		rec.HoldReason = "M3 eval suite not green under the §5.9 run protocol"
	case in.NitP50 > pol.NitP50Max:
		// The nit guardrail (§3.3): a breach pauses adoption progress — the ramp
		// holds; calibration iterates by data PR, never a silent retune.
		rec.Outcome = OutcomeHoldAdvisory
		rec.ShipRuleMet = false
		rec.HoldReason = fmt.Sprintf("nit guardrail breached: p50 %.2f rejected/review > %.2f — adoption holds, calibration iterates by data PR (never silent)", in.NitP50, pol.NitP50Max)
	case precisionVal < pol.PrecisionThreshold:
		// M2: precision over the floor must meet the threshold. Below it,
		// calibration iterates by data PR and the checkpoint re-runs (§3.3) — the
		// ramp holds meanwhile.
		rec.Outcome = OutcomeHoldAdvisory
		rec.ShipRuleMet = false
		rec.HoldReason = fmt.Sprintf("M2 precision below threshold over the floor: %.2f < %.2f — calibration iterates by data PR; the checkpoint re-runs", precisionVal, pol.PrecisionThreshold)
	default:
		// Every term cleared — the ship rule is MET. The operator MAY now record a
		// promotion; the checkpoint does not promote (§3.3).
		rec.Outcome = OutcomeShipRuleMet
		rec.ShipRuleMet = true
		rec.HoldReason = ""
	}

	return rec
}
