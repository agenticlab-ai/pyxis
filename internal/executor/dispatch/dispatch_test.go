package dispatch

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/changesize"
	"github.com/agenticlab-ai/pyxis/internal/executor/port"
	"github.com/agenticlab-ai/pyxis/internal/executor/worker"
	"github.com/agenticlab-ai/pyxis/internal/prdescription"
	"github.com/agenticlab-ai/pyxis/internal/prdescription/prdescriptiontest"
)

// The fakes implement all five ports in-process — no network, git, or CLI — and
// feed REAL Workers, so a Sweep exercises the whole frontier→claim→run→verify
// path hermetically (docs/tech-design/implement-now.md §3.10, the substrate-first
// harness). Compile-time assertions pin them to the port contracts; the fakes are
// concurrency-safe because a sweep runs many Workers against one set of ports.
var (
	_ port.WorkTracker  = (*fakeTracker)(nil)
	_ port.Lease        = (*fakeLease)(nil)
	_ port.Claim        = (*fakeClaim)(nil)
	_ port.Workspace    = (*fakeWorkspace)(nil)
	_ port.Workdir      = (*fakeWorkdir)(nil)
	_ port.AgentRuntime = (*fakeRuntime)(nil)
	_ port.Handle       = fakeHandle{}
	_ port.CodeHost     = (*fakeHost)(nil)
	_ port.Lease        = leaseFunc(nil)
	_ port.Claim        = noopClaim{}
)

// --- fake WorkTracker ---------------------------------------------------------

type fakeTracker struct {
	mu          sync.Mutex
	frontier    []port.Task
	comments    []trackerComment
	frontierErr error
}

type trackerComment struct {
	issue int
	body  string
}

func (f *fakeTracker) setFrontier(tasks ...port.Task) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.frontier = tasks
}

func (f *fakeTracker) Frontier(context.Context) ([]port.Task, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.frontierErr != nil {
		return nil, f.frontierErr
	}
	out := make([]port.Task, len(f.frontier))
	copy(out, f.frontier)
	return out, nil
}

func (f *fakeTracker) Comment(_ context.Context, issue int, body string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.comments = append(f.comments, trackerComment{issue: issue, body: body})
	return nil
}

func (f *fakeTracker) commentsSnapshot() []trackerComment {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]trackerComment, len(f.comments))
	copy(out, f.comments)
	return out
}

// --- fake Lease / Claim -------------------------------------------------------

type fakeLease struct {
	mu       sync.Mutex
	claimErr map[int]error // per task.Number: e.g. port.ErrClaimLost -> a skip
	acquired []int         // task numbers Acquire was asked for
	releases atomic.Int32  // Release call count (atomic)

	concurrent  atomic.Int32 // live Acquire callers inside inner work (atomic)
	maxAcquire  int32        // peak of concurrent (atomic)
	acquireHold time.Duration
}

func (l *fakeLease) Acquire(_ context.Context, task port.Task) (port.Claim, error) {
	cur := l.concurrent.Add(1)
	recordMax(&l.maxAcquire, cur)
	if l.acquireHold > 0 {
		// A widened window so a non-serialized claim path would be observed to
		// overlap; the serialLease guard keeps maxAcquire at 1 regardless.
		time.Sleep(l.acquireHold)
	}
	l.concurrent.Add(-1)

	l.mu.Lock()
	l.acquired = append(l.acquired, task.Number)
	err := l.claimErr[task.Number]
	l.mu.Unlock()
	if err != nil {
		return nil, err
	}
	return &fakeClaim{lease: l}, nil
}

func (l *fakeLease) acquiredCount() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.acquired)
}

type fakeClaim struct{ lease *fakeLease }

func (c *fakeClaim) Release(context.Context) error {
	c.lease.releases.Add(1)
	return nil
}

// leaseFunc adapts a function to port.Lease, for the serialLease unit test.
type leaseFunc func(context.Context, port.Task) (port.Claim, error)

func (f leaseFunc) Acquire(ctx context.Context, t port.Task) (port.Claim, error) { return f(ctx, t) }

type noopClaim struct{}

func (noopClaim) Release(context.Context) error { return nil }

// --- fake Workspace / Workdir -------------------------------------------------

type fakeWorkspace struct {
	mu       sync.Mutex
	dir      string
	prepared []int
}

