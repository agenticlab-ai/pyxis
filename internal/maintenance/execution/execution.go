// Package execution runs one positively claimed maintenance occurrence.
package execution

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/entity"
	"github.com/agenticlab-ai/pyxis/internal/executor/port"
	"github.com/agenticlab-ai/pyxis/internal/maintenance/admission"
	"github.com/agenticlab-ai/pyxis/internal/maintenance/claim"
	"github.com/agenticlab-ai/pyxis/internal/maintenance/workspace"
	"github.com/agenticlab-ai/pyxis/internal/nativerouting"
)

var (
	ErrUnsupportedDelivery = errors.New("maintenance delivery is not supported")
	ErrLifecycle           = errors.New("maintenance lifecycle observation failed")
	ErrTerminalMetadata    = errors.New("maintenance terminal metadata was not confirmed")
)

type Outcome string

const (
	OutcomeRefused    Outcome = "refused"
	OutcomeConsumed   Outcome = "consumed"
	OutcomeSucceeded  Outcome = "succeeded"
	OutcomeFailed     Outcome = "failed"
	OutcomeTimedOut   Outcome = "timed_out"
	OutcomeIncomplete Outcome = "incomplete"
	OutcomeUnproved   Outcome = "unproved"
)

type Result struct {
	Outcome   Outcome
	Claim     claim.ClaimResult
	Workspace *workspace.Prepared
	Usage     port.RunResult
	Capture   *port.LaunchCapture
	Harness   entity.Harness
	Shared    *claim.Record
	ElapsedMS *int64
	Launched  bool
	Quiescent bool
	Retained  bool
}

type ClaimService interface {
	Acquire(context.Context, admission.Prepared) (claim.ClaimResult, error)
	Transition(context.Context, claim.Record, claim.Terminal) (claim.Record, error)
}

type Preparer interface {
	Prepare(context.Context, admission.Prepared, claim.ClaimResult) (workspace.Prepared, error)
}

type Boundary string

const (
	BoundaryAdmission Boundary = "admission"
	BoundaryClaim     Boundary = "claim"
	BoundaryWorkspace Boundary = "workspace"
	BoundaryAgent     Boundary = "agent"
	BoundaryTerminal  Boundary = "terminal"
)

type Observation struct {
	Boundary  Boundary
	Result    Result
	CauseCode string
	Cause     error
}

type Lifecycle interface {
	Observe(context.Context, Observation) error
}

type Service struct {
	Claims         ClaimService
	Workspaces     Preparer
	Lifecycle      Lifecycle
	Now            func() time.Time
	CleanupTimeout time.Duration
	FinalTimeout   time.Duration
}

