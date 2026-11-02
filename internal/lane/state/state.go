// Package state is the merge lane's repo-resident lane-state record — the
// halted state's home (merge-lane-v1 §3.6, DINV-7; plan T5). It is the fourth
// lane data class, the layer that turns a goal-verify failure (the acting
// adapter's res.Halt, T4) into a durable, repo-resident circuit breaker read at
// every subsequent act gate, and the kill-switch trip a discovered floor
// violation fires.
//
// The record lives on a dedicated lane-state ref outside the protected branch
// namespace, so — unlike a protected branch — nothing platform-side keeps a
// non-machine actor from writing it. The design answers that with enforced, not
// merely asserted, write/clear authority (DINV-7): an activation-time
// ref-protection restricts the namespace to the machine identity (read as a
// precondition, never performed here — FR-9's pattern), AND the act gate honors
// a "cleared" reading only after Resolve validates that the ref's writer was the
// machine identity and that every clear cites a human disposition recorded on
// the affected drive's delivery ledger (the argus alarm surface). A forged
// clear, a non-machine write, or a clear citing no disposition does NOT un-halt
// the lane; a planted halt is fail-safe — it only over-stops.
//
// Layering (mirrors the acting package's split): state.go owns the record data
// class and its append-only closed parse; resolve.go owns the act-gate clear
// validation; halt.go owns the synchronous halt write and the delivery-ledger
// alarm; kill.go owns the floor-violation kill trip and the re-activation gate.
// This package holds no credential and knows nothing of the platform beyond the
// thin Store/Alarm/DispositionSource ports the workflow adapter fills; every
// port is local-substitutable so the tests inject no mocks. Stdlib only, per the
// repo Go rules; every uncertain input fails closed to halted, never a silent
// un-halt.
package state

import (
	"encoding/json"
	"fmt"
	"strings"
)

// RecordVersion is the one lane-state record shape this build writes and parses.
// Versioned, closed-schema data — additive evolution only (AP-6); a line
// declaring any other version is a defect, never parsed leniently (DINV-8's
// closed-load discipline carried to the lane-state record).
const RecordVersion = "lane-state/v1"

// Kind is an entry's type in the append-only lane-state record. The grammar is
// closed: exactly two kinds, and an unknown kind is a parse defect.
type Kind string

const (
	// KindHalt records a goal-verify failure — the platform reported the
	// proposal merged but its content is not an ancestor of the mainline (a lying
	// badge, incident I2's class). Written synchronously by the machine before any
	// further act (DINV-7).
	KindHalt Kind = "halt"
	// KindClear records a resolution appended after a halt: it cites the human
	// disposition, recorded on the affected drive's delivery ledger, that
	// authorizes lifting the halt. A clear un-halts the lane ONLY when Resolve
	// validates the ref's writer as the machine identity and the cited
	// disposition as present (DINV-7).
	KindClear Kind = "clear"
)

// DispositionRef cites a human disposition recorded on a drive's delivery
// ledger — the argus alarm surface, the only surface that authorizes a clear
// (DINV-7). Both fields are required on a clear: a clear whose Anchor is empty
// cites no disposition and cannot un-halt the lane.
type DispositionRef struct {
	// Ledger is the affected drive's delivery ledger path
	// (docs/problems/<slug>/delivery.md).
	Ledger string `json:"ledger"`
	// Anchor is the cited entry anchor within that ledger (e.g. "E-7") — the
	// recorded human disposition DispositionSource verifies exists.
	Anchor string `json:"anchor"`
}

// cited reports whether the reference names both a ledger and an anchor. An
// uncited clear (either empty) is refused before any lookup — it authorizes
// nothing.
func (r DispositionRef) cited() bool {
	return strings.TrimSpace(r.Ledger) != "" && strings.TrimSpace(r.Anchor) != ""
}

// String renders the citation for a readable refusal reason.
func (r DispositionRef) String() string { return r.Ledger + "#" + r.Anchor }