func (w *fakeWorkspace) Prepare(_ context.Context, task port.Task) (port.Workdir, error) {
	w.mu.Lock()
	w.prepared = append(w.prepared, task.Number)
	w.mu.Unlock()
	// Model the real workspace's per-task isolation, including across retries.
	dir := filepath.Join(w.dir, fmt.Sprint(task.Number))
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	return &fakeWorkdir{dir: dir, branch: task.HeadBranch}, nil
}

func (w *fakeWorkspace) preparedCount() int {
	w.mu.Lock()
	defer w.mu.Unlock()
	return len(w.prepared)
}

type fakeWorkdir struct {
	dir    string
	branch string
}

func (w *fakeWorkdir) Dir() string                              { return w.dir }
func (w *fakeWorkdir) Branch() string                           { return w.branch }
func (w *fakeWorkdir) Checkpoint(context.Context) (bool, error) { return true, nil }
func (w *fakeWorkdir) Push(context.Context) (bool, error)       { return true, nil }
func (w *fakeWorkdir) Cleanup(context.Context) error            { return nil }

// --- fake AgentRuntime / Handle -----------------------------------------------

type fakeRuntime struct {
	launches    atomic.Int32 // Launch call count == model calls (atomic)
	inFlight    atomic.Int32 // workers between Launch and Await return (atomic)
	maxInFlight int32        // peak of inFlight (atomic)
	await       func()       // optional barrier invoked inside Await

	mu    sync.Mutex
	specs []port.LaunchSpec
}

func (r *fakeRuntime) AuthCheck(context.Context) error { return nil }

func (r *fakeRuntime) ModelRoute() port.ModelRoute {
	return port.ModelRoute{Model: "fake-model", Effort: "fake-effort"}
}

func (r *fakeRuntime) Launch(_ context.Context, spec port.LaunchSpec) (port.Handle, error) {
	r.launches.Add(1)
	cur := r.inFlight.Add(1)
	recordMax(&r.maxInFlight, cur)
	r.mu.Lock()
	r.specs = append(r.specs, spec)
	r.mu.Unlock()
	return fakeHandle{}, nil
}

func (r *fakeRuntime) Await(context.Context, port.Handle) (port.RunResult, error) {
	if r.await != nil {
		r.await()
	}
	r.inFlight.Add(-1)
	return port.RunResult{Tokens: 1, Turns: 1, Output: prdescriptiontest.Message}, nil
}

func (r *fakeRuntime) Kill(context.Context, port.Handle) error { return nil }

func (r *fakeRuntime) launchCount() int { return int(r.launches.Load()) }

func (r *fakeRuntime) specsSnapshot() []port.LaunchSpec {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]port.LaunchSpec, len(r.specs))
	copy(out, r.specs)
	return out
}

type fakeHandle struct{}

func (fakeHandle) RuntimeHandle() {}

// --- fake CodeHost ------------------------------------------------------------

type fakeHost struct {
	mu           sync.Mutex
	checkByNum   map[int]port.CheckState // per task.Number; default CheckGreen
	openErrByNum map[int]error           // per task.Number; e.g. port.ErrNoProposal
}

func newFakeHost() *fakeHost {
	return &fakeHost{checkByNum: map[int]port.CheckState{}, openErrByNum: map[int]error{}}
}

func (h *fakeHost) OpenProposal(_ context.Context, task port.Task, work port.Workdir, _ prdescription.Description) (port.Proposal, error) {
	h.mu.Lock()
	err := h.openErrByNum[task.Number]
	h.mu.Unlock()
	if err != nil {
		return port.Proposal{}, err
	}
	return port.Proposal{Number: task.Number, Branch: work.Branch()}, nil
}

func (h *fakeHost) Checks(_ context.Context, p port.Proposal) (port.CheckState, error) {
	h.mu.Lock()
	st, ok := h.checkByNum[p.Number]
	h.mu.Unlock()
	if !ok {
		return port.CheckGreen, nil
	}
	return st, nil
}

// --- helpers ------------------------------------------------------------------

// recordMax raises *addr to cur if cur is larger, atomically.
func recordMax(addr *int32, cur int32) {
	for {
		old := atomic.LoadInt32(addr)
		if cur <= old || atomic.CompareAndSwapInt32(addr, old, cur) {
			return
		}
	}
}

