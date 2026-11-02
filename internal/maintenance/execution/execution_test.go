package execution

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	_ "github.com/agenticlab-ai/pyxis/internal/adapters/harness/claude"
	_ "github.com/agenticlab-ai/pyxis/internal/adapters/harness/codex"
	_ "github.com/agenticlab-ai/pyxis/internal/adapters/harness/cursor"
	_ "github.com/agenticlab-ai/pyxis/internal/adapters/harness/opencode"
	"github.com/agenticlab-ai/pyxis/internal/entity"
	"github.com/agenticlab-ai/pyxis/internal/executiondemand"
	"github.com/agenticlab-ai/pyxis/internal/executor/port"
	executorruntime "github.com/agenticlab-ai/pyxis/internal/executor/runtime"
	"github.com/agenticlab-ai/pyxis/internal/gitrefstore"
	"github.com/agenticlab-ai/pyxis/internal/harnessconfig/harnessconfigtest"
	"github.com/agenticlab-ai/pyxis/internal/maintenance/admission"
	"github.com/agenticlab-ai/pyxis/internal/maintenance/claim"
	"github.com/agenticlab-ai/pyxis/internal/maintenance/installation"
	"github.com/agenticlab-ai/pyxis/internal/maintenance/jobs"
	"github.com/agenticlab-ai/pyxis/internal/maintenance/workspace"
	"github.com/agenticlab-ai/pyxis/internal/nativerouting"
)

type testHandle struct{}

func (testHandle) RuntimeHandle() {}

type nativeRuntime interface {
	port.AgentRuntime
	port.CatalogProvider
	port.LaunchBuilder
	port.NativeLaunchCapturer
	nativerouting.NativeCapability
}

type runtimeFixture struct {
	nativeRuntime
	harness  entity.Harness
	capture  port.LaunchCapture
	launches int
	kills    int
	spec     port.LaunchSpec
	launch   func(context.Context, port.LaunchSpec) (port.Handle, error)
	await    func(context.Context, port.Handle) (port.RunResult, error)
	kill     func(context.Context, port.Handle) error
}

func (r *runtimeFixture) Launch(ctx context.Context, spec port.LaunchSpec) (port.Handle, error) {
	r.launches++
	r.spec = spec
	var handle port.Handle = testHandle{}
	var err error
	if r.launch != nil {
		handle, err = r.launch(ctx, spec)
	}
	if handle != nil {
		finish := port.ObserveInvocation(ctx, r.harness, spec.Purpose, r.capture)
		if finish != nil {
			finish(port.RunResult{}, err)
		}
	}
	return handle, err
}

func (r *runtimeFixture) Await(ctx context.Context, handle port.Handle) (port.RunResult, error) {
	if r.await != nil {
		return r.await(ctx, handle)
	}
	return port.RunResult{Tokens: 7, Turns: 1}, nil
}

func (r *runtimeFixture) Kill(ctx context.Context, handle port.Handle) error {
	r.kills++
	if r.kill != nil {
		return r.kill(ctx, handle)
	}
	return nil
}

type registryFixture struct{ runtime *runtimeFixture }

func (r registryFixture) ResolveConfigured(harness entity.Harness, profiles map[nativerouting.Role][]nativerouting.ProfileSpec) (port.AgentRuntime, error) {
	resolved, err := executorruntime.LiveRegistry().ResolveConfigured(harness, profiles)
	if err != nil {
		return nil, err
	}
	native, ok := resolved.(nativeRuntime)
	if !ok {
		return nil, errors.New("fixture runtime lacks native APIs")
	}
	r.runtime.nativeRuntime = native
	r.runtime.harness = harness
	return r.runtime, nil
}

type invocationFixture struct {
	harness entity.Harness
	purpose port.LaunchPurpose
	capture port.LaunchCapture
	result  port.RunResult
	err     error
}

func (o *invocationFixture) Launched(harness entity.Harness, purpose port.LaunchPurpose, capture port.LaunchCapture) func(port.RunResult, error) {
	o.harness, o.purpose, o.capture = harness, purpose, capture
	return func(result port.RunResult, err error) { o.result, o.err = result, err }
}

type authorityFixture struct {
	ready      installation.ReadyBinding
	validates  int
	failAt     int
	guardCalls int
}

func (a *authorityFixture) Ready(context.Context) (installation.ReadyBinding, error) {
	return a.ready, nil
}

func (a *authorityFixture) ValidateReady(context.Context, installation.ReadyBinding) error {
	a.validates++
	if a.validates == a.failAt {
		return installation.ErrGenerationReplaced
	}
	return nil
}

func (a *authorityFixture) GuardReady(ctx context.Context, ready installation.ReadyBinding, run func() error) error {
	a.guardCalls++
	if err := a.ValidateReady(ctx, ready); err != nil {
		return err
	}
	return run()
}

type controlFixture struct{}

func (controlFixture) RefStore(string) (*gitrefstore.Store, error) { return nil, errors.New("unused") }
func (controlFixture) Close() error                                { return nil }

type publisherFixture struct{ document jobs.Document }

func (p publisherFixture) Capture(context.Context, jobs.SourceBinding) (admission.Published, error) {
	data, err := jobs.Encode(p.document)
	return admission.Published{Commit: strings.Repeat("a", 40), Jobs: data, Config: []byte(harnessconfigtest.YAML), Control: controlFixture{}}, err
}

