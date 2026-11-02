package checkpoint_test

import (
	"errors"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/implreview/checkpoint"
	"github.com/agenticlab-ai/pyxis/internal/implreview/report"
)

// clearingRecord is a ship-rule-met checkpoint record — the justification a
// legitimate promotion cites. A guardrail-breach record (ShipRuleMet false) is
// the other case the Promote contract must refuse.
func clearingRecord() checkpoint.Record {
	return checkpoint.Record{ShipRuleMet: true, Outcome: checkpoint.OutcomeShipRuleMet}
}

func breachRecord(reason string) checkpoint.Record {
	return checkpoint.Record{
		ShipRuleMet: false,
		Outcome:     checkpoint.OutcomeHoldAdvisory,
		HoldReason:  reason,
	}
}

// TestAdvisoryGateBlocksNeitherRaiseNorLaneRouting — the FR-11 core: BEFORE a
// promotion the verdict gates NOTHING. Every verdict — including request-changes
// and escalate, the two downgrade verdicts — blocks neither PR-raise nor lane
// routing while the gate is advisory (its default). Findings still deliver and
// persist (this package holds no delivery/persist; that a request-changes does
// not BLOCK is the whole advisory contract this asserts).
func TestAdvisoryGateBlocksNeitherRaiseNorLaneRouting(t *testing.T) {
	t.Parallel()

	// A freshly-constructed gate AND a zero-value gate are both advisory by
	// default — the fail-safe default (nothing gates until an explicit promotion).
	constructed := checkpoint.NewGate("agenticlab-ai/pyxis")
	var zero checkpoint.Gate

	for _, g := range []*checkpoint.Gate{constructed, &zero} {
		if g.State() != checkpoint.StateAdvisory {
			t.Fatalf("default gate state = %q, want advisory", g.State())
		}
		if g.Adopted() {
			t.Fatalf("default gate reports adopted; want advisory")
		}

		// Every verdict blocks nothing in advisory — the request-changes and
		// escalate rows are the load-bearing ones (a downgrade verdict that would
		// block post-adoption must NOT block pre-adoption).
		for _, verdict := range []string{
			report.VerdictApprove,
			report.VerdictApproveWithAdvisories,
			report.VerdictRequestChanges,
			report.VerdictEscalate,
		} {
			if g.RaiseBlocked(verdict) {
				t.Errorf("advisory gate blocked PR-raise on verdict %q; want NOT blocked (FR-11: raise proceeds regardless)", verdict)
			}
			if g.LaneRoutingBlocked(verdict) {
				t.Errorf("advisory gate blocked lane routing on verdict %q; want NOT blocked (FR-11: lane routing proceeds regardless)", verdict)
			}
		}
	}
}

// TestPromotionIsRecordedDefaultOffPerRepoOperatorAct — promotion is a RECORDED,
// DEFAULT-OFF, PER-REPO operator act (§3.3, FR-11). The default is advisory; a
// promotion requires an operator identity and a clearing checkpoint; and a
// promoted gate is bound to its own repo. Only AFTER a recorded promotion does a
// downgrade verdict gate.
func TestPromotionIsRecordedDefaultOffPerRepoOperatorAct(t *testing.T) {
	t.Parallel()

	g := checkpoint.NewGate("agenticlab-ai/pyxis")

	// Default OFF: nothing is promoted, no promotion record exists.
	if g.Promotion() != nil {
		t.Fatalf("fresh gate carries a promotion; want nil (default off)")
	}

	// A promotion is a RECORDED operator act — it needs an operator identity.
	if err := g.Promote("", clearingRecord()); err == nil {
		t.Fatalf("Promote(\"\", clearing) succeeded; want an error (a promotion is a recorded OPERATOR act)")
	}
	if g.Adopted() {
		t.Fatalf("gate adopted after a rejected operator-less promotion; want still advisory")
	}

	// A valid recorded promotion flips to adopted and records WHO + the justifying
	// checkpoint (the audit trail).
	if err := g.Promote("mdarabi", clearingRecord()); err != nil {
		t.Fatalf("Promote(operator, clearing) failed: %v", err)
	}
	if g.State() != checkpoint.StateAdopted || !g.Adopted() {
		t.Fatalf("gate state = %q after promotion; want adopted", g.State())
	}
	promo := g.Promotion()
	if promo == nil || promo.Operator != "mdarabi" {
		t.Fatalf("promotion record = %+v; want operator mdarabi recorded", promo)
	}
	if !promo.Checkpoint.ShipRuleMet {
		t.Fatalf("promotion did not retain its justifying checkpoint")
	}

	// POST-adoption a downgrade verdict now gates; approve verdicts still do not.
	if !g.RaiseBlocked(report.VerdictRequestChanges) {
		t.Errorf("adopted gate did not block raise on request-changes; want blocked")
	}
	if !g.RaiseBlocked(report.VerdictEscalate) {
		t.Errorf("adopted gate did not block raise on escalate; want blocked")
	}
	if !g.LaneRoutingBlocked(report.VerdictEscalate) {
		t.Errorf("adopted gate did not force lane routing on escalate; want blocked")
	}
	if g.RaiseBlocked(report.VerdictApprove) {
		t.Errorf("adopted gate blocked raise on approve; want NOT blocked (approve withholds a downgrade, never grants AUTO)")
	}
	if g.RaiseBlocked(report.VerdictApproveWithAdvisories) {
		t.Errorf("adopted gate blocked raise on approve-with-advisories; want NOT blocked")
	}

	// A second promotion is refused (one-way flip per ramp).
	if err := g.Promote("mdarabi", clearingRecord()); !errors.Is(err, checkpoint.ErrAlreadyPromoted) {
		t.Fatalf("second Promote error = %v; want ErrAlreadyPromoted", err)
	}

	// PER-REPO: a different repo's gate is independent — promoting one never
	// promotes the other.
	other := checkpoint.NewGate("agenticlab-ai/harbormaster")
	if other.Adopted() {
		t.Fatalf("a second repo's gate is adopted because the first was; want independent (per-repo)")
	}
	if other.RaiseBlocked(report.VerdictRequestChanges) {
		t.Fatalf("a second repo's advisory gate blocks; promotion leaked across repos")
	}
}

