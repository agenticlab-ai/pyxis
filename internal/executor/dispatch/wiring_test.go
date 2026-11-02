package dispatch

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/entity"
	"github.com/agenticlab-ai/pyxis/internal/executor/port"
	"github.com/agenticlab-ai/pyxis/internal/executor/report"
	"github.com/agenticlab-ai/pyxis/internal/executor/worker"
	"github.com/agenticlab-ai/pyxis/internal/implreview"
	"github.com/agenticlab-ai/pyxis/internal/prdescription"
	"github.com/agenticlab-ai/pyxis/internal/routingevidence"
)

// captureEmitter records every emitted event for assertion — the trivial fake the
// Emitter interface is narrow enough to allow. It is concurrency-safe because a
// sweep emits from many Worker goroutines.
type captureEmitter struct {
	mu     sync.Mutex
	events []report.Event
	onEmit func(report.Event)
}

func (c *captureEmitter) Emit(e report.Event) {
	c.mu.Lock()
	c.events = append(c.events, e)
	if c.onEmit != nil {
		c.onEmit(e)
	}
	c.mu.Unlock()
}

func (c *captureEmitter) snapshot() []report.Event {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]report.Event, len(c.events))
	copy(out, c.events)
	return out
}

// find returns the first event of the given kind for the given task id.
func (c *captureEmitter) find(kind report.Kind, taskID string) (report.Event, bool) {
	for _, e := range c.snapshot() {
		if e.Kind == kind && e.TaskID == taskID {
			return e, true
		}
	}
	return report.Event{}, false
}

func (c *captureEmitter) countKind(kind report.Kind) int {
	n := 0
	for _, e := range c.snapshot() {
		if e.Kind == kind {
			n++
		}
	}
	return n
}

func TestImplementationFactoryOptionsAndFailures(t *testing.T) {
	admissionErr := errors.New("admission unavailable")
	outcomeErr := errors.New("outcome unavailable")
	d := &Dispatcher{}
	admissionFactory := ImplementationAdmissionFactory(func(port.AgentRuntime) (worker.ImplementationAdmission, error) {
		return nil, admissionErr
	})
	outcomeFactory := ImplementationOutcomeFactory(func(port.AgentRuntime) (worker.ImplementationOutcome, error) {
		return nil, outcomeErr
	})
	WithImplementationAdmissionFactory(admissionFactory)(d)
	WithImplementationOutcomeFactory(outcomeFactory)(d)
	if d.implementationAdmissionFactory == nil || d.implementationOutcomeFactory == nil {
		t.Fatal("implementation factories were not installed")
	}
	if _, err := d.workerOptsFor(port.Task{}, &fakeRuntime{}); !errors.Is(err, admissionErr) {
		t.Fatalf("admission factory error = %v", err)
	}
	d.implementationAdmissionFactory = func(port.AgentRuntime) (worker.ImplementationAdmission, error) { return nil, nil }
	if _, err := d.workerOptsFor(port.Task{}, &fakeRuntime{}); err == nil || !strings.Contains(err.Error(), "returned nil") {
		t.Fatalf("nil admission error = %v", err)
	}
	d.implementationAdmissionFactory = func(port.AgentRuntime) (worker.ImplementationAdmission, error) {
		return func(context.Context, port.Task) (routingevidence.BoundaryPermit, error) { return nil, nil }, nil
	}
	d.implementationOutcomeFactory = nil
	if _, err := d.workerOptsFor(port.Task{}, &fakeRuntime{}); err == nil || !strings.Contains(err.Error(), "outcome factory is absent") {
		t.Fatalf("missing outcome factory error = %v", err)
	}
}

// fakeGreenDriver stands in for the drivetogreen.Driver at the Worker seam: it
// captures the runtime it was built over (to prove the per-task binding) and,
// on Drive, simulates one self-heal attempt (emitting ci_fix_attempt the way the
// real driver's OnAttempt does) before returning its scripted result.
type fakeGreenDriver struct {
	rt       port.AgentRuntime
	onDrive  func(taskID string)
	result   worker.GreenDriveResult
	driveErr error
}

type routedFakeRuntime struct {
	port.AgentRuntime
	route port.ModelRoute
}

func (r *routedFakeRuntime) ModelRoute() port.ModelRoute { return r.route }

func (g *fakeGreenDriver) Drive(_ context.Context, task port.Task, _ port.Workdir, _ port.Proposal) (worker.GreenDriveResult, error) {
	if g.onDrive != nil {
		g.onDrive(task.ID)
	}
	return g.result, g.driveErr
}

// GateGreen defaults to not-green: these wiring tests exercise clean-exit and
// respawn dispatch, not the ship-truncated-green path, so a truncation stays
// respawn-eligible.
func (g *fakeGreenDriver) GateGreen(_ context.Context, _ port.Workdir) (bool, error) {
	return false, nil
}

// DriveLocal passes vacuously: the wiring tests exercise dispatch, not the
// local fixer loop (covered in package drivetogreen).
func (g *fakeGreenDriver) DriveLocal(_ context.Context, _ port.Task, _ port.Workdir) (worker.LocalDriveResult, error) {
	return worker.LocalDriveResult{Green: true}, nil
}

// --- resolver: each task routes to its resolved backend runtime ---------------

// With a runtime resolver configured, each task runs on the runtime its charter
// resolves to, and task_dispatched names that resolved backend — the per-task
// routing seam (§3.8). Deps.Runtime is left nil, proving the resolver fully
// replaces the single-runtime path.
func TestSweep_ResolvesBackendPerTask(t *testing.T) {
	tr, ls, ws, _, hs := greenFakes(t)
	alphaBase, rtBravo := &fakeRuntime{}, &fakeRuntime{}
	rtAlpha := &routedFakeRuntime{
		AgentRuntime: alphaBase,
		route:        port.ModelRoute{Model: "alpha-model", Effort: "high"},
	}
	// The resolver maps a task's charter backend slot to a concrete runtime and
	// reports the resolved backend name for the event.
	resolver := func(task port.Task) (entity.Harness, port.AgentRuntime, error) {
		switch task.Harness {
		case "alpha":
			return "alpha", rtAlpha, nil
		default:
			return "bravo", rtBravo, nil
		}
	}
	cap := &captureEmitter{}
	d, err := New(Deps{Tracker: tr, Lease: ls, Space: ws, Host: hs}, // no Deps.Runtime
		Config{Width: 2},
		WithRuntimeResolver(resolver), WithReporter(cap))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	a := task("TA", 1)
	a.Harness = "alpha"
	b := task("TB", 2)
	b.Harness = "codex" // falls to the resolver default → bravo
	tr.setFrontier(a, b)

	rep, err := d.Sweep(context.Background())
	if err != nil {
		t.Fatalf("sweep: %v", err)
	}
	if rep.Succeeded != 2 {
		t.Fatalf("succeeded = %d, want 2", rep.Succeeded)
	}
	// Each task ran on its resolved runtime — a fresh process per backend.
	if got := alphaBase.launchCount(); got != 1 {
		t.Errorf("alpha runtime launches = %d, want 1 (TA routed to it)", got)
	}
	if got := rtBravo.launchCount(); got != 1 {
		t.Errorf("bravo runtime launches = %d, want 1 (TB routed to it)", got)
	}
	// task_dispatched names the resolved backend, not the charter slot.
	if e, ok := cap.find(report.KindTaskDispatched, "TA"); !ok || e.Harness != "alpha" {
		t.Errorf("TA dispatched event = %+v, want backend alpha", e)
	} else if e.Model != "alpha-model" || e.Effort != "high" {
		t.Errorf("TA dispatched route = model %q effort %q, want alpha-model/high", e.Model, e.Effort)
	}
	if e, ok := cap.find(report.KindTaskDispatched, "TB"); !ok || e.Harness != "bravo" {
		t.Errorf("TB dispatched event = %+v, want resolved backend bravo (not the charter codex)", e)
	}
}

