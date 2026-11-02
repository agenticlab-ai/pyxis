package port

import (
	"context"
	"fmt"
)

// FakeStateReader is an in-process ProposalStateReader for consumer tests —
// no network, git or CLI (the hermetic-fake convention used across the
// executor tree, §3.10). A test registers each ref's answer with Set or
// SetUnknown before driving the reader; States then answers exactly what was
// registered, in the order refs was given (DINV-1).
//
// Requesting a ref nothing was registered for is a test-authoring bug, not a
// runtime condition any real adapter would hit, so States panics rather than
// inventing an unknown reason no medium would actually report — panic is for
// programmer error only.
//
// FakeStateReader is not safe for concurrent use; each test constructs its
// own.
type FakeStateReader struct {
	states map[ProposalRef]ProposalState
	err    error // when set, States returns this transport-fatal error instead
}

var _ ProposalStateReader = (*FakeStateReader)(nil)

// NewFakeStateReader returns an empty FakeStateReader; register each ref's
// answer with Set or SetUnknown before use.
func NewFakeStateReader() *FakeStateReader {
	return &FakeStateReader{states: make(map[ProposalRef]ProposalState)}
}

// Set registers ref's healthy answer for a subsequent States call. state must
// describe a genuine read: Landing must not be LandingUnknown and Unknown
// must be nil (register a failed read with SetUnknown instead), and a
// LandingLanded state must carry a non-empty LandedRevision (DINV-3) — Set
// panics on any of these to keep the fake from ever modelling the
// self-contradictory shape the port forbids.
func (f *FakeStateReader) Set(ref ProposalRef, state ProposalState) {
	if state.Unknown != nil {
		panic(fmt.Sprintf("port: FakeStateReader.Set(%s): state carries a non-nil Unknown; use SetUnknown", ref))
	}
	if state.Landing == LandingUnknown {
		panic(fmt.Sprintf("port: FakeStateReader.Set(%s): Landing = LandingUnknown without a reason; use SetUnknown", ref))
	}
	if state.Landing == LandingLanded && state.LandedRevision == "" {
		panic(fmt.Sprintf("port: FakeStateReader.Set(%s): LandingLanded with an empty LandedRevision (DINV-3)", ref))
	}
	f.states[ref] = state
}

// SetUnknown registers ref as unanswerable this batch, with the given typed
// reason (DINV-1, DINV-2). reason must be one of the named UnknownReason
// values.
func (f *FakeStateReader) SetUnknown(ref ProposalRef, reason UnknownReason) {
	if !validUnknownReason(reason) {
		panic(fmt.Sprintf("port: FakeStateReader.SetUnknown(%s): %d is not a named UnknownReason", ref, int(reason)))
	}
	f.states[ref] = ProposalState{Landing: LandingUnknown, Unknown: &reason}
}

// SetErr makes every subsequent States call return err instead of per-ref
// answers — a transport-fatal failure (context cancellation, an unreadable
// local budget file), never a per-proposal condition (DINV-2).
func (f *FakeStateReader) SetErr(err error) {
	f.err = err
}

// States implements ProposalStateReader over the registered fixtures: every
// requested ref is answered exactly once, in order (DINV-1), with Comments
// filtered to those newer than since's per-surface position.
func (f *FakeStateReader) States(ctx context.Context, refs []ProposalRef, since Cursor) ([]ProposalState, error) {
	if f.err != nil {
		return nil, f.err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	out := make([]ProposalState, len(refs))
	for i, ref := range refs {
		st, ok := f.states[ref]
		if !ok {
			panic(fmt.Sprintf("port: FakeStateReader.States: no fixture registered for %s", ref))
		}
		st.Comments = newerComments(st.Comments, since)
		out[i] = st
	}
	return out, nil
}

// newerComments returns only the comments strictly newer than since's
// per-surface position; a surface absent from since.Positions has no lower
// bound.
func newerComments(comments []Comment, since Cursor) []Comment {
	if len(comments) == 0 {
		return nil
	}
	out := make([]Comment, 0, len(comments))
	for _, c := range comments {
		if pos, ok := since.Positions[c.Surface]; ok && !c.CreatedAt.After(pos) {
			continue
		}
		out = append(out, c)
	}
	return out
}