// task builds a green-by-default port.Task with the given id, tracker number, and
// blocker issue numbers.
func task(id string, number int, blockedBy ...int) port.Task {
	return port.Task{
		ID:     id,
		Number: number,
		Title:  "task " + id,
		Brief:  "charter for " + id,

		Harness:    "claude",
		HeadBranch: fmt.Sprintf("feat/%s-%d", id, number),
		BlockedBy:  blockedBy,
	}
}

// greenFakes wires a set of fakes that drive every task to a green proposal.
func greenFakes(t *testing.T) (*fakeTracker, *fakeLease, *fakeWorkspace, *fakeRuntime, *fakeHost) {
	t.Helper()
	auth := t.TempDir()
	t.Setenv("CLAUDE_CONFIG_DIR", auth)
	t.Setenv("CODEX_HOME", auth)
	if err := os.WriteFile(filepath.Join(auth, ".credentials.json"), []byte("test-credential"), 0o600); err != nil {
		t.Fatal(err)
	}
	return &fakeTracker{},
		&fakeLease{claimErr: map[int]error{}},
		&fakeWorkspace{dir: t.TempDir()},
		&fakeRuntime{},
		newFakeHost()
}

func mustDispatcher(t *testing.T, width int, tr *fakeTracker, ls *fakeLease, ws *fakeWorkspace, rt *fakeRuntime, hs *fakeHost) *Dispatcher {
	t.Helper()
	d, err := New(Deps{Tracker: tr, Lease: ls, Space: ws, Runtime: rt, Host: hs}, Config{Width: width})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return d
}

// classOf returns the reported class of the task with the given tracker number.
func classOf(t *testing.T, rep RunReport, number int) Class {
	t.Helper()
	for _, r := range rep.Results {
		if r.Task.Number == number {
			return r.Class
		}
	}
	t.Fatalf("task #%d absent from the report; the sweep must class every task", number)
	return ClassBlocked
}

// --- acceptance criterion 1: foundation, then both leaves, 3 succeeded --------

