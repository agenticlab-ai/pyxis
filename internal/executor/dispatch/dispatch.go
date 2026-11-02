// Package dispatch is the pyxis executor's local in-process Dispatcher — the
// frontier-sweep run model over the portable single-task Worker
// (docs/tech-design/implement-now.md §3.6). It owns the two things the Worker
// deliberately does not: the frontier walk over the frozen plan's DAG and the
// bounded concurrency that runs many Workers at once. The Worker stays
// concurrency-agnostic and runtime-agnostic, so a future remote/Docker fleet is a
// second Dispatcher over the same Worker core, not a rewrite (DINV-10).
//
// A run is one sweep, not a daemon: compute the ready frontier, resolve each
// ready task's proposal status before claim, claim up to a width bound
// serially, run the claimed Workers concurrently, verify each by artifact,
// and report a classed RunReport, then exit.
// Dependents blocked on an unmerged blocker are reported, not waited on — the
// driver lands blockers and re-invokes, and the next sweep sees them
// unblocked (§3.6 step 5). An empty frontier spawns nothing and makes zero
// model calls (DINV-8, AP-19).
package dispatch

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/entity"
	"github.com/agenticlab-ai/pyxis/internal/executor/port"
	"github.com/agenticlab-ai/pyxis/internal/executor/report"
	"github.com/agenticlab-ai/pyxis/internal/executor/worker"
)

// DefaultWidth is the parallel-dispatch width used when Config.Width is not set.
// It is a sensible default the operator overrides per repo on the config surface
// (§3.8, open question Q2); the cap is enforced mechanically here, never
// model-judged (AP-19).
const DefaultWidth = 4

// DefaultReviewWidth is independent from implementation width: review latency
// can never retain or consume a Worker slot.
const DefaultReviewWidth = 2

// DefaultRespawnCap is the respawn cap used when Config.RespawnCap is not set — a
// sensible, conservative default the operator overrides per repo on the config
// surface (§3.8, Q2). A non-converging Worker (one that hit its mechanical bound)
// resumes from its pushed WIP branch, so one respawn grants a genuinely-slow task
// a second bounded window to make cumulative progress; the cap keeps that
// bounded, then the task escalates (DINV-9, AP-16).
const DefaultRespawnCap = 1

// ErrRespawnCapExhausted is the escalation signal a task carries when it hit its
// mechanical turn/wall-clock bound (worker.OutcomeUnconverged) and was respawned
// up to the configured cap without converging. The task is classed ClassFailed
// and this error is wrapped into its TaskResult.Err, so the reporting surface can
// escalate it (attended: surfaced in-session; unattended: argus's delivery
// journal + fail-closed escalation — §3.4, §3.9). Callers match it with
// errors.Is (DINV-9, AP-16, prd:implement-now/FR-13).
var ErrRespawnCapExhausted = errors.New("dispatch: worker respawn cap exhausted without convergence")

// Deps are the five consumer-side ports the Dispatcher needs: the WorkTracker it
// reads the frontier from, and the four ports it hands each Worker it builds
// (Lease, Workspace, AgentRuntime, CodeHost). They are the exact ports a Worker
// composes (§3.1); the Dispatcher adds only the frontier walk and concurrency.
type Deps struct {
	Tracker port.WorkTracker
	Lease   port.Lease
	Space   port.Workspace
	Runtime port.AgentRuntime
	Host    port.CodeHost
}

// Config is the Dispatcher's closed-schema policy. Today it carries the
// parallel-dispatch width cap; routing and the other caps arrive on the same
// surface in a later task (§3.8). A zero or negative Width falls back to
// DefaultWidth so a mis-set cap fails safe, never to unbounded concurrency.
type Config struct {
	// Width bounds how many Workers run concurrently in a sweep (the
	// parallel-dispatch cap). Claims are always taken serially regardless of
	// Width, so two Workers never take one task.
	Width       int
	ReviewWidth int

	// RespawnCap bounds how many times a non-converging Worker (one that hit its
	// mechanical turn/wall-clock bound, worker.OutcomeUnconverged) is respawned
	// within a sweep before the task escalates — classed failed with
	// ErrRespawnCapExhausted. It bounds per-task retry effort (DINV-9, AP-16). A
	// value below 1 falls back to DefaultRespawnCap so a mis-set cap fails safe,
	// never to unbounded respawning.
	RespawnCap int

	// WorkerBudget is the uniform wall-clock budget each Worker enforces around the
	// agent run (worker.WithBudget): the mechanical turn bound for backends without
	// a native cap, uniform across all backends (§3.7, DINV-9). A non-positive
	// value means no wall-clock bound (a backend's native cap, where present, still
	// applies).
	WorkerBudget time.Duration

	// PrePublishReview runs the configured review as the local-first loop
	// (worker/prepublish.go): review the exact local revision after the local
	// gate, remediate on request-changes up to RemediationRounds
	// (worker.DefaultRemediationRounds when non-positive), and publish only an
	// accepted revision. It applies only when a review factory is installed
	// whose service implements worker.PrePublishReviewer; false keeps review
	// post-green only, exactly as before.
	PrePublishReview  bool
	RemediationRounds int
}

