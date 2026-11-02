package dispatch

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/entity"
	"github.com/agenticlab-ai/pyxis/internal/executor/port"
	"github.com/agenticlab-ai/pyxis/internal/executor/report"
	"github.com/agenticlab-ai/pyxis/internal/executor/worker"
)

// This file is the T13 integration seam: the additive Dispatcher options that let
// the /implement-now surface wire per-task backend routing, drive-to-green, and
// the normalized two-sink event stream onto the frontier-sweep — without changing
// the base Sweep contract (docs/tech-design/implement-now.md §3.4, §3.8, §3.9).
// A Dispatcher built with no Option behaves exactly as before: a single runtime,
// no green driver, no events.

// RuntimeResolver resolves the AgentRuntime for one task. It returns the
// resolved harness name alongside the runtime, so task_dispatched names the
// harness the task actually ran on (§3.4). It is a plain closure rather than a
// domain port; a composition root may capture the run context when it needs to
// discover an authenticated local provider.
type RuntimeResolver func(task port.Task) (harness entity.Harness, rt port.AgentRuntime, err error)

// GreenDriverFactory builds the drive-to-green GreenDriver bound to one task's
// resolved runtime, so a task's CI-fix fixer runs a fresh process on the SAME
// backend it was dispatched to (DINV-11, §3.8). It is invoked once per ready
// task. A nil factory (the default) leaves the base verify unchanged — a non-green
// proposal stays failed — so drive-to-green (prd:implement-now/FR-7) is opt-in
// through this seam, exactly as worker.WithGreenDriver makes it opt-in per Worker.
type GreenDriverFactory func(rt port.AgentRuntime) (worker.GreenDriver, error)

// ReviewInvokerFactory builds one impl-review invoker for an already-selected
// harness. The Dispatcher calls it once per distinct effective review harness
// in the ready frontier, then shares that preflighted service across tasks
// routed to the same harness.
type ReviewInvokerFactory func(harness entity.Harness) (worker.ReviewService, error)

// ImplementationAdmissionFactory binds the shared routing-evidence admission
// service to the concrete runtime selected for one task. It is installed only
// by the all-path activation composition.
type ImplementationAdmissionFactory func(port.AgentRuntime) (worker.ImplementationAdmission, error)
type ImplementationOutcomeFactory func(port.AgentRuntime) (worker.ImplementationOutcome, error)

// ReviewHandoff is one persisted obligation whose implementation terminal did
// not yet receive its canonical handoff marker before process interruption.
type ReviewHandoff struct {
	ObligationID string
	Task         port.Task
	Harness      entity.Harness
	ProposalRef  string
	Revision     string
	ReviewResult worker.ReviewResult
}

// ReviewHandoffRepairer exact-reads the incomplete green-boundary frontier and
// commits its marker only after Dispatcher republishes the succeeded terminal.
type ReviewHandoffRepairer interface {
	Pending(context.Context) ([]ReviewHandoff, error)
	Complete(context.Context, string) error
}

// ReviewPairResult is one pair_verified crash tail repaired before any backend
// setup. The task identity is kept beside the review result for EVT-15.
type ReviewPairResult struct {
	TaskID string
	Result worker.ReviewResult
}

// ReviewPairLifecycleRepairer is an optional capability on the handoff
// repairer. Its separate interface preserves existing adapters while allowing
// the production adapter to authenticate and finish pair_verified tails before
// any implementation or review runtime is considered.
type ReviewPairLifecycleRepairer interface {
	RepairPairs(context.Context) ([]ReviewPairResult, error)
}

// Option configures a Dispatcher at construction. Options are additive: New with
// no Option builds the base Dispatcher, so every existing caller compiles and
// behaves unchanged.
type Option func(*Dispatcher)

// WithRuntimeResolver installs per-task backend resolution (§3.8). With a resolver
// set, Deps.Runtime is optional — each task's runtime is resolved from its
// charter through the resolver, so one sweep can route different tasks to
// different backends. A nil resolver is ignored (the base single-runtime path).
func WithRuntimeResolver(resolve RuntimeResolver) Option {
	return func(d *Dispatcher) {
		if resolve != nil {
			d.resolveRuntime = resolve
		}
	}
}

// WithGreenDriverFactory attaches drive-to-green: each ready Worker is built with
// a GreenDriver bound to that task's resolved runtime, driving a red proposal
// toward green before terminal classification (§3.6, prd:implement-now/FR-7). A
// nil factory is ignored (the base verify, unchanged).
func WithGreenDriverFactory(factory GreenDriverFactory) Option {
	return func(d *Dispatcher) {
		if factory != nil {
			d.greenFactory = factory
		}
	}
}