// A foundation with two file-disjoint leaves runs the foundation first (the
// leaves are gated behind it), and once the driver lands the foundation the next
// sweep runs both leaves concurrently — three succeeded across the run. This is
// the frontier-sweep + driver-reinvoke model: dependents are reported blocked,
// not waited on, until their blocker leaves the open set (§3.6, DINV-2).
func TestSweep_FoundationThenLeaves(t *testing.T) {
	tr, ls, ws, rt, hs := greenFakes(t)
	d := mustDispatcher(t, 2, tr, ls, ws, rt, hs)

	foundation := task("T1", 1) // no blockers
	leafA := task("T2", 2, 1)   // blocked by the foundation (#1)
	leafB := task("T3", 3, 1)   // blocked by the foundation (#1)

	// Sweep 1: the tracker still shows the foundation open, so both leaves are
	// gated. Only the foundation is ready.
	tr.setFrontier(foundation, leafA, leafB)
	rep1, err := d.Sweep(context.Background())
	if err != nil {
		t.Fatalf("sweep 1: %v", err)
	}
	if rep1.Succeeded != 1 || rep1.Blocked != 2 || rep1.Failed != 0 || rep1.Skipped != 0 {
		t.Fatalf("sweep 1 counts = %+v, want 1 succeeded / 2 blocked", rep1)
	}
	if c := classOf(t, rep1, 1); c != ClassSucceeded {
		t.Errorf("foundation class = %v, want succeeded", c)
	}
	if c := classOf(t, rep1, 2); c != ClassBlocked {
		t.Errorf("leaf A class = %v, want blocked", c)
	}
	if c := classOf(t, rep1, 3); c != ClassBlocked {
		t.Errorf("leaf B class = %v, want blocked", c)
	}
	// The foundation ran; the gated leaves made no model call yet — proof the
	// foundation runs strictly before the leaves.
	if got := rt.launchCount(); got != 1 {
		t.Fatalf("model calls after sweep 1 = %d, want 1 (foundation only)", got)
	}

	// The driver lands the foundation: it leaves the open frontier. Re-invoke.
	tr.setFrontier(leafA, leafB)
	rep2, err := d.Sweep(context.Background())
	if err != nil {
		t.Fatalf("sweep 2: %v", err)
	}
	if rep2.Succeeded != 2 || rep2.Blocked != 0 || rep2.Failed != 0 || rep2.Skipped != 0 {
		t.Fatalf("sweep 2 counts = %+v, want 2 succeeded", rep2)
	}
	if c := classOf(t, rep2, 2); c != ClassSucceeded {
		t.Errorf("leaf A class = %v, want succeeded", c)
	}
	if c := classOf(t, rep2, 3); c != ClassSucceeded {
		t.Errorf("leaf B class = %v, want succeeded", c)
	}

	// Across the run: three tasks succeeded, and every task ran exactly once.
	if total := rep1.Succeeded + rep2.Succeeded; total != 3 {
		t.Fatalf("total succeeded across the run = %d, want 3", total)
	}
	if got := rt.launchCount(); got != 3 {
		t.Fatalf("total model calls = %d, want 3 (one per task)", got)
	}
	if got := ws.preparedCount(); got != 3 {
		t.Errorf("workspaces prepared = %d, want 3", got)
	}
	if got := ls.releases.Load(); got != 3 {
		t.Errorf("claims released = %d, want 3 (fire-or-release, DINV-5)", got)
	}

	// Port-wiring: the Dispatcher handed each Worker the runtime, so each task's
	// own charter and its clamped (model, effort) reached Launch (AP-21).
	specs := rt.specsSnapshot()
	if len(specs) != 3 {
		t.Fatalf("launch specs = %d, want 3", len(specs))
	}
	for _, s := range specs {
		if s.Brief == "" || s.Model != "fake-model" || s.Effort != "fake-effort" {
			t.Errorf("launch spec = %+v, want a non-empty brief and the clamped model/effort", s)
		}
	}
	// Port-wiring: the Dispatcher handed each Worker the tracker, so each opened
	// proposal posted its progress comment on its own issue (#1, #2, #3).
	comments := tr.commentsSnapshot()
	if len(comments) != 3 {
		t.Fatalf("progress comments = %d, want one per task", len(comments))
	}
	seen := map[int]bool{}
	for _, c := range comments {
		if c.body == "" {
			t.Errorf("comment on issue #%d had an empty body", c.issue)
		}
		seen[c.issue] = true
	}
	for _, n := range []int{1, 2, 3} {
		if !seen[n] {
			t.Errorf("no progress comment on issue #%d", n)
		}
	}
}

// --- acceptance criterion 2: empty frontier, zero model calls -----------------

// A sweep over an empty frontier spawns no Worker and makes zero model calls,
// reporting nothing-ready (DINV-8, AP-19, prd:implement-now/FR-12).
func TestSweep_EmptyFrontierIsZeroCost(t *testing.T) {
	tr, ls, ws, rt, hs := greenFakes(t)
	d := mustDispatcher(t, 4, tr, ls, ws, rt, hs)
	// tr.frontier is empty by default.

	rep, err := d.Sweep(context.Background())
	if err != nil {
		t.Fatalf("sweep: %v", err)
	}
	if !rep.NothingReady() {
		t.Fatalf("report = %+v, want NothingReady", rep)
	}
	if got := rt.launchCount(); got != 0 {
		t.Fatalf("model calls = %d, want 0 on an empty frontier (DINV-8)", got)
	}
	if len(rep.Results) != 0 {
		t.Errorf("results = %d, want 0", len(rep.Results))
	}
	if ls.acquiredCount() != 0 {
		t.Errorf("claims acquired = %d, want 0 (nothing claimed before a ready gate)", ls.acquiredCount())
	}
	if ws.preparedCount() != 0 {
		t.Errorf("workspaces prepared = %d, want 0", ws.preparedCount())
	}
}

// A frontier of only-blocked candidates is also nothing-ready and zero-cost: the
// tracker returned open items, but every one is gated behind an open blocker, so
// no Worker runs. The blocked tasks are still reported for the driver.
func TestSweep_AllBlockedIsNothingReadyButReported(t *testing.T) {
	tr, ls, ws, rt, hs := greenFakes(t)
	d := mustDispatcher(t, 4, tr, ls, ws, rt, hs)

	// Every candidate is gated behind another still-open candidate, so none is
	// ready: #1 waits on #2, #2 on #3, #3 on #1.
	tr.setFrontier(task("TA", 1, 2), task("TB", 2, 3), task("TC", 3, 1))

	rep, err := d.Sweep(context.Background())
	if err != nil {
		t.Fatalf("sweep: %v", err)
	}
	if !rep.NothingReady() {
		t.Fatalf("report = %+v, want NothingReady when every candidate is gated", rep)
	}
	if rep.Blocked != 3 {
		t.Errorf("blocked = %d, want 3", rep.Blocked)
	}
	if got := rt.launchCount(); got != 0 {
		t.Errorf("model calls = %d, want 0 (no candidate was ready)", got)
	}
}