// Dispatcher walks the frontier and allocates Workers to a width bound. It holds
// the ports and the width; it carries no per-run state, so one Dispatcher serves
// repeated sweeps (the driver re-invokes Sweep across waves).
type Dispatcher struct {
	tracker     port.WorkTracker
	lease       port.Lease
	runtime     port.AgentRuntime // the single fallback runtime; unused when a resolver is set
	space       port.Workspace
	host        port.CodeHost
	width       int
	reviewWidth int

	// respawnCap bounds respawns of a non-converging Worker before a task escalates
	// (DINV-9, AP-16). budget is the uniform wall-clock bound every Worker enforces
	// around its agent run — the mechanical turn bound for cap-less backends,
	// uniform across all backends (§3.7, worker.WithBudget).
	respawnCap int
	budget     time.Duration

	// The T13 integration seams (all optional; nil preserves the base behavior —
	// see wiring.go). resolveRuntime routes each task to its backend (§3.8);
	// greenFactory attaches drive-to-green per task (§3.6); reporter is the
	// normalized two-sink event stream (§3.4, §3.9).
	resolveRuntime                 RuntimeResolver
	greenFactory                   GreenDriverFactory
	reviewFactory                  ReviewInvokerFactory
	reviewHarness                  entity.Harness
	prePublishReview               bool
	remediationRounds              int
	headReader                     worker.ProposalHeadReader
	reviewPublisher                worker.ReviewPublisher
	sessions                       worker.SessionContinuity
	feedback                       *feedbackDetection
	reviewHandoffRepair            ReviewHandoffRepairer
	implementationAdmissionFactory ImplementationAdmissionFactory
	implementationOutcomeFactory   ImplementationOutcomeFactory
	describedCheckpoint            worker.DescribedCheckpointObserver
	reporter                       report.Emitter

	// initiator and scope attribute the sweep-start event (self-drive-wake/001
	// T8, EVT-6): who invoked this sweep ("operator", "wake", …) and over what
	// scope (a repo slug, a plan reference, …). Both default to "" — a
	// Dispatcher built without WithAttribution emits run_started exactly as
	// before this property existed.
	initiator string
	scope     string

	// probe is the pre-claim duplicate-dispatch guard (withholdProposed):
	// optional, nil preserves the base behavior exactly (every ready task
	// proceeds to claim). When wired, a ready task that already carries an
	// open, non-red proposal never reaches a claim, a worktree, or a model.
	probe port.ProposalProbe
}

// New builds a Dispatcher from its ports, config, and any options. Every port is
// required and a nil port is a wiring error, reported (never panicked) so the
// caller fails closed — except the single AgentRuntime, which is required only
// when no per-task runtime resolver is configured (WithRuntimeResolver): the cmd
// surface wires a resolver and leaves Deps.Runtime nil, while the in-process
// harness wires one runtime and no resolver. Width and the respawn cap are
// clamped to their safe defaults. Options are additive — New(deps, cfg) with no
// option builds the base Dispatcher exactly as before.
func New(deps Deps, cfg Config, opts ...Option) (*Dispatcher, error) {
	width := cfg.Width
	if width < 1 {
		width = DefaultWidth
	}
	respawnCap := cfg.RespawnCap
	if respawnCap < 1 {
		respawnCap = DefaultRespawnCap
	}
	reviewWidth := cfg.ReviewWidth
	if reviewWidth < 1 {
		reviewWidth = DefaultReviewWidth
	}
	d := &Dispatcher{
		tracker:           deps.Tracker,
		lease:             deps.Lease,
		runtime:           deps.Runtime,
		space:             deps.Space,
		host:              deps.Host,
		width:             width,
		reviewWidth:       reviewWidth,
		respawnCap:        respawnCap,
		budget:            cfg.WorkerBudget,
		prePublishReview:  cfg.PrePublishReview,
		remediationRounds: cfg.RemediationRounds,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(d)
		}
	}
	switch {
	case deps.Tracker == nil:
		return nil, errors.New("dispatch: nil WorkTracker")
	case deps.Lease == nil:
		return nil, errors.New("dispatch: nil Lease")
	case deps.Space == nil:
		return nil, errors.New("dispatch: nil Workspace")
	case deps.Host == nil:
		return nil, errors.New("dispatch: nil CodeHost")
	}
	// The single runtime is required only absent a resolver; with a resolver each
	// task's runtime is resolved from its charter, so Deps.Runtime may be nil.
	if d.resolveRuntime == nil && deps.Runtime == nil {
		return nil, errors.New("dispatch: nil AgentRuntime (set Deps.Runtime or configure a runtime resolver)")
	}
	return d, nil
}

