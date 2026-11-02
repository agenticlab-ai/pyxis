package state_test

import (
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/lane/policy"
	"github.com/agenticlab-ai/pyxis/internal/lane/state"
)

func floorViolation() state.FloorViolation {
	return state.FloorViolation{
		Proposal:     "p42",
		HeadRevision: "head-sha",
		FloorClass:   "governing-doc-edit",
		RodeClass:    "drive-doc",
		Ledger:       "docs/problems/x/delivery.md",
		Date:         "2026-07-15",
	}
}

// A-10 / UB-8: a discovered floor violation trips the kill switch, appends the
// incident entry through the coded writer, alarms on the delivery ledger, and
// yields the kill directive that flips the lane HUMAN-only.
func TestTripKill_AppendsIncident_EngagesKill(t *testing.T) {
	root, cfg := incidentFixture(t)
	alarm := &memAlarm{}

	trip, defects := state.TripKill(root, cfg, alarm, floorViolation())
	if len(defects) != 0 {
		t.Fatalf("TripKill reported defects: %+v", defects)
	}
	// The incident is appended densely (I1 pre-exists → I2), and the lesson names
	// the floor violation.
	if trip.IncidentID != 2 {
		t.Fatalf("incident ID = %d, want the next dense I2", trip.IncidentID)
	}
	reg := readRegistry(t, root, cfg)
	if !strings.Contains(reg, "**I2 (2026-07-15)") || !strings.Contains(reg, "floor-violation") {
		t.Fatalf("the incident entry was not appended: %s", reg)
	}
	if !strings.Contains(reg, "fresh operator ratification") {
		t.Fatalf("the incident lesson does not name the re-activation rule: %s", reg)
	}
	// The kill directive flips the lane HUMAN-only.
	if !trip.Directive.KillSwitch {
		t.Fatal("the trip did not engage the kill switch")
	}
	// The kill alarmed on the delivery ledger.
	if len(alarm.events) != 1 || alarm.events[0].Kind != state.AlarmKill {
		t.Fatalf("no kill alarm on the delivery ledger: %+v", alarm.events)
	}
}

// re-activation is refused while the lane is killed; only a fresh ratified
// policy that lands kill_switch=false permits it.
func TestReactivationRefused_UntilFreshRatification(t *testing.T) {
	killed := &policy.Policy{SchemaVersion: policy.SchemaVersion, KillSwitch: true}
	refused, d := state.ReactivationRefused(killed)
	if !refused {
		t.Fatal("a killed lane was not refused re-activation")
	}
	if !strings.Contains(d.Reason, "kill_switch is set") {
		t.Fatalf("refusal does not name the kill: %+v", d)
	}

	// A fresh ratified policy with the kill cleared permits re-activation.
	fresh := &policy.Policy{SchemaVersion: policy.SchemaVersion, KillSwitch: false}
	if refused, _ := state.ReactivationRefused(fresh); refused {
		t.Fatal("a fresh ratified policy with kill_switch=false was still refused re-activation")
	}

	// A nil policy (a load defect) is refused too — fail closed.
	if refused, _ := state.ReactivationRefused(nil); !refused {
		t.Fatal("a policy that failed to load was not refused re-activation")
	}
}

// A kill trip onto a red/unreadable registry is fail-closed: the incident write
// fails, TripKill reports its defects, and no kill directive is returned as if
// the lesson were captured.
func TestTripKill_FailClosedOnIncidentWriteFailure(t *testing.T) {
	root := t.TempDir()
	cfg := config.Defaults() // no registry file exists under this root → the append fails
	trip, defects := state.TripKill(root, cfg, &memAlarm{}, floorViolation())
	if len(defects) == 0 {
		t.Fatal("a failed incident append produced no defects")
	}
	if trip.Directive.KillSwitch || trip.IncidentID != 0 {
		t.Fatalf("a failed incident write still returned a live trip: %+v", trip)
	}
}