// --- acceptance criterion 3: one failing leaf never halts the run -------------

// Two independent leaves run concurrently; one comes back not-green (failed) and
// the other still completes. Every task is classed — one failure never halts the
// sweep (prd:implement-now/FR-15, AP-16).
func TestSweep_OneFailingLeafDoesNotHaltRun(t *testing.T) {
	tr, ls, ws, rt, hs := greenFakes(t)
	hs.checkByNum[2] = port.CheckRed // leaf A's proposal is not green -> failed
	d := mustDispatcher(t, 2, tr, ls, ws, rt, hs)

	leafA := task("T2", 2) // independent
	leafB := task("T3", 3) // independent
	tr.setFrontier(leafA, leafB)

	rep, err := d.Sweep(context.Background())
	if err != nil {
		t.Fatalf("sweep: %v", err)
	}
	if rep.Succeeded != 1 || rep.Failed != 1 || rep.Blocked != 0 || rep.Skipped != 0 {
		t.Fatalf("counts = %+v, want 1 succeeded / 1 failed", rep)
	}
	if c := classOf(t, rep, 2); c != ClassFailed {
		t.Errorf("failing leaf class = %v, want failed", c)
	}
	if c := classOf(t, rep, 3); c != ClassSucceeded {
		t.Errorf("independent leaf class = %v, want succeeded", c)
	}
	if len(rep.Results) != 2 {
		t.Errorf("results = %d, want 2 (every task classed)", len(rep.Results))
	}
	// Both leaves ran: the failure of one did not skip the other.
	if got := rt.launchCount(); got != 2 {
		t.Errorf("model calls = %d, want 2 (both leaves ran)", got)
	}
}

// --- supporting: a lost claim is a skip, not a failure ------------------------

// A task whose claim is already held (port.ErrClaimLost) is classed skipped, not
// failed; an independent sibling still succeeds (DINV-5, AP-22).
func TestSweep_ClaimLostIsSkipped(t *testing.T) {
	tr, ls, ws, rt, hs := greenFakes(t)
	ls.claimErr[2] = port.ErrClaimLost // leaf A is held elsewhere
	d := mustDispatcher(t, 2, tr, ls, ws, rt, hs)

	tr.setFrontier(task("T2", 2), task("T3", 3))
	rep, err := d.Sweep(context.Background())
	if err != nil {
		t.Fatalf("sweep: %v", err)
	}
	if rep.Skipped != 1 || rep.Succeeded != 1 || rep.Failed != 0 {
		t.Fatalf("counts = %+v, want 1 skipped / 1 succeeded", rep)
	}
	if c := classOf(t, rep, 2); c != ClassSkipped {
		t.Errorf("held task class = %v, want skipped", c)
	}
	if c := classOf(t, rep, 3); c != ClassSucceeded {
		t.Errorf("sibling class = %v, want succeeded", c)
	}
	// The skipped task consumed no model call — the losing racer no-ops before
	// launching an agent.
	if got := rt.launchCount(); got != 1 {
		t.Errorf("model calls = %d, want 1 (skipped task launches nothing)", got)
	}
}

// A hollow run (clean exit, no proposal) is failed in the report, never
// succeeded, but the hollow outcome stays visible on the result (DINV-3).
func TestSweep_HollowIsFailedButVisible(t *testing.T) {
	tr, ls, ws, rt, hs := greenFakes(t)
	hs.openErrByNum[5] = port.ErrNoProposal
	d := mustDispatcher(t, 2, tr, ls, ws, rt, hs)

	tr.setFrontier(task("T5", 5))
	rep, err := d.Sweep(context.Background())
	if err != nil {
		t.Fatalf("sweep: %v", err)
	}
	if rep.Failed != 1 || rep.Succeeded != 0 {
		t.Fatalf("counts = %+v, want 1 failed", rep)
	}
	r := rep.Results[0]
	if r.Class != ClassFailed {
		t.Errorf("class = %v, want failed", r.Class)
	}
	if r.Outcome != worker.OutcomeHollow {
		t.Errorf("outcome = %v, want hollow preserved on the result", r.Outcome)
	}
}