type claimerFixture struct {
	result      claim.ClaimResult
	err         error
	transition  error
	calls       int
	transitions int
	terminal    claim.Terminal
	until       time.Time
}

func (c *claimerFixture) Transition(_ context.Context, owned claim.Record, terminal claim.Terminal) (claim.Record, error) {
	c.transitions++
	c.terminal = terminal
	if c.transition != nil {
		return claim.Record{}, c.transition
	}
	owned.Outcome, owned.FinishedAt, owned.DurationMS = terminal.Outcome, &terminal.FinishedAt, terminal.DurationMS
	owned.Summary, owned.Cause, owned.Evidence = terminal.Summary, terminal.Cause, terminal.Evidence
	return owned, nil
}

func (c *claimerFixture) Acquire(ctx context.Context, prepared admission.Prepared) (claim.ClaimResult, error) {
	c.calls++
	c.until, _ = ctx.Deadline()
	if c.err != nil {
		return claim.ClaimResult{}, c.err
	}
	if c.result.Disposition == claim.DispositionOwned {
		err := prepared.GuardReady(ctx, func() error { return nil })
		if err != nil {
			return claim.ClaimResult{}, err
		}
	}
	return c.result, nil
}

type preparerFixture struct {
	result workspace.Prepared
	err    error
	wait   bool
	calls  int
	until  time.Time
}

type lifecycleFixture struct {
	fail         Boundary
	failures     int
	observations []Observation
}

func (l *lifecycleFixture) Observe(_ context.Context, observed Observation) error {
	l.observations = append(l.observations, observed)
	if observed.Boundary == l.fail && l.failures > 0 {
		l.failures--
		return errors.New("journal unavailable")
	}
	return nil
}

func (p *preparerFixture) Prepare(ctx context.Context, _ admission.Prepared, _ claim.ClaimResult) (workspace.Prepared, error) {
	p.calls++
	p.until, _ = ctx.Deadline()
	if p.wait {
		<-ctx.Done()
		return workspace.Prepared{}, ctx.Err()
	}
	return p.result, p.err
}

func preparedFixture(t *testing.T, harness entity.Harness, runtime *runtimeFixture, authority *authorityFixture) admission.Prepared {
	t.Helper()
	job := jobs.Job{ID: "daily", Enabled: true, Schedule: jobs.Schedule{Cron: "0 2 * * *", Timezone: "America/Los_Angeles"}, Timeout: "90m", Prompt: "literal maintenance prompt",
		Runner: jobs.Runner{Harness: harness, Routing: executiondemand.ProfileMinimum{Capability: executiondemand.CapabilityStrong, Deliberation: executiondemand.DeliberationDeep, Context: executiondemand.ContextExtended, Assurance: executiondemand.AssuranceElevated}}}
	document := jobs.Document{SchemaVersion: jobs.SchemaVersion, Source: jobs.SourceBinding{Remote: "origin", Branch: "main"}, Jobs: []jobs.Job{job}}
	schedules, err := installation.ScheduleFingerprints(document)
	if err != nil {
		t.Fatal(err)
	}
	authority.ready = installation.ReadyBinding{Generation: installation.InstallationGeneration(strings.Repeat("1", 32)), Source: document.Source, Schedules: schedules}
	started := time.Now().UTC()
	result, err := (admission.Service{Authority: authority, Publisher: publisherFixture{document}, Registry: registryFixture{runtime},
		ProcessSupport: func(context.Context) error { return nil }, Authenticate: func(context.Context, port.AgentRuntime) error { return nil },
	}).Prepare(t.Context(), admission.Request{JobID: job.ID, StartedUTC: started})
	if err != nil {
		t.Fatal(err)
	}
	runtime.capture = result.Prepared.Route.Capture
	t.Cleanup(func() { _ = result.Prepared.Close() })
	return *result.Prepared
}

func owned(prepared admission.Prepared) claim.ClaimResult {
	return claim.ClaimResult{Disposition: claim.DispositionOwned, Record: claim.Record{JobID: prepared.Job.ID, JobDate: prepared.JobDate, OwnerNonce: strings.Repeat("b", 64)}}
}

func TestRunPreservesFrozenNativeLaunchAndFreshImplementationMetadata(t *testing.T) {
	for _, harness := range []entity.Harness{entity.Claude, entity.Codex, entity.Cursor, entity.OpenCode} {
		t.Run(harness.String(), func(t *testing.T) {
			runtime, authority := &runtimeFixture{}, &authorityFixture{}
			prepared := preparedFixture(t, harness, runtime, authority)
			claims := &claimerFixture{result: owned(prepared)}
			spaces := &preparerFixture{result: workspace.Prepared{Workdir: "/private/source", RuntimeStateDir: "/private/state", HeadBranch: "maintenance/attempt-owned"}}
			result, err := (Service{Claims: claims, Workspaces: spaces}).Run(t.Context(), prepared)
			if err != nil || result.Outcome != OutcomeSucceeded || result.Workspace == nil || result.Workspace.Workdir != spaces.result.Workdir || !result.Launched || !result.Quiescent || runtime.launches != 1 || runtime.kills != 0 {
				t.Fatalf("result=%+v err=%v launches=%d kills=%d", result, err, runtime.launches, runtime.kills)
			}
			native := prepared.Route.Launch
			if runtime.spec.Brief != prepared.Job.Prompt || runtime.spec.Purpose != port.LaunchPurposeImplementation || runtime.spec.Workdir != spaces.result.Workdir || runtime.spec.RuntimeStateDir != spaces.result.RuntimeStateDir || runtime.spec.HeadBranch != spaces.result.HeadBranch || runtime.spec.Session != nil || runtime.spec.Model != native.NativeModel || runtime.spec.Effort != native.NativeEffort {
				t.Fatalf("launch spec = %+v, native=%+v", runtime.spec, native)
			}
			if !prepared.Route.Capture.MatchesSelected(prepared.Selection) || claims.until != prepared.DeadlineUTC || spaces.until != prepared.DeadlineUTC {
				t.Fatalf("fidelity/deadline capture=%+v selection=%+v claim=%v prep=%v want=%v", prepared.Route.Capture, prepared.Selection, claims.until, spaces.until, prepared.DeadlineUTC)
			}
		})
	}
}