// A resolver that fails for a task classes that task failed (a wiring error is
// never a panic) while an independent sibling still succeeds — one mis-routed
// task never halts the sweep (AP-16).
func TestSweep_ResolverErrorClassesTaskFailed(t *testing.T) {
	tr, ls, ws, _, hs := greenFakes(t)
	good := &fakeRuntime{}
	resolver := func(task port.Task) (entity.Harness, port.AgentRuntime, error) {
		if task.ID == "TBad" {
			return "", nil, context.DeadlineExceeded // any resolution failure
		}
		return "good", good, nil
	}
	cap := &captureEmitter{}
	d, err := New(Deps{Tracker: tr, Lease: ls, Space: ws, Host: hs}, Config{Width: 2},
		WithRuntimeResolver(resolver), WithReporter(cap))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	tr.setFrontier(task("TBad", 1), task("TGood", 2))

	rep, err := d.Sweep(context.Background())
	if err != nil {
		t.Fatalf("sweep: %v", err)
	}
	if rep.Failed != 1 || rep.Succeeded != 1 {
		t.Fatalf("counts = %+v, want 1 failed / 1 succeeded", rep)
	}
	if c := classOf(t, rep, 1); c != ClassFailed {
		t.Errorf("mis-routed task class = %v, want failed", c)
	}
	// A task that never resolved a runtime emits a task_outcome but no
	// task_dispatched (it was never dispatched to a Worker).
	if _, ok := cap.find(report.KindTaskDispatched, "TBad"); ok {
		t.Error("mis-routed task must not emit task_dispatched")
	}
	if _, ok := cap.find(report.KindTaskOutcome, "TBad"); !ok {
		t.Error("mis-routed task must still emit a task_outcome")
	}
}

// --- green driver: a red proposal is driven to green per task -----------------

// With a green-driver factory configured, a red proposal is driven to green
// before terminal classification, and the factory is invoked with the task's
// resolved runtime (the fixer runs on the same backend, DINV-11). The self-heal
// attempt surfaces as a ci_fix_attempt event.
func TestSweep_AttachesGreenDriverPerTask(t *testing.T) {
	tr, ls, ws, rt, hs := greenFakes(t)
	hs.checkByNum[1] = port.CheckRed // the proposal is red → the driver must engage
	cap := &captureEmitter{}

	var gotRuntime port.AgentRuntime
	factory := func(r port.AgentRuntime) (worker.GreenDriver, error) {
		gotRuntime = r
		return &fakeGreenDriver{
			rt:      r,
			onDrive: func(id string) { cap.Emit(report.CIFixAttempt(id, 1, 3)) },
			result:  worker.GreenDriveResult{Green: true, Attempts: 1, Cap: 3},
		}, nil
	}
	d, err := New(Deps{Tracker: tr, Lease: ls, Space: ws, Runtime: rt, Host: hs},
		Config{Width: 1}, WithGreenDriverFactory(factory), WithReporter(cap))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	tr.setFrontier(task("T1", 1))

	rep, err := d.Sweep(context.Background())
	if err != nil {
		t.Fatalf("sweep: %v", err)
	}
	if rep.Succeeded != 1 {
		t.Fatalf("succeeded = %d, want 1 (the red proposal was driven to green)", rep.Succeeded)
	}
	if gotRuntime != rt {
		t.Error("the green driver factory was not built over the task's resolved runtime")
	}
	if cap.countKind(report.KindCIFixAttempt) != 1 {
		t.Errorf("ci_fix_attempt events = %d, want 1", cap.countKind(report.KindCIFixAttempt))
	}
	// The task was verified green after the drive.
	if e, ok := cap.find(report.KindTaskVerified, "T1"); !ok || e.Verdict != report.VerdictGreen {
		t.Errorf("verified event = %+v, want verdict green", e)
	}
}

// Without a green-driver factory, a red proposal stays failed — drive-to-green is
// strictly opt-in through the seam, and its absence changes nothing (the base
// verify).
func TestSweep_NoGreenFactoryLeavesRedProposalFailed(t *testing.T) {
	tr, ls, ws, rt, hs := greenFakes(t)
	hs.checkByNum[1] = port.CheckRed
	d := mustDispatcher(t, 1, tr, ls, ws, rt, hs)
	tr.setFrontier(task("T1", 1))

	rep, err := d.Sweep(context.Background())
	if err != nil {
		t.Fatalf("sweep: %v", err)
	}
	if rep.Failed != 1 || rep.Succeeded != 0 {
		t.Fatalf("counts = %+v, want 1 failed (no green driver, red stays failed)", rep)
	}
}

// --- two sinks: the same run reports under attended and unattended ------------

