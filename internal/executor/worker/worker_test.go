package worker

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/changesize"
	"github.com/agenticlab-ai/pyxis/internal/executiondemand"
	"github.com/agenticlab-ai/pyxis/internal/executor/port"
	"github.com/agenticlab-ai/pyxis/internal/nativerouting"
	"github.com/agenticlab-ai/pyxis/internal/prdescription"
	"github.com/agenticlab-ai/pyxis/internal/prdescription/prdescriptiontest"
	"github.com/agenticlab-ai/pyxis/internal/routingevidence"
)

func TestRoutedImplementationTerminalClassifiesEveryExit(t *testing.T) {
	capability := port.LaunchCapability{AttemptID: "attempt-1", OwnerGeneration: 2, CapabilityRef: "capability-1"}
	usage := port.RunResult{Turns: 7}
	cases := []struct {
		name     string
		outcome  Outcome
		err      error
		launched bool
		want     routingevidence.OutcomeClass
		cause    routingevidence.OutcomeCause
	}{
		{name: "claim lost", err: port.ErrClaimLost, launched: true, want: routingevidence.OutcomeBlocked, cause: routingevidence.CauseClaimLost},
		{name: "cancel before launch", err: context.Canceled, want: routingevidence.OutcomeBlocked, cause: routingevidence.CauseCancelled},
		{name: "preflight failure", err: errors.New("preflight"), want: routingevidence.OutcomeFailed, cause: routingevidence.CausePreflightFailed},
		{name: "cancel after launch", err: context.DeadlineExceeded, launched: true, want: routingevidence.OutcomeBlocked, cause: routingevidence.CauseCancelled},
		{name: "verified", outcome: OutcomeSucceeded, launched: true, want: routingevidence.OutcomeVerified, cause: routingevidence.CauseTerminalVerified},
		{name: "worker failure", outcome: OutcomeFailed, err: errors.New("worker"), launched: true, want: routingevidence.OutcomeFailed, cause: routingevidence.CauseWorkerFailed},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := routedImplementationTerminal(tc.outcome, tc.err, tc.launched, capability, usage)
			if got.Outcome != tc.want || got.Cause != tc.cause {
				t.Fatalf("terminal = %+v", got)
			}
			if tc.launched && !errors.Is(tc.err, port.ErrClaimLost) && got.Turns != usage.Turns {
				t.Fatalf("turns = %d, want %d", got.Turns, usage.Turns)
			}
		})
	}
}

func TestWorkerUsageReturnsObservedTelemetry(t *testing.T) {
	w := &Worker{usage: port.RunResult{Tokens: 12, Turns: 3}}
	if got := w.Usage(); got != w.usage {
		t.Fatalf("Usage = %+v, want %+v", got, w.usage)
	}
}

// The inline fakes implement all five ports in-process, with no network, git or
// CLI (docs/tech-design/implement-now.md §3.10 — the substrate-first hermetic
// harness). Compile-time assertions pin them to the port contracts, so any
// signature drift is a build failure the parallel adapter workers will also see.
var (
	_ port.WorkTracker  = (*fakeTracker)(nil)
	_ port.Lease        = (*fakeLease)(nil)
	_ port.Claim        = (*fakeClaim)(nil)
	_ port.Workspace    = (*fakeWorkspace)(nil)
	_ port.Workdir      = (*fakeWorkdir)(nil)
	_ port.AgentRuntime = (*fakeRuntime)(nil)
	_ port.Handle       = fakeHandle{}
	_ port.CodeHost     = (*fakeHost)(nil)
)

// --- fake WorkTracker ---------------------------------------------------------

type fakeTracker struct {
	frontierCalls int
	comments      []trackerComment
	commentErr    error
}

type trackerComment struct {
	issue int
	body  string
}

// Frontier must never be reached by a Worker: a Worker reads only its handed
// Task (prd:implement-now/FR-3). It records the call and errors loudly so a
// regression that peeks at sibling state fails the sibling-isolation test.
func (f *fakeTracker) Frontier(context.Context) ([]port.Task, error) {
	f.frontierCalls++
	return nil, errors.New("fakeTracker: a Worker must not read the frontier")
}

func (f *fakeTracker) Comment(_ context.Context, issue int, body string) error {
	f.comments = append(f.comments, trackerComment{issue: issue, body: body})
	return f.commentErr
}

// --- fake Lease / Claim -------------------------------------------------------

type fakeLease struct {
	acquireErr    error       // when set, Acquire returns it (e.g. ErrClaimLost)
	acquiredTasks []port.Task // every task Acquire was asked for
	claim         *fakeClaim
}

func (f *fakeLease) Acquire(_ context.Context, task port.Task) (port.Claim, error) {
	f.acquiredTasks = append(f.acquiredTasks, task)
	if f.acquireErr != nil {
		return nil, f.acquireErr
	}
	f.claim = &fakeClaim{}
	return f.claim, nil
}

type fakeClaim struct {
	releases int
}

func (c *fakeClaim) Release(context.Context) error {
	c.releases++
	return nil
}

// --- fake Workspace / Workdir -------------------------------------------------

type fakeWorkspace struct {
	dir           string // t.TempDir(); the isolated workdir path
	prepareErr    error
	preparedTasks []port.Task
	// push behavior handed to the Workdir it hands out (default: publishes).
	pushNothing   bool  // the Workdir's Push reports published=false (hollow)
	pushErr       error // the Workdir's Push returns this error
	checkpointErr error // the Workdir's Checkpoint returns this error
	noWork        bool  // the Workdir's Checkpoint reports hasWork=false (empty run)
	size          changesize.Size
	sizeErr       error
	last          *fakeWorkdir // the Workdir from the most recent Prepare, for assertions
}