// Sweep runs one frontier-sweep and returns its classed RunReport.
//
// The steps mirror §3.6, with one pre-claim stage from
// implement-now-e2e-tasks §3.1 inserted between readiness and claim: read the
// candidate frontier from the tracker; compute the ready set through one
// predicate — unblocked (no open blocker) ∧ unclaimed; when an origin resolver
// is configured, resolve each ready task's e2e origin/slot before any claim,
// diverting a refused or manual-human-routed task to its own terminal class
// with no lease, worktree, or model call spent (§3.2, §3.7, DINV-2); claim the
// remaining ready set up to the width bound serially so two Workers never take
// one task; run the claimed Workers concurrently to that bound, each a fresh
// worker.New(...).Run over the same ports; verify each by artifact (the
// Worker's job); classify. A candidate still gated behind an open blocker is
// reported blocked, not run (§3.6 step 5). An empty ready frontier spawns no
// Worker and makes zero model calls (DINV-8): Sweep returns before constructing
// any Worker.
//
// Sweep returns a non-nil error only for a sweep-level operational failure — it
// could not read the frontier. One task's failure is classed in the report and
// never errors the sweep (AP-16).
func (d *Dispatcher) Sweep(ctx context.Context) (RunReport, error) {
	// run_started brackets the whole sweep, so even a frontier-read failure has a
	// matching run_finished on the event stream (§3.4, EVT-1/EVT-6).
	d.emit(report.RunStarted(d.initiator, d.scope))
	var pairs []sweepItem
	if repairer, ok := d.reviewHandoffRepair.(ReviewPairLifecycleRepairer); ok {
		repaired, pairErr := repairer.RepairPairs(ctx)
		if pairErr != nil {
			result := worker.ReviewResult{Harness: d.reviewHarness, Phase: worker.ReviewPhaseObligation, Status: report.ReviewStatusFailed, ObligationStatus: report.ReviewObligationNotCreated, FailureClass: worker.ReviewFailureInfrastructure, Err: pairErr}
			item := sweepItem{Origin: originPair, Review: reviewState{Phase: reviewPairPending}}
			item.setReviewResult(result)
			d.emitReview(result, "")
			return d.finish(nil, getReviewResults([]sweepItem{item})), nil
		}
		pairs = make([]sweepItem, 0, len(repaired))
		for _, repairedPair := range repaired {
			result := repairedPair.Result
			item := sweepItem{Origin: originPair, Task: port.Task{ID: repairedPair.TaskID}, Review: reviewState{Phase: reviewPairPending}}
			item.setReviewResult(result)
			pairs = append(pairs, item)
			d.emitReview(result, item.Task.ID)
		}
		// A corrupt or temporarily unreadable pair remains a red, pending review
		// result, but it must not monopolize the repository-wide sweep. Continue to
		// the implementation frontier so unrelated ready work can still settle;
		// the failed pair stays selected for a later repair invocation.
	}
	recovered, handoffFailures, repairErr := d.repairReviewHandoffs(ctx)
	if repairErr != nil {
		result := worker.ReviewResult{Harness: d.reviewHarness, Phase: worker.ReviewPhaseObligation, Status: report.ReviewStatusFailed, ObligationStatus: report.ReviewObligationNotCreated, FailureClass: worker.ReviewFailureInfrastructure, Err: repairErr}
		item := sweepItem{Origin: originHandoff, Review: reviewState{Phase: reviewHandoffPending}}
		item.setReviewResult(result)
		handoffFailures = append(handoffFailures, item)
		d.emitReview(result, "")
		return d.finish(nil, getReviewResults(pairs, handoffFailures)), nil
	}
	candidates, err := d.tracker.Frontier(ctx)
	if err != nil {
		d.emit(report.RunFinished(report.Counts{}))
		return RunReport{}, fmt.Errorf("dispatch: read frontier: %w", err)
	}

	// The one frontier-eligibility predicate (DINV-4): a candidate is ready when
	// no issue in its BlockedBy set is still open. Openness is read from the
	// candidate set itself — the tracker returns exactly the open tagged items,
	// so a blocker absent from that set has merged/closed and no longer gates.
	openIssues := make(map[int]bool, len(candidates))
	for _, c := range candidates {
		openIssues[c.Number] = true
	}

	// Partition candidates into ready and blocked, preserving tracker order so
	// the report is deterministic. ready holds pointers into frontier, so later
	// stages write the same item a Worker runs.
	frontier := make([]sweepItem, len(candidates))
	var ready []*sweepItem
	for i, c := range candidates {
		frontier[i] = sweepItem{Task: c, Origin: originFrontier, Impl: implState{Phase: implCandidate}}
		if unblocked(c, openIssues) {
			frontier[i].Impl.Phase = implReady
			ready = append(ready, &frontier[i])
			continue
		}
		frontier[i].applyTaskResult(TaskResult{Task: c, Class: ClassBlocked})
		// A blocked task is a terminal class the driver reads to know what to
		// land next (§3.6 step 5); it is dispatched to no Worker, so it emits a
		// task_outcome but no task_dispatched/task_verified.
		d.emitOutcome(frontier[i].Task, frontier[i].taskResult())
	}

	// DINV-8: an empty ready frontier consumes zero model tokens. No Worker is
	// built, so the runtime is never launched; only blocked candidates (if any)
	// populate the report.
	if len(ready) == 0 && len(recovered) == 0 {
		return d.finish(getTaskResults(frontier), getReviewResults(pairs, handoffFailures)), nil
	}

	// The pre-claim proposal preflight: a ready task that is already proposed
	// is awaited, not restarted. Runs after the origin decision and before any
	// claim, so an already-proposed task costs no lease, worktree, or model
	// call (DINV-2, DINV-8).
	ready = d.withholdProposed(ctx, ready)
	if len(ready) == 0 && len(recovered) == 0 {
		return d.finish(getTaskResults(frontier), getReviewResults(pairs, handoffFailures)), nil
	}

	// A recovered handoff needs a review service. Without a factory the sweep
	// already knows it cannot invoke, so the failure is recorded here and the
	// item is not handed to a review goroutine. Frontier implementation still runs.
	var toReview []sweepItem
	if d.reviewFactory == nil {
		for i := range recovered {
			item := &recovered[i]
			result := worker.ReviewResult{ObligationID: item.Review.ObligationID, Harness: item.Review.Harness, ProposalRef: item.Review.ProposalRef, Revision: item.Review.Revision, Phase: worker.ReviewPhasePreflight, Status: report.ReviewStatusFailed, ObligationStatus: report.ReviewObligationPending, FailureClass: worker.ReviewFailureInfrastructure, Err: errors.New("recovered review has no preflighted service")}
			item.setReviewResult(result)
			d.emitReview(result, item.Task.ID)
		}
	} else {
		reviewServices := map[entity.Harness]worker.ReviewService{}
		for _, item := range ready {
			backend := d.reviewHarness
			if item.Task.NativeRouting != nil {
				backend = item.Task.NativeRouting.Review.Harness
			}
			if backend == "" {
				var routeErr error
				backend, _, routeErr = d.runtimeFor(item.Task)
				if routeErr != nil {
					result := worker.ReviewResult{Harness: "unresolved", Phase: worker.ReviewPhasePreflight, Status: report.ReviewStatusFailed, ObligationStatus: report.ReviewObligationNotCreated, FailureClass: worker.ReviewFailureInfrastructure, Err: fmt.Errorf("resolve inherited review backend for task %s: %w", item.Task.ID, routeErr)}
					d.emitReview(result, "")
					rep := RunReport{ReviewFailed: 1, ReviewInfrastructureFailed: 1}
					d.emit(report.RunFinished(report.Counts{ReviewFailed: 1, ReviewInfrastructureFailed: 1}))
					return rep, nil
				}
			}
			reviewer, setupErr := d.reviewerFor(ctx, backend, reviewServices)
			if setupErr != nil {
				result := worker.ReviewResult{Harness: backend, Phase: worker.ReviewPhasePreflight, Status: report.ReviewStatusFailed, ObligationStatus: report.ReviewObligationNotCreated, FailureClass: worker.ReviewFailureInfrastructure, Err: fmt.Errorf("review preflight: %w", setupErr)}
				d.emitReview(result, "")
				rep := RunReport{ReviewFailed: 1, ReviewInfrastructureFailed: 1}
				d.emit(report.RunFinished(report.Counts{ReviewFailed: 1, ReviewInfrastructureFailed: 1}))
				return rep, nil
			}
			item.Reviewer = reviewer
		}
		for i := range recovered {
			item := &recovered[i]
			reviewer, setupErr := d.reviewerFor(ctx, item.Review.Harness, reviewServices)
			if setupErr != nil {
				wrapped := fmt.Errorf("review preflight: %w", setupErr)
				for j := range recovered {
					pending := &recovered[j]
					result := worker.ReviewResult{ObligationID: pending.Review.ObligationID, Harness: pending.Review.Harness, ProposalRef: pending.Review.ProposalRef, Revision: pending.Review.Revision, Phase: worker.ReviewPhasePreflight, Status: report.ReviewStatusFailed, ObligationStatus: report.ReviewObligationPending, FailureClass: worker.ReviewFailureInfrastructure, Err: wrapped}
					pending.setReviewResult(result)
					d.emitReview(result, pending.Task.ID)
				}
				return d.finish(nil, getReviewResults(pairs, handoffFailures, recovered)), nil
			}
			item.Reviewer = reviewer
		}
		toReview = recovered
	}

	d.runReadyTasks(ctx, ready)
	d.runRecoveredReviews(ctx, toReview)
	return d.finish(getTaskResults(frontier), getReviewResults(pairs, handoffFailures, recovered)), nil
}

