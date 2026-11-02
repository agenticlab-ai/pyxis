package watch

import (
	"context"
	"testing"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/executor/port"
)

// A new comment on an open proposal moves the transition key (the wake the
// human-feedback round rides on); the same comments leave it byte-identical.
func TestDeriveKeyMovesOnNewProposalComment(t *testing.T) {
	ref := port.ProposalRef{Repo: "o/r", Number: 7}
	plan := Plan{Tasks: []Task{{ID: "T1", Proposal: &ref}}}
	reader := port.NewFakeStateReader()
	t0 := time.Date(2026, 9, 1, 10, 0, 0, 0, time.UTC)
	reader.Set(ref, port.ProposalState{Landing: port.LandingOpen, Checks: []port.CheckContext{{Name: "ci", State: port.CheckGreen}},
		Comments: []port.Comment{{Author: "bot", CreatedAt: t0, Body: "impl-review report"}}})
	first, err := Derive(context.Background(), plan, reader)
	if err != nil {
		t.Fatal(err)
	}
	same, _ := Derive(context.Background(), plan, reader)
	if first.key() != same.key() {
		t.Fatal("unchanged comments must not move the key")
	}
	reader.Set(ref, port.ProposalState{Landing: port.LandingOpen, Checks: []port.CheckContext{{Name: "ci", State: port.CheckGreen}},
		Comments: []port.Comment{{Author: "bot", CreatedAt: t0, Body: "impl-review report"}, {Author: "human", CreatedAt: t0.Add(time.Minute), Body: "please rename"}}})
	next, _ := Derive(context.Background(), plan, reader)
	if next.key() == first.key() {
		t.Fatal("a new comment must move the transition key")
	}
	if len(next.Awaits) != 1 || !next.Awaits[0].LastActivity.Equal(t0.Add(time.Minute)) {
		t.Fatalf("await = %+v", next.Awaits)
	}
}
