package routingevidence

import (
	"context"
	"errors"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/executor/port"
)

type reaperDiscovery []string

func (d reaperDiscovery) OverdueResolved(context.Context) ([]string, error) {
	return append([]string(nil), d...), nil
}

type failingReaperDiscovery struct{ err error }

func (d failingReaperDiscovery) OverdueResolved(context.Context) ([]string, error) {
	return nil, d.err
}

type reaperObserver struct {
	kind   port.RuntimeObservationKind
	handle port.Handle
	err    error
	calls  int
}

func (o *reaperObserver) FenceAndObserve(_ context.Context, attempt string, generation port.OwnerGeneration) (port.RuntimeFenceProof, port.RuntimeObservation, error) {
	o.calls++
	if o.err != nil {
		return port.RuntimeFenceProof{}, port.RuntimeObservation{}, o.err
	}
	proof := port.RuntimeFenceProof{AttemptID: attempt, OwnerGeneration: generation, Observation: o.kind, Nonce: "nonce", ProofReference: "fence-proof", Signature: "signed"}
	observation := port.RuntimeObservation{AttemptID: attempt, OwnerGeneration: generation, Kind: o.kind, Handle: o.handle}
	return proof, observation, nil
}

type reaperHandle struct{}

func (reaperHandle) RuntimeHandle() {}

type reaperTerminalSource struct{ calls int }

func (s *reaperTerminalSource) CrashTerminal(_ context.Context, attempt AttemptView, _ port.RuntimeFenceProof) (AttemptTerminal, error) {
	s.calls++
	return AttemptTerminal{AttemptID: attempt.AttemptID, Outcome: OutcomeBlocked, Cause: CauseCrashReaped,
		OwnerGeneration: attempt.OwnerGeneration, LaunchCapabilityRef: "capability-recovered", LaunchFidelity: FidelityUnavailable,
		UsageProvenance: UsageRuntimeUnavailable}, nil
}

type reaperCompleter struct {
	calls    int
	terminal AttemptTerminal
}

func (c *reaperCompleter) CompleteAttempt(_ context.Context, terminal AttemptTerminal) (EvidenceProof, error) {
	c.calls++
	c.terminal = terminal
	return EvidenceProof{EvidenceRef: "refs/pyxis/routing-evidence", Commit: "outcome-head",
		Entries: []EvidenceEntry{{EventID: "outcome", ContentDigest: "digest"}}}, nil
}

func reaperFixture(t *testing.T, kind port.RuntimeObservationKind) (*AttemptReaper, *reaperObserver, *reaperTerminalSource, *reaperCompleter) {
	t.Helper()
	_, store, _, _ := outcomeServiceFixture(t, "implementation")
	observer := &reaperObserver{kind: kind}
	terminals := &reaperTerminalSource{}
	completer := &reaperCompleter{}
	return &AttemptReaper{Store: store, Discovery: reaperDiscovery{"attempt-7"}, Runtime: observer, Terminals: terminals, Completer: completer}, observer, terminals, completer
}

func TestAttemptReaperRoutesEveryRuntimeObservationWithoutLaunching(t *testing.T) {
	tests := []struct {
		kind port.RuntimeObservationKind
		want ReapState
	}{
		{port.RuntimeRunning, ReapReattached},
		{port.RuntimeNotStarted, ReapTransferable},
		{port.RuntimeAbsentProven, ReapTransferable},
		{port.RuntimeUnknown, ReapNonAccepting},
		{port.RuntimeTerminal, ReapCompleted},
	}
	for _, tc := range tests {
		t.Run(string(tc.kind), func(t *testing.T) {
			reaper, observer, terminals, completer := reaperFixture(t, tc.kind)
			if tc.kind == port.RuntimeRunning {
				observer.handle = reaperHandle{}
			}
			got, err := reaper.ReapResolved(t.Context(), "attempt-7")
			if err != nil {
				t.Fatal(err)
			}
			if got.State != tc.want || got.Observation != tc.kind || observer.calls != 1 {
				t.Fatalf("result = %+v, observation calls=%d", got, observer.calls)
			}
			wantTerminalCalls := 0
			if tc.kind == port.RuntimeTerminal {
				wantTerminalCalls = 1
			}
			if terminals.calls != wantTerminalCalls || completer.calls != wantTerminalCalls {
				t.Fatalf("terminal/completer calls = %d/%d, want %d", terminals.calls, completer.calls, wantTerminalCalls)
			}
			if (tc.kind == port.RuntimeNotStarted || tc.kind == port.RuntimeAbsentProven) && got.FenceProof.ProofReference == "" {
				t.Fatal("transferable observation lost exact fence proof")
			}
		})
	}
}

func TestAttemptReaperFreshDiscoveryDeduplicatesAndKeepsFailuresVisible(t *testing.T) {
	reaper, observer, _, _ := reaperFixture(t, port.RuntimeUnknown)
	reaper.Discovery = reaperDiscovery{"attempt-7", "attempt-7", ""}
	results, err := reaper.ReapOverdue(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].State != ReapNonAccepting || results[0].Err != nil || observer.calls != 1 {
		t.Fatalf("results = %+v, calls=%d", results, observer.calls)
	}

	observer.err = errors.New("runtime unavailable")
	results, err = reaper.ReapOverdue(t.Context())
	if err != nil || len(results) != 1 || results[0].Err == nil {
		t.Fatalf("failed sibling result = %+v, err=%v", results, err)
	}
}

func TestAttemptReaperRejectsFailedResolutionAndMismatchedProof(t *testing.T) {
	reaper, observer, _, _ := reaperFixture(t, port.RuntimeUnknown)
	store := reaper.Store.(*outcomeTestStore)
	store.attempt.State = StateResolutionFailed
	if _, err := reaper.ReapResolved(t.Context(), "attempt-7"); err == nil || observer.calls != 0 {
		t.Fatalf("failed resolution reached runtime: err=%v calls=%d", err, observer.calls)
	}

	store.attempt.State = StateResolvedPendingOutcome
	observer.kind = "foreign"
	if _, err := reaper.ReapResolved(t.Context(), "attempt-7"); err == nil {
		t.Fatal("unknown runtime observation accepted")
	}
}

func TestAttemptReaperBoundaryRefusals(t *testing.T) {
	if err := (ReaperOutcomeAuthorizer{}).AuthorizeOutcome(t.Context(), AttemptTerminal{}, AttemptView{}); err == nil {
		t.Fatal("empty reaper authorization was accepted")
	}
	if _, err := (*AttemptReaper)(nil).ReapOverdue(t.Context()); err == nil {
		t.Fatal("nil reaper discovery was accepted")
	}
	wantErr := errors.New("discovery failed")
	if _, err := (&AttemptReaper{Discovery: failingReaperDiscovery{err: wantErr}}).ReapOverdue(t.Context()); !errors.Is(err, wantErr) {
		t.Fatalf("discovery error = %v", err)
	}

	reaper, _, _, _ := reaperFixture(t, port.RuntimeUnknown)
	reaper.Discovery = reaperDiscovery{"attempt-7"}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := reaper.ReapOverdue(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled reap error = %v", err)
	}
	if _, err := (&AttemptReaper{}).ReapResolved(t.Context(), ""); err == nil {
		t.Fatal("incomplete resolved reaper was accepted")
	}

	reaper, observer, _, _ := reaperFixture(t, port.RuntimeRunning)
	observer.handle = nil
	if _, err := reaper.ReapResolved(t.Context(), "attempt-7"); err == nil {
		t.Fatal("running observation without handle was accepted")
	}
}
