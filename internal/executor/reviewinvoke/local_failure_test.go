package reviewinvoke

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/executor/port"
)

func TestReviewLocal_RuntimeFailuresCannotProduceApproval(t *testing.T) {
	world := newLocalWorld(t)
	awaitErr := errors.New("reviewer connection lost")
	for _, tc := range []struct {
		name          string
		badCredential bool
		run           port.RunResult
		awaitErr      error
		phase, want   string
		launches      int
	}{
		{name: "unsafe credential", badCredential: true, phase: "launch", want: "not a regular file"},
		{name: "await failure", awaitErr: awaitErr, phase: "await", want: awaitErr.Error(), launches: 1},
		{name: "incomplete without reason", run: port.RunResult{Incomplete: true}, phase: "await", want: "runtime reported an incomplete run", launches: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			credentials := t.TempDir()
			t.Setenv("CLAUDE_CONFIG_DIR", credentials)
			path := filepath.Join(credentials, ".credentials.json")
			if tc.badCredential {
				if err := os.Mkdir(path, 0700); err != nil {
					t.Fatal(err)
				}
			} else if err := os.WriteFile(path, []byte("{}"), 0600); err != nil {
				t.Fatal(err)
			}
			rt := &fakeRuntime{result: tc.run, awaitErr: tc.awaitErr}
			inv := localInvoker(t, rt, world.repo)
			processes := &fakeProcessCoordinator{}
			inv.processes = processes
			result := inv.ReviewLocal(t.Context(), port.Task{ID: "T-local", Brief: "charter"}, localWorkdir{dir: world.worktree, branch: world.branch}, 1)
			if !result.Failed() || result.FailureClass != "infrastructure" || string(result.Phase) != tc.phase || result.Err == nil || !strings.Contains(result.Err.Error(), tc.want) {
				t.Fatalf("result = %+v, want %s infrastructure failure containing %q", result, tc.phase, tc.want)
			}
			if result.Verdict != "" || result.Artifact != "" || rt.launches != tc.launches {
				t.Fatalf("failed review yielded approval or unexpected launch: result=%+v launches=%d", result, rt.launches)
			}
			if tc.awaitErr != nil && !errors.Is(result.Err, tc.awaitErr) {
				t.Fatalf("await failure lost its cause: %v", result.Err)
			}
			if processes.prepares != tc.launches || processes.completes != tc.launches {
				t.Fatalf("failed review process was admitted or left unsettled: prepares=%d completes=%d", processes.prepares, processes.completes)
			}
			if tc.launches != 0 {
				for _, dir := range []string{rt.spec.Workdir, rt.spec.RuntimeStateDir} {
					if _, err := os.Stat(dir); !errors.Is(err, os.ErrNotExist) {
						t.Fatalf("failed review retained disposable directory %s: %v", dir, err)
					}
				}
			}
		})
	}
}
