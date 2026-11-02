package state_test

import (
	"context"
	"errors"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/lane/state"
)

// A-6 / DINV-7: a goal-verify failure writes the halt synchronously to the
// repo-resident record and alarms on the delivery ledger. After the write, a
// fresh Resolve over the store's block reads the lane halted — the "read at
// every subsequent act gate" property.
func TestWriteHalt_SynchronousThenReadHalted(t *testing.T) {
	store := &memStore{writer: machineWriter}
	alarm := &memAlarm{}
	ledger := "docs/problems/x/delivery.md"

	if err := state.WriteHalt(context.Background(), store, alarm, ledger, "p1", "head-sha", "merged badge, absent mainline content"); err != nil {
		t.Fatalf("WriteHalt: %v", err)
	}

	// Synchronous: the block grew before WriteHalt returned.
	if store.appends != 1 {
		t.Fatalf("halt was not written synchronously (appends=%d)", store.appends)
	}

	// The alarm fired on the delivery ledger.
	if len(alarm.events) != 1 || alarm.events[0].Kind != state.AlarmHalt || alarm.events[0].Ledger != ledger {
		t.Fatalf("no halt alarm on the delivery ledger: %+v", alarm.events)
	}
	if alarm.events[0].Proposal != "p1" {
		t.Fatalf("alarm does not name the proposal: %+v", alarm.events[0])
	}

	// Read at the next act gate: the lane is halted.
	b, w, _ := store.ReadState(context.Background())
	res, err := state.Resolve(context.Background(), b, w, newDisp())
	if err != nil {
		t.Fatalf("resolve after halt: %v", err)
	}
	if !res.Halted || res.ActiveHalt == nil || res.ActiveHalt.Proposal != "p1" {
		t.Fatalf("the written halt is not read at the next act gate: %+v", res)
	}
}

// A failed record write is fail-closed: WriteHalt returns the error and does not
// silently proceed as if the lane were safely halted.
func TestWriteHalt_RecordWriteFailsClosed(t *testing.T) {
	store := &memStore{writer: machineWriter, appendErr: errors.New("ref write rejected")}
	if err := state.WriteHalt(context.Background(), store, &memAlarm{}, "l", "p1", "head-sha", "badge lied"); err == nil {
		t.Fatal("a failed halt write did not surface — the lane must fail closed (DINV-7)")
	}
}

// A failed alarm surfaces, but the halt is already durable — the circuit breaker
// holds even when the notification does not land.
func TestWriteHalt_AlarmFailsButHaltDurable(t *testing.T) {
	store := &memStore{writer: machineWriter}
	alarm := &memAlarm{raiseErr: errors.New("ledger append failed")}
	if err := state.WriteHalt(context.Background(), store, alarm, "l", "p1", "head-sha", "badge lied"); err == nil {
		t.Fatal("a failed alarm did not surface")
	}
	// The halt record is durable regardless of the alarm.
	if store.appends != 1 {
		t.Fatal("the halt record was not persisted before the alarm was attempted")
	}
	res, _ := state.Resolve(context.Background(), store.block, machineWriter, newDisp())
	if !res.Halted {
		t.Fatal("the halt did not hold after a failed alarm")
	}
}

// WriteHalt rejects a missing store or missing pins rather than losing the halt.
func TestWriteHalt_GuardsInputs(t *testing.T) {
	if err := state.WriteHalt(context.Background(), nil, &memAlarm{}, "l", "p1", "h", "r"); err == nil {
		t.Fatal("WriteHalt with a nil store must error (the halt must be persistable)")
	}
	if err := state.WriteHalt(context.Background(), &memStore{}, &memAlarm{}, "l", "", "h", "r"); err == nil {
		t.Fatal("WriteHalt with no proposal must error")
	}
}