// finish finalizes the report and emits run_finished with its headline counts —
// the single place a sweep's terminal event is produced (EVT-16, §3.4).
func (d *Dispatcher) finish(results []TaskResult, reviews []worker.ReviewResult) RunReport {
	rep := finalize(results, reviews...)
	// The routing counters intentionally remain zero until the semantic
	// routing cutover supplies canonical EVT-7/EVT-8 evidence. The current
	// plan/v1 compatibility path cannot truthfully infer those classes from a
	// selected model name alone.
	d.emit(report.RunFinished(report.Counts{
		Succeeded:                  rep.Succeeded,
		Failed:                     rep.Failed,
		Blocked:                    rep.Blocked,
		Tokens:                     rep.Tokens,
		Skipped:                    rep.Skipped,
		AwaitingProposal:           rep.AwaitingProposal,
		ProbeUnavailable:           rep.ProbeUnavailable,
		PrePublishReviewed:         rep.PrePublishReviewed,
		FeedbackRounds:             rep.FeedbackRounds,
		ReviewCompleted:            rep.ReviewCompleted,
		ReviewFailed:               rep.ReviewFailed,
		ReviewPending:              rep.ReviewPending,
		ReviewArtifactUnverified:   rep.ReviewArtifactUnverified,
		ReviewArtifactUntrusted:    rep.ReviewArtifactUntrusted,
		ReviewInfrastructureFailed: rep.ReviewInfrastructureFailed,
	}))
	return rep
}

// unblocked is the readiness half of the frontier predicate: a candidate is
// unblocked when none of the issues it is BlockedBy is still open (present in the
// open candidate set). A task with no blockers is always unblocked.
func unblocked(task port.Task, openIssues map[int]bool) bool {
	for _, blocker := range task.BlockedBy {
		if openIssues[blocker] {
			return false
		}
	}
	return true
}

// reviewerFor returns the preflighted review service for one backend, building
// it once per sweep and sharing it across items routed to that backend.
func (d *Dispatcher) reviewerFor(ctx context.Context, harness entity.Harness, cache map[entity.Harness]worker.ReviewService) (worker.ReviewService, error) {
	if reviewer, ok := cache[harness]; ok {
		return reviewer, nil
	}
	reviewer, err := d.reviewFactory(harness)
	if err == nil && reviewer == nil {
		err = errors.New("review factory returned a nil invoker")
	}
	if err == nil {
		err = reviewer.Preflight(ctx)
	}
	if err != nil {
		return nil, err
	}
	cache[harness] = reviewer
	return reviewer, nil
}

func (d *Dispatcher) repairReviewHandoffs(ctx context.Context) (ready, failures []sweepItem, err error) {
	if d.reviewHandoffRepair == nil {
		return nil, nil, nil
	}
	rows, err := d.reviewHandoffRepair.Pending(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("exact-read review handoff frontier: %w", err)
	}
	ready = make([]sweepItem, 0, len(rows))
	for _, row := range rows {
		if row.ObligationID == "" || row.Task.ID == "" || row.Task.Number <= 0 || row.Harness == "" || row.ProposalRef == "" || row.Revision == "" {
			return nil, failures, errors.New("dispatch: malformed persisted review handoff")
		}
		item := sweepItem{
			Origin: originHandoff,
			Task:   row.Task,
			Impl:   implState{Phase: implSucceeded, Class: ClassSucceeded, Outcome: worker.OutcomeSucceeded, Ran: true},
			Review: reviewState{Phase: reviewHandoffPending, ObligationID: row.ObligationID, Harness: row.Harness, ProposalRef: row.ProposalRef, Revision: row.Revision},
		}
		// Reconstruct the existing green implementation terminal before making
		// review eligible. Both emissions precede the canonical marker append.
		d.emit(report.TaskVerified(item.Task.ID, item.Task.Number, report.VerdictGreen))
		d.emitOutcome(item.Task, item.taskResult())
		if err := d.reviewHandoffRepair.Complete(ctx, row.ObligationID); err != nil {
			result := worker.ReviewResult{ObligationID: row.ObligationID, Harness: row.Harness, ProposalRef: row.ProposalRef, Revision: row.Revision, Phase: worker.ReviewPhaseObligation, Status: report.ReviewStatusFailed, ObligationStatus: report.ReviewObligationPending, FailureClass: worker.ReviewFailureInfrastructure, Err: fmt.Errorf("complete recovered review handoff: %w", err)}
			item.setReviewResult(result)
			failures = append(failures, item)
			d.emitReview(result, item.Task.ID)
			continue
		}
		item.Review.Phase = reviewHandoffReady
		ready = append(ready, item)
	}
	return ready, failures, nil
}

