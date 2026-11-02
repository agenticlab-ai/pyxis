package main

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/maintenance/admission"
	"github.com/agenticlab-ai/pyxis/internal/maintenance/audit"
	"github.com/agenticlab-ai/pyxis/internal/maintenance/claim"
	"github.com/agenticlab-ai/pyxis/internal/maintenance/execution"
)

type maintenanceResetProbe struct {
	err   error
	calls int
}

func (p *maintenanceResetProbe) ResetRefusals(ctx context.Context, _ string) error {
	p.calls++
	return errors.Join(p.err, ctx.Err())
}

type maintenanceAppendProbe struct{ events []audit.Event }

func (p *maintenanceAppendProbe) Append(ctx context.Context, event audit.Event, _ audit.Durability) (audit.Result, error) {
	if err := ctx.Err(); err != nil {
		return audit.Result{}, err
	}
	p.events = append(p.events, event)
	return audit.Result{}, nil
}

func TestMaintenanceResetsOnlyProvedCoordinationAndAuditsResetFailure(t *testing.T) {
	failure := errors.New("injected counter reset failure")
	for _, tc := range []struct {
		name           string
		disposition    claim.Disposition
		fail, canceled bool
		resets         int
	}{
		{"owned", claim.DispositionOwned, false, false, 1},
		{"consumed", claim.DispositionConsumed, false, false, 1},
		{"unproved", claim.Disposition(""), false, false, 0},
		{"owned reset fails after cancellation", claim.DispositionOwned, true, true, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			counter, journal := &maintenanceResetProbe{}, &maintenanceAppendProbe{}
			if tc.fail {
				counter.err = failure
			}
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			if tc.canceled {
				cancel()
			}
			identity := audit.Identity{TraceID: strings.Repeat("a", 32), InvocationID: strings.Repeat("b", 32)}
			observer := maintenanceExecutionLifecycle{Journal: journal, Identity: identity, Admitted: admission.Result{JobID: "daily"}, Refusals: counter}
			result := execution.Result{Claim: claim.ClaimResult{Disposition: tc.disposition, Record: claim.Record{JobID: "daily", OwnerNonce: strings.Repeat("c", 64)}}}
			err := observer.Observe(ctx, execution.Observation{Boundary: execution.BoundaryClaim, Result: result, CauseCode: "fixture"})
			if errors.Is(err, failure) != tc.fail || counter.calls != tc.resets {
				t.Fatalf("reset boundary: err=%v calls=%d want failure=%t calls=%d", err, counter.calls, tc.fail, tc.resets)
			}
			if tc.fail {
				if len(journal.events) != 1 || journal.events[0].CauseCode != "counter_write_failed" || journal.events[0].Claim == nil || journal.events[0].Claim.Owner != result.Claim.Record.OwnerNonce {
					t.Fatalf("reset failure lost its correlated audit diagnostic: %+v", journal.events)
				}
			}
		})
	}
}
