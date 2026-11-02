// Package conformancetest owns the common ClaimStore contract assertions.
package conformancetest

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/maintenance/claim"
)

type Fixture struct {
	First      claim.ClaimStore
	Second     claim.ClaimStore
	Initial    claim.Record
	Competitor claim.Record
	NextDate   claim.Record
	Terminal   claim.Terminal
}

func Run(t *testing.T, fixture func(*testing.T) Fixture) {
	t.Helper()
	if err := check(t.Context(), fixture(t)); err != nil {
		t.Fatal(err)
	}
}

func check(ctx context.Context, f Fixture) error {
	winner, err := f.First.Claim(ctx, f.Initial)
	if err != nil || winner.Disposition != claim.DispositionOwned || !claim.SameOwner(f.Initial, winner.Record) {
		return fmt.Errorf("first claim did not prove ownership: result=%+v err=%v", winner, err)
	}
	loser, err := f.Second.Claim(ctx, f.Competitor)
	if err != nil || loser.Disposition != claim.DispositionConsumed || !claim.SameOwner(f.Initial, loser.Record) {
		return fmt.Errorf("competing claim did not observe consumed date: result=%+v err=%v", loser, err)
	}
	if _, err := f.Second.Transition(ctx, f.Competitor, f.Terminal); err == nil {
		return errors.New("losing owner made a terminal transition")
	}
	terminal, err := f.First.Transition(ctx, f.Initial, f.Terminal)
	if err != nil || terminal.Outcome != f.Terminal.Outcome || !claim.SameOwner(f.Initial, terminal) {
		return fmt.Errorf("winning owner terminal transition: record=%+v err=%v", terminal, err)
	}
	retained, err := f.Second.Claim(ctx, f.Competitor)
	if err != nil || retained.Disposition != claim.DispositionConsumed || retained.Record.Outcome != f.Terminal.Outcome {
		return fmt.Errorf("terminal occurrence was not retained as consumed: result=%+v err=%v", retained, err)
	}
	next, err := f.First.Claim(ctx, f.NextDate)
	if err != nil || next.Disposition != claim.DispositionOwned {
		return fmt.Errorf("next date was not independently eligible: result=%+v err=%v", next, err)
	}
	return nil
}