// WithReviewInvokerFactory attaches the impl-review invocation seam (#1029).
// The Dispatcher constructs and preflights one invoker per distinct effective
// backend before execution, then runs the matching invoker only for a verified-
// green proposal after its implementation claim and width slot are released.
// A nil factory leaves review uninstalled.
func WithReviewInvokerFactory(factory ReviewInvokerFactory) Option {
	return func(d *Dispatcher) {
		if factory != nil {
			d.reviewFactory = factory
		}
	}
}

func WithImplementationAdmissionFactory(factory ImplementationAdmissionFactory) Option {
	return func(d *Dispatcher) {
		if factory != nil {
			d.implementationAdmissionFactory = factory
		}
	}
}

func WithImplementationOutcomeFactory(factory ImplementationOutcomeFactory) Option {
	return func(d *Dispatcher) {
		if factory != nil {
			d.implementationOutcomeFactory = factory
		}
	}
}

// WithDescribedCheckpointObserver hands every Worker the hook that observes
// the committed checkpoint its attempt's complete description covers
// (worker.WithDescribedCheckpointObserver). A nil observer is ignored.
func WithDescribedCheckpointObserver(observe worker.DescribedCheckpointObserver) Option {
	return func(d *Dispatcher) {
		if observe != nil {
			d.describedCheckpoint = observe
		}
	}
}

// WithProposalHeadReader installs the exact proposal-head read the local-first
// loop uses to decide whether its accepted review covers the published head
// (worker.WithProposalHeadReader). Without it the Worker never assumes coverage
// and the post-green review stays owed. A nil reader is ignored.
func WithProposalHeadReader(h worker.ProposalHeadReader) Option {
	return func(d *Dispatcher) {
		if h != nil {
			d.headReader = h
		}
	}
}

// WithReviewPublisher installs the proposal-side publication of a covering
// pre-publish review artifact (worker.WithReviewPublisher). A nil publisher is
// ignored: the artifact then lives in the review process store and the report.
func WithReviewPublisher(p worker.ReviewPublisher) Option {
	return func(d *Dispatcher) {
		if p != nil {
			d.reviewPublisher = p
		}
	}
}

// prePublishObserver projects the Worker's local-first loop moments onto the
// event stream (review_round, remediation_attempt) without the worker package
// importing report.
type prePublishObserver struct{ d *Dispatcher }

func (o prePublishObserver) ReviewRound(task port.Task, r worker.PrePublishReview) {
	reason := ""
	if r.Err != nil {
		reason = r.Err.Error()
	}
	o.d.emit(report.ReviewRound(report.ReviewRoundFields{TaskID: task.ID, Issue: task.Number, Round: r.Round,
		Verdict: r.Verdict, Revision: r.Revision, ProposalRef: r.ProposalRef, ReviewID: r.ReviewID, Blocking: r.Blocking,
		Model: r.Model, Effort: r.Effort, Phase: r.Phase, FailureClass: r.FailureClass, Reason: reason}))
}

func (o prePublishObserver) RemediationAttempt(task port.Task, attempt, cap int) {
	o.d.emit(report.RemediationAttempt(task.ID, task.Number, attempt, cap))
}

// emitPrePublishTerminal emits the loop's terminal for one task — its rounds,
// remediations, last verdict/revision, coverage, artifact reference, and the
// escalation reason when the loop did not converge.
func (d *Dispatcher) emitPrePublishTerminal(res TaskResult) {
	out := res.PrePublish
	if out == nil {
		return
	}
	reason := ""
	if out.Err != nil {
		reason = out.Err.Error()
		// An escalation's error names only the brief's first line; the
		// journal keeps the whole brief.
		var escalation *worker.ReviewEscalationError
		if errors.As(out.Err, &escalation) && strings.TrimSpace(escalation.Brief) != "" {
			reason += "\n" + strings.TrimSpace(escalation.Brief)
		}
	} else if out.PublishErr != nil {
		reason = "artifact publication failed: " + out.PublishErr.Error()
	} else if out.CoverageErr != nil {
		reason = "coverage undecided: " + out.CoverageErr.Error()
	}
	d.emit(report.PrePublishReviewTerminal(report.PrePublishReviewTerminalFields{
		TaskID: res.Task.ID, Issue: res.Task.Number, Rounds: out.Rounds, Remediations: out.Remediations, Cap: out.Cap,
		Verdict: out.Verdict, Revision: out.Revision, ReviewID: out.ReviewID, Covered: out.Covered,
		ArtifactRef: out.PublishedRef, Reason: reason, Escalated: out.Err != nil, ScopeDrift: out.ScopeDrift,
	}))
}

