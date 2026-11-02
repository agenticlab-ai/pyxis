package watch

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"
)

func TestCombinedLiveValidatesInputsAndPropagatesEarlyFailures(t *testing.T) {
	if _, err := RunCombinedLive(t.Context(), nil, nil, ReviewStateReader{}, RecoveryPolicy{}, "", nil, nil, nil, nil, nil, nil); err == nil {
		t.Fatal("RunCombinedLive accepted missing dependencies")
	}
	provider := &mutableReviewProvider{}
	reviews, sink := combinedFixture(t, provider)
	reader := &fakeReader{cadence: time.Second}
	sweep := func(context.Context) error { return nil }
	reviewSweep := func(context.Context, ReviewRecoveryRequest) error { return nil }
	ids := func() (string, error) { return "round", nil }
	if _, err := RunCombinedLive(t.Context(), constantSource(Plan{}), reader, reviews, RecoveryPolicy{Bound: 0}, "scope", sweep, reviewSweep, ids, sink, immediateClock{}, nil); err == nil {
		t.Fatal("RunCombinedLive accepted invalid recovery policy")
	}
	sourceErr := errors.New("source unavailable")
	if _, err := RunCombinedLive(t.Context(), func(context.Context) (Plan, error) { return Plan{}, sourceErr }, reader, reviews, RecoveryPolicy{Bound: 1}, "scope", sweep, reviewSweep, ids, sink, immediateClock{}, nil); !errors.Is(err, sourceErr) {
		t.Fatalf("source error = %v", err)
	}
	sweepErr := errors.New("sweep unavailable")
	if _, err := RunCombinedLive(t.Context(), constantSource(Plan{Tasks: []Task{{ID: "ready"}}}), reader, reviews, RecoveryPolicy{Bound: 1}, "scope", func(context.Context) error { return sweepErr }, reviewSweep, ids, sink, immediateClock{}, nil); !errors.Is(err, sweepErr) {
		t.Fatalf("implementation sweep error = %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := RunCombinedLive(ctx, constantSource(Plan{}), reader, reviews, RecoveryPolicy{Bound: 1}, "scope", sweep, reviewSweep, ids, sink, immediateClock{}, nil); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled error = %v", err)
	}
}

func TestCombinedLiveUnknownStateCoversRoundAndCadenceFailures(t *testing.T) {
	provider := &mutableReviewProvider{err: errors.New("offline")}
	reviews, sink := combinedFixture(t, provider)
	reader := &fakeReader{cadence: time.Second}
	sweep := func(context.Context) error { return nil }
	reviewSweep := func(context.Context, ReviewRecoveryRequest) error { return nil }
	roundErr := errors.New("round unavailable")
	if _, err := RunCombinedLive(t.Context(), constantSource(Plan{}), reader, reviews, RecoveryPolicy{Bound: 1}, "scope", sweep, reviewSweep, func() (string, error) { return "", roundErr }, sink, immediateClock{}, nil); !errors.Is(err, roundErr) {
		t.Fatalf("round error = %v", err)
	}
	reader.cadence = 0
	if _, err := RunCombinedLive(t.Context(), constantSource(Plan{}), reader, reviews, RecoveryPolicy{Bound: 1}, "scope", sweep, reviewSweep, func() (string, error) { return "round", nil }, sink, immediateClock{}, nil); err == nil {
		t.Fatal("non-positive cadence was accepted")
	}

	if _, err := NewFileRecoveryLedger(filepath.Join(t.TempDir(), "ledger.jsonl")); err != nil {
		t.Fatal(err)
	}
}

func TestNextUniqueRoundIDRejectsSourceEmptyAndDuplicate(t *testing.T) {
	want := errors.New("id source failed")
	if _, err := nextUniqueRoundID(func() (string, error) { return "", want }, map[string]bool{}); !errors.Is(err, want) {
		t.Fatalf("source error = %v", err)
	}
	if _, err := nextUniqueRoundID(func() (string, error) { return "", nil }, map[string]bool{}); err == nil {
		t.Fatal("empty id was accepted")
	}
	seen := map[string]bool{"same": true}
	if _, err := nextUniqueRoundID(func() (string, error) { return "same", nil }, seen); err == nil {
		t.Fatal("duplicate id was accepted")
	}
}

func TestCombinedLiveRejectsNonPositiveImplementationCadence(t *testing.T) {
	provider := &mutableReviewProvider{}
	reviews, sink := combinedFixture(t, provider)
	reader := &fakeReader{cadence: 0}
	if _, err := RunCombinedLive(t.Context(), constantSource(Plan{Tasks: []Task{{ID: "ready"}}}), reader, reviews,
		RecoveryPolicy{Bound: 1}, "scope", func(context.Context) error { return nil },
		func(context.Context, ReviewRecoveryRequest) error { return nil }, func() (string, error) { return "round", nil },
		sink, immediateClock{}, nil); err == nil {
		t.Fatal("non-positive implementation cadence was accepted")
	}
}
