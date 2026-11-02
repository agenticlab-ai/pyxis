// Package worker composes the five executor ports into the single-task cycle —
// preflight, claim, workspace, run agent, open + verify proposal, release
// (docs/tech-design/implement-now.md §3.1, §3.6). It is the reusable unit the
// local runtime and a future remote fleet share unchanged (DINV-10); it carries
// no concurrency and no runtime-specific code — the Dispatcher owns those.
package worker

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/executor/port"
	executorruntime "github.com/agenticlab-ai/pyxis/internal/executor/runtime"
	"github.com/agenticlab-ai/pyxis/internal/nativerouting"
	"github.com/agenticlab-ai/pyxis/internal/prdescription"
	"github.com/agenticlab-ai/pyxis/internal/routingevidence"
)

// Outcome is the terminal classification of one task a Worker ran.
type Outcome int

const (
	// OutcomeFailed is the zero value: the safe, fail-closed default. A task is
	// failed when it ran but could not be verified green, or when an operational
	// error occurred during the cycle.
	OutcomeFailed Outcome = iota
	// OutcomeSucceeded means a proposal exists and every required check is green
	// (DINV-3, AP-13, prd:implement-now/FR-10). It is the only success class.
	OutcomeSucceeded
	// OutcomeHollow means the agent finished cleanly but produced no proposal — a
	// clean exit with no artifact, never counted as succeeded (DINV-3).
	OutcomeHollow
	// OutcomeUnconverged means the Worker hit its mechanical turn/wall-clock bound
	// before converging: the agent process was killed and the task exits resumably
	// — its worktree branch persists as WIP (the Worker reaps nothing of its own).
	// It is never a success; the Dispatcher respawns it up to a cap, then escalates
	// (DINV-9, AP-16, prd:implement-now/FR-13).
	OutcomeUnconverged
)

// String renders the outcome for logs and test output.
func (o Outcome) String() string {
	switch o {
	case OutcomeSucceeded:
		return "succeeded"
	case OutcomeHollow:
		return "hollow"
	case OutcomeFailed:
		return "failed"
	case OutcomeUnconverged:
		return "unconverged"
	default:
		return fmt.Sprintf("Outcome(%d)", int(o))
	}
}

// Deps are the five consumer-side ports a Worker composes. All are required.
type Deps struct {
	Tracker port.WorkTracker
	Lease   port.Lease
	Space   port.Workspace
	Runtime port.AgentRuntime
	Host    port.CodeHost
}

// Worker runs the whole cycle for exactly one task against the ports in Deps. It
// holds no task list, no session cache and no sibling state — its only inputs
// are the ports and the single Task passed to Run (prd:implement-now/FR-3,
// DINV-11).
type Worker struct {
	tracker port.WorkTracker
	lease   port.Lease
	space   port.Workspace
	runtime port.AgentRuntime
	host    port.CodeHost

	// greenDriver, when set via WithGreenDriver, waits through pending CI and
	// drives an explicitly-red proposal to green before terminal classification
	// (§3.6). Nil by default: the base verify is unchanged and a non-green
	// proposal is classified failed as before.
	greenDriver GreenDriver

	reviewCapturer ReviewCapturer
	// reviewCapture and greenProposal are populated only on the genuinely-green
	// exit. The fresh Worker instance lets Dispatcher recover the independent review
	// terminal and proposal identity without changing Run's stable return shape.
	reviewCapture         *ReviewCapture
	greenProposal         *port.Proposal
	verificationAttempted bool
	greenVerified         GreenVerifiedObserver
	greenCaptured         GreenCapturedObserver

	// usage is the aggregate telemetry reported by this Worker's launched
	// implementation process. It is observational only and never participates in
	// success classification; Dispatcher sums it into EVT-16.
	usage              port.RunResult
	usageSeen          bool
	recoveryDiagnostic string
	activityObserver   func(port.Task, Activity)
	activeTask         port.Task

	// description is the pull request description the implementing agent
	// ended its final message with (internal/prdescription); described says
	// it was complete, and descriptionProblems says why it was not. Every
	// publish fills in the computed fields for the revision it pushes.
	description         prdescription.Description
	descriptionProblems []prdescription.Problem
	described           bool

	// budget, when set via WithBudget, is the uniform wall-clock bound the Worker
	// enforces around the agent's run — the mechanical turn bound for backends
	// whose CLI carries no native turn cap, uniform across all backends (DINV-9,
	// FR-13, §3.7). Zero (the default) means no wall-clock bound: only a backend's
	// native cap, where present, applies.
	budget time.Duration

	// implementationAdmission is the optional routing-evidence cutover seam.
	// When present, its exact-read permit is consumed immediately before the
	// ordinary lease boundary; claim mutual exclusion remains lease-owned.
	implementationAdmission ImplementationAdmission
	implementationOutcome   ImplementationOutcome

	// The local-first convergence loop (prepublish.go). prePublish, when set
	// via WithPrePublishReview, reviews the exact local revision after the local
	// gate is green and before any push, remediating on request-changes up to
	// remediationCap rounds. headReader and reviewPublisher decide and record
	// whether that review covers the published head; prePublishObs is the
	// reporting hook; prePublishOutcome is the attempt's loop record. All nil by
	// default: a Worker built without them publishes exactly as before.
	prePublish        PrePublishReviewer
	remediationCap    int
	headReader        ProposalHeadReader
	reviewPublisher   ReviewPublisher
	prePublishObs     PrePublishObserver
	prePublishOutcome *PrePublishOutcome

	// launchImpl is the implementation launcher bound in Run — plain or fenced
	// under an activated route — so every sublaunch of this attempt (the
	// remediation runs) starts under the same authority as the first process.
	// implModel/implEffort are the resolved implementation route it runs at.
	launchImpl   func(context.Context, port.LaunchSpec) (port.Handle, error)
	implModel    string
	implEffort   string
	implStateDir string

	// sessions, when set via WithSessionContinuity, lets every
	// implementation-role launch of this task (the implementation, its
	// remediation sublaunches, and — through SessionContinuityAware — the
	// drive-to-green fixers) continue the task's own harness conversation.
	// sessionObs is its reporting hook. Nil by default: fresh conversations.
	sessions      SessionContinuity
	sessionObs    SessionObserver
	localGateObs  func(port.Task, bool, int, int, string)
	checkpointObs DescribedCheckpointObserver
}