// withholdProposed removes from ready every task that already carries an open,
// non-red proposal, classing it ClassAwaitingProposal before any claim.
//
// This is the executor's own half of the duplicate-dispatch guard. The frontier
// is "open ∧ tagged ∧ unblocked" tracker items, and an item stays open from the
// moment its proposal opens until that proposal merges — a window in which the
// Worker has already released its claim. Any sweep invoked in that window would
// otherwise start a second worker on work that is already proposed, and because
// the re-open is idempotent on the task's deterministic head branch, that second
// worker does not open a second proposal: it pushes fresh commits onto the one
// under review. The guard holds no matter which scheduler invoked the sweep.
//
// EVERY open proposal is withheld, whatever its checks say. Check colour is not
// recovery evidence: a red proposal is just as likely a Worker that exhausted
// its bounded self-heal, recorded the failure and released its claim, and
// restarting that one resets the per-sweep bounds and pushes more commits onto
// an already-failed proposal — the unbounded cross-sweep retry DINV-9 exists to
// prevent. Resuming work on an open proposal needs an explicit, durable
// recovery authorization that reads attempt, outcome, claim, branch and
// proposal together; it is never inferred from check colour here (AP-17).
//
// A probe error is NOT this class: it is ClassProbeUnavailable, a failure that
// reddens the run (see the class comment). The sweep still withholds — a
// dispatch act requires the sweep's own positive evidence (AP-17) — but it never
// reports that as healthy awaiting.
//
// With no probe wired the ready set is returned unchanged, so a Dispatcher built
// without WithProposalProbe behaves exactly as before this stage existed.
func (d *Dispatcher) withholdProposed(ctx context.Context, ready []*sweepItem) []*sweepItem {
	if d.probe == nil || len(ready) == 0 {
		return ready
	}
	tasks := make([]port.Task, 0, len(ready))
	for _, item := range ready {
		// Fill the head branch exactly as runOne does, so the probe asks about
		// the very branch a Worker would open this task's proposal from. The
		// copy is local; the ready set is untouched.
		t := item.Task
		if strings.TrimSpace(t.HeadBranch) == "" {
			t.HeadBranch = headBranchFor(t)
		}
		tasks = append(tasks, t)
	}
	open, probeErr := d.probe.OpenProposals(ctx, tasks)
	if probeErr != nil {
		probeErr = fmt.Errorf("dispatch: probe open proposals: %w", probeErr)
	}
	remaining := make([]*sweepItem, 0, len(ready))
	for _, item := range ready {
		if probeErr != nil {
			res := TaskResult{Task: item.Task, Class: ClassProbeUnavailable, Err: probeErr}
			item.applyTaskResult(res)
			d.emitOutcome(item.Task, item.taskResult())
			continue
		}
		proposal, proposed := open[item.Task.ID]
		if !proposed {
			remaining = append(remaining, item)
			continue
		}
		// The one exception to withholding an open proposal: a HUMAN reviewed
		// it since the task's session last answered. That is a feedback round
		// (feedback.go) — durable evidence of new human activity (AP-17), the
		// cursor advanced so it runs once, the task's own session resumed.
		task := item.Task
		if strings.TrimSpace(task.HeadBranch) == "" {
			task.HeadBranch = headBranchFor(task)
		}
		admitted, due, err := d.admitFeedback(ctx, task, proposal.Number)
		if due {
			item.Task = admitted
			item.Impl.Phase = implFeedbackAdmitted
			remaining = append(remaining, item)
			continue
		}
		res := TaskResult{Task: item.Task, Class: ClassAwaitingProposal}
		if err != nil {
			res.Err = fmt.Errorf("%w: %v", errFeedbackUndecided, err)
		}
		item.applyTaskResult(res)
		d.emitOutcome(item.Task, item.taskResult())
	}
	return remaining
}

// runRecoveredReviews invokes the saved reviewer for handoffs whose
// implementation already succeeded. It uses its own review-width pool and
// starts only after runReady returns, so a recovered review does not share
// capacity with a frontier review or occupy an implementation slot.
func (d *Dispatcher) runRecoveredReviews(ctx context.Context, items []sweepItem) {
	reviewSem := make(chan struct{}, d.reviewWidth)
	var wg sync.WaitGroup
	for i := range items {
		wg.Add(1)
		go func(item *sweepItem) {
			defer wg.Done()
			d.reviewHandoffItem(ctx, reviewSem, item)
		}(&items[i])
	}
	wg.Wait()
}

