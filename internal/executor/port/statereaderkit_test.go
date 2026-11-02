package port_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/executor/port"
)

// buildMixedFixture returns a batch of three refs — an ordinary open proposal
// with checks, a required-missing context and comments split across a cursor
// boundary; a landed proposal carrying its claimed revision id; and one the
// medium cannot answer for — registered on a fresh FakeStateReader, plus the
// ReaderConformanceCase describing it (state-reader-v2.md §3.3's mixed-batch
// acceptance case).
func buildMixedFixture(t *testing.T) (*port.FakeStateReader, port.ReaderConformanceCase) {
	t.Helper()

	openRef := port.ProposalRef{Repo: "acme/widget", Number: 1}
	landedRef := port.ProposalRef{Repo: "acme/widget", Number: 2}
	unknownRef := port.ProposalRef{Repo: "acme/widget", Number: 3}

	before := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	after := time.Date(2026, 7, 30, 0, 0, 0, 0, time.UTC)

	fake := port.NewFakeStateReader()
	fake.Set(openRef, port.ProposalState{
		Landing: port.LandingOpen,
		Checks: []port.CheckContext{
			{Name: "unit-tests", State: port.CheckGreen},
			{Name: "lint", State: port.CheckRed},
		},
		RequiredMissing: []string{"deploy-preview"},
		Comments: []port.Comment{
			{Author: "reviewer", CreatedAt: before, Surface: port.SurfaceDiscussion, Body: "stale, at the cursor"},
			{Author: "reviewer", CreatedAt: after, Surface: port.SurfaceDiscussion, Body: "fresh, after the cursor"},
		},
	})
	fake.Set(landedRef, port.ProposalState{
		Landing:        port.LandingLanded,
		LandedRevision: "abc123def",
	})
	fake.SetUnknown(unknownRef, port.UnknownRateLimited)

	c := port.ReaderConformanceCase{
		Refs:           []port.ProposalRef{openRef, landedRef, unknownRef},
		Since:          port.Cursor{Positions: map[port.CommentSurface]time.Time{port.SurfaceDiscussion: before}},
		Landed:         landedRef,
		LandedRevision: "abc123def",
		Unknown:        unknownRef,
	}
	return fake, c
}

func TestCheckReaderConformance_Fake(t *testing.T) {
	fake, c := buildMixedFixture(t)
	if err := port.CheckReaderConformance(context.Background(), fake, c); err != nil {
		t.Fatalf("CheckReaderConformance against FakeStateReader: %v", err)
	}
}

// TestFakeStateReader_MixedBatch drills into the raw States response beyond
// what the kit's summary asserts: every ref answered exactly once in order,
// the open proposal's checks/required-missing/cursor-filtered comments intact,
// the landed claim carrying its revision id, and the unknown ref typed.
func TestFakeStateReader_MixedBatch(t *testing.T) {
	fake, c := buildMixedFixture(t)

	got, err := fake.States(context.Background(), c.Refs, c.Since)
	if err != nil {
		t.Fatalf("States: %v", err)
	}
	if len(got) != len(c.Refs) {
		t.Fatalf("States returned %d results for %d refs", len(got), len(c.Refs))
	}

	open, landed, unknown := got[0], got[1], got[2]

	if open.Landing != port.LandingOpen {
		t.Errorf("open ref Landing = %v, want open", open.Landing)
	}
	if len(open.Comments) != 1 || open.Comments[0].Body != "fresh, after the cursor" {
		t.Errorf("open ref Comments = %+v, want only the comment strictly after the cursor", open.Comments)
	}
	if len(open.RequiredMissing) != 1 || open.RequiredMissing[0] != "deploy-preview" {
		t.Errorf("open ref RequiredMissing = %v, want [deploy-preview]", open.RequiredMissing)
	}
	if len(open.Checks) != 2 {
		t.Errorf("open ref Checks = %+v, want 2 entries", open.Checks)
	}

	if landed.Landing != port.LandingLanded {
		t.Errorf("landed ref Landing = %v, want landed", landed.Landing)
	}
	if landed.LandedRevision != "abc123def" {
		t.Errorf("landed ref LandedRevision = %q, want %q", landed.LandedRevision, "abc123def")
	}

	if unknown.Unknown == nil || *unknown.Unknown != port.UnknownRateLimited {
		t.Errorf("unknown ref Unknown = %v, want rate-limited", unknown.Unknown)
	}
	if unknown.Landing != port.LandingUnknown {
		t.Errorf("unknown ref Landing = %v, want unknown", unknown.Landing)
	}
}

// --- negative cases: the kit must fail a non-conforming implementation -----

// stubReader lets a test script an arbitrary — including non-conforming —
// States response, to prove CheckReaderConformance actually catches what it
// claims to.
type stubReader func(ctx context.Context, refs []port.ProposalRef, since port.Cursor) ([]port.ProposalState, error)