// The same frontier-sweep reports identically under an attended invocation (the
// operator's live slog view) and an unattended one (argus's JSONL journal), over
// ONE event stream fanned to two sinks (§3.9, AP-2). This is T13's headline
// acceptance criterion driven end to end through the Dispatcher.
func TestSweep_SameRunReportsUnderBothSinks(t *testing.T) {
	tr, ls, ws, rt, hs := greenFakes(t)

	var opBuf, journal bytes.Buffer
	op := report.NewOperatorSink(slog.New(slog.NewTextHandler(&opBuf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	argus := report.NewArgusSink(&journal)
	reporter := report.NewReporter("run-both", op, argus)

	d, err := New(Deps{Tracker: tr, Lease: ls, Space: ws, Runtime: rt, Host: hs},
		Config{Width: 2}, WithReporter(reporter))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	tr.setFrontier(task("T1", 1), task("T2", 2))

	rep, err := d.Sweep(context.Background())
	if err != nil {
		t.Fatalf("sweep: %v", err)
	}
	if rep.Succeeded != 2 {
		t.Fatalf("succeeded = %d, want 2", rep.Succeeded)
	}

	// Attended: the operator view carries the run and per-task lifecycle.
	opText := opBuf.String()
	for _, want := range []string{"run started", "task dispatched", "task verified", "task outcome", "run finished", "run=run-both"} {
		if !strings.Contains(opText, want) {
			t.Errorf("operator view missing %q\n%s", want, opText)
		}
	}

	// Unattended: the identical stream is on the JSONL journal. Assert the full
	// per-task lifecycle for both tasks plus the run brackets.
	evs := parseJournal(t, &journal)
	if n := countJournalKind(evs, report.KindRunStarted); n != 1 {
		t.Errorf("run_started in journal = %d, want 1", n)
	}
	if n := countJournalKind(evs, report.KindRunFinished); n != 1 {
		t.Errorf("run_finished in journal = %d, want 1", n)
	}
	for _, id := range []string{"T1", "T2"} {
		assertJournalHas(t, evs, report.KindTaskDispatched, id)
		assertJournalHas(t, evs, report.KindTaskVerified, id)
		assertJournalHas(t, evs, report.KindTaskOutcome, id)
	}
	// run_finished carries the final tally.
	for _, e := range evs {
		if e.Kind == report.KindRunFinished {
			if e.Counts == nil || e.Counts.Succeeded != 2 {
				t.Errorf("journal run_finished counts = %+v, want succeeded 2", e.Counts)
			}
		}
	}
}

// --- fail-closed escalation: a stuck task escalates to the argus sink ---------

// A task that never converges (its Await parks until the wall-clock bound kills
// it) exhausts its respawn cap and fail-closed-escalates: the argus journal
// carries a task_outcome with escalated=true, the escalation handler fires, and
// the sink's escalation count reflects it — T13's fail-closed criterion (§3.9).
func TestSweep_StuckTaskFailClosedEscalatesToArgus(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		tr := &fakeTracker{}
		ls := &fakeLease{claimErr: map[int]error{}}
		ws := &fakeWorkspace{dir: t.TempDir()}
		rt := &scriptedRuntime{blockAttempts: 1000} // every attempt hits the bound
		hs := newIdempotentHost()

		var journal bytes.Buffer
		var escalated []report.Event
		argus := report.NewArgusSink(&journal, report.WithEscalationHandler(func(e report.Event) {
			escalated = append(escalated, e)
		}))
		reporter := report.NewReporter("run-stuck", argus)

		d, err := New(Deps{Tracker: tr, Lease: ls, Space: ws, Runtime: rt, Host: hs},
			Config{Width: 1, RespawnCap: 1, WorkerBudget: 30 * time.Minute},
			WithReporter(reporter))
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		tr.setFrontier(task("T1", 1))

		rep, err := d.Sweep(context.Background())
		if err != nil {
			t.Fatalf("sweep: %v", err)
		}
		if rep.Failed != 1 {
			t.Fatalf("counts = %+v, want 1 failed (escalated)", rep)
		}
		if got := argus.Escalations(); got != 1 {
			t.Fatalf("argus escalations = %d, want 1", got)
		}
		if len(escalated) != 1 || escalated[0].TaskID != "T1" || !escalated[0].Escalated {
			t.Fatalf("escalation handler saw %+v, want one escalated T1", escalated)
		}
		// The escalated outcome is on the journal (files-as-truth), flagged escalated.
		evs := parseJournal(t, &journal)
		e := requireJournal(t, evs, report.KindTaskOutcome, "T1")
		if e.Class != "failed" || !e.Escalated {
			t.Errorf("journaled outcome = %+v, want escalated failed", e)
		}
		if e.Reason == "" {
			t.Error("escalated outcome should carry a reason (the wrapped escalation error)")
		}

	})
}

// --- cancel: cancelling a run kills in-flight Workers and releases claims ------

// Cancelling a run kills its in-flight Worker (its agent process) and releases the
// claim (fire-or-release), leaving the task recoverable — T13's cancel criterion,
// exercised through the full reporting wiring (the run is bracketed by
// run_started/run_finished even when cancelled mid-flight).
func TestSweep_CancelKillsInFlightAndReleasesClaims(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		tr := &fakeTracker{}
		ls := &fakeLease{claimErr: map[int]error{}}
		ws := &fakeWorkspace{dir: t.TempDir()}
		rt := &scriptedRuntime{blockAttempts: 1, entered: make(chan struct{}, 1)}
		hs := newIdempotentHost()
		cap := &captureEmitter{}

		d, err := New(Deps{Tracker: tr, Lease: ls, Space: ws, Runtime: rt, Host: hs},
			Config{Width: 1, WorkerBudget: time.Minute}, // the cancel wins, not the budget
			WithReporter(cap))
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		tr.setFrontier(task("T1", 1))

		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan RunReport, 1)
		go func() {
			rep, err := d.Sweep(ctx)
			if err != nil {
				t.Errorf("sweep: %v", err)
			}
			done <- rep
		}()

		<-rt.entered // the agent is in flight
		cancel()
		<-done

		if got := rt.killed.Load(); got != 1 {
			t.Errorf("Kill count = %d, want 1 (cancellation kills the in-flight process)", got)
		}
		if got := ls.releases.Load(); got != 1 {
			t.Errorf("claims released = %d, want 1 (fire-or-release on cancellation)", got)
		}
		// The run stayed bracketed on the event stream despite the cancel.
		if cap.countKind(report.KindRunStarted) != 1 || cap.countKind(report.KindRunFinished) != 1 {
			t.Errorf("run brackets = started %d / finished %d, want 1 / 1",
				cap.countKind(report.KindRunStarted), cap.countKind(report.KindRunFinished))
		}

	})
}

// --- journal helpers ----------------------------------------------------------

func parseJournal(t *testing.T, buf *bytes.Buffer) []report.Event {
	t.Helper()
	var out []report.Event
	sc := bufio.NewScanner(buf)
	for sc.Scan() {
		if len(bytes.TrimSpace(sc.Bytes())) == 0 {
			continue
		}
		var e report.Event
		if err := json.Unmarshal(sc.Bytes(), &e); err != nil {
			t.Fatalf("journal line not JSON: %q: %v", sc.Bytes(), err)
		}
		out = append(out, e)
	}
	return out
}

func countJournalKind(evs []report.Event, kind report.Kind) int {
	n := 0
	for _, e := range evs {
		if e.Kind == kind {
			n++
		}
	}
	return n
}

func assertJournalHas(t *testing.T, evs []report.Event, kind report.Kind, taskID string) {
	t.Helper()
	for _, e := range evs {
		if e.Kind == kind && e.TaskID == taskID {
			return
		}
	}
	t.Errorf("journal missing a %s event for task %s", kind, taskID)
}

func requireJournal(t *testing.T, evs []report.Event, kind report.Kind, taskID string) report.Event {
	t.Helper()
	for _, e := range evs {
		if e.Kind == kind && e.TaskID == taskID {
			return e
		}
	}
	t.Fatalf("journal missing a %s event for task %s", kind, taskID)
	return report.Event{}
}

// --- review invoker: the impl-review seam fires once per green task (#556) ---

// recordingInvoker counts Invoke calls and captures its inputs.
type recordingInvoker struct {
	mu                sync.Mutex
	backend           entity.Harness
	preflightCalls    int
	preflightErr      error
	calls             int
	active            int
	peak              int
	task              port.Task
	proposal          port.Proposal
	result            worker.ReviewResult
	events            *captureEmitter
	sawGreen          bool
	lease             *fakeLease
	sawReleased       bool
	captures          int
	handoffs          int
	handoffErr        error
	handoffWhileHeld  bool
	handoffSawOutcome bool
	capturedWhileHeld bool
	captureResult     worker.ReviewCapture
	entered           chan struct{}
	block             chan struct{}
}

type recordingHandoffRepairer struct {
	rows               []ReviewHandoff
	events             *captureEmitter
	completeCalls      int
	completeSawOutcome bool
	err                error
}

type recordingPairRepairer struct {
	*recordingHandoffRepairer
	pairs []ReviewPairResult
	err   error
}

func (r *recordingPairRepairer) RepairPairs(context.Context) ([]ReviewPairResult, error) {
	return append([]ReviewPairResult(nil), r.pairs...), r.err
}

func (r *recordingHandoffRepairer) Pending(context.Context) ([]ReviewHandoff, error) {
	return append([]ReviewHandoff(nil), r.rows...), r.err
}

func (r *recordingHandoffRepairer) Complete(_ context.Context, _ string) error {
	r.completeCalls++
	if r.events != nil {
		_, r.completeSawOutcome = r.events.find(report.KindTaskOutcome, "T1")
	}
	return r.err
}

