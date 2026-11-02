package drivetogreen

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/synctest"

	"github.com/agenticlab-ai/pyxis/internal/entity"
	"github.com/agenticlab-ai/pyxis/internal/executor/port"
)

const seededSecret = "ghp_leakedTOKEN0123456789abcdefGHIJKL"

func newTask() port.Task {
	return port.Task{ID: "T-42", Number: 42, Title: "widget", Brief: "Implement the widget per the charter.", Harness: "codex", HeadBranch: "feat/widget-42"}
}

func gateChecks() []Check {
	return []Check{{Name: "build", Argv: []string{"make", "build"}}}
}

// markerRunner is the gate's runner: it reports pass/fail from the SAME predicate
// the gate-mirrors-CI test uses (requiredCheck reads build.ok in the workdir), so
// the fixer "fixes" the tree by writing that marker.
func markerRunner(_ context.Context, dir string, _ []string) CommandResult {
	if requiredCheck(dir) {
		return CommandResult{Passed: true, Output: "build: ok\n"}
	}
	return CommandResult{Passed: false, Output: "build: FAIL — missing build.ok\n"}
}

// writeMarkerOnLaunch is an onLaunch hook that makes the fixer "fix" the tree.
func writeMarkerOnLaunch(dir string) func(port.LaunchSpec) {
	return func(port.LaunchSpec) {
		_ = os.WriteFile(filepath.Join(dir, "build.ok"), []byte("ok"), 0o644)
	}
}