// TestGuardrailBreachHoldsRampAtAdvisory — a guardrail breach HOLDS the ramp at
// advisory (§3.3). The mechanism: Evaluate never reports ShipRuleMet on a breach,
// and Promote REFUSES to promote on a non-clearing checkpoint — so a breach
// cannot be promoted through, the ramp holds, and the verdict keeps gating
// nothing.
func TestGuardrailBreachHoldsRampAtAdvisory(t *testing.T) {
	t.Parallel()

	g := checkpoint.NewGate("agenticlab-ai/pyxis")

	// A breach checkpoint (ship rule NOT met) cannot promote the ramp.
	if err := g.Promote("mdarabi", breachRecord("nit guardrail breached")); err == nil {
		t.Fatalf("Promote on a breach checkpoint succeeded; want refused (a guardrail breach holds the ramp at advisory)")
	}
	if g.Adopted() {
		t.Fatalf("gate adopted on a breach checkpoint; want held at advisory")
	}
	// The ramp is still advisory: a downgrade verdict still gates nothing.
	if g.RaiseBlocked(report.VerdictRequestChanges) {
		t.Fatalf("held-advisory gate blocks a raise; want still advisory (gates nothing)")
	}

	// Once a clearing checkpoint exists, promotion proceeds — proving the hold was
	// the breach, not an inability to ever promote.
	if err := g.Promote("mdarabi", clearingRecord()); err != nil {
		t.Fatalf("Promote on a clearing checkpoint failed: %v", err)
	}
	if !g.Adopted() {
		t.Fatalf("gate not adopted after a clearing promotion")
	}
}

// TestRevertKillSwitchReturnsToAdvisory — the §6.5 kill switch: reverting the
// adoption flip returns the ramp to advisory (verdicts gate nothing again), and a
// later promotion may re-adopt.
func TestRevertKillSwitchReturnsToAdvisory(t *testing.T) {
	t.Parallel()

	g := checkpoint.NewGate("agenticlab-ai/pyxis")
	if err := g.Promote("mdarabi", clearingRecord()); err != nil {
		t.Fatalf("Promote failed: %v", err)
	}
	if !g.Adopted() {
		t.Fatalf("gate not adopted before revert")
	}

	g.Revert()
	if g.State() != checkpoint.StateAdvisory || g.Adopted() {
		t.Fatalf("gate state = %q after revert; want advisory", g.State())
	}
	if g.Promotion() != nil {
		t.Fatalf("promotion record survived a revert; want cleared")
	}
	if g.RaiseBlocked(report.VerdictRequestChanges) {
		t.Fatalf("reverted gate still blocks a raise; want advisory (gates nothing)")
	}

	// Revert is idempotent, and re-promotion works.
	g.Revert()
	if err := g.Promote("mdarabi", clearingRecord()); err != nil {
		t.Fatalf("re-Promote after revert failed: %v", err)
	}
	if !g.Adopted() {
		t.Fatalf("gate not re-adopted after revert + promote")
	}
}
