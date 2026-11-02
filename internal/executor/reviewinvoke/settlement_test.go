package reviewinvoke

import (
	"context"
	"errors"
	"slices"
	"testing"
	"testing/synctest"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/executor/port"
	"github.com/agenticlab-ai/pyxis/internal/executor/reviewobligation"
	"github.com/agenticlab-ai/pyxis/internal/executor/worker"
	"github.com/agenticlab-ai/pyxis/internal/implreview"
	implreport "github.com/agenticlab-ai/pyxis/internal/implreview/report"
)

var settlementSteps = []string{"process", "proof", "publication", "receipt", "verification", "pair"}

type settlementDelays struct {
	delays map[string]time.Duration
	calls  []string
}

func (d *settlementDelays) wait(ctx context.Context, step string) error {
	d.calls = append(d.calls, step)
	timer := time.NewTimer(d.delays[step])
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return ctx.Err()
	}
}

type delayedSettlementProcess struct {
	implreview.ProcessCoordinator
	delays *settlementDelays
}

func (p delayedSettlementProcess) Complete(ctx context.Context, admitted implreview.AdmittedProcess, handoff implreview.ProcessHandoff) (implreview.AggregateHandoff, error) {
	// Failure cleanup uses its own bound and does not replay the completed handoff.
	if handoff.Status == implreview.HandoffCompleted {
		if err := p.delays.wait(ctx, "process"); err != nil {
			return implreview.AggregateHandoff{}, err
		}
	}
	return p.ProcessCoordinator.Complete(ctx, admitted, handoff)
}

type delayedSettlementVerifier struct {
	ReviewTerminalVerifier
	delays *settlementDelays
}

func (v delayedSettlementVerifier) VerifyAttemptProof(ctx context.Context, obligation reviewobligation.Obligation, processID, proofRef string) (implreview.VerifiedPublicationBinding, error) {
	if err := v.delays.wait(ctx, "proof"); err != nil {
		return implreview.VerifiedPublicationBinding{}, err
	}
	return v.ReviewTerminalVerifier.VerifyAttemptProof(ctx, obligation, processID, proofRef)
}

func (v delayedSettlementVerifier) VerifyPair(ctx context.Context, publication implreview.PublicationTerminal, host worker.ReviewPublicationTerminal) error {
	if err := v.delays.wait(ctx, "verification"); err != nil {
		return err
	}
	return v.ReviewTerminalVerifier.VerifyPair(ctx, publication, host)
}

type delayedSettlementPersistence struct {
	implreview.HostPersistenceAdapter
	delays *settlementDelays
}

func (p delayedSettlementPersistence) Complete(ctx context.Context, binding implreview.VerifiedPublicationBinding, handoff implreview.AggregateHandoff) (implreview.PublicationTerminal, error) {
	if err := p.delays.wait(ctx, "publication"); err != nil {
		return implreview.PublicationTerminal{}, err
	}
	return p.HostPersistenceAdapter.Complete(ctx, binding, handoff)
}

func (p delayedSettlementPersistence) ExactRead(ctx context.Context, receipt string) (*implreport.Report, error) {
	if err := p.delays.wait(ctx, "receipt"); err != nil {
		return nil, err
	}
	return p.HostPersistenceAdapter.ExactRead(ctx, receipt)
}

type delayedSettlementStore struct {
	reviewobligation.Store
	delays *settlementDelays
}

func (s delayedSettlementStore) RecordPair(ctx context.Context, pair reviewobligation.PublicationPair) error {
	if err := s.delays.wait(ctx, "pair"); err != nil {
		return err
	}
	return s.Store.RecordPair(ctx, pair)
}

func settlementInvoker(t *testing.T, delays *settlementDelays) (*Invoker, *fakeRuntime) {
	t.Helper()
	runtime := &fakeRuntime{}
	i := lifecycleInvoker(t, runtime, &fakeVerifier{}, time.Minute)
	i.processes = delayedSettlementProcess{i.processes, delays}
	i.terminalVerifier = delayedSettlementVerifier{i.terminalVerifier, delays}
	i.persistence = delayedSettlementPersistence{i.persistence, delays}
	i.store = delayedSettlementStore{i.store, delays}
	return i, runtime
}