func mustDriver(t *testing.T, deps Deps, cfg Config) *Driver {
	t.Helper()
	auth := t.TempDir()
	t.Setenv("CLAUDE_CONFIG_DIR", auth)
	t.Setenv("CODEX_HOME", auth)
	if err := os.WriteFile(filepath.Join(auth, ".credentials.json"), []byte(`{"token":"unit-test-only"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	d, err := New(deps, cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return d
}

// Acceptance criterion 2: a red proposal is driven to green within the cap, and
// the failed-CI logs are redacted BEFORE they reach the fixer (ABUSE-3, §3.5).
func TestDrive_ConvergesWithinCap_RedactsLogsBeforeFixer(t *testing.T) {
	dir := t.TempDir()
	work := &fakeWorkdir{dir: dir, branch: "feat/widget-42"}
	ciLog := "--- FAIL: TestWidget\ndeploy token=" + seededSecret + "\nassertion failed\n"

	runtime := &fakeRuntime{onLaunch: writeMarkerOnLaunch(dir)} // the fixer repairs the tree
	fetcher := &fakeFetcher{logs: []string{ciLog}}
	pusher := &fakePusher{}
	// Every pre-write observation stays on the expected red head. CI remains red
	// after the first re-push, then goes green after the second → attempt 2.
	host := &fakeHost{states: []port.CheckState{
		port.CheckRed, // initial settlement
		port.CheckRed, // attempt 1 pre-launch guard
		port.CheckRed, // attempt 1 pre-push guard
		port.CheckRed, // attempt 1 post-push CI
		port.CheckRed, // attempt 2 pre-launch guard
		port.CheckRed, // attempt 2 pre-push guard
		port.CheckGreen,
	}}

	var events []AttemptEvent
	d := mustDriver(t, Deps{Host: host, Runtime: runtime, Fetcher: fetcher, Pusher: pusher, Runner: markerRunner},
		Config{Cap: 4, GateChecks: gateChecks(), OnAttempt: func(e AttemptEvent) { events = append(events, e) }})

	res, err := d.Drive(context.Background(), newTask(), work, port.Proposal{Number: 7})
	if err != nil {
		t.Fatalf("Drive: unexpected error: %v", err)
	}
	if !res.Green {
		t.Fatal("proposal should have been driven to green within the cap")
	}
	if res.Attempts != 2 || res.Cap != 4 {
		t.Fatalf("result = %+v, want green at attempt 2 of cap 4", res)
	}

	// The whole point of ABUSE-3: the seeded secret must never reach the fixer.
	if len(runtime.briefs) == 0 {
		t.Fatal("the fixer was never launched")
	}
	for i, brief := range runtime.briefs {
		if strings.Contains(brief, seededSecret) {
			t.Fatalf("the seeded secret reached the fixer in brief %d: %q", i, brief)
		}
	}
	if !strings.Contains(runtime.briefs[0], redactionMark) {
		t.Errorf("the fixer brief should show the redaction mark where the secret was scrubbed: %q", runtime.briefs[0])
	}
	// The fixer brief still carries the real failure signal (the failing test).
	if !strings.Contains(runtime.briefs[0], "TestWidget") {
		t.Errorf("the fixer brief lost the failure signal: %q", runtime.briefs[0])
	}
	// The charter is the instruction; the log is fenced as data (NFR-2, ABUSE-1).
	if !strings.Contains(runtime.briefs[0], "Implement the widget per the charter.") {
		t.Errorf("the fixer brief must carry the task charter: %q", runtime.briefs[0])
	}

	// Two green-gated re-pushes happened (one per attempt), and the attempt count
	// is observable (EVT-3).
	if pusher.pushes != 2 {
		t.Fatalf("pushes = %d, want 2 (one per attempt)", pusher.pushes)
	}
	if len(events) != 2 || events[0].Attempt != 1 || events[1].Attempt != 2 || events[1].Cap != 4 {
		t.Fatalf("attempt events = %+v, want two events (1,2) with cap 4", events)
	}
}

// A proposal that is already fixable converges in a single attempt.
func TestDrive_ConvergesFirstAttempt(t *testing.T) {
	dir := t.TempDir()
	work := &fakeWorkdir{dir: dir}
	runtime := &fakeRuntime{onLaunch: writeMarkerOnLaunch(dir)}
	d := mustDriver(t, Deps{
		Host:    &fakeHost{states: []port.CheckState{port.CheckRed, port.CheckRed, port.CheckRed, port.CheckGreen}},
		Runtime: runtime,
		Fetcher: &fakeFetcher{logs: []string{"boom\n"}},
		Pusher:  &fakePusher{},
		Runner:  markerRunner,
	}, Config{Cap: 3, GateChecks: gateChecks()})

	res, err := d.Drive(context.Background(), newTask(), work, port.Proposal{Number: 5})
	if err != nil {
		t.Fatalf("Drive: %v", err)
	}
	if !res.Green || res.Attempts != 1 {
		t.Fatalf("result = %+v, want green at attempt 1", res)
	}
}

// Acceptance criterion 3: at the self-heal cap the task stops, is flagged failed
// (not green), and escalates (an *EscalationError) — DINV-9, AP-16, FR-9. Two
// paths reach the cap: the local gate never passes, and remote CI never goes
// green.
func TestDrive_CapExhausted_Escalates(t *testing.T) {
	t.Run("local gate never passes", func(t *testing.T) {
		dir := t.TempDir()
		work := &fakeWorkdir{dir: dir}
		runtime := &fakeRuntime{} // the fixer never writes the marker → gate stays red
		pusher := &fakePusher{}
		var events []AttemptEvent
		d := mustDriver(t, Deps{
			Host:    &fakeHost{states: []port.CheckState{port.CheckRed}},
			Runtime: runtime,
			Fetcher: &fakeFetcher{logs: []string{"--- FAIL: TestWidget\nboom\n"}},
			Pusher:  pusher,
			Runner:  markerRunner,
		}, Config{Cap: 3, GateChecks: gateChecks(), OnAttempt: func(e AttemptEvent) { events = append(events, e) }})

		res, err := d.Drive(context.Background(), newTask(), work, port.Proposal{Number: 9})

		var esc *EscalationError
		if !errors.As(err, &esc) {
			t.Fatalf("err = %v, want an *EscalationError", err)
		}
		if res.Green {
			t.Fatal("a cap-exhausted drive must not be green")
		}
		if res.Attempts != 3 || res.Cap != 3 {
			t.Fatalf("result = %+v, want failed at attempt 3 of cap 3", res)
		}
		if esc.TaskID != "T-42" || esc.Attempts != 3 || esc.Cap != 3 {
			t.Fatalf("escalation = %+v, want T-42 3/3", esc)
		}
		if !strings.Contains(esc.Reason, "local gate red") {
			t.Errorf("escalation reason should name the local gate: %q", esc.Reason)
		}
		// A locally-broken fix is never pushed (the gate mirrors CI, FR-8).
		if pusher.pushes != 0 {
			t.Fatalf("a locally-failing fix must never be pushed: pushes = %d", pusher.pushes)
		}
		if len(events) != 3 {
			t.Fatalf("want 3 observable attempts, got %d", len(events))
		}
	})

	t.Run("remote CI never goes green", func(t *testing.T) {
		dir := t.TempDir()
		work := &fakeWorkdir{dir: dir}
		runtime := &fakeRuntime{onLaunch: writeMarkerOnLaunch(dir)} // local gate passes...
		pusher := &fakePusher{}
		d := mustDriver(t, Deps{
			Host:    &fakeHost{states: []port.CheckState{port.CheckRed}}, // ...but CI stays red
			Runtime: runtime,
			Fetcher: &fakeFetcher{logs: []string{"still failing on CI\n"}},
			Pusher:  pusher,
			Runner:  markerRunner,
		}, Config{Cap: 2, GateChecks: gateChecks()})

		res, err := d.Drive(context.Background(), newTask(), work, port.Proposal{Number: 11})

		var esc *EscalationError
		if !errors.As(err, &esc) {
			t.Fatalf("err = %v, want an *EscalationError", err)
		}
		if res.Green || res.Attempts != 2 {
			t.Fatalf("result = %+v, want failed at attempt 2", res)
		}
		if !strings.Contains(esc.Reason, "CI") {
			t.Errorf("escalation reason should name the failing CI: %q", esc.Reason)
		}
		// Each attempt passed the local gate and re-pushed, then CI re-checked red.
		if pusher.pushes != 2 {
			t.Fatalf("pushes = %d, want 2 (one per attempt)", pusher.pushes)
		}
	})
}

// Operational failures stop the loop with a wrapped error (not an escalation) so
// the Worker classifies them failed and the sweep continues (AP-16). Each
// collaborator's failure is exercised.
func TestDrive_OperationalFailuresPropagate(t *testing.T) {
	newDeps := func() Deps {
		return Deps{
			Host:    &fakeHost{states: []port.CheckState{port.CheckRed}},
			Runtime: &fakeRuntime{},
			Fetcher: &fakeFetcher{logs: []string{"boom\n"}},
			Pusher:  &fakePusher{},
			Runner:  func(context.Context, string, []string) CommandResult { return CommandResult{Passed: true} },
		}
	}
	tests := []struct {
		name string
		mut  func(*Deps)
	}{
		{"fetch fails", func(d *Deps) { d.Fetcher = &fakeFetcher{err: errors.New("log api down")} }},
		{"launch fails", func(d *Deps) { d.Runtime = &fakeRuntime{launchErr: errors.New("cli missing")} }},
		{"await fails", func(d *Deps) { d.Runtime = &fakeRuntime{awaitErr: errors.New("process crashed")} }},
		{"gate run fails", func(d *Deps) {
			d.Runner = func(context.Context, string, []string) CommandResult {
				return CommandResult{Err: errors.New("exec not found")}
			}
		}},
		{"push fails", func(d *Deps) { d.Pusher = &fakePusher{err: errors.New("remote rejected")} }},
		{"check read fails", func(d *Deps) { d.Host = &fakeHost{checkErr: errors.New("checks api down")} }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			deps := newDeps()
			tc.mut(&deps)
			d := mustDriver(t, deps, Config{Cap: 3, GateChecks: gateChecks()})

			res, err := d.Drive(context.Background(), newTask(), &fakeWorkdir{dir: t.TempDir()}, port.Proposal{Number: 3})
			if err == nil {
				t.Fatalf("Drive: want an operational error for %q", tc.name)
			}
			var esc *EscalationError
			if errors.As(err, &esc) {
				t.Fatalf("an operational failure must not be an escalation: %v", err)
			}
			if res.Green {
				t.Fatal("an operational failure must not be green")
			}
		})
	}
}

// A newly-opened proposal with no registered/running checks is pending, not
// broken. Settlement polls consume no fixer, push, failed-log read, or attempt.
func TestDrive_PendingToGreenConsumesNoRepairAttempt(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		host := &fakeHost{observations: []proposalObservation{
			{landing: port.LandingOpen, head: "head-a", checks: port.CheckPending},
			{landing: port.LandingOpen, head: "head-a", checks: port.CheckPending},
			{landing: port.LandingOpen, head: "head-a", checks: port.CheckGreen},
		}}
		runtime := &fakeRuntime{}
		fetcher := &fakeFetcher{}
		pusher := &fakePusher{}
		var events []AttemptEvent
		d := mustDriver(t, Deps{
			Host: host, Inspector: host, Runtime: runtime, Fetcher: fetcher, Pusher: pusher,
			Runner: func(context.Context, string, []string) CommandResult { return CommandResult{Passed: true} },
		}, Config{Cap: 3, SettleInterval: DefaultSettleInterval, OnAttempt: func(e AttemptEvent) { events = append(events, e) }})

		res, err := d.Drive(context.Background(), newTask(), &fakeWorkdir{dir: t.TempDir()}, port.Proposal{Number: 12})
		if err != nil {
			t.Fatalf("Drive: %v", err)
		}
		if !res.Green || res.Attempts != 0 {
			t.Fatalf("result = %+v, want green with zero repair attempts", res)
		}
		if len(runtime.briefs) != 0 || fetcher.calls != 0 || pusher.pushes != 0 || len(events) != 0 {
			t.Fatalf("pending spent repair work: fixer=%d logs=%d pushes=%d events=%d", len(runtime.briefs), fetcher.calls, pusher.pushes, len(events))
		}

	})
}

func TestDrive_PendingToGreenNeedsNoRuntimeRouteOrReservation(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		resolved, reserved := 0, 0
		host := &fakeHost{observations: []proposalObservation{
			{landing: port.LandingOpen, head: "head-a", checks: port.CheckPending},
			{landing: port.LandingOpen, head: "head-a", checks: port.CheckGreen},
		}}
		d := mustDriver(t, Deps{
			Host: host, Inspector: host,
			ResolveRuntime: func(context.Context) (entity.Harness, port.AgentRuntime, error) {
				resolved++
				return "codex", &fakeRuntime{}, nil
			},
			Fetcher: &fakeFetcher{}, Pusher: &fakePusher{}, Runner: markerRunner,
		}, Config{Cap: 1, ReserveAttempt: func(context.Context, string) (int, int, error) {
			reserved++
			return 1, 1, nil
		}})
		result, err := d.Drive(context.Background(), newTask(), &fakeWorkdir{dir: t.TempDir()}, port.Proposal{Number: 12})
		if err != nil || !result.Green || resolved != 0 || reserved != 0 {
			t.Fatalf("pending continuation = %+v, %v; resolved=%d reserved=%d", result, err, resolved, reserved)
		}
	})
}

func TestDriveCancellationWhilePendingIsExplicitAndSpendsNothing(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		resolved, reserved := 0, 0
		host := &fakeHost{observations: []proposalObservation{{landing: port.LandingOpen, head: "head-a", checks: port.CheckPending}}}
		d := mustDriver(t, Deps{
			Host: host, Inspector: host,
			ResolveRuntime: func(context.Context) (entity.Harness, port.AgentRuntime, error) {
				resolved++
				return "codex", &fakeRuntime{}, nil
			},
			Fetcher: &fakeFetcher{}, Pusher: &fakePusher{}, Runner: markerRunner,
		}, Config{Cap: 1, ReserveAttempt: func(context.Context, string) (int, int, error) {
			reserved++
			return 1, 1, nil
		}})
		dir := t.TempDir()
		done := make(chan error, 1)
		go func() {
			_, err := d.Drive(ctx, newTask(), &fakeWorkdir{dir: dir}, port.Proposal{Number: 12})
			done <- err
		}()
		synctest.Wait()
		cancel()
		if err := <-done; !errors.Is(err, context.Canceled) || resolved != 0 || reserved != 0 {
			t.Fatalf("pending cancellation = %v; resolved=%d reserved=%d", err, resolved, reserved)
		}
	})
}

func TestDriveRuntimeRefusalDoesNotConsumeRepairAuthority(t *testing.T) {
	reserved, resolved := 0, 0
	host := &fakeHost{observations: []proposalObservation{
		{landing: port.LandingOpen, head: "head-a", checks: port.CheckRed},
		{landing: port.LandingOpen, head: "head-a", checks: port.CheckRed},
	}}
	d := mustDriver(t, Deps{
		Host: host, Inspector: host,
		ResolveRuntime: func(context.Context) (entity.Harness, port.AgentRuntime, error) {
			resolved++
			return "", nil, errors.New("auth refused")
		},
		Fetcher: &fakeFetcher{logs: []string{"red"}}, Pusher: &fakePusher{}, Runner: markerRunner,
	}, Config{Cap: 1, ReserveAttempt: func(context.Context, string) (int, int, error) {
		reserved++
		return 1, 1, nil
	}})
	_, err := d.Drive(context.Background(), newTask(), &fakeWorkdir{dir: t.TempDir()}, port.Proposal{Number: 12})
	if err == nil || reserved != 0 || resolved != 1 {
		t.Fatalf("red route refusal = %v; reserved=%d resolved=%d", err, reserved, resolved)
	}
}

// After a legitimate red repair is pushed, its new CI run may be pending. The
// same attempt waits for settlement instead of launching a second fixer.
func TestDrive_RedRepairWaitsThroughPendingToGreen(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		dir := t.TempDir()
		host := &fakeHost{observations: []proposalObservation{
			{landing: port.LandingOpen, head: "head-a", checks: port.CheckRed},     // initial verdict
			{landing: port.LandingOpen, head: "head-a", checks: port.CheckRed},     // pre-launch guard
			{landing: port.LandingOpen, head: "head-a", checks: port.CheckRed},     // pre-push guard
			{landing: port.LandingOpen, head: "head-b", checks: port.CheckPending}, // new head, CI queued
			{landing: port.LandingOpen, head: "head-b", checks: port.CheckPending}, // CI running
			{landing: port.LandingOpen, head: "head-b", checks: port.CheckGreen},
		}}
		runtime := &fakeRuntime{onLaunch: writeMarkerOnLaunch(dir)}
		pusher := &fakePusher{}
		d := mustDriver(t, Deps{
			Host: host, Inspector: host, Runtime: runtime, Fetcher: &fakeFetcher{logs: []string{"failure"}}, Pusher: pusher, Runner: markerRunner,
		}, Config{Cap: 3, GateChecks: gateChecks(), SettleInterval: DefaultSettleInterval})

		res, err := d.Drive(context.Background(), newTask(), &fakeWorkdir{dir: dir}, port.Proposal{Number: 13})
		if err != nil {
			t.Fatalf("Drive: %v", err)
		}
		if !res.Green || res.Attempts != 1 || len(runtime.briefs) != 1 || pusher.pushes != 1 {
			t.Fatalf("result=%+v fixer=%d pushes=%d, want one repair attempt", res, len(runtime.briefs), pusher.pushes)
		}

	})
}

// A human merge during a fixer run is re-observed before the push. Green
// landing ends successfully and leaves the local repair unpushed.
func TestDrive_MergeDuringRepairStopsBeforePush(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		dir := t.TempDir()
		host := &fakeHost{observations: []proposalObservation{
			{landing: port.LandingOpen, head: "head-a", checks: port.CheckRed},
			{landing: port.LandingOpen, head: "head-a", checks: port.CheckRed},
			{landing: port.LandingLanded, head: "head-a", checks: port.CheckGreen},
		}}
		pusher := &fakePusher{}
		d := mustDriver(t, Deps{
			Host: host, Inspector: host, Runtime: &fakeRuntime{onLaunch: writeMarkerOnLaunch(dir)},
			Fetcher: &fakeFetcher{logs: []string{"failure"}}, Pusher: pusher, Runner: markerRunner,
		}, Config{Cap: 3, GateChecks: gateChecks(), SettleInterval: DefaultSettleInterval})

		res, err := d.Drive(context.Background(), newTask(), &fakeWorkdir{dir: dir}, port.Proposal{Number: 14})
		if err != nil {
			t.Fatalf("Drive: %v", err)
		}
		if !res.Green || res.Attempts != 1 || pusher.pushes != 0 {
			t.Fatalf("result=%+v pushes=%d, want green landing with no repair push", res, pusher.pushes)
		}

	})
}

// A foreign head advance is movement, not a CI-red verdict. The driver stops
// before pushing and reports the typed cause while preserving local work.
func TestDrive_HeadAdvanceDuringRepairStopsBeforePush(t *testing.T) {
	dir := t.TempDir()
	host := &fakeHost{observations: []proposalObservation{
		{landing: port.LandingOpen, head: "head-a", checks: port.CheckRed},
		{landing: port.LandingOpen, head: "head-a", checks: port.CheckRed},
		{landing: port.LandingOpen, head: "head-b", checks: port.CheckRed},
	}}
	pusher := &fakePusher{}
	d := mustDriver(t, Deps{
		Host: host, Inspector: host, Runtime: &fakeRuntime{onLaunch: writeMarkerOnLaunch(dir)},
		Fetcher: &fakeFetcher{logs: []string{"failure"}}, Pusher: pusher, Runner: markerRunner,
	}, Config{Cap: 3, GateChecks: gateChecks()})

	_, err := d.Drive(context.Background(), newTask(), &fakeWorkdir{dir: dir}, port.Proposal{Number: 15})
	var moved *ProposalChangedError
	if !errors.As(err, &moved) || moved.ExpectedHead != "head-a" || moved.ActualHead != "head-b" {
		t.Fatalf("err = %v, want typed head-advance error", err)
	}
	if pusher.pushes != 0 {
		t.Fatalf("foreign head advance received %d pushes, want zero", pusher.pushes)
	}
}

// force-with-lease can lose the final race after the pre-push observation. A
// fresh read that finds the expected proposal green+merged converts the stale
// rejection into successful completion rather than a false red task.
func TestDrive_PushRejectedAfterConcurrentGreenMergeSucceeds(t *testing.T) {
	dir := t.TempDir()
	host := &fakeHost{observations: []proposalObservation{
		{landing: port.LandingOpen, head: "head-a", checks: port.CheckRed},
		{landing: port.LandingOpen, head: "head-a", checks: port.CheckRed},
		{landing: port.LandingOpen, head: "head-a", checks: port.CheckRed},
		{landing: port.LandingLanded, head: "head-a", checks: port.CheckGreen},
	}}
	pusher := &fakePusher{err: errors.New("stale info")}
	d := mustDriver(t, Deps{
		Host: host, Inspector: host, Runtime: &fakeRuntime{onLaunch: writeMarkerOnLaunch(dir)},
		Fetcher: &fakeFetcher{logs: []string{"failure"}}, Pusher: pusher, Runner: markerRunner,
	}, Config{Cap: 3, GateChecks: gateChecks()})

	res, err := d.Drive(context.Background(), newTask(), &fakeWorkdir{dir: dir}, port.Proposal{Number: 16})
	if err != nil {
		t.Fatalf("Drive: %v", err)
	}
	if !res.Green || res.Attempts != 1 || pusher.pushes != 1 {
		t.Fatalf("result=%+v pushes=%d, want concurrent landing recognized after guarded rejection", res, pusher.pushes)
	}
}

// A cancelled context stops the loop promptly with a wrapped cancellation error.
func TestDrive_HonorsCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	d := mustDriver(t, Deps{
		Host:    &fakeHost{states: []port.CheckState{port.CheckRed}},
		Runtime: &fakeRuntime{},
		Fetcher: &fakeFetcher{logs: []string{"boom\n"}},
		Pusher:  &fakePusher{},
		Runner:  markerRunner,
	}, Config{Cap: 3, GateChecks: gateChecks()})

	_, err := d.Drive(ctx, newTask(), &fakeWorkdir{dir: t.TempDir()}, port.Proposal{Number: 1})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
}

// New defaults an unset cap to DefaultCap and clamps a non-positive one, never to
// an unbounded loop (AP-16). A nil dependency is a wiring error.
func TestNew_DefaultsAndValidation(t *testing.T) {
	full := func() Deps {
		return Deps{
			Host: &fakeHost{}, Runtime: &fakeRuntime{}, Fetcher: &fakeFetcher{},
			Pusher: &fakePusher{}, Runner: markerRunner,
		}
	}
	d, err := New(full(), Config{}) // Cap unset
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if d.Cap() != DefaultCap {
		t.Fatalf("Cap() = %d, want DefaultCap %d", d.Cap(), DefaultCap)
	}
	if d.Gate() == nil {
		t.Fatal("Gate() must expose the shared local gate")
	}

	if d2, _ := New(full(), Config{Cap: -5}); d2.Cap() != DefaultCap {
		t.Fatalf("a non-positive cap must clamp to DefaultCap, got %d", d2.Cap())
	}

	for _, tc := range []struct {
		name string
		mut  func(*Deps)
	}{
		{"nil host", func(d *Deps) { d.Host = nil }},
		{"nil runtime", func(d *Deps) { d.Runtime = nil }},
		{"nil fetcher", func(d *Deps) { d.Fetcher = nil }},
		{"nil pusher", func(d *Deps) { d.Pusher = nil }},
		{"nil runner", func(d *Deps) { d.Runner = nil }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			deps := full()
			tc.mut(&deps)
			if _, err := New(deps, Config{}); err == nil {
				t.Fatalf("New(%s): want a wiring error", tc.name)
			}
		})
	}
}

// A nil workdir is a wiring error, reported (never panicked).
func TestDrive_NilWorkdirIsError(t *testing.T) {
	d := mustDriver(t, Deps{
		Host: &fakeHost{}, Runtime: &fakeRuntime{}, Fetcher: &fakeFetcher{},
		Pusher: &fakePusher{}, Runner: markerRunner,
	}, Config{Cap: 2})
	if _, err := d.Drive(context.Background(), newTask(), nil, port.Proposal{Number: 1}); err == nil {
		t.Fatal("Drive(nil workdir): want an error")
	}
}