// WithLocalGateObserver reports the final local CI result, including a green
// first pass that consumes no fixer attempt.
func WithLocalGateObserver(observe func(port.Task, bool, int, int, string)) Option {
	return func(w *Worker) { w.localGateObs = observe }
}

// DescribedCheckpointObserver receives the committed checkpoint an attempt's
// complete description covers, once per attempt and before local
// verification, so an operator can later publish that exact checkpoint
// without the description being lost with the attempt. It is a reporting
// hook: it cannot change the attempt's course.
type DescribedCheckpointObserver func(ctx context.Context, task port.Task, work port.Workdir, description prdescription.Description)

// WithDescribedCheckpointObserver installs the described-checkpoint hook.
func WithDescribedCheckpointObserver(observe DescribedCheckpointObserver) Option {
	return func(w *Worker) { w.checkpointObs = observe }
}

// New builds a Worker from its ports and any options. Every port is required; a
// nil port is a wiring error, reported (never panicked) so the caller fails
// closed. The variadic options are additive — New(deps) builds the base Worker,
// so existing callers compile unchanged (prd:implement-now/FR-7 wires the green
// driver through WithGreenDriver without changing the base signature).
func New(deps Deps, opts ...Option) (*Worker, error) {
	switch {
	case deps.Tracker == nil:
		return nil, errors.New("worker: nil WorkTracker")
	case deps.Lease == nil:
		return nil, errors.New("worker: nil Lease")
	case deps.Space == nil:
		return nil, errors.New("worker: nil Workspace")
	case deps.Runtime == nil:
		return nil, errors.New("worker: nil AgentRuntime")
	case deps.Host == nil:
		return nil, errors.New("worker: nil CodeHost")
	}
	w := &Worker{
		tracker: deps.Tracker,
		lease:   deps.Lease,
		space:   deps.Space,
		runtime: deps.Runtime,
		host:    deps.Host,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(w)
		}
	}
	return w, nil
}

