package wake

import (
	"github.com/agenticlab-ai/pyxis/internal/docgate/argusledger"
)

// Disposition is a delivery ledger's wake-relevant state, derived from its
// journal alone (the derived block is a regenerable hint, never trusted —
// argus-artifacts DINV-6). It gates the two things the backstop must never do:
// probe a parked problem (DINV-2) or reopen a closed one (DINV-3).
type Disposition int

const (
	// Open — the drive may resume: the backstop probes this ledger's awaits.
	Open Disposition = iota
	// Parked — the drive is blocked on a human answer (a brief emitted, not yet
	// answered). Parks wake only on their answer append or a human touch; the
	// backstop skips them so a parked problem draws zero scheduled activity
	// (DINV-2; the behavior design's DINV-4).
	Parked
	// Closed — the journal carries its one `delivered` entry and the file is
	// frozen. No wake reopens it: a signal naming a closed ledger produces at
	// most a journaled no-op observation, never a byte of the frozen file
	// (DINV-3).
	Closed
)

func (d Disposition) String() string {
	switch d {
	case Parked:
		return "parked"
	case Closed:
		return "closed"
	default:
		return "open"
	}
}

// Dispose derives a delivery ledger's disposition from its journal entries in
// document order (argusledger.Ledger.Order). A `delivered` entry is terminal —
// the ledger is Closed and dominates (closed wins over any park state). Absent a
// close, the ledger is Parked when its last park-relevant entry is a
// `brief-emitted` with no following `answer-received`, else Open. It is
// meaningful only on a delivery ledger; any other class (or a nil ledger) is
// Open — not this derivation's concern.
func Dispose(l *argusledger.Ledger) Disposition {
	if l == nil || l.Class.DerivedRegion == "" {
		return Open
	}
	parked := false
	for _, id := range l.Order {
		switch argusledger.EntryType(l.Entries[id]) {
		case argusledger.TypeDelivered:
			return Closed
		case argusledger.TypeBriefEmitted:
			parked = true
		case argusledger.TypeAnswerReceived:
			parked = false
		}
	}
	if parked {
		return Parked
	}
	return Open
}