// runReadyTasks claims and runs the ready frontier. Claims are serialized through a
// per-sweep lease guard (so two Workers never take one task), while the Workers
// themselves run concurrently to the width bound. Each item is written only by
// its own goroutine; the caller reads them after the pool drains.
func (d *Dispatcher) runReadyTasks(ctx context.Context, items []*sweepItem) {
	// Serialize every Worker's claim step behind one guard for this sweep. The
	// Worker owns acquire→release (DINV-5); the Dispatcher only makes the acquire
	// mutually exclusive, keeping the Worker concurrency-agnostic (DINV-10).
	claim := &serialLease{inner: d.lease}

	// A buffered channel is the width semaphore: at most width Workers hold a
	// slot, so at most width run at once (§3.6 step 2, the mechanical cap).
	sem := make(chan struct{}, d.width)
	var wg sync.WaitGroup
	reviewSem := make(chan struct{}, d.reviewWidth)

	for _, item := range items {
		// Honor cancellation promptly: stop launching new Workers once the
		// context is done. Slots are freed by in-flight Workers as they observe
		// cancellation through the same context.
		if ctx.Err() != nil {
			item.applyTaskResult(TaskResult{Task: item.Task, Class: ClassFailed, Ran: false, Err: ctx.Err()})
			continue
		}
		sem <- struct{}{} // blocks when width Workers are already in flight
		wg.Add(1)
		go func(item *sweepItem) {
			defer wg.Done()
			res := d.runOne(ctx, claim, item)
			<-sem // implementation width is released before review admission

			if res.PrePublish != nil {
				d.emitPrePublishTerminal(res)
			}
			if item.Reviewer == nil || res.Class != ClassSucceeded || (res.PrePublish != nil && res.PrePublish.Covered) {
				// No post-green review is owed: none configured, nothing green,
				// or the pre-publish review already covers the exact published
				// head (worker/prepublish.go).
				item.Review.Phase = reviewNotOwed
				d.emitOutcome(res.Task, res)
				return
			}
			if res.ReviewCapture == nil || res.ReviewCapture.Failed() {
				// Implementation truth is already green and remains independently
				// terminal even when the configured-review handoff cannot be created
				// or completed. The review failure is emitted separately below.
				if res.ReviewCapture == nil || !res.ReviewCapture.Persisted {
					d.emitOutcome(res.Task, res)
				}
				err := errors.New("durable review obligation was not created")
				if res.ReviewCapture != nil && res.ReviewCapture.Err != nil {
					err = res.ReviewCapture.Err
				}
				obligationStatus := report.ReviewObligationNotCreated
				if res.ReviewCapture != nil && res.ReviewCapture.Persisted {
					obligationStatus = report.ReviewObligationPending
				}
				reviewResult := worker.ReviewResult{Harness: d.reviewHarness, Phase: worker.ReviewPhaseObligation, Status: report.ReviewStatusFailed, ObligationStatus: obligationStatus, FailureClass: worker.ReviewFailureInfrastructure, Err: err}
				if res.ReviewCapture != nil {
					reviewResult.Harness = res.ReviewCapture.Harness
					reviewResult.ProposalRef, reviewResult.Revision, reviewResult.ObligationID = res.ReviewCapture.ProposalRef, res.ReviewCapture.Revision, res.ReviewCapture.ObligationID
				}
				item.setReviewResult(reviewResult)
				d.emitReview(reviewResult, res.Task.ID)
				return
			}
			d.invokeReview(ctx, reviewSem, item, res.Task, res.Proposal)
		}(item)
	}
	wg.Wait()
}

// reviewHandoffItem loads the saved obligation and invokes its reviewer. Sweep
// only passes a handoff whose reviewer is already bound.
func (d *Dispatcher) reviewHandoffItem(ctx context.Context, reviewSem chan struct{}, item *sweepItem) {
	reviewer := item.Reviewer
	task, proposal, err := reviewer.PreparePending(ctx, item.Review.ObligationID)
	if err != nil {
		result := worker.ReviewResult{ObligationID: item.Review.ObligationID, Harness: item.Review.Harness, ProposalRef: item.Review.ProposalRef, Revision: item.Review.Revision, Phase: worker.ReviewPhaseObligation, Status: report.ReviewStatusFailed, ObligationStatus: report.ReviewObligationPending, FailureClass: worker.ReviewFailureInfrastructure, Err: err}
		item.setReviewResult(result)
		d.emitReview(result, item.Task.ID)
		return
	}
	d.invokeReview(ctx, reviewSem, item, task, proposal)
}

// invokeReview holds one review-width slot across Invoke, then records the
// result on the item. The caller supplies the task and proposal: the worker's
// for a frontier item, the saved obligation's for a handoff item.
func (d *Dispatcher) invokeReview(ctx context.Context, reviewSem chan struct{}, item *sweepItem, task port.Task, proposal port.Proposal) {
	reviewSem <- struct{}{}
	item.Review.Phase = reviewInvoked
	result := item.Reviewer.Invoke(ctx, task, proposal)
	<-reviewSem
	item.setReviewResult(result)
	d.emitReview(result, item.Task.ID)
}