// WithReviewHarness supplies the configured harness identity for a setup
// failure that occurs before an invoker exists.
func WithReviewHarness(harness entity.Harness) Option {
	return func(d *Dispatcher) { d.reviewHarness = harness }
}

// WithReviewHandoffRepairer installs crash recovery for obligation-persisted,
// implementation-terminal-incomplete green handoffs.
func WithReviewHandoffRepairer(repair ReviewHandoffRepairer) Option {
	return func(d *Dispatcher) {
		if repair != nil {
			d.reviewHandoffRepair = repair
		}
	}
}

// WithProposalProbe installs the pre-claim duplicate-dispatch guard: before any
// claim, the Dispatcher asks the probe which ready tasks already carry an open
// proposal and withholds the ones whose proposal is doing its job
// (withholdProposed). A nil probe is ignored — the base behavior, unchanged:
// every ready task proceeds to claim.
func WithProposalProbe(probe port.ProposalProbe) Option {
	return func(d *Dispatcher) {
		if probe != nil {
			d.probe = probe
		}
	}
}

// WithReporter routes the normalized executor event stream (§3.4) to an emitter —
// the two-sink Reporter the cmd surface builds. Without it the Dispatcher emits
// nothing (the base behavior; every event call is a no-op). A nil emitter is
// ignored.
func WithReporter(reporter report.Emitter) Option {
	return func(d *Dispatcher) {
		if reporter != nil {
			d.reporter = reporter
		}
	}
}

// WithAttribution names who invoked this sweep and over what scope, carried
// on the run_started event (self-drive-wake/001 T8, EVT-6): initiator
// distinguishes an attended operator run from a wake-driven one; scope names
// what is being swept (a repo slug, a plan reference, …). Without it a
// Dispatcher emits run_started with both fields empty, exactly as before this
// property existed.
func WithAttribution(initiator, scope string) Option {
	return func(d *Dispatcher) {
		d.initiator = initiator
		d.scope = scope
	}
}

// runtimeFor resolves a task's harness and runtime: through the per-task resolver
// when one is configured, else the single fallback runtime (Deps.Runtime) with
// the task's charter harness as its name.
func (d *Dispatcher) runtimeFor(task port.Task) (harness entity.Harness, rt port.AgentRuntime, err error) {
	if d.resolveRuntime != nil {
		return d.resolveRuntime(task)
	}
	return task.Harness, d.runtime, nil
}

// workerOptsFor builds the per-task Worker options: the uniform wall-clock budget
// (the mechanical turn bound for cap-less backends, §3.7) plus — when a green
// factory is configured — a GreenDriver bound to this task's runtime. Building the
// driver once per task (not per respawn attempt) is safe: a GreenDriver is
// stateless and spawns a fresh fixer process per Drive (DINV-11).
func (d *Dispatcher) workerOptsFor(_ port.Task, rt port.AgentRuntime) ([]worker.Option, error) {
	opts := []worker.Option{worker.WithBudget(d.budget)}
	if d.implementationAdmissionFactory != nil {
		admit, err := d.implementationAdmissionFactory(rt)
		if err != nil {
			return nil, fmt.Errorf("build implementation admission: %w", err)
		}
		if admit == nil {
			return nil, errors.New("build implementation admission: activated factory returned nil")
		}
		opts = append(opts, worker.WithImplementationAdmission(admit))
		if d.implementationOutcomeFactory == nil {
			return nil, errors.New("build implementation admission: activated outcome factory is absent")
		}
		complete, err := d.implementationOutcomeFactory(rt)
		if err != nil {
			return nil, fmt.Errorf("build implementation outcome: %w", err)
		}
		if complete == nil {
			return nil, errors.New("build implementation outcome: activated factory returned nil")
		}
		opts = append(opts, worker.WithImplementationOutcome(complete))
	}
	if d.greenFactory != nil {
		gd, err := d.greenFactory(rt)
		if err != nil {
			return nil, fmt.Errorf("build green driver: %w", err)
		}
		if gd != nil {
			opts = append(opts, worker.WithGreenDriver(gd))
		}
	}
	return opts, nil
}

// emit delivers one normalized event when a reporter is wired; otherwise it is a
// no-op, so a Dispatcher built without WithReporter stays event-free (the base
// behavior the existing tests assert).
func (d *Dispatcher) emit(e report.Event) {
	if d.reporter != nil {
		d.reporter.Emit(e)
	}
}

