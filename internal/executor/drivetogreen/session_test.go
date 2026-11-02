package drivetogreen

import (
	"context"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/entity"
	"github.com/agenticlab-ai/pyxis/internal/executor/port"
	"github.com/agenticlab-ai/pyxis/internal/executor/worker"
)

type fakeContinuity struct {
	specs   []port.SessionSpec
	records []port.RunResult
	harness string
}

func (f *fakeContinuity) Session(_ context.Context, task port.Task, harness entity.Harness, _ string) (port.SessionSpec, error) {
	f.harness = harness.String()
	spec := port.SessionSpec{ID: "sess-" + task.ID, Resume: len(f.specs) > 0}
	f.specs = append(f.specs, spec)
	return spec, nil
}

func (f *fakeContinuity) Record(_ context.Context, _ port.Task, _ entity.Harness, _ string, _ port.SessionSpec, res port.RunResult) error {
	f.records = append(f.records, res)
	return nil
}

// With continuity handed over, each local fixer launch continues the task's
// session (first start, then resume) and is recorded; without it, fixers run
// fresh exactly as before.
func TestDriveLocal_FixerContinuesTheTaskSession(t *testing.T) {
	dir := t.TempDir()
	var launched []port.LaunchSpec
	runtime := &fakeRuntime{onLaunch: func(spec port.LaunchSpec) {
		launched = append(launched, spec)
		if len(launched) == 2 {
			writeMarkerOnLaunch(dir)(spec)
		}
	}}
	d := localDriver(t, runtime, &fakeHost{}, &fakePusher{}, 3, nil)
	cont := &fakeContinuity{}
	var events []worker.SessionEvent
	d.UseSessionContinuity(cont, "claude", func(_ port.Task, e worker.SessionEvent) { events = append(events, e) })

	res, err := d.DriveLocal(context.Background(), port.Task{ID: "T1"}, &fakeWorkdir{dir: dir, branch: "feat/x"})
	if err != nil || !res.Green || res.Attempts != 2 {
		t.Fatalf("result = %+v, %v", res, err)
	}
	if len(launched) != 2 || launched[0].Session == nil || launched[0].Session.Resume || launched[1].Session == nil || !launched[1].Session.Resume {
		t.Fatalf("fixer sessions = %+v / %+v", launched[0].Session, launched[1].Session)
	}
	if cont.harness != "claude" || len(cont.records) != 2 {
		t.Fatalf("continuity harness=%q records=%d", cont.harness, len(cont.records))
	}
	if launched[0].RuntimeStateDir == "" || launched[0].RuntimeStateDir == dir || launched[0].RuntimeStateDir != launched[1].RuntimeStateDir {
		t.Fatal("fixers did not share isolated native conversation state")
	}
	if len(events) != 2 || events[0].Action != "started" || events[1].Action != "resumed" || events[0].Purpose != "fix" {
		t.Fatalf("events = %+v", events)
	}

	launched = nil
	plain := localDriver(t, &fakeRuntime{onLaunch: func(spec port.LaunchSpec) { launched = append(launched, spec) }}, &fakeHost{}, &fakePusher{}, 1, nil)
	_, _ = plain.DriveLocal(context.Background(), port.Task{ID: "T1"}, &fakeWorkdir{dir: t.TempDir(), branch: "feat/x"})
	if len(launched) != 1 || launched[0].Session != nil {
		t.Fatalf("without continuity the fixer must run fresh: %+v", launched)
	}
}

func TestDriveLocal_OlderConversationMigratesToIsolatedStorage(t *testing.T) {
	dir := t.TempDir()
	var launch port.LaunchSpec
	runtime := &fakeRuntime{onLaunch: func(spec port.LaunchSpec) {
		launch = spec
		writeMarkerOnLaunch(dir)(spec)
	}}
	d := localDriver(t, runtime, &fakeHost{}, &fakePusher{}, 1, nil)
	cont := &fakeContinuity{specs: []port.SessionSpec{{ID: "previous-conversation"}}}
	var degraded bool
	d.UseSessionContinuity(cont, "claude", func(_ port.Task, event worker.SessionEvent) {
		if event.Action == "degraded" && event.Err != nil {
			degraded = true
		}
	})
	result, err := d.DriveLocal(t.Context(), port.Task{ID: "T1"}, &fakeWorkdir{dir: dir, branch: "feat/x"})
	if err != nil || !result.Green || launch.Session == nil || launch.Session.Resume || launch.RuntimeStateDir == "" || !degraded || len(cont.records) != 1 {
		t.Fatalf("migration: result=%+v session=%+v degraded=%v records=%d err=%v", result, launch.Session, degraded, len(cont.records), err)
	}
}