func TestRegisteredAdaptersTransportLiteralPromptWithExactNativeSelection(t *testing.T) {
	toolDir := t.TempDir()
	fixture := filepath.Join(toolDir, "native-fixture")
	if output, err := exec.CommandContext(t.Context(), "go", "build", "-o", fixture, "./testdata/native").CombinedOutput(); err != nil {
		t.Fatalf("build native fixture: %v: %s", err, output)
	}
	for _, name := range []string{"claude", "codex", "cursor-agent", "opencode"} {
		if err := os.Symlink(fixture, filepath.Join(toolDir, name)); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", toolDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	for _, harness := range []entity.Harness{entity.Claude, entity.Codex, entity.Cursor, entity.OpenCode} {
		t.Run(harness.String(), func(t *testing.T) {
			authority := &authorityFixture{}
			job := jobs.Job{ID: "daily", Enabled: true, Schedule: jobs.Schedule{Cron: "0 2 * * *", Timezone: "America/Los_Angeles"}, Timeout: "90m", Prompt: "literal-maintenance-prompt",
				Runner: jobs.Runner{Harness: harness, Routing: executiondemand.ProfileMinimum{Capability: executiondemand.CapabilityStrong, Deliberation: executiondemand.DeliberationDeep, Context: executiondemand.ContextExtended, Assurance: executiondemand.AssuranceElevated}}}
			document := jobs.Document{SchemaVersion: jobs.SchemaVersion, Source: jobs.SourceBinding{Remote: "origin", Branch: "main"}, Jobs: []jobs.Job{job}}
			schedules, err := installation.ScheduleFingerprints(document)
			if err != nil {
				t.Fatal(err)
			}
			authority.ready = installation.ReadyBinding{Generation: installation.InstallationGeneration(strings.Repeat("2", 32)), Source: document.Source, Schedules: schedules}
			admitted, err := (admission.Service{Authority: authority, Publisher: publisherFixture{document}, Registry: executorruntime.LiveRegistry(),
				ProcessSupport: func(context.Context) error { return nil }, Authenticate: func(context.Context, port.AgentRuntime) error { return nil },
			}).Prepare(t.Context(), admission.Request{JobID: job.ID, StartedUTC: time.Now().UTC()})
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = admitted.Prepared.Close() }()
			runtime, native, err := admitted.Prepared.ExecutionRoute()
			if err != nil {
				t.Fatal(err)
			}
			work := t.TempDir()
			if output, err := exec.Command("git", "init", "-q", "-b", "maintenance/attempt-fixture", work).CombinedOutput(); err != nil {
				t.Fatalf("git init: %v: %s", err, output)
			}
			if err := os.WriteFile(filepath.Join(work, "README.md"), []byte("fixture\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			for _, args := range [][]string{{"-C", work, "add", "README.md"}, {"-C", work, "-c", "user.name=fixture", "-c", "user.email=fixture@example.test", "commit", "-q", "-m", "fixture"}} {
				if output, err := exec.Command("git", args...).CombinedOutput(); err != nil {
					t.Fatalf("git %v: %v: %s", args, err, output)
				}
			}
			state, _, err := executorruntime.PrepareTaskState(t.Context(), work, harness)
			if err != nil {
				t.Fatal(err)
			}
			observer := &invocationFixture{}
			ctx := port.WithInvocationObserver(t.Context(), observer)
			handle, err := runtime.Launch(ctx, port.LaunchSpec{Brief: job.Prompt, Purpose: port.LaunchPurposeImplementation,
				Workdir: work, RuntimeStateDir: state, HeadBranch: "maintenance/attempt-fixture", Model: native.NativeModel, Effort: native.NativeEffort})
			if err != nil {
				t.Fatal(err)
			}
			result, err := runtime.Await(ctx, handle)
			if err != nil || result.Incomplete {
				t.Fatalf("await result=%+v err=%v", result, err)
			}
			captured, err := os.ReadFile(filepath.Join(work, ".native-capture"))
			if err != nil || !strings.Contains(string(captured), job.Prompt) {
				t.Fatalf("native prompt capture=%q err=%v", captured, err)
			}
			if observer.harness != harness || observer.purpose != port.LaunchPurposeImplementation || !observer.capture.MatchesSelected(admitted.Prepared.Selection) || observer.err != nil {
				t.Fatalf("observed harness=%s purpose=%s capture=%+v selection=%+v err=%v", observer.harness, observer.purpose, observer.capture, admitted.Prepared.Selection, observer.err)
			}
		})
	}
}