func TestSweepClassesPairAndHandoffRepairInfrastructureFailures(t *testing.T) {
	for _, tc := range []struct {
		name   string
		repair ReviewHandoffRepairer
	}{
		{name: "pair lifecycle", repair: &recordingPairRepairer{
			recordingHandoffRepairer: &recordingHandoffRepairer{},
			err:                      errors.New("pair repair unavailable"),
		}},
		{name: "handoff frontier", repair: &recordingHandoffRepairer{err: errors.New("handoff frontier unavailable")}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tr, ls, ws, rt, hs := greenFakes(t)
			d, err := New(Deps{Tracker: tr, Lease: ls, Space: ws, Runtime: rt, Host: hs}, Config{Width: 1},
				WithReviewHandoffRepairer(tc.repair))
			if err != nil {
				t.Fatal(err)
			}
			rep, err := d.Sweep(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			if rep.ReviewFailed != 1 || rep.ReviewInfrastructureFailed != 1 {
				t.Fatalf("repair failure report = %+v", rep)
			}
		})
	}
}

func (r *recordingInvoker) Preflight(context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.preflightCalls++
	return r.preflightErr
}

func (r *recordingInvoker) Capture(_ context.Context, task port.Task, proposal port.Proposal) worker.ReviewCapture {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.captures++
	if r.lease != nil {
		r.capturedWhileHeld = r.lease.releases.Load() == 0
	}
	backend := r.backend
	if backend == "" {
		backend = "claude"
	}
	if r.captureResult.Err != nil || r.captureResult.ObligationID != "" {
		return r.captureResult
	}
	return worker.ReviewCapture{ProposalRef: "github:agenticlab-ai/pyxis#1", Revision: "abc123", ObligationID: "review-obligation-1", Harness: backend, Persisted: true}
}

func (r *recordingInvoker) CompleteHandoff(_ context.Context, _ worker.ReviewCapture) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.handoffs++
	if r.lease != nil {
		r.handoffWhileHeld = r.lease.releases.Load() == 0
	}
	if r.events != nil {
		_, r.handoffSawOutcome = r.events.find(report.KindTaskOutcome, "T1")
	}
	return r.handoffErr
}

func (r *recordingInvoker) PreparePending(_ context.Context, obligationID string) (port.Task, port.Proposal, error) {
	if obligationID == "" {
		return port.Task{}, port.Proposal{}, errors.New("empty obligation")
	}
	return task("T1", 1), port.Proposal{Number: 1}, nil
}

func (r *recordingInvoker) PrepareSelected(ctx context.Context, obligationID, driveRoundID string) (port.Task, port.Proposal, error) {
	if driveRoundID == "" {
		return port.Task{}, port.Proposal{}, errors.New("empty drive round")
	}
	return r.PreparePending(ctx, obligationID)
}

func (r *recordingInvoker) Invoke(_ context.Context, task port.Task, proposal port.Proposal) worker.ReviewResult {
	r.mu.Lock()
	r.calls++
	r.active++
	if r.active > r.peak {
		r.peak = r.active
	}
	r.task, r.proposal = task, proposal
	if r.events != nil {
		_, r.sawGreen = r.events.find(report.KindTaskVerified, task.ID)
	}
	if r.lease != nil {
		r.sawReleased = r.lease.releases.Load() > 0
	}
	entered, block := r.entered, r.block
	result := r.result
	if result.Harness == "" {
		result.Harness = r.backend
	}
	if result.Status == report.ReviewStatusCompleted {
		if result.AttemptID == "" {
			result.AttemptID = "attempt-1"
		}
		if result.ObligationID == "" {
			result.ObligationID = "review-obligation-1"
		}
		if result.ProposalRef == "" {
			result.ProposalRef = "github:agenticlab-ai/pyxis#1"
		}
		if result.Revision == "" {
			result.Revision = "abc123"
		}
		if result.ArtifactRef == "" {
			result.ArtifactRef = "sha256:artifact"
		}
		if result.ArtifactProofRef == "" {
			result.ArtifactProofRef = "review-attempt-proof-1"
		}
		if result.NativeReceipt == "" {
			result.NativeReceipt = "github-comment:R_test:77"
		}
		if result.EVT13Ref == "" {
			result.EVT13Ref = "evidence:evt13:1"
		}
		if result.EVT17Ref == "" {
			result.EVT17Ref = "evidence:evt17:1"
		}
		if result.ImplReviewTerminal == nil {
			result.ImplReviewTerminal = &implreview.PublicationTerminal{
				ReviewProcessID: result.AttemptID, ReviewObligationID: result.ObligationID,
				PublicationAttemptID: "publication-1", ProposalID: result.ProposalRef,
				ReviewedRevision: result.Revision, Medium: "github", IdempotencyID: "idempotency-1",
				Status: implreview.PublicationVerified, ArtifactRef: result.ArtifactRef,
				NativeReceipt: result.NativeReceipt, AttemptProofRef: result.ArtifactProofRef, EVT13Ref: result.EVT13Ref,
			}
		}
		if result.Publication == nil {
			result.Publication = &worker.ReviewPublicationTerminal{
				ReviewAttemptID: result.AttemptID, ReviewObligationID: result.ObligationID,
				ProposalRef: result.ProposalRef, Revision: result.Revision, Status: result.Status,
				ArtifactRef: result.ArtifactRef, NativeReceipt: result.NativeReceipt, AttemptProofRef: result.ArtifactProofRef,
			}
		}
	}
	r.mu.Unlock()
	if entered != nil {
		select {
		case entered <- struct{}{}:
		default:
		}
	}
	if block != nil {
		<-block
	}
	r.mu.Lock()
	r.active--
	r.mu.Unlock()
	return result
}