func (f *fakeWorkspace) Prepare(_ context.Context, task port.Task) (port.Workdir, error) {
	f.preparedTasks = append(f.preparedTasks, task)
	if f.prepareErr != nil {
		return nil, f.prepareErr
	}
	// A real adapter names the worktree branch after the task's head branch.
	wd := &fakeWorkdir{dir: f.dir, branch: task.HeadBranch, pushNothing: f.pushNothing, pushErr: f.pushErr, checkpointErr: f.checkpointErr, noWork: f.noWork, size: f.size, sizeErr: f.sizeErr}
	f.last = wd
	return wd, nil
}

type fakeWorkdir struct {
	dir           string
	branch        string
	pushNothing   bool  // when true, Push reports published=false (nothing to publish)
	pushErr       error // when set, Push returns it
	pushes        int   // Push call count
	checkpoints   int   // Checkpoint call count
	checkpointErr error // when set, Checkpoint returns it
	noWork        bool  // when true, Checkpoint reports hasWork=false (empty run)
	size          changesize.Size
	sizeErr       error
}

func (w *fakeWorkdir) Dir() string    { return w.dir }
func (w *fakeWorkdir) Branch() string { return w.branch }

// Checkpoint models the suspend-time WIP backstop. It records the call so a
// non-convergence path can be asserted to have checkpointed before returning,
// returns checkpointErr to exercise the fail-closed branch, and reports hasWork
// (true unless noWork is set) so the empty-run-respawns path can be exercised.
func (w *fakeWorkdir) Checkpoint(context.Context) (bool, error) {
	w.checkpoints++
	if w.checkpointErr != nil {
		return false, w.checkpointErr
	}
	return !w.noWork, nil
}

// Push models the deterministic publish step. By default it publishes
// (published=true); pushNothing makes it report nothing-to-publish and pushErr
// makes it fail, so the Worker's hollow/failed branches can be exercised.
func (w *fakeWorkdir) Push(context.Context) (bool, error) {
	if w.checkpoints == 0 {
		return false, errors.New("publication preceded supervisor checkpoint")
	}
	w.pushes++
	if w.pushErr != nil {
		return false, w.pushErr
	}
	return !w.pushNothing, nil
}

func (w *fakeWorkdir) Cleanup(context.Context) error { return nil }

// --- fake AgentRuntime / Handle -----------------------------------------------

type fakeRuntime struct {
	authErr   error
	launchErr error
	awaitErr  error
	result    port.RunResult
	launches  []port.LaunchSpec // every spec Launch was called with
	// noDescription makes the agent ignore a description request; by default
	// a launch that asks for one gets agentDescription at the end of its output.
	noDescription bool
}

// agentDescription is the complete description a compliant agent ends its
// final message with.
const agentDescription = prdescriptiontest.Message

// describedResult is what a compliant agent asked for a description returns.
func describedResult(res port.RunResult, spec port.LaunchSpec) port.RunResult {
	if spec.RequestDescription && !strings.Contains(res.Output, prdescription.BeginMarker) {
		res.Output = strings.TrimSpace(res.Output + "\n\n" + agentDescription)
	}
	return res
}

func (r *fakeRuntime) AuthCheck(context.Context) error { return r.authErr }

func (r *fakeRuntime) ModelRoute() port.ModelRoute {
	return port.ModelRoute{Model: "fake-model", Effort: "fake-effort"}
}

func (r *fakeRuntime) Launch(_ context.Context, spec port.LaunchSpec) (port.Handle, error) {
	r.launches = append(r.launches, spec)
	if r.launchErr != nil {
		return nil, r.launchErr
	}
	return fakeHandle{}, nil
}

func (r *fakeRuntime) Activate(attempt string, generation port.OwnerGeneration, proof port.OwnershipProof) (port.LaunchCapability, error) {
	return port.LaunchCapability{AttemptID: attempt, Harness: proof.Harness, OwnerGeneration: generation,
		ProofReference: proof.Reference, ProofDigest: proof.Digest, Nonce: "fake-nonce", CapabilityRef: "fake-capability", Signature: "fake-signature"}, nil
}

func (r *fakeRuntime) LaunchFenced(ctx context.Context, spec port.LaunchSpec, _ port.LaunchCapability) (port.Handle, error) {
	return r.Launch(ctx, spec)
}

func (*fakeRuntime) FenceAndObserve(_ context.Context, attempt string, generation port.OwnerGeneration) (port.RuntimeFenceProof, port.RuntimeObservation, error) {
	return port.RuntimeFenceProof{AttemptID: attempt, OwnerGeneration: generation}, port.RuntimeObservation{AttemptID: attempt, OwnerGeneration: generation, Kind: port.RuntimeAbsentProven}, nil
}

func (*fakeRuntime) AuthorizeTerminal(port.LaunchCapability) error { return nil }

func (r *fakeRuntime) Await(context.Context, port.Handle) (port.RunResult, error) {
	if r.awaitErr != nil {
		return port.RunResult{}, r.awaitErr
	}
	if r.noDescription || len(r.launches) == 0 {
		return r.result, nil
	}
	return describedResult(r.result, r.launches[len(r.launches)-1]), nil
}

func (r *fakeRuntime) Kill(context.Context, port.Handle) error { return nil }