func TestRunRefusesUnsupportedDeliveryAndExpiredDeadlineBeforeClaim(t *testing.T) {
	for _, test := range []struct {
		name string
		edit func(*admission.Prepared)
		want error
	}{
		{"delivery", func(p *admission.Prepared) {
			p.Job.Delivery = &jobs.Delivery{Mode: "draft-pr", Branch: "maintenance/daily"}
		}, ErrUnsupportedDelivery},
		{"expired", func(p *admission.Prepared) { p.DeadlineUTC = time.Now().Add(-time.Second) }, context.DeadlineExceeded},
	} {
		t.Run(test.name, func(t *testing.T) {
			runtime, authority := &runtimeFixture{}, &authorityFixture{}
			prepared := preparedFixture(t, entity.Codex, runtime, authority)
			test.edit(&prepared)
			claims, spaces := &claimerFixture{}, &preparerFixture{}
			result, err := (Service{Claims: claims, Workspaces: spaces}).Run(t.Context(), prepared)
			if !errors.Is(err, test.want) || result.Outcome != OutcomeRefused || claims.calls != 0 || spaces.calls != 0 || runtime.launches != 0 {
				t.Fatalf("result=%+v err=%v claim=%d prep=%d launch=%d", result, err, claims.calls, spaces.calls, runtime.launches)
			}
		})
	}
	runtime, authority := &runtimeFixture{}, &authorityFixture{}
	prepared := preparedFixture(t, entity.Codex, runtime, authority)
	claims, spaces := &claimerFixture{}, &preparerFixture{}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	result, err := (Service{Claims: claims, Workspaces: spaces}).Run(ctx, prepared)
	if !errors.Is(err, context.Canceled) || result.Outcome != OutcomeRefused || claims.calls != 0 || runtime.launches != 0 {
		t.Fatalf("preclaim cancellation result=%+v err=%v claims=%d launches=%d", result, err, claims.calls, runtime.launches)
	}
	if _, err := (Service{}).Run(t.Context(), admission.Prepared{}); err == nil {
		t.Fatal("empty execution service succeeded")
	}
	if _, err := (Service{Claims: claims, Workspaces: spaces}).Run(t.Context(), admission.Prepared{Job: jobs.Job{Prompt: "brief"}}); err == nil {
		t.Fatal("missing captured route succeeded")
	}
	prepared = preparedFixture(t, entity.Codex, runtime, authority)
	result, err = (Service{Claims: claims, Workspaces: spaces, Now: func() time.Time { return prepared.DeadlineUTC }}).Run(t.Context(), prepared)
	if !errors.Is(err, context.DeadlineExceeded) || result.Outcome != OutcomeRefused {
		t.Fatalf("injected expired clock result=%+v err=%v", result, err)
	}
}

func TestRunConsumesOnlyPositiveOwnershipAndNeverRetries(t *testing.T) {
	for _, test := range []struct {
		name     string
		claim    claim.ClaimResult
		claimErr error
		outcome  Outcome
		wantCode string
	}{
		{"consumed failed", claim.ClaimResult{Disposition: claim.DispositionConsumed, Record: claim.Record{JobID: "daily", Outcome: claim.OutcomeFailed}}, nil, OutcomeConsumed, "date_consumed_failed"},
		{"uncertain", claim.ClaimResult{}, errors.New("claim acknowledgement lost"), OutcomeUnproved, "claim_unproved"},
	} {
		t.Run(test.name, func(t *testing.T) {
			runtime, authority := &runtimeFixture{}, &authorityFixture{}
			prepared := preparedFixture(t, entity.Codex, runtime, authority)
			claims, spaces := &claimerFixture{result: test.claim, err: test.claimErr}, &preparerFixture{}
			lifecycle := &lifecycleFixture{}
			result, err := (Service{Claims: claims, Workspaces: spaces, Lifecycle: lifecycle}).Run(t.Context(), prepared)
			if (err != nil) != (test.claimErr != nil) || result.Outcome != test.outcome || claims.calls != 1 || spaces.calls != 0 || runtime.launches != 0 {
				t.Fatalf("result=%+v err=%v claim=%d prep=%d launch=%d", result, err, claims.calls, spaces.calls, runtime.launches)
			}
			if got := lifecycle.observations[len(lifecycle.observations)-1].CauseCode; got != test.wantCode {
				t.Fatalf("terminal cause code = %q, want %q", got, test.wantCode)
			}
		})
	}
}

func TestRunDeadlineIncludesPreparationAndFinalReadiness(t *testing.T) {
	t.Run("slow preparation", func(t *testing.T) {
		runtime, authority := &runtimeFixture{}, &authorityFixture{}
		prepared := preparedFixture(t, entity.Codex, runtime, authority)
		prepared.DeadlineUTC = time.Now().Add(20 * time.Millisecond)
		claims, spaces := &claimerFixture{result: owned(prepared)}, &preparerFixture{wait: true}
		result, err := (Service{Claims: claims, Workspaces: spaces}).Run(t.Context(), prepared)
		if !errors.Is(err, context.DeadlineExceeded) || result.Outcome != OutcomeTimedOut || runtime.launches != 0 || !result.Retained || claims.until != spaces.until {
			t.Fatalf("result=%+v err=%v claim deadline=%v prep=%v launch=%d", result, err, claims.until, spaces.until, runtime.launches)
		}
	})
	t.Run("ready changed before start", func(t *testing.T) {
		runtime, authority := &runtimeFixture{}, &authorityFixture{failAt: 3}
		prepared := preparedFixture(t, entity.Codex, runtime, authority)
		claims := &claimerFixture{result: owned(prepared)}
		spaces := &preparerFixture{result: workspace.Prepared{Workdir: "/private/source", RuntimeStateDir: "/private/state", HeadBranch: "maintenance/attempt-owned"}}
		result, err := (Service{Claims: claims, Workspaces: spaces}).Run(t.Context(), prepared)
		if !errors.Is(err, installation.ErrGenerationReplaced) || result.Outcome != OutcomeFailed || runtime.launches != 0 || authority.guardCalls != 2 {
			t.Fatalf("result=%+v err=%v launches=%d guards=%d", result, err, runtime.launches, authority.guardCalls)
		}
	})
}

