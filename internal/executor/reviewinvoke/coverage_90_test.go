package reviewinvoke

import (
	"context"
	"errors"
	"log/slog"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/adapters/harness/claude"
	"github.com/agenticlab-ai/pyxis/internal/executor/port"
	"github.com/agenticlab-ai/pyxis/internal/executor/reviewobligation"
	"github.com/agenticlab-ai/pyxis/internal/executor/worker"
	"github.com/agenticlab-ai/pyxis/internal/implreview"
	"github.com/agenticlab-ai/pyxis/internal/nativerouting"
	"github.com/agenticlab-ai/pyxis/internal/routingevidence"
)

type coverageBoundaryPermit struct{ err error }

func (p coverageBoundaryPermit) Authorize(routingevidence.BoundaryOperation, routingevidence.BoundaryBinding) error {
	return p.err
}

type coverageRoutePermit struct {
	coverageBoundaryPermit
	native       port.NativeLaunchSpec
	nativeErr    error
	ownership    port.OwnershipProof
	ownershipErr error
}

func (p coverageRoutePermit) NativeLaunch(nativerouting.Role) (port.NativeLaunchSpec, error) {
	return p.native, p.nativeErr
}

func (p coverageRoutePermit) RuntimeOwnershipProof() (port.OwnershipProof, error) {
	return p.ownership, p.ownershipErr
}

type coverageBasicRuntime struct{ inner *fakeRuntime }

func (r *coverageBasicRuntime) AuthCheck(ctx context.Context) error { return r.inner.AuthCheck(ctx) }
func (r *coverageBasicRuntime) Launch(ctx context.Context, spec port.LaunchSpec) (port.Handle, error) {
	return r.inner.Launch(ctx, spec)
}
func (r *coverageBasicRuntime) Await(ctx context.Context, handle port.Handle) (port.RunResult, error) {
	return r.inner.Await(ctx, handle)
}
func (r *coverageBasicRuntime) Kill(ctx context.Context, handle port.Handle) error {
	return r.inner.Kill(ctx, handle)
}

type coverageActivateRuntime struct {
	*fakeRuntime
	err error
}

func (r *coverageActivateRuntime) Activate(attempt string, generation port.OwnerGeneration, proof port.OwnershipProof) (port.LaunchCapability, error) {
	if r.err != nil {
		return port.LaunchCapability{}, r.err
	}
	return r.fakeRuntime.Activate(attempt, generation, proof)
}

type coverageProcessCoordinator struct {
	prepareErr  error
	completeErr error
}

type coverageNativeFailureRuntime struct {
	*claude.Adapter
	catalogErr error
	invalidCat bool
	buildErr   error
	captureErr error
	mismatch   bool
}

func (r *coverageNativeFailureRuntime) Catalog(role nativerouting.Role) (nativerouting.Catalog, nativerouting.QualificationSet, error) {
	if r.catalogErr != nil {
		return nativerouting.Catalog{}, nil, r.catalogErr
	}
	catalog, qualifications, err := r.Adapter.Catalog(role)
	if r.invalidCat {
		catalog.SchemaVersion = "invalid"
	}
	return catalog, qualifications, err
}

func (r *coverageNativeFailureRuntime) Build(selected nativerouting.SelectedProfile) (port.NativeLaunchSpec, error) {
	if r.buildErr != nil {
		return port.NativeLaunchSpec{}, r.buildErr
	}
	return r.Adapter.Build(selected)
}

func (r *coverageNativeFailureRuntime) CaptureNativeLaunch(spec port.NativeLaunchSpec) (port.LaunchCapture, error) {
	if r.captureErr != nil {
		return port.LaunchCapture{}, r.captureErr
	}
	capture, err := r.Adapter.CaptureNativeLaunch(spec)
	if r.mismatch {
		capture.NativeModel += "-different"
	}
	return capture, err
}

