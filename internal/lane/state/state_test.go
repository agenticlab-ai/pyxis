package state_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/lane/state"
)

// The record data class round-trips and marshals deterministically (AP-6).
func TestEntry_MarshalDeterministic_RoundTrips(t *testing.T) {
	halt := state.Halt("p1", "head-sha", "merged badge, absent mainline content")
	if string(halt.Marshal()) != string(halt.Marshal()) {
		t.Fatal("halt entry marshal is not byte-stable")
	}
	var round state.Entry
	if err := json.Unmarshal(halt.Marshal(), &round); err != nil {
		t.Fatalf("halt does not round-trip: %v", err)
	}
	if round.Kind != state.KindHalt || round.Proposal != "p1" || round.HeadRevision != "head-sha" {
		t.Fatalf("halt round-trip lost fields: %+v", round)
	}

	clear := state.Clear(state.DispositionRef{Ledger: "docs/problems/x/delivery.md", Anchor: "E-7"})
	if clear.Kind != state.KindClear || clear.Disposition == nil || clear.Disposition.Anchor != "E-7" {
		t.Fatalf("clear built wrong: %+v", clear)
	}
}

// The block is append-only JSONL: every entry becomes one line, earlier lines
// are never rewritten, and a nil block starts a fresh one.
func TestAppendBlock_AppendOnly(t *testing.T) {
	var block []byte
	block = state.AppendBlock(block, state.Halt("p1", "h1", "badge lied"))
	block = state.AppendBlock(block, state.Clear(state.DispositionRef{Ledger: "docs/problems/x/delivery.md", Anchor: "E-3"}))
	block = state.AppendBlock(block, state.Halt("p2", "h2", "badge lied again"))

	lines := strings.Split(strings.TrimRight(string(block), "\n"), "\n")
	if len(lines) != 3 {
		t.Fatalf("append-only block has %d lines, want 3: %q", len(lines), block)
	}
	entries, err := state.ParseBlock(block)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(entries) != 3 || entries[0].Proposal != "p1" || entries[2].Proposal != "p2" {
		t.Fatalf("entries out of order or lost: %+v", entries)
	}
	if entries[1].Kind != state.KindClear {
		t.Fatalf("second entry is not the clear: %+v", entries[1])
	}
}

// The parse is closed (DINV-8): a malformed line, an unknown version, or an
// unknown kind is a named fault, never a silent skip.
func TestParseBlock_ClosedFaults(t *testing.T) {
	cases := []struct {
		name  string
		block string
	}{
		{"malformed json", `{"record_version":"lane-state/v1","kind":"halt"` + "\n"},
		{"unknown version", `{"record_version":"lane-state/v2","kind":"halt"}` + "\n"},
		{"unknown kind", `{"record_version":"lane-state/v1","kind":"resume"}` + "\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := state.ParseBlock([]byte(tc.block)); err == nil {
				t.Fatalf("a %s line parsed clean — the grammar must be closed (DINV-8)", tc.name)
			}
		})
	}
}

// An empty or blank-only block parses to no entries (no halt ever written).
func TestParseBlock_Empty(t *testing.T) {
	for _, b := range [][]byte{nil, {}, []byte("\n\n  \n")} {
		entries, err := state.ParseBlock(b)
		if err != nil || len(entries) != 0 {
			t.Fatalf("empty block: entries=%v err=%v", entries, err)
		}
	}
}
