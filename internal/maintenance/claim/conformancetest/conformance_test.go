package conformancetest

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/maintenance/claim"
)

type claimStep struct {
	result claim.ClaimResult
	err    error
}

type transitionStep struct {
	record claim.Record
	err    error
}

type scriptedStore struct {
	claims      []claimStep
	transitions []transitionStep
}

func (s *scriptedStore) Claim(context.Context, claim.Record) (claim.ClaimResult, error) {
	step := s.claims[0]
	s.claims = s.claims[1:]
	return step.result, step.err
}

func (s *scriptedStore) Transition(context.Context, claim.Record, claim.Terminal) (claim.Record, error) {
	step := s.transitions[0]
	s.transitions = s.transitions[1:]
	return step.record, step.err
}

func TestCheck(t *testing.T) {
	if err := check(context.Background(), validFixture()); err != nil {
		t.Fatalf("check valid fixture: %v", err)
	}

	tests := []struct {
		name string
		want string
		fail func(Fixture)
	}{
		{"first claim", "first claim", func(f Fixture) { f.First.(*scriptedStore).claims[0].err = errors.New("unavailable") }},
		{"competing claim", "competing claim", func(f Fixture) { f.Second.(*scriptedStore).claims[0].err = errors.New("unavailable") }},
		{"losing transition", "losing owner", func(f Fixture) { f.Second.(*scriptedStore).transitions[0].err = nil }},
		{"winning transition", "winning owner", func(f Fixture) { f.First.(*scriptedStore).transitions[0].err = errors.New("write failed") }},
		{"retained terminal", "terminal occurrence", func(f Fixture) { f.Second.(*scriptedStore).claims[1].err = errors.New("unavailable") }},
		{"next date", "next date", func(f Fixture) { f.First.(*scriptedStore).claims[1].err = errors.New("unavailable") }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fixture := validFixture()
			tt.fail(fixture)
			if err := check(context.Background(), fixture); err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("check error = %v, want containing %q", err, tt.want)
			}
		})
	}
}

func TestRun(t *testing.T) {
	Run(t, func(*testing.T) Fixture { return validFixture() })
}

func validFixture() Fixture {
	initial := claim.Record{SchemaVersion: claim.SchemaVersion, JobID: "daily", JobDate: "2026-10-20", OwnerNonce: "owner-a"}
	competitor := initial
	competitor.OwnerNonce = "owner-b"
	next := initial
	next.JobDate = "2026-10-21"
	next.OwnerNonce = "owner-c"
	terminal := initial
	terminal.Outcome = claim.OutcomeFailed
	return Fixture{
		First: &scriptedStore{
			claims: []claimStep{
				{result: claim.ClaimResult{Disposition: claim.DispositionOwned, Record: initial}},
				{result: claim.ClaimResult{Disposition: claim.DispositionOwned, Record: next}},
			},
			transitions: []transitionStep{{record: terminal}},
		},
		Second: &scriptedStore{
			claims: []claimStep{
				{result: claim.ClaimResult{Disposition: claim.DispositionConsumed, Record: initial}},
				{result: claim.ClaimResult{Disposition: claim.DispositionConsumed, Record: terminal}},
			},
			transitions: []transitionStep{{err: errors.New("not owner")}},
		},
		Initial:    initial,
		Competitor: competitor,
		NextDate:   next,
		Terminal:   claim.Terminal{Outcome: claim.OutcomeFailed},
	}
}