// Run executes the full cycle for one task and returns its terminal Outcome.
//
// The cycle is: preflight the backend's auth, claim the task, prepare an
// isolated workspace, run a fresh agent process, open a proposal, verify it by
// artifact, and release the claim. Its semantics:
//
//   - OutcomeSucceeded is returned only when a proposal exists AND every
//     required check is green (DINV-3, AP-13).
//   - A clean run that opens no proposal is OutcomeHollow, never succeeded
//     (DINV-3).
//   - Any other determined result — a non-green proposal — and any operational
//     error is OutcomeFailed.
//
// The claim is released on every terminal path (fire-or-release, DINV-5): the
// deferred release is armed only after a successful Acquire, and it runs even on
// context cancellation. ErrClaimLost is returned (wrapped) when the task is
// already held — a skip the Dispatcher distinguishes with errors.Is, not a work
// failure. The Worker reads only this Task; it never consults the frontier or
// any sibling/session state (prd:implement-now/FR-3, DINV-11).
func (w *Worker) Run(ctx context.Context, task port.Task) (outcome Outcome, runErr error) {
	w.activeTask = task
	ctx = executorruntime.WithOutputObserver(ctx, func(ref executorruntime.OutputReference) {
		w.activity(Activity{Status: "logs_opened", StdoutLog: ref.Stdout, StderrLog: ref.Stderr})
	})
	ctx, cleanupVerification, err := executorruntime.VerificationContext(ctx)
	if err != nil {
		return OutcomeFailed, err
	}
	defer cleanupVerification()
	var routedPermit ImplementationRoutePermit
	var routedNative port.NativeLaunchSpec
	var routeCapability port.LaunchCapability
	runtimeStarted := false
	// Preflight auth before claiming, so a missing credential consumes no
	// attempt (prd:implement-now/FR-16). The Dispatcher may also preflight a
	// backend once per sweep; this per-task check keeps the Worker self-standing.
	if err := w.runtime.AuthCheck(ctx); err != nil {
		return OutcomeFailed, fmt.Errorf("worker: auth preflight for task %s: %w", task.ID, err)
	}
	if w.implementationAdmission != nil {
		permit, err := w.implementationAdmission(ctx, task)
		if err != nil {
			return OutcomeFailed, fmt.Errorf("worker: route admission for task %s: %w", task.ID, err)
		}
		if permit == nil {
			return OutcomeFailed, fmt.Errorf("worker: route admission for task %s returned no permit", task.ID)
		}
		var ok bool
		routedPermit, ok = permit.(ImplementationRoutePermit)
		if !ok {
			return OutcomeFailed, fmt.Errorf("worker: route admission for task %s returned no fenced launch authority", task.ID)
		}
		if err := permit.Authorize(routingevidence.OperationClaim, routingevidence.BoundaryBinding{
			DemandID: string(task.DemandID), TaskID: task.ID, Harness: task.Harness,
			Role: nativerouting.RoleImplementation,
		}); err != nil {
			return OutcomeFailed, fmt.Errorf("worker: verify claim permit for task %s: %w", task.ID, err)
		}
		if w.implementationOutcome == nil {
			return OutcomeFailed, fmt.Errorf("worker: activated route for task %s has no outcome recorder", task.ID)
		}
		routedNative, err = routedPermit.NativeLaunch(nativerouting.RoleImplementation)
		if err != nil {
			return OutcomeFailed, fmt.Errorf("worker: native route for task %s: %w", task.ID, err)
		}
		ownership, err := routedPermit.RuntimeOwnershipProof()
		if err != nil {
			return OutcomeFailed, fmt.Errorf("worker: runtime ownership for task %s: %w", task.ID, err)
		}
		fenced, ok := w.runtime.(port.FencedRuntime)
		if !ok {
			return OutcomeFailed, fmt.Errorf("worker: activated backend for task %s has no fenced runtime", task.ID)
		}
		routeCapability, err = fenced.Activate(ownership.AttemptID, ownership.OwnerGeneration, ownership)
		if err != nil {
			return OutcomeFailed, fmt.Errorf("worker: activate runtime fence for task %s: %w", task.ID, err)
		}
		defer func() {
			terminal := routedImplementationTerminal(outcome, runErr, runtimeStarted, routeCapability, w.usage)
			terminalCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
			defer cancel()
			if err := w.implementationOutcome(terminalCtx, terminal); err != nil {
				outcome = OutcomeFailed
				runErr = errors.Join(runErr, fmt.Errorf("worker: persist routed outcome for task %s: %w", task.ID, err))
			}
		}()
	}

	// Claim the task. A lost race is a skip (propagated for the caller to
	// distinguish via errors.Is); only a held claim arms the deferred release.
	claim, err := w.lease.Acquire(ctx, task)
	if err != nil {
		return OutcomeFailed, fmt.Errorf("worker: acquire claim for task %s: %w", task.ID, err)
	}
	defer func() {
		// Fire-or-release (DINV-5): a held claim is released on every terminal
		// path, including cancellation — so release runs on a non-cancellable
		// copy of the context. A release error is reaped by age (AP-12) and does
		// not alter the already-determined outcome, so it is intentionally not
		// propagated here.
		_ = claim.Release(context.WithoutCancel(ctx))
	}()

	// Prepare the isolated workspace off freshly-fetched mainline (DINV-1/2).
	work, err := w.space.Prepare(ctx, task)
	if err != nil {
		return OutcomeFailed, fmt.Errorf("worker: prepare workspace for task %s: %w", task.ID, err)
	}

	// Run a fresh agent process for this task (DINV-11). The ordinary path reads
	// the route already resolved for this runtime. An activated admission
	// supplies the same pair from its native launch, below.
	var model, effort string
	if routedPermit == nil {
		route, err := port.RouteOf(w.runtime)
		if err != nil {
			return OutcomeFailed, fmt.Errorf("worker: model route for task %s: %w", task.ID, err)
		}
		model, effort = route.Model, route.Effort
	}
	// Task session continuity: hand the drive-to-green fixer the same seam
	// (its fixers are implementation-role launches of this task), then resolve
	// the session this launch continues. A feedback round's comments ride the
	// spec as data for the brief assembler.
	if aware, ok := w.greenDriver.(SessionContinuityAware); ok && w.sessions != nil {
		aware.UseSessionContinuity(w.sessions, task.Harness, w.sessionObs)
	}
	launchPurpose := "implementation"
	if task.Feedback != nil {
		launchPurpose = "feedback"
	}
	session := w.sessionFor(ctx, task, work, launchPurpose)
	stateDir, freshState, err := executorruntime.PrepareTaskState(ctx, work.Dir(), task.Harness)
	if err != nil {
		return OutcomeFailed, fmt.Errorf("worker: prepare task runtime: %w", err)
	}
	w.implStateDir = stateDir
	if freshState && session != nil && session.Resume {
		session.Resume = false
		w.observeSession(task, SessionEvent{Action: "degraded", Harness: task.Harness, SessionID: session.ID, Purpose: launchPurpose,
			Err: errors.New("prior conversation predates isolated runtime state; starting a fresh conversation")})
	}
	launchSpec := port.LaunchSpec{
		RecoveryDiagnostic: w.recoveryDiagnostic,
		RuntimeStateDir:    stateDir,
		Brief:              task.Brief,
		Purpose:            port.LaunchPurposeImplementation,
		Workdir:            work.Dir(),
		HeadBranch:         work.Branch(),
		Model:              model,
		Effort:             effort,
		Feedback:           task.Feedback,
		Session:            session,
		RequestDescription: true,
	}
	// Bind the implementation launcher once — plain, or fenced under the
	// activated route — so the remediation sublaunches of this attempt start
	// under exactly the same authority and route as the first process.
	if routedPermit == nil {
		w.launchImpl = w.runtime.Launch
	} else {
		fenced, ok := w.runtime.(port.FencedRuntime)
		if !ok {
			return OutcomeFailed, fmt.Errorf("worker: activated backend for task %s has no fenced runtime", task.ID)
		}
		launchSpec.Model, launchSpec.Effort = routedNative.NativeModel, routedNative.NativeEffort
		capability := routeCapability
		w.launchImpl = func(ctx context.Context, spec port.LaunchSpec) (port.Handle, error) {
			return fenced.LaunchFenced(ctx, spec, capability)
		}
	}
	w.implModel, w.implEffort = launchSpec.Model, launchSpec.Effort
	if binder, ok := w.greenDriver.(interface{ BindRoute(string, string) }); ok {
		binder.BindRoute(w.implModel, w.implEffort)
	}
	if binder, ok := w.greenDriver.(interface {
		BindAwaiter(func(context.Context, port.Handle) (port.RunResult, error))
	}); ok {
		binder.BindAwaiter(func(ctx context.Context, handle port.Handle) (port.RunResult, error) {
			return w.awaitPhase(ctx, handle, "gate_repair")
		})
	}
	if binder, ok := w.greenDriver.(interface {
		BindDescriber(func(context.Context, port.Workdir) error)
	}); ok {
		binder.BindDescriber(func(ctx context.Context, work port.Workdir) error {
			_, err := w.describeOpen(ctx, task, work)
			return err
		})
	}
	handle, err := w.launchImpl(ctx, launchSpec)
	if err != nil {
		return OutcomeFailed, fmt.Errorf("worker: launch agent for task %s: %w", task.ID, err)
	}
	runtimeStarted = true
	// Await blocks for the process to finish, bounded by the Worker's mechanical
	// turn/wall-clock budget where one is set (DINV-9, FR-13). Its resource usage
	// is telemetry, not a success verdict — the artifact decides that below
	// (DINV-3).
	res, err := w.awaitPhase(ctx, handle, launchPurpose)
	w.addUsage(res)
	w.recordSession(context.WithoutCancel(ctx), task, work, session, res, launchPurpose)
	// The agent's final message carries the proposal's description; a run its
	// budget cut short usually has none, and then cannot ship until a fresh
	// attempt finishes the handoff.
	w.description, w.descriptionProblems = prdescription.FromAgent(res.Output)
	w.described = len(w.descriptionProblems) == 0
	if err != nil {
		if errors.Is(err, errBudgetExhausted) {
			// The mechanical bound tripped: the process was killed mid-run. Hand it
			// to suspendUnconverged, which checkpoints the WIP and then decides by
			// artifact — shipping the work if it is locally green, else leaving it
			// committed for the Dispatcher to respawn (DINV-9, AP-16).
			return w.suspendUnconverged(ctx, work, task)
		}
		// Await has reaped the child. The detached,
		// bounded checkpoint preserves WIP without publishing cancelled work.
		if errors.Is(err, errNotQuiescent) {
			return OutcomeFailed, err
		}
		checkpointCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
		defer cancel()
		if _, checkpointErr := work.Checkpoint(checkpointCtx); checkpointErr != nil {
			err = errors.Join(err, fmt.Errorf("worker: checkpoint failed task %s: %w", task.ID, checkpointErr))
		}
		return OutcomeFailed, fmt.Errorf("worker: await agent for task %s: %w", task.ID, err)
	}
	// The process exited on its own, but the CLI can report it stopped BEFORE
	// finishing — it hit its own turn cap or errored internally (res.Incomplete)
	// rather than reaching a clean end. That is a recoverable truncation, not a
	// deliberate no-op: hand it to suspendUnconverged (checkpoint, then ship-if-
	// green or respawn), the same path as the wall-clock bound above. Otherwise a
	// plan-only run that has not written a line yet is indistinguishable from a
	// genuine hollow, and its work is lost, un-retried — the exact failure mode
	// that stranded a task's first attempt.
	if res.Incomplete {
		if res.NonRetryable {
			cause := fmt.Errorf("%w: %s", port.ErrRunNonRetryable, res.Reason)
			checkpointCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
			defer cancel()
			if _, checkpointErr := work.Checkpoint(checkpointCtx); checkpointErr != nil {
				cause = errors.Join(cause, fmt.Errorf("checkpoint refused run: %w", checkpointErr))
			}
			return OutcomeFailed, cause
		}
		return w.suspendUnconverged(ctx, work, task)
	}

	// The publish flow checkpoints before revision-based verification.
	return w.publishAndVerify(ctx, task, work, false)
}