func (p coverageProcessCoordinator) Prepare(_ context.Context, subject implreview.ReviewSubject) (implreview.AdmittedProcess, error) {
	if p.prepareErr != nil {
		return implreview.AdmittedProcess{}, p.prepareErr
	}
	return implreview.AdmittedProcess{ReviewProcessID: "review-process-coverage", Subject: subject}, nil
}

func (p coverageProcessCoordinator) Complete(context.Context, implreview.AdmittedProcess, implreview.ProcessHandoff) (implreview.AggregateHandoff, error) {
	if p.completeErr != nil {
		return implreview.AggregateHandoff{}, p.completeErr
	}
	return implreview.AggregateHandoff{Status: implreview.HandoffMissing}, nil
}

func validCoverageRoutePermit() coverageRoutePermit {
	return coverageRoutePermit{
		native: port.NativeLaunchSpec{NativeProfileID: "review-maximum", Role: nativerouting.RoleReview, NativeModel: "review-model", NativeEffort: "max"},
		ownership: port.OwnershipProof{Kind: port.OwnershipProofResolution, Reference: "route-event", Digest: strings.Repeat("a", 64),
			AttemptID: "review-route-attempt", Harness: "claude", OwnerGeneration: 1},
	}
}

func TestCoverage90RoutedTerminalClassifiesEveryLaunchOutcome(t *testing.T) {
	capability := port.LaunchCapability{AttemptID: "attempt", OwnerGeneration: 2, CapabilityRef: "capability"}
	tests := []struct {
		name     string
		result   worker.ReviewResult
		launched bool
		runErr   error
		outcome  routingevidence.OutcomeClass
		cause    routingevidence.OutcomeCause
	}{
		{name: "cancelled before launch", runErr: context.Canceled, outcome: routingevidence.OutcomeBlocked, cause: routingevidence.CauseCancelled},
		{name: "failed before launch", runErr: errors.New("preflight"), outcome: routingevidence.OutcomeFailed, cause: routingevidence.CausePreflightFailed},
		{name: "verified", launched: true, result: worker.ReviewResult{Status: "verified"}, outcome: routingevidence.OutcomeVerified, cause: routingevidence.CauseTerminalVerified},
		{name: "cancelled after launch", launched: true, runErr: context.DeadlineExceeded, outcome: routingevidence.OutcomeBlocked, cause: routingevidence.CauseCancelled},
		{name: "worker failed", launched: true, result: worker.ReviewResult{Status: "failed", Err: errors.New("review failed")}, outcome: routingevidence.OutcomeFailed, cause: routingevidence.CauseWorkerFailed},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			terminal := routedReviewTerminal(tc.result, tc.launched, capability, tc.runErr)
			if terminal.Outcome != tc.outcome || terminal.Cause != tc.cause || terminal.AttemptID != capability.AttemptID {
				t.Fatalf("terminal = %+v", terminal)
			}
		})
	}
}

func TestCoverage90InvokeRejectsIncompleteRouteWorkspaceAndRuntimeState(t *testing.T) {
	task, proposal := port.Task{ID: "T5"}, port.Proposal{Number: 548, Branch: "feat/x"}
	tests := []struct {
		name   string
		mutate func(*Invoker)
		want   string
	}{
		{name: "incomplete route", mutate: func(i *Invoker) {
			i.route = func(context.Context, port.Task, string, string) (Route, error) { return Route{}, nil }
		}, want: "route is incomplete"},
		{name: "workspace prepare", mutate: func(i *Invoker) {
			i.prepare = func(context.Context, string) (string, func(), error) {
				return "", func() {}, errors.New("prepare failed")
			}
		}, want: "prepare failed"},
		{name: "runtime state parent", mutate: func(i *Invoker) {
			i.prepare = func(context.Context, string) (string, func(), error) {
				return filepath.Join(t.TempDir(), "missing", "worktree"), func() {}, nil
			}
		}, want: "create writable reviewer runtime state"},
		{name: "unsupported authentication backend", mutate: func(i *Invoker) {
			i.harness = "foreign"
		}, want: "prepare disposable reviewer authentication"},
		{name: "incomplete run reason", mutate: func(i *Invoker) {
			i.runtime = &fakeRuntime{result: port.RunResult{Incomplete: true, Output: "partial"}}
		}, want: "runtime reported an incomplete run"},
		{name: "process completion", mutate: func(i *Invoker) {
			i.runtime = &fakeRuntime{result: port.RunResult{Output: " "}}
			i.processes = coverageProcessCoordinator{completeErr: errors.New("complete failed")}
		}, want: "complete failed"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			i := lifecycleInvoker(t, &fakeRuntime{}, &fakeVerifier{}, time.Minute)
			tc.mutate(i)
			result := i.Invoke(context.Background(), task, proposal)
			if result.Err == nil || !strings.Contains(result.Err.Error(), tc.want) {
				t.Fatalf("Invoke = %+v, want error containing %q", result, tc.want)
			}
		})
	}
}