func TestReviewSettlementAllowsCumulativeLatencyBeyondThirtySeconds(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		delays := &settlementDelays{delays: make(map[string]time.Duration)}
		for _, step := range settlementSteps {
			delays.delays[step] = 8 * time.Second
		}
		i, runtime := settlementInvoker(t, delays)
		started := time.Now()
		result := i.Invoke(context.Background(), port.Task{ID: "T5"}, port.Proposal{Number: 548, Branch: "feat/x"})
		if result.Err != nil || result.Status != "verified" || runtime.launches != 1 {
			t.Fatalf("bounded successful settlement failed: result=%+v launches=%d", result, runtime.launches)
		}
		if !slices.Equal(delays.calls, settlementSteps) || time.Since(started) != 48*time.Second {
			t.Fatalf("settlement calls=%v elapsed=%v", delays.calls, time.Since(started))
		}
		state, found, err := i.store.State(context.Background(), result.ObligationID)
		if err != nil || !found || state.Status != "verified" || state.Pair == nil {
			t.Fatalf("accepted review was not durably paired and settled: state=%+v found=%v err=%v", state, found, err)
		}
	})
}

func TestReviewSettlementPhaseOverrunFailsClosed(t *testing.T) {
	for index, step := range settlementSteps {
		t.Run(step, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				delays := &settlementDelays{delays: map[string]time.Duration{step: 31 * time.Second}}
				i, _ := settlementInvoker(t, delays)
				started := time.Now()
				result := i.Invoke(context.Background(), port.Task{ID: "T5"}, port.Proposal{Number: 548, Branch: "feat/x"})
				if !errors.Is(result.Err, context.DeadlineExceeded) || result.Status != "failed" || result.ObligationStatus != "pending" {
					t.Fatalf("phase overrun accepted: %+v", result)
				}
				if !slices.Equal(delays.calls, settlementSteps[:index+1]) || time.Since(started) != 30*time.Second {
					t.Fatalf("overrun continued settlement: calls=%v elapsed=%v", delays.calls, time.Since(started))
				}
				state, found, err := i.store.State(context.Background(), result.ObligationID)
				if err != nil || !found || state.Status != "review_pending" || state.Pair != nil {
					t.Fatalf("overrun minted accepted evidence: state=%+v found=%v err=%v", state, found, err)
				}
				terminal, found, err := i.store.Terminal(context.Background(), result.ObligationID, result.AttemptID)
				if err != nil || !found || terminal.Status != "failed" {
					t.Fatalf("failed attempt was not retained: terminal=%+v found=%v err=%v", terminal, found, err)
				}
			})
		})
	}
}

func TestReviewSettlementPhaseCannotExtendOverallDeadline(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), settlementBudget)
		defer cancel()
		started := time.Now()
		time.Sleep(175 * time.Second)
		_, err := settleReviewPhase(ctx, func(phaseCtx context.Context) (struct{}, error) {
			return struct{}{}, (&settlementDelays{delays: map[string]time.Duration{"last": 8 * time.Second}}).wait(phaseCtx, "last")
		})
		if !errors.Is(err, context.DeadlineExceeded) || time.Since(started) != 3*time.Minute {
			t.Fatalf("phase extended overall deadline: elapsed=%v err=%v", time.Since(started), err)
		}
		called := false
		_, err = settleReviewPhase(ctx, func(context.Context) (struct{}, error) {
			called = true
			return struct{}{}, nil
		})
		if called || !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("expired settlement started another operation: called=%v err=%v", called, err)
		}
	})
}

func TestReviewSettlementRejectsLateSuccessAndPreservesContext(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx := implreview.WithNativeModel(context.Background(), "review-model")
		var retained context.Context
		_, err := settleReviewPhase(ctx, func(phaseCtx context.Context) (string, error) {
			retained = phaseCtx
			if model, found := implreview.NativeModel(phaseCtx); !found || model != "review-model" {
				t.Fatalf("phase lost native model binding: model=%q found=%v", model, found)
			}
			// An adapter that overlooks cancellation must not authorize later steps.
			time.Sleep(31 * time.Second)
			return "late success", nil
		})
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("late success accepted: %v", err)
		}
		if retained.Err() == nil {
			t.Fatal("phase context remained live after operation returned")
		}
	})
}
