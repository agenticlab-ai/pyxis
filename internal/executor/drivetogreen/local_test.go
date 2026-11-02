package drivetogreen

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/executor/port"
)

// DriveLocal is the "ensure local CI is green" step: it never pushes and never
// reads the code host. These tests pin its four terminals — green first try
// (zero model calls), red→fix→green within the cap, red past the cap
// (escalation, nothing pushed), and a gate that cannot run (fail closed).

func localDriver(t *testing.T, runtime *fakeRuntime, host *fakeHost, pusher *fakePusher, cap int, onAttempt func(AttemptEvent)) *Driver {
	t.Helper()
	return mustDriver(t, Deps{Host: host, Runtime: runtime, Fetcher: &fakeFetcher{}, Pusher: pusher, Runner: markerRunner},
		Config{Cap: cap, GateChecks: gateChecks(), OnAttempt: onAttempt})
}

func TestDriveLocal_GreenFirstTryLaunchesNothing(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "build.ok"), []byte("ok"), 0o644); err != nil {
		t.Fatal(err)
	}
	runtime, host, pusher := &fakeRuntime{}, &fakeHost{}, &fakePusher{}
	var events []AttemptEvent
	d := localDriver(t, runtime, host, pusher, 3, func(e AttemptEvent) { events = append(events, e) })

	res, err := d.DriveLocal(context.Background(), port.Task{ID: "T1"}, &fakeWorkdir{dir: dir, branch: "feat/x"})
	if err != nil {
		t.Fatalf("DriveLocal: %v", err)
	}
	if !res.Green || res.Attempts != 0 || res.Cap != 3 {
		t.Fatalf("result = %+v, want green with zero attempts at cap 3", res)
	}
	if len(runtime.briefs) != 0 {
		t.Fatalf("a green tree launched %d fixer(s), want 0 (deterministic work spends no inference)", len(runtime.briefs))
	}
	if len(events) != 0 || pusher.pushes != 0 || host.calls != 0 || host.inspectCalls != 0 {
		t.Fatalf("events=%d pushes=%d host reads=%d/%d — a local drive must touch neither the code host nor the push", len(events), pusher.pushes, host.calls, host.inspectCalls)
	}
}

func TestDriveLocal_RedFixesToGreenWithinCap(t *testing.T) {
	dir := t.TempDir()
	runtime := &fakeRuntime{onLaunch: writeMarkerOnLaunch(dir)}
	host, pusher := &fakeHost{}, &fakePusher{}
	var events []AttemptEvent
	d := localDriver(t, runtime, host, pusher, 3, func(e AttemptEvent) { events = append(events, e) })

	res, err := d.DriveLocal(context.Background(), port.Task{ID: "T1", Brief: "build the widget"}, &fakeWorkdir{dir: dir, branch: "feat/x"})
	if err != nil {
		t.Fatalf("DriveLocal: %v", err)
	}
	if !res.Green || res.Attempts != 1 {
		t.Fatalf("result = %+v, want green after exactly one local fix", res)
	}
	if len(runtime.briefs) != 1 || !strings.Contains(runtime.briefs[0], "build: FAIL") || !strings.Contains(runtime.briefs[0], "build the widget") {
		t.Fatalf("fixer briefs = %q, want one brief carrying the charter and the gate's own output", runtime.briefs)
	}
	if len(events) != 1 || !events[0].Local || events[0].Attempt != 1 || events[0].Cap != 3 || events[0].TaskID != "T1" {
		t.Fatalf("events = %+v, want one LOCAL attempt 1/3 for T1", events)
	}
	if pusher.pushes != 0 || host.calls != 0 || host.inspectCalls != 0 {
		t.Fatalf("pushes=%d host reads=%d/%d — the local loop must never push or read the code host", pusher.pushes, host.calls, host.inspectCalls)
	}
}