func TestRunLaunchRefusalConsumesDateWithoutFallback(t *testing.T) {
	runtime, authority := &runtimeFixture{launch: func(context.Context, port.LaunchSpec) (port.Handle, error) {
		return nil, errors.New("native start refused")
	}}, &authorityFixture{}
	prepared := preparedFixture(t, entity.Codex, runtime, authority)
	result, err := (Service{Claims: &claimerFixture{result: owned(prepared)}, Workspaces: &preparerFixture{result: workspace.Prepared{Workdir: "/private/source", RuntimeStateDir: "/private/state", HeadBranch: "maintenance/attempt-owned"}}}).Run(t.Context(), prepared)
	if err == nil || result.Outcome != OutcomeFailed || result.Launched || !result.Retained || runtime.launches != 1 || runtime.kills != 0 {
		t.Fatalf("result=%+v err=%v launches=%d kills=%d", result, err, runtime.launches, runtime.kills)
	}
}

func TestRunRejectsMissingHandleAndIncompleteNativeResult(t *testing.T) {
	for _, test := range []struct {
		name    string
		launch  func(context.Context, port.LaunchSpec) (port.Handle, error)
		await   func(context.Context, port.Handle) (port.RunResult, error)
		outcome Outcome
		wantErr bool
	}{
		{"missing handle", func(context.Context, port.LaunchSpec) (port.Handle, error) { return nil, nil }, nil, OutcomeIncomplete, true},
		{"incomplete result", nil, func(context.Context, port.Handle) (port.RunResult, error) {
			return port.RunResult{Incomplete: true, Reason: "native limit"}, nil
		}, OutcomeIncomplete, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			runtime, authority := &runtimeFixture{launch: test.launch, await: test.await}, &authorityFixture{}
			prepared := preparedFixture(t, entity.Codex, runtime, authority)
			result, err := (Service{Claims: &claimerFixture{result: owned(prepared)}, Workspaces: &preparerFixture{result: workspace.Prepared{Workdir: "/private/source", RuntimeStateDir: "/private/state", HeadBranch: "maintenance/attempt-owned"}}}).Run(t.Context(), prepared)
			if (err != nil) != test.wantErr || result.Outcome != test.outcome || runtime.launches != 1 {
				t.Fatalf("result=%+v err=%v launches=%d", result, err, runtime.launches)
			}
		})
	}
}

func TestRunActualLaunchMismatchCannotBecomeSuccess(t *testing.T) {
	runtime, authority := &runtimeFixture{}, &authorityFixture{}
	prepared := preparedFixture(t, entity.Codex, runtime, authority)
	runtime.capture.NativeModel = "different-model"
	claims := &claimerFixture{result: owned(prepared)}
	result, err := (Service{Claims: claims, Workspaces: &preparerFixture{result: workspace.Prepared{Workdir: "/private/source", RuntimeStateDir: "/private/state", HeadBranch: "maintenance/attempt-owned"}}}).Run(t.Context(), prepared)
	if err == nil || result.Outcome != OutcomeFailed || result.Capture == nil || result.Capture.NativeModel != "different-model" || result.Shared == nil || result.Shared.Outcome != claim.OutcomeFailed {
		t.Fatalf("result=%+v err=%v", result, err)
	}
}

