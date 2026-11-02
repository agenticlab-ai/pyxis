package port

import (
	"context"
	"errors"
	"fmt"
)

// ReaderConformanceCase is the fixture a ProposalStateReader implementation is
// driven against by CheckReaderConformance. It names one mixed batch — an
// ordinary ref among others, one landed with its claimed revision id, and one
// the medium cannot answer — so a single corpus exercises the three
// properties every implementation of this port must hold
// (docs/tech-design/state-reader-v2.md §3.3, Scope): every ref answered
// exactly once (DINV-1), a failed read typed rather than omitted or
// defaulted (DINV-1, DINV-2), and landing presented as the medium's claim,
// carrying a usable revision id (DINV-3).
//
// A new adapter builds its own ReaderConformanceCase from its own recorded or
// store-derived fixtures and passes it to CheckReaderConformance verbatim
// (AP-3): passing the kit is what makes it a conforming adapter, not a design
// read. See statereaderkit_test.go for a worked example against
// FakeStateReader.
type ReaderConformanceCase struct {
	// Refs is the full requested batch, in the order States must answer them.
	// It must list Landed and Unknown among its entries, and the two must
	// differ.
	Refs []ProposalRef

	// Since is the cursor passed to States.
	Since Cursor

	// Landed is the ref the medium reports landed; LandedRevision, if given,
	// is the exact revision id its result must carry — otherwise the result
	// need only carry some non-empty revision id (DINV-3). A consumer
	// verifies mainline ancestry from this value alone, with no further read.
	Landed         ProposalRef
	LandedRevision string

	// Unknown is the ref the medium cannot answer for this batch; its result
	// must carry a typed reason, never an omission or a zero value (DINV-1,
	// DINV-2).
	Unknown ProposalRef

	// MaxRoundTrips is asserted when reader implements RoundTripReader. A zero
	// value leaves the bound disabled for adapters whose transport does not
	// expose a counter.
	MaxRoundTrips int
}

// RoundTripReader is the optional metered-medium hook used by the kit. The
// count is cumulative for the reader instance, allowing the kit to assert the
// call made by its single States invocation without issuing a second read.
type RoundTripReader interface {
	RoundTrips() int
}

// CheckReaderConformance drives one ProposalStateReader implementation
// through a mixed batch and returns every contract violation joined into a
// single error (nil when the adapter conforms). It talks only to the port
// interface, so it is medium-agnostic, and it is hermetic whenever reader is
// (e.g. FakeStateReader, or a production adapter wired to recorded or local
// fixtures — never a live network call).
//
// It asserts, over the one States call:
//   - every requested ref is answered exactly once, in the order requested
//     (DINV-1);
//   - c.Unknown's result carries a non-nil, validly-typed reason, LandingUnknown,
//     and no checks, comments or revision id alongside it — a failed read
//     spills no state (DINV-1, DINV-2);
//   - every other ref's result is not marked Unknown and does not carry
//     LandingUnknown;
//   - c.Landed's result reports LandingLanded with a non-empty
//     LandedRevision, matching c.LandedRevision when given (DINV-3).
//
// This is the kit's skeleton: the three properties named in state-reader-v2's
// Scope (all-refs-answered, typed-unknown, claim-marking). It does not assert
// DINV-4's required/aggregate reduction or DINV-5's round-trip bound — those
// are each adapter's own task-scoped conformance additions.
func CheckReaderConformance(ctx context.Context, reader ProposalStateReader, c ReaderConformanceCase) error {
	if reader == nil {
		return errors.New("port: CheckReaderConformance: nil ProposalStateReader")
	}
	if !containsRef(c.Refs, c.Landed) {
		return fmt.Errorf("port: CheckReaderConformance: fixture's Landed ref %s is not in Refs", c.Landed)
	}
	if !containsRef(c.Refs, c.Unknown) {
		return fmt.Errorf("port: CheckReaderConformance: fixture's Unknown ref %s is not in Refs", c.Unknown)
	}
	if c.Landed == c.Unknown {
		return errors.New("port: CheckReaderConformance: fixture's Landed and Unknown refs must differ")
	}

	results, err := reader.States(ctx, c.Refs, c.Since)
	if err != nil {
		return fmt.Errorf("port: CheckReaderConformance: States: %w", err)
	}
	if len(results) != len(c.Refs) {
		return fmt.Errorf("port: CheckReaderConformance: States returned %d results for %d requested refs — every ref must be answered exactly once (DINV-1)", len(results), len(c.Refs))
	}
	if c.MaxRoundTrips > 0 {
		metered, ok := reader.(RoundTripReader)
		if !ok {
			return fmt.Errorf("port: CheckReaderConformance: fixture requires a round-trip bound of %d, but reader does not expose RoundTrips", c.MaxRoundTrips)
		}
		if got := metered.RoundTrips(); got > c.MaxRoundTrips {
			return fmt.Errorf("port: CheckReaderConformance: States used %d round trips; want at most %d (DINV-5)", got, c.MaxRoundTrips)
		}
	}

	var errs []error
	for i, ref := range c.Refs {
		st := results[i]
		switch ref {
		case c.Unknown:
			errs = append(errs, checkUnknownResult(ref, st)...)
		default:
			errs = append(errs, checkAnsweredResult(ref, st)...)
			if ref == c.Landed {
				errs = append(errs, checkLandedClaim(ref, st, c.LandedRevision)...)
			}
		}
	}
	return errors.Join(errs...)
}