func routedImplementationTerminal(outcome Outcome, runErr error, launched bool, capability port.LaunchCapability, usage port.RunResult) routingevidence.AttemptTerminal {
	terminal := routingevidence.AttemptTerminal{AttemptID: capability.AttemptID, OwnerGeneration: int(capability.OwnerGeneration),
		LaunchCapability: capability, LaunchCapabilityRef: capability.CapabilityRef}
	switch {
	case errors.Is(runErr, port.ErrClaimLost):
		terminal.Outcome, terminal.Cause = routingevidence.OutcomeBlocked, routingevidence.CauseClaimLost
		terminal.LaunchFidelity, terminal.UsageProvenance = routingevidence.FidelityUnavailable, routingevidence.UsageNotLaunched
	case !launched:
		if errors.Is(runErr, context.Canceled) || errors.Is(runErr, context.DeadlineExceeded) {
			terminal.Outcome, terminal.Cause = routingevidence.OutcomeBlocked, routingevidence.CauseCancelled
		} else {
			terminal.Outcome, terminal.Cause = routingevidence.OutcomeFailed, routingevidence.CausePreflightFailed
		}
		terminal.LaunchFidelity, terminal.UsageProvenance = routingevidence.FidelityUnavailable, routingevidence.UsageNotLaunched
	case errors.Is(runErr, context.Canceled) || errors.Is(runErr, context.DeadlineExceeded):
		terminal.Outcome, terminal.Cause = routingevidence.OutcomeBlocked, routingevidence.CauseCancelled
		terminal.LaunchFidelity, terminal.UsageProvenance = routingevidence.FidelityExact, routingevidence.UsageRuntimeObserved
		terminal.Turns = usage.Turns
	case outcome == OutcomeSucceeded && runErr == nil:
		terminal.Outcome, terminal.Cause = routingevidence.OutcomeVerified, routingevidence.CauseTerminalVerified
		terminal.LaunchFidelity, terminal.UsageProvenance = routingevidence.FidelityExact, routingevidence.UsageRuntimeObserved
		terminal.Turns = usage.Turns
	default:
		terminal.Outcome, terminal.Cause = routingevidence.OutcomeFailed, routingevidence.CauseWorkerFailed
		terminal.LaunchFidelity, terminal.UsageProvenance = routingevidence.FidelityExact, routingevidence.UsageRuntimeObserved
		terminal.Turns = usage.Turns
	}
	return terminal
}