func TestDriveLocal_CapExhaustedEscalatesWithoutPush(t *testing.T) {
	dir := t.TempDir()
	runtime := &fakeRuntime{} // the fixer never repairs the tree
	host, pusher := &fakeHost{}, &fakePusher{}
	var events []AttemptEvent
	d := localDriver(t, runtime, host, pusher, 2, func(e AttemptEvent) { events = append(events, e) })

	res, err := d.DriveLocal(context.Background(), port.Task{ID: "T1"}, &fakeWorkdir{dir: dir, branch: "feat/x"})
	var esc *EscalationError
	if !errors.As(err, &esc) {
		t.Fatalf("err = %v, want *EscalationError at the cap", err)
	}
	if esc.Attempts != 2 || esc.Cap != 2 || !strings.Contains(esc.Reason, "local gate red: build") {
		t.Fatalf("escalation = %+v, want 2/2 naming the failing check", esc)
	}
	if res.Green || res.Attempts != 2 {
		t.Fatalf("result = %+v, want not green after 2 attempts", res)
	}
	if len(runtime.briefs) != 2 || len(events) != 2 {
		t.Fatalf("fixers=%d events=%d, want exactly cap (2) of each", len(runtime.briefs), len(events))
	}
	if pusher.pushes != 0 || host.calls != 0 {
		t.Fatalf("pushes=%d host reads=%d — escalation must leave nothing pushed", pusher.pushes, host.calls)
	}
}

func TestDriveLocal_GateRunFailureFailsClosed(t *testing.T) {
	dir := t.TempDir()
	runtime, host, pusher := &fakeRuntime{}, &fakeHost{}, &fakePusher{}
	d := mustDriver(t, Deps{Host: host, Runtime: runtime, Fetcher: &fakeFetcher{}, Pusher: pusher,
		Runner: func(context.Context, string, []string) CommandResult {
			return CommandResult{Err: errors.New("go: command not found")}
		}}, Config{Cap: 3, GateChecks: gateChecks()})

	res, err := d.DriveLocal(context.Background(), port.Task{ID: "T1"}, &fakeWorkdir{dir: dir, branch: "feat/x"})
	if err == nil || !strings.Contains(err.Error(), "command not found") {
		t.Fatalf("err = %v, want the gate's operational failure surfaced", err)
	}
	var esc *EscalationError
	if errors.As(err, &esc) {
		t.Fatal("an operational gate failure must not be reported as an escalation")
	}
	if res.Green || len(runtime.briefs) != 0 {
		t.Fatalf("result=%+v fixers=%d — a gate that cannot run certifies nothing and launches no fixer", res, len(runtime.briefs))
	}
}

func TestDriveLocal_FixerLaunchFailureStopsLoop(t *testing.T) {
	dir := t.TempDir()
	runtime := &fakeRuntime{launchErr: errors.New("claude: not installed")}
	d := localDriver(t, runtime, &fakeHost{}, &fakePusher{}, 3, nil)

	res, err := d.DriveLocal(context.Background(), port.Task{ID: "T1"}, &fakeWorkdir{dir: dir, branch: "feat/x"})
	if err == nil || !strings.Contains(err.Error(), "not installed") {
		t.Fatalf("err = %v, want the launch failure", err)
	}
	if res.Green || res.Attempts != 1 {
		t.Fatalf("result = %+v, want not green with the one attempt consumed", res)
	}
}

func TestDriveLocal_NilWorkdirIsWiringError(t *testing.T) {
	d := localDriver(t, &fakeRuntime{}, &fakeHost{}, &fakePusher{}, 3, nil)
	if _, err := d.DriveLocal(context.Background(), port.Task{ID: "T1"}, nil); err == nil {
		t.Fatal("nil workdir must be refused")
	}
}

func TestDriveLocal_HonorsCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	d := localDriver(t, &fakeRuntime{}, &fakeHost{}, &fakePusher{}, 3, nil)
	if _, err := d.DriveLocal(ctx, port.Task{ID: "T1"}, &fakeWorkdir{dir: t.TempDir(), branch: "feat/x"}); !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
}