// With a review-invoker factory configured, a task that reaches ladder-green
// invokes the reviewer exactly once with its green proposal; the factory result
// installs through the Worker option, and the review's advisory contract keeps
// the outcome untouched (#556, impl-review-v1 §3.4).
func TestSweep_AttachesReviewInvokerPerTask(t *testing.T) {
	tr, ls, ws, rt, hs := greenFakes(t)
	hs.checkByNum[1] = port.CheckGreen
	events := &captureEmitter{}
	outcomeBeforeRelease := false
	events.onEmit = func(e report.Event) {
		if e.Kind == report.KindTaskOutcome && e.TaskID == "T1" {
			outcomeBeforeRelease = ls.releases.Load() == 0
		}
	}
	inv := &recordingInvoker{events: events, lease: ls, result: worker.ReviewResult{
		Harness: "claude", ReviewerPrincipal: "review-bot", ProposalRef: "github:agenticlab-ai/pyxis#1", Revision: "abc123", AttemptID: "attempt-1", ObligationID: "review-obligation-1",
		NativeProfileID: "opus-max-review", CatalogID: "claude-review-v1", PolicyIdentity: "sha256:policy",
		Model: "claude-opus-5", Effort: "max", Phase: worker.ReviewPhaseArtifact, ArtifactRef: "github:77", ArtifactProofRef: "github:broker-receipt/77",
		Status: report.ReviewStatusCompleted, ObligationStatus: report.ReviewObligationVerified, Tokens: 37,
	}}
	d, err := New(Deps{Tracker: tr, Lease: ls, Space: ws, Runtime: rt, Host: hs},
		Config{Width: 1},
		WithReporter(events),
		WithReviewInvokerFactory(func(entity.Harness) (worker.ReviewService, error) { return inv, nil }))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	tr.setFrontier(task("T1", 1))

	rep, err := d.Sweep(context.Background())
	if err != nil {
		t.Fatalf("sweep: %v", err)
	}
	if rep.Succeeded != 1 || rep.ReviewCompleted != 1 || rep.ReviewFailed != 0 || rep.Tokens != 38 {
		t.Fatalf("counts = %+v, want one succeeded task and one completed review", rep)
	}
	if inv.calls != 1 {
		t.Fatalf("reviewer invoked %d times, want exactly 1 at ladder-green", inv.calls)
	}
	if inv.captures != 1 || !inv.capturedWhileHeld || inv.handoffs != 1 || !inv.handoffWhileHeld || !inv.handoffSawOutcome {
		t.Fatalf("capture/handoff = captures:%d capture-held:%v handoffs:%d handoff-held:%v saw-outcome:%v", inv.captures, inv.capturedWhileHeld, inv.handoffs, inv.handoffWhileHeld, inv.handoffSawOutcome)
	}
	if !outcomeBeforeRelease {
		t.Fatal("succeeded implementation terminal was not emitted before claim release")
	}
	if inv.task.ID != "T1" || inv.proposal.Number != 1 {
		t.Errorf("reviewer inputs = task %q proposal %d, want T1/1", inv.task.ID, inv.proposal.Number)
	}
	if !inv.sawGreen {
		t.Fatal("reviewer ran before task_verified green authorized it")
	}
	if !inv.sawReleased {
		t.Fatal("reviewer ran before the implementation claim was released")
	}
	reviewTerminal, ok := events.find(report.KindImplReviewTerminal, "T1")
	if !ok {
		t.Fatal("review result was absent from impl_review_terminal")
	}
	if reviewTerminal.Status != report.ReviewStatusCompleted || reviewTerminal.Harness != "claude" || reviewTerminal.ArtifactRef != "github:77" || reviewTerminal.NativeProfile != "opus-max-review" {
		t.Fatalf("impl_review_terminal fields = %+v", reviewTerminal)
	}
	hostedTerminal, ok := events.find(report.KindHostedReviewPublicationTerminal, "")
	if !ok || hostedTerminal.ReviewAttemptID != "attempt-1" || hostedTerminal.ReviewObligationID != "review-obligation-1" || hostedTerminal.NativeReceipt == "" || hostedTerminal.AttemptProofRef == "" {
		t.Fatalf("hosted publication terminal fields = %+v", hostedTerminal)
	}
	publicationTerminal, ok := events.find(report.KindReviewArtifactPublicationTerminal, "")
	if !ok || publicationTerminal.ReviewProcessID != "attempt-1" || publicationTerminal.ReviewObligationID != "review-obligation-1" || publicationTerminal.ProposalID == "" || publicationTerminal.ReviewedRevision == "" {
		t.Fatalf("impl-review publication terminal fields = %+v", publicationTerminal)
	}
	snapshot := events.snapshot()
	verifiedAt, outcomeAt, publicationAt, hostedAt, reviewAt := -1, -1, -1, -1, -1
	for idx, event := range snapshot {
		if event.TaskID == "T1" && event.Kind == report.KindTaskVerified {
			verifiedAt = idx
		}
		if event.TaskID == "T1" && event.Kind == report.KindTaskOutcome {
			outcomeAt = idx
		}
		if event.Kind == report.KindHostedReviewPublicationTerminal {
			hostedAt = idx
		}
		if event.Kind == report.KindReviewArtifactPublicationTerminal {
			publicationAt = idx
		}
		if event.TaskID == "T1" && event.Kind == report.KindImplReviewTerminal {
			reviewAt = idx
		}
	}
	if verifiedAt < 0 || outcomeAt <= verifiedAt || publicationAt <= outcomeAt || hostedAt <= publicationAt || reviewAt <= hostedAt {
		t.Fatalf("event order = verified %d outcome %d publication %d hosted %d review %d; events=%+v", verifiedAt, outcomeAt, publicationAt, hostedAt, reviewAt, snapshot)
	}
	if events.countKind(report.KindTaskVerified) != 1 {
		t.Fatalf("task_verified events = %d, want exactly one", events.countKind(report.KindTaskVerified))
	}
	runFinished, ok := events.find(report.KindRunFinished, "")
	if !ok || runFinished.Counts == nil || runFinished.Counts.ReviewCompleted != 1 || runFinished.Counts.Tokens != 38 {
		t.Fatalf("run_finished review counts = %+v", runFinished)
	}
}