// runOne resolves the task's backend runtime (§3.8), builds its Worker options
// (the uniform budget plus, when configured, a drive-to-green driver bound to that
// runtime), then runs the task and returns its classed result — emitting the
// per-task lifecycle events around it (task_dispatched → task_verified →
// task_outcome, §3.4). A runtime-resolution or option-build failure is classed
// failed rather than panicked, so one mis-routed task never crashes the sweep; it
// emits a task_outcome but no task_dispatched, since no Worker ever ran.
func (d *Dispatcher) runOne(ctx context.Context, claim port.Lease, item *sweepItem) TaskResult {
	task := item.Task
	reviewer := item.Reviewer
	// Derive the task's head branch when the tracker left it empty (neither
	// adapter sets it). This is medium-independent — every tracker benefits — and
	// deterministic, so a respawned Worker re-derives the same branch and its
	// re-open stays idempotent (AP-22). Without it, workspace.Prepare fails closed
	// ("no head branch") before any Worker can run (#396, the A-12 dogfood gap).
	// task is a by-value copy, so this mutation is local to this dispatch and
	// flows down into the Worker unchanged.
	if strings.TrimSpace(task.HeadBranch) == "" {
		task.HeadBranch = headBranchFor(task)
	}

	harness, rt, err := d.runtimeFor(task)
	if err != nil {
		res := TaskResult{Task: task, Class: ClassFailed, Ran: false, Err: fmt.Errorf("dispatch: resolve runtime for task %s: %w", task.ID, err)}
		item.applyTaskResult(res)
		return res
	}
	// Carry the already-selected harness into the Worker's immutable task copy.
	// A configured routing-evidence admission binds this value at the claim
	// boundary; runtime selection remains owned by runtimeFor.
	task.Harness = harness
	item.Task = task
	item.Runtime = rt
	opts, err := d.workerOptsFor(task, rt)
	if err != nil {
		res := TaskResult{Task: task, Class: ClassFailed, Ran: false, Err: fmt.Errorf("dispatch: prepare worker for task %s: %w", task.ID, err)}
		item.applyTaskResult(res)
		return res
	}
	opts = append(opts, worker.WithLocalGateObserver(func(observed port.Task, green bool, attempts, cap int, reason string) {
		d.emit(report.LocalGateTerminal(observed.ID, green, attempts, cap, reason))
	}))
	if d.describedCheckpoint != nil {
		opts = append(opts, worker.WithDescribedCheckpointObserver(d.describedCheckpoint))
	}
	if reviewer != nil {
		opts = append(opts, worker.WithReviewCapturer(reviewer), worker.WithGreenCapturedObserver(func(observed port.Task, _ port.Proposal, capture worker.ReviewCapture) error {
			d.emit(report.TaskVerified(observed.ID, observed.Number, report.VerdictGreen))
			d.emitOutcome(observed, TaskResult{Task: observed, Class: ClassSucceeded, Outcome: worker.OutcomeSucceeded, Ran: true})
			return reviewer.CompleteHandoff(ctx, capture)
		}))
		// The local-first loop: when enabled and the review service can review
		// a local revision, the Worker reviews before it pushes and the
		// post-green capture runs only for a head the loop does not cover.
		if pp, ok := reviewer.(worker.PrePublishReviewer); ok && d.prePublishReview {
			opts = append(opts, worker.WithPrePublishReview(pp, d.remediationRounds),
				worker.WithProposalHeadReader(d.headReader), worker.WithReviewPublisher(d.reviewPublisher),
				worker.WithPrePublishObserver(prePublishObserver{d: d}))
		}
	}
	// Task session continuity: every implementation-role launch of this task
	// continues its own harness conversation (feedback.go, worker/session.go).
	if d.sessions != nil {
		opts = append(opts, worker.WithSessionContinuity(d.sessions), worker.WithSessionObserver(d.sessionObserver()))
	}

	// task_dispatched (EVT-2): the task is handed to a Worker on its resolved
	// backend. A locally routed runtime also exposes the concrete model decision
	// for ordinary reporting; it is telemetry, never launch authority.
	dispatched := report.TaskDispatched(task.ID, task.Number, harness)
	if routed, ok := rt.(port.ModelRouteProvider); ok {
		route := routed.ModelRoute()
		dispatched.Model = route.Model
		dispatched.Effort = route.Effort
	}
	d.emit(dispatched)
	res := d.runAttempts(ctx, claim, item, opts)
	item.applyTaskResult(res)
	if reviewer == nil || res.Class != ClassSucceeded || res.ReviewCapture == nil || (res.ReviewCapture.Failed() && !res.ReviewCapture.Persisted) {
		d.emitVerified(task, res)
	}
	return res
}