func (f stubReader) States(ctx context.Context, refs []port.ProposalRef, since port.Cursor) ([]port.ProposalState, error) {
	return f(ctx, refs, since)
}

func TestCheckReaderConformance_CatchesOmittedRef(t *testing.T) {
	_, c := buildMixedFixture(t)
	reader := stubReader(func(_ context.Context, refs []port.ProposalRef, _ port.Cursor) ([]port.ProposalState, error) {
		return make([]port.ProposalState, len(refs)-1), nil // one short
	})
	if err := port.CheckReaderConformance(context.Background(), reader, c); err == nil {
		t.Fatal("CheckReaderConformance: want an error for a response one ref short, got nil")
	}
}

func TestCheckReaderConformance_CatchesZeroValueInPlaceOfUnknown(t *testing.T) {
	_, c := buildMixedFixture(t)
	reader := stubReader(func(_ context.Context, refs []port.ProposalRef, _ port.Cursor) ([]port.ProposalState, error) {
		// Every ref, including the one the fixture says the medium cannot
		// answer for, comes back as a healthy-looking untyped zero value.
		return make([]port.ProposalState, len(refs)), nil
	})
	if err := port.CheckReaderConformance(context.Background(), reader, c); err == nil {
		t.Fatal("CheckReaderConformance: want an error for an untyped zero value standing in for Unknown, got nil")
	}
}

func TestCheckReaderConformance_CatchesMissingLandedRevision(t *testing.T) {
	fake, c := buildMixedFixture(t)
	reader := stubReader(func(ctx context.Context, refs []port.ProposalRef, since port.Cursor) ([]port.ProposalState, error) {
		got, err := fake.States(ctx, refs, since)
		if err != nil {
			return nil, err
		}
		for i, ref := range refs {
			if ref == c.Landed {
				got[i].LandedRevision = "" // drop the claim's revision id
			}
		}
		return got, nil
	})
	if err := port.CheckReaderConformance(context.Background(), reader, c); err == nil {
		t.Fatal("CheckReaderConformance: want an error for a landed result with no revision id, got nil")
	}
}

func TestCheckReaderConformance_CatchesUnknownSpillingCheckState(t *testing.T) {
	fake, c := buildMixedFixture(t)
	reader := stubReader(func(ctx context.Context, refs []port.ProposalRef, since port.Cursor) ([]port.ProposalState, error) {
		got, err := fake.States(ctx, refs, since)
		if err != nil {
			return nil, err
		}
		for i, ref := range refs {
			if ref == c.Unknown {
				// A failed read must not also carry a healthy-looking checks
				// answer (DINV-2) — even with Unknown correctly set.
				got[i].Checks = []port.CheckContext{{Name: "unit-tests", State: port.CheckGreen}}
			}
		}
		return got, nil
	})
	if err := port.CheckReaderConformance(context.Background(), reader, c); err == nil {
		t.Fatal("CheckReaderConformance: want an error for an Unknown result that also carries checks, got nil")
	}
}

func TestCheckReaderConformance_TopLevelErrorPropagates(t *testing.T) {
	fake, c := buildMixedFixture(t)
	wantErr := errors.New("boom")
	fake.SetErr(wantErr)
	if err := port.CheckReaderConformance(context.Background(), fake, c); !errors.Is(err, wantErr) {
		t.Fatalf("CheckReaderConformance: err = %v, want it to wrap %v", err, wantErr)
	}
}

func TestCheckReaderConformance_RejectsMalformedFixture(t *testing.T) {
	_, c := buildMixedFixture(t)
	fake := port.NewFakeStateReader()

	badLanded := c
	badLanded.Landed = port.ProposalRef{Repo: "not/in-refs", Number: 999}
	if err := port.CheckReaderConformance(context.Background(), fake, badLanded); err == nil {
		t.Fatal("CheckReaderConformance: want an error when Landed is not among Refs, got nil")
	}

	sameRef := c
	sameRef.Unknown = sameRef.Landed
	if err := port.CheckReaderConformance(context.Background(), fake, sameRef); err == nil {
		t.Fatal("CheckReaderConformance: want an error when Landed and Unknown are the same ref, got nil")
	}

	missingUnknown := c
	missingUnknown.Unknown = port.ProposalRef{Repo: "not/in-refs", Number: 1000}
	if err := port.CheckReaderConformance(context.Background(), fake, missingUnknown); err == nil {
		t.Fatal("CheckReaderConformance: want an error when Unknown is not among Refs, got nil")
	}

	if err := port.CheckReaderConformance(context.Background(), nil, c); err == nil {
		t.Fatal("CheckReaderConformance: want an error for a nil reader, got nil")
	}
}
