package drivetogreen

import (
	"context"
	"errors"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/executor/port"
)

func TestDriveCheckpointsBeforeGateAndPublication(t *testing.T) {
	for _, checkpointError := range []error{nil, errors.New("index is locked")} {
		t.Run("checkpoint error="+errorText(checkpointError), func(t *testing.T) {
			work := &fakeWorkdir{dir: t.TempDir(), branch: "task", checkpointErr: checkpointError}
			runtime := &fakeRuntime{}
			pusher := &fakePusher{}
			host := &fakeHost{states: []port.CheckState{port.CheckRed, port.CheckRed, port.CheckRed, port.CheckGreen}}
			gateCalled := false
			d := mustDriver(t, Deps{Host: host, Runtime: runtime, Fetcher: &fakeFetcher{}, Pusher: pusher,
				Runner: func(context.Context, string, []string) CommandResult {
					gateCalled = true
					if !work.checkpointed {
						t.Error("verification ran before checkpoint")
					}
					return CommandResult{Passed: true}
				}}, Config{Cap: 1, GateChecks: gateChecks()})
			result, err := d.Drive(t.Context(), newTask(), work, port.Proposal{Number: 7})
			if checkpointError != nil {
				if !errors.Is(err, checkpointError) || gateCalled || pusher.pushes != 0 {
					t.Fatalf("checkpoint fault: result=%+v err=%v gate=%v pushes=%d", result, err, gateCalled, pusher.pushes)
				}
			} else if err != nil || !result.Green || !gateCalled || pusher.pushes != 1 {
				t.Fatalf("repair: result=%+v err=%v gate=%v pushes=%d", result, err, gateCalled, pusher.pushes)
			}
		})
	}
}

func TestInterruptedFixerCheckpointsWithoutPublication(t *testing.T) {
	checkpointFault := errors.New("checkpoint refused")
	for _, tc := range []struct {
		name          string
		awaitErr      error
		checkpointErr error
		checkpoint    bool
	}{
		{"cancelled", context.Canceled, nil, true},
		{"checkpoint failure", context.Canceled, checkpointFault, true},
		{"possibly live writer", errors.Join(context.Canceled, port.ErrNotQuiescent), nil, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			work := &fakeWorkdir{dir: t.TempDir(), branch: "task", checkpointErr: tc.checkpointErr}
			pusher := &fakePusher{}
			d := mustDriver(t, Deps{Host: &fakeHost{}, Runtime: &fakeRuntime{awaitErr: tc.awaitErr},
				Fetcher: &fakeFetcher{}, Pusher: pusher, Runner: markerRunner}, Config{Cap: 1, GateChecks: gateChecks()})
			_, err := d.Drive(t.Context(), newTask(), work, port.Proposal{Number: 7})
			if !errors.Is(err, context.Canceled) || (tc.checkpointErr != nil && !errors.Is(err, tc.checkpointErr)) {
				t.Fatalf("interruption and checkpoint causes must remain inspectable: %v", err)
			}
			if work.checkpointed != tc.checkpoint || pusher.pushes != 0 {
				t.Fatalf("checkpointed=%v want=%v pushes=%d", work.checkpointed, tc.checkpoint, pusher.pushes)
			}
		})
	}
}

func errorText(err error) string {
	if err == nil {
		return "none"
	}
	return err.Error()
}