func TestCoverage90InvokeRejectsEachActivatedRouteBoundary(t *testing.T) {
	task, proposal := port.Task{ID: "T5"}, port.Proposal{Number: 548, Branch: "feat/x"}
	tests := []struct {
		name    string
		runtime port.AgentRuntime
		permit  routingevidence.BoundaryPermit
		outcome ReviewOutcome
		want    string
	}{
		{name: "nil permit", runtime: &fakeRuntime{}, outcome: func(context.Context, routingevidence.AttemptTerminal) error { return nil }, want: "returned no permit"},
		{name: "no fenced authority", runtime: &fakeRuntime{}, permit: coverageBoundaryPermit{}, outcome: func(context.Context, routingevidence.AttemptTerminal) error { return nil }, want: "no fenced launch authority"},
		{name: "no outcome recorder", runtime: &fakeRuntime{}, permit: validCoverageRoutePermit(), want: "no outcome recorder"},
		{name: "native launch read", runtime: &fakeRuntime{}, permit: func() coverageRoutePermit {
			p := validCoverageRoutePermit()
			p.nativeErr = errors.New("native unavailable")
			return p
		}(), outcome: func(context.Context, routingevidence.AttemptTerminal) error { return nil }, want: "native unavailable"},
		{name: "native launch mismatch", runtime: &fakeRuntime{}, permit: func() coverageRoutePermit {
			p := validCoverageRoutePermit()
			p.native.NativeModel = "different"
			return p
		}(), outcome: func(context.Context, routingevidence.AttemptTerminal) error { return nil }, want: "differs from review route"},
		{name: "ownership proof", runtime: &fakeRuntime{}, permit: func() coverageRoutePermit {
			p := validCoverageRoutePermit()
			p.ownershipErr = errors.New("ownership unavailable")
			return p
		}(), outcome: func(context.Context, routingevidence.AttemptTerminal) error { return nil }, want: "ownership unavailable"},
		{name: "unfenced runtime", runtime: &coverageBasicRuntime{inner: &fakeRuntime{}}, permit: validCoverageRoutePermit(), outcome: func(context.Context, routingevidence.AttemptTerminal) error { return nil }, want: "no fenced runtime"},
		{name: "activation failure", runtime: &coverageActivateRuntime{fakeRuntime: &fakeRuntime{}, err: errors.New("activation failed")}, permit: validCoverageRoutePermit(), outcome: func(context.Context, routingevidence.AttemptTerminal) error { return nil }, want: "activation failed"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			i := lifecycleInvoker(t, tc.runtime, &fakeVerifier{}, time.Minute)
			i.admitReview = func(context.Context, port.Task, string, string, Route) (routingevidence.BoundaryPermit, error) {
				return tc.permit, nil
			}
			i.completeReview = tc.outcome
			result := i.Invoke(context.Background(), task, proposal)
			if result.Err == nil || !strings.Contains(result.Err.Error(), tc.want) {
				t.Fatalf("Invoke = %+v, want error containing %q", result, tc.want)
			}
		})
	}
}