// --- supporting: concurrency is bounded by the width -------------------------

// With three ready tasks and a width of two, at most two Workers run at once. The
// bound is proved without a clock: the runtime holds each Worker in Await until
// the test releases it, so exactly `width` Workers pile up while the third is
// held out by the semaphore.
func TestSweep_ConcurrencyBoundedByWidth(t *testing.T) {
	const width = 2
	const tasks = 3
	tr, ls, ws, rt, hs := greenFakes(t)

	arrived := make(chan struct{}, tasks) // buffered so a late arrival never blocks
	release := make(chan struct{})
	rt.await = func() {
		arrived <- struct{}{}
		<-release
	}
	d := mustDispatcher(t, width, tr, ls, ws, rt, hs)
	tr.setFrontier(task("T1", 1), task("T2", 2), task("T3", 3))

	done := make(chan RunReport, 1)
	go func() {
		rep, err := d.Sweep(context.Background())
		if err != nil {
			t.Errorf("sweep: %v", err)
		}
		done <- rep
	}()

	// Wait until `width` Workers are simultaneously in Await (holding their
	// slots). The third cannot Launch until a slot frees.
	for range width {
		<-arrived
	}
	if got := rt.inFlight.Load(); got != width {
		t.Errorf("in-flight while held = %d, want %d", got, width)
	}
	close(release) // let the held Workers finish; the third now runs

	rep := <-done
	if got := atomic.LoadInt32(&rt.maxInFlight); got != width {
		t.Fatalf("peak concurrency = %d, want %d (the width bound)", got, width)
	}
	if rep.Succeeded != tasks {
		t.Fatalf("succeeded = %d, want %d", rep.Succeeded, tasks)
	}
}

// --- supporting: claims are serialized ---------------------------------------

// The serialLease makes a Worker's claim step mutually exclusive across a sweep,
// so two Workers never race a claim before the underlying compare-and-swap (§3.6
// step 2). A widened window would let a non-serialized path overlap; the guard
// keeps the peak at one.
func TestSerialLease_AcquireIsMutuallyExclusive(t *testing.T) {
	var (
		concurrent atomic.Int32
		maxSeen    int32
	)
	inner := leaseFunc(func(context.Context, port.Task) (port.Claim, error) {
		cur := concurrent.Add(1)
		recordMax(&maxSeen, cur)
		time.Sleep(5 * time.Millisecond) // widen the window; the guard still serializes
		concurrent.Add(-1)
		return noopClaim{}, nil
	})
	sl := &serialLease{inner: inner}

	var wg sync.WaitGroup
	for range 4 {
		wg.Go(func() {
			c, err := sl.Acquire(context.Background(), port.Task{})
			if err != nil {
				t.Errorf("Acquire: %v", err)
				return
			}
			_ = c.Release(context.Background())
		})
	}
	wg.Wait()
	if maxSeen != 1 {
		t.Fatalf("peak concurrent Acquire = %d, want 1 (serialLease must serialize claims)", maxSeen)
	}
}

// The fake lease, wrapped by the Dispatcher's serialLease, sees claims one at a
// time even while Workers run concurrently — the "claim serially, run
// concurrently" contract end to end.
func TestSweep_ClaimsAreSerialWhileWorkersConcurrent(t *testing.T) {
	tr, ls, ws, rt, hs := greenFakes(t)
	ls.acquireHold = 2 * time.Millisecond // widen the claim window
	d := mustDispatcher(t, 4, tr, ls, ws, rt, hs)
	tr.setFrontier(task("T1", 1), task("T2", 2), task("T3", 3), task("T4", 4))

	if _, err := d.Sweep(context.Background()); err != nil {
		t.Fatalf("sweep: %v", err)
	}
	if got := atomic.LoadInt32(&ls.maxAcquire); got != 1 {
		t.Fatalf("peak concurrent claim = %d, want 1 (claims are serialized)", got)
	}
}

