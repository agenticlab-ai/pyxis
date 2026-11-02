package state_test

import (
	"context"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/lane/state"
)

// disp7 is the one recorded human disposition the valid-clear cases cite.
var disp7 = state.DispositionRef{Ledger: "docs/problems/x/delivery.md", Anchor: "E-7"}

func block(entries ...state.Entry) []byte {
	var b []byte
	for _, e := range entries {
		b = state.AppendBlock(b, e)
	}
	return b
}

func refusalMentions(res state.Resolution, sub string) bool {
	for _, d := range res.Refusals {
		if strings.Contains(d.Reason, sub) {
			return true
		}
	}
	return false
}

// A halt with no clear halts the lane, and the active halt is named for the
// alarm/debug surface.
func TestResolve_HaltHalts(t *testing.T) {
	b := block(state.Halt("p1", "head-sha", "merged badge, absent mainline content"))
	res, err := state.Resolve(context.Background(), b, machineWriter, newDisp())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Halted {
		t.Fatal("a halt entry did not halt the lane")
	}
	if res.ActiveHalt == nil || res.ActiveHalt.Proposal != "p1" {
		t.Fatalf("active halt not surfaced: %+v", res.ActiveHalt)
	}
}

// A machine-written clear citing a recorded human disposition un-halts the lane.
func TestResolve_ValidClearUnhalts(t *testing.T) {
	b := block(
		state.Halt("p1", "head-sha", "badge lied"),
		state.Clear(disp7),
	)
	res, err := state.Resolve(context.Background(), b, machineWriter, newDisp(disp7))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Halted {
		t.Fatalf("a valid clear did not un-halt the lane: %+v", res.Refusals)
	}
	if res.ActiveHalt != nil {
		t.Fatalf("cleared lane still names an active halt: %+v", res.ActiveHalt)
	}
	if len(res.Refusals) != 0 {
		t.Fatalf("a valid clear produced refusals: %+v", res.Refusals)
	}
}

// A-6 / DINV-7: a forged clear does not un-halt the lane. A non-machine write to
// the lane-state ref, or a clear citing no (or an unrecorded) human disposition,
// leaves the lane halted with a named refusal.
func TestResolve_ForgedClearDoesNotUnhalt(t *testing.T) {
	cases := []struct {
		name    string
		writer  state.Writer
		clear   state.Entry
		disp    *memDisp
		mention string
	}{
		{
			name:    "non-machine write",
			writer:  humanWriter,
			clear:   state.Clear(disp7),
			disp:    newDisp(disp7), // even a real disposition can't help a non-machine write
			mention: "non-machine write",
		},
		{
			name:    "uncited clear (no anchor)",
			writer:  machineWriter,
			clear:   state.Clear(state.DispositionRef{Ledger: "docs/problems/x/delivery.md"}),
			disp:    newDisp(),
			mention: "citing no human disposition",
		},
		{
			name:    "cited disposition not recorded on the ledger",
			writer:  machineWriter,
			clear:   state.Clear(disp7),
			disp:    newDisp(), // the cited E-7 is not present
			mention: "not a human disposition recorded",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			b := block(state.Halt("p1", "head-sha", "badge lied"), tc.clear)
			res, err := state.Resolve(context.Background(), b, tc.writer, tc.disp)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !res.Halted {
				t.Fatal("a forged clear un-halted the lane (DINV-7)")
			}
			if res.ActiveHalt == nil || res.ActiveHalt.Proposal != "p1" {
				t.Fatalf("the halt is no longer active after a refused clear: %+v", res.ActiveHalt)
			}
			if !refusalMentions(res, tc.mention) {
				t.Fatalf("refusal does not name %q: %+v", tc.mention, res.Refusals)
			}
		})
	}
}

// An unverifiable clear (the delivery-ledger read errored) fails closed: the
// clear does not count, the lane stays halted, and the error surfaces.
func TestResolve_UnverifiableClearFailsClosed(t *testing.T) {
	b := block(state.Halt("p1", "head-sha", "badge lied"), state.Clear(disp7))
	res, err := state.Resolve(context.Background(), b, machineWriter, &memDisp{err: errDisp})
	if err == nil {
		t.Fatal("an unverifiable clear did not surface the read error")
	}
	if !res.Halted {
		t.Fatal("an unverifiable clear un-halted the lane — must fail closed to halted (DINV-7)")
	}
	if !refusalMentions(res, "could not be verified") {
		t.Fatalf("refusal does not name the unverifiable clear: %+v", res.Refusals)
	}
}

// A corrupted lane-state record fails closed to halted and surfaces the parse
// error (DINV-8) — never a false un-halt.
func TestResolve_CorruptedRecordFailsClosed(t *testing.T) {
	res, err := state.Resolve(context.Background(), []byte(`{"record_version":"lane-state/v9","kind":"halt"}`+"\n"), machineWriter, newDisp())
	if err == nil {
		t.Fatal("a corrupted record parsed clean")
	}
	if !res.Halted {
		t.Fatal("a corrupted record did not fail closed to halted (DINV-8)")
	}
}

// A re-halt after a valid clear halts again — the record is a history, not a
// latest-wins cell, and the latest unresolved halt governs.
func TestResolve_ReHaltAfterClear(t *testing.T) {
	b := block(
		state.Halt("p1", "h1", "badge lied"),
		state.Clear(disp7),
		state.Halt("p2", "h2", "badge lied again"),
	)
	res, err := state.Resolve(context.Background(), b, machineWriter, newDisp(disp7))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Halted || res.ActiveHalt == nil || res.ActiveHalt.Proposal != "p2" {
		t.Fatalf("the later halt does not govern: halted=%v active=%+v", res.Halted, res.ActiveHalt)
	}
}

// An empty record (no halt ever) is not halted.
func TestResolve_EmptyNotHalted(t *testing.T) {
	res, err := state.Resolve(context.Background(), nil, state.Writer{}, newDisp())
	if err != nil || res.Halted {
		t.Fatalf("empty record: halted=%v err=%v", res.Halted, err)
	}
}
