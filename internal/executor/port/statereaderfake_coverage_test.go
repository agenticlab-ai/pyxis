package port

import (
	"context"
	"errors"
	"testing"
)

func TestFakeStateReaderRejectsInvalidFixtures(t *testing.T) {
	assertPanics := func(t *testing.T, fn func()) {
		t.Helper()
		defer func() {
			if recover() == nil {
				t.Fatal("call did not panic")
			}
		}()
		fn()
	}
	ref := ProposalRef{Repo: "repo", Number: 1}
	reason := UnknownOffline
	assertPanics(t, func() { NewFakeStateReader().Set(ref, ProposalState{Landing: LandingOpen, Unknown: &reason}) })
	assertPanics(t, func() { NewFakeStateReader().Set(ref, ProposalState{Landing: LandingUnknown}) })
	assertPanics(t, func() { NewFakeStateReader().Set(ref, ProposalState{Landing: LandingLanded}) })
	assertPanics(t, func() { NewFakeStateReader().SetUnknown(ref, UnknownReason(999)) })
	assertPanics(t, func() { _, _ = NewFakeStateReader().States(context.Background(), []ProposalRef{ref}, Cursor{}) })
}

func TestFakeStateReaderReturnsTopLevelFailures(t *testing.T) {
	want := errors.New("transport failed")
	fake := NewFakeStateReader()
	fake.SetErr(want)
	if _, err := fake.States(context.Background(), nil, Cursor{}); !errors.Is(err, want) {
		t.Fatalf("States(SetErr) = %v, want %v", err, want)
	}

	fake = NewFakeStateReader()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := fake.States(ctx, nil, Cursor{}); !errors.Is(err, context.Canceled) {
		t.Fatalf("States(cancelled) = %v, want context.Canceled", err)
	}
}