// runAttempts builds a fresh Worker over the sweep's ports (a fresh process per
// task is the runtime adapter's job, DINV-11; a fresh Worker struct keeps the
// cycle self-contained) and runs the one task, respawning a non-converging Worker
// up to the cap.
//
// A non-converging Worker — one that hit its mechanical turn/wall-clock bound and
// returned worker.OutcomeUnconverged — checkpointed and exited resumably (its
// claim released, its WIP branch preserved). runAttempts respawns it up to
// respawnCap, then escalates: a fresh Worker per attempt (DINV-11) re-acquires the
// claim and resumes from the pushed WIP branch. The re-open is idempotent on the
// task's deterministic head branch, so no respawn yields a duplicate proposal
// (AP-22). The loop is bounded by respawnCap+1 iterations — never an unbounded
// retry (DINV-9). attempts counts total Worker runs (1 initial + up to respawnCap
// respawns).
func (d *Dispatcher) runAttempts(ctx context.Context, claim port.Lease, item *sweepItem, opts []worker.Option) TaskResult {
	task := item.Task
	rt := item.Runtime
	item.Impl.Phase = implRunning
	attempts := 0
	tokens := 0
	recoveryDiagnostic := ""
	for {
		// Honor cancellation promptly: never respawn into a cancelled context.
		if err := ctx.Err(); err != nil {
			item.Impl.Attempts = attempts
			return TaskResult{Task: task, Class: ClassFailed, Outcome: worker.OutcomeFailed, Ran: attempts > 0, Tokens: tokens, Err: fmt.Errorf("dispatch: task %s cancelled after %d attempt(s): %w", task.ID, attempts, err)}
		}
		w, err := worker.New(worker.Deps{
			Tracker: d.tracker,
			Lease:   claim,
			Space:   d.space,
			Runtime: rt,
			Host:    d.host,
		}, append(append([]worker.Option(nil), opts...), worker.WithRecoveryDiagnostic(recoveryDiagnostic), worker.WithActivityObserver(func(task port.Task, a worker.Activity) {
			if d.reporter == nil {
				return
			}
			event := report.Event{Kind: report.KindAgentProgress, TaskID: task.ID, Issue: task.Number, Harness: task.Harness, Phase: a.Phase, Status: a.Status, ElapsedMS: &a.ElapsedMS, RemainingMS: a.RemainingMS, StdoutLog: a.StdoutLog, StderrLog: a.StderrLog}
			if a.Usage != nil {
				event.SessionID = a.Usage.SessionID
				event.Usage = &report.Usage{Tokens: a.Usage.Tokens, Turns: a.Usage.Turns, TokensObserved: a.Usage.TokensObserved, TurnsObserved: a.Usage.TurnsObserved}
			}
			if a.Err != nil {
				event.Reason = a.Err.Error()
			}
			d.reporter.Emit(event)
		}))...)
		if err != nil {
			item.Impl.Attempts = attempts
			return TaskResult{Task: task, Class: ClassFailed, Ran: false, Err: fmt.Errorf("dispatch: build worker for task %s: %w", task.ID, err)}
		}
		attempts++
		item.Impl.Attempts = attempts
		outcome, runErr := w.Run(ctx, task)
		tokens += w.Usage().Tokens
		if outcome == worker.OutcomeUnconverged && runErr == nil {
			recoveryDiagnostic = w.RecoveryDiagnostic()
			if attempts <= d.respawnCap {
				if recoveryDiagnostic != "" && d.reporter != nil {
					d.reporter.Emit(report.Event{Kind: report.KindRecoveryHandoff, TaskID: task.ID, Issue: task.Number, Attempt: attempts + 1, Cap: d.respawnCap + 1, Phase: "local_gate", Status: "queued", Reason: recoveryDiagnostic})
				}
				continue // under the cap: respawn and resume from the pushed WIP
			}
			// Cap exhausted: escalate. A fail-closed terminal (ClassFailed) carrying
			// the escalation sentinel the reporting surface delivers to its two sinks
			// (§3.4, §3.9). The unconverged Outcome is preserved for visibility.
			return TaskResult{
				Task:    task,
				Class:   ClassFailed,
				Outcome: worker.OutcomeUnconverged,
				Ran:     true,
				Tokens:  tokens,
				Err:     fmt.Errorf("dispatch: task %s did not converge within its bound after %d respawn(s): %w; last gate evidence: %s", task.ID, d.respawnCap, ErrRespawnCapExhausted, recoveryDiagnostic),
			}
		}
		result := classify(task, outcome, runErr)
		result.VerificationAttempted = w.VerificationAttempted()
		result.Tokens = tokens
		if proposal, ok := w.Proposal(); ok {
			result.Proposal = proposal
		}
		if out, ok := w.PrePublishOutcome(); ok {
			result.PrePublish = &out
		}
		if capture, ok := w.ReviewCapture(); ok {
			result.ReviewCapture = &capture
		}
		return result
	}
}

// finalize computes the headline counts once, after every Results element is
// settled, and returns the report by value.
func finalize(results []TaskResult, reviews ...worker.ReviewResult) RunReport {
	rep := RunReport{Results: results, ReviewResults: reviews}
	rep.tally()
	return rep
}

// HeadBranch returns the explicit or deterministic proposal branch. Direct
// issue recovery uses the same identity as dispatch's duplicate-proposal guard.
func HeadBranch(task port.Task) string {
	if task.HeadBranch != "" {
		return task.HeadBranch
	}
	return headBranchFor(task)
}

// headBranchFor derives the deterministic branch a task's proposal is opened
// from, for a task whose tracker left HeadBranch empty (both adapters do — it is
// resolved here, medium-independently). The name is feat/<slug(ID Title)>-<Number>:
// stable across respawns (a resumed Worker re-derives the same branch, keeping
// the re-open idempotent — AP-22, DINV per runAttempts) and unique per task via
// the tracker Number, so two tasks never collide on one branch.
func headBranchFor(task port.Task) string {
	slug := slugify(task.ID + " " + task.Title)
	if slug == "" {
		slug = "task" // degenerate charter (no alnum in ID+Title); Number keeps it unique
	}
	if scope := task.ScopeToken(); scope != "" {
		return fmt.Sprintf("feat/%s/%s-%d", scope, slug, task.Number)
	}
	return fmt.Sprintf("feat/%s-%d", slug, task.Number)
}

// slugify reduces s to a lowercase, hyphen-joined, ASCII-alnum slug bounded in
// length, so a long title never yields an unwieldy branch name. Runs of
// non-alnum characters collapse to a single hyphen and leading/trailing hyphens
// are trimmed, yielding a clean git-ref component.
func slugify(s string) string {
	const maxLen = 48
	var b strings.Builder
	lastHyphen := false
	for _, r := range strings.ToLower(s) {
		if b.Len() >= maxLen {
			break
		}
		switch {
		case (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'):
			b.WriteRune(r)
			lastHyphen = false
		default:
			if !lastHyphen && b.Len() > 0 {
				b.WriteByte('-')
				lastHyphen = true
			}
		}
	}
	return strings.Trim(b.String(), "-")
}

// isClaimLost reports whether err is (or wraps) port.ErrClaimLost — the skip
// signal a losing claim race returns (DINV-5, AP-22).
func isClaimLost(err error) bool {
	return errors.Is(err, port.ErrClaimLost)
}

// serialLease decorates a port.Lease so that Acquire is mutually exclusive across
// the Workers of one sweep. It realizes "claim serially, run concurrently" (§3.6
// step 2) without touching the Worker: the Worker calls Acquire as ever, and the
// guard makes those calls one-at-a-time so no two Workers race a claim before the
// underlying compare-and-swap. Only Acquire is serialized; Release runs on the
// returned Claim and is never gated, so a slow claim never blocks a sibling's
// release.
type serialLease struct {
	mu    sync.Mutex
	inner port.Lease
}

// Acquire serializes access to the underlying Lease's Acquire and returns its
// Claim unchanged.
func (s *serialLease) Acquire(ctx context.Context, task port.Task) (port.Claim, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.inner.Acquire(ctx, task)
}
