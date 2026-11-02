package state

import (
	"context"
	"fmt"
	"strconv"

	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/docgate/incidents"
	"github.com/agenticlab-ai/pyxis/internal/docgate/schema"
	"github.com/agenticlab-ai/pyxis/internal/lane/policy"
)

// FloorViolation is a discovered defect: an AUTO-merged proposal later found to
// violate the floor — a change that should have hit the floor and routed HUMAN
// rode AUTO instead (the zero-tolerance M2 failure, merge-lane-v1 §1). It is the
// input to the kill trip (A-10, UB-8). Date is a YYYY-MM-DD argument, never a
// clock read, so the trip stays hermetic (the incident writer's discipline).
type FloorViolation struct {
	Proposal     string
	HeadRevision string
	FloorClass   string // the floor class the merged content should have hit
	RodeClass    string // the AUTO class it wrongly rode
	Ledger       string // the affected drive's delivery ledger, for the alarm
	Date         string // YYYY-MM-DD, an argument (hermeticity)
}

// KillTrip is the outcome of a floor-violation kill trip: the appended incident
// entry's dense ID and the kill directive the operator's surface / acting
// workflow applies to the policy. Applying the directive is a single data change
// (kill_switch → true, A-4) that flips the lane HUMAN-only; re-activation is
// then refused until fresh operator ratification (ReactivationRefused).
type KillTrip struct {
	// IncidentID is the dense ID of the incident entry the trip appended.
	IncidentID int
	// Directive is the kill the caller applies to the policy — the single data
	// change that flips the lane HUMAN-only.
	Directive KillDirective
}

// KillDirective is the kill the trip engages: set the policy's kill_switch true.
// T5 produces the directive; the write to the ratified policy file is the
// operator's surface / acting workflow (the layering the acting package uses for
// its Machine/Medium ports — T5 owns the logic and the data, the adapter owns
// the I/O). A directive never widens anything: it only stops acting.
type KillDirective struct {
	KillSwitch bool   // true — the lane goes HUMAN-only (A-4)
	Reason     string // the recorded reason, cold-readable years later (NFR-4)
}

// TripKill fires the kill switch on a discovered floor violation: it appends the
// paid-for lesson to the incident registry through the coded writer (never a
// hand-written entry) and — where an alarm port is wired — alarms on the
// affected drive's delivery ledger, then returns the kill directive the caller
// applies to the policy. It is fail-closed on the incident append: if the
// registry write fails, TripKill returns its defects and a zero KillTrip so the
// caller does not proceed as if the lesson were captured.
//
// The re-activation refusal is not TripKill's to enforce beyond producing the
// directive: once kill_switch is set, ReactivationRefused holds until a fresh
// ratified policy lands kill_switch=false — and the policy loader's
// ratified-lineage check (T1) is what makes that flip require the operator
// (DINV-8, §3.7). TripKill only ensures the lane stops and the lesson is
// recorded.
func TripKill(root string, cfg config.Config, alarm Alarm, fv FloorViolation) (KillTrip, []schema.Defect) {
	reason := killReason(fv)
	id, defects := incidents.Add(root, cfg, incidentEntry(fv, reason))
	if len(defects) > 0 {
		return KillTrip{}, defects
	}

	if alarm != nil {
		ev := AlarmEvent{Kind: AlarmKill, Ledger: fv.Ledger, Proposal: fv.Proposal, HeadRevision: fv.HeadRevision, Reason: reason}
		if err := alarm.Raise(context.Background(), ev); err != nil {
			return KillTrip{}, []schema.Defect{def("kill", fv.Proposal,
				"the kill incident I"+strconv.Itoa(id)+" was recorded, but raising the delivery-ledger alarm failed: "+err.Error(),
				"the kill directive still holds; re-raise the alarm (§3.4)")}
		}
	}

	return KillTrip{IncidentID: id, Directive: KillDirective{KillSwitch: true, Reason: reason}}, nil
}

// ReactivationRefused reports whether re-activation is refused for the given
// policy. A killed lane (kill_switch set) refuses re-activation: the only flip
// back to an enforcing lane is a FRESH ratified policy that lands
// kill_switch=false, and the policy loader's ratified-lineage check (T1, DINV-8)
// is the gate that makes that flip require the operator. While kill_switch is
// set, re-activation is refused and the defect names the required fresh
// ratification. A nil policy (a load defect) is refused too — the lane cannot
// enforce on a policy it could not load (fail closed).
func ReactivationRefused(p *policy.Policy) (bool, schema.Defect) {
	if p == nil {
		return true, def("reactivate", "risk-policy",
			"the lane cannot re-activate on a policy that failed to load",
			"land a well-formed, ratified policy first; a load defect keeps the lane HUMAN-only (DINV-8)")
	}
	if p.KillSwitch {
		return true, def("reactivate", "risk-policy",
			"the lane is killed (kill_switch is set); re-activation is refused until fresh operator ratification",
			"the operator lands a fresh ratified policy with kill_switch=false; the ratified-lineage check gates that flip (UB-8, §3.7)")
	}
	return false, schema.Defect{}
}

// killReason renders the kill's recorded reason from the violation.
func killReason(fv FloorViolation) string {
	return fmt.Sprintf("AUTO-merged proposal %s (head %s) was found to violate the floor class %q it should have hit; it rode the AUTO class %q",
		fv.Proposal, fv.HeadRevision, fv.FloorClass, fv.RodeClass)
}

// incidentEntry builds the incident capture for a floor-violation kill — the
// paid-for lesson (UB-8): a floor-violating change rode AUTO (the zero-tolerance
// M2 failure), so the kill switch trips and re-activation requires fresh
// operator ratification.
func incidentEntry(fv FloorViolation, reason string) incidents.Entry {
	return incidents.Entry{
		Date:     fv.Date,
		Tags:     []string{"merge-lane", "auto-merge", "floor-violation"},
		Decision: "The AUTO merge lane merged a proposal a class routed AUTO. " + reason + ".",
		Broke:    "A floor-violating change rode the AUTO lane — the zero-tolerance M2 failure the lane exists to prevent (merge-lane-v1 §1).",
		Lesson:   "A discovered floor violation trips the kill switch to a HUMAN-only lane; re-activation is refused until fresh operator ratification (UB-8, merge-lane-v1 §3.7).",
		Links:    "proposal " + fv.Proposal + " @ " + fv.HeadRevision + ".",
	}
}