type fakeHandle struct{}

func (fakeHandle) RuntimeHandle() {}

type fakeBoundaryPermit struct {
	err      error
	calls    int
	boundary routingevidence.BoundaryOperation
	binding  routingevidence.BoundaryBinding
}

func (p *fakeBoundaryPermit) Authorize(boundary routingevidence.BoundaryOperation, binding routingevidence.BoundaryBinding) error {
	p.calls++
	p.boundary, p.binding = boundary, binding
	return p.err
}

func (*fakeBoundaryPermit) RuntimeOwnershipProof() (port.OwnershipProof, error) {
	return port.OwnershipProof{Kind: port.OwnershipProofResolution, Reference: "resolution", Digest: strings.Repeat("a", 64),
		AttemptID: "attempt-42", Harness: "claude", OwnerGeneration: 1}, nil
}

func (*fakeBoundaryPermit) NativeLaunch(role nativerouting.Role) (port.NativeLaunchSpec, error) {
	return port.NativeLaunchSpec{NativeProfileID: "claude-implementation-strong", Role: role, NativeModel: "native-model", NativeEffort: "native-effort"}, nil
}

// --- fake CodeHost ------------------------------------------------------------

type fakeHost struct {
	openErr     error // e.g. port.ErrNoProposal, or a hard error
	proposal    port.Proposal
	openedTasks []port.Task
	openedDirs  []string // workdir.Dir() seen at OpenProposal
	checkState  port.CheckState
	checkErr    error
	checksRead  []int // proposal numbers Checks was called with

	opened               []prdescription.Description // the description each OpenProposal carried
	describes            []prdescription.Description // every DescribeProposal's description
	describedAfterPushes []int                       // the workdir's push count at each DescribeProposal
	describeOpen         bool                        // DescribeProposal reports an open proposal
	describeErr          error
}

func (h *fakeHost) OpenProposal(_ context.Context, task port.Task, work port.Workdir, d prdescription.Description) (port.Proposal, error) {
	h.openedTasks = append(h.openedTasks, task)
	h.opened = append(h.opened, d)
	h.openedDirs = append(h.openedDirs, work.Dir())
	if h.openErr != nil {
		return port.Proposal{}, h.openErr
	}
	p := h.proposal
	if p.Branch == "" {
		p.Branch = work.Branch() // reflect the workspace branch, as a real adapter would
	}
	return p, nil
}

func (h *fakeHost) Checks(_ context.Context, p port.Proposal) (port.CheckState, error) {
	h.checksRead = append(h.checksRead, p.Number)
	if h.checkErr != nil {
		return port.CheckPending, h.checkErr
	}
	return h.checkState, nil
}

// --- helpers ------------------------------------------------------------------

func newTestTask() port.Task {
	return port.Task{
		ID:     "T-42",
		Number: 42,
		Title:  "add a widget",
		Brief:  "Implement the widget per the charter.",

		Harness:    "claude",
		HeadBranch: "feat/widget-42",
	}
}

func TestRun_UnavailableVerificationScratchDoesNotClaimOrLaunch(t *testing.T) {
	deps, _, lease, space, runtime, host := greenDeps(t)
	w := mustWorker(t, deps)
	t.Setenv("TMPDIR", filepath.Join(t.TempDir(), "missing"))
	outcome, err := w.Run(t.Context(), newTestTask())
	if outcome != OutcomeFailed || err == nil || !strings.Contains(err.Error(), "verification scratch") {
		t.Fatalf("scratch failure = %s, %v", outcome, err)
	}
	if len(lease.acquiredTasks) != 0 || len(space.preparedTasks) != 0 || len(runtime.launches) != 0 || len(host.openedTasks) != 0 {
		t.Fatal("scratch failure consumed a claim, workspace, launch, or publication")
	}
}