func (s Service) Run(ctx context.Context, prepared admission.Prepared) (result Result, err error) {
	now := time.Now
	if s.Now != nil {
		now = s.Now
	}
	defer func() { result, err = s.finish(ctx, prepared, result, err, now) }()
	if prepared.Job.Delivery != nil {
		result.Outcome, result.Quiescent = OutcomeRefused, true
		return result, ErrUnsupportedDelivery
	}
	if s.Claims == nil || s.Workspaces == nil {
		result.Outcome, result.Quiescent = OutcomeRefused, true
		return result, errors.New("maintenance execution requires claim and workspace services")
	}
	runtime, native, err := prepared.ExecutionRoute()
	if err != nil {
		result.Outcome, result.Quiescent = OutcomeRefused, true
		return result, err
	}
	if !now().Before(prepared.DeadlineUTC) {
		result.Outcome, result.Quiescent = OutcomeRefused, true
		return result, fmt.Errorf("maintenance deadline expired before claim: %w", context.DeadlineExceeded)
	}
	deadline, cancelDeadline := context.WithDeadline(ctx, prepared.DeadlineUTC)
	defer cancelDeadline()
	attempt, cancel := context.WithCancelCause(deadline)
	defer cancel(nil)
	if err := attempt.Err(); err != nil {
		result.Outcome, result.Quiescent = OutcomeRefused, true
		return result, err
	}
	if err := s.observe(attempt, BoundaryAdmission, result, "audit_admitted", nil); err != nil {
		result.Outcome, result.Quiescent = OutcomeRefused, true
		return result, err
	}
	result.Claim, err = s.Claims.Acquire(attempt, prepared)
	if err != nil {
		result.Outcome, result.Quiescent, result.Retained = OutcomeUnproved, true, true
		err = errors.Join(err, s.observe(attempt, BoundaryClaim, result, "claim_unproved", err))
		return result, err
	}
	if result.Claim.Disposition == claim.DispositionConsumed {
		result.Outcome, result.Quiescent = OutcomeConsumed, true
		return result, s.observe(attempt, BoundaryClaim, result, consumedCode(result.Claim.Record), nil)
	}
	result.Retained = true
	if err := s.observe(attempt, BoundaryClaim, result, "claim_owned", nil); err != nil {
		return failed(result, attempt, err)
	}
	space, err := s.Workspaces.Prepare(attempt, prepared, result.Claim)
	if err != nil {
		return failed(result, attempt, err)
	}
	result.Workspace = &space
	if err := s.observe(attempt, BoundaryWorkspace, result, "workspace_prepared", nil); err != nil {
		return failed(result, attempt, err)
	}
	spec := port.LaunchSpec{Brief: prepared.Job.Prompt, Purpose: port.LaunchPurposeImplementation,
		Workdir: space.Workdir, RuntimeStateDir: space.RuntimeStateDir, HeadBranch: space.HeadBranch,
		Model: native.NativeModel, Effort: native.NativeEffort, Session: nil}
	observed := &launchObserver{ctx: attempt, lifecycle: s.Lifecycle, cancel: cancel, result: &result, expected: prepared.Selection}
	launchCtx := port.WithInvocationObserver(attempt, observed)
	var handle port.Handle
	err = prepared.GuardReady(launchCtx, func() error {
		var launchErr error
		handle, launchErr = runtime.Launch(launchCtx, spec)
		return launchErr
	})
	if err != nil {
		return failed(result, attempt, fmt.Errorf("launch maintenance runtime: %w", err))
	}
	if handle == nil {
		return failed(result, attempt, fmt.Errorf("launch returned no handle: %w", port.ErrNotQuiescent))
	}
	if !observed.launched() {
		cancel(errors.New("maintenance native launch was not observed"))
	}
	result.Usage, err = s.await(attempt, ctx, runtime, handle)
	err = errors.Join(observed.err(), err)
	if err != nil {
		return failed(result, attempt, err)
	}
	result.Quiescent = true
	if result.Usage.Incomplete {
		result.Outcome = OutcomeIncomplete
		return result, nil
	}
	result.Outcome = OutcomeSucceeded
	return result, nil
}

func (s Service) observe(ctx context.Context, boundary Boundary, result Result, code string, cause error) error {
	if s.Lifecycle == nil {
		return nil
	}
	if err := s.Lifecycle.Observe(ctx, Observation{Boundary: boundary, Result: result, CauseCode: code, Cause: cause}); err != nil {
		return errors.Join(ErrLifecycle, fmt.Errorf("observe maintenance %s: %w", boundary, err))
	}
	return nil
}

func (s Service) finish(ctx context.Context, prepared admission.Prepared, result Result, runErr error, now func() time.Time) (Result, error) {
	bound := s.FinalTimeout
	if bound <= 0 {
		bound = 10 * time.Second
	}
	final, cancel := context.WithTimeout(context.WithoutCancel(ctx), bound)
	defer cancel()
	finished := now().UTC()
	if !prepared.StartedUTC.IsZero() && !finished.Before(prepared.StartedUTC) {
		elapsed := finished.Sub(prepared.StartedUTC).Milliseconds()
		result.ElapsedMS = &elapsed
	}
	code := outcomeCode(result, runErr)
	if result.Claim.Disposition == claim.DispositionOwned && result.Quiescent {
		terminal := claim.Terminal{Outcome: sharedOutcome(result.Outcome), FinishedAt: finished, DurationMS: result.ElapsedMS,
			Summary: outcomeSummary(result.Outcome), Cause: code, Evidence: retainedEvidence(result)}
		confirmed, err := s.Claims.Transition(final, result.Claim.Record, terminal)
		if err != nil {
			runErr, code = errors.Join(runErr, ErrTerminalMetadata, err), "terminal_metadata_failed"
		} else {
			result.Shared = &confirmed
		}
	}
	auditErr := s.observe(final, BoundaryTerminal, result, code, runErr)
	return result, errors.Join(runErr, auditErr)
}

func sharedOutcome(outcome Outcome) claim.Outcome {
	switch outcome {
	case OutcomeSucceeded:
		return claim.OutcomeSucceeded
	case OutcomeTimedOut:
		return claim.OutcomeTimedOut
	default:
		return claim.OutcomeFailed
	}
}

