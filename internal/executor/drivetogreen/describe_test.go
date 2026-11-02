package drivetogreen

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/executor/port"
)

// The owning Worker's describer runs before every re-push, after the repair is
// checkpointed, so the pushed revision's checks read its description. A failed
// refresh stops the loop before anything is pushed.
func TestDrive_DescribesBeforeEachRepush(t *testing.T) {
	for _, describeErr := range []error{nil, errors.New("code host down")} {
		t.Run("describe error="+errorText(describeErr), func(t *testing.T) {
			dir := t.TempDir()
			work := &fakeWorkdir{dir: dir}
			pusher := &fakePusher{}
			d := mustDriver(t, Deps{
				Host:    &fakeHost{states: []port.CheckState{port.CheckRed, port.CheckRed, port.CheckRed, port.CheckGreen}},
				Runtime: &fakeRuntime{onLaunch: writeMarkerOnLaunch(dir)},
				Fetcher: &fakeFetcher{logs: []string{"boom\n"}},
				Pusher:  pusher,
				Runner:  markerRunner,
			}, Config{Cap: 3, GateChecks: gateChecks()})
			var described []int
			d.BindDescriber(func(_ context.Context, w port.Workdir) error {
				if w != work || !work.checkpointed {
					t.Error("described before the repair was checkpointed")
				}
				described = append(described, pusher.pushes)
				return describeErr
			})
			res, err := d.Drive(context.Background(), newTask(), work, port.Proposal{Number: 5})
			if describeErr != nil {
				if !errors.Is(err, describeErr) || !strings.Contains(err.Error(), "before re-push") || pusher.pushes != 0 || res.Green {
					t.Fatalf("a failed refresh: res=%+v err=%v pushes=%d", res, err, pusher.pushes)
				}
				return
			}
			if err != nil || !res.Green || pusher.pushes != 1 || len(described) != 1 || described[0] != 0 {
				t.Fatalf("res=%+v err=%v pushes=%d described at pushes %v", res, err, pusher.pushes, described)
			}
		})
	}
}
