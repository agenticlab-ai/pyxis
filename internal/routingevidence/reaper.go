package routingevidence

import (
	"context"
	"errors"
	"fmt"
	"sort"

	"github.com/agenticlab-ai/pyxis/internal/executor/port"
)

// ReaperOutcomeAuthorizer is deliberately narrower than owner authority. The
// configured reference represents the operator/policy authorization checked by
// the reaper's producer capability; it can sign only a crash-reaped terminal.
type ReaperOutcomeAuthorizer struct{ AuthorizationReference string }

func (a ReaperOutcomeAuthorizer) AuthorizeOutcome(_ context.Context, terminal AttemptTerminal, attempt AttemptView) error {
	if a.AuthorizationReference == "" {
		return errors.New("routing outcome: reaper authorization is absent")
	}
	if terminal.Cause != CauseCrashReaped || terminal.Outcome != OutcomeBlocked || terminal.OwnerGeneration != attempt.OwnerGeneration {
		return errors.New("routing outcome: reaper may close only the current generation as crash_reaped/blocked")
	}
	return nil
}

// OverdueResolvedSource owns the timeout/lease policy. The routing evidence
// layer never infers overdue state from event or git time; it receives explicit
// candidates and independently rechecks canonical state from a fresh read.
type OverdueResolvedSource interface {
	OverdueResolved(context.Context) ([]string, error)
}

type AttemptObserver interface {
	FenceAndObserve(context.Context, string, port.OwnerGeneration) (port.RuntimeFenceProof, port.RuntimeObservation, error)
}

type AttemptCompleter interface {
	CompleteAttempt(context.Context, AttemptTerminal) (EvidenceProof, error)
}

// ReapTerminalSource joins the runtime's private terminal telemetry to the
// bounded canonical terminal. It must not expose prompts, output, or code.
type ReapTerminalSource interface {
	CrashTerminal(context.Context, AttemptView, port.RuntimeFenceProof) (AttemptTerminal, error)
}

type ReapState string

const (
	ReapAlreadyTerminal ReapState = "already_terminal"
	ReapReattached      ReapState = "reattached"
	ReapTransferable    ReapState = "transferable"
	ReapNonAccepting    ReapState = "non_accepting"
	ReapCompleted       ReapState = "completed"
)

type ReapResult struct {
	AttemptID    string
	State        ReapState
	Observation  port.RuntimeObservationKind
	Handle       port.Handle
	FenceProof   port.RuntimeFenceProof
	OutcomeProof EvidenceProof
	Err          error
}

type AttemptReaper struct {
	Store     Store
	Discovery OverdueResolvedSource
	Runtime   AttemptObserver
	Terminals ReapTerminalSource
	Completer AttemptCompleter
}

// ReapOverdue fresh-reads every explicit overdue candidate. One broken
// attempt remains visible in its result and does not prevent sibling recovery.
func (r *AttemptReaper) ReapOverdue(ctx context.Context) ([]ReapResult, error) {
	if r == nil || r.Discovery == nil {
		return nil, errors.New("routing outcome reaper: overdue source is unavailable")
	}
	ids, err := r.Discovery.OverdueResolved(ctx)
	if err != nil {
		return nil, fmt.Errorf("routing outcome reaper: discover overdue attempts: %w", err)
	}
	sort.Strings(ids)
	results := make([]ReapResult, 0, len(ids))
	for i, id := range ids {
		if err := ctx.Err(); err != nil {
			return results, err
		}
		if id == "" || (i > 0 && id == ids[i-1]) {
			continue
		}
		result, reapErr := r.ReapResolved(ctx, id)
		result.Err = reapErr
		results = append(results, result)
	}
	return results, nil
}

// ReapResolved never launches. A running process is returned for reattachment;
// proven non-start/absence returns the exact fence proof needed by the separate
// ownership-transfer flow; unknown liveness remains visibly non-accepting. Only
// a runtime terminal is converted to the bounded crash-reaped outcome.
func (r *AttemptReaper) ReapResolved(ctx context.Context, attemptID string) (ReapResult, error) {
	result := ReapResult{AttemptID: attemptID}
	if r == nil || r.Store == nil || r.Runtime == nil || attemptID == "" {
		return result, errors.New("routing outcome reaper: store, runtime, and attempt identity are required")
	}
	read, err := r.Store.ReadSubject(ctx, SubjectID{Kind: SubjectAttempt, ID: attemptID})
	if err != nil {
		return result, fmt.Errorf("routing outcome reaper: exact-read attempt: %w", err)
	}
	if read.Subject == nil || read.Subject.Attempt == nil || read.Subject.Attempt.AttemptID != attemptID {
		return result, errors.New("routing outcome reaper: canonical attempt is absent")
	}
	attempt := *read.Subject.Attempt
	if attempt.OutcomeEventID != "" {
		result.State = ReapAlreadyTerminal
		return result, nil
	}
	if attempt.State != StateResolvedPendingOutcome || attempt.OwnerGeneration < 1 {
		return result, fmt.Errorf("routing outcome reaper: attempt state %s is not resolved pending outcome", attempt.State)
	}
	proof, observation, err := r.Runtime.FenceAndObserve(ctx, attemptID, port.OwnerGeneration(attempt.OwnerGeneration))
	if err != nil {
		return result, fmt.Errorf("routing outcome reaper: fence and observe: %w", err)
	}
	if proof.AttemptID != attemptID || int(proof.OwnerGeneration) != attempt.OwnerGeneration ||
		observation.AttemptID != attemptID || int(observation.OwnerGeneration) != attempt.OwnerGeneration ||
		proof.Observation != observation.Kind || proof.Nonce == "" || proof.ProofReference == "" || proof.Signature == "" {
		return result, errors.New("routing outcome reaper: runtime observation differs from its fence proof")
	}
	result.Observation, result.FenceProof = observation.Kind, proof
	switch observation.Kind {
	case port.RuntimeRunning:
		if observation.Handle == nil {
			return result, errors.New("routing outcome reaper: running observation has no reattach handle")
		}
		result.State, result.Handle = ReapReattached, observation.Handle
		return result, nil
	case port.RuntimeNotStarted, port.RuntimeAbsentProven:
		result.State = ReapTransferable
		return result, nil
	case port.RuntimeUnknown:
		result.State = ReapNonAccepting
		return result, nil
	case port.RuntimeTerminal:
		if r.Terminals == nil || r.Completer == nil {
			return result, errors.New("routing outcome reaper: terminal source and completer are required")
		}
		terminal, terminalErr := r.Terminals.CrashTerminal(ctx, attempt, proof)
		if terminalErr != nil {
			return result, fmt.Errorf("routing outcome reaper: recover terminal: %w", terminalErr)
		}
		if terminal.AttemptID != attemptID || terminal.OwnerGeneration != attempt.OwnerGeneration || terminal.Cause != CauseCrashReaped {
			return result, errors.New("routing outcome reaper: recovered terminal differs from the fenced current attempt")
		}
		result.OutcomeProof, err = r.Completer.CompleteAttempt(ctx, terminal)
		if err != nil {
			return result, fmt.Errorf("routing outcome reaper: complete terminal: %w", err)
		}
		result.State = ReapCompleted
		return result, nil
	default:
		return result, fmt.Errorf("routing outcome reaper: unknown observation %q", observation.Kind)
	}
}
