package state

import (
	"context"

	"github.com/agenticlab-ai/pyxis/internal/docgate/schema"
)

// verb names this package in every defect it emits (schema.Defect.Verb).
const verb = "lane-state"

// Writer identifies who wrote the lane-state ref tip, as the platform attributes
// it — NOT a value self-declared inside the record (a forger controls the
// bytes, never the platform's attribution). Only the machine identity's writes
// are trusted to clear a halt: the ref lives outside the protected branch
// namespace, so branch protection does not guard it, and the act gate must
// therefore validate the writer itself (DINV-7's enforced, not asserted,
// authority). It mirrors acting.Identity's shape without importing it — state is
// the lower layer the acting adapter composes over, never the reverse.
type Writer struct {
	// Name is the platform-attributed writer of the ref tip, for a readable
	// refusal.
	Name string
	// Machine is true only for the dedicated machine identity. A non-machine
	// writer cannot un-halt the lane (DINV-7): its clears are refused.
	Machine bool
}

// DispositionSource verifies a cited human disposition exists on a drive's
// delivery ledger — the argus alarm surface, the only surface that authorizes a
// clear (DINV-7). It is a read-only port the workflow adapter fills (an
// argusledger-backed read over the cited delivery ledger); the tests substitute
// an in-memory set, per the design's local-substitutable seams (no injected
// mocks).
type DispositionSource interface {
	// HasDisposition reports whether the cited delivery ledger records a human
	// disposition at the anchor. A missing ledger or anchor is (false, nil): an
	// uncited or forged clear, refused — it does not un-halt the lane. An
	// unreadable ledger is (false, err): the clear cannot be verified, so it does
	// not count and the lane stays halted (fail closed).
	HasDisposition(ctx context.Context, ref DispositionRef) (bool, error)
}

// Resolution is the act gate's reading of the lane-state record: whether the
// lane is halted, the active halt entry (for the alarm/debug surface, NFR-5),
// and every named refusal of an invalid clear so a forged or uncited clear is
// visible, never silently dropped.
type Resolution struct {
	// Halted is the load-bearing signal the act gate reads first: true means no
	// further AUTO act occurs until a valid clear lifts it. It is the value that
	// feeds the acting adapter's Preconditions.Halted.
	Halted bool
	// ActiveHalt is the unresolved halt entry when Halted, else nil — the datum
	// the delivery-ledger alarm and the debug surface name.
	ActiveHalt *Entry
	// Refusals names every clear the gate refused to honor (a non-machine write,
	// an uncited citation, an absent or unverifiable disposition) so the forged
	// clear is loud, not silent (DINV-7).
	Refusals []schema.Defect
}

// Resolve reads the lane-state block into the act gate's decision: it walks the
// append-only entries in order, a halt sets the lane halted, and a clear lifts
// it ONLY when the ref's writer is the machine identity AND the cited human
// disposition verifies on its delivery ledger. Every other clear is a named
// refusal that leaves the lane halted — a non-machine write, an uncited
// citation, an absent disposition, or one that could not be verified
// (fail-closed). A malformed or version-drifted block is a fault: Resolve fails
// closed to halted and returns the parse error, never a false un-halt (DINV-8).
//
// Resolve returns a non-nil error only for an infrastructure fault — a
// malformed block or a disposition-source read that errored; in both the
// returned Resolution.Halted is the SAFE value (halted), so a caller that acts
// on the Resolution before checking the error still fails closed.
func Resolve(ctx context.Context, block []byte, writer Writer, disp DispositionSource) (Resolution, error) {
	entries, err := ParseBlock(block)
	if err != nil {
		// A record we cannot read is a fault; over-stop rather than risk a false
		// un-halt (DINV-8's closed-load discipline).
		return Resolution{
			Halted: true,
			Refusals: []schema.Defect{def("resolve", "lane-state",
				"the lane-state record could not be parsed: "+err.Error(),
				"fix the corrupted record; an unreadable lane-state record fails closed to halted (DINV-8)")},
		}, err
	}

	res := Resolution{}
	var active *Entry
	var firstErr error
	for i := range entries {
		e := entries[i]
		switch e.Kind {
		case KindHalt:
			active = &entries[i]
			res.Halted = true
		case KindClear:
			file := clearFile(active, e)
			// Enforced write authority: a clear from a non-machine ref writer
			// cannot un-halt the lane. A planted halt over-stops (safe); a forged
			// clear is refused (DINV-7).
			if !writer.Machine {
				res.Refusals = append(res.Refusals, def("clear", file,
					"a non-machine write to the lane-state ref (writer "+quote(writer.Name)+") cannot clear a halt",
					"only the machine identity's write clears a halt; the lane-state ref-protection restricts the namespace to it (DINV-7)"))
				continue
			}
			// A clear must cite a disposition; an uncited clear authorizes nothing.
			if e.Disposition == nil || !e.Disposition.cited() {
				res.Refusals = append(res.Refusals, def("clear", file,
					"a clear entry citing no human disposition cannot un-halt the lane",
					"cite a human disposition recorded on the affected drive's delivery ledger (DINV-7)"))
				continue
			}
			ok, derr := disp.HasDisposition(ctx, *e.Disposition)
			if derr != nil {
				// Cannot verify → the clear does not count and the lane stays
				// halted; remember the error to surface (fail closed).
				if firstErr == nil {
					firstErr = derr
				}
				res.Refusals = append(res.Refusals, def("clear", file,
					"the cited human disposition "+quote(e.Disposition.String())+" could not be verified: "+derr.Error(),
					"an unverifiable clear does not un-halt the lane; fix the delivery-ledger read (DINV-7, fail closed)"))
				continue
			}
			if !ok {
				res.Refusals = append(res.Refusals, def("clear", file,
					"the cited disposition "+quote(e.Disposition.String())+" is not a human disposition recorded on the delivery ledger",
					"only a disposition actually recorded on the affected drive's delivery ledger clears a halt (DINV-7)"))
				continue
			}
			// A valid clear lifts the halt.
			active = nil
			res.Halted = false
		}
	}
	res.ActiveHalt = active
	return res, firstErr
}

// clearFile chooses the traceability surface a clear refusal is keyed to: the
// active halt's proposal when one is open, else the clear's own citation.
func clearFile(active *Entry, clear Entry) string {
	if active != nil && active.Proposal != "" {
		return active.Proposal
	}
	if clear.Disposition != nil {
		return clear.Disposition.Ledger
	}
	return "lane-state"
}

// def builds one named lane-state defect keyed to a surface.
func def(op, file, reason, remedy string) schema.Defect {
	return schema.Defect{Verb: verb, File: file, Op: op, Reason: reason, Remedy: remedy}
}

// quote wraps a value for a readable, unambiguous reason.
func quote(s string) string { return "'" + s + "'" }