// emitVerified emits the task_verified event (EVT-4) derived from a Worker's
// outcome — the verify-by-artifact verdict. It fires only for a task that ran and
// actually reached verification: a claim-lost skip never verified, and an
// unconverged Worker exited before verify-by-artifact (its story is the escalated
// task_outcome), so neither emits a verdict.
func (d *Dispatcher) emitVerified(task port.Task, res TaskResult) {
	if !res.Ran {
		return
	}
	if res.Outcome == worker.OutcomeFailed && !res.VerificationAttempted {
		return // a pre-proposal failure never observed remote CI red
	}
	v := verdictOf(res.Outcome)
	if v == "" {
		return
	}
	d.emit(report.TaskVerified(task.ID, task.Number, v))
}

// emitOutcome emits the legacy task_outcome wire event: the terminal class, the
// fail-closed escalation flag (a stuck task that exhausted its respawn cap), and
// the advisory failure reason. The escalation flag is set from the Dispatcher's
// own ErrRespawnCapExhausted; a self-heal-cap failure reaches the sink as a failed
// class, which the argus sink already treats as a fail-closed escalation (§3.9).
func (d *Dispatcher) emitOutcome(task port.Task, res TaskResult) {
	reason := ""
	if res.Err != nil {
		reason = res.Err.Error()
	}
	escalated := errors.Is(res.Err, ErrRespawnCapExhausted)
	d.emit(report.TaskOutcome(task.ID, task.Number, res.Class.String(), escalated, reason))
}

func (d *Dispatcher) emitReview(r worker.ReviewResult, taskID string) {
	reason := ""
	if r.Err != nil {
		reason = r.Err.Error()
	}
	if r.Publication != nil {
		if p := r.ImplReviewTerminal; p != nil {
			d.emit(report.ReviewArtifactPublicationTerminal(report.ReviewArtifactPublicationTerminalFields{
				ProcessID: p.ReviewProcessID, ObligationID: p.ReviewObligationID,
				PublicationAttemptID: p.PublicationAttemptID, ProposalID: p.ProposalID,
				ReviewedRevision: p.ReviewedRevision, Medium: p.Medium, IdempotencyID: p.IdempotencyID,
				Status: string(p.Status), ArtifactRef: p.ArtifactRef, NativeReceipt: p.NativeReceipt,
				AttemptProofRef: p.AttemptProofRef, FailureClass: string(p.FailureClass), Reason: p.Reason,
			}))
		}
		p := r.Publication
		d.emit(report.HostedReviewPublicationTerminal(report.HostedReviewPublicationTerminalFields{
			AttemptID: p.ReviewAttemptID, ObligationID: p.ReviewObligationID,
			ProposalRef: p.ProposalRef, Revision: p.Revision, Status: p.Status,
			ArtifactRef: p.ArtifactRef, NativeReceipt: p.NativeReceipt,
			AttemptProofRef: p.AttemptProofRef, FailureClass: p.FailureClass,
			Reason: p.Reason, Escalated: p.Escalated,
		}))
	}
	d.emit(report.ImplReviewTerminal(report.ImplReviewTerminalFields{
		DriveRoundID: r.DriveRoundID, AttemptID: r.AttemptID, ObligationID: r.ObligationID,
		TaskID: taskID, ProposalRef: r.ProposalRef, Revision: r.Revision,
		Harness: r.Harness, ReviewerPrincipal: r.ReviewerPrincipal,
		NativeProfile: r.NativeProfileID, Catalog: r.CatalogID, Policy: r.PolicyIdentity,
		Model: r.Model, Effort: r.Effort, Phase: r.Phase, Status: r.Status,
		ObligationStatus: r.ObligationStatus, FailureClass: r.FailureClass,
		ArtifactRef: r.ArtifactRef, ArtifactProofRef: r.ArtifactProofRef,
		Reason: reason, Escalated: r.Failed(),
	}))
}

// verdictOf maps a Worker Outcome to its verify-by-artifact verdict for the
// task_verified event. OutcomeUnconverged returns "" — a bound-hit Worker exited
// before opening or verifying a proposal, so there is no verdict to report.
func verdictOf(o worker.Outcome) string {
	switch o {
	case worker.OutcomeSucceeded:
		return report.VerdictGreen
	case worker.OutcomeHollow:
		return report.VerdictHollow
	case worker.OutcomeFailed:
		return report.VerdictRed
	default:
		return ""
	}
}