// --- supporting: a frontier read failure errors the sweep --------------------

func TestSweep_FrontierErrorIsReturned(t *testing.T) {
	tr, ls, ws, rt, hs := greenFakes(t)
	tr.frontierErr = errors.New("tracker unreachable")
	d := mustDispatcher(t, 2, tr, ls, ws, rt, hs)

	_, err := d.Sweep(context.Background())
	if err == nil {
		t.Fatal("want an error when the frontier read fails")
	}
	if got := rt.launchCount(); got != 0 {
		t.Errorf("model calls = %d, want 0 when the frontier could not be read", got)
	}
}

// --- supporting: construction and config -------------------------------------

func TestNew_RejectsNilPorts(t *testing.T) {
	full := func() Deps {
		return Deps{
			Tracker: &fakeTracker{},
			Lease:   &fakeLease{},
			Space:   &fakeWorkspace{},
			Runtime: &fakeRuntime{},
			Host:    &fakeHost{},
		}
	}
	tests := []struct {
		name string
		mut  func(*Deps)
	}{
		{"nil tracker", func(d *Deps) { d.Tracker = nil }},
		{"nil lease", func(d *Deps) { d.Lease = nil }},
		{"nil space", func(d *Deps) { d.Space = nil }},
		{"nil runtime", func(d *Deps) { d.Runtime = nil }},
		{"nil host", func(d *Deps) { d.Host = nil }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			deps := full()
			tt.mut(&deps)
			if _, err := New(deps, Config{}); err == nil {
				t.Fatalf("New(%s): want an error", tt.name)
			}
		})
	}
	if _, err := New(full(), Config{}); err != nil {
		t.Fatalf("New(full): unexpected error: %v", err)
	}
}

func TestNew_WidthDefaultsWhenUnset(t *testing.T) {
	full := Deps{
		Tracker: &fakeTracker{}, Lease: &fakeLease{}, Space: &fakeWorkspace{},
		Runtime: &fakeRuntime{}, Host: &fakeHost{},
	}
	for _, w := range []int{0, -3} {
		d, err := New(full, Config{Width: w})
		if err != nil {
			t.Fatalf("New(width=%d): %v", w, err)
		}
		if d.width != DefaultWidth {
			t.Errorf("width for cfg %d = %d, want default %d", w, d.width, DefaultWidth)
		}
	}
	d, err := New(full, Config{Width: 7})
	if err != nil {
		t.Fatalf("New(width=7): %v", err)
	}
	if d.width != 7 {
		t.Errorf("width = %d, want 7", d.width)
	}
}

// --- supporting: report shape ------------------------------------------------

func TestClassString(t *testing.T) {
	tests := []struct {
		class Class
		want  string
	}{
		{ClassSucceeded, "succeeded"},
		{ClassFailed, "failed"},
		{ClassBlocked, "blocked"},
		{ClassSkipped, "skipped"},
		{Class(99), "Class(99)"},
	}
	for _, tt := range tests {
		if got := tt.class.String(); got != tt.want {
			t.Errorf("Class(%d).String() = %q, want %q", int(tt.class), got, tt.want)
		}
	}
}

func TestRunReport_RanAndNothingReady(t *testing.T) {
	rep := RunReport{}
	rep.Results = []TaskResult{
		{Class: ClassSucceeded}, {Class: ClassSucceeded}, {Class: ClassFailed},
		{Class: ClassBlocked}, {Class: ClassSkipped},
	}
	rep.tally()
	if rep.Succeeded != 2 || rep.Failed != 1 || rep.Blocked != 1 || rep.Skipped != 1 {
		t.Fatalf("tally = %+v", rep)
	}
	if rep.Ran() != 3 {
		t.Errorf("Ran() = %d, want 3 (succeeded + failed)", rep.Ran())
	}
	if rep.NothingReady() {
		t.Error("NothingReady() = true, want false when work ran")
	}
	empty := RunReport{}
	empty.tally()
	if !empty.NothingReady() {
		t.Error("NothingReady() = false, want true for an empty report")
	}
}

func (h *fakeHost) DescribeProposal(context.Context, port.Task, port.Workdir, prdescription.Description) (bool, error) {
	return false, nil
}

func (w *fakeWorkdir) ChangeSize(context.Context) (changesize.Size, error) {
	return changesize.Size{}, nil
}