func TestRunCancellationAndUnprovedQuiescenceRemainIncomplete(t *testing.T) {
	t.Run("failed kill preserves await completion", func(t *testing.T) {
		ctx, cancel := context.WithCancelCause(t.Context())
		defer cancel(nil)
		cause := errors.New("operator canceled invocation")
		nativeErr := errors.New("native completion failed")
		killErr := errors.New("runtime kill failed")
		awaitStarted, killStarted, awaitReturned := make(chan struct{}), make(chan struct{}), make(chan struct{})
		usage := port.RunResult{Tokens: 13, Turns: 2, ToolCalls: 5, TokensObserved: true, TurnsObserved: true, ToolCallsObserved: true, Reason: "native limit", Output: "partial output", SessionID: "native-session"}
		runtime, authority := &runtimeFixture{}, &authorityFixture{}
		runtime.launch = func(context.Context, port.LaunchSpec) (port.Handle, error) {
			cancel(cause)
			return testHandle{}, nil
		}
		runtime.await = func(context.Context, port.Handle) (port.RunResult, error) {
			close(awaitStarted)
			<-killStarted
			defer close(awaitReturned)
			return usage, nativeErr
		}
		runtime.kill = func(context.Context, port.Handle) error {
			<-awaitStarted
			close(killStarted)
			<-awaitReturned
			return killErr
		}
		prepared := preparedFixture(t, entity.Codex, runtime, authority)
		claims := &claimerFixture{result: owned(prepared)}
		spaces := &preparerFixture{result: workspace.Prepared{Workdir: "/private/source", RuntimeStateDir: "/private/state", HeadBranch: "maintenance/attempt-owned"}}
		result, err := (Service{Claims: claims, Workspaces: spaces, CleanupTimeout: time.Second}).Run(ctx, prepared)
		for _, want := range []error{cause, context.Canceled, nativeErr, killErr, port.ErrNotQuiescent} {
			if !errors.Is(err, want) {
				t.Errorf("error %v does not preserve %v", err, want)
			}
		}
		if result.Usage != usage || result.Outcome != OutcomeIncomplete || result.Quiescent || !result.Retained || !result.Launched || claims.calls != 1 || runtime.launches != 1 || runtime.kills != 1 {
			t.Fatalf("result=%+v err=%v claims=%d launches=%d kills=%d", result, err, claims.calls, runtime.launches, runtime.kills)
		}
	})
	t.Run("caller cancellation uses bounded kill and preserves cause", func(t *testing.T) {
		stopped := make(chan struct{})
		runtime, authority := &runtimeFixture{}, &authorityFixture{}
		runtime.await = func(ctx context.Context, _ port.Handle) (port.RunResult, error) {
			<-stopped
			return port.RunResult{}, ctx.Err()
		}
		runtime.kill = func(context.Context, port.Handle) error { close(stopped); return nil }
		prepared := preparedFixture(t, entity.Codex, runtime, authority)
		claims := &claimerFixture{result: owned(prepared)}
		spaces := &preparerFixture{result: workspace.Prepared{Workdir: "/private/source", RuntimeStateDir: "/private/state", HeadBranch: "maintenance/attempt-owned"}}
		ctx, cancel := context.WithCancelCause(t.Context())
		cause := errors.New("operator canceled invocation")
		time.AfterFunc(10*time.Millisecond, func() { cancel(cause) })
		result, err := (Service{Claims: claims, Workspaces: spaces}).Run(ctx, prepared)
		if !errors.Is(err, cause) || result.Outcome != OutcomeFailed || !result.Quiescent || runtime.launches != 1 || runtime.kills != 1 {
			t.Fatalf("result=%+v err=%v launches=%d kills=%d", result, err, runtime.launches, runtime.kills)
		}
	})
	t.Run("await cannot acknowledge successful kill", func(t *testing.T) {
		stopped := make(chan struct{})
		runtime, authority := &runtimeFixture{}, &authorityFixture{}
		runtime.await = func(context.Context, port.Handle) (port.RunResult, error) {
			<-stopped
			return port.RunResult{}, port.ErrNotQuiescent
		}
		runtime.kill = func(context.Context, port.Handle) error { close(stopped); return nil }
		prepared := preparedFixture(t, entity.Codex, runtime, authority)
		prepared.DeadlineUTC = time.Now().Add(15 * time.Millisecond)
		result, err := (Service{Claims: &claimerFixture{result: owned(prepared)}, Workspaces: &preparerFixture{result: workspace.Prepared{Workdir: "/private/source", RuntimeStateDir: "/private/state", HeadBranch: "maintenance/attempt-owned"}}, CleanupTimeout: time.Second}).Run(t.Context(), prepared)
		if !errors.Is(err, port.ErrNotQuiescent) || result.Outcome != OutcomeIncomplete || result.Quiescent {
			t.Fatalf("result=%+v err=%v", result, err)
		}
	})
	t.Run("kill without await acknowledgement reaches cleanup bound", func(t *testing.T) {
		release := make(chan struct{})
		defer close(release)
		runtime, authority := &runtimeFixture{}, &authorityFixture{}
		runtime.await = func(context.Context, port.Handle) (port.RunResult, error) { <-release; return port.RunResult{}, nil }
		prepared := preparedFixture(t, entity.Codex, runtime, authority)
		prepared.DeadlineUTC = time.Now().Add(10 * time.Millisecond)
		claims := &claimerFixture{result: owned(prepared)}
		var result Result
		var err error
		returned := make(chan struct{})
		go func() {
			result, err = (Service{Claims: claims, Workspaces: &preparerFixture{result: workspace.Prepared{Workdir: "/private/source", RuntimeStateDir: "/private/state", HeadBranch: "maintenance/attempt-owned"}}, CleanupTimeout: 5 * time.Millisecond}).Run(t.Context(), prepared)
			close(returned)
		}()
		select {
		case <-returned:
		case <-time.After(time.Second):
			t.Fatal("missing Await acknowledgement exceeded cleanup bound")
		}
		if !errors.Is(err, port.ErrNotQuiescent) || result.Outcome != OutcomeIncomplete || result.Quiescent || !result.Retained || !result.Launched || claims.calls != 1 || runtime.launches != 1 || runtime.kills != 1 {
			t.Fatalf("result=%+v err=%v claims=%d launches=%d kills=%d", result, err, claims.calls, runtime.launches, runtime.kills)
		}
	})
	t.Run("parent exit with surviving descendants", func(t *testing.T) {
		runtime, authority := &runtimeFixture{await: func(context.Context, port.Handle) (port.RunResult, error) {
			return port.RunResult{}, port.ErrNotQuiescent
		}}, &authorityFixture{}
		prepared := preparedFixture(t, entity.Codex, runtime, authority)
		result, err := (Service{Claims: &claimerFixture{result: owned(prepared)}, Workspaces: &preparerFixture{result: workspace.Prepared{Workdir: "/private/source", RuntimeStateDir: "/private/state", HeadBranch: "maintenance/attempt-owned"}}}).Run(t.Context(), prepared)
		if !errors.Is(err, port.ErrNotQuiescent) || result.Outcome != OutcomeIncomplete || result.Quiescent || !result.Retained || runtime.launches != 1 {
			t.Fatalf("result=%+v err=%v launches=%d", result, err, runtime.launches)
		}
	})
	t.Run("failed kill is bounded and unproved", func(t *testing.T) {
		stopped := make(chan struct{})
		runtime, authority := &runtimeFixture{}, &authorityFixture{}
		runtime.await = func(context.Context, port.Handle) (port.RunResult, error) { <-stopped; return port.RunResult{}, nil }
		runtime.kill = func(context.Context, port.Handle) error { close(stopped); return errors.New("descendants survived") }
		prepared := preparedFixture(t, entity.Codex, runtime, authority)
		prepared.DeadlineUTC = time.Now().Add(15 * time.Millisecond)
		result, err := (Service{Claims: &claimerFixture{result: owned(prepared)}, Workspaces: &preparerFixture{result: workspace.Prepared{Workdir: "/private/source", RuntimeStateDir: "/private/state", HeadBranch: "maintenance/attempt-owned"}}, CleanupTimeout: 50 * time.Millisecond}).Run(t.Context(), prepared)
		if !errors.Is(err, port.ErrNotQuiescent) || result.Outcome != OutcomeIncomplete || result.Quiescent || runtime.launches != 1 || runtime.kills != 1 {
			t.Fatalf("result=%+v err=%v launches=%d kills=%d", result, err, runtime.launches, runtime.kills)
		}
	})
}