func mustWorker(t *testing.T, deps Deps, opts ...Option) *Worker {
	t.Helper()
	auth := t.TempDir()
	t.Setenv("CLAUDE_CONFIG_DIR", auth)
	t.Setenv("CODEX_HOME", auth)
	if err := os.WriteFile(filepath.Join(auth, ".credentials.json"), []byte(`{"token":"unit-test-only"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	w, err := New(deps, opts...)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return w
}

// greenDeps wires a set of fakes that drive a task to a green proposal.
func greenDeps(t *testing.T) (Deps, *fakeTracker, *fakeLease, *fakeWorkspace, *fakeRuntime, *fakeHost) {
	t.Helper()
	tracker := &fakeTracker{}
	lease := &fakeLease{}
	space := &fakeWorkspace{dir: t.TempDir()}
	runtime := &fakeRuntime{result: port.RunResult{Tokens: 1200, Turns: 5}}
	host := &fakeHost{proposal: port.Proposal{Number: 7}, checkState: port.CheckGreen}
	deps := Deps{Tracker: tracker, Lease: lease, Space: space, Runtime: runtime, Host: host}
	return deps, tracker, lease, space, runtime, host
}

// --- acceptance criterion 1: full cycle -> succeeded --------------------------

func TestRun_FullCycleSucceeded(t *testing.T) {
	deps, tracker, lease, space, runtime, host := greenDeps(t)
	w := mustWorker(t, deps)
	task := newTestTask()

	outcome, err := w.Run(context.Background(), task)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if outcome != OutcomeSucceeded {
		t.Fatalf("outcome = %v, want succeeded", outcome)
	}

	// The whole cycle ran, once, on this one task.
	if len(lease.acquiredTasks) != 1 || lease.acquiredTasks[0].ID != task.ID {
		t.Fatalf("Acquire not called once for the task: %+v", lease.acquiredTasks)
	}
	if lease.claim == nil || lease.claim.releases != 1 {
		t.Fatalf("claim not released exactly once: %+v", lease.claim)
	}
	if len(space.preparedTasks) != 1 {
		t.Fatalf("Prepare called %d times, want 1", len(space.preparedTasks))
	}
	if len(runtime.launches) != 1 {
		t.Fatalf("Launch called %d times, want 1", len(runtime.launches))
	}

	// The frozen rung was clamped, and the resolved model/effort + brief +
	// workdir reached Launch (the AP-21 clamp, the DINV-11 fresh brief).
	spec := runtime.launches[0]
	if spec.Brief != task.Brief {
		t.Errorf("launch brief = %q, want %q", spec.Brief, task.Brief)
	}
	if spec.Workdir != space.dir {
		t.Errorf("launch workdir = %q, want %q", spec.Workdir, space.dir)
	}
	if spec.Model != "fake-model" || spec.Effort != "fake-effort" {
		t.Errorf("launch model/effort = %q/%q, want fake-model/fake-effort", spec.Model, spec.Effort)
	}

	// The proposal was opened from this workspace and verified by its checks.
	if len(host.openedTasks) != 1 {
		t.Fatalf("OpenProposal called %d times, want 1", len(host.openedTasks))
	}
	if host.openedDirs[0] != space.dir {
		t.Errorf("OpenProposal workdir = %q, want %q", host.openedDirs[0], space.dir)
	}
	if len(host.checksRead) != 1 || host.checksRead[0] != 7 {
		t.Fatalf("Checks not read once for proposal 7: %v", host.checksRead)
	}

	// A best-effort progress comment landed on this issue only.
	if len(tracker.comments) != 1 || tracker.comments[0].issue != task.Number {
		t.Fatalf("expected one comment on issue %d, got %+v", task.Number, tracker.comments)
	}
}

func TestRun_RoutingPermitMechanicallyGatesClaim(t *testing.T) {
	for _, tc := range []struct {
		name       string
		admitErr   error
		permitErr  error
		claimErr   error
		wantClaims int
		wantLaunch int
		wantRoute  routingevidence.OutcomeClass
	}{
		{name: "admission unavailable", admitErr: errors.New("evidence unavailable")},
		{name: "invalid permit", permitErr: errors.New("proof altered")},
		{name: "claim loss after permit", claimErr: port.ErrClaimLost, wantClaims: 1, wantRoute: routingevidence.OutcomeBlocked},
		{name: "valid permit", wantClaims: 1, wantLaunch: 1, wantRoute: routingevidence.OutcomeVerified},
	} {
		t.Run(tc.name, func(t *testing.T) {
			deps, _, lease, _, runtime, _ := greenDeps(t)
			lease.acquireErr = tc.claimErr
			permit := &fakeBoundaryPermit{err: tc.permitErr}
			admitCalls := 0
			var terminal *routingevidence.AttemptTerminal
			w := mustWorker(t, deps, WithImplementationAdmission(func(_ context.Context, task port.Task) (routingevidence.BoundaryPermit, error) {
				admitCalls++
				if task.ID != "T-42" || task.Harness != "claude" {
					t.Fatalf("admission task = %+v", task)
				}
				if tc.admitErr != nil {
					return nil, tc.admitErr
				}
				return permit, nil
			}), WithImplementationOutcome(func(_ context.Context, got routingevidence.AttemptTerminal) error {
				terminal = &got
				return nil
			}))
			task := newTestTask()
			task.DemandID = "demand-42"
			task.DemandProfile = executiondemand.ProfileMinimum{
				Capability: executiondemand.CapabilityStrong, Deliberation: executiondemand.DeliberationDeep,
				Context: executiondemand.ContextLarge, Assurance: executiondemand.AssuranceElevated,
			}
			_, _ = w.Run(context.Background(), task)
			if admitCalls != 1 || len(lease.acquiredTasks) != tc.wantClaims || len(runtime.launches) != tc.wantLaunch {
				t.Fatalf("calls: admit=%d claim=%d launch=%d, want 1/%d/%d", admitCalls, len(lease.acquiredTasks), len(runtime.launches), tc.wantClaims, tc.wantLaunch)
			}
			if tc.admitErr == nil {
				if permit.calls != 1 || permit.boundary != routingevidence.OperationClaim || permit.binding.Role != nativerouting.RoleImplementation ||
					permit.binding.DemandID != string(task.DemandID) || permit.binding.TaskID != task.ID || permit.binding.Harness != task.Harness {
					t.Fatalf("claim permit binding = calls:%d boundary:%s binding:%+v", permit.calls, permit.boundary, permit.binding)
				}
			}
			if tc.wantRoute == "" && terminal != nil {
				t.Fatalf("unexpected routed terminal %+v", terminal)
			}
			if tc.wantRoute != "" && (terminal == nil || terminal.Outcome != tc.wantRoute || terminal.AttemptID != "attempt-42" || terminal.LaunchCapabilityRef == "") {
				t.Fatalf("routed terminal = %+v, want %s exact attempt/capability", terminal, tc.wantRoute)
			}
		})
	}
}

// --- acceptance criterion 2: no proposal -> hollow, never succeeded -----------

func TestRun_NoProposalIsHollow(t *testing.T) {
	deps, _, lease, _, _, host := greenDeps(t)
	host.openErr = port.ErrNoProposal
	w := mustWorker(t, deps)

	outcome, err := w.Run(context.Background(), newTestTask())
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if outcome == OutcomeSucceeded {
		t.Fatal("a clean run with no proposal must never be succeeded (DINV-3)")
	}
	if outcome != OutcomeHollow {
		t.Fatalf("outcome = %v, want hollow", outcome)
	}
	// There was no proposal, so no checks were read...
	if len(host.checksRead) != 0 {
		t.Errorf("Checks read for a hollow run: %v", host.checksRead)
	}
	// ...and the claim was still released (fire-or-release, DINV-5).
	if lease.claim == nil || lease.claim.releases != 1 {
		t.Fatalf("claim not released exactly once on hollow: %+v", lease.claim)
	}
}

// A run the CLI reports as truncated (it hit its turn cap or errored internally,
// res.Incomplete) is recoverable, not a deliberate no-op: it routes to
// OutcomeUnconverged so the Dispatcher respawns it and the idempotent resume
// continues the WIP — never a spurious hollow that strands the work un-retried.
func TestRun_IncompleteRunIsUnconvergedNotHollow(t *testing.T) {
	deps, _, _, space, runtime, host := greenDeps(t)
	// Even a plan-only run that wrote (and pushed) nothing must be respawn-eligible
	// when the CLI flags it incomplete — the truncation, not the empty worktree,
	// decides.
	runtime.result = port.RunResult{Turns: 150, Incomplete: true, Reason: "error_max_turns"}
	w := mustWorker(t, deps)

	outcome, err := w.Run(context.Background(), newTestTask())
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if outcome != OutcomeUnconverged {
		t.Fatalf("outcome = %v, want unconverged (a truncated run is respawn-eligible, not hollow)", outcome)
	}
	// The truncation is caught right after Await, before publish — no push, no
	// proposal — so nothing half-done is opened.
	if space.last != nil && space.last.pushes != 0 {
		t.Errorf("Push attempted on a truncated run: %+v", space.last)
	}
	if len(host.openedTasks) != 0 {
		t.Errorf("OpenProposal called on a truncated run: %v", host.openedTasks)
	}
	// The WIP is checkpointed (committed) before suspending, so the resume
	// continues from a committed base and the work is not stranded uncommitted.
	if space.last == nil || space.last.checkpoints != 1 {
		t.Fatalf("Checkpoint called %v times on a truncated run, want exactly 1", space.last)
	}
}

// The mechanical bound (a killed process, errBudgetExhausted) is the other
// non-convergence exit: it too checkpoints the WIP before returning
// OutcomeUnconverged, so a wall-clock/turn-bound kill mid-implementation does not
// strand the agent's uncommitted work.
func TestRun_BudgetExhaustedCheckpointsThenUnconverged(t *testing.T) {
	deps, _, _, space, runtime, host := greenDeps(t)
	runtime.awaitErr = errBudgetExhausted
	w := mustWorker(t, deps)

	outcome, err := w.Run(context.Background(), newTestTask())
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if outcome != OutcomeUnconverged {
		t.Fatalf("outcome = %v, want unconverged (a bound-killed run is respawn-eligible)", outcome)
	}
	if space.last == nil || space.last.checkpoints != 1 {
		t.Fatalf("Checkpoint called %v times on a bound-killed run, want exactly 1", space.last)
	}
	if space.last.pushes != 0 || len(host.openedTasks) != 0 {
		t.Errorf("publish attempted on a bound-killed run: pushes=%d opened=%v", space.last.pushes, host.openedTasks)
	}
}

// A checkpoint that itself fails is a real git fault in the isolated worktree:
// the Worker fails closed (INV-6) and surfaces it, rather than silently
// discarding the error and reporting a clean non-convergence.
func TestRun_CheckpointFailureFailsClosed(t *testing.T) {
	deps, _, _, space, runtime, _ := greenDeps(t)
	runtime.result = port.RunResult{Turns: 150, Incomplete: true, Reason: "error_max_turns"}
	space.checkpointErr = errors.New("index.lock held")
	w := mustWorker(t, deps)

	outcome, err := w.Run(context.Background(), newTestTask())
	if outcome != OutcomeFailed {
		t.Fatalf("outcome = %v, want failed (a checkpoint fault fails closed)", outcome)
	}
	if err == nil || !strings.Contains(err.Error(), "checkpoint") {
		t.Fatalf("err = %v, want it to name the checkpoint failure", err)
	}
}

// --- ship-green-on-truncation: verify by artifact, not by the agent's signal ---

// The T5 dogfood scenario: the agent truncated (Incomplete) but its checkpointed
// WIP passes the local gate, so the Worker publishes and opens a proposal — the
// green work ships (converges) rather than escalating with the PR never opened.
func TestRun_TruncatedButGreenShips(t *testing.T) {
	deps, _, _, space, runtime, host := greenDeps(t)
	runtime.result = port.RunResult{Turns: 150, Incomplete: true, Reason: "error_max_turns"}
	host.checkState = port.CheckGreen
	driver := &fakeGreenDriver{gateGreen: true} // committed WIP passes the local gate
	w := mustWorker(t, deps, WithGreenDriver(driver))

	outcome, err := w.Run(context.Background(), newTestTask())
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if outcome != OutcomeSucceeded {
		t.Fatalf("outcome = %v, want succeeded (truncated-but-green work ships)", outcome)
	}
	// It checkpointed, gated, THEN published and opened the proposal.
	if space.last == nil || space.last.checkpoints != 1 {
		t.Fatalf("Checkpoint called %v times, want 1", space.last)
	}
	if driver.gateCalled != 1 {
		t.Fatalf("GateGreen called %d times, want 1", driver.gateCalled)
	}
	if space.last.pushes != 1 {
		t.Errorf("Push called %d times, want 1 (the green truncated work is published)", space.last.pushes)
	}
	if len(host.openedTasks) != 1 {
		t.Errorf("OpenProposal called %d times, want 1", len(host.openedTasks))
	}
}

// A truncated run whose committed WIP does NOT pass the local gate respawns from
// the committed base — never a churny PR opened from partial/red work.
func TestRun_TruncatedRedWorkRespawnsNoPublish(t *testing.T) {
	deps, _, _, space, runtime, host := greenDeps(t)
	runtime.result = port.RunResult{Turns: 150, Incomplete: true, Reason: "error_max_turns"}
	driver := &fakeGreenDriver{gateGreen: false} // committed WIP fails the local gate
	w := mustWorker(t, deps, WithGreenDriver(driver))

	outcome, err := w.Run(context.Background(), newTestTask())
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if outcome != OutcomeUnconverged {
		t.Fatalf("outcome = %v, want unconverged (red truncated work respawns)", outcome)
	}
	if driver.gateCalled != 1 {
		t.Fatalf("GateGreen called %d times, want 1 (the gate decides)", driver.gateCalled)
	}
	if space.last.pushes != 0 || len(host.openedTasks) != 0 {
		t.Errorf("published red truncated work: pushes=%d opened=%d", space.last.pushes, len(host.openedTasks))
	}
}

// A truncated run that produced NOTHING respawns without even running the gate —
// the empty-run short-circuit (preserves the plan-only-respawn behavior).
func TestRun_TruncatedEmptyRunRespawnsWithoutGate(t *testing.T) {
	deps, _, _, space, runtime, host := greenDeps(t)
	runtime.result = port.RunResult{Turns: 150, Incomplete: true, Reason: "error_max_turns"}
	space.noWork = true // Checkpoint reports hasWork=false
	driver := &fakeGreenDriver{gateGreen: true}
	w := mustWorker(t, deps, WithGreenDriver(driver))

	outcome, err := w.Run(context.Background(), newTestTask())
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if outcome != OutcomeUnconverged {
		t.Fatalf("outcome = %v, want unconverged (empty truncation respawns)", outcome)
	}
	if space.last == nil || space.last.checkpoints != 1 {
		t.Fatalf("Checkpoint called %v times, want 1", space.last)
	}
	if driver.gateCalled != 0 {
		t.Errorf("GateGreen called %d times on an empty run, want 0 (short-circuit before the gate)", driver.gateCalled)
	}
	if space.last.pushes != 0 || len(host.openedTasks) != 0 {
		t.Errorf("published an empty run: pushes=%d opened=%d", space.last.pushes, len(host.openedTasks))
	}
}

// An operational gate FAULT (a check that could not be run — not a check
// failing) fails closed: the Worker surfaces it rather than silently respawning
// on an unverifiable gate.
func TestRun_TruncatedGateFaultFailsClosed(t *testing.T) {
	deps, _, _, _, runtime, _ := greenDeps(t)
	runtime.result = port.RunResult{Turns: 150, Incomplete: true, Reason: "error_max_turns"}
	driver := &fakeGreenDriver{gateErr: errors.New("go: command not found")}
	w := mustWorker(t, deps, WithGreenDriver(driver))

	outcome, err := w.Run(context.Background(), newTestTask())
	if outcome != OutcomeFailed {
		t.Fatalf("outcome = %v, want failed (a gate fault fails closed)", outcome)
	}
	if err == nil || !strings.Contains(err.Error(), "local gate") {
		t.Fatalf("err = %v, want it to name the local gate failure", err)
	}
}

// --- the supervisor checkpoints, publishes, then opens the proposal ---

// The head branch is published before a proposal is opened — the missing step
// the A-12 dogfood exposed (nothing pushed → OpenProposal 404). On the green
// path the Worker calls Push exactly once and only then opens the proposal.
func TestRun_PublishesBeforeOpenProposal(t *testing.T) {
	deps, _, _, space, runtime, host := greenDeps(t)
	w := mustWorker(t, deps)
	task := newTestTask()

	outcome, err := w.Run(context.Background(), task)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if outcome != OutcomeSucceeded {
		t.Fatalf("outcome = %v, want succeeded", outcome)
	}
	if space.last == nil || space.last.pushes != 1 {
		t.Fatalf("Push called %v times, want exactly 1 before OpenProposal", space.last)
	}
	if len(host.openedTasks) != 1 {
		t.Fatalf("OpenProposal called %d times, want 1 (after a successful push)", len(host.openedTasks))
	}
	// The agent receives its assigned branch as context; Go owns the commit.
	if len(runtime.launches) != 1 || runtime.launches[0].HeadBranch != task.HeadBranch {
		t.Fatalf("Launch head branch = %+v, want %q", runtime.launches, task.HeadBranch)
	}
}

// Nothing published (the agent changed nothing) is hollow, never succeeded,
// and no proposal is opened — a worktree with no commits is detected at the push
// step, before any code-host round-trip (DINV-3).
func TestRun_NothingToPublishIsHollow(t *testing.T) {
	deps, _, lease, space, _, host := greenDeps(t)
	space.pushNothing = true
	w := mustWorker(t, deps)

	outcome, err := w.Run(context.Background(), newTestTask())
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if outcome != OutcomeHollow {
		t.Fatalf("outcome = %v, want hollow when nothing is published", outcome)
	}
	if space.last == nil || space.last.pushes != 1 {
		t.Fatalf("Push not attempted exactly once: %+v", space.last)
	}
	// Nothing to propose: OpenProposal is never reached (no remote round-trip).
	if len(host.openedTasks) != 0 {
		t.Errorf("OpenProposal called for a nothing-to-publish run: %d", len(host.openedTasks))
	}
	if lease.claim == nil || lease.claim.releases != 1 {
		t.Fatalf("claim not released exactly once on hollow: %+v", lease.claim)
	}
}

// A publish failure fails the task (fail-closed) and never reaches OpenProposal;
// the claim is still released.
func TestRun_PublishFailureIsFailed(t *testing.T) {
	deps, _, lease, space, _, host := greenDeps(t)
	space.pushErr = errors.New("remote rejected the push")
	w := mustWorker(t, deps)

	outcome, err := w.Run(context.Background(), newTestTask())
	if err == nil {
		t.Fatal("Run: want an error for a failed publish")
	}
	if !strings.Contains(err.Error(), "publish") {
		t.Errorf("error should name the publish step, got %q", err)
	}
	if outcome != OutcomeFailed {
		t.Fatalf("outcome = %v, want failed", outcome)
	}
	if len(host.openedTasks) != 0 {
		t.Errorf("OpenProposal called despite a failed publish: %d", len(host.openedTasks))
	}
	if lease.claim == nil || lease.claim.releases != 1 {
		t.Fatalf("claim not released exactly once on publish failure: %+v", lease.claim)
	}
}

// --- acceptance criterion 3: reads only its own task --------------------------

func TestRun_ReadsOnlyItsOwnTask(t *testing.T) {
	deps, tracker, lease, space, runtime, host := greenDeps(t)
	host.proposal = port.Proposal{Number: 9}
	w := mustWorker(t, deps)
	task := newTestTask()

	if _, err := w.Run(context.Background(), task); err != nil {
		t.Fatalf("Run: %v", err)
	}

	// The Worker never consulted the frontier: it works only its handed task,
	// never sibling or session state (prd:implement-now/FR-3, DINV-11 — the
	// charter-bleed guard). This assertion is the regression trap.
	if tracker.frontierCalls != 0 {
		t.Fatalf("Worker read the frontier %d times; it must read only its own task", tracker.frontierCalls)
	}

	// Every port interaction referenced only this task's identity.
	for _, got := range lease.acquiredTasks {
		if got.ID != task.ID {
			t.Errorf("Acquire saw a foreign task %q, want %q", got.ID, task.ID)
		}
	}
	for _, got := range space.preparedTasks {
		if got.ID != task.ID {
			t.Errorf("Prepare saw a foreign task %q, want %q", got.ID, task.ID)
		}
	}
	for _, got := range host.openedTasks {
		if got.ID != task.ID {
			t.Errorf("OpenProposal saw a foreign task %q, want %q", got.ID, task.ID)
		}
	}
	for _, c := range tracker.comments {
		if c.issue != task.Number {
			t.Errorf("comment on foreign issue %d, want %d", c.issue, task.Number)
		}
	}

	// The brief handed to the agent is exactly this task's charter — no other
	// task's instruction bled in (DINV-11).
	if len(runtime.launches) != 1 || runtime.launches[0].Brief != task.Brief {
		t.Fatalf("agent brief did not match this task's charter: %+v", runtime.launches)
	}
}

// --- supporting invariant coverage --------------------------------------------

// A proposal that exists but is not green is failed, never succeeded: success
// is the verified green artifact (DINV-3, AP-13).
func TestRun_NonGreenProposalIsFailed(t *testing.T) {
	for _, tc := range []struct {
		name  string
		state port.CheckState
	}{
		{"red", port.CheckRed},
		{"pending", port.CheckPending},
	} {
		t.Run(tc.name, func(t *testing.T) {
			deps, _, lease, _, _, host := greenDeps(t)
			host.proposal = port.Proposal{Number: 11}
			host.checkState = tc.state
			w := mustWorker(t, deps)

			outcome, err := w.Run(context.Background(), newTestTask())
			if err != nil {
				t.Fatalf("Run returned error: %v", err)
			}
			if outcome != OutcomeFailed {
				t.Fatalf("outcome = %v, want failed for a %s proposal", outcome, tc.name)
			}
			if lease.claim == nil || lease.claim.releases != 1 {
				t.Fatalf("claim not released exactly once: %+v", lease.claim)
			}
		})
	}
}

// A lost claim is a skip, not a work failure: the sentinel propagates for the
// Dispatcher to distinguish, and nothing downstream runs (DINV-5, AP-22).
func TestRun_ClaimLostSkips(t *testing.T) {
	deps, _, lease, space, runtime, _ := greenDeps(t)
	lease.acquireErr = port.ErrClaimLost
	w := mustWorker(t, deps)

	outcome, err := w.Run(context.Background(), newTestTask())
	if !errors.Is(err, port.ErrClaimLost) {
		t.Fatalf("err = %v, want wrapped ErrClaimLost", err)
	}
	if outcome != OutcomeFailed {
		t.Errorf("outcome = %v, want failed (advisory; caller keys on the sentinel)", outcome)
	}
	// No claim was held, so none is recorded; nothing downstream ran.
	if lease.claim != nil {
		t.Error("a claim was recorded despite a lost race")
	}
	if len(space.preparedTasks) != 0 {
		t.Error("workspace prepared despite a lost claim")
	}
	if len(runtime.launches) != 0 {
		t.Error("agent launched despite a lost claim")
	}
}

// The claim is released even when the cycle fails mid-run (fire-or-release on
// every terminal path, DINV-5).
func TestRunUnquiescedAgentNeverCheckpointsOrPublishes(t *testing.T) {
	deps, _, _, space, runtime, host := greenDeps(t)
	runtime.awaitErr = port.ErrNotQuiescent
	_, err := mustWorker(t, deps).Run(t.Context(), newTestTask())
	if !errors.Is(err, port.ErrNotQuiescent) || space.last.checkpoints != 0 || space.last.pushes != 0 || len(host.openedTasks) != 0 {
		t.Fatalf("unquiesced agent reached a write: err=%v work=%+v", err, space.last)
	}
}

func TestRun_ReleasesClaimOnOperationalFailure(t *testing.T) {
	deps, _, lease, space, runtime, host := greenDeps(t)
	runtime.awaitErr = errors.New("agent process crashed")
	w := mustWorker(t, deps)

	outcome, err := w.Run(context.Background(), newTestTask())
	if err == nil {
		t.Fatal("Run: want an error for a crashed agent")
	}
	if outcome != OutcomeFailed {
		t.Fatalf("outcome = %v, want failed", outcome)
	}
	if lease.claim == nil || lease.claim.releases != 1 {
		t.Fatalf("claim not released exactly once on failure: %+v", lease.claim)
	}
	if space.last.checkpoints != 1 || space.last.pushes != 0 || len(host.openedTasks) != 0 {
		t.Fatal("crashed agent must leave checkpointed WIP without publication")
	}
}

// A missing credential degrades before any claim, so no attempt is consumed
// (prd:implement-now/FR-16): no claim, no workspace, no launch.
func TestRun_AuthFailureConsumesNoAttempt(t *testing.T) {
	deps, _, lease, space, runtime, _ := greenDeps(t)
	runtime.authErr = errors.New("backend not authenticated")
	w := mustWorker(t, deps)

	outcome, err := w.Run(context.Background(), newTestTask())
	if err == nil {
		t.Fatal("Run: want an error for a failed auth preflight")
	}
	if outcome != OutcomeFailed {
		t.Fatalf("outcome = %v, want failed", outcome)
	}
	if len(lease.acquiredTasks) != 0 {
		t.Error("task claimed despite a failed auth preflight")
	}
	if len(space.preparedTasks) != 0 {
		t.Error("workspace prepared despite a failed auth preflight")
	}
	if len(runtime.launches) != 0 {
		t.Error("agent launched despite a failed auth preflight")
	}
}

// A workspace failure fails the task and still releases the claim.
func TestRun_WorkspaceFailureIsFailed(t *testing.T) {
	deps, _, lease, space, _, _ := greenDeps(t)
	space.prepareErr = errors.New("worktree add failed")
	w := mustWorker(t, deps)

	outcome, err := w.Run(context.Background(), newTestTask())
	if err == nil {
		t.Fatal("Run: want an error for a failed workspace prepare")
	}
	if outcome != OutcomeFailed {
		t.Fatalf("outcome = %v, want failed", outcome)
	}
	if lease.claim == nil || lease.claim.releases != 1 {
		t.Fatalf("claim not released exactly once: %+v", lease.claim)
	}
}

// A best-effort comment failure must not fail an otherwise-green task: the
// proposal, not the comment, is the artifact (DINV-3).
func TestRun_CommentFailureIsNonFatal(t *testing.T) {
	deps, tracker, _, _, _, _ := greenDeps(t)
	tracker.commentErr = errors.New("tracker unavailable")
	w := mustWorker(t, deps)

	outcome, err := w.Run(context.Background(), newTestTask())
	if err != nil {
		t.Fatalf("a comment failure must not fail the task: %v", err)
	}
	if outcome != OutcomeSucceeded {
		t.Fatalf("outcome = %v, want succeeded", outcome)
	}
}

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
			if _, err := New(deps); err == nil {
				t.Fatalf("New(%s): want an error", tt.name)
			}
		})
	}
	if _, err := New(full()); err != nil {
		t.Fatalf("New(full): unexpected error: %v", err)
	}
}

func TestOutcomeString(t *testing.T) {
	tests := []struct {
		outcome Outcome
		want    string
	}{
		{OutcomeSucceeded, "succeeded"},
		{OutcomeHollow, "hollow"},
		{OutcomeFailed, "failed"},
		{Outcome(99), "Outcome(99)"},
	}
	for _, tt := range tests {
		if got := tt.outcome.String(); got != tt.want {
			t.Errorf("Outcome(%d).String() = %q, want %q", int(tt.outcome), got, tt.want)
		}
	}
}

func (h *fakeHost) DescribeProposal(_ context.Context, _ port.Task, work port.Workdir, d prdescription.Description) (bool, error) {
	h.describes = append(h.describes, d)
	if fw, ok := work.(*fakeWorkdir); ok {
		h.describedAfterPushes = append(h.describedAfterPushes, fw.pushes)
	}
	return h.describeOpen, h.describeErr
}

func (w *fakeWorkdir) ChangeSize(context.Context) (changesize.Size, error) {
	return w.size, w.sizeErr
}