func TestSweep_RepairsHandoffAndReviewsWithEmptyImplementationFrontier(t *testing.T) {
	tr, ls, ws, rt, hs := greenFakes(t)
	events := &captureEmitter{}
	repair := &recordingHandoffRepairer{events: events, rows: []ReviewHandoff{{ObligationID: "review-obligation-1", Task: task("T1", 1), Harness: "claude", ProposalRef: "github:o/r#1", Revision: "abc123"}}}
	inv := &recordingInvoker{result: worker.ReviewResult{Harness: "claude", ReviewerPrincipal: "review-bot", ProposalRef: "github:o/r#1", Revision: "abc123", AttemptID: "attempt-1", ObligationID: "review-obligation-1", Phase: worker.ReviewPhaseArtifact, ArtifactRef: "github:77", ArtifactProofRef: "github:broker-receipt/77", Status: report.ReviewStatusCompleted, ObligationStatus: report.ReviewObligationVerified}}
	d, err := New(Deps{Tracker: tr, Lease: ls, Space: ws, Runtime: rt, Host: hs}, Config{Width: 1, ReviewWidth: 1},
		WithReporter(events), WithReviewHandoffRepairer(repair),
		WithReviewInvokerFactory(func(entity.Harness) (worker.ReviewService, error) { return inv, nil }))
	if err != nil {
		t.Fatal(err)
	}
	tr.setFrontier()
	rep, err := d.Sweep(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if rep.Succeeded != 0 || rep.Failed != 0 || rep.ReviewCompleted != 1 || rep.ReviewFailed != 0 {
		t.Fatalf("counts = %+v", rep)
	}
	if repair.completeCalls != 1 || !repair.completeSawOutcome || inv.calls != 1 {
		t.Fatalf("repair calls=%d saw-outcome=%v review calls=%d", repair.completeCalls, repair.completeSawOutcome, inv.calls)
	}
	if rt.launchCount() != 0 || ls.releases.Load() != 0 {
		t.Fatalf("implementation launches=%d releases=%d, want zero", rt.launchCount(), ls.releases.Load())
	}
	if events.countKind(report.KindTaskVerified) != 1 || events.countKind(report.KindTaskOutcome) != 1 || events.countKind(report.KindImplReviewTerminal) != 1 {
		t.Fatalf("events = %+v", events.snapshot())
	}
}

func TestSweep_UnboundRecoveredHandoffFailsBeforeReview(t *testing.T) {
	tr, ls, ws, rt, hs := greenFakes(t)
	events := &captureEmitter{}
	repair := &recordingHandoffRepairer{events: events, rows: []ReviewHandoff{{
		ObligationID: "review-obligation-1", Task: task("T1", 1), Harness: "claude",
		ProposalRef: "github:o/r#1", Revision: "abc123",
	}}}
	d, err := New(Deps{Tracker: tr, Lease: ls, Space: ws, Runtime: rt, Host: hs}, Config{Width: 1, ReviewWidth: 1},
		WithReporter(events), WithReviewHandoffRepairer(repair))
	if err != nil {
		t.Fatal(err)
	}
	tr.setFrontier()
	rep, err := d.Sweep(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if rep.Succeeded != 0 || rep.Failed != 0 || rep.ReviewCompleted != 0 || rep.ReviewFailed != 1 {
		t.Fatalf("counts = %+v", rep)
	}
	if repair.completeCalls != 1 || rt.launchCount() != 0 || ls.releases.Load() != 0 {
		t.Fatalf("repair calls=%d launches=%d releases=%d", repair.completeCalls, rt.launchCount(), ls.releases.Load())
	}
	terminal, ok := events.find(report.KindImplReviewTerminal, "T1")
	if !ok || !strings.Contains(terminal.Reason, "no preflighted service") {
		t.Fatalf("review terminal = %+v", terminal)
	}
}

func TestSweepRepairsPairVerifiedLifecycleBeforeOrdinaryFrontierWork(t *testing.T) {
	tr, ls, ws, rt, hs := greenFakes(t)
	events := &captureEmitter{}
	implTerminal := &implreview.PublicationTerminal{ReviewProcessID: "process-1", ReviewObligationID: "obligation-1",
		PublicationAttemptID: "publication-1", ProposalID: "github:o/r#1", ReviewedRevision: "abc123",
		Medium: "github", IdempotencyID: "idempotency-1", Status: implreview.PublicationVerified,
		ArtifactRef: "sha256:artifact", NativeReceipt: "github-comment:R_test:7", AttemptProofRef: "proof-1", EVT13Ref: "evidence:evt13:1"}
	hostTerminal := &worker.ReviewPublicationTerminal{ReviewAttemptID: "process-1", ReviewObligationID: "obligation-1",
		ProposalRef: "github:o/r#1", Revision: "abc123", Status: "verified", ArtifactRef: "sha256:artifact",
		NativeReceipt: "github-comment:R_test:7", AttemptProofRef: "proof-1"}
	result := worker.ReviewResult{AttemptID: "process-1", ObligationID: "obligation-1", Harness: "codex",
		ProposalRef: "github:o/r#1", Revision: "abc123", Phase: worker.ReviewPhaseArtifact,
		Status: report.ReviewStatusCompleted, ObligationStatus: report.ReviewObligationVerified,
		ArtifactRef: "sha256:artifact", ArtifactProofRef: "proof-1", NativeReceipt: "github-comment:R_test:7",
		EVT13Ref: "evidence:evt13:1", EVT17Ref: "evidence:evt17:1", ImplReviewTerminal: implTerminal, Publication: hostTerminal}
	repair := &recordingPairRepairer{recordingHandoffRepairer: &recordingHandoffRepairer{},
		pairs: []ReviewPairResult{{TaskID: "T1", Result: result}}}
	d, err := New(Deps{Tracker: tr, Lease: ls, Space: ws, Runtime: rt, Host: hs}, Config{Width: 1},
		WithReporter(events), WithReviewHandoffRepairer(repair))
	if err != nil {
		t.Fatal(err)
	}
	tr.setFrontier()
	rep, err := d.Sweep(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if rep.ReviewCompleted != 1 || rep.ReviewFailed != 0 || rt.launchCount() != 0 || ls.releases.Load() != 0 {
		t.Fatalf("pair lifecycle repair report=%+v implementation launches=%d releases=%d", rep, rt.launchCount(), ls.releases.Load())
	}
	for _, kind := range []report.Kind{report.KindReviewArtifactPublicationTerminal, report.KindHostedReviewPublicationTerminal, report.KindImplReviewTerminal, report.KindRunFinished} {
		if events.countKind(kind) != 1 {
			t.Errorf("%s events=%d, want 1: %+v", kind, events.countKind(kind), events.snapshot())
		}
	}
}

func TestSweepUnrepairablePairDoesNotBlockUnrelatedImplementationFrontier(t *testing.T) {
	tr, ls, ws, rt, hs := greenFakes(t)
	hs.checkByNum[2] = port.CheckGreen
	failure := worker.ReviewResult{AttemptID: "process-bad", ObligationID: "obligation-bad", Harness: "codex",
		Phase: worker.ReviewPhaseAttribution, Status: report.ReviewStatusFailed,
		ObligationStatus: report.ReviewObligationPending, FailureClass: worker.ReviewFailureArtifactUntrusted,
		Err: errors.New("reciprocal pair is corrupt")}
	repair := &recordingPairRepairer{recordingHandoffRepairer: &recordingHandoffRepairer{},
		pairs: []ReviewPairResult{{TaskID: "T1", Result: failure}}}
	d, err := New(Deps{Tracker: tr, Lease: ls, Space: ws, Runtime: rt, Host: hs}, Config{Width: 1},
		WithReviewHandoffRepairer(repair))
	if err != nil {
		t.Fatal(err)
	}
	tr.setFrontier(task("T2", 2))
	rep, err := d.Sweep(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if rep.Succeeded != 1 || rep.ReviewFailed != 1 || rep.ReviewPending != 1 || rt.launchCount() != 1 {
		t.Fatalf("unrepairable pair monopolized sweep: report=%+v launches=%d", rep, rt.launchCount())
	}
}

func TestSweep_ReviewLatencyDoesNotHoldImplementationWidth(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		tr, ls, ws, rt, hs := greenFakes(t)
		hs.checkByNum[1], hs.checkByNum[2] = port.CheckGreen, port.CheckGreen
		entered, unblock := make(chan struct{}, 1), make(chan struct{})
		release := sync.OnceFunc(func() { close(unblock) })
		defer release()
		inv := &recordingInvoker{lease: ls, entered: entered, block: unblock, result: worker.ReviewResult{Harness: "claude", ReviewerPrincipal: "review-bot", AttemptID: "attempt-1", ObligationID: "review-obligation-1", Phase: worker.ReviewPhaseArtifact, Status: report.ReviewStatusCompleted, ObligationStatus: report.ReviewObligationVerified, ArtifactRef: "github:7", ArtifactProofRef: "github:broker-receipt/7"}}
		d, err := New(Deps{Tracker: tr, Lease: ls, Space: ws, Runtime: rt, Host: hs}, Config{Width: 1, ReviewWidth: 1}, WithReviewInvokerFactory(func(entity.Harness) (worker.ReviewService, error) { return inv, nil }))
		if err != nil {
			t.Fatal(err)
		}
		tr.setFrontier(task("T1", 1), task("T2", 2))
		done := make(chan error, 1)
		go func() { _, err := d.Sweep(context.Background()); done <- err }()
		<-entered
		synctest.Wait() // all implementation work has progressed while review is held
		if rt.launchCount() != 2 {
			t.Fatalf("implementation launches=%d while review blocked, want 2", rt.launchCount())
		}
		release()
		if err := <-done; err != nil {
			t.Fatal(err)
		}

	})
}

func TestSweep_ReviewCapacityIsBoundedIndependently(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		tr, ls, ws, rt, hs := greenFakes(t)
		hs.checkByNum[1], hs.checkByNum[2], hs.checkByNum[3] = port.CheckGreen, port.CheckGreen, port.CheckGreen
		entered, unblock := make(chan struct{}, 3), make(chan struct{})
		release := sync.OnceFunc(func() { close(unblock) })
		defer release()
		inv := &recordingInvoker{entered: entered, block: unblock, result: worker.ReviewResult{
			Harness: "claude", ReviewerPrincipal: "review-bot", AttemptID: "attempt-1", ObligationID: "review-obligation-1",
			Phase: worker.ReviewPhaseArtifact, Status: report.ReviewStatusCompleted, ObligationStatus: report.ReviewObligationVerified,
			ArtifactRef: "github:7", ArtifactProofRef: "github:broker-receipt/7",
		}}
		d, err := New(Deps{Tracker: tr, Lease: ls, Space: ws, Runtime: rt, Host: hs}, Config{Width: 3, ReviewWidth: 2},
			WithReviewInvokerFactory(func(entity.Harness) (worker.ReviewService, error) { return inv, nil }))
		if err != nil {
			t.Fatal(err)
		}
		tr.setFrontier(task("T1", 1), task("T2", 2), task("T3", 3))
		done := make(chan error, 1)
		go func() { _, err := d.Sweep(context.Background()); done <- err }()
		for range 2 {
			<-entered
		}
		synctest.Wait() // the third admission is blocked, not merely unscheduled
		if rt.launchCount() != 3 {
			t.Fatalf("implementation launches=%d, want 3 before releasing review capacity", rt.launchCount())
		}
		select {
		case <-entered:
			t.Fatal("third reviewer exceeded review_width=2")
		default:
		}
		release()
		if err := <-done; err != nil {
			t.Fatal(err)
		}
		if inv.peak != 2 {
			t.Fatalf("review concurrency peak over the complete run=%d, want 2", inv.peak)
		}
		if inv.calls != 3 {
			t.Fatalf("review calls=%d, want 3 after capacity released", inv.calls)
		}

	})
}