func outcomeCode(result Result, cause error) string {
	if errors.Is(cause, ErrLifecycle) {
		return "audit_failed"
	}
	switch result.Outcome {
	case OutcomeSucceeded:
		return "execution_completed"
	case OutcomeTimedOut:
		return "execution_timed_out"
	case OutcomeIncomplete:
		return "execution_incomplete"
	case OutcomeConsumed:
		return consumedCode(result.Claim.Record)
	case OutcomeUnproved:
		return "claim_unproved"
	case OutcomeRefused:
		return "execution_refused"
	default:
		return "execution_failed"
	}
}

func consumedCode(record claim.Record) string {
	if record.Outcome == "" {
		return "date_consumed"
	}
	return "date_consumed_" + string(record.Outcome)
}

func outcomeSummary(outcome Outcome) string {
	return "maintenance execution " + string(outcome)
}

func retainedEvidence(result Result) []string {
	if result.Retained && result.Workspace != nil && result.Claim.Record.OwnerNonce != "" {
		return []string{"maintenance-attempt:" + result.Claim.Record.OwnerNonce}
	}
	return nil
}

type launchObserver struct {
	ctx       context.Context
	lifecycle Lifecycle
	cancel    context.CancelCauseFunc
	result    *Result
	expected  nativerouting.SelectedProfile
	mu        sync.Mutex
	fired     bool
	cause     error
}

func (o *launchObserver) Launched(harness entity.Harness, purpose port.LaunchPurpose, capture port.LaunchCapture) func(port.RunResult, error) {
	o.mu.Lock()
	o.fired, o.result.Launched, o.result.Capture, o.result.Harness = true, true, &capture, harness
	if harness != o.expected.Harness || purpose != port.LaunchPurposeImplementation || !capture.MatchesSelected(o.expected) {
		o.cause = errors.New("maintenance runtime observed an unexpected launch identity")
		o.cancel(o.cause)
	}
	if o.lifecycle != nil {
		if err := o.lifecycle.Observe(o.ctx, Observation{Boundary: BoundaryAgent, Result: *o.result, CauseCode: "agent_launched"}); err != nil {
			o.cause = errors.Join(o.cause, ErrLifecycle, fmt.Errorf("observe maintenance agent: %w", err))
			o.cancel(o.cause)
		}
	}
	o.mu.Unlock()
	return func(port.RunResult, error) {}
}

func (o *launchObserver) launched() bool { o.mu.Lock(); defer o.mu.Unlock(); return o.fired }
func (o *launchObserver) err() error     { o.mu.Lock(); defer o.mu.Unlock(); return o.cause }

func (s Service) await(attempt, caller context.Context, runtime port.AgentRuntime, handle port.Handle) (port.RunResult, error) {
	type completion struct {
		usage port.RunResult
		err   error
	}
	done := make(chan completion, 1)
	go func() {
		usage, err := runtime.Await(attempt, handle)
		done <- completion{usage: usage, err: err}
	}()
	select {
	case completed := <-done:
		return completed.usage, errors.Join(context.Cause(attempt), completed.err)
	case <-attempt.Done():
	}
	cause := context.Cause(attempt)
	cleanupBound := s.CleanupTimeout
	if cleanupBound <= 0 {
		cleanupBound = 5 * time.Second
	}
	cleanup, cancel := context.WithTimeout(context.WithoutCancel(caller), cleanupBound)
	defer cancel()
	if err := runtime.Kill(cleanup, handle); err != nil {
		cause = errors.Join(cause, attempt.Err(), fmt.Errorf("%w: terminate maintenance runtime: %w", port.ErrNotQuiescent, err))
	}
	select {
	case completed := <-done:
		return completed.usage, errors.Join(cause, completed.err)
	case <-cleanup.Done():
		select {
		case completed := <-done:
			return completed.usage, errors.Join(cause, completed.err)
		default:
		}
		return port.RunResult{}, errors.Join(cause, port.ErrNotQuiescent)
	}
}

func failed(result Result, ctx context.Context, err error) (Result, error) {
	result.Quiescent = !errors.Is(err, port.ErrNotQuiescent)
	switch {
	case errors.Is(err, port.ErrNotQuiescent):
		result.Outcome = OutcomeIncomplete
	case errors.Is(err, context.DeadlineExceeded) || errors.Is(context.Cause(ctx), context.DeadlineExceeded):
		result.Outcome = OutcomeTimedOut
	default:
		result.Outcome = OutcomeFailed
	}
	return result, err
}