func TestCoverage90WorkspaceAndLocalFailureBranches(t *testing.T) {
	inv := &Invoker{repoDir: filepath.Join(t.TempDir(), "missing"), harness: "foreign", log: slog.New(slog.DiscardHandler)}
	if _, cleanup, err := inv.prepareReviewWorkspace(context.Background(), "revision"); err == nil {
		cleanup()
		t.Fatal("missing repository produced an isolated workspace")
	}
	if _, cleanup, err := inv.prepareLocalWorkspace(context.Background(), filepath.Join(t.TempDir(), "missing"), "branch", "revision"); err == nil {
		cleanup()
		t.Fatal("missing local repository produced an isolated workspace")
	}
	if _, cleanup, err := inv.prepareRuntimeState(t.Context(), filepath.Join(t.TempDir(), "worktree")); err == nil {
		cleanup()
		t.Fatal("unsupported backend produced reviewer runtime state")
	}
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := gitOut(cancelled, t.TempDir(), "status"); err == nil {
		t.Fatal("cancelled git command succeeded")
	}

	world := newLocalWorld(t)
	local := localInvoker(t, &fakeRuntime{}, world.repo)
	local.remote, local.mainline = "", ""
	revision, base, err := local.localRevision(context.Background(), world.worktree)
	if err != nil || revision != world.tip || base != world.base {
		t.Fatalf("default local revision = %q/%q, err=%v", revision, base, err)
	}
	if _, cleanup, err := local.prepareLocalWorkspace(context.Background(), filepath.Join(t.TempDir(), "missing"), world.branch, world.tip); err == nil {
		cleanup()
		t.Fatal("unfetchable local branch produced a workspace")
	}
	if _, cleanup, err := local.prepareLocalWorkspace(context.Background(), world.worktree, "", "not-a-revision"); err == nil {
		cleanup()
		t.Fatal("invalid local revision produced a workspace")
	}

	local.processes = coverageProcessCoordinator{prepareErr: errors.New("process admission failed")}
	result := local.ReviewLocal(context.Background(), port.Task{ID: "T-local"}, localWorkdir{dir: world.worktree, branch: world.branch}, 1)
	if result.Err == nil || !strings.Contains(result.Err.Error(), "process admission failed") {
		t.Fatalf("ReviewLocal = %+v", result)
	}
}

func TestCoverage90PairRepairPreflightPropagatesStoreFailure(t *testing.T) {
	repairer, _, _, _, lease := pairRepairFixture(t)
	defer func() { _ = lease.Close() }() // fixture teardown; the assertion below is the contract
	repairer.store = &coverageReviewStore{preflightErr: errors.New("store unavailable")}
	if err := repairer.Preflight(context.Background(), "o/r"); err == nil || !strings.Contains(err.Error(), "store unavailable") {
		t.Fatalf("Preflight error = %v", err)
	}
}

func TestCoverage90LocalReviewRejectsEveryPermitBoundary(t *testing.T) {
	world := newLocalWorld(t)
	work := localWorkdir{dir: world.worktree, branch: world.branch}
	task := port.Task{ID: "T-local", DemandID: "demand", Brief: "charter"}
	for _, tc := range []struct {
		name    string
		permit  routingevidence.BoundaryPermit
		outcome ReviewOutcome
		want    string
	}{
		{name: "nil permit", outcome: func(context.Context, routingevidence.AttemptTerminal) error { return nil }, want: "returned no permit"},
		{name: "authorization", permit: coverageBoundaryPermit{err: errors.New("denied")}, outcome: func(context.Context, routingevidence.AttemptTerminal) error { return nil }, want: "verify reviewer permit"},
		{name: "unfenced permit", permit: coverageBoundaryPermit{}, outcome: func(context.Context, routingevidence.AttemptTerminal) error { return nil }, want: "no fenced launch authority"},
		{name: "missing outcome recorder", permit: validCoverageRoutePermit(), want: "no outcome recorder"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			inv := localInvoker(t, &fakeRuntime{}, world.repo)
			inv.admitReview = func(context.Context, port.Task, string, string, Route) (routingevidence.BoundaryPermit, error) {
				return tc.permit, nil
			}
			inv.completeReview = tc.outcome
			result := inv.ReviewLocal(context.Background(), task, work, 1)
			if result.Err == nil || !strings.Contains(result.Err.Error(), tc.want) {
				t.Fatalf("ReviewLocal = %+v, want %q", result, tc.want)
			}
		})
	}
}