func TestSweep_InheritedReviewPreflightsEachEffectiveBackendOnce(t *testing.T) {
	tr, ls, ws, _, hs := greenFakes(t)
	hs.checkByNum[1], hs.checkByNum[2], hs.checkByNum[3] = port.CheckGreen, port.CheckGreen, port.CheckGreen
	runtimes := map[entity.Harness]*fakeRuntime{entity.Codex: {}, entity.Claude: {}}
	resolver := func(task port.Task) (entity.Harness, port.AgentRuntime, error) {
		return task.Harness, runtimes[task.Harness], nil
	}
	builds := map[entity.Harness]int{}
	invokers := map[entity.Harness]*recordingInvoker{}
	factory := func(backend entity.Harness) (worker.ReviewService, error) {
		builds[backend]++
		inv := &recordingInvoker{backend: backend, result: worker.ReviewResult{
			ReviewerPrincipal: "review-bot", AttemptID: "attempt-" + backend.String(), ObligationID: "review-obligation-1",
			Phase: worker.ReviewPhaseArtifact, Status: report.ReviewStatusCompleted,
			ObligationStatus: report.ReviewObligationVerified, ArtifactRef: "github:7", ArtifactProofRef: "github:broker-receipt/7",
		}}
		invokers[backend] = inv
		return inv, nil
	}
	d, err := New(Deps{Tracker: tr, Lease: ls, Space: ws, Host: hs}, Config{Width: 3, ReviewWidth: 2},
		WithRuntimeResolver(resolver), WithReviewInvokerFactory(factory))
	if err != nil {
		t.Fatal(err)
	}
	a, b, c := task("TA", 1), task("TB", 2), task("TC", 3)
	a.Harness, b.Harness, c.Harness = "codex", "claude", "codex"
	tr.setFrontier(a, b, c)
	rep, err := d.Sweep(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if rep.Succeeded != 3 || rep.ReviewCompleted != 3 {
		t.Fatalf("counts = %+v, want three implementation and review successes", rep)
	}
	if builds["codex"] != 1 || builds["claude"] != 1 || len(builds) != 2 {
		t.Fatalf("review factory builds = %v, want one per distinct inherited backend", builds)
	}
	if invokers["codex"].preflightCalls != 1 || invokers["claude"].preflightCalls != 1 {
		t.Fatalf("review preflights = codex:%d claude:%d, want 1/1", invokers["codex"].preflightCalls, invokers["claude"].preflightCalls)
	}
	if invokers["codex"].calls != 2 || invokers["claude"].calls != 1 {
		t.Fatalf("review invokes = codex:%d claude:%d, want 2/1", invokers["codex"].calls, invokers["claude"].calls)
	}
}

func TestSweep_FailedReviewReddensRunAndEscalatesWithoutReclassifyingTask(t *testing.T) {
	tr, ls, ws, rt, hs := greenFakes(t)
	hs.checkByNum[1] = port.CheckGreen
	events := &captureEmitter{}
	inv := &recordingInvoker{result: worker.ReviewResult{
		Harness: "claude", ProposalRef: "github:o/r#1", Revision: "abc123",
		Phase: worker.ReviewPhaseArtifact, Status: report.ReviewStatusFailed, ObligationStatus: report.ReviewObligationPending,
		FailureClass: worker.ReviewFailureArtifactUnverified, Err: errors.New("artifact absent"),
	}}
	d, err := New(Deps{Tracker: tr, Lease: ls, Space: ws, Runtime: rt, Host: hs}, Config{Width: 1},
		WithReporter(events),
		WithReviewInvokerFactory(func(entity.Harness) (worker.ReviewService, error) { return inv, nil }))
	if err != nil {
		t.Fatal(err)
	}
	tr.setFrontier(task("T1", 1))
	rep, err := d.Sweep(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if rep.Succeeded != 1 || rep.Failed != 0 || rep.ReviewFailed != 1 || rep.ReviewCompleted != 0 {
		t.Fatalf("counts = %+v, want succeeded implementation plus failed review", rep)
	}
	outcome, ok := events.find(report.KindTaskOutcome, "T1")
	if !ok || outcome.Class != ClassSucceeded.String() || outcome.Escalated {
		t.Fatalf("implementation outcome was changed by review: %+v", outcome)
	}
	reviewTerminal, ok := events.find(report.KindImplReviewTerminal, "T1")
	if !ok || reviewTerminal.Status != report.ReviewStatusFailed || !reviewTerminal.Escalated || !report.IsEscalation(reviewTerminal) {
		t.Fatalf("review failure terminal = %+v", reviewTerminal)
	}
	finished, ok := events.find(report.KindRunFinished, "")
	if !ok || finished.Counts == nil || finished.Counts.ReviewFailed != 1 {
		t.Fatalf("run_finished = %+v, want review_failed=1", finished)
	}
}

func TestSweep_ObligationPersistenceFailurePreservesGreenImplementationTerminal(t *testing.T) {
	tr, ls, ws, rt, hs := greenFakes(t)
	hs.checkByNum[1] = port.CheckGreen
	events := &captureEmitter{}
	inv := &recordingInvoker{captureResult: worker.ReviewCapture{
		Harness: "claude", ProposalRef: "github:o/r#1", Revision: "abc123", ObligationID: "review-obligation-1",
		Err: errors.New("canonical obligation store unavailable"),
	}}
	d, err := New(Deps{Tracker: tr, Lease: ls, Space: ws, Runtime: rt, Host: hs}, Config{Width: 1},
		WithReporter(events), WithReviewHarness("claude"),
		WithReviewInvokerFactory(func(entity.Harness) (worker.ReviewService, error) { return inv, nil }))
	if err != nil {
		t.Fatal(err)
	}
	tr.setFrontier(task("T1", 1))
	rep, err := d.Sweep(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if rep.Succeeded != 1 || rep.Failed != 0 || rep.ReviewFailed != 1 || rep.ReviewInfrastructureFailed != 1 || rep.ReviewPending != 0 {
		t.Fatalf("counts = %+v, want green implementation truth plus not-created review failure", rep)
	}
	if inv.calls != 0 || ls.releases.Load() != 1 {
		t.Fatalf("review calls=%d claim releases=%d, want 0/1", inv.calls, ls.releases.Load())
	}
	if events.countKind(report.KindTaskVerified) != 1 || events.countKind(report.KindTaskOutcome) != 1 {
		t.Fatalf("implementation evidence = %+v, want one green verification and succeeded terminal", events.snapshot())
	}
	terminal, ok := events.find(report.KindImplReviewTerminal, "T1")
	if !ok || terminal.ObligationStatus != report.ReviewObligationNotCreated || terminal.FailureClass != worker.ReviewFailureInfrastructure || !terminal.Escalated {
		t.Fatalf("review terminal = %+v", terminal)
	}
}

func TestSweep_HandoffMarkerFailurePreservesSucceededTerminalAndPendingObligation(t *testing.T) {
	tr, ls, ws, rt, hs := greenFakes(t)
	hs.checkByNum[1] = port.CheckGreen
	events := &captureEmitter{}
	inv := &recordingInvoker{events: events, lease: ls, handoffErr: errors.New("canonical handoff marker unavailable")}
	d, err := New(Deps{Tracker: tr, Lease: ls, Space: ws, Runtime: rt, Host: hs}, Config{Width: 1},
		WithReporter(events), WithReviewHarness("claude"),
		WithReviewInvokerFactory(func(entity.Harness) (worker.ReviewService, error) { return inv, nil }))
	if err != nil {
		t.Fatal(err)
	}
	tr.setFrontier(task("T1", 1))
	rep, err := d.Sweep(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if rep.Succeeded != 1 || rep.ReviewFailed != 1 || rep.ReviewPending != 1 || rep.ReviewInfrastructureFailed != 1 {
		t.Fatalf("counts = %+v", rep)
	}
	if inv.calls != 0 || inv.handoffs != 1 || !inv.handoffSawOutcome || !inv.handoffWhileHeld {
		t.Fatalf("calls=%d handoffs=%d saw-outcome=%v while-held=%v", inv.calls, inv.handoffs, inv.handoffSawOutcome, inv.handoffWhileHeld)
	}
	terminal, ok := events.find(report.KindImplReviewTerminal, "T1")
	if !ok || terminal.ObligationStatus != report.ReviewObligationPending || terminal.FailureClass != worker.ReviewFailureInfrastructure {
		t.Fatalf("review terminal = %+v", terminal)
	}
	if events.countKind(report.KindTaskVerified) != 1 || events.countKind(report.KindTaskOutcome) != 1 || ls.releases.Load() != 1 {
		t.Fatalf("task verified=%d outcomes=%d releases=%d", events.countKind(report.KindTaskVerified), events.countKind(report.KindTaskOutcome), ls.releases.Load())
	}
}

func TestSweep_ConfiguredReviewFactoryFailureIsNotSilentlyDropped(t *testing.T) {
	tr, ls, ws, rt, hs := greenFakes(t)
	events := &captureEmitter{}
	d, err := New(Deps{Tracker: tr, Lease: ls, Space: ws, Runtime: rt, Host: hs}, Config{Width: 1},
		WithReporter(events),
		WithReviewInvokerFactory(func(entity.Harness) (worker.ReviewService, error) {
			return nil, errors.New("review adapter construction failed")
		}))
	if err != nil {
		t.Fatal(err)
	}
	tr.setFrontier(task("T1", 1))
	rep, err := d.Sweep(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if rep.Failed != 0 || rep.Succeeded != 0 || rep.ReviewCompleted != 0 || rep.ReviewFailed != 1 || rep.ReviewInfrastructureFailed != 1 {
		t.Fatalf("counts = %+v, want one run-level review infrastructure failure", rep)
	}
	if events.countKind(report.KindTaskDispatched) != 0 || events.countKind(report.KindTaskOutcome) != 0 || events.countKind(report.KindImplReviewTerminal) != 1 {
		t.Fatalf("events = %+v, want one review terminal and no task terminal/dispatch", events.snapshot())
	}
	terminal, ok := events.find(report.KindImplReviewTerminal, "")
	if !ok || terminal.Phase != worker.ReviewPhasePreflight || !strings.Contains(terminal.Reason, "review adapter construction failed") {
		t.Fatalf("terminal = %+v, want named review construction failure", terminal)
	}
}

func TestSweep_ReviewAuthFailureIsRunLevelAndSpawnsNoTask(t *testing.T) {
	tr, ls, ws, rt, hs := greenFakes(t)
	events := &captureEmitter{}
	inv := &recordingInvoker{backend: "claude", preflightErr: errors.New("reviewer login missing")}
	d, err := New(Deps{Tracker: tr, Lease: ls, Space: ws, Runtime: rt, Host: hs}, Config{Width: 1},
		WithReporter(events), WithReviewHarness("claude"),
		WithReviewInvokerFactory(func(entity.Harness) (worker.ReviewService, error) { return inv, nil }))
	if err != nil {
		t.Fatal(err)
	}
	tr.setFrontier(task("T1", 1))
	rep, err := d.Sweep(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if rep.Failed != 0 || rep.ReviewFailed != 1 || rep.ReviewInfrastructureFailed != 1 {
		t.Fatalf("counts = %+v, want one run-level review infrastructure failure", rep)
	}
	if inv.preflightCalls != 1 || inv.calls != 0 || rt.launchCount() != 0 || ls.acquiredCount() != 0 {
		t.Fatalf("preflight=%d review=%d implementation=%d claims=%d, want 1/0/0/0", inv.preflightCalls, inv.calls, rt.launchCount(), ls.acquiredCount())
	}
	terminal, ok := events.find(report.KindImplReviewTerminal, "")
	if !ok || terminal.Phase != worker.ReviewPhasePreflight || terminal.ObligationStatus != report.ReviewObligationNotCreated || terminal.FailureClass != worker.ReviewFailureInfrastructure {
		t.Fatalf("review auth terminal = %+v", terminal)
	}
	if events.countKind(report.KindTaskDispatched) != 0 || events.countKind(report.KindTaskOutcome) != 0 {
		t.Fatalf("auth failure emitted task lifecycle: %+v", events.snapshot())
	}
}

func TestSweep_HandsEveryWorkerTheDescribedCheckpointObserver(t *testing.T) {
	tr, ls, ws, rt, hs := greenFakes(t)
	var mu sync.Mutex
	observed := map[string]string{}
	d, err := New(Deps{Tracker: tr, Lease: ls, Space: ws, Runtime: rt, Host: hs}, Config{Width: 2},
		WithDescribedCheckpointObserver(func(_ context.Context, task port.Task, _ port.Workdir, description prdescription.Description) {
			mu.Lock()
			defer mu.Unlock()
			observed[task.ID] = description.What
		}), WithDescribedCheckpointObserver(nil))
	if err != nil {
		t.Fatal(err)
	}
	tr.setFrontier(task("T1", 1), task("T2", 2))
	if rep, err := d.Sweep(context.Background()); err != nil || rep.Succeeded != 2 {
		t.Fatalf("sweep: %+v %v", rep, err)
	}
	if len(observed) != 2 || observed["T1"] != "- Implement the task." || observed["T2"] != "- Implement the task." {
		t.Fatalf("observed checkpoints %v", observed)
	}
}