func TestRunPreservesCancellationCauseAndNativeErrorInEitherCompletionOrder(t *testing.T) {
	for _, waitForKill := range []bool{false, true} {
		name := "await cancels and returns"
		if waitForKill {
			name = "await returns after successful kill"
		}
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithCancelCause(t.Context())
			defer cancel(nil)
			cause := errors.New("operator canceled this invocation")
			nativeErr := errors.New("native execution failed during cancellation")
			stopped := make(chan struct{})
			runtime, authority := &runtimeFixture{}, &authorityFixture{}
			runtime.launch = func(context.Context, port.LaunchSpec) (port.Handle, error) {
				if waitForKill {
					cancel(cause)
				}
				return testHandle{}, nil
			}
			runtime.await = func(attempt context.Context, _ port.Handle) (port.RunResult, error) {
				if waitForKill {
					<-stopped
				} else {
					cancel(cause)
				}
				return port.RunResult{Tokens: 13, Turns: 2}, errors.Join(attempt.Err(), nativeErr)
			}
			runtime.kill = func(context.Context, port.Handle) error {
				if waitForKill {
					close(stopped)
				}
				return nil
			}
			prepared := preparedFixture(t, entity.Codex, runtime, authority)
			claims := &claimerFixture{result: owned(prepared)}
			spaces := &preparerFixture{result: workspace.Prepared{Workdir: "/private/source", RuntimeStateDir: "/private/state", HeadBranch: "maintenance/attempt-owned"}}
			result, err := (Service{Claims: claims, Workspaces: spaces}).Run(ctx, prepared)
			if !errors.Is(err, cause) || !errors.Is(err, context.Canceled) || !errors.Is(err, nativeErr) {
				t.Fatalf("lost cancellation or native error: %v", err)
			}
			if result.Usage.Tokens != 13 || result.Usage.Turns != 2 || result.Outcome != OutcomeFailed || !result.Quiescent || !result.Retained || runtime.launches != 1 || claims.calls != 1 {
				t.Fatalf("result=%+v launches=%d claims=%d", result, runtime.launches, claims.calls)
			}
		})
	}
}