// Usage returns the implementation process telemetry observed by this Worker.
// It is safe after Run returns; a Worker is never shared across attempts.
func (w *Worker) Usage() port.RunResult { return w.usage }

// VerificationAttempted distinguishes a real proposal check from failures in
// implementation or local convergence, before any remote CI existed.
func (w *Worker) VerificationAttempted() bool { return w.verificationAttempted }

// publishAndVerify runs the local-first convergence loop and then the
// deterministic publish-and-verify tail (DINV-3), shared by a clean agent exit
// and a bound-truncated run whose committed WIP already passes the local gate.
// Before anything is pushed: the tree is committed; the CI-mirroring local
// gate must be green (its bounded fixer loop runs here, never against the code
// host); when a pre-publish reviewer is installed, the exact local revision
// must be review-accepted, remediating on
// request-changes within the remediation bound (prepublish.go). Any failure
// there fails the task closed with the work committed on its branch — no
// proposal is opened and zero remote self-heal iterations are spent
// (DINV-6, DINV-7). Once fit, it publishes the supervisor's checkpointed branch
// (prd:implement-now/FR-10), opens a proposal, and classifies by
// artifact: green is success; a non-green proposal is driven to green through the
// bounded remote self-heal loop when a driver is installed, else failed. A
// worktree with no commits beyond its base publishes nothing and is hollow — a
// clean no-op (DINV-3), no remote round-trip. Every genuinely-green exit routes
// through admitSucceeded, where the pre-publish review's coverage of the
// published head is decided.
func (w *Worker) publishAndVerify(ctx context.Context, task port.Task, work port.Workdir, committed bool) (Outcome, error) {
	// Commit whatever the agent left uncommitted so the local gate, the
	// reviewer and the push all see one revision. Nothing beyond the base is a
	// clean no-op — hollow, never succeeded (DINV-3) — and spends no gate, no
	// review and no remote round-trip. A caller that already checkpointed (the
	// truncated path) skips the redundant commit.
	if !committed {
		hasWork, err := work.Checkpoint(ctx)
		if err != nil {
			return OutcomeFailed, fmt.Errorf("worker: checkpoint task %s before the local gate: %w", task.ID, err)
		}
		if !hasWork {
			return OutcomeHollow, nil
		}
	}

	// A proposal is published with its description or not at all. Without one
	// the agent's handoff is incomplete, the same as a run its budget cut
	// short: the work stays committed and a fresh attempt resumes it with the
	// reason, bounded by the respawn cap (DINV-9).
	if !w.described {
		w.recoveryDiagnostic = w.descriptionDiagnostic()
		return OutcomeUnconverged, nil
	}
	if w.checkpointObs != nil {
		w.checkpointObs(ctx, task, work, w.description)
	}

	// Local-first convergence (prepublish.go): the CI-mirroring local gate
	// (with its bounded fixer loop) must be green, and — when a pre-publish reviewer is installed — the
	// exact local revision must be review-accepted, remediating on
	// request-changes within the remediation bound. Only then is anything
	// pushed. Every failure here is fail-closed with the work committed on its
	// branch and nothing published (DINV-6, DINV-7, AP-16).
	if err := w.localGreen(ctx, task, work); err != nil {
		return OutcomeFailed, err
	}
	if w.prePublish != nil {
		if err := w.reviewLoop(ctx, task, work); err != nil {
			return OutcomeFailed, err
		}
	}

	// Describe an already open proposal (a feedback round, a resumed attempt)
	// before the push, so the checks that run on the pushed revision read its
	// description.
	description, err := w.describeOpen(ctx, task, work)
	if err != nil {
		return OutcomeFailed, err
	}
	published, err := work.Push(ctx)
	if err != nil {
		return OutcomeFailed, fmt.Errorf("worker: publish branch for task %s: %w", task.ID, err)
	}
	if !published {
		return OutcomeHollow, nil
	}

	proposal, err := w.host.OpenProposal(ctx, task, work, description)
	switch {
	case errors.Is(err, port.ErrNoProposal):
		// Clean run, nothing to propose: hollow, never succeeded (DINV-3).
		return OutcomeHollow, nil
	case err != nil:
		return OutcomeFailed, fmt.Errorf("worker: open proposal for task %s: %w", task.ID, err)
	}

	// A proposal exists; post a best-effort progress note on this task's issue.
	w.notifyProposalOpened(ctx, task, proposal)

	w.verificationAttempted = true
	state, err := w.host.Checks(ctx, proposal)
	if err != nil {
		return OutcomeFailed, fmt.Errorf("worker: read checks for task %s: %w", task.ID, err)
	}
	switch state {
	case port.CheckGreen:
		// Ladder-green: the proposal is verified green. The mandate and route
		// admission (below) still decides whether it counts succeeded.
		return w.admitSucceeded(ctx, task, proposal)
	case port.CheckPending, port.CheckRed:
		// Both are not-green, but they are not equivalent. The GreenDriver waits
		// through PENDING without inference and enters self-heal only after an
		// explicit RED read. Keeping the three states explicit here prevents the
		// just-opened/no-check-runs race from silently becoming a fixer call.
	default:
		return OutcomeFailed, fmt.Errorf("worker: unsupported check state %s for task %s", state, task.ID)
	}

	// A proposal exists but is not green. With a green driver installed, wait for
	// pending CI or drive an explicitly-red result through the bounded self-heal
	// loop before classifying (§3.6, prd:implement-now/FR-7, FR-9). Without one,
	// the base cycle classifies a non-green proposal as failed — unchanged
	// behavior (DINV-3).
	if w.greenDriver != nil {
		res, err := w.greenDriver.Drive(ctx, task, work, proposal)
		if err != nil {
			// The drive did not converge: a bound-exhausted escalation or an
			// operational failure. Either way the task is failed (fail-closed,
			// DINV-3); the error carries the escalation/context up to the report
			// (§3.4, §3.9). The attempt count is bounded and observable (FR-9).
			return OutcomeFailed, fmt.Errorf("worker: drive task %s to green: %w", task.ID, err)
		}
		if res.Green {
			// Ladder-green reached via the self-heal drive: the same succeeded
			// exit as the direct path.
			return w.admitSucceeded(ctx, task, proposal)
		}
		// A non-error, non-green drive should not occur (the driver returns an
		// error on non-convergence), but fail closed if it does.
		return OutcomeFailed, nil
	}
	return OutcomeFailed, nil
}