func TestCoverage90NativeRouteFailureBoundaries(t *testing.T) {
	adapter, err := claude.New(claude.WithProfiles(fixtureHarnessConfig(t).Profiles("claude")))
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name   string
		mutate func(*coverageNativeFailureRuntime, *port.Task)
		want   string
	}{
		{name: "catalog", mutate: func(r *coverageNativeFailureRuntime, _ *port.Task) { r.catalogErr = errors.New("catalog failed") }, want: "read claude review catalog"},
		{name: "invalid catalog", mutate: func(r *coverageNativeFailureRuntime, _ *port.Task) { r.invalidCat = true }, want: "validate claude review catalog"},
		{name: "unresolved", mutate: func(_ *coverageNativeFailureRuntime, task *port.Task) { task.DemandProfile.Capability = "impossible" }, want: "did not resolve"},
		{name: "build", mutate: func(r *coverageNativeFailureRuntime, _ *port.Task) { r.buildErr = errors.New("build failed") }, want: "build selected review profile"},
		{name: "capture", mutate: func(r *coverageNativeFailureRuntime, _ *port.Task) { r.captureErr = errors.New("capture failed") }, want: "capture selected review launch"},
		{name: "mismatch", mutate: func(r *coverageNativeFailureRuntime, _ *port.Task) { r.mismatch = true }, want: "differs from the adapter launch capture"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			runtime := &coverageNativeFailureRuntime{Adapter: adapter}
			task := coverageSemanticTask()
			tc.mutate(runtime, &task)
			inv := coverageInvoker(&coverageReviewStore{})
			inv.runtime = runtime
			if _, err := inv.resolveNativeRoute(task, "github:o/r#7", "abc123"); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("resolveNativeRoute error = %v, want %q", err, tc.want)
			}
		})
	}

	t.Run("successful route", func(t *testing.T) {
		runtime := &coverageNativeFailureRuntime{Adapter: adapter}
		inv := coverageInvoker(&coverageReviewStore{})
		inv.runtime = runtime
		route, err := inv.resolveNativeRoute(coverageSemanticTask(), "github:o/r#7", "abc123")
		if err != nil || route.NativeProfileID == "" {
			t.Fatalf("resolveNativeRoute = %+v, %v", route, err)
		}
	})
}

func TestCoverage90ReviewStoreSuccessAndMissingObligation(t *testing.T) {
	inv := coverageInvoker(&coverageReviewStore{})
	if err := inv.CompleteHandoff(context.Background(), worker.ReviewCapture{ProposalRef: "github:o/r#1", Revision: "abc", ObligationID: "o-1", Harness: "claude", Persisted: true}); err != nil {
		t.Fatalf("CompleteHandoff: %v", err)
	}

	store, err := reviewobligation.NewFileStore(filepath.Join(t.TempDir(), "review.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	inv = coverageInvoker(store)
	task, proposal := coverageSemanticTask(), port.Proposal{Number: 7}
	inv.captures[captureKey(task, proposal)] = worker.ReviewCapture{
		ProposalRef: "github:o/r#7", Revision: "abc123", ObligationID: "missing", Harness: "claude", Persisted: true,
	}
	result := inv.Invoke(context.Background(), task, proposal)
	if result.Err == nil || !strings.Contains(result.Err.Error(), "disappeared") {
		t.Fatalf("Invoke = %+v", result)
	}
}
