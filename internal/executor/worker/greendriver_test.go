package worker

import (
	"context"
	"errors"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/executor/port"
)

// fakeGreenDriver is an in-process GreenDriver: it records the call and returns a
// scripted result/error, so the Worker↔driver wiring is tested without the real
// self-heal loop (that loop is covered in package drivetogreen).
type fakeGreenDriver struct {
	called      int
	result      GreenDriveResult
	err         error
	sawTask     port.Task
	sawProposal port.Proposal
	sawDir      string

	// local-gate verdict (GateGreen), scripted independently of Drive: the
	// pre-publish predicate for a bound-truncated run. Defaults to not-green, so a
	// truncated run respawns unless a test opts its committed WIP into shippable.
	gateGreen  bool
	gateErr    error
	gateCalled int

	// local convergence drive (DriveLocal), scripted independently: the
	// pre-publish "ensure local CI is green" step. Defaults to green with zero
	// fixer attempts so the existing publish-path tests run unchanged; a test
	// scripts localResult/localErr to exercise the fail-closed branches.
	localCalled  int
	localResult  *LocalDriveResult
	localErr     error
	localErrAt   int // when > 0, localErr applies only to the Nth DriveLocal call
	localSawDirs []string
}

func (d *fakeGreenDriver) Drive(_ context.Context, task port.Task, work port.Workdir, p port.Proposal) (GreenDriveResult, error) {
	d.called++
	d.sawTask = task
	d.sawProposal = p
	if work != nil {
		d.sawDir = work.Dir()
	}
	return d.result, d.err
}

func (d *fakeGreenDriver) GateGreen(_ context.Context, _ port.Workdir) (bool, error) {
	d.gateCalled++
	return d.gateGreen, d.gateErr
}

func (d *fakeGreenDriver) DriveLocal(_ context.Context, _ port.Task, work port.Workdir) (LocalDriveResult, error) {
	d.localCalled++
	if work != nil {
		d.localSawDirs = append(d.localSawDirs, work.Dir())
	}
	if d.localErr != nil && (d.localErrAt == 0 || d.localErrAt == d.localCalled) {
		return LocalDriveResult{}, d.localErr
	}
	if d.localResult != nil {
		return *d.localResult, nil
	}
	return LocalDriveResult{Green: true, Cap: 3}, nil
}

var _ GreenDriver = (*fakeGreenDriver)(nil)

// A red proposal with a green driver installed is driven to green, then counts
// succeeded (prd:implement-now/FR-7, FR-9). The driver sees this task's proposal
// and workspace.
func TestRun_GreenDriverDrivesRedToGreen(t *testing.T) {
	deps, _, _, space, _, host := greenDeps(t)
	host.proposal = port.Proposal{Number: 7}
	host.checkState = port.CheckRed
	driver := &fakeGreenDriver{result: GreenDriveResult{Green: true, Attempts: 2, Cap: 3}}
	w := mustWorker(t, deps, WithGreenDriver(driver))

	outcome, err := w.Run(context.Background(), newTestTask())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if outcome != OutcomeSucceeded {
		t.Fatalf("outcome = %v, want succeeded (driven to green)", outcome)
	}
	if driver.called != 1 {
		t.Fatalf("driver called %d times, want 1", driver.called)
	}
	if driver.sawProposal.Number != 7 {
		t.Errorf("driver saw proposal #%d, want #7", driver.sawProposal.Number)
	}
	if driver.sawTask.ID != "T-42" {
		t.Errorf("driver saw task %q, want T-42", driver.sawTask.ID)
	}
	if driver.sawDir != space.dir {
		t.Errorf("driver saw workdir %q, want %q", driver.sawDir, space.dir)
	}
}

// When the driver exhausts its bound it returns an error (an escalation); the
// Worker classifies the task failed and carries the escalation up (DINV-3, §3.9).
func TestRun_GreenDriverEscalationIsFailed(t *testing.T) {
	deps, _, lease, _, _, host := greenDeps(t)
	host.proposal = port.Proposal{Number: 8}
	host.checkState = port.CheckRed
	escErr := errors.New("not green after 3/3 self-heal attempts")
	driver := &fakeGreenDriver{result: GreenDriveResult{Green: false, Attempts: 3, Cap: 3}, err: escErr}
	w := mustWorker(t, deps, WithGreenDriver(driver))

	outcome, err := w.Run(context.Background(), newTestTask())
	if outcome != OutcomeFailed {
		t.Fatalf("outcome = %v, want failed", outcome)
	}
	if !errors.Is(err, escErr) {
		t.Fatalf("err = %v, want it to wrap the escalation", err)
	}
	if driver.called != 1 {
		t.Fatalf("driver called %d times, want 1", driver.called)
	}
	// The claim is still released on this terminal path (fire-or-release, DINV-5).
	if lease.claim == nil || lease.claim.releases != 1 {
		t.Fatalf("claim not released exactly once: %+v", lease.claim)
	}
}

// A green proposal never invokes the driver — there is nothing to drive.
func TestRun_GreenDriverNotCalledWhenAlreadyGreen(t *testing.T) {
	deps, _, _, _, _, host := greenDeps(t)
	host.checkState = port.CheckGreen
	driver := &fakeGreenDriver{result: GreenDriveResult{Green: true}}
	w := mustWorker(t, deps, WithGreenDriver(driver))

	outcome, err := w.Run(context.Background(), newTestTask())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if outcome != OutcomeSucceeded {
		t.Fatalf("outcome = %v, want succeeded", outcome)
	}
	if driver.called != 0 {
		t.Fatalf("driver called %d times for an already-green proposal, want 0", driver.called)
	}
}

// Pending is routed to the driver as a settlement wait, not collapsed into the
// red path. The real driver proves this call consumes zero fixer attempts; this
// seam test pins the Worker's explicit three-state dispatch.
func TestRun_GreenDriverSettlesPendingToGreen(t *testing.T) {
	deps, _, _, _, _, host := greenDeps(t)
	host.checkState = port.CheckPending
	driver := &fakeGreenDriver{result: GreenDriveResult{Green: true, Attempts: 0, Cap: 3}}
	w := mustWorker(t, deps, WithGreenDriver(driver))

	outcome, err := w.Run(context.Background(), newTestTask())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if outcome != OutcomeSucceeded || driver.called != 1 {
		t.Fatalf("outcome=%v driver calls=%d, want pending settled once to success", outcome, driver.called)
	}
}

// The DEFAULT (no option) leaves the base verify unchanged: a red proposal is
// failed, with no error and no drive — exactly the pre-T10 behavior.
func TestRun_NoDriverKeepsBaseBehavior(t *testing.T) {
	deps, _, _, _, _, host := greenDeps(t)
	host.proposal = port.Proposal{Number: 9}
	host.checkState = port.CheckRed
	w := mustWorker(t, deps) // no WithGreenDriver

	outcome, err := w.Run(context.Background(), newTestTask())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if outcome != OutcomeFailed {
		t.Fatalf("outcome = %v, want failed (base behavior, no driver)", outcome)
	}
}

// A nil driver passed to the option is ignored — the base path is never silently
// disabled.
func TestWithGreenDriver_NilIsIgnored(t *testing.T) {
	deps, _, _, _, _, host := greenDeps(t)
	host.checkState = port.CheckRed
	w := mustWorker(t, deps, WithGreenDriver(nil))
	if w.greenDriver != nil {
		t.Fatal("a nil driver must not be installed")
	}
	outcome, err := w.Run(context.Background(), newTestTask())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if outcome != OutcomeFailed {
		t.Fatalf("outcome = %v, want failed", outcome)
	}
}