// admitSucceeded is publishAndVerify's single succeeded exit, reached from
// both its genuinely-green paths (the direct check read and the post-drive
// re-check). It captures the independent impl-review obligation and returns succeeded.
func (w *Worker) admitSucceeded(ctx context.Context, task port.Task, proposal port.Proposal) (Outcome, error) {
	w.greenProposal = &proposal
	// When the pre-publish review accepted exactly the head the proposal now
	// carries, that review covers it: the artifact is landed on the proposal
	// and no post-green obligation is captured. A moved head (the remote
	// self-heal loop re-pushed) or any doubt keeps the post-green review owed
	// on the new revision (AP-13).
	covered := w.coverPublished(ctx, proposal)
	if w.reviewCapturer != nil && !covered {
		capture := w.reviewCapturer.Capture(ctx, task, proposal)
		w.reviewCapture = &capture
		if !capture.Failed() && w.greenCaptured != nil {
			if err := w.greenCaptured(task, proposal, capture); err != nil {
				capture.Err = fmt.Errorf("complete configured-review handoff: %w", err)
				w.reviewCapture = &capture
			}
		}
	}
	if w.greenVerified != nil {
		w.greenVerified(task)
	}
	return OutcomeSucceeded, nil
}

// suspendUnconverged handles a run that stopped off the success path (a turn cap
// or an internal error). It first checkpoints the agent's WIP — committing it so
// the respawn's idempotent resume (AP-22) continues from a clean base and the
// work is not stranded uncommitted on cap exhaustion. It then decides by ARTIFACT
// (DINV-3), not by the truncation itself:
//
//   - No work beyond base: the run produced nothing, so it is respawn-eligible
//     (its next window starts fresh) — never shipped, never a terminal hollow.
//   - Committed work that passes the local CI-mirroring gate: shippable — the
//     agent's turns ran out, not the work. Publish + open a proposal (converge)
//     rather than escalate green work with the PR never opened.
//   - Committed but not locally green (or no gate installed to verify): respawn
//     from the committed base — never a churny PR opened from partial work.
//
// A checkpoint or gate FAULT (a git/tooling failure, distinct from a check
// legitimately failing) fails closed (INV-6): the operator sees it, and the WIP
// stays committed for the next attempt or manual recovery.
func (w *Worker) suspendUnconverged(ctx context.Context, work port.Workdir, task port.Task) (Outcome, error) {
	hasWork, err := work.Checkpoint(ctx)
	if err != nil {
		return OutcomeFailed, fmt.Errorf("worker: checkpoint WIP for task %s: %w", task.ID, err)
	}
	if !hasWork {
		// Nothing committed beyond base — respawn a fresh attempt (DINV-9); never
		// a terminal hollow, which would strand a truncated run un-retried.
		return OutcomeUnconverged, nil
	}
	// Verify by artifact: only ship truncated work the local gate certifies green,
	// so partial/red work respawns instead of opening a churny PR. Without a gate
	// (no green driver) the Worker cannot verify locally, so it respawns rather
	// than publish unverified truncated work.
	if w.greenDriver != nil {
		var green bool
		var err error
		if diagnosticGate, ok := w.greenDriver.(interface {
			InspectLocal(context.Context, port.Workdir) (LocalGateResult, error)
		}); ok {
			var result LocalGateResult
			result, err = diagnosticGate.InspectLocal(ctx, work)
			green, w.recoveryDiagnostic = result.Green, result.Diagnostic
		} else {
			green, err = w.greenDriver.GateGreen(ctx, work)
		}
		if err != nil {
			return OutcomeFailed, fmt.Errorf("worker: local gate for task %s after truncation: %w", task.ID, err)
		}
		if green {
			return w.publishAndVerify(ctx, task, work, true)
		}
	}
	return OutcomeUnconverged, nil
}