// Entry is one append-only line of the lane-state record. A halt entry carries
// the proposal whose badge lied, the exact evaluated head the voided act bound
// to, and the goal-verify failure reason; a clear entry carries the cited
// disposition. Every entry is versioned so the record is cold-readable and its
// grammar closed years later (NFR-4, AP-6). The record is append-only across
// events — earlier entries are never rewritten, only followed — so it is the
// halt/clear history, not a latest-wins cell.
type Entry struct {
	RecordVersion string          `json:"record_version"`
	Kind          Kind            `json:"kind"`
	Proposal      string          `json:"proposal,omitempty"`      // halt: the proposal whose badge lied
	HeadRevision  string          `json:"head_revision,omitempty"` // halt: the evaluated head the voided act bound to
	Reason        string          `json:"reason,omitempty"`        // halt: the goal-verify failure reason
	Disposition   *DispositionRef `json:"disposition,omitempty"`   // clear: the cited human disposition
}

// Halt builds a halt entry for a goal-verify failure. proposal and head pin the
// exact act whose badge lied; reason names why the goal-object verification
// failed (the merged content is not a mainline ancestor).
func Halt(proposal, head, reason string) Entry {
	return Entry{
		RecordVersion: RecordVersion,
		Kind:          KindHalt,
		Proposal:      proposal,
		HeadRevision:  head,
		Reason:        reason,
	}
}

// Clear builds a clear entry citing the human disposition that authorizes
// lifting the halt. The clear un-halts the lane only after Resolve validates the
// writer and the cited disposition (DINV-7); an uncited clear is refused.
func Clear(ref DispositionRef) Entry {
	r := ref
	return Entry{
		RecordVersion: RecordVersion,
		Kind:          KindClear,
		Disposition:   &r,
	}
}

// Marshal serializes one entry deterministically — the struct's field order is
// fixed and every field is a scalar or a two-scalar ref — so the same entry
// yields byte-identical bytes (AP-6). One line, no trailing newline; AppendBlock
// adds the separator.
func (e Entry) Marshal() []byte {
	b, err := json.Marshal(e)
	if err != nil {
		// Entry carries only strings and a scalar ref, so Marshal cannot fail;
		// this guards a future field that breaks that (programmer error) rather
		// than emitting a partial, unreadable record.
		panic("state: entry marshal: " + err.Error())
	}
	return b
}

// AppendBlock appends one entry to the append-only lane-state block (JSONL) and
// returns the new block. Append-only across events — earlier entries are never
// rewritten, only followed — so the block is the halt/clear history. A
// nil/empty block starts a fresh one.
func AppendBlock(block []byte, e Entry) []byte {
	line := e.Marshal()
	if len(strings.TrimSpace(string(block))) == 0 {
		return append(line, '\n')
	}
	out := append([]byte{}, block...)
	if out[len(out)-1] != '\n' {
		out = append(out, '\n')
	}
	out = append(out, line...)
	return append(out, '\n')
}

// ParseBlock reads an append-only lane-state block back into its entries, in
// order. The parse is closed (DINV-8's discipline): a non-empty line that is not
// a well-formed entry of the pinned version, or one declaring an unknown kind,
// is a named defect — never a silent skip. A corrupted or version-drifted block
// is a fault, and a faulted read fails closed to halted at the caller (Resolve),
// never a false un-halt.
func ParseBlock(block []byte) ([]Entry, error) {
	var entries []Entry
	for i, line := range strings.Split(string(block), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var e Entry
		if err := json.Unmarshal([]byte(line), &e); err != nil {
			return nil, fmt.Errorf("lane-state block line %d is malformed: %w", i+1, err)
		}
		if e.RecordVersion != RecordVersion {
			return nil, fmt.Errorf("lane-state block line %d declares version %q, not %q", i+1, e.RecordVersion, RecordVersion)
		}
		switch e.Kind {
		case KindHalt, KindClear:
		default:
			return nil, fmt.Errorf("lane-state block line %d declares unknown kind %q", i+1, e.Kind)
		}
		entries = append(entries, e)
	}
	return entries, nil
}