// checkUnknownResult asserts the typed-unknown property (DINV-1, DINV-2): a
// ref the medium cannot answer for carries a validly-typed reason and nothing
// else — never an omission, never a healthy-looking zero value.
func checkUnknownResult(ref ProposalRef, st ProposalState) []error {
	var errs []error
	if st.Unknown == nil {
		errs = append(errs, fmt.Errorf("%s: Unknown is nil; a proposal the medium cannot answer for must carry a typed reason, never an omission (DINV-1, DINV-2)", ref))
		return errs
	}
	if !validUnknownReason(*st.Unknown) {
		errs = append(errs, fmt.Errorf("%s: Unknown reason %v is not one of the typed reasons — never an untyped zero value (DINV-1)", ref, *st.Unknown))
	}
	if st.Landing != LandingUnknown {
		errs = append(errs, fmt.Errorf("%s: Landing = %v alongside a non-nil Unknown; want LandingUnknown — a failed read must not also present a landing claim (DINV-2)", ref, st.Landing))
	}
	if len(st.Checks) != 0 || len(st.Comments) != 0 || st.LandedRevision != "" {
		errs = append(errs, fmt.Errorf("%s: an Unknown result also carries checks, comments or a revision id — no consumer-visible state may derive from a failed read (DINV-2)", ref))
	}
	return errs
}

// checkAnsweredResult asserts the all-refs-answered property for a ref the
// fixture expects a genuine read for: it must not be marked Unknown, and must
// not carry the fail-closed LandingUnknown zero value (DINV-1).
func checkAnsweredResult(ref ProposalRef, st ProposalState) []error {
	var errs []error
	if st.Unknown != nil {
		errs = append(errs, fmt.Errorf("%s: Unknown = %v on a ref the fixture expects answered — a partial medium failure must not spill onto an unrelated ref", ref, *st.Unknown))
	}
	if st.Landing == LandingUnknown {
		errs = append(errs, fmt.Errorf("%s: Landing = unknown with a nil Unknown reason — a healthy-looking zero value standing in for a typed reason (DINV-1)", ref))
	}
	return errs
}

// checkLandedClaim asserts the claim-marking property (DINV-3): landing is
// presented as the medium's claim, carrying a usable revision id a consumer
// verifies ancestry from without a further read.
func checkLandedClaim(ref ProposalRef, st ProposalState, wantRevision string) []error {
	var errs []error
	if st.Landing != LandingLanded {
		errs = append(errs, fmt.Errorf("%s: Landing = %v; want LandingLanded", ref, st.Landing))
	}
	if st.LandedRevision == "" {
		errs = append(errs, fmt.Errorf("%s: LandingLanded carries an empty LandedRevision — landing must carry the medium's claimed revision id (DINV-3)", ref))
	} else if wantRevision != "" && st.LandedRevision != wantRevision {
		errs = append(errs, fmt.Errorf("%s: LandedRevision = %q; want %q", ref, st.LandedRevision, wantRevision))
	}
	return errs
}

// containsRef reports whether refs contains ref.
func containsRef(refs []ProposalRef, ref ProposalRef) bool {
	for _, r := range refs {
		if r == ref {
			return true
		}
	}
	return false
}

// validUnknownReason reports whether r is one of the named typed reasons —
// never the reserved zero value (DINV-1).
func validUnknownReason(r UnknownReason) bool {
	switch r {
	case UnknownRateLimited, UnknownOffline, UnknownParseDefect, UnknownTimeout:
		return true
	default:
		return false
	}
}