// describeOpen fills in the computed fields of the agent's description for the
// revision about to be pushed and puts it on the open proposal, if there is
// one. The drive-to-green loop calls it before each re-push.
func (w *Worker) describeOpen(ctx context.Context, task port.Task, work port.Workdir) (prdescription.Description, error) {
	d, err := Describe(ctx, task, work, w.description)
	if err != nil {
		return d, err
	}
	if _, err := w.host.DescribeProposal(ctx, task, work, d); err != nil {
		return d, fmt.Errorf("worker: describe the open proposal for task %s: %w", task.ID, err)
	}
	return d, nil
}

// Describe fills in the computed fields of an implementer's description for
// the revision work holds: Lines of code from the branch, and Complexity from
// the task's routing assessment when it has one.
func Describe(ctx context.Context, task port.Task, work port.Workdir, d prdescription.Description) (prdescription.Description, error) {
	size, err := work.ChangeSize(ctx)
	if err != nil {
		return d, fmt.Errorf("worker: measure task %s's change for its description: %w", task.ID, err)
	}
	d.LinesOfCode = size.String()
	if demand, err := task.ImplementationDemand(); err == nil {
		if level, ok := prdescription.ComplexityFor(demand.Capability); ok {
			d.Complexity = fmt.Sprintf("%s (implementation capability: %s)", level, demand.Capability)
		}
	}
	return d, nil
}

// descriptionDiagnostic is the recovery handoff for an attempt that ended
// without a complete description. It is data for the next attempt's brief.
func (w *Worker) descriptionDiagnostic() string {
	var b strings.Builder
	b.WriteString("The previous attempt's final message did not give a complete pull request description. The work is committed; check it, finish anything left, and end your final message with the complete description. Problems:")
	for _, p := range w.descriptionProblems {
		b.WriteString("\n- " + p.String())
	}
	return b.String()
}

// notifyProposalOpened posts a best-effort comment linking the task's issue to
// its freshly-opened proposal. Any error is intentionally swallowed: the
// notification is not the success artifact (DINV-3), so a tracker hiccup must
// not fail an otherwise-good task. It touches only this task's issue number,
// never a sibling.
func (w *Worker) notifyProposalOpened(ctx context.Context, task port.Task, p port.Proposal) {
	body := fmt.Sprintf("Opened proposal #%d from branch %s.", p.Number, p.Branch)
	_ = w.tracker.Comment(ctx, task.Number, body)
}