func TestRunAuditFailuresRefuseOrStopEffects(t *testing.T) {
	t.Run("before claim", func(t *testing.T) {
		runtime, authority := &runtimeFixture{}, &authorityFixture{}
		prepared := preparedFixture(t, entity.Codex, runtime, authority)
		claims, spaces := &claimerFixture{}, &preparerFixture{}
		lifecycle := &lifecycleFixture{fail: BoundaryAdmission, failures: 1}
		result, err := (Service{Claims: claims, Workspaces: spaces, Lifecycle: lifecycle}).Run(t.Context(), prepared)
		if !errors.Is(err, ErrLifecycle) || result.Outcome != OutcomeRefused || claims.calls != 0 || claims.transitions != 0 || spaces.calls != 0 || runtime.launches != 0 {
			t.Fatalf("result=%+v err=%v claim=%d transition=%d workspace=%d launch=%d", result, err, claims.calls, claims.transitions, spaces.calls, runtime.launches)
		}
	})
	t.Run("after positive claim", func(t *testing.T) {
		runtime, authority := &runtimeFixture{}, &authorityFixture{}
		prepared := preparedFixture(t, entity.Codex, runtime, authority)
		claims, spaces := &claimerFixture{result: owned(prepared)}, &preparerFixture{}
		lifecycle := &lifecycleFixture{fail: BoundaryClaim, failures: 1}
		result, err := (Service{Claims: claims, Workspaces: spaces, Lifecycle: lifecycle}).Run(t.Context(), prepared)
		if !errors.Is(err, ErrLifecycle) || result.Outcome != OutcomeFailed || result.Shared == nil || result.Shared.Outcome != claim.OutcomeFailed || claims.transitions != 1 || claims.terminal.Cause != "audit_failed" || spaces.calls != 0 || runtime.launches != 0 {
			t.Fatalf("result=%+v err=%v transitions=%d terminal=%+v workspace=%d launch=%d", result, err, claims.transitions, claims.terminal, spaces.calls, runtime.launches)
		}
	})
	t.Run("after native launch", func(t *testing.T) {
		stopped := make(chan struct{})
		runtime, authority := &runtimeFixture{}, &authorityFixture{}
		runtime.await = func(context.Context, port.Handle) (port.RunResult, error) {
			<-stopped
			return port.RunResult{Tokens: 0, TokensObserved: true}, nil
		}
		runtime.kill = func(context.Context, port.Handle) error { close(stopped); return nil }
		prepared := preparedFixture(t, entity.Codex, runtime, authority)
		claims := &claimerFixture{result: owned(prepared)}
		spaces := &preparerFixture{result: workspace.Prepared{Workdir: "/private/source", RuntimeStateDir: "/private/state", HeadBranch: "maintenance/attempt-owned"}}
		lifecycle := &lifecycleFixture{fail: BoundaryAgent, failures: 1}
		result, err := (Service{Claims: claims, Workspaces: spaces, Lifecycle: lifecycle, CleanupTimeout: time.Second}).Run(t.Context(), prepared)
		if !errors.Is(err, ErrLifecycle) || result.Outcome != OutcomeFailed || !result.Quiescent || !result.Launched || result.Capture == nil || !result.Usage.TokensObserved || claims.transitions != 1 || claims.terminal.Outcome != claim.OutcomeFailed || runtime.kills != 1 {
			t.Fatalf("result=%+v err=%v transitions=%d terminal=%+v kills=%d", result, err, claims.transitions, claims.terminal, runtime.kills)
		}
	})
}

func TestRunTerminalFailuresPreserveClaimAuthority(t *testing.T) {
	t.Run("terminal audit after acknowledged success", func(t *testing.T) {
		runtime, authority := &runtimeFixture{}, &authorityFixture{}
		prepared := preparedFixture(t, entity.Codex, runtime, authority)
		claims := &claimerFixture{result: owned(prepared)}
		spaces := &preparerFixture{result: workspace.Prepared{Workdir: "/private/source", RuntimeStateDir: "/private/state", HeadBranch: "maintenance/attempt-owned"}}
		lifecycle := &lifecycleFixture{fail: BoundaryTerminal, failures: 1}
		finished := prepared.StartedUTC.Add(2 * time.Second)
		result, err := (Service{Claims: claims, Workspaces: spaces, Lifecycle: lifecycle, Now: func() time.Time { return finished }}).Run(t.Context(), prepared)
		if !errors.Is(err, ErrLifecycle) || result.Outcome != OutcomeSucceeded || result.Shared == nil || result.Shared.Outcome != claim.OutcomeSucceeded || result.ElapsedMS == nil || *result.ElapsedMS != 2000 || claims.transitions != 1 {
			t.Fatalf("result=%+v err=%v transitions=%d", result, err, claims.transitions)
		}
	})
	t.Run("terminal mutation or confirmation failure", func(t *testing.T) {
		metadataErr := errors.New("remote confirmation unavailable")
		runtime, authority := &runtimeFixture{}, &authorityFixture{}
		prepared := preparedFixture(t, entity.Codex, runtime, authority)
		claims := &claimerFixture{result: owned(prepared), transition: metadataErr}
		spaces := &preparerFixture{result: workspace.Prepared{Workdir: "/private/source", RuntimeStateDir: "/private/state", HeadBranch: "maintenance/attempt-owned"}}
		lifecycle := &lifecycleFixture{}
		result, err := (Service{Claims: claims, Workspaces: spaces, Lifecycle: lifecycle}).Run(t.Context(), prepared)
		if !errors.Is(err, ErrTerminalMetadata) || !errors.Is(err, metadataErr) || result.Outcome != OutcomeSucceeded || result.Shared != nil || claims.transitions != 1 {
			t.Fatalf("result=%+v err=%v transitions=%d", result, err, claims.transitions)
		}
		last := lifecycle.observations[len(lifecycle.observations)-1]
		if last.Boundary != BoundaryTerminal || last.CauseCode != "terminal_metadata_failed" || !errors.Is(last.Cause, metadataErr) {
			t.Fatalf("terminal diagnostic = %+v", last)
		}
	})
	t.Run("unproved quiescence preserves claimed state", func(t *testing.T) {
		runtime, authority := &runtimeFixture{await: func(context.Context, port.Handle) (port.RunResult, error) {
			return port.RunResult{}, port.ErrNotQuiescent
		}}, &authorityFixture{}
		prepared := preparedFixture(t, entity.Codex, runtime, authority)
		claims := &claimerFixture{result: owned(prepared)}
		result, err := (Service{Claims: claims, Workspaces: &preparerFixture{result: workspace.Prepared{Workdir: "/private/source", RuntimeStateDir: "/private/state", HeadBranch: "maintenance/attempt-owned"}}, Lifecycle: &lifecycleFixture{}}).Run(t.Context(), prepared)
		if !errors.Is(err, port.ErrNotQuiescent) || result.Outcome != OutcomeIncomplete || result.Quiescent || result.Shared != nil || claims.transitions != 0 {
			t.Fatalf("result=%+v err=%v transitions=%d", result, err, claims.transitions)
		}
	})
}
